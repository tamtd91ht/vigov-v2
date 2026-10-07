// @vitest-environment jsdom
//
// jsdom for this file: a category row pressed, a checkbox ticked, a tab opened and the reads that
// follow are events and captured requests — a markup string cannot show them.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type {
  finance_categoryProgressOut,
  finance_curvePointOut,
  finance_duAnRa,
  finance_projectSummaryOut,
} from "@/lib/api/schema.gen";

import { BangDuAn } from "./bang-du-an";
import { ChiTietDuAn, ThongTinDuAn } from "./chi-tiet-du-an";
import { actualSegments, axisMoneyLabel, chartScale, CumulativeChart } from "./cumulative-chart";
import { KpiCards, yearIsBehind } from "./disbursement-overview";
import { pendingPart } from "./nhan-ghi-giai-ngan";
import { groupProjects } from "./project-groups";

/**
 * §3 KPI cards, §4 chart, §5 category table, §7.1 filters and grouping, §8 elapsed marker and §8.3
 * `Biểu đồ` against the routes of a3fdcac2. Every figure asserted here is one the fake server sent —
 * the screen adds no money up.
 */

// The list opens a project on a row click through the App Router (G5); no router is mounted here.
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));

const fakeSession = { ok: true as const, duLieu: { permissions: ["budget.read"] } };
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
  PhienProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

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

function typeInto(input: HTMLInputElement, value: string): void {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function point(month: number, planned: number, disbursed: number | null): finance_curvePointOut {
  return { month, planned_cumulative: planned, disbursed_cumulative: disbursed };
}

/** Twelve months; actual known through September, `null` after (months not yet begun). */
const MONTHLY = Array.from({ length: 12 }, (_, i) =>
  point(i + 1, Math.round((33_230_000_000 * (i + 1)) / 12), i < 9 ? 400_000_000 * (i + 1) : null),
);

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

const CAT_A = category({
  category_id: "HM-A",
  label: "Các công trình chuyển tiếp",
  project_count: 2,
  planned: 800_000_000,
  disbursed: 260_690_000,
  undisbursed: 539_310_000,
  disbursed_ratio: 3259,
  undisbursed_ratio: 6741,
  disbursement_deadline: "2026-12-31",
});
const CAT_B = category({
  category_id: "HM-B",
  label: "Vốn đầu tư các công trình xây dựng mới",
  order: 2,
  project_count: 1,
  planned: 100_000_000,
  disbursed: 120_000_000,
  undisbursed: -20_000_000,
  disbursed_ratio: 12000,
  undisbursed_ratio: -2000,
});

function summary(over: Partial<finance_projectSummaryOut> = {}): finance_projectSummaryOut {
  return {
    year: 2026,
    planned_total: 33_230_000_000,
    project_count: 63,
    disbursed_total: 3_433_990_000,
    disbursed_ratio: 1033,
    time_elapsed_ratio: 7096,
    remaining_total: 29_796_010_000,
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    delayed_project_count: 29,
    monthly: MONTHLY,
    disbursed_after_year: 0,
    by_category: [CAT_A, CAT_B],
    total: category({
      order: 0,
      in_catalogue: false,
      project_count: 3,
      planned: 900_000_000,
      disbursed: 380_690_000,
      undisbursed: 519_310_000,
      disbursed_ratio: 4230,
      undisbursed_ratio: 5770,
    }),
    ...over,
  };
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
    disbursed_amount: 90_000_000,
    remaining_amount: 10_000_000,
    disbursed_ratio: 9000,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...over,
  };
}

const PROJECTS = [project("DA1", "HM-A"), project("DA2", "HM-B"), project("DA3", "HM-A")];

type Seen = { method: string; url: string };

