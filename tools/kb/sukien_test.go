package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vietTep(t *testing.T, goc, rel, noiDung string) {
	t.Helper()
	p := filepath.Join(goc, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(noiDung), 0o644); err != nil {
		t.Fatal(err)
	}
}

const protoThu = `syntax = "proto3";

package vigov.thu.v1;

// Chú thích đầu tệp, cách message một dòng trống — KHÔNG được coi là tên của message nào.
//
//     thu.khong_phai_ten.v9

// ThuDaXong is the payload of the event named:
//
//     thu.da_xong.v1
//
// Vài dòng lý do nữa.
message ThuDaXong {
  string lookup_code = 1;

  ThuKemTheo kem = 2;
}

// ThuKemTheo là hình dạng con, KHÔNG phải một sự kiện: nó không mang tên nào và được dùng làm
// kiểu trường ở trên.
message ThuKemTheo {
  string nhan = 1;
}
`

// Tên sự kiện đọc từ đúng chỗ .proto khai nó, và message con không bị nhận nhầm là sự kiện.
func TestSuKienTuProto(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu)

	sk, canhBao, err := suKienTuProto(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk) != 1 {
		t.Fatalf("chờ đúng 1 sự kiện, gặp %d: %+v", len(sk), sk)
	}
	if sk[0].Ten != "thu.da_xong.v1" {
		t.Errorf("tên sai: %q", sk[0].Ten)
	}
	if sk[0].Owner != "thu" || sk[0].Version != "v1" {
		t.Errorf("xuất xứ sai: owner=%q version=%q", sk[0].Owner, sk[0].Version)
	}
	if sk[0].Schema != "proto/vigov/thu/v1/events.proto#ThuDaXong" {
		t.Errorf("lược đồ sai: %q", sk[0].Schema)
	}
	if len(canhBao) != 0 {
		t.Errorf("không chờ cảnh báo nào: %v", canhBao)
	}
}

