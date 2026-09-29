import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { commitImport, previewImport } from "@/lib/api/excel-import";
import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type { MapAssetTypeImportRow } from "@/lib/api/map-asset-type-import";
import type { ResidentialUnitImportRow } from "@/lib/api/residential-unit-import";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type, imported not declared

import { keyAfterAttempt, keyForAttempt } from "./excel-import-flow";
import { ExcelImportView } from "./excel-import-panel";
import {
  CATALOGUE_IMPORTS,
  DOCUMENT_TYPE_IMPORT_TARGET,
  MAP_ASSET_TYPE_IMPORT_TARGET,
  RESIDENTIAL_UNIT_IMPORT_TARGET,
  RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET,
  TASK_BLOC_IMPORT_TARGET,
  catalogueImportFor,
} from "./excel-import-targets";
import { NhomMuc } from "./tab-danh-muc"; // vi-name-ok: existing export of tab-danh-muc.tsx, imported not declared (rule 12 inv 3)

/**
 * The shared import panel, bound to the Danh mục group `Loại tài nguyên bản đồ`; its gate in the
 * Danh mục tab (allowed AND denied); and the one key per attempt.
 */

const T = MAP_ASSET_TYPE_IMPORT_TARGET;

function view(
  preview: KetQua<ImportPreview<MapAssetTypeImportRow>> | null,
  result: ImportResult<MapAssetTypeImportRow> | null = null,
) {
  return renderToStaticMarkup(
    <ExcelImportView
      target={T}
      fileChosen
      preview={preview}
      result={result}
      busy=""
      templateError=""
      onDownloadTemplate={() => {}}
      onChooseFile={() => {}}
      onPreview={() => {}}
      onImport={() => {}}
      onClose={() => {}}
    />,
  );
}

const ERRORS = [
  { row: 3, column: "Tên hiển thị", message: "Tên này trùng với loại đang có trong xã." },
  { row: 0, column: "", message: "Tệp thiếu cột Mã." },
];

describe("Loại tài nguyên bản đồ — the import panel", () => {
  it("title, template button, file input with its own id", () => {
    const html = view(null);
    expect(html).toContain("Nhập loại tài nguyên bản đồ từ Excel");
    expect(html).toContain(">Tải tệp mẫu</button>");
    expect(html).toContain('id="o-tep-nhap-loai-tai-nguyen"');
    expect(html).toContain('accept=".xlsx,');
  });

  it("valid preview → the types to create, codes monospaced, then the import button", () => {
    const html = view({
      ok: true,
      duLieu: { valid: true, errors: [], rows: [{ row: 2, code: "ho-kinh-doanh", label: "Hộ kinh doanh", order: 1 }] },
    });
    expect(html).toContain("Sẽ tạo 1 loại tài nguyên bản đồ");
    expect(html).toContain(
      '<th scope="col">Dòng</th><th scope="col">Tên hiển thị</th><th scope="col">Mã</th><th scope="col">Thứ tự</th>',
    );
    expect(html).toContain('<td>2</td><td>Hộ kinh doanh</td><td class="ma-muc">ho-kinh-doanh</td><td>1</td>');
    expect(html).toContain(">Nhập các loại này</button>");
  });

  it("invalid preview → row · column · message table, the heading names what was NOT created, no import", () => {
    const html = view({ ok: true, duLieu: { valid: false, rows: [], errors: ERRORS } });
    expect(html).toContain("chưa loại tài nguyên nào được tạo");
    expect(html).toContain("<td>3</td><td>Tên hiển thị</td><td>Tên này trùng với loại đang có trong xã.</td>");
    expect(html).toContain("<td>Cả tệp</td><td>—</td><td>Tệp thiếu cột Mã.</td>");
    expect(html).not.toContain("Nhập các loại này");
  });

  it("413 / 415 on the preview → one sentence", () => {
    const html = view({ ok: false, thongBao: "Chỉ nhận tệp Excel .xlsx." });
    expect(html).toContain('role="alert">Chỉ nhận tệp Excel .xlsx.</p>');
  });

  it("400 import_invalid on the import itself → its message and the same errors table", () => {
    const html = view(
      { ok: true, duLieu: { valid: true, errors: [], rows: [] } },
      { ok: false, message: "Tệp có lỗi nên chưa loại nào được tạo.", errors: ERRORS },
    );
    expect(html).toContain("Tệp có lỗi nên chưa loại nào được tạo.");
    expect(html).toContain("<td>Cả tệp</td>");
  });

  it("success, and a replayed success, both say the catalogue was re-read", () => {
    expect(view(null, { ok: true, created: [{ row: 2, code: "a", label: "A", order: 0, id: "01J" }] })).toContain(
      "Đã nhập 1 loại tài nguyên bản đồ. Danh mục đã được tải lại.",
    );
    expect(view(null, { ok: true, created: null })).toContain("đã được nhập ở lần gửi trước");
  });

  it("NO RAW HTML: a label with markup is text", () => {
    const html = view({
      ok: true,
      duLieu: { valid: true, errors: [], rows: [{ row: 2, code: "x", label: "<script>alert(1)</script>", order: 0 }] },
    });
    expect(html).toContain("&lt;script&gt;");
    expect(html).not.toContain("<script>");
  });
});

