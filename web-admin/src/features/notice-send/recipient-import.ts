/**
 * The two imported recipient sources of "Gửi tin ZNS / SMS" (ADR 0099 §Sửa đổi 17/10/2026, S1–S4): a
 * typed list of numbers, and a `.xlsx` / `.csv` file READ IN THE BROWSER. The pure half — parsing, limits,
 * the `{{variable_N}}` detection, the rows the POST carries, every sentence the officer reads.
 *
 * THE FILE NEVER LEAVES THE BROWSER (S2, the owner: "file import thì có thể xoá sau khi import xong"):
 * it is read here into cells, the officer maps columns, and only the ROWS go up in the preview / send body.
 * Comms checks every row again — the limits below are the same ones it enforces, repeated so the officer
 * is told before a 10.000-row body travels, not instead of the server.
 *
 * PERSONAL DATA (rule 3): numbers and per-row texts are held in memory only — never logged, never in a
 * URL, never in a key. Wherever this screen SHOWS a number it is masked (`maskPhone`, comms' own shape).
 * Numbers are sent exactly as typed: normalising `0…` / `84…` / `+84…` is comms' job (S1), and a second
 * normaliser here would be a second rule that drifts from the one that decides.
 */

import type { NoticeRecipientMapping, NoticeRecipientRow, SmsContentMode } from "@/lib/api/notice-campaigns";

// ---- limits (S1, and the task's per-row text cap) ---------------------------------------------------

export const MAX_TYPED_PHONES = 1000;
export const MAX_FILE_ROWS = 10_000;
export const MAX_FILE_BYTES = 5 * 1024 * 1024;
export const MAX_CONTENT_LENGTH = 1000;

const vi = (n: number) => n.toLocaleString("vi-VN");

// ---- the source switch ------------------------------------------------------------------------------

export type RecipientSourceKind = "directory" | "typed" | "file";

export const SOURCE_LABEL: Readonly<Record<RecipientSourceKind, string>> = {
  directory: "Danh bạ",
  typed: "Nhập số điện thoại",
  file: "Nhập tệp",
};
export const SOURCE_KINDS = Object.keys(SOURCE_LABEL) as RecipientSourceKind[];
export const SOURCE_LEGEND = "Nguồn người nhận";

// ---- typed list -------------------------------------------------------------------------------------

export const TYPED_LABEL = "Số điện thoại";
export const TYPED_HINT = `Các số cách nhau bằng dấu phẩy; dán danh sách nhiều dòng cũng được. Tối đa ${vi(MAX_TYPED_PHONES)} số.`;
export const TYPED_EMPTY = "Chưa nhập số điện thoại nào.";

/** One entry per line or per comma; blanks dropped. Nothing else is decided here — comms normalises. */
export function splitTypedPhones(text: string): string[] {
  return text
    .split(/[\n\r,;]+/)
    .map((s) => s.trim())
    .filter((s) => s !== "");
}

export function typedCountText(n: number): string {
  return `${vi(n)} số đã nhập`;
}

/** The typed list's refusal, or null. */
export function typedPhonesError(phones: readonly string[]): string | null {
  if (phones.length === 0) return TYPED_EMPTY;
  if (phones.length > MAX_TYPED_PHONES) {
    return `Tối đa ${vi(MAX_TYPED_PHONES)} số mỗi lần nhập — đang có ${vi(phones.length)} số. Dùng Nhập tệp cho danh sách dài hơn.`;
  }
  return null;
}

// ---- CSV --------------------------------------------------------------------------------------------

/**
 * RFC 4180 CSV: quoted fields (`"a, b"`, `""` for a quote, line breaks inside quotes), CRLF / LF / CR,
 * a leading UTF-8 BOM dropped. The delimiter is `,` — or `;` when the FIRST line has more semicolons
 * than commas outside quotes: Excel writes `;` on a machine whose list separator is `;`, and a
 * Vietnamese-locale Windows is commonly one of them. A trailing line break adds no empty row.
 */
