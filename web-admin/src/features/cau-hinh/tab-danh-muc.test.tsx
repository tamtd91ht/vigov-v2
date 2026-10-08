import type { ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { MucDanhMucGhi } from "@/lib/api/danh-muc";
import type { BayDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";

import {
  GHI_CHU_BA_TANG,
  GHI_CHU_NHOM_CHI_XEM,
  GIAI_THICH_DA_TAT,
  GIAI_THICH_THANG_BAC,
  giaiThichKhongThaoTac,
  nhanNhomRong,
} from "@/features/cau-hinh/nhan-danh-muc";
import { nhomDanhMuc, type KhoaNhom } from "@/features/cau-hinh/nhom-danh-muc";
import {
  ADDED,
  CatalogueView,
  catalogueImportButton,
  DELETE_ENTRY,
  EDIT_LABEL,
  EMPTY_DRAFT,
  SET_DEFAULT,
  SET_DEFAULT_TITLE,
  type CatalogueActions,
} from "@/features/cau-hinh/tab-danh-muc";
import { TANG_DON_VI, TANG_HE_THONG, TANG_RE_NHANH } from "@/features/cau-hinh/tang-danh-muc";

/**
 * The Danh mục tab as it reaches the page (spec `05-danh-muc.md`, ADR 0079): one table across groups,
 * the group filter, the grey add row, in-place edit, the compact delete-with-reason step.
 *
 * WHY RENDER AND NOT ONLY TEST THE PURE MODULES: `thaoTacTheoTang` can answer right while the JSX still
 * draws a bin on a system row. The tier cases below guard exactly that place. `renderToStaticMarkup`
 * runs in plain Node; events and focus stay out of scope.
 */

const NO_ACTIONS: CatalogueActions = {
  toggleAdd: () => {},
  edit: () => {},
  remove: () => {},
  setActive: () => {},
  makeDefault: () => {},
  submit: () => {},
  cancel: () => {},
};

function entry(tier: number, opts: Partial<MucDanhMucGhi> = {}): MucDanhMucGhi {
  return {
    id: `01JH-${tier}-${opts.label ?? "x"}`,
    code: "cong-van",
    label: "Công văn",
    active: true,
    is_default: false,
    order: 7,
    source: tier === TANG_DON_VI ? "don-vi" : "he-thong",
    tier,
    ...opts,
  };
}

type Items = Partial<Record<KhoaNhom, readonly MucDanhMucGhi[] | string>>;

/** Seven reads; a string is a failed read carrying the server's sentence. */
function groupsOf(items: Items) {
  const read = (k: KhoaNhom) => {
    const v = items[k];
    if (typeof v === "string") return { ok: false as const, thongBao: v };
    const rows = v ?? [];
    return {
      ok: true as const,
      duLieu: { items: k === "loaiNhiemVu" ? rows.map((r) => ({ ...r, requires_directive: false })) : rows },
    };
  };
  const bay = {
    loaiTaiNguyenBanDo: read("loaiTaiNguyenBanDo"),
    hangMucKeHoachVon: read("hangMucKeHoachVon"),
    loaiVanBan: read("loaiVanBan"),
    loaiDonViDanCu: read("loaiDonViDanCu"),
    khoiNhiemVu: read("khoiNhiemVu"),
    loaiNhiemVu: read("loaiNhiemVu"),
    mucUuTienNhiemVu: read("mucUuTienNhiemVu"),
  } as BayDanhMuc;
  return nhomDanhMuc(bay);
}

type Props = ComponentProps<typeof CatalogueView>;

function view(items: Items, p: Partial<Props> = {}): string {
  return renderToStaticMarkup(
    <CatalogueView
      groups={groupsOf(items)}
      taskStatuses={{ ok: true, duLieu: { items: [] } }}
      shownGroup={null}
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

const editLabel = (label: string) => `aria-label="${EDIT_LABEL} — mục ${label}"`;
const deleteLabel = (label: string) => `aria-label="${DELETE_ENTRY} — mục ${label}"`;

describe("one table across groups (spec 05 §3)", () => {
  it("rows of several groups sit in ONE table, group name in the first column — no per-group card", () => {
    const html = view({
      loaiVanBan: [entry(TANG_DON_VI)],
      loaiTaiNguyenBanDo: [entry(TANG_DON_VI, { id: "a1", code: "cot-dien", label: "Cột điện" })],
    });
    expect(count(html, "<table")).toBe(1);
    expect(html).toContain(">Nhóm danh mục</th>");
    expect(html).toContain('<td class="text-ink-muted">Loại văn bản</td>');
    expect(html).toContain('<td class="text-ink-muted">Loại tài nguyên bản đồ</td>');
    expect(html).toContain('<code class="text-[11.5px]">cot-dien</code>');
    expect(html).not.toContain("nhom-danh-muc");
  });

  it("filter row: 'Tất cả (n)' pressed by default, one button per group, 'Thêm mục' at the end", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI), entry(TANG_HE_THONG, { id: "b" })] });
    expect(html).toMatch(/aria-pressed="true"[^>]*>Tất cả \(2\)<\/button>/);
    expect(html).toContain(">Loại văn bản</button>");
    expect(html).toMatch(/ml-auto[^>]*>.*Thêm mục<\/button>/);
  });

  it("D22 — a group with no row gets NO filter button (prototype derives them from the data)", () => {
    // Failed before: every group always had a button.
    const html = view({ loaiVanBan: [entry(TANG_DON_VI)] });
    expect(html).toContain(">Loại văn bản</button>");
    expect(html).not.toContain(">Mức ưu tiên nhiệm vụ</button>");
    expect(html).not.toContain(">Loại tài nguyên bản đồ</button>");
    expect(html).not.toContain(">Trạng thái nhiệm vụ</button>");
    expect(html).toContain(">Tất cả (1)</button>");
  });

  it("D22 — the add row still lists EVERY writable group, empty ones included", () => {
    const html = view(
      { loaiVanBan: [entry(TANG_DON_VI)] },
      { open: { kind: "add", idempotencyKey: "k" }, draft: { ...EMPTY_DRAFT, group: "loaiVanBan" } },
    );
    expect(count(html, "<option ")).toBe(7);
    expect(html).toContain(">Mức ưu tiên nhiệm vụ</option>");
  });

  it("D22 — a chosen group that has become empty falls back to 'Tất cả'", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI)] }, { shownGroup: "loaiTaiNguyenBanDo" });
    expect(html).toMatch(/aria-pressed="true"[^>]*>Tất cả \(1\)<\/button>/);
    expect(html).toContain(">cong-van<");
  });

  it("filtering narrows the table to the chosen group", () => {
    const html = view(
      { loaiVanBan: [entry(TANG_DON_VI)], loaiTaiNguyenBanDo: [entry(TANG_DON_VI, { id: "a1", code: "cot-dien" })] },
      { shownGroup: "loaiTaiNguyenBanDo" },
    );
    expect(html).toContain("cot-dien");
    expect(html).not.toContain(">cong-van<");
  });

  it("Nguồn reads 'Xã tự thêm' / 'Hệ thống'; status 'Đang dùng' / 'Ngừng dùng'; Thứ tự is the contract's", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI), entry(TANG_HE_THONG, { id: "b", active: false })] });
    expect(html).toContain('<span class="text-[12px]">Xã tự thêm</span>');
    expect(html).toContain('<span class="text-ink-muted text-[12px]">Hệ thống</span>');
    expect(html).toContain("Đang dùng");
    expect(html).toContain("Ngừng dùng");
    expect(html).toContain("<td>7</td>");
  });

  it("the explanatory notes are gone: no three-tier note, no 'disabled stay' note, no per-row reason", () => {
    // Regression (failed before this change): spec 05 has none of these sentences; the rule is still
    // enforced by which buttons are drawn (cases below).
    const html = view({
      loaiVanBan: [entry(TANG_HE_THONG), entry(TANG_RE_NHANH, { id: "c" })],
      mucUuTienNhiemVu: [entry(TANG_DON_VI, { id: "p" })],
    });
    for (const s of [GHI_CHU_BA_TANG, GIAI_THICH_DA_TAT, GIAI_THICH_THANG_BAC, GHI_CHU_NHOM_CHI_XEM]) {
      expect(html).not.toContain(s);
    }
    expect(html).not.toContain(giaiThichKhongThaoTac(TANG_HE_THONG));
    expect(html).not.toContain(giaiThichKhongThaoTac(TANG_RE_NHANH));
  });
});

