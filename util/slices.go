package util

const minLen = 2

func Unique[S ~[]E, E comparable](s S) S {
	if len(s) < minLen {
		return s
	}

	result := make([]E, 0, len(s))
	seen := make(map[E]struct{}, len(s))

	for _, item := range s {
		if _, ok := seen[item]; ok {
			continue
		}

		seen[item] = struct{}{}
		result = append(result, item)
	}

	return result
}
