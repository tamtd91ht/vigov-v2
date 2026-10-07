import { serverTransitions } from "@/features/nhiem-vu/task-transitions.fixture";
import type {
  identity_danhSachCaLamViecRa,
  identity_danhSachKhoiNhiemVuRa,
  page_Result_petitions_deNghiChoDuyetRa,
  page_Result_petitions_deNghiLuiHanRa,
  page_Result_petitions_nhatKyNhiemVuRa,
  page_Result_petitions_nhiemVuRa,
  petitions_danhSachLoaiNhiemVuRa,
  petitions_danhSachMucUuTienRa,
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
  petitions_taskCountsOut,
} from "@/lib/api/schema.gen";

import { PREVIEW_UNITS } from "./disbursement.fixture";

/**
 * FIXTURE — what the dev-only screenshot preview answers for the Nhiệm vụ routes (`fixture-fetch.ts`).
 * Twelve tasks over all seven statuses and both task types, priorities mixed, one overdue (a directive
 * task, so the Sổ theo dõi has it), one leader-approved but not upper-approved (NV107), one due tomorrow; one parent (`PREVIEW_TASK_CODE`) carrying everything the detail draws — three directive
 * documents, a timeline with comments and an attachment, a pending deadline-extension request and two
 * sub-tasks. Every type comes from the contract (`schema.gen.ts`), so a contract change turns this file
 * red instead of letting the preview draw a shape the server no longer sends.
 *
 * NO REAL PERSON (rule 3): people are "Cán bộ A/B/C" (`PREVIEW_STAFF`, codes CB-0000x); no phone, no
 * address, no citizen name in any title. Document numbers and dates are invented.
 *
 * `allowed_transitions` comes from the SAME sample of the server's map the tests use
 * (`task-transitions.fixture.ts`) — one copy, not a second one drifting here.
 *
 * DATES ARE RELATIVE TO NOW, so "overdue" and "due tomorrow" stay true on every day a screenshot is
 * taken — the screen derives both from `due_at` and the clock (rule 10, invariant 3).
 */

const STAFF_A = "CB-00001";
const STAFF_B = "CB-00002";
const STAFF_C = "CB-00003";

const UNIT_OFFICE = PREVIEW_UNITS.items[0]!.id;
const UNIT_ECONOMY = PREVIEW_UNITS.items[1]!.id;

/** The task the detail preview opens: the parent with documents, log, attachment, extension, children. */
export const PREVIEW_TASK_CODE = "NV105";
/** Two tasks of the first Kanban column — what `?chon=2` ticks. */
export const PREVIEW_SELECTED_CODES = ["NV101", "NV102"] as const;

const TYPE_BY_DOCUMENT = "theo-van-ban";
const TYPE_BASIC = "co-ban";

export const PREVIEW_TASK_TYPES: petitions_danhSachLoaiNhiemVuRa = {
  items: [
    { id: "01PREVIEWTTY0000000000001", code: TYPE_BY_DOCUMENT, label: "Theo văn bản", is_default: false, active: true, order: 1, source: "he-thong", tier: 3, requires_directive: true },
    { id: "01PREVIEWTTY0000000000002", code: TYPE_BASIC, label: "Cơ bản", is_default: true, active: true, order: 2, source: "he-thong", tier: 2, requires_directive: false },
  ],
};

export const PREVIEW_TASK_PRIORITIES: petitions_danhSachMucUuTienRa = {
  items: [
    { id: "01PREVIEWTPR0000000000001", code: "khan", label: "Khẩn", is_default: false, active: true, order: 1, source: "he-thong", tier: 2 },
    { id: "01PREVIEWTPR0000000000002", code: "cao", label: "Cao", is_default: false, active: true, order: 2, source: "he-thong", tier: 2 },
    { id: "01PREVIEWTPR0000000000003", code: "thuong", label: "Thường", is_default: true, active: true, order: 3, source: "he-thong", tier: 2 },
  ],
};