function stubFinance(opts: { summary?: () => Response; list?: (url: string) => Response } = {}): Seen[] {
  const seen: Seen[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      seen.push({ method, url });
      if (url.startsWith("/api/v1/investment-project-summary?")) return opts.summary?.() ?? json(200, summary());
      if (url.startsWith("/api/v1/investment-projects?")) {
        return (
          opts.list?.(url) ??
          json(200, { items: PROJECTS, year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" })
        );
      }
      return new Response("unexpected", { status: 500 });
    }),
  );
  return seen;
}

const summaryReads = (seen: Seen[]) => seen.filter((s) => s.url.startsWith("/api/v1/investment-project-summary?"));
const listReads = (seen: Seen[]) => seen.filter((s) => s.url.startsWith("/api/v1/investment-projects?"));
const groupHeaders = (el: ParentNode) =>
  [...el.querySelectorAll<HTMLElement>("tr[data-group-header]")].map((r) => r.textContent);

/* ── §3 KPI cards ────────────────────────────────────────────────────────────────────────── */

describe("§3 KPI cards — the server's figures, formatted, never re-derived", () => {
  it("draws the four cards with §3's worked example, and paints the year red when behind", () => {
    const el = mount(<KpiCards summary={summary()} />);
    const text = el.textContent ?? "";
    expect(text).toContain("33.230.000.000 đ");
    expect(text).toContain("63 dự án");
    expect(text).toContain("3.433.990.000 đ");
    expect(text).toContain("10,33% kế hoạch · thời gian đã qua 70,96%");
    expect(text).toContain("29.796.010.000 đ");
    expect(text).toContain("Ngưỡng cảnh báo chậm: 10,00 điểm");
    expect(text).toContain("29 dự án chậm");
    // 70,96 − 10,33 = 60,63 > 10: the disbursed figure and the late count are red.
    expect(yearIsBehind(summary())).toBe(true);
    expect(el.querySelectorAll("p.text-danger-600")).toHaveLength(2);
  });

  it("a year with no plan says so in words — never `0%`, never red", () => {
    const s = summary({ planned_total: 0, project_count: 0, disbursed_total: 0, disbursed_ratio: null, delayed_project_count: 0 });
    const el = mount(<KpiCards summary={s} />);
    expect(el.textContent).toContain("Chưa bố trí vốn · thời gian đã qua 70,96%");
    expect(el.textContent).not.toContain("0,00%");
    expect(yearIsBehind(s)).toBe(false);
    expect(el.querySelectorAll("p.text-danger-600")).toHaveLength(0);
  });

  it("an over-disbursed year: remainder NEGATIVE and ratio above 100%, not clamped", () => {
    const el = mount(
      <KpiCards summary={summary({ disbursed_total: 11_000, planned_total: 10_000, remaining_total: -1_000, disbursed_ratio: 11000 })} />,
    );
    expect(el.textContent).toContain("-1.000 đ");
    expect(el.textContent).toContain("110,00% kế hoạch");
  });

  it("on the threshold exactly is NOT behind (strictly greater, as `domain.LaCham`)", () => {
    expect(yearIsBehind(summary({ time_elapsed_ratio: 2033, disbursed_ratio: 1033, delay_threshold: 1000 }))).toBe(false);
    expect(yearIsBehind(summary({ time_elapsed_ratio: 2034, disbursed_ratio: 1033, delay_threshold: 1000 }))).toBe(true);
  });

  it("fourth card: the server's open-issue count; the at-risk half stays a '?' with no number", () => {
    const el = mount(<KpiCards summary={summary({ open_issue_count: 4 })} />);
    expect(el.querySelector("[data-open-issues]")?.textContent).toBe("4 vướng mắc đang theo dõi");
    const spot = el.querySelector<HTMLElement>("[data-pending]")!;
    expect(spot.textContent).not.toMatch(/\d/);
    expect(spot.querySelector("button[data-pending-marker]")?.getAttribute("aria-label")).toContain(
      pendingPart("Nguy cơ không giải ngân hết").ten,
    );
  });

  it("fourth card: zero is said as 0; an ABSENT count is not turned into 0", () => {
    const zero = mount(<KpiCards summary={summary({ open_issue_count: 0 })} />);
    expect(zero.querySelector("[data-open-issues]")?.textContent).toBe("0 vướng mắc đang theo dõi");
    act(() => root?.unmount());
    host?.remove();
    const absent = mount(<KpiCards summary={summary({ open_issue_count: undefined })} />);
    expect(absent.querySelector("[data-open-issues]")?.textContent).toBe("Chưa đọc được số vướng mắc");
  });
});

