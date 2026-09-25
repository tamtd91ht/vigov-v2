"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";

import { ChonNam } from "@/components/chon-nam";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  gieoNgayNghiLeMacDinh,
  gieoTuanMacDinh,
  suaCaLamViec,
  suaNgayLamBu,
  suaNgayNghiLe,
  themCaLamViec,
  themNgayLamBu,
  themNgayNghiLe,
  xoaCaLamViec,
  xoaNgayLamBu,
  xoaNgayNghiLe,
} from "@/lib/api/lich-ghi";
import { layLichLamViec, layNgayLamBu, layNgayNghiLe } from "@/lib/api/lich-lam-viec";
import type {
  identity_caLamBuRa,
  identity_caLamViecRa,
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
  identity_danhSachSLARa,
  identity_dongSLARa,
  identity_ngayNghiLeRa,
} from "@/lib/api/schema.gen";
import { gieoThoiHanMacDinh, layThoiHanXuLy, suaThoiHanXuLy } from "@/lib/api/thoi-han-xu-ly";
import { namTheoDongHoMay } from "@/lib/nam";

import { khoiCanhBao, tinhTrangBang, type KhoiCanhBao } from "./chua-cau-hinh";
import { DAN_LICH_LAM_VIEC, nhanCa, nhanGio, nhanNgay, tenThu } from "./nhan-lich-lam-viec";
import {
  CANH_BAO_XOA_CA,
  CANH_BAO_XOA_NGAY_LAM_BU,
  CANH_BAO_XOA_NGAY_NGHI,
  CON_THIEU_NGAY_LE,
  DAN_THOI_HAN_1,
  DAN_THOI_HAN_2,
  DA_LUU_LICH,
  DA_LUU_THOI_HAN,
  DA_XOA_LICH,
  GHI_CHU_HAI_COT_LEO_THANG,
  GHI_CHU_SAU_KHI_GIEO,
  GIAI_THICH_CA,
  GIAI_THICH_LY_DO_XOA,
  KHOI_CHUA_KHAI_HAU_QUA,
  KHOI_CHUA_KHAI_THIEU_LICH_TUAN,
  KHOI_CHUA_KHAI_THIEU_THOI_HAN,
  KHOI_CHUA_KHAI_TIEU_DE,
  LOI_THIEU_LY_DO,
  NUT_GIEO_NGAY_LE,
  NUT_GIEO_THOI_HAN,
  NUT_GIEO_TUAN,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  NUT_THEM_CA,
  NUT_THEM_NGAY_LAM_BU,
  NUT_THEM_NGAY_NGHI,
  NUT_XAC_NHAN_XOA,
  NUT_XOA,
  O_GHI_CHU_CA,
  O_GIO_BAT_DAU,
  O_GIO_KET_THUC,
  O_LY_DO_XOA,
  O_NGAY,
  O_TEN_NGAY_LAM_BU,
  O_TEN_NGAY_NGHI,
  O_THU,
  cauGieoNgayLe,
  cauGieoThoiHan,
  cauGieoTuan,
  nhanLinhVuc,
  nhanLoaiViec,
  nhanSoGio,
} from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan } from "./quyen-tab";
import { COT_GIO, NHAN_COT, banTuDong, soanSua, type BanNhapGio } from "./sua-thoi-han";

/**
 * Tab "Thời hạn xử lý và lịch làm việc" — `docs/ui-ux/14-cau-hinh.md §8`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VIỆC SỐ MỘT CỦA MÀN NÀY KHÔNG PHẢI MỘT BẢNG ĐẸP. Nó là làm cho một xã mới biết mình đang thiếu
 * gì và bấm được hai nút.
 *
 * Chuỗi phía sau `POST /api/v1/incoming-documents` đi qua `identity.ResolveDeadlines`, và hàm ấy
 * từ chối khi bảng thời hạn rỗng, từ chối khi lịch làm việc rỗng. Một xã vừa nhận hệ thống vì thế
 * KHÔNG vào sổ được văn bản đến và KHÔNG nhận được phản ánh — và lỗi ấy hiện ra ở một màn hình
 * khác hẳn màn hình sửa được nó. Cả hai bảng đều đã có tuyến gieo idempotent; thứ còn thiếu đúng
 * là chỗ để người ta bấm. Khối `KhoiChuaKhai` là thứ thay cho một bước hướng dẫn ban đầu không
 * tồn tại, nên nó nói HẬU QUẢ chứ không nói "còn thiếu dữ liệu".
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ NÚT `+ Thêm thời hạn cho một lĩnh vực`, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH CHỨ KHÔNG PHẢI BỎ SÓT.
 * Đặc tả vẽ nút ấy ở `:293`, nhưng tuyến đứng sau nó cố ý chưa có: một đường ghi nhận mã lĩnh vực
 * từ client phải đối chiếu mã ấy với bộ mã tầng 1 ở `platform`, và đường đọc ấy chưa có ADR nào —
 * ADR 0026 điều kiện dừng #2. Vẽ một nút gọi vào tuyến không tồn tại là lời hứa suông: cán bộ bấm,
 * nhận lỗi, và kết luận hệ thống hỏng, trong khi thứ đang thiếu là một quyết định của khách.
 *
 * CỔNG QUYỀN BỌC PHẦN GHI, KHÔNG BỌC BẢNG — cùng khuôn với tab Danh mục và cùng lý do: ẩn cả bảng
 * là giao diện từ chối điều máy chủ đang phục vụ. Ba tuyến ĐỌC lịch là `any-authenticated`. Riêng
 * `GET /api/v1/sla` thì máy chủ ĐÒI `admin.sla` thật (`sla.go` nói vì sao: bảng này đọc để CẤU
 * HÌNH, không màn hình nghiệp vụ nào vẽ nó), nên tài khoản thiếu khoá nhận 403 ngay ở lượt đọc và
 * màn hình hiện NGUYÊN câu của máy chủ — không dựng thêm một cổng client đoán trước điều đó.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: mười một tuyến ghi khai `RequirePermission
 * ("admin.sla")` và kiểm trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO Ở ĐÂY. `identity` sở hữu bốn bảng và sở hữu phép cộng
 * (ADR 0007). Màn hình chỉ hiện những dòng máy chủ trả về và những câu máy chủ viết.
 */

