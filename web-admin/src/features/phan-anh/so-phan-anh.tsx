"use client";

import {
  AlarmClock,
  ArrowUp,
  Building2,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  CloudOff,
  ImageOff,
  Landmark,
  Map as MapIcon,
  MapPin,
  MessageSquare,
  Paperclip,
  RefreshCw,
  Search,
  Shapes,
  ShieldAlert,
  Star,
  UserRound,
  X,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { PendingButton, PendingFeature, PendingMarker, PendingTab } from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";
import { cn } from "@/lib/cn";
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
import type { KetQua } from "@/lib/api/goi";
import {
  chuyenCapTrenPhieu,
  chuyenXuLyPhieu,
  dongPhieu,
  khongTiepNhanPhieu,
  layPhieuPhanAnh,
  laySoPhanAnh,
  phanLoaiPhieu,
  setPetitionPublication,
  tienTrangThaiPhieu,
  type LocPhanAnh,
  type PublicationTarget,
} from "@/lib/api/phieu-phan-anh";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  identity_thonToDanPhoRa,
  page_Result_petitions_phieuPhanAnhRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { coQuyen, REPORT_READ_PERMISSION } from "@/lib/quyen";
import { DrillDownBanner } from "@/components/drill-down-banner";
import { NO_DRILL_DOWN, drillDownQuery, type DrillDown } from "@/lib/drill-down";
import { residentialUnitFilterLabel } from "@/features/cau-hinh/nhan-thon";

import {
  AFTER_PHOTO_GO_TO,
  AFTER_PHOTO_REQUIRED_HINT,
  ASSIGNED_TOAST,
  branchReasonLabel,
  CANH_BAO_RE_NHANH,
  cardSenderLabel,
  CARD_NO_LOCATION,
  cauGiaiThichTrangThai,
  CHI_TRE_HAN_NHAN,
  CO_QUAN_TOI_DA,
  composerTitle,
  congThaoTac,
  DANG_TAI_SO,
  danhBaTheoMa,
  dateTimeLabel,
  DE_BO_PHAN_PHAN_CONG,
  demKyTu,
  GHI_CHU_O_KET_QUA,
  GHI_CHU_TOI_DA,
  GOI_Y_CO_QUAN,
  HIDDEN_TOAST,
  initialClassifyField,
  isContactUnverified,
  LINH_VUC_PHAN_ANH,
  linhVucPhanAnh,
  LIST_EMPTY_HINT,
  LIST_EMPTY_TITLE,
  loiCoQuan,
  loiGhiChuNoiBo,
  loiLyDo,
  lopHan,
  LOW_RATING_FILTER_LABEL,
  LOW_RATING_MAX,
  LOW_RATING_REOPENED,
  LY_DO_TOI_DA,
  MOI_BO_PHAN_NHAN,
  MOI_DIA_BAN_NHAN,
  MOI_KENH,
  MOI_KENH_NHAN,
  MOI_LINH_VUC_NHAN,
  MOI_TRANG_THAI,
  MOI_TRANG_THAI_NHAN,
  movedToast,
  NHAN_CHON_CAN_BO,
  NHAN_O_CO_QUAN,
  NHAN_O_GHI_CHU_NOI_BO,
  NHAN_O_KET_QUA,
  NHAN_O_LY_DO,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanHan,
  nhanKenh,
  nhanLinhVuc,
  nhanLuaChonCanBo,
  nhanNguoiGui,
  nhanThoiDiem,
  nhanTrangThai,
  NO_ASSIGNEE_YET,
  NO_UNIT_YET,
  PHAM_VI_GIAO_CHO_TOI,
  PHAM_VI_TOAN_XA,
  petitionPendingPart,
  petitionTaskOffered,
  PUBLISHED_TOAST,
  RATING_MAX_STARS,
  RATING_TITLE,
  ratingStars,
  ratingView,
  RE_NHANH,
  reopenLine,
  RESTRICTED_FEEDBACK_PERMISSION,
  RESTRICTED_FLOW_NOTE,
  RESULT_PLACEHOLDER,
  SCENE_COORDINATES_NOTE,
  LOCATION_SECTION_TITLE,
  SCENE_NO_ADDRESS,
  sceneCoordinates,
  SCOPE_RELATED_LABEL,
  SO_RONG,
  STRIP_NO_PERMISSION,
  statusStrip,
  TIM_PLACEHOLDER,
  trangThaiHan,
  UNIT_WITHOUT_STAFF,
  UNVERIFIED_CONTACT_NOTE,
  UPDATE_NOTE_INTERNAL,
  UPDATE_NOTE_PLACEHOLDER,
  type CongThaoTac,
  type StatusStripView,
  type StripStep,
} from "./nhan-phieu";
import { PublicationBox, starsLabel } from "./citizen-report-blocks";
import { NhatKyPhieu } from "./nhat-ky-phieu";
import { PetitionTaskBlock } from "./petition-task";
import {
  buttonClass,
  Glyph,
  HINT_CLASS,
  LoadingBar,
  PetitionCardsSkeleton,
  PetitionStatusBadge,
  statusActiveClass,
  statusIcon,
  TEXTAREA_CLASS,
  UnverifiedContactBadge,
} from "./petition-ui";
import { PetitionKpis } from "./petition-kpis";
import { afterPhotosHeadingId, ScenePhotos } from "./scene-photos";
import { VerificationPhotos } from "./verification-photos";
import {
  QUYEN_DONG_PHAN_ANH,
  QUYEN_PHAN_CONG_PHAN_ANH,
  QUYEN_PHAN_LOAI_PHAN_ANH,
  QUYEN_XEM_PHAN_ANH,
} from "@/lib/quyen";

/**
 * Sổ Phản ánh của người dân — `docs/ui-ux/09-phan-anh-nguoi-dan.md` §2 (quyển sổ), §4 (bộ lọc),
 * §8 (chi tiết) và bốn thao tác xử lý.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỂM CHÍNH CỦA MÀN HÌNH NÀY LÀ HAI CỔNG KHÔNG GIỐNG NHAU, và nó dễ bị gộp lại "cho gọn":
 *
 *   chip bước kế tiếp (advance)  KHÔNG có cổng ở giao diện. Tuyến khai `feedback.read`, điều kiện
 *                               thật là `feedback.resolve` HOẶC chính là cán bộ được phân công —
 *                               LUẬT NẮM GIỮ. Vế thứ hai giao diện cố ý không tính lại (xem
 *                               `congThaoTac`), nên chip bấm được với mọi người xem được sổ và câu
 *                               403 của máy chủ ra thẳng màn hình.
 *   chip `Đã đóng` (close)      CÓ cổng: `feedback.resolve`, và **không** được nới theo luật nắm
 *                               giữ. Đóng phiếu ghi một kết quả NGƯỜI DÂN ĐỌC (luật 10, bất biến
 *                               6) — câu hỏi mở #7 chốt 16/09/2026.
 *
 * Since 09/10/2026 (owner decision D1) both are chips of the status strip; `statusStrip` decides which
 * chip is pressable and which act it opens.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * DỮ LIỆU CÁ NHÂN VỀ ĐÃ CHE, VÀ MÀN HÌNH KHÔNG GHÉP LẠI. Ở tuyến danh sách thì che là tuyệt đối —
 * kể cả tài khoản có `feedback.unmask` — vì một lời gọi mở hai mươi người gửi không viết nổi một
 * dòng vết kiểm toán trung thực (luật 6, bất biến 7).
 *
 * PHIẾU LĨNH VỰC `can-bo` KHÔNG CÓ TRONG TRANG khi tài khoản thiếu `feedback.restricted`, và màn
 * hình **không** nói "có phiếu bị ẩn": nói ra là nói cho một đồng nghiệp của người bị phản ánh
 * biết rằng phiếu ấy tồn tại (luật 4, cấm #2).
 */

/** Bao nhiêu thẻ một trang. Đủ để lướt buổi sáng, không nhiều tới mức trang đầu tải chậm. */
const SO_THE_MOI_TRANG = 20;

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

/** Một thao tác đã gửi: `true`/`false` = máy chủ nhận / từ chối; `void` = bên gọi không nói. */
type KetQuaGui = void | Promise<boolean>;

/**
 * Xoá ô ghi chú nội bộ CHỈ khi thao tác thành công. Thất bại thì chữ còn nguyên để sửa rồi gửi lại —
 * xoá lúc bấm là bắt cán bộ gõ lại mỗi lần máy chủ trả 409.
 */
function xoaKhiThanhCong(kq: KetQuaGui, xoa: () => void): void {
  void Promise.resolve(kq).then((ok) => {
    if (ok === true) xoa();
  });
}

/** Bộ lọc đang chọn trên màn hình. Cùng hình dạng với `LocPhanAnh`, trừ phân trang. */
type BoLoc = Omit<LocPhanAnh, "limit" | "cursor">;

const KHONG_LOC: BoLoc = {};

/** Selects of a processing act: label above, 40px, full column; two columns from 640px. */
const SELECT_GRID = "grid grid-cols-1 gap-4 sm:grid-cols-2";

/** Câu của dải lọc Tổng quan trên sổ này — nói ra đúng những gì màn tạm tắt. */
export const DRILL_DOWN_NOTE_CITIZEN_REPORTS =
  "Bộ lọc, ô tìm và phạm vi tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

export function SoPhanAnh({
  drillDown = NO_DRILL_DOWN,
  reloadSignal = 0,
  openCode = null,
}: {
  /** Lọc mở từ trang Tổng quan, đọc ở máy chủ (`app/phan-anh/page.tsx`). */
  drillDown?: DrillDown<"citizen-reports">;
  /** Bumped by the page header after `+ Nhập hộ phản ánh` booked a petition: the register re-reads. */
  reloadSignal?: number;
  /** `?id=<mã tra cứu>`: open that petition's drawer once, through the detail route. */
  openCode?: string | null;
} = {}) {
  // LỌC TỔNG QUAN ĐANG BẬT THÌ NÓ LÀ BỘ LỌC DUY NHẤT: hàng lọc §4 không vẽ, nên không lối nào ghép
  // thêm một ô vào lát cắt của con số. Màn được dựng lại khi lọc đổi (`key` ở trang), nên giá trị
  // đầu của state là đủ.
  const drillDownActive = drillDown.kind === "active";
  const [loc, datLoc] = useState<BoLoc>(() =>
    drillDownActive ? drillDownQuery(drillDown) : KHONG_LOC,
  );
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  // Cán bộ đã tự đổi bộ lọc: câu "đường dẫn lọc không hợp lệ — đang hiện toàn bộ" hết đúng.
  const [filtersChanged, setFiltersChanged] = useState(false);

  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_phieuPhanAnhRa>;
  } | null>(null);
  const [daTaiBoPhan, datDaTaiBoPhan] = useState<readonly identity_boPhanRa[]>([]);
  const [daTaiThon, datDaTaiThon] = useState<readonly identity_thonToDanPhoRa[]>([]);
  // `null` = chưa tải xong. Giữ NGUYÊN `KetQua` chứ không chỉ `items`: ô chọn cán bộ cần nói ra câu
  // lỗi của máy chủ, còn ô `Đang giao cho` cần biết "chưa có danh bạ" khác "không có trong danh bạ".
  const [daTaiDanhBa, datDaTaiDanhBa] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  const [dangMo, datDangMo] = useState<petitions_phieuPhanAnhRa | null>(null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);
  // The commune's 409 `after_photo_required` sentence — shown IN the close block, not the drawer's
  // general error line, because what it asks for (a verification photo) is one card above it.
  const [closeRefusal, setCloseRefusal] = useState<string | null>(null);
  // Any refusal of `Chuyển xử lý` — shown IN the assign block next to its button (tester report
  // 05/10/2026, PA-03): in the general line at the top of the card it read as a button that did nothing.
  const [assignRefusal, setAssignRefusal] = useState<string | null>(null);
  // The `?id=` deep link's refusal (404 for a code that is not this commune's, 403), verbatim.
  const [deepLinkRefusal, setDeepLinkRefusal] = useState<string | null>(null);

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}|${reloadSignal}`;

  // `?id=<mã tra cứu>` (prototype `app/phan-anh/page.tsx`, `initialOpenId`): read THAT petition through
  // the detail route — never looked up in the page in view, which may not hold it — and open its drawer.
  // Once per mount; the server answers 404 for a code of another commune or a restricted field.
  useEffect(() => {
    if (openCode === null) return;
    let dropped = false;
    layPhieuPhanAnh(openCode).then((kq) => {
      if (dropped) return;
      if (kq.ok) {
        datDangMo(kq.duLieu);
        setDeepLinkRefusal(null);
      } else {
        setDeepLinkRefusal(kq.thongBao);
      }
    });
    return () => {
      dropped = true;
    };
  }, [openCode]);

  // SEARCH AS YOU TYPE, 300ms after the last key (prototype `FeedbackWorkspace.tsx:147-154`). Same
  // policy as before for WHERE the words go: only into the list request, never the address bar or
  // storage — the debounce only sends the same string sooner than Enter did (rule 3, forbidden #4).
  useEffect(() => {
    const next = tim.trim() === "" ? undefined : tim.trim();
    if (next === loc.tim) return;
    const timer = setTimeout(() => {
      datLoc({ ...loc, tim: next });
      datNganXep(TRANG_DAU);
      setFiltersChanged(true);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [tim, loc]);

  useEffect(() => {
    let bo = false;
    laySoPhanAnh({ ...loc, limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, nganXep.hienTai, khoa]);

  // BA DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN. Cả ba tuyến là `any-authenticated`, nên mọi tài khoản
  // xem được sổ đều đọc được. Danh mục hỏng thì ô tương ứng rỗng — KHÔNG làm hỏng quyển sổ: ba câu
  // trả lời rời nhau, mỗi cái nói chuyện của nó.
  //
  // DANH BẠ CHỌN NGƯỜI ĐỌC CẢ XÃ, KHÔNG THEO BỘ PHẬN: cùng một danh sách vừa tra họ tên người đang
  // giữ phiếu (người ấy có thể ở bất kỳ bộ phận nào), vừa làm ô chọn cán bộ — lọc theo bộ phận ở
  // trình duyệt, đúng phép so khớp `department_id` mà `?unit=` của máy chủ làm
  // (`service-identity/internal/store/can_bo_chon_nguoi.go:77-79`). Một lần gọi, không hai bản.
  useEffect(() => {
    let bo = false;
    layDanhMucBoPhan().then((kq) => {
      if (!bo && kq.ok) datDaTaiBoPhan(kq.duLieu.items);
    });
    layDanhSachThonToDanPho().then((kq) => {
      if (!bo && kq.ok) datDaTaiThon(kq.duLieu.items);
    });
    layDanhBaChonNguoi().then((kq) => {
      if (!bo) datDaTaiDanhBa(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào — "chưa rõ" không được
  // hành xử như "có" (luật 1, cấm #1).
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const cong = congThaoTac(
    coQuyen(dsQuyen, QUYEN_PHAN_LOAI_PHAN_ANH),
    coQuyen(dsQuyen, QUYEN_PHAN_CONG_PHAN_ANH),
    coQuyen(dsQuyen, QUYEN_DONG_PHAN_ANH),
  );

  const so = taiTu(daTai, khoa);
  const tenBoPhan = new Map(daTaiBoPhan.map((b) => [b.id, b.name]));

  /** Đổi bộ lọc là về trang đầu: con trỏ của bộ lọc cũ không có nghĩa với bộ lọc mới. */
  function datLocMoi(moi: BoLoc): void {
    datLoc(moi);
    datNganXep(TRANG_DAU);
    setFiltersChanged(true);
  }

  /** Một lần ghi xong: giữ phiếu máy chủ vừa trả, xoá lỗi cũ, và đọc lại quyển sổ. */
  function xongGhi(kq: KetQua<petitions_phieuPhanAnhRa>): void {
    datDangGui(false);
    // Another act answered: an earlier assignment refusal no longer describes the petition on screen.
    setAssignRefusal(null);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ. 409 của các tuyến này mang đúng quy tắc đã từ chối ("phiếu đã
      // chuyển trạng thái trong lúc bạn đang mở màn hình", "xã chưa cấu hình thời hạn xử lý cho
      // lĩnh vực này"), và viết lại nó ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    datDangMo(kq.duLieu);
    datLanTai((n) => n + 1);
  }

  /**
   * Trả `true` khi máy chủ nhận — để biểu mẫu xoá ô ghi chú nội bộ chỉ SAU một lần thành công. A success
   * says so in a toast (ADR 0068 lần 6 #4); a refusal stays INLINE, in the drawer's error line.
   */
  function chay(goi: Promise<KetQua<petitions_phieuPhanAnhRa>>, thanhCong?: string): Promise<boolean> {
    datDangGui(true);
    return goi.then((kq) => {
      xongGhi(kq);
      if (kq.ok && thanhCong !== undefined) toast.success(thanhCong);
      return kq.ok;
    });
  }

  // PRESENTATION ONLY (spec v2 §8b) — nothing below reads the network or changes what is sent.
  // The previous answer, while a new key loads: shown dimmed and inert, never acted on.
  const staleItems =
    so.pha === "dangTai" && daTai !== null && daTai.kq.ok && daTai.kq.duLieu.items.length > 0
      ? daTai.kq.duLieu.items
      : null;
  // "Nothing to show at all" vs "nothing under these filters": no filter key set, no drill-down,
  // first page. Only then is the filter sentence `SO_RONG` the wrong one.
  const noFilter =
    !drillDownActive && !coTrangTruoc(nganXep) && Object.values(loc).every((v) => v === undefined);
  const openPetition = (p: petitions_phieuPhanAnhRa) => {
    datDangMo(p);
    datLoiGhi(null);
    setCloseRefusal(null);
    setAssignRefusal(null);
  };
  const listCards = (items: readonly petitions_phieuPhanAnhRa[]) => (
    <DanhSachThe
      phieu={items}
      bayGio={new Date()}
      maDangMo={dangMo?.code ?? null}
      moPhieu={openPetition}
      empty={noFilter ? <ListEmpty title={LIST_EMPTY_TITLE} hint={LIST_EMPTY_HINT} /> : undefined}
    />
  );

  return (
    // The prototype's composition (`FeedbackWorkspace.tsx:91-268`): KPI cards → the three tabs → in
    // the list tab, ONE filter row over the cards. Direct children lose their legacy vertical margins:
    // the section's `gap` is the one spacing between blocks.
    <section
      className="man-phan-anh mt-0 flex min-w-0 flex-col gap-5 [&>*]:my-0"
      aria-labelledby="tieu-de-so-phan-anh"
    >
      {/* Kept for the section's accessible name; the page's visible title is the `<h1>` above. */}
      <h2 id="tieu-de-so-phan-anh" className="an-thi-giac">
        Sổ phản ánh của xã
      </h2>

      {/* §3 — the four KPI cards. The route checks `feedback.read` AND `report.read`; without the
          second the row is not drawn at all (an account that may not read reports gets no figures). */}
      {coQuyen(dsQuyen, QUYEN_XEM_PHAN_ANH) && coQuyen(dsQuyen, REPORT_READ_PERMISSION) && <PetitionKpis />}

      {/* Owner decision D3 (09/10/2026) — the prototype's note (`FeedbackWorkspace.tsx:125-131`), first
          sentence only, for an account WITHOUT `feedback.restricted`. It states a routing rule, never a
          count: nothing here says whether any such petition exists (rule 4, forbidden #2). */}
      {!coQuyen(dsQuyen, RESTRICTED_FEEDBACK_PERMISSION) && (
        <p className="flex items-start gap-2 rounded-[10px] border border-solid border-line bg-canvas px-3.5 py-2.5 text-[12px] text-ink-muted">
          <Glyph icon={ShieldAlert} className="mt-0.5 size-4 shrink-0" />
          {RESTRICTED_FLOW_NOTE}
        </p>
      )}

      {deepLinkRefusal !== null && (
        <p className="thong-bao-loi" role="alert">
          {deepLinkRefusal}
        </p>
      )}

      <div className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
        {/* §2 main tabs `[Danh sách] [Bản đồ nhiệt] [Báo cáo]`, no icons (`FeedbackWorkspace.tsx:134-138`).
            Only the list exists; the other two are disabled placeholders with their "?" (ADR 0068 §14,
            owner decision D2). NO COUNT on `Danh sách` (owner decision D5): the register is paged by the
            server and returns no total, so a count would be one page passed off as the commune's. */}
        <TabList aria-label="Phần của sổ phản ánh">
          <Tab selected id="petition-tab-list" aria-controls="petition-list-panel">
            Danh sách
          </Tab>
          <PendingTab info={petitionPendingPart("heatMapTab")} />
          <PendingTab info={petitionPendingPart("reportTab")} />
        </TabList>

        <div
          className="flex min-w-0 flex-col gap-4 [&>*]:my-0"
          id="petition-list-panel"
          role="tabpanel"
          aria-labelledby="petition-tab-list"
        >
          <DrillDownBanner
            drillDown={drillDown}
            clearHref="/phan-anh"
            note={DRILL_DOWN_NOTE_CITIZEN_REPORTS}
            showInvalid={!filtersChanged}
          />

          {!drillDownActive && (
            <HangLoc
              loc={loc}
              tim={tim}
              datTim={datTim}
              datLoc={datLocMoi}
              boPhan={daTaiBoPhan}
              thon={daTaiThon}
            />
          )}

          {/* LOADING. The sentence stays the live region; the eye gets a 2px bar and either the
              PREVIOUS page dimmed (a re-read: `daTai` still holds the last answer while the new key
              loads) or card-shaped placeholders (the first read). Dimmed cards are `inert`. */}
          {so.pha === "dangTai" && (
            <>
              <LoadingBar />
              <p className="an-thi-giac" role="status">
                {DANG_TAI_SO}
              </p>
              {staleItems !== null ? (
                <div inert className="pointer-events-none opacity-60 transition-opacity">
                  {listCards(staleItems)}
                </div>
              ) : (
                <PetitionCardsSkeleton />
              )}
            </>
          )}

        {/* LOAD ERROR: the server's sentence VERBATIM stays the alert; `Tải lại` asks the same read
            again through the screen's existing re-read key (`lanTai`). */}
        {so.pha === "loi" && (
          <div className="rounded-card border border-solid border-line bg-surface">
            <EmptyState
              icon={CloudOff}
              title="Chưa tải được sổ phản ánh"
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
          </div>
        )}

        {so.pha === "xong" && (
          <>
            {listCards(so.duLieu.items)}
            {/* Paging is ours, not the prototype's (it loads every row): the server pages by cursor. */}
            <nav className="dieu-huong-trang m-0 flex justify-end" aria-label="Phân trang sổ phản ánh">
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
          </>
        )}
        </div>
      </div>

      {dangMo !== null && (
        <ChiTietPhieu
          // KHOÁ THEO MÃ PHIẾU: mở phiếu khác là dựng lại khối từ đầu. Không có khoá thì lý do vừa gõ
          // cho phiếu A vẫn nằm trong ô khi cán bộ mở phiếu B — và một lần bấm gửi nó cho phiếu B.
          key={dangMo.code}
          phieu={dangMo}
          bayGio={new Date()}
          cong={cong}
          tenBoPhan={tenBoPhan}
          boPhan={daTaiBoPhan}
          danhBa={daTaiDanhBa}
          dangGui={dangGui}
          loiGhi={loiGhi}
          // TIỆN DỤNG, KHÔNG PHẢI CỔNG: mọi người vào được màn này đã có `feedback.read`; máy chủ
          // quyết ai ghi được nhật ký, và câu 403 của nó ra nguyên văn.
          coGhiNhatKy={coQuyen(dsQuyen, QUYEN_XEM_PHAN_ANH)}
          // Mỗi thao tác thành công là một dòng máy chủ vừa ghi vào nhật ký: đọc lại.
          lanLamMoiNhatKy={lanTai}
          permissions={dsQuyen}
          // A task booked from the petition wrote a `tao-nhiem-vu` row: bump the same counter so the
          // log re-reads it. The petition itself is unchanged by the act, so `dangMo` stays.
          onTaskCreated={() => datLanTai((n) => n + 1)}
          dong={() => {
            datDangMo(null);
            datLoiGhi(null);
            setCloseRefusal(null);
            setAssignRefusal(null);
          }}
          phanLoai={(linhVuc, ghiChu) =>
            chay(phanLoaiPhieu(dangMo.code, linhVuc, ghiChu), movedToast("dang-phan-loai"))
          }
          chuyenXuLy={(boPhanID, maCanBo, ghiChu) => {
            datDangGui(true);
            // From `dang-phan-loai` the assignment IS the move to `da-chuyen-xu-ly` (SauKhiPhanCong);
            // later it keeps the status. The toast says which, from the status that was open.
            const fromScreening = dangMo.status === "dang-phan-loai";
            return chuyenXuLyPhieu(dangMo.code, boPhanID, maCanBo, ghiChu).then((kq) => {
              if (!kq.ok) {
                // The server's sentence VERBATIM, but drawn in the assign block, not the general line.
                datDangGui(false);
                datLoiGhi(null);
                setAssignRefusal(kq.thongBao);
                return false;
              }
              xongGhi(kq);
              toast.success(fromScreening ? movedToast("da-chuyen-xu-ly") : ASSIGNED_TOAST);
              return true;
            });
          }}
          assignRefusal={assignRefusal}
          tienTrangThai={(ghiChu) => {
            // The server holds the map (`tienTrinhChinh`); the toast names the status it answered with.
            datDangGui(true);
            return tienTrangThaiPhieu(dangMo.code, ghiChu).then((kq) => {
              xongGhi(kq);
              if (kq.ok) toast.success(movedToast(kq.duLieu.status));
              return kq.ok;
            });
          }}
          closeRefusal={closeRefusal}
          dongPhieuLai={(ketQua, ghiChu) => {
            datDangGui(true);
            return dongPhieu(dangMo.code, ketQua, ghiChu).then((kq) => {
              if (!kq.ok && kq.afterPhotoRequired) {
                // The commune requires a verification photo: say it in the close block, next to the
                // way out, in the commune's own words. The petition did not change — nothing to re-read.
                datDangGui(false);
                datLoiGhi(null);
                setCloseRefusal(kq.thongBao);
                return false;
              }
              setCloseRefusal(null);
              xongGhi(kq);
              if (kq.ok) toast.success(movedToast("da-dong"));
              return kq.ok;
            });
          }}
          khongTiepNhan={(lyDo, ghiChu) =>
            chay(khongTiepNhanPhieu(dangMo.code, lyDo, ghiChu), movedToast("khong-tiep-nhan"))
          }
          chuyenCapTren={(lyDo, coQuan, ghiChu) =>
            chay(chuyenCapTrenPhieu(dangMo.code, lyDo, coQuan, ghiChu), movedToast("chuyen-cap-tren"))
          }
          // Same path as the six processing acts: the 200 body replaces the open petition, a refusal
          // (409 `never_public`, 403) goes verbatim to the drawer's error line.
          setPublication={(target) =>
            chay(
              setPetitionPublication(dangMo.code, target),
              target === "cong-khai" ? PUBLISHED_TOAST : HIDDEN_TOAST,
            )
          }
        />
      )}
    </section>
  );
}

/**
 * Bộ lọc §4: hai tab phạm vi và tám ô (thêm `Bị đánh giá thấp` → `rating_max=2`), đúng những tham số máy chủ nhận — không vẽ ô nào không có
 * tuyến đứng sau. Phạm vi nằm CHUNG `BoLoc` với bảy ô, nên đổi tab giữ nguyên các ô đang chọn và
 * về trang đầu như mọi bộ lọc khác; cùng cách giữ trạng thái (state của trang, không lên URL).
 */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  boPhan,
  thon,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  boPhan: readonly identity_boPhanRa[];
  thon: readonly identity_thonToDanPhoRa[];
}) {
  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  // THE PROTOTYPE'S ONE FILTER ROW (`FeedbackWorkspace.tsx:141-226`), in its order — scope · search ·
  // status · field · hamlet — then the two filters the prototype does not have and our route offers
  // (unit holding the petition, intake channel), then its two checkboxes. No "Bộ lọc" panel (ADR 0068
  // §12 replaced for this screen, lần 5 #3). Labels are visually hidden — each select's first option
  // names it, as in the prototype — but every control keeps a real `<label>`.
  return (
    <div id="petition-filters" className="flex min-w-0 flex-wrap items-center gap-2.5 [&>*]:my-0">
      {/* HAI TAB PHẠM VI, CÙNG KHUÔN SỔ NHIỆM VỤ. `mine` KHÔNG mang danh tính nào — máy chủ lấy mã cán
          bộ từ PHIÊN. Tab thứ ba `Liên quan đến tôi` là CHỖ GIỮ vô hiệu có dấu "?" (ADR 0068 §14): máy
          chủ trả 400 cho `scope=related`, nên nó không có `onClick` và không bao giờ đặt `phamVi`.
          The buttons stay native `aria-pressed` toggles, their text the only child (tests read it). */}
      {/* THE PROTOTYPE'S `ScopeFilter` (`components/common/ScopeFilter.tsx:41-67`), local: one bordered
          group, 36px, active = navy. Each choice's hint is its `title`. The third choice is a disabled
          placeholder with its "?" (ADR 0068 §14): the server answers 400 to `scope=related`, so it has no
          `onClick`. Buttons stay native `aria-pressed` toggles, their text the only child (tests read it). */}
      <span className="inline-flex items-center gap-1.5">
        <div
          className="inline-flex overflow-hidden rounded-md border border-solid border-line bg-white"
          role="group"
          aria-label="Phạm vi"
        >
          <button
            type="button"
            title={SCOPE_HINT_ALL}
            className={scopeButtonClass(loc.phamVi !== "mine")}
            aria-pressed={loc.phamVi !== "mine"}
            onClick={() => datLoc({ ...loc, phamVi: undefined })}
          >
            {PHAM_VI_TOAN_XA}
          </button>
          <button
            type="button"
            title={SCOPE_HINT_MINE}
            className={scopeButtonClass(loc.phamVi === "mine")}
            aria-pressed={loc.phamVi === "mine"}
            onClick={() => datLoc({ ...loc, phamVi: "mine" })}
          >
            {PHAM_VI_GIAO_CHO_TOI}
          </button>
          <button
            type="button"
            disabled
            title={SCOPE_HINT_RELATED}
            aria-pressed={false}
            className={cn(scopeButtonClass(false), "cursor-not-allowed opacity-60 hover:bg-white")}
          >
            {SCOPE_RELATED_LABEL}
          </button>
        </div>
        <PendingMarker info={petitionPendingPart("scopeRelated")} side="bottom" />
      </span>

      {/* Search as you type (300ms after the last key, in `SoPhanAnh`); Enter sends at once. The
          prototype's box: 256px, a magnifier inside, no visible label, no button. */}
      <form className="m-0 w-64 max-w-full" onSubmit={timNgay} role="search">
        <Field label="Tìm trong sổ" htmlFor="tim-phan-anh" icon={Search} hideLabel grow="auto" className={FILTER_CONTROL}>
          <input
            id="tim-phan-anh"
            name="tim-phan-anh"
            type="search"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            onChange={(e) => datTim(e.target.value)}
            autoComplete="off"
            // Máy chủ trả 400 khi quá 200 ký tự (`store.TimPhieuToiDa`). Chặn ở ô nhập để cán bộ thấy
            // giới hạn thay vì thấy "không tải được".
            maxLength={200}
          />
        </Field>
      </form>

      <Field label="Trạng thái" htmlFor="loc-trang-thai" kind="select" hideLabel grow="auto" className={FILTER_CONTROL}>
        <select
          id="loc-trang-thai"
          value={loc.trangThai ?? ""}
          onChange={(e) => datLoc({ ...loc, trangThai: e.target.value || undefined })}
        >
          <option value="">{MOI_TRANG_THAI_NHAN}</option>
          {MOI_TRANG_THAI.map((ma) => (
            <option key={ma} value={ma}>
              {nhanTrangThai(ma)}
            </option>
          ))}
        </select>
      </Field>

      <Field label="Lĩnh vực" htmlFor="loc-linh-vuc" kind="select" hideLabel grow="auto" className={FILTER_CONTROL}>
        <select
          id="loc-linh-vuc"
          value={loc.linhVuc ?? ""}
          onChange={(e) => datLoc({ ...loc, linhVuc: e.target.value || undefined })}
        >
          <option value="">{MOI_LINH_VUC_NHAN}</option>
          {LINH_VUC_PHAN_ANH.map((l) => (
            <option key={l.ma} value={l.ma}>
              {l.nhan}
            </option>
          ))}
        </select>
      </Field>

      <Field label="Địa bàn" htmlFor="loc-dia-ban" kind="select" hideLabel grow="auto" className={FILTER_CONTROL}>
        <select
          id="loc-dia-ban"
          value={loc.thonID ?? ""}
          onChange={(e) => datLoc({ ...loc, thonID: e.target.value || undefined })}
        >
          <option value="">{MOI_DIA_BAN_NHAN}</option>
          {/* A FILTER, not a picker: out-of-use units STAY (petitions recorded there must still be
              findable), marked as such (`residentialUnitFilterLabel`). */}
          {thon.map((t) => (
            <option key={t.id} value={t.id}>
              {residentialUnitFilterLabel(t)}
            </option>
          ))}
        </select>
      </Field>

      <Field label="Bộ phận đang giữ" htmlFor="loc-bo-phan" kind="select" hideLabel grow="auto" className={FILTER_CONTROL}>
        <select
          id="loc-bo-phan"
          value={loc.boPhanID ?? ""}
          onChange={(e) => datLoc({ ...loc, boPhanID: e.target.value || undefined })}
        >
          <option value="">{MOI_BO_PHAN_NHAN}</option>
          {boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </Field>

      <Field label="Kênh tiếp nhận" htmlFor="loc-kenh" kind="select" hideLabel grow="auto" className={FILTER_CONTROL}>
        <select
          id="loc-kenh"
          value={loc.kenh ?? ""}
          onChange={(e) => datLoc({ ...loc, kenh: e.target.value || undefined })}
        >
          <option value="">{MOI_KENH_NHAN}</option>
          {MOI_KENH.map((ma) => (
            <option key={ma} value={ma}>
              {nhanKenh(ma)}
            </option>
          ))}
        </select>
      </Field>

      <label htmlFor="loc-tre-han" className={CHECKBOX_LABEL}>
        <input
          id="loc-tre-han"
          type="checkbox"
          className="accent-brand size-3.5"
          checked={loc.chiTreHan === true}
          // Ô bỏ tích thì tham số VẮNG MẶT HẲN, không gửi `late=false` — máy chủ chỉ nhận đúng chuỗi
          // `true` và trả 400 cho mọi giá trị khác.
          onChange={(e) => datLoc({ ...loc, chiTreHan: e.target.checked ? true : undefined })}
        />{" "}
        {CHI_TRE_HAN_NHAN}
      </label>

      <label htmlFor="loc-danh-gia-thap" className={CHECKBOX_LABEL}>
        <input
          id="loc-danh-gia-thap"
          type="checkbox"
          className="accent-brand size-3.5"
          checked={loc.ratingMax === LOW_RATING_MAX}
          // Spec §4 "phiếu 1–2 sao" → `rating_max=2`. Unticked, the parameter is ABSENT — never an empty
          // `rating_max=`, which the server answers with 400.
          onChange={(e) => datLoc({ ...loc, ratingMax: e.target.checked ? LOW_RATING_MAX : undefined })}
        />{" "}
        {LOW_RATING_FILTER_LABEL}
      </label>
    </div>
  );
}

/** A checkbox of the filter row (`FeedbackWorkspace.tsx:200-225`): box, then its words, 12.5px. */
const CHECKBOX_LABEL = "flex items-center gap-2 text-[12.5px] text-ink";

/** How long after the last key the search box filters (prototype: on every key; 300ms debounce). */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * The prototype's filter control (`FeedbackWorkspace.tsx:29-30`, `SELECT_CLASS`): 36px, white, 12.5px.
 * Set on the `Field` wrapper and marked important: `Field`'s own descendant rules carry the same
 * specificity, and a shared component is not changed for one screen.
 */
const FILTER_CONTROL =
  "[&_select]:h-9! [&_select]:bg-white! [&_select]:text-[12.5px]! [&_input]:h-9! [&_input]:text-[12.5px]!";

/** The three hints of the prototype's `ScopeFilter` (`components/common/ScopeFilter.tsx:13-23`), verbatim. */
const SCOPE_HINT_ALL = "Tất cả hồ sơ trong xã";
const SCOPE_HINT_MINE = "Đích danh tôi là người xử lý";
const SCOPE_HINT_RELATED = "Tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ";

/** One segment of the scope group — `ScopeFilter.tsx:56-61`. */
function scopeButtonClass(active: boolean): string {
  return cn(
    "h-9 cursor-pointer border-0 border-r border-solid border-line px-3 [font-family:inherit] text-[12.5px] font-semibold whitespace-nowrap transition-colors last:border-r-0",
    "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500",
    active ? "bg-navy text-white" : "bg-white text-ink hover:bg-canvas",
  );
}

/**
 * The register's empty state — the prototype's box (`FeedbackWorkspace.tsx:235-243`): one bold navy
 * line, one line under it, in a white card with its shadow.
 */
function ListEmpty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="rounded-card border border-solid border-line bg-white p-10 text-center shadow-card [&>p]:m-0">
      <p className="text-[14px] font-semibold text-navy">{title}</p>
      {hint !== undefined && <p className="mt-1.5 text-[12.5px] text-ink-muted">{hint}</p>}
    </div>
  );
}

/**
 * Quyển sổ — the prototype's grid of CARDS (`FeedbackWorkspace.tsx:245-253`): one column, two from
 * 1536px. Tên `DanhSachThe` giữ nguyên vì nó là tên đã xuất.
 *
 * `empty` — what an empty page shows. The screen passes "nothing to show" or leaves the filter
 * sentence `SO_RONG` (spec §8b), a choice it makes from its own filter state; this list never guesses.
 */
export function DanhSachThe({
  phieu,
  bayGio,
  maDangMo,
  moPhieu,
  empty,
}: {
  phieu: readonly petitions_phieuPhanAnhRa[];
  bayGio: Date;
  maDangMo: string | null;
  moPhieu: (p: petitions_phieuPhanAnhRa) => void;
  empty?: ReactNode;
}) {
  if (phieu.length === 0) return empty ?? <ListEmpty title={SO_RONG} />;

  return (
    <ul className="m-0 grid list-none gap-2.5 p-0 2xl:grid-cols-2" aria-label="Danh sách phiếu phản ánh">
      {phieu.map((p) => (
        <li key={p.code} className="min-w-0" aria-current={p.code === maDangMo ? "true" : undefined}>
          <ThePhieu phieu={p} bayGio={bayGio} mo={() => moPhieu(p)} />
        </li>
      ))}
    </ul>
  );
}

/**
 * Một thẻ phiếu — the prototype's `FeedbackCard`: a square tile on the left, then code · field, the
 * content (two lines), the place, the sender, and status · deadline · stars-or-channel at the foot.
 * The whole card is the button that opens the drawer.
 *
 * THE TILE IS A NEUTRAL PLACEHOLDER, NEVER A PHOTO: the list carries no photo reference
 * (`petitions_phieuPhanAnhRa`), and reading `GET …/photos` per card would sign fresh links over a
 * citizen's photographs (rule 3) for twenty petitions nobody opened. The photos are in the drawer.
 *
 * THE SENDER IS THE SERVER'S MASKED PAIR (`cardSenderLabel`) — the prototype prints the phone in full; the
 * list route always masks (rule 3, ADR 0030), and an anonymous petition says so and nothing more.
 *
 * NO HAMLET AND NO "N phiếu trùng": the petition carries neither (`PHAN_CHUA_DUNG` `sceneMap`,
 * `duplicates`). The place line falls back to the address, as the prototype's own fallback does.
 *
 * NO `title` TOOLTIP ON THE CONTENT: it is a citizen's words (rule 3 keeps personal data out of
 * attributes); the full text is in the drawer.
 */
export function ThePhieu({
  phieu,
  bayGio,
  mo,
}: {
  phieu: petitions_phieuPhanAnhRa;
  bayGio: Date;
  /**
   * Accepted and IGNORED: the prototype's card has no "open" ring (row 19-28, 09/10/2026). The list
   * marks the open card with `aria-current` on its `<li>` instead.
   */
  dangMo?: boolean;
  mo: () => void;
}) {
  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  // A comparison of the stored deadline with now (`trangThaiHan`), recomputed on every render —
  // never kept (rule 10, invariant 3).
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);
  const pastDeadline = hanXuLy.loai === "quaHan";
  const rating = phieu.rating ?? null;

  return (
    <button
      type="button"
      onClick={mo}
      aria-haspopup="dialog"
      data-tre-han={pastDeadline ? "" : undefined}
      className={cn(
        "flex w-full min-w-0 cursor-pointer gap-3 overflow-hidden rounded-card border border-solid border-line bg-white p-2.5 text-left [font-family:inherit] text-inherit shadow-card",
        "motion-safe:transition-shadow motion-safe:duration-150 hover:shadow-[0_6px_20px_rgba(16,43,67,0.10)]",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
        // The left rule follows the same comparison as the deadline line, so the two never differ; the
        // AlarmClock and the words say it too — never colour alone.
        pastDeadline && "border-l-4 border-l-danger",
      )}
    >
      <span
        aria-hidden="true"
        className="grid size-20 shrink-0 place-items-center rounded-[8px] bg-[#F7FAFC] text-ink-muted"
      >
        <ImageOff className="size-5 opacity-40" strokeWidth={1.6} focusable="false" />
      </span>

      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex flex-wrap items-center gap-1.5">
          <span className="ma-muc rounded border border-solid border-line bg-[#F7FAFC] px-1.5 py-0.5 text-[10.5px] font-semibold text-ink-muted">
            {phieu.code}
          </span>
          <span className="rounded-full bg-brand/12 px-2 py-0.5 text-[10.5px] font-semibold text-brand">
            {nhanLinhVuc(linhVuc)}
          </span>
        </span>

        {/* Nội dung phản ánh KHÔNG che (cán bộ không đọc được thì không xử lý được), nhưng nó là chữ
            của một công dân: không bao giờ ghi nó vào log, tên tệp hay URL. */}
        <span className="mt-1.5 line-clamp-2 text-[12.8px] leading-snug text-navy">{phieu.content}</span>

        <span className="mt-1 flex min-w-0 items-center gap-1 text-[11px] text-ink-muted">
          <Glyph icon={MapPin} className="size-3 shrink-0" />
          <span className="truncate">{phieu.address === "" ? CARD_NO_LOCATION : phieu.address}</span>
        </span>

        {/* WRAPS instead of squeezing the sender to nothing: at the narrowest width the badge alone is
            wider than what is left of the line beside the tile. */}
        <span className="mt-1 flex min-w-0 flex-wrap items-center gap-x-1 gap-y-0.5 text-[11px] text-ink-muted">
          <Glyph icon={UserRound} className="size-3 shrink-0" />
          <span className="min-w-0 truncate">{cardSenderLabel(phieu)}</span>
          {isContactUnverified(phieu) && <UnverifiedContactBadge />}
        </span>

        <span className="mt-auto flex flex-wrap items-center gap-2 pt-1.5">
          <PetitionStatusBadge status={phieu.status}>{nhanTrangThai(phieu.status)}</PetitionStatusBadge>
          {/* The words are `nhanHan`'s ("Quá hạn · hạn cuối …"); AlarmClock beside them, never instead. */}
          <span className={cn(lopHan(hanXuLy), "inline-flex items-center gap-1 text-[11px]")}>
            {pastDeadline && <Glyph icon={AlarmClock} className="size-3 shrink-0" />}
            {nhanHan(hanXuLy)}
          </span>
          {rating !== null ? (
            // §7 corner: the citizen's stars once rated — danger at 1–2, tangerine otherwise (`FeedbackCard`).
            <span
              className={cn("ml-auto text-[11.5px]", rating <= LOW_RATING_MAX ? "text-danger" : "text-tangerine")}
              role="img"
              aria-label={starsLabel(rating)}
            >
              {ratingStars(rating)}
            </span>
          ) : (
            <span className="ml-auto inline-flex items-center gap-1 text-[10.5px] text-ink-muted">
              <Glyph icon={MessageSquare} className="size-3 shrink-0" />
              {nhanKenh(phieu.channel)}
            </span>
          )}
        </span>
      </span>
    </button>
  );
}

/**
 * Khối chi tiết §8 — the prototype's right-hand drawer (`FeedbackDetailDrawer.tsx`, ADR 0068 lần 5), in
 * `LargeDialog`. Mounted = open: the register renders it only while a petition is open.
 *
 * THE STATUS STRIP IS WHERE A PETITION MOVES (owner decision D1, 09/10/2026): a chip is pressable only
 * for a move our server performs for staff, and pressing it opens THE EXISTING ACT for that move —
 * classify, assign, reject, refer, advance, close — in the composer under the strip (`StatusStrip`).
 * Every act keeps its own route and its own key (`statusStrip`, `congThaoTac`).
 */
export function ChiTietPhieu({
  phieu,
  bayGio,
  cong,
  tenBoPhan,
  boPhan,
  danhBa,
  dangGui,
  loiGhi,
  coGhiNhatKy = false,
  lanLamMoiNhatKy = 0,
  permissions = [],
  onTaskCreated,
  dong,
  phanLoai,
  chuyenXuLy,
  tienTrangThai,
  dongPhieuLai,
  khongTiepNhan,
  chuyenCapTren,
  setPublication,
  closeRefusal = null,
  assignRefusal = null,
  initialStep = null,
}: {
  phieu: petitions_phieuPhanAnhRa;
  bayGio: Date;
  cong: CongThaoTac;
  tenBoPhan: ReadonlyMap<string, string>;
  boPhan: readonly identity_boPhanRa[];
  /** Danh bạ chọn người của xã. `null` = chưa tải xong. */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  dangGui: boolean;
  loiGhi: string | null;
  /** Vẽ ô `Ghi nhật ký`. Mặc định KHÔNG — chưa rõ quyền thì không hành xử như có. */
  coGhiNhatKy?: boolean;
  /** Tăng sau mỗi thao tác thành công, để nhật ký đọc lại dòng máy chủ vừa ghi. */
  lanLamMoiNhatKy?: number;
  /**
   * The session's permission keys — ONLY to decide whether `Tạo nhiệm vụ` and the photos are drawn.
   * Empty by default: an unknown session holds no key (rule 1, forbidden #1).
   */
  permissions?: readonly string[];
  /** Called after POST …/tasks answered 201. Absent = the block is not drawn (no write path). */
  onTaskCreated?: () => void;
  dong: () => void;
  /*
   * `ghiChu` ở cả sáu thao tác là `Nội dung cập nhật` TUỲ CHỌN, vào nhật ký, không gửi người dân. Trả
   * `Promise<boolean>` (máy chủ nhận hay không) để ô ấy chỉ được xoá SAU một lần thành công;
   * `void` vẫn được nhận — khi ấy ô giữ nguyên chữ.
   */
  phanLoai: (linhVuc: string, ghiChu: string) => KetQuaGui;
  /** `maCanBo` là MÃ CÁN BỘ (`code` của danh bạ), vắng khi để bộ phận tự phân công. */
  chuyenXuLy: (boPhanID: string, maCanBo: string | undefined, ghiChu: string) => KetQuaGui;
  tienTrangThai: (ghiChu: string) => KetQuaGui;
  dongPhieuLai: (ketQua: string, ghiChu: string) => KetQuaGui;
  /**
   * The commune's refusal to close without a verification photo (409 `after_photo_required`), verbatim,
   * or `null`. Drawn inside the close act with the way to the `Sau khi xử lý` upload.
   */
  closeRefusal?: string | null;
  /** Any refusal of `Chuyển xử lý`, verbatim, or `null`. Drawn inside the assign form, by its button. */
  assignRefusal?: string | null;
  khongTiepNhan: (lyDo: string, ghiChu: string) => KetQuaGui;
  chuyenCapTren: (lyDo: string, coQuanTiepNhan: string, ghiChu: string) => KetQuaGui;
  /**
   * PUT …/publication. Optional so a caller without a write path renders the box read-only; the
   * buttons also need `cong.moderate`.
   */
  setPublication?: (target: PublicationTarget) => KetQuaGui;
  /**
   * The strip chip whose composer starts OPEN — tests only (a static render cannot press a chip). It
   * opens only if that chip is pressable for this petition and account.
   */
  initialStep?: string | null;
}) {
  // Seeded with the field already chosen at intake (PA-03) — a lazy initialiser, so the hook order the
  // `chon-can-bo.test.tsx` seeding relies on is unchanged.
  const [linhVucChon, datLinhVucChon] = useState(() => initialClassifyField(phieu.field));
  const [boPhanChon, datBoPhanChon] = useState("");
  const [canBoChon, datCanBoChon] = useState("");
  const [ketQua, datKetQua] = useState("");
  // SAU bốn hook trên, không trước: `chon-can-bo.test.tsx` gieo giá trị theo THỨ TỰ gọi hook. Bốn ô
  // `Nội dung cập nhật`, MỖI THAO TÁC MỘT Ô: một ô chung sẽ mang chữ viết cho lần chuyển xử lý sang lần
  // đóng phiếu. The open chip lives in `StatusStrip` (a child), never here.
  const [ghiChuPhanLoai, datGhiChuPhanLoai] = useState("");
  const [ghiChuPhanCong, datGhiChuPhanCong] = useState("");
  const [ghiChuTien, datGhiChuTien] = useState("");
  const [ghiChuDong, datGhiChuDong] = useState("");

  const bangDanhBa = danhBa !== null && danhBa.ok ? danhBaTheoMa(danhBa.duLieu.items) : null;
  // Chỉ người thuộc ĐÚNG bộ phận đang chọn — cùng phép so khớp `?unit=` của máy chủ. Người chưa
  // thuộc bộ phận nào không nằm trong ô chọn của bộ phận nào.
  const canBoCuaBoPhan =
    danhBa !== null && danhBa.ok && boPhanChon !== ""
      ? danhBa.duLieu.items.filter((cb) => cb.department_id === boPhanChon)
      : [];

  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  const hanTiepNhan = trangThaiHan(phieu.acknowledge_due, "khongApDung", bayGio);
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);
  const hanPhanLoai = trangThaiHan(phieu.classify_due, "khongApDung", bayGio);
  const reopened = reopenLine(phieu.reopen_count);
  const strip = statusStrip(phieu, cong);
  const coordinates = sceneCoordinates(phieu);
  const rating = ratingView(phieu);
  // `Chuyển xử lý, không đổi trạng thái` only where an assignment KEEPS the status
  // (`domain.SauKhiPhanCong`); from `dang-phan-loai` the same act is the strip's move to
  // `da-chuyen-xu-ly`, and the composer holds it.
  const handOverHere = cong.phanCong && ASSIGN_KEEPS_STATUS.includes(phieu.status);

  function submitAssign(after: () => void): void {
    if (boPhanChon !== "" && loiGhiChuNoiBo(ghiChuPhanCong) === null) {
      xoaKhiThanhCong(chuyenXuLy(boPhanChon, canBoChon === "" ? undefined : canBoChon, ghiChuPhanCong), () => {
        datGhiChuPhanCong("");
        after();
      });
    }
  }

  // The unit and officer pickers — the prototype's `HandoverFields`. ONE copy: the composer (first
  // routing, `dang-phan-loai`) and the hand-over section (later) are never drawn at the same time.
  const assignFields = (
    <>
      <div className={SELECT_GRID}>
        <Field label="Bộ phận" htmlFor="chon-bo-phan" icon={Building2} kind="select" grow="auto">
          <select
            id="chon-bo-phan"
            value={boPhanChon}
            onChange={(e) => {
              datBoPhanChon(e.target.value);
              // Đổi bộ phận thì bỏ người đã chọn: người ấy thuộc bộ phận cũ, và một lựa chọn không còn
              // nằm trong ô chọn là một lựa chọn cán bộ không nhìn thấy mà vẫn gửi đi.
              datCanBoChon("");
            }}
          >
            <option value="">— Chọn bộ phận —</option>
            {boPhan.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </Field>
        {/* GIÁ TRỊ CỦA MỖI LỰA CHỌN LÀ MÃ CÁN BỘ (`code`), không phải id nội bộ — luật nắm giữ ở máy chủ
            so đúng mã ấy với phiên của người được giao. Mã chỉ nằm trong thân POST, không lên URL. The
            unit comes first: the route needs a unit, the officer is optional. */}
        <Field label={NHAN_CHON_CAN_BO} htmlFor="chon-can-bo" icon={UserRound} kind="select" grow="auto">
          <select
            id="chon-can-bo"
            value={canBoChon}
            disabled={boPhanChon === ""}
            onChange={(e) => datCanBoChon(e.target.value)}
          >
            <option value="">{DE_BO_PHAN_PHAN_CONG}</option>
            {canBoCuaBoPhan.map((cb) => (
              <option key={cb.code} value={cb.code}>
                {nhanLuaChonCanBo(cb)}
              </option>
            ))}
          </select>
        </Field>
      </div>
      {danhBa !== null && !danhBa.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          Không tải được danh bạ cán bộ: {danhBa.thongBao} Vẫn chuyển được phiếu cho bộ phận tự phân công.
        </p>
      )}
      {danhBa !== null && danhBa.ok && boPhanChon !== "" && canBoCuaBoPhan.length === 0 && (
        <p className="m-0 text-[11.5px] text-tangerine">{UNIT_WITHOUT_STAFF}</p>
      )}
    </>
  );

  const assignRefusalBox =
    assignRefusal !== null ? (
      <div className="flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
        <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
        <p className="thong-bao-loi" role="alert">
          {assignRefusal}
        </p>
      </div>
    ) : null;

  /** The form of one move, in the composer. `close` folds the composer (Huỷ, or after a success). */
  function renderAct(step: StripStep, close: () => void): ReactNode {
    switch (step.act) {
      case "classify":
        return (
          <form
            className={COMPOSER_FORM}
            onSubmit={(e) => {
              e.preventDefault();
              if (linhVucChon !== "" && loiGhiChuNoiBo(ghiChuPhanLoai) === null) {
                xoaKhiThanhCong(phanLoai(linhVucChon, ghiChuPhanLoai), () => datGhiChuPhanLoai(""));
              }
            }}
          >
            <div className={SELECT_GRID}>
              <Field label="Lĩnh vực" htmlFor="chon-linh-vuc" icon={Shapes} kind="select" grow="auto">
                <select id="chon-linh-vuc" value={linhVucChon} onChange={(e) => datLinhVucChon(e.target.value)}>
                  <option value="">— Chọn lĩnh vực —</option>
                  {LINH_VUC_PHAN_ANH.map((l) => (
                    <option key={l.ma} value={l.ma}>
                      {l.nhan}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <ONhapGhiChuNoiBo id="ghi-chu-phan-loai" giaTri={ghiChuPhanLoai} datGiaTri={datGhiChuPhanLoai} />
            <ComposerButtons
              cancel={close}
              busy={dangGui}
              disabled={dangGui || linhVucChon === "" || loiGhiChuNoiBo(ghiChuPhanLoai) !== null}
            />
          </form>
        );
      case "assign":
        return (
          <form
            className={COMPOSER_FORM}
            onSubmit={(e) => {
              e.preventDefault();
              submitAssign(close);
            }}
          >
            {assignFields}
            <ONhapGhiChuNoiBo id="ghi-chu-phan-cong" giaTri={ghiChuPhanCong} datGiaTri={datGhiChuPhanCong} />
            {assignRefusalBox}
            <ComposerButtons
              cancel={close}
              busy={dangGui}
              disabled={dangGui || boPhanChon === "" || loiGhiChuNoiBo(ghiChuPhanCong) !== null}
            />
          </form>
        );
      case "reject":
      case "refer":
        return (
          <BieuMauReNhanh
            // Khoá theo nhánh: đổi nhánh là ô trống lại, không mang lý do của nhánh kia sang.
            key={step.act}
            loai={step.act === "reject" ? "khong-tiep-nhan" : "chuyen-cap-tren"}
            dangGui={dangGui}
            gui={(lyDo, coQuan, ghiChu) =>
              step.act === "reject" ? khongTiepNhan(lyDo, ghiChu) : chuyenCapTren(lyDo, coQuan, ghiChu)
            }
            huy={close}
          />
        );
      case "advance":
        // NO UI GATE: the holder rule is the server's, and its 403 sentence reaches the error line.
        return (
          <form
            className={COMPOSER_FORM}
            onSubmit={(e) => {
              e.preventDefault();
              if (loiGhiChuNoiBo(ghiChuTien) === null) {
                xoaKhiThanhCong(tienTrangThai(ghiChuTien), () => datGhiChuTien(""));
              }
            }}
          >
            <ONhapGhiChuNoiBo id="ghi-chu-tien" giaTri={ghiChuTien} datGiaTri={datGhiChuTien} />
            <ComposerButtons cancel={close} busy={dangGui} disabled={dangGui || loiGhiChuNoiBo(ghiChuTien) !== null} />
          </form>
        );
      case "close":
        // `feedback.resolve` ONLY — never widened by the holder rule (open question #7, 16/09/2026).
        return (
          <form
            className={COMPOSER_FORM}
            onSubmit={(e) => {
              e.preventDefault();
              if (ketQua.trim() !== "" && loiGhiChuNoiBo(ghiChuDong) === null) {
                xoaKhiThanhCong(dongPhieuLai(ketQua.trim(), ghiChuDong), () => datGhiChuDong(""));
              }
            }}
          >
            <div>
              <label htmlFor="ket-qua-xu-ly" className={COMPOSER_LABEL}>
                {NHAN_O_KET_QUA}
              </label>
              <textarea
                id="ket-qua-xu-ly"
                name="ket-qua-xu-ly"
                rows={2}
                className={cn(TEXTAREA_CLASS, "bg-white text-[12.5px] md:text-[12.5px]")}
                value={ketQua}
                placeholder={RESULT_PLACEHOLDER}
                onChange={(e) => datKetQua(e.target.value)}
              />
              <p className={HINT_CLASS}>{GHI_CHU_O_KET_QUA}</p>
            </div>
            <ONhapGhiChuNoiBo id="ghi-chu-dong" giaTri={ghiChuDong} datGiaTri={datGhiChuDong} />
            {/* 409 `after_photo_required`: the commune's sentence VERBATIM, then where to go. */}
            {closeRefusal !== null && (
              <div className="flex flex-col gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
                <p className="thong-bao-loi inline-flex items-start gap-2" role="alert">
                  <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
                  <span>{closeRefusal}</span>
                </p>
                <p className="text-sm text-ink-700">{AFTER_PHOTO_REQUIRED_HINT}</p>
                <p>
                  <a className={buttonClass("secondary", "sm")} href={`#${afterPhotosHeadingId(phieu.code)}`}>
                    <Glyph icon={ArrowUp} />
                    {AFTER_PHOTO_GO_TO}
                  </a>
                </p>
              </div>
            )}
            <ComposerButtons
              cancel={close}
              busy={dangGui}
              disabled={dangGui || ketQua.trim() === "" || loiGhiChuNoiBo(ghiChuDong) !== null}
            />
          </form>
        );
      default:
        return null;
    }
  }

  return (
    // THE PROTOTYPE'S RIGHT-HAND DRAWER (`FeedbackDetailDrawer.tsx:69-110`): 72rem wide (never past 98vw),
    // a navy/30 veil, the left-cast shadow — set on the shared `LargeDialog` by className only.
    //
    //   header   code · channel · booked at (the dialog's NAME), the field as the title, the sender
    //   strip    the lifecycle's steps, the branches, the current status's sentence, the composer
    //   facts    three cells: Hạn xử lý (both clocks) · Đang giao cho · Hiển thị với người dân
    //   body     left: content (+ result / branch reason), photos, location, duplicates "?", rating,
    //            hand-over, task; right (from 768px; under the left column below): the log
    //
    // THE DIALOG'S NAME IS THE CODE, CHANNEL AND TIME, NEVER THE CONTENT: a name goes into the
    // accessibility tree, and the content is a citizen's words (rule 3, forbidden #4).
    <LargeDialog
      titleId="tieu-de-chi-tiet-phieu"
      onDismiss={dong}
      className="max-w-[98vw] backdrop:bg-navy/30 md:w-[72rem] md:shadow-[-10px_0_36px_rgba(16,43,67,0.18)] xl:w-[72rem]"
    >
      <header className="flex shrink-0 items-start gap-3 border-b border-solid border-line bg-white px-5 py-4">
        <div className="min-w-0 flex-1 [&>p]:m-0">
          <p
            id="tieu-de-chi-tiet-phieu"
            tabIndex={-1}
            className="text-[11px] leading-snug font-semibold text-ink-muted tabular-nums outline-none"
          >
            <span className="ma-muc">{phieu.code}</span> · {nhanKenh(phieu.channel)} · {dateTimeLabel(phieu.booked_at)}
          </p>
          <h2 className="m-0 mt-0.5 text-[15px] leading-snug font-bold text-navy">{nhanLinhVuc(linhVuc)}</h2>
          <p className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[11.5px] text-ink-muted">
            <span>{nhanNguoiGui(phieu)}</span>
            {isContactUnverified(phieu) && <UnverifiedContactBadge />}
          </p>
          {isContactUnverified(phieu) && <p className="mt-1 text-[11px] text-ink-muted">{UNVERIFIED_CONTACT_NOTE}</p>}
        </div>
        <IconButton label="Đóng chi tiết phiếu" type="button" variant="secondary" onClick={dong}>
          <Glyph icon={X} />
        </IconButton>
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <StatusStrip status={phieu.status} view={strip} initialStep={initialStep} renderAct={renderAct} />

        {/* THE THREE FACT CELLS (`FeedbackDetailDrawer.tsx:229-310`). The deadline cell holds BOTH
            clocks (ADR 0027/0028) and the classify deadline — each a comparison of a stored instant with
            now, never a stored flag (rule 10, invariant 3); absolute deadlines, never a day count. */}
        <div className="grid shrink-0 grid-cols-1 gap-px border-b border-solid border-line bg-line sm:grid-cols-3">
          <Fact label="Hạn xử lý">
            <dl className={FACT_LIST}>
              <dt>Hạn xử lý xong</dt>
              <dd>
                <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
                {/* Requirement `FeedbackDetailDrawer.tsx:242-246`. The deadline is NOT recomputed on a
                    reopening (ADR 0050 point 2) — this line explains an old deadline on a petition that
                    is back in progress. */}
                {reopened !== null && <p className="nhan-lech">{reopened}</p>}
              </dd>
              <dt>Hạn tiếp nhận</dt>
              <dd>
                <span className={lopHan(hanTiepNhan)}>{nhanHan(hanTiepNhan)}</span>
              </dd>
              {/* `classify_due` — ADR 0035 #26, one working day from intake to classify. */}
              <dt>Hạn phân loại</dt>
              <dd>
                <span className={lopHan(hanPhanLoai)}>{nhanHan(hanPhanLoai)}</span>
              </dd>
              <dt>Người dân gửi lúc</dt>
              <dd className="tabular-nums">{nhanThoiDiem(phieu.clock_from)}</dd>
            </dl>
          </Fact>

          <Fact label="Đang giao cho">
            <p className="m-0 text-[12.5px] text-navy">
              {phieu.unit === "" ? NO_UNIT_YET : nhanBoPhan(phieu.unit, tenBoPhan)}
            </p>
            <p className="m-0 mt-0.5 text-[11px] text-ink-muted">
              {phieu.assignee === "" ? NO_ASSIGNEE_YET : nhanCanBoXuLy(phieu.assignee, bangDanhBa)}
            </p>
          </Fact>

          <Fact label="Hiển thị với người dân">
            <div className="[&>p]:m-0">
              <PublicationBox
                petition={phieu}
                mayModerate={cong.moderate}
                busy={dangGui}
                setPublication={
                  setPublication === undefined ? undefined : (target) => void setPublication(target)
                }
              />
            </div>
          </Fact>
        </div>

        <div className="flex min-w-0 flex-1 flex-col md:flex-row">
          {/* The prototype's left column (`FeedbackDetailDrawer.tsx:313-541`): canvas background, flat
              sections divided by a hairline, 12.5px titles — no cards, no icon titles. */}
          <div className="min-w-0 flex-1 bg-canvas px-5 py-4">
            {/* Right under the facts, so a refusal of the publication buttons above (409 `never_public`)
                or of an act reads next to what caused it. The `<p>` itself is unchanged (a test reads it). */}
            {loiGhi !== null && (
              <div className="mb-4 flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
                <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
                <p className="thong-bao-loi" role="alert">
                  {loiGhi}
                </p>
              </div>
            )}

            {/* `Nội dung phản ánh`, then what the citizen was answered. */}
            <section aria-labelledby="tieu-de-noi-dung-phieu">
              <h3 id="tieu-de-noi-dung-phieu" className={cn(SECTION_HEADING, "mb-1.5")}>
                Nội dung phản ánh
              </h3>
              <p className="noi-dung-phan-anh m-0 rounded-[10px] border border-solid border-line bg-white px-3 py-2.5 text-[13px] text-navy">
                {phieu.content}
              </p>
              {/* Kết quả CHỈ hiện khi đã có: một ô trống ở đây trông như một trường chưa điền. */}
              {phieu.result !== "" && (
                <div className="mt-2 rounded-[10px] border border-solid border-leaf/25 bg-leaf/8 px-3 py-2.5 [&>p]:m-0">
                  <p className="text-[11px] font-semibold text-ink-muted uppercase">Kết quả xử lý đã trả lời người dân</p>
                  <p className="noi-dung-phan-anh mt-0.5 text-[12.5px] text-navy">{phieu.result}</p>
                </div>
              )}
              {/* LÝ DO VÀ CƠ QUAN NHẬN CHỈ CÓ Ở HAI NHÁNH RẼ (`FeedbackDetailDrawer.tsx:336-345`). Ở trạng
                  thái khác hai trường không tồn tại — vẽ ô trống là vẽ một trường trông như chưa điền. */}
              {RE_NHANH.includes(phieu.status) && (
                <div className="mt-2 rounded-[10px] border border-solid border-tangerine/25 bg-tangerine/8 px-3 py-2.5 [&>p]:m-0">
                  <p className="text-[11px] font-semibold text-ink-muted uppercase">{branchReasonLabel(phieu.status)}</p>
                  <p className="noi-dung-phan-anh mt-0.5 text-[12.5px] text-navy">{nhanTruongNhanh(phieu.reason)}</p>
                  <dl className={BRANCH_LIST}>
                    {phieu.status === "chuyen-cap-tren" && (
                      <>
                        <dt>Cơ quan tiếp nhận</dt>
                        <dd>{nhanTruongNhanh(phieu.receiving_body)}</dd>
                      </>
                    )}
                    <dt>{phieu.status === "chuyen-cap-tren" ? "Chuyển lúc" : "Kết thúc lúc"}</dt>
                    <dd>
                      {phieu.branch_ended_at === undefined || phieu.branch_ended_at === null
                        ? KHONG_CO_TRONG_PHAN_HOI
                        : nhanThoiDiem(phieu.branch_ended_at)}
                    </dd>
                  </dl>
                </div>
              )}
            </section>

            {/* §8.4 photos. A separate component, NOT a hook here: it reads the network (`useEffect`),
                and `chon-can-bo.test.tsx` calls this block as a plain function. Hidden without
                `feedback.read` — UX only; the route checks the same key and `feedback.restricted`. */}
            {coQuyen(permissions, QUYEN_XEM_PHAN_ANH) && (
              <ScenePhotos
                lookupCode={phieu.code}
                after={
                  <VerificationPhotos
                    lookupCode={phieu.code}
                    status={phieu.status}
                    canUpload={coQuyen(permissions, QUYEN_DONG_PHAN_ANH)}
                  />
                }
              />
            )}

            {/* `Vị trí` — ONLY WHEN THE CITIZEN SENT COORDINATES (`FeedbackDetailDrawer.tsx:394-406`,
                owner decision D2). The mini-map is a disabled placeholder with its "?" in the map's place:
                it loads NO tile until the self-hosted basemap is built. Under it, the address, and the
                coordinates as text. */}
            {coordinates !== null && (
              <section aria-labelledby="tieu-de-vi-tri-phieu" className={SECTION}>
                <h3 id="tieu-de-vi-tri-phieu" className={SECTION_HEADING}>
                  {LOCATION_SECTION_TITLE}
                </h3>
                <PendingFeature info={petitionPendingPart("sceneMap")} className="flex w-full">
                  <div
                    aria-disabled="true"
                    className="grid h-52 w-full place-items-center rounded-[10px] border border-solid border-line bg-white text-ink-400"
                  >
                    <MapIcon aria-hidden="true" className="size-7" strokeWidth={1.5} focusable="false" />
                  </div>
                </PendingFeature>
                <p className="m-0 mt-1.5 flex items-center gap-1 text-[11.5px] text-ink-muted">
                  <Glyph icon={MapPin} className="size-3 shrink-0" />
                  {phieu.address === "" ? SCENE_NO_ADDRESS : phieu.address}
                </p>
                <p className="m-0 mt-0.5 text-[11px] text-ink-muted">
                  <span className="tabular-nums">{coordinates}</span> · {SCENE_COORDINATES_NOTE}
                </p>
              </section>
            )}

            {/* `Có thể trùng với phiếu khác` (`FeedbackDetailDrawer.tsx:408-439`) — no route detects or
                merges duplicates: the title with its "?", no button that would do nothing. */}
            <section className={SECTION} data-pending="">
              <h3 className={cn(SECTION_HEADING, "flex items-center gap-1.5 text-ink-muted")}>
                {petitionPendingPart("duplicates").ten}
                <PendingMarker info={petitionPendingPart("duplicates")} />
              </h3>
            </section>

            {/* `Đánh giá của người dân` — ONLY WHEN RATED (`FeedbackDetailDrawer.tsx:441-482`). */}
            {rating.kind === "rated" && (
              <section aria-labelledby="tieu-de-danh-gia-phieu" className={SECTION}>
                <h3 id="tieu-de-danh-gia-phieu" className={SECTION_HEADING}>
                  {RATING_TITLE}
                </h3>
                <div
                  className={cn(
                    "rounded-[10px] border border-solid px-3 py-2.5 [&>p]:m-0",
                    rating.low ? "border-danger/25 bg-danger/8" : "border-leaf/25 bg-leaf/8",
                  )}
                >
                  <p className="flex flex-wrap items-center gap-1.5">
                    <span role="img" aria-label={`${rating.score} sao`} className="inline-flex items-center gap-1.5">
                      {Array.from({ length: RATING_MAX_STARS }, (_, i) => (
                        <Star
                          key={i}
                          aria-hidden="true"
                          focusable="false"
                          className={cn("size-4", i < (phieu.rating ?? 0) ? "fill-tangerine text-tangerine" : "text-ink-muted")}
                        />
                      ))}
                    </span>
                    <span className="ml-1 text-[12.5px] font-semibold text-navy">{rating.score}</span>
                    {rating.at !== null && <span className="text-[11px] text-ink-muted">{rating.at}</span>}
                  </p>
                  {rating.comment !== "" && (
                    <p className="noi-dung-phan-anh mt-1.5 text-[12.5px]">“{rating.comment}”</p>
                  )}
                  {rating.low && (
                    <p className="mt-1.5 text-[11.5px] font-semibold text-danger">{LOW_RATING_REOPENED}</p>
                  )}
                </div>
              </section>
            )}

            {/* `Chuyển xử lý, không đổi trạng thái` (`FeedbackDetailDrawer.tsx:492-514`) — moving a
                petition between units while it keeps its status. Behind `feedback.assign` (UX; the server
                checks). Inline here, not a child: `chon-can-bo.test.tsx` drives this form. */}
            {handOverHere && (
              <section aria-labelledby="tieu-de-chuyen-xu-ly-phieu" className={SECTION}>
                <h3 id="tieu-de-chuyen-xu-ly-phieu" className={SECTION_HEADING}>
                  Chuyển xử lý, không đổi trạng thái
                </h3>
                <form
                  className="m-0 flex flex-col gap-3 rounded-[10px] border border-solid border-line bg-white p-3"
                  onSubmit={(e) => {
                    e.preventDefault();
                    submitAssign(() => {});
                  }}
                >
                  {assignFields}
                  <ONhapGhiChuNoiBo id="ghi-chu-phan-cong" giaTri={ghiChuPhanCong} datGiaTri={datGhiChuPhanCong} />
                  {assignRefusalBox}
                  <div>
                    <button
                      type="submit"
                      className={buttonClass("primary", "sm")}
                      disabled={dangGui || boPhanChon === "" || loiGhiChuNoiBo(ghiChuPhanCong) !== null}
                    >
                      Chuyển xử lý
                    </button>
                  </div>
                </form>
              </section>
            )}

            {/* `Tạo nhiệm vụ` (§13, owner decision D5) — `task.create` AND `feedback.read`; UX only. */}
            {onTaskCreated !== undefined && petitionTaskOffered(permissions, phieu) && (
              <div className={SECTION}>
                <PetitionTaskBlock lookupCode={phieu.code} danhBa={danhBa} onCreated={onTaskCreated} />
              </div>
            )}
          </div>

          {/* ── NHẬT KÝ XỬ LÝ (§8.7) — the right column, 26rem, as in the prototype. Under the left column
              below 768px. A component of its own, NOT a hook here: it reads the network (`useEffect`),
              and `chon-can-bo.test.tsx` calls this block as a plain function. */}
          <aside className="shrink-0 border-t border-solid border-line bg-white md:w-[26rem] md:border-t-0 md:border-l">
            <NhatKyPhieu
              maTraCuu={phieu.code}
              tenBoPhan={tenBoPhan}
              danhBa={bangDanhBa}
              coNutGhi={coGhiNhatKy}
              lanLamMoi={lanLamMoiNhatKy}
            />
          </aside>
        </div>
      </div>
    </LargeDialog>
  );
}

