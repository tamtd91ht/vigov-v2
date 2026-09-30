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
  // "Gửi phản ánh mới" at the foot of the Phản ánh tab — the prototype's `Plus`.
  plus: <path d="M12,5v14M5,12h14" />,
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

  /*
   * THE PROTOTYPE'S SET (`PROTOTYPE.md` §5.4, §6, §8 — lucide shapes on a 24 grid), drawn here once so the
   * screen waves after the frame only pick names. Already above under another name, so NOT drawn twice:
   * House → `home` · User → `user` · Phone → `phone` · Bell → `bell` · MapPin → `pin` · Clock → `clock` ·
   * AlertTriangle → `alert` · Send → `send` · Search → `search` · Plus → `plus`. No eye: no view counts.
   */
  // Home tile "Gửi phản ánh": a camera with a pin in the lens — a photo OF A PLACE.
  "camera-pin": (
    <>
      <path d="M14.5,4h-5L7,7H4a2,2,0,0,0,-2,2v9a2,2,0,0,0,2,2h16a2,2,0,0,0,2,-2V9a2,2,0,0,0,-2,-2h-3z" />
      <path d="M12,17.2s2.9,-2.5,2.9,-4.5a2.9,2.9,0,0,0,-5.8,0c0,2,2.9,4.5,2.9,4.5z" />
      <circle cx="12" cy="12.7" r="0.8" />
    </>
  ),
  "file-search": (
    <>
      <path d="M14,2v4a2,2,0,0,0,2,2h4" />
      <path d="M4.3,21a2,2,0,0,0,1.7,1H18a2,2,0,0,0,2,-2V7l-5,-5H6a2,2,0,0,0,-2,2v3" />
      <path d="M9,18l-1.5,-1.5" />
      <circle cx="5" cy="14" r="3" />
    </>
  ),
  "contact-book": (
    <>
      <path d="M15,13a3,3,0,1,0,-6,0" />
      <path d="M4,19.5v-15A2.5,2.5,0,0,1,6.5,2H19a1,1,0,0,1,1,1v18a1,1,0,0,1,-1,1H6.5a1,1,0,0,1,0,-5H20" />
      <circle cx="12" cy="8" r="2" />
    </>
  ),
  // Truyền thanh: a loudspeaker with two waves (lucide Volume2).
  speaker: (
    <>
      <path d="M11,4.7a.7.7,0,0,0,-1.2,-.5L6.4,7.6A1.4,1.4,0,0,1,5.4,8H3a1,1,0,0,0,-1,1v6a1,1,0,0,0,1,1h2.4a1.4,1.4,0,0,1,1,.4l3.4,3.4a.7.7,0,0,0,1.2,-.5z" />
      <path d="M16,9a5,5,0,0,1,0,6" />
      <path d="M19.4,18.4a9,9,0,0,0,0,-12.8" />
    </>
  ),
  video: (
    <>
      <path d="M16,13l5.2,3.5a.5.5,0,0,0,.8,-.4V7.9a.5.5,0,0,0,-.8,-.4L16,10.5" />
      <rect x="2" y="6" width="14" height="12" rx="2" />
    </>
  ),
  calendar: (
    <>
      <path d="M8,2v4M16,2v4" />
      <rect x="3" y="4" width="18" height="18" rx="2" />
      <path d="M3,10h18" />
      <path d="M8,14h.01M12,14h.01M16,14h.01M8,18h.01M12,18h.01M16,18h.01" />
    </>
  ),
  newspaper: (
    <>
      <path d="M15,18h-5M18,14h-8" />
      <path d="M4,22h16a2,2,0,0,0,2,-2V4a2,2,0,0,0,-2,-2H8a2,2,0,0,0,-2,2v16a2,2,0,0,1,-4,0v-9a2,2,0,0,1,2,-2h2" />
      <rect x="10" y="6" width="8" height="4" rx="1" />
    </>
  ),
  // Bottom tab "Phản ánh": a speech bubble with an exclamation mark (lucide MessageSquareWarning).
  "message-square-warning": (
    <>
      <path d="M21,15a2,2,0,0,1,-2,2H7l-4,4V5a2,2,0,0,1,2,-2h14a2,2,0,0,1,2,2z" />
      <path d="M12,7v3.5M12,13.5h.01" />
    </>
  ),
  "message-square": <path d="M21,15a2,2,0,0,1,-2,2H7l-4,4V5a2,2,0,0,1,2,-2h14a2,2,0,0,1,2,2z" />,
  "message-square-plus": (
    <>
      <path d="M21,15a2,2,0,0,1,-2,2H7l-4,4V5a2,2,0,0,1,2,-2h14a2,2,0,0,1,2,2z" />
      <path d="M12,7v6M9,10h6" />
    </>
  ),
  "message-circle": <path d="M7.9,20A9,9,0,1,0,4,16.1L2,22z" />,
  "wifi-off": (
    <>
      <path d="M12,20h.01" />
      <path d="M8.5,16.4a5,5,0,0,1,7,0" />
      <path d="M5,12.9a10,10,0,0,1,5.2,-2.7" />
      <path d="M19,12.9a10,10,0,0,0,-2,-1.5" />
      <path d="M2,8.8a15,15,0,0,1,4.2,-2.6" />
      <path d="M22,8.8a15,15,0,0,0,-11.3,-3.8" />
      <path d="M2,2l20,20" />
    </>
  ),
  "check-circle": (
    <>
      <circle cx="12" cy="12" r="10" />
      <path d="M9,12l2,2,4,-4" />
    </>
  ),
  "chevron-right": <path d="M9,18l6,-6,-6,-6" />,
  "chevron-left": <path d="M15,18l-6,-6,6,-6" />,
  refresh: (
    <>
      <path d="M3,12a9,9,0,0,1,9,-9,9.8,9.8,0,0,1,6.7,2.7L21,8" />
      <path d="M21,3v5h-5" />
      <path d="M21,12a9,9,0,0,1,-9,9,9.8,9.8,0,0,1,-6.7,-2.7L3,16" />
      <path d="M8,16H3v5" />
    </>
  ),
  // The twelve field icons of the platform's catalogue (`PROTOTYPE.md` §8).
  trash: (
    <>
      <path d="M3,6h18" />
      <path d="M19,6v14c0,1,-1,2,-2,2H7c-1,0,-2,-1,-2,-2V6" />
      <path d="M8,6V4c0,-1,1,-2,2,-2h4c1,0,2,1,2,2v2" />
      <path d="M10,11v6M14,11v6" />
    </>
  ),
  "traffic-cone": (
    <>
      <path d="M9.3,6.2a4.6,4.6,0,0,0,5.4,0" />
      <path d="M7.9,10.7c.9,.8,2.4,1.3,4.1,1.3s3.2,-.5,4.1,-1.3" />
      <path d="M13.9,3.5a1.9,1.9,0,0,0,-3.8,-.1l-3,10c-.1,.2,-.1,.4,-.1,.6,0,1.7,2.2,3,5,3s5,-1.3,5,-3c0,-.2,0,-.4,-.1,-.5z" />
      <path d="M7.5,12.2l-4.7,2.7c-.5,.3,-.8,.7,-.8,1.1s.3,.8,.8,1.1l7.6,4.5c.9,.5,2.1,.5,3,0l7.6,-4.5c.7,-.3,1,-.7,1,-1.1s-.3,-.8,-.8,-1.1l-4.7,-2.8" />
    </>
  ),
  leaf: (
    <>
      <path d="M11,20A7,7,0,0,1,9.8,6.1C15.5,5,17,4.5,19,2c1,2,2,4.2,2,8,0,5.5,-4.8,10,-10,10z" />
      <path d="M2,21c0,-3,1.9,-5.4,5.1,-6C9.5,14.5,12,13,13,12" />
    </>
  ),
  droplets: (
    <>
      <path d="M7,16.3c2.2,0,4,-1.8,4,-4.1,0,-1.2,-.6,-2.3,-1.7,-3.2S7.3,6.8,7,5.3c-.3,1.5,-1.1,2.8,-2.3,3.8S3,11.1,3,12.3c0,2.2,1.8,4,4,4z" />
      <path d="M12.6,6.6A11,11,0,0,0,14,3c.5,2.5,2,4.9,4,6.5s3,3.5,3,5.5a7,7,0,0,1,-11.9,5" />
    </>
  ),
  lightbulb: (
    <>
      <path d="M15,14c.2,-1,.7,-1.7,1.5,-2.5,1,-.9,1.5,-2.2,1.5,-3.5A6,6,0,0,0,6,8c0,1,.2,2.2,1.5,3.5,.7,.7,1.3,1.5,1.5,2.5" />
      <path d="M9,18h6M10,22h4" />
    </>
  ),
  store: (
    <>
      <path d="M2,7l4.4,-4.4A2,2,0,0,1,7.8,2h8.4a2,2,0,0,1,1.4,.6L22,7" />
      <path d="M4,12v8a2,2,0,0,0,2,2h12a2,2,0,0,0,2,-2v-8" />
      <path d="M15,22v-4a2,2,0,0,0,-2,-2h-2a2,2,0,0,0,-2,2v4" />
      <path d="M2,7h20v3a2,2,0,0,1,-2,2,2.7,2.7,0,0,1,-2,-.8,2.7,2.7,0,0,1,-4,0,2.7,2.7,0,0,1,-4,0,2.7,2.7,0,0,1,-4,0,2.7,2.7,0,0,1,-2,.8,2,2,0,0,1,-2,-2z" />
    </>
  ),
  "shield-alert": (
    <>
      <path d="M20,13c0,5,-3.5,7.5,-7.7,9a1,1,0,0,1,-.7,0C7.5,20.5,4,18,4,13V6a1,1,0,0,1,1,-1c2,0,4.5,-1.2,6.2,-2.7a1.2,1.2,0,0,1,1.5,0C14.5,3.8,17,5,19,5a1,1,0,0,1,1,1z" />
      <path d="M12,8v4M12,16h.01" />
    </>
  ),
  construction: (
    <>
      <rect x="2" y="6" width="20" height="8" rx="1" />
      <path d="M17,14v7M7,14v7M17,3v3M7,3v3" />
      <path d="M10,14L2.3,6.3M14,6l7.7,7.7M8,6l8,8" />
    </>
  ),
  factory: (
    <>
      <path d="M2,20a2,2,0,0,0,2,2h16a2,2,0,0,0,2,-2V8l-7,5V8l-7,5V4a2,2,0,0,0,-2,-2H4a2,2,0,0,0,-2,2z" />
      <path d="M17,18h1M12,18h1M7,18h1" />
    </>
  ),
  hammer: (
    <>
      <path d="M15,12l-8.4,8.4a1,1,0,1,1,-3,-3L12,9" />
      <path d="M18,15l4,-4" />
      <path d="M21.5,11.5l-1.9,-1.9A2,2,0,0,1,19,8.2V7l-2.3,-2.3a6,6,0,0,0,-4.2,-1.8L9,3l.9,.8A6.2,6.2,0,0,1,12,8.4V10l2,2h1.2a2,2,0,0,1,1.4,.6l1.9,1.9" />
    </>
  ),
  stethoscope: (
    <>
      <path d="M11,2v2M5,2v2" />
      <path d="M5,3H4a2,2,0,0,0,-2,2v4a6,6,0,0,0,12,0V5a2,2,0,0,0,-2,-2h-1" />
      <path d="M8,15a6,6,0,0,0,12,0v-3" />
      <circle cx="20" cy="10" r="2" />
    </>
  ),
  "user-round-x": (
    <>
      <path d="M2,21a8,8,0,0,1,11.9,-7" />
      <circle cx="10" cy="8" r="5" />
      <path d="M17,17l5,5M22,17l-5,5" />
    </>
  ),
  utensils: (
    <>
      <path d="M3,2v7c0,1.1,.9,2,2,2h4a2,2,0,0,0,2,-2V2" />
      <path d="M7,2v20" />
      <path d="M21,15V2a5,5,0,0,0,-5,5v6c0,1.1,.9,2,2,2h3zm0,0v7" />
    </>
  ),
  zap: (
    <path d="M4,14a1,1,0,0,1,-.8,-1.6l9.9,-10.2a.5.5,0,0,1,.9,.5l-1.9,6A1,1,0,0,0,13,10h7a1,1,0,0,1,.8,1.6l-9.9,10.2a.5.5,0,0,1,-.9,-.5l1.9,-6A1,1,0,0,0,11,14z" />
  ),
  // Disaster alert band (§6.1): rain from a cloud.
  "cloud-rain": (
    <>
      <path d="M4,14.9A7,7,0,1,1,15.7,8h1.8a4.5,4.5,0,0,1,2.5,8.2" />
      <path d="M16,14v6M8,14v6M12,16v6" />
    </>
  ),
  // Broadcast player (§6.10): a bare triangle and two bars — `play` above is the circled Video mark.
  "play-triangle": <path d="M6,3l14,9,-14,9z" />,
  pause: (
    <>
      <rect x="14" y="4" width="4" height="16" rx="1" />
      <rect x="6" y="4" width="4" height="16" rx="1" />
    </>
  ),
  "rotate-ccw": (
    <>
      <path d="M3,12a9,9,0,1,0,9,-9,9.8,9.8,0,0,0,-6.7,2.7L3,8" />
      <path d="M3,3v5h5" />
    </>
  ),
  "rotate-cw": (
    <>
      <path d="M21,12a9,9,0,1,1,-9,-9c2.5,0,4.9,1,6.7,2.7L21,8" />
      <path d="M21,3v5h-5" />
    </>
  ),
  share: (
    <>
      <circle cx="18" cy="5" r="3" />
      <circle cx="6" cy="12" r="3" />
      <circle cx="18" cy="19" r="3" />
      <path d="M8.6,13.5l6.8,4M15.4,6.5l-6.8,4" />
    </>
  ),
  // Cá nhân "Ngôn ngữ".
  globe: (
    <>
      <circle cx="12" cy="12" r="10" />
      <path d="M12,2a14.5,14.5,0,0,0,0,20,14.5,14.5,0,0,0,0,-20" />
      <path d="M2,12h20" />
    </>
  ),
} as const satisfies Record<string, ReactNode>;

export type TenBieuTuong = keyof typeof HINH;

/**
 * `stroke`: line weight. 1.8 everywhere; the current bottom tab draws heavier (2.4, `PROTOTYPE.md` §5.4) as a
 * second signal beside its bold label and `aria-current` — never the only one.
 */
export function BieuTuong({ ten, co = 22, stroke = 1.8 }: { ten: TenBieuTuong; co?: number; stroke?: number }) {
  return (
    <svg
      className="xa-bt"
      width={co}
      height={co}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={stroke}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {HINH[ten]}
    </svg>
  );
}
