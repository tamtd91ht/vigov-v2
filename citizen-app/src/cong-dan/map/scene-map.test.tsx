import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { NewsReadResult } from "../api/goi-vigov";
import { type CommuneMapFrame, communeMapFrameAddress, readCommuneMapFrame } from "../api/hop-dong-cong-khai";
import { accountlessReportBody, OPTIONAL_LOCATION_ADJUSTED_KEY, thanGuiPhanAnh } from "../api/hop-dong-phan-anh";
import { AddressBlock } from "../man/address-block";
import { XA_PA } from "../man/noi-dung";
import { COMMUNE_LOCATION_WORDS, sendBody } from "../man/PhanAnhAppXa";

import {
  BASEMAP_ATTRIBUTION,
  BASEMAP_SOURCE_ID,
  BASEMAP_TEXT_FIELD,
  buildCommuneBasemapStyle,
  communeBasemapBase,
  isOptionalBasemapSourceError,
  PLACES_ATTRIBUTION,
  PLACES_DOT_LAYER_ID,
  PLACES_FILE,
  PLACES_LABEL_LAYER_ID,
  PLACES_SOURCE_ID,
} from "./commune-basemap-style";
import { COMMUNE_BASEMAP_PATH } from "../api/hop-dong-cong-khai";
import { SceneMapArea } from "./scene-map";
import {
  frameOutcome,
  initialMapView,
  locationAdjustedFlag,
  mapNotice,
  PIN_ZOOM,
  placedLocation,
  type SceneMapState,
} from "./scene-map-logic";

/**
 * THE SEND FORM'S REAL MAP (ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M1–M5) — everything that is decided, not drawn.
 * MapLibre itself needs WebGL and is never loaded here (`scene-map.tsx` imports it inside an effect, which
 * `renderToStaticMarkup` never runs); drawing and dragging on a real phone are NOT covered by any test.
 */

// Invented numbers, not any real commune's frame.
const FRAME: CommuneMapFrame = { bounds: [108.3, 15.6, 108.5, 15.8], center: { lat: 15.7, lng: 108.4 } };
const INSIDE = { lat: 15.71234, lng: 108.41234 };
const OUTSIDE = { lat: 16.2, lng: 108.4 };
const DOMAIN = "xa-vi-du.vigov.example";
const READY: SceneMapState = {
  kind: "ready",
  frame: FRAME,
  base: "https://comms.api.vigov.vn/api/v1/commune-basemap",
  domain: DOMAIN,
};

const FORM = { linh_vuc: "moi-truong", noi_dung: "Rác ứ đọng", dia_chi: "", ho_ten: "Nguyễn Văn Thử", dien_thoai: "0900000000", an_danh: false };

const ok = (frame: CommuneMapFrame | null): NewsReadResult<{ frame: CommuneMapFrame | null }> => ({ kieu: "xong", gia_tri: { frame } });

describe("the commune's frame — read field by field", () => {
  it("configured:false is 'no frame' (null), not malformed", () => {
    expect(readCommuneMapFrame({ configured: false })).toBeNull();
  });

  it("a configured frame: bounds longitude first, the centre kept", () => {
    expect(
      readCommuneMapFrame({ configured: true, center_lat: 15.7, center_lng: 108.4, radius_km: 8, bounds: [108.3, 15.6, 108.5, 15.8] }),
    ).toEqual(FRAME);
  });

  it("a frame the map cannot honour is MALFORMED (undefined), never a guessed frame", () => {
    const good = { configured: true, center_lat: 15.7, center_lng: 108.4, bounds: [108.3, 15.6, 108.5, 15.8] };
    for (const bad of [
      null,
      "x",
      {},
      { configured: "true" },
      { ...good, bounds: [108.3, 15.6, 108.5] },
      { ...good, bounds: [108.5, 15.6, 108.3, 15.8] }, // west > east
      { ...good, bounds: [108.3, 15.8, 108.5, 15.6] }, // south > north
      { ...good, bounds: [108.3, 15.6, 181, 15.8] },
      { ...good, bounds: [108.3, "15.6", 108.5, 15.8] },
      { ...good, center_lat: 17 }, // centre outside its own box
      { ...good, center_lng: Number.NaN },
    ]) {
      expect(readCommuneMapFrame(bad), JSON.stringify(bad)).toBeUndefined();
    }
  });

  it("the read goes to comms with the domain as `host` and nothing else", () => {
    const u = new URL(communeMapFrameAddress("xa-vi-du.vigov.example"));
    expect(u.host).toBe("comms.api.vigov.vn");
    expect(u.pathname).toBe("/api/v1/commune-map-frame");
    expect([...u.searchParams.keys()]).toEqual(["host"]);
  });
});

