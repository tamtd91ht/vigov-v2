"use client";

import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  CircleMinus,
  CloudOff,
  Eye,
  FilePlus2,
  ListChecks,
  LoaderCircle,
  LockKeyhole,
  Pencil,
  Plus,
  RefreshCw,
  Scissors,
  Signature,
  Stamp,
  Trash2,
  Undo2,
  X,
  type LucideIcon,
} from "lucide-react";
import {
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { OChonCanBo } from "@/components/o-chon-can-bo";
import {
  Button,
  buttonVariants,
  LEGACY_BUTTON_CLASS,
  type ButtonVariant,
} from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { taskDetailHref } from "@/features/nhiem-vu/task-link";
import { Glyph, TaskStatusBadge } from "@/features/nhiem-vu/task-ui";
import { cn } from "@/lib/cn";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import {
  docBangNhanTrangThai,
  nhanNgay,
  nhanTrangThai,
  type BangNhanTrangThai,
} from "@/features/nhiem-vu/nhan-nhiem-vu";
import { FormGiaoViec, type DanhMucNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { danhBaTheoMa, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import { usePhien } from "@/features/phien/phien-hien-tai";
import {
  boDauKhongPhatSinh,
  danhDauKhongPhatSinh,
  kyBienBan,
  layBienBan,
  layNhiemVuCuaKetLuan,
  laySoBienBan,
  suaBienBan,
  suaKetLuan,
  tachKetLuanThanhNhiemVu,
  taoBienBan,
  themKetLuan,
  xoaBienBan,
  xoaKetLuan,
  type SuaBienBanVao,
  type TachKetLuanVao,
  type TaoBienBanVao,
  type ThongBaoVao,
} from "@/lib/api/bien-ban";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_canBoChonNguoiRa,
  identity_danhBaChonNguoiRa,
  page_Result_petitions_bienBanRa,
  petitions_bienBanRa,
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_ketLuanRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";
import { coQuyen, QUYEN_DUYET_GIA_HAN, QUYEN_KY_BIEN_BAN } from "@/lib/quyen";

import {
  BIEU_MAU_TRONG,
  CANH_BAO_BI_MAT,
  CAU_THONG_BAO_MOT_LAN,
  CAU_XAC_NHAN_KY,
  CREATE_MEETING_DESCRIPTION,
  CREATE_MEETING_TITLE,
  DANG_TAI_SO,
  DIA_DIEM_TOI_DA,
  dongMeta,
  giaTriTuBienBan,
  idTuNeo,
  KET_LUAN_MOI_LAN_TOI_DA,
  KHONG_GHI_CAN_BO,
  laDaKy,
  LY_DO_XOA_TOI_DA,
  MEETING_DATE_REQUIRED,
  MEETING_SAVED_TOAST,
  MEETING_TITLE_REQUIRED,
  MO_TA_MAN,
  neoBienBan,
  NGUON_GIAO_KHOA,
  NHAN_NUT_BO_DAU,
  NHAN_NUT_BO_SUNG,
  NHAN_NUT_DANH_DAU,
  NHAN_NUT_DONG_BIEN_BAN,
  NHAN_NUT_GHI_THONG_BAO,
  NHAN_NUT_GO_KL,
  NHAN_NUT_HUY,
  NHAN_NUT_KY,
  NHAN_NUT_LUU,
  NHAN_NUT_LUU_SUA,
  NHAN_NUT_NHAP_BIEN_BAN,
  NHAN_NUT_SUA_BIEN_BAN,
  NHAN_NUT_SUA_KL,
  NHAN_NUT_TACH,
  NHAN_NUT_THEM_KET_LUAN,
  NHAN_NUT_XAC_NHAN_KY,
  NHAN_NUT_XEM_BIEN_BAN,
  NHAN_NUT_XOA_BIEN_BAN,
  nhanBadge,
  nhanCanBo,
  nhanNgayHop,
  nhanNutTach,
  nhanNutXemNhiemVu,
  nhanThanhPhan,
  nhanThongBao,
  nhanTienDoBienBan,
  nhanTienDoKetLuan,
  nhanTrangThaiBienBan,
  nhanTrangThaiKetLuan,
  NOI_DUNG_BIEN_BAN_TOI_DA,
  NOI_DUNG_KET_LUAN_TOI_DA,
  PLACEHOLDER_KET_LUAN,
  quyTacBienBan,
  quyTacKetLuan,
  SO_HIEU_TOI_DA,
  SO_RONG,
  SO_THONG_BAO_TOI_DA,
  SPLIT_DIALOG_TITLE,
  SPLIT_DONE_TOAST,
  SPLIT_SUBMIT_LABEL,
  soThuTuKetLuan,
  TEN_CUOC_HOP_TOI_DA,
  thanSuaTuBieuMau,
  thanTaoTuBieuMau,
  thanThongBao,
  TIEU_DE_MAN,
  type GiaTriBieuMau,
  type QuyTacNut,
} from "./nhan-bien-ban";
import { ScanAttachmentField, SuggestedDeadlineHint } from "./meeting-pending";
import { ConclusionOrdinal, ConclusionStatusBadge, MeetingStatusBadge } from "./meeting-ui";

/**
 * Sổ Biên bản và kết luận họp — `docs/ui-ux/04-bien-ban-hop.md` §2 (thẻ), §3 (tách), §4 (biểu
 * mẫu), §7 (quy tắc), cộng vòng đời dự thảo → đã ký của migration 0012.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA ĐIỀU MÀ MÁY CHỦ ĐÃ QUYẾT VÀ MÀN HÌNH NÀY PHẢI THEO — cả ba đều dễ bị "sửa cho gọn":
 *
 *   SỐ THỨ TỰ KẾT LUẬN     nối tiếp số ĐÃ CẤP, không đếm lại số dòng còn sống. Gỡ ② thì kế tiếp
 *                          là ④, và khoảng trống ấy ĐÚNG (luật 7, bất biến 3). Màn hình vẽ
 *                          `ordinal` máy chủ trả, không bao giờ vẽ `index + 1`.
 *   BIÊN BẢN KHÔNG CÓ MÃ   `so_hieu` gõ tay, tuỳ chọn, KHÔNG duy nhất. Nó là một thông tin trong
 *                          dòng meta, không phải một mã định danh.
 *   TRẠNG THÁI KẾT LUẬN    do MÁY CHỦ suy (chưa giao · đang thực hiện · quá hạn · hoàn thành).
 *                          Màn hình chỉ đổi mã thành chữ; "quá hạn" không được tính ở hai nơi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG QUYỀN DUY NHẤT Ở CLIENT LÀ NÚT KÝ (`task.approve`), và nó là TIỆN DỤNG. Dịch vụ `petitions`
 * kiểm quyền trên TỪNG lời gọi; tài khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra màn hình
 * (luật 5, cấm #1). Mọi nút khác (nhập, sửa, gỡ, đánh dấu, tách) KHÔNG có cổng `task.create` ở
 * client, đúng khuôn màn Văn bản: ẩn một nút chưa bao giờ là biện pháp, và thiếu nó chỉ tốn một lần
 * bấm. Đây là lựa chọn, không phải phần còn thiếu — nên không có dấu "?" nào cho nó (ADR 0068 §14).
 *
 * NỘI DUNG BIÊN BẢN LÀ CHỮ CỦA MỘT CUỘC HỌP CÓ THỂ NHẮC TỚI HỒ SƠ CÔNG DÂN: không dòng nào ở đây
 * ghi nó vào log, vào storage hay vào một URL (luật 3, cấm #1 và #4). URL chỉ mang id biên bản —
 * một ULID mờ — và chỉ sau dấu `#`, phần không bao giờ rời trình duyệt.
 */

/** Bao nhiêu thẻ một trang. Mỗi thẻ mang cả danh sách kết luận, nên trang mỏng hơn quyển sổ phản ánh. */
const SO_THE_MOI_TRANG = 10;

/** Chưa đọc được danh mục nào thì các ô chọn của biểu mẫu Giao việc rỗng — xem `PhepTach`. */
const KHONG_DANH_MUC: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/**
 * Mọi thứ luồng "Tách thành nhiệm vụ" (§3) cần, gom thành MỘT đối tượng đi qua ba tầng thành phần.
 *
 * MỘT HỘP GIAO VIỆC MỘT LÚC TRÊN CẢ MÀN (`moOKetLuan` là một id, không phải một tập): hai biểu mẫu
 * mở cùng lúc là hai khoá chống trùng sống song song trên cùng một màn.
 */
export type PhepTach = {
  /** Bốn danh mục đổ vào ô chọn của biểu mẫu Giao việc. */
  readonly danhMuc: DanhMucNhiemVu;
  /**
   * Câu trả lời NGUYÊN VẸN của danh bạ chọn người — `null` = chưa đọc xong. Ba ô chọn cán bộ của
   * biểu mẫu Giao việc cần phân biệt "đang tải" với "tải hỏng" để nói đúng câu (xem `FormGiaoViec`).
   */
  readonly danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  /** Danh bạ đã lọc `permission=task.extend` cho ô `Lãnh đạo giao việc` — xem `FormGiaoViec`. */
  readonly danhBaLanhDao: KetQua<identity_danhBaChonNguoiRa> | null;
  readonly dangGui: boolean;
  /** Id của kết luận đang mở hộp, hoặc `null`. */
  readonly moOKetLuan: string | null;
  readonly loi: { readonly ketLuanID: string; readonly thongBao: string } | null;
  readonly mo: (ketLuanID: string) => void;
  readonly dong: () => void;
  /**
   * ⚠ NHẬN CẢ DÒNG KẾT LUẬN, KHÔNG NHẬN MỘT CON SỐ. `{stt}` trên đường dẫn phải là `ordinal` máy
   * chủ trả; một tham số `thuTu: number` ở đây là chỗ `viTri + 1` đi lọt vào mà không có gì đỏ.
   */
  readonly gui: (
    bienBanID: string,
    ketLuan: petitions_ketLuanRa,
    than: TachKetLuanVao,
    khoaChongTrung: string,
  ) => void;
};

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

/** Những hộp nhỏ mở ngay dưới một thẻ hay một dòng kết luận. Một hộp một lúc trên cả màn. */
export type LoaiHop = "ky" | "thong-bao" | "xoa" | "sua-kl" | "go-kl";

/**
 * Mọi thứ VÒNG ĐỜI biên bản cần (xem, sửa, ký, ghi Thông báo, gỡ, bổ sung; sửa/gỡ/đánh dấu một
 * kết luận; mở danh sách nhiệm vụ đã tách), gom thành một đối tượng — cùng lý do với `PhepTach`.
 *
 * `dich` LÀ ID BẢN GHI MÀ MỘT HỘP HAY MỘT CÂU LỖI NÓI VỀ — id biên bản hoặc id kết luận (ULID, không
 * trùng nhau). Một câu từ chối của kết luận ③ hiện trên dòng ① là nói với cán bộ rằng họ vừa làm
 * hỏng một việc họ không đụng tới.
 */
export type PhepVongDoi = {
  readonly dangGui: boolean;
  /** Phiên có `task.approve` — CHỈ để ẩn nút Ký; máy chủ vẫn kiểm. */
  readonly coQuyenKy: boolean;
  /** `null` khi danh bạ chưa đọc được — mọi mã cán bộ khi ấy hiện nguyên mã. */
  readonly danhBa: DanhBaTheoMa | null;
  readonly nhanTT: BangNhanTrangThai;
  readonly hop: { readonly dich: string; readonly loai: LoaiHop } | null;
  readonly loi: { readonly dich: string; readonly thongBao: string } | null;
  /** Biên bản đang mở "Xem biên bản", đọc qua tuyến chi tiết. */
  readonly chiTiet: { readonly id: string; readonly tai: TrangThaiTai<petitions_bienBanRa> } | null;
  /** Danh sách nhiệm vụ đã tách, theo id kết luận. CÓ KHOÁ = đang mở. */
  readonly nhiemVuKL: ReadonlyMap<string, TrangThaiTai<readonly petitions_nhiemVuRa[]>>;
  readonly moHop: (dich: string, loai: LoaiHop) => void;
  readonly dongHop: () => void;
  readonly dongChiTiet: () => void;
  readonly moSua: (bb: petitions_bienBanRa) => void;
  readonly moBoSung: (bb: petitions_bienBanRa) => void;
  readonly ky: (bb: petitions_bienBanRa, thongBao: ThongBaoVao | null) => void;
  readonly ghiThongBao: (bb: petitions_bienBanRa, thongBao: ThongBaoVao) => void;
  readonly xoa: (bb: petitions_bienBanRa, lyDo: string) => void;
  readonly suaKL: (bb: petitions_bienBanRa, kl: petitions_ketLuanRa, noiDung: string) => void;
  readonly goKL: (bb: petitions_bienBanRa, kl: petitions_ketLuanRa, lyDo: string) => void;
  readonly danhDau: (bb: petitions_bienBanRa, kl: petitions_ketLuanRa) => void;
  readonly boDau: (bb: petitions_bienBanRa, kl: petitions_ketLuanRa) => void;
  readonly batNhiemVuKL: (bb: petitions_bienBanRa, kl: petitions_ketLuanRa) => void;
};

/** Biểu mẫu biên bản đang mở: nhập mới (có thể là bổ sung cho một biên bản đã ký), hoặc sửa nháp. */
export type CheBieuMau =
  | { readonly loai: "tao"; readonly boSungCho: petitions_bienBanRa | null }
  | { readonly loai: "sua"; readonly ban: petitions_bienBanRa };

/* ── Neo `#bien-ban-{id}` là nguồn DUY NHẤT của "biên bản nào đang mở" ─────────────────────────
 *
 * Đọc qua `useSyncExternalStore` chứ không chép vào state: hai nguồn cho một sự thật trôi khỏi nhau
 * đúng lúc cán bộ bấm "Quay lại" của trình duyệt.
 */
function dangKyNeo(baoDoi: () => void): () => void {
  window.addEventListener("hashchange", baoDoi);
  return () => window.removeEventListener("hashchange", baoDoi);
}

function docNeo(): string {
  return window.location.hash;
}

function docNeoMayChu(): string {
  return "";
}

/** Bảng mới THIẾU một khoá — không sửa bảng cũ: state của React phải là một giá trị mới. */
function boKhoa<V>(bang: ReadonlyMap<string, V>, khoa: string): ReadonlyMap<string, V> {
  return new Map([...bang].filter(([k]) => k !== khoa));
}

export function SoBienBan() {
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_bienBanRa>;
  } | null>(null);

  const [bieuMau, datBieuMau] = useState<CheBieuMau | null>(null);
  const [dangGui, datDangGui] = useState(false);
  const [loiBieuMau, datLoiBieuMau] = useState<string | null>(null);
  const [loiKetLuan, datLoiKetLuan] = useState<{ bienBanID: string; thongBao: string } | null>(
    null,
  );
  // The card whose `Thêm kết luận` is in flight — only THAT button spins (prototype `isPending`);
  // `dangGui` still disables every write on the screen, as before.
  const [addingTo, setAddingTo] = useState<string | null>(null);

  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của hàng thêm kết luận và của biểu mẫu, nên một lần
  // ghi xong là một khoá chống trùng mới. Lần ghi HỎNG không tăng — giữ chữ đã gõ VÀ khoá cũ.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // §3 — luồng Tách. Tất cả mang ID KẾT LUẬN.
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);
  const [moTachO, datMoTachO] = useState<string | null>(null);
  const [loiTach, datLoiTach] = useState<{ ketLuanID: string; thongBao: string } | null>(null);

  // Vòng đời.
  const [kqDanhBa, datKqDanhBa] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [kqDanhBaLanhDao, datKqDanhBaLanhDao] =
    useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [kqNhanTT, datKqNhanTT] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(
    null,
  );
  const [hop, datHop] = useState<PhepVongDoi["hop"]>(null);
  const [loiVD, datLoiVD] = useState<PhepVongDoi["loi"]>(null);
  const [luotChiTiet, datLuotChiTiet] = useState(0);
  const [daTaiChiTiet, datDaTaiChiTiet] = useState<{
    khoa: string;
    kq: KetQua<petitions_bienBanRa>;
  } | null>(null);
  const [nhiemVuKL, datNhiemVuKL] = useState<
    ReadonlyMap<string, TrangThaiTai<readonly petitions_nhiemVuRa[]>>
  >(new Map());

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào (luật 1, cấm #1).
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coQuyenKy = coQuyen(dsQuyen, QUYEN_KY_BIEN_BAN);

  const neo = useSyncExternalStore(dangKyNeo, docNeo, docNeoMayChu);
  const idChiTiet = idTuNeo(neo);

  const khoa = `${nganXep.hienTai ?? ""}|${lanTai}`;
  const khoaChiTiet = `${idChiTiet ?? ""}|${luotChiTiet}`;

  useEffect(() => {
    let bo = false;
    laySoBienBan({ limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nganXep.hienTai, khoa]);

  // "Xem biên bản" đọc qua tuyến CHI TIẾT: danh sách cố ý không trả toàn văn và thành phần.
  useEffect(() => {
    if (idChiTiet === null) return;
    let bo = false;
    layBienBan(idChiTiet).then((kq) => {
      if (!bo) datDaTaiChiTiet({ khoa: khoaChiTiet, kq });
    });
    return () => {
      bo = true;
    };
  }, [idChiTiet, khoaChiTiet]);

  /**
   * DANH MỤC CỦA BIỂU MẪU GIAO VIỆC, DANH BẠ CHỌN NGƯỜI VÀ NHÃN TRẠNG THÁI NHIỆM VỤ — đọc một lần
   * cho cả màn. Mỗi thứ hỏng thì chỉ phần của nó hỏng, KHÔNG làm hỏng quyển sổ.
   */
  useEffect(() => {
    let bo = false;
    Promise.all([
      layLoaiNhiemVu(),
      layMucUuTienNhiemVu(),
      layKhoiNhiemVu(),
      layDanhMucBoPhan(),
    ]).then(([loai, uuTien, khoiNV, boPhan]) => {
      if (bo) return;
      datDanhMuc({
        loai: loai.ok ? loai.duLieu.items : [],
        mucUuTien: uuTien.ok ? uuTien.duLieu.items : [],
        khoi: khoiNV.ok ? khoiNV.duLieu.items : [],
        boPhan: boPhan.ok ? boPhan.duLieu.items : [],
      });
    });
    layDanhBaChonNguoi().then((kq) => {
      if (!bo) datKqDanhBa(kq);
    });
    // Ô `Lãnh đạo giao việc` của biểu mẫu Tách chỉ gợi người cầm quyền duyệt gia hạn (ADR 0038).
    layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN).then((kq) => {
      if (!bo) datKqDanhBaLanhDao(kq);
    });
    layTrangThaiNhiemVu().then((kq) => {
      if (!bo) datKqNhanTT(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const so = taiTu(daTai, khoa);
  const dsDanhBa: readonly identity_canBoChonNguoiRa[] =
    kqDanhBa !== null && kqDanhBa.ok ? kqDanhBa.duLieu.items : [];
  const bangDanhBa: DanhBaTheoMa | null =
    kqDanhBa !== null && kqDanhBa.ok ? danhBaTheoMa(kqDanhBa.duLieu.items) : null;
  const loiDanhBa = kqDanhBa !== null && !kqDanhBa.ok ? kqDanhBa.thongBao : null;
  const { bang: nhanTT } = docBangNhanTrangThai(kqNhanTT);

  /** Đọc lại cả trang VÀ biên bản đang xem: bộ đếm và trạng thái suy ra đều do MÁY CHỦ tính. */
  function docLai(): void {
    datLanTai((n) => n + 1);
    datLuotChiTiet((n) => n + 1);
  }

  function dongChiTiet(): void {
    // `replaceState` không bắn `hashchange`, nên báo tay — `useSyncExternalStore` đọc lại neo.
    window.history.replaceState(null, "", window.location.pathname + window.location.search);
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  }

  /** Một lần ghi của vòng đời: khoá nút, gọi, câu lỗi NGUYÊN VĂN gắn vào đúng bản ghi. */
  function chay<T>(dich: string, loiGoi: Promise<KetQua<T>>, xong?: () => void): void {
    datDangGui(true);
    loiGoi.then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiVD({ dich, thongBao: kq.thongBao });
        return;
      }
      datLoiVD(null);
      datHop(null);
      xong?.();
      docLai();
    });
  }

  function taiNhiemVuKL(bienBanID: string, kl: petitions_ketLuanRa): void {
    datNhiemVuKL((cu) => new Map(cu).set(kl.id, { pha: "dangTai" }));
    layNhiemVuCuaKetLuan(bienBanID, kl).then((kq) => {
      datNhiemVuKL((cu) => {
        // Đã đóng trong lúc chờ thì không mở lại.
        if (!cu.has(kl.id)) return cu;
        return new Map(cu).set(
          kl.id,
          kq.ok
            ? { pha: "xong", duLieu: kq.duLieu.items }
            : { pha: "loi", thongBao: kq.thongBao },
        );
      });
    });
  }

  const vongDoi: PhepVongDoi = {
    dangGui,
    coQuyenKy,
    danhBa: bangDanhBa,
    nhanTT,
    hop,
    loi: loiVD,
    chiTiet: idChiTiet === null ? null : { id: idChiTiet, tai: taiTu(daTaiChiTiet, khoaChiTiet) },
    nhiemVuKL,
    moHop: (dich, loai) => {
      datHop((cu) => (cu?.dich === dich && cu.loai === loai ? null : { dich, loai }));
      datLoiVD(null);
    },
    dongHop: () => {
      datHop(null);
      datLoiVD(null);
    },
    dongChiTiet,
    moSua: (bb) => {
      // Sửa đọc qua tuyến CHI TIẾT: biểu mẫu cần toàn văn và thành phần mà danh sách không trả.
      // Điền biểu mẫu từ tấm thẻ sẽ gửi đi một nội dung RỖNG thay cho toàn văn đang lưu.
      datDangGui(true);
      layBienBan(bb.id).then((kq) => {
        datDangGui(false);
        if (!kq.ok) {
          datLoiVD({ dich: bb.id, thongBao: kq.thongBao });
          return;
        }
        datLoiVD(null);
        datLoiBieuMau(null);
        datBieuMau({ loai: "sua", ban: kq.duLieu });
      });
    },
    moBoSung: (bb) => {
      datLoiBieuMau(null);
      datBieuMau({ loai: "tao", boSungCho: bb });
    },
    ky: (bb, thongBao) => chay(bb.id, kyBienBan(bb.id, { notice: thongBao })),
    ghiThongBao: (bb, thongBao) => chay(bb.id, suaBienBan(bb.id, { notice: thongBao })),
    xoa: (bb, lyDo) =>
      chay(bb.id, xoaBienBan(bb.id, lyDo), () => {
        if (idChiTiet === bb.id) dongChiTiet();
      }),
    suaKL: (bb, kl, noiDung) => chay(kl.id, suaKetLuan(bb.id, kl, noiDung)),
    goKL: (bb, kl, lyDo) => chay(kl.id, xoaKetLuan(bb.id, kl, lyDo)),
    danhDau: (bb, kl) => chay(kl.id, danhDauKhongPhatSinh(bb.id, kl)),
    boDau: (bb, kl) => chay(kl.id, boDauKhongPhatSinh(bb.id, kl)),
    batNhiemVuKL: (bb, kl) => {
      if (nhiemVuKL.has(kl.id)) {
        datNhiemVuKL((cu) => boKhoa(cu, kl.id));
        return;
      }
      taiNhiemVuKL(bb.id, kl);
    },
  };

  function guiKetLuan(bienBanID: string, noiDung: string, khoaChongTrung: string): void {
    datDangGui(true);
    setAddingTo(bienBanID);
    themKetLuan(bienBanID, { content: noiDung }, khoaChongTrung).then((kq) => {
      datDangGui(false);
      setAddingTo(null);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ — kể cả câu 409 "biên bản họp đã ký — … lập biên bản bổ sung".
        datLoiKetLuan({ bienBanID, thongBao: kq.thongBao });
        return;
      }
      datLoiKetLuan(null);
      datLanGhiXong((n) => n + 1);
      docLai();
    });
  }

  /**
   * §3 — tách một kết luận thành một nhiệm vụ. `ketLuan` đi nguyên xuống lớp gọi; `{stt}` đọc từ
   * `ordinal` của chính bản ghi ấy. Ghi xong thì đóng hộp (lần mở sau là khoá mới), đọc lại trang
   * (bộ đếm do máy chủ cộng), và đọc lại danh sách nhiệm vụ của kết luận nếu nó đang mở.
   */
  function guiTach(
    bienBanID: string,
    ketLuan: petitions_ketLuanRa,
    than: TachKetLuanVao,
    khoaChongTrung: string,
  ): void {
    datDangGui(true);
    tachKetLuanThanhNhiemVu(bienBanID, ketLuan, than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiTach({ ketLuanID: ketLuan.id, thongBao: kq.thongBao });
        return;
      }
      datLoiTach(null);
      // The prototype's toast (owner 09/10/2026). The new task shows in the row's split-task list.
      toast.success(SPLIT_DONE_TOAST);
      datMoTachO(null);
      datLanGhiXong((n) => n + 1);
      docLai();
      if (nhiemVuKL.has(ketLuan.id)) taiNhiemVuKL(bienBanID, ketLuan);
    });
  }

  const phepTach: PhepTach = {
    danhMuc,
    danhBa: kqDanhBa,
    danhBaLanhDao: kqDanhBaLanhDao,
    dangGui,
    moOKetLuan: moTachO,
    loi: loiTach,
    mo: (ketLuanID) => {
      datMoTachO((dang) => (dang === ketLuanID ? null : ketLuanID));
      datLoiTach(null);
    },
    dong: () => {
      datMoTachO(null);
      datLoiTach(null);
    },
    gui: guiTach,
  };

  function xongBieuMau(): void {
    datLoiBieuMau(null);
    datBieuMau(null);
    datLanGhiXong((n) => n + 1);
  }

  function luuBienBan(than: TaoBienBanVao, khoaChongTrung: string): void {
    datDangGui(true);
    taoBienBan(than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiBieuMau(kq.thongBao);
        return;
      }
      toast.success(MEETING_SAVED_TOAST);
      xongBieuMau();
      // Về TRANG ĐẦU: con trỏ của trang đang xem không còn nghĩa sau khi một bản ghi chen vào.
      datNganXep(TRANG_DAU);
      docLai();
    });
  }

  function luuSua(bienBanID: string, than: SuaBienBanVao): void {
    datDangGui(true);
    suaBienBan(bienBanID, than).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiBieuMau(kq.thongBao);
        return;
      }
      // Same dialog, same act (save the record) — same sentence as the create path.
      toast.success(MEETING_SAVED_TOAST);
      xongBieuMau();
      docLai();
    });
  }

  const chiTiet = vongDoi.chiTiet;
  const chiTietNgoaiTrang =
    chiTiet !== null && so.pha === "xong" && !so.duLieu.items.some((bb) => bb.id === chiTiet.id);
  // The previous page, while a re-read is in flight (after every write `docLai` bumps the key): drawn
  // `inert` in its own subtree, never acted on — NOT dimmed: the prototype keeps the old cards on
  // screen unchanged while it re-reads. It is a DIFFERENT subtree from the fresh list, so the fresh
  // list mounts anew exactly as it did when the old screen showed only a sentence.
  const sanCu =
    so.pha === "dangTai" && daTai !== null && daTai.kq.ok && daTai.kq.duLieu.items.length > 0
      ? daTai.kq.duLieu.items
      : null;

  return (
    <>
      {/* The prototype's header: title + one line, `+ Nhập biên bản` on the right of the same row.
          The button OPENS the dialog (prototype) — it no longer toggles an in-page form. */}
      <PageHeader
        icon={ClipboardList}
        className="mb-5"
        title={TIEU_DE_MAN}
        subtitle={<span className="mo-ta-trang m-0 max-w-none text-[13px] text-ink-500">{MO_TA_MAN}</span>}
        actions={
          <Button
            type="button"
            variant="primary"
            icon={<Glyph icon={Plus} />}
            aria-haspopup="dialog"
            onClick={() => {
              datBieuMau({ loai: "tao", boSungCho: null });
              datLoiBieuMau(null);
            }}
          >
            {NHAN_NUT_NHAP_BIEN_BAN}
          </Button>
        }
      />

    {/* `[&>*]:my-0`: the wrapper's `gap` is the one spacing between blocks (the prototype's
        `space-y-4`); legacy vertical margins on direct children would add to it unevenly. No list
        heading: the prototype has none, and each card's title is the page's `h2` (h1 → h2). */}
    <div className="man-bien-ban mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0">
      {bieuMau !== null && (
        <FormNhapBienBan
          // Khoá dựng lại: mỗi lần GHI XONG, và mỗi lần đổi biểu mẫu, là một khoá chống trùng mới.
          key={`bieu-mau|${lanGhiXong}|${bieuMau.loai === "sua" ? bieuMau.ban.id : (bieuMau.boSungCho?.id ?? "")}`}
          che={bieuMau}
          danhBa={dsDanhBa}
          loiDanhBa={loiDanhBa}
          dangGui={dangGui}
          loi={loiBieuMau}
          huy={() => {
            datBieuMau(null);
            datLoiBieuMau(null);
          }}
          luu={luuBienBan}
          sua={luuSua}
        />
      )}

      {chiTietNgoaiTrang && chiTiet !== null && (
        // Biên bản mở từ một đường dẫn (drawer nhiệm vụ, liên kết bổ sung) mà không nằm ở trang
        // đang xem: vẫn mở được, ngay đầu danh sách.
        <ChiTietBienBan tai={chiTiet.tai} danhBa={bangDanhBa} dong={dongChiTiet} />
      )}

      {/* LOADING. The sentence stays the live region, read out as before; the eye gets the
          prototype's single `Skeleton h-40 w-full` on the first read, or the previous page as it
          was (inert, not dimmed) on a re-read. */}
      {so.pha === "dangTai" && (
        <div className="flex min-w-0 flex-col gap-4">
          <p className="an-thi-giac" role="status">
            {DANG_TAI_SO}
          </p>
          {sanCu !== null ? (
            <div inert className="pointer-events-none">
              <DanhSachBienBan
                bienBan={sanCu}
                lanGhiXong={lanGhiXong}
                dangGui={dangGui}
                loiKetLuan={loiKetLuan}
                guiKetLuan={guiKetLuan}
                tach={phepTach}
                vongDoi={vongDoi}
                addingTo={addingTo}
              />
            </div>
          ) : (
            <Skeleton className="h-40 w-full" />
          )}
        </div>
      )}
      {/* LOAD ERROR (spec §8b): the server's sentence VERBATIM stays the alert; `Tải lại` asks the
          same read again through the screen's existing re-read key (`lanTai`). */}
      {so.pha === "loi" && (
        <Card>
          <EmptyState
            icon={CloudOff}
            title="Chưa tải được danh sách biên bản"
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
        </Card>
      )}

      {so.pha === "xong" && (
        <>
          <DanhSachBienBan
            bienBan={so.duLieu.items}
            lanGhiXong={lanGhiXong}
            dangGui={dangGui}
            loiKetLuan={loiKetLuan}
            guiKetLuan={guiKetLuan}
            tach={phepTach}
            vongDoi={vongDoi}
            addingTo={addingTo}
          />
          <nav className="dieu-huong-trang m-0" aria-label="Phân trang danh sách biên bản">
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
    </>
  );
}

/** Danh sách THẺ §2 — xếp dọc, ĐÚNG THỨ TỰ MÁY CHỦ TRẢ (ngày họp mới nhất ở trên). */
export function DanhSachBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
  vongDoi,
  addingTo = null,
}: {
  bienBan: readonly petitions_bienBanRa[];
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: { bienBanID: string; thongBao: string } | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
  vongDoi: PhepVongDoi;
  /** Id of the card whose `Thêm kết luận` is in flight, or `null`. */
  addingTo?: string | null;
}) {
  if (bienBan.length === 0) {
    // The screen has no filter, so "empty" here is only ever the empty register — never "empty under
    // these filters". The prototype's dashed box with its one sentence.
    return (
      <p className="m-0 rounded-card border border-dashed border-line bg-surface px-6 py-12 text-center text-[13px] text-ink-500">
        {SO_RONG}
      </p>
    );
  }

  return (
    <ul aria-label="Danh sách biên bản họp" className="m-0 flex list-none flex-col gap-4 p-0">
      {bienBan.map((bb) => (
        // `id` là neo `#bien-ban-{id}` — liên kết từ drawer nhiệm vụ cuộn tới đúng thẻ này.
        <li key={bb.id} id={neoBienBan(bb.id)} className="scroll-mt-4">
          <TheBienBan
            bienBan={bb}
            lanGhiXong={lanGhiXong}
            dangGui={dangGui}
            loiKetLuan={loiKetLuan?.bienBanID === bb.id ? loiKetLuan.thongBao : null}
            guiKetLuan={guiKetLuan}
            tach={tach}
            vongDoi={vongDoi}
            adding={addingTo === bb.id}
          />
        </li>
      ))}
    </ul>
  );
}

