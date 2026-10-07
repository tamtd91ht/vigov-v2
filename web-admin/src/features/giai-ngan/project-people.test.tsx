// @vitest-environment jsdom
//
// jsdom for this file: the selects are chosen and the form submitted, and the hook's reads are counted —
// events and captured requests, not a markup string.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type {
  finance_danhSachDuAnRa,
  finance_duAnRa,
  identity_boPhanRa,
  identity_canBoChonNguoiRa,
} from "@/lib/api/schema.gen";

import { BangDanhSach } from "./bang-du-an";
import { ThongTinDuAn } from "./chi-tiet-du-an";
import { FormDuAn, giaTriTuDuAn } from "./ghi-du-an";
import { FORM_DU_AN_TRONG, PHAN_CHUA_DUNG_GHI, pendingPart, thanSuaDuAn, thanThemDuAn } from "./nhan-ghi-giai-ngan";
import {
  OFFICER_NOT_LISTED,
  staffCatalogue,
  UNASSIGNED,
  unitsCatalogue,
  useProjectPeople,
  type ProjectPeople,
} from "./project-people";

/**
 * §7.2 `Đơn vị / phụ trách`, §8 `Đơn vị thực hiện` / `phụ trách …`, §9 the two selects.
 *
 * THE EXPENSIVE FAILURES HERE ARE SILENT ONES: an internal id printed where a name belongs, a project
 * that IS assigned shown as "Chưa phân công" because a catalogue failed to load, and an edit that
 * clears an assignment nobody touched. Each has a case below.
 */

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function choose(el: HTMLSelectElement, value: string): void {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

const UNITS: identity_boPhanRa[] = [
  { id: "01JUNIT_DIACHINH", code: "dia-chinh", name: "Địa chính – Xây dựng", parent_id: "", order: 1, staff_count: 3 },
  { id: "01JUNIT_VANPHONG", code: "van-phong", name: "Văn phòng UBND", parent_id: "", order: 2, staff_count: 5 },
];
const STAFF: identity_canBoChonNguoiRa[] = [
  { code: "CB-00101", full_name: "Nguyễn Văn An", position: "Công chức địa chính", department_id: "01JUNIT_DIACHINH" },
  { code: "CB-00102", full_name: "Trần Thị Bình", position: "", department_id: "01JUNIT_VANPHONG" },
];

const READY: ProjectPeople = {
  units: unitsCatalogue({ ok: true, duLieu: { items: UNITS } }),
  staff: staffCatalogue({ ok: true, duLieu: { items: STAFF } }),
};
const DENIED_SENTENCE = "Bạn không có quyền xem mục này.";
const DENIED: ProjectPeople = {
  units: unitsCatalogue({ ok: false, thongBao: DENIED_SENTENCE }),
  staff: staffCatalogue({ ok: false, thongBao: DENIED_SENTENCE }),
};

function project(over: Partial<finance_duAnRa> = {}): finance_duAnRa {
  return {
    id: "01JDA1",
    code: "DA01",
    year: 2026,
    category_id: "01JHM1",
    name: "Bê tông hoá đường trục chính thôn Hà Lam",
    planned_amount: 100_000_000,
    approved_amount: 100_000_000,
    disbursed_amount: 0,
    remaining_amount: 100_000_000,
    disbursed_ratio: 0,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...over,
  };
}

function list(items: finance_duAnRa[]): finance_danhSachDuAnRa {
  return { items, year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" };
}

function unitOwnerCells(html: string): string[] {
  const doc = new DOMParser().parseFromString(`<table>${html}</table>`, "text/html");
  return [...doc.querySelectorAll("[data-unit-owner]")].map((td) => td.textContent ?? "");
}

describe("§9 selects — add form", () => {
  function addForm(people: ProjectPeople, save = vi.fn()) {
    return mount(
      <FormDuAn
        tieuDeForm="Thêm dự án"
        budgetYear={2026}
        giaTriDau={{ ...FORM_DU_AN_TRONG, hangMucID: "01JHM1", ten: "Dự án A", keHoachVon: "100000000" }}
        danhMuc={[{ id: "01JHM1", code: "a", label: "A", is_default: true, active: true, order: 1, source: "he-thong", tier: 1 }]}
        fundingCatalogue={{ phase: "ready", items: [] }}
        people={people}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={save}
      />,
    );
  }

  it("both selects list the commune's catalogue under the spec's placeholders", () => {
    const el = addForm(READY);
    const unit = el.querySelector<HTMLSelectElement>("#don-vi-du-an")!;
    const officer = el.querySelector<HTMLSelectElement>("#can-bo-du-an")!;
    expect(unit.disabled).toBe(false);
    expect(officer.disabled).toBe(false);
    expect([...unit.options].map((o) => o.textContent)).toEqual([
      "— Chưa xác định —",
      "Địa chính – Xây dựng",
      "Văn phòng UBND",
      // Spec 04's free-text choice, last and disabled: a backend dependency (ADR 0068 lần 6 #11).
      "— Đơn vị khác, nhập tay —",
    ]);
    expect([...unit.options].at(-1)!.disabled).toBe(true);
    expect([...officer.options].map((o) => o.textContent)).toEqual([
      "— Chưa phân công —",
      // Spec 04: the officer's name only.
      "Nguyễn Văn An",
      "Trần Thị Bình",
    ]);
    // The only "?" in the form is the free-text unit's.
    expect([...el.querySelectorAll("[data-pending]")].map((p) => p.textContent)).toEqual(["— Đơn vị khác, nhập tay —?"]);
  });

  it("the chosen unit id and staff CODE reach the POST body; unset selects send nothing", () => {
    const save = vi.fn();
    const el = addForm(READY, save);
    choose(el.querySelector<HTMLSelectElement>("#don-vi-du-an")!, "01JUNIT_DIACHINH");
    choose(el.querySelector<HTMLSelectElement>("#can-bo-du-an")!, "CB-00101");
    act(() => el.querySelector<HTMLButtonElement>('button[type="submit"]')!.click());

    expect(save).toHaveBeenCalledTimes(1);
    const body = thanThemDuAn(2026, save.mock.calls[0]![0]);
    expect(body.ok && body.than.org_unit_id).toBe("01JUNIT_DIACHINH");
    expect(body.ok && body.than.assignee_id).toBe("CB-00101");

    const blank = thanThemDuAn(2026, { ...FORM_DU_AN_TRONG, hangMucID: "01JHM1", ten: "B", keHoachVon: "1" });
    expect(blank.ok).toBe(true);
    expect(blank.ok && blank.than.org_unit_id).toBeUndefined();
    expect(blank.ok && blank.than.assignee_id).toBeUndefined();
  });

  it("lookup refused (403): selects disabled, the server's sentence shown, nothing sent for them", () => {
    const save = vi.fn();
    const el = addForm(DENIED, save);
    const unit = el.querySelector<HTMLSelectElement>("#don-vi-du-an")!;
    expect(unit.disabled).toBe(true);
    expect(el.querySelector<HTMLSelectElement>("#can-bo-du-an")!.disabled).toBe(true);
    expect(el.textContent).toContain(DENIED_SENTENCE);
    // The rest of the form still saves.
    act(() => el.querySelector<HTMLButtonElement>('button[type="submit"]')!.click());
    const body = thanThemDuAn(2026, save.mock.calls[0]![0]);
    expect(body.ok).toBe(true);
    expect(body.ok && body.than.org_unit_id).toBeUndefined();
    expect(body.ok && body.than.assignee_id).toBeUndefined();
  });
});

describe("§9 selects — edit (PATCH only what changed)", () => {
  const assigned = project({ org_unit_id: "01JUNIT_DIACHINH", assignee_id: "CB-00101" });

  it("pre-filled from the project; an untouched select is ABSENT from the PATCH", () => {
    const start = giaTriTuDuAn(assigned);
    expect(start.orgUnitId).toBe("01JUNIT_DIACHINH");
    expect(start.assigneeId).toBe("CB-00101");
    const kq = thanSuaDuAn(start, { ...start, ten: "Tên mới" });
    expect(kq).toEqual({ ok: true, than: { name: "Tên mới" } });
  });

  it("a changed officer is sent; setting back to 'Chưa phân công' sends \"\" (the server stores NULL)", () => {
    const start = giaTriTuDuAn(assigned);
    expect(thanSuaDuAn(start, { ...start, assigneeId: "CB-00102" })).toEqual({
      ok: true,
      than: { assignee_id: "CB-00102" },
    });
    expect(thanSuaDuAn(start, { ...start, orgUnitId: "" })).toEqual({ ok: true, than: { org_unit_id: "" } });
  });

  it("a saved officer no longer in the directory keeps an option of its own — never the placeholder, never the code", () => {
    const stale = project({ assignee_id: "CB-09999" });
    const el = mount(
      <FormDuAn
        tieuDeForm="Sửa dự án"
        budgetYear={2026}
        giaTriDau={giaTriTuDuAn(stale)}
        maChiDoc={stale.code}
        danhMuc={[]}
        fundingCatalogue={{ phase: "ready", items: [] }}
        people={READY}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />,
    );
    const officer = el.querySelector<HTMLSelectElement>("#can-bo-du-an")!;
    expect(officer.value).toBe("CB-09999");
    expect(officer.selectedOptions[0]?.textContent).toBe(OFFICER_NOT_LISTED);
    expect(el.textContent).not.toContain("CB-09999");
  });
});

describe("§7.2 list column `Đơn vị / phụ trách`", () => {
  it("unit name first, the officer when no unit, 'Chưa phân công' when neither — never an id", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={list([
          project({ id: "P1", org_unit_id: "01JUNIT_VANPHONG", assignee_id: "CB-00101" }),
          project({ id: "P2", assignee_id: "CB-00102" }),
          project({ id: "P3" }),
        ])}
        danhMuc={[]}
        people={READY}
      />,
    );
    expect(unitOwnerCells(html)).toEqual(["Văn phòng UBND", "Trần Thị Bình", UNASSIGNED]);
    expect(html).not.toContain('aria-label="Đơn vị và cán bộ phụ trách');
  });

  it("references the commune's catalogues do not list read 'Chưa phân công', and are not printed", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={list([project({ org_unit_id: "01JUNIT_GONE", assignee_id: "CB-09999" })])}
        danhMuc={[]}
        people={READY}
      />,
    );
    expect(unitOwnerCells(html)).toEqual([UNASSIGNED]);
    expect(html).not.toContain("01JUNIT_GONE");
    expect(html).not.toContain("CB-09999");
  });

  it("lookup refused: an ASSIGNED row shows '—' (not a false 'Chưa phân công'), ids hidden; unassigned still says so", () => {
    const html = renderToStaticMarkup(
      <BangDanhSach
        duLieu={list([project({ id: "P1", org_unit_id: "01JUNIT_DIACHINH", assignee_id: "CB-00101" }), project({ id: "P2" })])}
        danhMuc={[]}
        people={DENIED}
      />,
    );
    expect(unitOwnerCells(html)).toEqual(["—", UNASSIGNED]);
    expect(html).not.toContain("01JUNIT_DIACHINH");
    expect(html).not.toContain("CB-00101");
  });
});

