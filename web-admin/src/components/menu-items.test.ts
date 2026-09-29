import { describe, expect, it } from "vitest";

import { dangChon, KHOA_MO_CAU_HINH, locMenu, NHOM_MENU } from "./menu-items";

/**
 * Ca kiểm của luật LỌC MENU. Ba nhánh đáng kiểm nhất — thiếu quyền, chưa đọc xong phiên, và mục
 * chưa có màn — là ba nhánh người viết mã không bao giờ nhìn thấy: tài khoản của họ luôn đủ
 * quyền, phiên luôn về kịp, và họ biết màn nào chưa dựng.
 */

const DU_QUYEN = [
  "document.read",
  "budget.read",
  "feedback.read",
  "admin.user",
  "admin.lookup",
  "task.read",
  "announcement.create",
  "content.read",
  "report.read",
] as const;

function tenMuc(nhom: ReturnType<typeof locMenu>): string[] {
  return nhom.flatMap((n) => n.muc.map((m) => m.nhan));
}

describe("locMenu", () => {
  it("đủ quyền: thấy cả 14 mục của đặc tả §2", () => {
    expect(tenMuc(locMenu(NHOM_MENU, DU_QUYEN))).toHaveLength(14);
  });

  it("KHÔNG quyền nào: mười một mục có màn biến mất, ba mục chưa có màn Ở LẠI", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, []));

    // Mười mục có màn đều mở ra dữ liệu thật, nên chúng đi theo quyền. `Thu - Chi ngân sách` vào
    // danh sách này ngày 23/09/2026, khi màn `/giai-ngan/thu-chi` ra đời; `Nhiệm vụ`,
    // `Biên bản họp` và `Thông báo` cùng ngày, cùng lý do.
    for (const n of [
      "Văn bản & Đơn thư",
      "Giải ngân",
      "Thu - Chi ngân sách",
      "Phản ánh người dân",
      "Danh bạ cán bộ",
      "Cấu hình",
      "Nhiệm vụ",
      "Biên bản họp",
      "Thông báo",
      "Nội dung Mini App",
      "Tổng quan",
    ]) {
      expect(ten).not.toContain(n);
    }
    // Ba mục chưa có màn không mở ra dữ liệu nào cả — không có gì để rò rỉ, và lọc chúng theo
    // quyền sẽ buộc phải ĐOÁN một khoá cho một màn chưa tồn tại. `Tổng quan` rời nhóm này ngày
    // 28/09/2026, khi màn `/tong-quan` ra đời.
    expect(ten).toHaveLength(3);
    expect(ten).toContain("Sổ tay lãnh đạo");
    expect(ten).toContain("Báo cáo");
  });

  it("CHƯA ĐỌC XONG phiên (null) hành xử như KHÔNG có quyền, không như có", () => {
    // FAIL CLOSED. "Chưa rõ" mà mở menu ra là mời cán bộ bấm vào thứ chắc chắn trả 403, và tệ
    // hơn: menu rút bớt ngay sau đó, nên họ thấy một mục rồi thấy nó biến mất.
    expect(tenMuc(locMenu(NHOM_MENU, null))).toEqual(tenMuc(locMenu(NHOM_MENU, [])));
  });

  it("khoá cùng nhóm `admin.*` mà KHÔNG canh tab nào (`admin.user.delete`) không mở mục nào", () => {
    // Một phép khớp tiền tố `admin.*` sẽ mở luôn Cấu hình và Danh bạ — đúng điều luật 5 bất biến
    // 3b cấm, và `coQuyen` so chuỗi chính xác để chặn.
    //
    // SỬA CÓ CHỦ Ý 29/09/2026: ca này từng dùng `admin.audit` làm "khoá không canh tab nào". Tab Nhật
    // ký hệ thống (ADR 0054) nay canh đúng khoá ấy, nên nó mở mục Cấu hình — và CHỈ mục ấy (ca dưới).
    const ten = tenMuc(locMenu(NHOM_MENU, ["admin.user.delete", "admin.users", "admin"]));
    expect(ten).not.toContain("Cấu hình");
    expect(ten).not.toContain("Danh bạ cán bộ");
    expect(ten).toHaveLength(3);
  });

  it("chỉ có `admin.audit`: THẤY mục Cấu hình (tab Nhật ký hệ thống), không thấy Danh bạ", () => {
    const names = tenMuc(locMenu(NHOM_MENU, ["admin.audit"]));
    expect(names).toContain("Cấu hình");
    expect(names).not.toContain("Danh bạ cán bộ");
    expect(names).toHaveLength(4);
  });

  it("chỉ có `document.read`: THẤY mục Văn bản & Đơn thư", () => {
    // Hai tuyến đọc sổ (`GET /api/v1/incoming-documents` · `outgoing-documents`) đòi đúng khoá
    // này. Máy chủ trả sổ cho người ấy, nên menu không được giấu lối vào.
    const ten = tenMuc(locMenu(NHOM_MENU, ["document.read"]));
    expect(ten).toContain("Văn bản & Đơn thư");
    expect(ten).toHaveLength(4);
  });

  it("chỉ có `document.route` (KHÔNG có `document.read`): KHÔNG thấy mục Văn bản & Đơn thư", () => {
    // `document.route` là khoá GHI — chuyển xử lý. Người thiếu khoá đọc bấm vào sẽ gặp 403 ngay ở
    // lượt đọc, nên một mục menu dẫn tới đó là hứa một chức năng không dùng được.
    const ten = tenMuc(locMenu(NHOM_MENU, ["document.route", "document.create"]));
    expect(ten).not.toContain("Văn bản & Đơn thư");
    expect(ten).toHaveLength(3);
  });

  it("chỉ có `report.read`: THẤY mục Tổng quan, và không mục nào khác", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, ["report.read"]));
    expect(ten).toContain("Tổng quan");
    expect(ten).toHaveLength(4);
  });

  it("có mọi khoá mô-đun nhưng THIẾU `report.read`: KHÔNG thấy mục Tổng quan — ca bị từ chối", () => {
    // Mỗi tuyến số liệu của Tổng quan đòi `report.read` CÙNG khoá đọc của mô-đun. Thiếu khoá thứ
    // nhất thì mọi khối trên trang đều 403, nên một mục menu dẫn tới đó là hứa một trang rỗng.
    const ten = tenMuc(
      locMenu(NHOM_MENU, ["task.read", "document.read", "feedback.read", "budget.read"]),
    );
    expect(ten).not.toContain("Tổng quan");
  });

  it("nhóm rỗng thì biến mất, không để lại một nhãn nhóm trống", () => {
    const nhom = locMenu([{ ten: "RỖNG", muc: [{ nhan: "X", duong: "/x", khoa: "khong.co" }] }], []);
    expect(nhom).toHaveLength(0);
  });
});