/** Một nút có thể TẮT kèm lý do: lý do là một câu nhìn thấy được, nối với nút bằng `aria-describedby`. */
function NutCoLyDo({
  quyTac,
  id,
  nhan,
  ariaLabel,
  dangGui,
  onClick,
  moRa,
  icon,
  variant = "secondary",
  size = "md",
  className,
  describedBy,
}: {
  quyTac: QuyTacNut;
  id: string;
  nhan: string;
  ariaLabel?: string;
  dangGui: boolean;
  onClick: () => void;
  moRa?: boolean;
  /** Decorative leading icon; the label always stays (spec v2: important actions keep their words). */
  icon?: LucideIcon;
  variant?: ButtonVariant;
  size?: "sm" | "md";
  className?: string;
  /**
   * Id of the reason sentence, when several buttons share ONE (a conclusion's Sửa and Gỡ are locked
   * for the same reason — spec 05 §A9: the sentence is in the DOM once). Default `ly-do-{id}`.
   */
  describedBy?: string;
}) {
  if (!quyTac.hien) return null;
  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      className={className}
      icon={icon === undefined ? undefined : <Glyph icon={icon} />}
      aria-label={ariaLabel}
      aria-expanded={moRa}
      aria-describedby={quyTac.viSaoTat === null ? undefined : (describedBy ?? `ly-do-${id}`)}
      // Spec 05 §B: the reason is also the button's `title`. Mouse-only extra — the visible sentence
      // under the row and `aria-describedby` stay the real carriers.
      title={quyTac.viSaoTat ?? undefined}
      disabled={dangGui || quyTac.viSaoTat !== null}
      onClick={onClick}
    >
      {nhan}
    </Button>
  );
}