describe("row buttons follow the tier (ADR 0024)", () => {
  it("tier 1 — commune's own: pencil, Tắt, bin", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI)] });
    expect(html).toContain(editLabel("Công văn"));
    expect(html).toContain('title="Sửa nhãn"');
    expect(html).toContain(">Tắt</button>");
    expect(html).toContain(deleteLabel("Công văn"));
    expect(html).toContain('title="Xoá mục"');
  });

  it("tier 2 — shipped with the software: Tắt, NO bin", () => {
    const html = view({ loaiVanBan: [entry(TANG_HE_THONG)] });
    expect(html).toContain(editLabel("Công văn"));
    expect(html).toContain(">Tắt</button>");
    expect(html).not.toContain(deleteLabel("Công văn"));
  });

  it("tier 3 — code branches on it: pencil only, NO Tắt, NO bin", () => {
    const html = view({ loaiVanBan: [entry(TANG_RE_NHANH)] });
    expect(html).toContain(editLabel("Công văn"));
    expect(html).not.toContain(">Tắt</button>");
    expect(html).not.toContain(deleteLabel("Công văn"));
  });

  it("a disabled entry offers 'Bật' at every tier, tier 3 included", () => {
    const html = view({ loaiVanBan: [entry(TANG_RE_NHANH, { active: false })] });
    expect(html).toContain(">Bật</button>");
    expect(html).not.toContain(deleteLabel("Công văn"));
  });

  it("DENIED — no `admin.lookup`: no Thêm mục, no pencil, no Tắt, no bin, no actions column; table still shows", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI)] }, { canWrite: false });
    expect(html).not.toContain("Thêm mục");
    expect(html).not.toContain(EDIT_LABEL);
    expect(html).not.toContain(">Tắt</button>");
    expect(html).not.toContain(DELETE_ENTRY);
    expect(html).not.toContain("Thao tác");
    expect(html).toContain("cong-van");
  });
});

