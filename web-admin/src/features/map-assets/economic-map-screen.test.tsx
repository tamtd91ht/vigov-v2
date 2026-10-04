// @vitest-environment jsdom
//
// The whole `/ban-do` screen, mounted, against a fake server and a FAKE MapLibre (no WebGL in jsdom).
// What is held here is what a working screen never shows breaking: no map before / without a frame,
// the frame passed IN THE CONSTRUCTOR, a toggle that filters without a request, a filter that does
// request, no camera move outside the frame, and the denied cases of every permission-gated control.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)

import { EconomicMapScreen } from "./economic-map-screen";
import {
  DELETE_REASON_REQUIRED,
  FRAME_ASK_ADMIN,
  FRAME_NOT_SET,
  NO_BASEMAP,
  NO_GROUPS,
  OUTSIDE_FRAME,
  SEED_DEFAULTS_BUTTON,
} from "./labels";
import { LAYER_LABELS, LAYER_POINTS, SOURCE_ID } from "./map-logic";

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
const FRAME_SET = { configured: true, center_lat: 15.7, center_lng: 108.35, radius_km: 20, bounds: BOUNDS };
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

type Setup = { permissions: string[]; frame?: unknown; frameStatus?: number; types?: unknown[] };

let calls: { path: string; method: string; body: unknown }[] = [];

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function serve({ permissions, frame = FRAME_SET, frameStatus = 200, types = TYPES }: Setup) {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      calls.push({ path, method, body: init?.body === undefined ? undefined : JSON.parse(String(init.body)) });
      if (path === "/api/v1/sessions/current") return json(200, { permissions, must_change_password: false });
      if (path === "/api/v1/map-asset-types") return json(200, { items: types });
      if (path === "/api/v1/map-frame")
        return frameStatus === 200 ? json(200, frame) : json(frameStatus, { code: "x", message: "Máy chủ lỗi." });
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
      <PhienProvider>
        <EconomicMapScreen styleUrl={styleUrl} />
      </PhienProvider>,
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

  it("one clustered GeoJSON source, points as [lng, lat], and the five layers", async () => {
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
    expect(button("Xoá")).toBeUndefined();
    expect(button("Bỏ xác minh")).toBeUndefined();
  });

  it("with asset.update: Sửa, Bỏ xác minh, Xoá — and Xoá requires a reason", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    const map = await load();
    await act(async () => map.fire(`click:${LAYER_POINTS}`, { features: [{ properties: { id: "a1" } }] }));
    await settle();
    expect(button("Sửa")).toBeDefined();
    expect(button("Bỏ xác minh")).toBeDefined();
    await act(async () => button("Xoá")!.click());
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
  });
});
