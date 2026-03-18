# vimyt

TUI for YouTube Music with vim keybindings and radio mixes. Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

Search, queue, playlists, radio mixes, and history. All navigated with vim keybindings (`j/k`, `gg/G`, visual select, yank/delete/paste, undo/redo). Session state persists across launches.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) -- YouTube search and audio URL resolution
- [mpv](https://mpv.io/) -- audio playback engine

`yt-dlp` and `mpv` must be on your `PATH`.

## Installation

```bash
go install github.com/Sadoaz/vimyt@latest
```

Or build from source:

```bash
git clone https://github.com/Sadoaz/vimyt.git
cd vimyt
go build -o vimyt .
```

## Usage

```bash
./vimyt
```

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Move down / up |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+d` / `Ctrl+u` | Half-page down / up |
| `h` / `l` | Navigate back / forward (enter playlists) |
| `H` / `J` / `K` / `L` | Switch panel focus (left / down / up / right) |
| `1`-`5` | Jump to panel by number |
| `Alt+h` / `Alt+l` | Cycle panels |
| `Ctrl+o` / `Ctrl+i` | Jumplist back / forward |
| `z` | Zoom current panel |
| `:<n>` | Jump to line number |

### Playback

| Key | Action |
|---|---|
| `Enter` | Play selected track |
| `Space` | Toggle play / pause |
| `>` / `<` | Seek forward / backward |
| `g` + time | Seek to time (e.g. `1:23`) |
| `+` / `-` | Volume up / down |
| `n` | Next track |

### Editing

| Key | Action |
|---|---|
| `v` / `V` | Visual selection mode |
| `yy` / `y` | Yank (copy) |
| `dd` / `d` | Delete |
| `x` | Cut |
| `p` | Paste |
| `o` | Swap visual selection end |
| `u` | Undo |
| `Ctrl+r` | Redo |

### Other

| Key | Action |
|---|---|
| `/` | Search (from any panel) |
| `f` | Filter current panel |
| `a` | Add track to playlist |
| `F` | Toggle favorite |
| `r` | Start radio mix from track |
| `S` | Open settings |
| `?` | Help overlay |
| `q` | Quit |

## Settings

Press `S` to open settings:

- Autoplay / Shuffle
- Focus Queue on add
- Relative line numbers
- Pin Search / Playlist / Radio panels
- Show/Hide History / Radio History
- YT Auth -- browser cookie auth (Firefox, Chrome, Chromium, Brave, Edge) for private playlists
- Import Playlist by URL

## Data Storage

Everything is stored locally on disk in `~/.config/vimyt/`:

```
~/.config/vimyt/
  session.json        # Session state (cursors, settings, playback position)
  queue.json          # Persisted queue
  play_history.json   # Last 500 played tracks
  radio_history.json  # Last 100 radio mix sessions
  playlists/          # Playlist JSON files
```


