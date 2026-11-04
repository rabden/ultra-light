package grok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskStreams(t *testing.T) {
	var got struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct{ Role, Content string }
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, s := range []string{"# Top", "ology\n\n", "is **math**."} {
			b, _ := json.Marshal(s)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", b)
		}
		fmt.Fprint(w, ": keep-alive\n\ndata: {\"choices\":[{\"delta\":{}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	var out strings.Builder
	c := Client{Endpoint: srv.URL}
	if err := c.Ask(context.Background(), "k", "what is topology", func(s string) { out.WriteString(s) }); err != nil {
		t.Fatal(err)
	}
	if out.String() != "# Topology\n\nis **math**." {
		t.Errorf("answer %q", out.String())
	}
	if got.Model != Model || !got.Stream || len(got.Messages) != 2 || got.Messages[1].Content != "what is topology" {
		t.Errorf("request %+v", got)
	}
}

func TestAskErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		key    bool
		msg    string
	}{
		{401, `{"code":"x","error":"Incorrect API key provided"}`, true, "Incorrect API key provided"},
		{429, `{"error":{"message":"rate limited"}}`, false, "rate limited"},
		{500, `<html>oops</html>`, false, "500 Internal Server Error"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		}))
		c := Client{Endpoint: srv.URL}
		err := c.Ask(context.Background(), "k", "q", func(string) {})
		srv.Close()
		if err == nil || errors.Is(err, ErrKey) != tc.key || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%d: %v", tc.status, err)
		}
	}
}
