import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { nhanTrangThai } from "./nhan-nhiem-vu";

/**
 * DÂY NỐI của bảng nhãn xã bên trong `SoNhiemVu` — thứ `nhan-trang-thai-xa.test.tsx` không thấy được.
 *
 * Tệp kia kết xuất TỪNG component con với một bảng nhãn nó tự đưa vào, nên nó chứng minh "con vẽ
 * đúng bảng được truyền". Nó không chứng minh được `SoNhiemVu` truyền ĐÚNG bảng: component cha đọc
 * `GET /task-statuses` trong `useEffect`, thứ `renderToStaticMarkup` không chạy (xem chú thích của
 * `CanhBaoNhanTrangThai`). Đo 24/09/2026 bằng bản chép tạm:
 *
 *   - đổi `nhanTT={nhanTT}` của `<BangKanban>` thành `nhanTT={BANG_NHAN_MAC_DINH}` → mọi ca XANH
 *   - xoá `<CanhBaoNhanTrangThai canhBao={canhBaoNhanTT} />` → mọi ca XANH
 *
 * Ca thứ nhất là "một màn còn hiện nhãn gõ cứng" (hai nguồn cho một chữ, quyết định #21); ca thứ hai
 * là "lui về mặc định mà không nói" — cả hai đều im lặng trên màn hình của một xã đã đổi nhãn. Không
 * có jsdom (cố ý, `vitest.config.ts`), nên dây nối được kiểm bằng cách đọc mã nguồn, theo tiền lệ
 * `src/ranh-gioi-nguon.test.ts`.
 */

const NGUON = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8")
  // Bỏ chú thích: chú thích của tệp này GIẢI THÍCH nhãn mặc định và nhắc tên nó.
  .replace(/\/\*[\s\S]*?\*\//g, "")
  .replace(/\{\/\*[\s\S]*?\*\/\}/g, "")
  .split("\n")
  .filter((d) => !/^\s*\/\//.test(d))
  .join("\n");

describe("SoNhiemVu nối bảng nhãn xã tới MỌI chỗ vẽ", () => {
  it("màn hình không tự chọn bảng mặc định — chỉ `docBangNhanTrangThai` được chọn đường lui", () => {
    expect(NGUON).not.toContain("BANG_NHAN_MAC_DINH");
  });

  it("bảng vẽ ra là bảng đọc từ `GET /task-statuses`, qua đúng một phép đọc", () => {
    expect(NGUON).toMatch(/layTrangThaiNhiemVu\(\),?\s*\]\)\.then\(/);
    expect(NGUON).toContain("datKqNhanTT(nhanTT)");
    expect(NGUON).toContain(
      "const { bang: nhanTT, canhBao: canhBaoNhanTT } = docBangNhanTrangThai(kqNhanTT);",
    );
  });

  it("câu cảnh báo đường lui LÊN TRANG, mang đúng cảnh báo vừa đọc", () => {
    expect(NGUON).toContain("<CanhBaoNhanTrangThai canhBao={canhBaoNhanTT} />");
  });

  it("mọi prop `nhanTT` đều nhận bảng của xã — không chỗ nào nhận một bảng khác", () => {
    const moiProp = [...NGUON.matchAll(/nhanTT=\{([^}]*)\}/g)].map((m) => m[1]);
    // Bốn chỗ: ô lọc, Kanban, bảng Danh sách, drawer. Ít hơn = một chỗ đã tách khỏi bảng xã.
    expect(moiProp.length).toBeGreaterThanOrEqual(4);
    expect(moiProp.filter((v) => v !== "nhanTT")).toEqual([]);
  });

  it("mọi lời gọi `nhanTrangThai(` đọc bảng `nhanTT` — không bảng nào khác", () => {
    const doiSoDau = [...NGUON.matchAll(/nhanTrangThai\(\s*([^,)]*)/g)].map((m) => m[1]);
    expect(doiSoDau.length).toBeGreaterThan(0);
    expect(doiSoDau.filter((v) => v !== "nhanTT")).toEqual([]);
  });

  it("`nhanTrangThai` không có tham số mặc định — quên truyền bảng phải là lỗi, không phải chữ mặc định", () => {
    // `Function.length` đếm tham số TRƯỚC tham số mặc định đầu tiên: `(bang = X, ma)` cho 0.
    expect(nhanTrangThai.length).toBe(2);
  });
});
