package events

import (
	"errors"
	"testing"
)

func TestTopicCarriesTheProfileAsPrefix(t *testing.T) {
	for _, p := range []string{"dev", "staging", "prod"} {
		got, err := Topic(p, "comms.zalo_oa_event_received.v1")
		if err != nil || got != p+".comms.zalo_oa_event_received.v1" {
			t.Errorf("Topic(%q) = %q, %v", p, got, err)
		}
	}
}

// A missing or unknown profile is refused — never defaulted, since the default would be somebody
// else's environment on the shared cluster.
func TestTopicRefusesAMissingProfile(t *testing.T) {
	for _, p := range []string{"", "production", "PROD", "local"} {
		if _, err := Topic(p, "comms.zalo_oa_event_received.v1"); !errors.Is(err, ErrNoProfile) {
			t.Errorf("Topic(%q): err = %v, muốn ErrNoProfile", p, err)
		}
	}
}

func TestTopicRefusesAnUnversionedOrPrefixedEvent(t *testing.T) {
	for _, ev := range []string{"", "comms.zalo_oa_event_received", "prod.comms.x.v1x", "Comms.x.v1", "v1"} {
		if _, err := Topic("dev", ev); err == nil {
			t.Errorf("Topic(dev, %q) được chấp nhận", ev)
		}
	}
}

func TestConsumerGroup(t *testing.T) {
	got, err := ConsumerGroup("prod", "comms", "zalo-oa-events")
	if err != nil || got != "prod.comms.zalo-oa-events" {
		t.Fatalf("ConsumerGroup = %q, %v", got, err)
	}
	if _, err := ConsumerGroup("", "comms", "x"); !errors.Is(err, ErrNoProfile) {
		t.Errorf("thiếu profile: err = %v", err)
	}
	for _, bad := range [][2]string{{"", "x"}, {"comms", ""}, {"Comms", "x"}, {"comms", "a.b"}} {
		if _, err := ConsumerGroup("dev", bad[0], bad[1]); err == nil {
			t.Errorf("ConsumerGroup(dev, %q, %q) được chấp nhận", bad[0], bad[1])
		}
	}
}
