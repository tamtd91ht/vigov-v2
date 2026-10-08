package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// THE UNVERIFIED INTAKE — ADR 0080: a petition from a Mini App session with no verified phone, owned
// by the session's Zalo account.
//
//	PROVED HERE   the row carries `zalo_account_id` and NO `cong_dan_id` · the trail's actor is the
//	              account id with Kind zalo-account, in the SAME transaction · the delta says
//	              contact_unverified and holds no typed value · NO outbox row (no ZNS, decision 4) ·
//	              the ceiling is counted inside the transaction, after the account's lock, from the
//	              commune's midnight · at the ceiling nothing is written and ErrUnverifiedDailyLimit
//	              is returned · a verified citizen is never counted.
//
//	NOT PROVED    PostgreSQL: the count's real result, the advisory lock's real effect, migration
//	              0032's CHECKs. store/petition_owner_test.go asserts the statements' text.

const idZaloThu = "01JZALOACCOUNTTHUOCPHIEN0"

func zaloThu() IntakeSender {
	return IntakeSender{Owner: domain.PetitionOwner{Kind: domain.OwnerZaloAccount, ID: idZaloThu}, IP: "10.0.0.9"}
}

func TestUnverifiedIntakeStoresZaloOwnerAndNoCitizen(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), zaloThu())
	if err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if len(kho.thay) != 1 {
		t.Fatalf("ghi %d phiếu, muốn 1", len(kho.thay))
	}
	got := kho.thay[0]
	if got.ZaloAccountID != idZaloThu || got.CongDanID != "" {
		t.Errorf("chủ phiếu = (cong_dan_id %q, zalo_account_id %q), muốn ('', %q) — ADR 0080 #2",
			got.CongDanID, got.ZaloAccountID, idZaloThu)
	}
	if !p.ContactUnverified() || got.Kenh != domain.KenhZaloMiniApp {
		t.Errorf("phiếu trả về: unverified=%v kênh=%q", p.ContactUnverified(), got.Kenh)
	}
	// The typed contact details are KEPT — as contact, not identity (decision 2).
	if got.NguoiGuiDienThoai != "0900000000" {
		t.Errorf("số tự khai không được giữ làm liên hệ: %q", got.NguoiGuiDienThoai)
	}
}

// TestUnverifiedIntakeTrailAndNoOutbox — the statements the driver saw, in order and inside one
// committed transaction: lock, count, row, trail. NO su_kien_di row: without `cong_dan_id` there is no
// recipient, and ADR 0080 decision 4 forbids a message to a self-declared number.
func TestUnverifiedIntakeTrailAndNoOutbox(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{sentToday: 9}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), zaloThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	want := []string{"pg_advisory_xact_lock", "SELECT count(*)", "INSERT INTO phieu_phan_anh", "INSERT INTO audit_log"}
	if len(k.lenh) != len(want) {
		t.Fatalf("chạy %d câu lệnh, muốn %d (khoá, đếm, phiếu, vết — KHÔNG sự kiện đi): %v", len(k.lenh), len(want), k.lenh)
	}
	for i, w := range want {
		if !strings.Contains(k.lenh[i].sql, w) {
			t.Errorf("câu lệnh %d = %q, muốn chứa %q", i, k.lenh[i].sql, w)
		}
		if !k.lenh[i].trongGiaoDich {
			t.Errorf("câu lệnh %d chạy NGOÀI giao dịch: %q", i, k.lenh[i].sql)
		}
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "su_kien_di") {
			t.Errorf("ghi dòng sự kiện đi cho phiếu chưa xác thực — ZNS tới số tự khai (ADR 0080 điều kiện dừng #2): %q", l.sql)
		}
	}
	if k.commit != 1 || k.rollback != 0 {
		t.Errorf("commit=%d rollback=%d, muốn 1/0", k.commit, k.rollback)
	}

	// $1 tenant_id · $2 actor_id · $3 actor_kind · $4 actor_ip · $5 action · $6 subject · $7 at · $8 delta
	vet := k.lenh[3].args
	if vet[1] != idZaloThu || vet[2] != audit.KindZaloAccount || vet[3] != "10.0.0.9" {
		t.Errorf("chủ thể vết = (%v, %v, %v), muốn (%q, %q, 10.0.0.9) — ADR 0080 #9",
			vet[1], vet[2], vet[3], idZaloThu, audit.KindZaloAccount)
	}
	if vet[5] != maCoDinh {
		t.Errorf("subject = %v, muốn mã tra cứu", vet[5])
	}
	raw, _ := vet[7].([]byte)
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatalf("delta không phải JSON: %q", raw)
	}
	if delta["contact_unverified"] != true {
		t.Errorf("delta thiếu contact_unverified=true: %s", raw)
	}
	for _, cam := range []string{"0900000000", "Nguyễn Văn An", "Đống rác", idZaloThu} {
		if strings.Contains(string(raw), cam) {
			t.Errorf("delta mang %q: %s", cam, raw)
		}
	}
}

