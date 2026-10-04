import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { ApiError, type CommuneDetail, type MapFrameDefault } from "@/lib/api";
import { AUTH_UNAVAILABLE, handleGuardedError, INVALID_BODY } from "@/lib/errors";
import { canManageCommune } from "@/lib/permissions";

import { CommuneDetailBody } from "./commune-detail";
import {
  buildMapFrameDefaultChange,
  canSave,
  CENTER_OUTSIDE_MAINLAND,
  CONFIRM_LABEL,
  DEFAULT_NOTE,
  deviationWarning,
  formFromView,
  mapFrameDefaultError,
  NOT_CONFIGURED,
  radiusHint,
  warningRadius,
  type MapFrameDefaultForm,
} from "./map-frame-default-model";
import { MapFrameDefaultFormFields, MapFrameDefaultSaveButton, MapFrameDefaultView } from "./map-frame-default-section";

/**
 * "Khung bản đồ mặc định" (ADR 0072 amendment 2, K1–K2). Views rendered to strings, the model tested
 * on its own — the vitest environment is Node, no DOM (vitest.config.mts).
 */

const TENANT = "ops.tenant.manage";
const OTHERS = ["ops.domain.manage", "ops.profile.manage", "ops.mini_app.manage", "ops.upload_policy.manage", "ops.qr.issue", "ops.petition_field.manage"];
const noop = () => {};

const HINTS = { recommended_radius_km: 10, usual_radius_km: [3, 20] as [number, number], max_radius_km: 50 };

const UNSET: MapFrameDefault = { configured: false, ...HINTS };
const SET: MapFrameDefault = {
  configured: true,
  center_lat: 21.028511,
  center_lng: 105.804817,
  radius_km: 7.5,
  bounds: [105.732, 20.961, 105.877, 21.096],
  updated_at: "2026-10-04T03:00:00Z",
  updated_by: "VH-00001",
  ...HINTS,
};

const FORM: MapFrameDefaultForm = {
  lat: "21.028511",
  lng: "105.804817",
  radius: "10",
  reason: "Đặt theo trụ sở UBND xã",
  acknowledged: false,
  serverAskedConfirmation: false,
};

const COMMUNE: CommuneDetail = {
  id: "01J0000000000000000000000A",
  name: "Xã Kiểm Thử",
  province: "Tỉnh Kiểm Thử",
  active: true,
  domains: ["chinh.example.vn"],
  mini_apps: [],
};

const err = (status: number, code: string) => new ApiError(status, code, "", "");

describe("view: configured and unconfigured", () => {
  it("configured: centre at 6 decimals, radius, updated at/by, the default note", () => {
    const html = renderToString(<MapFrameDefaultView state={{ status: "ready", view: SET }} canManage={false} onEdit={noop} />);
    expect(html).toContain("21.028511");
    expect(html).toContain("105.804817");
    expect(html).toContain("7,5 km");
    expect(html).toContain("VH-00001");
    // 03:00Z is 10:00 in the business time zone.
    expect(html).toContain("10:00");
    expect(html).toContain(DEFAULT_NOTE);
    expect(html).not.toContain(NOT_CONFIGURED);
  });

  it("unconfigured: the 'Chưa đặt' sentence and the note", () => {
    const html = renderToString(<MapFrameDefaultView state={{ status: "ready", view: UNSET }} canManage={false} onEdit={noop} />);
    expect(html).toContain(NOT_CONFIGURED);
    expect(html).toContain(DEFAULT_NOTE);
  });

  it("an error renders the sentence, not the code", () => {
    const html = renderToString(<MapFrameDefaultView state={{ status: "error", message: AUTH_UNAVAILABLE }} canManage onEdit={noop} />);
    expect(html).toContain(AUTH_UNAVAILABLE);
    expect(html).not.toContain("Đặt khung mặc định");
  });
});

