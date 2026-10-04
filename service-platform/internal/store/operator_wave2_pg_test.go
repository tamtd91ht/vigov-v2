package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ADR 0073 wave 2 against a real PostgreSQL — skipped without VIGOV_TEST_DSN, like every
// *_pg_test.go here. What only the database can show: the policy row and its platform_audit_log
// entry commit together, the merged two-table read pages without loss or repeat, and the read's own
// entry is written and never listed back.

func TestPgChangeUploadPolicyWritesRowAndTrailTogether(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewUploadPolicyStore(db)
	ctx := context.Background()
	trail := `SELECT count(*) FROM platform_audit_log WHERE subject = 'petition-photo' AND action = 'upload_policy.changed'`
	before := count(t, db, trail)

	next := domain.UploadPolicy{Purpose: "petition-photo", MaxBytes: 5242880,
		AllowedMIMETypes: []string{"image/png", "image/jpeg"}}
	out, changed, err := s.ChangeUploadPolicy(ctx, next, "Giảm dung lượng", operatorFake)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if out.MaxBytes != 5242880 || out.FileCountLimited || out.UpdatedBy != "VH-00001" ||
		strings.Join(out.AllowedMIMETypes, ",") != "image/png,image/jpeg" {
		t.Errorf("stored %+v", out)
	}
	if count(t, db, trail) != before+1 {
		t.Fatal("no trail entry for the change")
	}
	var actor, ip, reason, b, a string
	if err := db.QueryRow(`SELECT actor, actor_ip, reason, before::text, after::text FROM platform_audit_log
		WHERE subject = 'petition-photo' ORDER BY id DESC LIMIT 1`).Scan(&actor, &ip, &reason, &b, &a); err != nil {
		t.Fatal(err)
	}
	if actor != "VH-00001" || ip != operatorFake.IP || reason != "Giảm dung lượng" ||
		!strings.Contains(b, `"max_files_per_subject": 5`) || !strings.Contains(a, `"max_files_per_subject": null`) {
		t.Errorf("entry actor=%s ip=%s reason=%s before=%s after=%s", actor, ip, reason, b, a)
	}

	// The same values again (MIME order aside): nothing written.
	next.AllowedMIMETypes = []string{"image/jpeg", "image/png"}
	if _, changed, err := s.ChangeUploadPolicy(ctx, next, "Lặp lại", operatorFake); err != nil || changed {
		t.Fatalf("repeat: changed=%v err=%v", changed, err)
	}
	if count(t, db, trail) != before+1 {
		t.Error("a no-op wrote a trail entry")
	}

	// Refusals write nothing.
	if _, _, err := s.ChangeUploadPolicy(ctx, next, "x", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("no actor: %v", err)
	}
	if _, err := db.Exec(`UPDATE upload_policy SET deleted_at = now(), deleted_by = 'system', delete_reason = 'test'
		WHERE purpose = 'staff-avatar'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ChangeUploadPolicy(ctx, domain.UploadPolicy{Purpose: "staff-avatar", MaxBytes: 1,
		AllowedMIMETypes: []string{"image/png"}}, "x", operatorFake); !errors.Is(err, ErrUploadPolicyNotFound) {
		t.Errorf("withdrawn purpose: %v", err)
	}
	// The database's own bound refuses — and rolls the whole change back.
	n := count(t, db, `SELECT count(*) FROM platform_audit_log`)
	if _, _, err := s.ChangeUploadPolicy(ctx, domain.UploadPolicy{Purpose: "content-video", MaxBytes: 5368709121,
		AllowedMIMETypes: []string{"video/mp4"}}, "x", operatorFake); err == nil {
		t.Error("over the 5 GiB CHECK accepted")
	}
	if count(t, db, `SELECT count(*) FROM platform_audit_log`) != n {
		t.Error("a refused change left a trail entry")
	}
}

func TestPgOperatorLogMergesPagesAndTrailsTheRead(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Tân Phú", true)
	w := NewRegistryWriter(corestore.New(db))
	if err := w.AddDomain(inCommune(ulidA), "a.example.gov.vn", operatorFake); err != nil {
		t.Fatal(err)
	}
	if err := w.RecordMiniAppSecretForward(inCommune(ulidB), domain.SecretForward{AppID: "3291993990104489440",
		Version: "01JDVERSIONFAKE00000000000", Reason: "Đặt khoá"}, operatorFake); err != nil {
		t.Fatal(err)
	}
	// A STAFF entry in the same table must never reach the operator log.
	if err := corestore.New(db).For(inCommune(ulidA)).Tx(context.Background(), func(tx *corestore.ScopedTx) error {
		return audit.Write(context.Background(), tx, audit.Entry{Actor: audit.Actor{ID: "CB-00001", Kind: "staff"},
			Action: "sua_ho_so_hien_thi", Subject: "logo"})
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewUploadPolicyStore(db).ChangeUploadPolicy(context.Background(), domain.UploadPolicy{
		Purpose: "content-audio", MaxBytes: 1048576, AllowedMIMETypes: []string{"audio/mpeg"},
		FileCountLimited: true, MaxFilesPerSubject: 1}, "Thu hẹp", operatorFake); err != nil {
		t.Fatal(err)
	}

	l := NewOperatorLog(db)
	reader := domain.OperatorActor{Code: "VH-00002", IP: "203.0.113.9"}
	var all []domain.OperatorLogEntry
	cursor := ""
	for i := 0; i < 50; i++ {
		req, err := page.New(OperatorLogOrder, "", "", "2", cursor)
		if err != nil {
			t.Fatal(err)
		}
		res, err := l.Read(context.Background(), OperatorLogQuery{Page: req}, reader)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, res.Items...)
		if !res.HasMore {
			break
		}
		cursor = res.NextCursor
	}
	seen := map[string]int{}
	for _, e := range all {
		seen[e.Action]++
		if e.Action == "sua_ho_so_hien_thi" || e.Action == ActionOperatorLogRead {
			t.Errorf("listed %s", e.Action)
		}
		if strings.Contains(string(e.After), "secret") && e.Action == ActionSetMiniAppSecret {
			t.Errorf("a secret-ish key in %s", e.After)
		}
	}
	if seen[ActionAddDomain] != 1 || seen[ActionSetMiniAppSecret] != 1 || seen[ActionUploadPolicyChanged] != 1 ||
		seen["upload_policy.seeded"] == 0 {
		t.Errorf("seen %v", seen)
	}
	for i := 1; i < len(all); i++ {
		if all[i].At.After(all[i-1].At) {
			t.Fatal("not newest first")
		}
	}
	for _, e := range all {
		if e.Action == ActionAddDomain && (e.CommuneID != ulidA || e.CommuneName != "Xã Thăng Bình" || e.Before != nil ||
			!json.Valid(e.After)) {
			t.Errorf("add-domain entry %+v", e)
		}
		if e.Action == ActionUploadPolicyChanged && (e.CommuneID != "" || e.Reason != "Thu hẹp" || e.Before == nil) {
			t.Errorf("policy entry %+v", e)
		}
	}
	if n := count(t, db, `SELECT count(*) FROM platform_audit_log WHERE action = $1 AND actor = 'VH-00002' AND actor_ip = '203.0.113.9'`,
		ActionOperatorLogRead); n < 2 {
		t.Errorf("read entries %d — every page must leave one", n)
	}

	// One commune: its own operator acts only, no platform-wide entries.
	req, _ := page.New(OperatorLogOrder, "", "", "", "")
	res, err := l.Read(context.Background(), OperatorLogQuery{CommuneID: ulidB, Page: req}, reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Action != ActionSetMiniAppSecret || res.Items[0].Reason != "Đặt khoá" {
		t.Errorf("commune B log %+v", res.Items)
	}
	if _, err := l.Read(context.Background(), OperatorLogQuery{Page: req}, domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("anonymous read: %v", err)
	}
}
