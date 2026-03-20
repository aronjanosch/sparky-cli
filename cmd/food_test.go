package cmd

import "testing"

func TestNormalizeExternalFoodForImport_DefaultVariantServingSizeNull(t *testing.T) {
	food := map[string]any{
		"name": "OFF Item",
		"default_variant": map[string]any{
			"serving_size": nil,
			"serving_unit": "g",
		},
	}

	got := normalizeExternalFoodForImport(food)
	v := variantMap(got)

	if size := floatVal(v, "serving_size"); size != 100 {
		t.Fatalf("expected serving_size to default to 100 (per 100g), got %v", v["serving_size"])
	}
	if unit := strVal(v, "serving_unit"); unit != "g" {
		t.Fatalf("expected serving_unit to remain 'g', got %q", unit)
	}
}

func TestNormalizeExternalFoodForImport_DefaultVariantMissing(t *testing.T) {
	food := map[string]any{
		"name": "OFF Item Missing Variant",
	}

	got := normalizeExternalFoodForImport(food)
	v := variantMap(got)

	if size := floatVal(v, "serving_size"); size != 100 {
		t.Fatalf("expected serving_size to default to 100 (per 100g), got %v", v["serving_size"])
	}
	if unit := strVal(v, "serving_unit"); unit != "g" {
		t.Fatalf("expected serving_unit to default to 'g', got %q", unit)
	}
}

func TestNormalizeExternalFoodForImport_VariantsArrayNullServingSize(t *testing.T) {
	food := map[string]any{
		"name": "OFF Multi",
		"default_variant": map[string]any{
			"serving_size": 100.0,
			"serving_unit": "g",
		},
		"variants": []any{
			map[string]any{"serving_size": nil, "serving_unit": "g"},
			map[string]any{"serving_size": 50.0, "serving_unit": "g"},
		},
	}

	got := normalizeExternalFoodForImport(food)
	arr, ok := got["variants"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected variants slice of length 2, got %#v", got["variants"])
	}
	v0 := arr[0].(map[string]any)
	if floatVal(v0, "serving_size") != 100 {
		t.Fatalf("variant[0] serving_size: got %v", v0["serving_size"])
	}
	v1 := arr[1].(map[string]any)
	if floatVal(v1, "serving_size") != 50 {
		t.Fatalf("variant[1] serving_size should stay 50, got %v", v1["serving_size"])
	}
}
