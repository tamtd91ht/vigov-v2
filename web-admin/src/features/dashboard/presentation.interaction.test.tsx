// @vitest-environment jsdom
//
// jsdom: "Trình chiếu" (spec 01 §2, user decision 07/10/2026) is a MODE of the page — a full-screen
// dark overlay, larger type, keyboard stops per block. What breaks silently is the plumbing around it:
// the page behind still scrolling or still reachable by Tab, Esc swallowed from an open "?", the
// body left locked after a drill-down navigates away. Each is pinned here on the mounted page.

import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { petitions_taskSummaryOut } from "@/lib/api/schema.gen";

import { DashboardPage } from "./overview";
import { periodWindows } from "./period";
import { PRESENTATION_LABEL, PresentationContext } from "./presentation";
import { blockVisibility, DashboardBlocks, DashboardHeader, DashboardView } from "./view";
import type { DashboardData, SummaryPair } from "./view";

const COMMUNE = { displayName: "Xã Kiểm Thử", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" };
const PERMISSIONS = ["report.read", "task.read", "document.read", "feedback.read", "budget.read"];

vi.mock("@/lib/api/phien", () => ({
  layPhienHienTai: () => Promise.resolve({ ok: true, duLieu: { permissions: PERMISSIONS } }),
}));
vi.mock("@/features/mat-khau/bat-doi-mat-khau", () => ({ duongDanBatDoiMatKhau: () => null }));

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

beforeEach(() => {
  // Never answers: the blocks stay "loading", which is all the mode needs — every block is drawn.
  vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  document.body.style.overflow = "";
  vi.unstubAllGlobals();
});

/**
 * The real page inside a stand-in of the signed-in shell (`app/tong-quan/page.tsx`): the navy header
 * and the sidebar are SIBLINGS of `<main>`, which is exactly what the overlay must make unreachable.
 */
async function mountPage(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  await act(async () => {
    r.render(
      <CauHinhXaProvider giaTri={COMMUNE}>
        <PhienProvider>
          <div className="khung-trang">
            <header data-testid="shell-header">
              <a href="/tim-kiem">Tìm kiếm</a>
            </header>
            <nav data-testid="shell-sidebar">
              <a href="/nhiem-vu">Nhiệm vụ</a>
            </nav>
            <main className="than-trang">
              <DashboardPage />
            </main>
          </div>
        </PhienProvider>
      </CauHinhXaProvider>,
    );
  });
  await act(async () => {
    await Promise.resolve();
  });
  return host;
}

function toggle(el: ParentNode): HTMLButtonElement {
  const found = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Trình chiếu");
  if (found === undefined) throw new Error('no button "Trình chiếu"');
  return found;
}

function frame(el: ParentNode): HTMLElement | null {
  return el.querySelector<HTMLElement>('[data-presentation="on"]');
}

function press(key: string) {
  const target = document.activeElement ?? document.body;
  act(() => {
    target.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
  });
}

function currentBlock(el: ParentNode): string | null {
  return el.querySelector<HTMLElement>("[data-block][data-current]")?.dataset.block ?? null;
}

describe("Trình chiếu — the toggle", () => {
  it("is a real toggle named 'Chế độ trình chiếu phòng họp', not the pending '?'", async () => {
    const el = await mountPage();
    const b = toggle(el);
    expect(b.disabled).toBe(false);
    expect(b.getAttribute("aria-label")).toBe(PRESENTATION_LABEL);
    expect(b.getAttribute("aria-pressed")).toBe("false");
    expect(b.closest("[data-pending]")).toBeNull();
    expect(el.querySelector(`button[aria-label="${pendingMarkerLabel(PRESENTATION_LABEL)}"]`)).toBeNull();
    expect(frame(el)).toBeNull();
  });

  it("pressing it opens the dark full-screen overlay; the shell behind is locked and unreachable", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    expect(toggle(el).getAttribute("aria-pressed")).toBe("true");
    const f = frame(el)!;
    expect(f).not.toBeNull();
    expect(f.className.split(" ")).toEqual(expect.arrayContaining(["fixed", "inset-0", "z-40", "overflow-auto"]));
    // every block and the header with the toggle are INSIDE the overlay
    expect(f.querySelector('[data-block="tasks"]')).not.toBeNull();
    expect(f.querySelector('[data-block="urgent"]')).not.toBeNull();
    expect(f.contains(toggle(el))).toBe(true);
    expect(document.body.style.overflow).toBe("hidden");
    expect(el.querySelector('[data-testid="shell-header"]')!.hasAttribute("inert")).toBe(true);
    expect(el.querySelector('[data-testid="shell-sidebar"]')!.hasAttribute("inert")).toBe(true);
    // the overlay itself, and the chain of its ancestors, stay live
    expect(f.closest("[inert]")).toBeNull();
    expect(f.contains(document.activeElement)).toBe(true);
  });

  it("pressing it again closes: no overlay, scroll and the shell given back, focus on the toggle", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    act(() => toggle(el).click());
    expect(frame(el)).toBeNull();
    expect(toggle(el).getAttribute("aria-pressed")).toBe("false");
    expect(document.body.style.overflow).toBe("");
    expect(el.querySelector("[inert]")).toBeNull();
  });
});

