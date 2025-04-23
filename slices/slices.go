/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package slices

// Deduplicate removes duplicate elements from a slice.
func Deduplicate[S ~[]E, E comparable](s S) S {
	ded := make(map[E]struct{}, len(s))
	for _, v := range s {
		ded[v] = struct{}{}
	}
	unique := make(S, 0, len(ded))
	for v := range ded {
		unique = append(unique, v)
	}
	return unique
}
