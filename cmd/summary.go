package cmd

import (
	"encoding/json"
	"fmt"
	"time"
)

// ── Summary ───────────────────────────────────────────────────────────────────

type SummaryCmd struct {
	Start string `short:"s" default:"" help:"Start date (YYYY-MM-DD)."`
	End   string `short:"e" default:"" help:"End date (YYYY-MM-DD)."`
}

func (c *SummaryCmd) Run(ctx *Context) error {
	start := c.Start
	if start == "" {
		start = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	end := c.End
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	raw, err := ctx.Client().Get("/reports", map[string]string{
		"startDate": start,
		"endDate":   end,
	})
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	var r struct {
		NutritionData   []map[string]any `json:"nutritionData"`
		ExerciseEntries []map[string]any `json:"exerciseEntries"`
		MeasurementData []map[string]any `json:"measurementData"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		fmt.Println(string(raw))
		return nil
	}

	var totalCal, totalPro, totalCarb, totalFat float64
	for _, d := range r.NutritionData {
		totalCal += floatVal(d, "calories")
		totalPro += floatVal(d, "protein")
		totalCarb += floatVal(d, "carbs")
		totalFat += floatVal(d, "fat")
	}

	var totalExCal, totalExDur float64
	for _, e := range r.ExerciseEntries {
		totalExCal += floatVal(e, "calories_burned")
		totalExDur += floatVal(e, "duration_minutes")
	}

	var weightSum float64
	var weightCount int
	for _, m := range r.MeasurementData {
		if w := floatVal(m, "weight"); w > 0 {
			weightSum += w
			weightCount++
		}
	}

	dash := func(v float64, unit string) string {
		if v == 0 {
			return "—"
		}
		return fmt.Sprintf("%.1f %s", v, unit)
	}

	fmt.Printf("Summary: %s → %s\n", start, end)
	fmt.Printf("\nNutrition\n")
	fmt.Printf("  %-22s %s\n", "Calories:", dash(totalCal, "kcal"))
	fmt.Printf("  %-22s %s\n", "Protein:", dash(totalPro, "g"))
	fmt.Printf("  %-22s %s\n", "Carbs:", dash(totalCarb, "g"))
	fmt.Printf("  %-22s %s\n", "Fat:", dash(totalFat, "g"))
	fmt.Printf("\nExercise\n")
	fmt.Printf("  %-22s %s\n", "Calories burned:", dash(totalExCal, "kcal"))
	fmt.Printf("  %-22s %s\n", "Duration:", dash(totalExDur, "min"))
	fmt.Printf("\nWellbeing\n")
	if weightCount > 0 {
		fmt.Printf("  %-22s %.2f kg\n", "Avg weight:", weightSum/float64(weightCount))
	} else {
		fmt.Printf("  %-22s —\n", "Avg weight:")
	}
	return nil
}

// ── Trends ────────────────────────────────────────────────────────────────────

type TrendsCmd struct {
	Days int `short:"n" default:"30" help:"Number of days to look back."`
}

func (c *TrendsCmd) Run(ctx *Context) error {
	raw, err := ctx.Client().Get("/reports/nutrition-trends-with-goals", map[string]string{
		"days": fmt.Sprintf("%d", c.Days),
	})
	if err != nil {
		return err
	}

	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}

	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		fmt.Println(string(raw))
		return nil
	}

	if len(rows) == 0 {
		fmt.Printf("No nutrition trends data for the last %d days.\n", c.Days)
		return nil
	}

	// Detect goal columns from first row
	_, hasCalGoal := rows[0]["calorie_goal"]
	_, hasProGoal := rows[0]["protein_goal"]
	_, hasCarbGoal := rows[0]["carb_goal"]
	_, hasFatGoal := rows[0]["fat_goal"]

	fmt.Printf("Nutrition trends — last %d days\n\n", c.Days)

	if !hasCalGoal && !hasProGoal && !hasCarbGoal && !hasFatGoal {
		fmt.Printf("%-12s  %8s  %8s  %8s  %8s\n", "Date", "Calories", "Protein", "Carbs", "Fat")
		fmt.Printf("%-12s  %8s  %8s  %8s  %8s\n", "----------", "--------", "-------", "-----", "---")
		for _, row := range rows {
			date := strVal(row, "date")
			cal := floatVal(row, "calories")
			pro := floatVal(row, "protein")
			carb := floatVal(row, "carbs")
			fat := floatVal(row, "fat")
			fmt.Printf("%-12s  %8.0f  %8.1f  %8.1f  %8.1f\n", date, cal, pro, carb, fat)
		}
		return nil
	}

	// With goal columns
	fmt.Printf("%-12s  %8s  %9s  %8s  %9s  %8s  %9s  %8s  %9s\n",
		"Date", "Calories", "Cal Goal", "Protein", "Pro Goal", "Carbs", "Carb Goal", "Fat", "Fat Goal")
	fmt.Printf("%-12s  %8s  %9s  %8s  %9s  %8s  %9s  %8s  %9s\n",
		"----------", "--------", "---------", "-------", "--------", "-----", "---------", "---", "--------")
	for _, row := range rows {
		date := strVal(row, "date")
		cal := floatVal(row, "calories")
		pro := floatVal(row, "protein")
		carb := floatVal(row, "carbs")
		fat := floatVal(row, "fat")

		calGoal := goalStr(row, "calorie_goal", hasCalGoal)
		proGoal := goalStr(row, "protein_goal", hasProGoal)
		carbGoal := goalStr(row, "carb_goal", hasCarbGoal)
		fatGoal := goalStr(row, "fat_goal", hasFatGoal)

		fmt.Printf("%-12s  %8.0f  %9s  %8.1f  %9s  %8.1f  %9s  %8.1f  %9s\n",
			date, cal, calGoal, pro, proGoal, carb, carbGoal, fat, fatGoal)
	}
	return nil
}

func goalStr(row map[string]any, key string, present bool) string {
	if !present {
		return "—"
	}
	v := floatVal(row, key)
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", v)
}
