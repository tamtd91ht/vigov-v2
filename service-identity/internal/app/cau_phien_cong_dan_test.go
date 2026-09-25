package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// The citizen-session bridge use case (ADR 0045), on the fake driver of dang_nhap_giao_dich_test.go
// for the transaction and audit_log, and on Go fakes for the stores — the SQL of those stores runs
// in internal/store/cau_phien_pg_test.go (SKIPS without VIGOV_TEST_DSN).
//
// What is under test here: each row of ADR 0045 §Chế độ, the transaction boundary (one transaction,
// every entry inside it, rolled back together), the switch that revokes the old sessions, and that
// neither the phone number nor the Zalo id reaches an audit entry or a log line.

const (
	cauAppChinh = "app-chinh-1234"
	cauAppRieng = "app-rieng-5678"
	cauMaZalo   = "zalo-user-GIA-0000000000"
	cauIP       = "10.0.0.9"
)

// --- fakes --------------------------------------------------------------------------------------

type cauNenTangGia struct {
	apps   map[string]platformclient.MiniApp
	xa     map[tenant.ID]tenant.Tenant
	loiApp error
	loiXa  error
}

func (n *cauNenTangGia) MiniApp(_ context.Context, appID string) (platformclient.MiniApp, bool, error) {
	if n.loiApp != nil {
		return platformclient.MiniApp{}, false, n.loiApp
	}
	a, ok := n.apps[appID]
	return a, ok, nil
}

func (n *cauNenTangGia) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) {
	if n.loiXa != nil {
		return tenant.Tenant{}, false, n.loiXa
	}
	t, ok := n.xa[tenant.MustFrom(ctx)]
	return t, ok, nil
}

type cauKhoGia struct {
	mu          sync.Mutex
	taiKhoan    map[string]crosstenant.TaiKhoanZalo // key: app|ma
	dinhDanh    map[string]string                   // phone -> identity id
	phien       []cauPhienDaGhi
	cacTx       map[*sql.Tx]bool
	docLan      int
	loiTaoPhien error
	// xaDaNhoTrongTx, when set, is what the LOCKED read returns — simulating a switch that committed
	// between the pre-transaction read and the lock.
	xaDaNhoTrongTx *tenant.ID
}

type cauPhienDaGhi struct {
	sid, xa, taiKhoan, congDan string
	thuHoi                     bool
}

func moiKhoGia() *cauKhoGia {
	return &cauKhoGia{taiKhoan: map[string]crosstenant.TaiKhoanZalo{}, dinhDanh: map[string]string{},
		cacTx: map[*sql.Tx]bool{}}
}

func (k *cauKhoGia) Doc(_ context.Context, appID, ma string) (crosstenant.TaiKhoanZalo, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.docLan++
	tk, ok := k.taiKhoan[appID+"|"+ma]
	return tk, ok, nil
}

func (k *cauKhoGia) TimHoacTao(_ context.Context, tx *sql.Tx, appID, ma string) (crosstenant.TaiKhoanZalo, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.cacTx[tx] = true
	khoa := appID + "|" + ma
	tk, ok := k.taiKhoan[khoa]
	if !ok {
		tk = crosstenant.TaiKhoanZalo{ID: "TKZALO" + strings.Repeat("0", 19) + string(rune('A'+len(k.taiKhoan)))}
		k.taiKhoan[khoa] = tk
	}
	if k.xaDaNhoTrongTx != nil {
		tk.XaDaNho = *k.xaDaNhoTrongTx
	}
	return tk, !ok, nil
}

func (k *cauKhoGia) TroToiDinhDanh(_ context.Context, tx *sql.Tx, id, congDan string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.cacTx[tx] = true
	for khoa, tk := range k.taiKhoan {
		if tk.ID == id {
			tk.CongDanID = congDan
			k.taiKhoan[khoa] = tk
		}
	}
	return nil
}

func (k *cauKhoGia) NhoXaCuaGiaoDich(_ context.Context, tx *store.ScopedTx, id string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.cacTx[tx.Underlying()] = true
	for khoa, tk := range k.taiKhoan {
		if tk.ID == id {
			tk.XaDaNho = tx.TenantID()
			k.taiKhoan[khoa] = tk
		}
	}
	return nil
}

