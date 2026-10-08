"use client";

import {
  ArrowUpDown,
  KeyRound,
  Lock,
  LockOpen,
  Pencil,
  Plus,
  Search,
  Trash2,
  UserCog,
  UserPlus,
} from "lucide-react";
import { useCallback, useEffect, useId, useMemo, useState, type ReactNode } from "react";
import { toast } from "sonner";

import {
  BieuMauGhiCanBo,
  type DangMoGhi,
  type MucChon,
} from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import {
  ACCOUNT_EDIT_TITLE,
  ACCOUNT_SAVED,
  BAN_TRONG,
  GIAI_THICH_THEM,
  NUT_DOI_VAI_TRO,
  NUT_KHOA,
  NUT_MO_KHOA,
  NUT_THEM_CAN_BO,
  banTuCanBo,
  daDatKhoa,
  daDoiVaiTro,
  daThem,
  khoaChongTrungMoi,
  staffCodeNote,
  thanSua,
  thanThem,
  tieuDeKhoa,
  tieuDeThem,
  tieuDeVaiTro,
  type BanNhapCanBo,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  datKhoaCanBo,
  docTrangDanhBa,
  doiVaiTroCanBo,
  getStaffCounts,
  suaCanBo,
  themCanBo,
  xoaCanBo,
} from "@/lib/api/can-bo";
import { ketQuaGuiTim, type LocDanhBa } from "@/features/danh-ba/loc-danh-ba";
import { HopXoa } from "@/features/danh-ba/hop-xoa";
import { daXoa, tieuDeXoa, yeuCauXoa } from "@/features/danh-ba/xoa-dong";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { PendingMarker, type PendingFeatureInfo } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { docDanhMucDanhBa, type DanhMucDanhBa } from "@/lib/api/danh-muc";
import type { identity_canBoTomTat, page_Result_identity_canBoTomTat } from "@/lib/api/schema.gen";
import { capTaiKhoan, datLaiMatKhau } from "@/lib/api/tai-khoan";
import { cn } from "@/lib/cn";

import { ConfigDialog } from "./config-dialog";
import { ConfigTable, EmptyRow, RowActions, SMALL_BUTTON_CLASS } from "./config-ui";
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
  DELETE_ACCOUNT_BUTTON,
  DELETE_BLOCKED_REASON,
  EDIT_ACCOUNT_BUTTON,
  EMPTY_STAFF_LIST,
  NO_EMAIL_ACCOUNT_REASON,
  SEARCH_LABEL,
  SEARCH_PLACEHOLDER,
  canIssueAccount,
  emailLabel,
  nhanDangNhapGanNhat,
  nhanTaiKhoan,
  nhanTrangThai,
  nhanVaiTro,
  orDash,
  staffCountLine,
  unitCellLabel,
} from "./nhan-can-bo";
import { bangTraTuKetQua, traTen, type BangTraDanhMuc, type KetTra } from "./tra-danh-muc";

/**
 * Bảng danh bạ cán bộ của màn `/nguoi-dung` — theo prototype (`vigov-require/apps/admin/src/components/
 * admin/UserTable.tsx`, `UserFormDialog.tsx`) và quyết định của chủ dự án 08/10/2026 (fix-web-admin thẻ A).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * Hợp đồng REST phục vụ màn hình này: đọc `GET /api/v1/staff`, `POST /api/v1/staff/searches` (chữ tìm
 * đi trong thân), `GET /api/v1/staff-counts` (tổng của dòng đếm), hai danh mục `GET /api/v1/org-units`
 * · `GET /api/v1/roles`; ghi `POST /staff`, `PATCH /staff/{id}`, `POST`/`DELETE /staff/{id}/lockout`,
 * `PUT /staff/{id}/role`, `POST /staff/{id}/account`, `PUT /staff/{id}/password` (cùng `admin.user`) và
 * `DELETE /staff/{id}` (`admin.user.delete`, xoá mềm kèm lý do — luật 7).
 *
 * CHỖ MÀN HÌNH NÀY KHÁC PROTOTYPE, VÀ VÌ SAO (hạng 1/2 thắng prototype):
 *
 *   · Hai cột / hai ô điện thoại (#16) thay cho một "Điện thoại".
 *   · Không ô mật khẩu, không hộp vai trò trong hộp thoại hồ sơ: mật khẩu tạm do máy chủ sinh, hiện
 *     một lần (#9); vai trò đi tuyến riêng mang ràng buộc #13/#14 (`Đổi vai trò`).
 *   · Thêm `Đổi vai trò`, `Khoá/Mở khoá`, `Cấp TK/Đặt lại MK` ở mỗi dòng, và cột `Vai trò`, `Tài khoản`.
 *   · Xoá hỏi lý do (luật 7) và chỉ cho dòng KHÔNG có tài khoản (#10: nghỉ thì khoá, không xoá).
 *   · Tìm ở MÁY CHỦ, theo trang con trỏ: danh bạ một xã không đọc hết về trình duyệt để lọc.
 *   · Đầu cột có nút sắp xếp của prototype nhưng VÔ HIỆU kèm dấu "?" (ADR 0068 §14): máy chủ chỉ sắp
 *     theo mã và ngày tạo, hai cột ấy chủ dự án đã bỏ khỏi bảng. Không sắp giả ở client.
 *
 * KHÔNG CÓ CỔNG QUYỀN RIÊNG CHO PHẦN GHI `admin.user`: `TabNguoiDung` đã dùng đúng khoá ấy để quyết định
 * có dựng màn hình này hay không. `canDelete` (từ `admin.user.delete`) chỉ quyết định có VẼ `Trash2`.
 * Lớp chặn THẬT nằm ở máy chủ, trên TỪNG yêu cầu (luật 5, cấm #1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Trạng thái của một lần đọc. Ba nhánh rời nhau — không nhánh nào suy ra được từ nhánh khác. */
