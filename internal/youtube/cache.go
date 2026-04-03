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

// URLCache caches resolved audio stream URLs by YouTube video ID.
type URLCache struct {
	mu   sync.RWMutex
	urls map[string]string
}

var urlCache = &URLCache{urls: make(map[string]string)}

// Get returns the cached URL for a video ID, or empty string if not cached.
func (c *URLCache) Get(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.urls[id]
}

// Set stores an audio URL for a video ID.
func (c *URLCache) Set(id, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.urls[id] = url
}

// Invalidate removes a cached URL (e.g., when it expires).
func (c *URLCache) Invalidate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.urls, id)
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
	// Try formats in order: bestaudio, bestaudio*, best (fallback for videos without separate audio)
	formats := []string{"bestaudio", "bestaudio*", "best"}

	// Try with current cookie setting first, then fallback without cookies
	cookieConfigs := [][]string{CookieArgs()}
	if len(cookieConfigs[0]) > 0 {
		// If cookies are enabled, also try without as fallback
		cookieConfigs = append(cookieConfigs, nil)
	}

	var lastErr error
	for _, cookies := range cookieConfigs {
		for _, f := range formats {
			// Check if caller already cancelled before spawning another process
			if err := parent.Err(); err != nil {
				return "", fmt.Errorf("cancelled: %w", err)
			}

			args := []string{"-f", f, "--get-url", id, "--no-warnings", "--extractor-retries", "3"}
			args = append(args, cookies...)
			ctx, cancel := context.WithTimeout(parent, 30*time.Second)
			cmd := exec.CommandContext(ctx, "yt-dlp", args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				cancel()
				errMsg := strings.TrimSpace(stderr.String())
				lastErr = fmt.Errorf("yt-dlp get-url failed for %s (format %s): %w (%s)", id, f, err, errMsg)
				continue
			}

			url := strings.TrimSpace(stdout.String())
			if url == "" {
				cancel()
				lastErr = fmt.Errorf("yt-dlp returned empty URL for %s (format %s)", id, f)
				continue
			}
			cancel()
			return url, nil
		}
	}
	return "", lastErr
}
