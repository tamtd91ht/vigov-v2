package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Renaming a task through PATCH (`code`, vigov-require 7764c8a, user decision 28/09/2026), over the
// real store on the fake driver. The fake's `issuedCodes` is `task_issued_code` (migration 0015).
//
//	PROVED HERE   the rename issues the new code into the ledger BEFORE moving `ma`, in the act's one
//	              transaction, with a timeline row and an audit entry naming both codes · a code
//	              already issued — including THIS task's own previous code — is refused with nothing
//	              written · minting and the duplicate check read the LEDGER, not the register.
//	NOT PROVED    that PostgreSQL enforces the ledger and the relaxed trigger — see
//	              internal/store/task_issued_code_pg_test.go (skips without VIGOV_TEST_DSN).

func codeEdit(code string) petstore.SuaNhiemVu { return petstore.SuaNhiemVu{Code: &code} }

// ledgerWith is the fixture task (NV19) with its code already issued, as the backfill and the insert
// trigger of 0015 guarantee for every real row.
func ledgerWith(codes ...string) *khoNhiemVuGia {
	k := khoNVMau()
	k.issuedCodes = map[string]bool{maNVGoc: true}
	for _, c := range codes {
		k.issuedCodes[c] = true
	}
	return k
}

func TestChangeCode_IssuesThenRenamesInOneTransaction(t *testing.T) {
	k := ledgerWith()
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.Sua(ctx, maNVGoc, codeEdit("NV45"), canBoThu())
	if err != nil {
		t.Fatalf("đổi mã: %v", err)
	}
	if after.Ma != "NV45" {
		t.Errorf("phản hồi mang mã %q, muốn NV45", after.Ma)
	}

	// THE ORDER THE TRIGGER CHECKS: the ledger row first, then the rename. Reversed, the database
	// refuses the UPDATE (0015, nhiem_vu_bat_bien) and the whole act rolls back.
	issue, rename := -1, -1
	for i, l := range k.lenh {
		switch {
		case strings.Contains(l.sql, "INSERT INTO task_issued_code"):
			issue = i
			if l.args[1] != "NV45" || l.args[2] != idNVGoc {
				t.Errorf("cấp mã: args = %v, muốn mã NV45 cho id %s", l.args, idNVGoc)
			}
		case strings.Contains(l.sql, "UPDATE nhiem_vu SET ma"):
			rename = i
			// $3 new code, $4 old code — the WHERE clause carries the old one.
			if l.args[2] != "NV45" || l.args[3] != maNVGoc {
				t.Errorf("đổi mã: args = %v, muốn NV45 thay %s", l.args, maNVGoc)
			}
		}
	}
	if issue < 0 || rename < 0 || issue > rename {
		t.Fatalf("thứ tự sai: cấp mã ở %d, đổi mã ở %d — phải cấp trước rồi mới đổi", issue, rename)
	}

	// THE OLD CODE STAYS ISSUED — nothing removes it (the fake has no removal, and neither does the
	// store: there is no statement for it).
	if !k.issuedCodes[maNVGoc] || !k.issuedCodes["NV45"] {
		t.Errorf("sổ mã đã cấp = %v, muốn giữ cả %s và NV45", k.issuedCodes, maNVGoc)
	}

	// THE TIMELINE ROW names both codes.
	nk := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(nk) != 1 {
		t.Fatalf("ghi %d dòng nhật ký, muốn 1", len(nk))
	}
	if got := nk[0].args[8]; got != "Đổi mã: "+maNVGoc+" → NV45" {
		t.Errorf("nhật ký = %v, muốn %q", got, "Đổi mã: "+maNVGoc+" → NV45")
	}

	// THE AUDIT ENTRY: `ma` before and after, filed under the NEW code.
	delta := auditDeltaText(t, k)
	for _, want := range []string{`"ma":"` + maNVGoc + `"`, `"ma":"NV45"`} {
		if !strings.Contains(delta, want) {
			t.Errorf("vết kiểm toán thiếu %s: %s", want, delta)
		}
	}
	if subj := vetKiemToan(t, k).args[5]; subj != "NV45" {
		t.Errorf("đối tượng vết = %v, muốn mã mới NV45", subj)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestChangeCode_ReusedCodeIs409AndWritesNothing — every way a code can already be issued. The last
// case is the one vigov-require gets wrong: NV12 was renamed away from, is on no row of the register,
// and must still be refused.
func TestChangeCode_ReusedCodeIs409AndWritesNothing(t *testing.T) {
	for name, code := range map[string]string{
		"mã hiện tại của nhiệm vụ khác":      "NV20",
		"mã của nhiệm vụ đã xoá mềm":         "NV07",
		"mã một nhiệm vụ đã đổi đi (bỏ lại)": "NV12",
	} {
		t.Run(name, func(t *testing.T) {
			k := ledgerWith("NV20", "NV07", "NV12")
			uc, ctx := dungGhiNhiemVu(t, k)

			_, err := uc.Sua(ctx, maNVGoc, codeEdit(code), canBoThu())
			if !errors.Is(err, petstore.ErrMaNhiemVuDaTonTai) {
				t.Fatalf("lỗi = %v, muốn ErrMaNhiemVuDaTonTai (409 code_taken)", err)
			}
			khongGhiGi(t, k)
		})
	}
}

// TestChangeCode_BackToOwnFormerCodeIsRefused — "never issued again" means to THIS task too. A rename
// NV19 → NV45 → NV19 would make NV19 name the task twice with a gap, and the timeline between the two
// would read as another task's.
func TestChangeCode_BackToOwnFormerCodeIsRefused(t *testing.T) {
	k := ledgerWith()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.Sua(ctx, maNVGoc, codeEdit("NV45"), canBoThu()); err != nil {
		t.Fatalf("đổi mã lần một: %v", err)
	}
	// The fake register row now carries NV45, as the database would.
	k.nhiemVu[idNVGoc]["ma"] = "NV45"
	k.lenh = nil
	k.daCommit = 0

	_, err := uc.Sua(ctx, "NV45", codeEdit(maNVGoc), canBoThu())
	if !errors.Is(err, petstore.ErrMaNhiemVuDaTonTai) {
		t.Fatalf("đổi về mã cũ của chính nó: lỗi = %v, muốn ErrMaNhiemVuDaTonTai", err)
	}
	khongGhiGi(t, k)
}

// TestChangeCode_SameCodeIsNoOp — sending the code the task already carries changes nothing and writes
// nothing (the route's idem.KhongCan rests on it). Trimmed first, like a typed code on create.
func TestChangeCode_SameCodeIsNoOp(t *testing.T) {
	k := ledgerWith()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.Sua(ctx, maNVGoc, codeEdit("  "+maNVGoc+" "), canBoThu()); err != nil {
		t.Fatalf("gửi lại đúng mã: %v", err)
	}
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "UPDATE") || strings.HasPrefix(l.sql, "INSERT") {
			t.Errorf("ghi dù mã không đổi: %q", l.sql)
		}
	}
}

