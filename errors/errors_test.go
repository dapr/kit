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

package errors

import (
	"encoding/json"
	"fmt"
	"go/types"
	"net/http"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/context"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestError_HTTPStatusCode(t *testing.T) {
	httpStatusCode := http.StatusTeapot
	kitErr := NewBuilder(
		grpcCodes.ResourceExhausted,
		httpStatusCode,
		"Test Msg",
		"SOME_ERROR",
	).
		WithErrorInfo("fake", map[string]string{"fake": "test"}).
		Build()

	err, ok := kitErr.(Error)
	require.True(t, ok, httpStatusCode, err.HTTPStatusCode())
}

func TestError_GrpcStatusCode(t *testing.T) {
	grpcStatusCode := grpcCodes.ResourceExhausted
	kitErr := NewBuilder(
		grpcStatusCode,
		http.StatusTeapot,
		"Test Msg",
		"SOME_ERROR",
	).
		WithErrorInfo("fake", map[string]string{"fake": "test"}).
		Build()

	err, ok := kitErr.(Error)
	require.True(t, ok, grpcStatusCode, err.GrpcStatusCode())
}

func TestError_AddDetails(t *testing.T) {
	reason := "example_reason"
	metadata := map[string]string{"key": "value"}

	details1 := &errdetails.ErrorInfo{
		Domain:   Domain,
		Reason:   reason,
		Metadata: metadata,
	}

	details2 := &errdetails.PreconditionFailure_Violation{
		Type:        "TOS",
		Subject:     "google.com/cloud",
		Description: "test_description",
	}

	expected := Error{
		grpcCode: grpcCodes.ResourceExhausted,
		httpCode: http.StatusTeapot,
		message:  "fake_message",
		tag:      "DAPR_FAKE_TAG",
		details: []proto.Message{
			details1,
			details2,
		},
	}

	kitErr := &Error{
		grpcCode: grpcCodes.ResourceExhausted,
		httpCode: http.StatusTeapot,
		message:  "fake_message",
		tag:      "DAPR_FAKE_TAG",
	}

	kitErr.AddDetails(details1, details2)
	assert.Equal(t, expected, *kitErr)
}

// Ensure Err format does not break users expecting this format
func TestError_Error(t *testing.T) {
	type fields struct {
		message  string
		grpcCode grpcCodes.Code
	}
	tests := []struct {
		name    string
		builder *ErrorBuilder
		fields  fields
		want    string
	}{
		{
			name: "Has_GrpcCode_And_Message",
			builder: NewBuilder(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{
				message:  "Msg",
				grpcCode: grpcCodes.ResourceExhausted,
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.ResourceExhausted, "Msg"),
		},
		{
			name: "Has_Only_Message",
			builder: NewBuilder(
				grpcCodes.OK,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{
				message: "Msg",
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.OK, "Msg"),
		},
		{
			name: "Has_Only_GrpcCode",
			builder: NewBuilder(
				grpcCodes.Canceled,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{
				grpcCode: grpcCodes.Canceled,
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.Canceled, "Msg"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := tt.builder.Build()
			if got := kitErr.Error(); got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}

			err, ok := kitErr.(Error)
			require.True(t, ok, err.Is(kitErr))
		})
	}
}

func TestErrorBuilder_WithErrorInfo(t *testing.T) {
	reason := "fake"
	metadata := map[string]string{"fake": "test"}
	details := &errdetails.ErrorInfo{
		Domain:   Domain,
		Reason:   reason,
		Metadata: metadata,
	}

	expected := Error{
		grpcCode: grpcCodes.ResourceExhausted,
		httpCode: http.StatusTeapot,
		message:  "fake_message",
		tag:      "DAPR_FAKE_TAG",
		details: []proto.Message{
			details,
		},
	}

	builder := NewBuilder(
		grpcCodes.ResourceExhausted,
		http.StatusTeapot,
		"fake_message",
		"DAPR_FAKE_TAG",
	).
		WithErrorInfo(reason, metadata)

	assert.Equal(t, expected, builder.Build())
}

// helperSlicesEqual compares slices element by element
func helperSlicesEqual(a, b []proto.Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func TestErrorBuilder_WithDetails(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		tag      string
	}

	type args struct {
		a []proto.Message
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   Error
	}{
		{
			name: "Has_Multiple_Details",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{
				&errdetails.ErrorInfo{
					Domain:   Domain,
					Reason:   "example_reason",
					Metadata: map[string]string{"key": "value"},
				},
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			}},
			want: Error{
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   Domain,
						Reason:   "example_reason",
						Metadata: map[string]string{"key": "value"},
					},
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := NewBuilder(
				test.fields.grpcCode,
				test.fields.httpCode,
				test.fields.message,
				test.fields.tag,
			).WithDetails(test.args.a...)

			assert.Equal(t, test.want, kitErr.Build())
		})
	}
}

