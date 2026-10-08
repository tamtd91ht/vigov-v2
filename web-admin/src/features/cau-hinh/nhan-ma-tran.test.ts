import { describe, expect, it } from "vitest";

import type { identity_vaiTroCotRa } from "@/lib/api/schema.gen";

import { nhanChuaCauHinh, nhanO, nhanSoNguoiGiuVaiTro } from "./nhan-ma-tran";

function cot(soCanBo: number, soTaiKhoanHoatDong: number): identity_vaiTroCotRa {
  return {
    id: "vt-chuyen-vien",
    code: "chuyen-vien",
    name: "Chuyên viên chuyên môn",
    is_leader: false,
    staff_count: soCanBo,
    active_account_count: soTaiKhoanHoatDong,
  };
}

// The prototype's role head (`RolePermissionMatrix.tsx:147-149`): ONE count, "{n} cán bộ" — owner
// decision 08/10/2026 (card B, P13) drops the active-account clause and the zero-holder sentence.
describe("đầu cột vai trò hiện MỘT số đếm, như prototype", () => {
  it("staff_count only — the active-account count is not printed", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(3, 1))).toBe("3 cán bộ");
    expect(nhanSoNguoiGiuVaiTro(cot(3, 0))).toBe("3 cán bộ");
  });

  it("zero holders reads '0 cán bộ', as the prototype prints it", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(0, 0))).toBe("0 cán bộ");
  });
});

describe("nhãn một ô", () => {
  it("hai câu khác nhau, và không câu nào là ô trống", () => {
    expect(nhanO(true)).toBe("Đã cấp");
    expect(nhanO(false)).toBe("Chưa cấp");
  });
});

describe("xã chưa cấu hình: có câu giải thích, không bao giờ là bảng trống", () => {
  it("ba ca, ba câu khác nhau — ba người sửa khác nhau", () => {
    const vaiTro = nhanChuaCauHinh("vaiTro");
    const danhMuc = nhanChuaCauHinh("danhMucQuyen");
    const caHai = nhanChuaCauHinh("caHai");

    expect(new Set([vaiTro, danhMuc, caHai]).size).toBe(3);
    for (const cau of [vaiTro, danhMuc, caHai]) expect(cau.length).toBeGreaterThan(0);
  });

  it("mỗi câu nói VIỆC KẾ TIẾP, không chỉ nói cái gì đang thiếu", () => {
    // `skills/accessibility-elderly`: một thông báo phải nói làm gì tiếp theo.
    for (const thieu of ["vaiTro", "danhMucQuyen", "caHai"] as const) {
      expect(nhanChuaCauHinh(thieu)).toMatch(/liên hệ|Liên hệ|báo cho/);
    }
  });

  it("no sentence points at the removed 'Tạo tám vai trò mẫu' button (owner decision 08/10/2026)", () => {
    for (const thieu of ["vaiTro", "danhMucQuyen", "caHai"] as const) {
      expect(nhanChuaCauHinh(thieu)).not.toContain("Tạo tám vai trò mẫu");
    }
  });

  it("không câu nào gọi đây là lỗi — xã vừa khởi tạo là hệ thống đang chạy đúng", () => {
    for (const thieu of ["vaiTro", "danhMucQuyen", "caHai"] as const) {
      expect(nhanChuaCauHinh(thieu).toLowerCase()).not.toContain("lỗi");
    }
  });
});
