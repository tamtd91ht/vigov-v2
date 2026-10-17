// @vitest-environment jsdom
//
// The imported sources' pure half (ADR 0099 §Sửa đổi S1–S4). The failures that matter are silent: a CSV
// quote splitting a text in two, a BOM glued to the first header, a variable left in the text that no
// column fills, a row whose phone came from the wrong column, a number shown unmasked.

import { describe, expect, it } from "vitest";

import {
  buildMapping,
  buildRows,
  columnLabel,
  columnLetter,
  CONTENT_COLUMN_MISSING,
  CONTENT_EMPTY,
  detectVariables,
  FILE_NO_HEADER,
  FILE_NO_ROWS,
  FILE_NOT_UTF8,
  FILE_TOO_BIG,
  FILE_WRONG_TYPE,
  fileTooManyRows,
  fillVariables,
  guessPhoneColumn,
  importErrors,
  maskedPreview,
  maskPhone,
  MAX_FILE_BYTES,
  MAX_FILE_ROWS,
  MAX_TYPED_PHONES,
  parseCsv,
  PHONE_COLUMN_MISSING,
  readRecipientFile,
  splitTypedPhones,
  tableFromCells,
  TYPED_EMPTY,
  TYPED_NO_VARIABLES,
  typedPhonesError,
  type ImportedSource,
  type ImportedTable,
  type SmsImportComposition,
} from "./recipient-import";

const TABLE: ImportedTable = {
  fileName: "ds.csv",
  headers: ["Họ tên", "Số điện thoại", "Ngày", "Nội dung"],
  rows: [
    ["Nguyễn A", "0900000000", "20/10", "Mời họp tổ 1"],
    ["Trần B", "0900000001", "21/10", "Mời họp tổ 2"],
  ],
};
const FILE: ImportedSource = { kind: "file", table: TABLE, phoneColumn: 1 };
const sms = (over: Partial<SmsImportComposition> = {}): SmsImportComposition => ({
  mode: "noi-dung-chung",
  contentColumn: null,
  content: "",
  variableColumns: {},
  ...over,
});

describe("parseCsv", () => {
  it("quotes, escaped quotes, commas and line breaks inside quotes, CRLF, a BOM, no trailing empty row", () => {
    const text = '﻿Họ tên,Số điện thoại,Nội dung\r\n"Nguyễn, A",0900000000,"Nói ""chào""\r\ndòng 2"\r\nTrần B,0900000001,x\r\n';
    expect(parseCsv(text)).toEqual([
      ["Họ tên", "Số điện thoại", "Nội dung"],
      ["Nguyễn, A", "0900000000", 'Nói "chào"\r\ndòng 2'],
      ["Trần B", "0900000001", "x"],
    ]);
  });

  it("LF and lone CR line ends; empty fields kept; the last line without a break", () => {
    expect(parseCsv("a,b\n1,\r2,3")).toEqual([
      ["a", "b"],
      ["1", ""],
      ["2", "3"],
    ]);
  });

  it("`;` when the first line has more semicolons than commas outside quotes", () => {
    expect(parseCsv('Tên;SĐT;Ghi chú\n"A, B";0900000000;x,y')).toEqual([
      ["Tên", "SĐT", "Ghi chú"],
      ["A, B", "0900000000", "x,y"],
    ]);
  });
});

describe("the typed list", () => {
  it("one per line or comma; blanks dropped; nothing normalised here", () => {
    expect(splitTypedPhones(" 0900000000\n\n0900000001, +84900000002 ,\r\n")).toEqual(["0900000000", "0900000001", "+84900000002"]);
  });

  it("empty and over 1.000 are refused with a sentence; 1.000 exactly passes", () => {
    expect(typedPhonesError([])).toBe(TYPED_EMPTY);
    expect(typedPhonesError(Array<string>(MAX_TYPED_PHONES).fill("0900000000"))).toBeNull();
    expect(typedPhonesError(Array<string>(MAX_TYPED_PHONES + 1).fill("0900000000"))).toBe(
      "Tối đa 1.000 số mỗi lần nhập — đang có 1.001 số. Dùng Nhập tệp cho danh sách dài hơn.",
    );
  });
});