describe("§8 detail — `Đơn vị thực hiện` and `phụ trách …`", () => {
  it("names both", () => {
    const el = mount(
      <ThongTinDuAn duAn={project({ org_unit_id: "01JUNIT_DIACHINH", assignee_id: "CB-00101" })} people={READY} />,
    );
    expect(el.querySelector("[data-project-officer]")?.textContent).toBe(" · phụ trách Nguyễn Văn An");
    const unitFigure = [...el.querySelectorAll("dt")].find((dt) => dt.textContent === "Đơn vị thực hiện");
    expect(unitFigure?.nextElementSibling?.textContent).toBe("Địa chính – Xây dựng");
    expect(el.querySelector("[data-pending]")).toBeNull();
  });

  it("nobody assigned: '—' for the unit, 'Chưa phân công' for the officer (spec §8)", () => {
    const el = mount(<ThongTinDuAn duAn={project()} people={READY} />);
    expect(el.querySelector("[data-project-officer]")?.textContent).toBe(" · phụ trách Chưa phân công");
    const unitFigure = [...el.querySelectorAll("dt")].find((dt) => dt.textContent === "Đơn vị thực hiện");
    expect(unitFigure?.nextElementSibling?.textContent).toBe("—");
  });

  it("lookup refused: '—' for both, no id on the page", () => {
    const el = mount(
      <ThongTinDuAn duAn={project({ org_unit_id: "01JUNIT_DIACHINH", assignee_id: "CB-00101" })} people={DENIED} />,
    );
    expect(el.querySelector("[data-project-officer]")?.textContent).toBe(" · phụ trách —");
    expect(el.textContent).not.toContain("01JUNIT_DIACHINH");
    expect(el.textContent).not.toContain("CB-00101");
  });
});

