import { describe, expect, it } from "vitest";

import { BAN_TRONG, banTuCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  FULL_NAME_REQUIRED,
  UNIT_REQUIRED,
  firstContactError,
  publicationStep,
  validateContact,
  zaloAfterCreate,
} from "./staff-contact";

/** Fake numbers only (rule 3, invariant 5). */
function staff(over: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Chuyên viên",
    department_id: "BP-LE",
    role_id: "",
    phone: "",
    mobile: "0900000001",
    has_account: false,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...over,
  };
}

describe("the two required fields of the prototype's form", () => {
  it("blank name and no unit → one message under each, the name's first for the toast", () => {
    const e = validateContact({ ...BAN_TRONG, hoTen: "   " });
    expect(e).toEqual({ fullName: FULL_NAME_REQUIRED, unit: UNIT_REQUIRED });
    expect(firstContactError(e)).toBe(FULL_NAME_REQUIRED);
  });

  it("a name and a unit → valid; nothing else is required", () => {
    const e = validateContact({ ...BAN_TRONG, hoTen: "Trần Thị B", boPhanID: "BP-LE" });
    expect(e).toEqual({});
    expect(firstContactError(e)).toBeNull();
  });
});

describe("'Gọi được qua Zalo' while ADDING — the create route has no such field", () => {
  it("ticked → a PATCH body carrying ONLY has_zalo (every other field null = unchanged)", () => {
    expect(zaloAfterCreate({ ...BAN_TRONG, coZalo: true })).toEqual({
      full_name: null,
      position: null,
      email: null,
      org_unit_id: null,
      office_phone: null,
      mobile: null,
      has_zalo: true,
    });
  });

  it("not ticked → no second call", () => {
    expect(zaloAfterCreate(BAN_TRONG)).toBeNull();
  });
});

describe("'Hiện trên danh bạ Mini App' never publishes — it opens the consent step (#12, Decree 13)", () => {
  it("ticked for a person not on the Mini App → the per-person consent box, never a publish", () => {
    const row = staff();
    expect(publicationStep(true, row, true)).toEqual({ kieu: "congKhai", canBo: row });
  });

  it("unticked for a person on the Mini App → the withdrawal confirmation", () => {
    const row = staff({ published: true });
    expect(publicationStep(false, row, true)).toEqual({ kieu: "rut", canBo: row });
  });

  it("unchanged → nothing to ask", () => {
    expect(publicationStep(true, staff({ published: true }), true)).toBeNull();
    expect(publicationStep(false, staff(), true)).toBeNull();
  });

  it("decided on the row the SERVER returned: a changed mobile withdrew the person, a box left ticked asks again", () => {
    // The edit started with the person published (box ticked); the server withdrew them on save.
    const before = staff({ published: true });
    const after = staff({ published: false, mobile: "0900000009" });
    expect(banTuCanBo(before).diDongCaNhan).not.toBe(after.mobile);
    expect(publicationStep(true, after, true)).toEqual({ kieu: "congKhai", canBo: after });
  });

  it("without `content.update` (or an unread session) → nothing, fail closed", () => {
    expect(publicationStep(true, staff(), false)).toBeNull();
    expect(publicationStep(false, staff({ published: true }), false)).toBeNull();
  });
});