export function parseCsv(input: string): string[][] {
  const text = input.charCodeAt(0) === 0xfeff ? input.slice(1) : input;
  const delimiter = detectDelimiter(text);
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let quoted = false;
  let i = 0;
  while (i < text.length) {
    const c = text[i]!;
    if (quoted) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 2;
          continue;
        }
        quoted = false;
        i++;
        continue;
      }
      field += c;
      i++;
      continue;
    }
    if (c === '"' && field === "") {
      quoted = true;
      i++;
      continue;
    }
    if (c === delimiter) {
      row.push(field);
      field = "";
      i++;
      continue;
    }
    if (c === "\r" || c === "\n") {
      row.push(field);
      rows.push(row);
      row = [];
      field = "";
      i += c === "\r" && text[i + 1] === "\n" ? 2 : 1;
      continue;
    }
    field += c;
    i++;
  }
  if (field !== "" || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows;
}

function detectDelimiter(text: string): "," | ";" {
  let commas = 0;
  let semicolons = 0;
  let quoted = false;
  for (const c of text) {
    if (c === '"') quoted = !quoted;
    else if (!quoted && (c === "\n" || c === "\r")) break;
    else if (!quoted && c === ",") commas++;
    else if (!quoted && c === ";") semicolons++;
  }
  return semicolons > commas ? ";" : ",";
}

// ---- the file ---------------------------------------------------------------------------------------

/** A file read into cells: row 1 = the headers, every row padded to the same width, all cells text. */
export type ImportedTable = { fileName: string; headers: string[]; rows: string[][] };

export type FileReadResult = { ok: true; table: ImportedTable } | { ok: false; error: string };

export const FILE_LABEL = "Tệp danh sách";
export const FILE_HINT = `Tệp .xlsx hoặc .csv, tối đa 5 MB và ${vi(MAX_FILE_ROWS)} dòng. Hệ thống đọc trang tính đầu tiên; dòng 1 là tiêu đề cột. Tệp chỉ được đọc trên máy này, không tải lên máy chủ.`;
export const FILE_WRONG_TYPE = "Chỉ nhận tệp .xlsx hoặc .csv. Tệp .xls cũ: mở bằng Excel rồi lưu lại dạng .xlsx.";
export const FILE_TOO_BIG = "Tệp lớn hơn 5 MB. Bớt cột hoặc chia thành nhiều tệp.";
export const FILE_NO_HEADER = "Dòng 1 của tệp đang trống. Dòng 1 phải là tiêu đề cột (ví dụ: Số điện thoại, Nội dung).";
export const FILE_NO_ROWS = "Tệp không có dòng dữ liệu nào dưới dòng tiêu đề.";
export const FILE_NOT_UTF8 = "Tệp CSV không phải mã UTF-8 nên chữ có dấu sẽ hỏng. Trong Excel chọn Lưu thành → \"CSV UTF-8\".";
export const FILE_UNREADABLE = "Không đọc được tệp. Kiểm tra tệp có đúng định dạng .xlsx hoặc .csv và không bị khoá mật khẩu.";

export function fileTooManyRows(n: number): string {
  return `Tệp có ${vi(n)} dòng dữ liệu, quá giới hạn ${vi(MAX_FILE_ROWS)} dòng. Chia thành nhiều tệp.`;
}

export function fileSummary(t: ImportedTable): string {
  return `${t.fileName} · ${vi(t.rows.length)} dòng`;
}

type FileKind = "xlsx" | "csv";

export function fileKindOf(name: string): FileKind | null {
  const lower = name.toLowerCase();
  if (lower.endsWith(".xlsx")) return "xlsx";
  if (lower.endsWith(".csv")) return "csv";
  return null;
}

/**
 * Cells → a table, or the sentence that says why not. Fully empty rows are dropped (a sheet's trailing
 * blank lines are not recipients); every row is padded to the widest one so a column index is always
 * a cell.
 */
