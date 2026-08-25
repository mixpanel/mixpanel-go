package flags

import (
	"math"
	"regexp"
	"strings"
	"time"

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
	cmp := compareSemver(actualVersion, targetVersion)
	return comparatorMatches(cmp, symbol)
}

// Strip optional build metadata and separate the core version from pre-release identifiers
func splitSemver(version string) (core, prerelease []string) {
	if plus := strings.Index(version, `+`); plus != -1 {
		version = version[:plus]
	}
	dash := strings.Index(version, `-`)
	if dash == -1 {
		return strings.Split(version, `.`), nil
	}
	return strings.Split(version[:dash], `.`), strings.Split(version[dash+1:], `.`)
}

func isNumericIdentifier(identifier string) bool {
	if identifier == `` {
		return false
	}
	for i := 0; i < len(identifier); i++ {
		if identifier[i] < '0' || identifier[i] > '9' {
			return false
		}
	}
	return true
}

// Numeric identifiers carry no leading zeros, so the longer run of digits is the larger number.
// Comparing them as digits rather than parsing to a fixed-width integer keeps versions that overflow
// an int64 ordered correctly.
func compareNumeric(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// SemVer 2.0.0 section 11.4: digits compare numerically, a numeric identifier ranks below an
// alphanumeric one, and anything else compares by ASCII order.
func comparePrereleaseIdentifier(a, b string) int {
	aNumeric, bNumeric := isNumericIdentifier(a), isNumericIdentifier(b)
	switch {
	case aNumeric && bNumeric:
		return compareNumeric(a, b)
	case aNumeric:
		return -1
	case bNumeric:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// Ordering per SemVer 2.0.0 section 11. Both operands have already been normalized and matched
// against the official regex, so the core holds exactly three numeric identifiers and every
// prerelease field is well-formed; the split needs no error path.
func compareSemver(a, b string) int {
	aCore, aPrerelease := splitSemver(a)
	bCore, bPrerelease := splitSemver(b)

	for i := range aCore {
		if result := compareNumeric(aCore[i], bCore[i]); result != 0 {
			return result
		}
	}

	// A prerelease ranks below the release it belongs to (section 11.3).
	switch {
	case len(aPrerelease) == 0 && len(bPrerelease) == 0:
		return 0
	case len(aPrerelease) == 0:
		return 1
	case len(bPrerelease) == 0:
		return -1
	}

	for i := 0; i < len(aPrerelease) && i < len(bPrerelease); i++ {
		if result := comparePrereleaseIdentifier(aPrerelease[i], bPrerelease[i]); result != 0 {
			return result
		}
	}
	// Every field so far is equal, so the longer list wins (section 11.4.4).
	switch {
	case len(aPrerelease) < len(bPrerelease):
		return -1
	case len(aPrerelease) > len(bPrerelease):
		return 1
	}
	return 0
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
	cmp := int(actualSec - targetSec)
	return comparatorMatches(cmp, symbol)
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
	case `===`:
		return cmp == 0
	case `!==`:
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
