"use client";

import {
  ArrowUpDown,
  Eye,
  KeyRound,
  Lock,
  LockOpen,
  Pencil,
  Plus,
  Search,
  UserCog,
  UserPlus,
  UsersRound,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";

import {
  BieuMauGhiCanBo,
  type DangMoGhi,
  type MucChon,
} from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import {
  BAN_TRONG,
  NUT_DOI_VAI_TRO,
  NUT_KHOA,
  NUT_MO_KHOA,
  NUT_SUA,
  NUT_THEM_CAN_BO,
  VI_SAO_KHONG_CO_NUT_XOA,
  banTuCanBo,
  daDatKhoa,
  daDoiVaiTro,
  daLuuHoSo,
  daThem,
  khoaChongTrungMoi,
  thanSua,
  thanThem,
  tieuDeKhoa,
  tieuDeSua,
  tieuDeThem,
  tieuDeVaiTro,
  type BanNhapCanBo,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  datKhoaCanBo,
  docTrangDanhBa,
  doiVaiTroCanBo,
  layChiTietCanBo,
  suaCanBo,
  themCanBo,
  type ChieuSapXep,
  type KhoaSapXep,
} from "@/lib/api/can-bo";
import {
  GOI_Y_O_TIM,
  NUT_TIM,
  ketQuaGuiTim,
  maBoPhanLoc,
  type LocDanhBa,
} from "@/features/danh-ba/loc-danh-ba";
import { KHONG_KHOP_LOC } from "@/features/danh-ba/nhan-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { docDanhMucDanhBa, type DanhMucDanhBa } from "@/lib/api/danh-muc";
import type { identity_canBoTomTat, page_Result_identity_canBoTomTat } from "@/lib/api/schema.gen";
import { capTaiKhoan, datLaiMatKhau } from "@/lib/api/tai-khoan";
import { cn } from "@/lib/cn";

import { ConfigDialog } from "./config-dialog";
import { ExcelImportPanel } from "./excel-import-panel";
import { STAFF_IMPORT_TARGET } from "./excel-import-targets";
import {
  CAU_PHAT_LAI_KHONG_CO_MAT_KHAU,
  NUT_CAP_TAI_KHOAN,
  NUT_DAT_LAI_MAT_KHAU,
  OMatKhauTam,
  XacNhanTaiKhoan,
  tieuDeXacNhan,
  type DangMoTaiKhoan,
  type MatKhauTamHienRa,
} from "./mat-khau-tam";
import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  TRANG_DAU,
  type NganXepConTro,
} from "./ngan-xep-con-tro";
import {
  NO_EMAIL_ACCOUNT_REASON,
  canIssueAccount,
  emailLabel,
  nhanBoPhan,
  nhanDangNhapGanNhat,
  nhanNgayTao,
  nhanTaiKhoan,
  nhanTrangThai,
  nhanVaiTro,
} from "./nhan-can-bo";
import { bangTraTuKetQua, traTen, type BangTraDanhMuc, type KetTra } from "./tra-danh-muc";

/**
 * Bảng danh bạ cán bộ — `docs/ui-ux/14-cau-hinh.md §3`, tab "Người dùng".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MÀN HÌNH NÀY VẪN ÍT HƠN ĐẶC TẢ, VÀ ĐÓ LÀ CHỦ Ý. Hợp đồng REST phục vụ màn hình này bằng mười hai
 * tuyến: năm tuyến đọc (`GET /api/v1/staff`, `POST /api/v1/staff/searches` — tìm theo chữ, chữ đi
 * trong thân —, `GET /api/v1/staff/{id}`, và hai danh mục của xã
 * `GET /api/v1/org-units` · `GET /api/v1/roles`) và **bảy tuyến ghi** hạ cánh 22/09/2026 —
 * `POST /staff`, `PATCH /staff/{id}`, `POST`/`DELETE /staff/{id}/lockout`, `PUT /staff/{id}/role`,
 * `POST /staff/{id}/account`, `PUT /staff/{id}/password`. Mỗi thứ đặc tả vẽ mà ở đây không có đều
 * mang một chú thích ngay tại chỗ nói vì sao nó vắng và cái gì mở khoá nó.
 *
 * Vẽ ra một điều khiển không chạy được tệ hơn hẳn không vẽ: một ô tìm kiếm gõ vào không có gì
 * xảy ra khiến cán bộ gõ tên một người, thấy danh sách không đổi, và kết luận người đó không
 * có trong hệ thống. Cùng lý lẽ ấy là vì sao **không có nút Xoá** — xem
 * `VI_SAO_KHONG_CO_NUT_XOA`.
 *
 * KHÔNG CÓ CỔNG QUYỀN RIÊNG CHO PHẦN GHI, và đó không phải sơ suất: cả bảy tuyến ghi khai đúng
 * một khoá `admin.user` — cùng khoá mà `TabNguoiDung` đã dùng để quyết định có dựng màn hình này
 * hay không, kể cả hai tuyến thông tin đăng nhập (`routes.go:819` và `:856` cùng khai
 * `RequirePermission(d.Checker, "admin.user")`). Một cổng thứ hai cho cùng một khoá là một bản sao
 * sẽ trôi. Và lớp chặn THẬT vẫn nằm ở máy chủ, trên TỪNG yêu cầu (luật 5, cấm #1): ẩn một nút chỉ
 * để cán bộ khỏi bấm vào thứ chắc chắn trả 403.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Trạng thái của một lần đọc. Ba nhánh rời nhau — không nhánh nào suy ra được từ nhánh khác. */
type TrangThaiTrang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; trang: page_Result_identity_canBoTomTat };

type TrangThaiChiTiet =
  | { pha: "dangTai"; id: string }
  | { pha: "loi"; id: string; thongBao: string }
  | { pha: "xong"; id: string; canBo: identity_canBoTomTat };

/**
 * `active` — whether the tab holding this screen is the one on display. The Cấu hình shell keeps hidden
 * panels mounted (`khung-tab-cau-hinh.tsx`), so without it the catalogues read at mount would stay frozen
 * while the officer creates a unit in "Sơ đồ tổ chức" or a role in "Phân quyền" — and the new one would
 * be missing from this tab's pickers until a full page reload (tester report 05/10, ND-01/ND-02).
 * Omitted means a screen with no tabs around it: always on display.
 */
