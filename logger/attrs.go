/*
Copyright 2026 The Dapr Authors
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

package logger

import "log/slog"

// Field names shared across Dapr components. Logging the same concept under
// the same key everywhere is the point of the exercise: a message with the
// value interpolated into it cannot be filtered on, and neither can one where
// half the codebase says "appID" and the other half says "app_id".
const (
	FieldError      = "error"
	FieldAppID      = logFieldAppID
	FieldComponent  = "component"
	FieldNamespace  = "namespace"
	FieldActorType  = "actor_type"
	FieldActorID    = "actor_id"
	FieldPubsub     = "pubsub"
	FieldTopic      = "topic"
	FieldOperation  = "operation"
	FieldStore      = "store"
	FieldMethod     = "method"
	FieldInstanceID = "instance_id"
)

// Err returns an attribute carrying an error.
//
// Prefer this over interpolating an error into the message. It replaces the
// trailing ": %v" that most error logging in Dapr ends with, and gives one
// place to enrich error reporting later (status codes, unwrapping) without
// touching call sites.
//
// A nil error yields an empty attribute, which is dropped rather than emitted
// as "error=<nil>".
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	return slog.String(FieldError, err.Error())
}

// AppID returns an app_id attribute.
func AppID(id string) slog.Attr { return slog.String(FieldAppID, id) }

// Component returns a component attribute.
func Component(name string) slog.Attr { return slog.String(FieldComponent, name) }

// Namespace returns a namespace attribute.
func Namespace(ns string) slog.Attr { return slog.String(FieldNamespace, ns) }

// ActorType returns an actor_type attribute.
func ActorType(t string) slog.Attr { return slog.String(FieldActorType, t) }

// ActorID returns an actor_id attribute.
func ActorID(id string) slog.Attr { return slog.String(FieldActorID, id) }

// Pubsub returns a pubsub attribute naming the pub/sub component.
func Pubsub(name string) slog.Attr { return slog.String(FieldPubsub, name) }

// Topic returns a topic attribute.
func Topic(t string) slog.Attr { return slog.String(FieldTopic, t) }

// Operation returns an operation attribute.
func Operation(op string) slog.Attr { return slog.String(FieldOperation, op) }

// Store returns a store attribute naming a state store component.
func Store(name string) slog.Attr { return slog.String(FieldStore, name) }

// Method returns a method attribute.
func Method(m string) slog.Attr { return slog.String(FieldMethod, m) }

// InstanceID returns an instance_id attribute, used for workflow instances.
func InstanceID(id string) slog.Attr { return slog.String(FieldInstanceID, id) }
