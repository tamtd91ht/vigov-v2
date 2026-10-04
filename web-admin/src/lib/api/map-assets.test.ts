import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createMapAsset,
  deleteMapAsset,
  listMapAssetPoints,
  listMapAssets,
  seedMapAssetTypeDefaults,
  setMapAssetConfirmation,
  updateMapAsset,
} from "./map-assets";
import { getMapFrame, putMapFrame } from "./map-frame";

type Call = { path: string; init: RequestInit };

function fake(status: number, body: unknown = {}): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit) => {
      calls.push({ path, init });
      return new Response(status === 204 ? null : JSON.stringify(body), { status });
    }),
  );
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

describe("map-asset clients — relative paths, no tenant anywhere, contract names", () => {
  it("points: only the server filters that are set; never a group (toggles are client-side)", async () => {
    const calls = fake(200, { type: "FeatureCollection", features: [] });
    await listMapAssetPoints({ q: " chợ ", industryCode: "47", residentialUnitId: "", status: "" });
    expect(calls[0]!.path).toBe("/api/v1/map-asset-points?q=ch%E1%BB%A3&industry_code=47");
    expect(calls[0]!.path).not.toContain("tenant");
  });

  it("register page: limit + cursor", async () => {
    const calls = fake(200, { items: [], next_cursor: "", has_more: false });
    await listMapAssets({ q: "", industryCode: "", residentialUnitId: "u1", status: "tam-ngung" }, "c1", 100);
    expect(calls[0]!.path).toBe("/api/v1/map-assets?residential_unit_id=u1&status=tam-ngung&limit=100&cursor=c1");
  });

  it("create: Idempotency-Key header, 201, no `verified` in the body", async () => {
    const calls = fake(201, { id: "x" });
    const r = await createMapAsset({ asset_type_code: "cho", name: "Chợ", lat: 15.7, lng: 108.3 }, "k-1");
    expect(r.ok).toBe(true);
    expect((calls[0]!.init.headers as Record<string, string>)["Idempotency-Key"]).toBe("k-1");
    expect(JSON.parse(String(calls[0]!.init.body))).not.toHaveProperty("verified");
  });

  it("patch sends only fields present", async () => {
    const calls = fake(200, { id: "x" });
    await updateMapAsset("01J/A", { name: "B", address: null });
    expect(calls[0]!.path).toBe("/api/v1/map-assets/01J%2FA");
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ name: "B", address: null });
  });

  it("delete: reason in the body, 204 is success", async () => {
    const calls = fake(204);
    expect(await deleteMapAsset("a", "Trùng")).toEqual({ ok: true, duLieu: null });
    expect(calls[0]!.init.method).toBe("DELETE");
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ reason: "Trùng" });
  });

  it("confirmation and defaults", async () => {
    const calls = fake(200, {});
    await setMapAssetConfirmation("a", false);
    await seedMapAssetTypeDefaults();
    expect(calls[0]!.path).toBe("/api/v1/map-assets/a/confirmation");
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ verified: false });
    expect(calls[1]!.path).toBe("/api/v1/map-asset-types/defaults");
    expect(calls[1]!.init.body).toBeUndefined();
  });

  it("frame: GET and PUT on /api/v1/map-frame; a 422 is the server's own sentence", async () => {
    let calls = fake(200, { configured: false });
    await getMapFrame();
    expect(calls[0]!.path).toBe("/api/v1/map-frame");
    calls = fake(422, { code: "center_outside_mainland", message: "Tâm khung phải nằm trên đất liền Việt Nam." });
    const r = await putMapFrame({ center_lat: 16.5, center_lng: 112.3, radius_km: 10 });
    expect(r).toEqual({ ok: false, thongBao: "Tâm khung phải nằm trên đất liền Việt Nam." });
    expect(calls[0]!.init.method).toBe("PUT");
  });
});
