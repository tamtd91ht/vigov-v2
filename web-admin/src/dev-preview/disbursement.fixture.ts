import type {
  finance_categoryProgressOut,
  finance_chungTuRa,
  finance_curvePointOut,
  finance_danhSachDuAnRa,
  finance_danhSachHangMucRa,
  finance_duAnRa,
  finance_fundingSourceProjectsOut,
  finance_fundingSourcesOut,
  finance_projectCommentsOut,
  finance_projectCurveOut,
  finance_projectIssuesOut,
  finance_projectSummaryOut,
  finance_projectVouchersOut,
  identity_danhBaChonNguoiRa,
  identity_danhSachBoPhanRa,
  page_Result_comms_notificationOut,
} from "@/lib/api/schema.gen";

/**
 * FIXTURE — what the dev-only screenshot preview answers for the Giải ngân routes (`fixture-fetch.ts`).
 * Shaped like the prototype's sample: four categories, six projects (one delayed, one with no source
 * yet), two funding sources, vouchers in the three states, two issues, two comments. Every type comes
 * from the contract (`schema.gen.ts`), so a contract change turns this file red instead of letting the
 * preview draw a shape the server no longer sends.
 *
 * NO REAL PERSON (rule 3): staff are "Cán bộ A/B/C" with made-up codes; no phone, no address. Amounts
 * are invented. Ratios are hundredths of a percent, as the server sends them (7400 = 74%).
 *
 * The YEAR is whatever the screen asks for: the preview echoes the requested `year`, so the register's
 * year select (anchored on the machine clock) always finds projects.
 */

const CAT_NEW = "01PREVIEWCAT0000000000001";
const CAT_REPAIR = "01PREVIEWCAT0000000000002";
const CAT_DIGITAL = "01PREVIEWCAT0000000000003";
const CAT_ENVIRONMENT = "01PREVIEWCAT0000000000004";

const UNIT_OFFICE = "01PREVIEWUNIT000000000001";
const UNIT_ECONOMY = "01PREVIEWUNIT000000000002";

const SOURCE_PROVINCE = "01PREVIEWSRC0000000000001";
const SOURCE_COMMUNE = "01PREVIEWSRC0000000000002";

export const PREVIEW_PROJECT_ID = "01PREVIEWPRJ0000000000001";
const PRJ_ROAD = "01PREVIEWPRJ0000000000002";
const PRJ_SCHOOL = "01PREVIEWPRJ0000000000003";
const PRJ_OFFICE = "01PREVIEWPRJ0000000000004";
const PRJ_CAMERA = "01PREVIEWPRJ0000000000005";
const PRJ_EMBANKMENT = "01PREVIEWPRJ0000000000006";

export const PREVIEW_CATEGORIES: finance_danhSachHangMucRa = {
  items: [
    { id: CAT_NEW, code: "xay-dung-moi", label: "Công trình xây dựng mới", is_default: true, active: true, order: 1, source: "he-thong", tier: 2 },
    { id: CAT_REPAIR, code: "sua-chua-cai-tao", label: "Sửa chữa, cải tạo", is_default: false, active: true, order: 2, source: "don-vi", tier: 1 },
    { id: CAT_DIGITAL, code: "ha-tang-so", label: "Hạ tầng số", is_default: false, active: true, order: 3, source: "don-vi", tier: 1 },
    { id: CAT_ENVIRONMENT, code: "moi-truong", label: "Môi trường", is_default: false, active: true, order: 4, source: "don-vi", tier: 1 },
  ],
};

export const PREVIEW_UNITS: identity_danhSachBoPhanRa = {
  items: [
    { id: UNIT_OFFICE, code: "van-phong", name: "Văn phòng HĐND – UBND", parent_id: "", order: 1, staff_count: 6 },
    { id: UNIT_ECONOMY, code: "kinh-te", name: "Phòng Kinh tế", parent_id: "", order: 2, staff_count: 5 },
  ],
};

export const PREVIEW_STAFF: identity_danhBaChonNguoiRa = {
  items: [
    { code: "CB-00001", full_name: "Cán bộ A", position: "Kế toán ngân sách", department_id: UNIT_OFFICE },
    { code: "CB-00002", full_name: "Cán bộ B", position: "Công chức Tài chính – Kế toán", department_id: UNIT_ECONOMY },
    { code: "CB-00003", full_name: "Cán bộ C", position: "Phó Chủ tịch UBND", department_id: UNIT_OFFICE },
  ],
};

