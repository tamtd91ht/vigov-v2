// @vitest-environment jsdom
//
// jsdom: `/bao-cao`'s export buttons are pressed (owner 09/10/2026, D3). The builders are mocked (they
// run for real in `report-export.test.ts`); what is tested is the gate, the click, the file name and
// the two toasts.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { DashboardExport } from "@/features/dashboard/export-model";
import type { BlocksData } from "@/features/dashboard/view";
import type { petitions_taskSummaryOut } from "@/lib/api/schema.gen";

import { reportAccess } from "./report-access";
import { REPORT_EXPORT_FAILED } from "./report-export-actions";
import { reportWindows } from "./report-period";
import { ReportView } from "./report-view";

const builders = vi.hoisted(() => ({
  pdf: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
  xlsx: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
  pptx: vi.fn<(doc: DashboardExport) => Promise<Blob>>(),
}));
vi.mock("@/features/dashboard/export-files", () => ({ EXPORT_BUILDERS: builders }));
const saveFile = vi.hoisted(() => vi.fn());
vi.mock("@/features/nhiem-vu/save-file", () => ({ saveFile }));
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  for (const b of Object.values(builders)) b.mockReset();
  saveFile.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

const AT = Date.parse("2026-09-28T09:43:00Z");
const MONTH = reportWindows({ kind: "month" }, new Date(AT));
const TASKS: petitions_taskSummaryOut = { in_progress: 3, overdue: 1, suspended: 0, completed: 2, on_time_sample: 2, on_time: 1 };
const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });
const PERMS = ["report.read", "report.export", "task.read"];

function figures(loaded = true): BlocksData {
  const none = { current: null, previous: null };
  return {
    windows: MONTH,
    tasks: loaded ? { current: ok(TASKS), previous: ok(TASKS) } : none,
    incomingDocuments: none,
    citizenReports: none,
    fiscal: null,
    fiscalYear: 2026,
    budget: null,
    queue: null,
    taskTypeLabels: null,
  };
}

/** `commune = null` = no runtime configuration reached the page. */
function mount(permissions = PERMS, loaded = true, commune: { displayName: string; parentAuthority: string } | null = { displayName: "Xã Kiểm Thử", parentAuthority: "" }) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <ReportView
        windows={MONTH}
        fetchedAt={AT}
        figures={figures(loaded)}
        access={reportAccess(permissions)}
        units={{ ok: true, duLieu: [] }}
        commune={commune ?? undefined}
        onNamedPeriod={() => {}}
        onCustomPeriod={() => {}}
        onReload={() => {}}
        onReloadUnits={() => {}}
      />,
    ),
  );
  return host;
}

const buttons = (el: ParentNode) => [...el.querySelectorAll<HTMLButtonElement>("button[data-export-format]")];
const button = (el: ParentNode, f: "pdf" | "xlsx" | "pptx") =>
  el.querySelector<HTMLButtonElement>(`button[data-export-format="${f}"]`)!;

async function settle() {
  await act(async () => {
    for (let i = 0; i < 5; i++) await Promise.resolve();
  });
}

describe("Báo cáo export — the gate", () => {
  it("ALLOWED: report.read + report.export, figures loaded → three enabled 'Xuất …' buttons", () => {
    const el = mount();
    expect(buttons(el).map((b) => b.textContent)).toEqual(["Xuất PDF", "Xuất XLSX", "Xuất PPTX"]);
    for (const b of buttons(el)) expect(b.disabled).toBe(false);
  });

  it("DENIED: without report.export there is no button to press", () => {
    const el = mount(["report.read", "task.read"]);
    expect(buttons(el)).toHaveLength(0);
  });

  it("LOADING: disabled, nothing built", () => {
    const el = mount(PERMS, false);
    for (const b of buttons(el)) expect(b.disabled).toBe(true);
    act(() => button(el, "pdf").click());
    expect(builders.pdf).not.toHaveBeenCalled();
  });

  it("NO COMMUNE CONFIGURATION: disabled", () => {
    const el = mount(PERMS, true, null);
    for (const b of buttons(el)) expect(b.disabled).toBe(true);
  });
});

describe("Báo cáo export — the click", () => {
  it("builds from the report's figures, saves bao-cao-<period>.<ext>, toasts 'Đã tải …'", async () => {
    const blob = new Blob(["x"]);
    builders.xlsx.mockResolvedValue(blob);
    const el = mount();
    act(() => button(el, "xlsx").click());
    await settle();
    const doc = builders.xlsx.mock.calls[0]![0];
    expect(doc.title).toBe("Báo cáo điều hành");
    expect(doc.communeName).toBe("Xã Kiểm Thử");
    expect(doc.blocks[0]!.figures.map((f) => f.value)).toEqual(["3", "1", "2", "0", "50,0%"]);
    expect(saveFile).toHaveBeenCalledWith(blob, "bao-cao-thang-20260901.xlsx");
    expect(toast.success).toHaveBeenCalledWith("Đã tải bao-cao-thang-20260901.xlsx.");
  });

  it("one file at a time: every button waits with the spinner while one is made", async () => {
    let finish: (b: Blob) => void = () => {};
    builders.pdf.mockReturnValue(new Promise<Blob>((r) => (finish = r)));
    const el = mount();
    act(() => button(el, "pdf").click());
    await settle();
    for (const b of buttons(el)) {
      expect(b.disabled).toBe(true);
      expect(b.querySelector("svg")?.getAttribute("class")).toContain("animate-spin");
    }
    await act(async () => finish(new Blob(["%PDF"])));
    await settle();
    for (const b of buttons(el)) expect(b.disabled).toBe(false);
  });

  it("a failed build toasts 'Không xuất được báo cáo.' and saves nothing", async () => {
    builders.pptx.mockRejectedValue(new Error("boom"));
    const el = mount();
    act(() => button(el, "pptx").click());
    await settle();
    expect(saveFile).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalledWith(REPORT_EXPORT_FAILED);
    expect(REPORT_EXPORT_FAILED).toBe("Không xuất được báo cáo.");
  });
});
