"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

import {
  Building2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  CloudOff,
  Construction,
  Eye,
  EyeOff,
  List,
  RotateCw,
  Search,
  SearchX,
  Send,
  Users,
  type LucideIcon,
} from "lucide-react";

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
import {
  datCongKhaiCanBo,
  docTrangDanhBa,
  publishStaffBulk,
  suaCanBo,
  xoaCanBo,
} from "@/lib/api/can-bo";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardFooter } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Field, Toolbar } from "@/components/ui/field";
import { Notice } from "@/components/ui/notice";
import { Segmented } from "@/components/ui/segmented";
import { cn } from "@/lib/cn";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type {
  identity_canBoTomTat,
  identity_danhSachBoPhanRa,
  page_Result_identity_canBoTomTat,
} from "@/lib/api/schema.gen";
import type { KetQua } from "@/lib/api/goi";

import { BangLienHe } from "./bang-lien-he";
import {
  BULK_OPEN_BUTTON,
  addSelected,
  bulkRequest,
  bulkResultLines,
  removeSelected,
  setConsent,
  type BulkSelection,
} from "./bulk-publication";
import { BulkPublicationPanel, type BulkOutcome } from "./bulk-publication-panel";
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
  type MaHienThi,
  type TruyVanDanhBa,
} from "./loc-danh-ba";
import {
  DANH_BA_RONG,
  GHI_CHU_SO_DIEN_THOAI,
  KHONG_KHOP_LOC,
  LOAD_FAILED_TITLE,
  NO_MATCH_TITLE,
  PAGE_NEXT,
  PAGE_PREVIOUS,
  PHAN_CHUA_DUNG,
  RELOAD,
  TIEU_DE_PHAN_CHUA_DUNG,
  demSoKhoi,
  notBuiltSummary,
  unitCountText,
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
 * BỐN THAO TÁC GHI THUỘC VỀ MÀN DANH BẠ: `PATCH /api/v1/staff/{id}` (sửa chức vụ, khối/đơn vị, số
 * liên hệ, Có Zalo), `PUT .../publication` (công khai / rút MỘT người trên Mini App, #12,
 * `content.update`), `POST /api/v1/staff/publications` (công khai NHIỀU người, xác nhận đồng ý TỪNG
 * dòng — người dùng chốt 30/09/2026, `bulk-publication.ts`, cùng khoá `content.update`) và
 * `DELETE /api/v1/staff/{id}` (xoá một dòng NHẬP TRÙNG, #10, `admin.user.delete`). Bốn tuyến còn
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
   * "Công khai nhiều người". The selection SURVIVES a page or filter change — that is how people on
   * several pages are gathered into one request — and is emptied only after a completed send.
   *
   * THE IDEMPOTENCY KEY BELONGS TO ONE BODY. It is re-minted whenever the selection or a tick
   * changes, and after every completed send; a retry after a network failure keeps it, because
   * that first send may already have published people. Reusing a key for a DIFFERENT body would
   * get the first body's replay back and hide what the second one asked for.
   */
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkSelection, setBulkSelection] = useState<BulkSelection>([]);
  const [bulkKey, setBulkKey] = useState(() => crypto.randomUUID());
  const [bulkOutcome, setBulkOutcome] = useState<BulkOutcome | null>(null);
  const [bulkError, setBulkError] = useState("");

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
    setBulkOpen(false);
    datDangMoCK(null);
    datDangXoa(null);
    datDangSua(cb);
    datBan(banTuCanBo(cb));
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  /** Mở hộp công khai hoặc rút cho MỘT người. Đóng biểu mẫu sửa nếu đang mở. */
  const moCongKhai = useCallback((dm: DangMoCongKhai) => {
    setBulkOpen(false);
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
    setBulkOpen(false);
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

  /** Changing WHAT would be sent: new body, new key, and the previous result no longer applies. */
  const changeBulk = useCallback((next: (cu: BulkSelection) => BulkSelection) => {
    setBulkSelection(next);
    setBulkKey(crypto.randomUUID());
    setBulkOutcome(null);
    setBulkError("");
  }, []);

  const openBulk = useCallback(() => {
    datDangSua(null);
    datBan(BAN_TRONG);
    datDangMoCK(null);
    datDangXoa(null);
    datLoiMayChu("");
    datCauDaXong("");
    setBulkOutcome(null);
    setBulkError("");
    setBulkOpen(true);
  }, []);

  const closeBulk = useCallback(() => {
    setBulkOpen(false);
    setBulkOutcome(null);
    setBulkError("");
  }, []);

  /**
   * Send the bulk request. Refusals that need no server (nobody selected, nobody ticked, over the
   * cap) stop HERE. On a 200 — per-row results or a replay — the selection is emptied, the key is
   * re-minted and the register is re-read: the table, not this panel, says who is now on.
   */
  const sendBulk = useCallback(() => {
    if (dangGui) return;
    const req = bulkRequest(bulkSelection);
    if ("error" in req) {
      setBulkError(req.error);
      return;
    }
    setBulkError("");
    setBulkOutcome(null);
    datCauDaXong("");
    datDangGui(true);
    const sentSelection = bulkSelection;
    void publishStaffBulk(req.rows, bulkKey)
      .then((kq) => {
        if (!kq.ok) {
          // Same key kept: a retry of this very body must be recognised as one.
          setBulkError(kq.thongBao);
          return;
        }
        setBulkOutcome(
          kq.duLieu.kind === "replayed"
            ? { kind: "replayed" }
            : { kind: "lines", lines: bulkResultLines(kq.duLieu.items, sentSelection) },
        );
        setBulkSelection([]);
        setBulkKey(crypto.randomUUID());
        datLanDoc((n) => n + 1);
      })
      .finally(() => datDangGui(false));
  }, [bulkKey, bulkSelection, dangGui]);

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

  /**
   * "Tải lại" after a failed read — the SAME mechanism a completed write uses (`lanDoc`): the effect
   * re-reads with the very query on screen. `dangTai` is set here, in the event, for the reason
   * `doiTruyVan` gives.
   */
  const docLai = useCallback(() => {
    datTrangThai({ pha: "dangTai" });
    datLanDoc((n) => n + 1);
  }, []);

  return (
    <section className="man-danh-ba mt-0 flex min-w-0 flex-col gap-4" aria-labelledby="tieu-de-danh-ba-lien-he">
      <h2 id="tieu-de-danh-ba-lien-he" className="an-thi-giac">
        Danh sách cán bộ
      </h2>

      {/*
        THE RIGHT HALF OF THE PAGE HEADER. The `<h1>` stays in `app/danh-ba/page.tsx`, OUTSIDE the
        permission gate, so the page keeps its title while the gate is still reading the session or
        when it refuses. The count badge and the "Công khai nhiều người" button need this screen's
        state, so they render here — and from `lg` up they are lifted into the header row by
        `lg:absolute` against the `relative` wrapper `page.tsx` puts around header + gate (the
        header reserves the space with `lg:pr-*`). Below `lg` they simply flow as the first row.

        MỘT THẺ KPI, KHÔNG BA — xem `PHAN_CHUA_DUNG`. Con số chỉ ghép khi đã đếm xong (`unitCountText`).
      */}
      <div className="flex flex-wrap items-center gap-3 lg:absolute lg:top-0 lg:right-0 lg:h-12 lg:justify-end">
        <p className="m-0">
          <Badge tone="info" icon={Building2}>
            {unitCountText(soKhoi)}
          </Badge>
        </p>
        {duocCongKhai && !bulkOpen && (
          // A native `<button>` whose ONLY child is the label: the flow test finds this button by
          // `children === BULK_OPEN_BUTTON`. The `Send` icon is therefore drawn beside it, over the
          // button's left padding, and lets clicks through.
          <span className="relative inline-flex">
            <button
              type="button"
              className={cn("nut-chinh", buttonVariants({ variant: "primary" }), "pl-11 max-lg:h-11")}
              onClick={openBulk}
            >
              {BULK_OPEN_BUTTON}
            </button>
            <Send
              aria-hidden="true"
              focusable="false"
              className="pointer-events-none absolute top-1/2 left-4 size-[18px] -translate-y-1/2 text-white"
            />
          </span>
        )}
      </div>

      {/* Danh mục bộ phận hỏng thì NÓI RA MỘT LẦN Ở ĐÂY, không để hai mươi ô cùng báo lỗi. Hiện
          đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
      {soKhoi.pha === "loi" && (
        <p className="thong-bao-loi m-0" role="alert">
          Danh mục khối / đơn vị: {soKhoi.thongBao}
        </p>
      )}

      {/* Câu xác nhận sau một lần ghi. `role="status"` chứ không `alert`: không có gì hỏng. */}
      {cauDaXong !== "" && (
        <Notice tone="info" icon={CircleCheck} role="status">
          {cauDaXong}
        </Notice>
      )}

      {/*
        MỘT BIỂU MẪU, MỘT CHỖ TRÊN MÀN HÌNH, ĐẶT TRÊN BẢNG.

        Không dựng biểu mẫu lồng trong dòng của bảng: ở bề rộng nhỏ nhất (320px) bảng cuộn NGANG,
        nên một biểu mẫu nằm trong một ô của bảng có thể mở ra ngoài khung nhìn và người dùng không
        thấy nó đã mở. Tiêu đề biểu mẫu luôn gọi tên người đang được sửa (`tieuDeSua`), nên không
        có ca nào sửa nhầm hồ sơ vì không biết biểu mẫu thuộc về dòng nào.
      */}
      {/* Gated by `content.update` like the single-person buttons — convenience only: the server
          checks the key on the request itself (rule 5, forbidden #1). */}
      {duocCongKhai && bulkOpen && (
        <BulkPublicationPanel
          selection={bulkSelection}
          pageRows={trangThai.pha === "xong" ? trangThai.trang.items : []}
          onSelect={(cb) => changeBulk((cu) => addSelected(cu, cb))}
          onUnselect={(id) => changeBulk((cu) => removeSelected(cu, id))}
          onSetConsent={(id, v) => changeBulk((cu) => setConsent(cu, id, v))}
          error={bulkError}
          sending={dangGui}
          outcome={bulkOutcome}
          onSubmit={sendBulk}
          onClose={closeBulk}
        />
      )}

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

      {/* ONE card: filter row, then exactly one of loading · error · empty · table, then the pager.
          Below 768px the card frame drops away and the rows are cards of their own (spec §9). */}
      <Card className="max-md:overflow-visible max-md:rounded-none max-md:border-0 max-md:bg-transparent max-md:shadow-none">
        <HangLoc loc={truyVan.loc} boPhan={mucBoPhan} doiLoc={doiLoc} />

        {trangThai.pha === "dangTai" && <LoadingRows />}

        {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải. Mọi mã lỗi — kể cả 401, 403, 404 —
            đều trả cùng hình dạng `httpx.Error`, nên không có chỗ nào ở đây rẽ nhánh theo `code` để
            đoán chuyện gì đã xảy ra, và `trace_id` không hiện ra: nó là mốc tra log, không phải mã
            lỗi nghiệp vụ (`lib/api/goi.ts`). */}
        {trangThai.pha === "loi" && (
          <div role="alert">
            <EmptyState
              icon={CloudOff}
              title={LOAD_FAILED_TITLE}
              description={trangThai.thongBao}
              className="[&>span:first-child]:bg-danger-50 [&>span:first-child]:text-danger-600"
              action={
                <Button type="button" variant="secondary" icon={<RotateCw aria-hidden="true" />} onClick={docLai}>
                  {RELOAD}
                </Button>
              }
            />
          </div>
        )}

        {trangThai.pha === "xong" &&
          trangThai.trang.items.length === 0 &&
          (dangLoc(truyVan.loc) ? (
            <EmptyState icon={SearchX} title={NO_MATCH_TITLE} description={KHONG_KHOP_LOC} />
          ) : (
            <EmptyState icon={Users} title={DANH_BA_RONG} />
          ))}

        {trangThai.pha === "xong" && trangThai.trang.items.length > 0 && (
          <>
            <BangLienHe
              danhSach={trangThai.trang.items}
              traBoPhan={traBoPhan}
              onSua={moSua}
              congKhai={hanhDongCongKhai}
              onXoa={duocXoa ? moXoa : undefined}
            />
            <CardFooter className="justify-end max-md:mt-3 max-md:border-0 max-md:px-0">
              <DieuHuongTrang
                nganXep={truyVan.nganXep}
                conTroTiep={trangThai.trang.next_cursor}
                conTrangSau={trangThai.trang.has_more}
                diToiTrang={diToiTrang}
              />
            </CardFooter>
          </>
        )}
      </Card>

      {trangThai.pha === "xong" && trangThai.trang.items.length > 0 && (
        <Notice tone="legal" className="m-0">
          {GHI_CHU_SO_DIEN_THOAI}
        </Notice>
      )}

      <KhoiChuaMo />
    </section>
  );
}

/**
 * First-load placeholder in the shape of the table rows (spec v2 §8b) — static grey blocks, no
 * shimmer: a moving background is the motion the spec rules out. The words stay for a screen reader
 * (`role="status"`), the blocks are hidden from it.
 */
function LoadingRows() {
  return (
    <div role="status" className="px-4 py-2 max-md:px-0">
      <span className="an-thi-giac">Đang tải danh bạ…</span>
      <ul aria-hidden="true" className="m-0 list-none p-0">
        {[0, 1, 2, 3, 4].map((i) => (
          <li key={i} className="flex items-center gap-3 border-b border-line py-2 last:border-b-0">
            <span className="size-8 shrink-0 rounded-full bg-[#eef1f6]" />
            <span className="flex min-w-0 flex-1 flex-col gap-1.5">
              <span className="h-3 w-2/5 max-w-48 rounded bg-[#eef1f6]" />
              <span className="h-2.5 w-1/4 max-w-28 rounded bg-[#eef1f6]" />
            </span>
            <span className="hidden h-3 w-28 rounded bg-[#eef1f6] md:block" />
            <span className="h-6 w-20 rounded-full bg-[#eef1f6]" />
          </li>
        ))}
      </ul>
    </div>
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
    <Toolbar className="hang-loc m-0 flex-row max-md:border-0 max-md:px-0 max-md:pt-0">
      <div className="flex min-w-0 flex-[1_1_320px] flex-col md:max-w-[30rem]">
        <form className="form-tra-cuu m-0 flex flex-row items-end gap-2" role="search" method="post" onSubmit={gui}>
          <Field
            label={NHAN_O_TIM}
            htmlFor="tim-danh-ba"
            hideLabel
            icon={Search}
            grow="search"
            className={cn("max-w-none", TALL_ON_PHONE)}
          >
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
          </Field>
          <Button type="submit" variant="secondary" className="max-md:h-11">
            {NUT_TIM}
          </Button>
        </form>
        {/* Always in the DOM (a live region added later is not read by every screen reader), but
            zero-height while empty so it does not push the row's bottom edge out of line. */}
        <p
          id="loi-tim-danh-ba"
          className="thong-bao-loi m-0 min-h-0 [&:not(:empty)]:mt-1.5"
          role="alert"
        >
          {loiTim}
        </p>
      </div>

      <Field
        label={NHAN_LOC_KHOI}
        htmlFor="loc-khoi-danh-ba"
        hideLabel
        icon={Building2}
        kind="select"
        className={cn("max-md:flex-[1_1_100%]", TALL_ON_PHONE)}
      >
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
      </Field>

      {/* THREE OPTIONS → SEGMENTED (spec §7), emitting EXACTLY the select's three codes ("", "1",
          "0") through the same `maHienThi` → `doiLoc`. Its radios carry a `name` (a radio group needs
          one) and that is safe for the reason the search box has none: they sit OUTSIDE the search
          `<form>`, so no native submit can put them in a URL — and they hold a filter code, never
          what somebody typed. */}
      <Segmented
        legend={NHAN_LOC_HIEN_THI}
        name="loc-hien-thi-danh-ba"
        value={loc.hienThi}
        options={THU_TU_HIEN_THI.map((ma) => ({
          value: ma,
          label: LUA_CHON_HIEN_THI[ma].nhan,
          icon: SEGMENT_ICON[ma],
        }))}
        onChange={(v) => doiLoc({ hienThi: maHienThi(v) })}
        className="max-md:[&_label]:h-[38px]"
      />
    </Toolbar>
  );
}

/** Field descendants at 44px below 768px — the same selectors Field uses for 40px, so they win there. */
const TALL_ON_PHONE =
  "max-md:[&_input:not([type=checkbox]):not([type=radio])]:h-11 max-md:[&_select]:h-11";

const SEGMENT_ICON: Record<MaHienThi, LucideIcon> = { "": List, "1": Eye, "0": EyeOff };

/**
 * Danh sách "đặc tả có, ở đây không" — hiện ngay trên màn hình, không giấu trong chú thích.
 *
 * ĐẶT CUỐI TRANG, KHÔNG ĐẦU TRANG: người mở danh bạ đến để tìm một số điện thoại, và bảy dòng giải
 * thích chắn trước bảng là bảy dòng bị lướt qua mỗi ngày. Ở cuối, nó là thứ người ta đọc đúng lúc
 * đi tìm một nút không thấy.
 *
 * THU GỌN, XÁM, VIỀN ĐỨT (đặc tả giao diện §8.1, ADR 0068): đây không phải báo động. Nội dung vẫn
 * nguyên văn và vẫn trong HTML khi đóng — `<details>` chỉ thu gọn, không bỏ đi; mở ra bằng một cú
 * bấm hay phím Enter/Space trên dòng tiêu đề.
 */
function KhoiChuaMo() {
  return (
    <details className="khoi-chua-khai group m-0" aria-labelledby="tieu-de-danh-ba-chua-mo">
      <summary className="flex cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden">
        <Construction aria-hidden="true" focusable="false" className="size-[18px] shrink-0 text-ink-500" />
        <span className="font-semibold text-ink-700">{notBuiltSummary(PHAN_CHUA_DUNG.length)}</span>
        <ChevronDown
          aria-hidden="true"
          focusable="false"
          className="ml-auto size-4 shrink-0 text-ink-500 transition-transform group-open:rotate-180"
        />
      </summary>
      <h3 id="tieu-de-danh-ba-chua-mo" className="mt-2 mb-1.5 text-[13px] font-semibold text-ink-700">
        {TIEU_DE_PHAN_CHUA_DUNG}
      </h3>
      <ul className="m-0 pl-5">
        {PHAN_CHUA_DUNG.map((p) => (
          <li key={p.ten} className="mb-1.5">
            <strong>{p.ten}</strong> — {p.viSao}
          </li>
        ))}
      </ul>
    </details>
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
    <nav className="dieu-huong-trang m-0" aria-label="Phân trang danh bạ cán bộ">
      <Button
        type="button"
        variant="secondary"
        className="max-md:h-11"
        icon={<ChevronLeft aria-hidden="true" />}
        disabled={!coTrangTruoc(nganXep)}
        onClick={() => diToiTrang(veTrangTruoc(nganXep))}
      >
        {PAGE_PREVIOUS}
      </Button>
      <Button
        type="button"
        variant="secondary"
        className="max-md:h-11"
        disabled={!coSau}
        onClick={() => diToiTrang(sangTrangSau(nganXep, conTroTiep))}
      >
        {PAGE_NEXT}
        <ChevronRight aria-hidden="true" />
      </Button>
    </nav>
  );
}
