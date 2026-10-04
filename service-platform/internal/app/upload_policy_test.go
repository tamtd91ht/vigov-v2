package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// A purpose core/storage gains must be classed before the console can edit it — and a class must
// never name a type core/storage cannot sniff.
func TestEveryPurposeHasAClass(t *testing.T) {
	for _, p := range storage.Purposes() {
		c, ok := purposeClass[p]
		if !ok {
			t.Errorf("purpose %q has no media class — add it to purposeClass", p)
			continue
		}
		for _, m := range c.mimes {
			if _, ok := storage.ExtForMIME(m); !ok {
				t.Errorf("%s: class %s names %q, not in core/storage's allow-list", p, c.name, m)
			}
		}
		if c.maxBytes <= 0 || c.maxBytes > 5*gib {
			t.Errorf("%s: cap %d outside (0, 5 GiB] — the database CHECK of 0008", p, c.maxBytes)
		}
	}
	if len(purposeClass) != len(storage.Purposes()) {
		t.Errorf("purposeClass has %d entries, core/storage %d", len(purposeClass), len(storage.Purposes()))
	}
}

// HEIC is never allowed on a purpose whose pipeline re-encodes (core/imaging cannot decode it).
func TestReencodedPurposesRefuseHEIC(t *testing.T) {
	for _, p := range []storage.Purpose{storage.PurposeTenantLogo, storage.PurposeTenantBanner,
		storage.PurposePetitionPhoto, storage.PurposePetitionVerificationPhoto, storage.PurposeContentImage,
		storage.PurposeContentBodyImage, storage.PurposeStaffAvatar} {
		err := ValidateUploadPolicy(domain.UploadPolicy{Purpose: string(p), MaxBytes: 2 * mib,
			AllowedMIMETypes: []string{storage.MIMEJPEG, storage.MIMEHEIC}})
		if !errors.Is(err, ErrMIMETypes) {
			t.Errorf("%s with HEIC: err = %v, want ErrMIMETypes", p, err)
		}
	}
}

func TestValidateUploadPolicy(t *testing.T) {
	ok := domain.UploadPolicy{Purpose: "petition-photo", MaxBytes: 10 * mib,
		AllowedMIMETypes: []string{"image/jpeg", "image/png"}, FileCountLimited: true, MaxFilesPerSubject: 5}
	if err := ValidateUploadPolicy(ok); err != nil {
		t.Fatalf("valid edit refused: %v", err)
	}
	noCount := ok
	noCount.FileCountLimited, noCount.MaxFilesPerSubject = false, 0
	if err := ValidateUploadPolicy(noCount); err != nil {
		t.Fatalf("explicit no-count refused: %v", err)
	}
	video := domain.UploadPolicy{Purpose: "content-video", MaxBytes: 5 * gib, AllowedMIMETypes: []string{"video/mp4"}}
	if err := ValidateUploadPolicy(video); err != nil {
		t.Fatalf("video at the S3 ceiling refused: %v", err)
	}

	for name, c := range map[string]struct {
		mut  func(*domain.UploadPolicy)
		want error
	}{
		"unknown purpose": {func(p *domain.UploadPolicy) { p.Purpose = "everything" }, ErrUnknownPurpose},
		"zero bytes":      {func(p *domain.UploadPolicy) { p.MaxBytes = 0 }, ErrMaxBytes},
		"over image cap":  {func(p *domain.UploadPolicy) { p.MaxBytes = 50*mib + 1 }, ErrMaxBytes},
		"no types":        {func(p *domain.UploadPolicy) { p.AllowedMIMETypes = nil }, ErrMIMETypes},
		"pdf on a photo":  {func(p *domain.UploadPolicy) { p.AllowedMIMETypes = []string{"application/pdf"} }, ErrMIMETypes},
		"unknown type":    {func(p *domain.UploadPolicy) { p.AllowedMIMETypes = []string{"image/gif"} }, ErrMIMETypes},
		"repeated type":   {func(p *domain.UploadPolicy) { p.AllowedMIMETypes = []string{"image/png", "image/png"} }, ErrMIMETypes},
		"client spelling": {func(p *domain.UploadPolicy) { p.AllowedMIMETypes = []string{"image/jpg"} }, ErrMIMETypes},
		"zero files":      {func(p *domain.UploadPolicy) { p.MaxFilesPerSubject = 0 }, ErrMaxFilesInvalid},
		"over files cap":  {func(p *domain.UploadPolicy) { p.MaxFilesPerSubject = MaxFilesCap + 1 }, ErrMaxFilesInvalid},
	} {
		p := ok
		p.AllowedMIMETypes = append([]string(nil), ok.AllowedMIMETypes...)
		c.mut(&p)
		if err := ValidateUploadPolicy(p); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}
