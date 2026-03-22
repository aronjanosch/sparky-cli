package cmd

import "testing"

// The server's POST /foods handler reads nutrients from top-level fields, not
// from default_variant. These tests verify that normalizeExternalFoodForImport
// flattens default_variant to the top level and ensures serving_size is non-null.

func TestNormalizeExternalFoodForImport_FlattenDefaultVariant(t *testing.T) {
	// serving_size=113g with calories=250 → normalize to per-100g: 250*(100/113)≈221.24
	food := map[string]any{
		"name": "OFF Item",
		"default_variant": map[string]any{
			"serving_size": 113.0,
			"serving_unit": "g",
			"calories":     250.0,
			"protein":      18.0,
		},
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 100 {
		t.Fatalf("expected serving_size=100 (normalized), got %v", got["serving_size"])
	}
	if cal, _ := got["calories"].(float64); cal != 250.0*100/113 {
		t.Fatalf("expected calories=%v (scaled to per-100g), got %v", 250.0*100/113, got["calories"])
	}
}

func TestNormalizeExternalFoodForImport_NullServingSizeDefaultsTo100(t *testing.T) {
	food := map[string]any{
		"name": "OFF Item",
		"default_variant": map[string]any{
			"serving_size": nil,
			"serving_unit": "g",
			"calories":     188.0,
		},
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 100 {
		t.Fatalf("expected top-level serving_size=100, got %v", got["serving_size"])
	}
	if unit, _ := got["serving_unit"].(string); unit != "g" {
		t.Fatalf("expected serving_unit=g, got %q", got["serving_unit"])
	}
}

func TestNormalizeExternalFoodForImport_NoDefaultVariant(t *testing.T) {
	food := map[string]any{
		"name": "Minimal Item",
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 100 {
		t.Fatalf("expected serving_size=100, got %v", got["serving_size"])
	}
	if unit, _ := got["serving_unit"].(string); unit != "g" {
		t.Fatalf("expected serving_unit=g, got %q", got["serving_unit"])
	}
}

func TestNormalizeExternalFoodForImport_VariantsRemoved(t *testing.T) {
	food := map[string]any{
		"name":     "OFF Item",
		"variants": []any{map[string]any{"serving_size": nil}},
	}

	got := normalizeExternalFoodForImport(food)

	if _, ok := got["variants"]; ok {
		t.Fatalf("expected variants to be removed, still present")
	}
}

func TestNormalizeExternalFoodForImport_NormalizeTo100g(t *testing.T) {
	// serving_size=50g, calories=60 → normalize to per-100g: calories=120
	// This matches the Rindergulasch bug: app showed 4× wrong values because
	// it recalculates from food.calories using 100g as base unit.
	food := map[string]any{
		"name":         "Rindergulasch",
		"serving_size": 50.0,
		"serving_unit": "g",
		"calories":     60.0,
		"protein":      5.0,
		"carbs":        3.0,
		"fat":          2.0,
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 100 {
		t.Fatalf("expected serving_size=100, got %v", got["serving_size"])
	}
	if cal, _ := got["calories"].(float64); cal != 120 {
		t.Fatalf("expected calories=120, got %v", got["calories"])
	}
	if pro, _ := got["protein"].(float64); pro != 10 {
		t.Fatalf("expected protein=10, got %v", got["protein"])
	}
}

func TestNormalizeExternalFoodForImport_Already100gUnchanged(t *testing.T) {
	food := map[string]any{
		"name":         "Already Normalized",
		"serving_size": 100.0,
		"serving_unit": "g",
		"calories":     120.0,
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 100 {
		t.Fatalf("expected serving_size=100, got %v", got["serving_size"])
	}
	if cal, _ := got["calories"].(float64); cal != 120 {
		t.Fatalf("expected calories unchanged at 120, got %v", got["calories"])
	}
}

func TestNormalizeExternalFoodForImport_NonGramServingUnchanged(t *testing.T) {
	// Non-gram units (e.g. "oz") should not be scaled.
	food := map[string]any{
		"name":         "Item in oz",
		"serving_size": 1.0,
		"serving_unit": "oz",
		"calories":     50.0,
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 1 {
		t.Fatalf("expected serving_size=1 (unchanged for oz), got %v", got["serving_size"])
	}
	if cal, _ := got["calories"].(float64); cal != 50 {
		t.Fatalf("expected calories=50 (unchanged), got %v", got["calories"])
	}
}
