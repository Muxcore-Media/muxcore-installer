package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/prereqs"
)

func (m *Model) initRuntimeKeep() {
	m.runtimeMenu = NewMenu([]MenuItem{
		{Label: "Just run it now", Desc: "Starts with ./up.sh; stops when you log out or reboot", Value: "host:once"},
		{Label: "Keep running when I log in", Desc: "Installs a user-level background service, no admin password needed", Value: "host:user"},
		{Label: "Keep running always, even before login", Desc: "Installs a system service — will ask for your admin password", Value: "host:system"},
		{Label: "Docker Compose", Desc: "Pulls container images instead of host binaries; needs Docker or Podman", Value: "compose:compose"},
	}, false)
	m.phase = phaseRuntimeKeep
}

func (m *Model) updateRuntimeKeep(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.runtimeMenu.HandleKey(km) {
		val := m.runtimeMenu.Selected()[0]
		switch val {
		case "host:once":
			m.answers.Runtime, m.answers.KeepMode = "host", "once"
		case "host:user":
			m.answers.Runtime, m.answers.KeepMode = "host", "user"
		case "host:system":
			m.answers.Runtime, m.answers.KeepMode = "host", "system"
		case "compose:compose":
			m.answers.Runtime, m.answers.KeepMode = "compose", "compose"
			if !prereqs.HaveCompose() && !prereqs.HaveDocker() {
				// We still let them proceed — the pipeline will surface a
				// clear error if Compose is unavailable at install time —
				// but flag it now so it's not a surprise later.
				m.runtimeMenu.Items[3].Desc = "Docker/Podman Compose not detected yet — install it before we start MuxCore, or pick another option"
			}
		}
		m.initLibraries()
	}
	return m, nil
}

func (m *Model) viewRuntimeKeep() string {
	return styleHint.Render("MuxCore stays as simple host processes by default. Pick how it should behave between reboots.") +
		"\n\n" + m.runtimeMenu.View()
}