/* ── §4 chart ────────────────────────────────────────────────────────────────────────────── */

describe("§4 cumulative chart", () => {
  it("five Y ticks from the plan, §4's own example: 33,23 tỷ → 0 · 8,5 · 17 · 25,5 · 34 tỷ", () => {
    const { top, ticks } = chartScale(MONTHLY);
    expect(top).toBe(34_000_000_000);
    expect(ticks.map(axisMoneyLabel)).toEqual(["0 đ", "8,5 tỷ", "17 tỷ", "25,5 tỷ", "34 tỷ"]);
  });

  it("an actual line above the plan widens the scale instead of leaving the frame", () => {
    expect(chartScale([point(1, 100, 180)]).top).toBe(180);
  });

  it("the actual line STOPS at a null month: one run Jan–Sep, nine dots, no point after", () => {
    expect(actualSegments(MONTHLY).map((r) => r.length)).toEqual([9]);
    const el = mount(<CumulativeChart points={MONTHLY} caption="c" emptyText="trống" />);
    expect(el.querySelectorAll('polyline[data-line="actual"]')).toHaveLength(1);
    expect(el.querySelectorAll("circle[data-dot]")).toHaveLength(9);
    expect(el.querySelectorAll('polyline[data-line="plan"]')).toHaveLength(1);
    // The hidden table says the future months are not reached — never "0 đ".
    const rows = [...el.querySelectorAll("table tbody tr")];
    expect(rows).toHaveLength(12);
    expect(rows[11]!.textContent).toContain("Chưa đến tháng này");
    expect(el.textContent).toContain("Kế hoạch");
    expect(el.textContent).toContain("Thực hiện");
  });

  it("a gap in the middle breaks the line rather than bridging it", () => {
    expect(actualSegments([point(1, 1, 1), point(2, 2, null), point(3, 3, 3)]).map((r) => r.map((p) => p.index))).toEqual([
      [0],
      [2],
    ]);
  });

  it("nothing to scale → the sentence, no chart", () => {
    const html = renderToStaticMarkup(
      <CumulativeChart points={[point(1, 0, 0)]} caption="c" emptyText="Chưa có kế hoạch vốn để vẽ." />,
    );
    expect(html).toContain("Chưa có kế hoạch vốn để vẽ.");
    expect(html).not.toContain("<svg");
  });
});

/* ── Register: summary, §5 click filter, §7.1 filters, grouping, re-reads ──────────────────── */

