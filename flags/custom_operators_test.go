package flags

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/diegoholiveira/jsonlogic/v3"
	"github.com/stretchr/testify/require"
)

// evalCase drives one rule/data pair through the JsonLogic engine and asserts the membership result.
type evalCase struct {
	name string
	rule map[string]any
	data map[string]any
	want bool
}

// evalRule marshals rule and data to JSON before evaluating, matching how runtime rules reach the
// engine in production (all numbers arrive as float64).
func evalRule(t *testing.T, rule, data map[string]any) bool {
	t.Helper()
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)
	jsonRule, err := json.Marshal(rule)
	require.NoError(t, err)

	var result bytes.Buffer
	err = jsonlogic.Apply(strings.NewReader(string(jsonRule)), strings.NewReader(string(jsonData)), &result)
	require.NoError(t, err)

	got, err := strconv.ParseBool(strings.TrimSpace(result.String()))
	require.NoError(t, err)
	return got
}

func runCases(t *testing.T, cases []evalCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, evalRule(t, tc.rule, tc.data))
		})
	}
}

func varNode(key string) map[string]any { return map[string]any{"var": key} }

func semverRule(key, sym, target string) map[string]any {
	return map[string]any{"semver_compare": []any{varNode(key), sym, target}}
}

// datetime targets are epoch milliseconds (numbers), matching what the feature-flags UI emits.
func datetimeRule(key, sym string, target float64) map[string]any {
	return map[string]any{"datetime_compare": []any{varNode(key), sym, target}}
}

// customBetween mirrors how the UI emits a semver Between: {"and": [{op:[v,">=",lo]}, {op:[v,"<=",hi]}]}.
func customBetween(op, key, lo, hi string) map[string]any {
	return map[string]any{"and": []any{
		map[string]any{op: []any{varNode(key), ">=", lo}},
		map[string]any{op: []any{varNode(key), "<=", hi}},
	}}
}

// datetimeBetween mirrors the UI's datetime Between with epoch-millisecond bounds.
func datetimeBetween(key string, lo, hi float64) map[string]any {
	return map[string]any{"and": []any{
		map[string]any{"datetime_compare": []any{varNode(key), ">=", lo}},
		map[string]any{"datetime_compare": []any{varNode(key), "<=", hi}},
	}}
}

