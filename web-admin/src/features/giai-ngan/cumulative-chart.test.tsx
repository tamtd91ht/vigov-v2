// @vitest-environment jsdom
//
// jsdom for this file: the tooltip is driven by pointer and keyboard events — a markup string cannot
// show a month being hovered or an arrow key moving the reading.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it } from "vitest";

import type { finance_curvePointOut, finance_projectCurveOut } from "@/lib/api/schema.gen";

import { CumulativeChart } from "./cumulative-chart";
import { ProjectCurveView } from "./project-curve";

/**
 * G4a — the hover reading of the prototype (`CumulativeChart.tsx:71-86`): the exact amounts of one
 * month, in full đồng, by pointer AND by keyboard. G4b — the dashed `Hoàn thành dự kiến` line of the
 * prototype (`:88-100`) on a project's curve, at the server's `expected_end_month`.
 */

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

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function point(month: number, planned: number, disbursed: number | null): finance_curvePointOut {
  return { month, planned_cumulative: planned, disbursed_cumulative: disbursed };
}

const POINTS = [point(4, 25_000_000, 0), point(5, 50_000_000, 10_000_000), point(6, 100_000_000, null)];

const tooltip = (el: ParentNode) => el.querySelector<HTMLElement>("[data-chart-tooltip]");
const focusTarget = (el: ParentNode) => el.querySelector<HTMLElement>("[data-chart-focus]")!;

function hover(target: Element): void {
  act(() => {
    target.dispatchEvent(new MouseEvent("mouseover", { bubbles: true, relatedTarget: null }));
  });
}

function unhover(target: Element): void {
  act(() => {
    target.dispatchEvent(new MouseEvent("mouseout", { bubbles: true, relatedTarget: document.body }));
  });
}

function press(target: Element, key: string): void {
  act(() => {
    target.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

describe("G4a — tooltip with the exact amounts of one month", () => {
  it("nothing is shown until a month is pointed at", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" />);
    expect(tooltip(el)).toBeNull();
  });

  it("hovering a month shows its plan and actual in full đồng; leaving hides it", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" />);
    const hit = el.querySelector('[data-chart-hit="5"]')!;
    hover(hit);
    const tip = tooltip(el)!;
    expect(tip).not.toBeNull();
    expect(tip.textContent).toContain("Tháng 5");
    expect(tip.textContent).toContain("Kế hoạch luỹ kế: 50.000.000 đ");
    expect(tip.textContent).toContain("Thực hiện luỹ kế: 10.000.000 đ");
    // The month being read is marked on the chart too.
    expect(el.querySelectorAll("[data-active-month]")).toHaveLength(1);

    unhover(hit);
    expect(tooltip(el)).toBeNull();
  });

  it("a month not yet begun says so — never `0 đ` for the actual", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" />);
    hover(el.querySelector('[data-chart-hit="6"]')!);
    expect(tooltip(el)!.textContent).toContain("Thực hiện luỹ kế: Chưa đến tháng này");
  });

  it("keyboard: focus shows the first month, arrows / Home / End move, Escape and blur hide", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" />);
    const target = focusTarget(el);
    expect(target.tabIndex).toBe(0);

    act(() => target.focus());
    expect(tooltip(el)!.textContent).toContain("Tháng 4");
    expect(tooltip(el)!.textContent).toContain("Kế hoạch luỹ kế: 25.000.000 đ");

    press(target, "ArrowRight");
    expect(tooltip(el)!.textContent).toContain("Tháng 5");
    press(target, "End");
    expect(tooltip(el)!.textContent).toContain("Tháng 6");
    press(target, "ArrowRight"); // stays on the last month, never wraps to an empty reading
    expect(tooltip(el)!.textContent).toContain("Tháng 6");
    press(target, "Home");
    expect(tooltip(el)!.textContent).toContain("Tháng 4");
    press(target, "ArrowLeft");
    expect(tooltip(el)!.textContent).toContain("Tháng 4");

    press(target, "Escape");
    expect(tooltip(el)).toBeNull();
    press(target, "ArrowRight");
    expect(tooltip(el)!.textContent).toContain("Tháng 5");

    act(() => target.blur());
    expect(tooltip(el)).toBeNull();
  });

  it("the focus target describes itself, and the hidden table for screen readers stays", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="Luỹ kế của dự án" emptyText="trống" />);
    expect(focusTarget(el).getAttribute("aria-label")).toContain("Luỹ kế của dự án");
    expect(el.querySelector("table caption")?.textContent).toBe("Luỹ kế của dự án");
    expect(el.querySelectorAll("table tbody tr")).toHaveLength(3);
  });
});

describe("G4b — `Hoàn thành dự kiến` reference line", () => {
  it("drawn at the month the server names, labelled as the prototype", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" expectedEndMonth={6} />);
    const line = el.querySelector<SVGLineElement>("line[data-expected-end]")!;
    expect(line).not.toBeNull();
    expect(line.getAttribute("stroke-dasharray")).toBeTruthy();
    // Same x as the month-6 axis label: the line stands on that month, not beside it.
    const t6 = [...el.querySelectorAll("svg text")].find((t) => t.textContent === "T6")!;
    expect(line.getAttribute("x1")).toBe(t6.getAttribute("x"));
    expect(line.getAttribute("x2")).toBe(t6.getAttribute("x"));
    expect(el.querySelector("svg [data-expected-end-label]")?.textContent).toBe("Hoàn thành dự kiến");
  });

  it("no month from the server → no line (never a guessed one)", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" />);
    expect(el.querySelector("line[data-expected-end]")).toBeNull();
    expect(el.textContent).not.toContain("Hoàn thành dự kiến");
  });

  it("a month outside the drawn points → no line rather than one at the frame's edge", () => {
    const el = mount(<CumulativeChart points={POINTS} caption="c" emptyText="trống" expectedEndMonth={9} />);
    expect(el.querySelector("line[data-expected-end]")).toBeNull();
  });
});

function curve(over: Partial<finance_projectCurveOut> = {}): finance_projectCurveOut {
  return { project_id: "DA/1", year: 2026, points: POINTS, expected_end_month: 6, disbursed_after_year: 0, ...over };
}

describe("project `Biểu đồ` tab — the line replaces the sentence that stood in for it", () => {
  it("with `expected_end_month`: the line is drawn; the words stay for screen readers only", () => {
    const el = mount(<ProjectCurveView curve={curve()} />);
    expect(el.querySelector("line[data-expected-end]")).not.toBeNull();
    const words = el.querySelector<HTMLElement>("[data-expected-end-note]")!;
    expect(words.textContent).toContain("ở T6");
    expect(words.className).toContain("an-thi-giac");
  });

  it("without it: no line and no sentence about a month nobody named", () => {
    for (const c of [curve({ expected_end_month: null }), curve({ expected_end_month: undefined })]) {
      const el = mount(<ProjectCurveView curve={c} />);
      expect(el.querySelector("line[data-expected-end]")).toBeNull();
      expect(el.querySelector("[data-expected-end-note]")).toBeNull();
      act(() => root?.unmount());
      host?.remove();
      root = null;
    }
  });
});