/** Statuses where an assignment KEEPS the status — `service-petitions/internal/domain/xu_ly_phan_anh.go:351`. */
const ASSIGN_KEEPS_STATUS: readonly string[] = ["da-chuyen-xu-ly", "dang-xu-ly", "da-xu-ly", "cho-dan-xac-nhan"];

/** A flat drawer section (`FeedbackDetailDrawer.tsx:630-643`, `Section`): a hairline above, 16px in. */
const SECTION = "mt-5 border-t border-solid border-line pt-4";

/** Its title: 12.5px bold navy. */
const SECTION_HEADING = "m-0 mb-2.5 text-[12.5px] font-bold text-navy";

/** One act's form inside the composer. */
const COMPOSER_FORM = "m-0 flex flex-col gap-2.5";

/** A label inside the composer — prototype `Label className="text-[11.5px]"`. */
const COMPOSER_LABEL = "mb-1 block text-[11.5px] leading-tight font-semibold text-ink";

/** The label–value pairs inside the `Hạn xử lý` cell: small label, value under it. */
const FACT_LIST = cn(
  "m-0 flex flex-col text-[12.5px] text-navy",
  "[&>dt]:text-[11px] [&>dt]:font-semibold [&>dt]:text-ink-muted [&>dt:not(:first-of-type)]:mt-1.5",
  "[&>dd]:m-0 [&_dd_p]:my-0 [&_dd_p]:mt-0.5",
);

