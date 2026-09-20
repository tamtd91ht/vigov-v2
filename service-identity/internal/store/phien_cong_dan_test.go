package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/httpx"
)

// Tests that need no database.
//
// THEY EXIST BECAUSE THE INTEGRATION FILE NEXT TO THIS ONE SKIPS SILENTLY when VIGOV_TEST_DSN
// is unset, and the package still prints `ok`. Everything that can be decided without SQL —
// which row the registry accepts, which arguments it refuses, what a second matching row means
// — is decided here, where it runs on every machine.

// dongGia stands in for *sql.Rows so quetPhien's decisions can be exercised directly. Those
// decisions (no row, two rows, a read that failed) are precisely the ones an integration suite
// cannot reach without contriving a collision in the database.
type dongGia struct {
	hang    [][3]string
	vt      int
	loiDoc  error
	loiScan error
}

func (d *dongGia) Next() bool {
	if d.vt >= len(d.hang) {
		return false
	}
	d.vt++
	return true
}

func (d *dongGia) Scan(dest ...any) error {
	if d.loiScan != nil {
		return d.loiScan
	}
	h := d.hang[d.vt-1]
	for i := range dest {
		p, ok := dest[i].(*string)
		if !ok {
			return errors.New("dongGia: cột không phải chuỗi")
		}
		*p = h[i]
	}
	return nil
}

func (d *dongGia) Err() error { return d.loiDoc }

func TestQuetPhienDocDungTungCot(t *testing.T) {
	// THE ORDER IS THE WHOLE TEST. cotPhienCongDan selects (tenant_id, id, cong_dan_id) and all
	// three are strings: swapping the session id with the citizen id compiles, returns rows, and
	// hands every request the wrong citizen. Three visibly different values are what turn that
	// into a red test instead of a silent leak.
	rows := &dongGia{hang: [][3]string{{"XA-01", "SID-02", "CD-03"}}}

	p, ok := quetPhien(rows)
	if !ok {
		t.Fatal("một dòng hợp lệ mà không tra ra phiên")
	}
	if string(p.TenantID) != "XA-01" {
		t.Errorf("TenantID = %q, muốn XA-01", p.TenantID)
	}
	if p.ID != "SID-02" {
		t.Errorf("ID = %q, muốn SID-02", p.ID)
	}
	if p.CitizenID != "CD-03" {
		t.Errorf("CitizenID = %q, muốn CD-03", p.CitizenID)
	}
}

func TestQuetPhienKhongCoDongThiTuChoi(t *testing.T) {
	// Unknown token, expired session and revoked session all arrive here as "no row" — the SQL
	// filtered them in one WHERE precisely so they cannot be told apart.
	if _, ok := quetPhien(&dongGia{}); ok {
		t.Error("không có dòng nào mà vẫn trả về phiên dùng được")
	}
}

func TestQuetPhienHaiDongThiTuChoi(t *testing.T) {
	// The UNIQUE on bam_token is composite with tenant_id (a unique index on a hash-partitioned
	// table must contain the partition key), so it is per commune and not global. Two rows means
	// the registry cannot say WHOSE session this is — and handing commune B's session to the
	// holder of commune A's token is the one outcome that must be impossible.
	rows := &dongGia{hang: [][3]string{
		{"XA-01", "SID-02", "CD-03"},
		{"XA-99", "SID-98", "CD-97"},
	}}
	if _, ok := quetPhien(rows); ok {
		t.Error("RÒ RỈ: hai phiên khớp một token mà vẫn chọn đại một cái")
	}
}

func TestQuetPhienLoiDocThiTuChoi(t *testing.T) {
	// A connection that dropped mid-read must not be readable as an answer about somebody's
	// session. Fail closed: rule 1, and core/httpx's one negative answer.
	if _, ok := quetPhien(&dongGia{loiDoc: errors.New("kết nối đứt")}); ok {
		t.Error("lỗi đọc mà vẫn trả về phiên dùng được")
	}
	rows := &dongGia{
		hang:    [][3]string{{"XA-01", "SID-02", "CD-03"}},
		loiScan: errors.New("kiểu cột lệch"),
	}
	if _, ok := quetPhien(rows); ok {
		t.Error("lỗi quét dòng mà vẫn trả về phiên dùng được")
	}
}

