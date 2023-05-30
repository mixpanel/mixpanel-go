package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/mixpanel/mixpanel-go"
)

func parseTime(day string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func Export() error {
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewApiClient(
		"token",
		// Can use either ApiSecret or ServiceAccount
		mixpanel.ServiceAccount(0, "user_name", "secret"),
	)

	startDate, err := parseTime("2023-05-22")
	if err != nil {
		return err
	}
	endDate, err := parseTime("2023-05-23")
	if err != nil {
		return err
	}

	// We will export all events between startDate and endDate
	events, err := mp.Export(ctx, startDate, endDate, 100, mixpanel.ExportNoEventFilter, mixpanel.ExportNoWhereFilter)
	if err != nil {
		return err
	}

	for _, event := range events {
		fmt.Println(event.Name)
	}

	return nil
}