describe("tableFromCells", () => {
  it("row 1 = headers; empty rows dropped; every row padded to the widest; cells trimmed", () => {
    const r = tableFromCells("a.csv", [["SĐT ", "Tên"], ["", ""], ["0900000000", "A", "thừa"], [" 0900000001"]]);
    expect(r).toEqual({
      ok: true,
      table: {
        fileName: "a.csv",
        headers: ["SĐT", "Tên", ""],
        rows: [
          ["0900000000", "A", "thừa"],
          ["0900000001", "", ""],
        ],
      },
    });
  });

  it("no header, no data, too many rows: each its own sentence", () => {
    expect(tableFromCells("a", [])).toEqual({ ok: false, error: FILE_NO_HEADER });
    expect(tableFromCells("a", [["", ""], ["0900000000"]])).toEqual({ ok: false, error: FILE_NO_HEADER });
    expect(tableFromCells("a", [["SĐT"], [""]])).toEqual({ ok: false, error: FILE_NO_ROWS });
    const many = [["SĐT"], ...Array.from({ length: MAX_FILE_ROWS + 1 }, () => ["0900000000"])];
    expect(tableFromCells("a", many)).toEqual({ ok: false, error: fileTooManyRows(MAX_FILE_ROWS + 1) });
    expect(fileTooManyRows(10_001)).toBe("Tệp có 10.001 dòng dữ liệu, quá giới hạn 10.000 dòng. Chia thành nhiều tệp.");
    expect(tableFromCells("a", [["SĐT"], ...Array.from({ length: MAX_FILE_ROWS }, () => ["0900000000"])]).ok).toBe(true);
  });
});

describe("readRecipientFile — in the browser, never uploaded", () => {
  it("wrong type and too big are refused before reading", async () => {
    expect(await readRecipientFile(new File(["x"], "ds.xls"))).toEqual({ ok: false, error: FILE_WRONG_TYPE });
    const big = new File([new Uint8Array(MAX_FILE_BYTES + 1)], "ds.csv");
    expect(await readRecipientFile(big)).toEqual({ ok: false, error: FILE_TOO_BIG });
  });

  it("CSV: UTF-8 with a BOM is read; a non-UTF-8 file (Excel's ANSI CSV) is refused, not garbled", async () => {
    const ok = await readRecipientFile(new File(["﻿Số điện thoại,Tên\n0900000000,Bà Lê\n"], "DS.CSV"));
    expect(ok).toEqual({ ok: true, table: { fileName: "DS.CSV", headers: ["Số điện thoại", "Tên"], rows: [["0900000000", "Bà Lê"]] } });
    const ansi = new File([new Uint8Array([0x53, 0x44, 0x54, 0x0a, 0xe0, 0xe1, 0x0a])], "ds.csv");
    expect(await readRecipientFile(ansi)).toEqual({ ok: false, error: FILE_NOT_UTF8 });
  });

  it("XLSX: the first sheet, numbers kept as the stored text (no float)", async () => {
    const { default: writeXlsxFile } = await import("write-excel-file/universal");
    const blob = await writeXlsxFile([
      [{ value: "Số điện thoại" }, { value: "Mã" }],
      [{ value: "0900000000" }, { value: 12345678901234 }],
    ]).toBlob();
    const r = await readRecipientFile(new File([blob], "ds.xlsx"));
    expect(r).toEqual({ ok: true, table: { fileName: "ds.xlsx", headers: ["Số điện thoại", "Mã"], rows: [["0900000000", "12345678901234"]] } });
  });
});

