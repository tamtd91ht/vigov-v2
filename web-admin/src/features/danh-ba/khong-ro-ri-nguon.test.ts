import { readFileSync, readdirSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * CHỮ TÌM CỦA DANH BẠ KHÔNG ĐƯỢC CÓ CHỖ NÀO ĐỂ NẰM LẠI NGOÀI STATE CỦA REACT.
 *
 * Chữ tìm là họ tên hoặc số điện thoại (luật 3, cấm #4); người dùng chốt 24/09 là nó sống trong
 * state, không vào router, không vào log. `can-bo.test.ts` giữ đường MẠNG (không URL nào mang chữ);
 * `danh-ba-lien-he.luong.test.tsx` giữ đường console khi chạy. Tệp này giữ phần còn lại — những lối
 * mà một dòng "tiện tay" mở ra và không ca chạy nào đi qua: lưu bộ lọc vào `localStorage` để giữ
 * khi tải lại, đẩy lên `useSearchParams` để chia sẻ đường dẫn, một `console.log` lúc gỡ lỗi.
 *
 * ĐÂY LÀ PHÉP KIỂM CÚ PHÁP, và giới hạn của nó nói thẳng: một lời gọi đi qua một biến trung gian
 * (`const s = window["local" + "Storage"]`) lọt qua. Nó bắt dạng thường gặp — đúng dạng một người
 * viết khi không nghĩ tới luật 3 — chứ không chứng minh là không có đường nào.
 *
 * Cùng khuôn `features/noi-dung/ranh-gioi-html.test.ts`: bỏ chú thích trước khi quét (chính các tệp
 * bị quét giải thích vì sao không dùng `history.state`), và bộ quét rỗng phải đỏ.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

const GOC = fileURLToPath(new URL("../../", import.meta.url));

/** Mã của màn Danh bạ và tuyến gọi của nó. */
const PHAM_VI = [
  fileURLToPath(new URL(".", import.meta.url)),
  fileURLToPath(new URL("../../components/danh-ba/", import.meta.url)),
  fileURLToPath(new URL("../../app/danh-ba/", import.meta.url)),
  fileURLToPath(new URL("../../lib/api/can-bo.ts", import.meta.url)),
];

function boChuThich(noiDung: string): string {
  return noiDung
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .filter((dong) => !/^\s*\/\//.test(dong))
    .join("\n");
}

function moiTep(): { duongDan: string; noiDung: string }[] {
  const ra: { duongDan: string; noiDung: string }[] = [];
  const themTep = (day: string) => {
    if (![".ts", ".tsx"].includes(extname(day))) return;
    if (day.endsWith(".test.ts") || day.endsWith(".test.tsx")) return;
    ra.push({
      duongDan: relative(GOC, day).replace(/\\/g, "/"),
      noiDung: boChuThich(readFileSync(day, "utf8")),
    });
  };
  const duyet = (duong: string) => {
    if (!statSync(duong).isDirectory()) {
      themTep(duong);
      return;
    }
    for (const m of readdirSync(duong, { withFileTypes: true })) {
      const day = join(duong, String(m.name));
      if (m.isDirectory()) duyet(day);
      else themTep(day);
    }
  };
  for (const p of PHAM_VI) duyet(p);
  return ra;
}

const TEP = moiTep();

/** Mỗi lối, và vì sao nó là một chỗ chữ tìm nằm lại. */
const LOI_CAM: readonly { mau: RegExp; viSao: string }[] = [
  { mau: /\bconsole\s*\./, viSao: "log trình duyệt được công cụ theo dõi thu về" },
  { mau: /\b(localStorage|sessionStorage|indexedDB)\b/, viSao: "nằm lại trên máy dùng chung sau khi đăng xuất" },
  { mau: /\bdocument\s*\.\s*cookie\b/, viSao: "đi theo mọi yêu cầu, nằm lại trên máy" },
  { mau: /["']next\/navigation["']|["']next\/router["']/, viSao: "router đưa trạng thái lên thanh địa chỉ" },
  { mau: /\b(pushState|replaceState)\s*\(/, viSao: "lịch sử trình duyệt" },
  { mau: /\blocation\s*\.\s*(search|hash|href)\s*=/, viSao: "thanh địa chỉ" },
];

describe("màn Danh bạ — không lối nào để chữ tìm nằm lại ngoài state", () => {
  it("quét được đủ tệp — một bộ quét rỗng là một bộ quét luôn xanh", () => {
    const ds = TEP.map((t) => t.duongDan);
    expect(ds).toContain("features/danh-ba/danh-ba-lien-he.tsx");
    expect(ds).toContain("features/danh-ba/loc-danh-ba.ts");
    expect(ds).toContain("lib/api/can-bo.ts");
    expect(ds).toContain("components/danh-ba/bieu-mau-ghi-can-bo.tsx");
  });

  for (const { mau, viSao } of LOI_CAM) {
    it(`không tệp nào khớp ${String(mau)} — ${viSao}`, () => {
      expect(TEP.filter((t) => mau.test(t.noiDung)).map((t) => t.duongDan)).toEqual([]);
    });
  }
});
