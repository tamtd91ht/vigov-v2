import { describe, expect, it } from "vitest";

import { khoiNguoiDung } from "./khoi-nguoi-dung";
import type { PhienDaDoc } from "./phien-hien-tai";

/**
 * BA TRẠNG THÁI, KHÔNG HAI — và ca đắt nhất là ca ở giữa.
 *
 * "Chưa đọc xong" và "đọc xong mà hỏng" trông giống nhau trên màn hình (đều không có tên), nên
 * rất dễ gộp làm một. Gộp rồi thì khi phiên hết hạn, đầu trang sẽ nằm mãi ở trạng thái "đang
 * tải" — và người dùng chờ một thứ không bao giờ tới thay vì được nói rằng phiên đã hết.
 */
const dangDoc: PhienDaDoc = null;

const doc: PhienDaDoc = {
  ok: true,
  duLieu: {
    sid: "01J000000000000000000000SD",
    expires_at: "2026-09-18T03:00:00Z",
    staff: { code: "CB-001", full_name: "Huỳnh Văn 1", position: "Chủ tịch UBND xã" },
    permissions: ["admin.user"],
  },
};

const hong: PhienDaDoc = { ok: false, thongBao: "Phiên làm việc đã hết hạn." };

describe("khoiNguoiDung", () => {
  it("chưa đọc xong thì chưa hiện gì", () => {
    const kq = khoiNguoiDung(dangDoc);
    expect(kq).toEqual({ hien: false, vi: "dang-doc" });
  });

  it("đọc được thì hiện họ tên và chức vụ", () => {
    const kq = khoiNguoiDung(doc);
    expect(kq).toEqual({ hien: true, hoTen: "Huỳnh Văn 1", chucVu: "Chủ tịch UBND xã" });
  });

  // BÀI QUAN TRỌNG NHẤT. Đầu trang của một cơ quan nhà nước hiện SAI tên người đang đăng nhập
  // là một sự cố có người phải trả lời — nặng hơn hẳn một khoảng trống. Nên khi không đọc được
  // phiên, không có tên dự phòng nào, không có "Người dùng", không có chuỗi rỗng.
  it("không đọc được thì KHÔNG hiện tên nào", () => {
    const kq = khoiNguoiDung(hong);
    expect(kq.hien).toBe(false);
    if (kq.hien) throw new Error("không thể tới đây");
    expect(kq.vi).toBe("khong-doc-duoc");
    expect(JSON.stringify(kq)).not.toContain("Huỳnh");
  });

  it("phân biệt được chưa-đọc-xong với đọc-xong-mà-hỏng", () => {
    const a = khoiNguoiDung(dangDoc);
    const b = khoiNguoiDung(hong);
    expect(a).not.toEqual(b);
  });

  // `canBoGon` cố ý không có thư điện tử — địa chỉ thư công vụ là dữ liệu cá nhân (luật 3), và
  // nó không cần thiết để một người nhận ra chính mình trên đầu trang mình vừa đăng nhập.
  it("không mang thư điện tử ra đầu trang", () => {
    expect(JSON.stringify(khoiNguoiDung(doc))).not.toContain("@");
  });
});
