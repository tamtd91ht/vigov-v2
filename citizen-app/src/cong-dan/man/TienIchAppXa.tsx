/**
 * CÁC MÀN CÒN LẠI CỦA APP RIÊNG — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026: "làm đủ
 * các màn như bản mẫu"): tra cứu hồ sơ, truyền thanh, video, bản đồ, thông báo, cá nhân. Không màn định
 * danh, không đăng nhập (bỏ 28/09/2026). Họ tên ở Cá nhân CHỈ ĐỂ HIỂN THỊ (29/09/2026): nó được lấy từ
 * Zalo một lần lúc mở app (`TrangXa`), và không có thì hiện "Chưa xác định" — màn này không gọi Zalo,
 * vì hỏi lại ở đây là hỏi lần hai một câu bà con đã trả lời lúc mở app.
 *
 * MÀN NÀO CHƯA CÓ DỮ LIỆU THẬT thì hiện trạng thái trống bằng lời ("xã chưa cập nhật…"), KHÔNG dữ liệu
 * giả: một bản tin bịa trong app mang tên cơ quan nhà nước là một thông tin sai do xã phát hành.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: quét căn cước (dữ liệu định danh — luật 3, điều kiện dừng), đăng nhập, đăng xuất, lưu
 * cài đặt xuống máy (`localStorage` cấm ở nửa này — `ranh-gioi-hai-nua.test.ts` §3b; cỡ chữ sống trong
 * bộ nhớ của lần mở; công tắc thông báo bỏ tới khi có thông báo thật).
 */
import { useState } from "react";

import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, OBieuTuong, TrangCon } from "./khung-xa";
import { CUA_TOI, XA_TN } from "./noi-dung";
import { ONhapDong } from "./o-nhap";
import { chuCaiDau } from "./trai-nghiem";

/* ═══════════════════════════════ TRA CỨU HỒ SƠ ═══════════════════════════════ */

/**
 * Tra cứu hồ sơ một cửa — theo spec kho yêu cầu (`05-nghiep-vu.md:148`): SỐ ĐIỆN THOẠI ĐẦY ĐỦ + 4 SỐ CUỐI
 * của số hồ sơ, cả hai bắt buộc. CHƯA có hệ thống một cửa nào nối vào ViGov, nên mọi lần tra nói thật điều
 * ấy — không bao giờ trả một kết quả dựng ra. Hai ô gõ vào không rời máy.
 */
