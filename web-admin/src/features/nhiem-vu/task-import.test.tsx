import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { SoNhiemVu } from "./so-nhiem-vu";
import { ImportAnswerView, TaskImportView, type ImportAnswer } from "./task-import-dialog";
import {
  IMPORT_CHECK_BUTTON,
  IMPORT_DEADLINE_NOTE,
  IMPORT_DESCRIPTION,
  IMPORT_FILE_TOO_LARGE,
  IMPORT_NOT_COMMITTED,
  IMPORT_NOT_XLSX,
  IMPORT_OPEN_BUTTON,
  IMPORT_SUBMIT_BUTTON,
  IMPORT_TEMPLATE_BUTTON,
  checkedOkText,
  importErrorLine,
  importFileProblem,
  importKeyFor,
  importedText,
  replayedText,
  sortedImportErrors,
} from "./task-import";

/**
 * `⬆ Nhập từ Excel` (W7, §8). No DOM: the drawing in each phase, the pure rules, and by source the
 * page wiring and the key rule.
 */

const REPORT = { total_rows: 3, created: 0, committed: false, errors: [], codes: [] };

function view(p: Partial<Parameters<typeof TaskImportView>[0]> = {}): string {
  return renderToStaticMarkup(
    <TaskImportView
      fileName={null}
      fileProblem={null}
      sending={null}
      answer={null}
      templateBusy={false}
      templateError={null}
      onTemplate={() => {}}
      onChoose={() => {}}
      onCheck={() => {}}
      onImport={() => {}}
      onClose={() => {}}
      {...p}
    />,
  );
}

const answer = (a: ImportAnswer) => renderToStaticMarkup(<ImportAnswerView answer={a} />);