/** The branch box's extra facts (receiving body · when). */
const BRANCH_LIST = cn(
  "m-0 mt-1.5 grid grid-cols-1 gap-x-4 gap-y-0.5 text-[11.5px] sm:grid-cols-[9rem_minmax(0,1fr)]",
  "[&>dt]:font-semibold [&>dt]:text-ink-muted [&>dd]:m-0 [&>dd]:min-w-0 [&>dd]:text-navy",
);

/** One titled cell of the facts row (`FeedbackDetailDrawer.tsx:562-577`): 10.5px bold uppercase label. */
function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 bg-white px-4 py-2.5">
      <p className="m-0 mb-1 text-[10.5px] font-bold tracking-wide text-ink-muted uppercase">{label}</p>
      {children}
    </div>
  );
}

/**
 * THE STATUS STRIP AND ITS COMPOSER — prototype `FeedbackStatusPipeline.tsx:160-333`, owner decision
 * D1. A CHILD COMPONENT ON PURPOSE: the open chip is state, and `ChiTietPhieu` must not gain a hook
 * (`chon-can-bo.test.tsx` seeds its hooks by call order).
 *
 * The open chip is DERIVED against the current view: after a successful move the petition's status
 * changes, the chip is no longer a `next` one, and the composer folds by itself.
 */
