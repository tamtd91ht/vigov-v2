// @vitest-environment jsdom
//
// The whole `/ban-do` screen, mounted, against a fake server and a FAKE MapLibre (no WebGL in jsdom).
// What is held here is what a working screen never shows breaking: no map before / without a frame,
// the frame passed IN THE CONSTRUCTOR, a toggle that filters without a request, a filter that does
// request, no camera move outside the frame, and the denied cases of every permission-gated control.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa"; // vi-name-ok: existing export (rule 12 inv 3)
import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)

import { EconomicMapScreen } from "./economic-map-screen";
import { RADIUS_ERROR } from "./frame-form";
import {
  CENTRE_LEGEND,
  CENTRE_LEGEND_DEFAULT,
  CLEAR_FILTERS,
  DELETE_BUTTON,
  DELETE_REASON_REQUIRED,
  FRAME_ASK_ADMIN,
  FRAME_CHANGE_BUTTON,
  FRAME_NOT_SET,
  FRAME_RADIUS_UNUSUAL,
  FRAME_RESET_BUTTON,
  FRAME_RESET_DONE,
  FRAME_RESET_DONE_NO_DEFAULT,
  FRAME_SAVED,
  FRAME_SOURCE_COMMUNE,
  FRAME_SOURCE_DEFAULT,
  MAP_COLLAPSE,
  MAP_EXPAND,
  MAP_FRAME_NOTICE_ACK,
  MAP_FRAME_NOTICE_ITEMS,
  MAP_FRAME_NOTICE_TITLE,
  MAP_FRAME_NOTICE_VERSION,
  NOTICE_CONFIRM_BUTTON,
  NOTICE_RELOADED,
  NO_BASEMAP,
  NO_GROUPS,
  OUTSIDE_FRAME,
  SEED_DEFAULTS_BUTTON,
  noticeOutdated,
} from "./labels";
import { CENTRE_SOURCE_ID, LAYER_CENTRE, LAYER_CENTRE_LABEL, LAYER_LABELS, LAYER_POINTS, SOURCE_ID } from "./map-logic";

/* ---- fake MapLibre ------------------------------------------------------------------------------ */

const ml = vi.hoisted(() => {
  type Handler = (e?: unknown) => void;
  class FakeSource {
    data: unknown;
    constructor(public spec: Record<string, unknown>) {
      this.data = spec.data;
    }
    setData(d: unknown) {
      this.data = d;
    }
    async getClusterExpansionZoom() {
      return 15;
    }
  }
  class FakeMap {
    static instances: FakeMap[] = [];
    handlers = new Map<string, Handler[]>();
    sources = new Map<string, FakeSource>();
    layers: Record<string, unknown>[] = [];
    filterCalls: [string, unknown][] = [];
    layout: [string, string, unknown][] = [];
    flights: unknown[] = [];
    controls: unknown[] = [];
    removed = false;
    minZoom: number | null = null;
    touchZoomRotate = { disableRotation() {} };
    constructor(public opts: Record<string, unknown>) {
      FakeMap.instances.push(this);
    }
    addControl(c: unknown) {
      this.controls.push(c);
      return this;
    }
    on(event: string, a: string | Handler, b?: Handler) {
      const key = typeof a === "string" ? `${event}:${a}` : event;
      const fn = typeof a === "string" ? b! : a;
      this.handlers.set(key, [...(this.handlers.get(key) ?? []), fn]);
      return this;
    }
    fire(key: string, e?: unknown) {
      for (const h of this.handlers.get(key) ?? []) h(e);
    }
    getZoom() {
      return 12;
    }
    setMinZoom(z: number) {
      this.minZoom = z;
    }
    resizes = 0;
    resize() {
      this.resizes += 1;
    }
    getStyle() {
      return {
        layers: [
          { id: "label_city", type: "symbol", layout: { "text-field": ["coalesce", ["get", "name_en"], ["get", "name"]] } },
          { id: "road_shield", type: "symbol", layout: { "text-field": "{ref}" } },
          { id: "water", type: "fill" },
        ],
      };
    }
    setLayoutProperty(id: string, p: string, v: unknown) {
      this.layout.push([id, p, v]);
    }
    addSource(id: string, spec: Record<string, unknown>) {
      this.sources.set(id, new FakeSource(spec));
    }
    getSource(id: string) {
      return this.sources.get(id);
    }
    addLayer(l: Record<string, unknown>) {
      this.layers.push(l);
    }
    setFilter(id: string, f: unknown) {
      this.filterCalls.push([id, f]);
    }
    flyTo(o: unknown) {
      this.flights.push(o);
    }
    easeTo(o: unknown) {
      this.flights.push(o);
    }
    getCanvas() {
      return { style: {} as Record<string, string> };
    }
    remove() {
      this.removed = true;
    }
  }
  class FakeControl {
    constructor(public opts?: Record<string, unknown>) {}
  }
  class FakeMarker {
    el = { parentElement: null as unknown };
    constructor(public opts?: unknown) {}
    setLngLat() {
      return this;
    }
    addTo() {
      this.el.parentElement = {};
      return this;
    }
    getElement() {
      return this.el;
    }
    getLngLat() {
      return { lng: 0, lat: 0 };
    }
    on() {
      return this;
    }
    remove() {}
  }
  return { FakeMap, FakeControl, FakeMarker };
});

