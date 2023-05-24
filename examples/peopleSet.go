package examples

import (
	"context"
	"fmt"

	"github.com/mixpanel/mixpanel-go"
)

func PeopleSet() error {
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewClient(
		"token",
		mixpanel.ProjectID(0),
		mixpanel.ServiceAccount("user_name", "secret"),
	)

	if err := mp.PeopleSet(ctx, "Spartan-117", map[string]any{
		mixpanel.UserFirstNameProperty: "John",
		mixpanel.UserLastNameProperty:  "",
		mixpanel.UserNameProperty:      "Spartan 117",
		"ai":                           "Cortana",
	}); err != nil {
		return err
	}

	fmt.Println("PeopleSet called successfully")
	return nil
}