function StatusStrip({
  status,
  view,
  initialStep,
  renderAct,
}: {
  status: string;
  view: StatusStripView;
  initialStep: string | null;
  renderAct: (step: StripStep, close: () => void) => ReactNode;
}) {
  const [openCode, setOpenCode] = useState<string | null>(initialStep);
  const open = [...view.main, ...view.branches].find((s) => s.code === openCode && s.role === "next") ?? null;
  const close = () => setOpenCode(null);
  const explanation = cauGiaiThichTrangThai(status);
  const chip = (s: StripStep) => (
    <StepChip key={s.code} step={s} open={open?.code === s.code} pick={() => setOpenCode(s.code)} />
  );

  return (
    <div className="shrink-0 border-b border-solid border-line bg-white px-5 py-3 [&>p]:m-0">
      <ol aria-label="Các bước xử lý phiếu" className={STEP_LIST}>
        {view.main.map(chip)}
      </ol>
      {/* Only the branch the petition is on, or one it can move to (`FeedbackStatusPipeline.tsx:156-158`). */}
      {view.branches.length > 0 && (
        <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-[11px] text-ink-muted">Rẽ nhánh:</span>
          <ol aria-label="Rẽ nhánh" className={STEP_LIST}>
            {view.branches.map(chip)}
          </ol>
        </div>
      )}
      {explanation !== null && <p className="mt-2 text-[11.5px] text-ink-muted">{explanation}</p>}

      {open !== null && (
        <div
          className="mt-2.5 rounded-[10px] border border-l-4 border-solid border-brand/35 border-l-brand bg-white p-3 [&>p]:m-0"
          role="group"
          aria-label={composerTitle(open.code)}
        >
          <p className="text-[12.5px] font-semibold text-navy">{composerTitle(open.code)}</p>
          {cauGiaiThichTrangThai(open.code) !== null && (
            <p className="mt-0.5 text-[11.5px] text-ink-muted">{cauGiaiThichTrangThai(open.code)}</p>
          )}
          <div className="mt-2">{renderAct(open, close)}</div>
        </div>
      )}

      {view.noPermission && <p className="mt-2 text-[11px] text-ink-muted">{STRIP_NO_PERMISSION}</p>}
    </div>
  );
}

