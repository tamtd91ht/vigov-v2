"use client";

import { useEffect, useState } from "react";

import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing org-unit catalogue client
import { listImplementingUnits } from "@/lib/api/du-an";
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
 *
 * A THIRD SOURCE, NO LOOKUP: `implementing_unit` (65afbdcd) is free text typed for a unit outside the
 * org chart (a contractor). It IS the name, so it is shown as stored and needs no catalogue.
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

/** The year's typed implementing units (`GET /api/v1/implementing-units`) as a form holds them. */
export type KnownUnits =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly items: readonly string[] };

/** What a form holds before the typed-unit read has answered. */
export const KNOWN_UNITS_LOADING: KnownUnits = { phase: "loading" };

/**
 * Reads the typed units of `year` once per year. Kept with the year that produced it, as the funding
 * catalogue is. A failure is not shown as an error: the select still offers the org units and manual
 * entry, so it only means fewer suggestions.
 */
export function useImplementingUnits(year: number): KnownUnits {
  const [loaded, setLoaded] = useState<{ year: number; units: KnownUnits } | null>(null);
  useEffect(() => {
    let dropped = false;
    listImplementingUnits(year).then((r) => {
      if (dropped) return;
      setLoaded({
        year,
        units: r.ok ? { phase: "ready", items: r.duLieu.items } : { phase: "error", message: r.thongBao },
      });
    });
    return () => {
      dropped = true;
    };
  }, [year]);
  return loaded !== null && loaded.year === year ? loaded.units : { phase: "loading" };
}

/** Option values of the §9 `Đơn vị thực hiện` select. */
export const UNIT_ORG_PREFIX = "org:";
export const UNIT_TEXT_PREFIX = "text:";
export const UNIT_MANUAL = "manual";

/**
 * The §9 `Đơn vị thực hiện` options (prototype `BudgetItemForm.tsx:99-113`): the commune's org units
 * first, then the typed units already used this year, NO NAME TWICE — a typed unit spelled exactly as
 * an org unit is dropped, so picking that name stores the org unit. Org units by id, typed units by
 * their text. The manual-entry option is the select's own, not listed here.
 */
export function implementingUnitOptions(
  units: readonly identity_boPhanRa[],
  known: KnownUnits,
): { value: string; label: string }[] {
  const seen = new Set<string>();
  const out: { value: string; label: string }[] = [];
  for (const u of units) {
    const name = u.name.trim();
    if (name === "" || seen.has(name)) continue;
    seen.add(name);
    out.push({ value: UNIT_ORG_PREFIX + u.id, label: name });
  }
  if (known.phase === "ready") {
    for (const t of known.items) {
      const name = t.trim();
      if (name === "" || seen.has(name)) continue;
      seen.add(name);
      out.push({ value: UNIT_TEXT_PREFIX + name, label: name });
    }
  }
  return out;
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

/** The typed `implementing_unit` (65afbdcd), trimmed; "" when none. The server stores blank as NULL. */
function typedUnit(project: finance_duAnRa): string {
  return (project.implementing_unit ?? "").trim();
}

/**
 * §7.2 list cell `Đơn vị / phụ trách` (spec 02 §8): typed `implementing_unit` ?? org-unit name ??
 * officer name ?? "Chưa phân công" — prototype `BudgetItemTable.tsx:299-304` ("ở cấp xã một công trình
 * thuộc về một đơn vị, còn cán bộ theo dõi thì đổi"). The typed unit comes first because it needs no
 * catalogue: it is the name. "—" when a stored reference cannot be resolved because a catalogue did not
 * load: saying "Chưa phân công" then would state something nobody checked.
 */
export function unitOwnerLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const typed = typedUnit(project);
  if (typed !== "") return typed;
  const unit = resolve(project.org_unit_id, people.units);
  if (unit.kind === "name") return unit.name;
  const officer = resolve(project.assignee_id, people.staff);
  if (officer.kind === "name") return officer.name;
  if (unit.kind === "unavailable" || officer.kind === "unavailable") return "—";
  return UNASSIGNED;
}

/** One option of the list's `Tất cả đơn vị phụ trách` filter. `key` is `org:<id>` or `text:<unit>`. */
export type UnitFilterOption = { readonly key: string; readonly name: string };

/**
 * The unit filter's options: the distinct org units AND typed units the LOADED rows name (prototype
 * `BudgetWorkspace.tsx:297-319` lists what the data holds), sorted by name. Two key spaces, because an
 * org unit is matched by its id and a typed unit by its exact text — the server's `?implementing_unit=`
 * is exact too. While the org-unit catalogue is not ready its options say "—" rather than an id.
 */
export function unitFilterOptions(items: readonly finance_duAnRa[], people: ProjectPeople): UnitFilterOption[] {
  const ids = [...new Set(items.map((d) => (d.org_unit_id ?? "").trim()).filter((id) => id !== ""))];
  const texts = [...new Set(items.map(typedUnit).filter((t) => t !== ""))];
  const nameOf = (id: string): string =>
    people.units.phase === "ready" ? (people.units.names.get(id) ?? UNIT_NOT_LISTED) : "—";
  return [
    ...ids.map((id) => ({ key: UNIT_ORG_PREFIX + id, name: nameOf(id) })),
    ...texts.map((t) => ({ key: UNIT_TEXT_PREFIX + t, name: t })),
  ].sort((a, b) => a.name.localeCompare(b.name, "vi"));
}

/** Rows a `unitFilterOptions` key selects; "" = every row. An unknown key selects nothing (fail closed). */
export function matchUnitFilter(items: readonly finance_duAnRa[], key: string): finance_duAnRa[] {
  if (key === "") return [...items];
  if (key.startsWith(UNIT_ORG_PREFIX)) {
    const id = key.slice(UNIT_ORG_PREFIX.length);
    return items.filter((d) => (d.org_unit_id ?? "").trim() === id);
  }
  if (key.startsWith(UNIT_TEXT_PREFIX)) {
    const text = key.slice(UNIT_TEXT_PREFIX.length);
    return items.filter((d) => typedUnit(d) === text);
  }
  return [];
}

/** §8 header line `phụ trách …` (prototype `BudgetItemDetail.tsx:261`). */
export function officerLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const officer = resolve(project.assignee_id, people.staff);
  if (officer.kind === "name") return officer.name;
  return officer.kind === "unavailable" ? "—" : UNASSIGNED;
}

/**
 * §8 figure `ĐƠN VỊ THỰC HIỆN`: the typed unit, else the org unit's name, or "—" (spec §8 draws "—" for
 * an unset unit).
 */
export function unitLabel(project: finance_duAnRa, people: ProjectPeople): string {
  const typed = typedUnit(project);
  if (typed !== "") return typed;
  const unit = resolve(project.org_unit_id, people.units);
  return unit.kind === "name" ? unit.name : "—";
}
