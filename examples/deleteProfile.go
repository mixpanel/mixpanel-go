package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go/v2"
)

func DeleteProfile() error {
	ctx := context.Background()

	mp := mixpanel.NewApiClient(
		"token",
	)

	// Can use the SetReversedProperty or make the reserved property yourself
	spartan117 := mixpanel.NewPeopleProperties("Spartan-117", map[string]any{
		string(mixpanel.PeopleFirstNameProperty): "John",
		"ai":                                     "Cortana",
	})

	if err := mp.PeopleSet(ctx,
		[]*mixpanel.PeopleProperties{
			spartan117,
		},
	); err != nil {
		return err
	}

	// Spartans never die. They're just missing in action
	if err := mp.PeopleDeleteProfile(ctx, "Spartan-117", true); err != nil {
		return err
	}

	return nil
}
