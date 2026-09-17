import { afterEach, describe, expect, it, vi } from "vitest";

import { layDanhSachCanBo } from "@/lib/api/can-bo";
import { layPhienHienTai } from "@/lib/api/phien";

import { quyetDinhTabNguoiDung } from "./quyen-tab";

/**
 * Kiểm cả đường: phản hồi HTTP của `GET /api/v1/sessions/current` → quyết định ẩn/hiện.
 *
 * Cố ý KHÔNG gọi `quyetDinhTabNguoiDung` với một đối tượng `KetQua` dựng sẵn: ca đáng lo nhất
 * là phiên hết hạn, và nó đến từ một phản hồi 401 chứ không từ một giá trị ai đó gõ trong test.
 */

function phanHoiPhien(quyen: readonly string[]) {
  return new Response(
    JSON.stringify({
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: quyen,
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

function batFetch(tra: Response) {
  vi.stubGlobal("fetch", vi.fn(async () => tra));
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ẩn/hiện tab Người dùng theo quyền", () => {
  it("CÓ `admin.user` thì hiện", async () => {
    batFetch(phanHoiPhien(["admin.user", "task.read"]));
    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({ hien: true });
  });

  it("KHÔNG có `admin.user` thì ẩn — ca bị từ chối, không chỉ ca được phép", async () => {
    batFetch(phanHoiPhien(["task.read", "document.read", "feedback.read"]));
    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("không quyền nào (vai trò vừa bị gỡ hết) thì ẩn", async () => {
    batFetch(phanHoiPhien([]));
    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("quyền khác cùng nhóm KHÔNG mở được tab — `admin.audit` không phải `admin.user`", async () => {
    // Nếu có ngày ai đó viết một phép khớp tiền tố `admin.*` thì test này đỏ. Bộ quyền của khách
    // không phải tích Descartes: 43 khoá được liệt kê từng cái một chính vì chúng khác nhau.
    batFetch(phanHoiPhien(["admin.audit", "admin.org", "admin.role", "admin.lookup"]));
    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("chuỗi gần giống cũng không mở được tab", async () => {
    batFetch(phanHoiPhien(["admin.users", "admin.user.read", "ADMIN.USER"]));
    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({
      hien: false,
      vi: "khong-du-quyen",
    });
  });

  it("phiên hết hạn (401): ẩn, và hiện đúng câu của máy chủ", async () => {
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

    expect(quyetDinhTabNguoiDung(await layPhienHienTai())).toEqual({
      hien: false,
      vi: "khong-doc-duoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });
  });

  it("mạng hỏng: ẩn — 'chưa rõ có quyền hay không' hành xử như 'không có'", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    const quyetDinh = quyetDinhTabNguoiDung(await layPhienHienTai());
    expect(quyetDinh.hien).toBe(false);
  });
});

describe("lớp client chỉ là tiện dụng — máy chủ vẫn chặn", () => {
  it("ẩn tab KHÔNG chặn được lời gọi: gọi thẳng tuyến vẫn nhận 403 của máy chủ", async () => {
    // Đây là bài test viết ra để nói thành lời điều luật 5 cấm #1 nói: ẩn một tab là trải
    // nghiệm người dùng, không phải an ninh. Không cần sửa một dòng JavaScript nào — chỉ cần
    // gọi tuyến bằng chính cookie phiên của mình.
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

    const ketQua = await layDanhSachCanBo();
    expect(ketQua).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});
