import { afterEach, describe, expect, it, vi } from "vitest";

import {
  datDongTong,
  doiCachTinh,
  duongDanBang,
  duongDanChiSo,
  ghiDot,
  goBang,
  goDot,
  goKhoanMuc,
  layBang,
  layChiSoNganSach,
  layDot,
  suaBang,
  suaKhoanMuc,
  taoBang,
  themKhoanMuc,
  type SuaBangVao,
} from "./thu-chi";

/**
 * Kiểm cả tám tuyến của màn Thu - Chi: đường dẫn dựng ra sao, THÂN gửi đi có đúng những trường
 * máy chủ nhận hay không, và phản hồi HTTP thành cái gì.
 *
 * Ca đáng lo nhất KHÔNG phải ca 200. Nó là ba ca dưới đây, và cả ba đều im lặng khi hỏng:
 *
 *   1. Thân mang `method` / `level` / `is_headline` → máy chủ trả 400 với MỌI lần ghi.
 *   2. `DELETE` không mang `reason` → 400 với mọi lần gỡ, trong khi hộp thoại trông như đã xong.
 *   3. `POST` thiếu `Idempotency-Key` → 400, và khi có thì một lần bấm hai lần không cộng đôi một
 *      con số vào tổng của dòng cha.
 */

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)` không tham số
  // làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function traJSON(than: unknown, ma: number): Response {
  return new Response(JSON.stringify(than), {
    status: ma,
    headers: { "Content-Type": "application/json" },
  });
}

function loi(ma: number, cau: string): Response {
  return traJSON({ code: "invalid_request", message: cau, trace_id: "01JTRACE" }, ma);
}

function than(goi: ReturnType<typeof batFetch>): Record<string, unknown> {
  const tuyChon = goi.mock.calls[0]?.[1];
  const tho = tuyChon?.body;
  return typeof tho === "string" ? (JSON.parse(tho) as Record<string, unknown>) : {};
}

function header(goi: ReturnType<typeof batFetch>): Record<string, string> {
  const h = goi.mock.calls[0]?.[1]?.headers;
  return (h ?? {}) as Record<string, string>;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("đường dẫn đọc", () => {
  it("bảng LUÔN mang cả `year` lẫn `kind` — hợp đồng khai tuỳ chọn, handler trả 400 khi thiếu", () => {
    // `finance_get_budget_sheets["truyVan"]` sinh ra hai trường `?`, nhưng `namVaLoai` ở
    // `thu_chi_ngan_sach.go:379` từ chối cả hai khi vắng. Hợp đồng thắng đặc tả, handler thắng
    // hợp đồng khi hai bên lệch.
    expect(duongDanBang({ nam: 2026, loai: "chi" })).toBe(
      "/api/v1/budget-sheets?year=2026&kind=chi",
    );
    expect(duongDanBang({ nam: 2026, loai: "thu" })).toBe(
      "/api/v1/budget-sheets?year=2026&kind=thu",
    );
  });

  it("chỉ số mang `year`, và KHÔNG mang `kind` — nó đọc cả hai bảng", () => {
    expect(duongDanChiSo(2026)).toBe("/api/v1/budget-indicators?year=2026");
  });

  it("KHÔNG có `tenant_id` ở bất kỳ đâu, và đường dẫn là TƯƠNG ĐỐI", () => {
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2); một host trong mã là một xã
    // trong mã.
    for (const duong of [duongDanBang({ nam: 2026, loai: "thu" }), duongDanChiSo(2026)]) {
      expect(duong).not.toMatch(/tenant/i);
      expect(duong).not.toMatch(/xa=/);
      expect(duong.startsWith("/api/")).toBe(true);
    }
  });
});

describe("đọc bảng", () => {
  it("200: trả nguyên phản hồi", async () => {
    const bang = { sheet: { id: "01JBANG" }, columns: [], lines: [], summary: {} };
    batFetch(traJSON(bang, 200));

    const kq = await layBang({ nam: 2026, loai: "chi" });
    expect(kq.ok).toBe(true);
  });

  it("404 'xã chưa lập bảng' ra màn hình bằng NGUYÊN câu máy chủ", async () => {
    batFetch(loi(404, "Không tìm thấy bảng ngân sách này."));

    expect(await layBang({ nam: 2026, loai: "chi" })).toEqual({
      ok: false,
      thongBao: "Không tìm thấy bảng ngân sách này.",
    });
  });

  it("403 thiếu `budget.read`: câu của máy chủ, không câu do web đoán", async () => {
    batFetch(loi(403, "Bạn không có quyền thực hiện thao tác này."));

    expect(await layBang({ nam: 2026, loai: "thu" })).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});

describe("đọc chỉ số", () => {
  it("chỉ số KHÔNG 404 khi xã chưa có bảng — nó về 200 kèm lý do, và lý do KHÔNG phải số 0", async () => {
    // `DocChiSoNganSach` trả 200 kèm `unavailable_reason` chứ không 404, đúng để thẻ KPI ở lại
    // trên màn hình. Một `null` đọc thành `0` là báo lên lãnh đạo một con số cân đối chưa ai khai.
    batFetch(
      traJSON(
        {
          year: 2026,
          revenue_achievement: { name: "Thu đạt dự toán", basis_points: null, unavailable_reason: "xã chưa có bảng ngân sách cho năm và loại này" },
          expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
          balance: { amount: null, unavailable_reason: "xã chưa có bảng ngân sách cho năm và loại này" },
          revenue_totals: [],
        },
        200,
      ),
    );

    const kq = await layChiSoNganSach(2026);
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(kq.duLieu.revenue_achievement.basis_points).toBeNull();
    expect(kq.duLieu.balance.amount).toBeNull();
  });
});

describe("lập bảng", () => {
  it("mang `Idempotency-Key` và KHÔNG mang `code`", async () => {
    const goi = batFetch(traJSON({ id: "01JBANG", code: "NS-2026-CHI-01" }, 201));

    await taoBang(
      {
        year: 2026,
        kind: "chi",
        title: "BÁO CÁO CHI NGÂN SÁCH",
        unit: "trieu-dong",
        columns: [{ name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam" }],
      },
      "khoa-cua-bai-kiem",
    );

    expect(header(goi)["Idempotency-Key"]).toBe("khoa-cua-bai-kiem");
    // `code` do hệ thống cấp: một client đặt được mã là một client ĐỐT vĩnh viễn một mã của xã,
    // vì `UNIQUE (tenant_id, ma)` đếm cả dòng đã xoá mềm.
    expect(than(goi)).not.toHaveProperty("code");
    expect(than(goi)["kind"]).toBe("chi");
    // Đơn vị là MÃ — máy chủ trả 400 cho chữ tự do, kể cả "Triệu đồng".
    expect(than(goi)["unit"]).toBe("trieu-dong");
  });

  it("409 'bảng đã tồn tại': câu máy chủ ra thẳng màn hình", async () => {
    batFetch(loi(409, "ngan_sach: xã đã có bảng ngân sách đang dùng cho năm và loại này"));

    const kq = await taoBang(
      { year: 2026, kind: "chi", title: "T", unit: "trieu-dong", columns: [] },
      "k",
    );
    expect(kq).toEqual({
      ok: false,
      thongBao: "ngan_sach: xã đã có bảng ngân sách đang dùng cho năm và loại này",
    });
  });
});

describe("gỡ bảng và gỡ khoản mục", () => {
  it("cả hai gửi `reason` TRONG THÂN, và thành công bằng 204 không thân", async () => {
    // §6 chỉ vẽ "hộp xác nhận"; máy chủ đòi `reason` (luật 7 bất biến 1 kể tên `delete_reason`).
    // Lý do đi trong thân chứ không trong URL: chữ tự do về ngân sách một cơ quan nhà nước mà nằm
    // trong chuỗi truy vấn là chữ nằm lại trong mọi nhật ký truy cập.
    const goi = batFetch(new Response(null, { status: 204 }));

    expect(await goBang("01JBANG", "nạp nhầm biểu của năm trước")).toEqual({
      ok: true,
      duLieu: null,
    });
    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-sheets/01JBANG");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("DELETE");
    expect(than(goi)["reason"]).toBe("nạp nhầm biểu của năm trước");
  });

  it("gỡ khoản mục mã hoá `id` vào đường dẫn thay vì ghép thẳng", async () => {
    const goi = batFetch(new Response(null, { status: 204 }));

    await goKhoanMuc("a/b", "nhập trùng");
    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-lines/a%2Fb");
  });

  it("409 'khoản mục còn dòng con' là câu của máy chủ, không phải một lần gỡ cả nhánh", async () => {
    batFetch(loi(409, "ngan_sach: khoản mục còn khoản mục con thì không gỡ thẳng"));

    expect(await goKhoanMuc("01JDONG", "x")).toEqual({
      ok: false,
      thongBao: "ngan_sach: khoản mục còn khoản mục con thì không gỡ thẳng",
    });
  });
});

describe("thêm và sửa khoản mục", () => {
  it("thân thêm KHÔNG mang `method`, `level`, `is_headline` — ba trường ấy bị máy chủ từ chối", async () => {
    // Chúng có mặt trong DTO của máy chủ "present only so it can be refused": gửi lên là 400 kèm
    // `ErrCachTinhDoTuClient`. Cách tính do máy suy từ CÂY (`CachTinhTheoCay`), không do client chọn.
    const goi = batFetch(traJSON({ id: "01JDONG" }, 201));

    await themKhoanMuc(
      { sheet_id: "01JBANG", parent_id: "01JCHA", no: "1.1", name: "Chi quốc phòng", order: 3 },
      "khoa-cua-bai-kiem",
    );

    const t = than(goi);
    expect(t).not.toHaveProperty("method");
    expect(t).not.toHaveProperty("level");
    expect(t).not.toHaveProperty("is_headline");
    expect(header(goi)["Idempotency-Key"]).toBe("khoa-cua-bai-kiem");
  });

  it("thân sửa KHÔNG mang `sheet_id` hay `parent_id` — dời một dòng là dời con số giữa hai tổng", async () => {
    const goi = batFetch(traJSON({ id: "01JDONG" }, 200));

    await suaKhoanMuc("01JDONG", { name: "Chi an ninh" });

    const t = than(goi);
    expect(t).not.toHaveProperty("sheet_id");
    expect(t).not.toHaveProperty("parent_id");
    expect(t).not.toHaveProperty("method");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("PATCH");
  });

  it("`null` trong `values` GIỮ NGUYÊN là null trên dây — nó XOÁ TRẮNG ô, không phải số 0", async () => {
    // §9 quy tắc 4: ô trống là một trạng thái, vẽ `—`. Nếu `null` bị rơi mất trên đường đi thì lệnh
    // xoá trắng một ô trở thành "không nhắc tới", và con số cũ ở lại — im lặng.
    const goi = batFetch(traJSON({ id: "01JDONG" }, 200));

    await suaKhoanMuc("01JDONG", { values: { "01JCOT1": null, "01JCOT2": 977310 } });

    expect(than(goi)["values"]).toEqual({ "01JCOT1": null, "01JCOT2": 977310 });
  });

  it("409 'gõ số vào dòng có con' ra màn hình NGUYÊN câu quy tắc của khách", async () => {
    batFetch(loi(409, "ngan_sach: khoản mục có dòng con thì con số là tổng các con"));

    expect(await suaKhoanMuc("01JCHA", { values: { "01JCOT1": 1 } })).toEqual({
      ok: false,
      thongBao: "ngan_sach: khoản mục có dòng con thì con số là tổng các con",
    });
  });
});

describe("đánh dấu dòng tổng", () => {
  it("POST KHÔNG THÂN và KHÔNG `Content-Type`", async () => {
    // Hành vi này không mang thông tin nào ngoài "dòng nào" và "ai", mà cả hai đã nằm trong đường
    // dẫn và trong phiên. Gửi `{}` kèm `Content-Type` là tuyên bố có một thân — thứ mời người sau
    // điền vào.
    const goi = batFetch(traJSON({ id: "01JDONG", is_headline: true }, 200));

    await datDongTong("01JDONG");

    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-lines/01JDONG/headline");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(goi.mock.calls[0]?.[1]?.body).toBeUndefined();
    expect(header(goi)["Content-Type"]).toBeUndefined();
  });

  it("403 thiếu `budget.confirm`: ngôi sao KHÔNG phải quyền nhập liệu", async () => {
    // Đánh sao đổi DÒNG mà mọi ô tóm tắt và cả hai chỉ số đọc từ đó — những con số đi vào văn bản
    // gửi lên cấp trên. Nó đứng sau `budget.confirm`, không sau `budget.update` (`routes.go:943`).
    batFetch(loi(403, "Bạn không có quyền thực hiện thao tác này."));

    expect(await datDongTong("01JDONG")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});

describe("sửa bảng", () => {
  it("PATCH chỉ mang ba trường được phép — `year`/`kind`/`code`/`columns` bị máy chủ trả 400", async () => {
    const goi = batFetch(traJSON({ id: "01JBANG" }, 200));

    // Một đối tượng rộng hơn kiểu tham số (như nguyên một `bangRa`) vẫn KHÔNG được mang trường
    // thừa lên dây — hàm dựng từng trường.
    const rong = {
      title: "TIÊU ĐỀ MỚI",
      cumulative_to: "",
      unit: "nghin-dong",
      year: 2027,
      kind: "thu",
      code: "NS-X",
      columns: [],
    };
    await suaBang("01JBANG", rong as SuaBangVao);

    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-sheets/01JBANG");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(than(goi)).toEqual({ title: "TIÊU ĐỀ MỚI", cumulative_to: "", unit: "nghin-dong" });
  });

  it("trường vắng thì KHÔNG có mặt — vắng là 'để nguyên', `\"\"` là 'bỏ mốc'", async () => {
    const goi = batFetch(traJSON({ id: "01JBANG" }, 200));

    await suaBang("01JBANG", { unit: "dong" });

    expect(than(goi)).toEqual({ unit: "dong" });
  });
});

