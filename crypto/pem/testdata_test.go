/*
Copyright 2022 The Dapr Authors
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

package pem

const (
	// good for 30 years
	selfSignedRootCert = `-----BEGIN CERTIFICATE-----
MIIBYjCCAQegAwIBAgIRAKTEJxGnjLLxJHupLBXWs4EwCgYIKoZIzj0EAwIwDzEN
MAsGA1UEAxMEcm9vdDAgFw0yNTA4MjcxMDAzNDFaGA8yMDU1MDgyMDEwMDM0MVow
DzENMAsGA1UEAxMEcm9vdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFK+gysd
ykg3PGGeMxupM+I/14VAQzHyUiBY5gCb/TwLPbsGVmr+IQhcVv9qrEUntBkURtsR
QIvpxgo1vdcDdrKjQjBAMA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBR8yVxFTE2lUXehLqNPNPPb7aAxmzAKBggqhkjOPQQDAgNJADBG
AiEAnLIvHFK/8tg1+A5GmqBAga4CsgnBsBlaYE0nWGxhULACIQDNG7+1ibiKui7y
asNeuhl1GHo6ODBdh/8jPYtdwu9+DA==
-----END CERTIFICATE-----
` // #nosec G101
)
