/**
 * MẢNH GIAO DIỆN DÙNG CHUNG CỦA APP RIÊNG MỘT XÃ — theo bản mẫu `vi-gov/zalo-miniapp`
 * (`components/common.tsx`, `components/DataState.tsx`), lớp CSS `.xa-*` trong `styles.css`.
 *
 * Khác bản mẫu ở chỗ bắt buộc (`skills/accessibility-elderly`, `accessibility.test.ts`): chữ không dưới
 * 16px, đích chạm không dưới 48px, chữ phụ dùng `--ink-muted` (bản mẫu dùng một màu xám chỉ 3,2:1 trên
 * nền trắng), và không vòng quay động — "đang tải" là CHỮ.
 */
import type { ReactNode } from "react";

import { QUAY_LAI, XA_GIAO_DIEN } from "./noi-dung";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";

/** Header của màn con: nút quay lại + tiêu đề. Nút Back của Zalo đóng cả app, nên đây là đường về. */
export function DauManCon({ tieu_de, onQuayLai }: { tieu_de: string; onQuayLai: () => void }) {
  return (
    <div className="xa-dau-con">
      <button type="button" className="xa-dau-con__lui" onClick={onQuayLai}>
        <BieuTuong ten="back" co={22} />
        <span>{QUAY_LAI}</span>
      </button>
      <h1 className="xa-dau-con__tieu-de">{tieu_de}</h1>
    </div>
  );
}

/** Tiêu đề một khối trên trang chủ + "Xem tất cả". */
export function DauKhoi({ tieu_de, onXemTatCa }: { tieu_de: string; onXemTatCa?: () => void }) {
  return (
    <div className="xa-dau-khoi">
      <h2 className="xa-dau-khoi__tieu-de">{tieu_de}</h2>
      {onXemTatCa && (
        <button type="button" className="xa-dau-khoi__them" onClick={onXemTatCa}>
          {XA_GIAO_DIEN.xem_tat_ca}
        </button>
      )}
    </div>
  );
}

/** Ô biểu tượng tròn nền nhạt; màu là tên một lớp `xa-mau--*` đã đo trong `styles.css`. */
export function OBieuTuong({ ten, mau }: { ten: TenBieuTuong; mau: "hong" | "xanh" | "luc" | "cam" | "navy" }) {
  return (
    <span className={`xa-o-bt xa-mau--${mau}`}>
      <BieuTuong ten={ten} co={24} />
    </span>
  );
}

/** Khối trạng thái: đang tải · rỗng · lỗi (kèm nút). Luôn là chữ đọc được, không chỉ biểu tượng. */
export function KhoiTrangThai(props: {
  bieu_tuong: TenBieuTuong;
  cau: string;
  loi?: boolean;
  dang_tai?: boolean;
  nut?: { nhan: string; onBam: () => void };
}) {
  return (
    <div className={`xa-trang-thai${props.loi ? " xa-trang-thai--loi" : ""}`}>
      <BieuTuong ten={props.bieu_tuong} co={36} />
      <p role={props.loi ? "alert" : props.dang_tai ? "status" : undefined}>{props.cau}</p>
      {props.nut && (
        <button type="button" className="xa-nut" onClick={props.nut.onBam}>
          {props.nut.nhan}
        </button>
      )}
    </div>
  );
}

/**
 * Chỗ của một việc cần đăng nhập (gửi, xem, tra cứu phản ánh) khi app riêng chưa có phiên. Đường đăng
 * nhập theo App ID của app xã CHƯA DỰNG (ADR 0047 §6); nói thật điều ấy, và chỉ người dân tới xã.
 */
export function ChuaDangNhap({ cau }: { cau: string }) {
  return (
    <div className="xa-the xa-the--dem xa-ghi-chu">
      <BieuTuong ten="info" co={22} />
      <div>
        <p className="xa-ghi-chu__tieu-de">{XA_GIAO_DIEN.chua_dang_nhap_tieu_de}</p>
        <p>{cau}</p>
      </div>
    </div>
  );
}

export function TrangCon({ children }: { children: ReactNode }) {
  return <div className="xa-trang xa-trang--con">{children}</div>;
}
