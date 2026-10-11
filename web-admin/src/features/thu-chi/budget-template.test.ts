// `Tải file mẫu` of `Nạp từ Excel` (customer bug sheet row 49): the workbook is built with the REAL
// library (write-excel-file), unzipped, and its cells read back — then checked against the rules
// `service-finance/internal/domain/budget_import.go` applies to every uploaded sheet. The Go parser
// itself cannot run here; `acceptedShape` restates only the rules a TEMPLATE can break, each with
// the line of the parser it mirrors, so a template that would be refused fails this test.

import { strFromU8, unzipSync } from "fflate";
import { describe, expect, it } from "vitest";

import type { finance_bangDayDuRa } from "@/lib/api/schema.gen";

import {
  budgetTemplateFileName,
  budgetTemplateFor,
  budgetTemplateRows,
  buildBudgetTemplateXlsx,
  type BudgetTemplate,
} from "./budget-template";

/** Column letters → 0-based index (`A` 0, `AA` 26). */
function columnIndex(ref: string): number {
  let n = 0;
  for (const ch of ref.replace(/\d+$/, "")) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n - 1;
}

function unescapeXml(s: string): string {
  return s
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&amp;/g, "&");
}

/** The first worksheet's text, row 1 first, as `core/xlsx.ReadSheets` hands it to the parser. */
async function readBack(blob: Blob): Promise<{ sheetName: string; rows: string[][] }> {
  const files = unzipSync(new Uint8Array(await blob.arrayBuffer()));
  const text = (name: string) => (files[name] === undefined ? "" : strFromU8(files[name]!));
  const shared = [...text("xl/sharedStrings.xml").matchAll(/<si>([\s\S]*?)<\/si>/g)].map((m) =>
    unescapeXml([...m[1]!.matchAll(/<t[^>]*>([\s\S]*?)<\/t>/g)].map((t) => t[1]).join("")),
  );
  const sheetName = unescapeXml(/<sheet [^>]*name="([^"]*)"/.exec(text("xl/workbook.xml"))?.[1] ?? "");
  const rows: string[][] = [];
  for (const row of text("xl/worksheets/sheet1.xml").matchAll(/<row [^>]*r="(\d+)"[^>]*>([\s\S]*?)<\/row>/g)) {
    const r = Number(row[1]) - 1;
    const cells: string[] = [];
    for (const c of row[2]!.matchAll(/<c ([^>]*?)(?:\/>|>([\s\S]*?)<\/c>)/g)) {
      const attrs = c[1]!;
      const ref = /r="([A-Z]+\d+)"/.exec(attrs)![1]!;
      const body = c[2] ?? "";
      const type = /t="([^"]*)"/.exec(attrs)?.[1];
      let v = "";
      if (type === "s") v = shared[Number(/<v>(\d+)<\/v>/.exec(body)?.[1])] ?? "";
      else if (type === "inlineStr") v = unescapeXml([...body.matchAll(/<t[^>]*>([\s\S]*?)<\/t>/g)].map((t) => t[1]).join(""));
      else v = unescapeXml(/<v>([\s\S]*?)<\/v>/.exec(body)?.[1] ?? "");
      cells[columnIndex(ref)] = v;
    }
    rows[r] = Array.from(cells, (x) => x ?? "");
  }
  return { sheetName, rows: Array.from(rows, (x) => x ?? []) };
}

const oneLine = (s: string) => s.split(/\s+/).filter((w) => w !== "").join(" ");

/**
 * What the importer reads from a sheet, or the sentence it would refuse it with — the rules a template
 * can break, from `parseBudgetSheet` (budget_import.go):
 *   header   first row of the first 20 whose column A is `TT`/`STT` (:252-259)
 *   columns  every non-empty heading from column C (:270-286); `%`/`tỷ lệ`/`tỉ lệ` = percentage (:281)
 *   unit     a "Đơn vị tính: …" line ABOVE the header (:299-311), one of the closed list (:337)
 *   kind     "thu"/"chi" as words in the headings above (columns A–C), then the tab name (:312, :539)
 */
function acceptedShape(sheetName: string, rows: string[][]) {
  const header = rows.slice(0, 20).findIndex((r) => {
    const a = (r[0] ?? "").trim().replace(/\.+$/, "").toLowerCase();
    return a === "tt" || a === "stt";
  });
  if (header < 0) return { error: "no header" } as const;
  const columns = (rows[header] ?? [])
    .slice(2)
    .map((h) => oneLine(h))
    .filter((h) => h !== "")
    .map((name) => {
      const low = name.toLowerCase();
      return { name, percent: low.includes("%") || low.includes("tỷ lệ") || low.includes("tỉ lệ") };
    });
  const headings: string[] = [];
  let unit: string | null = null;
  for (let r = 0; r < header; r++) {
    (rows[r] ?? []).forEach((cell, c) => {
      const t = oneLine(cell);
      if (t === "") return;
      const low = t.toLowerCase();
      if (low.includes("đơn vị tính") || low.startsWith("đvt")) {
        unit ??= t.slice(t.indexOf(":") + 1).trim();
        return;
      }
      if (c <= 2) headings.push(t);
    });
  }
  if (unit === null || !["đồng", "nghìn đồng", "triệu đồng"].includes((unit as string).toLowerCase())) {
    return { error: "unit" } as const;
  }
  let kind: "thu" | "chi" | null = null;
  let title = "";
  for (const [i, text] of [...headings, sheetName].entries()) {
    const words = text.toLowerCase().split(/[^\p{L}]+/u);
    const thu = words.indexOf("thu");
    const chi = words.indexOf("chi");
    if (thu >= 0 && (chi < 0 || thu < chi)) kind = "thu";
    else if (chi >= 0) kind = "chi";
    if (kind !== null) {
      title = i < headings.length ? text : "";
      break;
    }
  }
  if (kind === null) return { error: "kind" } as const;
  return { kind, title, unit: unit as string, columns, firstDataRow: header + 1 } as const;
}