func TestQuetPhienChuaChonXaVanLaPhienDungDuoc(t *testing.T) {
	// A session with no commune is a REAL answer, not a broken row (ADR 0005): the citizen has
	// signed in and has not chosen a commune, and that is exactly when the commune-choice screen
	// calls. The registry says "usable"; core/httpx.XaTuPhien is what answers 401 on any
	// business route. Refusing it here would break the one screen it exists for.
	rows := &dongGia{hang: [][3]string{{xaChuaChon, "SID-02", "CD-03"}}}

	p, ok := quetPhien(rows)
	if !ok {
		t.Fatal("phiên chưa chọn xã bị coi là không dùng được")
	}
	if p.TenantID.Valid() {
		t.Errorf("TenantID = %q — phải là giá trị rỗng, để mọi tuyến nghiệp vụ trả 401", p.TenantID)
	}
}

func phienMoiHopLe() PhienMoi {
	return PhienMoi{CongDanID: "CD-01", Nguon: NguonApp, ThoiHan: time.Hour}
}

func TestPhienMoiTuChoiDauVaoSai(t *testing.T) {
	ca := map[string]struct {
		sua  func(*PhienMoi)
		muon error
	}{
		"thiếu công dân":         {func(p *PhienMoi) { p.CongDanID = "" }, ErrThieuCongDan},
		"công dân toàn dấu cách": {func(p *PhienMoi) { p.CongDanID = "   " }, ErrThieuCongDan},
		"nguồn lạ":               {func(p *PhienMoi) { p.Nguon = "web" }, ErrNguonKhongHopLe},
		"nguồn rỗng":             {func(p *PhienMoi) { p.Nguon = "" }, ErrNguonKhongHopLe},
		"thiếu thời hạn":         {func(p *PhienMoi) { p.ThoiHan = 0 }, ErrThieuThoiHan},
		"thời hạn âm":            {func(p *PhienMoi) { p.ThoiHan = -time.Hour }, ErrThieuThoiHan},
	}
	for ten, c := range ca {
		t.Run(ten, func(t *testing.T) {
			p := phienMoiHopLe()
			c.sua(&p)
			if err := p.kiemTra(); !errors.Is(err, c.muon) {
				t.Errorf("kiemTra = %v, muốn %v", err, c.muon)
			}
		})
	}
	if err := phienMoiHopLe().kiemTra(); err != nil {
		t.Errorf("phiên hợp lệ bị từ chối: %v", err)
	}
}

func TestThoiHanKhongCoMacDinh(t *testing.T) {
	// ADR 0019 leaves the TTL of a paired session open (§CÒN MỞ #1) and nothing has decided the
	// app session's either. A zero value must therefore be a REFUSAL, never "use the usual one":
	// a default chosen here would answer an open question silently, in the one place that is
	// expensive to change later.
	p := phienMoiHopLe()
	p.ThoiHan = 0
	if err := p.kiemTra(); !errors.Is(err, ErrThieuThoiHan) {
		t.Errorf("thời hạn 0 không bị từ chối, err = %v — có ai đó đã điền mặc định", err)
	}
}

// Every argument is checked before the transaction is touched, so nil is safe here — and that
// ordering is what makes these refusals testable at all on a machine with no database.

func TestTaoTuChoiKhiKhongCoGiaoDich(t *testing.T) {
	s := &PhienCongDanStore{}
	if _, _, err := s.Tao(context.Background(), nil, phienMoiHopLe()); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("Tao không kèm giao dịch: err = %v, muốn ErrThieuGiaoDich", err)
	}
	if _, _, err := s.TaoChuaChonXa(context.Background(), nil, phienMoiHopLe()); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("TaoChuaChonXa không kèm giao dịch: err = %v, muốn ErrThieuGiaoDich", err)
	}
}

func TestTaoKiemDauVaoTruocGiaoDich(t *testing.T) {
	// The precise error matters: accepting either one would keep this test green if the
	// validation were deleted outright.
	s := &PhienCongDanStore{}
	p := phienMoiHopLe()
	p.Nguon = "web"
	if _, _, err := s.Tao(context.Background(), nil, p); !errors.Is(err, ErrNguonKhongHopLe) {
		t.Errorf("err = %v, muốn ErrNguonKhongHopLe", err)
	}
}