vi.mock("maplibre-gl", () => ({
  setWorkerUrl: () => {},
  Map: ml.FakeMap,
  AttributionControl: ml.FakeControl,
  NavigationControl: ml.FakeControl,
  ScaleControl: ml.FakeControl,
  Marker: ml.FakeMarker,
}));

/* ---- fake server ------------------------------------------------------------------------------- */

const BOUNDS = [108.1, 15.5, 108.6, 15.9];
const DEFAULT_BOUNDS = [108.0, 15.4, 108.5, 15.8];
/** Hint fields every map-frame reply carries (contract `comms_mapFrameOut`); merged into each fake reply. */
const HINTS = { recommended_radius_km: 10, usual_radius_km: [3, 20], max_radius_km: 50, notice_version: "2026-10-04.1" };
const DEFAULT = { center_lat: 15.6, center_lng: 108.25, radius_km: 12, bounds: DEFAULT_BOUNDS };
const FRAME_SET = {
  configured: true,
  source: "commune",
  center_lat: 15.7,
  center_lng: 108.35,
  radius_km: 20,
  bounds: BOUNDS,
  default: DEFAULT,
};
const FRAME_DEFAULT = { configured: true, source: "default", ...DEFAULT, default: DEFAULT };
const TYPES = [
  { id: "t1", code: "doanh-nghiep", label: "Doanh nghiệp", is_default: true, active: true, order: 1, source: "he-thong", tier: 1 },
  { id: "t2", code: "cho", label: "Chợ, trung tâm thương mại", is_default: true, active: true, order: 4, source: "he-thong", tier: 1 },
];
const POINTS = {
  type: "FeatureCollection",
  features: [
    { type: "Feature", id: "a1", geometry: { type: "Point", coordinates: [108.3, 15.7] }, properties: { id: "a1", asset_type_code: "doanh-nghiep", name: "Công ty A", status: "dang-hoat-dong", verified: true } },
    { type: "Feature", id: "a2", geometry: { type: "Point", coordinates: [108.4, 15.8] }, properties: { id: "a2", asset_type_code: "cho", name: "Chợ B", status: "dang-hoat-dong", verified: false } },
  ],
};
const DETAIL_MASKED = {
  id: "a1",
  asset_type_code: "doanh-nghiep",
  name: "Công ty A",
  lat: 15.7,
  lng: 108.3,
  representative: "N*** V*** H***",
  phone: "090****000",
  tax_code: "01******00",
  status: "dang-hoat-dong",
  verified: true,
  custom_values: {},
  masked: true,
  created_at: "2026-10-04T00:00:00Z",
  updated_at: "2026-10-04T00:00:00Z",
};
const FAR_AWAY = { ...DETAIL_MASKED, id: "far", name: "Ngoài khung", lat: 16.5, lng: 112.3 };

type Reply = { readonly status: number; readonly body: unknown };

type Setup = {
  permissions: string[];
  frame?: Record<string, unknown>;
  frameStatus?: number;
  types?: unknown[];
  /** GET map-frame replies after the first, in order (a reload after a stale notice). */
  laterFrames?: Record<string, unknown>[];
  /** Replies of PUT map-frame, in order; the last repeats. Default: 200 with the saved values. */
  puts?: Reply[];
  /** Reply of POST map-frame/reset. */
  reset?: Reply;
};

