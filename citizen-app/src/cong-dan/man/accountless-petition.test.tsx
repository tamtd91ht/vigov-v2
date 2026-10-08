import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * ACCOUNTLESS PETITIONS (ADR 0083, 08/10/2026, TEMPORARY) — the state half, without a DOM.
 *
 * Measured on a real phone: App ViHAT opened from a commune QR gets -1401 for `access-token` whether the name
 * card was accepted or declined, so no session of any kind can open. What must hold:
 *   · the gate offers the accountless path ONLY for that refusal, and only for sending and looking up;
 *   · a non-transient refusal is remembered for the open: the next act shows it at once, Zalo is not asked again;
 *   · the three calls carry NO bearer (even with a session on the phone), go only behind a well-formed domain,
 *     and the send's body carries exactly that domain;
 *   · the two 429 codes are two branches; every "not found" of the lookup is one branch (rule 4);
 *   · the readers are strict, and a refusal reason never shows on a petition still being handled.
 * The DOM-driven send (same form, no photos, same key on "Gửi lại") is in `hieu-ung-khong-phien.test.tsx`.
 */
vi.mock("../api/dia-chi-vigov", () => ({
  diaChiViGov: (_service: string, path: string) => `https://vigov.vidu.example${path}`,
}));

import {
  lookupAccountlessReport,
  publicReportFields,
  sendAccountlessReport,
} from "../api/goi-vigov";
import { accountlessReportBody, readAccountlessReceipt, readAccountlessReport } from "../api/hop-dong-phan-anh";
import { taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export, imported not renamed (rule 12 #3)
import type { OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { datPhienViGov } from "../api/phien-vigov"; // vi-name-ok: existing export, imported not renamed (rule 12 #3)

import { createSessionGate, offersAccountless, offersManualSend, type SessionGateState } from "./commune-session";
import { ACCOUNTLESS, LOI_GUI, PHONE_VERIFICATION, TRA_CUU, TYPED_CONTACT, XA_PA } from "./noi-dung";
import {
  AccountlessReportCard,
  accountlessLookupOutcome,
  accountlessSendBody,
  accountlessSendOutcome,
  AccountlessSendDone,
} from "./PhanAnhAppXa";
import { SessionGateScreen } from "./TrangXa";

const COMMUNE = "Xã Thử Nghiệm";
const DOMAIN = "xa-thu-nghiem.vigov.example";
const ACCESS = { capability: "access-token", code: -1401, transient: false } as const;
const ACCESS_TRANSIENT = { capability: "access-token", code: -1408, transient: true } as const;
const PHONE = { capability: "phone", code: -1401, transient: false } as const;
const settle = () => new Promise((r) => setTimeout(r, 0));

afterEach(() => {
  datPhienViGov(null);
  vi.unstubAllGlobals();
});

function harness(open: OpenCommuneAppSession) {
  const states: (SessionGateState | null)[] = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => states.push(s));
  return { gate, last: () => states[states.length - 1] };
}

/* ─────────────────────────────── the gate ─────────────────────────────── */

describe("gate — the accountless path only when Zalo refused the ACCESS TOKEN", () => {
  it("offered for sending and looking up under an access-token refusal; never for the phone step, a network failure, other acts", () => {
    expect(offersAccountless("thu-lai", "submit", ACCESS)).toBe(true);
    expect(offersAccountless("thu-lai", "lookup", ACCESS)).toBe(true);
    expect(offersAccountless("thu-lai", "submit", ACCESS_TRANSIENT)).toBe(true);
    expect(offersAccountless("thu-lai", "submit", PHONE)).toBe(false);
    expect(offersAccountless("thu-lai", "submit")).toBe(false);
    for (const task of ["mine", "rate", undefined] as const) expect(offersAccountless("thu-lai", task, ACCESS), task).toBe(false);
    for (const outcome of ["tu-choi", "ngoai-zalo", "khac-xa", "chua-ket-noi", "tam-ngung", "cho-lat", "chua-mo", "no-phone"] as const) {
      expect(offersAccountless(outcome, "submit", ACCESS), outcome).toBe(false);
    }
    // The typed-contact path of ADR 0080 stays where it was: not under an access-token refusal.
    expect(offersManualSend("thu-lai", "submit", ACCESS)).toBe(false);
  });

  it("agree → Zalo refuses the access token → the result carries Zalo's failure, and the act never runs", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => ({ kieu: "thu-lai", zalo: ACCESS }));
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run, "submit");
    await gate.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "thu-lai", zalo: ACCESS });
    expect(run).not.toHaveBeenCalled();
  });

  it("declined, then 'Gửi bằng họ tên…' fails the SAME way (the phone-less open needs the access token too) → offered", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => ({ kieu: "thu-lai", zalo: ACCESS }));
    const { gate, last } = harness(open);
    gate.require(() => {}, "submit");
    gate.decline();
    await gate.manual();
    expect(open).toHaveBeenCalledTimes(1);
    const s = last()!;
    expect(s.kieu === "ket-qua" && offersAccountless(s.outcome, "submit", s.zalo)).toBe(true);
  });

  it("a NON-transient refusal is remembered: the next act shows it at once and asks Zalo nothing", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => ({ kieu: "thu-lai", zalo: ACCESS }));
    const { gate, last } = harness(open);
    gate.require(() => {}, "submit");
    await gate.allow();
    gate.reset();
    gate.require(() => {}, "lookup");
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "thu-lai", zalo: ACCESS });
    gate.require(() => {}, "mine");
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "thu-lai", zalo: ACCESS });
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("a TRANSIENT refusal is not remembered: the next act asks again", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => ({ kieu: "thu-lai", zalo: ACCESS_TRANSIENT }));
    const { gate, last } = harness(open);
    gate.require(() => {}, "submit");
    await gate.allow();
    gate.reset();
    gate.require(() => {}, "submit");
    expect(last()).toEqual({ kieu: "hoi" });
  });

  it("a session that opens later still runs acts normally (the memory only stands while there is no session)", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => ({ kieu: "thu-lai", zalo: ACCESS }));
    const { gate } = harness(open);
    gate.require(() => {}, "submit");
    await gate.allow();
    datPhienViGov({ token: "t", ten_xa: COMMUNE, phone_verified: true });
    const run = vi.fn();
    gate.require(run, "mine");
    expect(run).toHaveBeenCalledTimes(1);
  });
});

