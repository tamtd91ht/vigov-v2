import { afterEach, describe, expect, it, vi } from "vitest";

import { layMaTranQuyen } from "@/lib/api/phan-quyen";

import { oDaCap, trangThaiMaTran } from "./ma-tran-quyen";

/**
 * Kiểm CẢ ĐƯỜNG: phản hồi HTTP của `GET /api/v1/role-permissions` → trạng thái màn hình dựng được.
 *
 * Cố ý KHÔNG gọi `trangThaiMaTran` với một `KetQua` dựng sẵn ở phần lớn các ca: hai ca đáng lo
 * nhất — phiên hết hạn và quyền vừa bị gỡ — đến từ một phản hồi 401/403 thật, không từ một giá
 * trị ai đó gõ trong test. Cùng lý do đã ghi ở `quyen-tab.test.ts`.
 */

type OCap = { role_id: string; permission: string };

function phanHoiMaTran(than: {
  groups?: unknown[];
  roles?: unknown[];
  grants?: OCap[];
}): Response {
  return new Response(
    JSON.stringify({ groups: than.groups ?? [], roles: than.roles ?? [], grants: than.grants ?? [] }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

function vaiTro(id: string, ten: string, soCanBo = 1, soTaiKhoan = 1, laLanhDao = false) {
  return {
    id,
    code: id,
    name: ten,
    is_leader: laLanhDao,
    staff_count: soCanBo,
    active_account_count: soTaiKhoan,
  };
}

const NHOM_QUAN_TRI = {
  name: "QUẢN TRỊ",
  permissions: [
    { code: "admin.role", label: "Phân quyền" },
    { code: "admin.user", label: "Quản lý người dùng" },
  ],
};

function batFetch(tra: Response) {
  vi.stubGlobal("fetch", vi.fn(async () => tra));
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("năm trạng thái của tab Phân quyền — bốn ca đọc dữ liệu", () => {
  it("chưa đọc xong là `dangDoc`, KHÔNG phải một ma trận rỗng", () => {
    expect(trangThaiMaTran(null)).toEqual({ pha: "dangDoc" });
  });

  it("đọc được và có dữ liệu: giữ nguyên thứ tự hàng và cột máy chủ trả", async () => {
    batFetch(
      phanHoiMaTran({
        groups: [NHOM_QUAN_TRI],
        roles: [vaiTro("vt-chu-tich", "Chủ tịch UBND"), vaiTro("vt-ke-toan", "Kế toán")],
        grants: [{ role_id: "vt-chu-tich", permission: "admin.role" }],
      }),
    );

    const tt = trangThaiMaTran(await layMaTranQuyen());
    if (tt.pha !== "coDuLieu") throw new Error(`mong đợi coDuLieu, nhận ${tt.pha}`);

    expect(tt.nhom.map((n) => n.name)).toEqual(["QUẢN TRỊ"]);
    expect(tt.vaiTro.map((v) => v.id)).toEqual(["vt-chu-tich", "vt-ke-toan"]);
  });

  it("đọc được nhưng xã chưa có vai trò nào: `chuaCauHinh`, không phải lỗi", async () => {
    batFetch(phanHoiMaTran({ groups: [NHOM_QUAN_TRI], roles: [] }));
    expect(trangThaiMaTran(await layMaTranQuyen())).toEqual({
      pha: "chuaCauHinh",
      thieu: "vaiTro",
    });
  });

  it("danh mục quyền rỗng: cũng là `chuaCauHinh`, nhưng nói rõ THIẾU TRỤC KHÁC", async () => {
    // Hai ca có hai người sửa khác nhau: không có vai trò là việc của xã, danh mục quyền rỗng là
    // chuyện của danh mục dùng chung. Gộp hai ca vào một câu là chỉ sai một trong hai người.
    batFetch(phanHoiMaTran({ groups: [], roles: [vaiTro("vt-ke-toan", "Kế toán")] }));
    expect(trangThaiMaTran(await layMaTranQuyen())).toEqual({
      pha: "chuaCauHinh",
      thieu: "danhMucQuyen",
    });
  });

  it("xã vừa onboard — cả hai trục rỗng", async () => {
    batFetch(phanHoiMaTran({}));
    expect(trangThaiMaTran(await layMaTranQuyen())).toEqual({ pha: "chuaCauHinh", thieu: "caHai" });
  });

  it("thiếu quyền (403): không đọc được, và hiện ĐÚNG câu của máy chủ", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "forbidden",
          message: "Bạn không có quyền thực hiện thao tác này.",
          trace_id: "01JTRACE",
        }),
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(trangThaiMaTran(await layMaTranQuyen())).toEqual({
      pha: "khongDocDuoc",
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });

  it("phiên hết hạn (401): không đọc được, KHÔNG dựng bảng rỗng", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "unauthenticated",
          message: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
          trace_id: "01JTRACE",
        }),
        { status: 401, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(trangThaiMaTran(await layMaTranQuyen())).toEqual({
      pha: "khongDocDuoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });
  });

  it("mạng hỏng: không đọc được — KHÔNG được biến thành 'xã chưa cấu hình'", async () => {
    // Ca này là lý do `chuaCauHinh` và `khongDocDuoc` là hai nhánh rời nhau. Gộp lại thì một lần
    // rớt mạng hiện ra câu "đơn vị chưa có vai trò nào" — báo sai một sự thật về cả một xã.
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );

    const tt = trangThaiMaTran(await layMaTranQuyen());
    expect(tt.pha).toBe("khongDocDuoc");
  });

  it("máy chủ trả 200 nhưng thân không phải JSON: vẫn là không đọc được", async () => {
    batFetch(new Response("<html>proxy</html>", { status: 200 }));
    expect(trangThaiMaTran(await layMaTranQuyen()).pha).toBe("khongDocDuoc");
  });
});

describe("tra một ô của ma trận", () => {
  async function bangDaCap(grants: OCap[]) {
    batFetch(
      phanHoiMaTran({
        groups: [NHOM_QUAN_TRI],
        roles: [vaiTro("vt-chu-tich", "Chủ tịch UBND"), vaiTro("vt-ke-toan", "Kế toán")],
        grants,
      }),
    );
    const tt = trangThaiMaTran(await layMaTranQuyen());
    if (tt.pha !== "coDuLieu") throw new Error(`mong đợi coDuLieu, nhận ${tt.pha}`);
    return tt.daCap;
  }

  it("ô có trong `grants` là đã cấp, ô không có là CHƯA CẤP — không có trạng thái thứ ba", async () => {
    const bang = await bangDaCap([{ role_id: "vt-chu-tich", permission: "admin.role" }]);
    expect(oDaCap(bang, "vt-chu-tich", "admin.role")).toBe(true);
    expect(oDaCap(bang, "vt-chu-tich", "admin.user")).toBe(false);
  });

  it("quyền của vai trò này KHÔNG tràn sang vai trò khác", async () => {
    // Ô mang theo cả hai đầu của nó chính để chuyện này không xảy ra được.
    const bang = await bangDaCap([{ role_id: "vt-chu-tich", permission: "admin.role" }]);
    expect(oDaCap(bang, "vt-ke-toan", "admin.role")).toBe(false);
  });

  it("vai trò không có ô nào: mọi ô của cột ấy là chưa cấp, không phải 'không rõ'", async () => {
    const bang = await bangDaCap([]);
    expect(oDaCap(bang, "vt-ke-toan", "admin.role")).toBe(false);
  });

  it("không khớp tiền tố, không khớp gần đúng — `admin.role` không mở `admin.user`", async () => {
    const bang = await bangDaCap([{ role_id: "vt-chu-tich", permission: "admin.role" }]);
    expect(oDaCap(bang, "vt-chu-tich", "admin")).toBe(false);
    expect(oDaCap(bang, "vt-chu-tich", "admin.role.read")).toBe(false);
    expect(oDaCap(bang, "vt-chu-tich", "ADMIN.ROLE")).toBe(false);
  });

  it("một vai trò giữ nhiều quyền: đủ cả, không chỉ quyền cuối cùng", async () => {
    const bang = await bangDaCap([
      { role_id: "vt-chu-tich", permission: "admin.role" },
      { role_id: "vt-chu-tich", permission: "admin.user" },
    ]);
    expect(oDaCap(bang, "vt-chu-tich", "admin.role")).toBe(true);
    expect(oDaCap(bang, "vt-chu-tich", "admin.user")).toBe(true);
  });
});
