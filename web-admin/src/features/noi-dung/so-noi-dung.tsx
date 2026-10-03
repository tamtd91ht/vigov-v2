"use client";

import {
  ArrowRight,
  AudioLines,
  BookUser,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  CircleDot,
  Clock,
  CloudOff,
  Eye,
  EyeOff,
  FileText,
  FolderTree,
  Image as ImageIcon,
  ImagePlus,
  LayoutGrid,
  Link as LinkIcon,
  ListOrdered,
  LoaderCircle,
  MapPin,
  Newspaper,
  Pencil,
  Pilcrow,
  Plus,
  RefreshCw,
  Send,
  Shapes,
  Smartphone,
  Trash2,
  Video,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
import { cn } from "@/lib/cn";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { tabKeTheoPhim } from "@/features/cau-hinh/thanh-tab-cau-hinh";
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
  comms_coverImageOut,
  comms_danhMucRa,
  comms_danhSachDanhMucRa,
  comms_noiDungRa,
  page_Result_comms_noiDungRa,
} from "@/lib/api/schema.gen";

import {
  ACTIONS_COLUMN_LABEL,
  BANNER_APP_PATHS,
  BANNER_ARTICLE_PATH_HINT,
  BANNER_COVER_NOTICE,
  bannerLinkTapWarning,
  canEditContent,
  CANH_BAO_HTML_THO,
  CHUA_XEP_DANH_MUC,
  CLOSE_LABEL,
  CONTENT_DELETE_GONE,
  CONTENT_DELETE_TITLE,
  CONTENT_DELETED,
  contentDeleteAriaLabel,
  coThayDoi,
  DANG_TAI_SO,
  DANG_TAI_TOAN_VAN,
  DANH_MUC_RONG,
  DAU_GACH,
  DISPLAY_ORDER_HINT,
  DISPLAY_ORDER_MAX,
  dungCayDanhMuc,
  EDIT_FORM_TITLE,
  EVENT_PLACE_MAX_CHARS,
  EVENT_TIME_HINT,
  FORM_TRONG,
  giaTriTuHang,
  KHONG_CO_GI_DOI,
  LINK_TO_HINT,
  LINK_TO_MAX_CHARS,
  LOAI_MAC_DINH,
  MO_TA_FORM_THEM,
  MO_TA_MAN,
  MOI_DANH_MUC,
  MOI_LOAI,
  MOI_LOAI_NHAN,
  NHAN_DA_SUA_TAY,
  NHAN_NUT_DANH_MUC,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_SUA,
  NHAN_NUT_THEM,
  NHAN_O_DANG,
  nhanLoai,
  nhanMoc,
  nhanMucDanhMuc,
  nhanNgayDang,
  nhanNguon,
  nhanTepDinhKem,
  nhanTrangThai,
  pageAfterDelete,
  PENDING_REVIEW_HINT,
  PHAN_CHUA_DUNG,
  portalCategoryLabel,
  publishedAtLabel,
  publishedStaffText,
  SLUG_DANH_MUC_TOI_DA,
  SO_RONG,
  SOURCE_PORTAL_SYNC,
  STATUS_FILTER_OPTIONS,
  STATUS_PENDING_REVIEW,
  SUMMARY_SAPO_HINT,
  TEN_DANH_MUC_TOI_DA,
  TIEU_DE_FORM_THEM,
  TIEU_DE_MAN,
  TIEU_DE_THE_DANH_BA,
  TIEU_DE_TOI_DA,
  THAN_BAI_RONG,
  THU_TU_DANH_MUC_TOI_DA,
  TIM_PLACEHOLDER,
  TOM_TAT_TOI_DA,
  tenDanhMuc,
  thanSua,
  thanThem,
  trichTomTat,
  URL_TOI_DA,
  validateTypeFields,
  VIDEO_URL_HINT,
  VIEW_COUNT_LABEL,
  viewCountText,
  type GiaTriFormNoiDung,
} from "./nhan-noi-dung";
import {
  COVER_ACCEPT,
  COVER_HINT,
  COVER_NONE,
  COVER_PICK_BUTTON,
  COVER_PREVIEW_ALT,
  COVER_PREVIEW_MISSING,
  COVER_REMOVE_BUTTON,
  COVER_REPLACE_BUTTON,
  COVER_RETRY_BUTTON,
  COVER_WAIT_NOTE,
  COVER_WILL_DETACH,
  coverInFlight,
  coverPreviewSrc,
  coverStateText,
  retryCoverCompletion,
  runCoverUpload,
  savedCoverText,
  type CoverUploadState,
} from "./cover-image";
import { AUDIO_SAVE_FIRST, AUDIO_WAIT_NOTE, type AudioUploadState } from "./broadcast-audio";
import {
  BODY_IMAGE_WAIT_COVER,
  previewsFromItem,
  retryBodyImageCompletion,
  runBodyImageFromUrl,
  runBodyImageUpload,
  type BodyImageState,
} from "./body-image";
import type { BodyImageSource } from "./body-image-panel";
import { BroadcastAudioField } from "./broadcast-audio-field";
import { CategoryAdmin } from "./category-admin";
import { ContentDeleteDialog } from "./content-delete-dialog";
import { OverlayDialog } from "./overlay-dialog";
import { PortalSyncCard } from "./portal-sync-card";
import { RichTextEditor } from "./rich-text-editor";

/**
 * Màn "Nội dung Mini App" — `docs/ui-ux/11-noi-dung-mini-app.md` §2 (bố cục), §3 (thẻ đồng bộ
 * Cổng, `portal-sync-card.tsx`), §5 (sáu tab), §6 (bảng), §7 (biểu mẫu).
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
 * minus the missing reason. TABLE ROW ONLY: §6 draws the actions in the table, §7's form has none. The
 * reversible way off the Mini App is still unticking `Đăng lên Mini App` in the edit form. (A CATEGORY
 * has its own soft delete — `category-admin.tsx`, ADR 0067 §3.)
 *
 * WRITE CONTROLS FOLLOW `content.update` (`canEditContent`, 02/10/2026, as the prototype does): without
 * it the screen is read-only — no `+ Thêm nội dung`, no `✎`, no `🗑`, no `⊞ Danh mục tin`, no run/config on the
 * portal card. That is CONVENIENCE: `service-comms` still checks `content.read` / `content.update` on
 * EVERY call, and a 403 sentence still reaches the screen verbatim (rule 5, forbidden #1).
 *
 * TIÊU ĐỀ, TÓM TẮT VÀ THÂN BÀI LÀ TIN BÀI CỦA XÃ, thường nhắc tên và hoàn cảnh của một công dân cụ
 * thể: không dòng nào ở đây ghi chúng vào log, vào tên tệp hay vào một URL (luật 3, cấm #1 và #4).
 */

/** Bao nhiêu hàng một trang của bảng §6. */
const SO_HANG_MOI_TRANG = 20;

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
  readonly loai: string;
  readonly danhMucID: string;
  readonly tim: string;
  /** §6 `Trạng thái` — `""` = every status (no `status` sent). */
  readonly status: string;
};

const LOC_TRONG: BoLocMan = { loai: "", danhMucID: "", tim: "", status: "" };

/** The submit button's word while the save is in flight (spec §8b "Nút đang xử lý"). */
const SAVING_LABEL = "Đang lưu…";

/**
 * The editor dialog draws its own sticky header and footer, so the dialog's scroll box loses its
 * padding (they would otherwise stick 1rem short of its edges). Wider than the default overlay: two
 * selects side by side need the room.
 */
const EDITOR_DIALOG_CLASS = "w-[min(52rem,calc(100vw-1rem))] p-0";

/**
 * The table's own scroller, both ways, so the header row can stay put (`sticky` inside an `overflow`
 * box sticks to THAT box) — same shape as the task register (`so-nhiem-vu.tsx`). 48px rows (spec v2).
 */
