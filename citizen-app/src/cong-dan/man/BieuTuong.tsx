/**
 * BỘ BIỂU TƯỢNG CỦA APP RIÊNG MỘT XÃ — nét vẽ lấy từ bản mẫu `vi-gov/zalo-miniapp`
 * (`src/components/Icon.tsx`), chủ dự án chọn làm giao diện 28/09/2026.
 *
 * VIẾT LẠI BẰNG PHẦN TỬ JSX, KHÔNG CHÉP NGUYÊN: bản mẫu nhét chuỗi SVG qua `dangerouslySetInnerHTML`,
 * và nửa nhà nước cấm hẳn API ấy (`cong-khai.test.tsx`). Chuỗi ở đây là hằng, nhưng một ngoại lệ cho
 * "chuỗi hằng" là ngoại lệ ai đó sẽ nới cho chuỗi không hằng.
 *
 * Chỉ mang những hình app riêng thật sự vẽ. Biểu tượng luôn `aria-hidden`: nghĩa nằm ở chữ bên cạnh.
 */
import type { ReactNode } from "react";

const HINH = {
  home: (
    <>
      <path d="M3.5,10.5L12,3.5l8.5,7" />
      <path d="M5.5,9.5V20a1,1,0,0,0,1,1h11a1,1,0,0,0,1,-1V9.5" />
      <path d="M9.5,21v-6h5v6" />
    </>
  ),
  chat: (
    <>
      <path d="M21,14.5a2,2,0,0,1,-2,2H8l-4,3.5V5a2,2,0,0,1,2,-2h13a2,2,0,0,1,2,2z" />
      <path d="M8,8h9M8,11.5h6" />
    </>
  ),
  news: (
    <>
      <path d="M4,4.5A1.5,1.5,0,0,1,5.5,3H16v18H5.5A1.5,1.5,0,0,1,4,19.5z" />
      <path d="M16,8h4v11a2,2,0,0,1,-4,0" />
      <path d="M7.5,7.5h5M7.5,11h5M7.5,14.5h3" />
    </>
  ),
  users: (
    <>
      <circle cx="9.5" cy="8" r="3.3" />
      <path d="M3.5,20a6,6,0,0,1,12,0" />
      <path d="M16.5,5.2a3.3,3.3,0,0,1,0,6.4" />
      <path d="M18,20a6.2,6.2,0,0,0,-2.6,-4.6" />
    </>
  ),
  megaphone: (
    <>
      <path d="M4,9.5v5a1.5,1.5,0,0,0,1.5,1.5H8l7,4.5V5L8,9.5H5.5A1.5,1.5,0,0,0,4,11z" />
      <path d="M18,9.2a4.2,4.2,0,0,1,0,5.6" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="M20.5,20.5L16.7,16.7" />
    </>
  ),
  phone: (
    <path d="M20.5,16.9v2.6a1.8,1.8,0,0,1,-2,1.8,18.4,18.4,0,0,1,-8,-2.9,18,18,0,0,1,-5.5,-5.5,18.4,18.4,0,0,1,-2.9,-8.1,1.8,1.8,0,0,1,1.8,-2h2.6a1.8,1.8,0,0,1,1.8,1.6c.1.9.4,1.9.7,2.7a1.8,1.8,0,0,1,-.4,1.9l-1.1,1.1a14.6,14.6,0,0,0,5.5,5.5l1.1,-1.1a1.8,1.8,0,0,1,1.9,-.4c.9.3,1.8.6,2.7.7a1.8,1.8,0,0,1,1.6,1.9z" />
  ),
  back: <path d="M19,12H5M11,6l-6,6,6,6" />,
  right: <path d="M9,5.5l6.5,6.5L9,18.5" />,
  alert: (
    <>
      <path d="M12,3.4l8.8,15.6H3.2z" />
      <path d="M12,9.5v4.2M12,16.7h.01" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="8.6" />
      <path d="M12,11v5.2M12,7.8h.01" />
    </>
  ),
  build: (
    <>
      <path d="M3,21h18" />
      <path d="M5,21V8.5L12,4l7,4.5V21" />
      <path d="M9.5,21v-5h5v5" />
    </>
  ),
  bell: (
    <>
      <path d="M18,8.5a6,6,0,1,0,-12,0c0,6.5,-2.5,8.5,-2.5,8.5h17S18,15,18,8.5" />
      <path d="M13.8,20.5a2,2,0,0,1,-3.6,0" />
    </>
  ),
  radio: (
    <>
      <rect x="3" y="8" width="18" height="12.5" rx="2" />
      <path d="M7.5,8l10,-4" />
      <circle cx="16" cy="14.2" r="3" />
      <path d="M6.5,12.5h4M6.5,16h4" />
    </>
  ),
  play: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M10,8.5v7l5.5,-3.5z" />
    </>
  ),
  map: (
    <>
      <path d="M9,4L3,6.5v14L9,18l6,2.5,6,-2.5v-14L15,6.5z" />
      <path d="M9,4v14M15,6.5v14" />
    </>
  ),
  user: (
    <>
      <circle cx="12" cy="8.4" r="3.6" />
      <path d="M4.5,20.5a7.5,7.5,0,0,1,15,0" />
    </>
  ),
  pin: (
    <>
      <path d="M12,21s6.8,-6.3,6.8,-10.8A6.8,6.8,0,0,0,5.2,10.2C5.2,14.7,12,21,12,21z" />
      <circle cx="12" cy="10.2" r="2.4" />
    </>
  ),
  check: <path d="M4.5,12.5l5,5,10,-11" />,
  clock: (
    <>
      <circle cx="12" cy="12" r="8.6" />
      <path d="M12,7.2V12l3.1,2" />
    </>
  ),
  history: (
    <>
      <path d="M3.5,12a8.5,8.5,0,1,0,2.6,-6.1" />
      <path d="M3.2,4v4h4" />
      <path d="M12,7.6V12l3,2" />
    </>
  ),
  text: (
    <>
      <path d="M4,6.5V4.5h16v2" />
      <path d="M12,4.5v15M8.5,19.5h7" />
    </>
  ),
  logout: (
    <>
      <path d="M15,4.5h3a1.5,1.5,0,0,1,1.5,1.5v12a1.5,1.5,0,0,1,-1.5,1.5h-3" />
      <path d="M10.5,16.5L6,12l4.5,-4.5M6,12h10" />
    </>
  ),
  shield: (
    <>
      <path d="M12,3.2l7.5,3v5.4c0,4.6,-3.1,8,-7.5,9.2,-4.4,-1.2,-7.5,-4.6,-7.5,-9.2V6.2z" />
      <path d="M9,12l2.2,2.2L15.4,10" />
    </>
  ),
  send: <path d="M21,3.5L10.5,14M21,3.5l-6.6,17.2,-3.9,-6.7,-6.7,-3.9z" />,
  star: <path d="M12,3.5l2.6,5.3,5.9,0.9,-4.3,4.1,1,5.8,-5.2,-2.7,-5.2,2.7,1,-5.8,-4.3,-4.1,5.9,-0.9z" />,
} as const satisfies Record<string, ReactNode>;

export type TenBieuTuong = keyof typeof HINH;

export function BieuTuong({ ten, co = 22 }: { ten: TenBieuTuong; co?: number }) {
  return (
    <svg
      className="xa-bt"
      width={co}
      height={co}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {HINH[ten]}
    </svg>
  );
}
