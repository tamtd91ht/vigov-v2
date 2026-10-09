/**
 * Fixture data of `/xem-thu/thu-chi` — DEV ONLY (ADR 0068 lần 6 #10). A plain module (no "use client"),
 * so the server page reads its query words as values.
 *
 * The board's reads (`budget-sheets`, `budget-indicators`, `budget-period-closes`) and the
 * `Nạp từ Excel` route, answered by `budget-preview.tsx`. `?case=` picks the import answer, in the
 * server's own statuses and bodies (`budget_import.go`), so each toast can be screenshotted:
 *   ok       201, two sheets (chi replaces the live sheet, thu is new)
 *   closed   409 `budget_period_closed`
 *   entries  409 `budget_sheet_has_entries`
 *   errors   400 `import_invalid` with row errors
 * The `ok` answer writes nothing: the board re-reads the same fixture sheet afterwards.
 */

import type {
  finance_bangDayDuRa,
  finance_budgetImportOut,
  finance_budgetImportSheetOut,
  finance_budgetPeriodClosesOut,
  finance_chiSoNamRa,
  finance_cotRa,
  finance_dongRa,
} from "@/lib/api/schema.gen";

export const BUDGET_PREVIEW_MODALS = ["nap-excel"] as const;
export type BudgetPreviewModal = (typeof BUDGET_PREVIEW_MODALS)[number];
export const BUDGET_PREVIEW_CASES = [
  "ok",
  "closed",
  "entries",
  "errors",
] as const;
export type BudgetPreviewCase = (typeof BUDGET_PREVIEW_CASES)[number];

function first(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

/** An unknown word opens nothing — never a guessed dialog. */
export function budgetPreviewModal(
  v: string | string[] | undefined,
): BudgetPreviewModal | null {
  return BUDGET_PREVIEW_MODALS.find((x) => x === first(v)) ?? null;
}

/** Read on the client by the answering wrapper; unknown or absent: `ok`. */
export function budgetPreviewCase(
  v: string | null | undefined,
): BudgetPreviewCase {
  return BUDGET_PREVIEW_CASES.find((x) => x === v) ?? "ok";
}

const CHI_COLUMNS: finance_cotRa[] = [
  {
    id: "C1",
    name: "Dự toán năm",
    order: 1,
    type: "so",
    role: "du-toan-nam",
    numerator_column_id: null,
    denominator_column_id: null,
  },
  {
    id: "C2",
    name: "Chi ngân sách",
    order: 2,
    type: "so",
    role: "chi-ngan-sach",
    numerator_column_id: null,
    denominator_column_id: null,
  },
  {
    id: "C3",
    name: "So sánh TH/DT (%)",
    order: 3,
    type: "phan_tram",
    numerator_column_id: "C2",
    denominator_column_id: "C1",
  },
];

/** The fixture lines that have children (`A` → `A1`, `A2`). */
const PARENT_LINES: ReadonlySet<string> = new Set(["A"]);

function line(
  id: string,
  no: string,
  name: string,
  order: number,
  c1: number,
  c2: number,
  parent?: string,
): finance_dongRa {
  return {
    id,
    parent_id: parent,
    no,
    name,
    order,
    // Only a line WITH children sums them; every other line is typed (`manual`) — so the preview shows
    // a parent (disabled select, no entries) beside leaves at the top level and below.
    method: PARENT_LINES.has(id) ? "children" : "manual",
    level: parent === undefined ? 0 : 1,
    is_headline: id === "L0",
    values: { C1: c1, C2: c2 },
    percent_basis_points: {
      C3: c1 === 0 ? null : Math.round((c2 * 10000) / c1),
    },
  };
}

/** The live chi sheet of the year — as loaded from a file earlier (source file shown). */
export function previewBudgetSheet(
  year: number,
  kind: string,
): finance_bangDayDuRa | null {
  if (kind !== "chi") return null;
  const lines = [
    line("L0", "", "Tổng số", 0, 5502660000000, 3401673300000),
    line("A", "A", "Chi cân đối ngân sách", 1, 4802660000000, 3001673300000),
    line(
      "A1",
      "I",
      "Chi đầu tư phát triển",
      1,
      1802660000000,
      1101673300000,
      "A",
    ),
    line("A2", "II", "Chi thường xuyên", 2, 3000000000000, 1900000000000, "A"),
    line("B", "B", "Chi chương trình mục tiêu", 2, 700000000000, 400000000000),
  ];
  return {
    sheet: {
      id: "01PREVIEWBUDGETCHI0000000",
      code: `NS-${year}-CHI-01`,
      year,
      kind: "chi",
      revision: 1,
      title: `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM ${year}`,
      unit: "trieu-dong",
      unit_label: "Triệu đồng",
      cumulative_to: `${year}-08-25`,
      source_file: "bao-cao-thu-chi-thang-8.xlsx",
    },
    columns: CHI_COLUMNS,
    lines,
    summary: {
      headline_line_id: "L0",
      cells: [
        {
          column_id: "C1",
          name: "Dự toán năm",
          role: "du-toan-nam",
          value: 5502660000000,
        },
        {
          column_id: "C2",
          name: "Chi ngân sách",
          role: "chi-ngan-sach",
          value: 3401673300000,
        },
      ],
      indicator: { name: "Chi đạt dự toán", basis_points: 6182 },
    },
  };
}

export function previewBudgetIndicators(year: number): finance_chiSoNamRa {
  const none = "ngan_sach: chưa có bảng thu của năm này";
  return {
    year,
    revenue_achievement: {
      name: "Thu đạt dự toán",
      basis_points: null,
      unavailable_reason: none,
    },
    expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 6182 },
    balance: { amount: null, unavailable_reason: none },
    revenue_totals: [],
  };
}

