package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go"
)

func PeopleAppendListProperty() error {
	// Let's make a User Profile
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewApiClient(
		"token",
	)

	// Can use the SetReversedProperty or make the reserved property yourself
	spartan117 := mixpanel.NewPeopleProperties("Spartan-117", map[string]any{
		string(mixpanel.PeopleFirstNameProperty): "John",
		"ai":                                     "Cortana",
		"vehicles":                               []string{"warthog", "scorpion", "pelican"},
	})
	spartan117.SetReservedProperty(mixpanel.PeopleNameProperty, "Master Chief")

	if err := mp.PeopleSet(ctx,
		[]*mixpanel.PeopleProperties{
			spartan117,
		},
	); err != nil {
		return err
	}

	// we have a list of vehicles, lets add a new one
	if err := mp.PeopleAppendListProperty(ctx, "Spartan-117", map[string]any{
		"vehicles": 1,
	}); err != nil {
		return err
	}

	// now the list of vehicles is ["warthog", "scorpion", "pelican", "puma"]
	return nil
}
