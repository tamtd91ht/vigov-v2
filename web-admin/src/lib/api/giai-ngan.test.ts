import { afterEach, describe, expect, it, vi } from "vitest";

import {
  goChungTu,
  khoaChungTu,
  moKhoaChungTu,
  suaChungTu,
  suaDuAn,
  themChungTu,
  themDuAn,
  xacNhanChungTu,
  xoaDuAn,
  type SuaChungTuVao,
  type SuaDuAnVao,
  type ThemChungTuVao,
  type ThemDuAnVao,
} from "./giai-ngan";

/**
 * Chín tuyến GHI của phân hệ Giải ngân.
 *
 * BA NHÓM QUAN TRỌNG NHẤT Ở TỆP NÀY, và cả ba canh thứ không bài kiểm chức năng nào thấy:
 *
 *   1. `Idempotency-Key` ĐÚNG HAI TUYẾN. Hợp đồng đòi nó ở `POST /disbursements` và
 *      `POST /investment-projects`, và KHÔNG đòi ở bảy tuyến còn lại. Quên nó ở tuyến chứng từ là
 *      một khoản tiền được đếm hai lần trong tổng đã giải ngân — và `fetch` giả trong lúc phát
 *      triển nhận mọi yêu cầu, nên không màn hình nào đỏ.
 *   2. THÂN KHÔNG MANG TRƯỜNG MÁY CHỦ TỰ QUYẾT — `status` ở chứng từ, `code`/`year` ở lần sửa dự
 *      án. Một lần ai đó "cho đủ trường" bằng cách trải một hàng vừa đọc về sẽ biên dịch được nếu
 *      kiểu bị nới, và đây là chỗ thứ hai canh nó.
 *   3. MÃ HTTP MONG ĐỢI ĐÚNG TỪNG TUYẾN: 201 · 200 · 204. Đọc 204 bằng `.json()` biến một lần gỡ
 *      thành công thành "không đọc được", và mong 200 ở một tuyến trả 201 thì mọi lần thêm thành
 *      công đều báo lỗi.
 */

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)` không tham số
  // làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function traJSON(ma: number, than: unknown): Response {
  return new Response(JSON.stringify(than), {
    status: ma,
    headers: { "Content-Type": "application/json" },
  });
}

function traTrong(ma: number): Response {
  return new Response(null, { status: ma });
}

function loiGoi(gia: ReturnType<typeof batFetch>, n: number) {
  const [duongDan, tuyChon] = gia.mock.calls[n] as unknown as [string, RequestInit];
  return { duongDan, tuyChon, header: new Headers(tuyChon.headers) };
}

function thanDaGui(gia: ReturnType<typeof batFetch>, n: number): Record<string, unknown> {
  return JSON.parse(String(loiGoi(gia, n).tuyChon.body)) as Record<string, unknown>;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

/** Một chứng từ như máy chủ trả về. Số tiền là SỐ ĐỒNG, không phải chuỗi đã định dạng. */
const CHUNG_TU = {
  id: "01JCT1",
  project_id: "01JDA1",
  payment_date: "2026-09-07",
  amount: 30000000,
  description: "Thanh toán đợt 3",
  status: "ke-toan-nhap",
  entered_by: "CB-00123",
  unlock_count: 0,
};

const THEM_CHUNG_TU_DAY_DU: ThemChungTuVao = {
  project_id: "01JDA1",
  payment_date: "2026-09-07",
  amount: 30000000,
  description: "Thanh toán đợt 3",
  counterparty: "Công ty ABC",
  voucher_no: "CT-2026-0912",
  funding_source_id: "01JNV1",
};

const SUA_CHUNG_TU_DAY_DU: SuaChungTuVao = {
  payment_date: "2026-09-08",
  amount: 12000000,
  description: "Thanh toán đợt 2",
  counterparty: "",
  voucher_no: "",
  funding_source_id: "",
};

const DU_AN_GHI_RA = {
  id: "01JDA1",
  code: "DA01",
  year: 2026,
  category_id: "01JHM1",
  name: "Bê tông hoá đường trục chính thôn Hà Lam",
  planned_amount: 7500000000,
  approved_amount: 7500000000,
  approved_amount_set: false,
  disbursement_deadline: "2026-12-31",
  funding_allocated_total: 0,
};

const THEM_DU_AN_DAY_DU: ThemDuAnVao = {
  code: "DA01",
  year: 2026,
  category_id: "01JHM1",
  name: "Bê tông hoá đường trục chính thôn Hà Lam",
  description: "Tuyến 1,2 km",
  planned_amount: 7500000000,
  approved_amount: 9000000000,
  org_unit_id: "01JBP1",
  assignee_id: "01JCB1",
  start_date: "2026-03-01",
  completion_date: "2026-11-30",
  disbursement_deadline: "2026-12-31",
  funding_allocations: [{ funding_source_id: "01JNV1", amount: 7500000000 }],
};

describe("Đường dẫn và ranh giới xã", () => {
  it("mọi tuyến ghi dùng đường dẫn TƯƠNG ĐỐI, không host, không `tenant_id`", async () => {
    const gia = batFetch(traJSON(201, CHUNG_TU));

    await themChungTu(THEM_CHUNG_TU_DAY_DU, "khoa-1");
    await suaChungTu("01JCT1", { description: "x" });
    await xacNhanChungTu("01JCT1");
    await khoaChungTu("01JCT1");

    for (let i = 0; i < 4; i += 1) {
      const duong = String(loiGoi(gia, i).duongDan);
      expect(duong.startsWith("/api/")).toBe(true);
      expect(duong).not.toMatch(/^https?:/);
      // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã suy từ `Host`.
      expect(duong).not.toMatch(/tenant/i);
      expect(String(loiGoi(gia, i).tuyChon.body ?? "")).not.toMatch(/tenant/i);
    }
  });

  it("id đi vào đường dẫn được MÃ HOÁ, không ghép thẳng", async () => {
    const gia = batFetch(traJSON(200, CHUNG_TU));

    await xacNhanChungTu("a/b?c");

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/disbursements/a%2Fb%3Fc/confirmation");
  });
});

describe("POST /api/v1/disbursements — khoá chống trùng và thân", () => {
  it("MANG `Idempotency-Key` ĐÚNG GIÁ TRỊ ĐƯỢC TRUYỀN VÀO, và mong 201", async () => {
    const gia = batFetch(traJSON(201, CHUNG_TU));

    const kq = await themChungTu(THEM_CHUNG_TU_DAY_DU, "khoa-mo-bieu-mau");

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/disbursements");
    expect(goi.tuyChon.method).toBe("POST");
    // Sinh khoá BÊN TRONG hàm là không chống được gì: mỗi lần bấm lại sau lỗi mạng sẽ là một khoá
    // mới, trong khi lần gửi đầu có thể đã ghi một khoản chi.
    expect(goi.header.get("Idempotency-Key")).toBe("khoa-mo-bieu-mau");
    expect(kq.ok).toBe(true);
  });

  it("thân KHÔNG mang `status` — trường ấy là 400 ở máy chủ", async () => {
    const gia = batFetch(traJSON(201, CHUNG_TU));

    // Một đối tượng "đọc từ nơi khác" mang thêm trường máy chủ từ chối. Hàm dựng TỪNG TRƯỜNG nên
    // nó không đi lên dây; một `...than` sẽ làm bài kiểm này đỏ.
    const ban = { ...THEM_CHUNG_TU_DAY_DU, status: "da-khoa", entered_by: "CB-999" };
    await themChungTu(ban as ThemChungTuVao, "khoa-1");

    const than = thanDaGui(gia, 0);
    expect(than).not.toHaveProperty("status");
    expect(than).not.toHaveProperty("entered_by");
    expect(Object.keys(than).sort()).toEqual(
      [
        "amount",
        "counterparty",
        "description",
        "funding_source_id",
        "payment_date",
        "project_id",
        "voucher_no",
      ].sort(),
    );
  });

  it("503 (khoá chống trùng không tới được store) ra NGUYÊN VĂN câu máy chủ", async () => {
    batFetch(
      traJSON(503, { code: "idem_store", message: "Hệ thống đang bận, vui lòng thử lại sau.", trace_id: "t1" }),
    );

    const kq = await themChungTu(THEM_CHUNG_TU_DAY_DU, "khoa-1");

    expect(kq.ok).toBe(false);
    if (!kq.ok) expect(kq.thongBao).toBe("Hệ thống đang bận, vui lòng thử lại sau.");
  });
});

describe("PATCH /api/v1/disbursements/{id}", () => {
  it("KHÔNG mang `Idempotency-Key` (hợp đồng không đòi), mong 200", async () => {
    const gia = batFetch(traJSON(200, { ...CHUNG_TU, status: "ke-toan-nhap" }));

    const kq = await suaChungTu("01JCT1", SUA_CHUNG_TU_DAY_DU);

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/disbursements/01JCT1");
    expect(goi.tuyChon.method).toBe("PATCH");
    expect(goi.header.get("Idempotency-Key")).toBeNull();
    expect(kq.ok).toBe(true);
  });

  it("thân KHÔNG mang `status` và KHÔNG mang `project_id` — cả hai là 400 ở máy chủ", async () => {
    const gia = batFetch(traJSON(200, CHUNG_TU));

    const ban = { ...SUA_CHUNG_TU_DAY_DU, status: "da-xac-nhan", project_id: "01JDA9" };
    await suaChungTu("01JCT1", ban as SuaChungTuVao);

    const than = thanDaGui(gia, 0);
    expect(than).not.toHaveProperty("status");
    // Chuyển chứng từ sang dự án khác là chuyển tiền giữa hai con số đã báo cáo.
    expect(than).not.toHaveProperty("project_id");
  });

  it("`\"\"` ĐI LÊN DÂY: xoá trắng đối tác và GỠ nguồn vốn là hai giá trị có nghĩa", async () => {
    const gia = batFetch(traJSON(200, CHUNG_TU));

    await suaChungTu("01JCT1", { counterparty: "", funding_source_id: "" });

    const than = thanDaGui(gia, 0);
    expect(than.counterparty).toBe("");
    // `""` đưa chứng từ về đúng trạng thái §13 quy tắc 6 — "đã chi nhưng chưa ghi rút từ nguồn
    // nào". Rụng mất nó thì lần gỡ nguồn im lặng không xảy ra.
    expect(than.funding_source_id).toBe("");
  });

  it("trường `undefined` RỤNG khỏi thân — vắng mặt là 'để nguyên'", async () => {
    const gia = batFetch(traJSON(200, CHUNG_TU));

    await suaChungTu("01JCT1", { description: "Sửa nội dung" });

    expect(Object.keys(thanDaGui(gia, 0))).toEqual(["description"]);
  });
});

