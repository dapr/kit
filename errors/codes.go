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
	CodeNotFound      = "NOT_FOUND"
	CodeNotConfigured = "NOT_CONFIGURED"
	CodeNotSupported  = "NOT_SUPPORTED"
	CodeIllegalKey    = "ILLEGAL_KEY"

	// Components
	CodePrefixStateStore         = "DAPR_STATE_"
	CodePrefixPubSub             = "DAPR_PUBSUB_"
	CodePrefixBindings           = "DAPR_BINDING_"
	CodePrefixSecretStore        = "DAPR_SECRET_"
	CodePrefixConfigurationStore = "DAPR_CONFIGURATION_"
	CodePrefixLock               = "DAPR_LOCK_"
	CodePrefixNameResolution     = "DAPR_NAME_RESOLUTION_"
	CodePrefixMiddleware         = "DAPR_MIDDLEWARE_"
	CodePrefixCryptography       = "DAPR_CRYPTOGRAPHY_"
	CodePrefixPlacement          = "DAPR_PLACEMENT_"

	// State
	CodePostfixGetStateFailed      = "GET_STATE_FAILED"
	CodePostfixTooManyTransactions = "TOO_MANY_TRANSACTIONS"
	CodePostfixQueryFailed         = "QUERY_FAILED"
)
