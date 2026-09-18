/**
 * HAI TÍNH NĂNG CỦA MÀN LIÊN HỆ — `getLocation` và `getPhoneNumber`.
 *
 * Cả hai đứng trên màn Liên hệ chứ không thành tab riêng, và đó là chủ đích: người mở màn Liên
 * hệ đang muốn nói chuyện với công ty. "Tìm văn phòng gần tôi" và "Đăng ký nhận tư vấn" là đúng
 * hai việc họ định làm ở đó — một tab riêng tên "Quyền" thì nói với người duyệt rằng đây là một
 * app đi xin quyền, còn hai tính năng đặt đúng chỗ thì nói rằng đây là app có việc để làm.
 *
 * ⚠ RANH GIỚI THẬT, VÀ MÃ NÀY KHÔNG GIẢ VỜ VƯỢT QUA NÓ:
 *
 *   `getLocation` chỉ trả **token**; `latitude`/`longitude` đã `@deprecated` và tệp này không
 *   đọc chúng. `navigator.geolocation` thì bị cấm bởi dây bẫy trong
 *   `phase1-collects-nothing.test.ts` và cũng là một quyền khác hẳn. Nghĩa là **không thể xếp
 *   văn phòng theo khoảng cách trên máy** — nên ở đây không có một dòng nào tính khoảng cách, và
 *   màn hình nói ra điều đó bằng tiếng Việt (`VAN_PHONG.chua_xep_duoc`).
 *
 *   `getPhoneNumber` cũng chỉ trả token, và không có đường gửi nó đi đâu (dây bẫy cấm `fetch`).
 *   Nên "đăng ký tư vấn" ở bản này dừng ở chỗ nhận được mã, và màn hình nói thẳng như vậy rồi
 *   đưa ngay hai đường liên hệ CHẠY ĐƯỢC BÂY GIỜ: hotline và email.
 *
 *   Một người muốn được tư vấn phải liên hệ được ngay, không phải chờ một giai đoạn sau.
 */
import { CONTACT, OFFICES } from "../../content/company-profile";
import { CompassGlyph, HandshakeGlyph, MailGlyph, PhoneGlyph, PinGlyph } from "../company-intro/icons";

import { KetQuaToken, TinhNangCoTrangThai } from "./khung";
import { LOI_MO_NGOAI, TU_VAN, VAN_PHONG } from "./noi-dung";
import { moTrangWeb, xinTokenSoDienThoai, xinTokenViTri } from "./zalo-api";
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

  async function chiDuong(dia_chi: string) {
    const ket_qua = await moTrangWeb(duongDanBanDo(dia_chi));
    datKhongMoDuoc(ket_qua.kieu !== "xong");
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
 * Hai đường liên hệ chạy được ngay bây giờ, đặt ngay dưới nút đăng ký.
 *
 * Chúng lặp lại hotline và email ở đầu màn Liên hệ, và việc lặp lại ấy là CÓ CHỦ ĐÍCH: người vừa
 * đọc câu "bản này chưa gửi yêu cầu đi" phải thấy ngay một đường khác, ở đúng chỗ mắt họ đang
 * nhìn. Bắt họ cuộn ngược lên là bắt một khách hàng tiềm năng tự đi tìm cách liên hệ.
 */
function DuongLienHeNgay() {
  return (
    <>
      <p className="tn__nhac">{TU_VAN.nhac_lien_he}</p>
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

export function DangKyTuVan() {
  return (
    <TinhNangCoTrangThai
      ma="tu-van"
      xin={xinTokenSoDienThoai}
      cap_tieu_de="h2"
      glyph={<HandshakeGlyph className="tn__glyph" />}
      dan_nhap={TU_VAN.dan_nhap}
      veKetQua={(token) => (
        <KetQuaToken
          ma="tu-van"
          token={token}
          da_nhan={TU_VAN.da_nhan_ma}
          noi_them={TU_VAN.chua_gui_di}
        />
      )}
      duoi_cung={<DuongLienHeNgay />}
    />
  );
}
