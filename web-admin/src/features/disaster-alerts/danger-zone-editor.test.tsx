// @vitest-environment jsdom
//
// jsdom: these tests press the canvas and drag a vertex — the editor's whole job.

import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { comms_DangerZonePoint } from "@/lib/api/schema.gen";

import { ZONE_ADD_POINT, ZONE_CLEAR, ZONE_REMOVE_LAST } from "./alert-form";
import { DangerZoneEditor } from "./danger-zone-editor";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;
let latest: comms_DangerZonePoint[] = [];

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  latest = [];
});

function Harness({ start }: { start: comms_DangerZonePoint[] }) {
  const [zone, setZone] = useState(start);
  return (
    <DangerZoneEditor
      value={zone}
      onChange={(z) => {
        latest = z;
        setZone(z);
      }}
    />
  );
}

function mount(start: comms_DangerZonePoint[] = []): SVGSVGElement {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(<Harness start={start} />));
  const svg = host.querySelector<SVGSVGElement>('[data-testid="danger-zone-canvas"]')!;
  // A 200×200 box at (0,0): one grid unit = 2px.
  vi.spyOn(svg, "getBoundingClientRect").mockReturnValue({
    left: 0, top: 0, width: 200, height: 200, right: 200, bottom: 200, x: 0, y: 0, toJSON: () => ({}),
  } as DOMRect);
  return svg;
}

function pointer(el: Element, type: string, x: number, y: number): void {
  act(() => {
    el.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y }));
  });
}

function button(label: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === label);
  if (b === undefined) throw new Error(`no button "${label}"`);
  return b;
}

const background = () => host!.querySelector('[data-testid="danger-zone-background"]')!;
const vertices = () => host!.querySelectorAll('[data-testid="danger-zone-vertex"]');

describe("drawing the danger zone", () => {
  it("a press on the canvas adds a vertex at the grid position", () => {
    mount();
    pointer(background(), "pointerdown", 20, 40);
    pointer(background(), "pointerdown", 120, 40);
    pointer(background(), "pointerdown", 100, 180);
    expect(latest).toEqual([
      { x: 10, y: 20 },
      { x: 60, y: 20 },
      { x: 50, y: 90 },
    ]);
    expect(vertices()).toHaveLength(3);
    const polygon = host!.querySelector('[data-testid="danger-zone-polygon"]')!;
    expect(polygon.getAttribute("points")).toBe("10,20 60,20 50,90");
    // Drawn as the Mini App draws it: red, translucent, dashed.
    expect(polygon.getAttribute("fill-opacity")).toBe("0.3");
    expect(polygon.getAttribute("stroke-dasharray")).toBe("3 2");
  });

  it("dragging a vertex moves it and adds nothing", () => {
    const svg = mount([{ x: 10, y: 10 }, { x: 50, y: 10 }, { x: 30, y: 50 }]);
    const second = vertices()[1]!;
    pointer(second, "pointerdown", 100, 20);
    pointer(svg, "pointermove", 140, 60);
    pointer(svg, "pointermove", 160, 80);
    pointer(svg, "pointerup", 160, 80);
    expect(latest).toEqual([{ x: 10, y: 10 }, { x: 80, y: 40 }, { x: 30, y: 50 }]);
    // After the drag, moving the pointer changes nothing.
    pointer(svg, "pointermove", 10, 10);
    expect(latest[1]).toEqual({ x: 80, y: 40 });
  });

  it("Xoá điểm cuối removes the last vertex; Xoá vùng removes all", () => {
    mount([{ x: 10, y: 10 }, { x: 50, y: 10 }, { x: 30, y: 50 }]);
    act(() => button(ZONE_REMOVE_LAST).click());
    expect(latest).toEqual([{ x: 10, y: 10 }, { x: 50, y: 10 }]);
    act(() => button(ZONE_CLEAR).click());
    expect(latest).toEqual([]);
    expect(button(ZONE_CLEAR).disabled).toBe(true);
    expect(button(ZONE_REMOVE_LAST).disabled).toBe(true);
  });

  it("keyboard path: Thêm điểm adds a point, its X/Y boxes set it (clamped to 0..100)", () => {
    mount();
    act(() => button(ZONE_ADD_POINT).click());
    expect(latest).toEqual([{ x: 50, y: 50 }]);
    const x = host!.querySelector<HTMLInputElement>('input[aria-label="Điểm 1, X"]')!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    act(() => {
      setter.call(x, "130");
      x.dispatchEvent(new Event("input", { bubbles: true }));
    });
    expect(latest).toEqual([{ x: 100, y: 50 }]);
    const remove = host!.querySelector<HTMLButtonElement>('button[aria-label="Bỏ dòng — điểm 1"]')!;
    act(() => remove.click());
    expect(latest).toEqual([]);
  });
});
