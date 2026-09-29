import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * LỚP GỌI ViGov VỚI MỘT PHIÊN GIẢ — chỉ trong tệp này.
 *
 * `vi.mock` thay `vigov-session` và `vigov-address` cho RIÊNG tệp test này: mã sản phẩm vẫn chỉ có một
 * nguồn phiên, và nguồn ấy trả `null` (`citizen.test.tsx` khẳng định điều đó trên mã thật). Không
 * có khe tham số nào trong mã sản phẩm để "đưa" một phiên vào — đó là lý do phải giả lập ở tầng mô-đun.
 */
const page = vi.hoisted(() => ({
  session: { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" } as { token: string; commune_name: string } | null,
  host: "https://vigov.vidu.vn",
}));

vi.mock("./vigov-session", () => ({ getVigovSession: () => page.session }));
vi.mock("./vigov-address", () => ({
  vigovAddress: (_service: "petitions", d: string) => (page.host === "" ? "" : `${page.host}${d}`),
}));

import { SendResult } from "../screens/SubmitReportScreen";
import { MY_REPORTS, SEND, REPORT_CARD, LOOKUP, STATUS, STATUS_UNLABELLED } from "../screens/copy";
import {
  startLoading,
  FIRST_LIST,
  MyReportsScreen,
  afterLoad,
  ListBody,
  ReportSummaryCard,
} from "../screens/MyReportsScreen";
import { LookupResult, ReportLookupScreen } from "../screens/ReportLookupScreen";

import { submitReport, myReports, lookupReport } from "./vigov-client";
import {
  BRANCH_END_MAX_LENGTH,
  readReport,
  readMyReportsPage,
  MY_REPORTS_PATH,
  OPTIONAL_SCENE_FIELDS,
  type NewReport,
  type MyReportSummary,
  PAGE_SIZE,
  submitReportBody,
  ACCEPTED_FIELDS,
} from "./citizen-report-contract";
import { createSendAttempt } from "./send-attempt";

type Call = { address: string; options: RequestInit };

let calls: Call[] = [];

/** Một phản hồi giả. Không dùng `Response` để khỏi phụ thuộc môi trường Node có hay không. */
function respond(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(...responses: Array<ReturnType<typeof respond> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (address: string, options: RequestInit) => {
    calls.push({ address, options });
    const p = responses[Math.min(i++, responses.length - 1)]!;
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const headers = (g: Call) => g.options.headers as Record<string, string>;

const REPORT_OUT = {
  code: "PA7K2QX9M4TD",
  channel: "zalo-mini-app",
  status: "da-tiep-nhan",
  field: "",
  field_label: "",
  content: "Ổ gà lớn trước cổng chợ",
  address: "Đầu ngõ thôn Hà Lam",
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0000",
  anonymous: false,
  clock_from: "2026-09-24T01:30:00Z",
  acknowledge_due: "2026-09-24T03:30:00Z",
  resolve_due: null,
  result: "",
};

const PA: NewReport = {
  content: "  Ổ gà lớn trước cổng chợ  ",
  address: "Đầu ngõ thôn Hà Lam",
  full_name: "Nguyễn Văn A",
  phone: "0900000000",
  anonymous: false,
};

beforeEach(() => {
  calls = [];
  page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
  page.host = "https://vigov.vidu.vn";
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("gửi phản ánh — khoá chống trùng, bearer, thân", () => {
  it("POST đúng tuyến, bearer lấy từ nguồn phiên, có Idempotency-Key", async () => {
    stubFetch(respond(201, REPORT_OUT));
    const result = await submitReport(createSendAttempt(submitReportBody(PA)));

    expect(result.kind).toBe("xong");
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}`);
    expect(calls[0]!.options.method).toBe("POST");
    expect(headers(calls[0]!)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    expect(headers(calls[0]!)["Idempotency-Key"]).toMatch(/^[0-9a-f]{32}$/);
    // Dữ liệu cá nhân không bao giờ nằm trên đường dẫn.
    expect(calls[0]!.address).not.toContain("0900000000");
  });

  it("GỬI LẠI cùng lần gửi: cùng khoá, cùng thân. Lần gửi MỚI: khoá mới", async () => {
    stubFetch(new Error("mất mạng"), respond(201, REPORT_OUT), respond(201, REPORT_OUT));
    const attempt = createSendAttempt(submitReportBody(PA));

    expect(await submitReport(attempt)).toEqual({ kind: "loi-mang" });
    expect((await submitReport(attempt)).kind).toBe("xong");
    const new_attempt = createSendAttempt(submitReportBody(PA));
    await submitReport(new_attempt);

    const keys = calls.map((g) => headers(g)["Idempotency-Key"]);
    expect(keys[0]).toBe(keys[1]);
    expect(calls[0]!.options.body).toBe(calls[1]!.options.body);
    expect(keys[2]).not.toBe(keys[0]);
  });

  it("thân mang ĐÚNG năm trường máy chủ nhận — không lĩnh vực, không xã, không người gửi", () => {
    const body = JSON.parse(submitReportBody(PA)) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual([...ACCEPTED_FIELDS].sort());
    for (const banned of [
      "field",
      "linh_vuc",
      "tenant",
      "tenant_id",
      "commune",
      "xa",
      "citizen_id",
      "cong_dan_id",
      "channel",
      "code",
      "status",
      "clock_from",
      "acknowledge_due",
      "resolve_due",
    ]) {
      expect(body, `thân gửi đi mang trường bị từ chối: ${banned}`).not.toHaveProperty(banned);
    }
    expect(body["content"]).toBe("Ổ gà lớn trước cổng chợ");

    // 29/09/2026 (ViGov b5d17bb): the ONLY keys allowed beyond the five are the optional pair `lat`/`lng`,
    // and only together. The same refused list still holds with a location attached.
    const allowed = [...ACCEPTED_FIELDS, ...OPTIONAL_SCENE_FIELDS].sort();
    const with_location = JSON.parse(
      submitReportBody({ ...PA, scene_location: { lat: 15.57, lng: 108.47 } }),
    ) as Record<string, unknown>;
    expect(Object.keys(with_location).sort()).toEqual(allowed);
    const partial = JSON.parse(submitReportBody({ ...PA, scene_location: { lat: 15.57 } as never })) as Record<
      string,
      unknown
    >;
    expect(Object.keys(partial).sort(), "a half location went on the wire").toEqual([...ACCEPTED_FIELDS].sort());
  });

  it("POST with a location: `lat`/`lng` are JSON numbers in the body, never on the address", async () => {
    stubFetch(respond(201, { ...REPORT_OUT, lat: 15.57, lng: 108.47 }));
    const result = await submitReport(createSendAttempt(submitReportBody({ ...PA, scene_location: { lat: 15.57, lng: 108.47 } })));
    expect(result.kind).toBe("xong");
    const body = JSON.parse(calls[0]!.options.body as string) as Record<string, unknown>;
    expect(body["lat"]).toBe(15.57);
    expect(body["lng"]).toBe(108.47);
    // Coordinates are personal data (often a doorstep): never in the URL (rule 3, forbidden #4).
    expect(calls[0]!.address).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}`);
    expect(calls[0]!.address).not.toMatch(/15\.57|108\.47/);
  });

  it("gửi ẩn danh thì KHÔNG gửi họ tên và số điện thoại", () => {
    const body = JSON.parse(submitReportBody({ ...PA, anonymous: true })) as Record<string, unknown>;
    expect(body["anonymous"]).toBe(true);
    expect(body["reporter_name"]).toBe("");
    expect(body["reporter_phone"]).toBe("");
  });

  it("201 vẽ ra MÃ TRA CỨU, tình trạng, và hạn xem phiếu theo giờ Việt Nam", async () => {
    stubFetch(respond(201, REPORT_OUT));
    const result = await submitReport(createSendAttempt(submitReportBody(PA)));
    if (result.kind !== "xong") throw new Error(`mong đợi xong, nhận ${result.kind}`);

    const html = renderToStaticMarkup(createElement(SendResult, { report: result.report, onSendAnother: () => {} }));
    expect(html).toContain("PA7K2QX9M4TD");
    expect(html).toContain("Đã tiếp nhận");
    // 03:30Z = 10:30 giờ Việt Nam.
    expect(html).toContain(SEND.will_view_by("24/09/2026 10:30"));
    // Chỉ những gì máy chủ trả: số điện thoại ĐÃ CHE, không phải số người dân gõ.
    expect(html).toContain("09****0000");
    expect(html).not.toContain("0900000000");
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh", async () => {
    for (const [status, kind] of [
      [400, "khong-hop-le"],
      [401, "het-phien"],
      [409, "dang-xu-ly-truoc"],
      [503, "kenh-chua-mo"],
      [500, "loi-may-chu"],
      [502, "loi-may-chu"],
    ] as const) {
      stubFetch(respond(status, { code: "x", message: "y", trace_id: "" }));
      expect((await submitReport(createSendAttempt("{}"))).kind, `mã ${status}`).toBe(kind);
    }
    stubFetch(respond(201, { code: "" }));
    expect((await submitReport(createSendAttempt("{}"))).kind, "201 sai khuôn").toBe("loi-may-chu");
  });

  it("không phiên hoặc không địa chỉ: dừng TRƯỚC fetch", async () => {
    const fake_fetch = vi.fn();
    vi.stubGlobal("fetch", fake_fetch);

    page.session = null;
    expect(await submitReport(createSendAttempt("{}"))).toEqual({ kind: "chua-co-phien" });
    expect(await lookupReport("PA7K2QX9M4TD")).toEqual({ kind: "chua-co-phien" });

    page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    page.host = "";
    expect(await submitReport(createSendAttempt("{}"))).toEqual({ kind: "chua-cau-hinh" });
    expect(await lookupReport("PA7K2QX9M4TD")).toEqual({ kind: "chua-cau-hinh" });

    expect(fake_fetch).not.toHaveBeenCalled();
  });
});

describe("tra cứu phiếu", () => {
  it("GET đúng tuyến với mã đã mã hoá, bearer, KHÔNG có Idempotency-Key", async () => {
    stubFetch(respond(200, REPORT_OUT));
    const result = await lookupReport("  PA7K2QX9M4TD ");
    expect(result.kind).toBe("xong");
    expect(calls[0]!.address).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}/PA7K2QX9M4TD`);
    expect(calls[0]!.options.method).toBe("GET");
    expect(headers(calls[0]!)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    expect(headers(calls[0]!)).not.toHaveProperty("Idempotency-Key");

    stubFetch(respond(404, {}));
    await lookupReport("a/../b");
    expect(calls[1]!.address).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}/a%2F..%2Fb`);
  });

  it("404 là MỘT câu trung tính — bất kể máy chủ nói gì trong thân", async () => {
    stubFetch(respond(404, { code: "not_found", message: "Không tìm thấy phiếu phản ánh.", trace_id: "t1" }));
    const first = await lookupReport("MA-KHONG-CO");
    stubFetch(respond(404, { code: "khac", message: "một câu khác hẳn", trace_id: "t2" }));
    const second = await lookupReport("MA-CUA-NGUOI-KHAC");
    expect(first).toEqual({ kind: "khong-thay" });
    expect(second).toEqual(first);

    const render = (result: typeof first) => renderToStaticMarkup(createElement(LookupResult, { result }));
    expect(render(first)).toContain(LOOKUP.not_found);
    expect(render(second)).toBe(render(first));
    expect(render(first)).not.toContain("một câu khác hẳn");
  });

  it("phiếu đã đóng hiện kết quả của xã, hai hạn theo +07, và không lộ trường nội bộ", async () => {
    stubFetch(
      respond(200, {
        ...REPORT_OUT,
        status: "da-dong",
        field: "giao-thong",
        field_label: "",
        resolve_due: "2026-09-26T09:00:00Z",
        result: "Đã vá ổ gà ngày 25/09.",
        assignee: "01JCANBONOIBO000000000000",
        note: "ghi chú nội bộ của cán bộ",
      }),
    );
    const result = await lookupReport("PA7K2QX9M4TD");
    const html = renderToStaticMarkup(createElement(LookupResult, { result }));
    expect(html).toContain("Đã đóng");
    expect(html).toContain("Đã vá ổ gà ngày 25/09.");
    expect(html).toContain("26/09/2026 16:00");
    expect(html).toContain("24/09/2026 10:30");
    // Mã lĩnh vực thô không hiện; trường máy chủ lỡ gửi thêm không đi tới màn hình.
    expect(html).not.toContain("giao-thong");
    expect(html).not.toContain("01JCANBONOIBO");
    expect(html).not.toContain("ghi chú nội bộ");
  });

  it("phiếu ẩn danh không hiện họ tên hay số, kể cả đã che", async () => {
    stubFetch(respond(200, { ...REPORT_OUT, anonymous: true, reporter_name: "", reporter_phone: "" }));
    const html = renderToStaticMarkup(
      createElement(LookupResult, { result: await lookupReport("PA7K2QX9M4TD") }),
    );
    expect(html).toContain("Gửi ẩn danh");
    expect(html).not.toContain("09****");
  });
});

/**
 * HAI NHÁNH KẾT THÚC — `reason` và `receiving_body` (migration 0011, `phieu_cua_toi.go`). Hai trường
 * TUỲ CHỌN, chỉ có nghĩa ở `khong-tiep-nhan` (lý do) và `chuyen-cap-tren` (lý do + cơ quan nhận).
 */
describe("tra cứu phiếu — lý do và cơ quan nhận của hai nhánh kết thúc", () => {
  const REASON = "Việc thuộc thẩm quyền của Ban quản lý khu công nghiệp.";
  const RECEIVING_BODY = "Ban quản lý các khu công nghiệp tỉnh";

  async function renderLookup(body: unknown): Promise<string> {
    stubFetch(respond(200, body));
    const result = await lookupReport("PA7K2QX9M4TD");
    expect(result.kind).toBe("xong");
    return renderToStaticMarkup(createElement(LookupResult, { result }));
  }

  it("parser nhận hai trường khi có, và coi vắng mặt là rỗng", () => {
    const rejected = readReport({ ...REPORT_OUT, status: "khong-tiep-nhan", reason: REASON });
    expect(rejected?.reason).toBe(REASON);
    expect(rejected?.receiving_body).toBe("");

    const referred = readReport({ ...REPORT_OUT, status: "chuyen-cap-tren", reason: REASON, receiving_body: RECEIVING_BODY });
    expect(referred?.reason).toBe(REASON);
    expect(referred?.receiving_body).toBe(RECEIVING_BODY);

    // Phiếu cũ, không có hai khoá: vẫn là một phiếu hợp lệ.
    const old = readReport(REPORT_OUT);
    expect(old).not.toBeNull();
    expect(old?.reason).toBe("");
    expect(old?.receiving_body).toBe("");
  });

  it("parser từ chối sai kiểu, kể cả ở trạng thái không phải nhánh", () => {
    for (const status of ["khong-tiep-nhan", "chuyen-cap-tren", "da-tiep-nhan"]) {
      for (const wrong of [null, 42, true, ["x"], { vi: "x" }]) {
        expect(readReport({ ...REPORT_OUT, status, reason: wrong }), `${status} reason=${String(wrong)}`).toBeNull();
        expect(readReport({ ...REPORT_OUT, status, receiving_body: wrong }), `${status} body=${String(wrong)}`).toBeNull();
      }
    }
  });

  it("giới hạn đếm theo KÝ TỰ: 2000 / 200 ký tự có dấu nhận, thêm một ký tự là sai khuôn", () => {
    expect(BRANCH_END_MAX_LENGTH).toEqual({ reason: 2000, receiving_body: 200 });
    const base = { ...REPORT_OUT, status: "chuyen-cap-tren" };
    // "ệ" là một ký tự; đếm theo byte UTF-8 sẽ là ba và từ chối oan.
    expect(readReport({ ...base, reason: "ệ".repeat(2000) })?.reason).toHaveLength(2000);
    expect(readReport({ ...base, reason: "ệ".repeat(2001) })).toBeNull();
    expect(readReport({ ...base, receiving_body: "ệ".repeat(200) })?.receiving_body).toHaveLength(200);
    expect(readReport({ ...base, receiving_body: "ệ".repeat(201) })).toBeNull();
    // Ký tự ngoài BMP là HAI đơn vị UTF-16 nhưng MỘT ký tự — như `utf8.RuneCountInString`.
    expect(readReport({ ...base, receiving_body: "𠀀".repeat(200) })).not.toBeNull();
  });

  it("trạng thái khác: hai trường bị bỏ đi và KHÔNG hiện, dù máy chủ lỡ gửi", async () => {
    for (const status of Object.keys(STATUS).filter((s) => s !== "khong-tiep-nhan" && s !== "chuyen-cap-tren")) {
      const body = { ...REPORT_OUT, status, reason: REASON, receiving_body: RECEIVING_BODY };
      const p = readReport(body);
      expect(p?.reason, status).toBe("");
      expect(p?.receiving_body, status).toBe("");
      const html = await renderLookup(body);
      expect(html, status).not.toContain(REASON);
      expect(html, status).not.toContain(RECEIVING_BODY);
      expect(html, status).not.toContain(REPORT_CARD.rejection_reason);
      expect(html, status).not.toContain(REPORT_CARD.receiving_body_label);
      expect(html, status).not.toContain(REPORT_CARD.referral_reason);
    }
  });

  it("không tiếp nhận: hiện lý do, KHÔNG hiện cơ quan nhận (không ai nhận cả)", async () => {
    const html = await renderLookup({ ...REPORT_OUT, status: "khong-tiep-nhan", reason: REASON, receiving_body: RECEIVING_BODY });
    // Chip là nhóm "Đã đóng" (ADR 0050 #5); dòng phụ ngay dưới nói vì sao — xã không tiếp nhận.
    expect(html).toContain('<strong class="cd-phieu__trang-thai">Đã đóng</strong>');
    expect(html).toContain(STATUS["khong-tiep-nhan"]!.explanation!);
    expect(html).toContain(REPORT_CARD.rejection_reason);
    expect(html).toContain(REASON);
    expect(html).not.toContain(REPORT_CARD.receiving_body_label);
    expect(html).not.toContain(RECEIVING_BODY);
    expect(html).not.toContain(REPORT_CARD.contact_body);
  });

  it("chuyển cấp trên: cơ quan tiếp nhận, lý do chuyển, và việc làm tiếp", async () => {
    const html = await renderLookup({ ...REPORT_OUT, status: "chuyen-cap-tren", reason: REASON, receiving_body: RECEIVING_BODY });
    // Chip là nhóm "Đã đóng" (ADR 0050 #5); dòng phụ ngay dưới nói phiếu đã chuyển đi.
    expect(html).toContain('<strong class="cd-phieu__trang-thai">Đã đóng</strong>');
    expect(html).toContain(STATUS["chuyen-cap-tren"]!.explanation!);
    expect(html).toContain(REPORT_CARD.receiving_body_label);
    expect(html).toContain(RECEIVING_BODY);
    expect(html).toContain(REPORT_CARD.referral_reason);
    expect(html).toContain(REASON);
    expect(html).toContain(REPORT_CARD.contact_body);
    // Cơ quan trước, lý do sau — cùng thứ tự câu dòng phụ trạng thái chỉ xuống.
    expect(html.indexOf(RECEIVING_BODY)).toBeLessThan(html.indexOf(REASON));
  });

  it("nhánh kết thúc mà máy chủ không gửi chữ: nói việc cần làm, không để ô trống", async () => {
    const html = await renderLookup({ ...REPORT_OUT, status: "chuyen-cap-tren" });
    expect(html).toContain(REPORT_CARD.receiving_body_label);
    expect(html).toContain(REPORT_CARD.not_recorded);
    // Không có tên cơ quan thì không mời "liên hệ cơ quan ở trên".
    expect(html).not.toContain(REPORT_CARD.contact_body);
  });

  it("lý do dài hiện ĐỦ, trong lớp xuống dòng, không bị cắt", async () => {
    const long_reason = `${"Xã đã xác minh tại hiện trường và nhận thấy ".repeat(40)}HẾT.\nDòng hai của lý do.`;
    expect([...long_reason].length).toBeLessThanOrEqual(2000);
    const html = await renderLookup({ ...REPORT_OUT, status: "khong-tiep-nhan", reason: long_reason });
    expect(html).toContain(`<span class="cd-phieu__ly-do">${long_reason}</span>`);

    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const block = /\.cd-phieu__ly-do\s*\{([^}]*)\}/.exec(css);
    expect(block, "styles.css không còn khối nào cho .cd-phieu__ly-do").not.toBeNull();
    expect(block![1]).toMatch(/white-space:\s*pre-wrap/);
    expect(block![1]).toMatch(/overflow-wrap:\s*anywhere/);
    // Không một luật nào ở bất kỳ đâu cắt chữ của lớp này.
    for (const m of css.matchAll(/([^{}]*cd-phieu__ly-do[^{}]*)\{([^}]*)\}/g)) {
      expect(m[2]).not.toMatch(/text-overflow|line-clamp|max-height|overflow:\s*hidden|nowrap/);
    }
  });

  it("đọc và vẽ hai nhánh không ghi gì ra console (luật 3)", async () => {
    const spies = (["log", "info", "warn", "error", "debug"] as const).map((k) => vi.spyOn(console, k));
    await renderLookup({ ...REPORT_OUT, status: "chuyen-cap-tren", reason: REASON, receiving_body: RECEIVING_BODY });
    readReport({ ...REPORT_OUT, status: "khong-tiep-nhan", reason: 42 });
    for (const s of spies) {
      expect(s).not.toHaveBeenCalled();
      s.mockRestore();
    }
  });
});

