package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initNetwork() {
	m.networkMenu = NewMenu([]MenuItem{
		{Label: "Just this machine", Desc: "Binds to 127.0.0.1 — safest default, use a browser on this computer"},
		{Label: "My whole home network", Desc: "Binds to 0.0.0.0 so phones/TVs on your LAN can reach it too"},
	}, false)
	m.phase = phaseNetwork
}

func (m *Model) updateNetwork(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.networkMenu.HandleKey(km) {
		m.answers.BindAllInterfaces = m.networkMenu.Cursor == 1
		m.initAdmin()
	}
	return m, nil
}

func (m *Model) viewNetwork() string {
	note := styleWarn.Render("Note: this installer runs with TLS disabled (MUXCORE_INSECURE_DISABLE_TLS) — fine on a trusted home network, not for the open internet.")
	return styleHint.Render("Decide who can open the MuxCore admin and player pages.") + "\n\n" +
		m.networkMenu.View() + "\n\n" + note
}
