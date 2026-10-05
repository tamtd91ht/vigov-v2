"use client";

import {
  AlarmClock,
  ArrowUp,
  ArrowUpRight,
  Ban,
  BarChart3,
  Building2,
  ChevronLeft,
  ChevronRight,
  ChevronsRight,
  Circle,
  CircleAlert,
  CircleCheck,
  CircleDot,
  CloudOff,
  Flame,
  Forward,
  ImageOff,
  Landmark,
  List,
  ListChecks,
  Map as MapIcon,
  MapPin,
  MessageSquare,
  RefreshCw,
  Search,
  Shapes,
  ShieldX,
  Tags,
  UserRound,
  X,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { Notice } from "@/components/ui/notice";
import { PendingMarker, PendingSection, PendingTab } from "@/components/ui/pending-feature";
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
  buocLuongChinh,
  buocReNhanh,
  AFTER_PHOTO_GO_TO,
  ASSIGN_NEEDS_CLASSIFICATION,
  AFTER_PHOTO_REQUIRED_HINT,
  CANH_BAO_RE_NHANH,
  CAU_THIEU_QUYEN_DONG,
  CAU_THIEU_QUYEN_PHAN_CONG,
  CAU_THIEU_QUYEN_PHAN_LOAI,
  CHI_TRE_HAN_NHAN,
  CO_QUAN_TOI_DA,
  cauGiaiThichTrangThai,
  conBuocKeTiep,
  congThaoTac,
  DANG_TAI_SO,
  danhBaTheoMa,
  DE_BO_PHAN_PHAN_CONG,
  demKyTu,
  dongDuocTrenManHinh,
  GHI_CHU_O_KET_QUA,
  GHI_CHU_TOI_DA,
  GOI_Y_CO_QUAN,
  GHI_CHU_TIEN_TRANG_THAI,
  LINH_VUC_PHAN_ANH,
  initialClassifyField,
  linhVucPhanAnh,
  loiCoQuan,
  loiGhiChuNoiBo,
  loiLyDo,
  lopHan,
  LY_DO_TOI_DA,
  LOW_RATING_FILTER_LABEL,
  LOW_RATING_MAX,
  MOI_BO_PHAN_NHAN,
  MOI_DIA_BAN_NHAN,
  MOI_KENH,
  MOI_KENH_NHAN,
  MOI_LINH_VUC_NHAN,
  MOI_TRANG_THAI,
  MOI_TRANG_THAI_NHAN,
  NHAN_CHON_CAN_BO,
  NHAN_CHUYEN_CAP_TREN,
  NHAN_KHONG_TIEP_NHAN,
  NHAN_O_CO_QUAN,
  NHAN_O_GHI_CHU_NOI_BO,
  NHAN_O_KET_QUA,
  NHAN_O_LY_DO,
  NHAN_TIEN_TRANG_THAI,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanHan,
  nhanLuaChonCanBo,
  nhanKenh,
  nhanLinhVuc,
  nhanNguoiGui,
  nhanThoiDiem,
  nhanTrangThai,
  PHAM_VI_GIAO_CHO_TOI,
  PHAM_VI_TOAN_XA,
  petitionPendingPart,
  CARD_NO_LOCATION,
  LIST_EMPTY_HINT,
  LIST_EMPTY_TITLE,
  petitionTaskOffered,
  phanLoaiDuoc,
  RE_NHANH,
  reNhanhDuoc,
  reopenLine,
  ratingStars,
  SCENE_LOCATION_LABEL,
  SCOPE_RELATED_LABEL,
  SO_RONG,
  TIM_PLACEHOLDER,
  trangThaiHan,
  type CongThaoTac,
} from "./nhan-phieu";
import {
  CitizenRatingBlock,
  PublicationBox,
  SceneLocation,
  starsLabel,
} from "./citizen-report-blocks";
import { NhatKyPhieu } from "./nhat-ky-phieu";
import { PetitionTaskBlock } from "./petition-task";
import {
  ACT_CLASS,
  buttonClass,
  Glyph,
  HINT_CLASS,
  LABEL_CLASS,
  LoadingBar,
  PetitionCardsSkeleton,
  PetitionStatusBadge,
  SectionTitle,
  TEXTAREA_CLASS,
  TOGGLE_TRACK,
  toggleButtonClass,
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
 *   `Chuyển sang bước kế tiếp`  KHÔNG có cổng ở giao diện. Tuyến khai `feedback.read`, điều kiện
 *                               thật là `feedback.resolve` HOẶC chính là cán bộ được phân công —
 *                               LUẬT NẮM GIỮ. Vế thứ hai giao diện cố ý không tính lại (xem
 *                               `congThaoTac`), nên nút hiện với mọi người xem được sổ và câu 403
 *                               của máy chủ ra thẳng màn hình.
 *   `Đóng phiếu`                CÓ cổng: `feedback.resolve`, và **không** được nới theo luật nắm
 *                               giữ. Đóng phiếu ghi một kết quả NGƯỜI DÂN ĐỌC (luật 10, bất biến
 *                               6) — câu hỏi mở #7 chốt 16/09/2026.
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

/** A row of the status strip (§8.2, `FeedbackStatusPipeline.tsx:162`): chips that share the width and wrap. */
const STEP_LIST = "m-0 flex min-w-0 flex-1 list-none flex-wrap items-stretch gap-1.5 p-0";

/** Selects of a processing act: label above, 40px, full column; two columns from 640px. */
const SELECT_GRID = "grid grid-cols-1 gap-4 sm:grid-cols-2";

/** Câu của dải lọc Tổng quan trên sổ này — nói ra đúng những gì màn tạm tắt. */
export const DRILL_DOWN_NOTE_CITIZEN_REPORTS =
  "Bộ lọc, ô tìm và phạm vi tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

export function SoPhanAnh({
  drillDown = NO_DRILL_DOWN,
  reloadSignal = 0,
}: {
  /** Lọc mở từ trang Tổng quan, đọc ở máy chủ (`app/phan-anh/page.tsx`). */
  drillDown?: DrillDown<"citizen-reports">;
  /** Bumped by the page header after `+ Nhập hộ phản ánh` booked a petition: the register re-reads. */
  reloadSignal?: number;
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

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}|${reloadSignal}`;

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

  /** Trả `true` khi máy chủ nhận — để biểu mẫu xoá ô ghi chú nội bộ chỉ SAU một lần thành công. */
  function chay(goi: Promise<KetQua<petitions_phieuPhanAnhRa>>): Promise<boolean> {
    datDangGui(true);
    return goi.then((kq) => {
      xongGhi(kq);
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

      <div className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
        {/* §2 main tabs `[Danh sách] [Bản đồ nhiệt] [Báo cáo]`. Only the list exists; the other two
            are disabled placeholders with their "?" (ADR 0068 §14). One live tab, so no arrow-key
            handling to teach about disabled tabs. NO COUNT on `Danh sách` (the prototype prints the
            rows it holds): the register is paged by the server and returns no total, so a count here
            would be the count of one page passed off as the commune's. */}
        <TabList aria-label="Phần của sổ phản ánh">
          <Tab selected id="petition-tab-list" aria-controls="petition-list-panel" icon={List}>
            Danh sách
          </Tab>
          <PendingTab info={petitionPendingPart("heatMapTab")} icon={Flame} />
          <PendingTab info={petitionPendingPart("reportTab")} icon={BarChart3} />
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
          phanLoai={(linhVuc, ghiChu) => chay(phanLoaiPhieu(dangMo.code, linhVuc, ghiChu))}
          chuyenXuLy={(boPhanID, maCanBo, ghiChu) => {
            datDangGui(true);
            return chuyenXuLyPhieu(dangMo.code, boPhanID, maCanBo, ghiChu).then((kq) => {
              if (!kq.ok) {
                // The server's sentence VERBATIM, but drawn in the assign block, not the general line.
                datDangGui(false);
                datLoiGhi(null);
                setAssignRefusal(kq.thongBao);
                return false;
              }
              xongGhi(kq);
              return true;
            });
          }}
          assignRefusal={assignRefusal}
          tienTrangThai={(ghiChu) => chay(tienTrangThaiPhieu(dangMo.code, ghiChu))}
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
              return kq.ok;
            });
          }}
          khongTiepNhan={(lyDo, ghiChu) => chay(khongTiepNhanPhieu(dangMo.code, lyDo, ghiChu))}
          chuyenCapTren={(lyDo, coQuan, ghiChu) =>
            chay(chuyenCapTrenPhieu(dangMo.code, lyDo, coQuan, ghiChu))
          }
          // Same path as the six processing acts: the 200 body replaces the open petition, a refusal
          // (409 `never_public`, 403) goes verbatim to the drawer's error line.
          setPublication={(target) => chay(setPetitionPublication(dangMo.code, target))}
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
      <div className={TOGGLE_TRACK} role="group" aria-label="Phạm vi">
        <button
          type="button"
          className={toggleButtonClass(loc.phamVi !== "mine")}
          aria-pressed={loc.phamVi !== "mine"}
          onClick={() => datLoc({ ...loc, phamVi: undefined })}
        >
          {PHAM_VI_TOAN_XA}
        </button>
        <button
          type="button"
          className={toggleButtonClass(loc.phamVi === "mine")}
          aria-pressed={loc.phamVi === "mine"}
          onClick={() => datLoc({ ...loc, phamVi: "mine" })}
        >
          {PHAM_VI_GIAO_CHO_TOI}
        </button>
        <span className="relative inline-flex">
          <button
            type="button"
            disabled
            aria-pressed={false}
            className={cn(toggleButtonClass(false), "cursor-not-allowed pr-8 opacity-60 hover:text-ink-500")}
          >
            {SCOPE_RELATED_LABEL}
          </button>
          <PendingMarker info={petitionPendingPart("scopeRelated")} side="bottom" placement="end" />
        </span>
      </div>

      {/* Ô TÌM GỬI BẰNG SUBMIT (Enter), KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán
          bộ đang gõ vào một URL — và chuỗi ấy có thể là tên hay địa chỉ một công dân (luật 3, cấm #4).
          The prototype's box: 256px, a magnifier inside, no visible label, no button. */}
      <form className="m-0 w-64 max-w-full" onSubmit={timNgay} role="search">
        <Field label="Tìm trong sổ" htmlFor="tim-phan-anh" icon={Search} hideLabel grow="auto">
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

      <Field label="Trạng thái" htmlFor="loc-trang-thai" kind="select" hideLabel grow="auto">
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

      <Field label="Lĩnh vực" htmlFor="loc-linh-vuc" kind="select" hideLabel grow="auto">
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

      <Field label="Địa bàn" htmlFor="loc-dia-ban" kind="select" hideLabel grow="auto">
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

      <Field label="Bộ phận đang giữ" htmlFor="loc-bo-phan" kind="select" hideLabel grow="auto">
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

      <Field label="Kênh tiếp nhận" htmlFor="loc-kenh" kind="select" hideLabel grow="auto">
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
          className="size-4 accent-brand-600"
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
          className="size-4 accent-brand-600"
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

/** A checkbox of the filter row (`FeedbackWorkspace.tsx:200-225`): box, then its words. */
const CHECKBOX_LABEL = "flex items-center gap-2 text-[13px] text-ink-900";

/**
 * The register's empty state — the prototype's box (`FeedbackWorkspace.tsx:235-243`): one bold line,
 * one line under it.
 */
function ListEmpty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="rounded-card border border-solid border-line bg-surface p-10 text-center [&>p]:m-0">
      <p className="text-sm font-semibold text-ink-900">{title}</p>
      {hint !== undefined && <p className="mt-1.5 text-[13px] text-ink-500">{hint}</p>}
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
        <li key={p.code} className="min-w-0">
          <ThePhieu phieu={p} bayGio={bayGio} dangMo={p.code === maDangMo} mo={() => moPhieu(p)} />
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
 * THE SENDER IS THE SERVER'S MASKED PAIR (`nhanNguoiGui`) — the prototype prints the phone in full; the
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
  dangMo,
  mo,
}: {
  phieu: petitions_phieuPhanAnhRa;
  bayGio: Date;
  dangMo: boolean;
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
        "flex w-full min-w-0 cursor-pointer gap-3 overflow-hidden rounded-card border border-solid border-line bg-surface p-2.5 text-left [font-family:inherit] text-inherit",
        "motion-safe:transition-shadow motion-safe:duration-150 hover:shadow-md",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
        // The left rule follows the same comparison as the deadline line, so the two never differ; the
        // AlarmClock and the words say it too — never colour alone.
        pastDeadline && "border-l-4 border-l-danger-600",
        dangMo && "ring-2 ring-brand-500",
      )}
    >
      <span
        aria-hidden="true"
        className="grid size-20 shrink-0 place-items-center rounded-lg bg-surface-muted text-ink-400"
      >
        <ImageOff className="size-5" strokeWidth={1.6} focusable="false" />
      </span>

      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex flex-wrap items-center gap-1.5">
          <span className="ma-muc rounded border border-solid border-line bg-surface-muted px-1.5 py-0.5 text-[11px] font-semibold text-ink-500">
            {phieu.code}
          </span>
          <span className="rounded-full bg-accent-50 px-2 py-0.5 text-[11px] font-semibold text-ink-900">
            {nhanLinhVuc(linhVuc)}
          </span>
        </span>

        {/* Nội dung phản ánh KHÔNG che (cán bộ không đọc được thì không xử lý được), nhưng nó là chữ
            của một công dân: không bao giờ ghi nó vào log, tên tệp hay URL. */}
        <span className="mt-1.5 line-clamp-2 text-[13px] leading-snug text-ink-900">{phieu.content}</span>

        <span className="mt-1 flex min-w-0 items-center gap-1 text-xs text-ink-500">
          <Glyph icon={MapPin} className="size-3 shrink-0" />
          <span className="truncate">{phieu.address === "" ? CARD_NO_LOCATION : phieu.address}</span>
        </span>

        <span className="mt-1 flex min-w-0 items-center gap-1 text-xs text-ink-500">
          <Glyph icon={UserRound} className="size-3 shrink-0" />
          <span className="truncate">{nhanNguoiGui(phieu)}</span>
        </span>

        <span className="mt-auto flex flex-wrap items-center gap-2 pt-1.5">
          <PetitionStatusBadge status={phieu.status}>{nhanTrangThai(phieu.status)}</PetitionStatusBadge>
          {/* The words are `nhanHan`'s ("Quá hạn · hạn cuối …"); AlarmClock beside them, never instead. */}
          <span className={cn(lopHan(hanXuLy), "inline-flex items-center gap-1 text-xs")}>
            {pastDeadline && <Glyph icon={AlarmClock} className="size-3.5 shrink-0" />}
            {nhanHan(hanXuLy)}
          </span>
          {rating !== null ? (
            // §7 corner: the citizen's stars once rated.
            <span
              className={cn("ml-auto text-[13px]", rating <= LOW_RATING_MAX ? "chip nhan-lech" : "chip")}
              role="img"
              aria-label={starsLabel(rating)}
            >
              {ratingStars(rating)}
            </span>
          ) : (
            <span className="ml-auto inline-flex items-center gap-1 text-[11px] text-ink-500">
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
 * Khối chi tiết §8, kèm bốn thao tác — the prototype's right-hand drawer, in `LargeDialog` (ADR 0068
 * lần 5; layout under `return`). Mounted = open: the register renders it only while a petition is open.
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
  /** Vẽ nút `Ghi nhật ký`. Mặc định KHÔNG — chưa rõ quyền thì không hành xử như có. */
  coGhiNhatKy?: boolean;
  /** Tăng sau mỗi thao tác thành công, để nhật ký đọc lại dòng máy chủ vừa ghi. */
  lanLamMoiNhatKy?: number;
  /**
   * The session's permission keys — ONLY to decide whether `Tạo nhiệm vụ` is drawn
   * (`petitionTaskOffered`). Empty by default: an unknown session holds no key (rule 1, forbidden #1).
   */
  permissions?: readonly string[];
  /** Called after POST …/tasks answered 201. Absent = the block is not drawn (no write path). */
  onTaskCreated?: () => void;
  dong: () => void;
  /*
   * `ghiChu` ở cả sáu thao tác là GHI CHÚ NỘI BỘ tuỳ chọn, vào nhật ký, không gửi người dân. Trả
   * `Promise<boolean>` (máy chủ nhận hay không) để ô ghi chú chỉ được xoá SAU một lần thành công;
   * `void` vẫn được nhận — khi ấy ô giữ nguyên chữ.
   */
  phanLoai: (linhVuc: string, ghiChu: string) => KetQuaGui;
  /** `maCanBo` là MÃ CÁN BỘ (`code` của danh bạ), vắng khi để bộ phận tự phân công. */
  chuyenXuLy: (boPhanID: string, maCanBo: string | undefined, ghiChu: string) => KetQuaGui;
  tienTrangThai: (ghiChu: string) => KetQuaGui;
  dongPhieuLai: (ketQua: string, ghiChu: string) => KetQuaGui;
  /**
   * The commune's refusal to close without a verification photo (409 `after_photo_required`), verbatim,
   * or `null`. Drawn inside the close block with the way to the `Sau khi xử lý` upload.
   */
  closeRefusal?: string | null;
  /** Any refusal of `Chuyển xử lý`, verbatim, or `null`. Drawn inside the assign block, by its button. */
  assignRefusal?: string | null;
  khongTiepNhan: (lyDo: string, ghiChu: string) => KetQuaGui;
  chuyenCapTren: (lyDo: string, coQuanTiepNhan: string, ghiChu: string) => KetQuaGui;
  /**
   * PUT …/publication. Optional so a caller without a write path renders the box read-only; the
   * buttons also need `cong.moderate`.
   */
  setPublication?: (target: PublicationTarget) => KetQuaGui;
}) {
  // Seeded with the field already chosen at intake (PA-03) — a lazy initialiser, so the hook order the
  // `chon-can-bo.test.tsx` seeding relies on is unchanged.
  const [linhVucChon, datLinhVucChon] = useState(() => initialClassifyField(phieu.field));
  const [boPhanChon, datBoPhanChon] = useState("");
  const [canBoChon, datCanBoChon] = useState("");
  const [ketQua, datKetQua] = useState("");
  // SAU bốn hook trên, không trước: `chon-can-bo.test.tsx` gieo giá trị theo THỨ TỰ gọi hook.
  const [reNhanhMo, datReNhanhMo] = useState<"khong-tiep-nhan" | "chuyen-cap-tren" | null>(null);
  // Bốn ô ghi chú nội bộ, MỖI BIỂU MẪU MỘT Ô: một ô chung sẽ mang ghi chú viết cho lần chuyển xử
  // lý sang lần đóng phiếu. Cũng đứng SAU năm hook trên, vì cùng lý do.
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
  const giaiThich = cauGiaiThichTrangThai(phieu.status);
  const reopened = reopenLine(phieu.reopen_count);

  return (
    // THE PROTOTYPE'S RIGHT-HAND DRAWER (`FeedbackDetailDrawer.tsx`, ADR 0068 lần 5) in the shared
    // `LargeDialog` (pinned right, full height, Esc and ✕ close it), top to bottom:
    //
    //   header   code · channel · booked at (the dialog's NAME), the field large, the sender (masked)
    //   strip    the lifecycle's steps, the two branches, one sentence on the current status
    //   facts    three cells: Hạn xử lý (both clocks) · Đang giao cho · Hiển thị với người dân
    //   body     left: content (+ result / branch reason), photos, location, duplicates "?", rating,
    //            the processing acts; right (from 768px; under the left column below): the log
    //
    // THE DIALOG'S NAME IS THE CODE, CHANNEL AND TIME, NEVER THE FIELD OR THE CONTENT: a name goes into
    // the accessibility tree, and the content is a citizen's words (rule 3, forbidden #4).
    <LargeDialog titleId="tieu-de-chi-tiet-phieu" onDismiss={dong}>
      <header className="flex shrink-0 items-start gap-3 border-b border-solid border-line bg-surface px-5 py-4">
        <div className="min-w-0 flex-1 [&>p]:m-0">
          <h2
            id="tieu-de-chi-tiet-phieu"
            tabIndex={-1}
            className="m-0 text-xs leading-snug font-semibold text-ink-500 tabular-nums"
          >
            <span className="ma-muc">{phieu.code}</span> · {nhanKenh(phieu.channel)} · {nhanThoiDiem(phieu.booked_at)}
          </h2>
          <p className="mt-0.5 text-base leading-snug font-bold text-ink-900">{nhanLinhVuc(linhVuc)}</p>
          <p className="mt-1 text-[13px] text-ink-500">{nhanNguoiGui(phieu)}</p>
        </div>
        <IconButton label="Đóng chi tiết phiếu" type="button" variant="secondary" onClick={dong}>
          <Glyph icon={X} />
        </IconButton>
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        {/* THE STATUS STRIP (`FeedbackStatusPipeline.tsx:160-183`): the whole path, the current step
            filled. NOT CLICKABLE, unlike the prototype's: every move here carries its own form below
            (a field to settle, a unit to pick, a result the citizen reads, a mandatory reason), each
            behind its own key (`congThaoTac`). Icon + word on every step — never colour alone. */}
        <div className="flex shrink-0 flex-col gap-1.5 border-b border-solid border-line bg-surface px-5 py-3 [&>p]:m-0">
          <ol aria-label="Các bước xử lý phiếu" className={STEP_LIST}>
            {buocLuongChinh(phieu.status).map((o) => (
              <StepChip
                key={o.ma}
                label={o.nhan}
                role={o.vaiTro === "dangODay" ? "current" : o.vaiTro === "daQua" ? "done" : "ahead"}
              />
            ))}
          </ol>
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="mr-1 text-xs text-ink-500">Rẽ nhánh:</span>
            <ol aria-label="Rẽ nhánh" className={STEP_LIST}>
              {buocReNhanh(phieu.status).map((o) => (
                <StepChip
                  key={o.ma}
                  label={o.nhan}
                  role={o.vaiTro === "dangODay" ? "current" : "ahead"}
                  icon={o.ma === "khong-tiep-nhan" ? Ban : ArrowUpRight}
                />
              ))}
            </ol>
          </div>
          {giaiThich !== null && <p className="mt-1 text-xs text-ink-500">{giaiThich}</p>}
        </div>

        {/* THE THREE FACT CELLS (`FeedbackDetailDrawer.tsx:229-310`). The deadline cell holds BOTH
            clocks (ADR 0027/0028) and the classify deadline — each a comparison of a stored instant with
            now, never a stored flag (rule 10, invariant 3). */}
        <div className="grid shrink-0 grid-cols-1 gap-px border-b border-solid border-line bg-line sm:grid-cols-3">
          <Fact label="Hạn xử lý">
            <dl className={FACT_LIST}>
              <dt>Hạn xử lý xong</dt>
              <dd>
                <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
                {/* Requirement `FeedbackDetailDrawer.tsx:242-246`. The deadline is NOT recomputed on a
                    reopening (ADR 0050 point 2) — this line is what explains an old deadline on a
                    petition that is back in progress. */}
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
            <p className="m-0 text-sm text-ink-900">{nhanBoPhan(phieu.unit, tenBoPhan)}</p>
            <p className="m-0 mt-0.5 text-xs text-ink-500">{nhanCanBoXuLy(phieu.assignee, bangDanhBa)}</p>
          </Fact>

          <Fact label="Hiển thị với người dân">
            <div className="text-sm text-ink-900 [&>p]:m-0 [&>p+p]:mt-0.5">
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
          <div className="flex min-w-0 flex-1 flex-col gap-4 bg-surface-muted px-5 py-4 [&>*]:my-0">
            {/* Right under the facts, so a refusal of the publication buttons above (409
                `never_public`) reads next to the box that caused it. The `<p>` itself is unchanged (a
                test reads it). */}
            {loiGhi !== null && (
              <div className="flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
                <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
                <p className="thong-bao-loi" role="alert">
                  {loiGhi}
                </p>
              </div>
            )}

            {/* `Nội dung phản ánh` with its title, then what the citizen was answered. */}
            <section aria-labelledby="tieu-de-noi-dung-phieu" className="flex flex-col gap-2 [&>*]:my-0">
              <h3 id="tieu-de-noi-dung-phieu" className={SECTION_HEADING}>
                Nội dung phản ánh
              </h3>
              <p className="noi-dung-phan-anh rounded-[10px] border border-solid border-line bg-surface px-3 py-2.5 text-[15px] leading-relaxed text-ink-900">
                {phieu.content}
              </p>
              {/* Kết quả CHỈ hiện khi đã có: một ô trống ở đây trông như một trường chưa điền, trong
                  khi phiếu chưa đóng thì nó chưa tồn tại. */}
              {phieu.result !== "" && (
                <div className="rounded-[10px] border border-solid border-success-500/30 bg-success-50 px-3 py-2.5 [&>p]:m-0">
                  <p className="text-[11px] font-semibold text-ink-500 uppercase">
                    Kết quả xử lý đã trả lời người dân
                  </p>
                  <p className="noi-dung-phan-anh mt-0.5 text-sm text-ink-900">{phieu.result}</p>
                </div>
              )}
              {/* LÝ DO VÀ CƠ QUAN NHẬN CHỈ CÓ Ở HAI NHÁNH RẼ. Ở trạng thái khác hai trường không tồn
                  tại — vẽ ô trống là vẽ một trường trông như chưa điền. */}
              {RE_NHANH.includes(phieu.status) && (
                <dl className={cn(BRANCH_LIST, "rounded-[10px] border border-solid border-warning-500/30 bg-warning-50 px-3 py-2.5")}>
                  <dt>Lý do</dt>
                  <dd>{nhanTruongNhanh(phieu.reason)}</dd>

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
              )}
            </section>

            {/* §8.4 photos. A separate component, NOT a hook here: it reads the network (`useEffect`),
                and `chon-can-bo.test.tsx` calls this block as a plain function. Hidden without
                `feedback.read` — UX only; the route checks the same key and `feedback.restricted`
                (rule 5, forbidden #1). */}
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

            {/* `Vị trí` — the address and the coordinates as TEXT, always said (absence too). The map
                and the hamlet beside the address are a disabled placeholder with its "?" (ADR 0068
                §14): it draws NO map and loads NO tile — that is exactly the undecided step (a
                citizen's coordinates sent to an outside provider). */}
            <section aria-labelledby="tieu-de-vi-tri-phieu" className="flex flex-col gap-2 [&>*]:my-0">
              <h3 id="tieu-de-vi-tri-phieu" className={SECTION_HEADING}>
                {SCENE_LOCATION_LABEL}
              </h3>
              <div className="rounded-[10px] border border-solid border-line bg-surface px-3 py-2.5 text-sm text-ink-900 [&>p]:m-0 [&>p+p]:mt-1">
                <SceneLocation petition={phieu} />
              </div>
              <PendingSection info={petitionPendingPart("sceneMap")} titleAs="h3">
                <div
                  aria-hidden="true"
                  className="grid h-32 place-items-center rounded-lg border border-dashed border-line-strong bg-surface-muted text-ink-400"
                >
                  <MapIcon className="size-6" strokeWidth={1.6} focusable="false" />
                </div>
              </PendingSection>
            </section>

            {/* `Có thể trùng với phiếu khác` (`FeedbackDetailDrawer.tsx:408-439`) — no route detects or
                merges duplicates: a placeholder with its "?", no button that would do nothing. */}
            <PendingSection info={petitionPendingPart("duplicates")} titleAs="h3" />

            <CitizenRatingBlock petition={phieu} headingId="tieu-de-danh-gia-phieu" />

            {/* ── XỬ LÝ PHIẾU — the processing acts, one card, a hairline between acts ───────────────── */}
            <Card as="section" aria-labelledby="tieu-de-xu-ly-phieu">
              <CardHeader>
                <SectionTitle icon={ListChecks} id="tieu-de-xu-ly-phieu">
                  Xử lý phiếu
                </SectionTitle>
              </CardHeader>
              <div className="flex min-w-0 flex-col divide-y divide-line">
            {/* ── 1. PHÂN LOẠI ─────────────────────────────────────────────────────────────────── */}
            {cong.phanLoai ? (
              phanLoaiDuoc(phieu.status) && (
                <form
                  className={ACT_CLASS}
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (linhVucChon !== "" && loiGhiChuNoiBo(ghiChuPhanLoai) === null) {
                      xoaKhiThanhCong(phanLoai(linhVucChon, ghiChuPhanLoai), () => datGhiChuPhanLoai(""));
                    }
                  }}
                >
                  <SectionTitle as="h5" icon={Tags}>
                    Phân loại phiếu
                  </SectionTitle>
                  <p className="ghi-chu m-0">
                    Chốt lĩnh vực là hành vi ấn định hạn xử lý xong theo cấu hình thời hạn của xã. Xã chưa
                    cấu hình lĩnh vực này thì máy chủ từ chối và nói ra màn cần vào.
                  </p>
                  {/* Same 2-column grid as `Chuyển xử lý` (owner, 02/10/2026: label above, 40px, the full
                      column) — one select takes one column, so both acts line up. */}
                  <div className={SELECT_GRID}>
                  <Field label="Lĩnh vực" htmlFor="chon-linh-vuc" icon={Shapes} kind="select" grow="auto">
                    <select
                      id="chon-linh-vuc"
                      value={linhVucChon}
                      onChange={(e) => datLinhVucChon(e.target.value)}
                    >
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
                  <div className="cum-nut justify-end">
                    <button
                      type="submit"
                      className={buttonClass("primary")}
                      disabled={dangGui || linhVucChon === "" || loiGhiChuNoiBo(ghiChuPhanLoai) !== null}
                    >
                      <Glyph icon={Tags} />
                      Chốt lĩnh vực
                    </button>
                  </div>
                </form>
              )
            ) : (
              <div className="p-4">
                <Notice tone="neutral" icon={ShieldX}>
                  {CAU_THIEU_QUYEN_PHAN_LOAI}
                </Notice>
              </div>
            )}

            {/* ── 1b. HAI NHÁNH RẼ — cùng cổng `feedback.classify`, chỉ ở `dang-phan-loai` ──────────── */}
            {cong.phanLoai && reNhanhDuoc(phieu.status) && (
              <div className="cum-nut p-4">
                <button
                  type="button"
                  className={buttonClass("secondary")}
                  aria-pressed={reNhanhMo === "khong-tiep-nhan"}
                  onClick={() => datReNhanhMo(reNhanhMo === "khong-tiep-nhan" ? null : "khong-tiep-nhan")}
                >
                  <Glyph icon={Ban} />
                  {NHAN_KHONG_TIEP_NHAN}
                </button>
                <button
                  type="button"
                  className={buttonClass("secondary")}
                  aria-pressed={reNhanhMo === "chuyen-cap-tren"}
                  onClick={() => datReNhanhMo(reNhanhMo === "chuyen-cap-tren" ? null : "chuyen-cap-tren")}
                >
                  <Glyph icon={ArrowUpRight} />
                  {NHAN_CHUYEN_CAP_TREN}
                </button>
              </div>
            )}
            {cong.phanLoai && reNhanhDuoc(phieu.status) && reNhanhMo !== null && (
              <div className="p-4">
              <BieuMauReNhanh
                // Khoá theo nhánh: đổi nhánh là ô trống lại, không mang lý do của nhánh kia sang.
                key={reNhanhMo}
                loai={reNhanhMo}
                dangGui={dangGui}
                gui={(lyDo, coQuan, ghiChu) =>
                  reNhanhMo === "khong-tiep-nhan"
                    ? khongTiepNhan(lyDo, ghiChu)
                    : chuyenCapTren(lyDo, coQuan, ghiChu)
                }
                huy={() => datReNhanhMo(null)}
              />
              </div>
            )}

            {/* ── 2. CHUYỂN XỬ LÝ (§8.5) ───────────────────────────────────────────────────────── */}
            {/* Not drawn while the petition is still classifiable: the server answers 409 from
                `da-tiep-nhan` (ADR 0027). The reason is said instead — UX only, the server still decides. */}
            {cong.phanCong && phanLoaiDuoc(phieu.status) ? (
              <div className="p-4">
                <Notice tone="neutral" icon={Tags}>
                  {ASSIGN_NEEDS_CLASSIFICATION}
                </Notice>
              </div>
            ) : cong.phanCong ? (
              <form
                className={ACT_CLASS}
                onSubmit={(e) => {
                  e.preventDefault();
                  if (boPhanChon !== "" && loiGhiChuNoiBo(ghiChuPhanCong) === null) {
                    xoaKhiThanhCong(
                      chuyenXuLy(boPhanChon, canBoChon === "" ? undefined : canBoChon, ghiChuPhanCong),
                      () => datGhiChuPhanCong(""),
                    );
                  }
                }}
              >
                <SectionTitle as="h5" icon={Forward}>
                  Chuyển xử lý, không đổi trạng thái
                </SectionTitle>
                <div className={SELECT_GRID}>
                <Field label="Bộ phận" htmlFor="chon-bo-phan" icon={Building2} kind="select" grow="auto">
                  <select
                    id="chon-bo-phan"
                    value={boPhanChon}
                    onChange={(e) => {
                      datBoPhanChon(e.target.value);
                      // Đổi bộ phận thì bỏ người đã chọn: người ấy thuộc bộ phận cũ, và một lựa chọn
                      // không còn nằm trong ô chọn là một lựa chọn cán bộ không nhìn thấy mà vẫn gửi đi.
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
                {/* GIÁ TRỊ CỦA MỖI LỰA CHỌN LÀ MÃ CÁN BỘ (`code`), không phải id nội bộ — luật nắm giữ ở
                    máy chủ so đúng mã ấy với phiên của người được giao. Mã chỉ nằm trong thân POST, không
                    lên URL. */}
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
                    Không tải được danh bạ cán bộ: {danhBa.thongBao} Vẫn chuyển được phiếu cho bộ phận tự
                    phân công.
                  </p>
                )}
                {danhBa !== null && danhBa.ok && boPhanChon !== "" && canBoCuaBoPhan.length === 0 && (
                  <p className="ghi-chu m-0">
                    Bộ phận này chưa có cán bộ nào có tài khoản đang hoạt động. Phiếu sẽ để bộ phận tự
                    phân công.
                  </p>
                )}
                <ONhapGhiChuNoiBo id="ghi-chu-phan-cong" giaTri={ghiChuPhanCong} datGiaTri={datGhiChuPhanCong} />
                {assignRefusal !== null && (
                  <div className="flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
                    <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
                    <p className="thong-bao-loi" role="alert">
                      {assignRefusal}
                    </p>
                  </div>
                )}
                <div className="cum-nut justify-end">
                  <button
                    type="submit"
                    className={buttonClass("primary")}
                    disabled={dangGui || boPhanChon === "" || loiGhiChuNoiBo(ghiChuPhanCong) !== null}
                  >
                    <Glyph icon={Forward} />
                    Chuyển xử lý
                  </button>
                </div>
              </form>
            ) : (
              <div className="p-4">
                <Notice tone="neutral" icon={ShieldX}>
                  {CAU_THIEU_QUYEN_PHAN_CONG}
                </Notice>
              </div>
            )}

            {/* ── 3. TIẾN TRẠNG THÁI — KHÔNG CÓ CỔNG Ở GIAO DIỆN ───────────────────────────────── */}
            {conBuocKeTiep(phieu.status) && (
              <div className={ACT_CLASS}>
                <ONhapGhiChuNoiBo id="ghi-chu-tien" giaTri={ghiChuTien} datGiaTri={datGhiChuTien} />
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="ghi-chu m-0 min-w-0 flex-1 basis-60">{GHI_CHU_TIEN_TRANG_THAI}</p>
                  <button
                    type="button"
                    className={buttonClass("primary")}
                    disabled={dangGui || loiGhiChuNoiBo(ghiChuTien) !== null}
                    onClick={() => xoaKhiThanhCong(tienTrangThai(ghiChuTien), () => datGhiChuTien(""))}
                  >
                    <Glyph icon={ChevronsRight} />
                    {NHAN_TIEN_TRANG_THAI}
                  </button>
                </div>
              </div>
            )}

            {/* ── 4. ĐÓNG PHIẾU — CỔNG `feedback.resolve`, KHÔNG NỚI THEO LUẬT NẮM GIỮ ─────────── */}
            {/* Chỉ ở hai điểm máy chủ cho đóng (`dongDuocTrenManHinh`) — điểm `da-xu-ly` đọc cờ
                `has_citizen`, và quay về luật kênh `can-bo-nhap-ho` khi cờ vắng (`coCongDanXacNhan`). */}
            {cong.dongPhieu ? (
              dongDuocTrenManHinh(phieu) && (
              <form
                className={ACT_CLASS}
                onSubmit={(e) => {
                  e.preventDefault();
                  if (ketQua.trim() !== "" && loiGhiChuNoiBo(ghiChuDong) === null) {
                    xoaKhiThanhCong(dongPhieuLai(ketQua.trim(), ghiChuDong), () => datGhiChuDong(""));
                  }
                }}
              >
                <SectionTitle as="h5" icon={CircleCheck}>
                  Đóng phiếu
                </SectionTitle>
                <div>
                  <label htmlFor="ket-qua-xu-ly" className={LABEL_CLASS}>
                    {NHAN_O_KET_QUA}
                  </label>
                  <textarea
                    id="ket-qua-xu-ly"
                    name="ket-qua-xu-ly"
                    rows={3}
                    className={TEXTAREA_CLASS}
                    value={ketQua}
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
                <div className="cum-nut justify-end">
                  <button
                    type="submit"
                    className={buttonClass("primary")}
                    disabled={dangGui || ketQua.trim() === "" || loiGhiChuNoiBo(ghiChuDong) !== null}
                  >
                    <Glyph icon={CircleCheck} />
                    Đóng phiếu
                  </button>
                </div>
              </form>
              )
            ) : (
              <div className="p-4">
                <Notice tone="neutral" icon={ShieldX}>
                  {CAU_THIEU_QUYEN_DONG}
                </Notice>
              </div>
            )}

            {/* ── 4b. TẠO NHIỆM VỤ (§13) — `task.create` AND `feedback.read`; UX only, the server decides ── */}
            {onTaskCreated !== undefined && petitionTaskOffered(permissions, phieu) && (
              <PetitionTaskBlock lookupCode={phieu.code} danhBa={danhBa} onCreated={onTaskCreated} />
            )}
              </div>
            </Card>

          </div>

          {/* ── NHẬT KÝ XỬ LÝ (§8.7) — the right column, as in the prototype: "where has this petition
              been" is what the handler looks at most. Under the left column below 768px. A component
              of its own, NOT a hook here: it reads the network (`useEffect`), and
              `chon-can-bo.test.tsx` calls this block as a plain function. */}
          <aside className="shrink-0 border-t border-solid border-line bg-surface px-4 py-3 md:w-[26rem] md:border-t-0 md:border-l">
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

/** Heading of a drawer section (`FeedbackDetailDrawer.tsx:320`, `Section`): 13px bold. */
const SECTION_HEADING = "m-0 text-[13px] font-bold text-ink-900";

/** The label–value pairs inside the `Hạn xử lý` cell: small label, value under it. */
const FACT_LIST = cn(
  "m-0 flex flex-col text-sm text-ink-900",
  "[&>dt]:text-[11px] [&>dt]:font-semibold [&>dt]:text-ink-500 [&>dt:not(:first-of-type)]:mt-1.5",
  "[&>dd]:m-0 [&_dd_p]:my-0 [&_dd_p]:mt-0.5",
);

/** The branch box (reason · receiving body · when): the prototype's `Lý do …` panel. */
const BRANCH_LIST = cn(
  "m-0 grid grid-cols-1 gap-x-4 gap-y-1 text-sm sm:grid-cols-[10rem_minmax(0,1fr)]",
  "[&>dt]:text-xs [&>dt]:font-semibold [&>dt]:text-ink-500 [&>dd]:m-0 [&>dd]:min-w-0 [&>dd]:text-ink-900",
);

/** One titled cell of the facts row (`FeedbackDetailDrawer.tsx:562-577`). */
function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 bg-surface px-4 py-2.5">
      <p className="m-0 mb-1 text-[11px] font-bold tracking-wide text-ink-500 uppercase">{label}</p>
      {children}
    </div>
  );
}

/**
 * One step of the status strip — the prototype's chip (`FeedbackStatusPipeline.tsx:112-153`): the
 * step's word on top, its role under it ("đang ở đây" / "đã qua" / "—"). The current step is filled
 * navy and carries `aria-current`; every step has its icon, so the state is never colour alone.
 */
function StepChip({
  label,
  role,
  icon,
}: {
  label: string;
  role: "current" | "done" | "ahead";
  icon?: LucideIcon;
}) {
  const Icon = icon ?? (role === "current" ? CircleDot : role === "done" ? CircleCheck : Circle);
  return (
    <li
      aria-current={role === "current" ? "step" : undefined}
      className={cn(
        "min-w-[6.5rem] flex-1 rounded-lg border border-solid px-2.5 py-1.5",
        role === "current"
          ? "border-transparent bg-brand-600 text-white"
          : role === "done"
            ? "border-line bg-surface text-ink-900"
            : "border-line bg-surface text-ink-500",
      )}
    >
      <span className="flex items-center gap-1 text-xs font-semibold">
        <Glyph icon={Icon} className="size-3.5 shrink-0" />
        {label}
      </span>
      <span className={cn("block text-[11px]", role === "current" ? "text-white/80" : "text-ink-500")}>
        {role === "current" ? "đang ở đây" : role === "done" ? "đã qua" : "—"}
      </span>
    </li>
  );
}

/**
 * Ô `Ghi chú nội bộ` TUỲ CHỌN của sáu thao tác. Chữ đi vào nhật ký xử lý, KHÔNG tới người dân —
 * khác hẳn `reason`/`result`, là hai câu người dân đọc. Trống là không gửi gì (`thanGhiChu`).
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
      <label htmlFor={id} className={LABEL_CLASS}>
        {NHAN_O_GHI_CHU_NOI_BO}
      </label>
      <textarea
        id={id}
        name={id}
        rows={2}
        className={cn(TEXTAREA_CLASS, loi !== null && "border-danger-600")}
        value={giaTri}
        onChange={(e) => datGiaTri(e.target.value)}
        aria-describedby={`${id}-dem`}
      />
      <p className={cn(HINT_CLASS, loi !== null && "text-danger-600")} id={`${id}-dem`} aria-live="polite">
        {demKyTu(giaTri)}/{GHI_CHU_TOI_DA} ký tự{loi !== null ? ` · ${loi}` : ""}
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
 * Biểu mẫu của MỘT nhánh rẽ. Tách ra để kiểm được bằng HTML tĩnh: ở trong `ChiTietPhieu` nó chỉ
 * hiện sau một cú bấm, thứ `renderToStaticMarkup` không bấm được.
 *
 * LÝ DO CHỈ SỐNG TRONG STATE CỦA KHỐI NÀY rồi đi vào thân POST. Không lưu nháp vào bộ nhớ trình
 * duyệt, không ghi console: đó là chữ về việc của một công dân (luật 3).
 *
 * Nút gửi bị khoá khi giới hạn phía client chưa đạt — nhưng giới hạn thật ở máy chủ, và câu 400
 * của nó vẫn ra nguyên văn nếu hai bên lệch nhau.
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
   * `coQuanTiepNhan` là chuỗi rỗng ở nhánh `khong-tiep-nhan` và bị bỏ qua. `ghiChu` là ghi chú nội
   * bộ tuỳ chọn — vào nhật ký, KHÔNG phải lý do người dân đọc.
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

  // THE SCREEN'S CONFIRM FLOW, IN THE SPEC'S CONFIRM FRAME (v2 §7): the title is the specific
  // question, the consequence sentence follows, the confirm button names the act. Same form, same
  // fields, same submit — `ConfirmDialog` only draws.
  return (
    <ConfirmDialog
      as="form"
      tone="danger"
      icon={chuyenCap ? ArrowUpRight : Ban}
      titleAs="h4"
      title={chuyenCap ? "Chuyển phiếu này lên cấp trên?" : "Không tiếp nhận phiếu này?"}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        if (hopLe) gui(lyDo, chuyenCap ? coQuan : "", ghiChu);
      }}
      actions={
        <>
          <button type="submit" className={buttonClass("primary")} disabled={dangGui || !hopLe}>
            <Glyph icon={chuyenCap ? ArrowUpRight : Ban} />
            {chuyenCap ? "Xác nhận chuyển cấp trên" : "Xác nhận không tiếp nhận"}
          </button>
          <button type="button" className={buttonClass("secondary")} onClick={huy} disabled={dangGui}>
            Huỷ
          </button>
        </>
      }
    >
      <p className="m-0 font-medium text-danger-600">{CANH_BAO_RE_NHANH}</p>

      <div>
        <label htmlFor={idLyDo} className={LABEL_CLASS}>
          {NHAN_O_LY_DO}
        </label>
        <textarea
          id={idLyDo}
          name={idLyDo}
          rows={4}
          className={cn(TEXTAREA_CLASS, loiO1 !== null && lyDo !== "" && "border-danger-600")}
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
    </ConfirmDialog>
  );
}
