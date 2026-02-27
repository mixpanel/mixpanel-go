package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go/v2"
)

func PeopleRemoveListProperty() error {
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
		"vehicles":                               []string{"warthog", "scorpion", "pelican", "puma"},
	})
	spartan117.SetReservedProperty(mixpanel.PeopleNameProperty, "Master Chief")

	if err := mp.PeopleSet(ctx,
		[]*mixpanel.PeopleProperties{
			spartan117,
		},
	); err != nil {
		return err
	}

	// we upset some people since a warthog looks more like a puma so let's remove it
	if err := mp.PeopleRemoveListProperty(ctx, "Spartan-117", map[string]any{
		"vehicles": "warthog",
	}); err != nil {
		return err
	}

	// now the list of vehicles is ["scorpion", "pelican", "puma"]
	return nil
}
