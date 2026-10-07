import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import { EMPTY_SELECTION, toggleSelected } from "./batch-delete";
import { BANG_NHAN_MAC_DINH, TRANG_THAI_CHINH, kanbanSharedError } from "./nhan-nhiem-vu";
import {
  BangKanban,
  BangSoTheoDoi,
  type CotKanban, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type DanhMucNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./so-nhiem-vu";
import {
  APPROVAL_TICKED,
  REGISTER_COLUMNS,
  REGISTER_DOCS_MISSING,
  REGISTER_UNKNOWN_GROUP,
  SUPERIOR_NOT_YET,
  registerRowDocuments,
} from "./task-register";

/**
 * Sổ theo dõi — spec 05 (W6, redrawn 07/10/2026). No DOM: what the table RENDERS for a row read with
 * `include=documents`, the thirteen columns in the prototype's order, and — by source — the page wiring.
 */

/** Spec 05 `Cột`, verbatim (the owner's spec files are outside git, so the list is written here). */
const SPEC_COLUMNS = [
  "Mã",
  "Nội dung nhiệm vụ / Trích yếu văn bản",
  "Cơ quan chủ trì tham mưu",
  "Chuyên viên VP tham mưu / theo dõi",
  "Đơn vị thực hiện",
  "Văn bản cấp trên giao",
  "Văn bản chỉ đạo của Đảng uỷ",
  "Hạn hoàn thành",
  "Trạng thái",
  "Kết quả thực hiện / Sản phẩm đầu ra",
  "Lãnh đạo phê duyệt",
  "Ghi chú",
];

const ROW: petitions_nhiemVuRa = {
  code: "NV33",
  child_count: 0,
  allowed_transitions: [],
  updated_at: "2026-06-01T02:00:00Z",
  type: "theo-van-ban",
  bloc: "khoi-uy-ban",
  priority: "",
  title: "Triển khai thông báo kết luận của Thành uỷ",
  description: "Mô tả giả",
  status: "dang-thuc-hien",
  source: "truc-tiep",
  source_id: "",
  unit: "01JUNIT",
  assignee: "CB-2026-THUCHIEN",
  assigner: "",
  due_at: "2026-12-20T23:59:59+07:00",
  original_due_at: "2026-12-20T23:59:59+07:00",
  completed_at: null,
  progress: 0,
  result_summary: "Đã báo cáo giả",
  note: "",
  leader_approved: true,
  superior_acknowledged: false,
  parent: "",
  created_by: "CB-2026-VANTHU",
  created_at: "2026-06-01T02:00:00Z",
  documents: [
    { id: "d1", group: "cap-tren-giao", reference: "90-TB/TU", date: "2026-11-30", summary: "Thông báo giả", position: 1 },
    { id: "d2", group: "san-pham-dau-ra", reference: "", date: "", summary: "Báo cáo giả", position: 1 },
  ],
};

const CATALOGUES: DanhMucNhiemVu = {
  loai: [],
  mucUuTien: [],
  khoi: [{ id: "k", code: "khoi-uy-ban", label: "Khối Uỷ ban", is_default: false, active: true, order: 1, source: "he-thong", tier: 1 }],
  boPhan: [],
};
const UNITS = new Map([
  ["01JUNIT", "BỘ PHẬN GIẢ THỰC HIỆN"],
]);
const DIRECTORY = danhBaTheoMa([
  { code: "CB-2026-THUCHIEN", full_name: "Nguyễn Văn Giả", position: "", department_id: "" },
]);

function table(rows: readonly petitions_nhiemVuRa[], withSelection = false): string {
  return renderToStaticMarkup(
    <BangSoTheoDoi
      nhiemVu={rows}
      danhMuc={CATALOGUES}
      danhBa={DIRECTORY}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={UNITS}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maDangMo={null}
      moNhiemVu={() => {}}
      selection={
        withSelection
          ? { selected: toggleSelected(EMPTY_SELECTION, ROW), toggle: () => {}, disabled: false }
          : null
      }
    />,
  );
}

describe("spec 05 columns — the prototype's thirteen, in order", () => {
  it("the twelve data columns after `☐`, verbatim and in order, with the prototype's widths", () => {
    expect(REGISTER_COLUMNS.map((c) => c.label)).toEqual(SPEC_COLUMNS);
    expect(REGISTER_COLUMNS.map((c) => c.width)).toEqual([
      "w-16", "w-96", "w-40", "w-36", "w-40", "w-64", "w-64", "w-28", "w-28", "w-64", "w-28 text-center", "w-56",
    ]);
  });

  it("the table draws those headers in that order, fixed 1700px; `☐` only with `task.delete` (13 with it)", () => {
    const html = table([ROW]);
    const heads = [...html.matchAll(/<th scope="col" class="[^"]*">([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(heads).toEqual(SPEC_COLUMNS);
    expect(html).toContain("min-w-[1700px] table-fixed");
    expect(html).not.toContain('type="checkbox"');
    const row = html.split("<tbody>")[1] ?? "";
    expect(row.match(/<td[ >]/g)?.length).toBe(12);
    const withBox = table([ROW], true);
    expect(withBox).toMatch(/aria-label="Chọn NV33" checked=""/);
    expect((withBox.split("<tbody>")[1] ?? "").match(/<td[ >]/g)?.length).toBe(13);
  });

  it("`Cơ quan chủ trì` / `Chuyên viên VP` show the SAME data as `Đơn vị thực hiện` (owner 07/10/2026 #8)", () => {
    const cells = [...(table([ROW]).split("<tbody>")[1] ?? "").matchAll(/<td[^>]*>([\s\S]*?)<\/td>/g)].map((m) => m[1] ?? "");
    expect(cells[2]).toBe("BỘ PHẬN GIẢ THỰC HIỆN");
    expect(cells[3]).toBe("Nguyễn Văn Giả");
    expect(cells[4]).toContain("BỘ PHẬN GIẢ THỰC HIỆN");
    expect(cells[4]).toContain("Nguyễn Văn Giả");
  });
});

describe("a row — names resolved, documents split, nothing blank that is not empty", () => {
  it("title + description (3 lines) + bloc label; unit + assignee NAMES", () => {
    const html = table([ROW]);
    expect(html).toContain('<div class="text-navy font-semibold">Triển khai thông báo kết luận của Thành uỷ</div>');
    expect(html).toContain('<div class="text-ink-muted mt-0.5 line-clamp-3 text-[11.5px]">Mô tả giả</div>');
    expect(html).toContain('<div class="text-ink-muted mt-1 text-[11px]">Khối Uỷ ban</div>');
    expect(html).toContain("BỘ PHẬN GIẢ THỰC HIỆN");
    expect(html).toContain("Nguyễn Văn Giả");
  });

  it("documents (spec 05 `RefList`): number bold, ` · date`, summary on 2 lines; `Không số`; `—` for none", () => {
    const html = table([ROW]);
    expect(html).toContain('<span class="text-navy font-semibold">90-TB/TU</span><span class="text-ink-muted"> · 30/11/2026</span>');
    expect(html).toContain('<div class="text-ink-muted line-clamp-2 text-[11.5px]">Thông báo giả</div>');
    expect(html).toContain('<span class="text-navy font-semibold">Không số</span>');
    const d = registerRowDocuments(ROW);
    expect(d?.byGroup["chi-dao-dang-uy"]).toEqual([]);
    expect(d?.unknown).toBe(0);
  });

  it("a row WITHOUT the block says so once — never three `—` that read as \"no documents\"", () => {
    const { documents: _omit, ...bare } = ROW;
    void _omit;
    expect(registerRowDocuments({})).toBeNull();
    const html = table([bare]);
    expect(html.split(REGISTER_DOCS_MISSING).length - 1).toBe(1);
  });

  it("a document in an unknown group is not dropped silently", () => {
    const odd = { ...ROW, documents: [{ id: "x", group: "nhom-moi", reference: "", date: "", summary: "", position: 1 }] };
    expect(registerRowDocuments(odd)?.unknown).toBe(1);
    expect(table([odd])).toContain(REGISTER_UNKNOWN_GROUP);
  });

  it("deadline, the status badge, result + output, ONE approval column (tick / `Cấp trên chưa duyệt`), note `—`", () => {
    const html = table([ROW]);
    expect(html).toContain("20/12/2026");
    expect(html).toMatch(/data-status="dang-thuc-hien" data-tone="status">[\s\S]*?Đang thực hiện<\/span>/);
    expect(html).toContain('<div class="mb-1">Đã báo cáo giả</div>');
    expect(html).toMatch(/lucide-check text-leaf mx-auto block size-4/);
    expect(html).toContain(`<span class="an-thi-giac">${APPROVAL_TICKED}</span>`);
    expect(html).toContain(`>${SUPERIOR_NOT_YET}</div>`);
    const none = table([{ ...ROW, leader_approved: false }]);
    expect(none).toMatch(/lucide-minus text-ink-muted mx-auto block size-4/);
    expect(none).not.toContain(SUPERIOR_NOT_YET);
  });

  it("an overdue row: pink tint, the deadline cell red with `trễ N ngày` under the date", () => {
    const html = renderToStaticMarkup(
      <BangSoTheoDoi
        nhiemVu={[{ ...ROW, due_at: "2026-09-10T10:00:00+07:00" }]}
        danhMuc={CATALOGUES}
        danhBa={DIRECTORY}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={UNITS}
        bayGio={new Date("2026-09-15T03:00:00Z")}
        maDangMo={null}
        moNhiemVu={() => {}}
      />,
    );
    expect(html).toMatch(/<tr data-tre-han="" class="[^"]*bg-danger\/6/);
    expect(html).toMatch(/text-danger font-semibold">10\/9\/2026<div class="text-\[11px\]">trễ 5 ngày<\/div>/);
  });
});

describe("Kanban — one refusal said ONCE (W6, coordinator decision)", () => {
  const REFUSAL = "Xã chưa cấu hình ngưỡng sắp đến hạn cho nhiệm vụ…";
  const failed = (msg: string) => ({ pha: "loi" as const, thongBao: msg });

  it("`kanbanSharedError`: all columns refused with ONE sentence ⇒ that sentence; otherwise `null`", () => {
    expect(kanbanSharedError([{ tai: failed("a") }, { tai: failed("a") }])).toBe("a");
    expect(kanbanSharedError([{ tai: failed("a") }, { tai: failed("b") }])).toBeNull();
    expect(kanbanSharedError([{ tai: failed("a") }, { tai: { pha: "dangTai" } }])).toBeNull();
  });

  it("the board draws it once above, not per column, and not again in the counts line", () => {
    const cot: CotKanban[] = TRANG_THAI_CHINH.map((ma) => ({ ma, tai: failed(REFUSAL) }));
    const html = renderToStaticMarkup(
      <BangKanban
        cot={cot}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        bayGio={new Date("2026-09-15T03:00:00Z")}
        maDangMo={null}
        moNhiemVu={() => {}}
        counts={failed(REFUSAL)}
      />,
    );
    expect(html.split(REFUSAL).length - 1).toBe(1);
  });

  it("different refusals per column stay per column", () => {
    const cot: CotKanban[] = TRANG_THAI_CHINH.map((ma, i) => ({ ma, tai: failed(`lỗi ${i}`) }));
    const html = renderToStaticMarkup(
      <BangKanban
        cot={cot}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        bayGio={new Date("2026-09-15T03:00:00Z")}
        maDangMo={null}
        moNhiemVu={() => {}}
        counts={{ pha: "dangTai" }}
      />,
    );
    for (let i = 0; i < TRANG_THAI_CHINH.length; i++) expect(html).toContain(`lỗi ${i}`);
  });
});

describe("page wiring (source)", () => {
  const SRC = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("third view button; register view reads the page with `include=documents` under its own key", () => {
    expect(SRC).toContain('["so-theo-doi", REGISTER_VIEW_LABEL, ClipboardList],');
    expect(SRC).toContain("onClick={() => onChange(v)}");
    expect(SRC).toContain('includeDocuments: viewMode === "so-theo-doi",');
    expect(SRC).toContain('|${viewMode === "so-theo-doi" ? "docs" : ""}');
  });

  it("the book's query forces the `Theo văn bản` type (spec 02 §State) — rows, counts AND the export", () => {
    expect(SRC).toContain('() => (viewMode === "so-theo-doi" ? { ...loc, loai: LOAI_THEO_VAN_BAN } : loc),');
    expect(SRC).toContain("getTaskCounts(viewLoc)");
    expect(SRC).toContain("hideType={viewMode === \"so-theo-doi\"}");
  });

  it("export (kept, owner 07/10/2026 #5): the SAME query and sort as the screen; refusal verbatim as a toast", () => {
    expect(SRC).toContain("downloadTaskRegister({ ...viewLoc, sapXep: sx.cot, chieu: sx.chieu })");
    expect(SRC).toContain("toast.error(`${REGISTER_EXPORT_REFUSED} ${r.thongBao}`);");
    expect(SRC).toContain("saveFile(r.duLieu.blob, r.duLieu.fileName);");
  });
});
