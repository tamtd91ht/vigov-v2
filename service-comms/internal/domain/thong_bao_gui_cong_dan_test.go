package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR. Two rules decide whether a notification is worth sending, both are pure,
// and both fail SILENTLY if they stop holding:
//
//  1. the message carries the lookup code, says what changed, and says what happens next
//     (rule 10, invariant 6). A message that fails this still sends, still looks fine in a log,
//     and is useless to the person who receives it;
//  2. two notifications about the same fact produce the same key, and two notifications about
//     DIFFERENT facts never do. A key that is too loose swallows a real second message; a key
//     that is too tight sends a duplicate. Neither shows up anywhere except on a citizen's phone.

// --- (1) what a message must contain --------------------------------------------------------

func thamSoDay() ThamSoThongBao {
	return ThamSoThongBao{
		MaTraCuu:     "PA-2026-7F3K9Q",
		MocNhan:      "Đã chuyển xử lý",
		ViecTiepTheo: "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
	}
}

func TestThamSoDayDuThiHopLe(t *testing.T) {
	if err := thamSoDay().KiemTra(); err != nil {
		t.Fatalf("tham số đầy đủ mà bị từ chối: %v", err)
	}
}

func TestThamSoThieuTungPhanThiTuChoi(t *testing.T) {
	// EACH REFUSAL IS ASSERTED SEPARATELY, by its sentinel. A single "invalid" error would let
	// two of these three be dropped without a test turning red, and the one most likely to be
	// dropped is ViecTiepTheo — it is the one that looks like a nicety.
	ca := []struct {
		ten  string
		sua  func(*ThamSoThongBao)
		muon error
	}{
		{"thiếu mã tra cứu", func(t *ThamSoThongBao) { t.MaTraCuu = "" }, ErrThieuMaTraCuu},
		{"mã tra cứu chỉ có khoảng trắng", func(t *ThamSoThongBao) { t.MaTraCuu = "   " }, ErrThieuMaTraCuu},
		{"thiếu mốc", func(t *ThamSoThongBao) { t.MocNhan = "" }, ErrThieuMocNhan},
		{"thiếu việc tiếp theo", func(t *ThamSoThongBao) { t.ViecTiepTheo = "" }, ErrThieuViecTiepTheo},
		{"việc tiếp theo chỉ có khoảng trắng", func(t *ThamSoThongBao) { t.ViecTiepTheo = "\t \n" }, ErrThieuViecTiepTheo},
	}
	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			ts := thamSoDay()
			c.sua(&ts)
			if err := ts.KiemTra(); !errors.Is(err, c.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.muon)
			}
		})
	}
}

func TestThamSoViecTiepTheoLapLaiMocThiTuChoi(t *testing.T) {
	// THE LOOPHOLE IN THE PREVIOUS TEST, CLOSED. "Say what happens next" is trivially satisfied by
	// copying "what changed" into it, which produces "Đã xử lý. Đã xử lý." — worse than the bare
	// message rule 10 invariant 6 refuses, and it passes every emptiness check.
	//
	// The capitalisation differs on purpose: the copy somebody makes is the copy with the case
	// changed, and a case-sensitive comparison would wave it through.
	ts := thamSoDay()
	ts.MocNhan = "Đã xử lý"
	ts.ViecTiepTheo = "đã XỬ LÝ"

	if err := ts.KiemTra(); !errors.Is(err, ErrViecTiepTheoTrung) {
		t.Fatalf("lỗi = %v, muốn ErrViecTiepTheoTrung", err)
	}
}

func TestThamSoJSONMangDungBaKhoaCuaHopDong(t *testing.T) {
	// THE KEYS ARE THE CONTRACT WITH TWO THINGS AT ONCE: the CHECK constraint on `tham_so` in
	// migration 0004 names them, and the approved ZNS template is keyed by them. Renaming one
	// here breaks a constraint at the bottom of a transaction, which is the least readable place
	// for it to break.
	//
	// The ABSENT keys matter more than the present ones: an extra field added to
	// ThamSoThongBao would ship in every message to every citizen, and personal data leaving for
	// an external service is rule 3's second stop condition — not something to discover from a
	// diff.
	b, err := thamSoDay().JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var tho map[string]json.RawMessage
	if err := json.Unmarshal(b, &tho); err != nil {
		t.Fatalf("không phải JSON: %q", string(b))
	}
	muon := map[string]bool{"ma_tra_cuu": true, "moc_nhan": true, "viec_tiep_theo": true}
	for khoa := range tho {
		if !muon[khoa] {
			t.Errorf("trường ngoài khai báo lọt vào thông điệp gửi ra ngoài: %q — %s", khoa, string(b))
		}
	}
	if len(tho) != len(muon) {
		t.Errorf("số khoá = %d, muốn %d: %s", len(tho), len(muon), string(b))
	}
}

func TestThamSoKhongHopLeThiKhongCoJSONDeMaGhi(t *testing.T) {
	// A caller that swallowed the validation error upstream must not be handed a serialisable
	// value it can still write. Returning `{}` with the error would give it exactly that, and the
	// CHECK constraint would then be the only thing left standing — a database error at the bottom
	// of a transaction, with no explanation of which rule was broken.
	ts := thamSoDay()
	ts.ViecTiepTheo = ""

	b, err := ts.JSON()
	if !errors.Is(err, ErrThieuViecTiepTheo) {
		t.Fatalf("lỗi = %v, muốn ErrThieuViecTiepTheo", err)
	}
	if b != nil {
		t.Errorf("từ chối mà vẫn trả %d byte để ghi", len(b))
	}
}

