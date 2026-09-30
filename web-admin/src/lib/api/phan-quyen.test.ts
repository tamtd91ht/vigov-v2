import { afterEach, describe, expect, it, vi } from "vitest";

import { luuPhanQuyenVaiTro } from "./phan-quyen";

/**
 * `PUT /api/v1/roles/{id}/permissions` nhìn từ phía DÂY: đường dẫn, phương thức, thân, và câu từ
 * chối của máy chủ đi ra nguyên văn. Câu mẫu dưới đây chép đúng câu `service-identity/internal/http/
 * quyen.go` (`traLoiLoiPhanQuyen`) viết — màn hình không có bản câu nào của riêng nó.
 */

function ghiGia(ma: number, than: unknown) {
  const gia = vi.fn(
    async () =>
      new Response(JSON.stringify(than), {
        status: ma,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("PUT /api/v1/roles/{id}/permissions", () => {
  it("gửi PUT tới đúng vai trò, thân đúng một trường `permissions`, chờ 200", async () => {
    const gia = ghiGia(200, { role_id: "01J0VT", permissions: ["admin.user", "task.read"] });
    const kq = await luuPhanQuyenVaiTro("01J0VT", { permissions: ["admin.user", "task.read"] });

    expect(gia).toHaveBeenCalledTimes(1);
    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/roles/01J0VT/permissions");
    expect(tuyChon.method).toBe("PUT");
    expect(JSON.parse(String(tuyChon.body))).toEqual({ permissions: ["admin.user", "task.read"] });
    expect(kq).toEqual({
      ok: true,
      duLieu: { role_id: "01J0VT", permissions: ["admin.user", "task.read"] },
    });
  });

  it("`[]` đi ra đúng là `[]` — gỡ hết quyền là một yêu cầu hợp lệ, không bị bỏ trường", async () => {
    const gia = ghiGia(200, { role_id: "01J0VT", permissions: [] });
    await luuPhanQuyenVaiTro("01J0VT", { permissions: [] });
    expect(JSON.parse(String(loiGoi(gia, 0).tuyChon.body))).toEqual({ permissions: [] });
  });

  it("id vai trò được mã hoá trong đường dẫn, không ghép thô", async () => {
    const gia = ghiGia(200, { role_id: "a/b", permissions: [] });
    await luuPhanQuyenVaiTro("a/b", { permissions: [] });
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/roles/a%2Fb/permissions");
  });

  const TU_CHOI: ReadonlyArray<[number, string, string]> = [
    [400, "invalid_request", "Khoá quyền sai dạng."],
    [
      400,
      "permission_not_found",
      "Quyền không có trong danh mục của hệ thống: x.khong-co. Hãy tải lại ma trận phân quyền.",
    ],
    [
      403,
      "self_target_forbidden",
      "Không lưu được phân quyền cho vai trò mà chính bạn đang giữ. Hãy nhờ một người quản trị khác của xã thực hiện.",
    ],
    [
      403,
      "permission_escalation",
      "Bạn không thêm hay bỏ được quyền mà tài khoản của bạn không có: budget.confirm. Hãy nhờ người có đủ quyền thực hiện.",
    ],
    [404, "role_not_found", "Không tìm thấy vai trò."],
    [
      409,
      "last_holder",
      "Xã phải luôn còn ít nhất một cán bộ đang hoạt động giữ quyền admin.user. Hãy cấp quyền này cho một vai trò khác có cán bộ đang hoạt động trước, rồi lưu lại.",
    ],
  ];

  for (const [ma, code, message] of TU_CHOI) {
    it(`${ma} ${code}: câu của máy chủ đi ra NGUYÊN VĂN, không kèm mã`, async () => {
      ghiGia(ma, { code, message, trace_id: "t-1" });
      const kq = await luuPhanQuyenVaiTro("01J0VT", { permissions: ["admin.user"] });
      expect(kq).toEqual({ ok: false, thongBao: message });
    });
  }
});
