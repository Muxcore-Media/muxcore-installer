package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
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
	note := styleHint.Render("Security: MuxCore's services talk to each other over TLS with their own certificate authority " +
		"(household profile). The admin and player pages are plain HTTP — keep them on your home network, or put an HTTPS " +
		"reverse proxy in front for anything else. An existing install that runs the dev profile stays dev until you re-run with --household.")
	if m.answers.RequestedProfile == mesh.ProfileDev {
		note = styleWarn.Render("Note: --dev — the mesh runs without TLS (dev profile). Development only, never for real data or the open internet.")
	}
	return styleHint.Render("Decide who can open the MuxCore admin and player pages.") + "\n\n" +
		m.networkMenu.View() + "\n\n" + note
}