/** Câu lý do của một nút đang tắt. */
function LyDoTat({ quyTac, id, className }: { quyTac: QuyTacNut; id: string; className: string }) {
  if (!quyTac.hien || quyTac.viSaoTat === null) return null;
  return (
    <p className={className} id={`ly-do-${id}`}>
      {quyTac.viSaoTat}
    </p>
  );
}

/** Small ghost buttons of a conclusion row (spec 05 §B: `h-7 px-2 text-[11.5px]`, 14px icons). */
const CONCLUSION_ACTION = "h-7 px-2 text-[11.5px]";

/** Một thẻ biên bản §2: header · con số · hành động · các dòng kết luận · hàng thêm kết luận. */
export function TheBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
  vongDoi,
  adding = false,
}: {
  bienBan: petitions_bienBanRa;
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: string | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
  vongDoi: PhepVongDoi;
  /** This card's `Thêm kết luận` is in flight — its button spins (prototype `isPending`). */
  adding?: boolean;
}) {
  const qt = quyTacBienBan(bienBan, vongDoi.coQuyenKy);
  const loi = vongDoi.loi?.dich === bienBan.id ? vongDoi.loi.thongBao : null;
  const hop = vongDoi.hop?.dich === bienBan.id ? vongDoi.hop.loai : null;
  const chiTiet = vongDoi.chiTiet?.id === bienBan.id ? vongDoi.chiTiet : null;

  return (
    <Card as="article">
      {/* THE PROTOTYPE'S CARD HEADER (`MeetingMinutes.tsx:110-126`): icon tile · title over the meta
          line · the count pill on the right. Ours carries two pills: the Dự thảo / Đã ký status
          (owner, 25/09) and the count, `{a}/{n} kết luận xong · {x}/{y} nhiệm vụ xong` (owner, 09/10). */}
      <CardHeader className="px-5 py-4">
        <span aria-hidden="true" className="bg-navy/8 text-navy grid size-9 place-items-center rounded-[9px] shrink-0">
          <ClipboardList className="size-4" focusable="false" />
        </span>
        <div className="min-w-0 flex-1 basis-48">
          <CardTitle as="h2" className="text-[14.5px] font-bold break-words">
            {bienBan.title}
          </CardTitle>
          <p className="m-0 text-[11.5px] text-ink-muted tabular-nums">{dongMeta(bienBan)}</p>
        </div>
        <MeetingStatusBadge meeting={bienBan}>{nhanTrangThaiBienBan(bienBan)}</MeetingStatusBadge>
        {/* Cả bốn số do máy chủ đếm. The prototype's shadcn Badge `bg-surface text-ink border-line`
            (its `bg-surface` is our `bg-canvas`), text only, nothing bold. */}
        <Badge icon={null} className="border-line bg-canvas text-ink tabular-nums">
          {nhanTienDoBienBan(bienBan)} · {nhanBadge(bienBan)}
        </Badge>
      </CardHeader>

      {/* The body is the prototype's block flow (`px-5 py-4`, margins on the children), not a gap
          column: spec 05 places the lock reason `-mt-1` under the button row, which only reads as
          intended between margins. */}
      <div className="px-5 py-4">
        {bienBan.supplements_id !== undefined && bienBan.supplements_id !== "" && (
          <p className="ghi-chu m-0 mb-3">
            Biên bản bổ sung — <a href={`#${neoBienBan(bienBan.supplements_id)}`}>xem biên bản gốc</a>
          </p>
        )}

        {/* RECORD ACTIONS — not in the prototype, which has no lifecycle; spec 05 §B puts them at the
            top of the body, `mb-3 flex flex-wrap gap-2`. ONE solid button: `Ký biên bản`, the act
            that fixes the record. */}
        <div className="mb-3 flex flex-wrap items-center gap-2">
          {chiTiet !== null ? (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Glyph icon={X} />}
              onClick={vongDoi.dongChiTiet}
            >
              {NHAN_NUT_DONG_BIEN_BAN}
            </Button>
          ) : (
            <a
              className={cn("nut-phu", buttonVariants({ variant: "secondary", size: "sm" }))}
              href={`#${neoBienBan(bienBan.id)}`}
            >
              <Glyph icon={Eye} />
              {NHAN_NUT_XEM_BIEN_BAN}
            </a>
          )}
          <NutCoLyDo
            quyTac={qt.sua}
            id={`sua-${bienBan.id}`}
            nhan={NHAN_NUT_SUA_BIEN_BAN}
            icon={Pencil}
            size="sm"
            dangGui={vongDoi.dangGui}
            onClick={() => vongDoi.moSua(bienBan)}
          />
          <NutCoLyDo
            quyTac={qt.ky}
            id={`ky-${bienBan.id}`}
            nhan={NHAN_NUT_KY}
            icon={Signature}
            variant="primary"
            size="sm"
            dangGui={vongDoi.dangGui}
            moRa={hop === "ky"}
            onClick={() => vongDoi.moHop(bienBan.id, "ky")}
          />
          <NutCoLyDo
            quyTac={qt.ghiThongBao}
            id={`thong-bao-${bienBan.id}`}
            nhan={NHAN_NUT_GHI_THONG_BAO}
            icon={Stamp}
            size="sm"
            dangGui={vongDoi.dangGui}
            moRa={hop === "thong-bao"}
            onClick={() => vongDoi.moHop(bienBan.id, "thong-bao")}
          />
          <NutCoLyDo
            quyTac={qt.boSung}
            id={`bo-sung-${bienBan.id}`}
            nhan={NHAN_NUT_BO_SUNG}
            icon={FilePlus2}
            size="sm"
            dangGui={vongDoi.dangGui}
            onClick={() => vongDoi.moBoSung(bienBan)}
          />
          <NutCoLyDo
            quyTac={qt.xoa}
            id={`xoa-${bienBan.id}`}
            nhan={NHAN_NUT_XOA_BIEN_BAN}
            icon={Trash2}
            // Spec 05 §B: an OUTLINE button with a red word — no red fill.
            className="text-danger hover:not-disabled:text-danger"
            size="sm"
            dangGui={vongDoi.dangGui}
            moRa={hop === "xoa"}
            onClick={() => vongDoi.moHop(bienBan.id, "xoa")}
          />
        </div>
        <LyDoTat quyTac={qt.xoa} id={`xoa-${bienBan.id}`} className="m-0 -mt-1 mb-3 text-[11.5px] text-ink-muted" />

        {loi !== null && (
          <p className="thong-bao-loi m-0 mb-3" role="alert">
            {loi}
          </p>
        )}

        {hop === "ky" && (
          <div className="mb-3">
            <FormKy
              bienBanID={bienBan.id}
              question={`Ký biên bản “${bienBan.title}”?`}
              dangGui={vongDoi.dangGui}
              huy={vongDoi.dongHop}
              ky={(tb) => vongDoi.ky(bienBan, tb)}
            />
          </div>
        )}
        {hop === "thong-bao" && (
          <div className="mb-3">
            <FormThongBao
              bienBanID={bienBan.id}
              dangGui={vongDoi.dangGui}
              huy={vongDoi.dongHop}
              ghi={(tb) => vongDoi.ghiThongBao(bienBan, tb)}
            />
          </div>
        )}
        {/* Spec 05 §B: `Gỡ biên bản` confirms in a DIALOG — with the reason field rule 7 requires. */}
        {hop === "xoa" && (
          <FormLyDo
            id={`ly-do-xoa-bien-ban-${bienBan.id}`}
            question={`Gỡ biên bản “${bienBan.title}”?`}
            nhan="Lý do gỡ biên bản *"
            nhanNut={NHAN_NUT_XOA_BIEN_BAN}
            ghiChu="Biên bản là hồ sơ lưu trữ: gỡ là xoá mềm, ghi lại ai gỡ, lúc nào và vì sao."
            dangGui={vongDoi.dangGui}
            huy={vongDoi.dongHop}
            gui={(lyDo) => vongDoi.xoa(bienBan, lyDo)}
          />
        )}

        {chiTiet !== null && (
          <div className="mb-3">
            <ChiTietBienBan tai={chiTiet.tai} danhBa={vongDoi.danhBa} dong={vongDoi.dongChiTiet} />
          </div>
        )}

        {/* THE PROTOTYPE'S CONCLUSION ROWS: one bordered box per conclusion, `mb-2.5` each. A record
            with none draws nothing here — §7.3 (saved first, conclusions later) still holds through
            the add row below; the prototype has no sentence for it. */}
        {bienBan.conclusions.length > 0 && (
          <ol aria-label={`Các kết luận của biên bản ${bienBan.title}`} className="m-0 list-none p-0">
            {/* ⚠ KHÔNG LẤY CHỈ SỐ CỦA `map` RA DÙNG. Số trong ô tròn và `{stt}` trên đường dẫn đều
                là `ordinal` MÁY CHỦ TRẢ — xem `DongKetLuan` và `soThuTuKetLuan`. */}
            {bienBan.conclusions.map((kl) => (
              <li key={kl.id} className="mb-2.5 rounded-[10px] border border-line px-4 py-3">
                <DongKetLuan bienBan={bienBan} ketLuan={kl} tach={tach} vongDoi={vongDoi} />
              </li>
            ))}
          </ol>
        )}

        {loiKetLuan !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loiKetLuan}
          </p>
        )}

        {/* Biên bản ĐÃ KÝ không nhận kết luận mới — sai sót đi đường biên bản bổ sung. */}
        {qt.themKetLuan.hien && (
          <HangThemKetLuan
            key={`${bienBan.id}|${lanGhiXong}`}
            bienBanID={bienBan.id}
            dangGui={dangGui}
            adding={adding}
            gui={guiKetLuan}
          />
        )}
      </div>
    </Card>
  );
}