export function tableFromCells(fileName: string, cells: readonly (readonly string[])[]): FileReadResult {
  const trimmed = cells.map((r) => r.map((c) => c.trim()));
  const [head, ...body] = trimmed;
  if (head === undefined || head.every((c) => c === "")) return { ok: false, error: FILE_NO_HEADER };
  const rows = body.filter((r) => r.some((c) => c !== ""));
  if (rows.length === 0) return { ok: false, error: FILE_NO_ROWS };
  if (rows.length > MAX_FILE_ROWS) return { ok: false, error: fileTooManyRows(rows.length) };
  const width = Math.max(head.length, ...rows.map((r) => r.length));
  const pad = (r: string[]) => (r.length >= width ? r : [...r, ...Array<string>(width - r.length).fill("")]);
  return { ok: true, table: { fileName, headers: pad(head), rows: rows.map(pad) } };
}

/** An `.xlsx` cell as text. Numbers arrive as their stored text (`parseNumber`), dates as dd/mm/yyyy. */
export function cellText(v: unknown): string {
  if (v === null || v === undefined) return "";
  if (v instanceof Date) return dateText(v);
  return String(v);
}

function dateText(d: Date): string {
  if (Number.isNaN(d.getTime())) return "";
  const two = (n: number) => String(n).padStart(2, "0");
  // read-excel-file returns spreadsheet dates as UTC instants: read them back in UTC, or a date typed
  // "20/10/2026" in a cell would show as the 19th west of Greenwich.
  const date = `${two(d.getUTCDate())}/${two(d.getUTCMonth() + 1)}/${d.getUTCFullYear()}`;
  return d.getUTCHours() === 0 && d.getUTCMinutes() === 0 ? date : `${date} ${two(d.getUTCHours())}:${two(d.getUTCMinutes())}`;
}

/**
 * Reads the picked file IN THE BROWSER. The size and the type are checked before a byte is read;
 * `read-excel-file` is a dynamic import, so it enters the bundle only for the officer who imports.
 */
export async function readRecipientFile(file: File): Promise<FileReadResult> {
  const kind = fileKindOf(file.name);
  if (kind === null) return { ok: false, error: FILE_WRONG_TYPE };
  if (file.size > MAX_FILE_BYTES) return { ok: false, error: FILE_TOO_BIG };
  if (kind === "csv") {
    let text: string;
    try {
      text = new TextDecoder("utf-8", { fatal: true }).decode(await file.arrayBuffer());
    } catch {
      return { ok: false, error: FILE_NOT_UTF8 };
    }
    return tableFromCells(file.name, parseCsv(text));
  }
  try {
    const { readSheet } = await import("read-excel-file/browser");
    // `parseNumber` keeps a number as the text the sheet stores: a float would turn 0912… into 912…
    // and a long code into 1.2e+10.
    const sheet = await readSheet<string>(file, { parseNumber: (s: string) => s });
    return tableFromCells(
      file.name,
      sheet.map((r) => r.map(cellText)),
    );
  } catch {
    return { ok: false, error: FILE_UNREADABLE };
  }
}

// ---- columns ----------------------------------------------------------------------------------------

