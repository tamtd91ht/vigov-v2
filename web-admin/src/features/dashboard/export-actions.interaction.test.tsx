// @vitest-environment jsdom
//
// jsdom: the export buttons are pressed — enabled only with the figures loaded AND `report.export`,
// one file at a time, a Vietnamese sentence when a file cannot be made. The heavy builders are mocked
// here (they are exercised for real in `export.test.ts`); what is tested is the gate and the click.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { petitions_taskSummaryOut } from "@/lib/api/schema.gen";

import { EXPORT_FAILED, EXPORT_UNAVAILABLE } from "./export-actions";
import type { DashboardExport } from "./export-model";
import { periodWindows } from "./period";
import { blockVisibility, DashboardHeader, DashboardView, EXPORT_NEEDS_PERMISSION, EXPORT_WAITING } from "./view";
import type { DashboardData, ExportAccess } from "./view";

const builders = vi.hoisted(() => ({
  pdf: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
  xlsx: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
  pptx: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
}));
vi.mock("./export-files", () => ({ EXPORT_BUILDERS: builders }));
const saveFile = vi.hoisted(() => vi.fn());
vi.mock("@/features/nhiem-vu/save-file", () => ({ saveFile }));

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
  for (const b of Object.values(builders)) b.mockReset();
  saveFile.mockReset();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function mount(node: React.ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

const AT = Date.parse("2026-09-28T09:43:00Z");
const TASKS: petitions_taskSummaryOut = { in_progress: 3, overdue: 1, suspended: 0, completed: 2, on_time_sample: 2, on_time: 1 };
const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });
const TASK_KEYS = blockVisibility(["report.read", "task.read"]);

function loaded(): DashboardData {
  return {
    windows: periodWindows("month", new Date(AT)),
    fetchedAt: AT,
    fiscalYear: 2026,
    tasks: { current: ok(TASKS), previous: ok(TASKS) },
    incomingDocuments: { current: null, previous: null },
    citizenReports: { current: null, previous: null },
    fiscal: null,
    queue: ok({ rows: [], failures: [] }),
    taskTypeLabels: null,
  };
}

const ACCESS: ExportAccess = { commune: { displayName: "Xã Kiểm Thử", parentAuthority: "" }, canExport: true };

function view(data: DashboardData, exportAccess: ExportAccess | null = ACCESS): HTMLDivElement {
  return mount(
    <DashboardView
      data={data}
      visible={TASK_KEYS}
      onPeriodChange={() => {}}
      exportAccess={exportAccess === null ? undefined : exportAccess}
    />,
  );
}

function exportButtons(el: ParentNode): HTMLButtonElement[] {
  return [...el.querySelectorAll<HTMLButtonElement>("button[data-export-format]")];
}

function button(el: ParentNode, format: "pdf" | "xlsx" | "pptx"): HTMLButtonElement {
  return el.querySelector<HTMLButtonElement>(`button[data-export-format="${format}"]`)!;
}

async function settle() {
  await act(async () => {
    for (let i = 0; i < 5; i++) await Promise.resolve();
  });
}

describe("Tổng quan export — the gate", () => {
  it("ALLOWED: figures loaded + report.export → the three buttons are enabled, no '?'", () => {
    const el = view(loaded());
    expect(exportButtons(el).map((b) => b.textContent)).toEqual(["PDF", "XLSX", "PPTX"]);
    for (const b of exportButtons(el)) expect(b.disabled).toBe(false);
    expect(el.querySelector('[data-pending] button[data-export-format]')).toBeNull();
  });

  it("DENIED: without report.export the buttons are disabled and the reason is stated", () => {
    const el = view(loaded(), { ...ACCESS, canExport: false });
    for (const b of exportButtons(el)) expect(b.disabled).toBe(true);
    expect(el.textContent).toContain(EXPORT_NEEDS_PERMISSION);
    act(() => button(el, "pdf").click());
    expect(builders.pdf).not.toHaveBeenCalled();
  });

  it("LOADING: while a visible block has not answered, the buttons wait", () => {
    const el = view({ ...loaded(), tasks: { current: ok(TASKS), previous: null } });
    for (const b of exportButtons(el)) expect(b.disabled).toBe(true);
    expect(el.textContent).toContain(EXPORT_WAITING);
  });

  it("NO SOURCE (gate closed / session being read / no commune configuration): disabled", () => {
    const closed = mount(<DashboardHeader />);
    for (const b of exportButtons(closed)) expect(b.disabled).toBe(true);
    expect(closed.textContent).toContain(EXPORT_UNAVAILABLE);
    act(() => root?.unmount());
    const el = view(loaded(), null);
    for (const b of exportButtons(el)) expect(b.disabled).toBe(true);
  });

  it("Trình chiếu stays the pending '?'", () => {
    const el = view(loaded());
    const show = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Trình chiếu");
    expect(show?.disabled).toBe(true);
    expect(show?.closest("[data-pending]")).not.toBeNull();
  });
});

describe("Tổng quan export — the click", () => {
  it("builds from the figures on screen and saves tong-quan-<period>.<ext>", async () => {
    const blob = new Blob(["x"], { type: "application/vnd.openxmlformats-officedocument.presentationml.presentation" });
    builders.pptx.mockResolvedValue(blob);
    const el = view(loaded());
    act(() => button(el, "pptx").click());
    await settle();
    expect(builders.pptx).toHaveBeenCalledTimes(1);
    const doc = builders.pptx.mock.calls[0]![0];
    expect(doc.communeName).toBe("Xã Kiểm Thử");
    expect(doc.blocks[0]!.figures.map((f) => f.value)).toEqual(["3", "1", "2", "0", "50,0%"]);
    expect(saveFile).toHaveBeenCalledWith(blob, "tong-quan-thang-20260901.pptx");
  });

  it("one file at a time: the pressed button is busy, the others wait, all return after", async () => {
    let finish: (b: Blob) => void = () => {};
    builders.pdf.mockReturnValue(new Promise<Blob>((r) => (finish = r)));
    const el = view(loaded());
    act(() => button(el, "pdf").click());
    await settle();
    expect(button(el, "pdf").getAttribute("aria-busy")).toBe("true");
    expect(button(el, "xlsx").disabled).toBe(true);
    expect(button(el, "pptx").disabled).toBe(true);
    expect(el.textContent).toContain("Đang tạo tệp PDF…");
    await act(async () => finish(new Blob(["%PDF"], { type: "application/pdf" })));
    await settle();
    expect(saveFile).toHaveBeenCalledWith(expect.any(Blob), "tong-quan-thang-20260901.pdf");
    for (const b of exportButtons(el)) expect(b.disabled).toBe(false);
    expect(button(el, "pdf").hasAttribute("aria-busy")).toBe(false);
  });

  it("a failed build shows the Vietnamese sentence and saves nothing; the buttons come back", async () => {
    builders.xlsx.mockRejectedValue(new Error("boom"));
    const el = view(loaded());
    act(() => button(el, "xlsx").click());
    await settle();
    expect(saveFile).not.toHaveBeenCalled();
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(EXPORT_FAILED);
    for (const b of exportButtons(el)) expect(b.disabled).toBe(false);
  });
});
