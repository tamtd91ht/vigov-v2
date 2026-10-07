// @vitest-environment jsdom
//
// jsdom for this file: a header pressed, a unit chosen, a row clicked are events — a markup string
// cannot show them.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type {
  finance_categoryProgressOut,
  finance_duAnRa,
  finance_projectSummaryOut,
  identity_boPhanRa,
} from "@/lib/api/schema.gen";

import { BangDanhSach, BangDuAn } from "./bang-du-an";
import { CategoryProgressTable } from "./disbursement-overview";
import { shortDongLabel } from "./nhan-du-an";
import { PROGRESS_TONE_THRESHOLDS, progressTone } from "./progress-tone";
import { sortProjects } from "./project-sort";

/**
 * The Giải ngân list built as the prototype draws it (user decisions 07/10/2026, prototype
 * `apps/admin/src/components/budget/BudgetItemTable.tsx`, `BudgetWorkspace.tsx`,
 * `CategoryReportPanel.tsx`, `lib/budget-display.ts`): sortable money/progress headers, the elapsed-time
 * marker in each bar, the unit filter, the whole row opening the project, short amounts, and the
 * 80/50/30 colour tiers.
 */

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  push.mockReset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function settle(): Promise<void> {
  for (let i = 0; i < 6; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function choose(el: HTMLSelectElement, value: string): void {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function project(id: string, categoryId: string, over: Partial<finance_duAnRa> = {}): finance_duAnRa {
  return {
    id,
    code: id,
    year: 2026,
    category_id: categoryId,
    name: `Dự án ${id}`,
    planned_amount: 100_000_000,
    approved_amount: 100_000_000,
    disbursed_amount: 20_000_000,
    remaining_amount: 80_000_000,
    disbursed_ratio: 2000,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...over,
  };
}

// Server order is code order (`ORDER BY da.ma`): DA1, DA2, DA3.
const DA1 = project("DA1", "HM-A", { org_unit_id: "U-VP" });
const DA2 = project("DA2", "HM-B", {
  planned_amount: 7_500_000_000,
  disbursed_amount: 6_000_000_000,
  remaining_amount: 1_500_000_000,
  disbursed_ratio: 8000,
  org_unit_id: "U-DC",
});
const DA3 = project("DA3", "HM-A", {
  planned_amount: 300_000_000,
  disbursed_amount: 150_000_000,
  remaining_amount: 150_000_000,
  disbursed_ratio: 5000,
});
const PROJECTS = [DA1, DA2, DA3];

const UNITS: identity_boPhanRa[] = [
  { id: "U-DC", code: "dia-chinh", name: "Địa chính – Xây dựng", parent_id: "", order: 1, staff_count: 3 },
  { id: "U-VP", code: "van-phong", name: "Văn phòng UBND", parent_id: "", order: 2, staff_count: 5 },
  { id: "U-KHONG-DUNG", code: "khac", name: "Bộ phận không dự án nào dùng", parent_id: "", order: 3, staff_count: 1 },
];

function category(over: Partial<finance_categoryProgressOut>): finance_categoryProgressOut {
  return {
    order: 1,
    in_catalogue: true,
    project_count: 0,
    planned: 0,
    disbursed: 0,
    undisbursed: 0,
    disbursed_ratio: null,
    undisbursed_ratio: null,
    ...over,
  };
}

function summary(over: Partial<finance_projectSummaryOut> = {}): finance_projectSummaryOut {
  return {
    year: 2026,
    planned_total: 7_900_000_000,
    project_count: 3,
    disbursed_total: 6_170_000_000,
    disbursed_ratio: 7810,
    time_elapsed_ratio: 7096,
    remaining_total: 1_730_000_000,
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    delayed_project_count: 0,
    monthly: [],
    disbursed_after_year: 0,
    by_category: [
      category({ category_id: "HM-A", label: "Hạng mục A", project_count: 2, disbursed_ratio: 4250 }),
      category({ category_id: "HM-B", label: "Hạng mục B", order: 2, project_count: 1, disbursed_ratio: 8000 }),
    ],
    total: category({ order: 0, in_catalogue: false, project_count: 3, disbursed_ratio: 7810 }),
    ...over,
  };
}

function stub(opts: { items?: finance_duAnRa[]; summary?: finance_projectSummaryOut } = {}): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      if (url.startsWith("/api/v1/investment-project-summary?")) return json(200, opts.summary ?? summary());
      if (url.startsWith("/api/v1/investment-projects?")) {
        return json(200, {
          items: opts.items ?? PROJECTS,
          year: 2026,
          delay_threshold: 1000,
          delay_threshold_source: "mac_dinh",
        });
      }
      if (url === "/api/v1/org-units") return json(200, { items: UNITS });
      if (url === "/api/v1/staff-directory") return json(200, { items: [] });
      return new Response("unexpected", { status: 500 });
    }),
  );
}

