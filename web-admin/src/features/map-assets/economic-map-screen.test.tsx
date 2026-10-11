// @vitest-environment jsdom
//
// The whole `/ban-do` screen, mounted, against a fake server and a FAKE MapLibre (no WebGL in jsdom).
// What is held here is what a working screen never shows breaking: no map before / without a frame,
// the frame passed IN THE CONSTRUCTOR, a toggle that filters without a request, a filter that does
// request, no camera move outside the frame, and the denied cases of every permission-gated control.
// The basemap is the SELF-HOSTED one (`buildBasemapStyle`, same origin, `/basemap/*` — owner 11/10/2026);
// the heat layer reads the real petition points and only with `feedback.read`.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa"; // vi-name-ok: existing export (rule 12 inv 3)
import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { HEAT_MAP_LOAD_FAILED } from "@/features/phan-anh/nhan-phieu";
import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)
import { BASEMAP_FONTS, BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";

import { DetailPanel } from "./detail-panel";
import { EconomicMapScreen } from "./economic-map-screen";
import { RADIUS_ERROR } from "./frame-form";
import {
  ADD_BUTTON,
  CENTRE_LEGEND,
  CENTRE_LEGEND_DEFAULT,
  CLEAR_FILTERS,
  DELETE_BUTTON,
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
  HEAT_NEEDS_FEEDBACK_READ,
  HEAT_NEEDS_MAP,
  MAP_COLLAPSE,
  MAP_EXPAND,
  MAP_FRAME_NOTICE_ACK,
  MAP_FRAME_NOTICE_ITEMS,
  MAP_FRAME_NOTICE_TITLE,
  MAP_FRAME_NOTICE_VERSION,
  NOTICE_CONFIRM_BUTTON,
  NOTICE_RELOADED,
  NO_GROUPS,
  OUTSIDE_FRAME,
  PHAN_CHUA_DUNG,
  PRESENTATION_BUTTON,
  PRESENTATION_TITLE,
  SEED_DEFAULTS_BUTTON,
  noticeOutdated,
  pendingPart,
} from "./labels";
import {
  CENTRE_SOURCE_ID,
  HEAT_SOURCE_ID,
  LAYER_CENTRE,
  LAYER_CENTRE_LABEL,
  LAYER_CLUSTERS,
  LAYER_HALO,
  LAYER_HEAT,
  LAYER_LABELS,
  LAYER_POINTS,
  SOURCE_ID,
} from "./map-logic";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

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
    paint: [string, string, unknown][] = [];
    setPaintProperty(id: string, p: string, v: unknown) {
      this.paint.push([id, p, v]);
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
    static instances: FakeMarker[] = [];
    el = { parentElement: null as unknown };
    constructor(public opts?: Record<string, unknown>) {
      FakeMarker.instances.push(this);
    }
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
  addProtocol: () => {},
  Map: ml.FakeMap,
  AttributionControl: ml.FakeControl,
  NavigationControl: ml.FakeControl,
  ScaleControl: ml.FakeControl,
  Marker: ml.FakeMarker,
}));

