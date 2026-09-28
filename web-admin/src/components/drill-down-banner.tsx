import Link from "next/link";

import {
  INVALID_DRILL_DOWN_LINE,
  type DrillDown,
  type DrillDownRegister,
} from "@/lib/drill-down";

/** Tên nút trở về danh sách thường. */
export const CLEAR_DRILL_DOWN_LABEL = "Bỏ lọc";

/**
 * Dải "Đang xem: <số liệu> — <kỳ>" trên đầu một danh sách mở từ trang Tổng quan (SRS M7.2.2).
 *
 * NÓI RA THÀNH CHỮ, CẢ KHI ĐƯỜNG DẪN HỎNG. Danh sách lọc theo một con số mà không có dải này thì cán
 * bộ không biết mình đang xem một lát cắt; đường dẫn hỏng mà không có câu `invalid` thì cán bộ tin
 * cả quyển sổ là lát cắt họ vừa bấm. Cả hai là cùng một con số bị đọc sai.
 *
 * `Bỏ lọc` LÀ MỘT LIÊN KẾT TỚI ĐƯỜNG DẪN TRƠN, không phải `router.push`: nó thay cả trang bằng danh
 * sách thường, và màn hình không ghi trạng thái nào của riêng nó lên thanh địa chỉ.
 *
 * `note` là câu của TỪNG màn về những ô lọc nó tạm tắt — mỗi màn tắt một bộ khác nhau.
 */
export function DrillDownBanner<R extends DrillDownRegister>({
  drillDown,
  clearHref,
  note,
  showInvalid = true,
}: {
  drillDown: DrillDown<R>;
  /** Đường dẫn trơn của màn, không tham số (`/nhiem-vu`). */
  clearHref: string;
  note: string;
  /** `false` khi cán bộ đã tự đổi bộ lọc — câu "đang hiện toàn bộ" lúc ấy không còn đúng. */
  showInvalid?: boolean;
}) {
  if (drillDown.kind === "none") return null;
  if (drillDown.kind === "invalid") {
    if (!showInvalid) return null;
    return (
      <p className="canh-bao-pham-vi" role="status">
        {INVALID_DRILL_DOWN_LINE}
      </p>
    );
  }
  return (
    <div className="canh-bao-pham-vi" role="status">
      <p>
        <strong>
          Đang xem: {drillDown.label} — {drillDown.periodLabel}
        </strong>
      </p>
      <p className="ghi-chu">{note}</p>
      <p className="cum-nut">
        <Link href={clearHref} className="nut-phu">
          {CLEAR_DRILL_DOWN_LABEL}
        </Link>
      </p>
    </div>
  );
}
