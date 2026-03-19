package cmd

import (
	"encoding/json"
	"fmt"
)

// resolveProviderID fetches /external-providers and returns the ID for the given providerType.
func resolveProviderID(ctx *Context, providerType string) (string, error) {
	raw, err := ctx.Client().Get("/external-providers", nil)
	if err != nil {
		return "", err
	}
	var providers []map[string]any
	if err := json.Unmarshal(raw, &providers); err != nil {
		return "", err
	}
	for _, p := range providers {
		if strVal(p, "provider_type") == providerType {
			return strVal(p, "id"), nil
		}
	}
	return "", fmt.Errorf("provider %q not configured in Sparky", providerType)
}

func resolveUserID(ctx *Context) (string, error) {
	raw, err := ctx.Client().Get("/identity/user", nil)
	if err != nil {
		return "", err
	}
	var u map[string]any
	if err := json.Unmarshal(raw, &u); err != nil {
		return "", err
	}
	return strVal(u, "activeUserId"), nil
}

func strVal(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func floatVal(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			}
		}
	}
	return 0
}
