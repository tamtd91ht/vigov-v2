import { afterEach, describe, expect, it, vi } from "vitest";

import { QUYEN_DUYET_GIA_HAN } from "@/lib/permissions";

import { duongDanDanhBaChonNguoi, layDanhBaChonNguoi } from "./staff-directory";

function batFetch(tra: Response) {
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("danh bạ chọn người — GET /api/v1/staff-directory", () => {
  it("không truyền bộ phận: đúng đường dẫn, KHÔNG có `unit`, không dấu hỏi thừa", async () => {
    const gia = batFetch(
      new Response(JSON.stringify({ items: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await layDanhBaChonNguoi();
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/staff-directory");
    expect(gia.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("có bộ phận: gửi `unit` và chỉ `unit`", () => {
    const duong = duongDanDanhBaChonNguoi("01JBOPHAN");
    expect(duong).toBe("/api/v1/staff-directory?unit=01JBOPHAN");
  });

  it("bộ phận rỗng hoặc toàn khoảng trắng: tham số vắng mặt hẳn, không gửi `unit=` rỗng", () => {
    expect(duongDanDanhBaChonNguoi("")).toBe("/api/v1/staff-directory");
    expect(duongDanDanhBaChonNguoi("   ")).toBe("/api/v1/staff-directory");
  });

  it("lọc người cầm quyền duyệt gia hạn: gửi `permission=task.extend`, chỉ khi được yêu cầu", async () => {
    const gia = batFetch(
      new Response(JSON.stringify({ items: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN);
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/staff-directory?permission=task.extend");
    // Không truyền quyền thì tham số VẮNG MẶT HẲN — máy chủ từ chối `permission=` rỗng.
    expect(duongDanDanhBaChonNguoi()).toBe("/api/v1/staff-directory");
    expect(duongDanDanhBaChonNguoi("01JBOPHAN")).not.toContain("permission");
  });

  it("không một chỗ nào mang `tenant_id` — xã suy từ `Host`", () => {
    expect(duongDanDanhBaChonNguoi("01JBOPHAN")).not.toMatch(/tenant/i);
  });

  it("200: trả nguyên `items` máy chủ gửi, mã cán bộ là mã nghiệp vụ", async () => {
    const than = {
      items: [
        { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "01JBOPHAN" },
      ],
    };
    batFetch(
      new Response(JSON.stringify(than), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    expect(await layDanhBaChonNguoi()).toEqual({ ok: true, duLieu: than });
  });

  it("lỗi: câu của máy chủ đi thẳng ra, nguyên văn", async () => {
    batFetch(
      new Response(
        JSON.stringify({ code: "internal", message: "Đã xảy ra lỗi. Vui lòng thử lại.", trace_id: "01JT" }),
        { status: 500, headers: { "Content-Type": "application/json" } },
      ),
    );
    expect(await layDanhBaChonNguoi()).toEqual({
      ok: false,
      thongBao: "Đã xảy ra lỗi. Vui lòng thử lại.",
    });
  });
});