let calls: { path: string; method: string; body: unknown }[] = [];

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function serve({ permissions, frame = FRAME_SET, frameStatus = 200, types = TYPES, laterFrames = [], puts = [], reset }: Setup) {
  calls = [];
  let frameGets = 0;
  let putCount = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      calls.push({ path, method, body: init?.body === undefined ? undefined : JSON.parse(String(init.body)) });
      if (path === "/api/v1/sessions/current") return json(200, { permissions, must_change_password: false });
      if (path === "/api/v1/map-asset-types") return json(200, { items: types });
      if (path === "/api/v1/map-frame" && method === "PUT") {
        const r = puts[Math.min(putCount++, puts.length - 1)];
        if (r !== undefined) return json(r.status, r.status === 200 ? { ...HINTS, ...(r.body as object) } : r.body);
        const b = JSON.parse(String(init!.body)) as Record<string, number>;
        return json(200, { ...HINTS, ...FRAME_SET, center_lat: b.center_lat, center_lng: b.center_lng, radius_km: b.radius_km });
      }
      if (path === "/api/v1/map-frame/reset" && method === "POST") {
        const r = reset ?? { status: 200, body: FRAME_DEFAULT };
        return json(r.status, r.status === 200 ? { ...HINTS, ...(r.body as object) } : r.body);
      }
      if (path === "/api/v1/map-frame") {
        const f = frameGets === 0 ? frame : (laterFrames[frameGets - 1] ?? laterFrames.at(-1) ?? frame);
        frameGets += 1;
        return frameStatus === 200 ? json(200, { ...HINTS, ...f }) : json(frameStatus, { code: "x", message: "Máy chủ lỗi." });
      }
      if (path === "/api/v1/map-asset-summary")
        return json(200, { total: 2, verified: 1, verified_ratio: 0.5, by_type: [{ asset_type_code: "cho", count: 1, verified: 0 }] });
      if (path === "/api/v1/residential-units") return json(200, { items: [] });
      if (path.startsWith("/api/v1/map-asset-points")) return json(200, POINTS);
      if (path.startsWith("/api/v1/map-field-schemas")) return json(200, { items: [] });
      if (path === "/api/v1/map-assets/a1" && method === "GET") return json(200, DETAIL_MASKED);
      if (path === "/api/v1/map-assets/far" && method === "GET") return json(200, FAR_AWAY);
      if (path.startsWith("/api/v1/map-assets?q=C"))
        return json(200, { items: [{ id: "a1", asset_type_code: "doanh-nghiep", name: "Công ty A", status: "dang-hoat-dong", verified: true }], next_cursor: "", has_more: false });
      if (path.startsWith("/api/v1/map-assets?"))
        return json(200, { items: [{ id: "far", asset_type_code: "cho", name: "Ngoài khung", status: "dang-hoat-dong", verified: false }], next_cursor: "", has_more: false });
      return json(404, { code: "not_found", message: "Không có." });
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  ml.FakeMap.instances = [];
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

async function settle(): Promise<void> {
  for (let i = 0; i < 8; i += 1) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function mount(setup: Setup, styleUrl: string | null = "https://tiles.openfreemap.org/styles/liberty") {
  serve(setup);
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <CauHinhXaProvider giaTri={{ displayName: "Xã Thăng Bình", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" }}>
        <PhienProvider>
          <EconomicMapScreen styleUrl={styleUrl} />
        </PhienProvider>
      </CauHinhXaProvider>,
    ),
  );
  await settle();
}

function theMap() {
  expect(ml.FakeMap.instances).toHaveLength(1);
  return ml.FakeMap.instances[0]!;
}

async function load() {
  const map = theMap();
  await act(async () => map.fire("load"));
  await settle();
  return map;
}

function text(): string {
  return host!.textContent ?? "";
}

function button(label: string): HTMLButtonElement | undefined {
  return Array.from(host!.querySelectorAll("button")).find((b) => b.textContent === label || b.getAttribute("aria-label") === label);
}

/* ---- the frame decides whether a map exists ------------------------------------------------------ */

describe("no frame → NO map (ADR 0072 H3)", () => {
  it("configured:false, asset.read only: no map, the sentence, and NO form (denied case)", async () => {
    await mount({ permissions: ["asset.read"], frame: { configured: false } });
    expect(ml.FakeMap.instances).toHaveLength(0);
    expect(text()).toContain(FRAME_NOT_SET);
    expect(text()).toContain(FRAME_ASK_ADMIN);
    expect(host!.querySelector("#frame-center-lat")).toBeNull();
  });

  it("configured:false with admin.lookup: no map, and the frame form", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"], frame: { configured: false } });
    expect(ml.FakeMap.instances).toHaveLength(0);
    expect(host!.querySelector("#frame-center-lat")).not.toBeNull();
    expect(host!.querySelector("#frame-radius")).not.toBeNull();
    // Points exist → the "mean of existing points" helper is offered.
    expect(button("Lấy tâm từ các đối tượng đã có")).toBeDefined();
  });

  it("map-frame answers 500: no map at all", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"], frameStatus: 500 });
    expect(ml.FakeMap.instances).toHaveLength(0);
    expect(text()).toContain("Máy chủ lỗi.");
  });

  it("no basemap configured: no map, the sentence; the register still loads", async () => {
    await mount({ permissions: ["asset.read"] }, null);
    expect(ml.FakeMap.instances).toHaveLength(0);
    expect(text()).toContain(NO_BASEMAP);
    await act(async () => button("Sổ địa điểm")!.click());
    await settle();
    expect(calls.some((c) => c.path.startsWith("/api/v1/map-assets?"))).toBe(true);
  });
});