export function previewBudgetCloses(
  year: number,
): finance_budgetPeriodClosesOut {
  return { year, closes: [] };
}

const SOURCE_FILE = "bao-cao-thu-chi-ngan-sach.xlsx";

function chiSheet(
  year: number,
  over: Partial<finance_budgetImportSheetOut> = {},
): finance_budgetImportSheetOut {
  return {
    sheet_name: "Chi NS",
    kind: "chi",
    title: `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM ${year}`,
    unit: "trieu-dong",
    unit_label: "Triệu đồng",
    line_count: 59,
    columns: [
      {
        name: "Dự toán năm",
        type: "so",
        role: "du-toan-nam",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: "Chi ngân sách",
        type: "so",
        role: "chi-ngan-sach",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: "So sánh TH/DT (%)",
        type: "phan_tram",
        numerator_name: "Chi ngân sách",
        denominator_name: "Dự toán năm",
      },
    ],
    headline: { no: "", name: "Tổng số" },
    action: "replace",
    replaces_code: `NS-${year}-CHI-01`,
    warnings: [],
    ...over,
  };
}

function thuSheet(year: number): finance_budgetImportSheetOut {
  return {
    sheet_name: "Thu NS",
    kind: "thu",
    title: `BÁO CÁO THU NGÂN SÁCH NHÀ NƯỚC NĂM ${year}`,
    unit: "trieu-dong",
    unit_label: "Triệu đồng",
    line_count: 31,
    columns: [
      {
        name: `Dự toán ${year} TP giao`,
        type: "so",
        role: "du-toan-tp-giao",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: `Dự toán ${year} Xã giao`,
        type: "so",
        role: "du-toan-xa-giao",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: "Thu ngân sách NSNN",
        type: "so",
        role: "thu-nsnn",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: "Thu ngân sách Thu xã hưởng",
        type: "so",
        role: "thu-xa-huong",
        numerator_name: null,
        denominator_name: null,
      },
      {
        name: "Tỷ lệ % thu",
        type: "phan_tram",
        numerator_name: null,
        denominator_name: null,
      },
    ],
    headline: { no: "", name: "Tổng thu ngân sách nhà nước trên địa bàn" },
    action: "create",
    warnings: [
      {
        sheet: "Thu NS",
        row: 0,
        column: "Tỷ lệ % thu",
        message:
          "Không xác định được hai cột của tỷ lệ — các ô sẽ hiện Không tính được.",
      },
    ],
  };
}

/** What `POST /api/v1/budget-sheets/imports` answers for one `?case=`: status and body, as the server does. */
export function previewBudgetImportAnswer(
  year: number,
  c: BudgetPreviewCase,
): { status: number; body: unknown } {
  switch (c) {
    case "ok": {
      const body: finance_budgetImportOut = {
        valid: true,
        year,
        source_file: SOURCE_FILE,
        sheets: [chiSheet(year), thuSheet(year)],
        warnings: [],
        errors: [],
      };
      return { status: 201, body };
    }
    case "closed":
      return {
        status: 409,
        body: {
          code: "budget_period_closed",
          message:
            `ngan_sach: kỳ tháng 9/${year} đã chốt (mã CK-${year}-09) — không nạp Excel vào năm ngân sách ${year} khi ` +
            "năm hoặc bất kỳ tháng nào của năm đã chốt, vì nạp là thay số liệu của cả năm. Muốn nạp, người có quyền " +
            "xác nhận mở chốt kỳ này, kèm lý do",
          trace_id: "",
        },
      };
    case "entries":
      return {
        status: 409,
        body: {
          code: "budget_sheet_has_entries",
          message:
            `ngan_sach: bảng chi năm ${year} (mã NS-${year}-CHI-01) đã có số liệu nhập tay (2 đợt thu, chi ở các ` +
            "khoản mục: I Chi đầu tư phát triển; 3 ô số gõ hoặc sửa tay) nên không nạp đè. Gỡ bảng trước (nút Gỡ, " +
            "kèm lý do) rồi nạp lại tệp Excel.",
          trace_id: "",
        },
      };
    case "errors":
      return {
        status: 400,
        body: {
          code: "import_invalid",
          message:
            "Tệp có lỗi nên chưa nạp bảng nào. Sửa các chỗ được liệt kê rồi nạp lại — tệp được nạp toàn bộ hoặc không gì cả.",
          trace_id: "",
          errors: [
            {
              sheet: "Chi NS",
              row: 14,
              column: "Dự toán năm",
              message: "Ô ghi chữ “khoảng 120” — cần một con số.",
            },
            {
              sheet: "Chi NS",
              row: 27,
              column: "",
              message: "Dòng có số liệu nhưng thiếu nội dung khoản mục.",
            },
            {
              sheet: "Thu NS",
              row: 0,
              column: "",
              message: "Sheet không có dòng tiêu đề nên không nạp.",
            },
          ],
        },
      };
  }
}
