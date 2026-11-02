package mixpanel

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	scenarioUserID    = "my_user_id"
	scenarioCompanyID = "my_company_id"
	scenarioTeamID    = "my_team_id"
	scenarioDeviceID  = "my_device_id"

	scenarioCombinedDistinctID = "$company:" + scenarioCompanyID + "|$user:" + scenarioUserID
)

var (
	scenarioUser          = MixpanelIdentity{UserID: scenarioUserID, CompanyID: scenarioCompanyID}
	scenarioUserAndDevice = MixpanelIdentity{UserID: scenarioUserID, CompanyID: scenarioCompanyID, DeviceID: scenarioDeviceID}
)

// eventKeys are the properties every event built by NewIdentityEvent carries besides its identity.
var eventKeys = []string{"token", "mp_lib", "$lib_version"}

func requireKeys(t *testing.T, m map[string]any, want ...string) {
	t.Helper()
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	want = append([]string(nil), want...)
	sort.Strings(want)
	require.Equal(t, want, got)
}

// onlyEvent decodes the single event of the single captured request.
func onlyEvent(t *testing.T, requests []capturedRequest) (string, map[string]any) {
	t.Helper()
	require.Len(t, requests, 1)
	var events []map[string]any
	require.NoError(t, json.Unmarshal(requests[0].body, &events))
	require.Len(t, events, 1)
	properties, ok := events[0]["properties"].(map[string]any)
	require.True(t, ok)
	return requests[0].url, properties
}

func trackIdentity(t *testing.T, mp *ApiClient, identity MixpanelIdentity, properties map[string]any) {
	t.Helper()
	event, err := mp.NewIdentityEvent("Scenario Event", identity, properties)
	require.NoError(t, err)
	require.NoError(t, mp.Track(context.Background(), []*Event{event}))
}

// requireUserProfileUpdate checks the /groups shape of a hierarchical groups user profile update.
func requireUserProfileUpdate(t *testing.T, requests []capturedRequest, wantURL, operation string) map[string]any {
	t.Helper()
	u, message := onlyMessage(t, requests)
	require.Equal(t, wantURL, u)
	requireKeys(t, message, "$token", "$group_key", "$group_id", "$company_id", operation)
	require.Equal(t, "token", message["$token"])
	require.Equal(t, "$user_id", message["$group_key"])
	require.Equal(t, scenarioUserID, message["$group_id"])
	require.Equal(t, scenarioCompanyID, message["$company_id"])
	return message
}

// requireGroupProfileUpdate checks the /groups shape of a group profile update bound to the company.
func requireGroupProfileUpdate(t *testing.T, requests []capturedRequest, wantURL, groupKey, groupID, operation string) map[string]any {
	t.Helper()
	u, message := onlyMessage(t, requests)
	require.Equal(t, wantURL, u)
	requireKeys(t, message, "$token", "$group_key", "$group_id", "$company_id", operation)
	require.Equal(t, "token", message["$token"])
	require.Equal(t, groupKey, message["$group_key"])
	require.Equal(t, groupID, message["$group_id"])
	require.Equal(t, scenarioCompanyID, message["$company_id"])
	return message
}

// Events

func TestScenarioTrackAnonymous(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), MixpanelIdentity{DeviceID: scenarioDeviceID}, nil)

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "$device_id")...)
	require.Equal(t, scenarioDeviceID, props["$device_id"])
}

func TestScenarioTrackIdentified(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), scenarioUserAndDevice, nil)

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "$user_id", "$company_id", "$device_id")...)
	require.Equal(t, scenarioUserID, props["$user_id"])
	require.Equal(t, scenarioCompanyID, props["$company_id"])
	require.Equal(t, scenarioDeviceID, props["$device_id"])
}

func TestScenarioTrackIdentifiedWithoutDevice(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), scenarioUser, nil)

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "$user_id", "$company_id")...)
	require.Equal(t, scenarioUserID, props["$user_id"])
	require.Equal(t, scenarioCompanyID, props["$company_id"])
}

func TestScenarioTrackWithCustomGroup(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), scenarioUser, map[string]any{"team_id": scenarioTeamID})

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "$user_id", "$company_id", "team_id")...)
	require.Equal(t, scenarioTeamID, props["team_id"])
}