describe("one Idempotency-Key per attempt", () => {
  it("minted once, KEPT after a failed send (the retry reuses it), dropped after success", () => {
    const mint = vi.fn(() => "k-1");
    const first = keyForAttempt(null, mint);
    const afterFailure = keyAfterAttempt(first, false);
    const retry = keyForAttempt(afterFailure, mint);
    expect(retry).toBe("k-1");
    expect(mint).toHaveBeenCalledTimes(1);
    expect(keyAfterAttempt(retry, true)).toBeNull();
    // A new attempt after success mints a new key.
    expect(keyForAttempt(keyAfterAttempt(retry, true), () => "k-2")).toBe("k-2");
  });
});

function session(permissions: readonly string[]): KetQua<identity_phienHienTaiRa> {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: [...permissions],
    },
  } as KetQua<identity_phienHienTaiRa>;
}

describe("Danh mục — which group offers the import, to whom", () => {
  it("today: four groups have an import (one route per owning service, ADR 0059 §3); the rest do not", () => {
    expect(Object.keys(CATALOGUE_IMPORTS)).toEqual(["loaiTaiNguyenBanDo", "loaiVanBan", "loaiDonViDanCu", "khoiNhiemVu"]);
    // Being built by their owners (petitions, finance) — no button until their routes are in the contract.
    for (const g of ["loaiNhiemVu", "mucUuTienNhiemVu", "hangMucKeHoachVon"] as const) {
      expect(catalogueImportFor(g, session(["admin.lookup"]))).toBeNull();
    }
  });

  it("allowed: `admin.lookup` on the group that has an import", () => {
    expect(catalogueImportFor("loaiTaiNguyenBanDo", session(["admin.lookup"]))).not.toBeNull();
  });

  it("DENIED: no `admin.lookup`, a look-alike key, an unread or failed session", () => {
    expect(catalogueImportFor("loaiTaiNguyenBanDo", session(["admin.org", "admin.lookups", "asset.read"]))).toBeNull();
    expect(catalogueImportFor("loaiTaiNguyenBanDo", null)).toBeNull();
    expect(catalogueImportFor("loaiTaiNguyenBanDo", { ok: false, thongBao: "Phiên đã hết hạn" })).toBeNull();
    expect(catalogueImportFor(null, session(["admin.lookup"]))).toBeNull();
  });

  const noActions = { them: () => {}, sua: () => {}, xoa: () => {}, datTrangThai: () => {} };
  const group = {
    khoa: "loaiTaiNguyenBanDo" as const,
    nhan: "Loại tài nguyên bản đồ",
    thuTuLaThangBac: false,
    ghi: null,
    trangThai: { pha: "chuaCoMuc" as const },
  };

  it("the group draws '⬆ Nhập từ Excel' only when told it may, and the panel under itself", () => {
    const shown = renderToStaticMarkup(
      <NhomMuc
        nhom={group}
        coQuyenGhi={false}
        thaoTac={noActions}
        form={null}
        importAllowed
        importPanel={<p>PANEL</p>}
      />,
    );
    expect(shown).toContain(">⬆ Nhập từ Excel</button>");
    expect(shown).toContain("<p>PANEL</p>");
    const denied = renderToStaticMarkup(<NhomMuc nhom={group} coQuyenGhi={false} thaoTac={noActions} form={null} />);
    expect(denied).not.toContain("Nhập từ Excel");
  });
});

