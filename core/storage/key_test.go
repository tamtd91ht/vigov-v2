package storage

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testTenant = "01J9ZK7Q3M5N8P2R4T6V8W0XYZ"
	testObject = "01JAB3CD4EF5GH6JK7MN8PQ9RS"
)

func validKey() Key {
	return Key{
		Class:     ClassContentSource,
		TenantID:  testTenant,
		CreatedAt: time.Date(2026, 9, 28, 23, 30, 0, 0, time.UTC),
		Service:   ServiceComms,
		Purpose:   PurposeContentVideo,
		ObjectID:  testObject,
		Variant:   VariantOriginal,
		Ext:       "mp4",
	}
}

const validPath = "content-source/t_01j9zk7q3m5n8p2r4t6v8w0xyz/2026/09/comms/content-video/01jab3cd4ef5gh6jk7mn8pq9rs/original.mp4"

func TestKeyPathAndParseRoundTrip(t *testing.T) {
	p, err := validKey().Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if p != validPath {
		t.Fatalf("Path = %q\nwant  %q", p, validPath)
	}
	k, err := ParseKey(p)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	// Parsed ids come back UPPERCASE, so they compare equal to the tenant_id in the context.
	if k.TenantID != testTenant || k.ObjectID != testObject {
		t.Errorf("ids = %q, %q", k.TenantID, k.ObjectID)
	}
	if k.Class != ClassContentSource || k.Service != ServiceComms || k.Purpose != PurposeContentVideo ||
		k.Variant != VariantOriginal || k.Ext != "mp4" || k.CreatedAt.Year() != 2026 || k.CreatedAt.Month() != 9 {
		t.Errorf("parsed = %+v", k)
	}
}

func TestKeyLowercaseInputIsAccepted(t *testing.T) {
	k := validKey()
	k.TenantID = strings.ToLower(testTenant)
	k.ObjectID = strings.ToLower(testObject)
	p, err := k.Path()
	if err != nil || p != validPath {
		t.Fatalf("Path = %q, %v", p, err)
	}
}

func TestKeyMonthIsUTC(t *testing.T) {
	k := validKey()
	// 00:30 on 1 October in UTC+7 is still September in UTC.
	k.CreatedAt = time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("ICT", 7*3600))
	p, err := k.Path()
	if err != nil || !strings.Contains(p, "/2026/09/") {
		t.Fatalf("Path = %q, %v", p, err)
	}
}

func TestKeyUploadAndExportPaths(t *testing.T) {
	up, err := validKey().UploadPath()
	if err != nil || up != "upload/"+validPath {
		t.Fatalf("UploadPath = %q, %v", up, err)
	}
	if _, err := ParseUploadKey(up); err != nil {
		t.Fatalf("ParseUploadKey: %v", err)
	}
	if _, err := ParseUploadKey(validPath); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("destination key accepted as upload key: %v", err)
	}
	ex, err := validKey().ExportPath()
	if err != nil || ex != "export/"+validPath {
		t.Fatalf("ExportPath = %q, %v", ex, err)
	}
	if _, err := ParseExportKey(ex); err != nil {
		t.Fatalf("ParseExportKey: %v", err)
	}
	if _, err := ParseExportKey(up); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("upload key accepted as export key: %v", err)
	}
}

