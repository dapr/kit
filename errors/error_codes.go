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
	// Generic
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeNotConfigured = "NOT_CONFIGURED"
	ErrCodeNotSupported  = "NOT_SUPPORTED"
	ErrCodeIllegalKey    = "ILLEGAL_KEY"

	// Components
	ErrCodeStateStore         = "DAPR_STATE_"
	ErrCodePubSub             = "DAPR_PUBSUB_"
	ErrCodeBindings           = "DAPR_BINDING_"
	ErrCodeSecretStore        = "DAPR_SECRET_"
	ErrCodeConfigurationStore = "DAPR_CONFIGURATION_"
	ErrCodeLock               = "DAPR_LOCK_"
	ErrCodeNameResolution     = "DAPR_NAME_RESOLUTION_"
	ErrCodeMiddleware         = "DAPR_MIDDLEWARE_"

	// State
	ErrCodeGetStateFailed      = "GET_STATE_FAILED"
	ErrCodeTooManyTransactions = "TOO_MANY_TRANSACTIONS"
	ErrCodeQueryFailed         = "QUERY_FAILED"
)