func (k *cauKhoGia) TimHoacTaoDD(so string) string { return k.dinhDanh[so] }

type cauDinhDanhGia struct{ k *cauKhoGia }

func (d cauDinhDanhGia) TimHoacTao(_ context.Context, tx *sql.Tx, so string) (crosstenant.DinhDanh, bool, error) {
	d.k.mu.Lock()
	defer d.k.mu.Unlock()
	d.k.cacTx[tx] = true
	id, ok := d.k.dinhDanh[so]
	if !ok {
		id = "CD" + strings.Repeat("0", 23) + string(rune('A'+len(d.k.dinhDanh)))
		d.k.dinhDanh[so] = id
	}
	return crosstenant.DinhDanh{ID: id}, !ok, nil
}

type cauPhienGia struct{ k *cauKhoGia }

func (p cauPhienGia) TaoQuaCau(_ context.Context, tx *store.ScopedTx, m idstore.PhienCauMoi) (string, string, time.Time, error) {
	p.k.mu.Lock()
	defer p.k.mu.Unlock()
	if p.k.loiTaoPhien != nil {
		return "", "", time.Time{}, p.k.loiTaoPhien
	}
	p.k.cacTx[tx.Underlying()] = true
	sid := "SID-" + string(rune('A'+len(p.k.phien)))
	p.k.phien = append(p.k.phien, cauPhienDaGhi{sid: sid, xa: string(tx.TenantID()),
		taiKhoan: m.TaiKhoanZaloID, congDan: m.CongDanID})
	return sid, "TOKEN-" + sid, time.Now().Add(m.ThoiHan), nil
}

func (p cauPhienGia) ThuHoiCuaTaiKhoanZalo(_ context.Context, tx *store.ScopedTx, id, _ string) (int64, error) {
	p.k.mu.Lock()
	defer p.k.mu.Unlock()
	p.k.cacTx[tx.Underlying()] = true
	var n int64
	for i := range p.k.phien {
		if p.k.phien[i].taiKhoan == id && !p.k.phien[i].thuHoi {
			p.k.phien[i].thuHoi = true
			n++
		}
	}
	return n, nil
}

type banThuCau struct {
	uc  *CauPhienCongDan
	g   *ghiChep
	k   *cauKhoGia
	nt  *cauNenTangGia
	log *bytes.Buffer
}

func dungBanThuCau(t *testing.T) banThuCau {
	t.Helper()
	db, g := moDB(t)
	k := moiKhoGia()
	nt := &cauNenTangGia{
		apps: map[string]platformclient.MiniApp{
			cauAppChinh: {AppID: cauAppChinh, CheDo: platformclient.CheDoAppChinh},
			cauAppRieng: {AppID: cauAppRieng, CheDo: platformclient.CheDoAppRieng, XaRieng: xaThu},
		},
		xa: map[tenant.ID]tenant.Tenant{
			xaThu:  {ID: xaThu, Name: "Xã Thăng Bình", Active: true},
			xaKhac: {ID: xaKhac, Name: "Xã Bình Dương", Active: true},
		},
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := NewCauPhienCongDan(db, nt, k, cauDinhDanhGia{k}, cauPhienGia{k}, 30*24*time.Hour, log)
	return banThuCau{uc: uc, g: g, k: k, nt: nt, log: &buf}
}

func (b banThuCau) vet(hanhDong string) []lenhGhi {
	b.g.mu.Lock()
	defer b.g.mu.Unlock()
	var ra []lenhGhi
	for _, l := range b.g.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			for _, a := range l.args {
				if s, ok := a.(string); ok && s == hanhDong {
					ra = append(ra, l)
				}
			}
		}
	}
	return ra
}

func yeuCauCau(app string) YeuCauMoPhienCau {
	return YeuCauMoPhienCau{AppID: app, MaZalo: cauMaZalo, IP: cauIP, ThietBi: "Zalo/test"}
}

// --- rows of ADR 0045 §Chế độ --------------------------------------------------------------------