describe("Mở rộng bản đồ / Thu gọn", () => {
  const region = () => host!.querySelector<HTMLElement>("#economic-map-region")!;

  it("grows the map region (map + detail panel) into a page overlay, resizes the map, and Thu gọn brings it back", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    expect(region().hasAttribute("data-map-expanded")).toBe(false);
    const expand = button(MAP_EXPAND)!;
    expect(expand.getAttribute("aria-pressed")).toBe("false");
    expect(expand.getAttribute("aria-controls")).toBe("economic-map-region");

    await act(async () => expand.click());
    await act(async () => new Promise<void>((r) => requestAnimationFrame(() => r())));
    expect(region().hasAttribute("data-map-expanded")).toBe(true);
    expect(region().className).toContain("fixed");
    expect(document.body.style.overflow).toBe("hidden");
    expect(map.resizes).toBeGreaterThan(0);
    // Still the SAME map: expanding never rebuilds it, so the frame (maxBounds) is untouched.
    expect(ml.FakeMap.instances).toHaveLength(1);

    const collapse = button(MAP_COLLAPSE)!;
    expect(collapse.getAttribute("aria-pressed")).toBe("true");
    await act(async () => collapse.click());
    expect(region().hasAttribute("data-map-expanded")).toBe(false);
    expect(document.body.style.overflow).toBe("");
  });

  it("Esc collapses the overlay", async () => {
    await mount({ permissions: ["asset.read"] });
    await load();
    await act(async () => button(MAP_EXPAND)!.click());
    expect(region().hasAttribute("data-map-expanded")).toBe(true);
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(region().hasAttribute("data-map-expanded")).toBe(false);
  });

  it("no map on screen (no frame) → no expand button", async () => {
    await mount({ permissions: ["asset.read"], frame: { configured: false } });
    expect(button(MAP_EXPAND)).toBeUndefined();
  });
});

describe("the saved commune centre is drawn as a landmark", () => {
  it("one point at the frame's centre [lng, lat], labelled with the commune name, beneath every asset layer", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    const src = map.getSource(CENTRE_SOURCE_ID) as unknown as { data: { features: { geometry: { coordinates: number[] }; properties: { label: string } }[] } };
    expect(src.data.features).toHaveLength(1);
    const [centre] = src.data.features;
    expect(centre?.geometry.coordinates).toEqual([FRAME_SET.center_lng, FRAME_SET.center_lat]);
    expect(centre?.properties.label).toBe("Xã Thăng Bình");
    const ids = map.layers.map((l) => l.id);
    expect(ids).toContain(LAYER_CENTRE);
    expect(ids).toContain(LAYER_CENTRE_LABEL);
    // Beneath the assets: it never covers an asset nor takes its clicks.
    expect(ids.indexOf(LAYER_CENTRE)).toBeLessThan(ids.indexOf(LAYER_POINTS));
    expect(ids.indexOf(LAYER_CENTRE_LABEL)).toBeLessThan(ids.indexOf(LAYER_POINTS));
  });
});

describe("the map — created once, bounded in the constructor, removed on unmount", () => {
  it("maxBounds AND bounds = the API's bounds, in the constructor; no world copies; one map; remove() on unmount", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = theMap();
    expect(map.opts.maxBounds).toEqual(BOUNDS);
    expect(map.opts.bounds).toEqual(BOUNDS);
    expect(map.opts.renderWorldCopies).toBe(false);
    expect(map.opts.style).toBe("https://tiles.openfreemap.org/styles/liberty");
    await load();
    expect(ml.FakeMap.instances).toHaveLength(1); // re-renders after load did not build a second map
    expect(map.minZoom).toBe(12);
    act(() => root!.unmount());
    root = null;
    expect(map.removed).toBe(true);
  });

  it("one clustered GeoJSON source, points as [lng, lat], the centre landmark first, then the five asset layers", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    const src = map.getSource(SOURCE_ID)!;
    expect(src.spec).toMatchObject({ type: "geojson", cluster: true, clusterMaxZoom: 14, clusterRadius: 50 });
    const data = src.data as { features: { geometry: { coordinates: number[] } }[] };
    expect(data.features.map((f) => f.geometry.coordinates)).toEqual([
      [108.3, 15.7],
      [108.4, 15.8],
    ]);
    expect(map.layers.map((l) => l.id)).toEqual([
      "commune-centre-halo",
      "commune-centre-point",
      "commune-centre-label",
      "economic-clusters",
      "economic-cluster-count",
      "economic-points",
      "economic-selected-point",
      "economic-labels",
    ]);
  });

  it("basemap NAME labels rewritten to name:vi then name; shields left alone", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    expect(map.layout).toEqual([["label_city", "text-field", ["coalesce", ["get", "name:vi"], ["get", "name"]]]]);
  });

  it("attribution control carries the OpenFreeMap attribution", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = theMap();
    const attribution = map.controls.find((c) => (c as { opts?: Record<string, unknown> }).opts?.customAttribution !== undefined) as {
      opts: { customAttribution: string };
    };
    expect(attribution.opts.customAttribution).toContain("OpenFreeMap");
    expect(attribution.opts.customAttribution).toContain("OpenStreetMap");
  });
});

