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
  REGISTER_COLUMNS,
  REGISTER_DOCS_MISSING,
  REGISTER_UNKNOWN_GROUP,
  registerRowDocuments,
} from "./task-register";

/**
 * Sổ theo dõi §4.3 (W6). No DOM: what the table RENDERS for a row read with `include=documents`,
 * the column order against the spec's own lines, and — by source — the page wiring.
 */

const SPEC = readFileSync(fileURLToPath(new URL("../../../../docs/ui-ux/02-nhiem-vu.md", import.meta.url)), "utf8");

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

describe("§4.3 columns — exactly the spec's order", () => {
  it("every column named in `02-nhiem-vu.md` §4.3 appears, in that order", () => {
    const section = SPEC.slice(SPEC.indexOf("### 4.3 Sổ theo dõi"), SPEC.indexOf("## 5. Chi tiết"));
    for (const c of REGISTER_COLUMNS) {
      // The spec abbreviates two: "Tóm tắt kết quả", and the two approval ticks as "các ô tick phê duyệt".
      if (c.startsWith("Lãnh đạo xã") || c.startsWith("Cấp trên")) continue;
      expect(section).toContain(c);
    }
    const pos = REGISTER_COLUMNS.slice(0, -2).map((c) => section.indexOf(c));
    expect([...pos].sort((a, b) => a - b)).toEqual(pos);
  });

  it("ELEVEN columns — no `Cơ quan chủ trì` / `Chuyên viên … theo dõi` (ADR 0065 NV5, same as the export)", () => {
    expect(REGISTER_COLUMNS).toHaveLength(11);
    expect(REGISTER_COLUMNS.some((c) => c.includes("chủ trì") || c.includes("theo dõi"))).toBe(false);
    // Every cell of a row is one column: 11 `<td>` per row without the `☐` column.
    const row = table([ROW]).split("<tbody>")[1] ?? "";
    expect(row.match(/<td[ >]/g)?.length).toBe(11);
  });

  it("the table draws those headers in that order; `☐` only with `task.delete`", () => {
    const html = table([ROW]);
    const heads = [...html.matchAll(/<th scope="col">([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(heads).toEqual(REGISTER_COLUMNS);
    expect(html).not.toContain('type="checkbox"');
    expect(table([ROW], true)).toMatch(/aria-label="Chọn NV33" checked=""/);
  });
});

describe("a row — names resolved, documents split, nothing blank that is not empty", () => {
  it("title + description + bloc label; unit + assignee NAMES", () => {
    const html = table([ROW]);
    expect(html).toContain("Triển khai thông báo kết luận của Thành uỷ");
    expect(html).toContain('<span class="dong-phu">Mô tả giả</span>');
    expect(html).toContain('<span class="chip chip-ngung">Khối Uỷ ban</span>');
    expect(html).toContain("BỘ PHẬN GIẢ THỰC HIỆN");
    expect(html).toContain("Nguyễn Văn Giả");
  });

  it("documents: `90-TB/TU · 30/11/2026` + summary; `Không số` when unnumbered; `—` for an empty group", () => {
    const html = table([ROW]);
    expect(html).toContain("90-TB/TU · 30/11/2026");
    expect(html).toContain('<span class="dong-phu">Thông báo giả</span>');
    expect(html).toContain("Không số");
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

  it("deadline with the late part, result, note `—`, and the two approval ticks as words", () => {
    const html = table([ROW]);
    expect(html).toContain("20/12/2026");
    expect(html).toContain("<td>Đã báo cáo giả</td>");
    expect(html).toContain("<td>Đã đánh dấu</td><td>Chưa đánh dấu</td>");
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
    expect(SRC).toContain('onClick={() => datCheDoXem("so-theo-doi")}');
    expect(SRC).toContain('includeDocuments: viewMode === "so-theo-doi",');
    expect(SRC).toContain('|${viewMode === "so-theo-doi" ? "docs" : ""}');
  });

  it("export: the SAME filters and sort as the screen; refusal verbatim; saved without navigating", () => {
    expect(SRC).toContain("downloadTaskRegister({ ...loc, sapXep: sx.cot, chieu: sx.chieu })");
    expect(SRC).toContain("{REGISTER_EXPORT_REFUSED} {exportResult.thongBao}");
    expect(SRC).toContain("saveFile(r.duLieu.blob, r.duLieu.fileName);");
  });
});
