import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";

import { STAFF_IMPORT_DESCRIPTION, STAFF_IMPORT_ERRORS_HEADING, STAFF_IMPORT_TITLE } from "./nhan-can-bo";
import { StaffImportView, type StaffImportBusy } from "./staff-import-dialog";

/**
 * `Nhập người dùng từ Excel` (user 09/10/2026) — the body of the 672px dialog, rendered without state.
 * The flow (preview first, then the write under one key) is `StaffImportDialog`'s; what is pinned here
 * is what the officer reads in each state. The password values are fakes (rule 8).
 */

type View = {
  fileName?: string | null;
  preview?: KetQua<ImportPreview<StaffImportPlannedRow>> | null;
  result?: ImportResult<StaffImportCreatedRow> | null;
  busy?: StaffImportBusy;
};

function view(v: View = {}): string {
  return renderToStaticMarkup(
    <StaffImportView
      fileName={v.fileName ?? null}
      preview={v.preview ?? null}
      result={v.result ?? null}
      busy={v.busy ?? ""}
      templateError=""
      onTemplate={() => {}}
      onChoose={() => {}}
      onImport={() => {}}
      onClose={() => {}}
    />,
  );
}

function button(html: string, text: string): string {
  return new RegExp(`<button[^>]*>(?:(?!</button>).)*${text}</button>`).exec(html)?.[0] ?? "";
}

describe("staff import dialog — the prototype's words and shape", () => {
  it("title and description are the user's sentences", () => {
    expect(STAFF_IMPORT_TITLE).toBe("Nhập người dùng từ Excel");
    expect(STAFF_IMPORT_DESCRIPTION).toBe(
      "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.",
    );
  });

  it("link 'Tải tệp mẫu' with a download icon, dashed box with 'Chọn tệp .xlsx', footer Đóng · Nhập", () => {
    const html = view();
    expect(button(html, "Tải tệp mẫu")).toContain("lucide-download");
    expect(html).toContain("border-dashed");
    expect(button(html, "Chọn tệp .xlsx")).not.toBe("");
    expect(html.indexOf(">Đóng</button>")).toBeLessThan(html.indexOf(">Nhập</button>"));
    // The old panel's long guidance and its separate check button are gone.
    expect(html).not.toContain("Kiểm tra tệp");
    expect(html).not.toContain("mật khẩu tạm");
  });

  it("Nhập is DISABLED until a file is chosen, enabled after", () => {
    expect(button(view(), "Nhập")).toContain('disabled=""');
    expect(button(view({ fileName: "can-bo.xlsx" }), "Nhập")).not.toContain('disabled=""');
    expect(view({ fileName: "can-bo.xlsx" })).toContain(">can-bo.xlsx</p>");
  });

  it("invalid preview → the errors by row (row 0 = whole file), nothing written, Nhập still offered", () => {
    const html = view({
      fileName: "can-bo.xlsx",
      preview: {
        ok: true,
        duLieu: {
          valid: false,
          rows: [],
          errors: [
            { row: 4, column: "Bộ phận", message: "Không có bộ phận này trong danh mục." },
            { row: 0, column: "", message: "Thiếu trang tính Cán bộ." },
          ],
        },
      },
    });
    expect(html).toContain(STAFF_IMPORT_ERRORS_HEADING);
    expect(html).toContain(">4</td>");
    expect(html).toContain(">Cả tệp</td>");
    expect(html).toContain("Không có bộ phận này trong danh mục.");
    expect(button(html, "Nhập")).not.toContain('disabled=""');
  });

  it("a refusal of the FILE → the server's sentence, verbatim", () => {
    const sentence = "Tệp có cột Vai trò nên tài khoản của bạn cần thêm quyền Phân quyền.";
    expect(view({ fileName: "x.xlsx", preview: { ok: false, thongBao: sentence } })).toContain(sentence);
  });

  it("while checking or importing: both buttons of the footer are busy-safe", () => {
    const html = view({ fileName: "x.xlsx", busy: "import" });
    expect(button(html, "Nhập")).toContain('disabled=""');
    expect(button(html, "Đóng")).toContain('disabled=""');
    expect(html).toContain("Đang nhập…");
  });

  it("201 → the imported sentence and the one-time password table, which owns closing (no Đóng · Nhập)", () => {
    const html = view({
      fileName: "x.xlsx",
      result: {
        ok: true,
        created: [
          {
            row: 2,
            code: "CB-00009",
            full_name: "Trần Thị B",
            login: "ttb@demo.invalid",
            account_issued: true,
            temporary_password: "mat-khau-gia-de-kiem-tra",
          } as unknown as StaffImportCreatedRow,
        ],
      },
    });
    expect(html).toContain("Đã nhập 1 cán bộ.");
    expect(html).toContain(">mat-khau-gia-de-kiem-tra<");
    expect(html).toContain("Tôi đã lưu, đóng");
    expect(html).not.toContain(">Nhập</button>");
    expect(html).not.toContain("border-dashed");
  });
});