type TrangThaiTrang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; trang: page_Result_identity_canBoTomTat };

/**
 * `active` — whether the screen is on display. Omitted means a screen with no tabs around it: always on
 * display. Kept so a host that keeps the component mounted while hidden does not read catalogues for a
 * screen nobody is looking at (ND-01/ND-02, tester report 05/10).
 */
export function DanhBaCanBo({
  active = true,
  canDelete = false,
  importOpen = false,
  onImportClose = () => {},
}: {
  active?: boolean;
  /** The session holds `admin.user.delete` — draws `Trash2`. UX only: the server checks the key. */
  canDelete?: boolean;
  /**
   * Whether the staff Excel import dialog is open. Owned by `TabNguoiDung`, because its button sits in
   * the page header (prototype); the dialog and what it reloads belong here.
   */
  importOpen?: boolean;
  /** The dialog's own close — the ONLY thing that ends an import attempt (its N passwords included). */
  onImportClose?: () => void;
} = {}) {
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  /**
   * Bộ lọc đang ÁP DỤNG — chữ tìm đã chuẩn hoá. Không phải thứ đang gõ dở trong ô.
   *
   * CHỮ TÌM SỐNG Ở ĐÂY VÀ CHỈ Ở ĐÂY: state của component, chết cùng component. Không URL, không
   * `searchParams`, không `localStorage`, không khoá bộ đệm — nó thường là họ tên hay số điện thoại
   * (luật 3, cấm #4). Nó rời trình duyệt đúng một đường: thân `POST /api/v1/staff/searches`.
   */
  const [loc, datLoc] = useState<LocNguoiDung>(LOC_DAU);
  const [trangThai, datTrangThai] = useState<TrangThaiTrang>({ pha: "dangTai" });
  /** `null` là chưa đọc xong. Hai danh mục của xã, đọc MỘT lần cho cả màn hình — xem dưới. */
  const [danhMuc, datDanhMuc] = useState<DanhMucDanhBa | null>(null);
  /**
   * The commune's register size for the count line, from `GET /api/v1/staff-counts`. `null` = not read
   * (yet, or refused): the line then says only what is on screen — never a total nobody counted.
   */
  const [staffTotal, setStaffTotal] = useState<number | null>(null);

  /* ---- trạng thái của đường GHI ---------------------------------------------------------- */

  const [dangMo, datDangMo] = useState<DangMoGhi | null>(null);
  const [ban, datBan] = useState<BanNhapCanBo>(BAN_TRONG);
  const [vaiTroID, datVaiTroID] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  /** The row whose deletion is being confirmed, and the reason typed for it (rule 7: never without one). */
  const [deleting, setDeleting] = useState<identity_canBoTomTat | null>(null);
  const [deleteReason, setDeleteReason] = useState("");
  /**
   * Đếm số lần cần đọc lại danh sách. Tăng sau MỖI lần ghi thành công.
   *
   * VÌ SAO ĐỌC LẠI CẢ TRANG CHỨ KHÔNG VÁ MỘT DÒNG TẠI CHỖ: với `POST` thì người mới có thể thuộc về
   * một TRANG KHÁC (thứ tự do máy chủ quyết định), và một dòng chèn vào trang đang xem là một dòng ở
   * sai chỗ. Hai cách cư xử cho một nút Lưu là chỗ người dùng học sai cách màn hình hoạt động.
   */
  const [lanDoc, datLanDoc] = useState(0);

  /* ---- trạng thái của hai tuyến THÔNG TIN ĐĂNG NHẬP --------------------------------------- */

  /**
   * Bốn trạng thái RIÊNG, không dùng lại `dangMo`/`loiMayChu`/`dangGui` của biểu mẫu danh bạ: đường
   * này phải để lại trên màn hình một giá trị KHÔNG LẤY LẠI ĐƯỢC, và một ô trạng thái dùng chung là mở
   * đúng một đường cho một lần ghi khác — hay một lần đọc lại danh sách — xoá mất mật khẩu tạm.
   */
  const [moTaiKhoan, datMoTaiKhoan] = useState<DangMoTaiKhoan | null>(null);
  const [loiTaiKhoan, datLoiTaiKhoan] = useState("");
  const [dangGuiTaiKhoan, datDangGuiTaiKhoan] = useState(false);
  /**
   * Mật khẩu tạm đang hiện. `null` là không có gì để hiện. ĐÂY LÀ NƠI DUY NHẤT GIÁ TRỊ ẤY SỐNG: state
   * của component, chết cùng component. Không storage, không biến mức module, không `console.*` ở bất
   * kỳ nhánh nào chạm tới nó (luật 3, cấm #1 và #4 — xem đầu tệp `mat-khau-tam.tsx`).
   */
  const [matKhauTam, datMatKhauTam] = useState<MatKhauTamHienRa | null>(null);

  /**
   * HAI DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT MỖI LẦN MÀN NÀY ĐƯỢC HIỆN. Bộ phận hay vai trò vừa tạo ở màn khác
   * phải có mặt ở đây mà không cần tải lại trang (ND-01/ND-02). Không đọc lại khi đổi trang hay tìm,
   * và tuyệt đối không đọc theo từng dòng (`skills/load-data-once`, dạng 1). KHÔNG CÓ BỘ ĐỆM NÀO SỐNG
   * QUA LẦN MỞ MÀN HÌNH: một biến mức module giữ danh mục là đúng hình dạng của một lần danh mục xã
   * này hiện trên màn hình xã khác (`lib/api/danh-muc.ts`).
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

  const traBoPhan = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.boPhan),
    [danhMuc],
  );
  const traVaiTro = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.vaiTro),
    [danhMuc],
  );

  // The total of the count line: read on arrival and after every write (a write can add or remove a
  // row), not on paging or searching — the commune's total does not move between two page turns.
  useEffect(() => {
    let stale = false;
    void getStaffCounts().then((answer) => {
      if (!stale) setStaffTotal(answer.ok ? answer.duLieu : null);
    });
    return () => {
      stale = true;
    };
  }, [lanDoc]);

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau.
    let bo = false;

    // Không truyền `limit` (mặc định của máy chủ), không truyền `sort`/`order`: hai cột máy chủ sắp
    // được (mã, ngày tạo) đã rời bảng, nên thứ tự là thứ tự mặc định của máy chủ. MỘT chỗ rẽ GET hay
    // POST, và nó ở `docTrangDanhBa` — cùng hàm màn `/danh-ba` dùng.
    docTrangDanhBa(loc.tuKhoa, { boPhan: loc.boPhan, cursor: nganXep.hienTai }).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
    });

    return () => {
      bo = true;
    };
  }, [nganXep, loc, lanDoc]);

  /**
   * Chuyển trang. `dangTai` được đặt Ở ĐÂY, trong sự kiện, chứ không trong thân effect: gọi setState
   * thẳng trong thân effect kéo theo một lượt render phụ, và lint của React chặn đúng mẫu ấy.
   */
  const diToiTrang = useCallback((toi: NganXepConTro) => {
    datTrangThai({ pha: "dangTai" });
    datNganXep(toi);
  }, []);

  /**
   * Đổi chữ tìm là VỀ TRANG ĐẦU, luôn luôn: con trỏ của trang 3 thuộc về truy vấn cũ, gửi nó kèm bộ lọc
   * mới thì máy chủ hoặc từ chối, hoặc trả một trang giữa chừng và cán bộ không thấy những người đứng
   * trước con trỏ.
   */
  const doiLoc = useCallback(
    (doi: Partial<LocNguoiDung>) => {
      datLoc((cu) => ({ ...cu, ...doi }));
      diToiTrang(TRANG_DAU);
    },
    [diToiTrang],
  );

  /* ---- mở, đóng và gửi bốn biểu mẫu ghi ---------------------------------------------------- */

  /**
   * Danh sách mục cho hai ô chọn, lấy từ CHÍNH hai danh mục đã đọc cho bảng tra — không đọc lại khi mở
   * biểu mẫu. Danh mục hỏng thì ra mảng rỗng, và biểu mẫu vẫn mở được (giá trị đang lưu vẫn giữ).
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
    setDeleting(null);
    // Đóng biểu mẫu xác nhận của đường thông tin đăng nhập: hai biểu mẫu mở cùng lúc là hai nút Lưu
    // cho hai người khác nhau. KHÔNG đụng `matKhauTam` — chỉ một hành động rõ ràng mới đóng nó.
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
   * Sau một lần ghi thành công: đóng biểu mẫu, báo bằng toast (ADR 0068 lần 6 #4 — lỗi trong biểu mẫu
   * vẫn hiện tại chỗ), và đọc lại danh sách cùng tổng.
   */
  const ghiXong = useCallback((cau: string) => {
    datDangMo(null);
    datBan(BAN_TRONG);
    datVaiTroID("");
    datLoiMayChu("");
    setDeleting(null);
    setDeleteReason("");
    toast.success(cau);
    datLanDoc((n) => n + 1);
  }, []);

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    datLoiMayChu("");
    datDangGui(true);

    // KHÔNG KIỂM ĐỘ DÀI, KHUÔN THƯ ĐIỆN TỬ HAY KÝ TỰ SỐ ĐIỆN THOẠI Ở ĐÂY. Máy chủ kiểm cả ba, mỗi thứ
    // kèm một câu tiếng Việt nói rõ phải sửa gì (`domain/danh_ba_ghi.go`); chép xuống client là bản
    // sao thứ hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
    const goi =
      dangMo.kieu === "them"
        ? themCanBo(thanThem(ban), dangMo.khoaChongTrung).then((kq) =>
            kq.ok ? ghiXong(daThem(kq.duLieu.full_name)) : datLoiMayChu(kq.thongBao),
          )
        : dangMo.kieu === "sua"
          ? suaCanBo(dangMo.canBo.id, thanSua(ban, dangMo.canBo)).then((kq) =>
              kq.ok ? ghiXong(ACCOUNT_SAVED) : datLoiMayChu(kq.thongBao),
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

  /* ---- xoá mềm một dòng (decision 1, 08/10/2026) ------------------------------------------------ */

  const openDelete = useCallback((cb: identity_canBoTomTat) => {
    datDangMo(null);
    datMoTaiKhoan(null);
    datLoiTaiKhoan("");
    datLoiMayChu("");
    setDeleteReason("");
    setDeleting(cb);
  }, []);

  const closeDelete = useCallback(() => {
    setDeleting(null);
    setDeleteReason("");
    datLoiMayChu("");
  }, []);

  /**
   * Send the deletion. A row with an account, an empty or too long reason stop HERE (`yeuCauXoa`, the
   * same decision `/danh-ba` uses) — no request is built without a reason: `xoaCanBo` only takes a
   * validated one. The server still refuses a row with an account (409, its sentence shown verbatim).
   */
  const sendDelete = useCallback(() => {
    if (deleting === null || dangGui) return;
    const check = yeuCauXoa(deleting, deleteReason);
    if ("loi" in check) {
      datLoiMayChu(check.loi);
      return;
    }
    datLoiMayChu("");
    datDangGui(true);
    const fullName = deleting.full_name;
    void xoaCanBo(deleting.id, check.lyDo)
      .then((answer) => {
        if (answer.ok) ghiXong(daXoa(fullName));
        else datLoiMayChu(answer.thongBao);
      })
      .finally(() => datDangGui(false));
  }, [dangGui, deleteReason, deleting, ghiXong]);

  /* ---- cấp tài khoản và đặt lại mật khẩu ---------------------------------------------------- */

  /**
   * Mở biểu mẫu xác nhận của một trong hai tuyến. KHOÁ CHỐNG TRÙNG SINH LÚC MỞ, chỉ cho `datLai`: sinh
   * lúc gửi thì mỗi lần bấm lại sau một lỗi mạng là một mật khẩu tạm KHÁC vô hiệu hoá cái quản trị
   * viên vừa đọc qua điện thoại (`lib/api/tai-khoan.ts`). Tuyến cấp có khoá tự nhiên — lần hai trả 409.
   */
  const moCapTaiKhoan = useCallback((cb: identity_canBoTomTat) => {
    datDangMo(null);
    setDeleting(null);
    datLoiMayChu("");
    datLoiTaiKhoan("");
    datMoTaiKhoan({ kieu: "cap", canBo: cb });
  }, []);

  const moDatLaiMatKhau = useCallback((cb: identity_canBoTomTat) => {
    datDangMo(null);
    setDeleting(null);
    datLoiMayChu("");
    datLoiTaiKhoan("");
    datMoTaiKhoan({ kieu: "datLai", canBo: cb, khoaChongTrung: khoaChongTrungMoi() });
  }, []);

  const dongXacNhanTaiKhoan = useCallback(() => {
    datMoTaiKhoan(null);
    datLoiTaiKhoan("");
  }, []);

  /** Đóng ô mật khẩu tạm — HÀNH ĐỘNG DUY NHẤT xoá được giá trị ấy khỏi màn hình. Không đếm ngược. */
  const dongMatKhauTam = useCallback(() => datMatKhauTam(null), []);

  /**
   * Gửi một trong hai tuyến ghi thông tin đăng nhập.
   *
   * TÊN VÀ MÃ LẤY TỪ DÒNG NGƯỜI DÙNG VỪA BẤM, KHÔNG TỪ THÂN CÂU TRẢ LỜI: một lần phát lại theo khoá
   * chống trùng trả 200 kèm thân `{"code":…,"replayed":true}` (`core/idem/idem.go`), không có `staff`.
   * VÌ VẬY KIỂM HÌNH DẠNG THÂN TRƯỚC KHI MỞ Ô: `temporary_password` của một lần phát lại là `undefined`,
   * và không ai kiểm thì ô mật khẩu mở ra rỗng hoặc đọc to chữ "undefined".
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
        datLanDoc((n) => n + 1);
      })
      .finally(() => datDangGuiTaiKhoan(false));
  }, [dangGuiTaiKhoan, moTaiKhoan]);

  /** The row actions, in ONE object so no seventh action is added without passing through here. */
  const thaoTac = useMemo<ThaoTacDong>(
    () => ({
      sua: (cb) => moBieuMau({ kieu: "sua", canBo: cb }),
      doiVaiTro: (cb) => moBieuMau({ kieu: "vaiTro", canBo: cb }),
      datKhoa: (cb) => moBieuMau({ kieu: "khoa", canBo: cb, khoa: cb.active }),
      capTaiKhoan: moCapTaiKhoan,
      datLaiMatKhau: moDatLaiMatKhau,
      xoa: openDelete,
    }),
    [moBieuMau, moCapTaiKhoan, moDatLaiMatKhau, openDelete],
  );

  return (
    // THE PROTOTYPE'S COMPOSITION (`UserTable`): one toolbar row — search, then `Thêm cán bộ` at its right
    // end —, the bordered table, the count line. Every write opens in a centred dialog.
    <section className="min-w-0 space-y-3" aria-label="Danh sách người dùng">
      <div className="flex min-w-0 items-center gap-3">
        <HangLocNguoiDung loc={loc} doiLoc={doiLoc} />
        <Button
          type="button"
          variant="primary"
          className="ml-auto"
          icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
          aria-haspopup="dialog"
          onClick={() => moBieuMau({ kieu: "them", khoaChongTrung: khoaChongTrungMoi() })}
        >
          {NUT_THEM_CAN_BO}
        </Button>
      </div>

      {/*
        THE DIALOGS. Each one is OUTSIDE every branch of `trangThai`: a successful write re-reads the
        list, and a re-read that fails replaces the table with an error line — it must not take an open
        dialog (or a password) with it. All are top-layer `<dialog>`s, so their place here adds no space.
      */}

      {/* Staff import. Only the panel's own close calls `onImportClose` (`excel-import-panel.tsx`). */}
      {importOpen && (
        <ExcelImportPanel
          target={STAFF_IMPORT_TARGET}
          onImported={() => datLanDoc((n) => n + 1)}
          onClose={onImportClose}
          asDialog
        />
      )}

      {dangMo !== null && (
        <AccountDialog
          title={dialogTitle(dangMo)}
          description={
            dangMo.kieu === "them"
              ? GIAI_THICH_THEM
              : dangMo.kieu === "sua"
                ? staffCodeNote(dangMo.canBo.code)
                : undefined
          }
          wide={dangMo.kieu === "them" || dangMo.kieu === "sua"}
          busy={dangGui}
          onDismiss={dongBieuMau}
        >
          <BieuMauGhiCanBo
            layout="account"
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
        </AccountDialog>
      )}

      {/* Delete: the reason dialog `/danh-ba` already uses, inside this screen's dialog frame. */}
      {deleting !== null && (
        <ConfigDialog
          title={tieuDeXoa(deleting.full_name)}
          hideHeader
          onDismiss={() => {
            if (!dangGui) closeDelete();
          }}
        >
          <HopXoa
            canBo={deleting}
            lyDo={deleteReason}
            datLyDo={setDeleteReason}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={sendDelete}
            onHuy={closeDelete}
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

      {/* Ô MẬT KHẨU TẠM — Esc does NOT close it: the value is shown once, so only "Tôi đã ghi lại" does. */}
      {matKhauTam !== null && (
        <ConfigDialog title="Mật khẩu tạm — chỉ hiện một lần" hideHeader onDismiss={() => {}}>
          <OMatKhauTam matKhauTam={matKhauTam} onDong={dongMatKhauTam} />
        </ConfigDialog>
      )}

      {/* LOADING (prototype `Loading`): three 44px bars; the sentence stays the live region. */}
      {trangThai.pha === "dangTai" && (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải danh sách…
          </p>
          <div className="space-y-2" aria-hidden="true">
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
          </div>
        </div>
      )}

      {/* LỖI: đúng `message` của máy chủ, không diễn giải, không `trace_id` (xem `lib/api/goi.ts`). */}
      {trangThai.pha === "loi" && (
        <ErrorState role="alert" title="Chưa tải được danh sách cán bộ" message={trangThai.thongBao} />
      )}

      {trangThai.pha === "xong" && (
        <>
          {/* DANH MỤC HỎNG THÌ NÓI RA MỘT LẦN Ở ĐÂY, chứ không để hai mươi ô cùng báo lỗi. */}
          {(traBoPhan.pha === "loi" || traVaiTro.pha === "loi") && (
            <div className="flex flex-col gap-2 [&>*]:my-0">
              <BaoLoiDanhMuc nhan="Danh mục bộ phận" bang={traBoPhan} />
              <BaoLoiDanhMuc nhan="Danh mục vai trò" bang={traVaiTro} />
            </div>
          )}
          <BangCanBo
            danhSach={trangThai.trang.items}
            thaoTac={thaoTac}
            traBoPhan={traBoPhan}
            traVaiTro={traVaiTro}
            canDelete={canDelete}
          />
          {/* The prototype's count line; the cursor pager (kept, decision 3) at the right of it. */}
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
            <p className="text-ink-muted m-0 text-[12px]">
              {staffCountLine(trangThai.trang.items.length, staffTotal)}
            </p>
            <DieuHuongTrang
              nganXep={nganXep}
              conTroTiep={trangThai.trang.next_cursor}
              conTrangSau={trangThai.trang.has_more}
              diToiTrang={diToiTrang}
            />
          </div>
        </>
      )}
    </section>
  );
}

/** The heading of the write dialog. Edit is the prototype's; add keeps "…vào danh bạ" (#9: no account yet). */
function dialogTitle(open: DangMoGhi): string {
  switch (open.kieu) {
    case "them":
      return tieuDeThem();
    case "sua":
      return ACCOUNT_EDIT_TITLE;
    case "vaiTro":
      return tieuDeVaiTro(open.canBo.full_name);
    case "khoa":
      return tieuDeKhoa(open.canBo.full_name, open.khoa);
  }
}

/**
 * The write dialog: `ModalDialog` + header, the form placed DIRECTLY in it (not in `ConfigDialog`'s
 * scroll region, which would clip the footer band's negative margins). `wide` is the prototype's
 * `sm:max-w-2xl` for the profile form, whose name field takes the opening focus (`autoFocus` there).
 * Esc and ✕ do nothing while sending — a send in flight must not lose its form from view.
 */
function AccountDialog({
  title,
  description,
  wide,
  busy,
  onDismiss,
  children,
}: {
  title: string;
  description?: string;
  wide: boolean;
  busy: boolean;
  onDismiss: () => void;
  children: ReactNode;
}) {
  const titleId = useId();
  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!busy) onDismiss();
      }}
      closeDisabled={busy}
      initialFocusId={wide ? "o-ho-ten-can-bo" : undefined}
      className={wide ? "sm:max-w-2xl" : undefined}
    >
      <ModalDialogHeader titleId={titleId} title={title} description={description} />
      {children}
    </ModalDialog>
  );
}