describe("columns and masking", () => {
  it("spreadsheet letters, labels, a guessed phone column only when exactly one header names it", () => {
    expect([0, 25, 26, 51, 52].map(columnLetter)).toEqual(["A", "Z", "AA", "AZ", "BA"]);
    expect(columnLabel(["Tên", ""], 0)).toBe("Tên (cột A)");
    expect(columnLabel(["Tên", ""], 1)).toBe("Cột B");
    expect(guessPhoneColumn(TABLE.headers)).toBe(1);
    expect(guessPhoneColumn(["SĐT nhà", "Số điện thoại"])).toBeNull();
    expect(guessPhoneColumn(["Tên"])).toBeNull();
  });

  it("maskPhone is comms' shape; the preview masks the phone column AND any phone-looking cell", () => {
    expect(maskPhone("0900000000")).toBe("09****0000");
    expect(maskPhone("1234567")).toBe("*******");
    const t: ImportedTable = { fileName: "x", headers: ["A", "B", "C"], rows: [["0900000001", "090 000 0002", "abc"]] };
    expect(maskedPreview(t, null)).toEqual([["09****0001", "09******0002", "abc"]]);
    const preview = maskedPreview(TABLE, 1);
    expect(preview).toHaveLength(2);
    expect(JSON.stringify(preview)).not.toMatch(/0\d{9}/);
  });
});

describe("{{variable_N}}", () => {
  it("detects the distinct N ascending; ignores look-alikes", () => {
    expect(detectVariables("Mời {{variable_2}} họp ngày {{variable_1}}, {{variable_2}} nhé")).toEqual([1, 2]);
    expect(detectVariables("{{variable_0}} {{ variable_1 }} {{variable_x}} {variable_3}")).toEqual([]);
    expect(detectVariables("{{variable_10}}")).toEqual([10]);
  });

  it("fills the mapped ones, leaves the others as written", () => {
    expect(fillVariables("A {{variable_1}} B {{variable_2}}", { variable_1: "x" })).toBe("A x B {{variable_2}}");
  });
});

describe("buildRows — what the POST carries per mode", () => {
  it("typed: {phone} only, as typed", () => {
    expect(buildRows({ kind: "typed", phones: ["0900000000", "+84900000001"] }, "sms", sms({ content: "Chào" }), {})).toEqual([
      { phone: "0900000000" },
      { phone: "+84900000001" },
    ]);
  });

  it("file, SMS from a column: {phone, content: cell}", () => {
    expect(buildRows(FILE, "sms", sms({ mode: "cot-trong-tep", contentColumn: 3 }), {})).toEqual([
      { phone: "0900000000", content: "Mời họp tổ 1" },
      { phone: "0900000001", content: "Mời họp tổ 2" },
    ]);
  });

  it("file, SMS shared text: {phone, values: {variable_N: cell}} for the detected, mapped N only", () => {
    const c = sms({ content: "Chào {{variable_1}}, họp ngày {{variable_2}}", variableColumns: { 1: 0, 2: 2, 5: 3 } });
    expect(buildRows(FILE, "sms", c, {})).toEqual([
      { phone: "0900000000", values: { variable_1: "Nguyễn A", variable_2: "20/10" } },
      { phone: "0900000001", values: { variable_1: "Trần B", variable_2: "21/10" } },
    ]);
    expect(buildRows(FILE, "sms", sms({ content: "Không biến" }), {})).toEqual([{ phone: "0900000000" }, { phone: "0900000001" }]);
  });

  it("file, ZNS: {phone, values: {<field>: cell}} for parameters mapped to a column; shared ones stay out", () => {
    expect(buildRows(FILE, "zns", null, { ngay: 2, ten: null })).toEqual([
      { phone: "0900000000", values: { ngay: "20/10" } },
      { phone: "0900000001", values: { ngay: "21/10" } },
    ]);
  });

  it("no phone column chosen: no rows", () => {
    expect(buildRows({ ...FILE, phoneColumn: null }, "zns", null, {})).toEqual([]);
  });
});

