import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE OPTIONAL THÔN / TỔ DÂN PHỐ AT THE API LAYER (ADR 0088, 09/10/2026): the list route on IDENTITY's host, the one
 * optional body key, and the two refusals of the send. Session and host are faked for THIS file only, as in
 * `goi-vigov.test.tsx` — product code has no slot to inject either.
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-test", ten_xa: "Xã Thử Nghiệm" } as { token: string; ten_xa: string } | null, // vi-name-ok: existing session field
  services: [] as string[],
}));

vi.mock("./phien-vigov", () => ({ layPhienViGov: () => state.session })); // vi-name-ok: existing export, mocked not renamed
vi.mock("./dia-chi-vigov", () => ({
  diaChiViGov: (service: string, path: string) => { // vi-name-ok: existing export, mocked not renamed
    state.services.push(service);
    return `https://${service}.api.example${path}`;
  },
}));

import { guiPhanAnh, myResidentialUnits } from "./goi-vigov"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import {
  accountlessReportBody,
  MY_RESIDENTIAL_UNITS_PATH,
  OPTIONAL_RESIDENTIAL_UNIT_KEY,
  type PhanAnhMoi, // vi-name-ok: existing contract type, not renamed (rule 12 #3)
  readResidentialUnits,
  thanGuiPhanAnh, // vi-name-ok: existing export, not renamed (rule 12 #3)
} from "./hop-dong-phan-anh";
import { taoLanGui } from "./lan-gui"; // vi-name-ok: existing export, not renamed (rule 12 #3)

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];

function answer(status: number, body: unknown) {
  return { status, ok: status < 300, json: async () => body };
}

function stubFetch(...answers: Array<ReturnType<typeof answer>>) {
  let i = 0;
  vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return Promise.resolve(answers[Math.min(i++, answers.length - 1)]!);
  });
}

// vi-name-ok: the keys of the existing contract type `PhanAnhMoi`
const FORM: PhanAnhMoi = { noi_dung: "Đèn đường hỏng", dia_chi: "", ho_ten: "", dien_thoai: "", an_danh: true };
const UNIT_ID = "01J9ZQ4V6W8X0Y2Z4A6B8C0D2E";

/** Keys that would let the client name a commune — none may ever leave the phone (rule 1, forbidden #2). */
const COMMUNE_KEYS = ["tenant_id", "tenantId", "commune", "commune_id", "xa", "ma_xa", "host"];

const send = (form: PhanAnhMoi) => guiPhanAnh(taoLanGui(thanGuiPhanAnh(form))); // vi-name-ok: existing names called

