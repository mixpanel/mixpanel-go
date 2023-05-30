package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go"
)

func PeopleUnionProperties() error {
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

	// let's add a new vehicle to the list
	// but we don't want to add duplicates
	// so we use the union operator
	// will add warthog to the list
	if err := mp.PeopleUnionProperty(ctx, "Spartan-117", map[string]any{
		"vehicles": []string{"warthog"},
	}); err != nil {
		return err
	}

	// now the list of vehicles is ["warthog", "scorpion", "pelican"]
	// which is the same as before

	// Not let's a vechicle to the list
	if err := mp.PeopleUnionProperty(ctx, "Spartan-117", map[string]any{
		"vehicles": []string{"mongoose"},
	}); err != nil {
		return err
	}

	// since mongoose is not in the list it will be added
	// now the list of vehicles is ["warthog", "scorpion", "pelican", "mongoose"]

	return nil
}