const DELAY_THRESHOLD = 1000;
const SCOPE_NOTICE = "Số liệu mẫu của trang xem thử — không phải số liệu của xã nào.";

type ProjectSeed = {
  id: string;
  code: string;
  name: string;
  category: string;
  planned: number;
  disbursed: number;
  delayScore: number | null;
  unit: string;
  assignee: string;
  deadlineMonthDay: string;
  allocations: { source: string; amount: number; disbursed: number }[];
  latestIssue?: { id: string; text: string; resolved: boolean };
};

const SEEDS: readonly ProjectSeed[] = [
  {
    id: PREVIEW_PROJECT_ID,
    code: "DA-001",
    name: "Nhà văn hoá thôn Bình An",
    category: CAT_NEW,
    planned: 2_500_000_000,
    disbursed: 1_850_000_000,
    delayScore: -300,
    unit: UNIT_ECONOMY,
    assignee: "CB-00002",
    deadlineMonthDay: "12-15",
    allocations: [
      { source: SOURCE_PROVINCE, amount: 1_500_000_000, disbursed: 1_200_000_000 },
      { source: SOURCE_COMMUNE, amount: 1_000_000_000, disbursed: 650_000_000 },
    ],
    latestIssue: { id: "01PREVIEWISS0000000000001", text: "Chờ bổ sung hồ sơ nghiệm thu giai đoạn 2", resolved: false },
  },
  {
    id: PRJ_ROAD,
    code: "DA-002",
    name: "Đường bê tông liên thôn Bình An – Bình Trung",
    category: CAT_NEW,
    planned: 4_200_000_000,
    disbursed: 1_050_000_000,
    delayScore: 3136,
    unit: UNIT_ECONOMY,
    assignee: "CB-00002",
    deadlineMonthDay: "12-31",
    allocations: [{ source: SOURCE_PROVINCE, amount: 4_200_000_000, disbursed: 1_050_000_000 }],
    latestIssue: { id: "01PREVIEWISS0000000000003", text: "Vướng giải phóng mặt bằng đoạn qua thôn Bình Trung", resolved: false },
  },
  {
    id: PRJ_SCHOOL,
    code: "DA-003",
    name: "Sửa chữa trường mầm non trung tâm",
    category: CAT_REPAIR,
    planned: 850_000_000,
    disbursed: 620_000_000,
    delayScore: -200,
    unit: UNIT_OFFICE,
    assignee: "CB-00001",
    deadlineMonthDay: "11-30",
    allocations: [{ source: SOURCE_COMMUNE, amount: 850_000_000, disbursed: 620_000_000 }],
  },
  {
    id: PRJ_OFFICE,
    code: "DA-004",
    name: "Cải tạo hội trường trụ sở UBND xã",
    category: CAT_REPAIR,
    planned: 1_200_000_000,
    disbursed: 480_000_000,
    delayScore: 800,
    unit: UNIT_OFFICE,
    assignee: "CB-00003",
    deadlineMonthDay: "12-20",
    allocations: [{ source: SOURCE_COMMUNE, amount: 900_000_000, disbursed: 480_000_000 }],
  },
  {
    id: PRJ_CAMERA,
    code: "DA-005",
    name: "Lắp đặt camera an ninh các thôn",
    category: CAT_DIGITAL,
    planned: 600_000_000,
    disbursed: 540_000_000,
    delayScore: -1500,
    unit: UNIT_OFFICE,
    assignee: "CB-00001",
    deadlineMonthDay: "10-31",
    allocations: [{ source: SOURCE_PROVINCE, amount: 600_000_000, disbursed: 540_000_000 }],
  },
  {
    id: PRJ_EMBANKMENT,
    code: "DA-006",
    name: "Kè chống sạt lở bờ sông Ly Ly",
    category: CAT_ENVIRONMENT,
    planned: 3_000_000_000,
    disbursed: 0,
    delayScore: null,
    unit: "",
    assignee: "",
    deadlineMonthDay: "12-31",
    allocations: [],
  },
];

const SOURCE_NAMES: Record<string, string> = {
  [SOURCE_PROVINCE]: "Ngân sách thành phố hỗ trợ",
  [SOURCE_COMMUNE]: "Ngân sách xã",
};

/** Hundredths of a percent, as the server rounds them; `null` when the base is 0. */
function ratio(part: number, whole: number): number | null {
  return whole === 0 ? null : Math.round((part / whole) * 10000);
}

