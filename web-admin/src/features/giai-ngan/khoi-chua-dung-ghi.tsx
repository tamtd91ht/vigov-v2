import { PHAN_CHUA_DUNG_GHI } from "./nhan-ghi-giai-ngan";

/**
 * Những phần bản thiết kế đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã và không vẽ một nút chắc chắn hỏng.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc. Cùng khuôn `KhoiChuaDung` của màn Nội dung Mini App và màn Thu - Chi.
 *
 * KHÔNG PHẢI `"use client"`: nó không có trạng thái, không có sự kiện, không gọi hook nào — một
 * component máy chủ thuần, nhúng được vào cả hai màn.
 */
export function KhoiChuaDungGhi() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG_GHI.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và
        lý do
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG_GHI.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}
