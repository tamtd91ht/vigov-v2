/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE COMMUNE APP ON THE REAL REGISTER (W6b, 29/09/2026) — what the screens send and how they read every
 * answer. A fake session and host for THIS FILE ONLY, exactly as `citizen-report-rating.test.tsx` does: product
 * code has one session source (`api/vigov-session.ts`).
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-test", commune_name: "Xã Thử Nghiệm" } as { token: string; commune_name: string } | null,
}));

vi.mock("../api/vigov-session", () => ({ getVigovSession: () => state.session, setVigovSession: () => {} }));
vi.mock("../api/vigov-address", () => ({
  vigovAddress: (_service: "petitions", path: string) => `https://vigov.vidu.vn${path}`,
}));

import { citizenReportFields, submitReport, type CallResult } from "../api/vigov-client";
import { readCitizenFields } from "../api/citizen-report-contract";
import { createSendAttempt } from "../api/send-attempt";

import { MY_REPORTS, CHANNEL_NOT_OPEN, SEND_ERROR, LOOKUP, COMMUNE_APP_REPORTS } from "./copy";
import {
  catalogueOutcome,
  countLabel,
  fieldIcon,
  FieldStep,
  fieldTone,
  listFailureText,
  listOutcome,
  lookupOutcome,
  normaliseCode,
  CommuneReportList,
  sendBody,
  sendOutcome,
} from "./CommuneAppReports";
import type { ReportDraft } from "./commune-app-model";

const FORM: ReportDraft = {
  linh_vuc: "dien",
  noi_dung: "  Đèn đường hỏng đầu ngõ ",
  dia_chi: " Ngõ 12 ",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];

