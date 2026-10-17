import { afterEach, describe, expect, it, vi } from "vitest";

import {
  ADVANCED_SEARCH_OFF,
  CITIZEN_CONTACT_PAGE_SIZE,
  NO_CITIZEN_CONTACT_FILTER,
  PHONE_TAKEN_CODE,
  citizenContactCountBody,
  citizenContactListPath,
  citizenContactPhonePath,
  citizenContactSearchBody,
  contactEmailMasked,
  countCitizenContacts,
  createCitizenContact,
  deleteCitizenContact,
  getCitizenContact,
  hasAdvancedCitizenFilter,
  isCitizenContactFilterEmpty,
  listCitizenContactTags,
  normalizeCitizenKeyword,
  readCitizenContactPage,
  revealCitizenContactPhone,
  updateCitizenContact,
  type CitizenContactBody,
} from "./citizen-contacts";
import { LOI_KHONG_RO } from "./goi";

/**
 * The contact routes of the citizen directory (ADR 0097). Pinned: the keyword NEVER on a URL, the
 * paths, the mandatory reason, the key on create, the one reveal route and its 403 shape. Every number
 * is the repo's fake `0900000000` (rule 3, invariant 5).
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const empty = (status: number) => new Response(null, { status });
const sentBody = (fake: ReturnType<typeof stub>) => JSON.parse(String(fake.mock.calls[0]?.[1]?.body)) as unknown;
const sentHeaders = (fake: ReturnType<typeof stub>) => fake.mock.calls[0]?.[1]?.headers as Record<string, string> | undefined;

const PAGE = { items: [], next_cursor: "", has_more: false };
const BODY: CitizenContactBody = {
  full_name: "Nguyễn Văn A",
  phone: "0900000001",
  residential_unit_id: "",
  address: "",
  note: "",
  tags: ["tổ trưởng"],
  group_ids: [],
  is_active: true,
  email: "nguoidan@example.com",
  attributes: { hongheo1: "ho-ngheo", sonhan01: 4, ngaysinh: 1760634000000, nhucau01: ["dien", "nuoc"] },
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the filter", () => {
  it("empty by default; a keyword, a group or the Zalo box makes it non-empty", () => {
    expect(isCitizenContactFilterEmpty(NO_CITIZEN_CONTACT_FILTER)).toBe(true);
    expect(isCitizenContactFilterEmpty({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" })).toBe(false);
    expect(isCitizenContactFilterEmpty({ ...NO_CITIZEN_CONTACT_FILTER, groupId: "01JGROUP" })).toBe(false);
    expect(isCitizenContactFilterEmpty({ ...NO_CITIZEN_CONTACT_FILTER, hasOwnAppSession: true })).toBe(false);
  });

  it("the keyword is normalised as the server normalises it: collapsed whitespace, 200 characters, empty = none", () => {
    expect(normalizeCitizenKeyword("  Nguyễn   Văn ")).toEqual({ loai: "hopLe", tu: "Nguyễn Văn" });
    expect(normalizeCitizenKeyword("   ")).toEqual({ loai: "rong" });
    expect(normalizeCitizenKeyword("a".repeat(201))).toEqual({ loai: "quaDai" });
  });

  it("search body: every field, the page size, the cursor; count body: the same filter without paging", () => {
    const f = { ...NO_CITIZEN_CONTACT_FILTER, keyword: "0001", groupId: "01JGROUP", hasOwnAppSession: true };
    expect(citizenContactSearchBody(f, "con-tro-2")).toEqual({
      q: "0001",
      group_id: "01JGROUP",
      residential_unit_id: "",
      active: null,
      has_own_app_session: true,
      limit: CITIZEN_CONTACT_PAGE_SIZE,
      cursor: "con-tro-2",
    });
    expect(citizenContactSearchBody(NO_CITIZEN_CONTACT_FILTER, null).cursor).toBe("");
    expect(citizenContactCountBody(f)).toEqual({ q: "0001", group_id: "01JGROUP", residential_unit_id: "", active: null, has_own_app_session: true });
  });
});

describe("the ADR 0101 additions to the filter", () => {
  const ADV = {
    ...NO_CITIZEN_CONTACT_FILTER,
    createdFrom: 1760634000000,
    createdTo: null,
    attributeFilters: [
      { code: "hongheo1", values: ["ho-ngheo", "can-ngheo"] },
      { code: "sonhan01", from: 2 },
    ],
  };

  it("a date range or an attribute filter makes the filter non-empty AND advanced; a keyword alone is not advanced", () => {
    expect(isCitizenContactFilterEmpty({ ...NO_CITIZEN_CONTACT_FILTER, createdTo: 1 })).toBe(false);
    expect(isCitizenContactFilterEmpty({ ...NO_CITIZEN_CONTACT_FILTER, attributeFilters: [{ code: "hongheo1", values: ["x"] }] })).toBe(false);
    expect(hasAdvancedCitizenFilter(ADV)).toBe(true);
    expect(hasAdvancedCitizenFilter({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" })).toBe(false);
  });

  it("search and count bodies carry created_from / created_to / filters ONLY when set — today's body otherwise", () => {
    expect(citizenContactSearchBody(ADV, null)).toMatchObject({
      q: "",
      created_from: 1760634000000,
      filters: [
        { code: "hongheo1", values: ["ho-ngheo", "can-ngheo"] },
        { code: "sonhan01", from: 2 },
      ],
    });
    expect(citizenContactSearchBody(ADV, null)).not.toHaveProperty("created_to");
    expect(citizenContactCountBody(ADV)).toMatchObject({ created_from: 1760634000000, filters: ADV.attributeFilters });
    const plain = citizenContactSearchBody({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" }, null);
    for (const k of ["created_from", "created_to", "filters"]) expect(plain).not.toHaveProperty(k);
  });

  it("503 on an ADVANCED search: `searchUnavailable` and the fixed sentence, not the server's", async () => {
    const fake = stub(json({ code: "search_unavailable", message: "elasticsearch not configured", trace_id: "" }, 503));
    expect(await readCitizenContactPage(ADV, null)).toEqual({ ok: false, thongBao: ADVANCED_SEARCH_OFF, searchUnavailable: true });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-searches");
    expect(sentBody(fake)).toMatchObject({ created_from: 1760634000000 });
    expect(ADVANCED_SEARCH_OFF).toBe("Chưa bật tìm kiếm nâng cao trên máy chủ này");
  });

  it("503 on a KEYWORD-only search is the service down: the ordinary refusal, no `searchUnavailable`", async () => {
    stub(json({ code: "unavailable", message: "Dịch vụ tạm ngừng.", trace_id: "" }, 503));
    const r = await readCitizenContactPage({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" }, null);
    expect(r).toEqual({ ok: false, thongBao: "Dịch vụ tạm ngừng." });
  });

  it("200 on an advanced search: the page", async () => {
    stub(json(PAGE, 200));
    expect(await readCitizenContactPage(ADV, "c2")).toEqual({ ok: true, duLieu: PAGE });
  });
});

describe("the masked address", () => {
  it("read from `email_masked` or `email` (name unsettled until regen), empty when absent or null", () => {
    expect(contactEmailMasked({ email_masked: "ng***@example.com" })).toBe("ng***@example.com");
    expect(contactEmailMasked({ email: "ng***@example.com" })).toBe("ng***@example.com");
    expect(contactEmailMasked({ email_masked: null })).toBe("");
    expect(contactEmailMasked({})).toBe("");
  });
});

describe("page reads", () => {
  it("no filter: GET with limit=50, no `cursor=` on the first page, the cursor on the next", async () => {
    expect(citizenContactListPath(null)).toBe("/api/v1/citizen-contacts?limit=50");
    expect(citizenContactListPath("abc")).toBe("/api/v1/citizen-contacts?limit=50&cursor=abc");
    const fake = stub(json(PAGE, 200));
    expect(await readCitizenContactPage(NO_CITIZEN_CONTACT_FILTER, null)).toEqual({ ok: true, duLieu: PAGE });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts?limit=50");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("a keyword: POST /citizen-contact-searches, the keyword and the cursor IN THE BODY, the URL a constant", async () => {
    const fake = stub(json(PAGE, 200));
    await readCitizenContactPage({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "0900000001" }, "trang-2");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-searches");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentBody(fake)).toMatchObject({ q: "0900000001", cursor: "trang-2", limit: 50 });
    expect(String(fake.mock.calls[0]?.[0])).not.toContain("0900000001");
    expect(sentHeaders(fake)?.["Idempotency-Key"]).toBeUndefined();
  });

  it("only the Zalo box: still the search route (the list's query has no such field in the contract)", async () => {
    const fake = stub(json(PAGE, 200));
    await readCitizenContactPage({ ...NO_CITIZEN_CONTACT_FILTER, hasOwnAppSession: true }, null);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-searches");
    expect(sentBody(fake)).toMatchObject({ q: "", has_own_app_session: true });
  });

  it("count: POST /citizen-contact-count-queries, `{count}` unwrapped", async () => {
    const fake = stub(json({ count: 12 }, 200));
    expect(await countCitizenContacts({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" })).toEqual({ ok: true, duLieu: 12 });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-count-queries");
    expect(sentBody(fake)).toEqual({ q: "An", group_id: "", residential_unit_id: "", active: null, has_own_app_session: null });
  });

  it("tags and detail: their GET paths; the detail's number arrives masked", async () => {
    const fake = stub(json({ items: [{ tag: "hộ kinh doanh", count: 12 }] }, 200));
    expect(await listCitizenContactTags()).toEqual({ ok: true, duLieu: { items: [{ tag: "hộ kinh doanh", count: 12 }] } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-tags");
    const detail = { id: "01JC", full_name: "Nguyễn Văn A", phone_masked: "09****0001" };
    const fake2 = stub(json(detail, 200));
    expect(await getCitizenContact("01JC")).toEqual({ ok: true, duLieu: detail });
    expect(fake2.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts/01JC");
  });

  it("no path carries a tenant id — the commune is the Host's", () => {
    for (const p of [citizenContactListPath("x"), citizenContactPhonePath("01JC")]) expect(p).not.toMatch(/tenant/i);
  });
});

describe("writes", () => {
  it("create: POST with the caller's Idempotency-Key, 201 → the id; email and attributes travel in the body", async () => {
    const fake = stub(json({ id: "01JNEW" }, 201));
    expect(await createCitizenContact(BODY, "khoa-1")).toEqual({ ok: true, duLieu: { id: "01JNEW" } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)?.["Idempotency-Key"]).toBe("khoa-1");
    expect(sentBody(fake)).toEqual(BODY);
  });

  it("create 409 phone_taken: the server's sentence AND its code, so the dialog can say its own sentence", async () => {
    stub(json({ code: "phone_taken", message: "Số này đã có trong danh bạ.", trace_id: "01JT" }, 409));
    expect(await createCitizenContact(BODY, "khoa-1")).toEqual({ ok: false, thongBao: "Số này đã có trong danh bạ.", code: PHONE_TAKEN_CODE });
  });

  it("update: PATCH /citizen-contacts/{id}, no key, 204 → null; 409 carries the code", async () => {
    const fake = stub(empty(204));
    expect(await updateCitizenContact("01JC", BODY)).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts/01JC");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(sentHeaders(fake)?.["Idempotency-Key"]).toBeUndefined();
    stub(json({ code: "phone_taken", message: "Trùng số.", trace_id: "" }, 409));
    expect(await updateCitizenContact("01JC", BODY)).toEqual({ ok: false, thongBao: "Trùng số.", code: "phone_taken" });
  });

  it("delete: POST /citizen-contacts/{id}/deletion with `{reason}` — the reason is sent verbatim", async () => {
    const fake = stub(empty(204));
    expect(await deleteCitizenContact("01JC", "Nhập trùng")).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts/01JC/deletion");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentBody(fake)).toEqual({ reason: "Nhập trùng" });
  });

  it("the id is percent-encoded in every path", () => {
    expect(citizenContactPhonePath("a/b")).toBe("/api/v1/citizen-contacts/a%2Fb/phone");
  });
});

describe("Xem số đầy đủ — GET /citizen-contacts/{id}/phone", () => {
  it("200: the full number, exactly as the server sent it", async () => {
    const fake = stub(json({ phone: "0900000001" }, 200));
    expect(await revealCitizenContactPhone("01JC")).toEqual({ ok: true, duLieu: { phone: "0900000001" } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contacts/01JC/phone");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("403: `forbidden` is set — the button goes, the mask stays; 404 keeps the server's sentence without it", async () => {
    stub(json({ code: "forbidden", message: "Không có quyền.", trace_id: "" }, 403));
    expect(await revealCitizenContactPhone("01JC")).toEqual({ ok: false, thongBao: "Không có quyền.", forbidden: true });
    stub(json({ code: "citizen_contact_not_found", message: "Không tìm thấy người dân này trong danh bạ.", trace_id: "" }, 404));
    expect(await revealCitizenContactPhone("01JC")).toEqual({ ok: false, thongBao: "Không tìm thấy người dân này trong danh bạ." });
  });

  it("network down: the shared sentence, nothing thrown", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("offline"))));
    expect(await revealCitizenContactPhone("01JC")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});