export const PREVIEW_TASK_BLOCS: identity_danhSachKhoiNhiemVuRa = {
  items: [
    { id: "01PREVIEWTBL0000000000001", code: "khoi-uy-ban", label: "Khối Uỷ ban", is_default: true, active: true, order: 1, source: "he-thong", tier: 2 },
    { id: "01PREVIEWTBL0000000000002", code: "khoi-dang", label: "Khối Đảng", is_default: false, active: true, order: 2, source: "he-thong", tier: 2 },
    { id: "01PREVIEWTBL0000000000003", code: "khac", label: "Khác", is_default: false, active: true, order: 3, source: "he-thong", tier: 2 },
  ],
};

/** The software's default labels and order, as the server returns them for a commune that changed nothing. */
const STATUS_ROWS: readonly [code: string, label: string, role: "chinh" | "re-nhanh"][] = [
  ["moi-giao", "Mới giao", "chinh"],
  ["da-tiep-nhan", "Đã tiếp nhận", "chinh"],
  ["dang-thuc-hien", "Đang thực hiện", "chinh"],
  ["cho-duyet", "Chờ duyệt", "chinh"],
  ["hoan-thanh", "Hoàn thành", "chinh"],
  ["tam-dung", "Tạm dừng", "re-nhanh"],
  ["chuyen-tiep", "Chuyển tiếp", "re-nhanh"],
];

export const PREVIEW_TASK_STATUSES: petitions_danhSachTrangThaiNhiemVuRa = {
  items: STATUS_ROWS.map(([code, label, role], i) => ({
    code,
    label,
    order: i + 1,
    role,
    default_label: label,
    default_order: i + 1,
    customised: false,
  })),
};

/** Monday to Friday, two sessions a day (ISO weekday: 1 = Monday). */
export const PREVIEW_WORKING_HOURS: identity_danhSachCaLamViecRa = {
  items: [1, 2, 3, 4, 5].flatMap((weekday) => [
    { id: `01PREVIEWWH${weekday}A00000000000001`, weekday, start: "07:30", end: "11:30", note: "" },
    { id: `01PREVIEWWH${weekday}P00000000000001`, weekday, start: "13:30", end: "17:00", note: "" },
  ]),
  problems: [],
};

/** Who holds `task.extend` — the answer to `staff-directory?permission=task.extend`. */
export const PREVIEW_EXTENSION_APPROVERS: readonly string[] = [STAFF_A, STAFF_C];

/** `now` shifted by whole days, at 10:00 UTC (17:00 in Việt Nam) — the end of the afternoon session. */
function day(now: Date, offsetDays: number): string {
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() + offsetDays, 10, 0, 0));
  return d.toISOString();
}

/** `now` shifted by whole days and hours, for timestamps of acts already done. */
function at(now: Date, offsetDays: number, hourUtc: number): string {
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() + offsetDays, hourUtc, 15, 0));
  return d.toISOString();
}

type TaskSeed = {
  code: string;
  type: string;
  bloc: string;
  priority: string;
  title: string;
  description?: string;
  status: string;
  source: string;
  unit: string;
  assignee: string;
  assigner: string;
  /** Days from today; `null` = no deadline. */
  due: number | null;
  /** Days from today of the original deadline when it was extended; else the same as `due`. */
  originalDue?: number;
  /** Days from today of creation (negative). */
  created: number;
  progress: number;
  parent?: string;
  childCount?: number;
  result?: string;
  meeting?: { id: string; title: string; conclusion: number };
  /** `leader_approved` ticked by hand (default: only a finished task). */
  leaderApproved?: boolean;
  /** Approved extensions — matches `previewTaskExtensionHistory` (default 0). */
  extensions?: number;
  /** A request is waiting — matches `previewTaskExtensions` (default false). */
  pendingExtension?: boolean;
};

