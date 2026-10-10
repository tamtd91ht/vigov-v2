"use client";

import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  ArrowUpDown,
  CalendarClock,
  Check,
  ClipboardList,
  Clock,
  CloudOff,
  Download,
  GitBranch,
  LayoutGrid,
  List,
  Loader2,
  Minus,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  SearchX,
  TimerReset,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
  type ReactNode,
} from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { OChonCanBo } from "@/components/o-chon-can-bo";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { RecordTabStrip, recordTabDomId } from "@/components/ui/record-tabs";
import {
  activateRecordTab,
  closeAllRecordTabs,
  closeRecordTab,
  emptyRecordTabs,
  mergeRecordTabs,
  openRecordTab,
  readStoredRecordTabs,
  updateRecordTab,
  writeStoredRecordTabs,
  type RecordTab,
  type RecordTabsState,
} from "@/components/ui/record-tabs-state";
import { PageHeader } from "@/components/ui/page-header";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { duongDanBienBan } from "@/features/bien-ban/nhan-bien-ban";
import { nhanThoiDiem, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import { TRANG_DAU, type NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import {
  doiTrangThaiNhiemVu,
  downloadTaskRegister,
  getTaskCounts,
  layNhiemVu,
  laySoNhiemVu,
  taoNhiemVu,
  xoaNhiemVu,
  type CotSapXepNhiemVu,
  type LocNhiemVu,
  type StatusMoveExtras,
} from "@/lib/api/nhiem-vu";
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";
import { NO_DRILL_DOWN, type DrillDown } from "@/lib/drill-down";
import { coQuyen, QUYEN_DUYET_GIA_HAN, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen";
import { DrillDownBanner } from "@/components/drill-down-banner";
import type {
  identity_boPhanRa,
  identity_canBoChonNguoiRa,
  identity_danhBaChonNguoiRa,
  identity_khoiNhiemVuRa,
  page_Result_petitions_nhiemVuRa,
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_deNghiLuiHanRa,
  petitions_loaiNhiemVuRa,
  petitions_mucUuTienRa,
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
  petitions_taskAssignmentIn,
  petitions_taskCountsOut,
} from "@/lib/api/schema.gen";

import {
  ADD_CHILD_BUTTON,
  CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN,
  CAU_LOC_TRANG_THAI_KHONG_CO_COT,
  CHI_QUA_HAN_NHAN,
  CHI_TIET_THIEU_VAN_BAN,
  CHUA_GIAO_BO_PHAN,
  CHUA_PHAN_CONG,
  CHUA_XAC_DINH,
  CHU_THICH_HAI_O_TICK,
  COT_RONG,
  DANG_TAI_SO,
  DE_BO_PHAN_TU_PHAN_CONG,
  DANG_TAI_VAN_BAN,
  DUE_COLUMN_LABEL,
  GHI_CHU_NHIEM_VU_TOI_DA,
  GHI_CHU_LANH_DAO_GIAO_VIEC,
  GHI_CHU_TU_SINH_MA,
  KANBAN_COUNTS_ERROR,
  kanbanSharedError,
  KHONG_DOC_DUOC_VAN_BAN,
  KHONG_SO,
  MOI_BO_PHAN_NHAN,
  MOI_KHOI_NHAN,
  MOI_LOAI_NHAN,
  MOI_MUC_UU_TIEN_NHAN,
  MOI_NGUOI_THUC_HIEN_NHAN,
  MOI_NGUON_GIAO,
  MOI_NGUON_GIAO_NHAN,
  MOI_NHOM_VAN_BAN,
  MO_TA_FORM_GIAO_VIEC,
  NHAN_CHE_DO_DANH_SACH,
  NHAN_CHE_DO_KANBAN,
  NHAN_COT_MA,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_THEM_VAN_BAN,
  NHAN_TIEU_DE_THEO_VAN_BAN,
  PRIORITY_COLUMN_LABEL,
  STATUS_COLUMN_LABEL,
  TITLE_COLUMN_LABEL,
  O_TRONG,
  PHAM_VI_CUA_TOI,
  SCOPE_RELATED_LABEL,
  DUE_SOON_FILTER_LABEL,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  SO_KY_HIEU_VAN_BAN_TOI_DA,
  SO_RONG,
  TIEU_DE_KHOI_VAN_BAN,
  TIEU_DE_NHIEM_VU_TOI_DA,
  TIM_PLACEHOLDER,
  TOM_TAT_KET_QUA_TOI_DA,
  TRICH_YEU_VAN_BAN_TOI_DA,
  ariaSapXep,
  bamCotSapXep,
  canDocLaiTruocKhiLuu,
  cauLoiDanhBaGiaoViec,
  cauLoiDanhBaLanhDao,
  cauLoiDanhBaLoc,
  canhBaoSua,
  createTaskErrors,
  CREATE_TASK_FIELD_IDS,
  childCountLabel,
  childFormNote,
  kanbanColumnCount,
  kanbanPartialNote,
  danhBaChoNhatKy,
  cauTuKetLuan,
  chiaNhomVanBan,
  changedSince,
  defaultNewTaskDueInput,
  defaultTaskType,
  splitNewTaskDue,
  basicTaskEditBody,
  TASK_CHANGED_NOTE,
  TASK_TYPE_PLACEHOLDER,
  canWriteLogEntry,
  clickableTransitions,
  reasonMove,
  kanbanDropHint,
  kanbanMoveDoneText,
  kanbanMovePendingText,
  needsDirective,
  directiveTaskType,
  REGISTER_EMPTY,
  blankDocumentRow,
  usableDocumentRows,
  extensionCountText,
  cotPhaiDoc,
  docBangNhanTrangThai,
  docDanhBaChonNguoi,
  dongCuaNhom,
  duongDanTuLoc,
  formSuaTuChiTiet,
  hoanThanhTreHan,
  lateText,
  locTuDuongDan,
  maskedEmailOf,
  mergeChildPages,
  mucUuTienMacDinh,
  nhanBoDem,
  nhanCanBoNgan,
  nhanHanThe,
  nhanHoanThanhTreHan,
  nhanNgay,
  nhanNguonGiao,
  nhanNhomVanBan,
  nhanNutGoVanBan,
  nhanOTieuDe,
  nhanTrangThai,
  nhanTrongOChonCanBo,
  ngayVanBan,
  quyenNhiemVu,
  loiSauKhiDocLai,
  lyDoKhoaSua,
  placeholderNhomVanBan,
  sapXepDayDu,
  thanGiaoViec,
  thanSuaNhiemVu,
  theoThuTuXa,
  type BangNhanTrangThai,
  type SapXepSo,
  type DongVanBanNhap,
  type DongVanBanSua,
  type FormSuaNhiemVu,
  type NhomVanBan,
  type QuyenNhiemVu,
  type TrangThaiNhiemVu,
} from "./nhan-nhiem-vu";
import { BatchDeleteDialog } from "./batch-delete-bar";
import {
  BATCH_DELETE_BUTTON,
  EMPTY_SELECTION,
  SELECT_ALL_LABEL,
  batchFailureToasts,
  batchSuccessToast,
  keepFailed,
  runBatchDelete,
  selectAllState,
  selectLabel,
  selectedCountLabel,
  toggleAllSelected,
  toggleSelected,
  type TaskSelection,
  type TaskSelectionState,
} from "./batch-delete";
import { ChildTasks } from "./child-tasks";
import { StaffEmailReveal } from "./staff-email";
import { TaskStatusPipeline } from "./task-status-pipeline";
import { saveFile } from "./save-file";
import { TaskImportDialog } from "./task-import-dialog";
import { IMPORT_OPEN_BUTTON } from "./task-import";
import {
  APPROVAL_TICKED,
  APPROVAL_UNTICKED,
  REGISTER_COLUMNS,
  REGISTER_DOCS_MISSING,
  REGISTER_EXPORT_BUTTON,
  REGISTER_EXPORT_NOTE,
  REGISTER_EXPORT_REFUSED,
  REGISTER_UNKNOWN_GROUP,
  REGISTER_VIEW_LABEL,
  SUPERIOR_NOT_YET,
  registerExportDoneText,
  registerRowDocuments,
} from "./task-register";
import {
  KANBAN_CARD_ROLE,
  KANBAN_DRAG_INSTRUCTIONS,
  KANBAN_DROP_REFUSED_HINT,
  columnKeyboardCoordinates,
  dragTargets,
  draggedTask,
  dropOnKanban,
  kanbanAnnouncements,
  kanbanCollision,
} from "./kanban-drag";
import { KanbanMoveMenu } from "./kanban-move-menu";
import { NhatKyNhiemVu } from "./nhat-ky-nhiem-vu";
import { ASSIGNMENT_TITLE, canShowAssignment } from "./task-assignment";
import { TaskAssignmentBlock } from "./task-assignment-block";
import { DueDateInput } from "./due-date-input";
import { TaskDetailHost, useTaskDetailRead, useTaskDetailState } from "./task-detail-host";
import { readTaskParam, searchWithTask, useTaskDialogUrl } from "./task-dialog-url";
import { TaskExtensionHistory, TaskExtensionSection } from "./task-extension-block";
import { TaskPersonPicker } from "./task-person-picker";
import { isTaskTabId, parseTaskTabData, taskTabsStorageKey, type TaskTabData } from "./task-tabs";
import {
  BADGE_CLASS,
  CHECKBOX_CLASS,
  FILTER_SELECT_CLASS,
  FORM_SELECT_CLASS,
  HINT_CLASS,
  INPUT_CLASS,
  LABEL_CLASS,
  SECTION_CLASS,
  SECTION_TITLE_CLASS,
  TABLE_CLASS,
  TABLE_FRAME_CLASS,
  TD_CLASS,
  TEXTAREA_CLASS,
  TH_CLASS,
  daysOverdue,
  isOverdueNow,
  rowClass,
  specDefaultTaskType,
  statusDotClass,
  taskDisplayState,
  withSpecLabels,
} from "./task-spec";
import { Glyph, taskStatusIcon } from "./task-ui";

/**
 * Sổ Quản lý nhiệm vụ — `docs/ui-ux/02-nhiem-vu.md` §2 (bố cục), §3 (bộ lọc), §4.1 (bảng Kanban),
 * §4.2 (bảng danh sách), §5 (drawer chi tiết), §6 (vòng đời) và §7 (form Giao việc mới).
 *
 * Chế độ xem thứ ba của §4 — `Sổ theo dõi` §4.3 — KHÔNG có ở đây: nó lấy quá nửa số cột từ ba
 * nhóm văn bản chỉ đạo, mà tuyến đọc sổ cố ý không trả `documents` và chưa có tuyến xuất. Xem
 * `PHAN_CHUA_DUNG`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỐN LẦN TỪ CHỐI CỦA MÁY CHỦ MÀ MÀN HÌNH PHẢI NÓI ĐÚNG, và chúng là lý do màn này không có một
 * hằng chuỗi lỗi nào:
 *
 *   cha `hoan-thanh` còn con     câu từ chối LIỆT KÊ MÃ việc con còn lại
 *   xoá cha còn con              câu từ chối mang SỐ việc con
 *   duyệt lùi hạn                `task.extend` ở cổng + đúng người ghi ở `lanh_dao_giao_viec_ma`
 *   bước không có trong §6       409 kèm tên hai trạng thái
 *
 * Ở cả bốn, DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để
 * lại cho cán bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng
 * ở đâu. Nên mọi nhánh hỏng dưới đây vẽ THẲNG `KetQua.thongBao` — nguyên văn `message` máy chủ
 * viết (luật 9, cấm #2).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG NÚT THEO KHOÁ `task.*` (27/09/2026) — `quyenNhiemVu`, đọc danh sách quyền của PHIÊN. Nút
 * `+ Giao việc mới` · `✎ Sửa` · khối Chuyển trạng thái · khối Giao việc, chuyển việc (`task.assign`)
 * · ô gửi đề nghị lùi hạn · Xoá · Duyệt lùi hạn ẩn với tài khoản thiếu khoá tương ứng. ĐÓ LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: mỗi tuyến khai
 * `RequirePermission` và kiểm trên TỪNG yêu cầu (luật 5, cấm #1). Phần ĐỌC vẫn không có cổng ở
 * client — thiếu `task.read` thì câu 403 của máy chủ ra nguyên văn, cùng khuôn `document.read`.
 * Lớp hai của ADR 0038 KHÔNG phải một khoá quyền: nó là phép so mã cán bộ với cột trên bản ghi, và
 * nó vẫn chạy đầy đủ SAU lớp khoá.
 *
 * LỚP CSS: `.man-nhiem-vu` NAY ĐÃ CÓ — `globals.css:920-929`, thêm 23/09/2026 cùng lúc mở mục
 * menu. Khối này trước viết "`.man-nhiem-vu` không tồn tại", đúng lúc viết và hết đúng vài giờ
 * sau; sửa vì một chú thích nói thiếu thứ không còn thiếu là chú thích đẩy người sau đi thêm lần
 * hai. Lý do gốc vẫn giữ nguyên và vẫn đúng: KHÔNG mượn lớp của màn khác — nó trông gần đúng hôm
 * nay rồi lệch hẳn vào ngày lớp ấy đổi vì cái nó thật sự phục vụ.
 *
 * Bốn lớp của bảng Kanban §4.1 — `.bang-kanban` · `.cot-kanban` · `.danh-sach-the` ·
 * `.the-nhiem-vu` — CŨNG ĐÃ CÓ (`globals.css:1213-1279`): năm cột xếp dọc trên điện thoại và nằm
 * cạnh nhau từ 768px, có chủ ý — lý do ghi ngay tại chỗ trong `globals.css`.
 */

/**
 * Rows per read of the list and the register — the server's maximum (`limit` 1–100). The prototype
 * reads up to 300 rows at once; this list is cursor-paged, so it reads 100 and `Xem thêm` appends the
 * next 100 (owner 07/10/2026, ADR 0076 lần 2 #5). Sorting stays on the server: a client sort of one
 * page would order 100 rows of a register that has more.
 */
const SO_DONG_MOI_TRANG = 100;

/**
 * Cards read for ONE Kanban column — the server's maximum, for the same reason.
 *
 * Kanban KHÔNG có phân trang từng cột — năm ngăn xếp con trỏ song song là năm chỗ để lạc, và đặc
 * tả không vẽ nút trang nào trên bảng. Đầu cột hiện TỔNG THẬT (`/task-counts`); cột có nhiều hơn số
 * thẻ đang hiện thì dưới cột nói ra (`kanbanPartialNote`) và cán bộ sang Danh sách.
 */
const SO_THE_MOI_COT = 100;

/** Hai chế độ xem dựng được (§2, §4). Kanban là MẶC ĐỊNH, đúng tiêu đề §4.1. */
/** `so-theo-doi` = §4.3 Sổ theo dõi (W6): the list's rows read with `include=documents`. */
type CheDoXem = "kanban" | "danh-sach" | "so-theo-doi";

export type TrangThaiTai<T> =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: T };

function taiTu<T>(daTai: { khoa: string; kq: KetQua<T> } | null, khoa: string): TrangThaiTai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

/** Năm câu trả lời của năm cột Kanban, đọc trong cùng một lượt. */
type DaTaiKanban = {
  khoa: string;
  cot: readonly { ma: TrangThaiNhiemVu; kq: KetQua<page_Result_petitions_nhiemVuRa> }[];
};

/** Lấy pha của MỘT cột ra khỏi lượt đọc chung. Cùng phép so khoá với `taiTu`, nên gọi lại nó. */
function taiCot(
  daTai: DaTaiKanban | null,
  khoa: string,
  ma: TrangThaiNhiemVu,
): TrangThaiTai<page_Result_petitions_nhiemVuRa> {
  const c = daTai !== null && daTai.khoa === khoa ? daTai.cot.find((x) => x.ma === ma) : undefined;
  return taiTu(c === undefined ? null : { khoa, kq: c.kq }, khoa);
}

/**
 * The key to DRAW by: `key`, or — while the same question is being asked again after a write — the key
 * of the answer already on screen.
 *
 * A write in the drawer (a status move, `Lãnh đạo đã phê duyệt`, `Cấp trên đã công nhận`…) bumps the
 * re-read counter, which is the LAST segment of every key. Drawing that as "loading" swapped the board
 * for five grey blocks and the list for a dimmed copy, so the page behind the drawer shrank, scrolled
 * and grew back (customer sheet row 17). Same filters, only a newer counter → the previous answer stays
 * exactly as it was until the new one replaces it in place. A FAILED previous answer is never kept:
 * `Tải lại` must show that it is reading.
 */
export function softRefresh(
  stored:
    | { readonly khoa: string; readonly kq: KetQua<unknown> }
    | { readonly khoa: string; readonly cot: readonly { readonly kq: KetQua<unknown> }[] }
    | null,
  key: string,
): string {
  if (stored === null || stored.khoa === key) return key;
  const withoutCounter = (k: string) => k.slice(0, k.lastIndexOf("|"));
  if (withoutCounter(stored.khoa) !== withoutCounter(key)) return key;
  const ok = "kq" in stored ? stored.kq.ok : stored.cot.length > 0 && stored.cot.every((c) => c.kq.ok);
  return ok ? stored.khoa : key;
}

/** Bộ lọc đang chọn trên màn hình. Cùng hình dạng với `LocNhiemVu`, trừ phân trang. */
type BoLoc = Omit<LocNhiemVu, "limit" | "cursor">;

const KHONG_LOC: BoLoc = {};

/**
 * Bấm tiêu đề một cột sắp được của bảng §4.2 → bộ lọc mới VÀ ngăn xếp con trỏ mới.
 *
 * TRẢ CẢ HAI, KHÔNG CHỈ BỘ LỌC, vì hai thứ ấy không tách được: con trỏ mang `sort`/`order` bên trong
 * và máy chủ trả 400 `invalid_cursor` cho con trỏ của một cách sắp khác (`core/page/page.go:456-457`).
 * The screen now pages with `Xem thêm` (one read key per filter + sort), so a new sort starts a new
 * key and drops the appended pages; `TRANG_DAU` stays the answer for any caller keeping a stack.
 */
export function bamSapXep(
  loc: BoLoc,
  cot: CotSapXepNhiemVu,
): { readonly loc: BoLoc; readonly nganXep: NganXepConTro } {
  const moi = bamCotSapXep(sapXepDayDu(loc), cot);
  return { loc: { ...loc, sapXep: moi.cot, chieu: moi.chieu }, nganXep: TRANG_DAU };
}

/**
 * `useSyncExternalStore` đòi một hàm đăng ký. Màn này CỐ Ý không nghe `popstate` CHO BỘ LỌC: đường
 * dẫn chỉ là bộ lọc LÚC MỞ; sau đó bộ lọc sống trong state và màn hình tự ghi nó ra (`replaceState`
 * không bắn `popstate`). Nút Quay lại vì vậy rời màn chứ không lật lại từng ô lọc — đúng lý do dùng
 * `replace`. The ONE history move this screen answers is the task dialog's (`useTaskDialogUrl`).
 */
function khongTheoDoiDuongDan(): () => void {
  return () => {};
}

/**
 * The address bar's query WITHOUT `?task=`: the dialog pushes and replaces that parameter, and a
 * snapshot that changed with it would rebuild `loc` — and re-read the register — on every opening.
 */
function filterSearchSnapshot(): string {
  return searchWithTask(window.location.search, null);
}

/** Ba danh mục đổ vào ô chọn, đọc một lần cho cả màn. */
export type DanhMucNhiemVu = {
  readonly loai: readonly petitions_loaiNhiemVuRa[];
  readonly mucUuTien: readonly petitions_mucUuTienRa[];
  readonly khoi: readonly identity_khoiNhiemVuRa[];
  readonly boPhan: readonly identity_boPhanRa[];
};

const KHONG_DANH_MUC: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/** Nhãn của một mã danh mục. Mã lạ hiện NGUYÊN VĂN — không dấu gạch, không im lặng bỏ qua. */
function nhanDanhMuc(
  ds: readonly { code: string; label: string }[],
  ma: string,
): string {
  if (ma === "") return O_TRONG;
  return ds.find((m) => m.code === ma)?.label ?? ma;
}

/** `#RRGGBB` — the only shape of a catalogue `color` this screen puts into an inline style. */
const HEX_COLOUR = /^#[0-9a-fA-F]{6}$/;

/**
 * Colour of the Kanban card's top strip (ADR 0082 #6, replacing ADR 0076 lần 2 #10's rank rule):
 *   1. the colour the commune chose for that priority in its catalogue (`color`, `#RRGGBB`) — an
 *      inline style, since a per-commune value cannot be a class baked into the bundle;
 *   2. otherwise by CODE, as the prototype (`TaskCard.tsx:35,46` + `lib/task-display.ts:114-118`):
 *      `khan` red, `cao` tangerine (v2's `warning-500`), everything else — `thuong`, no priority, an
 *      unknown code — brand.
 * A value that is not `#RRGGBB` is ignored (falls to 2), never written into the style as typed.
 * The colour is the SECOND signal: the card says the priority in words too.
 */
export function priorityStrip(
  scale: readonly { readonly code: string; readonly color: string | null }[],
  code: string,
): { readonly className: string; readonly style?: { readonly backgroundColor: string } } {
  const chosen = code === "" ? null : (scale.find((m) => m.code === code)?.color ?? null);
  if (chosen !== null && HEX_COLOUR.test(chosen)) return { className: "", style: { backgroundColor: chosen } };
  if (code === "khan") return { className: "bg-danger-500" };
  if (code === "cao") return { className: "bg-warning-500" };
  return { className: "bg-brand-500" };
}

/**
 * Drawer đang mở: nhiệm vụ để vẽ, khối văn bản §5.4, và LƯỢT ĐỌC chi tiết đang chờ.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO DRAWER ĐỌC TUYẾN CHI TIẾT, KHÔNG DÙNG DÒNG CỦA SỔ
 *
 * Tuyến sổ `GET /api/v1/tasks` CỐ Ý không trả `documents` (`service-petitions/internal/http/
 * nhiem_vu.go:167-185`): trên sổ, trường vắng mặt nghĩa là "không phục vụ ở đây", KHÔNG phải
 * "không có văn bản". Vẽ khối §5.4 từ dòng của sổ là báo một khối rỗng cho một nhiệm vụ có ba văn
 * bản. Nên `mo` KHÔNG BAO GIỜ lấy `documents` từ dòng được bấm: khối vào pha `dangTai` và tuyến
 * chi tiết (luôn trả một mảng, có thể rỗng) là nguồn duy nhất của nó.
 *
 * SAU MỘT LẦN GHI: ĐỌC LẠI, KHÔNG GỘP TAY. Phản hồi của `…/status` không mang `documents`
 * (`service-petitions/internal/app/nhiem_vu.go:862`). `ghiXong` lấy ngay các trường vô hướng của
 * phản hồi, GIỮ NGUYÊN khối văn bản đang hiện (một lần đổi trạng thái không đụng tới văn bản), và
 * tăng `luotDoc` để đọc lại chi tiết — câu trả lời ấy thay cả hai. Tăng lượt còn làm một việc thứ
 * hai: một lượt đọc chi tiết GỬI TRƯỚC lần ghi mà về SAU nó sẽ bị bỏ, thay vì đè trạng thái cũ lên
 * trạng thái máy chủ vừa trả.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export type DrawerNhiemVu = {
  readonly nhiemVu: petitions_nhiemVuRa;
  readonly vanBan: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  /** Tăng ở mỗi lần phải đọc lại chi tiết. Câu trả lời mang lượt cũ bị bỏ. */
  readonly luotDoc: number;
};

export type ViecDrawer =
  /** Bấm `Mở NV…` trên thẻ hoặc dòng — `nhiemVu` là DÒNG CỦA SỔ, không có `documents`. */
  | { readonly loai: "mo"; readonly nhiemVu: petitions_nhiemVuRa }
  /** Tuyến chi tiết trả lời lượt `luotDoc` của nhiệm vụ `ma`. */
  | {
      readonly loai: "chiTietVe";
      readonly ma: string;
      readonly luotDoc: number;
      readonly kq: KetQua<petitions_nhiemVuRa>;
    }
  /** Một tuyến ghi trả về nhiệm vụ (tạo, đổi trạng thái). */
  | { readonly loai: "ghiXong"; readonly nhiemVu: petitions_nhiemVuRa }
  /**
   * Nhiệm vụ `ma` vừa đổi ở NƠI KHÁC — một quyết định lùi hạn ở hàng chờ, tuyến không trả nhiệm
   * vụ. Drawer đang mở đúng mã ấy thì đọc lại; mở mã khác hoặc đóng thì không làm gì.
   */
  | { readonly loai: "docLai"; readonly ma: string }
  | { readonly loai: "dong" };

export function chuyenDrawer(s: DrawerNhiemVu | null, v: ViecDrawer): DrawerNhiemVu | null {
  switch (v.loai) {
    case "dong":
      return null;
    case "mo":
      return { nhiemVu: v.nhiemVu, vanBan: { pha: "dangTai" }, luotDoc: (s?.luotDoc ?? 0) + 1 };
    case "ghiXong": {
      const docs = v.nhiemVu.documents;
      const cungViec = s !== null && s.nhiemVu.code === v.nhiemVu.code;
      const vanBan: DrawerNhiemVu["vanBan"] = Array.isArray(docs)
        ? { pha: "xong", duLieu: docs }
        : cungViec
          ? s.vanBan
          : { pha: "dangTai" };
      return { nhiemVu: v.nhiemVu, vanBan, luotDoc: (s?.luotDoc ?? 0) + 1 };
    }
    case "docLai":
      // GIỮ nguyên những gì đang hiện, chỉ tăng lượt: tuyến chi tiết thay cả nhiệm vụ lẫn khối
      // văn bản khi về, và một lượt đọc cũ về sau bị bỏ.
      if (s === null || s.nhiemVu.code !== v.ma) return s;
      return { ...s, luotDoc: s.luotDoc + 1 };
    case "chiTietVe": {
      if (s === null || s.nhiemVu.code !== v.ma || s.luotDoc !== v.luotDoc) return s;
      if (!v.kq.ok) return { ...s, vanBan: { pha: "loi", thongBao: v.kq.thongBao } };
      const docs = v.kq.duLieu.documents;
      // Hợp đồng hứa một MẢNG ở tuyến này. Vắng mặt là hứa bị vỡ — báo lỗi, không đọc thành rỗng.
      if (!Array.isArray(docs)) {
        return { ...s, nhiemVu: v.kq.duLieu, vanBan: { pha: "loi", thongBao: CHI_TIET_THIEU_VAN_BAN } };
      }
      return { ...s, nhiemVu: v.kq.duLieu, vanBan: { pha: "xong", duLieu: docs } };
    }
  }
}

/**
 * The detail PANEL: its record tabs (owner 05/10/2026), whether it is on screen, and the drawer of
 * the ACTIVE tab (`chuyenDrawer`, unchanged). One reducer, so a row click opens the drawer AND its
 * tab in one step — two states updated by two calls would render once with a drawer whose tab does
 * not exist yet, and the tab-read effect would fire a needless read in that gap.
 *
 *   open (row, card, queue, child, parent, `?task=`, a created task)   tab added or activated, shown
 *   tab click / ←→                                                      `chonTab`, its read follows
 *   panel ✕ / Esc / Back (`dong`)                                       HIDDEN — the tabs stay
 *   tab ✕                                                               neighbour (right, else left); last → hidden
 *   Đóng tất cả                                                         every tab gone, hidden
 *   delete succeeded (`boTab`)                                          that record's tab gone, hidden
 *
 * ONLY THE ACTIVE TAB HAS A DRAWER. Switching to a tab whose drawer is not loaded drops the drawer
 * and the effect in `SoNhiemVu` reads the task through `layNhiemVu` — the same `task.read` +
 * commune check as every read. A refusal stays IN THAT TAB's content (`tabError`), closable.
 * The detail is keyed by code, so half-typed input in one tab is discarded by switching — the
 * same as moving to a parent or a child today; there is no dirty guard to ask first.
 */
export type TaskPanel = {
  readonly drawer: DrawerNhiemVu | null;
  readonly tabs: RecordTabsState<TaskTabData>;
  /** On screen. `false` with tabs kept is the HIDDEN panel of rule 4. */
  readonly open: boolean;
  /** The active tab's read was refused: the server's sentence, verbatim. */
  readonly tabError: { readonly code: string; readonly message: string } | null;
};

export const NO_TASK_PANEL: TaskPanel = {
  drawer: null,
  tabs: emptyRecordTabs<TaskTabData>(),
  open: false,
  tabError: null,
};

export type TaskPanelAction =
  /** The drawer's own moves. `dong` HIDES the panel; the tabs stay. */
  | ViecDrawer
  | { readonly loai: "chonTab"; readonly ma: string }
  /** `layNhiemVu(ma)` answered for the tab that was active when it was asked. */
  | { readonly loai: "tabVe"; readonly ma: string; readonly kq: KetQua<petitions_nhiemVuRa> }
  | { readonly loai: "dongTab"; readonly ma: string }
  | { readonly loai: "dongTatCa" }
  /** These records were deleted: their tabs go; the panel hides if it showed one of them. */
  | { readonly loai: "boTab"; readonly ma: readonly string[] }
  /** Tabs read back from sessionStorage, folded under the open ones. */
  | { readonly loai: "khoiPhuc"; readonly tabs: readonly RecordTab<TaskTabData>[] };

function taskTabData(n: petitions_nhiemVuRa): TaskTabData {
  return { title: n.title, status: n.status };
}

export function chuyenTaskPanel(s: TaskPanel, v: TaskPanelAction): TaskPanel {
  switch (v.loai) {
    case "dong":
      return { ...s, drawer: null, open: false, tabError: null };
    case "chonTab": {
      const tabs = activateRecordTab(s.tabs, v.ma);
      if (tabs.active !== v.ma) return s;
      const drawer = s.drawer?.nhiemVu.code === v.ma ? s.drawer : null;
      return { ...s, tabs, drawer, open: true, tabError: null };
    }
    case "tabVe": {
      // A read overtaken by another switch, or arriving after the panel was hidden: dropped.
      if (!s.open || s.tabs.active !== v.ma || s.drawer !== null) return s;
      if (!v.kq.ok) return { ...s, tabError: { code: v.ma, message: v.kq.thongBao } };
      // A task code never changes (ADR 0065 NV3: `PATCH` has no `code`), so the answer is always
      // the tab's own record.
      return chuyenTaskPanel(s, { loai: "mo", nhiemVu: v.kq.duLieu });
    }
    case "dongTab": {
      const tabs = closeRecordTab(s.tabs, v.ma);
      if (tabs === s.tabs) return s;
      if (tabs.active === null) return { ...s, tabs, drawer: null, open: false, tabError: null };
      return {
        ...s,
        tabs,
        drawer: s.drawer?.nhiemVu.code === tabs.active ? s.drawer : null,
        tabError: s.tabError?.code === tabs.active ? s.tabError : null,
      };
    }
    case "dongTatCa":
      return { ...NO_TASK_PANEL, tabs: closeAllRecordTabs(s.tabs) };
    case "boTab": {
      let tabs = s.tabs;
      for (const ma of v.ma) tabs = closeRecordTab(tabs, ma);
      const shownGone = s.tabs.active !== null && v.ma.includes(s.tabs.active);
      return shownGone ? { ...s, tabs, drawer: null, open: false, tabError: null } : { ...s, tabs };
    }
    case "khoiPhuc":
      return { ...s, tabs: mergeRecordTabs(s.tabs, v.tabs) };
    default: {
      const drawer = chuyenDrawer(s.drawer, v);
      if (drawer === s.drawer) return s;
      if (drawer === null) return { ...s, drawer };
      const code = drawer.nhiemVu.code;
      const data = taskTabData(drawer.nhiemVu);
      const before = s.drawer?.nhiemVu.code ?? null;
      // An OPENING: a row/card/link (`mo`), or a write answered with no drawer on screen (a task
      // just created from the list's form opens its detail, as before tabs).
      if (v.loai === "mo" || before === null) {
        return { ...s, drawer, tabs: openRecordTab(s.tabs, code, data), open: true, tabError: null };
      }
      // A write or a re-read of the record on screen: same code (ADR 0065 NV3), new title/status.
      return { ...s, drawer, tabs: updateRecordTab(s.tabs, code, data) };
    }
  }
}

/**
 * The content of a tab whose detail is not on screen: being read, or refused. The heading carries
 * the dialog's accessible name exactly as the detail's does, from the tab's own label. Exported for
 * `TaskDetailHost`, which draws it for every screen that opens the detail.
 */
export function TaskTabPending({
  code,
  title,
  error,
  onHide,
}: {
  code: string;
  title: string;
  error: string | null;
  onHide: () => void;
}) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-white">
      {/* `border-b` alone: with `border-solid` beside it, preflight off, the other sides paint 3px. */}
      <header className="border-line flex shrink-0 items-start gap-3 border-b px-5 py-4">
        <div className="flex min-w-0 flex-1 items-start gap-2">
          <span className="border-line text-ink-muted mt-0.5 shrink-0 rounded border bg-[#F7FAFC] px-1.5 py-0.5 text-[10.5px] font-semibold">
            {code}
          </span>
          <h2
            id={TASK_DETAIL_TITLE_ID}
            className="text-navy m-0 min-w-0 text-[16px] leading-snug font-bold [overflow-wrap:anywhere]"
          >
            {title}
          </h2>
        </div>
        <CloseDetailButton onClick={onHide} />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        {/* Always in the DOM: a live region inserted later is not always announced. */}
        <p className="an-thi-giac" role="status">
          {error === null ? TASK_TAB_LOADING : ""}
        </p>
        {error === null ? (
          <div className="space-y-3" aria-hidden="true">
            <Skeleton className="h-7 w-3/4" />
            <Skeleton className="h-24 w-full" />
          </div>
        ) : (
          <p className="thong-bao-loi mt-0" role="alert">
            {error}
          </p>
        )}
      </div>
    </div>
  );
}