describe("toggles filter, server filters refetch", () => {
  it("a layer toggle sets the layer filter WITHOUT a request", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    const before = calls.length;
    map.filterCalls = [];
    await act(async () => host!.querySelector<HTMLButtonElement>('[data-layer-toggle="cho"]')!.click());
    await settle();
    expect(calls.length).toBe(before);
    const last = map.filterCalls.filter(([id]) => id === LAYER_POINTS).at(-1)![1];
    expect(JSON.stringify(last)).toContain('["literal",["doanh-nghiep"]]');
    expect(map.filterCalls.some(([id]) => id === LAYER_LABELS)).toBe(true);
    const data = map.getSource(SOURCE_ID)!.data as { features: unknown[] };
    expect(data.features).toHaveLength(1); // clusters recount only what is shown
  });

  it("'chỉ lớp này' shows that group alone, WITHOUT a request; 'Hiện hết' brings every group back", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    const before = calls.length;
    await act(async () => host!.querySelector<HTMLButtonElement>('[data-layer-only="cho"]')!.click());
    await settle();
    expect(calls.length).toBe(before);
    const last = map.filterCalls.filter(([id]) => id === LAYER_POINTS).at(-1)![1];
    expect(JSON.stringify(last)).toContain('["literal",["cho"]]');
    expect(host!.querySelector('[data-layer-toggle="doanh-nghiep"]')!.getAttribute("aria-pressed")).toBe("false");
    expect(button("Ẩn hết")).toBeUndefined();
    await act(async () => button("Hiện hết")!.click());
    expect(host!.querySelector('[data-layer-toggle="doanh-nghiep"]')!.getAttribute("aria-pressed")).toBe("true");
    expect(button("Ẩn hết")).toBeDefined();
  });

  it("'Xoá lọc' appears only while a filter is set, and clears it (a refetch without the filter)", async () => {
    await mount({ permissions: ["asset.read"] });
    await load();
    expect(button(CLEAR_FILTERS)).toBeUndefined();
    const sel = host!.querySelector<HTMLSelectElement>("#map-status")!;
    await act(async () => {
      sel.value = "tam-ngung";
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await settle();
    calls = [];
    await act(async () => button(CLEAR_FILTERS)!.click());
    await settle();
    expect(host!.querySelector<HTMLSelectElement>("#map-status")!.value).toBe("");
    expect(calls.some((c) => c.path === "/api/v1/map-asset-points")).toBe(true);
    expect(button(CLEAR_FILTERS)).toBeUndefined();
  });

  it("a status filter REFETCHES the points with the filter", async () => {
    await mount({ permissions: ["asset.read"] });
    await load();
    const sel = host!.querySelector<HTMLSelectElement>("#map-status")!;
    await act(async () => {
      sel.value = "tam-ngung";
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await settle();
    expect(calls.some((c) => c.path === "/api/v1/map-asset-points?status=tam-ngung")).toBe(true);
  });
});

describe("detail panel", () => {
  it("clicking a point loads GET map-assets/{id}; masked fields shown exactly as returned", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    await act(async () => map.fire(`click:${LAYER_POINTS}`, { features: [{ properties: { id: "a1" } }] }));
    await settle();
    expect(calls.some((c) => c.path === "/api/v1/map-assets/a1" && c.method === "GET")).toBe(true);
    expect(text()).toContain("090****000");
    expect(text()).toContain("N*** V*** H***");
    // asset.read only: no write controls (denied case).
    expect(button("Sửa")).toBeUndefined();
    expect(button(DELETE_BUTTON)).toBeUndefined();
    expect(button("Bỏ xác minh")).toBeUndefined();
  });

  it("with asset.update: Sửa, Bỏ xác minh, Xoá khỏi bản đồ — and deleting requires a reason", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    const map = await load();
    await act(async () => map.fire(`click:${LAYER_POINTS}`, { features: [{ properties: { id: "a1" } }] }));
    await settle();
    expect(button("Sửa")).toBeDefined();
    expect(button("Bỏ xác minh")).toBeDefined();
    await act(async () => button(DELETE_BUTTON)!.click());
    await settle();
    await act(async () => button("Xác nhận xoá")!.form!.requestSubmit());
    await settle();
    expect(text()).toContain(DELETE_REASON_REQUIRED);
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  });
});

async function searchFor(q: string): Promise<void> {
  const input = host!.querySelector<HTMLInputElement>("#map-search")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, q);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => {
    vi.advanceTimersByTime(400);
  });
  await settle();
}

