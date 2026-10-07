package store

// CitizenLetterStore against a real PostgreSQL. SKIPPED UNLESS VIGOV_TEST_DSN IS SET (schema from
// TestMain in loai_van_ban_pg_test.go). What only the database can prove: the column list scans the
// real schema, writes pass 0006's CHECKs, the commune binds every statement, the duplicate query's
// lower(sender_name) match, the derived closed_at, and the denunciation summary staying out of search.
//
// Test data uses the agreed fake phone number (rule 3, invariant 5).

import (
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

func TestPgCitizenLetterStoreRoundTrip(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	ctxA, ctxB := ctxXa(a), ctxXa(b)
	kho := pkgstore.New(db)
	s := NewCitizenLetterStore(kho)
	series := NewDaySoStore(kho)
	received := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)

	book := func(id string, typ domain.LetterType, name, summary string) {
		t.Helper()
		err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
			n, err := series.CapSo(ctxA, tx, SeriesCitizenLetter, 2026)
			if err != nil {
				return err
			}
			return s.Insert(ctxA, tx, domain.CitizenLetter{ID: id, Number: n, Year: 2026, ReceivedDate: received,
				Type: typ, SenderName: name, SenderPhone: "0900000000", Summary: summary, CreatedByCode: "CB-TEST01"})
		})
		mustAccept(t, err, "vào sổ "+id)
	}
	book(a+"-1", domain.LetterTypeFeedback, "Nguyễn Văn A", "Đề nghị sửa đường liên thôn")
	book(a+"-2", domain.LetterTypeDenunciation, "Trần Thị B", "Tố cáo sửa đường sai thiết kế")

	// Read back in commune A, invisible from commune B.
	err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		l, err := s.ForUpdate(ctxA, tx, a+"-1")
		if err != nil {
			return err
		}
		if l.Number != 1 || l.Status != domain.LetterStatusNew || l.SenderName != "Nguyễn Văn A" || !l.ClosedAt.IsZero() {
			t.Errorf("đọc lại sai: %+v", l)
		}
		// Move to a processing-phase outcome with its log row: closed_at must derive from the log.
		l.Status = domain.LetterStatusScreening
		if err := s.UpdateStatus(ctxA, tx, l, "CB-TEST01"); err != nil {
			return err
		}
		l.Status = domain.LetterStatusFiled
		if err := s.UpdateStatus(ctxA, tx, l, "CB-TEST01"); err != nil {
			return err
		}
		return s.InsertLog(ctxA, tx, domain.LetterLogEntry{ID: a + "-log", LetterID: l.ID, At: at,
			ActorCode: "CB-TEST01", Kind: domain.LetterLogStatusChange,
			FromStatus: domain.LetterStatusScreening, ToStatus: domain.LetterStatusFiled})
	})
	mustAccept(t, err, "đổi trạng thái")

	err = kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		l, err := s.ByID(ctxA, tx, a+"-1")
		if err != nil {
			return err
		}
		if !l.ClosedAt.Equal(at) {
			t.Errorf("closed_at suy ra = %v, muốn %v", l.ClosedAt, at)
		}
		return nil
	})
	mustAccept(t, err, "đọc closed_at")

	_ = kho.For(ctxB).Tx(ctxB, func(tx *pkgstore.ScopedTx) error {
		if _, err := s.ByID(ctxB, tx, a+"-1"); err != ErrCitizenLetterNotFound {
			t.Errorf("xã B đọc được đơn của xã A: %v", err)
		}
		return nil
	})

	// Duplicates: case-insensitive name match, same commune only.
	rows, err := s.DuplicateCandidates(ctxA, "nguyễn văn a", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC))
	mustAccept(t, err, "kiểm trùng")
	if len(rows) != 1 || rows[0].ID != a+"-1" {
		t.Errorf("ứng viên trùng theo tên: %d dòng", len(rows))
	}
	if rows, _ := s.DuplicateCandidates(ctxB, "nguyễn văn a", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)); len(rows) != 0 {
		t.Error("kiểm trùng lọt sang xã khác")
	}
	// A denunciation is never a candidate, by name or by window (ADR 0078 #4).
	if rows, _ := s.DuplicateCandidates(ctxA, "trần thị b", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)); len(rows) != 0 {
		t.Error("đơn tố cáo là ứng viên trùng theo tên — lộ người tố cáo")
	}
	if rows, _ := s.DuplicateCandidates(ctxA, "", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)); len(rows) != 1 {
		t.Errorf("không tên: %d ứng viên, muốn 1 (đơn tố cáo bị loại)", len(rows))
	}

	// Search on "sửa đường" finds the ordinary letter only — a denunciation's summary is not searched.
	req, err := page.New(SortCitizenLetters, "", "", "", "")
	mustAccept(t, err, "page.New")
	res, err := s.List(ctxA, CitizenLetterFilter{Search: "sửa đường"}, req)
	mustAccept(t, err, "danh sách")
	if len(res.Items) != 1 || res.Items[0].ID != a+"-1" {
		t.Errorf("tìm kiếm trả %d dòng — trích yếu đơn tố cáo không được tìm", len(res.Items))
	}

	rep, err := s.ReportRows(ctxA, 2026, time.Date(2026, 1, 1, 0, 0, 0, 0, domain.AutomationZone))
	mustAccept(t, err, "báo cáo")
	if len(rep) != 2 {
		t.Errorf("báo cáo đọc %d dòng, muốn 2", len(rep))
	}
}