describe("'Đặt mặc định' — Loại nhiệm vụ only", () => {
  it("an active, non-default task kind offers it, with the spec's title", () => {
    const html = view({ loaiNhiemVu: [entry(TANG_DON_VI, { code: "hanh-chinh", label: "Hành chính" })] });
    expect(html).toContain(`title="${SET_DEFAULT_TITLE}"`);
    expect(html).toContain(`>${SET_DEFAULT}</button>`);
  });

  it("not on the default one (which shows the 'Mặc định' badge), not when off, not in another group", () => {
    const html = view({
      loaiNhiemVu: [
        entry(TANG_DON_VI, { id: "d", label: "Hành chính", is_default: true }),
        entry(TANG_DON_VI, { id: "o", label: "Tạm", active: false }),
      ],
      loaiVanBan: [entry(TANG_DON_VI)],
    });
    expect(html).not.toContain(`>${SET_DEFAULT}</button>`);
    expect(html).toContain("bg-brand/12");
    expect(html).toContain(">Mặc định</span>");
  });
});

describe("in-place edit and delete", () => {
  const row = entry(TANG_DON_VI);
  const key = `loaiVanBan:${row.id}`;

  it("edit: the label becomes an input (h-8, 12.5px), the order a small box, 'Mặc định' a checkbox; Lưu / Huỷ", () => {
    const html = view(
      { loaiVanBan: [row] },
      { open: { kind: "edit", rowKey: key }, draft: { ...EMPTY_DRAFT, label: "Công văn", order: "7" } },
    );
    expect(html).toMatch(/<input class="[^"]*h-8[^"]*text-\[12\.5px\][^"]*" aria-label="Nhãn hiển thị — mục Công văn"[^>]*value="Công văn"/);
    expect(html).toContain('aria-label="Thứ tự — mục Công văn" inputMode="numeric" value="7"');
    expect(html).toContain('type="checkbox"');
    expect(html).toContain(">Lưu</button>");
    expect(html).toContain(">Huỷ</button>");
    expect(html).not.toContain(editLabel("Công văn"));
  });

  it("edit: an empty label is refused in place", () => {
    const html = view(
      { loaiVanBan: [row] },
      { open: { kind: "edit", rowKey: key }, localError: "Nhãn hiển thị không được để trống." },
    );
    expect(html).toContain('role="alert"');
    expect(html).toContain("Nhãn hiển thị không được để trống.");
    expect(html).toContain('aria-invalid="true"');
  });

  it("delete keeps the REASON step, inline in the row: 'Lý do xoá' box + Xoá (danger) + Huỷ", () => {
    const html = view({ loaiVanBan: [row] }, { open: { kind: "delete", rowKey: key } });
    expect(html).toContain('placeholder="Lý do xoá"');
    expect(html).toContain("nut-xoa");
    expect(html).toContain(">Xoá</button>");
    expect(html).toContain(">Huỷ</button>");
    // The code stays reserved — said on the box, for hover and for a screen reader.
    expect(html).toContain("không dùng lại được");
  });

  it("delete: an empty reason is refused in place, the box marked invalid", () => {
    const html = view(
      { loaiVanBan: [row] },
      { open: { kind: "delete", rowKey: key }, localError: "Vui lòng nhập lý do xoá." },
    );
    expect(html).toContain("Vui lòng nhập lý do xoá.");
    expect(html).toContain('role="alert"');
    expect(html).toContain('aria-invalid="true"');
  });
});