/** A row of the status strip (`FeedbackStatusPipeline.tsx:162`): chips that share the width and wrap. */
const STEP_LIST = "m-0 flex min-w-0 flex-1 list-none flex-wrap items-stretch gap-1.5 p-0";

/**
 * One chip (`FeedbackStatusPipeline.tsx:112-153`): the step's icon and word on top, its role under it —
 * "đang ở đây" / "chuyển sang" / "—". The current step is filled with ITS status colour and carries
 * `aria-current`. A chip that cannot be pressed is `aria-disabled` (still focusable, so its `title` —
 * the reason — can be read), and pressing it does nothing.
 */
function StepChip({ step, open, pick }: { step: StripStep; open: boolean; pick: () => void }) {
  const current = step.role === "current";
  const next = step.role === "next";
  return (
    <li className="flex min-w-[6.5rem] flex-1">
      <button
        type="button"
        title={step.reason}
        aria-current={current ? "step" : undefined}
        aria-disabled={next ? undefined : "true"}
        aria-expanded={next ? open : undefined}
        onClick={next ? pick : undefined}
        className={cn(
          "w-full rounded-[8px] border border-solid px-2.5 py-1.5 text-left [font-family:inherit] transition-colors",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
          current
            ? cn("cursor-default border-transparent", statusActiveClass(step.code))
            : next
              ? cn("cursor-pointer border-line bg-white text-ink hover:bg-canvas", open && "border-brand bg-brand/12")
              : "cursor-not-allowed border-line/60 bg-white text-ink-muted/60",
        )}
      >
        <span className="flex items-center gap-1 text-[12px] font-semibold">
          <Glyph icon={statusIcon(step.code)} className="size-3.5 shrink-0" />
          {step.label}
        </span>
        <span className={cn("block text-[10.5px]", current ? "text-white/80" : "text-ink-muted/70")}>
          {current ? "đang ở đây" : next ? "chuyển sang" : "—"}
        </span>
      </button>
    </li>
  );
}

