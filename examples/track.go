package examples

import (
	"context"
	"fmt"

	"github.com/mixpanel/mixpanel-go"
)

func Track() error {
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewClient(
		"token",
		mixpanel.ProjectID(0),
		mixpanel.ServiceAccount("user_name", "secret"),
	)

	if err := mp.Track(ctx, []*mixpanel.Event{
		mp.NewEvent("test event", mixpanel.EmptyDistinctID, nil),
	}); err != nil {
		return err
	}

	fmt.Println("track successfully called")
	return nil
}
