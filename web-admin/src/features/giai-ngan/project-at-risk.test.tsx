// @vitest-environment jsdom
//
// jsdom for this file: ticking the box and pressing `Lưu dự án` are events, and what matters is the
// PATCH body that leaves the browser — a markup string cannot show either.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { finance_duAnRa } from "@/lib/api/schema.gen";

import { BangDanhSach } from "./bang-du-an";
import { ChiTietDuAn, ThongTinDuAn } from "./chi-tiet-du-an";
import { giaTriTuDuAn } from "./ghi-du-an";
import { thanSuaDuAn } from "./nhan-ghi-giai-ngan";

/**
 * "Nguy cơ không giải ngân hết" (ADR 0081 #2): a flag a `budget.update` holder ticks in `Sửa dự án`,
 * shown as a word badge on the list row and the project card. The card count is in
 * `disbursement-overview.test.tsx`.
 */

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));

const session = { ok: true as const, duLieu: { permissions: [] as string[] } };
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => session,
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

function project(over: Partial<finance_duAnRa> = {}): finance_duAnRa {
  return {
    id: "DA1",
    code: "DA01",
    year: 2026,
    category_id: "HM-A",
    name: "Dự án mẫu",
    planned_amount: 100_000_000,
    approved_amount: 100_000_000,
    disbursed_amount: 40_000_000,
    remaining_amount: 60_000_000,
    disbursed_ratio: 4000,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...over,
  };
}

const LIST = (items: finance_duAnRa[]) => ({ items, year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" });

describe("badges — drawn only for a project whose flag is on", () => {
  it("list row: the prototype's tangerine words under the name", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach duLieu={LIST([project({ at_risk: true })])} danhMuc={[]} />,
    );
    expect(html).toMatch(/<span class="text-tangerine font-semibold" data-at-risk="">nguy cơ không giải ngân hết<\/span>/);
  });

  it("list row: flag off or absent → no badge", () => {
    for (const p of [project({ at_risk: false }), project()]) {
      const html = renderToStaticMarkup(<BangDanhSach duLieu={LIST([p])} danhMuc={[]} />);
      expect(html).not.toContain("data-at-risk");
      expect(html).not.toContain("nguy cơ không giải ngân hết");
    }
  });

  it("project card: the badge beside the progress pill, in the tangerine tokens", () => {
    const html = renderToStaticMarkup(<ThongTinDuAn duAn={project({ at_risk: true })} />);
    expect(html).toMatch(/<span class="[^"]*bg-tangerine\/12[^"]*text-tangerine[^"]*" data-at-risk="">Nguy cơ không giải ngân hết<\/span>/);
    expect(html).toContain("border-tangerine/25");
  });

  it("project card: flag off or absent → no badge", () => {
    for (const p of [project({ at_risk: false }), project()]) {
      expect(renderToStaticMarkup(<ThongTinDuAn duAn={p} />)).not.toContain("Nguy cơ không giải ngân hết");
    }
  });
});

describe("thanSuaDuAn — `at_risk` goes out only when the box was toggled", () => {
  it("toggled on, toggled off, untouched", () => {
    const off = giaTriTuDuAn(project({ at_risk: false }));
    expect(thanSuaDuAn(off, { ...off, atRisk: true })).toEqual({ ok: true, than: { at_risk: true } });
    const on = giaTriTuDuAn(project({ at_risk: true }));
    expect(on.atRisk).toBe(true);
    expect(thanSuaDuAn(on, { ...on, atRisk: false })).toEqual({ ok: true, than: { at_risk: false } });
    // Untouched box + another field: no `at_risk` key at all.
    const kq = thanSuaDuAn(on, { ...on, ten: "Tên mới" });
    expect(kq).toEqual({ ok: true, than: { name: "Tên mới" } });
  });

  it("an older reply without the field reads as not flagged", () => {
    expect(giaTriTuDuAn(project()).atRisk).toBe(false);
  });
});

