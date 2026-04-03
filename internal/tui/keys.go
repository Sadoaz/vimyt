package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	HalfDown   key.Binding
	HalfUp     key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Search     key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Quit       key.Binding
	Tab1       key.Binding
	Tab2       key.Binding
	Tab3       key.Binding
	Tab4       key.Binding
	Tab5       key.Binding
	TabNext    key.Binding
	TabPrev    key.Binding
	Playlist   key.Binding
	Space      key.Binding
	Next       key.Binding
	Prev       key.Binding
	NavForward key.Binding
	NavBack    key.Binding
	Backspace  key.Binding
	SeekFwd    key.Binding
	SeekBack   key.Binding
	PanelLeft  key.Binding
	PanelRight key.Binding
	PanelUp    key.Binding
	PanelDown  key.Binding
	Delete     key.Binding
	Cut        key.Binding
	Paste      key.Binding
	MoveUp     key.Binding
	MoveDown   key.Binding
	Visual     key.Binding
	VisualEnd  key.Binding
	Yank       key.Binding
	Help       key.Binding
	Clear      key.Binding
	AddToList  key.Binding
	CreatePL   key.Binding
	EditPL     key.Binding
	Radio      key.Binding
	Colon      key.Binding
	Undo       key.Binding
	Redo       key.Binding

	FocusSearch key.Binding
	Zoom        key.Binding
	Filter      key.Binding
	Favorite    key.Binding
	Settings    key.Binding
	VolumeUp    key.Binding
	VolumeDown  key.Binding
	JumpBack    key.Binding
	JumpFwd     key.Binding
	Randomize   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "move down"),
	),
	HalfDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "half page down"),
	),
	HalfUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "half page up"),
	),
	Top: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("gg", "go to top"),
	),
	Bottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "go to bottom"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search/filter"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select/confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel/back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Tab1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "search panel"),
	),
	Tab2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "playlists panel"),
	),
	Tab3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "queue panel"),
	),
	Tab4: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "history panel"),
	),
	Tab5: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "radio history panel"),
	),
	TabNext: key.NewBinding(
		key.WithKeys("alt+l"),
		key.WithHelp("M-l/L", "next panel"),
	),
	TabPrev: key.NewBinding(
		key.WithKeys("alt+h"),
		key.WithHelp("M-h/H", "prev panel"),
	),
	Playlist: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "playlist panel"),
		key.WithDisabled(),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "pause/resume"),
	),
	Next: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "next track"),
	),
	Prev: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "prev track"),
	),
	NavForward: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "enter/forward"),
	),
	NavBack: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "back"),
	),
	Backspace: key.NewBinding(
		key.WithKeys("backspace"),
		key.WithHelp("bs", "back"),
	),
	SeekFwd: key.NewBinding(
		key.WithKeys(">"),
		key.WithHelp(">", "seek forward 5s"),
	),
	SeekBack: key.NewBinding(
		key.WithKeys("<"),
		key.WithHelp("<", "seek backward 5s"),
	),
	PanelLeft: key.NewBinding(
		key.WithKeys("H"),
		key.WithHelp("H", "panel left"),
	),
	PanelRight: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "panel right"),
	),
	PanelUp: key.NewBinding(
		key.WithKeys("K"),
		key.WithHelp("K", "panel up"),
	),
	PanelDown: key.NewBinding(
		key.WithKeys("J"),
		key.WithHelp("J", "panel down"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("dd/d", "delete"),
	),
	Cut: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "cut"),
	),
	Paste: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "paste"),
	),
	MoveUp: key.NewBinding(
		key.WithKeys("alt+k"),
		key.WithHelp("M-k/K", "panel up"),
	),
	MoveDown: key.NewBinding(
		key.WithKeys("alt+j"),
		key.WithHelp("M-j/J", "panel down"),
	),
	Visual: key.NewBinding(
		key.WithKeys("v", "V"),
		key.WithHelp("v/V", "visual select"),
	),
	VisualEnd: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "swap visual end"),
	),
	Yank: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("yy/y", "yank to queue"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Clear: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "clear queue"),
	),
	AddToList: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add to playlist"),
	),
	CreatePL: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "goto current track"),
	),
	EditPL: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "rename playlist"),
	),
	Radio: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "start radio"),
	),
	Colon: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":<n>", "jump to line"),
	),
	Undo: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "undo"),
	),
	Redo: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "redo"),
	),

	FocusSearch: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "focus search"),
		key.WithDisabled(),
	),
	Zoom: key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("z", "zoom panel"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter list"),
	),
	Favorite: key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("F", "toggle favorite"),
	),
	Settings: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "settings"),
	),
	VolumeUp: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+/=", "volume up"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "volume down"),
	),
	JumpBack: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "jump back"),
	),
	JumpFwd: key.NewBinding(
		key.WithKeys("ctrl+i", "tab"),
		key.WithHelp("ctrl+i", "jump forward"),
	),
	Randomize: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "randomize queue"),
	),
}

