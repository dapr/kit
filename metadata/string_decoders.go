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
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/dapr/kit/ptr"
	kitstrings "github.com/dapr/kit/strings"
)

func toTruthyBoolHookFunc() mapstructure.DecodeHookFunc {
	stringType := reflect.TypeOf("")
	boolType := reflect.TypeOf(true)
	boolPtrType := reflect.TypeOf(ptr.Of(true))

	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f == stringType && t == boolType {
			return kitstrings.IsTruthy(data.(string)), nil
		}
		if f == stringType && t == boolPtrType {
			return ptr.Of(kitstrings.IsTruthy(data.(string))), nil
		}
		return data, nil
	}
}

func toStringArrayHookFunc() mapstructure.DecodeHookFunc {
	stringType := reflect.TypeOf("")
	stringSliceType := reflect.TypeOf([]string{})
	stringSlicePtrType := reflect.TypeOf(ptr.Of([]string{}))

	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f == stringType && t == stringSliceType {
			return strings.Split(data.(string), ","), nil
		}
		if f == stringType && t == stringSlicePtrType {
			return ptr.Of(strings.Split(data.(string), ",")), nil
		}
		return data, nil
	}
}

func toTimeDurationArrayHookFunc() mapstructure.DecodeHookFunc {
	convert := func(input string) ([]time.Duration, error) {
		parts := strings.Split(input, ",")
		res := make([]time.Duration, 0, len(parts))
		for _, v := range parts {
			input := strings.TrimSpace(v)
			if input == "" {
				continue
			}
			val, err := time.ParseDuration(input)
			if err != nil {
				// If we can't parse the duration, try parsing it as int64 seconds
				seconds, errParse := strconv.ParseInt(input, 10, 0)
				if errParse != nil {
					return nil, errors.Join(err, errParse)
				}
				val = time.Duration(seconds * int64(time.Second))
			}
			res = append(res, val)
		}
		return res, nil
	}

	stringType := reflect.TypeOf("")
	durationSliceType := reflect.TypeOf([]time.Duration{})
	durationSlicePtrType := reflect.TypeOf(ptr.Of([]time.Duration{}))

	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f == stringType && t == durationSliceType {
			inputArrayString := data.(string)
			return convert(inputArrayString)
		}
		if f == stringType && t == durationSlicePtrType {
			inputArrayString := data.(string)
			res, err := convert(inputArrayString)
			if err != nil {
				return nil, err
			}
			return ptr.Of(res), nil
		}
		return data, nil
	}
}