/**
 * Một dòng báo khi không đọc được một danh mục. KHÔNG dựng gì khi danh mục đang đọc hoặc đã đọc xong:
 * một chỗ trống dành sẵn cho thông báo lỗi là một chỗ trống nhảy chữ vào giữa lúc người dùng đọc bảng.
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
 * Lớp CSS đi theo LOẠI kết quả tra chứ không theo câu chữ, để "không tra được" (một dòng dữ liệu lệch)
 * không trông giống một trạng thái bình thường. "Chưa gán" của cột Bộ phận là "—" trơn (prototype).
 */
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

/* ---- ô tìm ------------------------------------------------------------------------------------ */

/**
 * Bộ lọc của màn Người dùng — chữ tìm. `boPhan` stays in the type (the list call still takes it) but no
 * control sets it since the owner dropped the unit filter (08/10/2026): it is always `""`.
 */
export type LocNguoiDung = Pick<LocDanhBa, "tuKhoa" | "boPhan">;

export const LOC_DAU: LocNguoiDung = { tuKhoa: null, boPhan: "" };

/** Pause after the last key before the search runs (prototype: live search). */
export const SEARCH_DEBOUNCE_MS = 300;

function sameKeyword(a: LocNguoiDung["tuKhoa"], b: LocNguoiDung["tuKhoa"]): boolean {
  return (a?.tu ?? null) === (b?.tu ?? null);
}