vi.mock("pmtiles", () => ({
  Protocol: class {
    tile = () => {};
  },
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
  industry_code: "10",
  established_on: "2020-03-05",
  description: "Dòng một\nDòng hai",
  custom_values: {},
  masked: true,
  created_at: "2026-10-04T00:00:00Z",
  updated_at: "2026-10-04T00:00:00Z",
};
const FAR_AWAY = { ...DETAIL_MASKED, id: "far", name: "Ngoài khung", lat: 16.5, lng: 112.3 };
const DETAIL_UNVERIFIED = {
  id: "a2",
  asset_type_code: "cho",
  name: "Chợ B",
  lat: 15.8,
  lng: 108.4,
  status: "dang-hoat-dong",
  verified: false,
  custom_values: {},
  masked: false,
  created_at: "2026-10-04T00:00:00Z",
  updated_at: "2026-10-04T00:00:00Z",
};

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
  /** Reply of GET citizen-report-points (the heat layer). Default: 200 with two points. */
  heatPoints?: Reply;
  /** Status of DELETE map-assets/{id}. Default 204. */
  deleteStatus?: number;
  /** Items of the register (GET map-assets?…, not the search). Default: one row, "Ngoài khung". */
  register?: unknown[];
  /** The register request never answers (its loading state). */
  registerHold?: boolean;
  /** Items of GET map-field-schemas. Default: none. */
  fields?: unknown[];
};

const HEAT_POINTS = { items: [{ lat: 15.7, lng: 108.3, status: "dang-xu-ly" }, { lat: 15.75, lng: 108.32, status: "da-dong" }] };

let calls: { path: string; method: string; body: unknown }[] = [];

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function serve({
  permissions,
  frame = FRAME_SET,
  frameStatus = 200,
  types = TYPES,
  laterFrames = [],
  puts = [],
  reset,
  heatPoints,
  deleteStatus = 204,
  register,
  registerHold = false,
  fields = [],
}: Setup) {
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
        return json(200, { total: 2, verified: 1, verified_ratio: 0.5, by_type: [
          { asset_type_code: "doanh-nghiep", count: 4, verified: 1 },
          { asset_type_code: "ho-kinh-doanh", count: 3, verified: 0 },
          { asset_type_code: "cho", count: 1, verified: 0 },
        ] });
      if (path === "/api/v1/residential-units") return json(200, { items: [] });
      if (path.startsWith("/api/v1/citizen-report-points")) {
        const r = heatPoints ?? { status: 200, body: HEAT_POINTS };
        return json(r.status, r.body);
      }
      if (path.startsWith("/api/v1/map-asset-points")) return json(200, POINTS);
      if (path.startsWith("/api/v1/map-field-schemas")) return json(200, { items: fields });
      if (path.startsWith("/api/v1/map-assets/") && method === "DELETE") {
        return deleteStatus === 204
          ? new Response(null, { status: 204 })
          : json(deleteStatus, { code: "locked", message: "Bản ghi đang được người khác sửa." });
      }
      if (path === "/api/v1/map-assets/a2/confirmation" && method === "POST") {
        const b = JSON.parse(String(init!.body)) as { verified: boolean };
        return json(200, { ...DETAIL_UNVERIFIED, verified: b.verified });
      }
      if (path === "/api/v1/map-assets" && method === "POST") {
        const b = JSON.parse(String(init!.body)) as Record<string, unknown>;
        return json(201, { ...DETAIL_UNVERIFIED, ...b, id: "new" });
      }
      if (path === "/api/v1/map-assets/a1" && method === "GET") return json(200, DETAIL_MASKED);
      if (path === "/api/v1/map-assets/a2" && method === "GET") return json(200, DETAIL_UNVERIFIED);
      if (path === "/api/v1/map-assets/far" && method === "GET") return json(200, FAR_AWAY);
      if (path.startsWith("/api/v1/map-assets?q=C"))
        return json(200, { items: [{ id: "a1", asset_type_code: "doanh-nghiep", name: "Công ty A", status: "dang-hoat-dong", verified: true }], next_cursor: "", has_more: false });
      if (path.startsWith("/api/v1/map-assets?")) {
        // The register asks for a page of 100; the search box asks for 10.
        const isRegister = path.includes("limit=100");
        if (isRegister && registerHold) return new Promise<Response>(() => {});
        const items =
          isRegister && register !== undefined
            ? register
            : [{ id: "far", asset_type_code: "cho", name: "Ngoài khung", status: "dang-hoat-dong", verified: false }];
        return json(200, { items, next_cursor: "", has_more: false });
      }
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
  ml.FakeMarker.instances = [];
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.info).mockClear();
  vi.mocked(toast.error).mockClear();
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

async function mount(setup: Setup, basemapAvailable = true) {
  serve(setup);
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <CauHinhXaProvider giaTri={{ displayName: "Xã Thăng Bình", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" }}>
        <PhienProvider>
          <EconomicMapScreen basemapAvailable={basemapAvailable} />
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

  it("no basemap configured (BASEMAP_URL unset): no map, /phan-anh's sentence, no fallback; the register still loads", async () => {
    await mount({ permissions: ["asset.read"] }, false);
    expect(ml.FakeMap.instances).toHaveLength(0);
    expect(text()).toContain(BASEMAP_MISSING_SENTENCE);
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
    expect(expand.textContent).toBe("");
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
    await load();
    expect(ml.FakeMap.instances).toHaveLength(1); // re-renders after load did not build a second map
    expect(map.minZoom).toBe(12);
    act(() => root!.unmount());
    root = null;
    expect(map.removed).toBe(true);
  });

  it("one clustered GeoJSON source, points as [lng, lat], the centre landmark first, then the five asset layers (no selection ring)", async () => {
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
      "feedback-heat",
      "commune-centre-halo",
      "commune-centre-point",
      "commune-centre-label",
      "economic-clusters",
      "economic-cluster-count",
      "economic-points-halo",
      "economic-points",
      "economic-labels",
    ]);
  });

  it("labels: no runtime text-field rewrite (the self-hosted style already reads name:vi), and every font is one /basemap serves", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    expect(map.layout.filter(([, p]) => p === "text-field")).toEqual([]);
    const fonts = map.layers.flatMap((l) => ((l.layout as Record<string, unknown> | undefined)?.["text-font"] as string[] | undefined) ?? []);
    expect(fonts.length).toBeGreaterThan(0);
    for (const f of fonts) expect(BASEMAP_FONTS as readonly string[]).toContain(f);
  });

  it("the style is the SELF-HOSTED basemap: built in code, every URL this origin + /basemap, no OpenFreeMap", async () => {
    await mount({ permissions: ["asset.read"] });
    const style = theMap().opts.style as Record<string, unknown>;
    expect(typeof style).toBe("object");
    const urls = JSON.stringify(style).match(/(?:https?|pmtiles):\/\/[^"]+/g) ?? [];
    expect(urls.length).toBeGreaterThan(0);
    for (const u of urls) expect(u.replace(/^pmtiles:\/\//, "").startsWith(`${window.location.origin}/basemap/`), u).toBe(true);
    expect(JSON.stringify(style)).not.toMatch(/openfreemap|openmaptiles/i);
  });

  it("attribution: the style source's compact OpenStreetMap credit only — no OpenFreeMap string", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = theMap();
    expect(map.opts.attributionControl).toEqual({ compact: true });
    expect(JSON.stringify(map.opts.style)).toContain("OpenStreetMap");
    const custom = map.controls.filter((c) => (c as { opts?: Record<string, unknown> }).opts?.customAttribution !== undefined);
    expect(custom).toEqual([]);
  });
});

/* ---- "Bản đồ nhiệt phản ánh" — real petition points, gated by feedback.read ------------------------- */

describe("heat layer switch", () => {
  const heatSwitch = () => host!.querySelector<HTMLButtonElement>('[data-heat-switch] [role="switch"]')!;
  const pointCalls = () => calls.filter((c) => c.path.startsWith("/api/v1/citizen-report-points"));
  const visibility = (map: InstanceType<typeof ml.FakeMap>) =>
    map.layout.filter(([id, p]) => id === LAYER_HEAT && p === "visibility").at(-1)?.[2];

  it("DENIED: without feedback.read the switch is disabled, says why in title, and NO point is fetched", async () => {
    await mount({ permissions: ["asset.read"] });
    await load();
    const sw = heatSwitch();
    expect(sw.disabled).toBe(true);
    expect(sw.getAttribute("aria-checked")).toBe("false");
    expect(sw.title).toBe(HEAT_NEEDS_FEEDBACK_READ);
    await act(async () => sw.click());
    await settle();
    expect(pointCalls()).toEqual([]);
    // No "?" pending marker on this row any more.
    expect(host!.querySelector(`[data-heat-switch] [aria-label="${pendingMarkerLabel("Bản đồ nhiệt phản ánh")}"]`)).toBeNull();
  });

  it("feedback.read: enabled, off by default (nothing fetched); on → GET citizen-report-points, drawn beneath the pins", async () => {
    await mount({ permissions: ["asset.read", "feedback.read"] });
    const map = await load();
    const sw = heatSwitch();
    expect(sw.disabled).toBe(false);
    expect(sw.getAttribute("aria-checked")).toBe("false");
    expect(pointCalls()).toEqual([]);
    const heat = map.layers.find((l) => l.id === LAYER_HEAT)!;
    expect(heat).toMatchObject({ type: "heatmap", source: HEAT_SOURCE_ID, layout: { visibility: "none" } });
    expect(heat.paint).toMatchObject({ "heatmap-radius": 28, "heatmap-opacity": 0.45 });
    const ids = map.layers.map((l) => l.id);
    expect(ids.indexOf(LAYER_HEAT)).toBeLessThan(ids.indexOf(LAYER_CLUSTERS));
    expect(ids.indexOf(LAYER_HEAT)).toBeLessThan(ids.indexOf(LAYER_POINTS));

    await act(async () => sw.click());
    await settle();
    expect(pointCalls().map((c) => c.path)).toEqual(["/api/v1/citizen-report-points"]);
    expect(heatSwitch().getAttribute("aria-checked")).toBe("true");
    const data = map.getSource(HEAT_SOURCE_ID)!.data as { features: { geometry: { coordinates: number[] }; properties: object }[] };
    expect(data.features.map((f) => f.geometry.coordinates)).toEqual([
      [108.3, 15.7],
      [108.32, 15.75],
    ]);
    // Position only — not even the status travels into the map.
    expect(data.features.every((f) => Object.keys(f.properties).length === 0)).toBe(true);
    expect(visibility(map)).toBe("visible");

    await act(async () => heatSwitch().click());
    await settle();
    expect(heatSwitch().getAttribute("aria-checked")).toBe("false");
    expect(visibility(map)).toBe("none");
  });

  it("422 too_many_points: the server's sentence, and nothing drawn", async () => {
    await mount({
      permissions: ["asset.read", "feedback.read"],
      heatPoints: { status: 422, body: { code: "too_many_points", message: "Quá nhiều điểm, hãy thu hẹp bộ lọc." } },
    });
    const map = await load();
    await act(async () => heatSwitch().click());
    await settle();
    expect(text()).toContain("Quá nhiều điểm, hãy thu hẹp bộ lọc.");
    expect((map.getSource(HEAT_SOURCE_ID)!.data as { features: unknown[] }).features).toEqual([]);
    expect(visibility(map) ?? "none").toBe("none");
  });

  it("server error: a visible sentence, nothing drawn", async () => {
    await mount({ permissions: ["asset.read", "feedback.read"], heatPoints: { status: 500, body: { code: "x", message: "Máy chủ lỗi." } } });
    const map = await load();
    await act(async () => heatSwitch().click());
    await settle();
    const alert = host!.querySelector('[data-heat-switch-note][role="alert"]')!;
    expect(alert.textContent).toContain(HEAT_MAP_LOAD_FAILED);
    expect(alert.textContent).toContain("Máy chủ lỗi.");
    expect((map.getSource(HEAT_SOURCE_ID)!.data as { features: unknown[] }).features).toEqual([]);
  });

  it("no basemap: the switch is disabled even with feedback.read — no map, so no citizen coordinate is fetched", async () => {
    await mount({ permissions: ["asset.read", "feedback.read"] }, false);
    const sw = heatSwitch();
    expect(sw.disabled).toBe(true);
    expect(sw.title).toBe(HEAT_NEEDS_MAP);
    expect(pointCalls()).toEqual([]);
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

  it("with asset.update: Sửa and Bỏ xác minh side by side, Xoá khỏi bản đồ below", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openPoint("a1");
    expect(button("Sửa")).toBeDefined();
    expect(button("Bỏ xác minh")).toBeDefined();
    expect(button(DELETE_BUTTON)).toBeDefined();
  });
});

/* ---- detail panel: prototype `AssetCard.tsx` / spec 05 ------------------------------------------------ */

async function openPoint(id: string): Promise<void> {
  const map = await load();
  await act(async () => map.fire(`click:${LAYER_POINTS}`, { features: [{ properties: { id } }] }));
  await settle();
}

function panel(): HTMLElement {
  return host!.querySelector<HTMLElement>("aside.detail-panel")!;
}

function classes(el: Element | null | undefined): string[] {
  return (el?.getAttribute("class") ?? "").split(/\s+/);
}

async function typeInto(el: HTMLInputElement | HTMLTextAreaElement, value: string): Promise<void> {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("detail panel — prototype AssetCard (spec 05)", () => {
  it("header: 15px bold navy title, ghost h-7 px-2 close button with an accessible name", async () => {
    await mount({ permissions: ["asset.read"] });
    await openPoint("a1");
    const h2 = panel().querySelector("h2")!;
    for (const c of ["text-[15px]", "font-bold", "text-navy", "leading-snug"]) expect(classes(h2)).toContain(c);
    const close = button("Đóng thông tin đối tượng")!;
    for (const c of ["h-7", "px-2"]) expect(classes(close)).toContain(c);
    expect(classes(close.querySelector("svg"))).toContain("size-4");
    expect(classes(panel())).toContain("bg-white");
  });

  it("badges: verified leaf, status navy/8 — prototype classes, no invented third state", async () => {
    await mount({ permissions: ["asset.read"] });
    await openPoint("a1");
    const badges = Array.from(panel().querySelectorAll("[data-badge]"));
    const verified = badges.find((b) => b.textContent === "Đã xác minh")!;
    for (const c of ["bg-leaf/12", "text-leaf", "border-leaf/25"]) expect(classes(verified)).toContain(c);
    const status = badges.find((b) => b.textContent === "Đang hoạt động")!;
    for (const c of ["bg-navy/8", "text-navy", "border-line"]) expect(classes(status)).toContain(c);
    expect(text()).not.toContain("Cần cập nhật");
  });

  it("unverified badge: ink-muted on the line border", async () => {
    await mount({ permissions: ["asset.read"] });
    await openPoint("a2");
    const badge = Array.from(panel().querySelectorAll("[data-badge]")).find((b) => b.textContent === "Chưa xác minh")!;
    for (const c of ["bg-ink-muted/12", "text-ink-muted", "border-line"]) expect(classes(badge)).toContain(c);
  });

  it("facts: dl 12.5px, dt 11.5px muted; address '—' when empty; phone plain text; tax code in code 11.5px; VSIC 'code — label'; d/M/yyyy; pre-line description; 5-decimal coordinates", async () => {
    await mount({ permissions: ["asset.read"] });
    await openPoint("a1");
    const dl = panel().querySelector("dl")!;
    expect(classes(dl)).toContain("text-[12.5px]");
    for (const dt of Array.from(dl.querySelectorAll("dt"))) {
      expect(classes(dt)).toContain("text-[11.5px]");
      expect(classes(dt)).toContain("text-ink-muted");
    }
    const row = (label: string) =>
      Array.from(dl.querySelectorAll("dt")).find((d) => d.textContent === label)!.parentElement!.querySelector("dd")!;
    expect(row("Địa chỉ").textContent).toBe("—");
    // Rule 3: the number as the server sent it, and never in a URL.
    expect(row("Điện thoại").textContent).toBe("090****000");
    expect(panel().querySelector('a[href^="tel:"]')).toBeNull();
    const code = row("Mã số thuế").querySelector("code")!;
    expect(code.textContent).toBe("01******00");
    expect(classes(code)).toContain("text-[11.5px]");
    expect(row("Ngành nghề").textContent).toBe("10 — Chế biến thực phẩm");
    for (const c of ["text-navy", "text-right", "font-medium"]) expect(classes(row("Ngành nghề"))).toContain(c);
    expect(row("Thành lập").textContent).toBe("5/3/2020");
    expect(classes(row("Mô tả"))).toContain("whitespace-pre-line");
    expect(row("Toạ độ").textContent).toBe("15.70000, 108.30000");
    expect(classes(row("Toạ độ").querySelector("code"))).toContain("text-[11px]");
  });

  it("footer: Sửa outline · Xác minh primary (flex-1 each); Xác minh → POST confirmation and the prototype's toast", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openPoint("a2");
    const edit = button("Sửa")!;
    const verify = button("Xác minh")!;
    expect(classes(edit)).toContain("flex-1");
    expect(classes(verify)).toContain("flex-1");
    expect(classes(verify)).toContain("nut-chinh");
    await act(async () => verify.click());
    await settle();
    expect(calls.some((c) => c.path === "/api/v1/map-assets/a2/confirmation" && c.method === "POST")).toBe(true);
    expect(toast.success).toHaveBeenCalledWith("Đã đánh dấu đã xác minh.");
    // Kept against the prototype: the only way to undo a confirmation, in the same slot.
    expect(button("Bỏ xác minh")).toBeDefined();
  });

  it("delete is the INLINE box in the footer, with the prototype's sentence and a REQUIRED reason (rule 7) — no dialog", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openPoint("a1");
    await act(async () => button(DELETE_BUTTON)!.click());
    const box = panel().querySelector<HTMLElement>("[data-delete-confirm]")!;
    expect(box).not.toBeNull();
    expect(host!.querySelector("dialog")).toBeNull();
    for (const c of ["space-y-2", "rounded-[9px]", "border", "border-danger/30", "bg-danger/6", "p-2.5"]) expect(classes(box)).toContain(c);
    expect(box.textContent).toContain("Xoá khỏi bản đồ? Bản ghi vẫn được lưu vết, chỉ không hiện nữa.");
    const del = Array.from(box.querySelectorAll("button")).find((b) => b.textContent === "Xoá")!;
    const reason = box.querySelector<HTMLTextAreaElement>("textarea")!;
    expect(reason.required).toBe(true);
    expect(box.querySelector(`label[for="${reason.id}"]`)).not.toBeNull();
    // Blank or whitespace: Xoá stays disabled and nothing is sent.
    expect(del.disabled).toBe(true);
    await typeInto(reason, "   ");
    expect(del.disabled).toBe(true);
    await typeInto(reason, "Cơ sở đã chuyển đi");
    expect(del.disabled).toBe(false);
    await act(async () => del.click());
    await settle();
    const sent = calls.find((c) => c.method === "DELETE")!;
    expect(sent.path).toBe("/api/v1/map-assets/a1");
    expect(sent.body).toEqual({ reason: "Cơ sở đã chuyển đi" });
    expect(toast.success).toHaveBeenCalledWith("Đã xoá khỏi bản đồ.");
    expect(host!.querySelector("aside.detail-panel")).toBeNull();
  });

  it("Huỷ closes the box and sends nothing", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openPoint("a1");
    await act(async () => button(DELETE_BUTTON)!.click());
    await act(async () => button("Huỷ")!.click());
    expect(panel().querySelector("[data-delete-confirm]")).toBeNull();
    expect(button(DELETE_BUTTON)).toBeDefined();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  });

  it("refused: 'Không xoá được.' with the server's sentence; the panel stays open", async () => {
    await mount({ permissions: ["asset.read", "asset.update"], deleteStatus: 409 });
    await openPoint("a1");
    await act(async () => button(DELETE_BUTTON)!.click());
    await typeInto(panel().querySelector("textarea")!, "Trùng");
    await act(async () => button("Xoá")!.click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Không xoá được.", { description: "Bản ghi đang được người khác sửa." });
    expect(host!.querySelector("aside.detail-panel")).not.toBeNull();
  });
});

describe("detail panel — loading state (spec 05)", () => {
  it("aside w-80 border-l bg-white p-4 with a spinner, and a status for screen readers", async () => {
    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    act(() =>
      root!.render(
        <DetailPanel
          detail="loading"
          groupLabel={(c) => c}
          unitName={(c) => c}
          fields={[]}
          canUpdate
          busy={false}
          onClose={() => {}}
          onEdit={() => {}}
          onToggleVerified={() => {}}
          onDelete={async () => false}
        />,
      ),
    );
    const aside = host.querySelector("aside")!;
    // Beside the map from xl (the prototype's column); a bottom sheet below that, as the loaded panel.
    for (const c of ["xl:w-80", "xl:border-l", "bg-white", "p-4"]) expect(classes(aside)).toContain(c);
    expect(classes(aside.querySelector("svg"))).toContain("animate-spin");
    expect(aside.querySelector('[role="status"]')!.className).toContain("an-thi-giac");
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
    expect(toast.info).toHaveBeenCalledWith(OUTSIDE_FRAME);
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
    // The pin map of the form is on the same self-hosted basemap.
    await settle();
    expect(ml.FakeMap.instances).toHaveLength(2);
    const picker = ml.FakeMap.instances.at(-1)!;
    expect(JSON.stringify(picker.opts.style)).toContain(`${window.location.origin}/basemap/`);
    expect(JSON.stringify(picker.opts.style)).not.toMatch(/openfreemap/i);
    expect(picker.opts.attributionControl).toEqual({ compact: true });
  });

  it("header, in the prototype's order: group select · Mẫu Excel · Nhập Excel · Thêm đối tượng · Bản đồ / Sổ địa điểm · Trình chiếu", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    const header = host!.querySelector("header")!;
    expect(header.className).toContain("border-b");
    expect(header.className).toContain("px-5");
    expect(header.className).toContain("py-3");
    const h1 = header.querySelector("h1")!;
    expect(h1.className).toContain("text-[18px]");
    expect(h1.className).toContain("text-navy");
    expect(header.querySelector("[data-summary-line]")!.textContent).toBe("2 đối tượng · 50% đã xác minh");
    const controls = Array.from(header.querySelectorAll("select, button")).map((el) =>
      el.tagName === "SELECT" ? `select:${el.getAttribute("aria-label")}` : (el.textContent ?? ""),
    );
    expect(controls).toEqual([
      "select:Nhóm để nhập hoặc tải mẫu",
      "Mẫu Excel",
      "Nhập Excel",
      ADD_BUTTON,
      "Bản đồ",
      "Sổ địa điểm",
      PRESENTATION_BUTTON,
    ]);
  });

  it("the summary line says it is loading until the summary answers", async () => {
    serve({ permissions: ["asset.read"] });
    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    act(() =>
      root!.render(
        <CauHinhXaProvider giaTri={{ displayName: "Xã Thăng Bình", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" }}>
          <PhienProvider>
            <EconomicMapScreen basemapAvailable />
          </PhienProvider>
        </CauHinhXaProvider>,
      ),
    );
    expect(host.querySelector("header p")!.textContent).toBe("Đang tải danh mục…");
    await settle();
  });

  it("Mẫu Excel / Nhập Excel: outline, DISABLED, reason in title, and NO '?' marker (spec 08 A4)", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    const header = host!.querySelector("header")!;
    expect(header.querySelector("[data-pending-marker]")).toBeNull();
    for (const name of ["Mẫu Excel", "Nhập Excel"]) {
      const b = button(name)!;
      expect(b.disabled).toBe(true);
      expect(b.title).toBe(pendingPart(name).viSao);
      // The disabled button takes no pointer events, so the wrapper carries the title for the hover too.
      expect(b.parentElement!.getAttribute("title")).toBe(pendingPart(name).viSao);
    }
  });

  it("DENIED: asset.read only → Mẫu Excel stays, Nhập Excel is not offered", async () => {
    await mount({ permissions: ["asset.read"] });
    expect(button("Mẫu Excel")).toBeDefined();
    expect(button("Nhập Excel")).toBeUndefined();
  });

  it("segmented Bản đồ | Sổ địa điểm: the prototype's shell and item classes, aria-pressed follows the view", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = button("Bản đồ")!;
    const reg = button("Sổ địa điểm")!;
    const shell = map.parentElement!;
    for (const c of ["rounded-[10px]", "border", "border-line", "bg-canvas", "p-1", "flex", "gap-1"]) expect(shell.className.split(" ")).toContain(c);
    expect(map.getAttribute("aria-pressed")).toBe("true");
    for (const c of ["bg-white", "text-navy", "shadow-card", "rounded-[8px]", "text-[12.5px]", "font-semibold"]) expect(map.className.split(" ")).toContain(c);
    expect(reg.className.split(" ")).toContain("text-ink-muted");
    await act(async () => reg.click());
    await settle();
    expect(reg.getAttribute("aria-pressed")).toBe("true");
    expect(reg.className.split(" ")).toContain("bg-white");
    expect(button("Bản đồ")!.className.split(" ")).toContain("text-ink-muted");
  });
});

describe("Trình chiếu — the meeting-room mode (spec 03)", () => {
  const overlay = () => host!.querySelector<HTMLElement>("[data-map-presentation]");

  it("no longer an unbuilt part: no PHAN_CHUA_DUNG entry", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.ten === "Trình chiếu")).toBe(false);
  });

  it("opens a fixed overlay: H1 26px and the button only, 4 KPI cards (2 from by_type, 2 '—' with a reason), no aside, no scale bar, big pins; Esc closes", async () => {
    await mount({ permissions: ["asset.read", "asset.update", "admin.lookup"] });
    const map = await load();
    const toggle = button(PRESENTATION_BUTTON)!;
    expect(toggle.title).toBe(PRESENTATION_TITLE);
    expect(toggle.getAttribute("aria-pressed")).toBe("false");
    expect(overlay()).toBeNull();

    await act(async () => toggle.click());
    await settle();
    const o = overlay()!;
    for (const c of ["fixed", "inset-0", "z-50", "bg-canvas"]) expect(o.className.split(" ")).toContain(c);
    expect(document.body.style.overflow).toBe("hidden");
    const header = o.querySelector("header")!;
    expect(header.querySelector("h1")!.className).toContain("text-[26px]");
    expect(Array.from(header.querySelectorAll("button, select")).map((b) => b.textContent)).toEqual([PRESENTATION_BUTTON]);
    expect(header.querySelector("[data-summary-line]")).toBeNull();
    expect(button(PRESENTATION_BUTTON)!.getAttribute("aria-pressed")).toBe("true");
    // The button that was pressed keeps focus (the same element in both states).
    expect(document.activeElement === toggle || document.activeElement === o).toBe(true);

    const cards = Array.from(o.querySelectorAll<HTMLElement>("[data-kpi]"));
    expect(cards.map((c) => c.dataset.kpi)).toEqual(["doanh-nghiep", "ho-kinh-doanh", "lao-dong", "thanh-lap-moi"]);
    const value = (c: HTMLElement) => c.querySelector("[data-kpi-value]")!;
    expect(cards.map((c) => value(c).textContent)).toEqual(["4", "3", "—", "—"]);
    expect(cards.map((c) => c.querySelector("[data-kpi-label]")!.textContent)).toEqual([
      "Doanh nghiệp",
      "Hộ kinh doanh",
      "Lao động",
      "Thành lập mới năm nay",
    ]);
    // "—" is never a fake 0: the reason travels with it (title + text for a screen reader).
    for (const c of cards.slice(2)) {
      expect(c.title).not.toBe("");
      expect(c.textContent).toContain(c.title);
    }
    expect(cards[0]!.title).toBe("");
    expect(value(cards[0]!).className).toContain("text-[40px]");

    expect(o.querySelector("aside")).toBeNull();
    expect(button(FRAME_CHANGE_BUTTON)).toBeUndefined();
    expect(button(MAP_EXPAND)).toBeUndefined();
    // Same map, no rebuild; the scale bar is hidden and the pins grow.
    expect(ml.FakeMap.instances).toHaveLength(1);
    expect(host!.querySelector<HTMLElement>('[data-testid="economic-map"]')!.className).toContain("[&_.maplibregl-ctrl-scale]:hidden");
    expect(map.paint).toContainEqual([LAYER_POINTS, "circle-radius", 8]);
    expect(map.paint).toContainEqual([LAYER_HALO, "circle-radius", 13]);
    expect(map.layout).toContainEqual([LAYER_LABELS, "text-size", 14]);

    await act(async () => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    await settle();
    expect(overlay()).toBeNull();
    expect(document.body.style.overflow).toBe("");
    expect(host!.querySelector("aside")).not.toBeNull();
    expect(map.paint).toContainEqual([LAYER_POINTS, "circle-radius", 6]);
    expect(map.layout).toContainEqual([LAYER_LABELS, "text-size", 11]);
  });

  it("the map is shown even when Sổ địa điểm was open; leaving goes back to the register", async () => {
    await mount({ permissions: ["asset.read"] });
    await act(async () => button("Sổ địa điểm")!.click());
    await settle();
    expect(host!.querySelector('[data-testid="economic-map"]')).toBeNull();
    await act(async () => button(PRESENTATION_BUTTON)!.click());
    await settle();
    expect(host!.querySelector('[data-testid="economic-map"]')).not.toBeNull();
    await act(async () => button(PRESENTATION_BUTTON)!.click());
    await settle();
    expect(host!.querySelector('[data-testid="economic-map"]')).toBeNull();
    expect(button("Sổ địa điểm")!.getAttribute("aria-pressed")).toBe("true");
  });
});