// TestUnverifiedIntakeCountsFromCommuneMidnight — the lock and the count are for THIS account, and the
// count starts at 00:00 Asia/Ho_Chi_Minh of the send. mocGuiThu is 07:14 UTC = 14:14 ICT on 22/09, so
// the day began at 21/09 17:00 UTC.
func TestUnverifiedIntakeCountsFromCommuneMidnight(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), zaloThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if len(kho.lockedFor) != 1 || kho.lockedFor[0] != idZaloThu {
		t.Errorf("khoá cho %v, muốn [%s]", kho.lockedFor, idZaloThu)
	}
	if len(kho.countedFor) != 1 || kho.countedFor[0] != idZaloThu {
		t.Errorf("đếm cho %v, muốn [%s]", kho.countedFor, idZaloThu)
	}
	want := time.Date(2026, 9, 21, 17, 0, 0, 0, time.UTC)
	if len(kho.countedSince) != 1 || !kho.countedSince[0].Equal(want) {
		t.Errorf("đếm từ %v, muốn %v (00:00 giờ Việt Nam ngày gửi)", kho.countedSince, want)
	}
}

// TestUnverifiedIntakeAtCeilingWritesNothing — the 11th send of the day: ErrUnverifiedDailyLimit, the
// transaction rolled back, no row, no trail, no code handed back.
func TestUnverifiedIntakeAtCeilingWritesNothing(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{sentToday: domain.UnverifiedDailyCeiling}, hanThu()

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), zaloThu())
	if !errors.Is(err, ErrUnverifiedDailyLimit) {
		t.Fatalf("lỗi = %v, muốn ErrUnverifiedDailyLimit", err)
	}
	if p.MaTraCuu != "" {
		t.Errorf("trả mã tra cứu %q cho một lượt bị từ chối", p.MaTraCuu)
	}
	if len(kho.thay) != 0 {
		t.Errorf("ghi %d phiếu dù đã chạm trần", len(kho.thay))
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT") {
			t.Errorf("chạy %q dù đã chạm trần", l.sql)
		}
	}
	if k.commit != 0 || k.rollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.commit, k.rollback)
	}
	// The account id never travels in the error (rule 3).
	if strings.Contains(err.Error(), idZaloThu) {
		t.Errorf("mã tài khoản Zalo lọt vào lỗi: %v", err)
	}
}

// TestVerifiedIntakeIsNeverCounted — the ceiling belongs to the unverified path alone; a verified
// citizen's intake takes no lock and runs no count.
func TestVerifiedIntakeIsNeverCounted(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{sentToday: 1000}, hanThu()
	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui của công dân đã xác thực bị từ chối: %v", err)
	}
	if len(kho.lockedFor) != 0 || len(kho.countedFor) != 0 {
		t.Errorf("đường công dân đã xác thực bị khoá/đếm: %v / %v", kho.lockedFor, kho.countedFor)
	}
	if kho.thay[0].ZaloAccountID != "" || kho.thay[0].CongDanID != idCongDan {
		t.Errorf("chủ phiếu công dân sai: %+v", kho.thay[0])
	}
}

// TestZaloOwnedTransitionWritesNoOutboxRow — decision 4 on the STAFF acts: every later transition of
// a Zalo-owned petition goes through the same builder, and with `cong_dan_id` empty it writes nothing,
// even on a transition that owes a citizen a message.
func TestZaloOwnedTransitionWritesNoOutboxRow(t *testing.T) {
	for _, moi := range []domain.TrangThai{domain.DangPhanLoai, domain.DaChuyenXuLy, domain.DaXuLy, domain.DaDong} {
		t.Run(string(moi), func(t *testing.T) {
			k := &khoGia{}
			db := pkgstore.New(sql.OpenDB(k))
			ctx := ctxXa(xaThu)
			p := domain.PhieuPhanAnh{MaTraCuu: maCoDinh, TrangThai: moi, ZaloAccountID: idZaloThu,
				HanTiepNhan: mocHanThu, HanXuLyXong: mocHanThu}
			err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
				return ghiSuKienDoiTrangThai(ctx, tx, petstore.NewSuKienDiStore(db),
					func() (string, error) { return idCoDinh, nil }, p, moi, mocGuiThu)
			})
			if err != nil {
				t.Fatalf("ghiSuKienDoiTrangThai: %v", err)
			}
			if len(k.lenh) != 0 {
				t.Errorf("ghi %d câu lệnh cho phiếu chủ là tài khoản Zalo — ZNS tới số tự khai: %v", len(k.lenh), k.lenh)
			}
		})
	}
}
