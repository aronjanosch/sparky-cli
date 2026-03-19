---
name: sparky
description: SparkyFitness CLI for food diary, exercise tracking, biometric check-ins, and health summaries.
homepage: https://github.com/CodeWithCJ/SparkyFitness
metadata: {"clawdbot":{"emoji":"🏃","requires":{"bins":["sparky"]}}}
---

# sparky

Use `sparky` to interact with a self-hosted SparkyFitness server — log food, exercise, weight, steps, and mood.

Install
- Download a pre-built binary from https://github.com/aron/sparky-cli/releases and place it in your PATH, or
- Build from source (requires Go 1.21+):
  ```
  git clone https://github.com/aron/sparky-cli
  cd sparky-cli
  go build -o sparky .
  sudo mv sparky /usr/local/bin/
  ```

Setup (once)
- `sparky config set-url <url>` — e.g. `sparky config set-url https://sparky.example.com`
- `sparky config set-key <key>`
- `sparky config show`
- `sparky ping` — verify connection

Food
- Search: `sparky food search "chicken breast" [-l 10]`
- Log: `sparky food log "chicken breast" -m lunch -q 150 -u g [-d YYYY-MM-DD]`
- Diary: `sparky food diary [-d YYYY-MM-DD]`
- Delete: `sparky food delete <uuid>`

Exercise
- Search: `sparky exercise search running [-l 10]`
- Log: `sparky exercise log running --duration 45 --calories 400 [-d YYYY-MM-DD]`
- Diary: `sparky exercise diary [-d YYYY-MM-DD]`
- Delete: `sparky exercise delete <uuid>`

Check-ins
- Weight: `sparky checkin weight 75.5 [-u kg|lbs] [-d YYYY-MM-DD]`
- Steps: `sparky checkin steps 9500 [-d YYYY-MM-DD]`
- Mood: `sparky checkin mood 8 [-n "notes"] [-d YYYY-MM-DD]`
- Diary: `sparky checkin diary [-d YYYY-MM-DD]` — shows biometrics + mood together

Summary & trends
- `sparky summary [-s YYYY-MM-DD] [-e YYYY-MM-DD]` — nutrition/exercise/wellbeing totals (default: last 7 days)
- `sparky trends [-n 30]` — day-by-day nutrition table

Notes
- `-j` / `--json` is a **root-level flag**: `sparky -j food diary`, not `sparky food diary -j`
- `food log` and `exercise log` always pick the first search result — run `search` first to confirm
- Weight is stored in kg; lbs are auto-converted (`166 lbs → 75.30 kg`)
- Full UUIDs for delete: `sparky -j food diary | jq '.[0].id'`
- Meal options: `breakfast`, `lunch`, `dinner`, `snacks` (default: `snacks`)
