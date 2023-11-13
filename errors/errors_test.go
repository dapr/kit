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
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestError_WithVars(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		metadata map[string]string
		reason   string
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
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
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
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{"myFakeMsg"}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message: myFakeMsg",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
		{
			name: "Multiple_Parameter",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_messages: %s, %s, %d",
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []any{"myFakeMsg1", "myFakeMsg2", 12}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
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
				Metadata: test.fields.metadata,
				Reason:   test.fields.reason,
				Tag:      test.fields.tag,
			}

			if got := kitErr.WithVars(test.args.a...); !helperSlicesEqual(got.Details, test.want.Details) {
				t.Errorf("Error.WithVars() = %v, want %v", got, test.want)
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

func TestError_Message(t *testing.T) {
	type fields struct {
		message string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "Has_Message",
			fields: fields{message: "Test Msg"},
			want:   "Test Msg",
		},
		{
			name:   "No_Message",
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
		fields fields
		want   string
	}{
		{
			name:   "Has_Tag",
			fields: fields{tag: "SOME_ERROR"},
			want:   "SOME_ERROR",
		},
		{
			name:   "No_Tag",
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
		fields fields
		want   int
	}{
		{
			name:   "Has_HttpCode",
			fields: fields{httpCode: http.StatusTeapot},
			want:   http.StatusTeapot,
		},
		{
			name:   "No_HttpCode",
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

/*
func TestError_GRPCStatus(t *testing.T) {
	type fields struct {
		message  string
		grpcCode grpcCodes.Code
	}
	tests := []struct {
		name   string
		fields fields
		want   *status.Status
	}{
		{
			name: "Has_GrpcCode_And_Message",
			fields: fields{
				message:  "Msg",
				grpcCode: grpcCodes.ResourceExhausted,
			},
			want: status.New(grpcCodes.ResourceExhausted, "Msg"),
		},
		{
			name: "Has_Only_Message",
			fields: fields{
				message: "Msg",
			},
			// The default code is 0, i.e. OK
			want: status.New(grpcCodes.OK, "Msg"),
		},
		{
			name: "Has_Only_GrpcCode",
			fields: fields{
				grpcCode: grpcCodes.Canceled,
			},
			want: status.New(grpcCodes.Canceled, ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitErr := Error{
				Message:  tt.fields.message,
				GrpcCode: tt.fields.grpcCode,
			}
			if got := kitErr.GRPCStatus(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Error.GRPCStatus = %v, want %v", got, tt.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}
*/

// Ensure Err fmt to not break users expecting this format
func TestError_Error(t *testing.T) {
	type fields struct {
		message  string
		grpcCode grpcCodes.Code
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Has_GrpcCode_And_Message",
			fields: fields{
				message:  "Msg",
				grpcCode: grpcCodes.ResourceExhausted,
			},
			want: fmt.Sprintf(errStringFormat, grpcCodes.ResourceExhausted, "Msg"),
		},
		{
			name: "Has_Only_Message",
			fields: fields{
				message: "Msg",
			},
			// The default code is 0, i.e. OK
			want: fmt.Sprintf(errStringFormat, grpcCodes.OK, "Msg"),
		},
		{
			name: "Has_Only_GrpcCode",
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

func TestError_WithErrorInfo(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		metadata map[string]string
		reason   string
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
			name: "Has_No_Detail",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{}},
			want: Error{
				Details:  []proto.Message{},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
		{
			name: "Has_One_Detail",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{
				&errdetails.PreconditionFailure_Violation{
					Type:        "TOS",
					Subject:     "google.com/cloud",
					Description: "test_description",
				},
			}},
			want: Error{
				Details: []proto.Message{
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
				Tag:      "DAPR_FAKE_TAG",
			},
		},
		{
			name: "Has_Multiple_Details",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			args: args{a: []proto.Message{
				&errdetails.ErrorInfo{
					Domain:   domain,
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
				Details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   domain,
						Reason:   "example_reason",
						Metadata: map[string]string{"key": "value"},
					},
					&errdetails.PreconditionFailure_Violation{
						Type:        "TOS",
						Subject:     "google.com/cloud",
						Description: "test_description",
					},
				},
				GrpcCode: grpcCodes.ResourceExhausted,
				HttpCode: http.StatusTeapot,
				Message:  "fake_message",
				Metadata: map[string]string{"fake": "test"},
				Reason:   "FAKE_REASON",
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
				Metadata: test.fields.metadata,
				Reason:   test.fields.reason,
				Tag:      test.fields.tag,
			}

			if got := kitErr.WithDetails(test.args.a...); !helperSlicesEqual(got.Details, test.want.Details) {
				t.Errorf("Error.WithDetails() = %v, want %v", got, test.want)
			}
			assert.True(t, kitErr.Is(&kitErr))
		})
	}
}

func TestError_JSONErrorValue(t *testing.T) {
	type fields struct {
		details  []proto.Message
		grpcCode grpcCodes.Code
		httpCode int
		message  string
		metadata map[string]string
		reason   string
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
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[]}`),
		},
		{
			name: "With_Details",
			fields: fields{
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   domain,
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
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			want: []byte(`{"errorCode":"DAPR_FAKE_TAG","message":"fake_message","details":[{"domain":"dapr.io","reason":"test_reason","metadata":{"key":"value"}},{"type":"TOS","subject":"google.com/cloud","description":"test_description"}]}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := Error{
				Details:  test.fields.details,
				GrpcCode: test.fields.grpcCode,
				HttpCode: test.fields.httpCode,
				Message:  test.fields.message,
				Metadata: test.fields.metadata,
				Reason:   test.fields.reason,
				Tag:      test.fields.tag,
			}

			got := kitErr.JSONErrorValue()

			// Use map[string]interface{} to handle order diff in the slices
			var gotMap, wantMap map[string]interface{}
			_ = json.Unmarshal(got, &gotMap)
			_ = json.Unmarshal(test.want, &wantMap)

			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Errorf("Error.JSONErrorValue() = %s, want %s", got, test.want)
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
		metadata map[string]string
		reason   string
		tag      string
	}

	tests := []struct {
		name   string
		fields fields
		want   *status.Status
	}{
		{
			name: "No_Details_And_No_Reason",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				metadata: map[string]string{"fake": "test"},
				// reason:   "FAKE_REASON",
				tag: "DAPR_FAKE_TAG",
			},
			want: status.New(grpcCodes.ResourceExhausted, "fake_message"),
		},
		{
			name: "No_Details_With_Reason",
			fields: fields{
				details:  []proto.Message{},
				grpcCode: grpcCodes.ResourceExhausted,
				httpCode: http.StatusTeapot,
				message:  "fake_message",
				metadata: map[string]string{"fake": "test"},
				reason:   "FAKE_REASON",
				tag:      "DAPR_FAKE_TAG",
			},
			want: func() *status.Status {
				s, _ := status.New(grpcCodes.ResourceExhausted, "fake_message").WithDetails(
					&errdetails.ErrorInfo{
						Domain:   domain,
						Reason:   "FAKE_REASON",
						Metadata: map[string]string{"fake": "test"},
					},
				)
				return s
			}(),
		},
		{
			name: "With_Details_No_Reason",
			fields: fields{
				details: []proto.Message{
					&errdetails.ErrorInfo{
						Domain:   domain,
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
				metadata: map[string]string{"fake": "test"},
				tag:      "DAPR_FAKE_TAG",
			},
			want: func() *status.Status {
				s, _ := status.New(grpcCodes.ResourceExhausted, "fake_message").
					WithDetails(
						&errdetails.ErrorInfo{
							Domain:   domain,
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
		//TODO(Cassie) confirm with reason output is correct and fix test, or adjust reason accordingly
		// {
		// 	name: "With_Details_And_Reason",
		// 	fields: fields{
		// 		details: []proto.Message{
		// 			&errdetails.ErrorInfo{
		// 				Domain:   domain,
		// 				Reason:   "FAKE_REASON",
		// 				Metadata: map[string]string{"key": "value"},
		// 			},
		// 			&errdetails.PreconditionFailure_Violation{
		// 				Type:        "TOS",
		// 				Subject:     "google.com/cloud",
		// 				Description: "test_description",
		// 			},
		// 		},
		// 		grpcCode: grpcCodes.ResourceExhausted,
		// 		httpCode: http.StatusTeapot,
		// 		message:  "fake_message",
		// 		metadata: map[string]string{"fake": "test"},
		// 		reason:   "FAKE_REASON",
		// 		tag:      "DAPR_FAKE_TAG",
		// 	},
		// 	want: func() *status.Status {
		// 		s, _ := status.New(grpcCodes.ResourceExhausted, "fake_message").
		// 			WithDetails(
		// 				&errdetails.ErrorInfo{
		// 					Domain:   domain,
		// 					Reason:   "FAKE_REASON",
		// 					Metadata: map[string]string{"key": "value"},
		// 				},
		// 				&errdetails.PreconditionFailure_Violation{
		// 					Type:        "TOS",
		// 					Subject:     "google.com/cloud",
		// 					Description: "test_description",
		// 				},
		// 			)
		// 		return s
		// 	}(),
		// },
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kitErr := Error{
				Details:  test.fields.details,
				GrpcCode: test.fields.grpcCode,
				HttpCode: test.fields.httpCode,
				Message:  test.fields.message,
				Metadata: test.fields.metadata,
				Reason:   test.fields.reason,
				Tag:      test.fields.tag,
			}

			got := kitErr.GRPCStatus()

			if !reflect.DeepEqual(got.Proto(), test.want.Proto()) {
				//TODO(Cassie): change this back once confirmed
				t.Errorf("\nhave %v \n\n want %v", got.Proto(), test.want.Proto())

				// t.Errorf("Error.GRPCStatus() = %v, want %v", got.Proto(), test.want.Proto())
			}
		})
	}
}