describe("left column — prototype look (spec 04)", () => {
  it("section titles are 11.5px bold uppercase muted", async () => {
    await mount({ permissions: ["asset.read"] });
    for (const id of ["layers-title", "filters-title", "density-title"]) {
      const h = host!.querySelector(`#${id}`)!;
      for (const c of ["text-[11.5px]", "font-bold", "uppercase", "tracking-wide", "text-ink-muted"]) expect(h.className.split(" ")).toContain(c);
    }
  });

  it("filters: 11.5px labels, h-9 selects, the search box with its placeholder", async () => {
    await mount({ permissions: ["asset.read"] });
    const search = host!.querySelector<HTMLInputElement>("#map-search")!;
    expect(search.placeholder).toBe("Tên, địa chỉ, mã số thuế…");
    expect(search.className).toContain("pl-8");
    for (const id of ["map-industry", "map-unit", "map-status"]) {
      const sel = host!.querySelector<HTMLSelectElement>(`#${id}`)!;
      expect(sel.className.split(" ")).toContain("h-9");
      expect(sel.className.split(" ")).toContain("text-[12.5px]");
      expect(host!.querySelector(`label[for="${id}"]`)!.className).toContain("text-[11.5px]");
    }
  });

  it("Quy mô lao động stays disabled with a help hint; Mật độ theo thôn carries the hint, not the 'đang phát triển' sentence", async () => {
    await mount({ permissions: ["asset.read"] });
    expect(host!.querySelector<HTMLSelectElement>("#map-labour")!.disabled).toBe(true);
    const hints = Array.from(host!.querySelectorAll<HTMLElement>("[data-help-hint]"));
    expect(hints.map((h) => h.getAttribute("aria-label"))).toEqual([
      `Quy mô lao động: ${pendingPart("Quy mô lao động").viSao}`,
      `Mật độ theo thôn: ${pendingPart("Mật độ theo thôn").viSao}`,
    ]);
    expect(text()).not.toContain("tính năng đang phát triển");
    expect(host!.querySelector("aside [data-pending-marker]")).toBeNull();
  });
});

