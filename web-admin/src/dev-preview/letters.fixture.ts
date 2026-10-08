import type {
  documents_citizenLetterItemOut,
  documents_citizenLetterOut,
  documents_duplicatesOut,
  documents_letterLogEntryOut,
  documents_letterLogOut,
  documents_letterReportOut,
  page_Result_documents_citizenLetterItemOut,
} from "@/lib/api/schema.gen";

import { PREVIEW_UNITS } from "./disbursement.fixture";

/**
 * FIXTURE — the citizen-letter register of the dev-only preview (`/xem-thu/van-ban?tab=don-thu`,
 * `preview-gate.ts`). Nothing here is a real letter or person: senders are "Nguyễn Văn A"-style
 * placeholders and every phone is the agreed fake number AS THE SERVER WOULD RETURN IT — masked
 * (`09****0000`, rule 3). Addresses never appear: the server never sends one (only
 * `has_sender_address`). Dates are relative to the machine clock.
 *
 * WHAT EACH ROW IS FOR (task card W2): every C4 type; statuses across C3; #11 overdue WITH a stored
 * processing deadline (the only letter with one — every other deadline is null, as on the real server
 * today, ADR 0078 #3); #10 a denunciation (identity + summary withheld on the list; in the drawer the
 * preview's session — Cán bộ C, `petition.create`, not its assignee — sees the summary, not the sender);
 * #9 Đang giải quyết (result form open); #8 resolved with its result; #7 a branch end (Chuyển đơn);
 * #6 linked to an earlier letter as a duplicate; #5 a long summary and no sender at all. #12 has an
 * EMPTY log, #11 a three-entry log.
 *
 * The answers follow what `service-documents/internal/http/citizen_letter.go` returns for THIS
 * session (Cán bộ C, CB-00003, holder of `petition.create`): `domain.DetailDisclosure` decides.
 */

const DAY = 24 * 60 * 60 * 1000;
const UNIT_OFFICE = PREVIEW_UNITS.items[0]!.id;
const UNIT_ECONOMY = PREVIEW_UNITS.items[1]!.id;
/** The preview session's own staff code (`shell.fixture.ts`, person `lanh-dao`). */
const VIEWER = "CB-00003";
const MASKED_PHONE = "09****0000";

/** Every letter id starts with this — `documents-preview.tsx` reads it to open the right drawer. */
export const PREVIEW_LETTER_PREFIX = "01PREVIEWDONTHU";

export function previewLetterId(number: number): string {
  return `${PREVIEW_LETTER_PREFIX}00000000${String(number).padStart(2, "0")}`;
}

/** The letter whose drawer shows Mới vào sổ with the routing box and an empty log. */
export const PREVIEW_LETTER_NEW = previewLetterId(12);
/** Overdue (stored processing deadline in the past), three log entries. */
export const PREVIEW_LETTER_OVERDUE = previewLetterId(11);
/** The denunciation, opened by a viewer who is not its assignee. */
export const PREVIEW_LETTER_DENUNCIATION = previewLetterId(10);
/** Đang giải quyết — the result form is open. */
export const PREVIEW_LETTER_RESOLVING = previewLetterId(9);
/** Đã giải quyết with its result. */
export const PREVIEW_LETTER_RESOLVED = previewLetterId(8);
/** A branch end (Chuyển đơn). */
export const PREVIEW_LETTER_FORWARDED = previewLetterId(7);

