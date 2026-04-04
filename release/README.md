# stand-Reminder

A lightweight Windows sedentary reminder written in Go.

## How It Works

- Polls Windows for the last keyboard or mouse input time.
- Accumulates active time while you are still using the computer.
- Resets the active timer when you have been idle long enough.
- Sends a Windows notification when the reminder threshold is reached.
- Runs as a tray app and serves a local control panel at `http://127.0.0.1:47831`.

## Config

Edit `config.json`:

```json
{
  "remind_after_minutes": 45,
  "idle_reset_minutes": 5,
  "check_interval_seconds": 5,
  "notification_title": "Stand Reminder",
  "notification_message": "You've been active for a while. Time to stand up and stretch."
}
```

## Local Run

1. Install Go 1.22 or newer.
2. Run `go mod tidy`.
3. Run `go build -ldflags="-H windowsgui" -o stand-reminder.exe .`
4. Launch `stand-reminder.exe`.
5. Click the tray icon to open the local console.

## Build

GitHub Actions builds the Windows executable with the Windows GUI subsystem and uploads it as an artifact.