/** Đoạn văn nhiều dòng: mỗi dòng xuống một `<br>` — không CSS, không `dangerouslySetInnerHTML`. */
function DoanNhieuDong({ chu }: { chu: string }) {
  const dong = chu.split("\n");
  return (
    <p>
      {dong.map((d, i) => (
        // Chỉ số ở đây là vị trí DÒNG CHỮ trong một đoạn văn, không phải số của bản ghi nào.
        <span key={i}>
          {d}
          {i < dong.length - 1 && <br />}
        </span>
      ))}
    </p>
  );
}

/**
 * "Xem biên bản" — toàn văn và các trường danh sách không trả: nội dung, thành phần, chủ trì, thư
 * ký, người ký và thời điểm ký, Thông báo kết luận, và HAI CHIỀU bổ sung.
 */
export function ChiTietBienBan({
  tai,
  danhBa,
  dong,
}: {
  tai: TrangThaiTai<petitions_bienBanRa>;
  danhBa: DanhBaTheoMa | null;
  dong: () => void;
}) {
  return (
    <section
      className="min-w-0 rounded-xl border border-line bg-surface-muted p-4 text-sm"
      aria-label="Toàn văn biên bản"
    >
      {tai.pha === "dangTai" && (
        <p role="status" className="m-0 inline-flex items-center gap-2 text-ink-500">
          <LoaderCircle aria-hidden="true" focusable="false" className="size-4 shrink-0 motion-safe:animate-spin" />
          Đang tải biên bản…
        </p>
      )}
      {/* No `Tải lại` here: this block has no re-read of its own to call — closing and reopening
          "Xem biên bản" is the existing way to read it again. */}
      {tai.pha === "loi" && (
        <p className="m-0 flex items-start gap-2">
          <CloudOff aria-hidden="true" focusable="false" className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
          <span className="text-danger-600" role="alert">
            {tai.thongBao}
          </span>
        </p>
      )}
      {tai.pha === "xong" && <NoiDungChiTiet bb={tai.duLieu} danhBa={danhBa} />}
      <div className="cum-nut mt-4 justify-end">
        <Button type="button" variant="secondary" icon={<Glyph icon={X} />} onClick={dong}>
          {NHAN_NUT_DONG_BIEN_BAN}
        </Button>
      </div>
    </section>
  );
}

