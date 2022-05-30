package util

// Get values of a map
func Values[K comparable, V any](m *map[K]V) []V {
	values := make([]V, 0, len(*m))

	for _, value := range *m {
		values = append(values, value)
	}
	return values
}

// Copy a map
func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	copy := make(map[K]V, len(m))
	for key, value := range m {
		copy[key] = value
	}
	return copy
}
