import { describe, expect, it } from "vitest";

import { dangChon, locMenu, NHOM_MENU } from "./muc-menu";

/**
 * Ca kiểm của luật LỌC MENU. Ba nhánh đáng kiểm nhất — thiếu quyền, chưa đọc xong phiên, và mục
 * chưa có màn — là ba nhánh người viết mã không bao giờ nhìn thấy: tài khoản của họ luôn đủ
 * quyền, phiên luôn về kịp, và họ biết màn nào chưa dựng.
 */

const DU_QUYEN = [
  "document.route",
  "budget.read",
  "feedback.read",
  "admin.user",
  "admin.lookup",
] as const;

function tenMuc(nhom: ReturnType<typeof locMenu>): string[] {
  return nhom.flatMap((n) => n.muc.map((m) => m.nhan));
}

describe("locMenu", () => {
  it("đủ quyền: thấy cả 14 mục của đặc tả §2", () => {
    expect(tenMuc(locMenu(NHOM_MENU, DU_QUYEN))).toHaveLength(14);
  });

  it("KHÔNG quyền nào: năm mục có màn biến mất, chín mục chưa có màn Ở LẠI", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, []));

    // Năm mục có màn đều mở ra dữ liệu thật, nên chúng đi theo quyền.
    for (const n of ["Văn bản & Đơn thư", "Giải ngân", "Phản ánh người dân", "Danh bạ cán bộ", "Cấu hình"]) {
      expect(ten).not.toContain(n);
    }
    // Chín mục chưa có màn không mở ra dữ liệu nào cả — không có gì để rò rỉ, và lọc chúng theo
    // quyền sẽ buộc phải ĐOÁN một khoá cho một màn chưa tồn tại.
    expect(ten).toHaveLength(9);
    expect(ten).toContain("Nhiệm vụ");
    expect(ten).toContain("Báo cáo");
  });

  it("CHƯA ĐỌC XONG phiên (null) hành xử như KHÔNG có quyền, không như có", () => {
    // FAIL CLOSED. "Chưa rõ" mà mở menu ra là mời cán bộ bấm vào thứ chắc chắn trả 403, và tệ
    // hơn: menu rút bớt ngay sau đó, nên họ thấy một mục rồi thấy nó biến mất.
    expect(tenMuc(locMenu(NHOM_MENU, null))).toEqual(tenMuc(locMenu(NHOM_MENU, [])));
  });

  it("một quyền mở ĐÚNG một mục, không mở mục hàng xóm cùng nhóm khoá", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, ["admin.user"]));
    expect(ten).toContain("Danh bạ cán bộ");
    // `admin.lookup` là khoá của Cấu hình. Một phép khớp tiền tố `admin.*` sẽ mở luôn màn ấy —
    // đúng điều luật 5 bất biến 3b cấm, và `coQuyen` so chuỗi chính xác để chặn.
    expect(ten).not.toContain("Cấu hình");
  });

  it("nhóm rỗng thì biến mất, không để lại một nhãn nhóm trống", () => {
    const nhom = locMenu([{ ten: "RỖNG", muc: [{ nhan: "X", duong: "/x", khoa: "khong.co" }] }], []);
    expect(nhom).toHaveLength(0);
  });
});

describe("dangChon", () => {
  it("khớp chính xác", () => {
    expect(dangChon("/giai-ngan", "/giai-ngan")).toBe(true);
  });

  it("khớp theo ĐOẠN: /giai-ngan sáng khi đang ở /giai-ngan/thu-chi", () => {
    expect(dangChon("/giai-ngan", "/giai-ngan/thu-chi")).toBe(true);
  });

  it("KHÔNG khớp tiền tố chuỗi trần: /van-ban không sáng ở /van-ban-khac", () => {
    // Đây là ca một phép `startsWith` viết vội sẽ trượt, và nó trượt về phía sai: hai mục cùng
    // sáng thì người dùng không biết mình đang ở đâu.
    expect(dangChon("/van-ban", "/van-ban-khac")).toBe(false);
  });

  it("mục chưa có màn không bao giờ được coi là đang chọn", () => {
    expect(dangChon(null, "/")).toBe(false);
  });
});
