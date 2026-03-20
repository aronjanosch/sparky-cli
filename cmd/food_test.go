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

	if size := floatVal(v, "serving_size"); size != 1 {
		t.Fatalf("expected serving_size to default to 1, got %v", v["serving_size"])
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

	if size := floatVal(v, "serving_size"); size != 1 {
		t.Fatalf("expected serving_size to default to 1, got %v", v["serving_size"])
	}
	if unit := strVal(v, "serving_unit"); unit != "serving" {
		t.Fatalf("expected serving_unit to default to 'serving', got %q", unit)
	}
}
