package util

func CastInterfaceSliceToStringSlice(input []interface{}) []string {
	cast := make([]string, len(input))
	for i := range input {
		cast[i] = input[i].(string)
	}
	return cast
}
