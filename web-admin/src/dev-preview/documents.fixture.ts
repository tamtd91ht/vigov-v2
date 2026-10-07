import type {
  documents_danhSachLichSuChuyenRa,
  documents_danhSachLoaiVanBanRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
  documents_vanBanDiRa,
  page_Result_documents_vanBanDenRa,
  page_Result_documents_vanBanDiRa,
} from "@/lib/api/schema.gen";

import { PREVIEW_UNITS } from "./disbursement.fixture";

/**
 * FIXTURE — the dev-only screenshot preview of Văn bản & Đơn thư (`/xem-thu/van-ban`, `preview-gate.ts`).
 * Nothing here is a real document, body or person: issuing bodies are generic public offices of the
 * preview's own sample city, people are "Cán bộ A/B/C" of `PREVIEW_STAFF` or "Nguyễn Văn A" (rule 3: no
 * real name, no real phone in source). Dates are relative to the machine clock, so the overdue row is
 * overdue and the others are not on whatever day the screenshot is taken.
 *
 * Coverage the task card asked for: seven incoming rows — every stored status code, one overdue, one
 * with a long issuing body and summary (the table cuts both to one line) — a document with NO routing
 * and one with THREE; four outgoing rows (one without a signer, one whose recipient is a citizen).
 */

const DAY = 24 * 60 * 60 * 1000;
const UNIT_OFFICE = PREVIEW_UNITS.items[0]!.id;
const UNIT_ECONOMY = PREVIEW_UNITS.items[1]!.id;

