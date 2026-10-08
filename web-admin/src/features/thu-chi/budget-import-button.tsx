"use client";

import { Loader2, Upload } from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import {
  commitBudgetImport,
  type BudgetImportResult,
} from "@/lib/api/budget-import";
import type { finance_budgetImportSheetOut } from "@/lib/api/schema.gen";

import { nhanLoaiBang } from "./nhan-thu-chi";

/**
 * `⬆ Nạp từ Excel` of Thu - Chi — the prototype's flow exactly (`FiscalReportPanel.tsx:131-153, 189-211`,
 * ADR 0081 #6): the button opens a hidden `.xlsx` picker, the picked file is loaded AT ONCE, the button
 * spins and is disabled while it uploads, and a toast says what was loaded or why not. No preview step:
 * the server checks the whole file and loads all of it or nothing, so a refused file has changed nothing.
 *
 * NO TEMPLATE: the file is the finance office's own report ("không có mẫu riêng để tải về", NT-1).
 *
 * `year` is the year selected on the screen (the build's year select — allowed against the prototype,
 * which used the machine's year).
 *
 * ONE `Idempotency-Key` PER PICK: a double submit of one pick is one import, never a second revision.
 *
 * Shown only with `budget.update` (the caller's `canRecord`), as the prototype does — UX only, the route
 * checks the key itself (rule 5).
 */
export function BudgetImportButton({
  year,
  disabled,
  onImported,
  mintKey = khoaChongTrungMoi,
}: {
  year: number;
  disabled?: boolean;
  /** After a 201: the board re-reads, then selects `firstKind` (the first sheet returned), when known. */
  onImported: (firstKind: "thu" | "chi" | null) => void;
  /** The key source — a parameter so a test can read it. */
  mintKey?: () => string;
}) {
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function send(file: File) {
    setBusy(true);
    const r = await commitBudgetImport(file, file.name, year, mintKey());
    setBusy(false);
    if (!r.ok) {
      toast.error(importFailureSentence(r));
      return;
    }
    const sheets = r.out?.sheets ?? [];
    if (sheets.length > 0) toast.success(importedSentence(sheets));
    else toast.success(IMPORTED_REPLAY);
    const k = sheets[0]?.kind;
    onImported(k === "thu" || k === "chi" ? k : null);
  }

  return (
    <>
      <input
        ref={fileRef}
        type="file"
        accept=".xlsx"
        className="hidden"
        aria-hidden="true"
        tabIndex={-1}
        onChange={(event) => {
          const file = event.target.files?.[0];
          // Cleared so the SAME file can be picked again after it was fixed.
          event.target.value = "";
          if (file) void send(file);
        }}
      />
      <Button
        type="button"
        variant="primary"
        size="sm"
        icon={
          busy ? (
            <Loader2 aria-hidden="true" className="animate-spin" />
          ) : (
            <Upload aria-hidden="true" focusable="false" />
          )
        }
        disabled={disabled || busy}
        aria-busy={busy}
        onClick={() => fileRef.current?.click()}
      >
        {IMPORT_BUTTON}
      </Button>
    </>
  );
}

export const IMPORT_BUTTON = "Nạp từ Excel";

/** The prototype's three failure sentences (`FiscalReportPanel.tsx:145-151`), verbatim. */
export const NO_FISCAL_SHEET =
  "Không tìm thấy bảng thu - chi trong tệp. Kiểm tra lại tệp có hàng tiêu đề bắt đầu bằng TT hoặc STT.";
export const BAD_WORKBOOK = "Không đọc được tệp. Tệp phải là .xlsx.";
export const NOT_IMPORTED = "Chưa nạp được tệp.";

/** A replayed success carries no sheets to name; the board is re-read either way. */
export const IMPORTED_REPLAY = "Đã nạp tệp.";

/** `thu` / `chi` in words; an unknown kind is printed as the server wrote it, never guessed. */
function kindLabel(kind: string): string {
  return kind === "thu" || kind === "chi" ? nhanLoaiBang(kind) : kind;
}

/** The prototype's success toast: `Đã nạp: Chi ngân sách (59 khoản mục) · Thu ngân sách (31 khoản mục)`. */
export function importedSentence(
  sheets: readonly finance_budgetImportSheetOut[],
): string {
  return `Đã nạp: ${sheets.map((s) => `${kindLabel(s.kind)} (${s.line_count} khoản mục)`).join(" · ")}`;
}

/**
 * The failure toast. The prototype's codes mapped onto what THIS server answers (no code invented):
 *
 *   prototype `bad_workbook`    ← 415 `unsupported_file_type` (not .xlsx, macro, password) · 400 `malformed_file`
 *   prototype `no_fiscal_sheet` ← 400 `import_invalid` whose ONLY error names no sheet: the one file-level
 *                                 error `domain.ParseBudgetWorkbook` has is "no sheet with a TT/STT header"
 *                                 (every other error names its sheet) — matched by SHAPE, never by wording
 *   the ADR 0081 #6 refusals    409 `budget_period_closed` · `budget_sheet_has_entries`: the server's
 *                               sentence — it names the close or the sheet, and the way out
 *   other `import_invalid`       the first error with where it is, and how many more
 *   anything else                the prototype's "Chưa nạp được tệp."
 */
export function importFailureSentence(
  r: Extract<BudgetImportResult, { ok: false }>,
): string {
  switch (r.code) {
    case "unsupported_file_type":
    case "malformed_file":
      return BAD_WORKBOOK;
    case "budget_period_closed":
    case "budget_sheet_has_entries":
      return r.message;
    case "import_invalid": {
      const first = r.errors[0];
      if (first === undefined) return r.message;
      if (r.errors.length === 1 && first.sheet === "") return NO_FISCAL_SHEET;
      const where = [
        first.sheet === "" ? "" : `Sheet ${first.sheet}`,
        first.row > 0 ? `dòng ${first.row}` : "",
        first.column === "" ? "" : `cột ${first.column}`,
      ]
        .filter((x) => x !== "")
        .join(" ");
      const more =
        r.errors.length > 1 ? ` … và ${r.errors.length - 1} lỗi khác` : "";
      return `${where === "" ? "" : `${where}: `}${first.message}${more}`;
    }
    default:
      return NOT_IMPORTED;
  }
}