const rowCodes = (el: ParentNode) => [...el.querySelectorAll("td.ma-muc")].map((c) => c.textContent);
const groupKeys = (el: ParentNode) =>
  [...el.querySelectorAll<HTMLElement>("tr[data-group-header]")].map((r) => r.dataset.groupHeader);
function sortButton(el: ParentNode, label: string): HTMLButtonElement {
  const button = [...el.querySelectorAll<HTMLButtonElement>("thead th button")].find(
    (b) => b.textContent?.trim() === label,
  );
  if (button === undefined) throw new Error(`no sort button "${label}"`);
  return button;
}

/* ── G1 sortable headers ──────────────────────────────────────────────────────────────────── */

describe("G1 — KH vốn năm / Đã giải ngân / Tiến độ sort the loaded list, largest first", () => {
  it("one press sorts descending, a second press returns to code order; the indicator follows", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    act(() => el.querySelector<HTMLInputElement>("#loc-gop-hang-muc")!.click()); // flat list
    expect(rowCodes(el)).toEqual(["DA1", "DA2", "DA3"]);

    act(() => sortButton(el, "KH vốn năm").click());
    expect(rowCodes(el)).toEqual(["DA2", "DA3", "DA1"]);
    expect(sortButton(el, "KH vốn năm").closest("th")!.getAttribute("aria-sort")).toBe("descending");
    expect(sortButton(el, "Đã giải ngân").closest("th")!.hasAttribute("aria-sort")).toBe(false);

    act(() => sortButton(el, "Tiến độ").click());
    expect(rowCodes(el)).toEqual(["DA2", "DA3", "DA1"]);
    expect(sortButton(el, "Tiến độ").closest("th")!.getAttribute("aria-sort")).toBe("descending");
    expect(sortButton(el, "KH vốn năm").closest("th")!.hasAttribute("aria-sort")).toBe(false);

    act(() => sortButton(el, "Tiến độ").click());
    expect(rowCodes(el)).toEqual(["DA1", "DA2", "DA3"]);
    expect(el.querySelector("thead th[aria-sort]")).toBeNull();
  });

  it("sorting reads nothing again — the list route already returned the whole year", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    const fetchSpy = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
    const before = fetchSpy.mock.calls.length;
    act(() => sortButton(el, "Đã giải ngân").click());
    await settle();
    expect(fetchSpy.mock.calls.length).toBe(before);
  });

  it("grouped: groups follow the first row of each, so the leading project's category leads (BIT:104-106)", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(groupKeys(el)).toEqual(["HM-A", "HM-B"]);
    expect(rowCodes(el)).toEqual(["DA1", "DA3", "DA2"]);

    act(() => sortButton(el, "Đã giải ngân").click());
    expect(groupKeys(el)).toEqual(["HM-B", "HM-A"]);
    expect(rowCodes(el)).toEqual(["DA2", "DA3", "DA1"]);
    // Sorting hides no row: the server's category totals still stand over each header.
    expect(el.querySelector('tr[data-group-header="HM-A"]')!.textContent).toContain("kế hoạch");
  });

  it("sortProjects: ties keep code order; a project with no plan (`null` ratio) sorts after every figure", () => {
    const a = project("A", "", { disbursed_ratio: null });
    const b = project("B", "", { disbursed_ratio: 0 });
    const c = project("C", "", { disbursed_ratio: 0 });
    const d = project("D", "", { disbursed_ratio: 12000 });
    expect(sortProjects([a, b, c, d], "disbursed_ratio").map((p) => p.id)).toEqual(["D", "B", "C", "A"]);
    expect(sortProjects([a, b, c, d], "code").map((p) => p.id)).toEqual(["A", "B", "C", "D"]);
  });
});

/* ── G2 elapsed-time marker ───────────────────────────────────────────────────────────────── */

