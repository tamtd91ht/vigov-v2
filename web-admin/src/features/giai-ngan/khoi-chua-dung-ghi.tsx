import { ChevronDown, Construction } from "lucide-react";

import { PHAN_CHUA_DUNG_GHI } from "./nhan-ghi-giai-ngan";

/**
 * Những phần bản thiết kế đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã và không vẽ một nút chắc chắn hỏng.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc. Cùng khuôn `KhoiChuaDung` của màn Nội dung Mini App và màn Thu - Chi.
 *
 * COLLAPSED, GREY, DASHED (spec §8.1, ADR 0068): a list of what is not built is not an alarm. The
 * words stay verbatim and stay in the HTML while closed — `<details>` only folds them.
 *
 * KHÔNG PHẢI `"use client"`: nó không có trạng thái, không có sự kiện, không gọi hook nào — một
 * component máy chủ thuần, nhúng được vào cả hai màn.
 */
export function KhoiChuaDungGhi() {
  return (
    <details className="khoi-chua-khai group m-0">
      <summary className="flex cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden">
        <Construction aria-hidden="true" focusable="false" className="size-[18px] shrink-0 text-ink-500" />
        <span className="font-semibold text-ink-700">
          {PHAN_CHUA_DUNG_GHI.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và
          lý do
        </span>
        <ChevronDown
          aria-hidden="true"
          focusable="false"
          className="ml-auto size-4 shrink-0 text-ink-500 transition-transform group-open:rotate-180"
        />
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