/**
 * The prototype's search box (`UserTable.tsx`): magnifier inside, `pl-9`, no visible label, no button.
 * It runs AS YOU TYPE, on the SERVER (`POST /api/v1/staff/searches`, the words in the body) once the
 * hand pauses for `SEARCH_DEBOUNCE_MS` — one call per pause, not per key. Every decision is
 * `loc-danh-ba.ts`'s (`ketQuaGuiTim`), the one `/danh-ba` uses: too long → the refusal under the box
 * and NOTHING leaves the browser; blank → back to the plain list.
 *
 * THE INPUT HAS NO `name`, THE FORM IS `method="post"`, AUTOFILL IS OFF: before JavaScript runs, Enter
 * in a GET form puts every named field on the URL; and the browser remembers what was typed for the
 * next person at a shared desk (rule 3, forbidden #4). Enter is swallowed — the search already runs.
 *
 * HOOKS: only `useState`/`useEffect` — a test drives this component with a minimal hook runner.
 */
export function HangLocNguoiDung({
  loc,
  doiLoc,
}: {
  loc: LocNguoiDung;
  doiLoc: (doi: Partial<LocNguoiDung>) => void;
}) {
  const [oTim, datOTim] = useState("");
  const [loiTim, datLoiTim] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => {
      const kq = ketQuaGuiTim(oTim);
      if ("loi" in kq) {
        datLoiTim(kq.loi);
        return;
      }
      datLoiTim("");
      // Same words as the filter already applied (first mount, or a pause after the search ran):
      // nothing to send, and no reset of the page the officer is on.
      if (sameKeyword(kq.doi.tuKhoa, loc.tuKhoa)) return;
      doiLoc(kq.doi);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [oTim, loc.tuKhoa, doiLoc]);

  return (
    <form
      className="relative m-0 max-w-80 min-w-0 flex-1"
      role="search"
      method="post"
      autoComplete="off"
      onSubmit={(e) => e.preventDefault()}
    >
      {/* Its own `relative` box, so the magnifier stays centred on the input when the refusal line
          below adds height to the form. */}
      <div className="relative">
        <Search
          aria-hidden="true"
          focusable="false"
          className="text-ink-muted pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
        />
        <input
          id="tim-nguoi-dung"
          className={cn(controlClass, "pl-9")}
          value={oTim}
          onChange={(e) => datOTim(e.target.value)}
          placeholder={SEARCH_PLACEHOLDER}
          aria-label={SEARCH_LABEL}
          autoComplete="off"
          aria-describedby="loi-tim-nguoi-dung"
          aria-invalid={loiTim !== ""}
        />
      </div>
      {/* Always in the DOM (the live region the box points at); takes no room while empty. */}
      <p id="loi-tim-nguoi-dung" className="text-danger m-0 mt-1 text-[12px] font-medium empty:hidden" role="alert">
        {loiTim}
      </p>
    </form>
  );
}

