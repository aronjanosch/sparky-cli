package cmd

import (
	"fmt"
)

type ConfigCmd struct {
	SetURL SetURLCmd `cmd:"" name:"set-url" help:"Set the Sparky server URL."`
	SetKey SetKeyCmd `cmd:"" name:"set-key" help:"Set the API key."`
	Show   ShowCmd   `cmd:"" help:"Show current configuration."`
}

type SetURLCmd struct {
	URL string `arg:"" help:"Server URL (e.g. http://localhost:5435)."`
}

func (s *SetURLCmd) Run(ctx *Context) error {
	ctx.Config.URL = s.URL
	if err := ctx.Config.Save(ctx.ConfigPath); err != nil {
		return err
	}
	fmt.Printf("URL set to %s\n", s.URL)
	return nil
}

type SetKeyCmd struct {
	Key string `arg:"" help:"API key."`
}

func (s *SetKeyCmd) Run(ctx *Context) error {
	ctx.Config.APIKey = s.Key
	if err := ctx.Config.Save(ctx.ConfigPath); err != nil {
		return err
	}
	fmt.Println("API key saved.")
	return nil
}

type ShowCmd struct{}

func (s *ShowCmd) Run(ctx *Context) error {
	url := ctx.Config.URL
	if url == "" {
		url = "(not set)"
	}
	key := ctx.Config.APIKey
	masked := "(not set)"
	if key != "" {
		if len(key) > 8 {
			masked = key[:4] + "..." + key[len(key)-4:]
		} else {
			masked = "****"
		}
	}
	fmt.Printf("URL:     %s\n", url)
	fmt.Printf("API key: %s\n", masked)
	if ctx.ConfigPath != "" {
		fmt.Printf("Config:  %s\n", ctx.ConfigPath)
	}
	return nil
}