func TestCauAppKhongDangKyThiTuChoi(t *testing.T) {
	b := dungBanThuCau(t)
	_, err := b.uc.Mo(context.Background(), yeuCauCau("app-la"))
	if !errors.Is(err, ErrCauAppChuaSanSang) {
		t.Fatalf("err = %v, muốn ErrCauAppChuaSanSang (FAILED_PRECONDITION)", err)
	}
	if b.g.soGiaoDich() != 0 {
		t.Fatal("app chưa đăng ký mà đã mở giao dịch")
	}
}

func TestCauAppRiengTheoXaCuaAppBoQuaGoiY(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppRieng)
	yc.GoiYXa, yc.DaXacNhanXa = string(xaKhac), true

	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.Xa != xaThu || kq.TenXa != "Xã Thăng Bình" || kq.Token == "" || kq.CheDo != domain.CheDoAppRieng {
		t.Fatalf("kết quả = %+v — app riêng phải theo xã của app, bỏ qua t=", kq)
	}
	for _, tk := range b.k.taiKhoan {
		if tk.XaDaNho != "" {
			t.Fatal("app riêng ghi xã đã nhớ — chỉ app chính nhớ xã")
		}
	}
}

func TestCauAppRiengXaNgungThiTuChoiKhongDiTheoKeThua(t *testing.T) {
	b := dungBanThuCau(t)
	// Platform: binding exists but its commune is inactive → XaRieng absent.
	b.nt.apps[cauAppRieng] = platformclient.MiniApp{AppID: cauAppRieng, CheDo: platformclient.CheDoAppRieng}
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppRieng)); !errors.Is(err, ErrCauAppChuaSanSang) {
		t.Fatalf("err = %v, muốn ErrCauAppChuaSanSang", err)
	}
	// And when the platform still names it but GetTenant says inactive.
	b.nt.apps[cauAppRieng] = platformclient.MiniApp{AppID: cauAppRieng, CheDo: platformclient.CheDoAppRieng, XaRieng: xaThu}
	b.nt.xa[xaThu] = tenant.Tenant{ID: xaThu, Active: false}
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppRieng)); !errors.Is(err, ErrCauAppChuaSanSang) {
		t.Fatalf("err = %v, muốn ErrCauAppChuaSanSang", err)
	}
	if b.g.soGiaoDich() != 0 {
		t.Fatal("xã không hoạt động mà đã mở giao dịch")
	}
}

func TestCauAppChinhXacNhanQRThiVaoXaVaNhoXa(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa = string(xaThu), true

	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.Xa != xaThu || kq.Token == "" || kq.CheDo != domain.CheDoAppChinh {
		t.Fatalf("kết quả = %+v", kq)
	}
	tk := b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo]
	if tk.XaDaNho != xaThu {
		t.Fatalf("xã đã nhớ = %q, muốn %q", tk.XaDaNho, xaThu)
	}
	if len(b.vet(HanhDongDoiXaDaNho)) != 1 {
		t.Fatal("xã đã nhớ đổi mà không có mục vết doi_xa_da_nho")
	}
}

func TestCauAppChinhXacNhanXaNgungThiTuChoi(t *testing.T) {
	b := dungBanThuCau(t)
	b.nt.xa[xaKhac] = tenant.Tenant{ID: xaKhac, Active: false}
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa = string(xaKhac), true
	if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauXaKhongHoatDong) {
		t.Fatalf("err = %v, muốn ErrCauXaKhongHoatDong", err)
	}
	// Unknown to the registry is refused the same way.
	delete(b.nt.xa, xaKhac)
	if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauXaKhongHoatDong) {
		t.Fatalf("xã không có trong sổ: err = %v", err)
	}
}

func TestCauAppChinhChuaXacNhanTheoXaDaNhoVaBoQuaGoiY(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo] = crosstenant.TaiKhoanZalo{ID: "TK-A", XaDaNho: xaThu}
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa = string(xaKhac) // QR of another commune, NOT confirmed

	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.Xa != xaThu || kq.Token == "" {
		t.Fatalf("kết quả = %+v — chưa xác nhận thì theo xã đã nhớ, KHÔNG theo t=", kq)
	}
	if b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo].XaDaNho != xaThu {
		t.Fatal("xã đã nhớ bị đổi khi chưa xác nhận")
	}
	if len(b.vet(HanhDongDoiXaDaNho)) != 0 {
		t.Fatal("có vết đổi xã khi xã không đổi")
	}
}

