---
name: sparky
description: SparkyFitness CLI for food diary, exercise tracking, biometric check-ins, and health summaries.
homepage: https://github.com/CodeWithCJ/SparkyFitness
metadata: {"clawdbot":{"emoji":"🏃","requires":{"bins":["sparky"]}}}
---

# sparky

Use `sparky` to interact with a self-hosted SparkyFitness server — log food, exercise, weight, steps, and mood.

Install
- Homebrew (macOS/Linux): `brew tap aronjanosch/tap && brew install sparky-cli`
- Build from source (requires Go 1.21+):
  ```
  git clone https://github.com/aronjanosch/sparky-cli
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
- Search: `sparky food search "chicken breast" [-l 10]` — local DB first, falls back to Open Food Facts automatically
- Log by name: `sparky food log "chicken breast" -m lunch -q 150 -u g [-d YYYY-MM-DD]` — auto-imports if not found locally
- Log by ID: `sparky food log --id <uuid> -m lunch -q 150 -u g` — skips search, unambiguous
- Create custom: `sparky food create "My Meal" --calories 450 --protein 28 --carbs 42 --fat 16` — adds a custom food to your library; defaults to 100g; optional: --fiber, --sugar, --sodium, --saturated-fat, --brand
- Diary: `sparky food diary [-d YYYY-MM-DD]`
- Delete: `sparky food delete <uuid>`

Exercise
- Search: `sparky exercise search "bench press" [-l 10]` — local DB first, falls back to Free Exercise DB
- Search external only: `sparky exercise search --external "pushup"` — bypasses local cache
- Log by name: `sparky exercise log "Pushups" [--duration 45] [--calories 400] [-d YYYY-MM-DD]`
- Log by ID: `sparky exercise log --id <uuid> --set 10x80@8 --set 10x80@9` — skips search, unambiguous
- Sets format: `REPS[xWEIGHT][@RPE]` — e.g. `10x80@8` = 10 reps, 80 kg, RPE 8; `10x80` or `10@8` also valid
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

Agentic workflow (always prefer --id to avoid ambiguity)

Exercise — search first, then log by ID:
```
# 1. Find candidates; use --external to bypass local cache if needed
sparky -j exercise search --external "pushup"
# Each result has is_local: true/false
#   is_local: true  → id is a UUID → use --id directly
#   is_local: false → id is a source string → log by exact name to import first,
#                     then search again to get the UUID

# 2a. Local exercise
sparky -j exercise log --id <uuid> --set 3x10@8

# 2b. External exercise (import on first log, then switch to --id)
sparky -j exercise log "Pushups" --set 3x10
sparky -j exercise search "Pushups"        # now is_local: true
sparky -j exercise log --id <uuid> --set 3x10
```

Food — same pattern:
```
sparky -j food search "chicken breast"
# result has id (UUID) if local, provider_external_id if external
sparky -j food log --id <uuid> -q 150 -m lunch
```

Custom food (when you have nutrition facts and it's not in the DB):
```
# Ingredients/beverages — nutrition per 100g/ml (default)
sparky -j food create "Craft Beer" --calories 43 --protein 0.5 --carbs 3.6 --fat 0 --serving-unit ml
sparky -j food log --id <uuid> -q 330 -m dinner

# Meals (Cookidoo, Chefkoch, etc.) — nutrition per serving, specify explicitly
sparky -j food create "Lasagna" --calories 450 --protein 28 --carbs 42 --fat 16 --serving-size 1 --serving-unit serving
sparky -j food log --id <uuid> -q 1 -m dinner
```

Notes
- `-j` / `--json` is a **root-level flag**: `sparky -j food diary`, not `sparky food diary -j`
- Logging by name triggers a fuzzy search — if no exact match, picks `results[0]` silently; use `--id` in scripts
- Both search commands fall back to online providers automatically; matches are added to your library on first log
- Weight is stored in kg; lbs are auto-converted (`166 lbs → 75.30 kg`)
- Full UUIDs for delete: `sparky -j food diary | jq '.[0].id'`
- Meal options: `breakfast`, `lunch`, `dinner`, `snacks` (default: `snacks`)
