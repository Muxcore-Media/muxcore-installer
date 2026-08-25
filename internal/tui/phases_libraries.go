package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initLibraries() {
	m.libMenu = NewMenu([]MenuItem{
		{Label: "Movies"}, {Label: "TV"}, {Label: "Music"},
		{Label: "Books"}, {Label: "Comics"}, {Label: "Audiobooks"},
	}, true)
	m.libMenu.PreselectValues(m.answers.Libraries)
	m.phase = phaseLibraries
}

func (m *Model) updateLibraries(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.libMenu.HandleKey(km) {
		sel := m.libMenu.Selected()
		if len(sel) == 0 {
			sel = []string{"Movies", "TV"}
		}
		m.answers.Libraries = sel
		m.initMediaRoot()
	}
	return m, nil
}

func (m *Model) viewLibraries() string {
	return styleHint.Render("MuxCore organizes files you already have — pick every kind you keep.") +
		"\n\n" + m.libMenu.View()
}