describe("mục Cấu hình — mở khi có BẤT KỲ khoá nào canh một tab trên /cau-hinh", () => {
  it("chỉ có `admin.sla`: THẤY mục Cấu hình — lối vào tab Thời hạn xử lý", () => {
    // Ca thật đã hỏng: menu từng canh bằng riêng `admin.lookup`, nên người được giao gieo bảng
    // thời hạn — việc mà thiếu nó xã không nhận được phản ánh nào — không tìm thấy lối vào.
    const ten = tenMuc(locMenu(NHOM_MENU, ["admin.sla"]));
    expect(ten).toContain("Cấu hình");
    // Và KHÔNG mở thêm mục nào khác: ba mục chưa có màn + đúng một mục này.
    expect(ten).toHaveLength(4);
  });

  it.each(["admin.org", "admin.user", "admin.role", "admin.lookup", "admin.sla", "admin.audit"])(
    "chỉ có `%s`: thấy mục Cấu hình",
    (khoa) => {
      expect(tenMuc(locMenu(NHOM_MENU, [khoa]))).toContain("Cấu hình");
    },
  );

  it("không khoá nào trong tập ấy: KHÔNG thấy mục Cấu hình — ca bị từ chối", () => {
    const ten = tenMuc(
      locMenu(NHOM_MENU, ["document.read", "task.read", "feedback.read", "admin.user.delete"]),
    );
    expect(ten).not.toContain("Cấu hình");
  });

  it("chuỗi gần giống không mở được mục", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, ["admin.slas", "ADMIN.SLA", "admin.sla.read", "admin"]));
    expect(ten).not.toContain("Cấu hình");
  });

  it("chưa đọc xong phiên (null): KHÔNG thấy mục Cấu hình", () => {
    expect(tenMuc(locMenu(NHOM_MENU, null))).not.toContain("Cấu hình");
  });

  it("danh sách khoá rỗng thì đóng, không mở", () => {
    // `[].some(...)` là `false` — canh ca này để một lần đổi sang `every` (`[].every` là `true`)
    // không lặng lẽ mở một mục cho mọi tài khoản.
    const nhom = locMenu([{ ten: "X", muc: [{ nhan: "Y", duong: "/y", khoa: [] }] }], ["admin.sla"]);
    expect(nhom).toHaveLength(0);
  });

  it("tập khoá liệt kê từng khoá, không chứa khoá không canh tab nào", () => {
    // SỬA CÓ CHỦ Ý 29/09/2026 (ADR 0054 §6): ca này từng ghim `admin.audit` VẮNG, vì khi ấy chưa tab
    // nào canh nó. Tab Nhật ký hệ thống nay canh đúng khoá ấy, nên nó phải CÓ MẶT — vắng thì cán bộ
    // chỉ giữ `admin.audit` không tìm thấy lối vào tab của mình. `admin.user.delete` vẫn vắng: nút
    // của nó ở `/danh-ba`, không ở đây.
    expect(KHOA_MO_CAU_HINH).toContain("admin.audit");
    expect(KHOA_MO_CAU_HINH).not.toContain("admin.user.delete");
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
