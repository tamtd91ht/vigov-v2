package events

import (
	"errors"
	"fmt"
	"regexp"
)

// WHY THE PROFILE IS IN EVERY TOPIC AND GROUP NAME: dev and prod share ONE Kafka cluster (owner,
// 17/10/2026). A topic named only after its event — "comms.zalo_oa_event_received.v1" — is the same
// topic for both, so a dev pod publishes test events into prod's stream and a dev consumer joining the
// same group steals prod's partitions: prod messages processed by dev code against the dev database,
// silently, with every test green. The prefix `<ENV>.` is what keeps the two apart, and it is a prefix
// (not a suffix) so the broker's ACLs can grant `prod.` as one prefixed resource.
//
// These two functions are the ONLY place a Kafka name is built. service_boundary_guard blocks a Kafka
// client import outside core/events and a literal topic or group name inside it — a name typed by hand
// is a name typed without the profile.

// ErrNoProfile is a name requested without a valid profile. Refused, never defaulted: a default here
// would be "prod" for somebody's dev pod.
var ErrNoProfile = errors.New("events: no valid profile (ENV) for a Kafka name")

// profiles are the values of ENV (core/config.EnvDev/EnvStaging/EnvProd). Kept here as literals because
// core/config imports nothing from this package and this package must not import config for one list.
var profiles = map[string]bool{"dev": true, "staging": true, "prod": true}

var (
	eventName = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+\.v[1-9][0-9]*$`)
	namePart  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Topic is the Kafka topic of one event in one profile: "<profile>.<event name>",
// e.g. "prod.comms.zalo_oa_event_received.v1". The event name must carry its version (rule 2, inv. 4).
func Topic(profile, event string) (string, error) {
	if !profiles[profile] {
		return "", fmt.Errorf("%w: %q", ErrNoProfile, profile)
	}
	if !eventName.MatchString(event) {
		return "", fmt.Errorf("events: event name %q is not <service>.<fact>.v<N>", event)
	}
	return profile + "." + event, nil
}

// ConsumerGroup is the Kafka consumer group of one consumer in one profile: "<profile>.<service>.<purpose>",
// e.g. "prod.comms.zalo-oa-events". service and purpose are lower-case kebab words.
func ConsumerGroup(profile, service, purpose string) (string, error) {
	if !profiles[profile] {
		return "", fmt.Errorf("%w: %q", ErrNoProfile, profile)
	}
	if !namePart.MatchString(service) || !namePart.MatchString(purpose) {
		return "", fmt.Errorf("events: consumer group parts %q, %q must be lower-case kebab", service, purpose)
	}
	return profile + "." + service + "." + purpose, nil
}
