// @vitest-environment jsdom
//
// jsdom for this file: the maps are BEHAVIOUR — which reads start, what a refusal draws, what MapLibre is
// handed. MapLibre and PMTiles are replaced by fakes that record what they are given.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { FRAME_NOT_SET } from "@/features/map-assets/labels";
import type { KetQua } from "@/lib/api/goi";
import { citizenReportCountsPath, citizenReportPointsPath, duongDanSoPhanAnh, type PointsResult } from "@/lib/api/phieu-phan-anh";
import type { comms_mapFrameOut } from "@/lib/api/schema.gen";
import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";

import { HEAT_MAP_EMPTY_TITLE, MINI_MAP_OUTSIDE_FRAME } from "./nhan-phieu";
import { PetitionHeatMap, PetitionMapCanvas, PetitionMiniMap, type PetitionMapCanvasProps } from "./petition-map";

/* ── fake MapLibre / PMTiles ─────────────────────────────────────────────────────────────────── */

const ml = vi.hoisted(() => {
  const calls: string[] = [];
  const maps: FakeMap[] = [];
  const markers: { opts: unknown; at: unknown }[] = [];
  const protocols: string[] = [];
  class FakeMap {
    handlers = new Map<string, () => void>();
    sources = new Map<string, unknown>();
    layers: Record<string, unknown>[] = [];
    removed = false;
    touchZoomRotate = { disableRotation() {} };
    constructor(public opts: Record<string, unknown>) {
      calls.push("new Map");
      maps.push(this);
    }
    on(event: string, handler: () => void) {
      this.handlers.set(event, handler);
    }
    addControl() {}
    setMinZoom() {}
    getZoom() {
      return 12;
    }
    addSource(id: string, spec: unknown) {
      this.sources.set(id, spec);
    }
    getSource() {
      return { setData() {} };
    }
    addLayer(l: Record<string, unknown>) {
      this.layers.push(l);
    }
    remove() {
      this.removed = true;
    }
  }
  class FakeControl {
    constructor(public opts?: unknown) {}
  }
  class FakeMarker {
    constructor(public opts?: unknown) {}
    at: unknown = null;
    setLngLat(at: unknown) {
      this.at = at;
      return this;
    }
    addTo() {
      markers.push({ opts: this.opts, at: this.at });
      return this;
    }
  }
  return { calls, maps, markers, protocols, FakeMap, FakeControl, FakeMarker };
});

vi.mock("maplibre-gl", () => ({
  setWorkerUrl: () => ml.calls.push("setWorkerUrl"),
  addProtocol: (name: string) => {
    ml.calls.push(`addProtocol:${name}`);
    ml.protocols.push(name);
  },
  Map: ml.FakeMap,
  NavigationControl: ml.FakeControl,
  Marker: ml.FakeMarker,
}));

vi.mock("pmtiles", () => ({
  Protocol: class {
    tile = () => {};
  },
}));

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
  ml.calls.length = 0;
  ml.maps.length = 0;
  ml.markers.length = 0;
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function flush(times = 8) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

const BOUNDS = [108.1, 15.5, 108.6, 15.9];
const FRAME: KetQua<comms_mapFrameOut> = {
  ok: true,
  duLieu: {
    configured: true,
    source: "commune",
    center_lat: 15.7,
    center_lng: 108.35,
    radius_km: 20,
    bounds: BOUNDS,
    default: null,
    recommended_radius_km: 10,
    usual_radius_km: [3, 20],
    max_radius_km: 50,
    notice_version: "2026-10-04.1",
  } as unknown as comms_mapFrameOut,
};
const FRAME_UNSET: KetQua<comms_mapFrameOut> = {
  ok: true,
  duLieu: { configured: false, source: "none", default: null } as unknown as comms_mapFrameOut,
};
const POINTS: PointsResult = {
  ok: true,
  data: {
    items: [
      { lat: 15.7, lng: 108.3, status: "dang-xu-ly" },
      { lat: 15.71, lng: 108.31, status: "da-tiep-nhan" },
    ],
  },
};