/* ---- bảng ------------------------------------------------------------------------------------- */

/**
 * Các hành động một dòng mở ra. Mọi hành động nhận CẢ DÒNG chứ không nhận `id`: hộp thoại phải gọi
 * tên người đang được thao tác, `datKhoa` phải biết người ấy đang khoá hay chưa, và ô mật khẩu tạm lấy
 * tên với mã từ chính dòng ấy. `capTaiKhoan` VÀ `datLaiMatKhau` LÀ HAI TRƯỜNG: máy chủ tách chúng bằng
 * hai tuyến và hai điều kiện loại trừ nhau. `xoa` chỉ được VẼ khi `canDelete` (`admin.user.delete`).
 */
export type ThaoTacDong = {
  sua: (cb: identity_canBoTomTat) => void;
  doiVaiTro: (cb: identity_canBoTomTat) => void;
  datKhoa: (cb: identity_canBoTomTat) => void;
  capTaiKhoan: (cb: identity_canBoTomTat) => void;
  datLaiMatKhau: (cb: identity_canBoTomTat) => void;
  xoa: (cb: identity_canBoTomTat) => void;
};

/** One row action: the prototype's `sm outline` icon button; the action in `title`, the person in `aria-label`. */
function RowButton({
  action,
  person,
  onClick,
  className,
  children,
}: {
  action: string;
  person: string;
  onClick: () => void;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant="outline"
      title={action}
      aria-label={`${action}: ${person}`}
      aria-haspopup="dialog"
      onClick={onClick}
      className={className}
    >
      {children}
    </Button>
  );
}

