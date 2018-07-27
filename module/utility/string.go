package utility

import (
	"strings"
)

func (this *Instance) WildcardMatch(pattern, subject string) bool {
	const WILDCARD = "*"
	// Empty pattern can only match empty subject
	if pattern == "" {
		return subject == pattern
	}

	// If the pattern _is_ awildcard, it matches everything
	if pattern == WILDCARD {
		return true
	}

	parts := strings.Split(pattern, WILDCARD)

	if len(parts) == 1 {
		// No wildcards in pattern, so test for equality
		return subject == pattern
	}

	leadingWildcard := strings.HasPrefix(pattern, WILDCARD)
	trailingWildcard := strings.HasSuffix(pattern, WILDCARD)
	end := len(parts) - 1

	// Go over the leading parts and ensure they match.
	for i := 0; i < end; i++ {
		idx := strings.Index(subject, parts[i])

		switch i {
		case 0:
			// Check the first section. Requires special handling.
			if !leadingWildcard && idx != 0 {
				return false
			}
		default:
			// Check that the middle parts match.
			if idx < 0 {
				return false
			}
		}

		// Trim evaluated text from subject as we loop over the pattern.
		subject = subject[idx+len(parts[i]):]
	}

	// Reached the last section. Requires special handling.
	return trailingWildcard || strings.HasSuffix(subject, parts[end])
}