/* ─────────────────────────────── the gate's screen ─────────────────────────────── */

function gateScreen(state: SessionGateState, task: "submit" | "lookup" | "mine", withAccountless = true) {
  return renderToStaticMarkup(
    createElement(SessionGateScreen, {
      state,
      task,
      onAllow() {},
      onDecline() {},
      onManual() {},
      ...(withAccountless ? { onAccountless() {} } : {}),
      onClose() {},
    }),
  );
}

describe("SessionGateScreen — the offer, its cost said above the button, and no code in the sentence", () => {
  const refused: SessionGateState = { kieu: "ket-qua", outcome: "thu-lai", zalo: ACCESS };

  it("sending: the cost (no photos, no notification, keep the code) BEFORE the button; Zalo's code on its own line", () => {
    const html = gateScreen(refused, "submit");
    expect(html).toContain(ACCOUNTLESS.offer);
    expect(html).toContain(`>${ACCOUNTLESS.send_button}</button>`);
    expect(html.indexOf(ACCOUNTLESS.offer)).toBeLessThan(html.indexOf(ACCOUNTLESS.send_button));
    expect(ACCOUNTLESS.offer).toContain("KHÔNG nhận được thông báo");
    expect(html).not.toContain(`>${TYPED_CONTACT.button}</button>`);
    // No retry that cannot help (non-transient), and the way home is always there.
    expect(html).not.toContain(`>${PHONE_VERIFICATION.allow}</button>`);
  });

  it("looking up: the lookup offer and button", () => {
    const html = gateScreen(refused, "lookup");
    expect(html).toContain(ACCOUNTLESS.lookup_offer);
    expect(html).toContain(`>${ACCOUNTLESS.lookup_button}</button>`);
  });

  it("not under a phone-step refusal, not for 'Phản ánh của tôi', not without the handler", () => {
    expect(gateScreen({ kieu: "ket-qua", outcome: "thu-lai", zalo: PHONE }, "submit")).not.toContain(ACCOUNTLESS.send_button);
    expect(gateScreen(refused, "mine")).not.toContain(ACCOUNTLESS.send_button);
    expect(gateScreen(refused, "submit", false)).not.toContain(ACCOUNTLESS.send_button);
  });

  it("no wording of this path carries an error code", () => {
    for (const text of [...Object.values(ACCOUNTLESS), LOI_GUI["rate-limited"].cau, LOI_GUI["commune-daily-limit"].cau]) {
      expect(text).not.toMatch(/-?\d{3,}|rate_limited|commune_daily_limit|commune_not_found/);
    }
  });
});