func TestScenarioImportIdentified(t *testing.T) {
	requests := captureRequests(t)
	mp := hgClient()

	event, err := mp.NewIdentityEvent("Scenario Event", scenarioUserAndDevice, nil)
	require.NoError(t, err)
	_, err = mp.Import(context.Background(), []*Event{event}, ImportOptions{Strict: true})
	require.NoError(t, err)

	u, props := onlyEvent(t, *requests)
	require.Equal(t, importURL, u)
	requireKeys(t, props, append(eventKeys, "$user_id", "$company_id", "$device_id")...)
	require.Equal(t, scenarioUserID, props["$user_id"])
	require.Equal(t, scenarioCompanyID, props["$company_id"])
	require.Equal(t, scenarioDeviceID, props["$device_id"])
}

func TestScenarioIdentityOverridesProperties(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), scenarioUser, map[string]any{"$user_id": "other_user", "$company_id": "other_company"})

	_, props := onlyEvent(t, *requests)
	require.Equal(t, scenarioUserID, props["$user_id"])
	require.Equal(t, scenarioCompanyID, props["$company_id"])
}

func TestScenarioTrackWithCombinedDistinctID(t *testing.T) {
	requests := captureRequests(t)
	mp := hgClient()

	require.NoError(t, mp.Track(context.Background(), []*Event{mp.NewEvent("Scenario Event", scenarioCombinedDistinctID, nil)}))

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "distinct_id")...)
	require.Equal(t, scenarioCombinedDistinctID, props["distinct_id"])
}

func TestScenarioTrackWithCombinedDistinctIDIdentity(t *testing.T) {
	requests := captureRequests(t)

	trackIdentity(t, hgClient(), MixpanelIdentity{DistinctID: scenarioCombinedDistinctID}, nil)

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "distinct_id")...)
	require.Equal(t, scenarioCombinedDistinctID, props["distinct_id"])
}

// User profile

func TestScenarioSetUserProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleSetByIdentity(context.Background(), scenarioUser, map[string]any{"plan": "pro"}))

	message := requireUserProfileUpdate(t, *requests, groupSetUrl, "$set")
	require.Equal(t, map[string]any{"plan": "pro"}, message["$set"])
}

func TestScenarioSetUserPropertyWithCombinedDistinctID(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleSet(context.Background(), []*PeopleProperties{NewPeopleProperties(scenarioCombinedDistinctID, map[string]any{"plan": "pro"})})
	require.NoError(t, err)

	message := requireUserProfileUpdate(t, *requests, groupSetUrl, "$set")
	require.Equal(t, map[string]any{"plan": "pro"}, message["$set"])
}

func TestScenarioSetUserPropertyWithCombinedDistinctIDIdentity(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleSetByIdentity(context.Background(), MixpanelIdentity{DistinctID: scenarioCombinedDistinctID}, map[string]any{"plan": "pro"})
	require.NoError(t, err)

	message := requireUserProfileUpdate(t, *requests, groupSetUrl, "$set")
	require.Equal(t, map[string]any{"plan": "pro"}, message["$set"])
}

func TestScenarioSetUserPropertyWithDistinctIDUnsupported(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleSet(context.Background(), []*PeopleProperties{NewPeopleProperties(scenarioUserID, map[string]any{"plan": "pro"})})

	require.ErrorIs(t, err, ErrInvalidIdentity)
	require.Empty(t, *requests)
}

func TestScenarioSetUserProperties(t *testing.T) {
	requests := captureRequests(t)
	set := map[string]any{"plan": "pro", "seats": float64(5)}

	require.NoError(t, hgClient().PeopleSetByIdentity(context.Background(), scenarioUser, set))

	message := requireUserProfileUpdate(t, *requests, groupSetUrl, "$set")
	require.Equal(t, set, message["$set"])
}

func TestScenarioSetOnceUserProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleSetOnceByIdentity(context.Background(), scenarioUser, map[string]any{"first_seen": "today"}))

	message := requireUserProfileUpdate(t, *requests, groupsSetOnceUrl, "$set_once")
	require.Equal(t, map[string]any{"first_seen": "today"}, message["$set_once"])
}

func TestScenarioUnionUserProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleUnionPropertyByIdentity(context.Background(), scenarioUser, map[string]any{"tags": []any{"a"}}))

	message := requireUserProfileUpdate(t, *requests, groupsUnionListPropertyUrl, "$union")
	require.Equal(t, map[string]any{"tags": []any{"a"}}, message["$union"])
}

func TestScenarioRemoveUserProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleRemoveListPropertyByIdentity(context.Background(), scenarioUser, map[string]any{"tags": "a"}))

	message := requireUserProfileUpdate(t, *requests, groupsRemoveFromListPropertyUrl, "$remove")
	require.Equal(t, map[string]any{"tags": "a"}, message["$remove"])
}

func TestScenarioUnsetUserProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleDeletePropertyByIdentity(context.Background(), scenarioUser, []string{"plan"}))

	message := requireUserProfileUpdate(t, *requests, groupsDeletePropertyUrl, "$unset")
	require.Equal(t, []any{"plan"}, message["$unset"])
}

func TestScenarioDeleteUser(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, hgClient().PeopleDeleteProfileByIdentity(context.Background(), scenarioUser))

	requireUserProfileUpdate(t, *requests, groupsDeleteGroupUrl, "$delete")
}

func TestScenarioIncrementUserPropertyUnsupported(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleIncrement(context.Background(), scenarioUserID, map[string]int{"logins": 1})

	require.ErrorIs(t, err, ErrUnsupportedHierarchicalOperation)
	require.Empty(t, *requests)
}

func TestScenarioAppendUserPropertyUnsupported(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleAppendListProperty(context.Background(), scenarioUserID, map[string]any{"tags": "a"})

	require.ErrorIs(t, err, ErrUnsupportedHierarchicalOperation)
	require.Empty(t, *requests)
}

// Company

func TestScenarioSetCompanyProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupSet(context.Background(), "$company_id", scenarioCompanyID, map[string]any{"plan": "enterprise"}))

	message := requireGroupProfileUpdate(t, *requests, groupSetUrl, "$company_id", scenarioCompanyID, "$set")
	require.Equal(t, map[string]any{"plan": "enterprise"}, message["$set"])
}

func TestScenarioSetCompanyProperties(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)
	set := map[string]any{"plan": "enterprise", "seats": float64(100)}

	require.NoError(t, company.GroupSet(context.Background(), "$company_id", scenarioCompanyID, set))

	message := requireGroupProfileUpdate(t, *requests, groupSetUrl, "$company_id", scenarioCompanyID, "$set")
	require.Equal(t, set, message["$set"])
}

func TestScenarioSetOnceCompanyProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupSetOnce(context.Background(), "$company_id", scenarioCompanyID, map[string]any{"founded": "2020"}))

	message := requireGroupProfileUpdate(t, *requests, groupsSetOnceUrl, "$company_id", scenarioCompanyID, "$set_once")
	require.Equal(t, map[string]any{"founded": "2020"}, message["$set_once"])
}

func TestScenarioUnionCompanyProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupUnionListProperty(context.Background(), "$company_id", scenarioCompanyID, map[string]any{"regions": []any{"eu"}}))

	message := requireGroupProfileUpdate(t, *requests, groupsUnionListPropertyUrl, "$company_id", scenarioCompanyID, "$union")
	require.Equal(t, map[string]any{"regions": []any{"eu"}}, message["$union"])
}

func TestScenarioRemoveCompanyProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupRemoveListProperty(context.Background(), "$company_id", scenarioCompanyID, map[string]any{"regions": "eu"}))

	message := requireGroupProfileUpdate(t, *requests, groupsRemoveFromListPropertyUrl, "$company_id", scenarioCompanyID, "$remove")
	require.Equal(t, map[string]any{"regions": "eu"}, message["$remove"])
}

func TestScenarioUnsetCompanyProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupDeleteProperty(context.Background(), "$company_id", scenarioCompanyID, []string{"plan"}))

	message := requireGroupProfileUpdate(t, *requests, groupsDeletePropertyUrl, "$company_id", scenarioCompanyID, "$unset")
	require.Equal(t, []any{"plan"}, message["$unset"])
}

