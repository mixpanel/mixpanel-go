package mixpanel

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mixpanel/mixpanel-go/v2/flags"
)

const (
	propertyUserID    = "$user_id"
	propertyCompanyID = "$company_id"
	propertyDeviceID  = "$device_id"

	// userIDGroupKey is the group key of a user profile on a hierarchical groups project.
	userIDGroupKey = "$user_id"

	// The distinct id Mixpanel builds for a hierarchical groups user is
	// combinedCompanyPrefix + company + combinedUserSeparator + user.
	combinedCompanyPrefix = "$company:"
	combinedUserSeparator = "|$user:"
)

// groupOperations are the profile operations /groups supports. It has no $add, $append or $merge.
var groupOperations = []string{"$set", "$set_once", "$unset", "$union", "$remove", "$delete"}

var (
	// ErrInvalidIdentity is returned when an identity, or a company id, cannot be used for the
	// requested call: it is empty, it does not match how the client was configured, or it does
	// not name a profile.
	ErrInvalidIdentity = errors.New("invalid identity")

	// ErrUnsupportedHierarchicalOperation is returned on a hierarchical groups client for people
	// operations that have no /groups equivalent, such as PeopleIncrement and PeopleAppendListProperty,
	// and for Alias and Merge.
	ErrUnsupportedHierarchicalOperation = errors.New("operation not supported for hierarchical groups")

	// ErrCompanyRequired is returned on a hierarchical groups client for a group profile update
	// whose company is unknown. Bind one with ForCompany.
	ErrCompanyRequired = errors.New("company required for hierarchical groups")
)

// MixpanelIdentity identifies who an event or profile update belongs to.
//
// Mixpanel resolves an identity from whichever of distinct_id, $user_id, $company_id and
// $device_id an event carries, so any combination may be set. An empty string leaves a value
// unset, and unset values are never sent. At least one value is required; this is checked when
// the identity is used.
//
//	mixpanel.MixpanelIdentity{DistinctID: "user_123"}                     // a plain distinct id
//	mixpanel.MixpanelIdentity{UserID: "user_123", DeviceID: "device_abc"} // simplified id merge
//	mixpanel.MixpanelIdentity{UserID: "user_123", CompanyID: "acme"}      // hierarchical groups
//	mixpanel.MixpanelIdentity{DeviceID: "device_abc"}                     // anonymous, no user yet
//	mixpanel.MixpanelIdentity{CompanyID: "acme"}                          // a company, no user
//
// Which combinations are valid depends on the project, so the client rather than this type decides.
// On a hierarchical groups project the company is part of the user's identity: a UserID sent
// without a CompanyID resolves to a different user that nothing reconciles later, so include the
// company whenever the user has one. Events still send a user id without one as given; user profile
// updates need the company. A user who belongs to two companies is two users, by design.
//
// It is named MixpanelIdentity because this package already exports an Identity interface
// (Alias and Merge).
type MixpanelIdentity struct {
	// DistinctID is a distinct id Mixpanel resolves directly. On a hierarchical groups project
	// Mixpanel builds the distinct id from the user and company, so a raw distinct id names a
	// different, unrelated user that cannot be merged afterwards. Set one only when Mixpanel built
	// it, of the form $company:{company}|$user:{user}.
	DistinctID string
	// UserID is the user's id. On a hierarchical groups project it only needs to be unique within
	// the company.
	UserID string
	// CompanyID is the id of the top-level company, on a hierarchical groups project.
	CompanyID string
	// DeviceID is the client's device id. Events carrying one are attributed to the device until a
	// later event names the user, at which point that earlier activity follows the user.
	DeviceID string
}

// IsHierarchical reports whether the identity names a company, which is what sends it down
// Mixpanel's hierarchical groups resolution.
func (i MixpanelIdentity) IsHierarchical() bool {
	return i.CompanyID != ""
}

func (i MixpanelIdentity) isEmpty() bool {
	return i.DistinctID == "" && i.UserID == "" && i.CompanyID == "" && i.DeviceID == ""
}

// String lists the identity's set values, for error messages.
func (i MixpanelIdentity) String() string {
	var parts []string
	for _, f := range []struct{ name, value string }{
		{"DistinctID", i.DistinctID},
		{"UserID", i.UserID},
		{"CompanyID", i.CompanyID},
		{"DeviceID", i.DeviceID},
	} {
		if f.value != "" {
			parts = append(parts, f.name+"="+f.value)
		}
	}
	return "MixpanelIdentity{" + strings.Join(parts, ", ") + "}"
}