const SEEDS: readonly TaskSeed[] = [
  {
    code: "NV101", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "thuong", status: "moi-giao", source: "truc-tiep",
    title: "Chuẩn bị hội trường tiếp xúc cử tri quý IV", unit: UNIT_OFFICE, assignee: STAFF_B, assigner: STAFF_C,
    due: 6, created: -1, progress: 0,
  },
  {
    code: "NV102", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "cao", status: "moi-giao", source: "ket-luan-hop",
    title: "Tổng hợp ý kiến góp ý dự thảo quy chế làm việc của UBND xã", unit: UNIT_OFFICE, assignee: "", assigner: STAFF_C,
    due: 9, created: -2, progress: 0,
    meeting: { id: "01PREVIEWMTG0000000000001", title: "Giao ban UBND xã tháng này", conclusion: 2 },
  },
  {
    code: "NV103", type: TYPE_BY_DOCUMENT, bloc: "khoi-uy-ban", priority: "khan", status: "dang-thuc-hien", source: "phan-anh",
    title: "Khắc phục đèn chiếu sáng hỏng trên tuyến đường liên thôn", unit: UNIT_ECONOMY, assignee: STAFF_B, assigner: STAFF_A,
    due: -3, created: -12, progress: 60,
  },
  {
    code: "NV104", type: TYPE_BY_DOCUMENT, bloc: "khoi-dang", priority: "cao", status: "da-tiep-nhan", source: "van-ban-den",
    title: "Triển khai kế hoạch tuyên truyền phòng, chống cháy nổ mùa hanh khô", unit: UNIT_OFFICE, assignee: STAFF_A, assigner: STAFF_C,
    due: 1, created: -5, progress: 10,
  },
  {
    code: PREVIEW_TASK_CODE, type: TYPE_BY_DOCUMENT, bloc: "khoi-uy-ban", priority: "khan", status: "dang-thuc-hien", source: "van-ban-den",
    title: "Rà soát, cập nhật danh sách hộ nghèo, hộ cận nghèo trên địa bàn xã",
    description:
      "Rà soát theo bộ tiêu chí hiện hành; lập biểu tổng hợp theo từng thôn; báo cáo UBND xã trước khi trình cấp trên.",
    unit: UNIT_OFFICE, assignee: STAFF_B, assigner: STAFF_C,
    due: 4, created: -15, progress: 45, childCount: 2, extensions: 1, pendingExtension: true,
  },
  {
    code: "NV106", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "thuong", status: "dang-thuc-hien", source: "truc-tiep",
    title: "Cập nhật số liệu thống kê đất đai năm nay", unit: UNIT_ECONOMY, assignee: STAFF_A, assigner: STAFF_C,
    due: 12, created: -8, progress: 30,
  },
  {
    code: "NV107", type: TYPE_BY_DOCUMENT, bloc: "khoi-uy-ban", priority: "cao", status: "cho-duyet", source: "van-ban-den",
    title: "Báo cáo kết quả thực hiện chương trình mục tiêu quốc gia 9 tháng", unit: UNIT_ECONOMY, assignee: STAFF_B, assigner: STAFF_C,
    due: 2, originalDue: -1, created: -20, progress: 100, leaderApproved: true, extensions: 1,
    result: "Đã hoàn thành báo cáo, gửi kèm biểu số liệu.",
  },
  {
    code: "NV108", type: TYPE_BASIC, bloc: "khac", priority: "thuong", status: "cho-duyet", source: "truc-tiep",
    title: "Kiểm kê tài sản, trang thiết bị bộ phận một cửa", unit: UNIT_OFFICE, assignee: STAFF_A, assigner: STAFF_C,
    due: 5, created: -10, progress: 100,
    result: "Đã kiểm kê xong, lập biên bản kiểm kê.",
  },
  {
    code: "NV109", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "cao", status: "hoan-thanh", source: "ket-luan-hop",
    title: "Hoàn thiện hồ sơ đề nghị công nhận thôn đạt chuẩn văn hoá", unit: UNIT_OFFICE, assignee: STAFF_B, assigner: STAFF_C,
    due: -6, created: -30, progress: 100,
    result: "Hồ sơ đã nộp đúng hạn.",
  },
  {
    code: "NV110", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "thuong", status: "tam-dung", source: "truc-tiep",
    title: "Khảo sát nhu cầu lắp đặt camera an ninh tại các thôn", unit: UNIT_ECONOMY, assignee: STAFF_A, assigner: STAFF_C,
    due: 20, created: -9, progress: 20,
  },
  {
    code: "NV111", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "thuong", status: "moi-giao", source: "truc-tiep",
    title: "Thu thập phiếu rà soát hộ gia đình thôn Bình An", unit: UNIT_OFFICE, assignee: STAFF_A, assigner: STAFF_C,
    due: 3, created: -4, progress: 0, parent: PREVIEW_TASK_CODE,
  },
  {
    code: "NV112", type: TYPE_BASIC, bloc: "khoi-uy-ban", priority: "cao", status: "chuyen-tiep", source: "truc-tiep",
    title: "Đối chiếu danh sách với dữ liệu bảo hiểm y tế", unit: UNIT_ECONOMY, assignee: STAFF_B, assigner: STAFF_C,
    due: 4, created: -4, progress: 15, parent: PREVIEW_TASK_CODE,
  },
];

