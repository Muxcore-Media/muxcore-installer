package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Muxcore-Media/muxcore-installer/internal/wizard"
)

type stepState int

const (
	stepPending stepState = iota
	stepActive
	stepDone
	stepWarn
	stepError
)

type workState struct {
	root       string
	dryRun     bool
	states     map[wizard.StepID]stepState
	current    wizard.StepID
	spin       spinner.Model
	bar        progress.Model
	barPct     float64
	barActive  bool
	barLabel   string
	vp         viewport.Model
	lines      []string
	fatalErr   error
	finished   bool
	successMsg string
}

func newWorkState(root string, dryRun bool, width, height int) *workState {
	w := &workState{
		root:   root,
		dryRun: dryRun,
		states: map[wizard.StepID]stepState{},
		spin:   newSpinner(),
		bar:    newProgressBar(),
	}
	for _, s := range wizard.Steps {
		w.states[s.ID] = stepPending
	}
	w.resize(width, height)
	return w
}

func (w *workState) resize(width, height int) {
	vpHeight := height - len(wizard.Steps) - 10
	if vpHeight < 6 {
		vpHeight = 6
	}
	vpWidth := width - 6
	if vpWidth < 20 {
		vpWidth = 60
	}
	w.vp = viewport.New(vpWidth, vpHeight)
	w.vp.SetContent(strings.Join(w.lines, "\n"))
	w.vp.GotoBottom()
	w.bar.Width = vpWidth - 10
	if w.bar.Width < 10 {
		w.bar.Width = 10
	}
}

func (w *workState) appendLine(kind wizard.StepKind, text string) {
	if text == "" {
		return
	}
	// Long lines (docker/compose errors especially) are common here — wrap
	// them to the pane width instead of letting them overflow, which used
	// to make the bordered box's own width calculation misbehave.
	if w.vp.Width > 0 {
		text = lipgloss.NewStyle().Width(w.vp.Width).Render(text)
	}
	styled := styleLogLine.Render(text)
	switch kind {
	case wizard.KindWarn:
		styled = styleWarn.Render(text)
	case wizard.KindError:
		styled = styleLogErr.Render(text)
	}
	w.lines = append(w.lines, styled)
	const maxLines = 2000
	if len(w.lines) > maxLines {
		w.lines = w.lines[len(w.lines)-maxLines:]
	}
	w.vp.SetContent(strings.Join(w.lines, "\n"))
	w.vp.GotoBottom()
}

// workProgressMsg wraps wizard.Progress so it has its own bubbletea message type.
type workProgressMsg wizard.Progress

// workDoneMsg reports pipeline completion.
type workDoneMsg struct{ err error }

func (m *Model) initSummary() {
	m.summaryConfirm = true
	m.phase = phaseSummary
}

func (m *Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q":
		return m, quitCmd()
	case "e":
		m.initRuntimeKeep()
		return m, nil
	case "enter":
		return m, m.startWork()
	}
	return m, nil
}

func (m *Model) startWork() tea.Cmd {
	m.work = newWorkState(m.answers.Root, dryRunEnv(), m.width, m.height)
	m.phase = phaseWorking
	go m.runPipeline()
	return nil
}

func (m *Model) runPipeline() {
	pipe := &wizard.Pipeline{
		Answers: &m.answers,
		DryRun:  m.work.dryRun,
		Emit: func(p wizard.Progress) {
			if m.program != nil {
				m.program.Send(workProgressMsg(p))
			}
		},
	}
	err := pipe.Run(context.Background())
	if m.program != nil {
		m.program.Send(workDoneMsg{err: err})
	}
}

func (m *Model) handleWorkProgress(msg workProgressMsg) (tea.Model, tea.Cmd) {
	w := m.work
	if w == nil {
		return m, nil
	}
	switch msg.Kind {
	case wizard.KindStart:
		w.states[msg.Step] = stepActive
		w.current = msg.Step
		if msg.Step != "fetch" {
			w.barActive = false
		}
	case wizard.KindProgress:
		w.barActive = true
		w.barLabel = msg.Text
		if msg.Total > 0 {
			w.barPct = float64(msg.Read) / float64(msg.Total)
		} else {
			w.barPct = 0
		}
		return m, nil
	case wizard.KindLine:
		w.appendLine(msg.Kind, msg.Text)
	case wizard.KindWarn:
		w.appendLine(msg.Kind, msg.Text)
		if w.states[msg.Step] == stepActive {
			w.states[msg.Step] = stepWarn
		}
	case wizard.KindError:
		w.appendLine(msg.Kind, msg.Text)
		w.states[msg.Step] = stepError
		w.fatalErr = errors.New(msg.Text)
	case wizard.KindDone:
		if w.states[msg.Step] != stepWarn && w.states[msg.Step] != stepError {
			w.states[msg.Step] = stepDone
		}
		w.barActive = false
		if msg.Text != "" {
			w.appendLine(wizard.KindLine, msg.Text)
		}
	}
	return m, nil
}

