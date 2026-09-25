"use client";

import {
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
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
import { coQuyen, QUYEN_KY_BIEN_BAN } from "@/lib/quyen";

import {
  BIEU_MAU_TRONG,
  CANH_BAO_BI_MAT,
  CAU_THONG_BAO_MOT_LAN,
  CAU_XAC_NHAN_KY,
  cauDaTach,
  DANG_TAI_SO,
  DIA_DIEM_TOI_DA,
  dongMeta,
  giaTriTuBienBan,
  idTuNeo,
  KET_LUAN_MOI_LAN_TOI_DA,
  KHONG_GHI_CAN_BO,
  laDaKy,
  lopChipBienBan,
  lopChipKetLuan,
  luaChonCanBo,
  LY_DO_XOA_TOI_DA,
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
  PHAN_CHUA_DUNG,
  PLACEHOLDER_KET_LUAN,
  quyTacBienBan,
  quyTacKetLuan,
  SO_HIEU_TOI_DA,
  SO_RONG,
  SO_THONG_BAO_TOI_DA,
  soThuTuKetLuan,
  TEN_CUOC_HOP_TOI_DA,
  thanSuaTuBieuMau,
  thanTaoTuBieuMau,
  thanThongBao,
  type GiaTriBieuMau,
  type QuyTacNut,
} from "./nhan-bien-ban";

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
 * (luật 5, cấm #1). Xem `PHAN_CHUA_DUNG`.
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
  readonly dangGui: boolean;
  /** Id của kết luận đang mở hộp, hoặc `null`. */
  readonly moOKetLuan: string | null;
  readonly loi: { readonly ketLuanID: string; readonly thongBao: string } | null;
  readonly daXong: { readonly ketLuanID: string; readonly maNhiemVu: string } | null;
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

  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của hàng thêm kết luận và của biểu mẫu, nên một lần
  // ghi xong là một khoá chống trùng mới. Lần ghi HỎNG không tăng — giữ chữ đã gõ VÀ khoá cũ.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // §3 — luồng Tách. Tất cả mang ID KẾT LUẬN.
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);
  const [moTachO, datMoTachO] = useState<string | null>(null);
  const [loiTach, datLoiTach] = useState<{ ketLuanID: string; thongBao: string } | null>(null);
  const [daTach, datDaTach] = useState<{ ketLuanID: string; maNhiemVu: string } | null>(null);

  // Vòng đời.
  const [kqDanhBa, datKqDanhBa] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
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

  const vungBieuMau = useRef<HTMLDivElement>(null);

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
    layTrangThaiNhiemVu().then((kq) => {
      if (!bo) datKqNhanTT(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  // Biểu mẫu mở ở đầu màn; bấm "Sửa" ở thẻ thứ chín thì đưa cán bộ lên chỗ nó. Không đặt state.
  useEffect(() => {
    if (bieuMau !== null) vungBieuMau.current?.scrollIntoView({ block: "start" });
  }, [bieuMau]);

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
    themKetLuan(bienBanID, { content: noiDung }, khoaChongTrung).then((kq) => {
      datDangGui(false);
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
      datDaTach({ ketLuanID: ketLuan.id, maNhiemVu: kq.duLieu.code });
      datMoTachO(null);
      datLanGhiXong((n) => n + 1);
      docLai();
      if (nhiemVuKL.has(ketLuan.id)) taiNhiemVuKL(bienBanID, ketLuan);
    });
  }

  const phepTach: PhepTach = {
    danhMuc,
    dangGui,
    moOKetLuan: moTachO,
    loi: loiTach,
    daXong: daTach,
    mo: (ketLuanID) => {
      datMoTachO((dang) => (dang === ketLuanID ? null : ketLuanID));
      datLoiTach(null);
      datDaTach(null);
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
      xongBieuMau();
      docLai();
    });
  }

  const chiTiet = vongDoi.chiTiet;
  const chiTietNgoaiTrang =
    chiTiet !== null && so.pha === "xong" && !so.duLieu.items.some((bb) => bb.id === chiTiet.id);
  const moNhapMoi = bieuMau?.loai === "tao" && bieuMau.boSungCho === null;

  return (
    <section className="man-bien-ban" aria-labelledby="tieu-de-so-bien-ban">
      <h2 id="tieu-de-so-bien-ban">Danh sách biên bản</h2>

      <KhoiChuaDung />

      <div className="cum-nut">
        <button
          type="button"
          className="nut-chinh"
          aria-expanded={moNhapMoi}
          onClick={() => {
            datBieuMau(moNhapMoi ? null : { loai: "tao", boSungCho: null });
            datLoiBieuMau(null);
          }}
        >
          {NHAN_NUT_NHAP_BIEN_BAN}
        </button>
      </div>

      <div ref={vungBieuMau}>
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
      </div>

      {chiTietNgoaiTrang && chiTiet !== null && (
        // Biên bản mở từ một đường dẫn (drawer nhiệm vụ, liên kết bổ sung) mà không nằm ở trang
        // đang xem: vẫn mở được, ngay đầu danh sách.
        <ChiTietBienBan tai={chiTiet.tai} danhBa={bangDanhBa} dong={dongChiTiet} />
      )}

      {so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
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
          />
          <nav className="dieu-huong-trang" aria-label="Phân trang danh sách biên bản">
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
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
            </button>
          </nav>
        </>
      )}
    </section>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã.
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

/** Danh sách THẺ §2 — xếp dọc, ĐÚNG THỨ TỰ MÁY CHỦ TRẢ (ngày họp mới nhất ở trên). */
export function DanhSachBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
  vongDoi,
}: {
  bienBan: readonly petitions_bienBanRa[];
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: { bienBanID: string; thongBao: string } | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
  vongDoi: PhepVongDoi;
}) {
  if (bienBan.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    <ul aria-label="Danh sách biên bản họp">
      {bienBan.map((bb) => (
        // `id` là neo `#bien-ban-{id}` — liên kết từ drawer nhiệm vụ cuộn tới đúng thẻ này.
        <li key={bb.id} id={neoBienBan(bb.id)}>
          <TheBienBan
            bienBan={bb}
            lanGhiXong={lanGhiXong}
            dangGui={dangGui}
            loiKetLuan={loiKetLuan?.bienBanID === bb.id ? loiKetLuan.thongBao : null}
            guiKetLuan={guiKetLuan}
            tach={tach}
            vongDoi={vongDoi}
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
}: {
  quyTac: QuyTacNut;
  id: string;
  nhan: string;
  ariaLabel?: string;
  dangGui: boolean;
  onClick: () => void;
  moRa?: boolean;
}) {
  if (!quyTac.hien) return null;
  return (
    <button
      type="button"
      className="nut-phu"
      aria-label={ariaLabel}
      aria-expanded={moRa}
      aria-describedby={quyTac.viSaoTat === null ? undefined : `ly-do-${id}`}
      disabled={dangGui || quyTac.viSaoTat !== null}
      onClick={onClick}
    >
      {nhan}
    </button>
  );
}

/** Câu lý do của một nút đang tắt. */
function LyDoTat({ quyTac, id }: { quyTac: QuyTacNut; id: string }) {
  if (!quyTac.hien || quyTac.viSaoTat === null) return null;
  return (
    <p className="ghi-chu" id={`ly-do-${id}`}>
      {quyTac.viSaoTat}
    </p>
  );
}

/** Một thẻ biên bản §2: header · con số · hành động · các dòng kết luận · hàng thêm kết luận. */
export function TheBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
  vongDoi,
}: {
  bienBan: petitions_bienBanRa;
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: string | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
  vongDoi: PhepVongDoi;
}) {
  const qt = quyTacBienBan(bienBan, vongDoi.coQuyenKy);
  const loi = vongDoi.loi?.dich === bienBan.id ? vongDoi.loi.thongBao : null;
  const hop = vongDoi.hop?.dich === bienBan.id ? vongDoi.hop.loai : null;
  const chiTiet = vongDoi.chiTiet?.id === bienBan.id ? vongDoi.chiTiet : null;

  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <span aria-hidden="true">📋</span>
        <h3>{bienBan.title}</h3>
        <span className={lopChipBienBan(bienBan)}>{nhanTrangThaiBienBan(bienBan)}</span>
      </div>

      <p className="dong-phu">{dongMeta(bienBan)}</p>

      {/* CON SỐ CHÍNH là kết luận hoàn thành; số nhiệm vụ là phụ. Cả bốn số do máy chủ đếm. */}
      <p>
        <strong>{nhanTienDoBienBan(bienBan)}</strong>{" "}
        <span className="dong-phu">· {nhanBadge(bienBan)}</span>
      </p>

      {bienBan.supplements_id !== undefined && bienBan.supplements_id !== "" && (
        <p className="ghi-chu">
          Biên bản bổ sung — <a href={`#${neoBienBan(bienBan.supplements_id)}`}>xem biên bản gốc</a>
        </p>
      )}

      <div className="cum-nut">
        {chiTiet !== null ? (
          <button type="button" className="nut-phu" onClick={vongDoi.dongChiTiet}>
            {NHAN_NUT_DONG_BIEN_BAN}
          </button>
        ) : (
          <a className="nut-phu" href={`#${neoBienBan(bienBan.id)}`}>
            {NHAN_NUT_XEM_BIEN_BAN}
          </a>
        )}
        <NutCoLyDo
          quyTac={qt.sua}
          id={`sua-${bienBan.id}`}
          nhan={NHAN_NUT_SUA_BIEN_BAN}
          dangGui={vongDoi.dangGui}
          onClick={() => vongDoi.moSua(bienBan)}
        />
        <NutCoLyDo
          quyTac={qt.ky}
          id={`ky-${bienBan.id}`}
          nhan={NHAN_NUT_KY}
          dangGui={vongDoi.dangGui}
          moRa={hop === "ky"}
          onClick={() => vongDoi.moHop(bienBan.id, "ky")}
        />
        <NutCoLyDo
          quyTac={qt.ghiThongBao}
          id={`thong-bao-${bienBan.id}`}
          nhan={NHAN_NUT_GHI_THONG_BAO}
          dangGui={vongDoi.dangGui}
          moRa={hop === "thong-bao"}
          onClick={() => vongDoi.moHop(bienBan.id, "thong-bao")}
        />
        <NutCoLyDo
          quyTac={qt.boSung}
          id={`bo-sung-${bienBan.id}`}
          nhan={NHAN_NUT_BO_SUNG}
          dangGui={vongDoi.dangGui}
          onClick={() => vongDoi.moBoSung(bienBan)}
        />
        <NutCoLyDo
          quyTac={qt.xoa}
          id={`xoa-${bienBan.id}`}
          nhan={NHAN_NUT_XOA_BIEN_BAN}
          dangGui={vongDoi.dangGui}
          moRa={hop === "xoa"}
          onClick={() => vongDoi.moHop(bienBan.id, "xoa")}
        />
      </div>
      <LyDoTat quyTac={qt.xoa} id={`xoa-${bienBan.id}`} />

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      {hop === "ky" && (
        <FormKy
          bienBanID={bienBan.id}
          dangGui={vongDoi.dangGui}
          huy={vongDoi.dongHop}
          ky={(tb) => vongDoi.ky(bienBan, tb)}
        />
      )}
      {hop === "thong-bao" && (
        <FormThongBao
          bienBanID={bienBan.id}
          dangGui={vongDoi.dangGui}
          huy={vongDoi.dongHop}
          ghi={(tb) => vongDoi.ghiThongBao(bienBan, tb)}
        />
      )}
      {hop === "xoa" && (
        <FormLyDo
          id={`ly-do-xoa-bien-ban-${bienBan.id}`}
          nhan="Lý do gỡ biên bản *"
          nhanNut={NHAN_NUT_XOA_BIEN_BAN}
          ghiChu="Biên bản là hồ sơ lưu trữ: gỡ là xoá mềm, ghi lại ai gỡ, lúc nào và vì sao."
          dangGui={vongDoi.dangGui}
          huy={vongDoi.dongHop}
          gui={(lyDo) => vongDoi.xoa(bienBan, lyDo)}
        />
      )}

      {chiTiet !== null && (
        <ChiTietBienBan tai={chiTiet.tai} danhBa={vongDoi.danhBa} dong={vongDoi.dongChiTiet} />
      )}

      {bienBan.conclusions.length === 0 ? (
        // §7.3: biên bản không có kết luận nào VẪN LƯU ĐƯỢC (nhập nháp trước, bổ sung sau).
        <p className="trang-thai-rong">Biên bản này chưa ghi kết luận nào.</p>
      ) : (
        <ol aria-label={`Các kết luận của biên bản ${bienBan.title}`}>
          {/* ⚠ KHÔNG LẤY CHỈ SỐ CỦA `map` RA DÙNG. Số trong ô tròn và `{stt}` trên đường dẫn đều
              là `ordinal` MÁY CHỦ TRẢ — xem `DongKetLuan` và `soThuTuKetLuan`. */}
          {bienBan.conclusions.map((kl) => (
            <li key={kl.id}>
              <DongKetLuan bienBan={bienBan} ketLuan={kl} tach={tach} vongDoi={vongDoi} />
            </li>
          ))}
        </ol>
      )}

      {loiKetLuan !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiKetLuan}
        </p>
      )}

      {/* Biên bản ĐÃ KÝ không nhận kết luận mới — sai sót đi đường biên bản bổ sung. */}
      {qt.themKetLuan.hien && (
        <HangThemKetLuan
          key={`${bienBan.id}|${lanGhiXong}`}
          bienBanID={bienBan.id}
          dangGui={dangGui}
          gui={guiKetLuan}
        />
      )}
    </div>
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
    <section className="khoi-chi-tiet" aria-label="Toàn văn biên bản">
      {tai.pha === "dangTai" && <p role="status">Đang tải biên bản…</p>}
      {tai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {tai.thongBao}
        </p>
      )}
      {tai.pha === "xong" && <NoiDungChiTiet bb={tai.duLieu} danhBa={danhBa} />}
      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={dong}>
          {NHAN_NUT_DONG_BIEN_BAN}
        </button>
      </div>
    </section>
  );
}

