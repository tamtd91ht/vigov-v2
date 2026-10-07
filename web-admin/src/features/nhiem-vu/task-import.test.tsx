import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { SoNhiemVu } from "./so-nhiem-vu";
import { TaskImportDialog, TaskImportView } from "./task-import-dialog";
import {
  IMPORT_CHECKING,
  IMPORT_DESCRIPTION,
  IMPORT_FILE_TOO_LARGE,
  IMPORT_NOT_XLSX,
  IMPORT_OPEN_BUTTON,
  IMPORT_TEMPLATE_BUTTON,
  IMPORT_TEMPLATE_SAVE_NAME,
  IMPORT_TITLE,
  canCommitImport,
  importButtonLabel,
  importFileProblem,
  importKeyFor,
  importedToast,
  previewHeading,
  replayedText,
  sortedImportErrors,
  wrongRowCount,
} from "./task-import";

/**
 * `Nhập từ Excel` — spec 09 (the shared `ExcelImportDialog`, Giải ngân spec 05). No DOM: the drawing
 * in each phase, the pure rules, and by source the page wiring, the key rule and the toasts.
 */

const CLEAN = { total_rows: 3, created: 0, committed: false, errors: [], codes: [] };
const ERRORS = [
  { row: 5, column: "Tiêu đề", message: "thiếu tiêu đề" },
  { row: 3, column: "Hạn hoàn thành (ngày giờ)", message: "hạn hoàn thành phải có cả ngày và giờ" },
  { row: 3, column: "Tiêu đề", message: "thiếu tiêu đề" },
];

function view(p: Partial<Parameters<typeof TaskImportView>[0]> = {}): string {
  return renderToStaticMarkup(
    <TaskImportView
      fileName={null}
      problem={null}
      report={null}
      busy=""
      onTemplate={() => {}}
      onChoose={() => {}}
      onImport={() => {}}
      onClose={() => {}}
      {...p}
    />,
  );
}

describe("the dialog — spec 09: title, description, template link, dashed box, Đóng / Nhập", () => {
  it("42rem, the spec's words in order; the check runs on choosing (no separate `Kiểm tra trước`)", () => {
    const html = renderToStaticMarkup(<TaskImportDialog onClose={() => {}} onImported={() => {}} />);
    const at = (s: string) => html.indexOf(s);
    expect(html).toMatch(/<dialog aria-labelledby="tieu-de-nhap-excel"[^>]*max-w-\[42rem\]/);
    expect(html).toContain(`id="tieu-de-nhap-excel" tabindex="-1"`);
    expect(at(IMPORT_TITLE)).toBeLessThan(at(IMPORT_DESCRIPTION));
    expect(at(IMPORT_DESCRIPTION)).toBeLessThan(at(IMPORT_TEMPLATE_BUTTON));
    expect(at(IMPORT_TEMPLATE_BUTTON)).toBeLessThan(at(">Chọn tệp .xlsx<"));
    expect(at(">Chọn tệp .xlsx<")).toBeLessThan(at(">Đóng<"));
    expect(at(">Đóng<")).toBeLessThan(at(">Nhập<"));
    expect(html).not.toContain("Kiểm tra trước");
    expect(IMPORT_TITLE).toBe("Giao việc hàng loạt từ Excel");
    expect(IMPORT_TEMPLATE_BUTTON).toBe("Tải mẫu giao việc");
    expect(IMPORT_TEMPLATE_SAVE_NAME).toBe("mau-giao-viec.xlsx");
    // Presentation pins (ADR 0068 §5): the dashed drop box and the hidden native input.
    expect(html).toContain('class="border-line rounded-[10px] border border-dashed p-5 text-center"');
    expect(html).toMatch(/<input id="tep-nhap-nhiem-vu"[^>]*type="file"[^>]*accept="\.xlsx[^"]*" class="hidden"/);
  });

  it("no clean check ⇒ `Nhập` is off; a file with a problem says its sentence, still off", () => {
    expect(view()).toMatch(/<button class="nut-chinh[^"]*" type="button" disabled="">Nhập<\/button>/);
    const bad = view({ fileName: "a.xls", problem: IMPORT_NOT_XLSX });
    expect(bad).toContain(`role="alert">${IMPORT_NOT_XLSX}</p>`);
    expect(bad).toContain(">a.xls</p>");
    expect(bad).toMatch(/disabled="">Nhập<\/button>/);
  });

  it("checking: `Đang kiểm tệp…` in a live line; importing: every button off", () => {
    expect(view({ fileName: "a.xlsx", busy: "check" })).toMatch(new RegExp(`role="status">.*${IMPORT_CHECKING}</p>`));
    const html = view({ fileName: "a.xlsx", busy: "import", report: CLEAN });
    expect(html).toMatch(/disabled="">Đóng<\/button>/);
    expect(html).toMatch(/disabled="" aria-busy="true"><svg[^>]*animate-spin[^>]*>.*?<\/svg>Nhập 3 nhiệm vụ<\/button>/);
  });
});

