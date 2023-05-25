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

	fmt.Println("PeopleSet called successfully")
	return nil
}
