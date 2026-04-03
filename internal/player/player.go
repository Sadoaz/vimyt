// Package player controls mpv playback via JSON IPC.
package player

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/Sadoaz/vimyt/internal/model"
	"github.com/Sadoaz/vimyt/internal/youtube"
)

// Player controls mpv via JSON IPC over a Unix socket.
type Player struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	conn      net.Conn
	socket    string
	state     model.PlayerState
	track     *model.Track
	pos       time.Duration
	dur       time.Duration
	volume    int
	wasPlay   bool // was playing before current status poll (for end detection)
	errMsg    string
	resolving bool   // true while resolving audio URL
	gen       uint64 // generation counter — incremented on each Play/PlayPaused call
	resuming  bool   // true during PlayPaused resume — prevents Status() from overriding pause state
	// Debounce: when Play() is called rapidly (e.g. spamming n/N),
	// we delay the yt-dlp resolve so only the last skip actually hits YouTube.
	debounceTimer *time.Timer
	// resolveCancel cancels any in-flight yt-dlp URL resolution, killing the process.
	resolveCancel context.CancelFunc
}

// ipcResponse is the JSON structure from mpv IPC.
type ipcResponse struct {
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
	Event     string          `json:"event"`
	RequestID int             `json:"request_id"`
}

// New creates a player backed by an mpv subprocess.
func New() *Player {
	p := &Player{
		state:  model.Stopped,
		volume: 50,
	}

	if _, err := exec.LookPath("mpv"); err != nil {
		p.errMsg = "mpv not found on PATH"
		return p
	}

	p.socket = fmt.Sprintf("/tmp/vimyt-mpv-%d.sock", os.Getpid())
	os.Remove(p.socket)

	p.cmd = exec.Command("mpv",
		"--no-video",
		"--really-quiet",
		"--idle",
		fmt.Sprintf("--input-ipc-server=%s", p.socket),
		fmt.Sprintf("--volume=%d", p.volume),
	)
	p.cmd.Stdout = nil
	p.cmd.Stderr = nil

	if err := p.cmd.Start(); err != nil {
		p.errMsg = fmt.Sprintf("failed to start mpv: %v", err)
		return p
	}

	// Wait for socket to appear
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		if conn, err := net.Dial("unix", p.socket); err == nil {
			p.conn = conn
			break
		}
	}
	if p.conn == nil {
		p.errMsg = "failed to connect to mpv IPC socket"
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
	}

	return p
}

// sendCommand sends a JSON IPC command to mpv and returns the response data.
func (p *Player) sendCommand(args ...any) (json.RawMessage, error) {
	if p.conn == nil {
		return nil, fmt.Errorf("no mpv connection")
	}

	msg := map[string]any{
		"command": args,
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := p.conn.Write(data); err != nil {
		return nil, fmt.Errorf("mpv write error: %w", err)
	}

	// Read response — skip event lines, find the command response
	p.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(p.conn)

	for scanner.Scan() {
		var resp ipcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		// Skip event messages
		if resp.Event != "" {
			continue
		}
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv error: %s", resp.Error)
		}
		return resp.Data, nil
	}

	return nil, fmt.Errorf("mpv read error: no response")
}

// getProperty queries a single mpv property.
func (p *Player) getProperty(name string) (json.RawMessage, error) {
	return p.sendCommand("get_property", name)
}

// playDebounce is how long Play() waits before resolving the URL.
// If another Play() call arrives within this window, the previous one is cancelled.
// This prevents spamming YouTube/yt-dlp when the user holds down n/N.
const playDebounce = 150 * time.Millisecond

// Play resolves the audio URL for the track and sends it to mpv.
// Rapid consecutive calls are debounced: only the last call within
// the debounce window actually resolves the URL and starts playback.
func (p *Player) Play(t *model.Track) {
	p.mu.Lock()
	p.gen++ // invalidate any in-flight goroutines from previous Play calls
	myGen := p.gen
	p.track = t
	p.pos = 0
	p.dur = t.Duration
	p.state = model.Playing
	p.wasPlay = false // don't detect EOF until track actually starts playing
	p.resolving = true
	p.resuming = false // clear any pending resume

	// Cancel any pending debounce timer from a previous Play() call
	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}

	// Kill any in-flight yt-dlp process from a previous resolve
	if p.resolveCancel != nil {
		p.resolveCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.resolveCancel = cancel

	// Start a debounce timer — the actual resolve happens after the delay.
	// If Play() is called again before the timer fires, this timer is
	// stopped and replaced, so only the final track gets resolved.
	p.debounceTimer = time.AfterFunc(playDebounce, func() {
		p.resolveAndLoad(ctx, t, myGen)
	})
	p.mu.Unlock()
}