export function DanhBaCanBo({
  active = true,
  importOpen = false,
  onImportClose = () => {},
}: {
  active?: boolean;
  /**
   * Whether the staff Excel import dialog is open. Owned by `TabNguoiDung`, because its button sits in
   * the page header (prototype); the dialog and what it reloads belong here.
   */
  importOpen?: boolean;
  /** The dialog's own close — the ONLY thing that ends an import attempt (its N passwords included). */
  onImportClose?: () => void;
} = {}) {
  const [khoaSapXep, datKhoaSapXep] = useState<KhoaSapXep>("code");
  const [chieu, datChieu] = useState<ChieuSapXep>("asc");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  /**
   * Bộ lọc đang ÁP DỤNG — chữ tìm đã chuẩn hoá và id bộ phận. Không phải thứ đang gõ dở trong ô.
   *
   * CHỮ TÌM SỐNG Ở ĐÂY VÀ CHỈ Ở ĐÂY: state của component, chết cùng component. Không URL, không
   * `searchParams`, không `localStorage`, không khoá bộ đệm — nó thường là họ tên hay số điện thoại
   * (luật 3, cấm #4). Nó rời trình duyệt đúng một đường: thân `POST /api/v1/staff/searches`.
   */
  const [loc, datLoc] = useState<LocNguoiDung>(LOC_DAU);
  const [trangThai, datTrangThai] = useState<TrangThaiTrang>({ pha: "dangTai" });
  const [chiTiet, datChiTiet] = useState<TrangThaiChiTiet | null>(null);
  /** `null` là chưa đọc xong. Hai danh mục của xã, đọc MỘT lần cho cả màn hình — xem dưới. */
  const [danhMuc, datDanhMuc] = useState<DanhMucDanhBa | null>(null);
  /** Id của lần bấm "Chi tiết" mới nhất — xem `moChiTiet`. */
  const idDangDoi = useRef<string | null>(null);

  /* ---- trạng thái của đường GHI ---------------------------------------------------------- */

  const [dangMo, datDangMo] = useState<DangMoGhi | null>(null);
  const [ban, datBan] = useState<BanNhapCanBo>(BAN_TRONG);
  const [vaiTroID, datVaiTroID] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");
  /**
   * Đếm số lần cần đọc lại danh sách. Tăng sau MỖI lần ghi thành công.
   *
   * VÌ SAO ĐỌC LẠI CẢ TRANG CHỨ KHÔNG VÁ MỘT DÒNG TẠI CHỖ: lần ghi trả về đúng dòng vừa đổi, nên
   * vá tại chỗ là làm được — nhưng nó chỉ đúng với `PATCH`, `lockout` và `role`. Với `POST` thì
   * người mới có thể thuộc về một TRANG KHÁC (danh sách sắp theo mã, còn mã do máy chủ sinh), và
   * một dòng chèn vào trang đang xem là một dòng ở sai chỗ so với thứ tự sắp xếp đang hiện. Hai
   * cách cư xử cho một nút Lưu là chỗ người dùng học sai cách màn hình hoạt động.
   */
  const [lanDoc, datLanDoc] = useState(0);

  /* ---- trạng thái của hai tuyến THÔNG TIN ĐĂNG NHẬP --------------------------------------- */

  /**
   * Bốn trạng thái RIÊNG, không dùng lại `dangMo`/`loiMayChu`/`dangGui` của bốn biểu mẫu danh bạ.
   *
   * Không phải để tách cho gọn: `ghiXong` của đường danh bạ dọn sạch màn hình sau mỗi lần ghi
   * thành công, còn đường này phải để lại trên màn hình một giá trị KHÔNG LẤY LẠI ĐƯỢC. Dùng chung
   * một ô trạng thái là mở đúng một đường cho một lần ghi khác — hay một lần đọc lại danh sách —
   * xoá mất mật khẩu tạm trước khi quản trị viên kịp đọc, và không bài test nào thấy.
   */
  const [moTaiKhoan, datMoTaiKhoan] = useState<DangMoTaiKhoan | null>(null);
  const [loiTaiKhoan, datLoiTaiKhoan] = useState("");
  const [dangGuiTaiKhoan, datDangGuiTaiKhoan] = useState(false);
  /**
   * Mật khẩu tạm đang hiện. `null` là không có gì để hiện.
   *
   * ĐÂY LÀ NƠI DUY NHẤT GIÁ TRỊ ẤY SỐNG: state của component đang hiện nó, chết cùng component.
   * Không `localStorage`, không `sessionStorage`, không biến ở mức module, không `console.*` ở bất
   * kỳ nhánh nào chạm tới nó (luật 3, cấm #1 và #4 — xem đầu tệp `mat-khau-tam.tsx`).
   */
  const [matKhauTam, datMatKhauTam] = useState<MatKhauTamHienRa | null>(null);

  // The Excel import panel's open state (ADR 0059 §1) is the `importOpen` prop: the panel holds its own
  // attempt — including, after a 201, the N temporary passwords — and they die when it unmounts. NOTHING
  // BUT THE PANEL'S OWN CLOSE (`onImportClose`) ends it: a list re-read, a failed re-read, another dialog —
  // none of them may take the passwords off the screen before they are saved. Same rule as `matKhauTam`.

  /**
   * HAI DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT MỖI LẦN MÀN NÀY ĐƯỢC HIỆN — `[active]` ở cuối effect là phần
   * quan trọng nhất của khối này.
   *
   * Bộ phận hay vai trò vừa tạo ở màn khác phải có mặt ở đây mà không cần tải lại trang (ND-01/ND-02).
   * Từ 05/10/2026 màn này là `/nguoi-dung`, không còn là một tab: đi sang `/cau-hinh` hay
   * `/nguoi-dung/phan-quyen` rồi quay lại là dựng lại component, nên effect chạy lại khi dựng. `active`
   * vẫn được nhận để một nơi giữ component mà ẩn nó đi không đọc cho một màn không ai nhìn.
   *
   * Không đọc lại khi đổi trang, đổi sắp xếp hay mở khối chi tiết: danh mục bộ phận và vai trò
   * của một xã không đổi giữa hai lần bấm "Trang sau". Và tuyệt đối không đọc theo từng dòng —
   * ở đây mỗi trang hai mươi dòng, nên một lời gọi mỗi dòng là bốn mươi lời gọi thay vì hai,
   * và con số ấy đi lên theo dữ liệu chứ không đứng yên (`skills/load-data-once`, dạng 1).
   *
   * VÌ SAO ĐỌC Ở TRÌNH DUYỆT CHỨ KHÔNG Ở MÁY CHỦ: hai tuyến này đòi đã đăng nhập, nên gọi phía
   * máy chủ thì phải tự chuyển tiếp cookie phiên — thêm một chỗ cầm cookie, và là đúng chỗ dễ
   * chuyển tiếp sang sai host. Ở đây đường dẫn tương đối trên chính host của xã, trình duyệt
   * tự gửi cookie host-only (`lib/api/goi.ts`). Khác hẳn `GET /api/v1/communes/current`: tuyến
   * ấy công khai nên đọc được ở máy chủ (`lib/tenant-config.ts`).
   *
   * KHÔNG CÓ BỘ ĐỆM NÀO SỐNG QUA LẦN MỞ MÀN HÌNH: bảng tra nằm trong state của component, chết
   * cùng component. Một biến ở mức module giữ danh mục lại là đúng hình dạng của một lần danh
   * mục xã này hiện trên màn hình xã khác (`lib/api/danh-muc.ts`).
   */
  useEffect(() => {
    if (!active) return;
    let bo = false;
    docDanhMucDanhBa().then((dm) => {
      if (!bo) datDanhMuc(dm);
    });
    return () => {
      bo = true;
    };
  }, [active]);

  // Dựng bảng tra một lần cho mỗi lần danh mục đổi, không dựng lại ở mỗi dòng.
  const traBoPhan = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.boPhan),
    [danhMuc],
  );
  const traVaiTro = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.vaiTro),
    [danhMuc],
  );

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau. Không có nó thì
    // bấm "Trang sau" hai lần nhanh có thể để lại trên màn hình đúng trang vừa rời khỏi.
    let bo = false;

    // Không truyền `limit`: để máy chủ áp mặc định của chính nó (20). Giữ một bản sao của con
    // số ấy ở client là giữ một bản sẽ trôi.
    //
    // MỘT chỗ rẽ GET hay POST, và nó ở `docTrangDanhBa` — cùng hàm màn `/danh-ba` dùng. Có chữ tìm
    // thì chữ, bộ phận và con trỏ đi trong thân POST; `sort`/`order` chỉ đi nhánh GET, vì tuyến tìm
    // không nhận chúng (xem `ThanhSapXep` cho câu nói điều ấy ra màn hình).
    docTrangDanhBa(loc.tuKhoa, {
      boPhan: loc.boPhan,
      cursor: nganXep.hienTai,
      sort: khoaSapXep,
      order: chieu,
    }).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
    });

    return () => {
      bo = true;
    };
  }, [khoaSapXep, chieu, nganXep, loc, lanDoc]);

  /**
   * Chuyển trang. `dangTai` được đặt Ở ĐÂY, trong sự kiện bấm, chứ không trong thân effect:
   * gọi setState thẳng trong thân effect kéo theo một lượt render phụ mỗi lần chạy, và lint của
   * React chặn đúng mẫu ấy. Trạng thái khởi tạo đã là `dangTai` nên lần tải đầu không cần ai
   * đặt gì.
   */
  const dongChiTiet = useCallback(() => {
    idDangDoi.current = null;
    datChiTiet(null);
  }, []);

  const diToiTrang = useCallback(
    (toi: NganXepConTro) => {
      datTrangThai({ pha: "dangTai" });
      dongChiTiet();
      datNganXep(toi);
    },
    [dongChiTiet],
  );

  /**
   * Đổi sắp xếp là VỀ TRANG ĐẦU, luôn luôn.
   *
   * Một con trỏ thuộc về đúng một cách sắp xếp: nó mã hoá mốc `(khoá sắp xếp, id)` của dòng
   * cuối vừa phát ra. Mang con trỏ của `sort=code` sang `sort=created_at` thì máy chủ trả 400
   * "con trỏ không hợp lệ" (`core/page/page.go`, `ErrCursor`) — nên ngăn xếp cũ phải bỏ đi,
   * không phải giữ lại.
   */
  const doiSapXep = useCallback(
    (khoa: KhoaSapXep) => {
      if (khoa === khoaSapXep) {
        datChieu((truoc) => (truoc === "asc" ? "desc" : "asc"));
      } else {
        datKhoaSapXep(khoa);
        datChieu("asc");
      }
      diToiTrang(TRANG_DAU);
    },
    [khoaSapXep, diToiTrang],
  );

  /**
   * Đổi chữ tìm hoặc bộ phận là VỀ TRANG ĐẦU, luôn luôn — cùng lý do với `doiSapXep`, và nặng hơn:
   * con trỏ của trang 3 thuộc về truy vấn cũ, gửi nó kèm bộ lọc mới thì máy chủ hoặc từ chối, hoặc
   * trả một trang giữa chừng của truy vấn mới và cán bộ không thấy những người đứng trước con trỏ.
   *
   * KHÔNG đụng `khoaSapXep`/`chieu`: bỏ tìm thì danh sách quay về đúng cách sắp xếp đang chọn.
   */
  const doiLoc = useCallback(
    (doi: Partial<LocNguoiDung>) => {
      datLoc((cu) => ({ ...cu, ...doi }));
      diToiTrang(TRANG_DAU);
    },
    [diToiTrang],
  );

  const dangTim = loc.tuKhoa !== null;
  // Thứ tự THẬT của trang đang hiện. Khi đang tìm, máy chủ luôn trả theo mã tăng dần
  // (`service-identity/internal/http/can_bo_tim.go:40`), bất kể khoá đang chọn — nên chú thích
  // bảng và `aria-sort` phải đọc từ đây, không từ state.
  const khoaHien: KhoaSapXep = dangTim ? "code" : khoaSapXep;
  const chieuHien: ChieuSapXep = dangTim ? "asc" : chieu;

  /**
   * Mở khối chi tiết của một cán bộ.
   *
   * `idDangDoi` KHÔNG PHẢI TỐI ƯU HOÁ. Không có nó, một phản hồi đến muộn của lần bấm trước sẽ
   * ghi đè khối chi tiết: trên màn hình là hồ sơ của người A nằm dưới dòng người B vừa bấm —
   * ghép sai dữ liệu cá nhân với sai người, không phải một lỗi hiển thị.
   */
  const moChiTiet = useCallback(async (id: string) => {
    idDangDoi.current = id;
    datChiTiet({ pha: "dangTai", id });
    const ketQua = await layChiTietCanBo(id);
    if (idDangDoi.current !== id) return;
    datChiTiet(
      ketQua.ok
        ? { pha: "xong", id, canBo: ketQua.duLieu }
        : { pha: "loi", id, thongBao: ketQua.thongBao },
    );
  }, []);

  /* ---- mở, đóng và gửi bốn biểu mẫu ghi ---------------------------------------------------- */

  /**
   * Danh sách mục cho hai ô chọn, lấy từ CHÍNH hai danh mục đã đọc cho bảng tra.
   *
   * KHÔNG ĐỌC LẠI KHI MỞ BIỂU MẪU. Dữ liệu đã nằm trong tay màn hình; một lời gọi nữa ở đây chỉ
   * thêm một câu trả lời thứ hai có thể lệch với tên đang hiện trên chính dòng người dùng vừa bấm.
   *
   * DANH MỤC HỎNG THÌ RA MẢNG RỖNG, VÀ BIỂU MẪU VẪN MỞ ĐƯỢC. Ô chọn khi ấy chỉ còn mục "chưa
   * phân bộ phận" cộng mục giữ nguyên giá trị đang lưu (`OChon`), nên sửa số điện thoại vẫn làm
   * được trong lúc tuyến danh mục đang hỏng — và không thao tác nào ghi đè liên kết cũ.
   */
  const mucBoPhan = useMemo<readonly MucChon[]>(
    () => (danhMuc !== null && danhMuc.boPhan.ok ? danhMuc.boPhan.duLieu.items : []),
    [danhMuc],
  );
  const mucVaiTro = useMemo<readonly MucChon[]>(
    () => (danhMuc !== null && danhMuc.vaiTro.ok ? danhMuc.vaiTro.duLieu.items : []),
    [danhMuc],
  );

  /** Mở một biểu mẫu: dọn sạch mọi thông báo của lần trước, và nạp giá trị đang có vào bản nháp. */
  const moBieuMau = useCallback((m: DangMoGhi) => {
    datDangMo(m);
    datBan(m.kieu === "sua" ? banTuCanBo(m.canBo) : BAN_TRONG);
    datVaiTroID(m.kieu === "vaiTro" ? m.canBo.role_id : "");
    datLoiMayChu("");
    datCauDaXong("");
    // Đóng biểu mẫu xác nhận của đường thông tin đăng nhập: hai biểu mẫu mở cùng lúc là hai nút
    // Lưu cạnh nhau cho hai người khác nhau. KHÔNG đụng `matKhauTam` — ô ấy giữ một giá trị không
    // lấy lại được, và chỉ một hành động rõ ràng của người dùng mới được đóng nó.
    datMoTaiKhoan(null);
    datLoiTaiKhoan("");
  }, []);

  const dongBieuMau = useCallback(() => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datVaiTroID("");
    datLoiMayChu("");
  }, []);

  /**
   * Sau một lần ghi thành công: đóng biểu mẫu, nói ra đã làm gì, và đọc lại danh sách.
   *
   * ĐÓNG LUÔN KHỐI CHI TIẾT. Khối ấy giữ một bản chụp đọc trước lần ghi, nên để nó mở lại là để
   * trên màn hình hai câu trả lời khác nhau về cùng một người — dòng trong bảng đã cập nhật, khối
   * chi tiết ngay dưới vẫn là hồ sơ cũ.
   */
  const ghiXong = useCallback((cau: string) => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datVaiTroID("");
    datLoiMayChu("");
    datCauDaXong(cau);
    idDangDoi.current = null;
    datChiTiet(null);
    datLanDoc((n) => n + 1);
  }, []);

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    datLoiMayChu("");
    datCauDaXong("");
    datDangGui(true);

    // KHÔNG KIỂM ĐỘ DÀI, KHUÔN THƯ ĐIỆN TỬ HAY KÝ TỰ SỐ ĐIỆN THOẠI Ở ĐÂY. Máy chủ kiểm cả ba, mỗi
    // thứ kèm một câu tiếng Việt nói rõ phải sửa gì (`domain/danh_ba_ghi.go`); chép chúng xuống
    // client là dựng bản sao thứ hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
    const goi =
      dangMo.kieu === "them"
        ? themCanBo(thanThem(ban), dangMo.khoaChongTrung).then((kq) =>
            kq.ok ? ghiXong(daThem(kq.duLieu.full_name)) : datLoiMayChu(kq.thongBao),
          )
        : dangMo.kieu === "sua"
          ? suaCanBo(dangMo.canBo.id, thanSua(ban, dangMo.canBo)).then((kq) =>
              kq.ok ? ghiXong(daLuuHoSo(kq.duLieu.full_name)) : datLoiMayChu(kq.thongBao),
            )
          : dangMo.kieu === "vaiTro"
            ? doiVaiTroCanBo(dangMo.canBo.id, vaiTroID).then((kq) =>
                kq.ok ? ghiXong(daDoiVaiTro(kq.duLieu.full_name)) : datLoiMayChu(kq.thongBao),
              )
            : datKhoaCanBo(dangMo.canBo.id, dangMo.khoa).then((kq) =>
                kq.ok
                  ? ghiXong(daDatKhoa(kq.duLieu.full_name, dangMo.khoa))
                  : datLoiMayChu(kq.thongBao),
              );

    void goi.finally(() => datDangGui(false));
  }, [ban, dangGui, dangMo, ghiXong, vaiTroID]);

  /* ---- cấp tài khoản và đặt lại mật khẩu ---------------------------------------------------- */

  /**
   * Mở biểu mẫu xác nhận của một trong hai tuyến.
   *
   * KHOÁ CHỐNG TRÙNG SINH Ở ĐÂY, LÚC MỞ — không lúc gửi, và chỉ cho nhánh `datLai`. Sinh lúc gửi
   * thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới, tức một mật khẩu tạm KHÁC vô hiệu hoá
   * cái quản trị viên vừa đọc qua điện thoại (`lib/api/tai-khoan.ts`, `datLaiMatKhau`). Tuyến cấp
   * tài khoản không có khoá vì chính tài khoản là khoá tự nhiên — lần gửi thứ hai trả 409.
   *
   * ĐÓNG BIỂU MẪU DANH BẠ ĐANG MỞ, KHÔNG ĐÓNG Ô MẬT KHẨU TẠM. Xem `moBieuMau` cho nửa đối xứng.
   */
  const moCapTaiKhoan = useCallback((cb: identity_canBoTomTat) => {
    datDangMo(null);
    datLoiMayChu("");
    datCauDaXong("");
    datLoiTaiKhoan("");
    datMoTaiKhoan({ kieu: "cap", canBo: cb });
  }, []);

  const moDatLaiMatKhau = useCallback((cb: identity_canBoTomTat) => {
    datDangMo(null);
    datLoiMayChu("");
    datCauDaXong("");
    datLoiTaiKhoan("");
    datMoTaiKhoan({ kieu: "datLai", canBo: cb, khoaChongTrung: khoaChongTrungMoi() });
  }, []);

  const dongXacNhanTaiKhoan = useCallback(() => {
    datMoTaiKhoan(null);
    datLoiTaiKhoan("");
  }, []);

  /**
   * Đóng ô mật khẩu tạm — HÀNH ĐỘNG DUY NHẤT xoá được giá trị ấy khỏi màn hình.
   *
   * Không có bộ đếm ngược, không có `setTimeout`, không có lần đọc lại danh sách nào chạm tới nó:
   * một ô tự biến mất sau 30 giây là ô biến mất đúng lúc cán bộ ở đầu dây bên kia hỏi lại.
   */
  const dongMatKhauTam = useCallback(() => datMatKhauTam(null), []);

  /**
   * Gửi một trong hai tuyến ghi thông tin đăng nhập.
   *
   * TÊN VÀ MÃ LẤY TỪ DÒNG NGƯỜI DÙNG VỪA BẤM, KHÔNG TỪ THÂN CÂU TRẢ LỜI. Hai lẽ, và lẽ thứ hai là
   * lẽ nặng: `kq.duLieu.staff` KHÔNG CHẮC CÓ MẶT — một lần phát lại theo khoá chống trùng trả đúng
   * mã 200 kèm thân `{"code":…,"replayed":true}`, vì `core/idem` cố ý không lưu thân câu trả lời
   * nào (`core/idem/idem.go:421`). Đọc `.staff.full_name` trên thân ấy là một `TypeError` ném ra
   * giữa một `then`, không ai bắt, và màn hình đứng im không nói gì.
   *
   * VÌ VẬY PHẢI KIỂM HÌNH DẠNG THÂN TRƯỚC KHI MỞ Ô. TypeScript ép kiểu thân JSON mà không kiểm gì
   * lúc chạy, nên `temporary_password` của một lần phát lại chỉ đơn giản là `undefined` — và nếu
   * không ai kiểm thì ô mật khẩu mở ra rỗng, hoặc đọc to hai chữ "undefined" cho một cán bộ đang
   * cầm bút.
   *
   * ĐỌC LẠI DANH SÁCH SAU KHI THÀNH CÔNG vì `has_account` vừa đổi ở nhánh `cap`, và cột Tài khoản
   * cùng cặp nút của dòng ấy đều đọc từ nó. Ô mật khẩu tạm được dựng NGOÀI mọi nhánh của
   * `trangThai`, nên một lần đọc lại hỏng cũng không xoá mất giá trị đang hiện.
   */
  const guiTaiKhoan = useCallback(() => {
    if (moTaiKhoan === null || dangGuiTaiKhoan) return;

    const canBo = moTaiKhoan.canBo;
    const kieu = moTaiKhoan.kieu;

    datLoiTaiKhoan("");
    datDangGuiTaiKhoan(true);

    const goi =
      moTaiKhoan.kieu === "cap"
        ? capTaiKhoan(canBo.id)
        : datLaiMatKhau(canBo.id, moTaiKhoan.khoaChongTrung);

    void goi
      .then((kq) => {
        // Câu của máy chủ ra nguyên văn — 409 của tuyến cấp nghĩa là người này ĐÃ có tài khoản, và
        // câu ấy do máy chủ viết. Ca "không có câu trả lời nào" được `XacNhanTaiKhoan` nói thêm.
        if (!kq.ok) {
          datLoiTaiKhoan(kq.thongBao);
          return;
        }

        const matKhau = kq.duLieu.temporary_password;
        if (typeof matKhau !== "string" || matKhau === "") {
          datLoiTaiKhoan(CAU_PHAT_LAI_KHONG_CO_MAT_KHAU);
          return;
        }

        datMoTaiKhoan(null);
        datMatKhauTam({ kieu, maCanBo: canBo.code, hoTen: canBo.full_name, matKhau });
        idDangDoi.current = null;
        datChiTiet(null);
        datLanDoc((n) => n + 1);
      })
      .finally(() => datDangGuiTaiKhoan(false));
  }, [dangGuiTaiKhoan, moTaiKhoan]);

  /**
   * Sáu hành động của một dòng. Gom vào MỘT đối tượng để `BangCanBo` nhận đúng một tham số thay
   * vì sáu — và để không ai thêm được một hành động thứ bảy mà không đi qua chỗ này.
   */
  const thaoTac = useMemo<ThaoTacDong>(
    () => ({
      chiTiet: (id) => void moChiTiet(id),
      sua: (cb) => moBieuMau({ kieu: "sua", canBo: cb }),
      doiVaiTro: (cb) => moBieuMau({ kieu: "vaiTro", canBo: cb }),
      datKhoa: (cb) => moBieuMau({ kieu: "khoa", canBo: cb, khoa: cb.active }),
      capTaiKhoan: moCapTaiKhoan,
      datLaiMatKhau: moDatLaiMatKhau,
    }),
    [moBieuMau, moChiTiet, moCapTaiKhoan, moDatLaiMatKhau],
  );

  // Tiêu đề biểu mẫu ghi — chính câu `BieuMauGhiCanBo` dùng cho `aria-label` và `<h4>` của nó; trong hộp
  // thoại, `<h4>` ấy được ẩn (`.hop-thoai-cau-hinh`) và câu này thành tiêu đề của hộp.
  const tieuDeBieuMau =
    dangMo === null
      ? ""
      : dangMo.kieu === "them"
        ? tieuDeThem()
        : dangMo.kieu === "sua"
          ? tieuDeSua(dangMo.canBo.full_name)
          : dangMo.kieu === "vaiTro"
            ? tieuDeVaiTro(dangMo.canBo.full_name)
            : tieuDeKhoa(dangMo.canBo.full_name, dangMo.khoa);

  return (
    // THE PROTOTYPE'S COMPOSITION (`UserWorkspace` + `UserTable`, ADR 0068 lần 5): under the page header,
    // one filter row with `Thêm cán bộ` at its right end, then the bordered table, then the count. Every
    // write opens in a centred dialog, as the prototype's `UserFormDialog` does.
    <section className="tab-nguoi-dung flex min-w-0 flex-col gap-3 [&>*]:my-0" aria-label="Danh sách người dùng">
      {/*
        ĐẶC TẢ CÓ, Ở ĐÂY KHÔNG — và mỗi dòng nói luôn cái gì mở khoá nó:

          · Ô tìm và bộ lọc theo bộ phận: ĐÃ DỰNG (26/09/2026) — `HangLocNguoiDung` dưới đây, đi
            qua `docTrangDanhBa` như màn `/danh-ba`. Chữ gợi ý KHÔNG theo nguyên văn đặc tả/prototype
            ("Tìm theo tên, thư điện tử, bộ phận…"): máy chủ tìm trên họ tên, chức danh và hai số
            điện thoại (`store/can_bo_danh_sach.go`, `menhDeLocCanBo`), KHÔNG trên thư điện tử;
            bộ phận là ô chọn riêng. Hứa tìm theo thư điện tử là để cán bộ gõ một địa chỉ, thấy
            danh sách rỗng, và kết luận người đó không có trong hệ thống.
          · Sắp xếp khi đang tìm: tuyến tìm không nhận `sort`/`order`, nên `ThanhSapXep` nói ra điều
            ấy và hai đầu cột thôi là nút.
          · Bộ lọc theo trạng thái (đang hoạt động / đã khoá): `LocCanBo` chỉ có bộ phận và công
            khai — hợp đồng chưa có tham số trạng thái.
          · `Nhập từ Excel`: ĐÃ DỰNG (29/09/2026, ADR 0059 §1) — nút ở đầu trang (`TabNguoiDung`),
            hộp thoại ở đây; mật khẩu tạm của lượt nhập hiện một lần ở `staff-import-result.tsx`.
          · `Xuất Excel`: không có tuyến nào trong hợp đồng. Bản xuất còn kéo theo một quyết định
            chưa có: #11 chốt KHÔNG che số trên màn hình nội bộ nhưng VẪN CHE ở bản xuất.
          · `Ảnh đại diện`: không có cột nào trong lược đồ.
          · Ô `Hiện trên Mini App` (#12) và `Xoá khỏi danh bạ` (prototype: `Trash2` ở mỗi dòng; quyền
            `admin.user.delete`): CỐ Ý đặt ở màn `/danh-ba`, không ở đây — xem `VI_SAO_KHONG_CO_NUT_XOA`.
      */}
      <div className="flex min-w-0 flex-wrap items-center gap-3">
        <HangLocNguoiDung loc={loc} boPhan={mucBoPhan} doiLoc={doiLoc} />
        <Button
          type="button"
          variant="primary"
          className="ml-auto"
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          aria-haspopup="dialog"
          onClick={() => moBieuMau({ kieu: "them", khoaChongTrung: khoaChongTrungMoi() })}
        >
          {NUT_THEM_CAN_BO}
        </Button>
      </div>
      <ThanhSapXep khoa={khoaSapXep} chieu={chieu} doiSapXep={dangTim ? null : doiSapXep} />

      {/* Câu xác nhận sau một lần ghi. `role="status"` chứ không `alert`: không có gì hỏng. */}
      {cauDaXong !== "" && (
        <p role="status" className="m-0 text-sm font-medium text-success-600">
          {cauDaXong}
        </p>
      )}

      {/*
        THE DIALOGS. Each one is OUTSIDE every branch of `trangThai`, and that place is a condition, not
        an order of presentation: a successful write re-reads the list, and a re-read that fails replaces
        the table with an error line — it must not take an open dialog (or a password) with it.
      */}

      {/* Staff import (`asDialog`). Esc does nothing while sending, nor after a success that shows
          passwords (`excel-import-panel.tsx`); only the panel's own close calls `onImportClose`. */}
      {importOpen && (
        <ExcelImportPanel
          target={STAFF_IMPORT_TARGET}
          onImported={() => {
            idDangDoi.current = null;
            datChiTiet(null);
            datLanDoc((n) => n + 1);
          }}
          onClose={onImportClose}
          asDialog
        />
      )}

      {/* MỘT BIỂU MẪU, MỘT HỘP THOẠI. Tiêu đề hộp luôn gọi tên người đang được thao tác, nên không có ca
          nào sửa nhầm hồ sơ vì không biết biểu mẫu thuộc về dòng nào. Esc huỷ, trừ lúc đang gửi. */}
      {dangMo !== null && (
        <ConfigDialog
          title={tieuDeBieuMau}
          onDismiss={() => {
            if (!dangGui) dongBieuMau();
          }}
        >
          <BieuMauGhiCanBo
            dangMo={dangMo}
            ban={ban}
            datBan={datBan}
            vaiTroID={vaiTroID}
            datVaiTroID={datVaiTroID}
            boPhan={mucBoPhan}
            vaiTro={mucVaiTro}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={guiBieuMau}
            onHuy={dongBieuMau}
          />
        </ConfigDialog>
      )}

      {moTaiKhoan !== null && (
        <ConfigDialog
          title={tieuDeXacNhan(moTaiKhoan)}
          hideHeader
          onDismiss={() => {
            if (!dangGuiTaiKhoan) dongXacNhanTaiKhoan();
          }}
        >
          <XacNhanTaiKhoan
            dangMo={moTaiKhoan}
            loiMayChu={loiTaiKhoan}
            dangGui={dangGuiTaiKhoan}
            onGui={guiTaiKhoan}
            onHuy={dongXacNhanTaiKhoan}
          />
        </ConfigDialog>
      )}

      {/* Ô MẬT KHẨU TẠM — a dialog Esc does NOT close (`onDismiss` does nothing): the value is shown once
          and cannot be shown again, so only the explicit "Tôi đã ghi lại" removes it. */}
      {matKhauTam !== null && (
        <ConfigDialog title="Mật khẩu tạm — chỉ hiện một lần" hideHeader onDismiss={() => {}}>
          <OMatKhauTam matKhauTam={matKhauTam} onDong={dongMatKhauTam} />
        </ConfigDialog>
      )}

      {chiTiet !== null && (
        <KhoiChiTiet chiTiet={chiTiet} dong={dongChiTiet} traBoPhan={traBoPhan} traVaiTro={traVaiTro} />
      )}

      {/* FIRST LOAD (spec §8b): the sentence stays the live region, read out as before; the eye gets
          row-shaped placeholders. */}
      {trangThai.pha === "dangTai" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải danh sách…
          </p>
          <SkeletonRows rows={5} />
        </>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải. Mọi mã lỗi — kể cả 401, 403, 404 — đều
          trả cùng hình dạng `httpx.Error`, nên không có chỗ nào ở đây rẽ nhánh theo `code`, và
          `trace_id` không hiện ra (xem `lib/api/goi.ts`). */}
      {trangThai.pha === "loi" && (
        <ErrorState role="alert" title="Chưa tải được danh sách cán bộ" message={trangThai.thongBao} />
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length === 0 && (
        // TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI. Một xã vừa onboard có danh bạ rỗng thật; máy chủ
        // trả `items: []` chứ không bao giờ trả `null`. Khi đang tìm hay lọc thì câu ấy là SAI: "không
        // ai khớp" không phải "xã chưa có ai".
        <EmptyState
          icon={loc.tuKhoa !== null || loc.boPhan !== "" ? Search : UsersRound}
          title={
            loc.tuKhoa !== null || loc.boPhan !== ""
              ? KHONG_KHOP_LOC
              : "Đơn vị chưa có cán bộ nào trong danh bạ. Khi cán bộ được thêm vào, danh sách sẽ hiện ở đây."
          }
        />
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length > 0 && (
        <>
          {/* DANH MỤC HỎNG THÌ NÓI RA MỘT LẦN Ở ĐÂY, chứ không để hai mươi ô cùng báo lỗi — đúng
              `message` của máy chủ, mỗi danh mục một dòng, chỉ khi có bảng để mà thiếu cột. */}
          {(traBoPhan.pha === "loi" || traVaiTro.pha === "loi") && (
            <div className="flex flex-col gap-2 [&>*]:my-0">
              <BaoLoiDanhMuc nhan="Danh mục bộ phận" bang={traBoPhan} />
              <BaoLoiDanhMuc nhan="Danh mục vai trò" bang={traVaiTro} />
            </div>
          )}
          <Card className="border border-line">
            <BangCanBo
              danhSach={trangThai.trang.items}
              khoa={khoaHien}
              chieu={chieuHien}
              doiSapXep={dangTim ? null : doiSapXep}
              thaoTac={thaoTac}
              traBoPhan={traBoPhan}
              traVaiTro={traVaiTro}
            />
          </Card>
          {/* The prototype's footer line ("Hiển thị N/M cán bộ."). No "/M": the contract pages by cursor
              and returns no total (`DieuHuongTrang`), so the count is this page's rows, said as such. */}
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
            <p className="m-0 text-[13px] text-ink-500">
              Hiển thị {trangThai.trang.items.length} cán bộ trên trang này.
            </p>
            <DieuHuongTrang
              nganXep={nganXep}
              conTroTiep={trangThai.trang.next_cursor}
              conTrangSau={trangThai.trang.has_more}
              diToiTrang={diToiTrang}
            />
          </div>
          {/* CÂU NGHỊ ĐỊNH 13 GIỮ NGUYÊN: hai cột số điện thoại in NGUYÊN VẸN (#11, 22/09/2026), và cán
              bộ đọc câu cạnh bảng trước khi chụp màn hình gửi đi. Một lời khai sai về mức bảo vệ dữ liệu
              cá nhân, ngay cạnh dữ liệu chưa che, nguy hiểm hơn không nói gì. */}
          <Notice tone="legal" className="ghi-chu m-0 min-w-0">
            Số điện thoại hiển thị đầy đủ cho cán bộ trong xã. Đây là dữ liệu cá nhân theo Nghị
            định 13/2023/NĐ-CP — không sao chép ra ngoài cơ quan.
          </Notice>
          <p className="ghi-chu m-0 text-[13px] text-ink-500">{VI_SAO_KHONG_CO_NUT_XOA}</p>
        </>
      )}
    </section>
  );
}

/**
 * Một dòng báo khi không đọc được một danh mục.
 *
 * KHÔNG dựng gì khi danh mục đang đọc hoặc đã đọc xong: một chỗ trống dành sẵn cho thông báo
 * lỗi là một chỗ trống nhảy chữ vào giữa lúc người dùng đang đọc bảng.
 */
function BaoLoiDanhMuc({ nhan, bang }: { nhan: string; bang: BangTraDanhMuc }) {
  if (bang.pha !== "loi") return null;
  return (
    <p className="thong-bao-loi" role="alert">
      {nhan}: {bang.thongBao}
    </p>
  );
}

/**
 * Một ô tra danh mục — cột Bộ phận và cột Vai trò dùng chung.
 *
 * KHÔNG BAO GIỜ DỰNG RA MỘT Ô TRỐNG: `nhan` luôn trả một câu, kể cả khi không tra được. Lớp CSS
 * đi theo LOẠI kết quả chứ không theo câu chữ, để "chưa gán" (một trạng thái bình thường) và
 * "không tra được" (một dòng dữ liệu lệch) không trông giống nhau.
 */
function ODanhMuc({ ket, nhan }: { ket: KetTra; nhan: (ket: KetTra) => string }) {
  const chu = nhan(ket);
  return (
    <span className={`block max-w-[14rem] overflow-hidden text-ellipsis ${lopNhanDanhMuc(ket) ?? ""}`} title={chu}>
      {chu}
    </span>
  );
}

/** One line of free text with "…" and the full words on hover — `Chức danh` and the like. */
function OMotDong({ text }: { text: string }) {
  return (
    <span className="block max-w-[14rem] overflow-hidden text-ellipsis" title={text}>
      {text}
    </span>
  );
}

function lopNhanDanhMuc(ket: KetTra): string | undefined {
  switch (ket.loai) {
    case "coTen":
      return undefined;
    case "khongTraDuoc":
      return "nhan-lech";
    default:
      return "nhan-trong";
  }
}

/**
 * Sorting control above the table. Since ADR 0068 lần 5 the sortable column heads (`Mã cán bộ`,
 * `Ngày tạo`, the prototype's `ArrowUpDown` heads) are the ONLY place to sort, so this bar no longer draws
 * buttons: with `doiSapXep` given it renders nothing. CHỈ HAI KHOÁ, và đó là toàn bộ những gì máy chủ
 * nhận (xem `KHOA_SAP_XEP`).
 *
 * `doiSapXep === null` LÀ ĐANG TÌM: tuyến tìm không có tham số sắp xếp và luôn trả theo mã tăng
 * dần, nên thanh này NÓI RA điều ấy thay vì để hai đầu cột mời bấm mà không có gì xảy ra — hay tệ
 * hơn, một mũi tên "Ngày tạo ↓" nằm trên một bảng đang xếp theo mã.
 */
export function ThanhSapXep({
  doiSapXep,
}: {
  khoa: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: ((khoa: KhoaSapXep) => void) | null;
}) {
  if (doiSapXep !== null) return null;
  return <p className="ghi-chu m-0 text-[13px] text-ink-500">{CAU_SAP_XEP_KHI_TIM}</p>;
}

/** Nhãn người đọc của hai khoá sắp xếp. Khoá là chuỗi của hợp đồng, nhãn là chữ của đặc tả. */
const NHAN_KHOA: Record<KhoaSapXep, string> = {
  code: "Mã cán bộ",
  created_at: "Ngày tạo",
};

/** Câu thay cho thanh sắp xếp khi đang tìm. Nói cả thứ tự thật lẫn cách lấy lại quyền chọn. */
export const CAU_SAP_XEP_KHI_TIM =
  "Kết quả tìm kiếm được sắp xếp theo mã cán bộ, tăng dần. Xoá nội dung ô tìm để sắp xếp theo cột khác.";

/* ---- ô tìm và bộ lọc bộ phận ---------------------------------------------------------------- */

/**
 * Bộ lọc của tab Người dùng — HAI trong ba trường của `LocDanhBa` (màn `/danh-ba`).
 *
 * KHÔNG CÓ `hienThi` (công khai trên Mini App), và đó là quyết định chứ không thiếu sót: bộ lọc và
 * ô công khai ở màn `/danh-ba`, không ở đây. Cũng không có bộ lọc trạng thái hoạt động — hợp đồng
 * không có tham số ấy (`LocCanBo`).
 */
export type LocNguoiDung = Pick<LocDanhBa, "tuKhoa" | "boPhan">;

export const LOC_DAU: LocNguoiDung = { tuKhoa: null, boPhan: "" };

export const NHAN_O_TIM_NGUOI_DUNG = "Tìm người dùng";
export const NHAN_LOC_BO_PHAN = "Bộ phận";
export const TAT_CA_BO_PHAN = "Tất cả bộ phận";

/**
 * Hàng lọc — ô tìm và ô chọn bộ phận, kết hợp theo AND. Mọi phép quyết định là của `loc-danh-ba.ts`
 * (`ketQuaGuiTim`, `maBoPhanLoc`) — cùng phép màn `/danh-ba` dùng, không chép lại.
 *
 * GỬI BẰNG SUBMIT (Enter hoặc nút Tìm), KHÔNG THEO TỪNG PHÍM: mỗi phím là một lời gọi mạng mang chữ
 * gõ dở. Ngoại lệ duy nhất: ô vừa bị xoá TRẮNG trong lúc đang có một lần tìm → bỏ tìm ngay, vì để
 * trên màn hình kết quả của một chữ không còn thấy ở đâu là để người dùng đọc sai danh sách.
 *
 * Ô NHẬP KHÔNG CÓ `name`, FORM LÀ `method="post"`, `autoComplete="off"`: cùng ba lớp với `HangLoc`
 * của `/danh-ba` — trước khi JavaScript chạy xong, Enter trong một form GET đưa mọi ô có `name` lên
 * URL; và trình duyệt nhớ những gì đã gõ để gợi ý cho người dùng sau trên máy dùng chung (luật 3,
 * cấm #4). Không có `maxLength`: thuộc tính ấy đếm đơn vị UTF-16, không đếm ký tự.
 */
export function HangLocNguoiDung({
  loc,
  boPhan,
  doiLoc,
}: {
  loc: LocNguoiDung;
  boPhan: readonly MucChon[];
  doiLoc: (doi: Partial<LocNguoiDung>) => void;
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

  function go(giaTri: string) {
    datOTim(giaTri);
    if (giaTri.trim() === "" && loc.tuKhoa !== null) {
      datLoiTim("");
      doiLoc({ tuKhoa: null });
    }
  }

  return (
    // THE PROTOTYPE'S FILTER ROW (`UserTable`, ADR 0068 lần 5): a search box with the magnifier inside
    // it and no visible label, ~28rem at most. The labels stay — native `<label>`s, visually hidden — in
    // this component's own tree: the tab's test reads the unrendered element tree for "both controls
    // are labelled", and a screen reader needs them.
    //
    // KEPT, THOUGH THE PROTOTYPE HAS NEITHER: the `Tìm` button (this search runs on submit, not per key —
    // a box that does nothing while typing needs a visible way to run it) and the Bộ phận picker.
    <div className="hang-loc-nguoi-dung m-0 flex min-w-0 flex-[1_1_480px] flex-wrap items-center gap-2">
      <form
        className="m-0 flex min-w-0 flex-[1_1_300px] items-center gap-2 sm:max-w-[28rem]"
        role="search"
        method="post"
        onSubmit={gui}
      >
        <label htmlFor="tim-nguoi-dung" className="an-thi-giac">
          {NHAN_O_TIM_NGUOI_DUNG}
        </label>
        <span className="relative min-w-0 flex-1">
          <Search
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-500"
          />
          <input
            id="tim-nguoi-dung"
            type="search"
            className={cn(controlClass, "pl-9")}
            value={oTim}
            onChange={(e) => go(e.target.value)}
            placeholder={GOI_Y_O_TIM}
            autoComplete="off"
            aria-describedby="loi-tim-nguoi-dung"
            aria-invalid={loiTim !== ""}
          />
        </span>
        <Button variant="secondary" type="submit" className="shrink-0">
          {NUT_TIM}
        </Button>
      </form>

      <label htmlFor="loc-bo-phan-nguoi-dung" className="an-thi-giac">
        {NHAN_LOC_BO_PHAN}
      </label>
      <select
        id="loc-bo-phan-nguoi-dung"
        className={cn(controlClass, "w-auto max-w-[16rem] min-w-[11rem] flex-[0_1_14rem]")}
        value={loc.boPhan}
        onChange={(e) => doiLoc({ boPhan: maBoPhanLoc(e.target.value, boPhan) })}
      >
        <option value="">{TAT_CA_BO_PHAN}</option>
        {boPhan.map((bp) => (
          <option key={bp.id} value={bp.id}>
            {bp.name}
          </option>
        ))}
      </select>
      {/* Always in the DOM (the live region the search box points at); a full-width line of its own
          so a message never squeezes the row. */}
      <p id="loi-tim-nguoi-dung" className="thong-bao-loi m-0 basis-full empty:hidden" role="alert">
        {loiTim}
      </p>
    </div>
  );
}

/**
 * Sáu hành động một dòng danh bạ mở ra. **KHÔNG hành động nào tên là "Xoá".**
 *
 * Kiểu này là chỗ hẹp nhất mà một nút Xoá phải đi qua: thêm nó vào đây là thêm một trường vào một
 * kiểu có bài kiểm đọc lại, chứ không phải thêm một dòng JSX không ai thấy. Vì sao không có nó:
 * `VI_SAO_KHONG_CO_NUT_XOA`.
 *
 * Mọi hành động trừ `chiTiet` nhận CẢ DÒNG chứ không nhận `id`: biểu mẫu mở ra phải gọi tên người
 * đang được thao tác trên tiêu đề, `datKhoa` còn phải biết người ấy đang khoá hay chưa để chọn
 * đúng chiều, và ô mật khẩu tạm lấy tên với mã từ chính dòng ấy (xem `guiTaiKhoan`). Truyền `id`
 * rồi đi tìm lại dòng là mở đường cho một lần tìm ra dòng khác.
 *
 * `capTaiKhoan` VÀ `datLaiMatKhau` LÀ HAI TRƯỜNG, KHÔNG PHẢI MỘT TRƯỜNG MANG CỜ. Máy chủ tách
 * chúng bằng hai tuyến, hai mã thành công (201 và 200) và hai điều kiện loại trừ nhau trong mệnh
 * đề WHERE; một trường chung ở đây là chỗ giao diện gộp lại thứ máy chủ vừa tách.
 */
export type ThaoTacDong = {
  chiTiet: (id: string) => void;
  sua: (cb: identity_canBoTomTat) => void;
  doiVaiTro: (cb: identity_canBoTomTat) => void;
  datKhoa: (cb: identity_canBoTomTat) => void;
  capTaiKhoan: (cb: identity_canBoTomTat) => void;
  datLaiMatKhau: (cb: identity_canBoTomTat) => void;
};

/**
 * Cụm nút của một dòng.
 *
 * MỖI NÚT MANG TÊN NGƯỜI TRONG `aria-label`. Hai mươi dòng cho ra hai mươi nút đọc lên giống hệt
 * nhau là danh sách mà người dùng trình đọc màn hình không chọn đúng được dòng nào — và ở đây
 * chọn nhầm dòng nghĩa là khoá nhầm tài khoản của một cán bộ.
 *
 * KHOÁ HAY MỞ KHOÁ ĐỌC TỪ `active`, không phải từ một cờ riêng. `active` là `dang_hoat_dong` của
 * máy chủ, và nó cũng chính là thứ tuyến khoá/mở khoá ghi vào — nên nhãn nút không thể lệch với
 * việc nút ấy sắp làm.
 */
function NutCuaDong({ cb, thaoTac }: { cb: identity_canBoTomTat; thaoTac: ThaoTacDong }) {
  const nhanKhoa = cb.active ? NUT_KHOA : NUT_MO_KHOA;
  const ten = (nut: string) => `${nut}: ${cb.full_name}`;
  return (
    // THE PROTOTYPE'S ROW ACTIONS ARE ICON BUTTONS (`UserTable`: Pencil, Trash2), one line, right-aligned —
    // so every row keeps one height. `IconButton` makes the name both `aria-label` and the hover `title`,
    // and the name carries the person (see above). 44px squares: older staff tap these on a phone
    // (`.o-thao-tac .nut-phu`, `skills/accessibility-elderly`).
    <span className="o-thao-tac o-thao-tac-icon">
      <IconButton className="size-11" label={ten("Chi tiết")} type="button" aria-haspopup="dialog" onClick={() => thaoTac.chiTiet(cb.id)}>
        <Eye aria-hidden="true" focusable="false" strokeWidth={1.8} />
      </IconButton>
      <IconButton className="size-11" label={ten(NUT_SUA)} type="button" aria-haspopup="dialog" onClick={() => thaoTac.sua(cb)}>
        <Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />
      </IconButton>
      <IconButton className="size-11" label={ten(NUT_DOI_VAI_TRO)} type="button" aria-haspopup="dialog" onClick={() => thaoTac.doiVaiTro(cb)}>
        <UserCog aria-hidden="true" focusable="false" strokeWidth={1.8} />
      </IconButton>
      <IconButton className="size-11" label={ten(nhanKhoa)} type="button" aria-haspopup="dialog" onClick={() => thaoTac.datKhoa(cb)}>
        {cb.active ? (
          <Lock aria-hidden="true" focusable="false" strokeWidth={1.8} />
        ) : (
          <LockOpen aria-hidden="true" focusable="false" strokeWidth={1.8} />
        )}
      </IconButton>

      {/*
        MỘT NÚT, KHÔNG HAI — và nút kia VẮNG MẶT chứ không mờ đi.

        `has_account` là hai thế giới loại trừ nhau, không phải hai trạng thái của một việc: máy
        chủ tách chúng ngay trong mệnh đề WHERE (`AND NOT co_tai_khoan` cho tuyến cấp, và tuyến đặt
        lại chỉ có nghĩa khi tài khoản đã tồn tại), nên không nút nào làm được việc của nút kia.

        Một nút mờ đi mời người dùng hỏi "vì sao không bấm được" và đi tìm một quyền họ không
        thiếu; một nút vắng mặt nói đúng điều đang đúng — việc ấy không áp dụng cho dòng này.
      */}
      {cb.has_account ? (
        <IconButton className="size-11" label={ten(NUT_DAT_LAI_MAT_KHAU)} type="button" aria-haspopup="dialog" onClick={() => thaoTac.datLaiMatKhau(cb)}>
          <KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />
        </IconButton>
      ) : canIssueAccount(cb.email) ? (
        <IconButton className="size-11" label={ten(NUT_CAP_TAI_KHOAN)} type="button" aria-haspopup="dialog" onClick={() => thaoTac.capTaiKhoan(cb)}>
          <UserPlus aria-hidden="true" focusable="false" strokeWidth={1.8} />
        </IconButton>
      ) : (
        // NO EMAIL, NO ACCOUNT: disabled, tied by `aria-describedby` to the reason, which is VISIBLE
        // text in the row's "Tài khoản" cell (`OTaiKhoan`). Unlike the pair above, this is the SAME
        // action blocked by a fixable gap, so the person must learn what to fix. The server still
        // refuses (409 `staff_has_no_email`); if that sentence ever arrives, `guiTaiKhoan` shows it.
        <IconButton className="size-11" label={ten(NUT_CAP_TAI_KHOAN)} type="button" aria-describedby={`no-email-reason-${cb.id}`} disabled>
          <UserPlus aria-hidden="true" focusable="false" strokeWidth={1.8} />
        </IconButton>
      )}
    </span>
  );
}

/**
 * Ô "Tài khoản": has / has not, and — for a row that cannot get one yet — the reason, one line with "…"
 * and the full sentence on hover and for the screen reader (`aria-describedby` of the disabled button).
 */
function OTaiKhoan({ cb }: { cb: identity_canBoTomTat }) {
  const khongCoThu = !cb.has_account && !canIssueAccount(cb.email);
  return (
    <>
      <span className="block">{nhanTaiKhoan(cb.has_account)}</span>
      {khongCoThu && (
        <span className="dong-phu max-w-[14rem] overflow-hidden text-ellipsis" title={NO_EMAIL_ACCOUNT_REASON} id={`no-email-reason-${cb.id}`}>
          {NO_EMAIL_ACCOUNT_REASON}
        </span>
      )}
    </>
  );
}

// EXPORTED SO THE TWO PHONE COLUMNS CAN BE PINNED BY A RENDER TEST. The parent reads the API in
// `useEffect`, which `renderToStaticMarkup` never runs, so rendering it proves nothing about a row.
// The property being pinned is not cosmetic: merging these two back into one column is a one-line
// edit that no existing test sees, and it would put duty information and Decree 13 personal data
// under one label — see the header comment on the columns.
export function BangCanBo({
  danhSach,
  khoa,
  chieu,
  doiSapXep,
  thaoTac,
  traBoPhan,
  traVaiTro,
}: {
  danhSach: readonly identity_canBoTomTat[];
  khoa: KhoaSapXep;
  chieu: ChieuSapXep;
  /** `null` = đang tìm: hai ô tiêu đề hiện chữ, không hiện nút (xem `ThanhSapXep`). */
  doiSapXep: ((khoa: KhoaSapXep) => void) | null;
  thaoTac: ThaoTacDong;
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số. Không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  traVaiTro: BangTraDanhMuc;
}) {
  return (
    // `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím. Ở dưới 768px bảng
    // cuộn ngang chứ không đổi thành thẻ: đổi `display` của các phần tử bảng làm mất ngữ nghĩa
    // bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng.
    // The prototype's table box: it scrolls sideways INSIDE itself, never the page. Cells stay one line
    // (`.bang-can-bo td` is `nowrap`); free text is capped with "…" and the full words on hover, so every
    // row has one height and nothing spills into the next column.
    <TableScroll sticky aria-label="Danh sách cán bộ" className="rounded-none border-0 shadow-none">
      <table className={`bang-can-bo ${DATA_TABLE_CLASS}`}>
        <caption className="an-thi-giac">
          Danh sách cán bộ của đơn vị, sắp xếp theo {NHAN_KHOA[khoa].toLowerCase()}{" "}
          {chieu === "asc" ? "tăng dần" : "giảm dần"}
        </caption>
        <thead>
          <tr>
            <OTieuDeSapXep khoa="code" khoaHienTai={khoa} chieu={chieu} doiSapXep={doiSapXep} />
            <th scope="col">Họ và tên</th>
            <th scope="col">Chức danh</th>
            {/*
              HAI CỘT NÀY TRA TỪ DANH MỤC, KHÔNG HIỆN ID. Hợp đồng trả `department_id` và
              `role_id` là ULID; một ULID trên màn hình là một chuỗi vô nghĩa với cán bộ. Tên
              lấy từ `GET /api/v1/org-units` và `GET /api/v1/roles`, đọc một lần cho cả màn
              hình, và mọi ca không tra được đều có câu chữ riêng (`tra-danh-muc.ts`).

              Cột `Vai trò` không nằm trong bảng cột của đặc tả §3 — §3 chỉ liệt `Bộ phận` — mà
              đến từ ô `Vai trò` của form người dùng ngay dưới đó và từ lý do phân quyền của
              chính tuyến `GET /api/v1/roles` ("cột Vai trò của danh bạ"). Nêu ra vì đây là chỗ
              màn hình NHIỀU hơn bảng cột của đặc tả, ngược với phần còn lại của tệp này.
            */}
            <th scope="col">Bộ phận</th>
            <th scope="col">Vai trò</th>
            {/* HAI CỘT, KHÔNG MỘT — và nhãn phải nói rõ loại nào, không phải "Điện thoại" trung
                tính. Câu mở #16, chốt 22/09/2026: máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá
                nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai địa vị pháp lý khác nhau nghĩa là hai
                luật che, hai luật xuất Excel, hai luật công khai ra Mini App.

                Một nhãn trung tính là chỗ người sắp bấm nút xuất, hay sắp tick ô công khai, không
                biết mình đang đụng loại nào — và đó là lúc một số di động cá nhân rời khỏi cơ quan
                mà không ai định làm thế. Cột này trước 22/09 hiện số CƠ QUAN dưới nhãn trung tính
                ấy, tức nó còn mập mờ theo cả chiều ngược lại. */}
            <th scope="col">Máy bàn cơ quan</th>
            <th scope="col">Di động cá nhân</th>
            <th scope="col">Đăng nhập gần nhất</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Tài khoản</th>
            <OTieuDeSapXep
              khoa="created_at"
              khoaHienTai={khoa}
              chieu={chieu}
              doiSapXep={doiSapXep}
            />
            <th scope="col" className="text-right">
              <span className="an-thi-giac">Hành động</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {danhSach.map((cb) => (
            <tr key={cb.id}>
              <td>{cb.code}</td>
              <td>
                <span className="ten-can-bo max-w-[16rem] overflow-hidden text-ellipsis" title={cb.full_name}>
                  {cb.full_name}
                </span>
                <span className="dong-phu">{emailLabel(cb.email)}</span>
              </td>
              <td>
                <OMotDong text={cb.position} />
              </td>
              <td>
                <ODanhMuc ket={traTen(traBoPhan, cb.department_id)} nhan={nhanBoPhan} />
              </td>
              <td>
                <ODanhMuc ket={traTen(traVaiTro, cb.role_id)} nhan={nhanVaiTro} />
              </td>
              {/* HAI Ô RIÊNG. Không gộp bằng `phone || mobile` và không nối bằng dấu phẩy: một ô
                  chứa hai loại số là ô mà mọi luật che, luật xuất và luật công khai về sau phải áp
                  CHUNG một mức cho hai thứ có địa vị pháp lý khác nhau — và mức an toàn buộc lấy
                  theo loại nhạy hơn, tức số máy bàn của cơ quan cũng bị che vô cớ.

                  HIỆN NGUYÊN VĂN thứ máy chủ trả, không định dạng lại thành `0900 000 001` như ví
                  dụ trong đặc tả. Câu chú thích cũ ở đây nói `phone` "LUÔN về đây đã che" — nay
                  SAI: #11 chốt 22/09/2026 là không che trong nội bộ xã, và máy chủ trả số nguyên
                  vẹn (`soRaManHinhNoiBo`). Việc che còn nguyên ở bản xuất Excel và ở mọi đường ra
                  ngoài cơ quan, hai bề mặt chưa tồn tại. */}
              <td>{cb.phone}</td>
              <td>{cb.mobile}</td>
              <td>{nhanDangNhapGanNhat(cb.last_login_at)}</td>
              <td>
                {/* Tone by the CODE (`active`), never by the words; icon + word, never colour alone. */}
                <Badge tone={cb.active ? "success" : "neutral"}>{nhanTrangThai(cb.active)}</Badge>
              </td>
              <td>
                <OTaiKhoan cb={cb} />
              </td>
              <td>{nhanNgayTao(cb.created_at)}</td>
              <td className="text-right">
                <NutCuaDong cb={cb} thaoTac={thaoTac} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** Ô tiêu đề của một cột sắp xếp được. `aria-sort` để trình đọc màn hình đọc đúng chiều. */
function OTieuDeSapXep({
  khoa,
  khoaHienTai,
  chieu,
  doiSapXep,
}: {
  khoa: KhoaSapXep;
  khoaHienTai: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: ((khoa: KhoaSapXep) => void) | null;
}) {
  const dangSapXep = khoa === khoaHienTai;
  const mui = dangSapXep ? (chieu === "asc" ? " ↑" : " ↓") : "";
  return (
    <th
      scope="col"
      aria-sort={dangSapXep ? (chieu === "asc" ? "ascending" : "descending") : "none"}
    >
      {doiSapXep === null ? (
        // Đang tìm: chữ thường, không nút. Mũi tên chỉ còn ở cột thật sự đang xếp (mã, tăng dần).
        <>
          {NHAN_KHOA[khoa]}
          {mui}
        </>
      ) : (
        // The prototype's sortable head: the words, then `ArrowUpDown` while unsorted, an arrow once sorted.
        <button type="button" className="nut-sap-xep inline-flex items-center gap-1.5" onClick={() => doiSapXep(khoa)}>
          {NHAN_KHOA[khoa]}
          {mui === "" ? (
            <ArrowUpDown aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3 opacity-40" />
          ) : (
            mui
          )}
        </button>
      )}
    </th>
  );
}

/**
 * Phân trang theo con trỏ.
 *
 * KHÔNG CÓ SỐ TRANG VÀ KHÔNG CÓ TỔNG SỐ, và đó không phải thiếu sót: hợp đồng trả `next_cursor`
 * + `has_more` chứ không trả `total`, vì máy chủ đọc theo mốc và cố ý không chạy `COUNT(*)` trên
 * bảng đã phân mảnh. Hiện "Trang 3/12" ở đây là báo một con số không ai tính (`core/page`).
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
  // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
  // Bấm khi con trỏ rỗng thì `sangTrangSau` ném lỗi — nút phải mờ đi trước khi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang" aria-label="Phân trang danh sách cán bộ">
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

/**
 * Chi tiết một cán bộ — `GET /api/v1/staff/{id}`.
 *
 * VÌ SAO KHÔNG PHẢI MỘT ĐƯỜNG DẪN RIÊNG `/…/{id}`: tên tài nguyên URL cho khái niệm "cán bộ"
 * CHƯA ĐƯỢC KHÁCH CHỐT (`kb/00-foundation/ubiquitous-language.md` — ô "Tài nguyên URL" của dòng
 * Cán bộ ghi rõ "CHƯA CHỐT — HỎI KHÁCH", và đường dẫn `/cau-hinh/nguoi-dung` của đặc tả cũ
 * không dùng nữa). Một đường dẫn đã chạy thật thì không sửa lại được, nên ở đây không đặt ra
 * đoạn đường dẫn nào cả; khối chi tiết mở ngay trong trang.
 *
 * BỘ PHẬN VÀ VAI TRÒ HIỆN Ở ĐÂY BẰNG CHÍNH BẢNG TRA CỦA BẢNG DANH SÁCH, không đọc lại danh mục
 * khi mở khối này: dữ liệu đã nằm trong tay màn hình rồi, và một lời gọi nữa ở đây chỉ thêm một
 * câu trả lời thứ hai có thể lệch với câu đang hiện trên dòng ngay phía trên.
 */
function KhoiChiTiet({
  chiTiet,
  dong,
  traBoPhan,
  traVaiTro,
}: {
  chiTiet: TrangThaiChiTiet;
  dong: () => void;
  traBoPhan: BangTraDanhMuc;
  traVaiTro: BangTraDanhMuc;
}) {
  return (
    // A dialog since ADR 0068 lần 5, like every other per-row view of the redesigned screen; Esc and
    // "Đóng" both close it — it holds nothing that cannot be read again.
    <ConfigDialog title="Chi tiết cán bộ" onDismiss={dong}>

      {chiTiet.pha === "dangTai" && <p role="status">Đang tải…</p>}

      {chiTiet.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {chiTiet.thongBao}
        </p>
      )}

      {chiTiet.pha === "xong" && (
        <dl className="danh-sach-truong">
          <dt>Mã cán bộ</dt>
          <dd>{chiTiet.canBo.code}</dd>
          <dt>Họ và tên</dt>
          <dd>{chiTiet.canBo.full_name}</dd>
          <dt>Thư điện tử</dt>
          <dd>{emailLabel(chiTiet.canBo.email)}</dd>
          <dt>Chức danh</dt>
          <dd>{chiTiet.canBo.position}</dd>
          <dt>Bộ phận</dt>
          <dd>
            <ODanhMuc ket={traTen(traBoPhan, chiTiet.canBo.department_id)} nhan={nhanBoPhan} />
          </dd>
          <dt>Vai trò</dt>
          <dd>
            <ODanhMuc ket={traTen(traVaiTro, chiTiet.canBo.role_id)} nhan={nhanVaiTro} />
          </dd>
          {/* Cùng lý do như hai cột của bảng: hai địa vị pháp lý khác nhau thì hai nhãn khác nhau,
              kể cả ở màn chi tiết nơi chỗ hiển thị không thiếu. */}
          <dt>Máy bàn cơ quan</dt>
          <dd>{chiTiet.canBo.phone}</dd>
          <dt>Di động cá nhân</dt>
          <dd>{chiTiet.canBo.mobile}</dd>
          <dt>Đăng nhập gần nhất</dt>
          <dd>{nhanDangNhapGanNhat(chiTiet.canBo.last_login_at)}</dd>
          <dt>Trạng thái</dt>
          <dd>{nhanTrangThai(chiTiet.canBo.active)}</dd>
          <dt>Tài khoản</dt>
          <dd>{nhanTaiKhoan(chiTiet.canBo.has_account)}</dd>
          <dt>Ngày tạo</dt>
          <dd>{nhanNgayTao(chiTiet.canBo.created_at)}</dd>
        </dl>
      )}
      <div className="flex justify-end">
        <Button type="button" variant="secondary" onClick={dong}>
          Đóng
        </Button>
      </div>
    </ConfigDialog>
  );
}
