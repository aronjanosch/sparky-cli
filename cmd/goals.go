package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type GoalsCmd struct {
	Date string `short:"d" default:"" help:"Date (YYYY-MM-DD). Defaults to today."`
}

func (c *GoalsCmd) Run(ctx *Context) error {
	date := c.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	userID, err := resolveUserID(ctx)
	if err != nil {
		return err
	}

	goalsRaw, goalsErr := ctx.Client().Get("/goals/for-date", map[string]string{
		"date":   date,
		"userId": userID,
	})
	foodRaw, foodErr := ctx.Client().Get("/food-entries/by-date/"+date, nil)
	exRaw, exErr := ctx.Client().Get("/exercise-entries/by-date", map[string]string{
		"date": date,
	})

	if ctx.JSON {
		var goalsData, foodData, exData any
		if goalsErr == nil {
			_ = json.Unmarshal(goalsRaw, &goalsData)
		}
		if foodErr == nil {
			_ = json.Unmarshal(foodRaw, &foodData)
		}
		if exErr == nil {
			_ = json.Unmarshal(exRaw, &exData)
		}
		combined, _ := json.Marshal(map[string]any{
			"goals":    goalsData,
			"food":     foodData,
			"exercise": exData,
		})
		fmt.Println(string(combined))
		return nil
	}

	// Parse goals — try object or single-element array
	var goals map[string]any
	if goalsErr == nil && len(goalsRaw) > 0 {
		if err := json.Unmarshal(goalsRaw, &goals); err != nil {
			// some endpoints return an array
			var arr []map[string]any
			if err2 := json.Unmarshal(goalsRaw, &arr); err2 == nil && len(arr) > 0 {
				goals = arr[0]
			}
		}
	}

	// Sum food totals
	var foodCal, foodPro, foodCarb, foodFat, foodFiber float64
	if foodErr == nil {
		var entries []map[string]any
		if err := json.Unmarshal(foodRaw, &entries); err == nil {
			for _, e := range entries {
				foodCal += floatVal(e, "calories")
				foodPro += floatVal(e, "protein")
				foodCarb += floatVal(e, "carbs")
				foodFat += floatVal(e, "fat")
				foodFiber += floatVal(e, "dietary_fiber", "fiber")
			}
		}
	}

	// Sum exercise calories burned
	var exCal float64
	if exErr == nil {
		var entries []map[string]any
		if err := json.Unmarshal(exRaw, &entries); err == nil {
			for _, e := range entries {
				exCal += floatVal(e, "calories_burned")
			}
		}
	}

	fmt.Printf("Goals — %s\n\n", date)

	if len(goals) == 0 {
		fmt.Println("No goals set for this date.")
		fmt.Printf("\n  Eaten: %.0f kcal  |  Protein: %.1fg  |  Carbs: %.1fg  |  Fat: %.1fg\n",
			foodCal, foodPro, foodCarb, foodFat)
		if exCal > 0 {
			fmt.Printf("  Exercise: %.0f kcal burned  →  Net: %.0f kcal\n", exCal, foodCal-exCal)
		}
		return nil
	}

	calGoal := floatVal(goals, "calories", "calorie_goal", "calorie")
	proGoal := floatVal(goals, "protein", "protein_goal")
	carbGoal := floatVal(goals, "carbs", "carb_goal", "carbohydrates")
	fatGoal := floatVal(goals, "fat", "fat_goal")
	fiberGoal := floatVal(goals, "dietary_fiber", "fiber", "fiber_goal")

	fmt.Printf("%-10s  %10s  %10s  %9s  %5s  %s\n", "Nutrient", "Goal", "Eaten", "Left", "%", "Progress")
	fmt.Printf("%-10s  %10s  %10s  %9s  %5s  %s\n",
		"----------", "----------", "----------", "---------", "-----", "------------")

	printGoalRow("Calories", calGoal, foodCal, "kcal")
	printGoalRow("Protein", proGoal, foodPro, "g")
	printGoalRow("Carbs", carbGoal, foodCarb, "g")
	printGoalRow("Fat", fatGoal, foodFat, "g")
	if fiberGoal > 0 || foodFiber > 0 {
		printGoalRow("Fiber", fiberGoal, foodFiber, "g")
	}

	if exCal > 0 {
		netCal := foodCal - exCal
		fmt.Printf("\n  Exercise: %.0f kcal burned\n", exCal)
		if calGoal > 0 {
			netLeft := calGoal - netCal
			fmt.Printf("  Net calories: %.0f / %.0f  (%.0f left)\n", netCal, calGoal, netLeft)
		} else {
			fmt.Printf("  Net calories: %.0f\n", netCal)
		}
	}

	return nil
}

func printGoalRow(name string, goal, actual float64, unit string) {
	var goalStr, actualStr, leftStr, pctStr string

	if goal > 0 {
		left := goal - actual
		pct := (actual / goal) * 100
		pctStr = fmt.Sprintf("%.0f%%", pct)
		if unit == "kcal" {
			goalStr = fmt.Sprintf("%.0f kcal", goal)
			actualStr = fmt.Sprintf("%.0f", actual)
			leftStr = fmt.Sprintf("%.0f", left)
		} else {
			goalStr = fmt.Sprintf("%.0f%s", goal, unit)
			actualStr = fmt.Sprintf("%.1f%s", actual, unit)
			leftStr = fmt.Sprintf("%.1f%s", left, unit)
		}
	} else {
		goalStr = "—"
		leftStr = "—"
		pctStr = "—"
		if unit == "kcal" {
			actualStr = fmt.Sprintf("%.0f", actual)
		} else {
			actualStr = fmt.Sprintf("%.1f%s", actual, unit)
		}
	}

	var bar string
	if goal > 0 {
		pct := (actual / goal) * 100
		bar = goalProgressBar(pct, 12)
	}

	fmt.Printf("%-10s  %10s  %10s  %9s  %5s  %s\n", name, goalStr, actualStr, leftStr, pctStr, bar)
}

func goalProgressBar(pct float64, width int) string {
	if pct <= 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	filled := int(math.Round(pct / 100 * float64(width)))
	if filled > width {
		filled = width
	}
	fill := "█"
	if pct > 100 {
		fill = "▓" // over goal
	}
	return "[" + strings.Repeat(fill, filled) + strings.Repeat("░", width-filled) + "]"
}