/** Share of the year already elapsed, from the machine clock — what the server derives the same way. */
function timeElapsed(year: number): number {
  const now = new Date();
  if (now.getFullYear() > year) return 10000;
  if (now.getFullYear() < year) return 0;
  const start = new Date(year, 0, 1).getTime();
  const end = new Date(year + 1, 0, 1).getTime();
  return Math.round(((now.getTime() - start) / (end - start)) * 10000);
}

function project(seed: ProjectSeed, year: number): finance_duAnRa {
  const allocated = seed.allocations.reduce((sum, a) => sum + a.amount, 0);
  const status = seed.allocations.length === 0 ? "chua-gan-nguon" : allocated < seed.planned ? "chua-du" : "du";
  return {
    id: seed.id,
    code: seed.code,
    year,
    category_id: seed.category,
    name: seed.name,
    description: "Dữ liệu mẫu của trang xem thử.",
    planned_amount: seed.planned,
    approved_amount: seed.planned,
    disbursed_amount: seed.disbursed,
    remaining_amount: seed.planned - seed.disbursed,
    disbursed_ratio: ratio(seed.disbursed, seed.planned),
    delay_score: seed.delayScore,
    is_delayed: seed.delayScore !== null && seed.delayScore > DELAY_THRESHOLD,
    org_unit_id: seed.unit === "" ? undefined : seed.unit,
    assignee_id: seed.assignee === "" ? undefined : seed.assignee,
    start_date: `${year}-02-15`,
    completion_date: `${year}-${seed.deadlineMonthDay}`,
    disbursement_deadline: `${year}-${seed.deadlineMonthDay}`,
    delay_threshold: DELAY_THRESHOLD,
    delay_threshold_source: "mac_dinh",
    scope_notice: SCOPE_NOTICE,
    funding_status: {
      status,
      source_count: seed.allocations.length,
      allocated_total: allocated,
      shortfall_amount: Math.max(0, seed.planned - allocated),
    },
    funding_source_names: seed.allocations.map((a) => SOURCE_NAMES[a.source]!),
    funding_allocations: seed.allocations.map((a) => ({
      funding_source_id: a.source,
      name: SOURCE_NAMES[a.source]!,
      amount: a.amount,
      disbursed_amount: a.disbursed,
      disbursed_ratio: ratio(a.disbursed, a.amount),
    })),
    unallocated_plan_amount: Math.max(0, seed.planned - allocated),
    time_elapsed_ratio: timeElapsed(year),
    latest_issue:
      seed.latestIssue === undefined
        ? null
        : { ...seed.latestIssue, recorded_at: `${year}-09-${seed.id === PRJ_ROAD ? "22" : "28"}T02:30:00Z` },
  };
}

export function previewProjects(year: number): finance_danhSachDuAnRa {
  return {
    items: SEEDS.map((s) => project(s, year)),
    year,
    delay_threshold: DELAY_THRESHOLD,
    delay_threshold_source: "mac_dinh",
    scope_notice: SCOPE_NOTICE,
  };
}

/** One project of the preview year, or `null` (the route then answers 404, as the server would). */
export function previewProject(id: string, year: number): finance_duAnRa | null {
  const seed = SEEDS.find((s) => s.id === id);
  return seed === undefined ? null : project(seed, year);
}

/** A cumulative curve that grows month by month and stops at the current month. */
function curve(planned: number, disbursed: number, year: number): finance_curvePointOut[] {
  const now = new Date();
  const lastMonth = now.getFullYear() > year ? 12 : now.getFullYear() < year ? 0 : now.getMonth() + 1;
  const shape = [0.02, 0.05, 0.12, 0.2, 0.3, 0.4, 0.5, 0.6, 0.72, 0.84, 0.94, 1];
  return shape.map((share, i) => {
    const month = i + 1;
    const doneShare = lastMonth === 0 ? 0 : Math.min(1, share / shape[lastMonth - 1]!);
    return {
      month,
      planned_cumulative: Math.round(planned * share),
      disbursed_cumulative: month <= lastMonth ? Math.round(disbursed * doneShare) : null,
    };
  });
}

