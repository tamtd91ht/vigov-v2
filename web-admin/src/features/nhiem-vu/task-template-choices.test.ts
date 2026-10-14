import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "@/lib/api/goi";

import {
  fetchTaskTemplate,
  gatherTaskTemplateChoices,
  readChoiceCodes,
  type TaskChoiceLoaders,
} from "./task-template-choices";

/**
 * `Tải mẫu giao việc` with its dropdown lists (owner 14/10/2026). Pinned: the screen's lists are used
 * when held, a missing one is read at the click, only codes go out, inactive blocs stay out, and a list
 * that cannot be read costs its dropdown — never the template.
 */

const ok = <T,>(duLieu: T) => Promise.resolve({ ok: true as const, duLieu });
const refused = () => Promise.resolve({ ok: false as const, thongBao: "Không đọc được." });

function loaders(over: Partial<TaskChoiceLoaders> = {}) {
  const l = {
    orgUnits: vi.fn(() => ok({ items: [{ code: "van-phong" }, { code: "tu-phap" }] })),
    staff: vi.fn(() => ok({ items: [{ code: "CB-00001" }, { code: "CB-00002" }] })),
    blocs: vi.fn(() =>
      ok({ items: [{ code: "khoi-uy-ban", active: true }, { code: "khoi-cu", active: false }, { code: "khac", active: true }] }),
    ),
    ...over,
  };
  return l;
}

function stubTemplate() {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => new Response(new Uint8Array([80, 75]), { status: 200 }));
  vi.stubGlobal("fetch", fake);
  return fake;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("gatherTaskTemplateChoices", () => {
  it("the screen's lists are sent as they are, and nothing is read again", async () => {
    const l = loaders();
    const onScreen = { orgUnitCodes: ["a"], staffCodes: ["CB-9"], blocCodes: ["k"] };
    expect(await gatherTaskTemplateChoices(onScreen, l)).toEqual(onScreen);
    expect(l.orgUnits).not.toHaveBeenCalled();
    expect(l.staff).not.toHaveBeenCalled();
    expect(l.blocs).not.toHaveBeenCalled();
  });

  it("a list the screen does not hold (absent or empty) is read at the click; blocs: active only", async () => {
    const l = loaders();
    expect(await gatherTaskTemplateChoices({ orgUnitCodes: [], staffCodes: ["CB-9"] }, l)).toEqual({
      orgUnitCodes: ["van-phong", "tu-phap"],
      staffCodes: ["CB-9"],
      blocCodes: ["khoi-uy-ban", "khac"],
    });
    expect(l.staff).not.toHaveBeenCalled();
  });

  it("a refused or throwing read leaves its list out — the others still come back", async () => {
    const l = loaders({ staff: refused, blocs: () => Promise.reject(new Error("offline")) });
    expect(await gatherTaskTemplateChoices({}, l)).toEqual({
      orgUnitCodes: ["van-phong", "tu-phap"],
      staffCodes: undefined,
      blocCodes: undefined,
    });
  });

  it("readChoiceCodes: codes only, never the rest of the row", async () => {
    expect(await readChoiceCodes(() => ok({ items: [{ code: "x", name: "Văn phòng" }] }))).toEqual(["x"]);
  });
});

describe("fetchTaskTemplate — what the dialog's template link calls", () => {
  it("sends the gathered codes as repeated params", async () => {
    const fake = stubTemplate();
    const r = await fetchTaskTemplate({ staffCodes: ["CB-9"] }, loaders());
    expect(r.ok).toBe(true);
    expect(fake.mock.calls[0]?.[0]).toBe(
      "/api/v1/tasks/import-template?org_unit=van-phong&org_unit=tu-phap&staff=CB-9&bloc=khoi-uy-ban&bloc=khac",
    );
  });

  it("every list read fails: the template STILL downloads, bare, and no error is returned for the lists", async () => {
    const fake = stubTemplate();
    const r = await fetchTaskTemplate({}, { orgUnits: refused, staff: refused, blocs: () => Promise.reject(new Error("x")) });
    expect(r.ok && r.duLieu.size).toBe(2);
    expect(fake.mock.calls.map((c) => c[0])).toEqual(["/api/v1/tasks/import-template"]);
  });

  it("only the template's own refusal is an error", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("offline"))));
    expect(await fetchTaskTemplate({ orgUnitCodes: ["a"] }, loaders())).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});
