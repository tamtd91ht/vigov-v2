"use client";

import {
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  CloudOff,
  Forward,
  Inbox,
  Pencil,
  Plus,
  RefreshCw,
  SearchX,
  Settings,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";

import { ChonNam } from "@/components/chon-nam";
import { ActionMenu, type ActionMenuItem } from "@/components/ui/action-menu";
import { Button } from "@/components/ui/button";
import { Card, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { FilterBar } from "@/components/ui/filter-bar";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import {
  bangTraTuKetQua,
  traTen,
  type BangTraDanhMuc,
} from "@/features/cau-hinh/tra-danh-muc";
import {
  TRANG_DAU,
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import { layLoaiVanBan } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_vanBanDenRa,
  page_Result_documents_vanBanDenRa,
} from "@/lib/api/schema.gen";
import {
  layLichSuChuyenVanBanDen,
  laySoVanBanDen,
  layVanBanDen,
  suaVanBanDen,
  vaoSoVanBanDen,
  type LocVanBanDen,
} from "@/lib/api/van-ban";
import { DrillDownBanner } from "@/components/drill-down-banner";
import { NO_DRILL_DOWN, drillDownQuery, type DrillDown } from "@/lib/drill-down";
import { namTheoDongHoMay } from "@/lib/nam";
import { QUYEN_CHUYEN_VAN_BAN, QUYEN_GHI_SO_VAN_BAN, quyetDinhTheoKhoa } from "@/lib/quyen";

import {
  BAN_CHUYEN_TRONG,
  CANH_BAO_GO_KHONG_TRA_SO,
  DAN_DUONG_TOI_CAU_HINH,
  DAN_HAN_DO_MAY_CHU_AN_DINH,
  DAN_SO_DEN,
  GIAI_THICH_LY_DO_GO,
  LOI_THIEU_LY_DO_GO,
  MA_DO_KHAN,
  MA_TRANG_THAI,
  NHAN_DUONG_TOI_CAU_HINH,
  NUT_CHUYEN,
  NUT_GO,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  NUT_VAO_SO,
  NUT_XAC_NHAN_GO,
  NUT_XEM,
  O_CO_QUAN_BAN_HANH,
  O_DO_KHAN,
  O_LOAI_VAN_BAN,
  O_LY_DO_GO,
  O_NGAY_DEN,
  O_NGAY_VAN_BAN,
  O_SO_KY_HIEU,
  O_TRICH_YEU,
  SO_DEN_RONG,
  nhanBoPhanDangGiu,
  nhanDoKhan,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanSoVaoSo,
  nhanTrangThai,
  trangThaiHanVanBan,
  type BanChuyen,
} from "./nhan-van-ban";
import {
  ChonThuTu,
  GOI_Y_TIM_DEN,
  OTimVanBan,
  doiLocVeTrangDau,
  sapXepTheoThuTu,
  type MaThuTu,
} from "./loc-so-van-ban";
import {
  DeadlineMark,
  DocumentStatusBadge,
  Glyph,
  REGISTER_TABLE_SCROLLER,
  RegisterRowsSkeleton,
} from "./document-ui";
import { NganVanBanDen } from "./ngan-van-ban-den";
import { guiChuyenVanBan, guiGoVanBanDen } from "./thao-tac-van-ban";

/**
 * Sổ văn bản ĐẾN — `docs/ui-ux/05-van-ban-don-thu.md §3`, năm tuyến của `service-documents`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VIỆC SỐ MỘT CỦA MÀN NÀY LÀ NHẬP NHANH. Một xã nhận vài chục văn bản mỗi tuần và một cán bộ văn
 * phòng ngồi gõ, tờ giấy trên tay. Vì vậy: ô nhập xếp đúng thứ tự người ta đọc trên tờ văn bản
 * (ngày đến → số ký hiệu → ngày văn bản → cơ quan ban hành → loại → trích yếu), biểu mẫu mở NGAY
 * TRONG luồng trang chứ không phải một hộp thoại nổi, và mọi lần từ chối đều hiện thành chữ to
 * rõ chứ không im lặng bỏ qua.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG QUYỀN BỌC PHẦN GHI, KHÔNG BỌC BẢNG — cùng khuôn với tab Danh mục và tab Thời hạn xử lý, và
 * cùng lý do: ẩn cả bảng là giao diện từ chối điều máy chủ đang phục vụ. Tuyến đọc đòi
 * `document.read` thật, nên tài khoản thiếu khoá ấy nhận 403 ở lượt đọc và màn hình hiện NGUYÊN
 * câu của máy chủ; không có cổng client nào đoán trước điều đó.
 *
 * HAI KHOÁ GHI, KHÔNG MỘT: `document.create` mở biểu mẫu vào sổ / sửa / gỡ, `document.route` mở
 * riêng khối chuyển xử lý. Ẩn nút là TIỆN DỤNG, không phải biện pháp — cả năm tuyến khai
 * `RequirePermission` và kiểm trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG CÓ NÚT NÀO SỬA HAY XOÁ MỘT DÒNG LỊCH SỬ CHUYỂN XỬ LÝ, và vế phủ định ấy là vế chịu lực:
 * bảng lịch sử chỉ-thêm ở tầng CSDL (trigger `lich_su_chuyen_chi_them`), hợp đồng không có tuyến
 * nào sửa hay xoá nó, và một nút như thế sẽ là một nút gọi vào hư không (luật 7, cấm #5).
 *
 * CHUYỂN XỬ LÝ NẰM TRONG NGĂN CHI TIẾT (`ngan-van-ban-den.tsx`), không trong biểu mẫu của trang:
 * người chuyển cần thấy dòng thời gian đã có TRƯỚC khi viết thêm một dòng không sửa được. Ba biểu
 * mẫu vào sổ / sửa / gỡ vẫn mở trong luồng trang như cũ.
 */

/* ---- trạng thái ---------------------------------------------------------------------------- */

/** Biểu mẫu nào đang mở. MỘT biểu mẫu cho cả màn: hai bản nháp cùng lúc là hai lần gửi nhầm. */
export type DangMoDen =
  | { kieu: "them"; khoaChongTrung: string }
  | { kieu: "sua"; vb: documents_vanBanDenRa }
  | { kieu: "go"; vb: documents_vanBanDenRa }
  | null;

/** Bản nháp đang gõ. Chuỗi hết — ô nhập của trình duyệt trả về chuỗi. */
export type BanNhapDen = {
  ngayDen: string;
  soKyHieu: string;
  ngayVanBan: string;
  coQuanBanHanh: string;
  loaiVanBan: string;
  trichYeu: string;
  doKhan: string;
  lyDoGo: string;
};

export const BAN_DEN_TRONG: BanNhapDen = {
  ngayDen: "",
  soKyHieu: "",
  ngayVanBan: "",
  coQuanBanHanh: "",
  loaiVanBan: "",
  trichYeu: "",
  doKhan: "",
  lyDoGo: "",
};

/** Năm thao tác một dòng hoặc thanh nút có thể yêu cầu. */
export type ThaoTacDen = {
  readonly them: () => void;
  readonly sua: (vb: documents_vanBanDenRa) => void;
  readonly go: (vb: documents_vanBanDenRa) => void;
  /** Mở ngăn chi tiết, tiêu điểm vào ô bộ phận của khối chuyển xử lý. */
  readonly chuyen: (vb: documents_vanBanDenRa) => void;
  /** Mở ngăn chi tiết, tiêu điểm vào tiêu đề ngăn. */
  readonly xem: (vb: documents_vanBanDenRa) => void;
};

/**
 * `id` DOM của nút "Xem chi tiết" trên một dòng — chỗ tiêu điểm quay về khi ngăn đóng.
 * Chỉ mang id văn bản (ULID mờ đục), không mang trích yếu hay cơ quan ban hành (luật 3, cấm #4).
 */
export function idNutXem(id: string): string {
  return `xem-van-ban-den-${id}`;
}

/** Ngày hôm nay dạng `YYYY-MM-DD` — giá trị mặc định của ô "Ngày đến". */
function homNay(): string {
  const d = new Date();
  const hai = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${hai(d.getMonth() + 1)}-${hai(d.getDate())}`;
}

/** Nạp bản nháp từ một dòng đang có — biểu mẫu sửa mở ra với đúng giá trị đang lưu. */
function banTuDong(vb: documents_vanBanDenRa): BanNhapDen {
  return {
    ...BAN_DEN_TRONG,
    ngayDen: vb.received_date,
    soKyHieu: vb.reference_no ?? "",
    ngayVanBan: vb.document_date ?? "",
    coQuanBanHanh: vb.issuing_body,
    loaiVanBan: vb.document_type,
    trichYeu: vb.summary,
    doKhan: vb.urgency ?? "",
  };
}

/* ---- vỏ đọc dữ liệu ------------------------------------------------------------------------ */

/**
 * Câu của dải lọc Tổng quan trên sổ này. NĂM CỦA SỔ tắt cùng các ô khác, và đó là ô dễ quên nhất:
 * `open`/`overdue` đếm mọi năm, còn một kỳ `arrived` có thể vắt qua ngày 01/01.
 */
export const DRILL_DOWN_NOTE_INCOMING =
  "Năm của sổ, bộ lọc, ô tìm và thứ tự tạm tắt để danh sách khớp đúng con số ở trang Tổng quan. " +
  "Bấm “Bỏ lọc” để dùng lại.";

export function SoVanBanDen({
  drillDown = NO_DRILL_DOWN,
}: {
  /**
   * Lọc mở từ trang Tổng quan — đọc ở MÁY CHỦ (`app/van-ban/page.tsx`) và chuyển xuống bằng prop.
   * Quyển sổ vẫn KHÔNG tự đọc hay ghi thanh địa chỉ (`ngan-van-ban-den.test.tsx`, ca cuối).
   */
  drillDown?: DrillDown<"incoming-documents">;
} = {}) {
  const drillDownActive = drillDown.kind === "active";
  // Cán bộ đã tự đổi bộ lọc: câu "đường dẫn lọc không hợp lệ — đang hiện toàn bộ" hết đúng.
  const [filtersChanged, setFiltersChanged] = useState(false);
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);
  const [trangThai, datTrangThai] = useState("");
  const [loaiLoc, datLoaiLoc] = useState("");
  const [boPhanLoc, datBoPhanLoc] = useState("");
  const [tim, datTim] = useState("");
  const [thuTu, datThuTu] = useState<MaThuTu>("");

  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  /** Tăng sau mỗi lần ghi thành công để ĐỌC LẠI từ máy chủ, không vá mảng tại chỗ. */
  const [lanDoc, datLanDoc] = useState(0);

  /**
   * Kết quả đọc, GIỮ KÈM BỘ LỌC ĐÃ SINH RA NÓ; "đang tải" được SUY RA từ chỗ bộ lọc lưu khác bộ
   * lọc đang chọn. Đặt `null` trong thân effect thì đơn giản hơn, nhưng nó là một lần `setState`
   * đồng bộ trong effect (cascading render, `react-hooks/set-state-in-effect`) — và cùng khuôn
   * này đã dùng cho hai bảng theo năm ở tab Thời hạn xử lý, vì cùng một lý do: không để bảng của
   * bộ lọc cũ đứng dưới một ô lọc đã hiện giá trị mới.
   */
  const [kq, datKq] = useState<{
    loc: LocVanBanDen;
    kq: KetQua<page_Result_documents_vanBanDenRa>;
  } | null>(null);
  const [loai, datLoai] = useState<BangTraDanhMuc>({ pha: "dangDoc" });
  const [boPhan, datBoPhan] = useState<BangTraDanhMuc>({ pha: "dangDoc" });

  const [dangMo, datDangMo] = useState<DangMoDen>(null);
  const [ban, datBan] = useState<BanNhapDen>(BAN_DEN_TRONG);
  const [loi, datLoi] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  /**
   * NGĂN CHI TIẾT ĐANG MỞ — chỉ id văn bản, không gì khác. Không đẩy lên URL, không cất vào bộ nhớ
   * trình duyệt: không có gì trong ngăn cần sống qua một lần tải lại trang.
   */
  const [xem, datXem] = useState<{ id: string; tieuDiemChuyen: boolean } | null>(null);
  /** Tăng sau mỗi lần chuyển (hay sửa) thành công để ĐỌC LẠI văn bản và dòng thời gian. */
  const [lanDocNgan, datLanDocNgan] = useState(0);
  /**
   * Câu trả lời của hai tuyến đọc, giữ KÈM id đã sinh ra nó. Đổi sang văn bản khác thì id lệch và
   * ngăn hiện "đang tải"; đọc lại CÙNG văn bản sau một lần chuyển thì dữ liệu cũ đứng yên tới khi
   * dữ liệu mới về — khối chuyển không bị gỡ ra rồi dựng lại dưới tay người đang dùng.
   */
  const [ngan, datNgan] = useState<{
    id: string;
    vb: KetQua<documents_vanBanDenRa>;
    lichSu: KetQua<documents_danhSachLichSuChuyenRa>;
  } | null>(null);
  const [banChuyen, datBanChuyen] = useState<BanChuyen>(BAN_CHUYEN_TRONG);
  const [loiChuyen, datLoiChuyen] = useState("");
  const [dangGuiChuyen, datDangGuiChuyen] = useState(false);
  const [cauChuyenXong, datCauChuyenXong] = useState("");

  const phien = usePhien();
  // BA TRẠNG THÁI, KHÔNG HAI: chưa đọc xong phiên thì chưa vẽ nút ghi nào. "Chưa biết" không được
  // hành xử như "có quyền", và cũng không được hành xử như "thiếu quyền".
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_GHI_SO_VAN_BAN);
  const quyetDinhChuyen = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_CHUYEN_VAN_BAN);

  const loc = useMemo<LocVanBanDen>(
    () =>
      // LỌC TỔNG QUAN LÀ BỘ LỌC DUY NHẤT KHI BẬT — kể cả `nam` VẮNG MẶT (xem `DRILL_DOWN_NOTE_INCOMING`).
      // Hàng lọc không vẽ, nên các state kia đứng ở giá trị đầu và không được ghép vào.
      drillDownActive
        ? { ...drillDownQuery(drillDown), cursor: nganXep.hienTai }
        : {
            nam,
            trangThai,
            loaiVanBan: loaiLoc,
            boPhanDangGiu: boPhanLoc,
            tim,
            ...sapXepTheoThuTu(thuTu),
            cursor: nganXep.hienTai,
          },
    [drillDownActive, drillDown, nam, trangThai, loaiLoc, boPhanLoc, tim, thuTu, nganXep],
  );

  useEffect(() => {
    let bo = false;
    laySoVanBanDen(loc).then((k) => {
      if (!bo) datKq({ loc, kq: k });
    });
    return () => {
      bo = true;
    };
  }, [loc, lanDoc]);

  // HAI DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN HÌNH — không phải mỗi dòng một lời gọi. Loại văn bản tra
  // theo MÃ (hồ sơ giữ mã làm giá trị), bộ phận tra theo id.
  useEffect(() => {
    let bo = false;
    layLoaiVanBan().then((k) => {
      if (bo) return;
      datLoai(
        bangTraTuKetQua(
          k.ok
            ? { ok: true, duLieu: { items: k.duLieu.items.map((m) => ({ id: m.code, name: m.label })) } }
            : k,
        ),
      );
    });
    layDanhMucBoPhan().then((k) => {
      if (!bo) datBoPhan(bangTraTuKetQua(k));
    });
    return () => {
      bo = true;
    };
  }, []);

  const idXem = xem?.id ?? null;
  useEffect(() => {
    if (idXem === null) return;
    let bo = false;
    // HAI TUYẾN, MỘT LẦN GÁN: ngăn hiện văn bản và dòng thời gian của CÙNG một lượt đọc, không bao
    // giờ văn bản sau lần chuyển cạnh dòng thời gian trước lần chuyển.
    Promise.all([layVanBanDen(idXem), layLichSuChuyenVanBanDen(idXem)]).then(([vb, lichSu]) => {
      if (!bo) datNgan({ id: idXem, vb, lichSu });
    });
    return () => {
      bo = true;
    };
  }, [idXem, lanDocNgan]);

  const moNgan = useCallback((vb: documents_vanBanDenRa, tieuDiemChuyen: boolean) => {
    datXem({ id: vb.id, tieuDiemChuyen });
    datBanChuyen(BAN_CHUYEN_TRONG);
    datLoiChuyen("");
    datCauChuyenXong("");
  }, []);

  const dongNgan = useCallback(() => {
    const id = xem?.id;
    datXem(null);
    // BẢN NHÁP LÝ DO BỎ ĐI CÙNG NGĂN — nó chỉ từng sống trong trạng thái này (luật 3).
    datBanChuyen(BAN_CHUYEN_TRONG);
    datLoiChuyen("");
    datCauChuyenXong("");
    // TIÊU ĐIỂM VỀ ĐÚNG DÒNG VỪA MỞ, sau khi ngăn đã rời khỏi DOM. Dòng không còn trên trang (đã đổi
    // bộ lọc) thì thôi — không đoán một chỗ khác.
    if (id !== undefined) {
      requestAnimationFrame(() => document.getElementById(idNutXem(id))?.focus());
    }
  }, [xem]);

  const guiChuyen = useCallback(() => {
    if (xem === null || dangGuiChuyen) return;
    datLoiChuyen("");
    datCauChuyenXong("");
    datDangGuiChuyen(true);
    // PHÉP KIỂM BỘ PHẬN VÀ LÝ DO NẰM TRONG `guiChuyenVanBan` — xem `thao-tac-van-ban.ts`.
    void guiChuyenVanBan(xem.id, banChuyen).then((k) => {
      datDangGuiChuyen(false);
      if (!k.ok) {
        datLoiChuyen(k.thongBao);
        return;
      }
      datBanChuyen(BAN_CHUYEN_TRONG);
      datCauChuyenXong("Đã chuyển và ghi vào dòng thời gian.");
      // ĐỌC LẠI CẢ BA: văn bản, dòng thời gian, và dòng của sổ (bộ phận đang giữ vừa đổi).
      datLanDocNgan((n) => n + 1);
      datLanDoc((n) => n + 1);
    });
  }, [banChuyen, dangGuiChuyen, xem]);

  /** Đổi một bộ lọc, ô tìm hay thứ tự là về TRANG ĐẦU — `doiLocVeTrangDau`. */
  const doiLoc = useCallback((dat: () => void) => {
    setFiltersChanged(true);
    doiLocVeTrangDau(dat, datNganXep);
  }, []);

  const mo = useCallback((m: DangMoDen, banDau: BanNhapDen) => {
    datDangMo(m);
    datBan(banDau);
    datLoi("");
    datCauDaXong("");
  }, []);

  const dong = useCallback(() => {
    datDangMo(null);
    datBan(BAN_DEN_TRONG);
    datLoi("");
  }, []);

  const thaoTac: ThaoTacDen = {
    // KHOÁ CHỐNG TRÙNG SINH Ở ĐÂY, LÚC MỞ BIỂU MẪU — không lúc gửi. Sinh lúc gửi thì mỗi lần bấm
    // lại sau một lỗi mạng là một khoá MỚI, tức một số đến thứ hai bị tiêu (`lib/api/van-ban.ts`).
    them: () => mo({ kieu: "them", khoaChongTrung: crypto.randomUUID() }, { ...BAN_DEN_TRONG, ngayDen: homNay() }),
    sua: (vb) => mo({ kieu: "sua", vb }, banTuDong(vb)),
    go: (vb) => mo({ kieu: "go", vb }, BAN_DEN_TRONG),
    chuyen: (vb) => moNgan(vb, true),
    xem: (vb) => moNgan(vb, false),
  };

  /** Một lượt ghi: dọn thông báo cũ, gửi, rồi hoặc nói đã làm gì và ĐỌC LẠI, hoặc hiện NGUYÊN câu
   *  máy chủ viết. Không rẽ nhánh theo `code`, không hiện `trace_id`, không hiện số hiệu HTTP. */
  const thucHien = useCallback(function <T>(
    goi: Promise<KetQua<T>>,
    cau: string,
    sauKhiXong?: () => void,
  ) {
    datLoi("");
    datCauDaXong("");
    datDangGui(true);
    void goi.then((k) => {
      datDangGui(false);
      if (!k.ok) {
        datLoi(k.thongBao);
        return;
      }
      datDangMo(null);
      datBan(BAN_DEN_TRONG);
      datCauDaXong(cau);
      datLanDoc((n) => n + 1);
      // Ngăn chi tiết đang mở đọc lại luôn: một lần sửa đổi đúng những ô ngăn đang hiện.
      datLanDocNgan((n) => n + 1);
      sauKhiXong?.();
    });
  }, []);

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    // KHÔNG KIỂM ĐỘ DÀI, KHUÔN NGÀY, HAY NGÀY VĂN BẢN SAU NGÀY ĐẾN Ở ĐÂY. Máy chủ kiểm cả ba, mỗi
    // thứ kèm một câu tiếng Việt nói rõ phải sửa gì (`domain/van_ban.go`); chép chúng xuống client
    // là dựng bản sao thứ hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
    switch (dangMo.kieu) {
      case "them":
        thucHien(
          vaoSoVanBanDen(
            {
              received_date: ban.ngayDen,
              reference_no: ban.soKyHieu,
              document_date: ban.ngayVanBan,
              issuing_body: ban.coQuanBanHanh,
              document_type: ban.loaiVanBan,
              summary: ban.trichYeu,
              urgency: ban.doKhan,
            },
            dangMo.khoaChongTrung,
          ),
          "Đã vào sổ văn bản đến. Số đến do hệ thống cấp.",
        );
        return;
      case "sua":
        thucHien(
          suaVanBanDen(dangMo.vb.id, {
            received_date: ban.ngayDen,
            reference_no: ban.soKyHieu,
            document_date: ban.ngayVanBan,
            issuing_body: ban.coQuanBanHanh,
            document_type: ban.loaiVanBan,
            summary: ban.trichYeu,
            urgency: ban.doKhan,
          }),
          "Đã lưu thay đổi.",
        );
        return;
      default: {
        // PHÉP KIỂM LÝ DO NẰM TRONG `guiGoVanBanDen`, không ở đây — xem `thao-tac-van-ban.ts`.
        const idGo = dangMo.vb.id;
        thucHien(
          guiGoVanBanDen(idGo, ban.lyDoGo),
          "Đã gỡ văn bản khỏi sổ. Số đến của văn bản ấy không được cấp lại.",
          // Ngăn đang mở đúng văn bản vừa gỡ thì đóng: đọc lại nó chỉ còn ra câu "không tìm thấy".
          () => datXem((x) => (x !== null && x.id === idGo ? null : x)),
        );
      }
    }
  }, [ban, dangGui, dangMo, thucHien]);

  const nganHienTai = ngan !== null && ngan.id === idXem ? ngan : null;

  return (
    <ManSoVanBanDen
      drillDown={drillDown}
      showInvalidDrillDown={!filtersChanged}
      kq={kq !== null && kq.loc === loc ? kq.kq : null}
      // "Tải lại" after a failed read asks the SAME read again through the existing re-read key.
      onReload={() => datLanDoc((n) => n + 1)}
      bayGio={new Date()}
      nam={nam}
      namGoc={namGoc}
      datNam={(n) => doiLoc(() => datNam(n))}
      trangThai={trangThai}
      datTrangThai={(v) => doiLoc(() => datTrangThai(v))}
      loaiLoc={loaiLoc}
      datLoaiLoc={(v) => doiLoc(() => datLoaiLoc(v))}
      boPhanLoc={boPhanLoc}
      datBoPhanLoc={(v) => doiLoc(() => datBoPhanLoc(v))}
      tim={tim}
      datTim={(v) => doiLoc(() => datTim(v))}
      thuTu={thuTu}
      datThuTu={(v) => doiLoc(() => datThuTu(v))}
      traLoai={loai}
      traBoPhan={boPhan}
      coQuyenGhi={quyetDinhGhi !== null && quyetDinhGhi.hien}
      thieuQuyenGhi={
        quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen"
      }
      coQuyenChuyen={quyetDinhChuyen !== null && quyetDinhChuyen.hien}
      thaoTac={thaoTac}
      cauDaXong={cauDaXong}
      loiNgoaiForm={dangMo === null ? loi : ""}
      nganXep={nganXep}
      diToiTrang={datNganXep}
      idDangXem={idXem}
      ngan={
        xem === null ? null : (
          // `key` theo id: mở văn bản khác là một ngăn MỚI — tiêu điểm về tiêu đề, bản nháp sạch.
          <NganVanBanDen
            key={xem.id}
            vb={nganHienTai?.vb ?? null}
            lichSu={nganHienTai?.lichSu ?? null}
            bayGio={new Date()}
            traLoai={loai}
            traBoPhan={boPhan}
            coQuyenChuyen={quyetDinhChuyen !== null && quyetDinhChuyen.hien}
            ban={banChuyen}
            datBan={datBanChuyen}
            loi={loiChuyen}
            dangGui={dangGuiChuyen}
            cauDaXong={cauChuyenXong}
            tieuDiemChuyen={xem.tieuDiemChuyen}
            onGui={guiChuyen}
            onDong={dongNgan}
          />
        )
      }
      form={
        dangMo === null ? null : (
          <BieuMauVanBanDen
            dangMo={dangMo}
            ban={ban}
            datBan={datBan}
            traLoai={loai}
            loi={loi}
            dangGui={dangGui}
            onGui={guiBieuMau}
            onHuy={dong}
          />
        )
      }
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

/**
 * Toàn bộ phần nhìn thấy được của sổ văn bản đến, THUẦN TRÌNH BÀY.
 *
 * XUẤT RA để bài kiểm kết xuất được bằng `react-dom/server` mà không cần trình duyệt giả lập. Lỗ
 * hổng đã đo ở tab Danh mục: mọi ca kiểm canh một QUYẾT ĐỊNH trong module thuần, còn việc quyết
 * định ấy có ra tới trang hay không thì không ca nào canh.
 */
export function ManSoVanBanDen({
  drillDown = NO_DRILL_DOWN,
  showInvalidDrillDown = true,
  kq,
  onReload,
  bayGio,
  nam,
  namGoc,
  datNam,
  trangThai,
  datTrangThai,
  loaiLoc,
  datLoaiLoc,
  boPhanLoc,
  datBoPhanLoc,
  tim,
  datTim,
  thuTu,
  datThuTu,
  traLoai,
  traBoPhan,
  coQuyenGhi,
  thieuQuyenGhi,
  coQuyenChuyen,
  thaoTac,
  cauDaXong,
  loiNgoaiForm,
  nganXep,
  diToiTrang,
  idDangXem = null,
  ngan = null,
  form,
}: {
  /** Lọc mở từ trang Tổng quan. Bật thì hàng lọc không vẽ và dải "Đang xem" nói lọc gì. */
  drillDown?: DrillDown<"incoming-documents">;
  showInvalidDrillDown?: boolean;
  kq: KetQua<page_Result_documents_vanBanDenRa> | null;
  /** Re-read after a failed load ("Tải lại"). Absent → the error state draws no button. */
  onReload?: () => void;
  /** Thời điểm hiện tại TRUYỀN VÀO, không đọc đồng hồ trong lúc vẽ: bài kiểm phải đứng được ở
   *  hai phía của một hạn. */
  bayGio: Date;
  nam: number;
  namGoc: number;
  datNam: (n: number) => void;
  trangThai: string;
  datTrangThai: (v: string) => void;
  loaiLoc: string;
  datLoaiLoc: (v: string) => void;
  boPhanLoc: string;
  datBoPhanLoc: (v: string) => void;
  tim: string;
  datTim: (v: string) => void;
  thuTu: MaThuTu;
  datThuTu: (v: MaThuTu) => void;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
  coQuyenGhi: boolean;
  thieuQuyenGhi: boolean;
  coQuyenChuyen: boolean;
  thaoTac: ThaoTacDen;
  cauDaXong: string;
  loiNgoaiForm: string;
  nganXep: NganXepConTro;
  diToiTrang: (toi: NganXepConTro) => void;
  /** Id văn bản đang mở trong ngăn chi tiết — nút của dòng ấy mang `aria-expanded="true"`. */
  idDangXem?: string | null;
  /** Ngăn chi tiết, dựng sẵn bởi bên gọi (`NganVanBanDen`). */
  ngan?: ReactNode;
  form: ReactNode;
}) {
  // "Nothing at all" vs "nothing under these filters" (spec §8b) — read off the props this screen
  // already holds: the default year, every filter empty, the first page, no drill-down. Only then
  // may the empty table say the register has nothing yet.
  const filtered =
    drillDown.kind === "active" ||
    nam !== namGoc ||
    trangThai !== "" ||
    loaiLoc !== "" ||
    boPhanLoc !== "" ||
    tim !== "" ||
    coTrangTruoc(nganXep);

  return (
    <section
      className="man-van-ban mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0"
      aria-labelledby="tieu-de-so-den"
    >
      <Card>
        <CardHeader className="justify-between">
          <div className="min-w-0 flex-1 basis-64">
            <CardTitle as="h2" id="tieu-de-so-den" className="flex items-center gap-2">
              <Glyph icon={Inbox} className="size-[18px] shrink-0 text-brand-600" />
              Sổ văn bản đến
            </CardTitle>
            <p className="m-0 mt-1 text-[13px] text-ink-500">{DAN_SO_DEN}</p>
          </div>
          {coQuyenGhi && (
            <Button type="button" variant="primary" icon={<Glyph icon={Plus} />} onClick={thaoTac.them}>
              {NUT_VAO_SO}
            </Button>
          )}
        </CardHeader>

        {/* Messages, the open form and the drill-down line, in the order they always had. `empty:hidden`:
            with nothing to say the block takes no room. */}
        <div className="flex min-w-0 flex-col gap-3 border-b border-line px-4 py-3 empty:hidden [&>*]:my-0">
          {thieuQuyenGhi && (
            <Notice tone="neutral">
              Tài khoản của bạn không có quyền vào sổ, sửa hay gỡ văn bản. Sổ dưới đây vẫn xem được.
            </Notice>
          )}

          {cauDaXong !== "" && (
            <p role="status" className="flex items-center gap-2 text-sm font-medium text-success-600">
              <Glyph icon={CircleCheck} className="size-[18px] shrink-0" />
              {cauDaXong}
            </p>
          )}
          {loiNgoaiForm !== "" && (
            <p className="thong-bao-loi" role="alert">
              {loiNgoaiForm}
            </p>
          )}

          {form}

          <DrillDownBanner
            drillDown={drillDown}
            clearHref="/van-ban"
            note={DRILL_DOWN_NOTE_INCOMING}
            showInvalid={showInvalidDrillDown}
          />
        </div>

      {drillDown.kind !== "active" && (
        <LocSoVanBanDen
          nam={nam}
          namGoc={namGoc}
          datNam={datNam}
          trangThai={trangThai}
          datTrangThai={datTrangThai}
          loaiLoc={loaiLoc}
          datLoaiLoc={datLoaiLoc}
          boPhanLoc={boPhanLoc}
          datBoPhanLoc={datBoPhanLoc}
          tim={tim}
          datTim={datTim}
          thuTu={thuTu}
          datThuTu={datThuTu}
          traLoai={traLoai}
          traBoPhan={traBoPhan}
        />
      )}

      <BangVanBanDen
        kq={kq}
        bayGio={bayGio}
        traLoai={traLoai}
        traBoPhan={traBoPhan}
        coQuyenGhi={coQuyenGhi}
        coQuyenChuyen={coQuyenChuyen}
        thaoTac={thaoTac}
        soCuTruoc={thuTu === "so-tang"}
        idDangXem={idDangXem}
        filtered={filtered}
        onReload={onReload}
      />

      {kq !== null && kq.ok && (
        <CardFooter className="justify-end">
          <DieuHuongTrang
            nganXep={nganXep}
            conTroTiep={kq.duLieu.next_cursor}
            conTrangSau={kq.duLieu.has_more}
            diToiTrang={diToiTrang}
          />
        </CardFooter>
      )}
      </Card>

      {ngan}
    </section>
  );
}

/** Hàng lọc: năm · trạng thái · loại · bộ phận · tìm chữ · thứ tự — đúng các tham số tuyến nhận. */
function LocSoVanBanDen({
  nam,
  namGoc,
  datNam,
  trangThai,
  datTrangThai,
  loaiLoc,
  datLoaiLoc,
  boPhanLoc,
  datBoPhanLoc,
  tim,
  datTim,
  thuTu,
  datThuTu,
  traLoai,
  traBoPhan,
}: {
  nam: number;
  namGoc: number;
  datNam: (n: number) => void;
  trangThai: string;
  datTrangThai: (v: string) => void;
  loaiLoc: string;
  datLoaiLoc: (v: string) => void;
  boPhanLoc: string;
  datBoPhanLoc: (v: string) => void;
  tim: string;
  datTim: (v: string) => void;
  thuTu: MaThuTu;
  datThuTu: (v: MaThuTu) => void;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
}) {
  // Filters behind the "Bộ lọc" button that are not at their default — the defaults are the ones
  // this screen already starts from ("" = all, "" = server's number order).
  const moreActiveCount = [loaiLoc !== "", boPhanLoc !== "", thuTu !== ""].filter(Boolean).length;

  // Search first (owner, 02/10/2026: "ô tìm kiếm nên nằm ở bên trái"), then the register's year and
  // the status — the two filters that decide which page of the register is in front of you.
  return (
    <FilterBar
      id="incoming-document-filters"
      className="m-0 border-b border-line px-4 py-3.5"
      moreActiveCount={moreActiveCount}
      primary={
        <>
          <OTimVanBan id="tim-van-ban-den" goiY={GOI_Y_TIM_DEN} tim={tim} datTim={datTim} />

          <ChonNam id="nam-so-van-ban-den" nhan="Năm của sổ" nam={nam} namGoc={namGoc} datNam={datNam} />

          <p className="chon-hang-muc">
            <label htmlFor="loc-trang-thai-den">Trạng thái</label>{" "}
            <select
              id="loc-trang-thai-den"
              value={trangThai}
              onChange={(e) => datTrangThai(e.target.value)}
            >
              <option value="">Tất cả trạng thái</option>
              {MA_TRANG_THAI.map((ma) => (
                <option key={ma} value={ma}>
                  {nhanTrangThai(ma)}
                </option>
              ))}
            </select>
          </p>
        </>
      }
      more={
        <>
          <p className="chon-hang-muc">
            <label htmlFor="loc-loai-den">Loại văn bản</label>{" "}
            <select id="loc-loai-den" value={loaiLoc} onChange={(e) => datLoaiLoc(e.target.value)}>
              <option value="">Tất cả loại</option>
              {traLoai.pha === "xong" &&
                [...traLoai.ten].map(([ma, ten]) => (
                  <option key={ma} value={ma}>
                    {ten}
                  </option>
                ))}
            </select>
          </p>

          <p className="chon-hang-muc">
            <label htmlFor="loc-bo-phan-den">Bộ phận đang giữ</label>{" "}
            <select id="loc-bo-phan-den" value={boPhanLoc} onChange={(e) => datBoPhanLoc(e.target.value)}>
              <option value="">Tất cả bộ phận</option>
              {traBoPhan.pha === "xong" &&
                [...traBoPhan.ten].map(([id, ten]) => (
                  <option key={id} value={id}>
                    {ten}
                  </option>
                ))}
            </select>
          </p>

          <ChonThuTu id="thu-tu-so-den" thuTu={thuTu} datThuTu={datThuTu} />
        </>
      }
    />
  );
}

/**
 * Bảng sổ văn bản đến.
 *
 * Ô TRÍCH YẾU VÀ Ô CƠ QUAN BAN HÀNH HIỆN NGUYÊN VĂN, không che: đó là nội dung quyển sổ, và máy
 * chủ trả về nguyên vẹn cho cán bộ của chính xã ấy. Điều màn hình BẢO ĐẢM hẹp hơn và nói ra được:
 * hai giá trị ấy không đi vào một `aria-label`, một `title`, một tên tệp hay một URL nào (luật 3,
 * cấm #4) — nhãn trợ năng của từng nút dùng SỐ ĐẾN, thứ vốn để đọc qua điện thoại.
 */
export function BangVanBanDen({
  kq,
  bayGio,
  traLoai,
  traBoPhan,
  coQuyenGhi,
  coQuyenChuyen,
  thaoTac,
  soCuTruoc = false,
  idDangXem = null,
  filtered = false,
  onReload,
}: {
  kq: KetQua<page_Result_documents_vanBanDenRa> | null;
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
  coQuyenGhi: boolean;
  coQuyenChuyen: boolean;
  thaoTac: ThaoTacDen;
  /** Chú thích bảng nói đúng thứ tự đang xem — một câu "số mới nhất trước" trên bảng xếp tăng là sai. */
  soCuTruoc?: boolean;
  idDangXem?: string | null;
  /** A filter, a later page or a drill-down is on: an empty answer is "nothing matches", not "nothing yet". */
  filtered?: boolean;
  /** "Tải lại" on a failed read. Absent → no button. */
  onReload?: () => void;
}) {
  if (kq === null) {
    // FIRST LOAD (spec §8b): the sentence stays the live region, read out as before; the eye gets
    // row-shaped placeholders so the card does not jump when the page arrives.
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải sổ văn bản đến…
        </p>
        <RegisterRowsSkeleton />
      </>
    );
  }
  if (!kq.ok) {
    // NGUYÊN VĂN câu máy chủ viết — kể cả 403 của tài khoản thiếu `document.read`. The screen does
    // not branch on the status code, so there is ONE error look here, not a separate "no permission".
    return (
      <EmptyState
        icon={CloudOff}
        title="Chưa tải được sổ văn bản đến"
        description={
          <span className="text-danger-600" role="alert">
            {kq.thongBao}
          </span>
        }
        action={
          onReload !== undefined ? (
            <Button type="button" variant="secondary" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
              Tải lại
            </Button>
          ) : undefined
        }
      />
    );
  }
  if (kq.duLieu.items.length === 0) {
    // The sentence is the register's own (`SO_DEN_RONG`), unchanged; only the icon and the hint say
    // whether a filter is the reason.
    return (
      <EmptyState
        icon={filtered ? SearchX : Inbox}
        title={SO_DEN_RONG}
        description={filtered ? "Thử đổi hoặc bỏ bớt bộ lọc." : undefined}
      />
    );
  }

  const coThaoTac = coQuyenGhi || coQuyenChuyen;

  return (
    <div className={REGISTER_TABLE_SCROLLER} role="region" aria-label="Sổ văn bản đến" tabIndex={0}>
      <table className="bang-danh-muc bang-van-ban">
        <caption className="an-thi-giac">
          Các văn bản đến đã vào sổ, {soCuTruoc ? "số cũ nhất trước" : "số mới nhất trước"}
        </caption>
        <thead>
          <tr>
            <th scope="col">Số đến</th>
            <th scope="col">Ngày đến</th>
            <th scope="col">Số, ký hiệu</th>
            <th scope="col">Cơ quan ban hành</th>
            <th scope="col">Loại văn bản</th>
            <th scope="col">Trích yếu</th>
            <th scope="col">Độ khẩn</th>
            <th scope="col">Đang giữ</th>
            <th scope="col">Hạn xử lý</th>
            <th scope="col">Trạng thái</th>
            {coThaoTac && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {kq.duLieu.items.map((vb) => {
            // SUY RA LÚC VẼ, mỗi dòng một lần. Không có trường nào được lưu lại (luật 10, bất biến 3).
            const han = trangThaiHanVanBan(vb.due_at, bayGio);
            const so = nhanSoVaoSo(vb.number, vb.year);
            const dangXem = vb.id === idDangXem;
            const more = incomingRowMoreActions(vb, coQuyenGhi, coQuyenChuyen, thaoTac);
            return (
              <tr
                key={vb.id}
                className="cursor-pointer"
                // BẤM MỘT DÒNG LÀ MỞ NGĂN (§3.1) — lối tắt cho chuột. Lối cho bàn phím và trình đọc
                // màn hình là nút ở ô "Số đến"; bấm trúng một nút hay ô nhập trong dòng thì để nút
                // ấy làm việc của nó, không mở ngăn chồng lên.
                onClick={(e) => {
                  if ((e.target as Element).closest("button, a, input, select, textarea")) return;
                  thaoTac.xem(vb);
                }}
              >
                <td>
                  <Button
                    type="button"
                    id={idNutXem(vb.id)}
                    variant="ghost"
                    size="sm"
                    className="px-2 font-semibold text-brand-700 tabular-nums"
                    aria-expanded={dangXem}
                    aria-label={`${NUT_XEM} văn bản đến số ${so}`}
                    onClick={() => thaoTac.xem(vb)}
                  >
                    {so}
                  </Button>
                </td>
                <td>{nhanNgayCoThe(vb.received_date)}</td>
                <td>{vb.reference_no === undefined || vb.reference_no === "" ? "Không ghi" : vb.reference_no}</td>
                <td>{vb.issuing_body}</td>
                <td>{nhanLoaiVanBan(traTen(traLoai, vb.document_type))}</td>
                <td className="o-trich-yeu">{vb.summary}</td>
                <td>{nhanDoKhan(vb.urgency ?? "")}</td>
                <td>{nhanBoPhanDangGiu(traTen(traBoPhan, vb.holding_unit ?? ""))}</td>
                <td>
                  <DeadlineMark deadline={han} />
                </td>
                <td>
                  <DocumentStatusBadge status={vb.status}>{nhanTrangThai(vb.status)}</DocumentStatusBadge>
                </td>
                {coThaoTac && (
                  <td>
                    {/* ONE visible control for the row's everyday action, the rest in "⋯" with WORDS
                        (spec v2 §7): Sửa as an icon when the account may write, otherwise "Chuyển xử
                        lý" with its words. Every label names the row by its NUMBER, never by its
                        summary or issuing body (rule 3, forbidden #4). */}
                    <span className="flex items-center justify-end gap-1">
                      {coQuyenGhi ? (
                        <IconButton
                          type="button"
                          label={`${NUT_SUA} văn bản đến số ${so}`}
                          onClick={() => thaoTac.sua(vb)}
                        >
                          <Pencil aria-hidden="true" />
                        </IconButton>
                      ) : (
                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          icon={<Glyph icon={Forward} />}
                          aria-label={`${NUT_CHUYEN} văn bản đến số ${so}`}
                          onClick={() => thaoTac.chuyen(vb)}
                        >
                          {NUT_CHUYEN}
                        </Button>
                      )}
                      {more.length > 0 && (
                        <ActionMenu label={`Thao tác khác: văn bản đến số ${so}`} items={more} />
                      )}
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

/**
 * The "⋯" items of one incoming row — everything the visible control does not already do:
 * "Chuyển xử lý" (`document.route`) when Sửa holds the visible slot, then — after a separator, red,
 * last — "Gỡ khỏi sổ" (`document.create`). Without `document.create` the visible control IS
 * "Chuyển xử lý", so the list is empty and no menu is drawn. Each item calls the existing handler.
 *
 * Exported so tests read which actions a row offers without opening a Radix menu (a closed menu
 * renders no items to the HTML).
 */
export function incomingRowMoreActions(
  doc: documents_vanBanDenRa,
  canWrite: boolean,
  canRoute: boolean,
  actions: ThaoTacDen,
): ActionMenuItem[] {
  if (!canWrite) return [];
  const items: ActionMenuItem[] = [];
  if (canRoute) {
    items.push({ kind: "item", id: "chuyen", label: NUT_CHUYEN, icon: Forward, onSelect: () => actions.chuyen(doc) });
    items.push({ kind: "separator", id: "tach-go" });
  }
  items.push({ kind: "item", id: "go", label: NUT_GO, icon: Trash2, tone: "danger", onSelect: () => actions.go(doc) });
  return items;
}

/** Hai nút trang. Đọc theo mốc nên "trang trước" phải tự nhớ — `ngan-xep-con-tro.ts`. */
export function DieuHuongTrang({
  nganXep,
  conTroTiep,
  conTrangSau,
  diToiTrang,
}: {
  nganXep: NganXepConTro;
  conTroTiep: string;
  conTrangSau: boolean;
  diToiTrang: (toi: NganXepConTro) => void;
}) {
  // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang m-0 flex gap-2" aria-label="Phân trang sổ văn bản">
      <Button
        type="button"
        variant="secondary"
        size="sm"
        icon={<Glyph icon={ChevronLeft} />}
        disabled={!coTrangTruoc(nganXep)}
        onClick={() => diToiTrang(veTrangTruoc(nganXep))}
      >
        Trang trước
      </Button>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        disabled={!coSau}
        onClick={() => diToiTrang(sangTrangSau(nganXep, conTroTiep))}
      >
        Trang sau
        <Glyph icon={ChevronRight} />
      </Button>
    </nav>
  );
}

/* ---- biểu mẫu ------------------------------------------------------------------------------ */

/** Titles of the two entry forms. The removal form asks the specific question instead — see below. */
const TIEU_DE_DEN: Record<"them" | "sua", string> = {
  them: "Vào sổ văn bản đến",
  sua: "Sửa văn bản đến",
};

/**
 * Title of the removal confirmation (spec v2 §7 "Hộp xác nhận"): the SPECIFIC question, naming the
 * document by its NUMBER — never by summary or issuing body, since the title is also the form's
 * `aria-label` (rule 3, forbidden #4).
 */
export function removeIncomingQuestion(number: string): string {
  return `Gỡ văn bản đến số ${number} khỏi sổ?`;
}

/**
 * Một biểu mẫu cho cả ba thao tác vào sổ / sửa / gỡ. Chuyển xử lý nằm trong ngăn chi tiết.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị đi vào qua `ban`, mọi thay đổi đi ra qua `datBan`, phép kiểm nằm ở
 * `thao-tac-van-ban.ts`. Tách như vậy để ba nhánh KHÔNG ai nhìn thấy trong lúc phát triển — "còn
 * thiếu lý do", "máy chủ vừa từ chối", và khối cảnh báo trước khi gỡ — kết xuất được bằng
 * `react-dom/server`.
 */
export function BieuMauVanBanDen({
  dangMo,
  ban,
  datBan,
  traLoai,
  loi,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: NonNullable<DangMoDen>;
  ban: BanNhapDen;
  datBan: (b: BanNhapDen) => void;
  traLoai: BangTraDanhMuc;
  /**
   * MỘT VÙNG LỖI, KHÔNG HAI — khác `BieuMauThoiHan` ở tab Cấu hình, và sự khác nhau ấy có lý do:
   * ở đó phép kiểm tại chỗ và câu từ chối của máy chủ có thể cùng có mặt, còn ở đây phép kiểm
   * (`thao-tac-van-ban.ts`) CẮT đường đi tới lời gọi mạng, nên hai câu không bao giờ cùng lúc.
   * Dựng hai vùng cho một thứ là dựng một vùng luôn trống mà không ai biết vì sao.
   */
  loi: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    onGui();
  };

  // CÂU TỪ CHỐI RA NGUYÊN VĂN, dù nó đến từ phép kiểm ở client hay từ máy chủ. Không rẽ nhánh theo
  // `code`, không hiện `trace_id`, không hiện số hiệu HTTP — trong đó có câu 409 nói xã chưa cấu
  // hình thời hạn xử lý, và câu ấy do máy chủ viết.
  const refusal =
    loi !== "" ? (
      <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
        {loi}
      </p>
    ) : null;

  if (dangMo.kieu === "go") {
    const question = removeIncomingQuestion(nhanSoVaoSo(dangMo.vb.number, dangMo.vb.year));
    return (
      <ConfirmDialog
        as="form"
        tone="danger"
        icon={Trash2}
        title={question}
        aria-label={question}
        onSubmit={submit}
        actions={
          <>
            <Button type="submit" variant="danger" disabled={dangGui} aria-busy={dangGui || undefined}>
              <BusyLabel busy={dangGui} label={NUT_XAC_NHAN_GO} busyText="Đang gỡ…" />
            </Button>
            <Button type="button" variant="secondary" onClick={onHuy} disabled={dangGui}>
              {NUT_HUY}
            </Button>
          </>
        }
      >
        {/* CÂU QUAN TRỌNG NHẤT MÀN HÌNH, và nó đứng ĐÚNG CHỖ sắp bấm xoá. */}
        <p className="m-0">{CANH_BAO_GO_KHONG_TRA_SO}</p>
        <Field
          label={O_LY_DO_GO}
          htmlFor="o-ly-do-go-den"
          grow="auto"
          hint={<span id="giai-thich-ly-do-go-den">{GIAI_THICH_LY_DO_GO}</span>}
        >
          {/* `required` là lớp nhắc của trình duyệt, KHÔNG phải phép kiểm: nó không bắt được một
              ô toàn dấu cách, và tắt được. Phép kiểm thật chạy trong `guiGoVanBanDen`. */}
          <input
            id="o-ly-do-go-den"
            name="lyDoGo"
            required
            value={ban.lyDoGo}
            onChange={(e) => datBan({ ...ban, lyDoGo: e.target.value })}
            aria-invalid={loi === LOI_THIEU_LY_DO_GO}
            aria-describedby="giai-thich-ly-do-go-den"
          />
        </Field>
        {refusal}
      </ConfirmDialog>
    );
  }

  const tieuDe = TIEU_DE_DEN[dangMo.kieu];

  return (
    <Card as="form" aria-label={tieuDe} onSubmit={submit}>
      <CardHeader className="justify-between">
        <CardTitle as="h3">{tieuDe}</CardTitle>
        {dangMo.kieu === "sua" && (
          <p className="m-0 text-[13px] text-ink-500 tabular-nums">
            Văn bản đến số {nhanSoVaoSo(dangMo.vb.number, dangMo.vb.year)}
          </p>
        )}
      </CardHeader>

      {/* Labels above, 40px controls, two columns from 640px (spec §6.3, §6.5). */}
      <div className="grid min-w-0 gap-4 p-4 sm:grid-cols-2">
          {/* THỨ TỰ Ô LÀ THỨ TỰ NGƯỜI TA ĐỌC TRÊN TỜ VĂN BẢN, không phải thứ tự cột của CSDL. */}
          <Field label={O_NGAY_DEN} htmlFor="o-ngay-den" grow="auto">
            {/* `type="date"` phát ra đúng `YYYY-MM-DD`, đúng khuôn hợp đồng đòi. Một ô chữ tự do
                mời gõ "2/9/2026" rồi nhận một lời từ chối không nói được phải gõ ra sao. */}
            <input
              id="o-ngay-den"
              name="ngayDen"
              type="date"
              value={ban.ngayDen}
              onChange={(e) => datBan({ ...ban, ngayDen: e.target.value })}
            />
          </Field>

          <Field label={O_SO_KY_HIEU} htmlFor="o-so-ky-hieu" grow="auto">
            <input
              id="o-so-ky-hieu"
              name="soKyHieu"
              value={ban.soKyHieu}
              onChange={(e) => datBan({ ...ban, soKyHieu: e.target.value })}
              placeholder="1742-CV/BTCTU"
            />
          </Field>

          <Field label={O_NGAY_VAN_BAN} htmlFor="o-ngay-van-ban" grow="auto">
            <input
              id="o-ngay-van-ban"
              name="ngayVanBan"
              type="date"
              value={ban.ngayVanBan}
              onChange={(e) => datBan({ ...ban, ngayVanBan: e.target.value })}
            />
          </Field>

          <Field label={O_CO_QUAN_BAN_HANH} htmlFor="o-co-quan" grow="auto">
            <input
              id="o-co-quan"
              name="coQuanBanHanh"
              value={ban.coQuanBanHanh}
              onChange={(e) => datBan({ ...ban, coQuanBanHanh: e.target.value })}
            />
          </Field>

          <Field label={O_LOAI_VAN_BAN} htmlFor="o-loai-van-ban" kind="select" grow="auto">
            <select
              id="o-loai-van-ban"
              name="loaiVanBan"
              value={ban.loaiVanBan}
              onChange={(e) => datBan({ ...ban, loaiVanBan: e.target.value })}
            >
              <option value="">— Chọn loại văn bản —</option>
              {traLoai.pha === "xong" &&
                [...traLoai.ten].map(([ma, ten]) => (
                  <option key={ma} value={ma}>
                    {ten}
                  </option>
                ))}
              {/* MÃ ĐANG LƯU LUÔN CÓ MẶT, kể cả khi xã đã tắt loại ấy: không có mục này thì mở
                  biểu mẫu sửa một văn bản cũ sẽ lặng lẽ đổi loại của nó sang mục đầu danh sách. */}
              {ban.loaiVanBan !== "" &&
                !(traLoai.pha === "xong" && traLoai.ten.has(ban.loaiVanBan)) && (
                  <option value={ban.loaiVanBan}>{ban.loaiVanBan} (không còn trong danh mục)</option>
                )}
            </select>
          </Field>

          <Field label={O_TRICH_YEU} htmlFor="o-trich-yeu" grow="auto" className="sm:col-span-2">
            <textarea
              id="o-trich-yeu"
              name="trichYeu"
              rows={3}
              className="py-2"
              value={ban.trichYeu}
              onChange={(e) => datBan({ ...ban, trichYeu: e.target.value })}
            />
          </Field>

          <Field label={O_DO_KHAN} htmlFor="o-do-khan" kind="select" grow="auto">
            <select
              id="o-do-khan"
              name="doKhan"
              value={ban.doKhan}
              onChange={(e) => datBan({ ...ban, doKhan: e.target.value })}
            >
              {/* CHUỖI RỖNG LÀ MỘT CÂU TRẢ LỜI THẬT, không phải "chưa chọn". */}
              <option value="">Không ghi độ khẩn</option>
              {MA_DO_KHAN.map((ma) => (
                <option key={ma} value={ma}>
                  {nhanDoKhan(ma)}
                </option>
              ))}
            </select>
          </Field>

          {dangMo.kieu === "them" && (
            <div className="flex min-w-0 flex-col gap-3 sm:col-span-2">
              <p className="m-0 text-[13px] text-ink-500">{DAN_HAN_DO_MAY_CHU_AN_DINH}</p>
              {/* ĐƯỜNG DẪN HIỆN THƯỜNG TRỰC, không chỉ khi máy chủ từ chối — xem
                  `DAN_DUONG_TOI_CAU_HINH`: rẽ nhánh theo lời văn của máy chủ là dựng một bản sao
                  của quy tắc nghiệp vụ ở client, và bản ấy hỏng lặng lẽ khi câu chữ đổi. */}
              <Notice tone="neutral" icon={Settings}>
                {DAN_DUONG_TOI_CAU_HINH} <Link href="/cau-hinh">{NHAN_DUONG_TOI_CAU_HINH}</Link>
              </Notice>
            </div>
          )}

          {refusal}
      </div>

      {/* Same order as before (submit, then cancel), right-aligned (spec §6.5). */}
      <CardFooter className="justify-end">
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
          <BusyLabel busy={dangGui} label={NUT_LUU} busyText="Đang lưu…" />
        </Button>
        <Button type="button" variant="secondary" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </Button>
      </CardFooter>
    </Card>
  );
}
