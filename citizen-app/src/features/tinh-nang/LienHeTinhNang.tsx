/**
 * KHỐI ĐĂNG NHẬP (`getPhoneNumber`) VÀ DANH SÁCH VĂN PHÒNG CỦA MÀN LIÊN HỆ.
 *
 * ⚠ VĂN PHÒNG KHÔNG XIN VỊ TRÍ NỮA (11/10/2026, chủ dự án: "xử lý luôn"). `getLocation` chỉ trả một
 * mã, và màn này chưa bao giờ đổi được nó thành toạ độ — nút "Tìm văn phòng gần tôi" xin quyền vị
 * trí mà không xếp được gì. Nay ba văn phòng hiện thẳng, mỗi văn phòng một nút "Chỉ đường": bản đồ
 * tự dùng vị trí của người dùng ở bước chỉ đường, và ứng dụng không cần biết họ đứng ở đâu.
 *
 * ⚠ KHỐI `getPhoneNumber` LÀ KHỐI ĐĂNG NHẬP, KHÔNG PHẢI "ĐĂNG KÝ NHẬN TƯ VẤN" NỮA (ADR 0020):
 *
 *   Đăng nhập MỘT CHẠM bằng chính số Zalo. **Không OTP, không ô nhập sáu số, không màn chờ mã** —
 *   ADR 0020 bác đường ấy vì màn nhập sáu số là rào thật với người cao tuổi, và kênh này người
 *   dân không chọn. Ai định thêm một bước nhập mã vào đây phải mở lại ADR ấy trước.
 *
 *   Khối này có mặt ở CẢ HAI biến thể, **và cả hai đều gọi máy chủ thật** — kể cả BẢN NỘP. Một
 *   nút đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút nói "bản này chưa nối máy
 *   chủ" (điều 3.3.4). Máy chủ là `vihat-miniapp`, backend riêng của Tập đoàn ViHAT Group
 *   (bên vận hành, câu mở #28 — 25/09/2026).
 *
 * ⚠ `getPhoneNumber` chỉ trả token. Đổi token cần khoá bí mật của Mini App, và khoá ấy chỉ
 *   được nằm ở máy chủ (ADR 0020, bất biến 2) — nên không có bước đổi mã nào ở client, ở bất kỳ
 *   biến thể nào.
 *
 * ⚠ HAI ĐƯỜNG LIÊN HỆ Ở LẠI NGAY DƯỚI NÚT, VÀ CHÚNG KHÔNG PHẢI TRANG TRÍ: người từ chối chia sẻ
 * số điện thoại vẫn phải liên hệ được ngay. "Từ chối không làm mất tính năng" có ca test riêng.
 */
import { CONTACT, OFFICES } from "../../content/company-profile";
import { CompassGlyph, HandshakeGlyph, MailGlyph, PhoneGlyph, PinGlyph } from "../company-intro/icons";
import { MOC_TIM_VAN_PHONG } from "../company-intro/dieu-huong";
import { PhatHanhPhien } from "../dang-nhap/PhatHanhPhien";

import { HotlineLink } from "./HotlineLink";
import { TinhNangCoTrangThai } from "./khung";
import { moRaNgoai } from "./mo-ra-ngoai";
import { DANG_NHAP, LOI_MO_NGOAI, MA_RONG, VAN_PHONG } from "./noi-dung";
import { type MaDangNhap, xinMaDangNhap } from "./zalo-api";
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

/** Ba văn phòng, THUẦN. */
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

  // Cùng khung chữ với các khối tính năng (`tn`), nhưng không có nút xin quyền nào. `id` của tiêu đề là
  // mỏ neo mục "Văn phòng" của menu nhanh (`MOC_TIM_VAN_PHONG`).
  return (
    <section className="tn hien-len" aria-labelledby={MOC_TIM_VAN_PHONG}>
      <div className="tn__dau">
        <span className="tn__huy-hieu" aria-hidden="true">
          <CompassGlyph className="tn__glyph" />
        </span>
        <h2 className="tn__tieu-de" id={MOC_TIM_VAN_PHONG}>
          {VAN_PHONG.tieu_de}
        </h2>
      </div>
      <DanhSachVanPhong onChiDuong={(dia_chi) => void chiDuong(dia_chi)} />
      {khong_mo_duoc && <p className="tn__loi">{LOI_MO_NGOAI}</p>}
    </section>
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
        {/* Through `HotlineLink`: inside Zalo a bare `tel:` does nothing (owner 12/10/2026). */}
        <HotlineLink className="tn-hanh-dong tn-hanh-dong--rong">
          <PhoneGlyph className="tn-hanh-dong__glyph" />
          Gọi {CONTACT.hotlineLabel}
        </HotlineLink>
        <a className="tn-hanh-dong tn-hanh-dong--rong" href={`mailto:${CONTACT.email}`}>
          <MailGlyph className="tn-hanh-dong__glyph" />
          {CONTACT.email}
        </a>
      </div>
    </>
  );
}

/**
 * Kết quả một lần đăng nhập: bước máy chủ đổi mã lấy phiên. Mã không hiện lên màn hình.
 *
 * MÃ RỖNG THÌ DỪNG Ở ĐÂY, KHÔNG GỌI MÁY CHỦ. `zmp-sdk` trả mã rỗng ở môi trường phát triển
 * (`MA_RONG` nói ra điều đó). Gửi một mã rỗng đi thì máy chủ từ chối, và người đọc nhận một câu
 * "mã đã quá hạn" — một câu SAI, chỉ ra một việc phải làm không giải quyết được gì.
 */
export function KetQuaDangNhap({ ma }: { ma: MaDangNhap }) {
  if (ma.ma_so_dien_thoai === "") return <p className="tn__loi">{MA_RONG}</p>;
  return <PhatHanhPhien ma={ma} />;
}

/**
 * KHỐI ĐĂNG NHẬP ĐỊNH DANH — `getPhoneNumber` + `getAccessToken`, một lần chạm.
 *
 * Vẫn đúng khuôn `TinhNangCoTrangThai` như năm khối kia: một tiêu đề, một lý do đọc được, một
 * nút, một chỗ hiện kết quả, và bốn nhánh — xong · từ chối · ngoài Zalo · không lấy được. Từ
 * chối là ĐƯỜNG BÌNH THƯỜNG (`code === -201`), không phải lỗi.
 *
 * Since 12/10/2026 it is the first action of the "Kết nối với ViHAT" card (`heading="h3"`,
 * `company-intro/ConnectWithVihat.tsx`), which shows it only while no session is held.
 */
export function KhoiDangNhap({ heading = "h2" }: { heading?: "h2" | "h3" }) {
  return (
    <TinhNangCoTrangThai
      ma="dang-nhap"
      xin={xinMaDangNhap}
      cap_tieu_de={heading}
      glyph={<HandshakeGlyph className="tn__glyph" />}
      veKetQua={(ma) => <KetQuaDangNhap ma={ma} />}
      duoi_cung={<DuongLienHeNgay />}
    />
  );
}
