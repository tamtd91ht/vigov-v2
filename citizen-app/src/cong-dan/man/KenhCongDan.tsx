/**
 * Vỏ của kênh công dân: một màn chọn việc (gửi · phản ánh của tôi · tra cứu), rồi đúng màn ấy.
 *
 * Có mặt trong bản dựng duy nhất (27/09/2026). Chừng nào chưa có phiên ViGov, ba màn phản ánh nói
 * "kênh chưa mở" — `api/phien-vigov.ts` — và màn chọn việc nói điều ấy TRƯỚC khi người dân bấm
 * (`CHUA_DANG_NHAP_XA`).
 *
 * HAI MÀN CÔNG KHAI (27/09/2026): "Tin tức của xã" và "Danh bạ cán bộ xã". Chúng cần TÊN MIỀN xã cho
 * `?host=`, và `App.tsx` là bên chọn nó (`khoaTraCongKhai`): tên miền chính của xã CỦA PHIÊN
 * (`communePrimaryHost`), nếu không có thì `d` công dân đã xác nhận ở lần mở này. Không có cả hai —
 * mở app không qua QR và phiên không mang tên miền — thì hai lối vào BIẾN MẤT thay vì đoán một xã
 * (bundle không được mang giá trị theo xã, ADR 0047 điều kiện dừng #2).
 */
import { useState } from "react";

import type { ReopenWithPhone } from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { DanhBaCanBoScreen } from "./DanhBaCanBoScreen";
import { GuiPhanAnhScreen } from "./GuiPhanAnhScreen";
import { CHUA_DANG_NHAP_XA, CUA_TOI, DANH_BA, GUI, QUAY_LAI, TIN_XA, TRA_CUU } from "./noi-dung";
import { PhanAnhCuaToiScreen } from "./PhanAnhCuaToiScreen";
import { TinTucXaScreen } from "./TinTucXaScreen";
import { TraCuuPhieuScreen } from "./TraCuuPhieuScreen";

export const NHAN_KENH_CONG_DAN = "Phản ánh với xã";

/** Nút mở kênh, đặt trên tab đầu khi đã xác nhận xã. Nhãn nằm ở đây — không viết thẳng trong `App.tsx`. */
export function NutVaoKenhCongDan({ onBam }: { onBam: () => void }) {
  return (
    <button type="button" className="cd-nut" onClick={onBam}>
      {NHAN_KENH_CONG_DAN}
    </button>
  );
}

type Man =
  | { kieu: "chon" }
  | { kieu: "gui" }
  | { kieu: "cua-toi" }
  /** `tu_danh_sach`: mở từ "Phản ánh của tôi" — "Quay lại" về đúng danh sách ấy. */
  | { kieu: "tra-cuu"; ma: string; tu_danh_sach: boolean }
  | { kieu: "tin-tuc" }
  | { kieu: "danh-ba" };

export function KenhCongDan({
  onDong,
  ten_mien = null,
  reopenWithPhone,
}: {
  /** Không truyền = không có nút "Quay lại" (app riêng của xã: kênh là màn gốc, không có chỗ để về). */
  onDong?: () => void;
  /** Tên miền xã công dân đã xác nhận ở lần mở này, hoặc `null`. Chỉ làm khoá tra `?host=`. */
  ten_mien?: string | null;
  /**
   * Mở lại phiên kèm số điện thoại khi một tuyến phản ánh trả 403 `chua_xac_thuc_so` — lớp vỏ dựng nó
   * (`App.tsx`), ba màn phản ánh chỉ gọi nó SAU cú bấm đồng ý của công dân (`phone-verification.tsx`).
   */
  reopenWithPhone?: ReopenWithPhone;
}) {
  const [man, datMan] = useState<Man>({ kieu: "chon" });
  // Đọc MỘT LẦN lúc dựng, cùng cách ba màn phản ánh đọc (`useState(layPhienViGov)`): phiên chỉ được
  // ghi trước khi kênh mở, ở bước xác nhận xã.
  const [co_phien] = useState(() => layPhienViGov() !== null);
  const veChon = () => datMan({ kieu: "chon" });

  if (man.kieu === "gui") return <GuiPhanAnhScreen onQuayLai={veChon} reopenWithPhone={reopenWithPhone} />;
  if (man.kieu === "tin-tuc" && ten_mien !== null) {
    return <TinTucXaScreen ten_mien={ten_mien} onQuayLai={veChon} />;
  }
  if (man.kieu === "danh-ba" && ten_mien !== null) {
    return <DanhBaCanBoScreen ten_mien={ten_mien} onQuayLai={veChon} />;
  }

  if (man.kieu === "cua-toi" || (man.kieu === "tra-cuu" && man.tu_danh_sach)) {
    // DANH SÁCH GIỮ NGUYÊN khi mở một phiếu: nó chỉ bị ẩn (`hidden`), không bị gỡ, nên "Quay lại"
    // trả người dân về đúng những trang họ đã bấm "Xem thêm" — không tải lại từ đầu. Vẫn chỉ trong
    // bộ nhớ; đóng kênh là mất.
    const dang_mo_phieu = man.kieu === "tra-cuu";
    return (
      <>
        <div hidden={dang_mo_phieu}>
          <PhanAnhCuaToiScreen
            onQuayLai={veChon}
            onMoPhieu={(ma) => datMan({ kieu: "tra-cuu", ma, tu_danh_sach: true })}
            onGuiPhanAnh={() => datMan({ kieu: "gui" })}
            reopenWithPhone={reopenWithPhone}
          />
        </div>
        {man.kieu === "tra-cuu" && (
          <TraCuuPhieuScreen
            key={man.ma}
            ma_ban_dau={man.ma}
            onQuayLai={() => datMan({ kieu: "cua-toi" })}
            reopenWithPhone={reopenWithPhone}
          />
        )}
      </>
    );
  }

  if (man.kieu === "tra-cuu") return <TraCuuPhieuScreen onQuayLai={veChon} reopenWithPhone={reopenWithPhone} />;

  return (
    <section className="cd-man" aria-label={NHAN_KENH_CONG_DAN}>
      {onDong !== undefined && (
        <button type="button" className="quay-lai" onClick={onDong}>
          {QUAY_LAI}
        </button>
      )}
      <h1 className="cd-tieu-de">{NHAN_KENH_CONG_DAN}</h1>
      {!co_phien && (
        <p className="cd-loi" role="status">
          {CHUA_DANG_NHAP_XA.cau}
          {ten_mien !== null && ` ${CHUA_DANG_NHAP_XA.con_lai}`}
        </p>
      )}
      <button type="button" className="cd-nut" onClick={() => datMan({ kieu: "gui" })}>
        {GUI.tieu_de}
      </button>
      <button type="button" className="cd-nut" onClick={() => datMan({ kieu: "cua-toi" })}>
        {CUA_TOI.tieu_de}
      </button>
      <button
        type="button"
        className="cd-nut"
        onClick={() => datMan({ kieu: "tra-cuu", ma: "", tu_danh_sach: false })}
      >
        {TRA_CUU.tieu_de}
      </button>
      {ten_mien !== null && (
        <>
          <button type="button" className="cd-nut" onClick={() => datMan({ kieu: "tin-tuc" })}>
            {TIN_XA.tieu_de}
          </button>
          <button type="button" className="cd-nut" onClick={() => datMan({ kieu: "danh-ba" })}>
            {DANH_BA.tieu_de}
          </button>
        </>
      )}
    </section>
  );
}
