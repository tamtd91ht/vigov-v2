import type {
  audit_EntryView,
  comms_danhSachLoaiTaiNguyenRa,
  comms_mailSettingsOut,
  comms_mapFieldSchemaOut,
  identity_automationJobsOut,
  identity_danhSachBoPhanRa,
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachLoaiDonViDanCuRa,
  identity_danhSachNgayNghiLeRa,
  identity_danhSachSLARa,
  identity_danhSachThonToDanPhoRa,
  page_Result_audit_EntryView,
  petitions_petitionFieldListOut,
  petitions_systemMessageOut,
  platform_brandingSettingsOut,
} from "@/lib/api/schema.gen";
import type { ZaloChannelSettings, ZaloLinkCurrent, ZaloLinkedStaff } from "@/lib/api/zalo";

import { PREVIEW_CATEGORIES } from "./disbursement.fixture";
import { PREVIEW_DOCUMENT_TYPES } from "./documents.fixture";
import { PREVIEW_TASK_BLOCS, PREVIEW_TASK_PRIORITIES, PREVIEW_TASK_STATUSES, PREVIEW_TASK_TYPES } from "./tasks.fixture";

/**
 * FIXTURE — the reads of the Cấu hình screen (`app/cau-hinh/page.tsx`, every tab of `KhungTabCauHinh`)
 * for the dev-only preview `/xem-thu/cau-hinh` (ADR 0068 lần 6 #10). Nothing here is a real commune, a
 * real person or a real setting: officers are "Cán bộ A…E" with `CB-0000x` codes, the only phone-shaped
 * value anywhere is the agreed fake `0900000000` (rule 3, invariant 5), the mail host is an `.example`
 * name (RFC 2606) and no secret is present — `password_set` is a boolean, as the contract returns it.
 *
 * `answerSettings` is the ONE entry point: it answers a GET the screen makes and returns `null` for any
 * other path. Writes are never answered here — the shared answering machine (`fixture-fetch.ts`) refuses
 * every non-GET in the server's error shape.
 *
 * `?state=empty` answers every list empty (the screen's own empty states); `?state=loading` is handled
 * by the caller, which holds every read `isSettingsReadPath` names unsettled; `?state=mail-off` answers
 * the mail server configured but switched off (`is_enabled: false`), everything else as normal.
 */

export type SettingsPreviewState = "loading" | "empty" | "mail-off" | null;

/** `?state=` of `/xem-thu/cau-hinh`, read from the browser URL at call time (`null` on the server). */
export function settingsPreviewState(): SettingsPreviewState {
  if (typeof window === "undefined") return null;
  const v = new URLSearchParams(window.location.search).get("state");
  return v === "loading" || v === "empty" || v === "mail-off" ? v : null;
}

// ── Sơ đồ tổ chức: three levels. The two department ids are the shared fixture's (`PREVIEW_STAFF`
//    points at them), so the staff picker of other tabs still finds its people in a unit.
const UNIT_ROOT = "01PREVIEWUNIT000000000000";
const UNIT_OFFICE = "01PREVIEWUNIT000000000001";
const UNIT_ECONOMY = "01PREVIEWUNIT000000000002";
const UNIT_CULTURE = "01PREVIEWUNIT000000000003";
const UNIT_POLICE = "01PREVIEWUNIT000000000004";
const UNIT_ONE_STOP = "01PREVIEWUNIT000000000011";
const UNIT_ARCHIVE = "01PREVIEWUNIT000000000012";
const UNIT_LAND = "01PREVIEWUNIT000000000021";
const UNIT_FINANCE = "01PREVIEWUNIT000000000022";