// TestChangeCode_BadFormatIs400BeforeTheTransaction — the create-time format rule, and "" is a refusal,
// never "not mentioned" (nil is that).
func TestChangeCode_BadFormatIs400BeforeTheTransaction(t *testing.T) {
	for _, code := range []string{"", "nv 12", "NV/12", "NVĐ1"} {
		k := ledgerWith()
		uc, ctx := dungGhiNhiemVu(t, k)
		_, err := uc.Sua(ctx, maNVGoc, codeEdit(code), canBoThu())
		if !domain.LaLoiDauVaoNhiemVu(err) {
			t.Errorf("mã %q: lỗi = %v, muốn lỗi đầu vào (400)", code, err)
		}
		if k.batDau != 0 {
			t.Errorf("mã %q: mở giao dịch dù đầu vào sai", code)
		}
	}
}

// TestChangeCode_RaceOnRenameIsTaskState — the rename's WHERE carries the old code; zero rows means
// somebody renamed or removed the task in between, and the act is refused as a race (409 task_state).
func TestChangeCode_RaceOnRenameIsTaskState(t *testing.T) {
	k := ledgerWith()
	k.doiDong = 0
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.Sua(ctx, maNVGoc, codeEdit("NV45"), canBoThu())
	if !errors.Is(err, petstore.ErrNhiemVuDaChuyenTrang) {
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuDaChuyenTrang", err)
	}
	if k.daCommit != 0 {
		t.Error("đã commit dù đổi mã không khớp dòng nào")
	}
}

// TestMintingAndDuplicateCheckReadTheLedger — the property the whole card rests on, at the SQL the
// store sends: auto-generation and the typed-code check ask `task_issued_code`, never `nhiem_vu`.
// Over the register, a code renamed away from would be free again.
func TestMintingAndDuplicateCheckReadTheLedger(t *testing.T) {
	t.Run("tự sinh", func(t *testing.T) {
		k := ledgerWith()
		k.soLonNhat = 44 // e.g. NV44 was renamed away from: still the high-water mark
		uc, ctx := dungGhiNhiemVu(t, k)
		n, err := uc.Tao(ctx, taoMau(), canBoThu())
		if err != nil {
			t.Fatalf("giao việc: %v", err)
		}
		if n.Ma != "NV45" {
			t.Errorf("mã tự sinh = %q, muốn NV45", n.Ma)
		}
		assertReadsLedger(t, k, "MAX(")
	})
	t.Run("gõ tay trùng mã đã đổi đi", func(t *testing.T) {
		k := ledgerWith("NV12")
		uc, ctx := dungGhiNhiemVu(t, k)
		yc := taoMau()
		yc.TuSinhMa = false
		yc.Ma = "NV12"
		_, err := uc.Tao(ctx, yc, canBoThu())
		if !errors.Is(err, petstore.ErrMaNhiemVuDaTonTai) {
			t.Fatalf("lỗi = %v, muốn ErrMaNhiemVuDaTonTai", err)
		}
		assertReadsLedger(t, k, "count(*)")
		khongGhiGi(t, k)
	})
}

func assertReadsLedger(t *testing.T, k *khoNhiemVuGia, marker string) {
	t.Helper()
	q := k.cau(marker)
	if len(q) == 0 {
		t.Fatalf("không có câu đọc %q", marker)
	}
	for _, l := range q {
		if !strings.Contains(l.sql, "FROM task_issued_code") {
			t.Errorf("câu %q đọc sổ nhiệm vụ thay vì sổ mã đã cấp: %q", marker, l.sql)
		}
	}
}