describe("the add row (spec 05 §2)", () => {
  function addRow(p: Partial<Props> = {}) {
    return view(
      {},
      { open: { kind: "add", idempotencyKey: "k" }, draft: { ...EMPTY_DRAFT, group: "loaiVanBan" }, ...p },
    );
  }

  it("group select of the writable groups · label · disabled 'Màu' with its '?' · Thêm / Huỷ — no 'Mã' field", () => {
    const html = addRow();
    expect(html).toContain("sm:grid-cols-[16rem_1fr_6rem_auto]");
    expect(html).toContain(">Nhóm danh mục</label>");
    expect(count(html, "<option ")).toBe(7);
    expect(html).toContain('placeholder="Ví dụ: Chợ và thương mại"');
    expect(html).toMatch(/type="color" disabled=""/);
    expect(html).toContain(pendingMarkerLabel("Màu của mục danh mục"));
    expect(html).toContain(">Thêm</button>");
    expect(html).toContain(">Huỷ</button>");
    expect(html).not.toContain('name="code"');
    expect(html).not.toContain(">Mã</label>");
  });

  it("a refusal (e.g. the derived code is taken) shows the server's sentence in place", () => {
    const html = addRow({ serverError: "Mã cong-van đã có trong danh mục." });
    expect(html).toContain("Mã cong-van đã có trong danh mục.");
    expect(html).toContain('role="alert"');
  });

  it("no add row without the write key, even if one is marked open", () => {
    expect(addRow({ canWrite: false })).not.toContain("Ví dụ: Chợ và thương mại");
  });

  it("the success sentence is the spec's", () => {
    expect(ADDED).toBe("Đã thêm mục mới vào danh mục.");
  });
});

