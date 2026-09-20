package crosstenant

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Tests that need no database, for the pairing-code lookup. The pg suite next door SKIPS
// without VIGOV_TEST_DSN while the package still prints `ok`.

const (
	maThoThu = "ma-ghep-gia-khong-phai-ma-that"
	idGhepCT = "GP-01"
)

var (
	mocTaoCT  = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	mocHetHan = mocTaoCT.Add(120 * time.Second)
)

// dongMaGhep is one sample row. dung_luc and huy_luc are the two adjacent NULLABLE TIMESTAMPS
// whose swap produces no error at all and turns "somebody cancelled this" into "somebody used
// this"; the caller passes both to the state derivation, so a swap changes the answer given to
// a screen without changing anything a compiler can see.
func dongMaGhep(dungLuc, huyLuc any) map[string]driver.Value {
	return map[string]driver.Value{
		"tenant_id":       xaMotCT,
		"id":              idGhepCT,
		"ky_tu_doi_chieu": "K7QM",
		"het_han_luc":     mocHetHan,
		"dung_luc":        dungLuc,
		"huy_luc":         huyLuc,
	}
}

// --- how the code is looked up -------------------------------------------------------------------

func TestTheoMaTimTheoMaBamChuKhongPhaiMaTho(t *testing.T) {
	// THE CODE IS A BEARER SECRET FOR TWO MINUTES. Only its hash is bound as a parameter, so a
	// driver error that echoes the statement cannot carry the code into a log — and the stored
	// column is a hash, which is what makes a leaked backup useless for pairing.
	k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(nil, nil)}}

	if _, _, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu); err != nil {
		t.Fatalf("TheoMa lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "WHERE bam_ma = $1") {
		t.Errorf("không tra theo mã băm: %q", l.sql)
	}
	tong := sha256.Sum256([]byte(maThoThu))
	if len(l.args) < 1 || l.args[0] != hex.EncodeToString(tong[:]) {
		t.Fatalf("$1 = %v, muốn SHA-256 hex của mã", l.args)
	}
	for _, a := range l.args {
		if s, ok := a.(string); ok && s == maThoThu {
			t.Fatal("mã thô đi vào tham số truy vấn")
		}
	}
	if strings.Contains(l.sql, maThoThu) {
		t.Fatal("mã thô nằm trong chính câu lệnh")
	}
}

func TestTheoMaKhongPhamViHoaTheoXaVaKhongLocTrangThai(t *testing.T) {
	// ADR 0019, INVARIANT 8 IS WHY BOTH HALVES OF THIS ARE TRUE.
	//
	// No commune in the predicate: a mismatch must produce 401 AND AN ALERT, and an alert
	// requires knowing the code exists somewhere ELSE rather than nowhere. Scoping the lookup
	// would still fail closed — and would report it as "no such code", which is a typo.
	//
	// No state in the predicate either: "expired" and "somebody already used this" are two
	// different things to tell a citizen at a counter, and both differ from "this code does not
	// exist". Filtering here collapses all three into silence.
	k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(nil, nil)}}

	if _, _, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu); err != nil {
		t.Fatalf("TheoMa lỗi: %v", err)
	}
	q := k.cuoi()
	if strings.Contains(q.sql, "tenant_id = $") {
		t.Errorf("tra mã ghép lại phạm vi hoá theo xã — mất báo động lệch xã: %q", q.sql)
	}
	for _, manh := range []string{"dung_luc IS NULL", "huy_luc IS NULL", "het_han_luc >"} {
		if strings.Contains(q.sql, manh) {
			t.Errorf("câu lệnh lọc mất trạng thái %q — ba trường hợp gộp thành im lặng: %q", manh, q.sql)
		}
	}
}

func TestTheoMaKhongDocCongDanVaPhien(t *testing.T) {
	// Whoever presents a code holds the code; they must not thereby learn WHO spent it. The
	// state (DaGhep) is what a caller needs in order to refuse — the identity behind it is a
	// fact about a citizen, and this lookup is reachable by anyone holding the code.
	k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(mocTaoCT.Add(time.Second), nil)}}

	if _, _, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu); err != nil {
		t.Fatalf("TheoMa lỗi: %v", err)
	}
	for _, cot := range []string{"cong_dan_id", "phien_id"} {
		if strings.Contains(k.cuoi().sql, cot) {
			t.Errorf("câu lệnh đọc %q — ai dùng mã không phải việc của người cầm mã: %q", cot, k.cuoi().sql)
		}
	}
}

