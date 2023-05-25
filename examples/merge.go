package examples

import (
	"context"

	"github.com/mixpanel/mixpanel-go"
)

func Merge() error {
	ctx := context.Background()

	mp := mixpanel.NewClient(
		"token",
	)

	// Can use the SetReversedProperty or make the reserved property yourself
	spartan117 := mixpanel.NewPeopleProperties("Spartan-117", map[string]any{
		string(mixpanel.PeopleFirstNameProperty): "John",
		"ai":                                     "Cortana",
	})

	john117 := mixpanel.NewPeopleProperties("John-117", map[string]any{
		string(mixpanel.PeopleFirstNameProperty): "John",
		"ai":                                     "Cortana",
	})

	if err := mp.PeopleSet(ctx,
		[]*mixpanel.PeopleProperties{
			spartan117,
			john117,
		},
	); err != nil {
		return err
	}

	// spartan117 and john117 will be merged into one profile
	if err := mp.Merge(ctx, spartan117.DistinctID, john117.DistinctID); err != nil {
		return err
	}

	return nil
}