func TestKeyInvalidSegments(t *testing.T) {
	cases := map[string]func(*Key){
		"unknown class":            func(k *Key) { k.Class = "misc" },
		"empty class":              func(k *Key) { k.Class = "" },
		"tenant too short":         func(k *Key) { k.TenantID = "01J9ZK" },
		"tenant bad alphabet":      func(k *Key) { k.TenantID = "01J9ZK7Q3M5N8P2R4T6V8W0XYU" },
		"tenant overflow":          func(k *Key) { k.TenantID = "81J9ZK7Q3M5N8P2R4T6V8W0XYZ" },
		"tenant is a domain":       func(k *Key) { k.TenantID = "xa-an-binh.vigov.vn" },
		"zero time":                func(k *Key) { k.CreatedAt = time.Time{} },
		"unknown service":          func(k *Key) { k.Service = "billing" },
		"unknown purpose":          func(k *Key) { k.Purpose = "avatar" },
		"purpose equals service":   func(k *Key) { k.Purpose = "comms" },
		"object id not ulid":       func(k *Key) { k.ObjectID = "photo-1" },
		"variant uppercase":        func(k *Key) { k.Variant = "Original" },
		"variant underscore":       func(k *Key) { k.Variant = "thumb_320" },
		"variant slash":            func(k *Key) { k.Variant = "a/b" },
		"variant empty":            func(k *Key) { k.Variant = "" },
		"variant is a file name":   func(k *Key) { k.Variant = "don-nguyen-van-a" },
		"variant mp4 no height":    func(k *Key) { k.Variant = "mp4-p" },
		"ext not allowed":          func(k *Key) { k.Ext = "html" },
		"ext svg":                  func(k *Key) { k.Ext = "svg" },
		"ext uppercase":            func(k *Key) { k.Ext = "MP4" },
		"ext jpeg (not canonical)": func(k *Key) { k.Ext = "jpeg" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			k := validKey()
			mutate(&k)
			if _, err := k.Path(); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("err = %v, want ErrInvalidKey", err)
			}
		})
	}
}

func TestParseKeyRefusesNonCanonical(t *testing.T) {
	cases := map[string]string{
		"uppercase tenant":   strings.Replace(validPath, "t_01j9zk7q3m5n8p2r4t6v8w0xyz", "t_01J9ZK7Q3M5N8P2R4T6V8W0XYZ", 1),
		"colon tenant":       strings.Replace(validPath, "t_", "t:", 1),
		"no tenant prefix":   strings.Replace(validPath, "t_", "", 1),
		"uppercase object":   strings.Replace(validPath, "01jab3cd4ef5gh6jk7mn8pq9rs", "01JAB3CD4EF5GH6JK7MN8PQ9RS", 1),
		"one-digit month":    strings.Replace(validPath, "/09/", "/9/", 1),
		"month 13":           strings.Replace(validPath, "/09/", "/13/", 1),
		"short year":         strings.Replace(validPath, "/2026/", "/26/", 1),
		"extra segment":      validPath + "/x",
		"missing segment":    strings.Replace(validPath, "/comms", "", 1),
		"double ext":         strings.Replace(validPath, "original.mp4", "original.tar.mp4", 1),
		"no ext":             strings.Replace(validPath, "original.mp4", "original", 1),
		"file name in key":   strings.Replace(validPath, "original.mp4", "don-nguyen-van-a.pdf", 1),
		"purpose is service": strings.Replace(validPath, "/content-video/", "/petitions/", 1),
		"leading slash":      "/" + validPath,
		"empty":              "",
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseKey(s); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("ParseKey(%q) err = %v, want ErrInvalidKey", s, err)
			}
		})
	}
}

// The IAM wildcard `…/*/comms/*` also matches a key whose purpose (or class) is `comms`, in
// ANOTHER service's tree (ADR 0052 §Cái giá). The closed lists must never collide.
func TestClosedListsDoNotCollide(t *testing.T) {
	for p := range knownPurposes {
		if knownServices[Service(p)] {
			t.Errorf("purpose %q equals a service name", p)
		}
		if !isSegment(string(p)) {
			t.Errorf("purpose %q is not [a-z0-9-]+", p)
		}
	}
	for c := range knownClasses {
		if knownServices[Service(c)] {
			t.Errorf("class %q equals a service name", c)
		}
		if !isSegment(string(c)) {
			t.Errorf("class %q is not [a-z0-9-]+", c)
		}
	}
	for s := range knownServices {
		if !isSegment(string(s)) {
			t.Errorf("service %q is not [a-z0-9-]+", s)
		}
	}
}

func TestKnownVariantsAccepted(t *testing.T) {
	for _, v := range []string{VariantOriginal, VariantPoster, "mp4-720p", "mp4-1080p", "thumb-320"} {
		k := validKey()
		k.Variant = v
		if _, err := k.Path(); err != nil {
			t.Errorf("variant %q refused: %v", v, err)
		}
	}
}

func TestNewObjectIDIsAValidKeySegment(t *testing.T) {
	id, err := NewObjectID()
	if err != nil {
		t.Fatalf("NewObjectID: %v", err)
	}
	k := validKey()
	k.ObjectID = id
	if _, err := k.Path(); err != nil {
		t.Fatalf("generated id rejected: %v", err)
	}
}
