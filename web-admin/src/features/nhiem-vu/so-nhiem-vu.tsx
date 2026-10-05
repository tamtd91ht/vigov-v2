"use client";

import {
  AlarmClock,
  BookOpen,
  Building2,
  ChevronLeft,
  ChevronRight,
  CircleDot,
  ClipboardList,
  Clock,
  CloudOff,
  Download,
  Eye,
  FileText,
  Flag,
  GitBranch,
  History,
  Landmark,
  Layers,
  Link2,
  List,
  ListChecks,
  Paperclip,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  SearchX,
  Shapes,
  SquareKanban,
  Trash2,
  Upload,
  UserRound,
  Workflow,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import {
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  useSyncExternalStore,
  type Dispatch,
  type FormEvent,
  type KeyboardEvent,
  type ReactNode,
  type SetStateAction,
} from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { OChonCanBo } from "@/components/o-chon-can-bo";
import { StaffCombobox } from "@/components/staff-combobox";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardFooter } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { FilterBar } from "@/components/ui/filter-bar";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { RecordTabStrip, recordTabDomId } from "@/components/ui/record-tabs";
import {
  activateRecordTab,
  closeAllRecordTabs,
  closeRecordTab,
  emptyRecordTabs,
  mergeRecordTabs,
  openRecordTab,
  readStoredRecordTabs,
  renameRecordTab,
  updateRecordTab,
  writeStoredRecordTabs,
  type RecordTab,
  type RecordTabsState,
} from "@/components/ui/record-tabs-state";
import { PageHeader } from "@/components/ui/page-header";
import { PendingFeature } from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";
import { Tooltip } from "@/components/ui/tooltip";
import { cn } from "@/lib/cn";
import { duongDanBienBan } from "@/features/bien-ban/nhan-bien-ban";
import { nhanThoiDiem, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import { layLichLamViec } from "@/lib/api/lich-lam-viec";
import {
  deNghiLuiHan,
  doiTrangThaiNhiemVu,
  downloadTaskRegister,
  getTaskCounts,
  layNhiemVu,
  laySoNhiemVu,
  quyetDinhLuiHan,
  reassignTask,
  suaNhiemVu,
  taoNhiemVu,
  xoaNhiemVu,
  type CotSapXepNhiemVu,
  type LocNhiemVu,
} from "@/lib/api/nhiem-vu";
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";
import { NO_DRILL_DOWN, type DrillDown } from "@/lib/drill-down";
import { QUYEN_DUYET_GIA_HAN } from "@/lib/quyen";
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
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
  petitions_taskAssignmentIn,
  petitions_taskCountsOut,
} from "@/lib/api/schema.gen";

import {
  ADD_CHILD_BUTTON,
  cardHolderText,
  CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN,
  CAU_LOC_TRANG_THAI_KHONG_CO_COT,
  CAU_THIEU_QUYEN_DUYET_HOAN_THANH,
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
  GHI_CHU_HAN_VIEC_CON,
  GHI_CHU_NHIEM_VU_TOI_DA,
  GHI_CHU_LANH_DAO_GIAO_VIEC,
  GHI_CHU_LUI_HAN,
  GHI_CHU_TU_SINH_MA,
  KANBAN_COUNTS_ERROR,
  kanbanSharedError,
  KHONG_DOC_DUOC_VAN_BAN,
  LY_DO_TRA_LAI_TOI_DA,
  MOI_BO_PHAN_NHAN,
  MOI_KHOI_NHAN,
  MOI_LOAI_NHAN,
  MOI_MUC_UU_TIEN_NHAN,
  MOI_NGUOI_THUC_HIEN_NHAN,
  MOI_NGUON_GIAO,
  MOI_NGUON_GIAO_NHAN,
  MOI_NHOM_VAN_BAN,
  MOI_TRANG_THAI_NHAN,
  MO_TA_FORM_GIAO_VIEC,
  NHAN_CHE_DO_DANH_SACH,
  NHAN_CHE_DO_KANBAN,
  NHAN_COT_MA,
  NHAN_COT_NGAY_GIAO,
  NHAN_LY_DO_TRA_LAI,
  NHAN_NUT_TRA_LAI,
  NHAN_NUT_HUY,
  PARENT_NONE,
  PARENT_TITLE,
  NHAN_NUT_LUU,
  NHAN_THEM_VAN_BAN,
  NHAN_TIEU_DE_THEO_VAN_BAN,
  NO_DEADLINE_LAST_NOTE,
  NO_PRIORITY_LAST_NOTE,
  PRIORITY_COLUMN_LABEL,
  TITLE_COLUMN_LABEL,
  O_TRONG,
  PHAM_VI_CUA_TOI,
  SCOPE_RELATED_LABEL,
  SCOPE_RELATED_NOTE,
  DUE_SOON_FILTER_LABEL,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  SO_KY_HIEU_VAN_BAN_TOI_DA,
  SO_RONG,
  TIEU_DE_KHOI_VAN_BAN,
  TIEU_DE_NHIEM_VU_TOI_DA,
  TIM_PLACEHOLDER,
  TOM_TAT_KET_QUA_TOI_DA,
  TRANG_THAI_CHINH,
  TRANG_THAI_RE_NHANH,
  TRICH_YEU_VAN_BAN_TOI_DA,
  ariaSapXep,
  bamCotSapXep,
  canDocLaiTruocKhiLuu,
  cauLoiDanhBaGiaoViec,
  cauLoiDanhBaLanhDao,
  cauLoiDanhBaLoc,
  canhBaoSua,
  canhBaoVanBan,
  cauGiaiThichTrangThai,
  childCountLabel,
  childCreatedText,
  childFormNote,
  kanbanColumnCount,
  kanbanPartialNote,
  danhBaChoNhatKy,
  cauTuKetLuan,
  chiaNhomVanBan,
  canMoveTask,
  changedSince,
  defaultDueTime,
  defaultNewTaskDue,
  NEW_TASK_DUE_PREFILLED_NOTE,
  NEW_TASK_DUE_LATER_NOTE,
  newTaskDueProblem,
  DUE_EDIT_BUTTON,
  DUE_EDIT_NOTE,
  DUE_SET_BUTTON,
  dueTimeHint,
  EXTENSION_NO_DUE,
  TASK_CHANGED_NOTE,
  TASK_PROGRESS_PENDING,
  TASK_TYPE_MISSING,
  TASK_TYPE_PLACEHOLDER,
  type CalendarLoad,
  canWriteLogEntry,
  clickableTransitions,
  lacksApprovalFor,
  reasonMove,
  reopenNote,
  REOPEN_BUTTON,
  REOPEN_REASON_LABEL,
  kanbanDropHint,
  kanbanMoveDoneText,
  kanbanMovePendingText,
  kanbanMoveRefusedPrefix,
  coKhoiVanBanChiDao,
  cotPhaiDoc,
  docBangNhanTrangThai,
  docDanhBaChonNguoi,
  dongCuaNhom,
  dongVanBan,
  duongDanTuLoc,
  formSuaTuChiTiet,
  locTuDuongDan,
  mucUuTienMacDinh,
  nhanCanBoDrawer,
  nhanCanBoNgan,
  quyenNhiemVu,
  ghiChuKanbanReNhanh,
  ghiChuTraLai,
  loiSauKhiDocLai,
  lyDoKhoaSua,
  yeuCauTraLai,
  hoanThanhTreHan,
  mocCuoiNgay,
  nhanBoDem,
  nhanHanThe,
  nhanHoanThanhTreHan,
  nhanNgay,
  nhanNguonGiao,
  nhanNhomVanBan,
  nhanNutGoVanBan,
  nhanOTieuDe,
  nhanTrangThai,
  nhanTrongOChonCanBo,
  oHan,
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
  type ReasonMove,
  type TrangThaiNhiemVu,
} from "./nhan-nhiem-vu";
import { BatchDeleteBar } from "./batch-delete-bar";
import {
  EMPTY_SELECTION,
  keepFailed,
  runBatchDelete,
  selectLabel,
  toggleSelected,
  type BatchDeleteResult,
  type TaskSelection,
  type TaskSelectionState,
} from "./batch-delete";
import { ChildTasks, ParentTaskField } from "./child-tasks";
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
  REGISTER_EXPORT_PENDING,
  REGISTER_EXPORT_REFUSED,
  REGISTER_UNKNOWN_GROUP,
  REGISTER_VIEW_LABEL,
  registerExportDoneText,
  registerRowDocuments,
} from "./task-register";
import { HangChoLuiHan } from "./hang-cho-lui-han";
import { taskDetailHref } from "./task-link";
import { KanbanMoveMenu } from "./kanban-move-menu";
import { NhatKyNhiemVu } from "./nhat-ky-nhiem-vu";
import {
  ASSIGNMENT_STEPPER_HINT,
  ASSIGNMENT_UNIT_FIELD_ID,
  canShowAssignment,
} from "./task-assignment";
import { TaskAssignmentBlock } from "./task-assignment-block";
import { readTaskParam, searchWithTask, useTaskDialogUrl } from "./task-dialog-url";
import { TaskExtensionBlock } from "./task-extension-block";
import { isTaskTabId, parseTaskTabData, taskTabsStorageKey, type TaskTabData } from "./task-tabs";
import {
  Glyph,
  LoadingBar,
  TaskRowsSkeleton,
  TaskStatusBadge,
  taskStatusIcon,
  TOGGLE_TRACK,
  toggleButtonClass,
} from "./task-ui";

/**
 * The list and register tables inside the list card (spec §6.7, v2 §8.1): flush with the card (no
 * second frame), their OWN scroller both ways so the header row can stay put (`sticky` inside
 * the scroller), 48px rows, thin horizontal rules only — `.bang-danh-muc` already draws those.
 */
const TABLE_IN_CARD = cn(
  "bang-cuon max-h-[70vh] overflow-auto rounded-none border-0 shadow-none",
  "[&_thead_th]:sticky [&_thead_th]:top-0 [&_thead_th]:z-[1] [&_thead_th]:shadow-[inset_0_-1px_0_var(--line)]",
  "[&_tbody_td]:h-12",
);

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

/** Bao nhiêu dòng một trang. */
const SO_DONG_MOI_TRANG = 20;

/**
 * Bao nhiêu thẻ đọc cho MỘT cột Kanban.
 *
 * Kanban KHÔNG có phân trang từng cột — năm ngăn xếp con trỏ song song là năm chỗ để lạc, và đặc
 * tả không vẽ nút trang nào trên bảng. Đầu cột hiện TỔNG THẬT (`/task-counts`); cột có nhiều hơn số
 * thẻ đang hiện thì dưới cột nói ra (`kanbanPartialNote`) và cán bộ sang Danh sách.
 */
const SO_THE_MOI_COT = 20;

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

/** Bộ lọc đang chọn trên màn hình. Cùng hình dạng với `LocNhiemVu`, trừ phân trang. */
type BoLoc = Omit<LocNhiemVu, "limit" | "cursor">;

const KHONG_LOC: BoLoc = {};

/**
 * Bấm tiêu đề một cột sắp được của bảng §4.2 → bộ lọc mới VÀ ngăn xếp con trỏ mới.
 *
 * TRẢ CẢ HAI, KHÔNG CHỈ BỘ LỌC, vì hai thứ ấy không tách được: con trỏ mang `sort`/`order` bên trong
 * và máy chủ trả 400 `invalid_cursor` cho con trỏ của một cách sắp khác (`core/page/page.go:456-457`).
 * Đang ở trang 3 mà đổi thứ tự rồi giữ con trỏ trang 3 là một trang lỗi — nên hàm này trả thẳng
 * `TRANG_DAU`, và chỗ gọi không có cách nào quên nó.
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
      const n = v.kq.duLieu;
      // The code was renamed meanwhile: the tab follows the record rather than keep a dead code.
      const tabs = n.code === v.ma ? s.tabs : renameRecordTab(s.tabs, v.ma, n.code, taskTabData(n));
      return chuyenTaskPanel({ ...s, tabs }, { loai: "mo", nhiemVu: n });
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
      // A write or a re-read of the record on screen. A different code here is a RENAME (the
      // PATCH reply carries the new code, 3c3525f): the tab keeps its place under the new code.
      const tabs =
        before !== code ? renameRecordTab(s.tabs, before, code, data) : updateRecordTab(s.tabs, code, data);
      return { ...s, drawer, tabs };
    }
  }
}

/**
 * The content of a tab whose detail is not on screen: being read, or refused. The heading carries
 * the dialog's accessible name exactly as the detail's does, from the tab's own label.
 */
