package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Sadoaz/vimyt/internal/model"
)

var (
	nowPlayingStyle = lipgloss.NewStyle().
			Padding(0, 1)
	npTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))
	npArtistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("247"))
	npTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
	npPausedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))
)

var npSettingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

func renderNowPlaying(status model.PlayerStatus, width int, favSet map[string]bool, autoplay bool, shuffle bool, loopTrack bool, loopTotal int, tick int) string {
	if status.Track == nil {
		line := "  Nothing playing"
		return nowPlayingStyle.Width(width).MaxWidth(width).Render(line)
	}

	t := status.Track
	stateStr := ""
	if favSet[t.ID] {
		stateStr = favStyle.Render(" <3")
	}
	if status.State == model.Paused {
		stateStr += npPausedStyle.Render(" ||")
	}

	if shuffle {
		stateStr += npSettingStyle.Render(" [S]")
	}
	if autoplay {
		stateStr += npSettingStyle.Render(" [A]")
	}
	if loopTrack {
		if loopTotal == 0 {
			stateStr += npSettingStyle.Render(" [L∞]")
		} else {
			stateStr += npSettingStyle.Render(fmt.Sprintf(" [L%d]", loopTotal))
		}
	}

	pos := formatDuration(status.Position)
	dur := formatDuration(t.Duration)
	timeStr := npTimeStyle.Render(fmt.Sprintf("%s / %s", pos, dur))

	stateW := lipgloss.Width(stateStr)
	timeW := lipgloss.Width(timeStr)
	minBarW := 10
	padding := 2 // from nowPlayingStyle Padding(0,1)

	// Fixed parts: "  " + textPart + "  " + timeStr + "  " + bar + stateStr + padding
	fixedW := 2 + 2 + timeW + 2 + minBarW + stateW + padding

	// Available width for title + "  " + artist
	textMax := width - fixedW
	textMax = max(textMax, 10)

	// Combine title and artist into one styled string, then marquee if needed
	textPart := npTitleStyle.Render(t.Title) + "  " + npArtistStyle.Render(t.Artist)
	textPart = marquee(textPart, textMax, tick)

	prefix := fmt.Sprintf("  %s  %s  ",
		textPart,
		timeStr,
	)
	prefixW := lipgloss.Width(prefix)

	// Progress bar fills whatever space remains
	barWidth := width - prefixW - stateW - padding
	barWidth = max(barWidth, minBarW)
	var progress float64
	if t.Duration > 0 {
		progress = float64(status.Position) / float64(t.Duration)
	}
	progress = min(progress, 1)
	filled := int(progress * float64(barWidth))
	barFilledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.BarFilled))
	barEmptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.BarEmpty))
	bar := barFilledStyle.Render(strings.Repeat("━", filled)) + barEmptyStyle.Render(strings.Repeat("─", barWidth-filled))

	line := prefix + bar + stateStr

	return ansi.Truncate(nowPlayingStyle.Width(width).MaxWidth(width).Render(line), width, "")
}

// renderInputWithNowPlaying shows a text input on the left and abbreviated now-playing on the right.
func renderInputWithNowPlaying(inputView string, status model.PlayerStatus, width int, autoplay bool, shuffle bool, loopTrack bool, loopTotal int) string {
	if status.Track == nil {
		// No track — just show the input full-width
		return nowPlayingStyle.Width(width).MaxWidth(width).Render(inputView)
	}
	t := status.Track
	pos := formatDuration(status.Position)
	dur := formatDuration(t.Duration)

	stateIcons := ""
	if status.State == model.Paused {
		stateIcons += npPausedStyle.Render(" ||")
	}
	if shuffle {
		stateIcons += npSettingStyle.Render(" [S]")
	}
	if autoplay {
		stateIcons += npSettingStyle.Render(" [A]")
	}
	if loopTrack {
		if loopTotal == 0 {
			stateIcons += npSettingStyle.Render(" [L∞]")
		} else {
			stateIcons += npSettingStyle.Render(fmt.Sprintf(" [L%d]", loopTotal))
		}
	}

	inputW := lipgloss.Width(inputView)
	iconsW := lipgloss.Width(stateIcons)
	timeRendered := npTimeStyle.Render(pos + " / " + dur)
	timeW := lipgloss.Width(timeRendered)

	// Available width for title on the right side
	// Layout: input + gap(1+) + "  " + title + "  " + time + icons + padding(2)
	titleMax := width - inputW - 1 - 2 - 2 - timeW - iconsW - 2
	titleMax = max(titleMax, 3)
	titleRendered := npTitleStyle.Render(ansi.Truncate(t.Title, titleMax, "…"))

	rightPart := fmt.Sprintf("  %s  %s%s",
		titleRendered,
		timeRendered,
		stateIcons,
	)

	rightW := lipgloss.Width(rightPart)
	gap := width - inputW - rightW - 2
	gap = max(gap, 1)
	line := inputView + strings.Repeat(" ", gap) + rightPart

	return ansi.Truncate(nowPlayingStyle.Width(width).MaxWidth(width).Render(line), width, "")
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