const TABLE_SCROLL_CLASS = cn(
  "bang-cuon m-0 max-h-[70vh] overflow-auto rounded-none border-0 shadow-none",
  "[&_thead_th]:sticky [&_thead_th]:top-0 [&_thead_th]:z-[1] [&_thead_th]:shadow-[inset_0_-1px_0_var(--line)]",
  "[&_tbody_td]:h-12",
);

/**
 * A label without the symbol it starts with (`⊞ Danh mục tin` → `Danh mục tin`, `🔗 Có ảnh` → `Có ảnh`):
 * where a lucide icon now draws it (ADR 0068 §2, no emoji as icon). The words themselves stay the
 * labels file's, unchanged. Only for labels known to start with a symbol — `—` alone would be emptied.
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

  // The row whose `🗑` was pressed — the whole row, since the dialog shows only its title (no body
  // needed). `deleteNotice` is the line above the table after the dialog closed on a 204 or a 404.
  const [deleting, setDeleting] = useState<comms_noiDungRa | null>(null);
  const [deleteNotice, setDeleteNotice] = useState<string | null>(null);

  const khoaSo = `${loc.loai}|${loc.danhMucID}|${loc.tim}|${loc.status}|${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    const yc: BoLocNoiDung = {
      loai: loc.loai,
      danhMucID: loc.danhMucID,
      tim: loc.tim,
      status: loc.status,
      limit: SO_HANG_MOI_TRANG,
      cursor: nganXep.hienTai,
    };
    laySoNoiDung(yc).then((kq) => {
      if (!bo) datDaTai({ khoa: khoaSo, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc.loai, loc.danhMucID, loc.tim, loc.status, nganXep.hienTai, khoaSo]);

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

  function dongMoiBieuMau(): void {
    datDangMoThem(false);
    datDangMoDanhMuc(false);
    datDangSuaID(null);
    datLoiForm(null);
    setDeleting(null);
    setDeleteNotice(null);
  }

  /**
   * After the delete dialog closed on a 204 or a 404: say what happened and read the SAME page again
   * (one back if that row was its last — `pageAfterDelete`), not the first page as after a save: the
   * officer clearing several rows on page 3 stays on page 3.
   */
  function afterDelete(notice: string): void {
    const rows = so.pha === "xong" ? so.duLieu.items.length : 0;
    setDeleting(null);
    setDeleteNotice(notice);
    datNganXep((stack) => pageAfterDelete(stack, rows));
    datLanTai((n) => n + 1);
  }

  /** Esc on an open overlay: refused while a save is in flight, like the forms' own `Huỷ`. */
  function dismissOverlay(): void {
    if (!dangGui) dongMoiBieuMau();
  }

  function themBai(gt: GiaTriFormNoiDung, khoa: string): void {
    datDangGui(true);
    themNoiDung(thanThem(gt), khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. Câu 400 của nó nói đúng trường sai và đúng quy tắc bị chạm —
        // viết lại ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangMoThem(false);
      datLanGhiXong((n) => n + 1);
      taiLaiSo();
    });
  }

  function suaBai(id: string, dau: GiaTriFormNoiDung, moi: GiaTriFormNoiDung): void {
    const than = thanSua(dau, moi);
    if (!coThayDoi(than)) {
      // KHÔNG GỌI `PATCH` RỖNG. Một lần Lưu không đổi gì vẫn là một lần ghi ở máy chủ, và §10.4
      // gắn cờ "đã sửa tay" lên bài — cờ ấy đưa bài ra khỏi lượt đồng bộ VĨNH VIỄN.
      datLoiForm(KHONG_CO_GI_DOI);
      return;
    }
    datDangGui(true);
    suaNoiDung(id, than).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangSuaID(null);
      datLanGhiXong((n) => n + 1);
      taiLaiSo();
    });
  }

  function themDanhMuc(ten: string, slug: string, chaID: string, thuTu: number, khoa: string): void {
    datDangGui(true);
    themDanhMucNoiDung({ name: ten, slug, parent_id: chaID, order: thuTu }, khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiForm(kq.thongBao);
        return;
      }
      datLoiForm(null);
      datDangMoDanhMuc(false);
      datLanGhiXong((n) => n + 1);
      datLanTaiDanhMuc((n) => n + 1);
    });
  }

  return (
    <div className="man-noi-dung mt-0">
      {/* §2: `[+ Thêm nội dung]` sits in the page header, above the two cards. The header is drawn here,
          not by the page: the button's state (the open form, the `content.update` gate) lives in this
          component — same split as `/nhiem-vu`. Title and sentence stay `nhan-noi-dung.ts`'s. */}
      <PageHeader
        icon={Smartphone}
        title={TIEU_DE_MAN}
        subtitle={MO_TA_MAN}
        actions={
          <HeaderActions
            canEdit={canEdit}
            addOpen={dangMoThem}
            openAdd={() => {
              dongMoiBieuMau();
              datDangMoThem(true);
            }}
          />
        }
      />

      <KhoiChuaDung />

      {/* §2: Card 1 and Card 2 side by side on a wide screen, stacked at 320px. */}
      <div className="content-top-cards">
        <PortalSyncCard
          canEdit={canEdit}
          // `Xem tin chờ duyệt` on the card is the same `Trạng thái` filter, set from there — one filter,
          // two ways in. Every other filter is kept: the queue narrowed by type is still the queue.
          showPendingReview={() => {
            datNganXep(TRANG_DAU);
            datLoc({ ...loc, status: STATUS_PENDING_REVIEW });
          }}
        />
        <TheDanhBaChinhQuyen publishedCount={publishedCount} />
      </div>

      {canEdit && dangMoThem && (
        <OverlayDialog titleId="tieu-de-form-noi-dung" onDismiss={dismissOverlay} className={EDITOR_DIALOG_CLASS}>
          <FormNoiDung
            // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
            key={`them|${lanGhiXong}`}
            tieuDeForm={TIEU_DE_FORM_THEM}
            moTa={MO_TA_FORM_THEM}
            giaTriDau={FORM_TRONG}
            danhMuc={dsDanhMuc}
            dangGui={dangGui}
            loi={loiForm}
            huy={dongMoiBieuMau}
            luu={(gt, khoa) => themBai(gt, khoa)}
          />
        </OverlayDialog>
      )}

      {canEdit && dangMoDanhMuc && (
        <OverlayDialog titleId="tieu-de-hop-danh-muc" onDismiss={dismissOverlay}>
          <div className="dau-khoi-chi-tiet mb-2 flex-nowrap border-b border-line pb-3">
            <h3 id="tieu-de-hop-danh-muc" className="flex items-center gap-2 text-base">
              <LayoutGrid aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
              {withoutLeadingGlyph(NHAN_NUT_DANH_MUC)}
            </h3>
            <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" disabled={dangGui} onClick={dongMoiBieuMau}>
              <X aria-hidden="true" focusable="false" />
            </IconButton>
          </div>
          <CategoryAdmin
            categories={dsDanhMuc}
            update={updateContentCategory}
            remove={deleteContentCategory}
            changed={() => datLanTaiDanhMuc((n) => n + 1)}
          />
          <FormDanhMuc
            key={`danh-muc|${lanGhiXong}`}
            danhMuc={dsDanhMuc}
            dangGui={dangGui}
            loi={loiForm}
            huy={dongMoiBieuMau}
            luu={themDanhMuc}
          />
        </OverlayDialog>
      )}

      {canEdit && chiTiet !== null && dangSuaID !== null && (
        <OverlayDialog
          titleId="tieu-de-form-noi-dung"
          onDismiss={dismissOverlay}
          className={chiTiet.pha === "xong" ? EDITOR_DIALOG_CLASS : undefined}
        >
          {chiTiet.pha === "xong" ? (
            <FormNoiDung
              key={`sua|${dangSuaID}|${lanGhiXong}`}
              tieuDeForm={`Sửa nội dung: ${chiTiet.duLieu.title}`}
              moTa={MO_TA_FORM_THEM}
              giaTriDau={giaTriTuHang(chiTiet.duLieu)}
              hang={chiTiet.duLieu}
              danhMuc={dsDanhMuc}
              dangGui={dangGui}
              loi={loiForm}
              huy={dongMoiBieuMau}
              luu={(gt) => suaBai(dangSuaID, giaTriTuHang(chiTiet.duLieu), gt)}
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
          deleted={() => afterDelete(CONTENT_DELETED)}
          gone={() => afterDelete(CONTENT_DELETE_GONE)}
          close={() => setDeleting(null)}
        />
      )}

      {/* Spec §8.5: ONE list card — tabs, the filter row, the table and its pagination. */}
      <Card as="section" className="mt-4" aria-labelledby="tieu-de-so-noi-dung">
      <CardHeader className="border-b-0 pb-0">
        <CardTitle id="tieu-de-so-noi-dung">Sổ nội dung Mini App</CardTitle>
      </CardHeader>
      <ThanhTabLoai
        loai={loc.loai}
        datLoai={(l) => {
          datNganXep(TRANG_DAU);
          datLoc({ ...loc, loai: l });
        }}
      />

      <div
        role="tabpanel"
        id={CONTENT_TABPANEL_ID}
        aria-labelledby={contentTabId(loc.loai)}
        // The panel itself takes focus after the tablist (WAI-ARIA tabs): Tab from the selected tab
        // lands here, not on the first filter box three controls further down.
        tabIndex={0}
      >
        <HangLocNoiDung
          danhMucID={loc.danhMucID}
          datDanhMucID={(id) => {
            datNganXep(TRANG_DAU);
            datLoc({ ...loc, danhMucID: id });
          }}
          tim={tim}
          datTim={datTim}
          timNgay={() => {
            datNganXep(TRANG_DAU);
            datLoc({ ...loc, tim: tim.trim() });
          }}
          danhMuc={dsDanhMuc}
          status={loc.status}
          setStatus={(st) => {
            datNganXep(TRANG_DAU);
            datLoc({ ...loc, status: st });
          }}
          // §2: `[⊞ Danh mục tin]` closes the filter row. `content.update` only — it opens the tree's
          // edit/hide/delete and the add form, nothing to read in it that the filter does not show.
          extra={
            canEdit ? (
              <Button
                type="button"
                variant="secondary"
                aria-haspopup="dialog"
                icon={<LayoutGrid aria-hidden="true" focusable="false" />}
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
          <p className="thong-bao-loi mx-4 my-3" role="alert">
            {danhMuc.thongBao}
          </p>
        )}

        {deleteNotice !== null && (
          <Notice className="mx-4 my-3" icon={CircleCheck} role="status">
            {deleteNotice}
          </Notice>
        )}
        {/* LOADING (spec §8b): the sentence stays the live announcement; the eye gets rows shaped like
            the table, so the card does not jump when the answer lands. */}
        {so.pha === "dangTai" && (
          <>
            <p role="status" className="an-thi-giac">
              {DANG_TAI_SO}
            </p>
            <RowsSkeleton />
          </>
        )}
        {/* LOAD ERROR: the server's sentence VERBATIM stays the alert; `Tải lại` asks the same read again
            through the screen's existing re-read key (`lanTai`) — no new call, no new route. */}
        {so.pha === "loi" && (
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
        )}

        {so.pha === "xong" && (
        <>
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
            canEdit={canEdit}
          />
          <nav
            className="dieu-huong-trang m-0 border-t border-line px-4 py-3"
            aria-label="Phân trang sổ nội dung Mini App"
          >
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<ChevronLeft aria-hidden="true" focusable="false" />}
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="flex-row-reverse"
              icon={<ChevronRight aria-hidden="true" focusable="false" />}
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
            </Button>
          </nav>
        </>
        )}
      </div>
      </Card>
    </div>
  );
}