function categoryRow(category: string, label: string, order: number, year: number): finance_categoryProgressOut {
  const rows = SEEDS.filter((s) => s.category === category);
  const planned = rows.reduce((sum, s) => sum + s.planned, 0);
  const disbursed = rows.reduce((sum, s) => sum + s.disbursed, 0);
  return {
    category_id: category,
    label,
    order,
    in_catalogue: true,
    project_count: rows.length,
    planned,
    disbursed,
    undisbursed: planned - disbursed,
    disbursed_ratio: ratio(disbursed, planned),
    undisbursed_ratio: ratio(planned - disbursed, planned),
    disbursement_deadline: `${year}-12-31`,
  };
}

export function previewSummary(year: number): finance_projectSummaryOut {
  const planned = SEEDS.reduce((sum, s) => sum + s.planned, 0);
  const disbursed = SEEDS.reduce((sum, s) => sum + s.disbursed, 0);
  return {
    year,
    planned_total: planned,
    project_count: SEEDS.length,
    disbursed_total: disbursed,
    disbursed_ratio: ratio(disbursed, planned),
    time_elapsed_ratio: timeElapsed(year),
    remaining_total: planned - disbursed,
    delay_threshold: DELAY_THRESHOLD,
    delay_threshold_source: "mac_dinh",
    delayed_project_count: SEEDS.filter((s) => s.delayScore !== null && s.delayScore > DELAY_THRESHOLD).length,
    open_issue_count: 2,
    monthly: curve(planned, disbursed, year),
    disbursed_after_year: 0,
    by_category: PREVIEW_CATEGORIES.items.map((c) => categoryRow(c.id, c.label, c.order, year)),
    total: {
      order: 0,
      in_catalogue: true,
      project_count: SEEDS.length,
      planned,
      disbursed,
      undisbursed: planned - disbursed,
      disbursed_ratio: ratio(disbursed, planned),
      undisbursed_ratio: ratio(planned - disbursed, planned),
    },
    scope_notice: SCOPE_NOTICE,
  };
}

const GRANTED: Record<string, number> = { [SOURCE_PROVINCE]: 7_000_000_000, [SOURCE_COMMUNE]: 3_500_000_000 };

export function previewFundingSources(year: number): finance_fundingSourcesOut {
  return {
    year,
    items: [SOURCE_PROVINCE, SOURCE_COMMUNE].map((id, i) => {
      const lines = SEEDS.flatMap((s) => s.allocations.filter((a) => a.source === id));
      const allocated = lines.reduce((sum, a) => sum + a.amount, 0);
      const disbursed = lines.reduce((sum, a) => sum + a.disbursed, 0);
      const granted = GRANTED[id]!;
      return {
        id,
        name: SOURCE_NAMES[id]!,
        order: i + 1,
        year,
        granted_amount: granted,
        allocated_amount: allocated,
        project_count: lines.length,
        disbursed_amount: disbursed,
        unallocated_amount: Math.max(0, granted - allocated),
        overallocated_amount: Math.max(0, allocated - granted),
        allocated_ratio: ratio(allocated, granted),
        disbursed_of_allocated_ratio: ratio(disbursed, allocated),
        disbursed_of_granted_ratio: ratio(disbursed, granted),
      };
    }),
    unattributed_disbursed_amount: 0,
    scope_notice: SCOPE_NOTICE,
  };
}

export function previewFundingSourceProjects(id: string, year: number): finance_fundingSourceProjectsOut | null {
  const name = SOURCE_NAMES[id];
  if (name === undefined) return null;
  return {
    funding_source_id: id,
    name,
    year,
    items: SEEDS.flatMap((s) =>
      s.allocations
        .filter((a) => a.source === id)
        .map((a) => ({
          id: s.id,
          code: s.code,
          name: s.name,
          planned_amount: s.planned,
          allocated_amount: a.amount,
          disbursed_amount: a.disbursed,
          disbursed_ratio: ratio(a.disbursed, a.amount),
        })),
    ),
    disbursed_without_allocation_amount: 0,
  };
}