// FlagContext builds a feature flag evaluation context for the identity, so callers do not
// hand-write reserved keys such as $company_id. The ids go under the keys flags read, groups such
// as {"department": "finance"} at the top level, and customProperties, used by runtime targeting
// rules, under custom_properties. Either map may be nil.
//
//	fc := identity.FlagContext(nil, map[string]any{"plan": "enterprise"})
func (i MixpanelIdentity) FlagContext(groups, customProperties map[string]any) flags.FlagContext {
	fc := flags.FlagContext{}
	for key, value := range groups {
		fc[key] = value
	}
	for _, f := range []struct{ key, value string }{
		{"distinct_id", i.DistinctID},
		{propertyUserID, i.UserID},
		{propertyCompanyID, i.CompanyID},
		{"device_id", i.DeviceID},
	} {
		if f.value != "" {
			fc[f.key] = f.value
		}
	}
	if len(customProperties) > 0 {
		custom := make(map[string]any, len(customProperties))
		for key, value := range customProperties {
			custom[key] = value
		}
		fc["custom_properties"] = custom
	}
	return fc
}

// applyTo writes the identity's keys into properties, replacing caller-supplied values for the same keys.
func (i MixpanelIdentity) applyTo(properties map[string]any) {
	if i.DistinctID != "" {
		properties[propertyDistinctID] = i.DistinctID
	}
	if i.UserID != "" {
		properties[propertyUserID] = i.UserID
	}
	if i.CompanyID != "" {
		properties[propertyCompanyID] = i.CompanyID
	}
	if i.DeviceID != "" {
		properties[propertyDeviceID] = i.DeviceID
	}
}

// HierarchicalGroups marks the client as belonging to a project that uses hierarchical groups.
//
// Mixpanel then builds the distinct id from $user_id and $company_id, so prefer NewIdentityEvent
// with a MixpanelIdentity, which sends those properties. NewEvent still works, and Mixpanel
// accepts a distinct id it built itself, but a distinct id that is not in that form names a
// different user that nothing reconciles later.
//
// User profiles are group profiles keyed by $user_id, so they support only what /groups supports,
// and no request goes to /engage. PeopleIncrement and PeopleAppendListProperty return
// ErrUnsupportedHierarchicalOperation. The other distinct id People methods send to /groups when
// the distinct id has the form $company:{company}|$user:{user}, and return ErrInvalidIdentity for
// any other distinct id. The ByIdentity people methods address a profile without needing a
// distinct id. Group profile updates require a company: bind one with ForCompany, or they return
// ErrCompanyRequired.
//
// Events are sent unchanged.
//
// Feature flags providers configured on the client, before or after this option, attribute
// exposure events by the context's $company_id and $user_id as well as by distinct_id.
func HierarchicalGroups() Options {
	return func(mixpanel *ApiClient) {
		mixpanel.hierarchicalGroups = true
		// Providers configured by an earlier option were built without hierarchical groups.
		if mixpanel.LocalFlags != nil {
			mixpanel.buildLocalFlags()
		}
		if mixpanel.RemoteFlags != nil {
			mixpanel.buildRemoteFlags()
		}
	}
}

// ForCompany returns a copy of the client that adds a top-level $company_id to every group
// profile update it sends. Hierarchical groups projects require $company_id on every /groups
// write, including the company's own profile and user profiles. The Company methods update the
// bound company's own profile:
//
//	acme, err := mp.ForCompany("acme")
//	acme.CompanySet(ctx, companyProperties)
//	acme.GroupSet(ctx, "department", "finance", departmentProperties)
//
// The copy shares the receiver's http client, credentials, endpoints and feature flag providers;
// the receiver is unchanged. The ByIdentity people methods use the identity's own company and do
// not need a bound company; on a bound client they return ErrInvalidIdentity for a user from
// another company.
func (a *ApiClient) ForCompany(companyID string) (*ApiClient, error) {
	if companyID == "" {
		return nil, fmt.Errorf("%w: ForCompany needs a non-empty company id", ErrInvalidIdentity)
	}
	bound := *a
	bound.companyID = companyID
	return &bound, nil
}

// CompanySet sets properties on the profile of the company bound with ForCompany. It sends the
// same request as GroupSet(ctx, "$company_id", companyID, set) on that client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanySet(ctx context.Context, set map[string]any) error {
	companyID, err := a.boundCompanyID("CompanySet")
	if err != nil {
		return err
	}
	return a.GroupSet(ctx, propertyCompanyID, companyID, set)
}