/** The detail's ✕ (spec 07 §1: `Button variant="outline" size="icon"`, `aria-label="Đóng"`). */
export const CLOSE_DETAIL_LABEL = "Đóng";
function CloseDetailButton({ onClick }: { onClick: () => void }) {
  return (
    <Button type="button" variant="outline" className="size-9 shrink-0 px-0" aria-label={CLOSE_DETAIL_LABEL} onClick={onClick}>
      <X aria-hidden="true" focusable="false" className="size-4" />
    </Button>
  );
}

/** Said while a tab's task is read. */
export const TASK_TAB_LOADING = "Đang tải nhiệm vụ…";
/** The record-tab strip's accessible name, and the prefix of its tab ids. */
export const TASK_TABS_LABEL = "Nhiệm vụ đang mở";
const TASK_TAB_ID_PREFIX = "task-record-tab";
const TASK_TAB_PANEL_ID = "task-record-panel";
export function closeTaskTabLabel(code: string): string {
  return `Đóng tab ${code}`;
}

/**
 * Câu của dải lọc Tổng quan trên sổ này — nói ra đúng những gì màn tạm tắt để số dòng bằng con số.
 * Kanban tắt vì mỗi cột chỉ đọc một trang và không vẽ các trạng thái rẽ nhánh (`Tạm dừng` là một).
 */
export const DRILL_DOWN_NOTE_TASKS =
  "Bộ lọc, ô tìm, phạm vi và chế độ Kanban tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

/** Spec 02 footer: `Hiển thị {n} nhiệm vụ.` — `n` is the SERVER's count, `—` until it is read. */
export function shownCountText(total: number | null): string {
  return total === null ? `Hiển thị ${O_TRONG} nhiệm vụ.` : nhanBoDem(total);
}

/** Total of a `/task-counts` answer — every status, under the view's filters. Never a page's length. */
export function countsTotal(counts: TrangThaiTai<petitions_taskCountsOut>): number | null {
  return counts.pha === "xong" ? counts.duLieu.by_status.reduce((sum, r) => sum + r.count, 0) : null;
}

/** `Xem thêm` under the list (owner 07/10/2026, ADR 0076 lần 2 #5). */
export const LOAD_MORE_LABEL = "Xem thêm";

/** Spec 06 success toast of `Giao việc mới`. */
export const CREATE_TASK_DONE = "Đã giao nhiệm vụ";

/** Pages appended by `Xem thêm` to the first page of read key `khoa`. */
type MorePages = {
  readonly khoa: string;
  readonly items: readonly petitions_nhiemVuRa[];
  readonly cursor: string;
  readonly hasMore: boolean;
};