describe("buildMapping — the column names the history keeps", () => {
  it("SMS column / SMS shared / ZNS; headers by name, an empty header as 'Cột X'; typed has none", () => {
    expect(buildMapping(FILE, "sms", sms({ mode: "cot-trong-tep", contentColumn: 3 }), {})).toEqual({
      phone_column: "Số điện thoại",
      content_column: "Nội dung",
    });
    expect(buildMapping(FILE, "sms", sms({ content: "{{variable_1}} {{variable_2}}", variableColumns: { 1: 0, 2: null } }), {})).toEqual({
      phone_column: "Số điện thoại",
      variables: { variable_1: "Họ tên" },
    });
    expect(buildMapping(FILE, "zns", null, { ngay: 2, ten: null })).toEqual({ phone_column: "Số điện thoại", params: { ngay: "Ngày" } });
    expect(buildMapping(FILE, "zns", null, {})).toEqual({ phone_column: "Số điện thoại" });
    const blank: ImportedSource = { kind: "file", table: { ...TABLE, headers: ["", "", "", ""] }, phoneColumn: 1 };
    expect(buildMapping(blank, "zns", null, {})).toEqual({ phone_column: "Cột B" });
    expect(buildMapping({ kind: "typed", phones: ["0900000000"] }, "sms", sms(), {})).toBeUndefined();
    expect(buildMapping({ ...FILE, phoneColumn: null }, "zns", null, {})).toBeUndefined();
  });
});

describe("importErrors — the sentences before a preview", () => {
  it("file without a phone column; SMS without a content column", () => {
    const noPhone: ImportedSource = { ...FILE, phoneColumn: null };
    expect(importErrors(noPhone, "zns", null, [])).toEqual([PHONE_COLUMN_MISSING]);
    expect(importErrors(FILE, "sms", sms({ mode: "cot-trong-tep" }), [])).toEqual([CONTENT_COLUMN_MISSING]);
  });

  it("column content: empty cells and over-1.000 cells named by their SHEET row (data row + 2)", () => {
    const rows = [
      { phone: "0900000000", content: "" },
      { phone: "0900000001", content: "x".repeat(1001) },
      { phone: "0900000002", content: "ok" },
    ];
    expect(importErrors(FILE, "sms", sms({ mode: "cot-trong-tep", contentColumn: 3 }), rows)).toEqual([
      "Ô nội dung trống ở dòng 2 của tệp.",
      "Nội dung quá 1.000 ký tự ở dòng 3 của tệp.",
    ]);
  });

  it("shared content: empty, too long, variables in a typed list, too long after filling", () => {
    const typed: ImportedSource = { kind: "typed", phones: ["0900000000"] };
    expect(importErrors(typed, "sms", sms(), [])).toEqual([CONTENT_EMPTY]);
    expect(importErrors(typed, "sms", sms({ content: "x".repeat(1001) }), [])).toEqual(["Nội dung dài 1.001 ký tự, quá giới hạn 1.000 ký tự."]);
    expect(importErrors(typed, "sms", sms({ content: "Chào {{variable_1}}" }), [])).toEqual([TYPED_NO_VARIABLES]);
    const c = sms({ content: `${"x".repeat(985)}{{variable_1}}`, variableColumns: { 1: 0 } });
    const rows = [{ phone: "0900000000", values: { variable_1: "ngắn" } }, { phone: "0900000001", values: { variable_1: "y".repeat(20) } }];
    expect(importErrors(FILE, "sms", c, rows)).toEqual(["Nội dung sau khi thay biến quá 1.000 ký tự ở dòng 3 của tệp."]);
  });

  it("an unmapped variable is NOT a client error — comms reports it as missing_params", () => {
    const c = sms({ content: "Chào {{variable_1}}" });
    expect(importErrors(FILE, "sms", c, buildRows(FILE, "sms", c, {}))).toEqual([]);
  });

  it("typed list: the typed refusal; ZNS has no content checks", () => {
    expect(importErrors({ kind: "typed", phones: [] }, "zns", null, [])).toEqual([TYPED_EMPTY]);
    expect(importErrors({ kind: "typed", phones: ["0900000000"] }, "zns", null, [{ phone: "0900000000" }])).toEqual([]);
  });
});