// resolveAndLoad does the actual yt-dlp URL resolution and mpv loading.
// Called after the debounce timer fires. The context allows cancellation
// when the user skips to another track before resolution completes.
func (p *Player) resolveAndLoad(ctx context.Context, t *model.Track, myGen uint64) {
	url, err := youtube.ResolveURLCtx(ctx, t.ID)

	p.mu.Lock()
	defer p.mu.Unlock()

	// If another Play() was called while we were resolving, abandon this one
	if p.gen != myGen {
		return
	}

	p.resolving = false

	if err != nil {
		// Don't report cancellation as a playback error
		if ctx.Err() != nil {
			return
		}
		p.state = model.Stopped
		p.track = nil // prevent autoplay from firing on failure
		p.errMsg = fmt.Sprintf("URL resolve failed: %v", err)
		return
	}

	if p.conn == nil {
		return
	}

	// Ensure mpv is unpaused before loading so playback starts immediately
	p.sendCommand("set_property", "pause", false)

	// Load the file in mpv
	_, loadErr := p.sendCommand("loadfile", url)
	if loadErr != nil {
		// Maybe expired cached URL — retry once
		youtube.InvalidateURL(t.ID)
		url2, err2 := youtube.ResolveURLCtx(ctx, t.ID)
		if err2 != nil {
			if ctx.Err() != nil {
				return
			}
			p.state = model.Stopped
			p.track = nil
			p.errMsg = fmt.Sprintf("Failed to play: %v", err2)
			return
		}
		if _, err3 := p.sendCommand("loadfile", url2); err3 != nil {
			p.state = model.Stopped
			p.track = nil
			p.errMsg = fmt.Sprintf("Failed to load track: %v", err3)
			return
		}
	}
	// wasPlay stays false — Status() will set it to true once time-pos is available
}

// PlayPaused loads a track but keeps mpv paused. Used for session resume so
// no audio is heard until the user explicitly presses play.
func (p *Player) PlayPaused(t *model.Track, seekPos float64) {
	p.mu.Lock()
	p.gen++
	myGen := p.gen
	p.track = t
	p.pos = time.Duration(seekPos * float64(time.Second))
	p.dur = t.Duration
	p.state = model.Paused
	p.wasPlay = false // don't trigger EOF detection
	p.resolving = true
	p.resuming = true // prevent Status() from overriding pause state during resume

	// Kill any in-flight yt-dlp process from a previous resolve
	if p.resolveCancel != nil {
		p.resolveCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.resolveCancel = cancel
	p.mu.Unlock()

	go func() {
		url, err := youtube.ResolveURLCtx(ctx, t.ID)

		p.mu.Lock()
		if p.gen != myGen {
			p.mu.Unlock()
			return
		}
		p.resolving = false

		if err != nil {
			if ctx.Err() != nil {
				p.mu.Unlock()
				return
			}
			p.state = model.Stopped
			p.track = nil
			p.errMsg = fmt.Sprintf("URL resolve failed: %v", err)
			p.mu.Unlock()
			return
		}

		if p.conn == nil {
			p.mu.Unlock()
			return
		}

		// Only pause mpv if user hasn't already unpaused while we were resolving
		if p.state == model.Paused {
			p.sendCommand("set_property", "pause", true)
		}

		_, loadErr := p.sendCommand("loadfile", url)
		if loadErr != nil {
			youtube.InvalidateURL(t.ID)
			url2, err2 := youtube.ResolveURLCtx(ctx, t.ID)
			if err2 != nil {
				if ctx.Err() != nil {
					p.mu.Unlock()
					return
				}
				p.state = model.Stopped
				p.track = nil
				p.mu.Unlock()
				return
			}
			p.sendCommand("loadfile", url2)
		}
		p.mu.Unlock()

		// Wait for mpv to load the stream, then seek to saved position.
		// Release the lock during wait so Status() isn't blocked.
		if seekPos > 0 {
			time.Sleep(1500 * time.Millisecond)
			p.mu.Lock()
			if p.gen == myGen && p.conn != nil {
				p.sendCommand("seek", fmt.Sprintf("%.1f", seekPos), "absolute")
				p.pos = time.Duration(seekPos * float64(time.Second))
				// Explicitly set mpv pause to match user's intent
				// p.state is authoritative here since resuming=true blocks Status() overrides
				if p.state == model.Playing {
					p.sendCommand("set_property", "pause", false)
				}
			}
			p.resuming = false // allow Status() to sync pause state again
			p.mu.Unlock()
		} else {
			p.mu.Lock()
			p.resuming = false
			p.mu.Unlock()
		}
	}()
}

// Pause toggles between Playing and Paused.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.state == model.Stopped {
		return
	}

	p.sendCommand("cycle", "pause")

	switch p.state {
	case model.Playing:
		p.state = model.Paused
	case model.Paused:
		p.state = model.Playing
		// wasPlay will be set by Status() once time-pos confirms playback
	}
}

