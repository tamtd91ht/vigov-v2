import { afterEach, describe, expect, it, vi } from "vitest";

import { contactPetitionsPath, directoryMatchPath, listContactPetitions, readDirectoryMatch } from "./petition-directory-link";

/** ADR 0102: the two read routes of the petition ↔ directory link — paths, the four answers, no number anywhere. */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("directory match", () => {
  it("path: the lookup code, percent-encoded", () => {
    expect(directoryMatchPath("PA-7K2Q9X")).toBe("/api/v1/citizen-reports/PA-7K2Q9X/directory-match");
    expect(directoryMatchPath("a/b")).toBe("/api/v1/citizen-reports/a%2Fb/directory-match");
  });

  it("found → the row id and the flag; GET, same-origin, no-store", async () => {
    const fake = stub(json({ status: "found", contact_id: "01JCONTACT", phone_unverified: false }, 200));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "found", contactId: "01JCONTACT", phoneUnverified: false });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-1/directory-match");
    expect(fake.mock.calls[0]?.[1]).toMatchObject({ method: "GET", credentials: "same-origin", cache: "no-store" });
  });

  it("not_found keeps phone_unverified; not_applicable is its own answer", async () => {
    stub(json({ status: "not_found", contact_id: null, phone_unverified: true }, 200));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "not_found", phoneUnverified: true });
    stub(json({ status: "not_applicable", contact_id: null, phone_unverified: false }, 200));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "not_applicable" });
  });

  it("FAIL CLOSED: 503, 403, an unknown status, `found` without an id, a network failure → error, never not_found", async () => {
    stub(json({ code: "identity_unavailable", message: "Chưa kết nối được danh bạ.", trace_id: "" }, 503));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "error" });
    stub(json({ code: "forbidden", message: "Không có quyền.", trace_id: "" }, 403));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "error" });
    stub(json({ status: "maybe", contact_id: null, phone_unverified: false }, 200));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "error" });
    stub(json({ status: "found", contact_id: null, phone_unverified: false }, 200));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "error" });
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("offline"))));
    expect(await readDirectoryMatch("PA-1")).toEqual({ kind: "error" });
  });
});

describe("a person's petitions", () => {
  it("path: the ROW id in the path, page and page_size=10 in the query — no phone", () => {
    expect(contactPetitionsPath("01JCONTACT", 1)).toBe("/api/v1/citizen-report-lists/by-contact/01JCONTACT?page=1&page_size=10");
    expect(contactPetitionsPath("01JCONTACT", 2)).toBe("/api/v1/citizen-report-lists/by-contact/01JCONTACT?page=2&page_size=10");
  });

  it("200 → the page; a refusal → the server's sentence", async () => {
    const page = { items: [{ lookup_code: "PA-1", title: "Đèn đường hỏng", status: "moi-tiep-nhan", received_at: "2026-10-16T01:00:00Z" }], total: 1, page: 1, page_size: 10 };
    const fake = stub(json(page, 200));
    expect(await listContactPetitions("01JCONTACT", 1)).toEqual({ ok: true, duLieu: page });
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    stub(json({ code: "forbidden", message: "Tài khoản thiếu quyền xem phản ánh.", trace_id: "" }, 403));
    expect(await listContactPetitions("01JCONTACT", 1)).toEqual({ ok: false, thongBao: "Tài khoản thiếu quyền xem phản ánh." });
  });
});
