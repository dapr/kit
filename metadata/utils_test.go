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
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataDecode(t *testing.T) {
	t.Run("Test metadata decoding", func(t *testing.T) {
		type TestEmbedded struct {
			MyEmbedded        string `mapstructure:"embedded"`
			MyEmbeddedAliased string `mapstructure:"embalias" mapstructurealiases:"embalias2"`
		}
		type testMetadata struct {
			TestEmbedded `mapstructure:",squash"`

			Mystring                    string           `mapstructure:"mystring"`
			Myduration                  Duration         `mapstructure:"myduration"`
			Myinteger                   int              `mapstructure:"myinteger"`
			Myfloat64                   float64          `mapstructure:"myfloat64"`
			Mybool                      *bool            `mapstructure:"mybool"`
			MyRegularDuration           time.Duration    `mapstructure:"myregularduration"`
			MyDurationWithoutUnit       time.Duration    `mapstructure:"mydurationwithoutunit"`
			MyRegularDurationEmpty      time.Duration    `mapstructure:"myregulardurationempty"`
			MyDurationArray             []time.Duration  `mapstructure:"mydurationarray"`
			MyDurationArrayPointer      *[]time.Duration `mapstructure:"mydurationarraypointer"`
			MyDurationArrayPointerEmpty *[]time.Duration `mapstructure:"mydurationarraypointerempty"`

			MyRegularDurationDefaultValueUnset time.Duration `mapstructure:"myregulardurationdefaultvalueunset"`
			MyRegularDurationDefaultValueEmpty time.Duration `mapstructure:"myregulardurationdefaultvalueempty"`

			AliasedFieldA string `mapstructure:"aliasA1" mapstructurealiases:"aliasA2"`
			AliasedFieldB string `mapstructure:"aliasB1" mapstructurealiases:"aliasB2"`
		}

		var m testMetadata
		m.MyRegularDurationDefaultValueUnset = time.Hour
		m.MyRegularDurationDefaultValueEmpty = time.Hour

		testData := map[string]string{
			"mystring":               "test",
			"myduration":             "3s",
			"myinteger":              "1",
			"myfloat64":              "1.1",
			"mybool":                 "true",
			"myregularduration":      "6m",
			"mydurationwithoutunit":  "17",
			"myregulardurationempty": "",
			// Not setting myregulardurationdefaultvalueunset on purpose
			"myregulardurationdefaultvalueempty": "",
			"mydurationarray":                    "1s,2s,3s,10",
			"mydurationarraypointer":             "1s,10,2s,20,3s,30",
			"mydurationarraypointerempty":        ",",
			"aliasA2":                            "hello",
			"aliasB1":                            "ciao",
			"aliasB2":                            "bonjour",
			"embedded":                           "hi",
			"embalias2":                          "ciao",
		}

		err := DecodeMetadata(testData, &m)

		require.NoError(t, err)
		assert.True(t, *m.Mybool)
		assert.Equal(t, "test", m.Mystring)
		assert.Equal(t, 1, m.Myinteger)
		assert.InEpsilon(t, 1.1, m.Myfloat64, 0.01)
		assert.Equal(t, Duration{Duration: 3 * time.Second}, m.Myduration)
		assert.Equal(t, 6*time.Minute, m.MyRegularDuration)
		assert.Equal(t, time.Second*17, m.MyDurationWithoutUnit)
		assert.Equal(t, time.Duration(0), m.MyRegularDurationEmpty)
		assert.Equal(t, time.Hour, m.MyRegularDurationDefaultValueUnset)
		assert.Equal(t, time.Duration(0), m.MyRegularDurationDefaultValueEmpty)
		assert.Equal(t, []time.Duration{time.Second, time.Second * 2, time.Second * 3, time.Second * 10}, m.MyDurationArray)
		assert.Equal(t, []time.Duration{time.Second, time.Second * 10, time.Second * 2, time.Second * 20, time.Second * 3, time.Second * 30}, *m.MyDurationArrayPointer)
		assert.Equal(t, []time.Duration{}, *m.MyDurationArrayPointerEmpty)
		assert.Equal(t, "hello", m.AliasedFieldA)
		assert.Equal(t, "ciao", m.AliasedFieldB)
		assert.Equal(t, "hi", m.TestEmbedded.MyEmbedded)
		assert.Equal(t, "ciao", m.TestEmbedded.MyEmbeddedAliased)
	})

	t.Run("Test metadata decode hook for truthy values", func(t *testing.T) {
		type testMetadata struct {
			BoolPointer            *bool
			BoolPointerNotProvided *bool
			BoolValueOn            bool
			BoolValue1             bool
			BoolValueTrue          bool
			BoolValue0             bool
			BoolValueFalse         bool
			BoolValueNonsense      bool
		}

		var m testMetadata

		testData := make(map[string]string)
		testData["boolpointer"] = "on"
		testData["boolvalueon"] = "on"
		testData["boolvalue1"] = "1"
		testData["boolvaluetrue"] = "true"
		testData["boolvalue0"] = "0"
		testData["boolvaluefalse"] = "false"
		testData["boolvaluenonsense"] = "nonsense"

		err := DecodeMetadata(testData, &m)
		require.NoError(t, err)
		assert.True(t, *m.BoolPointer)
		assert.True(t, m.BoolValueOn)
		assert.True(t, m.BoolValue1)
		assert.True(t, m.BoolValueTrue)
		assert.False(t, m.BoolValue0)
		assert.False(t, m.BoolValueFalse)
		assert.False(t, m.BoolValueNonsense)
		assert.Nil(t, m.BoolPointerNotProvided)
	})

	t.Run("Test metadata decode for string arrays", func(t *testing.T) {
		type testMetadata struct {
			StringArray                           []string
			StringArrayPointer                    *[]string
			EmptyStringArray                      []string
			EmptyStringArrayPointer               *[]string
			EmptyStringArrayWithComma             []string
			EmptyStringArrayPointerWithComma      *[]string
			StringArrayOneElement                 []string
			StringArrayOneElementPointer          *[]string
			StringArrayOneElementWithComma        []string
			StringArrayOneElementPointerWithComma *[]string
		}

		var m testMetadata

		testData := make(map[string]string)
		testData["stringarray"] = "one,two,three"
		testData["stringarraypointer"] = "one,two,three"
		testData["emptystringarray"] = ""
		testData["emptystringarraypointer"] = ""
		testData["stringarrayoneelement"] = "test"
		testData["stringarrayoneelementpointer"] = "test"
		testData["stringarrayoneelementwithcomma"] = "test,"
		testData["stringarrayoneelementpointerwithcomma"] = "test,"
		testData["emptystringarraywithcomma"] = ","
		testData["emptystringarraypointerwithcomma"] = ","

		err := DecodeMetadata(testData, &m)
		require.NoError(t, err)
		assert.Equal(t, []string{"one", "two", "three"}, m.StringArray)
		assert.Equal(t, []string{"one", "two", "three"}, *m.StringArrayPointer)
		assert.Equal(t, []string{""}, m.EmptyStringArray)
		assert.Equal(t, []string{""}, *m.EmptyStringArrayPointer)
		assert.Equal(t, []string{"test"}, m.StringArrayOneElement)
		assert.Equal(t, []string{"test"}, *m.StringArrayOneElementPointer)
		assert.Equal(t, []string{"test", ""}, m.StringArrayOneElementWithComma)
		assert.Equal(t, []string{"test", ""}, *m.StringArrayOneElementPointerWithComma)
		assert.Equal(t, []string{"", ""}, m.EmptyStringArrayWithComma)
		assert.Equal(t, []string{"", ""}, *m.EmptyStringArrayPointerWithComma)
	})

	t.Run("Test metadata decode hook for byte sizes", func(t *testing.T) {
		type testMetadata struct {
			BytesizeValue1              ByteSize
			BytesizeValue2              ByteSize
			BytesizeValue3              ByteSize
			BytesizeValue4              ByteSize
			BytesizeValueNotProvided    ByteSize
			BytesizeValuePtr            *ByteSize
			BytesizeValuePtrNotProvided *ByteSize
		}

		var m testMetadata

		testData := make(map[string]any)
		testData["bytesizevalue1"] = "100"
		testData["bytesizevalue2"] = 100
		testData["bytesizevalue3"] = "1Ki"
		testData["bytesizevalue4"] = "1000k"
		testData["bytesizevalueptr"] = "1Gi"

		err := DecodeMetadata(testData, &m)
		require.NoError(t, err)
		assert.Equal(t, "100", m.BytesizeValue1.String())
		assert.Equal(t, "100", m.BytesizeValue2.String())
		assert.Equal(t, "1Ki", m.BytesizeValue3.String())
		assert.Equal(t, "1M", m.BytesizeValue4.String())
		assert.Equal(t, "1Gi", m.BytesizeValuePtr.String())
		assert.Nil(t, m.BytesizeValuePtrNotProvided)
		assert.Equal(t, "0", m.BytesizeValueNotProvided.String())
	})
}