describe("denied case first: the edit control follows ops.tenant.manage only", () => {
  it("no other key shows the control", () => {
    for (const k of OTHERS) expect(canManageCommune([k])).toBe(false);
    expect(canManageCommune([])).toBe(false);
    const denied = renderToString(<MapFrameDefaultView state={{ status: "ready", view: SET }} canManage={canManageCommune(OTHERS)} onEdit={noop} />);
    expect(denied).not.toContain("Sửa khung mặc định");
    expect(denied).not.toContain("Đặt khung mặc định");
  });

  it("with the key: Đặt when unset, Sửa when set", () => {
    expect(canManageCommune([TENANT])).toBe(true);
    expect(renderToString(<MapFrameDefaultView state={{ status: "ready", view: UNSET }} canManage onEdit={noop} />)).toContain("Đặt khung mặc định");
    expect(renderToString(<MapFrameDefaultView state={{ status: "ready", view: SET }} canManage onEdit={noop} />)).toContain("Sửa khung mặc định");
  });

  it("the section is on the commune page for any key (a read), with no style attribute", () => {
    for (const keys of [OTHERS.slice(0, 1), [TENANT]]) {
      const html = renderToString(<CommuneDetailBody commune={COMMUNE} permissionKeys={keys} onChanged={noop} onMiniAppAttached={noop} />);
      expect(html).toContain("Khung bản đồ mặc định (Bản đồ kinh tế số)");
      expect(html).not.toContain("style=");
    }
  });
});

describe("form: hint, warning band, Lưu", () => {
  const fields = (form: MapFrameDefaultForm) =>
    renderToString(<MapFrameDefaultFormFields hints={HINTS} form={form} onChange={noop} busy={false} error={null} />);
  const save = (form: MapFrameDefaultForm) => renderToString(<MapFrameDefaultSaveButton hints={HINTS} form={form} busy={false} />);

  it("the hint is built from the server's numbers", () => {
    expect(radiusHint(HINTS)).toBe("Đề xuất: 10 km (thường 3–20 km); tối đa 50 km.");
    expect(fields(FORM)).toContain("Đề xuất: 10 km (thường 3–20 km); tối đa 50 km.");
    expect(radiusHint({ recommended_radius_km: 12, usual_radius_km: [4, 25], max_radius_km: 50 })).toContain("thường 4–25 km");
  });

  it.each([
    ["2.9", true],
    ["20.1", true],
    ["3", false],
    ["20", false],
    ["10", false],
  ])("radius %s → warning %s", (radius, shown) => {
    const form = { ...FORM, radius };
    expect(warningRadius(form, HINTS) !== null).toBe(shown);
    const html = fields(form);
    expect(html.includes(CONFIRM_LABEL)).toBe(shown);
    expect(html.includes("lệch nhiều so với đề xuất")).toBe(shown);
  });

  it("the warning sentence, with the Vietnamese decimal comma", () => {
    expect(deviationWarning(2.9, HINTS)).toBe(
      "Bán kính 2,9 km lệch nhiều so với đề xuất (10 km, thường 3–20 km). Khung quá nhỏ có thể không bao hết địa bàn xã; khung quá lớn có thể trùm địa bàn ngoài xã. Người đặt chịu trách nhiệm về giá trị này.",
    );
  });

  it("Lưu is disabled until the warning box is ticked; a usual radius needs no tick", () => {
    expect(canSave({ ...FORM, radius: "25" }, HINTS)).toBe(false);
    expect(save({ ...FORM, radius: "25" })).toMatch(/<button[^>]* disabled=""/);
    expect(canSave({ ...FORM, radius: "25", acknowledged: true }, HINTS)).toBe(true);
    expect(save({ ...FORM, radius: "25", acknowledged: true })).not.toMatch(/<button[^>]* disabled=""/);
    expect(canSave(FORM, HINTS)).toBe(true);
    expect(save(FORM)).not.toMatch(/<button[^>]* disabled=""/);
  });

  it("the server asking for confirmation shows the warning even for a radius this copy calls usual", () => {
    const form = { ...FORM, radius: "15", serverAskedConfirmation: true };
    expect(warningRadius(form, HINTS)).toBe(15);
    expect(canSave(form, HINTS)).toBe(false);
    expect(fields(form)).toContain(CONFIRM_LABEL);
  });

  it("prefills from a configured view, empty when unset; never a ticked box", () => {
    expect(formFromView(SET)).toMatchObject({ lat: "21.028511", lng: "105.804817", radius: "7.5", reason: "", acknowledged: false });
    expect(formFromView(UNSET)).toMatchObject({ lat: "", lng: "", radius: "" });
  });
});