describe("đổi cách tính", () => {
  it("PATCH mang ĐÚNG MỘT trường `method`, không kèm `values`", async () => {
    const goi = batFetch(traJSON({ id: "01JDONG", method: "entries" }, 200));

    await doiCachTinh("01JDONG", "entries");

    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-lines/01JDONG");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(than(goi)).toEqual({ method: "entries" });
  });

  it("409 'dòng có con' là câu của máy chủ", async () => {
    batFetch(loi(409, "ngan_sach: khoản mục có dòng con thì cách tính là cộng con"));

    expect(await doiCachTinh("01JCHA", "manual")).toEqual({
      ok: false,
      thongBao: "ngan_sach: khoản mục có dòng con thì cách tính là cộng con",
    });
  });

  it("409 'tổng các đợt quá lớn để chép' (entries → manual) ra NGUYÊN câu máy chủ", async () => {
    // `domain.ErrTongDotVuotMuc` (56d3224): tổng không thể thành một số gõ tay. Câu nói việc phải
    // làm — gỡ đợt ghi nhầm — và một câu do web viết lại sẽ trôi khỏi quy tắc thật.
    const cau =
      "ngan_sach: tổng các đợt của khoản mục vượt mức một con số ngân sách có thể có — gỡ đợt ghi nhầm để tính lại";
    batFetch(traJSON({ code: "budget_tree", message: cau, trace_id: "01JTRACE" }, 409));

    expect(await doiCachTinh("01JLA", "manual")).toEqual({ ok: false, thongBao: cau });
  });
});