func TestCauAppChinhXaDaNhoNgungThiKhongXaKhongGhiGi(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo] = crosstenant.TaiKhoanZalo{ID: "TK-A", XaDaNho: xaThu}
	b.nt.xa[xaThu] = tenant.Tenant{ID: xaThu, Active: false}

	kq, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppChinh))
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.Xa != "" || kq.Token != "" || kq.Sid != "" || kq.TenXa != "" {
		t.Fatalf("kết quả = %+v — xã đã nhớ ngừng hoạt động thì KHÔNG xã, không token", kq)
	}
	if b.g.soGiaoDich() != 0 || len(b.k.phien) != 0 {
		t.Fatal("không có xã mà vẫn ghi — câu #25 chưa có bảng vết, không được phát phiên không xã")
	}
}

func TestCauAppChinhChuaNhoGiThiKhongXaKhongPhienKhongGhi(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa = string(xaThu)      // unconfirmed QR
	yc.SoDaXacThuc = "84900000000" // even with a verified phone, nothing is written

	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.Xa != "" || kq.Token != "" || kq.DaCoSo {
		t.Fatalf("kết quả = %+v", kq)
	}
	if b.g.soGiaoDich() != 0 || len(b.k.taiKhoan) != 0 || len(b.k.dinhDanh) != 0 {
		t.Fatal("không có xã mà đã ghi tài khoản / danh tính — ghi không vết")
	}
}

// --- switch, phone, transaction -----------------------------------------------------------------

func TestCauDoiXaThuHoiNgayPhienXaCuVaGhiVetHaiXa(t *testing.T) {
	b := dungBanThuCau(t)
	ctx := context.Background()

	// First: confirmed into xaThu.
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa = string(xaThu), true
	cu, err := b.uc.Mo(ctx, yc)
	if err != nil {
		t.Fatal(err)
	}
	// Then: QR of xaKhac, confirmed.
	yc.GoiYXa = string(xaKhac)
	moi, err := b.uc.Mo(ctx, yc)
	if err != nil {
		t.Fatal(err)
	}

	if moi.Xa != xaKhac || moi.Sid == cu.Sid {
		t.Fatalf("phiên mới = %+v — đổi xã phải là phiên MỚI ở xã mới", moi)
	}
	for _, p := range b.k.phien {
		if p.sid == cu.Sid && !p.thuHoi {
			t.Fatal("phiên xã cũ CÒN SỐNG sau khi đổi xã — mỗi lúc một phiên (trả lời CÒN MỞ #3)")
		}
		if p.sid == moi.Sid && p.thuHoi {
			t.Fatal("phiên mới bị thu hồi")
		}
	}
	vet := b.vet(HanhDongDoiXaDaNho)
	if len(vet) != 2 {
		t.Fatalf("số mục doi_xa_da_nho = %d, muốn 2 (vào xã đầu, rồi đổi)", len(vet))
	}
	cuoi := vet[1]
	if cuoi.args[0] != string(xaKhac) {
		t.Errorf("mục đổi xã ghi ở xã %v, muốn xã MỚI", cuoi.args[0])
	}
	delta := string(cuoi.args[7].([]byte))
	if !strings.Contains(delta, `"truoc":"`+string(xaThu)+`"`) || !strings.Contains(delta, `"sau":"`+string(xaKhac)+`"`) ||
		!strings.Contains(delta, `"so_phien_thu_hoi":1`) {
		t.Errorf("nội dung vết đổi xã = %s — phải có xã cũ, xã mới và số phiên đã thu hồi", delta)
	}
}

