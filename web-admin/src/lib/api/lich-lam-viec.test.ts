import { afterEach, describe, expect, it, vi } from "vitest";

import {
  duongDanNgayLamBu,
  duongDanNgayNghiLe,
  layLichLamViec,
  layNgayLamBu,
  layNgayNghiLe,
} from "./lich-lam-viec";

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)`
  // không tham số làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loi(ma: number, code: string, message: string): Response {
  return new Response(JSON.stringify({ code, message, trace_id: "01JTRACE" }), {
    status: ma,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("đường dẫn ba tuyến lịch", () => {
  it("lịch tuần KHÔNG có tham số năm — lịch tuần không gắn với một năm nào", async () => {
    const gia = batFetch(
      new Response(JSON.stringify({ items: [], problems: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await layLichLamViec();
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/working-hours");
  });

  it("hai tuyến theo năm mang ĐÚNG MỘT `year`", () => {
    // Máy chủ đọc nguyên mảng `year` và từ chối khi có hai giá trị: `?year=2026&year=2027` sẽ
    // lặng lẽ đọc một trong hai năm người gọi hỏi.
    expect(duongDanNgayNghiLe(2026)).toBe("/api/v1/public-holidays?year=2026");
    expect(duongDanNgayLamBu(2026)).toBe("/api/v1/swap-working-days?year=2026");
  });

  it("không đường dẫn nào mang `tenant_id`, và cả ba đều tương đối", () => {
    for (const d of [duongDanNgayNghiLe(2026), duongDanNgayLamBu(2026), "/api/v1/working-hours"]) {
      expect(d).not.toMatch(/tenant/i);
      expect(d.startsWith("/api/")).toBe(true);
    }
  });
});

describe("phản hồi của ba tuyến lịch", () => {
  it("lịch tuần TRỐNG vẫn là 200 kèm `problems` — màn hình sửa lịch phải mở được", async () => {
    // Máy chủ cố ý không trả lỗi cho một xã chưa cấu hình giờ làm: từ chối ở đây sẽ khoá đúng
    // màn hình duy nhất sửa được lịch. Và hôm nay đó là trạng thái của mọi xã.
    batFetch(
      new Response(
        JSON.stringify({
          items: [],
          problems: [
            {
              kind: "empty_calendar",
              weekday: null,
              session_ids: [],
              message: "Xã chưa cấu hình giờ làm việc.",
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const kq = await layLichLamViec();
    expect(kq.ok).toBe(true);
    if (kq.ok) {
      expect(kq.duLieu.items).toEqual([]);
      expect(kq.duLieu.problems[0]?.kind).toBe("empty_calendar");
      // `weekday` là `null` chứ KHÔNG phải 0: 0 không phải một thứ ISO, và hiện "thứ 0" là công
      // bố một ngày không ai cấu hình.
      expect(kq.duLieu.problems[0]?.weekday).toBeNull();
    }
  });

  it("409 lịch mâu thuẫn: CẢ HAI tuyến theo năm cùng trả về một câu, và câu ấy đi nguyên ra màn hình", async () => {
    // Máy chủ từ chối ở cả hai phía có chủ ý: nếu chỉ một phía báo thì bảng kia trông vẫn lành
    // và xã sẽ không sửa gì. Câu thông báo nêu đích danh những ngày mâu thuẫn — không rút gọn.
    const cau =
      "Cấu hình lịch của xã đang mâu thuẫn: ngày 2026-09-02 vừa được khai là ngày nghỉ lễ " +
      "vừa được khai là ngày làm bù. Hệ thống không tự chọn bên nào — vui lòng sửa một trong hai.";

    batFetch(loi(409, "calendar_conflict", cau));
    expect(await layNgayNghiLe(2026)).toEqual({ ok: false, thongBao: cau });

    batFetch(loi(409, "calendar_conflict", cau));
    expect(await layNgayLamBu(2026)).toEqual({ ok: false, thongBao: cau });
  });

  it("400 năm sai: câu của máy chủ, không câu do web đoán", async () => {
    batFetch(
      loi(400, "invalid_query", "Tham số year không hợp lệ — hãy nêu một năm trong khoảng 2000–2100."),
    );
    expect(await layNgayNghiLe(99999)).toEqual({
      ok: false,
      thongBao: "Tham số year không hợp lệ — hãy nêu một năm trong khoảng 2000–2100.",
    });
  });

  it("401 phiên hết hạn trên tuyến `any-authenticated`", async () => {
    // `any-authenticated` KHÔNG nghĩa là công khai: vẫn phải có phiên, và phiên ấy thuộc đúng
    // một xã.
    batFetch(loi(401, "unauthenticated", "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại."));
    expect(await layLichLamViec()).toEqual({
      ok: false,
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });
  });
});