describe("points, halo and labels — prototype paint (AssetMap.tsx:200-251)", () => {
  it("pins r6, white 1.5 stroke, unverified .55; halo r10 .18 beneath; labels from z14, 11px navy with a white halo; centre #E5484D", async () => {
    await mount({ permissions: ["asset.read"] });
    const map = await load();
    const layer = (id: string) => map.layers.find((l) => l.id === id)!;
    expect(layer(LAYER_POINTS).paint).toMatchObject({
      "circle-radius": 6,
      "circle-stroke-width": 1.5,
      "circle-stroke-color": "#ffffff",
      "circle-opacity": ["case", ["==", ["get", "verified"], true], 1, 0.55],
    });
    expect(layer(LAYER_HALO)).toMatchObject({ type: "circle", source: SOURCE_ID, paint: { "circle-radius": 10, "circle-opacity": 0.18 } });
    expect((layer(LAYER_HALO).paint as Record<string, unknown>)["circle-color"]).toEqual((layer(LAYER_POINTS).paint as Record<string, unknown>)["circle-color"]);
    expect(layer(LAYER_LABELS)).toMatchObject({
      minzoom: 14,
      layout: { "text-size": 11, "text-offset": [0, 1.4], "text-max-width": 12 },
      paint: { "text-color": "#102B43", "text-halo-color": "#ffffff", "text-halo-width": 1.4 },
    });
    expect((layer(LAYER_CENTRE).paint as Record<string, unknown>)["circle-color"]).toBe("#E5484D");
    expect(layer(LAYER_CENTRE_LABEL).paint).toMatchObject({ "text-color": "#102B43", "text-halo-color": "#ffffff", "text-halo-width": 1.4 });
  });

  it("frame buttons float on the map, top-left, with the icon-only Mở rộng bản đồ", async () => {
    await mount({ permissions: ["asset.read", "admin.lookup"] });
    await load();
    const group = button(FRAME_CHANGE_BUTTON)!.parentElement!;
    for (const c of ["absolute", "top-3", "left-3", "z-10", "flex", "gap-2"]) expect(group.className.split(" ")).toContain(c);
    const expand = button(MAP_EXPAND)!;
    expect(expand.parentElement).toBe(group);
    expect(expand.textContent).toBe("");
    expect(expand.title).toBe(MAP_EXPAND);
    expect(button(FRAME_CHANGE_BUTTON)!.className).toContain("shadow-card");
  });
});

