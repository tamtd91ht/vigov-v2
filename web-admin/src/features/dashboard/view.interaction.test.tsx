// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS buttons. The period picker became a segmented control and
// the load error gained "Tải lại"; both must call the page's ONE existing handler with the values
// the old buttons sent — a restyle that changed what a click emits would pass every markup test.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { periodWindows } from "./period";
import type { PeriodKind } from "./period";
import { blockVisibility, DashboardView } from "./view";
import type { DashboardData, SummaryPair } from "./view";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function data(patch: Partial<DashboardData>): DashboardData {
  const none: SummaryPair<never> = { current: null, previous: null };
  return {
    windows: periodWindows("month", new Date("2026-09-28T09:43:00Z")),
    fetchedAt: Date.parse("2026-09-28T09:43:00Z"),
    tasks: none,
    incomingDocuments: none,
    citizenReports: none,
    fiscal: null,
    fiscalYear: 2026,
    budget: null,
    queue: { ok: true, duLieu: { rows: [], failures: [] } },
    taskTypeLabels: null,
    ...patch,
  };
}

function mount(d: DashboardData, onPeriodChange: (k: PeriodKind) => void): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <DashboardView
        data={d}
        visible={blockVisibility(["report.read", "task.read"])}
        onPeriodChange={onPeriodChange}
      />,
    ),
  );
  return host;
}

function button(el: HTMLElement, text: string): HTMLButtonElement {
  const found = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === text);
  if (found === undefined) throw new Error(`no button "${text}"`);
  return found;
}

describe("DashboardView — what a click emits", () => {
  it("each period segment emits its PeriodKind, the pressed one included", () => {
    const onPeriodChange = vi.fn();
    const el = mount(data({}), onPeriodChange);
    expect(button(el, "Tháng này").getAttribute("aria-pressed")).toBe("true");
    act(() => button(el, "Quý này").click());
    act(() => button(el, "Tuần này").click());
    act(() => button(el, "Năm nay").click());
    act(() => button(el, "Tháng này").click());
    expect(onPeriodChange.mock.calls).toEqual([["quarter"], ["week"], ["year"], ["month"]]);
  });

  it("'Tải lại' on a failed load re-asks the CURRENT period through the same handler", () => {
    const onPeriodChange = vi.fn();
    const el = mount(
      data({ tasks: { current: { ok: false, thongBao: "Mạng hỏng." }, previous: null } }),
      onPeriodChange,
    );
    act(() => button(el, "Tải lại").click());
    expect(onPeriodChange.mock.calls).toEqual([["month"]]);
  });
});
