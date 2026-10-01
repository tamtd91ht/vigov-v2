import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

/**
 * THE SHARED APP'S FIELD STEP AS MARKUP, AND THE TABLES BEHIND IT (owner, 01/10/2026: the field is required
 * in the shared app, step 1, read again on the confirmation step). The mounted, pressed flow — disabled
 * "Tiếp tục", the code in the POST body, `field_not_offered` back to step 1 — is in `hieu-ung-khong-phien.test.tsx`,
 * the only file with a DOM that can deliver a click.
 */
import { thanGuiPhanAnh } from "../api/hop-dong-phan-anh";

import { readCatalogueAnswer } from "./field-catalogue";
import { buocSauKhiGui, FieldPickStep, PHAN_ANH_TRONG, sharedCatalogueFailureText } from "./GuiPhanAnhScreen";
import { CUA_TOI, GUI, LOI_GUI, XA_PA } from "./noi-dung";
import { catalogueOutcome } from "./PhanAnhAppXa";

const ITEM = { code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", icon: "Trash2", tone: "orange" };

const step = (catalogue: Parameters<typeof FieldPickStep>[0]["catalogue"], picked = "", fieldChanged = false) =>
  renderToStaticMarkup(
    createElement(FieldPickStep, { catalogue, picked, fieldChanged, onPick: () => {}, onNext: () => {}, onRetry: () => {} }),
  );

describe("step 1 of the shared app — words for every state, never a built-in list", () => {
  it("ready: the commune's labels in its order, as radios in a radiogroup; the code is never shown", () => {
    const html = step({ kind: "ready", fields: [ITEM, { code: "an-ninh", label: "An ninh trật tự", icon: null, tone: null }] });
    expect(html).toContain('role="radiogroup"');
    expect(html.match(/role="radio"/g)).toHaveLength(2);
    expect(html.indexOf(ITEM.label)).toBeLessThan(html.indexOf("An ninh trật tự"));
    expect(html).not.toContain("rac-thai");
    expect(html).toContain(GUI.field_prompt);
    // The shared look, not the commune's red branding.
    expect(html).not.toMatch(/xa-o-lv|xa-mau--/);
    expect(html).not.toMatch(/<(input|select|form|textarea)[\s/>]/);
  });

  it("nothing picked: 'Tiếp tục' is disabled and the sentence says why; picked: enabled, 'Đã chọn' in words", () => {
    const none = step({ kind: "ready", fields: [ITEM] });
    expect(none).toContain(`<button type="button" class="cd-nut" disabled="">${GUI.nut_tiep}</button>`);
    expect(none).toContain(GUI.field_pick_first);
    const picked = step({ kind: "ready", fields: [ITEM] }, "rac-thai");
    expect(picked).toContain(`<button type="button" class="cd-nut">${GUI.nut_tiep}</button>`);
    expect(picked).toContain('aria-checked="true"');
    expect(picked).toContain(GUI.field_picked);
    expect(picked).not.toContain(GUI.field_pick_first);
    // A code the catalogue does not offer cannot be gone on with.
    expect(step({ kind: "ready", fields: [ITEM] }, "khong-con")).toContain('disabled=""');
  });

  it("empty catalogue: one sentence with the next step — no tile, no 'Tiếp tục', no 'Thử lại'", () => {
    const html = step({ kind: "ready", fields: [] });
    expect(html).toContain(GUI.field_empty);
    expect(html).not.toContain('role="radio"');
    expect(html).not.toContain(GUI.nut_tiep);
    expect(html).not.toContain(CUA_TOI.nut_thu_lai);
  });

  it("loading and every failure: words, no tile; 'Thử lại' except after a 401, which says to reopen the app", () => {
    expect(step({ kind: "loading" })).toContain(GUI.field_loading);
    expect(step({ kind: "loading" })).not.toContain('role="radio"');
    for (const failure of ["unavailable", "network", "server"] as const) {
      const html = step({ kind: "failed", failure });
      expect(html, failure).toContain(sharedCatalogueFailureText(failure));
      expect(html, failure).toContain(CUA_TOI.nut_thu_lai);
      expect(html, failure).not.toContain('role="radio"');
      expect(html, failure).not.toContain(GUI.nut_tiep);
    }
    const expired = step({ kind: "failed", failure: "expired" });
    expect(expired).toContain(GUI.field_expired);
    expect(expired).not.toContain(CUA_TOI.nut_thu_lai);
    expect(sharedCatalogueFailureText("unavailable")).toBe(GUI.field_unavailable);
  });

  it("the shared app speaks 'bạn', not the commune app's 'bà con'", () => {
    const words = [
      GUI.field_prompt,
      GUI.field_pick_first,
      GUI.field_unavailable,
      GUI.field_network,
      GUI.field_server,
      GUI.field_expired,
      GUI.field_empty,
      GUI.field_missing,
    ];
    for (const w of words) expect(w).not.toMatch(/bà con/i);
    expect(GUI.field_unavailable).not.toBe(XA_PA.fields_unavailable);
  });

  it("after field_not_offered: the reason stands above the list", () => {
    const html = step({ kind: "ready", fields: [ITEM] }, "", true);
    expect(html).toContain(LOI_GUI["field-not-offered"].cau);
    expect(html.indexOf(LOI_GUI["field-not-offered"].cau)).toBeLessThan(html.indexOf(ITEM.label));
  });
});

describe("the tables", () => {
  it("send: field_not_offered goes back to step 1 (no longer the dead-end error step)", () => {
    expect(buocSauKhiGui({ kieu: "field-not-offered" })).toEqual({ kieu: "field", changed: true });
    expect(buocSauKhiGui({ kieu: "field-catalogue-unavailable" })).toEqual({
      kieu: "loi",
      nhanh: "field-catalogue-unavailable",
    });
  });

  it("one reading of a catalogue answer for both apps; the commune app's outcomes are unchanged by it", () => {
    expect(readCatalogueAnswer({ kieu: "xong", fields: [ITEM] })).toEqual({ kind: "ready", fields: [ITEM] });
    expect(readCatalogueAnswer({ kieu: "chua-co-phien" })).toEqual({ kind: "no-session" });
    expect(readCatalogueAnswer({ kieu: "chua-cau-hinh" })).toEqual({ kind: "not-configured" });
    expect(readCatalogueAnswer({ kieu: "kenh-chua-mo" })).toEqual({ kind: "intake-closed" });
    expect(readCatalogueAnswer({ kieu: "het-phien" })).toEqual({ kind: "expired" });
    expect(readCatalogueAnswer({ kieu: "can-xac-thuc-so" })).toEqual({ kind: "phone-required" });
    expect(readCatalogueAnswer({ kieu: "field-catalogue-unavailable" })).toEqual({ kind: "failed", failure: "unavailable" });
    expect(readCatalogueAnswer({ kieu: "loi-mang" })).toEqual({ kind: "failed", failure: "network" });
    expect(readCatalogueAnswer({ kieu: "khong-hop-le" })).toEqual({ kind: "failed", failure: "server" });
    // The commune app: 503-other stayed "server", no address stayed "closed", session-shaped → its gate.
    expect(catalogueOutcome({ kieu: "kenh-chua-mo" })).toEqual({ kind: "failed", failure: "server" });
    expect(catalogueOutcome({ kieu: "chua-cau-hinh" })).toEqual({ kind: "failed", failure: "closed" });
    for (const kieu of ["chua-co-phien", "het-phien", "can-xac-thuc-so"] as const) {
      expect(catalogueOutcome({ kieu })).toBe("session");
    }
  });

  it("a picked code is the one extra key of the body; no pick → no `field` key at all", () => {
    const withField = JSON.parse(thanGuiPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "x", field: "rac-thai" })) as Record<
      string,
      unknown
    >;
    expect(withField["field"]).toBe("rac-thai");
    expect(JSON.parse(thanGuiPhanAnh({ ...PHAN_ANH_TRONG, noi_dung: "x" }))).not.toHaveProperty("field");
  });
});
