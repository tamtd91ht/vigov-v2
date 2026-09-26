"use client";

import Link from "next/link";
import { useEffect, useReducer, useRef, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { duongDanBienBan } from "@/features/bien-ban/nhan-bien-ban";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import {
  deNghiLuiHan,
  doiTrangThaiNhiemVu,
  layNhiemVu,
  laySoNhiemVu,
  quyetDinhLuiHan,
  suaNhiemVu,
  taoNhiemVu,
  xoaNhiemVu,
  type LocNhiemVu,
} from "@/lib/api/nhiem-vu";
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";
import type {
  identity_boPhanRa,
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
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_HAN_MOT_LAN,
  CAU_LOC_TRANG_THAI_KHONG_CO_COT,
  CHI_QUA_HAN_NHAN,
  CHI_TIET_THIEU_VAN_BAN,
  CHUA_GIAO_BO_PHAN,
  CHUA_PHAN_CONG,
  CHUA_XAC_DINH,
  CHU_THICH_HAI_O_TICK,
  COT_RONG,
  DANG_TAI_SO,
  DANG_TAI_VAN_BAN,
  GHI_CHU_DEM_COT,
  GHI_CHU_HAN_VIEC_CON,
  GHI_CHU_NHIEM_VU_TOI_DA,
  GHI_CHU_KHONG_CO_O_GHI_CHU,
  GHI_CHU_LANH_DAO_GIAO_VIEC,
  GHI_CHU_LUI_HAN,
  GHI_CHU_THIEU_SO_THEO_DOI,
  GHI_CHU_TU_SINH_MA,
  KHONG_DOC_DUOC_VAN_BAN,
  LY_DO_KHONG_SUA_CHU_TRI,
  LY_DO_KHONG_SUA_HAN,
  LY_DO_KHONG_SUA_MA,
  MOI_BO_PHAN_NHAN,
  MOI_KHOI_NHAN,
  MOI_LOAI_NHAN,
  MOI_MUC_UU_TIEN_NHAN,
  MOI_NGUON_GIAO,
  MOI_NGUON_GIAO_NHAN,
  MOI_NHOM_VAN_BAN,
  MOI_TRANG_THAI,
  MOI_TRANG_THAI_NHAN,
  MO_TA_FORM_GIAO_VIEC,
  NHAN_CHE_DO_DANH_SACH,
  NHAN_CHE_DO_KANBAN,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_SUA,
  NHAN_THEM_VAN_BAN,
  NHAN_TIEU_DE_THEO_VAN_BAN,
  O_TRONG,
  PHAM_VI_CUA_TOI,
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
  canDocLaiTruocKhiLuu,
  canhBaoSua,
  canhBaoVanBan,
  cauGiaiThichTrangThai,
  cauTuKetLuan,
  chiaNhomVanBan,
  chuyenSangDuoc,
  coKhoiVanBanChiDao,
  cotPhaiDoc,
  docBangNhanTrangThai,
  dongCuaNhom,
  dongVanBan,
  formSuaTuChiTiet,
  ghiChuKanbanReNhanh,
  loiSauKhiDocLai,
  lyDoKhoaSua,
  hoanThanhTreHan,
  mocCuoiNgay,
  nhanBoDem,
  nhanDemCot,
  nhanHanThe,
  nhanHoanThanhTreHan,
  nhanNgay,
  nhanNguonGiao,
  nhanNhomVanBan,
  nhanNutGoVanBan,
  nhanOTieuDe,
  nhanTrangThai,
  oHan,
  placeholderNhomVanBan,
  quyetDinhDuyetLuiHan,
  thanGiaoViec,
  thanSuaNhiemVu,
  theoThuTuXa,
  type BangNhanTrangThai,
  type DongVanBanNhap,
  type DongVanBanSua,
  type FormSuaNhiemVu,
  type NhomVanBan,
  type TrangThaiNhiemVu,
} from "./nhan-nhiem-vu";

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
 * KHÔNG CÓ CỔNG QUYỀN Ở GIAO DIỆN, và sự vắng mặt ấy được nói ra ở `PHAN_CHUA_DUNG` chứ không
 * giấu: `lib/quyen.ts` chưa có hằng cho bảy khoá `task.*` và lượt này không sửa tệp ấy. Lớp chặn
 * thật không đổi — mỗi tuyến khai `RequirePermission` và kiểm trên TỪNG yêu cầu (luật 5, cấm #1);
 * thứ thiếu là sự tiện dụng, cùng khuôn `document.read` đã chọn. NGOẠI LỆ DUY NHẤT là lớp hai của
 * ADR 0038, vì nó KHÔNG phải một khoá quyền: nó là phép so mã cán bộ với cột trên bản ghi, và nó
 * vẫn chạy đầy đủ ở đây.
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
 * tả không vẽ nút trang nào trên bảng. Cột đầy thì đầu cột hiện `20+` và cán bộ sang Danh sách.
 */
const SO_THE_MOI_COT = 20;

/** Hai chế độ xem dựng được (§2, §4). Kanban là MẶC ĐỊNH, đúng tiêu đề §4.1. */
type CheDoXem = "kanban" | "danh-sach";

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

export function SoNhiemVu() {
  const [loc, datLoc] = useState<BoLoc>(KHONG_LOC);
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [cheDoXem, datCheDoXem] = useState<CheDoXem>("kanban");

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

  const [drawer, guiDrawer] = useReducer(chuyenDrawer, null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);
  const [moFormTao, datMoFormTao] = useState(false);

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}`;
  // Kanban KHÔNG mang con trỏ: nó không phân trang, nên bộ lọc và lần ghi gần nhất là tất cả những
  // gì làm câu trả lời cũ hết hiệu lực.
  const khoaKanban = `${JSON.stringify(loc)}|${lanTai}`;

  useEffect(() => {
    // CHỈ ĐỌC CHẾ ĐỘ ĐANG XEM. Không có dòng này thì mỗi lần đổi bộ lọc trên Kanban là SÁU lời gọi
    // thay vì năm, và lời gọi thứ sáu đọc một trang không ai vẽ ra.
    if (cheDoXem !== "danh-sach") return;
    let bo = false;
    laySoNhiemVu({ ...loc, limit: SO_DONG_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, nganXep.hienTai, khoa, cheDoXem]);

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
    if (cheDoXem !== "kanban") return;
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
    return () => {
      bo = true;
    };
  }, [loc, khoaKanban, cheDoXem]);

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

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có mã cán bộ — và không có mã thì
  // không so được với `lanh_dao_giao_viec_ma`, nên nút duyệt lùi hạn ẩn (luật 1, cấm #1).
  const maNguoiDangNhap = phien !== null && phien.ok ? phien.duLieu.staff.code : "";

  const so = taiTu(daTai, khoa);
  const cotKanban: readonly CotKanban[] = cotPhaiDoc(loc.trangThai).map((ma) => ({
    ma,
    tai: taiCot(daTaiKanban, khoaKanban, ma),
  }));
  const tenBoPhan = new Map(danhMuc.boPhan.map((b) => [b.id, b.name]));
  const { bang: nhanTT, canhBao: canhBaoNhanTT } = docBangNhanTrangThai(kqNhanTT);

  /** Đổi bộ lọc là về trang đầu: con trỏ của bộ lọc cũ không có nghĩa với bộ lọc mới. */
  function datLocMoi(moi: BoLoc): void {
    datLoc(moi);
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

  return (
    <section className="man-nhiem-vu" aria-labelledby="tieu-de-so-nhiem-vu">
      <h2 id="tieu-de-so-nhiem-vu">Sổ nhiệm vụ của xã</h2>

      <KhoiChuaDung />

      <CanhBaoNhanTrangThai canhBao={canhBaoNhanTT} />

      <div className="cum-nut">
        <button
          type="button"
          className="nut-chinh"
          onClick={() => {
            datMoFormTao((m) => !m);
            datLoiGhi(null);
          }}
          aria-expanded={moFormTao}
        >
          {moFormTao ? "Đóng biểu mẫu giao việc" : "+ Giao việc mới"}
        </button>
      </div>

      {moFormTao && (
        <FormGiaoViec
          danhMuc={danhMuc}
          // `POST /api/v1/tasks` nhận `documents`; màn Biên bản thì không — xem prop.
          coDanhSachVanBan
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

      <HangLoc
        loc={loc}
        tim={tim}
        datTim={datTim}
        datLoc={datLocMoi}
        danhMuc={danhMuc}
        nhanTT={nhanTT}
      />

      {/* CỤM CHỌN CHẾ ĐỘ XEM — §2, bên phải hàng lọc 2. HAI nút chứ không phải ba: `Sổ theo dõi`
          §4.3 cần `documents` trên tuyến đọc sổ, và tuyến ấy cố ý không trả — xem `PHAN_CHUA_DUNG`. */}
      <div className="o-chon" role="group" aria-label="Chế độ xem">
        <button
          type="button"
          className="nut-phu"
          aria-pressed={cheDoXem === "kanban"}
          onClick={() => datCheDoXem("kanban")}
        >
          {NHAN_CHE_DO_KANBAN}
        </button>
        <button
          type="button"
          className="nut-phu"
          aria-pressed={cheDoXem === "danh-sach"}
          onClick={() => datCheDoXem("danh-sach")}
        >
          {NHAN_CHE_DO_DANH_SACH}
        </button>
      </div>
      <p className="ghi-chu">{GHI_CHU_THIEU_SO_THEO_DOI}</p>

      {cheDoXem === "kanban" && (
        <BangKanban
          cot={cotKanban}
          danhMuc={danhMuc}
          nhanTT={nhanTT}
          bayGio={new Date()}
          maDangMo={maDrawer}
          moNhiemVu={(n) => {
            guiDrawer({ loai: "mo", nhiemVu: n });
            datLoiGhi(null);
          }}
        />
      )}

      {cheDoXem === "danh-sach" && so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {cheDoXem === "danh-sach" && so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
      )}

      {cheDoXem === "danh-sach" && so.pha === "xong" && (
        <>
          <BangNhiemVu
            nhiemVu={so.duLieu.items}
            danhMuc={danhMuc}
            nhanTT={nhanTT}
            tenBoPhan={tenBoPhan}
            bayGio={new Date()}
            maDangMo={maDrawer}
            moNhiemVu={(n) => {
              guiDrawer({ loai: "mo", nhiemVu: n });
              datLoiGhi(null);
            }}
          />
          <p className="ghi-chu">{nhanBoDem(so.duLieu.items.length)}</p>
          <nav className="dieu-huong-trang" aria-label="Phân trang sổ nhiệm vụ">
            <button
              type="button"
              className="nut-phu"
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </button>
            <button
              type="button"
              className="nut-phu"
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
            </button>
          </nav>
        </>
      )}

      {drawer !== null && (
        <ChiTietNhiemVu
          nhiemVu={drawer.nhiemVu}
          vanBan={drawer.vanBan}
          danhMuc={danhMuc}
          nhanTT={nhanTT}
          tenBoPhan={tenBoPhan}
          bayGio={new Date()}
          maNguoiDangNhap={maNguoiDangNhap}
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
              guiDrawer({ loai: "dong" });
              datLanTai((n) => n + 1);
            });
          }}
          guiDeNghiLuiHan={(hanMoi, lyDo) => deNghiLuiHan(drawer.nhiemVu.code, hanMoi, lyDo)}
          quyetDinh={(deNghiID, duyet, ghiChu) =>
            quyetDinhLuiHan(drawer.nhiemVu.code, deNghiID, duyet, ghiChu)
          }
          // §5.4 `✎ Sửa`. Thành công: phản hồi PATCH MANG `documents` (`app/nhiem_vu.go:696-741`),
          // nên `ghiXong` thay cả trường vô hướng lẫn khối văn bản, rồi đọc lại sổ như mọi lần ghi
          // khác. Hỏng: trả `KetQua` nguyên vẹn về form, để form GIỮ chữ cán bộ đã gõ và in câu máy
          // chủ — không đóng form, không xoá gì.
          suaKhoiVanBan={(than) =>
            suaNhiemVu(drawer.nhiemVu.code, than).then((kq) => {
              if (kq.ok) {
                datLoiGhi(null);
                guiDrawer({ loai: "ghiXong", nhiemVu: kq.duLieu });
                datLanTai((n) => n + 1);
              }
              return kq;
            })
          }
          docLaiChiTiet={() => layNhiemVu(drawer.nhiemVu.code)}
        />
      )}
    </section>
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
 * TÁM Ô, ĐÚNG TÁM THAM SỐ MÁY CHỦ NHẬN — không vẽ ô nào không có tuyến đứng sau. Hai ô của đặc
 * tả vắng mặt CÓ CHỦ Ý và lý do ra tới `PHAN_CHUA_DUNG`: tab `Liên quan đến tôi` và ô tick
 * `Sắp đến hạn` đều bị máy chủ TỪ CHỐI bằng 400 kèm lý do, nên vẽ chúng ra là vẽ hai ô mà mỗi lần
 * bấm đổi quyển sổ thành một trang lỗi.
 *
 * Ô `Người thực hiện` là Ô GÕ MÃ CÁN BỘ chứ không phải ô chọn: danh bạ đứng sau `admin.user` —
 * xem `PHAN_CHUA_DUNG`.
 */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  danhMuc,
  nhanTT,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  danhMuc: DanhMucNhiemVu;
  /** Nhãn và thứ tự bảy trạng thái của xã — ô lọc hiện đúng chữ và thứ tự xã đặt. */
  nhanTT: BangNhanTrangThai;
}) {
  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  return (
    <div className="hang-loc">
      {/* HAI TAB PHẠM VI. `mine` KHÔNG mang theo danh tính nào — máy chủ điền mã cán bộ từ PHIÊN
          (`nhiem_vu.go:273-288`). Một tab gửi lên `?assignee=CB-…` của chính mình sẽ là client tự
          khai mình là ai, điều luật 1 cấm #2 không cho phép. */}
      <div className="o-chon" role="group" aria-label="Phạm vi">
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc.phamVi !== "mine"}
          onClick={() => datLoc({ ...loc, phamVi: undefined })}
        >
          {PHAM_VI_TOAN_XA}
        </button>
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc.phamVi === "mine"}
          onClick={() => datLoc({ ...loc, phamVi: "mine" })}
        >
          {PHAM_VI_CUA_TOI}
        </button>
      </div>

      {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
          đang gõ vào một URL, và một URL đi vào mọi log truy cập (luật 3, cấm #4). */}
      <form className="form-tra-cuu" onSubmit={timNgay} role="search">
        <div className="o-nhap">
          <label htmlFor="tim-nhiem-vu">Tìm trong sổ</label>
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
        </div>
        <button className="nut-phu" type="submit">
          Tìm
        </button>
      </form>

      <div className="o-chon">
        <label htmlFor="loc-trang-thai">Trạng thái</label>
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
      </div>

      <div className="o-chon">
        <label htmlFor="loc-loai">Loại nhiệm vụ</label>
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
      </div>

      <div className="o-chon">
        <label htmlFor="loc-khoi">Khối</label>
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
      </div>

      <div className="o-chon">
        <label htmlFor="loc-uu-tien">Mức ưu tiên</label>
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
      </div>

      <div className="o-chon">
        <label htmlFor="loc-nguon-giao">Nguồn giao</label>
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
      </div>

      <div className="o-chon">
        <label htmlFor="loc-bo-phan">Bộ phận</label>
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
      </div>

      <div className="o-nhap">
        <label htmlFor="loc-nguoi-thuc-hien">Người thực hiện (mã cán bộ)</label>
        <input
          id="loc-nguoi-thuc-hien"
          name="loc-nguoi-thuc-hien"
          value={loc.nguoiThucHienMa ?? ""}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datLoc({ ...loc, nguoiThucHienMa: e.target.value || undefined })}
        />
      </div>

      <div className="o-chon">
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
    </div>
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
 * KHÔNG CÓ KÉO-THẢ, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH VỀ KHẢ NĂNG TIẾP CẬN, KHÔNG PHẢI MỘT VIỆC CÒN DANG DỞ.
 *
 * Kéo-thả HTML5 không có lối bàn phím tương đương. Một bảng chỉ đổi được trạng thái bằng cách kéo
 * là một bảng mà cán bộ dùng bàn phím — hoặc cầm chuột không vững, cụ thể là phần lớn người dùng
 * lớn tuổi của màn này — KHÔNG thao tác được (`skills/accessibility-elderly`). Nên thẻ mở drawer,
 * và drawer có nguyên khối `Chuyển trạng thái` §6: CÙNG một tuyến `POST /api/v1/tasks/{ma}/status`,
 * cùng một chỗ in NGUYÊN VĂN câu từ chối của máy chủ — kể cả câu liệt kê mã việc con còn lại. Kanban
 * vì thế không đổi trạng thái tệ hơn Danh sách; nó đổi ở đúng chỗ Danh sách đổi.
 *
 * Sự vắng mặt ấy ra tới màn hình qua `PHAN_CHUA_DUNG`, không nằm lại trong chú thích này.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * Năm cột xếp dọc dưới 768px và nằm cạnh nhau từ 768px (xem khối đầu tệp). KHÔNG CÓ chấm màu đầu
 * cột lẫn viền trái tô theo mức ưu tiên: hai thứ ấy là màu, và màu không bao giờ là tín hiệu duy
 * nhất (a11y) — mức ưu tiên hiện thành CHỮ trên thẻ, tình trạng trễ hiện thành chữ `Trễ N ngày`.
 *
 * KHÔNG CÓ Ô TICK CHỌN HÀNG LOẠT, cùng lý do bảng §4.2 không có: `Xoá đã chọn` không có tuyến nào.
 */
export function BangKanban({
  cot,
  danhMuc,
  nhanTT,
  bayGio,
  maDangMo,
  moNhiemVu,
}: {
  cot: readonly CotKanban[];
  danhMuc: DanhMucNhiemVu;
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

  return (
    <>
      <div className="bang-cuon" role="region" aria-label="Bảng Kanban nhiệm vụ" tabIndex={0}>
        <div className="bang-kanban">
          {cotSap.map((c) => (
            <section key={c.ma} className="cot-kanban" aria-labelledby={`cot-kanban-${c.ma}`}>
              <h3 id={`cot-kanban-${c.ma}`}>
                {/* NHÃN CỘT LÀ NHÃN CỦA XÃ — cùng một chữ với chip và ô lọc. Chữ "Chưa thực hiện"
                    của §4.1 là thứ xã tự đặt cho `moi-giao` ở tab Danh mục (xem `nhan-nhiem-vu.ts`). */}
                {nhanTrangThai(nhanTT, c.ma)}{" "}
                {c.tai.pha === "xong" && (
                  <span className="chip chip-ngung">
                    {nhanDemCot(c.tai.duLieu.items.length, c.tai.duLieu.has_more)}
                  </span>
                )}
              </h3>

              {/* NĂM CỘT LÀ NĂM CÂU TRẢ LỜI RỜI NHAU. Một cột hỏng thì bốn cột kia vẫn là sổ —
                  và câu hỏng của nó hiện nguyên văn ở đúng cột ấy, không nuốt thành một lỗi chung. */}
              {c.tai.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
              {c.tai.pha === "loi" && (
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
                        nhanTT={nhanTT}
                        bayGio={bayGio}
                        maDangMo={maDangMo}
                        moNhiemVu={moNhiemVu}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))}
        </div>
      </div>

      <p className="ghi-chu">{nhanBoDem(soThe)}</p>
      <p className="ghi-chu">{GHI_CHU_DEM_COT}</p>
      {/* Chỗ một cán bộ tìm lại việc "biến mất" của mình: một việc vừa sang `tam-dung` rời khỏi
          Kanban hoàn toàn, và §4.1 muốn thế. Không nói ra thì người giao việc kết luận nó đã bị xoá. */}
      <p className="ghi-chu">{ghiChuKanbanReNhanh(nhanTT)}</p>
    </>
  );
}

/**
 * Một thẻ nhiệm vụ §4.1.
 *
 * MỨC ƯU TIÊN HIỆN THÀNH CHỮ. Đặc tả mã hoá nó bằng màu viền trái, mà màu một mình thì người không
 * phân biệt được màu đọc ra bằng không (a11y). Khi lớp CSS viền trái được thêm, nó là tín hiệu THỨ
 * HAI chồng lên chữ này chứ không thay chữ này.
 *
 * KHÔNG CÓ CHIP `{n} việc con`: phản hồi không mang số việc con — xem `PHAN_CHUA_DUNG`.
 */
export function TheNhiemVu({
  nhiemVu,
  danhMuc,
  nhanTT,
  bayGio,
  maDangMo,
  moNhiemVu,
}: {
  nhiemVu: petitions_nhiemVuRa;
  danhMuc: DanhMucNhiemVu;
  nhanTT: BangNhanTrangThai;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
}) {
  const treHan = oHan(nhiemVu.due_at, bayGio).phanTre !== "";

  return (
    <article className="the-nhiem-vu" aria-labelledby={`the-nhiem-vu-${nhiemVu.code}`}>
      <p className="ma-muc">{nhiemVu.code}</p>
      <p id={`the-nhiem-vu-${nhiemVu.code}`} className="tieu-de-the">
        {nhiemVu.title}
      </p>
      {/* `Trễ 87 ngày` · `Hạn 20/12/2026` · `Hạn —` — cùng một hàm với dải hạn của drawer. */}
      <p className={treHan ? "nhan-lech" : "dong-phu"}>
        <span aria-hidden="true">⏱ </span>
        {nhanHanThe(nhiemVu.due_at, bayGio)}
      </p>
      <p className="dong-phu">
        {nhiemVu.assignee === "" ? CHUA_PHAN_CONG : nhiemVu.assignee} ·{" "}
        {nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)}
      </p>
      {hoanThanhTreHan(nhiemVu.completed_at, nhiemVu.original_due_at) && (
        <p>
          <span className="chip chip-hoat-dong">{nhanHoanThanhTreHan(nhanTT)}</span>
        </p>
      )}
      <button
        type="button"
        className="nut-phu"
        onClick={() => moNhiemVu(nhiemVu)}
        aria-expanded={nhiemVu.code === maDangMo}
      >
        {nhiemVu.code === maDangMo ? "Đang mở" : `Mở ${nhiemVu.code}`}
      </button>
    </article>
  );
}

/**
 * Bảng Danh sách §4.2.
 *
 * KHÔNG CÓ CỘT Ô TICK: `Xoá đã chọn` không có tuyến nào (xem `PHAN_CHUA_DUNG`), và một ô tick
 * không dẫn tới thao tác nào là một ô tick mời cán bộ chọn hai mươi dòng rồi không tìm thấy nút.
 *
 * KHÔNG CÓ CHIP `{n} việc con`: phản hồi không mang số việc con, và đếm trong trang đang mở cho ra
 * một con số PHỤ THUỘC VÀO TRANG.
 */
export function BangNhiemVu({
  nhiemVu,
  danhMuc,
  nhanTT,
  tenBoPhan,
  bayGio,
  maDangMo,
  moNhiemVu,
}: {
  nhiemVu: readonly petitions_nhiemVuRa[];
  danhMuc: DanhMucNhiemVu;
  nhanTT: BangNhanTrangThai;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
}) {
  if (nhiemVu.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    <div className="bang-cuon" role="region" aria-label="Sổ nhiệm vụ" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Tên việc</th>
            <th scope="col">Người thực hiện</th>
            <th scope="col">Bộ phận</th>
            <th scope="col">Ưu tiên</th>
            <th scope="col">Hạn</th>
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
              <tr key={n.code}>
                <td className="ma-muc">{n.code}</td>
                <td>
                  {n.title}
                  <span className="dong-phu">{nhanNguonGiao(n.source)}</span>
                </td>
                <td>{n.assignee === "" ? CHUA_PHAN_CONG : n.assignee}</td>
                <td>{n.unit === "" ? O_TRONG : (tenBoPhan.get(n.unit) ?? n.unit)}</td>
                <td>{nhanDanhMuc(danhMuc.mucUuTien, n.priority)}</td>
                <td>
                  {o.ngay}
                  {/* PHẦN TRỄ TÁCH RIÊNG để tô đỏ, đúng §4.2. `oHan` trả hai mảnh sẵn, nên ở đây
                      không có phép cắt chuỗi nào. */}
                  {o.phanTre !== "" && <span className="nhan-lech"> {o.phanTre}</span>}
                </td>
                <td>
                  <span className="chip chip-ngung">{nhanTrangThai(nhanTT, n.status)}</span>
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

/**
 * Chi tiết §5 — dải bước §5.2, ba ô tóm tắt §5.3, hai hạn §5.6, đổi trạng thái §6, đề nghị lùi
 * hạn §5.8 và xoá §11.5.
 *
 * ĐẶC TẢ GỌI NÓ LÀ DetailDrawer "mở gần toàn màn hình". Ở đây nó là một khối nằm dưới bảng —
 * KHÔNG phải một lớp phủ — vì một lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này
 * không được thêm CSS. Đã báo về tên lớp cần thêm.
 */
export function ChiTietNhiemVu({
  nhiemVu,
  vanBan,
  danhMuc,
  nhanTT,
  tenBoPhan,
  bayGio,
  maNguoiDangNhap,
  dangGui,
  loiGhi,
  dong,
  doiTrangThai,
  xoa,
  guiDeNghiLuiHan,
  quyetDinh,
  suaKhoiVanBan,
  docLaiChiTiet,
}: {
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
  bayGio: Date;
  /** `phien.staff.code` — mã nghiệp vụ `CB-…`, rỗng khi chưa đọc được phiên. */
  maNguoiDangNhap: string;
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

  const o = oHan(nhiemVu.due_at, bayGio);
  const giaiThich = cauGiaiThichTrangThai(nhiemVu.status);
  const congDuyet = quyetDinhDuyetLuiHan(
    maNguoiDangNhap,
    nhiemVu.assigner,
    // ⚠ LỚP MỘT (`task.extend`) TRUYỀN VÀO `true` VÌ MÀN HÌNH CHƯA ĐỌC ĐƯỢC KHOÁ NÀO — xem
    // `PHAN_CHUA_DUNG`. Hệ quả có giới hạn và được nói thẳng: lãnh đạo được ghi trên bản ghi mà
    // THIẾU `task.extend` sẽ thấy nút rồi nhận nguyên văn câu 403 của máy chủ. Lớp hai — phép so
    // mã cán bộ của ADR 0038 — vẫn chạy đủ ngay dưới đây, và nó là lớp mà một khoá quyền không
    // diễn đạt được.
    true,
  );

  return (
    <div className="khoi-chi-tiet" aria-labelledby="tieu-de-chi-tiet-nhiem-vu">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-chi-tiet-nhiem-vu" className="ma-muc">
          [{nhiemVu.code}] {nhiemVu.title}
        </h3>
        <button type="button" className="nut-phu" onClick={dong} aria-label="Đóng chi tiết nhiệm vụ">
          ✕
        </button>
      </div>

      <p className="ghi-chu">
        {nhiemVu.unit === ""
          ? CHUA_GIAO_BO_PHAN
          : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit)}{" "}
        · {nhiemVu.assignee === "" ? CHUA_PHAN_CONG : nhiemVu.assignee}
      </p>

      {/* ── DẢI BƯỚC §5.2 ─────────────────────────────────────────────────────────────────
          THỜI GIAN ĐÃ Ở TRẠNG THÁI (`19 ngày 23 giờ`) HIỆN DẤU GẠCH, không hiện số 0: mốc đổi
          trạng thái gần nhất nằm trong nhật ký, và hợp đồng không có tuyến nhật ký nào. Một con
          số 0 ở đây đọc ra là "vừa chuyển xong", đúng điều ngược lại với "không biết". */}
      <ol aria-label="Các bước của vòng đời nhiệm vụ">
        {TRANG_THAI_CHINH.map((ma) => (
          <li key={ma}>
            <span
              className={ma === nhiemVu.status ? "chip chip-hoat-dong" : "chip chip-ngung"}
            >
              {nhanTrangThai(nhanTT, ma)}
            </span>{" "}
            {O_TRONG}
          </li>
        ))}
      </ol>
      <p className="ghi-chu">
        Rẽ nhánh:{" "}
        {TRANG_THAI_RE_NHANH.map((ma) => (
          <span key={ma}>
            <span
              className={ma === nhiemVu.status ? "chip chip-hoat-dong" : "chip chip-ngung"}
            >
              {nhanTrangThai(nhanTT, ma)}
            </span>{" "}
            {O_TRONG}{" "}
          </span>
        ))}
      </p>
      {giaiThich !== "" && <p className="ghi-chu">{giaiThich}</p>}

      <dl className="danh-sach-truong">
        {/* §5.3 — HAI MẢNH, KHÔNG MỘT: `Trễ 87 ngày` (đỏ) **và** `Hạn 20/6/2026`. Chỉ hiện số
            ngày trễ thì cán bộ không biết hạn là ngày nào để đối chiếu với văn bản giấy; chỉ hiện
            ngày thì con số phải tự nhẩm. Đặc tả đòi cả hai và cả hai đều có việc riêng. */}
        <dt>Hạn xử lý</dt>
        <dd>
          {o.phanTre !== "" ? (
            <>
              <span className="nhan-lech">{nhanHanThe(nhiemVu.due_at, bayGio)}</span> · Hạn{" "}
              {nhanNgay(nhiemVu.due_at)}
            </>
          ) : (
            nhanHanThe(nhiemVu.due_at, bayGio)
          )}
        </dd>

        {/* §5.6 — HAI HẠN CẠNH NHAU, và đó là toàn bộ điểm của hai cột. `HẠN BAN ĐẦU` không đổi
            khi gia hạn; tỷ lệ đúng hạn §11.3 đếm theo nó. */}
        <dt>Hạn ban đầu</dt>
        <dd>{nhanNgay(nhiemVu.original_due_at)}</dd>

        <dt>Đang giao cho</dt>
        <dd>
          {nhiemVu.unit === ""
            ? CHUA_GIAO_BO_PHAN
            : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit)}{" "}
          · {nhiemVu.assignee === "" ? CHUA_PHAN_CONG : nhiemVu.assignee}
        </dd>

        <dt>Mức ưu tiên</dt>
        <dd>
          {nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)} · {nhiemVu.progress}% tiến độ ghi nhận
        </dd>

        <dt>Loại nhiệm vụ</dt>
        <dd>{nhanDanhMuc(danhMuc.loai, nhiemVu.type)}</dd>

        <dt>Khối</dt>
        <dd>{nhanDanhMuc(danhMuc.khoi, nhiemVu.bloc)}</dd>

        <dt>Nguồn giao</dt>
        <dd>
          {nhanNguonGiao(nhiemVu.source)}
          {/* §7.4 — LIÊN KẾT NGƯỢC VỀ BIÊN BẢN GỐC. Chỉ khi máy chủ nối được (`meeting_id`), xem
              `cauTuKetLuan`. Neo `#bien-ban-{id}` mở đúng biên bản ấy dù nó không ở trang đầu. */}
          {cauTuKetLuan(nhiemVu) !== null && nhiemVu.meeting_id !== undefined && (
            <>
              <br />
              <Link href={duongDanBienBan(nhiemVu.meeting_id)}>{cauTuKetLuan(nhiemVu)}</Link>
            </>
          )}
        </dd>

        <dt>Lãnh đạo giao việc</dt>
        <dd>{nhiemVu.assigner === "" ? O_TRONG : nhiemVu.assigner}</dd>

        <dt>Cơ quan chủ trì tham mưu</dt>
        <dd>
          {nhiemVu.lead_unit === ""
            ? O_TRONG
            : (tenBoPhan.get(nhiemVu.lead_unit) ?? nhiemVu.lead_unit)}
        </dd>

        <dt>Chuyên viên theo dõi</dt>
        <dd>{nhiemVu.monitor === "" ? O_TRONG : nhiemVu.monitor}</dd>

        <dt>Mô tả nhiệm vụ</dt>
        <dd>{nhiemVu.description === "" ? O_TRONG : nhiemVu.description}</dd>

        <dt>Tóm tắt kết quả thực hiện</dt>
        <dd>{nhiemVu.result_summary === "" ? O_TRONG : nhiemVu.result_summary}</dd>

        <dt>Ghi chú</dt>
        <dd>{nhiemVu.note === "" ? O_TRONG : nhiemVu.note}</dd>

        <dt>Lãnh đạo xã đã phê duyệt hoàn thành</dt>
        <dd>{nhiemVu.leader_approved ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>

        <dt>Cấp trên đã công nhận hoàn thành</dt>
        <dd>{nhiemVu.superior_acknowledged ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>
      </dl>
      <p className="ghi-chu">{CHU_THICH_HAI_O_TICK}</p>

      {/* §5.4 — CHỈ với loại `Theo văn bản`. Rẽ nhánh trên MÃ, không trên nhãn: xem
          `LOAI_THEO_VAN_BAN`. */}
      {/* `key` theo mã: mở một nhiệm vụ KHÁC khi đang sửa thì form sửa phải biến mất, không được
          hiện ra trên nhiệm vụ mới với chế độ sửa còn bật. */}
      {coKhoiVanBanChiDao(nhiemVu.type) && (
        <KhoiVanBanChiDao
          key={nhiemVu.code}
          tai={vanBan}
          nhiemVu={nhiemVu}
          tenBoPhan={tenBoPhan}
          luu={suaKhoiVanBan}
          docLai={docLaiChiTiet}
        />
      )}

      {loiGhi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiGhi}
        </p>
      )}

      {/* ── ĐỔI TRẠNG THÁI §6 ─────────────────────────────────────────────────────────────
          CHỈ VẼ NHỮNG BƯỚC §6 CÓ. Một nút thừa ở đây không mở được gì — máy chủ kiểm lại bằng
          `ChuyenTrangThaiDuoc` — nhưng một nút thiếu thì cán bộ báo ngay, vì họ đang cần bấm nó.

          BƯỚC `hoan-thanh` VẪN HIỆN KỂ CẢ KHI CÒN VIỆC CON: màn hình không biết có việc con hay
          không (phản hồi không mang số ấy), và câu từ chối của máy chủ LIỆT KÊ MÃ việc con còn
          lại — thông tin cán bộ cần, và là thông tin màn hình không tự dựng được. */}
      <div className="form-danh-muc">
        <h4>Chuyển trạng thái</h4>
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
          {MOI_TRANG_THAI.filter((t) => chuyenSangDuoc(nhiemVu.status, t)).map((t) => (
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
        {MOI_TRANG_THAI.filter((t) => chuyenSangDuoc(nhiemVu.status, t)).length === 0 && (
          <p className="trang-thai-rong">
            Vòng đời §6 không có lối ra khỏi trạng thái này — nhiệm vụ khép lại tại đây.
          </p>
        )}
      </div>

      <KhoiLuiHan
        congDuyet={congDuyet}
        hanHienTai={nhiemVu.due_at}
        dangGui={dangGui}
        guiDeNghi={guiDeNghiLuiHan}
        quyetDinh={quyetDinh}
      />

      {/* ── XOÁ MỀM §11.5 ────────────────────────────────────────────────────────────────── */}
      <form
        className="form-danh-muc"
        onSubmit={(e) => {
          e.preventDefault();
          if (lyDoXoa.trim() !== "") xoa(lyDoXoa.trim());
        }}
      >
        <h4>Xoá nhiệm vụ khỏi sổ</h4>
        <p className="ghi-chu">
          Xoá mềm: dòng ở lại cùng người xoá và lý do, mã sổ đã cấp thì không bao giờ cấp lại. Nhiệm
          vụ còn việc con chưa xoá thì máy chủ từ chối và nói rõ còn mấy việc.
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
        <button type="submit" className="nut-xoa" disabled={dangGui || lyDoXoa.trim() === ""}>
          Xoá nhiệm vụ
        </button>
      </form>
    </div>
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
  tenBoPhan,
  luu,
  docLai,
}: {
  tai: TrangThaiTai<readonly petitions_nhiemVuVanBanRa[]>;
  nhiemVu: petitions_nhiemVuRa;
  tenBoPhan: ReadonlyMap<string, string>;
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
  const hienForm = dangSua && tai.pha === "xong" && lyDoKhoa === null;

  function thoatSua() {
    traTieuDiem.current = true;
    datDangSua(false);
  }

  return (
    <div className="form-danh-muc" aria-labelledby="tieu-de-khoi-van-ban">
      <div className="dau-khoi-chi-tiet">
        <h4 id="tieu-de-khoi-van-ban">{TIEU_DE_KHOI_VAN_BAN}</h4>
        {!hienForm && (
          <button
            ref={nutSua}
            type="button"
            className="nut-phu"
            aria-label={`Sửa ${TIEU_DE_KHOI_VAN_BAN.toLowerCase()}`}
            aria-describedby={lyDoKhoa !== null ? "ly-do-khoa-sua-van-ban" : undefined}
            disabled={lyDoKhoa !== null}
            onClick={() => datDangSua(true)}
          >
            {NHAN_NUT_SUA}
          </button>
        )}
      </div>
      {!hienForm && lyDoKhoa !== null && (
        <p id="ly-do-khoa-sua-van-ban" className="ghi-chu">
          {lyDoKhoa}
        </p>
      )}

      {hienForm && tai.pha === "xong" && (
        <FormSuaKhoiVanBan
          nhiemVu={nhiemVu}
          vanBan={tai.duLieu}
          tenBoPhan={tenBoPhan}
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
 * KHÔNG CÓ KHOÁ LẠC QUAN: hợp đồng không có trường phiên bản hay `etag`. Nếu người khác vừa lưu, lần
 * lưu này thắng ở đúng những trường nó gửi — và chỉ gửi trường đã đổi là để phạm vi ấy nhỏ nhất.
 *
 * Dòng văn bản dùng LẠI `NhomVanBanNhap` của form tạo: cùng ô, cùng giới hạn, cùng cách kiểm (quyết
 * định của người dùng 24/09/2026). Không có thao tác chuyển dòng sang nhóm khác — máy chủ từ chối
 * (`ErrDoiNhomVanBan`); muốn đổi nhóm thì `✕` rồi `+ Thêm văn bản` ở nhóm mới.
 */
export function FormSuaKhoiVanBan({
  nhiemVu,
  vanBan,
  tenBoPhan,
  luu,
  docLai,
  xong,
}: {
  nhiemVu: petitions_nhiemVuRa;
  vanBan: readonly petitions_nhiemVuVanBanRa[];
  tenBoPhan: ReadonlyMap<string, string>;
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
  const demKhoaVanBan = useRef(0);
  // Mở form thì tiêu điểm vào ô đầu tiên sửa được; sau đó là ô vừa thêm / nút thêm của nhóm vừa gỡ.
  const oCanTieuDiem = useRef<string | null>("sua-tieu-de");

  useEffect(() => {
    if (oCanTieuDiem.current === null) return;
    document.getElementById(oCanTieuDiem.current)?.focus();
    oCanTieuDiem.current = null;
  }, [f.vanBan]);

  const than = thanSuaNhiemVu(f, goc.nhiemVu, goc.vanBan);
  const chan = canhBaoSua(f);

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

    luu(than).then((kq) => {
      datDangLuu(false);
      if (!kq.ok) {
        // Ở LẠI chế độ sửa, giữ nguyên chữ đã gõ, in NGUYÊN VĂN câu máy chủ — kể cả câu 409 "gỡ dòng
        // ấy rồi thêm lại ở nhóm mới" hay "dòng không còn thuộc nhiệm vụ này".
        datLoi(kq.thongBao);
        return;
      }
      datLoi(null);
      xong();
    });
  }

  const tenCoQuanChuTri =
    nhiemVu.lead_unit === "" ? O_TRONG : (tenBoPhan.get(nhiemVu.lead_unit) ?? nhiemVu.lead_unit);

  return (
    <form onSubmit={gui} aria-labelledby="tieu-de-sua-khoi-van-ban">
      <h5 id="tieu-de-sua-khoi-van-ban">Sửa {TIEU_DE_KHOI_VAN_BAN.toLowerCase()}</h5>

      {/* HAI Ô CHỈ ĐỌC ĐẦU TIÊN, đúng thứ tự bảng §5.4, mỗi ô kèm lý do không sửa được. */}
      <dl className="danh-sach-truong">
        <dt>Mã nhiệm vụ</dt>
        <dd>
          {nhiemVu.code}
          <span className="dong-phu">{LY_DO_KHONG_SUA_MA}</span>
        </dd>
      </dl>

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

      <dl className="danh-sach-truong">
        <dt>Hạn xử lý</dt>
        <dd>
          {nhanNgay(nhiemVu.due_at)}
          <span className="dong-phu">{LY_DO_KHONG_SUA_HAN}</span>
        </dd>
        <dt>Cơ quan chủ trì tham mưu</dt>
        <dd>{tenCoQuanChuTri}</dd>
        <dt>Chuyên viên Văn phòng tham mưu / theo dõi</dt>
        <dd>{nhiemVu.monitor === "" ? O_TRONG : nhiemVu.monitor}</dd>
      </dl>
      <p className="ghi-chu">{LY_DO_KHONG_SUA_CHU_TRI}</p>

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
 * §5.8 — đề nghị lùi hạn, và quyết định của lãnh đạo giao việc.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HAI NỬA CỦA KHỐI NÀY ĐỨNG SAU HAI KHOÁ KHÁC NHAU, và gộp chúng là làm hỏng đúng điều ADR 0038
 * dựng ra:
 *
 *   gửi đề nghị     `task.update` — việc của NGƯỜI ĐANG LÀM. Ô này luôn hiện.
 *   duyệt / từ chối `task.extend` ở cổng **và** đúng người ghi ở `lanh_dao_giao_viec_ma`.
 *
 * Bỏ lớp thứ hai thì **mọi lãnh đạo cầm khoá duyệt được mọi nhiệm vụ của cả xã**, kể cả của bộ
 * phận họ không liên quan — vì luật 5 kiểm `(tenant_id, role, permission)` và không có chiều "bản
 * ghi nào".
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function KhoiLuiHan({
  congDuyet,
  hanHienTai,
  dangGui,
  guiDeNghi,
  quyetDinh,
}: {
  congDuyet: ReturnType<typeof quyetDinhDuyetLuiHan>;
  hanHienTai: string | null;
  dangGui: boolean;
  guiDeNghi: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  quyetDinh: (
    deNghiID: string,
    duyet: boolean,
    ghiChu?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
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
  }

  return (
    <div className="form-danh-muc">
      <h4>Đề nghị lùi hạn</h4>
      <p className="ghi-chu">{GHI_CHU_LUI_HAN}</p>

      {hanHienTai === null && (
        <p className="trang-thai-rong">
          Nhiệm vụ này không có hạn, nên không có gì để lùi. Hạn chỉ đặt được một lần, lúc tạo việc.
        </p>
      )}

      {hanHienTai !== null && (
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

      {deNghi !== null && (
        <dl className="danh-sach-truong">
          <dt>Đề nghị đang chờ</dt>
          <dd>
            Hạn mới {nhanNgay(deNghi.new_due_at)} · người đề nghị {deNghi.requested_by} · trạng thái{" "}
            {deNghi.status}
          </dd>
        </dl>
      )}

      {/* ── QUYẾT ĐỊNH — LỚP HAI CỦA ADR 0038 ────────────────────────────────────────────
          ẨN MỘT NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP (luật 5, cấm #1): máy chủ kiểm lại cùng
          hai câu hỏi ấy trong giao dịch, trên dòng đọc dưới khoá. Việc của khối này là để cán bộ
          không bấm vào một thứ chắc chắn bị từ chối, và để người KHÔNG phải lãnh đạo giao việc đọc
          được vì sao. */}
      {!congDuyet.hien && congDuyet.vi !== "thieu-quyen" && (
        <p className="trang-thai-rong">{congDuyet.thongBao}</p>
      )}

      {congDuyet.hien &&
        (deNghi === null ? (
          <p className="trang-thai-rong">
            Bạn là lãnh đạo giao việc của nhiệm vụ này, nhưng hợp đồng chưa có tuyến liệt kê đề nghị
            đang chờ — xem phần chưa dựng được ở đầu màn.
          </p>
        ) : (
          <div className="cum-nut">
            <button
              type="button"
              className="nut-chinh"
              disabled={dangGui || dangChay}
              onClick={() => {
                datDangChay(true);
                quyetDinh(deNghi.id, true).then(xong);
              }}
            >
              Duyệt lùi hạn
            </button>
            <button
              type="button"
              className="nut-phu"
              disabled={dangGui || dangChay}
              onClick={() => {
                datDangChay(true);
                quyetDinh(deNghi.id, false).then(xong);
              }}
            >
              Từ chối
            </button>
          </div>
        ))}
    </div>
  );
}

/**
 * Form `Giao việc mới` §7.
 *
 * HAI LOẠI, HAI BỘ TRƯỜNG (§7.2 / §7.3), rẽ nhánh trên MÃ `theo-van-ban` (mã tầng 3, xem
 * `LOAI_THEO_VAN_BAN`). Loại nào khác thì ô tiêu đề thành `Tên nhiệm vụ`, và cơ quan chủ trì /
 * chuyên viên / ba nhóm văn bản BIẾN KHỎI MÀN và KHÔNG LÊN DÂY — phần "không lên dây" ở
 * `thanGiaoViec`, nơi có bài kiểm. Ô `Ghi chú` của §7.2 không có: `taoNhiemVuVao` không nhận `note`.
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
  coDanhSachVanBan = false,
  dangGui,
  loi,
  huy,
  giaoViec,
  maChaCoSan,
  tieuDeCoSan,
}: {
  danhMuc: DanhMucNhiemVu;
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
  /** Id nội bộ của việc cha, khi có. Hôm nay KHÔNG BAO GIỜ có — xem `PHAN_CHUA_DUNG`. */
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
  const [ma, datMa] = useState("");
  const [loai, datLoai] = useState("");
  const [khoi, datKhoi] = useState("");
  const [tieuDe, datTieuDe] = useState(tieuDeCoSan ?? "");
  const [moTa, datMoTa] = useState("");
  const [mucUuTien, datMucUuTien] = useState("");
  const [boPhan, datBoPhan] = useState("");
  const [nguoiThucHien, datNguoiThucHien] = useState("");
  const [lanhDaoGiaoViec, datLanhDaoGiaoViec] = useState("");
  const [coQuanChuTri, datCoQuanChuTri] = useState("");
  const [chuyenVien, datChuyenVien] = useState("");
  const [han, datHan] = useState("");
  const [vanBan, datVanBan] = useState<readonly DongVanBanNhap[]>([]);
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
  const theoVanBan = coKhoiVanBanChiDao(loaiChon);
  const hienVanBan = theoVanBan && coDanhSachVanBan;
  const chanVanBan = hienVanBan ? canhBaoVanBan(vanBan) : null;

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
    if (tieuDe.trim() === "" || loaiChon === "" || chanVanBan !== null) return;

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
        coQuanChuTri,
        chuyenVien,
        han,
        vanBan,
      },
      { coDanhSachVanBan, maCha: maChaCoSan },
    );

    giaoViec(than, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={gui}>
      <h4>Giao việc mới</h4>
      <p className="ghi-chu">{MO_TA_FORM_GIAO_VIEC}</p>

      <div className="o-chon">
        <label htmlFor="giao-loai">Loại nhiệm vụ</label>
        <select id="giao-loai" value={loaiChon} onChange={(e) => datLoai(e.target.value)}>
          {danhMuc.loai.length === 0 && <option value="">{CHUA_XAC_DINH}</option>}
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

      {/* §7.3 BỎ TOÀN BỘ ô này ở loại khác `Theo văn bản`. Ẩn ở đây là phần nhìn; phần không gửi
          nằm ở `thanGiaoViec`. */}
      {theoVanBan && (
        <div className="o-chon">
          <label htmlFor="giao-co-quan-chu-tri">Cơ quan chủ trì tham mưu</label>
          <select
            id="giao-co-quan-chu-tri"
            value={coQuanChuTri}
            onChange={(e) => datCoQuanChuTri(e.target.value)}
          >
            <option value="">{CHUA_XAC_DINH}</option>
            {danhMuc.boPhan.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </div>
      )}

      {/* BA Ô GÕ MÃ CÁN BỘ, KHÔNG PHẢI BA Ô CHỌN — danh bạ đứng sau `admin.user`, xem
          `PHAN_CHUA_DUNG`. Giá trị là MÃ NGHIỆP VỤ `CB-…`, đúng loại định danh ba cột kia giữ:
          một ULID ở đây được máy chủ nhận nhưng không khớp cán bộ nào, và không có gì đỏ ở đâu
          (luật 6, bất biến 8). */}
      <div className="o-nhap">
        <label htmlFor="giao-nguoi-thuc-hien">Người thực hiện (mã cán bộ, bỏ trống để bộ phận tự phân công)</label>
        <input
          id="giao-nguoi-thuc-hien"
          name="giao-nguoi-thuc-hien"
          value={nguoiThucHien}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datNguoiThucHien(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="giao-lanh-dao">Lãnh đạo giao việc (mã cán bộ)</label>
        <input
          id="giao-lanh-dao"
          name="giao-lanh-dao"
          value={lanhDaoGiaoViec}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datLanhDaoGiaoViec(e.target.value)}
        />
      </div>
      {/* KHÔNG CHỈ LÀ NƠI NHẬN THÔNG BÁO: ô này quyết định AI DUYỆT ĐƯỢC ĐỀ NGHỊ LÙI HẠN (ADR
          0038), và bỏ trống nghĩa là KHÔNG AI duyệt được — vĩnh viễn, vì `PATCH` cố ý không sửa
          được cột này. Câu ấy đứng cạnh ô chứ không nằm trong tài liệu. */}
      <p className="ghi-chu">
        {GHI_CHU_LANH_DAO_GIAO_VIEC} Bỏ trống thì không ai duyệt được đề nghị lùi hạn của nhiệm vụ
        này, và ô này không sửa lại được sau khi tạo.
      </p>

      {theoVanBan && (
        <div className="o-nhap">
          <label htmlFor="giao-chuyen-vien">Chuyên viên Văn phòng tham mưu / theo dõi (mã cán bộ)</label>
          <input
            id="giao-chuyen-vien"
            name="giao-chuyen-vien"
            value={chuyenVien}
            placeholder="CB-…"
            autoComplete="off"
            onChange={(e) => datChuyenVien(e.target.value)}
          />
        </div>
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
          <p className="ghi-chu">{GHI_CHU_KHONG_CO_O_GHI_CHU}</p>
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
      <p className="ghi-chu">{CANH_BAO_HAN_MOT_LAN}</p>
      {maChaCoSan !== undefined && maChaCoSan !== "" && (
        <p className="ghi-chu">{GHI_CHU_HAN_VIEC_CON}</p>
      )}

      {/* KHÔNG `role="alert"`: câu này hiện ngay khi bấm `+ Thêm văn bản` (dòng mới còn trống), và
          một vùng thông báo khẩn sẽ cắt ngang đúng lúc tiêu điểm vừa sang ô trích yếu. Nó là lý do
          nút `Giao việc` đang khoá, nên đứng ngay trên nút ấy. */}
      {chanVanBan !== null && <p className="thong-bao-loi">{chanVanBan}</p>}

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
          disabled={dangGui || tieuDe.trim() === "" || loaiChon === "" || chanVanBan !== null}
        >
          Giao việc
        </button>
      </div>
    </form>
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