describe("fallbacks — every failure is 'no map, send anyway'", () => {
  it("a frame AND a basemap address → ready; anything else → unavailable", () => {
    expect(frameOutcome(ok(FRAME), READY.base, DOMAIN)).toEqual(READY);
    expect(frameOutcome(ok(null), READY.base, DOMAIN)).toEqual({ kind: "unavailable" }); // frame not configured
    expect(frameOutcome(ok(FRAME), "", DOMAIN)).toEqual({ kind: "unavailable" }); // no `comms` host: no basemap
    expect(frameOutcome(ok(FRAME), READY.base, "")).toEqual({ kind: "unavailable" }); // no domain: comms would 400
    for (const kieu of ["khong-hop-le", "khong-thay", "tam-ngung", "loi-may-chu", "loi-mang", "chua-cau-hinh"] as const) {
      expect(frameOutcome({ kieu }, READY.base, DOMAIN), kieu).toEqual({ kind: "unavailable" });
    }
    expect(frameOutcome({ kieu: "rate-limited", retryAfterSeconds: 3 }, READY.base, DOMAIN)).toEqual({ kind: "unavailable" });
  });

  it("each state says ONE sentence in words — no code, no grey box", () => {
    const render = (state: SceneMapState) =>
      renderToStaticMarkup(createElement(SceneMapArea, { state, location: null, onPlace: () => {}, onFail: () => {} }));
    expect(render({ kind: "off" })).toBe("");
    expect(render({ kind: "loading" })).toContain(XA_PA.map_loading);
    const down = render({ kind: "unavailable" });
    expect(down).toContain(XA_PA.map_unavailable);
    expect(down).toContain('role="status"');
    expect(down).not.toContain("xa-scene-map__canvas");
    expect(XA_PA.map_unavailable).toMatch(/bà con|Bà con/);
    expect(XA_PA.map_unavailable).toMatch(/vẫn gửi phản ánh được/);
    expect(XA_PA.map_unavailable).not.toMatch(/\d{3}|error|lỗi mã/i);
    // Ready but not drawn yet: the canvas region, labelled, and "loading" in words — nothing fetched by rendering.
    const ready = render(READY);
    expect(ready).toContain(`aria-label="${XA_PA.map_label}"`);
    expect(ready).toContain(XA_PA.map_loading);
    expect(ready).not.toMatch(/<img|<script|<iframe/);
  });

  it("the sentence under the map, once drawn: the hint, or 'outside' for a Zalo point outside the frame", () => {
    expect(mapNotice({ kind: "off" }, INSIDE, true)).toBeNull();
    expect(mapNotice(READY, null, false)).toBe("loading");
    expect(mapNotice(READY, null, true)).toBe("hint");
    expect(mapNotice(READY, INSIDE, true)).toBe("hint");
    expect(mapNotice(READY, OUTSIDE, true)).toBe("outside");
    expect(mapNotice({ kind: "unavailable" }, INSIDE, true)).toBe("unavailable");
  });
});