// CompanySetOnce sets properties on the profile of the company bound with ForCompany only if they
// are not already set. It sends the same request as GroupSetOnce(ctx, "$company_id", companyID, set)
// on that client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanySetOnce(ctx context.Context, set map[string]any) error {
	companyID, err := a.boundCompanyID("CompanySetOnce")
	if err != nil {
		return err
	}
	return a.GroupSetOnce(ctx, propertyCompanyID, companyID, set)
}

// CompanyDeleteProperty removes properties from the profile of the company bound with ForCompany.
// It sends the same request as GroupDeleteProperty(ctx, "$company_id", companyID, unset) on that
// client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanyDeleteProperty(ctx context.Context, unset []string) error {
	companyID, err := a.boundCompanyID("CompanyDeleteProperty")
	if err != nil {
		return err
	}
	return a.GroupDeleteProperty(ctx, propertyCompanyID, companyID, unset)
}

// CompanyRemoveListProperty removes values from list properties on the profile of the company
// bound with ForCompany. It sends the same request as
// GroupRemoveListProperty(ctx, "$company_id", companyID, remove) on that client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanyRemoveListProperty(ctx context.Context, remove map[string]any) error {
	companyID, err := a.boundCompanyID("CompanyRemoveListProperty")
	if err != nil {
		return err
	}
	return a.GroupRemoveListProperty(ctx, propertyCompanyID, companyID, remove)
}

// CompanyUnionListProperty merges values into list properties on the profile of the company bound
// with ForCompany, ignoring values already present. It sends the same request as
// GroupUnionListProperty(ctx, "$company_id", companyID, union) on that client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanyUnionListProperty(ctx context.Context, union map[string]any) error {
	companyID, err := a.boundCompanyID("CompanyUnionListProperty")
	if err != nil {
		return err
	}
	return a.GroupUnionListProperty(ctx, propertyCompanyID, companyID, union)
}

// CompanyDelete deletes the profile of the company bound with ForCompany. It sends the same
// request as GroupDelete(ctx, "$company_id", companyID) on that client.
//
// On a client with no bound company it returns ErrCompanyRequired.
func (a *ApiClient) CompanyDelete(ctx context.Context) error {
	companyID, err := a.boundCompanyID("CompanyDelete")
	if err != nil {
		return err
	}
	return a.GroupDelete(ctx, propertyCompanyID, companyID)
}

// NewIdentityEvent creates a new mixpanel event for the given identity, for Track or Import.
//
// The identity's keys replace any values for the same keys in properties; other properties are
// kept, and properties itself is not modified. An identity without a DistinctID produces an event
// with no distinct_id; on a hierarchical groups project Mixpanel builds it from $user_id and
// $company_id.
//
// It returns ErrInvalidIdentity when the identity is empty or names a company on a client created
// without HierarchicalGroups.
func (a *ApiClient) NewIdentityEvent(name string, identity MixpanelIdentity, properties map[string]any) (*Event, error) {
	if err := a.requireIdentityMatchesProject(identity); err != nil {
		return nil, err
	}

	merged := make(map[string]any, len(properties)+6)
	for k, v := range properties {
		merged[k] = v
	}
	identity.applyTo(merged)
	merged[propertyToken] = a.token
	merged[propertyMpLib] = goLib
	merged[propertyLibVersion] = version

	return &Event{Name: name, Properties: merged}, nil
}

// PeopleSetByIdentity sets properties on the user profile for the given identity.
//
// An identity with a DistinctID is the same as PeopleSet for that distinct id, so on a hierarchical
// groups client the DistinctID must have the form $company:{company}|$user:{user}. On a
// hierarchical groups client an identity with a UserID and CompanyID updates the $user_id group
// profile within that company through /groups. Any other identity cannot name a profile and returns
// ErrInvalidIdentity; a hierarchical user without a company returns ErrCompanyRequired.
//
// Hierarchical groups projects support set, set once, unset, union, remove and delete, so
// there is no ByIdentity variant of PeopleIncrement or PeopleAppendListProperty.
func (a *ApiClient) PeopleSetByIdentity(ctx context.Context, identity MixpanelIdentity, set map[string]any) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleSet(ctx, []*PeopleProperties{NewPeopleProperties(identity.DistinctID, set)})
	}
	return a.groupSet(ctx, userIDGroupKey, userID, set, companyID)
}

