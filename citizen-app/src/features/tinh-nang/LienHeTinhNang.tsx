/**
 * HAI TÍNH NĂNG CỦA MÀN LIÊN HỆ — `getLocation` và `getPhoneNumber`.
 *
 * Cả hai đứng trên màn Liên hệ chứ không thành tab riêng, và đó là chủ đích: người mở màn Liên
 * hệ đang muốn nói chuyện với công ty. "Tìm văn phòng gần tôi" và "Đăng nhập bằng số Zalo" là
 * đúng hai việc họ định làm ở đó — một tab riêng tên "Quyền" thì nói với người duyệt rằng đây
 * là một app đi xin quyền, còn hai tính năng đặt đúng chỗ thì nói rằng đây là app có việc làm.
 *
 * ⚠ KHỐI `getPhoneNumber` LÀ KHỐI ĐĂNG NHẬP, KHÔNG PHẢI "ĐĂNG KÝ NHẬN TƯ VẤN" NỮA (ADR 0020):
 *
 *   Đăng nhập MỘT CHẠM bằng chính số Zalo. **Không OTP, không ô nhập sáu số, không màn chờ mã** —
 *   ADR 0020 bác đường ấy vì màn nhập sáu số là rào thật với người cao tuổi, và kênh này người
 *   dân không chọn. Ai định thêm một bước nhập mã vào đây phải mở lại ADR ấy trước.
 *
 *   Khối này có mặt ở CẢ HAI biến thể, **và cả hai đều gọi máy chủ thật** — kể cả BẢN NỘP. Một
 *   nút đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút nói "bản này chưa nối máy
 *   chủ" (điều 3.3.4). Máy chủ là `vihat-miniapp`, backend riêng của VihatSoftware.
 *
 * ⚠ RANH GIỚI THẬT, VÀ MÃ NÀY KHÔNG GIẢ VỜ VƯỢT QUA NÓ:
 *
 *   `getLocation` chỉ trả **token**; `latitude`/`longitude` đã `@deprecated` và tệp này không
 *   đọc chúng. `navigator.geolocation` thì bị cấm bởi dây bẫy trong
 *   `phase1-collects-nothing.test.ts` và cũng là một quyền khác hẳn. Nghĩa là **không thể xếp
 *   văn phòng theo khoảng cách trên máy** — nên ở đây không có một dòng nào tính khoảng cách, và
 *   màn hình nói ra điều đó bằng tiếng Việt (`VAN_PHONG.chua_xep_duoc`).
 *
 *   `getPhoneNumber` cũng chỉ trả token. Đổi token cần khoá bí mật của Mini App, và khoá ấy chỉ
 *   được nằm ở máy chủ (ADR 0020, bất biến 2) — nên không có bước đổi mã nào ở client, ở bất kỳ
 *   biến thể nào.
 *
 * ⚠ HAI ĐƯỜNG LIÊN HỆ Ở LẠI NGAY DƯỚI NÚT, VÀ CHÚNG KHÔNG PHẢI TRANG TRÍ: người từ chối chia sẻ
 * số điện thoại vẫn phải liên hệ được ngay. "Từ chối không làm mất tính năng" có ca test riêng.
 */
import { CONTACT, OFFICES } from "../../content/company-profile";
import { CompassGlyph, HandshakeGlyph, MailGlyph, PhoneGlyph, PinGlyph } from "../company-intro/icons";
import { PhatHanhPhien } from "../dang-nhap/PhatHanhPhien";

import { KetQuaToken, TinhNangCoTrangThai } from "./khung";
import { moRaNgoai } from "./mo-ra-ngoai";
import { DANG_NHAP, LOI_MO_NGOAI, VAN_PHONG } from "./noi-dung";
import { type MaDangNhap, xinMaDangNhap, xinTokenViTri } from "./zalo-api";
import { useState } from "react";

/**
 * Đường mở bản đồ cho một địa chỉ văn phòng.
 *
 * Địa chỉ là dữ liệu doanh nghiệp đã công bố (`content/company-profile.ts`), không phải dữ liệu
 * cá nhân — nên đưa nó vào một URL là hợp lệ. KHÔNG có gì của người dùng đi kèm: không toạ độ,
 * không token, không mã định danh nào. Bản đồ chỉ nhận đúng địa chỉ văn phòng.
 */
