import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { OrgUnitImportResult } from "@/lib/api/org-unit-import";
import type { identity_orgUnitImportPreviewOut } from "@/lib/api/schema.gen";

import {
  CONFIRM_IMPORT_BUTTON,
  ERRORS_HEADING,
  canImport,
  errorRows,
  keyForAttempt,
  parentText,
} from "./org-unit-import-flow";
import { OrgUnitImportView } from "./org-unit-import-panel";

/**
 * "⬆ Nhập từ Excel": the errors table a refused file produces, the preview list, and the one key
 * per import attempt.
 */

function view(
  preview: KetQua<identity_orgUnitImportPreviewOut> | null,
  result: OrgUnitImportResult | null = null,
) {
  return renderToStaticMarkup(
    <OrgUnitImportView
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
  { row: 4, column: "Bộ phận cha", message: "Không có bộ phận mang mã lanh-dao." },
  { row: 0, column: "", message: "Tệp thiếu cột Mã." },
];

describe("errorRows", () => {
  it("row 0 is the whole file, column '' the whole row — said in words", () => {
    expect(errorRows(ERRORS)).toEqual([
      { row: "4", column: "Bộ phận cha", message: "Không có bộ phận mang mã lanh-dao." },
      { row: "Cả tệp", column: "—", message: "Tệp thiếu cột Mã." },
    ]);
  });
});

describe("preview", () => {
  it("valid:false → errors TABLE (row · column · message), no import button", () => {
    const html = view({ ok: true, duLieu: { valid: false, units: [], errors: ERRORS } });
    expect(html).toContain(ERRORS_HEADING);
    expect(html).toContain("<th scope=\"col\">Dòng</th><th scope=\"col\">Cột</th><th scope=\"col\">Lỗi</th>");
    expect(html).toContain("<td>4</td><td>Bộ phận cha</td><td>Không có bộ phận mang mã lanh-dao.</td>");
    expect(html).toContain("<td>Cả tệp</td><td>—</td><td>Tệp thiếu cột Mã.</td>");
    expect(html).not.toContain(CONFIRM_IMPORT_BUTTON);
  });

  it("valid:true → the units to create, then the import button", () => {
    const html = view({
      ok: true,
      duLieu: {
        valid: true,
        errors: [],
        units: [
          { row: 2, code: "lanh-dao", name: "LÃNH ĐẠO", parent_id: "", parent_code: "", order: 1 },
          { row: 3, code: "van-phong", name: "VĂN PHÒNG", parent_id: "", parent_row: 2, parent_code: "lanh-dao", order: 1 },
        ],
      },
    });
    expect(html).toContain("Sẽ tạo 2 bộ phận");
    expect(html).toContain("VĂN PHÒNG");
    expect(html).toContain("lanh-dao (dòng 2 của tệp)");
    expect(html).toContain(CONFIRM_IMPORT_BUTTON);
  });

  it("a refused FILE (415/413) is one sentence", () => {
    const html = view({ ok: false, thongBao: "Chỉ nhận tệp Excel .xlsx." });
    expect(html).toContain("Chỉ nhận tệp Excel .xlsx.");
    expect(html).not.toContain(CONFIRM_IMPORT_BUTTON);
  });

  it("400 import_invalid on the import itself → the same errors table", () => {
    const html = view(
      { ok: true, duLieu: { valid: true, errors: [], units: [] } },
      { ok: false, message: "Tệp có lỗi nên chưa bộ phận nào được tạo.", errors: ERRORS },
    );
    expect(html).toContain("Tệp có lỗi nên chưa bộ phận nào được tạo.");
    expect(html).toContain("<td>Cả tệp</td>");
  });

  it("a replayed success still says the file is in", () => {
    const html = view({ ok: true, duLieu: { valid: true, errors: [], units: [] } }, { ok: true, created: null });
    expect(html).toContain("đã được nhập ở lần gửi trước");
  });
});

describe("the import attempt's key", () => {
  it("kept across retries of one attempt, minted only when there is none", () => {
    const mint = vi.fn(() => "k-moi");
    expect(keyForAttempt(null, mint)).toBe("k-moi");
    expect(keyForAttempt("k-cu", mint)).toBe("k-cu");
    expect(mint).toHaveBeenCalledTimes(1);
  });

  it("import is offered only for a valid preview with units", () => {
    expect(canImport(null)).toBe(false);
    expect(canImport({ valid: false, units: [1] })).toBe(false);
    expect(canImport({ valid: true, units: [] })).toBe(false);
    expect(canImport({ valid: true, units: [1] })).toBe(true);
  });

  it("parent text: existing unit, row of the file, or top level", () => {
    const base = { row: 5, code: "x", name: "X", parent_id: "", order: 0 };
    expect(parentText({ ...base, parent_code: "" })).toBe("Cấp cao nhất");
    expect(parentText({ ...base, parent_code: "", parent_row: 3 })).toBe("Dòng 3 của tệp");
    expect(parentText({ ...base, parent_code: "van-phong", parent_id: "01J" })).toBe("van-phong");
  });
});