describe("Sửa dự án page — the box behind `budget.update`", () => {
  type Sent = { method: string; url: string; body: unknown };

  function stubDetail(p: finance_duAnRa): Sent[] {
    const sent: Sent[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        const method = init?.method ?? "GET";
        sent.push({ method, url, body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined });
        if (url === "/api/v1/investment-projects/DA1" && method === "GET") return json(200, p);
        if (url === "/api/v1/investment-projects/DA1" && method === "PATCH") {
          return json(200, { ...p, at_risk: !(p.at_risk === true) });
        }
        if (url === "/api/v1/capital-plan-categories") {
          return json(200, {
            items: [{ id: "HM-A", code: "a", label: "Hạng mục A", is_default: false, active: true, order: 1, source: "don-vi", tier: 1 }],
          });
        }
        return new Response("unexpected", { status: 500 });
      }),
    );
    return sent;
  }

  const patches = (sent: Sent[]) => sent.filter((s) => s.method === "PATCH");

  it("with `budget.update`: tick the box, save → the PATCH body is exactly `{ at_risk: true }`", async () => {
    session.duLieu.permissions = ["budget.read", "budget.update"];
    const sent = stubDetail(project({ at_risk: false }));
    const el = mount(<ChiTietDuAn id="DA1" />);
    await settle();

    const edit = [...el.querySelectorAll("button")].find((b) => b.textContent === "Sửa dự án")!;
    act(() => edit.click());
    await settle();
    const box = el.querySelector<HTMLInputElement>("#nguy-co-du-an")!;
    expect(box).not.toBeNull();
    expect(box.checked).toBe(false);
    expect(box.disabled).toBe(false);

    act(() => box.click());
    expect(box.checked).toBe(true);
    const save = [...el.querySelectorAll<HTMLButtonElement>('button[type="submit"]')].find(
      (b) => b.textContent === "Lưu dự án",
    )!;
    act(() => save.click());
    await settle();

    expect(patches(sent)).toEqual([{ method: "PATCH", url: "/api/v1/investment-projects/DA1", body: { at_risk: true } }]);
  });

  it("with `budget.update`: saving with the box untouched sends no `at_risk`", async () => {
    session.duLieu.permissions = ["budget.read", "budget.update"];
    const sent = stubDetail(project({ at_risk: true }));
    const el = mount(<ChiTietDuAn id="DA1" />);
    await settle();
    act(() => [...el.querySelectorAll("button")].find((b) => b.textContent === "Sửa dự án")!.click());
    await settle();
    expect(el.querySelector<HTMLInputElement>("#nguy-co-du-an")!.checked).toBe(true);

    const name = el.querySelector<HTMLInputElement>("#ten-du-an")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(name, "Tên mới");
      name.dispatchEvent(new Event("input", { bubbles: true }));
    });
    act(() =>
      [...el.querySelectorAll<HTMLButtonElement>('button[type="submit"]')].find((b) => b.textContent === "Lưu dự án")!.click(),
    );
    await settle();
    expect(patches(sent)).toHaveLength(1);
    expect(patches(sent)[0]!.body).toEqual({ name: "Tên mới" });
  });

  it("WITHOUT `budget.update` (even with `budget.confirm`): no edit button, no box — the badge still shows", async () => {
    session.duLieu.permissions = ["budget.read", "budget.confirm"];
    const sent = stubDetail(project({ at_risk: true }));
    const el = mount(<ChiTietDuAn id="DA1" />);
    await settle();
    expect([...el.querySelectorAll("button")].some((b) => b.textContent === "Sửa dự án")).toBe(false);
    expect(el.querySelector("#nguy-co-du-an")).toBeNull();
    expect(el.querySelector("[data-at-risk]")?.textContent).toBe("Nguy cơ không giải ngân hết");
    expect(patches(sent)).toHaveLength(0);
  });
});