/* ---- trạng thái ---------------------------------------------------------------------------- */

/** Biểu mẫu nào đang mở. MỘT biểu mẫu cho cả tab: bốn bản nháp cùng lúc là bốn bản gửi nhầm. */
export type DangMo =
  | { kieu: "suaThoiHan"; dong: identity_dongSLARa }
  | { kieu: "themCa" }
  | { kieu: "suaCa"; ca: identity_caLamViecRa }
  | { kieu: "xoaCa"; ca: identity_caLamViecRa }
  | { kieu: "themNghi" }
  | { kieu: "suaNghi"; ngay: identity_ngayNghiLeRa }
  | { kieu: "xoaNghi"; ngay: identity_ngayNghiLeRa }
  | { kieu: "themLamBu" }
  | { kieu: "suaLamBu"; ca: identity_caLamBuRa }
  | { kieu: "xoaLamBu"; ca: identity_caLamBuRa }
  | null;

/** Bản nháp đang gõ. Chuỗi hết — ô nhập của trình duyệt trả về chuỗi. */
export type BanNhap = {
  gio: BanNhapGio;
  thu: string;
  batDau: string;
  ketThuc: string;
  ghiChu: string;
  ngay: string;
  ten: string;
  lyDo: string;
};

const GIO_TRONG: BanNhapGio = {
  acknowledge_hours: "",
  resolve_hours: "",
  due_soon_hours: "",
  escalate_leader_hours: "",
  escalate_president_hours: "",
};

export const BAN_TRONG: BanNhap = {
  gio: GIO_TRONG,
  thu: "1",
  batDau: "",
  ketThuc: "",
  ghiChu: "",
  ngay: "",
  ten: "",
  lyDo: "",
};

/** Mười ba thao tác mà một dòng, một tiêu đề hoặc khối cảnh báo có thể yêu cầu. */
export type ThaoTacThoiHan = {
  readonly gieoThoiHan: () => void;
  readonly gieoTuan: () => void;
  readonly gieoNgayLe: () => void;
  readonly suaThoiHan: (d: identity_dongSLARa) => void;
  readonly themCa: () => void;
  readonly suaCa: (c: identity_caLamViecRa) => void;
  readonly xoaCa: (c: identity_caLamViecRa) => void;
  readonly themNghi: () => void;
  readonly suaNghi: (n: identity_ngayNghiLeRa) => void;
  readonly xoaNghi: (n: identity_ngayNghiLeRa) => void;
  readonly themLamBu: () => void;
  readonly suaLamBu: (c: identity_caLamBuRa) => void;
  readonly xoaLamBu: (c: identity_caLamBuRa) => void;
};

/** Bốn lượt đọc của tab. `null` là CHƯA đọc xong — khác hẳn "đọc xong và rỗng". */
export type DuLieuTab = {
  readonly thoiHan: KetQua<identity_danhSachSLARa> | null;
  readonly tuan: KetQua<identity_danhSachCaLamViecRa> | null;
  readonly nghi: KetQua<identity_danhSachNgayNghiLeRa> | null;
  readonly lamBu: KetQua<identity_danhSachCaLamBuRa> | null;
};

/* ---- vỏ đọc dữ liệu ------------------------------------------------------------------------ */

