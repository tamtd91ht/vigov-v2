// @vitest-environment jsdom
//
// `/nguoi-dan` with the commune's attributes (ADR 0101): the list's extra columns (`show_in_list`, option
// labels), the advanced filter panel and the body it sends (keyword still in the BODY,
// D11), and the 503 "search engine not configured" answer that leaves the list one press away.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import { ADVANCED_SEARCH_OFF, type CitizenContact } from "@/lib/api/citizen-contacts";
import type { CitizenField } from "@/lib/api/citizen-fields";

import { CitizenDirectoryScreen } from "./citizen-directory-screen";
import { DROP_ADVANCED_FILTER, FILTER_APPLY, FILTER_TOGGLE, filterToggleLabel } from "./citizen-directory-words";

const H = vi.hoisted(() => ({ session: null as PhienDaDoc }));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const session = (permissions: string[]) =>
  ({ ok: true, duLieu: { staff: { full_name: "Nguyễn Văn Hùng", position: "Chuyên viên" }, role: null, permissions } }) as unknown as PhienDaDoc;

const base = (over: Partial<CitizenField>): CitizenField => ({
  code: "abcdef",
  label: "X",
  field_type: "singleText",
  placeholder: "",
  options: [],
  number_kind: "",
  date_pattern: "",
  required: false,
  show_in_list: false,
  sort_order: 0,
  created_at: "2026-10-17T00:00:00Z",
  updated_at: "2026-10-17T00:00:00Z",
  retired_at: null,
  ...over,
});
const FIELDS: CitizenField[] = [
  base({
    code: "hongheo1",
    label: "Phân loại hộ",
    field_type: "singleBox",
    show_in_list: true,
    sort_order: 1,
    options: [
      { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
      { value: "can-ngheo", label: "Cận nghèo", retired: false },
    ],
  }),
  base({ code: "sonhan01", label: "Số nhân khẩu", field_type: "number", number_kind: "integer", sort_order: 2 }),
  base({ code: "nghenghi", label: "Nghề nghiệp", sort_order: 3 }),
];

/** Fake data only (rule 3, invariant 5). */
const ROWS: CitizenContact[] = [
  {
    id: "01JCONTACTA",
    full_name: "Nguyễn Văn A",
    phone_masked: "09****0001",
    residential_unit_id: "",
    residential_unit_name: "",
    tags: [],
    group_ids: [],
    is_active: true,
    source: "nhap-tay",
    has_own_app_session: false,
    created_at: "2026-10-16T01:00:00Z",
    attributes: { hongheo1: "ho-ngheo", sonhan01: 4 },
  },
];

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
let searchStatus = 200;
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

beforeEach(() => {
  calls.length = 0;
  searchStatus = 200;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-fields") return json({ items: FIELDS });
      if (url === "/api/v1/citizen-groups") return json({ items: [] });
      if (url === "/api/v1/citizen-contact-tags") return json({ items: [] });
      if (url.startsWith("/api/v1/citizen-contacts?")) return json({ items: ROWS, next_cursor: "", has_more: false });
      if (url === "/api/v1/citizen-contact-searches")
        return searchStatus === 503
          ? json({ code: "search_unavailable", message: "search engine not configured", trace_id: "" }, 503)
          : json({ items: ROWS, next_cursor: "", has_more: false });
      if (url === "/api/v1/citizen-contact-count-queries") return json({ count: 1 });
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

const settle = async () => {
  for (let i = 0; i < 6; i++) await act(async () => {});
};

async function mount(): Promise<HTMLDivElement> {
  H.session = session(["citizen.read"]);
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => root!.render(<CitizenDirectoryScreen />));
  await settle();
  return host;
}

const buttonNamed = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent === text)!;
function typeInto(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
const searches = () => calls.filter((c) => c.url === "/api/v1/citizen-contact-searches");
const bodyOf = (c: Call) => JSON.parse(String(c.init?.body)) as Record<string, unknown>;

describe("list columns", () => {
  it("reads the LIVE definitions (no include_retired); a show_in_list field is a column with the option LABEL; no Email column (the list item carries none)", async () => {
    const el = await mount();
    expect(calls.some((c) => c.url === "/api/v1/citizen-fields")).toBe(true);
    const heads = [...el.querySelectorAll("thead th")].map((th) => th.textContent);
    expect(heads).not.toContain("Email");
    expect(heads).toContain("Phân loại hộ");
    expect(heads).not.toContain("Số nhân khẩu");
    const row = el.querySelector("tbody tr")!;
    expect(row.textContent).toContain("Hộ nghèo");
    expect(row.textContent).not.toContain("ho-ngheo");
  });
});

describe("advanced filters", () => {
  it("the panel offers choices for singleBox, a range for number, nothing for text; Áp dụng POSTs them in the BODY", async () => {
    const el = await mount();
    await act(async () => buttonNamed(el, FILTER_TOGGLE).click());
    const panel = el.querySelector<HTMLFormElement>("#bo-loc-nguoi-dan")!;
    expect(panel.querySelector('[data-filter="hongheo1"]')).not.toBeNull();
    expect(panel.querySelector('[data-filter="loc-sonhan01"]')).not.toBeNull();
    expect(panel.textContent).not.toContain("Nghề nghiệp");

    act(() => panel.querySelector<HTMLInputElement>('[data-filter="hongheo1"] input')!.click());
    typeInto(panel.querySelector<HTMLInputElement>("#loc-sonhan01-tu")!, "3");
    typeInto(panel.querySelector<HTMLInputElement>("#loc-ngay-tao-tu")!, "2026-10-17");
    await act(async () => buttonNamed(panel, FILTER_APPLY).click());
    await settle();

    const s = searches().at(-1)!;
    expect(s.init?.method).toBe("POST");
    expect(bodyOf(s)).toMatchObject({
      q: "",
      created_from: Date.UTC(2026, 9, 16, 17, 0),
      filters: [
        { code: "hongheo1", values: ["ho-ngheo"], from: null, to: null },
        { code: "sonhan01", values: [], from: 3, to: null },
      ],
      created_to: null,
    });
    for (const c of calls) expect(c.url).not.toContain("ho-ngheo");
    // One for the date range, one per attribute filter.
    expect(buttonNamed(el, filterToggleLabel(3))).toBeDefined();
  });

  it("503 on an advanced search: the fixed sentence, and one press drops the advanced part and reads the list again", async () => {
    searchStatus = 503;
    const el = await mount();
    await act(async () => buttonNamed(el, FILTER_TOGGLE).click());
    const panel = el.querySelector<HTMLFormElement>("#bo-loc-nguoi-dan")!;
    typeInto(panel.querySelector<HTMLInputElement>("#loc-sonhan01-den")!, "5");
    await act(async () => buttonNamed(panel, FILTER_APPLY).click());
    await settle();
    const notice = el.querySelector("[data-search-unavailable]")!;
    expect(notice.textContent).toContain(ADVANCED_SEARCH_OFF);
    expect(el.textContent).not.toContain("search engine not configured");

    const listReads = () => calls.filter((c) => c.url.startsWith("/api/v1/citizen-contacts?")).length;
    const before = listReads();
    await act(async () => buttonNamed(el, DROP_ADVANCED_FILTER).click());
    await settle();
    expect(listReads()).toBe(before + 1);
    expect(el.querySelector("[data-search-unavailable]")).toBeNull();
    expect(el.querySelectorAll("tbody tr")).toHaveLength(1);
  });

  it("a reversed range is refused under its control and nothing is sent", async () => {
    const el = await mount();
    await act(async () => buttonNamed(el, FILTER_TOGGLE).click());
    const panel = el.querySelector<HTMLFormElement>("#bo-loc-nguoi-dan")!;
    typeInto(panel.querySelector<HTMLInputElement>("#loc-sonhan01-tu")!, "9");
    typeInto(panel.querySelector<HTMLInputElement>("#loc-sonhan01-den")!, "1");
    const before = searches().length;
    await act(async () => buttonNamed(panel, FILTER_APPLY).click());
    await settle();
    expect(searches()).toHaveLength(before);
    expect(panel.textContent).toContain("“Từ” phải trước hoặc bằng “đến”.");
  });
});