describe("Ba tuyến vòng đời — xác nhận · khoá · mở khoá", () => {
  it("xác nhận: POST, KHÔNG thân và KHÔNG `Content-Type`", async () => {
    const gia = batFetch(traJSON(200, { ...CHUNG_TU, status: "da-xac-nhan" }));

    await xacNhanChungTu("01JCT1");

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/disbursements/01JCT1/confirmation");
    expect(goi.tuyChon.method).toBe("POST");
    expect(goi.tuyChon.body).toBeUndefined();
    // Gửi `{}` kèm `Content-Type` là tuyên bố có một thân — thứ mời người sau điền vào.
    expect(goi.header.get("Content-Type")).toBeNull();
  });

  it("khoá: POST `/lockout`, KHÔNG thân", async () => {
    const gia = batFetch(traJSON(200, { ...CHUNG_TU, status: "da-khoa" }));

    await khoaChungTu("01JCT1");

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/disbursements/01JCT1/lockout");
    expect(goi.tuyChon.method).toBe("POST");
    expect(goi.tuyChon.body).toBeUndefined();
  });

  it("mở khoá: DELETE `/lockout` KÈM lý do trong THÂN, không trong chuỗi truy vấn", async () => {
    const gia = batFetch(traJSON(200, { ...CHUNG_TU, status: "da-xac-nhan", unlock_count: 1 }));

    const kq = await moKhoaChungTu("01JCT1", "Sai số tiền, phải sửa lại theo hoá đơn");

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/disbursements/01JCT1/lockout");
    expect(goi.tuyChon.method).toBe("DELETE");
    // Chữ tự do về chi tiêu của một cơ quan nhà nước mà nằm trong URL là chữ nằm lại trong mọi
    // nhật ký truy cập (luật 3, cấm #4).
    expect(goi.duongDan).not.toMatch(/reason/);
    expect(thanDaGui(gia, 0)).toEqual({ reason: "Sai số tiền, phải sửa lại theo hoá đơn" });
    expect(kq.ok).toBe(true);
  });

  it("409 'người vừa khoá không tự mở lại được' ra NGUYÊN VĂN", async () => {
    const cau =
      "chung_tu: người vừa khoá chứng từ không tự mở lại được — cần một cán bộ khác có quyền " +
      "`budget.confirm`";
    batFetch(traJSON(409, { code: "voucher_state", message: cau, trace_id: "t2" }));

    const kq = await moKhoaChungTu("01JCT1", "lý do");

    expect(kq.ok).toBe(false);
    // Nuốt câu này thành "Có lỗi xảy ra" là lấy mất đúng thứ cán bộ cần để biết phải làm gì.
    if (!kq.ok) expect(kq.thongBao).toBe(cau);
  });
});

