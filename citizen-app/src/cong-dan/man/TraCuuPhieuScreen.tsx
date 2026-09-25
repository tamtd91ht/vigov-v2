/**
 * MÀN "TRA CỨU PHIẾU CỦA TÔI" — nhập mã, xem tình trạng, các hạn và kết quả khi đã đóng.
 *
 * ⚠ MỘT CÂU CHO MỌI 404: "không có mã này", "phiếu của người khác", "phiếu của xã khác" là cùng một
 * câu trả lời từ máy chủ, và màn hình không được tách chúng (luật 4, cấm #2).
 *
 * ⚠ MÃ TRA CỨU KHÔNG ĐI VÀO NHẬT KÝ, không ghi xuống máy. Nó chỉ đi trên đường dẫn của đúng một
 * lời gọi, kèm bearer của phiên — hợp đồng đặt nó ở đó.
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG VẼ Ô NHẬP và KHÔNG GỌI MẠNG — hôm nay là luôn luôn.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";

import { type KetQuaGoi, traCuuPhieu } from "../api/goi-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { BangXa, KenhChuaMo, ThePhieu } from "./khung";
import { QUAY_LAI, TRA_CUU } from "./noi-dung";
import { ONhapDong } from "./o-nhap";

/** Độ dài tối đa ô mã. Mã do máy chủ sinh ngẫu nhiên và ngắn hơn nhiều; đây chỉ là trần ô nhập. */
const MA_TOI_DA = 64;

/**
 * Kết quả tra → thứ hiện dưới ô nhập. THUẦN, để test dựng thẳng từng nhánh.
 *
 * `chua-co-phien` / `chua-cau-hinh` không tới được đây trong luồng thường (màn đã dừng trước), nên
 * chúng hiện đúng khối "kênh chưa mở" chứ không một câu lỗi.
 */
export function KetQuaTraCuu({ kq }: { kq: KetQuaGoi }): ReactNode {
  switch (kq.kieu) {
    case "xong":
      return <ThePhieu phieu={kq.phieu} />;
    case "chua-co-phien":
    case "chua-cau-hinh":
      return <KenhChuaMo />;
    case "khong-thay":
      return (
        <p className="cd-loi" role="alert">
          {TRA_CUU.khong_thay}
        </p>
      );
    case "het-phien":
      return (
        <p className="cd-loi" role="alert">
          {TRA_CUU.het_phien}
        </p>
      );
    case "loi-mang":
      return (
        <p className="cd-loi" role="alert">
          {TRA_CUU.loi_mang}
        </p>
      );
    default:
      return (
        <p className="cd-loi" role="alert">
          {TRA_CUU.loi_may_chu}
        </p>
      );
  }
}

/**
 * `ma_ban_dau`: mã đã chọn từ "Phản ánh của tôi" — điền sẵn vào ô và tra ngay khi mở, để người dân
 * không phải gõ lại một mã họ vừa chạm vào. Vẫn đi qua ĐÚNG lời gọi tra cứu, với ĐÚNG phiên.
 */
export function TraCuuPhieuScreen({ onQuayLai, ma_ban_dau = "" }: { onQuayLai: () => void; ma_ban_dau?: string }) {
  const [phien] = useState(layPhienViGov);
  const [ma, datMa] = useState(ma_ban_dau);
  const [dangTra, datDangTra] = useState(false);
  const [thieuMa, datThieuMa] = useState(false);
  const [kq, datKq] = useState<KetQuaGoi | null>(null);
  // Tra mã điền sẵn ĐÚNG MỘT LẦN, kể cả khi React dựng hiệu ứng hai lần (StrictMode).
  const da_tra_san = useRef(false);

  useEffect(() => {
    if (phien === null || ma_ban_dau.trim() === "" || da_tra_san.current) return;
    da_tra_san.current = true;
    void tra();
  }, [phien, ma_ban_dau]);

  const nutQuayLai = (
    <button type="button" className="quay-lai" onClick={onQuayLai}>
      {QUAY_LAI}
    </button>
  );

  if (phien === null) {
    return (
      <section className="cd-man" aria-label={TRA_CUU.tieu_de}>
        {nutQuayLai}
        <h1 className="cd-tieu-de">{TRA_CUU.tieu_de}</h1>
        <KenhChuaMo />
      </section>
    );
  }

  async function tra() {
    if (ma.trim() === "") {
      datThieuMa(true);
      return;
    }
    datThieuMa(false);
    datDangTra(true);
    datKq(null);
    datKq(await traCuuPhieu(ma));
    datDangTra(false);
  }

  return (
    <section className="cd-man" aria-label={TRA_CUU.tieu_de}>
      {nutQuayLai}
      <BangXa ten_xa={phien.ten_xa} />
      <h1 className="cd-tieu-de">{TRA_CUU.tieu_de}</h1>
      <ONhapDong
        id="cd-ma-tra-cuu"
        nhan={TRA_CUU.nhan_ma}
        goi_y={TRA_CUU.goi_y_ma}
        gia_tri={ma}
        toi_da={MA_TOI_DA}
        onDoi={datMa}
      />
      {thieuMa && (
        <p className="cd-loi" role="alert">
          {TRA_CUU.thieu_ma}
        </p>
      )}
      <button type="button" className="cd-nut" disabled={dangTra} onClick={() => void tra()}>
        {TRA_CUU.nut_tra}
      </button>
      {dangTra && (
        <p className="cd-cau" role="status">
          {TRA_CUU.dang_tra}
        </p>
      )}
      {kq !== null && <KetQuaTraCuu kq={kq} />}
    </section>
  );
}