// Custom groups

func TestScenarioSetUsersCustomGroup(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.PeopleSetByIdentity(context.Background(), scenarioUser, map[string]any{"team_id": scenarioTeamID}))

	message := requireUserProfileUpdate(t, *requests, groupSetUrl, "$set")
	require.Equal(t, map[string]any{"team_id": scenarioTeamID}, message["$set"])
}

func TestScenarioRemoveUsersCustomGroup(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.PeopleDeletePropertyByIdentity(context.Background(), scenarioUser, []string{"team_id"}))

	message := requireUserProfileUpdate(t, *requests, groupsDeletePropertyUrl, "$unset")
	require.Equal(t, []any{"team_id"}, message["$unset"])
}

func TestScenarioSetCustomGroupProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupSet(context.Background(), "team_id", scenarioTeamID, map[string]any{"name": "Platform"}))

	message := requireGroupProfileUpdate(t, *requests, groupSetUrl, "team_id", scenarioTeamID, "$set")
	require.Equal(t, map[string]any{"name": "Platform"}, message["$set"])
}

func TestScenarioSetCustomGroupProperties(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)
	set := map[string]any{"name": "Platform", "size": float64(8)}

	require.NoError(t, company.GroupSet(context.Background(), "team_id", scenarioTeamID, set))

	message := requireGroupProfileUpdate(t, *requests, groupSetUrl, "team_id", scenarioTeamID, "$set")
	require.Equal(t, set, message["$set"])
}

func TestScenarioSetOnceCustomGroupProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupSetOnce(context.Background(), "team_id", scenarioTeamID, map[string]any{"created": "2024"}))

	message := requireGroupProfileUpdate(t, *requests, groupsSetOnceUrl, "team_id", scenarioTeamID, "$set_once")
	require.Equal(t, map[string]any{"created": "2024"}, message["$set_once"])
}

func TestScenarioUnionCustomGroupProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupUnionListProperty(context.Background(), "team_id", scenarioTeamID, map[string]any{"tools": []any{"go"}}))

	message := requireGroupProfileUpdate(t, *requests, groupsUnionListPropertyUrl, "team_id", scenarioTeamID, "$union")
	require.Equal(t, map[string]any{"tools": []any{"go"}}, message["$union"])
}

func TestScenarioRemoveCustomGroupProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupRemoveListProperty(context.Background(), "team_id", scenarioTeamID, map[string]any{"tools": "go"}))

	message := requireGroupProfileUpdate(t, *requests, groupsRemoveFromListPropertyUrl, "team_id", scenarioTeamID, "$remove")
	require.Equal(t, map[string]any{"tools": "go"}, message["$remove"])
}

func TestScenarioUnsetCustomGroupProperty(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupDeleteProperty(context.Background(), "team_id", scenarioTeamID, []string{"name"}))

	message := requireGroupProfileUpdate(t, *requests, groupsDeletePropertyUrl, "team_id", scenarioTeamID, "$unset")
	require.Equal(t, []any{"name"}, message["$unset"])
}

func TestScenarioDeleteCompany(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupDelete(context.Background(), "$company_id", scenarioCompanyID))

	requireGroupProfileUpdate(t, *requests, groupsDeleteGroupUrl, "$company_id", scenarioCompanyID, "$delete")
}

func TestScenarioDeleteCustomGroup(t *testing.T) {
	requests := captureRequests(t)
	company := mustForCompany(t, hgClient(), scenarioCompanyID)

	require.NoError(t, company.GroupDelete(context.Background(), "team_id", scenarioTeamID))

	requireGroupProfileUpdate(t, *requests, groupsDeleteGroupUrl, "team_id", scenarioTeamID, "$delete")
}

// Unsupported or rejected

func TestScenarioIdentifyWithoutCompany(t *testing.T) {
	requests := captureRequests(t)
	mp := hgClient()
	userOnly := MixpanelIdentity{UserID: scenarioUserID}

	_, err := mp.NewIdentityEvent("Scenario Event", userOnly, nil)
	require.ErrorIs(t, err, ErrInvalidIdentity)

	err = mp.PeopleSetByIdentity(context.Background(), userOnly, map[string]any{"plan": "pro"})
	require.ErrorIs(t, err, ErrInvalidIdentity)

	require.Empty(t, *requests)
}