/** A canvas double: records what it was handed, draws nothing. */
const seen: PetitionMapCanvasProps[] = [];
function FakeCanvas(props: PetitionMapCanvasProps) {
  seen.push(props);
  return <div data-testid="fake-canvas" />;
}

describe("Bản đồ nhiệt — what is read and what is drawn", () => {
  it("basemap NOT configured: the one sentence, no map — and NO coordinate or frame is even fetched", async () => {
    const loadPoints = vi.fn(async () => POINTS);
    const loadFrame = vi.fn(async () => FRAME);
    const h = mount(
      <PetitionHeatMap filter={{}} basemapAvailable={false} loadPoints={loadPoints} loadFrame={loadFrame} Canvas={FakeCanvas} />,
    );
    await flush();
    expect(h.textContent).toContain(BASEMAP_MISSING_SENTENCE);
    expect(loadPoints).not.toHaveBeenCalled();
    expect(loadFrame).not.toHaveBeenCalled();
    expect(h.querySelector('[data-testid="fake-canvas"]')).toBeNull();
  });

  it("422 `too_many_points`: the server's sentence, NOTHING drawn, no `Tải lại` (never a partial set)", async () => {
    const cau = "Có hơn 5000 phản ánh có vị trí theo bộ lọc này. Hãy thu hẹp bộ lọc để xem bản đồ nhiệt.";
    const h = mount(
      <PetitionHeatMap
        filter={{}}
        basemapAvailable
        loadPoints={async () => ({ ok: false, message: cau, tooManyPoints: true })}
        loadFrame={async () => FRAME}
        Canvas={FakeCanvas}
      />,
    );
    await flush();
    expect(h.textContent).toContain(cau);
    expect(h.querySelector('[data-testid="fake-canvas"]')).toBeNull();
    expect(h.textContent).not.toContain("Tải lại");
  });

  it("any other refusal: the sentence and `Tải lại`, which asks again", async () => {
    const loadPoints = vi
      .fn<(f: object) => Promise<PointsResult>>()
      .mockResolvedValueOnce({ ok: false, message: "Không kết nối được máy chủ. Vui lòng thử lại.", tooManyPoints: false })
      .mockResolvedValueOnce(POINTS);
    seen.length = 0;
    const h = mount(
      <PetitionHeatMap filter={{}} basemapAvailable loadPoints={loadPoints} loadFrame={async () => FRAME} Canvas={FakeCanvas} />,
    );
    await flush();
    expect(h.querySelector('[role="alert"]')?.textContent).toContain("Không kết nối được máy chủ");
    const reload = [...h.querySelectorAll("button")].find((b) => b.textContent?.includes("Tải lại"));
    act(() => reload?.click());
    await flush();
    expect(loadPoints).toHaveBeenCalledTimes(2);
    expect(h.querySelector('[data-testid="fake-canvas"]')).not.toBeNull();
  });

  it("no located petition: the prototype's empty state", async () => {
    const h = mount(
      <PetitionHeatMap
        filter={{}}
        basemapAvailable
        loadPoints={async () => ({ ok: true, data: { items: [] } })}
        loadFrame={async () => FRAME}
        Canvas={FakeCanvas}
      />,
    );
    await flush();
    expect(h.textContent).toContain(HEAT_MAP_EMPTY_TITLE);
  });

  it("points + frame: the canvas gets the commune frame and every point; the filters are the list's", async () => {
    seen.length = 0;
    const loadPoints = vi.fn(async () => POINTS);
    const filter = { trangThai: "dang-xu-ly", tim: "ngõ 5" };
    const h = mount(
      <PetitionHeatMap filter={filter} basemapAvailable loadPoints={loadPoints} loadFrame={async () => FRAME} Canvas={FakeCanvas} />,
    );
    await flush();
    expect(loadPoints).toHaveBeenCalledWith(filter);
    const last = seen.at(-1)!;
    expect(last.frame.bounds).toEqual(BOUNDS);
    expect(last.content).toEqual({ kind: "heat", points: POINTS.ok ? POINTS.data.items : [] });
    expect(h.textContent).toContain("2 phản ánh có vị trí");
    // The keyword rides in the REQUEST only — never the address bar.
    expect(window.location.href).not.toContain("ng%C3%B5");
    expect(window.location.href).not.toContain("ngõ");
  });

  it("the commune has no frame: the economic map's sentence, no map", async () => {
    const h = mount(
      <PetitionHeatMap filter={{}} basemapAvailable loadPoints={async () => POINTS} loadFrame={async () => FRAME_UNSET} Canvas={FakeCanvas} />,
    );
    await flush();
    expect(h.textContent).toContain(FRAME_NOT_SET);
    expect(h.querySelector('[data-testid="fake-canvas"]')).toBeNull();
  });
});