/* ─────────────────────────────── the client ─────────────────────────────── */

type Seen = { url: string; init: { method: string; headers: Record<string, string>; body?: string } };

function stubFetch(status: number, body: unknown): Seen[] {
  const seen: Seen[] = [];
  vi.stubGlobal("fetch", async (url: string, init: Seen["init"]) => {
    seen.push({ url, init });
    return { status, ok: status < 300, json: async () => body };
  });
  return seen;
}

const FORM = {
  linh_vuc: "rac-thai",
  noi_dung: "Rác tồn đọng",
  dia_chi: "Ngõ 12",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: true,
};

describe("client — no bearer, a domain gate, and the body's domain is the gate's", () => {
  it("the body is the session body + `host`, anonymous forced off, and NEVER a location on this path", () => {
    const body = JSON.parse(accountlessSendBody(FORM, DOMAIN)) as Record<string, unknown>;
    expect(body).toEqual({
      content: "Rác tồn đọng",
      address: "Ngõ 12",
      reporter_name: "Nguyễn Văn An",
      reporter_phone: "0900000000",
      anonymous: false,
      field: "rac-thai",
      host: DOMAIN,
    });
    // Even a caller that hands the contract a valid pair gets no `lat`/`lng` out (owner, 08/10/2026).
    const withPair = JSON.parse(
      accountlessReportBody(
        { noi_dung: "a", dia_chi: "b", ho_ten: "c", dien_thoai: "0900000000", an_danh: false, scene_location: { lat: 10.5, lng: 106.7 } },
        DOMAIN,
      ),
    ) as Record<string, unknown>;
    expect(withPair).not.toHaveProperty("lat");
    expect(withPair).not.toHaveProperty("lng");
  });

  it("send: POST to the public route with the key, NO Authorization even with a session on the phone; 201 read", async () => {
    datPhienViGov({ token: "tok-must-not-leave", ten_xa: COMMUNE, phone_verified: true });
    const seen = stubFetch(201, { code: "PB9M3K7Q2XTD", status: "da-tiep-nhan", acknowledge_due: "2026-10-08T03:00:00Z", resolve_due: null });
    const attempt = taoLanGui(accountlessSendBody(FORM, DOMAIN));
    expect(await sendAccountlessReport(DOMAIN, attempt)).toEqual({
      kieu: "xong",
      receipt: { code: "PB9M3K7Q2XTD", status: "da-tiep-nhan", acknowledgeDue: "2026-10-08T03:00:00Z", resolveDue: null },
    });
    expect(seen).toHaveLength(1);
    expect(seen[0]!.url).toBe("https://vigov.vidu.example/api/v1/public-citizen-reports");
    expect(seen[0]!.init.method).toBe("POST");
    expect(seen[0]!.init.headers["Authorization"]).toBeUndefined();
    expect(seen[0]!.init.headers["Idempotency-Key"]).toBe(attempt.khoa);
    expect(attempt.khoa).toMatch(/^[0-9a-f]{32}$/);
    expect(seen[0]!.init.body).toBe(attempt.than);
  });

  it("send: nothing leaves without a well-formed domain, or when the body names another domain", async () => {
    const seen = stubFetch(201, {});
    const attempt = taoLanGui(accountlessSendBody(FORM, DOMAIN));
    expect(await sendAccountlessReport("", attempt)).toEqual({ kieu: "chua-cau-hinh" });
    expect(await sendAccountlessReport("not a domain", attempt)).toEqual({ kieu: "chua-cau-hinh" });
    expect(await sendAccountlessReport("xa-khac.vigov.example", attempt)).toEqual({ kieu: "chua-cau-hinh" });
    expect(await sendAccountlessReport(DOMAIN, taoLanGui(JSON.stringify({ content: "x" })))).toEqual({ kieu: "chua-cau-hinh" });
    expect(seen).toEqual([]);
  });

  it.each<[number, unknown, string]>([
    [429, { code: "rate_limited" }, "rate-limited"],
    [429, { code: "commune_daily_limit" }, "commune-daily-limit"],
    [429, null, "rate-limited"],
    [404, { code: "commune_not_found" }, "khong-thay"],
    [400, { code: "field_not_offered" }, "field-not-offered"],
    [503, { code: "field_catalogue_unavailable" }, "field-catalogue-unavailable"],
    [503, {}, "kenh-chua-mo"],
    [409, {}, "dang-xu-ly-truoc"],
    [401, {}, "loi-may-chu"],
    [201, { code: "", status: "da-tiep-nhan", acknowledge_due: null, resolve_due: null }, "loi-may-chu"],
  ])("send: %i %j → %s", async (status, body, kieu) => {
    stubFetch(status, body);
    expect((await sendAccountlessReport(DOMAIN, taoLanGui(accountlessSendBody(FORM, DOMAIN)))).kieu).toBe(kieu);
  });

  it("send outcomes on screen: 404 and 'not sent' point to the commune; the 429s keep their own sentences", () => {
    expect(accountlessSendOutcome({ kieu: "khong-thay" })).toEqual({ kind: "failed", failure: "kenh-chua-mo" });
    expect(accountlessSendOutcome({ kieu: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "kenh-chua-mo" });
    expect(accountlessSendOutcome({ kieu: "rate-limited" })).toEqual({ kind: "failed", failure: "rate-limited" });
    expect(accountlessSendOutcome({ kieu: "commune-daily-limit" })).toEqual({ kind: "failed", failure: "commune-daily-limit" });
    expect(LOI_GUI["rate-limited"].cau).toContain("Bộ phận tiếp nhận của Ủy ban nhân dân xã");
  });

  it("field list: GET by domain, no bearer; a stray 401 is a server failure, never a session loop", async () => {
    datPhienViGov({ token: "tok-must-not-leave", ten_xa: COMMUNE, phone_verified: true });
    const seen = stubFetch(200, { items: [{ code: "rac-thai", label: "Rác thải", icon: null, tone: null }] });
    expect(await publicReportFields(DOMAIN)).toEqual({
      kieu: "xong",
      fields: [{ code: "rac-thai", label: "Rác thải", icon: null, tone: null }],
    });
    expect(seen[0]!.url).toBe(`https://vigov.vidu.example/api/v1/public-citizen-report-fields?host=${DOMAIN}`);
    expect(seen[0]!.init.headers["Authorization"]).toBeUndefined();
    stubFetch(401, {});
    expect(await publicReportFields(DOMAIN)).toEqual({ kieu: "loi-may-chu" });
    expect(await publicReportFields("x")).toEqual({ kieu: "chua-cau-hinh" });
  });

  it("lookup: GET by code and domain, no bearer; 404 one branch; 429 its own; empty code sends nothing", async () => {
    const seen = stubFetch(200, {
      code: "PB9M3K7Q2XTD",
      status: "da-xu-ly",
      acknowledge_due: null,
      resolve_due: "2026-10-10T03:00:00Z",
      result: "Đã thu gom.",
      reason: "",
    });
    const found = await lookupAccountlessReport(DOMAIN, " PB9M3K7Q2XTD ");
    expect(found.kieu).toBe("xong");
    expect(seen[0]!.url).toBe(`https://vigov.vidu.example/api/v1/public-citizen-reports/PB9M3K7Q2XTD?host=${DOMAIN}`);
    expect(seen[0]!.init.headers["Authorization"]).toBeUndefined();
    stubFetch(404, {});
    expect(await lookupAccountlessReport(DOMAIN, "SAI")).toEqual({ kieu: "khong-thay" });
    stubFetch(429, { code: "rate_limited" });
    expect(await lookupAccountlessReport(DOMAIN, "X")).toEqual({ kieu: "rate-limited" });
    const none = stubFetch(200, {});
    expect(await lookupAccountlessReport(DOMAIN, "  ")).toEqual({ kieu: "khong-thay" });
    expect(none).toEqual([]);
  });

  it("lookup outcomes are sentences without codes; one sentence for every 'not found'", () => {
    expect(accountlessLookupOutcome({ kieu: "khong-thay" })).toEqual({ kind: "failed", text: XA_PA.khong_thay_phieu });
    expect(accountlessLookupOutcome({ kieu: "rate-limited" })).toEqual({ kind: "failed", text: ACCOUNTLESS.lookup_rate_limited });
    expect(accountlessLookupOutcome({ kieu: "loi-mang" })).toEqual({ kind: "failed", text: TRA_CUU.loi_mang });
  });
});

/* ─────────────────────────────── the readers and the two views ─────────────────────────────── */

describe("readers — strict, and the lookup never carries more than ADR 0083 #4 allows", () => {
  const base = { code: "PB9M3K7Q2XTD", status: "da-tiep-nhan", acknowledge_due: null, resolve_due: null };

  it("receipt: every key present and typed; a missing deadline key is malformed (null is not absent)", () => {
    expect(readAccountlessReceipt(base)).toEqual({ code: base.code, status: base.status, acknowledgeDue: null, resolveDue: null });
    expect(readAccountlessReceipt({ ...base, code: 7 })).toBeNull();
    expect(readAccountlessReceipt({ code: base.code, status: base.status, resolve_due: null })).toBeNull();
    expect(readAccountlessReceipt({ ...base, acknowledge_due: 3 })).toBeNull();
    expect(readAccountlessReceipt(null)).toBeNull();
  });

  it("lookup: `result` required; `reason` optional and dropped outside the two terminal branches", () => {
    expect(readAccountlessReport(base)).toBeNull();
    expect(readAccountlessReport({ ...base, result: "", reason: "Lý do lạc" })).toMatchObject({ reason: "" });
    expect(readAccountlessReport({ ...base, status: "khong-tiep-nhan", result: "", reason: "Ngoài địa bàn" })).toMatchObject({
      reason: "Ngoài địa bàn",
    });
    expect(readAccountlessReport({ ...base, result: "" })).toMatchObject({ reason: "" });
    expect(readAccountlessReport({ ...base, result: "", reason: "x".repeat(2001) })).toBeNull();
    // Extra keys a server might add are not read into the type at all.
    expect(Object.keys(readAccountlessReport({ ...base, result: "", reporter_name: "X" })!).sort()).toEqual(
      ["acknowledgeDue", "code", "reason", "resolveDue", "result", "status"],
    );
  });

  it("the done screen: the code, the no-notification sentence, the lookup button; stored deadlines only", () => {
    const html = renderToStaticMarkup(
      createElement(AccountlessSendDone, {
        receipt: { code: "PB9M3K7Q2XTD", status: "da-tiep-nhan", acknowledgeDue: null, resolveDue: null },
        onFollow() {},
        onHome() {},
      }),
    );
    expect(html).toContain("#PB9M3K7Q2XTD");
    expect(html).toContain(ACCOUNTLESS.done_notice);
    expect(html).toContain(`>${ACCOUNTLESS.follow}</button>`);
    expect(html).not.toContain("xa-deadline-band");
  });

  it("the lookup card: status in words, the commune's result, the scope sentence", () => {
    const html = renderToStaticMarkup(
      createElement(AccountlessReportCard, {
        report: {
          code: "PB9M3K7Q2XTD",
          status: "da-xu-ly",
          acknowledgeDue: null,
          resolveDue: null,
          result: "Đã thu gom rác.",
          reason: "",
        },
      }),
    );
    expect(html).toContain("Đã thu gom rác.");
    expect(html).toContain(ACCOUNTLESS.lookup_scope);
    expect(html).toContain("xa-chip-tt");
  });
});

/** The body builder the dossier test also measures — kept honest from this side too. */
describe("contract — the accountless body never drops a session key", () => {
  it("same keys as the session body plus `host`", () => {
    const pa = { noi_dung: "a", dia_chi: "b", ho_ten: "c", dien_thoai: "0900000000", an_danh: false };
    const keys = Object.keys(JSON.parse(accountlessReportBody(pa, DOMAIN)) as Record<string, unknown>);
    expect(keys).toEqual(["content", "address", "reporter_name", "reporter_phone", "anonymous", "host"]);
  });
});
