package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Bottom bar background style
	sbBgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
)

// Panel border styles are constructed manually in renderPanel.

// renderPanel renders content inside a manually constructed border with label and hint
// embedded in the top border line: ╭─ Label ────── hint ─╮
// width/height are the outer dimensions including border.
func renderPanel(label, hint, content string, width, height int, focused bool) string {
	borderColor := lipgloss.Color("240")
	labelColor := lipgloss.Color("240")
	if focused {
		borderColor = lipgloss.Color("75")
		labelColor = lipgloss.Color("75")
	}
	hintColor := lipgloss.Color("243")

	bc := lipgloss.NewStyle().Foreground(borderColor)
	lc := lipgloss.NewStyle().Foreground(labelColor).Bold(focused)
	hc := lipgloss.NewStyle().Foreground(hintColor)

	innerW := width - 2
	innerH := height - 2
	innerW = max(innerW, 1)
	innerH = max(innerH, 1)

	// Build top border with label and optional hint
	labelRendered := lc.Render(label)
	labelVisW := lipgloss.Width(labelRendered)

	var topLine string
	if hint != "" {
		hintRendered := hc.Render(hint)
		hintVisW := lipgloss.Width(hintRendered)
		// fixed overhead = 8 for border chars and spacing
		dashFill := max(innerW-labelVisW-hintVisW-6, 1)
		topLine = bc.Render("\u256d\u2500 ") + labelRendered + bc.Render(" "+strings.Repeat("\u2500", dashFill)+" ") + hintRendered + bc.Render(" \u2500\u256e")
	} else {
		dashFill := max(innerW-labelVisW-3, 1)
		topLine = bc.Render("\u256d\u2500 ") + labelRendered + bc.Render(" "+strings.Repeat("\u2500", dashFill)+"\u256e")
	}

	// Build content lines padded to innerW, wrapped with vertical borders
	vl := bc.Render("\u2502")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	var result strings.Builder
	result.WriteString(topLine)
	result.WriteString("\n")
	for _, line := range lines {
		lineW := lipgloss.Width(line)
		pad := max(innerW-lineW, 0)
		result.WriteString(vl + line + strings.Repeat(" ", pad) + vl)
		result.WriteString("\n")
	}

	// Bottom border
	bottomLine := bc.Render("\u2570" + strings.Repeat("\u2500", innerW) + "\u256f")
	result.WriteString(bottomLine)

	return result.String()
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Build favorites set for heart indicators
	favSet := a.playlist.store.FavoritesSet()
	a.search.favSet = favSet
	a.queue.favSet = favSet
	a.playlist.favSet = favSet
	a.history.favSet = favSet
	// Propagate relative line numbers setting
	a.search.relNumbers = a.relNumbers
	a.queue.relNumbers = a.relNumbers
	a.playlist.relNumbers = a.relNumbers
	a.history.relNumbers = a.relNumbers
	// Propagate tick counter and focus state — only focused panel gets tick/focus
	a.search.tick = 0
	a.queue.tick = 0
	a.playlist.tick = 0
	a.history.tick = 0
	a.search.focused = false
	a.queue.focused = false
	a.playlist.focused = false
	a.history.focused = false
	switch a.focusedPanel {
	case panelSearch:
		a.search.tick = a.tickCount
		a.search.focused = true
	case panelQueue:
		a.queue.tick = a.tickCount
		a.queue.focused = true
	case panelPlaylist:
		a.playlist.tick = a.tickCount
		a.playlist.focused = true
	case panelHistory:
		a.history.tick = a.tickCount
		a.history.focused = true
	case panelRadioHist:
		// radio history uses app-level fields, no sub-model tick/focused needed
	}

	// Bottom bar
	var bottomBar string
	if a.gotoActive {
		bottomBar = renderInputWithNowPlaying(a.gotoInput.View(), a.player.Status(), a.width, a.autoplay, a.shuffle, a.loopTrack, a.loopTotal)
	} else if a.colonActive {
		bottomBar = renderInputWithNowPlaying(a.colonInput.View(), a.player.Status(), a.width, a.autoplay, a.shuffle, a.loopTrack, a.loopTotal)
	} else if a.playlist.isFilterActive() {
		bottomBar = "/" + a.playlist.input.View()
	} else if a.queue.isFilterActive() {
		bottomBar = "/" + a.queue.filterInput.View()
	} else if a.search.isFilterActive() {
		bottomBar = "/" + a.search.filterInput.View()
	} else if a.history.isFilterActive() {
		bottomBar = "/" + a.history.filterInput.View()
	} else if a.radioHistFiltering {
		bottomBar = "/" + a.radioHistFilterInput.View()
	} else if a.artistsFiltering {
		bottomBar = "/" + a.artistsFilterInp.View()
	} else if a.depErr != "" {
		bottomBar = sbBgStyle.Width(a.width).Render("  " + a.depErr)
	} else if a.statusMsg != "" {
		bottomBar = sbBgStyle.Width(a.width).Render("  " + a.statusMsg)
	} else {
		bottomBar = renderNowPlaying(a.player.Status(), a.width, favSet, a.autoplay, a.shuffle, a.loopTrack, a.loopTotal, a.tickCount)
	}

	// Content area height = total height minus bottom bar (1)
	contentHeight := max(a.height-1, 1)

	// Help overlay — centered box, replaces panels
	if a.showHelp {
		return a.renderHelp() + "\n" + bottomBar
	}

	// Settings overlay — centered, replaces panels
	if a.showSettings {
		box := a.renderSettings()
		centered := lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
		return centered + "\n" + bottomBar
	}

	// Add-to-playlist overlay — centered, replaces panels
	if a.overlay.active {
		box := a.overlay.View(a.width)
		centered := lipgloss.Place(a.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
		return centered + "\n" + bottomBar
	}

	// --- Panel layout ---

	// Zoomed mode: focused panel takes full content area
	if a.zoomed {
		zW := a.width - 2
		zH := contentHeight - 2
		if zW < 1 {
			zW = 1
		}
		if zH < 1 {
			zH = 1
		}
		var label, hint, zContent string
		switch a.focusedPanel {
		case panelSearch:
			zContent = a.search.ViewConstrained(zW, zH)
			hint = "1  z=unzoom"
			if a.search.input.Focused() {
				hint = "INSERT  1  z=unzoom"
			} else if a.search.visual {
				hint = "VISUAL  1  z=unzoom"
			}
			label = "Search"
		case panelPlaylist:
			zContent = a.playlist.ViewConstrained(zW, zH)
			label = "Playlists"
			if a.playlist.level == levelDetail {
				if p := a.playlist.currentPlaylist(); p != nil {
					label = p.Name
				}
			}
			hint = "2  z=unzoom"
			if a.playlist.visual {
				hint = "VISUAL  2  z=unzoom"
			}
		case panelHistory:
			zContent = a.history.ViewConstrained(zW, zH)
			label = "Play History"
			hint = "5  z=unzoom"
			if a.history.visual {
				hint = "VISUAL  5  z=unzoom"
			}
		case panelQueue:
			zContent = renderQueueConstrained(a.qdata, &a.queue, zW, zH)
			label = "Queue"
			hint = "3  z=unzoom"
			if a.queue.visual {
				hint = "VISUAL  3  z=unzoom"
			}
		case panelRadioHist:
			zContent = a.renderRadioHistConstrained(zW, zH)
			label = "Radio History"
			hint = "6  z=unzoom"
			if a.radioHistVisual {
				hint = "VISUAL  6  z=unzoom"
			} else if a.radioHistFilter != "" {
				hint = "FILTER  6  z=unzoom"
			}
		case panelArtists:
			zContent = a.renderArtistsPanelConstrained(zW, zH)
			label = "Artists"
			hint = "4  z=unzoom"
			if a.artistsLevel == 1 {
				label = a.artistsPanelName
			} else if a.artistsLevel == 2 {
				label = a.artistsPanelAlbN
			}
		}
		panel := renderPanel(label, hint, zContent, a.width, contentHeight, true)
		return panel + "\n" + bottomBar
	}

	// Normal layout: Search top, Playlists left + Queue right

	// Dynamic search height:
	// - Unfocused or no results: minimal (just input + border = 3 lines)
	// - Focused with results: grows up to half screen
	searchFocused := a.focusedPanel == panelSearch
	searchMinH := 3 // border(2) + input(1)

	// Compute minimum bottom height needed for left column panels
	minBottomH := 3 // at least playlist
	if a.showArtistsPanel {
		minBottomH += 3
	}
	if a.showHistory {
		minBottomH += 3
	}
	if a.showRadio {
		minBottomH += 3
	}
	minBottomH = max(minBottomH, 5)

	// Cap search to leave enough room for bottom panels
	searchMaxH := contentHeight - minBottomH
	searchMaxH = min(searchMaxH, contentHeight/2)
	searchMaxH = max(searchMaxH, searchMinH)
	searchH := searchMinH
	expandSearch := searchFocused || a.pinSearch
	if expandSearch {
		nRes := len(a.search.results)
		if nRes > 0 || a.search.loading {
			// Each result = 1 line, plus input(1) + blank(1) + border(2)
			needed := nRes + 4
			if a.search.loading {
				needed = searchMinH + 3
			}
			needed = min(needed, searchMaxH)
			searchH = max(searchH, needed)
		} else if a.search.hasSearched && searchFocused {
			// Focused, searched but no results: show "No results found"
			searchH = searchMinH + 2
		} else if !a.pinSearch {
			// Focused but hasn't searched yet: stay compact
			searchH = searchMinH
		}
	}
	bottomH := contentHeight - searchH
	if bottomH < minBottomH {
		bottomH = minBottomH
		searchH = contentHeight - bottomH
	}
	searchH = max(searchH, 3)

	// Bottom area: Left column (Playlists [+ History] + Radio History) | Queue (right)
	leftW := a.width * 40 / 100
	rightW := a.width - leftW
	if leftW < 10 {
		leftW = 10
	}
	if rightW < 10 {
		rightW = 10
	}

	// Compute left column split: Playlists, [Artists], History, and Radio History heights
	// Playlist is "expanded" only when viewing tracks inside a playlist (levelDetail).
	// At list level, it's compact regardless of focus.
	plFocused := a.focusedPanel == panelPlaylist
	plExpanded := a.playlist.level == levelDetail && (plFocused || a.pinPlaylist)
	compactH := 3 // border(2) + 1 line summary
	radioHistExpanded := a.focusedPanel == panelRadioHist || a.pinRadio
	plListH := max(len(a.playlist.visiblePlaylists())+2, 3) // border(2) + 1 line per playlist

	// Pre-compute artists height needs
	artistsFocused := a.focusedPanel == panelArtists
	artistsExpanded := artistsFocused || (a.pinArtists && a.artistsLevel > 0)

	// Reserve space for artists panel in the left column calculations
	artistsReserve := 0 // extra height needed for artists panel
	if a.showArtistsPanel {
		if artistsExpanded && !plFocused && !plExpanded {
			// Will get bulk — handled separately below
			artistsReserve = 0
		} else if plFocused || plExpanded {
			artistsReserve = compactH
		} else {
			// Show artist list: need enough to display all artists
			artistsReserve = max(a.artistStore.Len()+2, compactH)
		}
	}

	var playlistH, historyH, radioHistH int

	// Compute minimum space needed for non-artist panels
	minNonArtist := 3 // at least playlist
	if a.showHistory {
		minNonArtist += 3
	}
	if a.showRadio {
		minNonArtist += 3
	}

	// Effective bottomH for non-artist panels when artists takes fixed space.
	// Clamp artistsReserve if it would leave too little for the other panels.
	if artistsReserve > 0 && bottomH-artistsReserve < minNonArtist {
		artistsReserve = max(bottomH-minNonArtist, compactH)
	}
	effectiveBottomH := bottomH - artistsReserve

	// Count how many compact panels are in the left column
	if a.showHistory && a.showRadio {
		// 3 panels (+ optional artists): Playlists, Play History, Radio History
		if plExpanded {
			// Playlist viewing tracks: give it bulk, others compact
			historyH = compactH
			radioHistH = compactH
			if radioHistExpanded {
				radioHistH = effectiveBottomH / 3
				radioHistH = max(radioHistH, 5)
			}
			playlistH = effectiveBottomH - historyH - radioHistH
		} else if a.focusedPanel == panelHistory {
			// History gets the bulk
			playlistH = plListH
			radioHistH = compactH
			if radioHistExpanded {
				radioHistH = effectiveBottomH / 3
				radioHistH = max(radioHistH, 5)
			}
			historyH = effectiveBottomH - playlistH - radioHistH
			if historyH < 5 {
				historyH = 5
				playlistH = effectiveBottomH - historyH - radioHistH
			}
		} else if a.focusedPanel == panelRadioHist || (radioHistExpanded && a.focusedPanel != panelHistory) {
			// Radio history gets the bulk
			playlistH = plListH
			historyH = compactH
			radioHistH = effectiveBottomH - playlistH - historyH
			if radioHistH < 5 {
				radioHistH = 5
				playlistH = effectiveBottomH - historyH - radioHistH
			}
		} else {
			// None focused — compact for all non-expanded
			playlistH = plListH
			historyH = compactH
			radioHistH = compactH
			remaining := effectiveBottomH - playlistH - historyH - radioHistH
			if remaining > 0 {
				// Give extra space to playlist if it can use it
				playlistH += remaining
			} else {
				// Not enough space, shrink playlist
				playlistH = effectiveBottomH - historyH - radioHistH
				if playlistH < 3 {
					playlistH = 3
					historyH = compactH
					radioHistH = effectiveBottomH - playlistH - historyH
					if radioHistH < 3 {
						radioHistH = 3
						historyH = effectiveBottomH - playlistH - radioHistH
					}
				}
			}
		}
	} else if a.showHistory && !a.showRadio {
		// 2 panels: Playlists + Play History (no Radio History)
		radioHistH = 0
		if plExpanded {
			historyH = compactH
			playlistH = effectiveBottomH - historyH
		} else if a.focusedPanel == panelHistory {
			playlistH = plListH
			historyH = effectiveBottomH - playlistH
			if historyH < 5 {
				historyH = 5
				playlistH = effectiveBottomH - historyH
			}
		} else {
			playlistH = plListH
			historyH = compactH
			remaining := effectiveBottomH - playlistH - historyH
			if remaining > 0 {
				playlistH += remaining
			} else {
				playlistH = effectiveBottomH - historyH
				if playlistH < 3 {
					playlistH = 3
					historyH = effectiveBottomH - playlistH
				}
			}
		}
	} else if !a.showHistory && a.showRadio {
		// 2 panels: Playlists + Radio History (no Play History)
		historyH = 0
		if plExpanded {
			radioHistH = compactH
			if radioHistExpanded {
				radioHistH = effectiveBottomH / 3
				radioHistH = max(radioHistH, 5)
			}
			playlistH = effectiveBottomH - radioHistH
		} else if a.focusedPanel == panelRadioHist || radioHistExpanded {
			playlistH = plListH
			radioHistH = effectiveBottomH - playlistH
			if radioHistH < 5 {
				radioHistH = 5
				playlistH = effectiveBottomH - radioHistH
			}
		} else {
			playlistH = plListH
			radioHistH = compactH
			remaining := effectiveBottomH - playlistH - radioHistH
			if remaining > 0 {
				playlistH += remaining
			} else {
				playlistH = effectiveBottomH - radioHistH
				if playlistH < 3 {
					playlistH = 3
					radioHistH = effectiveBottomH - playlistH
				}
			}
		}
	} else {
		// 1 panel: Playlists only (no history, no radio)
		historyH = 0
		radioHistH = 0
		playlistH = effectiveBottomH
	}

	// Clamp all panel heights to valid minimums
	playlistH = max(playlistH, 3)
	if a.showHistory {
		historyH = max(historyH, 1)
	}
	if a.showRadio {
		radioHistH = max(radioHistH, 1)
	}

	// Ensure non-artist left column panels sum exactly to effectiveBottomH.
	// Absorb difference into the focused/bulk panel to preserve layout intent.
	{
		diff := effectiveBottomH - (playlistH + historyH + radioHistH)
		if diff != 0 {
			if a.focusedPanel == panelHistory && a.showHistory {
				historyH += diff
				historyH = max(historyH, 1)
			} else if (a.focusedPanel == panelRadioHist || radioHistExpanded) && a.showRadio {
				radioHistH += diff
				radioHistH = max(radioHistH, 1)
			} else {
				playlistH += diff
				playlistH = max(playlistH, 3)
			}
		}
	}

	// Inner dimensions (subtract 2 for border on each axis)
	searchInnerW := a.width - 2
	searchInnerH := searchH - 2
	playlistInnerW := leftW - 2
	playlistInnerH := playlistH - 2
	queueInnerW := rightW - 2
	queueInnerH := bottomH - 2

	if searchInnerW < 1 {
		searchInnerW = 1
	}
	if searchInnerH < 1 {
		searchInnerH = 1
	}
	if playlistInnerW < 1 {
		playlistInnerW = 1
	}
	if playlistInnerH < 1 {
		playlistInnerH = 1
	}
	if queueInnerW < 1 {
		queueInnerW = 1
	}
	if queueInnerH < 1 {
		queueInnerH = 1
	}

	// Render panel contents
	searchContent := a.search.ViewConstrained(searchInnerW, searchInnerH)
	var playlistContent string
	if a.playlist.level == levelDetail && !plExpanded {
		// Collapsed detail: render list view to avoid cramped tracks
		playlistContent = a.playlist.viewListConstrained(0, playlistInnerW, playlistInnerH)
	} else {
		playlistContent = a.playlist.ViewConstrained(playlistInnerW, playlistInnerH)
	}
	queueContent := renderQueueConstrained(a.qdata, &a.queue, queueInnerW, queueInnerH)

	// Render bordered panels with labels and shortcut hints
	searchHint := "1"
	if a.focusedPanel == panelSearch && a.search.input.Focused() {
		searchHint = "INSERT  1"
	} else if a.focusedPanel == panelSearch && a.search.visual {
		searchHint = "VISUAL  1"
	}
	searchPanel := renderPanel("Search", searchHint, searchContent, a.width, searchH, a.focusedPanel == panelSearch)

	playlistLabel := "Playlists"
	if a.playlist.level == levelDetail && plExpanded {
		if p := a.playlist.currentPlaylist(); p != nil {
			playlistLabel = p.Name
		}
	}
	playlistHint := "2"
	if a.focusedPanel == panelPlaylist && (a.playlist.visual || a.playlist.listVisual) {
		playlistHint = "VISUAL  2"
	}
	playlistPanel := renderPanel(playlistLabel, playlistHint, playlistContent, leftW, playlistH, a.focusedPanel == panelPlaylist)

	queueLabel := "Queue"
	queueHint := "3"
	if a.focusedPanel == panelQueue && a.queue.visual {
		queueHint = "VISUAL  3"
	}
	queuePanel := renderPanel(queueLabel, queueHint, queueContent, rightW, bottomH, a.focusedPanel == panelQueue)

	// Artists panel (shown when showArtistsPanel is true)
	var artistsPanel string
	var artistsH int
	if a.showArtistsPanel {
		if artistsExpanded && !plFocused && !plExpanded {
			// Artists focused/pinned with content: gets the bulk, playlist compact
			artistsH = bottomH - compactH - historyH - radioHistH
			artistsH = max(artistsH, 5)
			playlistH = compactH
		} else if plFocused || plExpanded {
			// Playlist focused/expanded: artists compact
			artistsH = compactH
		} else {
			// Neither focused: split combined playlist+artists space equally
			combined := playlistH + artistsReserve
			artistsH = combined / 2
			playlistH = combined - artistsH // absorbs rounding
			// Clamp both to at least compactH
			if artistsH < compactH {
				artistsH = compactH
				playlistH = combined - artistsH
			}
			if playlistH < compactH {
				playlistH = compactH
				artistsH = combined - playlistH
			}
		}

		// Ensure all left panels sum exactly to bottomH
		artistsH = max(artistsH, compactH)
		playlistH = bottomH - artistsH - historyH - radioHistH
		if playlistH < 3 {
			playlistH = 3
			artistsH = bottomH - playlistH - historyH - radioHistH
			artistsH = max(artistsH, 3)
		}
		// Final guarantee
		{
			total := playlistH + artistsH + historyH + radioHistH
			if total != bottomH {
				// Absorb difference into the focused/bulk panel
				if a.focusedPanel == panelHistory {
					historyH += bottomH - total
					historyH = max(historyH, 1)
				} else if a.focusedPanel == panelRadioHist || radioHistExpanded {
					radioHistH += bottomH - total
					radioHistH = max(radioHistH, 1)
				} else {
					playlistH += bottomH - total
					playlistH = max(playlistH, 3)
				}
			}
		}

		playlistInnerH = playlistH - 2
		if playlistInnerH < 1 {
			playlistInnerH = 1
		}
		// Re-render playlist content with new height
		if playlistH <= compactH && a.focusedPanel != panelPlaylist {
			// Compact: just show count
			playlistContent = fmt.Sprintf("  %d playlists\n", len(a.playlist.store.Playlists))
		} else if a.playlist.level == levelDetail && !plExpanded {
			playlistContent = a.playlist.viewListConstrained(0, playlistInnerW, playlistInnerH)
		} else {
			playlistContent = a.playlist.ViewConstrained(playlistInnerW, playlistInnerH)
		}
		playlistPanel = renderPanel(playlistLabel, playlistHint, playlistContent, leftW, playlistH, a.focusedPanel == panelPlaylist)

		artistsInnerW := leftW - 2
		artistsInnerH := artistsH - 2
		if artistsInnerW < 1 {
			artistsInnerW = 1
		}
		if artistsInnerH < 1 {
			artistsInnerH = 1
		}
		// When unfocused and pin is off, show the artist list (level 0)
		// instead of a drilled-down album/track view.
		showLevel := a.artistsLevel
		if !artistsFocused && !a.pinArtists && showLevel > 0 {
			showLevel = 0
		}
		savedLevel := a.artistsLevel
		a.artistsLevel = showLevel
		var artistsContent string
		if artistsFocused || artistsH > compactH {
			artistsContent = a.renderArtistsPanelConstrained(artistsInnerW, artistsInnerH)
		} else {
			artistsContent = a.renderArtistsCompact(artistsInnerW)
		}
		a.artistsLevel = savedLevel
		artistsLabel := "Artists"
		switch showLevel {
		case 1:
			artistsLabel = a.artistsPanelName
		case 2:
			artistsLabel = a.artistsPanelAlbN
		}
		artistsHint := "4"
		if artistsFocused && a.artistsVisual {
			artistsHint = "VISUAL  4"
		} else if artistsFocused && a.artistsFilter != "" {
			artistsHint = "FILTER  4"
		}
		artistsPanel = renderPanel(artistsLabel, artistsHint, artistsContent, leftW, artistsH, artistsFocused)
	}

	// Radio History panel (rendered after artists to use final radioHistH)
	var radioHistPanel string
	if a.showRadio {
		radioHistInnerW := leftW - 2
		radioHistInnerH := radioHistH - 2
		if radioHistInnerW < 1 {
			radioHistInnerW = 1
		}
		if radioHistInnerH < 1 {
			radioHistInnerH = 1
		}
		var radioHistContent string
		if a.focusedPanel == panelRadioHist || radioHistExpanded {
			radioHistContent = a.renderRadioHistConstrained(radioHistInnerW, radioHistInnerH)
		} else {
			radioHistContent = a.renderRadioHistCompact(radioHistInnerW)
		}
		radioHistHint := "6"
		if a.focusedPanel == panelRadioHist && a.radioHistVisual {
			radioHistHint = "VISUAL  6"
		} else if a.focusedPanel == panelRadioHist && a.radioHistFilter != "" {
			radioHistHint = "FILTER  6"
		}
		radioHistPanel = renderPanel("Radio History", radioHistHint, radioHistContent, leftW, radioHistH, a.focusedPanel == panelRadioHist)
	}

	// Build left column
	var leftCol string
	if a.showHistory && a.showRadio {
		historyInnerW := leftW - 2
		historyInnerH := historyH - 2
		if historyInnerW < 1 {
			historyInnerW = 1
		}
		if historyInnerH < 1 {
			historyInnerH = 1
		}

		var historyContent string
		if a.focusedPanel == panelHistory {
			historyContent = a.history.ViewConstrained(historyInnerW, historyInnerH)
		} else {
			historyContent = a.history.ViewCompact(historyInnerW)
		}
		historyHint := "5"
		if a.focusedPanel == panelHistory && a.history.visual {
			historyHint = "VISUAL  5"
		} else if a.focusedPanel == panelHistory && a.history.isFiltered() {
			historyHint = "FILTER  5"
		}
		historyPanel := renderPanel("Play History", historyHint, historyContent, leftW, historyH, a.focusedPanel == panelHistory)
		leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, historyPanel, radioHistPanel)
		if a.showArtistsPanel {
			leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, artistsPanel, historyPanel, radioHistPanel)
		}
	} else if a.showHistory && !a.showRadio {
		historyInnerW := leftW - 2
		historyInnerH := historyH - 2
		if historyInnerW < 1 {
			historyInnerW = 1
		}
		if historyInnerH < 1 {
			historyInnerH = 1
		}

		var historyContent string
		if a.focusedPanel == panelHistory {
			historyContent = a.history.ViewConstrained(historyInnerW, historyInnerH)
		} else {
			historyContent = a.history.ViewCompact(historyInnerW)
		}
		historyHint := "5"
		if a.focusedPanel == panelHistory && a.history.visual {
			historyHint = "VISUAL  5"
		} else if a.focusedPanel == panelHistory && a.history.isFiltered() {
			historyHint = "FILTER  5"
		}
		historyPanel := renderPanel("Play History", historyHint, historyContent, leftW, historyH, a.focusedPanel == panelHistory)
		leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, historyPanel)
		if a.showArtistsPanel {
			leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, artistsPanel, historyPanel)
		}
	} else if !a.showHistory && a.showRadio {
		leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, radioHistPanel)
		if a.showArtistsPanel {
			leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, artistsPanel, radioHistPanel)
		}
	} else {
		leftCol = playlistPanel
		if a.showArtistsPanel {
			leftCol = lipgloss.JoinVertical(lipgloss.Left, playlistPanel, artistsPanel)
		}
	}

	// Bottom row = left column + queue side by side
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, queuePanel)

	// Stack: search on top, bottom row below
	content := lipgloss.JoinVertical(lipgloss.Left, searchPanel, bottomRow)

	return content + "\n" + bottomBar
}