describe("Trình chiếu — the keyboard", () => {
  it("→ / PageDown / ← / PageUp move the current block; Home / End jump to the first / last", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    expect(currentBlock(el)).toBeNull();
    press("ArrowRight");
    expect(currentBlock(el)).toBe("tasks");
    press("ArrowRight");
    expect(currentBlock(el)).toBe("documents");
    press("PageDown");
    expect(currentBlock(el)).toBe("budget");
    press("ArrowLeft");
    expect(currentBlock(el)).toBe("documents");
    press("PageUp");
    expect(currentBlock(el)).toBe("tasks");
    press("ArrowLeft");
    expect(currentBlock(el)).toBe("tasks");
    press("End");
    expect(currentBlock(el)).toBe("urgent");
    press("ArrowRight");
    expect(currentBlock(el)).toBe("urgent");
    press("Home");
    expect(currentBlock(el)).toBe("tasks");
    // the current block takes focus, so a screen reader announces the block it lands on
    expect(document.activeElement?.getAttribute("data-block")).toBe("tasks");
  });

  it("the keys do nothing when the mode is off", async () => {
    const el = await mountPage();
    press("ArrowRight");
    expect(currentBlock(el)).toBeNull();
  });

  it("Esc closes the mode, focus returns to the toggle, the body scrolls again", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    press("ArrowRight");
    press("Escape");
    expect(frame(el)).toBeNull();
    expect(document.activeElement).toBe(toggle(el));
    expect(document.body.style.overflow).toBe("");
    expect(el.querySelector("[inert]")).toBeNull();
    // reopened, it starts with no current block
    act(() => toggle(el).click());
    expect(currentBlock(el)).toBeNull();
  });

  it("an open '?' owns the keys: Esc closes the description, not the mode; arrows do not move", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    const marker = el.querySelector<HTMLButtonElement>(
      `button[aria-label="${pendingMarkerLabel("Kinh tế & Tài nguyên")}"]`,
    )!;
    act(() => marker.click());
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull();
    press("ArrowRight");
    expect(currentBlock(el)).toBeNull();
    press("Escape");
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
    expect(frame(el)).not.toBeNull();
  });

  it("leaving the page while presenting (a drill-down link) releases the body and the shell", async () => {
    const el = await mountPage();
    act(() => toggle(el).click());
    expect(document.body.style.overflow).toBe("hidden");
    act(() => root?.unmount());
    root = null;
    expect(document.body.style.overflow).toBe("");
    expect(el.querySelector("[inert]")).toBeNull();
  });
});

/* ── what the mode draws ─────────────────────────────────────────────────────────────────── */

const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });
const ZERO: petitions_taskSummaryOut = { in_progress: 0, overdue: 0, suspended: 0, completed: 0, on_time_sample: 0, on_time: 0 };
const none: SummaryPair<never> = { current: null, previous: null };

function data(tasks: SummaryPair<petitions_taskSummaryOut>): DashboardData {
  return {
    windows: periodWindows("month", new Date("2026-09-28T09:43:00Z")),
    fetchedAt: Date.parse("2026-09-28T09:43:00Z"),
    tasks,
    incomingDocuments: none,
    citizenReports: none,
    fiscal: null,
    fiscalYear: 2026,
    queue: ok({ rows: [], failures: [] }),
    taskTypeLabels: null,
  };
}

