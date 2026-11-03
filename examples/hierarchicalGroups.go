package examples

import (
	"context"
	"fmt"

	"github.com/mixpanel/mixpanel-go/v2"
)

func HierarchicalGroups() error {
	ctx := context.Background()

	// The project uses hierarchical groups, so users are identified by user id within a company
	mp := mixpanel.NewApiClient("token", mixpanel.HierarchicalGroups())
	user := mixpanel.MixpanelIdentity{UserID: "user_123", CompanyID: "acme"}

	event, err := mp.NewIdentityEvent("Signup", user, map[string]any{"plan": "enterprise"})
	if err != nil {
		return err
	}
	if err := mp.Track(ctx, []*mixpanel.Event{event}); err != nil {
		return err
	}

	// The user's profile is the $user_id group profile within the company
	if err := mp.PeopleSetByIdentity(ctx, user, map[string]any{"role": "admin"}); err != nil {
		return err
	}

	// The company's own profile is its own company, so it needs no ForCompany
	if err := mp.GroupSet(ctx, "$company_id", "acme", map[string]any{"tier": "enterprise"}); err != nil {
		return err
	}

	fmt.Println("Hierarchical groups calls succeeded")
	return nil
}
