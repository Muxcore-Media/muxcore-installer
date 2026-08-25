package tui

import (
	"crypto/rand"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const genPasswordChars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

func genPassword(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	out := make([]byte, n)
	for i, v := range b {
		out[i] = genPasswordChars[int(v)%len(genPasswordChars)]
	}
	return string(out)
}

func (m *Model) initAdmin() {
	m.adminUser = newTextInput("admin", m.answers.AdminUser, false)
	pass := genPassword(16)
	m.adminPass = newTextInput("", pass, true)
	m.answers.AdminPass = pass
	m.adminGen = true
	m.adminReveal = false
	m.adminFocus = 0
	m.adminUser.Focus()
	m.phase = phaseAdmin
}

func (m *Model) updateAdmin(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "tab", "down", "up":
		m.adminFocus = 1 - m.adminFocus
		if m.adminFocus == 0 {
			m.adminUser.Focus()
			m.adminPass.Blur()
		} else {
			m.adminPass.Focus()
			m.adminUser.Blur()
		}
		return m, nil
	case "r":
		if m.adminFocus == 1 {
			m.adminReveal = !m.adminReveal
			if m.adminReveal {
				m.adminPass.EchoMode = textinput.EchoNormal
			} else {
				m.adminPass.EchoMode = textinput.EchoPassword
			}
			return m, nil
		}
	case "ctrl+g":
		if m.adminFocus == 1 {
			m.adminPass.SetValue(genPassword(16))
			m.adminGen = true
			return m, nil
		}
	case "enter":
		user := m.adminUser.Value()
		if user == "" {
			user = "admin"
		}
		m.answers.AdminUser = user
		m.answers.AdminPass = m.adminPass.Value()
		if m.answers.AdminPass == "" {
			m.answers.AdminPass = genPassword(16)
		}
		m.initMetadata()
		return m, nil
	}
	var cmd tea.Cmd
	if m.adminFocus == 0 {
		m.adminUser, cmd = m.adminUser.Update(km)
	} else {
		m.adminPass, cmd = m.adminPass.Update(km)
		m.adminGen = false
	}
	return m, cmd
}

func (m *Model) viewAdmin() string {
	reveal := "hidden"
	if m.adminReveal {
		reveal = "visible"
	}
	genNote := ""
	if m.adminGen {
		genNote = styleHint.Render("  (generated for you — ctrl+g for a new one)")
	}
	return styleFieldLabel.Render("Username") + " " + m.adminUser.View() + "\n\n" +
		styleFieldLabel.Render("Password ("+reveal+")") + " " + m.adminPass.View() + genNote + "\n\n" +
		styleHint.Render("tab switch field · r toggle reveal · ctrl+g generate new password · enter continue")
}