function NoiDungChiTiet({ bb, danhBa }: { bb: petitions_bienBanRa; danhBa: DanhBaTheoMa | null }) {
  const thanhPhan = bb.attendees ?? [];
  const boSungBoi = bb.supplemented_by ?? [];
  return (
    <>
      <h4>{bb.title}</h4>
      <dl className="danh-sach-truong">
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
            <ul>
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
              <ul>
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
      <h4>Nội dung biên bản</h4>
      {bb.content === undefined || bb.content === null || bb.content === "" ? (
        <p className="nhan-trong">Không ghi toàn văn.</p>
      ) : (
        <DoanNhieuDong chu={bb.content} />
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
    <>
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
    </>
  );
}

/** Hộp Ký — câu xác nhận nói đúng hệ quả, và hai ô Thông báo tuỳ chọn. */
export function FormKy({
  bienBanID,
  dangGui,
  huy,
  ky,
}: {
  bienBanID: string;
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
    <form className="form-danh-muc" onSubmit={gui} aria-label={NHAN_NUT_KY}>
      <p role="note">{CAU_XAC_NHAN_KY}</p>
      <OThongBao bienBanID={bienBanID} so={so} ngay={ngay} datSo={datSo} datNgay={datNgay} />
      <p className="ghi-chu">Thông báo kết luận không bắt buộc lúc ký; ghi sau được một lần.</p>
      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {NHAN_NUT_XAC_NHAN_KY}
        </button>
      </div>
    </form>
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
    <form className="form-danh-muc" onSubmit={gui} aria-label={NHAN_NUT_GHI_THONG_BAO}>
      <OThongBao bienBanID={bienBanID} so={so} ngay={ngay} datSo={datSo} datNgay={datNgay} />
      <p className="ghi-chu">{CAU_THONG_BAO_MOT_LAN}</p>
      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || tb === null}>
          {NHAN_NUT_GHI_THONG_BAO}
        </button>
      </div>
    </form>
  );
}

/** Ô lý do BẮT BUỘC của một lần gỡ (biên bản hay kết luận) — xoá mềm phải ghi vì sao. */
export function FormLyDo({
  id,
  nhan,
  nhanNut,
  ghiChu,
  dangGui,
  huy,
  gui,
}: {
  id: string;
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

  return (
    <form className="form-danh-muc" onSubmit={guiNgay}>
      <div className="o-nhap">
        <label htmlFor={id}>{nhan}</label>
        <textarea
          id={id}
          rows={2}
          value={lyDo}
          maxLength={LY_DO_XOA_TOI_DA}
          onChange={(e) => datLyDo(e.target.value)}
        />
        <p className="ghi-chu">{ghiChu}</p>
      </div>
      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-xoa" disabled={dangGui || gon === ""}>
          {nhanNut}
        </button>
      </div>
    </form>
  );
}

/** Sửa LỜI một kết luận của bản nháp. */
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

  function gui(e: FormEvent) {
    e.preventDefault();
    if (doiDuoc) luu(gon);
  }

  return (
    <form className="form-danh-muc" onSubmit={gui}>
      <div className="o-nhap">
        <label htmlFor={`sua-ket-luan-${ketLuan.id}`}>
          Nội dung kết luận số {soThuTuKetLuan(ketLuan)}
        </label>
        <textarea
          id={`sua-ket-luan-${ketLuan.id}`}
          rows={3}
          value={noiDung}
          maxLength={NOI_DUNG_KET_LUAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
      </div>
      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || !doiDuoc}>
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
  if (tai.pha === "dangTai") return <p role="status">Đang tải nhiệm vụ đã tách…</p>;
  if (tai.pha === "loi") {
    return (
      <p className="thong-bao-loi" role="alert">
        {tai.thongBao}
      </p>
    );
  }
  if (tai.duLieu.length === 0) {
    return <p className="ghi-chu">Không còn nhiệm vụ nào đang hiệu lực tách từ kết luận này.</p>;
  }
  return (
    <ul aria-label="Nhiệm vụ đã tách từ kết luận">
      {tai.duLieu.map((nv) => (
        <li key={nv.code}>
          <span className="ma-muc">{nv.code}</span> {nv.title}
          <span className="dong-phu">
            {" "}
            · {nhanTrangThai(nhanTT, nv.status)} · Hạn {nhanNgay(nv.due_at)}
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
  const daXong = tach.daXong?.ketLuanID === ketLuan.id ? tach.daXong.maNhiemVu : null;
  const loi = vongDoi.loi?.dich === ketLuan.id ? vongDoi.loi.thongBao : null;
  const hop = vongDoi.hop?.dich === ketLuan.id ? vongDoi.hop.loai : null;
  const nhiemVu = vongDoi.nhiemVuKL.get(ketLuan.id);

  return (
    <div>
      <span className="chip">{so}</span> <span>{ketLuan.content}</span>{" "}
      <span className={lopChipKetLuan(ketLuan)}>{nhanTrangThaiKetLuan(ketLuan)}</span>
      <p className="dong-phu">{nhanTienDoKetLuan(ketLuan)}</p>

      {ketLuan.task_count > 0 && (
        <button
          type="button"
          className="nut-phu"
          aria-expanded={nhiemVu !== undefined}
          aria-label={`${nhanNutXemNhiemVu(ketLuan, nhiemVu !== undefined)} — kết luận số ${so}`}
          onClick={() => vongDoi.batNhiemVuKL(bienBan, ketLuan)}
        >
          {nhanNutXemNhiemVu(ketLuan, nhiemVu !== undefined)}
        </button>
      )}
      {nhiemVu !== undefined && <DanhSachNhiemVuKetLuan tai={nhiemVu} nhanTT={vongDoi.nhanTT} />}

      <div className="cum-nut">
        {/* KHÔNG CÓ CỔNG QUYỀN Ở ĐÂY: tài khoản thiếu `task.create` nhận nguyên câu 403 của máy chủ
            ngay dưới dòng. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1). */}
        {qt.tach.hien && (
          <button
            type="button"
            className="nut-phu"
            aria-label={nhanNutTach(ketLuan)}
            aria-expanded={dangMo}
            disabled={tach.dangGui}
            onClick={() => tach.mo(ketLuan.id)}
          >
            {NHAN_NUT_TACH}
          </button>
        )}
        <NutCoLyDo
          quyTac={qt.sua}
          id={`sua-kl-${ketLuan.id}`}
          nhan={NHAN_NUT_SUA_KL}
          ariaLabel={`Sửa kết luận số ${so}`}
          dangGui={vongDoi.dangGui}
          moRa={hop === "sua-kl"}
          onClick={() => vongDoi.moHop(ketLuan.id, "sua-kl")}
        />
        <NutCoLyDo
          quyTac={qt.go}
          id={`go-kl-${ketLuan.id}`}
          nhan={NHAN_NUT_GO_KL}
          ariaLabel={`Gỡ kết luận số ${so}`}
          dangGui={vongDoi.dangGui}
          moRa={hop === "go-kl"}
          onClick={() => vongDoi.moHop(ketLuan.id, "go-kl")}
        />
        <NutCoLyDo
          quyTac={qt.danhDau}
          id={`danh-dau-${ketLuan.id}`}
          nhan={NHAN_NUT_DANH_DAU}
          ariaLabel={`${NHAN_NUT_DANH_DAU} — kết luận số ${so}`}
          dangGui={vongDoi.dangGui}
          onClick={() => vongDoi.danhDau(bienBan, ketLuan)}
        />
        <NutCoLyDo
          quyTac={qt.boDau}
          id={`bo-dau-${ketLuan.id}`}
          nhan={NHAN_NUT_BO_DAU}
          ariaLabel={`${NHAN_NUT_BO_DAU} — kết luận số ${so}`}
          dangGui={vongDoi.dangGui}
          onClick={() => vongDoi.boDau(bienBan, ketLuan)}
        />
      </div>
      {/* Sửa và Gỡ tắt vì CÙNG một lý do — hiện MỘT câu; nút Gỡ trỏ `aria-describedby` vào câu
          của chính nó nên câu ấy cũng phải có mặt, chỉ không lặp lại cho mắt nhìn. */}
      <LyDoTat quyTac={qt.sua} id={`sua-kl-${ketLuan.id}`} />
      {qt.go.viSaoTat !== null && (
        <p className="an-thi-giac" id={`ly-do-go-kl-${ketLuan.id}`}>
          {qt.go.viSaoTat}
        </p>
      )}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      {hop === "sua-kl" && (
        <FormSuaKetLuan
          ketLuan={ketLuan}
          dangGui={vongDoi.dangGui}
          huy={vongDoi.dongHop}
          luu={(noiDung) => vongDoi.suaKL(bienBan, ketLuan, noiDung)}
        />
      )}
      {hop === "go-kl" && (
        <FormLyDo
          id={`o-ly-do-go-kl-${ketLuan.id}`}
          nhan={`Lý do gỡ kết luận số ${so} *`}
          nhanNut={`Gỡ kết luận số ${so}`}
          ghiChu="Gỡ là xoá mềm. Số thứ tự đã cấp không cấp lại cho kết luận thêm sau."
          dangGui={vongDoi.dangGui}
          huy={vongDoi.dongHop}
          gui={(lyDo) => vongDoi.goKL(bienBan, ketLuan, lyDo)}
        />
      )}

      {daXong !== null && (
        <p className="ghi-chu" role="status">
          {cauDaTach(daXong)}
        </p>
      )}

      {dangMo && qt.tach.hien && (
        <div>
          {/* KẾT LUẬN GỐC ĐỨNG NGAY TRÊN BIỂU MẪU và Ở LẠI kể cả khi ô đã tự điền: ô kia là thứ
              cán bộ SẼ SỬA, nên câu gốc phải còn để đối chiếu. */}
          <p>
            <span className="chip">{so}</span> {ketLuan.content}
          </p>
          <p className="ghi-chu">{NGUON_GIAO_KHOA}</p>
          <FormGiaoViec
            danhMuc={tach.danhMuc}
            dangGui={tach.dangGui}
            loi={loiTach}
            huy={tach.dong}
            giaoViec={(than, khoaChongTrung) =>
              tach.gui(bienBan.id, ketLuan, than, khoaChongTrung)
            }
            tieuDeCoSan={ketLuan.content}
          />
        </div>
      )}
    </div>
  );
}

/** Hàng thêm kết luận §2, ở cuối mỗi thẻ DỰ THẢO. `POST /api/v1/meetings/{id}/conclusions`. */
export function HangThemKetLuan({
  bienBanID,
  dangGui,
  gui,
}: {
  bienBanID: string;
  dangGui: boolean;
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

  return (
    <form className="form-danh-muc" onSubmit={guiNgay}>
      <div className="o-nhap">
        <label htmlFor={`them-ket-luan-${bienBanID}`}>Thêm một kết luận</label>
        <textarea
          id={`them-ket-luan-${bienBanID}`}
          name="noi-dung-ket-luan"
          rows={1}
          value={noiDung}
          placeholder={PLACEHOLDER_KET_LUAN}
          maxLength={NOI_DUNG_KET_LUAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
      </div>
      <button type="submit" className="nut-phu" disabled={dangGui || canGon === ""}>
        {NHAN_NUT_THEM_KET_LUAN}
      </button>
    </form>
  );
}

/**
 * Ô chọn một cán bộ (Chủ trì, Thư ký) từ danh bạ chọn người. GỬI MÃ (`code`), HIỆN `Họ tên · Chức
 * vụ`. Giá trị đang lưu mà không còn trong danh bạ vẫn có một dòng — xem `luaChonCanBo`.
 */
function OChonCanBo({
  id,
  nhan,
  giaTri,
  danhBa,
  dat,
}: {
  id: string;
  nhan: string;
  giaTri: string;
  danhBa: readonly identity_canBoChonNguoiRa[];
  dat: (ma: string) => void;
}) {
  return (
    <div className="o-nhap">
      <label htmlFor={id}>{nhan}</label>
      <select id={id} className="o-chon" value={giaTri} onChange={(e) => dat(e.target.value)}>
        <option value="">{KHONG_GHI_CAN_BO}</option>
        {luaChonCanBo(danhBa, giaTri).map((lc) => (
          <option key={lc.ma} value={lc.ma}>
            {lc.nhan}
          </option>
        ))}
      </select>
    </div>
  );
}

/**
 * Biểu mẫu biên bản §4 — NHẬP MỚI, NHẬP BỔ SUNG cho một biên bản đã ký, hoặc SỬA một bản nháp.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL; ở đây nó là một khối trong trang (xem `PHAN_CHUA_DUNG`, mục CSS).
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

  const doi = (moi: Partial<GiaTriBieuMau>) => datGt((cu) => ({ ...cu, ...moi }));

  // HAI TRƯỜNG BẮT BUỘC, ĐÚNG HAI: "Tên cuộc họp" và "Ngày họp" (§4).
  const duTruongBatBuoc = gt.ten.trim() !== "" && gt.ngay !== "";
  const thanSua = che.loai === "sua" ? thanSuaTuBieuMau(gt, che.ban) : null;
  // Bản sửa không đổi gì thì không có gì để lưu — một PATCH rỗng không ghi được gì.
  const duDieuKien = duTruongBatBuoc && (che.loai === "tao" || thanSua !== null);

  const tieuDe =
    che.loai === "sua"
      ? "Sửa biên bản (dự thảo)"
      : che.boSungCho !== null
        ? "Lập biên bản bổ sung"
        : "Nhập biên bản";

  function luuNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    if (che.loai === "sua") {
      if (thanSua !== null) sua(che.ban.id, thanSua);
      return;
    }
    luu(thanTaoTuBieuMau(gt, che.boSungCho?.id ?? null), khoaChongTrung);
  }

  const chuaChon = danhBa.filter((cb) => !gt.thanhPhanCanBo.includes(cb.code));

  return (
    <form className="form-danh-muc" onSubmit={luuNgay} aria-labelledby="tieu-de-nhap-bien-ban">
      <h3 id="tieu-de-nhap-bien-ban">{tieuDe}</h3>

      {/* ĐÃ QUYẾT 25/09/2026: nội dung mật không bao giờ vào ViGov — không có cờ "mật" nào. */}
      <p className="canh-bao-pham-vi" role="note">
        {CANH_BAO_BI_MAT}
      </p>

      {che.loai === "tao" && che.boSungCho !== null && (
        <p className="ghi-chu">
          Bổ sung cho biên bản đã ký: <strong>{che.boSungCho.title}</strong> ·{" "}
          {dongMeta(che.boSungCho)}
        </p>
      )}

      <div className="o-nhap">
        <label htmlFor="ten-cuoc-hop">Tên cuộc họp *</label>
        <input
          id="ten-cuoc-hop"
          name="ten-cuoc-hop"
          value={gt.ten}
          maxLength={TEN_CUOC_HOP_TOI_DA}
          autoComplete="off"
          onChange={(e) => doi({ ten: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="ngay-hop">Ngày họp *</label>
        {/* `type="date"` trả đúng `2026-08-05` — ngày lịch, không múi giờ. */}
        <input
          id="ngay-hop"
          name="ngay-hop"
          type="date"
          value={gt.ngay}
          onChange={(e) => doi({ ngay: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="so-hieu-bien-ban">Số hiệu biên bản</label>
        <input
          id="so-hieu-bien-ban"
          name="so-hieu-bien-ban"
          value={gt.soHieu}
          maxLength={SO_HIEU_TOI_DA}
          autoComplete="off"
          onChange={(e) => doi({ soHieu: e.target.value })}
        />
        <p className="ghi-chu">
          Ví dụ 31/BB-UBND. Không bắt buộc, và không phải mã tra cứu — số hiệu lặp lại giữa các
          năm.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="dia-diem-hop">Địa điểm</label>
        <input
          id="dia-diem-hop"
          name="dia-diem-hop"
          value={gt.diaDiem}
          maxLength={DIA_DIEM_TOI_DA}
          autoComplete="off"
          onChange={(e) => doi({ diaDiem: e.target.value })}
        />
      </div>

      <OChonCanBo
        id="chu-tri-bien-ban"
        nhan="Chủ trì"
        giaTri={gt.chuTri}
        danhBa={danhBa}
        dat={(ma) => doi({ chuTri: ma })}
      />
      <OChonCanBo
        id="thu-ky-bien-ban"
        nhan="Thư ký"
        giaTri={gt.thuKy}
        danhBa={danhBa}
        dat={(ma) => doi({ thuKy: ma })}
      />
      {loiDanhBa !== null && (
        <p className="thong-bao-loi" role="alert">
          Chưa đọc được danh bạ cán bộ: {loiDanhBa}
        </p>
      )}

      <fieldset className="o-nhap">
        <legend>Thành phần tham dự</legend>
        {gt.thanhPhanCanBo.length > 0 && (
          <ul aria-label="Cán bộ tham dự đã chọn">
            {gt.thanhPhanCanBo.map((ma) => (
              <li key={ma}>
                {nhanThanhPhan(ma, bangDanhBa)}{" "}
                <button
                  type="button"
                  className="nut-phu"
                  aria-label={`Bỏ ${nhanThanhPhan(ma, bangDanhBa)} khỏi thành phần tham dự`}
                  onClick={() =>
                    doi({ thanhPhanCanBo: gt.thanhPhanCanBo.filter((m) => m !== ma) })
                  }
                >
                  Bỏ
                </button>
              </li>
            ))}
          </ul>
        )}
        <label htmlFor="chon-thanh-phan">Thêm cán bộ tham dự</label>
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
        <label htmlFor="thanh-phan-khac">Thành phần khác</label>
        <textarea
          id="thanh-phan-khac"
          name="thanh-phan-khac"
          rows={3}
          value={gt.thanhPhanKhac}
          // KHÔNG CÓ `maxLength`: trần máy chủ là 200 dòng VÀ 200 ký tự mỗi dòng — một con số trên
          // cả ô sẽ cắt sai ở cả hai chiều. Máy chủ từ chối bằng câu nói đúng dòng nào sai.
          onChange={(e) => doi({ thanhPhanKhac: e.target.value })}
        />
        <p className="ghi-chu">
          Mỗi dòng một người — khách mời, đại diện thôn, người không có tài khoản trên hệ thống.
        </p>
      </fieldset>

      <div className="o-nhap">
        <label htmlFor="noi-dung-bien-ban">Nội dung biên bản</label>
        <textarea
          id="noi-dung-bien-ban"
          name="noi-dung-bien-ban"
          rows={6}
          value={gt.noiDung}
          maxLength={NOI_DUNG_BIEN_BAN_TOI_DA}
          onChange={(e) => doi({ noiDung: e.target.value })}
        />
        <p className="ghi-chu">Toàn văn. Đọc lại bằng nút “{NHAN_NUT_XEM_BIEN_BAN}” trên thẻ.</p>
      </div>

      {che.loai === "tao" && (
        <div className="o-nhap">
          <label htmlFor="cac-ket-luan">Các kết luận</label>
          <textarea
            id="cac-ket-luan"
            name="cac-ket-luan"
            rows={4}
            value={gt.ketLuan}
            onChange={(e) => doi({ ketLuan: e.target.value })}
          />
          <p className="ghi-chu">
            Mỗi dòng một kết luận, đánh số ① ② ③ theo thứ tự nhập. Tối đa{" "}
            {KET_LUAN_MOI_LAN_TOI_DA} kết luận một lần nhập; thêm tiếp bằng nút “
            {NHAN_NUT_THEM_KET_LUAN}” ở cuối thẻ. Để trống cũng lưu được.
          </p>
        </div>
      )}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || !duDieuKien}>
          {che.loai === "sua" ? NHAN_NUT_LUU_SUA : NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}