/** The three directive-document groups, on the three `theo-van-ban` tasks (the Sổ theo dõi's columns). */
function documentsOf(code: string, now: Date): petitions_nhiemVuVanBanRa[] {
  const date = (offset: number) => day(now, offset).slice(0, 10);
  const doc = (n: number, group: string, reference: string, offset: number, summary: string): petitions_nhiemVuVanBanRa => ({
    id: `01PREVIEWDOC${code}${String(n).padStart(9, "0")}`,
    group,
    reference,
    date: date(offset),
    summary,
    position: n,
  });
  switch (code) {
    case PREVIEW_TASK_CODE:
      return [
        doc(1, "cap-tren-giao", "45/KH-UBND", -18, "Kế hoạch rà soát hộ nghèo, hộ cận nghèo của thành phố"),
        doc(2, "chi-dao-dang-uy", "12-CV/ĐU", -16, "Công văn chỉ đạo của Đảng uỷ xã về công tác giảm nghèo"),
        doc(3, "san-pham-dau-ra", "", 4, "Báo cáo kết quả rà soát gửi UBND thành phố"),
      ];
    case "NV104":
      return [doc(1, "cap-tren-giao", "88/CV-PCCC", -7, "Công văn tăng cường phòng cháy, chữa cháy mùa hanh khô")];
    case "NV107":
      return [
        doc(1, "cap-tren-giao", "210/UBND-KT", -25, "Yêu cầu báo cáo chương trình mục tiêu quốc gia"),
        doc(2, "san-pham-dau-ra", "31/BC-UBND", -1, "Báo cáo 9 tháng của UBND xã"),
      ];
    default:
      return [];
  }
}

function build(seed: TaskSeed, now: Date, withDocuments: boolean): petitions_nhiemVuRa {
  const due = seed.due === null ? null : day(now, seed.due);
  const task: petitions_nhiemVuRa = {
    code: seed.code,
    type: seed.type,
    bloc: seed.bloc,
    priority: seed.priority,
    title: seed.title,
    description: seed.description ?? "",
    status: seed.status,
    allowed_transitions: serverTransitions(seed.status),
    source: seed.source,
    source_id: seed.source === "truc-tiep" ? "" : `01PREVIEWSRC${seed.code}000000000`,
    unit: seed.unit,
    assignee: seed.assignee,
    assigner: seed.assigner,
    due_at: due,
    original_due_at: seed.originalDue === undefined ? due : day(now, seed.originalDue),
    completed_at: seed.status === "hoan-thanh" ? at(now, (seed.due ?? 0) - 1, 8) : null,
    progress: seed.progress,
    result_summary: seed.result ?? "",
    note: "",
    leader_approved: seed.leaderApproved ?? seed.status === "hoan-thanh",
    superior_acknowledged: false,
    parent: seed.parent ?? "",
    child_count: seed.childCount ?? 0,
    extension_count: seed.extensions ?? 0,
    pending_extension: seed.pendingExtension ?? false,
    created_by: STAFF_C,
    created_at: at(now, seed.created, 2),
    updated_at: at(now, Math.min(seed.created + 2, 0), 3),
  };
  if (seed.meeting !== undefined) {
    task.meeting_id = seed.meeting.id;
    task.meeting_title = seed.meeting.title;
    task.conclusion_no = seed.meeting.conclusion;
  }
  if (withDocuments) task.documents = documentsOf(seed.code, now);
  return task;
}

