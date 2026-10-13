package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestShowRecent(t *testing.T) {
	now := time.Unix(1000, 0)
	makeMessage := func(date int64, text string) *message {
		return &message{Date: date, From: &person{ID: 123, FirstName: "Dan"}, Chat: person{ID: -456, Title: "Chat Name"}, Text: text}
	}
	updates := []update{
		{Message: makeMessage(880, "older\nsecond line")},
		{Message: makeMessage(699, "too old")},
		{Message: makeMessage(1001, "future")},
		{},
		{Message: makeMessage(990, "newer")},
	}
	var out bytes.Buffer
	if err := showRecent(&out, updates, now); err != nil {
		t.Fatal(err)
	}
	want := "Dan (id=123) · Chat Name (id=-456) · 10s ago\n> newer\n\nDan (id=123) · Chat Name (id=-456) · 2m ago\n> older\n> second line\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestCaptionSenderChatAndBoundary(t *testing.T) {
	now := time.Unix(1000, 0)
	updates := []update{{ChannelPost: &message{
		Date: 700, SenderChat: &person{ID: -12, Title: "Channel"},
		Chat: person{ID: -12, Title: "Channel"}, Caption: "photo caption",
	}}}
	var out bytes.Buffer
	if err := showRecent(&out, updates, now); err != nil {
		t.Fatal(err)
	}
	want := "Channel (id=-12) · Channel (id=-12) · 5m ago\n> photo caption\n"
	if out.String() != want {
		t.Fatalf("got %q", out.String())
	}
}

func TestEmptyAndNonText(t *testing.T) {
	var out bytes.Buffer
	if err := showRecent(&out, nil, time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No messages") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := showRecent(&out, []update{{Message: &message{Date: 1000}}}, time.Unix(1000, 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "> [non-text message]") {
		t.Fatal(out.String())
	}
}

func TestFetchUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botsecret/getUpdates" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Has("offset") {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, `{"ok":true,"result":[{"update_id":1,"message":{"date":1000,"text":"hello"}}]}`)
	}))
	defer server.Close()
	updates, err := fetchUpdates(context.Background(), server.Client(), server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].Message.Text != "hello" {
		t.Fatalf("unexpected updates: %+v", updates)
	}
}

func TestFetchErrors(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, want string
	}{
		{401, `{"ok":false,"error_code":401,"description":"secret"}`, "TG_BOT_KEY"},
		{409, `{"ok":false,"error_code":409}`, "webhook"},
		{429, `{"ok":false,"error_code":429}`, "429"},
		{502, "not JSON", "invalid Telegram response"},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			_, err := fetchUpdates(context.Background(), server.Client(), server.URL, "secret")
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchUpdates(ctx, http.DefaultClient, "https://api.telegram.org", "secret")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected transport error: %v", err)
	}
}

func TestRunValidation(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		key, want string
	}{
		{nil, "", "usage:"},
		{[]string{"messages", "recent"}, "", "TG_BOT_KEY"},
		{[]string{"messages", "recent", "extra"}, "secret", "usage:"},
	} {
		if err := run(tc.args, tc.key, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := run([]string{"--help"}, "", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