describe("the dialog — §8 order: description → template → file → Đóng / Nhập", () => {
  it("the parts in order, and the deadline note that says date AND time", () => {
    const html = view();
    const at = (s: string) => html.indexOf(s);
    expect(html).toContain('role="dialog"');
    expect(at(IMPORT_DESCRIPTION)).toBeGreaterThan(-1);
    expect(at(IMPORT_DESCRIPTION)).toBeLessThan(at(IMPORT_TEMPLATE_BUTTON));
    expect(at(IMPORT_TEMPLATE_BUTTON)).toBeLessThan(at("Chọn tệp .xlsx"));
    expect(at("Chọn tệp .xlsx")).toBeLessThan(at(">Đóng<"));
    expect(at(">Đóng<")).toBeLessThan(at(`>${IMPORT_SUBMIT_BUTTON}<`));
    expect(html).toContain("30/09/2026 17:00");
    expect(IMPORT_DEADLINE_NOTE).toContain("cả ngày và giờ");
    expect(IMPORT_DESCRIPTION).toContain("NV01, NV02");
    expect(html).toMatch(/<input id="tep-nhap-nhiem-vu"[^>]*type="file"[^>]*accept="\.xlsx/);
  });

  it("no file ⇒ `Kiểm tra trước` and `Nhập` are off; a file with a problem ⇒ still off, and the sentence", () => {
    expect(view()).toMatch(new RegExp(`disabled="">${IMPORT_CHECK_BUTTON}<`));
    const bad = view({ fileName: "a.xls", fileProblem: IMPORT_NOT_XLSX });
    expect(bad).toContain(IMPORT_NOT_XLSX);
    expect(bad).toMatch(new RegExp(`class="nut-chinh" disabled="">${IMPORT_SUBMIT_BUTTON}<`));
    const ok = view({ fileName: "a.xlsx" });
    expect(ok).toContain("Tệp đã chọn: a.xlsx");
    expect(ok).not.toMatch(new RegExp(`class="nut-chinh" disabled="">${IMPORT_SUBMIT_BUTTON}<`));
  });

  it("sending: the live line, every button off", () => {
    const html = view({ fileName: "a.xlsx", sending: "import" });
    expect(html).toMatch(/role="status">Đang nhập…/);
    expect(html).toMatch(/disabled="">Đóng</);
  });
});

describe("the report", () => {
  it("row refusals: row, column, the server's sentence — in sheet order; the heading says NOTHING was imported", () => {
    const errors = [
      { row: 5, column: "Tiêu đề", message: "thiếu tiêu đề" },
      { row: 3, column: "Hạn hoàn thành (ngày giờ)", message: "hạn hoàn thành phải có cả ngày và giờ" },
    ];
    const html = answer({ dryRun: false, ok: true, outcome: { kind: "checked", report: { ...REPORT, errors } } });
    expect(html).toContain("chưa nhiệm vụ nào được nhập");
    expect(html.indexOf("Dòng 3")).toBeLessThan(html.indexOf("Dòng 5"));
    expect(html).toContain("Dòng 3 · cột “Hạn hoàn thành (ngày giờ)”: hạn hoàn thành phải có cả ngày và giờ");
    expect(sortedImportErrors(errors).map((e) => e.row)).toEqual([3, 5]);
    expect(importErrorLine({ row: 2, column: "", message: "x" })).toBe("Dòng 2: x");
  });

  it("a clean CHECK says the file would import and nothing was written; a clean-looking REAL 200 does not say \"bấm Nhập\"", () => {
    expect(answer({ dryRun: true, ok: true, outcome: { kind: "checked", report: REPORT } })).toContain(
      checkedOkText(3),
    );
    expect(answer({ dryRun: false, ok: true, outcome: { kind: "checked", report: REPORT } })).toContain(
      IMPORT_NOT_COMMITTED,
    );
  });

  it("201: the issued codes; replay: said plainly, with the first code", () => {
    const html = answer({
      dryRun: false,
      ok: true,
      outcome: { kind: "imported", report: { ...REPORT, committed: true, created: 2, codes: ["NV31", "NV32"] } },
    });
    expect(html).toContain(importedText(["NV31", "NV32"]));
    expect(importedText(["NV31", "NV32"])).toBe("Đã nhập 2 nhiệm vụ: NV31, NV32.");
    expect(answer({ dryRun: false, ok: true, outcome: { kind: "replayed", firstCode: "NV31" } })).toContain(
      replayedText("NV31"),
    );
  });

  it("a refusal sentence verbatim", () => {
    expect(answer({ dryRun: false, ok: false, message: "Tệp có quá 500 dòng nhiệm vụ." })).toContain(
      "Tệp có quá 500 dòng nhiệm vụ.",
    );
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

  it("a CHECK takes a fresh key; a REAL import reuses its key; success drops key and file and reloads", () => {
    expect(DIALOG).toContain(
      "const key = dryRun ? crypto.randomUUID() : importKeyFor(importKey, () => crypto.randomUUID());",
    );
    expect(DIALOG).toContain('if (r.duLieu.kind === "imported" || r.duLieu.kind === "replayed") {');
    expect(DIALOG).toContain("setImportKey(null);\n        setFile(null);\n        onImported();");
    // A new file forgets the old key.
    expect(DIALOG).toMatch(/function choose\(f: File \| null\) \{\n    setAnswer\(null\);\n    setImportKey\(null\);/);
  });

  it("the button sits under `task.create` next to `+ Giao việc mới`; a success reloads the register", () => {
    expect(PAGE).toContain("{IMPORT_OPEN_BUTTON}");
    expect(PAGE).toContain("{importOpen && quyen.giaoViec && (");
    expect(PAGE).toContain("onImported={() => datLanTai((n) => n + 1)}");
    expect(IMPORT_OPEN_BUTTON).toBe("⬆ Nhập từ Excel");
  });
});

describe("DENIED — no `task.create` (session unread ⇒ every gate closed): no import button", () => {
  it("the register renders without `⬆ Nhập từ Excel` and without the dialog", () => {
    const html = renderToStaticMarkup(
      <PhienProvider>
        <SoNhiemVu />
      </PhienProvider>,
    );
    expect(html).not.toContain(IMPORT_OPEN_BUTTON);
    expect(html).not.toContain('aria-labelledby="tieu-de-nhap-excel"');
  });
});