// --- (2) the deduplication key ----------------------------------------------------------------

const (
	maPhieuThu    = "PA-2026-7F3K9Q"
	maCongDanThu  = "cd-01JCONGDANMAUTHUNGHIEM"
	maCongDanKhac = "cd-01JCONGDANKHACTHUNGHIE"
)

func khoaThu(t *testing.T, moc string, lan int, nguoiNhan string) string {
	t.Helper()
	k, err := KhoaLanGui(DoiTuongPhieuPhanAnh, maPhieuThu, moc, lan, nguoiNhan, KenhZaloZNS)
	if err != nil {
		t.Fatalf("KhoaLanGui: %v", err)
	}
	return k
}

func TestKhoaLanGuiCungMotSuKienThiCungMotKhoa(t *testing.T) {
	// THE PROPERTY THE WHOLE IDEMPOTENCY RESTS ON. Queues deliver at least once, so the same fact
	// arrives twice; the second arrival must land on the same key and become a no-op at the
	// UNIQUE (tenant_id, khoa_lan_gui) constraint.
	//
	// It is NOT a test of determinism-in-general: it is a test that the key is built from the
	// FACT. A key that mixed in a message id, a timestamp or a random value would differ between
	// these two calls and send the citizen the same message twice.
	if a, b := khoaThu(t, "da-dong", 1, maCongDanThu), khoaThu(t, "da-dong", 1, maCongDanThu); a != b {
		t.Fatalf("hai lần giao cùng một sự kiện cho ra hai khoá: %q vs %q", a, b)
	}
}

func TestKhoaLanGuiPhanBietTungThanhPhan(t *testing.T) {
	// EACH COMPONENT IS ASSERTED TO CHANGE THE KEY, because dropping any one of them is a
	// plausible-looking simplification with a specific victim:
	//
	//	moc      two different transitions on one record collapse into one message
	//	lan      a petition closed, reopened and closed AGAIN (ADR 0008) owes a second
	//	         notification; without `lan` it is swallowed as a duplicate, and swallowing looks
	//	         exactly like correct deduplication from the inside
	//	người    two recipients of one transition collapse into one message
	goc := khoaThu(t, "da-dong", 1, maCongDanThu)

	khac := map[string]string{
		"mốc khác":        khoaThu(t, "dang-xu-ly", 1, maCongDanThu),
		"lần khác":        khoaThu(t, "da-dong", 2, maCongDanThu),
		"người nhận khác": khoaThu(t, "da-dong", 1, maCongDanKhac),
	}
	for ten, k := range khac {
		if k == goc {
			t.Errorf("%s vẫn cho ra cùng một khoá %q — một thông báo thật sẽ bị nuốt", ten, k)
		}
	}
}

func TestKhoaLanGuiPhanBietKenh(t *testing.T) {
	// A second channel is a second delivery, not a repeat of the first. Asserted separately
	// because there is only one channel today, which makes this the component most likely to be
	// dropped as "unused" — and the day a second channel exists, the mistake is invisible.
	a, err := KhoaLanGui(DoiTuongPhieuPhanAnh, maPhieuThu, "da-dong", 1, maCongDanThu, KenhZaloZNS)
	if err != nil {
		t.Fatalf("KhoaLanGui: %v", err)
	}
	b, err := KhoaLanGui(DoiTuongPhieuPhanAnh, maPhieuThu, "da-dong", 1, maCongDanThu, Kenh("kenh-khac"))
	if err != nil {
		t.Fatalf("KhoaLanGui: %v", err)
	}
	if a == b {
		t.Fatalf("hai kênh cho ra cùng một khoá %q", a)
	}
}

func TestKhoaLanGuiThanhPhanChuaDauPhanCachThiTuChoi(t *testing.T) {
	// REFUSE, DO NOT ESCAPE. Two different escaping conventions produce two different keys for one
	// fact, and the drift surfaces as duplicate messages to real people months later, with the
	// escaping function nowhere near the symptom.
	//
	// The refusal is also the only thing standing between a component and key COLLISION: with a
	// separator inside a value, ("a|b", "c") and ("a", "b|c") join to the same string — two
	// different facts, one key, and the second notification silently never sent.
	_, err := KhoaLanGui(DoiTuongPhieuPhanAnh, "PA|2026", "da-dong", 1, maCongDanThu, KenhZaloZNS)
	if !errors.Is(err, ErrKhoaCoDauPhanCach) {
		t.Fatalf("lỗi = %v, muốn ErrKhoaCoDauPhanCach", err)
	}
}

func TestKhoaLanGuiKhongMangDuLieuCaNhan(t *testing.T) {
	// THE KEY IS STORED, INDEXED, AND READ BY WHOEVER DEBUGS THE SEND PATH. Every component of it
	// is either a code or an opaque id (rule 3, invariant 2 allows a business code), and this
	// pins that: the only inputs are the ones passed in, so a future version that reached for the
	// recipient's number "to make the key more precise" has to change this signature first.
	k := khoaThu(t, "da-dong", 1, maCongDanThu)
	for _, phan := range strings.Split(k, dauPhanCach) {
		if phan == "" {
			t.Fatalf("khoá có thành phần rỗng: %q", k)
		}
	}
	if n := len(strings.Split(k, dauPhanCach)); n != 6 {
		t.Fatalf("khoá có %d thành phần, muốn 6: %q", n, k)
	}
}