describe("useProjectPeople — one read per catalogue", () => {
  function Probe({ onPeople }: { onPeople: (p: ProjectPeople) => void }) {
    onPeople(useProjectPeople());
    return null;
  }

  it("reads org-units and staff-directory ONCE each, and a 403 is that catalogue's error phase only", async () => {
    const fetchSpy = vi.fn(async (url: string) => {
      if (url === "/api/v1/org-units") return new Response(JSON.stringify({ items: UNITS }), { status: 200 });
      if (url === "/api/v1/staff-directory") {
        return new Response(JSON.stringify({ code: "forbidden", message: DENIED_SENTENCE }), { status: 403 });
      }
      return new Response("unexpected", { status: 500 });
    });
    vi.stubGlobal("fetch", fetchSpy);
    let seen: ProjectPeople | null = null;
    mount(<Probe onPeople={(p) => (seen = p)} />);
    await act(async () => {});
    await act(async () => {});

    expect(fetchSpy.mock.calls.map((c) => c[0])).toEqual(["/api/v1/org-units", "/api/v1/staff-directory"]);
    const people = seen as unknown as ProjectPeople;
    expect(people.units.phase).toBe("ready");
    expect(people.staff.phase).toBe("error");
  });
});

describe("registry — the two unit / officer placeholders are gone", () => {
  it("no PHAN_CHUA_DUNG_GHI entry and no stale sentence", () => {
    for (const ten of ["Đơn vị thực hiện và Cán bộ phụ trách", "Đơn vị và cán bộ phụ trách của dự án"]) {
      expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === ten)).toBe(false);
      expect(() => pendingPart(ten)).toThrow();
    }
    // A sentence saying the officer cannot be set yet. Merely NAMING the officer is fine: §8.1's tracking
    // task entry ("Khi dự án đã có cán bộ phụ trách…") is about something else that is not built.
    for (const p of PHAN_CHUA_DUNG_GHI) expect(p.viSao).not.toMatch(/chưa[^.]*cán bộ phụ trách/);
  });
});
