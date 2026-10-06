"use client";

import { useEffect, useState } from "react";

import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing org-unit catalogue client
import type { KetQua } from "@/lib/api/goi";
import type {
  finance_duAnRa,
  identity_boPhanRa,
  identity_canBoChonNguoiRa,
  identity_danhBaChonNguoiRa,
  identity_danhSachBoPhanRa,
} from "@/lib/api/schema.gen";

/**
 * `Đơn vị thực hiện` and `Cán bộ phụ trách` of a project (spec §7.2, §8, §9): the two references
 * `finance` stores (`org_unit_id`, `assignee_id`) turned into names, and the options of the two §9 selects.
 *
 * BOTH CATALOGUES ARE `service-identity`'s, READ THROUGH ITS TWO `AnyAuthenticated` ROUTES — the same two
 * every assignment picker uses (`GET /api/v1/org-units`, `GET /api/v1/staff-directory`). Neither needs
 * an admin key, so a `budget.read` account resolves names with no new permission. Both return the WHOLE
 * commune or refuse (no pagination), which is what makes "not in the map" mean "not in the commune".
 *
 * WHAT `assignee_id` HOLDS IS THE STAFF BUSINESS CODE (`CB-00123`), not an internal id: the staff
 * directory returns no internal id on purpose, and the code is what every other assignment in the
 * system stores (`can_bo_xu_ly_id` in petitions, rule 6 invariant 8). `finance` checks neither value
 * exists (`ThamChieuToiDa`, `service-finance/internal/domain/du_an_ghi.go`) — so an unknown reference
 * is drawn as "not assigned", never as the raw string.
 *
 * ONE READ PER CATALOGUE PER SCREEN, mapped by key — never one per row.
 */

/** One catalogue as a screen holds it: `names` maps the stored reference to the name shown. */
export type PeopleCatalogue<T> =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly items: readonly T[]; readonly names: ReadonlyMap<string, string> };

export type ProjectPeople = {
  readonly units: PeopleCatalogue<identity_boPhanRa>;
  readonly staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
};

/** What a screen holds before either read has answered. */
export const PEOPLE_LOADING: ProjectPeople = { units: { phase: "loading" }, staff: { phase: "loading" } };

/** §7.2 / §8 words for a project with nobody assigned, and the §9 select placeholders. */
export const UNASSIGNED = "Chưa phân công";
export const UNIT_PLACEHOLDER = "— Chưa xác định —";
export const OFFICER_PLACEHOLDER = "— Chưa phân công —";
/** Option for a saved reference the catalogue no longer lists, so the select never shows "unset" over it. */
export const UNIT_NOT_LISTED = "Bộ phận không còn trong sơ đồ tổ chức";
export const OFFICER_NOT_LISTED = "Cán bộ không còn trong danh bạ";

/** The org-unit catalogue, mapped `id` → `name` (the ULID other records reference). */
export function unitsCatalogue(result: KetQua<identity_danhSachBoPhanRa>): PeopleCatalogue<identity_boPhanRa> {
  if (!result.ok) return { phase: "error", message: result.thongBao };
  const items = result.duLieu.items;
  return { phase: "ready", items, names: new Map(items.map((u) => [u.id, u.name])) };
}

/** The staff directory, mapped `code` → `full_name` (see the file comment for why the code). */
export function staffCatalogue(
  result: KetQua<identity_danhBaChonNguoiRa>,
): PeopleCatalogue<identity_canBoChonNguoiRa> {
  if (!result.ok) return { phase: "error", message: result.thongBao };
  const items = result.duLieu.items;
  return { phase: "ready", items, names: new Map(items.map((s) => [s.code, s.full_name])) };
}

/** Reads both catalogues once for the screen that mounts it. A failed read is its own phase, not a page error. */
export function useProjectPeople(): ProjectPeople {
  const [units, setUnits] = useState<ProjectPeople["units"]>({ phase: "loading" });
  const [staff, setStaff] = useState<ProjectPeople["staff"]>({ phase: "loading" });
  useEffect(() => {
    let dropped = false;
    layDanhMucBoPhan().then((r) => {
      if (!dropped) setUnits(unitsCatalogue(r));
    });
    layDanhBaChonNguoi().then((r) => {
      if (!dropped) setStaff(staffCatalogue(r));
    });
    return () => {
      dropped = true;
    };
  }, []);
  return { units, staff };
}

/**
 * One reference resolved: `none` = nothing stored; `name`; `unknown` = stored but not in the commune's
 * catalogue; `unavailable` = stored, but the catalogue is still loading or failed — we do not know.
 */
type Resolved =
  | { readonly kind: "none" }
  | { readonly kind: "name"; readonly name: string }
  | { readonly kind: "unknown" }
  | { readonly kind: "unavailable" };

function resolve<T>(ref: string | undefined, catalogue: PeopleCatalogue<T>): Resolved {
  const key = (ref ?? "").trim();
  if (key === "") return { kind: "none" };
  if (catalogue.phase !== "ready") return { kind: "unavailable" };
  const name = catalogue.names.get(key);
  return name === undefined ? { kind: "unknown" } : { kind: "name", name };
}

/**
 * §7.2 list cell `Đơn vị / phụ trách`: the unit first, the officer when no unit names — prototype
 * `BudgetItemTable.tsx:299-304` ("ở cấp xã một công trình thuộc về một đơn vị, còn cán bộ theo dõi thì
 * đổi"). "—" when a stored reference cannot be resolved because a catalogue did not load: saying
 * "Chưa phân công" then would state something nobody checked.
 */
export function unitOwnerLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const unit = resolve(project.org_unit_id, people.units);
  if (unit.kind === "name") return unit.name;
  const officer = resolve(project.assignee_id, people.staff);
  if (officer.kind === "name") return officer.name;
  if (unit.kind === "unavailable" || officer.kind === "unavailable") return "—";
  return UNASSIGNED;
}

/** §8 header line `phụ trách …` (prototype `BudgetItemDetail.tsx:261`). */
export function officerLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const officer = resolve(project.assignee_id, people.staff);
  if (officer.kind === "name") return officer.name;
  return officer.kind === "unavailable" ? "—" : UNASSIGNED;
}

/** §8 figure `ĐƠN VỊ THỰC HIỆN`: the unit's name, or "—" (spec §8 draws "—" for an unset unit). */
export function unitLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const unit = resolve(project.org_unit_id, people.units);
  return unit.kind === "name" ? unit.name : "—";
}
