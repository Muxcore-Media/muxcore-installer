package tui

import (
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/legaltext"
)

func (m *Model) initLegal() {
	vp := viewport.New(74, 12)
	vp.SetContent(legaltext.Text())
	m.legalVP = vp
	m.legalMenu = NewMenu([]MenuItem{
		{Label: "I agree — I will only use MuxCore with media I have the right to use"},
		{Label: "I do not agree"},
	}, false)
	m.legalMenu.Cursor = 1 // default No, matches the old gum confirm default
	m.phase = phaseLegal
}

func (m *Model) updateLegal(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q":
		return m, quitCmd()
	case "y":
		m.acceptLegal()
		return m, nil
	case "n":
		m.quitting = true
		return m, quitCmd()
	case "up", "down", "k", "j":
		var cmd tea.Cmd
		m.legalVP, cmd = m.legalVP.Update(km)
		_ = cmd
		m.legalMenu.HandleKey(km)
		return m, nil
	case "enter":
		if m.legalMenu.Cursor == 0 {
			m.acceptLegal()
		} else {
			m.quitting = true
			return m, quitCmd()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) acceptLegal() {
	m.answers.Agreed = true
	m.initInstallDir()
}

// recordLegalAcceptance writes proof-of-consent into the final install root,
// once one has been chosen (see afterInstallDir in phases_installdir.go).
func recordLegalAcceptance(root string) {
	out := filepath.Join(root, "data", "legal-accepted.txt")
	os.MkdirAll(filepath.Dir(out), 0o755)
	body := "accepted_at=" + time.Now().UTC().Format(time.RFC3339) + "\n" +
		"tos_sha256=" + legaltext.SHA256() + "\n"
	os.WriteFile(out, []byte(body), 0o644)
}

func (m *Model) viewLegal() string {
	box := styleBox.Width(78).Render(m.legalVP.View())
	return box + "\n\n" + m.legalMenu.View()
}