/**
 * Cụm nút của một dòng (`flex items-center justify-end gap-1.5`, prototype). MỖI NÚT MANG TÊN NGƯỜI
 * TRONG `aria-label`: hai mươi nút đọc lên giống hệt nhau là danh sách người dùng trình đọc màn hình
 * không chọn đúng được dòng — và chọn nhầm ở đây là khoá nhầm tài khoản của một cán bộ.
 */
function NutCuaDong({
  cb,
  thaoTac,
  canDelete,
}: {
  cb: identity_canBoTomTat;
  thaoTac: ThaoTacDong;
  canDelete: boolean;
}) {
  const person = cb.full_name;
  const icon = "size-3.5";
  return (
    <RowActions>
      <RowButton action={EDIT_ACCOUNT_BUTTON} person={person} onClick={() => thaoTac.sua(cb)}>
        <Pencil aria-hidden="true" focusable="false" className={icon} />
      </RowButton>
      <RowButton action={NUT_DOI_VAI_TRO} person={person} onClick={() => thaoTac.doiVaiTro(cb)}>
        <UserCog aria-hidden="true" focusable="false" className={icon} />
      </RowButton>
      {/* Khoá hay mở khoá đọc từ `active` — chính thứ tuyến lockout ghi vào, nên nhãn không lệch việc. */}
      <RowButton action={cb.active ? NUT_KHOA : NUT_MO_KHOA} person={person} onClick={() => thaoTac.datKhoa(cb)}>
        {cb.active ? (
          <Lock aria-hidden="true" focusable="false" className={icon} />
        ) : (
          <LockOpen aria-hidden="true" focusable="false" className={icon} />
        )}
      </RowButton>

      {/* MỘT NÚT, KHÔNG HAI: `has_account` tách hai thế giới loại trừ nhau ngay trong mệnh đề WHERE,
          nên nút kia VẮNG MẶT chứ không mờ đi — việc ấy không áp dụng cho dòng này. */}
      {cb.has_account ? (
        <RowButton action={NUT_DAT_LAI_MAT_KHAU} person={person} onClick={() => thaoTac.datLaiMatKhau(cb)}>
          <KeyRound aria-hidden="true" focusable="false" className={icon} />
        </RowButton>
      ) : canIssueAccount(cb.email) ? (
        <RowButton action={NUT_CAP_TAI_KHOAN} person={person} onClick={() => thaoTac.capTaiKhoan(cb)}>
          <UserPlus aria-hidden="true" focusable="false" className={icon} />
        </RowButton>
      ) : (
        // NO EMAIL, NO ACCOUNT: the SAME action blocked by a fixable gap, so it is disabled and tied to
        // the VISIBLE reason in the row's "Tài khoản" cell. The server still refuses (409).
        <Button
          type="button"
          size="sm"
          variant="outline"
          aria-label={`${NUT_CAP_TAI_KHOAN}: ${person}`}
          aria-describedby={`no-email-reason-${cb.id}`}
          disabled
        >
          <UserPlus aria-hidden="true" focusable="false" className={icon} />
        </Button>
      )}

      {canDelete &&
        (cb.has_account ? (
          // A ROW HOLDING A SIGN-IN ACCOUNT CANNOT BE DELETED (#10, server 409 `staff_has_account`):
          // retirement or transfer is a lock. Disabled, with the reason on hover (the wrapper — a
          // disabled button gets no pointer events) and for a screen reader.
          <span className="inline-flex" title={DELETE_BLOCKED_REASON}>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className={cn("text-danger", SMALL_BUTTON_CLASS)}
              aria-label={`${DELETE_ACCOUNT_BUTTON}: ${person}`}
              aria-describedby={`delete-blocked-${cb.id}`}
              disabled
            >
              <Trash2 aria-hidden="true" focusable="false" className={icon} />
            </Button>
            <span id={`delete-blocked-${cb.id}`} className="an-thi-giac">
              {DELETE_BLOCKED_REASON}
            </span>
          </span>
        ) : (
          <RowButton
            action={DELETE_ACCOUNT_BUTTON}
            person={person}
            onClick={() => thaoTac.xoa(cb)}
            className="text-danger"
          >
            <Trash2 aria-hidden="true" focusable="false" className={icon} />
          </RowButton>
        ))}
    </RowActions>
  );
}