// KHỐI CHÚ THÍCH CÁCH MỘT DÒNG TRỐNG KHÔNG PHẢI CHÚ THÍCH CỦA MESSAGE BÊN DƯỚI.
//
// Nếu không cắt ở dòng trống thì một message mượn được tên viết cho thứ khác — và tệp sinh ra
// công bố một sự kiện không ai phát, dưới một cái tên không ai khai. Cùng kỷ luật `apidoc` áp
// cho chú thích route: DÍNH LIỀN, không cách dòng.
func TestChuThichCachDongTrongThiKhongPhaiTenCuaMessage(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", `syntax = "proto3";

package vigov.thu.v1;

// ThuDaXong is the payload of the event named:
//
//     thu.da_xong.v1
message ThuDaXong {
  string a = 1;
}

//     thu.khong_phai_ten.v9

// ThuKhac có chú thích riêng, và chú thích ấy KHÔNG mang tên sự kiện nào.
message ThuKhac {
  string b = 1;
}
`)
	sk, _, err := suKienTuProto(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk) != 1 {
		t.Fatalf("chờ đúng 1 sự kiện, gặp %d: %+v", len(sk), sk)
	}
	if sk[0].Ten != "thu.da_xong.v1" {
		t.Errorf("tên sai: %q", sk[0].Ten)
	}
}

// MESSAGE LỒNG BÊN TRONG MỘT MESSAGE KHÁC KHÔNG PHẢI MỘT SỰ KIỆN.
//
// `message X {` sau khi cắt khoảng trắng thì trông y hệt ở mọi độ sâu, nên thứ duy nhất phân
// biệt được là độ sâu ngoặc. Không đếm thì một hình dạng con mang chú thích có dạng tên sẽ
// thành một sự kiện thứ hai không ai phát.
func TestMessageLongNhauKhongPhaiSuKien(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", `syntax = "proto3";

package vigov.thu.v1;

// ThuDaXong is the payload of the event named:
//
//     thu.da_xong.v1
message ThuDaXong {
  // Hình dạng con, viết lồng bên trong. Dòng dưới KHÔNG khai một sự kiện:
  //
  //     thu.long_nhau.v1
  message Con {
    string a = 1;
  }

  Con con = 1;
}
`)
	sk, _, err := suKienTuProto(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk) != 1 || sk[0].Ten != "thu.da_xong.v1" {
		t.Fatalf("chờ đúng sự kiện cấp một, gặp: %+v", sk)
	}
}

// Một message cấp một không mang tên sự kiện VÀ không được dùng làm kiểu trường: báo, chứ không
// lặng lẽ bỏ qua — bỏ qua trong im lặng đúng là cách tệp này rỗng suốt từ đầu.
func TestMessageMoCoiThiBao(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu+`
// Không tên, không ai dùng.
message ThuBoRoi {
  string a = 1;
}
`)
	_, canhBao, err := suKienTuProto(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(canhBao) != 1 || !strings.Contains(canhBao[0], "ThuBoRoi") {
		t.Fatalf("chờ một cảnh báo về ThuBoRoi, gặp: %v", canhBao)
	}
}

// Tên sự kiện không cùng miền với gói proto là một cái tên không dịch vụ nào sở hữu.
func TestTenLechMienThiBao(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", `syntax = "proto3";

package vigov.thu.v1;

// Sự kiện mang tên của dịch vụ khác:
//
//     khac.da_xong.v1
message ThuDaXong {
  string a = 1;
}
`)
	_, canhBao, err := suKienTuProto(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(canhBao) == 0 || !strings.Contains(canhBao[0], "không cùng miền") {
		t.Fatalf("chờ cảnh báo lệch miền, gặp: %v", canhBao)
	}
}

// CHƯA CÓ BÊN PHÁT PHẢI ĐƯỢC NÓI RA THÀNH LỜI. Hai danh sách rỗng đọc được theo hai nghĩa —
// "chưa ai" và "chưa ai đi tìm" — và người mở tệp này đang hỏi "đổi cái này thì ai vỡ".
func TestChuaCoBenNaoThiNoiRa(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu)

	sk, _, err := quetSuKien(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk) != 1 {
		t.Fatalf("chờ 1 sự kiện, gặp %d", len(sk))
	}
	if len(sk[0].BenPhat) != 0 || len(sk[0].BenNhan) != 0 {
		t.Fatalf("kho giả không có mã Go nào: %+v", sk[0])
	}
	noi := strings.Join(sk[0].TinhTrang, " | ")
	if !strings.Contains(noi, "CHƯA CÓ BÊN PHÁT") {
		t.Errorf("thiếu câu nói rõ chưa có bên phát: %q", noi)
	}
	if !strings.Contains(noi, "CHƯA CÓ BÊN NHẬN") {
		t.Errorf("thiếu câu nói rõ chưa có bên nhận: %q", noi)
	}
}

// Bên phát và bên nhận đọc từ mã Go thật, không từ một dấu khai báo viết tay.
func TestBenPhatVaBenNhanTuMaGo(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu)
	vietTep(t, goc, "service-thu/internal/event/phat.go", `package event

import "vd/core/events"

func Phat() events.Envelope {
	return events.Envelope{Name: "thu.da_xong.v1"}
}
`)
	vietTep(t, goc, "service-khac/internal/event/nghe.go", `package event

func DinhTuyen(ten string) {
	switch ten {
	case "thu.da_xong.v1":
	}
}
`)
	// Một tệp test KHÔNG được tính là bên phát: nó dựng thông điệp để kiểm, không phải để gửi.
	vietTep(t, goc, "service-khac/internal/event/nghe_test.go", `package event

import "vd/core/events"

func dung() events.Envelope { return events.Envelope{Name: "thu.da_xong.v1"} }
`)

	sk, canhBao, err := quetSuKien(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk[0].BenPhat) != 1 || sk[0].BenPhat[0].Service != "thu" {
		t.Fatalf("bên phát sai: %+v", sk[0].BenPhat)
	}
	if !strings.HasPrefix(sk[0].BenPhat[0].At, "service-thu/internal/event/phat.go:") {
		t.Errorf("vị trí bên phát sai: %q", sk[0].BenPhat[0].At)
	}
	if len(sk[0].BenNhan) != 1 || sk[0].BenNhan[0].Service != "khac" {
		t.Fatalf("bên nhận sai: %+v", sk[0].BenNhan)
	}
	if len(sk[0].ChuaRo) != 0 {
		t.Errorf("không chờ mốc chưa rõ vai: %+v", sk[0].ChuaRo)
	}
	if len(sk[0].TinhTrang) != 0 {
		t.Errorf("có cả hai bên rồi thì không còn câu 'chưa có': %v", sk[0].TinhTrang)
	}
	if len(canhBao) != 0 {
		t.Errorf("không chờ cảnh báo: %v", canhBao)
	}
}

// HÌNH DẠNG CỦA CONSUMER THẬT: một hằng số cấp gói giữ tên, rồi so tên phong bì với hằng ấy.
//
// Đây là cách ĐÚNG để viết — một chỗ duy nhất giữ tên sự kiện — nên một bộ sinh chỉ nhìn chuỗi
// viết thẳng sẽ đọc chính đoạn mã tốt nhất thành "không rõ vai", và tệp sinh ra sẽ nói "chưa có
// bên nhận" trong khi bên nhận đang nằm ngay đó.
func TestBenNhanQuaHangSoVaPhepSoTen(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu)
	vietTep(t, goc, "service-khac/internal/event/nhan.go", `package event

import "vd/core/events"

const TenSuKienThu = "thu.da_xong.v1"

func Nhan(e events.Envelope) error {
	if e.Name != TenSuKienThu {
		return nil
	}
	return nil
}
`)
	sk, canhBao, err := quetSuKien(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk[0].BenNhan) != 1 || sk[0].BenNhan[0].Service != "khac" {
		t.Fatalf("bên nhận sai: %+v", sk[0].BenNhan)
	}
	// Chỗ khai hằng số KHÔNG được báo lại như một mốc chưa rõ vai: cùng một sự thật, kể hai lần.
	if len(sk[0].ChuaRo) != 0 {
		t.Errorf("không chờ mốc chưa rõ vai: %+v", sk[0].ChuaRo)
	}
	if len(canhBao) != 0 {
		t.Errorf("không chờ cảnh báo: %v", canhBao)
	}
}

// Một hình dạng bộ sinh chưa biết KHÔNG được biến mất: nó hiện ra kèm cảnh báo. Đó là khác biệt
// giữa bộ sinh này và tệp rỗng nó thay thế.
func TestNhacTenMaKhongRoVaiThiBao(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, "proto/vigov/thu/v1/events.proto", protoThu)
	vietTep(t, goc, "service-thu/internal/event/la.go", `package event

const TenSuKien = "thu.da_xong.v1"
`)
	sk, canhBao, err := quetSuKien(goc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sk[0].ChuaRo) != 1 {
		t.Fatalf("chờ 1 mốc chưa rõ vai: %+v", sk[0].ChuaRo)
	}
	if len(canhBao) == 0 || !strings.Contains(canhBao[0], "không nhận ra vai trò") {
		t.Fatalf("chờ cảnh báo về mốc chưa rõ vai, gặp: %v", canhBao)
	}
}

// Sự kiện ĐẦU TIÊN của kho thật, đọc từ chính .proto. Không phải golden file: thứ cần ghim là
// tệp sinh nói ĐÚNG những gì nguồn chuẩn khai.
func TestKhoThatCoSuKienDauTien(t *testing.T) {
	goc, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	sk, _, err := quetSuKien(goc)
	if err != nil {
		t.Fatal(err)
	}
	var thay *luongSuKien
	for i := range sk {
		if sk[i].Ten == "petitions.status_changed.v1" {
			thay = &sk[i]
		}
	}
	if thay == nil {
		t.Fatalf("không thấy petitions.status_changed.v1 trong %d sự kiện", len(sk))
	}
	if thay.Schema != "proto/vigov/petitions/v1/events.proto#PetitionStatusChanged" {
		t.Errorf("lược đồ sai: %q", thay.Schema)
	}
	if thay.Owner != "petitions" || thay.Version != "v1" {
		t.Errorf("xuất xứ sai: %q %q", thay.Owner, thay.Version)
	}
}
