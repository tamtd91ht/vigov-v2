// @vitest-environment jsdom
//
// jsdom: the report job's card must come from the server and save/run like the others, and a job's
// switch must save at once and fall back when refused — events, not markup.

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

/** `scheduled_reports` as identity returns it (ADR 0086 B2): weekday + HH:MM, kind monthly_and_weekly. */
const REPORTS = job({
  job: "scheduled_reports",
  schedule_kind: "monthly_and_weekly",
  interval_minutes: null,
  min_interval_minutes: null,
  run_hour: 7,
  run_minute: 45,
  weekday: 1,
});

const FOUR_JOBS: readonly AutomationJob[] = [
  job({}),
  job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, run_hour: 7, run_minute: 0 }),
  job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }),
  REPORTS,
];

function mount(jobs: readonly AutomationJob[] = FOUR_JOBS): HTMLDivElement {
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

describe("Tự động hoá — 'Gửi báo cáo định kỳ' is a live card from the server (ADR 0086 B)", () => {
  it("is drawn from the server's 4th job, last, with an ENABLED switch and no '?'", () => {
    const el = mount();
    const headings = [...el.querySelectorAll("h3")].map((h) => h.textContent);
    expect(headings).toHaveLength(4);
    expect(headings.at(-1)).toBe("Gửi báo cáo định kỳ");
    expect(el.textContent).toContain("Báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1.");
    const sw = el.querySelector<HTMLButtonElement>("#tu-dong-hoa-scheduled_reports-bat")!;
    expect(sw.disabled).toBe(false);
    expect(el.querySelector(`button[aria-label="${pendingMarkerLabel("Gửi báo cáo định kỳ")}"]`)).toBeNull();
    expect(el.querySelector("[data-pending]")).toBeNull();
  });

  it("no server job → no card: nothing is drawn the server did not return", () => {
    const el = mount(FOUR_JOBS.slice(0, 3));
    expect(el.textContent).not.toContain("Gửi báo cáo định kỳ");
  });

  it("has no placeholder for 'Tính lại số liệu Tổng quan' — the owner dropped that job (ADR 0053, 0086 C)", () => {
    expect(mount().textContent).not.toContain("Tính lại số liệu");
  });

  it("on: weekday + HH:MM inputs and the last-run line, like the weekly job", () => {
    const el = mount([
      {
        ...REPORTS,
        configured: true,
        enabled: true,
        enabled_at: "2026-10-01T00:00:00Z",
        last_runs: [
          {
            work_kind: "nhiem-vu",
            run_id: "01RUN",
            trigger: "schedule",
            claimed_at: "2026-10-05T00:45:00Z",
            outcome: "succeeded",
            records_examined: 3,
            notices_delivered: 1,
            records_without_recipient: 0,
            recorded_at: "2026-10-05T00:45:01Z",
          },
        ],
      },
    ]);
    expect(el.querySelector<HTMLSelectElement>("#tu-dong-hoa-scheduled_reports-thu")!.value).toBe("1");
    expect(el.querySelector<HTMLInputElement>("#tu-dong-hoa-scheduled_reports-gio")!.value).toBe("07:45");
    expect(el.querySelector("#tu-dong-hoa-scheduled_reports-nhip")).toBeNull();
    expect(el.textContent).toContain("Chạy lần cuối");
  });

  it("switching on sends weekday + hour + minute for monthly_and_weekly, interval null", async () => {
    api.saveAutomationJob.mockResolvedValue({ ok: true, duLieu: { ...REPORTS, configured: true, enabled: true } });
    const el = mount([REPORTS]);
    act(() => el.querySelector<HTMLButtonElement>("#tu-dong-hoa-scheduled_reports-bat")!.click());
    expect(api.saveAutomationJob).toHaveBeenCalledWith("scheduled_reports", {
      enabled: true,
      interval_minutes: null,
      run_hour: 7,
      run_minute: 45,
      weekday: 1,
    });
    await settle();
    expect(toasts.success).toHaveBeenCalledWith("Đã lưu: Gửi báo cáo định kỳ");
  });

  it("'Lưu nhịp' after changing the weekday sends the new weekday", async () => {
    const on = { ...REPORTS, configured: true, enabled: true, enabled_at: "2026-10-01T00:00:00Z" };
    api.saveAutomationJob.mockResolvedValue({ ok: true, duLieu: { ...on, weekday: 3 } });
    const el = mount([on]);
    const select = el.querySelector<HTMLSelectElement>("#tu-dong-hoa-scheduled_reports-thu")!;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!;
      setter.call(select, "3");
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const save = [...el.querySelectorAll("button")].find((b) => b.textContent === "Lưu nhịp")!;
    act(() => save.click());
    expect(api.saveAutomationJob).toHaveBeenCalledWith("scheduled_reports", {
      enabled: true,
      interval_minutes: null,
      run_hour: 7,
      run_minute: 45,
      weekday: 3,
    });
    await settle();
    expect(toasts.success).toHaveBeenCalledWith("Đã lưu: Gửi báo cáo định kỳ");
  });

  it("'Chạy ngay' posts the run request for scheduled_reports and draws the request line", async () => {
    const on = { ...REPORTS, configured: true, enabled: true, enabled_at: "2026-10-01T00:00:00Z" };
    api.requestAutomationRun.mockResolvedValue({ ok: true, duLieu: { ...on, run_requested_at: "2026-10-09T02:00:00Z" } });
    const el = mount([on]);
    const run = [...el.querySelectorAll("button")].find((b) => b.textContent === "Chạy ngay")!;
    act(() => run.click());
    expect(api.requestAutomationRun).toHaveBeenCalledWith("scheduled_reports");
    await settle();
    expect(toasts.success).toHaveBeenCalledWith("Đã ghi yêu cầu chạy ngay: Gửi báo cáo định kỳ");
    expect(el.textContent).toContain("Đã ghi yêu cầu chạy ngay lúc");
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
