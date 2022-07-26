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

// Based on https://github.com/Azure/azure-sdk-for-go/blob/sdk/azcore/v1.1.1/sdk/azcore/to/to_test.go
// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package to

import (
	"testing"
)

func TestPtr(t *testing.T) {
	b := true
	pb := Ptr(b)
	if pb == nil {
		t.Fatal("unexpected nil conversion")
	}
	if *pb != b {
		t.Fatalf("got %v, want %v", *pb, b)
	}
}

func TestSliceOfPtrs(t *testing.T) {
	arr := SliceOfPtrs[int]()
	if len(arr) != 0 {
		t.Fatal("expected zero length")
	}
	arr = SliceOfPtrs(1, 2, 3, 4, 5)
	for i, v := range arr {
		if *v != i+1 {
			t.Fatal("values don't match")
		}
	}
}
