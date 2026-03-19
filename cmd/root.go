package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/aron/sparky-cli/internal/client"
	"github.com/aron/sparky-cli/internal/config"
)

// Context is passed to every command's Run method.
type Context struct {
	Config     *config.Config
	ConfigPath string
	JSON       bool
	client     *client.Client
}

func (c *Context) Client() *client.Client {
	if c.client == nil {
		if c.Config.URL == "" {
			fmt.Fprintln(os.Stderr, "error: server URL not configured. Run: sparky config set-url <url>")
			os.Exit(1)
		}
		base := strings.TrimRight(c.Config.URL, "/") + "/api"
		c.client = client.New(base, c.Config.APIKey)
	}
	return c.client
}

// CLI is the root command struct parsed by Kong.
type CLI struct {
	Config   ConfigCmd   `cmd:"" help:"Manage sparky-cli configuration."`
	Ping     PingCmd     `cmd:"" help:"Test connection to the Sparky server."`
	Food     FoodCmd     `cmd:"" help:"Search, log, and view food entries."`
	Exercise ExerciseCmd `cmd:"" help:"Search, log, and view exercise entries."`
	Checkin  CheckinCmd  `cmd:"" help:"Log and view daily check-ins and biometrics."`
	Summary  SummaryCmd  `cmd:"" help:"Show nutrition and exercise summary."`
	Trends   TrendsCmd   `cmd:"" help:"Show nutrition trends over time."`

	Version    kong.VersionFlag `short:"v" name:"version" help:"Print version and exit."`
	JSON       bool             `short:"j" help:"Output raw JSON."`
	ConfigFile string           `short:"c" name:"config" help:"Path to config file." type:"path"`
}
