/*
Copyright 2023 The Dapr Authors
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

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
)

// GetMetadataProperty returns a property from the metadata map, with support for case-insensitive keys and aliases.
func GetMetadataProperty(props map[string]string, keys ...string) (val string, ok bool) {
	lcProps := make(map[string]string, len(props))
	for k, v := range props {
		lcProps[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		val, ok = lcProps[strings.ToLower(k)]
		if ok {
			return val, true
		}
	}
	return "", false
}

// DecodeMetadata decodes a component metadata into a struct.
// This is an extension of mitchellh/mapstructure which also supports decoding durations.
func DecodeMetadata(input any, result any) error {
	// avoids a common mistake of passing the metadata struct, instead of the properties map
	// if input is of type struct, cast it to metadata.Base and access the Properties instead
	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Struct {
		f := v.FieldByName("Properties")
		if f.IsValid() && f.Kind() == reflect.Map {
			input = f.Interface().(map[string]string)
		}
	}

	inputMap, err := cast.ToStringMapStringE(input)
	if err != nil {
		return fmt.Errorf("input object cannot be cast to map[string]string: %w", err)
	}

	// Handle aliases
	err = resolveAliases(inputMap, reflect.TypeOf(result))
	if err != nil {
		return fmt.Errorf("failed to resolve aliases: %w", err)
	}

	// Finally, decode the metadata using mapstructure
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			toTimeDurationArrayHookFunc(),
			toTimeDurationHookFunc(),
			toTruthyBoolHookFunc(),
			toStringArrayHookFunc(),
			toByteSizeHookFunc(),
		),
		Metadata:         nil,
		Result:           result,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(inputMap)
}

func resolveAliases(md map[string]string, t reflect.Type) error {
	// Get the list of all keys in the map
	keys := make(map[string]string, len(md))
	for k := range md {
		lk := strings.ToLower(k)

		// Check if there are duplicate keys after lowercasing
		_, ok := keys[lk]
		if ok {
			return fmt.Errorf("key %s is duplicate in the metadata", lk)
		}

		keys[lk] = k
	}

	// Error if result is not pointer to struct, or pointer to pointer to struct
	if t.Kind() != reflect.Pointer {
		return fmt.Errorf("not a pointer: %s", t.Kind().String())
	}
	t = t.Elem()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("not a struct: %s", t.Kind().String())
	}

	// Iterate through all the properties, possibly recursively
	resolveAliasesInType(md, keys, t)

	return nil
}

func resolveAliasesInType(md map[string]string, keys map[string]string, t reflect.Type) {
	// Iterate through all the properties of the type to see if anyone has the "mapstructurealiases" property
	for i := 0; i < t.NumField(); i++ {
		currentField := t.Field(i)

		// Ignored fields that are not exported or that don't have a "mapstructure" tag
		mapstructureTag := currentField.Tag.Get("mapstructure")
		if !currentField.IsExported() || mapstructureTag == "" {
			continue
		}

		// Check if this is an embedded struct
		if mapstructureTag == ",squash" {
			resolveAliasesInType(md, keys, currentField.Type)
			continue
		}

		// If the current property has a value in the metadata, then we don't need to handle aliases
		_, ok := keys[strings.ToLower(mapstructureTag)]
		if ok {
			continue
		}

		// Check if there's a "mapstructurealiases" tag
		aliasesTag := strings.ToLower(currentField.Tag.Get("mapstructurealiases"))
		if aliasesTag == "" {
			continue
		}

		// Look for the first alias that has a value
		var mdKey string
		for _, alias := range strings.Split(aliasesTag, ",") {
			mdKey, ok = keys[alias]
			if !ok {
				continue
			}

			// We found an alias
			md[mapstructureTag] = md[mdKey]
			break
		}
	}
}
