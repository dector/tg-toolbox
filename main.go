package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type person struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Title     string `json:"title"`
}

func (p person) name() string {
	if p.Title != "" {
		return p.Title
	}
	if name := strings.TrimSpace(p.FirstName + " " + p.LastName); name != "" {
		return name
	}
	if p.Username != "" {
		return "@" + p.Username
	}
	return "Unknown"
}

type message struct {
	ID         int64   `json:"message_id"`
	Date       int64   `json:"date"`
	From       *person `json:"from"`
	SenderChat *person `json:"sender_chat"`
	Chat       person  `json:"chat"`
	Text       string  `json:"text"`
	Caption    string  `json:"caption"`
}

type update struct {
	ID          int64    `json:"update_id"`
	Message     *message `json:"message"`
	ChannelPost *message `json:"channel_post"`
}

func fetchUpdates(ctx context.Context, client *http.Client, baseURL, key string) ([]update, error) {
	// No offset: do not acknowledge updates or discard another consumer's backlog.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/bot"+key+"/getUpdates?limit=100&timeout=0", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create Telegram request")
	}
	resp, err := client.Do(req)
	if err != nil {
		// HTTP errors can contain the URL, which includes the secret token.
		return nil, fmt.Errorf("cannot reach Telegram (check network connectivity)")
	}
	defer resp.Body.Close()
	var result struct {
		OK        bool     `json:"ok"`
		Result    []update `json:"result"`
		ErrorCode int      `json:"error_code"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid Telegram response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || !result.OK {
		code := result.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		switch code {
		case 401, 404:
			return nil, fmt.Errorf("Telegram rejected TG_BOT_KEY (code %d)", code)
		case 409:
			return nil, fmt.Errorf("Telegram conflict: disable the bot's webhook and stop any other getUpdates consumer")
		default:
			return nil, fmt.Errorf("Telegram API failed (code %d)", code)
		}
	}
	return result.Result, nil
}

func showRecent(w io.Writer, updates []update, now time.Time) error {
	var messages []message
	for _, u := range updates {
		m := u.Message
		if m == nil {
			m = u.ChannelPost
		}
		if m == nil {
			continue
		}
		date := time.Unix(m.Date, 0)
		if date.Before(now.Add(-5*time.Minute)) || date.After(now) {
			continue
		}
		messages = append(messages, *m)
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].Date > messages[j].Date })
	if len(messages) == 0 {
		_, err := fmt.Fprintln(w, "No messages from the last 5 minutes in pending updates.")
		return err
	}
	for i, m := range messages {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		sender := person{}
		if m.From != nil {
			sender = *m.From
		}
		if m.SenderChat != nil {
			sender = *m.SenderChat
		}
		age := now.Sub(time.Unix(m.Date, 0))
		ago := fmt.Sprintf("%ds ago", int(age.Seconds()))
		if age >= time.Minute {
			ago = fmt.Sprintf("%dm ago", int(age.Minutes()))
		}
		if _, err := fmt.Fprintf(w, "%s (id=%d) · %s (id=%d) · %s\n", sender.name(), sender.ID, m.Chat.name(), m.Chat.ID, ago); err != nil {
			return err
		}
		text := m.Text
		if text == "" {
			text = m.Caption
		}
		if text == "" {
			text = "[non-text message]"
		}
		for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if _, err := fmt.Fprintf(w, "> %s\n", line); err != nil {
				return err
			}
		}
	}
	return nil
}

func run(args []string, key string, w io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, err := fmt.Fprintln(w, "Usage: TG_BOT_KEY=<token> tg-toolbox messages recent")
		return err
	}
	if len(args) != 2 || args[0] != "messages" || args[1] != "recent" {
		return fmt.Errorf("usage: tg-toolbox messages recent")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("TG_BOT_KEY is not set. Set it to your Telegram bot token from @BotFather:\n  export TG_BOT_KEY='your-bot-token'\nThen run: tg-toolbox messages recent")
	}
	client := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	updates, err := fetchUpdates(context.Background(), client, "https://api.telegram.org", key)
	if err != nil {
		return err
	}
	return showRecent(w, updates, time.Now())
}

func main() {
	if err := run(os.Args[1:], os.Getenv("TG_BOT_KEY"), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
