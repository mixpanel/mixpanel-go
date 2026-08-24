package flags

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/diegoholiveira/jsonlogic/v3"
)

// Using the official semantic versioning 2.0.0 regular expression to handle cross-platform validation
// differences on other SDK's. For example, some platforms allow leading zeros even though it is not valid
// as part of the Semver 2.0.0 spec. See https://semver.org/
var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Strict RFC3339 guard for datetime strings.
var rfc3339Regex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]\d{2}:\d{2})$`)

// SemVer 2.0.0 requires major.minor.patch; partial versions are zero-padded to this.
const semverParts = 3

// Register the operators into jsonlogic's process-global registry when the package loads.
func init() {
	jsonlogic.AddOperator("semver_compare", semverCompare)
	jsonlogic.AddOperator("datetime_compare", datetimeCompare)
}

// Implements a custom operation for semantic versioning comparison that conforms to the semver 2.0.0
// standard. Prior to comparison, any leading version prefix is stripped.
func semverCompare(values, _ any) any {
	actual, symbol, target, ok := operands(values)
	if !ok {
		return false
	}
	actualStr, ok := actual.(string)
	if !ok {
		return false
	}
	targetStr, ok := target.(string)
	if !ok {
		return false
	}
	actualVersion := normalizeSemver(actualStr)
	targetVersion := normalizeSemver(targetStr)
	if !semverRegex.MatchString(actualVersion) || !semverRegex.MatchString(targetVersion) {
		return false
	}
	actualVer, err := semver.NewVersion(actualVersion)
	if err != nil {
		return false
	}
	targetVer, err := semver.NewVersion(targetVersion)
	if err != nil {
		return false
	}
	return comparatorMatches(actualVer.Compare(targetVer), symbol)
}

// Implements a custom operation for datetime comparison. The target value stored on the feature flag is
// the millisecond epoch, whereas the actual value provided at evaluation time must be RFC-3339 formatted.
func datetimeCompare(values, _ any) any {
	actual, symbol, target, ok := operands(values)
	if !ok {
		return false
	}
	actualSec, ok := convertRFC3339ToUnixSeconds(actual)
	if !ok {
		return false
	}
	targetSec, ok := convertUnixMillisecondsToSeconds(target)
	if !ok {
		return false
	}
	return comparatorMatches(int(actualSec-targetSec), symbol)
}

func operands(values any) (actual any, symbol string, target any, ok bool) {
	slice, isSlice := values.([]any)
	if !isSlice || len(slice) != 3 {
		return nil, ``, nil, false
	}
	if symbol, ok = slice[1].(string); !ok {
		return nil, ``, nil, false
	}
	return slice[0], symbol, slice[2], true
}

func comparatorMatches(cmp int, symbol string) bool {
	switch symbol {
	case `=`:
		return cmp == 0
	case `!=`:
		return cmp != 0
	case `<`:
		return cmp < 0
	case `<=`:
		return cmp <= 0
	case `>`:
		return cmp > 0
	case `>=`:
		return cmp >= 0
	default:
		return false
	}
}

func normalizeSemver(raw string) string {
	stripped := strings.TrimSpace(raw)
	if strings.HasPrefix(stripped, `v`) || strings.HasPrefix(stripped, `V`) {
		stripped = stripped[1:]
	}

	suffixStart := len(stripped)
	for _, separator := range []string{`-`, `+`} {
		if index := strings.Index(stripped, separator); index != -1 && index < suffixStart {
			suffixStart = index
		}
	}

	core := stripped[:suffixStart]
	suffix := stripped[suffixStart:]

	segments := strings.Split(core, `.`)
	for len(segments) < semverParts {
		segments = append(segments, `0`)
	}
	return strings.Join(segments, `.`) + suffix
}

func convertRFC3339ToUnixSeconds(v any) (int64, bool) {
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	normalized := strings.ToUpper(strings.TrimSpace(s))
	if !rfc3339Regex.MatchString(normalized) {
		return 0, false
	}
	parsed, err := time.Parse(time.RFC3339, normalized)
	if err != nil {
		return 0, false
	}
	return parsed.Unix(), true
}

func convertUnixMillisecondsToSeconds(v any) (int64, bool) {
	ms, ok := v.(float64)
	if !ok {
		return 0, false
	}
	// Converting a float64 that int64 cannot represent yields an implementation-defined value, which
	// would turn an out-of-range target into a real instant instead of failing closed.
	if math.IsNaN(ms) || ms >= math.MaxInt64 || ms <= math.MinInt64 {
		return 0, false
	}
	return int64(ms) / 1000, true
}
