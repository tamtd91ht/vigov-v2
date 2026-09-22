import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BAN_TRONG, banTuCanBo, canhBaoKhoa, daDatKhoa, thanSua, thanThem } from "./nhan-ghi-danh-ba";

/**
 * Phép đổi hình dạng giữa ĐƯỜNG ĐỌC và ĐƯỜNG GHI — chỗ dễ sai nhất của cả màn hình, vì hợp đồng
 * đặt hai tên khác nhau cho cùng một thứ:
 *
 *   đọc `phone`         ↔ ghi `office_phone`
 *   đọc `department_id` ↔ ghi `org_unit_id`
 *
 * Cặp thứ nhất là cặp nguy hiểm: `phone` và `mobile` cùng kiểu chuỗi, cùng trông như một số điện
 * thoại. Lắp nhầm thì `tsc` sạch, màn hình đẹp, và số di động CÁ NHÂN của một người được ghi vào
 * cột máy bàn CƠ QUAN — tức đổi địa vị pháp lý của dữ liệu ấy (#16), và mọi luật che / xuất Excel
 * / công khai ra Mini App về sau sẽ áp sai mức cho cả hai.
 */

const MAY_BAN = "02350000000";
const DI_DONG = "0900000000";

const CAN_BO: identity_canBoTomTat = {
  id: "01J000000000000000000001",
  code: "CB001",
  full_name: "Huỳnh Văn A",
  email: "demo@thangbinh.test",
  position: "Chuyên viên",
  department_id: "01J0000000000000000BOPHAN",
  role_id: "01J00000000000000000VAITRO",
  phone: MAY_BAN,
  mobile: DI_DONG,
  has_account: true,
  active: true,
  last_login_at: null,
  created_at: "2026-09-22T08:00:00Z",
};

describe("nạp một dòng danh bạ vào bản nháp", () => {
  it("`phone` về ô MÁY BÀN và `mobile` về ô DI ĐỘNG — không đổi chỗ", () => {
    const ban = banTuCanBo(CAN_BO);

    expect(ban.mayBanCoQuan).toBe(MAY_BAN);
    expect(ban.diDongCaNhan).toBe(DI_DONG);
  });

  it("`department_id` về ô bộ phận", () => {
    expect(banTuCanBo(CAN_BO).boPhanID).toBe(CAN_BO.department_id);
  });

  it("KHÔNG mang `role_id` và KHÔNG mang `active` vào bản nháp", () => {
    // Vai trò đi qua tuyến riêng mang hai ràng buộc của #14; trạng thái khoá đi qua tuyến riêng
    // mang phép chặn của #13. Một trường của chúng lọt vào bản nháp sửa hồ sơ là lối đi vòng qua
    // cả hai phép chặn, và là lối không ai nhìn thấy vì nó nằm trong một kiểu chứ trong một nút.
    expect(Object.keys(banTuCanBo(CAN_BO)).sort()).toEqual(Object.keys(BAN_TRONG).sort());
    expect(Object.keys(BAN_TRONG)).toHaveLength(6);
  });
});

describe("thân yêu cầu dùng ĐÚNG tên trường của hợp đồng ghi", () => {
  it("POST /staff: sáu trường, `office_phone` và `org_unit_id`, KHÔNG có `code`", () => {
    const than = thanThem(banTuCanBo(CAN_BO));

    expect(than).toEqual({
      full_name: "Huỳnh Văn A",
      position: "Chuyên viên",
      email: "demo@thangbinh.test",
      org_unit_id: CAN_BO.department_id,
      office_phone: MAY_BAN,
      mobile: DI_DONG,
    });

    // #15 — mã do hệ thống sinh. Ba cách gọi tên hay gặp, chặn cả ba.
    expect(than).not.toHaveProperty("code");
    expect(than).not.toHaveProperty("ma");
    expect(than).not.toHaveProperty("staff_code");
    // #14 / #13 — vai trò và trạng thái khoá không đi kèm lần tạo.
    expect(than).not.toHaveProperty("role_id");
    expect(than).not.toHaveProperty("active");
  });

  it("PATCH /staff/{id}: gửi ĐỦ sáu trường, kể cả ô vừa bị xoá trắng", () => {
    // Chuỗi rỗng là "ô này trống", một giá trị hợp lệ. Nếu chỗ này đổi rỗng thành `null` (nghĩa
    // "không đổi" của máy chủ) thì việc xoá một số điện thoại lặng lẽ không xảy ra, mà màn hình
    // vẫn báo đã lưu.
    const than = thanSua({ ...banTuCanBo(CAN_BO), diDongCaNhan: "" });

    expect(than.mobile).toBe("");
    expect(than.office_phone).toBe(MAY_BAN);
    expect(Object.keys(than).sort()).toEqual([
      "email",
      "full_name",
      "mobile",
      "office_phone",
      "org_unit_id",
      "position",
    ]);
  });
});

describe("câu chữ của thao tác khoá", () => {
  it("cảnh báo khoá nói CẢ HAI nửa: mất đường đăng nhập, và vẫn còn trong danh bạ", () => {
    const cau = canhBaoKhoa(true);

    expect(cau).toContain("không đăng nhập được nữa");
    expect(cau).toContain("VẪN CÒN trong danh bạ");
    // Không được nói thành xoá: đó chính là hiểu nhầm #10 tồn tại để chặn.
    expect(cau).not.toMatch(/xo[áa]/i);
  });

  it("khoá và mở khoá có hai câu xác nhận khác nhau", () => {
    expect(daDatKhoa("Huỳnh Văn A", true)).toBe("Đã khoá tài khoản của Huỳnh Văn A.");
    expect(daDatKhoa("Huỳnh Văn A", false)).toBe("Đã mở khoá tài khoản của Huỳnh Văn A.");
  });
});