// PeopleSetOnceByIdentity sets properties on the user profile for the given identity only if they
// are not already set. See PeopleSetByIdentity for how the identity is resolved.
func (a *ApiClient) PeopleSetOnceByIdentity(ctx context.Context, identity MixpanelIdentity, setOnce map[string]any) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleSetOnce(ctx, []*PeopleProperties{NewPeopleProperties(identity.DistinctID, setOnce)})
	}
	return a.groupSetOnce(ctx, userIDGroupKey, userID, setOnce, companyID)
}

// PeopleUnionPropertyByIdentity merges values into list properties on the user profile for the
// given identity, ignoring values already present. See PeopleSetByIdentity for how the identity
// is resolved.
func (a *ApiClient) PeopleUnionPropertyByIdentity(ctx context.Context, identity MixpanelIdentity, union map[string]any) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleUnionProperty(ctx, identity.DistinctID, union)
	}
	return a.groupUnionListProperty(ctx, userIDGroupKey, userID, union, companyID)
}

// PeopleRemoveListPropertyByIdentity removes values from list properties on the user profile for
// the given identity. See PeopleSetByIdentity for how the identity is resolved.
func (a *ApiClient) PeopleRemoveListPropertyByIdentity(ctx context.Context, identity MixpanelIdentity, remove map[string]any) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleRemoveListProperty(ctx, identity.DistinctID, remove)
	}
	return a.groupRemoveListProperty(ctx, userIDGroupKey, userID, remove, companyID)
}

// PeopleDeletePropertyByIdentity removes properties from the user profile for the given identity.
// See PeopleSetByIdentity for how the identity is resolved.
func (a *ApiClient) PeopleDeletePropertyByIdentity(ctx context.Context, identity MixpanelIdentity, unset []string) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleDeleteProperty(ctx, identity.DistinctID, unset)
	}
	return a.groupDeleteProperty(ctx, userIDGroupKey, userID, unset, companyID)
}

// PeopleDeleteProfileByIdentity deletes the user profile for the given identity. An identity with
// a DistinctID is the same as PeopleDeleteProfile with ignoreAlias false. See PeopleSetByIdentity
// for how the identity is resolved.
func (a *ApiClient) PeopleDeleteProfileByIdentity(ctx context.Context, identity MixpanelIdentity) error {
	userID, companyID, err := a.resolveProfile(identity)
	if err != nil {
		return err
	}
	if identity.DistinctID != "" {
		return a.PeopleDeleteProfile(ctx, identity.DistinctID, false)
	}
	return a.groupDelete(ctx, userIDGroupKey, userID, companyID)
}

// requireIdentityMatchesProject checks the identity against the client's mode. Hierarchical groups
// is a project setting, and ingestion accepts the keys on any project but ignores them, so a
// mismatch would be silently mis-attributed.
func (a *ApiClient) requireIdentityMatchesProject(identity MixpanelIdentity) error {
	if identity.isEmpty() {
		return fmt.Errorf("%w: an identity needs at least one of DistinctID, UserID, CompanyID or DeviceID", ErrInvalidIdentity)
	}
	if identity.IsHierarchical() && !a.hierarchicalGroups {
		return fmt.Errorf("%w: %s requires a hierarchical groups client: mixpanel.NewApiClient(token, mixpanel.HierarchicalGroups())",
			ErrInvalidIdentity, identity)
	}
	return nil
}

// resolveProfile validates an identity for a user profile update. When the identity has a
// DistinctID the caller sends the update through the distinct id People method, which resolves it
// for the client's mode; otherwise it returns the user and company of the $user_id group profile.
func (a *ApiClient) resolveProfile(identity MixpanelIdentity) (userID, companyID string, err error) {
	if err := a.requireIdentityMatchesProject(identity); err != nil {
		return "", "", err
	}
	if identity.DistinctID != "" {
		return "", "", nil
	}
	if !a.hierarchicalGroups {
		return "", "", fmt.Errorf("%w: %s cannot identify a profile on this project. User profiles are addressed "+
			"by distinct id unless the project uses hierarchical groups; set DistinctID", ErrInvalidIdentity, identity)
	}
	if identity.UserID == "" {
		if identity.CompanyID == "" {
			return "", "", fmt.Errorf("%w: %s names no user, so it cannot identify a profile. "+
				"Device ids identify anonymous events, not profiles", ErrInvalidIdentity, identity)
		}
		return "", "", fmt.Errorf("%w: user profile updates require a UserID, and %s has none. "+
			"Use CompanySet(ctx, properties) on ForCompany(companyID) to update the company's profile",
			ErrInvalidIdentity, identity)
	}
	if identity.CompanyID == "" {
		// Events accept a user without a company, but a user profile is a group profile within one.
		return "", "", fmt.Errorf("%w: user profile updates on a hierarchical groups project need the identity's "+
			"CompanyID, and %s has none", ErrCompanyRequired, identity)
	}
	if err := a.requireBoundCompany(identity.CompanyID, identity.String()); err != nil {
		return "", "", err
	}
	return identity.UserID, identity.CompanyID, nil
}