describe("search never moves the camera outside the frame (security review #2)", () => {
  it("choosing a result INSIDE the frame: flyTo [lng, lat], selected, detail open", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    await searchFor("Công");
    expect(calls.some((c) => c.path.startsWith("/api/v1/map-assets?q=C"))).toBe(true);
    const result = Array.from(host!.querySelectorAll("button")).find((b) => b.textContent?.startsWith("Công ty A"));
    await act(async () => result!.click());
    await settle();
    expect(map.flights).toHaveLength(1);
    expect(map.flights[0]).toMatchObject({ center: [108.3, 15.7] });
    expect(text()).toContain("090****000");
  });

  it("choosing a result outside the frame: the sentence, and NO flyTo", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    await searchFor("Ngoài");
    const result = Array.from(host!.querySelectorAll("button")).find((b) => b.textContent?.startsWith("Ngoài khung"));
    expect(result).toBeDefined();
    await act(async () => result!.click());
    await settle();
    expect(text()).toContain(OUTSIDE_FRAME);
    expect(map.flights).toEqual([]);
  });
});

describe("empty catalogue", () => {
  it("admin.lookup: the sentence and the seed button", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"], types: [] });
    expect(text()).toContain(NO_GROUPS);
    expect(button(SEED_DEFAULTS_BUTTON)).toBeDefined();
  });

  it("DENIED: without admin.lookup, the sentence and NO button", async () => {
    await mount({ permissions: ["asset.read", "asset.update"], types: [] });
    expect(text()).toContain(NO_GROUPS);
    expect(button(SEED_DEFAULTS_BUTTON)).toBeUndefined();
  });
});

describe("add button follows asset.update", () => {
  it("DENIED: asset.read only → no Thêm đối tượng", async () => {
    await mount({ permissions: ["asset.read"] });
    expect(button("Thêm đối tượng")).toBeUndefined();
  });

  it("asset.update → the button; the form opens", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await act(async () => button("Thêm đối tượng")!.click());
    await settle();
    expect(host!.querySelector("#asset-name")).not.toBeNull();
    expect(host!.querySelector<HTMLInputElement>("#asset-lat")!.value).toBe("15.700000");
    // The prototype's place search sits above the pin map, disabled with its "?" (no geocoder).
    expect(host!.querySelector<HTMLInputElement>("#asset-place-search")!.disabled).toBe(true);
  });

  it("header, in the prototype's order: Mẫu Excel · Nhập Excel · Thêm đối tượng · Bản đồ / Sổ địa điểm · Trình chiếu", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    const header = host!.querySelector("header")!;
    const labels = Array.from(header.querySelectorAll("button"))
      .map((b) => b.textContent ?? "")
      .filter((t) => t !== "" && t !== "?");
    expect(labels).toEqual(["Mẫu Excel", "Nhập Excel", "Thêm đối tượng", "Bản đồ", "Sổ địa điểm", "Trình chiếu"]);
  });

  it("DENIED: asset.read only → Mẫu Excel stays, Nhập Excel is not offered", async () => {
    await mount({ permissions: ["asset.read"] });
    const header = host!.querySelector("header")!;
    expect(header.querySelector(`[aria-label="${pendingMarkerLabel("Mẫu Excel")}"]`)).not.toBeNull();
    expect(header.querySelector(`[aria-label="${pendingMarkerLabel("Nhập Excel")}"]`)).toBeNull();
  });
});

describe("Sổ địa điểm", () => {
  it("a group header folds and unfolds its rows", async () => {
    await mount({ permissions: ["asset.read"] });
    await act(async () => button("Sổ địa điểm")!.click());
    await settle();
    const toggle = host!.querySelector<HTMLButtonElement>('[data-group-toggle="cho"]')!;
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(text()).toContain("Ngoài khung");
    await act(async () => toggle.click());
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(text()).not.toContain("Ngoài khung");
    await act(async () => toggle.click());
    expect(text()).toContain("Ngoài khung");
  });
});

/* ---- frame change: limits, legal notice, "Về mặc định" (ADR 0072 lần 2, K2/K4/K6) ------------------ */

function confirmStep(): HTMLElement | null {
  return host!.querySelector<HTMLElement>("[data-notice-confirm]");
}

function inStep(label: string): HTMLButtonElement | undefined {
  return Array.from(confirmStep()!.querySelectorAll("button")).find((b) => b.textContent === label);
}

function ack(): HTMLInputElement {
  return confirmStep()!.querySelector<HTMLInputElement>("[data-notice-ack]")!;
}