/**
 * The composer's buttons (`FeedbackStatusPipeline.tsx:273-317`): `Đính kèm ảnh, tệp` (a disabled
 * placeholder — no act route takes a file), `Huỷ` on the right, `Xác nhận`.
 */
function ComposerButtons({ cancel, busy, disabled }: { cancel: () => void; busy: boolean; disabled: boolean }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <PendingButton
        info={petitionPendingPart("composerAttachment")}
        size="sm"
        icon={<Glyph icon={Paperclip} className="size-3.5" />}
      >
        Đính kèm ảnh, tệp
      </PendingButton>
      <button type="button" className={buttonClass("secondary", "sm", "ml-auto")} onClick={cancel} disabled={busy}>
        Huỷ
      </button>
      <button type="submit" className={buttonClass("primary", "sm")} disabled={disabled}>
        Xác nhận
      </button>
    </div>
  );
}

/**
 * The optional `Nội dung cập nhật` of every act (`FeedbackStatusPipeline.tsx:235-245`). The words go to
 * the processing log, NOT to the citizen — unlike `reason` / `result`, which the citizen reads; the line
 * under the box says so. Blank sends nothing (`thanGhiChu`).
 */
export function ONhapGhiChuNoiBo({
  id,
  giaTri,
  datGiaTri,
}: {
  id: string;
  giaTri: string;
  datGiaTri: (s: string) => void;
}) {
  const loi = loiGhiChuNoiBo(giaTri);
  return (
    <div>
      <label htmlFor={id} className={COMPOSER_LABEL}>
        {NHAN_O_GHI_CHU_NOI_BO}
      </label>
      <textarea
        id={id}
        name={id}
        rows={2}
        className={cn(TEXTAREA_CLASS, "bg-white", loi !== null && "border-danger-600")}
        value={giaTri}
        placeholder={UPDATE_NOTE_PLACEHOLDER}
        onChange={(e) => datGiaTri(e.target.value)}
        aria-describedby={`${id}-dem`}
      />
      <p className={cn(HINT_CLASS, loi !== null && "text-danger-600")} id={`${id}-dem`} aria-live="polite">
        {demKyTu(giaTri)}/{GHI_CHU_TOI_DA} ký tự · {UPDATE_NOTE_INTERNAL}
        {loi !== null ? ` · ${loi}` : ""}
      </p>
    </div>
  );
}

