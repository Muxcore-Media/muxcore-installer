package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/pathutil"
)

// startPathPicker is the reusable "default / recommended / browse / type"
// flow used for the install dir, media root, and every per-library path edit.
func (m *Model) startPathPicker(def, rec, prompt string, returnTo phase, onPick func(string)) {
	m.pathPrompt = prompt
	m.pathReturn = returnTo
	m.pathOnPick = onPick
	items := []MenuItem{{Label: "Use " + def, Value: "default"}}
	if rec != "" && rec != def {
		items = append(items, MenuItem{Label: "Use " + rec, Value: "recommended"})
	}
	items = append(items,
		MenuItem{Label: "Browse folders", Value: "browse"},
		MenuItem{Label: "Type a path", Value: "type"},
	)
	m.pathMenu = NewMenu(items, false)
	m.pathMenu.defVal, m.pathMenu.recVal = def, rec
	m.phase = phaseInstallDir
}

func (m *Model) initInstallDir() {
	root := m.answers.Root
	m.startPathPicker(pathutil.InstallDefault(), pathutil.InstallRecommended(),
		"Where should MuxCore be installed?", phaseInstallDir, func(p string) {
			m.answers.Root = p
			m.afterInstallDir()
		})
	_ = root
}

func (m *Model) afterInstallDir() {
	if err := pathutil.EnsureWritableDir(m.answers.Root); err != nil {
		m.err = err
	}
	recordLegalAcceptance(m.answers.Root)
	envPath := filepath.Join(m.answers.Root, ".env")
	if info, err := os.Stat(envPath); err == nil && !info.IsDir() {
		m.answers.ExistingFound = true
		m.initExistingChoice()
		return
	}
	m.initRuntimeKeep()
}

func (m *Model) updateInstallDir(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.pathMenu.HandleKey(km) {
		switch m.pathMenu.Selected()[0] {
		case "default":
			m.pathOnPick(pathutil.Resolve(m.pathMenu.defVal))
		case "recommended":
			m.pathOnPick(pathutil.Resolve(m.pathMenu.recVal))
		case "browse":
			start, _ := os.UserHomeDir()
			if d := pathutil.Resolve(m.pathMenu.defVal); d != "" {
				if p := filepath.Dir(d); dirExists(p) {
					start = p
				}
			}
			m.dirBrowser = NewDirBrowser(start)
			m.phase = phaseInstallDirBrowse
		case "type":
			m.textInput = newTextInput(m.pathMenu.defVal, m.pathMenu.defVal, false)
			m.textInput.Focus()
			m.typingPath = true
			m.phase = phaseInstallDirBrowse // reuse this phase id for "typing" sub-mode too
		}
	}
	return m, nil
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func (m *Model) updateInstallDirBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.typingPath {
		switch km.String() {
		case "esc":
			m.typingPath = false
			m.phase = phaseInstallDir
			return m, nil
		case "enter":
			p := pathutil.Resolve(m.textInput.Value())
			m.typingPath = false
			m.pathOnPick(p)
			return m, nil
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(km)
		return m, cmd
	}
	if km.String() == "esc" {
		m.phase = phaseInstallDir
		return m, nil
	}
	if p, ok := m.dirBrowser.HandleKey(km); ok {
		m.pathOnPick(pathutil.Resolve(p))
	}
	return m, nil
}

func (m *Model) viewInstallDir() string {
	if m.typingPath {
		return styleHint.Render(m.pathPrompt) + "\n\n" + m.textInput.View() + "\n\n" + styleHint.Render("enter confirm · esc back")
	}
	return styleHint.Render(m.pathPrompt) + "\n\n" + m.pathMenu.View()
}

// ---- existing install detection ----

func (m *Model) initExistingChoice() {
	m.existingMenu = NewMenu([]MenuItem{
		{Label: "Reconfigure", Desc: "Walk through every question again, keep existing data"},
		{Label: "Just update and restart", Desc: "Skip the wizard, re-download binaries, restart the stack"},
		{Label: "Start fresh", Desc: "Wipe data/ in this folder and begin a brand-new install"},
	}, false)
	m.phase = phaseExistingChoice
}

func (m *Model) updateExistingChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "q" {
		return m, quitCmd()
	}
	if m.existingMenu.HandleKey(km) {
		switch m.existingMenu.Cursor {
		case 0:
			m.answers.ExistingChoice = "reconfigure"
			m.initRuntimeKeep()
		case 1:
			m.answers.ExistingChoice = "restart-only"
			m.initSummary()
		case 2:
			m.answers.ExistingChoice = "fresh"
			os.RemoveAll(filepath.Join(m.answers.Root, "data"))
			os.Remove(filepath.Join(m.answers.Root, ".env"))
			m.initRuntimeKeep()
		}
	}
	return m, nil
}

func (m *Model) viewExistingChoice() string {
	return styleHint.Render("Found an existing install at "+m.answers.Root) + "\n\n" + m.existingMenu.View()
}