export function TabThoiHanXuLy() {
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);

  /** Tăng sau mỗi lần ghi thành công để ĐỌC LẠI từ máy chủ, không vá mảng tại chỗ. */
  const [lanDoc, datLanDoc] = useState(0);

  const [thoiHan, datThoiHan] = useState<KetQua<identity_danhSachSLARa> | null>(null);
  const [tuan, datTuan] = useState<KetQua<identity_danhSachCaLamViecRa> | null>(null);
  /**
   * Hai bảng theo năm giữ kết quả KÈM NĂM đã sinh ra nó; "đang tải" được SUY RA từ chỗ năm lưu
   * khác năm đang chọn. Đặt `null` trong thân effect để lại một cửa sổ, dù hẹp, ở đó bảng ngày
   * nghỉ của năm cũ đứng dưới một ô chọn đã hiện năm mới.
   */
  const [nghi, datNghi] = useState<{
    nam: number;
    kq: KetQua<identity_danhSachNgayNghiLeRa>;
  } | null>(null);
  const [lamBu, datLamBu] = useState<{ nam: number; kq: KetQua<identity_danhSachCaLamBuRa> } | null>(
    null,
  );

  const [dangMo, datDangMo] = useState<DangMo>(null);
  const [ban, datBan] = useState<BanNhap>(BAN_TRONG);
  const [loiTaiCho, datLoiTaiCho] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  const phien = usePhien();
  /**
   * BA TRẠNG THÁI, KHÔNG HAI: chưa đọc xong phiên thì chưa vẽ nút ghi nào. "Chưa biết" không được
   * hành xử như "có quyền", và cũng không được hành xử như "thiếu quyền" — một câu giải thích
   * thiếu quyền hiện ra trong lúc còn đang đọc là nói một điều chưa biết đúng hay sai.
   */
  const quyetDinhGhi = phien === null ? null : quyetDinhGhiThoiHan(phien);
  const coQuyenGhi = quyetDinhGhi !== null && quyetDinhGhi.hien;

  useEffect(() => {
    let bo = false;
    layThoiHanXuLy().then((kq) => {
      if (!bo) datThoiHan(kq);
    });
    layLichLamViec().then((kq) => {
      if (!bo) datTuan(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  useEffect(() => {
    let bo = false;
    layNgayNghiLe(nam).then((kq) => {
      if (!bo) datNghi({ nam, kq });
    });
    layNgayLamBu(nam).then((kq) => {
      if (!bo) datLamBu({ nam, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam, lanDoc]);

  const mo = useCallback((m: DangMo, banDau: BanNhap) => {
    datDangMo(m);
    datBan(banDau);
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const dong = useCallback(() => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datLoiTaiCho("");
    datLoiMayChu("");
  }, []);

  /**
   * Một lượt ghi: dọn thông báo cũ, gửi, rồi hoặc nói đã làm gì và ĐỌC LẠI, hoặc hiện NGUYÊN câu
   * máy chủ viết. Không rẽ nhánh theo `code`, không hiện `trace_id`, không hiện số hiệu HTTP.
   */
  const thucHien = useCallback(function <T>(goi: Promise<KetQua<T>>, cau: (d: T) => string) {
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong("");
    datDangGui(true);
    void goi.then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiMayChu(kq.thongBao);
        return;
      }
      datDangMo(null);
      datBan(BAN_TRONG);
      datCauDaXong(cau(kq.duLieu));
      datLanDoc((n) => n + 1);
    });
  }, []);

  const thaoTac: ThaoTacThoiHan = {
    gieoThoiHan: () =>
      thucHien(gieoThoiHanMacDinh(), (d) => cauGieoThoiHan(d.seeded, d.kept)),
    gieoTuan: () => thucHien(gieoTuanMacDinh(), (d) => cauGieoTuan(d.seeded, d.kept, d.skipped)),
    gieoNgayLe: () =>
      thucHien(gieoNgayNghiLeMacDinh(nam), (d) => cauGieoNgayLe(nam, d.seeded, d.kept, d.skipped)),
    suaThoiHan: (d) => mo({ kieu: "suaThoiHan", dong: d }, { ...BAN_TRONG, gio: banTuDong(d) }),
    themCa: () => mo({ kieu: "themCa" }, BAN_TRONG),
    suaCa: (c) =>
      mo(
        { kieu: "suaCa", ca: c },
        {
          ...BAN_TRONG,
          thu: String(c.weekday),
          batDau: nhanGio(c.start),
          ketThuc: nhanGio(c.end),
          ghiChu: c.note,
        },
      ),
    xoaCa: (c) => mo({ kieu: "xoaCa", ca: c }, BAN_TRONG),
    themNghi: () => mo({ kieu: "themNghi" }, BAN_TRONG),
    suaNghi: (n) =>
      mo({ kieu: "suaNghi", ngay: n }, { ...BAN_TRONG, ngay: n.date, ten: n.name }),
    xoaNghi: (n) => mo({ kieu: "xoaNghi", ngay: n }, BAN_TRONG),
    themLamBu: () => mo({ kieu: "themLamBu" }, BAN_TRONG),
    suaLamBu: (c) =>
      mo(
        { kieu: "suaLamBu", ca: c },
        {
          ...BAN_TRONG,
          ngay: c.date,
          batDau: nhanGio(c.start),
          ketThuc: nhanGio(c.end),
          ten: c.name,
        },
      ),
    xoaLamBu: (c) => mo({ kieu: "xoaLamBu", ca: c }, BAN_TRONG),
  };

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    // XOÁ: phép kiểm duy nhất màn hình tự làm ngoài phép kiểm số giờ — lý do rỗng thì không gửi.
    if (dangMo.kieu === "xoaCa" || dangMo.kieu === "xoaNghi" || dangMo.kieu === "xoaLamBu") {
      const lyDo = ban.lyDo.trim();
      if (lyDo === "") {
        datLoiTaiCho(LOI_THIEU_LY_DO);
        return;
      }
      if (dangMo.kieu === "xoaCa") thucHien(xoaCaLamViec(dangMo.ca.id, lyDo), () => DA_XOA_LICH);
      else if (dangMo.kieu === "xoaNghi")
        thucHien(xoaNgayNghiLe(dangMo.ngay.id, lyDo), () => DA_XOA_LICH);
      else thucHien(xoaNgayLamBu(dangMo.ca.id, lyDo), () => DA_XOA_LICH);
      return;
    }

    if (dangMo.kieu === "suaThoiHan") {
      const soan = soanSua(dangMo.dong, ban.gio);
      if (!soan.ok) {
        datLoiTaiCho(soan.loi);
        return;
      }
      thucHien(suaThoiHanXuLy(dangMo.dong.id, soan.than), () => DA_LUU_THOI_HAN);
      return;
    }

    // KHÔNG KIỂM KHUÔN GIỜ, THỨ HỢP LỆ, CA CHỒNG NHAU HAY NGÀY VỪA NGHỈ VỪA LÀM BÙ Ở ĐÂY. Máy chủ
    // kiểm cả bốn, mỗi thứ kèm một câu tiếng Việt nói rõ phải sửa gì; chép chúng xuống client là
    // dựng bản sao thứ hai của một bộ quy tắc nghiệp vụ, và bản sao ấy trôi mà không ai thấy.
    switch (dangMo.kieu) {
      case "themCa":
        thucHien(
          themCaLamViec({
            weekday: Number(ban.thu),
            start: ban.batDau,
            end: ban.ketThuc,
            note: ban.ghiChu,
          }),
          () => DA_LUU_LICH,
        );
        return;
      case "suaCa":
        thucHien(
          suaCaLamViec(dangMo.ca.id, {
            weekday: Number(ban.thu),
            start: ban.batDau,
            end: ban.ketThuc,
            note: ban.ghiChu,
          }),
          () => DA_LUU_LICH,
        );
        return;
      case "themNghi":
        thucHien(themNgayNghiLe({ date: ban.ngay, name: ban.ten }), () => DA_LUU_LICH);
        return;
      case "suaNghi":
        thucHien(
          suaNgayNghiLe(dangMo.ngay.id, { date: ban.ngay, name: ban.ten }),
          () => DA_LUU_LICH,
        );
        return;
      case "themLamBu":
        thucHien(
          themNgayLamBu({
            date: ban.ngay,
            start: ban.batDau,
            end: ban.ketThuc,
            name: ban.ten,
          }),
          () => DA_LUU_LICH,
        );
        return;
      default:
        thucHien(
          suaNgayLamBu(dangMo.ca.id, {
            date: ban.ngay,
            start: ban.batDau,
            end: ban.ketThuc,
            name: ban.ten,
          }),
          () => DA_LUU_LICH,
        );
    }
  }, [ban, dangGui, dangMo, thucHien]);

  return (
    <ManThoiHanXuLy
      du={{
        thoiHan,
        tuan,
        nghi: nghi !== null && nghi.nam === nam ? nghi.kq : null,
        lamBu: lamBu !== null && lamBu.nam === nam ? lamBu.kq : null,
      }}
      nam={nam}
      namGoc={namGoc}
      datNam={datNam}
      coQuyenGhi={coQuyenGhi}
      thieuQuyen={quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen"}
      thaoTac={thaoTac}
      cauDaXong={cauDaXong}
      form={
        dangMo === null ? null : (
          <BieuMauThoiHan
            dangMo={dangMo}
            ban={ban}
            datBan={datBan}
            loiTaiCho={loiTaiCho}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={guiBieuMau}
            onHuy={dong}
          />
        )
      }
      nhomForm={nhomCuaForm(dangMo)}
      loiMayChuNgoaiForm={dangMo === null ? loiMayChu : ""}
      dangGui={dangGui}
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

/**
 * Toàn bộ phần nhìn thấy được của tab, THUẦN TRÌNH BÀY.
 *
 * XUẤT RA để bài kiểm kết xuất được bằng `react-dom/server` mà không cần trình duyệt giả lập. Đó
 * không phải tiện lợi: lỗ hổng đã đo được ở tab Danh mục là mọi ca kiểm canh một QUYẾT ĐỊNH trong
 * module thuần, còn việc quyết định ấy có ra tới trang hay không thì không ca nào canh — bôi trắng
 * một câu quan trọng nhất màn hình vẫn xanh hết.
 */
export function ManThoiHanXuLy({
  du,
  nam,
  namGoc,
  datNam,
  coQuyenGhi,
  thieuQuyen,
  thaoTac,
  cauDaXong,
  form,
  nhomForm,
  loiMayChuNgoaiForm,
  dangGui,
}: {
  du: DuLieuTab;
  nam: number;
  namGoc: number;
  datNam: (n: number) => void;
  coQuyenGhi: boolean;
  thieuQuyen: boolean;
  thaoTac: ThaoTacThoiHan;
  cauDaXong: string;
  /** Biểu mẫu đang mở, hoặc `null`. Nó mở NGAY TRONG nhóm của nó, không phải một hộp thoại nổi. */
  form: ReactNode;
  /**
   * Nhóm nào sở hữu biểu mẫu ấy — truyền vào THÀNH MỘT GIÁ TRỊ, không đọc ngược từ `form`.
   *
   * Đọc `form.props` để đoán xem nó thuộc về ai là để giao diện phụ thuộc vào hình dạng bên trong
   * một phần tử React, thứ không có gì bảo đảm và không kiểu nào canh. `nhomCuaForm` là một hàm
   * thuần có bài kiểm, và bên gọi truyền xuống kết quả của nó.
   */
  nhomForm: NhomForm;
  /** Lỗi của một thao tác không mở biểu mẫu nào (ba nút gieo). */
  loiMayChuNgoaiForm: string;
  dangGui: boolean;
}) {
  const khoi = khoiCanhBao(tinhTrangBang(du.thoiHan), tinhTrangBang(du.tuan));

  return (
    <section className="tab-thoi-han" aria-labelledby="tieu-de-thoi-han">
      <h2 id="tieu-de-thoi-han">Thời hạn xử lý và lịch làm việc</h2>

      <KhoiChuaKhai khoi={khoi} coQuyenGhi={coQuyenGhi} dangGui={dangGui} thaoTac={thaoTac} />

      {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}
      {loiMayChuNgoaiForm !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChuNgoaiForm}
        </p>
      )}
      {thieuQuyen && (
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền sửa cấu hình thời hạn xử lý và lịch làm việc. Bảng dưới
          đây vẫn xem được.
        </p>
      )}

      <BangThoiHan
        kq={du.thoiHan}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        thaoTac={thaoTac}
        form={nhomForm === "thoiHan" ? form : null}
      />

      <h3>Lịch làm việc của đơn vị</h3>
      <p className="ghi-chu">{DAN_LICH_LAM_VIEC}</p>

      <BangGioLamViec
        kq={du.tuan}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        thaoTac={thaoTac}
        form={nhomForm === "tuan" ? form : null}
      />

      <ChonNam
        id="nam-lich-lam-viec"
        nhan="Năm của lịch nghỉ lễ và làm bù"
        nam={nam}
        namGoc={namGoc}
        datNam={datNam}
      />

      <BangNgayNghi
        kq={du.nghi}
        nam={nam}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        thaoTac={thaoTac}
        form={nhomForm === "nghi" ? form : null}
      />
      <BangNgayLamBu
        kq={du.lamBu}
        nam={nam}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        thaoTac={thaoTac}
        form={nhomForm === "lamBu" ? form : null}
      />
    </section>
  );
}

/**
 * Khối "đơn vị chưa khai xong" — phần đáng giá nhất của màn hình.
 *
 * NÓ NÓI HẬU QUẢ TRƯỚC, DANH SÁCH THIẾU SAU. Người mở màn hình này là cán bộ văn phòng hoặc chủ
 * tịch xã, mở đúng một lần khi mới nhận hệ thống. "Chưa có dữ liệu thời hạn" đọc ra là một việc để
 * hôm khác; "chưa nhận được văn bản đến và phản ánh" là một việc phải làm ngay.
 *
 * VẮNG MẶT KHI CẢ HAI BẢNG ĐÃ CÓ DÒNG, và vế phủ định ấy mới là vế chịu lực: một lời báo động
 * hiện cả ở xã đã cấu hình xong là một lời báo động người ta học cách bỏ qua, rồi bỏ qua nốt lần
 * nó đúng.
 */
export function KhoiChuaKhai({
  khoi,
  coQuyenGhi,
  dangGui,
  thaoTac,
}: {
  khoi: KhoiCanhBao;
  coQuyenGhi: boolean;
  dangGui: boolean;
  thaoTac: ThaoTacThoiHan;
}) {
  if (!khoi.hien) return null;

  return (
    <div className="khoi-chua-khai" role="alert" aria-labelledby="tieu-de-chua-khai">
      <h3 id="tieu-de-chua-khai">{KHOI_CHUA_KHAI_TIEU_DE}</h3>
      <p className="hau-qua">{KHOI_CHUA_KHAI_HAU_QUA}</p>
      <ul>
        {khoi.thieuThoiHan && <li>{KHOI_CHUA_KHAI_THIEU_THOI_HAN}</li>}
        {khoi.thieuLichTuan && <li>{KHOI_CHUA_KHAI_THIEU_LICH_TUAN}</li>}
      </ul>

      {/* HAI NÚT, ĐÚNG HAI BẢNG ĐANG THIẾU. Nút cho một bảng đã đầy là một nút không làm gì và
          làm người đọc nghi ngờ cả khối. Thiếu quyền thì không vẽ nút nào — nhưng câu hậu quả
          VẪN hiện, vì người không có quyền cũng cần biết vì sao xã chưa tiếp nhận được, để đi tìm
          đúng người. */}
      {coQuyenGhi && (
        <p className="cum-nut">
          {khoi.thieuThoiHan && (
            <button
              type="button"
              className="nut-chinh"
              disabled={dangGui}
              onClick={thaoTac.gieoThoiHan}
            >
              {NUT_GIEO_THOI_HAN}
            </button>
          )}
          {khoi.thieuLichTuan && (
            <button type="button" className="nut-chinh" disabled={dangGui} onClick={thaoTac.gieoTuan}>
              {NUT_GIEO_TUAN}
            </button>
          )}
        </p>
      )}
      <p className="ghi-chu">{GHI_CHU_SAU_KHI_GIEO}</p>
    </div>
  );
}

/** Khung "đang tải" / "lỗi" dùng chung cho bốn bảng. */
function KhungTai({ kq, dangTai }: { kq: KetQua<unknown> | null; dangTai: string }) {
  if (kq === null) return <p role="status">{dangTai}</p>;
  if (kq.ok) return null;
  // NGUYÊN VĂN câu máy chủ viết — kể cả 403 thiếu quyền của `GET /api/v1/sla`.
  return (
    <p className="thong-bao-loi" role="alert">
      {kq.thongBao}
    </p>
  );
}

/**
 * Bảng thời hạn xử lý.
 *
 * KHÔNG CÓ NÚT THÊM DÒNG — xem khối chú thích đầu tệp (ADR 0026 điều kiện dừng #2). Có nút SỬA,
 * vì `PATCH` chỉ nhận năm con số và không nhận mã lĩnh vực nào.
 */
export function BangThoiHan({
  kq,
  coQuyenGhi,
  dangGui,
  thaoTac,
  form,
}: {
  kq: KetQua<identity_danhSachSLARa> | null;
  coQuyenGhi: boolean;
  dangGui: boolean;
  thaoTac: ThaoTacThoiHan;
  form: ReactNode;
}) {
  return (
    <div className="nhom-lich">
      <h3>Thời hạn xử lý</h3>
      {/* HAI CÂU DẪN BẮT BUỘC GIỮ của đặc tả. Câu đầu là điều dễ hiểu sai nhất trên màn hình. */}
      <p className="canh-bao-pham-vi">{DAN_THOI_HAN_1}</p>
      <p className="ghi-chu">{DAN_THOI_HAN_2}</p>
      <p className="ghi-chu">{GHI_CHU_HAI_COT_LEO_THANG}</p>

      <KhungTai kq={kq} dangTai="Đang tải bảng thời hạn xử lý…" />

      {kq !== null && kq.ok && (
        <>
          {/* SAI SÓT CỦA CHÍNH DỮ LIỆU XÃ, máy chủ suy ra từ đúng những dòng nó vừa trả về. Tuyến
              vẫn trả 200 có chủ ý: đây là màn hình SỬA nó, nên phải mở được kể cả khi đang hỏng. */}
          {kq.duLieu.problems.map((v, i) => (
            <p className="thong-bao-loi" role="alert" key={`${v.kind}-${v.work_kind}-${i}`}>
              {v.message}
            </p>
          ))}

          {/* Nút gieo ở ĐÂY chỉ dành cho bảng ĐÃ CÓ dòng (vá lại bộ thiếu). Bảng rỗng thì nút nằm
              trong khối cảnh báo phía trên — một việc, một nút, không hai chỗ cùng lúc. */}
          {coQuyenGhi && kq.duLieu.items.length > 0 && (
            <p>
              <button
                type="button"
                className="nut-phu"
                disabled={dangGui}
                onClick={thaoTac.gieoThoiHan}
              >
                {NUT_GIEO_THOI_HAN}
              </button>
            </p>
          )}

          {kq.duLieu.items.length > 0 && (
            <div className="bang-cuon" role="region" aria-label="Thời hạn xử lý" tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Số giờ làm việc cho từng loại việc và lĩnh vực của đơn vị
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Loại việc</th>
                    <th scope="col">Lĩnh vực</th>
                    {COT_GIO.map((c) => (
                      <th scope="col" key={c}>
                        {NHAN_COT[c]}
                      </th>
                    ))}
                    {coQuyenGhi && (
                      <th scope="col">
                        <span className="an-thi-giac">Thao tác</span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {/* GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ VỀ. Sắp lại theo tên lĩnh vực sẽ tách dòng mặc
                      định khỏi nhóm của nó, mà dòng mặc định là dòng mọi lĩnh vực không có dòng
                      riêng rơi về. */}
                  {kq.duLieu.items.map((d) => (
                    <tr key={d.id}>
                      <td>{nhanLoaiViec(d.work_kind)}</td>
                      <td className={d.is_default ? undefined : "ma-muc"}>
                        {nhanLinhVuc(d.field, d.is_default)}
                      </td>
                      {COT_GIO.map((c) => (
                        <td key={c}>{nhanSoGio(d[c])}</td>
                      ))}
                      {coQuyenGhi && (
                        <td className="o-thao-tac">
                          <button
                            type="button"
                            className="nut-phu"
                            aria-label={`${NUT_SUA} thời hạn ${nhanLoaiViec(d.work_kind)} — ${nhanLinhVuc(d.field, d.is_default)}`}
                            onClick={() => thaoTac.suaThoiHan(d)}
                          >
                            {NUT_SUA}
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {form}
    </div>
  );
}

export function BangGioLamViec({
  kq,
  coQuyenGhi,
  dangGui,
  thaoTac,
  form,
}: {
  kq: KetQua<identity_danhSachCaLamViecRa> | null;
  coQuyenGhi: boolean;
  dangGui: boolean;
  thaoTac: ThaoTacThoiHan;
  form: ReactNode;
}) {
  return (
    <div className="nhom-lich">
      <h3>Giờ làm việc trong tuần</h3>
      <p className="ghi-chu">{GIAI_THICH_CA}</p>

      <KhungTai kq={kq} dangTai="Đang tải giờ làm việc…" />

      {kq !== null && kq.ok && (
        <>
          {kq.duLieu.problems.map((v, i) => (
            <p className="thong-bao-loi" role="alert" key={`${v.kind}-${v.weekday ?? "chung"}-${i}`}>
              {v.message}
            </p>
          ))}

          {coQuyenGhi && (
            <p className="cum-nut">
              <button type="button" className="nut-phu" disabled={dangGui} onClick={thaoTac.themCa}>
                {NUT_THEM_CA}
              </button>
              {kq.duLieu.items.length > 0 && (
                <button
                  type="button"
                  className="nut-phu"
                  disabled={dangGui}
                  onClick={thaoTac.gieoTuan}
                >
                  {NUT_GIEO_TUAN}
                </button>
              )}
            </p>
          )}

          {kq.duLieu.items.length > 0 && (
            <div className="bang-cuon" role="region" aria-label="Giờ làm việc trong tuần" tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Các ca làm việc thông thường của đơn vị theo từng thứ trong tuần
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Thứ</th>
                    <th scope="col">Ca làm việc</th>
                    <th scope="col">Ghi chú</th>
                    {coQuyenGhi && (
                      <th scope="col">
                        <span className="an-thi-giac">Thao tác</span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {kq.duLieu.items.map((c) => (
                    <tr key={c.id}>
                      <td>{tenThu(c.weekday)}</td>
                      <td>{nhanCa(c.start, c.end)}</td>
                      <td>{c.note === "" ? <span className="nhan-trong">—</span> : c.note}</td>
                      {coQuyenGhi && (
                        <td className="o-thao-tac">
                          <span className="cum-nut">
                            <button
                              type="button"
                              className="nut-phu"
                              aria-label={`${NUT_SUA} ca ${tenThu(c.weekday)} ${nhanCa(c.start, c.end)}`}
                              onClick={() => thaoTac.suaCa(c)}
                            >
                              {NUT_SUA}
                            </button>
                            <button
                              type="button"
                              className="nut-phu nut-xoa"
                              aria-label={`${NUT_XOA} ca ${tenThu(c.weekday)} ${nhanCa(c.start, c.end)}`}
                              onClick={() => thaoTac.xoaCa(c)}
                            >
                              {NUT_XOA}
                            </button>
                          </span>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {form}
    </div>
  );
}

export function BangNgayNghi({
  kq,
  nam,
  coQuyenGhi,
  dangGui,
  thaoTac,
  form,
}: {
  kq: KetQua<identity_danhSachNgayNghiLeRa> | null;
  nam: number;
  coQuyenGhi: boolean;
  dangGui: boolean;
  thaoTac: ThaoTacThoiHan;
  form: ReactNode;
}) {
  return (
    <div className="nhom-lich">
      <h3>Ngày nghỉ lễ — đơn vị KHÔNG làm việc</h3>

      <KhungTai kq={kq} dangTai="Đang tải ngày nghỉ lễ…" />

      {kq !== null && kq.ok && (
        <>
          {coQuyenGhi && (
            <p className="cum-nut">
              <button
                type="button"
                className="nut-phu"
                disabled={dangGui}
                onClick={thaoTac.themNghi}
              >
                {NUT_THEM_NGAY_NGHI}
              </button>
              <button
                type="button"
                className="nut-phu"
                disabled={dangGui}
                onClick={thaoTac.gieoNgayLe}
              >
                {NUT_GIEO_NGAY_LE}
              </button>
            </p>
          )}

          {/* ⚠ CÂU NỢ NGƯỜI BẤM NÚT GIEO, đứng NGAY CẠNH nút và cạnh chỗ kết quả gieo hiện ra. Tuyến
              chỉ gieo BỐN ngày cố định theo dương lịch và `seeded: 4` đọc ra là "xong" — trong khi
              Tết Nguyên đán, Giỗ Tổ Hùng Vương và ngày liền kề 02/9 vẫn còn thiếu. Không nói ra thì
              xã tưởng đã đủ, và mọi hạn rơi vào dịp Tết bị tính sai mà không gì báo lỗi. */}
          <p className="canh-bao-pham-vi">{CON_THIEU_NGAY_LE}</p>

          {kq.duLieu.items.length === 0 ? (
            <p className="trang-thai-rong">
              Năm {nam} chưa khai ngày nghỉ lễ nào, nên mọi thời hạn của năm này đang được đếm như
              thể đơn vị không nghỉ ngày nào.
            </p>
          ) : (
            <div className="bang-cuon" role="region" aria-label={`Ngày nghỉ lễ năm ${nam}`} tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Những ngày đơn vị đóng cửa trong năm {nam}, gồm cả lễ quốc gia lẫn lễ địa phương
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Ngày</th>
                    <th scope="col">Tên</th>
                    {coQuyenGhi && (
                      <th scope="col">
                        <span className="an-thi-giac">Thao tác</span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {kq.duLieu.items.map((n) => (
                    <tr key={n.id}>
                      <td>{nhanNgay(n.date)}</td>
                      <td>{n.name}</td>
                      {coQuyenGhi && (
                        <td className="o-thao-tac">
                          <span className="cum-nut">
                            <button
                              type="button"
                              className="nut-phu"
                              aria-label={`${NUT_SUA} ngày nghỉ ${nhanNgay(n.date)}`}
                              onClick={() => thaoTac.suaNghi(n)}
                            >
                              {NUT_SUA}
                            </button>
                            <button
                              type="button"
                              className="nut-phu nut-xoa"
                              aria-label={`${NUT_XOA} ngày nghỉ ${nhanNgay(n.date)}`}
                              onClick={() => thaoTac.xoaNghi(n)}
                            >
                              {NUT_XOA}
                            </button>
                          </span>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {form}
    </div>
  );
}

export function BangNgayLamBu({
  kq,
  nam,
  coQuyenGhi,
  dangGui,
  thaoTac,
  form,
}: {
  kq: KetQua<identity_danhSachCaLamBuRa> | null;
  nam: number;
  coQuyenGhi: boolean;
  dangGui: boolean;
  thaoTac: ThaoTacThoiHan;
  form: ReactNode;
}) {
  return (
    <div className="nhom-lich">
      <h3>Ngày làm bù — đơn vị CÓ làm việc</h3>

      <KhungTai kq={kq} dangTai="Đang tải ngày làm bù…" />

      {kq !== null && kq.ok && (
        <>
          {kq.duLieu.problems.map((v, i) => (
            <p className="thong-bao-loi" role="alert" key={`${v.kind}-${v.date}-${i}`}>
              {v.message}
            </p>
          ))}

          {/* KHÔNG CÓ NÚT GIEO Ở ĐÂY, và đó là câu trả lời chứ không phải thiếu sót: một ngày làm bù
              chỉ tồn tại vì Thủ tướng công bố cho riêng một năm, nên không có bộ mặc định nào để
              gieo. Phần lớn năm, phần lớn xã không có ngày làm bù nào. */}
          {coQuyenGhi && (
            <p>
              <button
                type="button"
                className="nut-phu"
                disabled={dangGui}
                onClick={thaoTac.themLamBu}
              >
                {NUT_THEM_NGAY_LAM_BU}
              </button>
            </p>
          )}

          {kq.duLieu.items.length === 0 ? (
            <p className="trang-thai-rong">
              Năm {nam} không có ngày làm bù nào. Phần lớn các năm là như vậy — khác hẳn giờ làm
              việc trong tuần để trống, vốn nghĩa là đơn vị không có giờ làm việc nào.
            </p>
          ) : (
            <div className="bang-cuon" role="region" aria-label={`Ngày làm bù năm ${nam}`} tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Những ngày đơn vị vẫn làm việc trong năm {nam} dù lịch tuần nói không, kèm giờ làm
                  của chính ngày đó
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Ngày</th>
                    <th scope="col">Ca làm việc</th>
                    <th scope="col">Theo thông báo</th>
                    {coQuyenGhi && (
                      <th scope="col">
                        <span className="an-thi-giac">Thao tác</span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {kq.duLieu.items.map((c) => (
                    <tr key={c.id}>
                      <td>{nhanNgay(c.date)}</td>
                      <td>{nhanCa(c.start, c.end)}</td>
                      <td>{c.name}</td>
                      {coQuyenGhi && (
                        <td className="o-thao-tac">
                          <span className="cum-nut">
                            <button
                              type="button"
                              className="nut-phu"
                              aria-label={`${NUT_SUA} ca làm bù ${nhanNgay(c.date)}`}
                              onClick={() => thaoTac.suaLamBu(c)}
                            >
                              {NUT_SUA}
                            </button>
                            <button
                              type="button"
                              className="nut-phu nut-xoa"
                              aria-label={`${NUT_XOA} ca làm bù ${nhanNgay(c.date)}`}
                              onClick={() => thaoTac.xoaLamBu(c)}
                            >
                              {NUT_XOA}
                            </button>
                          </span>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {form}
    </div>
  );
}

/** Bốn nhóm bảng trên tab. Đây là cách MÀN HÌNH gom nhóm, không phải dữ liệu của hợp đồng. */
export type NhomForm = "thoiHan" | "tuan" | "nghi" | "lamBu" | null;

/** Biểu mẫu đang mở thuộc nhóm nào — hàm thuần, để một `switch` không nằm giữa hai thẻ JSX. */
export function nhomCuaForm(dangMo: DangMo): NhomForm {
  if (dangMo === null) return null;
  switch (dangMo.kieu) {
    case "suaThoiHan":
      return "thoiHan";
    case "themCa":
    case "suaCa":
    case "xoaCa":
      return "tuan";
    case "themNghi":
    case "suaNghi":
    case "xoaNghi":
      return "nghi";
    default:
      return "lamBu";
  }
}

/* ---- biểu mẫu ------------------------------------------------------------------------------ */

const TIEU_DE: Record<NonNullable<DangMo>["kieu"], string> = {
  suaThoiHan: "Sửa thời hạn xử lý",
  themCa: "Thêm ca làm việc",
  suaCa: "Sửa ca làm việc",
  xoaCa: "Xoá ca làm việc",
  themNghi: "Thêm ngày nghỉ lễ",
  suaNghi: "Sửa ngày nghỉ lễ",
  xoaNghi: "Xoá ngày nghỉ lễ",
  themLamBu: "Thêm ca làm bù",
  suaLamBu: "Sửa ca làm bù",
  xoaLamBu: "Xoá ca làm bù",
};

/**
 * Một biểu mẫu cho cả mười thao tác.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị đi vào qua `ban`, mọi thay đổi đi ra qua `datBan`, phép kiểm nằm ở
 * chỗ gọi. Tách như vậy để hai nhánh KHÔNG ai nhìn thấy trong lúc phát triển — "lý do xoá còn
 * trống" và "máy chủ vừa từ chối" — kết xuất được bằng `react-dom/server`.
 */
export function BieuMauThoiHan({
  dangMo,
  ban,
  datBan,
  loiTaiCho,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: NonNullable<DangMo>;
  ban: BanNhap;
  datBan: (b: BanNhap) => void;
  loiTaiCho: string;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const laXoa = dangMo.kieu === "xoaCa" || dangMo.kieu === "xoaNghi" || dangMo.kieu === "xoaLamBu";
  const tieuDe = TIEU_DE[dangMo.kieu];

  return (
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>

      {dangMo.kieu === "suaThoiHan" && (
        <>
          <p className="canh-bao-pham-vi">{DAN_THOI_HAN_1}</p>
          {COT_GIO.map((c) => (
            <div className="o-nhap" key={c}>
              <label htmlFor={`o-gio-${c}`}>{NHAN_COT[c]} (giờ làm việc)</label>
              {/* `inputMode="numeric"` chứ không `type="number"`: ô số của trình duyệt có nút tăng
                  giảm bé xíu và cuộn chuột đổi giá trị mà người dùng không biết. */}
              <input
                id={`o-gio-${c}`}
                name={c}
                inputMode="numeric"
                value={ban.gio[c]}
                onChange={(e) => datBan({ ...ban, gio: { ...ban.gio, [c]: e.target.value } })}
              />
            </div>
          ))}
        </>
      )}

      {(dangMo.kieu === "themCa" || dangMo.kieu === "suaCa") && (
        <>
          <div className="o-nhap">
            <label htmlFor="o-thu">{O_THU}</label>
            {/* Ô CHỌN chứ không ô gõ số: hợp đồng đếm thứ theo ISO 8601 (1 = thứ Hai … 7 = Chủ
                nhật), khác `Date.getDay()` của JavaScript. Một ô gõ số mời người dùng gõ "2" cho
                thứ Hai, và lệch một ngày thì không có gì báo lỗi. */}
            <select
              id="o-thu"
              name="thu"
              value={ban.thu}
              onChange={(e) => datBan({ ...ban, thu: e.target.value })}
            >
              {[1, 2, 3, 4, 5, 6, 7].map((t) => (
                <option key={t} value={t}>
                  {tenThu(t)}
                </option>
              ))}
            </select>
          </div>
          <OGio ban={ban} datBan={datBan} />
          <div className="o-nhap">
            <label htmlFor="o-ghi-chu-ca">{O_GHI_CHU_CA}</label>
            <input
              id="o-ghi-chu-ca"
              name="ghiChu"
              value={ban.ghiChu}
              onChange={(e) => datBan({ ...ban, ghiChu: e.target.value })}
            />
          </div>
        </>
      )}

      {(dangMo.kieu === "themNghi" || dangMo.kieu === "suaNghi") && (
        <>
          <ONgay ban={ban} datBan={datBan} />
          <div className="o-nhap">
            <label htmlFor="o-ten-lich">{O_TEN_NGAY_NGHI}</label>
            <input
              id="o-ten-lich"
              name="ten"
              value={ban.ten}
              onChange={(e) => datBan({ ...ban, ten: e.target.value })}
            />
          </div>
        </>
      )}

      {(dangMo.kieu === "themLamBu" || dangMo.kieu === "suaLamBu") && (
        <>
          <ONgay ban={ban} datBan={datBan} />
          <OGio ban={ban} datBan={datBan} />
          <div className="o-nhap">
            <label htmlFor="o-ten-lich">{O_TEN_NGAY_LAM_BU}</label>
            <input
              id="o-ten-lich"
              name="ten"
              value={ban.ten}
              onChange={(e) => datBan({ ...ban, ten: e.target.value })}
              aria-describedby="giai-thich-ten-lam-bu"
            />
            <p className="ghi-chu" id="giai-thich-ten-lam-bu">
              Ghi rõ thông báo mà ngày này thực hiện, ví dụ “Làm bù nghỉ Tết theo Thông báo số …”.
              Chính câu đó là câu trả lời khi có người hỏi vì sao một hạn chạy qua ngày thứ Bảy.
            </p>
          </div>
        </>
      )}

      {laXoa && (
        <>
          <p className="canh-bao-pham-vi">
            {dangMo.kieu === "xoaCa"
              ? CANH_BAO_XOA_CA
              : dangMo.kieu === "xoaNghi"
                ? CANH_BAO_XOA_NGAY_NGHI
                : CANH_BAO_XOA_NGAY_LAM_BU}
          </p>
          <div className="o-nhap">
            <label htmlFor="o-ly-do-xoa-lich">{O_LY_DO_XOA}</label>
            {/* `required` là lớp nhắc của trình duyệt, KHÔNG phải phép kiểm: nó không bắt được một
                ô toàn dấu cách, và tắt được. Phép kiểm thật chạy trước khi gửi. */}
            <input
              id="o-ly-do-xoa-lich"
              name="lyDo"
              required
              value={ban.lyDo}
              onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
              aria-invalid={loiTaiCho !== ""}
              aria-describedby="giai-thich-ly-do-xoa"
            />
            <p className="ghi-chu" id="giai-thich-ly-do-xoa">
              {GIAI_THICH_LY_DO_XOA}
            </p>
          </div>
        </>
      )}

      {/* HAI VÙNG LỖI RIÊNG, KHÔNG GỘP. Lỗi tại chỗ nói "bạn còn thiếu một ô"; lỗi máy chủ nói "yêu
          cầu vừa rồi bị từ chối" — trong đó có câu 409 giải thích vì sao một giờ mở ca đã xoá vẫn
          chắn đường. Gộp chúng vào một dòng thì câu sau đè mất câu trước ở đúng lúc cần đọc cả hai. */}
      {loiTaiCho !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiTaiCho}
        </p>
      )}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {laXoa ? NUT_XAC_NHAN_XOA : NUT_LUU}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}

/**
 * Hai ô giờ.
 *
 * `type="time"` CHỨ KHÔNG PHẢI Ô CHỮ TỰ DO: máy chủ đòi đúng `HH:MM` hoặc `HH:MM:SS`, hai chữ số
 * mỗi phần, và từ chối "7:3" chứ không đọc thành 07:03. Một ô chữ mời cán bộ gõ "7h30" rồi nhận
 * một lời từ chối không nói được là phải gõ ra sao.
 */
function OGio({ ban, datBan }: { ban: BanNhap; datBan: (b: BanNhap) => void }) {
  return (
    <>
      <div className="o-nhap">
        <label htmlFor="o-bat-dau">{O_GIO_BAT_DAU}</label>
        <input
          id="o-bat-dau"
          name="batDau"
          type="time"
          value={ban.batDau}
          onChange={(e) => datBan({ ...ban, batDau: e.target.value })}
        />
      </div>
      <div className="o-nhap">
        <label htmlFor="o-ket-thuc">{O_GIO_KET_THUC}</label>
        <input
          id="o-ket-thuc"
          name="ketThuc"
          type="time"
          value={ban.ketThuc}
          onChange={(e) => datBan({ ...ban, ketThuc: e.target.value })}
        />
      </div>
    </>
  );
}

/**
 * Ô ngày.
 *
 * `type="date"` phát ra đúng `YYYY-MM-DD` — đúng hình dạng hợp đồng đòi. Một ô chữ tự do mời gõ
 * "2/9/2026", và một ngày nghỉ lệch là một hạn đếm xuyên qua ngày trụ sở đóng cửa.
 */
function ONgay({ ban, datBan }: { ban: BanNhap; datBan: (b: BanNhap) => void }) {
  return (
    <div className="o-nhap">
      <label htmlFor="o-ngay-lich">{O_NGAY}</label>
      <input
        id="o-ngay-lich"
        name="ngay"
        type="date"
        value={ban.ngay}
        onChange={(e) => datBan({ ...ban, ngay: e.target.value })}
      />
    </div>
  );
}
