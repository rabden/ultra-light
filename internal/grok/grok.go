// Package grok asks Grok through xAI's API (an OpenAI-style chat
// completions endpoint) and streams its answer, in Markdown, as it comes.
package grok

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Endpoint is xAI's chat completions.
	Endpoint = "https://api.x.ai/v1/chat/completions"
	// Model is xAI's alias of its newest Grok.
	Model = "grok-latest"
	// KeysURL is where people make an API key.
	KeysURL = "https://console.x.ai/"
)

// system asks for what fits a launcher's panel.
const system = "You answer questions typed into a desktop launcher. Answer directly and concisely, " +
	"leading with the answer. Use Markdown (headings, lists, code blocks, tables) when it helps."

// ErrKey is a key xAI refused (401, 403).
var ErrKey = errors.New("xAI refused the API key")

// Client asks Grok. Its zero value uses Endpoint, Model and
// http.DefaultClient.
type Client struct {
	Endpoint, Model string
	HTTP            *http.Client
}

// Ask sends question with key and calls delta with each piece of the answer
// as it streams in. It returns once the answer is complete, ctx is done, or
// the request failed.
func (c *Client) Ask(ctx context.Context, key, question string, delta func(string)) error {
	body, _ := json.Marshal(map[string]any{
		"model":  cmpOr(c.Model, Model),
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": question},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cmpOr(c.Endpoint, Endpoint), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: %s", ErrKey, apiError(b, resp.Status))
		}
		return errors.New(apiError(b, resp.Status))
	}
	return read(resp.Body, delta)
}

// read takes the server-sent events of a streamed completion.
func read(r io.Reader, delta func(string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			return nil
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		if len(ev.Error) > 0 && string(ev.Error) != "null" {
			return errors.New(apiError([]byte(data), "Grok stopped answering"))
		}
		for _, ch := range ev.Choices {
			if ch.Delta.Content != "" {
				delta(ch.Delta.Content)
			}
		}
	}
	return sc.Err()
}

// apiError finds the message in an error's body: {"error": "…"} or
// {"error": {"message": "…"}}; else fallback.
func apiError(b []byte, fallback string) string {
	var v struct {
		Error json.RawMessage `json:"error"`
		Code  string          `json:"code"`
	}
	if json.Unmarshal(b, &v) == nil && len(v.Error) > 0 {
		var s string
		if json.Unmarshal(v.Error, &s) == nil && s != "" {
			return s
		}
		var o struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(v.Error, &o) == nil && o.Message != "" {
			return o.Message
		}
	}
	if s := strings.TrimSpace(string(b)); s != "" && len(s) < 200 && !strings.HasPrefix(s, "<") {
		return s
	}
	return fallback
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
