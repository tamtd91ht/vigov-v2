import type { NextConfig } from "next";

/**
 * Cấu hình build của web quản trị xã.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỀU QUAN TRỌNG NHẤT Ở TỆP NÀY LÀ THỨ **KHÔNG** CÓ Ở ĐÂY: không một `env` nào, và không
 * một `NEXT_PUBLIC_*` nào.
 *
 * Một biến mang tiền tố `NEXT_PUBLIC_` được **nung thẳng vào bundle trình duyệt** lúc build
 * (luật 8, bất biến 4). Nung một giá trị của xã vào bundle nghĩa là một ảnh Docker chỉ còn
 * phục vụ đúng một xã — trong khi cả kiến trúc này là MỘT tiến trình phục vụ 200+ xã, phân
 * biệt nhau bằng tên miền (luật 1, bất biến 10).
 *
 * Hỏng sẽ không lộ ra lúc build và cũng không lộ ra ở xã đầu tiên. Nó lộ ra ở xã THỨ HAI,
 * dưới dạng tên xã khác hiện trên màn hình của một cơ quan nhà nước — và tới lúc đó thì
 * ảnh ấy đã chạy ở mọi nơi.
 *
 * Mọi giá trị riêng của xã được đọc LÚC CHẠY, ở phía máy chủ, suy từ `Host`:
 * `src/lib/tenant.server.ts`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
const nextConfig: NextConfig = {
  // `standalone` gói sẵn đúng những phụ thuộc thật sự chạy vào `.next/standalone`, nên ảnh
  // phát hành không cần `node_modules` (vài trăm MB) lẫn mã nguồn. Ít thứ trong ảnh cũng là
  // ít thứ phải vá khi có CVE.
  output: "standalone",

  // Không lộ phiên bản Next qua header `X-Powered-By`. Không phải một biện pháp an ninh —
  // chỉ là không tự khai đúng thứ người dò tìm muốn biết.
  poweredByHeader: false,

  // React strict mode: cảnh báo sớm những mẫu sẽ hỏng khi render đồng thời.
  reactStrictMode: true,
};

export default nextConfig;