function day(days: number): string {
  const d = new Date(Date.now() + days * DAY);
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

function at(days: number): string {
  return new Date(Date.now() + days * DAY).toISOString();
}

/** C3's arrows — the server's `next_statuses`, mirrored for the fixture only. */
const NEXT: Readonly<Record<string, readonly string[]>> = {
  "moi-vao-so": ["dang-xu-ly-don"],
  "dang-xu-ly-don": ["thu-ly", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"],
  "thu-ly": ["dang-giai-quyet"],
  "dang-giai-quyet": ["da-giai-quyet", "dinh-chi"],
};

type Seed = {
  number: number;
  received: number;
  type: string;
  name: string | null;
  phone: boolean;
  address: boolean;
  summary: string;
  status: string;
  unit: string;
  assignee: string;
  processingDue: number | null;
  related: number | null;
  closedDaysAgo: number | null;
  result: boolean;
};

const SEEDS: readonly Seed[] = [
  {
    number: 12, received: 0, type: "kien-nghi-phan-anh", name: "Nguyễn Văn A", phone: true, address: true,
    summary: "Kiến nghị sửa chữa đoạn đường bê tông thôn Bình An bị sụt lún sau mưa lớn",
    status: "moi-vao-so", unit: "", assignee: "", processingDue: null, related: null, closedDaysAgo: null, result: false,
  },
  {
    number: 11, received: -9, type: "khieu-nai", name: "Trần Thị B", phone: true, address: true,
    summary: "Khiếu nại việc chậm giải quyết hồ sơ đề nghị cấp giấy chứng nhận quyền sử dụng đất",
    status: "dang-xu-ly-don", unit: UNIT_OFFICE, assignee: VIEWER, processingDue: -2, related: null, closedDaysAgo: null, result: false,
  },
  {
    number: 10, received: -6, type: "to-cao", name: "Lê Văn C", phone: true, address: false,
    summary: "Tố cáo hành vi lấn chiếm hành lang an toàn giao thông tại tuyến đường liên thôn",
    status: "thu-ly", unit: UNIT_ECONOMY, assignee: "CB-00002", processingDue: null, related: null, closedDaysAgo: null, result: false,
  },
  {
    number: 9, received: -14, type: "de-nghi", name: "Phạm Thị D", phone: false, address: true,
    summary: "Đề nghị hỗ trợ kinh phí sửa chữa nhà văn hoá thôn Bình Trung",
    status: "dang-giai-quyet", unit: UNIT_ECONOMY, assignee: VIEWER, processingDue: null, related: null, closedDaysAgo: null, result: false,
  },
  {
    number: 8, received: -20, type: "kien-nghi-phan-anh", name: "Hoàng Văn E", phone: true, address: true,
    summary: "Phản ánh mương thoát nước khu dân cư tổ 3 bị tắc gây ngập úng",
    status: "da-giai-quyet", unit: UNIT_OFFICE, assignee: VIEWER, processingDue: null, related: null, closedDaysAgo: 3, result: true,
  },
  {
    number: 7, received: -18, type: "khieu-nai", name: "Võ Thị G", phone: true, address: true,
    summary: "Khiếu nại quyết định xử phạt vi phạm hành chính của cơ quan cấp tỉnh",
    status: "chuyen-don", unit: UNIT_OFFICE, assignee: "", processingDue: null, related: null, closedDaysAgo: 10, result: false,
  },
  {
    number: 6, received: -1, type: "kien-nghi-phan-anh", name: "Hoàng Văn E", phone: true, address: true,
    summary: "Phản ánh mương thoát nước tổ 3 vẫn còn tắc sau đợt nạo vét",
    status: "moi-vao-so", unit: "", assignee: "", processingDue: null, related: 8, closedDaysAgo: null, result: false,
  },
  {
    number: 5, received: -4, type: "de-nghi", name: null, phone: false, address: false,
    summary:
      "Đề nghị UBND xã xem xét bố trí kinh phí lắp đặt hệ thống đèn chiếu sáng công cộng trên toàn tuyến " +
      "đường trục chính từ ngã ba chợ đến trường tiểu học, đồng thời bổ sung biển báo giao thông tại các " +
      "điểm giao cắt nguy hiểm để bảo đảm an toàn cho học sinh đi học buổi sáng sớm và chiều tối",
    status: "dang-xu-ly-don", unit: UNIT_ECONOMY, assignee: "CB-00002", processingDue: null, related: null, closedDaysAgo: null, result: false,
  },
  {
    number: 4, received: -25, type: "kien-nghi-phan-anh", name: "Đặng Văn H", phone: true, address: false,
    summary: "Kiến nghị về tiếng ồn từ xưởng gỗ trong khu dân cư",
    status: "luu-don", unit: UNIT_OFFICE, assignee: "", processingDue: null, related: null, closedDaysAgo: 15, result: false,
  },
];

const CLOSED = new Set(["da-giai-quyet", "dinh-chi", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"]);
const RESOLVED = new Set(["da-giai-quyet", "dinh-chi"]);

/** "Số ngày xử lý" as the server counts it: received day included, stopped when closed. */
function daysOpen(seed: Seed): number {
  const end = seed.closedDaysAgo === null ? 0 : -seed.closedDaysAgo;
  return Math.max(1, end - seed.received + 1);
}

function itemOf(seed: Seed, year: number): documents_citizenLetterItemOut {
  const withheld = seed.type === "to-cao";
  return {
    id: previewLetterId(seed.number),
    number: seed.number,
    year,
    received_date: day(seed.received),
    letter_type: seed.type,
    sender_name: withheld ? null : seed.name,
    sender_phone: withheld || !seed.phone ? null : MASKED_PHONE,
    identity_withheld: withheld,
    summary: withheld ? null : seed.summary,
    summary_withheld: withheld,
    holding_unit_id: seed.unit === "" ? undefined : seed.unit,
    assignee_code: seed.assignee === "" ? undefined : seed.assignee,
    status: seed.status,
    processing_due_at: seed.processingDue === null ? null : at(seed.processingDue),
    resolution_due_at: null,
    days_open: daysOpen(seed),
    is_resolved: RESOLVED.has(seed.status),
    is_closed: CLOSED.has(seed.status),
    related_letter_id: seed.related === null ? undefined : previewLetterId(seed.related),
  };
}

export type PreviewLetterQuery = {
  status?: string;
  holdingUnit?: string;
  assignee?: string;
  receivedFrom?: string;
  receivedTo?: string;
  scope?: string;
};

/** One page of the register under the query the screen sent, newest booking first. */
export function previewLetters(query: PreviewLetterQuery): page_Result_documents_citizenLetterItemOut {
  const year = new Date().getFullYear();
  const items = SEEDS.map((s) => itemOf(s, year)).filter(
    (l) =>
      (query.status === undefined || l.status === query.status) &&
      (query.holdingUnit === undefined || l.holding_unit_id === query.holdingUnit) &&
      (query.assignee === undefined || l.assignee_code === query.assignee) &&
      (query.receivedFrom === undefined || l.received_date >= query.receivedFrom) &&
      (query.receivedTo === undefined || l.received_date <= query.receivedTo) &&
      (query.scope !== "mine" || l.assignee_code === VIEWER) &&
      (query.scope !== "related" || l.assignee_code === VIEWER || l.holding_unit_id === UNIT_OFFICE),
  );
  return { items, next_cursor: "", has_more: false };
}

/** The drawer's letter, disclosed as `domain.DetailDisclosure` would for Cán bộ C. */
export function previewLetter(id: string): documents_citizenLetterOut | null {
  const seed = SEEDS.find((s) => previewLetterId(s.number) === id);
  if (seed === undefined) return null;
  const year = new Date().getFullYear();
  const item = itemOf(seed, year);
  const denunciation = seed.type === "to-cao";
  // A denunciation: the sender to its assignee alone; the summary also to `petition.create` holders.
  const showIdentity = !denunciation || seed.assignee === VIEWER;
  const closedAt = seed.closedDaysAgo === null ? null : at(-seed.closedDaysAgo);
  return {
    ...item,
    sender_name: showIdentity ? seed.name : null,
    sender_phone: showIdentity && seed.phone ? MASKED_PHONE : null,
    identity_withheld: !showIdentity,
    has_sender_address: showIdentity && seed.address,
    sender_unknown: showIdentity ? seed.name === null && !seed.phone && !seed.address : null,
    summary: seed.summary,
    summary_withheld: false,
    next_statuses: [...(NEXT[seed.status] ?? [])],
    accepted_at: ["thu-ly", "dang-giai-quyet", "da-giai-quyet"].includes(seed.status) ? at(seed.received + 2) : null,
    resolved_at: RESOLVED.has(seed.status) ? closedAt : null,
    closed_at: closedAt,
    result_document_no: seed.result ? "45/TB-UBND" : undefined,
    result_document_date: seed.result ? day(-3) : undefined,
    result_signer: seed.result ? "Phó Chủ tịch UBND xã" : undefined,
    result_issuer: seed.result ? "UBND xã Thăng Bình" : undefined,
    result_summary: seed.result ? "Đã nạo vét toàn tuyến mương tổ 3; thông báo kết quả cho người gửi đơn." : null,
    created_by_code: "CB-00001",
    created_at: `${day(seed.received)}T01:30:00Z`,
    updated_at: `${day(seed.received)}T01:30:00Z`,
  };
}

function entry(
  letterNumber: number,
  n: number,
  daysAgo: number,
  actor: string,
  kind: string,
  extra: Partial<documents_letterLogEntryOut>,
): documents_letterLogEntryOut {
  return { id: `01PREVIEWNHATKY${letterNumber}${n}`, letter_id: previewLetterId(letterNumber), at: at(-daysAgo), actor_code: actor, kind, ...extra };
}

/** The processing log, NEWEST FIRST as the server sends it. */
export function previewLetterLog(id: string): documents_letterLogOut | null {
  const seed = SEEDS.find((s) => previewLetterId(s.number) === id);
  if (seed === undefined) return null;
  if (seed.number === 12) return { items: [] };
  if (seed.number === 11) {
    return {
      items: [
        entry(11, 3, 1, VIEWER, "ghi-chu", { content: "Đã liên hệ bộ phận Một cửa để đối chiếu hồ sơ gốc, chờ phản hồi." }),
        entry(11, 2, 6, "CB-00001", "chuyen-trang-thai", {
          from_status: "moi-vao-so",
          to_status: "dang-xu-ly-don",
          content: "Đơn đủ thông tin, chuyển xem xét thụ lý.",
        }),
        entry(11, 1, 8, "CB-00001", "luan-chuyen", {
          to_unit_id: UNIT_OFFICE,
          assignee_code: VIEWER,
          content: "Chuyển Văn phòng tham mưu, giao Cán bộ C phụ trách.",
        }),
      ],
    };
  }
  if (seed.unit === "") return { items: [] };
  return {
    items: [
      entry(seed.number, 1, -seed.received - 1, "CB-00001", "luan-chuyen", {
        to_unit_id: seed.unit,
        assignee_code: seed.assignee === "" ? undefined : seed.assignee,
        content: "Chuyển xử lý theo chức năng.",
      }),
    ],
  };
}

/** The duplicate warning the preview's `?dup=1` shows: two earlier letters of the same sender. */
export function previewLetterDuplicates(): documents_duplicatesOut {
  const year = new Date().getFullYear();
  return {
    items: [
      { id: previewLetterId(8), number: 8, year, received_date: day(-20), similarity: 0.82, summary: SEEDS[4]!.summary },
      { id: previewLetterId(6), number: 6, year, received_date: day(-1), similarity: 0.57, summary: SEEDS[6]!.summary },
    ],
  };
}

/** The year's report — full, or (for `?state=empty`) a year with nothing in it. */
export function previewLetterReport(year: number, empty: boolean): documents_letterReportOut {
  const thisYear = new Date().getFullYear();
  const months = Array.from({ length: 12 }, (_, i) => ({ month: i + 1, received: 0, resolved: 0 }));
  if (empty || year !== thisYear) {
    return {
      year,
      received: 0,
      resolved: 0,
      closed_in_processing: 0,
      in_progress: 0,
      overdue: 0,
      on_time_percent: null,
      average_days: null,
      by_type: [],
      by_unit: [],
      by_month: months,
    };
  }
  const shape = [3, 4, 2, 5, 3, 6, 4, 3, 5, 4, 0, 0];
  const done = [1, 3, 2, 3, 2, 4, 3, 2, 4, 2, 0, 0];
  return {
    year,
    received: 39,
    resolved: 26,
    closed_in_processing: 5,
    in_progress: 9,
    overdue: 1,
    // No letter carries a resolution deadline yet (ADR 0078 #3): the on-time share has no denominator.
    on_time_percent: null,
    average_days: 11.4,
    by_type: [
      { letter_type: "kien-nghi-phan-anh", total: 18, resolved: 13, in_progress: 3, overdue: 0 },
      { letter_type: "khieu-nai", total: 11, resolved: 7, in_progress: 3, overdue: 1 },
      { letter_type: "to-cao", total: 3, resolved: 2, in_progress: 1, overdue: 0 },
      { letter_type: "de-nghi", total: 7, resolved: 4, in_progress: 2, overdue: 0 },
    ],
    by_unit: [
      { unit_id: UNIT_OFFICE, total: 21, in_progress: 4, resolved: 15, overdue: 1, on_time_percent: null },
      { unit_id: UNIT_ECONOMY, total: 15, in_progress: 3, resolved: 11, overdue: 0, on_time_percent: null },
      { unit_id: null, total: 3, in_progress: 2, resolved: 0, overdue: 0, on_time_percent: null },
    ],
    by_month: months.map((m, i) => ({ ...m, received: shape[i]!, resolved: done[i]! })),
  };
}