const TASK_KEYS = blockVisibility(["report.read", "task.read"]);

function render(d: DashboardData, on: boolean): string {
  return renderToStaticMarkup(
    <DashboardView data={d} visible={TASK_KEYS} onPeriodChange={() => {}} presentation={{ on, onChange: () => {} }} />,
  );
}

function taskTiles(html: string): string {
  return /data-block="tasks".*?<ul class="([^"]*)"/s.exec(html)?.[1] ?? "";
}

describe("Trình chiếu — what it draws", () => {
  it("the overlay carries the dark token scope and every tile grid is 2 columns", () => {
    const html = render(data({ current: ok(ZERO), previous: ok(ZERO) }), true);
    expect(html).toContain('data-presentation="on"');
    // tasks has five figures: 3 columns normally, ALWAYS 2 here (prototype DashboardWorkspace:230-236)
    expect(taskTiles(html)).toContain("grid-cols-2");
    expect(taskTiles(html)).not.toContain("grid-cols-3");
    expect(taskTiles(render(data({ current: ok(ZERO), previous: ok(ZERO) }), false))).toContain("@md:grid-cols-3");
  });

  it("'Kỳ trước: —' and 'Kỳ trước: 0' are hidden; a real comparison stays", () => {
    const real = data({ current: ok({ ...ZERO, completed: 9 }), previous: ok({ ...ZERO, completed: 8 }) });
    expect(render(real, true)).toContain("+12,5% so với kỳ trước");
    const fromZero = data({ current: ok({ ...ZERO, completed: 3 }), previous: ok(ZERO) });
    expect(render(fromZero, false)).toContain("Kỳ trước: 0");
    expect(render(fromZero, true)).not.toContain("Kỳ trước: 0");
    const failed = data({ current: ok({ ...ZERO, completed: 3 }), previous: { ok: false, thongBao: "Mạng hỏng." } });
    expect(render(failed, false)).toContain("Kỳ trước: —");
    expect(render(failed, true)).not.toContain("Kỳ trước: —");
    // the failed previous call is still SAID — hiding the empty line must not hide the error
    expect(render(failed, true)).toContain("Không tải được số liệu kỳ trước: Mạng hỏng.");
  });

  it("'Cần xử lý ngay' and the '?' blocks stay in the mode", () => {
    const html = render(data(none), true);
    expect(html).toContain('data-block="urgent"');
    expect(html).toContain(pendingMarkerLabel("Kinh tế & Tài nguyên").replaceAll("&", "&amp;"));
  });

  it("the gate-closed header draws the toggle DISABLED, with no '?' and no overlay", () => {
    const html = renderToStaticMarkup(<DashboardHeader />);
    expect(html).toMatch(/<button[^>]*aria-label="Chế độ trình chiếu phòng họp"[^>]*disabled=""/);
    expect(html).not.toContain(pendingMarkerLabel(PRESENTATION_LABEL));
    expect(html).not.toContain('data-presentation="on"');
  });

  it("/bao-cao's blocks never take the mode, even inside a presenting context", () => {
    const html = renderToStaticMarkup(
      <PresentationContext.Provider value={{ on: true, current: "tasks" }}>
        <DashboardBlocks data={data({ current: ok(ZERO), previous: ok(ZERO) })} visible={TASK_KEYS} onReload={() => {}} layout="report" />
      </PresentationContext.Provider>,
    );
    expect(html).not.toContain("data-current");
    expect(html).not.toContain("40px");
    expect(taskTiles(html)).toContain("grid-cols-2");
  });

  it("a stateful page: the mode follows the toggle (controlled from the hook half)", () => {
    function Harness() {
      const [on, setOn] = useState(false);
      return <DashboardView data={data(none)} visible={TASK_KEYS} onPeriodChange={() => {}} presentation={{ on, onChange: setOn }} />;
    }
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<Harness />));
    act(() => toggle(host!).click());
    expect(frame(host)).not.toBeNull();
  });
});