/**
 * "PHẢN ÁNH CỦA TÔI" — `GET /api/v1/my-citizen-reports` (danh sách), với phiên giả của tệp này.
 */
describe("phản ánh của tôi — lời gọi và đọc trang", () => {
  const ROW_OUT = {
    code: "PA7K2QX9M4TD",
    status: "dang-xu-ly",
    field: "giao-thong",
    field_label: "Giao thông",
    content_excerpt: "Ổ gà lớn trước cổng chợ…",
    clock_from: "2026-09-24T01:30:00Z",
    acknowledge_due: "2026-09-24T03:30:00Z",
    resolve_due: "2026-09-26T09:00:00Z",
  };
  const PAGE_OUT = { items: [ROW_OUT], next_cursor: "c1+/=&x", has_more: true };

  /** Mọi tham số trên đường dẫn của một lời gọi. */
  const queryParams = (g: Call) => new URL(g.address).searchParams;

  it("GET đúng tuyến, bearer từ nguồn phiên, CHỈ `limit` ở trang đầu — không danh tính, không xã", async () => {
    stubFetch(respond(200, PAGE_OUT));
    const result = await myReports("");
    expect(result.kind).toBe("xong");
    expect(calls).toHaveLength(1);

    const g = calls[0]!;
    const url = new URL(g.address);
    expect(`${url.origin}${url.pathname}`).toBe(`https://vigov.vidu.vn${MY_REPORTS_PATH}`);
    expect(g.options.method).toBe("GET");
    expect(g.options.body).toBeUndefined();
    expect(headers(g)["Authorization"]).toBe("Bearer tok-thu-nghiem");
    // Đúng hai tiêu đề — không tiêu đề nào mang xã hay danh tính, không khoá chống trùng.
    expect(Object.keys(headers(g)).sort()).toEqual(["Accept", "Authorization"]);
    expect([...queryParams(g).keys()]).toEqual(["limit"]);
    expect(queryParams(g).get("limit")).toBe(String(PAGE_SIZE));
    expect(PAGE_SIZE).toBeGreaterThanOrEqual(1);
    expect(PAGE_SIZE).toBeLessThanOrEqual(100);
  });

  it("trang sau truyền con trỏ NGUYÊN VĂN — kể cả ký tự `+ / = &`", async () => {
    stubFetch(respond(200, PAGE_OUT));
    await myReports("c1+/=&x");
    const q = queryParams(calls[0]!);
    expect(q.get("cursor")).toBe("c1+/=&x");
    expect([...q.keys()].sort()).toEqual(["cursor", "limit"]);
  });

  it("không một tham số nào nói của ai / xã nào / xếp thế nào, ở mọi trang", async () => {
    stubFetch(respond(200, PAGE_OUT));
    await myReports("");
    await myReports("c1");
    expect(calls).toHaveLength(2);
    for (const g of calls) {
      for (const banned of [
        "tenant",
        "tenant_id",
        "commune",
        "xa",
        "phone",
        "reporter_phone",
        "citizen_id",
        "cong_dan_id",
        "sort",
        "order",
        "status",
      ]) {
        expect(queryParams(g).has(banned), `tham số bị cấm: ${banned}`).toBe(false);
      }
    }
  });

  it("ánh xạ đủ trường, giữ hai `null` của hai hạn", async () => {
    stubFetch(
      respond(200, {
        items: [ROW_OUT, { ...ROW_OUT, code: "PB2", acknowledge_due: null, resolve_due: null }],
        next_cursor: "",
        has_more: false,
      }),
    );
    const result = await myReports("");
    if (result.kind !== "xong") throw new Error(`mong đợi xong, nhận ${result.kind}`);
    expect(result.page.has_more).toBe(false);
    expect(result.page.cursor).toBe("");
    expect(result.page.entries[0]).toEqual({
      lookup_code: "PA7K2QX9M4TD",
      status: "dang-xu-ly",
      field: "giao-thong",
      field_label: "Giao thông",
      content_excerpt: "Ổ gà lớn trước cổng chợ…",
      clock_from: "2026-09-24T01:30:00Z",
      acknowledge_due: "2026-09-24T03:30:00Z",
      resolve_due: "2026-09-26T09:00:00Z",
      rating: null,
      rated_at: null,
    });
    expect(result.page.entries[1]!.acknowledge_due).toBeNull();
    expect(result.page.entries[1]!.resolve_due).toBeNull();
  });

  it("danh sách rỗng là một trang hợp lệ", () => {
    expect(readMyReportsPage({ items: [], next_cursor: "", has_more: false })).toEqual({
      entries: [],
      cursor: "",
      has_more: false,
    });
  });

  it("sai khuôn là null — một dòng hỏng làm hỏng cả trang, không bỏ lặng lẽ", () => {
    const missing_code: Record<string, unknown> = { ...ROW_OUT };
    delete missing_code["code"];
    for (const wrong of [
      null,
      [],
      { items: null, next_cursor: "", has_more: false },
      { items: [], has_more: false },
      { items: [], next_cursor: "", has_more: "false" },
      // Còn nữa mà không có con trỏ: "Xem thêm" sẽ tải lại trang đầu và nhân đôi danh sách.
      { items: [], next_cursor: "", has_more: true },
      { items: [ROW_OUT, missing_code], next_cursor: "", has_more: false },
      { items: [{ ...ROW_OUT, code: "" }], next_cursor: "", has_more: false },
      { items: [{ ...ROW_OUT, resolve_due: 5 }], next_cursor: "", has_more: false },
      { items: [{ ...ROW_OUT, content_excerpt: undefined }], next_cursor: "", has_more: false },
    ]) {
      expect(readMyReportsPage(wrong), JSON.stringify(wrong)).toBeNull();
    }
  });

  it("mỗi mã trạng thái rơi vào đúng một nhánh — 404/409/503 không mượn câu của tuyến khác", async () => {
    for (const [status, kind] of [
      [400, "khong-hop-le"],
      [401, "het-phien"],
      [404, "loi-may-chu"],
      [409, "loi-may-chu"],
      [503, "loi-may-chu"],
      [500, "loi-may-chu"],
    ] as const) {
      stubFetch(respond(status, { code: "x", message: "y", trace_id: "" }));
      expect((await myReports("")).kind, `mã ${status}`).toBe(kind);
    }
    stubFetch(respond(200, { items: "x" }));
    expect((await myReports("")).kind, "200 sai khuôn").toBe("loi-may-chu");
    stubFetch(new Error("mất mạng"));
    expect(await myReports("")).toEqual({ kind: "loi-mang" });
  });

  it("không phiên hoặc không địa chỉ: dừng TRƯỚC fetch", async () => {
    const fake_fetch = vi.fn();
    vi.stubGlobal("fetch", fake_fetch);
    page.session = null;
    expect(await myReports("")).toEqual({ kind: "chua-co-phien" });
    page.session = { token: "", commune_name: "Xã Thử Nghiệm" };
    expect(await myReports("c1")).toEqual({ kind: "chua-co-phien" });
    page.session = { token: "tok-thu-nghiem", commune_name: "Xã Thử Nghiệm" };
    page.host = "";
    expect(await myReports("")).toEqual({ kind: "chua-cau-hinh" });
    expect(fake_fetch).not.toHaveBeenCalled();
  });
});

