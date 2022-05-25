package util

// Copyright 2022 Thomas Pilz

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

func CastInterfaceSliceToStringSlice(input []interface{}) []string {
	cast := make([]string, len(input))
	for i := range input {
		cast[i] = input[i].(string)
	}
	return cast
}

func CastToInterfaceSlice(slice ...interface{}) []interface{} {
	return slice
}
