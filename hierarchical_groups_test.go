package mixpanel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/mixpanel/mixpanel-go/v2/flags"
	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	url  string // path and fragment, e.g. /groups#group-set
	body []byte
}

// captureRequests answers every request with a success response and records it.
func captureRequests(t *testing.T) *[]capturedRequest {
	t.Helper()
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	var requests []capturedRequest
	httpmock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		u := req.URL.Path
		if req.URL.Fragment != "" {
			u += "#" + req.URL.Fragment
		}
		requests = append(requests, capturedRequest{url: u, body: body})

		response := "1"
		switch req.URL.Path {
		case trackURL:
			response = `{"error":"","status":1}`
		case importURL:
			response = `{"code":200,"num_records_imported":1,"status":"OK"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	return &requests
}

// onlyMessage decodes the single profile message of the single captured request.
func onlyMessage(t *testing.T, requests []capturedRequest) (string, map[string]any) {
	t.Helper()
	require.Len(t, requests, 1)
	var messages []map[string]any
	require.NoError(t, json.Unmarshal(requests[0].body, &messages))
	require.Len(t, messages, 1)
	return requests[0].url, messages[0]
}

func hgClient() *ApiClient {
	return NewApiClient("token", HierarchicalGroups())
}

func legacyClient() *ApiClient {
	return NewApiClient("token")
}

func mustForCompany(t *testing.T, client *ApiClient, companyID string) *ApiClient {
	t.Helper()
	bound, err := client.ForCompany(companyID)
	require.NoError(t, err)
	return bound
}

var hgUser = MixpanelIdentity{UserID: "user_123", CompanyID: "acme"}

func TestIdentityEvents(t *testing.T) {
	t.Run("hierarchical event sends user and company without distinct id", func(t *testing.T) {
		event, err := hgClient().NewIdentityEvent("Signup", hgUser, nil)
		require.NoError(t, err)

		require.Equal(t, "Signup", event.Name)
		require.Equal(t, "user_123", event.Properties["$user_id"])
		require.Equal(t, "acme", event.Properties["$company_id"])
		require.NotContains(t, event.Properties, "distinct_id")
		require.NotContains(t, event.Properties, "$device_id")
		require.Equal(t, "token", event.Properties["token"])
		require.Equal(t, goLib, event.Properties[propertyMpLib])
		require.Equal(t, version, event.Properties[propertyLibVersion])
	})

	t.Run("hierarchical event with device id", func(t *testing.T) {
		event, err := hgClient().NewIdentityEvent("Signup",
			MixpanelIdentity{UserID: "user_123", CompanyID: "acme", DeviceID: "device_abc"}, nil)
		require.NoError(t, err)

		require.Equal(t, "device_abc", event.Properties["$device_id"])
		require.NotContains(t, event.Properties, "distinct_id")
	})

	t.Run("company only event", func(t *testing.T) {
		event, err := hgClient().NewIdentityEvent("Renewed", MixpanelIdentity{CompanyID: "acme"}, nil)
		require.NoError(t, err)

		require.Equal(t, "acme", event.Properties["$company_id"])
		require.NotContains(t, event.Properties, "$user_id")
		require.NotContains(t, event.Properties, "distinct_id")
	})

	t.Run("legacy identity sends distinct id", func(t *testing.T) {
		event, err := legacyClient().NewIdentityEvent("Signup", MixpanelIdentity{DistinctID: "user_123"}, nil)
		require.NoError(t, err)

		require.Equal(t, "user_123", event.Properties["distinct_id"])
		require.NotContains(t, event.Properties, "$user_id")
		require.NotContains(t, event.Properties, "$company_id")
	})

	t.Run("identity overrides caller supplied keys and keeps other properties", func(t *testing.T) {
		caller := map[string]any{"$company_id": "wrong", "plan": "enterprise"}

		event, err := hgClient().NewIdentityEvent("Signup", hgUser, caller)
		require.NoError(t, err)

		require.Equal(t, "acme", event.Properties["$company_id"])
		require.Equal(t, "enterprise", event.Properties["plan"])
		require.Equal(t, "wrong", caller["$company_id"], "caller properties must not be modified")
		require.NotContains(t, caller, "$user_id")
	})

	t.Run("hierarchical import event", func(t *testing.T) {
		requests := captureRequests(t)
		mp := hgClient()

		event, err := mp.NewIdentityEvent("Signup", hgUser, nil)
		require.NoError(t, err)
		event.AddInsertID("insert-id")
		_, err = mp.Import(context.Background(), []*Event{event}, ImportOptions{Strict: true})
		require.NoError(t, err)

		require.Len(t, *requests, 1)
		require.Equal(t, importURL, (*requests)[0].url)
		var events []*Event
		require.NoError(t, json.Unmarshal((*requests)[0].body, &events))
		props := events[0].Properties
		require.Equal(t, "user_123", props["$user_id"])
		require.Equal(t, "acme", props["$company_id"])
		require.Equal(t, "insert-id", props["$insert_id"])
		require.NotContains(t, props, "distinct_id")
	})

	t.Run("hierarchical track event", func(t *testing.T) {
		requests := captureRequests(t)
		mp := hgClient()

		event, err := mp.NewIdentityEvent("Signup", hgUser, nil)
		require.NoError(t, err)
		require.NoError(t, mp.Track(context.Background(), []*Event{event}))

		require.Len(t, *requests, 1)
		require.NotContains(t, string((*requests)[0].body), `"distinct_id"`)
	})

	t.Run("identity needs at least one value", func(t *testing.T) {
		for _, mp := range []*ApiClient{hgClient(), legacyClient()} {
			_, err := mp.NewIdentityEvent("Signup", MixpanelIdentity{}, nil)
			require.ErrorIs(t, err, ErrInvalidIdentity)
		}
	})

	t.Run("empty string leaves a value unset", func(t *testing.T) {
		identity := MixpanelIdentity{UserID: "user_123", DeviceID: ""}
		require.Equal(t, "MixpanelIdentity{UserID=user_123}", identity.String())

		event, err := legacyClient().NewIdentityEvent("Signup", identity, nil)
		require.NoError(t, err)
		require.NotContains(t, event.Properties, "$device_id")
	})

	t.Run("string lists set values", func(t *testing.T) {
		identity := MixpanelIdentity{DistinctID: "d", UserID: "u", CompanyID: "c", DeviceID: "v"}
		require.Equal(t, "MixpanelIdentity{DistinctID=d, UserID=u, CompanyID=c, DeviceID=v}", identity.String())
		require.True(t, identity.IsHierarchical())
		require.False(t, MixpanelIdentity{UserID: "u"}.IsHierarchical())
	})

	t.Run("user and device without company is valid for simplified id merge", func(t *testing.T) {
		event, err := legacyClient().NewIdentityEvent("Signup",
			MixpanelIdentity{UserID: "user_123", DeviceID: "device_abc"}, nil)
		require.NoError(t, err)

		require.Equal(t, "user_123", event.Properties["$user_id"])
		require.Equal(t, "device_abc", event.Properties["$device_id"])
		require.NotContains(t, event.Properties, "distinct_id")
		require.NotContains(t, event.Properties, "$company_id")
	})

	t.Run("user without company or device is rejected for hierarchical groups", func(t *testing.T) {
		_, err := hgClient().NewIdentityEvent("Signup", MixpanelIdentity{UserID: "user_123"}, nil)

		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Contains(t, err.Error(), "cannot be merged")
	})

	t.Run("user with device but no company is allowed for hierarchical groups", func(t *testing.T) {
		// Mixpanel ignores the user id and keeps the event under the device, which later links to
		// the company-scoped user. Recoverable, so the library does not refuse it.
		event, err := hgClient().NewIdentityEvent("Signup",
			MixpanelIdentity{UserID: "user_123", DeviceID: "device_abc"}, nil)
		require.NoError(t, err)

		require.Equal(t, "device_abc", event.Properties["$device_id"])
		require.NotContains(t, event.Properties, "$company_id")
	})

	t.Run("anonymous device event works in both modes", func(t *testing.T) {
		for _, mp := range []*ApiClient{hgClient(), legacyClient()} {
			event, err := mp.NewIdentityEvent("Page View", MixpanelIdentity{DeviceID: "device_abc"}, nil)
			require.NoError(t, err)

			require.Equal(t, "device_abc", event.Properties["$device_id"])
			require.NotContains(t, event.Properties, "distinct_id")
			require.NotContains(t, event.Properties, "$user_id")
			require.NotContains(t, event.Properties, "$company_id")
		}
	})

	t.Run("combined distinct id event passes through", func(t *testing.T) {
		// Ingestion parses $company:{company}|$user:{user} back into $company_id and $user_id.
		combined := "$company:acme|$user:user_123"
		mp := hgClient()

		fromIdentity, err := mp.NewIdentityEvent("Signup", MixpanelIdentity{DistinctID: combined}, nil)
		require.NoError(t, err)
		for _, event := range []*Event{mp.NewEvent("Signup", combined, nil), fromIdentity} {
			require.Equal(t, combined, event.Properties["distinct_id"])
			require.NotContains(t, event.Properties, "$user_id")
		}
	})

	t.Run("raw distinct id events are not refused", func(t *testing.T) {
		requests := captureRequests(t)
		ctx := context.Background()

		for _, mp := range []*ApiClient{hgClient(), legacyClient()} {
			require.NoError(t, mp.Track(ctx, []*Event{mp.NewEvent("Signup", "user_123", nil)}))
			_, err := mp.Import(ctx, []*Event{mp.NewEvent("Signup", "user_123", nil)}, ImportOptions{})
			require.NoError(t, err)
		}
		require.Len(t, *requests, 4)
		for _, r := range *requests {
			require.Contains(t, string(r.body), `"distinct_id":"user_123"`)
		}
	})

	t.Run("hierarchical identity requires hierarchical client", func(t *testing.T) {
		mp := legacyClient()

		_, err := mp.NewIdentityEvent("Signup", hgUser, nil)
		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Contains(t, err.Error(), "HierarchicalGroups()")

		_, err = mp.NewIdentityEvent("Renewed", MixpanelIdentity{CompanyID: "acme"}, nil)
		require.ErrorIs(t, err, ErrInvalidIdentity)
	})
}

func TestIdentityUserProfiles(t *testing.T) {
	ctx := context.Background()
	props := map[string]any{"plan": "pro"}

	t.Run("device identity cannot identify a profile", func(t *testing.T) {
		requests := captureRequests(t)
		for _, mp := range []*ApiClient{hgClient(), legacyClient()} {
			err := mp.PeopleSetByIdentity(ctx, MixpanelIdentity{DeviceID: "device_abc"}, props)
			require.ErrorIs(t, err, ErrInvalidIdentity)
		}
		require.Empty(t, *requests)
	})

	t.Run("increment and append are refused for hierarchical groups", func(t *testing.T) {
		requests := captureRequests(t)
		for _, mp := range []*ApiClient{hgClient(), mustForCompany(t, hgClient(), "acme")} {
			require.ErrorIs(t, mp.PeopleIncrement(ctx, "user_123", map[string]int{"logins": 1}),
				ErrUnsupportedHierarchicalOperation)
			require.ErrorIs(t, mp.PeopleAppendListProperty(ctx, "user_123", props),
				ErrUnsupportedHierarchicalOperation)
		}
		require.Empty(t, *requests)
	})

	t.Run("increment error names what is supported", func(t *testing.T) {
		err := hgClient().PeopleIncrement(ctx, "user_123", map[string]int{"logins": 1})

		require.Contains(t, err.Error(), "$add")
		require.Contains(t, err.Error(), "$set, $set_once, $unset, $union, $remove, $delete")
		require.Contains(t, err.Error(), "PeopleIncrement and PeopleAppendListProperty")
	})

	t.Run("distinct id people methods send a combined distinct id to groups", func(t *testing.T) {
		mp := hgClient()
		id := "$company:acme|$user:user_123"
		cases := []struct {
			url       string
			operation string
			call      func() error
		}{
			{groupSetUrl, "$set", func() error { return mp.PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}) }},
			{groupsSetOnceUrl, "$set_once", func() error { return mp.PeopleSetOnce(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}) }},
			{groupsDeleteGroupUrl, "$delete", func() error { return mp.PeopleDeleteProfile(ctx, id, true) }},
			{groupsRemoveFromListPropertyUrl, "$remove", func() error { return mp.PeopleRemoveListProperty(ctx, id, props) }},
			{groupsUnionListPropertyUrl, "$union", func() error {
				return mp.PeopleUnionProperty(ctx, id, map[string]any{"tags": []string{"a"}})
			}},
			{groupsDeletePropertyUrl, "$unset", func() error { return mp.PeopleDeleteProperty(ctx, id, []string{"plan"}) }},
		}
		for _, c := range cases {
			t.Run(c.operation, func(t *testing.T) {
				requests := captureRequests(t)
				require.NoError(t, c.call())

				u, message := onlyMessage(t, *requests)
				require.Equal(t, c.url, u)
				requireKeys(t, message, "$token", "$group_key", "$group_id", "$company_id", c.operation)
				require.Equal(t, "$user_id", message["$group_key"])
				require.Equal(t, "user_123", message["$group_id"])
				require.Equal(t, "acme", message["$company_id"])
			})
		}
	})

	t.Run("combined distinct id matches the hierarchical identity", func(t *testing.T) {
		requests := captureRequests(t)
		mp := hgClient()

		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties("$company:acme|$user:user_123", props)}))
		require.NoError(t, mp.PeopleSetByIdentity(ctx, hgUser, props))

		require.Len(t, *requests, 2)
		require.Equal(t, (*requests)[1], (*requests)[0])
	})

	t.Run("distinct id people methods reject other distinct ids", func(t *testing.T) {
		mp := hgClient()
		ids := []string{
			"user_123",
			"$company:acme|$user:",         // missing user
			"$company:|$user:user_123",     // missing company
			"$company:acme",                // no user part
			"$user:user_123|$company:acme", // wrong order
			"company:acme|$user:user_123",  // wrong prefix
			"$Company:acme|$user:user_123", // prefix is case sensitive
			"",
		}
		for _, id := range ids {
			t.Run(id, func(t *testing.T) {
				requests := captureRequests(t)
				calls := []func() error{
					func() error { return mp.PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}) },
					func() error { return mp.PeopleSetOnce(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}) },
					func() error { return mp.PeopleDeleteProfile(ctx, id, false) },
					func() error { return mp.PeopleRemoveListProperty(ctx, id, props) },
					func() error { return mp.PeopleUnionProperty(ctx, id, props) },
					func() error { return mp.PeopleDeleteProperty(ctx, id, []string{"plan"}) },
					func() error { return mp.PeopleSetByIdentity(ctx, MixpanelIdentity{DistinctID: id}, props) },
				}
				for _, call := range calls {
					require.ErrorIs(t, call(), ErrInvalidIdentity)
				}
				require.Empty(t, *requests)
			})
		}
	})

	t.Run("distinct id error explains how to address a profile", func(t *testing.T) {
		err := hgClient().PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties("user_123", props)})

		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Contains(t, err.Error(), "need a user and a company")
		require.Contains(t, err.Error(), "MixpanelIdentity{UserID: userID, CompanyID: companyID}")
		require.Contains(t, err.Error(), "$company:{company}|$user:{user}")
		require.Contains(t, err.Error(), `"user_123"`)
	})

	t.Run("batch with one bad distinct id sends nothing", func(t *testing.T) {
		requests := captureRequests(t)
		people := []*PeopleProperties{
			NewPeopleProperties("$company:acme|$user:user_123", props),
			NewPeopleProperties("user_456", props),
		}

		require.ErrorIs(t, hgClient().PeopleSet(ctx, people), ErrInvalidIdentity)
		require.ErrorIs(t, hgClient().PeopleSetOnce(ctx, people), ErrInvalidIdentity)
		require.Empty(t, *requests)
	})

	t.Run("batch of combined distinct ids is one groups request", func(t *testing.T) {
		requests := captureRequests(t)
		people := []*PeopleProperties{
			NewPeopleProperties("$company:acme|$user:user_123", map[string]any{"plan": "pro"}),
			NewPeopleProperties("$company:globex|$user:user_456", map[string]any{"plan": "free"}),
		}
		people[1].SetIp(net.ParseIP("10.0.0.1"))

		require.NoError(t, hgClient().PeopleSet(ctx, people))

		require.Len(t, *requests, 1)
		require.Equal(t, groupSetUrl, (*requests)[0].url)
		require.JSONEq(t, `[
			{"$token":"token","$group_key":"$user_id","$group_id":"user_123","$company_id":"acme","$set":{"plan":"pro"}},
			{"$token":"token","$group_key":"$user_id","$group_id":"user_456","$company_id":"globex","$set":{"plan":"free","$ip":"10.0.0.1"}}
		]`, string((*requests)[0].body))
	})

	t.Run("identity set reroutes to user group profile", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, hgClient().PeopleSetByIdentity(ctx, hgUser, map[string]any{"role": "admin"}))

		u, message := onlyMessage(t, *requests)
		require.Equal(t, groupSetUrl, u)
		require.Equal(t, "$user_id", message["$group_key"])
		require.Equal(t, "user_123", message["$group_id"])
		require.Equal(t, "acme", message["$company_id"])
		require.Equal(t, "admin", message["$set"].(map[string]any)["role"])
		require.NotContains(t, message["$set"], "$company_id")
		require.NotContains(t, message, "$distinct_id")
	})

	t.Run("every identity profile operation reroutes", func(t *testing.T) {
		mp := hgClient()
		cases := []struct {
			url       string
			operation string
			call      func() error
		}{
			{groupSetUrl, "$set", func() error { return mp.PeopleSetByIdentity(ctx, hgUser, props) }},
			{groupsSetOnceUrl, "$set_once", func() error { return mp.PeopleSetOnceByIdentity(ctx, hgUser, props) }},
			{groupsDeleteGroupUrl, "$delete", func() error { return mp.PeopleDeleteProfileByIdentity(ctx, hgUser) }},
			{groupsRemoveFromListPropertyUrl, "$remove", func() error { return mp.PeopleRemoveListPropertyByIdentity(ctx, hgUser, props) }},
			{groupsUnionListPropertyUrl, "$union", func() error {
				return mp.PeopleUnionPropertyByIdentity(ctx, hgUser, map[string]any{"tags": []string{"a"}})
			}},
			{groupsDeletePropertyUrl, "$unset", func() error { return mp.PeopleDeletePropertyByIdentity(ctx, hgUser, []string{"plan"}) }},
		}
		for _, c := range cases {
			t.Run(c.operation, func(t *testing.T) {
				requests := captureRequests(t)
				require.NoError(t, c.call())

				u, message := onlyMessage(t, *requests)
				require.Equal(t, c.url, u)
				require.Contains(t, message, c.operation)
				require.Equal(t, "acme", message["$company_id"])
				require.Equal(t, "$user_id", message["$group_key"])
				require.Equal(t, "user_123", message["$group_id"])
			})
		}
	})

	t.Run("hierarchical identity requires hierarchical client", func(t *testing.T) {
		requests := captureRequests(t)

		err := legacyClient().PeopleSetByIdentity(ctx, hgUser, map[string]any{"role": "admin"})

		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Empty(t, *requests)
	})

	t.Run("identity company wins over ForCompany", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, mustForCompany(t, hgClient(), "other").PeopleSetByIdentity(ctx, hgUser, props))

		_, message := onlyMessage(t, *requests)
		require.Equal(t, "acme", message["$company_id"])
	})

	t.Run("legacy distinct id identity uses engage", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, legacyClient().PeopleSetByIdentity(ctx, MixpanelIdentity{DistinctID: "user_123"},
			map[string]any{"role": "admin"}))

		u, message := onlyMessage(t, *requests)
		require.Equal(t, peopleSetURL, u)
		require.Equal(t, "user_123", message["$distinct_id"])
		require.Equal(t, "admin", message["$set"].(map[string]any)["role"])
	})

	t.Run("distinct id identity profile update goes to groups on hierarchical client", func(t *testing.T) {
		requests := captureRequests(t)
		combined := "$company:acme|$user:user_123"

		require.NoError(t, hgClient().PeopleSetByIdentity(ctx, MixpanelIdentity{DistinctID: combined}, props))

		u, message := onlyMessage(t, *requests)
		require.Equal(t, groupSetUrl, u)
		requireKeys(t, message, "$token", "$group_key", "$group_id", "$company_id", "$set")
		require.Equal(t, "$user_id", message["$group_key"])
		require.Equal(t, "user_123", message["$group_id"])
		require.Equal(t, "acme", message["$company_id"])
	})

	t.Run("distinct id identity delete matches PeopleDeleteProfile", func(t *testing.T) {
		requests := captureRequests(t)
		mp := legacyClient()

		require.NoError(t, mp.PeopleDeleteProfileByIdentity(ctx, MixpanelIdentity{DistinctID: "user_123"}))
		require.NoError(t, mp.PeopleDeleteProfile(ctx, "user_123", false))

		require.Len(t, *requests, 2)
		require.Equal(t, (*requests)[1], (*requests)[0])
	})

	t.Run("company only identity profile update is rejected", func(t *testing.T) {
		err := hgClient().PeopleSetByIdentity(ctx, MixpanelIdentity{CompanyID: "acme"}, map[string]any{"tier": "ent"})

		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Contains(t, err.Error(), "CompanySet")
	})

	t.Run("profile update rejects empty identity", func(t *testing.T) {
		require.ErrorIs(t, hgClient().PeopleSetByIdentity(ctx, MixpanelIdentity{}, props), ErrInvalidIdentity)
	})

	t.Run("legacy client cannot address a profile without distinct id", func(t *testing.T) {
		err := legacyClient().PeopleSetByIdentity(ctx, MixpanelIdentity{UserID: "user_123", DeviceID: "device_abc"}, props)

		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Contains(t, err.Error(), "distinct id")
	})

	t.Run("hierarchical user without company is company required", func(t *testing.T) {
		requests := captureRequests(t)

		// Valid for events (the device id rescues it) but a profile needs the company.
		err := hgClient().PeopleSetByIdentity(ctx, MixpanelIdentity{UserID: "user_123", DeviceID: "device_abc"}, props)

		require.ErrorIs(t, err, ErrCompanyRequired)
		require.Empty(t, *requests)
	})
}

func TestHierarchicalGroupProfiles(t *testing.T) {
	ctx := context.Background()
	props := map[string]any{"plan": "enterprise"}

	t.Run("ForCompany adds top level company id", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, mustForCompany(t, hgClient(), "acme").GroupSet(ctx, "department", "finance", props))

		u, message := onlyMessage(t, *requests)
		require.Equal(t, groupSetUrl, u)
		require.Equal(t, "acme", message["$company_id"])
		require.Equal(t, "department", message["$group_key"])
		require.Equal(t, "finance", message["$group_id"])
		require.NotContains(t, message["$set"], "$company_id")
	})

	t.Run("user and company profiles have bare group ids", func(t *testing.T) {
		acme := mustForCompany(t, hgClient(), "acme")

		requests := captureRequests(t)
		require.NoError(t, acme.GroupSet(ctx, "$user_id", "user_123", map[string]any{"role": "admin"}))
		_, user := onlyMessage(t, *requests)
		require.Equal(t, "$user_id", user["$group_key"])
		require.Equal(t, "user_123", user["$group_id"])
		require.Equal(t, "acme", user["$company_id"])

		requests = captureRequests(t)
		require.NoError(t, acme.GroupSet(ctx, "$company_id", "acme", map[string]any{"tier": "ent"}))
		_, company := onlyMessage(t, *requests)
		require.Equal(t, "$company_id", company["$group_key"])
		require.Equal(t, "acme", company["$group_id"])
		require.Equal(t, "acme", company["$company_id"])
	})

	groupCalls := func(mp *ApiClient) []func() error {
		return []func() error{
			func() error { return mp.GroupSet(ctx, "department", "finance", props) },
			func() error { return mp.GroupSetOnce(ctx, "department", "finance", props) },
			func() error { return mp.GroupDelete(ctx, "department", "finance") },
			func() error { return mp.GroupRemoveListProperty(ctx, "department", "finance", props) },
			func() error {
				return mp.GroupUnionListProperty(ctx, "department", "finance", map[string]any{"tags": []string{"a"}})
			},
			func() error { return mp.GroupDeleteProperty(ctx, "department", "finance", []string{"plan"}) },
		}
	}

	t.Run("every group operation carries company", func(t *testing.T) {
		requests := captureRequests(t)
		for _, call := range groupCalls(mustForCompany(t, hgClient(), "acme")) {
			require.NoError(t, call())
		}

		require.Len(t, *requests, 6)
		for _, r := range *requests {
			var messages []map[string]any
			require.NoError(t, json.Unmarshal(r.body, &messages))
			require.Equal(t, "acme", messages[0]["$company_id"], r.url)
		}
	})

	t.Run("hierarchical group without company is rejected", func(t *testing.T) {
		requests := captureRequests(t)
		for _, call := range groupCalls(hgClient()) {
			err := call()
			require.ErrorIs(t, err, ErrCompanyRequired)
			require.Contains(t, err.Error(), "ForCompany")
		}
		require.Empty(t, *requests)
	})

	t.Run("legacy group unchanged", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, legacyClient().GroupSet(ctx, "company", "Acme Inc.", map[string]any{"plan": "x"}))

		require.Len(t, *requests, 1)
		require.JSONEq(t, `[{"$token":"token","$group_key":"company","$group_id":"Acme Inc.","$set":{"plan":"x"}}]`,
			string((*requests)[0].body))
	})

	t.Run("ForCompany on a legacy client adds company id", func(t *testing.T) {
		requests := captureRequests(t)

		require.NoError(t, mustForCompany(t, legacyClient(), "acme").GroupSet(ctx, "company", "acme", props))

		_, message := onlyMessage(t, *requests)
		require.Equal(t, "acme", message["$company_id"])
	})

	companyCalls := []struct {
		name    string
		company func(mp *ApiClient) error
		group   func(mp *ApiClient) error
	}{
		{
			"CompanySet",
			func(mp *ApiClient) error { return mp.CompanySet(ctx, props) },
			func(mp *ApiClient) error { return mp.GroupSet(ctx, "$company_id", "acme", props) },
		},
		{
			"CompanySetOnce",
			func(mp *ApiClient) error { return mp.CompanySetOnce(ctx, props) },
			func(mp *ApiClient) error { return mp.GroupSetOnce(ctx, "$company_id", "acme", props) },
		},
		{
			"CompanyDeleteProperty",
			func(mp *ApiClient) error { return mp.CompanyDeleteProperty(ctx, []string{"plan"}) },
			func(mp *ApiClient) error { return mp.GroupDeleteProperty(ctx, "$company_id", "acme", []string{"plan"}) },
		},
		{
			"CompanyRemoveListProperty",
			func(mp *ApiClient) error { return mp.CompanyRemoveListProperty(ctx, props) },
			func(mp *ApiClient) error { return mp.GroupRemoveListProperty(ctx, "$company_id", "acme", props) },
		},
		{
			"CompanyUnionListProperty",
			func(mp *ApiClient) error {
				return mp.CompanyUnionListProperty(ctx, map[string]any{"tags": []string{"a"}})
			},
			func(mp *ApiClient) error {
				return mp.GroupUnionListProperty(ctx, "$company_id", "acme", map[string]any{"tags": []string{"a"}})
			},
		},
		{
			"CompanyDelete",
			func(mp *ApiClient) error { return mp.CompanyDelete(ctx) },
			func(mp *ApiClient) error { return mp.GroupDelete(ctx, "$company_id", "acme") },
		},
	}

	t.Run("Company methods send the same bytes as Group with the bound company", func(t *testing.T) {
		for _, client := range []struct {
			name string
			mp   *ApiClient
		}{{"hierarchical", hgClient()}, {"legacy", legacyClient()}} {
			acme := mustForCompany(t, client.mp, "acme")
			for _, c := range companyCalls {
				t.Run(client.name+"/"+c.name, func(t *testing.T) {
					requests := captureRequests(t)
					require.NoError(t, c.company(acme))
					require.NoError(t, c.group(acme))

					require.Len(t, *requests, 2)
					require.Equal(t, (*requests)[1].url, (*requests)[0].url)
					require.Equal(t, string((*requests)[1].body), string((*requests)[0].body))
					_, message := onlyMessage(t, (*requests)[:1])
					require.Equal(t, "$company_id", message["$group_key"])
					require.Equal(t, "acme", message["$group_id"])
					require.Equal(t, "acme", message["$company_id"])
				})
			}
		}
	})

	t.Run("Company methods without a bound company are rejected", func(t *testing.T) {
		for _, mp := range []*ApiClient{hgClient(), legacyClient()} {
			requests := captureRequests(t)
			for _, c := range companyCalls {
				err := c.company(mp)
				require.ErrorIs(t, err, ErrCompanyRequired, c.name)
				require.Contains(t, err.Error(), "ForCompany", c.name)
				require.Contains(t, err.Error(), c.name)
			}
			require.Empty(t, *requests)
		}
	})

	t.Run("ForCompany rejects empty", func(t *testing.T) {
		bound, err := hgClient().ForCompany("")
		require.ErrorIs(t, err, ErrInvalidIdentity)
		require.Nil(t, bound)
	})

	t.Run("ForCompany returns a copy sharing configuration", func(t *testing.T) {
		httpClient := &http.Client{}
		mp := NewApiClient("token", HierarchicalGroups(), HttpClient(httpClient), EuResidency(),
			ServiceAccount(117, "user", "secret"))

		acme := mustForCompany(t, mp, "acme")

		require.NotSame(t, mp, acme)
		require.Empty(t, mp.companyID, "the receiver must be unchanged")
		require.Equal(t, "acme", acme.companyID)
		require.Same(t, mp.client, acme.client)
		require.Same(t, mp.serviceAccount, acme.serviceAccount)
		require.Same(t, mp.debugHttpCall, acme.debugHttpCall)
		require.Equal(t, mp.token, acme.token)
		require.Equal(t, mp.apiEndpoint, acme.apiEndpoint)
		require.Equal(t, mp.dataEndpoint, acme.dataEndpoint)
		require.True(t, acme.hierarchicalGroups)

		other := mustForCompany(t, acme, "other")
		require.Equal(t, "acme", acme.companyID)
		require.Equal(t, "other", other.companyID)
	})
}

// TestHierarchicalGroupsLeavesValidMessagesUnchanged checks that the option only adds validation:
// every pre-existing call that is valid on both kinds of client sends the same bytes.
func TestHierarchicalGroupsLeavesValidMessagesUnchanged(t *testing.T) {
	ctx := context.Background()
	id := "$company:acme|$user:user_123"
	props := map[string]any{"plan": "pro"}

	sendEvents := func(t *testing.T, mp *ApiClient) []capturedRequest {
		requests := captureRequests(t)
		event := mp.NewEvent("Signup", id, map[string]any{"plan": "pro"})
		event.AddInsertID("insert-id")
		require.NoError(t, mp.Track(ctx, []*Event{event}))
		_, err := mp.Import(ctx, []*Event{event}, ImportOptions{})
		require.NoError(t, err)
		return *requests
	}
	sendProfiles := func(t *testing.T, mp *ApiClient) []capturedRequest {
		requests := captureRequests(t)
		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}))
		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{NewPeopleProperties(id, props)}))
		require.NoError(t, mp.PeopleUnionProperty(ctx, id, props))
		require.NoError(t, mp.PeopleRemoveListProperty(ctx, id, props))
		require.NoError(t, mp.PeopleDeleteProperty(ctx, id, []string{"plan"}))
		require.NoError(t, mp.PeopleDeleteProfile(ctx, id, true))
		return *requests
	}

	legacyEvents := sendEvents(t, legacyClient())
	require.Len(t, legacyEvents, 2)
	require.Equal(t, legacyEvents, sendEvents(t, hgClient()))

	t.Run("profile updates go to engage on a legacy client and groups on a hierarchical client", func(t *testing.T) {
		legacy := sendProfiles(t, legacyClient())
		require.Len(t, legacy, 6)
		require.JSONEq(t, `[{"$token":"token","$distinct_id":"$company:acme|$user:user_123","$set":{"plan":"pro"},"$ip":"0"}]`,
			string(legacy[0].body))
		require.JSONEq(t, `[{"$token":"token","$distinct_id":"$company:acme|$user:user_123","$delete":"null","$ignore_alias":"true"}]`,
			string(legacy[5].body))
		for _, r := range legacy {
			require.True(t, strings.HasPrefix(r.url, "/engage#"), r.url)
		}

		hierarchical := sendProfiles(t, hgClient())
		require.Len(t, hierarchical, 6)
		for _, r := range hierarchical {
			require.True(t, strings.HasPrefix(r.url, "/groups#"), r.url)
			require.NotContains(t, string(r.body), "$distinct_id")
		}
	})

	t.Run("increment and append are unchanged on a legacy client", func(t *testing.T) {
		requests := captureRequests(t)
		mp := legacyClient()

		require.NoError(t, mp.PeopleIncrement(ctx, "user_123", map[string]int{"logins": 1}))
		require.NoError(t, mp.PeopleAppendListProperty(ctx, "user_123", props))

		require.Len(t, *requests, 2)
		require.JSONEq(t, `[{"$token":"token","$distinct_id":"user_123","$add":{"logins":1}}]`, string((*requests)[0].body))
		require.JSONEq(t, `[{"$token":"token","$distinct_id":"user_123","$append":{"plan":"pro"}}]`, string((*requests)[1].body))
	})

	t.Run("groups are unchanged on a legacy client", func(t *testing.T) {
		requests := captureRequests(t)
		mp := legacyClient()

		require.NoError(t, mp.GroupSet(ctx, "k", "g", props))
		require.NoError(t, mp.GroupSetOnce(ctx, "k", "g", props))
		require.NoError(t, mp.GroupDeleteProperty(ctx, "k", "g", []string{"plan"}))
		require.NoError(t, mp.GroupRemoveListProperty(ctx, "k", "g", props))
		require.NoError(t, mp.GroupUnionListProperty(ctx, "k", "g", props))
		require.NoError(t, mp.GroupDelete(ctx, "k", "g"))

		expected := []string{
			`[{"$token":"token","$group_key":"k","$group_id":"g","$set":{"plan":"pro"}}]`,
			`[{"$token":"token","$group_key":"k","$group_id":"g","$set_once":{"plan":"pro"}}]`,
			`[{"$token":"token","$group_key":"k","$group_id":"g","$unset":["plan"]}]`,
			`[{"$token":"token","$group_key":"k","$group_id":"g","$remove":{"plan":"pro"}}]`,
			`[{"$token":"token","$group_key":"k","$group_id":"g","$union":{"plan":"pro"}}]`,
			`[{"$token":"token","$group_key":"k","$group_id":"g","$delete":"null"}]`,
		}
		require.Len(t, *requests, len(expected))
		for i, want := range expected {
			require.JSONEq(t, want, string((*requests)[i].body))
		}
	})
}

func TestErrorsAreDistinct(t *testing.T) {
	require.False(t, errors.Is(ErrInvalidIdentity, ErrCompanyRequired))
	require.False(t, errors.Is(ErrCompanyRequired, ErrUnsupportedHierarchicalOperation))
	require.False(t, errors.Is(ErrUnsupportedHierarchicalOperation, ErrInvalidIdentity))
}

// flagExposures evaluates test-flag on a client's remote flags provider and returns the
// properties of the exposure events it tracked.
func flagExposures(t *testing.T, client *ApiClient, flagContext flags.FlagContext) []map[string]any {
	t.Helper()
	requests := captureRequests(t)
	httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
		httpmock.NewStringResponder(200, `{"code":200,"flags":{"test-flag":{"variant_key":"on","variant_value":true}}}`))

	enabled, err := client.RemoteFlags.IsEnabled(context.Background(), "test-flag", flagContext)
	require.NoError(t, err)
	require.True(t, enabled)

	var exposures []map[string]any
	for _, request := range *requests {
		require.Equal(t, trackURL, request.url)
		var events []Event
		require.NoError(t, json.Unmarshal(request.body, &events))
		for _, event := range events {
			require.Equal(t, "$experiment_started", event.Name)
			exposures = append(exposures, event.Properties)
		}
	}
	return exposures
}

func TestHierarchicalGroupsFlagExposure(t *testing.T) {
	hgContext := flags.FlagContext{"$user_id": "user_123", "$company_id": "acme"}

	t.Run("flags providers inherit the option in either order", func(t *testing.T) {
		for name, options := range map[string][]Options{
			"option first": {HierarchicalGroups(), WithRemoteFlags(flags.DefaultRemoteFlagsConfig(), nil)},
			"option last":  {WithRemoteFlags(flags.DefaultRemoteFlagsConfig(), nil), HierarchicalGroups()},
		} {
			t.Run(name, func(t *testing.T) {
				exposures := flagExposures(t, NewApiClient("token", options...), hgContext)

				require.Len(t, exposures, 1)
				require.Equal(t, "user_123", exposures[0]["$user_id"])
				require.Equal(t, "acme", exposures[0]["$company_id"])
				require.NotContains(t, exposures[0], "distinct_id")
				require.Equal(t, "token", exposures[0]["token"])
			})
		}
	})

	t.Run("local flags inherit the option in either order", func(t *testing.T) {
		config := flags.DefaultLocalFlagsConfig()
		config.EnablePolling = false
		for name, options := range map[string][]Options{
			"option first": {HierarchicalGroups(), WithLocalFlags(config)},
			"option last":  {WithLocalFlags(config), HierarchicalGroups()},
		} {
			t.Run(name, func(t *testing.T) {
				requests := captureRequests(t)
				httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
					httpmock.NewStringResponder(200, `{"flags":[{"id":"1","key":"test-flag","context":"distinct_id",
						"context_value_template":"$company:{$company_id}|$user:{$user_id}",
						"ruleset":{"variants":[{"key":"on","value":true,"split":1}],"rollout":[{"rollout_percentage":1}]}}]}`))
				client := NewApiClient("token", options...)
				require.NoError(t, client.LocalFlags.StartPollingForDefinitions(context.Background()))

				enabled, err := client.LocalFlags.IsEnabled(context.Background(), "test-flag", hgContext)
				require.NoError(t, err)
				require.True(t, enabled)

				require.Len(t, *requests, 1)
				var events []Event
				require.NoError(t, json.Unmarshal((*requests)[0].body, &events))
				require.Len(t, events, 1)
				require.Equal(t, "acme", events[0].Properties["$company_id"])
				require.NotContains(t, events[0].Properties, "distinct_id")
			})
		}
	})

	t.Run("hierarchical exposure keeps a distinct id the context sent", func(t *testing.T) {
		exposures := flagExposures(t, NewApiClient("token", HierarchicalGroups(), WithRemoteFlags(flags.DefaultRemoteFlagsConfig(), nil)),
			flags.FlagContext{"distinct_id": "$company:acme|$user:user_123", "$user_id": "user_123", "$company_id": "acme"})

		require.Len(t, exposures, 1)
		require.Equal(t, "$company:acme|$user:user_123", exposures[0]["distinct_id"])
		require.Equal(t, "acme", exposures[0]["$company_id"])
	})

	t.Run("legacy client exposure is unchanged", func(t *testing.T) {
		client := NewApiClient("token", WithRemoteFlags(flags.DefaultRemoteFlagsConfig(), nil))

		require.Empty(t, flagExposures(t, client, hgContext))

		exposures := flagExposures(t, client, flags.FlagContext{"distinct_id": "", "$company_id": "acme"})
		require.Len(t, exposures, 1)
		require.Contains(t, exposures[0], "distinct_id")
		require.Equal(t, "", exposures[0]["distinct_id"])
		require.NotContains(t, exposures[0], "$company_id")
	})

	t.Run("custom tracker receives an empty distinct id", func(t *testing.T) {
		var gotDistinctID *string
		builder := func(*ApiClient) flags.Tracker {
			return func(distinctID string, _ string, props map[string]any) {
				gotDistinctID = &distinctID
				require.Equal(t, "acme", props["$company_id"])
			}
		}
		client := NewApiClient("token", WithRemoteFlags(flags.DefaultRemoteFlagsConfig(), builder), HierarchicalGroups())

		require.Empty(t, flagExposures(t, client, hgContext))
		require.NotNil(t, gotDistinctID)
		require.Equal(t, "", *gotDistinctID)
	})
}
