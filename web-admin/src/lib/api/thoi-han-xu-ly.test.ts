import { afterEach, describe, expect, it, vi } from "vitest";

import { gieoThoiHanMacDinh, layThoiHanXuLy, suaThoiHanXuLy } from "./thoi-han-xu-ly";

/**
 * Ba tuyến của bảng thời hạn xử lý, nhìn từ phía **dây**: đúng đường dẫn, đúng phương thức, đúng
 * thân. Những thứ tệp này canh đều không hiện ra trên màn hình — chúng hiện ra ở chỗ một con số
 * của xã bị kéo lùi, hoặc một lần bấm nút thành hai dòng trong CSDL.
 */

function ghiGia(ma: number, than: unknown) {
  const gia = vi.fn(
    async () =>
      new Response(than === null ? null : JSON.stringify(than), {
        status: ma,
        headers: than === null ? {} : { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", gia);
  return gia;
}

function loiGoi(gia: ReturnType<typeof ghiGia>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

const DONG = {
  id: "01J0000000000000000000SLA",
  work_kind: "van-ban-den",
  field: "",
  is_default: true,
  acknowledge_hours: 8,
  resolve_hours: 40,
  due_soon_hours: 24,
  escalate_leader_hours: 24,
  escalate_president_hours: 48,
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("GET /api/v1/sla", () => {
  it("đường dẫn TƯƠNG ĐỐI, không host nào — và không có `tenant_id` ở bất kỳ đâu", async () => {
    const gia = ghiGia(200, { items: [DONG], problems: [] });
    await layThoiHanXuLy();

    const { duongDan, tuyChon } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/sla");
    expect(duongDan).not.toContain("http");
    expect(duongDan).not.toContain("tenant");
    expect(tuyChon.body).toBeUndefined();
  });

  it("403 thiếu quyền trả về NGUYÊN câu máy chủ viết, không diễn giải lại", async () => {
    // `GET /api/v1/sla` đòi `admin.sla` thật — khác ba tuyến đọc lịch. Màn hình không được dựng
    // một cổng quyền thứ hai để đoán trước điều đó; nó hiện đúng câu máy chủ trả về.
    ghiGia(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." });
    const kq = await layThoiHanXuLy();

    expect(kq).toEqual({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
  });
});

describe("PATCH /api/v1/sla/{id}", () => {
  it("thân mang ĐÚNG những cột đã đổi, và KHÔNG mang cột không đổi", async () => {
    const gia = ghiGia(200, DONG);
    await suaThoiHanXuLy(DONG.id, { resolve_hours: 56 });

    const { tuyChon } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("PATCH");
    // `undefined` bị `JSON.stringify` bỏ hẳn khỏi thân — không thành `null`, vì `null` vào một
    // `*int` của Go là con trỏ rỗng và máy chủ đọc nó thành "không nhắc tới".
    expect(JSON.parse(String(tuyChon.body))).toEqual({ resolve_hours: 56 });
  });

  it("KHÔNG gửi `work_kind` hay `field` — dòng này áp cho cái gì là bất biến", async () => {
    // Một trường lĩnh vực ở đây sẽ cho phép màn hình lặng lẽ trỏ lại một cam kết đang có sang lĩnh
    // vực khác, và nó đúng là tham số cần đối chiếu bộ mã tầng 1 mà ADR 0026 điều kiện dừng #2
    // chặn. Hợp đồng không có hai trường ấy; phép dựng thân từng trường chặn lúc chạy.
    const gia = ghiGia(200, DONG);
    await suaThoiHanXuLy(DONG.id, { acknowledge_hours: 4 });

    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("work_kind");
    expect(than).not.toHaveProperty("field");
    expect(than).not.toHaveProperty("id");
  });

  it("id vào đường dẫn ĐÃ MÃ HOÁ, không ghép thẳng", async () => {
    const gia = ghiGia(200, DONG);
    await suaThoiHanXuLy("a/b?c=d", { due_soon_hours: 12 });

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/sla/a%2Fb%3Fc%3Dd");
  });

  it("400 trả về nguyên câu máy chủ, không rẽ nhánh theo `code`, không hiện `trace_id`", async () => {
    ghiGia(400, {
      code: "invalid_request",
      message: "Không có số giờ nào được gửi lên để sửa.",
      trace_id: "01J00000000000000000TRACE",
    });
    const kq = await suaThoiHanXuLy(DONG.id, {});

    expect(kq).toEqual({ ok: false, thongBao: "Không có số giờ nào được gửi lên để sửa." });
    if (kq.ok) return;
    expect(kq.thongBao).not.toContain("TRACE");
    expect(kq.thongBao).not.toContain("400");
  });
});

describe("POST /api/v1/sla/defaults", () => {
  it("KHÔNG gửi thân và KHÔNG khai `Content-Type` — hợp đồng khai `than: never`", async () => {
    const gia = ghiGia(200, { seeded: 15, kept: 0 });
    await gieoThoiHanMacDinh();

    const { tuyChon, header } = loiGoi(gia, 0);
    expect(tuyChon.method).toBe("POST");
    expect(tuyChon.body).toBeUndefined();
    expect(header.get("Content-Type")).toBeNull();
  });

  it("KHÔNG mang `Idempotency-Key` — chống trùng nằm trong chính phép ghi", async () => {
    // VẾ CHỊU LỰC. Thêm một khoá chống trùng ở đây trông như cẩn thận hơn mà thực ra YẾU hơn:
    // khoá theo biểu mẫu không chặn được hai quản trị viên cùng bấm, còn phép ghi "không ghi đè"
    // thì chặn — bấm lần thứ hai trả `seeded: 0`, đó mới là trạng thái cuối đúng.
    const gia = ghiGia(200, { seeded: 0, kept: 15 });
    await gieoThoiHanMacDinh();

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBeNull();
  });

  it("đọc được cả hai con số — `kept` là lời cam đoan không đụng vào số xã đã sửa", async () => {
    ghiGia(200, { seeded: 3, kept: 12 });
    const kq = await gieoThoiHanMacDinh();

    expect(kq).toEqual({ ok: true, duLieu: { seeded: 3, kept: 12 } });
  });
});
