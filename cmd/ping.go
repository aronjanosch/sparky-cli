package cmd

import (
	"encoding/json"
	"fmt"
)

type PingCmd struct{}

func (p *PingCmd) Run(ctx *Context) error {
	raw, err := ctx.Client().Get("/identity/user", nil)
	if err != nil {
		return err
	}
	if ctx.JSON {
		fmt.Println(string(raw))
		return nil
	}
	var u struct {
		Email    string `json:"authenticatedUserEmail"`
		FullName string `json:"activeUserFullName"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(raw, &u); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("Connected  ✓\n")
	fmt.Printf("User:      %s (%s)\n", u.FullName, u.Email)
	fmt.Printf("Role:      %s\n", u.Role)
	return nil
}
