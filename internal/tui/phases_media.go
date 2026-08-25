package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/pathutil"
)

func (m *Model) initMediaRoot() {
	m.startPathPicker(pathutil.MediaDefault(), pathutil.MediaRecommended(),
		"Where should your media live? (each library gets its own folder underneath)",
		phaseMediaRoot, func(p string) {
			m.answers.MediaRoot = p
			m.rebuildMediaRows()
			m.phase = phaseMediaPaths
		})
}

var libKinds = []struct{ label, key, folder string }{
	{"Movies", "movies", "Movies"},
	{"TV", "tv", "TV"},
	{"Music", "music", "Music"},
	{"Books", "books", "Books"},
	{"Comics", "comics", "Comics"},
	{"Audiobooks", "audiobooks", "Audiobooks"},
}

// rebuildMediaRows re-derives every per-kind path under MediaRoot, keeping
// any path the user has already hand-edited (tracked simply by comparing to
// what auto-derivation would have produced last time — good enough for a
// wizard that only asks once).
func (m *Model) rebuildMediaRows() {
	var rows []mediaRow
	for _, k := range libKinds {
		if !m.answers.HasLibrary(k.label) {
			continue
		}
		path := m.answers.LibraryPaths[k.key]
		if path == "" {
			path = filepath.Join(m.answers.MediaRoot, k.folder)
		}
		m.answers.LibraryPaths[k.key] = path
		rows = append(rows, mediaRow{kind: k.key, label: k.label + " library", path: path, on: true})
	}
	if m.answers.ImportDir == "" {
		m.answers.ImportDir = filepath.Join(m.answers.MediaRoot, "Import")
	}
	rows = append(rows, mediaRow{kind: "import", label: "Watch folder (files you copy in yourself)", path: m.answers.ImportDir, on: m.answers.WatchFolder})
	rows = append(rows, mediaRow{kind: "_root", label: "Change media root…", path: m.answers.MediaRoot})
	rows = append(rows, mediaRow{kind: "_continue", label: "Continue"})
	m.mediaRows = rows
	m.mediaCursor = 0
}

func (m *Model) updateMediaRoot(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.updateInstallDir(msg)
}

func (m *Model) viewMediaRoot() string { return m.viewInstallDir() }

func (m *Model) updateMediaPaths(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.mediaEditing {
		switch km.String() {
		case "esc":
			m.mediaEditing = false
			return m, nil
		case "enter":
			val := pathutil.Resolve(m.mediaEditInput.Value())
			row := &m.mediaRows[m.mediaCursor]
			row.path = val
			if row.kind == "import" {
				m.answers.ImportDir = val
			} else {
				m.answers.LibraryPaths[row.kind] = val
			}
			m.mediaEditing = false
			return m, nil
		}
		var cmd tea.Cmd
		m.mediaEditInput, cmd = m.mediaEditInput.Update(km)
		return m, cmd
	}

	switch km.String() {
	case "q":
		return m, quitCmd()
	case "up", "k":
		if m.mediaCursor > 0 {
			m.mediaCursor--
		}
	case "down", "j":
		if m.mediaCursor < len(m.mediaRows)-1 {
			m.mediaCursor++
		}
	case "c":
		m.finishMediaPaths()
	case "enter":
		row := m.mediaRows[m.mediaCursor]
		switch row.kind {
		case "_root":
			m.initMediaRoot()
		case "_continue":
			m.finishMediaPaths()
		case "import":
			m.mediaRows[m.mediaCursor].on = !m.mediaRows[m.mediaCursor].on
			m.answers.WatchFolder = m.mediaRows[m.mediaCursor].on
		default:
			m.mediaEditInput = newTextInput(row.path, row.path, false)
			m.mediaEditInput.Focus()
			m.mediaEditing = true
		}
	case " ":
		row := m.mediaRows[m.mediaCursor]
		if row.kind == "import" {
			m.mediaRows[m.mediaCursor].on = !m.mediaRows[m.mediaCursor].on
			m.answers.WatchFolder = m.mediaRows[m.mediaCursor].on
		}
	}
	return m, nil
}

func (m *Model) finishMediaPaths() {
	// TV must not nest under the movie root, matching onboard.sh's guard.
	movies, tv := m.answers.LibraryPaths["movies"], m.answers.LibraryPaths["tv"]
	if movies != "" && tv != "" && (tv == movies || filepathHasPrefix(tv, movies)) {
		m.answers.LibraryPaths["tv"] = filepath.Join(m.answers.MediaRoot, "TV")
	}
	m.initPlayback()
}

func filepathHasPrefix(p, prefix string) bool {
	rel, err := filepath.Rel(prefix, p)
	return err == nil && rel != ".." && !hasDotDotPrefix(rel)
}
func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[:2] == ".."
}

func (m *Model) viewMediaPaths() string {
	var b string
	b += styleHint.Render("Review the folders below. Press enter to edit a row, space to toggle the watch folder.") + "\n\n"
	if m.mediaEditing {
		row := m.mediaRows[m.mediaCursor]
		return b + fmt.Sprintf("Editing %s:\n\n", row.label) + m.mediaEditInput.View() + "\n\n" + styleHint.Render("enter save · esc cancel")
	}
	for i, row := range m.mediaRows {
		cursor := "  "
		if i == m.mediaCursor {
			cursor = styleCursor.Render("> ")
		}
		switch row.kind {
		case "_root", "_continue":
			label := row.label
			if i == m.mediaCursor {
				label = styleSelected.Render(label)
			} else {
				label = styleOK.Render(label)
			}
			b += cursor + label + "\n"
			continue
		}
		box := ""
		if row.kind == "import" {
			if row.on {
				box = styleCheckOn.Render("[x] ")
			} else {
				box = styleCheckOff.Render("[ ] ")
			}
		}
		label := styleFieldLabel.Render(row.label)
		val := row.path
		if row.kind == "import" && !row.on {
			val = styleHint.Render("(disabled)")
		} else {
			val = styleFieldValue.Render(val)
		}
		line := cursor + box + label + " " + val
		b += line + "\n"
	}
	return b
}
