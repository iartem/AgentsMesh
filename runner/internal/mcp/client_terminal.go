package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// Terminal Operations

// ObserveTerminal gets terminal output from another pod.
func (c *BackendClient) ObserveTerminal(ctx context.Context, podKey string, lines int, raw bool, includeScreen bool) (*tools.TerminalOutput, error) {
	params := url.Values{}
	params.Set("lines", strconv.Itoa(lines))
	params.Set("raw", strconv.FormatBool(raw))
	params.Set("include_screen", strconv.FormatBool(includeScreen))

	path := fmt.Sprintf("%s/pods/%s/terminal/observe?%s", c.podAPIPath(), url.PathEscape(podKey), params.Encode())

	var result tools.TerminalOutput
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendTerminalText sends text input to a terminal.
func (c *BackendClient) SendTerminalText(ctx context.Context, podKey string, text string) error {
	body := map[string]interface{}{
		"input": text,
	}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("%s/pods/%s/terminal/input", c.podAPIPath(), url.PathEscape(podKey)), body, nil)
}

// SendTerminalKey sends special keys to a terminal.
func (c *BackendClient) SendTerminalKey(ctx context.Context, podKey string, keys []string) error {
	// Convert keys to escape sequences and concatenate
	input := convertKeysToInput(keys)
	body := map[string]interface{}{
		"input": input,
	}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("%s/pods/%s/terminal/input", c.podAPIPath(), url.PathEscape(podKey)), body, nil)
}

// convertKeysToInput converts key names to terminal escape sequences.
func convertKeysToInput(keys []string) string {
	var result string
	for _, key := range keys {
		switch key {
		case "enter":
			result += "\r"
		case "escape":
			result += "\x1b"
		case "tab":
			result += "\t"
		case "backspace":
			result += "\x7f"
		case "delete":
			result += "\x1b[3~"
		case "ctrl+c":
			result += "\x03"
		case "ctrl+d":
			result += "\x04"
		case "ctrl+u":
			result += "\x15"
		case "ctrl+l":
			result += "\x0c"
		case "ctrl+z":
			result += "\x1a"
		case "ctrl+a":
			result += "\x01"
		case "ctrl+e":
			result += "\x05"
		case "ctrl+k":
			result += "\x0b"
		case "ctrl+w":
			result += "\x17"
		case "up":
			result += "\x1b[A"
		case "down":
			result += "\x1b[B"
		case "left":
			result += "\x1b[D"
		case "right":
			result += "\x1b[C"
		case "home":
			result += "\x1b[H"
		case "end":
			result += "\x1b[F"
		case "pageup":
			result += "\x1b[5~"
		case "pagedown":
			result += "\x1b[6~"
		case "shift+tab":
			result += "\x1b[Z"
		default:
			// Single character keys
			if len(key) == 1 {
				result += key
			}
		}
	}
	return result
}