export const PREVIEW_SETTINGS_UNITS: identity_danhSachBoPhanRa = {
  items: [
    { id: UNIT_ROOT, code: "ubnd-xa", name: "UBND xã Thăng Bình", parent_id: "", order: 1, staff_count: 3 },
    { id: UNIT_OFFICE, code: "van-phong", name: "Văn phòng HĐND – UBND", parent_id: UNIT_ROOT, order: 1, staff_count: 6 },
    { id: UNIT_ECONOMY, code: "kinh-te", name: "Phòng Kinh tế", parent_id: UNIT_ROOT, order: 2, staff_count: 5 },
    { id: UNIT_CULTURE, code: "van-hoa-xa-hoi", name: "Phòng Văn hoá – Xã hội", parent_id: UNIT_ROOT, order: 3, staff_count: 4 },
    { id: UNIT_POLICE, code: "cong-an-xa", name: "Công an xã", parent_id: UNIT_ROOT, order: 4, staff_count: 8 },
    { id: UNIT_ONE_STOP, code: "mot-cua", name: "Bộ phận Một cửa", parent_id: UNIT_OFFICE, order: 1, staff_count: 3 },
    { id: UNIT_ARCHIVE, code: "van-thu-luu-tru", name: "Tổ Văn thư – Lưu trữ", parent_id: UNIT_OFFICE, order: 2, staff_count: 2 },
    { id: UNIT_LAND, code: "dia-chinh", name: "Tổ Địa chính – Xây dựng", parent_id: UNIT_ECONOMY, order: 1, staff_count: 3 },
    { id: UNIT_FINANCE, code: "tai-chinh-ke-toan", name: "Tổ Tài chính – Kế toán", parent_id: UNIT_ECONOMY, order: 2, staff_count: 2 },
  ],
};

// ── Thôn / Tổ dân phố ─────────────────────────────────────────────────────────────────────────────
export const PREVIEW_RESIDENTIAL_TYPES: identity_danhSachLoaiDonViDanCuRa = {
  items: [
    { id: "01PREVIEWRUT0000000000001", code: "thon", label: "Thôn", is_default: true, active: true, order: 1, source: "he-thong", tier: 2, color: null },
    { id: "01PREVIEWRUT0000000000002", code: "to-dan-pho", label: "Tổ dân phố", is_default: false, active: true, order: 2, source: "he-thong", tier: 2, color: null },
    { id: "01PREVIEWRUT0000000000003", code: "khu-pho", label: "Khu phố", is_default: false, active: false, order: 3, source: "don-vi", tier: 1, color: null },
  ],
};

export const PREVIEW_RESIDENTIAL_UNITS: identity_danhSachThonToDanPhoRa = {
  items: [
    { id: "01PREVIEWRU00000000000001", code: "thon-binh-an", name: "Thôn Bình An", type_code: "thon", type_label: "Thôn", household_count: 312, population_count: 1184, active: true, head_staff_code: "CB-00001", head_staff_name: "Cán bộ A", order: 1 },
    { id: "01PREVIEWRU00000000000002", code: "thon-phu-my", name: "Thôn Phú Mỹ", type_code: "thon", type_label: "Thôn", household_count: 278, population_count: 1032, active: true, head_staff_code: "CB-00002", head_staff_name: "Cán bộ B", order: 2 },
    { id: "01PREVIEWRU00000000000003", code: "thon-an-thanh", name: "Thôn An Thạnh", type_code: "thon", type_label: "Thôn", household_count: 195, population_count: 741, active: true, head_staff_code: "", head_staff_name: "", order: 3 },
    { id: "01PREVIEWRU00000000000004", code: "to-dan-pho-1", name: "Tổ dân phố 1", type_code: "to-dan-pho", type_label: "Tổ dân phố", household_count: 146, population_count: 520, active: true, head_staff_code: "CB-00003", head_staff_name: "Cán bộ C", order: 4 },
    { id: "01PREVIEWRU00000000000005", code: "to-dan-pho-2", name: "Tổ dân phố 2", type_code: "to-dan-pho", type_label: "Tổ dân phố", household_count: null, population_count: null, active: true, head_staff_code: "", head_staff_name: "", order: 5 },
    { id: "01PREVIEWRU00000000000006", code: "thon-dong-tay", name: "Thôn Đông Tây (đã sáp nhập)", type_code: "thon", type_label: "Thôn", household_count: 88, population_count: 301, active: false, head_staff_code: "", head_staff_name: "", order: 6 },
  ],
};

