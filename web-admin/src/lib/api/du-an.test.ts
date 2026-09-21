import { afterEach, describe, expect, it, vi } from "vitest";

import { duongDanDanhSachDuAn, layChiTietDuAn, layDanhSachDuAn } from "./du-an";

/**
 * Kiểm cả đường của hai tuyến dự án: đường dẫn được dựng ra sao, và phản hồi HTTP thành cái gì.
 *
 * Ca đáng lo nhất KHÔNG phải ca 200 — nó là ca thiếu quyền (403) và ca thiếu năm: cả hai đều
 * phải thành một câu của MÁY CHỦ chứ không thành một câu do web đoán.
 */

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)`
  // không tham số làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("đường dẫn danh sách dự án", () => {
  it("LUÔN mang `year` — máy chủ từ chối lời gọi thiếu năm và cố ý không mặc định", () => {
    expect(duongDanDanhSachDuAn({ nam: 2026 })).toBe("/api/v1/investment-projects?year=2026");
  });

  it("có hạng mục thì thêm `category`", () => {
    expect(duongDanDanhSachDuAn({ nam: 2026, hangMucId: "01JHANGMUC" })).toBe(
      "/api/v1/investment-projects?year=2026&category=01JHANGMUC",
    );
  });

  it("hạng mục RỖNG thì KHÔNG gửi `category=`", () => {
    // `category=` rỗng đi vào bộ lọc của máy chủ như một mã hạng mục rỗng, không như "mọi hạng
    // mục" — nên "Tất cả hạng mục" sẽ không trả về dự án nào, lặng lẽ.
    expect(duongDanDanhSachDuAn({ nam: 2026, hangMucId: "" })).toBe(
      "/api/v1/investment-projects?year=2026",
    );
  });

  it("KHÔNG có `tenant_id` ở bất kỳ đâu trong đường dẫn", () => {
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã suy từ `Host` ở rìa ngoài.
    const duong = duongDanDanhSachDuAn({ nam: 2026, hangMucId: "01JHANGMUC" });
    expect(duong).not.toMatch(/tenant/i);
    expect(duong).not.toMatch(/xa=/);
  });

  it("đường dẫn là TƯƠNG ĐỐI, không mang host nào", () => {
    // Một host trong mã là một xã trong mã: yêu cầu phải đi trên chính host của xã để cookie
    // phiên host-only theo cùng.
    expect(duongDanDanhSachDuAn({ nam: 2026 }).startsWith("/api/")).toBe(true);
  });
});

describe("gọi danh sách dự án", () => {
  it("200: trả về nguyên phản hồi, kể cả `year` và `delay_threshold`", async () => {
    batFetch(
      new Response(JSON.stringify({ items: [], year: 2026, delay_threshold: 1000 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    expect(await layDanhSachDuAn({ nam: 2026 })).toEqual({
      ok: true,
      duLieu: { items: [], year: 2026, delay_threshold: 1000 },
    });
  });

  it("403 thiếu `budget.read`: hiện ĐÚNG câu của máy chủ, không câu do web đoán", async () => {
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

    expect(await layDanhSachDuAn({ nam: 2026 })).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });

  it("CỔNG QUYỀN Ở GIAO DIỆN KHÔNG CHẶN ĐƯỢC GÌ — gọi thẳng vẫn nhận 403 của máy chủ", async () => {
    // Bài test này nói thành lời điều luật 5 cấm #1 nói: ẩn một màn là trải nghiệm người dùng,
    // không phải an ninh. Không cần sửa một dòng JavaScript nào — chỉ cần gọi tuyến bằng chính
    // cookie phiên của mình.
    batFetch(
      new Response(
        JSON.stringify({ code: "forbidden", message: "Bạn không có quyền.", trace_id: "01J" }),
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect((await layChiTietDuAn("01JDUAN")).ok).toBe(false);
  });

  it("400 thiếu năm: câu của máy chủ đi thẳng ra màn hình", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "invalid_argument",
          message: "Thiếu hoặc sai năm ngân sách. Ví dụ: ?year=2026",
          trace_id: "01JTRACE",
        }),
        { status: 400, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(await layDanhSachDuAn({ nam: 2026 })).toEqual({
      ok: false,
      thongBao: "Thiếu hoặc sai năm ngân sách. Ví dụ: ?year=2026",
    });
  });
});

describe("gọi chi tiết dự án", () => {
  it("mã hoá `id` vào đường dẫn thay vì ghép thẳng", async () => {
    const gia = batFetch(
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await layChiTietDuAn("a/b?c=1");
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/investment-projects/a%2Fb%3Fc%3D1");
  });

  it("404 dự án của xã khác và 404 dự án không tồn tại KHÔNG phân biệt được ở đây", async () => {
    // Máy chủ trả cùng một câu cho cả hai, có chủ ý: hai câu khác nhau sẽ nói cho người gọi biết
    // một bản ghi tồn tại bên trong một cơ quan họ không có việc gì để biết.
    batFetch(
      new Response(
        JSON.stringify({ code: "not_found", message: "Không tìm thấy dự án.", trace_id: "01J" }),
        { status: 404, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(await layChiTietDuAn("01JKHONGCO")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy dự án.",
    });
  });
});
