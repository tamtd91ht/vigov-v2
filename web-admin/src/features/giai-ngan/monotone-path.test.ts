import { describe, expect, it } from "vitest";

import { monotoneTangents, monotoneXPath } from "./monotone-path";

/**
 * ADR 0081 #4: both lines of the cumulative chart are monotone-X cubics, as the prototype's recharts
 * `type="monotone"` (= d3 `curveMonotoneX`). These pin the shape d3 produces and the property that
 * makes it safe for money: a segment never leaves the range of its two end values.
 */

const pts = (ys: number[]) => ys.map((y, i) => ({ x: i * 30, y }));

/** Every number of a path `d`, in order. */
function numbers(d: string): number[] {
  return (d.match(/-?\d+(\.\d+)?/g) ?? []).map(Number);
}

describe("monotoneXPath — degenerate inputs, as d3", () => {
  it("no point → empty; one → a bare move; two → a straight segment", () => {
    expect(monotoneXPath([])).toBe("");
    expect(monotoneXPath([{ x: 5, y: 7 }])).toBe("M5,7");
    expect(monotoneXPath([{ x: 0, y: 10 }, { x: 30, y: 4 }])).toBe("M0,10L30,4");
  });
});

describe("monotoneXPath — d3 curveMonotoneX reference", () => {
  it("a straight line stays straight: control points on the line, thirds of each step", () => {
    expect(monotoneXPath(pts([0, 10, 20]))).toBe("M0,0C10,3.33,20,6.67,30,10C40,13.33,50,16.67,60,20");
  });

  it("matches d3's output for an uneven rise (values computed by hand from d3's slope2/slope3)", () => {
    // x 0,1,2 · y 0,1,3: s0=1, s1=2, p=1.5 → t1=min(1,2,0.75)*2=1.5; t0=(3-1.5)/2=0.75; t2=(6-1.5)/2=2.25.
    const d = monotoneXPath([
      { x: 0, y: 0 },
      { x: 1, y: 1 },
      { x: 2, y: 3 },
    ]);
    expect(d).toBe("M0,0C0.33,0.25,0.67,0.5,1,1C1.33,1.5,1.67,2.25,2,3");
  });
});

describe("monotoneTangents — the properties that make it fit a cumulative amount", () => {
  it("a flat run (months with no payment) keeps a zero tangent: no dip, no bump", () => {
    const t = monotoneTangents(pts([0, 50, 50, 50, 80]));
    expect(t[2]).toBe(0);
    expect(t[1]).toBe(0);
    expect(t[3]).toBe(0);
  });

  it("a peak gets a zero tangent (opposite secants)", () => {
    expect(monotoneTangents(pts([0, 10, 0]))[1]).toBe(0);
  });

  it("no segment's control point leaves the range of its two end values (no overshoot)", () => {
    // A late lump after a long flat stretch — the shape that makes a natural spline dip below zero.
    const ys = [0, 0, 0, 0, 5, 5, 5, 100, 100, 110, 110, 300];
    const p = pts(ys);
    const d = monotoneXPath(p);
    const segs = d.split("C").slice(1).map(numbers);
    expect(segs).toHaveLength(ys.length - 1);
    segs.forEach((s, i) => {
      const lo = Math.min(ys[i]!, ys[i + 1]!);
      const hi = Math.max(ys[i]!, ys[i + 1]!);
      for (const cy of [s[1]!, s[3]!]) {
        expect(cy).toBeGreaterThanOrEqual(lo - 0.01);
        expect(cy).toBeLessThanOrEqual(hi + 0.01);
      }
      // Ends exactly at the next data point.
      expect(s[4]).toBe(p[i + 1]!.x);
      expect(s[5]).toBe(ys[i + 1]);
    });
  });

  it("works in screen coordinates too (y grows downward) — monotone either way", () => {
    const d = monotoneXPath(pts([200, 150, 150, 20]));
    expect(d.startsWith("M0,200C")).toBe(true);
    expect(d.endsWith(",90,20")).toBe(true);
  });
});