// ── Danh mục: map asset types (comms) — the other six catalogues are the shared fixtures' ──────────
export const PREVIEW_MAP_ASSET_TYPES: comms_danhSachLoaiTaiNguyenRa = {
  items: [
    { id: "01PREVIEWMAT0000000000001", code: "doanh-nghiep", label: "Doanh nghiệp", is_default: true, active: true, order: 1, source: "he-thong", tier: 2, color: "#2fb1f9" },
    { id: "01PREVIEWMAT0000000000002", code: "truong-hoc", label: "Trường học", is_default: false, active: true, order: 2, source: "he-thong", tier: 2, color: "#86b940" },
    { id: "01PREVIEWMAT0000000000003", code: "tram-y-te", label: "Trạm y tế", is_default: false, active: true, order: 3, source: "he-thong", tier: 2, color: null },
    { id: "01PREVIEWMAT0000000000004", code: "nha-van-hoa", label: "Nhà văn hoá thôn", is_default: false, active: true, order: 4, source: "don-vi", tier: 1, color: null },
    { id: "01PREVIEWMAT0000000000005", code: "camera-an-ninh", label: "Camera an ninh", is_default: false, active: false, order: 5, source: "don-vi", tier: 1, color: null },
  ],
};

// ── Trường bản đồ — `field_code` in the server's shape `^[a-z][a-z0-9_]*$` (underscores, never dashes) ─
const MAP_FIELDS: readonly comms_mapFieldSchemaOut[] = [
  { id: "01PREVIEWMFS0000000000001", asset_type_code: "doanh-nghiep", field_code: "ma_so_thue", label: "Mã số thuế", value_type: "van-ban", options: [], is_required: true, sort_order: 1, is_active: true },
  { id: "01PREVIEWMFS0000000000002", asset_type_code: "doanh-nghiep", field_code: "so_lao_dong", label: "Số lao động", value_type: "so-nguyen", options: [], is_required: false, sort_order: 2, is_active: true },
  {
    id: "01PREVIEWMFS0000000000003",
    asset_type_code: "doanh-nghiep",
    field_code: "nganh_nghe",
    label: "Ngành nghề chính",
    value_type: "chon",
    options: [
      { value: "nong-nghiep", label: "Nông nghiệp" },
      { value: "che-bien", label: "Chế biến, chế tạo" },
      { value: "thuong-mai", label: "Thương mại, dịch vụ" },
    ],
    is_required: true,
    sort_order: 3,
    is_active: true,
  },
  { id: "01PREVIEWMFS0000000000004", asset_type_code: "doanh-nghiep", field_code: "ngay_thanh_lap", label: "Ngày thành lập", value_type: "ngay", options: [], is_required: false, sort_order: 4, is_active: true },
  { id: "01PREVIEWMFS0000000000005", asset_type_code: "doanh-nghiep", field_code: "co_giay_phep_moi_truong", label: "Có giấy phép môi trường", value_type: "dung-sai", options: [], is_required: false, sort_order: 5, is_active: false },
  { id: "01PREVIEWMFS0000000000006", asset_type_code: "truong-hoc", field_code: "so_hoc_sinh", label: "Số học sinh", value_type: "so-nguyen", options: [], is_required: true, sort_order: 1, is_active: true },
  { id: "01PREVIEWMFS0000000000007", asset_type_code: "truong-hoc", field_code: "cap_hoc", label: "Cấp học", value_type: "chon", options: [{ value: "mam-non", label: "Mầm non" }, { value: "tieu-hoc", label: "Tiểu học" }, { value: "thcs", label: "Trung học cơ sở" }], is_required: true, sort_order: 2, is_active: true },
];