export function TraCuuHoSoXa({ onQuayLai }: { onQuayLai: () => void }) {
  const [phone, setPhone] = useState("");
  const [lastFour, setLastFour] = useState("");
  const [ket_qua, datKetQua] = useState<string | null>(null);
  const complete = phone.replace(/\D/g, "").length >= 9 && /^\d{4}$/.test(lastFour.trim());
  return (
    <>
      <DauManCon tieu_de={XA_TN.tra_cuu_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong id="xa-sdt-ho-so" nhan={XA_TN.o_so_dien_thoai_ho_so} goi_y={XA_TN.goi_y_so_dien_thoai_ho_so} gia_tri={phone} toi_da={20} kieu_ban_phim="tel" onDoi={setPhone} />
          <ONhapDong id="xa-bon-so-cuoi" nhan={XA_TN.o_bon_so_cuoi} goi_y={XA_TN.goi_y_bon_so_cuoi} gia_tri={lastFour} toi_da={4} kieu_ban_phim="tel" onDoi={setLastFour} />
          <button
            type="button"
            className="xa-nut"
            onClick={() => datKetQua(complete ? XA_TN.tra_cuu_chua_ket_noi : XA_TN.tra_cuu_can_ma)}
          >
            <BieuTuong ten="search" co={20} />
            {XA_TN.nut_tra_cuu}
          </button>
        </div>
        {ket_qua !== null && <KhoiTrangThai bieu_tuong="info" cau={ket_qua} />}
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ MÀN CHƯA CÓ DỮ LIỆU ═══════════════════════════════ */

export function ManChuaCoDuLieu(props: { tieu_de: string; bieu_tuong: TenBieuTuong; cau: string; onQuayLai: () => void }) {
  return (
    <>
      <DauManCon tieu_de={props.tieu_de} onQuayLai={props.onQuayLai} />
      <TrangCon>
        <KhoiTrangThai bieu_tuong={props.bieu_tuong} cau={props.cau} />
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ CÁ NHÂN ═══════════════════════════════ */

export type CoChu = "vua" | "lon" | "rat-lon";

export function CaNhanXa(props: {
  /** Tên Zalo lấy lúc mở app, hoặc `null` → "Chưa xác định". */
  ho_ten: string | null;
  ten_xa: string;
  tinh: string;
  /**
   * Petitions loaded in this open: the count, and whether more pages exist; `null` when not loaded (no
   * session yet) — then no number is shown, never an invented 0.
   */
  so_phieu: { readonly count: number; readonly more: boolean } | null;
  co_chu: CoChu;
  onDoiCoChu: (c: CoChu) => void;
  onMoPhanAnh: () => void;
  onMoTraCuu: () => void;
}) {
  const co_chu: ReadonlyArray<[CoChu, string]> = [
    ["vua", XA_TN.co_chu_vua],
    ["lon", XA_TN.co_chu_lon],
    ["rat-lon", XA_TN.co_chu_rat_lon],
  ];

  return (
    <div className="xa-trang xa-trang--tab">
      <div className="xa-the xa-the--dem xa-khoi">
        <div className="xa-ho-so">
          <span className="xa-can-bo__chu-dau xa-ho-so__chu" aria-hidden="true">
            {props.ho_ten ? chuCaiDau(props.ho_ten) : <BieuTuong ten="user" co={28} />}
          </span>
          <span className="xa-can-bo__chu">
            <strong className="xa-can-bo__ten">{props.ho_ten ?? XA_TN.chua_co_ten}</strong>
          </span>
        </div>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.tien_ich}</h2>
      <div className="xa-the">
        <button type="button" className="xa-hang" onClick={props.onMoPhanAnh}>
          <OBieuTuong ten="chat" mau="red" />
          <span className="xa-hang__chu">
            <strong>{CUA_TOI.tieu_de}</strong>
            <span className="xa-phu">
              {props.so_phieu === null
                ? XA_TN.so_phieu_unknown
                : props.so_phieu.more
                  ? XA_TN.so_phieu_more(props.so_phieu.count)
                  : XA_TN.so_phieu(props.so_phieu.count)}
            </span>
          </span>
          <BieuTuong ten="right" co={20} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onMoTraCuu}>
          <OBieuTuong ten="history" mau="xanh" />
          <span className="xa-hang__chu">
            <strong>{XA_TN.lich_su_tra_cuu}</strong>
            <span className="xa-phu">{XA_TN.lich_su_tra_cuu_phu}</span>
          </span>
          <BieuTuong ten="right" co={20} />
        </button>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.hien_thi}</h2>
      <div className="xa-the xa-the--dem">
        <p className="xa-nhan-o">{XA_TN.co_chu}</p>
        <div className="xa-chips" role="group" aria-label={XA_TN.co_chu}>
          {co_chu.map(([k, n]) => (
            <button
              key={k}
              type="button"
              className={`xa-chip${props.co_chu === k ? " xa-chip--on" : ""}`}
              aria-pressed={props.co_chu === k}
              onClick={() => props.onDoiCoChu(k)}
            >
              {n}
            </button>
          ))}
        </div>
        <p className="xa-phu">{XA_TN.xem_truoc_co_chu}</p>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.muc_thong_bao}</h2>
      {/* No switch until notifications exist: a switch wired to nothing lets a citizen believe they opted out
          of messages the commune will later send. The real opt-out lives on the server, next to ZNS. */}
      <div className="xa-the xa-the--dem">
        <p className="xa-phu">{XA_TN.thong_bao_chua_co}</p>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.ve_ung_dung}</h2>
      <div className="xa-the xa-the--dem">
        <p className="xa-nhan-o">{XA_TN.don_vi}</p>
        <p>
          {props.ten_xa}
          {props.tinh !== "" ? ` · ${props.tinh}` : ""}
        </p>
      </div>
    </div>
  );
}