func TestCauCoSoThiLienKetDinhDanhVaLanSauKhongCanSo(t *testing.T) {
	b := dungBanThuCau(t)
	ctx := context.Background()

	yc := yeuCauCau(cauAppRieng)
	dau, err := b.uc.Mo(ctx, yc)
	if err != nil {
		t.Fatal(err)
	}
	if dau.DaCoSo {
		t.Fatal("chưa gửi số mà phiên báo đã có số")
	}
	if b.k.phien[0].congDan != "" {
		t.Fatal("phiên chưa có số lại mang định danh công dân")
	}

	yc.SoDaXacThuc = "84900000000"
	coSo, err := b.uc.Mo(ctx, yc)
	if err != nil {
		t.Fatal(err)
	}
	if !coSo.DaCoSo || b.k.phien[1].congDan == "" {
		t.Fatalf("có số đã xác thực mà phiên không gắn danh tính: %+v", coSo)
	}
	if len(b.vet(HanhDongLienKetDinhDanh)) != 1 {
		t.Fatal("liên kết danh tính không có mục vết")
	}

	// Silent reopen: no phone token this time, and the session still carries the identity.
	yc.SoDaXacThuc = ""
	lanSau, err := b.uc.Mo(ctx, yc)
	if err != nil {
		t.Fatal(err)
	}
	if !lanSau.DaCoSo || b.k.phien[2].congDan != b.k.phien[1].congDan {
		t.Fatal("mở lại không số mà mất danh tính — trả lời CÒN MỞ #2: số xin MỘT lần")
	}
	if len(b.vet(HanhDongLienKetDinhDanh)) != 1 {
		t.Fatal("mở lại mà ghi thêm vết liên kết dù danh tính không đổi")
	}
}

func TestCauMotGiaoDichMoiMucVetBenTrong(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa, yc.SoDaXacThuc = string(xaThu), true, "84900000000"

	if _, err := b.uc.Mo(context.Background(), yc); err != nil {
		t.Fatal(err)
	}
	if b.g.soGiaoDich() != 1 {
		t.Fatalf("số giao dịch = %d, muốn 1 — mọi ghi phải chung MỘT giao dịch (luật 6 bất biến 3)", b.g.soGiaoDich())
	}
	if len(b.k.cacTx) != 1 {
		t.Fatalf("các kho ghi trên %d giao dịch khác nhau, muốn 1", len(b.k.cacTx))
	}
	for _, h := range []string{HanhDongLienKetDinhDanh, HanhDongDoiXaDaNho, HanhDongMoPhienCongDan} {
		v := b.vet(h)
		if len(v) != 1 || v[0].tx != 1 {
			t.Fatalf("mục %s: %d mục, giao dịch %v — muốn đúng 1, trong giao dịch 1", h, len(v), v)
		}
	}
	if b.g.ketThucCua(1) != "commit" {
		t.Fatalf("giao dịch kết thúc = %q", b.g.ketThucCua(1))
	}
}

func TestCauGhiPhienHongThiKhongGiDuocGiu(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.loiTaoPhien = errors.New("ổ đĩa đầy")
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa = string(xaThu), true

	if _, err := b.uc.Mo(context.Background(), yc); err == nil {
		t.Fatal("ghi phiên hỏng mà vẫn trả thành công")
	}
	if b.g.ketThucCua(1) != "rollback" {
		t.Fatalf("giao dịch kết thúc = %q, muốn rollback — vết và xã đã nhớ phải đi cùng phiên", b.g.ketThucCua(1))
	}
}

func TestCauNenTangHongThiKhongDoanXa(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo] = crosstenant.TaiKhoanZalo{ID: "TK-A", XaDaNho: xaThu}

	b.nt.loiApp = errors.New("platform down")
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppChinh)); !errors.Is(err, ErrCauNenTang) {
		t.Fatalf("ResolveMiniApp hỏng: err = %v, muốn ErrCauNenTang (UNAVAILABLE)", err)
	}
	b.nt.loiApp, b.nt.loiXa = nil, errors.New("platform down")
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppChinh)); !errors.Is(err, ErrCauNenTang) {
		t.Fatalf("GetTenant hỏng: err = %v, muốn ErrCauNenTang — KHÔNG dùng xã nhớ lần trước", err)
	}
	if b.g.soGiaoDich() != 0 {
		t.Fatal("nền tảng không tới được mà vẫn ghi")
	}
}

