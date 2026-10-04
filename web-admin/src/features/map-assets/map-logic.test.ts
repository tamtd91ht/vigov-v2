import { describe, expect, it } from "vitest";

import type { comms_mapAssetPointsOut, comms_mapFrameOut } from "@/lib/api/schema.gen";

import {
  GROUP_COLOURS,
  GROUP_SWATCH_CLASS,
  VIETNAMESE_TEXT_FIELD,
  frameDefaultFromApi,
  frameFromApi,
  frameHintsFromApi,
  insideFrame,
  meanCentre,
  nameLabelLayers,
  pointsFilter,
  radiusUnusual,
  summaryLine,
  toEconomicCollection,
  visibleCollection,
} from "./map-logic";

const BOUNDS = [108.1, 15.5, 108.6, 15.9] as const;

/** The hint fields every map-frame reply carries (contract `comms_mapFrameOut`). */
const HINTS = { recommended_radius_km: 10, usual_radius_km: [3, 20], max_radius_km: 50, notice_version: "2026-10-04.1" };

function out(fields: Partial<comms_mapFrameOut> & { configured: boolean }): comms_mapFrameOut {
  return { ...HINTS, ...fields };
}

describe("frameFromApi — fail closed: anything but a well-formed configured frame is NO map", () => {
  it("configured frame → bounds exactly as the server sent them, and its source", () => {
    const f = frameFromApi(out({ configured: true, source: "commune", center_lat: 15.7, center_lng: 108.35, radius_km: 10, bounds: [...BOUNDS] }));
    expect(f?.bounds).toEqual(BOUNDS);
    expect(f?.source).toBe("commune");
    const d = frameFromApi(out({ configured: true, source: "default", center_lat: 15.7, center_lng: 108.35, radius_km: 10, bounds: [...BOUNDS] }));
    expect(d?.source).toBe("default");
  });

  it.each([
    ["configured:false", { configured: false }],
    ["no bounds", { configured: true, source: "commune", center_lat: 15.7, center_lng: 108.35, radius_km: 10 }],
    ["three numbers", { configured: true, source: "commune", center_lat: 1, center_lng: 1, radius_km: 1, bounds: [1, 2, 3] }],
    ["min > max", { configured: true, source: "commune", center_lat: 1, center_lng: 1, radius_km: 1, bounds: [108.6, 15.5, 108.1, 15.9] }],
    ["NaN", { configured: true, source: "commune", center_lat: 1, center_lng: 1, radius_km: 1, bounds: [Number.NaN, 15.5, 108.1, 15.9] }],
    ["the whole world", { configured: true, source: "commune", center_lat: 0, center_lng: 0, radius_km: 1, bounds: [-200, -85, 200, 85] }],
    ["no centre", { configured: true, source: "commune", radius_km: 1, bounds: [...BOUNDS] }],
    ["no source", { configured: true, center_lat: 15.7, center_lng: 108.35, radius_km: 10, bounds: [...BOUNDS] }],
    ["unknown source", { configured: true, source: "platform", center_lat: 15.7, center_lng: 108.35, radius_km: 10, bounds: [...BOUNDS] }],
  ])("%s → null", (_name, fields) => {
    expect(frameFromApi(out(fields))).toBeNull();
  });
});

describe("frameDefaultFromApi / frameHintsFromApi / radiusUnusual", () => {
  it("the default, when present and well-formed; null otherwise (never a guessed one)", () => {
    const def = { center_lat: 15.6, center_lng: 108.3, radius_km: 12, bounds: [...BOUNDS] };
    expect(frameDefaultFromApi(out({ configured: true, default: def }))).toEqual({
      centerLat: 15.6,
      centerLng: 108.3,
      radiusKm: 12,
      bounds: BOUNDS,
    });
    expect(frameDefaultFromApi(out({ configured: true }))).toBeNull();
    expect(frameDefaultFromApi(out({ configured: true, default: null }))).toBeNull();
    expect(frameDefaultFromApi(out({ configured: true, default: { ...def, bounds: [1, 2, 3] } }))).toBeNull();
  });

  it("hints are the server's figures; malformed ranges give no warning", () => {
    expect(frameHintsFromApi(out({ configured: false }))).toEqual({ recommendedKm: 10, usualKm: [3, 20], noticeVersion: "2026-10-04.1" });
    expect(frameHintsFromApi(out({ configured: false, usual_radius_km: [20, 3] })).usualKm).toBeNull();
    expect(radiusUnusual(2.9, [3, 20])).toBe(true);
    expect(radiusUnusual(3, [3, 20])).toBe(false);
    expect(radiusUnusual(20, [3, 20])).toBe(false);
    expect(radiusUnusual(20.1, [3, 20])).toBe(true);
    expect(radiusUnusual(45, null)).toBe(false);
  });
});

