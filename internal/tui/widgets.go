package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MenuItem is one row in a Menu.
type MenuItem struct {
	Label string
	Desc  string // optional secondary line
	Value string // machine value; defaults to Label if empty
}

// Menu is a small, self-contained single/multi choice list rendered with
// lipgloss instead of bubbles/list, so we control card-style rendering.
type Menu struct {
	Items   []MenuItem
	Cursor  int
	Multi   bool
	Checked map[int]bool

	// defVal/recVal are a small convenience used by the path-picker screens
	// (default/recommended/browse/type) so callers don't need a parallel map.
	defVal, recVal string
}

func NewMenu(items []MenuItem, multi bool) *Menu {
	return &Menu{Items: items, Multi: multi, Checked: map[int]bool{}}
}

// PreselectValues turns on Checked for items whose Value/Label matches.
func (m *Menu) PreselectValues(values []string) {
	for i, it := range m.Items {
		v := it.Value
		if v == "" {
			v = it.Label
		}
		for _, want := range values {
			if strings.EqualFold(v, want) {
				m.Checked[i] = true
			}
		}
	}
}

// HandleKey processes navigation/toggle keys. Returns true if Enter was
// pressed (caller decides what "confirm" means).
func (m *Menu) HandleKey(msg tea.KeyMsg) (confirmed bool) {
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Items)-1 {
			m.Cursor++
		}
	case " ":
		if m.Multi {
			m.Checked[m.Cursor] = !m.Checked[m.Cursor]
		}
	case "enter":
		if m.Multi {
			// Enter also toggles nothing; it's the confirm key for multi-selects.
		} else {
			m.Checked = map[int]bool{m.Cursor: true}
		}
		return true
	}
	return false
}

// Selected returns the chosen Values (multi) or the single chosen Value.
func (m *Menu) Selected() []string {
	var out []string
	for i, it := range m.Items {
		if m.Checked[i] {
			v := it.Value
			if v == "" {
				v = it.Label
			}
			out = append(out, v)
		}
	}
	return out
}

func (m *Menu) View() string {
	var b strings.Builder
	for i, it := range m.Items {
		cursor := "  "
		if i == m.Cursor {
			cursor = styleCursor.Render("> ")
		}
		var box string
		if m.Multi {
			if m.Checked[i] {
				box = styleCheckOn.Render("[x] ")
			} else {
				box = styleCheckOff.Render("[ ] ")
			}
		}
		label := it.Label
		if i == m.Cursor {
			label = styleSelected.Render(label)
		} else {
			label = styleNormal.Render(label)
		}
		b.WriteString(cursor + box + label + "\n")
		if it.Desc != "" {
			b.WriteString("      " + styleHint.Render(it.Desc) + "\n")
		}
	}
	return b.String()
}

// --- simple directory browser -------------------------------------------------

// DirBrowser lets the user walk the filesystem and pick a directory.
type DirBrowser struct {
	Dir     string
	entries []string
	cursor  int
}

func NewDirBrowser(start string) *DirBrowser {
	d := &DirBrowser{Dir: start}
	d.reload()
	return d
}

func (d *DirBrowser) reload() {
	d.entries = nil
	entries, err := os.ReadDir(d.Dir)
	if err == nil {
		var names []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		d.entries = names
	}
	d.cursor = 0
}

// Rows returns the render list: ".. (up)", "[Use this folder]", then subdirs.
func (d *DirBrowser) rows() []string {
	rows := []string{"[Use this folder: " + d.Dir + "]"}
	if filepath.Dir(d.Dir) != d.Dir {
		rows = append(rows, "..")
	}
	rows = append(rows, d.entries...)
	return rows
}

// HandleKey navigates; returns (selectedPath, true) when the user confirms
// "use this folder", or ("", false) otherwise.
func (d *DirBrowser) HandleKey(msg tea.KeyMsg) (string, bool) {
	rows := d.rows()
	switch msg.String() {
	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
		}
	case "down", "j":
		if d.cursor < len(rows)-1 {
			d.cursor++
		}
	case "enter":
		if d.cursor == 0 {
			return d.Dir, true
		}
		sel := rows[d.cursor]
		if sel == ".." {
			d.Dir = filepath.Dir(d.Dir)
		} else {
			d.Dir = filepath.Join(d.Dir, sel)
		}
		d.reload()
	}
	return "", false
}

func (d *DirBrowser) View() string {
	var b strings.Builder
	b.WriteString(styleHint.Render("Browsing: "+d.Dir) + "\n\n")
	for i, row := range d.rows() {
		cursor := "  "
		style := styleNormal
		if i == d.cursor {
			cursor = styleCursor.Render("> ")
			style = styleSelected
		}
		if i == 0 {
			style = styleOK
			if i == d.cursor {
				style = styleOK.Bold(true)
			}
		}
		b.WriteString(cursor + style.Render(row) + "\n")
	}
	return b.String()
}
