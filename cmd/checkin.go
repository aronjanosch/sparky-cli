package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CheckinCmd struct {
	Weight CheckinWeightCmd `cmd:"" help:"Log body weight."`
	Steps  CheckinStepsCmd  `cmd:"" help:"Log step count."`
	Mood   CheckinMoodCmd   `cmd:"" help:"Log mood (1–10)."`
	Diary  CheckinDiaryCmd  `cmd:"" help:"View check-in diary for a date."`
}

// ── Weight ────────────────────────────────────────────────────────────────────

type CheckinWeightCmd struct {
	Value float64 `arg:"" help:"Weight value."`
	Unit  string  `short:"u" default:"kg" enum:"kg,lbs" help:"Unit: kg or lbs."`
	Date  string  `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *CheckinWeightCmd) Run(ctx *Context) error {
	date := c.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	weight := c.Value
	displayUnit := c.Unit
	if c.Unit == "lbs" {
		weight = weight * 0.453592
		displayUnit = "kg (converted from lbs)"
	}

	body := map[string]any{
		"user_id":    userID,
		"entry_date": date,
		"weight":     weight,
		"steps":      nil,
		"height":     nil,
		"neck":       nil,
		"waist":      nil,
		"hips":       nil,
		"body_fat":   nil,
	}

	raw, err := ctx.Client().Post("/measurements/check-in", body)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	fmt.Printf("Logged weight: %.2f %s on %s\n", weight, displayUnit, date)
	return nil
}

// ── Steps ─────────────────────────────────────────────────────────────────────

type CheckinStepsCmd struct {
	Value int    `arg:"" help:"Step count."`
	Date  string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *CheckinStepsCmd) Run(ctx *Context) error {
	date := c.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	body := map[string]any{
		"user_id":    userID,
		"entry_date": date,
		"weight":     nil,
		"steps":      c.Value,
		"height":     nil,
		"neck":       nil,
		"waist":      nil,
		"hips":       nil,
		"body_fat":   nil,
	}

	raw, err := ctx.Client().Post("/measurements/check-in", body)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	fmt.Printf("Logged steps: %d on %s\n", c.Value, date)
	return nil
}

// ── Mood ──────────────────────────────────────────────────────────────────────

type CheckinMoodCmd struct {
	Value int    `arg:"" help:"Mood score (1–10)."`
	Notes string `short:"n" default:"" help:"Optional notes."`
	Date  string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *CheckinMoodCmd) Run(ctx *Context) error {
	if c.Value < 1 || c.Value > 10 {
		return fmt.Errorf("mood value must be between 1 and 10, got %d", c.Value)
	}

	date := c.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	body := map[string]any{
		"user_id":    userID,
		"entry_date": date,
		"mood_value": c.Value,
		"notes":      c.Notes,
	}

	raw, err := ctx.Client().Post("/mood", body)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	fmt.Printf("Logged mood: %d/10 on %s\n", c.Value, date)
	return nil
}

// ── Diary ─────────────────────────────────────────────────────────────────────

type CheckinDiaryCmd struct {
	Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *CheckinDiaryCmd) Run(ctx *Context) error {
	date := c.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	bioRaw, bioErr := ctx.Client().Get("/measurements/check-in/"+date, nil)
	moodRaw, moodErr := ctx.Client().Get("/mood/date/"+date, nil)

	if ctx.JSON {
		var bioData, moodData any
		if bioErr == nil {
			_ = json.Unmarshal(bioRaw, &bioData)
		}
		if moodErr == nil {
			_ = json.Unmarshal(moodRaw, &moodData)
		}
		combined, _ := json.Marshal(map[string]any{
			"biometrics": bioData,
			"mood":       moodData,
		})
		fmt.Println(string(combined))
		return nil
	}

	fmt.Printf("Check-in — %s\n", date)

	fmt.Printf("\nBiometrics\n")
	if bioErr != nil {
		if strings.Contains(bioErr.Error(), "404") {
			fmt.Println("  No biometrics logged.")
		} else {
			fmt.Printf("  Error: %v\n", bioErr)
		}
	} else {
		var bio map[string]any
		if err := json.Unmarshal(bioRaw, &bio); err != nil || bio == nil {
			fmt.Println("  No biometrics logged.")
		} else {
			printOptionalField("Weight", floatVal(bio, "weight"), "kg")
			printOptionalSteps("Steps", bio)
			printOptionalField("Height", floatVal(bio, "height"), "cm")
			printOptionalField("Neck", floatVal(bio, "neck"), "cm")
			printOptionalField("Waist", floatVal(bio, "waist"), "cm")
			printOptionalField("Hips", floatVal(bio, "hips"), "cm")
			printOptionalField("Body fat", floatVal(bio, "body_fat"), "%")
		}
	}

	fmt.Printf("\nMood\n")
	if moodErr != nil {
		if strings.Contains(moodErr.Error(), "404") || strings.Contains(moodErr.Error(), "null") {
			fmt.Println("  No mood logged.")
		} else {
			fmt.Printf("  Error: %v\n", moodErr)
		}
	} else {
		var mood map[string]any
		if err := json.Unmarshal(moodRaw, &mood); err != nil || mood == nil {
			fmt.Println("  No mood logged.")
		} else {
			score := floatVal(mood, "mood_value")
			notes := strVal(mood, "notes")
			if score == 0 {
				fmt.Println("  No mood logged.")
			} else {
				fmt.Printf("  Score:  %.0f/10\n", score)
				if notes != "" {
					fmt.Printf("  Notes:  %s\n", notes)
				}
			}
		}
	}

	return nil
}

func printOptionalField(label string, val float64, unit string) {
	if val == 0 {
		fmt.Printf("  %-10s —\n", label+":")
	} else {
		fmt.Printf("  %-10s %.2f %s\n", label+":", val, unit)
	}
}

func printOptionalSteps(label string, bio map[string]any) {
	v, ok := bio["steps"]
	if !ok || v == nil {
		fmt.Printf("  %-10s —\n", label+":")
		return
	}
	switch n := v.(type) {
	case float64:
		fmt.Printf("  %-10s %.0f\n", label+":", n)
	case int:
		fmt.Printf("  %-10s %d\n", label+":", n)
	default:
		fmt.Printf("  %-10s —\n", label+":")
	}
}
