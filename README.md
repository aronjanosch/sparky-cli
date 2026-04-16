# sparky-cli

A CLI for [SparkyFitness](https://github.com/CodeWithCJ/SparkyFitness) — log food, exercise, weight, steps, and mood from your terminal.

## Install

**Homebrew** (macOS/Linux):
```bash
brew tap aronjanosch/tap
brew install sparky-cli
```

**From source** (requires Go 1.21+):
```bash
git clone https://github.com/aronjanosch/sparky-cli
cd sparky-cli
go build -o sparky .
sudo mv sparky /usr/local/bin/
```

**Pre-built binaries** on the [Releases](https://github.com/aronjanosch/sparky-cli/releases) page (Linux, macOS, Windows — amd64/arm64).

## Setup

```bash
sparky config set-url https://your-sparky-instance.example.com
sparky config set-key <your-api-key>
sparky ping   # verify connection
```

Config is stored at `~/.config/sparky/config.json`.

## Commands

> `-j` / `--json` is a **root-level** flag — place it right after `sparky`:
> `sparky -j food diary`, not `sparky food diary -j`

### food
```bash
sparky food search "chicken breast" [-l 10] [--internal]
sparky food log "chicken breast" -m lunch -q 150 -u g [-d YYYY-MM-DD]
sparky food diary [-d YYYY-MM-DD]
sparky food delete <uuid>
```
`-m` meal options: `breakfast`, `lunch`, `dinner`, `snacks` (default: `snacks`)
`food search` checks your local library first; if nothing is found it falls back to **Open Food Facts** automatically, unless --internal flag is set.
`food log` picks the first match and auto-imports from Open Food Facts if the food isn't in your library yet.

### exercise
```bash
sparky exercise search running [-l 10]
sparky exercise log running --duration 45 --calories 400 [-d YYYY-MM-DD]
sparky exercise diary [-d YYYY-MM-DD]
sparky exercise delete <uuid>
```
`exercise search` checks your local library first; if nothing is found it falls back to external providers (**Free Exercise DB**, then `wger`) automatically.
`exercise log` picks the first match and auto-imports it from the available external provider if the exercise isn't in your library yet.

### checkin
```bash
sparky checkin weight 75.5 [-u kg|lbs] [-d YYYY-MM-DD]   # lbs auto-converted to kg
sparky checkin steps 9500 [-d YYYY-MM-DD]
sparky checkin mood 8 [-n "notes"] [-d YYYY-MM-DD]        # score 1–10
sparky checkin diary [-d YYYY-MM-DD]                       # biometrics + mood together
```

### summary & trends
```bash
sparky summary [-s YYYY-MM-DD] [-e YYYY-MM-DD]   # default: last 7 days
sparky trends [-n 30]                              # day-by-day nutrition table
```

## Releasing a new version

```bash
git tag v0.x.0
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

This builds binaries for all platforms, publishes a GitHub release, and updates the Homebrew formula in `aronjanosch/homebrew-tap` automatically.

## OpenClaw skill

`skill/SKILL.md` packages this CLI as an [OpenClaw](https://github.com/steipete/openclaw) skill so Claude can invoke it directly.

---

## Development

### Project structure

```
sparky-cli/
├── main.go                     # entry point — wires Kong, builds Context
├── cmd/
│   ├── root.go                 # CLI struct (all top-level commands), Context type
│   ├── helpers.go              # shared: resolveUserID(), strVal(), floatVal()
│   ├── config.go               # config set-url / set-key / show
│   ├── ping.go                 # GET /identity/user
│   ├── food.go                 # food search / log / diary / delete
│   ├── exercise.go             # exercise search / log / diary / delete
│   ├── checkin.go              # checkin weight / steps / mood / diary
│   └── summary.go              # summary, trends
├── internal/
│   ├── client/client.go        # REST client: Get/Post/Put/Delete + x-api-key header
│   └── config/config.go        # load/save ~/.config/sparky/config.json
└── skill/SKILL.md              # OpenClaw skill definition
```

### Adding a new command

1. Create `cmd/yourfeature.go` in package `cmd`
2. Define a top-level struct and subcommand structs with Kong tags:
```go
type YourCmd struct {
    Sub YourSubCmd `cmd:"" help:"Does something."`
}

type YourSubCmd struct {
    Name string `arg:"" help:"The name."`
    Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *YourSubCmd) Run(ctx *Context) error {
    // ctx.Client().Get/Post/Put/Delete(path, ...)
    // ctx.JSON — true if -j flag was passed
    // resolveUserID(ctx) — fetches activeUserId from /identity/user
    // strVal(map, "key"), floatVal(map, "key") — safe map accessors
}
```
3. Register it in `cmd/root.go`:
```go
type CLI struct {
    ...
    YourFeature YourCmd `cmd:"" help:"Short description."`
    ...
}
```
4. Build and test: `go build -o sparky . && ./sparky yourfeature --help`

### Shared helpers (`cmd/helpers.go`)

| Helper | Purpose |
|--------|---------|
| `resolveUserID(ctx)` | `GET /identity/user` → returns `activeUserId` string |
| `resolveProviderID(ctx, type)` | `GET /external-providers` → returns ID for the given `provider_type` |
| `strVal(m, keys...)` | Safe string extraction from `map[string]any`; tries keys in order |
| `floatVal(m, keys...)` | Safe float64 extraction; handles `float64` and `int` JSON types |

### API base URL

`internal/client` prepends `<config.URL>/api` to every path. All endpoint paths in cmd files are relative to that base (e.g. `/food-entries`, `/exercises/search`).

### Key API endpoints

| Command | Method | Path |
|---------|--------|------|
| ping | GET | `/identity/user` |
| food search (local) | GET | `/foods/foods-paginated?searchTerm=...` |
| food search (online) | GET | `/v2/foods/search/openfoodfacts?query=...&autoScale=true` |
| food import | POST | `/foods` |
| food log | POST | `/food-entries` |
| food diary | GET | `/food-entries/by-date/{date}` |
| food delete | DELETE | `/food-entries/{id}` |
| exercise search (local) | GET | `/exercises/search?searchTerm=...` |
| exercise search (online) | GET | `/exercises/search-external?query=...&providerId=...&providerType=free-exercise-db` |
| exercise import | POST | `/exercises` |
| exercise log | POST | `/exercise-entries` |
| exercise diary | GET | `/exercise-entries/by-date?date=...` |
| exercise delete | DELETE | `/exercise-entries/{id}` |
| checkin weight/steps | POST | `/measurements/check-in` |
| checkin diary | GET | `/measurements/check-in/{date}` |
| mood log | POST | `/mood` |
| mood get | GET | `/mood/date/{date}` |
| summary | GET | `/reports?startDate=...&endDate=...` |
| trends | GET | `/reports/nutrition-trends-with-goals?days=N` |