describe("G2 — elapsed-time marker inside each row's bar", () => {
  it("at the summary's `time_elapsed_ratio` when the summary is of the list's year", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    const markers = el.querySelectorAll<HTMLElement>("tbody [data-elapsed-marker]");
    expect(markers).toHaveLength(3);
    expect(markers[0]!.style.left).toBe("70.96%");
    expect(markers[0]!.getAttribute("aria-hidden")).toBe("true");
  });

  it("no marker when the summary is of another year — never a figure from the browser's clock", async () => {
    stub({ summary: summary({ year: 2025 }) });
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(el.querySelectorAll("tbody [data-elapsed-marker]")).toHaveLength(0);
  });

  it("a project with no plan draws no bar, so no marker either", () => {
    const el = mount(
      <BangDanhSach
        duLieu={{ items: [project("X", "", { disbursed_ratio: null })], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }}
        danhMuc={[]}
        timeElapsedRatio={7096}
      />,
    );
    expect(el.querySelector("[data-elapsed-marker]")).toBeNull();
    expect(el.textContent).toContain("Chưa bố trí vốn");
  });
});

/* ── G3 unit filter ───────────────────────────────────────────────────────────────────────── */

describe("G3 — `Tất cả đơn vị phụ trách` filter", () => {
  it("options are the units the loaded rows name, by name; choosing one narrows the loaded list", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    const select = el.querySelector<HTMLSelectElement>("#loc-don-vi")!;
    expect(select).not.toBeNull();
    expect([...select.options].map((o) => o.textContent)).toEqual([
      "Tất cả đơn vị phụ trách",
      "Địa chính – Xây dựng",
      "Văn phòng UBND",
    ]);

    choose(select, "U-VP");
    expect(rowCodes(el)).toEqual(["DA1"]);
    // A subset of the category is on screen: the header counts rows, it carries no money.
    expect(el.querySelector('tr[data-group-header="HM-A"]')!.textContent).toContain("1 dự án khớp bộ lọc");

    choose(select, "");
    expect(rowCodes(el)).toEqual(["DA1", "DA3", "DA2"]);
  });

  it("hidden when no loaded project names a unit", async () => {
    stub({ items: [project("DA1", "HM-A"), project("DA3", "HM-A")] });
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(el.querySelector("#loc-don-vi")).toBeNull();
  });
});

/* ── G5 whole row opens the project ───────────────────────────────────────────────────────── */

describe("G5 — clicking anywhere on a row opens /giai-ngan/du-an/:id", () => {
  it("a click on a cell opens the project; the name stays a real link for keyboard and screen readers", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} canDelete />);
    await settle();
    const row = el.querySelector<HTMLElement>('tr[data-project-row="DA2"]')!;
    expect(row.querySelector("a")!.getAttribute("href")).toBe("/giai-ngan/du-an/DA2");

    act(() => row.querySelector<HTMLElement>("td.ma-muc")!.click());
    expect(push).toHaveBeenCalledWith("/giai-ngan/du-an/DA2");
  });

  it("the checkbox cell does not open the project — a mis-click would lose the selection", async () => {
    stub();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} canDelete />);
    await settle();
    const row = el.querySelector<HTMLElement>('tr[data-project-row="DA2"]')!;
    act(() => row.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
    act(() => row.querySelector<HTMLElement>("td")!.click());
    expect(push).not.toHaveBeenCalled();
    expect(row.querySelector<HTMLInputElement>('input[type="checkbox"]')!.checked).toBe(true);
  });

  it("an id is encoded in the path the row opens", () => {
    const open = vi.fn();
    const el = mount(
      <BangDanhSach
        duLieu={{ items: [project("a/b", "")], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }}
        danhMuc={[]}
        onOpenProject={open}
      />,
    );
    act(() => el.querySelector<HTMLElement>("td.ma-muc")!.click());
    expect(open).toHaveBeenCalledWith("a/b");
  });
});

/* ── G6 short amounts ─────────────────────────────────────────────────────────────────────── */

