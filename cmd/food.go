package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type FoodCmd struct {
	Search FoodSearchCmd `cmd:"" help:"Search for foods in the database."`
	Log    FoodLogCmd    `cmd:"" help:"Log a food entry."`
	Create FoodCreateCmd `cmd:"" help:"Create a custom food in your library."`
	Diary  FoodDiaryCmd  `cmd:"" help:"View food diary for a date."`
	Delete FoodDeleteCmd `cmd:"" help:"Delete a food diary entry by ID."`
	Remove FoodRemoveCmd `cmd:"" help:"Remove a food from your library."`
}

// ── Search ────────────────────────────────────────────────────────────────────

type FoodSearchCmd struct {
	Name     string `arg:"" optional:"" help:"Food name to search for."`
	Barcode  string `name:"barcode" short:"b" help:"Look up by barcode (exact product match)."`
	Limit    int    `short:"l" default:"10" help:"Max results to show."`
	Internal bool   `name:"internal" short:"i" help:"Search internal/local db only (bypass OpenFoodFacts API search)."`
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
		"autoScale": "false", // return per-100g values; we always store serving_size=100
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

// lookupFoodByBarcode calls GET /foods/barcode/:barcode and returns the food
// plus the source ("local", "openfoodfacts", etc.).
func lookupFoodByBarcode(ctx *Context, barcode string) (map[string]any, string, error) {
	raw, err := ctx.Client().Get("/foods/barcode/"+barcode, nil)
	if err != nil {
		return nil, "", err
	}
	var resp struct {
		Source string         `json:"source"`
		Food   map[string]any `json:"food"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", err
	}
	if resp.Source == "not_found" || resp.Food == nil {
		return nil, "not_found", fmt.Errorf("barcode %q not found", barcode)
	}
	return resp.Food, resp.Source, nil
}

// importExternalFood saves an externally-found food to the local Sparky DB and returns it.
func importExternalFood(ctx *Context, food map[string]any) (map[string]any, error) {
	raw, err := ctx.Client().Post("/foods", normalizeExternalFoodForImport(food))
	if err != nil {
		return nil, err
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		return nil, err
	}
	return created, nil
}

// normalizeExternalFoodForImport prepares an Open Food Facts food payload for
// POST /foods. The server's createFood handler reads all nutrients from top-level
// fields (serving_size, calories, protein, etc.) — it ignores default_variant and
// the variants array entirely. So we flatten default_variant to the top level and
// drop variants (only used in the CSV bulk-import path).
func normalizeExternalFoodForImport(food map[string]any) map[string]any {
	if food == nil {
		return map[string]any{}
	}

	// Flatten default_variant fields to top level (server reads from here).
	for k, val := range variantMap(food) {
		if val != nil {
			if existing, exists := food[k]; !exists || existing == nil {
				food[k] = val
			}
		}
	}
	delete(food, "variants") // variants array is only used by CSV bulk import

	// Ensure serving_size is non-null (DB NOT NULL constraint on food_variants).
	if floatVal(food, "serving_size") <= 0 {
		unit := strings.TrimSpace(strVal(food, "serving_unit"))
		if unit == "" || strings.EqualFold(unit, "g") {
			food["serving_size"] = 100.0
			food["serving_unit"] = "g"
		} else {
			food["serving_size"] = 1.0
		}
	}
	if strVal(food, "serving_unit") == "" {
		food["serving_unit"] = "g"
	}

	// Normalize gram-based foods to per-100g so the stored base unit is always
	// serving_size=100g. OFF often returns per-serving values (e.g. 50g serving),
	// which causes the app to display wrong totals when it recalculates from the
	// food's nutrient data rather than the stored entry values.
	unit := strings.TrimSpace(strVal(food, "serving_unit"))
	servingSize := floatVal(food, "serving_size")
	if (strings.EqualFold(unit, "g") || unit == "") && servingSize > 0 && servingSize != 100 {
		scale := 100.0 / servingSize
		for _, n := range []string{"calories", "protein", "carbs", "fat", "fiber", "sugar", "sodium", "saturated_fat"} {
			if v := floatVal(food, n); v != 0 {
				food[n] = v * scale
			}
		}
		food["serving_size"] = 100.0
		food["serving_unit"] = "g"
	}

	return food
}

func printFoodTable(foods []map[string]any) {
	fmt.Printf("%-36s  %-28s  %-16s  %12s  %7s  %7s  %7s\n", "ID", "Name", "Brand", "kcal/100g", "Pro/100g", "Carb/100g", "Fat/100g")
	fmt.Printf("%-36s  %-28s  %-16s  %12s  %7s  %7s  %7s\n", "----", "----", "-----", "---------", "--------", "---------", "--------")
	for _, food := range foods {
		id := strVal(food, "id", "provider_external_id")
		name := strVal(food, "name")
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		brand := strVal(food, "brand")
		if len(brand) > 16 {
			brand = brand[:13] + "..."
		}
		v := variantMap(food)
		cal := fmt.Sprintf("%.0f", floatVal(v, "calories"))
		pro := fmt.Sprintf("%.1f", floatVal(v, "protein"))
		carb := fmt.Sprintf("%.1f", floatVal(v, "carbs"))
		fat := fmt.Sprintf("%.1f", floatVal(v, "fat"))
		fmt.Printf("%-36s  %-28s  %-16s  %12s  %7s  %7s  %7s\n", id, name, brand, cal, pro, carb, fat)
	}
}

// pickFoodResult selects one food from results, showing macros + brand so the
// user can distinguish between products with the same name.
func pickFoodResult(query string, results []map[string]any, pick int, jsonMode bool) (map[string]any, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("food %q not found", query)
	}

	queryLow := strings.ToLower(strings.TrimSpace(query))

	// Exact name match — silent
	for _, r := range results {
		if strings.ToLower(strings.TrimSpace(strVal(r, "name"))) == queryLow {
			return r, nil
		}
	}

	// --pick N (for scripts/agents)
	if pick > 0 {
		if pick > len(results) {
			return nil, fmt.Errorf("--pick %d out of range (%d results)", pick, len(results))
		}
		return results[pick-1], nil
	}

	// JSON mode: always results[0]
	if jsonMode {
		return results[0], nil
	}

	foodLine := func(r map[string]any) string {
		v := variantMap(r)
		brand := strVal(r, "brand")
		name := strVal(r, "name")
		if brand != "" {
			name = fmt.Sprintf("%s [%s]", name, brand)
		}
		return fmt.Sprintf("%-44s  %.0f kcal | %.1fg P | %.1fg F  per 100g",
			name,
			floatVal(v, "calories"), floatVal(v, "protein"), floatVal(v, "fat"))
	}

	if len(results) == 1 {
		fmt.Printf("Best match: %s\nUse this? [Y/n] ", foodLine(results[0]))
		answer := readLine()
		if answer == "" || strings.ToLower(answer) == "y" {
			return results[0], nil
		}
		return nil, fmt.Errorf("cancelled — try a more specific name or use 'sparky food search'")
	}

	fmt.Printf("Multiple matches for %q. Pick one:\n", query)
	for i, r := range results {
		fmt.Printf("  [%d] %s\n", i+1, foodLine(r))
	}
	fmt.Printf("  [0] Cancel\n")
	fmt.Printf("Choice [1-%d]: ", len(results))

	answer := readLine()
	n, err := strconv.Atoi(strings.TrimSpace(answer))
	if err != nil || n == 0 {
		return nil, fmt.Errorf("cancelled")
	}
	if n < 1 || n > len(results) {
		return nil, fmt.Errorf("invalid choice %d", n)
	}
	return results[n-1], nil
}

func (f *FoodSearchCmd) Run(ctx *Context) error {
	if f.Barcode != "" {
		food, source, err := lookupFoodByBarcode(ctx, f.Barcode)
		if err != nil {
			return err
		}
		if ctx.JSON {
			raw, _ := json.Marshal(food)
			fmt.Println(string(raw))
			return nil
		}
		if source != "local" {
			fmt.Printf("[online — %s]\n", source)
		}
		printFoodTable([]map[string]any{food})
		return nil
	}

	if f.Name == "" {
		return fmt.Errorf("provide a food name or --barcode")
	}

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

	// Return nothing if internal flag is true, otherwise fall back to Open Food Facts
	if f.Internal {
		return nil
	}
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

// ── Create ────────────────────────────────────────────────────────────────────

type FoodCreateCmd struct {
	Name         string  `arg:"" help:"Food name."`
	Brand        string  `name:"brand" short:"b" help:"Brand name."`
	Calories     float64 `name:"calories" short:"c" required:"" help:"Calories per serving."`
	Protein      float64 `name:"protein" short:"p" required:"" help:"Protein (g) per serving."`
	Carbs        float64 `name:"carbs" required:"" help:"Carbohydrates (g) per serving."`
	Fat          float64 `name:"fat" short:"f" required:"" help:"Fat (g) per serving."`
	ServingSize  float64 `name:"serving-size" default:"100" help:"Serving size amount (default: 100g)."`
	ServingUnit  string  `name:"serving-unit" default:"g" help:"Serving unit (default: g)."`
	Fiber        float64 `name:"fiber" default:"0" help:"Fiber (g) per serving."`
	Sugar        float64 `name:"sugar" default:"0" help:"Sugar (g) per serving."`
	Sodium       float64 `name:"sodium" default:"0" help:"Sodium (mg) per serving."`
	SaturatedFat float64 `name:"saturated-fat" default:"0" help:"Saturated fat (g) per serving."`
}

func (f *FoodCreateCmd) Run(ctx *Context) error {
	payload := map[string]any{
		"name":          f.Name,
		"brand":         f.Brand,
		"calories":      f.Calories,
		"protein":       f.Protein,
		"carbs":         f.Carbs,
		"fat":           f.Fat,
		"fiber":         f.Fiber,
		"sugar":         f.Sugar,
		"sodium":        f.Sodium,
		"saturated_fat": f.SaturatedFat,
		"serving_size":  f.ServingSize,
		"serving_unit":  f.ServingUnit,
	}

	raw, err := ctx.Client().Post("/foods", payload)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	var food map[string]any
	if err := json.Unmarshal(raw, &food); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("Created food %q (ID: %s)  %.0f kcal per %.4g %s\n",
		strVal(food, "name"), strVal(food, "id"), f.Calories, f.ServingSize, f.ServingUnit)
	return nil
}

// ── Log ───────────────────────────────────────────────────────────────────────

type FoodLogCmd struct {
	Name    string  `arg:"" optional:"" help:"Food name to search and log."`
	ID      string  `name:"id" help:"Log by food ID directly (skips search)."`
	Barcode string  `name:"barcode" short:"b" help:"Look up by barcode (exact product match)."`
	Pick    int     `name:"pick" default:"0" help:"Pre-select the Nth search result (1-based) without prompting. For use in scripts/agents."`
	Meal    string  `short:"m" default:"snacks" enum:"breakfast,lunch,dinner,snacks" help:"Meal type."`
	Date    string  `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
	Qty     float64 `short:"q" default:"1" help:"Quantity (in the food's default serving unit)."`
	Unit    string  `short:"u" default:"" help:"Unit override (e.g. g, oz, serving)."`
}

func (f *FoodLogCmd) Run(ctx *Context) error {
	if f.ID == "" && f.Name == "" && f.Barcode == "" {
		return fmt.Errorf("provide a food name, --id, or --barcode")
	}

	date := f.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	mealTypeID, err := resolveMealTypeID(ctx, f.Meal)
	if err != nil {
		return err
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	var food map[string]any

	switch {
	case f.ID != "":
		fetched, fetchErr := getFoodByID(ctx, f.ID)
		if fetchErr != nil {
			return fmt.Errorf("food %q not found: %w", f.ID, fetchErr)
		}
		food = fetched

	case f.Barcode != "":
		found, source, lookupErr := lookupFoodByBarcode(ctx, f.Barcode)
		if lookupErr != nil {
			return fmt.Errorf("barcode lookup failed: %w", lookupErr)
		}
		if source != "local" {
			imported, importErr := importExternalFood(ctx, found)
			if importErr != nil {
				return fmt.Errorf("barcode %q found online but failed to import: %w", f.Barcode, importErr)
			}
			food = imported
			if !ctx.JSON {
				fmt.Printf("Added %q from %s to your library.\n", strVal(food, "name"), source)
			}
		} else {
			food = found
		}

	default:
		// Search locally first (up to 5 for picker)
		foods, searchErr := searchFoodsLocal(ctx, f.Name, 5)
		if searchErr != nil {
			return searchErr
		}

		if len(foods) > 0 {
			picked, pickErr := pickFoodResult(f.Name, foods, f.Pick, ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
			food = picked
		} else {
			extFoods, extErr := searchFoodsExternal(ctx, f.Name)
			if extErr != nil {
				return fmt.Errorf("food %q not found locally; external search failed: %w", f.Name, extErr)
			}
			if len(extFoods) == 0 {
				return fmt.Errorf("food %q not found — use 'sparky food search' to browse available foods", f.Name)
			}
			if len(extFoods) > 5 {
				extFoods = extFoods[:5]
			}
			picked, pickErr := pickFoodResult(f.Name, extFoods, f.Pick, ctx.JSON)
			if pickErr != nil {
				return pickErr
			}
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

	servingSize := floatVal(v, "serving_size")
	if servingSize <= 0 {
		servingSize = 100
	}
	// Send raw per-serving-size nutrients — the server snapshots these values and
	// the frontend scales them as (calories / serving_size) × quantity at display
	// time. Pre-scaling here would cause the frontend to double-scale.
	entry := map[string]any{
		"user_id":      userID,
		"meal_type_id": mealTypeID,
		"entry_date":   date,
		"quantity":     f.Qty,
		"unit":         unit,
		"food_id":      strVal(food, "id"),
		"variant_id":   strVal(v, "id"),
		"food_name":    strVal(food, "name"),
		"serving_size": servingSize,
		"serving_unit": strVal(v, "serving_unit"),
		"calories":     floatVal(v, "calories"),
		"protein":      floatVal(v, "protein"),
		"carbs":        floatVal(v, "carbs"),
		"fat":          floatVal(v, "fat"),
	}

	raw, err := ctx.Client().Post("/food-entries", entry)
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	cal := floatVal(v, "calories") / servingSize * f.Qty
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
		eServing := floatVal(e, "serving_size")
		if eServing <= 0 {
			eServing = 100
		}
		eScale := qty / eServing
		cal := floatVal(e, "calories") * eScale
		pro := floatVal(e, "protein") * eScale
		carb := floatVal(e, "carbs") * eScale
		fat := floatVal(e, "fat") * eScale
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

// ── Delete (diary entry) ───────────────────────────────────────────────────────

type FoodDeleteCmd struct {
	ID string `arg:"" help:"Diary entry ID to delete."`
}

func (f *FoodDeleteCmd) Run(ctx *Context) error {
	_, err := ctx.Client().Delete("/food-entries/" + f.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted diary entry %s.\n", f.ID)
	return nil
}

// ── Remove (food from library) ────────────────────────────────────────────────

type FoodRemoveCmd struct {
	ID string `arg:"" help:"Food ID to remove from your library."`
}

func (f *FoodRemoveCmd) Run(ctx *Context) error {
	_, err := ctx.Client().Delete("/foods/" + f.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Removed food %s from library.\n", f.ID)
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
