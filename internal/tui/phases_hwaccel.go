package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/prereqs"
)

func (m *Model) initHWAccel() {
	found := prereqs.DetectHWAccel()
	if len(found) == 0 {
		m.answers.HWAccelEnabled = false
		m.initPreflight()
		return
	}
	kind := found[0].Kind
	m.answers.HWAccelKind = kind
	m.hwMenu = NewMenu([]MenuItem{
		{Label: "Yes, use " + kind + " for transcoding", Desc: found[0].Note},
		{Label: "No, software transcoding only"},
	}, false)
	m.phase = phaseHWAccel
}

func (m *Model) updateHWAccel(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.hwMenu.HandleKey(km) {
		m.answers.HWAccelEnabled = m.hwMenu.Cursor == 0
		m.initPreflight()
	}
	return m, nil
}

func (m *Model) viewHWAccel() string {
	return styleHint.Render("We detected hardware video acceleration on this machine. Use it to transcode faster and cooler?") +
		"\n\n" + m.hwMenu.View()
}
