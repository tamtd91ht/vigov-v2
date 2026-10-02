import { ChevronDown, Construction } from "lucide-react";

import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

/**
 * Khối "phần chưa dựng" ở cuối màn Cấu hình — cùng khuôn `<details className="khoi-chua-khai">`
 * của màn Phản ánh và màn Nội dung. Không gọi mạng, không cổng quyền: danh sách này nói về phần
 * mềm, không về dữ liệu của đơn vị.
 */
export function KhoiChuaDung() {
  // Collapsed, dashed, neutral grey (spec §8.1 — the same disclosure as Phản ánh and Danh bạ): a list
  // of what is not built is not an alarm. The words and the list are unchanged.
  return (
    <details className="khoi-chua-khai group mt-6 mb-0">
      <summary className="flex cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden">
        <Construction aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-ink-500" />
        <span className="font-semibold text-ink-700">
          {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý do
        </span>
        <ChevronDown
          aria-hidden="true"
          focusable="false"
          strokeWidth={1.8}
          className="ml-auto size-4 shrink-0 text-ink-500 transition-transform group-open:rotate-180"
        />
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}