/**
 * Ô "Tài khoản": có / chưa, và — với dòng chưa cấp được — lý do, đọc được ngay trong ô (và bởi trình
 * đọc màn hình qua `aria-describedby` của nút bị vô hiệu). Không cắt "…" (U19): câu được xuống dòng.
 */
function OTaiKhoan({ cb }: { cb: identity_canBoTomTat }) {
  const khongCoThu = !cb.has_account && !canIssueAccount(cb.email);
  return (
    <>
      <span className="block">{nhanTaiKhoan(cb.has_account)}</span>
      {khongCoThu && (
        <span
          className="text-ink-muted block max-w-[16rem] text-[11.5px] whitespace-normal"
          id={`no-email-reason-${cb.id}`}
        >
          {NO_EMAIL_ACCOUNT_REASON}
        </span>
      )}
    </>
  );
}

/** What the column-sort "?" says: the server sorts by neither of the columns left on this table. */
const SORT_PENDING: PendingFeatureInfo = {
  ten: "Sắp xếp theo cột",
  viSao:
    "Máy chủ hiện chỉ sắp xếp danh sách cán bộ theo mã cán bộ và ngày tạo — hai cột này không còn trên " +
    "bảng. Sắp xếp theo các cột khác cần tuyến danh sách nhận thêm khoá sắp xếp; trong lúc chờ, danh " +
    "sách theo thứ tự mặc định của máy chủ.",
};

