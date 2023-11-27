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

const (
	ErrMsgDomain = "dapr.io"

	errStringFormat = "api error: code = %s desc = %s"

	// MESSAGES
	ErrMsgStateGet        = "fail to get %s from state store %s: %s"
	ErrMsgStateDelete     = "failed deleting state with key %s: %s"
	ErrMsgStateSave       = "failed saving state in state store %s: %s"
	ErrMsgStateDeleteBulk = "failed deleting state in state store %s: %s"

	// StateTransaction.
	ErrMsgStateTransactionsNotSupported = "state store %s doesn't support transaction"
	ErrMsgStateOperationNotSupported    = "operation type %s not supported"
	ErrMsgStateTransaction              = "error while executing state transaction: %s"
)