/** The filters the preview honours — the ones a screenshot of the register uses. Others are ignored. */
export type PreviewTaskQuery = {
  readonly status?: string;
  readonly parent?: string;
  readonly type?: string;
  readonly priority?: string;
  readonly bloc?: string;
  readonly unit?: string;
  readonly assignee?: string;
  readonly source?: string;
  readonly late?: boolean;
  /** `include=documents` — the Sổ theo dõi's projection. */
  readonly documents?: boolean;
};

function matches(seed: TaskSeed, q: PreviewTaskQuery, now: Date): boolean {
  const eq = (want: string | undefined, have: string) => want === undefined || want === have;
  if (!eq(q.status, seed.status) || !eq(q.parent, seed.parent ?? "") || !eq(q.type, seed.type)) return false;
  if (!eq(q.priority, seed.priority) || !eq(q.bloc, seed.bloc) || !eq(q.unit, seed.unit)) return false;
  if (!eq(q.assignee, seed.assignee) || !eq(q.source, seed.source)) return false;
  if (q.late === true) {
    if (seed.due === null || seed.status === "hoan-thanh") return false;
    if (new Date(day(now, seed.due)).getTime() >= now.getTime()) return false;
  }
  return true;
}

/** `GET /api/v1/tasks` — one page holding every match (the fixture is smaller than a page). */
export function previewTasks(q: PreviewTaskQuery, now: Date = new Date()): page_Result_petitions_nhiemVuRa {
  return {
    items: SEEDS.filter((s) => matches(s, q, now)).map((s) => build(s, now, q.documents === true)),
    next_cursor: "",
    has_more: false,
  };
}

/** `GET /api/v1/tasks/{code}` — the detail route ALWAYS carries `documents` (an array, maybe empty). */
export function previewTask(code: string, now: Date = new Date()): petitions_nhiemVuRa | null {
  const seed = SEEDS.find((s) => s.code === code);
  return seed === undefined ? null : build(seed, now, true);
}

/** `GET /api/v1/task-counts` — per status, under the same filters as the board. */
export function previewTaskCounts(q: PreviewTaskQuery, now: Date = new Date()): petitions_taskCountsOut {
  const rows = SEEDS.filter((s) => matches(s, { ...q, status: undefined }, now));
  return {
    by_status: STATUS_ROWS.map(([status]) => ({ status, count: rows.filter((s) => s.status === status).length })),
  };
}

/** `GET /api/v1/tasks/{code}/log-entries` — newest first. */
export function previewTaskLog(code: string, now: Date = new Date()): page_Result_petitions_nhatKyNhiemVuRa | null {
  const seed = SEEDS.find((s) => s.code === code);
  if (seed === undefined) return null;
  const entry = (
    n: number,
    offset: number,
    hour: number,
    actor: string,
    status: string,
    note: string,
    extra: Partial<petitions_nhatKyNhiemVuRa> = {},
  ): petitions_nhatKyNhiemVuRa => ({
    id: `01PREVIEWLOG${seed.code}${String(n).padStart(9, "0")}`,
    at: at(now, offset, hour),
    actor_code: actor,
    status,
    unit: "",
    assignee: "",
    note,
    attachments: [],
    ...extra,
  });
  const created = entry(1, seed.created, 2, STAFF_C, "moi-giao", "Giao việc.", { unit: seed.unit, assignee: seed.assignee });
  if (seed.code !== PREVIEW_TASK_CODE) {
    const items = seed.status === "moi-giao" ? [created] : [entry(2, Math.min(seed.created + 2, 0), 3, seed.assignee || STAFF_C, seed.status, ""), created];
    return { items, next_cursor: "", has_more: false };
  }
  return {
    items: [
      entry(6, -1, 4, STAFF_C, "dang-thuc-hien", "Đề nghị gửi bản tổng hợp trước ngày họp giao ban để đưa vào nội dung."),
      entry(5, -2, 7, STAFF_B, "dang-thuc-hien", "Đã nhận đủ phiếu của 4/6 thôn, gửi kèm báo cáo tiến độ đợt 1.", {
        attachments: [
          {
            id: "01PREVIEWATT0000000000001",
            file_name: "bao-cao-tien-do-dot-1.pdf",
            mime_type: "application/pdf",
            size_bytes: 248_312,
            status: "stored",
          },
        ],
      }),
      entry(4, -6, 2, STAFF_A, "dang-thuc-hien", "Văn phòng đã chuyển mẫu phiếu rà soát cho các thôn."),
      entry(3, -12, 3, STAFF_B, "dang-thuc-hien", "Bắt đầu rà soát; tách hai việc con cho từng phần."),
      entry(2, -14, 1, STAFF_B, "da-tiep-nhan", ""),
      created,
    ],
    next_cursor: "",
    has_more: false,
  };
}