describe("validate round 1 (owner 08/10/2026: 'Theo prototype')", () => {
  it("D16 — an empty table draws NO empty row and NO sentence, for 'Tất cả' and for a filtered group", () => {
    for (const shownGroup of [null, "loaiVanBan"] as const) {
      const html = view({}, { shownGroup });
      expect(html).not.toContain("py-10");
      expect(html).not.toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
      expect(html).not.toContain("chưa có mục");
      expect(html).toContain("<tbody></tbody>");
    }
  });

  it("D17 — while loading: no filter row, no 'Thêm mục'", () => {
    const html = view({}, { groups: null });
    expect(html).not.toContain("Tất cả");
    expect(html).not.toContain("Thêm mục");
    expect(html).not.toContain("Lọc theo nhóm danh mục");
  });

  it("D7 — 'Mặc định' is a text-only pill (no tone icon), brand classes", () => {
    const html = view({ loaiNhiemVu: [entry(TANG_DON_VI, { label: "Hành chính", is_default: true })] });
    const pill = html.match(/<span class="bg-brand\/12 text-brand border-brand\/25[^"]*">([^<]*)<\/span>/);
    expect(pill?.[1]).toBe("Mặc định");
    expect(html).not.toMatch(/<svg[^>]*>(?:(?!<\/span>).)*Mặc định/);
  });

  it("D21 — the Plus of 'Thêm mục' is size-4", () => {
    expect(view({})).toMatch(/<svg[^>]*class="[^"]*size-4[^"]*"[^>]*>(?:(?!<button).)*Thêm mục<\/button>/);
  });

  it("D15 — an empty `source` reads '—', never a blank cell", () => {
    const html = view({ loaiVanBan: [entry(TANG_DON_VI, { source: "" })] });
    expect(html).toContain('<span class="text-[12px]">—</span>');
  });

  it("D4 — the top 'Nhập từ Excel' is ALWAYS drawn for `admin.lookup`: working for a group, '?' otherwise", () => {
    const common = renderToStaticMarkup(<>{catalogueImportButton(true, null, () => {})}</>);
    // D4b (failed before): the "?" row is pinned to the 28px of the working sm button, and the disabled
    // button loses the legacy 32px min-height — the tab no longer jumps ~5px when the filter changes.
    expect(common).toContain('class="mb-3 flex h-7 items-center justify-end overflow-visible"');
    expect(common).toContain("[&amp;_button:not([data-pending-marker])]:h-7");
    expect(common).toContain("[&amp;_button:not([data-pending-marker])]:min-h-0");
    expect(common).toContain("Nhập từ Excel");
    expect(common).toMatch(/disabled=""/);
    expect(common).toContain(pendingMarkerLabel("Nhập Excel chung cho mọi nhóm danh mục"));

    const group = renderToStaticMarkup(<>{catalogueImportButton(true, "loaiVanBan", () => {})}</>);
    expect(group).toContain("Nhập từ Excel</button>");
    expect(group).not.toContain('disabled=""');
    expect(group).not.toContain("data-pending-marker");

    // DENIED: without the key, no button at all — neither the working one nor the "?".
    expect(renderToStaticMarkup(<>{catalogueImportButton(false, null, () => {})}</>)).toBe("");
    expect(renderToStaticMarkup(<>{catalogueImportButton(false, "loaiVanBan", () => {})}</>)).toBe("");
  });
});

describe("failed, loading", () => {

  it("a group that cannot be read: the server's sentence, named, as an alert — the others still render", () => {
    const html = view({ loaiVanBan: "Đã xảy ra lỗi. Vui lòng thử lại.", khoiNhiemVu: [entry(TANG_DON_VI, { code: "khoi-1" })] });
    expect(html).toContain("Loại văn bản: Đã xảy ra lỗi. Vui lòng thử lại.");
    expect(html).toContain('role="alert"');
    expect(html).toContain("khoi-1");
    expect(html).not.toContain(nhanNhomRong("Loại văn bản", "themDuoc"));
  });

  it("loading: three placeholder bars and a status line, no table", () => {
    const html = view({}, { groups: null });
    expect(html).toContain("Đang tải danh mục của đơn vị…");
    expect(count(html, "h-11 w-full")).toBe(3);
    expect(html).not.toContain("<table");
  });
});
