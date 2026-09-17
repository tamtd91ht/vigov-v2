import { describe, expect, it } from "vitest";

import { duongDanTiepTuc } from "./duong-dan-tiep-tuc";

describe("duongDanTiepTuc", () => {
  it("giữ đường dẫn nội bộ, kèm cả query", () => {
    expect(duongDanTiepTuc("/nhiem-vu?trang=2")).toBe("/nhiem-vu?trang=2");
  });

  it.each([
    ["https://ten-mien-gia.example", "URL tuyệt đối"],
    ["//ten-mien-gia.example", "URL lược giao thức — trình duyệt đọc là máy chủ khác"],
    ["/\\ten-mien-gia.example", "dấu gạch ngược, vài trình duyệt đọc như //"],
    ["javascript:alert(1)", "lược đồ javascript"],
  ])("về trang chủ với %s (%s)", (tho) => {
    expect(duongDanTiepTuc(tho)).toBe("/");
  });

  it("không quay lại chính trang đăng nhập — đó là vòng lặp không thoát ra được", () => {
    expect(duongDanTiepTuc("/dang-nhap")).toBe("/");
    expect(duongDanTiepTuc("/dang-nhap?tiep-tuc=/")).toBe("/");
  });

  it("rỗng hoặc không có thì về trang chủ", () => {
    expect(duongDanTiepTuc(null)).toBe("/");
    expect(duongDanTiepTuc("")).toBe("/");
    expect(duongDanTiepTuc(undefined)).toBe("/");
  });
});