describe("register with the year summary", () => {
  it("§5 table: rows, a bold `Tổng cộng`, the caption — and a negative remainder shown negative", async () => {
    stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    const table = el.querySelector("[data-category-table]")!;
    expect(table.querySelectorAll("tr[data-category-row]")).toHaveLength(2);
    const total = table.querySelector<HTMLElement>("tr[data-category-total]")!;
    expect(total.className).toContain("font-bold");
    expect(total.textContent).toContain("Tổng cộng");
    expect(total.textContent).toContain("900.000.000 đ");
    expect(table.textContent).toContain("-20.000.000 đ");
    expect(table.textContent).toContain("120,00%");
    expect(el.textContent).toContain("Chạm một hạng mục để lọc danh sách dự án bên dưới.");
  });

  it("pressing a category row filters the list by it (server `category`); pressing it again clears", async () => {
    const seen = stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(listReads(seen)).toHaveLength(1);

    act(() => el.querySelector<HTMLElement>('tr[data-category-row="HM-B"]')!.click());
    await settle();
    expect(listReads(seen).at(-1)!.url).toBe("/api/v1/investment-projects?year=2026&category=HM-B");
    expect(el.querySelector('tr[data-category-row="HM-B"] button')?.getAttribute("aria-pressed")).toBe("true");
    // The select shows the chosen category even though the catalogue prop is empty.
    expect(el.querySelector<HTMLSelectElement>("#loc-hang-muc")!.value).toBe("HM-B");

    act(() => el.querySelector<HTMLElement>('tr[data-category-row="HM-B"] button')!.click());
    await settle();
    expect(listReads(seen).at(-1)!.url).toBe("/api/v1/investment-projects?year=2026");
    // The year figures do not move with a list filter: no second summary read.
    expect(summaryReads(seen)).toHaveLength(1);
  });

  it("`Chỉ dự án chậm` asks the SERVER (`delayed_only=true`); unticking drops the parameter", async () => {
    const seen = stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    const box = el.querySelector<HTMLInputElement>("#loc-chi-du-an-cham")!;
    expect(box.disabled).toBe(false);
    expect(box.checked).toBe(false);

    act(() => box.click());
    await settle();
    expect(listReads(seen).at(-1)!.url).toBe("/api/v1/investment-projects?year=2026&delayed_only=true");

    act(() => box.click());
    await settle();
    expect(listReads(seen).at(-1)!.url).toBe("/api/v1/investment-projects?year=2026");
    expect(summaryReads(seen)).toHaveLength(1);
  });

  it("`Chỉ dự án chậm` with no late project: good news, not the empty-year invitation", async () => {
    stubFinance({
      list: (url) =>
        json(200, {
          items: url.includes("delayed_only=true") ? [] : PROJECTS,
          year: 2026,
          delay_threshold: 1000,
          delay_threshold_source: "mac_dinh",
        }),
    });
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    act(() => el.querySelector<HTMLInputElement>("#loc-chi-du-an-cham")!.click());
    await settle();
    expect(el.textContent).toContain("Không có dự án nào đang chậm");
    expect(el.textContent).not.toContain("Chưa có dự án nào");
  });

  it("`Gộp theo hạng mục` is ON by default: headers in first-row order with the SERVER's totals", async () => {
    stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(el.querySelector<HTMLInputElement>("#loc-gop-hang-muc")!.checked).toBe(true);
    expect(groupHeaders(el)).toEqual([
      "Các công trình chuyển tiếp — 2 dự án · kế hoạch 800.000.000 đ · đã giải ngân 260.690.000 đ",
      "Vốn đầu tư các công trình xây dựng mới — 1 dự án · kế hoạch 100.000.000 đ · đã giải ngân 120.000.000 đ",
    ]);
    // Rows under their header: DA1 and DA3 (HM-A) before DA2 (HM-B), server order kept inside a group.
    const codes = [...el.querySelectorAll("tbody tr:not([data-group-header]) td.ma-muc")].map((c) => c.textContent);
    expect(codes).toEqual(["DA1", "DA3", "DA2"]);

    act(() => el.querySelector<HTMLInputElement>("#loc-gop-hang-muc")!.click());
    expect(groupHeaders(el)).toEqual([]);
    expect([...el.querySelectorAll("td.ma-muc")].map((c) => c.textContent)).toEqual(["DA1", "DA2", "DA3"]);
  });

  it("with a keyword the headers carry the COUNT of matching rows only — never the category's money", async () => {
    stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    typeInto(el.querySelector<HTMLInputElement>("#tim-du-an")!, "DA3");
    expect(groupHeaders(el)).toEqual(["Các công trình chuyển tiếp — 1 dự án khớp bộ lọc"]);
  });

  it("summary failed: the list still works, headers count rows, the summary says why and retries", async () => {
    let fail = true;
    const seen = stubFinance({
      summary: () => (fail ? json(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }) : json(200, summary())),
    });
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(el.textContent).toContain("Chưa tải được số liệu tổng hợp của năm");
    expect(el.textContent).toContain("Bạn không có quyền thực hiện thao tác này.");
    expect(groupHeaders(el).every((h) => h !== null && !h.includes("kế hoạch"))).toBe(true);
    expect(el.querySelectorAll("td.ma-muc")).toHaveLength(3);

    fail = false;
    const retry = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Tải lại")!;
    act(() => retry.click());
    await settle();
    expect(summaryReads(seen)).toHaveLength(2);
    expect(el.querySelector("[data-kpi-cards]")).not.toBeNull();
  });

  it("re-reads the summary after a project is added (the page bumps `reloadSignal`)", async () => {
    const seen = stubFinance();
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(summaryReads(seen)).toHaveLength(1);
    act(() => root!.render(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={1} />));
    await settle();
    expect(summaryReads(seen)).toHaveLength(2);
    expect(listReads(seen)).toHaveLength(2);
    expect(el.querySelector("[data-kpi-cards]")).not.toBeNull();
  });

  it("asks the summary of the year on screen, and nothing about the commune", async () => {
    const seen = stubFinance();
    mount(<BangDuAn nam={2025} danhMuc={[]} reloadSignal={0} />);
    await settle();
    expect(summaryReads(seen)[0]!.url).toBe("/api/v1/investment-project-summary?year=2025");
  });
});