function TaskTabPending({
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
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="shrink-0 border-b border-solid border-line bg-surface px-4 py-3 md:px-6 md:py-4">
        <div className="flex items-start gap-3">
          <h2
            id={TASK_DETAIL_TITLE_ID}
            className="m-0 min-w-0 flex-1 text-lg leading-snug font-semibold text-ink-900 [overflow-wrap:anywhere]"
          >
            [{code}] {title}
          </h2>
          <IconButton label="Đóng chi tiết nhiệm vụ" type="button" variant="secondary" onClick={onHide}>
            <Glyph icon={X} />
          </IconButton>
        </div>
      </div>
      {error === null && <LoadingBar />}
      <div className="min-h-0 flex-1 overflow-y-auto p-4 md:p-6">
        {/* Always in the DOM: a live region inserted later is not always announced. */}
        <p className="an-thi-giac" role="status">
          {error === null ? TASK_TAB_LOADING : ""}
        </p>
        {error !== null && (
          <p className="thong-bao-loi mt-0" role="alert">
            {error}
          </p>
        )}
      </div>
    </div>
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
 * Kanban tắt vì mỗi cột chỉ đọc 20 thẻ và không vẽ các trạng thái rẽ nhánh (`Tạm dừng` là một).
 */
export const DRILL_DOWN_NOTE_TASKS =
  "Bộ lọc, ô tìm, phạm vi và chế độ Kanban tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

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
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [cheDoXem, datCheDoXem] = useState<CheDoXem>("kanban");
  // Kanban không cho số dòng bằng con số: 20 thẻ mỗi cột, không có cột rẽ nhánh. Lọc Tổng quan bật
  // thì luôn là Danh sách, có phân trang.
  const viewMode: CheDoXem = drillDownActive ? "danh-sach" : cheDoXem;

  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_nhiemVuRa>;
  } | null>(null);
  const [daTaiKanban, datDaTaiKanban] = useState<DaTaiKanban | null>(null);
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);
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
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);
  const [moFormTao, datMoFormTao] = useState(false);
  // Tăng khi một đề nghị lùi hạn vừa gửi xong từ drawer — hàng chờ phải thấy nó. KHÔNG gắn vào
  // `lanTai`: mỗi lần ghi trên sổ mà đọc lại hàng chờ là vứt mọi trang `Xem thêm` lãnh đạo đã mở.
  const [lanHangCho, datLanHangCho] = useState(0);
  // Kanban moves — separate from the drawer's `dangGui`/`loiGhi`: the answer belongs on the card
  // that was moved, not in a drawer that may be showing another task.
  const [kanbanPending, setKanbanPending] = useState<KanbanMove["pending"]>(null);
  const [kanbanResult, setKanbanResult] = useState<KanbanMoveResult | null>(null);
  // Real per-status totals for the Kanban headers (#15), keyed like the columns.
  const [countsLoaded, setCountsLoaded] = useState<{
    khoa: string;
    kq: KetQua<petitions_taskCountsOut>;
  } | null>(null);
  // `+ Thêm việc con` (#10): the create form, opened INSIDE the drawer of the parent it names.
  // Its own sending/error state: the drawer's `loiGhi` is shown by the drawer too, and one refusal
  // printed twice reads as two refusals.
  const [childFormFor, setChildFormFor] = useState<string | null>(null);
  const [childFormSending, setChildFormSending] = useState(false);
  const [childFormError, setChildFormError] = useState<string | null>(null);
  const [childCreated, setChildCreated] = useState<{ parent: string; text: string } | null>(null);
  // `Xuất Excel` of the Sổ theo dõi (W6): one export at a time; the answer stays until the next.
  const [exporting, setExporting] = useState(false);
  // `⬆ Nhập từ Excel` (W7, §8). Focus returns to the opening button when the dialog closes.
  const [importOpen, setImportOpen] = useState(false);
  const importButtonRef = useRef<HTMLButtonElement>(null);
  const [exportResult, setExportResult] = useState<KetQua<string> | null>(null);
  // `🗑 Xoá đã chọn` (§2). Survives paging and view switches on purpose: the clerk selects across
  // pages; the bar always says how many are selected, so nothing is chosen out of sight silently.
  const [selection, setSelection] = useState<TaskSelectionState>(EMPTY_SELECTION);
  const [batchProgress, setBatchProgress] = useState<{ done: number; total: number } | null>(null);
  const [batchResults, setBatchResults] = useState<readonly BatchDeleteResult[] | null>(null);

  // The register view reads the SAME page with `include=documents`: a different answer, so a
  // different key — switching view must not show a page read without the documents.
  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}|${viewMode === "so-theo-doi" ? "docs" : ""}`;
  // Kanban KHÔNG mang con trỏ: nó không phân trang, nên bộ lọc và lần ghi gần nhất là tất cả những
  // gì làm câu trả lời cũ hết hiệu lực.
  const khoaKanban = `${JSON.stringify(loc)}|${lanTai}`;

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
    let bo = false;
    // CÁCH SẮP LUÔN ĐI TRÊN DÂY, kể cả khi cán bộ chưa bấm cột nào: mũi tên trên đầu cột vẽ từ
    // `sapXepDayDu(loc)`, nên gửi đúng giá trị ấy là điều kiện để mũi tên nói thật về câu hỏi đã gửi.
    const sx = sapXepDayDu(loc);
    laySoNhiemVu({
      ...loc,
      sapXep: sx.cot,
      chieu: sx.chieu,
      limit: SO_DONG_MOI_TRANG,
      cursor: nganXep.hienTai,
      includeDocuments: viewMode === "so-theo-doi",
    }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, nganXep.hienTai, khoa, viewMode, daDocDuongDan]);

  /**
   * KANBAN ĐỌC CÙNG TUYẾN VÀ CÙNG BỘ LỌC VỚI DANH SÁCH — qua đúng `laySoNhiemVu`, nên mười tên
   * tham số truy vấn vẫn nằm ở một chỗ duy nhất (`themLocVaoTruyVan`). Hai chỗ ghép truy vấn là
   * hai chỗ sẽ trôi khỏi nhau, và cái trôi sẽ là cái không ai mở ra đọc.
   *
   * KHÁC ĐÚNG MỘT ĐIỀU: mỗi cột một lời gọi, mang thêm `status=<mã cột>`. Một trang 20 dòng chung
   * cho cả năm cột sẽ cho ra những cột rỗng chỉ vì trang ấy chưa tới lượt chúng — một cột rỗng vì
   * phân trang trông y hệt một cột rỗng vì xã không có việc nào.
   */
  useEffect(() => {
    if (!daDocDuongDan || viewMode !== "kanban") return;
    let bo = false;
    const ds = cotPhaiDoc(loc.trangThai);
    Promise.all(
      ds.map(async (ma) => ({
        ma,
        kq: await laySoNhiemVu({ ...loc, trangThai: ma, limit: SO_THE_MOI_COT }),
      })),
    ).then((cot) => {
      if (!bo) datDaTaiKanban({ khoa: khoaKanban, cot });
    });
    // THE SAME `loc` AS THE COLUMNS, through the same filter builder (`appendTaskFilters`): a
    // header counting under other filters than its cards is a number that disagrees with the board.
    // Read separately: a failed count must not hold the cards back, nor the other way round.
    getTaskCounts(loc).then((kq) => {
      if (!bo) setCountsLoaded({ khoa: khoaKanban, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, khoaKanban, viewMode, daDocDuongDan]);

  // NĂM DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN. Một danh mục hỏng thì ô lọc tương ứng rỗng — KHÔNG làm
  // hỏng quyển sổ: năm câu trả lời rời nhau, mỗi cái nói chuyện của nó. Riêng nhãn trạng thái
  // hỏng thì KHÔNG rỗng mà lui về nhãn mặc định KÈM một câu cảnh báo (`docBangNhanTrangThai`):
  // một cột Kanban không tên thì không ai đọc được.
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
  const luotDoc = drawer?.luotDoc ?? 0;
  useEffect(() => {
    if (maDrawer === null) return;
    let bo = false;
    layNhiemVu(maDrawer).then((kq) => {
      if (!bo) guiDrawer({ loai: "chiTietVe", ma: maDrawer, luotDoc, kq });
    });
    return () => {
      bo = true;
    };
  }, [maDrawer, luotDoc]);

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
   * How every click opens the detail dialog — a row, a card, the queue, a child task. The dialog
   * covers the list, so there is nothing to scroll to any more (the NV-09 scroll of 05/10/2026 is
   * gone with the in-page block). The address bar follows from `maDrawer` (`useTaskDialogUrl`
   * below), not from here: one place decides push vs replace for every way of opening.
   */
  function openDrawer(n: petitions_nhiemVuRa) {
    guiDrawer({ loai: "mo", nhiemVu: n });
  }
  // ADR 0068 §Sửa đổi 05/10/2026 #5. Back / Forward: `null` closes; a code opens it unless shown.
  // ADR 0068 §Sửa đổi 05/10/2026 #5, with record tabs: the address bar names the ACTIVE tab while
  // the panel is on screen. Opening from the list PUSHES (nothing shown → a code); switching or
  // closing a tab with another left REPLACES (a code → another code); hiding goes Back.
  const activeTab = panel.tabs.active;
  const shownCode = panel.open ? activeTab : null;
  useTaskDialogUrl(shownCode, (code) => {
    if (code === null) {
      guiDrawer({ loai: "dong" });
      datLoiGhi(null);
      return;
    }
    if (code === shownCode) return;
    if (panel.tabs.tabs.some((t) => t.id === code)) {
      guiDrawer({ loai: "chonTab", ma: code });
      datLoiGhi(null);
      return;
    }
    layNhiemVu(code).then((kq) => {
      if (!kq.ok) {
        setOpenTaskError(kq.thongBao);
        return;
      }
      openDrawer(kq.duLieu);
      datLoiGhi(null);
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
  // Bảng tra họ tên theo mã — dựng MỘT LẦN mỗi lần danh bạ về, dùng chung cho thẻ, dòng và drawer.
  // Không gọi mạng thêm lần nào: danh bạ đã đọc một lần cho cả màn ở khối danh mục trên.
  const danhBaMa = useMemo(() => danhBaChoNhatKy(kqDanhBa), [kqDanhBa]);

  const so = taiTu(daTai, khoa);
  const cotKanban: readonly CotKanban[] = cotPhaiDoc(loc.trangThai).map((ma) => ({
    ma,
    tai: taiCot(daTaiKanban, khoaKanban, ma),
  }));
  const tenBoPhan = new Map(danhMuc.boPhan.map((b) => [b.id, b.name]));
  const { bang: nhanTT, canhBao: canhBaoNhanTT } = docBangNhanTrangThai(kqNhanTT);

  /** Đổi bộ lọc là về trang đầu: con trỏ của bộ lọc cũ không có nghĩa với bộ lọc mới. */
  function datLocMoi(moi: BoLoc): void {
    datLocDaDoi(moi);
    datNganXep(TRANG_DAU);
  }

  /** Một lần ghi xong: giữ nhiệm vụ máy chủ vừa trả, xoá lỗi cũ, và đọc lại quyển sổ. */
  function xongGhi(kq: KetQua<petitions_nhiemVuRa>): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ — xem khối đầu tệp. Đây là chỗ câu "còn 3 việc con (NV20, NV21,
      // NV22)…" ra tới màn hình.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
    datLanTai((n) => n + 1);
  }

  function chay(goi: Promise<KetQua<petitions_nhiemVuRa>>): void {
    datDangGui(true);
    goi.then(xongGhi);
  }

  /**
   * NO OPTIMISTIC MOVE. The card stays where it is, marked pending, until the server answers;
   * only then does the board reload. A card that jumps and silently jumps back is a change the
   * clerk believes was made.
   */
  function moveOnKanban(task: petitions_nhiemVuRa, target: TrangThaiNhiemVu, via: KanbanMoveVia) {
    if (kanbanPending !== null) return;
    setKanbanResult(null);
    setKanbanPending({ code: task.code, target });
    moveTaskStatus(task.code, target, via).then((r) => {
      setKanbanPending(null);
      setKanbanResult(r);
      if (!r.ok) return;
      // The drawer re-reads only if it shows this very task (`docLai`); the board re-reads always.
      guiDrawer({ loai: "docLai", ma: task.code });
      datLanTai((n) => n + 1);
    });
  }

  /**
   * One soft delete per selected task, sequentially, one shared reason (`runBatchDelete`). NO
   * OPTIMISTIC REMOVAL: rows leave the board only when the register is read again after the last
   * answer. Failed tasks stay selected, each with the server's sentence.
   */
  async function runBatch(reason: string): Promise<void> {
    const tasks = [...selection.values()];
    if (tasks.length === 0 || batchProgress !== null) return;
    setBatchResults(null);
    setBatchProgress({ done: 0, total: tasks.length });
    const results = await runBatchDelete(tasks, reason, xoaNhiemVu, (done) =>
      setBatchProgress({ done, total: tasks.length }),
    );
    setBatchProgress(null);
    setBatchResults(results);
    setSelection((s) => keepFailed(s, results));
    // A drawer showing a task that is now deleted would show a record that is gone.
    // Their tabs go too; a panel showing one of them hides.
    const deleted = results.filter((r) => r.ok).map((r) => r.code);
    if (deleted.length > 0) guiDrawer({ loai: "boTab", ma: deleted });
    datLanTai((n) => n + 1);
  }

  /**
   * `Xuất Excel` — the register under the SAME filters and sort the screen shows (`sapXepDayDu`),
   * every matching row. A refusal (422 too large, 503 names unavailable, 409 no due-soon threshold)
   * is the server's sentence, verbatim; nothing is downloaded then.
   */
  function exportRegister(): void {
    if (exporting) return;
    setExporting(true);
    setExportResult(null);
    const sx = sapXepDayDu(loc);
    downloadTaskRegister({ ...loc, sapXep: sx.cot, chieu: sx.chieu }).then((r) => {
      setExporting(false);
      if (!r.ok) {
        setExportResult(r);
        return;
      }
      saveFile(r.duLieu.blob, r.duLieu.fileName);
      setExportResult({ ok: true, duLieu: r.duLieu.fileName });
    });
  }

  // `null` without `task.delete`: no checkbox anywhere (convenience — the route checks, rule 5).
  const taskSelection: TaskSelection | null = quyen.xoa
    ? {
        selected: selection,
        toggle: (t) => {
          setBatchResults(null);
          setSelection((s) => toggleSelected(s, t));
        },
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
  // drill-down, first page. Only then may the screen say the commune has no task yet.
  const noFilter =
    !drillDownActive &&
    !coTrangTruoc(nganXep) &&
    Object.entries(loc).every(([k, v]) => k === "sapXep" || k === "chieu" || v === undefined);
  const emptyList = noFilter ? (
    <EmptyState
      icon={ClipboardList}
      title="Chưa có nhiệm vụ nào"
      description={quyen.giaoViec ? "Bấm “Giao việc mới” hoặc nhập từ Excel để bắt đầu." : undefined}
      action={
        quyen.giaoViec && !moFormTao ? (
          <Button
            type="button"
            variant="secondary"
            icon={<Glyph icon={Plus} />}
            onClick={() => {
              // The header button's own steps, opening only: this button is not a toggle.
              datMoFormTao(true);
              setChildFormFor(null);
              datLoiGhi(null);
            }}
          >
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
      moNhiemVu={(n) => {
        openDrawer(n);
        datLoiGhi(null);
      }}
      sapXep={sapXepDayDu(loc)}
      doiSapXep={(cot) => {
        const moi = bamSapXep(loc, cot);
        datLocDaDoi(moi.loc);
        datNganXep(moi.nganXep);
      }}
      selection={taskSelection}
      empty={emptyList}
    />
  );

  return (
    <>
    <PageHeader
      icon={ListChecks}
      title="Quản lý nhiệm vụ"
      subtitle={
        <span className="inline-flex items-center gap-1.5">
          <Glyph icon={Workflow} />
          Giao việc từ kết luận họp, theo dõi tiến độ và đôn đốc tự động.
        </span>
      }
      actions={
        quyen.giaoViec ? (
          <>
            {/* §1 draws `[Nhập từ Excel] [Giao việc mới]` — both `task.create` (the import IS
                creation; the route checks again). */}
            <Button
              ref={importButtonRef}
              type="button"
              variant="secondary"
              icon={<Glyph icon={Upload} />}
              aria-expanded={importOpen}
              onClick={() => setImportOpen((o) => !o)}
            >
              {IMPORT_OPEN_BUTTON}
            </Button>
            <Button
              type="button"
              variant="primary"
              icon={<Glyph icon={moFormTao ? X : Plus} />}
              onClick={() => {
                datMoFormTao((m) => !m);
                setChildFormFor(null);
                datLoiGhi(null);
              }}
              aria-expanded={moFormTao}
            >
              {moFormTao ? "Đóng biểu mẫu giao việc" : "Giao việc mới"}
            </Button>
          </>
        ) : undefined
      }
    />
    {/* Direct children lose their legacy vertical margins: the section's `gap` is the one spacing
        between blocks (spec §6.9, 16px card ↔ card). */}
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
          danhMuc={danhMuc}
          danhBa={kqDanhBa}
          danhBaLanhDao={kqDanhBaLanhDao}
          // `POST /api/v1/tasks` nhận `documents`; màn Biên bản thì không — xem prop.
          coDanhSachVanBan
          staffSearch
          dangGui={dangGui}
          loi={loiGhi}
          huy={() => {
            datMoFormTao(false);
            datLoiGhi(null);
          }}
          giaoViec={(than, khoaChongTrung) => {
            datDangGui(true);
            taoNhiemVu(than, khoaChongTrung).then((kq) => {
              datDangGui(false);
              if (!kq.ok) {
                datLoiGhi(kq.thongBao);
                return;
              }
              datLoiGhi(null);
              datMoFormTao(false);
              guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
              datLanTai((n) => n + 1);
            });
          }}
        />
      )}

      {/* §5.8 — quyết định lùi hạn. Một MỤC trên sổ chứ không trong drawer: tuyến hàng chờ không
          lọc được theo nhiệm vụ (xem `HÀNG CHỜ DUYỆT LÙI HẠN`, `nhan-nhiem-vu.ts`). */}
      <HangChoLuiHan
        danhBa={danhBaMa}
        maNguoiDangNhap={maNguoiDangNhap}
        coQuyenDuyetGiaHan={quyen.duyetGiaHan}
        lanLamMoi={lanHangCho}
        moNhiemVu={(ma) =>
          layNhiemVu(ma).then((kq) => {
            if (kq.ok) {
              openDrawer(kq.duLieu);
              datLoiGhi(null);
            }
            return kq;
          })
        }
        daQuyet={(ma) => {
          // Duyệt là đổi hạn xử lý: drawer đang mở đúng việc ấy và quyển sổ đều đã cũ.
          guiDrawer({ loai: "docLai", ma });
          datLanTai((n) => n + 1);
        }}
      />

      <DrillDownBanner
        drillDown={drillDown}
        clearHref="/nhiem-vu"
        note={DRILL_DOWN_NOTE_TASKS}
        showInvalid={locDaDoi === null}
      />

      {/* THE LIST CARD (spec §5, §8.3): scope tabs + one filter row at its head, the view switch,
          the board or table in its own scroller, and notes + paging in its footer. `overflow-visible`:
          the staff combobox opens its list in the flow, and a focus ring on the edge must not be cut. */}
      <Card className="overflow-visible">
      {!drillDownActive && (
        <HangLoc
          loc={loc}
          tim={tim}
          datTim={datTim}
          datLoc={datLocMoi}
          danhMuc={danhMuc}
          danhBa={kqDanhBa}
          nhanTT={nhanTT}
        />
      )}

      {/* CỤM CHỌN CHẾ ĐỘ XEM — §2, bên phải hàng lọc 2. ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W6): BA nút.
          `Sổ theo dõi` §4.3 đọc cùng trang với Danh sách, kèm `include=documents` (90d12ff).
          Drawn as a segmented control; the buttons stay native `aria-pressed` toggles. */}
      {!drillDownActive && (
      <div className="flex flex-wrap items-center gap-3 border-b border-line px-4 py-3">
      <div className={TOGGLE_TRACK} role="group" aria-label="Chế độ xem">
        <button
          type="button"
          className={toggleButtonClass(cheDoXem === "kanban")}
          aria-pressed={cheDoXem === "kanban"}
          onClick={() => datCheDoXem("kanban")}
        >
          <Glyph icon={SquareKanban} />
          {NHAN_CHE_DO_KANBAN}
        </button>
        <button
          type="button"
          className={toggleButtonClass(cheDoXem === "danh-sach")}
          aria-pressed={cheDoXem === "danh-sach"}
          onClick={() => datCheDoXem("danh-sach")}
        >
          <Glyph icon={List} />
          {NHAN_CHE_DO_DANH_SACH}
        </button>
        <button
          type="button"
          className={toggleButtonClass(cheDoXem === "so-theo-doi")}
          aria-pressed={cheDoXem === "so-theo-doi"}
          onClick={() => datCheDoXem("so-theo-doi")}
        >
          <Glyph icon={BookOpen} />
          {REGISTER_VIEW_LABEL}
        </button>
      </div>
      </div>
      )}

      {quyen.xoa && (selection.size > 0 || batchResults !== null || batchProgress !== null) && (
        <div className="border-b border-line px-4 py-3 [&>*]:my-0">
        <BatchDeleteBar
          count={selection.size}
          progress={batchProgress}
          results={batchResults}
          onRun={(reason) => void runBatch(reason)}
          onClear={() => {
            setSelection(EMPTY_SELECTION);
            setBatchResults(null);
          }}
        />
        </div>
      )}

      {viewMode === "kanban" && (
        <div className="min-w-0 p-4">
        <BangKanban
          cot={cotKanban}
          danhMuc={danhMuc}
          danhBa={danhBaMa}
          unitNames={tenBoPhan}
          nhanTT={nhanTT}
          bayGio={new Date()}
          maDangMo={maDrawer}
          moNhiemVu={(n) => {
            openDrawer(n);
            datLoiGhi(null);
          }}
          counts={taiTu(countsLoaded, khoaKanban)}
          move={{
            permissions: quyen,
            staffCode: maNguoiDangNhap,
            pending: kanbanPending,
            result: kanbanResult,
            move: moveOnKanban,
          }}
          selection={taskSelection}
        />
        </div>
      )}

      {viewMode === "so-theo-doi" && (
        <div className="cum-nut border-b border-line px-4 py-3 [&>p]:my-0">
          <Button
            type="button"
            variant="secondary"
            icon={<Glyph icon={Download} />}
            disabled={exporting}
            onClick={exportRegister}
          >
            {REGISTER_EXPORT_BUTTON}
          </Button>
          <p className="ghi-chu">{REGISTER_EXPORT_NOTE}</p>
          {/* Always in the DOM in this view: a live region inserted later is not always announced. */}
          <p role="status">
            {exporting
              ? REGISTER_EXPORT_PENDING
              : exportResult !== null && exportResult.ok
                ? registerExportDoneText(exportResult.duLieu)
                : ""}
          </p>
          {!exporting && exportResult !== null && !exportResult.ok && (
            <p className="thong-bao-loi" role="alert">
              {REGISTER_EXPORT_REFUSED} {exportResult.thongBao}
            </p>
          )}
        </div>
      )}

      {/* LOADING (spec §8b). The sentence stays the live region, read out as before; the eye gets a
          2px bar and either the PREVIOUS page dimmed (a re-read: `daTai` still holds the last
          answer while the new key loads) or row-shaped placeholders (the first read). The dimmed
          rows are `inert`: an answer to the old question is shown, never acted on. Only the List
          view re-shows old rows — the register's document columns depend on `include=documents`,
          which the previous answer may not carry. */}
      {viewMode !== "kanban" && so.pha === "dangTai" && (
        <>
          <LoadingBar />
          <p className="an-thi-giac" role="status">
            {DANG_TAI_SO}
          </p>
          {viewMode === "danh-sach" && staleItems !== null ? (
            <div inert className="pointer-events-none opacity-60 transition-opacity">
              {listTable(staleItems)}
            </div>
          ) : (
            <TaskRowsSkeleton />
          )}
        </>
      )}
      {/* LOAD ERROR (spec §8b): the server's sentence VERBATIM stays the alert; `Tải lại` asks the
          same read again through the screen's existing re-read key (`lanTai`). */}
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
          nhiemVu={so.duLieu.items}
          danhMuc={danhMuc}
          danhBa={danhBaMa}
          tenBoPhan={tenBoPhan}
          bayGio={new Date()}
          maDangMo={maDrawer}
          moNhiemVu={(n) => {
            openDrawer(n);
            datLoiGhi(null);
          }}
          selection={taskSelection}
          empty={emptyList}
        />
      )}

      {viewMode !== "kanban" && so.pha === "xong" && (
        <>
          {viewMode === "danh-sach" && listTable(so.duLieu.items)}
          <CardFooter className="justify-between">
            <div className="min-w-0 [&>.ghi-chu]:m-0">
          <p className="ghi-chu">{nhanBoDem(so.duLieu.items.length)}</p>
          <p className="ghi-chu">{NO_DEADLINE_LAST_NOTE}</p>
          <p className="ghi-chu">{NO_PRIORITY_LAST_NOTE}</p>
            </div>
          <nav className="dieu-huong-trang m-0" aria-label="Phân trang sổ nhiệm vụ">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Glyph icon={ChevronLeft} />}
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
              <Glyph icon={ChevronRight} />
            </Button>
          </nav>
          </CardFooter>
        </>
      )}
      </Card>

      {/* THE DETAIL IS A LARGE DIALOG OVER THE LIST (ADR 0068 §Sửa đổi 05/10/2026 #3). Esc asks
          the same `dong` as the ✕ — it HIDES the panel, the record tabs stay. The dialog stays
          mounted across tab switches and parent / child moves (only the content is keyed by code),
          so focus still returns to the row that first opened it. */}
      {shownCode !== null && (
        <LargeDialog
          titleId={TASK_DETAIL_TITLE_ID}
          onDismiss={() => {
            guiDrawer({ loai: "dong" });
            datLoiGhi(null);
          }}
        >
        <RecordTabStrip
          label={TASK_TABS_LABEL}
          idPrefix={TASK_TAB_ID_PREFIX}
          panelId={TASK_TAB_PANEL_ID}
          // Code + title + the commune's status label: what the register row already shows.
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
            datLoiGhi(null);
          }}
          onClose={(ma) => {
            guiDrawer({ loai: "dongTab", ma });
            if (ma === shownCode) datLoiGhi(null);
          }}
          onCloseAll={() => {
            guiDrawer({ loai: "dongTatCa" });
            datLoiGhi(null);
          }}
          closeTabLabel={closeTaskTabLabel}
        />
        <div
          id={TASK_TAB_PANEL_ID}
          role="tabpanel"
          aria-labelledby={recordTabDomId(TASK_TAB_ID_PREFIX, shownCode)}
          className="flex min-h-0 min-w-0 flex-1 flex-col"
        >
        {drawer === null || drawer.nhiemVu.code !== shownCode ? (
          <TaskTabPending
            code={shownCode}
            title={panel.tabs.tabs.find((t) => t.id === shownCode)?.data.title ?? ""}
            error={panel.tabError?.code === shownCode ? panel.tabError.message : null}
            onHide={() => {
              guiDrawer({ loai: "dong" });
              datLoiGhi(null);
            }}
          />
        ) : (
        <ChiTietNhiemVu
          // A new task is a new detail: the open tab, the half-typed note and reason do not follow.
          key={drawer.nhiemVu.code}
          nhiemVu={drawer.nhiemVu}
          vanBan={drawer.vanBan}
          danhMuc={danhMuc}
          nhanTT={nhanTT}
          tenBoPhan={tenBoPhan}
          danhBa={kqDanhBa}
          // `luotDoc` tăng khi mở drawer VÀ sau mỗi lần ghi — đúng hai lúc nhật ký phải đọc lại.
          lanLamMoiNhatKy={drawer.luotDoc}
          bayGio={new Date()}
          maNguoiDangNhap={maNguoiDangNhap}
          quyen={quyen}
          dangGui={dangGui}
          loiGhi={loiGhi}
          dong={() => {
            guiDrawer({ loai: "dong" });
            datLoiGhi(null);
          }}
          doiTrangThai={(trangThai, ghiChu) =>
            chay(doiTrangThaiNhiemVu(drawer.nhiemVu.code, trangThai, ghiChu))
          }
          xoa={(lyDo) => {
            datDangGui(true);
            xoaNhiemVu(drawer.nhiemVu.code, lyDo).then((kq) => {
              datDangGui(false);
              if (!kq.ok) {
                // ĐÂY LÀ CHỖ CÂU "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước" RA
                // TỚI MÀN HÌNH, kèm đúng con số (ADR 0037 quyết định 3).
                datLoiGhi(kq.thongBao);
                return;
              }
              datLoiGhi(null);
              // The record is gone: its tab goes with it, and the panel hides as before tabs.
              guiDrawer({ loai: "boTab", ma: [drawer.nhiemVu.code] });
              datLanTai((n) => n + 1);
            });
          }}
          guiDeNghiLuiHan={(hanMoi, lyDo) =>
            deNghiLuiHan(drawer.nhiemVu.code, hanMoi, lyDo).then((kq) => {
              if (kq.ok) datLanHangCho((n) => n + 1);
              return kq;
            })
          }
          quyetDinh={(deNghiID, duyet, ghiChu) =>
            quyetDinhLuiHan(drawer.nhiemVu.code, deNghiID, duyet, ghiChu)
          }
          // The drawer's pending-request block re-reads on every drawer re-read AND when the queue
          // on the register moves (`lanHangCho`): a request sent or decided there is this block's.
          extensionRefreshKey={`${drawer.luotDoc}|${lanHangCho}`}
          onExtensionDecided={(ma) => {
            // Approving moves `due_at`: the drawer, the register and the queue are all stale.
            guiDrawer({ loai: "docLai", ma });
            datLanTai((n) => n + 1);
            datLanHangCho((n) => n + 1);
          }}
          openTask={(n) => {
            openDrawer(n);
            datLoiGhi(null);
          }}
          openTaskByCode={(ma) =>
            layNhiemVu(ma).then((kq) => {
              if (kq.ok) {
                openDrawer(kq.duLieu);
                datLoiGhi(null);
              }
              return kq;
            })
          }
          saveParent={(than) =>
            suaNhiemVu(drawer.nhiemVu.code, than).then((kq) => {
              if (kq.ok) {
                guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
                datLanTai((n) => n + 1);
              }
              // A refusal (409 `task_tree`) goes back to the field VERBATIM — see `ParentTaskField`.
              return kq;
            })
          }
          addChild={
            // `+ Thêm việc con` stands behind the SAME key as `+ Giao việc mới` — `task.create`
            // (`quyen.giaoViec`), the key of `POST /api/v1/tasks`. The server checks it anyway.
            quyen.giaoViec
              ? {
                  open: childFormFor === drawer.nhiemVu.code,
                  created:
                    childCreated !== null && childCreated.parent === drawer.nhiemVu.code
                      ? childCreated.text
                      : null,
                  toggle: () => {
                    // ONE create form on the page at a time: both render the same field ids
                    // (`giao-loai`…), and two labels pointing at one id is a broken form for a
                    // screen reader.
                    datMoFormTao(false);
                    setChildFormFor((f) =>
                      f === drawer.nhiemVu.code ? null : drawer.nhiemVu.code,
                    );
                    setChildFormError(null);
                    setChildCreated(null);
                  },
                  form: (
                    <FormGiaoViec
                      // A new form (and a new idempotency key) per parent.
                      key={drawer.nhiemVu.code}
                      danhMuc={danhMuc}
                      danhBa={kqDanhBa}
                      danhBaLanhDao={kqDanhBaLanhDao}
                      coDanhSachVanBan
                      staffSearch
                      maChaCoSan={drawer.nhiemVu.code}
                      dangGui={childFormSending}
                      loi={childFormError}
                      huy={() => {
                        setChildFormFor(null);
                        setChildFormError(null);
                      }}
                      giaoViec={(than, khoaChongTrung) => {
                        const parentCode = drawer.nhiemVu.code;
                        setChildFormSending(true);
                        taoNhiemVu(than, khoaChongTrung).then((kq) => {
                          setChildFormSending(false);
                          if (!kq.ok) {
                            // VERBATIM — an invalid parent is the server's 409 `task_tree` sentence.
                            setChildFormError(kq.thongBao);
                            return;
                          }
                          setChildFormError(null);
                          setChildFormFor(null);
                          setChildCreated({ parent: parentCode, text: childCreatedText(kq.duLieu.code) });
                          // STAY ON THE PARENT: re-read it (its `child_count`, its children block)
                          // instead of jumping to the new child, which would lose the tree.
                          guiDrawer({ loai: "docLai", ma: parentCode });
                          datLanTai((n) => n + 1);
                        });
                      }}
                    />
                  ),
                }
              : null
          }
          // §5.4 `✎ Sửa`. Thành công: phản hồi PATCH MANG `documents` (`app/nhiem_vu.go:696-741`),
          // nên `ghiXong` thay cả trường vô hướng lẫn khối văn bản, rồi đọc lại sổ như mọi lần ghi
          // khác. Hỏng: trả `KetQua` nguyên vẹn về form, để form GIỮ chữ cán bộ đã gõ và in câu máy
          // chủ — không đóng form, không xoá gì.
          // RENAME (3c3525f): the reply carries the NEW code; `ghiXong` takes the reply, so the drawer,
          // its next detail read and every block keyed by code switch to it — the old code 404s.
          // FAILURE: re-read the drawer (a 409 `task_changed` means the task moved on) — the form
          // keeps what was typed and says so; it does not adopt the new version itself.
          suaKhoiVanBan={(than) => {
            const code = drawer.nhiemVu.code;
            return suaNhiemVu(code, than).then((kq) => {
              if (kq.ok) {
                datLoiGhi(null);
                guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
                datLanTai((n) => n + 1);
              } else {
                guiDrawer({ loai: "docLai", ma: code });
              }
              return kq;
            });
          }}
          docLaiChiTiet={() => layNhiemVu(drawer.nhiemVu.code)}
          // §5.7 — same refresh as `✎ Sửa`: `ghiXong` takes the returned task (keeping the document
          // block, which this reply does not carry) and bumps `luotDoc`, so the detail AND the
          // timeline re-read; the register re-reads too — the card may have changed column
          // (`moi-giao`). A refusal goes back to the block, verbatim.
          reassign={(body) =>
            reassignTask(drawer.nhiemVu.code, body).then((kq) => {
              if (kq.ok) {
                datLoiGhi(null);
                guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
                datLanTai((n) => n + 1);
              }
              return kq;
            })
          }
        />
        )}
        </div>
        </LargeDialog>
      )}
    </section>
    </>
  );
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

/**
 * Bộ lọc §3.
 *
 * Mọi ô đều có tuyến đứng sau. ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5): tab `Liên quan đến tôi` và ô tick
 * `Sắp đến hạn` từng vắng mặt vì máy chủ từ chối; nay máy chủ phục vụ cả hai (3a4e60f).
 *
 * Ô `Người thực hiện` là Ô GÕ TÊN ĐỂ TÌM (`StaffCombobox`) trên danh bạ chọn người
 * (`GET /api/v1/staff-directory`, mọi cán bộ đăng nhập đọc được). Chữ gõ lọc TẠI CHỖ, không đi lên
 * mạng hay đường dẫn; giá trị là MÃ NGHIỆP VỤ `CB-…` — thứ `?assignee=` so khớp.
 */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  danhMuc,
  danhBa,
  nhanTT,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  danhMuc: DanhMucNhiemVu;
  /** Câu trả lời nguyên vẹn của danh bạ chọn người; `null` = chưa đọc xong. */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /** Nhãn và thứ tự bảy trạng thái của xã — ô lọc hiện đúng chữ và thứ tự xã đặt. */
  nhanTT: BangNhanTrangThai;
}) {
  const db = docDanhBaChonNguoi(danhBa);
  // Panel filters that are on — each is ABSENT from `loc` at its default, exactly what is not sent.
  const moreActiveCount = [
    loc.loai,
    loc.khoi,
    loc.mucUuTien,
    loc.nguonGiao,
    loc.boPhanID,
    loc.nguoiThucHienMa,
    loc.chiTreHan,
    loc.dueSoon,
  ].filter((v) => v !== undefined).length;

  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  return (
    <>
      {/* BA TAB PHẠM VI (§3). `mine` và `related` KHÔNG mang theo danh tính nào — máy chủ lấy mã cán
          bộ từ PHIÊN, và với `related` tự hỏi `identity` bộ phận của người ấy. Một tab gửi lên
          `?assignee=CB-…` của chính mình sẽ là client tự khai mình là ai (luật 1, cấm #2).
          ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5): tab thứ ba có từ khi máy chủ phục vụ `scope=related`.
          Drawn as underline tabs at the head of the list card (spec §8.3); the click handlers and the
          value each tab sends are unchanged. */}
      <TabList aria-label="Phạm vi" className="px-4">
        <Tab
          icon={Landmark}
          selected={loc.phamVi !== "mine" && loc.phamVi !== "related"}
          onClick={() => datLoc({ ...loc, phamVi: undefined })}
        >
          {PHAM_VI_TOAN_XA}
        </Tab>
        <Tab
          icon={UserRound}
          selected={loc.phamVi === "mine"}
          onClick={() => datLoc({ ...loc, phamVi: "mine" })}
        >
          {PHAM_VI_CUA_TOI}
        </Tab>
        <Tab
          icon={Link2}
          selected={loc.phamVi === "related"}
          aria-describedby={loc.phamVi === "related" ? "pham-vi-lien-quan-ghi-chu" : undefined}
          onClick={() => datLoc({ ...loc, phamVi: "related" })}
        >
          {SCOPE_RELATED_LABEL}
        </Tab>
      </TabList>
      {loc.phamVi === "related" && (
        <p id="pham-vi-lien-quan-ghi-chu" className="ghi-chu mx-4 mt-2">
          {SCOPE_RELATED_NOTE}
        </p>
      )}

      {/* ONE FILTER ROW (spec §6.4) + "Bộ lọc" (owner, 02/10/2026): the search box first, then the
          status; the seven narrower filters sit in the panel behind the button. Every native control
          keeps its id, value and handler. */}
      <FilterBar
        id="task-filters"
        className="m-0 border-b border-line px-4 py-3.5"
        moreActiveCount={moreActiveCount}
        primary={
          <>
            {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
                đang gõ vào một URL, và một URL đi vào mọi log truy cập (luật 3, cấm #4). */}
            <form
              className="form-tra-cuu m-0 flex min-w-0 flex-[1_1_320px] flex-row items-end gap-2 max-w-[480px]"
              onSubmit={timNgay}
              role="search"
            >
              <Field label="Tìm trong sổ" htmlFor="tim-nhiem-vu" icon={Search} grow="search" className="max-w-none">
                <input
                  id="tim-nhiem-vu"
                  name="tim-nhiem-vu"
                  value={tim}
                  placeholder={TIM_PLACEHOLDER}
                  onChange={(e) => datTim(e.target.value)}
                  autoComplete="off"
                  // Máy chủ trả 400 khi quá 200 ký tự (`store.TimNhiemVuToiDa`). Chặn ở ô nhập để cán bộ
                  // thấy giới hạn thay vì thấy "không tải được".
                  maxLength={200}
                />
              </Field>
              <Button variant="secondary" type="submit">
                Tìm
              </Button>
            </form>

            <Field label="Trạng thái" htmlFor="loc-trang-thai" icon={CircleDot} kind="select">
              <select
                id="loc-trang-thai"
                value={loc.trangThai ?? ""}
                onChange={(e) => datLoc({ ...loc, trangThai: e.target.value || undefined })}
              >
                <option value="">{MOI_TRANG_THAI_NHAN}</option>
                {nhanTT.thuTu.map((ma) => (
                  <option key={ma} value={ma}>
                    {nhanTrangThai(nhanTT, ma)}
                  </option>
                ))}
              </select>
            </Field>
          </>
        }
        more={
          <>
            <Field label="Loại nhiệm vụ" htmlFor="loc-loai" icon={Shapes} kind="select">
              <select
                id="loc-loai"
                value={loc.loai ?? ""}
                onChange={(e) => datLoc({ ...loc, loai: e.target.value || undefined })}
              >
                <option value="">{MOI_LOAI_NHAN}</option>
                {danhMuc.loai.map((l) => (
                  <option key={l.code} value={l.code}>
                    {l.label}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Khối" htmlFor="loc-khoi" icon={Layers} kind="select">
              <select
                id="loc-khoi"
                value={loc.khoi ?? ""}
                onChange={(e) => datLoc({ ...loc, khoi: e.target.value || undefined })}
              >
                <option value="">{MOI_KHOI_NHAN}</option>
                {danhMuc.khoi.map((k) => (
                  <option key={k.code} value={k.code}>
                    {k.label}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Mức ưu tiên" htmlFor="loc-uu-tien" icon={Flag} kind="select">
              <select
                id="loc-uu-tien"
                value={loc.mucUuTien ?? ""}
                onChange={(e) => datLoc({ ...loc, mucUuTien: e.target.value || undefined })}
              >
                <option value="">{MOI_MUC_UU_TIEN_NHAN}</option>
                {/* KHÔNG SẮP XẾP LẠI MẢNG NÀY: thứ tự `items` LÀ thang bậc của xã, không phải sở thích
                    trình bày (`muc_uu_tien_nhiem_vu.go`). */}
                {danhMuc.mucUuTien.map((m) => (
                  <option key={m.code} value={m.code}>
                    {m.label}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Nguồn giao" htmlFor="loc-nguon-giao" icon={GitBranch} kind="select">
              <select
                id="loc-nguon-giao"
                value={loc.nguonGiao ?? ""}
                onChange={(e) => datLoc({ ...loc, nguonGiao: e.target.value || undefined })}
              >
                <option value="">{MOI_NGUON_GIAO_NHAN}</option>
                {MOI_NGUON_GIAO.map((ma) => (
                  <option key={ma} value={ma}>
                    {nhanNguonGiao(ma)}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Bộ phận" htmlFor="loc-bo-phan" icon={Building2} kind="select">
              <select
                id="loc-bo-phan"
                value={loc.boPhanID ?? ""}
                onChange={(e) => datLoc({ ...loc, boPhanID: e.target.value || undefined })}
              >
                <option value="">{MOI_BO_PHAN_NHAN}</option>
                {danhMuc.boPhan.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </Field>

            {/* Mã đi lên URL (`?assignee=CB-…`) — mã nghiệp vụ, không phải dữ liệu cá nhân; họ tên
                chỉ nằm trong chữ của lựa chọn, không bao giờ lên URL (luật 3, cấm #4).
                The combobox is a shared component with its own label and hint; the frame below only
                restyles it to the Field look by descendant selectors and draws the leading icon. It aligns
                on its TOP (label row) because its hint line hangs below the control. */}
            <div className={STAFF_FILTER_FRAME}>
              <Glyph
                icon={UserRound}
                className="pointer-events-none absolute top-[32px] left-3 z-[1] size-[18px] text-ink-500"
              />
              <StaffCombobox
                id="loc-nguoi-thuc-hien"
                label="Người thực hiện"
                emptyLabel={nhanTrongOChonCanBo(db, MOI_NGUOI_THUC_HIEN_NHAN)}
                value={loc.nguoiThucHienMa ?? ""}
                directory={db.ds}
                disabled={db.dangTai}
                onChange={(ma) => datLoc({ ...loc, nguoiThucHienMa: ma || undefined })}
              />
            </div>
            {db.loi !== null && (
              <p className="thong-bao-loi col-span-full m-0" role="alert">
                {cauLoiDanhBaLoc(db.loi)}
              </p>
            )}

            <div className="o-chon h-10">
              <label htmlFor="loc-qua-han">
                <input
                  id="loc-qua-han"
                  type="checkbox"
                  checked={loc.chiTreHan === true}
                  // Ô bỏ tích thì tham số VẮNG MẶT HẲN, không gửi `late=false` — máy chủ chỉ nhận đúng
                  // chuỗi `true` và trả 400 cho mọi giá trị khác.
                  onChange={(e) => datLoc({ ...loc, chiTreHan: e.target.checked ? true : undefined })}
                />{" "}
                {CHI_QUA_HAN_NHAN}
              </label>
            </div>

            {/* §3 `☐ Sắp đến hạn` (W5). Only the switch goes up — `soon=true`, never a number: the
                threshold is the commune's own, read by the server from identity. A commune that set none
                gets the server's 409 sentence in place of the page, verbatim. Unticked = absent. */}
            <div className="o-chon h-10">
              <label htmlFor="loc-sap-den-han">
                <input
                  id="loc-sap-den-han"
                  type="checkbox"
                  checked={loc.dueSoon === true}
                  onChange={(e) => datLoc({ ...loc, dueSoon: e.target.checked ? true : undefined })}
                />{" "}
                {DUE_SOON_FILTER_LABEL}
              </label>
            </div>
          </>
        }
      />
    </>
  );
}

/**
 * Restyles the shared `StaffCombobox` (its own `.o-nhap` label + input + hint) to the Field look of
 * the filter row: 12px/600 label 6px above a 40px control, room for the leading icon. Descendant
 * selectors only — the component's markup, ids and behaviour are not touched (it is outside this
 * screen's scope). The icon's `top-[32px]` follows from this: 15px label (12px × 1.25, the Field
 * label) + 6px gap + (40−18)/2. It sits in the FilterBar panel grid, so it takes its cell's width.
 */
const STAFF_FILTER_FRAME = cn(
  "relative min-w-0 self-start",
  "[&_.o-nhap]:m-0 [&_.o-nhap]:flex [&_.o-nhap]:flex-col [&_.o-nhap]:gap-1.5",
  "[&_.o-nhap>label]:m-0 [&_.o-nhap>label]:text-xs [&_.o-nhap>label]:leading-tight [&_.o-nhap>label]:font-semibold [&_.o-nhap>label]:text-ink-700",
  "[&_.hop-tim-can-bo_input]:h-10 [&_.hop-tim-can-bo_input]:min-h-10 [&_.hop-tim-can-bo_input]:pl-10 [&_.hop-tim-can-bo_input]:text-base md:[&_.hop-tim-can-bo_input]:text-sm",
  "[&_.nut-mo-danh-sach]:size-10 [&_.nut-mo-danh-sach]:min-h-10 [&_.nut-mo-danh-sach]:min-w-10 [&_.nut-mo-danh-sach]:p-0",
  "[&_.goi-y-tim]:text-xs",
);

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
 * Bảng Kanban §4.1 — năm cột ứng với năm trạng thái CHÍNH.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KÉO-THẢ CÓ, VÀ CÓ MỘT LỐI BÀN PHÍM NGANG HÀNG (28/09/2026).
 *
 * Kéo-thả HTML5 một mình không có lối bàn phím: cán bộ dùng bàn phím, trình đọc màn hình, hay cầm
 * chuột không vững sẽ không thao tác được (`skills/accessibility-elderly`). Nên mỗi thẻ có thêm nút
 * `Chuyển sang cột…` (`KanbanMoveMenu`), liệt kê ĐÚNG các bước drawer liệt kê
 * (`clickableTransitions`). Hai lối gọi CÙNG một hàm (`move.move` → `moveTaskStatus`), tức cùng
 * tuyến `POST /api/v1/tasks/{ma}/status` của drawer, và câu từ chối của máy chủ hiện NGUYÊN VĂN
 * trên chính thẻ — kể cả câu liệt kê mã việc con còn lại.
 *
 * KHÔNG DI CHUYỂN LẠC QUAN: thẻ ở nguyên cột, hiện "đang chuyển", tới khi máy chủ trả lời. Cột chỉ
 * nhận thả khi vòng đời có bước ấy — thả vào cột khác thì trình duyệt không cho thả.
 *
 * Không có `move` (mặc định `null`) thì bảng CHỈ ĐỌC: không kéo, không nút.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * Năm cột xếp dọc dưới 768px và nằm cạnh nhau từ 768px (xem khối đầu tệp). KHÔNG CÓ chấm màu đầu
 * cột lẫn viền trái tô theo mức ưu tiên: hai thứ ấy là màu, và màu không bao giờ là tín hiệu duy
 * nhất (a11y) — mức ưu tiên hiện thành CHỮ trên thẻ, tình trạng trễ hiện thành chữ `Trễ N ngày`.
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W4): mỗi thẻ có `☐ Chọn` (§4.1) cho `🗑 Xoá đã chọn` khi phiên cầm
 * `task.delete` — `selection`, mặc định `null` = không ô tick. Trước đó: "không có tuyến nào"; nay
 * nút gom gọi từng tuyến xoá một (require a037b76).
 */
/** Default of `unitNames`: no catalogue yet — a card with no assignee then shows the unit id. */
const NO_UNIT_NAMES: ReadonlyMap<string, string> = new Map();

export function BangKanban({
  cot,
  danhMuc,
  danhBa = null,
  unitNames = NO_UNIT_NAMES,
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
  /** Unit names by id — a card with no assignee names the unit holding it (`cardHolderText`). */
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
   * Nhãn và thứ tự của xã: tên cột VÀ thứ tự cột. §4.1 cố định năm cột chính; thứ tự năm cột ấy
   * theo `order` xã đặt (`theoThuTuXa`). Xếp Ở ĐÂY chứ không ở chỗ gọi, để thứ tự vẽ ra kiểm được
   * bằng một lần kết xuất.
   */
  nhanTT: BangNhanTrangThai;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
}) {
  // The card being dragged. Held here, not in `dataTransfer`: `dragover` cannot read the payload,
  // and the column must know DURING the drag whether the lifecycle allows the drop.
  const [dragging, setDragging] = useState<petitions_nhiemVuRa | null>(null);
  const statusRef = useRef<HTMLParagraphElement>(null);
  const result = move?.result ?? null;

  useEffect(() => {
    // A menu move that succeeded reloads the board, and the button that had focus is gone with
    // the card. Put focus on the sentence that says where the card went, not on `<body>`.
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

  const soThe = cot.reduce(
    (tong, c) => tong + (c.tai.pha === "xong" ? c.tai.duLieu.items.length : 0),
    0,
  );
  const totalOf = (status: TrangThaiNhiemVu): number | null =>
    counts.pha === "xong" ? kanbanColumnCount(counts.duLieu.by_status, status) : null;

  // Every column refused for ONE reason (e.g. 409 under `Sắp đến hạn`): say it ONCE, above the board.
  const sharedError = kanbanSharedError(cot);

  const dropAllowed = (target: TrangThaiNhiemVu): boolean =>
    move !== null &&
    move.pending === null &&
    dragging !== null &&
    clickableTransitions(dragging, move.permissions, move.staffCode).includes(target);

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
      {/* ALWAYS IN THE DOM while moves are possible: a live region inserted later is one not every
          screen reader announces. `tabIndex={-1}` so focus can land here after a menu move. */}
      {move !== null && (
        <p ref={statusRef} role="status" tabIndex={-1} className="trang-thai-chuyen-cot">
          {statusText}
        </p>
      )}
      {sharedError !== null && (
        <p className="thong-bao-loi" role="alert">
          {sharedError}
        </p>
      )}
      <div className="bang-cuon" role="region" aria-label="Bảng Kanban nhiệm vụ" tabIndex={0}>
        <div className="bang-kanban">
          {cotSap.map((c) => (
            <section
              key={c.ma}
              className={dropAllowed(c.ma) ? "cot-kanban cot-nhan-tha" : "cot-kanban"}
              aria-labelledby={`cot-kanban-${c.ma}`}
              onDragOver={
                move === null
                  ? undefined
                  : (e) => {
                      // Accepting `dragover` IS what makes a column a drop target. A column the
                      // lifecycle does not allow never accepts it, so the drop cannot happen.
                      if (!dropAllowed(c.ma)) return;
                      e.preventDefault();
                      e.dataTransfer.dropEffect = "move";
                    }
              }
              onDrop={
                move === null
                  ? undefined
                  : (e) => {
                      e.preventDefault();
                      const task = dragging;
                      setDragging(null);
                      if (task !== null && dropAllowed(c.ma)) move.move(task, c.ma, "drag");
                    }
              }
            >
              <h3 id={`cot-kanban-${c.ma}`}>
                {/* NHÃN CỘT LÀ NHÃN CỦA XÃ — cùng một chữ với chip và ô lọc. Chữ "Chưa thực hiện"
                    của §4.1 là thứ xã tự đặt cho `moi-giao` ở tab Danh mục (xem `nhan-nhiem-vu.ts`). */}
                {nhanTrangThai(nhanTT, c.ma)}{" "}
                {/* THE REAL TOTAL (#15), not the cards loaded. Unreadable or missing → `—`, never
                    a `0` that reads as "nothing here". Still loading → no chip yet. */}
                {counts.pha !== "dangTai" && (
                  <span className="chip chip-ngung">
                    {totalOf(c.ma) === null ? O_TRONG : String(totalOf(c.ma))}
                  </span>
                )}
              </h3>
              {/* Words, not only the outline colour, say where the card may go (a11y §8). */}
              {dropAllowed(c.ma) && <p className="goi-y-tha">{kanbanDropHint(nhanTT, c.ma)}</p>}

              {/* NĂM CỘT LÀ NĂM CÂU TRẢ LỜI RỜI NHAU. Một cột hỏng thì bốn cột kia vẫn là sổ —
                  và câu hỏng của nó hiện nguyên văn ở đúng cột ấy, không nuốt thành một lỗi chung. */}
              {c.tai.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
              {c.tai.pha === "loi" && sharedError === null && (
                <p className="thong-bao-loi" role="alert">
                  {c.tai.thongBao}
                </p>
              )}
              {c.tai.pha === "xong" && c.tai.duLieu.items.length === 0 && (
                <p className="trang-thai-rong">{COT_RONG}</p>
              )}
              {c.tai.pha === "xong" && c.tai.duLieu.items.length > 0 && (
                <ul className="danh-sach-the">
                  {c.tai.duLieu.items.map((n) => (
                    <li key={n.code}>
                      <TheNhiemVu
                        nhiemVu={n}
                        danhMuc={danhMuc}
                        danhBa={danhBa}
                        unitNames={unitNames}
                        nhanTT={nhanTT}
                        bayGio={bayGio}
                        maDangMo={maDangMo}
                        moNhiemVu={moNhiemVu}
                        move={move}
                        selection={selection}
                        onDragStart={setDragging}
                        onDragEnd={() => setDragging(null)}
                      />
                    </li>
                  ))}
                </ul>
              )}
              {/* 20 cards under a header of 57 must say so — the board has no per-column paging. */}
              {c.tai.pha === "xong" &&
                (c.tai.duLieu.has_more || (totalOf(c.ma) ?? 0) > c.tai.duLieu.items.length) && (
                  <p className="ghi-chu">
                    {kanbanPartialNote(c.tai.duLieu.items.length, totalOf(c.ma))}
                  </p>
                )}
            </section>
          ))}
        </div>
      </div>

      {counts.pha === "loi" && counts.thongBao !== sharedError && (
        <p className="thong-bao-loi" role="alert">
          {KANBAN_COUNTS_ERROR} {counts.thongBao}
        </p>
      )}
      <p className="ghi-chu">{nhanBoDem(soThe)}</p>
      {/* Chỗ một cán bộ tìm lại việc "biến mất" của mình: một việc vừa sang `tam-dung` rời khỏi
          Kanban hoàn toàn, và §4.1 muốn thế. Không nói ra thì người giao việc kết luận nó đã bị xoá. */}
      <p className="ghi-chu">{ghiChuKanbanReNhanh(nhanTT)}</p>
    </>
  );
}

/**
 * Sổ theo dõi §4.3 — the register as the commune office's book of directive documents. Columns
 * EXACTLY `REGISTER_COLUMNS` (§4.3 order, the export's order too), after the `☐` column that only a
 * `task.delete` holder sees (W4).
 *
 * NAMES RESOLVED AS THE LIST RESOLVES THEM: units through `tenBoPhan`, staff through the directory
 * (`nhanCanBoNgan` — the name when known, the code otherwise, never blank). Documents come from the
 * row itself (`include=documents`); a row without the block says so instead of drawing three `—`.
 *
 * Read-only: nothing is edited here; the code opens the drawer, as on the list.
 */
export function BangSoTheoDoi({
  nhiemVu,
  danhMuc,
  danhBa = null,
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
    <div className={TABLE_IN_CARD} role="region" aria-label="Sổ theo dõi nhiệm vụ" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            {selection !== null && (
              <th scope="col">
                <span className="an-thi-giac">Chọn</span>
              </th>
            )}
            {REGISTER_COLUMNS.map((c) => (
              <th key={c} scope="col">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {nhiemVu.map((n) => {
            const o = oHan(n.due_at, bayGio);
            const docs = registerRowDocuments(n);
            return (
              <tr
                key={n.code}
                data-tre-han={o.phanTre !== "" ? "" : undefined}
                className={o.phanTre !== "" ? "dong-qua-han" : undefined}
              >
                {selection !== null && (
                  <td>
                    <input
                      type="checkbox"
                      aria-label={selectLabel(n.code)}
                      checked={selection.selected.has(n.code)}
                      disabled={selection.disabled}
                      onChange={() => selection.toggle(n)}
                    />
                  </td>
                )}
                <td className="ma-muc">
                  <button
                    type="button"
                    className="nut-phu"
                    onClick={() => moNhiemVu(n)}
                    aria-expanded={n.code === maDangMo}
                  >
                    {n.code}
                  </button>
                </td>
                <td>
                  {n.title}
                  {n.description !== "" && <span className="dong-phu">{n.description}</span>}
                  {n.bloc !== "" && (
                    <span className="chip chip-ngung">{nhanDanhMuc(danhMuc.khoi, n.bloc)}</span>
                  )}
                </td>
                <td>
                  {unit(n.unit)}
                  <span className="dong-phu">{nhanCanBoNgan(n.assignee, danhBa, CHUA_PHAN_CONG)}</span>
                </td>
                {MOI_NHOM_VAN_BAN.map((g, i) => (
                  <td key={g}>
                    {docs === null ? (
                      // Once per row, in the first document column — not three times.
                      i === 0 ? <span className="thong-bao-loi">{REGISTER_DOCS_MISSING}</span> : O_TRONG
                    ) : docs.byGroup[g].length === 0 ? (
                      O_TRONG
                    ) : (
                      <ul>
                        {docs.byGroup[g].map((v) => (
                          <li key={v.id}>
                            {dongVanBan(v.reference, v.date)}
                            {v.summary !== "" && <span className="dong-phu">{v.summary}</span>}
                          </li>
                        ))}
                      </ul>
                    )}
                    {docs !== null && docs.unknown > 0 && i === MOI_NHOM_VAN_BAN.length - 1 && (
                      <span className="dong-phu">{REGISTER_UNKNOWN_GROUP}</span>
                    )}
                  </td>
                ))}
                <td>
                  {o.ngay}
                  {o.phanTre !== "" && <span className="nhan-lech"> {o.phanTre}</span>}
                </td>
                <td>{n.result_summary === "" ? O_TRONG : n.result_summary}</td>
                <td>{n.note === "" ? O_TRONG : n.note}</td>
                <td>{n.leader_approved ? APPROVAL_TICKED : APPROVAL_UNTICKED}</td>
                <td>{n.superior_acknowledged ? APPROVAL_TICKED : APPROVAL_UNTICKED}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Một thẻ nhiệm vụ §4.1.
 *
 * MỨC ƯU TIÊN HIỆN THÀNH CHỮ. Đặc tả mã hoá nó bằng màu viền trái, mà màu một mình thì người không
 * phân biệt được màu đọc ra bằng không (a11y). Khi lớp CSS viền trái được thêm, nó là tín hiệu THỨ
 * HAI chồng lên chữ này chứ không thay chữ này.
 *
 * CHIP `{n} việc con` (#6) from `child_count`, which the SERVER counts — see `childCountLabel`.
 */
export function TheNhiemVu({
  nhiemVu,
  danhMuc,
  danhBa = null,
  unitNames = NO_UNIT_NAMES,
  nhanTT,
  bayGio,
  maDangMo,
  moNhiemVu,
  move = null,
  selection = null,
  onDragStart,
  onDragEnd,
}: {
  nhiemVu: petitions_nhiemVuRa;
  danhMuc: DanhMucNhiemVu;
  /** Xem `BangKanban`. Mã không có trong danh bạ thì hiện MÃ, không bao giờ để trống. */
  danhBa?: DanhBaTheoMa | null;
  /** See `BangKanban`. */
  unitNames?: ReadonlyMap<string, string>;
  nhanTT: BangNhanTrangThai;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
  /** See `BangKanban`. `null` = no drag, no menu. */
  move?: KanbanMove | null;
  /** `☐ Chọn` (§4.1) — see `TaskSelection`. `null` = no checkbox. */
  selection?: TaskSelection | null;
  onDragStart?: (task: petitions_nhiemVuRa) => void;
  onDragEnd?: () => void;
}) {
  const treHan = oHan(nhiemVu.due_at, bayGio).phanTre !== "";
  // Same list as the drawer's buttons. Empty (neither `task.update` nor the assignee, or nothing the
  // server lists) → neither a drag handle nor a menu: a control that can only be refused is not offered.
  const targets =
    move === null ? [] : clickableTransitions(nhiemVu, move.permissions, move.staffCode);
  const busy = move !== null && move.pending !== null;
  const pendingHere = move?.pending?.code === nhiemVu.code ? move.pending : null;
  const lastResult = move?.result ?? null;
  const refusal =
    lastResult !== null && !lastResult.ok && lastResult.code === nhiemVu.code ? lastResult : null;
  const canReturn =
    move !== null && reasonMove(nhiemVu, move.permissions, move.staffCode) === "return";

  return (
    <article
      className="the-nhiem-vu"
      aria-labelledby={`the-nhiem-vu-${nhiemVu.code}`}
      aria-busy={pendingHere !== null ? true : undefined}
      draggable={targets.length > 0 && !busy ? true : undefined}
      onDragStart={
        targets.length > 0 && !busy
          ? (e) => {
              e.dataTransfer.effectAllowed = "move";
              // Some browsers start no drag without a payload. The code is a business code,
              // not personal data (rule 3).
              e.dataTransfer.setData("text/plain", nhiemVu.code);
              onDragStart?.(nhiemVu);
            }
          : undefined
      }
      onDragEnd={targets.length > 0 ? () => onDragEnd?.() : undefined}
    >
      {selection !== null && (
        <label className="o-chon">
          <input
            type="checkbox"
            aria-label={selectLabel(nhiemVu.code)}
            checked={selection.selected.has(nhiemVu.code)}
            disabled={selection.disabled}
            onChange={() => selection.toggle(nhiemVu)}
          />{" "}
          Chọn
        </label>
      )}
      <p className="ma-muc">{nhiemVu.code}</p>
      <p id={`the-nhiem-vu-${nhiemVu.code}`} className="tieu-de-the">
        {nhiemVu.title}
      </p>
      {/* `Trễ 87 ngày` · `Hạn 20/12/2026` · `Hạn —` — cùng một hàm với dải hạn của drawer. */}
      <p className={treHan ? "nhan-lech" : "dong-phu"}>
        <Glyph icon={treHan ? AlarmClock : Clock} className="mr-1 inline size-3.5 align-[-2px]" />
        {nhanHanThe(nhiemVu.due_at, bayGio)}
      </p>
      <p className="dong-phu">
        {cardHolderText(nhiemVu, danhBa, unitNames)} ·{" "}
        {nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)}
      </p>
      {childCountLabel(nhiemVu.child_count) !== null && (
        <p>
          <span className="chip chip-ngung">{childCountLabel(nhiemVu.child_count)}</span>
        </p>
      )}
      {hoanThanhTreHan(nhiemVu.completed_at, nhiemVu.original_due_at) && (
        <p>
          <span className="chip chip-hoat-dong">{nhanHoanThanhTreHan(nhanTT)}</span>
        </p>
      )}
      {pendingHere !== null && (
        <p className="dong-phu">
          {kanbanMovePendingText(nhanTT, pendingHere.code, pendingHere.target)}
        </p>
      )}
      {/* THE SERVER'S SENTENCE, VERBATIM — the list of unfinished child tasks lives in it. */}
      {refusal !== null && (
        <p className="thong-bao-loi" role="alert">
          {kanbanMoveRefusedPrefix(nhanTT, refusal.target)} {refusal.message}
        </p>
      )}
      <div className="cum-nut-the">
        <button
          type="button"
          className="nut-phu"
          onClick={() => moNhiemVu(nhiemVu)}
          aria-expanded={nhiemVu.code === maDangMo}
        >
          {nhiemVu.code === maDangMo ? "Đang mở" : `Mở ${nhiemVu.code}`}
        </button>
        {move !== null && targets.length > 0 && (
          <KanbanMoveMenu
            code={nhiemVu.code}
            targets={targets}
            labels={nhanTT}
            disabled={busy}
            showReturnNote={canReturn}
            onMove={(t) => move.move(nhiemVu, t, "menu")}
          />
        )}
      </div>
    </article>
  );
}

/**
 * Tiêu đề một cột sắp được của bảng §4.2 — `aria-sort` cho trình đọc màn hình, một `<button>` để đi
 * được bằng bàn phím, và mũi tên bằng CHỮ (`↑`/`↓`/`⇅`) chứ không bằng màu. Cùng khuôn ô tiêu đề sắp
 * của danh bạ cán bộ (`features/cau-hinh/danh-ba-can-bo.tsx`, lớp `.nut-sap-xep` dùng chung).
 */
function OTieuDeSapXepNhiemVu({
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
  const mui = aria === "ascending" ? " ↑" : aria === "descending" ? " ↓" : " ⇅";
  return (
    <th scope="col" aria-sort={aria}>
      <button type="button" className="nut-sap-xep" onClick={() => doiSapXep(cot)}>
        {nhan}
        {mui}
      </button>
    </th>
  );
}

/**
 * Bảng Danh sách §4.2.
 *
 * HÀNG QUÁ HẠN: lớp `.dong-qua-han` (`globals.css`), bám vào đúng điều kiện `data-tre-han` đang đánh
 * dấu. Màu không phải tín hiệu duy nhất — cùng dòng ấy cột Hạn có chữ `(trễ N ngày)`.
 *
 * CỘT `Ngày giao` KHÔNG CÓ TRONG BẢNG CỘT §4.2, và nó có mặt vì một lý do: §4.2 đòi cột sắp được, mà
 * máy chủ chỉ sắp được theo `code` và `created_at`. Một cách sắp không có cột nào hiện giá trị của nó
 * là một thứ tự cán bộ không kiểm được bằng mắt.
 *
 * CỘT Ô TICK `☐` (§4.2) — CHỈ khi phiên cầm `task.delete` (`selection`, mặc định `null`). ĐỔI CHIỀU
 * CÓ CHỦ Ý 28/09/2026 (W4): trước đó không có cột này vì `Xoá đã chọn` chưa dựng; một ô tick không
 * dẫn tới thao tác nào mời cán bộ chọn hai mươi dòng rồi không tìm thấy nút — nên vẫn KHÔNG vẽ nó cho
 * tài khoản không xoá được.
 *
 * CHIP `{n} việc con` (#6) under the title, from the server's `child_count` — never counted from
 * the page on screen, which would give a number that depends on the page.
 *
 * `Hạn` IS SORTABLE (#13, `due_at`); tasks without a deadline stay last in both directions, and
 * the screen says so under the table (`NO_DEADLINE_LAST_NOTE`). `Tên việc` and `Ưu tiên` are sortable
 * too since backend P9 (`title`, `priority` — the commune's catalogue order, no-priority rows last,
 * `NO_PRIORITY_LAST_NOTE`).
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
  /** Bấm một tiêu đề sắp được. Bên gọi đưa về trang đầu — xem `bamSapXep`. */
  doiSapXep: (cot: CotSapXepNhiemVu) => void;
  /** The `☐` column — see `TaskSelection`. `null` = no column. */
  selection?: TaskSelection | null;
  /**
   * What an empty page shows. The screen passes "nothing yet" or "nothing under these filters"
   * (spec §8b) — a choice it makes from its own filter state; this table never guesses. Absent =
   * the filter sentence `SO_RONG`.
   */
  empty?: ReactNode;
}) {
  if (nhiemVu.length === 0) return empty ?? <EmptyState icon={ClipboardList} title={SO_RONG} />;

  return (
    <div className={TABLE_IN_CARD} role="region" aria-label="Sổ nhiệm vụ" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            {selection !== null && (
              <th scope="col">
                <span className="an-thi-giac">Chọn</span>
              </th>
            )}
            <OTieuDeSapXepNhiemVu
              cot="code"
              nhan={NHAN_COT_MA}
              sapXep={sapXep}
              doiSapXep={doiSapXep}
            />
            <OTieuDeSapXepNhiemVu
              cot="title"
              nhan={TITLE_COLUMN_LABEL}
              sapXep={sapXep}
              doiSapXep={doiSapXep}
            />
            <OTieuDeSapXepNhiemVu
              cot="created_at"
              nhan={NHAN_COT_NGAY_GIAO}
              sapXep={sapXep}
              doiSapXep={doiSapXep}
            />
            <th scope="col">Người thực hiện</th>
            <th scope="col">Bộ phận</th>
            <OTieuDeSapXepNhiemVu
              cot="priority"
              nhan={PRIORITY_COLUMN_LABEL}
              sapXep={sapXep}
              doiSapXep={doiSapXep}
            />
            <OTieuDeSapXepNhiemVu
              cot="due_at"
              nhan={DUE_COLUMN_LABEL}
              sapXep={sapXep}
              doiSapXep={doiSapXep}
            />
            <th scope="col">Trạng thái</th>
            <th scope="col">
              <span className="an-thi-giac">Thao tác</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {nhiemVu.map((n) => {
            const o = oHan(n.due_at, bayGio);
            return (
              <tr
                key={n.code}
                // SUY RA từ `due_at` so với bây giờ — cùng phép `oHan` cột Hạn dùng, không đọc cột
                // cờ nào (luật 10, bất biến 3). Nên nền hồng và chữ `(trễ N ngày)` không lệch nhau.
                data-tre-han={o.phanTre !== "" ? "" : undefined}
                className={o.phanTre !== "" ? "dong-qua-han" : undefined}
              >
                {selection !== null && (
                  <td>
                    <input
                      type="checkbox"
                      aria-label={selectLabel(n.code)}
                      checked={selection.selected.has(n.code)}
                      disabled={selection.disabled}
                      onChange={() => selection.toggle(n)}
                    />
                  </td>
                )}
                <td className="ma-muc">{n.code}</td>
                <td>
                  {n.title}
                  <span className="dong-phu">{nhanNguonGiao(n.source)}</span>
                  {childCountLabel(n.child_count) !== null && (
                    <span className="chip chip-ngung">{childCountLabel(n.child_count)}</span>
                  )}
                </td>
                <td>
                  <time dateTime={n.created_at}>{nhanNgay(n.created_at)}</time>
                </td>
                <td>{nhanCanBoNgan(n.assignee, danhBa, CHUA_PHAN_CONG)}</td>
                <td>{n.unit === "" ? O_TRONG : (tenBoPhan.get(n.unit) ?? n.unit)}</td>
                <td>{nhanDanhMuc(danhMuc.mucUuTien, n.priority)}</td>
                <td>
                  {o.ngay}
                  {/* PHẦN TRỄ TÁCH RIÊNG để tô đỏ, đúng §4.2. `oHan` trả hai mảnh sẵn, nên ở đây
                      không có phép cắt chuỗi nào. AlarmClock = the spec's overdue icon, beside the
                      words (never instead of them). */}
                  {o.phanTre !== "" && (
                    <span className="nhan-lech">
                      {" "}
                      <Glyph icon={AlarmClock} className="inline size-3.5 align-[-2px]" />
                      {o.phanTre}
                    </span>
                  )}
                </td>
                <td>
                  <TaskStatusBadge status={n.status}>{nhanTrangThai(nhanTT, n.status)}</TaskStatusBadge>
                  {hoanThanhTreHan(n.completed_at, n.original_due_at) && (
                    <span className="chip chip-hoat-dong">{nhanHoanThanhTreHan(nhanTT)}</span>
                  )}
                </td>
                <td className="o-thao-tac">
                  <button
                    type="button"
                    className="nut-phu"
                    onClick={() => moNhiemVu(n)}
                    aria-expanded={n.code === maDangMo}
                  >
                    {n.code === maDangMo ? "Đang mở" : `Mở ${n.code}`}
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** Id of the detail's heading — the dialog's accessible name (`LargeDialog titleId`). */
export const TASK_DETAIL_TITLE_ID = "tieu-de-chi-tiet-nhiem-vu";

/** The three action tabs of the detail (ADR 0068 §Sửa đổi 05/10/2026 #3, #4). */
export type TaskDetailTab = "view" | "edit" | "delete";

const TASK_DETAIL_TABS: readonly { id: TaskDetailTab; label: string; icon: LucideIcon }[] = [
  { id: "view", label: "Xem chi tiết", icon: Eye },
  { id: "edit", label: "Chỉnh sửa", icon: Pencil },
  { id: "delete", label: "Xoá", icon: Trash2 },
];

/** What the `Chỉnh sửa` tab holds — said once, so the officer knows each part saves on its own. */
export const TASK_EDIT_TAB_NOTE =
  "Mỗi phần dưới đây lưu riêng. Trạng thái, giao việc và lùi hạn nằm ở tab Xem chi tiết.";

/** Feedback of the `Sao chép liên kết` rail button. The link holds the register code only. */
export const COPY_LINK_DONE = "Đã sao chép liên kết tới nhiệm vụ.";
export function copyLinkFailedText(url: string): string {
  return `Không sao chép được. Liên kết: ${url}`;
}

/** Step chips (§5.2). Never buttons: a chip is not a way to move the task (ADR 0068 #3). */
const STEP_CHIP =
  "inline-flex h-8 items-center rounded-full border border-solid px-3 text-[13px] leading-none whitespace-nowrap";
const STEP_CHIP_LOOK = {
  current: "border-accent-500 bg-accent-50 font-semibold text-ink-900",
  before: "border-transparent bg-surface-subtle font-medium text-ink-500",
  after: "border-line-strong bg-surface font-medium text-ink-500",
} as const;

/** Round secondary-action button of the right rail: 44px, icon only, named by `aria-label`. */
const RAIL_BUTTON = cn(
  "grid size-11 shrink-0 cursor-pointer place-items-center rounded-full border border-solid border-line bg-surface p-0 text-ink-700 no-underline",
  "transition-[background-color,border-color,color] duration-150 hover:border-accent-500 hover:bg-accent-50 hover:text-ink-900",
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 [&_svg]:size-[18px]",
);

/** Initials for the assignee avatar, from the directory's full name; `""` when there is no name. */
export function staffInitials(fullName: string): string {
  const words = fullName.trim().split(/\s+/).filter((w) => w !== "");
  if (words.length === 0) return "";
  const first = words[0]!.charAt(0);
  const last = words.length > 1 ? words[words.length - 1]!.charAt(0) : "";
  return `${first}${last}`.toLocaleUpperCase("vi");
}

function RailButton({
  label,
  icon,
  onClick,
}: {
  label: string;
  icon: LucideIcon;
  onClick: () => void;
}) {
  // `aria-label`, not `title`: the Tooltip repeats the name for a mouse user, and a `title` too
  // would draw a second, native bubble over it.
  return (
    <Tooltip content={label} side="left">
      <button type="button" className={RAIL_BUTTON} aria-label={label} onClick={onClick}>
        <Glyph icon={icon} />
      </button>
    </Tooltip>
  );
}

/**
 * Chi tiết §5 — dải bước §5.2, ba ô tóm tắt §5.3, hai hạn §5.6, đổi trạng thái §6, đề nghị lùi
 * hạn §5.8 và xoá §11.5 — DRAWN AS THE CONTENT OF THE LARGE DIALOG (ADR 0068 §Sửa đổi 05/10/2026
 * #3; the spec's DetailDrawer "lớp phủ gần toàn màn hình", `docs/ui-ux/02-nhiem-vu.md:134-136`).
 * The caller wraps it in `LargeDialog`; on its own it renders as a block, which is how the tests
 * read it.
 *
 *   header      `[mã] tiêu đề` · `Tạo bởi … · lúc …` · ✕ · tabs Xem chi tiết / Chỉnh sửa / Xoá
 *   view        status card (pill, step chips, branch row, last update, the allowed moves) · two
 *               columns from 1024px: information left, Nhật ký & Trao đổi right
 *   edit        the EXISTING edit blocks only — deadline (`DueEditBlock`) or the document block's
 *               `✎ Sửa` (`Theo văn bản`), and `Việc cha`. No general edit form is invented
 *   delete      the EXISTING soft delete with a mandatory reason (`task.delete`, rule 7)
 *   rail        secondary actions only, round icon buttons with a tooltip; every main action
 *               stays a button with words (ADR 0068 §11)
 *
 * ALL THREE PANELS ARE MOUNTED, the inactive ones `hidden`: a half-typed reason or a half-edited
 * deadline survives a look at the other tab, and every block still exists exactly ONCE in the page
 * — so no field id is drawn twice.
 *
 * EVERY GATE IS TODAY'S: `canMoveTask`, `clickableTransitions`, `reasonMove`, `lacksApprovalFor`,
 * `canShowAssignment`, `canWriteLogEntry`, and `quyen.*`. The `Chỉnh sửa` tab shows with
 * `task.update` (every block in it is behind that key), `Xoá` with `task.delete`. Hiding is
 * convenience; each route checks the key itself (rule 5, forbidden #1).
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
  dangGui,
  loiGhi,
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
  openTaskByCode,
  saveParent,
  addChild,
  reassign,
  initialTab = "view",
}: {
  /** Tab open on first render. Tests pick one; the screen always opens on `view`. */
  initialTab?: TaskDetailTab;
  /**
   * `POST /api/v1/tasks/{ma}/assignment` — the §5.7 block. The caller refreshes the drawer, the
   * timeline and the register on success, then hands the `KetQua` back so the block can show a
   * refusal verbatim. REQUIRED: a caller that forgets it gets a red `tsc`.
   */
  reassign: (body: petitions_taskAssignmentIn) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Changes whenever this task's pending extension requests may have changed. */
  extensionRefreshKey: string;
  /** A decision in the drawer's block succeeded. */
  onExtensionDecided: (taskCode: string) => void;
  /** Open another task's drawer from a register row (a child). */
  openTask: (task: petitions_nhiemVuRa) => void;
  /** Open another task's drawer by register code (the parent). */
  openTaskByCode: (code: string) => Promise<KetQua<unknown>>;
  /** `PATCH /api/v1/tasks/{ma}` with `{ parent }` — the `Việc cha` field. */
  saveParent: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  /**
   * `+ Thêm việc con`, or `null` without `task.create`. REQUIRED, not optional: a caller that
   * forgets it gets a red `tsc`, not a drawer that silently lost the button.
   */
  addChild: {
    readonly open: boolean;
    readonly toggle: () => void;
    readonly form: ReactNode;
    /** `Đã giao việc con NV25.` after a success, else `null`. */
    readonly created: string | null;
  } | null;
  nhiemVu: petitions_nhiemVuRa;
  /**
   * Khối văn bản §5.4, đọc từ TUYẾN CHI TIẾT — không bao giờ từ `nhiemVu.documents` của một dòng
   * sổ, vì ở đó trường ấy vắng mặt có chủ ý. Xem `chuyenDrawer`.
   */
  vanBan: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  danhMuc: DanhMucNhiemVu;
  /**
   * Nhãn của xã cho dải bước và nút chuyển. DẢI BƯỚC GIỮ THỨ TỰ VÒNG ĐỜI §6, không theo `order`:
   * nó vẽ một đường đi (mới giao → … → hoàn thành), và xếp lại nó theo sở thích trình bày là vẽ
   * một vòng đời không có thật.
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
  dangGui: boolean;
  loiGhi: string | null;
  dong: () => void;
  doiTrangThai: (trangThai: string, ghiChu?: string) => void;
  xoa: (lyDo: string) => void;
  guiDeNghiLuiHan: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  quyetDinh: (
    deNghiID: string,
    duyet: boolean,
    ghiChu?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  /** PATCH /api/v1/tasks/{ma} của nút `✎ Sửa` §5.4. Bên gọi cập nhật drawer khi thành công. */
  suaKhoiVanBan: (than: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** GET /api/v1/tasks/{ma} — đọc lại ngay trước một lần lưu CÓ `documents`. */
  docLaiChiTiet: () => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [ghiChuChuyen, datGhiChuChuyen] = useState("");
  const [lyDoXoa, datLyDoXoa] = useState("");
  // The tab without `task.update` / `task.delete` does not exist; never leave the screen on one.
  const [tabChosen, setTab] = useState<TaskDetailTab>(initialTab);
  const [parentError, setParentError] = useState<string | null>(null);
  const [copyNote, setCopyNote] = useState<string | null>(null);
  // An element to focus once the `view` panel is shown again (rail buttons pointing into it).
  const pendingFocus = useRef<string | null>(null);

  const o = oHan(nhiemVu.due_at, bayGio);
  const giaiThich = cauGiaiThichTrangThai(nhiemVu.status);
  const danhBaMa = danhBaChoNhatKy(danhBa);
  // THE SERVER'S LIST (`allowed_transitions`, 3b2330b) — no second copy of the lifecycle here.
  // Shared with the Kanban menu — one list, so the two cannot offer different steps.
  const buocBamDuoc = clickableTransitions(nhiemVu, quyen, maNguoiDangNhap);
  // The return from review and the reopen are NOT `Chuyển sang …` buttons: both carry a mandatory
  // reason, so they have their own form. In the plain row a click would send an empty note.
  const reason = reasonMove(nhiemVu, quyen, maNguoiDangNhap);
  // The status block: `task.update`, or this task's assignee (ea55113). Convenience — the route
  // decides again on the row.
  const showStatusBlock = canMoveTask(quyen, nhiemVu, maNguoiDangNhap);
  // §5.7 — `task.assign` and a task that is not terminal. Convenience; the route checks both.
  const showAssignment = canShowAssignment(quyen, nhiemVu.status);
  // Assignee / `task.update` / assigner / creator — convenience; the row decides.
  const canWriteLog = canWriteLogEntry(quyen, nhiemVu, maNguoiDangNhap);
  const theoVanBan = coKhoiVanBanChiDao(nhiemVu.type);

  const tabs = TASK_DETAIL_TABS.filter(
    (t) => t.id === "view" || (t.id === "edit" ? quyen.capNhat : quyen.xoa),
  );
  const tab: TaskDetailTab = tabs.some((t) => t.id === tabChosen) ? tabChosen : "view";

  useEffect(() => {
    const id = pendingFocus.current;
    if (id === null || tab !== "view") return;
    pendingFocus.current = null;
    document.getElementById(id)?.focus();
  }, [tab]);

  /** Focus `id` inside the `view` panel, switching to it first when another tab is open. */
  function focusInView(id: string): void {
    if (tab === "view") {
      document.getElementById(id)?.focus();
      return;
    }
    pendingFocus.current = id;
    setTab("view");
  }

  function copyLink(): void {
    const url = new URL(taskDetailHref(nhiemVu.code), window.location.origin).href;
    const clipboard = typeof navigator !== "undefined" ? navigator.clipboard : undefined;
    if (clipboard === undefined) {
      setCopyNote(copyLinkFailedText(url));
      return;
    }
    clipboard.writeText(url).then(
      () => setCopyNote(COPY_LINK_DONE),
      () => setCopyNote(copyLinkFailedText(url)),
    );
  }

  /** Arrow keys move between the tabs (WAI-ARIA tabs pattern); Tab leaves the tab list. */
  function onTabKey(e: KeyboardEvent<HTMLDivElement>): void {
    const i = tabs.findIndex((t) => t.id === tab);
    const next =
      e.key === "ArrowRight" ? (i + 1) % tabs.length
      : e.key === "ArrowLeft" ? (i - 1 + tabs.length) % tabs.length
      : e.key === "Home" ? 0
      : e.key === "End" ? tabs.length - 1
      : -1;
    if (next < 0) return;
    e.preventDefault();
    const id = tabs[next]!.id;
    setTab(id);
    document.getElementById(`task-detail-tab-${id}`)?.focus();
  }

  const unitName =
    nhiemVu.unit === "" ? CHUA_GIAO_BO_PHAN : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit);
  const assigneeName = nhiemVu.assignee === "" ? "" : (danhBaMa?.get(nhiemVu.assignee)?.full_name ?? "");
  const initials = staffInitials(assigneeName);
  const currentStep = (TRANG_THAI_CHINH as readonly string[]).indexOf(nhiemVu.status);
  const StatusIcon = taskStatusIcon(nhiemVu.status);
  const meetingLink =
    cauTuKetLuan(nhiemVu) !== null && nhiemVu.meeting_id !== undefined
      ? duongDanBienBan(nhiemVu.meeting_id)
      : null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col" aria-labelledby={TASK_DETAIL_TITLE_ID}>
      {/* ── HEADER STRIP: title, who created it, ✕, the three action tabs ─────────────────── */}
      <div className="shrink-0 border-b border-solid border-line bg-surface px-4 pt-3 md:px-6 md:pt-4">
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <h2
              id={TASK_DETAIL_TITLE_ID}
              className="m-0 text-lg leading-snug font-semibold text-ink-900 [overflow-wrap:anywhere]"
            >
              [{nhiemVu.code}] {nhiemVu.title}
            </h2>
            {/* Metadata the record carries (v2 §8b "Minh bạch"). `created_by` is a staff business
                code, shown as `Họ tên (CB-…)` when the directory knows it. */}
            <p className="m-0 mt-1 text-[13px] text-ink-500">
              Tạo bởi {nhanCanBoDrawer(nhiemVu.created_by, danhBaMa, O_TRONG)} ·{" "}
              <time dateTime={nhiemVu.created_at}>{nhanThoiDiem(nhiemVu.created_at)}</time>
            </p>
          </div>
          <IconButton label="Đóng chi tiết nhiệm vụ" type="button" variant="secondary" onClick={dong}>
            <Glyph icon={X} />
          </IconButton>
        </div>
        <TabList aria-label="Thao tác với nhiệm vụ" className="mt-2 border-b-0" onKeyDown={onTabKey}>
          {tabs.map((t) => (
            <Tab
              key={t.id}
              id={`task-detail-tab-${t.id}`}
              aria-controls={`task-detail-panel-${t.id}`}
              selected={tab === t.id}
              tabIndex={tab === t.id ? 0 : -1}
              icon={t.icon}
              className={cn(tab === t.id && "border-brand-500 text-ink-900 hover:text-ink-900")}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </Tab>
          ))}
        </TabList>
      </div>

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 md:p-6">
          {/* Above the panels: a refusal from any tab (a move, a delete) stays in sight. */}
          {loiGhi !== null && (
            <p className="thong-bao-loi mt-0" role="alert">
              {loiGhi}
            </p>
          )}
          {/* Always in the DOM: a live region inserted later is not always announced. */}
          <p className="an-thi-giac" role="status">
            {copyNote ?? ""}
          </p>
          {copyNote !== null && (
            <p className="m-0 mb-3 text-[13px] text-ink-500" aria-hidden="true">
              {copyNote}
            </p>
          )}

          {/* ══ XEM CHI TIẾT ═══════════════════════════════════════════════════════════════ */}
          <div
            id="task-detail-panel-view"
            role="tabpanel"
            aria-labelledby="task-detail-tab-view"
            hidden={tab !== "view"}
            className="flex flex-col gap-4"
          >
            {/* ── STATUS CARD: the state now, the lifecycle, and today's allowed moves ──────── */}
            <Card as="section" aria-label="Trạng thái nhiệm vụ" className="overflow-visible p-4 md:p-5">
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <span className="inline-flex h-10 items-center gap-2 rounded-full bg-accent-50 px-4 text-base font-semibold text-ink-900">
                  <Glyph icon={StatusIcon} className="size-5 shrink-0" />
                  {nhanTrangThai(nhanTT, nhiemVu.status)}
                </span>
                {/* No "by whom": the task carries no `updated_by`; the timeline names every actor. */}
                <span className="inline-flex items-center gap-1 text-[13px] text-ink-500">
                  <Glyph icon={History} className="size-3.5" />
                  Cập nhật gần nhất{" "}
                  <time dateTime={nhiemVu.updated_at}>{nhanThoiDiem(nhiemVu.updated_at)}</time>
                </span>
              </div>
              {giaiThich !== "" && <p className="m-0 mt-2 text-sm text-ink-700">{giaiThich}</p>}

              {/* ── DẢI BƯỚC §5.2 ───────────────────────────────────────────────────────────
                  THỜI GIAN ĐÃ Ở TRẠNG THÁI HIỆN DẤU GẠCH, không hiện số 0: mốc đổi trạng thái gần
                  nhất nằm trong nhật ký, và dải bước chưa đọc nhật ký để tính nó.
                  "before" is POSITION in the main order, not a claim the step was passed — a task
                  may go from `dang-thuc-hien` straight to `hoan-thanh` (ADR 0065 NV1); the chip
                  says no word about it. Chips are text, never moves. */}
              <ol
                aria-label="Các bước của vòng đời nhiệm vụ"
                className="m-0 mt-4 flex list-none flex-wrap items-center gap-2 p-0"
              >
                {TRANG_THAI_CHINH.map((ma, i) => {
                  const look = i === currentStep ? "current" : currentStep >= 0 && i < currentStep ? "before" : "after";
                  return (
                    <li key={ma} className="inline-flex items-center gap-1 text-[13px] text-ink-500">
                      <span
                        className={cn(STEP_CHIP, STEP_CHIP_LOOK[look])}
                        aria-current={look === "current" ? "step" : undefined}
                        data-step={look}
                      >
                        {nhanTrangThai(nhanTT, ma)}
                      </span>{" "}
                      {O_TRONG}
                    </li>
                  );
                })}
              </ol>
              {/* `Chuyển tiếp` IS NO LONGER A STATUS MOVE (owner decision 28/09/2026; `…/status`
                  answers 400). When the §5.7 block is shown, its branch chip is a button that MOVES
                  FOCUS to that block — no call is made here (user decision 2). Otherwise the chip
                  stays plain text: an old `chuyen-tiep` row keeps its label, lit, and stays terminal. */}
              <p className="m-0 mt-3 flex flex-wrap items-center gap-2 text-[13px] text-ink-500">
                Rẽ nhánh:{" "}
                {TRANG_THAI_RE_NHANH.map((ma) => {
                  const look = ma === nhiemVu.status ? "current" : "after";
                  const chip = (
                    <span
                      className={cn(STEP_CHIP, STEP_CHIP_LOOK[look])}
                      aria-current={look === "current" ? "true" : undefined}
                      data-step={look === "current" ? "current" : "branch"}
                    >
                      {nhanTrangThai(nhanTT, ma)}
                    </span>
                  );
                  return (
                    <span key={ma} className="inline-flex items-center gap-1">
                      {ma === "chuyen-tiep" && showAssignment ? (
                        <button
                          type="button"
                          className="cursor-pointer rounded-full border-0 bg-transparent p-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
                          aria-controls={ASSIGNMENT_UNIT_FIELD_ID}
                          aria-label={`${nhanTrangThai(nhanTT, ma)} — ${ASSIGNMENT_STEPPER_HINT}`}
                          onClick={() => document.getElementById(ASSIGNMENT_UNIT_FIELD_ID)?.focus()}
                        >
                          {chip}
                        </button>
                      ) : (
                        chip
                      )}{" "}
                      {O_TRONG}{" "}
                    </span>
                  );
                })}
              </p>

              {/* ── ĐỔI TRẠNG THÁI §6 ─────────────────────────────────────────────────────────
                  CHỈ VẼ NHỮNG BƯỚC MÁY CHỦ LIỆT KÊ trên chính dòng này (`allowed_transitions`).

                  BƯỚC `hoan-thanh` VẪN HIỆN KỂ CẢ KHI CÒN VIỆC CON: `child_count` đếm việc con CÒN
                  SỐNG, không đếm việc con CHƯA XONG, nên màn hình không biết bước ấy có bị chặn hay
                  không. Câu từ chối của máy chủ LIỆT KÊ MÃ việc con còn lại.

                  CẢ KHỐI ĐỨNG SAU `canMoveTask` — `task.update`, hoặc đúng người thực hiện của việc
                  này. Duyệt `cho-duyet` → `hoan-thanh`, mở lại và trả lại đòi THÊM `task.approve`
                  (`transitionNeedsApproval`); thiếu khoá ấy thì nút ẩn và câu dưới nói vì sao.
                  `dang-thuc-hien` → `hoan-thanh` thì không cần (ADR 0065 NV1). */}
              {showStatusBlock && (
                <div className="mt-4 border-t border-solid border-line pt-4">
                  <h4 className="m-0 mb-2 text-sm font-semibold text-ink-900">Chuyển trạng thái</h4>
                  {/* No plain step (e.g. `hoan-thanh`, whose only move is the reopen form below): no
                      note field and no empty button row. */}
                  {buocBamDuoc.length > 0 && (
                    <>
                      <div className="o-nhap">
                        <label htmlFor="ghi-chu-chuyen-trang-thai">Ghi chú (không bắt buộc)</label>
                        <input
                          id="ghi-chu-chuyen-trang-thai"
                          name="ghi-chu-chuyen-trang-thai"
                          value={ghiChuChuyen}
                          autoComplete="off"
                          onChange={(e) => datGhiChuChuyen(e.target.value)}
                        />
                      </div>
                      <div className="cum-nut">
                        {buocBamDuoc.map((t) => (
                          <button
                            key={t}
                            type="button"
                            className="nut-phu"
                            disabled={dangGui}
                            onClick={() => doiTrangThai(t, ghiChuChuyen.trim())}
                          >
                            Chuyển sang {nhanTrangThai(nhanTT, t)}
                          </button>
                        ))}
                      </div>
                    </>
                  )}
                  {lacksApprovalFor(nhiemVu, quyen, maNguoiDangNhap) && (
                    <p className="ghi-chu">{CAU_THIEU_QUYEN_DUYET_HOAN_THANH}</p>
                  )}
                  {nhiemVu.allowed_transitions.length === 0 && (
                    <p className="trang-thai-rong">Máy chủ không liệt kê lối ra nào khỏi trạng thái này.</p>
                  )}
                </div>
              )}

              {/* `key` theo mã: mở một nhiệm vụ khác thì lý do đang gõ dở không đi theo sang việc ấy. */}
              {reason !== null && (
                <KhoiTraLai
                  key={`${reason}-${nhiemVu.code}`}
                  kind={reason}
                  nhanTT={nhanTT}
                  dangGui={dangGui}
                  gui={doiTrangThai}
                />
              )}
            </Card>

            <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:items-start">
              {/* ── LEFT: information, documents, children, assignment, deadline requests ─── */}
              <div className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
                <Card as="section" aria-labelledby="task-detail-info" className="overflow-visible p-4 md:p-5">
                  <h3 id="task-detail-info" className="m-0 mb-4 text-[15px] font-semibold text-ink-900">
                    Thông tin nhiệm vụ
                  </h3>
                  <dl className="m-0 grid gap-x-6 gap-y-4 sm:grid-cols-2 [&_dd]:m-0 [&_dd]:mt-1 [&_dd]:text-sm [&_dd]:text-ink-900 [&_dt]:text-xs [&_dt]:font-medium [&_dt]:text-ink-500">
                    {/* §5.3 — HAI MẢNH, KHÔNG MỘT: `Trễ 87 ngày` **và** `Hạn 20/6/2026`. Chỉ hiện số
                        ngày trễ thì cán bộ không biết hạn là ngày nào; chỉ hiện ngày thì con số
                        phải tự nhẩm. Overdue is DERIVED from `due_at` and now (rule 10, inv. 3). */}
                    <div className="sm:col-span-2">
                      <dt>Hạn xử lý</dt>
                      <dd className="flex flex-wrap items-center gap-2 text-base! font-semibold">
                        {o.phanTre !== "" ? (
                          <>
                            <Badge tone="danger" icon={AlarmClock} className="nhan-lech">
                              {nhanHanThe(nhiemVu.due_at, bayGio)}
                            </Badge>
                            <span>Hạn {nhanNgay(nhiemVu.due_at)}</span>
                          </>
                        ) : (
                          nhanHanThe(nhiemVu.due_at, bayGio)
                        )}
                      </dd>
                    </div>

                    {/* §5.6 — HAI HẠN CẠNH NHAU. `HẠN BAN ĐẦU` không đổi khi gia hạn; tỷ lệ đúng
                        hạn §11.3 đếm theo nó. */}
                    <div>
                      <dt>Hạn ban đầu</dt>
                      <dd>{nhanNgay(nhiemVu.original_due_at)}</dd>
                    </div>

                    <div>
                      <dt>Mức ưu tiên</dt>
                      <dd>
                        <Badge tone="neutral" icon={Flag}>
                          {nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)}
                        </Badge>
                      </dd>
                    </div>

                    <div className="sm:col-span-2">
                      <dt>Đang giao cho</dt>
                      <dd className="flex items-center gap-3">
                        {/* Decorative: the name and the unit are the text beside it. */}
                        <span
                          aria-hidden="true"
                          className="grid size-9 shrink-0 place-items-center rounded-full bg-accent-100 text-[13px] font-semibold text-ink-900"
                        >
                          {initials !== "" ? initials : <Glyph icon={UserRound} className="size-[18px]" />}
                        </span>
                        <span className="min-w-0">
                          {unitName} · {nhanCanBoDrawer(nhiemVu.assignee, danhBaMa, CHUA_PHAN_CONG)}
                        </span>
                      </dd>
                    </div>

                    <div>
                      <dt>Lãnh đạo giao việc</dt>
                      <dd>{nhanCanBoDrawer(nhiemVu.assigner, danhBaMa, O_TRONG)}</dd>
                    </div>

                    {/* ADR 0068 §14: no form edits `progress` yet, so the stored figure sits in a
                        DISABLED field with a "?" (report 05/10/2026, NV-08). */}
                    <div>
                      <dt>
                        <label htmlFor="chi-tiet-tien-do">Tiến độ</label>
                      </dt>
                      <dd>
                        <PendingFeature info={TASK_PROGRESS_PENDING}>
                          <input
                            id="chi-tiet-tien-do"
                            type="text"
                            disabled
                            readOnly
                            value={`${nhiemVu.progress}%`}
                            size={6}
                          />
                        </PendingFeature>
                      </dd>
                    </div>

                    <div>
                      <dt>Loại nhiệm vụ</dt>
                      <dd>{nhanDanhMuc(danhMuc.loai, nhiemVu.type)}</dd>
                    </div>

                    <div>
                      <dt>Khối</dt>
                      <dd>{nhanDanhMuc(danhMuc.khoi, nhiemVu.bloc)}</dd>
                    </div>

                    <div className="sm:col-span-2">
                      <dt>Nguồn giao</dt>
                      <dd>
                        {nhanNguonGiao(nhiemVu.source)}
                        {/* §7.4 — LIÊN KẾT NGƯỢC VỀ BIÊN BẢN GỐC. Chỉ khi máy chủ nối được
                            (`meeting_id`), xem `cauTuKetLuan`. */}
                        {meetingLink !== null && (
                          <>
                            <br />
                            <Link href={meetingLink}>{cauTuKetLuan(nhiemVu)}</Link>
                          </>
                        )}
                      </dd>
                    </div>

                    {/* `Việc cha` READ here, for every reader; changing it is the `Chỉnh sửa`
                        tab's `ParentTaskField` (`task.update`). Opening the parent REPLACES the
                        dialog's history entry (`useTaskDialogUrl`). */}
                    <div className="sm:col-span-2">
                      <dt>{PARENT_TITLE}</dt>
                      <dd>
                        {nhiemVu.parent === "" ? (
                          PARENT_NONE
                        ) : (
                          <button
                            type="button"
                            className="nut-phu"
                            onClick={() => {
                              setParentError(null);
                              openTaskByCode(nhiemVu.parent).then((r) => {
                                if (!r.ok) setParentError(r.thongBao);
                              });
                            }}
                          >
                            Mở việc cha {nhiemVu.parent}
                          </button>
                        )}
                        {parentError !== null && (
                          <span className="thong-bao-loi mt-1 block" role="alert">
                            {parentError}
                          </span>
                        )}
                      </dd>
                    </div>

                    <div className="sm:col-span-2">
                      <dt>Mô tả nhiệm vụ</dt>
                      <dd className="whitespace-pre-line">{nhiemVu.description === "" ? O_TRONG : nhiemVu.description}</dd>
                    </div>

                    <div className="sm:col-span-2">
                      <dt>Tóm tắt kết quả thực hiện</dt>
                      <dd className="whitespace-pre-line">{nhiemVu.result_summary === "" ? O_TRONG : nhiemVu.result_summary}</dd>
                    </div>

                    <div className="sm:col-span-2">
                      <dt>Ghi chú</dt>
                      <dd className="whitespace-pre-line">{nhiemVu.note === "" ? O_TRONG : nhiemVu.note}</dd>
                    </div>

                    <div>
                      <dt>Lãnh đạo xã đã phê duyệt hoàn thành</dt>
                      <dd>{nhiemVu.leader_approved ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>
                    </div>

                    <div>
                      <dt>Cấp trên đã công nhận hoàn thành</dt>
                      <dd>{nhiemVu.superior_acknowledged ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>
                    </div>
                  </dl>
                  <p className="ghi-chu mb-0">{CHU_THICH_HAI_O_TICK}</p>
                </Card>

                {/* §5.4 — READ here. Its `✎ Sửa` (and with it the deadline of this type) is the
                    `Chỉnh sửa` tab: the block below is the same read, so the two never disagree.
                    Rẽ nhánh trên MÃ, không trên nhãn: xem `LOAI_THEO_VAN_BAN`. */}
                {theoVanBan && (
                  <Card as="section" aria-labelledby="task-detail-documents" className="p-4 md:p-5">
                    <h3
                      id="task-detail-documents"
                      tabIndex={-1}
                      className="m-0 mb-2 text-[15px] font-semibold text-ink-900 focus-visible:outline-2 focus-visible:outline-brand-500"
                    >
                      {TIEU_DE_KHOI_VAN_BAN}
                    </h3>
                    <DocKhoiVanBan tai={vanBan} />
                  </Card>
                )}

                {/* §5.10 `Nhiệm vụ con` (#6). `key` by code: another task's "Xem thêm" page must
                    not follow into this one. Opening a child REPLACES the history entry. */}
                <ChildTasks
                  key={`con-${nhiemVu.code}`}
                  parentCode={nhiemVu.code}
                  refreshKey={lanLamMoiNhatKy}
                  labels={nhanTT}
                  directory={danhBaMa}
                  now={bayGio}
                  openTask={openTask}
                />
                {addChild !== null && (
                  <div className="form-danh-muc">
                    <div className="cum-nut">
                      <button
                        type="button"
                        className="nut-phu"
                        aria-expanded={addChild.open}
                        onClick={addChild.toggle}
                      >
                        {addChild.open ? "Đóng biểu mẫu việc con" : ADD_CHILD_BUTTON}
                      </button>
                    </div>
                    {addChild.created !== null && <p role="status">{addChild.created}</p>}
                    {addChild.open && addChild.form}
                  </div>
                )}

                {/* §5.7 — hand the SAME task to another unit or person ("Chuyển tiếp"); changing the
                    officer who follows it is changing the assignee (ADR 0065 NV5). */}
                {showAssignment && (
                  <TaskAssignmentBlock
                    key={`giao-lai-${nhiemVu.code}`}
                    task={nhiemVu}
                    units={danhMuc.boPhan}
                    directory={danhBa}
                    labels={nhanTT}
                    reassign={reassign}
                  />
                )}

                <KhoiLuiHan
                  coQuyenDeNghi={quyen.capNhat}
                  hanHienTai={nhiemVu.due_at}
                  dangGui={dangGui}
                  guiDeNghi={guiDeNghiLuiHan}
                />

                {/* §5.8 — this task's pending requests and, for the recorded assigner holding
                    `task.extend`, Duyệt / Từ chối (#11). Same row gate and route as the queue. */}
                <TaskExtensionBlock
                  key={`lui-han-${nhiemVu.code}`}
                  taskCode={nhiemVu.code}
                  assigner={nhiemVu.assigner}
                  directory={danhBaMa}
                  sessionStaffCode={maNguoiDangNhap}
                  // LAYER ONE — `task.extend` of the SESSION. Layer two (ADR 0038) runs inside the
                  // row gate, and the server decides both again in the transaction.
                  canApproveExtension={quyen.duyetGiaHan}
                  refreshKey={extensionRefreshKey}
                  decide={quyetDinh}
                  onDecided={onExtensionDecided}
                />
              </div>

              {/* ── RIGHT: Nhật ký & Trao đổi §5.9 — the existing form above the timeline ─────── */}
              <div className="min-w-0 [&>.khoi-chi-tiet]:m-0">
                <NhatKyNhiemVu
                  key={nhiemVu.code}
                  maNhiemVu={nhiemVu.code}
                  nhanTT={nhanTT}
                  danhBa={danhBaMa}
                  tenBoPhan={tenBoPhan}
                  lanLamMoi={lanLamMoiNhatKy}
                  canWrite={canWriteLog}
                />
              </div>
            </div>
          </div>

          {/* ══ CHỈNH SỬA — the existing edit blocks, nothing invented ═══════════════════════ */}
          {quyen.capNhat && (
            <div
              id="task-detail-panel-edit"
              role="tabpanel"
              aria-labelledby="task-detail-tab-edit"
              hidden={tab !== "edit"}
              className="flex flex-col gap-4 [&>*]:my-0"
            >
              <p className="ghi-chu">{TASK_EDIT_TAB_NOTE}</p>
              {/* ADR 0065 NV4 — the deadline is editable on EVERY type. `Theo văn bản` edits it
                  inside the document block's `✎ Sửa`; every other type gets `DueEditBlock`. Both
                  behind `task.update`, which the PATCH route checks itself. `key` by code: another
                  task's open edit form must not appear on this one. */}
              {theoVanBan ? (
                <KhoiVanBanChiDao
                  key={nhiemVu.code}
                  tai={vanBan}
                  nhiemVu={nhiemVu}
                  coQuyenSua={quyen.capNhat}
                  luu={suaKhoiVanBan}
                  docLai={docLaiChiTiet}
                />
              ) : (
                <DueEditBlock
                  key={`han-${nhiemVu.code}`}
                  nhiemVu={nhiemVu}
                  save={suaKhoiVanBan}
                  reread={docLaiChiTiet}
                />
              )}
              {/* §5.4 `Việc cha` (#10). */}
              <ParentTaskField
                key={`cha-${nhiemVu.code}`}
                code={nhiemVu.code}
                parent={nhiemVu.parent}
                canEdit={quyen.capNhat}
                save={saveParent}
                openParent={openTaskByCode}
              />
            </div>
          )}

          {/* ══ XOÁ MỀM §11.5 — `task.delete` ═══════════════════════════════════════════════ */}
          {quyen.xoa && (
            <div
              id="task-detail-panel-delete"
              role="tabpanel"
              aria-labelledby="task-detail-tab-delete"
              hidden={tab !== "delete"}
            >
              <ConfirmDialog
                as="form"
                tone="danger"
                icon={Trash2}
                title={`Xoá nhiệm vụ ${nhiemVu.code} khỏi sổ?`}
                onSubmit={(e: FormEvent<HTMLFormElement>) => {
                  e.preventDefault();
                  if (lyDoXoa.trim() !== "") xoa(lyDoXoa.trim());
                }}
                actions={
                  <>
                    <button type="button" className="nut-phu" onClick={() => setTab("view")}>
                      {NHAN_NUT_HUY}
                    </button>
                    <button
                      type="submit"
                      className="nut-xoa"
                      disabled={dangGui || lyDoXoa.trim() === ""}
                    >
                      <Glyph icon={Trash2} className="mr-1.5 inline size-[18px] align-[-4px]" />
                      Xoá nhiệm vụ
                    </button>
                  </>
                }
              >
                <p className="m-0">
                  Xoá mềm: dòng ở lại cùng người xoá và lý do, mã sổ đã cấp thì không bao giờ cấp
                  lại. Nhiệm vụ còn việc con chưa xoá thì máy chủ từ chối và nói rõ còn mấy việc.
                </p>
                <div className="o-nhap">
                  <label htmlFor="ly-do-xoa-nhiem-vu">Lý do xoá (bắt buộc)</label>
                  <input
                    id="ly-do-xoa-nhiem-vu"
                    name="ly-do-xoa-nhiem-vu"
                    value={lyDoXoa}
                    autoComplete="off"
                    onChange={(e) => datLyDoXoa(e.target.value)}
                  />
                </div>
              </ConfirmDialog>
            </div>
          )}
        </div>

        {/* ── RIGHT RAIL: secondary actions only, each with a tooltip and an accessible name.
            Moves, assignment and extensions stay buttons with words (ADR 0068 §11). Above the
            body on a phone, a column on the right from 768px. */}
        <div
          role="group"
          aria-label="Thao tác phụ"
          className="order-first flex shrink-0 gap-2 border-b border-solid border-line bg-surface p-2 md:order-none md:flex-col md:border-b-0 md:border-l"
        >
          <RailButton label="Sao chép liên kết" icon={Link2} onClick={copyLink} />
          {meetingLink !== null && (
            <Tooltip content="Mở biên bản nguồn" side="left">
              <Link href={meetingLink} className={RAIL_BUTTON} aria-label="Mở biên bản nguồn">
                <Glyph icon={Landmark} />
              </Link>
            </Tooltip>
          )}
          {theoVanBan && (
            <RailButton
              label="Xem văn bản chỉ đạo"
              icon={FileText}
              onClick={() => focusInView("task-detail-documents")}
            />
          )}
          {canWriteLog && (
            <RailButton
              label="Đính kèm vào nhật ký"
              icon={Paperclip}
              onClick={() => focusInView(`ghi-nhat-ky-${nhiemVu.code}-dinh-kem`)}
            />
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * "Trả lại để làm tiếp" — `cho-duyet` → `dang-thuc-hien`, kèm LÝ DO BẮT BUỘC (`yeuCauTraLai`).
 *
 * MỘT Ô RIÊNG, KHÔNG DÙNG Ô `Ghi chú (không bắt buộc)` CỦA KHỐI TRÊN: cùng trường `note` trên dây,
 * nhưng một ô ghi "không bắt buộc" không thể đồng thời là lý do bắt buộc của bước này. Nút khoá khi ô
 * rỗng hoặc toàn khoảng trắng — cùng phép cắt máy chủ dùng (`KiemLyDoTraLai`).
 *
 * Bên gọi chỉ vẽ khối này khi máy chủ liệt kê bước ấy VÀ tài khoản đi được nó (`reasonMove`). Câu
 * từ chối của máy chủ — 403 thiếu quyền, 400 thiếu lý do — ra nguyên văn ở `loiGhi` của drawer.
 *
 * `kind="reopen"` (28/09/2026): the SAME form for `hoan-thanh` → `dang-thuc-hien`. Same target, same
 * trimmed mandatory note (`yeuCauTraLai`), own title, label and field id. The server is about to require
 * the note too; this screen requires it already. REQUIRED prop: no default kind to fall into.
 */
export function KhoiTraLai({
  kind,
  nhanTT,
  dangGui,
  gui,
}: {
  kind: ReasonMove;
  /** Nhãn của xã — tên trạng thái đích trong câu giải thích. */
  nhanTT: BangNhanTrangThai;
  dangGui: boolean;
  gui: (trangThai: string, ghiChu: string) => void;
}) {
  const [lyDo, datLyDo] = useState("");
  const yeuCau = yeuCauTraLai(lyDo);
  const title = kind === "reopen" ? REOPEN_BUTTON : NHAN_NUT_TRA_LAI;
  const fieldId = kind === "reopen" ? "ly-do-mo-lai" : "ly-do-tra-lai";

  return (
    <form
      className="form-danh-muc"
      onSubmit={(e) => {
        e.preventDefault();
        if (yeuCau !== null) gui(yeuCau.trangThai, yeuCau.ghiChu);
      }}
    >
      <h4>{title}</h4>
      <p className="ghi-chu">{kind === "reopen" ? reopenNote(nhanTT) : ghiChuTraLai(nhanTT)}</p>
      <div className="o-nhap">
        <label htmlFor={fieldId}>{kind === "reopen" ? REOPEN_REASON_LABEL : NHAN_LY_DO_TRA_LAI}</label>
        <textarea
          id={fieldId}
          name={fieldId}
          rows={3}
          required
          maxLength={LY_DO_TRA_LAI_TOI_DA}
          value={lyDo}
          onChange={(e) => datLyDo(e.target.value)}
        />
      </div>
      <button type="submit" className="nut-phu" disabled={dangGui || yeuCau === null}>
        {title}
      </button>
    </form>
  );
}

/**
 * Ba nhóm văn bản của §5.4 — CHỈ ĐỌC.
 *
 * BA PHA, BA MÀN HÌNH KHÁC NHAU, và nhập hai pha đầu vào pha thứ ba là nói dối:
 *
 *   dangTai   câu `Đang tải…` — chưa biết gì
 *   loi       câu lỗi + câu máy chủ nguyên văn — KHÔNG vẽ ba nhóm, vì ba nhóm `—` đọc ra là
 *             "nhiệm vụ không có văn bản", điều chưa ai ghi
 *   xong      đủ ba nhóm; nhóm rỗng là `—` — lúc này máy chủ ĐÃ NÓI nhóm ấy rỗng
 *
 * Mỗi văn bản: `{số, ký hiệu} · {ngày}` rồi trích yếu ở dòng phụ, đúng §5.4.
 *
 * NÚT `✎ SỬA` Ở GÓC KHỐI, và nó KHOÁ ở hai pha đầu kèm lý do (`lyDoKhoaSua`): `documents` trên PATCH
 * là THAY CẢ TẬP, nên sửa từ một tập chưa đọc xong là gỡ mất những dòng cán bộ chưa từng thấy.
 * Tiêu điểm trở về nút ấy sau `Lưu` lẫn `Huỷ` — form biến mất, và tiêu điểm không được rơi về đầu
 * trang.
 */
export function KhoiVanBanChiDao({
  tai,
  nhiemVu,
  coQuyenSua,
  luu,
  docLai,
}: {
  tai: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  nhiemVu: petitions_nhiemVuRa;
  /** `task.update` — khoá của `PATCH /api/v1/tasks/{ma}`. Thiếu thì KHÔNG vẽ nút `✎ Sửa`. */
  coQuyenSua: boolean;
  luu: (than: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  docLai: () => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [dangSua, datDangSua] = useState(false);
  const nutSua = useRef<HTMLButtonElement>(null);
  const traTieuDiem = useRef(false);

  useEffect(() => {
    if (dangSua || !traTieuDiem.current) return;
    traTieuDiem.current = false;
    nutSua.current?.focus();
  }, [dangSua]);

  const lyDoKhoa = lyDoKhoaSua(tai);
  // PHẢI CÓ ĐỦ BA ĐIỀU cùng lúc. Khối rời pha `xong` giữa chừng (đọc lại hỏng) thì form tự ẩn —
  // không sửa tiếp trên một tập máy chủ vừa không xác nhận được.
  const hienForm = coQuyenSua && dangSua && tai.pha === "xong" && lyDoKhoa === null;

  function thoatSua() {
    traTieuDiem.current = true;
    datDangSua(false);
  }

  return (
    <div className="form-danh-muc" aria-labelledby="tieu-de-khoi-van-ban">
      <div className="dau-khoi-chi-tiet">
        <h4 id="tieu-de-khoi-van-ban">{TIEU_DE_KHOI_VAN_BAN}</h4>
        {!hienForm && coQuyenSua && (
          <button
            ref={nutSua}
            type="button"
            className="nut-phu"
            aria-label={`Sửa ${TIEU_DE_KHOI_VAN_BAN.toLowerCase()}`}
            aria-describedby={lyDoKhoa !== null ? "ly-do-khoa-sua-van-ban" : undefined}
            disabled={lyDoKhoa !== null}
            onClick={() => datDangSua(true)}
          >
            {/* `✎` was a glyph inside `NHAN_NUT_SUA`; that constant is shared with the Nội dung
                screen, so this button draws the lucide icon and the word itself. */}
            <Glyph icon={Pencil} className="size-[18px]" />
            Sửa
          </button>
        )}
      </div>
      {!hienForm && coQuyenSua && lyDoKhoa !== null && (
        <p id="ly-do-khoa-sua-van-ban" className="ghi-chu">
          {lyDoKhoa}
        </p>
      )}

      {hienForm && tai.pha === "xong" && (
        <FormSuaKhoiVanBan
          nhiemVu={nhiemVu}
          vanBan={tai.duLieu}
          luu={luu}
          docLai={docLai}
          xong={thoatSua}
        />
      )}

      {!hienForm && <DocKhoiVanBan tai={tai} />}
    </div>
  );
}

/** Ba pha đọc của khối §5.4 — xem `KhoiVanBanChiDao`. */
function DocKhoiVanBan({ tai }: { tai: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]> }) {
  return (
    <>
      {tai.pha === "dangTai" && <p role="status">{DANG_TAI_VAN_BAN}</p>}
      {tai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {KHONG_DOC_DUOC_VAN_BAN} {tai.thongBao}
        </p>
      )}
      {tai.pha === "xong" && (
        <dl className="danh-sach-truong">
          {chiaNhomVanBan(tai.duLieu).map((nhom) => (
            <div key={nhom.ma}>
              <dt>{nhom.nhan}</dt>
              <dd>
                {nhom.vanBan.length === 0 ? (
                  O_TRONG
                ) : (
                  <ul>
                    {nhom.vanBan.map((v) => (
                      <li key={v.id}>
                        {dongVanBan(v.reference, v.date)}
                        {v.summary !== "" && <span className="dong-phu">{v.summary}</span>}
                      </li>
                    ))}
                  </ul>
                )}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </>
  );
}

/** Nút `Lưu` khoá vì form chưa khác gì chi tiết đã đọc. Nói ra để cán bộ không tưởng nút hỏng. */
const CHUA_CO_GI_DOI = "Chưa có gì thay đổi để lưu.";

/**
 * Form `✎ Sửa` của khối §5.4.
 *
 * BẮT ĐẦU TỪ CHI TIẾT, VÀ CHỤP LẠI CHI TIẾT ẤY LÚC MỞ (`goc`). Thân PATCH so form với BẢN CHỤP, không
 * với chi tiết mới nhất: "không đổi" nghĩa là cán bộ chưa đụng tới, nên một lần đọc lại chi tiết giữa
 * chừng (sau một lần đổi trạng thái) không được biến những ô cán bộ để yên thành "đã đổi".
 *
 * KHOÁ LẠC QUAN (d2ed15e): mọi lần lưu mang `expected_updated_at` của BẢN CHỤP. Ai đó đã ghi nhiệm vụ
 * kể từ lúc mở form thì máy chủ từ chối 409 và câu của nó hiện nguyên văn; form đọc lại để biết chắc
 * (`changedSince`) và nói cán bộ mở lại form — nó KHÔNG tự thay bản chụp, vì lưu đè lên một phiên bản
 * cán bộ chưa thấy là đúng điều khoá sinh ra để chặn. Bên gọi đọc lại drawer sau mỗi lần lưu hỏng.
 *
 * GIỜ CỦA HẠN (quyết định của người dùng 28/09/2026): chọn một ngày khi ô giờ còn trống thì giờ điền
 * sẵn là giờ kết thúc ca cuối của thứ ấy theo lịch làm việc của xã (`defaultDueTime`,
 * `GET /api/v1/working-hours`). Không có ca, hoặc không đọc được lịch: ô giờ để trống và BẮT BUỘC.
 *
 * Dòng văn bản dùng LẠI `NhomVanBanNhap` của form tạo: cùng ô, cùng giới hạn, cùng cách kiểm (quyết
 * định của người dùng 24/09/2026). Không có thao tác chuyển dòng sang nhóm khác — máy chủ từ chối
 * (`ErrDoiNhomVanBan`); muốn đổi nhóm thì `✕` rồi `+ Thêm văn bản` ở nhóm mới.
 */
export function FormSuaKhoiVanBan({
  nhiemVu,
  vanBan,
  luu,
  docLai,
  xong,
}: {
  nhiemVu: petitions_nhiemVuRa;
  vanBan: readonly petitions_nhiemVuVanBanRa[];
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
  const [loi, datLoi] = useState<string | null>(null);
  const [staleNote, setStaleNote] = useState(false);
  const calendar = useDueCalendar(datF);
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
    if (than === null || chan !== null || dangLuu) return;
    datDangLuu(true);

    // THÂN CÓ `documents` THÌ ĐỌC LẠI TRƯỚC. Đọc hỏng, hoặc khối đã đổi ở máy chủ: KHÔNG gửi, giữ
    // nguyên chữ đã gõ, nói ra. Không tự gộp.
    if (canDocLaiTruocKhiLuu(than)) {
      // So với BẢN CHỤP lúc mở (`goc`), không với `vanBan` mới nhất của drawer: một dòng người khác
      // thêm mà drawer đã đọc lại giữa chừng thì CÓ trong `vanBan`, nhưng KHÔNG có trong form.
      const lyDoChan = loiSauKhiDocLai(await docLai(), goc.vanBan);
      if (lyDoChan !== null) {
        datDangLuu(false);
        datLoi(lyDoChan);
        return;
      }
    }

    luu(than).then(async (kq) => {
      if (!kq.ok) {
        // Ở LẠI chế độ sửa, giữ nguyên chữ đã gõ, in NGUYÊN VĂN câu máy chủ — kể cả câu 409 "gỡ dòng
        // ấy rồi thêm lại ở nhóm mới" hay "nhiệm vụ vừa được người khác sửa".
        datLoi(kq.thongBao);
        setStaleNote(changedSince(await docLai(), goc.nhiemVu.updated_at));
        datDangLuu(false);
        return;
      }
      datDangLuu(false);
      datLoi(null);
      setStaleNote(false);
      xong();
    });
  }

  return (
    <form onSubmit={gui} aria-labelledby="tieu-de-sua-khoi-van-ban">
      <h5 id="tieu-de-sua-khoi-van-ban">Sửa {TIEU_DE_KHOI_VAN_BAN.toLowerCase()}</h5>

      {/* NO `Mã nhiệm vụ` FIELD (ADR 0065 NV3, user decision 30/09/2026): an issued code is never
          edited — rule 7, invariant 3; the server answers 400 to `code` on PATCH. */}
      <div className="o-nhap">
        <label htmlFor="sua-tieu-de">{NHAN_TIEU_DE_THEO_VAN_BAN}</label>
        <input
          id="sua-tieu-de"
          name="sua-tieu-de"
          value={f.tieuDe}
          required
          maxLength={TIEU_DE_NHIEM_VU_TOI_DA}
          autoComplete="off"
          onChange={(e) => doi({ tieuDe: e.target.value })}
        />
      </div>

      <DueFieldset
        f={f}
        calendar={calendar}
        goc={goc.nhiemVu}
        changeDate={(date) => datF((cu) => withDueDate(cu, date, calendar))}
        changeTime={(dueTime) => doi({ dueTime })}
      />

      {MOI_NHOM_VAN_BAN.map((nhom) => (
        <NhomVanBanNhap
          key={nhom}
          tienTo="sua"
          nhom={nhom}
          dong={dongCuaNhom(f.vanBan, nhom)}
          them={() => themVanBan(nhom)}
          go={goVanBan}
          sua={suaVanBan}
        />
      ))}

      <div className="o-nhap">
        <label htmlFor="sua-tom-tat-ket-qua">Tóm tắt kết quả thực hiện</label>
        <textarea
          id="sua-tom-tat-ket-qua"
          name="sua-tom-tat-ket-qua"
          rows={3}
          maxLength={TOM_TAT_KET_QUA_TOI_DA}
          value={f.tomTatKetQua}
          onChange={(e) => doi({ tomTatKetQua: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="sua-ghi-chu">Ghi chú</label>
        <textarea
          id="sua-ghi-chu"
          name="sua-ghi-chu"
          rows={3}
          maxLength={GHI_CHU_NHIEM_VU_TOI_DA}
          value={f.ghiChu}
          onChange={(e) => doi({ ghiChu: e.target.value })}
        />
      </div>

      <div className="o-chon">
        <label htmlFor="sua-lanh-dao-phe-duyet">
          <input
            id="sua-lanh-dao-phe-duyet"
            type="checkbox"
            checked={f.lanhDaoPheDuyet}
            onChange={(e) => doi({ lanhDaoPheDuyet: e.target.checked })}
          />{" "}
          Lãnh đạo xã đã phê duyệt hoàn thành
        </label>
      </div>
      <div className="o-chon">
        <label htmlFor="sua-cap-tren-cong-nhan">
          <input
            id="sua-cap-tren-cong-nhan"
            type="checkbox"
            checked={f.capTrenCongNhan}
            onChange={(e) => doi({ capTrenCongNhan: e.target.checked })}
          />{" "}
          Cấp trên đã công nhận hoàn thành
        </label>
      </div>
      {/* §5.4 — chú thích BẮT BUỘC, và ở chế độ sửa nó càng phải có: đây là lúc cán bộ tick. */}
      <p className="ghi-chu">{CHU_THICH_HAI_O_TICK}</p>

      {/* Lý do nút `Lưu` khoá — KHÔNG `role="alert"`, cùng lý do form tạo (xem `FormGiaoViec`). */}
      {chan !== null && <p className="thong-bao-loi">{chan}</p>}
      {chan === null && than === null && <p className="ghi-chu">{CHUA_CO_GI_DOI}</p>}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
          {staleNote && ` ${TASK_CHANGED_NOTE}`}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={xong} disabled={dangLuu}>
          {NHAN_NUT_HUY}
        </button>
        <button
          type="submit"
          className="nut-chinh"
          disabled={dangLuu || than === null || chan !== null}
        >
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * The commune's weekly calendar for the deadline's default hour — read once per opening of a form
 * that edits a deadline (`FormSuaKhoiVanBan`, `DueEditForm`). A date chosen while it was still
 * loading gets its default once it arrives — only into an EMPTY time, never over a typed hour.
 */
function useDueCalendar(setForm: Dispatch<SetStateAction<FormSuaNhiemVu>>): CalendarLoad {
  const [calendar, setCalendar] = useState<CalendarLoad>({ pha: "dangTai" });
  useEffect(() => {
    let dropped = false;
    layLichLamViec().then((r) => {
      if (dropped) return;
      const cal: CalendarLoad = r.ok ? { pha: "xong", shifts: r.duLieu.items } : { pha: "loi" };
      setCalendar(cal);
      if (cal.pha === "xong") {
        setForm((cu) =>
          cu.dueDate !== "" && cu.dueTime === ""
            ? { ...cu, dueTime: defaultDueTime(cal.shifts, cu.dueDate) }
            : cu,
        );
      }
    });
    return () => {
      dropped = true;
    };
  }, [setForm]);
  return calendar;
}

/** A new deadline date. Only an EMPTY time is filled: an hour already there (read, or typed) stays. */
function withDueDate(cu: FormSuaNhiemVu, date: string, calendar: CalendarLoad): FormSuaNhiemVu {
  return {
    ...cu,
    dueDate: date,
    dueTime:
      cu.dueTime === "" && calendar.pha === "xong" ? defaultDueTime(calendar.shifts, date) : cu.dueTime,
  };
}

/** The deadline's date + time fields with their notes — shared by both deadline editors. */
function DueFieldset({
  f,
  calendar,
  goc,
  changeDate,
  changeTime,
}: {
  f: FormSuaNhiemVu;
  calendar: CalendarLoad;
  /** The task as the form opened it — the time hint compares against it. */
  goc: petitions_nhiemVuRa;
  changeDate: (date: string) => void;
  changeTime: (time: string) => void;
}) {
  const timeHint = dueTimeHint(calendar, f, goc);
  return (
    <fieldset className="o-nhap" aria-describedby="sua-han-ghi-chu">
      <legend>Hạn xử lý</legend>
      <label htmlFor="sua-han-ngay">Ngày</label>
      <input
        id="sua-han-ngay"
        name="sua-han-ngay"
        type="date"
        value={f.dueDate}
        onChange={(e) => changeDate(e.target.value)}
      />
      <label htmlFor="sua-han-gio">Giờ</label>
      <input
        id="sua-han-gio"
        name="sua-han-gio"
        type="time"
        value={f.dueTime}
        // Required as soon as a date is there: the server stores the instant as sent and defaults
        // no hour (`nhiem_vu_ghi.go:220-221`).
        required={f.dueDate !== ""}
        aria-describedby={timeHint !== null ? "sua-han-gio-goi-y" : undefined}
        onChange={(e) => changeTime(e.target.value)}
      />
      {timeHint !== null && (
        <p id="sua-han-gio-goi-y" className="ghi-chu">
          {timeHint}
        </p>
      )}
      <p id="sua-han-ghi-chu" className="ghi-chu">
        {DUE_EDIT_NOTE}
      </p>
    </fieldset>
  );
}

/**
 * The deadline editor of a task whose type has NO document block (ADR 0065 NV4; report 05/10/2026,
 * NV-04: "a task created without a deadline can never get one"). `Theo văn bản` keeps editing it in
 * `FormSuaKhoiVanBan`; the two never show together, so they share the field ids.
 *
 * The caller renders it only for `task.update` — the key `PATCH /api/v1/tasks/{code}` checks.
 */
export function DueEditBlock({
  nhiemVu,
  save,
  reread,
}: {
  nhiemVu: petitions_nhiemVuRa;
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  reread: () => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [editing, setEditing] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const returnFocus = useRef(false);

  useEffect(() => {
    if (editing || !returnFocus.current) return;
    returnFocus.current = false;
    button.current?.focus();
  }, [editing]);

  const label = nhiemVu.due_at === null || nhiemVu.due_at === "" ? DUE_SET_BUTTON : DUE_EDIT_BUTTON;

  return (
    <div className="form-danh-muc">
      {editing ? (
        <DueEditForm
          nhiemVu={nhiemVu}
          title={label}
          save={save}
          reread={reread}
          done={() => {
            returnFocus.current = true;
            setEditing(false);
          }}
        />
      ) : (
        <div className="cum-nut">
          <button ref={button} type="button" className="nut-phu" onClick={() => setEditing(true)}>
            <Glyph icon={Pencil} className="size-[18px]" />
            {label}
          </button>
        </div>
      )}
    </div>
  );
}

/**
 * The form behind `DueEditBlock`. Starts from the task as opened (`goc`) and sends ONLY `due_at`
 * plus the optimistic-lock token — the same field-by-field comparison as `thanSuaNhiemVu`, so an
 * untouched `23:59:59` is never re-sent as `23:59:00`. A refusal stays in the form, verbatim, and a
 * re-read says whether the task moved on (`changedSince`), exactly as in `FormSuaKhoiVanBan`.
 */
function DueEditForm({
  nhiemVu,
  title,
  save,
  reread,
  done,
}: {
  nhiemVu: petitions_nhiemVuRa;
  title: string;
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  reread: () => Promise<KetQua<petitions_nhiemVuRa>>;
  done: () => void;
}) {
  const [goc] = useState(nhiemVu);
  const [f, setF] = useState<FormSuaNhiemVu>(() => formSuaTuChiTiet(nhiemVu, []));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [staleNote, setStaleNote] = useState(false);
  const calendar = useDueCalendar(setF);

  useEffect(() => {
    document.getElementById("sua-han-ngay")?.focus();
  }, []);

  const dueAt = thanSuaNhiemVu(f, goc, [])?.due_at;
  const body: petitions_suaNhiemVuVao | null =
    dueAt === undefined || dueAt === null
      ? null
      : goc.updated_at !== ""
        ? { due_at: dueAt, expected_updated_at: goc.updated_at }
        : { due_at: dueAt };
  const block = canhBaoSua(f, goc);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (body === null || block !== null || saving) return;
    setSaving(true);
    const r = await save(body);
    if (!r.ok) {
      setError(r.thongBao);
      setStaleNote(changedSince(await reread(), goc.updated_at));
      setSaving(false);
      return;
    }
    setSaving(false);
    done();
  }

  return (
    <form onSubmit={submit} aria-labelledby="tieu-de-sua-han">
      <h5 id="tieu-de-sua-han">{title}</h5>
      <DueFieldset
        f={f}
        calendar={calendar}
        goc={goc}
        changeDate={(date) => setF((cu) => withDueDate(cu, date, calendar))}
        changeTime={(dueTime) => setF((cu) => ({ ...cu, dueTime }))}
      />

      {block !== null && <p className="thong-bao-loi">{block}</p>}
      {block === null && body === null && <p className="ghi-chu">{CHUA_CO_GI_DOI}</p>}

      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
          {staleNote && ` ${TASK_CHANGED_NOTE}`}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={done} disabled={saving}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={saving || body === null || block !== null}>
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * §5.8 — đề nghị lùi hạn, và quyết định của lãnh đạo giao việc.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HAI NỬA CỦA KHỐI NÀY ĐỨNG SAU HAI KHOÁ KHÁC NHAU, và gộp chúng là làm hỏng đúng điều ADR 0038
 * dựng ra:
 *
 *   gửi đề nghị     `task.update` — việc của NGƯỜI ĐANG LÀM. Ô này hiện với MỌI người cầm khoá
 *                   ấy, dù họ có `task.extend` hay không.
 *   duyệt / từ chối `task.extend` ở cổng **và** đúng người ghi ở `lanh_dao_giao_viec_ma`.
 *
 * Bỏ lớp thứ hai thì **mọi lãnh đạo cầm khoá duyệt được mọi nhiệm vụ của cả xã**, kể cả của bộ
 * phận họ không liên quan — vì luật 5 kiểm `(tenant_id, role, permission)` và không có chiều "bản
 * ghi nào".
 *
 * THIS COMPONENT IS NOW ONLY THE FIRST HALF (28/09/2026, #11). The second half — the pending
 * requests and Duyệt / Từ chối — is `TaskExtensionBlock`, right below it in the drawer, which reads
 * `GET /api/v1/task-extensions?task=` and gates each row exactly like the queue on the register.
 * The old in-memory decision buttons (only for a request sent in this very drawer session) are
 * gone: two decision surfaces in one drawer would be two gates to keep equal.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function KhoiLuiHan({
  coQuyenDeNghi,
  hanHienTai,
  dangGui,
  guiDeNghi,
}: {
  /**
   * `task.update` — khoá của tuyến GỬI đề nghị (`POST …/extensions`). KHÔNG PHẢI `task.extend`:
   * gắn ô đề nghị sau khoá duyệt là "chỉ người duyệt được mới xin được" (ADR 0038).
   */
  coQuyenDeNghi: boolean;
  hanHienTai: string | null;
  dangGui: boolean;
  guiDeNghi: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
}) {
  const [hanMoi, datHanMoi] = useState("");
  const [lyDo, datLyDo] = useState("");
  const [dangChay, datDangChay] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [deNghi, datDeNghi] = useState<petitions_deNghiLuiHanRa | null>(null);

  function xong(kq: KetQua<petitions_deNghiLuiHanRa>): void {
    datDangChay(false);
    if (!kq.ok) {
      datLoi(kq.thongBao);
      return;
    }
    datLoi(null);
    datDeNghi(kq.duLieu);
    datHanMoi("");
    datLyDo("");
  }

  return (
    <div className="form-danh-muc">
      <h4>Đề nghị lùi hạn</h4>
      <p className="ghi-chu">{GHI_CHU_LUI_HAN}</p>

      {hanHienTai === null && (
        <p className="trang-thai-rong">{EXTENSION_NO_DUE}</p>
      )}

      {hanHienTai !== null && coQuyenDeNghi && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (hanMoi === "" || lyDo.trim() === "") return;
            datDangChay(true);
            guiDeNghi(mocCuoiNgay(hanMoi), lyDo.trim()).then(xong);
          }}
        >
          <div className="o-nhap">
            <label htmlFor="han-moi-lui-han">Hạn mới</label>
            <input
              id="han-moi-lui-han"
              name="han-moi-lui-han"
              type="date"
              value={hanMoi}
              onChange={(e) => datHanMoi(e.target.value)}
            />
          </div>
          <div className="o-nhap">
            <label htmlFor="ly-do-lui-han">Lý do</label>
            <input
              id="ly-do-lui-han"
              name="ly-do-lui-han"
              value={lyDo}
              autoComplete="off"
              onChange={(e) => datLyDo(e.target.value)}
            />
          </div>
          <button
            type="submit"
            className="nut-chinh"
            disabled={dangGui || dangChay || hanMoi === "" || lyDo.trim() === ""}
          >
            Gửi đề nghị lùi hạn
          </button>
        </form>
      )}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      {/* The request itself appears in `TaskExtensionBlock` below, re-read from the server —
          the caller bumps its refresh key on success. */}
      {deNghi !== null && (
        <p role="status">
          Đã gửi đề nghị lùi hạn tới {nhanNgay(deNghi.new_due_at)}. Đề nghị hiện ở mục “Đề nghị lùi
          hạn đang chờ duyệt” ngay dưới.
        </p>
      )}
    </div>
  );
}

/**
 * Form `Giao việc mới` §7.
 *
 * HAI LOẠI, HAI BỘ TRƯỜNG (§7.2 / §7.3), rẽ nhánh trên MÃ `theo-van-ban` (mã tầng 3, xem
 * `LOAI_THEO_VAN_BAN`). Loại nào khác thì ô tiêu đề thành `Tên nhiệm vụ`, và ba nhóm văn bản BIẾN
 * KHỎI MÀN và KHÔNG LÊN DÂY — phần "không lên dây" ở
 * `thanGiaoViec`, nơi có bài kiểm. Ô `Ghi chú` của §7.2 đi cùng cổng với ba danh sách văn bản
 * (`hienVanBan`): tuyến tách kết luận của màn Biên bản không nhận `note`.
 *
 * KHÔNG CÓ Ô `Cơ quan chủ trì tham mưu` / `Chuyên viên theo dõi` (ADR 0065 NV5, người dùng chốt
 * 30/09/2026): hai vai ấy LÀ `Đơn vị thực hiện` và `Người thực hiện`.
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
  dangGui,
  loi,
  huy,
  giaoViec,
  maChaCoSan,
  tieuDeCoSan,
}: {
  /**
   * The three staff fields become type-to-search boxes (`StaffCombobox`). ONLY the Nhiệm vụ
   * screen sets it. Off by default so the Biên bản screen, which reuses this form, keeps its
   * native `<select>` until it is decided to adopt the box — a scope call, not a technical one.
   */
  staffSearch?: boolean;
  danhMuc: DanhMucNhiemVu;
  /**
   * Câu trả lời nguyên vẹn của danh bạ chọn người; `null` = chưa đọc xong. BẮT BUỘC, không tuỳ
   * chọn: bên gọi quên truyền thì `tsc` đỏ, thay vì một form có ba ô chọn rỗng không lời giải thích.
   */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /**
   * Danh bạ ĐÃ LỌC `permission=task.extend` cho riêng ô `Lãnh đạo giao việc` — người ghi ở đó là
   * người duyệt lùi hạn (ADR 0038). Cùng ba pha với `danhBa`. BẮT BUỘC vì cùng lý do: một bên gọi
   * quên truyền thì `tsc` đỏ, thay vì ô ấy lặng lẽ gợi cả xã — kể cả người không bao giờ duyệt được.
   */
  danhBaLanhDao: KetQua<identity_danhBaChonNguoiRa> | null;
  /**
   * Vẽ và gửi ba danh sách văn bản §7.2. CHỈ màn Nhiệm vụ bật.
   *
   * MẶC ĐỊNH TẮT, VÀ ĐÓ LÀ CHIỀU AN TOÀN: màn Biên bản dùng lại form này để gửi
   * `…/conclusions/{stt}/task`, mà `petitions.tachKetLuanVao` không có `documents`. Bên gọi nào
   * quên prop thì nhận một form không có ba danh sách — không phải một form để cán bộ gõ văn bản
   * rồi thấy chúng mất, hoặc bị 400.
   */
  coDanhSachVanBan?: boolean;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  giaoViec: (than: petitions_taoNhiemVuVao, khoaChongTrung: string) => void;
  /**
   * REGISTER CODE of the parent (`NV19`) when the form is opened by `+ Thêm việc con` (#10) — the
   * contract's `parent` takes a register code since ad7f821. Sent as-is; every invalid parent is
   * the server's 409 `task_tree`, shown verbatim through `loi`.
   */
  maChaCoSan?: string;
  /**
   * Điền sẵn ô `Nội dung nhiệm vụ` — `04-bien-ban-hop.md` §3, khi biểu mẫu này mở từ một kết
   * luận họp. Rỗng hoặc vắng là không điền.
   *
   * MỘT PROP THÊM VÀO, KHÔNG PHẢI MỘT BIỂU MẪU THỨ HAI. Màn Biên bản dùng LẠI chính biểu mẫu
   * này (`features/bien-ban/so-bien-ban.tsx`); chép một bản sang bên ấy sẽ điền sẵn được ngay
   * và sẽ là hai biểu mẫu cùng gửi một tuyến — ngày một bên thêm trường thì bên kia vẫn xanh
   * (luật 9, cấm #2). Ba dòng ở đây rẻ hơn hẳn cái giá ấy.
   *
   * CHỈ LÀ GIÁ TRỊ BAN ĐẦU, không phải một ô khoá: §3 muốn cán bộ sửa lại câu kết luận thành
   * một câu giao việc đọc được, chứ không chép nguyên văn.
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
  const [boPhan, datBoPhan] = useState("");
  const [nguoiThucHien, datNguoiThucHien] = useState("");
  const [lanhDaoGiaoViec, datLanhDaoGiaoViec] = useState("");
  // ADR 0065 NV6: pre-filled +7 calendar days at 17:00, computed ONCE when the form opens (lazy
  // initialiser) — the clerk may change or clear it.
  const [dueDefault] = useState(defaultNewTaskDue);
  const [han, datHan] = useState(dueDefault.date);
  const [dueTime, setDueTime] = useState(dueDefault.time);
  const [vanBan, datVanBan] = useState<readonly DongVanBanNhap[]>([]);
  const [ghiChu, datGhiChu] = useState("");
  const demKhoaVanBan = useRef(0);
  // Id ô cần nhận tiêu điểm SAU lần vẽ kế tiếp: dòng vừa thêm chưa có trong DOM lúc bấm nút.
  const oCanTieuDiem = useRef<string | null>(null);

  useEffect(() => {
    if (oCanTieuDiem.current === null) return;
    document.getElementById(oCanTieuDiem.current)?.focus();
    oCanTieuDiem.current = null;
  }, [vanBan]);

  // Loại mặc định lấy từ DANH MỤC CỦA XÃ (`is_default`), không gõ cứng `theo-van-ban`: §7.1 nói
  // loại `Theo văn bản` là mặc định, nhưng đó là một dòng danh mục xã sửa được.
  const loaiMacDinh = danhMuc.loai.find((l) => l.is_default)?.code ?? "";
  const loaiChon = loai === "" ? loaiMacDinh : loai;
  // §7.1 `Thường (mặc định)` — nhưng "Thường" là một dòng danh mục CỦA XÃ, không phải một chữ nung
  // vào đây. Xã chưa đặt dòng mặc định nào thì ô đứng ở `— Chưa xác định —`.
  const mucUuTien = mucUuTienDaChon ?? mucUuTienMacDinh(danhMuc.mucUuTien);
  const theoVanBan = coKhoiVanBanChiDao(loaiChon);
  const hienVanBan = theoVanBan && coDanhSachVanBan;
  const chanVanBan = hienVanBan ? canhBaoVanBan(vanBan) : null;
  const chanHan = newTaskDueProblem(han, dueTime);
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
    datVanBan((ds) => [...ds, { khoa, nhom, trichYeu: "", soKyHieu: "", ngay: "" }]);
  }

  function goVanBan(dong: DongVanBanNhap) {
    // Dòng vừa gỡ mang theo tiêu điểm; trả nó về nút thêm của cùng nhóm thay vì để rơi về đầu trang.
    oCanTieuDiem.current = `giao-them-van-ban-${dong.nhom}`;
    datVanBan((ds) => ds.filter((d) => d.khoa !== dong.khoa));
  }

  function suaVanBan(khoa: string, sua: Partial<Omit<DongVanBanNhap, "khoa" | "nhom">>) {
    datVanBan((ds) => ds.map((d) => (d.khoa === khoa ? { ...d, ...sua } : d)));
  }

  function gui(e: FormEvent) {
    e.preventDefault();
    if (
      tieuDe.trim() === "" ||
      loaiChon === "" ||
      chanVanBan !== null ||
      chanHan !== null ||
      dangTaiDanhBa
    ) {
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
        boPhan,
        nguoiThucHien,
        lanhDaoGiaoViec,
        han,
        dueTime,
        vanBan,
        ghiChu,
      },
      { coDanhSachVanBan, maCha: maChaCoSan },
    );

    giaoViec(than, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={gui}>
      <h4>Giao việc mới</h4>
      <p className="ghi-chu">{MO_TA_FORM_GIAO_VIEC}</p>
      {maChaCoSan !== undefined && maChaCoSan !== "" && (
        <p className="ghi-chu">{childFormNote(maChaCoSan)}</p>
      )}

      <div className="o-chon">
        <label htmlFor="giao-loai">Loại nhiệm vụ</label>
        {/* THE EMPTY OPTION WHENEVER NOTHING IS CHOSEN, not only when the catalogue is empty: a
            commune with no default row otherwise showed its FIRST type while the value was `""`,
            and `Giao việc` stayed grey for no visible reason (report 05/10/2026, NV-01). The first
            row is NOT picked for the clerk — the default is the commune's catalogue choice. */}
        <select
          id="giao-loai"
          value={loaiChon}
          aria-describedby={loaiChon === "" ? "giao-loai-thieu" : undefined}
          onChange={(e) => datLoai(e.target.value)}
        >
          {loaiChon === "" && <option value="">{TASK_TYPE_PLACEHOLDER}</option>}
          {danhMuc.loai.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-khoi">Khối nhiệm vụ</label>
        <select id="giao-khoi" value={khoi} onChange={(e) => datKhoi(e.target.value)}>
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.khoi.map((k) => (
            <option key={k.code} value={k.code}>
              {k.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-tu-sinh-ma">
          <input
            id="giao-tu-sinh-ma"
            type="checkbox"
            checked={tuSinhMa}
            onChange={(e) => datTuSinhMa(e.target.checked)}
          />{" "}
          Tự sinh mã
        </label>
      </div>
      <p className="ghi-chu">{GHI_CHU_TU_SINH_MA}</p>

      {!tuSinhMa && (
        <div className="o-nhap">
          <label htmlFor="giao-ma">Mã nhiệm vụ</label>
          <input
            id="giao-ma"
            name="giao-ma"
            value={ma}
            autoComplete="off"
            onChange={(e) => datMa(e.target.value)}
          />
        </div>
      )}

      <div className="o-nhap">
        <label htmlFor="giao-tieu-de">{nhanOTieuDe(loaiChon)}</label>
        <input
          id="giao-tieu-de"
          name="giao-tieu-de"
          value={tieuDe}
          required
          autoComplete="off"
          onChange={(e) => datTieuDe(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="giao-mo-ta">Mô tả</label>
        <textarea
          id="giao-mo-ta"
          name="giao-mo-ta"
          rows={3}
          value={moTa}
          onChange={(e) => datMoTa(e.target.value)}
        />
      </div>

      <div className="o-chon">
        <label htmlFor="giao-uu-tien">Mức ưu tiên</label>
        <select
          id="giao-uu-tien"
          value={mucUuTien}
          onChange={(e) => datMucUuTien(e.target.value)}
        >
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.mucUuTien.map((m) => (
            <option key={m.code} value={m.code}>
              {m.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-bo-phan">Đơn vị thực hiện</label>
        <select id="giao-bo-phan" value={boPhan} onChange={(e) => datBoPhan(e.target.value)}>
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      {/* HAI Ô CHỌN TỪ DANH BẠ CHỌN NGƯỜI (`GET /api/v1/staff-directory`). Giá trị là MÃ NGHIỆP VỤ
          `CB-…` (`code`), đúng loại định danh hai trường kia giữ: một ULID ở đây được máy chủ nhận
          nhưng không khớp cán bộ nào, và không có gì đỏ ở đâu (luật 6, bất biến 8). Danh bạ đọc
          hỏng thì các ô chỉ còn lựa chọn trống và câu lỗi nói hệ quả — không đoán ai. */}
      {db.loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {cauLoiDanhBaGiaoViec(db.loi)}
        </p>
      )}
      <StaffPicker
        search={staffSearch}
        id="giao-nguoi-thuc-hien"
        label="Người thực hiện"
        emptyLabel={nhanTrongOChonCanBo(db, DE_BO_PHAN_TU_PHAN_CONG)}
        value={nguoiThucHien}
        directory={db.ds}
        disabled={db.dangTai}
        onChange={datNguoiThucHien}
      />

      {/* LÃNH ĐẠO GIAO VIỆC — CHỈ NGƯỜI CẦM `task.extend` (`danhBaLanhDao`). Ba ca có chữ: đọc hỏng
          (câu máy chủ nguyên văn), đọc được mà rỗng (không ai cầm quyền — một câu, không một ô rỗng),
          còn lại là ô chọn. */}
      {dbLanhDao.loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {cauLoiDanhBaLanhDao(dbLanhDao.loi)}
        </p>
      )}
      {khongAiDuyetDuoc ? (
        <div className="o-nhap">
          <span>Lãnh đạo giao việc</span>
          <p className="ghi-chu" id="giao-lanh-dao-trong">
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
          emptyLabel={nhanTrongOChonCanBo(dbLanhDao, CHUA_XAC_DINH)}
          value={lanhDaoGiaoViec}
          directory={dbLanhDao.ds}
          disabled={dbLanhDao.dangTai}
          onChange={datLanhDaoGiaoViec}
        />
      )}
      {/* KHÔNG CHỈ LÀ NƠI NHẬN THÔNG BÁO: ô này quyết định AI DUYỆT ĐƯỢC ĐỀ NGHỊ LÙI HẠN (ADR
          0038), và bỏ trống nghĩa là KHÔNG AI duyệt được — vĩnh viễn, vì `PATCH` cố ý không sửa
          được cột này. Câu ấy đứng cạnh ô chứ không nằm trong tài liệu. */}
      {/* Không ai để chọn thì câu `CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN` đã nói đủ hệ quả ấy. */}
      {!khongAiDuyetDuoc && (
        <p className="ghi-chu">
          {GHI_CHU_LANH_DAO_GIAO_VIEC} Bỏ trống thì không ai duyệt được đề nghị lùi hạn của nhiệm vụ
          này, và ô này không sửa lại được sau khi tạo.
        </p>
      )}

      {hienVanBan && (
        <>
          {MOI_NHOM_VAN_BAN.map((nhom) => (
            <NhomVanBanNhap
              key={nhom}
              nhom={nhom}
              dong={dongCuaNhom(vanBan, nhom)}
              them={() => themVanBan(nhom)}
              go={goVanBan}
              sua={suaVanBan}
            />
          ))}
          {/* §7.2 đặt `Ghi chú` NGAY SAU ba danh sách. Cùng cổng `hienVanBan`: §7.3 bỏ ô này ở loại
              khác, và màn Biên bản gửi tới tuyến không có `note`. */}
          <div className="o-nhap">
            <label htmlFor="giao-ghi-chu">Ghi chú</label>
            <textarea
              id="giao-ghi-chu"
              name="giao-ghi-chu"
              rows={3}
              maxLength={GHI_CHU_NHIEM_VU_TOI_DA}
              value={ghiChu}
              onChange={(e) => datGhiChu(e.target.value)}
            />
          </div>
        </>
      )}

      <div className="o-nhap">
        <label htmlFor="giao-han">Hạn hoàn thành</label>
        <input
          id="giao-han"
          name="giao-han"
          type="date"
          value={han}
          onChange={(e) => datHan(e.target.value)}
        />
      </div>
      <div className="o-nhap">
        <label htmlFor="giao-han-gio">Giờ</label>
        <input
          id="giao-han-gio"
          name="giao-han-gio"
          type="time"
          value={dueTime}
          required={han !== ""}
          onChange={(e) => setDueTime(e.target.value)}
        />
      </div>
      <p className="ghi-chu">{NEW_TASK_DUE_PREFILLED_NOTE}</p>
      <p className="ghi-chu">{NEW_TASK_DUE_LATER_NOTE}</p>
      {maChaCoSan !== undefined && maChaCoSan !== "" && (
        <p className="ghi-chu">{GHI_CHU_HAN_VIEC_CON}</p>
      )}

      {/* KHÔNG `role="alert"`: câu này hiện ngay khi bấm `+ Thêm văn bản` (dòng mới còn trống), và
          một vùng thông báo khẩn sẽ cắt ngang đúng lúc tiêu điểm vừa sang ô trích yếu. Nó là lý do
          nút `Giao việc` đang khoá, nên đứng ngay trên nút ấy. */}
      {loaiChon === "" && (
        <p id="giao-loai-thieu" className="thong-bao-loi">
          {TASK_TYPE_MISSING}
        </p>
      )}
      {chanVanBan !== null && <p className="thong-bao-loi">{chanVanBan}</p>}
      {chanHan !== null && <p className="thong-bao-loi">{chanHan}</p>}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={huy} disabled={dangGui}>
          Huỷ
        </button>
        <button
          type="submit"
          className="nut-chinh"
          // Danh bạ CÒN ĐANG TẢI thì khoá: ba ô chọn chưa chọn được ai, và một nhiệm vụ tạo ra lúc
          // ấy mang `Lãnh đạo giao việc` rỗng VĨNH VIỄN (`PATCH` không sửa cột ấy). Tải HỎNG thì
          // không khoá — câu lỗi ngay trên nói hệ quả, và việc giao cho bộ phận vẫn phải làm được
          // khi `identity` trục trặc (cùng chiều màn Phản ánh).
          disabled={
            dangGui ||
            tieuDe.trim() === "" ||
            loaiChon === "" ||
            chanVanBan !== null ||
            chanHan !== null ||
            dangTaiDanhBa
          }
        >
          Giao việc
        </button>
      </div>
    </form>
  );
}

/**
 * The form's staff field: `StaffCombobox` when `search`, otherwise the native `OChonCanBo` the
 * Biên bản screen still uses. Same props either way, so the switch is one boolean.
 */
function StaffPicker({
  search,
  id,
  label,
  emptyLabel,
  value,
  directory,
  disabled,
  onChange,
}: {
  search: boolean;
  id: string;
  label: string;
  emptyLabel: string;
  value: string;
  directory: readonly identity_canBoChonNguoiRa[];
  disabled: boolean;
  onChange: (code: string) => void;
}) {
  return search ? (
    <StaffCombobox
      id={id}
      label={label}
      emptyLabel={emptyLabel}
      value={value}
      directory={directory}
      disabled={disabled}
      onChange={onChange}
    />
  ) : (
    <OChonCanBo
      id={id}
      nhan={label}
      nhanTrong={emptyLabel}
      giaTri={value}
      danhBa={directory}
      khoa={disabled}
      dat={onChange}
    />
  );
}

/**
 * Một trong ba danh sách văn bản động của §7.2: tiêu đề nhóm · các dòng · `+ Thêm văn bản`.
 *
 * MỖI DÒNG LÀ MỘT Ô TRÍCH YẾU BẮT BUỘC VÀ HAI Ô NHỎ TUỲ CHỌN (`Số, ký hiệu`, `Ngày văn bản`). Không
 * tách số và ngày ra khỏi câu trích yếu bằng máy: đoán số hiệu từ một câu tiếng Việt là in ra một
 * số văn bản không tồn tại (`nhiem_vu_van_ban.go:77-80`). Ai muốn điền riêng thì điền; bỏ trống thì
 * drawer ghi `Không số`, đúng như văn bản không số có thật.
 *
 * `role="group"` + tiêu đề thay cho `<fieldset>`: `fieldset` mặc định rộng tối thiểu bằng nội dung,
 * và ở 320px một placeholder dài đẩy nó tràn ngang — mà lượt này không thêm CSS.
 */
function NhomVanBanNhap({
  tienTo = "giao",
  nhom,
  dong,
  them,
  go,
  sua,
}: {
  /**
   * Tiền tố của mọi `id` trong nhóm. Form tạo và form `✎ Sửa` có thể CÙNG MỞ trên một trang; chung
   * tiền tố thì hai ô mang cùng `id`, và `<label htmlFor>` trỏ nhầm sang ô của form kia.
   */
  tienTo?: "giao" | "sua";
  nhom: NhomVanBan;
  dong: readonly DongVanBanNhap[];
  them: () => void;
  go: (dong: DongVanBanNhap) => void;
  sua: (khoa: string, sua: Partial<Omit<DongVanBanNhap, "khoa" | "nhom">>) => void;
}) {
  const idTieuDe = `${tienTo}-nhom-van-ban-${nhom}`;
  return (
    <div role="group" aria-labelledby={idTieuDe}>
      <h5 id={idTieuDe}>{nhanNhomVanBan(nhom)}</h5>
      {dong.map((d, i) => {
        const id = `${tienTo}-van-ban-${d.khoa}`;
        return (
          <div key={d.khoa} role="group" aria-label={`${nhanNhomVanBan(nhom)} — văn bản thứ ${i + 1}`}>
            <div className="o-nhap">
              <label htmlFor={id}>Trích yếu (văn bản thứ {i + 1})</label>
              <textarea
                id={id}
                name={id}
                rows={2}
                required
                maxLength={TRICH_YEU_VAN_BAN_TOI_DA}
                placeholder={placeholderNhomVanBan(nhom)}
                value={d.trichYeu}
                onChange={(e) => sua(d.khoa, { trichYeu: e.target.value })}
              />
            </div>
            <div className="o-nhap">
              <label htmlFor={`${id}-so`}>Số, ký hiệu (không bắt buộc)</label>
              <input
                id={`${id}-so`}
                name={`${id}-so`}
                maxLength={SO_KY_HIEU_VAN_BAN_TOI_DA}
                autoComplete="off"
                value={d.soKyHieu}
                onChange={(e) => sua(d.khoa, { soKyHieu: e.target.value })}
              />
            </div>
            <div className="o-nhap">
              <label htmlFor={`${id}-ngay`}>Ngày văn bản (không bắt buộc)</label>
              <input
                id={`${id}-ngay`}
                name={`${id}-ngay`}
                type="date"
                value={d.ngay}
                onChange={(e) => sua(d.khoa, { ngay: e.target.value })}
              />
            </div>
            <button
              type="button"
              className="nut-phu"
              aria-label={nhanNutGoVanBan(nhom, i + 1)}
              onClick={() => go(d)}
            >
              ✕
            </button>
          </div>
        );
      })}
      <button
        type="button"
        id={`${tienTo}-them-van-ban-${nhom}`}
        className="nut-phu"
        aria-label={`${NHAN_THEM_VAN_BAN} vào nhóm ${nhanNhomVanBan(nhom)}`}
        onClick={them}
      >
        {NHAN_THEM_VAN_BAN}
      </button>
    </div>
  );
}
