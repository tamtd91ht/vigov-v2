import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchLetterTemplate } from "./letter-import-dialog";

/**
 * The letter template's org-unit dropdown (owner 14/10/2026): the units are read at the click, their
 * CODES go out as `org_unit`, and a failed read still yields the template.
 */

function stubTemplate() {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => new Response(new Uint8Array([80, 75]), { status: 200 }));
  vi.stubGlobal("fetch", fake);
  return fake;
}

const unit = (code: string) => ({ id: `id-${code}`, code, name: `Tên ${code}`, parent_id: "", order: 0, staff_count: 0 });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("fetchLetterTemplate", () => {
  it("the units' codes — never ids, never names — as repeated `org_unit`", async () => {
    const fake = stubTemplate();
    const r = await fetchLetterTemplate(async () => ({ ok: true, duLieu: { items: [unit("van-phong"), unit("tu-phap")] } }));
    expect(r.ok).toBe(true);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-letters/import-template?org_unit=van-phong&org_unit=tu-phap");
  });

  it("the unit read is refused or throws: the template STILL downloads, without the list", async () => {
    const fake = stubTemplate();
    expect((await fetchLetterTemplate(async () => ({ ok: false, thongBao: "x" }))).ok).toBe(true);
    expect((await fetchLetterTemplate(() => Promise.reject(new Error("offline")))).ok).toBe(true);
    expect(fake.mock.calls.map((c) => c[0])).toEqual([
      "/api/v1/citizen-letters/import-template",
      "/api/v1/citizen-letters/import-template",
    ]);
  });
});