describe("groupProjects — when the server's totals may stand over a group", () => {
  const items = [project("DA1", "HM-A"), project("DA3", "HM-A")];

  it("summary count differs from the rows (two separate reads): count only, no money", () => {
    const [g] = groupProjects(items, {
      byCategory: [{ ...CAT_A, project_count: 3 }],
      danhMuc: [],
      totalsApply: true,
      filtered: false,
    });
    expect(g!.detail).toBe("2 dự án");
  });

  it("a category the summary does not list is still a group, named from the catalogue lookup", () => {
    const groups = groupProjects([project("DA9", "HM-X")], { byCategory: [CAT_A], danhMuc: [], totalsApply: true, filtered: false });
    expect(groups.map((g) => g.key)).toEqual(["HM-X"]);
    expect(groups[0]!.title).toContain("HM-X");
  });

  it("a row the catalogue no longer lists and the no-category row are named, never blank", () => {
    const groups = groupProjects([project("DA1", ""), project("DA2", "HM-OLD")], {
      byCategory: [category({ project_count: 1 }), category({ category_id: "HM-OLD", in_catalogue: false, project_count: 1 })],
      danhMuc: [],
      totalsApply: true,
      filtered: false,
    });
    expect(groups.map((g) => g.title)).toEqual(["Chưa gắn hạng mục", "Hạng mục không còn trong danh mục"]);
  });
});

/* ── §8 marker and §8.3 Biểu đồ ──────────────────────────────────────────────────────────── */

describe("§8 detail: elapsed-time marker", () => {
  it("marker at the SERVER's `time_elapsed_ratio`, and the caption names both figures", () => {
    const el = mount(<ThongTinDuAn duAn={project("DA1", "HM-A", { time_elapsed_ratio: 7096 })} />);
    const marker = el.querySelector<HTMLElement>("[data-elapsed-marker]")!;
    expect(marker.style.left).toBe("70.96%");
    expect(el.querySelector("[data-progress-caption]")?.textContent).toBe(
      "Giải ngân 90,00% · thời gian đã trôi qua 70,96%",
    );
  });

  it("no `time_elapsed_ratio` → no marker and no invented figure", () => {
    const el = mount(<ThongTinDuAn duAn={project("DA1", "HM-A")} />);
    expect(el.querySelector("[data-elapsed-marker]")).toBeNull();
    expect(el.querySelector("[data-progress-caption]")?.textContent).toBe("Giải ngân 90,00%");
  });

  it("no plan: the words say so, still with the elapsed share", () => {
    const el = mount(<ThongTinDuAn duAn={project("DA1", "HM-A", { disbursed_ratio: null, time_elapsed_ratio: 10000 })} />);
    expect(el.querySelector("[data-elapsed-marker]")).toBeNull();
    expect(el.querySelector("[data-progress-caption]")?.textContent).toBe(
      "Chưa bố trí vốn · thời gian đã trôi qua 100,00%",
    );
  });
});

