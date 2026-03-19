# Development Guide

## Build & run

```bash
go build -o sparky .
./sparky --help
```

## Project structure

```
sparky-cli/
├── main.go                    # entry point — wires Kong, loads config, calls Run
├── cmd/
│   ├── root.go                # CLI struct (all top-level commands + global flags), Context
│   ├── helpers.go             # shared: resolveUserID, strVal, floatVal
│   ├── config.go              # config set-url / set-key / show
│   ├── ping.go                # sparky ping → GET /identity/user
│   ├── food.go                # food search / log / diary / delete
│   ├── exercise.go            # exercise search / log / diary / delete
│   ├── checkin.go             # checkin weight / steps / mood / diary
│   └── summary.go             # summary, trends
├── internal/
│   ├── client/client.go       # HTTP client — Get/Post/Put/Delete + x-api-key header
│   └── config/config.go       # load/save ~/.config/sparky/config.json
└── skill/SKILL.md             # OpenClaw skill definition
```

## Adding a new command group

1. Create `cmd/mycommand.go` in package `cmd`
2. Define a top-level struct and subcommand structs:
   ```go
   type MyCmd struct {
       Sub MySubCmd `cmd:"" help:"..."`
   }
   type MySubCmd struct {
       Arg string `arg:""`
       Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
   }
   func (c *MySubCmd) Run(ctx *Context) error { ... }
   ```
3. Wire it into `cmd/root.go`:
   ```go
   My MyCmd `cmd:"" help:"..."`
   ```

## Shared helpers (`cmd/helpers.go`)

| Helper | Purpose |
|---|---|
| `resolveUserID(ctx)` | GET /identity/user → `activeUserId` string |
| `strVal(m, keys...)` | safe string extraction from `map[string]any` |
| `floatVal(m, keys...)` | safe float64 extraction from `map[string]any` |

Food-specific helpers in `cmd/food.go`: `variantMap`, `resolveMealTypeID`.

## Client (`internal/client/client.go`)

```go
ctx.Client().Get(path, queryParams)   // map[string]string or nil
ctx.Client().Post(path, body)         // any → JSON
ctx.Client().Put(path, body)
ctx.Client().Delete(path)
```

All methods return `(json.RawMessage, error)`. Errors include the HTTP status code and body on 4xx/5xx.

Base URL is `<config.URL>/api` — never include `/api` in the path you pass.

## Output pattern

Every command follows the same shape:

```go
if ctx.JSON {
    fmt.Println(string(raw))
    return nil
}
// human-readable table/text below
```

The `-j` / `--json` flag is **root-level**: `sparky -j food diary`, not `sparky food diary -j`.

## API endpoints (live-tested)

| Area | Method | Path |
|---|---|---|
| Auth / user | GET | `/identity/user` |
| Food search | GET | `/foods/foods-paginated?searchTerm=&foodFilter=all&currentPage=1&itemsPerPage=N&sortBy=name:asc` |
| Food log | POST | `/food-entries` |
| Food diary | GET | `/food-entries/by-date/{date}` |
| Food delete | DELETE | `/food-entries/{id}` |
| Meal types | GET | `/meal-types` |
| Exercise search | GET | `/exercises/search?query=` |
| Exercise log | POST | `/exercise-entries` |
| Exercise diary | GET | `/exercise-entries/by-date?date=` |
| Exercise delete | DELETE | `/exercise-entries/{id}` |
| Biometrics log | POST | `/measurements/check-in` |
| Biometrics diary | GET | `/measurements/check-in/{date}` |
| Mood log | POST | `/mood` |
| Mood diary | GET | `/mood/date/{date}` |
| Report summary | GET | `/reports?startDate=&endDate=` |
| Nutrition trends | GET | `/reports/nutrition-trends-with-goals?days=N` |

> The OpenAPI spec in `docs/openapi.json` is outdated — use the backend source at `SparkyFitnessServer/routes/` or network inspection as ground truth.

## Food log scaling

Nutrient values on `default_variant` are per `serving_size` units.
Scale factor = `requestedQty / serving_size`. Applied to calories, protein, carbs, fat before POSTing.

## Release

```bash
git tag v0.x.0
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

Builds Linux/macOS/Windows × amd64/arm64, uploads archives + checksums to GitHub Releases.
Version is injected via `-X main.version={{.Version}}`.