async function type(id: string, value: string): Promise<void> {
  const input = host!.querySelector<HTMLInputElement>(`#${id}`)!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function pressSave(): Promise<void> {
  await act(async () => button("Lưu khung bản đồ")!.form!.requestSubmit());
  await settle();
}

async function tickAndConfirm(): Promise<void> {
  await act(async () => ack().click());
  await act(async () => inStep(NOTICE_CONFIRM_BUTTON)!.click());
  await settle();
}

function writes(): { path: string; method: string; body: unknown }[] {
  return calls.filter((c) => c.path.startsWith("/api/v1/map-frame") && c.method !== "GET");
}

async function openChange(setup: Partial<Setup> = {}): Promise<void> {
  await mount({ permissions: ["asset.read", "admin.lookup"], ...setup });
  await act(async () => button(FRAME_CHANGE_BUTTON)!.click());
  await settle();
}

describe("the legal notice text is ADR 0072 K6, verbatim", () => {
  it("version 2026-10-04.1; title, six numbered items and the acknowledgement appear word for word in the ADR", () => {
    expect(MAP_FRAME_NOTICE_VERSION).toBe("2026-10-04.1");
    expect(MAP_FRAME_NOTICE_ITEMS).toHaveLength(6);
    // Read the owning file itself (rule 9): a paraphrase here, or an edit there without a new version, turns this red.
    // `process.cwd()` is web-admin/ (vitest root); jsdom's own `URL` refuses a file URL here.
    const adr = readFileSync(join(process.cwd(), "..", "kb", "10-decisions", "0072-ban-do-kinh-te-so-nen-tu-host.md"), "utf8");
    const k6 = adr.slice(adr.indexOf("### K6."), adr.indexOf("### Điểm dừng — thêm ở lần 2"));
    expect(k6).toContain(`phiên bản \`${MAP_FRAME_NOTICE_VERSION}\``);
    expect(k6).toContain(`> ${MAP_FRAME_NOTICE_TITLE}\n`);
    MAP_FRAME_NOTICE_ITEMS.forEach((item, i) => expect(k6).toContain(`> ${i + 1}. ${item}\n`));
    expect(k6).toContain(`> ☐ ${MAP_FRAME_NOTICE_ACK}\n`);
  });
});

describe("frame form — source, hints, limits", () => {
  it("dialog: prefilled from the commune frame, says whose frame is in effect, shows the default and the server's hint", async () => {
    await openChange();
    expect(host!.querySelector<HTMLInputElement>("#frame-radius")!.value).toBe("20");
    expect(host!.querySelector("[data-frame-source]")!.textContent).toBe(FRAME_SOURCE_COMMUNE);
    expect(host!.querySelector("[data-frame-default]")!.textContent).toContain("bán kính 12 km");
    expect(host!.querySelector("#frame-radius-hint")!.textContent).toContain("Đề xuất: 10 km (thường 3–20 km).");
    expect(host!.querySelector("[data-radius-warning]")).toBeNull();
  });

  it("no frame at all: the inline form is prefilled with the recommended radius", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"], frame: { configured: false } });
    expect(host!.querySelector<HTMLInputElement>("#frame-radius")!.value).toBe("10");
    expect(host!.querySelector("[data-frame-source]")).toBeNull();
  });

  it("radius > 50 refused before any step; 0.1 accepted", async () => {
    await openChange();
    await type("frame-radius", "50.1");
    await pressSave();
    expect(text()).toContain(RADIUS_ERROR);
    expect(confirmStep()).toBeNull();
    await type("frame-radius", "0.1");
    await pressSave();
    expect(confirmStep()).not.toBeNull();
    await tickAndConfirm();
    expect(writes()).toHaveLength(1);
    expect((writes()[0]!.body as { radius_km: number }).radius_km).toBe(0.1);
  });

  it("outside the usual 3–20 km: the warning shows, and the save still goes through", async () => {
    await openChange();
    await type("frame-radius", "45");
    expect(host!.querySelector("[data-radius-warning]")!.textContent).toContain(FRAME_RADIUS_UNUSUAL);
    await pressSave();
    await tickAndConfirm();
    expect(writes()).toEqual([
      { path: "/api/v1/map-frame", method: "PUT", body: { center_lat: 15.7, center_lng: 108.35, radius_km: 45, notice_version: "2026-10-04.1" } },
    ]);
    expect(text()).toContain(FRAME_SAVED);
  });
});

