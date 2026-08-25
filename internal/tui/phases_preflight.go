package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/prereqs"
)

func (m *Model) portsToCheck() []struct{ port, name string } {
	ports := []struct{ port, name string }{
		{"8080", "MuxCore core"}, {"8082", "Admin UI"},
		{"9401", "Auth (HTTP)"}, {"9403", "Auth (gRPC)"},
		{"18080", "REST API"}, {"9203", "Health monitor"},
	}
	if m.answers.HasPlayback("MuxCore player") {
		ports = append(ports, struct{ port, name string }{"5173", "MuxCore player"})
	}
	return ports
}

// initPreflight checks (synchronously — a handful of fast local TCP dials)
// whether anything is already listening on the ports MuxCore needs, then
// shows the result. This intentionally does not use a spinner/async Cmd:
// it finishes in well under a second even in the worst case.
func (m *Model) initPreflight() {
	m.preflightBusy = nil
	for _, p := range m.portsToCheck() {
		if prereqs.PortInUse(p.port) {
			m.preflightBusy = append(m.preflightBusy, p.name+" (:"+p.port+")")
		}
	}
	// docker compose version succeeds without a running daemon, so the
	// runtime step's own check can miss a stopped daemon — catch it here
	// too, right before we'd otherwise fail deep inside `docker compose up`.
	m.preflightDockerBad = m.answers.Runtime == "compose" && !prereqs.HaveDocker()
	m.preflightDone = true
	m.phase = phasePreflight
}

func (m *Model) updatePreflightPhase(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "q":
		return m, quitCmd()
	case "r":
		if m.preflightDockerBad {
			m.initRuntimeKeep()
		}
		return m, nil
	case "enter", "c":
		m.initSummary()
	}
	return m, nil
}

func (m *Model) viewPreflight() string {
	if len(m.preflightBusy) == 0 && !m.preflightDockerBad {
		return styleOK.Render("✓ All the ports MuxCore needs are free.") + "\n\n" + styleHint.Render("enter continue")
	}
	var body string
	if m.preflightDockerBad {
		body += styleErr.Render("✗ Docker Compose is selected, but Docker's daemon isn't reachable.") + "\n"
		body += "  Start Docker (Docker Desktop, or `sudo systemctl start docker`), then continue —\n"
		body += "  or press " + styleFieldEdit.Render("r") + " to go back and run MuxCore as host processes instead.\n\n"
	}
	if len(m.preflightBusy) > 0 {
		body += styleWarn.Render("⚠ Something is already listening on:") + "\n"
		for _, b := range m.preflightBusy {
			body += "  • " + b + "\n"
		}
		body += "\n" + styleHint.Render("MuxCore may fail to start until that's freed up.") + "\n"
	}
	body += "\n" + styleHint.Render("enter continue anyway"+map[bool]string{true: "  ·  r change runtime", false: ""}[m.preflightDockerBad])
	return body
}
