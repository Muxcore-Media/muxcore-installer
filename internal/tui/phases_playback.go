package tui

import (
	"crypto/tls"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initPlayback() {
	def := []string{}
	if m.answers.HasLibrary("Movies") || m.answers.HasLibrary("TV") {
		def = []string{"MuxCore player"}
	}
	m.playbackMenu = NewMenu([]MenuItem{
		{Label: "MuxCore player", Desc: "Built-in playback — needs ffmpeg for on-the-fly transcoding"},
		{Label: "Jellyfin", Desc: "Bridge to a Jellyfin server you already run"},
		{Label: "Plex", Desc: "Bridge to a Plex server you already run"},
		{Label: "Emby", Desc: "Bridge to an Emby server you already run"},
		{Label: "DLNA", Desc: "Expose your library to DLNA/UPnP clients on your LAN"},
	}, true)
	m.playbackMenu.PreselectValues(def)
	m.answers.Playback = def
	m.phase = phasePlayback
}

func (m *Model) updatePlayback(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.playbackMenu.HandleKey(km) {
		m.answers.Playback = m.playbackMenu.Selected()
		// A missing ffmpeg is non-fatal: media-ffprobe/transcoder just run in a
		// reduced mode, so there is nothing to check here.
		m.credQueue = nil
		for _, name := range []string{"Jellyfin", "Plex", "Emby"} {
			if m.answers.HasPlayback(name) {
				m.credQueue = append(m.credQueue, name)
			}
		}
		m.credIdx = 0
		if len(m.credQueue) > 0 {
			m.initPlaybackCred(m.credQueue[0])
		} else {
			m.initNetwork()
		}
	}
	return m, nil
}

func (m *Model) viewPlayback() string {
	return styleHint.Render("MuxCore can play files itself, or talk to a media server you already run.") +
		"\n\n" + m.playbackMenu.View()
}

// ---- per-server credential entry with a live connection check ----

func (m *Model) initPlaybackCred(name string) {
	var defURL string
	switch name {
	case "Jellyfin":
		defURL = "http://127.0.0.1:8096"
	case "Plex":
		defURL = "http://127.0.0.1:32400"
	case "Emby":
		defURL = "http://127.0.0.1:8096"
	}
	m.credURL = newTextInput(defURL, defURL, false)
	m.credTok = newTextInput("token / API key", "", true)
	m.credURL.Focus()
	m.credFocus = 0
	m.credChecking = false
	m.credResult = ""
	m.credSpinner = newSpinner()
	m.phase = phasePlaybackCreds
}

type credCheckMsg struct {
	name string
	ok   bool
	note string
}

func checkServerCmd(name, url, token string) tea.Cmd {
	return func() tea.Msg {
		ok, note := probeServer(name, url, token)
		return credCheckMsg{name: name, ok: ok, note: note}
	}
}

func probeServer(name, url, token string) (bool, string) {
	if url == "" || token == "" {
		return false, "URL and token/key are both required"
	}
	client := &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var req *http.Request
	switch name {
	case "Jellyfin", "Emby":
		req, _ = http.NewRequest(http.MethodGet, url+"/System/Info?api_key="+token, nil)
	case "Plex":
		req, _ = http.NewRequest(http.MethodGet, url+"/identity?X-Plex-Token="+token, nil)
	}
	if req == nil {
		return false, "unsupported server"
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, "connected"
	}
	return false, resp.Status
}

func (m *Model) handleCredCheck(msg credCheckMsg) (tea.Model, tea.Cmd) {
	m.credChecking = false
	if msg.ok {
		m.credResult = "connected"
	} else {
		m.credResult = "could not connect: " + msg.note
	}
	cred := &m.answers.Jellyfin
	switch msg.name {
	case "Plex":
		cred = &m.answers.Plex
	case "Emby":
		cred = &m.answers.Emby
	}
	cred.URL, cred.Token, cred.OK = m.credURL.Value(), m.credTok.Value(), msg.ok
	return m, nil
}

func (m *Model) updatePlaybackCreds(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	name := m.credQueue[m.credIdx]
	switch km.String() {
	case "q":
		return m, quitCmd()
	case "tab", "down":
		m.credFocus = (m.credFocus + 1) % 2
		m.focusCred()
		return m, nil
	case "up":
		m.credFocus = (m.credFocus + 1) % 2
		m.focusCred()
		return m, nil
	case "ctrl+t":
		m.credChecking = true
		m.credResult = ""
		return m, tea.Batch(checkServerCmd(name, m.credURL.Value(), m.credTok.Value()), m.credSpinner.Tick)
	case "enter":
		m.advanceCredQueue(name)
		return m, nil
	case "esc":
		m.answers.Playback = removeFromList(m.answers.Playback, name)
		m.advanceCredQueue(name)
		return m, nil
	}
	var cmd tea.Cmd
	if m.credFocus == 0 {
		m.credURL, cmd = m.credURL.Update(km)
	} else {
		m.credTok, cmd = m.credTok.Update(km)
	}
	return m, cmd
}

func (m *Model) focusCred() {
	if m.credFocus == 0 {
		m.credURL.Focus()
		m.credTok.Blur()
	} else {
		m.credTok.Focus()
		m.credURL.Blur()
	}
}

func (m *Model) advanceCredQueue(name string) {
	if m.credURL.Value() != "" && m.credTok.Value() != "" {
		cred := &m.answers.Jellyfin
		switch name {
		case "Plex":
			cred = &m.answers.Plex
		case "Emby":
			cred = &m.answers.Emby
		}
		cred.URL, cred.Token = m.credURL.Value(), m.credTok.Value()
	} else {
		m.answers.Playback = removeFromList(m.answers.Playback, name)
	}
	m.credIdx++
	if m.credIdx < len(m.credQueue) {
		m.initPlaybackCred(m.credQueue[m.credIdx])
	} else {
		m.initNetwork()
	}
}

func removeFromList(list []string, v string) []string {
	var out []string
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func (m *Model) viewPlaybackCreds() string {
	name := m.credQueue[m.credIdx]
	body := styleHint.Render("Existing "+name+" URL") + "\n" + m.credURL.View() + "\n\n" +
		styleHint.Render(name+" API key / token") + "\n" + m.credTok.View() + "\n\n"
	if m.credChecking {
		body += m.credSpinner.View() + " checking connection…\n"
	} else if m.credResult != "" {
		if m.credResult == "connected" {
			body += styleOK.Render("✓ "+m.credResult) + "\n"
		} else {
			body += styleErr.Render("✗ "+m.credResult) + "\n"
		}
	}
	body += "\n" + styleHint.Render("tab switch field · ctrl+t test connection · enter continue · esc skip "+name)
	return body
}
