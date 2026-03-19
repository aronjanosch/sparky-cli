package cmd

import (
	"encoding/json"
	"fmt"
)

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