// requireBoundCompany refuses a user from another company on a client bound with ForCompany,
// rather than silently updating that company's profile.
func (a *ApiClient) requireBoundCompany(companyID, described string) error {
	if a.companyID == "" || companyID == a.companyID {
		return nil
	}
	return fmt.Errorf("%w: this client is bound to company %q, but %s belongs to company %q. Update the user "+
		"through a client for that company, or one without ForCompany. Nothing sent",
		ErrInvalidIdentity, a.companyID, described, companyID)
}

// parseCombinedDistinctID splits a distinct id of the form $company:{company}|$user:{user} into its
// user and company. The company ends at the first |$user:. ok is false when the distinct id does
// not have that form or either part is empty.
func parseCombinedDistinctID(distinctID string) (userID, companyID string, ok bool) {
	if !strings.HasPrefix(distinctID, combinedCompanyPrefix) {
		return "", "", false
	}
	companyID, userID, found := strings.Cut(strings.TrimPrefix(distinctID, combinedCompanyPrefix), combinedUserSeparator)
	if !found || companyID == "" || userID == "" {
		return "", "", false
	}
	return userID, companyID, true
}

// hierarchicalProfile returns the user and company of the $user_id group profile that a distinct
// id names on a hierarchical groups client. User profiles on these projects are /groups profiles,
// so only a distinct id of the form $company:{company}|$user:{user} can name one.
func (a *ApiClient) hierarchicalProfile(distinctID string) (userID, companyID string, err error) {
	userID, companyID, ok := parseCombinedDistinctID(distinctID)
	if !ok {
		return "", "", fmt.Errorf("%w: user profile updates on hierarchical groups projects need a user and a company, "+
			"and distinct id %q does not name both. Pass MixpanelIdentity{UserID: userID, CompanyID: companyID} to a "+
			"ByIdentity method, or a distinct id of the form $company:{company}|$user:{user}. Nothing sent",
			ErrInvalidIdentity, distinctID)
	}
	if err := a.requireBoundCompany(companyID, strconv.Quote(distinctID)); err != nil {
		return "", "", err
	}
	return userID, companyID, nil
}

// groupCompanyID returns the company to send with a group profile update, or ErrCompanyRequired
// on a hierarchical groups client with no company bound.
func (a *ApiClient) groupCompanyID(groupKey, groupID string) (string, error) {
	if a.hierarchicalGroups && a.companyID == "" {
		return "", fmt.Errorf("%w: group profile updates for hierarchical groups projects require a company. "+
			"Use ForCompany(companyID) to send them. Nothing sent for group %s=%s", ErrCompanyRequired, groupKey, groupID)
	}
	return a.companyID, nil
}

// boundCompanyID returns the company bound with ForCompany, or ErrCompanyRequired when there is
// none, whether or not the client uses hierarchical groups.
func (a *ApiClient) boundCompanyID(method string) (string, error) {
	if a.companyID == "" {
		return "", fmt.Errorf("%w: %s updates the profile of the company bound with ForCompany, and this client "+
			"has none. Use ForCompany(companyID).%s. Nothing sent", ErrCompanyRequired, method, method)
	}
	return a.companyID, nil
}

// requireGroupOperation refuses, on a hierarchical groups client, a people operation that /groups
// does not support. A hierarchical groups user profile is a group profile, so $add and $append
// have no valid path at all.
func (a *ApiClient) requireGroupOperation(operation string) error {
	if !a.hierarchicalGroups {
		return nil
	}
	for _, supported := range groupOperations {
		if operation == supported {
			return nil
		}
	}
	return fmt.Errorf("%w: %s is not supported for hierarchical groups projects, whose user profiles are group "+
		"profiles keyed by $user_id. Supported operations are [%s], so PeopleIncrement and PeopleAppendListProperty "+
		"have no hierarchical groups equivalent", ErrUnsupportedHierarchicalOperation, operation, strings.Join(groupOperations, ", "))
}
