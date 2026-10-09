import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BieuMauGhiCanBo } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import type { DangMoGhi, MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { BAN_TRONG, banTuCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo, hasOtherPage } from "./danh-ba-can-bo";
import type { ThaoTacDong } from "./danh-ba-can-bo";
import { sangTrangSau, TRANG_DAU } from "./ngan-xep-con-tro";
import {
  DELETE_BLOCKED_REASON,
  EMAIL_LOCKED_HINT,
  EMPTY_STAFF_LIST,
  NO_EMAIL_SUBLINE,
  OWN_ROLE_HINT,
  staffCountLine,
} from "./nhan-can-bo";
import { StaffAccountForm } from "./staff-account-form";
import type { RoleChoice } from "./staff-account-form";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * `/nguoi-dung` against the prototype (`vigov-require/apps/admin/src/components/admin/UserTable.tsx`,
 * `UserFormDialog.tsx`), the owner's decisions of 08/10/2026 (fix-web-admin card A) and the user's of
 * 09/10/2026 (seven columns, one role in the edit dialog, no password field):
 *
 *   · the table's seven columns, in order;
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

describe("table columns — the prototype's seven (user 09/10/2026)", () => {
  it("SEVEN headers: six data columns then the actions column with no visible head", () => {
    expect(headers(table())).toEqual([
      "Họ và tên",
      "Chức danh",
      "Bộ phận",
      "Điện thoại",
      "Đăng nhập gần nhất",
      "Trạng thái",
      "",
    ]);
    // The columns the user removed are gone from the head.
    const head = thead(table());
    for (const gone of ["Vai trò", "Máy bàn cơ quan", "Di động cá nhân", "Tài khoản"]) {
      expect(head).not.toContain(`>${gone}<`);
    }
    // The actions head has no VISIBLE words; a screen reader still gets its name.
    expect(head).toContain('<th scope="col" class="text-right"><span class="an-thi-giac">Hành động</span></th>');
  });

  it("the five heads with a server sort key are ENABLED sort buttons; no '?' marker anywhere in the head", () => {
    const head = thead(table());
    const sortButtons = [...head.matchAll(/<button[^>]*class="flex items-center gap-1\.5[^"]*"[^>]*>([\s\S]*?)<\/button>/g)];
    expect(sortButtons.map((m) => (m[1] ?? "").replace(/<[^>]+>/g, "").trim())).toEqual([
      "Họ và tên",
      "Chức danh",
      "Bộ phận",
      "Đăng nhập gần nhất",
      "Trạng thái",
    ]);
    for (const m of sortButtons) {
      expect(m[0]).not.toContain("disabled");
      expect(m[0]).toMatch(/<svg[^>]*class="[^"]*size-3 opacity-40/);
    }
    expect(head).not.toContain("data-pending-marker");
  });

  it("Điện thoại is plain header text — the server has no key for `mobile ?? office`, none is invented", () => {
    const ths = [...thead(table()).matchAll(/<th[^>]*>([\s\S]*?)<\/th>/g)].map((m) => m[1] ?? "");
    const cell = ths.find((c) => c.replace(/<[^>]+>/g, "").trim() === "Điện thoại");
    expect(cell).toBe("Điện thoại");
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

  it("no email → the sub-line says 'Chưa có thư điện tử', muted, never an empty line", () => {
    const html = table([{ ...STAFF, email: "  " }]);
    expect(html).toContain(`<div class="text-ink-muted text-[11.5px]">${NO_EMAIL_SUBLINE}</div>`);
    expect(NO_EMAIL_SUBLINE).toBe("Chưa có thư điện tử");
  });

  it("empty Chức danh, Bộ phận and phone read '—'", () => {
    const html = table([{ ...STAFF, position: "", phone: "", mobile: "" }]);
    // Chức danh and Điện thoại as bare cells; Bộ phận as its catalogue span.
    expect([...html.matchAll(/<td>—<\/td>/g)]).toHaveLength(2);
    expect(html).toMatch(/<td[^>]*><span[^>]*>—<\/span><\/td>/);
    expect(html).not.toContain("Chưa phân bộ phận");
  });

  it("status chips are TEXT ONLY: active leaf 'Đang hoạt động', locked danger 'Tạm khoá', no icon", () => {
    const active = table([{ ...STAFF, active: true }]);
    expect(active).toMatch(/<span class="[^"]*bg-leaf\/12[^"]*text-leaf[^"]*">Đang hoạt động<\/span>/);
    expect(table([{ ...STAFF, active: false }])).toMatch(/<span class="[^"]*bg-danger\/12[^"]*text-danger[^"]*">Tạm khoá<\/span>/);
    const body = /<tbody[^>]*>([\s\S]*)<\/tbody>/.exec(active)?.[1] ?? "";
    expect(body).not.toContain("lucide-circle-check");
  });

  it("account chip beside the status: 'Chỉ trong danh bạ' / 'Có tài khoản', long reason in title + hidden text", () => {
    const none = table([{ ...STAFF, has_account: false }]);
    expect(none).toMatch(/<span class="[^"]*"[^>]*title="[^"]+"[^>]*>Chỉ trong danh bạ<\/span>/);
    expect(none).toContain(`id="account-reason-${STAFF.id}"`);
    const has = table([{ ...STAFF, has_account: true }]);
    expect(has).toMatch(/title="Đã cấp tài khoản đăng nhập; thư điện tử là tên đăng nhập\."[^>]*>Có tài khoản<\/span>/);
  });

  it("the frame is the prototype's hairline box, not a Card, and not the legacy `.bang-can-bo`", () => {
    const html = table();
    expect(html).toContain("rounded-[10px]");
    expect(html).not.toContain("bang-can-bo");
    expect(html).not.toContain('data-slot="card"');
  });
});

describe("row actions — Sửa · ⋯ · Xoá, right-aligned (user 09/10/2026)", () => {
  it("Pencil is titled 'Sửa tài khoản' and carries the person in aria-label; no 'Chi tiết' button", () => {
    const html = table();
    expect(html).toContain('title="Sửa tài khoản"');
    expect(html).toContain('aria-label="Sửa tài khoản: Huỳnh Văn A"');
    expect(html).not.toContain("Chi tiết");
    expect(html).toContain("flex items-center justify-end gap-1.5");
  });

  it("the ⋯ trigger sits between Sửa and Xoá, outline square, named for the person; no 'Đổi vai trò' button", () => {
    const html = table([STAFF], true);
    const trigger = buttons(html, "Thao tác khác")[0] ?? "";
    expect(trigger).toContain('aria-label="Thao tác khác: Huỳnh Văn A"');
    expect(trigger).toContain('aria-haspopup="menu"');
    expect(trigger).toContain("w-7");
    expect(html.indexOf("Sửa tài khoản: Huỳnh Văn A")).toBeLessThan(html.indexOf("Thao tác khác: Huỳnh Văn A"));
    expect(html.indexOf("Thao tác khác: Huỳnh Văn A")).toBeLessThan(html.indexOf("Xoá tài khoản: Huỳnh Văn A"));
    expect(html).not.toContain("Đổi vai trò");
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
    expect(html).toContain('colSpan="7"');
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

describe("pager — only when there is another page (user 09/10/2026)", () => {
  it("first page, nothing after → no pager", () => {
    expect(hasOtherPage(TRANG_DAU, false, "")).toBe(false);
    // `has_more` without a cursor is not a way forward either.
    expect(hasOtherPage(TRANG_DAU, true, "")).toBe(false);
  });

  it("a page after, or a page before → pager", () => {
    expect(hasOtherPage(TRANG_DAU, true, "CB-00020")).toBe(true);
    expect(hasOtherPage(sangTrangSau(TRANG_DAU, "CB-00020"), false, "")).toBe(true);
  });
});

/* ---- the /nguoi-dung account form (user 09/10/2026) -------------------------------------------- */

const ROLES: readonly MucChon[] = [
  { id: "VT1", name: "Chuyên viên chuyên môn" },
  { id: "VT2", name: "Kế toán" },
];

function accountForm(
  openForm: Extract<DangMoGhi, { kieu: "them" } | { kieu: "sua" }>,
  opts: { draft?: BanNhapCanBo; roleId?: string; roleChoice?: RoleChoice } = {},
): string {
  return renderToStaticMarkup(
    <StaffAccountForm
      open={openForm}
      draft={opts.draft ?? BAN_TRONG}
      setDraft={() => {}}
      roleId={opts.roleId ?? ""}
      setRoleId={() => {}}
      roleChoice={opts.roleChoice ?? "editable"}
      units={UNITS}
      roles={ROLES}
      serverError=""
      busy={false}
      onSubmit={() => {}}
      onCancel={() => {}}
    />,
  );
}

function input(html: string, id: string): string {
  return new RegExp(`<input[^>]*id="${id}"[^>]*>`).exec(html)?.[0] ?? "";
}

describe("StaffAccountForm — add (user 09/10/2026)", () => {
  const ADD = { kieu: "them", khoaChongTrung: "k" } as const;

  it("NO password field; primary 'Thêm tài khoản' after Huỷ; no role choice on add", () => {
    const html = accountForm(ADD);
    expect(html).not.toContain('type="password"');
    expect(html).not.toMatch(/Mật khẩu/);
    expect(html.indexOf(">Huỷ<")).toBeLessThan(html.indexOf("Thêm tài khoản</button>"));
    expect(html).not.toContain('type="radio"');
    expect(html).not.toContain("o-co-zalo-can-bo");
  });

  it("field order: Họ và tên · Thư điện tử · (Chức danh, Điện thoại (di động)) · Bộ phận · Máy bàn cơ quan", () => {
    const html = accountForm(ADD);
    const order = ["o-ho-ten-can-bo", "o-email-can-bo", "o-chuc-danh-can-bo", "o-di-dong-can-bo", "o-bo-phan-can-bo", "o-may-ban-can-bo"];
    const at = order.map((id) => html.indexOf(`for="${id}"`));
    expect(at.every((x) => x >= 0)).toBe(true);
    expect([...at].sort((a, b) => a - b)).toEqual(at);
    expect(html).toContain(">Điện thoại (di động)</label>");
    expect(html).toContain(">Máy bàn cơ quan</label>");
    expect(input(html, "o-email-can-bo")).not.toContain('disabled=""');
  });
});

describe("StaffAccountForm — edit (user 09/10/2026)", () => {
  it("staff WITH an account: email disabled, the hint under it, described by it", () => {
    const row = { ...STAFF, has_account: true };
    const html = accountForm({ kieu: "sua", canBo: row }, { draft: banTuCanBo(row) });
    const email = input(html, "o-email-can-bo");
    expect(email).toContain('disabled=""');
    expect(email).toContain('aria-describedby="o-email-can-bo-mo-ta"');
    expect(html).toContain(`id="o-email-can-bo-mo-ta" class="text-ink-muted m-0 mt-1 text-[11.5px]">${EMAIL_LOCKED_HINT}<`);
    expect(EMAIL_LOCKED_HINT).toBe("Không đổi được thư điện tử đăng nhập.");
  });

  it("staff WITHOUT an account: email stays editable (to issue the account later), no hint", () => {
    const html = accountForm({ kieu: "sua", canBo: STAFF }, { draft: banTuCanBo(STAFF) });
    expect(input(html, "o-email-can-bo")).not.toContain('disabled=""');
    expect(html).not.toContain(EMAIL_LOCKED_HINT);
  });

  it("ONE role: radios in the prototype's bordered 2-column grid, the stored role checked, 'no role' offered", () => {
    const html = accountForm({ kieu: "sua", canBo: STAFF }, { draft: banTuCanBo(STAFF), roleId: "VT2" });
    expect(html).toContain('role="radiogroup"');
    expect(html).toContain("border-line mt-1.5 grid grid-cols-2 gap-2 rounded-[10px] border border-solid p-3");
    const radios = [...html.matchAll(/<input[^>]*type="radio"[^>]*>/g)].map((m) => m[0]);
    expect(radios).toHaveLength(3);
    expect(radios.filter((r) => r.includes('checked=""'))).toHaveLength(1);
    expect(radios.find((r) => r.includes('value="VT2"'))).toContain('checked=""');
    expect(html).toContain(">Không giữ vai trò nào<");
    expect(html).not.toContain('type="checkbox" name="o-vai-tro');
  });

  it("own row (#14): every role radio disabled, the reason under the group", () => {
    const html = accountForm({ kieu: "sua", canBo: STAFF }, { draft: banTuCanBo(STAFF), roleChoice: "own" });
    const radios = [...html.matchAll(/<input[^>]*type="radio"[^>]*>/g)].map((m) => m[0]);
    expect(radios.length).toBeGreaterThan(0);
    for (const r of radios) expect(r).toContain('disabled=""');
    expect(html).toContain(OWN_ROLE_HINT);
  });

  it("another row: radios enabled, no own-row hint", () => {
    const html = accountForm({ kieu: "sua", canBo: STAFF }, { draft: banTuCanBo(STAFF) });
    for (const m of html.matchAll(/<input[^>]*type="radio"[^>]*>/g)) expect(m[0]).not.toContain('disabled=""');
    expect(html).not.toContain(OWN_ROLE_HINT);
  });

  it("keeps the Zalo checkbox; no password field; primary 'Lưu'", () => {
    const html = accountForm({ kieu: "sua", canBo: STAFF }, { draft: banTuCanBo(STAFF) });
    expect(html).toMatch(/<input[^>]*type="checkbox"[^>]*class="[^"]*accent-brand[^"]*size-3\.5/);
    expect(html).not.toContain('type="password"');
    expect(html.indexOf(">Huỷ<")).toBeLessThan(html.indexOf(">Lưu</button>"));
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

// `BieuMauGhiCanBo`'s account layout: since 09/10/2026 /nguoi-dung opens only its LOCK form in it (the
// profile forms moved to `StaffAccountForm`, above). The component itself is unchanged, so its shape
// is still pinned here.
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
