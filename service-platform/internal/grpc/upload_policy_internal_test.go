package grpc

import (
	"testing"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
)

// Every non-zero enum value round-trips through the derivation rule, so a value added to the proto
// is reachable from a stored row the day its row is seeded — no table to update.
func TestPurposeDerivationRoundTripsEveryEnumValue(t *testing.T) {
	for v, name := range platformv1.UploadPurpose_name {
		if v == 0 {
			continue
		}
		stored := purposeFromEnumName(name)
		got, ok := purposeToProto(stored)
		if !ok || int32(got) != v {
			t.Errorf("%s -> %q -> (%v, %v); want back %d", name, stored, got, ok, v)
		}
	}
}

func TestPurposeToProtoRefusesNonCanonicalSpellings(t *testing.T) {
	for _, s := range []string{"", "unspecified", "Content-Video", "content_video", "CONTENT-VIDEO",
		" content-video", "content-video ", "retired-purpose"} {
		if v, ok := purposeToProto(s); ok {
			t.Errorf("%q accepted as %v", s, v)
		}
	}
	if v, ok := purposeToProto("content-video"); !ok || v != platformv1.UploadPurpose_UPLOAD_PURPOSE_CONTENT_VIDEO {
		t.Errorf("content-video -> %v, %v", v, ok)
	}
}