func TestResolveAliases(t *testing.T) {
	type Embedded struct {
		Hello string `mapstructure:"hello" mapstructurealiases:"ciao"`
	}

	tests := []struct {
		name    string
		md      map[string]string
		result  any
		wantErr bool
		wantMd  map[string]string
	}{
		{
			name: "no aliases",
			md: map[string]string{
				"hello": "world",
				"ciao":  "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"hello"`
				Ciao    string `mapstructure:"ciao"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"hello": "world",
				"ciao":  "mondo",
			},
		},
		{
			name: "set with aliased field",
			md: map[string]string{
				"ciao": "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"hello" mapstructurealiases:"ciao"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"hello": "mondo",
				"ciao":  "mondo",
			},
		},
		{
			name: "do not overwrite existing fields with aliases",
			md: map[string]string{
				"hello": "world",
				"ciao":  "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"hello" mapstructurealiases:"ciao"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"hello": "world",
				"ciao":  "mondo",
			},
		},
		{
			name: "no fields with aliased value",
			md: map[string]string{
				"bonjour": "monde",
			},
			result: &struct {
				Hello   string `mapstructure:"hello" mapstructurealiases:"ciao"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"bonjour": "monde",
			},
		},
		{
			name: "multiple aliases",
			md: map[string]string{
				"bonjour": "monde",
			},
			result: &struct {
				Hello string `mapstructure:"hello" mapstructurealiases:"ciao,bonjour"`
			}{},
			wantMd: map[string]string{
				"hello":   "monde",
				"bonjour": "monde",
			},
		},
		{
			name: "first alias wins",
			md: map[string]string{
				"ciao":    "mondo",
				"bonjour": "monde",
			},
			result: &struct {
				Hello string `mapstructure:"hello" mapstructurealiases:"ciao,bonjour"`
			}{},
			wantMd: map[string]string{
				"hello":   "mondo",
				"ciao":    "mondo",
				"bonjour": "monde",
			},
		},
		{
			name: "no aliases with mixed case",
			md: map[string]string{
				"hello": "world",
				"CIAO":  "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"Hello"`
				Ciao    string `mapstructure:"ciao"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"hello": "world",
				"CIAO":  "mondo",
			},
		},
		{
			name: "set with aliased field with mixed case",
			md: map[string]string{
				"ciao": "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"Hello" mapstructurealiases:"CIAO"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"Hello": "mondo",
				"ciao":  "mondo",
			},
		},
		{
			name: "do not overwrite existing fields with aliases with mixed cases",
			md: map[string]string{
				"HELLO": "world",
				"CIAO":  "mondo",
			},
			result: &struct {
				Hello   string `mapstructure:"hELLo" mapstructurealiases:"cIAo"`
				Bonjour string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"HELLO": "world",
				"CIAO":  "mondo",
			},
		},
		{
			name: "multiple aliases with mixed cases",
			md: map[string]string{
				"bonjour": "monde",
			},
			result: &struct {
				Hello string `mapstructure:"HELLO" mapstructurealiases:"CIAO,BONJOUR"`
			}{},
			wantMd: map[string]string{
				"HELLO":   "monde",
				"bonjour": "monde",
			},
		},
		{
			name: "aliases in embedded struct",
			md: map[string]string{
				"ciao":    "mondo",
				"bonjour": "monde",
			},
			result: &struct {
				Embedded `mapstructure:",squash"`
				Bonjour  string `mapstructure:"bonjour"`
			}{},
			wantMd: map[string]string{
				"bonjour": "monde",
				"ciao":    "mondo",
				"hello":   "mondo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := maps.Clone(tt.md)
			err := resolveAliases(md, reflect.TypeOf(tt.result))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantMd, md)
		})
	}
}

func TestGetMetadataPropertyWithMatchedKey(t *testing.T) {
	props := map[string]string{
		"key1":     "value1",
		"key2":     "value2",
		"key3":     "value3",
		"emptyKey": "",
	}

	t.Run("Existing key", func(t *testing.T) {
		key, val, ok := GetMetadataPropertyWithMatchedKey(props, "key1", "key2")
		assert.True(t, ok)
		assert.Equal(t, "key1", key)
		assert.Equal(t, "value1", val)
	})

	t.Run("Case-insensitive matching", func(t *testing.T) {
		key, val, ok := GetMetadataPropertyWithMatchedKey(props, "KEY1")
		assert.True(t, ok)
		assert.Equal(t, "KEY1", key)
		assert.Equal(t, "value1", val)
	})

	t.Run("Non-existing key", func(t *testing.T) {
		key, val, ok := GetMetadataPropertyWithMatchedKey(props, "key4")
		assert.False(t, ok)
		assert.Equal(t, "", key)
		assert.Equal(t, "", val)
	})

	t.Run("Empty properties", func(t *testing.T) {
		key, val, ok := GetMetadataPropertyWithMatchedKey(nil, "key1")
		assert.False(t, ok)
		assert.Equal(t, "", key)
		assert.Equal(t, "", val)
	})

	t.Run("Value is empty", func(t *testing.T) {
		key, val, ok := GetMetadataPropertyWithMatchedKey(props, "EmptyKey")
		assert.True(t, ok)
		assert.Equal(t, "EmptyKey", key)
		assert.Equal(t, "", val)
	})
}
