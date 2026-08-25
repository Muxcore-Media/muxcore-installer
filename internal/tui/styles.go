package tui

import "github.com/charmbracelet/lipgloss"

// Palette. Kept small and deliberate: one accent, one success, one danger,
// muted text for secondary copy.
var (
	colAccent  = lipgloss.Color("212") // MuxCore pink/magenta, matches old gum banner
	colAccent2 = lipgloss.Color("99")
	colGood    = lipgloss.Color("42")
	colWarn    = lipgloss.Color("214")
	colBad     = lipgloss.Color("203")
	colMuted   = lipgloss.Color("245")
	colDim     = lipgloss.Color("238")
	colFg      = lipgloss.Color("255")
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleSub   = lipgloss.NewStyle().Foreground(colMuted)
	styleHint  = lipgloss.NewStyle().Foreground(colDim).Italic(true)

	styleStepBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(colAccent).
			Padding(0, 1).
			Bold(true)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent2).
			Padding(1, 2)

	styleBoxGood = styleBox.BorderForeground(colGood)
	styleBoxBad  = styleBox.BorderForeground(colBad)

	styleCursor   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleSelected = lipgloss.NewStyle().Foreground(colFg).Bold(true)
	styleNormal   = lipgloss.NewStyle().Foreground(colMuted)
	styleCheckOn  = lipgloss.NewStyle().Foreground(colGood).Bold(true)
	styleCheckOff = lipgloss.NewStyle().Foreground(colDim)

	styleFieldLabel = lipgloss.NewStyle().Foreground(colMuted).Width(20)
	styleFieldValue = lipgloss.NewStyle().Foreground(colFg)
	styleFieldEdit  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	styleErr  = lipgloss.NewStyle().Foreground(colBad).Bold(true)
	styleWarn = lipgloss.NewStyle().Foreground(colWarn)
	styleOK   = lipgloss.NewStyle().Foreground(colGood)

	styleFooter = lipgloss.NewStyle().Foreground(colDim)

	styleLogLine       = lipgloss.NewStyle().Foreground(colMuted)
	styleLogErr        = lipgloss.NewStyle().Foreground(colBad)
	styleLogPane       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colDim)
	styleTopPane       = lipgloss.NewStyle()
	styleSpinner       = lipgloss.NewStyle().Foreground(colAccent)
	styleChecklistDone = lipgloss.NewStyle().Foreground(colGood)
	styleChecklistWait = lipgloss.NewStyle().Foreground(colDim)
	styleChecklistCur  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleChecklistErr  = lipgloss.NewStyle().Foreground(colBad).Bold(true)
)

func header(step, total int, title string) string {
	badge := styleStepBadge.Render(paddedStep(step, total))
	return badge + "  " + styleTitle.Render(title)
}

func paddedStep(step, total int) string {
	if step <= 0 {
		return "MuxCore"
	}
	return renderStep(step, total)
}

func renderStep(step, total int) string {
	return "Step " + itoa(step) + "/" + itoa(total)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