describe("DELETE /api/v1/disbursements/{id} — gỡ mềm", () => {
  it("204 KHÔNG THÂN là THÀNH CÔNG, không phải 'không đọc được'", async () => {
    const gia = batFetch(traTrong(204));

    const kq = await goChungTu("01JCT1", "Nhập trùng với chứng từ CT-2026-0911");

    expect(loiGoi(gia, 0).tuyChon.method).toBe("DELETE");
    expect(thanDaGui(gia, 0)).toEqual({ reason: "Nhập trùng với chứng từ CT-2026-0911" });
    expect(kq.ok).toBe(true);
    if (kq.ok) expect(kq.duLieu).toBeNull();
  });

  it("200 thay vì 204 là HỎNG: mã mong đợi của tuyến này là 204", async () => {
    batFetch(traJSON(200, {}));

    const kq = await goChungTu("01JCT1", "lý do");

    expect(kq.ok).toBe(false);
  });
});

describe("Ba tuyến dự án", () => {
  it("POST: `Idempotency-Key` bắt buộc, mong 201, phân bổ nguồn vốn dựng lại TỪNG DÒNG", async () => {
    const gia = batFetch(traJSON(201, DU_AN_GHI_RA));

    const ban = {
      ...THEM_DU_AN_DAY_DU,
      funding_allocations: [
        { funding_source_id: "01JNV1", amount: 7500000000, ghi_chu: "thừa" },
      ],
    };
    const kq = await themDuAn(ban as ThemDuAnVao, "khoa-du-an");

    const goi = loiGoi(gia, 0);
    expect(goi.duongDan).toBe("/api/v1/investment-projects");
    expect(goi.header.get("Idempotency-Key")).toBe("khoa-du-an");
    expect(thanDaGui(gia, 0).funding_allocations).toEqual([
      { funding_source_id: "01JNV1", amount: 7500000000 },
    ]);
    expect(kq.ok).toBe(true);
  });

  it("PATCH: thân KHÔNG mang `code` và KHÔNG mang `year`", async () => {
    const gia = batFetch(traJSON(200, DU_AN_GHI_RA));

    const ban = { name: "Tên mới", code: "DA99", year: 2027 };
    await suaDuAn("01JDA1", ban as SuaDuAnVao);

    const than = thanDaGui(gia, 0);
    // Mã đã cấp thì không đánh lại (luật 7, cấm #4); mỗi năm là một tập dự án riêng (§13 quy tắc 8).
    expect(than).not.toHaveProperty("code");
    expect(than).not.toHaveProperty("year");
    expect(than.name).toBe("Tên mới");
  });

  it("PATCH không mang `Idempotency-Key`, và mong 200", async () => {
    const gia = batFetch(traJSON(200, DU_AN_GHI_RA));

    const kq = await suaDuAn("01JDA1", { name: "Tên mới" });

    expect(loiGoi(gia, 0).header.get("Idempotency-Key")).toBeNull();
    expect(kq.ok).toBe(true);
  });

  it("DELETE: 204 kèm lý do trong thân; 409 'còn chứng từ' ra nguyên văn", async () => {
    const gia = batFetch(traTrong(204));
    const kq = await xoaDuAn("01JDA1", "Trùng với dự án DA07");

    expect(loiGoi(gia, 0).duongDan).toBe("/api/v1/investment-projects/01JDA1");
    expect(thanDaGui(gia, 0)).toEqual({ reason: "Trùng với dự án DA07" });
    expect(kq.ok).toBe(true);

    const cau =
      "Dự án này còn chứng từ giải ngân nên chưa xoá được. Hãy gỡ các chứng từ kèm lý do trước, " +
      "rồi xoá dự án.";
    batFetch(traJSON(409, { code: "project_has_vouchers", message: cau, trace_id: "t3" }));
    const kq2 = await xoaDuAn("01JDA1", "lý do");

    expect(kq2.ok).toBe(false);
    if (!kq2.ok) expect(kq2.thongBao).toBe(cau);
  });
});

describe("Cookie phiên và bộ đệm — giống hệt mọi tuyến khác", () => {
  it("`credentials: same-origin` và `cache: no-store` trên mọi lần ghi", async () => {
    const gia = batFetch(traJSON(201, CHUNG_TU));

    await themChungTu(THEM_CHUNG_TU_DAY_DU, "k");

    const { tuyChon } = loiGoi(gia, 0);
    // "include" chỉ cần khi gửi sang origin khác — mà gửi phiên sang origin khác là đúng điều
    // không được phép xảy ra ở đây.
    expect(tuyChon.credentials).toBe("same-origin");
    expect(tuyChon.cache).toBe("no-store");
  });
});