func TestWithErrorHelp(t *testing.T) {
	// Initialize the Error struct with some default values
	err := NewBuilder(grpcCodes.InvalidArgument, http.StatusBadRequest, "Internal error", "INTERNAL_ERROR")

	// Define test data for the help links
	links := []*errdetails.Help_Link{
		{
			Description: "Link 1 Description",
			Url:         "http://example.com/1",
		},
		{
			Description: "Link 2 Description",
			Url:         "http://example.com/2",
		},
	}

	// Call WithHelp
	err = err.WithHelp(links)
	// Use require to make assertions
	require.Len(t, err.err.details, 1, "Details should contain exactly one item")

	// Type assert to *errdetails.Help
	helpDetail, ok := err.err.details[0].(*errdetails.Help)
	require.True(t, ok, "Details[0] should be of type *errdetails.Help")
	require.Equal(t, links, helpDetail.GetLinks(), "Links should match the provided links")
}

func TestWithErrorFieldViolation(t *testing.T) {
	// Initialize the Error struct with some default values
	err := NewBuilder(grpcCodes.InvalidArgument, http.StatusBadRequest, "Internal error", "INTERNAL_ERROR")

	// Define test data for the field violation
	fieldName := "testField"
	msg := "test message"

	// Call WithFieldViolation
	updatedErr := err.WithFieldViolation(fieldName, msg)

	// Check if the Details slice contains the expected BadRequest
	require.Len(t, updatedErr.err.details, 1)

	// Type assert to *errdetails.BadRequest
	br, ok := updatedErr.err.details[0].(*errdetails.BadRequest)
	require.True(t, ok, "Expected BadRequest type, got %T", updatedErr.err.details[0])

	// Check if the BadRequest contains the expected FieldViolation
	require.Len(t, br.GetFieldViolations(), 1, "Expected 1 field violation, got %d", len(br.GetFieldViolations()))
	require.Equal(t, fieldName, br.GetFieldViolations()[0].GetField(), "Expected field name %s, got %s", fieldName, br.GetFieldViolations()[0].GetField())
	require.Equal(t, msg, br.GetFieldViolations()[0].GetDescription(), "Expected description %s, got %s", msg, br.GetFieldViolations()[0].GetDescription())
}

