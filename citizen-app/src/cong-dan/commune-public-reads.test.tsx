import { describe, expect, it } from "vitest";

import { docDanhBa, docTrangTinXa } from "./api/hop-dong-cong-khai";
import { NEWS_TYPE_LABEL } from "./man/noi-dung";

/**
 * W6c — the commune app's public reads, as the server now sends them (29/09/2026): the directory's order and
 * village heads (556c6e9), news types (comms b22bf76). Every added field is OPTIONAL on the wire: absent is an
 * older server, never malformed. (`/commune-profiles` is no longer read: its office block and logo are gone,
 * customer sheet App Xã 12/10/2026.)
 */

const STAFF = { full_name: "Lê Văn Bình", position: "", department_name: "", phone: "", mobile: "", has_zalo: false };

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