// ── Lời hệ thống — three services, the same shape ─────────────────────────────────────────────────
const MESSAGES: Record<"petitions" | "finance" | "reporting", readonly petitions_systemMessageOut[]> = {
  petitions: [
    { code: "petition.not_found", group_code: "phan-anh", origin: "shipped", description: "Không tìm thấy phản ánh theo mã tra cứu", default_text: "Không tìm thấy phản ánh với mã này.", current_text: "Không tìm thấy phản ánh với mã này. Vui lòng kiểm tra lại mã trên tin nhắn đã nhận.", overridden: true, is_active: true, updated_at: "2026-09-28T02:15:00Z", updated_by: "CB-00003" },
    { code: "petition.closed", group_code: "phan-anh", origin: "shipped", description: "Phản ánh đã đóng, không nhận thêm ý kiến", default_text: "Phản ánh đã được xử lý xong, không thể gửi thêm ý kiến.", current_text: "Phản ánh đã được xử lý xong, không thể gửi thêm ý kiến.", overridden: false, is_active: true },
    { code: "petition.rate_limited", group_code: "phan-anh", origin: "shipped", description: "Gửi quá nhiều phản ánh trong thời gian ngắn", default_text: "Bạn đã gửi nhiều phản ánh liên tiếp. Vui lòng thử lại sau ít phút.", current_text: "Bạn đã gửi nhiều phản ánh liên tiếp. Vui lòng thử lại sau ít phút.", overridden: false, is_active: true },
    { code: "task.extension_pending", group_code: "phan-anh", origin: "shipped", description: "Nhiệm vụ đang có đề nghị gia hạn chờ duyệt", default_text: "Nhiệm vụ đang có một đề nghị gia hạn chờ duyệt.", current_text: "Nhiệm vụ đang có một đề nghị gia hạn chờ duyệt.", override_text: "Nhiệm vụ đang chờ lãnh đạo duyệt gia hạn, chưa sửa được hạn.", overridden: true, is_active: false, updated_at: "2026-10-02T03:00:00Z", updated_by: "CB-00003" },
    { code: "chung.loi-chao", group_code: "chung", origin: "commune", description: "Lời chào ở đầu tin gửi người dân", current_text: "UBND xã kính chào ông/bà.", overridden: false, is_active: true, updated_at: "2026-10-08T01:30:00Z", updated_by: "CB-00003" },
  ],
  finance: [
    { code: "budget.voucher_locked", group_code: "giai-ngan", origin: "shipped", description: "Chứng từ đã khoá, không sửa được", default_text: "Chứng từ đã khoá, không sửa được.", current_text: "Chứng từ đã khoá sổ. Liên hệ kế toán để mở khoá nếu cần điều chỉnh.", overridden: true, is_active: true, updated_at: "2026-09-30T08:40:00Z", updated_by: "CB-00001" },
    { code: "budget.self_confirm", group_code: "giai-ngan", origin: "shipped", description: "Người lập không tự xác nhận chứng từ của mình", default_text: "Người lập chứng từ không được tự xác nhận.", current_text: "Người lập chứng từ không được tự xác nhận.", overridden: false, is_active: true },
  ],
  reporting: [
    { code: "report.period_open", group_code: "bao-cao", origin: "shipped", description: "Kỳ báo cáo chưa kết thúc", default_text: "Kỳ báo cáo chưa kết thúc, số liệu có thể còn thay đổi.", current_text: "Kỳ báo cáo chưa kết thúc, số liệu có thể còn thay đổi.", overridden: false, is_active: true },
  ],
};

// ── Thời hạn xử lý: SLA rows, field labels, and the three calendar tables ─────────────────────────
const PETITION_FIELDS: petitions_petitionFieldListOut = {
  items: [
    { code: "an-ninh-trat-tu", label: "An ninh, trật tự", order: 1, default_label: "An ninh, trật tự", default_order: 1, icon: "shield", tone: "danger", active: true, enabled: true, customised: false },
    { code: "moi-truong", label: "Môi trường", order: 2, default_label: "Môi trường", default_order: 2, icon: "leaf", tone: "success", active: true, enabled: true, customised: false },
    { code: "giao-thong", label: "Giao thông, đường sá", order: 3, default_label: "Giao thông", default_order: 3, icon: "road", tone: "warning", active: true, enabled: true, customised: true },
    { code: "dien-chieu-sang", label: "Điện chiếu sáng", order: 4, default_label: "Điện chiếu sáng", default_order: 4, icon: "lightbulb", tone: "info", active: true, enabled: true, customised: false },
  ],
};

