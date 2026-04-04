package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpLines builds the full list of help lines.
func helpLines(filter string) []string {
	sections := helpSections()
	filterLower := strings.ToLower(filter)
	words := strings.Fields(filterLower)
	var lines []string
	for _, sec := range sections {
		var sectionLines []string
		for _, e := range sec.entries {
			if len(words) > 0 {
				combined := strings.ToLower(e.key + " " + e.action + " " + e.desc)
				match := true
				for _, w := range words {
					if !strings.Contains(combined, w) {
						match = false
						break
					}
				}
				if !match {
					continue
				}
			}
			sectionLines = append(sectionLines, e.key+"\t"+e.action+"\t"+e.desc)
		}
		if len(sectionLines) > 0 {
			lines = append(lines, "\t"+sec.title+"\t") // section header marker
			lines = append(lines, sectionLines...)
		}
	}
	return lines
}

var (
	helpKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	helpHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	helpDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpDescStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

func (a App) renderHelp() string {
	lines := helpLines(a.helpFilter)

	// Available height for content inside border (border=2, title=1, hint=2, footer=2, padding=2)
	maxH := max(a.height-10, 5)

	// Clamp scroll
	scroll := min(a.helpScroll, len(lines)-maxH)
	scroll = max(scroll, 0)

	end := min(scroll+maxH, len(lines))
	visible := lines[scroll:end]

	var b strings.Builder

	// Hint bar
	if a.helpFiltering {
		b.WriteString("  " + a.helpFilterInput.View())
	} else if a.helpFilter != "" {
		b.WriteString(helpDimStyle.Render(fmt.Sprintf("  j/k: move  /: search  Esc: clear filter  ?: close    filter: %s", a.helpFilter)))
	} else {
		b.WriteString(helpDimStyle.Render("  j/k: move  /: search  ctrl+d/u: faster scroll  ?: close"))
	}
	b.WriteString("\n\n")

	// Render lines
	for _, line := range visible {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 3 && parts[0] == "" && parts[2] == "" {
			// Section header
			b.WriteString("  " + helpHeaderStyle.Render(parts[1]) + "\n")
		} else if len(parts) == 3 {
			fmt.Fprintf(&b, "  * %-18s %-24s %s\n",
				helpKeyStyle.Render(parts[0]),
				parts[1],
				helpDescStyle.Render(parts[2]),
			)
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	// Scroll indicator
	if len(lines) > maxH {
		fmt.Fprintf(&b, "\n  %s", helpDimStyle.Render(fmt.Sprintf("More rows below (%d/%d)", end, len(lines))))
	}

	boxW := max(a.width-4, 50)
	box := overlayBorderStyle.Width(boxW).Render(
		overlayTitleStyle.Render("Help") + "\n" + b.String(),
	)
	return lipgloss.Place(a.width, a.height-1, lipgloss.Center, lipgloss.Center, box)
}

func (a App) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If filter input is active, handle text input
	if a.helpFiltering {
		switch {
		case key.Matches(msg, keys.Enter):
			a.helpFilter = a.helpFilterInput.Value()
			a.helpFiltering = false
			a.helpFilterInput.Blur()
			a.helpScroll = 0
			return a, nil
		case key.Matches(msg, keys.Escape):
			// If there's text, clear it. If empty, close filter.
			if a.helpFilterInput.Value() != "" {
				a.helpFilterInput.SetValue("")
				a.helpFilter = ""
				a.helpScroll = 0
			} else {
				a.helpFiltering = false
				a.helpFilterInput.Blur()
			}
			return a, nil
		default:
			var cmd tea.Cmd
			a.helpFilterInput, cmd = a.helpFilterInput.Update(msg)
			a.helpFilter = a.helpFilterInput.Value()
			a.helpScroll = 0
			return a, cmd
		}
	}

	lines := helpLines(a.helpFilter)
	maxH := max(a.height-10, 5)
	maxScroll := max(len(lines)-maxH, 0)

	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case msg.String() == "q", key.Matches(msg, keys.Escape), key.Matches(msg, keys.Help):
		a.showHelp = false
		a.helpScroll = 0
		a.helpFilter = ""
		a.helpFiltering = false
		return a, nil
	case msg.String() == "j", key.Matches(msg, keys.Down):
		if a.helpScroll < maxScroll {
			a.helpScroll++
		}
		return a, nil
	case msg.String() == "k", key.Matches(msg, keys.Up):
		if a.helpScroll > 0 {
			a.helpScroll--
		}
		return a, nil
	case key.Matches(msg, keys.HalfDown):
		a.helpScroll = min(a.helpScroll+maxH/2, maxScroll)
		return a, nil
	case key.Matches(msg, keys.HalfUp):
		a.helpScroll = max(a.helpScroll-maxH/2, 0)
		return a, nil
	case msg.String() == "g":
		a.helpScroll = 0
		return a, nil
	case msg.String() == "G":
		a.helpScroll = maxScroll
		return a, nil
	case msg.String() == "/":
		a.helpFiltering = true
		a.helpFilterInput.SetValue(a.helpFilter)
		a.helpFilterInput.Focus()
		return a, nil
	}
	return a, nil
}