describe("insideFrame", () => {
  it("inside, on the edge, outside, and a swapped [lat, lng] pair", () => {
    expect(insideFrame(BOUNDS, 108.3, 15.7)).toBe(true);
    expect(insideFrame(BOUNDS, 108.1, 15.5)).toBe(true);
    expect(insideFrame(BOUNDS, 112.3, 16.5)).toBe(false); // Hoàng Sa direction
    expect(insideFrame(BOUNDS, 15.7, 108.3)).toBe(false);
    expect(insideFrame(BOUNDS, Number.NaN, 15.7)).toBe(false);
  });
});

function pts(...coords: unknown[]): comms_mapAssetPointsOut {
  return {
    type: "FeatureCollection",
    features: coords.map((c, i) => ({
      type: "Feature",
      id: `a${i}`,
      geometry: { type: "Point", coordinates: c as number[] },
      properties: { id: `a${i}`, asset_type_code: i % 2 === 0 ? "cho" : "ocop", name: `N${i}`, status: "dang-hoat-dong", verified: false },
    })),
  };
}

describe("toEconomicCollection — [lng, lat], invalid skipped and counted", () => {
  it("keeps [lng, lat] order unchanged and skips invalid coordinates", () => {
    const { collection, skipped } = toEconomicCollection(pts([108.3, 15.7], [15.7, 200], null, [108.2], ["x", 1], [108.4, 15.8]));
    expect(collection.features.map((f) => f.geometry.coordinates)).toEqual([
      [108.3, 15.7],
      [108.4, 15.8],
    ]);
    expect(skipped).toBe(4);
  });

  it("visibleCollection narrows by group without touching the original", () => {
    const { collection } = toEconomicCollection(pts([108.3, 15.7], [108.4, 15.8]));
    expect(visibleCollection(collection, new Set(["ocop"])).features.map((f) => f.id)).toEqual(["a1"]);
    expect(collection.features).toHaveLength(2);
  });

  it("meanCentre = mean of valid points; none → null", () => {
    expect(meanCentre(toEconomicCollection(pts([108.2, 15.6], [108.4, 15.8])).collection.features)).toEqual({ lat: 15.7, lng: 108.3 });
    expect(meanCentre([])).toBeNull();
  });
});

describe("layers", () => {
  it("points filter excludes clusters and keeps only visible groups", () => {
    expect(pointsFilter(new Set(["ocop", "cho"]))).toEqual([
      "all",
      ["!", ["has", "point_count"]],
      ["in", ["get", "asset_type_code"], ["literal", ["cho", "ocop"]]],
    ]);
  });

  it("eleven distinct colours, and the swatch classes carry the same colours", () => {
    const values = Object.values(GROUP_COLOURS);
    expect(values).toHaveLength(11);
    expect(new Set(values).size).toBe(11);
    for (const [code, hex] of Object.entries(GROUP_COLOURS)) expect(GROUP_SWATCH_CLASS[code]).toBe(`bg-[${hex}]`);
  });
});

describe("Vietnamese basemap labels (ADR 0072 H1)", () => {
  it("rewrites name labels only — not shields, not non-symbol layers", () => {
    const ids = nameLabelLayers([
      { id: "place", type: "symbol", layout: { "text-field": ["coalesce", ["get", "name_en"], ["get", "name"]] } },
      { id: "poi", type: "symbol", layout: { "text-field": "{name}" } },
      { id: "shield", type: "symbol", layout: { "text-field": "{ref}" } },
      { id: "icon-only", type: "symbol", layout: { "icon-image": "x" } },
      { id: "water", type: "fill" },
    ]);
    expect(ids).toEqual(["place", "poi"]);
    expect(VIETNAMESE_TEXT_FIELD).toEqual(["coalesce", ["get", "name:vi"], ["get", "name"]]);
  });
});

describe("summaryLine", () => {
  it("Vietnamese decimal comma", () => {
    expect(summaryLine(26, 0.4231)).toBe("26 đối tượng · 42,3% đã xác minh");
    expect(summaryLine(0, 0)).toBe("0 đối tượng · 0% đã xác minh");
  });
});