const KHONG_CO_TRONG_PHAN_HOI = "Máy chủ không trả trường này";

/** Trường tuỳ chọn của nhánh rẽ: vắng hay rỗng thì NÓI RA, không để ô trống hay chữ `undefined`. */
function nhanTruongNhanh(giaTri: string | undefined): string {
  return giaTri === undefined || giaTri === "" ? KHONG_CO_TRONG_PHAN_HOI : giaTri;
}

/**
 * Biểu mẫu của MỘT nhánh rẽ, in the strip's composer (`FeedbackStatusPipeline.tsx:209-220`). Tách ra để
 * kiểm được bằng HTML tĩnh.
 *
 * LÝ DO CHỈ SỐNG TRONG STATE CỦA KHỐI NÀY rồi đi vào thân POST. Không lưu nháp vào bộ nhớ trình
 * duyệt, không ghi console: đó là chữ về việc của một công dân (luật 3).
 *
 * Nút gửi bị khoá khi giới hạn phía client chưa đạt (≥ 10 ký tự lý do, cơ quan tiếp nhận ở nhánh chuyển
 * cấp trên) — nhưng giới hạn thật ở máy chủ, và câu 400 của nó vẫn ra nguyên văn nếu hai bên lệch nhau.
 */
export function BieuMauReNhanh({
  loai,
  dangGui,
  gui,
  huy,
  lyDoBanDau = "",
  coQuanBanDau = "",
  ghiChuBanDau = "",
}: {
  loai: "khong-tiep-nhan" | "chuyen-cap-tren";
  dangGui: boolean;
  /**
   * `coQuanTiepNhan` là chuỗi rỗng ở nhánh `khong-tiep-nhan` và bị bỏ qua. `ghiChu` là `Nội dung cập
   * nhật` tuỳ chọn — vào nhật ký, KHÔNG phải lý do người dân đọc.
   */
  gui: (lyDo: string, coQuanTiepNhan: string, ghiChu: string) => void | Promise<boolean>;
  huy: () => void;
  /** Chỉ để kiểm: giá trị ban đầu của các ô. */
  lyDoBanDau?: string;
  coQuanBanDau?: string;
  ghiChuBanDau?: string;
}) {
  const [lyDo, datLyDo] = useState(lyDoBanDau);
  const [coQuan, datCoQuan] = useState(coQuanBanDau);
  const [ghiChu, datGhiChu] = useState(ghiChuBanDau);

  const chuyenCap = loai === "chuyen-cap-tren";
  const loiO1 = loiLyDo(lyDo);
  const loiO2 = chuyenCap ? loiCoQuan(coQuan) : null;
  const hopLe = loiO1 === null && loiO2 === null && loiGhiChuNoiBo(ghiChu) === null;
  const idLyDo = `ly-do-${loai}`;
  const idCoQuan = `co-quan-${loai}`;

  return (
    <form
      className={COMPOSER_FORM}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        if (hopLe) gui(lyDo, chuyenCap ? coQuan : "", ghiChu);
      }}
    >
      {/* The act ends the petition at the commune and cannot be undone — said before the reason box. */}
      <p className="m-0 text-[11.5px] font-medium text-danger">{CANH_BAO_RE_NHANH}</p>

      <div>
        <label htmlFor={idLyDo} className={COMPOSER_LABEL}>
          {NHAN_O_LY_DO}
        </label>
        <textarea
          id={idLyDo}
          name={idLyDo}
          rows={2}
          className={cn(TEXTAREA_CLASS, "bg-white", loiO1 !== null && lyDo !== "" && "border-danger-600")}
          value={lyDo}
          onChange={(e) => datLyDo(e.target.value)}
          aria-describedby={`${idLyDo}-dem`}
        />
        <p
          className={cn(HINT_CLASS, loiO1 !== null && lyDo !== "" && "text-danger-600")}
          id={`${idLyDo}-dem`}
          aria-live="polite"
        >
          {demKyTu(lyDo)}/{LY_DO_TOI_DA} ký tự{loiO1 !== null && lyDo !== "" ? ` · ${loiO1}` : ""}
        </p>
      </div>

      {chuyenCap && (
        <Field
          label={NHAN_O_CO_QUAN}
          htmlFor={idCoQuan}
          icon={Landmark}
          grow="auto"
          hint={
            <span id={`${idCoQuan}-dem`} aria-live="polite">
              {demKyTu(coQuan)}/{CO_QUAN_TOI_DA} ký tự
              {loiO2 !== null && coQuan !== "" ? ` · ${loiO2}` : ""}
            </span>
          }
        >
          <input
            id={idCoQuan}
            name={idCoQuan}
            value={coQuan}
            placeholder={GOI_Y_CO_QUAN}
            autoComplete="off"
            onChange={(e) => datCoQuan(e.target.value)}
            aria-describedby={`${idCoQuan}-dem`}
          />
        </Field>
      )}

      <ONhapGhiChuNoiBo id={`ghi-chu-${loai}`} giaTri={ghiChu} datGiaTri={datGhiChu} />
      <ComposerButtons cancel={huy} busy={dangGui} disabled={dangGui || !hopLe} />
    </form>
  );
}