/** Heading inside "Xem biên bản": 13px/600, the card's sub-section level. */
const DETAIL_HEADING = "m-0 mb-2 text-[13px] leading-snug font-semibold text-ink-900";

function NoiDungChiTiet({ bb, danhBa }: { bb: petitions_bienBanRa; danhBa: DanhBaTheoMa | null }) {
  const thanhPhan = bb.attendees ?? [];
  const boSungBoi = bb.supplemented_by ?? [];
  return (
    <>
      <h4 className={cn(DETAIL_HEADING, "text-[15px]")}>{bb.title}</h4>
      <dl className="danh-sach-truong mb-4">
        <dt>Ngày họp</dt>
        <dd>{nhanNgayHop(bb.held_on)}</dd>
        <dt>Số hiệu biên bản</dt>
        <dd>{bb.reference_no === "" ? "Không ghi" : bb.reference_no}</dd>
        <dt>Địa điểm</dt>
        <dd>{bb.location === "" ? "Không ghi" : bb.location}</dd>
        <dt>Trạng thái</dt>
        <dd>{nhanTrangThaiBienBan(bb)}</dd>
        <dt>Chủ trì</dt>
        <dd>{nhanCanBo(bb.chaired_by, danhBa)}</dd>
        <dt>Thư ký</dt>
        <dd>{nhanCanBo(bb.minutes_taker, danhBa)}</dd>
        <dt>Thành phần tham dự</dt>
        <dd>
          {thanhPhan.length === 0 ? (
            "Không ghi"
          ) : (
            <ul className="m-0 list-disc pl-5 marker:text-ink-400">
              {thanhPhan.map((d) => (
                <li key={d}>{nhanThanhPhan(d, danhBa)}</li>
              ))}
            </ul>
          )}
        </dd>
        <dt>Người ký</dt>
        <dd>
          {laDaKy(bb)
            ? `${nhanCanBo(bb.signed_by, danhBa)} · ngày ${nhanNgay(bb.signed_at ?? null)}`
            : "Chưa ký"}
        </dd>
        <dt>Thông báo kết luận</dt>
        <dd>{nhanThongBao(bb.notice)}</dd>
        {bb.supplements_id !== undefined && bb.supplements_id !== "" && (
          <>
            <dt>Bổ sung cho</dt>
            <dd>
              <a href={`#${neoBienBan(bb.supplements_id)}`}>Mở biên bản gốc</a>
            </dd>
          </>
        )}
        {boSungBoi.length > 0 && (
          <>
            <dt>Biên bản bổ sung</dt>
            <dd>
              <ul className="m-0 list-disc pl-5 marker:text-ink-400">
                {boSungBoi.map((id, i) => (
                  <li key={id}>
                    <a href={`#${neoBienBan(id)}`}>Mở biên bản bổ sung thứ {i + 1}</a>
                  </li>
                ))}
              </ul>
            </dd>
          </>
        )}
        <dt>Người nhập</dt>
        <dd>{nhanCanBo(bb.created_by, danhBa)}</dd>
      </dl>
      <h4 className={DETAIL_HEADING}>Nội dung biên bản</h4>
      {bb.content === undefined || bb.content === null || bb.content === "" ? (
        <p className="nhan-trong m-0">Không ghi toàn văn.</p>
      ) : (
        <div className="rounded-lg border border-line bg-surface p-3 leading-relaxed [&>p]:m-0">
          <DoanNhieuDong chu={bb.content} />
        </div>
      )}
    </>
  );
}

/** Hai ô Thông báo kết luận: số, ký hiệu · ngày (NGÀY LỊCH). */
function OThongBao({
  bienBanID,
  so,
  ngay,
  datSo,
  datNgay,
}: {
  bienBanID: string;
  so: string;
  ngay: string;
  datSo: (s: string) => void;
  datNgay: (s: string) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 [&>.o-nhap]:mb-0">
      <div className="o-nhap">
        <label htmlFor={`so-thong-bao-${bienBanID}`}>Số, ký hiệu Thông báo kết luận</label>
        <input
          id={`so-thong-bao-${bienBanID}`}
          value={so}
          maxLength={SO_THONG_BAO_TOI_DA}
          autoComplete="off"
          onChange={(e) => datSo(e.target.value)}
        />
      </div>
      <div className="o-nhap">
        <label htmlFor={`ngay-thong-bao-${bienBanID}`}>Ngày Thông báo kết luận</label>
        <input
          id={`ngay-thong-bao-${bienBanID}`}
          type="date"
          value={ngay}
          onChange={(e) => datNgay(e.target.value)}
        />
      </div>
    </div>
  );
}

/**
 * A form's submit button: the native `<button type="submit">` with `type` FIRST (tests read the tag
 * as `<button type="submit" …>`), the same classes as `Button`, and the spec §8b busy state —
 * spinner + "Đang lưu…", same width (`BusyLabel`). `busy` is the screen's one `dangGui` flag; the
 * button is already disabled by it, as before.
 */
function SubmitButton({
  label,
  busy,
  disabled,
  variant = "primary",
  busyText = BUSY_SAVING,
}: {
  label: string;
  busy: boolean;
  disabled: boolean;
  variant?: "primary" | "secondary" | "danger";
  busyText?: string;
}) {
  return (
    <button
      type="submit"
      className={cn(LEGACY_BUTTON_CLASS[variant], buttonVariants({ variant }))}
      disabled={disabled}
      aria-busy={busy}
    >
      <BusyLabel busy={busy} label={label} busyText={busyText} />
    </button>
  );
}

/** One helper line under a control (spec §3 "Chú thích": 12px, `--ink-500`). */
const HINT = "ghi-chu mt-1.5 mb-0 text-xs text-ink-500";

/** Busy words of a removal (gỡ = soft delete) — "Đang lưu…" would misname the act. */
const BUSY_REMOVING = "Đang gỡ…";

/** `Huỷ` of every form on this screen. */
function CancelButton({ dangGui, huy }: { dangGui: boolean; huy: () => void }) {
  return (
    <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
      {NHAN_NUT_HUY}
    </Button>
  );
}

/** Hộp Ký — câu xác nhận nói đúng hệ quả, và hai ô Thông báo tuỳ chọn. */
export function FormKy({
  bienBanID,
  question,
  dangGui,
  huy,
  ky,
}: {
  bienBanID: string;
  /** The specific question heading the box (spec §7 "Hộp xác nhận"), e.g. `Ký biên bản “…”?`. */
  question: string;
  dangGui: boolean;
  huy: () => void;
  ky: (thongBao: ThongBaoVao | null) => void;
}) {
  const [so, datSo] = useState("");
  const [ngay, datNgay] = useState("");

  function gui(e: FormEvent) {
    e.preventDefault();
    ky(thanThongBao(so, ngay));
  }

  return (
    <ConfirmDialog
      as="form"
      titleAs="h4"
      icon={Signature}
      title={question}
      onSubmit={gui}
      aria-label={NHAN_NUT_KY}
      actions={
        <>
          <CancelButton dangGui={dangGui} huy={huy} />
          <SubmitButton label={NHAN_NUT_XAC_NHAN_KY} busy={dangGui} disabled={dangGui} />
        </>
      }
    >
      <p role="note" className="m-0">
        {CAU_XAC_NHAN_KY}
      </p>
      <OThongBao bienBanID={bienBanID} so={so} ngay={ngay} datSo={datSo} datNgay={datNgay} />
      <p className="ghi-chu m-0">Thông báo kết luận không bắt buộc lúc ký; ghi sau được một lần.</p>
    </ConfirmDialog>
  );
}

/** Ghi số Thông báo kết luận cho biên bản ĐÃ KÝ — một lần. */
export function FormThongBao({
  bienBanID,
  dangGui,
  huy,
  ghi,
}: {
  bienBanID: string;
  dangGui: boolean;
  huy: () => void;
  ghi: (thongBao: ThongBaoVao) => void;
}) {
  const [so, datSo] = useState("");
  const [ngay, datNgay] = useState("");
  const tb = thanThongBao(so, ngay);

  function gui(e: FormEvent) {
    e.preventDefault();
    if (tb !== null) ghi(tb);
  }

  return (
    <ConfirmDialog
      as="form"
      titleAs="h4"
      icon={Stamp}
      title={NHAN_NUT_GHI_THONG_BAO}
      onSubmit={gui}
      aria-label={NHAN_NUT_GHI_THONG_BAO}
      actions={
        <>
          <CancelButton dangGui={dangGui} huy={huy} />
          <SubmitButton label={NHAN_NUT_GHI_THONG_BAO} busy={dangGui} disabled={dangGui || tb === null} />
        </>
      }
    >
      <OThongBao bienBanID={bienBanID} so={so} ngay={ngay} datSo={datSo} datNgay={datNgay} />
      <p className="ghi-chu m-0">{CAU_THONG_BAO_MOT_LAN}</p>
    </ConfirmDialog>
  );
}

