/**
 * Vỏ của kênh công dân: một màn chọn việc (gửi · tra cứu), rồi đúng màn ấy.
 *
 * Nó đứng sau cửa `bien-the/cong-dan` (`vite.config.ts`): bản `goc` — BẢN NỘP — thay cả tệp này bằng
 * bản rỗng, nên không một chữ nào của kênh công dân đi vào bundle gửi Zalo duyệt cho tới ngày cầu
 * phiên ViGov có thật.
 */
import { useState } from "react";

import { GuiPhanAnhScreen } from "./GuiPhanAnhScreen";
import { GUI, QUAY_LAI, TRA_CUU } from "./noi-dung";
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

export function KenhCongDan({ onDong }: { onDong: () => void }) {
  const [man, datMan] = useState<"chon" | "gui" | "tra-cuu">("chon");

  if (man === "gui") return <GuiPhanAnhScreen onQuayLai={() => datMan("chon")} />;
  if (man === "tra-cuu") return <TraCuuPhieuScreen onQuayLai={() => datMan("chon")} />;

  return (
    <section className="cd-man" aria-label={NHAN_KENH_CONG_DAN}>
      <button type="button" className="quay-lai" onClick={onDong}>
        {QUAY_LAI}
      </button>
      <h1 className="cd-tieu-de">{NHAN_KENH_CONG_DAN}</h1>
      <button type="button" className="cd-nut" onClick={() => datMan("gui")}>
        {GUI.tieu_de}
      </button>
      <button type="button" className="cd-nut" onClick={() => datMan("tra-cuu")}>
        {TRA_CUU.tieu_de}
      </button>
    </section>
  );
}
