import { afterEach, describe, expect, it, vi } from "vitest";

import {
  addSlaFieldRow,
  gieoThoiHanMacDinh,
  layThoiHanXuLy,
  readDocumentTypeOptions,
  readPetitionFieldOptions,
  readTaskPriorityOptions,
  removeSlaFieldRow,
  suaThoiHanXuLy,
} from "./thoi-han-xu-ly";

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
  unassigned_hold_hours: 8,
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

  it("ngưỡng giữ việc: `null` ĐI LÊN THẬT (tắt báo), `undefined` thì vắng hẳn (giữ nguyên)", async () => {
    // Máy chủ phân biệt ba trạng thái (`sla.go:286-288`). Nếu `null` bị bỏ khỏi thân như năm cột kia
    // thì "tắt báo" thành "giữ nguyên" và màn hình vẫn báo Đã lưu.
    const gia = ghiGia(200, DONG);
    await suaThoiHanXuLy(DONG.id, { unassigned_hold_hours: null });
    await suaThoiHanXuLy(DONG.id, { unassigned_hold_hours: 16 });
    await suaThoiHanXuLy(DONG.id, { resolve_hours: 56 });

    expect(String(loiGoi(gia, 0).tuyChon.body)).toBe('{"unassigned_hold_hours":null}');
    expect(JSON.parse(String(loiGoi(gia, 1).tuyChon.body))).toEqual({ unassigned_hold_hours: 16 });
    expect(JSON.parse(String(loiGoi(gia, 2).tuyChon.body))).not.toHaveProperty("unassigned_hold_hours");
  });

  it("400 Báo Chủ tịch < Báo lãnh đạo: câu máy chủ về nguyên văn", async () => {
    const cau = "Số giờ báo Chủ tịch không được nhỏ hơn số giờ báo lãnh đạo trực tiếp.";
    ghiGia(400, { code: "invalid_request", message: cau, trace_id: "" });
    const kq = await suaThoiHanXuLy(DONG.id, { escalate_president_hours: 4 });

    expect(kq).toEqual({ ok: false, thongBao: cau });
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

describe("ba danh mục lĩnh vực — nhãn cột Lĩnh vực (SLA-03) và ô chọn của hàng thêm", () => {
  const field = (code: string, label: string, active = true) => ({
    code, label, order: 1, default_label: label, default_order: 1, icon: "", tone: "", active, enabled: active, customised: false,
  });
  const entry = (code: string, label: string, active = true) => ({ id: `01J${code}`, code, label, is_default: false, active });

  it("lĩnh vực phản ánh: đường dẫn tương đối; mọi mã — kể cả mã đã tắt — giữ cờ `active`", async () => {
    const gia = ghiGia(200, { items: [field("an-ninh-trat-tu", "An ninh, trật tự"), field("cu", "Lĩnh vực cũ", false)] });
    const kq = await readPetitionFieldOptions();
    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/citizen-report-fields");
    expect(kq).toEqual({
      ok: true,
      duLieu: [
        { code: "an-ninh-trat-tu", label: "An ninh, trật tự", active: true },
        { code: "cu", label: "Lĩnh vực cũ", active: false },
      ],
    });
  });

  it("loại văn bản và mức ưu tiên: đúng tuyến của từng danh mục, giữ thứ tự máy chủ trả", async () => {
    const gia = ghiGia(200, { items: [entry("khan", "Khẩn"), entry("thuong", "Thường", false)] });
    const documents = await readDocumentTypeOptions();
    const tasks = await readTaskPriorityOptions();

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/document-types");
    expect(loiGoi(gia, 1).duongDan).toBe("/api/v1/task-priorities");
    expect(tasks).toEqual({
      ok: true,
      duLieu: [
        { code: "khan", label: "Khẩn", active: true },
        { code: "thuong", label: "Thường", active: false },
      ],
    });
    expect(documents.ok).toBe(true);
  });

  it("đọc hỏng → câu máy chủ, không ném lỗi (cột Lĩnh vực khi đó hiện mã thô)", async () => {
    ghiGia(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." });
    expect(await readPetitionFieldOptions()).toEqual({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
    vi.stubGlobal("fetch", vi.fn(async () => { throw new TypeError("network"); }));
    expect((await readDocumentTypeOptions()).ok).toBe(false);
  });
});

describe("POST /api/v1/sla — thêm thời hạn riêng cho một lĩnh vực", () => {
  const BODY = {
    work_kind: "phan-anh",
    field: "an-ninh-trat-tu",
    acknowledge_hours: 8,
    resolve_hours: 48,
    due_soon_hours: 24,
    escalate_leader_hours: 24,
    escalate_president_hours: 48,
    unassigned_hold_hours: null,
  };

  it("POST, đường dẫn tương đối, mang `Idempotency-Key` của biểu mẫu, đủ sáu con số, mong 201", async () => {
    const gia = ghiGia(201, { ...DONG, id: "01J00000000000000000NEW", ...BODY, is_default: false });
    const kq = await addSlaFieldRow(BODY, "khoa-bieu-mau-1");

    const { duongDan, tuyChon, header } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/sla");
    expect(tuyChon.method).toBe("POST");
    expect(header.get("Idempotency-Key")).toBe("khoa-bieu-mau-1");
    expect(JSON.parse(String(tuyChon.body))).toEqual(BODY);
    expect(kq.ok).toBe(true);
  });

  it("không khoá lạ nào đi kèm thân, kể cả khi bên gọi trao thừa", async () => {
    const gia = ghiGia(201, DONG);
    await addSlaFieldRow({ ...BODY, tenant_id: "x", id: "y" } as typeof BODY, "k");
    const than = JSON.parse(String(loiGoi(gia, 0).tuyChon.body)) as Record<string, unknown>;
    expect(than).not.toHaveProperty("tenant_id");
    expect(than).not.toHaveProperty("id");
  });

  it("409 sla_rule_exists / 400 sla_field_unknown / 503: câu máy chủ về NGUYÊN VĂN", async () => {
    const exists = "Lĩnh vực này đã có thời hạn riêng — hãy sửa dòng sẵn có.";
    ghiGia(409, { code: "sla_rule_exists", message: exists, trace_id: "" });
    expect(await addSlaFieldRow(BODY, "k")).toEqual({ ok: false, thongBao: exists });

    ghiGia(400, { code: "sla_field_unknown", message: "Mã lĩnh vực không có trong danh mục.", trace_id: "" });
    expect(await addSlaFieldRow(BODY, "k")).toEqual({ ok: false, thongBao: "Mã lĩnh vực không có trong danh mục." });

    ghiGia(503, { code: "sla_field_check_unavailable", message: "Chưa đối chiếu được danh mục.", trace_id: "" });
    expect(await addSlaFieldRow(BODY, "k")).toEqual({ ok: false, thongBao: "Chưa đối chiếu được danh mục." });
  });
});

describe("DELETE /api/v1/sla/{id} — xoá thời hạn riêng", () => {
  it("DELETE, id đã mã hoá, thân CHỈ có lý do, 204 không thân là thành công", async () => {
    const gia = ghiGia(204, null);
    const kq = await removeSlaFieldRow("a/b", "Gộp vào dòng mặc định");

    const { duongDan, tuyChon, header } = loiGoi(gia, 0);
    expect(duongDan).toBe("/api/v1/sla/a%2Fb");
    expect(tuyChon.method).toBe("DELETE");
    expect(JSON.parse(String(tuyChon.body))).toEqual({ reason: "Gộp vào dòng mặc định" });
    expect(header.get("Idempotency-Key")).toBeNull();
    expect(kq).toEqual({ ok: true, duLieu: null });
  });

  it("409 default_sla_rule: câu của spec 08, nguyên văn từ máy chủ", async () => {
    const cau = "Không xoá được thời hạn mặc định — mọi lĩnh vực chưa có quy định riêng đều dựa vào nó.";
    ghiGia(409, { code: "default_sla_rule", message: cau, trace_id: "" });
    expect(await removeSlaFieldRow(DONG.id, "x")).toEqual({ ok: false, thongBao: cau });
  });
});
