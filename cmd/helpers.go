package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// pickResult selects one entry from results by name match or interactive prompt.
//
// Rules:
//  1. Exact case-insensitive name match → use it silently.
//  2. Single result, name close enough (query is a substring or vice-versa) → confirm Y/n.
//  3. Multiple/ambiguous results → numbered list, user picks.
//  4. jsonMode = true → always return results[0] without interaction.
func pickResult(query, label string, results []map[string]any, nameField string, jsonMode bool) (map[string]any, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("%s %q not found", label, query)
	}

	queryLow := strings.ToLower(strings.TrimSpace(query))

	// 1. Exact match — silent
	for _, r := range results {
		if strings.ToLower(strings.TrimSpace(strVal(r, nameField))) == queryLow {
			return r, nil
		}
	}

	// In JSON mode skip interaction
	if jsonMode {
		return results[0], nil
	}

	// 2. Single result — quick confirm
	if len(results) == 1 {
		name := strVal(results[0], nameField)
		fmt.Printf("Best match: %q  — use this? [Y/n] ", name)
		answer := readLine()
		if answer == "" || strings.ToLower(answer) == "y" {
			return results[0], nil
		}
		return nil, fmt.Errorf("cancelled — try a more specific name or use 'sparky %s search'", label)
	}

	// 3. Multiple results — numbered list
	fmt.Printf("Multiple matches for %q. Pick one:\n", query)
	for i, r := range results {
		fmt.Printf("  [%d] %s\n", i+1, strVal(r, nameField))
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

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
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