const SLA: identity_danhSachSLARa = {
  items: [
    { id: "01PREVIEWSLA0000000000001", work_kind: "van-ban-den", field: "", is_default: true, acknowledge_hours: 8, resolve_hours: 40, due_soon_hours: 8, escalate_leader_hours: 8, escalate_president_hours: 16, unassigned_hold_hours: 4 },
    { id: "01PREVIEWSLA0000000000002", work_kind: "don-thu", field: "", is_default: true, acknowledge_hours: 16, resolve_hours: 80, due_soon_hours: 16, escalate_leader_hours: 16, escalate_president_hours: 40, unassigned_hold_hours: 8 },
    { id: "01PREVIEWSLA0000000000003", work_kind: "phan-anh", field: "", is_default: true, acknowledge_hours: 8, resolve_hours: 40, due_soon_hours: 4, escalate_leader_hours: 8, escalate_president_hours: 16, unassigned_hold_hours: 2 },
    { id: "01PREVIEWSLA0000000000004", work_kind: "phan-anh", field: "an-ninh-trat-tu", is_default: false, acknowledge_hours: 2, resolve_hours: 16, due_soon_hours: 2, escalate_leader_hours: 2, escalate_president_hours: 4, unassigned_hold_hours: 1 },
    { id: "01PREVIEWSLA0000000000005", work_kind: "phan-anh", field: "moi-truong", is_default: false, acknowledge_hours: 8, resolve_hours: 40, due_soon_hours: 8, escalate_leader_hours: 8, escalate_president_hours: 24, unassigned_hold_hours: null },
    { id: "01PREVIEWSLA0000000000006", work_kind: "nhiem-vu", field: "", is_default: true, acknowledge_hours: 8, resolve_hours: 40, due_soon_hours: 8, escalate_leader_hours: 16, escalate_president_hours: 40, unassigned_hold_hours: null },
  ],
  problems: [],
};

const WORKING_HOURS: identity_danhSachCaLamViecRa = {
  items: [1, 2, 3, 4, 5].flatMap((weekday) => [
    { id: `01PREVIEWWH0000000000000${weekday}A`, weekday, start: "07:30", end: "11:30", note: "Ca sáng" },
    { id: `01PREVIEWWH0000000000000${weekday}B`, weekday, start: "13:30", end: "17:00", note: "Ca chiều" },
  ]),
  problems: [],
};

function holidays(year: number): identity_danhSachNgayNghiLeRa {
  return {
    items: [
      { id: `01PREVIEWHOL${year}0000000001`, date: `${year}-01-01`, name: "Tết Dương lịch" },
      { id: `01PREVIEWHOL${year}0000000002`, date: `${year}-02-16`, name: "Tết Nguyên đán" },
      { id: `01PREVIEWHOL${year}0000000003`, date: `${year}-02-17`, name: "Tết Nguyên đán" },
      { id: `01PREVIEWHOL${year}0000000004`, date: `${year}-04-26`, name: "Giỗ Tổ Hùng Vương" },
      { id: `01PREVIEWHOL${year}0000000005`, date: `${year}-04-30`, name: "Ngày Chiến thắng" },
      { id: `01PREVIEWHOL${year}0000000006`, date: `${year}-05-01`, name: "Quốc tế Lao động" },
      { id: `01PREVIEWHOL${year}0000000007`, date: `${year}-09-02`, name: "Quốc khánh" },
    ],
  };
}

function swapDays(year: number): identity_danhSachCaLamBuRa {
  return {
    items: [
      { id: `01PREVIEWSWP${year}0000000001`, date: `${year}-02-07`, start: "07:30", end: "11:30", name: "Làm bù Tết Nguyên đán" },
      { id: `01PREVIEWSWP${year}0000000002`, date: `${year}-02-07`, start: "13:30", end: "17:00", name: "Làm bù Tết Nguyên đán" },
    ],
    problems: [],
  };
}

