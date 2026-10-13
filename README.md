# tg-toolbox

Small, dependency-free Go CLI for recent Telegram bot messages.

## Install and run

Requires Go 1.22+.

```sh
go build -o tg-toolbox .
export TG_BOT_KEY='your-bot-token'
./tg-toolbox messages recent
```

Or install into your Go bin directory (add it to `PATH`):

```sh
go install .
tg-toolbox messages recent
```

Output is newest first, from the last 5 minutes:

```text
Dan (id=123) · Chat Name (id=-456) · 2m ago
> Message Text
```

Multiline messages have each line prefixed with `> `. Media captions are shown; other non-text messages get a placeholder. Channel posts and anonymous group senders use the sending chat's name and ID.

## Telegram limitations

- Bots cannot fetch chat history. This command reads the **first 100 pending updates**, not necessarily the newest 100. If there is an older backlog, recent messages may be missing.
- Updates are not acknowledged, so repeated runs can show the same messages. Telegram retains pending updates for at most 24 hours.
- Another bot process can consume updates before this command sees them. Do not run concurrent `getUpdates` consumers.
- `getUpdates` does not work while a webhook is configured. Remove it first if you want to use this CLI; the CLI does not change webhook settings.
- To see ordinary group messages, disable group privacy via BotFather (`/setprivacy`), then remove and re-add the bot to the group if needed. Bots do not receive messages sent by other bots.
- Only new messages and channel posts are displayed; edits and other update types are ignored.

## Test

```sh
go test ./...
```
