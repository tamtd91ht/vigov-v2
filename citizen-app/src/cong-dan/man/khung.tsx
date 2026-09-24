/**
 * Ba mảnh dùng chung của hai màn nửa nhà nước, THUẦN — nhận mọi thứ qua tham số, để dựng được bằng
 * `react-dom/server` trong test (không có DOM để bấm).
 */
import type { ReactNode } from "react";

import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";

import { giaiThichTrangThai, KENH_CHUA_MO, NHAN_XA_DANG_GUI, nhanTrangThai, THE_PHIEU } from "./noi-dung";
import { THOI_DIEM_KHONG_DOC_DUOC, thoiDiemVN } from "./thoi-diem";

/**
 * TÊN XÃ CỦA PHIÊN, ĐẦU MỖI MÀN (README §Non-negotiables #2). Xã ấy là xã MÁY CHỦ sẽ ghi phiếu vào
 * — không phải xã lớp khám phá gợi ý. `role="note"` để trình đọc màn hình đọc nó ra.
 */
export function BangXa({ ten_xa }: { ten_xa: string }) {
  return (
    <p className="cd-xa" role="note">
      <span className="cd-xa__nhan">{NHAN_XA_DANG_GUI}</span>
      <strong className="cd-xa__ten">{ten_xa}</strong>
    </p>
  );
}

/** Kênh chưa mở — hiện khi không có phiên ViGov. Không có biểu mẫu, không có nút gửi. */
export function KenhChuaMo() {
  return (
    <section className="cd-dong" role="status" aria-labelledby="cd-dong-tieu-de">
      <h2 className="cd-dong__tieu-de" id="cd-dong-tieu-de">
        {KENH_CHUA_MO.tieu_de}
      </h2>
      <p className="cd-dong__cau">{KENH_CHUA_MO.cau}</p>
    </section>
  );
}

function moc(iso: string): string {
  return thoiDiemVN(iso) ?? THOI_DIEM_KHONG_DOC_DUOC;
}

function Dong({ nhan, children }: { nhan: string; children: ReactNode }) {
  return (
    <div className="cd-phieu__dong">
      <dt className="cd-phieu__nhan">{nhan}</dt>
      <dd className="cd-phieu__gia-tri">{children}</dd>
    </div>
  );
}

/**
 * MỘT PHIẾU, CHỈ NHỮNG GÌ `phieuCuaToiRa` TRẢ VỀ. Không có ghi chú cán bộ, lịch sử chuyển hay tên
 * người xử lý — máy chủ không gửi, và thẻ này không có ô nào chờ chúng (luật 10, bất biến 7).
 *
 * Trạng thái bằng CHỮ, không bằng màu (README §Non-negotiables #6). Hai `null` của hai hạn nói hai
 * câu khác nhau: `han_tiep_nhan` null là KHÔNG ÁP DỤNG, `han_xu_ly_xong` null là CHƯA CÓ.
 */
export function ThePhieu({ phieu }: { phieu: PhieuCuaToi }) {
  const giai_thich = giaiThichTrangThai(phieu.trang_thai);
  const linh_vuc =
    phieu.nhan_linh_vuc !== ""
      ? phieu.nhan_linh_vuc
      : phieu.linh_vuc !== ""
        ? THE_PHIEU.da_phan_loai
        : THE_PHIEU.chua_phan_loai;

  const nguoi_gui = phieu.an_danh
    ? THE_PHIEU.an_danh
    : [phieu.ho_ten_da_che, phieu.dien_thoai_da_che].filter((p) => p !== "").join(" · ") ||
      THE_PHIEU.khong_ghi_ten;

  return (
    <dl className="cd-phieu">
      <Dong nhan={THE_PHIEU.trang_thai}>
        <strong className="cd-phieu__trang-thai">{nhanTrangThai(phieu.trang_thai)}</strong>
        {giai_thich !== null && <span className="cd-phieu__phu">{giai_thich}</span>}
      </Dong>
      <Dong nhan={THE_PHIEU.linh_vuc}>{linh_vuc}</Dong>
      <Dong nhan={THE_PHIEU.gui_luc}>
        {moc(phieu.goc_dem_han)} {THE_PHIEU.gio_vn}
      </Dong>
      <Dong nhan={THE_PHIEU.han_xem}>
        {phieu.han_tiep_nhan === null
          ? THE_PHIEU.han_xem_khong_ap_dung
          : `${moc(phieu.han_tiep_nhan)} ${THE_PHIEU.gio_vn}`}
      </Dong>
      <Dong nhan={THE_PHIEU.han_xu_ly}>
        {phieu.han_xu_ly_xong === null
          ? THE_PHIEU.han_xu_ly_chua_co
          : `${moc(phieu.han_xu_ly_xong)} ${THE_PHIEU.gio_vn}`}
      </Dong>
      {phieu.ket_qua !== "" && (
        <Dong nhan={THE_PHIEU.ket_qua}>
          <span className="cd-phieu__ket-qua">{phieu.ket_qua}</span>
        </Dong>
      )}
      <Dong nhan={THE_PHIEU.noi_dung}>
        <span className="cd-phieu__van-ban">{phieu.noi_dung}</span>
      </Dong>
      <Dong nhan={THE_PHIEU.dia_chi}>{phieu.dia_chi !== "" ? phieu.dia_chi : THE_PHIEU.dia_chi_trong}</Dong>
      <Dong nhan={THE_PHIEU.nguoi_gui}>{nguoi_gui}</Dong>
    </dl>
  );
}