describe("where the map opens, and what a tap or a drag keeps", () => {
  it("on Zalo's point when it is inside the frame; on the whole frame otherwise", () => {
    expect(initialMapView(FRAME, INSIDE)).toEqual({ center: INSIDE, zoom: PIN_ZOOM });
    expect(initialMapView(FRAME, OUTSIDE)).toEqual({ center: FRAME.center, bounds: FRAME.bounds });
    expect(initialMapView(FRAME, null)).toEqual({ center: FRAME.center, bounds: FRAME.bounds });
  });

  it("a placement is kept only as a valid pair inside the frame", () => {
    expect(placedLocation({ lng: 108.41, lat: 15.71 }, FRAME)).toEqual({ lat: 15.71, lng: 108.41 });
    expect(placedLocation({ lng: 108.41, lat: 16.5 }, FRAME)).toBeNull();
    expect(placedLocation({ lng: Number.NaN, lat: 15.71 }, FRAME)).toBeNull();
  });
});

describe("pin → location_citizen_adjusted (M3)", () => {
  it("no coordinates → omitted; Zalo's untouched → false; moved on the map → true", () => {
    expect(locationAdjustedFlag(null, false)).toBeNull();
    expect(locationAdjustedFlag(null, true)).toBeNull();
    expect(locationAdjustedFlag(INSIDE, false)).toBe(false);
    expect(locationAdjustedFlag(INSIDE, true)).toBe(true);
  });

  it("the send body carries the flag ONLY with the pair", () => {
    const body = (location: typeof INSIDE | null, moved: boolean) =>
      JSON.parse(sendBody(FORM, location, false, "", locationAdjustedFlag(location, moved))) as Record<string, unknown>;
    expect(body(null, true)).not.toHaveProperty(OPTIONAL_LOCATION_ADJUSTED_KEY);
    expect(body(INSIDE, false)[OPTIONAL_LOCATION_ADJUSTED_KEY]).toBe(false);
    const moved = body(INSIDE, true);
    expect(moved[OPTIONAL_LOCATION_ADJUSTED_KEY]).toBe(true);
    expect([moved["lat"], moved["lng"]]).toEqual([INSIDE.lat, INSIDE.lng]);
    // The old call shape (no fifth argument) is unchanged: no key at all.
    expect(JSON.parse(sendBody(FORM, INSIDE))).not.toHaveProperty(OPTIONAL_LOCATION_ADJUSTED_KEY);
    // A flag handed in with an invalid pair goes nowhere — `true` alone would be the server's 422.
    const lone = JSON.parse(
      thanGuiPhanAnh({ noi_dung: "n", dia_chi: "", ho_ten: "", dien_thoai: "", an_danh: false, scene_location: { lat: 99, lng: 0 }, location_citizen_adjusted: true }),
    ) as Record<string, unknown>;
    expect(Object.keys(lone)).not.toContain(OPTIONAL_LOCATION_ADJUSTED_KEY);
    expect(Object.keys(lone)).not.toContain("lat");
  });

  it("the accountless body never carries it — that route answers 400 to the key", () => {
    const body = JSON.parse(
      accountlessReportBody(
        { noi_dung: "n", dia_chi: "", ho_ten: "x", dien_thoai: "0900000000", an_danh: false, scene_location: INSIDE, location_citizen_adjusted: true },
        "xa-vi-du.vigov.example",
      ),
    ) as Record<string, unknown>;
    expect(body).not.toHaveProperty(OPTIONAL_LOCATION_ADJUSTED_KEY);
    expect(body).not.toHaveProperty("lat");
  });
});

