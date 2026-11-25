// Package combinedid builds and parses the distinct id Mixpanel builds for a hierarchical groups
// user: Prefix + company + UserSeparator + user, as in $company:{company}|$user:{user}.
package combinedid

import "strings"

const (
	// Prefix starts a combined distinct id, before the company.
	Prefix = "$company:"
	// UserSeparator separates the company from the user in a combined distinct id.
	UserSeparator = "|$user:"
)

// Build returns the combined distinct id of a user within a company.
func Build(companyID, userID string) string {
	return Prefix + companyID + UserSeparator + userID
}

// Parse splits a distinct id of the form $company:{company}|$user:{user} into its company and
// user. The company ends at the first |$user:. ok is false when the distinct id does not have
// that form. Parts are returned as they are, even when empty.
func Parse(distinctID string) (companyID, userID string, ok bool) {
	if !strings.HasPrefix(distinctID, Prefix) {
		return "", "", false
	}
	return strings.Cut(strings.TrimPrefix(distinctID, Prefix), UserSeparator)
}