func (m *Model) handleWorkDone(msg workDoneMsg) (tea.Model, tea.Cmd) {
	if m.work == nil {
		return m, nil
	}
	m.work.finished = true
	if msg.err != nil {
		m.work.fatalErr = msg.err
	} else {
		m.work.successMsg = "MuxCore is ready."
		m.phase = phaseDone
	}
	return m, nil
}

func dryRunEnv() bool {
	return envTruthy("MUXCORE_DRY_RUN")
}

func (m *Model) updateWork(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	// Install finished (success or failure) — the viewport hint says "q to quit".
	if km.String() == "q" && m.work != nil && m.work.finished {
		m.quitting = true
		return m, quitCmd()
	}
	return m, nil
}

func (m *Model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q", "enter":
		m.quitting = true
		return m, quitCmd()
	}
	return m, nil
}

func (m *Model) viewSummary() string {
	a := m.answers
	line := func(label, val string) string {
		return styleFieldLabel.Render(label) + " " + styleFieldValue.Render(val) + "\n"
	}
	var b strings.Builder
	b.WriteString(line("Install folder:", a.Root))
	b.WriteString(line("Runtime:", runtimeDesc(a.Runtime, a.KeepMode)))
	b.WriteString(line("Libraries:", strings.Join(a.Libraries, ", ")))
	if len(a.Playback) > 0 {
		b.WriteString(line("Playback:", strings.Join(a.Playback, ", ")))
	}
	b.WriteString(line("Network:", map[bool]string{true: "whole network (0.0.0.0)", false: "this machine only (127.0.0.1)"}[a.BindAllInterfaces]))
	b.WriteString(line("Admin user:", a.AdminUser))
	metaDesc := "offline sample titles"
	if !a.TMDBFixture {
		metaDesc = "TMDB API key"
		if a.TMDBValidated {
			metaDesc += " (validated)"
		}
	}
	b.WriteString(line("Metadata:", metaDesc+", "+a.MetadataLang))
	b.WriteString(line("Database:", a.DBBackend))
	if a.HWAccelEnabled {
		b.WriteString(line("Hardware accel:", a.HWAccelKind))
	}
	if dryRunEnv() {
		b.WriteString("\n" + styleWarn.Render("MUXCORE_DRY_RUN=1 — this run will stop before downloading or starting anything.") + "\n")
	}
	box := styleBox.Width(70).Render(b.String())
	return box
}

func runtimeDesc(runtime, keep string) string {
	if runtime == "compose" {
		return "Docker Compose"
	}
	switch keep {
	case "user":
		return "host processes, user background service"
	case "system":
		return "host processes, system service (sudo)"
	default:
		return "host processes, run once"
	}
}

func (m *Model) viewWork() string {
	w := m.work
	if w == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n" + styleTitle.Render("Installing MuxCore") + "\n\n")
	for _, s := range wizard.Steps {
		state := w.states[s.ID]
		var mark, label string
		switch state {
		case stepDone:
			mark, label = "✓", styleChecklistDone.Render(s.Label)
		case stepWarn:
			mark, label = "!", styleWarn.Render(s.Label)
		case stepError:
			mark, label = "✗", styleChecklistErr.Render(s.Label)
		case stepActive:
			mark, label = w.spin.View(), styleChecklistCur.Render(s.Label)
		default:
			mark, label = "·", styleChecklistWait.Render(s.Label)
		}
		b.WriteString("  " + mark + " " + label + "\n")
		if state == stepActive && s.ID == "fetch" && w.barActive {
			label := w.barLabel
			if label == "" {
				label = "…"
			}
			b.WriteString("    " + styleHint.Render(label) + "\n")
			b.WriteString("    " + w.bar.ViewAs(w.barPct) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styleLogPane.Width(w.vp.Width + 2).Render(w.vp.View()))
	b.WriteString("\n\n")
	if w.fatalErr != nil {
		b.WriteString(styleErr.Render("Something went wrong: "+w.fatalErr.Error()) + "\n")
		b.WriteString(styleHint.Render("q to quit — check the log above, or the .log files under run/"))
	} else if m.phase == phaseDone {
		b.WriteString(m.viewDone())
	} else {
		b.WriteString(styleHint.Render("Sit tight — this can take a minute the first time…  ctrl+c to cancel"))
	}
	return b.String()
}

func (m *Model) viewDone() string {
	a := m.answers
	body := fmt.Sprintf(
		"Admin UI:  http://localhost:8082\n  login:   %s / %s\n\nCore health: http://127.0.0.1:8080/health\nCredentials: %s/run/VIEW-ME.txt\nAdmin token: %s/run/admin.token\n\nSmoke:     ./smoke-fixture.sh\nTip:       cat %s/run/VIEW-ME.txt for URLs, libraries, and commands",
		a.AdminUser, a.AdminPass, a.Root, a.Root, a.Root,
	)
	if a.HasPlayback("MuxCore player") {
		body = "Player:    http://127.0.0.1:5173\n" + body
	}
	box := styleBoxGood.Width(64).Render(styleOK.Render("MuxCore is ready!") + "\n\n" + body)
	return box + "\n\n" + styleHint.Render("q to exit the installer")
}
