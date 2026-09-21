import { describe, expect, it } from "vitest";

import { danhSachNam } from "./nam";

describe("cửa sổ năm của ô chọn", () => {
  it("giảm dần, năm sau đứng đầu", () => {
    // Năm sau có mặt vì kế hoạch vốn và thông báo nghỉ lễ của năm sau về vào quý IV — cán bộ
    // phải mở xem được nó trước khi năm ấy tới.
    expect(danhSachNam(2026)).toEqual([2027, 2026, 2025, 2024, 2023]);
  });

  it("không phụ thuộc đồng hồ: cùng một đầu vào luôn cho cùng một danh sách", () => {
    expect(danhSachNam(2030)).toEqual(danhSachNam(2030));
    expect(danhSachNam(2030)[0]).toBe(2031);
  });
});
