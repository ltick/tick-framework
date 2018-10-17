package utility

import "strings"

// inArray 判断字符串是否存在数组中
func InArrayString(needle string, haystack []string, caseSensitives ...bool) *int {
	caseSensitive := true
	if len(caseSensitives) > 0 {
		caseSensitive = caseSensitives[0]
	}
	for index, value := range haystack {
		if caseSensitive {
			if needle == value {
				return &index
			}
		} else {
			if strings.ToLower(needle) == strings.ToLower(value) {
				return &index
			}
		}
	}
	return nil
}

func InMapString(needle string, haystack map[string]string) bool {
	for _, value := range haystack {
		if needle == value {
			return true
		}
	}
	return false
}
