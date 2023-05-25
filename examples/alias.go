package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go"
)

func Alias() error {
	ctx := context.Background()

	mp := mixpanel.NewClient(
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

	if err := mp.Alias(ctx, "Spartan-117", "Master Chief"); err != nil {
		return err
	}

	return nil
}
