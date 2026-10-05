import { describe, expect, it } from "vitest";

import { dangChon, KHOA_MO_CAU_HINH, locMenu, NHOM_MENU } from "./muc-menu";

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
  "admin.role",
  "admin.lookup",
  "task.read",
  "announcement.create",
  "content.read",
  "report.read",
  "asset.read",
] as const;

function tenMuc(nhom: ReturnType<typeof locMenu>): string[] {
  return nhom.flatMap((n) => n.muc.map((m) => m.nhan));
}

describe("locMenu", () => {
  it("đủ quyền: thấy cả 16 mục — 14 của đặc tả §2, thêm Người dùng và Phân quyền (05/10/2026)", () => {
    expect(tenMuc(locMenu(NHOM_MENU, DU_QUYEN))).toHaveLength(16);
  });

  it("KHÔNG quyền nào: mọi mục biến mất — từ 04/10/2026 không còn mục nào chưa có màn", () => {
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
      "Sổ tay lãnh đạo",
      "Bản đồ kinh tế số",
      "Báo cáo",
      "Người dùng",
      "Phân quyền",
    ]) {
      expect(ten).not.toContain(n);
    }
    // Mục chưa có màn không mở ra dữ liệu nào cả nên không bị lọc (`locMenu`). `Tổng quan` rời nhóm
    // ấy ngày 28/09/2026; `Sổ tay lãnh đạo`, `Bản đồ kinh tế số` (ADR 0071, ADR 0072) và `Báo cáo`
    // (`/bao-cao`, ADR 0053 sửa đổi 04/10/2026) ngày 04/10/2026 — nhóm nay rỗng.
    expect(ten).toHaveLength(0);
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
    expect(ten).toHaveLength(0);
  });

  it("chỉ có `admin.audit`: THẤY mục Cấu hình (tab Nhật ký hệ thống), không thấy Danh bạ", () => {
    const names = tenMuc(locMenu(NHOM_MENU, ["admin.audit"]));
    expect(names).toContain("Cấu hình");
    expect(names).not.toContain("Danh bạ cán bộ");
    expect(names).toHaveLength(1);
  });

  it("chỉ có `document.read`: THẤY mục Văn bản & Đơn thư", () => {
    // Hai tuyến đọc sổ (`GET /api/v1/incoming-documents` · `outgoing-documents`) đòi đúng khoá
    // này. Máy chủ trả sổ cho người ấy, nên menu không được giấu lối vào.
    const ten = tenMuc(locMenu(NHOM_MENU, ["document.read"]));
    expect(ten).toContain("Văn bản & Đơn thư");
    expect(ten).toHaveLength(1);
  });

  it("chỉ có `document.route` (KHÔNG có `document.read`): KHÔNG thấy mục Văn bản & Đơn thư", () => {
    // `document.route` là khoá GHI — chuyển xử lý. Người thiếu khoá đọc bấm vào sẽ gặp 403 ngay ở
    // lượt đọc, nên một mục menu dẫn tới đó là hứa một chức năng không dùng được.
    const ten = tenMuc(locMenu(NHOM_MENU, ["document.route", "document.create"]));
    expect(ten).not.toContain("Văn bản & Đơn thư");
    expect(ten).toHaveLength(0);
  });

  it("chỉ có `report.read`: THẤY Tổng quan và Báo cáo, và không mục nào khác", () => {
    const ten = tenMuc(locMenu(NHOM_MENU, ["report.read"]));
    expect(ten).toContain("Tổng quan");
    expect(ten).toContain("Báo cáo");
    expect(ten).toHaveLength(2);
  });

  it("chỉ có `report.read`: Báo cáo dẫn tới /bao-cao (ADR 0053 sửa đổi 04/10/2026)", () => {
    const muc = locMenu(NHOM_MENU, ["report.read"]).flatMap((n) => n.muc);
    const report = muc.find((m) => m.nhan === "Báo cáo");
    expect(report?.duong).toBe("/bao-cao");
    expect(report?.khoa).toBe("report.read");
  });

  it("thiếu `report.read` (kể cả có `report.export`, `task.read`): KHÔNG thấy Báo cáo — ca bị từ chối", () => {
    for (const ds of [[], ["report.export", "task.read", "budget.read", "report.reads"], null]) {
      expect(tenMuc(locMenu(NHOM_MENU, ds))).not.toContain("Báo cáo");
    }
  });

  it("có mọi khoá mô-đun nhưng THIẾU `report.read`: KHÔNG thấy mục Tổng quan — ca bị từ chối", () => {
    // Mỗi tuyến số liệu của Tổng quan đòi `report.read` CÙNG khoá đọc của mô-đun. Thiếu khoá thứ
    // nhất thì mọi khối trên trang đều 403, nên một mục menu dẫn tới đó là hứa một trang rỗng.
    const ten = tenMuc(
      locMenu(NHOM_MENU, ["task.read", "document.read", "feedback.read", "budget.read"]),
    );
    expect(ten).not.toContain("Tổng quan");
    expect(ten).not.toContain("Báo cáo");
  });

  it("chỉ có `task.read`: THẤY Sổ tay lãnh đạo, dẫn tới /nhiem-vu/so-tay (ADR 0071)", () => {
    // Mọi tuyến của ba cột đòi đúng `task.read`; nhóm "Duyệt hoàn thành" thêm `task.approve` ở
    // TRONG màn, không ở mục menu.
    const muc = locMenu(NHOM_MENU, ["task.read"]).flatMap((n) => n.muc);
    const soTay = muc.find((m) => m.nhan === "Sổ tay lãnh đạo");
    expect(soTay?.duong).toBe("/nhiem-vu/so-tay");
    expect(soTay?.khoa).toBe("task.read");
  });

  it("thiếu `task.read` (kể cả có `task.approve`): KHÔNG thấy Sổ tay lãnh đạo — ca bị từ chối", () => {
    for (const ds of [[], ["task.approve", "task.extend", "report.read"], null]) {
      expect(tenMuc(locMenu(NHOM_MENU, ds))).not.toContain("Sổ tay lãnh đạo");
    }
  });

  it("chỉ có `asset.read`: THẤY Bản đồ kinh tế số, dẫn tới /ban-do (ADR 0072)", () => {
    const muc = locMenu(NHOM_MENU, ["asset.read"]).flatMap((n) => n.muc);
    const map = muc.find((m) => m.nhan === "Bản đồ kinh tế số");
    expect(map?.duong).toBe("/ban-do");
    expect(map?.khoa).toBe("asset.read");
  });

  it("thiếu `asset.read` (kể cả có `asset.update`, `admin.lookup`): KHÔNG thấy Bản đồ kinh tế số — ca bị từ chối", () => {
    for (const ds of [[], ["asset.update", "admin.lookup", "asset.reads"], null]) {
      expect(tenMuc(locMenu(NHOM_MENU, ds))).not.toContain("Bản đồ kinh tế số");
    }
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
    // Và KHÔNG mở thêm mục nào khác: đúng một mục này.
    expect(ten).toHaveLength(1);
  });

  it.each(["admin.org", "admin.lookup", "admin.sla", "admin.audit"])(
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

  it.each(["admin.user", "admin.role"])(
    "chỉ có `%s`: KHÔNG thấy mục Cấu hình — tab của khoá ấy đã rời /cau-hinh (05/10/2026)",
    (khoa) => {
      // Ca bị từ chối: giữ khoá này trong `KHOA_MO_CAU_HINH` là dẫn cán bộ tới một màn mà không phần
      // nào khoá ấy mở ra. Lối vào của họ nay là mục riêng (ca dưới).
      expect(tenMuc(locMenu(NHOM_MENU, [khoa]))).not.toContain("Cấu hình");
      expect(KHOA_MO_CAU_HINH).not.toContain(khoa);
    },
  );

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

describe("mục Người dùng và Phân quyền — mỗi mục đúng một khoá (05/10/2026, theo bản mẫu)", () => {
  it("chỉ có `admin.user`: THẤY Người dùng, dẫn tới /nguoi-dung, và không mục nào khác", () => {
    const muc = locMenu(NHOM_MENU, ["admin.user"]).flatMap((n) => n.muc);
    expect(muc.map((m) => m.nhan)).toEqual(["Danh bạ cán bộ", "Người dùng"]);
    expect(muc.find((m) => m.nhan === "Người dùng")?.duong).toBe("/nguoi-dung");
  });

  it("chỉ có `admin.role`: THẤY Phân quyền, dẫn tới /nguoi-dung/phan-quyen, và không mục nào khác", () => {
    const muc = locMenu(NHOM_MENU, ["admin.role"]).flatMap((n) => n.muc);
    expect(muc.map((m) => m.nhan)).toEqual(["Phân quyền"]);
    expect(muc[0]?.duong).toBe("/nguoi-dung/phan-quyen");
  });

  it("CA BỊ TỪ CHỐI: thiếu hai khoá (kể cả có `admin.user.delete`, mọi khoá admin.* khác) → không thấy cả hai", () => {
    for (const ds of [
      [],
      null,
      ["admin.user.delete", "admin.users", "admin.roles", "ADMIN.ROLE", "admin.org", "admin.lookup", "admin.sla", "admin.audit"],
    ]) {
      const ten = tenMuc(locMenu(NHOM_MENU, ds));
      expect(ten).not.toContain("Người dùng");
      expect(ten).not.toContain("Phân quyền");
    }
  });

  it("hai mục nằm trong HỆ THỐNG, giữa Báo cáo và Cấu hình — thứ tự của bản mẫu", () => {
    const systemItems = NHOM_MENU.find((n) => n.ten === "HỆ THỐNG")?.muc.map((m) => m.nhan);
    expect(systemItems).toEqual(["Danh bạ cán bộ", "Báo cáo", "Người dùng", "Phân quyền", "Cấu hình"]);
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