func TestThuHoiDoiDuLyDo(t *testing.T) {
	s := &PhienCongDanStore{}
	ctx := context.Background()

	if err := s.ThuHoi(ctx, nil, "", "đăng xuất"); !errors.Is(err, ErrThieuPhien) {
		t.Errorf("thu hồi không có sid: err = %v, muốn ErrThieuPhien", err)
	}
	// A revocation with no reason is an entry nobody can account for later, and the audit entry
	// the caller writes beside it is only as good as what it can say about why.
	if err := s.ThuHoi(ctx, nil, "SID-01", "   "); !errors.Is(err, ErrThieuLyDo) {
		t.Errorf("thu hồi không có lý do: err = %v, muốn ErrThieuLyDo", err)
	}
	if err := s.ThuHoi(ctx, nil, "SID-01", "đăng xuất"); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("thu hồi không kèm giao dịch: err = %v, muốn ErrThieuGiaoDich", err)
	}
	if err := s.ThuHoiChuaChonXa(ctx, nil, "SID-01", "đăng xuất"); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("thu hồi phiên chưa chọn xã: err = %v, muốn ErrThieuGiaoDich", err)
	}
	if err := s.ThuHoiCuaCongDan(ctx, nil, "", "khoá màn hình"); !errors.Is(err, ErrThieuCongDan) {
		t.Errorf("thu hồi theo công dân không có định danh: err = %v, muốn ErrThieuCongDan", err)
	}
	if err := s.ThuHoiCuaCongDan(ctx, nil, "CD-01", "khoá màn hình"); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("thu hồi theo công dân không kèm giao dịch: err = %v, muốn ErrThieuGiaoDich", err)
	}
}

func TestTraCuuTuChoiTokenRongTruocKhiChamCSDL(t *testing.T) {
	// A store with no connection would panic on any query. It does not, which is the assertion:
	// an empty Authorization header never reaches the database.
	if _, ok := (&PhienCongDanStore{}).TraCuu(context.Background(), ""); ok {
		t.Error("token rỗng mà vẫn tra ra phiên")
	}
}

func TestDungSoPhienCanCaKetNoiLanSoGhi(t *testing.T) {
	// Fail at wiring time. The alternative is a nil dereference on the first citizen request of
	// the day, in the code path that runs before anything else.
	defer func() {
		if r := recover(); r == nil {
			t.Error("dựng sổ phiên thiếu phụ thuộc mà không panic")
		}
	}()
	_ = NewPhienCongDanStore(nil, nil)
}

func TestLoiKhongMangDinhDanhHayToken(t *testing.T) {
	// An error travels into logs, into monitoring, sometimes onto a screen. A token substitutes
	// for a whole session and a citizen identifier is the key to their files (rule 3).
	for _, err := range []error{
		ErrThieuCongDan, ErrNguonKhongHopLe, ErrThieuThoiHan,
		ErrThieuGiaoDich, ErrThieuPhien, ErrThieuLyDo,
	} {
		if strings.ContainsAny(err.Error(), "0123456789") {
			t.Errorf("lỗi dựng sẵn chứa chữ số — dễ bị đọc là một giá trị thật: %v", err)
		}
	}
}

func TestCatNgan(t *testing.T) {
	// An unbounded client-supplied string in a government database is a liability, not a
	// feature. The device label is the one field a client fully controls.
	if got := catNgan(strings.Repeat("a", 250), 200); len(got) != 200 {
		t.Errorf("chuỗi dài bị cắt còn %d ký tự, muốn 200", len(got))
	}
	if got := catNgan("zmp-screen", 200); got != "zmp-screen" {
		t.Errorf("chuỗi ngắn bị sửa: %q", got)
	}
}

func TestCatNganKhongCatGiuaMotKyTu(t *testing.T) {
	// A Vietnamese device label is multi-byte UTF-8. Half a character is INVALID UTF-8, and
	// PostgreSQL refuses it outright — the insert fails, and because the audit entry shares that
	// transaction the whole sign-in rolls back. A citizen would be unable to sign in because of
	// the name of their phone.
	//
	// "ế" is three bytes, so a 200-byte cut of this string lands mid-character.
	nhan := strings.Repeat("ế", 100)
	got := catNgan(nhan, 200)
	if !utf8.ValidString(got) {
		t.Errorf("cắt xong không còn là UTF-8 hợp lệ: %q", got)
	}
	if len(got) > 200 {
		t.Errorf("cắt xong vẫn dài %d byte", len(got))
	}
}

// The interface assertion in phien_cong_dan.go is checked at compile time; this pins the shape
// the edge actually consumes, so a change to CitizenSession's fields shows up as a failing test
// here as well as a failing build.
func TestCaiDatDungInterfaceRiaCongDan(t *testing.T) {
	var so httpx.CitizenSessions = &PhienCongDanStore{}
	if _, ok := so.TraCuu(context.Background(), ""); ok {
		t.Error("TraCuu với token rỗng phải trả ok=false")
	}
}
