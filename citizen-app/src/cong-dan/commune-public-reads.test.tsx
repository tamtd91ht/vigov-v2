import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  type CommuneProfile,
  docDanhBa,
  docTrangTinXa,
  readCommuneProfiles,
} from "./api/hop-dong-cong-khai";
import { COMMUNE_OFFICE, NEWS_TYPE_LABEL } from "./man/noi-dung";
import { CommuneOffice, officeRows } from "./man/TrangXa";

/**
 * W6c — the commune app's three public reads, as the server now sends them (29/09/2026): the commune's
 * declared office (identity cdbf276), the directory's order and village heads (556c6e9), news types
 * (comms b22bf76). Every added field is OPTIONAL on the wire: absent is an older server, never malformed.
 */

const STAFF = { full_name: "Lê Văn Bình", position: "", department_name: "", phone: "", mobile: "", has_zalo: false };

describe("/commune-profiles — read field by field", () => {
  const P = { name: "Xã Thử Nghiệm", office_address: "Số 1 đường A", hotline: "0900000000", office_hours_text: "Thứ Hai–Thứ Sáu, 7:30–17:00" };

  it("one profile; an empty list; malformed on a missing or wrong-typed field", () => {
    expect(readCommuneProfiles({ items: [P] })).toEqual([P]);
    expect(readCommuneProfiles({ items: [] })).toEqual([]);
    expect(readCommuneProfiles({ items: [{ ...P, hotline: undefined }] })).toBeNull();
    expect(readCommuneProfiles({ items: [{ ...P, office_hours_text: 7 }] })).toBeNull();
    expect(readCommuneProfiles({})).toBeNull();
  });

  it("no logo is read — the route has none by design", () => {
    const got = readCommuneProfiles({ items: [{ ...P, logo_url: "https://x.example/logo.png" }] })!;
    expect(got[0]).not.toHaveProperty("logo_url");
  });
});

describe("the office block — only what the commune declared, nothing in its place", () => {
  const full: CommuneProfile = {
    name: "Xã Thử Nghiệm",
    office_address: "Số 1 đường A",
    hotline: "0900.000 000",
    office_hours_text: "Thứ Hai–Thứ Sáu",
  };

  it("rows in reading order; blank fields dropped", () => {
    expect(officeRows(full).map((r) => r.label)).toEqual([COMMUNE_OFFICE.address, COMMUNE_OFFICE.hours, COMMUNE_OFFICE.hotline]);
    expect(officeRows({ ...full, office_address: " ", hotline: "" }).map((r) => r.label)).toEqual([COMMUNE_OFFICE.hours]);
  });

  it("the hotline is a tel: button in words; the hours are shown verbatim, never parsed", () => {
    const html = renderToStaticMarkup(createElement(CommuneOffice, { profile: full }));
    expect(html).toContain('href="tel:0900000000"');
    expect(html).toContain(COMMUNE_OFFICE.call_hotline("0900.000 000"));
    expect(html).toContain("Thứ Hai–Thứ Sáu");
  });

  it("no profile, or nothing declared → no block at all (never a default)", () => {
    expect(renderToStaticMarkup(createElement(CommuneOffice, { profile: null }))).toBe("");
    const empty = { ...full, office_address: "", hotline: "", office_hours_text: "" };
    expect(renderToStaticMarkup(createElement(CommuneOffice, { profile: empty }))).toBe("");
  });
});

describe("/commune-staff — display_order and residential_units_headed", () => {
  it("absent → null and []; present → read", () => {
    expect(docDanhBa({ items: [STAFF] })![0]).toMatchObject({ display_order: null, residential_units_headed: [] });
    const got = docDanhBa({ items: [{ ...STAFF, display_order: 3, residential_units_headed: ["Thôn Hà Lam", " "] }] })!;
    // Blank unit names are dropped: a "Trưởng thôn" line with no name says nothing.
    expect(got[0]).toMatchObject({ display_order: 3, residential_units_headed: ["Thôn Hà Lam"] });
  });

  it("present but wrong-typed is malformed — the whole list, not a silent drop", () => {
    expect(docDanhBa({ items: [{ ...STAFF, display_order: "1" }] })).toBeNull();
    expect(docDanhBa({ items: [{ ...STAFF, display_order: 1.5 }] })).toBeNull();
    expect(docDanhBa({ items: [{ ...STAFF, residential_units_headed: "Thôn A" }] })).toBeNull();
    expect(docDanhBa({ items: [{ ...STAFF, residential_units_headed: [1] }] })).toBeNull();
  });
});

describe("/commune-news — `type`", () => {
  const ITEM = { id: "t1", title: "T", summary: "", published_on: "2026-09-29", category_name: "Y tế" };
  const page = (item: Record<string, unknown>) => docTrangTinXa({ items: [item], next_cursor: "", has_more: false });

  it("a listed type is read; absent, banner or an unknown code is null — never guessed from the category", () => {
    expect(page({ ...ITEM, type: "su-kien" })!.muc[0]!.type).toBe("su-kien");
    expect(page(ITEM)!.muc[0]!.type).toBeNull();
    expect(page({ ...ITEM, type: "banner" })!.muc[0]!.type).toBeNull();
    expect(page({ ...ITEM, type: "podcast", category_name: "Sự kiện" })!.muc[0]!.type).toBeNull();
  });

  it("`type` present but not a string is malformed", () => {
    expect(page({ ...ITEM, type: 3 })).toBeNull();
  });

  it("every listed type has the staff register's label", () => {
    expect(NEWS_TYPE_LABEL["su-kien"]).toBe("Sự kiện");
    expect(Object.keys(NEWS_TYPE_LABEL)).toEqual(["tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video"]);
  });
});
