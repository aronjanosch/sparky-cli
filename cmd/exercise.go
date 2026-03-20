package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ExerciseCmd struct {
	Search ExerciseSearchCmd `cmd:"" help:"Search for exercises (local first, falls back to external providers)."`
	Log    ExerciseLogCmd    `cmd:"" help:"Log an exercise entry."`
	Diary  ExerciseDiaryCmd  `cmd:"" help:"View exercise diary for a date."`
	Delete ExerciseDeleteCmd `cmd:"" help:"Delete an exercise diary entry by ID."`
}

// ── Search ────────────────────────────────────────────────────────────────────

type ExerciseSearchCmd struct {
	Term     string `arg:"" help:"Exercise term to search for."`
	Limit    int    `short:"l" default:"10" help:"Max results to show."`
	External bool   `name:"external" short:"e" help:"Search external providers only (bypasses local cache)."`
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
	providers := []string{"free-exercise-db", "wger"}
	var lastErr error

	for _, providerType := range providers {
		providerID, err := resolveProviderID(ctx, providerType)
		if err != nil {
			lastErr = err
			continue
		}
		raw, err := ctx.Client().Get("/exercises/search-external", map[string]string{
			"query":        term,
			"providerId":   providerID,
			"providerType": providerType,
			"limit":        fmt.Sprintf("%d", limit),
		})
		if err != nil {
			lastErr = err
			continue
		}

		results, err := parseExternalExerciseResults(raw)
		if err != nil {
			lastErr = err
			continue
		}
		if len(results) == 0 {
			continue
		}
		for _, ex := range results {
			if cleanString(strVal(ex, "provider_type")) == "" {
				ex["provider_type"] = providerType
			}
		}
		return results, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return []map[string]any{}, nil
}

func parseExternalExerciseResults(raw []byte) ([]map[string]any, error) {
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err == nil {
		return results, nil
	}
	var wrapped struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return wrapped.Items, nil
	}
	return nil, fmt.Errorf("failed to parse external exercise response")
}

// importExternalExercise saves an externally-found exercise to the local Sparky DB and returns it.
// POST /exercises uses multer (multipart/form-data) and expects the payload as a JSON string
// in the "exerciseData" field — plain application/json is not accepted by that route.
func importExternalExercise(ctx *Context, ex map[string]any) (map[string]any, error) {
	payload, err := normalizeExternalExerciseForImport(ex)
	if err != nil {
		return nil, err
	}
	raw, err := ctx.Client().PostMultipartJSON("/exercises", "exerciseData", payload)
	if err != nil {
		return nil, err
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, err
	}
	return created, nil
}

func normalizeExternalExerciseForImport(ex map[string]any) (map[string]any, error) {
	name := cleanString(strVal(ex, "name"))
	if name == "" {
		return nil, fmt.Errorf("external exercise is missing required field %q", "name")
	}
	category := cleanString(strVal(ex, "category"))
	if category == "" {
		category = "Other"
	}

	// DB column is `source` (NOT NULL) — map from provider_type or source field
	source := cleanString(strVal(ex, "provider_type", "source"))
	if source == "" {
		source = "manual"
	}

	// DB columns use primary_muscles / secondary_muscles, not muscle_groups
	primaryMuscles := mergeUniqueStrings(
		toCleanStringSlice(ex["primary_muscles"]),
		toCleanStringSlice(ex["muscle_groups"]), // fallback when provider only gives a combined list
	)
	secondaryMuscles := toCleanStringSlice(ex["secondary_muscles"])

	payload := map[string]any{
		"name":              name,
		"category":          category,
		"source":            source,
		"source_id":         cleanString(strVal(ex, "provider_external_id", "source_id", "id")),
		"equipment":         toCleanStringSlice(ex["equipment"]),
		"primary_muscles":   primaryMuscles,
		"secondary_muscles": secondaryMuscles,
		"instructions":      toCleanStringSlice(ex["instructions"]),
		"images":            toCleanStringSlice(ex["images"]),
		"description":       cleanString(strVal(ex, "description")),
		"force":             cleanString(strVal(ex, "force")),
		"level":             cleanString(strVal(ex, "level")),
		"mechanic":          cleanString(strVal(ex, "mechanic")),
		"is_custom":         true,
		"shared_with_public": false,
	}

	return payload, nil
}

func toCleanStringSlice(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			if s := cleanString(fmt.Sprintf("%v", item)); s != "" {
				out = append(out, s)
			}
		}
	case []string:
		for _, item := range x {
			if s := cleanString(item); s != "" {
				out = append(out, s)
			}
		}
	case string:
		if s := cleanString(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "undefined") || strings.EqualFold(s, "null") {
		return ""
	}
	return s
}

