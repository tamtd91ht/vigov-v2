import type { ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { BayDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_trangThaiNhiemVuRa,
} from "@/lib/api/schema.gen";

import { NUT_TAT, NUT_XOA } from "./nhan-danh-muc";
import { TASK_STATUS_GROUP, nhomDanhMuc } from "./nhom-danh-muc";
import {
  CatalogueView,
  DELETE_ENTRY,
  EDIT_LABEL,
  EMPTY_DRAFT,
  SET_DEFAULT,
  type CatalogueActions,
} from "./tab-danh-muc";
import {
  CHUA_DOI_GI,
  NUT_DAT_LAI,
  THU_TU_KHONG_PHAI_SO,
  thanDatLaiMacDinh,
  thanSuaTrangThai,
} from "./trang-thai-nhiem-vu";

/**
 * The eighth group of the Danh mục tab — `Trạng thái nhiệm vụ` (decision #21, ADR 0035 §C), now rows of
 * the tab's single table (spec 05).
 *
 * What this guards: these rows NEVER offer "Tắt", "Bật", the bin or "Đặt mặc định" — for any account,
 * any row. The seven other groups have them at tier 1, so reusing their row for convenience would carry
 * them here while the screen still looks normal. Pencil only.
 */

/** Seven rows as the server returns them: `moi-giao` relabelled, `cho-duyet` reordered. */
const SEVEN: readonly petitions_trangThaiNhiemVuRa[] = [
  { code: "moi-giao", label: "Chưa thực hiện", order: 1, role: "chinh", default_label: "Mới giao", default_order: 1, customised: true },
  { code: "da-tiep-nhan", label: "Đã tiếp nhận", order: 2, role: "chinh", default_label: "Đã tiếp nhận", default_order: 2, customised: false },
  { code: "cho-duyet", label: "Chờ duyệt", order: 3, role: "chinh", default_label: "Chờ duyệt", default_order: 4, customised: true },
  { code: "dang-thuc-hien", label: "Đang thực hiện", order: 3, role: "chinh", default_label: "Đang thực hiện", default_order: 3, customised: false },
  { code: "hoan-thanh", label: "Hoàn thành", order: 5, role: "chinh", default_label: "Hoàn thành", default_order: 5, customised: false },
  { code: "tam-dung", label: "Tạm dừng", order: 6, role: "re-nhanh", default_label: "Tạm dừng", default_order: 6, customised: false },
  { code: "chuyen-tiep", label: "Chuyển tiếp", order: 7, role: "re-nhanh", default_label: "Chuyển tiếp", default_order: 7, customised: false },
];

const READ: KetQua<petitions_danhSachTrangThaiNhiemVuRa> = { ok: true, duLieu: { items: [...SEVEN] } };

const EMPTY = { ok: true as const, duLieu: { items: [] } };
const NO_CATALOGUES = nhomDanhMuc({
  loaiTaiNguyenBanDo: EMPTY,
  hangMucKeHoachVon: EMPTY,
  loaiVanBan: EMPTY,
  loaiDonViDanCu: EMPTY,
  khoiNhiemVu: EMPTY,
  loaiNhiemVu: EMPTY,
  mucUuTienNhiemVu: EMPTY,
} as BayDanhMuc);

const NO_ACTIONS: CatalogueActions = {
  toggleAdd: () => {},
  edit: () => {},
  remove: () => {},
  setActive: () => {},
  makeDefault: () => {},
  submit: () => {},
  cancel: () => {},
};

function view(p: Partial<ComponentProps<typeof CatalogueView>> = {}): string {
  return renderToStaticMarkup(
    <CatalogueView
      groups={NO_CATALOGUES}
      taskStatuses={READ}
      shownGroup={TASK_STATUS_GROUP}
      onShowGroup={() => {}}
      canWrite
      sessionError=""
      importButton={null}
      open={null}
      draft={EMPTY_DRAFT}
      setDraft={() => {}}
      localError=""
      serverError=""
      busy={false}
      actions={NO_ACTIONS}
      {...p}
    />,
  );
}

function count(html: string, s: string): number {
  return html.split(s).length - 1;
}

describe("Trạng thái nhiệm vụ — seven rows, pencil only (#21)", () => {
  it("draws all SEVEN rows, in the server's order, group name first, code in <code>", () => {
    const html = view();
    const tbody = html.slice(html.indexOf("<tbody>"));
    expect(count(tbody, "<tr>")).toBe(7);
    expect(tbody.indexOf(">cho-duyet<")).toBeLessThan(tbody.indexOf(">dang-thuc-hien<"));
    expect(html).toContain('<code class="text-[11.5px]">moi-giao</code>');
    expect(count(tbody, '<td class="text-ink-muted">Trạng thái nhiệm vụ</td>')).toBe(7);
  });

  it("source 'Hệ thống', status 'Đang dùng' — the seven codes ship with the software and never switch off", () => {
    const html = view();
    expect(count(html, ">Hệ thống</span>")).toBe(7);
    expect(count(html, "Đang dùng")).toBe(7);
    expect(html).not.toContain("Xã tự thêm");
  });

  it("with `admin.lookup`: one pencil per row and NOTHING else — no Tắt, Bật, bin, Đặt mặc định, Đặt lại", () => {
    const html = view();
    expect(count(html, `title="${EDIT_LABEL}"`)).toBe(7);
    expect(html).not.toContain(`>${NUT_TAT}</button>`);
    expect(html).not.toContain(">Bật</button>");
    expect(html).not.toContain(DELETE_ENTRY);
    expect(html).not.toContain(`>${NUT_XOA}<`);
    expect(html).not.toContain(SET_DEFAULT);
    expect(html).not.toContain(NUT_DAT_LAI);
  });

  it("DENIED — no `admin.lookup`: no pencil; the table STILL shows", () => {
    const html = view({ canWrite: false });
    expect(html).not.toContain(EDIT_LABEL);
    expect(html).toContain("moi-giao");
    expect(html).toContain("Chưa thực hiện");
  });

  it("edit: label and order inputs only — no checkbox, no code field", () => {
    const html = view({
      open: { kind: "edit", rowKey: `${TASK_STATUS_GROUP}:moi-giao` },
      draft: { ...EMPTY_DRAFT, label: "Chưa thực hiện", order: "1" },
    });
    expect(html).toContain('aria-label="Nhãn hiển thị — mục Chưa thực hiện"');
    expect(html).toContain('aria-label="Thứ tự — mục Chưa thực hiện"');
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toContain('name="code"');
  });

  it("read failure: the server's sentence as an alert, and no table for this group", () => {
    const html = view({ taskStatuses: { ok: false, thongBao: "Đã xảy ra lỗi. Vui lòng thử lại." } });
    expect(html).toContain("Trạng thái nhiệm vụ: Đã xảy ra lỗi. Vui lòng thử lại.");
    expect(html).toContain('role="alert"');
    expect(html).not.toContain("<table");
  });
});

describe("PATCH body — built field by field, only what changed", () => {
  const row = SEVEN[1] as petitions_trangThaiNhiemVuRa; // da-tiep-nhan, "Đã tiếp nhận", 2

  it("label only → body has ONLY `label`, trimmed", () => {
    expect(thanSuaTrangThai(row, { nhan: "  Đã nhận việc  ", thuTu: "2" })).toEqual({
      ok: true,
      than: { label: "Đã nhận việc" },
    });
  });

  it("order only → body has ONLY `order`; an empty order box means UNCHANGED", () => {
    expect(thanSuaTrangThai(row, { nhan: "Đã tiếp nhận", thuTu: "8" })).toEqual({ ok: true, than: { order: 8 } });
    expect(thanSuaTrangThai(row, { nhan: "Mới", thuTu: "" })).toEqual({ ok: true, than: { label: "Mới" } });
  });

  it("NEVER `code` or `active` in the body", () => {
    const r = thanSuaTrangThai(row, { nhan: "X", thuTu: "9" });
    expect(r.ok && Object.keys(r.than).sort()).toEqual(["label", "order"]);
  });

  it("a mistyped order is refused in place, never silently dropped", () => {
    expect(thanSuaTrangThai(row, { nhan: "X", thuTu: "3 chữ" })).toEqual({ ok: false, loi: THU_TU_KHONG_PHAI_SO });
  });

  it("nothing changed: nothing sent", () => {
    expect(thanSuaTrangThai(row, { nhan: "Đã tiếp nhận", thuTu: "2" })).toEqual({ ok: false, loi: CHUA_DOI_GI });
  });

  it("`Đặt lại mặc định` body = exactly the server's `default_label` + `default_order`", () => {
    expect(thanDatLaiMacDinh(SEVEN[2] as petitions_trangThaiNhiemVuRa)).toEqual({ label: "Chờ duyệt", order: 4 });
  });
});
