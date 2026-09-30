/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE COMMUNE APP ON THE REAL REGISTER (W6b, 29/09/2026) — what the screens send and how they read every
 * answer. A fake session and host for THIS FILE ONLY, exactly as `petition-rating.test.tsx` does: product
 * code has one session source (`api/phien-vigov.ts`).
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-test", ten_xa: "Xã Thử Nghiệm" } as { token: string; ten_xa: string } | null,
}));

vi.mock("../api/phien-vigov", () => ({ layPhienViGov: () => state.session, datPhienViGov: () => {} }));
vi.mock("../api/dia-chi-vigov", () => ({
  diaChiViGov: (_service: "petitions", path: string) => `https://vigov.vidu.vn${path}`,
}));

import { citizenReportFields, guiPhanAnh, type KetQuaGoi } from "../api/goi-vigov"; // vi-name-ok: existing exports
import { readCitizenFields } from "../api/hop-dong-phan-anh";
import { taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export

import { CUA_TOI, KENH_CHUA_MO, LOI_GUI, TRA_CUU, XA_PA } from "./noi-dung";
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
  PetitionList,
  sendBody,
  sendOutcome,
} from "./PhanAnhAppXa";
import type { NhapPhieu } from "./trai-nghiem";

const FORM: NhapPhieu = {
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
  state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" };
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
    const attempt = taoLanGui(sendBody(FORM, null));
    await guiPhanAnh(attempt);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe("https://vigov.vidu.vn/api/v1/my-citizen-reports");
    const headers = calls[0]!.init.headers as Record<string, string>;
    expect(headers["Authorization"]).toBe("Bearer tok-test");
    expect(headers["Idempotency-Key"]).toBe(attempt.khoa);
    const body = JSON.parse(calls[0]!.init.body as string) as Record<string, unknown>;
    for (const k of ["citizen_id", "tenant_id", "phone", "commune"]) expect(body).not.toHaveProperty(k);
  });

  it("no session → nothing leaves the phone", async () => {
    state.session = null;
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    expect(await guiPhanAnh(taoLanGui(sendBody(FORM, null)))).toEqual({ kieu: "chua-co-phien" });
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("reading every answer — one branch per next step", () => {
  const phieu = { ma_tra_cuu: "PA-1" } as Extract<KetQuaGoi, { kieu: "xong" }>["phieu"];

  it("send: 201 → sent; 401/403-phone/no session → the gate again (same attempt); others → the shared sentences", () => {
    expect(sendOutcome({ kieu: "xong", phieu })).toEqual({ kind: "sent", petition: phieu });
    for (const kieu of ["het-phien", "can-xac-thuc-so", "chua-co-phien"] as const) {
      expect(sendOutcome({ kieu })).toEqual({ kind: "session" });
    }
    expect(sendOutcome({ kieu: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "kenh-chua-mo" });
    expect(sendOutcome({ kieu: "khong-thay" })).toEqual({ kind: "failed", failure: "loi-may-chu" });
    expect(sendOutcome({ kieu: "loi-mang" })).toEqual({ kind: "failed", failure: "loi-mang" });
    expect(sendOutcome({ kieu: "dang-xu-ly-truoc" })).toEqual({ kind: "failed", failure: "dang-xu-ly-truoc" });
  });

  it("lookup: 404 is ONE sentence (no telling 'someone else's' apart); session loss goes to the gate", () => {
    expect(lookupOutcome({ kieu: "xong", phieu })).toEqual({ kind: "found", petition: phieu });
    expect(lookupOutcome({ kieu: "khong-thay" })).toEqual({ kind: "failed", text: XA_PA.khong_thay_phieu });
    expect(lookupOutcome({ kieu: "het-phien" })).toEqual({ kind: "session" });
    expect(lookupOutcome({ kieu: "can-xac-thuc-so" })).toEqual({ kind: "session" });
    expect(lookupOutcome({ kieu: "loi-mang" })).toEqual({ kind: "failed", text: TRA_CUU.loi_mang });
    expect(lookupOutcome({ kieu: "loi-may-chu" })).toEqual({ kind: "failed", text: TRA_CUU.loi_may_chu });
    expect(lookupOutcome({ kieu: "chua-cau-hinh" })).toEqual({ kind: "failed", text: KENH_CHUA_MO.cau });
  });

  it("list: page, session loss, network, closed channel, anything else", () => {
    const page = { kieu: "xong" as const, trang: { muc: [], con_tro: "", con_nua: false } };
    expect(listOutcome(page)).toBe(page);
    expect(listOutcome({ kieu: "het-phien" })).toBe("session");
    expect(listOutcome({ kieu: "loi-mang" })).toBe("network");
    expect(listOutcome({ kieu: "chua-cau-hinh" })).toBe("closed");
    expect(listOutcome({ kieu: "khong-hop-le" })).toBe("server");
    expect(listFailureText("network")).toBe(CUA_TOI.loi_mang);
    expect(listFailureText("closed")).toBe(KENH_CHUA_MO.cau);
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
    expect(await citizenReportFields()).toEqual({ kieu: "xong", fields: [ITEM] });
    expect(calls[0]!.url).toBe("https://vigov.vidu.vn/api/v1/my-citizen-report-fields");
    expect(calls[0]!.init.method).toBe("GET");
    expect((calls[0]!.init.headers as Record<string, string>)["Authorization"]).toBe("Bearer tok-test");
    expect(calls[0]!.init.body).toBeUndefined();
  });

  it("503 field_catalogue_unavailable is its own branch; another 503 is not", async () => {
    stub(503, { code: "field_catalogue_unavailable", message: "never read" });
    expect(await citizenReportFields()).toEqual({ kieu: "field-catalogue-unavailable" });
    stub(503, { code: "intake_not_configured" });
    expect(await citizenReportFields()).toEqual({ kieu: "kenh-chua-mo" });
  });

  it("no session → nothing leaves the phone", async () => {
    state.session = null;
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    expect(await citizenReportFields()).toEqual({ kieu: "chua-co-phien" });
    expect(spy).not.toHaveBeenCalled();
  });

  it("outcome table: ready, gate again, unavailable, network, closed, anything else", () => {
    expect(catalogueOutcome({ kieu: "xong", fields: [ITEM] })).toEqual({ kind: "ready", fields: [ITEM] });
    expect(catalogueOutcome({ kieu: "het-phien" })).toBe("session");
    expect(catalogueOutcome({ kieu: "field-catalogue-unavailable" })).toEqual({ kind: "failed", failure: "unavailable" });
    expect(catalogueOutcome({ kieu: "loi-mang" })).toEqual({ kind: "failed", failure: "network" });
    expect(catalogueOutcome({ kieu: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "closed" });
    expect(catalogueOutcome({ kieu: "khong-hop-le" })).toEqual({ kind: "failed", failure: "server" });
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
    // Wave 1: the table covers the sixteen lucide names of `PROTOTYPE.md` §8, each drawn as the SAME shape;
    // anything else — absent, or a name added to the platform later — gets §8's neutral fallback, never a guess.
    expect(fieldIcon("ShieldAlert")).toBe("shield-alert");
    expect(fieldIcon("Trash2")).toBe("trash");
    const platformIcons = ["Trash2", "TrafficCone", "Droplets", "Zap", "Construction", "ShieldAlert", "Hammer", "Factory", "Stethoscope", "UserRoundX", "Utensils", "MessageSquare"];
    for (const icon of platformIcons) expect(fieldIcon(icon), icon).not.toBe("message-square-plus");
    expect(fieldIcon(null)).toBe("message-square-plus");
    expect(fieldIcon("NameAddedLater")).toBe("message-square-plus");
    expect(fieldIcon("toString")).toBe("message-square-plus");
    // One class per platform tone since 30/09/2026: red is the prototype's red tone (pink is gone), cyan its own.
    expect(fieldTone("red")).toBe("red");
    expect(fieldTone("cyan")).toBe("cyan");
    expect(fieldTone("blue")).toBe("xanh");
    expect(fieldTone("magenta")).toBe("navy");
  });

  it("loading, unavailable (with Thử lại), empty — words, and no field to pick", () => {
    expect(step({ kind: "loading" })).toContain(XA_PA.fields_loading);
    const failed = step({ kind: "failed", failure: "unavailable" });
    expect(failed).toContain(XA_PA.fields_unavailable);
    expect(failed).toContain(CUA_TOI.nut_thu_lai);
    expect(failed).not.toContain('role="radio"');
    expect(step({ kind: "ready", fields: [] })).toContain(XA_PA.fields_empty);
  });

  it("after 400 field_not_offered the step says why before the list", async () => {
    stub(400, { code: "field_not_offered" });
    expect(await guiPhanAnh(taoLanGui(sendBody(FORM, null)))).toEqual({ kieu: "field-not-offered" });
    stub(400, { code: "invalid_request" });
    expect(await guiPhanAnh(taoLanGui(sendBody(FORM, null)))).toEqual({ kieu: "khong-hop-le" });
    expect(sendOutcome({ kieu: "field-not-offered" })).toEqual({ kind: "failed", failure: "field-not-offered" });
    expect(step({ kind: "ready", fields: [ITEM] }, true)).toContain(LOI_GUI["field-not-offered"].cau);
  });

  it("503 field_catalogue_unavailable on submit: nothing recorded, 'Gửi lại' with the same attempt", async () => {
    stub(503, { code: "field_catalogue_unavailable" });
    expect(await guiPhanAnh(taoLanGui(sendBody(FORM, null)))).toEqual({ kieu: "field-catalogue-unavailable" });
    expect(LOI_GUI["field-catalogue-unavailable"].co_the_gui_lai).toBe(true);
  });
});

describe("the list before any session — it asks, it never fetches", () => {
  it("idle: the card explaining why, and a button — no row, no number", () => {
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    const html = renderToStaticMarkup(
      createElement(PetitionList, {
        state: { kind: "idle" },
        onOpenPetition: () => {},
        onLookup: () => {},
        onOpen: () => {},
        onRetry: () => {},
        onLoadMore: () => {},
      }),
    );
    expect(html).toContain(XA_PA.need_session_body);
    expect(html).toContain(XA_PA.need_session_button);
    expect(html).not.toContain("(0)");
    expect(spy).not.toHaveBeenCalled();
  });
});
