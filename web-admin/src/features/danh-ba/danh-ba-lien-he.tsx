"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

import { BieuMauGhiCanBo, type MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import {
  BAN_TRONG,
  banTuCanBo,
  daLuuHoSo,
  thanSua,
  type BanNhapCanBo,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { bangTraTuKetQua, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { datCongKhaiCanBo, docTrangDanhBa, suaCanBo, xoaCanBo } from "@/lib/api/can-bo";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type {
  identity_canBoTomTat,
  identity_danhSachBoPhanRa,
  page_Result_identity_canBoTomTat,
} from "@/lib/api/schema.gen";
import type { KetQua } from "@/lib/api/goi";

import { BangLienHe } from "./bang-lien-he";
import {
  banCongKhaiTu,
  daCongKhai,
  daRut,
  duocCongKhaiTheoPhien,
  yeuCauCongKhai,
  yeuCauRut,
  type BanCongKhai,
} from "./cong-khai";
import { HopCongKhai, type DangMoCongKhai } from "./hop-cong-khai";
import { HopXoa } from "./hop-xoa";
import { daXoa, duocXoaTheoPhien, yeuCauXoa } from "./xoa-dong";
import {
  GOI_Y_O_TIM,
  LUA_CHON_HIEN_THI,
  NHAN_LOC_HIEN_THI,
  NHAN_LOC_KHOI,
  NHAN_O_TIM,
  NUT_TIM,
  TAT_CA_KHOI,
  THU_TU_HIEN_THI,
  TRUY_VAN_DAU,
  apLoc,
  dangLoc,
  ketQuaGuiTim,
  maBoPhanLoc,
  maHienThi,
  thamSoDoc,
  type LocDanhBa,
  type TruyVanDanhBa,
} from "./loc-danh-ba";
import {
  DANH_BA_RONG,
  GHI_CHU_SO_DIEN_THOAI,
  KHONG_KHOP_LOC,
  PHAN_CHUA_DUNG,
  NHAN_SO_KHOI,
  TIEU_DE_PHAN_CHUA_DUNG,
  demSoKhoi,
  nhanSoKhoi,
} from "./nhan-danh-ba";

/**
 * Màn **Danh bạ cán bộ** — `docs/ui-ux/12-danh-ba-can-bo.md`, đường dẫn `/danh-ba`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KHÔNG TỆP NÀO Ở ĐÂY DỰNG LẠI MỘT LỜI GỌI ĐÃ CÓ. Các tuyến ghi và đọc của danh bạ đã có chủ ở
 * `lib/api/can-bo.ts`; biểu mẫu sửa, câu chữ và phép đổi hình dạng bản nháp đã có chủ ở
 * `components/danh-ba/`. Màn này chỉ thêm đúng thứ nó sở hữu: bố cục của một trang riêng, thẻ KPI
 * đếm được, hàng lọc (`loc-danh-ba.ts`), hộp công khai Mini App (`cong-khai.ts`), và danh sách nói
 * rõ phần nào chưa mở.
 *
 * BA THAO TÁC GHI THUỘC VỀ MÀN DANH BẠ: `PATCH /api/v1/staff/{id}` (sửa chức vụ, khối/đơn vị, số
 * liên hệ, Có Zalo), `PUT .../publication` (công khai MỘT người lên Mini App, #12, `content.update`)
 * và `DELETE /api/v1/staff/{id}` (xoá một dòng NHẬP TRÙNG, #10, `admin.user.delete`). Bốn tuyến còn
 * lại đổi THẨM QUYỀN hoặc đường đăng nhập của một người — thêm, đổi vai trò, khoá, mở khoá — và
 * chúng ở lại đúng chỗ đặc tả §1 đặt chúng: tab `Cấu hình → Người dùng`. Bày cùng một nút Khoá tài
 * khoản ở hai màn hình là hai chỗ để một thao tác có hậu quả nặng bị bấm nhầm.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG CỦA CẢ MÀN nằm ở `app/danh-ba/page.tsx` (`CongQuyen` + `admin.user`). Hai nút Mini App cần
 * THÊM `content.update`, nút 🗑 cần THÊM `admin.user.delete`, và mỗi nút ẩn theo đúng khoá của nó.
 * Mọi lớp ẩn đều chỉ là tiện dụng: lớp chặn THẬT ở máy chủ, trên TỪNG yêu cầu (luật 5, cấm #1).
 */

/** Trạng thái một lần đọc danh sách. Ba nhánh rời nhau. */
type TrangThaiTrang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; trang: page_Result_identity_canBoTomTat };

export function DanhBaLienHe() {
  /** Bộ lọc đang áp + ngăn xếp con trỏ — MỘT state, để đổi lọc không thể quên về trang đầu. */
  const [truyVan, datTruyVan] = useState<TruyVanDanhBa>(TRUY_VAN_DAU);
  const [trangThai, datTrangThai] = useState<TrangThaiTrang>({ pha: "dangTai" });
  /** `null` là chưa đọc xong danh mục bộ phận — KHÔNG phải "xã không có bộ phận nào". */
  const [boPhan, datBoPhan] = useState<KetQua<identity_danhSachBoPhanRa> | null>(null);

  const [dangSua, datDangSua] = useState<identity_canBoTomTat | null>(null);
  const [ban, datBan] = useState<BanNhapCanBo>(BAN_TRONG);
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");
  /** Tăng sau mỗi lần ghi thành công — buộc đọc lại trang đang xem. Xem `ghiXong`. */
  const [lanDoc, datLanDoc] = useState(0);

  /** Hộp công khai / rút Mini App đang mở. Không bao giờ mở cùng lúc với biểu mẫu sửa. */
  const [dangMoCK, datDangMoCK] = useState<DangMoCongKhai | null>(null);
  const [banCK, datBanCK] = useState<BanCongKhai>({ daHoiY: false, thuTu: "" });

  /**
   * Phiên có `content.update` hay không — quyết định có vẽ hai nút Mini App. Phiên CHƯA ĐỌC XONG
   * hay đọc hỏng thì coi như KHÔNG (fail closed, `quyetDinhTheoKhoa`).
   */
  const phien = usePhien();
  const duocCongKhai = duocCongKhaiTheoPhien(phien);

  /** Hộp xoá dòng nhập trùng đang mở, và lý do đang gõ. Ba hộp không bao giờ mở cùng lúc. */
  const [dangXoa, datDangXoa] = useState<identity_canBoTomTat | null>(null);
  const [lyDoXoa, datLyDoXoa] = useState("");
  /** Phiên có `admin.user.delete` — quyết định có vẽ nút 🗑. Chưa đọc xong / hỏng → không. */
  const duocXoa = duocXoaTheoPhien(phien);

  /**
   * MỘT DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT KHI MỞ MÀN HÌNH — `[]` ở cuối effect là phần quan trọng nhất
   * của khối này.
   *
   * Không đọc lại khi đổi trang hay khi mở biểu mẫu: danh mục bộ phận của một xã không đổi giữa
   * hai lần bấm "Trang sau". Và tuyệt đối không đọc theo từng dòng — mỗi trang hai mươi dòng, nên
   * một lời gọi mỗi dòng là hai mươi lời gọi thay vì một, và con số ấy đi lên theo dữ liệu chứ
   * không đứng yên (`skills/load-data-once`, dạng 1).
   *
   * CHỈ ĐỌC BỘ PHẬN, KHÔNG ĐỌC VAI TRÒ. Màn này không có cột Vai trò và không có biểu mẫu đổi vai
   * trò, nên `GET /api/v1/roles` là một lời gọi không ai dùng kết quả.
   *
   * VÌ SAO ĐỌC Ở TRÌNH DUYỆT CHỨ KHÔNG Ở MÁY CHỦ: tuyến này đòi đã đăng nhập, nên gọi phía máy chủ
   * thì phải tự chuyển tiếp cookie phiên — thêm một chỗ cầm cookie, và là đúng chỗ dễ chuyển tiếp
   * sang sai host. Ở đây đường dẫn tương đối trên chính host của xã, trình duyệt tự gửi cookie
   * host-only (`lib/api/goi.ts`).
   */
  useEffect(() => {
    let bo = false;
    layDanhMucBoPhan().then((kq) => {
      if (!bo) datBoPhan(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const traBoPhan = useMemo<BangTraDanhMuc>(() => bangTraTuKetQua(boPhan), [boPhan]);
  const soKhoi = useMemo(() => demSoKhoi(boPhan), [boPhan]);

  /** Mục cho ô chọn của biểu mẫu sửa — lấy từ CHÍNH danh mục đã đọc, không đọc lại. */
  const mucBoPhan = useMemo<readonly MucChon[]>(
    () => (boPhan !== null && boPhan.ok ? boPhan.duLieu.items : []),
    [boPhan],
  );

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau. Không có nó thì
    // bấm "Trang sau" hai lần nhanh có thể để lại trên màn hình đúng trang vừa rời khỏi.
    let bo = false;

    // KHÔNG TRUYỀN `limit`, `sort`, `order`: để máy chủ áp mặc định của chính nó (20 dòng, sắp
    // theo mã, tăng dần). Giữ một bản sao của ba mặc định ấy ở client là giữ một bản sẽ trôi.
    // Có chữ tìm thì `docTrangDanhBa` đi đường POST, chữ và con trỏ nằm trong thân.
    docTrangDanhBa(truyVan.loc.tuKhoa, thamSoDoc(truyVan)).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
    });

    return () => {
      bo = true;
    };
  }, [truyVan, lanDoc]);

  /**
   * Đổi truy vấn — chuyển trang hoặc đổi bộ lọc. `dangTai` đặt Ở ĐÂY, trong sự kiện, chứ không
   * trong thân effect: gọi setState thẳng trong thân effect kéo theo một lượt render phụ mỗi lần
   * chạy, và lint của React chặn đúng mẫu ấy. Trạng thái khởi tạo đã là `dangTai` nên lần tải đầu
   * không cần ai đặt gì.
   *
   * ĐÓNG BIỂU MẪU SỬA KHI ĐỔI TRUY VẤN. Biểu mẫu giữ bản nháp của một người ở trang vừa rời khỏi; để
   * nó mở là để trên màn hình một ô Lưu thuộc về một dòng không còn nhìn thấy.
   */
  const doiTruyVan = useCallback((tinh: (cu: TruyVanDanhBa) => TruyVanDanhBa) => {
    datTrangThai({ pha: "dangTai" });
    datDangSua(null);
    datBan(BAN_TRONG);
    datDangMoCK(null);
    datDangXoa(null);
    datLoiMayChu("");
    datTruyVan(tinh);
  }, []);

  const diToiTrang = useCallback(
    (toi: NganXepConTro) => doiTruyVan((cu) => ({ ...cu, nganXep: toi })),
    [doiTruyVan],
  );

  /** Đổi bộ lọc — `apLoc` luôn đưa ngăn xếp về trang đầu. */
  const doiLoc = useCallback(
    (doi: Partial<LocDanhBa>) => doiTruyVan((cu) => apLoc(cu, doi)),
    [doiTruyVan],
  );

  /** Mở biểu mẫu sửa: nạp giá trị đang có vào bản nháp, dọn mọi thông báo của lần trước. */
  const moSua = useCallback((cb: identity_canBoTomTat) => {
    datDangMoCK(null);
    datDangXoa(null);
    datDangSua(cb);
    datBan(banTuCanBo(cb));
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  /** Mở hộp công khai hoặc rút cho MỘT người. Đóng biểu mẫu sửa nếu đang mở. */
  const moCongKhai = useCallback((dm: DangMoCongKhai) => {
    datDangSua(null);
    datBan(BAN_TRONG);
    datDangXoa(null);
    datDangMoCK(dm);
    datBanCK(banCongKhaiTu(dm.canBo));
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const dongCongKhai = useCallback(() => {
    datDangMoCK(null);
    datLoiMayChu("");
  }, []);

  /** Mở hộp xoá cho MỘT dòng. Lý do luôn bắt đầu trống — mỗi lần xoá một lý do của riêng nó. */
  const moXoa = useCallback((cb: identity_canBoTomTat) => {
    datDangSua(null);
    datBan(BAN_TRONG);
    datDangMoCK(null);
    datDangXoa(cb);
    datLyDoXoa("");
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const dongXoa = useCallback(() => {
    datDangXoa(null);
    datLyDoXoa("");
    datLoiMayChu("");
  }, []);

  const dongSua = useCallback(() => {
    datDangSua(null);
    datBan(BAN_TRONG);
    datLoiMayChu("");
  }, []);

  /**
   * Sau một lần ghi thành công: đóng biểu mẫu, nói ra đã làm gì, và ĐỌC LẠI trang đang xem.
   *
   * ĐỌC LẠI CẢ TRANG CHỨ KHÔNG VÁ MỘT DÒNG TẠI CHỖ. `PATCH` trả về đúng dòng vừa sửa nên vá tại
   * chỗ là làm được — nhưng một dòng vá tại chỗ và một dòng đọc lại là hai đường cập nhật màn
   * hình, và đường ít chạy hơn là đường sẽ sai mà không ai thấy.
   */
  const ghiXong = useCallback((cau: string) => {
    datDangSua(null);
    datBan(BAN_TRONG);
    datDangMoCK(null);
    datDangXoa(null);
    datLyDoXoa("");
    datLoiMayChu("");
    datCauDaXong(cau);
    datLanDoc((n) => n + 1);
  }, []);

  const guiSua = useCallback(() => {
    if (dangSua === null || dangGui) return;

    datLoiMayChu("");
    datCauDaXong("");
    datDangGui(true);

    // KHÔNG KIỂM ĐỘ DÀI, KHUÔN THƯ ĐIỆN TỬ HAY KÝ TỰ SỐ ĐIỆN THOẠI Ở ĐÂY. Máy chủ kiểm cả ba, mỗi
    // thứ kèm một câu tiếng Việt nói rõ phải sửa gì (`domain/danh_ba_ghi.go`); chép chúng xuống
    // client là dựng bản sao thứ hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
    void suaCanBo(dangSua.id, thanSua(ban, dangSua))
      .then((kq) => {
        if (kq.ok) ghiXong(daLuuHoSo(kq.duLieu.full_name));
        else datLoiMayChu(kq.thongBao);
      })
      .finally(() => datDangGui(false));
  }, [ban, dangGui, dangSua, ghiXong]);

  const guiCongKhai = useCallback(() => {
    if (dangMoCK === null || dangGui) return;

    // Công khai: chưa tick hay thứ tự sai thì dừng TẠI ĐÂY, không gọi mạng. Rút: luôn gửi lại thứ
    // tự đang có, vì PUT thiếu `display_order` là xoá nó (`yeuCauRut`).
    let yc;
    if (dangMoCK.kieu === "congKhai") {
      const kq = yeuCauCongKhai(banCK);
      if ("loi" in kq) {
        datLoiMayChu(kq.loi);
        return;
      }
      yc = kq.yeuCau;
    } else {
      yc = yeuCauRut(dangMoCK.canBo);
    }

    datLoiMayChu("");
    datCauDaXong("");
    datDangGui(true);
    const congKhai = dangMoCK.kieu === "congKhai";
    void datCongKhaiCanBo(dangMoCK.canBo.id, yc)
      .then((kq) => {
        if (kq.ok) ghiXong(congKhai ? daCongKhai(kq.duLieu.full_name) : daRut(kq.duLieu.full_name));
        else datLoiMayChu(kq.thongBao);
      })
      .finally(() => datDangGui(false));
  }, [banCK, dangGui, dangMoCK, ghiXong]);

  /**
   * Gửi xoá. Dòng có tài khoản, lý do rỗng hay quá dài dừng TẠI ĐÂY (`yeuCauXoa`), không gọi mạng.
   *
   * SAU 204: `ghiXong` đọc lại trang bằng cách tăng `lanDoc` và KHÔNG đụng `truyVan`, nên lần đọc
   * lại mang đúng chữ tìm, bộ lọc và con trỏ đang áp — người đang xem kết quả tìm "Nguyễn Văn" vẫn
   * thấy kết quả ấy, trừ đúng dòng vừa xoá.
   */
  const guiXoa = useCallback(() => {
    if (dangXoa === null || dangGui) return;
    const kq = yeuCauXoa(dangXoa, lyDoXoa);
    if ("loi" in kq) {
      datLoiMayChu(kq.loi);
      return;
    }

    datLoiMayChu("");
    datCauDaXong("");
    datDangGui(true);
    const hoTen = dangXoa.full_name;
    void xoaCanBo(dangXoa.id, kq.lyDo)
      .then((ketQua) => {
        if (ketQua.ok) ghiXong(daXoa(hoTen));
        else datLoiMayChu(ketQua.thongBao);
      })
      .finally(() => datDangGui(false));
  }, [dangGui, dangXoa, ghiXong, lyDoXoa]);

  const hanhDongCongKhai = useMemo(
    () =>
      duocCongKhai
        ? {
            onThem: (cb: identity_canBoTomTat) => moCongKhai({ kieu: "congKhai", canBo: cb }),
            onRut: (cb: identity_canBoTomTat) => moCongKhai({ kieu: "rut", canBo: cb }),
          }
        : undefined,
    [duocCongKhai, moCongKhai],
  );

  return (
    <section className="man-danh-ba" aria-labelledby="tieu-de-danh-ba-lien-he">
      <h2 id="tieu-de-danh-ba-lien-he" className="an-thi-giac">
        Danh sách cán bộ
      </h2>

      {/* MỘT THẺ KPI, KHÔNG BA — xem `PHAN_CHUA_DUNG`. Dựng bằng `dl` chứ không bằng một thẻ
          trang trí: nhãn và con số phải đi liền nhau cả với trình đọc màn hình. */}
      <dl className="danh-sach-truong">
        <dt>{NHAN_SO_KHOI}</dt>
        <dd>{nhanSoKhoi(soKhoi)}</dd>
      </dl>

      {/* Danh mục bộ phận hỏng thì NÓI RA MỘT LẦN Ở ĐÂY, không để hai mươi ô cùng báo lỗi. Hiện
          đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
      {soKhoi.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          Danh mục khối / đơn vị: {soKhoi.thongBao}
        </p>
      )}

      {/* Hàng lọc đứng ngay dưới thẻ KPI, trên bảng — đúng bố cục đặc tả §2. */}
      <HangLoc loc={truyVan.loc} boPhan={mucBoPhan} doiLoc={doiLoc} />

      {/* Câu xác nhận sau một lần ghi. `role="status"` chứ không `alert`: không có gì hỏng. */}
      {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}

      {/*
        MỘT BIỂU MẪU, MỘT CHỖ TRÊN MÀN HÌNH, ĐẶT TRÊN BẢNG.

        Không dựng biểu mẫu lồng trong dòng của bảng: ở bề rộng nhỏ nhất (320px) bảng cuộn NGANG,
        nên một biểu mẫu nằm trong một ô của bảng có thể mở ra ngoài khung nhìn và người dùng không
        thấy nó đã mở. Tiêu đề biểu mẫu luôn gọi tên người đang được sửa (`tieuDeSua`), nên không
        có ca nào sửa nhầm hồ sơ vì không biết biểu mẫu thuộc về dòng nào.
      */}
      {dangSua !== null && (
        <BieuMauGhiCanBo
          dangMo={{ kieu: "sua", canBo: dangSua }}
          ban={ban}
          datBan={datBan}
          // Hai tham số của nhánh "đổi vai trò". Nhánh ấy không bao giờ chạy ở đây vì `dangMo.kieu`
          // luôn là `"sua"`; truyền giá trị rỗng chứ không đọc `GET /api/v1/roles` cho một ô chọn
          // không bao giờ dựng ra.
          vaiTroID=""
          datVaiTroID={() => undefined}
          boPhan={mucBoPhan}
          vaiTro={[]}
          loiMayChu={loiMayChu}
          dangGui={dangGui}
          onGui={guiSua}
          onHuy={dongSua}
        />
      )}

      {dangMoCK !== null && (
        <HopCongKhai
          dangMo={dangMoCK}
          ban={banCK}
          datBan={datBanCK}
          loiMayChu={loiMayChu}
          dangGui={dangGui}
          onGui={guiCongKhai}
          onHuy={dongCongKhai}
        />
      )}

      {dangXoa !== null && (
        <HopXoa
          canBo={dangXoa}
          lyDo={lyDoXoa}
          datLyDo={datLyDoXoa}
          loiMayChu={loiMayChu}
          dangGui={dangGui}
          onGui={guiXoa}
          onHuy={dongXoa}
        />
      )}

      {trangThai.pha === "dangTai" && <p role="status">Đang tải danh bạ…</p>}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải. Mọi mã lỗi — kể cả 401, 403, 404 —
          đều trả cùng hình dạng `httpx.Error`, nên không có chỗ nào ở đây rẽ nhánh theo `code` để
          đoán chuyện gì đã xảy ra, và `trace_id` không hiện ra: nó là mốc tra log, không phải mã
          lỗi nghiệp vụ (`lib/api/goi.ts`). */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length === 0 && (
        <p className="trang-thai-rong">{dangLoc(truyVan.loc) ? KHONG_KHOP_LOC : DANH_BA_RONG}</p>
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length > 0 && (
        <>
          <BangLienHe
            danhSach={trangThai.trang.items}
            traBoPhan={traBoPhan}
            onSua={moSua}
            congKhai={hanhDongCongKhai}
            onXoa={duocXoa ? moXoa : undefined}
          />
          <p className="ghi-chu">{GHI_CHU_SO_DIEN_THOAI}</p>
          <DieuHuongTrang
            nganXep={truyVan.nganXep}
            conTroTiep={trangThai.trang.next_cursor}
            conTrangSau={trangThai.trang.has_more}
            diToiTrang={diToiTrang}
          />
        </>
      )}

      <KhoiChuaMo />
    </section>
  );
}

/**
 * Hàng lọc — ô tìm, ô khối / đơn vị, ô trạng thái hiển thị (đặc tả §3). Ba bộ lọc kết hợp theo
 * AND; đổi bất kỳ bộ lọc nào là về trang đầu (`apLoc`).
 *
 * Ô TÌM GỬI BẰNG SUBMIT (Enter hoặc nút Tìm), KHÔNG THEO TỪNG PHÍM: mỗi phím là một lời gọi mạng
 * mang chữ đang gõ dở, và một danh sách nhảy liên tục dưới tay người đang gõ.
 *
 * Ô NHẬP KHÔNG CÓ THUỘC TÍNH `name`, VÀ ĐÓ LÀ LỚP CHẶN CHỨ KHÔNG PHẢI SƠ SÓT. Trước khi JavaScript
 * chạy xong (mạng chậm ở xã, hay một lỗi nạp bundle), bấm Enter trong một `<form>` là trình duyệt
 * tự gửi form theo kiểu GET — mọi ô CÓ `name` lên URL thành `?ten=chu-da-go`, vào thanh địa chỉ và
 * lịch sử trình duyệt (luật 3, cấm #4). Ô không có `name` thì không có gì để gửi. `method="post"`
 * là lớp thứ hai cho cùng ca ấy. Giá trị đọc từ state của React, không từ `FormData`.
 *
 * `autoComplete="off"`: trình duyệt nhớ những gì đã gõ vào ô nhập để gợi ý lại — trên một máy dùng
 * chung, đó là danh sách họ tên và số điện thoại người trước đã tìm.
 *
 * KHÔNG CÓ `maxLength`: thuộc tính ấy đếm đơn vị UTF-16, không phải ký tự, nên sẽ cắt sai. Giới hạn
 * được kiểm lúc gửi, bằng đúng phép đếm máy chủ dùng (`chuanHoaTuKhoaTim`).
 */
export function HangLoc({
  loc,
  boPhan,
  doiLoc,
}: {
  loc: LocDanhBa;
  boPhan: readonly MucChon[];
  doiLoc: (doi: Partial<LocDanhBa>) => void;
}) {
  const [oTim, datOTim] = useState("");
  const [loiTim, datLoiTim] = useState("");

  function gui(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const kq = ketQuaGuiTim(oTim);
    if ("loi" in kq) {
      datLoiTim(kq.loi);
      return;
    }
    datLoiTim("");
    doiLoc(kq.doi);
  }

  return (
    <div className="hang-loc">
      <form className="form-tra-cuu" role="search" method="post" onSubmit={gui}>
        <div className="o-nhap">
          <label htmlFor="tim-danh-ba">{NHAN_O_TIM}</label>
          <input
            id="tim-danh-ba"
            type="search"
            value={oTim}
            onChange={(e) => datOTim(e.target.value)}
            placeholder={GOI_Y_O_TIM}
            autoComplete="off"
            aria-describedby="loi-tim-danh-ba"
            aria-invalid={loiTim !== ""}
          />
        </div>
        <button className="nut-phu" type="submit">
          {NUT_TIM}
        </button>
      </form>
      <p id="loi-tim-danh-ba" className="thong-bao-loi" role="alert">
        {loiTim}
      </p>

      <p className="chon-hang-muc">
        <label htmlFor="loc-khoi-danh-ba">{NHAN_LOC_KHOI}</label>{" "}
        <select
          id="loc-khoi-danh-ba"
          value={loc.boPhan}
          onChange={(e) => doiLoc({ boPhan: maBoPhanLoc(e.target.value, boPhan) })}
        >
          <option value="">{TAT_CA_KHOI}</option>
          {boPhan.map((bp) => (
            <option key={bp.id} value={bp.id}>
              {bp.name}
            </option>
          ))}
        </select>
      </p>

      <p className="chon-hang-muc">
        <label htmlFor="loc-hien-thi-danh-ba">{NHAN_LOC_HIEN_THI}</label>{" "}
        <select
          id="loc-hien-thi-danh-ba"
          value={loc.hienThi}
          onChange={(e) => doiLoc({ hienThi: maHienThi(e.target.value) })}
        >
          {THU_TU_HIEN_THI.map((ma) => (
            <option key={ma} value={ma}>
              {LUA_CHON_HIEN_THI[ma].nhan}
            </option>
          ))}
        </select>
      </p>
    </div>
  );
}

/**
 * Danh sách "đặc tả có, ở đây không" — hiện ngay trên màn hình, không giấu trong chú thích.
 *
 * ĐẶT CUỐI TRANG, KHÔNG ĐẦU TRANG: người mở danh bạ đến để tìm một số điện thoại, và bảy dòng giải
 * thích chắn trước bảng là bảy dòng bị lướt qua mỗi ngày. Ở cuối, nó là thứ người ta đọc đúng lúc
 * đi tìm một nút không thấy.
 */
function KhoiChuaMo() {
  return (
    <aside className="khoi-chua-khai" aria-labelledby="tieu-de-danh-ba-chua-mo">
      <h3 id="tieu-de-danh-ba-chua-mo">{TIEU_DE_PHAN_CHUA_DUNG}</h3>
      <ul>
        {PHAN_CHUA_DUNG.map((p) => (
          <li key={p.ten}>
            <strong>{p.ten}</strong> — {p.viSao}
          </li>
        ))}
      </ul>
    </aside>
  );
}

/**
 * Phân trang theo con trỏ.
 *
 * KHÔNG CÓ SỐ TRANG VÀ KHÔNG CÓ TỔNG SỐ, và đó không phải thiếu sót: hợp đồng trả `next_cursor` +
 * `has_more` chứ không trả `total`, vì máy chủ đọc theo mốc và cố ý không chạy `COUNT(*)` trên bảng
 * đã phân mảnh. Hiện "Trang 3/12" ở đây là báo một con số không ai tính (`core/page`).
 */
function DieuHuongTrang({
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
  // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó. Bấm
  // khi con trỏ rỗng thì `sangTrangSau` ném lỗi — nút phải mờ đi trước khi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang" aria-label="Phân trang danh bạ cán bộ">
      <button
        type="button"
        className="nut-phu"
        disabled={!coTrangTruoc(nganXep)}
        onClick={() => diToiTrang(veTrangTruoc(nganXep))}
      >
        Trang trước
      </button>
      <button
        type="button"
        className="nut-phu"
        disabled={!coSau}
        onClick={() => diToiTrang(sangTrangSau(nganXep, conTroTiep))}
      >
        Trang sau
      </button>
    </nav>
  );
}
