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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/context"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

/*
//TODO confirm - do we still want this functionality: WithVars?
*/
func TestError_WithVars(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		tag      string
	}

	type args struct {
		a []any
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   Error
	}{
		{
			name: "No_Formatting",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
		{
			name: "String_Parameter",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message: %s",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{"myFakeMsg"}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message: myFakeMsg",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
		{
			name: "Multiple_Parameters",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_messages: %s, %s, %d",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{"myFakeMsg1", "myFakeMsg2", 12}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_messages: myFakeMsg1, myFakeMsg2, 12",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := Error{
				Details:  test.fields.details,
				GrpcCode: test.fields.grpcCode,
				HttpCode: test.fields.httpCode,
				Message:  test.fields.message,
				Tag:      test.fields.tag,
			}

			if got := kitErr.WithVars(test.args.a...); !reflect.DeepEqual(got, &test.want) {
				t.Errorf("Error.WithVars() = %v, want %v\n", got, test.want)
			}

			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}

func TestError_Message(t *testing.T) {
	type fields struct {
		message string
	}
	tests := []struct {
		name   string
		err    *Error
		fields fields
		want   string
	}{
		{
			name: "Has_Message",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"Test Msg",
				"DAPR_FAKE_TAG",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{message: "Test Msg"},
			want:   "Test Msg",
		},
		{
			name: "No_Message",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"",
				"DAPR_FAKE_TAG",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{message: ""},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := Error{
				Message: tt.fields.message,
			}
			if got := kitErr.Message; got != tt.want {
				t.Errorf("Error.Message = %v, want %v", got, tt.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}

func TestError_Tag(t *testing.T) {
	type fields struct {
		tag string
	}
	tests := []struct {
		name   string
		err    *Error
		fields fields
		want   string
	}{
		{
			name: "Has_Tag",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"Test Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{tag: "SOME_ERROR"},
			want:   "SOME_ERROR",
		},
		{
			name: "No_Tag",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"",
				"",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{tag: ""},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := Error{
				Tag: tt.fields.tag,
			}
			if got := kitErr.Tag; got != tt.want {
				t.Errorf("Error.Tag = %v, want %v", got, tt.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}

func TestError_HttpCode(t *testing.T) {
	type fields struct {
		httpCode int
	}
	tests := []struct {
		name   string
		err    *Error
		fields fields
		want   int
	}{
		{
			name: "Has_HttpCode",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"Test Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{httpCode: http.StatusTeapot},
			want:   http.StatusTeapot,
		},
		{
			name: "No_HttpCode",
			err: New(
				grpcCodes.ResourceExhausted,
				0,
				"",
				"",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{},
			want:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := Error{
				HttpCode: tt.fields.httpCode,
			}
			if got := kitErr.HttpCode; got != tt.want {
				t.Errorf("Error.HttpCode = %v, want %v", got, tt.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}

// Ensure Err format does not break users expecting this format
func TestError_Error(t *testing.T) {
	type fields struct {
		message  string
		grpcCode grpcCodes.Code
	}
	tests := []struct {
		name   string
		err    *Error
		fields fields
		want   string
	}{
		{
			name: "Has_GrpcCode_And_Message",
			err: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			),
			fields: fields{
				message:  "Msg",
				grpcCode: grpcCodes.ResourceExhausted,
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.ResourceExhausted, "Msg"),
		},
		{
			name: "Has_Only_Message",
			err: New(
				grpcCodes.OK,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			),
			fields: fields{
				message: "Msg",
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.OK, "Msg"),
		},
		{
			name: "Has_Only_GrpcCode",
			err: New(
				grpcCodes.Canceled,
				http.StatusTeapot,
				"Msg",
				"SOME_ERROR",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
			fields: fields{
				grpcCode: grpcCodes.Canceled,
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.Canceled, ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := Error{
				Message:  tt.fields.message,
				GrpcCode: tt.fields.grpcCode,
			}
			if got := kitErr.Error(); got != tt.want {
				t.Errorf("Error.Error() = %v, want %v", got, tt.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
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

func TestError_WithErrorInfo(t *testing.T) {
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
		want   *Error
	}{
		{
			name: "Has_No_Detail",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{}},
			want: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"fake_message",
				"DAPR_FAKE_TAG",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}),
		},
		{
			name: "Has_One_Detail",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			}},
			want: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"fake_message",
				"DAPR_FAKE_TAG",
			).WithErrorInfo("fake", map[string]string{"fake": "test"}).WithDetails(
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := New(
				test.fields.grpcCode,
				test.fields.httpCode,
				test.fields.message,
				test.fields.tag,
			).WithErrorInfo("fake", map[string]string{"fake": "test"})

			if got := kitErr.WithDetails(test.args.a...); !helperSlicesEqual(got.Details, test.want.Details) {
				t.Errorf("Error.WithDetails() = %v, want %v", got, test.want)
			}
			assert.True(t, kitErr.Is(kitErr))
		})
	}
}

func TestError_WithDetails(t *testing.T) {
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
		want   *Error
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
					Domain:   ErrMsgDomain,
					Reason:   "example_reason",
					Metadata: map[string]string{"key": "value"},
				},
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			}},
			want: New(
				grpcCodes.ResourceExhausted,
				http.StatusTeapot,
				"fake_message",
				"DAPR_FAKE_TAG",
			).WithErrorInfo("example_reason", map[string]string{"key": "value"}).WithDetails(
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := New(
				test.fields.grpcCode,
				test.fields.httpCode,
				test.fields.message,
				test.fields.tag,
			)

			if got := kitErr.WithDetails(test.args.a...); !helperSlicesEqual(got.Details, test.want.Details) {
				t.Errorf("Error.WithDetails() = %v, want %v", got, test.want)
			}
			assert.True(t, kitErr.Is(kitErr))
		})
	}
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
						Domain:   ErrMsgDomain,
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
						Domain:   ErrMsgDomain,
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
			kitErr := New(test.fields.grpcCode, test.fields.httpCode, test.fields.message, test.fields.tag).
				WithDetails(test.fields.details...)

			got := kitErr.JSONErrorValue()

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

			if !helperSlicesEqual(kitErr.Details, test.fields.details) {
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
		reason   string
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
						Domain:   ErrMsgDomain,
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
							Domain:   ErrMsgDomain,
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
			kitErr := New(
				test.fields.grpcCode,
				test.fields.httpCode,
				test.fields.message,
				test.fields.tag,
			).WithDetails(test.fields.details...)

			got := kitErr.GRPCStatus()

			if !reflect.DeepEqual(got.Proto(), test.want.Proto()) {
				t.Errorf("Error.GRPCStatus(): \ngot = %v, \nwant %v", got.Proto(), test.want.Proto())
			}
		})
	}
}