describe("the points and count paths — the list's filters, the list's builder", () => {
  it("same filter parameters as the list (paging aside), keyword included in the REQUEST", () => {
    const loc = { trangThai: "dang-xu-ly", thonID: "01JTHON1", tim: "ngõ 5", chiTreHan: true as const };
    const listQuery = duongDanSoPhanAnh(loc).split("?")[1];
    expect(citizenReportPointsPath(loc)).toBe(`/api/v1/citizen-report-points?${listQuery}`);
    expect(citizenReportCountsPath(loc)).toBe(`/api/v1/citizen-report-counts?${listQuery}`);
    expect(citizenReportPointsPath({})).toBe("/api/v1/citizen-report-points");
    expect(citizenReportPointsPath({ ...loc, limit: 20, cursor: "abc" } as object)).not.toMatch(/limit|cursor/);
  });
});

describe("Vị trí mini-map", () => {
  it("basemap not configured: the sentence; no frame read", async () => {
    const loadFrame = vi.fn(async () => FRAME);
    const h = mount(<PetitionMiniMap lat={15.7} lng={108.3} basemapAvailable={false} loadFrame={loadFrame} Canvas={FakeCanvas} />);
    await flush();
    expect(h.textContent).toContain(BASEMAP_MISSING_SENTENCE);
    expect(loadFrame).not.toHaveBeenCalled();
  });

  it("a pin inside the frame: the canvas pins THAT point", async () => {
    seen.length = 0;
    mount(<PetitionMiniMap lat={15.7} lng={108.3} basemapAvailable loadFrame={async () => FRAME} Canvas={FakeCanvas} />);
    await flush();
    expect(seen.at(-1)?.content).toEqual({ kind: "pin", lat: 15.7, lng: 108.3 });
  });

  it("a point outside the commune frame: no map (it could not be shown), a sentence instead", async () => {
    const h = mount(<PetitionMiniMap lat={21.0} lng={105.8} basemapAvailable loadFrame={async () => FRAME} Canvas={FakeCanvas} />);
    await flush();
    expect(h.textContent).toContain(MINI_MAP_OUTSIDE_FRAME);
    expect(h.querySelector('[data-testid="fake-canvas"]')).toBeNull();
  });
});

