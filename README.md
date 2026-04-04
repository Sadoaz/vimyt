# vimyt

TUI for YouTube Music with vim keybindings and radio mixes. Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

Search, queue, playlists, artists, radio mixes, and history. All navigated with vim keybindings (`j/k`, `gg/G`, visual select, yank/delete/paste, undo/redo). Session state persists across launches.

Demo:



https://github.com/user-attachments/assets/7719af94-4268-44dc-8ae1-32e5420bd23c


## Prerequisites

- [Go](https://go.dev/) 1.25+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) -- YouTube search and audio URL resolution
- [mpv](https://mpv.io/) -- audio playback engine

`yt-dlp` and `mpv` must be on your `PATH`.

## Installation

### Go install

Make sure `$GOPATH/bin` is in your `PATH`:

```bash
# bash/zsh
export PATH="$HOME/go/bin:$PATH"

# fish
fish_add_path ~/go/bin
```

Then run:

```bash
go install github.com/Sadoaz/vimyt@latest
vimyt
```

### From source

```bash
git clone https://github.com/Sadoaz/vimyt.git
cd vimyt
go build -o vimyt .
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
| `1`-`6` | Jump to panel by number |
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
| `R` | Randomize queue |

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
| `a` | Add track/playlist to queue or add artist |
| `F` | Toggle favorite |
| `r` | Start radio mix from track |
| `S` | Open settings |
| `?` | Help overlay |
| `q` | Quit |

## Settings

Press `S` to open settings:

- Autoplay / Shuffle / Loop Track (infinite or x times)
- Focus Queue on add
- Relative line numbers
- Pin Search / Playlist / Radio / Artists panels
- Show/Hide History / Radio History / Artists
- YT Auth -- browser cookie auth (Firefox, Chrome, Chromium, Brave, Edge) for private playlists
- Import Playlist by URL

## Data Storage

Only metadata is stored locally. No audio or video is downloaded. Location depends on your OS:

- **macOS**: `~/Library/Application Support/vimyt/`
- **Linux**: `~/.config/vimyt/`

```
  session.json        # Session state (cursors, settings, playback position)
  queue.json          # Persisted queue
  play_history.json   # Last 500 played tracks
  radio_history.json  # Last 100 radio mix sessions
  artists.json        # Followed artists and cached albums
  playlists/          # Playlist JSON files
```