// ── Tự động hoá — a run's `work_kind` is a real kind of work (phan-anh / van-ban-den / nhiem-vu),
//    never the job key ─────────────────────────────────────────────────────────────────────────────
const AUTOMATION: identity_automationJobsOut = {
  items: [
    {
      job: "sla_reminders",
      schedule_kind: "interval",
      configured: true,
      enabled: true,
      interval_minutes: 30,
      min_interval_minutes: 5,
      run_hour: null,
      run_minute: null,
      weekday: null,
      timezone: "Asia/Ho_Chi_Minh",
      enabled_at: "2026-09-29T01:00:00Z",
      run_requested_at: null,
      last_runs: [
        { work_kind: "phan-anh", run_id: "01PREVIEWRUN0000000000001", trigger: "schedule", claimed_at: "2026-10-08T02:30:00Z", outcome: "succeeded", records_examined: 142, notices_delivered: 9, records_without_recipient: 1, recorded_at: "2026-10-08T02:30:04Z" },
        { work_kind: "van-ban-den", run_id: "01PREVIEWRUN0000000000002", trigger: "schedule", claimed_at: "2026-10-08T02:00:00Z", outcome: "succeeded", records_examined: 140, notices_delivered: 4, records_without_recipient: 0, recorded_at: "2026-10-08T02:00:03Z" },
      ],
    },
    {
      job: "escalation",
      schedule_kind: "daily",
      configured: true,
      enabled: false,
      interval_minutes: null,
      min_interval_minutes: null,
      run_hour: 7,
      run_minute: 30,
      weekday: null,
      timezone: "Asia/Ho_Chi_Minh",
      enabled_at: null,
      run_requested_at: null,
      last_runs: [],
    },
    {
      job: "weekly_digest",
      schedule_kind: "weekly",
      configured: false,
      enabled: false,
      interval_minutes: null,
      min_interval_minutes: null,
      run_hour: null,
      run_minute: null,
      weekday: null,
      timezone: "Asia/Ho_Chi_Minh",
      enabled_at: null,
      run_requested_at: null,
      last_runs: [],
    },
  ],
};

// ── Máy chủ thư · Kênh Zalo · Nhận diện xã ────────────────────────────────────────────────────────
const MAIL: comms_mailSettingsOut = {
  configured: true,
  host: "smtp.xathangbinh.example",
  port: 587,
  security: "starttls",
  username: "thongbao@xathangbinh.example",
  from_address: "thongbao@xathangbinh.example",
  from_name: "UBND xã Thăng Bình",
  is_enabled: true,
  password_set: true,
  encryption_configured: true,
  last_test: { at: "2026-10-08T02:05:00Z", to: "c***@xathangbinh.example", ok: false, error_class: "sai-tai-khoan" },
};

const ZALO_SETTINGS: ZaloChannelSettings = {
  is_enabled: true,
  platform_ready: true,
  supported_events: [
    "nhiem-vu.sap-den-han", "nhiem-vu.qua-han", "nhiem-vu.chua-cu-nguoi", "nhiem-vu.leo-thang",
    "van-ban.sap-den-han", "van-ban.qua-han", "van-ban.chua-cu-nguoi", "van-ban.leo-thang",
    "phan-anh.sap-den-han", "phan-anh.qua-han", "phan-anh.chua-cu-nguoi", "phan-anh.leo-thang",
    "ban-tin-tuan",
  ],
  kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han", "nhiem-vu.leo-thang", "phan-anh.qua-han"],
  quiet_start: "21:00",
  quiet_end: "07:00",
  overdue_start_after_days: 1,
  overdue_repeat_every_days: 2,
  due_soon_days: 3,
  updated_at: "2026-10-05T03:20:00Z",
  updated_by: "CB-00003",
};

const ZALO_CURRENT: ZaloLinkCurrent = { linked: false, channel_enabled: true };

const ZALO_LINKED: ZaloLinkedStaff[] = [
  { staff_code: "CB-00003", staff_name: "Cán bộ C", linked_at: "2026-10-05T03:25:00Z" },
  { staff_code: "CB-00001", staff_name: "Cán bộ A", linked_at: "2026-10-06T08:10:00Z" },
];

