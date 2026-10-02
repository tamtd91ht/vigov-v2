"use client";

import {
  AlarmClock,
  ArrowUp,
  ArrowUpRight,
  Ban,
  Building2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsRight,
  Circle,
  CircleAlert,
  CircleCheck,
  CircleDot,
  CloudOff,
  Construction,
  FileText,
  Forward,
  Inbox,
  Landmark,
  ListChecks,
  MapPin,
  Radio,
  RefreshCw,
  Search,
  SearchX,
  Shapes,
  ShieldX,
  Tags,
  UserRound,
  Workflow,
  X,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
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
import { FilterBar } from "@/components/ui/filter-bar";
import { NO_DRILL_DOWN, drillDownQuery, type DrillDown } from "@/lib/drill-down";
import { residentialUnitFilterLabel } from "@/features/cau-hinh/nhan-thon";

import {
  buocLuongChinh,
  buocReNhanh,
  AFTER_PHOTO_GO_TO,
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
  PHAN_CHUA_DUNG,
  PETITION_INTAKE_PERMISSION,
  petitionTaskOffered,
  phanLoaiDuoc,
  RE_NHANH,
  reNhanhDuoc,
  reopenLine,
  ratingStars,
  SCENE_LOCATION_LABEL,
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
  FIELD_LIST_CLASS,
  Glyph,
  HINT_CLASS,
  LABEL_CLASS,
  LoadingBar,
  PetitionRowsSkeleton,
  PetitionStatusBadge,
  SectionTitle,
  TEXTAREA_CLASS,
  TOGGLE_TRACK,
  toggleButtonClass,
} from "./petition-ui";
import { PetitionKpis } from "./petition-kpis";
import { afterPhotosHeadingId, ScenePhotos } from "./scene-photos";
import { StaffIntakeButton } from "./staff-intake";
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

/** The StatusStepper rows (§8.2): pills that wrap, each followed by its role in words. */
const STEP_LIST = "m-0 flex list-none flex-wrap gap-x-4 gap-y-2 p-0";
const STEP_ITEM = "inline-flex items-center gap-1 text-xs text-ink-500";

/** Selects of a processing act: label above, 40px, full column; two columns from 640px. */
const SELECT_GRID = "grid grid-cols-1 gap-4 sm:grid-cols-2";

/** Câu của dải lọc Tổng quan trên sổ này — nói ra đúng những gì màn tạm tắt. */
export const DRILL_DOWN_NOTE_CITIZEN_REPORTS =
  "Bộ lọc, ô tìm và phạm vi tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

export function SoPhanAnh({
  drillDown = NO_DRILL_DOWN,
}: {
  /** Lọc mở từ trang Tổng quan, đọc ở máy chủ (`app/phan-anh/page.tsx`). */
  drillDown?: DrillDown<"citizen-reports">;
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

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}`;

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
  };
  const listTable = (items: readonly petitions_phieuPhanAnhRa[]) => (
    <DanhSachThe
      phieu={items}
      tenBoPhan={tenBoPhan}
      bayGio={new Date()}
      maDangMo={dangMo?.code ?? null}
      moPhieu={openPetition}
      empty={
        noFilter ? (
          // NOT "the commune has no petition": a petition of a restricted field is absent from the
          // page for an account without `feedback.restricted`, and the screen must neither say it
          // exists nor claim it does not (rule 4, forbidden #2). "Nothing to show" is true for both.
          <EmptyState icon={Inbox} title="Chưa có phiếu phản ánh nào để hiển thị." />
        ) : undefined
      }
    />
  );

  return (
    // Direct children lose their legacy vertical margins: the section's `gap` is the one spacing
    // between blocks (spec §6.9, 16px card ↔ card).
    <section
      className="man-phan-anh mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0"
      aria-labelledby="tieu-de-so-phan-anh"
    >
      {/* Kept for the section's accessible name; the page's visible title is the `<h1>` above. */}
      <h2 id="tieu-de-so-phan-anh" className="an-thi-giac">
        Sổ phản ánh của xã
      </h2>

      {/* §2 header action `+ Nhập hộ phản ánh` — `feedback.create` only (UX; the server decides). Here
          rather than in the page's PageHeader because the session's keys are read in this component. */}
      {coQuyen(dsQuyen, PETITION_INTAKE_PERMISSION) && (
        <div className="flex justify-end">
          <StaffIntakeButton permissions={dsQuyen} onBooked={() => datLanTai((n) => n + 1)} />
        </div>
      )}

      {/* §3 — the four KPI cards. The route checks `feedback.read` AND `report.read`; without the
          second the row is not drawn at all (an account that may not read reports gets no figures). */}
      {coQuyen(dsQuyen, QUYEN_XEM_PHAN_ANH) && coQuyen(dsQuyen, REPORT_READ_PERMISSION) && <PetitionKpis />}

      <KhoiChuaDung />

      <DrillDownBanner
        drillDown={drillDown}
        clearHref="/phan-anh"
        note={DRILL_DOWN_NOTE_CITIZEN_REPORTS}
        showInvalid={!filtersChanged}
      />

      {/* THE LIST CARD (spec §8.5): the filter row at its head, the table in its own scroller, the
          paging in its footer. */}
      <Card className="overflow-visible">
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
            loads) or row-shaped placeholders (the first read). Dimmed rows are `inert`. */}
        {so.pha === "dangTai" && (
          <>
            <LoadingBar />
            <p className="an-thi-giac" role="status">
              {DANG_TAI_SO}
            </p>
            {staleItems !== null ? (
              <div inert className="pointer-events-none opacity-60 transition-opacity">
                {listTable(staleItems)}
              </div>
            ) : (
              <PetitionRowsSkeleton />
            )}
          </>
        )}

        {/* LOAD ERROR: the server's sentence VERBATIM stays the alert; `Tải lại` asks the same read
            again through the screen's existing re-read key (`lanTai`). */}
        {so.pha === "loi" && (
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
        )}

        {so.pha === "xong" && (
          <>
            {listTable(so.duLieu.items)}
            <CardFooter className="justify-end">
              <nav className="dieu-huong-trang m-0" aria-label="Phân trang sổ phản ánh">
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
          }}
          phanLoai={(linhVuc, ghiChu) => chay(phanLoaiPhieu(dangMo.code, linhVuc, ghiChu))}
          chuyenXuLy={(boPhanID, maCanBo, ghiChu) =>
            chay(chuyenXuLyPhieu(dangMo.code, boPhanID, maCanBo, ghiChu))
          }
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
 * Những phần đặc tả đòi mà hợp đồng không có — HIỆN LÊN ĐẦU MÀN, không giấu trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: danh sách dài hơn quyển sổ ở những ngày đầu, và một
 * bức tường chữ trên đầu màn hình là bức tường người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  // Collapsed, dashed, neutral grey (spec §8.1 — the same disclosure as Danh bạ): a list of what is
  // not built is not an alarm. The words and the list are unchanged.
  return (
    <details className="khoi-chua-khai group m-0">
      <summary className="flex cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden">
        <Glyph icon={Construction} className="size-[18px] shrink-0 text-ink-500" />
        <span className="font-semibold text-ink-700">
          {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý do
        </span>
        <Glyph
          icon={ChevronDown}
          className="ml-auto size-4 shrink-0 text-ink-500 transition-transform group-open:rotate-180"
        />
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

  // Filters behind the "Bộ lọc" button that are on — each is ABSENT from `loc` at its default, the
  // same rule the request builder uses, so "on" here is "sent to the server".
  const moreActiveCount = [
    loc.linhVuc !== undefined,
    loc.thonID !== undefined,
    loc.boPhanID !== undefined,
    loc.kenh !== undefined,
    loc.chiTreHan !== undefined,
    loc.ratingMax !== undefined,
  ].filter(Boolean).length;

  // Search first (owner, 02/10/2026: "ô tìm kiếm nên nằm ở bên trái"), then the scope tabs and the
  // status — what decides which petitions are in front of you; the six narrower filters sit behind
  // "Bộ lọc".
  return (
    <FilterBar
      id="petition-filters"
      className="m-0 border-b border-line px-4 py-3.5"
      moreActiveCount={moreActiveCount}
      primary={
        <>
          {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
              đang gõ vào một URL — và chuỗi ấy có thể là tên hay địa chỉ một công dân (luật 3, cấm #4).
              Gõ xong rồi bấm là một lần. */}
          <form
            className="form-tra-cuu m-0 flex min-w-0 max-w-[480px] flex-[1_1_320px] flex-row items-end gap-2"
            onSubmit={timNgay}
            role="search"
          >
            <Field label="Tìm trong sổ" htmlFor="tim-phan-anh" icon={Search} grow="search" className="max-w-none">
              <input
                id="tim-phan-anh"
                name="tim-phan-anh"
                value={tim}
                placeholder={TIM_PLACEHOLDER}
                onChange={(e) => datTim(e.target.value)}
                autoComplete="off"
                // Máy chủ trả 400 khi quá 200 ký tự (`store.TimPhieuToiDa`). Chặn ở ô nhập để cán bộ
                // thấy giới hạn thay vì thấy "không tải được".
                maxLength={200}
              />
            </Field>
            <Button variant="secondary" type="submit">
              Tìm
            </Button>
          </form>

          {/* HAI TAB PHẠM VI, CÙNG KHUÔN SỔ NHIỆM VỤ. `mine` KHÔNG mang danh tính nào — máy chủ lấy mã
              cán bộ từ PHIÊN. Tab `Liên quan đến tôi` không vẽ: máy chủ trả 400 cho `scope=related`
              (xem `PHAN_CHUA_DUNG`). Drawn as a segmented control; the buttons stay native
              `aria-pressed` toggles, their text the only child (tests read it). */}
          <div className="flex flex-col gap-1.5">
            <span aria-hidden="true" className="text-xs leading-tight font-semibold text-ink-700">
              Phạm vi
            </span>
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
            </div>
          </div>

          <Field label="Trạng thái" htmlFor="loc-trang-thai" icon={CircleDot} kind="select">
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
        </>
      }
      more={
        <>
          <Field label="Lĩnh vực" htmlFor="loc-linh-vuc" icon={Shapes} kind="select">
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

          <Field label="Địa bàn" htmlFor="loc-dia-ban" icon={MapPin} kind="select">
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

          <Field label="Bộ phận đang giữ" htmlFor="loc-bo-phan" icon={Building2} kind="select">
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

          <Field label="Kênh tiếp nhận" htmlFor="loc-kenh" icon={Radio} kind="select">
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

          <div className="o-chon">
            <label htmlFor="loc-tre-han">
              <input
                id="loc-tre-han"
                type="checkbox"
                checked={loc.chiTreHan === true}
                // Ô bỏ tích thì tham số VẮNG MẶT HẲN, không gửi `late=false` — máy chủ chỉ nhận đúng
                // chuỗi `true` và trả 400 cho mọi giá trị khác.
                onChange={(e) => datLoc({ ...loc, chiTreHan: e.target.checked ? true : undefined })}
              />{" "}
              {CHI_TRE_HAN_NHAN}
            </label>
          </div>

          <div className="o-chon">
            <label htmlFor="loc-danh-gia-thap">
              <input
                id="loc-danh-gia-thap"
                type="checkbox"
                checked={loc.ratingMax === LOW_RATING_MAX}
                // Spec §4 "phiếu 1–2 sao" → `rating_max=2`. Unticked, the parameter is ABSENT — never an
                // empty `rating_max=`, which the server answers with 400.
                onChange={(e) =>
                  datLoc({ ...loc, ratingMax: e.target.checked ? LOW_RATING_MAX : undefined })
                }
              />{" "}
              {LOW_RATING_FILTER_LABEL}
            </label>
          </div>
        </>
      }
    />
  );
}

/**
 * The list table inside the list card (spec v2 §6.7, §8.5): flush with the card (no second frame),
 * its OWN scroller both ways so the header row can stay put (`sticky` inside the scroller), 48px
 * rows, thin horizontal rules only — `.bang-danh-muc` already draws those.
 */
const TABLE_IN_CARD = cn(
  "bang-cuon max-h-[70vh] overflow-auto rounded-none border-0 shadow-none",
  "[&_thead_th]:sticky [&_thead_th]:top-0 [&_thead_th]:z-[1] [&_thead_th]:shadow-[inset_0_-1px_0_var(--line)]",
  "[&_tbody_td]:h-12",
);

/**
 * Quyển sổ — một BẢNG từ đợt làm mới giao diện (ADR 0068 §8, §11: đặc tả v2 thắng §2 của chương 09
 * về hình thức; §2 vẽ danh sách thẻ). Tên `DanhSachThe` giữ nguyên vì nó là tên đã xuất.
 *
 * `empty` — what an empty page shows. The screen passes "nothing to show" or leaves the filter
 * sentence `SO_RONG` (spec §8b), a choice it makes from its own filter state; this table never guesses.
 */
export function DanhSachThe({
  phieu,
  tenBoPhan,
  bayGio,
  maDangMo,
  moPhieu,
  empty,
}: {
  phieu: readonly petitions_phieuPhanAnhRa[];
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moPhieu: (p: petitions_phieuPhanAnhRa) => void;
  empty?: ReactNode;
}) {
  if (phieu.length === 0) return empty ?? <EmptyState icon={SearchX} title={SO_RONG} />;

  return (
    <div className={TABLE_IN_CARD} role="region" aria-label="Danh sách phiếu phản ánh" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            <th scope="col">Mã phiếu</th>
            <th scope="col">Nội dung</th>
            <th scope="col">Lĩnh vực</th>
            <th scope="col">Người gửi</th>
            <th scope="col">Bộ phận đang giữ</th>
            <th scope="col">Kênh tiếp nhận</th>
            <th scope="col">Hạn xử lý xong</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">
              <span className="an-thi-giac">Thao tác</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {phieu.map((p) => (
            <ThePhieu
              key={p.code}
              phieu={p}
              tenBoPhan={tenBoPhan}
              bayGio={bayGio}
              dangMo={p.code === maDangMo}
              mo={() => moPhieu(p)}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Một dòng phiếu (§7) — mọi trường của thẻ cũ, nay là một hàng của bảng. Tên `ThePhieu` giữ nguyên.
 *
 * NO THUMBNAIL, ON PURPOSE: the scene photos are read per petition (`GET …/photos`, opened in the
 * detail — `ScenePhotos`). A thumbnail per row would be one call per row, each signing fresh links
 * over a citizen's photographs (rule 3) for twenty petitions nobody opened. Nor is a placeholder drawn
 * that looks like it is still loading.
 *
 * THE CONTENT IS CUT TO ONE LINE, WITH NO `title` TOOLTIP: it is a citizen's words (rule 3 keeps
 * personal data out of attributes); the full text is in the detail.
 */
export function ThePhieu({
  phieu,
  tenBoPhan,
  bayGio,
  dangMo,
  mo,
}: {
  phieu: petitions_phieuPhanAnhRa;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  dangMo: boolean;
  mo: () => void;
}) {
  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  // A comparison of the stored deadline with now (`trangThaiHan`), recomputed on every render —
  // never kept (rule 10, invariant 3).
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);
  const pastDeadline = hanXuLy.loai === "quaHan";

  return (
    <tr
      // The row tint follows the same comparison as the `Hạn xử lý xong` cell, so the two never differ.
      data-tre-han={pastDeadline ? "" : undefined}
      className={cn(pastDeadline && "dong-qua-han", dangMo && "bg-brand-50")}
    >
      <td className="ma-muc">{phieu.code}</td>
      <td className="min-w-[16rem] max-w-[26rem] whitespace-normal">
        {/* Nội dung phản ánh KHÔNG che (cán bộ không đọc được thì không xử lý được), nhưng nó là
            chữ của một công dân: không bao giờ ghi nó vào log, tên tệp hay URL. */}
        <span className="block truncate">{phieu.content}</span>
        <span className="dong-phu truncate">
          <Glyph icon={MapPin} className="mr-1 inline size-3.5 align-[-2px]" />
          {phieu.address === "" ? "Chưa rõ vị trí" : phieu.address}
        </span>
      </td>
      <td>{nhanLinhVuc(linhVuc)}</td>
      <td>{nhanNguoiGui(phieu)}</td>
      <td>{nhanBoPhan(phieu.unit, tenBoPhan)}</td>
      <td>{nhanKenh(phieu.channel)}</td>
      <td>
        {/* The words are `nhanHan`'s ("Quá hạn · hạn cuối …"); AlarmClock beside them, never instead. */}
        <span className={cn(lopHan(hanXuLy), "inline-flex items-center gap-1")}>
          {pastDeadline && <Glyph icon={AlarmClock} className="size-3.5 shrink-0" />}
          {nhanHan(hanXuLy)}
        </span>
      </td>
      <td>
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <PetitionStatusBadge status={phieu.status}>{nhanTrangThai(phieu.status)}</PetitionStatusBadge>
          {/* §7 corner: the citizen's stars once rated. */}
          {phieu.rating !== undefined && phieu.rating !== null && (
            <span
              className={phieu.rating <= LOW_RATING_MAX ? "chip nhan-lech" : "chip"}
              role="img"
              aria-label={starsLabel(phieu.rating)}
            >
              {ratingStars(phieu.rating)}
            </span>
          )}
        </span>
      </td>
      <td className="o-thao-tac text-right">
        <Button type="button" variant="secondary" size="sm" onClick={mo} aria-expanded={dangMo}>
          {dangMo ? "Đang mở phiếu này" : `Mở phiếu ${phieu.code}`}
        </Button>
      </td>
    </tr>
  );
}

/**
 * Khối chi tiết §8, kèm bốn thao tác.
 *
 * ĐẶC TẢ GỌI NÓ LÀ DetailDrawer "mở gần toàn màn hình". Ở đây nó là một khối nằm dưới danh sách —
 * KHÔNG phải một lớp phủ — vì một lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này
 * không được thêm CSS. Đã báo về tên lớp cần thêm.
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
  khongTiepNhan: (lyDo: string, ghiChu: string) => KetQuaGui;
  chuyenCapTren: (lyDo: string, coQuanTiepNhan: string, ghiChu: string) => KetQuaGui;
  /**
   * PUT …/publication. Optional so a caller without a write path renders the box read-only; the
   * buttons also need `cong.moderate`.
   */
  setPublication?: (target: PublicationTarget) => KetQuaGui;
}) {
  const [linhVucChon, datLinhVucChon] = useState("");
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
    // THE DRAWER IS A COLUMN OF CARDS (spec v2 §7: sections, not one wall of fields): the header,
    // the steps, the record, the photos, the rating, the processing acts, the log. Still a block
    // under the list, not an overlay — no new CSS this round.
    <section className="flex min-w-0 flex-col gap-4" aria-labelledby="tieu-de-chi-tiet-phieu">
      <Card>
        <CardHeader className="flex-nowrap items-start">
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-1.5">
            <h3 id="tieu-de-chi-tiet-phieu" className="m-0 text-base leading-snug font-semibold text-ink-900">
              <span className="ma-muc">{phieu.code}</span> · {nhanKenh(phieu.channel)} ·{" "}
              {nhanThoiDiem(phieu.booked_at)}
            </h3>
            <PetitionStatusBadge status={phieu.status}>{nhanTrangThai(phieu.status)}</PetitionStatusBadge>
          </div>
          <IconButton label="Đóng chi tiết phiếu" type="button" variant="secondary" onClick={dong}>
            <Glyph icon={X} />
          </IconButton>
        </CardHeader>

        {/* StatusStepper §8.2 — bảy ô luồng chính, rồi hai ô rẽ nhánh sáng theo trạng thái. Không ô
            nào bấm được: hai nhánh đi qua biểu mẫu riêng bên dưới vì phải mang lý do (`buocReNhanh`).
            Each step: icon + word in the pill, then its role in words — never colour alone. */}
        <CardContent className="flex flex-col gap-3">
          <SectionTitle icon={Workflow}>Các bước xử lý</SectionTitle>
          <ol aria-label="Các bước xử lý phiếu" className={STEP_LIST}>
            {buocLuongChinh(phieu.status).map((o) => (
              <li key={o.ma} className={STEP_ITEM}>
                <Badge
                  tone={o.vaiTro === "dangODay" ? "info" : o.vaiTro === "daQua" ? "success" : "neutral"}
                  icon={o.vaiTro === "dangODay" ? CircleDot : o.vaiTro === "daQua" ? CircleCheck : Circle}
                  className={o.vaiTro === "dangODay" ? "ring-1 ring-brand-500" : undefined}
                >
                  {o.nhan}
                </Badge>{" "}
                {o.vaiTro === "dangODay" ? "đang ở đây" : o.vaiTro === "daQua" ? "đã qua" : "—"}
              </li>
            ))}
          </ol>
          <ol aria-label="Rẽ nhánh" className={STEP_LIST}>
            {buocReNhanh(phieu.status).map((o) => (
              <li key={o.ma} className={STEP_ITEM}>
                <Badge
                  tone={o.vaiTro === "dangODay" ? "info" : "neutral"}
                  icon={o.ma === "khong-tiep-nhan" ? Ban : ArrowUpRight}
                  className={o.vaiTro === "dangODay" ? "ring-1 ring-brand-500" : undefined}
                >
                  {o.nhan}
                </Badge>{" "}
                {o.vaiTro === "dangODay" ? "đang ở đây" : "—"}
              </li>
            ))}
          </ol>
          {giaiThich !== null && <p className="ghi-chu m-0">{giaiThich}</p>}
        </CardContent>
      </Card>

      <Card as="section" aria-labelledby="tieu-de-thong-tin-phieu">
        <CardHeader>
          <SectionTitle icon={FileText} id="tieu-de-thong-tin-phieu">
            Thông tin phiếu
          </SectionTitle>
        </CardHeader>
        <CardContent className="py-1">
      <dl className={FIELD_LIST_CLASS}>
        <dt>Lĩnh vực</dt>
        <dd>{nhanLinhVuc(linhVuc)}</dd>

        <dt>Người gửi</dt>
        <dd>{nhanNguoiGui(phieu)}</dd>

        <dt>Nội dung</dt>
        <dd className="noi-dung-phan-anh">{phieu.content}</dd>

        <dt>{SCENE_LOCATION_LABEL}</dt>
        <dd>
          <SceneLocation petition={phieu} />
        </dd>

        <dt>Người dân gửi lúc</dt>
        <dd>{nhanThoiDiem(phieu.clock_from)}</dd>

        <dt>Hạn tiếp nhận</dt>
        <dd>
          <span className={lopHan(hanTiepNhan)}>{nhanHan(hanTiepNhan)}</span>
        </dd>

        <dt>Trần phân loại</dt>
        <dd>
          <span className={lopHan(hanPhanLoai)}>{nhanHan(hanPhanLoai)}</span>
        </dd>

        <dt>Hạn xử lý xong</dt>
        <dd>
          <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
          {/* Requirement `FeedbackDetailDrawer.tsx:242-246`. The deadline is NOT recomputed on a
              reopening (ADR 0050 point 2) — this line is what explains an old deadline on a
              petition that is back in progress. */}
          {reopened !== null && <p className="nhan-lech">{reopened}</p>}
        </dd>

        <dt>Đang giao cho</dt>
        <dd>
          {nhanBoPhan(phieu.unit, tenBoPhan)} · {nhanCanBoXuLy(phieu.assignee, bangDanhBa)}
        </dd>

        <dt>Hiển thị với người dân</dt>
        <dd>
          <PublicationBox
            petition={phieu}
            mayModerate={cong.moderate}
            busy={dangGui}
            setPublication={
              setPublication === undefined ? undefined : (target) => void setPublication(target)
            }
          />
        </dd>

        {/* Kết quả CHỈ hiện khi đã có: một ô trống ở đây trông như một trường chưa điền, trong khi
            phiếu chưa đóng thì nó chưa tồn tại. */}
        {phieu.result !== "" && (
          <>
            <dt>Kết quả xử lý</dt>
            <dd>{phieu.result}</dd>
          </>
        )}

        {/* LÝ DO VÀ CƠ QUAN NHẬN CHỈ CÓ Ở HAI NHÁNH RẼ. Ở trạng thái khác hai trường không tồn tại —
            vẽ ô trống là vẽ một trường trông như chưa điền. */}
        {RE_NHANH.includes(phieu.status) && (
          <>
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
          </>
        )}
      </dl>
        </CardContent>
      </Card>

      {/* Right under the list, so a refusal of the publication buttons above (409 `never_public`)
          reads next to the box that caused it. The `<p>` itself is unchanged (a test reads it); the
          frame around it is the spec's error look. */}
      {loiGhi !== null && (
        <div className="flex items-start gap-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 [&>p]:m-0">
          <Glyph icon={CircleAlert} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
          <p className="thong-bao-loi" role="alert">
            {loiGhi}
          </p>
        </div>
      )}

      {/* §8.4 photos. A separate component, NOT a hook here: it reads the network (`useEffect`), and
          `chon-can-bo.test.tsx` calls this block as a plain function. Hidden without `feedback.read` —
          UX only; the route checks the same key and `feedback.restricted` (rule 5, forbidden #1). */}
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
      {cong.phanCong ? (
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

      {/* ── 5. NHẬT KÝ XỬ LÝ (§8.7) ─────────────────────────────────────────────────────── */}
      {/* Một thành phần riêng, KHÔNG phải thêm hook vào đây: nó tự đọc mạng (`useEffect`), và
          `chon-can-bo.test.tsx` gọi khối này như một hàm thường — một `useEffect` ở đây sẽ nổ. */}
      <NhatKyPhieu
        maTraCuu={phieu.code}
        tenBoPhan={tenBoPhan}
        danhBa={bangDanhBa}
        coNutGhi={coGhiNhatKy}
        lanLamMoi={lanLamMoiNhatKy}
      />
    </section>
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
