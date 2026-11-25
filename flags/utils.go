package flags

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// FNV-1a 64-bit constants
// https://www.ietf.org/archive/id/draft-eastlake-fnv-21.html#section-6.1.2
const (
	fnvPrime  uint64 = 0x100000001B3
	fnvOffset uint64 = 0xCBF29CE484222325
)

// fnv1a64 computes the FNV-1a 64-bit hash of the input data
func fnv1a64(data []byte) uint64 {
	hash := fnvOffset
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime
	}
	return hash
}

// normalizedHash computes a hash value between 0.0 and 1.0 (exclusive)
// Used for variant assignment based on rollout percentages
func normalizedHash(key, salt string) float64 {
	combined := []byte(key + salt)
	hashValue := fnv1a64(combined)
	return float64(hashValue%100) / 100.0
}

// allocationBucket computes the allocation bucket, 0 to 99, for a
// multi-experiment flag. It uses the same mixed hash as mixedNormalizedHash, so
// every bucket bit depends on every input byte.
func allocationBucket(key, salt string) int {
	hashValue := fmix64(fnv1a64([]byte(key + salt)))
	return int(hashValue % 100)
}

// mixedNormalizedHash computes a hash value between 0.0 and 1.0 (exclusive) by
// passing FNV-1a through the MurmurHash3 64-bit finalizer, so every output bit
// depends on every input byte. Used for variant assignment on the experiment
// entries of a multi-experiment flag.
func mixedNormalizedHash(key, salt string) float64 {
	hashValue := fmix64(fnv1a64([]byte(key + salt)))
	return float64(hashValue%100) / 100.0
}

// fmix64 is the MurmurHash3 64-bit finalizer.
func fmix64(value uint64) uint64 {
	value ^= value >> 33
	value *= 0xff51afd7ed558ccd
	value ^= value >> 33
	value *= 0xc4ceb9fe1a85ec53
	value ^= value >> 33
	return value
}

// generateTraceparent generates a W3C traceparent header for distributed tracing
// Format: 00-{trace-id}-{parent-id}-{trace-flags}
func generateTraceparent() (string, error) {
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)

	if _, err := rand.Read(traceID); err != nil {
		return "", err
	}
	if _, err := rand.Read(spanID); err != nil {
		return "", err
	}

	return "00-" + hex.EncodeToString(traceID) + "-" + hex.EncodeToString(spanID) + "-01", nil
}

// lowercaseKeysAndValues recursively lowercases all string keys and values in a map
// Used for case-insensitive comparison in runtime rule evaluation
func lowercaseKeysAndValues(val any) any {
	switch v := val.(type) {
	case string:
		return strings.ToLower(v)
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			result[strings.ToLower(key)] = lowercaseKeysAndValues(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = lowercaseKeysAndValues(item)
		}
		return result
	default:
		return val
	}
}

// lowercaseOnlyLeafNodes recursively lowercases only string values (not keys)
// Used for normalizing JSON Logic rules
func lowercaseOnlyLeafNodes(val any) any {
	switch v := val.(type) {
	case string:
		return strings.ToLower(v)
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			result[key] = lowercaseOnlyLeafNodes(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = lowercaseOnlyLeafNodes(item)
		}
		return result
	default:
		return val
	}
}