/**
 * Ô lý do BẮT BUỘC của một lần gỡ (biên bản hay kết luận) — xoá mềm phải ghi vì sao.
 *
 * A CENTRED DIALOG since 09/10/2026 (spec 05 §B: "Gỡ phải mở Dialog xác nhận"). The reason field
 * stays inside it — rule 7 requires it, the prototype has no removal at all. Esc and ✕ are refused
 * while the removal is in flight, as `Huỷ` is (the screen's one `dangGui`).
 */
export function FormLyDo({
  id,
  question,
  nhan,
  nhanNut,
  ghiChu,
  dangGui,
  huy,
  gui,
}: {
  id: string;
  /** The specific question heading the box (spec §7 "Hộp xác nhận"), e.g. `Gỡ kết luận số 3?`. */
  question: string;
  nhan: string;
  nhanNut: string;
  ghiChu: string;
  dangGui: boolean;
  huy: () => void;
  gui: (lyDo: string) => void;
}) {
  const [lyDo, datLyDo] = useState("");
  const gon = lyDo.trim();

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (gon !== "") gui(gon);
  }

  const titleId = `${id}-title`;
  return (
    <ModalDialog
      titleId={titleId}
      initialFocusId={id}
      closeDisabled={dangGui}
      onDismiss={() => (dangGui ? undefined : huy())}
    >
      <ModalDialogHeader titleId={titleId} title={question} />
      <form className="m-0 flex min-h-0 flex-col gap-4" onSubmit={guiNgay}>
        <div className="o-nhap mb-0">
          <label htmlFor={id}>{nhan}</label>
          <textarea
            id={id}
            rows={2}
            value={lyDo}
            maxLength={LY_DO_XOA_TOI_DA}
            onChange={(e) => datLyDo(e.target.value)}
          />
          <p className="ghi-chu mt-1.5 mb-0">{ghiChu}</p>
        </div>
        <div className="flex shrink-0 justify-end gap-2">
          <CancelButton dangGui={dangGui} huy={huy} />
          <SubmitButton
            label={nhanNut}
            variant="danger"
            busy={dangGui}
            busyText={BUSY_REMOVING}
            disabled={dangGui || gon === ""}
          />
        </div>
      </form>
    </ModalDialog>
  );
}

/**
 * Sửa LỜI một kết luận của bản nháp — IN PLACE (spec 05 §B): the sentence becomes a box where it
 * stood. Enter saves, Shift+Enter breaks the line, Esc cancels; the two small buttons stay for a
 * pointer or a touch screen, which have no Esc. Same PATCH, same inline refusal as before.
 */
export function FormSuaKetLuan({
  ketLuan,
  dangGui,
  huy,
  luu,
}: {
  ketLuan: petitions_ketLuanRa;
  dangGui: boolean;
  huy: () => void;
  luu: (noiDung: string) => void;
}) {
  const [noiDung, datNoiDung] = useState(ketLuan.content);
  const gon = noiDung.trim();
  const doiDuoc = gon !== "" && gon !== ketLuan.content;
  const boxRef = useRef<HTMLTextAreaElement>(null);
  const boxId = `sua-ket-luan-${ketLuan.id}`;

  // Opening the editor puts the caret in the box — the point of pressing `Sửa`.
  useEffect(() => {
    boxRef.current?.focus();
  }, []);

  function gui(e: FormEvent) {
    e.preventDefault();
    if (doiDuoc && !dangGui) luu(gon);
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    // An IME composing a Vietnamese syllable also sends Enter: that Enter is the IME's, not a save.
    if (e.nativeEvent.isComposing) return;
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (doiDuoc && !dangGui) luu(gon);
    } else if (e.key === "Escape" && !dangGui) {
      e.preventDefault();
      huy();
    }
  }

  return (
    <form className="m-0" onSubmit={gui}>
      {/* `.o-nhap` = the same textarea frame as the add row below the conclusions. */}
      <div className="o-nhap mb-0">
        <label htmlFor={boxId} className="an-thi-giac">
          Nội dung kết luận số {soThuTuKetLuan(ketLuan)}
        </label>
        <textarea
          ref={boxRef}
          id={boxId}
          rows={2}
          className="text-[12.8px]"
          value={noiDung}
          maxLength={NOI_DUNG_KET_LUAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
          onKeyDown={onKeyDown}
        />
      </div>
      <div className="mt-1.5 flex flex-wrap justify-end gap-1">
        <Button type="button" variant="ghost" size="sm" className={CONCLUSION_ACTION} disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </Button>
        <button
          type="submit"
          className={cn(LEGACY_BUTTON_CLASS.primary, buttonVariants({ variant: "primary", size: "sm" }), CONCLUSION_ACTION)}
          disabled={dangGui || !doiDuoc}
          aria-busy={dangGui}
        >
          {NHAN_NUT_LUU_SUA}
        </button>
      </div>
    </form>
  );
}

/** Các nhiệm vụ đã tách từ một kết luận (§3) — mã · tên · trạng thái (nhãn của xã) · hạn. */
export function DanhSachNhiemVuKetLuan({
  tai,
  nhanTT,
}: {
  tai: TrangThaiTai<readonly petitions_nhiemVuRa[]>;
  nhanTT: BangNhanTrangThai;
}) {
  if (tai.pha === "dangTai") {
    return (
      <p role="status" className="m-0 mt-2 inline-flex items-center gap-2 text-[12px] text-ink-muted">
        <LoaderCircle aria-hidden="true" focusable="false" className="size-4 shrink-0 motion-safe:animate-spin" />
        Đang tải nhiệm vụ đã tách…
      </p>
    );
  }
  if (tai.pha === "loi") {
    return (
      <p className="thong-bao-loi m-0 mt-2" role="alert">
        {tai.thongBao}
      </p>
    );
  }
  if (tai.duLieu.length === 0) {
    return <p className="ghi-chu m-0 mt-2">Không còn nhiệm vụ nào đang hiệu lực tách từ kết luận này.</p>;
  }
  // Spec 05 §B: a quiet box on the page colour; each line = the task's title AS THE LINK to its
  // drawer (`/nhiem-vu?task=<code>`, the pattern of `van-ban/letter-task.tsx`), its status, its due.
  return (
    <ul
      aria-label="Nhiệm vụ đã tách từ kết luận"
      className="m-0 mt-2 list-none space-y-1.5 rounded-[8px] border border-line bg-canvas px-3 py-2"
    >
      {tai.duLieu.map((nv) => (
        <li key={nv.code} className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <a href={taskDetailHref(nv.code)} className="text-[12px] font-medium text-navy hover:underline">
            {nv.title}
          </a>
          {/* Icon by the status CODE, word = the commune's label for it (`nhanTT`), verbatim. */}
          <TaskStatusBadge status={nv.status}>{nhanTrangThai(nhanTT, nv.status)}</TaskStatusBadge>
          <span className="text-[11px] whitespace-nowrap text-ink-muted tabular-nums">
            {`Hạn ${nhanNgay(nv.due_at)}`}
          </span>
        </li>
      ))}
    </ul>
  );
}

/**
 * Một dòng kết luận §2: số thứ tự trong ô tròn · nội dung · chip trạng thái (máy chủ suy) · dòng
 * phụ đếm nhiệm vụ · danh sách nhiệm vụ mở rộng được · các nút theo `quyTacKetLuan`.
 *
 * SỐ THỨ TỰ LẤY TỪ `ordinal`, KHÔNG TỪ VỊ TRÍ TRONG MẢNG — nó đi vào ô tròn, vào tên đọc được của
 * mọi nút, và vào `{stt}` của mọi tuyến sửa/gỡ/đánh dấu/tách.
 */
