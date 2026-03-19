package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/aron/sparky-cli/cmd"
	"github.com/aron/sparky-cli/internal/config"
)

var version = "dev"

func main() {
	cli := &cmd.CLI{}

	ctx := kong.Parse(cli,
		kong.Name("sparky"),
		kong.Description("A CLI for Sparky Fitness."),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)

	cfg, err := config.Load(cli.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	cmdCtx := &cmd.Context{
		Config:     cfg,
		ConfigPath: cli.ConfigFile,
		JSON:       cli.JSON,
	}

	ctx.FatalIfErrorf(ctx.Run(cmdCtx))
}