/**
 * A column head in the prototype's shape (`<button className="flex items-center gap-1.5">label
 * <ArrowUpDown/></button>`), DISABLED with the "?" marker (ADR 0068 §14): no client-side sort of one
 * page would be honest on a cursor-paged list.
 */
function SortHead({ label }: { label: string }) {
  return (
    <th scope="col">
      <span className="inline-flex items-center gap-1.5">
        <button
          type="button"
          disabled
          className="flex items-center gap-1.5 border-0 bg-transparent p-0 [font-family:inherit] text-inherit [font-size:inherit] [font-weight:inherit] disabled:cursor-not-allowed"
        >
          {label}
          <ArrowUpDown aria-hidden="true" focusable="false" className="size-3 opacity-40" />
        </button>
        <PendingMarker info={SORT_PENDING} side="bottom" />
      </span>
    </th>
  );
}

/** The nine data columns, in the owner's order (decision 3). */
const DATA_COLUMNS = [
  "Họ và tên",
  "Chức danh",
  "Bộ phận",
  "Vai trò",
  // TWO phone columns, labelled by kind (#16): duty information vs Decree 13 personal data.
  "Máy bàn cơ quan",
  "Di động cá nhân",
  "Đăng nhập gần nhất",
  "Trạng thái",
  "Tài khoản",
] as const;

// EXPORTED SO THE COLUMNS CAN BE PINNED BY A RENDER TEST: the parent reads the API in `useEffect`,
// which `renderToStaticMarkup` never runs.
export function BangCanBo({
  danhSach,
  thaoTac,
  traBoPhan,
  traVaiTro,
  canDelete = false,
}: {
  danhSach: readonly identity_canBoTomTat[];
  thaoTac: ThaoTacDong;
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số. Không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  traVaiTro: BangTraDanhMuc;
  canDelete?: boolean;
}) {
  return (
    // The prototype's frame (`border-line overflow-hidden rounded-[10px] border` around shadcn's Table),
    // as every config table draws it (`ConfigTable`); it scrolls sideways inside itself at 320px.
    <ConfigTable label="Danh sách cán bộ" caption="Danh sách cán bộ của đơn vị">
      <thead>
        <tr>
          {DATA_COLUMNS.map((label) => (
            <SortHead key={label} label={label} />
          ))}
          <th scope="col" className="text-right">
            <span className="an-thi-giac">Hành động</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {danhSach.length === 0 ? (
          <EmptyRow colSpan={DATA_COLUMNS.length + 1}>{EMPTY_STAFF_LIST}</EmptyRow>
        ) : (
          danhSach.map((cb) => {
            const unit = traTen(traBoPhan, cb.department_id);
            const role = traTen(traVaiTro, cb.role_id);
            return (
              <tr key={cb.id}>
                <td>
                  <div className="text-navy font-semibold">{cb.full_name}</div>
                  <div className="text-ink-muted text-[11.5px]">{emailLabel(cb.email)}</div>
                </td>
                <td>{orDash(cb.position)}</td>
                <td>
                  <span className={unit.loai === "chuaGan" ? undefined : lopNhanDanhMuc(unit)}>{unitCellLabel(unit)}</span>
                </td>
                <td>
                  <span className={lopNhanDanhMuc(role)}>{nhanVaiTro(role)}</span>
                </td>
                {/* HAI Ô RIÊNG, HIỆN NGUYÊN VĂN thứ máy chủ trả (#11: không che trong nội bộ xã). Không
                    gộp bằng `phone || mobile`, không nối bằng dấu phẩy. */}
                <td>{orDash(cb.phone)}</td>
                <td>{orDash(cb.mobile)}</td>
                <td>{nhanDangNhapGanNhat(cb.last_login_at)}</td>
                <td>
                  {/* Tone by the CODE (`active`), never by the words; the Badge's icon is allowed
                      (ADR 0068 lần 6 #7). */}
                  <Badge tone={cb.active ? "success" : "danger"}>{nhanTrangThai(cb.active)}</Badge>
                </td>
                <td>
                  <OTaiKhoan cb={cb} />
                </td>
                <td className="text-right">
                  <NutCuaDong cb={cb} thaoTac={thaoTac} canDelete={canDelete} />
                </td>
              </tr>
            );
          })
        )}
      </tbody>
    </ConfigTable>
  );
}

/**
 * Phân trang theo con trỏ. KHÔNG CÓ SỐ TRANG: hợp đồng trả `next_cursor` + `has_more`, không trả số
 * trang — hiện "Trang 3/12" là báo một con số không ai tính (`core/page`).
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
  // Hai điều kiện: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang m-0" aria-label="Phân trang danh sách cán bộ">
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