describe("the address block with the map", () => {
  const block = (map: Parameters<typeof AddressBlock>[0]["map"]) =>
    renderToStaticMarkup(
      createElement(AddressBlock, {
        address: "",
        maxLength: 500,
        onAddress: () => {},
        canLocate: true,
        words: COMMUNE_LOCATION_WORDS,
        locating: false,
        location: INSIDE,
        failure: null,
        zalo: null,
        onLocate: () => {},
        map,
      }),
    );

  it("the coordinates line stays — the map's text alternative — and says WHO placed the point", () => {
    const zalo = block({ state: READY, placedOnMap: false, onPlace: () => {}, onFail: () => {} });
    expect(zalo).toContain("Đã ghi nhận vị trí (15.71234, 108.41234)");
    const moved = block({ state: READY, placedOnMap: true, onPlace: () => {}, onFail: () => {} });
    expect(moved).toContain(XA_PA.map_placed("15.71234, 108.41234"));
    expect(moved).not.toContain("Đã ghi nhận vị trí");
    // The address box and the location button are still there with a map.
    expect(moved).toContain('id="xa-dia-chi"');
    expect(moved).toContain("Lấy lại vị trí hiện tại");
  });

  it("no map prop (accountless, outside Zalo): no map area and no map sentence at all", () => {
    const html = block(undefined);
    expect(html).not.toContain("xa-scene-map");
    expect(html).not.toContain(XA_PA.map_unavailable);
  });

  it("the old SVG sketch and its 'minh hoạ' caption are gone", () => {
    expect(block(undefined)).not.toContain("xa-mini-map");
    expect(block(undefined)).not.toContain("minh hoạ");
  });
});

