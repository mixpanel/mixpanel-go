package examples

import (
	"context"
	"fmt"

	"github.com/mixpanel/mixpanel-go"
)

func LookupTableReplaceExample() error {
	ctx := context.Background()

	// fill in your project id and service account user name and secret
	mp := mixpanel.NewApiClient(
		"token",
		// MUST provide service account if you want to use the lookup tables api
		mixpanel.ServiceAccount(0, "user_name", "secret"),
	)

	lookupTableID := "some.lookup.table.id"
	table := [][]string{
		{
			"header 1", "header 2",
		},
		{
			"row_1_col_1", "row_1_col2",
		},
		{
			"row_2_col_1", "row_2_col2",
		},
	}
	if err := mp.LookupTableReplace(ctx, lookupTableID, table); err != nil {
		return err
	}

	fmt.Println("lookupTableReplace successfully called")
	return nil
}
