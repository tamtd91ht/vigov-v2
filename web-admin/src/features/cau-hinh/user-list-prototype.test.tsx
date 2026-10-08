import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BieuMauGhiCanBo } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import type { DangMoGhi, MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { BAN_TRONG, banTuCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo } from "./danh-ba-can-bo";
import type { ThaoTacDong } from "./danh-ba-can-bo";
import { DELETE_BLOCKED_REASON, EMPTY_STAFF_LIST, staffCountLine } from "./nhan-can-bo";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * `/nguoi-dung` against the prototype (`vigov-require/apps/admin/src/components/admin/UserTable.tsx`,
 * `UserFormDialog.tsx`) and the owner's decisions of 08/10/2026 (fix-web-admin card A):
 *
 *   · the table's columns, in order, with `Mã cán bộ` and `Ngày tạo` gone;
 *   · `Trash2` per row only for `admin.user.delete`, disabled on a row holding a sign-in account;
 *   · one empty row inside the table, the prototype's sentence;
 *   · the count line `Hiển thị N/M cán bộ.`;
 *   · the profile dialog in the prototype's two-column shape, `Huỷ` first — and the SAME component
 *     still rendering the old shape for `/danh-ba` and `/mini-app`, which must not change.
 *
 * Phone numbers are the repository's agreed fakes (rule 3, invariant 5).
 */

const EMPTY_LOOKUP: BangTraDanhMuc = { pha: "xong", ten: new Map() };

const NO_ACTIONS: ThaoTacDong = {
  sua: () => {},
  doiVaiTro: () => {},
  datKhoa: () => {},
  capTaiKhoan: () => {},
  datLaiMatKhau: () => {},
  xoa: () => {},
};

const STAFF: identity_canBoTomTat = {
  id: "01J000000000000000000001",
  code: "CB001",
  full_name: "Huỳnh Văn A",
  email: "demo@thangbinh.test",
  position: "Chuyên viên",
  department_id: "",
  role_id: "",
  phone: "02350000000",
  mobile: "0900000000",
  has_account: false,
  active: true,
  last_login_at: null,
  created_at: "2026-09-22T08:00:00Z",
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
};

function table(rows: readonly identity_canBoTomTat[] = [STAFF], canDelete = false): string {
  return renderToStaticMarkup(
    <BangCanBo
      danhSach={rows}
      thaoTac={NO_ACTIONS}
      traBoPhan={EMPTY_LOOKUP}
      traVaiTro={EMPTY_LOOKUP}
      canDelete={canDelete}
    />,
  );
}

function thead(html: string): string {
  return /<thead[^>]*>([\s\S]*?)<\/thead>/.exec(html)?.[1] ?? "";
}

/** Visible words of each `<th>`, in order (tags, hidden text and the "?" marker stripped). */
function headers(html: string): string[] {
  return [...thead(html).matchAll(/<th[^>]*>([\s\S]*?)<\/th>/g)].map((m) =>
    (m[1] ?? "")
      .replace(/<span class="an-thi-giac">[\s\S]*?<\/span>/g, "")
      .replace(/<span aria-hidden="true">\?<\/span>/g, "")
      .replace(/<[^>]+>/g, "")
      .trim(),
  );
}

/** Every `<button …>` opening tag whose `aria-label` starts with `label`. */
function buttons(html: string, label: string): string[] {
  return [...html.matchAll(new RegExp(`<button[^>]*aria-label="${label}[^"]*"[^>]*>`, "g"))].map((m) => m[0]);
}

describe("table columns — the owner's list, in order (decision 3)", () => {
  it("nine data columns then the actions column; no Mã cán bộ, no Ngày tạo", () => {
    expect(headers(table())).toEqual([
      "Họ và tên",
      "Chức danh",
      "Bộ phận",
      "Vai trò",
      "Máy bàn cơ quan",
      "Di động cá nhân",
      "Đăng nhập gần nhất",
      "Trạng thái",
      "Tài khoản",
      "",
    ]);
  });

  it("the prototype's six heads are ENABLED sort buttons; no '?' marker anywhere in the head (owner 08/10)", () => {
    const head = thead(table());
    const sortButtons = [...head.matchAll(/<button[^>]*class="flex items-center gap-1\.5[^"]*"[^>]*>([\s\S]*?)<\/button>/g)];
    expect(sortButtons.map((m) => (m[1] ?? "").replace(/<[^>]+>/g, "").trim())).toEqual([
      "Họ và tên",
      "Chức danh",
      "Bộ phận",
      "Máy bàn cơ quan",
      "Đăng nhập gần nhất",
      "Trạng thái",
    ]);
    for (const m of sortButtons) {
      expect(m[0]).not.toContain("disabled");
      expect(m[0]).toMatch(/<svg[^>]*class="[^"]*size-3 opacity-40/);
    }
    expect(head).not.toContain("data-pending-marker");
  });

  it("Vai trò, Di động cá nhân, Tài khoản are plain header text — no button, no arrow, no '?'", () => {
    const ths = [...thead(table()).matchAll(/<th[^>]*>([\s\S]*?)<\/th>/g)].map((m) => m[1] ?? "");
    for (const label of ["Vai trò", "Di động cá nhân", "Tài khoản"]) {
      const cell = ths.find((c) => c.replace(/<[^>]+>/g, "").trim() === label);
      expect(cell).toBe(label);
    }
  });

  it("no sort chosen → no aria-sort on any head", () => {
    expect(thead(table())).not.toContain("aria-sort");
  });

  it("the active column carries aria-sort with its direction, keeps the ArrowUpDown icon, no ↑/↓ glyph", () => {
    const render = (order: "asc" | "desc") =>
      thead(
        renderToStaticMarkup(
          <BangCanBo
            danhSach={[STAFF]}
            thaoTac={NO_ACTIONS}
            traBoPhan={EMPTY_LOOKUP}
            traVaiTro={EMPTY_LOOKUP}
            sort={{ key: "last_login_at", order }}
            onSort={() => {}}
          />,
        ),
      );
    for (const [order, aria] of [
      ["asc", "ascending"],
      ["desc", "descending"],
    ] as const) {
      const head = render(order);
      const sorted = [...head.matchAll(/<th[^>]*aria-sort="([^"]+)"[^>]*>([\s\S]*?)<\/th>/g)];
      expect(sorted).toHaveLength(1);
      expect(sorted[0]?.[1]).toBe(aria);
      expect((sorted[0]?.[2] ?? "").replace(/<[^>]+>/g, "").trim()).toBe("Đăng nhập gần nhất");
      expect(sorted[0]?.[2]).toMatch(/<svg[^>]*class="[^"]*size-3 opacity-40/);
      expect(head).not.toMatch(/[↑↓]/);
    }
  });

  it("name navy semibold, email muted 11.5px under it, no ellipsis anywhere", () => {
    const html = table();
    expect(html).toMatch(/<div class="text-navy font-semibold">Huỳnh Văn A<\/div>/);
    expect(html).toMatch(/<div class="text-ink-muted text-\[11\.5px\]">demo@thangbinh\.test<\/div>/);
    expect(html).not.toContain("text-ellipsis");
    expect(html).not.toContain("max-w-[14rem]");
  });

  it("empty Chức danh, Bộ phận and phones read '—'", () => {
    const html = table([{ ...STAFF, position: "", phone: "", mobile: "" }]);
    // Chức danh, Máy bàn, Di động as bare cells; Bộ phận as its catalogue span.
    expect([...html.matchAll(/<td[^>]*>—<\/td>/g)]).toHaveLength(3);
    expect(html).toMatch(/<td[^>]*><span[^>]*>—<\/span><\/td>/);
    expect(html).not.toContain("Chưa phân bộ phận");
  });

  it("status Badge: active leaf 'Đang hoạt động', locked danger 'Tạm khoá'", () => {
    expect(table([{ ...STAFF, active: true }])).toMatch(/bg-leaf\/12[^"]*text-leaf[^"]*"[^>]*>.*?Đang hoạt động/);
    expect(table([{ ...STAFF, active: false }])).toMatch(/bg-danger\/12[^"]*text-danger[^"]*"[^>]*>.*?Tạm khoá/);
  });

  it("the frame is the prototype's hairline box, not a Card, and not the legacy `.bang-can-bo`", () => {
    const html = table();
    expect(html).toContain("rounded-[10px]");
    expect(html).not.toContain("bang-can-bo");
    expect(html).not.toContain('data-slot="card"');
  });
});

describe("row actions — small outline icon buttons, right-aligned (decision 3)", () => {
  it("Pencil is titled 'Sửa tài khoản' and carries the person in aria-label; no 'Chi tiết' button", () => {
    const html = table();
    expect(html).toContain('title="Sửa tài khoản"');
    expect(html).toContain('aria-label="Sửa tài khoản: Huỳnh Văn A"');
    expect(html).not.toContain("Chi tiết");
    expect(html).toContain("flex items-center justify-end gap-1.5");
  });
});

describe("Trash2 — gated by admin.user.delete, disabled on a sign-in account (decision 1)", () => {
  it("DENIED: without the key there is no delete button on any row", () => {
    const html = table([STAFF, { ...STAFF, id: "01J000000000000000000002", has_account: true }], false);
    expect(html).not.toContain("Xoá tài khoản");
  });

  it("ALLOWED: with the key, a row with no account has an enabled red Trash2 named for the person", () => {
    const [button] = buttons(table([STAFF], true), "Xoá tài khoản");
    expect(button).toBeDefined();
    expect(button).toContain('title="Xoá tài khoản"');
    expect(button).toContain('aria-label="Xoá tài khoản: Huỳnh Văn A"');
    expect(button).toContain("text-danger");
    expect(button).not.toContain('disabled=""');
  });

  it("a row holding a sign-in account: Trash2 is disabled and the short reason is reachable", () => {
    const row = { ...STAFF, has_account: true };
    const html = table([row], true);
    const [button] = buttons(html, "Xoá tài khoản");
    expect(button).toContain('disabled=""');
    expect(button).toContain(`aria-describedby="delete-blocked-${row.id}"`);
    expect(html).toContain(`id="delete-blocked-${row.id}"`);
    expect(html).toContain(DELETE_BLOCKED_REASON);
  });

  it("the old 'no delete button here' paragraph is gone from this screen", () => {
    expect(table([STAFF], true)).not.toContain("Dòng nhập trùng");
  });
});

describe("empty state — one row inside the table (U26)", () => {
  it("no rows → the prototype's sentence in a full-width cell", () => {
    const html = table([]);
    expect(html).toContain('colSpan="10"');
    expect(html).toContain("text-ink-muted py-10 text-center");
    expect(html).toContain(EMPTY_STAFF_LIST);
    expect(EMPTY_STAFF_LIST).toBe("Không có cán bộ nào khớp điều kiện tìm kiếm.");
  });
});

describe("count line (decision 3)", () => {
  it("N/M when the commune total is known, N alone when it is not", () => {
    expect(staffCountLine(20, 57)).toBe("Hiển thị 20/57 cán bộ.");
    expect(staffCountLine(3, null)).toBe("Hiển thị 3 cán bộ.");
  });
});

/* ---- the profile dialog: prototype shape on /nguoi-dung, old shape everywhere else ---------------- */

const UNITS: readonly MucChon[] = [{ id: "BP1", name: "Văn phòng" }];

function form(openForm: DangMoGhi, layout?: "account", draft: BanNhapCanBo = BAN_TRONG): string {
  return renderToStaticMarkup(
    <BieuMauGhiCanBo
      dangMo={openForm}
      ban={draft}
      datBan={() => {}}
      vaiTroID=""
      datVaiTroID={() => {}}
      boPhan={UNITS}
      vaiTro={[]}
      loiMayChu=""
      dangGui={false}
      onGui={() => {}}
      onHuy={() => {}}
      layout={layout}
    />,
  );
}

describe("account layout — UserFormDialog's shape (decision 2)", () => {
  const ADD: DangMoGhi = { kieu: "them", khoaChongTrung: "k" };
  const EDIT: DangMoGhi = { kieu: "sua", canBo: STAFF };

  it("two-column grid, name spanning both, prototype placeholders", () => {
    const html = form(ADD, "account");
    expect(html).toContain("grid grid-cols-2 gap-4");
    expect(html).toMatch(/<div class="[^"]*col-span-2[^"]*"><label[^>]*for="o-ho-ten-can-bo"/);
    expect(html).toContain('placeholder="Nguyễn Văn A"');
    expect(html).toContain('placeholder="canbo@xa.gov.vn"');
    expect(html).toContain('placeholder="Công chức Văn phòng"');
    expect([...html.matchAll(/placeholder="0905…"/g)]).toHaveLength(2);
  });

  it("email: label 'Thư điện tử', type=email, editable on edit too", () => {
    const html = form(EDIT, "account", banTuCanBo(STAFF));
    expect(html).toMatch(/<label[^>]*for="o-email-can-bo"[^>]*>Thư điện tử<\/label>/);
    const input = /<input[^>]*id="o-email-can-bo"[^>]*>/.exec(html)?.[0] ?? "";
    expect(input).toContain('type="email"');
    expect(input).not.toContain('disabled=""');
  });

  it("the two phone fields keep their own labels (#16)", () => {
    const html = form(ADD, "account");
    expect(html).toContain(">Máy bàn cơ quan</label>");
    expect(html).toContain(">Di động cá nhân</label>");
  });

  it("Bộ phận spans both columns, native select, first option '— Chưa xếp bộ phận —'", () => {
    const html = form(ADD, "account");
    const select = /<select[^>]*id="o-bo-phan-can-bo"[^>]*>([\s\S]*?)<\/select>/.exec(html);
    expect(select?.[0]).toContain("h-9");
    expect(select?.[0]).toContain("text-[13px]");
    expect(select?.[1]).toMatch(/^<option value=""( selected="")?>— Chưa xếp bộ phận —<\/option>/);
    expect(html).toMatch(/<div class="[^"]*col-span-2[^"]*"><label[^>]*for="o-bo-phan-can-bo"/);
  });

  it("Has-Zalo checkbox stays on edit, native accent-brand", () => {
    const html = form(EDIT, "account", banTuCanBo(STAFF));
    expect(html).toMatch(/<input[^>]*type="checkbox"[^>]*class="[^"]*accent-brand[^"]*size-3\.5/);
  });

  it("footer: outline Huỷ FIRST, then the primary — 'Thêm cán bộ' on add, 'Lưu' on edit; no 44px rule hook", () => {
    const add = form(ADD, "account");
    expect(add.indexOf(">Huỷ<")).toBeGreaterThan(-1);
    expect(add.indexOf(">Huỷ<")).toBeLessThan(add.indexOf("Thêm cán bộ</button>"));
    const edit = form(EDIT, "account", banTuCanBo(STAFF));
    expect(edit.indexOf(">Huỷ<")).toBeLessThan(edit.indexOf(">Lưu</button>"));
    // `.form-danh-muc .nut-phu { min-height: 44px }` must not reach these buttons.
    expect(add).not.toContain("form-danh-muc");
    expect(add).not.toContain("cum-nut");
  });

  it("the same footer order for Đổi vai trò and Khoá on this screen", () => {
    const role = form({ kieu: "vaiTro", canBo: STAFF }, "account");
    expect(role.indexOf(">Huỷ<")).toBeLessThan(role.indexOf(">Lưu</button>"));
    const lock = form({ kieu: "khoa", canBo: STAFF, khoa: true }, "account");
    expect(lock.indexOf(">Huỷ<")).toBeLessThan(lock.indexOf("Xác nhận khoá</button>"));
  });
});

describe("contact layout — /danh-ba and /mini-app render UNCHANGED", () => {
  it("default layout keeps the in-flow form, its <h4>, Lưu before Huỷ, and the old labels", () => {
    const html = form({ kieu: "sua", canBo: STAFF }, undefined, banTuCanBo(STAFF));
    expect(html).toContain('class="form-danh-muc"');
    expect(html).toContain("<h4>Sửa hồ sơ: Huỳnh Văn A</h4>");
    expect(html).toContain("Thư điện tử công vụ");
    expect(html).toContain("— Chưa phân bộ phận —");
    expect(html).not.toContain("grid-cols-2");
    expect(html).not.toContain("placeholder=");
    expect(html.indexOf(">Lưu<")).toBeLessThan(html.indexOf(">Huỷ<"));
  });
});