func TestSemverCompareOperator(t *testing.T) {
	runCases(t, []evalCase{
		{"is, equal", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3"}, true},
		{"is, not equal", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.4"}, false},
		{"is not", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.4"}, true},
		{"less than, patch", semverRule("app_version", "<", "1.2.3"), map[string]any{"app_version": "1.2.2"}, true},
		{"less than, false", semverRule("app_version", "<", "1.2.3"), map[string]any{"app_version": "1.2.3"}, false},
		{"less or equal, boundary", semverRule("app_version", "<=", "1.2.3"), map[string]any{"app_version": "1.2.3"}, true},
		{"greater than, minor", semverRule("app_version", ">", "1.2.3"), map[string]any{"app_version": "1.3.0"}, true},
		{"greater or equal, boundary", semverRule("app_version", ">=", "1.2.3"), map[string]any{"app_version": "1.2.3"}, true},
		{"double-digit ordering (not lexical)", semverRule("app_version", ">", "1.9.0"), map[string]any{"app_version": "1.10.0"}, true},
		{"prerelease precedes release", semverRule("app_version", "<", "1.0.0"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"lenient v-prefix", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "v1.2.3"}, true},
		{"lenient minor-only target", semverRule("app_version", "=", "1.2"), map[string]any{"app_version": "1.2.0"}, true},
		// Every symbol is asserted in both directions.
		{"is not, equal", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3"}, false},
		{"less or equal, above", semverRule("app_version", "<=", "1.2.3"), map[string]any{"app_version": "1.2.4"}, false},
		{"greater than, below", semverRule("app_version", ">", "1.2.3"), map[string]any{"app_version": "1.2.2"}, false},
		{"greater or equal, below", semverRule("app_version", ">=", "1.2.3"), map[string]any{"app_version": "1.2.2"}, false},
		// Prerelease precedence, SemVer 2.0.0 section 11.
		{"prerelease alpha before beta", semverRule("app_version", "<", "1.0.0-beta"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"prerelease beta before rc1", semverRule("app_version", "<", "1.0.0-rc1"), map[string]any{"app_version": "1.0.0-beta"}, true},
		{"prerelease rc1 before rc2", semverRule("app_version", "<", "1.0.0-rc2"), map[string]any{"app_version": "1.0.0-rc1"}, true},
		{"more prerelease fields wins", semverRule("app_version", "<", "1.0.0-alpha.1"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"numeric identifier below alphanumeric", semverRule("app_version", "<", "1.0.0-alpha.beta"), map[string]any{"app_version": "1.0.0-alpha.1"}, true},
		{"fewer fields below alphanumeric", semverRule("app_version", "<", "1.0.0-alpha.beta"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"numeric identifiers compare numerically", semverRule("app_version", "<", "1.0.0-beta.11"), map[string]any{"app_version": "1.0.0-beta.2"}, true},
		{"dotted identifier ordering, letters", semverRule("app_version", "<", "1.0.0-b.1"), map[string]any{"app_version": "1.0.0-a.1"}, true},
		{"dotted identifier ordering, digits", semverRule("app_version", "<", "1.0.0-a.2"), map[string]any{"app_version": "1.0.0-a.1"}, true},
		{"identical prereleases are equal", semverRule("app_version", "=", "1.0.0-rc1"), map[string]any{"app_version": "1.0.0-rc1"}, true},
		{"rc1 outranks dotted rc.1", semverRule("app_version", ">", "1.0.0-rc.1"), map[string]any{"app_version": "1.0.0-rc1"}, true},
		{"core version dominates prerelease", semverRule("app_version", ">", "1.9.9"), map[string]any{"app_version": "2.0.0-alpha"}, true},
		// A release outranks its own prerelease, asserted from both sides and under every symbol.
		{"release outranks its prerelease", semverRule("app_version", ">", "1.0.0-alpha"), map[string]any{"app_version": "1.0.0"}, true},
		{"release at or above its prerelease", semverRule("app_version", ">=", "1.0.0-rc1"), map[string]any{"app_version": "1.0.0"}, true},
		{"release differs from its prerelease", semverRule("app_version", "!=", "1.0.0-alpha"), map[string]any{"app_version": "1.0.0"}, true},
		{"prerelease differs from its release", semverRule("app_version", "!=", "1.0.0"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"prerelease at or below its release", semverRule("app_version", "<=", "1.0.0"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"prerelease of a higher core still wins", semverRule("app_version", ">", "0.9.9"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"prerelease below the next patch", semverRule("app_version", "<", "1.0.1"), map[string]any{"app_version": "1.0.0-rc1"}, true},
		// Prerelease identifier comparison, SemVer 2.0.0 section 11.4.
		{"numeric identifiers are not compared lexically", semverRule("app_version", "<", "1.0.0-10"), map[string]any{"app_version": "1.0.0-2"}, true},
		{"numeric identifier ranks below alphanumeric", semverRule("app_version", "<", "1.0.0-alpha"), map[string]any{"app_version": "1.0.0-1"}, true},
		{"hyphen inside an identifier sorts by ascii", semverRule("app_version", "<", "1.0.0-alpha-1"), map[string]any{"app_version": "1.0.0-alpha"}, true},
		{"beta ranks below rc", semverRule("app_version", "<", "1.0.0-rc.1"), map[string]any{"app_version": "1.0.0-beta.11"}, true},
		{"last prerelease ranks below the release", semverRule("app_version", "<", "1.0.0"), map[string]any{"app_version": "1.0.0-rc.1"}, true},
		// Build metadata carries no precedence.
		{"build metadata ignored", semverRule("app_version", "=", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, true},
		{"build metadata ignored with prerelease", semverRule("app_version", "=", "1.0.0-alpha"), map[string]any{"app_version": "1.0.0-alpha+build"}, true},
		{"build metadata with hyphen ignored", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3+build.1-2"}, true},
		// Ignored means equal, so every symbol has to agree with that.
		{"build metadata leaves versions equal", semverRule("app_version", "!=", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, false},
		{"build metadata is not less", semverRule("app_version", "<", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, false},
		{"build metadata is not greater", semverRule("app_version", ">", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, false},
		{"build metadata at or below", semverRule("app_version", "<=", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, true},
		{"build metadata at or above", semverRule("app_version", ">=", "1.0.0+build2"), map[string]any{"app_version": "1.0.0+build1"}, true},
		{"build metadata does not block ordering", semverRule("app_version", "<", "1.0.1+build1"), map[string]any{"app_version": "1.0.0+build9"}, true},
		{"build metadata does not block reverse ordering", semverRule("app_version", ">", "1.0.0+build9"), map[string]any{"app_version": "1.0.1+build1"}, true},
		// Partial versions keep their prerelease once zero-padded.
		{"partial version with prerelease", semverRule("app_version", "=", "1.2.0-alpha"), map[string]any{"app_version": "1.2-alpha"}, true},
		{"partial prerelease below later minor", semverRule("app_version", "<", "1.3.1"), map[string]any{"app_version": "1.2-alpha"}, true},
		{"partial prerelease below its release", semverRule("app_version", "<", "1.2.0"), map[string]any{"app_version": "1.2-alpha"}, true},
		{"major-only with prerelease", semverRule("app_version", "<", "1.0.0"), map[string]any{"app_version": "1-rc1"}, true},
		// An empty prerelease is invalid, so it is rejected rather than treated as the bare release.
		{"empty prerelease, no match", semverRule("app_version", "=", "1.0.0"), map[string]any{"app_version": "1.0.0-"}, false},
		{"empty prerelease, not-equal also false", semverRule("app_version", "!=", "1.0.0"), map[string]any{"app_version": "1.0.0-"}, false},
		{"empty prerelease on partial version, no match", semverRule("app_version", "=", "1.2.0"), map[string]any{"app_version": "1.2-"}, false},
		{"empty prerelease on partial version, not-equal also false", semverRule("app_version", "!=", "1.2.0"), map[string]any{"app_version": "1.2-"}, false},
		// Hyphens are legal inside a prerelease identifier, so these are NOT empty prereleases.
		{"trailing hyphen inside identifier", semverRule("app_version", "<", "1.0.0"), map[string]any{"app_version": "1.0.0-alpha-"}, true},
		// SemVer 2.0.0 forbids leading zeros in the core, so these are rejected rather than normalized.
		{"leading zero in major, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "01.2.3"}, false},
		{"leading zero in major, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "01.2.3"}, false},
		{"leading zero in minor, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.02.3"}, false},
		{"leading zero in minor, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.02.3"}, false},
		{"leading zero in patch, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.03"}, false},
		{"leading zero in patch, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.03"}, false},
		{"leading zeros throughout, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "01.02.03"}, false},
		{"leading zeros throughout, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "01.02.03"}, false},
		// A numeric prerelease identifier may not carry a leading zero either (section 9).
		{"numeric prerelease with leading zero, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3-01"}, false},
		{"numeric prerelease with leading zero, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3-01"}, false},
		{"dotted numeric prerelease with leading zero, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3-rc.01"}, false},
		{"dotted numeric prerelease with leading zero, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3-rc.01"}, false},
		// An alphanumeric identifier may contain digits, so this one stays valid.
		{"alphanumeric prerelease with digits", semverRule("app_version", "<", "1.2.3"), map[string]any{"app_version": "1.2.3-rc01"}, true},
		// The v-prefix is accepted in either case.
		{"lenient uppercase V-prefix", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "V1.2.3"}, true},
		{"v-prefix keeps prerelease", semverRule("app_version", "<", "1.0.0"), map[string]any{"app_version": "v1.0.0-alpha"}, true},
		{"v-prefix, not equal", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "v1.2.4"}, true},
		{"v-prefix, at or below", semverRule("app_version", "<=", "1.2.3"), map[string]any{"app_version": "v1.2.3"}, true},
		{"v-prefix, greater", semverRule("app_version", ">", "1.2.3"), map[string]any{"app_version": "v1.2.4"}, true},
		{"v-prefix, at or above", semverRule("app_version", ">=", "1.2.3"), map[string]any{"app_version": "v1.2.3"}, true},
		{"between, inside", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "1.5.0"}, true},
		{"between, low boundary inclusive", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "1.2.3"}, true},
		{"between, high boundary inclusive", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "2.0.0"}, true},
		{"between, below", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "1.0.0"}, false},
		{"between, above", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "2.0.1"}, false},
		// A prerelease sits below its own release, which decides both boundary cases.
		{"between, prerelease inside", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "1.5.0-rc1"}, true},
		{"between, prerelease below the high bound", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "2.0.0-rc1"}, true},
		{"between, prerelease of the low bound falls out", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "1.2.3-rc1"}, false},
		{"between, invalid version", customBetween("semver_compare", "app_version", "1.2.3", "2.0.0"), map[string]any{"app_version": "not-a-version"}, false},
		{"between, single-point range", customBetween("semver_compare", "app_version", "1.2.3", "1.2.3"), map[string]any{"app_version": "1.2.3"}, true},
		// Fail-closed: unparseable or missing values never match.
		{"invalid actual, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "not-a-version"}, false},
		{"non-string actual, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": 123}, false},
		{"missing property, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{}, false},
		// A malformed version must never be padded or coerced into a real one. Both symbols are
		// asserted so that "accepted at all" is observable rather than masked by a single false.
		{"empty version, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": ""}, false},
		{"empty version, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": ""}, false},
		{"bare v, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "v"}, false},
		{"bare v, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "v"}, false},
		{"leading separator, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "-1.2.3"}, false},
		{"leading separator, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "-1.2.3"}, false},
		{"trailing dot, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1."}, false},
		{"trailing dot, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1."}, false},
		{"trailing dot after patch, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3."}, false},
		{"trailing dot after patch, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3."}, false},
		{"empty middle segment, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1..2"}, false},
		{"empty middle segment, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1..2"}, false},
		{"four components, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3.4"}, false},
		{"four components, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3.4"}, false},
		{"range prefix, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "^1.2.3"}, false},
		{"range prefix, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "^1.2.3"}, false},
		{"version inside text, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "abc1.2.3"}, false},
		{"version inside text, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "abc1.2.3"}, false},
		{"empty build metadata, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3+"}, false},
		{"empty build metadata, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3+"}, false},
		{"empty prerelease identifier, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3-alpha..1"}, false},
		{"empty prerelease identifier, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3-alpha..1"}, false},
		{"lone dot prerelease, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3-."}, false},
		{"lone dot prerelease, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3-."}, false},
		{"underscore in prerelease, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "1.2.3-ALPHA_BETA"}, false},
		{"underscore in prerelease, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "1.2.3-ALPHA_BETA"}, false},
		{"doubled v-prefix, no match", semverRule("app_version", "=", "1.2.3"), map[string]any{"app_version": "vv1.2.3"}, false},
		{"doubled v-prefix, not-equal also false", semverRule("app_version", "!=", "1.2.3"), map[string]any{"app_version": "vv1.2.3"}, false},
	})
}

