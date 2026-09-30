import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * DÂY NỐI của khoá chống trùng trong tab Sơ đồ tổ chức.
 *
 * `cay-bo-phan.test.ts` chứng minh `guiBieuMau` dùng `dangMo.khoaChongTrung` và `moThem` sinh khoá
 * đúng một lần. Nó không chứng minh được TAB sinh khoá ở lúc MỞ: hàm `gui` chạy trong sự kiện bấm,
 * thứ `renderToStaticMarkup` không có. Đo 24/09/2026 bằng bản chép tạm: đổi lời gọi thành
 * `guiBieuMau({ ...dangMo, khoaChongTrung: crypto.randomUUID() }, …)` — khoá mới MỖI LẦN BẤM — thì
 * mọi ca kiểm và `tsc` đều XANH.
 *
 * Khoá mới mỗi lần bấm là: lần đầu tới máy chủ nhưng phản hồi rơi giữa đường, cán bộ bấm lại, và sơ
 * đồ của xã có HAI bộ phận cùng tên — không ai thấy lỗi nào, chỉ thấy một cây sai.
 */

const NGUON = readFileSync(
  fileURLToPath(new URL("./tab-so-do-to-chuc.tsx", import.meta.url)),
  "utf8",
)
  .replace(/\/\*[\s\S]*?\*\//g, "")
  .split("\n")
  .filter((d) => !/^\s*\/\//.test(d))
  .join("\n");

describe("tab Sơ đồ tổ chức sinh khoá chống trùng lúc MỞ biểu mẫu, không lúc bấm Lưu", () => {
  it("mọi lần sinh khoá nằm trong `moThem(...)` — hai nút mở: Thêm bộ phận và ＋ bộ phận con", () => {
    const sinhKhoa = [...NGUON.matchAll(/randomUUID/g)].length;
    const trongMoThem = [...NGUON.matchAll(/moThem\([^;]*?\(\) => crypto\.randomUUID\(\)\)/g)].length;
    expect(sinhKhoa).toBe(2);
    expect(trongMoThem).toBe(2);
  });

  it("Lưu gửi NGUYÊN biểu mẫu đang mở — không dựng lại, không thay khoá", () => {
    expect(NGUON).toContain("guiBieuMau(dangMo, ban, API_SO_DO)");
    expect(NGUON).not.toMatch(/khoaChongTrung\s*:/);
  });
});