/** `YYYY-MM-DD` of today + `days`, on the machine's calendar. */
function day(days: number): string {
  const d = new Date(Date.now() + days * DAY);
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

/** RFC 3339 instant `hours` from now. */
function at(hours: number): string {
  return new Date(Date.now() + hours * 60 * 60 * 1000).toISOString();
}

/** The incoming document the drawer screenshot opens: three routings on its timeline. */
export const PREVIEW_DOCUMENT_WITH_ROUTINGS = "01PREVIEWVBDEN000000000011";
/** An incoming document never routed: the timeline's empty sentence. */
export const PREVIEW_DOCUMENT_NO_ROUTING = "01PREVIEWVBDEN000000000012";

export const PREVIEW_DOCUMENT_TYPES: documents_danhSachLoaiVanBanRa = {
  items: [
    ["cong-van", "Công văn"],
    ["quyet-dinh", "Quyết định"],
    ["ke-hoach", "Kế hoạch"],
    ["thong-bao", "Thông báo"],
    ["bao-cao", "Báo cáo"],
    ["to-trinh", "Tờ trình"],
  ].map(([code, label], i) => ({
    id: `01PREVIEWLOAIVB00000000${String(i + 1).padStart(3, "0")}`,
    code: code!,
    label: label!,
    active: true,
    is_default: i === 0,
    order: i + 1,
    source: "",
    tier: 0,
  })),
};

type IncomingSeed = {
  number: number;
  received: number;
  reference: string;
  issued: number | null;
  issuer: string;
  type: string;
  summary: string;
  urgency: string;
  unit: string;
  assignee: string;
  dueHours: number;
  status: string;
  by: string;
};

/**
 * Newest number first — the register's own default order. Only #10 is open AND past its deadline,
 * so it is the one overdue row; the closed rows are never drawn overdue (`incomingDeadlineState`).
 */
const INCOMING: readonly IncomingSeed[] = [
  {
    number: 12,
    received: 0,
    reference: "1520/UBND-KT",
    issued: -2,
    issuer: "UBND thành phố Đà Nẵng",
    type: "ke-hoach",
    summary: "V/v triển khai kế hoạch chuyển đổi số cấp xã",
    urgency: "",
    unit: "",
    assignee: "",
    dueHours: 72,
    status: "moi-vao-so",
    by: "CB-00001",
  },
  {
    number: 11,
    received: -1,
    reference: "862/SNV-CCVC",
    issued: -4,
    issuer: "Sở Nội vụ thành phố Đà Nẵng",
    type: "cong-van",
    summary: "V/v rà soát, cập nhật hồ sơ cán bộ, công chức cấp xã",
    urgency: "khan",
    unit: UNIT_ECONOMY,
    assignee: "CB-00002",
    dueHours: 30,
    status: "da-phan-cong",
    by: "CB-00001",
  },
  {
    number: 10,
    received: -6,
    reference: "245/BCH-TM",
    issued: -8,
    issuer: "Ban Chỉ huy Quân sự thành phố",
    type: "cong-van",
    summary: "V/v chuẩn bị công tác tuyển chọn, gọi công dân nhập ngũ",
    urgency: "hoa-toc",
    unit: UNIT_OFFICE,
    assignee: "CB-00003",
    dueHours: -20,
    status: "dang-xu-ly",
    by: "CB-00001",
  },
  {
    number: 9,
    received: -9,
    reference: "77/TB-TTKSBT",
    issued: -10,
    issuer: "Trung tâm Kiểm soát bệnh tật thành phố",
    type: "thong-bao",
    summary: "Thông báo lịch tiêm chủng mở rộng tháng này",
    urgency: "thuong",
    unit: UNIT_OFFICE,
    assignee: "",
    dueHours: 40,
    status: "da-giai-quyet",
    by: "CB-00001",
  },
  {
    number: 8,
    received: -12,
    reference: "1093/VPBCĐ-PCTT",
    issued: -13,
    issuer: "Văn phòng Ban Chỉ đạo phòng, chống thiên tai và tìm kiếm cứu nạn thành phố Đà Nẵng",
    type: "cong-van",
    summary:
      "V/v tăng cường công tác kiểm tra, rà soát các khu vực có nguy cơ sạt lở đất, ngập úng cục bộ " +
      "và chủ động phương án sơ tán dân cư trước mùa mưa bão, báo cáo kết quả trước ngày 30 hằng tháng",
    urgency: "thuong-khan",
    unit: UNIT_ECONOMY,
    assignee: "",
    dueHours: 60,
    status: "chuyen-cap-tren",
    by: "CB-00001",
  },
  {
    number: 7,
    received: -15,
    reference: "",
    issued: null,
    issuer: "Công ty TNHH Dịch vụ Viễn thông A",
    type: "cong-van",
    summary: "V/v giới thiệu giải pháp phần mềm quản lý văn bản",
    urgency: "",
    unit: "",
    assignee: "",
    dueHours: 80,
    status: "luu-khong-thu-ly",
    by: "CB-00001",
  },
  {
    number: 6,
    received: -3,
    reference: "310/STC-NS",
    issued: -5,
    issuer: "Sở Tài chính thành phố Đà Nẵng",
    type: "cong-van",
    summary: "V/v báo cáo tình hình thực hiện dự toán ngân sách xã",
    urgency: "",
    unit: UNIT_ECONOMY,
    assignee: "CB-00002",
    dueHours: 96,
    status: "dang-xu-ly",
    by: "CB-00003",
  },
];

function incomingRow(seed: IncomingSeed, year: number): documents_vanBanDenRa {
  const id = `01PREVIEWVBDEN0000000000${String(seed.number).padStart(2, "0")}`;
  return {
    id,
    number: seed.number,
    year,
    received_date: day(seed.received),
    reference_no: seed.reference,
    document_date: seed.issued === null ? "" : day(seed.issued),
    issuing_body: seed.issuer,
    document_type: seed.type,
    summary: seed.summary,
    urgency: seed.urgency,
    holding_unit: seed.unit,
    assignee: seed.assignee,
    due_at: at(seed.dueHours),
    status: seed.status,
    created_by: seed.by,
    created_at: `${day(seed.received)}T01:30:00Z`,
    updated_at: `${day(seed.received)}T01:30:00Z`,
  };
}

const OPEN_STATUSES = new Set(["moi-vao-so", "da-phan-cong", "dang-xu-ly"]);

export type PreviewIncomingQuery = {
  year: number;
  status?: string;
  unit?: string;
  type?: string;
  q?: string;
  metric?: string;
  ascending: boolean;
};

/**
 * One page of the incoming register under the query the screen sent. The rows live in the CURRENT
 * year only — another year is honestly empty. `metric=overdue` is the server's own definition (open
 * AND past the deadline), compared against the machine clock at answer time.
 */
export function previewIncomingDocuments(query: PreviewIncomingQuery): page_Result_documents_vanBanDenRa {
  const thisYear = new Date().getFullYear();
  const needle = query.q?.trim().toLowerCase() ?? "";
  const items =
    query.year !== thisYear
      ? []
      : INCOMING.map((s) => incomingRow(s, thisYear)).filter(
          (d) =>
            (query.status === undefined || d.status === query.status) &&
            (query.unit === undefined || d.holding_unit === query.unit) &&
            (query.type === undefined || d.document_type === query.type) &&
            (needle === "" ||
              d.summary.toLowerCase().includes(needle) ||
              (d.reference_no ?? "").toLowerCase().includes(needle)) &&
            (query.metric !== "overdue" || (OPEN_STATUSES.has(d.status) && Date.parse(d.due_at) < Date.now())),
        );
  items.sort((a, b) => (query.ascending ? a.number - b.number : b.number - a.number));
  return { items, next_cursor: "", has_more: false };
}

export function previewIncomingDocument(id: string): documents_vanBanDenRa | null {
  const seed = INCOMING.find((s) => incomingRow(s, 0).id === id);
  return seed === undefined ? null : incomingRow(seed, new Date().getFullYear());
}

function routing(
  documentId: string,
  n: number,
  hoursAgo: number,
  by: string,
  from: string,
  to: string,
  assignee: string,
  status: string,
  reason: string,
): documents_lichSuChuyenRa {
  return {
    id: `01PREVIEWLSCHUYEN00000000${n}`,
    document_id: documentId,
    routed_at: at(-hoursAgo),
    routed_by: by,
    status,
    from_unit: from,
    to_unit: to,
    assignee,
    reason,
    created_at: at(-hoursAgo),
  };
}

/**
 * The routing timeline, OLDEST FIRST as the server sends it. Three lines on
 * `PREVIEW_DOCUMENT_WITH_ROUTINGS`, none on `PREVIEW_DOCUMENT_NO_ROUTING` (and on the documents never
 * routed), one line on the others that hold a unit.
 */
export function previewRoutings(id: string): documents_danhSachLichSuChuyenRa | null {
  const doc = previewIncomingDocument(id);
  if (doc === null) return null;
  if (id === PREVIEW_DOCUMENT_WITH_ROUTINGS) {
    return {
      items: [
        routing(id, 1, 26, "CB-00001", "", UNIT_OFFICE, "", "da-phan-cong", "Chuyển Văn phòng để trình lãnh đạo xem xét."),
        routing(
          id,
          2,
          20,
          "CB-00003",
          UNIT_OFFICE,
          UNIT_ECONOMY,
          "",
          "da-phan-cong",
          "Thuộc lĩnh vực của Phòng Kinh tế, đề nghị tham mưu văn bản trả lời.",
        ),
        routing(
          id,
          3,
          4,
          "CB-00003",
          UNIT_ECONOMY,
          UNIT_ECONOMY,
          "CB-00002",
          "da-phan-cong",
          "Giao Cán bộ B tổng hợp hồ sơ, báo cáo trước hạn.",
        ),
      ],
    };
  }
  if (doc.holding_unit === "" || doc.holding_unit === undefined) return { items: [] };
  return {
    items: [routing(id, 4, 48, "CB-00001", "", doc.holding_unit, doc.assignee ?? "", "da-phan-cong", "Chuyển xử lý theo chức năng.")],
  };
}

type OutgoingSeed = { number: number; date: number; type: string; summary: string; recipient: string; signer: string };

const OUTGOING: readonly OutgoingSeed[] = [
  {
    number: 15,
    date: 0,
    type: "bao-cao",
    summary: "Báo cáo kết quả rà soát hộ nghèo, hộ cận nghèo trên địa bàn xã",
    recipient: "UBND thành phố Đà Nẵng",
    signer: "Chủ tịch UBND xã",
  },
  {
    number: 14,
    date: -2,
    type: "cong-van",
    summary: "V/v trả lời kiến nghị về tuyến mương thoát nước thôn Bình An",
    recipient: "Ông Nguyễn Văn A, thôn Bình An",
    signer: "Phó Chủ tịch UBND xã",
  },
  {
    number: 13,
    date: -5,
    type: "thong-bao",
    summary:
      "Thông báo lịch tiếp công dân định kỳ của lãnh đạo UBND xã và lịch trực giải quyết thủ tục " +
      "hành chính ngoài giờ hành chính vào sáng thứ Bảy hằng tuần",
    recipient: "Các thôn, tổ dân phố trên địa bàn xã",
    signer: "",
  },
  {
    number: 12,
    date: -8,
    type: "to-trinh",
    summary: "Tờ trình đề nghị phê duyệt dự toán sửa chữa nhà văn hoá thôn",
    recipient: "Sở Tài chính thành phố Đà Nẵng",
    signer: "Chủ tịch UBND xã",
  },
];

export function previewOutgoingDocuments(query: {
  year: number;
  type?: string;
  q?: string;
  ascending: boolean;
}): page_Result_documents_vanBanDiRa {
  const thisYear = new Date().getFullYear();
  const needle = query.q?.trim().toLowerCase() ?? "";
  const items: documents_vanBanDiRa[] =
    query.year !== thisYear
      ? []
      : OUTGOING.filter(
          (s) =>
            (query.type === undefined || s.type === query.type) &&
            (needle === "" || s.summary.toLowerCase().includes(needle) || s.recipient.toLowerCase().includes(needle)),
        ).map((s) => ({
          id: `01PREVIEWVBDI00000000000${String(s.number).padStart(2, "0")}`,
          number: s.number,
          year: thisYear,
          document_date: day(s.date),
          document_type: s.type,
          summary: s.summary,
          recipient: s.recipient,
          signer: s.signer,
          created_by: "CB-00001",
          created_at: `${day(s.date)}T02:00:00Z`,
          updated_at: `${day(s.date)}T02:00:00Z`,
        }));
  items.sort((a, b) => (query.ascending ? a.number - b.number : b.number - a.number));
  return { items, next_cursor: "", has_more: false };
}

/**
 * `?state=loading|empty|error` of `/xem-thu/van-ban`: what the two REGISTER reads answer, so the three
 * non-data states can be screenshotted on the real screen. Set by `DocumentsPreview` during its first
 * render — before the registers' effects fetch — and read by the answering machine (`fixture-fetch.ts`).
 * `null` = the fixtures above.
 */
export type DocumentPreviewState = "loading" | "empty" | "error";

let documentPreviewState: DocumentPreviewState | null = null;

export function setDocumentPreviewState(state: DocumentPreviewState | null): void {
  documentPreviewState = state;
}

export function documentPreviewStateNow(): DocumentPreviewState | null {
  return documentPreviewState;
}

/** The sentence the `error` state's refusal carries — the preview's own words, not a server's. */
export const PREVIEW_REGISTER_ERROR = "Trang xem thử: lỗi mẫu để chụp trạng thái tải hỏng của sổ.";
