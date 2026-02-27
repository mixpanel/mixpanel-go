package examples

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mixpanel/mixpanel-go/v2"
)

func ImportExample() error {
	ctx := context.Background()

	// fill in your token and project id and service account user name and secret
	mp := mixpanel.NewApiClient(
		"token",
		// Can provide service account api secret if you want to use the import api
		mixpanel.ServiceAccount(0, "user_name", "secret"),
	)

	event := mp.NewEvent("import test event", mixpanel.EmptyDistinctID, nil)
	event.AddTime(time.Now())
	event.AddInsertID("insert_id")
	importEvents := []*mixpanel.Event{event}

	success, err := mp.Import(ctx, importEvents, mixpanel.ImportOptionsRecommend)
	if err != nil {
		importFailed := &mixpanel.ImportFailedValidationError{}
		if errors.As(err, importFailed) {
			failedEvents := importFailed.FailedImportRecords
			for _, failedEvent := range failedEvents {
				e := importEvents[failedEvent.Index]
				fmt.Printf("Event %s at index %d failed to import because of %s \n", e.Name, failedEvent.Index, failedEvent.Message)
			}
			return err
		}

		backOffError := &mixpanel.ImportRateLimitError{}
		if errors.As(err, backOffError) {
			// Need to back off
			return err
		}

		return err
	}

	fmt.Printf("Successfully imported %d events\n", success.NumRecordsImported)
	return nil
}