describe("the basemap style — ViGov's comms only, the owner's three rewrites", () => {
  const base = communeBasemapBase();
  const style = buildCommuneBasemapStyle(base, DOMAIN);

  it("the base is comms' public basemap route", () => {
    const u = new URL(base);
    expect(u.host).toBe("comms.api.vigov.vn");
    expect(u.pathname).toBe(COMMUNE_BASEMAP_PATH);
  });

  it("EVERY URL in the style is on comms, https — no third-party host for tiles, glyphs or sprite", () => {
    const urls: string[] = [];
    const walk = (v: unknown) => {
      if (typeof v === "string") {
        if (/^[a-z]+:\/\//i.test(v)) urls.push(v);
      } else if (Array.isArray(v)) v.forEach(walk);
      else if (v !== null && typeof v === "object") Object.values(v).forEach(walk);
    };
    walk(style);
    expect(urls.length).toBeGreaterThanOrEqual(3);
    for (const url of urls) {
      const plain = url.replace(/^pmtiles:\/\//, "");
      expect(plain.startsWith(`${base}/`), url).toBe(true);
    }
  });

  it("EVERY file asks with `?host=<domain>` — comms refuses a basemap request that names no commune (400)", () => {
    // Every source, not only protomaps: a second archive without `?host=` would be a 400 that only shows on a phone.
    const sourceUrls = Object.values(style.sources).map((s) => (s as { url: string }).url.replace(/^pmtiles:\/\//, ""));
    expect(Object.keys(style.sources).sort()).toEqual([BASEMAP_SOURCE_ID, PLACES_SOURCE_ID].sort());
    for (const url of [style.glyphs!, String(style.sprite), ...sourceUrls]) {
      const u = new URL(url.replace("{fontstack}", "Noto Sans Regular").replace("{range}", "0-255"));
      expect([...u.searchParams.entries()], url).toEqual([["host", DOMAIN]]);
    }
    expect(() => buildCommuneBasemapStyle(base, "")).toThrow();
  });

  it("refuses a base that is not plain https", () => {
    for (const bad of ["", "http://comms.api.vigov.vn/x", "https://u:p@comms.api.vigov.vn/x", "https://comms.api.vigov.vn/x?k=1"]) {
      expect(() => buildCommuneBasemapStyle(bad, DOMAIN), bad).toThrow();
    }
  });

  it("labels read name:vi then name; boundaries carry the disputed/maritime guard; the OSM credit rides on the source", () => {
    const textField = (l: (typeof style.layers)[number]) =>
      JSON.stringify((("layout" in l ? l.layout : undefined) as Record<string, unknown> | undefined)?.["text-field"] ?? "");
    const named = style.layers.filter((l) => l.type === "symbol" && textField(l).includes("name"));
    expect(named.length).toBeGreaterThan(0);
    for (const l of named) {
      const tf = textField(l);
      // Shields / house numbers keep theirs; every place-name label is exactly the owner's coalesce.
      if (/"(?:pgf:)?name/.test(tf)) expect(tf, l.id).toBe(JSON.stringify(BASEMAP_TEXT_FIELD));
    }
    const boundaries = style.layers.filter((l) => "source-layer" in l && l["source-layer"] === "boundaries");
    expect(boundaries.length).toBeGreaterThan(0);
    for (const l of boundaries) {
      const f = JSON.stringify("filter" in l ? l.filter : null);
      expect(f, l.id).toContain('["!",["to-boolean",["get","disputed"]]]');
      expect(f, l.id).toContain('["!",["to-boolean",["get","maritime"]]]');
    }
    expect((style.sources["protomaps"] as { attribution?: string }).attribution).toBe(BASEMAP_ATTRIBUTION);
    expect(BASEMAP_ATTRIBUTION).toContain("OpenStreetMap");
    expect((style.sources[PLACES_SOURCE_ID] as { attribution?: string }).attribution).toBe(PLACES_ATTRIBUTION);
    expect(PLACES_ATTRIBUTION).toBe("© Overture Maps Foundation");
  });
});

describe("the Overture places layer (ADR 0072 §Sửa đổi 12/10/2026) — part of the basemap, optional", () => {
  const base = communeBasemapBase();
  const style = buildCommuneBasemapStyle(base, DOMAIN);

  it("the places archive: same route and `?host=` as the mainland one, zoom 13–15", () => {
    const src = style.sources[PLACES_SOURCE_ID] as { type: string; url: string; minzoom: number; maxzoom: number };
    expect(src.type).toBe("vector");
    expect(src.url).toBe(`pmtiles://${base}/${PLACES_FILE}?host=${DOMAIN}`);
    expect(PLACES_FILE).toBe("vn-places.pmtiles");
    expect([src.minzoom, src.maxzoom]).toEqual([13, 15]);
  });

  it("dot and label, appended AFTER every Protomaps layer, from zoom 13; the label is the owner's coalesce", () => {
    const ids = style.layers.map((l) => l.id);
    expect(ids.slice(-2)).toEqual([PLACES_DOT_LAYER_ID, PLACES_LABEL_LAYER_ID]);
    for (const l of style.layers.slice(0, -2)) expect("source" in l ? l.source : BASEMAP_SOURCE_ID, l.id).toBe(BASEMAP_SOURCE_ID);
    const [dot, label] = style.layers.slice(-2) as Array<Record<string, unknown>>;
    expect(dot).toMatchObject({ type: "circle", source: PLACES_SOURCE_ID, "source-layer": "places", minzoom: 13 });
    expect(label).toMatchObject({ type: "symbol", source: PLACES_SOURCE_ID, "source-layer": "places", minzoom: 13 });
    const layout = label!["layout"] as Record<string, unknown>;
    expect(JSON.stringify(layout["text-field"])).toBe(JSON.stringify(BASEMAP_TEXT_FIELD));
    expect(layout["text-font"]).toEqual(["Noto Sans Regular"]);
    expect(layout["text-allow-overlap"]).toBe(false);
  });

  it("an error from the places source does not give up the map; a protomaps / glyph / sprite error still does", () => {
    expect(isOptionalBasemapSourceError({ type: "error", sourceId: PLACES_SOURCE_ID, error: new Error("x") })).toBe(true);
    expect(isOptionalBasemapSourceError({ type: "error", sourceId: BASEMAP_SOURCE_ID, error: new Error("x") })).toBe(false);
    // Glyph and sprite failures carry no sourceId.
    expect(isOptionalBasemapSourceError({ type: "error", error: new Error("x") })).toBe(false);
    for (const odd of [null, undefined, "overture", 0]) expect(isOptionalBasemapSourceError(odd)).toBe(false);
  });
});
