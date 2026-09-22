/**
 * DANH THIẾP SỐ CỦA ViHAT Group — `keepScreen` + `downloadFile`.
 *
 * Đúng dòng sản phẩm đã công bố: "Giải pháp networking và quản lý danh thiếp số cho cá nhân và
 * doanh nghiệp" (`company-profile.ts`, `SOLUTIONS` mục `namecard`). Tấm thiếp này là chính dòng
 * sản phẩm ấy, dùng cho chính công ty — không phải một màn trình diễn quyền.
 *
 * BA ĐIỀU TỆP NÀY LÀM, VÀ MỖI ĐIỀU MỘT LÝ DO:
 *
 *   1. **Hiện mã QR chứa vCard** dựng từ `COMPANY` và `CONTACT`. Không một trường nào bịa thêm
 *      (`vcard.ts`), vì tấm thiếp này đi vào danh bạ của người khác và ở lại đó.
 *   2. **Giữ màn hình sáng** trong lúc người đối diện đang quét — và **tắt lại khi rời màn**.
 *      Cặp bật/tắt ấy nằm trong `hieuUngGiuManSang` và có phép kiểm riêng: bật rồi bỏ đó là lấy
 *      pin của người dùng cho một tính năng họ đã rời khỏi.
 *   3. **Tải tệp `.vcf` xuống máy** từ chính chuỗi vCard ấy, mã hoá base64 tại chỗ. Không `url`,
 *      không máy chủ, không một lời gọi mạng nào.
 *
 * ⚠ KHÔNG CÓ DỮ LIỆU CỦA NGƯỜI DÙNG Ở MÀN NÀY. Cả ba việc trên chỉ chạm tới thông tin doanh
 * nghiệp của chính chúng tôi. Tệp ghi xuống máy là tệp CỦA CHÚNG TÔI — chính sách quyền riêng
 * tư nói ra đúng điều đó, vì "ứng dụng ghi một tệp xuống máy bạn" là một hành vi mới và người
 * dùng có quyền biết tệp ấy chứa gì.
 */
import { useEffect, useState } from "react";

import { NamecardGlyph } from "../company-intro/icons";

import { hieuUngGiuManSang } from "./giu-man-sang";
import { KhungTinhNang, type TrangThai } from "./khung";
import { MaQR } from "./MaQR";
import { THIEP_CUA_CHUNG_TOI } from "./noi-dung";
import { base64Utf8, TRUONG_THIEP_CUA_CHUNG_TOI, vCardCuaChungToi } from "./vcard";
import { giuManHinhSang, rungMotNhip, taiTepVeMay } from "./zalo-api";

/**
 * Nhãn tiếng Việt của từng trường vCard, cho phần CHỮ dưới mã.
 *
 * Phần chữ ấy không phải trang trí: mã QR là một hình, trình đọc màn hình không đọc được nó, và
 * người không có máy thứ hai để quét cũng cần đọc được thông tin. Mọi thứ trong mã đều có mặt ở
 * đây bằng chữ — đó là điều kiện để cái mã kia được phép `aria-hidden`.
 */
const NHAN_TRUONG: Record<string, string> = {
  FN: "Tên",
  ORG: "Công ty",
  "TEL;TYPE=WORK,VOICE": "Hotline",
  "EMAIL;TYPE=WORK": "Email",
  URL: "Trang web",
};

/**
 * Thông tin trong mã, hiện bằng chữ. THUẦN — dựng thẳng từ danh sách trường của tấm thiếp.
 *
 * ⚠ BỎ QUA TRƯỜNG RỖNG, ĐÚNG LUẬT `dungVCard` DÙNG. Một trường chưa có nguồn (`URL`, từ
 * 21/09/2026) không được phép thành một dòng "Trang web" bỏ trống trên màn: nó vừa là một ô
 * trống trình đọc màn hình vẫn đọc nhãn, vừa nói rằng phần chữ và mã QR chứa khác nhau — trong
 * khi cả hai phải là một nguồn.
 */
export function ThongTinTrongMa() {
  return (
    <ul className="the-tt__danh-sach">
      {TRUONG_THIEP_CUA_CHUNG_TOI.filter(
        (truong) => NHAN_TRUONG[truong.ten] !== undefined && truong.gia_tri !== "",
      ).map(
        (truong) => (
          <li className="the-tt__dong" key={truong.ten}>
            <span className="the-tt__nhan">{NHAN_TRUONG[truong.ten]}</span>
            <span className="the-tt__gia-tri">{truong.gia_tri}</span>
          </li>
        ),
      )}
    </ul>
  );
}

