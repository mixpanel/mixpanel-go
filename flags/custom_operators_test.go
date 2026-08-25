package flags

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/diegoholiveira/jsonlogic/v3"
	"github.com/stretchr/testify/require"
)

// The golden vectors are the cross-SDK contract for the custom operators; the canonical copy and its
// README live in the analytics monorepo. Cases run through the JsonLogic engine so that operator
// registration is covered alongside the comparison itself.

//go:embed testdata/semver_compare_tests.json
var semverVectors []byte

//go:embed testdata/datetime_compare_tests.json
var datetimeVectors []byte

// The property key the vectors are evaluated against. It is plumbing the test supplies, so any name
// works as long as the rule and the data agree on it.
const vectorKey = "value"

// vectorCase is one [subject, symbol, target, expected] entry, tagged with the heading it appeared
// under. A nil subject means the property is not set.
type vectorCase struct {
	section string
	subject any
	symbol  string
	target  any
	want    bool
}

// rule builds the JsonLogic rule the case evaluates: {"<operator>": [{"var": key}, symbol, target]}.
func (c vectorCase) rule(operator string) map[string]any {
	return map[string]any{operator: []any{map[string]any{"var": vectorKey}, c.symbol, c.target}}
}

// data builds the event the rule reads from, omitting the key entirely for an unset property.
func (c vectorCase) data() map[string]any {
	if c.subject == nil {
		return map[string]any{}
	}
	return map[string]any{vectorKey: c.subject}
}

// loadVectors reads a golden-vector file. String entries are headings, array entries are cases.
func loadVectors(t *testing.T, raw []byte) []vectorCase {
	t.Helper()

	var entries []json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &entries))

	var section string
	cases := make([]vectorCase, 0, len(entries))
	for i, entry := range entries {
		var heading string
		if err := json.Unmarshal(entry, &heading); err == nil {
			section = heading
			continue
		}

		var row []json.RawMessage
		require.NoErrorf(t, json.Unmarshal(entry, &row), "entry %d is neither a heading nor a case", i)
		require.Lenf(t, row, 4, "entry %d must be [subject, operator, target, expected]", i)

		parsed := vectorCase{section: section}
		require.NoErrorf(t, json.Unmarshal(row[0], &parsed.subject), "entry %d has an unreadable subject", i)
		require.NoErrorf(t, json.Unmarshal(row[1], &parsed.symbol), "entry %d has a non-string operator", i)
		require.NoErrorf(t, json.Unmarshal(row[2], &parsed.target), "entry %d has an unreadable target", i)
		require.NoErrorf(t, json.Unmarshal(row[3], &parsed.want), "entry %d has a non-boolean expectation", i)
		cases = append(cases, parsed)
	}
	require.NotEmpty(t, cases)
	return cases
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

func runVectors(t *testing.T, operator string, raw []byte) {
	t.Helper()

	for i, tc := range loadVectors(t, raw) {
		rule, data := tc.rule(operator), tc.data()
		t.Run(fmt.Sprintf("%d %s: %v %s %v", i, tc.section, tc.subject, tc.symbol, tc.target), func(t *testing.T) {
			require.Equal(t, tc.want, evalRule(t, rule, data))
		})
	}
}

func TestSemverCompareOperator(t *testing.T) {
	runVectors(t, "semver_compare", semverVectors)
}

func TestDatetimeCompareOperator(t *testing.T) {
	runVectors(t, "datetime_compare", datetimeVectors)
}

// An unset property must produce data with no key at all, rather than a key holding a null. Both
// spellings fail closed, so the vectors alone cannot tell them apart.
func TestUnsetSubjectOmitsTheProperty(t *testing.T) {
	unset := vectorCase{subject: nil, symbol: "===", target: "1.2.3"}
	require.NotContains(t, unset.data(), vectorKey)

	set := vectorCase{subject: "1.2.3", symbol: "===", target: "1.2.3"}
	require.Equal(t, map[string]any{vectorKey: "1.2.3"}, set.data())
}
