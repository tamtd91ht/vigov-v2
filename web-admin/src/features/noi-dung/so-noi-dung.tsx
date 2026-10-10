"use client";

import {
  BookUser,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  Clock,
  CloudOff,
  Eye,
  EyeOff,
  FolderTree,
  Images,
  ListOrdered,
  LoaderCircle,
  MapPin,
  Newspaper,
  Paperclip,
  Pencil,
  Plus,
  Radio,
  RefreshCw,
  Search,
  Trash2,
  TriangleAlert,
  Upload,
  Video,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { createElement, useEffect, useRef, useState, type FormEvent, type KeyboardEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { controlClass, Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { PendingCell, PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import {
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { tabKeTheoPhim } from "@/features/cau-hinh/thanh-tab-cau-hinh";
import { miniAppTabHref } from "@/features/mini-app/mini-app-tabs";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  countPublishedStaff,
  layDanhMucNoiDung,
  layMotNoiDung,
  laySoNoiDung,
  portalCategoryName,
  CONTENT_TYPE_BANNER,
  CONTENT_TYPE_BROADCAST,
  CONTENT_TYPE_EVENT,
  CONTENT_TYPE_VIDEO,
  deleteContentCategory,
  deleteContentItem,
  suaNoiDung,
  themDanhMucNoiDung,
  themNoiDung,
  TU_KHOA_TIM_TOI_DA,
  updateContentCategory,
  type BoLocNoiDung,
} from "@/lib/api/noi-dung";
import type {
  comms_danhMucRa,
  comms_danhSachDanhMucRa,
  comms_noiDungRa,
  page_Result_comms_noiDungRa,
} from "@/lib/api/schema.gen";

import {
  ACTIONS_COLUMN_LABEL,
  attachmentState,
  BANNER_APP_PATHS,
  canEditContent,
  categoryCellLabel,
  categoryPath,
  CATEGORY_ADD_BUTTON,
  CATEGORY_ADD_LABEL,
  CATEGORY_ADD_PLACEHOLDER,
  CATEGORY_ADDED_TOAST,
  CATEGORY_DIALOG_DESCRIPTION,
  CATEGORY_DIALOG_TITLE,
  CATEGORY_NO_PARENT_OPTION,
  CATEGORY_PARENT_LABEL,
  CHUA_XEP_DANH_MUC,
  CLOSE_LABEL,
  CONTENT_DELETE_GONE,
  CONTENT_DELETE_TITLE,
  CONTENT_DELETED,
  contentDeleteAriaLabel,
  coThayDoi,
  DANG_TAI_SO,
  DANG_TAI_TOAN_VAN,
  dialogTypeLabel,
  DISPLAY_ORDER_MAX,
  dungCayDanhMuc,
  EDIT_FORM_TITLE,
  EVENT_PLACE_MAX_CHARS,
  FORM_TRONG,
  giaTriTuHang,
  KHONG_CO_GI_DOI,
  LINK_TO_MAX_CHARS,
  LOAI_MAC_DINH,
  MO_TA_FORM_THEM,
  MO_TA_MAN,
  MOI_DANH_MUC,
  MOI_LOAI,
  NHAN_NUT_DANH_MUC,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  nhanLoai,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanTrangThai,
  PAGE_SIZE,
  pageAfterDelete,
  pageRange,
  pendingContentPart,
  portalCategoryLabel,
  PUBLISH_BUTTON,
  PUBLISH_TOGGLE_FAILED,
  PUBLISHED_TOAST,
  publishToggleBody,
  SAVE_FAILED_TOAST,
  SAVED_TOAST,
  saveToast,
  SLUG_DANH_MUC_TOI_DA,
  SO_RONG,
  SOURCE_PORTAL_SYNC,
  STATUS_FILTER_OPTIONS,
  STATUS_PENDING_REVIEW,
  TEN_DANH_MUC_TOI_DA,
  THUMBNAIL_PART,
  TIEU_DE_FORM_THEM,
  TIEU_DE_MAN,
  TIEU_DE_THE_DANH_BA,
  TIEU_DE_TOI_DA,
  TIM_PLACEHOLDER,
  TITLE_REQUIRED,
  TOM_TAT_TOI_DA,
  TOTAL_ITEMS_PART,
  TOTAL_PAGES_PART,
  thanSua,
  thanThem,
  trichTomTat,
  UNPUBLISH_BUTTON,
  UNPUBLISHED_TOAST,
  UPLOADING_FILES_LABEL,
  URL_TOI_DA,
  validateTypeFields,
  VIEW_COUNT_LABEL,
  viewCountText,
  publishedStaffText,
  type GiaTriFormNoiDung,
} from "./nhan-noi-dung";
import {
  COVER_ACCEPT,
  COVER_BANNER_HINT,
  COVER_BANNER_LABEL,
  COVER_HINT,
  COVER_LABEL,
  COVER_PICK_BUTTON,
  COVER_REPLACE_BUTTON,
  COVER_RETRY_BUTTON,
  coverInFlight,
  coverSizeLabel,
  coverStateText,
  runCoverUpload,
  type CoverUploadState,
} from "./cover-image";
import {
  AUDIO_RETRY_BUTTON,
  AUDIO_SAVED_NOT_UPLOADED,
  AUDIO_STORAGE_FAILED,
  audioInFlight,
  audioStateText,
  heldAudioProblem,
  runAudioUpload,
  savedAudioPatch,
  type AudioPatch,
  type AudioUploadState,
  type HeldAudio,
} from "./broadcast-audio";
import {
  BODY_IMAGE_WAIT_COVER,
  bodyImageInFlight,
  previewsFromItem,
  runBodyImageFromUrl,
  runBodyImageUpload,
  type BodyImageState,
} from "./body-image";
import type { BodyImageSource } from "./body-image-panel";
import { HeldAudioField } from "./broadcast-audio-field";
import { CategoryAdmin, CategoryImportPending } from "./category-admin";
import { ContentDeleteDialog } from "./content-delete-dialog";
import { OverlayDialog } from "./overlay-dialog";
import { PortalSyncCard } from "./portal-sync-card";
import { RichTextEditor } from "./rich-text-editor";

/**
 * Màn "Nội dung Mini App" — drawn as the prototype's `ContentWorkspace` (`vigov-require/apps/admin/src/
 * components/content/ContentWorkspace.tsx`, ADR 0068 lần 5) with the owner's decisions of 09/10/2026
 * (D1 three statuses + `Đăng`/`Gỡ`, D3 extras removed). Top to bottom: the page header, the portal sync
 * card (`portal-sync-card.tsx`), the government-directory link card, the six type tabs, the filter row,
 * then ONE table frame holding the rows and the pager.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: **KHÔNG MỘT DÒNG NÀO Ở ĐÂY ĐƯA MỘT CHUỖI HTML VÀO TRANG**.
 *
 * `noi_dung` là HTML (§8). The server sanitises it on every write to the ADR 0067 §1 allow-list, and
 * the body is edited in Tiptap (`rich-text-editor.tsx`), which draws the document from its parsed
 * structure. So: NO `dangerouslySetInnerHTML` anywhere, not even for a preview — the editor IS the
 * preview. `ranh-gioi-html.test.ts` reads this folder's source and turns red if the string appears.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * EACH ROW HAS A DELETE (`🗑`, the prototype's trash button, user's decision 02/10/2026): a SOFT delete
 * with a MANDATORY reason (rule 7) asked in `ContentDeleteDialog` — the prototype's one-click delete
 * minus the missing reason. The reversible way off the Mini App is the row's `Gỡ` (D1). (A CATEGORY has
 * its own soft delete — `category-admin.tsx`, ADR 0067 §3.)
 *
 * WRITE CONTROLS FOLLOW `content.update` (`canEditContent`, as the prototype does): without it the screen
 * is read-only — no `Thêm nội dung`, no title-as-button, no `✎` / `Đăng` / `Gỡ` / `🗑`, no `Danh mục tin`,
 * no run/config on the portal card. That is CONVENIENCE: `service-comms` still checks `content.read` /
 * `content.update` on EVERY call, and a 403 sentence still reaches the screen (rule 5, forbidden #1).
 *
 * CONFIRMATIONS ARE TOASTS (sonner, the `Toaster` of `app/layout.tsx`), as in the prototype. A form's own
 * problems — the empty title, a per-type check, the server's 400 sentence — stay INLINE next to the boxes.
 *
 * TIÊU ĐỀ, TÓM TẮT VÀ THÂN BÀI LÀ TIN BÀI CỦA XÃ, thường nhắc tên và hoàn cảnh của một công dân cụ
 * thể: không dòng nào ở đây ghi chúng vào log, vào tên tệp hay vào một URL của trang (luật 3, cấm #1 và
 * #4). The search keyword lives in this component's state only — never in the address bar.
 */

/**
 * How long the search box waits after the last keystroke before it asks the server (prototype: filter as
 * you type). Each request carries what was typed in its query string, so a request per keystroke would be
 * a dozen half-words in the access log for one search; one request per pause is one.
 */
const SEARCH_DEBOUNCE_MS = 300;

type TrangThaiTai<T> =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: T };

function taiTu<T>(daTai: { khoa: string; kq: KetQua<T> } | null, khoa: string): TrangThaiTai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

/** Bộ lọc §6 mà màn giữ, KHÔNG gồm con trỏ — con trỏ do ngăn xếp phân trang giữ. */
type BoLocMan = {
  /** One of the six types — the open tab. Never empty: the prototype has no `Tất cả` tab. */
  readonly loai: string;
  readonly danhMucID: string;
  readonly tim: string;
  /** §6 `Trạng thái` — `""` = every status (no `status` sent). */
  readonly status: string;
};

/** The screen opens on `Tin tức`, the prototype's first tab (`ContentWorkspace.tsx:101`). */
const LOC_TRONG: BoLocMan = { loai: LOAI_MAC_DINH, danhMucID: "", tim: "", status: "" };

/**
 * The editor dialog: 600px, the prototype's `sm:max-w-150` (`ContentItemForm.tsx:198`). `p-0`: the form
 * draws its own header and scroll box edge to edge.
 */
const EDITOR_DIALOG_WIDTH = "w-[min(37.5rem,calc(100vw-1rem))]";
const EDITOR_DIALOG_CLASS = `${EDITOR_DIALOG_WIDTH} p-0`;
/** `Danh mục tin` — the prototype's `sm:max-w-[44rem]` (`CategoryManagerDialog.tsx:90`). */
const CATEGORY_DIALOG_CLASS = "w-[min(44rem,calc(100vw-1rem))] p-5";

/**
 * The table frame scrolls SIDEWAYS only (`.bang-cuon`'s `overflow-x: auto`, for narrow widths): the PAGE
 * scrolls vertically, as the prototype's (`ContentWorkspace.tsx:288-289`) — no box height, no sticky
 * header. 48px rows (spec v2).
 */
const TABLE_SCROLL_CLASS = "bang-cuon m-0 overflow-x-auto rounded-none border-0 shadow-none [&_tbody_td]:h-12";

/** The filter row's native selects — the prototype's `h-9 rounded-md border bg-white px-3 text-[12.5px]`. */
const FILTER_SELECT_CLASS = cn(controlClass, "h-9 w-auto max-w-full bg-white px-3 text-[12.5px] md:text-[12.5px]");

/**
 * A label without the symbol it starts with (`⊞ Danh mục tin` → `Danh mục tin`, `+ Thêm nội dung` →
 * `Thêm nội dung`): where a lucide icon now draws it (ADR 0068 §2, no emoji as icon). The words themselves
 * stay the labels file's, unchanged.
 */
function withoutLeadingGlyph(label: string): string {
  return label.replace(/^[^\p{L}\p{N}]+\s*/u, "");
}

export function SoNoiDung() {
  const [loc, datLoc] = useState<BoLocMan>(LOC_TRONG);
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_comms_noiDungRa>;
  } | null>(null);

  const [danhMuc, datDanhMuc] = useState<KetQua<comms_danhSachDanhMucRa> | null>(null);
  const [lanTaiDanhMuc, datLanTaiDanhMuc] = useState(0);

  const [dangMoThem, datDangMoThem] = useState(false);
  const [dangMoDanhMuc, datDangMoDanhMuc] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loiForm, datLoiForm] = useState<string | null>(null);
  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của biểu mẫu, nên một lần ghi xong là một lần biểu
  // mẫu dựng lại từ đầu: các ô trống trở lại VÀ một khoá chống trùng mới được sinh. Lần ghi HỎNG
  // thì không tăng — biểu mẫu giữ nguyên chữ đã gõ và giữ nguyên khoá cũ, đúng điều
  // `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // Id của hàng đang sửa, KHÔNG phải cả bản ghi: hàng của DANH SÁCH không mang thân bài, nên mở
  // biểu mẫu sửa từ nó sẽ cho một ô nội dung rỗng và lần Lưu đầu tiên xoá trắng bài viết. Toàn văn
  // phải đi hỏi tuyến chi tiết.
  const [dangSuaID, datDangSuaID] = useState<string | null>(null);
  const [daTaiChiTiet, datDaTaiChiTiet] = useState<{
    khoa: string;
    kq: KetQua<comms_noiDungRa>;
  } | null>(null);

  // The row whose `🗑` was pressed — the whole row, since the dialog shows only its title (no body needed).
  const [deleting, setDeleting] = useState<comms_noiDungRa | null>(null);
  // The row whose `Đăng` / `Gỡ` is in flight: its button is held so a double press sends one PATCH.
  const [publishingId, setPublishingId] = useState<string | null>(null);
  // A held audio file is being uploaded after its item was saved — Lưu then says `Đang tải tệp…`.
  const [audioUploading, setAudioUploading] = useState(false);
  // The outcome of a held file's upload that did not finish, handed to the edit dialog of the saved item.
  const [audioCarry, setAudioCarry] = useState<{ state: AudioUploadState; duration: string } | null>(null);

  const khoaSo = `${loc.loai}|${loc.danhMucID}|${loc.tim}|${loc.status}|${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    const yc: BoLocNoiDung = {
      loai: loc.loai,
      danhMucID: loc.danhMucID,
      tim: loc.tim,
      status: loc.status,
      limit: PAGE_SIZE,
      cursor: nganXep.hienTai,
    };
    laySoNoiDung(yc).then((kq) => {
      if (!bo) datDaTai({ khoa: khoaSo, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc.loai, loc.danhMucID, loc.tim, loc.status, nganXep.hienTai, khoaSo]);

  // Filter as you type (prototype), one request per pause: the typed text becomes the filter
  // `SEARCH_DEBOUNCE_MS` after the last keystroke, and a new filter starts again from page 1.
  useEffect(() => {
    const q = tim.trim();
    if (q === loc.tim) return;
    const t = setTimeout(() => {
      datNganXep(TRANG_DAU);
      datLoc((l) => ({ ...l, tim: q }));
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [tim, loc.tim]);

  useEffect(() => {
    let bo = false;
    layDanhMucNoiDung().then((kq) => {
      if (!bo) datDanhMuc(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanTaiDanhMuc]);

  const khoaChiTiet = `${dangSuaID ?? ""}|${lanGhiXong}`;

  useEffect(() => {
    if (dangSuaID === null) return;
    let bo = false;
    layMotNoiDung(dangSuaID).then((kq) => {
      if (!bo) datDaTaiChiTiet({ khoa: khoaChiTiet, kq });
    });
    return () => {
      bo = true;
    };
  }, [dangSuaID, khoaChiTiet]);

  // §4's count. `null` = still loading: the card shows no number until the server has answered.
  const [publishedCount, setPublishedCount] = useState<KetQua<number> | null>(null);

  useEffect(() => {
    let gone = false;
    // THE DOMAIN THE BROWSER IS ON is the one to ask about. This page rendered at all, so the edge
    // resolved this Host to a commune (`layCauHinhXa`, 404 otherwise); the route resolves the `host` it
    // is given through the same platform registry. The commune context deliberately carries no `host`
    // (`lib/cau-hinh-xa-hien-thi.ts`), so a second copy is not invented here. The route answers an
    // unknown domain with `{items: []}` — which is why it must be THIS page's domain, never a typed one.
    countPublishedStaff(window.location.hostname).then((r) => {
      if (!gone) setPublishedCount(r);
    });
    return () => {
      gone = true;
    };
  }, []);

  const session = usePhien();
  const canEdit = canEditContent(session);

  const so = taiTu(daTai, khoaSo);
  const dsDanhMuc: readonly comms_danhMucRa[] =
    danhMuc !== null && danhMuc.ok ? danhMuc.duLieu.items : [];
  const chiTiet = dangSuaID === null ? null : taiTu(daTaiChiTiet, khoaChiTiet);

  /** Sau mỗi lần ghi xong: về trang đầu và đọc lại sổ. */
  function taiLaiSo(): void {
    datNganXep(TRANG_DAU);
    datLanTai((n) => n + 1);
  }

  /** A filter changed: back to page 1 (prototype `setPage(0)` on every tab and filter). */
  function setFilter(next: Partial<BoLocMan>): void {
    datNganXep(TRANG_DAU);
    datLoc((l) => ({ ...l, ...next }));
  }

  function dongMoiBieuMau(): void {
    datDangMoThem(false);
    datDangMoDanhMuc(false);
    datDangSuaID(null);
    datLoiForm(null);
    setDeleting(null);
    setAudioCarry(null);
  }

  /**
   * After the delete dialog closed on a 204 or a 404: say what happened (a toast, as the prototype's) and
   * read the SAME page again (one back if that row was its last — `pageAfterDelete`), not the first page as
   * after a save: the officer clearing several rows on page 3 stays on page 3.
   */
  function afterDelete(gone: boolean): void {
    const rows = so.pha === "xong" ? so.duLieu.items.length : 0;
    setDeleting(null);
    if (gone) toast.info(CONTENT_DELETE_GONE);
    else toast.success(CONTENT_DELETED);
    datNganXep((stack) => pageAfterDelete(stack, rows));
    datLanTai((n) => n + 1);
  }

  /** Esc on an open overlay: refused while a save is in flight, like the forms' own `Huỷ`. */
  function dismissOverlay(): void {
    if (!dangGui) dongMoiBieuMau();
  }

  /**
   * After a save that returned the item: upload the audio file held for it, then `done()` — one press for
   * the officer, the file after the item (prototype `ContentItemForm.tsx:150-180`), with the existing
   * one-request upload (`runAudioUpload`). Nothing held → `done()` at once.
   *
   * A FAILED UPLOAD LEAVES THE ITEM SAVED, AND A DIALOG OPEN: the EDIT dialog of that item — never the
   * create form again, whose second Lưu would be a second item — with the prototype's sentence and the
   * upload's own reason (`Hoàn tất lại` there works on the same file, with the duration typed for it).
   */
  function uploadHeldAudio(id: string, audio: HeldAudio | undefined, done: () => void): void {
    if (audio === undefined) {
      setAudioCarry(null);
      done();
      return;
    }
    setAudioUploading(true);
    void runAudioUpload(audio.file, id, audio.duration, () => {})
      .catch((): AudioUploadState => ({ kind: "refused", message: AUDIO_STORAGE_FAILED }))
      .then((s) => {
        setAudioUploading(false);
        if (s.kind === "ready") {
          setAudioCarry(null);
          done();
          return;
        }
        datDangGui(false);
        toast.error(AUDIO_SAVED_NOT_UPLOADED);
        datDangMoThem(false);
        setAudioCarry({ state: s, duration: audio.duration });
        datLoiForm(AUDIO_SAVED_NOT_UPLOADED);
        datDangSuaID(id);
        datLanGhiXong((n) => n + 1);
        taiLaiSo();
      });
  }

  function themBai(gt: GiaTriFormNoiDung, khoa: string, audio?: HeldAudio): void {
    datDangGui(true);
    themNoiDung(thanThem(gt), khoa).then((kq) => {
      if (!kq.ok) {
        datDangGui(false);
        // The prototype's toast says it failed; the server's sentence stays INLINE, VERBATIM — its 400
        // names the field and the rule, and rewriting it here would be a second copy of a business rule.
        toast.error(SAVE_FAILED_TOAST);
        datLoiForm(kq.thongBao);
        return;
      }
      uploadHeldAudio(kq.duLieu.id, audio, () => {
        datDangGui(false);
        toast.success(saveToast(false, gt.publish));
        datLoiForm(null);
        datDangMoThem(false);
        datLanGhiXong((n) => n + 1);
        taiLaiSo();
      });
    });
  }

  function suaBai(
    id: string,
    dau: GiaTriFormNoiDung,
    moi: GiaTriFormNoiDung,
    audio?: HeldAudio,
    audioPatch: AudioPatch = {},
  ): void {
    // The audio half (`savedAudioPatch`): remove the attached file before a replacement is uploaded, or
    // correct the typed duration. Same PATCH as the other boxes — one write, one trail entry.
    const than = { ...thanSua(dau, moi), ...audioPatch };
    if (!coThayDoi(than) && audio === undefined) {
      // KHÔNG GỌI `PATCH` RỖNG. Một lần Lưu không đổi gì vẫn là một lần ghi ở máy chủ, và §10.4 gắn cờ
      // "đã sửa tay" lên bài — cờ ấy đưa bài ra khỏi lượt đồng bộ VĨNH VIỄN. (The row's `Đăng` / `Gỡ` is
      // a real write and DOES set that flag on a synced item; the owner accepted it, D1 09/10/2026 — it is
      // the officer's deliberate act, unlike an empty save.)
      datLoiForm(KHONG_CO_GI_DOI);
      return;
    }
    datDangGui(true);
    // A file held on the form goes after the PATCH, under the same id.
    const thenUpload = () =>
      uploadHeldAudio(id, audio, () => {
        datDangGui(false);
        toast.success(SAVED_TOAST);
        datLoiForm(null);
        datDangSuaID(null);
        datLanGhiXong((n) => n + 1);
        taiLaiSo();
      });
    // Only a new file, on a broadcast with none attached: no box changed, so NO PATCH (an empty one is
    // still a write, and flags a synced item as hand-edited — see above). The upload alone.
    if (!coThayDoi(than)) {
      thenUpload();
      return;
    }
    suaNoiDung(id, than).then((kq) => {
      if (!kq.ok) {
        datDangGui(false);
        toast.error(SAVE_FAILED_TOAST);
        datLoiForm(kq.thongBao);
        return;
      }
      thenUpload();
    });
  }

  /**
   * The row's `Đăng` / `Gỡ` (owner D1, 09/10/2026; prototype `ContentWorkspace.tsx:144-151`): a PATCH whose
   * body is `{publish}` and NOTHING else (`publishToggleBody`). The same page is read again afterwards — the
   * row's status changed, and under a status filter it may leave the page.
   */
  function togglePublish(nd: comms_noiDungRa): void {
    const body = publishToggleBody(nd.status);
    setPublishingId(nd.id);
    suaNoiDung(nd.id, body).then((kq) => {
      setPublishingId(null);
      if (!kq.ok) {
        // The prototype's title alone (`ContentWorkspace.tsx:149`), no description.
        toast.error(PUBLISH_TOGGLE_FAILED);
        return;
      }
      toast.success(body.publish ? PUBLISHED_TOAST : UNPUBLISHED_TOAST);
      datLanTai((n) => n + 1);
    });
  }

  /**
   * The add row of `Danh mục tin`. NO `order`: the prototype's row has no such box, and the server's
   * `order` is optional (0 when absent — what this form used to send by default). The dialog STAYS OPEN
   * (prototype `CategoryManagerDialog.tsx:73-86`): the new row appears in the list below, the box empties.
   */
  function themDanhMuc(ten: string, slug: string, chaID: string, khoa: string): void {
    datDangGui(true);
    themDanhMucNoiDung({ name: ten, slug, parent_id: chaID }, khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiForm(kq.thongBao);
        return;
      }
      toast.success(CATEGORY_ADDED_TOAST);
      datLoiForm(null);
      datLanGhiXong((n) => n + 1);
      datLanTaiDanhMuc((n) => n + 1);
    });
  }

  const pageIndex = nganXep.daQua.length;

  return (
    <div className="man-noi-dung mt-0 min-w-0">
      {/* Prototype `ContentWorkspace.tsx:164-186`: title and sentence on the left, `Thêm nội dung` pushed
          right, bottoms aligned. No icon. */}
      <div className="mb-5 flex flex-wrap items-end gap-4">
        <div className="min-w-0">
          <h1 className="m-0 text-[22px] leading-tight font-bold text-navy">{TIEU_DE_MAN}</h1>
          <p className="m-0 mt-1 text-[13px] text-ink-muted">{MO_TA_MAN}</p>
        </div>
        <HeaderActions
          canEdit={canEdit}
          addOpen={dangMoThem}
          openAdd={() => {
            dongMoiBieuMau();
            datDangMoThem(true);
          }}
        />
      </div>

      <PortalSyncCard canEdit={canEdit} />
      <TheDanhBaChinhQuyen publishedCount={publishedCount} />

      {canEdit && dangMoThem && (
        <OverlayDialog titleId="tieu-de-form-noi-dung" onDismiss={dismissOverlay} className={EDITOR_DIALOG_CLASS}>
          <FormNoiDung
            // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
            key={`them|${lanGhiXong}`}
            tieuDeForm={TIEU_DE_FORM_THEM}
            moTa={MO_TA_FORM_THEM}
            // The new item's type is the tab being looked at (prototype `defaultKind={kind}`).
            giaTriDau={{ ...FORM_TRONG, type: loc.loai }}
            danhMuc={dsDanhMuc}
            dangGui={dangGui}
            uploading={audioUploading}
            loi={loiForm}
            huy={dongMoiBieuMau}
            luu={(gt, khoa, audio) => themBai(gt, khoa, audio)}
          />
        </OverlayDialog>
      )}

      {canEdit && dangMoDanhMuc && (
        // The prototype's `CategoryManagerDialog` (`sm:max-w-[44rem]`): plain title + one sentence, the
        // portal-import box, the add row, then the grouped list.
        <OverlayDialog titleId="tieu-de-hop-danh-muc" onDismiss={dismissOverlay} className={CATEGORY_DIALOG_CLASS}>
          <div className="mb-4 flex items-start gap-3">
            <div className="min-w-0 flex-1">
              <h3 id="tieu-de-hop-danh-muc" className="m-0 text-lg leading-snug font-semibold text-navy">
                {CATEGORY_DIALOG_TITLE}
              </h3>
              <p className="m-0 mt-1 text-[13px] text-ink-muted">{CATEGORY_DIALOG_DESCRIPTION}</p>
            </div>
            <IconButton type="button" label={CLOSE_LABEL} className="-mt-1 -mr-2 min-h-0" disabled={dangGui} onClick={dongMoiBieuMau}>
              <X aria-hidden="true" focusable="false" />
            </IconButton>
          </div>
          <div className="flex flex-col gap-4">
            <CategoryImportPending />
            <FormDanhMuc
              key={`danh-muc|${lanGhiXong}`}
              danhMuc={dsDanhMuc}
              dangGui={dangGui}
              loi={loiForm}
              luu={themDanhMuc}
            />
            {danhMuc === null ? (
              <Skeleton className="h-40 w-full" />
            ) : !danhMuc.ok ? (
              <p className="m-0 text-[12px] font-medium text-danger" role="alert">
                {danhMuc.thongBao}
              </p>
            ) : (
              <CategoryAdmin
                categories={dsDanhMuc}
                update={updateContentCategory}
                remove={deleteContentCategory}
                changed={() => datLanTaiDanhMuc((n) => n + 1)}
              />
            )}
          </div>
        </OverlayDialog>
      )}

      {canEdit && chiTiet !== null && dangSuaID !== null && (
        <OverlayDialog
          titleId="tieu-de-form-noi-dung"
          onDismiss={dismissOverlay}
          className={chiTiet.pha === "xong" ? EDITOR_DIALOG_CLASS : EDITOR_DIALOG_WIDTH}
        >
          {chiTiet.pha === "xong" ? (
            <FormNoiDung
              key={`sua|${dangSuaID}|${lanGhiXong}`}
              // The prototype's title, verbatim ("Sửa nội dung"); the article's own title is the first box.
              tieuDeForm={EDIT_FORM_TITLE}
              moTa={MO_TA_FORM_THEM}
              giaTriDau={giaTriTuHang(chiTiet.duLieu)}
              hang={chiTiet.duLieu}
              danhMuc={dsDanhMuc}
              dangGui={dangGui}
              uploading={audioUploading}
              loi={loiForm}
              huy={dongMoiBieuMau}
              luu={(gt, _khoa, audio, audioPatch) => suaBai(dangSuaID, giaTriTuHang(chiTiet.duLieu), gt, audio, audioPatch)}
              initialAudioState={audioCarry?.state}
              initialAudioDuration={audioCarry?.duration}
            />
          ) : (
            // The dialog opens at once on `✎`; the full text arrives a moment later. Until then it
            // carries a heading (its accessible name) and a way out.
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-3 border-b border-line pb-3">
                <h3 id="tieu-de-form-noi-dung" className="m-0 min-w-0 flex-1 text-base font-semibold">
                  {EDIT_FORM_TITLE}
                </h3>
                <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" onClick={dongMoiBieuMau}>
                  <X aria-hidden="true" focusable="false" />
                </IconButton>
              </div>
              {chiTiet.pha === "dangTai" ? (
                <>
                  <p role="status" className="an-thi-giac">
                    {DANG_TAI_TOAN_VAN}
                  </p>
                  <FormSkeleton />
                </>
              ) : (
                <p className="thong-bao-loi m-0 flex items-start gap-2" role="alert">
                  <CloudOff aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5 size-[18px] shrink-0" />
                  <span>{chiTiet.thongBao}</span>
                </p>
              )}
              <div className="flex justify-end">
                <Button type="button" variant="secondary" onClick={dongMoiBieuMau}>
                  {CLOSE_LABEL}
                </Button>
              </div>
            </div>
          )}
        </OverlayDialog>
      )}

      {canEdit && deleting !== null && (
        <ContentDeleteDialog
          key={deleting.id}
          item={deleting}
          remove={deleteContentItem}
          deleted={() => afterDelete(false)}
          gone={() => afterDelete(true)}
          close={() => setDeleting(null)}
        />
      )}

      {/* The prototype's list, in its order: the type tabs (a pill bar), the filter row, then ONE bordered
          frame holding the table and its pagination. The heading stays for screen readers only — the
          prototype draws none, and a section without a name is a landmark nobody can find. */}
      <section className="min-w-0" aria-labelledby="tieu-de-so-noi-dung">
        <h2 id="tieu-de-so-noi-dung" className="an-thi-giac">
          Sổ nội dung Mini App
        </h2>
        <ThanhTabLoai loai={loc.loai} datLoai={(l) => setFilter({ loai: l })} />

        <div
          role="tabpanel"
          id={CONTENT_TABPANEL_ID}
          aria-labelledby={contentTabId(loc.loai)}
          // The panel itself takes focus after the tablist (WAI-ARIA tabs): Tab from the selected tab
          // lands here, not on the first filter box.
          tabIndex={0}
          className="min-w-0"
        >
          <HangLocNoiDung
            danhMucID={loc.danhMucID}
            datDanhMucID={(id) => setFilter({ danhMucID: id })}
            tim={tim}
            datTim={datTim}
            danhMuc={dsDanhMuc}
            status={loc.status}
            setStatus={(st) => setFilter({ status: st })}
            // `Danh mục tin` closes the filter row. `content.update` only — it opens the tree's
            // edit/hide/delete and the add form, nothing to read in it that the filter does not show.
            extra={
              canEdit ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="ml-auto"
                  aria-haspopup="dialog"
                  icon={<FolderTree aria-hidden="true" focusable="false" className="size-4" />}
                  onClick={() => {
                    dongMoiBieuMau();
                    datDangMoDanhMuc(true);
                  }}
                >
                  {withoutLeadingGlyph(NHAN_NUT_DANH_MUC)}
                </Button>
              ) : null
            }
          />

          {danhMuc !== null && !danhMuc.ok && (
            <p className="thong-bao-loi mb-4" role="alert">
              {danhMuc.thongBao}
            </p>
          )}

          {/* LOADING: the prototype's one `h-80` block; the sentence stays the live announcement. */}
          {so.pha === "dangTai" && (
            <>
              <p role="status" className="an-thi-giac">
                {DANG_TAI_SO}
              </p>
              <Skeleton className="h-80 w-full rounded-[10px]" />
            </>
          )}

          {/* LOAD ERROR: the server's sentence VERBATIM stays the alert; `Tải lại` asks the same read again
              through the screen's existing re-read key (`lanTai`) — no new call, no new route. */}
          {so.pha === "loi" && (
            <div className="min-w-0 overflow-hidden rounded-[10px] border border-line bg-white">
              <EmptyState
                icon={CloudOff}
                title="Chưa tải được danh sách nội dung"
                description={
                  <span className="text-danger-600" role="alert">
                    {so.thongBao}
                  </span>
                }
                action={
                  <Button
                    type="button"
                    variant="secondary"
                    icon={<RefreshCw aria-hidden="true" focusable="false" />}
                    onClick={() => datLanTai((n) => n + 1)}
                  >
                    Tải lại
                  </Button>
                }
              />
            </div>
          )}

          {so.pha === "xong" && (
            <div className="min-w-0 overflow-hidden rounded-[10px] border border-line bg-white">
              <BangNoiDung
                ds={so.duLieu.items}
                danhMuc={dsDanhMuc}
                sua={(id) => {
                  dongMoiBieuMau();
                  datDangSuaID(id);
                }}
                remove={(nd) => {
                  dongMoiBieuMau();
                  setDeleting(nd);
                }}
                togglePublish={togglePublish}
                publishingId={publishingId}
                canEdit={canEdit}
              />
              <ContentPager
                pageIndex={pageIndex}
                rows={so.duLieu.items.length}
                // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
                hasNext={so.duLieu.has_more && so.duLieu.next_cursor !== ""}
                previous={() => datNganXep(veTrangTruoc(nganXep))}
                next={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
              />
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

/**
 * The page header's action: `Thêm nội dung` — drawn as the prototype draws it, a `Plus` icon and the words
 * (`NHAN_NUT_THEM` without its "+", the icon now says it), pushed right (`ml-auto`). Drawn only with
 * `content.update` (`canEditContent`): before the session is read it is absent too, so it can appear but
 * never flash and vanish.
 */
export function HeaderActions({
  canEdit,
  addOpen,
  openAdd,
}: {
  canEdit: boolean;
  addOpen: boolean;
  openAdd: () => void;
}) {
  if (!canEdit) return null;
  return (
    <Button
      type="button"
      variant="primary"
      className="ml-auto"
      aria-haspopup="dialog"
      aria-expanded={addOpen}
      icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
      onClick={openAdd}
    >
      {withoutLeadingGlyph(NHAN_NUT_THEM)}
    </Button>
  );
}

/**
 * Thẻ §4 — Danh bạ chính quyền, drawn as the prototype's link card (`ContentWorkspace.tsx:194-209`): the
 * whole card is ONE link to the `Danh bạ cán bộ` tab of this screen, icon · title over the line · `Mở →`.
 *
 * THE NUMBER IS SHOWN ONLY AFTER THE SERVER ANSWERED (`publishedStaffText`). While loading, or when the
 * read failed, the line carries no number at all — a `0` there would tell the commune that residents
 * see nobody. Presentational: `SoNoiDung` reads the count (`countPublishedStaff`).
 */
export function TheDanhBaChinhQuyen({ publishedCount }: { publishedCount: KetQua<number> | null }) {
  const text = publishedStaffText(publishedCount);
  return (
    <Link
      href={miniAppTabHref("danh-ba")}
      className="mb-5 flex min-w-0 items-center gap-3 rounded-[10px] border border-line bg-white px-4 py-3 no-underline transition-colors hover:border-brand/40 hover:bg-brand/4"
    >
      <BookUser aria-hidden="true" focusable="false" className="size-5 shrink-0 text-brand" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span id="government-directory-title" className="text-[13px] font-semibold text-navy">
          {TIEU_DE_THE_DANH_BA}
        </span>
        <span className="text-[12px] text-ink-muted">{text.line}</span>
        {text.note !== null && <span className="ghi-chu text-xs">{text.note}</span>}
      </span>
      <span className="shrink-0 text-[12.5px] font-semibold text-brand">
        Mở <span aria-hidden="true">→</span>
      </span>
    </Link>
  );
}

/** The one panel the content-type tabs control: the filter row, the table and its pagination. */
export const CONTENT_TABPANEL_ID = "content-type-panel";

/** The id of one type's tab. */
export function contentTabId(type: string): string {
  return `content-type-tab-${type}`;
}

/**
 * The six type tabs of §5 — EXACTLY the prototype's six (`ContentWorkspace.tsx:75-86`), `Tin tức` open by
 * default, no `Tất cả`: each tab IS one type, so the table needs no `Loại` column.
 *
 * REAL TABS, THE CẤU HÌNH SCREEN'S PATTERN (`features/cau-hinh/khung-tab-cau-hinh.tsx`): `tablist` /
 * `tab` / `tabpanel`, one tab in the Tab order (roving `tabIndex`), ← → Home End move AND select
 * (`tabKeTheoPhim`, reused). ONE panel for six tabs: every tab shows the same table under a different
 * `type` filter, so all six `aria-controls` name it.
 */
export function ThanhTabLoai({
  loai,
  datLoai,
}: {
  loai: string;
  datLoai: (l: string) => void;
}) {
  const types: readonly string[] = MOI_LOAI;
  const buttons = useRef<Record<string, HTMLButtonElement | null>>({});

  function onKey(e: KeyboardEvent<HTMLButtonElement>, index: number): void {
    const next = tabKeTheoPhim(e.key, index, types.length);
    const to = next === null ? undefined : types[next];
    if (to === undefined) return;
    e.preventDefault();
    datLoai(to);
    buttons.current[to]?.focus();
  }

  return (
    // The prototype's pill bar: `w-fit gap-1 rounded-[10px] border bg-surface p-1` (its `bg-surface` is
    // this app's `bg-canvas`), the open tab white and raised. `flex-wrap` + `max-w-full`: six tabs wrap at
    // 320px instead of spilling past the page edge.
    <div
      className="mb-4 flex w-fit max-w-full flex-wrap gap-1 rounded-[10px] border border-line bg-canvas p-1"
      role="tablist"
      aria-label="Loại nội dung"
    >
      {types.map((type, i) => (
        <button
          key={type}
          ref={(el) => {
            buttons.current[type] = el;
          }}
          type="button"
          role="tab"
          id={contentTabId(type)}
          aria-selected={loai === type}
          aria-controls={CONTENT_TABPANEL_ID}
          tabIndex={loai === type ? 0 : -1}
          onClick={() => datLoai(type)}
          onKeyDown={(e) => onKey(e, i)}
          className={cn(
            "flex cursor-pointer items-center gap-1.5 rounded-[8px] border-0 px-3 py-1.5 [font-family:inherit] text-[12.5px] font-semibold whitespace-nowrap",
            loai === type ? "bg-white text-navy shadow-card" : "bg-transparent text-ink-muted hover:text-navy",
          )}
        >
          {createElement(typeTabIcon(type), { "aria-hidden": true, focusable: "false", className: "size-3.5" })}
          {nhanLoai(type)}
        </button>
      ))}
    </div>
  );
}

/** The prototype's tab icons: radio for broadcasts, video, images for banners, a newspaper for the rest. */
function typeTabIcon(type: string): LucideIcon {
  if (type === CONTENT_TYPE_BROADCAST) return Radio;
  if (type === CONTENT_TYPE_VIDEO) return Video;
  if (type === CONTENT_TYPE_BANNER) return Images;
  return Newspaper;
}

/**
 * Hàng lọc §6 — the prototype's row (`ContentWorkspace.tsx:231-283`): the search box, the category select
 * (only once the commune has a visible category), then (owner D1) the status select, and what closes the row
 * pushed right.
 *
 * THE SEARCH FILTERS AS YOU TYPE — no `Tìm` button. The parent waits for a pause (`SEARCH_DEBOUNCE_MS`) before
 * asking the server; the text stays in component state, never in the page's address.
 *
 * THE STATUS FILTER IS THE SERVER'S (`status`, 02/10/2026 C1), never a filter of the page in hand: a
 * client-side filter would hide every `Chờ duyệt` item on the other pages.
 */
export function HangLocNoiDung({
  danhMucID,
  datDanhMucID,
  tim,
  datTim,
  danhMuc,
  status,
  setStatus,
  extra,
}: {
  danhMucID: string;
  datDanhMucID: (id: string) => void;
  tim: string;
  datTim: (s: string) => void;
  danhMuc: readonly comms_danhMucRa[];
  status: string;
  setStatus: (status: string) => void;
  /** What closes the row — `Danh mục tin`, when the account may edit. */
  extra?: ReactNode;
}) {
  // Hidden categories are not offered as a filter (prototype: active ones only); the label still names the
  // parent from the FULL list, so a child of a hidden parent reads `Cha › Con` like the others.
  const visible = danhMuc.filter((d) => !d.hidden);

  function onSubmit(e: FormEvent) {
    // Enter changes nothing: the filter already follows the box.
    e.preventDefault();
  }

  return (
    <div className="mb-4 flex min-w-0 flex-wrap items-center gap-2.5">
      <form className="relative m-0 w-72 max-w-full" role="search" onSubmit={onSubmit}>
        <label htmlFor="tim-noi-dung" className="an-thi-giac">
          Tìm theo tiêu đề
        </label>
        <Search
          aria-hidden="true"
          focusable="false"
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-muted"
        />
        <input
          id="tim-noi-dung"
          name="tim-noi-dung"
          value={tim}
          placeholder={TIM_PLACEHOLDER}
          autoComplete="off"
          // Máy chủ trả 400 khi quá trần, và câu từ chối CỐ Ý không nhắc lại chữ vừa gõ. Chặn ở ô nhập
          // để cán bộ thấy giới hạn thay vì thấy "không tải được".
          maxLength={TU_KHOA_TIM_TOI_DA}
          className={cn(controlClass, "h-9 pl-9 text-[12.5px] md:text-[12.5px]")}
          onChange={(e) => datTim(e.target.value)}
        />
      </form>

      {/* Prototype: the category select only once the commune has categories — a select holding only
          "Tất cả danh mục" takes room and does nothing. Kept while a category is the active filter. */}
      {(visible.length > 0 || danhMucID !== "") && (
        <>
          <label htmlFor="loc-danh-muc" className="an-thi-giac">
            Lọc theo danh mục
          </label>
          <select
            id="loc-danh-muc"
            className={FILTER_SELECT_CLASS}
            value={danhMucID}
            onChange={(e) => datDanhMucID(e.target.value)}
          >
            <option value="">{MOI_DANH_MUC}</option>
            {dungCayDanhMuc(visible).map((m) => (
              <option key={m.dm.id} value={m.dm.id}>
                {categoryPath(m.dm, danhMuc)}
              </option>
            ))}
          </select>
        </>
      )}

      <label htmlFor="loc-trang-thai" className="an-thi-giac">
        Trạng thái
      </label>
      <select id="loc-trang-thai" className={FILTER_SELECT_CLASS} value={status} onChange={(e) => setStatus(e.target.value)}>
        {STATUS_FILTER_OPTIONS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>

      {extra}
    </div>
  );
}

/**
 * Bảng §6 — the prototype's columns (`ContentWorkspace.tsx:290-300`): a thumbnail (a "?" — the list route
 * serves no image link), Tiêu đề, Chuyên mục, Tệp đính kèm, Ngày đăng, Lượt xem, Trạng thái, and the
 * actions with no header text.
 *
 * THE ACTION COLUMN HOLDS `✎`, `Đăng`/`Gỡ` AND `🗑`. `🗑` only OPENS the dialog that asks for the reason —
 * nothing is deleted on the click itself. The title is a button opening the same edit dialog as `✎`.
 *
 * KHÔNG CỘT NÀO DỰNG HTML. `title` và `summary` là văn bản thuần ở đây; thân bài không có mặt trong phản
 * hồi danh sách và màn hình không đi đoán nó.
 *
 * WITHOUT `content.update` THE ACTION COLUMN IS NOT DRAWN AT ALL (`canEdit`), and the title is plain text.
 */
export function BangNoiDung({
  ds,
  danhMuc,
  sua,
  remove,
  togglePublish,
  publishingId = null,
  canEdit,
}: {
  ds: readonly comms_noiDungRa[];
  danhMuc: readonly comms_danhMucRa[];
  sua: (id: string) => void;
  /** Opens the delete dialog for this row. */
  remove: (nd: comms_noiDungRa) => void;
  /** `Đăng` / `Gỡ` pressed on this row. */
  togglePublish: (nd: comms_noiDungRa) => void;
  /** The row whose toggle is in flight — its button is held. */
  publishingId?: string | null;
  canEdit: boolean;
}) {
  const columns = canEdit ? 8 : 7;
  return (
    <div className={TABLE_SCROLL_CLASS}>
      {/* The prototype's widths (`w-16 / w-48 / w-28 / w-32 / w-24 / w-28 / w-32`) and, as there, the title
          capped by an absolute width (`max-w-[34rem]`): a long title can never push Ngày đăng and Trạng thái
          off the frame — it is cut by "…" with the whole title on hover. Below `min-w` the frame scrolls
          sideways, inside itself. */}
      <table className="bang-can-bo w-full min-w-[56rem]">
        <caption className="an-thi-giac">Sổ nội dung Mini App của xã</caption>
        <thead>
          <tr>
            {/* The prototype's thumbnail column, no header text: the list route serves no image link (only
                the detail does), so it holds the "?" of a part not built (`PHAN_CHUA_DUNG`). */}
            <ThumbnailHeader />
            <th scope="col">Tiêu đề</th>
            <th scope="col" className="w-48">
              Chuyên mục
            </th>
            <th scope="col" className="w-28">
              Tệp đính kèm
            </th>
            <th scope="col" className="w-32">
              Ngày đăng
            </th>
            {/* Not sortable (owner, 02/10/2026): keyset paging over a number that moves while the
                officer pages would skip and repeat rows. */}
            <th scope="col" className="w-24 text-right">
              {VIEW_COUNT_LABEL}
            </th>
            <th scope="col" className="w-28 text-center">
              Trạng thái
            </th>
            {canEdit && (
              <th scope="col" className="w-32">
                <span className="an-thi-giac">{ACTIONS_COLUMN_LABEL}</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {ds.length === 0 && (
            <tr>
              <td colSpan={columns} className="py-12 text-center text-ink-muted">
                {SO_RONG}
              </td>
            </tr>
          )}
          {ds.map((nd) => {
            const trich = trichTomTat(nd.summary);
            const category = categoryCellLabel(nd.category_id, danhMuc);
            const portalCategory = nd.source === SOURCE_PORTAL_SYNC ? portalCategoryName(nd) : "";
            const showing = nd.status === "dang-hien";
            return (
              <tr key={nd.id}>
                <PendingCell className="align-top" />
                <td className="max-w-[34rem] align-top">
                  <div className="max-w-[34rem] min-w-0">
                    {canEdit ? (
                      // `block w-full truncate`, not a wrap: a button shrinks to its text, so only a block
                      // of the cell's width can cut a long title instead of letting it spill over.
                      <button
                        type="button"
                        title={nd.title}
                        aria-haspopup="dialog"
                        className="m-0 block w-full cursor-pointer truncate border-0 bg-transparent p-0 text-left [font-family:inherit] [font-size:inherit] font-semibold text-navy hover:underline"
                        onClick={() => sua(nd.id)}
                      >
                        {nd.title}
                      </button>
                    ) : (
                      <span className="block truncate font-semibold text-navy" title={nd.title}>
                        {nd.title}
                      </span>
                    )}
                    {/* One line, not two: this column is for recognising an item, not reading it. */}
                    {trich !== "" && <p className="m-0 mt-0.5 truncate text-[11.5px] text-ink-muted">{trich}</p>}
                    {/* The portal category a synced item came in under (ADR 0067 §2 #5, C2 02/10/2026). */}
                    {portalCategory !== "" && (
                      <p className="m-0 mt-0.5 truncate text-[11.5px] text-ink-muted">
                        {portalCategoryLabel(portalCategory)}
                      </p>
                    )}
                  </div>
                </td>
                <td className="align-top text-[12px]">
                  <span className="block max-w-[14rem] truncate text-ink-muted" title={category}>
                    {category}
                  </span>
                </td>
                <td className="align-top text-[11.5px]">
                  <AttachmentCell nd={nd} />
                </td>
                <td className="align-top text-[12px]">{nhanNgayDang(nd.published_on)}</td>
                <td className="align-top text-right text-[12px] tabular-nums">
                  <span className="inline-flex items-center justify-end gap-1 whitespace-nowrap text-ink-muted">
                    <Eye aria-hidden="true" focusable="false" className="size-3" />
                    {viewCountText(nd.view_count)}
                  </span>
                </td>
                <td className="align-top text-center">
                  <ContentStatusBadge status={nd.status} />
                </td>
                {canEdit && (
                  <td className="align-top">
                    {/* The prototype's outline buttons, right-aligned: pencil, Đăng/Gỡ, trash. */}
                    <span className="flex justify-end gap-1">
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="min-h-0 px-2"
                        aria-haspopup="dialog"
                        onClick={() => sua(nd.id)}
                        // Ký hiệu một mình không đọc được bằng trình đọc màn hình, và mọi hàng đều mang
                        // cùng một ký hiệu. Nhãn mang theo tiêu đề để nói rõ đang sửa bài nào.
                        aria-label={`${NHAN_NUT_SUA} Sửa: ${nd.title}`}
                        title="Sửa"
                      >
                        <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
                      </Button>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="min-h-0"
                        disabled={publishingId === nd.id}
                        aria-busy={publishingId === nd.id || undefined}
                        // Same reason as the pencil: the word alone names no row.
                        aria-label={`${showing ? UNPUBLISH_BUTTON : PUBLISH_BUTTON}: ${nd.title}`}
                        onClick={() => togglePublish(nd)}
                      >
                        {showing ? UNPUBLISH_BUTTON : PUBLISH_BUTTON}
                      </Button>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="min-h-0 px-2 text-danger hover:not-disabled:text-danger"
                        aria-haspopup="dialog"
                        onClick={() => remove(nd)}
                        aria-label={contentDeleteAriaLabel(nd.title)}
                        title={CONTENT_DELETE_TITLE}
                      >
                        <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
                      </Button>
                    </span>
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** The thumbnail column's header: no words (the prototype draws none), only the "?" and its reason. */
function ThumbnailHeader() {
  return (
    <th scope="col" className="w-16" data-pending="">
      <span className="an-thi-giac">{THUMBNAIL_PART}</span>
      <PendingMarker info={pendingContentPart(THUMBNAIL_PART)} side="bottom" />
    </th>
  );
}

/** §6 `Tệp đính kèm` — `attachmentState` decides; this only draws it (icon + word, never colour alone). */
function AttachmentCell({ nd }: { nd: comms_noiDungRa }) {
  const a = attachmentState(nd);
  if (a.tone === "none") return <span className="text-ink-muted">{a.label}</span>;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 whitespace-nowrap",
        a.tone === "ok" && "text-leaf",
        a.tone === "missing" && "text-tangerine",
        a.tone === "muted" && "text-ink-muted",
      )}
    >
      {a.tone === "missing" ? (
        <TriangleAlert aria-hidden="true" focusable="false" className="size-3" />
      ) : (
        <Paperclip aria-hidden="true" focusable="false" className="size-3" />
      )}
      {a.label}
    </span>
  );
}

/**
 * The pager INSIDE the table frame (prototype `ContentWorkspace.tsx:492-525`): `a–b trên N mục` on the left,
 * `Trước` · `Trang x/y` · `Sau` on the right. The list route pages by cursor and never counts, so N and y
 * are each a disabled "?" (`PHAN_CHUA_DUNG`, ADR 0068 §14); `a–b` and `x` are known and drawn.
 *
 * HIDDEN when there is only one page — on the first page with nothing after it — as the prototype hides it
 * when everything fits on one page.
 */
export function ContentPager({
  pageIndex,
  rows,
  hasNext,
  previous,
  next,
}: {
  /** 0 = the first page. */
  pageIndex: number;
  rows: number;
  hasNext: boolean;
  previous: () => void;
  next: () => void;
}) {
  if (pageIndex === 0 && !hasNext) return null;
  return (
    <nav
      className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-2.5"
      aria-label="Phân trang sổ nội dung Mini App"
    >
      <p className="m-0 inline-flex items-center gap-1 text-[12px] text-ink-muted tabular-nums" data-pending="">
        {pageRange(pageIndex, rows)} trên
        <PendingMarker info={pendingContentPart(TOTAL_ITEMS_PART)} />
        mục
      </p>
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          icon={<ChevronLeft aria-hidden="true" focusable="false" className="size-4" />}
          disabled={pageIndex === 0}
          onClick={previous}
        >
          Trước
        </Button>
        <span className="inline-flex items-center gap-1 text-[12px] text-ink-muted tabular-nums" data-pending="">
          Trang {pageIndex + 1}/
          <PendingMarker info={pendingContentPart(TOTAL_PAGES_PART)} />
        </span>
        <Button type="button" variant="secondary" size="sm" disabled={!hasNext} onClick={next}>
          Sau
          <ChevronRight aria-hidden="true" focusable="false" className="size-4" />
        </Button>
      </div>
    </nav>
  );
}

/**
 * Biểu mẫu §7, dùng cho cả THÊM và SỬA — drawn as the prototype's `ContentItemForm` (`ContentItemForm.tsx:
 * 196-380`): title and the one sentence under it, then ONE scrolling column of boxes (no section headings),
 * then `Huỷ` · `Lưu`.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL, and it is one: `SoNoiDung` mounts it inside `OverlayDialog` (native
 * `<dialog>` + `showModal`, `overlay-dialog.tsx`). The form itself knows nothing of the overlay, so its
 * tests still render it alone.
 *
 * `Loại nội dung` IS LOCKED WHEN EDITING (prototype `:219`): an item keeps the type it was created with.
 *
 * Ô `Nội dung` IS THE TIPTAP EDITOR (`RichTextEditor`), limited to the server's allow-list. It reports
 * a change only when the officer edits, so `thanSua` never sends a body nobody touched.
 *
 * KHÔNG CÓ Ô `Trạng thái`: máy chủ suy trạng thái từ ô tích `Đăng lên Mini App`. Một thân tự khai
 * trạng thái là một bài đăng vượt qua bước duyệt mà §10.2 dành cho lượt đồng bộ.
 *
 * `Lưu` IS PRESSABLE WITH AN EMPTY TITLE (prototype): pressing it then says `Vui lòng nhập tiêu đề` under the
 * box instead of sending. It is held only while a file is still moving (`Đang tải tệp…`) or a per-type check
 * fails — saving then would drop the file, or be refused.
 *
 * `khoaChongTrung` SINH LÚC MỞ BIỂU MẪU, không lúc gửi: bấm lại sau một lỗi mạng phải dùng LẠI
 * đúng khoá ấy, vì lần gửi đầu có thể đã tới máy chủ và đã đăng một bài lên Mini App của cả xã.
 * Biểu mẫu SỬA không cần khoá (hợp đồng không đòi), nhưng vẫn nhận cùng chữ ký để hai lối gọi
 * không rẽ nhánh ở đây.
 */
export function FormNoiDung({
  tieuDeForm,
  moTa,
  giaTriDau,
  hang,
  danhMuc,
  dangGui,
  uploading = false,
  loi,
  huy,
  luu,
  initialCoverState,
  initialAudioState,
  initialAudioDuration,
  initialTitleTouched = false,
}: {
  tieuDeForm: string;
  moTa: string;
  giaTriDau: GiaTriFormNoiDung;
  /** Hàng gốc, chỉ có khi đang SỬA. */
  hang?: comms_noiDungRa;
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  /** The saved item's held audio file is being uploaded — Lưu says `Đang tải tệp…`. */
  uploading?: boolean;
  loi: string | null;
  huy: () => void;
  /**
   * `audio`: the picked audio file, uploaded once the save returns. `audioPatch`: the edit form's audio
   * fields for a broadcast that already has its file (`savedAudioPatch`) — `{}` otherwise.
   */
  luu: (gt: GiaTriFormNoiDung, khoaChongTrung: string, audio?: HeldAudio, audioPatch?: AudioPatch) => void;
  /** Tests only: render the cover block in a given upload state (this suite has no DOM events). */
  initialCoverState?: CoverUploadState;
  /** The audio's starting upload state — a held upload that did not finish after the save (and tests). */
  initialAudioState?: AudioUploadState;
  /** The duration typed with that held file. */
  initialAudioDuration?: string;
  /** Tests only: render as if Lưu had been pressed once. */
  initialTitleTouched?: boolean;
}) {
  const [gt, datGT] = useState<GiaTriFormNoiDung>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const [cover, setCover] = useState<CoverUploadState>(initialCoverState ?? { kind: "idle" });
  const [audioBusy, setAudioBusy] = useState(false);
  // Lưu pressed once: from then on an empty title is named under its box.
  const [titleTouched, setTitleTouched] = useState(initialTitleTouched);
  // THE ARTICLE EVERY FILE OF THIS FORM BELONGS TO: the saved article's id on the edit form; on the create
  // form, unknown until the first file — body image OR cover — reserves one (`body-image.ts`). Sent on
  // every later body image and on the cover, so the save finds all of them under one id. The first id
  // learned is kept (`a ?? id`).
  const [articleId, setArticleId] = useState<string | undefined>(hang?.id);
  // `file_id → preview_url` for the editor's figures: the detail's `body_images`, then each ready reply.
  const [previews, setPreviews] = useState<ReadonlyMap<string, string>>(() => previewsFromItem(hang?.body_images));
  const [bodyImageBusy, setBodyImageBusy] = useState(false);
  // The duration of the audio ALREADY attached to the saved item, as the box shows it; `null` = none. A
  // `Hoàn tất lại` that attaches the carried file sets it.
  const [attachedDuration, setAttachedDuration] = useState<string | null>(() =>
    hang !== undefined && (hang.audio_file_id ?? "") !== "" ? String(hang.audio_duration_seconds ?? "") : null,
  );
  // The audio file picked on this form, and the duration box — held here, sent only after the save
  // returns the item's id. The box opens on the attached file's duration (or the carried upload's).
  const [heldFile, setHeldFile] = useState<File | null>(null);
  const [heldDuration, setHeldDuration] = useState(initialAudioDuration ?? attachedDuration ?? "");
  // A held upload that did not finish after the save: its outcome, and `Hoàn tất lại` when the bytes are
  // already in the store (a new upload would meet 409 `audio_limit` while that file holds the one slot).
  const [audioState, setAudioState] = useState<AudioUploadState>(initialAudioState ?? { kind: "idle" });

  const editing = hang !== undefined;
  const tieuDeGon = gt.title.trim();
  const titleError = titleTouched && tieuDeGon === "" ? TITLE_REQUIRED : undefined;
  // The per-type check is shown LIVE next to the boxes and holds Lưu off: the author sees "end before
  // start" while fixing it, not after a round trip. It mirrors the server; the server's own 400 still
  // reaches `loi` word for word. The edit form passes its starting values: two banner rules depend on
  // what the item WAS.
  const typeFieldError = validateTypeFields(gt, editing ? giaTriDau : undefined);
  // A file still moving holds Lưu: saving now would drop the cover just chosen, hide an audio upload's
  // outcome, or lose a body image that is inserted only once ready.
  const coverBusy = coverInFlight(cover);
  const filesBusy = coverBusy || audioBusy || bodyImageBusy;
  const duDieuKien = typeFieldError === null && !filesBusy;
  // Every broadcast HOLDS a picked file until the save returns an id (the server requires
  // `content_item_id`); on a saved one with its file, the duration box also corrects the typed duration.
  const holdsAudio = gt.type === CONTENT_TYPE_BROADCAST;
  const durationEdited = attachedDuration !== null && heldDuration.trim() !== attachedDuration;
  const heldProblem = holdsAudio ? heldAudioProblem(heldFile, heldDuration, durationEdited) : null;
  // Said once the officer typed a duration or pressed Lưu — not the instant a file is picked.
  const shownHeldProblem = heldDuration.trim() !== "" || titleTouched ? heldProblem : null;
  const audioFailed = audioState.kind === "refused" || audioState.kind === "retry";
  const isBanner = gt.type === CONTENT_TYPE_BANNER;
  // Non-hidden categories only (prototype `ContentItemForm.tsx:253-255`), plus the item's OWN category when
  // it is hidden: dropping it would show `— Chưa xếp danh mục —` and the first Lưu would move the item.
  const formCategories = danhMuc.filter((d) => !d.hidden || d.id === giaTriDau.category_id);

  // Each state report; a `ready` one makes its id the form's cover. Functional update: the upload
  // resolves after renders the officer may have made meanwhile (typing the title), and those stay.
  // The first article id any file's reply names is the form's; a later one never replaces it.
  function keepArticleId(id: string): void {
    setArticleId((a) => a ?? id);
  }

  /** `Hoàn tất lại` of a carried upload: busy while it moves; attached once `ready`. */
  function onAudioState(s: AudioUploadState): void {
    setAudioState(s);
    setAudioBusy(audioInFlight(s));
    if (s.kind === "ready") setAttachedDuration(heldDuration.trim());
  }

  function onCoverState(s: CoverUploadState): void {
    setCover(s);
    if (s.kind === "ready") datGT((g) => ({ ...g, cover_image_file_id: s.id }));
  }

  /**
   * Each body-image report. `ready` adds its preview and fixes the article id; busy follows the state.
   * Functional updates: the reply lands after renders the officer may have made meanwhile.
   */
  function onBodyImageState(s: BodyImageState): void {
    setBodyImageBusy(bodyImageInFlight(s));
    if (s.kind !== "ready") return;
    const { fileId, contentItemId, previewUrl } = s.image;
    setArticleId((a) => a ?? contentItemId);
    if (previewUrl !== undefined && previewUrl !== "") {
      setPreviews((m) => new Map(m).set(fileId, previewUrl));
    }
  }

  // A cover's FIRST upload on a new article is in flight: its reply will name the reserved id, and a body
  // image sent now, with no id, would reserve a second draft. Held for the cover's WHOLE flight — the id
  // arrives only with the reply of the one request (ADR 0052 §Sửa đổi 09/10/2026).
  const coverReserving = articleId === undefined && coverInFlight(cover);

  const bodyImages: BodyImageSource = {
    blocked: coverReserving ? BODY_IMAGE_WAIT_COVER : null,
    upload: (file, onState) =>
      runBodyImageUpload(
        file,
        articleId,
        (s) => {
          onBodyImageState(s);
          onState(s);
        },
        keepArticleId,
      ),
    fromUrl: (url, onState) =>
      runBodyImageFromUrl(url, articleId, (s) => {
        onBodyImageState(s);
        onState(s);
      }),
  };

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (tieuDeGon === "" || heldProblem !== null) {
      setTitleTouched(true);
      return;
    }
    if (!duDieuKien) return;
    luu(
      gt,
      khoaChongTrung,
      holdsAudio && heldFile !== null ? { file: heldFile, duration: heldDuration } : undefined,
      holdsAudio ? savedAudioPatch(attachedDuration, heldFile, heldDuration) : {},
    );
  }

  // Prototype `ContentItemForm.tsx:370-375`: the word stays `Lưu` with a spinner while saving; it reads
  // `Đang tải tệp…` only while a file is moving.
  const moving = filesBusy || uploading;
  const submitLabel = moving ? UPLOADING_FILES_LABEL : NHAN_NUT_LUU;

  return (
    <form
      className="form-danh-muc m-0 border-0 bg-transparent p-0"
      onSubmit={guiNgay}
      noValidate
      aria-labelledby="tieu-de-form-noi-dung"
    >
      {/* HEADER: the title and, right under it, the dialog's one sentence (prototype `DialogDescription`).
          The X calls the same `huy` as `Huỷ`, and is off while saving for the same reason. */}
      <div className="flex items-start gap-3 px-5 pt-5 pb-3">
        <div className="min-w-0 flex-1">
          <h3 id="tieu-de-form-noi-dung" className="m-0 text-lg leading-snug font-semibold text-navy">
            {tieuDeForm}
          </h3>
          <p className="m-0 mt-1 text-[13px] text-ink-muted">{moTa}</p>
        </div>
        <IconButton type="button" label={CLOSE_LABEL} className="-mt-1 -mr-2 min-h-0" disabled={dangGui} onClick={huy}>
          <X aria-hidden="true" focusable="false" />
        </IconButton>
      </div>

      {/* ONE scrolling column, as the prototype's `max-h-[70vh] space-y-4 overflow-y-auto`. */}
      <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto px-5 pb-5">
        {/* Row 1: the type, then the category — or, for a banner, its running order. One column at 320px. */}
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="Loại nội dung"
            htmlFor="loai-noi-dung"
            kind="select"
            grow="auto"
            className="[&_select]:bg-canvas!"
          >
            <select
              id="loai-noi-dung"
              value={gt.type === "" ? LOAI_MAC_DINH : gt.type}
              disabled={editing}
              onChange={(e) => datGT({ ...gt, type: e.target.value })}
            >
              {MOI_LOAI.map((ma) => (
                <option key={ma} value={ma}>
                  {dialogTypeLabel(ma)}
                </option>
              ))}
            </select>
          </Field>

          {isBanner ? (
            // ADR 0067 §6: ascending — the SMALLER number runs first (the prototype's placeholder said the
            // opposite; the decision wins). Banner only: the server refuses it on any other type (422).
            <Field label="Thứ tự chạy" htmlFor="thu-tu-banner" icon={ListOrdered} grow="auto">
              <input
                id="thu-tu-banner"
                name="thu-tu-banner"
                type="number"
                inputMode="numeric"
                min={0}
                max={DISPLAY_ORDER_MAX}
                step={1}
                placeholder="Số nhỏ hiện trước"
                value={gt.display_order}
                onChange={(e) => datGT({ ...gt, display_order: e.target.value })}
              />
            </Field>
          ) : (
            <Field label="Danh mục" htmlFor="danh-muc-noi-dung" kind="select" grow="auto">
              <select
                id="danh-muc-noi-dung"
                value={gt.category_id}
                onChange={(e) => datGT({ ...gt, category_id: e.target.value })}
              >
                <option value="">{CHUA_XEP_DANH_MUC}</option>
                {dungCayDanhMuc(formCategories).map((m) => (
                  <option key={m.dm.id} value={m.dm.id}>
                    {categoryPath(m.dm, danhMuc)}
                  </option>
                ))}
              </select>
            </Field>
          )}
        </div>

        <Field label="Tiêu đề" htmlFor="tieu-de-noi-dung" required error={titleError} grow="auto">
          <input
            id="tieu-de-noi-dung"
            name="tieu-de-noi-dung"
            value={gt.title}
            maxLength={TIEU_DE_TOI_DA}
            autoComplete="off"
            aria-invalid={titleError !== undefined || undefined}
            onChange={(e) => datGT({ ...gt, title: e.target.value })}
          />
        </Field>

        {isBanner ? (
          // ADR 0067 §5. Only for `Banner`: the server refuses `link_to` on any other type (422), and a type
          // change away from banner clears it there — so this box is not sent then. The prototype's label and
          // placeholder alone (`ContentItemForm.tsx:278-284`); the suggestions stay (ADR 0067 B9).
          <div className="flex flex-col gap-1.5">
            <Field label="Bấm vào thì mở gì" htmlFor="lien-ket-banner" grow="auto">
              <input
                id="lien-ket-banner"
                name="lien-ket-banner"
                value={gt.link_to}
                maxLength={LINK_TO_MAX_CHARS}
                placeholder="Ví dụ: /tin-tuc — để trống thì ảnh chỉ để xem"
                autoComplete="off"
                list="lien-ket-banner-duong-app"
                onChange={(e) => datGT({ ...gt, link_to: e.target.value })}
              />
            </Field>
            {/* The in-app paths the citizen Mini App can open (`BANNER_APP_PATHS`); free https:// still typed. */}
            <datalist id="lien-ket-banner-duong-app">
              {BANNER_APP_PATHS.map((p) => (
                <option key={p.path} value={p.path} label={p.label} />
              ))}
            </datalist>
          </div>
        ) : (
          <>
            <Field label="Tóm tắt" htmlFor="tom-tat-noi-dung" grow="auto" className="[&_textarea]:py-2">
              <textarea
                id="tom-tat-noi-dung"
                name="tom-tat-noi-dung"
                rows={2}
                value={gt.summary}
                maxLength={TOM_TAT_TOI_DA}
                onChange={(e) => datGT({ ...gt, summary: e.target.value })}
              />
            </Field>

            <div className="flex flex-col gap-1.5">
              <span id="than-bai-noi-dung-nhan" className="text-[13px] leading-tight font-semibold text-navy">
                Nội dung
              </span>
              {/* The editor starts from the body as it was when the form opened (`giaTriDau`), never from
                  `gt.body`: feeding its own output back in would reset the cursor on every keystroke. */}
              <RichTextEditor
                id="than-bai-noi-dung"
                labelId="than-bai-noi-dung-nhan"
                initialHtml={giaTriDau.body}
                disabled={dangGui}
                onChange={(body) => datGT((g) => ({ ...g, body }))}
                previews={previews}
                images={bodyImages}
              />
            </div>
          </>
        )}

        {/* :131 of the spec, ADR 0047 §6. Rendered only for the type that can carry them — the server
            refuses an event field on any other type, and a box that is shown is a box somebody fills. */}
        {gt.type === CONTENT_TYPE_EVENT && (
          <>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Bắt đầu" htmlFor="bat-dau-su-kien" icon={CalendarClock} grow="auto">
                <input
                  id="bat-dau-su-kien"
                  name="bat-dau-su-kien"
                  type="datetime-local"
                  value={gt.event_starts_local}
                  onChange={(e) => datGT({ ...gt, event_starts_local: e.target.value })}
                />
              </Field>
              <Field label="Kết thúc" htmlFor="ket-thuc-su-kien" icon={CalendarClock} grow="auto">
                <input
                  id="ket-thuc-su-kien"
                  name="ket-thuc-su-kien"
                  type="datetime-local"
                  value={gt.event_ends_local}
                  onChange={(e) => datGT({ ...gt, event_ends_local: e.target.value })}
                />
              </Field>
            </div>
            <Field label="Địa điểm" htmlFor="dia-diem-su-kien" icon={MapPin} grow="auto">
              <input
                id="dia-diem-su-kien"
                name="dia-diem-su-kien"
                value={gt.event_place}
                maxLength={EVENT_PLACE_MAX_CHARS}
                autoComplete="off"
                onChange={(e) => datGT({ ...gt, event_place: e.target.value })}
              />
            </Field>
          </>
        )}

        {gt.type === CONTENT_TYPE_VIDEO && (
          <Field label="Liên kết video" htmlFor="lien-ket-video" icon={Video} grow="auto">
            <input
              id="lien-ket-video"
              name="lien-ket-video"
              type="url"
              value={gt.video_url}
              maxLength={URL_TOI_DA}
              placeholder="https://"
              autoComplete="off"
              onChange={(e) => datGT({ ...gt, video_url: e.target.value })}
            />
          </Field>
        )}

        {/* ADR 0067 §4 — the prototype's two boxes (`ContentItemForm.tsx:312-336`), the same on create and
            on edit. The picked file is HELD; the save uploads it with the item's id (prototype `:150-180`). */}
        {holdsAudio && (
          <>
            <HeldAudioField
              file={heldFile}
              hasExisting={attachedDuration !== null}
              duration={heldDuration}
              problem={shownHeldProblem}
              disabled={dangGui || audioBusy}
              onFile={setHeldFile}
              onDuration={setHeldDuration}
            />
            {audioState.kind !== "idle" && (
              <p
                className={audioFailed ? "thong-bao-loi m-0" : "m-0 text-[12px] text-ink-muted"}
                role={audioFailed ? "alert" : "status"}
              >
                {audioStateText(audioState)}
              </p>
            )}
            {audioState.kind === "retry" && (
              <div className="cum-nut">
                <button
                  type="button"
                  className="nut-phu"
                  disabled={dangGui || audioBusy}
                  onClick={() =>
                    void runAudioUpload(audioState.again.file, audioState.again.contentItemId, heldDuration, onAudioState)
                  }
                >
                  {AUDIO_RETRY_BUTTON}
                </button>
              </div>
            )}
          </>
        )}

        <CoverImageField
          banner={isBanner}
          fileId={gt.cover_image_file_id}
          state={cover}
          // While the FIRST body image of a new article is still reserving its id, a cover would
          // reserve another one.
          disabled={dangGui || (bodyImageBusy && articleId === undefined)}
          // The article id a body image reserved, when there is one — one draft for every file. A
          // cover uploaded first reserves it instead, and the body images then use the cover's.
          onPick={(file) => void runCoverUpload(file, articleId, onCoverState, keepArticleId)}
          onRetry={(file) => void runCoverUpload(file, articleId, onCoverState, keepArticleId)}
        />

        {/* The prototype's plain checkbox and words — no box around it. */}
        <label htmlFor="dang-len-mini-app" className="flex cursor-pointer items-center gap-2.5 text-[12.5px] text-navy">
          <input
            id="dang-len-mini-app"
            name="dang-len-mini-app"
            type="checkbox"
            className="m-0 size-4 shrink-0 accent-brand-600"
            checked={gt.publish}
            onChange={(e) => datGT({ ...gt, publish: e.target.checked })}
          />
          {NHAN_O_DANG}
        </label>

        {typeFieldError !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {typeFieldError}
          </p>
        )}

        {loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
          </p>
        )}

        {/* `Huỷ` · `Lưu`, right-aligned, at the end of the column (prototype `:362-376`). `type` FIRST on the
            native buttons — the tests read `<button type="submit"…`. */}
        <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
          <button
            type="button"
            className={cn("nut-phu w-auto min-h-0", buttonVariants({ variant: "secondary", size: "md" }))}
            disabled={dangGui}
            onClick={huy}
          >
            {NHAN_NUT_HUY}
          </button>
          {/* Busy: a spinner and the word of what it waits for, in a button that keeps its width. */}
          <button
            type="submit"
            className={cn("nut-chinh w-auto min-h-0 min-w-[7.5rem]", buttonVariants({ variant: "primary", size: "md" }))}
            disabled={dangGui || !duDieuKien}
            aria-busy={dangGui || moving || undefined}
          >
            {(dangGui || moving) && <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />}
            {submitLabel}
          </button>
        </div>
      </div>
    </form>
  );
}

/**
 * §7 `Ảnh đại diện` — `Ảnh banner` for a banner — drawn as the prototype's `FilePicker` (`ContentItemForm.tsx:
 * 392-444`): the label, then ONE dashed slot that IS the picker (upload icon · `Chọn tệp từ máy` or `Đã có
 * tệp — chọn tệp mới để thay` · the hint · the chosen file's size). Presentational: the form owns the id and
 * the upload state (`cover-image.ts` runs the one-request upload); progress, refusal and retry stay under the
 * slot. NO PREVIEW AND NO `Gỡ ảnh` — the prototype's slot has neither: an existing cover is said by the
 * slot's own words, and replaced by choosing another file.
 *
 * KEYBOARD: the file input is visually hidden and stays in the tab order, FIRST; the slot right after it is
 * its `<label>` and shows the focus ring (`peer-focus-visible`). Enter/Space on the input open the picker.
 */
export function CoverImageField({
  banner = false,
  fileId,
  state,
  disabled,
  onPick,
  onRetry,
}: {
  /** A banner's cover is the banner itself: `Ảnh banner`, and a hint about its shape. */
  banner?: boolean;
  /** The form's current `cover_image_file_id`, `""` = none. */
  fileId: string;
  state: CoverUploadState;
  disabled: boolean;
  onPick: (file: File) => void;
  /** `Gửi lại` after a refusal of the moment: the same file, sent again. */
  onRetry: (file: File) => void;
}) {
  // The file picked in this form — its name and size are shown in the slot, as in the prototype. Never
  // sent anywhere from here, never logged.
  const [chosen, setChosen] = useState<{ name: string; size: number } | null>(null);
  const busy = coverInFlight(state);
  const stateText = coverStateText(state);
  const refused = state.kind === "refused" || state.kind === "retry";
  const off = disabled || busy;

  return (
    <div className="flex min-w-0 flex-col gap-1.5 [&_p]:m-0" role="group" aria-labelledby="anh-noi-dung-nhan">
      <span id="anh-noi-dung-nhan" className="text-[13px] leading-tight font-semibold text-navy">
        {banner ? COVER_BANNER_LABEL : COVER_LABEL}
      </span>

      {/* The input FIRST, visually hidden but focusable; the slot right after it is its label. */}
      <input
        id="anh-noi-dung"
        name="anh-noi-dung"
        type="file"
        accept={COVER_ACCEPT}
        className="peer an-thi-giac"
        aria-describedby="anh-noi-dung-goi-y"
        disabled={off}
        onChange={(e) => {
          const f = e.target.files?.[0];
          e.target.value = ""; // the same file can be chosen again after a refusal
          if (f !== undefined) {
            setChosen({ name: f.name, size: f.size });
            onPick(f);
          }
        }}
      />
      <label
        htmlFor="anh-noi-dung"
        className={cn(
          "flex min-w-0 items-center gap-2.5 rounded-md border border-dashed border-line px-3 py-2.5 transition-colors",
          "peer-focus-visible:ring-3 peer-focus-visible:ring-ring/50",
          off ? "cursor-not-allowed opacity-60" : "cursor-pointer hover:border-brand/50 hover:bg-brand/4",
        )}
      >
        <Upload aria-hidden="true" focusable="false" className="size-4 shrink-0 text-ink-muted" />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[12.5px] font-medium text-navy">
            {chosen !== null ? chosen.name : fileId !== "" ? COVER_REPLACE_BUTTON : COVER_PICK_BUTTON}
          </span>
          <span className="block text-[11px] text-ink-muted" id="anh-noi-dung-goi-y">
            {banner ? COVER_BANNER_HINT : COVER_HINT}
          </span>
        </span>
        {chosen !== null && (
          <span className="shrink-0 text-[11.5px] font-semibold text-leaf">{coverSizeLabel(chosen.size)}</span>
        )}
      </label>

      {stateText !== "" && (
        <p role={refused ? "alert" : "status"} className={refused ? "thong-bao-loi" : "text-[12px] text-ink-muted"}>
          {stateText}
        </p>
      )}

      {state.kind === "retry" && (
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<RefreshCw aria-hidden="true" focusable="false" />}
            disabled={disabled}
            onClick={() => onRetry(state.file)}
          >
            {COVER_RETRY_BUTTON}
          </Button>
        </div>
      )}
    </div>
  );
}

/**
 * The add row of `⊞ Danh mục tin` — the prototype's `flex items-end gap-2` row (`CategoryManagerDialog.tsx:127-166`):
 * `Thêm danh mục` · `Thuộc mục (nếu có)` · `Thêm`, Enter submits. Rename, `Tắt`/`Bật` and delete live in
 * `CategoryAdmin` (`category-admin.tsx`, ADR 0067 §3), drawn below this row.
 *
 * ONE BOX MORE THAN THE PROTOTYPE, ON PURPOSE: `Slug`. The server requires it, and inventing it from the name
 * would be the screen issuing a code (an issued slug is never reissued — rule 7). `Thuộc mục` is the tree's
 * select, not free text: the server takes a `parent_id`.
 */
export function FormDanhMuc({
  danhMuc,
  dangGui,
  loi,
  luu,
}: {
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  loi: string | null;
  luu: (ten: string, slug: string, chaID: string, khoa: string) => void;
}) {
  const [ten, datTen] = useState("");
  const [slug, datSlug] = useState("");
  const [chaID, datChaID] = useState("");
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tenGon = ten.trim();
  const slugGon = slug.trim();
  const duDieuKien = tenGon !== "" && slugGon !== "";

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien || dangGui) return;
    luu(tenGon, slugGon, chaID, khoaChongTrung);
  }

  const cay = dungCayDanhMuc(danhMuc);

  return (
    <form className="m-0 flex flex-col gap-1.5" onSubmit={guiNgay} aria-label={CATEGORY_ADD_LABEL}>
      <div className="flex flex-wrap items-end gap-2">
        <Field label={CATEGORY_ADD_LABEL} htmlFor="ten-danh-muc" grow="auto" className="min-w-40 flex-1">
          <input
            id="ten-danh-muc"
            name="ten-danh-muc"
            value={ten}
            maxLength={TEN_DANH_MUC_TOI_DA}
            autoComplete="off"
            placeholder={CATEGORY_ADD_PLACEHOLDER}
            onChange={(e) => datTen(e.target.value)}
          />
        </Field>

        <Field label={CATEGORY_PARENT_LABEL} htmlFor="cha-danh-muc" kind="select" grow="auto" className="w-48">
          <select id="cha-danh-muc" value={chaID} onChange={(e) => datChaID(e.target.value)}>
            <option value="">{CATEGORY_NO_PARENT_OPTION}</option>
            {cay.map((m) => (
              <option key={m.dm.id} value={m.dm.id}>
                {nhanMucDanhMuc(m)}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Slug" htmlFor="slug-danh-muc" required grow="auto" className="w-40">
          <input
            id="slug-danh-muc"
            name="slug-danh-muc"
            value={slug}
            maxLength={SLUG_DANH_MUC_TOI_DA}
            autoComplete="off"
            placeholder="nong-nghiep"
            aria-describedby="slug-danh-muc-goi-y"
            onChange={(e) => datSlug(e.target.value)}
          />
        </Field>

        <Button
          type="submit"
          variant="primary"
          disabled={dangGui || !duDieuKien}
          aria-busy={dangGui || undefined}
          icon={
            dangGui ? (
              <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />
            ) : (
              <Plus aria-hidden="true" focusable="false" />
            )
          }
        >
          {CATEGORY_ADD_BUTTON}
        </Button>
      </div>

      {/* MÁY CHỦ TỪ CHỐI CHỨ KHÔNG TỰ HẠ CHỮ HOA: `Chuyen-Doi-So` bị trả 400 thay vì lặng lẽ thành
          `chuyen-doi-so`, vì mã lưu xuống phải đúng mã người ta thấy lúc gõ. Nói trước điều đó thay vì để
          họ gõ xong mới biết. */}
      <p className="m-0 text-[11px] text-ink-muted" id="slug-danh-muc-goi-y">
        Slug chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ <code>chuyen-doi-so</code>. Slug đã cấp thì
        KHÔNG cấp lại, kể cả khi danh mục mang slug đó đã bị xoá.
      </p>

      {loi !== null && (
        <p className="m-0 text-[12px] font-medium text-danger" role="alert">
          {loi}
        </p>
      )}
    </form>
  );
}

/**
 * Status pill by the server's CODE (spec §7: icon + word, never colour alone). The word is still
 * `nhanTrangThai`'s — the three statuses stay (owner D1, 09/10/2026); only the look follows the code:
 * `Chờ duyệt` amber (waiting), `Ẩn` grey.
 */
function ContentStatusBadge({ status }: { status: string }) {
  const look =
    status === "dang-hien"
      ? { tone: "success" as const, icon: Eye }
      : status === STATUS_PENDING_REVIEW
        ? { tone: "warning" as const, icon: Clock }
        : { tone: "neutral" as const, icon: EyeOff };
  return (
    <Badge tone={look.tone} icon={look.icon}>
      {nhanTrangThai(status)}
    </Badge>
  );
}

/** The edit dialog's placeholder while the full text loads. Decorative: the status sentence is what is heard. */
function FormSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <span className="block h-10 rounded-control bg-line motion-safe:animate-pulse" />
        <span className="block h-10 rounded-control bg-line motion-safe:animate-pulse" />
      </div>
      <span className="block h-10 rounded-control bg-line motion-safe:animate-pulse" />
      <span className="block h-48 rounded-control bg-line motion-safe:animate-pulse" />
    </div>
  );
}