// helpEntry is a single row in the help overlay: key, action, description.
type helpEntry struct {
	key    string
	action string
	desc   string
}

// helpSection groups help entries under a category header.
type helpSection struct {
	title   string
	entries []helpEntry
}

// helpSections returns categorized key bindings for the help overlay.
func helpSections() []helpSection {
	return []helpSection{
		{"Navigation", []helpEntry{
			{"j/k", "Move down/up", "cursor movement in all panels"},
			{"ctrl+d", "Half page down", "fast scroll down"},
			{"ctrl+u", "Half page up", "fast scroll up"},
			{"gg", "Go to top", "jump to first item"},
			{"G", "Go to bottom", "jump to last item"},
			{"l", "Enter/forward", "open playlist or confirm"},
			{"h", "Back", "go back to playlist list"},
			{"backspace", "Back", "same as h"},
			{":<n>", "Jump to line", "jump to line number"},
		}},
		{"Panels", []helpEntry{
			{"1", "Search panel", "focus search panel"},
			{"2", "Playlists panel", "focus playlists panel"},
			{"3", "Queue panel", "focus queue panel"},
			{"4", "History panel", "focus play history panel"},
			{"5", "Radio History", "focus radio history panel"},
			{"H/L", "Panel left/right", "move focus left/right"},
			{"J/K", "Panel down/up", "move focus down/up"},
			{"M-h/M-l", "Prev/next panel", "cycle through panels"},
			{"z", "Zoom panel", "toggle fullscreen on focused panel"},
		}},
		{"Search & Filter", []helpEntry{
			{"/", "Search", "jump to search input from anywhere"},
			{"f", "Filter list", "filter current panel's items"},
			{"enter", "Select/confirm", "play track or confirm action"},
			{"esc", "Cancel/back", "close input or go back"},
		}},
		{"Playback", []helpEntry{
			{"space", "Pause/resume", "toggle playback"},
			{"N", "Next track", "play next track in queue"},
			{"P", "Previous track", "go back to last played track"},
			{">", "Seek forward", "seek forward 5 seconds"},
			{"<", "Seek backward", "seek backward 5 seconds"},
			{"g", "Goto time", "type time to seek (e.g. 1:23)"},
			{"+/=", "Volume up", "increase volume by 5"},
			{"-", "Volume down", "decrease volume by 5"},
		}},
		{"Selection & Editing", []helpEntry{
			{"v/V", "Visual select", "start visual selection mode"},
			{"o", "Swap visual end", "swap anchor/cursor in visual"},
			{"yy/y", "Yank to queue", "copy tracks to queue"},
			{"dd/d", "Delete", "remove tracks from queue/playlist"},
			{"x", "Cut", "delete + copy to clipboard"},
			{"p", "Paste", "paste from clipboard"},
			{"M-k/M-j", "Move up/down", "reorder tracks in queue"},
			{"C", "Clear queue", "remove all tracks from queue"},
			{"R", "Randomize queue", "shuffle the order of tracks in queue"},
		}},
		{"Playlists & Tracks", []helpEntry{
			{"a", "Add to playlist", "add track(s) to a playlist; on playlist list: add all to queue"},
			{"o", "Create playlist", "new playlist (in playlist list)"},
			{"c", "Goto current", "jump to currently playing track in queue"},
			{"o", "Create playlist", "create new playlist (in playlist list view)"},
			{"e", "Rename playlist", "rename selected playlist"},
			{"F", "Toggle favorite", "add/remove from favorites"},
			{"r", "Start radio", "generate radio mix from track"},
		}},
		{"Radio History (panel 5)", []helpEntry{
			{"Enter", "Recover radio", "restore a previous radio mix"},
			{"dd/d", "Delete", "remove radio session(s)"},
			{"v", "Visual select", "select multiple sessions"},
			{"u", "Undo", "undo radio history change"},
			{"ctrl+r", "Redo", "redo radio history change"},
			{"f", "Filter", "filter radio history by name"},
		}},
		{"Other", []helpEntry{
			{"u", "Undo", "undo last queue/playlist change"},
			{"ctrl+r", "Redo", "redo last undone change"},
			{"ctrl+o", "Jump back", "go to previous panel (jumplist)"},
			{"ctrl+i", "Jump forward", "go to next panel (jumplist)"},
			{"S", "Settings", "open settings overlay"},
			{"?", "Help", "toggle this help overlay"},
			{"q", "Quit", "exit vimyt"},
		}},
	}
}
