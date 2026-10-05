// Package tui implements the muxcore-setup Bubbletea wizard: an interactive
// question flow on the main pane, and — once the user confirms — a work
// phase that shows a live checklist/progress bar up top and a scrolling
// viewport of raw command output at the bottom.
package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/wizard"
)

type phase int

const (
	phaseLegal phase = iota
	phaseInstallDir
	phaseInstallDirBrowse
	phaseExistingChoice
	phaseRuntimeKeep
	phaseLibraries
	phaseMediaRoot
	phaseMediaPaths
	phasePlayback
	phasePlaybackCreds
	phaseNetwork
	phaseAdmin
	phaseMetadata
	phaseDatabase
	phaseHWAccel
	phasePreflight
	phaseSummary
	phaseWorking
	phaseDone
	phaseQuit
)

// totalSteps is used only for the "Step N/Total" badge; it is intentionally
// approximate (skipped conditional steps just make the number feel a little
// generous, which is harmless).
const totalSteps = 14

type Model struct {
	program *tea.Program
	answers wizard.Answers
	phase   phase
	err     error

	width, height int

	// legal
	legalVP   viewport.Model
	legalMenu *Menu
	legalRead bool // true once the user has scrolled the statement to the bottom

	// install dir / browse (also reused by media-root and per-kind path edits)
	pathMenu   *Menu
	dirBrowser *DirBrowser
	pathOnPick func(string) // called with the resolved absolute path
	pathReturn phase        // phase to resume once a path is picked
	pathPrompt string
	textInput  textinput.Model
	typingPath bool

	existingMenu *Menu

	runtimeMenu *Menu

	libMenu *Menu

	mediaRows      []mediaRow
	mediaCursor    int
	mediaEditing   bool
	mediaEditInput textinput.Model

	playbackMenu *Menu
	credQueue    []string
	credIdx      int
	credURL      textinput.Model
	credTok      textinput.Model
	credFocus    int
	credChecking bool
	credResult   string
	credSpinner  spinner.Model

	networkMenu *Menu

	adminUser   textinput.Model
	adminPass   textinput.Model
	adminFocus  int
	adminReveal bool
	adminGen    bool

	metaMenu      *Menu
	metaKeyInput  textinput.Model
	metaLangInput textinput.Model
	metaFocus     int
	metaChecking  bool
	metaResult    string
	metaSpinner   spinner.Model

	dbMenu     *Menu
	dbURLInput textinput.Model

	hwMenu *Menu

	preflightDone      bool
	preflightBusy      []string
	preflightDockerBad bool

	summaryConfirm bool

	work *workState

	quitting bool
}

func New(root string) *Model {
	m := &Model{
		answers: wizard.Default(root),
		phase:   phaseLegal,
		width:   100,
		height:  36,
	}
	m.initLegal()
	return m
}

// SetRequestedProfile records the operator's explicit security profile
// (--dev / --household / MUXCORE_PROFILE; "" = none).
func (m *Model) SetRequestedProfile(p string) { m.answers.RequestedProfile = p }

// SetProgram lets background goroutines (download/exec streaming) push
// tea.Msg values in from outside the normal Update loop.
func (m *Model) SetProgram(p *tea.Program) { m.program = p }

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	if m.phase == phaseWorking || m.phase == phaseDone {
		return m.viewWork()
	}
	body := m.viewBody()
	title := stepTitle(m.phase)
	head := header(stepNumber(m.phase), totalSteps, title)
	foot := styleFooter.Render(footerHint(m.phase))
	return "\n" + head + "\n\n" + body + "\n\n" + foot + "\n"
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.work != nil {
			m.work.resize(msg.Width, msg.Height)
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case workProgressMsg:
		return m.handleWorkProgress(msg)
	case workDoneMsg:
		return m.handleWorkDone(msg)
	case spinner.TickMsg:
		return m.routeSpinnerTick(msg)
	case credCheckMsg:
		return m.handleCredCheck(msg)
	case metaCheckMsg:
		return m.handleMetaCheck(msg)
	}

	switch m.phase {
	case phaseLegal:
		return m.updateLegal(msg)
	case phaseInstallDir:
		return m.updateInstallDir(msg)
	case phaseInstallDirBrowse:
		return m.updateInstallDirBrowse(msg)
	case phaseExistingChoice:
		return m.updateExistingChoice(msg)
	case phaseRuntimeKeep:
		return m.updateRuntimeKeep(msg)
	case phaseLibraries:
		return m.updateLibraries(msg)
	case phaseMediaRoot:
		return m.updateMediaRoot(msg)
	case phaseMediaPaths:
		return m.updateMediaPaths(msg)
	case phasePlayback:
		return m.updatePlayback(msg)
	case phasePlaybackCreds:
		return m.updatePlaybackCreds(msg)
	case phaseNetwork:
		return m.updateNetwork(msg)
	case phaseAdmin:
		return m.updateAdmin(msg)
	case phaseMetadata:
		return m.updateMetadata(msg)
	case phaseDatabase:
		return m.updateDatabase(msg)
	case phaseHWAccel:
		return m.updateHWAccel(msg)
	case phasePreflight:
		return m.updatePreflightPhase(msg)
	case phaseSummary:
		return m.updateSummary(msg)
	case phaseWorking:
		return m.updateWork(msg)
	case phaseDone:
		return m.updateDone(msg)
	}
	return m, nil
}