/**
 * §2's page-header action: `[+ Thêm nội dung]`. Drawn only with `content.update` (`canEditContent`):
 * before the session is read it is absent too, so it can appear but never flash and vanish.
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
    <div className="cum-nut">
      <Button type="button" variant="primary" aria-haspopup="dialog" aria-expanded={addOpen} onClick={openAdd}>
        {NHAN_NUT_THEM}
      </Button>
    </div>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý
        do
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
 * Thẻ §4 — Danh bạ chính quyền: `Đang hiện {n} cán bộ cho bà con` and the link to `/danh-ba`.
 *
 * THE NUMBER IS SHOWN ONLY AFTER THE SERVER ANSWERED (`publishedStaffText`). While loading, or when the
 * read failed, the line carries no number at all — a `0` there would tell the commune that residents
 * see nobody. Presentational: `SoNoiDung` reads the count (`countPublishedStaff`).
 */
export function TheDanhBaChinhQuyen({ publishedCount }: { publishedCount: KetQua<number> | null }) {
  const text = publishedStaffText(publishedCount);
  return (
    <section className="khoi-chi-tiet flex flex-col gap-2" aria-labelledby="government-directory-title">
      <div className="dau-khoi-chi-tiet">
        <h3 id="government-directory-title" className="flex items-center gap-2">
          <BookUser aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
          {TIEU_DE_THE_DANH_BA}
        </h3>
        <Link
          className={cn("nut-phu flex-row-reverse", buttonVariants({ variant: "secondary", size: "sm" }))}
          href="/danh-ba"
        >
          <ArrowRight aria-hidden="true" focusable="false" />
          Mở danh bạ cán bộ
        </Link>
      </div>
      <p className="m-0 text-sm text-ink-700">{text.line}</p>
      {text.note !== null && <p className="ghi-chu m-0">{text.note}</p>}
    </section>
  );
}

/** The one panel the content-type tabs control: the filter row, the table and its pagination. */
export const CONTENT_TABPANEL_ID = "content-type-panel";

/** The id of one tab; `""` is `Tất cả`. */
export function contentTabId(type: string): string {
  return `content-type-tab-${type === "" ? "all" : type}`;
}

/**
 * Sáu tab loại nội dung §5, cộng một tab `Tất cả`.
 *
 * TAB `Tất cả` KHÔNG CÓ TRONG §5, và nó được thêm vào có chủ ý chứ không phải do nhầm: sổ phải mở
 * được ở trạng thái không lọc, vì `type` VẮNG là hình dạng duy nhất nói "mọi loại" — gửi `type=`
 * rỗng thì máy chủ vẫn coi là không lọc, nhưng màn hình sẽ không có nút nào để quay về đó.
 *
 * REAL TABS, THE CẤU HÌNH SCREEN'S PATTERN (`features/cau-hinh/khung-tab-cau-hinh.tsx`): `tablist` /
 * `tab` / `tabpanel`, one tab in the Tab order (roving `tabIndex`), ← → Home End move AND select
 * (`tabKeTheoPhim`, reused), and the same `.thanh-tab-cau-hinh` look (44px targets, wraps at 320px).
 * ONE panel for seven tabs: every tab shows the same table under a different `type` filter, so all
 * seven `aria-controls` name it.
 */
export function ThanhTabLoai({
  loai,
  datLoai,
}: {
  loai: string;
  datLoai: (l: string) => void;
}) {
  const types: readonly string[] = ["", ...MOI_LOAI];
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
    <div className="thanh-tab-cau-hinh mt-2 px-2" role="tablist" aria-label="Loại nội dung">
      {types.map((type, i) => (
        <button
          key={type === "" ? "all" : type}
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
        >
          {type === "" ? MOI_LOAI_NHAN : nhanLoai(type)}
        </button>
      ))}
    </div>
  );
}

/**
 * Hàng lọc §6 — ô tìm theo tiêu đề, ô chọn danh mục và ô chọn trạng thái.
 *
 * THE STATUS FILTER IS THE SERVER'S (`status`, 02/10/2026 C1), never a filter of the page in hand: a
 * client-side filter would hide every `Chờ duyệt` item on the other pages.
 */