/** 0 → "A", 25 → "Z", 26 → "AA" — the spreadsheet's own column name. */
export function columnLetter(i: number): string {
  let n = i + 1;
  let s = "";
  while (n > 0) {
    const r = (n - 1) % 26;
    s = String.fromCharCode(65 + r) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

/** "Số điện thoại (cột A)"; an empty header "Cột A". */
export function columnLabel(headers: readonly string[], i: number): string {
  const h = headers[i] ?? "";
  return h === "" ? `Cột ${columnLetter(i)}` : `${h} (cột ${columnLetter(i)})`;
}

const PHONE_HEADER = /(điện thoại|dien thoai|sđt|sdt|phone|di động|di dong|mobile)/i;

/** The column whose header names a phone, if exactly one does — a suggestion the officer can change. */
export function guessPhoneColumn(headers: readonly string[]): number | null {
  const hits = headers.map((h, i) => (PHONE_HEADER.test(h) ? i : -1)).filter((i) => i >= 0);
  return hits.length === 1 ? hits[0]! : null;
}

export const PHONE_COLUMN_LABEL = "Cột số điện thoại";
export const CONTENT_COLUMN_LABEL = "Cột nội dung";
export const PICK_COLUMN = "— Chọn cột —";
export const PHONE_COLUMN_MISSING = "Chọn cột số điện thoại.";

// ---- masking on screen ------------------------------------------------------------------------------

/** comms' `privacy.MaskPhone`, the same shape: 2 first + stars + 4 last; shorter than 8 → all stars. */
export function maskPhone(raw: string): string {
  const s = raw.trim();
  if (s.length < 8) return "*".repeat(s.length);
  return s.slice(0, 2) + "*".repeat(s.length - 6) + s.slice(-4);
}

/** A cell that reads like a phone number (8–15 digits once spaces, dots, dashes, `+` and brackets go). */
export function looksLikePhone(cell: string): boolean {
  const digits = cell.replace(/[\s.\-+()]/g, "");
  return /^\d{8,15}$/.test(digits);
}

export const PREVIEW_ROWS = 3;

/**
 * The first rows for the on-screen preview, numbers MASKED: the picked phone column always, and any
 * other cell that looks like a number — before a column is picked the phone could be in any of them.
 */
export function maskedPreview(t: ImportedTable, phoneColumn: number | null, n = PREVIEW_ROWS): string[][] {
  return t.rows.slice(0, n).map((r) => r.map((c, i) => (i === phoneColumn || looksLikePhone(c) ? maskPhone(c) : c)));
}

// ---- {{variable_N}} ---------------------------------------------------------------------------------

const VARIABLE = /\{\{variable_([1-9]\d*)\}\}/g;

/** The distinct N of every `{{variable_N}}` in the text, ascending. */
export function detectVariables(content: string): number[] {
  const seen = new Set<number>();
  for (const m of content.matchAll(VARIABLE)) seen.add(Number(m[1]));
  return [...seen].sort((a, b) => a - b);
}

export function variableKey(n: number): string {
  return `variable_${n}`;
}

export function variableToken(n: number): string {
  return `{{variable_${n}}}`;
}

/** The text with each mapped variable replaced; an unmapped one is left as written. */
export function fillVariables(content: string, values: Readonly<Record<string, string>>): string {
  return content.replace(VARIABLE, (whole, n: string) => values[variableKey(Number(n))] ?? whole);
}

export const VARIABLES_HELP =
  "Muốn mỗi người một nội dung riêng: viết {{variable_1}}, {{variable_2}}… vào chỗ cần thay trong nội dung. " +
  "Với mỗi biến, chọn một cột của tệp; khi gửi, biến được thay bằng ô của cột đó ở đúng dòng của người nhận. " +
  "Biến chưa chọn cột sẽ bị báo thiếu khi xem trước.";
export const VARIABLES_HELP_LABEL = "Cách dùng biến trong nội dung";
export const VARIABLES_TITLE = "Biến trong nội dung";
export const TYPED_NO_VARIABLES =
  "Danh sách nhập tay không có cột để thay biến. Bỏ {{variable_…}} khỏi nội dung, hoặc dùng Nhập tệp.";

// ---- SMS content to an imported list (S3) -----------------------------------------------------------

export const SMS_MODE_LEGEND = "Nội dung tin";
export const SMS_MODE_LABEL: Readonly<Record<SmsContentMode, string>> = {
  "cot-trong-tep": "Lấy nội dung từ cột trong tệp",
  "noi-dung-chung": "Nội dung chung cho cả danh sách",
};
export const SHARED_CONTENT_LABEL = "Nội dung";
export const CONTENT_EMPTY = "Chưa nhập nội dung.";
export const CONTENT_COLUMN_MISSING = "Chọn cột nội dung.";
export const SMS_NO_TEMPLATE_NOTE =
  "Tin SMS cho danh sách nhập không chọn mẫu: nội dung phải khớp một mẫu đã đăng ký với nhà mạng, dòng sai mẫu sẽ bị nhà cung cấp từ chối và ghi lỗi ở lịch sử.";

export function charCounterText(n: number): string {
  return `${vi(n)}/${vi(MAX_CONTENT_LENGTH)} ký tự`;
}

export function contentTooLong(n: number): string {
  return `Nội dung dài ${vi(n)} ký tự, quá giới hạn ${vi(MAX_CONTENT_LENGTH)} ký tự.`;
}

/** File rows named the way the officer finds them in the sheet: data row i is sheet row i + 2. */
function sheetRows(indexes: readonly number[]): string {
  const shown = indexes.slice(0, 5).map((i) => vi(i + 2));
  return indexes.length > 5 ? `${shown.join(", ")}…` : shown.join(", ");
}

// ---- ZNS parameters from a file (S4) ----------------------------------------------------------------

/** `null` = the shared value typed in the field; a number = that file column. */
export type ParamColumns = Readonly<Record<string, number | null>>;
export const PARAM_SOURCE_SHARED = "Giá trị chung";
export function paramSourceColumn(headers: readonly string[], i: number): string {
  return `Lấy từ ${columnLabel(headers, i)}`;
}

// ---- the composition, and the rows it sends ---------------------------------------------------------

export type ImportedSource =
  | { kind: "typed"; phones: readonly string[] }
  | { kind: "file"; table: ImportedTable; phoneColumn: number | null };

export type SmsImportComposition = {
  mode: SmsContentMode;
  contentColumn: number | null;
  content: string;
  /** variable N → file column (`null` = not chosen yet). */
  variableColumns: Readonly<Record<number, number | null>>;
};

/**
 * The rows the POST carries (S1–S4), built key by key:
 *   typed            `{phone}` — shared values / content travel at the top of the body
 *   file, SMS column `{phone, content: <cell>}`
 *   file, SMS shared `{phone, values: {variable_N: <cell>}}` for each detected, mapped N
 *   file, ZNS        `{phone, values: {<field>: <cell>}}` for each parameter mapped to a column
 * A row whose phone cell is empty still goes: comms records it as `khong-hop-le` (S5).
 */
export function buildRows(
  source: ImportedSource,
  channel: "zns" | "sms",
  sms: SmsImportComposition | null,
  paramColumns: ParamColumns,
): NoticeRecipientRow[] {
  if (source.kind === "typed") return source.phones.map((phone) => ({ phone }));
  const { table, phoneColumn } = source;
  if (phoneColumn === null) return [];
  const cell = (r: readonly string[], i: number) => r[i] ?? "";
  if (channel === "sms" && sms !== null) {
    if (sms.mode === "cot-trong-tep") {
      const col = sms.contentColumn;
      return table.rows.map((r) => (col === null ? { phone: cell(r, phoneColumn) } : { phone: cell(r, phoneColumn), content: cell(r, col) }));
    }
    const mapped = detectVariables(sms.content).flatMap((n) => {
      const col = sms.variableColumns[n];
      return col === null || col === undefined ? [] : [[variableKey(n), col] as const];
    });
    return table.rows.map((r) =>
      mapped.length === 0
        ? { phone: cell(r, phoneColumn) }
        : { phone: cell(r, phoneColumn), values: Object.fromEntries(mapped.map(([k, col]) => [k, cell(r, col)])) },
    );
  }
  const mapped = Object.entries(paramColumns).filter((e): e is [string, number] => e[1] !== null);
  return table.rows.map((r) =>
    mapped.length === 0
      ? { phone: cell(r, phoneColumn) }
      : { phone: cell(r, phoneColumn), values: Object.fromEntries(mapped.map(([k, col]) => [k, cell(r, col)])) },
  );
}

/** The header a column is recorded under: row 1's text; an empty header as "Cột A" (it has no other name). */
export function headerName(headers: readonly string[], i: number): string {
  const h = headers[i] ?? "";
  return h === "" ? `Cột ${columnLetter(i)}` : h;
}

/**
 * Which column fed what, for the campaign's history (owner 17/10/2026: "lịch sử nên lưu thông tin map
 * biến"). File source only — a typed list has no columns, so `undefined`. Only the parts the composition
 * USES are present: `content_column` for `cot-trong-tep`, `variables` for the detected, mapped
 * `{{variable_N}}` of `noi-dung-chung`, `params` for ZNS parameters taken from a column.
 */
export function buildMapping(
  source: ImportedSource,
  channel: "zns" | "sms",
  sms: SmsImportComposition | null,
  paramColumns: ParamColumns,
): NoticeRecipientMapping | undefined {
  if (source.kind !== "file" || source.phoneColumn === null) return undefined;
  const { headers } = source.table;
  const mapping: NoticeRecipientMapping = { phone_column: headerName(headers, source.phoneColumn) };
  if (channel === "sms" && sms !== null) {
    if (sms.mode === "cot-trong-tep") {
      if (sms.contentColumn !== null) mapping.content_column = headerName(headers, sms.contentColumn);
      return mapping;
    }
    const vars = detectVariables(sms.content).flatMap((n) => {
      const col = sms.variableColumns[n];
      return col === null || col === undefined ? [] : [[variableKey(n), headerName(headers, col)] as const];
    });
    if (vars.length > 0) mapping.variables = Object.fromEntries(vars);
    return mapping;
  }
  const params = Object.entries(paramColumns).filter((e): e is [string, number] => e[1] !== null);
  if (params.length > 0) mapping.params = Object.fromEntries(params.map(([k, col]) => [k, headerName(headers, col)]));
  return mapping;
}

/**
 * Everything the officer must fix before a preview is asked — the client's copy of comms' limits.
 * Unmapped variables / parameters are NOT here: comms reports them as `missing_params`, the same way an
 * empty template slot is reported (one place decides what is missing).
 */
export function importErrors(
  source: ImportedSource,
  channel: "zns" | "sms",
  sms: SmsImportComposition | null,
  rows: readonly NoticeRecipientRow[],
): string[] {
  const errors: string[] = [];
  if (source.kind === "typed") {
    const e = typedPhonesError(source.phones);
    if (e !== null) errors.push(e);
  } else if (source.phoneColumn === null) {
    errors.push(PHONE_COLUMN_MISSING);
  }
  if (channel !== "sms" || sms === null) return errors;

  if (source.kind === "file" && sms.mode === "cot-trong-tep") {
    if (sms.contentColumn === null) {
      errors.push(CONTENT_COLUMN_MISSING);
      return errors;
    }
    const empty: number[] = [];
    const long: number[] = [];
    rows.forEach((r, i) => {
      const text = r.content ?? "";
      if (text.trim() === "") empty.push(i);
      else if (text.length > MAX_CONTENT_LENGTH) long.push(i);
    });
    if (empty.length > 0) errors.push(`Ô nội dung trống ở dòng ${sheetRows(empty)} của tệp.`);
    if (long.length > 0) errors.push(`Nội dung quá ${vi(MAX_CONTENT_LENGTH)} ký tự ở dòng ${sheetRows(long)} của tệp.`);
    return errors;
  }

  // One shared text (typed list, or file "noi-dung-chung").
  if (sms.content.trim() === "") {
    errors.push(CONTENT_EMPTY);
    return errors;
  }
  if (sms.content.length > MAX_CONTENT_LENGTH) errors.push(contentTooLong(sms.content.length));
  const vars = detectVariables(sms.content);
  if (source.kind === "typed") {
    if (vars.length > 0) errors.push(TYPED_NO_VARIABLES);
    return errors;
  }
  if (vars.length > 0) {
    const long: number[] = [];
    rows.forEach((r, i) => {
      if (r.values !== undefined && fillVariables(sms.content, r.values).length > MAX_CONTENT_LENGTH) long.push(i);
    });
    if (long.length > 0) {
      errors.push(`Nội dung sau khi thay biến quá ${vi(MAX_CONTENT_LENGTH)} ký tự ở dòng ${sheetRows(long)} của tệp.`);
    }
  }
  return errors;
}

/** The campaign's default title when no template names it: the shared text cut, or a plain description. */
export function importTitle(sms: SmsImportComposition, sourceKind: "typed" | "file"): string {
  const text = sms.content.trim();
  if ((sms.mode === "noi-dung-chung" || sourceKind === "typed") && text !== "") {
    return text.length <= 60 ? text : `${text.slice(0, 60).trimEnd()}…`;
  }
  return "Tin SMS nội dung theo tệp";
}