func TestScenarioSetUserPropertyWithoutUser(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().PeopleSetByIdentity(context.Background(), MixpanelIdentity{CompanyID: scenarioCompanyID}, map[string]any{"plan": "pro"})

	require.ErrorIs(t, err, ErrInvalidIdentity)
	require.Empty(t, *requests)
}

func TestScenarioSetCompanyPropertyWithoutCompany(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().GroupSet(context.Background(), "$company_id", scenarioCompanyID, map[string]any{"plan": "enterprise"})

	require.ErrorIs(t, err, ErrCompanyRequired)
	require.Empty(t, *requests)
}

func TestScenarioSetCustomGroupPropertyWithoutCompany(t *testing.T) {
	requests := captureRequests(t)

	err := hgClient().GroupSet(context.Background(), "team_id", scenarioTeamID, map[string]any{"name": "Platform"})

	require.ErrorIs(t, err, ErrCompanyRequired)
	require.Empty(t, *requests)
}

func TestScenarioCompanyIdentityWithoutHierarchicalGroups(t *testing.T) {
	requests := captureRequests(t)
	mp := legacyClient()

	_, err := mp.NewIdentityEvent("Scenario Event", scenarioUser, nil)
	require.ErrorIs(t, err, ErrInvalidIdentity)

	err = mp.PeopleSetByIdentity(context.Background(), scenarioUser, map[string]any{"plan": "pro"})
	require.ErrorIs(t, err, ErrInvalidIdentity)

	require.Empty(t, *requests)
}

func TestScenarioAliasUnsupported(t *testing.T) {
	// A user is identified by company and user id, so there is nothing to alias.
	requests := captureRequests(t)

	err := hgClient().Alias(context.Background(), "alias_id", scenarioUserID)

	require.ErrorIs(t, err, ErrUnsupportedHierarchicalOperation)
	require.Empty(t, *requests)
}

func TestScenarioMergeUnsupported(t *testing.T) {
	// A user is identified by company and user id, so there is nothing to merge.
	requests := captureRequests(t)

	err := hgClient().Merge(context.Background(), scenarioUserID, scenarioDeviceID)

	require.ErrorIs(t, err, ErrUnsupportedHierarchicalOperation)
	require.Empty(t, *requests)
}

// Without hierarchical groups

func TestScenarioTrackWithDistinctID(t *testing.T) {
	requests := captureRequests(t)
	mp := legacyClient()

	require.NoError(t, mp.Track(context.Background(), []*Event{mp.NewEvent("Scenario Event", scenarioUserID, nil)}))

	u, props := onlyEvent(t, *requests)
	require.Equal(t, trackURL, u)
	requireKeys(t, props, append(eventKeys, "distinct_id")...)
	require.Equal(t, scenarioUserID, props["distinct_id"])
}

func TestScenarioSetUserPropertyWithDistinctID(t *testing.T) {
	requests := captureRequests(t)

	err := legacyClient().PeopleSet(context.Background(), []*PeopleProperties{NewPeopleProperties(scenarioUserID, map[string]any{"plan": "pro"})})
	require.NoError(t, err)

	u, message := onlyMessage(t, *requests)
	require.Equal(t, peopleSetURL, u)
	requireKeys(t, message, "$token", "$distinct_id", "$set", "$ip")
	require.Equal(t, scenarioUserID, message["$distinct_id"])
	require.Equal(t, map[string]any{"plan": "pro"}, message["$set"])
}

func TestScenarioSetGroupProperty(t *testing.T) {
	requests := captureRequests(t)

	require.NoError(t, legacyClient().GroupSet(context.Background(), "team_id", scenarioTeamID, map[string]any{"name": "Platform"}))

	u, message := onlyMessage(t, *requests)
	require.Equal(t, groupSetUrl, u)
	requireKeys(t, message, "$token", "$group_key", "$group_id", "$set")
	require.Equal(t, "team_id", message["$group_key"])
	require.Equal(t, scenarioTeamID, message["$group_id"])
}