// Stop stops playback.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		p.sendCommand("stop")
	}
	p.state = model.Stopped
	p.pos = 0
	p.wasPlay = false
}

// Seek adjusts position by delta seconds.
func (p *Player) Seek(deltaSec int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.state == model.Stopped {
		return
	}

	p.sendCommand("seek", strconv.Itoa(deltaSec), "relative")
}

// SeekAbsolute seeks to an absolute position in seconds.
func (p *Player) SeekAbsolute(sec float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.state == model.Stopped {
		return
	}

	p.sendCommand("seek", fmt.Sprintf("%.1f", sec), "absolute")
}

// Status returns the current player status, polling mpv for real-time data.
func (p *Player) Status() model.PlayerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	status := model.PlayerStatus{
		State:    p.state,
		Track:    p.track,
		Position: p.pos,
		Volume:   p.volume,
	}

	if p.conn == nil || p.state == model.Stopped || p.resolving {
		return status
	}

	// Poll mpv for current position
	timePosOK := false
	if data, err := p.getProperty("time-pos"); err == nil {
		var pos float64
		if json.Unmarshal(data, &pos) == nil {
			p.pos = time.Duration(pos * float64(time.Second))
			status.Position = p.pos
			timePosOK = true
			// Track is confirmed playing — enable EOF detection
			if !p.wasPlay && p.state == model.Playing {
				p.wasPlay = true
			}
		}
	}

	// Poll duration (might differ from track metadata)
	if data, err := p.getProperty("duration"); err == nil {
		var dur float64
		if json.Unmarshal(data, &dur) == nil && dur > 0 {
			p.dur = time.Duration(dur * float64(time.Second))
			// Update track duration if we have a track
			if p.track != nil {
				p.track.Duration = p.dur
			}
		}
	}

	// Check pause state from mpv — but not during resume (user's intent is authoritative)
	if !p.resuming {
		if data, err := p.getProperty("pause"); err == nil {
			var paused bool
			if json.Unmarshal(data, &paused) == nil {
				if paused && p.state == model.Playing {
					p.state = model.Paused
				} else if !paused && p.state == model.Paused {
					p.state = model.Playing
				}
			}
		}
	}
	status.State = p.state

	// Check if track ended — primary: eof-reached property
	eofDetected := false
	if data, err := p.getProperty("eof-reached"); err == nil {
		var eof bool
		if json.Unmarshal(data, &eof) == nil && eof && p.wasPlay {
			eofDetected = true
		}
	}

	// Fallback EOF: if time-pos became unavailable while we were playing,
	// mpv likely entered idle mode (track ended).
	if !eofDetected && !timePosOK && p.wasPlay {
		eofDetected = true
	}

	if eofDetected {
		p.state = model.Stopped
		p.wasPlay = false
		status.State = model.Stopped
	}

	return status
}

// SetVolume sets volume (clamped 0-100).
func (p *Player) SetVolume(v int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	v = max(v, 0)
	v = min(v, 100)
	p.volume = v

	if p.conn != nil {
		p.sendCommand("set_property", "volume", v)
	}
}

// ErrMsg returns any error message from initialization.
func (p *Player) ErrMsg() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.errMsg
}

// PopErr returns and clears any playback error message.
// Used by the TUI to surface errors (e.g. URL resolve failures) in the status bar.
func (p *Player) PopErr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	msg := p.errMsg
	p.errMsg = ""
	return msg
}

// Close terminates mpv and cleans up.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}
	// Kill any in-flight yt-dlp process
	if p.resolveCancel != nil {
		p.resolveCancel()
		p.resolveCancel = nil
	}
	if p.conn != nil {
		p.sendCommand("quit")
		p.conn.Close()
		p.conn = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Wait()
	}
	os.Remove(p.socket)
}