describe("G6 — short amounts in the KH vốn / Đã giải ngân columns", () => {
  it("shortDongLabel: the prototype's form, one decimal rounded DOWN, never up", () => {
    expect(shortDongLabel(7_500_000_000)).toBe("7,5 tỷ");
    expect(shortDongLabel(7_590_000_000)).toBe("7,5 tỷ");
    expect(shortDongLabel(8_000_000_000)).toBe("8 tỷ");
    expect(shortDongLabel(4_317_000_000_000)).toBe("4.317 tỷ");
    expect(shortDongLabel(100_000_000)).toBe("100 triệu");
    expect(shortDongLabel(1_250_000)).toBe("1,2 triệu");
    expect(shortDongLabel(950_000)).toBe("950.000 đ");
    expect(shortDongLabel(0)).toBe("0 đ");
    expect(shortDongLabel(-10_000_000)).toBe("-10 triệu");
    expect(shortDongLabel(Number.NaN)).toBe("Không đọc được");
  });

  it("the cells print the short form, and the full đồng stays on hover and for screen readers", () => {
    const el = mount(
      <BangDanhSach
        duLieu={{ items: [DA2], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }}
        danhMuc={[]}
      />,
    );
    const planned = el.querySelector<HTMLElement>("[data-money='planned']")!;
    const disbursed = el.querySelector<HTMLElement>("[data-money='disbursed']")!;
    expect(planned.querySelector("[title]")!.textContent).toBe("7,5 tỷ");
    expect(planned.querySelector("[title]")!.getAttribute("title")).toBe("7.500.000.000 đ");
    expect(planned.querySelector(".an-thi-giac")!.textContent).toBe("7.500.000.000 đ");
    expect(disbursed.querySelector("[title]")!.textContent).toBe("6 tỷ");
    expect(disbursed.querySelector(".an-thi-giac")!.textContent).toBe("6.000.000.000 đ");
  });
});

/* ── G8 colour tiers ──────────────────────────────────────────────────────────────────────── */

describe("G8 — 80/50/30 colour tiers (prototype `budget-display.ts:62-67`)", () => {
  it("one constant holds the thresholds, and the tiers fall on them", () => {
    expect(PROGRESS_TONE_THRESHOLDS).toEqual({ success: 80, brand: 50, warning: 30 });
    expect(progressTone(8000)).toBe("success");
    expect(progressTone(7999)).toBe("brand");
    expect(progressTone(5000)).toBe("brand");
    expect(progressTone(4999)).toBe("warning");
    expect(progressTone(3000)).toBe("warning");
    expect(progressTone(2999)).toBe("danger");
    expect(progressTone(12000)).toBe("success");
  });

  it("the list bar and its percent take the tier colour; the server's late flag stays the red edge", () => {
    const el = mount(
      <BangDanhSach
        duLieu={{
          items: [DA1, DA2, DA3, project("DA4", "", { disbursed_ratio: 3500, delay_score: 3596, is_delayed: true })],
          year: 2026,
          delay_threshold: 1000,
          delay_threshold_source: "mac_dinh",
        }}
        danhMuc={[]}
      />,
    );
    const tone = (id: string) =>
      el.querySelector<HTMLElement>(`tr[data-project-row="${id}"] [data-progress-tone]`)!.dataset.progressTone;
    expect(tone("DA1")).toBe("danger");
    expect(tone("DA2")).toBe("success");
    expect(tone("DA3")).toBe("brand");
    expect(tone("DA4")).toBe("warning");
    expect(el.querySelector('tr[data-project-row="DA2"] [data-progress-tone]')!.className).toContain("bg-success-500");
    // The late flag is the server's, unchanged: red left edge and "Chậm x điểm" only on DA4.
    expect(el.querySelector('tr[data-project-row="DA4"]')!.className).toContain("border-l-danger-500");
    expect(el.querySelector('tr[data-project-row="DA1"]')!.className).not.toContain("border-l-danger-500");
    expect(el.querySelector('tr[data-project-row="DA4"]')!.textContent).toContain("Chậm 35,96 điểm");
  });

  it("category table: the disbursed `Tỷ lệ` (rows and total) takes the tier colour; `—` and the undisbursed one do not", () => {
    const s = summary({
      by_category: [
        category({ category_id: "HM-A", label: "A", disbursed_ratio: 8500, undisbursed_ratio: 1500 }),
        category({ category_id: "HM-B", label: "B", disbursed_ratio: 2000, undisbursed_ratio: 8000 }),
        category({ category_id: "HM-C", label: "C", disbursed_ratio: null }),
      ],
      total: category({ disbursed_ratio: 4500, undisbursed_ratio: 5500 }),
    });
    const el = mount(
      <CategoryProgressTable
        year={2026}
        rows={s.by_category}
        total={s.total}
        selectedCategoryId=""
        onSelectCategory={() => {}}
      />,
    );
    const tones = [...el.querySelectorAll<HTMLElement>("td[data-ratio-tone]")].map((c) => c.dataset.ratioTone);
    expect(tones).toEqual(["success", "danger", "warning"]);
    expect(el.querySelector<HTMLElement>('td[data-ratio-tone="success"]')!.className).toContain("text-success-600");
  });
});