func TestError_JSONErrorValue(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		tag      string
	}

	tests := []struct {
		name   string
		fields fields
		want   []byte
	}{
		{
			name: "No_Details",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message"}`),
		},
		{
			name: "With_Multiple_Details",
			fields: fields{
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   Domain,
						Reason:   "test_reason",
						Metadata: map[string]string{"key": "value"},
					},
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"dapr.io","reason":"test_reason","metadata":{"key":"value"}},{"@type":"type.googleapis.com/google.rpc.PreconditionFailure.Violation","type":"TOS","subject":"google.com/cloud","description":"test_description"}]}`),
		},
		{
			name: "ErrorInfo",
			fields: fields{
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   Domain,
						Reason:   "test_reason",
						Metadata: map[string]string{"key": "value"},
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"dapr.io","reason":"test_reason","metadata":{"key":"value"}}]}`),
		},
		{
			name: "RetryInfo",
			fields: fields{
				details: []proto.Message{
					&errdetails.RetryInfo{
						RetryDelay: &durationpb.Duration{
							Seconds: 2,
							Nanos:   0,
						},
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retry_delay":"2s"}]}`),
		},
		{
			name: "DebugInfo",
			fields: fields{
				details: []proto.Message{
					&errdetails.DebugInfo{
						StackEntries: []string{
							"stack_entry_1",
							"stack_entry_2",
						},
						Detail: "debug_details",
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.DebugInfo","stack_entries":["stack_entry_1","stack_entry_2"],"detail":"debug_details"}]}`),
		},
		{
			name: "QuotaFailure",
			fields: fields{
				details: []proto.Message{
					&errdetails.QuotaFailure{
						Violations: []*errdetails.QuotaFailure_Violation{
							{
								Subject:     "quota_subject_1",
								Description: "quota_description_1",
							},
						},
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"subject":"quota_subject_1","description":"quota_description_1"}]}]}`),
		},
		{
			name: "PreconditionFailure",
			fields: fields{
				details: []proto.Message{
					&errdetails.PreconditionFailure{
						Violations: []*errdetails.PreconditionFailure_Violation{
							{
								Type:        "precondition_type_1",
								Subject:     "precondition_subject_1",
								Description: "precondition_description_1",
							},
						},
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.PreconditionFailure","violations":[{"type":"precondition_type_1","subject":"precondition_subject_1","description":"precondition_description_1"}]}]}`),
		},
		{
			name: "BadRequest",
			fields: fields{
				details: []proto.Message{
					&errdetails.BadRequest{
						FieldViolations: []*errdetails.BadRequest_FieldViolation{
							{
								Field:       "field_1",
								Description: "field_description_1",
							},
						},
					},
				},
				grpcCode: grpcCodes.InvalidArgument,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","field_violations":[{"field":"field_1","description":"field_description_1"}]}]}`),
		},
		{
			name: "RequestInfo",
			fields: fields{
				details: []proto.Message{
					&errdetails.RequestInfo{
						RequestId:   "request_id_1",
						ServingData: "serving_data_1",
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.RequestInfo","request_id":"request_id_1","serving_data":"serving_data_1"}]}`),
		},
		{
			name: "ResourceInfo",
			fields: fields{
				details: []proto.Message{
					&errdetails.ResourceInfo{
						ResourceType: "resource_type_1",
						ResourceName: "resource_name_1",
						Owner:        "owner_1",
						Description:  "description_1",
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.ResourceInfo","resource_type":"resource_type_1","resource_name":"resource_name_1","owner":"owner_1","description":"description_1"}]}`),
		},
		{
			name: "Help",
			fields: fields{
				details: []proto.Message{
					&errdetails.Help{
						Links: []*errdetails.Help_Link{
							{
								Description: "description_1",
								Url:         "dapr_url_1",
							},
						},
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.Help","links":[{"description":"description_1","url":"dapr_url_1"}]}]}`),
		},
		{
			name: "LocalizedMessage",
			fields: fields{
				details: []proto.Message{
					&errdetails.LocalizedMessage{
						Locale:  "en-US",
						Message: "fake_localized_message",
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.LocalizedMessage","locale":"en-US","message":"fake_localized_message"}]}`),
		},
		{
			name: "QuotaFailure_Violation",
			fields: fields{
				details: []proto.Message{
					&errdetails.QuotaFailure_Violation{
						Subject:     "test_subject",
						Description: "test_description",
					},
				},
				grpcCode: grpcCodes.FailedPrecondition,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure.Violation","subject":"test_subject","description":"test_description"}]}`),
		},
		{
			name: "PreconditionFailure_Violation",
			fields: fields{
				details: []proto.Message{
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.PreconditionFailure.Violation","type":"TOS","subject":"google.com/cloud","description":"test_description"}]}`),
		},
		{
			name: "BadRequest_FieldViolation",
			fields: fields{
				details: []proto.Message{
					&errdetails.BadRequest_FieldViolation{
						Field:       "test_field",
						Description: "test_description",
					},
				},
				grpcCode: grpcCodes.InvalidArgument,
				httpCode: http.StatusBadRequest,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.BadRequest.FieldViolation","field":"test_field","description":"test_description"}]}`),
		},
		{
			name: "Help_Link",
			fields: fields{
				details: []proto.Message{
					&errdetails.Help_Link{
						Description: "test_description",
						Url:         "https://docs.dapr.io/",
					},
				},
				grpcCode: grpcCodes.InvalidArgument,
				httpCode: http.StatusBadRequest,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"@type":"type.googleapis.com/google.rpc.Help.Link","description":"test_description","url":"https://docs.dapr.io/"}]}`),
		},
		{
			name: "Unknown_Detail_Type",
			fields: fields{
				details: []proto.Message{
					&context.AuditContext{
						TargetResource: "target_1",
					},
				},
				grpcCode: grpcCodes.Internal,
				httpCode: http.StatusInternalServerError,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"details":[{"unknownDetailType":"*context.AuditContext","unknownDetails":"\u0026context.AuditContext{state:impl.MessageState{NoUnkeyedLiterals:pragma.NoUnkeyedLiterals{}, DoNotCompare:pragma.DoNotCompare{}, DoNotCopy:pragma.DoNotCopy{}, atomicMessageInfo:(*impl.MessageInfo)(0x14000156b00)}, sizeCache:10, unknownFields:[]uint8(nil), AuditLog:[]uint8(nil), ScrubbedRequest:(*structpb.Struct)(nil), ScrubbedResponse:(*structpb.Struct)(nil), ScrubbedResponseItemCount:0, TargetResource:\"target_1\"}"}],"errorCode":"DAPR_FAKE_TAG","message":"fake_message"}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := NewBuilder(test.fields.grpcCode, test.fields.httpCode, test.fields.message, test.fields.tag).
				WithDetails(test.fields.details...)

			got := kitErr.err.JSONErrorValue()

			// Use map[string]interface{} to handle order diff in the slices
			var gotMap, wantMap map[string]interface{}
			_ = json.Unmarshal(got, &gotMap)
			_ = json.Unmarshal(test.want, &wantMap)

			// Compare only the errorCode field
			gotErrorCode, gotErrorCodeOK := gotMap["errorCode"].(string)
			wantErrorCode, wantErrorCodeOK := wantMap["errorCode"].(string)

			if gotErrorCodeOK && wantErrorCodeOK && gotErrorCode != wantErrorCode {
				t.Errorf("errorCode: \ngot = %s, \nwant %s", got, test.want)
			}

			// Compare only the message field
			gotMsg, gotMsgOK := gotMap["message"].(string)
			wantMsg, wantMsgOK := wantMap["message"].(string)

			if gotMsgOK && wantMsgOK && gotMsg != wantMsg {
				t.Errorf("message: \ngot = %s, \nwant %s", got, test.want)
			}

			// Compare only the tag field
			gotTag, gotTagOK := gotMap["tag"].(string)
			wantTag, wantTagOK := wantMap["tag"].(string)

			if gotTagOK && wantTagOK && gotTag != wantTag {
				t.Errorf("tag: \ngot = %s, \nwant %s", got, test.want)
			}

			if !helperSlicesEqual(kitErr.err.details, test.fields.details) {
				t.Errorf("Error.JSONErrorValue(): \ngot %s, \nwant %s", got, test.want)
			}
		})
	}
}

func TestError_GRPCStatus(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		tag      string
	}

	tests := []struct {
		name   string
		fields fields
		want   *status.Status
	}{
		{
			name: "No_Details",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: status.New(grpcCodes.ResourceExhausted, "fake_message"),
		},
		{
			name: "With_Details",
			fields: fields{
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   Domain,
						Reason:   "FAKE_REASON",
						Metadata: map[string]string{"key": "value"},
					},
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			want: func() *status.Status {
				s, _ := status.New(grpcCodes.ResourceExhausted, "fake_message").
					WithDetails(
						&errdetails.ErrorInfo{
							Domain:   Domain,
							Reason:   "FAKE_REASON",
							Metadata: map[string]string{"key": "value"},
						},
						&errdetails.PreconditionFailure_Violation{
							Type:        "TOS",
							Subject:     "google.com/cloud",
							Description: "test_description",
						},
					)
				return s
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := NewBuilder(
				test.fields.grpcCode,
				test.fields.httpCode,
				test.fields.message,
				test.fields.tag,
			).WithDetails(test.fields.details...)

			got := kitErr.err.GRPCStatus()

			if !reflect.DeepEqual(got.Proto(), test.want.Proto()) {
				t.Errorf("Error.GRPCStatus(): \ngot = %v, \nwant %v", got.Proto(), test.want.Proto())
			}
		})
	}
}

func TestErrorBuilder_Build(t *testing.T) {
	t.Run("With_ErrorInfo", func(t *testing.T) {
		built := NewBuilder(
			grpcCodes.ResourceExhausted,
			http.StatusTeapot,
			"Test Msg",
			"SOME_ERROR",
		).WithErrorInfo("fake", map[string]string{"fake": "test"}).Build()

		builtErr, ok := built.(Error)
		require.True(t, ok)

		containsErrorInfo := false

		for _, detail := range builtErr.details {
			_, isErrInfo := detail.(*errdetails.ErrorInfo)
			if isErrInfo {
				containsErrorInfo = true
				break
			}
		}

		assert.True(t, containsErrorInfo)
	})

	t.Run("Without_ErrorInfo", func(t *testing.T) {
		builder := NewBuilder(
			grpcCodes.ResourceExhausted,
			http.StatusTeapot,
			"Test Msg",
			"SOME_ERROR",
		)

		assert.PanicsWithValue(t, "Must include ErrorInfo in error details.", func() {
			_ = builder.Build()
		})
	})
}

// This test ensures that all the error details google provides are covered in our switch case
// in errors.go. If google adds an error detail, this test should fail, and we should add
// that specific error detail to the switch case
func TestEnsureAllErrDetailsCovered(t *testing.T) {
	packagePath := "google.golang.org/genproto/googleapis/rpc/errdetails"

	// Load the package
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, packagePath)
	if err != nil {
		t.Error(err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		t.Errorf("ensure package is correct: %v", packages.ListError)
	}

	// This is hard-coded from the switch statement in errors.go to ensure we stay up
	// to date on the error types we support, and to ensure we update our supported
	// error types when google adds to their error details
	mySwitchTypes := []string{
		"ErrorInfo",
		"RetryInfo",
		"DebugInfo",
		"QuotaFailure",
		"PreconditionFailure",
		"PreconditionFailure_Violation",
		"BadRequest",
		"BadRequest_FieldViolation",
		"RequestInfo",
		"ResourceInfo",
		"Help",
		"LocalizedMessage",
		"QuotaFailure_Violation",
		"Help_Link",
	}

	coveredTypes := make(map[string]bool)

	// Iterate through the types in googles error detail package
	for _, name := range pkgs[0].Types.Scope().Names() {
		obj := pkgs[0].Types.Scope().Lookup(name)
		if typ, ok := obj.Type().(*types.Named); ok {
			typeFullName := typ.Obj().Name()

			// Check if the type is covered in errors.go switch cases
			if containsType(mySwitchTypes, typ.Obj().Name()) {
				coveredTypes[typeFullName] = true
			} else {
				coveredTypes[typeFullName] = false
			}
		}
	}

	// Check if there are any uncovered types
	for typeName, covered := range coveredTypes {
		// Skip "FileDescriptor" && "Once" since those aren't types we care about
		if !covered && typeName != "FileDescriptor" && typeName != "Once" {
			t.Errorf("Type %s is not handled in switch cases, please update the switch case in errors.go",
				typeName)
		}
	}
}

// containsType checks if the slice of types contains a specific type
func containsType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