export function duongDanBanDo(dia_chi: string): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(dia_chi)}`;
}

/** Ba văn phòng, THUẦN — luôn hiện, kể cả khi người dùng từ chối chia sẻ vị trí. */
export function DanhSachVanPhong({ onChiDuong }: { onChiDuong: (dia_chi: string) => void }) {
  return (
    <ul className="van-phong">
      {OFFICES.map((van_phong) => (
        <li className="van-phong__mot" key={van_phong.name}>
          <PinGlyph className="van-phong__glyph" />
          <span className="van-phong__chu">
            <strong className="van-phong__ten">{van_phong.name}</strong>
            <span className="van-phong__dia-chi">{van_phong.address}</span>
          </span>
          <button
            type="button"
            className="tn-hanh-dong"
            onClick={() => onChiDuong(van_phong.address)}
          >
            {VAN_PHONG.nut_chi_duong}
          </button>
        </li>
      ))}
    </ul>
  );
}

export function TimVanPhong() {
  const [khong_mo_duoc, datKhongMoDuoc] = useState(false);

  // Qua CỬA KHAI BÁO (`mo-ra-ngoai.ts`), không gọi thẳng `moTrangWeb`: `"ban-do"` là một đích đã
  // khai trong `content/dich-ra-ngoai.ts`, tức một dòng đã có trong câu khai của chính sách.
  async function chiDuong(dia_chi: string) {
    datKhongMoDuoc(!(await moRaNgoai("ban-do", duongDanBanDo(dia_chi))));
  }

  return (
    <TinhNangCoTrangThai
      ma="van-phong"
      xin={xinTokenViTri}
      cap_tieu_de="h2"
      glyph={<CompassGlyph className="tn__glyph" />}
      dan_nhap={VAN_PHONG.dan_nhap}
      veKetQua={(token) => (
        <KetQuaToken
          ma="van-phong"
          token={token}
          da_nhan={VAN_PHONG.da_nhan_ma}
          noi_them={VAN_PHONG.chua_xep_duoc}
        />
      )}
      duoi_cung={
        <>
          <DanhSachVanPhong onChiDuong={(dia_chi) => void chiDuong(dia_chi)} />
          {khong_mo_duoc && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
        </>
      }
    />
  );
}

/**
 * Hai đường liên hệ chạy được ngay bây giờ, đặt ngay dưới nút đăng nhập.
 *
 * Chúng lặp lại hotline và email ở đầu màn Liên hệ, và việc lặp lại ấy là CÓ CHỦ ĐÍCH: người
 * vừa từ chối chia sẻ số điện thoại, hoặc vừa đọc câu "bản này chưa mở được phiên", phải thấy
 * ngay một đường khác ở đúng chỗ mắt họ đang nhìn. Bắt họ cuộn ngược lên là bắt một người đang
 * cần liên hệ tự đi tìm cách liên hệ.
 */
function DuongLienHeNgay() {
  return (
    <>
      <p className="tn__nhac">{DANG_NHAP.nhac_lien_he}</p>
      <div className="tn__doi-nut">
        <a className="tn-hanh-dong tn-hanh-dong--rong" href={`tel:${CONTACT.hotlineDialable}`}>
          <PhoneGlyph className="tn-hanh-dong__glyph" />
          Gọi {CONTACT.hotlineLabel}
        </a>
        <a className="tn-hanh-dong tn-hanh-dong--rong" href={`mailto:${CONTACT.email}`}>
          <MailGlyph className="tn-hanh-dong__glyph" />
          {CONTACT.email}
        </a>
      </div>
    </>
  );
}

/**
 * Kết quả một lần đăng nhập: mã nhận được, rồi bước máy chủ của ĐÚNG biến thể đang dựng.
 *
 * MÃ RỖNG THÌ DỪNG Ở ĐÂY, KHÔNG GỌI MÁY CHỦ. `zmp-sdk` trả mã rỗng ở môi trường phát triển
 * (`MA_RONG` nói ra điều đó). Gửi một mã rỗng đi thì máy chủ từ chối, và người đọc nhận một câu
 * "mã đã quá hạn" — một câu SAI, chỉ ra một việc phải làm không giải quyết được gì.
 */
function KetQuaDangNhap({ ma }: { ma: MaDangNhap }) {
  return (
    <>
      <KetQuaToken ma="dang-nhap" token={ma.ma_so_dien_thoai} da_nhan={DANG_NHAP.da_nhan_ma} />
      {ma.ma_so_dien_thoai !== "" && <PhatHanhPhien ma={ma} />}
    </>
  );
}

/**
 * KHỐI ĐĂNG NHẬP ĐỊNH DANH — `getPhoneNumber` + `getAccessToken`, một lần chạm.
 *
 * Vẫn đúng khuôn `TinhNangCoTrangThai` như năm khối kia: một tiêu đề, một lý do đọc được, một
 * nút, một chỗ hiện kết quả, và bốn nhánh — xong · từ chối · ngoài Zalo · không lấy được. Từ
 * chối là ĐƯỜNG BÌNH THƯỜNG (`code === -201`), không phải lỗi.
 */
export function KhoiDangNhap() {
  return (
    <TinhNangCoTrangThai
      ma="dang-nhap"
      xin={xinMaDangNhap}
      cap_tieu_de="h2"
      glyph={<HandshakeGlyph className="tn__glyph" />}
      dan_nhap={DANG_NHAP.dan_nhap}
      veKetQua={(ma) => <KetQuaDangNhap ma={ma} />}
      duoi_cung={<DuongLienHeNgay />}
    />
  );
}
