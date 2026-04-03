package youtube

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// cookieBrowser holds the browser name for --cookies-from-browser (empty = disabled).
var (
	cookieMu      sync.RWMutex
	cookieBrowser string
)

// SetCookieBrowser sets the browser for yt-dlp cookie authentication.
// Pass "" to disable.
func SetCookieBrowser(browser string) {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	cookieBrowser = browser
}

// GetCookieBrowser returns the current cookie browser setting.
func GetCookieBrowser() string {
	cookieMu.RLock()
	defer cookieMu.RUnlock()
	return cookieBrowser
}

// CookieArgs returns the yt-dlp args for cookie auth, or empty slice if disabled.
func CookieArgs() []string {
	b := GetCookieBrowser()
	if b == "" {
		return nil
	}
	return []string{"--cookies-from-browser", b}
}

// urlTTL is how long a cached audio stream URL is considered valid.
// YouTube URLs typically expire after ~6 hours; we evict earlier to
// avoid playback failures that trigger expensive retry loops.
const urlTTL = 4 * time.Hour

// maxURLCacheSize caps the number of entries to prevent unbounded growth.
const maxURLCacheSize = 200

type urlEntry struct {
	url      string
	cachedAt time.Time
}

// URLCache caches resolved audio stream URLs by YouTube video ID.
type URLCache struct {
	mu      sync.RWMutex
	entries map[string]urlEntry
}

var urlCache = &URLCache{entries: make(map[string]urlEntry)}

// Get returns the cached URL for a video ID, or empty string if not cached or expired.
func (c *URLCache) Get(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return ""
	}
	if time.Since(e.cachedAt) > urlTTL {
		return ""
	}
	return e.url
}

// Set stores an audio URL for a video ID with the current timestamp.
func (c *URLCache) Set(id, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = urlEntry{url: url, cachedAt: time.Now()}
	// Evict expired entries if cache is getting large
	if len(c.entries) > maxURLCacheSize {
		now := time.Now()
		for k, e := range c.entries {
			if now.Sub(e.cachedAt) > urlTTL {
				delete(c.entries, k)
			}
		}
	}
}

// Invalidate removes a cached URL (e.g., when it expires).
func (c *URLCache) Invalidate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

// ResolveURL returns the audio stream URL for a YouTube video ID.
// Checks cache first, falls back to yt-dlp subprocess.
func ResolveURL(id string) (string, error) {
	return ResolveURLCtx(context.Background(), id)
}

// ResolveURLCtx is like ResolveURL but accepts a context for cancellation.
// When the context is cancelled, any in-flight yt-dlp process is killed.
func ResolveURLCtx(ctx context.Context, id string) (string, error) {
	if url := urlCache.Get(id); url != "" {
		return url, nil
	}

	url, err := fetchAudioURL(ctx, id)
	if err != nil {
		return "", err
	}
	urlCache.Set(id, url)
	return url, nil
}

// InvalidateURL removes a cached URL so next ResolveURL re-fetches.
func InvalidateURL(id string) {
	urlCache.Invalidate(id)
}

func fetchAudioURL(parent context.Context, id string) (string, error) {
	// Try formats in order: bestaudio, bestaudio*, best (fallback for videos without separate audio).
	// Format availability doesn't depend on cookies, so we try all formats with the
	// current cookie config first. Only if every format fails AND cookies are enabled
	// do we retry the primary format without cookies (auth issue fallback).
	// This reduces worst-case yt-dlp calls from 6 to 4.
	formats := []string{"bestaudio", "bestaudio*", "best"}
	cookies := CookieArgs()

	var lastErr error
	for _, f := range formats {
		if url, err := tryResolve(parent, id, f, cookies); err == nil {
			return url, nil
		} else {
			lastErr = err
		}
	}

	// If cookies were enabled and all formats failed, retry primary format
	// without cookies in case the cookie auth itself is the problem.
	if len(cookies) > 0 {
		if url, err := tryResolve(parent, id, "bestaudio", nil); err == nil {
			return url, nil
		} else {
			lastErr = err
		}
	}

	return "", lastErr
}

// tryResolve attempts a single yt-dlp --get-url call for the given format and cookie config.
func tryResolve(parent context.Context, id, format string, cookies []string) (string, error) {
	if err := parent.Err(); err != nil {
		return "", fmt.Errorf("cancelled: %w", err)
	}

	args := []string{"-f", format, "--get-url", id, "--no-warnings", "--extractor-retries", "3"}
	args = append(args, cookies...)
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		return "", fmt.Errorf("yt-dlp get-url failed for %s (format %s): %w (%s)", id, format, err, errMsg)
	}

	url := strings.TrimSpace(stdout.String())
	if url == "" {
		return "", fmt.Errorf("yt-dlp returned empty URL for %s (format %s)", id, format)
	}
	return url, nil
}