beforeEach(() => {
  calls = [];
  state.services = [];
  state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" }; // vi-name-ok: existing session field
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("GET /api/v1/my-residential-units", () => {
  it("goes to identity's host with the session bearer, no query, nothing naming the commune", async () => {
    stubFetch(answer(200, { items: [{ id: UNIT_ID, name: "Thôn Bình An" }] }));
    const result = await myResidentialUnits();
    expect(result).toEqual({ kieu: "xong", units: [{ id: UNIT_ID, name: "Thôn Bình An" }] });
    expect(state.services).toEqual(["identity"]);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(`https://identity.api.example${MY_RESIDENTIAL_UNITS_PATH}`);
    expect(calls[0]!.init.method).toBe("GET");
    expect(calls[0]!.init.body).toBeUndefined();
    const headers = calls[0]!.init.headers as Record<string, string>;
    expect(headers["Authorization"]).toBe("Bearer tok-test");
    for (const h of Object.keys(headers)) expect(h.toLowerCase()).not.toMatch(/tenant|commune|xa/);
  });

  it("no session → no call at all", async () => {
    state.session = null;
    stubFetch(answer(200, { items: [] }));
    expect(await myResidentialUnits()).toEqual({ kieu: "chua-co-phien" });
    expect(calls).toEqual([]);
  });

  it("an empty list is a list; a malformed one (one bad row) is a server fault, never a shortened list", async () => {
    stubFetch(answer(200, { items: [] }));
    expect(await myResidentialUnits()).toEqual({ kieu: "xong", units: [] });
    expect(readResidentialUnits({ items: [{ id: UNIT_ID, name: "Thôn A" }, { id: "", name: "Thôn B" }] })).toBeNull();
    expect(readResidentialUnits({ items: [{ id: UNIT_ID, name: "  " }] })).toBeNull();
    expect(readResidentialUnits({ items: [{ id: UNIT_ID }] })).toBeNull();
    expect(readResidentialUnits({})).toBeNull();
    stubFetch(answer(200, { items: [{ id: 7, name: "Thôn A" }] }));
    expect(await myResidentialUnits()).toEqual({ kieu: "loi-may-chu" });
  });
});

describe("the send body — `residential_unit_id` only when one was chosen", () => {
  it("chosen → the id; none / null / blank → no key at all", () => {
    expect(JSON.parse(thanGuiPhanAnh({ ...FORM, residential_unit_id: UNIT_ID }))[OPTIONAL_RESIDENTIAL_UNIT_KEY]).toBe(UNIT_ID);
    for (const none of [undefined, null, "", "  "]) {
      expect(JSON.parse(thanGuiPhanAnh({ ...FORM, residential_unit_id: none }))).not.toHaveProperty(
        OPTIONAL_RESIDENTIAL_UNIT_KEY,
      );
    }
  });

  it("never a key naming the commune, with or without a unit", () => {
    for (const body of [thanGuiPhanAnh(FORM), thanGuiPhanAnh({ ...FORM, residential_unit_id: UNIT_ID })]) {
      const keys = Object.keys(JSON.parse(body) as object);
      for (const k of COMMUNE_KEYS) expect(keys).not.toContain(k);
    }
  });

  it("the accountless body drops it (the public route refuses the key with 400)", () => {
    const body = JSON.parse(accountlessReportBody({ ...FORM, residential_unit_id: UNIT_ID }, "xa.vigov.example"));
    expect(body).not.toHaveProperty(OPTIONAL_RESIDENTIAL_UNIT_KEY);
  });
});

describe("the send's two residential-unit refusals carry the server's sentence", () => {
  const sentence400 =
    "Thôn, tổ dân phố đã chọn hiện không có trong danh sách của xã. Vui lòng chọn lại thôn, tổ dân phố.";
  const sentence503 =
    "Chưa kiểm tra được thôn, tổ dân phố đã chọn nên phiếu CHƯA được ghi. Vui lòng thử lại sau ít phút.";

  it("400 residential_unit_not_offered → its own branch with the sentence", async () => {
    stubFetch(answer(400, { code: "residential_unit_not_offered", message: sentence400, trace_id: "t" }));
    expect(await send({ ...FORM, residential_unit_id: UNIT_ID })).toEqual({
      kieu: "residential-unit-not-offered",
      message: sentence400,
    });
  });

  it("503 residential_unit_check_unavailable → its own branch with the sentence", async () => {
    stubFetch(answer(503, { code: "residential_unit_check_unavailable", message: sentence503 }));
    expect(await send(FORM)).toEqual({ kieu: "residential-unit-check-unavailable", message: sentence503 });
  });

  it("no sentence in the body → `null` (the screen says its own); other codes keep their old branches", async () => {
    stubFetch(answer(400, { code: "residential_unit_not_offered" }));
    expect(await send(FORM)).toEqual({ kieu: "residential-unit-not-offered", message: null });
    stubFetch(answer(400, { code: "field_not_offered", message: "x" }));
    expect(await send(FORM)).toEqual({ kieu: "field-not-offered" });
    stubFetch(answer(400, { code: "invalid_request", message: "content" }));
    expect(await send(FORM)).toEqual({ kieu: "khong-hop-le" });
    stubFetch(answer(503, { code: "field_catalogue_unavailable" }));
    expect(await send(FORM)).toEqual({ kieu: "field-catalogue-unavailable" });
    stubFetch(answer(503, { code: "deadline_config_unavailable" }));
    expect(await send(FORM)).toEqual({ kieu: "kenh-chua-mo" });
  });
});
