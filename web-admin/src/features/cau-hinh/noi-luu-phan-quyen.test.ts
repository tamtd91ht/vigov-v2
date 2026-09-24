import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * DÂY NỐI giữa nút `Lưu` của ma trận và tuyến `PUT /api/v1/roles/{id}/permissions`.
 *
 * `sua-phan-quyen.test.ts` chứng minh `guiCot` gửi đúng MỘT cột. Nó không chứng minh được
 * `MaTranPhanQuyen` ĐI QUA `guiCot`: hàm `luuCot` của component chạy trong một sự kiện bấm, thứ
 * `renderToStaticMarkup` không có, và kho cố ý không dùng jsdom (`vitest.config.ts`). Đo 24/09/2026
 * bằng bản chép tạm: thay lời gọi ấy bằng `luuPhanQuyenVaiTro(vaiTroId, { permissions: <mọi khoá
 * của bangHienThi> })` — tức gửi CẢ MA TRẬN cho một vai trò — thì mọi ca kiểm và `tsc` đều XANH.
 *
 * Trên tuyến này, thân là TOÀN BỘ tập quyền của vai trò: gửi thừa là CẤP lặng lẽ, gửi thiếu là GỠ
 * lặng lẽ, và máy chủ lưu đúng thứ nhận được. Nên dây nối được kiểm bằng cách đọc mã nguồn, theo
 * tiền lệ `src/ranh-gioi-nguon.test.ts`.
 */

const NGUON = readFileSync(
  fileURLToPath(new URL("./ma-tran-phan-quyen.tsx", import.meta.url)),
  "utf8",
)
  .replace(/\/\*[\s\S]*?\*\//g, "")
  .split("\n")
  .filter((d) => !/^\s*\/\//.test(d))
  .join("\n");

describe("MaTranPhanQuyen gửi đúng một cột, qua đúng một đường", () => {
  it("hàm ghi chỉ được trao cho `guiCot` — không có lời gọi trực tiếp nào tự dựng thân", () => {
    const moiLan = [...NGUON.matchAll(/luuPhanQuyenVaiTro/g)].length;
    // Một lần ở dòng import, một lần làm đối số của `guiCot`. Không lần nào khác.
    expect(moiLan).toBe(2);
    expect(NGUON).toContain("await guiCot(b, vaiTroId, luuPhanQuyenVaiTro)");
  });

  it("component không tự dựng thân `permissions`", () => {
    expect(NGUON).not.toMatch(/permissions\s*:/);
    expect(NGUON).not.toContain("thanLuuCot");
  });

  it("phản hồi áp vào ĐÚNG cột đã gửi, qua `ketThucLuu`", () => {
    expect(NGUON).toContain("ketThucLuu(x, vaiTroId, kq)");
  });
});