describe("các đợt thu, chi", () => {
  it("đọc danh sách: GET đúng đường dẫn, id mã hoá", async () => {
    const goi = batFetch(traJSON({ line_id: "a/b", method: "entries", entries: [] }, 200));

    const kq = await layDot("a/b");

    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-lines/a%2Fb/entries");
    expect(kq.ok).toBe(true);
  });

  it("ghi đợt: POST mang `Idempotency-Key`, và thân đúng hình dạng `ghiDotVao`", async () => {
    const goi = batFetch(traJSON({ id: "01JDOT" }, 201));

    await ghiDot(
      "01JDONG",
      {
        date: "2026-09-25",
        content: "Thu tiền sử dụng đất đợt 2",
        document_no: "PT-12",
        values: { C1: null, C2: 3463459200000 },
      },
      "khoa-lan-gui-1",
    );

    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-lines/01JDONG/entries");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(header(goi)["Idempotency-Key"]).toBe("khoa-lan-gui-1");
    expect(than(goi)).toEqual({
      date: "2026-09-25",
      content: "Thu tiền sử dụng đất đợt 2",
      document_no: "PT-12",
      values: { C1: null, C2: 3463459200000 },
    });
    // Trường tuỳ chọn vắng thì không có mặt.
    expect(than(goi)).not.toHaveProperty("counterparty");
  });

  it("gửi lại CÙNG một lần gửi sau lỗi mang CÙNG khoá", async () => {
    const goi = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) =>
      loi(503, "Hệ thống tạm thời không nhận được, vui lòng thử lại."),
    );
    vi.stubGlobal("fetch", goi);
    const thanDot = { date: "2026-09-25", content: "Đợt 1", values: { C1: 5 } };

    await ghiDot("01JDONG", thanDot, "khoa-giu");
    await ghiDot("01JDONG", thanDot, "khoa-giu");

    const khoa = goi.mock.calls.map((c) => (c[1]?.headers as Record<string, string>)["Idempotency-Key"]);
    expect(khoa).toEqual(["khoa-giu", "khoa-giu"]);
  });

  it("`counterparty` đi trong THÂN, không bao giờ trong URL", async () => {
    const goi = batFetch(traJSON({ id: "01JDOT" }, 201));

    await ghiDot(
      "01JDONG",
      { date: "2026-09-25", content: "x", counterparty: "Nguyễn Văn A", values: { C1: 1 } },
      "k",
    );

    expect(goi.mock.calls[0]?.[0]).not.toContain("Nguy");
    expect(than(goi)["counterparty"]).toBe("Nguyễn Văn A");
  });

  it("409 'khoản mục đã đủ 2000 đợt' ra NGUYÊN câu máy chủ, không kèm mã hay trace_id", async () => {
    const cau =
      "ngan_sach: khoản mục đã đủ số đợt thu chi tối đa — không ghi thêm được; gỡ bớt đợt ghi nhầm trước";
    batFetch(traJSON({ code: "budget_tree", message: cau, trace_id: "01JTRACE" }, 409));

    const kq = await ghiDot("01JDONG", { date: "2026-09-25", content: "x", values: { C1: 1 } }, "k");
    expect(kq).toEqual({ ok: false, thongBao: cau });
  });

  it("gỡ đợt: DELETE với `reason` trong thân, 204 không thân", async () => {
    const goi = batFetch(new Response(null, { status: 204 }));

    expect(await goDot("01JDOT", "ghi trùng")).toEqual({ ok: true, duLieu: null });
    expect(goi.mock.calls[0]?.[0]).toBe("/api/v1/budget-entries/01JDOT");
    expect(goi.mock.calls[0]?.[1]?.method).toBe("DELETE");
    expect(than(goi)).toEqual({ reason: "ghi trùng" });
  });

  it("gỡ đợt thiếu `budget.confirm`: 403 là câu của máy chủ", async () => {
    batFetch(loi(403, "Bạn không có quyền thực hiện thao tác này."));

    expect(await goDot("01JDOT", "x")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});