export function SoNhiemVu({
  drillDown = NO_DRILL_DOWN,
  openTask = null,
}: {
  /** Lọc mở từ trang Tổng quan, đọc ở máy chủ (`app/nhiem-vu/page.tsx`). */
  drillDown?: DrillDown<"tasks">;
  /** `?task=NV19` (`task-link.ts`), read server-side: open this task's detail once, on arrival. */
  openTask?: string | null;
} = {}) {
  const drillDownActive = drillDown.kind === "active";
  /**
   * BỘ LỌC §3 CÓ HAI NGUỒN, THEO THỨ TỰ: cán bộ đã đổi một ô lọc trên màn (`locDaDoi`) thì đó là
   * bộ lọc; chưa đổi gì thì bộ lọc là ĐƯỜNG DẪN lúc mở (`locTuDuongDan`) — đúng thứ một đường dẫn
   * chia sẻ phải mang tới.
   *
   * `chuoiDuongDan` LÀ `null` Ở MÁY CHỦ VÀ LÚC HYDRATE (`useSyncExternalStore`, ảnh chụp máy chủ
   * `null`): máy chủ không có `window`, và đọc `location` sớm ở client là HTML hai bên lệch nhau.
   * KHÔNG LƯỢT ĐỌC SỔ NÀO CHẠY KHI NÓ CÒN `null` (`daDocDuongDan`): đọc sổ ngay với bộ lọc rỗng là
   * HAI lượt đọc, và lượt không lọc về SAU sẽ vẽ cả quyển sổ dưới những ô lọc đang nói là một lát cắt.
   */
  const [locDaDoi, datLocDaDoi] = useState<BoLoc | null>(null);
  const chuoiDuongDan = useSyncExternalStore(khongTheoDoiDuongDan, filterSearchSnapshot, () => null);
  // `useMemo` vì `locTuDuongDan` trả một đối tượng MỚI mỗi lần gọi: không nhớ thì mỗi lần vẽ là một
  // `loc` mới, và ba `useEffect` đọc sổ theo `loc` sẽ gọi mạng lại ở mọi lần vẽ.
  //
  // ĐƯỜNG DẪN MANG LỌC TỔNG QUAN (`drillDown` khác `none`) THÌ CÁC Ô LỌC §3 TRÊN ĐƯỜNG DẪN BỊ BỎ QUA:
  // hợp lệ thì chỉ lọc ấy được gửi (con số phải bằng số dòng); hỏng thì câu trên màn nói "đang hiện
  // toàn bộ danh sách", và nó chỉ đúng khi không ô lọc nào của đường dẫn lén đi kèm.
  const locDuongDan = useMemo<BoLoc | null>(() => {
    if (drillDown.kind === "active") {
      return { metric: drillDown.metric, from: drillDown.period?.from, to: drillDown.period?.to };
    }
    if (drillDown.kind === "invalid") return KHONG_LOC;
    return chuoiDuongDan === null ? null : locTuDuongDan(chuoiDuongDan);
  }, [chuoiDuongDan, drillDown]);
  // LỌC TỔNG QUAN ĐANG BẬT: cán bộ chỉ đổi được THỨ TỰ (bấm đầu cột) — thứ tự không đổi số dòng. Mọi
  // thứ khác của `locDaDoi` bị bỏ, để không lối nào ghép thêm một ô lọc vào lát cắt của con số.
  // `useMemo` BẮT BUỘC ở nhánh lọc: nó dựng một đối tượng MỚI, và `loc` mới mỗi lần vẽ là ba effect
  // đọc sổ gọi mạng lại mỗi lần vẽ — một vòng lặp, vì mỗi câu trả lời lại gây một lần vẽ.
  const loc: BoLoc = useMemo(
    () =>
      drillDownActive && locDuongDan !== null
        ? { ...locDuongDan, sapXep: locDaDoi?.sapXep, chieu: locDaDoi?.chieu }
        : (locDaDoi ?? locDuongDan ?? KHONG_LOC),
    [drillDownActive, locDuongDan, locDaDoi],
  );
  const daDocDuongDan = locDaDoi !== null || locDuongDan !== null;
  const [tim, datTim] = useState("");
  const [lanTai, datLanTai] = useState(0);
  const [cheDoXem, datCheDoXem] = useState<CheDoXem>("kanban");
  // Kanban không cho số dòng bằng con số: một trang mỗi cột, không có cột rẽ nhánh. Lọc Tổng quan bật
  // thì luôn là Danh sách.
  const viewMode: CheDoXem = drillDownActive ? "danh-sach" : cheDoXem;
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);
  // The catalogues have been answered (well or not) — `danhMuc` alone cannot tell "not read yet"
  // from "read, empty".
  const [typesRead, setTypesRead] = useState(false);
  /**
   * The type the Sổ theo dõi reads: `undefined` while the catalogue is being read, `null` when no
   * active type exists (or the read failed) — `directiveTaskType`, the prototype's choice.
   */
  const directiveType = typesRead ? directiveTaskType(danhMuc.loai) : undefined;
  /**
   * THE QUERY OF THE VIEW ON SCREEN. The Sổ theo dõi forces the type `directiveTaskType` chose,
   * whatever the `Loại` filter of the other views holds (that filter is hidden there); `null` reads
   * the book without a type filter, as the prototype does (ADR 0082 #9). The counts and the export
   * read the same query, so the footer and the file say what the table shows.
   *
   * ROOT TASKS ONLY (`roots`) on every view, as the prototype (ADR 0082 #8) — except a Tổng quan
   * drill-down, whose rows must equal a figure that counts sub-tasks too (ADR 0053).
   *
   * The book waits (`registerHeld`) only while the catalogue is being read: reading it before would
   * be one read without the type, then a second with it.
   */
  const registerHeld = viewMode === "so-theo-doi" && directiveType === undefined;
  const viewLoc = useMemo<BoLoc>(() => {
    const scoped: BoLoc = drillDownActive ? loc : { ...loc, roots: true };
    return viewMode === "so-theo-doi" && typeof directiveType === "string"
      ? { ...scoped, loai: directiveType }
      : viewMode === "so-theo-doi"
        ? { ...scoped, loai: undefined }
        : scoped;
  }, [loc, viewMode, directiveType, drillDownActive]);

  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_nhiemVuRa>;
  } | null>(null);
  const [more, setMore] = useState<MorePages | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<{ khoa: string; message: string } | null>(null);
  const [daTaiKanban, datDaTaiKanban] = useState<DaTaiKanban | null>(null);
  /** `null` = chưa đọc xong. Xem `docBangNhanTrangThai` cho ba nhánh. */
  const [kqNhanTT, datKqNhanTT] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(
    null,
  );
  // `null` = chưa đọc xong. Giữ NGUYÊN `KetQua`: ô chọn cán bộ phải nói được câu lỗi của máy chủ,
  // và phải phân biệt "đang tải" với "tải hỏng" (`docDanhBaChonNguoi`).
  const [kqDanhBa, datKqDanhBa] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  // Danh bạ RIÊNG của ô `Lãnh đạo giao việc`: chỉ người cầm `task.extend` (ADR 0038). Cùng ba pha.
  const [kqDanhBaLanhDao, datKqDanhBaLanhDao] =
    useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  // The panel: record tabs + the active tab's drawer (`chuyenTaskPanel`). `guiDrawer` keeps its
  // name — every drawer move below goes through the panel reducer, which forwards it.
  const [panel, guiDrawer] = useReducer(chuyenTaskPanel, NO_TASK_PANEL);
  const drawer = panel.drawer;
  // The detail's own state (`task-detail-host.tsx`): the write in flight — shared with the create form
  // below, as before — the extension-queue tick (NOT tied to `lanTai`: re-reading requests after every
  // write on the register is a wasted call) and the child form.
  const detailState = useTaskDetailState();
  const [moFormTao, datMoFormTao] = useState(false);
  // Kanban moves — the answer belongs on the board (a toast), not in a drawer that may be showing
  // another task.
  const [kanbanPending, setKanbanPending] = useState<KanbanMove["pending"]>(null);
  const [kanbanResult, setKanbanResult] = useState<KanbanMoveResult | null>(null);
  // Real per-status totals (#15): the Kanban headers AND the footer of every view, keyed like the view.
  const [countsLoaded, setCountsLoaded] = useState<{
    khoa: string;
    kq: KetQua<petitions_taskCountsOut>;
  } | null>(null);
  // `Xuất Excel` of the Sổ theo dõi (W6): one export at a time; the answer is a toast.
  const [exporting, setExporting] = useState(false);
  // `⬆ Nhập từ Excel` (W7, §8). Focus returns to the opening button when the dialog closes.
  const [importOpen, setImportOpen] = useState(false);
  const importButtonRef = useRef<HTMLButtonElement>(null);
  // `🗑 Xoá đã chọn` (§2). Survives view switches on purpose: the clerk selects across views; the
  // row always says how many are selected, so nothing is chosen out of sight silently.
  const [selection, setSelection] = useState<TaskSelectionState>(EMPTY_SELECTION);
  const [batchProgress, setBatchProgress] = useState<{ done: number; total: number } | null>(null);
  // The `Xoá đã chọn` dialog. Opened from the filter row; holds the reason and the run.
  const [batchOpen, setBatchOpen] = useState(false);

  // The register view reads with `include=documents`: a different answer, so a different key —
  // switching view must not show a page read without the documents.
  //
  // `lanTai` IS THE LAST SEGMENT of every key, on purpose: `softRefresh` (below) compares the rest to
  // tell "the same question asked again after a write" from "another question".
  const khoa = `${JSON.stringify(viewLoc)}|${viewMode === "so-theo-doi" ? "docs" : ""}|${lanTai}`;
  // Kanban KHÔNG mang con trỏ: nó không phân trang, nên bộ lọc và lần ghi gần nhất là tất cả những
  // gì làm câu trả lời cũ hết hiệu lực.
  const khoaKanban = `${JSON.stringify(loc)}|${lanTai}`;
  const khoaCounts = `${JSON.stringify(viewLoc)}|${lanTai}`;

  /**
   * GHI bộ lọc lên thanh địa chỉ mỗi lần CÁN BỘ đổi nó — `history.replaceState`, KHÔNG
   * `router.replace`. Chưa đổi gì thì không ghi: đường dẫn lúc mở đang đúng là bộ lọc đang hiện.
   *
   * `router.replace` là một lượt điều hướng mềm: trang này `force-dynamic`, nên mỗi lần tick một ô
   * lọc là một lượt máy chủ dựng lại trang và đọc lại cấu hình xã chỉ để đổi thanh địa chỉ.
   * `replaceState` thì App Router của Next đồng bộ sẵn (từ 14.1) mà không gọi máy chủ. `replace`,
   * không `push`: mỗi ô lọc là một mục lịch sử thì nút Quay lại phải bấm mười lần mới rời được màn.
   *
   * GIỮ `#…`: liên kết `#hang-cho-lui-han` của drawer dùng neo, và ghi đè mất nó là cắt đường ấy.
   * KEEPS `?task=` too (ADR 0068 §Sửa đổi 05/10/2026 #5): a filter change under an open dialog must
   * not drop the dialog's entry, or Back would no longer close it.
   */
  useEffect(() => {
    if (locDaDoi === null) return;
    // Lọc Tổng quan đang bật: đường dẫn ĐANG đúng là thứ màn hiện. Ghi đè nó bằng bộ lọc §3 là xoá
    // `metric` khỏi thanh địa chỉ, và lần tải lại sau đó mở cả quyển sổ.
    if (drillDownActive) return;
    const chuoi = duongDanTuLoc(locDaDoi);
    const { pathname, search, hash } = window.location;
    const query = chuoi === "" ? "" : `?${chuoi}`;
    const openInUrl = readTaskParam(search);
    const moi = `${pathname}${openInUrl === null ? query : searchWithTask(query, openInUrl)}${hash}`;
    if (moi !== `${pathname}${search}${hash}`) window.history.replaceState(null, "", moi);
  }, [locDaDoi, drillDownActive]);

  useEffect(() => {
    if (!daDocDuongDan) return;
    // CHỈ ĐỌC CHẾ ĐỘ ĐANG XEM. Không có dòng này thì mỗi lần đổi bộ lọc trên Kanban là SÁU lời gọi
    // thay vì năm, và lời gọi thứ sáu đọc một trang không ai vẽ ra.
    if (viewMode === "kanban") return;
    if (registerHeld) return;
    let bo = false;
    // CÁCH SẮP LUÔN ĐI TRÊN DÂY, kể cả khi cán bộ chưa bấm cột nào: mũi tên trên đầu cột vẽ từ
    // `sapXepDayDu(loc)`, nên gửi đúng giá trị ấy là điều kiện để mũi tên nói thật về câu hỏi đã gửi.
    const sx = sapXepDayDu(viewLoc);
    laySoNhiemVu({
      ...viewLoc,
      sapXep: sx.cot,
      chieu: sx.chieu,
      limit: SO_DONG_MOI_TRANG,
      includeDocuments: viewMode === "so-theo-doi",
    }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [viewLoc, khoa, viewMode, daDocDuongDan, registerHeld]);

  /**
   * KANBAN ĐỌC CÙNG TUYẾN VÀ CÙNG BỘ LỌC VỚI DANH SÁCH — qua đúng `laySoNhiemVu`, nên mười tên
   * tham số truy vấn vẫn nằm ở một chỗ duy nhất (`themLocVaoTruyVan`). Hai chỗ ghép truy vấn là
   * hai chỗ sẽ trôi khỏi nhau, và cái trôi sẽ là cái không ai mở ra đọc.
   *
   * KHÁC ĐÚNG MỘT ĐIỀU: mỗi cột một lời gọi, mang thêm `status=<mã cột>`. Một trang chung cho cả
   * năm cột sẽ cho ra những cột rỗng chỉ vì trang ấy chưa tới lượt chúng — một cột rỗng vì phân
   * trang trông y hệt một cột rỗng vì xã không có việc nào.
   */
  useEffect(() => {
    if (!daDocDuongDan || viewMode !== "kanban") return;
    let bo = false;
    const ds = cotPhaiDoc(loc.trangThai);
    Promise.all(
      ds.map(async (ma) => ({
        ma,
        // Root tasks only (ADR 0082 #8). Kanban never runs under a drill-down (`viewMode` above).
        kq: await laySoNhiemVu({ ...loc, roots: true, trangThai: ma, limit: SO_THE_MOI_COT }),
      })),
    ).then((cot) => {
      if (!bo) datDaTaiKanban({ khoa: khoaKanban, cot });
    });
    return () => {
      bo = true;
    };
  }, [loc, khoaKanban, viewMode, daDocDuongDan]);

  // THE COUNTS OF THE VIEW ON SCREEN, through the same filter builder (`appendTaskFilters`): the Kanban
  // headers and the footer's `Hiển thị {n} nhiệm vụ.` A total is never the length of a page. Read
  // separately: a failed count must not hold the rows back, nor the other way round.
  useEffect(() => {
    if (!daDocDuongDan || registerHeld) return;
    let bo = false;
    getTaskCounts(viewLoc).then((kq) => {
      if (!bo) setCountsLoaded({ khoa: khoaCounts, kq });
    });
    return () => {
      bo = true;
    };
  }, [viewLoc, khoaCounts, daDocDuongDan, registerHeld]);

  // NĂM DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN. Một danh mục hỏng thì ô lọc tương ứng rỗng — KHÔNG làm
  // hỏng quyển sổ: năm câu trả lời rời nhau, mỗi cái nói chuyện của nó. Bảng trạng thái hỏng thì lui
  // về thứ tự mặc định KÈM một câu cảnh báo (`docBangNhanTrangThai`).
  useEffect(() => {
    let bo = false;
    Promise.all([
      layLoaiNhiemVu(),
      layMucUuTienNhiemVu(),
      layKhoiNhiemVu(),
      layDanhMucBoPhan(),
      layTrangThaiNhiemVu(),
    ]).then(([loai, uuTien, khoiNV, boPhan, nhanTT]) => {
      if (bo) return;
      datDanhMuc({
        loai: loai.ok ? loai.duLieu.items : [],
        mucUuTien: uuTien.ok ? uuTien.duLieu.items : [],
        khoi: khoiNV.ok ? khoiNV.duLieu.items : [],
        boPhan: boPhan.ok ? boPhan.duLieu.items : [],
      });
      setTypesRead(true);
      datKqNhanTT(nhanTT);
    });
    // DANH BẠ CHỌN NGƯỜI — MỘT LẦN CHO CẢ MÀN, dùng chung cho ô lọc §3 và form Giao việc §7. Đọc
    // RIÊNG, không nằm trong `Promise.all` trên: danh bạ chậm hay hỏng không được giữ năm danh mục
    // kia lại, và ngược lại.
    layDanhBaChonNguoi().then((kq) => {
      if (!bo) datKqDanhBa(kq);
    });
    // Lời gọi THỨ HAI, không lọc tại chỗ từ lời gọi trên: danh bạ không mang quyền của ai, và chỉ
    // `identity` biết ai đang cầm `task.extend` trong xã.
    layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN).then((kq) => {
      if (!bo) datKqDanhBaLanhDao(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  // CHI TIẾT CỦA DRAWER — xem `chuyenDrawer`. Chạy lại ở mỗi lượt đọc mới (mở, hoặc sau một lần
  // ghi); `ma` và `luotDoc` đi kèm câu trả lời để reducer bỏ câu trả lời của lượt đã cũ.
  const maDrawer = drawer?.nhiemVu.code ?? null;
  useTaskDetailRead(drawer, guiDrawer);

  // `?task=NV19` from another screen (`task-link.ts`), or a Back / Forward onto such an entry: read
  // the task through the detail route — the same `task.read` + commune check as every read — and
  // open it through the SAME drawer action as a click on a row. A refusal (one 404 sentence for
  // every case) is shown verbatim above the list. The detail is read twice on this path (here, then
  // by the drawer's own effect): accepted, so the drawer keeps exactly one way of opening.
  const [openTaskError, setOpenTaskError] = useState<string | null>(null);
  useEffect(() => {
    if (openTask === null) return;
    let cancelled = false;
    layNhiemVu(openTask).then((kq) => {
      if (cancelled) return;
      if (!kq.ok) {
        setOpenTaskError(kq.thongBao);
        return;
      }
      // Inline rather than `openDrawer`: a component function in this effect would be a dependency.
      guiDrawer({ loai: "mo", nhiemVu: kq.duLieu });
    });
    return () => {
      cancelled = true;
    };
  }, [openTask]);
  /**
   * How every click opens the detail — a row, a card, a child task. The address bar follows from
   * `maDrawer` (`useTaskDialogUrl` below), not from here: one place decides push vs replace for every
   * way of opening.
   */
  function openDrawer(n: petitions_nhiemVuRa) {
    guiDrawer({ loai: "mo", nhiemVu: n });
  }
  // ADR 0068 §Sửa đổi 05/10/2026 #5, with record tabs: the address bar names the ACTIVE tab while
  // the panel is on screen. Opening from the list PUSHES (nothing shown → a code); switching or
  // closing a tab with another left REPLACES (a code → another code); hiding goes Back.
  const activeTab = panel.tabs.active;
  const shownCode = panel.open ? activeTab : null;
  useTaskDialogUrl(shownCode, (code) => {
    if (code === null) {
      guiDrawer({ loai: "dong" });
      return;
    }
    if (code === shownCode) return;
    if (panel.tabs.tabs.some((t) => t.id === code)) {
      guiDrawer({ loai: "chonTab", ma: code });
      return;
    }
    layNhiemVu(code).then((kq) => {
      if (!kq.ok) {
        setOpenTaskError(kq.thongBao);
        return;
      }
      openDrawer(kq.duLieu);
    });
  });

  // THE ACTIVE TAB'S READ, when its drawer is not on screen (a tab switch, Back onto a kept tab).
  // The cleanup drops an answer overtaken by the next switch; the reducer drops one that arrives
  // for a tab no longer active. The drawer's own effect then reads the detail once more — the same
  // accepted double read as `?task=` on load, so the drawer keeps exactly one way of opening.
  const tabNeedsRead =
    panel.open && activeTab !== null && maDrawer !== activeTab && panel.tabError?.code !== activeTab;
  useEffect(() => {
    if (!tabNeedsRead || activeTab === null) return;
    let cancelled = false;
    layNhiemVu(activeTab).then((kq) => {
      if (!cancelled) guiDrawer({ loai: "tabVe", ma: activeTab, kq });
    });
    return () => {
      cancelled = true;
    };
  }, [tabNeedsRead, activeTab]);

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có mã cán bộ — và không có mã thì
  // không so được với `lanh_dao_giao_viec_ma`, nên nút duyệt lùi hạn ẩn (luật 1, cấm #1).
  const maNguoiDangNhap = phien !== null && phien.ok ? phien.duLieu.staff.code : "";

  /**
   * RECORD TABS SURVIVE A RELOAD in sessionStorage (this browser tab only), keyed by commune host
   * AND staff code (`taskTabsStorageKey`). No staff code yet → nothing read, nothing written. Read
   * ONCE per key, then folded UNDER the tabs already open (a `?task=` link opened on load stays
   * active). The panel itself is not restored: a reload without `?task=` shows the list.
   * Another key appearing (a different officer in this browser tab) first drops every tab, so one
   * officer's tabs are never written under — or shown to — the next.
   */
  const restoredTabsKey = useRef<string | null>(null);
  useEffect(() => {
    const key = taskTabsStorageKey(window.location.host, maNguoiDangNhap);
    if (key === null || restoredTabsKey.current === key) return;
    if (restoredTabsKey.current !== null) guiDrawer({ loai: "dongTatCa" });
    restoredTabsKey.current = key;
    const stored = readStoredRecordTabs(key, parseTaskTabData).filter((t) => isTaskTabId(t.id));
    if (stored.length > 0) guiDrawer({ loai: "khoiPhuc", tabs: stored });
  }, [maNguoiDangNhap]);
  useEffect(() => {
    const key = taskTabsStorageKey(window.location.host, maNguoiDangNhap);
    // Written only once this key has been read: before that, writing would erase what is stored.
    if (key === null || restoredTabsKey.current !== key) return;
    writeStoredRecordTabs(key, panel.tabs.tabs);
  }, [panel.tabs.tabs, maNguoiDangNhap]);
  // Cùng chiều FAIL CLOSED: phiên chưa đọc xong hoặc đọc hỏng thì MỌI cổng nút đóng.
  const quyen = quyenNhiemVu(phien !== null && phien.ok ? phien.duLieu.permissions : null);
  // `Xem` of the assignee's address (ADR 0082 #3) — `task.read`, closed while the session is unread.
  const canRevealEmail = phien !== null && phien.ok && coQuyen(phien.duLieu.permissions, QUYEN_XEM_NHIEM_VU);
  // Bảng tra họ tên theo mã — dựng MỘT LẦN mỗi lần danh bạ về, dùng chung cho thẻ, dòng và drawer.
  // Không gọi mạng thêm lần nào: danh bạ đã đọc một lần cho cả màn ở khối danh mục trên.
  const danhBaMa = useMemo(() => danhBaChoNhatKy(kqDanhBa), [kqDanhBa]);

  // A held register is "loading" while the catalogue is read — never another view's rows.
  const held: TrangThaiTai<never> | null = registerHeld ? { pha: "dangTai" } : null;
  // After a write (`lanTai` bumped, same filters), the previous answer STAYS on screen as it was until
  // the new one replaces it — see `softRefresh`. The key of what is drawn follows it, so the `Xem thêm`
  // pages of that answer stay too until the re-read lands.
  const shownKey = softRefresh(daTai, khoa);
  const so = held ?? taiTu(daTai, shownKey);
  const counts = held ?? taiTu(countsLoaded, softRefresh(countsLoaded, khoaCounts));
  const boardKey = softRefresh(daTaiKanban, khoaKanban);
  const cotKanban: readonly CotKanban[] = cotPhaiDoc(loc.trangThai).map((ma) => ({
    ma,
    tai: taiCot(daTaiKanban, boardKey, ma),
  }));
  const tenBoPhan = new Map(danhMuc.boPhan.map((b) => [b.id, b.name]));
  // THE SPEC'S FIXED WORDS over the commune's ORDER (owner 07/10/2026, `withSpecLabels`). The
  // fallback warning still says when the commune's table could not be read.
  const { bang: bangNhanXa, canhBao: canhBaoNhanTT } = docBangNhanTrangThai(kqNhanTT);
  const nhanTT = withSpecLabels(bangNhanXa);

  // `Xem thêm`: the first page of this key plus the pages appended to it. An answer for another key
  // (filters, sort or view changed meanwhile) is never drawn as this one's.
  const extra = more !== null && more.khoa === shownKey ? more : null;
  const firstPage = so.pha === "xong" ? so.duLieu : null;
  const listItems = firstPage === null ? [] : mergeChildPages(firstPage.items, extra?.items ?? []);
  const nextCursor = extra !== null ? extra.cursor : (firstPage?.next_cursor ?? "");
  const hasMore = (extra !== null ? extra.hasMore : (firstPage?.has_more ?? false)) && nextCursor !== "";
  const moreErrorShown = moreError !== null && moreError.khoa === shownKey ? moreError.message : null;

  function loadMore(): void {
    if (loadingMore || !hasMore) return;
    // The cursor belongs to the answer ON SCREEN, which may be the one `softRefresh` kept.
    const key = shownKey;
    const cursor = nextCursor;
    const sx = sapXepDayDu(viewLoc);
    setLoadingMore(true);
    laySoNhiemVu({
      ...viewLoc,
      sapXep: sx.cot,
      chieu: sx.chieu,
      limit: SO_DONG_MOI_TRANG,
      cursor,
      includeDocuments: viewMode === "so-theo-doi",
    }).then((kq) => {
      setLoadingMore(false);
      if (!kq.ok) {
        setMoreError({ khoa: key, message: kq.thongBao });
        return;
      }
      setMoreError(null);
      setMore((m) => ({
        khoa: key,
        items: [...(m !== null && m.khoa === key ? m.items : []), ...kq.duLieu.items],
        cursor: kq.duLieu.next_cursor,
        hasMore: kq.duLieu.has_more,
      }));
    });
  }

  /** A new filter is a new read key: the `Xem thêm` pages of the old one are simply not drawn. */
  function datLocMoi(moi: BoLoc): void {
    datLocDaDoi(moi);
  }

  /**
   * NO OPTIMISTIC MOVE. The card stays where it is, marked busy, until the server answers; only then
   * does the board reload. The outcome is a toast (spec 03): `Đã chuyển sang “X”`, or the server's
   * sentence verbatim — the list of unfinished child tasks lives in it.
   */
  function moveOnKanban(task: petitions_nhiemVuRa, target: TrangThaiNhiemVu, via: KanbanMoveVia) {
    if (kanbanPending !== null) return;
    setKanbanResult(null);
    setKanbanPending({ code: task.code, target });
    moveTaskStatus(task.code, target, via).then((r) => {
      setKanbanPending(null);
      setKanbanResult(r);
      if (!r.ok) {
        toast.error(r.message);
        return;
      }
      toast.success(kanbanMoveToast(nhanTT, target));
      // The drawer re-reads only if it shows this very task (`docLai`); the board re-reads always.
      guiDrawer({ loai: "docLai", ma: task.code });
      datLanTai((n) => n + 1);
    });
  }

  /**
   * One soft-delete call per selected task, sequentially, one shared reason (`runBatchDelete`). NO
   * OPTIMISTIC REMOVAL: rows leave the board only when the register is read again after the last
   * answer. Failed tasks stay selected; the outcome is told by toasts (spec 02 §4).
   */
  async function runBatch(reason: string): Promise<void> {
    const tasks = [...selection.values()];
    if (tasks.length === 0 || batchProgress !== null) return;
    setBatchProgress({ done: 0, total: tasks.length });
    const results = await runBatchDelete(tasks, reason, xoaNhiemVu, (done) =>
      setBatchProgress({ done, total: tasks.length }),
    );
    setBatchProgress(null);
    const ok = batchSuccessToast(results);
    if (ok !== null) toast.success(ok);
    for (const line of batchFailureToasts(results)) toast.error(line);
    setSelection((s) => keepFailed(s, results));
    setBatchOpen(false);
    // A drawer showing a task that is now gone would show a record that is gone.
    // Their tabs go too; a panel showing one of them hides.
    const removed = results.filter((r) => r.ok).map((r) => r.code);
    if (removed.length > 0) guiDrawer({ loai: "boTab", ma: removed });
    datLanTai((n) => n + 1);
  }

  /**
   * `Xuất Excel` — the register under the SAME query and sort the screen shows (`viewLoc`,
   * `sapXepDayDu`), every matching row; the columns are the server's. A refusal (422 too large, 503
   * names unavailable, 409 no due-soon threshold) is the server's sentence, verbatim, as a toast;
   * nothing is downloaded then.
   */
  function exportRegister(): void {
    if (exporting || registerHeld) return;
    setExporting(true);
    const sx = sapXepDayDu(viewLoc);
    downloadTaskRegister({ ...viewLoc, sapXep: sx.cot, chieu: sx.chieu }).then((r) => {
      setExporting(false);
      if (!r.ok) {
        toast.error(`${REGISTER_EXPORT_REFUSED} ${r.thongBao}`);
        return;
      }
      saveFile(r.duLieu.blob, r.duLieu.fileName);
      toast.success(registerExportDoneText(r.duLieu.fileName));
    });
  }

  // `null` without `task.delete`: no checkbox anywhere (convenience — the route checks, rule 5).
  const taskSelection: TaskSelection | null = quyen.xoa
    ? {
        selected: selection,
        toggle: (t) => setSelection((s) => toggleSelected(s, t)),
        toggleAll: (rows) => setSelection((s) => toggleAllSelected(s, rows)),
        disabled: batchProgress !== null,
      }
    : null;

  // PRESENTATION ONLY (spec §8b) — nothing below reads the network or changes what is sent.
  // The previous answer, while a new key loads: shown dimmed and inert, never acted on.
  const staleItems =
    so.pha === "dangTai" && daTai !== null && daTai.kq.ok && daTai.kq.duLieu.items.length > 0
      ? daTai.kq.duLieu.items
      : null;
  // "Nothing at all" vs "nothing under these filters": no filter key set (sort keys aside), no
  // drill-down. Only then may the screen say the commune has no task yet.
  const noFilter =
    !drillDownActive &&
    Object.entries(loc).every(([k, v]) => k === "sapXep" || k === "chieu" || v === undefined);
  const openCreate = () => {
    datMoFormTao(true);
    detailState.setChildFormFor(null);
  };
  const emptyList = noFilter ? (
    <EmptyState
      icon={ClipboardList}
      title="Chưa có nhiệm vụ nào"
      description={quyen.giaoViec ? "Bấm “Giao việc mới” hoặc nhập từ Excel để bắt đầu." : undefined}
      action={
        quyen.giaoViec && !moFormTao ? (
          <Button type="button" variant="secondary" icon={<Glyph icon={Plus} />} onClick={openCreate}>
            Giao việc mới
          </Button>
        ) : undefined
      }
    />
  ) : (
    <EmptyState
      icon={SearchX}
      title={SO_RONG}
      description={drillDownActive ? undefined : "Thử đổi hoặc bỏ bớt bộ lọc."}
    />
  );
  const listTable = (items: readonly petitions_nhiemVuRa[]) => (
    <BangNhiemVu
      nhiemVu={items}
      danhMuc={danhMuc}
      danhBa={danhBaMa}
      nhanTT={nhanTT}
      tenBoPhan={tenBoPhan}
      bayGio={new Date()}
      maDangMo={maDrawer}
      moNhiemVu={openDrawer}
      sapXep={sapXepDayDu(loc)}
      doiSapXep={(cot) => datLocDaDoi(bamSapXep(loc, cot).loc)}
      selection={taskSelection}
      empty={emptyList}
    />
  );
  const kanbanLoading =
    viewMode === "kanban" && cotKanban.length > 0 && cotKanban.every((c) => c.tai.pha === "dangTai");
  const selectionOn = quyen.xoa && selection.size > 0;

  return (
    <>
    <PageHeader
      className="mb-5"
      title="Quản lý nhiệm vụ"
      subtitle="Giao việc từ kết luận họp, theo dõi tiến độ và đôn đốc tự động."
      actions={
        quyen.giaoViec ? (
          <>
            {/* The prototype's pair, in its order: `[Nhập từ Excel]` outline, then `[+ Giao việc
                mới]` solid — both `task.create` (the import IS creation; the route checks again).
                Each opens a dialog; neither is a toggle. */}
            <Button
              ref={importButtonRef}
              type="button"
              variant="outline"
              icon={<Upload aria-hidden="true" focusable="false" className="size-4" />}
              aria-haspopup="dialog"
              onClick={() => setImportOpen(true)}
            >
              {IMPORT_OPEN_BUTTON}
            </Button>
            <Button
              type="button"
              variant="primary"
              icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
              aria-haspopup="dialog"
              onClick={openCreate}
            >
              Giao việc mới
            </Button>
          </>
        ) : undefined
      }
    />
    {/* Direct children lose their legacy vertical margins: the section's `gap` is the one spacing
        between blocks. The prototype's page: header → ONE filter row → the board or the table →
        `Hiển thị N nhiệm vụ.` — no card around the filters, no block in between. */}
    <section
      className="man-nhiem-vu mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0"
      aria-labelledby="tieu-de-so-nhiem-vu"
    >
      {/* Kept for the section's accessible name; the page's visible title is the `<h1>` above. */}
      <h2 id="tieu-de-so-nhiem-vu" className="an-thi-giac">
        Sổ nhiệm vụ của xã
      </h2>

      <KhoiChuaDung />

      {openTaskError !== null && (
        <p className="thong-bao-loi" role="alert">
          {openTaskError}
        </p>
      )}

      <CanhBaoNhanTrangThai canhBao={canhBaoNhanTT} />

      {/* THE ONE FILTER ROW, in the prototype's order, the view switch at its right end. Under an
          overview drill-down it stays in sight but DISABLED: the banner below says why, and the
          rows must equal the overview's number (`DRILL_DOWN_NOTE_TASKS`). */}
      <HangLoc
        loc={loc}
        tim={tim}
        datTim={datTim}
        datLoc={datLocMoi}
        danhMuc={danhMuc}
        danhBa={kqDanhBa}
        nhanTT={nhanTT}
        disabled={drillDownActive}
        hideType={viewMode === "so-theo-doi"}
        end={
          <>
            {/* `Đã chọn N nhiệm vụ · Xoá đã chọn` beside the view switch, as in the prototype (spec 02
                §2 item 11). The reason is asked in the dialog — mandatory, one for all (rule 7). */}
            {selectionOn && (
              <div className="ml-auto flex items-center gap-2">
                <span className="text-ink-muted text-[12px]">{selectedCountLabel(selection.size)}</span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="text-danger hover:not-disabled:text-danger"
                  icon={<Trash2 aria-hidden="true" focusable="false" className="size-3.5" />}
                  aria-haspopup="dialog"
                  onClick={() => setBatchOpen(true)}
                >
                  {BATCH_DELETE_BUTTON}
                </Button>
              </div>
            )}
            <ViewSwitch
              value={viewMode}
              disabled={drillDownActive}
              pushRight={!selectionOn}
              onChange={datCheDoXem}
            />
          </>
        }
      />

      {/* A status filter arriving in the address bar (Sổ tay lãnh đạo, a shared link) has no
          control of its own on this row — the prototype has no status filter. Said here, with the
          way out, so a filtered list never passes for the whole register. */}
      {!drillDownActive && loc.trangThai !== undefined && (
        <p className="m-0 flex flex-wrap items-center gap-2 text-sm text-ink-700" role="status">
          Đang lọc theo trạng thái “{nhanTrangThai(nhanTT, loc.trangThai)}”.
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => datLocMoi({ ...loc, trangThai: undefined })}
          >
            Bỏ lọc trạng thái
          </Button>
        </p>
      )}

      <DrillDownBanner
        drillDown={drillDown}
        clearHref="/nhiem-vu"
        note={DRILL_DOWN_NOTE_TASKS}
        showInvalid={locDaDoi === null}
      />

      {importOpen && quyen.giaoViec && (
        <TaskImportDialog
          onClose={() => {
            setImportOpen(false);
            importButtonRef.current?.focus();
          }}
          // The new rows are read from the server, never spliced in: the register reloads.
          onImported={() => datLanTai((n) => n + 1)}
        />
      )}

      {moFormTao && quyen.giaoViec && (
        <FormGiaoViec
          dialog
          taskScreen
          danhMuc={danhMuc}
          danhBa={kqDanhBa}
          danhBaLanhDao={kqDanhBaLanhDao}
          // `POST /api/v1/tasks` nhận `documents`; màn Biên bản thì không — xem prop.
          coDanhSachVanBan
          staffSearch
          dangGui={detailState.sending}
          // The server's answer is a toast on this screen (spec 06); field errors stay inline.
          loi={null}
          huy={() => datMoFormTao(false)}
          giaoViec={(than, khoaChongTrung) => {
            detailState.setSending(true);
            taoNhiemVu(than, khoaChongTrung).then((kq) => {
              detailState.setSending(false);
              if (!kq.ok) {
                toast.error(kq.thongBao);
                return;
              }
              // Spec 06: toast, close — the new task shows in the register, which reloads.
              toast.success(CREATE_TASK_DONE);
              datMoFormTao(false);
              datLanTai((n) => n + 1);
            });
          }}
        />
      )}

      {/* `Xoá đã chọn` — the prototype's confirm dialog, with OUR mandatory shared reason (rule 7:
          a soft delete always says why). The outcome is a toast; failed tasks stay selected. */}
      {quyen.xoa && batchOpen && (
        <BatchDeleteDialog
          tasks={[...selection.values()].map((t) => ({
            code: t.code,
            title: t.title,
            holder: nhanCanBoNgan(t.assignee, danhBaMa, CHUA_PHAN_CONG),
          }))}
          progress={batchProgress}
          onRun={(reason) => void runBatch(reason)}
          onClose={() => {
            if (batchProgress === null) setBatchOpen(false);
          }}
        />
      )}

      {/* THE VIEW, then its footer (spec 02 §3). */}
      <div className="flex min-w-0 flex-col gap-3 [&>*]:my-0">
        {kanbanLoading && <ViewSkeleton />}

        {viewMode === "kanban" && !kanbanLoading && (
          <BangKanban
            cot={cotKanban}
            danhMuc={danhMuc}
            danhBa={danhBaMa}
            unitNames={tenBoPhan}
            nhanTT={nhanTT}
            bayGio={new Date()}
            maDangMo={maDrawer}
            moNhiemVu={openDrawer}
            counts={counts}
            move={{
              permissions: quyen,
              staffCode: maNguoiDangNhap,
              pending: kanbanPending,
              result: kanbanResult,
              move: moveOnKanban,
            }}
            selection={taskSelection}
          />
        )}

        {/* `Xuất Excel` of the Sổ theo dõi — kept (owner 07/10/2026, ADR 0076 lần 2 #5), restyled: one
            outline button above the book; the outcome is a toast. */}
        {viewMode === "so-theo-doi" && (
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              icon={
                exporting ? (
                  <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
                ) : (
                  <Download aria-hidden="true" focusable="false" className="size-3.5" />
                )
              }
              disabled={exporting}
              aria-busy={exporting || undefined}
              title={REGISTER_EXPORT_NOTE}
              onClick={exportRegister}
            >
              {REGISTER_EXPORT_BUTTON}
            </Button>
          </div>
        )}

        {/* LOADING (spec 02 §3): five grey blocks for the first read. A re-read of the list keeps the
            PREVIOUS page on screen, dimmed and `inert` — an answer to the old question is shown,
            never acted on. The sentence stays the live region. */}
        {viewMode !== "kanban" && so.pha === "dangTai" && (
          <>
            <p className="an-thi-giac" role="status">
              {DANG_TAI_SO}
            </p>
            {viewMode === "danh-sach" && staleItems !== null ? (
              <div inert className="pointer-events-none opacity-60 transition-opacity">
                {listTable(staleItems)}
              </div>
            ) : (
              <ViewSkeleton />
            )}
          </>
        )}
        {/* LOAD ERROR (spec §8b, kept — owner 07/10/2026 #13): the server's sentence VERBATIM stays
            the alert; `Tải lại` asks the same read again through the re-read key (`lanTai`). */}
        {viewMode !== "kanban" && so.pha === "loi" && (
          <EmptyState
            icon={CloudOff}
            title="Chưa tải được sổ nhiệm vụ"
            description={
              <span className="text-danger-600" role="alert">
                {so.thongBao}
              </span>
            }
            action={
              <Button
                type="button"
                variant="secondary"
                icon={<Glyph icon={RefreshCw} />}
                onClick={() => datLanTai((n) => n + 1)}
              >
                Tải lại
              </Button>
            }
          />
        )}

        {viewMode === "so-theo-doi" && so.pha === "xong" && (
          <BangSoTheoDoi
            nhiemVu={listItems}
            danhMuc={danhMuc}
            danhBa={danhBaMa}
            nhanTT={nhanTT}
            tenBoPhan={tenBoPhan}
            bayGio={new Date()}
            maDangMo={maDrawer}
            moNhiemVu={openDrawer}
            selection={taskSelection}
            empty={
              // The book's own sentence (prototype `TaskRegisterTable.tsx:114`); the filter hint only
              // when a filter is on, so an empty book is not read as "the commune has none".
              <EmptyState
                icon={ClipboardList}
                title={REGISTER_EMPTY}
                description={noFilter ? undefined : "Thử đổi hoặc bỏ bớt bộ lọc."}
              />
            }
          />
        )}
        {viewMode === "danh-sach" && so.pha === "xong" && listTable(listItems)}

        {viewMode !== "kanban" && so.pha === "xong" && moreErrorShown !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {moreErrorShown}
          </p>
        )}
        {viewMode !== "kanban" && so.pha === "xong" && hasMore && (
          <div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={loadingMore}
              aria-busy={loadingMore || undefined}
              onClick={loadMore}
            >
              {LOAD_MORE_LABEL}
            </Button>
          </div>
        )}

        {/* `Hiển thị N nhiệm vụ.` — N is the SERVER's count under the view's filters, never the
            length of the page on screen (`/task-counts`). */}
        {(viewMode === "kanban" ? !kanbanLoading : so.pha === "xong") && (
          <p className="text-ink-muted m-0 text-[12px]">{shownCountText(countsTotal(counts))}</p>
        )}
      </div>

      {/* THE DETAIL IS A DRAWER PINNED RIGHT (spec 07 §Vỏ: 72rem, square, white, navy/30 backdrop) —
          the record tabs above its content are KEPT (owner 07/10/2026, ADR 0076 lần 2 #1). Esc asks
          the same `dong` as the ✕ — it HIDES the panel, the record tabs stay. The dialog stays
          mounted across tab switches and parent / child moves (only the content is keyed by code),
          so focus still returns to the row that first opened it. The drawer and every write it makes
          are `TaskDetailHost` — the one wiring every screen shares (owner 09/10/2026). */}
      {shownCode !== null && (
        <TaskDetailHost
          shownCode={shownCode}
          drawer={drawer}
          pendingTitle={panel.tabs.tabs.find((t) => t.id === shownCode)?.data.title ?? ""}
          pendingError={panel.tabError?.code === shownCode ? panel.tabError.message : null}
          dispatch={guiDrawer}
          context={{
            catalogues: danhMuc,
            statusLabels: nhanTT,
            unitNames: tenBoPhan,
            directory: kqDanhBa,
            leaderDirectory: kqDanhBaLanhDao,
            staffCode: maNguoiDangNhap,
            permissions: quyen,
            canRevealEmail,
          }}
          state={detailState}
          onRegisterChanged={() => datLanTai((n) => n + 1)}
          // The record is gone: its tab goes with it, and the panel hides as before tabs.
          onDeleted={(code) => guiDrawer({ loai: "boTab", ma: [code] })}
          onChildFormToggle={() => datMoFormTao(false)}
          tabs={{
            panelId: TASK_TAB_PANEL_ID,
            labelledBy: recordTabDomId(TASK_TAB_ID_PREFIX, shownCode),
            strip: (
              <RecordTabStrip
                label={TASK_TABS_LABEL}
                idPrefix={TASK_TAB_ID_PREFIX}
                panelId={TASK_TAB_PANEL_ID}
                // Code + title + the status word: what the register row already shows.
                tabs={panel.tabs.tabs.map((t) => ({
                  id: t.id,
                  title: `[${t.id}] ${t.data.title}`,
                  secondary: nhanTrangThai(nhanTT, t.data.status),
                  icon: taskStatusIcon(t.data.status),
                }))}
                activeId={shownCode}
                onSelect={(ma) => {
                  if (ma === shownCode) return;
                  guiDrawer({ loai: "chonTab", ma });
                }}
                onClose={(ma) => guiDrawer({ loai: "dongTab", ma })}
                onCloseAll={() => guiDrawer({ loai: "dongTatCa" })}
                closeTabLabel={closeTaskTabLabel}
              />
            ),
          }}
        />
      )}
    </section>
    </>
  );
}

/**
 * The drawer's box over `LargeDialog`'s defaults (spec 07 §Vỏ): `w-[72rem] max-w-[98vw]`, white,
 * square, the prototype's left shadow, a navy/30 backdrop. From 768px only — below it the panel stays
 * the whole screen (`LargeDialog`), which is what the smallest supported width needs. `cn` lets these
 * win over the component's own `md:`/`xl:` width, radius, shadow and backdrop.
 */
export const DETAIL_DRAWER_CLASS =
  "bg-white backdrop:bg-navy/30 md:w-[72rem] md:max-w-[98vw] md:rounded-none md:shadow-[-10px_0_36px_rgba(16,43,67,0.13)] xl:w-[72rem]";

/** Spec 02 §3: five grey blocks while a view is read for the first time. Decorative. */
function ViewSkeleton() {
  return (
    <div className="grid grid-cols-5 gap-3.5" aria-hidden="true">
      {[0, 1, 2, 3, 4].map((i) => (
        <Skeleton key={i} className="h-64 w-full" />
      ))}
    </div>
  );
}

/** The three views (spec 02 §2 item 12): Kanban · Danh sách · Sổ theo dõi. */
const VIEWS = [
  ["kanban", NHAN_CHE_DO_KANBAN, LayoutGrid],
  ["danh-sach", NHAN_CHE_DO_DANH_SACH, List],
  ["so-theo-doi", REGISTER_VIEW_LABEL, ClipboardList],
] as const;

/**
 * The view switch — spec 02 §2 item 12, verbatim classes (`bg-surface` = our `bg-canvas`). Icons carry
 * an explicit `size-3.5`: lucide draws 24px by default.
 */
function ViewSwitch({
  value,
  disabled,
  pushRight,
  onChange,
}: {
  value: CheDoXem;
  disabled: boolean;
  /** `ml-auto` when no selection cluster stands before it. */
  pushRight: boolean;
  onChange: (v: CheDoXem) => void;
}) {
  return (
    <div
      className={cn("border-line bg-canvas flex gap-1 rounded-[10px] border p-1", pushRight && "ml-auto")}
      role="group"
      aria-label="Chế độ xem"
    >
      {VIEWS.map(([v, label, Icon]) => (
        <button
          key={v}
          type="button"
          className={cn(
            "flex cursor-pointer items-center gap-1.5 rounded-[8px] border-0 px-3 py-1.5 text-[12.5px] font-semibold [font-family:inherit]",
            "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-50",
            value === v ? "text-navy shadow-card bg-white" : "text-ink-muted bg-transparent",
          )}
          aria-pressed={value === v}
          disabled={disabled}
          onClick={() => onChange(v)}
        >
          <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5" />
          {label}
        </button>
      ))}
    </div>
  );
}

/** Toast of a Kanban move the server accepted (spec 03). */
export function kanbanMoveToast(labels: BangNhanTrangThai, target: string): string {
  return `Đã chuyển sang “${nhanTrangThai(labels, target)}”`;
}

/** Toast after the detail's soft-delete call succeeded. */
export function taskDeletedText(code: string): string {
  return `Đã xoá nhiệm vụ ${code}.`;
}

/**
 * Nhãn trạng thái đang là nhãn MẶC ĐỊNH vì không đọc được của xã — nói ra, một lần, đầu màn.
 * `role="status"` chứ không `alert`: sổ vẫn dùng được, chỉ chữ có thể khác chữ xã đặt. Xuất ra để
 * bài kiểm kết xuất được: `SoNhiemVu` đọc mạng trong `useEffect`, thứ `renderToStaticMarkup` không chạy.
 */
export function CanhBaoNhanTrangThai({ canhBao }: { canhBao: string | null }) {
  if (canhBao === null) return null;
  return (
    <p className="canh-bao-pham-vi" role="status">
      {canhBao}
    </p>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng không có — HIỆN LÊN ĐẦU MÀN, không giấu trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: danh sách dài hơn quyển sổ ở những ngày đầu, và một
 * bức tường chữ trên đầu màn hình là bức tường người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (A4): every part of the spec is built — `0 phần … chưa dựng được`
  // above a register would read as a broken screen. An empty list draws nothing.
  if (PHAN_CHUA_DUNG.length === 0) return null;
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý do
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

/** The scope group (spec 02 §2 item 1): value, label, and the hint the prototype puts in `title`. */
export const SCOPE_OPTIONS: readonly {
  readonly value: "" | "mine" | "related";
  readonly label: string;
  readonly hint: string;
}[] = [
  { value: "", label: PHAM_VI_TOAN_XA, hint: "Tất cả hồ sơ trong xã" },
  { value: "mine", label: PHAM_VI_CUA_TOI, hint: "Đích danh tôi là người xử lý" },
  {
    value: "related",
    label: SCOPE_RELATED_LABEL,
    hint: "Tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ",
  },
];

/**
 * Wait before a typed search is sent. The prototype filters on every keystroke (spec 02 §2 item 2);
 * this list is read from the server, so one request per key would be one request per letter — the
 * pause sends the word once the officer stops typing. Enter sends at once.
 */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Bộ lọc §3 — spec 02 §2, in the prototype's order: scope group → search → Bộ phận → Người thực hiện
 * → Mức ưu tiên → Loại (hidden in Sổ theo dõi) → Khối → Nguồn giao → Chỉ việc quá hạn · Sắp đến hạn →
 * (selection) → view switch. One wrapping row, no `Bộ lọc` panel.
 *
 * THREE SCOPES, as the spec (owner 07/10/2026, ADR 0076 lần 2 #4): `Tôi đã giao` is not on this page
 * (Sổ tay lãnh đạo keeps its own). `mine` and `related` carry NO identity: the server reads the
 * officer's code from the SESSION (rule 1, forbidden #2).
 *
 * Every select is named by `aria-label` (its first option names it on screen, as in the
 * prototype). Ô `Người thực hiện` lists the staff directory; the value is the BUSINESS code `CB-…`
 * — what `?assignee=` matches; names never reach the URL (rule 3, forbidden #4).
 */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  danhMuc,
  danhBa,
  disabled = false,
  hideType = false,
  end,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  danhMuc: DanhMucNhiemVu;
  /** Câu trả lời nguyên vẹn của danh bạ chọn người; `null` = chưa đọc xong. */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /**
   * Kept for the callers' shape. The row has NO status filter any more (the prototype has none);
   * a status arriving in the address bar is said under the row by the page.
   */
  nhanTT?: BangNhanTrangThai;
  /** Overview drill-down on: every filter is drawn but off — the rows must match the number. */
  disabled?: boolean;
  /** `Sổ theo dõi` hides `Loại`, as the prototype does — the book is the directive tasks' book. */
  hideType?: boolean;
  /** The right end of the row: the selection cluster and the view switch (the page's). */
  end?: ReactNode;
}) {
  const db = docDanhBaChonNguoi(danhBa);
  const sent = loc.tim ?? "";

  // The typed word is sent once typing pauses (`SEARCH_DEBOUNCE_MS`); the server caps it at 200
  // characters (`store.TimNhiemVuToiDa`), which the input enforces.
  useEffect(() => {
    const word = tim.trim();
    if (word === sent || disabled) return;
    const timer = window.setTimeout(() => datLoc({ ...loc, tim: word === "" ? undefined : word }), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [tim, sent, disabled, loc, datLoc]);

  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  // Overdue and due-soon EXCLUDE each other: a task cannot be both, and both on is an always-empty
  // list that reads as "nothing to do". Turning one on turns the other off. Off = ABSENT from the
  // query (the server only accepts the string `true`).
  function toggleLate(): void {
    const on = loc.chiTreHan !== true;
    datLoc({ ...loc, chiTreHan: on ? true : undefined, dueSoon: on ? undefined : loc.dueSoon });
  }
  function toggleDueSoon(): void {
    const on = loc.dueSoon !== true;
    datLoc({ ...loc, dueSoon: on ? true : undefined, chiTreHan: on ? undefined : loc.chiTreHan });
  }

  const scope = loc.phamVi === "mine" || loc.phamVi === "related" ? loc.phamVi : "";

  return (
    <div id="task-filters" className="flex min-w-0 flex-col gap-2 [&>*]:my-0">
      {/* `flex-row` written out: a legacy field rule in `globals.css` must never flip this row. */}
      <div className="flex min-w-0 flex-row flex-wrap items-center gap-2.5 [&>*]:my-0">
        {/* ONE joined group (spec 02 §2 item 1, prototype `ScopeFilter`). */}
        <div
          role="group"
          aria-label={SCOPE_GROUP_LABEL}
          className="border-line inline-flex overflow-hidden rounded-md border bg-white"
        >
          {SCOPE_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              title={o.hint}
              aria-pressed={scope === o.value}
              disabled={disabled}
              onClick={() => datLoc({ ...loc, phamVi: o.value === "" ? undefined : o.value })}
              className={cn(
                "border-line h-9 cursor-pointer border-0 border-r px-3 text-[12.5px] font-semibold [font-family:inherit] transition-colors last:border-r-0",
                "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-60",
                scope === o.value ? "bg-navy text-white" : "text-ink hover:bg-canvas bg-white",
              )}
            >
              {o.label}
            </button>
          ))}
        </div>

        <form className="relative m-0 w-64 max-w-full" onSubmit={timNgay} role="search">
          <label htmlFor="tim-nhiem-vu" className="an-thi-giac">
            Tìm theo tên nhiệm vụ
          </label>
          <Search
            aria-hidden="true"
            focusable="false"
            className="text-ink-muted pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
          />
          <input
            id="tim-nhiem-vu"
            name="tim-nhiem-vu"
            type="search"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            disabled={disabled}
            onChange={(e) => datTim(e.target.value)}
            autoComplete="off"
            maxLength={200}
            className={cn(INPUT_CLASS, "pl-9 text-[12.5px] md:text-[12.5px]")}
          />
        </form>

        <FilterSelect id="loc-bo-phan" label="Lọc theo bộ phận" value={loc.boPhanID ?? ""} disabled={disabled} onChange={(v) => datLoc({ ...loc, boPhanID: v || undefined })}>
          <option value="">{MOI_BO_PHAN_NHAN}</option>
          {danhMuc.boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </FilterSelect>

        <FilterSelect
          id="loc-nguoi-thuc-hien"
          label="Lọc theo người thực hiện"
          value={loc.nguoiThucHienMa ?? ""}
          disabled={disabled || db.dangTai}
          onChange={(v) => datLoc({ ...loc, nguoiThucHienMa: v || undefined })}
        >
          <option value="">{nhanTrongOChonCanBo(db, MOI_NGUOI_THUC_HIEN_NHAN)}</option>
          {db.ds.map((c) => (
            <option key={c.code} value={c.code}>
              {c.full_name}
            </option>
          ))}
        </FilterSelect>

        {/* KHÔNG SẮP XẾP LẠI MẢNG NÀY: thứ tự `items` LÀ thang bậc của xã (`muc_uu_tien_nhiem_vu.go`). */}
        <FilterSelect id="loc-uu-tien" label="Lọc theo mức ưu tiên" value={loc.mucUuTien ?? ""} disabled={disabled} onChange={(v) => datLoc({ ...loc, mucUuTien: v || undefined })}>
          <option value="">{MOI_MUC_UU_TIEN_NHAN}</option>
          {activeChoices(danhMuc.mucUuTien, loc.mucUuTien).map((m) => (
            <option key={m.code} value={m.code}>
              {m.label}
            </option>
          ))}
        </FilterSelect>

        {!hideType && (
          <FilterSelect id="loc-loai" label="Lọc theo loại nhiệm vụ" value={loc.loai ?? ""} disabled={disabled} onChange={(v) => datLoc({ ...loc, loai: v || undefined })} className={COMPACT_FILTER_TYPE}>
            <option value="">{MOI_LOAI_NHAN}</option>
            {activeChoices(danhMuc.loai, loc.loai).map((l) => (
              <option key={l.code} value={l.code}>
                {l.label}
              </option>
            ))}
          </FilterSelect>
        )}

        <FilterSelect id="loc-khoi" label="Lọc theo khối nhiệm vụ" value={loc.khoi ?? ""} disabled={disabled} onChange={(v) => datLoc({ ...loc, khoi: v || undefined })} className={COMPACT_FILTER_BLOC}>
          <option value="">{MOI_KHOI_NHAN}</option>
          {activeChoices(danhMuc.khoi, loc.khoi).map((k) => (
            <option key={k.code} value={k.code}>
              {k.label}
            </option>
          ))}
        </FilterSelect>

        <FilterSelect id="loc-nguon-giao" label="Lọc theo nguồn giao" value={loc.nguonGiao ?? ""} disabled={disabled} onChange={(v) => datLoc({ ...loc, nguonGiao: v || undefined })} className={COMPACT_FILTER_SOURCE}>
          <option value="">{MOI_NGUON_GIAO_NHAN}</option>
          {MOI_NGUON_GIAO.map((ma) => (
            <option key={ma} value={ma}>
              {nhanNguonGiao(ma)}
            </option>
          ))}
        </FilterSelect>

        {/* "Sắp đến hạn" carries no number of days: the threshold is the commune's own, read by the
            server from identity; a commune that set none gets the server's 409 sentence verbatim. */}
        <Button
          id="loc-qua-han"
          type="button"
          size="sm"
          variant={loc.chiTreHan === true ? "primary" : "outline"}
          aria-pressed={loc.chiTreHan === true}
          disabled={disabled}
          onClick={toggleLate}
        >
          {CHI_QUA_HAN_NHAN}
        </Button>
        <Button
          id="loc-sap-den-han"
          type="button"
          size="sm"
          variant={loc.dueSoon === true ? "primary" : "outline"}
          aria-pressed={loc.dueSoon === true}
          disabled={disabled}
          onClick={toggleDueSoon}
        >
          {DUE_SOON_FILTER_LABEL}
        </Button>

        {end}
      </div>

      {db.loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {cauLoiDanhBaLoc(db.loi)}
        </p>
      )}
    </div>
  );
}

/** The scope group's accessible name — prototype `ScopeFilter` (ADR 0082 #1). */
export const SCOPE_GROUP_LABEL = "Lọc nhanh theo người xử lý";

/**
 * A catalogue's ACTIVE rows, as the prototype lists them (`TaskWorkspace.tsx:223-255`,
 * `TaskAssignForm.tsx`): a retired row (ADR 0082 #2 — retired, never deleted) is not offered for a
 * new choice. The ONE exception is the value already chosen (`keep` — a shared URL, a form's
 * current value): dropping it would make the select silently show another option than the one the
 * query or the form holds.
 */
export function activeChoices<T extends { readonly code: string; readonly active: boolean }>(
  rows: readonly T[],
  keep?: string,
): readonly T[] {
  return rows.filter((r) => r.active || (keep !== undefined && keep !== "" && r.code === keep));
}

/**
 * FIXED, NARROW widths for `Mọi loại nhiệm vụ` · `Mọi khối` · `Mọi nguồn giao` (customer sheet row 11).
 * Without them every select took the global 12rem floor, or the width of its longest option, so the
 * row sat at the edge of wrapping: ticking a card (the `Đã chọn N nhiệm vụ · Xoá đã chọn` cluster) or
 * choosing a longer value pushed the view switch onto a second line and the row jumped. Each width fits
 * its first option at 12.5px plus the chevron's `pr-9`; a longer chosen value ends in "…" (row 2).
 */
const COMPACT_FILTER_TYPE = "w-40 min-w-0";
const COMPACT_FILTER_BLOC = "w-28 min-w-0";
const COMPACT_FILTER_SOURCE = "w-36 min-w-0";

/** One native select of the filter row (spec 02 §2), its label visually hidden. */
function FilterSelect({
  id,
  label,
  value,
  disabled,
  onChange,
  className,
  children,
}: {
  id: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
  /** A fixed width (`COMPACT_FILTER_*`) — see there. */
  className?: string;
  children: ReactNode;
}) {
  // `aria-label` on the select, NO sibling `<label>` (prototype `TaskWorkspace.tsx:191-195`): a
  // `label + select` pair as direct children of the row matched the legacy `globals.css` field rule
  // (`div:has(> label + select)` → a column with full-width selects) and stacked the whole row.
  return (
    <select
      id={id}
      aria-label={label}
      value={value}
      disabled={disabled}
      className={cn(FILTER_SELECT_CLASS, className)}
      onChange={(e) => onChange(e.target.value)}
    >
      {children}
    </select>
  );
}


/** Which gesture started a Kanban move — only focus handling differs, never the request. */
export type KanbanMoveVia = "menu" | "drag";

/** The server's answer to one Kanban move. A refusal keeps the server's sentence VERBATIM. */
export type KanbanMoveResult =
  | {
      readonly code: string;
      readonly target: TrangThaiNhiemVu;
      readonly via: KanbanMoveVia;
      readonly ok: true;
    }
  | {
      readonly code: string;
      readonly target: TrangThaiNhiemVu;
      readonly via: KanbanMoveVia;
      readonly ok: false;
      readonly message: string;
    };

/**
 * Everything the board needs to move cards. `null` (the default) = a READ-ONLY board: no drag,
 * no menu. Closed by default, so a caller that forgets the prop gets fewer controls, never more.
 */
export type KanbanMove = {
  /** The session's `task.*` keys. Hiding is convenience — the route checks again (rule 5). */
  readonly permissions: QuyenNhiemVu;
  /**
   * `phien.staff.code`, `""` while the session is unread. The task's ASSIGNEE may move it without
   * `task.update` (ea55113) — `canMoveTask`. REQUIRED: a forgotten code would silently hide every
   * assignee's own cards.
   */
  readonly staffCode: string;
  /** A move waiting for the server. One at a time: every control is disabled meanwhile. */
  readonly pending: { readonly code: string; readonly target: TrangThaiNhiemVu } | null;
  readonly result: KanbanMoveResult | null;
  readonly move: (task: petitions_nhiemVuRa, target: TrangThaiNhiemVu, via: KanbanMoveVia) => void;
};

/**
 * The ONE call both Kanban paths make — the drawer's route, `POST /api/v1/tasks/{ma}/status`,
 * through the same `doiTrangThaiNhiemVu`. No note: the optional note lives in the drawer.
 */
export function moveTaskStatus(
  code: string,
  target: TrangThaiNhiemVu,
  via: KanbanMoveVia,
): Promise<KanbanMoveResult> {
  return doiTrangThaiNhiemVu(code, target).then((kq) =>
    kq.ok
      ? { code, target, via, ok: true as const }
      : { code, target, via, ok: false as const, message: kq.thongBao },
  );
}

/** Một cột của bảng Kanban: mã trạng thái, và pha đọc của riêng nó. */
export type CotKanban = {
  readonly ma: TrangThaiNhiemVu;
  readonly tai: TrangThaiTai<page_Result_petitions_nhiemVuRa>;
};

/**
 * Bảng Kanban §4.1 / spec 03 — năm cột ứng với năm trạng thái CHÍNH, in the commune's ORDER, under
 * the spec's FIXED words (`withSpecLabels`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KÉO-THẢ CÓ, VÀ CÓ MỘT LỐI BÀN PHÍM NGANG HÀNG (28/09/2026, kept by the owner 07/10/2026 #5).
 *
 * Kéo-thả chạy bằng @dnd-kit (chuột, cảm ứng, bàn phím) — kéo-thả HTML5 không bắt đầu được từ nút mở
 * thẻ và không chạy trên màn cảm ứng; lý do đầy đủ ở `kanban-drag.ts`. Cán bộ cầm chuột không vững
 * vẫn cần một lối không phải kéo (`skills/accessibility-elderly`). Nên mỗi thẻ có thêm nút
 * `Chuyển sang cột…` (`KanbanMoveMenu`), liệt kê ĐÚNG các bước drawer liệt kê (`clickableTransitions`).
 * Hai lối gọi CÙNG một hàm (`move.move` → `moveTaskStatus`), tức cùng tuyến
 * `POST /api/v1/tasks/{ma}/status` của drawer; the outcome is a TOAST (spec 03) — the server's
 * sentence verbatim on a refusal, the list of unfinished child tasks included.
 *
 * KHÔNG DI CHUYỂN LẠC QUAN: thẻ ở nguyên cột, mờ đi, tới khi máy chủ trả lời. A drop on a column the
 * lifecycle refuses sends nothing and says, by toast, where the card CAN go (spec 03).
 *
 * Không có `move` (mặc định `null`) thì bảng CHỈ ĐỌC: không kéo, không nút.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * THE PROTOTYPE'S BOARD (spec 03 `Lưới cột`): five equal columns from 1280px, below that one row of
 * fixed 232px columns that scrolls sideways — at every narrower width, a phone included. The legacy
 * `.bang-kanban` (stacked on phones) is no longer used here.
 *
 * Each card has `☐ Chọn` for `🗑 Xoá đã chọn` when the session holds `task.delete` — `selection`,
 * default `null` = no checkbox.
 */
/** Spec 03: the toast of a drop on a column the lifecycle does not reach from this card. */
export function kanbanDropRefusedToast(
  labels: BangNhanTrangThai,
  from: string,
  allowed: readonly string[],
): string {
  return `Từ “${nhanTrangThai(labels, from)}” chỉ chuyển được sang: ${allowed
    .map((s) => nhanTrangThai(labels, s))
    .join(", ")}.`;
}

export function BangKanban({
  cot,
  danhMuc,
  danhBa = null,
  nhanTT,
  bayGio,
  maDangMo,
  moNhiemVu,
  counts,
  move = null,
  selection = null,
}: {
  cot: readonly CotKanban[];
  danhMuc: DanhMucNhiemVu;
  /** Danh bạ tra theo mã — họ tên người thực hiện trên thẻ. `null` = chưa có, thẻ hiện mã. */
  danhBa?: DanhBaTheoMa | null;
  /** Kept for the callers' shape: the card names the assignee only (spec 03). */
  unitNames?: ReadonlyMap<string, string>;
  /**
   * `GET /api/v1/task-counts` under the board's filters — the header numbers (#15). REQUIRED: a
   * caller that forgets it gets a red `tsc`, not headers that silently fall back to card counts.
   */
  counts: TrangThaiTai<petitions_taskCountsOut>;
  /** Moving cards — see `KanbanMove`. `null` = read-only board. */
  move?: KanbanMove | null;
  /** `☐ Chọn` on every card — see `TaskSelection`. `null` = no checkbox. */
  selection?: TaskSelection | null;
  /**
   * Nhãn và thứ tự: tên cột VÀ thứ tự cột. §4.1 cố định năm cột chính; thứ tự năm cột ấy theo
   * `order` xã đặt (`theoThuTuXa`). Xếp Ở ĐÂY chứ không ở chỗ gọi, để thứ tự vẽ ra kiểm được bằng
   * một lần kết xuất.
   */
  nhanTT: BangNhanTrangThai;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
}) {
  // The card being dragged: every column must know DURING the drag whether the lifecycle takes it.
  const [dragging, setDragging] = useState<petitions_nhiemVuRa | null>(null);
  const statusRef = useRef<HTMLParagraphElement>(null);
  const result = move?.result ?? null;
  const sensors = useSensors(
    // Mouse, not Pointer: a pointer sensor also fires for a finger and would race the touch
    // sensor for the same gesture. 6px before a press becomes a drag, so a plain click on the
    // card still opens the task (the prototype's constraint).
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    // A short press-and-hold on touch: a finger that moves at once is scrolling the board.
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: columnKeyboardCoordinates }),
  );

  useEffect(() => {
    // A menu move that succeeded reloads the board, and the button that had focus is gone with
    // the card. Put focus on the (visually hidden) sentence saying where the card went, not on `<body>`.
    if (result !== null && result.ok && result.via === "menu") statusRef.current?.focus();
  }, [result]);

  // Bộ lọc Trạng thái đang chọn một trạng thái rẽ nhánh (hoặc một mã lạ): KHÔNG cột nào khớp. Vẽ
  // năm cột rỗng ở đây là nói với cán bộ rằng xã không có việc nào — đúng điều ngược lại.
  if (cot.length === 0) {
    return <p className="trang-thai-rong">{CAU_LOC_TRANG_THAI_KHONG_CO_COT}</p>;
  }

  const viTri = theoThuTuXa(
    cot.map((c) => c.ma),
    nhanTT,
  );
  const cotSap = [...cot].sort((a, b) => viTri.indexOf(a.ma) - viTri.indexOf(b.ma));

  const totalOf = (status: TrangThaiNhiemVu): number | null =>
    counts.pha === "xong" ? kanbanColumnCount(counts.duLieu.by_status, status) : null;

  // Every column refused for ONE reason (e.g. 409 under `Sắp đến hạn`): say it ONCE, above the board.
  const sharedError = kanbanSharedError(cot);

  // `dragTargets` = `clickableTransitions` (the drawer's list), empty while a move is pending.
  const allowedTargets = dragging === null ? [] : dragTargets(dragging, move);
  const dropState = (target: TrangThaiNhiemVu): KanbanDropState =>
    dragging === null ? "idle" : allowedTargets.includes(target) ? "allowed" : "refused";

  const statusText =
    move === null
      ? ""
      : move.pending !== null
        ? kanbanMovePendingText(nhanTT, move.pending.code, move.pending.target)
        : result !== null && result.ok
          ? kanbanMoveDoneText(nhanTT, result.code, result.target)
          : "";

  return (
    <>
      {/* Where focus lands after a menu move (the card it came from is re-rendered). Visually hidden:
          the outcome is a toast; the toast region is what announces it. */}
      {move !== null && (
        <p ref={statusRef} tabIndex={-1} className="an-thi-giac">
          {statusText}
        </p>
      )}
      {sharedError !== null && (
        <p className="thong-bao-loi" role="alert">
          {sharedError}
        </p>
      )}
      <DndContext
        // A fixed id keeps dnd-kit's `aria-describedby` the same on the server and in the browser.
        id="bang-kanban-nhiem-vu"
        sensors={sensors}
        collisionDetection={kanbanCollision}
        accessibility={{
          announcements: kanbanAnnouncements(nhanTT, move),
          screenReaderInstructions: KANBAN_DRAG_INSTRUCTIONS,
        }}
        onDragStart={(e) => setDragging(draggedTask(e.active.data.current))}
        onDragCancel={() => setDragging(null)}
        onDragEnd={(e) => {
          const task = draggedTask(e.active.data.current);
          setDragging(null);
          // The SAME call as the menu: `move.move(task, target, "drag")`, once, and only for a column
          // `dragTargets` lists. A drop on another column sends nothing and says where the card CAN go.
          const sent = dropOnKanban(task, e.over?.id ?? null, move);
          const over = e.over?.id;
          if (!sent && task !== null && over !== undefined && over !== null && String(over) !== task.status) {
            const allowed = dragTargets(task, move);
            if (allowed.length > 0) toast.error(kanbanDropRefusedToast(nhanTT, task.status, allowed));
          }
        }}
      >
        {/* A bare scroller (`role="region"`, focusable, so the keyboard can scroll it sideways). */}
        <div className="max-w-full overflow-x-auto" role="region" aria-label="Bảng Kanban nhiệm vụ" tabIndex={0}>
          <div className="grid auto-cols-[232px] grid-flow-col gap-3.5 xl:auto-cols-auto xl:grid-flow-row xl:grid-cols-5">
            {cotSap.map((c) => (
              <KanbanColumn key={c.ma} status={c.ma} drop={dropState(c.ma)} droppable={move !== null}>
                <h3
                  id={`cot-kanban-${c.ma}`}
                  className="text-navy m-0 mb-3 flex items-center gap-2 px-1 text-[12.5px] font-bold"
                >
                  <span className={cn("size-2 shrink-0 rounded-full", statusDotClass(c.ma))} aria-hidden="true" />
                  {nhanTrangThai(nhanTT, c.ma)}{" "}
                  {/* THE REAL TOTAL (#15), not the cards loaded. Unreadable or missing → `—`, never
                      a `0` that reads as "nothing here". Still loading → no chip yet. */}
                  {counts.pha !== "dangTai" && (
                    <span className="kanban-count border-line text-ink-muted ml-auto rounded-[10px] border bg-white px-2 text-[11px] font-semibold">
                      {totalOf(c.ma) === null ? O_TRONG : String(totalOf(c.ma))}
                    </span>
                  )}
                </h3>
                {/* Words, not only the column colour, say where the card may go (a11y §8) — kept by the
                    owner 07/10/2026 #5, drawn at the spec's small size. */}
                {dropState(c.ma) === "allowed" && (
                  <p className="text-ink-muted m-0 mb-2 px-1 text-[11px]">{kanbanDropHint(nhanTT, c.ma)}</p>
                )}
                {dropState(c.ma) === "refused" && (
                  <p className="text-ink-muted m-0 mb-2 px-1 text-[11px]">{KANBAN_DROP_REFUSED_HINT}</p>
                )}

                {/* NĂM CỘT LÀ NĂM CÂU TRẢ LỜI RỜI NHAU. Một cột hỏng thì bốn cột kia vẫn là sổ —
                    và câu hỏng của nó hiện nguyên văn ở đúng cột ấy, không nuốt thành một lỗi chung. */}
                {c.tai.pha === "dangTai" && (
                  <p role="status" className="text-ink-muted m-0 px-1 py-6 text-center text-[11.5px]">
                    {DANG_TAI_SO}
                  </p>
                )}
                {c.tai.pha === "loi" && sharedError === null && (
                  <p className="thong-bao-loi" role="alert">
                    {c.tai.thongBao}
                  </p>
                )}
                {c.tai.pha === "xong" && c.tai.duLieu.items.length === 0 && (
                  <p className="text-ink-muted m-0 px-1 py-6 text-center text-[11.5px]">{COT_RONG}</p>
                )}
                {c.tai.pha === "xong" && c.tai.duLieu.items.length > 0 && (
                  // `minmax(0, 1fr)` + `min-w-0`: a card never grows past its column (a one-line
                  // `truncate` holder line is otherwise its min-content width).
                  <ul className="m-0 grid list-none grid-cols-[minmax(0,1fr)] gap-2.5 p-0">
                    {c.tai.duLieu.items.map((n) => (
                      <li key={n.code} className="min-w-0">
                        <TheNhiemVu
                          nhiemVu={n}
                          danhMuc={danhMuc}
                          danhBa={danhBa}
                          nhanTT={nhanTT}
                          bayGio={bayGio}
                          maDangMo={maDangMo}
                          moNhiemVu={moNhiemVu}
                          move={move}
                          selection={selection}
                        />
                      </li>
                    ))}
                  </ul>
                )}
                {/* One page per column (server paging, kept #5): a header of 157 above 100 cards
                    must say so. */}
                {c.tai.pha === "xong" &&
                  (c.tai.duLieu.has_more || (totalOf(c.ma) ?? 0) > c.tai.duLieu.items.length) && (
                    <p className="text-ink-muted m-0 mt-2 px-1 text-[11px]">
                      {kanbanPartialNote(c.tai.duLieu.items.length, totalOf(c.ma))}
                    </p>
                  )}
              </KanbanColumn>
            ))}
          </div>
        </div>
        {/* The copy under the pointer while the real card stays, dimmed, in its column — it moves
            only once the server answers. Outside the scrolling region: a fixed overlay inside an
            overflow container would be clipped. No drop animation: gliding back would read as
            "refused" while the request is still on its way. */}
        <DragOverlay dropAnimation={null}>
          {dragging !== null && (
            <TheNhiemVu
              overlay
              nhiemVu={dragging}
              danhMuc={danhMuc}
              danhBa={danhBa}
              nhanTT={nhanTT}
              bayGio={bayGio}
              maDangMo={maDangMo}
              moNhiemVu={moNhiemVu}
            />
          )}
        </DragOverlay>
      </DndContext>

      {counts.pha === "loi" && counts.thongBao !== sharedError && (
        <p className="thong-bao-loi" role="alert">
          {KANBAN_COUNTS_ERROR} {counts.thongBao}
        </p>
      )}
    </>
  );
}

/** A column during a drag: no drag (`idle`), takes the dragged card, or will not take it. */
type KanbanDropState = "idle" | "allowed" | "refused";

/**
 * One Kanban column as a @dnd-kit drop zone (spec 03 `Cột`). EVERY column is a drop zone while moves
 * are possible, refused ones included: a refused column must catch the drop (and do nothing but say
 * why) rather than let a neighbouring allowed column catch it. The column under the card takes the
 * spec's `isOver` look only when it accepts it.
 */
function KanbanColumn({
  status,
  drop,
  droppable,
  children,
}: {
  status: TrangThaiNhiemVu;
  drop: KanbanDropState;
  /** `false` on a read-only board: nothing to drop. */
  droppable: boolean;
  children: ReactNode;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: status, disabled: !droppable });
  return (
    <section
      ref={setNodeRef}
      className={cn(
        "border-line bg-canvas min-h-45 min-w-0 rounded-[12px] border p-3 transition-colors",
        drop === "allowed" && isOver && "border-brand/40 bg-brand/5",
      )}
      data-drop={drop === "idle" ? undefined : drop}
      aria-labelledby={`cot-kanban-${status}`}
    >
      {children}
    </section>
  );
}

/** A directive document line of the register (spec 05 `RefList`). */
function RegisterRefList({ docs }: { docs: readonly petitions_nhiemVuVanBanRa[] }) {
  if (docs.length === 0) return <span className="text-ink-muted">{O_TRONG}</span>;
  return (
    <ul className="m-0 list-none space-y-1.5 p-0">
      {docs.map((v) => (
        <li key={v.id}>
          <span className="text-navy font-semibold">{v.reference === "" ? KHONG_SO : v.reference}</span>
          {v.date !== "" && <span className="text-ink-muted"> · {ngayVanBan(v.date)}</span>}
          {v.summary !== "" && <div className="text-ink-muted line-clamp-2 text-[11.5px]">{v.summary}</div>}
        </li>
      ))}
    </ul>
  );
}

/** Spec 05 header cell class (`ne`) and prose cell class (`t9`). */
const REGISTER_TH = "whitespace-normal align-bottom leading-tight";
const REGISTER_TD = "whitespace-normal break-words align-top";

/**
 * Sổ theo dõi — spec 05: the commune office's book of directive documents, THIRTEEN columns
 * (`REGISTER_COLUMNS`, after the `☐` column only a `task.delete` holder sees), `min-w-[1700px]
 * table-fixed`, scrolling sideways.
 *
 * "Cơ quan chủ trì tham mưu" and "Chuyên viên VP tham mưu / theo dõi" print the SAME data as
 * "Đơn vị thực hiện" and its assignee line (owner 07/10/2026 #8, ADR 0065 NV5 — one role, one field).
 * Documents come from the row itself (`include=documents`); a row without the block says so instead
 * of drawing `—` in three columns.
 *
 * Read-only: the whole row opens the drawer, as the list does.
 */
export function BangSoTheoDoi({
  nhiemVu,
  danhMuc,
  danhBa = null,
  nhanTT,
  tenBoPhan,
  bayGio,
  maDangMo,
  moNhiemVu,
  selection = null,
  empty,
}: {
  nhiemVu: readonly petitions_nhiemVuRa[];
  danhMuc: DanhMucNhiemVu;
  danhBa?: DanhBaTheoMa | null;
  nhanTT: BangNhanTrangThai;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
  selection?: TaskSelection | null;
  /** What an empty page shows — see `BangNhiemVu`. */
  empty?: ReactNode;
}) {
  if (nhiemVu.length === 0) return empty ?? <EmptyState icon={ClipboardList} title={SO_RONG} />;
  const unit = (id: string) => (id === "" ? O_TRONG : (tenBoPhan.get(id) ?? id));

  return (
    <div className={TABLE_FRAME_CLASS}>
      <div className="overflow-x-auto" role="region" aria-label="Sổ theo dõi nhiệm vụ" tabIndex={0}>
        <table className={cn(TABLE_CLASS, "min-w-[1700px] table-fixed")}>
          <thead>
            <tr className="border-line border-b">
              {selection !== null && (
                <th scope="col" className={cn(TH_CLASS, "w-10")}>
                  <SelectAllBox selection={selection} rows={nhiemVu} />
                </th>
              )}
              {REGISTER_COLUMNS.map((c) => (
                <th key={c.label} scope="col" className={cn(TH_CLASS, REGISTER_TH, c.width)}>
                  {c.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {nhiemVu.map((n, i) => {
              const late = isOverdueNow(n, bayGio);
              const docs = registerRowDocuments(n);
              const state = taskDisplayState(n, nhanTT, bayGio);
              const assignee = nhanCanBoNgan(n.assignee, danhBa, CHUA_PHAN_CONG);
              return (
                <tr
                  key={n.code}
                  data-tre-han={late ? "" : undefined}
                  className={cn(rowClass(late), "align-top")}
                  onClick={() => moNhiemVu(n)}
                >
                  {selection !== null && (
                    <td className={TD_CLASS} onClick={(e) => e.stopPropagation()}>
                      <RowCheckbox selection={selection} task={n} />
                    </td>
                  )}
                  <td className={cn(TD_CLASS, REGISTER_TD)}>
                    {/* A real button too: the keyboard opens the row from its code. */}
                    <button
                      type="button"
                      className="text-ink-muted cursor-pointer border-0 bg-transparent p-0 text-left text-[11.5px] font-semibold [font-family:inherit] hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
                      aria-expanded={n.code === maDangMo}
                      onClick={(e) => {
                        e.stopPropagation();
                        moNhiemVu(n);
                      }}
                    >
                      {n.code === "" ? `#${i + 1}` : n.code}
                    </button>
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>
                    <div className="text-navy font-semibold">{n.title}</div>
                    {n.description !== "" && (
                      <div className="text-ink-muted mt-0.5 line-clamp-3 text-[11.5px]">{n.description}</div>
                    )}
                    {n.bloc !== "" && (
                      <div className="text-ink-muted mt-1 text-[11px]">{nhanDanhMuc(danhMuc.khoi, n.bloc)}</div>
                    )}
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>{unit(n.unit)}</td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>{assignee}</td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>
                    {unit(n.unit)}
                    <div className="text-ink-muted text-[11.5px]">{assignee}</div>
                  </td>
                  {(["cap-tren-giao", "chi-dao-dang-uy"] as const).map((g, gi) => (
                    <td key={g} className={cn(TD_CLASS, REGISTER_TD)}>
                      {docs === null ? (
                        // Once per row, in the first document column — not three times.
                        gi === 0 ? <span className="thong-bao-loi">{REGISTER_DOCS_MISSING}</span> : O_TRONG
                      ) : (
                        <RegisterRefList docs={docs.byGroup[g]} />
                      )}
                    </td>
                  ))}
                  <td className={cn(TD_CLASS, REGISTER_TD, late && "text-danger font-semibold")}>
                    {nhanNgay(n.due_at)}
                    {late && <div className="text-[11px]">{lateText(daysOverdue(n, bayGio))}</div>}
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>
                    <SpecStatusBadge
                      state={state}
                      status={n.status}
                      className="h-auto min-h-5 max-w-full whitespace-normal"
                    />
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD)}>
                    {n.result_summary !== "" && <div className="mb-1">{n.result_summary}</div>}
                    {docs !== null && <RegisterRefList docs={docs.byGroup["san-pham-dau-ra"]} />}
                    {docs !== null && docs.unknown > 0 && (
                      <div className="text-ink-muted text-[11px]">{REGISTER_UNKNOWN_GROUP}</div>
                    )}
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD, "text-center")}>
                    {n.leader_approved ? (
                      <Check aria-hidden="true" focusable="false" className="text-leaf mx-auto block size-4" />
                    ) : (
                      <Minus aria-hidden="true" focusable="false" className="text-ink-muted mx-auto block size-4" />
                    )}
                    <span className="an-thi-giac">{n.leader_approved ? APPROVAL_TICKED : APPROVAL_UNTICKED}</span>
                    {n.leader_approved && !n.superior_acknowledged && (
                      <div className="text-tangerine mt-1 text-[10.5px] leading-tight">{SUPERIOR_NOT_YET}</div>
                    )}
                  </td>
                  <td className={cn(TD_CLASS, REGISTER_TD, "text-ink-muted text-[11.5px]")}>
                    {n.note === "" ? O_TRONG : n.note}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** A row's / card's tick — native, the spec's `accent-brand` checkbox. */
function RowCheckbox({ selection, task }: { selection: TaskSelection; task: petitions_nhiemVuRa }) {
  return (
    <input
      type="checkbox"
      className={CHECKBOX_CLASS}
      aria-label={selectLabel(task.code)}
      checked={selection.selected.has(task.code)}
      disabled={selection.disabled}
      onChange={() => selection.toggle(task)}
    />
  );
}

/**
 * The Kanban card's tick in the prototype's look (`TaskCard.tsx:54`, shadcn `Checkbox`): a 16px box,
 * 4px radius, `input` hairline; ticked = navy fill with a white check (customer sheet row 10).
 *
 * STILL THE NATIVE `<input type="checkbox">` — only its drawing is replaced (`appearance-none`, the
 * check icon over it): Space, the label click, `checked`, the accessible name and every test that
 * finds the box by role are unchanged. Radix's Checkbox is not a dependency of this app.
 */
function CardCheckbox({
  label,
  checked,
  disabled,
  onToggle,
}: {
  label: string;
  checked: boolean;
  disabled: boolean;
  onToggle: () => void;
}) {
  return (
    <span className="relative inline-grid size-4 shrink-0 place-items-center">
      <input
        type="checkbox"
        className={cn(
          "peer m-0 size-4 cursor-pointer appearance-none rounded-[4px] border border-solid border-input bg-white transition-colors",
          "checked:border-primary checked:bg-primary",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          "disabled:cursor-not-allowed disabled:opacity-50",
        )}
        aria-label={label}
        checked={checked}
        disabled={disabled}
        onChange={onToggle}
      />
      <Check
        aria-hidden="true"
        focusable="false"
        strokeWidth={2.5}
        className="text-primary-foreground pointer-events-none absolute size-3.5 opacity-0 peer-checked:opacity-100"
      />
    </span>
  );
}

/** The header box of a table (prototype `SelectAllBox`): ticks or unticks every row shown. */
function SelectAllBox({
  selection,
  rows,
}: {
  selection: TaskSelection;
  rows: readonly petitions_nhiemVuRa[];
}) {
  const state = selectAllState(selection.selected, rows);
  if (selection.toggleAll === undefined) return <span className="an-thi-giac">Chọn</span>;
  const toggleAll = selection.toggleAll;
  return (
    <input
      type="checkbox"
      className={CHECKBOX_CLASS}
      aria-label={SELECT_ALL_LABEL}
      checked={state === "all"}
      ref={(el) => {
        if (el !== null) el.indeterminate = state === "some";
      }}
      disabled={selection.disabled}
      onChange={() => toggleAll(rows)}
    />
  );
}

/**
 * The status badge of a row (spec 04/05 `taskDisplayState`): finished late, then overdue, override
 * the status — both derived from dates (rule 10, invariant 3). WORD ONLY, no icon, as the prototype
 * (`TaskListTable.tsx`) — ADR 0082 #7 replaces ADR 0068 lần 6 #7 for this menu only. The word carries
 * the meaning; the tone is the second signal.
 */
export function SpecStatusBadge({
  state,
  status,
  className,
}: {
  state: ReturnType<typeof taskDisplayState>;
  status: string;
  /** Register cell only: lets the badge wrap inside its fixed w-28 column (review r2 D-1). */
  className?: string;
}) {
  return (
    <span className={cn(BADGE_CLASS, state.chip, className)} data-status={status} data-tone={state.tone}>
      {state.label}
    </span>
  );
}

/** The deadline line of a card (spec 03): finished late, overdue, or `Hạn {d}`. */
export function cardDueText(task: petitions_nhiemVuRa, labels: BangNhanTrangThai, now: Date): string {
  if (hoanThanhTreHan(task.completed_at, task.original_due_at)) return nhanHoanThanhTreHan(labels);
  if (isOverdueNow(task, now)) return nhanHanThe(task.due_at, now);
  return `Hạn ${nhanNgay(task.due_at)}`;
}

/**
 * Một thẻ nhiệm vụ — spec 03 `Thẻ nhiệm vụ`: a priority strip, `☐ Chọn` (with `task.delete`), then ONE
 * button holding code · title · `N việc con` · deadline · assignee.
 *
 * THE STRIP IS COLOUR, so the priority is also said in words for a screen reader (colour is never the
 * only signal). Its colour: the commune's catalogue colour, else by code (`priorityStrip`).
 *
 * The move button (`Chuyển sang cột…`, kept #5) sits at the card's top-right corner; the open button
 * leaves room for it so the title never runs under it.
 */
export function TheNhiemVu({
  nhiemVu,
  danhMuc,
  danhBa = null,
  nhanTT,
  bayGio,
  maDangMo,
  moNhiemVu,
  move = null,
  selection = null,
  overlay = false,
}: {
  nhiemVu: petitions_nhiemVuRa;
  danhMuc: DanhMucNhiemVu;
  /** Xem `BangKanban`. Mã không có trong danh bạ thì hiện MÃ, không bao giờ để trống. */
  danhBa?: DanhBaTheoMa | null;
  /** Kept for the callers' shape (the card names the assignee only, spec 03). */
  unitNames?: ReadonlyMap<string, string>;
  nhanTT: BangNhanTrangThai;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
  /** See `BangKanban`. `null` = no drag, no menu. */
  move?: KanbanMove | null;
  /** `☐ Chọn` — see `TaskSelection`. `null` = no checkbox. */
  selection?: TaskSelection | null;
  /**
   * The copy `DragOverlay` draws under the pointer: hidden from screen readers (the real card is
   * still on the board), never a second drag source. Registered under its own id — the real card's
   * id would overwrite the node dnd-kit is measuring.
   */
  overlay?: boolean;
}) {
  // Same list as the drawer's buttons. Empty (neither `task.update` nor the assignee, or nothing the
  // server lists) → neither a drag handle nor a menu: a control that can only be refused is not offered.
  const targets =
    move === null ? [] : clickableTransitions(nhiemVu, move.permissions, move.staffCode);
  const busy = move !== null && move.pending !== null;
  const canDrag = !overlay && targets.length > 0 && !busy;
  const drag = useDraggable({
    id: overlay ? `overlay:${nhiemVu.code}` : nhiemVu.code,
    data: { task: nhiemVu },
    disabled: !canDrag,
  });
  const { setNodeRef, setActivatorNodeRef } = drag;
  // Stable, so a re-render during the drag does not detach and re-attach the node dnd-kit measures.
  const setCardRef = useCallback(
    (node: HTMLElement | null) => {
      setNodeRef(node);
      // The keyboard drag starts only when the CARD itself has focus: Enter on its open button,
      // Space on its checkbox or its menu button keep their own meaning.
      setActivatorNodeRef(node);
    },
    [setNodeRef, setActivatorNodeRef],
  );
  // The overlay copy carries no id: the real card is still on the page and ids must stay unique.
  const titleId = `the-nhiem-vu-${nhiemVu.code}`;
  const pendingHere = move?.pending?.code === nhiemVu.code;
  const canReturn =
    move !== null && reasonMove(nhiemVu, move.permissions, move.staffCode) === "return";
  const hasMenu = move !== null && targets.length > 0 && !overlay;

  const priorityLabel = nhiemVu.priority === "" ? "" : nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority);
  const strip = priorityStrip(danhMuc.mucUuTien, nhiemVu.priority);
  const completedLate = hoanThanhTreHan(nhiemVu.completed_at, nhiemVu.original_due_at);
  const late = !completedLate && isOverdueNow(nhiemVu, bayGio);
  const children = childCountLabel(nhiemVu.child_count);
  const extensions = extensionCountText(nhiemVu.extension_count);

  return (
    <article
      // The whole card is the drag source; its open button stays a plain click because a drag
      // starts only after 6px of movement (mouse) or a held press (touch).
      ref={setCardRef}
      className={cn(
        "border-line shadow-card relative rounded-[10px] border bg-white transition",
        canDrag && "the-keo-duoc",
        (drag.isDragging || pendingHere) && "opacity-50",
        overlay && "the-dang-bay",
      )}
      data-task-card=""
      data-dragging={drag.isDragging ? "" : undefined}
      aria-labelledby={overlay ? undefined : titleId}
      aria-hidden={overlay ? true : undefined}
      aria-busy={pendingHere ? true : undefined}
      // Not `{...drag.attributes}`: its `role="button"` would turn an article holding three
      // controls into one button. The rest — focusable, described by the drag instructions — is kept.
      tabIndex={canDrag ? 0 : undefined}
      aria-roledescription={canDrag ? KANBAN_CARD_ROLE : undefined}
      aria-describedby={canDrag ? drag.attributes["aria-describedby"] : undefined}
      {...(canDrag ? drag.listeners : {})}
    >
      {/* THE CARD HEADER — strip, `Chọn`, the move button, code and title — is ONE hover area
          (customer sheet row 12): the fill used to cover only the thin `Chọn` row, so pointing at the
          title lit a band above it and nothing under it. The menu sits inside, so pointing at it keeps
          the fill. `bg-canvas` = the prototype's `hover:bg-surface` (token trap: our `surface` is white). */}
      <div className={cn("relative rounded-t-[9px] transition-colors", !overlay && "hover:bg-canvas")}>
        <div
          className={cn("h-[3px] rounded-t-[9px]", strip.className)}
          style={strip.style}
          aria-hidden="true"
        />
        {selection !== null && !overlay && (
          // OUTSIDE the open button, never inside it: a checkbox in a button is a nested control.
          <label className="text-ink-muted m-0 flex cursor-pointer items-center gap-2 px-3 pt-2.5 text-[11px]">
            <CardCheckbox
              label={selectLabel(nhiemVu.code)}
              checked={selection.selected.has(nhiemVu.code)}
              disabled={selection.disabled}
              onToggle={() => selection.toggle(nhiemVu)}
            />
            Chọn
          </label>
        )}
        {/* The keyboard path to moving the card — drag-and-drop alone has none (a11y). An icon button
            at the corner, its menu floating over the card. */}
        {hasMenu && (
          <div className="absolute top-2 right-2 z-[1]">
            <KanbanMoveMenu
              compact
              code={nhiemVu.code}
              targets={targets}
              labels={nhanTT}
              disabled={busy}
              showReturnNote={canReturn}
              onMove={(t) => move.move(nhiemVu, t, "menu")}
            />
          </div>
        )}
        <button
          type="button"
          className={cn(
            "text-ink block w-full cursor-pointer border-0 bg-transparent px-3 pt-3 pb-0 text-left [font-family:inherit]",
            "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500",
            hasMenu && selection === null && "pr-11",
          )}
          onClick={() => moNhiemVu(nhiemVu)}
          aria-expanded={nhiemVu.code === maDangMo}
        >
          {nhiemVu.code !== "" && (
            <span className="text-ink-muted mb-0.5 block text-[10.5px] font-semibold">{nhiemVu.code}</span>
          )}
          {/* At most three lines, always THREE LINES TALL (`.tieu-de-the`): a title's length must not set
              the card's height, or the cards of two columns stop lining up row by row. The full title is
              the hover text, so the clamp hides nothing a mouse user cannot read. */}
          <span
            id={overlay ? undefined : titleId}
            className="tieu-de-the text-navy text-[12.8px] leading-snug font-semibold"
            title={nhiemVu.title}
          >
            {nhiemVu.title}
          </span>
          {priorityLabel !== "" && <span className="an-thi-giac"> · Mức ưu tiên {priorityLabel}</span>}
        </button>
      </div>
      {/* The rest of the card sits outside the open button, as it did when the meta row carried a
          "?" button. A pointer press anywhere here opens the task too; the keyboard path is the
          button above. */}
      <div className="cursor-pointer px-3 pb-3" onClick={() => moNhiemVu(nhiemVu)}>
        {/* Spec 03 / prototype `TaskCard.tsx:78-91`: `n việc con` · `đã gia hạn n lần`, each only when
            its figure is above zero. The card draws no pending-extension marker (prototype). */}
        {/* `min-h-4` keeps the row's height when both figures are zero, so a card without them is
            exactly as tall as one with them (row alignment across columns). */}
        <span className="text-ink-muted mt-2 flex min-h-4 flex-wrap items-center gap-2 text-[10.5px]">
          {children !== null && (
            <span className="flex items-center gap-1">
              <GitBranch aria-hidden="true" focusable="false" className="size-3" />
              {children}
            </span>
          )}
          {extensions !== null && (
            <span className="text-tangerine flex items-center gap-1 font-semibold">
              <TimerReset aria-hidden="true" focusable="false" className="size-3" />
              {extensions}
            </span>
          )}
        </span>
        {/* Late (red) ≠ finished late (amber): a finished task owes nothing, so red would be wrong,
            but it came in after its deadline, so a plain date would hide it. Both DERIVED from the
            dates and the clock (rule 10, invariant 3). */}
        <span className="mt-2.5 flex items-center justify-between text-[10.5px]">
          <span
            className={cn(
              "flex items-center gap-1",
              late && "text-danger font-semibold",
              completedLate && "text-tangerine font-semibold",
              !late && !completedLate && "text-ink-muted",
            )}
          >
            <Clock aria-hidden="true" focusable="false" className="size-3" />
            {cardDueText(nhiemVu, nhanTT, bayGio)}
          </span>
        </span>
        <span className="text-ink-muted mt-2 block truncate text-[11px]">
          {nhanCanBoNgan(nhiemVu.assignee, danhBa, CHUA_PHAN_CONG)}
        </span>
      </div>
    </article>
  );
}

/**
 * A sortable header of the list (spec 04): the label and `ArrowUpDown` (`size-3`, faint until this
 * column is the sort). `aria-sort` tells a screen reader the direction the SERVER was asked for —
 * sorting is the server's (the list is cursor-paged), never a client sort of one page.
 */
function SortHeader({
  cot,
  nhan,
  sapXep,
  doiSapXep,
}: {
  cot: CotSapXepNhiemVu;
  nhan: string;
  sapXep: SapXepSo;
  doiSapXep: (cot: CotSapXepNhiemVu) => void;
}) {
  const aria = ariaSapXep(sapXep, cot);
  return (
    <th scope="col" aria-sort={aria} className={TH_CLASS}>
      <button
        type="button"
        className="text-navy flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 font-medium [font-family:inherit] text-[length:inherit] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
        onClick={() => doiSapXep(cot)}
      >
        {nhan}
        <ArrowUpDown
          aria-hidden="true"
          focusable="false"
          className={cn("size-3", aria === "none" ? "opacity-40" : "opacity-100")}
        />
      </button>
    </th>
  );
}

/**
 * Bảng Danh sách — spec 04: Mã · Tên việc · Người thực hiện · Bộ phận · Ưu tiên · Hạn · Trạng thái,
 * after the `☐` column only a `task.delete` holder sees (with the header box). The whole row opens
 * the drawer; an overdue row takes the spec's pink tint, and the `Hạn` cell says `(trễ N ngày)` in
 * words — colour is never the only signal.
 *
 * SORTING IS THE SERVER'S (owner 07/10/2026 #5, ADR 0082 #5): Mã, Tên việc, Ưu tiên, Hạn and Trạng
 * thái carry a sort button; Người thực hiện and Bộ phận are plain headers (owner 08/10/2026: not
 * sortable, no "?") — a client sort of one page would order 100 rows of a longer register. Cells do not wrap (prototype `table.tsx:86`): the table scrolls
 * sideways.
 */
export function BangNhiemVu({
  nhiemVu,
  danhMuc,
  danhBa = null,
  nhanTT,
  tenBoPhan,
  bayGio,
  maDangMo,
  moNhiemVu,
  sapXep,
  doiSapXep,
  selection = null,
  empty,
}: {
  nhiemVu: readonly petitions_nhiemVuRa[];
  danhMuc: DanhMucNhiemVu;
  /** Danh bạ tra theo mã — cột `Người thực hiện`. `null` = chưa có, cột hiện mã. */
  danhBa?: DanhBaTheoMa | null;
  nhanTT: BangNhanTrangThai;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
  /** Cách sắp ĐÃ GỬI cho trang này (`sapXepDayDu`). Bắt buộc: mũi tên phải nói đúng câu hỏi đã gửi. */
  sapXep: SapXepSo;
  /** Bấm một tiêu đề sắp được — a new read key, so the appended pages go. */
  doiSapXep: (cot: CotSapXepNhiemVu) => void;
  /** The `☐` column — see `TaskSelection`. `null` = no column. */
  selection?: TaskSelection | null;
  /**
   * What an empty page shows. The screen passes "nothing yet" or "nothing under these filters"
   * (spec §8b, kept — owner 07/10/2026 #13); this table never guesses. Absent = the filter sentence.
   */
  empty?: ReactNode;
}) {
  if (nhiemVu.length === 0) return empty ?? <EmptyState icon={ClipboardList} title={SO_RONG} />;

  return (
    <div className={TABLE_FRAME_CLASS}>
      <div className="overflow-x-auto" role="region" aria-label="Sổ nhiệm vụ" tabIndex={0}>
        <table className={TABLE_CLASS}>
          <thead>
            <tr className="border-line border-b">
              {selection !== null && (
                <th scope="col" className={cn(TH_CLASS, "w-10")}>
                  <SelectAllBox selection={selection} rows={nhiemVu} />
                </th>
              )}
              <SortHeader cot="code" nhan={NHAN_COT_MA} sapXep={sapXep} doiSapXep={doiSapXep} />
              <SortHeader cot="title" nhan={TITLE_COLUMN_LABEL} sapXep={sapXep} doiSapXep={doiSapXep} />
              <th scope="col" className={TH_CLASS}>
                Người thực hiện
              </th>
              <th scope="col" className={TH_CLASS}>
                Bộ phận
              </th>
              <SortHeader cot="priority" nhan={PRIORITY_COLUMN_LABEL} sapXep={sapXep} doiSapXep={doiSapXep} />
              <SortHeader cot="due_at" nhan={DUE_COLUMN_LABEL} sapXep={sapXep} doiSapXep={doiSapXep} />
              <SortHeader cot="status" nhan={STATUS_COLUMN_LABEL} sapXep={sapXep} doiSapXep={doiSapXep} />
            </tr>
          </thead>
          <tbody>
            {nhiemVu.map((n) => {
              // SUY RA từ `due_at` so với bây giờ (luật 10, bất biến 3) — the tint, the red cell and
              // the `Trễ hạn` badge come from the same comparison, so they never disagree.
              const late = isOverdueNow(n, bayGio);
              const extensions = extensionCountText(n.extension_count);
              return (
                <tr
                  key={n.code}
                  data-tre-han={late ? "" : undefined}
                  className={rowClass(late)}
                  onClick={() => moNhiemVu(n)}
                >
                  {selection !== null && (
                    <td className={TD_CLASS} onClick={(e) => e.stopPropagation()}>
                      <RowCheckbox selection={selection} task={n} />
                    </td>
                  )}
                  <td className={TD_CLASS}>
                    {/* A real button for the keyboard; both open through the same `moNhiemVu`. */}
                    <button
                      type="button"
                      className="text-ink-muted cursor-pointer border-0 bg-transparent p-0 text-[11.5px] font-semibold [font-family:inherit] hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
                      aria-expanded={n.code === maDangMo}
                      onClick={(e) => {
                        e.stopPropagation();
                        moNhiemVu(n);
                      }}
                    >
                      {n.code === "" ? O_TRONG : n.code}
                    </button>
                  </td>
                  {/* Capped at ~42% and cut with "…" (brief 08/10 §5.4): uncapped, one long title pushed
                      the Trạng thái column off a 1440px screen. The full title stays in `title`. */}
                  <td className={cn(TD_CLASS, "w-[42%] max-w-0")} title={n.title}>
                    <span className="text-navy block truncate font-semibold">{n.title}</span>
                    {/* `{nguồn} · đã gia hạn n lần` (prototype `TaskListTable.tsx:83-89`): the second part
                        only when the count is above zero. */}
                    <span className="text-ink-muted block truncate text-[11px]">
                      {nhanNguonGiao(n.source)}
                      {extensions !== null && ` · ${extensions}`}
                    </span>
                  </td>
                  <td className={TD_CLASS}>{nhanCanBoNgan(n.assignee, danhBa, CHUA_PHAN_CONG)}</td>
                  <td className={TD_CLASS}>{n.unit === "" ? O_TRONG : (tenBoPhan.get(n.unit) ?? n.unit)}</td>
                  <td className={TD_CLASS}>{nhanDanhMuc(danhMuc.mucUuTien, n.priority)}</td>
                  <td className={cn(TD_CLASS, late && "text-danger font-semibold")}>
                    {nhanNgay(n.due_at)}
                    {late && ` (${lateText(daysOverdue(n, bayGio))})`}
                  </td>
                  <td className={TD_CLASS}>
                    <SpecStatusBadge state={taskDisplayState(n, nhanTT, bayGio)} status={n.status} />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}


/** Id of the detail's heading — the dialog's accessible name (`LargeDialog titleId`). */
export const TASK_DETAIL_TITLE_ID = "tieu-de-chi-tiet-nhiem-vu";

/** The header's delete button (`task.delete`) — and the confirm dialog's red button. */
export const TASK_DELETE_BUTTON = "Xoá nhiệm vụ";
const TASK_DELETE_TITLE_ID = "task-delete-title";

/** Heading of the information block. */
const TASK_INFO_HEADING_ID = "task-detail-info";
/** Title of the information block of a task without the directive block (prototype). */
export const TASK_INFO_TITLE = "Thông tin nhiệm vụ";
/** `Sửa` of that block — its accessible name. */
export const TASK_INFO_EDIT_LABEL = "Sửa thông tin nhiệm vụ";
/** Above the inline form: the block changed look, and the one place that saves (prototype). */
export const TASK_EDITING_NOTE = "Đang sửa — bấm Lưu ở cuối khối để ghi lại.";
/** Spec 07 §6a toast after a save. */
export const TASK_SAVED = "Đã lưu nhiệm vụ.";

/** Second line of the fact grid's priority cell (spec 07 §4): the STORED progress figure. */
export function progressFactText(progress: number): string {
  return `${progress}% tiến độ ghi nhận`;
}

/** Initials of the timeline's avatar — `task-spec.ts`, re-exported for the callers of this module. */
export { staffInitials } from "./task-spec";

/** One cell of the fact grid (spec 07 §4). */
function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="bg-white px-4 py-2.5">
      <p className="text-ink-muted m-0 mb-1 text-[10.5px] font-bold tracking-wide uppercase">{label}</p>
      {children}
    </div>
  );
}

/** A left-column card of the drawer (spec 07 §5 `Section`). */
function Section({ id, title, children }: { id: string; title: ReactNode; children: ReactNode }) {
  return (
    <section className={SECTION_CLASS} aria-labelledby={id}>
      <h3 id={id} className={SECTION_TITLE_CLASS}>
        {title}
      </h3>
      {children}
    </section>
  );
}

/**
 * Chi tiết nhiệm vụ — spec 07, inside the record-tab panel (the tabs are KEPT, owner 07/10/2026 #1):
 *
 *   header   code chip + title, `unit · assignee`, [Xoá] (task.delete), ✕
 *   strip    `TaskStatusPipeline` — full width, `border-b`
 *   strip    `Chờ duyệt lùi hạn — đã gửi tới …` when a request waits
 *   facts    Hạn xử lý · Cơ quan thực hiện · Người thực hiện · Mức ưu tiên (4 cells)
 *   body     left column (page grey, own scroll): information → Mô tả → Thời hạn → Nhiệm vụ con →
 *            Giao việc, chuyển việc → Đề nghị lùi hạn; right `aside` 24rem: Nhật ký & Trao đổi
 *
 * NO RIGHT ICON RAIL any more (spec 07 line 3, owner 07/10/2026 #7): its Xoá is the header button; the
 * copy-link, source-meeting and focus shortcuts are gone with it (the meeting link stays in the
 * information block). NO `Việc cha` (owner #6). Below 1024px the two columns stack and the whole body
 * scrolls as one.
 *
 * EVERY GATE IS TODAY'S: `clickableTransitions` / `reasonMove` (inside the pipeline),
 * `canShowAssignment`, `canWriteLogEntry`, and `quyen.*` — `Sửa` and the approval ticks behind
 * `task.update`, `Xoá` behind `task.delete`. Hiding is convenience; each route checks the key itself
 * (rule 5, forbidden #1).
 */
export function ChiTietNhiemVu({
  nhiemVu,
  vanBan,
  danhMuc,
  nhanTT,
  tenBoPhan,
  danhBa = null,
  lanLamMoiNhatKy = 0,
  bayGio,
  maNguoiDangNhap,
  quyen,
  canRevealEmail = false,
  dangGui,
  dong,
  doiTrangThai,
  xoa,
  guiDeNghiLuiHan,
  quyetDinh,
  suaKhoiVanBan,
  docLaiChiTiet,
  extensionRefreshKey,
  onExtensionDecided,
  openTask,
  addChild,
  reassign,
}: {
  /**
   * `POST /api/v1/tasks/{ma}/assignment` — the §5.7 block. The caller refreshes the drawer, the
   * timeline and the register on success, then hands the `KetQua` back so the block can toast it.
   * REQUIRED: a caller that forgets it gets a red `tsc`.
   */
  reassign: (body: petitions_taskAssignmentIn) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Changes whenever this task's pending extension requests may have changed. */
  extensionRefreshKey: string;
  /** A decision in the drawer's block succeeded. */
  onExtensionDecided: (taskCode: string) => void;
  /** Open another task's drawer from a register row (a child). */
  openTask: (task: petitions_nhiemVuRa) => void;
  /**
   * `+ Thêm việc con`, or `null` without `task.create`. REQUIRED, not optional: a caller that
   * forgets it gets a red `tsc`, not a drawer that silently lost the button.
   */
  addChild: {
    readonly open: boolean;
    readonly toggle: () => void;
    readonly form: ReactNode;
  } | null;
  nhiemVu: petitions_nhiemVuRa;
  /**
   * Khối văn bản §5.4, đọc từ TUYẾN CHI TIẾT — không bao giờ từ `nhiemVu.documents` của một dòng
   * sổ, vì ở đó trường ấy vắng mặt có chủ ý. Xem `chuyenDrawer`.
   */
  vanBan: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  danhMuc: DanhMucNhiemVu;
  /**
   * The status words for the strip (the spec's fixed words on this screen). DẢI BƯỚC GIỮ THỨ TỰ VÒNG
   * ĐỜI §6, không theo `order`: nó vẽ một đường đi, và xếp lại nó là vẽ một vòng đời không có thật.
   */
  nhanTT: BangNhanTrangThai;
  tenBoPhan: ReadonlyMap<string, string>;
  /** Danh bạ chọn người màn hình đã đọc — tra họ tên cho nhật ký §5.9. `null` = chưa có. */
  danhBa?: KetQua<identity_danhBaChonNguoiRa> | null;
  /** Đổi thì nhật ký §5.9 đọc lại trang đầu. */
  lanLamMoiNhatKy?: number;
  bayGio: Date;
  /** `phien.staff.code` — mã nghiệp vụ `CB-…`, rỗng khi chưa đọc được phiên. */
  maNguoiDangNhap: string;
  /**
   * Cổng từng nút theo khoá `task.*` của phiên — `quyenNhiemVu`. BẮT BUỘC, không tuỳ chọn: một bên
   * gọi quên truyền thì `tsc` đỏ, thay vì một mặc định lặng lẽ mở (hay đóng) mọi nút.
   */
  quyen: QuyenNhiemVu;
  /**
   * `task.read` — the `Xem` beside the assignee's masked address (ADR 0082 #3). Not a field of
   * `QuyenNhiemVu`: those gate WRITE buttons, and `task.read` opens none of them. Absent = closed.
   */
  canRevealEmail?: boolean;
  dangGui: boolean;
  dong: () => void;
  /**
   * `POST /api/v1/tasks/{ma}/status`. The answer comes back so the compose box can keep the typed
   * note and toast the outcome; the caller refreshes the drawer on success.
   */
  doiTrangThai: (
    trangThai: string,
    ghiChu?: string,
    extra?: StatusMoveExtras,
  ) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Soft delete with its mandatory reason. The answer comes back so the confirm can show a refusal. */
  xoa: (lyDo: string) => Promise<KetQua<unknown>>;
  guiDeNghiLuiHan: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  quyetDinh: (
    deNghiID: string,
    duyet: boolean,
    ghiChu?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  /** PATCH /api/v1/tasks/{ma} — `Sửa` and the two approval ticks. Bên gọi cập nhật drawer khi thành công. */
  suaKhoiVanBan: (than: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** GET /api/v1/tasks/{ma} — đọc lại ngay trước một lần lưu CÓ `documents`. */
  docLaiChiTiet: () => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [deleteOpen, setDeleteOpen] = useState(false);
  // The timeline rows the log has loaded — the strip derives the time in status from them (no second
  // read). Keyed by code: another task's rows never time this one.
  const [history, setHistory] = useState<{
    code: string;
    rows: readonly petitions_nhatKyNhiemVuRa[];
    complete: boolean;
  } | null>(null);
  const onHistory = useCallback(
    (rows: readonly petitions_nhatKyNhiemVuRa[], complete: boolean) =>
      setHistory({ code: nhiemVu.code, rows, complete }),
    [nhiemVu.code],
  );

  const danhBaMa = danhBaChoNhatKy(danhBa);
  // §5.7 — `task.assign` and a task that is not terminal. Convenience; the route checks both.
  const showAssignment = canShowAssignment(quyen, nhiemVu.status);
  // Assignee / `task.update` / assigner / creator — convenience; the row decides.
  const canWriteLog = canWriteLogEntry(quyen, nhiemVu, maNguoiDangNhap);
  const late = isOverdueNow(nhiemVu, bayGio);
  const extensions = extensionCountText(nhiemVu.extension_count);

  const unitName = nhiemVu.unit === "" ? null : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit);
  const assigneeEmail = maskedEmailOf(nhiemVu.assignee, danhBaMa);
  const meetingLink =
    cauTuKetLuan(nhiemVu) !== null && nhiemVu.meeting_id !== undefined
      ? duongDanBienBan(nhiemVu.meeting_id)
      : null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-white" aria-labelledby={TASK_DETAIL_TITLE_ID}>
      {/* ── HEADER (spec 07 §1): code chip + title, `unit · assignee`, then Xoá (owner #7) and ✕. The
          code is shown, never edited (ADR 0065 NV3). `border-b` alone — no `border-solid` (3px trap). */}
      <header className="border-line flex shrink-0 items-start gap-3 border-b px-5 py-4">
        <div className="min-w-0 flex-1">
          <div className="flex items-start gap-2">
            {nhiemVu.code !== "" && (
              <span className="border-line text-ink-muted mt-0.5 shrink-0 rounded border bg-[#F7FAFC] px-1.5 py-0.5 text-[10.5px] font-semibold">
                {nhiemVu.code}
              </span>
            )}
            <h2
              id={TASK_DETAIL_TITLE_ID}
              className="text-navy m-0 min-w-0 text-[16px] leading-snug font-bold [overflow-wrap:anywhere]"
            >
              {nhiemVu.title}
            </h2>
          </div>
          <p className="text-ink-muted m-0 mt-1 text-[11.5px]">
            {unitName ?? O_TRONG} · {nhanCanBoNgan(nhiemVu.assignee, danhBaMa, CHUA_PHAN_CONG)}
          </p>
        </div>
        {/* §11.5 soft delete — `task.delete`. The button only OPENS the confirm; the act is its red
            button, after a mandatory reason (rule 7). */}
        {quyen.xoa && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="text-danger hover:not-disabled:text-danger mt-1"
            icon={<Trash2 aria-hidden="true" focusable="false" className="size-3.5" />}
            aria-label={TASK_DELETE_BUTTON}
            aria-haspopup="dialog"
            onClick={() => setDeleteOpen(true)}
          >
            Xoá
          </Button>
        )}
        <CloseDetailButton onClick={dong} />
      </header>

      {/* Below 1024px ONE scroll for everything; from 1024px the strips stay and the two columns
          scroll on their own (spec 07 §5). */}
      <div className="min-h-0 flex-1 overflow-y-auto lg:flex lg:flex-col lg:overflow-hidden">
        <TaskStatusPipeline
          task={nhiemVu}
          labels={nhanTT}
          permissions={quyen}
          staffCode={maNguoiDangNhap}
          showAssignment={showAssignment}
          sending={dangGui}
          move={doiTrangThai}
          history={history !== null && history.code === nhiemVu.code ? history : null}
          now={bayGio}
          units={danhMuc.boPhan}
          directory={danhBa}
        />

        {/* ── PENDING EXTENSION: a strip, not a status (spec 07 §3, prototype
            `TaskDetailDrawer.tsx:351-356`). The task's own `pending_extension` — what every reader of
            the task sees, whatever their right to read the queue. */}
        {nhiemVu.pending_extension && (
          <p
            className="border-tangerine/25 bg-tangerine/8 text-tangerine m-0 flex shrink-0 items-center gap-2 border-b px-5 py-2 text-[12px] font-semibold"
            role="status"
          >
            <CalendarClock aria-hidden="true" focusable="false" className="size-4 shrink-0" />
            {/* No assigner: the prototype's `nameOf(null)` word (`TaskWorkspace.tsx:107`). */}
            Chờ duyệt lùi hạn — đã gửi tới {nhanCanBoNgan(nhiemVu.assigner, danhBaMa, CHUA_PHAN_CONG)}
          </p>
        )}

        {/* ── THE FACT GRID — three columns as the prototype (`TaskDetailDrawer.tsx:358`, ADR 0082 #1:
            the prototype source wins over the spec's four); one column below 640px. Overdue is
            DERIVED from `due_at` and now (rule 10, invariant 3). */}
        <div className="border-line bg-line grid shrink-0 grid-cols-1 gap-px border-b sm:grid-cols-2 lg:grid-cols-4">
          <Fact label="Hạn xử lý">
            {/* Prototype `:366-375`: `Trễ N ngày` over `Hạn d`, else the date over `Còn trong hạn`. */}
            <p className={cn("m-0 text-[12.5px]", late ? "text-danger font-semibold" : "text-navy")}>
              {late ? nhanHanThe(nhiemVu.due_at, bayGio) : nhanNgay(nhiemVu.due_at)}
            </p>
            <p className="text-ink-muted m-0 mt-0.5 text-[11px]">
              {late ? `Hạn ${nhanNgay(nhiemVu.due_at)}` : "Còn trong hạn"}
              {extensions !== null && ` · ${extensions}`}
            </p>
          </Fact>
          {/* One field, two names (ADR 0065 NV5): "cơ quan chủ trì tham mưu" IS the unit. */}
          <Fact label="Cơ quan thực hiện (chủ trì tham mưu)">
            <p className="text-navy m-0 text-[12.5px]">{unitName ?? CHUA_GIAO_BO_PHAN}</p>
            <p className="text-ink-muted m-0 mt-0.5 text-[11px]">
              {nhiemVu.unit !== ""
                ? "Bộ phận chịu trách nhiệm"
                : nhiemVu.assignee !== ""
                  ? "Giao thẳng cho cá nhân"
                  : "Chưa cử ai"}
            </p>
          </Fact>
          {/* "chuyên viên tham mưu" IS the assignee. No assignee: `Chưa phân công`, the prototype's
              `nameOf(null)` (`TaskWorkspace.tsx:107`, `TaskDetailDrawer.tsx:409`).
              STAFF EMAIL (ADR 0082 #3, #10): the prototype's sub-line is `holder.email ?? <assigner
              line>` (`:412-415`) — here the MASKED address with `Xem`; no address (or an assignee the
              directory does not know) keeps the assigner line. */}
          <Fact label="Người thực hiện (chuyên viên tham mưu)">
            <p className="text-navy m-0 text-[12.5px]">{nhanCanBoNgan(nhiemVu.assignee, danhBaMa, CHUA_PHAN_CONG)}</p>
            <p className="text-ink-muted m-0 mt-0.5 text-[11px]">
              {assigneeEmail !== null ? (
                <StaffEmailReveal key={nhiemVu.assignee} code={nhiemVu.assignee} masked={assigneeEmail} canReveal={canRevealEmail} />
              ) : nhiemVu.assigner !== "" ? (
                `Lãnh đạo giao việc: ${nhanCanBoNgan(nhiemVu.assigner, danhBaMa, O_TRONG)}`
              ) : (
                "Chưa chỉ định người thực hiện"
              )}
            </p>
          </Fact>
          <Fact label="Mức ưu tiên">
            <p className="text-navy m-0 text-[12.5px]">{nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)}</p>
            <p className="text-ink-muted m-0 mt-0.5 text-[11px]">{progressFactText(nhiemVu.progress)}</p>
          </Fact>
        </div>

        <div className="lg:flex lg:min-h-0 lg:flex-1">
          {/* ── LEFT (spec 07 §6), in the prototype's order ─────────────────────────────────── */}
          <div className="bg-canvas min-w-0 px-5 py-4 lg:flex-1 lg:overflow-y-auto">
            {/* `key` by code: another task's open edit form must not appear on this one. */}
            <TaskInfoBlock
              key={`thong-tin-${nhiemVu.code}`}
              task={nhiemVu}
              types={danhMuc.loai}
              documents={vanBan}
              unitName={unitName ?? O_TRONG}
              assigneeName={nhanCanBoNgan(nhiemVu.assignee, danhBaMa, CHUA_PHAN_CONG)}
              canEdit={quyen.capNhat}
              meetingLink={meetingLink}
              save={suaKhoiVanBan}
              reread={docLaiChiTiet}
            />

            {/* Mô tả: only when there is one, as in the prototype — an empty card says nothing. */}
            {nhiemVu.description !== "" && (
              <Section id="task-detail-description" title="Mô tả nhiệm vụ">
                <p className="m-0 text-[13px] whitespace-pre-line">{nhiemVu.description}</p>
              </Section>
            )}

            {/* §5.6 — HAI HẠN CẠNH NHAU, with the time: the time is part of the deadline.
                `Hạn ban đầu` does not move with an extension; on-time rates count against it. */}
            <Section id="task-detail-deadlines" title="Thời hạn">
              <div className="grid grid-cols-2 gap-4">
                {(
                  [
                    ["Hạn xử lý", nhiemVu.due_at],
                    ["Hạn ban đầu", nhiemVu.original_due_at],
                  ] as const
                ).map(([label, at]) => (
                  <div key={label} className="mt-4">
                    <p className="text-ink-muted m-0 text-[11px] font-semibold tracking-wide uppercase">{label}</p>
                    <p className="m-0 text-[13px]">{at === null ? O_TRONG : nhanThoiDiem(at)}</p>
                  </div>
                ))}
              </div>
            </Section>

            {/* §5.10 `Nhiệm vụ con` (#6) — drawn only when there are children (spec 07 §6d).
                `key` by code: another task's "Xem thêm" page must not follow into this one. */}
            <ChildTasks
              key={`con-${nhiemVu.code}`}
              parentCode={nhiemVu.code}
              refreshKey={lanLamMoiNhatKy}
              openTask={openTask}
            />
            {/* `+ Thêm việc con` (kept, owner #6) opens THE SAME create dialog as `+ Giao việc mới`,
                stacked over this panel (two native modals stack in the top layer). */}
            {addChild !== null && (
              <div className="mb-3">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  icon={<Plus aria-hidden="true" focusable="false" className="size-3.5" />}
                  aria-haspopup="dialog"
                  onClick={addChild.toggle}
                >
                  {ADD_CHILD_BUTTON.replace(/^\+\s*/, "")}
                </Button>
                {addChild.open && addChild.form}
              </div>
            )}

            {/* §5.7 — hand the SAME task to another unit or person; changing the officer who follows
                it is changing the assignee (ADR 0065 NV5). */}
            {showAssignment && (
              <Section id="tieu-de-giao-viec-chuyen-viec" title={ASSIGNMENT_TITLE}>
                <TaskAssignmentBlock
                  key={`giao-lai-${nhiemVu.code}`}
                  task={nhiemVu}
                  units={danhMuc.boPhan}
                  directory={danhBa}
                  labels={nhanTT}
                  reassign={reassign}
                />
              </Section>
            )}

            {/* `Đề nghị lùi hạn` — ONE state at a time (spec 07 §6f): the pending request with Duyệt /
                Từ chối for its approver, else the form to ask. `Lịch sử gia hạn` follows it. */}
            <TaskExtensionSection
              key={`lui-han-${nhiemVu.code}`}
              taskCode={nhiemVu.code}
              dueAt={nhiemVu.due_at}
              directory={danhBaMa}
              sessionStaffCode={maNguoiDangNhap}
              // LAYER ONE — `task.extend` of the SESSION. Layer two (ADR 0038) runs inside the row
              // gate, and the server decides both again in the transaction.
              canApproveExtension={quyen.duyetGiaHan}
              // `task.update` — the key of SENDING a request (ADR 0038: never `task.extend`).
              canRequest={quyen.capNhat}
              refreshKey={extensionRefreshKey}
              decide={quyetDinh}
              request={guiDeNghiLuiHan}
              onDecided={onExtensionDecided}
            />
            {/* Spec 07 §6g / prototype `TaskDetailDrawer.tsx:615-643`: every request, newest first —
                drawn only when there is one. `GET /api/v1/tasks/{ma}/extensions` (ADR 0076 #4a). */}
            <TaskExtensionHistory
              key={`lich-su-gia-han-${nhiemVu.code}`}
              taskCode={nhiemVu.code}
              refreshKey={extensionRefreshKey}
            />
          </div>

          {/* ── RIGHT: Nhật ký & Trao đổi (spec 08) — a white `aside`, 24rem, its composer fixed at
              the top and the timeline scrolling under it. */}
          <aside className="border-line min-w-0 border-t bg-white lg:flex lg:w-[24rem] lg:shrink-0 lg:flex-col lg:border-t-0 lg:border-l">
            <NhatKyNhiemVu
              key={nhiemVu.code}
              maNhiemVu={nhiemVu.code}
              nhanTT={nhanTT}
              danhBa={danhBaMa}
              tenBoPhan={tenBoPhan}
              lanLamMoi={lanLamMoiNhatKy}
              canWrite={canWriteLog}
              onHistory={onHistory}
            />
          </aside>
        </div>
      </div>

      {quyen.xoa && deleteOpen && (
        <TaskDeleteDialog
          code={nhiemVu.code}
          sending={dangGui}
          remove={xoa}
          close={() => setDeleteOpen(false)}
        />
      )}
    </div>
  );
}

/**
 * Xoá mềm §11.5 — a centred confirm stacked over the detail, with the MANDATORY reason (rule 7: the
 * row stays with who deleted it and why; an issued code is never reissued). Same route, same body.
 *
 * A refusal ("còn 3 việc con chưa xoá…", ADR 0037 decision 3) shows HERE, verbatim, and the dialog
 * stays open with the reason kept. On success the caller toasts and drops the record's tab, which
 * unmounts the whole detail.
 */
function TaskDeleteDialog({
  code,
  sending,
  remove,
  close,
}: {
  code: string;
  sending: boolean;
  remove: (reason: string) => Promise<KetQua<unknown>>;
  close: () => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    document.getElementById("ly-do-xoa-nhiem-vu")?.focus();
  }, []);

  function submit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    const text = reason.trim();
    if (text === "" || busy) return;
    setBusy(true);
    setError(null);
    remove(text).then((r) => {
      setBusy(false);
      if (!r.ok) setError(r.thongBao);
    });
  }

  return (
    <ModalDialog titleId={TASK_DELETE_TITLE_ID} closeDisabled={busy} onDismiss={() => !busy && close()}>
      <ModalDialogHeader
        titleId={TASK_DELETE_TITLE_ID}
        title={`Xoá nhiệm vụ ${code} khỏi sổ?`}
        description="Xoá mềm: dòng ở lại cùng người xoá và lý do, mã sổ đã cấp thì không bao giờ cấp lại. Nhiệm vụ còn việc con chưa xoá thì máy chủ từ chối và nói rõ còn mấy việc."
      />
      <form className="m-0 flex flex-col gap-4" onSubmit={submit}>
        <div>
          <label htmlFor="ly-do-xoa-nhiem-vu" className={LABEL_CLASS}>
            Lý do xoá (bắt buộc)
          </label>
          <input
            id="ly-do-xoa-nhiem-vu"
            name="ly-do-xoa-nhiem-vu"
            value={reason}
            autoComplete="off"
            required
            className={INPUT_CLASS}
            onChange={(e) => setReason(e.target.value)}
          />
        </div>
        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" disabled={busy} onClick={close}>
            {NHAN_NUT_HUY}
          </Button>
          <Button
            type="submit"
            variant="primary"
            className="bg-danger hover:not-disabled:bg-danger/90 text-white"
            icon={
              busy ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
              ) : (
                <Trash2 aria-hidden="true" focusable="false" className="size-4" />
              )
            }
            disabled={busy || sending || reason.trim() === ""}
          >
            {TASK_DELETE_BUTTON}
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/** A labelled value of the information block (spec 07 §6a: label 11px muted, value 13px). */
function InfoField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <p className="text-ink-muted m-0 text-[11px]">{label}</p>
      <div className="m-0 text-[13px]">{children}</div>
    </div>
  );
}

/** Label class of the information block — bolder in edit mode (prototype `labelClass`). */
const EDIT_LABEL_CLASS = "text-ink mb-1 block text-[11.5px] font-semibold";

/**
 * Whether the information block is the "Sổ theo dõi văn bản chỉ đạo" (spec 07 §6a): a task whose
 * type the catalogue flags `requires_directive`, or a task that already carries a document, a result,
 * a note or an approval tick (prototype `TaskDetailDrawer.tsx:179-186`) — so a commune that changed a
 * task's type never hides what somebody entered. `documents` counts only once the detail has answered
 * (`xong`); the list row carries no documents (`DrawerNhiemVu`).
 */
export function showsDirectiveBlock(
  task: petitions_nhiemVuRa,
  types: readonly petitions_loaiNhiemVuRa[],
  documents?: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>,
): boolean {
  return (
    needsDirective(types, task.type) ||
    (documents !== undefined && documents.pha === "xong" && documents.duLieu.length > 0) ||
    task.result_summary !== "" ||
    task.note !== "" ||
    task.leader_approved ||
    task.superior_acknowledged
  );
}

/**
 * The information block (spec 07 §6a, prototype `TaskRegisterSection`) with its `Sửa` IN PLACE. Its
 * heading sits OUTSIDE the box; the box changes look while editing.
 *
 *   view   Mã · Tên · Hạn xử lý on one row; for the directive block also Cơ quan chủ trì tham mưu and
 *          Chuyên viên Văn phòng tham mưu / theo dõi (the SAME unit and assignee — display only,
 *          owner 07/10/2026 #8), the three document groups, the result summary, the note, and at the
 *          foot the two approval ticks, ticked IN VIEW (ADR 0076 #3)
 *   edit   `Theo văn bản` → `FormSuaKhoiVanBan`; any other type → `BasicTaskEditForm` (title, deadline).
 *          The code is shown, NEVER an input (ADR 0065 NV3)
 *
 * `Sửa` stands behind `task.update`. For a `Theo văn bản` task it is LOCKED, with the reason, until
 * the document block is read (`lyDoKhoaSua`): `documents` on PATCH replaces the whole set, so editing
 * an unread set drops rows nobody saw. Focus returns to `Sửa` after `Lưu` and `Huỷ`.
 */
function TaskInfoBlock({
  task,
  types,
  documents,
  unitName,
  assigneeName,
  canEdit,
  meetingLink,
  save,
  reread,
}: {
  task: petitions_nhiemVuRa;
  /** The commune's task types — `requires_directive` decides the directive block. */
  types: readonly petitions_loaiNhiemVuRa[];
  documents: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  unitName: string;
  assigneeName: string;
  /** `task.update`. Without it there is no `Sửa` and the ticks are read-only. */
  canEdit: boolean;
  meetingLink: string | null;
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  reread: () => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [editing, setEditing] = useState(false);
  const editButton = useRef<HTMLButtonElement>(null);
  const returnFocus = useRef(false);

  useEffect(() => {
    if (editing || !returnFocus.current) return;
    returnFocus.current = false;
    editButton.current?.focus();
  }, [editing]);

  const hasDocuments = needsDirective(types, task.type);
  const directive = showsDirectiveBlock(task, types, documents);
  const lock = hasDocuments ? lyDoKhoaSua(documents) : null;
  // ALL THREE AT ONCE. A `Theo văn bản` block that leaves the `xong` phase mid-edit (a failed
  // re-read) hides the form — no editing on a set the server just failed to confirm.
  const showForm =
    canEdit && editing && (!hasDocuments || (documents.pha === "xong" && lock === null));
  const title = directive ? TIEU_DE_KHOI_VAN_BAN : TASK_INFO_TITLE;

  function leave(): void {
    returnFocus.current = true;
    setEditing(false);
  }

  return (
    <section className="m-0 mb-5" aria-labelledby={TASK_INFO_HEADING_ID}>
      <div className="mb-2 flex items-center gap-2">
        <h3
          id={TASK_INFO_HEADING_ID}
          tabIndex={-1}
          className="text-navy m-0 text-[12px] font-bold tracking-wide uppercase focus-visible:outline-2 focus-visible:outline-brand-500"
        >
          {title}
        </h3>
        {canEdit && !showForm && (
          <Button
            ref={editButton}
            type="button"
            variant="outline"
            size="sm"
            className="ml-auto"
            icon={<Pencil aria-hidden="true" focusable="false" className="size-3.5" />}
            // Named by WHAT IT EDITS: a type without the flag opens the basic form (title, deadline)
            // even when the block is shown for the data it carries (`showsDirectiveBlock`).
            aria-label={hasDocuments ? `Sửa ${TIEU_DE_KHOI_VAN_BAN.toLowerCase()}` : TASK_INFO_EDIT_LABEL}
            aria-describedby={lock !== null ? "ly-do-khoa-sua-van-ban" : undefined}
            disabled={lock !== null}
            onClick={() => setEditing(true)}
          >
            Sửa
          </Button>
        )}
      </div>
      {canEdit && !showForm && lock !== null && (
        <p id="ly-do-khoa-sua-van-ban" className={cn(HINT_CLASS, "mb-2")}>
          {lock}
        </p>
      )}

      <div
        className={cn(
          "space-y-3 rounded-[10px] border bg-white",
          showForm ? "border-brand/35 border-l-brand border-l-4 p-4" : "border-line p-3",
        )}
      >
        {showForm ? (
          <>
            <p className="text-brand m-0 mb-1 flex items-center gap-1.5 text-[11.5px] font-semibold">
              <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
              {TASK_EDITING_NOTE}
            </p>
            {hasDocuments && documents.pha === "xong" ? (
              <FormSuaKhoiVanBan
                nhiemVu={task}
                vanBan={documents.duLieu}
                unitName={unitName}
                assigneeName={assigneeName}
                luu={save}
                docLai={reread}
                xong={leave}
              />
            ) : (
              <BasicTaskEditForm task={task} save={save} reread={reread} done={leave} />
            )}
          </>
        ) : (
          <>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-[7rem_1fr_10rem]">
              <InfoField label="Mã nhiệm vụ">
                <span className="font-semibold">{task.code === "" ? O_TRONG : task.code}</span>
              </InfoField>
              <InfoField label={nhanOTieuDe(directive)}>
                <span className="font-semibold">{task.title}</span>
              </InfoField>
              <InfoField label="Hạn xử lý">
                <span className="font-semibold">{nhanNgay(task.due_at)}</span>
              </InfoField>
            </div>
            {directive && (
              <>
                <InfoField label="Cơ quan chủ trì tham mưu">{unitName}</InfoField>
                <InfoField label="Chuyên viên Văn phòng tham mưu / theo dõi">{assigneeName}</InfoField>
                <DocKhoiVanBan tai={documents} />
                <InfoField label="Tóm tắt kết quả thực hiện">
                  <span className="whitespace-pre-line">{task.result_summary === "" ? O_TRONG : task.result_summary}</span>
                </InfoField>
                <InfoField label="Ghi chú">
                  <span className="whitespace-pre-line">{task.note === "" ? O_TRONG : task.note}</span>
                </InfoField>
              </>
            )}
            {/* §7.4 — the way back to the meeting a task was split from, when the server could link
                it (`meeting_id`). Kept from the removed rail (no ADR drops it). */}
            {meetingLink !== null && (
              <p className="m-0 text-[12px]">
                <Link href={meetingLink}>{cauTuKetLuan(task)}</Link>
              </p>
            )}
          </>
        )}

        {directive && <ApprovalTicks task={task} canEdit={canEdit} locked={showForm} save={save} />}
      </div>
    </section>
  );
}

/**
 * The two approval ticks at the foot of the directive block (spec 07 §6a, ADR 0076 #3): ticked IN
 * VIEW, each tick one `PATCH` at once with the optimistic-lock token of the task as read. Behind
 * `task.update` on the server AND here (never `task.assign`, ADR 0076 #3). Locked while the block is
 * being edited: a tick would move the task's version under the open form, and its save would be a
 * 409. Manual marks — they never change the status (the note under them says so).
 */
function ApprovalTicks({
  task,
  canEdit,
  locked,
  save,
}: {
  task: petitions_nhiemVuRa;
  canEdit: boolean;
  locked: boolean;
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [busy, setBusy] = useState(false);

  function tick(body: petitions_suaNhiemVuVao): void {
    setBusy(true);
    save({ ...body, expected_updated_at: task.updated_at }).then((r) => {
      setBusy(false);
      // The server's sentence verbatim; the caller re-reads the detail, so the box shows the truth.
      if (!r.ok) toast.error(r.thongBao);
    });
  }

  const disabled = !canEdit || busy || locked;
  return (
    <div className="border-line space-y-2 border-t pt-3">
      <label className="text-ink m-0 flex items-center gap-2.5 text-[12.5px]">
        <input
          type="checkbox"
          className={CHECKBOX_CLASS}
          checked={task.leader_approved}
          disabled={disabled}
          onChange={(e) => tick({ leader_approved: e.target.checked })}
        />
        Lãnh đạo xã đã phê duyệt hoàn thành
      </label>
      <label className="text-ink m-0 flex items-center gap-2.5 text-[12.5px]">
        <input
          type="checkbox"
          className={CHECKBOX_CLASS}
          checked={task.superior_acknowledged}
          disabled={disabled}
          onChange={(e) => tick({ superior_acknowledged: e.target.checked })}
        />
        Cấp trên đã công nhận hoàn thành
      </label>
      <p className="text-ink-muted m-0 text-[11px]">{CHU_THICH_HAI_O_TICK}</p>
    </div>
  );
}

/**
 * The three document groups, READ ONLY (spec 07 §6a) — THREE PHASES, THREE SCREENS: `dangTai` says
 * loading; `loi` says the server's sentence and draws NO group (three `—` would read as "no
 * documents", which nobody recorded); `xong` draws the groups, `—` for an empty one.
 */
function DocKhoiVanBan({ tai }: { tai: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]> }) {
  if (tai.pha === "dangTai") {
    return (
      <p role="status" className="text-ink-muted m-0 text-[12px]">
        {DANG_TAI_VAN_BAN}
      </p>
    );
  }
  if (tai.pha === "loi") {
    return (
      <p className="thong-bao-loi m-0" role="alert">
        {KHONG_DOC_DUOC_VAN_BAN} {tai.thongBao}
      </p>
    );
  }
  return (
    <>
      {chiaNhomVanBan(tai.duLieu).map((nhom) => (
        <InfoField key={nhom.ma} label={nhom.nhan}>
          {nhom.vanBan.length === 0 ? (
            O_TRONG
          ) : (
            <ul className="m-0 list-none space-y-1 p-0">
              {nhom.vanBan.map((v) => (
                <li key={v.id} className="text-[13px]">
                  <span className="font-semibold">{v.reference === "" ? KHONG_SO : v.reference}</span>
                  {v.date !== "" && <span className="text-ink-muted"> · {ngayVanBan(v.date)}</span>}
                  {v.summary !== "" && <div className="text-ink-muted text-[11.5px]">{v.summary}</div>}
                </li>
              ))}
            </ul>
          )}
        </InfoField>
      ))}
    </>
  );
}

/** `YYYY-MM-DDTHH:mm` of the form's deadline, or `""` (the `datetime-local` value). */
function dueInputOf(f: FormSuaNhiemVu): string {
  return f.dueDate !== "" && f.dueTime !== "" ? `${f.dueDate}T${f.dueTime}` : "";
}

/** The `datetime-local` value back into the form's two halves. */
function dueFromInput(value: string): Pick<FormSuaNhiemVu, "dueDate" | "dueTime"> {
  const [date = "", time = ""] = value.split("T");
  return { dueDate: date, dueTime: time.slice(0, 5) };
}

/**
 * The first row of both edit forms (prototype `grid-cols-[7rem_1fr_10rem]`): the code SHOWN (never an
 * input — ADR 0065 NV3: an issued register code is immutable, rule 7 invariant 3), the title, and the
 * deadline as ONE `datetime-local` box — correcting a typo in the date, not an extension (ADR 0065
 * NV4; extensions go through the request below).
 */
function InfoEditRow({
  code,
  titleLabel,
  title,
  due,
  setTitle,
  setDue,
}: {
  code: string;
  titleLabel: string;
  title: string;
  due: string;
  setTitle: (v: string) => void;
  setDue: (v: string) => void;
}) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-[7rem_1fr_10rem]">
      <div>
        <p className={EDIT_LABEL_CLASS}>Mã nhiệm vụ</p>
        <p className="m-0 mt-2 text-[13px] font-semibold">{code === "" ? O_TRONG : code}</p>
      </div>
      <div>
        <label htmlFor="sua-tieu-de" className={EDIT_LABEL_CLASS}>
          {titleLabel}
        </label>
        <input
          id="sua-tieu-de"
          name="sua-tieu-de"
          value={title}
          required
          maxLength={TIEU_DE_NHIEM_VU_TOI_DA}
          autoComplete="off"
          className={cn(INPUT_CLASS, "font-semibold")}
          onChange={(e) => setTitle(e.target.value)}
        />
      </div>
      <div>
        <label htmlFor="sua-han" className={EDIT_LABEL_CLASS}>
          Hạn xử lý
        </label>
        <input
          id="sua-han"
          name="sua-han"
          type="datetime-local"
          value={due}
          className={cn(INPUT_CLASS, "px-2")}
          onChange={(e) => setDue(e.target.value)}
        />
      </div>
    </div>
  );
}

/** The two buttons closing an edit form (spec 07 §6a: `Huỷ` · `Lưu`, small). */
function EditButtons({ busy, onCancel }: { busy: boolean; onCancel: () => void }) {
  return (
    <div className="flex justify-end gap-2">
      <Button type="button" variant="outline" size="sm" disabled={busy} onClick={onCancel}>
        {NHAN_NUT_HUY}
      </Button>
      <Button
        type="submit"
        variant="primary"
        size="sm"
        disabled={busy}
        aria-busy={busy || undefined}
        icon={busy ? <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" /> : undefined}
      >
        {NHAN_NUT_LUU}
      </Button>
    </div>
  );
}

/**
 * Form `Sửa` of the directive block.
 *
 * BẮT ĐẦU TỪ CHI TIẾT, VÀ CHỤP LẠI CHI TIẾT ẤY LÚC MỞ (`goc`). Thân PATCH so form với BẢN CHỤP, không
 * với chi tiết mới nhất: "không đổi" nghĩa là cán bộ chưa đụng tới, nên một lần đọc lại chi tiết giữa
 * chừng (sau một lần đổi trạng thái) không được biến những ô cán bộ để yên thành "đã đổi". Nothing
 * changed → `Lưu` simply leaves the form (no request).
 *
 * KHOÁ LẠC QUAN (d2ed15e): mọi lần lưu mang `expected_updated_at` của BẢN CHỤP. Ai đó đã ghi nhiệm vụ
 * kể từ lúc mở form thì máy chủ từ chối 409 — its sentence is the error toast, with `TASK_CHANGED_NOTE`
 * when a re-read confirms the task moved on; the form keeps what was typed and does NOT adopt the new
 * version itself (`changedSince`).
 *
 * Dòng văn bản dùng LẠI `NhomVanBanNhap` của form tạo: cùng ô, cùng giới hạn, cùng cách kiểm. The
 * prototype's ONE free-text box per document needs the server to parse a reference (BACKEND
 * DEPENDENCY, owner 07/10/2026 #11): our three fields stay, the "?" says why.
 */
export function FormSuaKhoiVanBan({
  nhiemVu,
  vanBan,
  unitName,
  assigneeName,
  luu,
  docLai,
  xong,
}: {
  nhiemVu: petitions_nhiemVuRa;
  vanBan: readonly petitions_nhiemVuVanBanRa[];
  /** Shown read-only: unit and assignee change through `Giao việc, chuyển việc` (ADR 0065 NV5). */
  unitName: string;
  assigneeName: string;
  luu: (than: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  /**
   * Đọc lại chi tiết NGAY TRƯỚC một lần lưu có `documents` — xem `vanBanDaDoiOMayChu`. Thu hẹp khe
   * hở gỡ nhầm văn bản người khác vừa thêm; KHÔNG đóng được nó khi hợp đồng chưa có phiên bản.
   */
  docLai: () => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Rời chế độ sửa — sau khi lưu thành công, hoặc khi bấm `Huỷ`. */
  xong: () => void;
}) {
  const [goc] = useState(() => ({ nhiemVu, vanBan }));
  const [f, datF] = useState<FormSuaNhiemVu>(() => formSuaTuChiTiet(nhiemVu, vanBan));
  const [dangLuu, datDangLuu] = useState(false);
  const demKhoaVanBan = useRef(0);
  // Mở form thì tiêu điểm vào ô đầu tiên sửa được; sau đó là ô vừa thêm / nút thêm của nhóm vừa gỡ.
  const oCanTieuDiem = useRef<string | null>("sua-tieu-de");

  useEffect(() => {
    if (oCanTieuDiem.current === null) return;
    document.getElementById(oCanTieuDiem.current)?.focus();
    oCanTieuDiem.current = null;
  }, [f.vanBan]);

  const than = thanSuaNhiemVu(f, goc.nhiemVu, goc.vanBan);
  const chan = canhBaoSua(f, goc.nhiemVu);
  function doi(sua: Partial<FormSuaNhiemVu>) {
    datF((cu) => ({ ...cu, ...sua }));
  }

  function themVanBan(nhom: NhomVanBan) {
    demKhoaVanBan.current += 1;
    const khoa = `moi${demKhoaVanBan.current}`;
    oCanTieuDiem.current = `sua-van-ban-${khoa}`;
    datF((cu) => ({
      ...cu,
      vanBan: [...cu.vanBan, { khoa, id: "", nhom, trichYeu: "", soKyHieu: "", ngay: "" }],
    }));
  }

  function goVanBan(dong: DongVanBanNhap) {
    oCanTieuDiem.current = `sua-them-van-ban-${dong.nhom}`;
    datF((cu) => ({ ...cu, vanBan: cu.vanBan.filter((d) => d.khoa !== dong.khoa) }));
  }

  function suaVanBan(khoa: string, sua: Partial<Omit<DongVanBanNhap, "khoa" | "nhom">>) {
    datF((cu) => ({
      ...cu,
      vanBan: cu.vanBan.map((d): DongVanBanSua => (d.khoa === khoa ? { ...d, ...sua } : d)),
    }));
  }

  async function gui(e: FormEvent) {
    e.preventDefault();
    if (chan !== null || dangLuu) return;
    // Nothing changed: the prototype's `Lưu` closes the form; there is nothing to send.
    if (than === null) {
      xong();
      return;
    }
    datDangLuu(true);

    // THÂN CÓ `documents` THÌ ĐỌC LẠI TRƯỚC. Đọc hỏng, hoặc khối đã đổi ở máy chủ: KHÔNG gửi, giữ
    // nguyên chữ đã gõ, nói ra. Không tự gộp.
    if (canDocLaiTruocKhiLuu(than)) {
      // So với BẢN CHỤP lúc mở (`goc`), không với `vanBan` mới nhất của drawer: một dòng người khác
      // thêm mà drawer đã đọc lại giữa chừng thì CÓ trong `vanBan`, nhưng KHÔNG có trong form.
      const lyDoChan = loiSauKhiDocLai(await docLai(), goc.vanBan);
      if (lyDoChan !== null) {
        datDangLuu(false);
        toast.error(lyDoChan);
        return;
      }
    }

    const kq = await luu(than);
    if (!kq.ok) {
      // Ở LẠI chế độ sửa, giữ nguyên chữ đã gõ; câu máy chủ NGUYÊN VĂN — kể cả câu 409 "gỡ dòng ấy
      // rồi thêm lại ở nhóm mới" hay "nhiệm vụ vừa được người khác sửa".
      const stale = changedSince(await docLai(), goc.nhiemVu.updated_at);
      datDangLuu(false);
      toast.error(stale ? `${kq.thongBao} ${TASK_CHANGED_NOTE}` : kq.thongBao);
      return;
    }
    datDangLuu(false);
    toast.success(TASK_SAVED);
    xong();
  }

  return (
    <form className="m-0 space-y-3" onSubmit={gui} aria-label={`Sửa ${TIEU_DE_KHOI_VAN_BAN.toLowerCase()}`}>
      <InfoEditRow
        code={nhiemVu.code}
        titleLabel={NHAN_TIEU_DE_THEO_VAN_BAN}
        title={f.tieuDe}
        due={dueInputOf(f)}
        setTitle={(v) => doi({ tieuDe: v })}
        setDue={(v) => doi(dueFromInput(v))}
      />

      {/* Display only (owner 07/10/2026 #8): the SAME unit and assignee, changed in the assignment block. */}
      <InfoField label="Cơ quan chủ trì tham mưu">{unitName}</InfoField>
      <InfoField label="Chuyên viên Văn phòng tham mưu / theo dõi">{assigneeName}</InfoField>

      {MOI_NHOM_VAN_BAN.map((nhom, i) => (
        <NhomVanBanNhap
          key={nhom}
          tienTo="sua"
          nhom={nhom}
          dong={dongCuaNhom(f.vanBan, nhom)}
          them={() => themVanBan(nhom)}
          go={goVanBan}
          sua={suaVanBan}
          pending={i === 0}
        />
      ))}

      <div>
        <label htmlFor="sua-tom-tat-ket-qua" className={EDIT_LABEL_CLASS}>
          Tóm tắt kết quả thực hiện
        </label>
        <textarea
          id="sua-tom-tat-ket-qua"
          name="sua-tom-tat-ket-qua"
          rows={2}
          maxLength={TOM_TAT_KET_QUA_TOI_DA}
          value={f.tomTatKetQua}
          className={TEXTAREA_CLASS}
          onChange={(e) => doi({ tomTatKetQua: e.target.value })}
        />
      </div>

      <div>
        <label htmlFor="sua-ghi-chu" className={EDIT_LABEL_CLASS}>
          Ghi chú
        </label>
        <textarea
          id="sua-ghi-chu"
          name="sua-ghi-chu"
          rows={2}
          maxLength={GHI_CHU_NHIEM_VU_TOI_DA}
          value={f.ghiChu}
          className={TEXTAREA_CLASS}
          onChange={(e) => doi({ ghiChu: e.target.value })}
        />
      </div>

      {/* A rule the form refuses — inline, under the fields (owner 07/10/2026 #12). */}
      {chan !== null && <p className="text-danger m-0 text-xs font-medium">{chan}</p>}

      <EditButtons busy={dangLuu} onCancel={xong} />
    </form>
  );
}

/**
 * The inline `Sửa` form of a task whose type has NO document block: the title and the deadline
 * (ADR 0065 NV4 — the deadline is editable on every type). `Theo văn bản` edits the same two fields
 * inside `FormSuaKhoiVanBan`; the two never show together, so they share the field ids.
 *
 * The body is `basicTaskEditBody` — title and `due_at` only, plus the optimistic-lock token of the
 * task AS THE FORM OPENED IT. A refusal — a 409 "changed by someone else" included — is the error
 * toast, the typed values kept. The caller renders it only for `task.update`.
 */
function BasicTaskEditForm({
  task,
  save,
  reread,
  done,
}: {
  task: petitions_nhiemVuRa;
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  reread: () => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Leave edit mode — after a successful save, or on `Huỷ`. */
  done: () => void;
}) {
  const [opened] = useState(task);
  const [f, setF] = useState<FormSuaNhiemVu>(() => formSuaTuChiTiet(task, []));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    document.getElementById("sua-tieu-de")?.focus();
  }, []);

  // Drawn only for a type without the directive block (`TaskInfoBlock`'s `hasDocuments`).
  const titleLabel = nhanOTieuDe(false);
  const body = basicTaskEditBody(f, opened);
  const block = canhBaoSua(f, opened, titleLabel);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (block !== null || saving) return;
    if (body === null) {
      done();
      return;
    }
    setSaving(true);
    const r = await save(body);
    if (!r.ok) {
      const stale = changedSince(await reread(), opened.updated_at);
      setSaving(false);
      toast.error(stale ? `${r.thongBao} ${TASK_CHANGED_NOTE}` : r.thongBao);
      return;
    }
    setSaving(false);
    toast.success(TASK_SAVED);
    done();
  }

  return (
    <form className="m-0 space-y-3" onSubmit={submit} aria-label={TASK_INFO_EDIT_LABEL}>
      <InfoEditRow
        code={opened.code}
        titleLabel={titleLabel}
        title={f.tieuDe}
        due={dueInputOf(f)}
        setTitle={(v) => setF((prev) => ({ ...prev, tieuDe: v }))}
        setDue={(v) => setF((prev) => ({ ...prev, ...dueFromInput(v) }))}
      />
      {block !== null && <p className="text-danger m-0 text-xs font-medium">{block}</p>}
      <EditButtons busy={saving} onCancel={done} />
    </form>
  );
}


/**
 * Form `Giao việc mới` §7 — spec 06 (`TaskAssignForm`). ONE form, shared IN PLACE with Biên bản
 * (`Tách thành nhiệm vụ`) and Phản ánh (`Tạo nhiệm vụ`): the look changes for all three; the
 * Nhiệm vụ-only rules hang on `taskScreen`.
 *
 * HAI LOẠI, HAI BỘ TRƯỜNG (§7.2 / §7.3), rẽ nhánh trên cờ `requires_directive` của danh mục loại
 * (`needsDirective`), không trên một mã gõ trong mã nguồn. Loại nào khác thì ô tiêu đề thành `Tên nhiệm vụ`, và ba nhóm văn bản BIẾN
 * KHỎI MÀN và KHÔNG LÊN DÂY — phần "không lên dây" ở `thanGiaoViec`, nơi có bài kiểm. Ô `Ghi chú`
 * §7.2 đi cùng cổng với ba danh sách văn bản (`hienVanBan`): tuyến tách kết luận của màn Biên bản
 * không nhận `note`.
 *
 * ON THE NHIỆM VỤ SCREEN (`taskScreen`, owner 07/10/2026, ADR 0076 lần 2):
 *   - the type starts on the commune's `is_default` row, else its FIRST active row (#2) — no empty
 *     choice; Biên bản and Phản ánh keep "no default → `— Chọn loại —`" (a1e5e64f);
 *   - `Cơ quan chủ trì tham mưu` / `Chuyên viên tham mưu / theo dõi` appear ONLY for `Theo văn bản`
 *     (spec 06 §5) — a basic task is assigned in its detail (`Giao việc, chuyển việc`). They ARE the
 *     unit and the assignee (ADR 0065 NV5): one field, two names;
 *   - `Lãnh đạo giao việc` empty = `— Người đang tạo nhiệm vụ —`, hint "…qua chuông và qua thư" (#2) —
 *     though the server stores NOBODY when it is left empty (ADR 0076 lần 2, consequence (b));
 *   - `Mức ưu tiên` has no empty choice: the commune's default row, else its first;
 *   - 500px wide, 800px for `Theo văn bản` (#9).
 *
 * `Tự sinh mã` MẶC ĐỊNH BẬT, đúng §7.1: mã do máy chủ cấp theo dãy `NV01, NV02…`, và một mã đã
 * cấp thì không bao giờ cấp lại kể cả sau xoá mềm (luật 7, bất biến 3).
 *
 * KHOÁ CHỐNG TRÙNG SỐNG BẰNG ĐỜI MỘT LẦN MỞ FORM. Sinh mới ở mỗi lần bấm thì lần bấm lại sau một
 * lỗi mạng là một khoá mới — tức đúng cái khoá chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ
 * đã tới máy chủ và đã cấp một số sổ.
 */
export function FormGiaoViec({
  danhMuc,
  danhBa,
  danhBaLanhDao,
  coDanhSachVanBan = false,
  staffSearch = false,
  taskScreen = false,
  dangGui,
  loi,
  huy,
  giaoViec,
  maChaCoSan,
  tieuDeCoSan,
  dialog = false,
  dialogTitle = "Giao việc mới",
  dialogDescription = MO_TA_FORM_GIAO_VIEC,
  lead,
  submitLabel = "Giao việc",
  initialUnit = "",
  initialAssignee = "",
  inheritedDue,
}: {
  /**
   * Starting values of `Đơn vị thực hiện` / `Người thực hiện` — the Văn bản screen's "Chuyển thành nhiệm
   * vụ" opens on the letter's holding unit and assignee (ADR 0085 §Trả lời #6). Editable, like `tieuDeCoSan`.
   */
  initialUnit?: string;
  initialAssignee?: string;
  /**
   * THE DEADLINE IS THE SERVER'S: drawn READ-ONLY in the deadline box's place, and NO `due_at` is sent.
   * For a route that sets the task's deadline itself and refuses one in the body
   * (`POST /api/v1/citizen-letter-tasks`, ADR 0085 A6). Absent: the usual editable box.
   */
  inheritedDue?: ReactNode;
  /**
   * Open as the prototype's centred DIALOG (`ModalDialog`) instead of inline — the Nhiệm vụ screen's
   * `+ Giao việc mới` and `+ Thêm việc con`, the Biên bản screen's `Tách thành nhiệm vụ` and Phản
   * ánh's `Tạo nhiệm vụ`. Off by default: an inline caller keeps the form in its own frame.
   */
  dialog?: boolean;
  /**
   * Heading, line under it and submit words of the DIALOG — the Biên bản screen's prototype box is
   * `Tách kết luận thành nhiệm vụ` / the conclusion / `Tạo nhiệm vụ`. Words only: the fields and the
   * body sent are this form's, unchanged, so the screens cannot drift on WHAT a task carries.
   */
  dialogTitle?: string;
  dialogDescription?: ReactNode;
  submitLabel?: string;
  /** Lines drawn above the first field (Biên bản: the locked `Nguồn giao` and the "?" hint). */
  lead?: ReactNode;
  /**
   * The staff fields become type-to-search boxes (`TaskPersonPicker`). ONLY the Nhiệm vụ screen
   * sets it; Biên bản keeps its native `<select>` (`OChonCanBo`) — a scope call, not a technical one.
   */
  staffSearch?: boolean;
  /** The Nhiệm vụ screen's spec 06 rules — see the block above. Off for Biên bản and Phản ánh. */
  taskScreen?: boolean;
  danhMuc: DanhMucNhiemVu;
  /**
   * Câu trả lời nguyên vẹn của danh bạ chọn người; `null` = chưa đọc xong. BẮT BUỘC, không tuỳ
   * chọn: bên gọi quên truyền thì `tsc` đỏ, thay vì một form có ba ô chọn rỗng không lời giải thích.
   */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /**
   * Danh bạ ĐÃ LỌC `permission=task.extend` cho riêng ô `Lãnh đạo giao việc` — người ghi ở đó là
   * người duyệt lùi hạn (ADR 0038). Cùng ba pha với `danhBa`. BẮT BUỘC vì cùng lý do.
   */
  danhBaLanhDao: KetQua<identity_danhBaChonNguoiRa> | null;
  /**
   * Vẽ và gửi ba danh sách văn bản §7.2. MẶC ĐỊNH TẮT, VÀ ĐÓ LÀ CHIỀU AN TOÀN: màn Biên bản dùng lại
   * form này để gửi `…/conclusions/{stt}/task`, mà `petitions.tachKetLuanVao` không có `documents`.
   */
  coDanhSachVanBan?: boolean;
  dangGui: boolean;
  /** The server's refusal shown in the form; `null` on the Nhiệm vụ screen (it toasts instead). */
  loi: string | null;
  huy: () => void;
  giaoViec: (than: petitions_taoNhiemVuVao, khoaChongTrung: string) => void;
  /**
   * REGISTER CODE of the parent (`NV19`) when the form is opened by `+ Thêm việc con` (#10) — the
   * contract's `parent` takes a register code since ad7f821. Every invalid parent is the server's
   * 409 `task_tree`, shown verbatim.
   */
  maChaCoSan?: string;
  /**
   * Điền sẵn ô `Nội dung nhiệm vụ` — `04-bien-ban-hop.md` §3, khi biểu mẫu này mở từ một kết luận
   * họp. CHỈ LÀ GIÁ TRỊ BAN ĐẦU, không phải một ô khoá.
   */
  tieuDeCoSan?: string;
}) {
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const [tuSinhMa, datTuSinhMa] = useState(true);
  /**
   * `null` = cán bộ CHƯA ĐỤNG ô ưu tiên → ô theo mức mặc định của xã (`mucUuTienMacDinh`). Tách
   * khỏi `""` có chủ ý: `""` là cán bộ CHỦ ĐỘNG chọn `— Chưa xác định —`, và lựa chọn ấy không được
   * bị mức mặc định đè lại. Danh mục về SAU lúc form mở cũng vì thế vẫn chọn sẵn đúng mức mặc định.
   */
  const [mucUuTienDaChon, datMucUuTien] = useState<string | null>(null);
  const [ma, datMa] = useState("");
  const [loai, datLoai] = useState("");
  const [khoi, datKhoi] = useState("");
  const [tieuDe, datTieuDe] = useState(tieuDeCoSan ?? "");
  const [moTa, datMoTa] = useState("");
  const [boPhan, datBoPhan] = useState(initialUnit);
  const [nguoiThucHien, datNguoiThucHien] = useState(initialAssignee);
  const [lanhDaoGiaoViec, datLanhDaoGiaoViec] = useState("");
  // ADR 0065 NV6: pre-filled +7 calendar days at 17:00, computed ONCE when the form opens (lazy
  // initialiser) — the clerk may change or clear it. ONE `datetime-local` value, as the prototype's
  // box; `han` / `dueTime` are derived from it so the body sent is unchanged (`thanGiaoViec`).
  // An INHERITED deadline keeps it empty: "" is "no deadline in the body", which is what such a route needs.
  const [dueInput, setDueInput] = useState(() => (inheritedDue === undefined ? defaultNewTaskDueInput() : ""));
  // The box half typed: its value reads "" (= no deadline), so this flag is what blocks sending.
  const [dueIncomplete, setDueIncomplete] = useState(false);
  const { date: han, time: dueTime } = splitNewTaskDue(dueInput);
  // Nhiệm vụ: ONE EMPTY ROW PER GROUP from the start, as the prototype (`TaskAssignForm.tsx:90-94`);
  // untouched rows are dropped on send (`usableDocumentRows`). Elsewhere: no row until `+ Thêm văn bản`.
  const [vanBan, datVanBan] = useState<readonly DongVanBanNhap[]>(() =>
    taskScreen ? MOI_NHOM_VAN_BAN.map((nhom, i) => blankDocumentRow(`vb${i + 1}`, nhom)) : [],
  );
  const [ghiChu, datGhiChu] = useState("");
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const [focusRequest, setFocusRequest] = useState(0);
  // Continues after the starting rows' keys, so a key is never reused within one opening.
  const demKhoaVanBan = useRef(taskScreen ? MOI_NHOM_VAN_BAN.length : 0);
  // Id ô cần nhận tiêu điểm SAU lần vẽ kế tiếp: dòng vừa thêm chưa có trong DOM lúc bấm nút.
  const oCanTieuDiem = useRef<string | null>(null);

  useEffect(() => {
    if (oCanTieuDiem.current === null) return;
    document.getElementById(oCanTieuDiem.current)?.focus();
    oCanTieuDiem.current = null;
  }, [vanBan]);

  // THE DEFAULT TYPE IS DERIVED, NOT COPIED INTO STATE: `loai` stays `""` until the clerk picks, and
  // while it is `""` the select shows the default — so a catalogue that arrives AFTER the form opened
  // still pre-selects, and a type the clerk picked is never overwritten by it.
  const typeOptions = taskScreen ? danhMuc.loai.filter((l) => l.active) : danhMuc.loai;
  const loaiChon =
    loai !== "" ? loai : taskScreen ? specDefaultTaskType(danhMuc.loai) : defaultTaskType(danhMuc.loai);
  // §7.1 `Thường (mặc định)` — but "Thường" is a row of the COMMUNE's catalogue (rule 1, invariant 10).
  // Nhiệm vụ: no empty choice (spec 06), so with no default row the FIRST active row is shown and sent.
  const priorityDefault = taskScreen
    ? mucUuTienMacDinh(danhMuc.mucUuTien) || (danhMuc.mucUuTien.find((m) => m.active)?.code ?? "")
    : mucUuTienMacDinh(danhMuc.mucUuTien);
  const mucUuTien = mucUuTienDaChon ?? priorityDefault;
  const theoVanBan = needsDirective(danhMuc.loai, loaiChon);
  const hienVanBan = theoVanBan && coDanhSachVanBan;
  // Nhiệm vụ: the unit and assignee fields belong to the `Theo văn bản` form only (spec 06 §5).
  const showUnitAndAssignee = !taskScreen || theoVanBan;
  // Nhiệm vụ: Khối / Mức ưu tiên offer ACTIVE rows only (prototype `TaskAssignForm.tsx:233-234,
  // 391-392`), keeping the current value so the select never shows another option than it sends.
  const blocOptions = taskScreen ? activeChoices(danhMuc.khoi, khoi) : danhMuc.khoi;
  // What is validated AND sent: Nhiệm vụ drops its all-blank rows silently (`usableDocumentRows`).
  const documentsToSend = taskScreen ? usableDocumentRows(vanBan) : vanBan;
  // Recomputed every render; SHOWN only after the first press of `Giao việc` (`submitAttempted`), so
  // nothing is red before the clerk has tried, and each error clears as its field is fixed.
  const errors = createTaskErrors({
    type: loaiChon,
    directive: theoVanBan,
    title: tieuDe,
    dueInput,
    dueIncomplete,
    documents: documentsToSend,
    documentsShown: hienVanBan,
  });
  const shown = submitAttempted ? errors : null;
  const firstInvalidId = errors.order[0];

  // One refused press = one focus move, to the FIRST invalid field in form order. Keyed on the press
  // count, not on the errors: fixing a field must not drag focus to the next one while typing.
  const focusHandled = useRef(0);
  useEffect(() => {
    if (focusRequest === focusHandled.current) return;
    focusHandled.current = focusRequest;
    if (firstInvalidId === undefined) return;
    // A plain `focus()` scrolls the field into view inside the dialog's scroll area (and the page,
    // for an inline form). No explicit scroll call: it would also scroll the page under the modal.
    document.getElementById(firstInvalidId)?.focus();
  }, [focusRequest, firstInvalidId]);

  const db = docDanhBaChonNguoi(danhBa);
  const dbLanhDao = docDanhBaChonNguoi(danhBaLanhDao);
  // Đọc được mà rỗng: không ai trong xã cầm `task.extend`. Một câu thay cho một ô chọn rỗng.
  const khongAiDuyetDuoc = !dbLanhDao.dangTai && dbLanhDao.loi === null && dbLanhDao.ds.length === 0;
  // Danh bạ nào CÒN ĐANG TẢI thì khoá nút gửi — xem chú thích ở nút `Giao việc`.
  const dangTaiDanhBa = db.dangTai || dbLanhDao.dangTai;

  function themVanBan(nhom: NhomVanBan) {
    demKhoaVanBan.current += 1;
    const khoa = `vb${demKhoaVanBan.current}`;
    oCanTieuDiem.current = `giao-van-ban-${khoa}`;
    datVanBan((ds) => [...ds, blankDocumentRow(khoa, nhom)]);
  }

  function goVanBan(dong: DongVanBanNhap) {
    // Nhiệm vụ: the group's LAST row is CLEARED, not removed, so the box never leaves the form
    // (prototype `TaskReferenceEditor.tsx:120-126`). Focus stays in that row's summary.
    if (taskScreen && dongCuaNhom(vanBan, dong.nhom).length === 1) {
      oCanTieuDiem.current = `giao-van-ban-${dong.khoa}`;
      datVanBan((ds) => ds.map((d) => (d.khoa === dong.khoa ? blankDocumentRow(d.khoa, d.nhom) : d)));
      return;
    }
    // Dòng vừa gỡ mang theo tiêu điểm; trả nó về nút thêm của cùng nhóm thay vì để rơi về đầu trang.
    oCanTieuDiem.current = `giao-them-van-ban-${dong.nhom}`;
    datVanBan((ds) => ds.filter((d) => d.khoa !== dong.khoa));
  }

  function suaVanBan(khoa: string, sua: Partial<Omit<DongVanBanNhap, "khoa" | "nhom">>) {
    datVanBan((ds) => ds.map((d) => (d.khoa === khoa ? { ...d, ...sua } : d)));
  }

  function gui(e: FormEvent) {
    e.preventDefault();
    // The button is disabled for both; Enter in a field still submits, so check again.
    if (dangGui || dangTaiDanhBa) return;
    if (errors.order.length > 0) {
      // Nothing is sent. Errors appear under their fields, and focus moves AFTER that render (the
      // `focusRequest` effect) — the error text and `aria-describedby` must exist when focus lands.
      setSubmitAttempted(true);
      setFocusRequest((n) => n + 1);
      return;
    }

    const than: petitions_taoNhiemVuVao = thanGiaoViec(
      {
        tuSinhMa,
        ma,
        loai: loaiChon,
        khoi,
        tieuDe,
        moTa,
        mucUuTien,
        // A field that is not drawn is not sent (Nhiệm vụ, basic type: no unit / assignee fields).
        boPhan: showUnitAndAssignee ? boPhan : "",
        nguoiThucHien: showUnitAndAssignee ? nguoiThucHien : "",
        lanhDaoGiaoViec,
        han,
        dueTime,
        vanBan: documentsToSend,
        ghiChu,
      },
      { coDanhSachVanBan, directive: theoVanBan, maCha: maChaCoSan },
    );

    giaoViec(than, khoaChongTrung);
  }

  const hasParent = maChaCoSan !== undefined && maChaCoSan !== "";
  const documentGroup = (group: NhomVanBan) => (
    <NhomVanBanNhap
      key={group}
      nhom={group}
      dong={dongCuaNhom(vanBan, group)}
      them={() => themVanBan(group)}
      go={goVanBan}
      sua={suaVanBan}
      rowErrors={shown?.documentRows}
      // ONE full-width box per document, as the prototype (`TaskReferenceEditor.tsx:101-105`, customer
      // sheet row 16): no `Số, ký hiệu`, no `Ngày`. Both are optional on the wire
      // (`petitions_vanBanNhiemVuVao`) and absent when blank (`thanGiaoViec`), so nothing is sent for
      // them. No "?" either: its sentence says "three boxes", which this form no longer draws.
      summaryOnly
    />
  );
  const leaderEmpty = taskScreen ? LEADER_EMPTY_LABEL : CHUA_XAC_DINH;

  // THE PROTOTYPE'S FIELD ORDER (`TaskAssignForm.tsx`): [Loại | Khối] → Mã + Tự sinh mã → Tên → Mô tả →
  // (đơn vị → người) → Lãnh đạo giao việc → (văn bản cấp trên, chỉ đạo Đảng uỷ) → [Hạn | Mức ưu tiên]
  // → (Kết quả đầu ra, Ghi chú) → Huỷ / Giao việc. The order follows the columns of the commune's own
  // tracking book, so a clerk copying from the Excel file reads straight down.
  const fields = (
    <>
      {lead}
      {hasParent && <p className={HINT_CLASS}>{childFormNote(maChaCoSan)}</p>}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label htmlFor={CREATE_TASK_FIELD_IDS.type} className={LABEL_CLASS}>
            Loại nhiệm vụ
          </label>
          {/* Nhiệm vụ: the `is_default` row, else the first (#2). Elsewhere: NO default row → the
              empty option `— Chọn loại —`, never the first row (tester report NV-01). */}
          <select
            id={CREATE_TASK_FIELD_IDS.type}
            value={loaiChon}
            className={FORM_SELECT_CLASS}
            {...fieldErrorProps(CREATE_TASK_FIELD_IDS.type, shown?.type)}
            onChange={(e) => datLoai(e.target.value)}
          >
            {loaiChon === "" && <option value="">{TASK_TYPE_PLACEHOLDER}</option>}
            {typeOptions.map((l) => (
              <option key={l.code} value={l.code}>
                {l.label}
              </option>
            ))}
          </select>
          <FieldError forId={CREATE_TASK_FIELD_IDS.type} message={shown?.type} />
        </div>

        <div>
          <label htmlFor="giao-khoi" className={LABEL_CLASS}>
            Khối nhiệm vụ
          </label>
          <select id="giao-khoi" value={khoi} className={FORM_SELECT_CLASS} onChange={(e) => datKhoi(e.target.value)}>
            <option value="">{CHUA_XAC_DINH}</option>
            {blocOptions.map((k) => (
              <option key={k.code} value={k.code}>
                {k.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Mã + `☐ Tự sinh mã` on ONE row, the code box disabled while the server numbers it. The
          box is EMPTY while ticked, so a code typed earlier is never shown as if it would be sent —
          `thanGiaoViec` drops it whenever `tuSinhMa` is on. */}
      <div>
        <label htmlFor="giao-ma" className={LABEL_CLASS}>
          Mã nhiệm vụ
        </label>
        <div className="flex items-center gap-3">
          <input
            id="giao-ma"
            name="giao-ma"
            value={tuSinhMa ? "" : ma}
            disabled={tuSinhMa}
            placeholder={tuSinhMa ? "Hệ thống sẽ tự sinh" : "NV01"}
            autoComplete="off"
            className={cn(INPUT_CLASS, "flex-1")}
            onChange={(e) => datMa(e.target.value)}
          />
          <label htmlFor="giao-tu-sinh-ma" className="text-ink m-0 flex shrink-0 items-center gap-2 text-[12.5px]">
            <input
              id="giao-tu-sinh-ma"
              type="checkbox"
              className={CHECKBOX_CLASS}
              checked={tuSinhMa}
              onChange={(e) => datTuSinhMa(e.target.checked)}
            />
            Tự sinh mã
          </label>
        </div>
        <p className={HINT_CLASS}>{GHI_CHU_TU_SINH_MA}</p>
      </div>

      <div>
        <label htmlFor={CREATE_TASK_FIELD_IDS.title} className={LABEL_CLASS}>
          {nhanOTieuDe(theoVanBan)}
          <span className="text-danger ml-1" aria-hidden="true">
            *
          </span>
        </label>
        <input
          id={CREATE_TASK_FIELD_IDS.title}
          name="giao-tieu-de"
          value={tieuDe}
          required
          autoComplete="off"
          className={INPUT_CLASS}
          {...fieldErrorProps(CREATE_TASK_FIELD_IDS.title, shown?.title)}
          onChange={(e) => datTieuDe(e.target.value)}
        />
        <FieldError forId={CREATE_TASK_FIELD_IDS.title} message={shown?.title} />
      </div>

      <div>
        <label htmlFor="giao-mo-ta" className={LABEL_CLASS}>
          Mô tả
        </label>
        <textarea
          id="giao-mo-ta"
          name="giao-mo-ta"
          rows={3}
          value={moTa}
          className={TEXTAREA_CLASS}
          onChange={(e) => datMoTa(e.target.value)}
        />
      </div>

      {/* ĐƠN VỊ → NGƯỜI THỰC HIỆN. Under `Theo văn bản` the same two fields carry the names the
          commune's tracking book gives them — "cơ quan chủ trì tham mưu" IS the unit and "chuyên viên
          tham mưu / theo dõi" IS the assignee (ADR 0065 NV5). One field, two names. */}
      {showUnitAndAssignee && (
        <>
          <div>
            <label htmlFor="giao-bo-phan" className={LABEL_CLASS}>
              {theoVanBan ? "Cơ quan chủ trì tham mưu (cơ quan thực hiện)" : "Đơn vị thực hiện"}
            </label>
            <select id="giao-bo-phan" value={boPhan} className={FORM_SELECT_CLASS} onChange={(e) => datBoPhan(e.target.value)}>
              <option value="">{theoVanBan ? "— Chọn cơ quan —" : CHUA_XAC_DINH}</option>
              {danhMuc.boPhan.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </div>

          {/* TỪ DANH BẠ CHỌN NGƯỜI (`GET /api/v1/staff-directory`). Giá trị là MÃ NGHIỆP VỤ `CB-…`
              (`code`) (luật 6, bất biến 8). Danh bạ đọc hỏng thì ô chỉ còn lựa chọn trống và câu lỗi
              nói hệ quả — không đoán ai. */}
          {db.loi !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {cauLoiDanhBaGiaoViec(db.loi)}
            </p>
          )}
          <StaffPicker
            search={staffSearch}
            id="giao-nguoi-thuc-hien"
            label={theoVanBan ? "Chuyên viên tham mưu / theo dõi (người thực hiện)" : "Người thực hiện"}
            // Nhiệm vụ: the prototype `PersonPicker`'s empty line (`common/PersonPicker.tsx:31`).
            emptyLabel={nhanTrongOChonCanBo(db, taskScreen ? ASSIGNEE_EMPTY_LABEL : DE_BO_PHAN_TU_PHAN_CONG)}
            value={nguoiThucHien}
            directory={db.ds}
            disabled={db.dangTai}
            onChange={datNguoiThucHien}
            hint={
              theoVanBan ? (
                <p className={HINT_CLASS}>
                  Người này là người thực hiện chính: nhiệm vụ hiện trong mục “Giao cho tôi” của họ ngay
                  khi lưu.
                </p>
              ) : undefined
            }
          />
        </>
      )}

      {/* LÃNH ĐẠO GIAO VIỆC — CHỈ NGƯỜI CẦM `task.extend` (`danhBaLanhDao`). Ba ca có chữ: đọc hỏng
          (câu máy chủ nguyên văn), đọc được mà rỗng (không ai cầm quyền — một câu, không một ô rỗng),
          còn lại là ô chọn. */}
      <div>
        {dbLanhDao.loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {cauLoiDanhBaLanhDao(dbLanhDao.loi)}
          </p>
        )}
        {khongAiDuyetDuoc ? (
          <div>
            {/* A span, not a label (nothing to point at) — but drawn at the label size. */}
            <span className={LABEL_CLASS}>Lãnh đạo giao việc</span>
            <p className={HINT_CLASS} id="giao-lanh-dao-trong">
              {CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN}
            </p>
          </div>
        ) : (
          // `dbLanhDao` — the `?permission=task.extend` directory, so typing can only ever find an
          // approver (ADR 0038). The whole-commune `db` must never feed this box.
          <StaffPicker
            search={staffSearch}
            id="giao-lanh-dao"
            label="Lãnh đạo giao việc"
            emptyLabel={nhanTrongOChonCanBo(dbLanhDao, leaderEmpty)}
            value={lanhDaoGiaoViec}
            directory={dbLanhDao.ds}
            disabled={dbLanhDao.dangTai}
            onChange={datLanhDaoGiaoViec}
            hint={
              // Nhiệm vụ: the spec's sentence verbatim (#2), "qua chuông và qua thư" included. Elsewhere:
              // the consequence the server holds — left empty, NOBODY can approve an extension (ADR 0038),
              // and `PATCH` cannot set this column afterwards.
              <p className={HINT_CLASS}>
                {taskScreen
                  ? LEADER_HINT_SPEC
                  : `${GHI_CHU_LANH_DAO_GIAO_VIEC} Bỏ trống thì không ai duyệt được đề nghị lùi hạn của nhiệm vụ này, và ô này không sửa lại được sau khi tạo.`}
              </p>
            }
          />
        )}
      </div>

      {/* Văn bản cấp trên giao + chỉ đạo của Đảng uỷ come BEFORE the deadline; the output group comes
          after it, beside `Ghi chú` — the prototype's order, which is the tracking book's columns. */}
      {hienVanBan && (
        <div className="space-y-3">
          {/* Too many rows is no single row's fault: the sentence stands above the lists and takes
              the focus itself (`tabIndex={-1}`). */}
          {shown?.documentLimit != null && (
            <p id={CREATE_TASK_FIELD_IDS.documentLimit} tabIndex={-1} className={FIELD_ERROR_CLASS}>
              {shown.documentLimit}
            </p>
          )}
          {MOI_NHOM_VAN_BAN.filter((g) => g !== "san-pham-dau-ra").map((g) => documentGroup(g))}
        </div>
      )}

      {/* [Hạn hoàn thành | Mức ưu tiên], halves, ONE `datetime-local` box — the prototype's row. Date
          AND time: a deadline "tomorrow" read as 00:00 counts work done tomorrow as late. */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {inheritedDue !== undefined ? (
          <div data-inherited-due="">{inheritedDue}</div>
        ) : (
          <div>
            <label htmlFor={CREATE_TASK_FIELD_IDS.due} className={LABEL_CLASS}>
              Hạn hoàn thành
            </label>
            {/* `dd/mm/yyyy hh:mm` in every browser language, the native calendar one click away
                (customer sheet row 16) — `DueDateInput`. Same value in and out as the native box. */}
            <DueDateInput
              id={CREATE_TASK_FIELD_IDS.due}
              name="giao-han"
              value={dueInput}
              invalidProps={fieldErrorProps(CREATE_TASK_FIELD_IDS.due, shown?.due)}
              onChange={(value, incomplete) => {
                setDueInput(value);
                setDueIncomplete(incomplete);
              }}
            />
            <FieldError forId={CREATE_TASK_FIELD_IDS.due} message={shown?.due} />
          </div>
        )}

        <div>
          <label htmlFor="giao-uu-tien" className={LABEL_CLASS}>
            Mức ưu tiên
          </label>
          <select id="giao-uu-tien" value={mucUuTien} className={FORM_SELECT_CLASS} onChange={(e) => datMucUuTien(e.target.value)}>
            {/* KHÔNG SẮP XẾP LẠI: thứ tự `items` LÀ thang bậc của xã. Nhiệm vụ: no empty choice (spec 06). */}
            {(!taskScreen || mucUuTien === "") && <option value="">{CHUA_XAC_DINH}</option>}
            {(taskScreen ? activeChoices(danhMuc.mucUuTien, mucUuTien) : danhMuc.mucUuTien).map((m) => (
              <option key={m.code} value={m.code}>
                {m.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {hienVanBan && (
        <div className="space-y-3">
          {documentGroup("san-pham-dau-ra")}
          {/* Cùng cổng `hienVanBan`: §7.3 bỏ ô này ở loại khác, và màn Biên bản gửi tới tuyến không
              có `note`. */}
          <div>
            <label htmlFor="giao-ghi-chu" className={LABEL_CLASS}>
              Ghi chú
            </label>
            <textarea
              id="giao-ghi-chu"
              name="giao-ghi-chu"
              rows={2}
              maxLength={GHI_CHU_NHIEM_VU_TOI_DA}
              value={ghiChu}
              className={TEXTAREA_CLASS}
              onChange={(e) => datGhiChu(e.target.value)}
            />
          </div>
        </div>
      )}

      {/* Field errors stand under their fields (`createTaskErrors`); a SERVER refusal is here, beside
          the buttons — for the screens that pass one (Nhiệm vụ toasts it instead). */}
      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
    </>
  );

  // Biên bản / Phản ánh keep their own button pair: their screens are not part of the Nhiệm vụ
  // spec, and their tests pin these exact buttons.
  const buttons = !taskScreen ? (
    <div className="cum-nut justify-end">
      <button type="button" className="nut-phu" onClick={huy} disabled={dangGui}>
        Huỷ
      </button>
      {/* Locked only while the directory loads (see below); missing input is refused in `gui`. */}
      <button type="submit" className="nut-chinh" disabled={dangGui || dangTaiDanhBa}>
        {submitLabel}
      </button>
    </div>
  ) : (
    <div className="flex justify-end gap-2 pt-1">
      <Button type="button" variant="outline" onClick={huy} disabled={dangGui}>
        Huỷ
      </Button>
      <Button
        type="submit"
        variant="primary"
        // Danh bạ CÒN ĐANG TẢI thì khoá: ba ô chọn chưa chọn được ai, và một nhiệm vụ tạo ra lúc
        // ấy mang `Lãnh đạo giao việc` rỗng VĨNH VIỄN (`PATCH` không sửa cột ấy). Tải HỎNG thì
        // không khoá — câu lỗi ngay trên nói hệ quả, và việc giao cho bộ phận vẫn phải làm được.
        // MISSING OR INVALID INPUT DOES NOT DISABLE IT (as the prototype): the press is refused in
        // `gui`, which shows each error under its field and moves focus to the first.
        disabled={dangGui || dangTaiDanhBa}
        aria-busy={dangGui || undefined}
        icon={dangGui ? <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" /> : undefined}
      >
        {submitLabel}
      </Button>
    </div>
  );

  if (!dialog) {
    // Inline callers: the same fields in the same order, in the screen's own frame.
    return (
      <form className="m-0 flex flex-col gap-4 [&>*]:my-0" noValidate onSubmit={gui}>
        <div className="[&>*]:my-0">
          <h4 className="text-navy m-0 text-base font-semibold">Giao việc mới</h4>
          <p className={HINT_CLASS}>{MO_TA_FORM_GIAO_VIEC}</p>
        </div>
        {fields}
        {buttons}
      </form>
    );
  }

  // THE PROTOTYPE'S DIALOG (`TaskAssignForm.tsx:195-197`): 500px, 800px when the three document
  // lists are drawn — `Theo văn bản` on a screen that sends them (Nhiệm vụ, Phản ánh; owner 07/10/2026
  // #9). Biên bản has nothing to widen for. The header stays in sight; the fields AND `Huỷ / Giao
  // việc` scroll under it, as in the prototype. Esc asks `huy`, as `Huỷ` does.
  return (
    <ModalDialog
      titleId={CREATE_TASK_TITLE_ID}
      size={hienVanBan ? "lg" : "md"}
      className={CREATE_TASK_DIALOG_CLASS}
      onDismiss={huy}
    >
      <ModalDialogHeader
        titleId={CREATE_TASK_TITLE_ID}
        title={dialogTitle}
        description={dialogDescription}
      />
      {/* `noValidate`: the browser's own bubble would answer the `required` fields before `gui`
          runs, in its words and only for the first — `createTaskErrors` covers every rule. */}
      <form className="m-0 flex min-h-0 flex-col" noValidate onSubmit={gui}>
        <div className={CREATE_TASK_BODY_CLASS}>
          {fields}
          {buttons}
        </div>
      </form>
    </ModalDialog>
  );
}

/** Spec 06: the empty line of `Lãnh đạo giao việc` on the Nhiệm vụ screen (owner 07/10/2026 #2). */
export const LEADER_EMPTY_LABEL = "— Người đang tạo nhiệm vụ —";
/** The empty line of `Chuyên viên tham mưu / theo dõi` on the Nhiệm vụ screen — prototype `PersonPicker`. */
export const ASSIGNEE_EMPTY_LABEL = "— Chưa phân công —";
/** Spec 06: its hint, verbatim (#2) — the screen promises a bell and a mail (ADR 0076 lần 2 (b)). */
export const LEADER_HINT_SPEC = "Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư.";

/**
 * The create dialog's box, over `ModalDialog`'s defaults: `p-4` (the prototype's `DialogContent`
 * padding) and at least 48px clear above and below at every height. The box sits centred because
 * `ModalDialog` itself carries `m-auto!` (the page's `[&>*]:my-0` once pinned it to the top edge).
 */
export const CREATE_TASK_DIALOG_CLASS = "max-h-[calc(100dvh-6rem)] p-4";

/**
 * The scrolling area under the header: the fields, then `Huỷ / Giao việc` at its END (prototype
 * `max-h-[70vh] space-y-4 overflow-y-auto pr-1`).
 *
 * `[&>*]:shrink-0`: this area is a height-capped flex COLUMN, so once its content is taller than the
 * cap the browser SHRINKS its children before it scrolls — a `.thong-bao-loi` was squeezed to its
 * `min-height` and its sentence spilt out of the frame (user screenshot 07/10/2026). Children scroll.
 */
export const CREATE_TASK_BODY_CLASS =
  "flex max-h-[70vh] min-h-0 flex-col gap-4 overflow-y-auto pr-1 [&>*]:my-0 [&>*]:shrink-0 [&_[aria-invalid=true]]:border-danger";

/** A field's error line, under the field — the prototype's `Field` error (12px, medium, danger). */
const FIELD_ERROR_CLASS = "text-danger m-0 mt-1.5 text-[12px] font-medium";

/** Id of the error line of the field `forId`. */
function fieldErrorId(forId: string): string {
  return `${forId}-loi`;
}

/** `aria-invalid` + `aria-describedby` of a field that has an error shown; nothing otherwise. */
function fieldErrorProps(
  forId: string,
  message: string | null | undefined,
): { "aria-invalid"?: true; "aria-describedby"?: string } {
  return message == null ? {} : { "aria-invalid": true, "aria-describedby": fieldErrorId(forId) };
}

/**
 * NO `role="alert"`, unlike the prototype: focus moves to the first invalid field, whose
 * `aria-describedby` reads this line — an alert as well would read it twice, and would read every
 * OTHER field's error at the same moment.
 */
function FieldError({ forId, message }: { forId: string; message: string | null | undefined }) {
  if (message == null) return null;
  return (
    <p id={fieldErrorId(forId)} className={FIELD_ERROR_CLASS}>
      {message}
    </p>
  );
}

/** Id of the create dialog's heading — its accessible name. */
export const CREATE_TASK_TITLE_ID = "tieu-de-giao-viec-moi";

/**
 * The form's staff field: the spec 06 `TaskPersonPicker` when `search` (Nhiệm vụ), otherwise the
 * native `OChonCanBo` the Biên bản screen still uses. Same props either way.
 */
function StaffPicker({
  search,
  id,
  label,
  emptyLabel,
  value,
  directory,
  disabled,
  hint,
  onChange,
}: {
  search: boolean;
  id: string;
  label: string;
  emptyLabel: string;
  value: string;
  directory: readonly identity_canBoChonNguoiRa[];
  disabled: boolean;
  hint?: ReactNode;
  onChange: (code: string) => void;
}) {
  return search ? (
    <TaskPersonPicker
      id={id}
      label={label}
      labelClassName={LABEL_CLASS}
      emptyLabel={emptyLabel}
      value={value}
      directory={directory}
      disabled={disabled}
      hint={hint}
      onChange={onChange}
    />
  ) : (
    <div>
      <OChonCanBo
        id={id}
        nhan={label}
        nhanTrong={emptyLabel}
        giaTri={value}
        danhBa={directory}
        khoa={disabled}
        dat={onChange}
      />
      {hint}
    </div>
  );
}

/** One free-text box per document needs the server to parse the reference (owner 07/10/2026 #11). */
export const SINGLE_REFERENCE_BOX_PENDING = {
  ten: "Một ô văn bản tự do",
  viSao:
    "Bản thiết kế nhập mỗi văn bản trong MỘT ô chữ (số, ngày, trích yếu viết liền). Máy chủ chưa tách " +
    "được số và ngày từ câu ấy, nên mỗi văn bản vẫn nhập ba ô: trích yếu, số ký hiệu, ngày.",
} as const;

/**
 * Một trong ba danh sách văn bản động của §7.2 — spec 06 `TaskReferenceEditor`: the group label, the
 * rows, a ghost `+ Thêm văn bản`.
 *
 * MỖI DÒNG LÀ MỘT Ô TRÍCH YẾU BẮT BUỘC VÀ HAI Ô NHỎ TUỲ CHỌN (`Số, ký hiệu`, `Ngày văn bản`) — the
 * prototype's single box needs the server to parse a reference (BACKEND DEPENDENCY, `pending` draws
 * its "?" once per form). Không tách số và ngày ra khỏi câu trích yếu bằng máy: đoán số hiệu từ một
 * câu tiếng Việt là in ra một số văn bản không tồn tại (`nhiem_vu_van_ban.go:77-80`).
 *
 * `role="group"` + tiêu đề thay cho `<fieldset>`: `fieldset` mặc định rộng tối thiểu bằng nội dung,
 * và ở 320px một placeholder dài đẩy nó tràn ngang.
 */
function NhomVanBanNhap({
  tienTo = "giao",
  nhom,
  dong,
  them,
  go,
  sua,
  rowErrors,
  pending = false,
  summaryOnly = false,
}: {
  /** The summary box alone, full width — the create form (see its `documentGroup`). */
  summaryOnly?: boolean;
  /**
   * Errors to draw under a row, by row key — the create form after a refused press
   * (`createTaskErrors`). Absent on the `Sửa` form, which keeps its own sentence (`canhBaoSua`).
   */
  rowErrors?: ReadonlyMap<string, string>;
  /**
   * Tiền tố của mọi `id` trong nhóm. Form tạo và form `Sửa` có thể CÙNG MỞ trên một trang; chung
   * tiền tố thì hai ô mang cùng `id`, và `<label htmlFor>` trỏ nhầm sang ô của form kia.
   */
  tienTo?: "giao" | "sua";
  nhom: NhomVanBan;
  dong: readonly DongVanBanNhap[];
  them: () => void;
  go: (dong: DongVanBanNhap) => void;
  sua: (khoa: string, sua: Partial<Omit<DongVanBanNhap, "khoa" | "nhom">>) => void;
  /** Draw the "?" of the single free-text box beside this group's label. */
  pending?: boolean;
}) {
  const idTieuDe = `${tienTo}-nhom-van-ban-${nhom}`;
  return (
    <div role="group" aria-labelledby={idTieuDe}>
      <p className="text-ink m-0 mb-1 flex items-center gap-1.5 text-[12.5px] font-medium">
        <span id={idTieuDe}>{nhanNhomVanBan(nhom)}</span>
        {pending && <PendingMarker info={SINGLE_REFERENCE_BOX_PENDING} />}
      </p>
      <div className="space-y-2">
        {dong.map((d, i) => {
          const id = `${tienTo}-van-ban-${d.khoa}`;
          const rowError = rowErrors?.get(d.khoa);
          return (
            <div
              key={d.khoa}
              role="group"
              aria-label={`${nhanNhomVanBan(nhom)} — văn bản thứ ${i + 1}`}
              className="flex items-start gap-1.5"
            >
              <div
                className={cn(
                  "grid min-w-0 flex-1 grid-cols-1 gap-1.5",
                  !summaryOnly && "sm:grid-cols-[1fr_9rem_9rem]",
                )}
              >
                <div className="min-w-0">
                  <label htmlFor={id} className="an-thi-giac">
                    Trích yếu (văn bản thứ {i + 1})
                  </label>
                  {/* The summary is the row's one required box, so its error sits under it and focus
                      lands in it. */}
                  <textarea
                    id={id}
                    name={id}
                    rows={2}
                    required
                    maxLength={TRICH_YEU_VAN_BAN_TOI_DA}
                    placeholder={placeholderNhomVanBan(nhom)}
                    value={d.trichYeu}
                    className={cn(TEXTAREA_CLASS, "resize-y")}
                    {...fieldErrorProps(id, rowError)}
                    onChange={(e) => sua(d.khoa, { trichYeu: e.target.value })}
                  />
                  <FieldError forId={id} message={rowError} />
                </div>
                {!summaryOnly && (
                <div className="min-w-0">
                  <label htmlFor={`${id}-so`} className="an-thi-giac">
                    Số, ký hiệu (không bắt buộc)
                  </label>
                  <input
                    id={`${id}-so`}
                    name={`${id}-so`}
                    maxLength={SO_KY_HIEU_VAN_BAN_TOI_DA}
                    autoComplete="off"
                    placeholder="Số, ký hiệu"
                    value={d.soKyHieu}
                    className={INPUT_CLASS}
                    onChange={(e) => sua(d.khoa, { soKyHieu: e.target.value })}
                  />
                </div>
                )}
                {!summaryOnly && (
                <div className="min-w-0">
                  <label htmlFor={`${id}-ngay`} className="an-thi-giac">
                    Ngày văn bản (không bắt buộc)
                  </label>
                  <input
                    id={`${id}-ngay`}
                    name={`${id}-ngay`}
                    type="date"
                    value={d.ngay}
                    className={cn(INPUT_CLASS, "px-2")}
                    onChange={(e) => sua(d.khoa, { ngay: e.target.value })}
                  />
                </div>
                )}
              </div>
              <Button
                type="button"
                variant="icon"
                size="lg"
                aria-label={nhanNutGoVanBan(nhom, i + 1)}
                title="Bỏ dòng này"
                onClick={() => go(d)}
              >
                <X aria-hidden="true" focusable="false" className="size-4" />
              </Button>
            </div>
          );
        })}
      </div>
      <Button
        type="button"
        id={`${tienTo}-them-van-ban-${nhom}`}
        variant="ghost"
        size="sm"
        className="mt-1"
        icon={<Plus aria-hidden="true" focusable="false" className="size-3.5" />}
        aria-label={`${NHAN_THEM_VAN_BAN} vào nhóm ${nhanNhomVanBan(nhom)}`}
        onClick={them}
      >
        {NHAN_THEM_VAN_BAN.replace(/^\+\s*/, "")}
      </Button>
    </div>
  );
}
