package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type FoodCmd struct {
	Search FoodSearchCmd `cmd:"" help:"Search for foods in the database."`
	Log    FoodLogCmd    `cmd:"" help:"Log a food entry."`
	Diary  FoodDiaryCmd  `cmd:"" help:"View food diary for a date."`
	Delete FoodDeleteCmd `cmd:"" help:"Delete a food diary entry by ID."`
}

// ── Search ────────────────────────────────────────────────────────────────────

type FoodSearchCmd struct {
	Name  string `arg:"" help:"Food name to search for."`
	Limit int    `short:"l" default:"10" help:"Max results to show."`
}

func searchFoodsLocal(ctx *Context, name string, limit int) ([]map[string]any, error) {
	raw, err := ctx.Client().Get("/foods/foods-paginated", map[string]string{
		"searchTerm":   name,
		"foodFilter":   "all",
		"currentPage":  "1",
		"itemsPerPage": fmt.Sprintf("%d", limit),
		"sortBy":       "name:asc",
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Foods []map[string]any `json:"foods"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Foods, nil
}

func searchFoodsExternal(ctx *Context, name string) ([]map[string]any, error) {
	raw, err := ctx.Client().Get("/v2/foods/search/openfoodfacts", map[string]string{
		"query":     name,
		"autoScale": "true",
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Foods []map[string]any `json:"foods"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		// try as a plain array
		var foods []map[string]any
		if err2 := json.Unmarshal(raw, &foods); err2 != nil {
			return nil, err
		}
		return foods, nil
	}
	return resp.Foods, nil
}

func getFoodByID(ctx *Context, id string) (map[string]any, error) {
	raw, err := ctx.Client().Get("/foods/"+id, nil)
	if err != nil {
		return nil, err
	}
	var food map[string]any
	if err := json.Unmarshal(raw, &food); err != nil {
		return nil, err
	}
	return food, nil
}

// importExternalFood saves an externally-found food to the local Sparky DB and returns it.
func importExternalFood(ctx *Context, food map[string]any) (map[string]any, error) {
	raw, err := ctx.Client().Post("/foods", food)
	if err != nil {
		return nil, err
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, err
	}
	return created, nil
}

func printFoodTable(foods []map[string]any) {
	fmt.Printf("%-36s  %-32s  %8s  %7s  %7s  %7s\n", "ID", "Name", "Cal", "Protein", "Carbs", "Fat")
	fmt.Printf("%-36s  %-32s  %8s  %7s  %7s  %7s\n", "----", "----", "---", "-------", "-----", "---")
	for _, food := range foods {
		id := strVal(food, "id", "provider_external_id")
		name := strVal(food, "name")
		if len(name) > 32 {
			name = name[:29] + "..."
		}
		v := variantMap(food)
		cal := fmt.Sprintf("%.0f", floatVal(v, "calories"))
		pro := fmt.Sprintf("%.1f", floatVal(v, "protein"))
		carb := fmt.Sprintf("%.1f", floatVal(v, "carbs"))
		fat := fmt.Sprintf("%.1f", floatVal(v, "fat"))
		unit := strVal(v, "serving_unit")
		fmt.Printf("%-36s  %-32s  %8s  %7s  %7s  %7s  /%s\n", id, name, cal, pro, carb, fat, unit)
	}
}

func (f *FoodSearchCmd) Run(ctx *Context) error {
	foods, err := searchFoodsLocal(ctx, f.Name, f.Limit)
	if err != nil {
		return err
	}

	if len(foods) > 0 {
		if ctx.JSON {
			raw, _ := json.Marshal(foods)
			fmt.Println(string(raw))
			return nil
		}
		printFoodTable(foods)
		return nil
	}

	// Fall back to Open Food Facts
	if !ctx.JSON {
		fmt.Printf("No local results for %q — searching Open Food Facts online...\n\n", f.Name)
	}
	extFoods, err := searchFoodsExternal(ctx, f.Name)
	if err != nil {
		return fmt.Errorf("external search failed: %w", err)
	}
	if len(extFoods) == 0 {
		fmt.Println("No results found.")
		return nil
	}
	if ctx.JSON {
		raw, _ := json.Marshal(extFoods)
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println("[online — Open Food Facts]")
	printFoodTable(extFoods)
	return nil
}

// ── Log ───────────────────────────────────────────────────────────────────────

type FoodLogCmd struct {
	Name string  `arg:"" optional:"" help:"Food name to search and log."`
	ID   string  `name:"id" help:"Log by food ID directly (skips search — recommended for scripts/agents)."`
	Meal string  `short:"m" default:"snacks" enum:"breakfast,lunch,dinner,snacks" help:"Meal type."`
	Date string  `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
	Qty  float64 `short:"q" default:"1" help:"Quantity (in the food's default serving unit)."`
	Unit string  `short:"u" default:"" help:"Unit override (e.g. g, oz, serving)."`
}

func (f *FoodLogCmd) Run(ctx *Context) error {
	if f.ID == "" && f.Name == "" {
		return fmt.Errorf("provide a food name or --id")
	}

	date := f.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Resolve meal type ID
	mealTypeID, err := resolveMealTypeID(ctx, f.Meal)
	if err != nil {
		return err
	}

	// Get user ID
	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	var food map[string]any

	if f.ID != "" {
		// Direct ID path — fetch food to get variant/nutrient data
		fetched, fetchErr := getFoodByID(ctx, f.ID)
		if fetchErr != nil {
			return fmt.Errorf("food %q not found: %w", f.ID, fetchErr)
		}
		food = fetched
	} else {
		// Search locally first (up to 5 for picker)
		foods, err := searchFoodsLocal(ctx, f.Name, 5)
		if err != nil {
			return err
		}

		if len(foods) > 0 {
			picked, pickErr := pickResult(f.Name, "food", foods, "name", ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
			food = picked
		} else {
			// Fall back to Open Food Facts — already returns multiple results
			extFoods, extErr := searchFoodsExternal(ctx, f.Name)
			if extErr != nil {
				return fmt.Errorf("food %q not found locally; external search failed: %w", f.Name, extErr)
			}
			if len(extFoods) == 0 {
				return fmt.Errorf("food %q not found — use 'sparky food search' to browse available foods", f.Name)
			}
			// Cap to 5 for readability
			if len(extFoods) > 5 {
				extFoods = extFoods[:5]
			}
			picked, pickErr := pickResult(f.Name, "food", extFoods, "name", ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
			// Import the chosen food into Sparky to get a local ID
			imported, importErr := importExternalFood(ctx, picked)
			if importErr != nil {
				return fmt.Errorf("found %q online but failed to add it to Sparky: %w", f.Name, importErr)
			}
			food = imported
			if !ctx.JSON {
				fmt.Printf("Added %q from Open Food Facts to your library.\n", strVal(food, "name"))
			}
		}
	}

	v := variantMap(food)

	unit := f.Unit
	if unit == "" {
		unit = strVal(v, "serving_unit")
		if unit == "" {
			unit = "serving"
		}
	}

	// Scale nutrients: variant values are per serving_size units, scale to requested qty
	servingSize := floatVal(v, "serving_size")
	if servingSize <= 0 {
		servingSize = 1
	}
	scale := f.Qty / servingSize
	entry := map[string]any{
		"user_id":      userID,
		"meal_type_id": mealTypeID,
		"entry_date":   date,
		"quantity":     f.Qty,
		"unit":         unit,
		"food_id":      strVal(food, "id"),
		"variant_id":   strVal(v, "id"),
		"food_name":    strVal(food, "name"),
		"calories":     floatVal(v, "calories") * scale,
		"protein":      floatVal(v, "protein") * scale,
		"carbs":        floatVal(v, "carbs") * scale,
		"fat":          floatVal(v, "fat") * scale,
	}

	raw, err := ctx.Client().Post("/food-entries", entry)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	cal := floatVal(v, "calories") * scale
	fmt.Printf("Logged: %s (%.4g %s) → %s on %s  [%.0f kcal]\n",
		entry["food_name"], f.Qty, unit, f.Meal, date, cal)
	return nil
}

// ── Diary ─────────────────────────────────────────────────────────────────────

type FoodDiaryCmd struct {
	Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (f *FoodDiaryCmd) Run(ctx *Context) error {
	date := f.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	raw, err := ctx.Client().Get("/food-entries/by-date/"+date, nil)
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
		fmt.Printf("No food entries for %s.\n", date)
		return nil
	}

	fmt.Printf("Food diary — %s\n\n", date)
	fmt.Printf("%-8s  %-10s  %-32s  %5s  %-7s  %6s  %7s  %7s  %6s\n",
		"ID", "Meal", "Food", "Qty", "Unit", "Cal", "Protein", "Carbs", "Fat")
	fmt.Printf("%-8s  %-10s  %-32s  %5s  %-7s  %6s  %7s  %7s  %6s\n",
		"--------", "----------", "--------------------------------", "-----", "-------", "------", "-------", "-------", "------")

	var totalCal, totalPro, totalCarb, totalFat float64
	for _, e := range entries {
		id := strVal(e, "id")
		if len(id) > 8 {
			id = id[:8]
		}
		meal := strVal(e, "meal_type")
		name := strVal(e, "food_name")
		if len(name) > 32 {
			name = name[:29] + "..."
		}
		qty := floatVal(e, "quantity")
		unit := strVal(e, "unit")
		cal := floatVal(e, "calories")
		pro := floatVal(e, "protein")
		carb := floatVal(e, "carbs")
		fat := floatVal(e, "fat")
		totalCal += cal
		totalPro += pro
		totalCarb += carb
		totalFat += fat
		fmt.Printf("%-8s  %-10s  %-32s  %5.0f  %-7s  %6.0f  %7.1f  %7.1f  %6.1f\n",
			id, meal, name, qty, unit, cal, pro, carb, fat)
	}
	fmt.Printf("\n%-62s  %6.0f  %7.1f  %7.1f  %6.1f\n",
		"TOTAL", totalCal, totalPro, totalCarb, totalFat)
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

type FoodDeleteCmd struct {
	ID string `arg:"" help:"Entry ID to delete."`
}

func (f *FoodDeleteCmd) Run(ctx *Context) error {
	_, err := ctx.Client().Delete("/food-entries/" + f.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted entry %s.\n", f.ID)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func variantMap(food map[string]any) map[string]any {
	if v, ok := food["default_variant"]; ok {
		if vm, ok := v.(map[string]any); ok {
			return vm
		}
	}
	return map[string]any{}
}

func resolveMealTypeID(ctx *Context, mealName string) (string, error) {
	raw, err := ctx.Client().Get("/meal-types", nil)
	if err != nil {
		return "", fmt.Errorf("fetch meal types: %w", err)
	}
	var types []map[string]any
	if err := json.Unmarshal(raw, &types); err != nil {
		return "", err
	}
	for _, t := range types {
		if strings.EqualFold(strVal(t, "name"), mealName) {
			return strVal(t, "id"), nil
		}
	}
	return "", fmt.Errorf("meal type %q not found", mealName)
}

