package utility

import (
	"bytes"
	"strings"
	"unsafe"
)

func WildcardMatch(pattern, subject string) bool {
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

func SnakeToUpperCamel(s string) string {
	buf := bytes.NewBufferString("")
	for _, v := range strings.Split(s, "_") {
		if len(v) > 0 {
			buf.WriteString(strings.ToUpper(v[:1]))
			buf.WriteString(v[1:])
		}
	}
	return buf.String()
}

// SnakeString converts the accepted string to a snake string (XxYy to xx_yy)
func SnakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	for _, d := range StringToBytes(s) {
		if d >= 'A' && d <= 'Z' {
			if j {
				data = append(data, '_')
				j = false
			}
		} else if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(BytesToString(data))
}

// CamelString converts the accepted string to a camel string (xx_yy to XxYy)
func CamelString(s string) string {
	data := make([]byte, 0, len(s))
	j := false
	k := false
	num := len(s) - 1
	for i := 0; i <= num; i++ {
		d := s[i]
		if k == false && d >= 'A' && d <= 'Z' {
			k = true
		}
		if d >= 'a' && d <= 'z' && (j || k == false) {
			d = d - 32
			j = false
			k = true
		}
		if k && d == '_' && num > i && s[i+1] >= 'a' && s[i+1] <= 'z' {
			j = true
			continue
		}
		data = append(data, d)
	}
	return string(data[:])
}

// StringToBytes convert string type to []byte type.
// NOTE: panic if modify the member value of the []byte.
func StringToBytes(s string) []byte {
	sp := *(*[2]uintptr)(unsafe.Pointer(&s))
	bp := [3]uintptr{sp[0], sp[1], sp[1]}
	return *(*[]byte)(unsafe.Pointer(&bp))
}