describe("the real canvas — what MapLibre is handed", () => {
  const frame = { centerLat: 15.7, centerLng: 108.35, radiusKm: 20, bounds: [108.1, 15.5, 108.6, 15.9] as const, source: "commune" as const };

  it("style is built in code and SAME-ORIGIN only; pmtiles registered and worker set BEFORE the map; frame in the constructor", async () => {
    mount(<PetitionMapCanvas frame={frame} content={{ kind: "heat", points: [] }} label="x" />);
    await flush();
    expect(ml.calls.indexOf("setWorkerUrl")).toBeGreaterThan(-1);
    expect(ml.calls.indexOf("addProtocol:pmtiles")).toBeGreaterThan(-1);
    expect(ml.calls.indexOf("setWorkerUrl")).toBeLessThan(ml.calls.indexOf("new Map"));
    expect(ml.calls.indexOf("addProtocol:pmtiles")).toBeLessThan(ml.calls.indexOf("new Map"));
    const opts = ml.maps[0]!.opts;
    expect(opts.maxBounds).toEqual([108.1, 15.5, 108.6, 15.9]);
    expect(opts.bounds).toEqual([108.1, 15.5, 108.6, 15.9]);
    // Every URL in the style is this page's origin + /basemap — nothing else, OpenFreeMap least of all.
    const style = opts.style as Record<string, unknown>;
    expect(typeof style).toBe("object");
    const urls = JSON.stringify(style).match(/(?:https?|pmtiles):\/\/[^"]+/g) ?? [];
    expect(urls.length).toBeGreaterThan(0);
    const origin = window.location.origin;
    for (const u of urls) {
      expect(u.replace(/^pmtiles:\/\//, "").startsWith(`${origin}/basemap/`), u).toBe(true);
    }
    expect(JSON.stringify(style)).not.toMatch(/openfreemap/i);
  });

  it("heat mode: on load, a heat layer and a dot layer over ONE geojson source", async () => {
    mount(<PetitionMapCanvas frame={frame} content={{ kind: "heat", points: POINTS.ok ? POINTS.data.items : [] }} label="x" />);
    await flush();
    const map = ml.maps[0]!;
    act(() => map.handlers.get("load")?.());
    expect(map.layers.map((l) => l.type)).toEqual(["heatmap", "circle"]);
    const src = map.sources.get("petition-points") as { data: { features: { geometry: { coordinates: number[] }; properties: object }[] } };
    expect(src.data.features.map((f) => f.geometry.coordinates)).toEqual([
      [108.3, 15.7],
      [108.31, 15.71],
    ]);
    // Nothing but the position goes onto the map — not even the status.
    expect(src.data.features.every((f) => Object.keys(f.properties).length === 0)).toBe(true);
  });

  it("pin mode: opens ON the pin, still bounded by the frame; one MapLibre marker, no HTML of ours", async () => {
    mount(<PetitionMapCanvas frame={frame} content={{ kind: "pin", lat: 15.7, lng: 108.3 }} label="x" />);
    await flush();
    const opts = ml.maps[0]!.opts;
    expect(opts.center).toEqual([108.3, 15.7]);
    expect(opts.maxBounds).toEqual([108.1, 15.5, 108.6, 15.9]);
    expect(ml.markers).toEqual([{ opts: { color: "#E5484D" }, at: [108.3, 15.7] }]);
  });

  it("a style that never loads: the failure sentence, not a blank box", async () => {
    const h = mount(<PetitionMapCanvas frame={frame} content={{ kind: "heat", points: [] }} label="x" />);
    await flush();
    act(() => ml.maps[0]!.handlers.get("error")?.());
    expect(h.textContent).toContain("Không tải được bản đồ nền");
  });
});

describe("source boundaries of the petition maps", () => {
  // CODE only: the header comment names what is forbidden, and must be able to.
  // `process.cwd()` is web-admin under vitest; jsdom's `URL` is not a file URL `fileURLToPath` accepts.
  const src = readFileSync(join(process.cwd(), "src/features/phan-anh/petition-map.tsx"), "utf8")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");

  it("never the economic map's style source: no MAP_STYLE_URL, no mapStyleUrl, no OpenFreeMap, no NEXT_PUBLIC", () => {
    expect(src).not.toMatch(/MAP_STYLE_URL|mapStyleUrl|openfreemap|NEXT_PUBLIC/i);
    expect(src).toContain("buildBasemapStyle(window.location.origin)");
  });

  it("MapLibre only as `import type` + dynamic import; maxBounds only in the constructor; no popup, no HTML", () => {
    expect(/^import\s+(?!type\b)[^;]*from\s+"maplibre-gl"/m.test(src)).toBe(false);
    expect(src).toContain('import("maplibre-gl")');
    expect(/setMaxBounds\s*\(/.test(src)).toBe(false);
    expect(/\.setHTML\s*\(|\.innerHTML\s*=|dangerouslySetInnerHTML|Popup\(/.test(src)).toBe(false);
  });
});
