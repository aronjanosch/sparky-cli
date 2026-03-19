# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git

- No `Co-Authored-By` trailers in commit messages
- Keep commit messages short and concise — one-line subject, no long body

## What This Is

`sparky-cli` is a Go CLI tool (binary: `sparky`) for interacting with a self-hosted [SparkyFitness](https://github.com/CodeWithCJ/SparkyFitness) server. It logs food, exercise, weight, steps, and mood from the terminal via REST API with API key authentication.

## Commands

```bash
# Build
go build -o sparky .

# Run
./sparky --help

# Release (tag first, then release)
git tag v0.x.0
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

There is no Makefile and no test suite currently.

## Architecture

Kong (`github.com/alecthomas/kong`) handles CLI parsing. Every command struct has a `Run(ctx *Context) error` method.

**Packages:**
- `cmd/` — all commands; `root.go` defines the top-level `CLI` struct and `Context` type
- `internal/client/` — thin HTTP client: adds `x-api-key` header, prepends `{baseURL}/api`, returns `json.RawMessage`
- `internal/config/` — `Config` struct (URL + APIKey), Load/Save to `~/.config/sparky/config.json`

**Context** (`cmd/root.go`) is passed to every command and holds:
- `Config` — loaded config
- `ConfigPath` — path to config file
- `JSON` — `-j` flag for raw JSON output
- `client` — lazy-initialized HTTP client

**Adding a new command:**
1. Create `cmd/<name>.go` with a struct embedding subcommand structs
2. Add a `Run(ctx *Context) error` method to each subcommand
3. Register it in the `CLI` struct in `cmd/root.go`

**Output pattern:** Every command checks `ctx.JSON` — if true, print raw JSON; if false, format as human-readable text.

**Nutrient scaling:** When logging food, nutrients on `default_variant` are per `serving_size` units. Scale before POST: `scale = requestedQty / serving_size`.

## API Base

All client paths are relative to `{config.URL}/api`. Key endpoints:

| Command | Method | Path |
|---------|--------|------|
| ping | GET | `/identity/user` |
| food search | GET | `/foods/foods-paginated?searchTerm=...` |
| food log | POST | `/food-entries` |
| food diary | GET | `/food-entries/by-date/{date}` |
| exercise search | GET | `/exercises/search?query=...` |
| exercise log | POST | `/exercise-entries` |
| checkin | POST/GET | `/measurements/check-in`, `/mood` |
| goals | GET | `/goals/for-date?date=&userId=` |
| summary | GET | `/reports?startDate=...&endDate=...` |
| trends | GET | `/reports/nutrition-trends-with-goals?days=N` |