beforeEach(() => {
  calls = [];
  state.session = { token: "tok-test", commune_name: "Xã Thử Nghiệm" };
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("send body — the contract's fields, `field` = the picked CODE, lat/lng only when tapped", () => {
  it("five fields trimmed, plus the field code picked from the commune's catalogue", () => {
    const body = JSON.parse(sendBody(FORM, null)) as Record<string, unknown>;
    expect(body).toEqual({
      content: "Đèn đường hỏng đầu ngõ",
      address: "Ngõ 12",
      reporter_name: "Nguyễn Văn An",
      reporter_phone: "0900000000",
      anonymous: false,
      field: "dien",
    });
  });

  it("no field picked → no `field` key at all (never an empty string)", () => {
    const body = JSON.parse(sendBody({ ...FORM, linh_vuc: "" }, null)) as Record<string, unknown>;
    expect(body).not.toHaveProperty("field");
  });

  it("anonymous: name and phone are not sent", () => {
    const body = JSON.parse(sendBody({ ...FORM, an_danh: true }, null)) as Record<string, unknown>;
    expect(body.reporter_name).toBe("");
    expect(body.reporter_phone).toBe("");
    expect(body.anonymous).toBe(true);
  });

  it("location from the tap → lat/lng; none → neither key", () => {
    const body = JSON.parse(sendBody(FORM, { lat: 15.5, lng: 108.4 })) as Record<string, unknown>;
    expect(body.lat).toBe(15.5);
    expect(body.lng).toBe(108.4);
  });

  it("goes out through the shared client: bearer from the session, one Idempotency-Key, no identity in the body", async () => {
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve({ status: 503, ok: false, json: async () => ({}) });
    });
    const attempt = createSendAttempt(sendBody(FORM, null));
    await submitReport(attempt);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe("https://vigov.vidu.vn/api/v1/my-citizen-reports");
    const headers = calls[0]!.init.headers as Record<string, string>;
    expect(headers["Authorization"]).toBe("Bearer tok-test");
    expect(headers["Idempotency-Key"]).toBe(attempt.key);
    const body = JSON.parse(calls[0]!.init.body as string) as Record<string, unknown>;
    for (const k of ["citizen_id", "tenant_id", "phone", "commune"]) expect(body).not.toHaveProperty(k);
  });

  it("no session → nothing leaves the phone", async () => {
    state.session = null;
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    expect(await submitReport(createSendAttempt(sendBody(FORM, null)))).toEqual({ kind: "chua-co-phien" });
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("reading every answer — one branch per next step", () => {
  const report = { lookup_code: "PA-1" } as Extract<CallResult, { kind: "xong" }>["report"];

  it("send: 201 → sent; 401/403-phone/no session → the gate again (same attempt); others → the shared sentences", () => {
    expect(sendOutcome({ kind: "xong", report })).toEqual({ kind: "sent", report });
    for (const kind of ["het-phien", "can-xac-thuc-so", "chua-co-phien"] as const) {
      expect(sendOutcome({ kind })).toEqual({ kind: "session" });
    }
    expect(sendOutcome({ kind: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "kenh-chua-mo" });
    expect(sendOutcome({ kind: "khong-thay" })).toEqual({ kind: "failed", failure: "loi-may-chu" });
    expect(sendOutcome({ kind: "loi-mang" })).toEqual({ kind: "failed", failure: "loi-mang" });
    expect(sendOutcome({ kind: "dang-xu-ly-truoc" })).toEqual({ kind: "failed", failure: "dang-xu-ly-truoc" });
  });

  it("lookup: 404 is ONE sentence (no telling 'someone else's' apart); session loss goes to the gate", () => {
    expect(lookupOutcome({ kind: "xong", report })).toEqual({ kind: "found", report });
    expect(lookupOutcome({ kind: "khong-thay" })).toEqual({ kind: "failed", text: COMMUNE_APP_REPORTS.report_not_found });
    expect(lookupOutcome({ kind: "het-phien" })).toEqual({ kind: "session" });
    expect(lookupOutcome({ kind: "can-xac-thuc-so" })).toEqual({ kind: "session" });
    expect(lookupOutcome({ kind: "loi-mang" })).toEqual({ kind: "failed", text: LOOKUP.network_error });
    expect(lookupOutcome({ kind: "loi-may-chu" })).toEqual({ kind: "failed", text: LOOKUP.server_error });
    expect(lookupOutcome({ kind: "chua-cau-hinh" })).toEqual({ kind: "failed", text: CHANNEL_NOT_OPEN.text });
  });

  it("list: page, session loss, network, closed channel, anything else", () => {
    const page = { kind: "xong" as const, page: { entries: [], cursor: "", has_more: false } };
    expect(listOutcome(page)).toBe(page);
    expect(listOutcome({ kind: "het-phien" })).toBe("session");
    expect(listOutcome({ kind: "loi-mang" })).toBe("network");
    expect(listOutcome({ kind: "chua-cau-hinh" })).toBe("closed");
    expect(listOutcome({ kind: "khong-hop-le" })).toBe("server");
    expect(listFailureText("network")).toBe(MY_REPORTS.network_error);
    expect(listFailureText("closed")).toBe(CHANNEL_NOT_OPEN.text);
  });

  it("the typed code: trimmed, a leading '#' (as cards show it) removed", () => {
    expect(normaliseCode("  #PA-ABC ")).toBe("PA-ABC");
    expect(normaliseCode("   ")).toBe("");
  });

  it("counts over loaded pages are a floor when more pages exist", () => {
    expect(countLabel(3, false)).toBe("3");
    expect(countLabel(20, true)).toBe("20+");
  });
});

describe("the commune's field catalogue (step 1) — no built-in list, ever", () => {
  const ITEM = { code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", icon: "Trash2", tone: "orange" };

  function stub(status: number, body: unknown) {
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve({ status, ok: status >= 200 && status < 300, json: async () => body });
    });
  }

  it("reader: code/label required; icon/tone null when absent or empty; one bad row → malformed", () => {
    expect(readCitizenFields({ items: [ITEM, { code: "khac", label: "Khác", icon: null, tone: "" }] })).toEqual([
      ITEM,
      { code: "khac", label: "Khác", icon: null, tone: null },
    ]);
    expect(readCitizenFields({ items: [] })).toEqual([]);
    expect(readCitizenFields({ items: [{ ...ITEM, code: "" }] })).toBeNull();
    expect(readCitizenFields({ items: [{ ...ITEM, label: " " }] })).toBeNull();
    expect(readCitizenFields({ items: [{ ...ITEM, tone: 3 }] })).toBeNull();
    expect(readCitizenFields({})).toBeNull();
  });

  it("GET with the session's bearer, no query, no body", async () => {
    stub(200, { items: [ITEM] });
    expect(await citizenReportFields()).toEqual({ kind: "xong", fields: [ITEM] });
    expect(calls[0]!.url).toBe("https://vigov.vidu.vn/api/v1/my-citizen-report-fields");
    expect(calls[0]!.init.method).toBe("GET");
    expect((calls[0]!.init.headers as Record<string, string>)["Authorization"]).toBe("Bearer tok-test");
    expect(calls[0]!.init.body).toBeUndefined();
  });

  it("503 field_catalogue_unavailable is its own branch; another 503 is not", async () => {
    stub(503, { code: "field_catalogue_unavailable", message: "never read" });
    expect(await citizenReportFields()).toEqual({ kind: "field-catalogue-unavailable" });
    stub(503, { code: "intake_not_configured" });
    expect(await citizenReportFields()).toEqual({ kind: "kenh-chua-mo" });
  });

  it("no session → nothing leaves the phone", async () => {
    state.session = null;
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    expect(await citizenReportFields()).toEqual({ kind: "chua-co-phien" });
    expect(spy).not.toHaveBeenCalled();
  });

  it("outcome table: ready, gate again, unavailable, network, closed, anything else", () => {
    expect(catalogueOutcome({ kind: "xong", fields: [ITEM] })).toEqual({ kind: "ready", fields: [ITEM] });
    expect(catalogueOutcome({ kind: "het-phien" })).toBe("session");
    expect(catalogueOutcome({ kind: "field-catalogue-unavailable" })).toEqual({ kind: "failed", failure: "unavailable" });
    expect(catalogueOutcome({ kind: "loi-mang" })).toEqual({ kind: "failed", failure: "network" });
    expect(catalogueOutcome({ kind: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "closed" });
    expect(catalogueOutcome({ kind: "khong-hop-le" })).toEqual({ kind: "failed", failure: "server" });
  });

  const step = (catalogue: Parameters<typeof FieldStep>[0]["catalogue"], fieldChanged = false) =>
    renderToStaticMarkup(
      createElement(FieldStep, { catalogue, picked: "", fieldChanged, onPick: () => {}, onRetry: () => {} }),
    );

  it("step 1 shows the commune's labels, in its order, with its tone and a drawable icon (neutral otherwise)", () => {
    const html = step({
      kind: "ready",
      fields: [ITEM, { code: "an-ninh", label: "An ninh trật tự", icon: "ShieldAlert", tone: null }],
    });
    expect(html.indexOf(ITEM.label)).toBeLessThan(html.indexOf("An ninh trật tự"));
    expect(html).toContain("xa-mau--cam");
    expect(html).toContain("xa-mau--navy");
    expect(html).not.toContain("rac-thai"); // the code is sent, never shown
    expect(fieldIcon("ShieldAlert")).toBe("shield");
    expect(fieldIcon("Trash2")).toBe("text");
    expect(fieldIcon(null)).toBe("text");
    expect(fieldTone("red")).toBe("hong");
    expect(fieldTone("magenta")).toBe("navy");
  });

  it("loading, unavailable (with Thử lại), empty — words, and no field to pick", () => {
    expect(step({ kind: "loading" })).toContain(COMMUNE_APP_REPORTS.fields_loading);
    const failed = step({ kind: "failed", failure: "unavailable" });
    expect(failed).toContain(COMMUNE_APP_REPORTS.fields_unavailable);
    expect(failed).toContain(MY_REPORTS.retry_button);
    expect(failed).not.toContain('role="radio"');
    expect(step({ kind: "ready", fields: [] })).toContain(COMMUNE_APP_REPORTS.fields_empty);
  });

  it("after 400 field_not_offered the step says why before the list", async () => {
    stub(400, { code: "field_not_offered" });
    expect(await submitReport(createSendAttempt(sendBody(FORM, null)))).toEqual({ kind: "field-not-offered" });
    stub(400, { code: "invalid_request" });
    expect(await submitReport(createSendAttempt(sendBody(FORM, null)))).toEqual({ kind: "khong-hop-le" });
    expect(sendOutcome({ kind: "field-not-offered" })).toEqual({ kind: "failed", failure: "field-not-offered" });
    expect(step({ kind: "ready", fields: [ITEM] }, true)).toContain(SEND_ERROR["field-not-offered"].text);
  });

  it("503 field_catalogue_unavailable on submit: nothing recorded, 'Gửi lại' with the same attempt", async () => {
    stub(503, { code: "field_catalogue_unavailable" });
    expect(await submitReport(createSendAttempt(sendBody(FORM, null)))).toEqual({ kind: "field-catalogue-unavailable" });
    expect(SEND_ERROR["field-catalogue-unavailable"].can_resend).toBe(true);
  });
});

describe("the list before any session — it asks, it never fetches", () => {
  it("idle: the card explaining why, and a button — no row, no number", () => {
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    const html = renderToStaticMarkup(
      createElement(CommuneReportList, {
        state: { kind: "idle" },
        onOpenReport: () => {},
        onLookup: () => {},
        onOpen: () => {},
        onRetry: () => {},
        onLoadMore: () => {},
      }),
    );
    expect(html).toContain(COMMUNE_APP_REPORTS.need_session_body);
    expect(html).toContain(COMMUNE_APP_REPORTS.need_session_button);
    expect(html).not.toContain("(0)");
    expect(spy).not.toHaveBeenCalled();
  });
});
