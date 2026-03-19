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

func (f *FoodSearchCmd) Run(ctx *Context) error {
	raw, err := ctx.Client().Get("/foods/foods-paginated", map[string]string{
		"searchTerm":  f.Name,
		"foodFilter":  "all",
		"currentPage": "1",
		"itemsPerPage": fmt.Sprintf("%d", f.Limit),
		"sortBy":      "name:asc",
	})
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	var resp struct {
		Foods []map[string]any `json:"foods"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		fmt.Println(string(raw))
		return nil
	}

	if len(resp.Foods) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("%-36s  %-32s  %8s  %7s  %7s  %7s\n", "ID", "Name", "Cal", "Protein", "Carbs", "Fat")
	fmt.Printf("%-36s  %-32s  %8s  %7s  %7s  %7s\n", "----", "----", "---", "-------", "-----", "---")
	for _, food := range resp.Foods {
		id := strVal(food, "id")
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
	return nil
}

// ── Log ───────────────────────────────────────────────────────────────────────

type FoodLogCmd struct {
	Name string  `arg:"" help:"Food name to search and log."`
	Meal string  `short:"m" default:"snacks" enum:"breakfast,lunch,dinner,snacks" help:"Meal type."`
	Date string  `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
	Qty  float64 `short:"q" default:"1" help:"Quantity (in the food's default serving unit)."`
	Unit string  `short:"u" default:"" help:"Unit override (e.g. g, oz, serving)."`
}

func (f *FoodLogCmd) Run(ctx *Context) error {
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

	// Search for food
	searchRaw, err := ctx.Client().Get("/foods/foods-paginated", map[string]string{
		"searchTerm":   f.Name,
		"foodFilter":   "all",
		"currentPage":  "1",
		"itemsPerPage": "1",
		"sortBy":       "name:asc",
	})
	if err != nil {
		return err
	}

	var searchResp struct {
		Foods []map[string]any `json:"foods"`
	}
	if err := json.Unmarshal(searchRaw, &searchResp); err != nil {
		return fmt.Errorf("parse food search: %w", err)
	}
	if len(searchResp.Foods) == 0 {
		return fmt.Errorf("food %q not found — use 'sparky food search' to find the exact name", f.Name)
	}

	food := searchResp.Foods[0]
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