describe("PUT body and client-side refusals", () => {
  it("a usual radius: no acknowledged_unusual key at all, the reason sent", () => {
    const r = buildMapFrameDefaultChange(FORM, HINTS);
    expect(r).toEqual({
      ok: true,
      body: { center_lat: 21.028511, center_lng: 105.804817, radius_km: 10, reason: "Đặt theo trụ sở UBND xã" },
    });
  });

  it("an unusual radius, confirmed: acknowledged_unusual true; unconfirmed: refused", () => {
    const ok = buildMapFrameDefaultChange({ ...FORM, radius: "2,9", acknowledged: true }, HINTS);
    expect(ok.ok && ok.body).toEqual({
      center_lat: 21.028511,
      center_lng: 105.804817,
      radius_km: 2.9,
      acknowledged_unusual: true,
      reason: "Đặt theo trụ sở UBND xã",
    });
    const no = buildMapFrameDefaultChange({ ...FORM, radius: "20.1" }, HINTS);
    expect(no.ok ? null : no.field).toBe("acknowledge");
  });

  it("a stale tick on a usual radius sends nothing extra", () => {
    const r = buildMapFrameDefaultChange({ ...FORM, acknowledged: true }, HINTS);
    expect(r.ok && Object.keys(r.body)).not.toContain("acknowledged_unusual");
  });

  it.each(["0", "50.1", "51", "-1", "0.05", "abc", ""])("radius %s is refused before sending", (radius) => {
    const r = buildMapFrameDefaultChange({ ...FORM, radius, acknowledged: true }, HINTS);
    expect(r.ok ? null : r.field).toBe("radius");
  });

  it("50 and 0.1 are allowed (with confirmation)", () => {
    expect(buildMapFrameDefaultChange({ ...FORM, radius: "50", acknowledged: true }, HINTS).ok).toBe(true);
    expect(buildMapFrameDefaultChange({ ...FORM, radius: "0.1", acknowledged: true }, HINTS).ok).toBe(true);
  });

  it("a centre outside the mainland box is refused with the server's sentence", () => {
    for (const lat of ["8.3", "23.5"]) {
      const r = buildMapFrameDefaultChange({ ...FORM, lat }, HINTS);
      expect(r).toEqual({ ok: false, field: "lat", text: CENTER_OUTSIDE_MAINLAND });
    }
    for (const lng of ["102", "111.2"]) {
      const r = buildMapFrameDefaultChange({ ...FORM, lng }, HINTS);
      expect(r).toEqual({ ok: false, field: "lng", text: CENTER_OUTSIDE_MAINLAND });
    }
    expect(buildMapFrameDefaultChange({ ...FORM, lat: "8.4", lng: "109.5" }, HINTS).ok).toBe(true);
  });

  it("more than 6 decimals, or a blank reason, is refused", () => {
    const r = buildMapFrameDefaultChange({ ...FORM, lat: "21.0285111" }, HINTS);
    expect(r.ok ? null : r.field).toBe("lat");
    const reason = buildMapFrameDefaultChange({ ...FORM, reason: "  " }, HINTS);
    expect(reason.ok ? null : reason.field).toBe("reason");
  });
});

describe("server refusals: one Vietnamese sentence each", () => {
  it.each([
    [409, "commune_inactive", "form"],
    [422, "center_outside_mainland", "form"],
    [422, "radius_out_of_range", "radius"],
    [422, "radius_unusual_unconfirmed", "acknowledge"],
    [422, "invalid_reason", "reason"],
    [404, "commune_not_found", "form"],
  ])("%s %s", (status, code, field) => {
    const known = mapFrameDefaultError(err(status, code));
    expect(known?.field).toBe(field);
    const text = handleGuardedError(err(status, code), "đặt khung bản đồ mặc định", vi.fn(), (e) => mapFrameDefaultError(e)?.text ?? null);
    expect(text).toBe(known?.text);
    expect(text).not.toContain(code);
  });

  it("the confirmation refusal names the box to tick", () => {
    expect(mapFrameDefaultError(err(422, "radius_unusual_unconfirmed"))?.text).toContain(CONFIRM_LABEL);
  });

  it("400 invalid_body, 503, 403 and 401 go through the shared handling", () => {
    const specific = (e: ApiError) => mapFrameDefaultError(e)?.text ?? null;
    expect(handleGuardedError(err(400, "invalid_body"), "x", vi.fn(), specific)).toBe(INVALID_BODY);
    expect(handleGuardedError(err(503, "operator_auth_unavailable"), "x", vi.fn(), specific)).toBe(AUTH_UNAVAILABLE);
    expect(handleGuardedError(err(403, "forbidden"), "đặt khung bản đồ mặc định", vi.fn(), specific)).toContain(
      "không có quyền đặt khung bản đồ mặc định",
    );
    const navigate = vi.fn();
    expect(handleGuardedError(err(401, "unauthenticated"), "x", navigate, specific)).toBeNull();
    expect(navigate).toHaveBeenCalled();
  });
});
