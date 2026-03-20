package cmd

import "testing"

// The server's POST /foods handler reads nutrients from top-level fields, not
// from default_variant. These tests verify that normalizeExternalFoodForImport
// flattens default_variant to the top level and ensures serving_size is non-null.

func TestNormalizeExternalFoodForImport_FlattenDefaultVariant(t *testing.T) {
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

	if size, _ := got["serving_size"].(float64); size != 113 {
		t.Fatalf("expected top-level serving_size=113, got %v", got["serving_size"])
	}
	if cal, _ := got["calories"].(float64); cal != 250 {
		t.Fatalf("expected top-level calories=250, got %v", got["calories"])
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

func TestNormalizeExternalFoodForImport_TopLevelServingSizePreserved(t *testing.T) {
	// If top-level serving_size is already set, don't overwrite it.
	food := map[string]any{
		"name":         "OFF Item",
		"serving_size": 150.0,
		"serving_unit": "g",
		"default_variant": map[string]any{
			"serving_size": 100.0,
			"serving_unit": "g",
		},
	}

	got := normalizeExternalFoodForImport(food)

	if size, _ := got["serving_size"].(float64); size != 150 {
		t.Fatalf("expected serving_size=150 (preserved), got %v", got["serving_size"])
	}
}