describe("frame change goes through the legal notice (K4)", () => {
  it("Lưu opens the notice; confirm disabled until ticked; the PUT carries notice_version", async () => {
    await openChange();
    await pressSave();
    const step = confirmStep()!;
    expect(step.textContent).toContain(MAP_FRAME_NOTICE_TITLE);
    expect(step.querySelectorAll("[data-notice-items] li")).toHaveLength(6);
    expect(step.textContent).toContain(MAP_FRAME_NOTICE_ACK);
    expect(document.activeElement).toBe(ack());
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(true);
    await act(async () => inStep(NOTICE_CONFIRM_BUTTON)!.click());
    expect(writes()).toEqual([]);
    await act(async () => ack().click());
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(false);
    await act(async () => inStep(NOTICE_CONFIRM_BUTTON)!.click());
    await settle();
    expect(writes()).toHaveLength(1);
    expect((writes()[0]!.body as { notice_version: string }).notice_version).toBe("2026-10-04.1");
    expect(confirmStep()).toBeNull();
    expect(text()).toContain(FRAME_SAVED);
  });

  it("Huỷ on the notice sends nothing and returns to the form", async () => {
    await openChange();
    await pressSave();
    await act(async () => ack().click());
    await act(async () => inStep("Huỷ")!.click());
    await settle();
    expect(confirmStep()).toBeNull();
    expect(writes()).toEqual([]);
    expect(host!.querySelector("#frame-radius")).not.toBeNull();
  });

  it("422 notice_not_acknowledged: reloads the frame and shows the notice again, unticked", async () => {
    await openChange({ puts: [{ status: 422, body: { code: "notice_not_acknowledged", message: "Văn bản lưu ý đã đổi." } }] });
    const getsBefore = calls.filter((c) => c.path === "/api/v1/map-frame" && c.method === "GET").length;
    await pressSave();
    await tickAndConfirm();
    expect(calls.filter((c) => c.path === "/api/v1/map-frame" && c.method === "GET").length).toBe(getsBefore + 1);
    expect(confirmStep()).not.toBeNull();
    expect(confirmStep()!.textContent).toContain(NOTICE_RELOADED);
    expect(ack().checked).toBe(false);
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(true);
  });

  it("422 and the reload brings a NEWER version than this page carries: refused here until the page is reloaded", async () => {
    await openChange({
      puts: [{ status: 422, body: { code: "notice_not_acknowledged", message: "Văn bản lưu ý đã đổi." } }],
      laterFrames: [{ ...FRAME_SET, notice_version: "2026-11-01.1" }],
    });
    await pressSave();
    await tickAndConfirm();
    expect(confirmStep()!.textContent).toContain(noticeOutdated("2026-11-01.1"));
    expect(ack().disabled).toBe(true);
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(true);
    expect(writes()).toHaveLength(1);
  });

  it("server already on another version: the officer is never asked to tick a text they are not shown", async () => {
    await openChange({ frame: { ...FRAME_SET, notice_version: "2026-11-01.1" } });
    await pressSave();
    expect(confirmStep()!.textContent).toContain(noticeOutdated("2026-11-01.1"));
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(true);
    expect(writes()).toEqual([]);
  });
});

describe("Về mặc định (K4)", () => {
  it("through the notice → POST reset with notice_version → the default frame applies", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"] });
    await load();
    expect(host!.querySelector("[data-centre-legend]")!.textContent).toBe(CENTRE_LEGEND);
    await act(async () => button(FRAME_RESET_BUTTON)!.click());
    await settle();
    expect(host!.querySelector("[data-frame-reset-lead]")!.textContent).toContain("bán kính 12 km");
    expect(inStep(NOTICE_CONFIRM_BUTTON)!.disabled).toBe(true);
    await tickAndConfirm();
    expect(writes()).toEqual([{ path: "/api/v1/map-frame/reset", method: "POST", body: { notice_version: "2026-10-04.1" } }]);
    expect(text()).toContain(FRAME_RESET_DONE);
    expect(button(FRAME_RESET_BUTTON)).toBeUndefined();
    expect(host!.querySelector("[data-centre-legend]")!.textContent).toBe(CENTRE_LEGEND_DEFAULT);
    // A new map, bounded by the DEFAULT frame.
    expect(ml.FakeMap.instances.at(-1)!.opts.maxBounds).toEqual(DEFAULT_BOUNDS);
  });

  it("no default on the platform: the reply is configured:false → no map, the no-frame state", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"], reset: { status: 200, body: { configured: false } } });
    await act(async () => button(FRAME_RESET_BUTTON)!.click());
    await settle();
    await tickAndConfirm();
    expect(text()).toContain(FRAME_RESET_DONE_NO_DEFAULT);
    expect(text()).toContain(FRAME_NOT_SET);
    expect(host!.querySelector("#frame-center-lat")).not.toBeNull();
  });

  it("Huỷ sends nothing; Esc on the notice closes IT and leaves the expanded map expanded", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"] });
    await load();
    await act(async () => button(MAP_EXPAND)!.click());
    await act(async () => button(FRAME_RESET_BUTTON)!.click());
    await settle();
    await act(async () => {
      ack().dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    });
    expect(confirmStep()).toBeNull();
    expect(host!.querySelector("#economic-map-region")!.hasAttribute("data-map-expanded")).toBe(true);
    await act(async () => button(FRAME_RESET_BUTTON)!.click());
    await settle();
    await act(async () => inStep("Huỷ")!.click());
    expect(confirmStep()).toBeNull();
    expect(writes()).toEqual([]);
  });

  it("default frame in effect: no Về mặc định, the form says so, the legend says (mặc định)", async () => {
    await openChange({ frame: FRAME_DEFAULT });
    expect(button(FRAME_RESET_BUTTON)).toBeUndefined();
    expect(host!.querySelector("[data-frame-source]")!.textContent).toBe(FRAME_SOURCE_DEFAULT);
    expect(host!.querySelector("[data-centre-legend]")!.textContent).toBe(CENTRE_LEGEND_DEFAULT);
  });

  it("DENIED: without admin.lookup, neither Đổi khung bản đồ nor Về mặc định", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    expect(button(FRAME_CHANGE_BUTTON)).toBeUndefined();
    expect(button(FRAME_RESET_BUTTON)).toBeUndefined();
  });
});