/** `GET /api/v1/task-extensions?task=` — the one pending request is on `PREVIEW_TASK_CODE`. */
export function previewTaskExtensions(task: string | null, now: Date = new Date()): page_Result_petitions_deNghiChoDuyetRa {
  const seed = SEEDS.find((s) => s.code === PREVIEW_TASK_CODE)!;
  const pending = {
    id: "01PREVIEWEXT0000000000001",
    task_code: seed.code,
    task_title: seed.title,
    task_due_at: seed.due === null ? null : day(now, seed.due),
    task_assigner: seed.assigner,
    new_due_at: day(now, (seed.due ?? 0) + 7),
    reason: "Còn hai thôn chưa nộp phiếu rà soát do trùng lịch thu hoạch mùa vụ.",
    requested_by: seed.assignee,
    requested_at: at(now, -1, 8),
  };
  return { items: task === null || task === seed.code ? [pending] : [], next_cursor: "", has_more: false };
}

/**
 * `GET /api/v1/tasks/{code}/extensions` — the task's extension history, newest first (every status,
 * the server's Vietnamese codes). `PREVIEW_TASK_CODE` has its pending request plus an earlier one
 * approved; NV107 has the approved request that moved its deadline. Others: none (the section hides).
 */
export function previewTaskExtensionHistory(code: string, now: Date = new Date()): page_Result_petitions_deNghiLuiHanRa {
  const seed = SEEDS.find((s) => s.code === code);
  if (seed === undefined) return { items: [], next_cursor: "", has_more: false };
  const pending = previewTaskExtensions(code, now).items[0];
  const items: page_Result_petitions_deNghiLuiHanRa["items"] = [];
  if (code === PREVIEW_TASK_CODE && pending !== undefined) {
    items.push({
      id: pending.id,
      requested_by: pending.requested_by,
      new_due_at: pending.new_due_at,
      reason: pending.reason,
      status: "cho-duyet",
      requested_at: pending.requested_at,
      decided_at: null,
      decision_note: null,
    });
    items.push({
      id: "01PREVIEWEXT0000000000002",
      requested_by: seed.assignee,
      decided_by: seed.assigner,
      new_due_at: day(now, seed.due ?? 0),
      reason: "Bổ sung tiêu chí rà soát theo hướng dẫn mới của thành phố.",
      status: "da-duyet",
      requested_at: at(now, -9, 2),
      decided_at: at(now, -8, 3),
      decision_note: "Đồng ý lùi hạn một lần.",
    });
  }
  if (code === "NV107") {
    items.push({
      id: "01PREVIEWEXT0000000000003",
      requested_by: seed.assignee,
      decided_by: seed.assigner,
      new_due_at: day(now, seed.due ?? 0),
      reason: "Chờ số liệu quyết toán của các thôn.",
      status: "da-duyet",
      requested_at: at(now, -3, 2),
      decided_at: at(now, -2, 3),
      decision_note: null,
    });
  }
  return { items, next_cursor: "", has_more: false };
}
