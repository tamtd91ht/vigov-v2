// @vitest-environment jsdom
//
// jsdom: the "?" must open its description on click and reach no server — events, not markup.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { AutomationJob } from "@/lib/api/automation-jobs";

import { AutomationTabView } from "./automation-tab";
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

function mount(): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <AutomationTabView
        loaded={{
          ok: true,
          duLieu: [
            job({}),
            job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, run_hour: 7, run_minute: 0 }),
            job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }),
          ],
        }}
      />,
    ),
  );
  return host;
}

describe("Tự động hoá — 'Gửi báo cáo định kỳ' as a disabled card with '?' (ADR 0068 §14)", () => {
  it("is the LAST card, after the three live jobs, with a disabled switch", () => {
    const el = mount();
    const headings = [...el.querySelectorAll("h3")].map((h) => h.textContent);
    expect(headings.at(-1)).toBe("Gửi báo cáo định kỳ");
    expect(headings).toHaveLength(4);
    const sw = el.querySelector<HTMLInputElement>("#tu-dong-hoa-bao-cao-dinh-ky-bat")!;
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
  });
});
