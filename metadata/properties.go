/*
Copyright 2024 The Dapr Authors
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

package metadata

// Properties contains metadata properties, as a key-value dictionary
type Properties map[string]string

// GetProperty returns a property from the metadata, with support for case-insensitive keys and aliases.
func (p Properties) GetProperty(keys ...string) (val string, ok bool) {
	return GetMetadataProperty(p, keys...)
}

// GetPropertyWithMatchedKey returns a property from the metadata, with support for case-insensitive keys and aliases,
// while returning the original matching metadata field key.
func (p Properties) GetPropertyWithMatchedKey(keys ...string) (key string, val string, ok bool) {
	return GetMetadataPropertyWithMatchedKey(p, keys...)
}

// Decode decodes  metadata into a struct.
// This is an extension of mitchellh/mapstructure which also supports decoding durations.
func (p Properties) Decode(result any) error {
	return decodeMetadataMap(p, result)
}
