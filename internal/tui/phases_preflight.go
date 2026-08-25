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
	case "enter", "c":
		m.initSummary()
	}
	return m, nil
}

func (m *Model) viewPreflight() string {
	if len(m.preflightBusy) == 0 {
		return styleOK.Render("✓ All the ports MuxCore needs are free.") + "\n\n" + styleHint.Render("enter continue")
	}
	body := styleWarn.Render("⚠ Something is already listening on:") + "\n"
	for _, b := range m.preflightBusy {
		body += "  • " + b + "\n"
	}
	body += "\n" + styleHint.Render("MuxCore may fail to start until that's freed up. You can continue anyway and fix it after — enter continue")
	return body
}