/** Tấm thiếp: mã QR cộng đúng những thông tin nằm trong mã, bằng chữ. THUẦN. */
export function TheQrCuaChungToi({ noi_dung_vcard }: { noi_dung_vcard: string }) {
  return (
    <div className="ma-qr__khung">
      <MaQR noi_dung={noi_dung_vcard} className="ma-qr" />
      <h3 className="the-tt__tieu-de">{THIEP_CUA_CHUNG_TOI.tieu_de_thong_tin}</h3>
      <ThongTinTrongMa />
    </div>
  );
}

export function ManThiepCuaChungToi() {
  const [trang_thai, datTrangThai] = useState<TrangThai<void>>({ kieu: "chua-goi" });
  const [giu_sang, datGiuSang] = useState(false);

  // Tính một lần: nội dung tấm thiếp là hằng, và sinh lại nó mỗi lần vẽ là dựng lại cả mã QR.
  const [vcard] = useState(vCardCuaChungToi);

  /**
   * ⚠ HAI CHIỀU CỦA `keepScreen` NẰM TRONG CÙNG MỘT HIỆU ỨNG, CÓ CHỦ ĐÍCH.
   *
   * Hàm dọn dẹp của `useEffect` chạy cả khi `giu_sang` đổi lẫn khi màn hình bị tháo ra — nên
   * "tắt lại khi rời màn" không phải một lời hứa mà là hệ quả của chỗ đặt lời gọi. Hợp đồng ấy
   * kiểm được mà không cần DOM: xem `hieuUngGiuManSang` và phép kiểm của nó.
   */
  useEffect(
    () => hieuUngGiuManSang(giu_sang, (bat) => void giuManHinhSang(bat)),
    [giu_sang],
  );

  async function tai() {
    datTrangThai({ kieu: "dang-cho" });
    const ket_qua = await taiTepVeMay(base64Utf8(vcard));
    datTrangThai(ket_qua);
    // Một nhịp rung khi tệp đã nằm trên máy: người vừa bấm thường đang nói chuyện với người
    // đối diện chứ không nhìn màn hình.
    if (ket_qua.kieu === "xong") await rungMotNhip();
  }

  return (
    <KhungTinhNang
      ma="thiep-cua-chung-toi"
      trang_thai={trang_thai}
      onBam={() => void tai()}
      /* `<h1>` TỪ 22/09/2026: khối này là khối MỞ ĐẦU màn Danh thiếp, và cấp tiêu đề đi theo chỗ
         đứng chứ không theo tính năng. Xem khối chú thích của `ManDanhThiep`. */
      cap_tieu_de="h1"
      glyph={<NamecardGlyph className="tn__glyph" />}
      dan_nhap={THIEP_CUA_CHUNG_TOI.dan_nhap}
      truoc_nut={
        <>
          <TheQrCuaChungToi noi_dung_vcard={vcard} />
          {/* Trạng thái bật/tắt nói bằng CHỮ trên chính cái nút, không bằng riêng màu nền —
              `aria-pressed` cho trình đọc màn hình, nhãn đổi cho người nhìn. */}
          <button
            type="button"
            className="tn__nut-giu"
            aria-pressed={giu_sang}
            onClick={() => datGiuSang((truoc) => !truoc)}
          >
            {giu_sang ? THIEP_CUA_CHUNG_TOI.nut_thoi_giu_sang : THIEP_CUA_CHUNG_TOI.nut_giu_sang}
          </button>
          {giu_sang && <p className="tn__giai-thich">{THIEP_CUA_CHUNG_TOI.dang_giu_sang}</p>}
        </>
      }
      veKetQua={() => (
        <>
          <p className="tn__xong">{THIEP_CUA_CHUNG_TOI.da_tai_xong}</p>
          <p className="tn__ranh-gioi">{THIEP_CUA_CHUNG_TOI.zalo_quyet_dinh_cho_luu}</p>
        </>
      )}
      duoi_cung={<p className="tn__rieng-tu">{THIEP_CUA_CHUNG_TOI.tep_la_cua_chung_toi}</p>}
    />
  );
}
