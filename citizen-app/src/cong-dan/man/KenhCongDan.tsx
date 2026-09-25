/**
 * Vỏ của kênh công dân: một màn chọn việc (gửi · phản ánh của tôi · tra cứu), rồi đúng màn ấy.
 *
 * Nó đứng sau cửa `bien-the/cong-dan` (`vite.config.ts`): bản `goc` — BẢN NỘP — thay cả tệp này bằng
 * bản rỗng, nên không một chữ nào của kênh công dân đi vào bundle gửi Zalo duyệt cho tới ngày cầu
 * phiên ViGov có thật.
 */
import { useState } from "react";

import { GuiPhanAnhScreen } from "./GuiPhanAnhScreen";
import { CUA_TOI, GUI, QUAY_LAI, TRA_CUU } from "./noi-dung";
import { PhanAnhCuaToiScreen } from "./PhanAnhCuaToiScreen";
import { TraCuuPhieuScreen } from "./TraCuuPhieuScreen";

export const NHAN_KENH_CONG_DAN = "Phản ánh với xã";

/** Nút mở kênh, đặt trên trang xã. Nhãn nằm ở đây — không viết thẳng trong `App.tsx`. */
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
  | { kieu: "tra-cuu"; ma: string; tu_danh_sach: boolean };

export function KenhCongDan({ onDong }: { onDong: () => void }) {
  const [man, datMan] = useState<Man>({ kieu: "chon" });
  const veChon = () => datMan({ kieu: "chon" });

  if (man.kieu === "gui") return <GuiPhanAnhScreen onQuayLai={veChon} />;

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
          />
        </div>
        {man.kieu === "tra-cuu" && (
          <TraCuuPhieuScreen
            key={man.ma}
            ma_ban_dau={man.ma}
            onQuayLai={() => datMan({ kieu: "cua-toi" })}
          />
        )}
      </>
    );
  }

  if (man.kieu === "tra-cuu") return <TraCuuPhieuScreen onQuayLai={veChon} />;

  return (
    <section className="cd-man" aria-label={NHAN_KENH_CONG_DAN}>
      <button type="button" className="quay-lai" onClick={onDong}>
        {QUAY_LAI}
      </button>
      <h1 className="cd-tieu-de">{NHAN_KENH_CONG_DAN}</h1>
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
    </section>
  );
}
