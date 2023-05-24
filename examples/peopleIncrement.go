package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go"
)

func PeopleIncrement() error {
	// Let's make a User Profile
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewClient(
		"token",
		mixpanel.ProjectID(0),
		mixpanel.ServiceAccount("user_name", "secret"),
	)

	// Can use the SetReversedProperty or make the reserved property yourself
	spartan117 := mixpanel.NewPeopleProperties("Spartan-117", map[string]any{
		string(mixpanel.PeopleFirstNameProperty): "John",
		"ai":                                     "Cortana",
	})
	spartan117.SetReservedProperty(mixpanel.PeopleNameProperty, "Master Chief")

	if err := mp.PeopleSet(ctx,
		[]*mixpanel.PeopleProperties{
			spartan117,
		},
	); err != nil {
		return err
	}

	// Ok lets increments the # of elites kill
	if err := mp.PeopleIncrement(ctx, "Spartan-117", map[string]int{
		"elites killed": 1,
	}); err != nil {
		return err
	}

	// Ok lets increments the # of grunts kill
	if err := mp.PeopleIncrement(ctx, "Spartan-117", map[string]int{
		"grunts killed": 5,
	}); err != nil {
		return err
	}

	return nil
}