export function DongKetLuan({
  bienBan,
  ketLuan,
  tach,
  vongDoi,
}: {
  bienBan: petitions_bienBanRa;
  ketLuan: petitions_ketLuanRa;
  tach: PhepTach;
  vongDoi: PhepVongDoi;
}) {
  const qt = quyTacKetLuan(bienBan, ketLuan);
  const so = soThuTuKetLuan(ketLuan);
  const dangMo = tach.moOKetLuan === ketLuan.id;
  const loiTach = tach.loi?.ketLuanID === ketLuan.id ? tach.loi.thongBao : null;
  const loi = vongDoi.loi?.dich === ketLuan.id ? vongDoi.loi.thongBao : null;
  const hop = vongDoi.hop?.dich === ketLuan.id ? vongDoi.hop.loai : null;
  const nhiemVu = vongDoi.nhiemVuKL.get(ketLuan.id);

  const moNhiemVu = nhiemVu !== undefined;
  // Sửa and Gỡ are locked for ONE reason (`quyTacKetLuan`): ONE sentence in the DOM, both buttons
  // point `aria-describedby` at it (spec 05 §A9 — it used to be drawn twice).
  const lockReason = qt.sua.viSaoTat ?? qt.go.viSaoTat;
  const lockId = `ly-do-kl-${ketLuan.id}`;

  // THE PROTOTYPE'S ROW (`MeetingMinutes.tsx:130-153`): ordinal circle · the sentence over its
  // sub-line · `Tách thành nhiệm vụ` on the right. Ours adds, inside the middle column, what the
  // prototype has no state for (spec 05 §B/§C): the server-derived status pill on the sub-line, the
  // action row (split-task list toggle, draft-only Sửa / Gỡ / Đánh dấu), the lock sentence, the list.
  return (
    <div className="flex min-w-0 flex-wrap items-start gap-3 sm:flex-nowrap">
      <ConclusionOrdinal n={so} className="mt-0.5" />
      <div className="min-w-0 flex-1 basis-48">
        {hop === "sua-kl" ? (
          <FormSuaKetLuan
            ketLuan={ketLuan}
            dangGui={vongDoi.dangGui}
            huy={vongDoi.dongHop}
            luu={(noiDung) => vongDoi.suaKL(bienBan, ketLuan, noiDung)}
          />
        ) : (
          <p className="m-0 text-[12.8px] break-words">{ketLuan.content}</p>
        )}
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <span className="text-[11px] text-ink-muted tabular-nums">{nhanTienDoKetLuan(ketLuan)}</span>
          <ConclusionStatusBadge conclusion={ketLuan}>{nhanTrangThaiKetLuan(ketLuan)}</ConclusionStatusBadge>
        </div>

        {(ketLuan.task_count > 0 || qt.sua.hien || qt.go.hien || qt.danhDau.hien || qt.boDau.hien) && (
          <div className="mt-2 flex flex-wrap items-center gap-1">
            {ketLuan.task_count > 0 && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className={CONCLUSION_ACTION}
                icon={<Glyph icon={ListChecks} />}
                aria-expanded={moNhiemVu}
                aria-label={`${nhanNutXemNhiemVu(ketLuan, moNhiemVu)} — kết luận số ${so}`}
                onClick={() => vongDoi.batNhiemVuKL(bienBan, ketLuan)}
              >
                {nhanNutXemNhiemVu(ketLuan, moNhiemVu)}
                <Glyph icon={ChevronDown} className={cn("transition-transform", moNhiemVu && "rotate-180")} />
              </Button>
            )}
            <NutCoLyDo
              quyTac={qt.sua}
              id={`sua-kl-${ketLuan.id}`}
              nhan={NHAN_NUT_SUA_KL}
              ariaLabel={`Sửa kết luận số ${so}`}
              icon={Pencil}
              variant="ghost"
              size="sm"
              className={CONCLUSION_ACTION}
              describedBy={lockId}
              dangGui={vongDoi.dangGui}
              moRa={hop === "sua-kl"}
              onClick={() => vongDoi.moHop(ketLuan.id, "sua-kl")}
            />
            <NutCoLyDo
              quyTac={qt.go}
              id={`go-kl-${ketLuan.id}`}
              nhan={NHAN_NUT_GO_KL}
              ariaLabel={`Gỡ kết luận số ${so}`}
              icon={Trash2}
              variant="ghost"
              size="sm"
              // Spec 05 §B: a red word on a ghost button, a 10% red only on hover — no red fill.
              className={cn(
                CONCLUSION_ACTION,
                "text-danger hover:bg-danger/10 hover:not-disabled:bg-danger/10 hover:not-disabled:text-danger",
              )}
              describedBy={lockId}
              dangGui={vongDoi.dangGui}
              moRa={hop === "go-kl"}
              onClick={() => vongDoi.moHop(ketLuan.id, "go-kl")}
            />
            <NutCoLyDo
              quyTac={qt.danhDau}
              id={`danh-dau-${ketLuan.id}`}
              nhan={NHAN_NUT_DANH_DAU}
              ariaLabel={`${NHAN_NUT_DANH_DAU} — kết luận số ${so}`}
              icon={CircleMinus}
              variant="ghost"
              size="sm"
              className={CONCLUSION_ACTION}
              dangGui={vongDoi.dangGui}
              onClick={() => vongDoi.danhDau(bienBan, ketLuan)}
            />
            <NutCoLyDo
              quyTac={qt.boDau}
              id={`bo-dau-${ketLuan.id}`}
              nhan={NHAN_NUT_BO_DAU}
              ariaLabel={`${NHAN_NUT_BO_DAU} — kết luận số ${so}`}
              icon={Undo2}
              variant="ghost"
              size="sm"
              className={CONCLUSION_ACTION}
              dangGui={vongDoi.dangGui}
              onClick={() => vongDoi.boDau(bienBan, ketLuan)}
            />
          </div>
        )}
        {lockReason !== null && (
          <p className="m-0 mt-1 text-[11px] text-ink-muted" id={lockId}>
            {lockReason}
          </p>
        )}
        {nhiemVu !== undefined && <DanhSachNhiemVuKetLuan tai={nhiemVu} nhanTT={vongDoi.nhanTT} />}

        {loi !== null && (
          <p className="thong-bao-loi m-0 mt-2" role="alert">
            {loi}
          </p>
        )}

        {/* Spec 05 §B: `Gỡ` confirms in a DIALOG — with the reason field rule 7 requires. */}
        {hop === "go-kl" && (
          <FormLyDo
            id={`o-ly-do-go-kl-${ketLuan.id}`}
            question={`Gỡ kết luận số ${so}?`}
            nhan={`Lý do gỡ kết luận số ${so} *`}
            nhanNut={`Gỡ kết luận số ${so}`}
            ghiChu="Gỡ là xoá mềm. Số thứ tự đã cấp không cấp lại cho kết luận thêm sau."
            dangGui={vongDoi.dangGui}
            huy={vongDoi.dongHop}
            gui={(lyDo) => vongDoi.goKL(bienBan, ketLuan, lyDo)}
          />
        )}
      </div>

      {/* KHÔNG CÓ CỔNG QUYỀN Ở ĐÂY: tài khoản thiếu `task.create` nhận nguyên câu 403 của máy chủ
          trong hộp Tách. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1). */}
      {qt.tach.hien && (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="shrink-0"
          icon={<Glyph icon={Scissors} />}
          aria-label={nhanNutTach(ketLuan)}
          aria-haspopup="dialog"
          disabled={tach.dangGui}
          onClick={() => tach.mo(ketLuan.id)}
        >
          {NHAN_NUT_TACH}
        </Button>
      )}

      {dangMo && qt.tach.hien && (
        // THE PROTOTYPE'S SPLIT DIALOG (`Tách kết luận thành nhiệm vụ`, the conclusion under it,
        // `Huỷ` / `Tạo nhiệm vụ`) — with `FormGiaoViec`'s fields, NOT the prototype's four. A
        // four-field box sends a valid body but loses `Lãnh đạo giao việc`, which `PATCH` can never
        // set afterwards: nobody could ever approve that task's extension (ADR 0038). It would also
        // drop the commune's default priority and the +7-day due rule (ADR 0065 NV6), and be a
        // second form posting a task body — the drift `tieuDeCoSan` exists to avoid. One form.
        <FormGiaoViec
          dialog
          dialogTitle={SPLIT_DIALOG_TITLE}
          dialogDescription={ketLuan.content}
          submitLabel={SPLIT_SUBMIT_LABEL}
          lead={
            <div className="flex flex-col gap-1.5 [&>*]:my-0">
              <p className="ghi-chu m-0 inline-flex items-center gap-1.5">
                <LockKeyhole aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
                {NGUON_GIAO_KHOA}
              </p>
              {/* Spec §3 "Hạn gợi ý" belongs beside the deadline field, whose markup is shared with
                  Nhiệm vụ and Phản ánh — so it heads the dialog instead. */}
              <SuggestedDeadlineHint />
            </div>
          }
          danhMuc={tach.danhMuc}
          danhBa={tach.danhBa}
          danhBaLanhDao={tach.danhBaLanhDao}
          dangGui={tach.dangGui}
          loi={loiTach}
          huy={tach.dong}
          giaoViec={(than, khoaChongTrung) => tach.gui(bienBan.id, ketLuan, than, khoaChongTrung)}
          tieuDeCoSan={ketLuan.content}
        />
      )}
    </div>
  );
}

/** Hàng thêm kết luận §2, ở cuối mỗi thẻ DỰ THẢO. `POST /api/v1/meetings/{id}/conclusions`. */
export function HangThemKetLuan({
  bienBanID,
  dangGui,
  adding,
  gui,
}: {
  bienBanID: string;
  dangGui: boolean;
  /** THIS card's add is in flight — the Plus becomes a spinning loader (prototype `isPending`). */
  adding: boolean;
  gui: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
}) {
  const [noiDung, datNoiDung] = useState("");
  // Sinh ở chỗ MỞ hàng, không ở chỗ gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng khoá ấy.
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const canGon = noiDung.trim();

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (canGon === "") return;
    gui(bienBanID, canGon, khoaChongTrung);
  }

  // THE PROTOTYPE'S ROW: a two-line box and `+ Thêm kết luận` beside it, no visible label — the
  // placeholder says what to type; the label stays for assistive technology.
  return (
    <form className="m-0 mt-3 flex items-start gap-2" onSubmit={guiNgay}>
      <div className="o-nhap mb-0 min-w-0 flex-1">
        <label htmlFor={`them-ket-luan-${bienBanID}`} className="an-thi-giac">
          Thêm một kết luận
        </label>
        <textarea
          id={`them-ket-luan-${bienBanID}`}
          name="noi-dung-ket-luan"
          rows={2}
          className="text-[12.5px]"
          value={noiDung}
          placeholder={PLACEHOLDER_KET_LUAN}
          maxLength={NOI_DUNG_KET_LUAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
      </div>
      {/* No busy words: the prototype only swaps the icon. Disabled while anything on the screen is
          sending (`dangGui`), exactly as before; only THIS card's button spins (`adding`). */}
      <button
        type="submit"
        className={cn(LEGACY_BUTTON_CLASS.secondary, buttonVariants({ variant: "secondary" }), "shrink-0")}
        disabled={dangGui || canGon === ""}
        aria-busy={adding}
      >
        {adding ? (
          <LoaderCircle aria-hidden="true" focusable="false" className="size-4 motion-safe:animate-spin" />
        ) : (
          <Glyph icon={Plus} />
        )}
        {NHAN_NUT_THEM_KET_LUAN}
      </button>
    </form>
  );
}

/** Id of the `Nhập biên bản` dialog's heading — its accessible name. */
const MEETING_FORM_TITLE_ID = "tieu-de-nhap-bien-ban";

/**
 * Biểu mẫu biên bản §4 — NHẬP MỚI, NHẬP BỔ SUNG cho một biên bản đã ký, hoặc SỬA một bản nháp.
 *
 * THE PROTOTYPE'S DIALOG (`Nhập biên bản họp`, 500px — ADR 0068 lần 5): it replaced the in-page
 * block this was until 06/10/2026, a layout preference the prototype overrides (lần 5 #3). The
 * prototype's four fields keep its order — Tên → [Ngày họp | Số hiệu] → Nội dung — and ours sit
 * where they read naturally: Địa điểm after Số hiệu, Chủ trì / Thư ký and Thành phần before the
 * content, Các kết luận and the "?" attachment after it.
 *
 * Ô CUỐI "Tệp đính kèm" LÀ CHỖ GIỮ VÔ HIỆU (`ScanAttachmentField`): không có `name`, không vào
 * `GiaTriBieuMau`, nên không bao giờ đi lên cùng thân yêu cầu.
 *
 * BẢN SỬA KHÔNG CÓ Ô "Các kết luận": kết luận của bản nháp sửa, gỡ, thêm TỪNG DÒNG trên thẻ, vì mỗi
 * dòng mang số đã cấp và có thể đã có nhiệm vụ trỏ vào.
 */
