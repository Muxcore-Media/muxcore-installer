package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initDatabase() {
	m.dbMenu = NewMenu([]MenuItem{
		{Label: "SQLite", Desc: "Recommended for a single machine — zero extra setup"},
		{Label: "Postgres", Desc: "Bring your own DATABASE_URL, or let us start a local Docker container"},
	}, false)
	if m.answers.DBBackend == "postgres" {
		m.dbMenu.Cursor = 1
	}
	m.dbURLInput = newTextInput("DATABASE_URL (blank = local Docker postgres:16-alpine)", m.answers.DatabaseURL, false)
	m.phase = phaseDatabase
}

func (m *Model) updateDatabase(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.dbMenu.Cursor == 1 && m.dbURLInput.Focused() {
		switch km.String() {
		case "esc":
			m.dbURLInput.Blur()
			return m, nil
		case "enter":
			m.answers.DBBackend = "postgres"
			m.answers.DatabaseURL = m.dbURLInput.Value()
			m.afterDatabase()
			return m, nil
		}
		var cmd tea.Cmd
		m.dbURLInput, cmd = m.dbURLInput.Update(km)
		return m, cmd
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.dbMenu.HandleKey(km) {
		if m.dbMenu.Cursor == 0 {
			m.answers.DBBackend = "sqlite"
			m.afterDatabase()
			return m, nil
		}
		m.answers.DBBackend = "postgres"
		m.dbURLInput.Focus()
	}
	return m, nil
}

func (m *Model) afterDatabase() {
	if m.answers.HasPlayback("MuxCore player") {
		m.initHWAccel()
	} else {
		m.initPreflight()
	}
}

func (m *Model) viewDatabase() string {
	body := styleHint.Render("Where should MuxCore keep its own data?") + "\n\n" + m.dbMenu.View()
	if m.dbMenu.Cursor == 1 {
		body += "\n" + styleFieldLabel.Render("DATABASE_URL") + " " + m.dbURLInput.View() +
			"\n\n" + styleHint.Render("enter blank to use a local Docker container · enter to continue")
	}
	return body
}