describe("Sổ địa điểm", () => {
  it("a group header folds and unfolds its rows", async () => {
    await mount({ permissions: ["asset.read"] });
    await openRegister();
    const toggle = host!.querySelector<HTMLButtonElement>('[data-group-toggle="cho"]')!;
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(text()).toContain("Ngoài khung");
    await act(async () => toggle.click());
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(text()).not.toContain("Ngoài khung");
    await act(async () => toggle.click());
    expect(text()).toContain("Ngoài khung");
  });

  it("the whole group row is clickable (bg-canvas, 12px bold navy), and the count stays muted", async () => {
    await mount({ permissions: ["asset.read"] });
    await openRegister();
    const toggle = host!.querySelector<HTMLButtonElement>('[data-group-toggle="cho"]')!;
    const tr = toggle.closest("tr")!;
    expect(classes(tr)).toContain("bg-canvas");
    expect(classes(tr)).toContain("cursor-pointer");
    for (const c of ["text-[12px]", "font-bold", "text-navy"]) expect(classes(toggle)).toContain(c);
    expect(toggle.textContent).toContain("1 địa điểm");
    // A click on the row, not on the label, folds the group too.
    await act(async () => tr.querySelector("th")!.click());
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
  });

  it("frame: wrapper h-full overflow-auto p-5; the card rounded-card border bg-white shadow-card; no sticky inner scroll", async () => {
    await mount({ permissions: ["asset.read"] });
    await openRegister();
    const table = host!.querySelector("table")!;
    const card = table.parentElement!;
    for (const c of ["overflow-x-auto", "rounded-card", "border", "border-line", "bg-white", "shadow-card"]) expect(classes(card)).toContain(c);
    expect(classes(card)).not.toContain("table-scroll--sticky");
    const wrapper = card.parentElement!;
    for (const c of ["h-full", "overflow-auto", "p-5"]) expect(classes(wrapper)).toContain(c);
  });

  it("columns at the prototype's widths; cells py-2; name 12.5px semibold navy, address 11px muted block; badge 10.5px", async () => {
    await mount({
      permissions: ["asset.read"],
      register: [
        { id: "far", asset_type_code: "cho", name: "Ngoài khung", address: "Thôn 3", representative: "N*** V***", phone: "090****000", status: "tam-ngung", verified: false },
      ],
    });
    await openRegister();
    const widths = Array.from(host!.querySelectorAll("thead th")).map((th) => th.className);
    expect(widths[0]).toContain("min-w-72");
    expect(widths[1]).toContain("w-44");
    expect(widths[2]).toContain("w-40");
    expect(widths[3]).toContain("w-28");
    expect(widths[4]).toContain("w-32");
    expect(widths[5]).toContain("w-24");
    const cells = Array.from(host!.querySelectorAll<HTMLTableCellElement>("tbody tr:not(:first-child) td"));
    for (const td of cells) expect(classes(td)).toContain("py-2");
    expect(classes(cells[0])).toContain("whitespace-normal");
    const name = Array.from(cells[0]!.querySelectorAll("span")).find((s) => s.textContent === "Ngoài khung")!;
    for (const c of ["text-[12.5px]", "font-semibold", "text-navy"]) expect(classes(name)).toContain(c);
    const addr = Array.from(cells[0]!.querySelectorAll("span")).find((s) => s.textContent === "Thôn 3")!;
    for (const c of ["block", "text-[11px]", "text-ink-muted"]) expect(classes(addr)).toContain(c);
    for (const c of ["text-[11.5px]", "text-ink-muted"]) expect(classes(cells[1])).toContain(c);
    for (const c of ["text-[11.5px]", "text-ink-muted"]) expect(classes(cells[2])).toContain(c);
    const phone = Array.from(cells[2]!.querySelectorAll("span")).find((s) => s.textContent === "090****000")!;
    for (const c of ["block", "tabular-nums"]) expect(classes(phone)).toContain(c);
    expect(classes(cells[3])).toContain("text-[11.5px]");
    expect(cells[3]!.textContent).toBe("Tạm ngừng");
    const badge = cells[4]!.querySelector("[data-badge]")!;
    expect(classes(badge)).toContain("text-[10.5px]");
    expect(badge.textContent).toBe("Chưa xác minh");
  });

  it("row actions: outline sm h-8 px-2 with the prototype's titles and names that say which place", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openRegister();
    const locate = button("Xem Ngoài khung trên bản đồ")!;
    const edit = button("Sửa Ngoài khung")!;
    const remove = button("Gỡ Ngoài khung khỏi bản đồ")!;
    expect(locate.title).toBe("Xem trên bản đồ");
    expect(edit.title).toBe("Sửa");
    expect(remove.title).toBe("Gỡ khỏi bản đồ");
    for (const b of [locate, edit, remove]) {
      for (const c of ["h-8", "px-2"]) expect(classes(b)).toContain(c);
      expect(classes(b.querySelector("svg"))).toContain("size-3.5");
    }
    expect(classes(remove)).toContain("text-danger");
  });

  it("DENIED: asset.read only — locate only, no Sửa and no Gỡ on the row", async () => {
    await mount({ permissions: ["asset.read"] });
    await openRegister();
    expect(button("Xem Ngoài khung trên bản đồ")).toBeDefined();
    expect(button("Sửa Ngoài khung")).toBeUndefined();
    expect(button("Gỡ Ngoài khung khỏi bản đồ")).toBeUndefined();
  });

  it("Gỡ asks ON THE ROW: Gỡ hẳn / Thôi and a REQUIRED reason; DELETE carries it; the prototype's toast", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openRegister();
    await act(async () => button("Gỡ Ngoài khung khỏi bản đồ")!.click());
    expect(host!.querySelector("dialog")).toBeNull();
    const confirm = button("Gỡ hẳn")!;
    for (const c of ["bg-danger", "text-white", "h-8", "px-2", "text-[11px]"]) expect(classes(confirm)).toContain(c);
    expect(button("Thôi")).toBeDefined();
    expect(button("Sửa Ngoài khung")).toBeUndefined();
    const reason = host!.querySelector<HTMLInputElement>("[data-row-delete-reason]")!;
    expect(reason.required).toBe(true);
    expect(host!.querySelector(`label[for="${reason.id}"]`)).not.toBeNull();
    expect(confirm.disabled).toBe(true);
    await typeInto(reason, "Đã giải thể từ năm trước");
    expect(button("Gỡ hẳn")!.disabled).toBe(false);
    await act(async () => button("Gỡ hẳn")!.click());
    await settle();
    const sent = calls.find((c) => c.method === "DELETE")!;
    expect(sent.path).toBe("/api/v1/map-assets/far");
    expect(sent.body).toEqual({ reason: "Đã giải thể từ năm trước" });
    expect(toast.success).toHaveBeenCalledWith("Đã gỡ Ngoài khung khỏi bản đồ.");
  });

  it("Thôi puts the buttons back and sends nothing; a refusal says 'Không gỡ được địa điểm này.'", async () => {
    await mount({ permissions: ["asset.read", "asset.update"], deleteStatus: 409 });
    await openRegister();
    await act(async () => button("Gỡ Ngoài khung khỏi bản đồ")!.click());
    await act(async () => button("Thôi")!.click());
    expect(button("Gỡ hẳn")).toBeUndefined();
    expect(button("Sửa Ngoài khung")).toBeDefined();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await act(async () => button("Gỡ Ngoài khung khỏi bản đồ")!.click());
    await typeInto(host!.querySelector<HTMLInputElement>("[data-row-delete-reason]")!, "Trùng");
    await act(async () => button("Gỡ hẳn")!.click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Không gỡ được địa điểm này.", { description: "Bản ghi đang được người khác sửa." });
  });

  it("empty, no filter: the prototype's card and sentences", async () => {
    await mount({ permissions: ["asset.read", "asset.update"], register: [] });
    await openRegister();
    const card = host!.querySelector<HTMLElement>("[data-register-empty]")!;
    for (const c of ["p-12", "text-center", "rounded-card", "border", "border-line", "bg-white", "shadow-card"]) expect(classes(card)).toContain(c);
    const [title, line] = Array.from(card.querySelectorAll("p"));
    expect(title!.textContent).toBe("Chưa có địa điểm nào");
    for (const c of ["text-[14px]", "font-semibold", "text-navy"]) expect(classes(title)).toContain(c);
    expect(line!.textContent).toBe("Thêm từng địa điểm bằng nút “Thêm đối tượng”, hoặc nhập cả danh sách từ Excel.");
    expect(classes(line)).toContain("text-[12.5px]");
  });

  it("empty BECAUSE OF A FILTER: says so, not 'Chưa có địa điểm nào'", async () => {
    await mount({ permissions: ["asset.read"], register: [] });
    await openRegister();
    const status = host!.querySelector<HTMLSelectElement>("#map-status")!;
    await act(async () => {
      status.value = "tam-ngung";
      status.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await settle();
    const card = host!.querySelector<HTMLElement>("[data-register-empty]")!;
    expect(card.textContent).toContain("Không có địa điểm nào khớp bộ lọc.");
    expect(card.textContent).not.toContain("Chưa có địa điểm nào");
  });

  it("loading: one Skeleton h-96 w-full, and a status for screen readers", async () => {
    await mount({ permissions: ["asset.read"], registerHold: true });
    await openRegister();
    const sk = host!.querySelector("[data-register-loading] span[aria-hidden]")!;
    for (const c of ["h-96", "w-full"]) expect(classes(sk)).toContain(c);
    expect(host!.querySelector("[data-register-loading] [role=status]")).not.toBeNull();
  });
});

async function openRegister(): Promise<void> {
  await act(async () => button("Sổ địa điểm")!.click());
  await settle();
}

/* ---- add / edit dialog: prototype `AssetFormDialog.tsx` + `LocationPicker.tsx` / spec 07 --------------- */

function dialog(): HTMLDialogElement {
  return host!.querySelector<HTMLDialogElement>("dialog")!;
}

function inDialog(label: string): HTMLButtonElement | undefined {
  return Array.from(dialog().querySelectorAll("button")).find((b) => b.textContent === label || b.getAttribute("aria-label") === label);
}

async function openAdd(setup: Partial<Setup> = {}): Promise<void> {
  await mount({ permissions: ["asset.read", "asset.update"], ...setup });
  await act(async () => button(ADD_BUTTON)!.click());
  await settle();
}

describe("add / edit dialog (spec 07)", () => {
  it("the app's dialog at sm:max-w-2xl, 86vh, scrolling; an ✕ top-right closes it", async () => {
    await openAdd();
    for (const c of ["max-w-2xl", "max-h-[86vh]", "overflow-auto"]) expect(classes(dialog())).toContain(c);
    const x = inDialog("Đóng")!;
    expect(x).toBeDefined();
    await act(async () => x.click());
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("Nhóm: no empty option, the header's group by default, enabled on create; selects and labels at the prototype's size", async () => {
    await openAdd();
    const group = host!.querySelector<HTMLSelectElement>("#asset-group")!;
    expect(Array.from(group.options).map((o) => o.value)).toEqual(["doanh-nghiep", "cho"]);
    expect(group.value).toBe("doanh-nghiep");
    expect(group.disabled).toBe(false);
    for (const c of ["mt-1.5", "h-9", "w-full", "rounded-md", "border-line", "bg-white", "px-3", "text-[13px]"]) expect(classes(group)).toContain(c);
    const label = host!.querySelector('label[for="asset-name"]')!;
    for (const c of ["text-sm", "font-medium"]) expect(classes(label)).toContain(c);
    expect(classes(host!.querySelector("#asset-name"))).toContain("mt-1.5");
  });

  it("Ngành nghề '01 — …' with '— Chưa phân ngành —'; Thôn '— Chưa xác định —'; no tax-code placeholder, no Bỏ toạ độ", async () => {
    await openAdd();
    const industry = host!.querySelector<HTMLSelectElement>("#asset-industry")!;
    expect(industry.options[0]!.textContent).toBe("— Chưa phân ngành —");
    expect(industry.options[1]!.textContent).toBe("01 — Nông nghiệp, trồng trọt, chăn nuôi");
    expect(host!.querySelector<HTMLSelectElement>("#asset-unit")!.options[0]!.textContent).toBe("— Chưa xác định —");
    expect(host!.querySelector<HTMLInputElement>("#asset-tax-code")!.placeholder).toBe("");
    expect(inDialog("Bỏ toạ độ")).toBeUndefined();
  });

  it("place search: disabled, generic placeholder, a help hint with the reason — no '?' marker", async () => {
    await openAdd();
    const search = host!.querySelector<HTMLInputElement>("#asset-place-search")!;
    expect(search.disabled).toBe(true);
    expect(search.placeholder).toBe("Tìm theo tên: chợ, trường học, thôn…");
    expect(dialog().querySelector("[data-help-hint]")!.getAttribute("aria-label")).toContain(pendingPart("Tìm địa điểm theo tên").viSao);
    expect(dialog().querySelector(`[aria-label="${pendingMarkerLabel("Tìm địa điểm theo tên")}"]`)).toBeNull();
  });

  it("edit: Nhóm is LOCKED", async () => {
    await mount({ permissions: ["asset.read", "asset.update"] });
    await openPoint("a2");
    await act(async () => button("Sửa")!.click());
    await settle();
    const group = host!.querySelector<HTMLSelectElement>("#asset-group")!;
    expect(group.value).toBe("cho");
    expect(group.disabled).toBe(true);
    expect(inDialog("Lưu")).toBeDefined();
  });

  it("empty name: 'Vui lòng nhập tên.' in the form, nothing sent; then POST and 'Đã thêm vào bản đồ.'", async () => {
    await openAdd();
    await act(async () => inDialog("Thêm vào bản đồ")!.click());
    await settle();
    expect(dialog().textContent).toContain("Vui lòng nhập tên.");
    expect(calls.some((c) => c.method === "POST" && c.path === "/api/v1/map-assets")).toBe(false);
    await typeInto(host!.querySelector<HTMLInputElement>("#asset-name")!, "Hợp tác xã C");
    await act(async () => inDialog("Thêm vào bản đồ")!.click());
    await settle();
    expect(calls.some((c) => c.method === "POST" && c.path === "/api/v1/map-assets")).toBe(true);
    expect(toast.success).toHaveBeenCalledWith("Đã thêm vào bản đồ.");
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("a required custom field carries the red ' *', and an empty one stops the save with the prototype's sentence", async () => {
    const field = { id: "f1", field_code: "loai_hinh", label: "Loại hình", value_type: "van-ban", is_required: true, is_active: true, sort_order: 1, options: [] };
    await openAdd({ fields: [field] });
    const label = host!.querySelector('label[for="asset-custom-loai_hinh"]')!;
    const star = label.querySelector("span.text-danger")!;
    expect(star.textContent).toBe(" *");
    await typeInto(host!.querySelector<HTMLInputElement>("#asset-name")!, "Hợp tác xã C");
    await act(async () => inDialog("Thêm vào bản đồ")!.click());
    await settle();
    expect(dialog().textContent).toContain("Còn trường bắt buộc chưa nhập.");
    expect(calls.some((c) => c.method === "POST" && c.path === "/api/v1/map-assets")).toBe(false);
  });

  it("picker: h-64 rounded-[10px] frame, crosshair, red MapPin marker anchored at the bottom, no compass, NO expand toggle", async () => {
    await openAdd();
    await settle();
    const frame = dialog().querySelector<HTMLElement>("[data-picker-frame]")!;
    for (const c of ["relative", "h-64", "overflow-hidden", "rounded-[10px]", "border", "border-line"]) expect(classes(frame)).toContain(c);
    expect(inDialog(MAP_EXPAND)).toBeUndefined();
    expect(inDialog(MAP_COLLAPSE)).toBeUndefined();
    const marker = ml.FakeMarker.instances.at(-1)!;
    expect(marker.opts).toMatchObject({ anchor: "bottom", draggable: true });
    const el = marker.opts!.element as HTMLElement;
    const svg = el.querySelector("svg")!;
    expect(svg.getAttribute("fill")).toBe("#E5484D");
    for (const c of ["size-7", "text-danger", "drop-shadow"]) expect(classes(svg)).toContain(c);
    const picker = ml.FakeMap.instances.at(-1)!;
    expect(picker.controls.some((c) => (c as { opts?: { showCompass?: boolean } }).opts?.showCompass === false)).toBe(true);
  });

  it("picker: no valid position → the hint inside the map; a typed valid position → flyTo in 400ms", async () => {
    await openAdd();
    await settle();
    expect(dialog().querySelector("[data-picker-hint]")).toBeNull();
    await typeInto(host!.querySelector<HTMLInputElement>("#asset-lat")!, "");
    const hint = dialog().querySelector<HTMLElement>("[data-picker-hint]")!;
    expect(hint.textContent).toBe("Bấm vào bản đồ để đặt ghim, rồi kéo ghim để chỉnh cho đúng.");
    for (const c of ["pointer-events-none", "absolute", "inset-x-0", "bottom-0", "bg-white/90", "text-[11.5px]"]) expect(classes(hint)).toContain(c);
    const picker = ml.FakeMap.instances.at(-1)!;
    await typeInto(host!.querySelector<HTMLInputElement>("#asset-lat")!, "15.75");
    expect(picker.flights.at(-1)).toMatchObject({ center: [108.35, 15.75], duration: 400 });
  });

  it("coordinates row: 11px labels, h-8 12px inputs, 'Vị trí của tôi' outline sm with its title", async () => {
    await openAdd();
    const row = host!.querySelector("#asset-lat")!.closest("[data-coordinates]")!;
    for (const c of ["grid", "sm:grid-cols-[1fr_1fr_auto]", "items-end", "gap-2"]) expect(classes(row)).toContain(c);
    expect(classes(row.querySelector('label[for="asset-lat"]'))).toContain("text-[11px]");
    for (const c of ["mt-1", "h-8", "text-[12px]"]) expect(classes(row.querySelector("#asset-lat"))).toContain(c);
    const me = inDialog("Vị trí của tôi")!;
    expect(me.title).toBe("Lấy toạ độ nơi đang đứng");
  });

  it("Vị trí của tôi: the prototype's three toasts", async () => {
    await openAdd();
    const original = Object.getOwnPropertyDescriptor(navigator, "geolocation");
    try {
      Object.defineProperty(navigator, "geolocation", { value: undefined, configurable: true });
      await act(async () => inDialog("Vị trí của tôi")!.click());
      expect(toast.error).toHaveBeenCalledWith("Trình duyệt này không cho lấy vị trí.");

      Object.defineProperty(navigator, "geolocation", {
        value: { getCurrentPosition: (_ok: unknown, fail: () => void) => fail() },
        configurable: true,
      });
      await act(async () => inDialog("Vị trí của tôi")!.click());
      expect(toast.error).toHaveBeenCalledWith("Không lấy được vị trí. Kiểm tra quyền định vị.");

      Object.defineProperty(navigator, "geolocation", {
        value: {
          getCurrentPosition: (ok: (p: { coords: { latitude: number; longitude: number } }) => void) =>
            ok({ coords: { latitude: 15.7512345678, longitude: 108.3512345678 } }),
        },
        configurable: true,
      });
      await act(async () => inDialog("Vị trí của tôi")!.click());
      expect(toast.success).toHaveBeenCalledWith("Đã lấy toạ độ hiện tại.");
      expect(host!.querySelector<HTMLInputElement>("#asset-lat")!.value).toBe("15.751235");
    } finally {
      if (original) Object.defineProperty(navigator, "geolocation", original);
      else delete (navigator as unknown as Record<string, unknown>).geolocation;
    }
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
    expect(toast.success).toHaveBeenCalledWith(FRAME_SAVED);
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
    expect(toast.success).toHaveBeenCalledWith(FRAME_SAVED);
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
    expect(toast.success).toHaveBeenCalledWith(FRAME_RESET_DONE);
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
    expect(toast.success).toHaveBeenCalledWith(FRAME_RESET_DONE_NO_DEFAULT);
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