describe("phản ánh của tôi — màn hình", () => {
  const report = (code: string, status = "da-tiep-nhan"): MyReportSummary => ({
    lookup_code: code,
    status,
    field: "",
    field_label: "",
    content_excerpt: "Đèn đường hỏng ở đầu ngõ",
    clock_from: "2026-09-24T01:30:00Z",
    acknowledge_due: "2026-09-24T03:30:00Z",
    resolve_due: null,
    rating: null,
    rated_at: null,
  });
  const done = (entries: MyReportSummary[], cursor: string) =>
    ({ kind: "xong", page: { entries, cursor, has_more: cursor !== "" } }) as const;
  const render = (list: typeof FIRST_LIST) =>
    renderToStaticMarkup(
      createElement(ListBody, { list, onOpen: () => {}, onLoad: () => {}, onSubmitReport: () => {} }),
    );

  it("có phiên: màn mở ra ở trạng thái đang tải, có tên xã của phiên, chưa hiện câu 'chưa gửi'", () => {
    const html = renderToStaticMarkup(
      createElement(MyReportsScreen, { onBack: () => {}, onOpenReport: () => {}, onSubmitReport: () => {} }),
    );
    expect(html).toContain(MY_REPORTS.title);
    expect(html).toContain("Xã Thử Nghiệm");
    expect(html).toContain(MY_REPORTS.loading);
    expect(html).not.toContain(MY_REPORTS.empty);
  });

  it("rỗng: câu 'Bạn chưa gửi phản ánh nào.' và một nút Gửi phản ánh", () => {
    const html = render(afterLoad(FIRST_LIST, done([], "")));
    expect(MY_REPORTS.empty).toBe("Bạn chưa gửi phản ánh nào.");
    expect(html).toContain(MY_REPORTS.empty);
    expect(html).toContain(`<button type="button" class="cd-nut">${SEND.title}</button>`);
    expect(html).not.toContain(MY_REPORTS.load_more_button);
    expect(html).not.toContain(MY_REPORTS.loading);
  });

  it("'Xem thêm' NỐI trang sau vào cuối, dùng con trỏ của trang trước, và bỏ dòng trùng", () => {
    let list = afterLoad(FIRST_LIST, done([report("PA1"), report("PA2")], "c1"));
    expect(list.cursor).toBe("c1");
    expect(render(list)).toContain(MY_REPORTS.load_more_button);

    list = startLoading(list);
    expect(list.loading).toBe(true);
    // Đang tải thêm: danh sách cũ VẪN hiện, không nháy về trạng thái rỗng.
    expect(render(list)).toContain("PA1");
    expect(render(list)).toContain(MY_REPORTS.loading_more);

    list = afterLoad(list, done([report("PA2"), report("PA3")], ""));
    expect(list.entries.map((p) => p.lookup_code)).toEqual(["PA1", "PA2", "PA3"]);
    expect(list.has_more).toBe(false);
    const html = render(list);
    expect(html).not.toContain(MY_REPORTS.load_more_button);
    expect(html).toContain(MY_REPORTS.end_of_list);
    expect(html.indexOf("PA1")).toBeLessThan(html.indexOf("PA3"));
  });

  it("lỗi giữ nguyên danh sách đã có và mời Thử lại; hết phiên thì không mời", () => {
    const found = afterLoad(FIRST_LIST, done([report("PA1")], "c1"));
    const network = afterLoad(startLoading(found), { kind: "loi-mang" });
    expect(network.entries).toHaveLength(1);
    expect(network.cursor).toBe("c1");
    expect(render(network)).toContain(MY_REPORTS.network_error);
    expect(render(network)).toContain(MY_REPORTS.retry_button);
    expect(render(network)).toContain("PA1");

    const after_first = afterLoad(FIRST_LIST, { kind: "khong-hop-le" });
    expect(render(after_first)).toContain(MY_REPORTS.server_error);
    expect(render(after_first)).toContain(MY_REPORTS.retry_button);
    expect(render(after_first)).not.toContain(MY_REPORTS.empty);

    const expired = afterLoad(FIRST_LIST, { kind: "het-phien" });
    expect(render(expired)).toContain(MY_REPORTS.session_expired);
    expect(render(expired)).not.toContain(MY_REPORTS.retry_button);

    // Máy chủ nói chưa có phiên: màn đóng lại thành "kênh chưa mở".
    expect(afterLoad(FIRST_LIST, { kind: "chua-co-phien" }).channel_closed).toBe(true);
  });

  it("thẻ: mã to, trạng thái bằng một trong BỐN NHÓM, lĩnh vực, trích đoạn, mốc +07 — đủ chín trạng thái", () => {
    // ADR 0050 #5, chủ dự án 28/09/2026: app chung cũng hiện bốn nhóm cho người dân (không còn chín nhãn).
    const EXPECTED_GROUP: Record<string, string> = {
      "da-tiep-nhan": "Đã tiếp nhận",
      "dang-phan-loai": "Đã tiếp nhận",
      "da-chuyen-xu-ly": "Đang xử lý",
      "dang-xu-ly": "Đang xử lý",
      "da-xu-ly": "Đã xử lý xong",
      "cho-dan-xac-nhan": "Đã xử lý xong",
      "da-dong": "Đã đóng",
      "khong-tiep-nhan": "Đã đóng",
      "chuyen-cap-tren": "Đã đóng",
    };
    expect(Object.keys(STATUS).sort()).toEqual(Object.keys(EXPECTED_GROUP).sort());
    for (const [code, label] of Object.entries(EXPECTED_GROUP)) {
      const html = renderToStaticMarkup(
        createElement(ReportSummaryCard, { report: report("PA7K2QX9M4TD", code), onOpen: () => {} }),
      );
      expect(html, code).toContain(`<strong class="cd-the-cua-toi__trang-thai">${label}</strong>`);
      expect(html, code).not.toContain(code);
    }
    // Unknown code: the neutral sentence — never guessed into "Đã đóng" or any group, never the raw code.
    const unknown = renderToStaticMarkup(
      createElement(ReportSummaryCard, { report: report("PA7K2QX9M4TD", "trang-thai-moi"), onOpen: () => {} }),
    );
    expect(unknown).toContain(`<strong class="cd-the-cua-toi__trang-thai">${STATUS_UNLABELLED}</strong>`);
    expect(unknown).not.toContain("trang-thai-moi");

    const html = renderToStaticMarkup(
      createElement(ReportSummaryCard, {
        report: { ...report("PA7K2QX9M4TD"), field_label: "Giao thông", resolve_due: "2026-09-26T09:00:00Z" },
        onOpen: () => {},
      }),
    );
    expect(html).toContain('<span class="cd-the-cua-toi__ma">PA7K2QX9M4TD</span>');
    expect(html).toContain("Giao thông");
    expect(html).toContain("Đèn đường hỏng ở đầu ngõ");
    expect(html).toContain(`${REPORT_CARD.sent_at}: 24/09/2026 08:30`);
    expect(html).toContain(`${REPORT_CARD.due_at}: 26/09/2026 16:00`);
    // Mốc cố định, không đếm "quá hạn" ở client (ADR 0007).
    expect(html).not.toMatch(/quá hạn/i);
    // Cả thẻ là một nút.
    expect(html).toMatch(/^<li class="cd-cua-toi__muc"><button type="button" class="cd-the-cua-toi">/);
  });

  it("thẻ: chưa có hạn xử lý thì KHÔNG hiện dòng hạn; mã lĩnh vực thô không bao giờ hiện", () => {
    const html = renderToStaticMarkup(
      createElement(ReportSummaryCard, {
        report: { ...report("PA1"), field: "giao-thong", field_label: "" },
        onOpen: () => {},
      }),
    );
    expect(html).not.toContain(REPORT_CARD.due_at);
    expect(html).not.toContain("giao-thong");
    expect(html).toContain(REPORT_CARD.classified);
  });

  it("chạm thẻ gọi onOpen với đúng mã", () => {
    const open = vi.fn();
    const el = ReportSummaryCard({ report: report("PA9"), onOpen: open }) as unknown as {
      props: { children: { props: { onClick: () => void } } };
    };
    el.props.children.props.onClick();
    expect(open).toHaveBeenCalledWith("PA9");
  });

  it("mở tra cứu từ danh sách: mã điền sẵn vào ô", () => {
    const html = renderToStaticMarkup(
      createElement(ReportLookupScreen, { onBack: () => {}, initial_code: "PA7K2QX9M4TD" }),
    );
    expect(html).toMatch(/<input[^>]*value="PA7K2QX9M4TD"/);
  });
});