export function FormNhapBienBan({
  che,
  danhBa,
  loiDanhBa,
  dangGui,
  loi,
  huy,
  luu,
  sua,
}: {
  che: CheBieuMau;
  danhBa: readonly identity_canBoChonNguoiRa[];
  loiDanhBa: string | null;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (than: TaoBienBanVao, khoaChongTrung: string) => void;
  sua: (bienBanID: string, than: SuaBienBanVao) => void;
}) {
  const bangDanhBa = danhBa.length === 0 ? null : danhBaTheoMa(danhBa);
  const [gt, datGt] = useState<GiaTriBieuMau>(() =>
    che.loai === "sua" ? giaTriTuBienBan(che.ban, bangDanhBa) : BIEU_MAU_TRONG,
  );
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  // Set by the first press of `Lưu`: from then on an empty required field says so under itself
  // (ADR 0068 lần 6 #4) — the button is no longer greyed out for missing input.
  const [triedSave, setTriedSave] = useState(false);

  const doi = (moi: Partial<GiaTriBieuMau>) => datGt((cu) => ({ ...cu, ...moi }));

  // HAI TRƯỜNG BẮT BUỘC, ĐÚNG HAI: "Tên cuộc họp" và "Ngày họp" (§4).
  const duTruongBatBuoc = gt.ten.trim() !== "" && gt.ngay !== "";
  const titleError = triedSave && gt.ten.trim() === "" ? MEETING_TITLE_REQUIRED : undefined;
  const dateError = triedSave && gt.ngay === "" ? MEETING_DATE_REQUIRED : undefined;
  const thanSua = che.loai === "sua" ? thanSuaTuBieuMau(gt, che.ban) : null;
  // Bản sửa không đổi gì thì không có gì để lưu — một PATCH rỗng không ghi được gì. That one stays a
  // greyed-out button: there is no field to point an error at.
  const nothingToSave = che.loai === "sua" && thanSua === null;

  const tieuDe =
    che.loai === "sua"
      ? "Sửa biên bản (dự thảo)"
      : che.boSungCho !== null
        ? "Lập biên bản bổ sung"
        : CREATE_MEETING_TITLE;
  // The line under the heading: the prototype's sentence for a new record; for a supplement, WHICH
  // signed record it supplements; nothing for an edit (the heading already says it).
  const moTa =
    che.loai === "sua" ? undefined : che.boSungCho !== null ? (
      <>
        Bổ sung cho biên bản đã ký: <strong>{che.boSungCho.title}</strong> · {dongMeta(che.boSungCho)}
      </>
    ) : (
      CREATE_MEETING_DESCRIPTION
    );

  function luuNgay(e: FormEvent) {
    e.preventDefault();
    setTriedSave(true);
    if (!duTruongBatBuoc || nothingToSave) return;
    if (che.loai === "sua") {
      if (thanSua !== null) sua(che.ban.id, thanSua);
      return;
    }
    luu(thanTaoTuBieuMau(gt, che.boSungCho?.id ?? null), khoaChongTrung);
  }

  const chuaChon = danhBa.filter((cb) => !gt.thanhPhanCanBo.includes(cb.code));

  // Esc asks `huy`, as `Huỷ` does — except while a save is in flight (the button is disabled too).
  return (
    <ModalDialog titleId={MEETING_FORM_TITLE_ID} onDismiss={() => (dangGui ? undefined : huy())}>
      <ModalDialogHeader titleId={MEETING_FORM_TITLE_ID} title={tieuDe} description={moTa} />
      <form className="m-0 flex min-h-0 flex-col gap-4" onSubmit={luuNgay}>
        {/* The fields scroll between the header and the buttons (`ModalDialog` layout contract).
            `[&_.o-nhap]:mb-0`: the column `gap` is the one spacing between fields; the legacy 1rem
            bottom margin of `.o-nhap` would double it. Labels stay above their controls. */}
        <div className="flex min-h-0 flex-col gap-4 overflow-y-auto pr-1 [&_.o-nhap]:mb-0">
          {/* ĐÃ QUYẾT 25/09/2026: nội dung mật không bao giờ vào ViGov — không có cờ "mật" nào. */}
          <Notice tone="legal" role="note">
            {CANH_BAO_BI_MAT}
          </Notice>

          {/* Labels are the prototype's `Field` (spec 04: navy 13px semibold, red `*`). The two
              required fields carry their error under themselves once `Lưu` was pressed. */}
          <Field label="Tên cuộc họp" htmlFor="ten-cuoc-hop" required grow="auto" className="min-w-0" error={titleError}>
            <input
              id="ten-cuoc-hop"
              name="ten-cuoc-hop"
              value={gt.ten}
              maxLength={TEN_CUOC_HOP_TOI_DA}
              autoComplete="off"
              placeholder="Giao ban tuần 34 năm 2026"
              aria-invalid={titleError === undefined ? undefined : true}
              onChange={(e) => doi({ ten: e.target.value })}
            />
          </Field>

          {/* The prototype's two columns; one column under 640px so a date box never squeezes. */}
          <div className="grid items-start gap-3 sm:grid-cols-2">
            <Field label="Ngày họp" htmlFor="ngay-hop" required grow="auto" className="min-w-0" error={dateError}>
              {/* `type="date"` trả đúng `2026-08-05` — ngày lịch, không múi giờ. */}
              <input
                id="ngay-hop"
                name="ngay-hop"
                type="date"
                value={gt.ngay}
                aria-invalid={dateError === undefined ? undefined : true}
                onChange={(e) => doi({ ngay: e.target.value })}
              />
            </Field>

            <Field label="Số hiệu biên bản" htmlFor="so-hieu-bien-ban" grow="auto" className="min-w-0">
              <input
                id="so-hieu-bien-ban"
                name="so-hieu-bien-ban"
                value={gt.soHieu}
                maxLength={SO_HIEU_TOI_DA}
                autoComplete="off"
                placeholder="12/BB-UBND"
                onChange={(e) => doi({ soHieu: e.target.value })}
              />
            </Field>
          </div>

          <Field label="Địa điểm" htmlFor="dia-diem-hop" grow="auto" className="min-w-0">
            <input
              id="dia-diem-hop"
              name="dia-diem-hop"
              value={gt.diaDiem}
              maxLength={DIA_DIEM_TOI_DA}
              autoComplete="off"
              onChange={(e) => doi({ diaDiem: e.target.value })}
            />
          </Field>

          {/* Owner's field order (09/10/2026): the content right after the prototype's own fields. */}
          <Field label="Nội dung biên bản" htmlFor="noi-dung-bien-ban" grow="auto" className="min-w-0">
            <textarea
              id="noi-dung-bien-ban"
              name="noi-dung-bien-ban"
              rows={5}
              value={gt.noiDung}
              maxLength={NOI_DUNG_BIEN_BAN_TOI_DA}
              placeholder="Dán nội dung biên bản vào đây…"
              onChange={(e) => doi({ noiDung: e.target.value })}
            />
          </Field>

          {/* Chủ trì and Thư ký side by side (owner, 02/10/2026). Their markup is `OChonCanBo`'s
              own — label above, full column width. */}
          <div className="grid gap-3 sm:grid-cols-2">
            <OChonCanBo
              id="chu-tri-bien-ban"
              nhan="Chủ trì"
              nhanTrong={KHONG_GHI_CAN_BO}
              giaTri={gt.chuTri}
              danhBa={danhBa}
              dat={(ma) => doi({ chuTri: ma })}
            />
            <OChonCanBo
              id="thu-ky-bien-ban"
              nhan="Thư ký"
              nhanTrong={KHONG_GHI_CAN_BO}
              giaTri={gt.thuKy}
              danhBa={danhBa}
              dat={(ma) => doi({ thuKy: ma })}
            />
          </div>
          {loiDanhBa !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              Chưa đọc được danh bạ cán bộ: {loiDanhBa}
            </p>
          )}

          <fieldset className="o-nhap m-0 flex min-w-0 flex-col gap-3 rounded-xl border border-line p-4">
            <legend className="px-1">Thành phần tham dự</legend>
            {gt.thanhPhanCanBo.length > 0 && (
              <ul aria-label="Cán bộ tham dự đã chọn" className="m-0 flex list-none flex-wrap gap-2 p-0">
                {gt.thanhPhanCanBo.map((ma) => (
                  <li
                    key={ma}
                    className="inline-flex max-w-full items-center gap-1 rounded-full border border-line bg-surface-muted py-0.5 pr-0.5 pl-3 text-[13px] text-ink-900"
                  >
                    <span className="min-w-0 break-words">{nhanThanhPhan(ma, bangDanhBa)}</span>{" "}
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 rounded-full px-2"
                      icon={<Glyph icon={X} />}
                      aria-label={`Bỏ ${nhanThanhPhan(ma, bangDanhBa)} khỏi thành phần tham dự`}
                      onClick={() =>
                        doi({ thanhPhanCanBo: gt.thanhPhanCanBo.filter((m) => m !== ma) })
                      }
                    >
                      Bỏ
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            {/* `[&_label]:mb-0`: inside this `.o-nhap` fieldset the legacy label margin would stack
                on the column's own 6px gap. One column: the dialog is 500px wide. */}
            <div className="flex flex-col gap-3 [&_label]:mb-0">
              {/* Field wraps the screen's own native `<select>` — same id, value and onChange; it
                  only draws the label above, the 40px frame and the chevron (owner, 02/10/2026). */}
              <Field label="Thêm cán bộ tham dự" htmlFor="chon-thanh-phan" kind="select" grow="auto" className="min-w-0">
                <select
                  id="chon-thanh-phan"
                  className="o-chon"
                  value=""
                  onChange={(e) => {
                    const ma = e.target.value;
                    if (ma !== "") doi({ thanhPhanCanBo: [...gt.thanhPhanCanBo, ma] });
                  }}
                >
                  <option value="">— Chọn cán bộ —</option>
                  {chuaChon.map((cb) => (
                    <option key={cb.code} value={cb.code}>
                      {nhanThanhPhan(cb.code, bangDanhBa)}
                    </option>
                  ))}
                </select>
              </Field>
              <div className="flex min-w-0 flex-col gap-1.5">
                <label htmlFor="thanh-phan-khac">Thành phần khác</label>
                <textarea
                  id="thanh-phan-khac"
                  name="thanh-phan-khac"
                  rows={2}
                  value={gt.thanhPhanKhac}
                  // KHÔNG CÓ `maxLength`: trần máy chủ là 200 dòng VÀ 200 ký tự mỗi dòng — một con số
                  // trên cả ô sẽ cắt sai ở cả hai chiều. Máy chủ từ chối bằng câu nói đúng dòng nào sai.
                  onChange={(e) => doi({ thanhPhanKhac: e.target.value })}
                />
                <p className={HINT}>
                  Mỗi dòng một người — khách mời, đại diện thôn, người không có tài khoản trên hệ thống.
                </p>
              </div>
            </div>
          </fieldset>

          {che.loai === "tao" && (
            <Field
              label="Các kết luận"
              htmlFor="cac-ket-luan"
              grow="auto"
              className="min-w-0"
              hint={
                <>
                  Mỗi dòng một kết luận, đánh số ① ② ③ theo thứ tự nhập. Tối đa{" "}
                  {KET_LUAN_MOI_LAN_TOI_DA} kết luận một lần nhập; thêm tiếp bằng nút “
                  {NHAN_NUT_THEM_KET_LUAN}” ở cuối thẻ. Để trống cũng lưu được.
                </>
              }
            >
              <textarea
                id="cac-ket-luan"
                name="cac-ket-luan"
                rows={4}
                value={gt.ketLuan}
                onChange={(e) => doi({ ketLuan: e.target.value })}
              />
            </Field>
          )}

          {/* Last field of spec §4, a disabled placeholder — see the block comment above. */}
          <ScanAttachmentField />

          {loi !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {loi}
            </p>
          )}
        </div>

        {/* The prototype's buttons: `Huỷ` then the save, right-aligned; the save stays LAST. */}
        <div className="flex shrink-0 justify-end gap-2">
          <CancelButton dangGui={dangGui} huy={huy} />
          {/* The prototype keeps the save's own words while saving, with a spinner (MM:396-399). */}
          <SubmitButton
            label={che.loai === "sua" ? NHAN_NUT_LUU_SUA : NHAN_NUT_LUU}
            busyText={che.loai === "sua" ? NHAN_NUT_LUU_SUA : NHAN_NUT_LUU}
            busy={dangGui}
            disabled={dangGui || nothingToSave}
          />
        </div>
      </form>
    </ModalDialog>
  );
}