func TestCauXaDaNhoVuaDoiThiHuyKhongPhatPhienXaCu(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo] = crosstenant.TaiKhoanZalo{ID: "TK-A", XaDaNho: xaThu}
	khac := xaKhac
	b.k.xaDaNhoTrongTx = &khac // a confirmed switch committed between the read and the lock

	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppChinh)); !errors.Is(err, ErrCauXungDot) {
		t.Fatalf("err = %v, muốn ErrCauXungDot", err)
	}
	if len(b.k.phien) != 0 {
		t.Fatal("đã phát phiên ở xã công dân vừa rời đi")
	}
}

func TestCauYeuCauSaiLaInvalidArgument(t *testing.T) {
	b := dungBanThuCau(t)
	for ten, sua := range map[string]func(*YeuCauMoPhienCau){
		"thiếu app":               func(y *YeuCauMoPhienCau) { y.AppID = "" },
		"thiếu mã Zalo":           func(y *YeuCauMoPhienCau) { y.MaZalo = " " },
		"t= không phải ULID":      func(y *YeuCauMoPhienCau) { y.GoiYXa = "thang-binh" },
		"xác nhận mà không có t=": func(y *YeuCauMoPhienCau) { y.DaXacNhanXa = true },
	} {
		t.Run(ten, func(t *testing.T) {
			yc := yeuCauCau(cauAppChinh)
			sua(&yc)
			if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauYeuCauSai) {
				t.Fatalf("err = %v, muốn ErrCauYeuCauSai", err)
			}
		})
	}
}

func TestCauKhongIdempotentMoiLanMotPhien(t *testing.T) {
	// The contract says so: a retry issues a second session and a second entry. Pinned so nobody
	// "fixes" it into returning the first session's token, which would need the token stored.
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppRieng)
	a, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatal(err)
	}
	if a.Sid == c.Sid || a.Token == c.Token {
		t.Fatal("hai lần gọi trả cùng một phiên")
	}
	if len(b.vet(HanhDongMoPhienCongDan)) != 2 {
		t.Fatal("hai lần mở phiên phải là hai mục vết")
	}
}

func TestCauKhongDuaSoHayMaZaloVaoVetHayLog(t *testing.T) {
	b := dungBanThuCau(t)
	const so = "84900000000"
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYXa, yc.DaXacNhanXa, yc.SoDaXacThuc = string(xaThu), true, so
	if _, err := b.uc.Mo(context.Background(), yc); err != nil {
		t.Fatal(err)
	}
	// And a failing call, so the error-path log line is covered too.
	b.nt.loiXa = errors.New("platform down")
	_, _ = b.uc.Mo(context.Background(), yc)

	for _, s := range b.g.moiThamSo() {
		if strings.Contains(s, so) || strings.Contains(s, cauMaZalo) {
			t.Fatalf("dữ liệu cá nhân lọt vào câu lệnh ghi CSDL của vết: %q", s)
		}
	}
	for _, l := range b.g.lenh {
		for _, a := range l.args {
			if bs, ok := a.([]byte); ok && (bytes.Contains(bs, []byte(so)) || bytes.Contains(bs, []byte(cauMaZalo))) {
				t.Fatalf("dữ liệu cá nhân lọt vào nội dung vết: %s", bs)
			}
		}
	}
	if strings.Contains(b.log.String(), so) || strings.Contains(b.log.String(), cauMaZalo) {
		t.Fatalf("dữ liệu cá nhân lọt vào log: %s", b.log.String())
	}
}

func TestCauChuTheVetLaDinhDanhHoacTaiKhoanZalo(t *testing.T) {
	b := dungBanThuCau(t)
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppRieng)); err != nil {
		t.Fatal(err)
	}
	v := b.vet(HanhDongMoPhienCongDan)
	tk := b.k.taiKhoan[cauAppRieng+"|"+cauMaZalo]
	// args: tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta
	if v[0].args[1] != tk.ID || v[0].args[2] != "citizen" || v[0].args[3] != cauIP {
		t.Fatalf("chủ thể vết = %v/%v/%v — chưa có số thì là mã tài khoản Zalo, loại citizen, IP của bên cầu",
			v[0].args[1], v[0].args[2], v[0].args[3])
	}
}
