# sparky-cli

A CLI for [SparkyFitness](https://github.com/CodeWithCJ/SparkyFitness) — log food, exercise, weight, steps, and mood from your terminal.

## Install

**From source** (requires Go 1.21+):
```bash
git clone https://github.com/aronjanosch/sparky-cli
cd sparky-cli
go build -o sparky .
sudo mv sparky /usr/local/bin/
```

**Pre-built binaries** are available on the [Releases](https://github.com/aronjanosch/sparky-cli/releases) page (Linux, macOS, Windows — amd64/arm64).

## Setup

```bash
sparky config set-url https://your-sparky-instance.example.com
sparky config set-key <your-api-key>
sparky ping   # verify connection
```

## Usage

```bash
# Food
sparky food search "chicken breast"
sparky food log "chicken breast" -m lunch -q 150 -u g
sparky food diary
sparky food delete <id>

# Exercise
sparky exercise search running
sparky exercise log running --duration 45 --calories 400
sparky exercise diary

# Check-ins
sparky checkin weight 75.5
sparky checkin steps 9500
sparky checkin mood 8 -n "feeling good"
sparky checkin diary

# Summary & trends
sparky summary
sparky trends --n 30
```

Use `-j` / `--json` (root-level flag) for raw JSON output:
```bash
sparky -j food diary | jq '.[0].id'
```

## OpenClaw skill

The `skill/sparky.md` file packages this CLI as an [OpenClaw](https://github.com/steipete/openclaw) skill so Claude can invoke it directly.