func (m *Model) viewBody() string {
	switch m.phase {
	case phaseLegal:
		return m.viewLegal()
	case phaseInstallDir:
		return m.viewInstallDir()
	case phaseInstallDirBrowse:
		return m.dirBrowser.View()
	case phaseExistingChoice:
		return m.viewExistingChoice()
	case phaseRuntimeKeep:
		return m.viewRuntimeKeep()
	case phaseLibraries:
		return m.viewLibraries()
	case phaseMediaRoot:
		return m.viewMediaRoot()
	case phaseMediaPaths:
		return m.viewMediaPaths()
	case phasePlayback:
		return m.viewPlayback()
	case phasePlaybackCreds:
		return m.viewPlaybackCreds()
	case phaseNetwork:
		return m.viewNetwork()
	case phaseAdmin:
		return m.viewAdmin()
	case phaseMetadata:
		return m.viewMetadata()
	case phaseDatabase:
		return m.viewDatabase()
	case phaseHWAccel:
		return m.viewHWAccel()
	case phasePreflight:
		return m.viewPreflight()
	case phaseSummary:
		return m.viewSummary()
	}
	return ""
}

func stepTitle(p phase) string {
	switch p {
	case phaseLegal:
		return "Before we start"
	case phaseInstallDir, phaseInstallDirBrowse:
		return "Install folder"
	case phaseExistingChoice:
		return "Existing install found"
	case phaseRuntimeKeep:
		return "How should MuxCore run?"
	case phaseLibraries:
		return "What do you keep?"
	case phaseMediaRoot:
		return "Where does your media live?"
	case phaseMediaPaths:
		return "Media layout"
	case phasePlayback:
		return "Watch and play"
	case phasePlaybackCreds:
		return "Connect your existing server"
	case phaseNetwork:
		return "Who can reach MuxCore?"
	case phaseAdmin:
		return "Your admin login"
	case phaseMetadata:
		return "Metadata"
	case phaseDatabase:
		return "Database"
	case phaseHWAccel:
		return "Hardware transcoding"
	case phasePreflight:
		return "Checking your system"
	case phaseSummary:
		return "Ready to install"
	}
	return "MuxCore setup"
}

func stepNumber(p phase) int {
	order := []phase{
		phaseLegal, phaseInstallDir, phaseRuntimeKeep, phaseLibraries, phaseMediaRoot,
		phaseMediaPaths, phasePlayback, phaseNetwork, phaseAdmin, phaseMetadata,
		phaseDatabase, phaseHWAccel, phasePreflight, phaseSummary,
	}
	for i, o := range order {
		if o == p {
			return i + 1
		}
	}
	return 0
}

func footerHint(p phase) string {
	switch p {
	case phaseLegal:
		return "↑/↓ scroll · y agree · n decline · q quit"
	case phaseMediaPaths:
		return "↑/↓ move · enter edit/toggle · c continue · q quit"
	case phaseAdmin:
		return "tab switch field · r reveal password · enter continue"
	case phaseSummary:
		return "enter install · e edit answers · q quit"
	default:
		return "↑/↓ move · enter select · space toggle (multi) · q quit"
	}
}

func quitCmd() tea.Cmd { return tea.Quit }

// mediaRow is one editable line in the consolidated media-layout screen.
type mediaRow struct {
	kind  string // movies, tv, music, books, comics, audiobooks, import
	label string
	path  string
	on    bool // for the optional import/watch row
}

func newTextInput(placeholder, value string, password bool) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = 4096
	ti.Width = 50
	if password {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}
	return ti
}

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleSpinner
	return s
}

func newProgressBar() progress.Model {
	return progress.New(progress.WithDefaultGradient())
}

// routeSpinnerTick advances whichever spinner is currently visible. Only one
// of these is ever active at a time since phases are mutually exclusive.
func (m *Model) routeSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case m.phase == phasePlaybackCreds && m.credChecking:
		m.credSpinner, cmd = m.credSpinner.Update(msg)
	case m.phase == phaseMetadata && m.metaChecking:
		m.metaSpinner, cmd = m.metaSpinner.Update(msg)
	case m.phase == phaseWorking && m.work != nil:
		m.work.spin, cmd = m.work.spin.Update(msg)
	}
	return m, cmd
}

func envTruthy(key string) bool {
	v := os.Getenv(key)
	return v == "1" || v == "true" || v == "TRUE"
}

var _ = fmt.Sprintf
var _ = os.Getenv
var _ = filepath.Join
