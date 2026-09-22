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
    role: { code: "chu-tich-ubnd", name: "Chủ tịch UBND", is_leader: true },
    permissions: ["admin.user"],
    // Câu mở #9, khách chốt 22/09/2026: `false` là "không bị bắt đổi", trạng thái của một người
    // đã dùng tài khoản bình thường — đúng tiền đề của mọi ca trong tệp này. Trường là BẮT BUỘC
    // trong hợp đồng, nên `tsc` đỏ khi nó vắng thay vì để một `undefined` lặng lẽ đọc thành
    // "không bắt đổi" ở màn hình thật.
    must_change_password: false,
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

  // `role` có thể là `null` — một người trong danh bạ chưa được gán vai trò. Khối người dùng
  // trên đầu trang KHÔNG đọc vai trò (nó in chức vụ, một trường khác), nên ca này phải vẫn
  // hiện tên bình thường. Bài này ghim điều đó: ngày ai đó thêm vai trò vào đầu trang mà quên
  // ca `null`, người chưa được gán vai trò sẽ mất luôn cả tên mình trên đầu trang.
  it("chưa gán vai trò thì vẫn hiện họ tên và chức vụ", () => {
    const chuaGan: PhienDaDoc = {
      ok: true,
      duLieu: { ...doc.ok ? doc.duLieu : (() => { throw new Error("fixture sai") })(), role: null },
    };
    expect(khoiNguoiDung(chuaGan)).toEqual({
      hien: true,
      hoTen: "Huỳnh Văn 1",
      chucVu: "Chủ tịch UBND xã",
    });
  });

  // `canBoGon` cố ý không có thư điện tử — địa chỉ thư công vụ là dữ liệu cá nhân (luật 3), và
  // nó không cần thiết để một người nhận ra chính mình trên đầu trang mình vừa đăng nhập.
  it("không mang thư điện tử ra đầu trang", () => {
    expect(JSON.stringify(khoiNguoiDung(doc))).not.toContain("@");
  });
});