/** Vouchers of the detail project — one in each of the three states (`nhan-ghi-giai-ngan.ts`). */
export function previewVouchers(projectId: string, year: number): finance_projectVouchersOut {
  const base = {
    project_id: projectId,
    counterparty: "Công ty TNHH Xây dựng mẫu",
    funding_source_id: SOURCE_PROVINCE,
    funding_source_name: SOURCE_NAMES[SOURCE_PROVINCE],
    entered_by: "CB-00001",
    unlock_count: 0,
  };
  const items: finance_chungTuRa[] =
    projectId === PREVIEW_PROJECT_ID
      ? [
          {
            ...base,
            id: "01PREVIEWVCH0000000000003",
            payment_date: `${year}-09-25`,
            amount: 450_000_000,
            description: "Tạm ứng đợt 3",
            status: "ke-toan-nhap",
            voucher_no: "PC-118",
          },
          {
            ...base,
            id: "01PREVIEWVCH0000000000002",
            payment_date: `${year}-07-10`,
            amount: 750_000_000,
            description: "Thanh toán khối lượng đợt 2",
            status: "da-xac-nhan",
            voucher_no: "PC-087",
            confirmed_by: "CB-00003",
          },
          {
            ...base,
            id: "01PREVIEWVCH0000000000001",
            payment_date: `${year}-04-02`,
            amount: 650_000_000,
            description: "Tạm ứng hợp đồng thi công",
            status: "da-khoa",
            voucher_no: "PC-041",
            funding_source_id: SOURCE_COMMUNE,
            funding_source_name: SOURCE_NAMES[SOURCE_COMMUNE],
            confirmed_by: "CB-00003",
            locked_by: "CB-00003",
            locked_at: `${year}-04-05T03:00:00Z`,
          },
        ]
      : [];
  return { project_id: projectId, items, count: items.length };
}

export function previewCurve(projectId: string, year: number): finance_projectCurveOut {
  const seed = SEEDS.find((s) => s.id === projectId);
  return {
    project_id: projectId,
    year,
    points: curve(seed?.planned ?? 0, seed?.disbursed ?? 0, year),
    expected_end_month: 12,
    disbursed_after_year: 0,
  };
}

export function previewIssues(projectId: string, year: number): finance_projectIssuesOut {
  const items: finance_projectIssuesOut["items"] =
    projectId === PREVIEW_PROJECT_ID
      ? [
          {
            id: "01PREVIEWISS0000000000001",
            project_id: projectId,
            title: "Chờ bổ sung hồ sơ nghiệm thu giai đoạn 2",
            description: "Đơn vị thi công chưa nộp biên bản nghiệm thu khối lượng.",
            recorded_by: "CB-00002",
            recorded_at: `${year}-09-28T02:30:00Z`,
            resolved: false,
          },
          {
            id: "01PREVIEWISS0000000000002",
            project_id: projectId,
            title: "Điều chỉnh thiết kế phần mái",
            recorded_by: "CB-00001",
            recorded_at: `${year}-06-12T08:00:00Z`,
            resolved: true,
            resolved_at: `${year}-07-01T08:00:00Z`,
            resolved_by: "CB-00003",
          },
        ]
      : [];
  return { project_id: projectId, items, count: items.length, open_count: items.filter((i) => !i.resolved).length };
}

export function previewComments(projectId: string, year: number): finance_projectCommentsOut {
  const items: finance_projectCommentsOut["items"] =
    projectId === PREVIEW_PROJECT_ID
      ? [
          {
            id: "01PREVIEWCMT0000000000001",
            project_id: projectId,
            body: "Đề nghị phòng Kinh tế đôn đốc đơn vị thi công nộp hồ sơ trước ngày 10.",
            author_code: "CB-00003",
            mentioned_staff_codes: ["CB-00002"],
            created_at: `${year}-09-29T01:15:00Z`,
          },
          {
            id: "01PREVIEWCMT0000000000002",
            project_id: projectId,
            body: "Đã liên hệ, đơn vị hẹn nộp trong tuần này.",
            author_code: "CB-00002",
            mentioned_staff_codes: [],
            created_at: `${year}-09-30T07:40:00Z`,
          },
        ]
      : [];
  return { project_id: projectId, items, count: items.length };
}

export function previewNotifications(year: number): page_Result_comms_notificationOut {
  return {
    items: [
      {
        id: "01PREVIEWNTF0000000000001",
        kind: "sap-den-han",
        title: "Dự án DA-002 sắp đến hạn giải ngân",
        body: "Đường bê tông liên thôn Bình An – Bình Trung",
        link: "",
        read: false,
        read_at: null,
        created_at: `${year}-10-01T01:00:00Z`,
      },
      {
        id: "01PREVIEWNTF0000000000002",
        kind: "ban-tin-tuan",
        title: "Bản tin tuần",
        body: "Tổng hợp tiến độ giải ngân tuần qua",
        link: "",
        read: true,
        read_at: `${year}-09-30T01:00:00Z`,
        created_at: `${year}-09-29T01:00:00Z`,
      },
    ],
    next_cursor: "",
    has_more: false,
  };
}