describe("the result box (spec 09 / Giải ngân 05)", () => {
  it("clean: green, `{n} dòng hợp lệ, sẵn sàng nhập`, and `Nhập {n} nhiệm vụ` ON", () => {
    const html = view({ fileName: "a.xlsx", report: CLEAN });
    expect(html).toContain('class="rounded-[10px] border p-3 border-leaf/25 bg-leaf/8"');
    expect(html).toContain(">3 dòng hợp lệ, sẵn sàng nhập</span>");
    expect(html).toMatch(/<button class="nut-chinh[^"]*" type="button">Nhập 3 nhiệm vụ<\/button>/);
  });

  it("errors: red, `{e} dòng sai trên tổng số {n} dòng` (rows, not cells), the Dòng | Cột | Vấn đề table in sheet order", () => {
    const html = view({ fileName: "a.xlsx", report: { ...CLEAN, errors: ERRORS } });
    expect(html).toContain('role="alert"');
    expect(html).toContain("border-danger/25 bg-danger/8");
    expect(html).toContain(">2 dòng sai trên tổng số 3 dòng</span>");
    expect(html).toContain(">Dòng</th>");
    expect(html).toContain(">Cột</th>");
    expect(html).toContain(">Vấn đề</th>");
    expect(html.indexOf(">3</td>")).toBeLessThan(html.indexOf(">5</td>"));
    expect(html).toMatch(/disabled="">Nhập<\/button>/);
    expect(sortedImportErrors(ERRORS).map((e) => e.row)).toEqual([3, 3, 5]);
    expect(wrongRowCount(ERRORS)).toBe(2);
  });

  it("the words", () => {
    expect(previewHeading(CLEAN)).toBe("3 dòng hợp lệ, sẵn sàng nhập");
    expect(importButtonLabel(null)).toBe("Nhập");
    expect(importButtonLabel(CLEAN)).toBe("Nhập 3 nhiệm vụ");
    expect(importButtonLabel({ ...CLEAN, errors: ERRORS })).toBe("Nhập");
    expect(canCommitImport(CLEAN)).toBe(true);
    expect(canCommitImport({ ...CLEAN, total_rows: 0 })).toBe(false);
    expect(importedToast({ codes: ["NV31", "NV32"], created: 2 })).toBe("Đã nhập 2 nhiệm vụ.");
    expect(importedToast({ codes: [], created: 4 })).toBe("Đã nhập 4 nhiệm vụ.");
    expect(replayedText("NV31")).toContain("NV31");
  });
});

describe("rules", () => {
  it("file check before sending: .xlsx only, at most 2 MB", () => {
    expect(importFileProblem({ name: "a.XLSX", size: 10 })).toBeNull();
    expect(importFileProblem({ name: "a.xls", size: 10 })).toBe(IMPORT_NOT_XLSX);
    expect(importFileProblem({ name: "a.xlsx", size: (2 << 20) + 1 })).toBe(IMPORT_FILE_TOO_LARGE);
  });

  it("the real import's key is KEPT across retries, minted only when there is none", () => {
    let n = 0;
    const mint = () => `k${++n}`;
    expect(importKeyFor(null, mint)).toBe("k1");
    expect(importKeyFor("k1", mint)).toBe("k1");
    expect(n).toBe(1);
  });
});

describe("wiring (source)", () => {
  const DIALOG = readFileSync(fileURLToPath(new URL("./task-import-dialog.tsx", import.meta.url)), "utf8");
  const PAGE = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("the CHECK runs on choosing, under a fresh key; the REAL import reuses its key; a new file forgets it", () => {
    expect(DIALOG).toContain("const r = await submitTaskImport(f, f.name, crypto.randomUUID(), true);");
    expect(DIALOG).toContain("const key = importKeyFor(importKey, () => crypto.randomUUID());");
    expect(DIALOG).toContain("const r = await submitTaskImport(file, file.name, key, false);");
    expect(DIALOG).toMatch(/setReport\(null\);\n    setImportKey\(null\);/);
  });

  it("outcomes are toasts; a success reloads the register and closes (spec 09)", () => {
    expect(DIALOG).toContain("toast.success(importedToast(o.report));\n      onImported();\n      onClose();");
    expect(DIALOG).toContain("toast.error(o.report.errors.length > 0 ? IMPORT_ROWS_STILL_WRONG : IMPORT_NOT_COMMITTED);");
    expect(DIALOG).toContain("saveFile(r.duLieu, IMPORT_TEMPLATE_SAVE_NAME);");
  });

  it("the button sits under `task.create` next to `+ Giao việc mới`; a success reloads the register", () => {
    expect(PAGE).toContain("{IMPORT_OPEN_BUTTON}");
    expect(PAGE).toContain("{importOpen && quyen.giaoViec && (");
    expect(PAGE).toContain("onImported={() => datLanTai((n) => n + 1)}");
    expect(IMPORT_OPEN_BUTTON).toBe("Nhập từ Excel");
    expect(PAGE).toContain('icon={<Upload aria-hidden="true" focusable="false" className="size-4" />}');
  });
});

describe("DENIED — no `task.create` (session unread ⇒ every gate closed): no import button", () => {
  it("the register renders without `Nhập từ Excel` and without the dialog", () => {
    const html = renderToStaticMarkup(
      <PhienProvider>
        <SoNhiemVu />
      </PhienProvider>,
    );
    expect(html).not.toContain(IMPORT_OPEN_BUTTON);
    expect(html).not.toContain('aria-labelledby="tieu-de-nhap-excel"');
  });
});