const THU_DEFAULT = budgetTemplateFor("thu", 2026, null);
const CHI_DEFAULT = budgetTemplateFor("chi", 2026, null);

describe("the template round-trips through the importer's rules", () => {
  for (const t of [THU_DEFAULT, CHI_DEFAULT]) {
    it(`${t.kind}: the built .xlsx reads back as the same sheet, with every column`, async () => {
      const { sheetName, rows } = await readBack(await buildBudgetTemplateXlsx(t));
      expect(sheetName).toBe(t.kind === "chi" ? "Chi" : "Thu");
      expect(rows.map((r) => r.join("|"))).toEqual(budgetTemplateRows(t).map((r) => r.join("|")));
      const shape = acceptedShape(sheetName, rows);
      expect(shape).toMatchObject({ kind: t.kind, title: t.title, unit: "Triệu đồng", firstDataRow: 3 });
      if (shape.columns === undefined) throw new Error("refused");
      expect(shape.columns.map((c) => c.name)).toEqual(t.columns.map((c) => c.name));
      expect(shape.columns.map((c) => c.percent)).toEqual(t.columns.map((c) => c.type === "phan_tram"));
    });
  }

  it("thu default: TT, Nội dung, then the customer's five columns (bug sheet rows 55–56)", () => {
    expect(budgetTemplateRows(THU_DEFAULT)[2]).toEqual([
      "TT",
      "Nội dung",
      "Dự toán 2026 TP giao",
      "Dự toán 2026 Xã giao",
      "Thu ngân sách NSNN",
      "Thu ngân sách Thu xã hưởng",
      "Tỷ lệ % thu",
    ]);
  });
});

describe("the template follows the sheet on screen", () => {
  const LOADED = {
    sheet: {
      id: "b1",
      code: "NS-2026-CHI-01",
      year: 2026,
      kind: "chi",
      revision: 1,
      title: "Báo cáo chi ngân sách năm 2026",
      unit: "nghin-dong",
      unit_label: "Nghìn đồng",
    },
    columns: [
      { id: "c2", name: "Thực hiện", order: 2, type: "so", numerator_column_id: null, denominator_column_id: null },
      { id: "c3", name: "So sánh", order: 3, type: "phan_tram", numerator_column_id: "c2", denominator_column_id: "c1" },
      { id: "c1", name: "Dự toán", order: 1, type: "so", numerator_column_id: null, denominator_column_id: null },
    ],
    lines: [],
    summary: {},
  } as unknown as finance_bangDayDuRa;

  it("its title, its unit, its columns in display order; a % column the server would misread gets (%)", async () => {
    const t = budgetTemplateFor("chi", 2026, LOADED);
    expect(budgetTemplateRows(t)).toEqual([
      ["Báo cáo chi ngân sách năm 2026"],
      ["Đơn vị tính: Nghìn đồng"],
      ["TT", "Nội dung", "Dự toán", "Thực hiện", "So sánh (%)"],
    ]);
    const { sheetName, rows } = await readBack(await buildBudgetTemplateXlsx(t));
    expect(acceptedShape(sheetName, rows)).toMatchObject({
      kind: "chi",
      unit: "Nghìn đồng",
      columns: [
        { name: "Dự toán", percent: false },
        { name: "Thực hiện", percent: false },
        { name: "So sánh (%)", percent: true },
      ],
    });
  });

  it("a sheet of ANOTHER year or kind is not used: the defaults are", () => {
    expect(budgetTemplateFor("thu", 2026, LOADED)).toEqual(THU_DEFAULT);
    expect(budgetTemplateFor("chi", 2025, LOADED).year).toBe(2025);
    expect(budgetTemplateFor("chi", 2025, LOADED).columns).toEqual(budgetTemplateFor("chi", 2025, null).columns);
  });

  it("a title naming the other kind first would file the import wrongly: the heading falls back", async () => {
    const t: BudgetTemplate = { ...CHI_DEFAULT, title: "Báo cáo thu, chi ngân sách" };
    expect(budgetTemplateRows(t)[0]).toEqual(["BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026"]);
    const { sheetName, rows } = await readBack(await buildBudgetTemplateXlsx(t));
    expect(acceptedShape(sheetName, rows)).toMatchObject({ kind: "chi" });
  });

  it("a title naming neither kind is kept: the tab name decides", async () => {
    const t: BudgetTemplate = { ...THU_DEFAULT, title: "Báo cáo ngân sách xã" };
    const { sheetName, rows } = await readBack(await buildBudgetTemplateXlsx(t));
    expect(rows[0]).toEqual(["Báo cáo ngân sách xã"]);
    expect(acceptedShape(sheetName, rows)).toMatchObject({ kind: "thu", title: "" });
  });

  it("the file name is ASCII, kind and year only", () => {
    expect(budgetTemplateFileName("chi", 2026)).toBe("mau-nap-chi-ngan-sach-2026.xlsx");
  });
});