const BRANDING: platform_brandingSettingsOut = {
  logo_public_url: "",
  web_admin_banner_public_url: "",
  updated_at: null,
};

// ── Nhật ký hệ thống — one page per service; IPs are private-range, deltas masked (rule 3) ────────
const AUDIT: Record<string, readonly audit_EntryView[]> = {
  "/api/v1/identity-audit-entries": [
    { at: "2026-10-08T02:12:00Z", actor_kind: "staff", actor_code: "CB-00003", actor_ip: "10.0.0.12", action: "org_unit.update", subject: UNIT_ONE_STOP, delta: { name: { before: "Bộ phận TN&TKQ", after: "Bộ phận Một cửa" } } },
    { at: "2026-10-07T09:40:00Z", actor_kind: "staff", actor_code: "CB-00003", actor_ip: "10.0.0.12", action: "sla.update", subject: "01PREVIEWSLA0000000000004", delta: { acknowledge_hours: { before: 4, after: 2 } } },
    { at: "2026-10-07T01:00:00Z", actor_kind: "system", actor_code: "system", actor_ip: "", action: "automation.run", subject: "sla_reminders", delta: { notices_delivered: 9 } },
  ],
  "/api/v1/documents-audit-entries": [
    { at: "2026-10-08T01:30:00Z", actor_kind: "staff", actor_code: "CB-00001", actor_ip: "10.0.0.21", action: "document_type.create", subject: "01PREVIEWDT00000000000009", delta: { label: { before: null, after: "Công văn hoả tốc" } } },
  ],
  "/api/v1/finance-audit-entries": [
    { at: "2026-10-06T07:05:00Z", actor_kind: "staff", actor_code: "CB-00002", actor_ip: "10.0.0.33", action: "capital_plan_category.update", subject: "sua-chua-cai-tao", delta: { order: { before: 3, after: 2 } } },
  ],
  "/api/v1/comms-audit-entries": [
    { at: "2026-10-05T03:20:00Z", actor_kind: "staff", actor_code: "CB-00003", actor_ip: "10.0.0.12", action: "zalo_channel.update", subject: "zalo-channel-settings", delta: { quiet_start: { before: "22:00", after: "21:00" } } },
  ],
  "/api/v1/petitions-audit-entries": [
    { at: "2026-10-04T10:15:00Z", actor_kind: "citizen", actor_code: "CD-00042", actor_ip: "", action: "petition.create", subject: "PA-7K3M9Q", delta: { status: { before: null, after: "moi-tiep-nhan" } } },
  ],
};

const SYSTEM_MESSAGE_PATHS: Record<string, keyof typeof MESSAGES> = {
  "/api/v1/petitions-system-messages": "petitions",
  "/api/v1/finance-system-messages": "finance",
  "/api/v1/reporting-system-messages": "reporting",
};

/**
 * The shared document types carry `source: ""` / `tier: 0` (the Văn bản screens never read them). The
 * Danh mục tab shows both columns, so here — and only here, `documents.fixture.ts` is another screen's —
 * the first four read as shipped with the system (`he-thong`, tier 2) and the rest as the commune's own
 * (`don-vi`, tier 1), like every other catalogue of this file.
 */
const SETTINGS_DOCUMENT_TYPES = {
  items: PREVIEW_DOCUMENT_TYPES.items.map((t, i) =>
    i < 4 ? { ...t, source: "he-thong", tier: 2 } : { ...t, source: "don-vi", tier: 1 },
  ),
};

/** Catalogue lists the Danh mục tab reads, answered from the shared fixtures (one source each). */
const SHARED_CATALOGUES: Record<string, { items: readonly unknown[] }> = {
  "/api/v1/capital-plan-categories": PREVIEW_CATEGORIES,
  "/api/v1/document-types": SETTINGS_DOCUMENT_TYPES,
  "/api/v1/task-blocs": PREVIEW_TASK_BLOCS,
  "/api/v1/task-types": PREVIEW_TASK_TYPES,
  "/api/v1/task-priorities": PREVIEW_TASK_PRIORITIES,
  "/api/v1/task-statuses": PREVIEW_TASK_STATUSES,
};