func TestTheoMaRongThiKhongChayCauLenhNao(t *testing.T) {
	k := &khoCT{}

	m, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), "")
	if err != nil || ok {
		t.Fatalf("mã rỗng: (%v, %v, %v), muốn (rỗng, false, nil)", m, ok, err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("mã rỗng mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestTheoMaDocDungTungCot(t *testing.T) {
	dung := mocTaoCT.Add(30 * time.Second)
	k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(dung, nil)}}

	m, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu)
	if err != nil || !ok {
		t.Fatalf("TheoMa = (%v, %v)", ok, err)
	}
	// THREE ADJACENT TEXT COLUMNS. A swap among them scans cleanly and hands the commune id to
	// a screen as the four characters to compare by eye.
	if string(m.XaID) != xaMotCT {
		t.Errorf("XaID = %q, muốn %q", m.XaID, xaMotCT)
	}
	if m.ID != idGhepCT {
		t.Errorf("ID = %q, muốn %q", m.ID, idGhepCT)
	}
	if m.KyTuDoiChieu != "K7QM" {
		t.Errorf("KyTuDoiChieu = %q, muốn K7QM", m.KyTuDoiChieu)
	}
	if !m.HetHanLuc.Equal(mocHetHan) {
		t.Errorf("HetHanLuc = %v, muốn %v", m.HetHanLuc, mocHetHan)
	}
	// THE TWO NULLABLE TIMESTAMPS. Swapping them turns "used" into "cancelled" and back, with
	// no error anywhere — and a screen would be told a pairing was abandoned when in fact a
	// session was issued in somebody's name.
	if m.DungLuc == nil || !m.DungLuc.Equal(dung) {
		t.Errorf("DungLuc = %v, muốn %v", m.DungLuc, dung)
	}
	if m.HuyLuc != nil {
		t.Errorf("HuyLuc = %v, muốn nil", m.HuyLuc)
	}
}

func TestTheoMaKhongCoDongThiOkFalseKhongLoi(t *testing.T) {
	// A mistyped or purged code is a legitimate answer, and it is kept separate from an error so
	// that a database outage is never read as a statement about somebody's pairing.
	k := &khoCT{}

	m, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu)
	if err != nil {
		t.Fatalf("không tìm thấy mã không phải lỗi, nhận: %v", err)
	}
	if ok {
		t.Fatalf("không có dòng nào mà vẫn trả về mã: %v", m)
	}
}

func TestTheoMaHaiDongThiTuChoi(t *testing.T) {
	// UNIQUE (tenant_id, bam_ma) is per commune — a unique index on a hash-partitioned table
	// must contain the partition key — so two rows for one hash is possible in principle. The
	// store cannot say whose code it is, so it says nothing: telling a screen in commune A
	// about commune B's pairing is the one outcome that must never happen.
	k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(nil, nil), dongMaGhep(nil, nil)}}

	_, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu)
	if !errors.Is(err, ErrMaGhepTrung) {
		t.Fatalf("lỗi = %v, muốn ErrMaGhepTrung", err)
	}
	if ok {
		t.Error("hai dòng mà vẫn trả về một mã")
	}
}

// --- the state, derived ------------------------------------------------------------------------

func TestMaGhepTrangThaiSuyTuBaMocThoiGian(t *testing.T) {
	// There is no trang_thai column and there must never be one. The row carries three
	// timestamps; the state is a comparison. "Expired" in particular is wrong the moment a job
	// that would have written it is late (rule 10, invariant 3).
	ca := []struct {
		ten    string
		dung   any
		huy    any
		bayGio time.Time
		muon   domain.TrangThaiMaGhep
	}{
		{"còn chờ", nil, nil, mocTaoCT.Add(time.Second), domain.ChoGhep},
		{"đã dùng", mocTaoCT.Add(time.Second), nil, mocTaoCT.Add(2 * time.Second), domain.DaGhep},
		{"đã huỷ", nil, mocTaoCT.Add(time.Second), mocTaoCT.Add(2 * time.Second), domain.DaHuy},
		{"hết hạn", nil, nil, mocHetHan, domain.HetHan},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			k := &khoCT{hang: []map[string]driver.Value{dongMaGhep(c.dung, c.huy)}}
			m, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu)
			if err != nil || !ok {
				t.Fatalf("TheoMa = (%v, %v)", ok, err)
			}
			if ra := m.TrangThai(c.bayGio); ra != c.muon {
				t.Errorf("trạng thái = %q, muốn %q", ra, c.muon)
			}
		})
	}
}

// --- the hash, and failures ------------------------------------------------------------------------

func TestBamMaGhepLaSHA256Hex(t *testing.T) {
	// PINNED, because the column it is compared against was written by the OTHER store
	// (store.GhepPhienStore.Tao, which uses bamRefresh). The two live in two packages and share
	// no code; if they ever computed different things, no code would fail to compile and every
	// redeem would simply come back "no such code" — a pairing flow that is broken for
	// everybody and blames the citizen.
	tong := sha256.Sum256([]byte(maThoThu))
	if bamMaGhep(maThoThu) != hex.EncodeToString(tong[:]) {
		t.Errorf("bamMaGhep = %q, muốn SHA-256 hex", bamMaGhep(maThoThu))
	}
	if len(bamMaGhep("")) != 64 {
		t.Errorf("mã băm dài %d ký tự, muốn 64", len(bamMaGhep("")))
	}
}

func TestTheoMaLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoCT{loi: goc}

	_, ok, err := NewGhepPhienStore(dbGiaCT(k)).TheoMa(context.Background(), maThoThu)
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if ok {
		t.Error("lỗi mà vẫn báo tìm thấy")
	}
	if strings.Contains(err.Error(), maThoThu) {
		t.Errorf("mã thô lọt vào thông điệp lỗi: %v", err)
	}
}