export function HangLocNoiDung({
  danhMucID,
  datDanhMucID,
  tim,
  datTim,
  timNgay,
  danhMuc,
  status,
  setStatus,
  extra,
}: {
  danhMucID: string;
  datDanhMucID: (id: string) => void;
  tim: string;
  datTim: (s: string) => void;
  timNgay: () => void;
  danhMuc: readonly comms_danhMucRa[];
  status: string;
  setStatus: (status: string) => void;
  /** What closes the row — §2's `[⊞ Danh mục tin]`, when the account may edit. */
  extra?: ReactNode;
}) {
  function gui(e: FormEvent) {
    e.preventDefault();
    timNgay();
  }

  return (
    <div className="hang-loc m-0 border-b border-line px-4 py-3.5">
      {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
          đang gõ vào một URL, và một URL đi vào mọi log truy cập (luật 3, cấm #4). */}
      <form className="form-tra-cuu" onSubmit={gui} role="search">
        <div className="o-nhap">
          <label htmlFor="tim-noi-dung">Tìm theo tiêu đề</label>
          <input
            id="tim-noi-dung"
            name="tim-noi-dung"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            autoComplete="off"
            // Máy chủ trả 400 khi quá trần, và câu từ chối CỐ Ý không nhắc lại chữ vừa gõ. Chặn ở ô
            // nhập để cán bộ thấy giới hạn thay vì thấy "không tải được".
            maxLength={TU_KHOA_TIM_TOI_DA}
            onChange={(e) => datTim(e.target.value)}
          />
        </div>
        <button className="nut-phu" type="submit">
          Tìm
        </button>
      </form>

      {/* `max-md:flex-none`: below 768px the row is a COLUMN, where Field's 220px basis would be a height. */}
      <Field label="Danh mục" htmlFor="loc-danh-muc" kind="select" icon={FolderTree} className="max-md:flex-none">
        <select
          id="loc-danh-muc"
          value={danhMucID}
          onChange={(e) => datDanhMucID(e.target.value)}
        >
          <option value="">{MOI_DANH_MUC}</option>
          {dungCayDanhMuc(danhMuc).map((m) => (
            <option key={m.dm.id} value={m.dm.id}>
              {nhanMucDanhMuc(m)}
            </option>
          ))}
        </select>
      </Field>

      <Field label="Trạng thái" htmlFor="loc-trang-thai" kind="select" icon={CircleDot} className="max-md:flex-none">
        <select id="loc-trang-thai" value={status} onChange={(e) => setStatus(e.target.value)}>
          {STATUS_FILTER_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </Field>

      {extra}
    </div>
  );
}

/**
 * Bảng §6.
 *
 * THE ACTION COLUMN HOLDS `✎` AND `🗑` (the prototype's pencil and trash). `🗑` only OPENS the dialog
 * that asks for the reason — nothing is deleted on the click itself.
 *
 * KHÔNG CỘT NÀO DỰNG HTML. `title` và `summary` là văn bản thuần ở đây; thân bài không có mặt
 * trong phản hồi danh sách và màn hình không đi đoán nó.
 *
 * WITHOUT `content.update` THE ACTION COLUMN IS NOT DRAWN AT ALL (`canEdit`), header included — an
 * empty `Thao tác` column is a column promising something.
 *
 * THE SUMMARY IS CLAMPED BY CSS TO ONE LINE (`.summary-one-line`), never cut by characters: the whole
 * text is in the page for a screen reader, and the cut follows the column's real width.
 */
export function BangNoiDung({
  ds,
  danhMuc,
  sua,
  remove,
  canEdit,
}: {
  ds: readonly comms_noiDungRa[];
  danhMuc: readonly comms_danhMucRa[];
  sua: (id: string) => void;
  /** Opens the delete dialog for this row. */
  remove: (nd: comms_noiDungRa) => void;
  canEdit: boolean;
}) {
  // ONE sentence for "nothing yet" and "nothing in this slice": the screen does not tell the two apart,
  // and `SO_RONG` already says "trong lát cắt đang xem" (spec §8b: only states the code distinguishes).
  if (ds.length === 0) return <EmptyState icon={Newspaper} title={SO_RONG} />;

  return (
    <>
      <div className={TABLE_SCROLL_CLASS}>
        <table className="bang-can-bo">
          <caption className="an-thi-giac">Sổ nội dung Mini App của xã</caption>
          <thead>
            <tr>
              <th scope="col">Tiêu đề</th>
              <th scope="col">Loại</th>
              <th scope="col">Chuyên mục</th>
              <th scope="col">Tệp đính kèm</th>
              <th scope="col">Ngày đăng</th>
              {/* Not sortable (owner, 02/10/2026): keyset paging over a number that moves while the
                  officer pages would skip and repeat rows. */}
              <th scope="col" className="text-right">
                {VIEW_COUNT_LABEL}
              </th>
              <th scope="col">Trạng thái</th>
              {canEdit && <th scope="col">{ACTIONS_COLUMN_LABEL}</th>}
            </tr>
          </thead>
          <tbody>
            {ds.map((nd) => {
              const trich = trichTomTat(nd.summary);
              const firstPublished = publishedAtLabel(nd);
              return (
                <tr key={nd.id}>
                  <td>
                    <span className="ten-can-bo">{nd.title}</span>
                    {trich !== "" && <span className="dong-phu summary-one-line">{trich}</span>}
                    {/* The portal sync's items say so under the title (ADR 0067 §2): the
                        commune must see which rows came from its portal, most of all the
                        `Chờ duyệt` ones waiting for somebody to publish them. */}
                    {nd.source === SOURCE_PORTAL_SYNC && (
                      <span className="dong-phu">{nhanNguon(nd.source)}</span>
                    )}
                    {/* The portal category it came in under (C2, 02/10/2026) — synced rows only. */}
                    {nd.source === SOURCE_PORTAL_SYNC && portalCategoryName(nd) !== "" && (
                      <span className="dong-phu">{portalCategoryLabel(portalCategoryName(nd))}</span>
                    )}
                    {nd.hand_edited && <span className="dong-phu">{NHAN_DA_SUA_TAY}</span>}
                  </td>
                  <td>{nhanLoai(nd.type)}</td>
                  <td>{tenDanhMuc(nd.category_id, danhMuc)}</td>
                  <td>
                    {nd.has_image ? (
                      <span className="inline-flex items-center gap-1.5">
                        <ImageIcon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 text-ink-500" />
                        {withoutLeadingGlyph(nhanTepDinhKem(true))}
                      </span>
                    ) : (
                      nhanTepDinhKem(false)
                    )}
                  </td>
                  <td>
                    {nhanNgayDang(nd.published_on)}
                    {firstPublished !== null && <span className="dong-phu">{firstPublished}</span>}
                  </td>
                  <td className="text-right tabular-nums">
                    <span className="inline-flex items-center justify-end gap-1.5">
                      <Eye aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 text-ink-500" />
                      {viewCountText(nd.view_count)}
                    </span>
                  </td>
                  <td>
                    <ContentStatusBadge status={nd.status} />
                  </td>
                  {canEdit && (
                    <td className="o-thao-tac flex-nowrap justify-end gap-1">
                      {/* 34px icon buttons (spec §7 "Nút trong bảng"). `min-h-0` lifts the legacy 44px of
                          `.o-thao-tac .nut-phu`; the names below are unchanged. */}
                      <Button
                        type="button"
                        variant="icon"
                        size="sm"
                        className="min-h-0"
                        aria-haspopup="dialog"
                        onClick={() => sua(nd.id)}
                        // Ký hiệu một mình không đọc được bằng trình đọc màn hình, và sáu hàng đều
                        // mang cùng một ký hiệu. Nhãn mang theo tiêu đề để nói rõ đang sửa bài nào.
                        aria-label={`${NHAN_NUT_SUA} Sửa: ${nd.title}`}
                        title="Sửa"
                      >
                        <Pencil aria-hidden="true" focusable="false" />
                      </Button>
                      <Button
                        type="button"
                        variant="icon"
                        size="sm"
                        className="nut-xoa min-h-0 border-transparent text-danger-600 hover:not-disabled:bg-danger-50 hover:not-disabled:text-danger-600"
                        aria-haspopup="dialog"
                        onClick={() => remove(nd)}
                        // Same reason as the pencil: the icon alone names no row.
                        aria-label={contentDeleteAriaLabel(nd.title)}
                        title={CONTENT_DELETE_TITLE}
                      >
                        <Trash2 aria-hidden="true" focusable="false" />
                      </Button>
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}

/**
 * Biểu mẫu §7, dùng cho cả THÊM và SỬA.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL, and it is one: `SoNoiDung` mounts it inside `OverlayDialog` (native
 * `<dialog>` + `showModal`, `overlay-dialog.tsx`). The form itself knows nothing of the overlay, so its
 * tests still render it alone.
 *
 * Ô `Nội dung` IS THE TIPTAP EDITOR (`RichTextEditor`), limited to the server's allow-list. It reports
 * a change only when the officer edits, so `thanSua` never sends a body nobody touched.
 *
 * KHÔNG CÓ Ô `Trạng thái`: máy chủ suy trạng thái từ ô tích `Đăng lên Mini App`. Một thân tự khai
 * trạng thái là một bài đăng vượt qua bước duyệt mà §10.2 dành cho lượt đồng bộ.
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
  loi,
  huy,
  luu,
  initialCoverState,
  initialAudioState,
}: {
  tieuDeForm: string;
  moTa: string;
  giaTriDau: GiaTriFormNoiDung;
  /** Hàng gốc, chỉ có khi đang SỬA — dùng cho khối thông tin chỉ đọc. */
  hang?: comms_noiDungRa;
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormNoiDung, khoaChongTrung: string) => void;
  /** Tests only: render the cover block in a given upload state (this suite has no DOM events). */
  initialCoverState?: CoverUploadState;
  /** Tests only: render the audio block in a given upload state. */
  initialAudioState?: AudioUploadState;
}) {
  const [gt, datGT] = useState<GiaTriFormNoiDung>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const [cover, setCover] = useState<CoverUploadState>(initialCoverState ?? { kind: "idle" });
  const [audioBusy, setAudioBusy] = useState(false);
  // THE ARTICLE EVERY FILE OF THIS FORM BELONGS TO: the saved article's id on the edit form; on the create
  // form, unknown until the first file — body image OR cover — reserves one (`body-image.ts`). Sent on
  // every later body image and on the cover, so the save finds all of them under one id. The first id
  // learned is kept (`a ?? id`).
  const [articleId, setArticleId] = useState<string | undefined>(hang?.id);
  // `file_id → preview_url` for the editor's figures: the detail's `body_images`, then each ready reply.
  const [previews, setPreviews] = useState<ReadonlyMap<string, string>>(() => previewsFromItem(hang?.body_images));
  const [bodyImageBusy, setBodyImageBusy] = useState(false);

  const tieuDeGon = gt.title.trim();
  // MỘT ĐIỀU KIỆN, KHÔNG BA. Hợp đồng đánh dấu `title` và `type` bắt buộc; `type` luôn có giá trị
  // vì ô chọn mặc định `Tin tức` và danh sách đóng. Máy chủ vẫn là nơi từ chối thật; nút tắt chỉ
  // để không phải gõ lại.
  //
  // The per-type check is shown LIVE next to the boxes and holds Lưu off: the author sees "end before
  // start" while fixing it, not after a round trip. It mirrors the server; the server's own 400 still
  // reaches `loi` word for word.
  // The edit form passes its starting values: two banner rules depend on what the item WAS.
  const typeFieldError = validateTypeFields(gt, hang === undefined ? undefined : giaTriDau);
  // A cover still moving holds Lưu: saving now would drop the image the officer just chose.
  const coverBusy = coverInFlight(cover);
  // An audio upload still moving holds Lưu too: closing the form mid-upload would hide its outcome.
  // A body image still moving holds Lưu too: it is inserted only once ready, and the form would be gone.
  const duDieuKien = tieuDeGon !== "" && typeFieldError === null && !coverBusy && !audioBusy && !bodyImageBusy;
  // The audio is uploaded for a SAVED broadcast only (the server requires `content_item_id`).
  const savedAsBroadcast = hang !== undefined && hang.type === CONTENT_TYPE_BROADCAST;
  const bannerLinkWarning = gt.type === CONTENT_TYPE_BANNER ? bannerLinkTapWarning(gt.link_to) : null;

  // Each state report; a `ready` one makes its id the form's cover. Functional update: the upload
  // resolves after renders the officer may have made meanwhile (typing the title), and those stay.
  // The first article id any file's reply names is the form's; a later one never replaces it.
  function keepArticleId(id: string): void {
    setArticleId((a) => a ?? id);
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
    setBodyImageBusy(s.kind === "requesting" || s.kind === "uploading" || s.kind === "checking" || s.kind === "fetching");
    if (s.kind !== "ready") return;
    const { fileId, contentItemId, previewUrl } = s.image;
    setArticleId((a) => a ?? contentItemId);
    if (previewUrl !== undefined && previewUrl !== "") {
      setPreviews((m) => new Map(m).set(fileId, previewUrl));
    }
  }

  // A cover's FIRST declaration on a new article is in flight: its reply will name the reserved id, and
  // a body image declared now, with no id, would reserve a second draft. Held for that one round trip.
  const coverReserving = articleId === undefined && cover.kind === "requesting";

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
    retry: (id, onState) =>
      retryBodyImageCompletion(id, (s) => {
        onBodyImageState(s);
        onState(s);
      }),
  };

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    luu(gt, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc m-0 border-0 bg-transparent p-0" onSubmit={guiNgay} aria-labelledby="tieu-de-form-noi-dung">
      {/* HEADER sticks to the top of the dialog's own scroll box (the dialog is `p-0` for it): the
          title and the way out stay in view while the officer scrolls a long article. The X calls the
          same `huy` as `Huỷ`, and is off while saving for the same reason. */}
      <div className="sticky top-0 z-10 flex items-start gap-3 border-b border-line bg-surface px-5 py-4">
        <div className="min-w-0 flex-1">
          <h3 id="tieu-de-form-noi-dung" className="m-0 text-lg leading-snug font-semibold text-ink-900">
            {tieuDeForm}
          </h3>
        </div>
        <IconButton type="button" label={CLOSE_LABEL} className="-mt-1 -mr-2 min-h-0" disabled={dangGui} onClick={huy}>
          <X aria-hidden="true" focusable="false" />
        </IconButton>
      </div>

      <div className="flex flex-col gap-5 px-5 py-5">
        {hang !== undefined && <ThongTinChiDoc hang={hang} />}

        <FormSection title="Thông tin chung" icon={FileText}>
          {/* TWO SELECTS, ONE ROW from 640px — each a full column, label above, 40px (owner, 02/10/2026:
              the selects were too narrow, label beside them). The native `<select>` is unchanged. */}
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Loại nội dung" htmlFor="loai-noi-dung" kind="select" icon={Shapes} grow="auto">
              <select
                id="loai-noi-dung"
                value={gt.type === "" ? LOAI_MAC_DINH : gt.type}
                onChange={(e) => datGT({ ...gt, type: e.target.value })}
              >
                {MOI_LOAI.map((ma) => (
                  <option key={ma} value={ma}>
                    {nhanLoai(ma)}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Danh mục" htmlFor="danh-muc-noi-dung" kind="select" icon={FolderTree} grow="auto">
              <select
                id="danh-muc-noi-dung"
                value={gt.category_id}
                onChange={(e) => datGT({ ...gt, category_id: e.target.value })}
              >
                <option value="">{CHUA_XEP_DANH_MUC}</option>
                {dungCayDanhMuc(danhMuc).map((m) => (
                  <option key={m.dm.id} value={m.dm.id}>
                    {nhanMucDanhMuc(m)}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <Field label="Tiêu đề *" htmlFor="tieu-de-noi-dung" grow="auto">
            <input
              id="tieu-de-noi-dung"
              name="tieu-de-noi-dung"
              value={gt.title}
              maxLength={TIEU_DE_TOI_DA}
              autoComplete="off"
              onChange={(e) => datGT({ ...gt, title: e.target.value })}
            />
          </Field>

          <Field
            label="Tóm tắt"
            htmlFor="tom-tat-noi-dung"
            grow="auto"
            className="[&_textarea]:py-2.5"
            hint={SUMMARY_SAPO_HINT}
          >
            <textarea
              id="tom-tat-noi-dung"
              name="tom-tat-noi-dung"
              rows={3}
              value={gt.summary}
              maxLength={TOM_TAT_TOI_DA}
              onChange={(e) => datGT({ ...gt, summary: e.target.value })}
            />
          </Field>

          {/* :131 of the spec, ADR 0047 §6. Rendered only for the type that can carry them — the server
              refuses an event field on any other type, and a box that is shown is a box somebody fills. */}
          {gt.type === CONTENT_TYPE_EVENT && (
            <>
              <div className="grid gap-4 sm:grid-cols-2">
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
                <p className="m-0 text-xs text-ink-500 sm:col-span-2">{EVENT_TIME_HINT}</p>
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
            <Field label="Liên kết video" htmlFor="lien-ket-video" icon={Video} grow="auto" hint={VIDEO_URL_HINT}>
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
        </FormSection>

        <FormSection title="Nội dung" icon={Pilcrow}>
          <div className="flex flex-col gap-1.5">
            <span id="than-bai-noi-dung-nhan" className="nhan-o text-xs leading-tight font-semibold text-ink-700">
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
            <p className="ghi-chu m-0 text-xs text-ink-500" id="than-bai-noi-dung-hint">
              {CANH_BAO_HTML_THO}
            </p>
            {gt.body === "" && <p className="ghi-chu m-0 text-xs text-ink-500">{THAN_BAI_RONG}</p>}
          </div>
        </FormSection>

        <FormSection title="Ảnh, âm thanh và đính kèm" icon={ImageIcon}>
          {gt.type === CONTENT_TYPE_BANNER && (
            <Notice tone="info">{BANNER_COVER_NOTICE}</Notice>
          )}

          <CoverImageField
            fileId={gt.cover_image_file_id}
            savedFileId={hang?.cover_image_file_id ?? ""}
            savedCover={hang?.cover_image}
            state={cover}
            // While the FIRST body image of a new article is still reserving its id, a cover would
            // reserve another one.
            disabled={dangGui || (bodyImageBusy && articleId === undefined)}
            // The article id a body image reserved, when there is one — one draft for every file. A
            // cover uploaded first reserves it instead, and the body images then use the cover's.
            onPick={(file) => void runCoverUpload(file, articleId, onCoverState, keepArticleId)}
            onRetry={(id) => void retryCoverCompletion(id, onCoverState, keepArticleId)}
            onRemove={() => {
              setCover({ kind: "idle" });
              datGT((g) => ({ ...g, cover_image_file_id: "" }));
            }}
          />

          {/* ADR 0067 §4. The audio is uploaded for a SAVED broadcast: on the create form, or on an item
              saved as another type, the officer is told to save first instead of being shown a picker the
              server would refuse (400 / 422 `audio_only_for_truyen_thanh`). */}
          {gt.type === CONTENT_TYPE_BROADCAST && !savedAsBroadcast && (
            <Notice tone="neutral" icon={AudioLines} role="note">
              {AUDIO_SAVE_FIRST}
            </Notice>
          )}
          {savedAsBroadcast && (
            <BroadcastAudioField
              itemId={hang.id}
              initialAudio={hang.audio ?? null}
              currentType={gt.type}
              disabled={dangGui}
              onBusyChange={setAudioBusy}
              initialState={initialAudioState}
            />
          )}

          {/* ADR 0067 §5. Only for `Banner`: the server refuses `link_to` / `display_order` on any other type
              (422), and a type change away from banner clears both there — so these boxes are not sent then. */}
          {gt.type === CONTENT_TYPE_BANNER && (
            <div className="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
              <div className="flex flex-col gap-1.5">
                <Field label="Liên kết khi bấm" htmlFor="lien-ket-banner" icon={LinkIcon} grow="auto">
                  <input
                    id="lien-ket-banner"
                    name="lien-ket-banner"
                    value={gt.link_to}
                    maxLength={LINK_TO_MAX_CHARS}
                    placeholder="/tin-tuc hoặc https://"
                    autoComplete="off"
                    list="lien-ket-banner-duong-app"
                    aria-describedby={
                      bannerLinkWarning === null ? "lien-ket-banner-goi-y" : "lien-ket-banner-goi-y lien-ket-banner-canh-bao"
                    }
                    onChange={(e) => datGT({ ...gt, link_to: e.target.value })}
                  />
                </Field>
                {/* The in-app paths the citizen Mini App can open (`BANNER_APP_PATHS`); free https:// still typed. */}
                <datalist id="lien-ket-banner-duong-app">
                  {BANNER_APP_PATHS.map((p) => (
                    <option key={p.path} value={p.path} label={p.label} />
                  ))}
                </datalist>
                <p className="ghi-chu m-0 text-xs text-ink-500" id="lien-ket-banner-goi-y">
                  {LINK_TO_HINT} {BANNER_ARTICLE_PATH_HINT}
                </p>
                {bannerLinkWarning !== null && (
                  // A WARNING, not a refusal: the server accepts any in-app path, and the Mini App may learn
                  // the screen later. The officer just must not believe it is tappable today.
                  <p className="ghi-chu m-0 text-xs text-warning-600" role="status" id="lien-ket-banner-canh-bao">
                    {bannerLinkWarning}
                  </p>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                <Field label="Thứ tự hiển thị" htmlFor="thu-tu-banner" icon={ListOrdered} grow="auto">
                  <input
                    id="thu-tu-banner"
                    name="thu-tu-banner"
                    type="number"
                    inputMode="numeric"
                    min={0}
                    max={DISPLAY_ORDER_MAX}
                    step={1}
                    value={gt.display_order}
                    aria-describedby="thu-tu-banner-goi-y"
                    onChange={(e) => datGT({ ...gt, display_order: e.target.value })}
                  />
                </Field>
                <p className="ghi-chu m-0 text-xs text-ink-500" id="thu-tu-banner-goi-y">
                  {DISPLAY_ORDER_HINT}
                </p>
              </div>
            </div>
          )}
        </FormSection>

        <FormSection title="Đăng lên Mini App" icon={Send}>
          {hang !== undefined && hang.status === "cho-duyet" && (
            <Notice tone="info" icon={Clock} role="note">
              {PENDING_REVIEW_HINT}
            </Notice>
          )}
          <label
            htmlFor="dang-len-mini-app"
            // The tint follows the form's own value (no `:checked` selector: the tests read the markup for
            // the word `checked` to know whether the box is ticked).
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-xl border px-3.5 py-3 text-sm font-semibold text-ink-900",
              gt.publish ? "border-brand-500 bg-brand-50" : "border-line-strong bg-surface",
            )}
          >
            <input
              id="dang-len-mini-app"
              name="dang-len-mini-app"
              type="checkbox"
              className="size-5 shrink-0 accent-brand-600"
              checked={gt.publish}
              onChange={(e) => datGT({ ...gt, publish: e.target.checked })}
            />{" "}
            {NHAN_O_DANG}
          </label>
          {/* The dialog's sentence (§7) is about exactly this tick, so it stands next to it. */}
          <Notice tone="neutral">{moTa}</Notice>
        </FormSection>

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
      </div>

      {/* FOOTER sticks to the bottom of the dialog: `Lưu` is reachable without scrolling to the end of a
          long article. `type` FIRST on the native buttons — the tests read `<button type="submit"…`. */}
      <div className="sticky bottom-0 z-10 flex flex-wrap items-center justify-end gap-2 border-t border-line bg-surface px-5 py-3">
        {coverBusy && <p className="ghi-chu m-0 mr-auto text-xs text-ink-500">{COVER_WAIT_NOTE}</p>}
        {audioBusy && <p className="ghi-chu m-0 mr-auto text-xs text-ink-500">{AUDIO_WAIT_NOTE}</p>}
        <button
          type="button"
          className={cn("nut-phu w-auto min-h-0", buttonVariants({ variant: "secondary", size: "md" }))}
          disabled={dangGui}
          onClick={huy}
        >
          {NHAN_NUT_HUY}
        </button>
        {/* Busy: the word becomes "Đang lưu…" with a spinner, in a button that keeps its width. */}
        <button
          type="submit"
          className={cn("nut-chinh w-auto min-h-0 min-w-[7.5rem]", buttonVariants({ variant: "primary", size: "md" }))}
          disabled={dangGui || !duDieuKien}
          aria-busy={dangGui || undefined}
        >
          {dangGui && <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />}
          {dangGui ? SAVING_LABEL : NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * §7 `Ảnh đại diện` — `Chọn tệp từ máy` · `JPG, PNG hoặc WebP — tối đa 50MB`. Presentational: the
 * form owns the id and the upload state (`cover-image.ts` runs the flow).
 *
 * THE PREVIEW IS THE SERVER'S SIGNED LINK AND NOTHING ELSE (`coverPreviewSrc`), shown only for the
 * cover already saved on the article. A just-uploaded file shows a sentence, not a picture: there is
 * no server preview for it until the article is saved, and a `blob:` of the officer's own file would
 * be a second, unchecked source of `src` on this screen.
 *
 * Same control shape as `📎 Đính kèm` (`task-attachments-ui.tsx`): the file input is visually hidden
 * and stays in the tab order, its label is the visible button, ringed on focus; `.task-attachments`
 * gives every control 44 px and 16 px text (reused rather than a new CSS rule).
 */
export function CoverImageField({
  fileId,
  savedFileId,
  savedCover,
  state,
  disabled,
  onPick,
  onRetry,
  onRemove,
}: {
  /** The form's current `cover_image_file_id`, `""` = none. */
  fileId: string;
  /** The article's cover when the form opened, `""` on the create form. */
  savedFileId: string;
  /** The detail route's cover block, edit form only. */
  savedCover: comms_coverImageOut | null | undefined;
  state: CoverUploadState;
  disabled: boolean;
  onPick: (file: File) => void;
  onRetry: (id: string) => void;
  onRemove: () => void;
}) {
  const busy = coverInFlight(state);
  const showingSaved = fileId !== "" && fileId === savedFileId;
  const previewSrc = showingSaved ? coverPreviewSrc(savedCover) : null;
  const stateText = coverStateText(state);
  const refused = state.kind === "refused" || state.kind === "retry";

  return (
    <div
      className="o-nhap task-attachments m-0 flex flex-col items-start gap-2 rounded-xl border border-dashed border-line-strong bg-surface-muted p-4 [&_p]:m-0"
      role="group"
      aria-labelledby="anh-noi-dung-nhan"
    >
      <span id="anh-noi-dung-nhan" className="flex items-center gap-2 text-sm font-semibold text-ink-900">
        <ImagePlus aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
        Ảnh đại diện
      </span>

      {showingSaved && (
        <>
          <p>{savedCoverText(savedCover)}</p>
          {previewSrc !== null ? (
            // A plain <img>, not next/image: the optimiser would fetch this presigned link server-side
            // and cache a bearer credential under the commune's own origin.
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={previewSrc}
              alt={COVER_PREVIEW_ALT}
              width={240}
              className="h-auto max-w-full rounded-lg border border-line bg-surface"
            />
          ) : (
            <p className="ghi-chu">{COVER_PREVIEW_MISSING}</p>
          )}
        </>
      )}
      {fileId === "" && savedFileId !== "" && <p>{COVER_WILL_DETACH}</p>}
      {fileId === "" && savedFileId === "" && <p className="ghi-chu">{COVER_NONE}</p>}

      {/* The input FIRST, visually hidden but focusable; the label right after it is the visible
          button (`input:focus-visible + label`). Enter/Space on the focused input open the picker. */}
      <input
        id="anh-noi-dung"
        name="anh-noi-dung"
        type="file"
        accept={COVER_ACCEPT}
        className="an-thi-giac"
        aria-describedby="anh-noi-dung-goi-y"
        disabled={disabled || busy}
        onChange={(e) => {
          const f = e.target.files?.[0];
          e.target.value = ""; // the same file can be chosen again after a refusal
          if (f !== undefined) onPick(f);
        }}
      />
      <label htmlFor="anh-noi-dung" className="nut-phu">
        {fileId === "" ? COVER_PICK_BUTTON : COVER_REPLACE_BUTTON}
        <span className="an-thi-giac"> — Ảnh đại diện</span>
      </label>
      <p className="ghi-chu text-xs text-ink-500" id="anh-noi-dung-goi-y">
        {COVER_HINT}
      </p>

      {stateText !== "" && (
        <p role={refused ? "alert" : "status"} className={refused ? "thong-bao-loi" : undefined}>
          {stateText}
        </p>
      )}

      {(state.kind === "retry" || fileId !== "") && (
        <div className="cum-nut">
          {state.kind === "retry" && (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<RefreshCw aria-hidden="true" focusable="false" />}
              disabled={disabled}
              onClick={() => onRetry(state.id)}
            >
              {COVER_RETRY_BUTTON}
            </Button>
          )}
          {fileId !== "" && (
            <Button
              type="button"
              variant="danger"
              size="sm"
              icon={<X aria-hidden="true" focusable="false" />}
              disabled={disabled || busy}
              onClick={onRemove}
            >
              {COVER_REMOVE_BUTTON}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Khối chỉ đọc của biểu mẫu SỬA — những thứ máy chủ quyết và biểu mẫu không đổi được.
 *
 * `Lượt xem` (`view_count`) is shown READ-ONLY: the server counts each open of the article in the
 * Mini App (ADR 0047, row 02/10/2026) and the form never sends it on write. `hand_edited` là nửa nhìn thấy được của §10.4, và
 * đây là chỗ duy nhất màn hình biết được rằng bản sửa vừa rồi nay được bảo vệ khỏi lượt đồng bộ
 * sau. `source_url` hiện dưới dạng CHỮ, không phải một liên kết bấm được: máy chủ chỉ nhận
 * `http(s)` khi GHI, nhưng một hàng cũ trong CSDL không có gì bảo đảm điều đó, và một `href` dựng
 * từ dữ liệu chưa kiểm là đúng lỗ hổng danh sách trắng lược đồ sinh ra để chặn.
 */
export function ThongTinChiDoc({ hang }: { hang: comms_noiDungRa }) {
  const firstPublished = publishedAtLabel(hang);
  return (
    // `dt`/`dd` stay class-free (the tests read `<dt>…</dt><dd>…</dd>`): the frame styles them from here.
    <dl
      className={cn(
        "danh-sach-truong m-0 grid gap-x-6 gap-y-3 rounded-xl border border-line bg-surface-muted p-4 sm:grid-cols-2",
        "[&_dt]:text-xs [&_dt]:font-semibold [&_dt]:text-ink-500 [&_dd]:m-0 [&_dd]:mt-0.5 [&_dd]:text-sm [&_dd]:text-ink-900",
      )}
    >
      <div>
        <dt>Trạng thái</dt>
        <dd>
          <ContentStatusBadge status={hang.status} />
        </dd>
      </div>
      <div>
        <dt>Ngày đăng</dt>
        <dd>
          {nhanNgayDang(hang.published_on)}
          {firstPublished !== null && <> · {firstPublished}</>}
        </dd>
      </div>
      <div>
        <dt>{VIEW_COUNT_LABEL}</dt>
        <dd>{viewCountText(hang.view_count)}</dd>
      </div>
      <div>
        <dt>Nguồn</dt>
        <dd>
          {nhanNguon(hang.source)}
          {hang.hand_edited && <> · {NHAN_DA_SUA_TAY}</>}
        </dd>
      </div>
      {hang.source === SOURCE_PORTAL_SYNC && portalCategoryName(hang) !== "" && (
        <div>
          <dt>Chuyên mục Cổng</dt>
          <dd>{portalCategoryName(hang)}</dd>
        </div>
      )}
      <div>
        <dt>Liên kết bài gốc</dt>
        <dd>{hang.source_url === "" ? DAU_GACH : hang.source_url}</dd>
      </div>
      {/* The LEGACY image link (`image_url`) has no box any more; a row that still carries one shows
          it as TEXT, never as a `src` — same reason as `source_url` above. */}
      {hang.image_url !== "" && (
        <div>
          <dt>Liên kết ảnh (cách cũ)</dt>
          <dd>{hang.image_url}</dd>
        </div>
      )}
      <div>
        <dt>Người soạn</dt>
        {/* MÃ NGHIỆP VỤ (`CB-2026-7K3M9Q`), không phải họ tên và không phải id nội bộ (luật 6,
            bất biến 8). `service-comms` không sở hữu danh bạ cán bộ nên không có tên để nối. */}
        <dd>{hang.author_code === "" ? DAU_GACH : hang.author_code}</dd>
      </div>
      <div>
        <dt>Cập nhật lúc</dt>
        <dd>{nhanMoc(hang.updated_at)}</dd>
      </div>
    </dl>
  );
}

/**
 * Biểu mẫu `⊞ Danh mục tin` §6 — THÊM một danh mục. Sửa, ẩn/hiện và xoá nằm ở `CategoryAdmin`
 * (`category-admin.tsx`, ADR 0067 §3), drawn right above this form.
 */
export function FormDanhMuc({
  danhMuc,
  dangGui,
  loi,
  huy,
  luu,
}: {
  danhMuc: readonly comms_danhMucRa[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (ten: string, slug: string, chaID: string, thuTu: number, khoa: string) => void;
}) {
  const [ten, datTen] = useState("");
  const [slug, datSlug] = useState("");
  const [chaID, datChaID] = useState("");
  const [thuTu, datThuTu] = useState("0");
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tenGon = ten.trim();
  const slugGon = slug.trim();
  const duDieuKien = tenGon !== "" && slugGon !== "";

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    // Số không đọc được thành 0: ô là `type=number`, nhưng một ô số rỗng cho ra chuỗi rỗng, và
    // `Number("")` là 0 — viết ra để không ai phải đoán.
    const n = Number.parseInt(thuTu, 10);
    luu(tenGon, slugGon, chaID, Number.isNaN(n) ? 0 : n, khoaChongTrung);
  }

  const cay = dungCayDanhMuc(danhMuc);

  return (
    <form
      className="form-danh-muc mt-4 mb-0 flex flex-col gap-4 rounded-xl border border-line border-l-line bg-surface-muted p-4 [&_h3]:m-0 [&_h3]:text-[15px]"
      onSubmit={guiNgay}
      aria-labelledby="tieu-de-form-danh-muc"
    >
      <h3 id="tieu-de-form-danh-muc" className="flex items-center gap-2">
        <Plus aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
        Thêm danh mục tin
      </h3>

      {cay.length === 0 && <p className="ghi-chu m-0">{DANH_MUC_RONG}</p>}

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Tên danh mục *" htmlFor="ten-danh-muc" grow="auto">
          <input
            id="ten-danh-muc"
            name="ten-danh-muc"
            value={ten}
            maxLength={TEN_DANH_MUC_TOI_DA}
            autoComplete="off"
            onChange={(e) => datTen(e.target.value)}
          />
        </Field>

        <Field label="Slug *" htmlFor="slug-danh-muc" grow="auto">
          <input
            id="slug-danh-muc"
            name="slug-danh-muc"
            value={slug}
            maxLength={SLUG_DANH_MUC_TOI_DA}
            autoComplete="off"
            onChange={(e) => datSlug(e.target.value)}
          />
        </Field>
        {/* MÁY CHỦ TỪ CHỐI CHỨ KHÔNG TỰ HẠ CHỮ HOA: `Chuyen-Doi-So` bị trả 400 thay vì lặng lẽ
            thành `chuyen-doi-so`, vì mã lưu xuống phải đúng mã người ta thấy lúc gõ. Nói trước
            điều đó thay vì để họ gõ xong mới biết. */}
        <p className="ghi-chu m-0 sm:col-span-2">
          Chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ <code>chuyen-doi-so</code>. Slug đã
          cấp thì KHÔNG cấp lại, kể cả khi danh mục mang slug đó đã bị xoá.
        </p>

        <Field label="Danh mục cha" htmlFor="cha-danh-muc" kind="select" icon={FolderTree} grow="auto">
          <select id="cha-danh-muc" value={chaID} onChange={(e) => datChaID(e.target.value)}>
            <option value="">— Không có danh mục cha —</option>
            {cay.map((m) => (
              <option key={m.dm.id} value={m.dm.id}>
                {nhanMucDanhMuc(m)}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Thứ tự hiển thị" htmlFor="thu-tu-danh-muc" icon={ListOrdered} grow="auto">
          <input
            id="thu-tu-danh-muc"
            name="thu-tu-danh-muc"
            type="number"
            min={0}
            max={THU_TU_DANH_MUC_TOI_DA}
            value={thuTu}
            onChange={(e) => datThuTu(e.target.value)}
          />
        </Field>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <button
          type="button"
          className={cn("nut-phu w-auto min-h-0", buttonVariants({ variant: "secondary", size: "md" }))}
          disabled={dangGui}
          onClick={huy}
        >
          {NHAN_NUT_HUY}
        </button>
        <button
          type="submit"
          className={cn("nut-chinh w-auto min-h-0 min-w-[7.5rem]", buttonVariants({ variant: "primary", size: "md" }))}
          disabled={dangGui || !duDieuKien}
          aria-busy={dangGui || undefined}
        >
          {dangGui && <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />}
          {dangGui ? SAVING_LABEL : NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * Status pill by the server's CODE (spec §7: icon + word, never colour alone). The word is still
 * `nhanTrangThai`'s; only the look follows the code — `Chờ duyệt` is amber (waiting), `Ẩn` grey.
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

/** One titled group of the editor form (Thông tin chung · Nội dung · …). Presentation only. */
function FormSection({ title, icon: Icon, children }: { title: string; icon: LucideIcon; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4 border-t border-line pt-5 first:border-t-0 first:pt-0">
      <h4 className="m-0 flex items-center gap-2 text-sm font-semibold text-ink-900">
        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 text-brand-600" />
        {title}
      </h4>
      {children}
    </section>
  );
}

/*
 * LOCAL SKELETONS — plain markup, no shared component (a shared Skeleton is being built in another
 * change; this file must not depend on it yet). Decorative: `aria-hidden`, the status sentence next to
 * them is what a screen reader hears. Fixed widths, never random (rule 13 forbids `Math.random`).
 */
const SKELETON_BAR = "block h-3 rounded-full bg-line motion-safe:animate-pulse";
const SKELETON_WIDTHS = ["w-3/5", "w-2/5", "w-1/2", "w-3/4", "w-2/5"] as const;

function RowsSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col">
      {SKELETON_WIDTHS.map((w, i) => (
        <div key={i} className="flex h-12 items-center gap-4 border-b border-line px-4">
          <span className={cn(SKELETON_BAR, w)} />
          <span className={cn(SKELETON_BAR, "ml-auto w-16")} />
          <span className={cn(SKELETON_BAR, "w-20")} />
        </div>
      ))}
    </div>
  );
}

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
