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

import { guiPhanAnh, type KetQuaGoi } from "../api/goi-vigov"; // vi-name-ok: existing exports
import { taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export

import { CUA_TOI, KENH_CHUA_MO, TRA_CUU, XA_PA } from "./noi-dung";
import {
  countLabel,
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
  linh_vuc: "Điện",
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

describe("send body — the contract's fields, no `field` yet, lat/lng only when tapped", () => {
  it("five fields, trimmed; the temporary field name is NOT sent (it is not a commune catalogue code)", () => {
    const body = JSON.parse(sendBody(FORM, null)) as Record<string, unknown>;
    expect(body).toEqual({
      content: "Đèn đường hỏng đầu ngõ",
      address: "Ngõ 12",
      reporter_name: "Nguyễn Văn An",
      reporter_phone: "0900000000",
      anonymous: false,
    });
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
