import { describe, expect, it } from "vitest";

import { coTrangTruoc, sangTrangSau, veTrangTruoc, TRANG_DAU } from "./ngan-xep-con-tro";

describe("ngăn xếp con trỏ", () => {
  it("trang đầu không có gì để lùi", () => {
    expect(coTrangTruoc(TRANG_DAU)).toBe(false);
    expect(TRANG_DAU.hienTai).toBeNull();
  });

  it("lùi từ trang đầu thì đứng yên, không rơi vào trạng thái lạ", () => {
    expect(veTrangTruoc(TRANG_DAU)).toEqual(TRANG_DAU);
  });

  it("đi ba trang rồi lùi ba lần thì về đúng trang đầu", () => {
    const t2 = sangTrangSau(TRANG_DAU, "MOC-1");
    const t3 = sangTrangSau(t2, "MOC-2");

    expect(t3.hienTai).toBe("MOC-2");
    expect(veTrangTruoc(t3)).toEqual(t2);
    expect(veTrangTruoc(veTrangTruoc(t3))).toEqual(TRANG_DAU);
  });

  it("ngăn xếp không bị sửa tại chỗ — mỗi bước là một giá trị mới", () => {
    const sau = sangTrangSau(TRANG_DAU, "MOC-1");
    expect(TRANG_DAU.daQua).toEqual([]);
    expect(sau.daQua).toEqual([null]);
  });

  it("con trỏ rỗng là lỗi lập trình: hỏng ngay, không âm thầm về trang đầu", () => {
    // Hợp đồng chỉ phát ra `next_cursor` không rỗng khi `has_more` là true. Nếu nuốt ca này thì
    // giao diện hiện trang 1 trong khi cán bộ tin mình vừa bấm sang trang 4.
    expect(() => sangTrangSau(TRANG_DAU, "")).toThrow();
  });
});
