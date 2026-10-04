import { describe, expect, it } from "vitest";

import type { comms_mapAssetOut, comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";

import { checkPosition, createBody, draftFromAsset, newAssetDraft, updateBody } from "./asset-form";
import { PIN_OUTSIDE_FRAME, REQUIRED_GROUP, REQUIRED_NAME, REQUIRED_POSITION } from "./labels";

const BOUNDS = [108.1, 15.5, 108.6, 15.9] as const;
const CENTRE = { lat: 15.7, lng: 108.35 };

function field(code: string, type: string, extra: Partial<comms_mapFieldSchemaOut> = {}): comms_mapFieldSchemaOut {
  return {
    id: code,
    asset_type_code: "doanh-nghiep",
    field_code: code,
    label: code,
    value_type: type,
    options: [],
    is_required: false,
    sort_order: 0,
    is_active: true,
    ...extra,
  };
}

const SAVED: comms_mapAssetOut = {
  id: "01JA",
  asset_type_code: "doanh-nghiep",
  name: "Công ty A",
  address: "Thôn 1",
  lat: 15.7,
  lng: 108.35,
  representative: "Nguyễn Văn Hùng",
  phone: "0900000000",
  status: "dang-hoat-dong",
  verified: false,
  tax_code: "0100000000",
  custom_values: { revenue: 100 },
  masked: false,
  created_at: "2026-10-04T00:00:00Z",
  updated_at: "2026-10-04T00:00:00Z",
};

describe("create — required group, name, position; the pin stays in the frame", () => {
  const ok = { ...newAssetDraft("cho", CENTRE), name: "Chợ Bình Trị" };

  it("new draft starts at the FRAME centre, never a coordinate written in code", () => {
    expect(newAssetDraft("cho", CENTRE)).toMatchObject({ lat: "15.700000", lng: "108.350000" });
    expect(newAssetDraft("cho", null)).toMatchObject({ lat: "", lng: "" });
  });

  it("missing group / name / position each stop the send with their sentence", () => {
    expect(createBody({ ...ok, assetTypeCode: "" }, [], BOUNDS)).toEqual({ kind: "error", message: REQUIRED_GROUP });
    expect(createBody({ ...ok, name: "  " }, [], BOUNDS)).toEqual({ kind: "error", message: REQUIRED_NAME });
    expect(createBody({ ...ok, lat: "" }, [], BOUNDS)).toEqual({ kind: "error", message: REQUIRED_POSITION });
  });

  it("a pin OUTSIDE the frame is refused", () => {
    expect(createBody({ ...ok, lat: "16.500000", lng: "112.300000" }, [], BOUNDS)).toEqual({ kind: "error", message: PIN_OUTSIDE_FRAME });
    expect(checkPosition("15.7", "108.35", BOUNDS)).toEqual({ ok: true, lat: 15.7, lng: 108.35 });
  });

  it("more than 6 decimals is refused", () => {
    expect(checkPosition("15.7000001", "108.35", BOUNDS).ok).toBe(false);
  });

  it("body: no `verified`, empty optionals absent, custom values typed", () => {
    const fields = [field("revenue", "so-thap-phan"), field("export", "dung-sai"), field("note", "van-ban")];
    const r = createBody({ ...ok, custom: { revenue: "12,5", export: "true", note: "" } }, fields, BOUNDS);
    expect(r.kind).toBe("send");
    if (r.kind !== "send") return;
    expect(r.body).not.toHaveProperty("verified");
    expect(r.body.address).toBeUndefined();
    expect(r.body.custom_values).toEqual({ revenue: 12.5, export: true });
    expect(r.body.lat).toBe(15.7);
    expect(r.body.lng).toBe(108.35);
  });

  it("a required custom field left empty stops the send", () => {
    const r = createBody(ok, [field("legal_form", "chon", { is_required: true, label: "Loại hình" })], BOUNDS);
    expect(r).toEqual({ kind: "error", message: "Hãy nhập Loại hình." });
  });

  it("a disabled field is not on the form and not demanded", () => {
    const r = createBody(ok, [field("old", "van-ban", { is_required: true, is_active: false })], BOUNDS);
    expect(r.kind).toBe("send");
  });
});

describe("update — only what changed; masked values never go back up", () => {
  it("unchanged → nothing to send", () => {
    expect(updateBody(SAVED, draftFromAsset(SAVED), [field("revenue", "so-thap-phan")], BOUNDS)).toEqual({ kind: "unchanged" });
  });

  it("one changed field → a body with that field only; emptied text → null (clear)", () => {
    const d = { ...draftFromAsset(SAVED), name: "Công ty B", address: "" };
    const r = updateBody(SAVED, d, [], BOUNDS);
    expect(r.kind).toBe("send");
    if (r.kind !== "send") return;
    const sent = JSON.parse(JSON.stringify(r.body)) as Record<string, unknown>;
    expect(sent).toEqual({ name: "Công ty B", address: null });
  });

  it("MASKED asset: representative / phone / tax code are never sent, even if typed", () => {
    const masked: comms_mapAssetOut = { ...SAVED, masked: true, representative: "N*** H***", phone: "09******00", tax_code: "01******00" };
    const d = draftFromAsset(masked);
    expect(d.representative).toBe("");
    expect(d.phone).toBe("");
    const r = updateBody(masked, { ...d, phone: "0911111111", name: "Đổi tên" }, [], BOUNDS);
    expect(r.kind).toBe("send");
    if (r.kind !== "send") return;
    const sent = JSON.parse(JSON.stringify(r.body)) as Record<string, unknown>;
    expect(sent).toEqual({ name: "Đổi tên" });
  });

  it("moving the pin outside the frame is refused on edit too", () => {
    const r = updateBody(SAVED, { ...draftFromAsset(SAVED), lat: "10.000000", lng: "106.000000" }, [], BOUNDS);
    expect(r).toEqual({ kind: "error", message: PIN_OUTSIDE_FRAME });
  });

  it("an emptied custom value goes up as null; an untouched one does not go up", () => {
    const fields = [field("revenue", "so-thap-phan"), field("note", "van-ban")];
    const r = updateBody(SAVED, { ...draftFromAsset(SAVED), custom: { revenue: "" } }, fields, BOUNDS);
    expect(r.kind).toBe("send");
    if (r.kind !== "send") return;
    expect(r.body.custom_values).toEqual({ revenue: null });
  });
});
