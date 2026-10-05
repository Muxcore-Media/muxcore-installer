package tui

import (
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initMetadata() {
	if !m.answers.HasLibrary("Movies") && !m.answers.HasLibrary("TV") {
		m.initDatabase()
		return
	}
	m.metaMenu = NewMenu([]MenuItem{
		{Label: "Offline sample titles", Desc: "No API key needed — good for trying MuxCore out"},
		{Label: "TMDB API key", Desc: "Real posters and descriptions for your movies and shows"},
	}, false)
	if !m.answers.TMDBFixture {
		m.metaMenu.Cursor = 1
	}
	m.metaKeyInput = newTextInput("TMDB v3 API key", m.answers.TMDBAPIKey, true)
	m.metaLangInput = newTextInput("en-US", orDefaultStr(m.answers.MetadataLang, "en-US"), false)
	m.metaFocus = 0
	m.metaChecking = false
	m.metaResult = ""
	m.metaSpinner = newSpinner()
	m.phase = phaseMetadata
}

func orDefaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

type metaCheckMsg struct {
	ok   bool
	note string
}

func checkTMDBCmd(key string) tea.Cmd {
	return func() tea.Msg {
		ok, note := probeTMDB(key)
		return metaCheckMsg{ok: ok, note: note}
	}
}

func probeTMDB(key string) (bool, string) {
	if key == "" {
		return false, "enter a key first"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.themoviedb.org/3/configuration?api_key=" + key)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 200 {
		return true, "valid key"
	}
	return false, resp.Status
}

func (m *Model) handleMetaCheck(msg metaCheckMsg) (tea.Model, tea.Cmd) {
	m.metaChecking = false
	if msg.ok {
		m.metaResult = "valid key"
		m.answers.TMDBValidated = true
	} else {
		m.metaResult = "problem: " + msg.note
		m.answers.TMDBValidated = false
	}
	return m, nil
}

func (m *Model) updateMetadata(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.metaFocus == 0 {
		if km.String() == "q" {
			return m, quitCmd()
		}
		if m.metaMenu.HandleKey(km) {
			if m.metaMenu.Cursor == 0 {
				m.answers.TMDBFixture = true
				m.answers.MetadataLang = m.metaLangInput.Value()
				m.initDatabase()
				return m, nil
			}
			m.answers.TMDBFixture = false
			m.metaFocus = 1
			m.metaKeyInput.Focus()
		}
		return m, nil
	}
	switch km.String() {
	case "ctrl+t":
		m.metaChecking = true
		m.metaResult = ""
		return m, tea.Batch(checkTMDBCmd(m.metaKeyInput.Value()), m.metaSpinner.Tick)
	case "tab":
		m.metaFocus = 2
		m.metaKeyInput.Blur()
		m.metaLangInput.Focus()
		return m, nil
	case "enter":
		if m.metaKeyInput.Value() == "" {
			return m, nil // TMDB key required once this branch is chosen
		}
		m.answers.TMDBAPIKey = m.metaKeyInput.Value()
		m.answers.MetadataLang = orDefaultStr(m.metaLangInput.Value(), "en-US")
		m.initDatabase()
		return m, nil
	case "esc":
		m.metaFocus = 0
		return m, nil
	}
	var cmd tea.Cmd
	if m.metaFocus == 1 {
		m.metaKeyInput, cmd = m.metaKeyInput.Update(km)
	} else {
		m.metaLangInput, cmd = m.metaLangInput.Update(km)
	}
	return m, cmd
}

func (m *Model) viewMetadata() string {
	extra := ""
	if m.answers.HasLibrary("Music") {
		extra = "\n" + styleHint.Render("Music metadata uses an offline MusicBrainz sample set automatically — no key needed.")
	}
	if m.metaFocus == 0 {
		return styleHint.Render("Movie and TV metadata") + "\n\n" + m.metaMenu.View() + extra
	}
	body := styleFieldLabel.Render("TMDB API key") + " " + m.metaKeyInput.View() + "\n\n"
	if m.metaChecking {
		body += m.metaSpinner.View() + " validating…\n\n"
	} else if m.metaResult != "" {
		if m.answers.TMDBValidated {
			body += styleOK.Render("✓ "+m.metaResult) + "\n\n"
		} else {
			body += styleErr.Render("✗ "+m.metaResult) + "\n\n"
		}
	}
	body += styleFieldLabel.Render("Language/region") + " " + m.metaLangInput.View() + "\n\n"
	body += styleHint.Render("ctrl+t validate key · tab switch field · enter continue · esc back")
	return body + extra
}
