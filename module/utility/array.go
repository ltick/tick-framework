package utility

import "strings"

// inArray 判断字符串是否存在数组中
func (this *Instance) InArrayString(needle string, haystack []string, caseSensitives ...bool) bool {
	caseSensitive := true
	if len(caseSensitives) > 0 {
		caseSensitive = caseSensitives[0]
	}
	for _, value := range haystack {
		if caseSensitive {
			if needle == value {
				return true
			}
		} else {
			if strings.ToLower(needle) == strings.ToLower(value) {
				return true
			}
		}
	}
	return false
}

func (this *Instance) InMapString(needle string, haystack map[string]string) bool {
	for _, value := range haystack {
		if needle == value {
			return true
		}
	}
	return false
}
