// @vitest-environment jsdom
//
// jsdom: the "?" must open its description on click and reach no server, and a job's switch must
// save at once and fall back when refused — events, not markup.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { AutomationJob } from "@/lib/api/automation-jobs";

const api = vi.hoisted(() => ({
  saveAutomationJob: vi.fn(),
  requestAutomationRun: vi.fn(),
  listAutomationJobs: vi.fn(),
}));
const toasts = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock("@/lib/api/automation-jobs", () => api);
vi.mock("sonner", () => ({ toast: toasts }));

const { AutomationTabView } = await import("./automation-tab");
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

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
  vi.clearAllMocks();
});

function job(patch: Partial<AutomationJob>): AutomationJob {
  return {
    job: "sla_reminders",
    schedule_kind: "interval",
    configured: false,
    enabled: false,
    interval_minutes: 15,
    min_interval_minutes: 5,
    run_hour: null,
    run_minute: null,
    weekday: null,
    timezone: "Asia/Ho_Chi_Minh",
    enabled_at: null,
    run_requested_at: null,
    last_runs: [],
    ...patch,
  };
}

const THREE_JOBS: readonly AutomationJob[] = [
  job({}),
  job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, run_hour: 7, run_minute: 0 }),
  job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }),
];

function mount(jobs: readonly AutomationJob[] = THREE_JOBS): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<AutomationTabView loaded={{ ok: true, duLieu: jobs }} />));
  return host;
}

/** Lets the awaited save resolve and React commit what follows it. */
async function settle() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("Tự động hoá — 'Gửi báo cáo định kỳ' as a disabled card with '?' (ADR 0068 §14)", () => {
  it("is the LAST card, after the three live jobs, with a disabled switch", () => {
    const el = mount();
    const headings = [...el.querySelectorAll("h3")].map((h) => h.textContent);
    expect(headings.at(-1)).toBe("Gửi báo cáo định kỳ");
    expect(headings).toHaveLength(4);
    const sw = el.querySelector<HTMLButtonElement>("#tu-dong-hoa-bao-cao-dinh-ky-bat")!;
    expect(sw.disabled).toBe(true);
    expect(sw.getAttribute("role")).toBe("switch");
  });

  it("has no placeholder for 'Tính lại số liệu Tổng quan' — the owner dropped that job (ADR 0053)", () => {
    expect(mount().textContent).not.toContain("Tính lại số liệu");
  });

  it("the '?' names the part, opens the entry's description, and calls no server", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount();
    const marker = el.querySelector<HTMLButtonElement>(
      `button[aria-label="${pendingMarkerLabel("Gửi báo cáo định kỳ")}"]`,
    );
    expect(marker).not.toBeNull();
    act(() => marker!.click());
    const entry = PHAN_CHUA_DUNG.find((p) => p.ten === "Gửi báo cáo định kỳ")!;
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(entry.viSao);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(api.saveAutomationJob).not.toHaveBeenCalled();
  });
});

describe("Tự động hoá — the switch saves at once (spec 09)", () => {
  it("REGRESSION: switching on sends the PUT with the saved cadence, no 'Lưu' press, and toasts 'Đã lưu: …'", async () => {
    const saved = job({ configured: true, enabled: true, enabled_at: "2026-10-08T01:00:00Z" });
    api.saveAutomationJob.mockResolvedValue({ ok: true, duLieu: saved });
    const el = mount([job({})]);
    const sw = el.querySelector<HTMLButtonElement>("#tu-dong-hoa-sla_reminders-bat")!;
    expect(sw.getAttribute("aria-checked")).toBe("false");
    act(() => sw.click());
    expect(api.saveAutomationJob).toHaveBeenCalledWith("sla_reminders", {
      enabled: true,
      interval_minutes: 15,
      run_hour: null,
      run_minute: null,
      weekday: null,
    });
    await settle();
    expect(sw.getAttribute("aria-checked")).toBe("true");
    expect(toasts.success).toHaveBeenCalledWith("Đã lưu: Nhắc việc sắp đến hạn và đã quá hạn");
    // On → the cadence row and "Chạy ngay" appear; the card turns white.
    expect(el.textContent).toContain("Cứ mỗi");
    expect(el.querySelector("section section")?.className).toContain("bg-white");
  });

  it("refused → the switch goes back and the server's sentence is shown", async () => {
    api.saveAutomationJob.mockResolvedValue({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
    const el = mount([job({ configured: true, enabled: true, enabled_at: "2026-10-08T01:00:00Z" })]);
    const sw = el.querySelector<HTMLButtonElement>("#tu-dong-hoa-sla_reminders-bat")!;
    act(() => sw.click());
    expect(sw.getAttribute("aria-checked")).toBe("false");
    await settle();
    expect(sw.getAttribute("aria-checked")).toBe("true");
    expect(toasts.error).toHaveBeenCalledWith("Bạn không có quyền thực hiện thao tác này.");
    expect(toasts.success).not.toHaveBeenCalled();
  });

  it("switching off toasts 'Đã tắt: …'", async () => {
    api.saveAutomationJob.mockResolvedValue({ ok: true, duLieu: job({ configured: true, enabled: false }) });
    const el = mount([job({ configured: true, enabled: true, enabled_at: "2026-10-08T01:00:00Z" })]);
    act(() => el.querySelector<HTMLButtonElement>("#tu-dong-hoa-sla_reminders-bat")!.click());
    expect(api.saveAutomationJob.mock.calls[0]?.[1].enabled).toBe(false);
    await settle();
    expect(toasts.success).toHaveBeenCalledWith("Đã tắt: Nhắc việc sắp đến hạn và đã quá hạn");
    expect(el.textContent).not.toContain("Cứ mỗi");
  });

  it("'Lưu nhịp' appears on change; a refusal is shown in place and the saved cadence comes back", async () => {
    api.saveAutomationJob.mockResolvedValue({ ok: false, thongBao: "Nhịp nhắc việc phải từ 5 đến 10080 phút." });
    const el = mount([job({ configured: true, enabled: true, enabled_at: "2026-10-08T01:00:00Z" })]);
    const select = el.querySelector<HTMLSelectElement>("#tu-dong-hoa-sla_reminders-nhip")!;
    expect(el.textContent).not.toContain("Lưu nhịp");
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!;
      setter.call(select, "30");
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const save = [...el.querySelectorAll("button")].find((b) => b.textContent === "Lưu nhịp")!;
    expect(save).toBeDefined();
    act(() => save.click());
    expect(api.saveAutomationJob.mock.calls[0]?.[1].interval_minutes).toBe(30);
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Nhịp nhắc việc phải từ 5 đến 10080 phút.");
    expect(select.value).toBe("15");
    expect(el.textContent).not.toContain("Lưu nhịp");
  });
});