/** Every list-shaped read this file answers, path → full answer (the year-bound two are below). */
function listAnswers(url: URL): Record<string, { items: readonly unknown[] }> {
  const assetType = url.searchParams.get("asset_type_code");
  return {
    ...SHARED_CATALOGUES,
    "/api/v1/org-units": PREVIEW_SETTINGS_UNITS,
    "/api/v1/residential-units": PREVIEW_RESIDENTIAL_UNITS,
    "/api/v1/residential-unit-types": PREVIEW_RESIDENTIAL_TYPES,
    "/api/v1/map-asset-types": PREVIEW_MAP_ASSET_TYPES,
    "/api/v1/map-field-schemas": { items: MAP_FIELDS.filter((f) => assetType === null || f.asset_type_code === assetType) },
    "/api/v1/sla": SLA,
    "/api/v1/citizen-report-fields": PETITION_FIELDS,
    "/api/v1/working-hours": WORKING_HOURS,
    "/api/v1/automation-jobs": AUTOMATION,
    "/api/v1/zalo-links": { items: ZALO_LINKED },
    ...Object.fromEntries(Object.entries(SYSTEM_MESSAGE_PATHS).map(([p, m]) => [p, { items: MESSAGES[m] }])),
  };
}

const SINGLE_ANSWERS: Record<string, unknown> = {
  "/api/v1/mail-settings": MAIL,
  "/api/v1/zalo-channel-settings": ZALO_SETTINGS,
  "/api/v1/zalo-links/current": ZALO_CURRENT,
  // The commune still on the shared bot (the common case); live_link_count feeds the switch dialog.
  "/api/v1/zalo-bots/current": { has_own_bot: false, bot: null, live_link_count: 2 },
  "/api/v1/commune-branding": BRANDING,
};

const YEAR_PATHS = new Set(["/api/v1/public-holidays", "/api/v1/swap-working-days"]);

/** Whether `answerSettings` answers this path — the caller holds these unsettled for `?state=loading`. */
export function isSettingsReadPath(p: string): boolean {
  return p in SINGLE_ANSWERS || p in AUDIT || YEAR_PATHS.has(p) || p in listAnswers(new URL("http://x/"));
}

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

/**
 * The fixture answer for one read of the Cấu hình screen, or `null` when the path is not one of its
 * reads (or the method is not GET — writes belong to the shared answering machine, which refuses them).
 */
export function answerSettings(method: string, url: URL): Response | null {
  if (method !== "GET") return null;
  const p = url.pathname;
  const empty = settingsPreviewState() === "empty";
  const year = Number(url.searchParams.get("year")) || new Date().getFullYear();

  const list = listAnswers(url)[p];
  if (list !== undefined) {
    if (!empty) return json(list);
    // `problems` (SLA, calendars) stays present and empty: the contract always sends it.
    return json("problems" in list ? { items: [], problems: [] } : { items: [] });
  }
  if (p === "/api/v1/public-holidays") return json(empty ? { items: [] } : holidays(year));
  if (p === "/api/v1/swap-working-days") return json(empty ? { items: [], problems: [] } : swapDays(year));
  const audit = AUDIT[p];
  if (audit !== undefined) {
    const page: page_Result_audit_EntryView = { items: empty ? [] : [...audit], next_cursor: "", has_more: false };
    return json(page);
  }
  if (p in SINGLE_ANSWERS) {
    if (empty && p === "/api/v1/mail-settings") {
      const blank: comms_mailSettingsOut = { ...MAIL, configured: false, host: "", username: "", from_address: "", from_name: "", is_enabled: false, password_set: false };
      return json(blank);
    }
    if (p === "/api/v1/mail-settings" && settingsPreviewState() === "mail-off") {
      return json({ ...MAIL, is_enabled: false } satisfies comms_mailSettingsOut);
    }
    return json(SINGLE_ANSWERS[p]);
  }
  return null;
}