describe("§8.3 Biểu đồ tab — the project's curve from the server", () => {
  function stubDetail(curve: () => Response): Seen[] {
    const seen: Seen[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        seen.push({ method: init?.method ?? "GET", url });
        if (url === "/api/v1/investment-projects/DA%2F1") return json(200, project("DA/1", "HM-A"));
        if (url === "/api/v1/investment-projects/DA%2F1/disbursements") {
          return json(200, { project_id: "DA/1", items: [], count: 0 });
        }
        if (url === "/api/v1/investment-projects/DA%2F1/disbursement-curve") return curve();
        return new Response("unexpected", { status: 500 });
      }),
    );
    return seen;
  }
  const curveReads = (seen: Seen[]) => seen.filter((s) => s.url.endsWith("/disbursement-curve"));

  it("reads the curve only when opened, draws it, and says where the plan line flattens", async () => {
    const seen = stubDetail(() =>
      json(200, {
        project_id: "DA/1",
        year: 2026,
        points: [point(4, 25_000_000, 0), point(5, 50_000_000, 10_000_000), point(6, 100_000_000, null)],
        expected_end_month: 6,
        disbursed_after_year: 5_000_000,
      }),
    );
    const el = mount(<ChiTietDuAn id="DA/1" />);
    await settle();
    expect(curveReads(seen)).toHaveLength(0);

    act(() => el.querySelector<HTMLButtonElement>("#tab-bieu-do-du-an")!.click());
    await settle();
    expect(curveReads(seen)).toHaveLength(1);
    const panel = el.querySelector("#panel-bieu-do-du-an")!;
    expect(panel.querySelector("[data-cumulative-chart]")).not.toBeNull();
    // Points start at the project's own start month; the line stops at the null month.
    expect([...panel.querySelectorAll("svg text")].map((t) => t.textContent)).toEqual(
      expect.arrayContaining(["T4", "T5", "T6"]),
    );
    expect(panel.querySelectorAll("circle[data-dot]")).toHaveLength(2);
    expect(panel.textContent).toContain("ở T6");
    expect(panel.textContent).toContain("5.000.000 đ chi sau ngày 31/12/2026");

    // Back to Chứng từ and again to Biểu đồ: a fresh read, so a voucher written meanwhile shows.
    act(() => el.querySelector<HTMLButtonElement>("#tab-chung-tu-du-an")!.click());
    act(() => el.querySelector<HTMLButtonElement>("#tab-bieu-do-du-an")!.click());
    await settle();
    expect(curveReads(seen)).toHaveLength(2);
  });

  it("a refused read is the server's sentence with a retry, never an empty chart", async () => {
    stubDetail(() => json(404, { code: "not_found", message: "Không tìm thấy dự án." }));
    const el = mount(<ChiTietDuAn id="DA/1" />);
    await settle();
    act(() => el.querySelector<HTMLButtonElement>("#tab-bieu-do-du-an")!.click());
    await settle();
    const panel = el.querySelector("#panel-bieu-do-du-an")!;
    expect(panel.textContent).toContain("Chưa tải được biểu đồ giải ngân");
    expect(panel.textContent).toContain("Không tìm thấy dự án.");
    expect(panel.querySelector("[data-cumulative-chart]")).toBeNull();
  });
});