func mergeUniqueStrings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, g := range groups {
		for _, s := range g {
			key := strings.ToLower(cleanString(s))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	return out
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

func stampLocal(results []map[string]any, local bool) {
	for _, r := range results {
		r["is_local"] = local
	}
}

func (e *ExerciseSearchCmd) Run(ctx *Context) error {
	if !e.External {
		results, err := searchExercisesLocal(ctx, e.Term)
		if err != nil {
			return err
		}
		if e.Limit > 0 && len(results) > e.Limit {
			results = results[:e.Limit]
		}
		if len(results) > 0 {
			stampLocal(results, true)
			if ctx.JSON {
				raw, _ := json.Marshal(results)
				fmt.Println(string(raw))
				return nil
			}
			printExerciseTable(results)
			return nil
		}
	}

	// External search — either forced via --external or local returned nothing
	if !ctx.JSON {
		if e.External {
			fmt.Printf("Searching external providers for %q...\n\n", e.Term)
		} else {
			fmt.Printf("No local results for %q — searching external providers online...\n\n", e.Term)
		}
	}
	extResults, err := searchExercisesExternal(ctx, e.Term, e.Limit)
	if err != nil {
		return fmt.Errorf("external search failed: %w", err)
	}
	if len(extResults) == 0 {
		if !ctx.JSON {
			fmt.Println("No results found.")
		}
		return nil
	}
	stampLocal(extResults, false)
	if ctx.JSON {
		raw, _ := json.Marshal(extResults)
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println("[online — external provider]")
	printExerciseTable(extResults)
	return nil
}

// ── Log ───────────────────────────────────────────────────────────────────────

type ExerciseLogCmd struct {
	Name     string   `arg:"" optional:"" help:"Exercise name to search and log."`
	ID       string   `name:"id" help:"Log by exercise ID directly (skips search — recommended for scripts/agents)."`
	Date     string   `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
	Duration int      `name:"duration" default:"0" help:"Duration in minutes (cardio)."`
	Calories int      `name:"calories" default:"0" help:"Calories burned."`
	Set      []string `name:"set" short:"s" help:"Set: REPS[xWEIGHT][@RPE] e.g. 10x12@7. Repeatable."`
	Notes    string   `name:"notes" short:"n" default:"" help:"Session notes."`
}

// parseSet converts "REPSxWEIGHT@RPE" (all parts optional except REPS) into a set map.
// Valid examples: "10x12@7", "10x12", "10@7", "10"
func parseSet(raw string, number int) (map[string]any, error) {
	s := strings.TrimSpace(raw)
	set := map[string]any{
		"set_number": number,
		"set_type":   "Working Set",
	}

	// strip @RPE suffix
	if idx := strings.LastIndex(s, "@"); idx >= 0 {
		rpe, err := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
		if err != nil {
			return nil, fmt.Errorf("invalid RPE in set %q: must be an integer", raw)
		}
		set["rpe"] = rpe
		s = s[:idx]
	}

	// split repsxweight
	parts := strings.SplitN(s, "x", 2)
	reps, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || reps <= 0 {
		return nil, fmt.Errorf("invalid set %q: expected format REPS[xWEIGHT][@RPE]", raw)
	}
	set["reps"] = reps

	if len(parts) == 2 {
		w, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weight in set %q: must be a number", raw)
		}
		set["weight"] = w
	}

	return set, nil
}

func (e *ExerciseLogCmd) Run(ctx *Context) error {
	if e.ID == "" && e.Name == "" {
		return fmt.Errorf("provide an exercise name or --id")
	}

	date := e.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	var exerciseID string
	var exerciseName string

	if e.ID != "" {
		// Direct ID path — no search, no ambiguity
		exerciseID = e.ID
	} else {
		// Search locally first (up to 5 for picker)
		results, err := searchExercisesLocal(ctx, e.Name)
		if err != nil {
			return err
		}

		var exercise map[string]any
		if len(results) > 0 {
			picked, pickErr := pickResult(e.Name, "exercise", results, "name", ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
			exercise = picked
		} else {
			// Fall back to external search — fetch up to 5 for picker
			extResults, extErr := searchExercisesExternal(ctx, e.Name, 5)
			if extErr != nil {
				return fmt.Errorf("exercise %q not found locally; external search failed: %w", e.Name, extErr)
			}
			if len(extResults) == 0 {
				return fmt.Errorf("exercise %q not found — use 'sparky exercise search' to browse", e.Name)
			}
			picked, pickErr := pickResult(e.Name, "exercise", extResults, "name", ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
			// Import the chosen external exercise into Sparky so it gets a local ID
			imported, importErr := importExternalExercise(ctx, picked)
			if importErr != nil {
				return fmt.Errorf("found %q online but failed to add it to Sparky: %w", e.Name, importErr)
			}
			exercise = imported
			if !ctx.JSON {
				source := cleanString(strVal(picked, "provider_type", "source"))
				if source == "" {
					source = "external provider"
				}
				fmt.Printf("Added %q from %s to your library.\n", strVal(exercise, "name"), source)
			}
		}

		exerciseID = strVal(exercise, "id")
		exerciseName = strVal(exercise, "name")
	}

	if exerciseID == "" {
		return fmt.Errorf("exercise %q has no local ID after import; try 'sparky exercise search %q' and import via web UI", exerciseName, e.Name)
	}

	// Parse sets
	var sets []map[string]any
	for i, raw := range e.Set {
		set, err := parseSet(raw, i+1)
		if err != nil {
			return err
		}
		sets = append(sets, set)
	}

	entry := map[string]any{
		"user_id":          userID,
		"exercise_id":      exerciseID,
		"entry_date":       date,
		"duration_minutes": e.Duration,
		"calories_burned":  e.Calories,
		"notes":            e.Notes,
		"distance":         nil,
		"avg_heart_rate":   nil,
	}
	if len(sets) > 0 {
		entry["sets"] = sets
	}

	raw, err := ctx.Client().Post("/exercise-entries", entry)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	// When logged by --id, pull name from the response
	if exerciseName == "" {
		var resp map[string]any
		if jsonErr := json.Unmarshal(raw, &resp); jsonErr == nil {
			exerciseName = strVal(resp, "exercise_name")
		}
		if exerciseName == "" {
			exerciseName = exerciseID
		}
	}

	if len(sets) > 0 {
		fmt.Printf("Logged: %s — %d set(s) on %s\n", exerciseName, len(sets), date)
		for _, s := range sets {
			reps, _ := s["reps"].(int)
			weight, hasWeight := s["weight"].(float64)
			rpe, hasRPE := s["rpe"].(int)
			line := fmt.Sprintf("  Set %d: %d reps", s["set_number"], reps)
			if hasWeight {
				line += fmt.Sprintf(" @ %.4gkg", weight)
			}
			if hasRPE {
				line += fmt.Sprintf("  RPE %d", rpe)
			}
			fmt.Println(line)
		}
	} else {
		fmt.Printf("Logged: %s — %d min on %s  [%d kcal]\n", exerciseName, e.Duration, date, e.Calories)
	}
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
		"selectedDate": date,
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

	var totalDur, totalCal float64
	for _, e := range entries {
		id := strVal(e, "id")
		if len(id) > 8 {
			id = id[:8]
		}
		name := strVal(e, "name")
		dur := floatVal(e, "duration_minutes")
		cal := floatVal(e, "calories_burned")
		notes := cleanString(strVal(e, "notes"))
		totalDur += dur
		totalCal += cal

		// Parse sets from response
		var sets []map[string]any
		if raw, ok := e["sets"]; ok {
			if arr, ok := raw.([]any); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]any); ok {
						sets = append(sets, m)
					}
				}
			}
		}

		if len(sets) > 0 {
			fmt.Printf("[%s] %s", id, name)
			if cal > 0 {
				fmt.Printf("  [%.0f kcal]", cal)
			}
			if notes != "" {
				fmt.Printf("  — %s", notes)
			}
			fmt.Println()
			for _, s := range sets {
				reps := floatVal(s, "reps")
				weight := floatVal(s, "weight")
				rpe := floatVal(s, "rpe")
				setNum := floatVal(s, "set_number")
				setNotes := cleanString(strVal(s, "notes"))
				line := fmt.Sprintf("    Set %.0f: %.0f reps", setNum, reps)
				if weight > 0 {
					line += fmt.Sprintf(" @ %.4gkg", weight)
				}
				if rpe > 0 {
					line += fmt.Sprintf("  RPE %.0f", rpe)
				}
				if setNotes != "" {
					line += fmt.Sprintf("  (%s)", setNotes)
				}
				fmt.Println(line)
			}
		} else {
			fmt.Printf("[%s] %-32s  %5.0f min  %6.0f kcal", id, name, dur, cal)
			if notes != "" {
				fmt.Printf("  — %s", notes)
			}
			fmt.Println()
		}
	}
	fmt.Printf("\nTotal: %.0f min  %.0f kcal\n", totalDur, totalCal)
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
