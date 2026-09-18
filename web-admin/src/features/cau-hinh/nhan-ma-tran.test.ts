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

describe("đầu cột vai trò hiện HAI số đếm", () => {
  it("3 cán bộ, 1 tài khoản đang hoạt động: cả hai số, và nói rõ số sau nằm TRONG số trước", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(3, 1))).toBe(
      "3 cán bộ · 1 trong số đó có tài khoản đang hoạt động",
    );
  });

  it("(3, 0): vẫn hiện ĐỦ HAI SỐ — đây là ca một số đếm duy nhất giấu mất", () => {
    // Một cột `3 · 0` là cột mà mọi ô đã cấp không tới được ai: ba cán bộ giữ vai trò, không ai
    // trong số họ có tài khoản đang mở. Rút gọn thành "3 cán bộ" là xoá đúng điều người quản trị
    // cần thấy trước khi tin rằng quyền đã tới tay ai đó.
    const nhan = nhanSoNguoiGiuVaiTro(cot(3, 0));
    expect(nhan).toBe("3 cán bộ · 0 trong số đó có tài khoản đang hoạt động");
    expect(nhan).toContain("3");
    expect(nhan).toContain("0");
  });

  it("(3, 3): vẫn là hai số, không rút thành một", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(3, 3))).toBe(
      "3 cán bộ · 3 trong số đó có tài khoản đang hoạt động",
    );
  });

  it("chưa ai giữ vai trò: MỘT câu, vì tập con của tập rỗng không nói thêm được gì", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(0, 0))).toBe("Chưa có cán bộ nào giữ vai trò này");
  });

  it("dữ liệu lệch (tập con lớn hơn tập cha) KHÔNG bị câu rút gọn nuốt mất", () => {
    // Máy chủ không phát ra được hình dạng này, và đúng vì thế nó phải hiện nguyên hai con số nếu
    // có ngày nó xảy ra — chứ không hiện "Chưa có cán bộ nào giữ vai trò này".
    expect(nhanSoNguoiGiuVaiTro(cot(0, 2))).toBe(
      "0 cán bộ · 2 trong số đó có tài khoản đang hoạt động",
    );
  });

  it("câu chữ nói quan hệ TẬP CON, không đặt hai con số cạnh nhau rồi để người đọc tự cộng", () => {
    expect(nhanSoNguoiGiuVaiTro(cot(5, 2))).toContain("trong số đó");
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

  it("không câu nào gọi đây là lỗi — xã vừa khởi tạo là hệ thống đang chạy đúng", () => {
    for (const thieu of ["vaiTro", "danhMucQuyen", "caHai"] as const) {
      expect(nhanChuaCauHinh(thieu).toLowerCase()).not.toContain("lỗi");
    }
  });
});