describe("Thôn / Tổ dân phố — its own import target (not a catalogue group)", () => {
  const R = RESIDENTIAL_UNIT_IMPORT_TARGET;
  const row = {
    row: 2,
    code: "thon-binh-an",
    name: "Thôn Bình An",
    type_code: "thon",
    type_label: "Thôn",
    head_staff_code: "",
    head_staff_name: "",
    household_count: null,
    population_count: 1132,
    order: 1,
  };

  function residentialView(
    preview: KetQua<ImportPreview<ResidentialUnitImportRow>> | null,
    result: ImportResult<ResidentialUnitImportRow> | null = null,
  ) {
    return renderToStaticMarkup(
      <ExcelImportView
        target={R}
        fileChosen
        preview={preview}
        result={result}
        busy=""
        templateError=""
        onDownloadTemplate={() => {}}
        onChooseFile={() => {}}
        onPreview={() => {}}
        onImport={() => {}}
        onClose={() => {}}
      />,
    );
  }

  it("is NOT in CATALOGUE_IMPORTS, and reads its three identity routes", () => {
    expect(Object.keys(CATALOGUE_IMPORTS)).not.toContain("thonToDanPho");
    expect(R.routes).toEqual({
      template: "/api/v1/residential-units/import-template",
      previews: "/api/v1/residential-units/import-previews",
      imports: "/api/v1/residential-units/imports",
    });
    expect(R.rowsField).toBe("units");
  });

  it("round: preview reads `units`, shows a blank count as 'Chưa nhập' (never 0), then imports with ONE key", async () => {
    const calls: [string, RequestInit][] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (path: string, init: RequestInit) => {
        calls.push([path, init]);
        if (path.endsWith("/import-previews")) {
          return new Response(JSON.stringify({ valid: true, units: [row], errors: [] }), { status: 200 });
        }
        return new Response(JSON.stringify({ created: [{ ...row, id: "01JNEW" }] }), { status: 201 });
      }),
    );
    try {
      const file = new Blob(["x"]);
      const p = await previewImport<ResidentialUnitImportRow>(R.routes, R.rowsField, file, "thon.xlsx");
      expect(p.ok && p.duLieu.rows).toEqual([row]);
      const html = residentialView(p);
      expect(html).toContain("Sẽ tạo 1 thôn / tổ dân phố");
      expect(html).toContain(
        '<td>2</td><td>Thôn Bình An</td><td class="ma-muc">thon-binh-an</td><td>Thôn</td><td>Chưa có</td><td>Chưa nhập</td><td>1.132</td><td>1</td>',
      );
      expect(html).toContain(">Nhập các địa bàn này</button>");

      const r = await commitImport<ResidentialUnitImportRow>(R.routes, file, "thon.xlsx", "k-1");
      expect(r).toEqual({ ok: true, created: [{ ...row, id: "01JNEW" }] });
      expect(calls[1]![0]).toBe("/api/v1/residential-units/imports");
      expect(new Headers(calls[1]![1].headers).get("Idempotency-Key")).toBe("k-1");
      expect(residentialView(null, r)).toContain("Đã nhập 1 thôn / tổ dân phố. Danh sách đã được tải lại.");
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("409 residential_units_changed: the server's sentence, nothing written", async () => {
    const msg = "Danh sách thôn / tổ dân phố đã thay đổi từ lúc kiểm tra tệp. Hãy kiểm tra lại tệp rồi nhập.";
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ code: "residential_units_changed", message: msg, trace_id: "t" }), { status: 409 })),
    );
    try {
      const r = await commitImport<ResidentialUnitImportRow>(R.routes, new Blob(["x"]), "thon.xlsx", "k-1");
      expect(r).toEqual({ ok: false, message: msg, errors: [] });
      expect(residentialView({ ok: true, duLieu: { valid: true, rows: [row], errors: [] } }, r)).toContain(msg);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("invalid preview: the heading names what was NOT created", () => {
    const html = residentialView({ ok: true, duLieu: { valid: false, rows: [], errors: ERRORS } });
    expect(html).toContain("chưa thôn / tổ dân phố nào được tạo");
    expect(html).not.toContain("Nhập các địa bàn này");
  });
});

