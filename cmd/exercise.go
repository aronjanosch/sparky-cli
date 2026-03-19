package cmd

import (
	"encoding/json"
	"fmt"
	"time"
)

type ExerciseCmd struct {
	Search ExerciseSearchCmd `cmd:"" help:"Search for exercises (local first, falls back to Free Exercise DB)."`
	Log    ExerciseLogCmd    `cmd:"" help:"Log an exercise entry."`
	Diary  ExerciseDiaryCmd  `cmd:"" help:"View exercise diary for a date."`
	Delete ExerciseDeleteCmd `cmd:"" help:"Delete an exercise diary entry by ID."`
}

// ── Search ────────────────────────────────────────────────────────────────────

type ExerciseSearchCmd struct {
	Term  string `arg:"" help:"Exercise term to search for."`
	Limit int    `short:"l" default:"10" help:"Max results to show."`
}

func searchExercisesLocal(ctx *Context, term string) ([]map[string]any, error) {
	raw, err := ctx.Client().Get("/exercises/search", map[string]string{
		"searchTerm": term,
	})
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func searchExercisesExternal(ctx *Context, term string, limit int) ([]map[string]any, error) {
	providerID, err := resolveProviderID(ctx, "free-exercise-db")
	if err != nil {
		return nil, err
	}
	raw, err := ctx.Client().Get("/exercises/search-external", map[string]string{
		"query":        term,
		"providerId":   providerID,
		"providerType": "free-exercise-db",
		"limit":        fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// importExternalExercise saves an externally-found exercise to the local Sparky DB and returns it.
func importExternalExercise(ctx *Context, ex map[string]any) (map[string]any, error) {
	raw, err := ctx.Client().Post("/exercises", ex)
	if err != nil {
		return nil, err
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, err
	}
	return created, nil
}

func printExerciseTable(results []map[string]any) {
	fmt.Printf("%-36s  %-32s  %-16s  %s\n", "ID", "Name", "Category", "Muscle Groups")
	fmt.Printf("%-36s  %-32s  %-16s  %s\n", "----", "----", "--------", "-------------")
	for _, ex := range results {
		id := strVal(ex, "id", "provider_external_id")
		name := strVal(ex, "name")
		if len(name) > 32 {
			name = name[:29] + "..."
		}
		category := strVal(ex, "category")
		if len(category) > 16 {
			category = category[:13] + "..."
		}
		muscles := ""
		if mg, ok := ex["muscle_groups"]; ok {
			if arr, ok := mg.([]any); ok {
				for i, m := range arr {
					if i > 0 {
						muscles += ", "
					}
					muscles += fmt.Sprintf("%v", m)
				}
			}
		}
		fmt.Printf("%-36s  %-32s  %-16s  %s\n", id, name, category, muscles)
	}
}

func (e *ExerciseSearchCmd) Run(ctx *Context) error {
	results, err := searchExercisesLocal(ctx, e.Term)
	if err != nil {
		return err
	}
	if e.Limit > 0 && len(results) > e.Limit {
		results = results[:e.Limit]
	}

	if len(results) > 0 {
		if ctx.JSON {
			raw, _ := json.Marshal(results)
			fmt.Println(string(raw))
			return nil
		}
		printExerciseTable(results)
		return nil
	}

	// Fall back to Free Exercise DB
	if !ctx.JSON {
		fmt.Printf("No local results for %q — searching Free Exercise DB online...\n\n", e.Term)
	}
	extResults, err := searchExercisesExternal(ctx, e.Term, e.Limit)
	if err != nil {
		return fmt.Errorf("external search failed: %w", err)
	}
	if len(extResults) == 0 {
		fmt.Println("No results found.")
		return nil
	}
	if ctx.JSON {
		raw, _ := json.Marshal(extResults)
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println("[online — Free Exercise DB]")
	printExerciseTable(extResults)
	return nil
}

// ── Log ───────────────────────────────────────────────────────────────────────

type ExerciseLogCmd struct {
	Name     string `arg:"" help:"Exercise name to search and log."`
	Date     string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
	Duration int    `name:"duration" default:"30" help:"Duration in minutes."`
	Calories int    `name:"calories" default:"0" help:"Calories burned."`
}

func (e *ExerciseLogCmd) Run(ctx *Context) error {
	date := e.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	// Search locally first
	results, err := searchExercisesLocal(ctx, e.Name)
	if err != nil {
		return err
	}

	var exercise map[string]any
	if len(results) > 0 {
		exercise = results[0]
	} else {
		// Fall back to external search
		extResults, extErr := searchExercisesExternal(ctx, e.Name, 1)
		if extErr != nil {
			return fmt.Errorf("exercise %q not found locally; external search failed: %w", e.Name, extErr)
		}
		if len(extResults) == 0 {
			return fmt.Errorf("exercise %q not found — use 'sparky exercise search' to browse available exercises", e.Name)
		}
		// Import the external exercise into Sparky so it gets a local ID
		imported, importErr := importExternalExercise(ctx, extResults[0])
		if importErr != nil {
			return fmt.Errorf("found %q online but failed to add it to Sparky: %w", e.Name, importErr)
		}
		exercise = imported
		if !ctx.JSON {
			fmt.Printf("Added %q from Free Exercise DB to your library.\n", strVal(exercise, "name"))
		}
	}

	exerciseID := strVal(exercise, "id")
	exerciseName := strVal(exercise, "name")

	entry := map[string]any{
		"user_id":          userID,
		"exercise_id":      exerciseID,
		"entry_date":       date,
		"duration_minutes": e.Duration,
		"calories_burned":  e.Calories,
	}

	raw, err := ctx.Client().Post("/exercise-entries", entry)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	fmt.Printf("Logged: %s — %d min on %s  [%d kcal]\n", exerciseName, e.Duration, date, e.Calories)
	return nil
}

// ── Diary ─────────────────────────────────────────────────────────────────────

type ExerciseDiaryCmd struct {
	Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (e *ExerciseDiaryCmd) Run(ctx *Context) error {
	date := e.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	raw, err := ctx.Client().Get("/exercise-entries/by-date", map[string]string{
		"date": date,
	})
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		fmt.Println(string(raw))
		return nil
	}

	if len(entries) == 0 {
		fmt.Printf("No exercise entries for %s.\n", date)
		return nil
	}

	fmt.Printf("Exercise diary — %s\n\n", date)
	fmt.Printf("%-8s  %-32s  %8s  %12s\n", "ID", "Exercise", "Duration", "Calories")
	fmt.Printf("%-8s  %-32s  %8s  %12s\n", "--------", "--------------------------------", "--------", "------------")

	var totalDur, totalCal float64
	for _, e := range entries {
		id := strVal(e, "id")
		if len(id) > 8 {
			id = id[:8]
		}
		name := strVal(e, "exercise_name")
		if len(name) > 32 {
			name = name[:29] + "..."
		}
		dur := floatVal(e, "duration_minutes")
		cal := floatVal(e, "calories_burned")
		totalDur += dur
		totalCal += cal
		fmt.Printf("%-8s  %-32s  %7.0f m  %10.0f kc\n", id, name, dur, cal)
	}
	fmt.Printf("\n%-42s  %7.0f m  %10.0f kc\n", "TOTAL", totalDur, totalCal)
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

type ExerciseDeleteCmd struct {
	ID string `arg:"" help:"Entry ID to delete."`
}

func (e *ExerciseDeleteCmd) Run(ctx *Context) error {
	_, err := ctx.Client().Delete("/exercise-entries/" + e.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted entry %s.\n", e.ID)
	return nil
}