// Epoch-millisecond constants (UTC instants) used as datetime targets, matching the UI's emitted format.
const (
	jul16Ms        = 1_784_160_000_000 // 2026-07-16T00:00:00Z
	jan1Ms         = 1_767_225_600_000 // 2026-01-01T00:00:00Z
	dec31Ms        = 1_798_675_200_000 // 2026-12-31T00:00:00Z
	jul16EndMs     = 1_784_246_399_999 // 2026-07-16T23:59:59.999Z
	leapDayMs      = 1_709_164_800_000 // 2024-02-29T00:00:00Z
	jul16IndiaMs   = 1_784_140_200_000 // 2026-07-16T00:00:00+05:30
	jul16PacificMs = 1_784_188_800_000 // 2026-07-16T00:00:00-08:00
)

func TestDatetimeCompareOperator(t *testing.T) {
	runCases(t, []evalCase{
		// Asymmetric contract: subject (runtime var) is a strict RFC3339 string, target is epoch ms.
		{"before, true", datetimeRule("signup", "<", jul16Ms), map[string]any{"signup": "2026-07-15T00:00:00Z"}, true},
		{"before, false", datetimeRule("signup", "<", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Z"}, false},
		{"on (equal), true", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Z"}, true},
		{"not on, true", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-17T00:00:00Z"}, true},
		{"since (>=), boundary", datetimeRule("signup", ">=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Z"}, true},
		{"after (>), true", datetimeRule("signup", ">", jul16Ms), map[string]any{"signup": "2026-07-17T00:00:00Z"}, true},
		{"after (>), false", datetimeRule("signup", ">", jul16Ms), map[string]any{"signup": "2026-07-15T00:00:00Z"}, false},
		// Every symbol is asserted in both directions.
		{"at or before, boundary", datetimeRule("signup", "<=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Z"}, true},
		{"at or before, after", datetimeRule("signup", "<=", jul16Ms), map[string]any{"signup": "2026-07-17T00:00:00Z"}, false},
		{"on (equal), false", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-17T00:00:00Z"}, false},
		{"not on, equal", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Z"}, false},
		{"since (>=), before", datetimeRule("signup", ">=", jul16Ms), map[string]any{"signup": "2026-07-15T00:00:00Z"}, false},
		{"between, inside", datetimeBetween("signup", jan1Ms, dec31Ms), map[string]any{"signup": "2026-06-15T00:00:00Z"}, true},
		{"between, low boundary inclusive", datetimeBetween("signup", jan1Ms, dec31Ms), map[string]any{"signup": "2026-01-01T00:00:00Z"}, true},
		{"between, high boundary inclusive", datetimeBetween("signup", jan1Ms, dec31Ms), map[string]any{"signup": "2026-12-31T00:00:00Z"}, true},
		{"between, before range", datetimeBetween("signup", jan1Ms, dec31Ms), map[string]any{"signup": "2025-12-31T00:00:00Z"}, false},
		{"between, after range", datetimeBetween("signup", jan1Ms, dec31Ms), map[string]any{"signup": "2027-01-01T00:00:00Z"}, false},
		// A leap day is a real date.
		{"leap day", datetimeRule("signup", "=", leapDayMs), map[string]any{"signup": "2024-02-29T00:00:00Z"}, true},
		// Time-zone offsets change the instant.
		{"offset with half-hour minutes", datetimeRule("signup", "=", jul16IndiaMs), map[string]any{"signup": "2026-07-16T00:00:00+05:30"}, true},
		{"rfc3339 subject with offset", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T02:00:00+02:00"}, true},
		{"positive offset precedes utc midnight", datetimeRule("signup", "<", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+05:30"}, true},
		{"negative offset", datetimeRule("signup", "=", jul16PacificMs), map[string]any{"signup": "2026-07-16T00:00:00-08:00"}, true},
		{"negative offset follows utc midnight", datetimeRule("signup", ">", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00-08:00"}, true},
		{"zero offset equals Z", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+00:00"}, true},
		// Sub-second precision is dropped, on both sides. The end-of-day rows are the window the UI
		// emits for a single date, whose upper bound carries .999.
		{"one-digit fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.5Z"}, true},
		{"three-digit fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.500Z"}, true},
		{"six-digit fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.123456Z"}, true},
		{"nine-digit fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.999999999Z"}, true},
		{"zero fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.0Z"}, true},
		{"fractional seconds truncated", datetimeRule("signup", ">=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.500Z"}, true},
		{"end-of-day target drops its .999", datetimeRule("signup", "=", jul16EndMs), map[string]any{"signup": "2026-07-16T23:59:59Z"}, true},
		{"end-of-day target is an inclusive bound", datetimeRule("signup", "<=", jul16EndMs), map[string]any{"signup": "2026-07-16T23:59:59Z"}, true},
		{"end-of-day, fractional subject too", datetimeRule("signup", "=", jul16EndMs), map[string]any{"signup": "2026-07-16T23:59:59.999Z"}, true},
		{"end-of-day inclusive, fractional subject", datetimeRule("signup", "<=", jul16EndMs), map[string]any{"signup": "2026-07-16T23:59:59.999Z"}, true},
		// Trimming and lowercasing.
		{"lowercased subject with fraction", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16t00:00:00.500z"}, true},
		{"lowercased subject with offset", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16t02:00:00+02:00"}, true},
		{"whitespace-padded subject", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": " 2026-07-16T00:00:00Z "}, true},
		{"lowercased rfc3339 subject", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16t00:00:00z"}, true},
		// Shape violations: asserted under both = and != so that "accepted at all" is observable.
		// RFC 3339 also permits 24:00:00 as end-of-day. Platforms disagree on it, so no vector
		// asserts it either way.
		{"one-digit month, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-7-16T00:00:00Z"}, false},
		{"one-digit month, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-7-16T00:00:00Z"}, false},
		{"space separator, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16 00:00:00Z"}, false},
		{"space separator, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16 00:00:00Z"}, false},
		{"missing zone, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00"}, false},
		{"missing zone, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00"}, false},
		{"empty fraction, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.Z"}, false},
		{"empty fraction, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00.Z"}, false},
		{"offset without colon, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+0200"}, false},
		{"offset without colon, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+0200"}, false},
		{"short offset, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+02"}, false},
		{"short offset, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00+02"}, false},
		{"trailing junk, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Zextra"}, false},
		{"trailing junk, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00Zextra"}, false},
		{"bare date, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16"}, false},
		{"bare date, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16"}, false},
		{"basic format, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "20260716T000000Z"}, false},
		{"basic format, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "20260716T000000Z"}, false},
		{"zone after lowercase z, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00z00:00"}, false},
		{"zone after lowercase z, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00z00:00"}, false},
		{"comma fractional separator, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00,5Z"}, false},
		{"comma fractional separator, not-equal also false", datetimeRule("signup", "!=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00,5Z"}, false},
		// Fail-closed: subject must be an RFC3339 string, target must be an epoch-ms number.
		{"numeric subject, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": jul16Ms}, false},
		{"negative epoch-ms target resolves to -1s", datetimeRule("signup", "=", -1500), map[string]any{"signup": "1969-12-31T23:59:59Z"}, true},
		{"negative epoch-ms target, not equal", datetimeRule("signup", "!=", -1500), map[string]any{"signup": "1969-12-31T23:59:59Z"}, false},
		{"negative epoch-ms target, at or after", datetimeRule("signup", ">=", -1500), map[string]any{"signup": "1969-12-31T23:59:59Z"}, true},
		{"negative epoch-ms target, before", datetimeRule("signup", "<", -1500), map[string]any{"signup": "1969-12-31T23:59:58Z"}, true},
		{"negative epoch-ms target, after", datetimeRule("signup", ">", -2500), map[string]any{"signup": "1969-12-31T23:59:59Z"}, true},
		{"subject floors, it does not truncate", datetimeRule("signup", "=", -2000), map[string]any{"signup": "1969-12-31T23:59:58.500Z"}, true},
		{"subject floors, not to -1s", datetimeRule("signup", "!=", -1000), map[string]any{"signup": "1969-12-31T23:59:58.500Z"}, true},
		{"target beyond representable range, no match", datetimeRule("signup", "=", 1e308), map[string]any{"signup": "2026-07-16T00:00:00Z"}, false},
		{"target beyond representable range, greater-than also false", datetimeRule("signup", ">", 1e308), map[string]any{"signup": "2026-07-16T00:00:00Z"}, false},
		{"target beyond representable range, less-than also false", datetimeRule("signup", "<", 1e308), map[string]any{"signup": "2026-07-16T00:00:00Z"}, false},
		{"bare date subject, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16"}, false},
		{"zoneless datetime subject, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "2026-07-16T00:00:00"}, false},
		{"non-datetime string, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{"signup": "yesterday"}, false},
		{"missing property, no match", datetimeRule("signup", "=", jul16Ms), map[string]any{}, false},
	})
}