describe("Loại văn bản · Loại đơn vị dân cư · Khối nhiệm vụ — registered in CATALOGUE_IMPORTS", () => {
  const cases = [
    {
      group: "loaiVanBan" as const,
      target: DOCUMENT_TYPE_IMPORT_TARGET,
      base: "/api/v1/document-types",
      field: "types",
      noun: "loại văn bản",
    },
    {
      group: "loaiDonViDanCu" as const,
      target: RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET,
      base: "/api/v1/residential-unit-types",
      field: "entries",
      noun: "loại đơn vị dân cư",
    },
    {
      group: "khoiNhiemVu" as const,
      target: TASK_BLOC_IMPORT_TARGET,
      base: "/api/v1/task-blocs",
      field: "entries",
      noun: "khối nhiệm vụ",
    },
  ];

  for (const c of cases) {
    it(`${c.noun}: its owner's three routes, rows in \`${c.field}\`, gated on admin.lookup (allowed and DENIED)`, () => {
      expect(c.target.routes).toEqual({
        template: `${c.base}/import-template`,
        previews: `${c.base}/import-previews`,
        imports: `${c.base}/imports`,
      });
      expect(c.target.rowsField).toBe(c.field);
      expect(CATALOGUE_IMPORTS[c.group]?.permission).toBe("admin.lookup");
      expect(catalogueImportFor(c.group, session(["admin.lookup"]))).not.toBeNull();
      expect(catalogueImportFor(c.group, session(["admin.org", "admin.user"]))).toBeNull();
      expect(catalogueImportFor(c.group, null)).toBeNull();
    });

    it(`${c.noun}: preview reads \`${c.field}\` and draws the four columns; success says the catalogue was re-read`, async () => {
      const row = { row: 2, code: "ma-thu", label: "Mục thử", order: 1 };
      vi.stubGlobal(
        "fetch",
        vi.fn(async (path: string) =>
          path.endsWith("/import-previews")
            ? new Response(JSON.stringify({ valid: true, [c.field]: [row], errors: [] }), { status: 200 })
            : new Response(JSON.stringify({ created: [{ ...row, id: "01JNEW" }] }), { status: 201 }),
        ),
      );
      try {
        const p = await previewImport<MapAssetTypeImportRow>(c.target.routes, c.target.rowsField, new Blob(["x"]), "f.xlsx");
        expect(p.ok && p.duLieu.rows).toEqual([row]);
        const html = renderToStaticMarkup(
          <ExcelImportView
            target={c.target}
            fileChosen
            preview={p}
            result={null}
            busy=""
            templateError=""
            onDownloadTemplate={() => {}}
            onChooseFile={() => {}}
            onPreview={() => {}}
            onImport={() => {}}
            onClose={() => {}}
          />,
        );
        expect(html).toContain(`Sẽ tạo 1 ${c.noun}`);
        expect(html).toContain('<td>2</td><td>Mục thử</td><td class="ma-muc">ma-thu</td><td>1</td>');
        const r = await commitImport<MapAssetTypeImportRow>(c.target.routes, new Blob(["x"]), "f.xlsx", "k-1");
        expect(r.ok && r.created).toHaveLength(1);
        expect(c.target.importedSentence(1)).toBe(`Đã nhập 1 ${c.noun}. Danh mục đã được tải lại.`);
      } finally {
        vi.unstubAllGlobals();
      }
    });
  }

  it("409 catalogue_changed: the server's sentence, nothing written", async () => {
    const msg = "Danh mục đã thay đổi từ lúc kiểm tra tệp. Hãy kiểm tra lại tệp rồi nhập.";
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ code: "catalogue_changed", message: msg, trace_id: "t" }), { status: 409 })),
    );
    try {
      const r = await commitImport(TASK_BLOC_IMPORT_TARGET.routes, new Blob(["x"]), "f.xlsx", "k-1");
      expect(r).toEqual({ ok: false, message: msg, errors: [] });
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
