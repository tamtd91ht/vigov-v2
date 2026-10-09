"use client";

import { ArrowUpDown, KeyRound, Lock, LockOpen, Pencil, Plus, Search, Trash2, UserPlus } from "lucide-react";
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
  NUT_KHOA,
  NUT_MO_KHOA,
  NUT_THEM_CAN_BO,
  banTuCanBo,
  daDatKhoa,
  daThem,
  khoaChongTrungMoi,
  thanSua,
  thanThem,
  tieuDeKhoa,
  type BanNhapCanBo,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  countStaffMatches,
  datKhoaCanBo,
  docTrangDanhBa,
  doiVaiTroCanBo,
  getStaffCounts,
  suaCanBo,
  themCanBo,
  xoaCanBo,
  type ChieuSapXep,
  type KhoaSapXep,
} from "@/lib/api/can-bo";
import { ketQuaGuiTim, type LocDanhBa } from "@/features/danh-ba/loc-danh-ba";
import { ActionMenu, type ActionMenuItem } from "@/components/ui/action-menu";
import { HopXoa } from "@/features/danh-ba/hop-xoa";
import { daXoa, tieuDeXoa, yeuCauXoa } from "@/features/danh-ba/xoa-dong";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { docDanhMucDanhBa, type DanhMucDanhBa } from "@/lib/api/danh-muc";
import type { identity_canBoTomTat, page_Result_identity_canBoTomTat } from "@/lib/api/schema.gen";
import { capTaiKhoan, datLaiMatKhau } from "@/lib/api/tai-khoan";
import { cn } from "@/lib/cn";

import { ConfigDialog } from "./config-dialog";
import { ConfigTable, EmptyRow, RowActions, SMALL_BUTTON_CLASS } from "./config-ui";
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
  ADD_ACCOUNT_DESCRIPTION,
  ADD_ACCOUNT_TITLE,
  ADD_GRANT_NOT_CHAINED,
  DELETE_ACCOUNT_BUTTON,
  DELETE_BLOCKED_REASON,
  EDIT_ACCOUNT_BUTTON,
  EMPTY_STAFF_LIST,
  NO_EMAIL_MENU_HINT,
  SEARCH_LABEL,
  SEARCH_PLACEHOLDER,
  accountBadgeReason,
  canIssueAccount,
  emailSubline,
  moreActionsLabel,
  nhanDangNhapGanNhat,
  nhanTaiKhoan,
  nhanTrangThai,
  orDash,
  phoneCell,
  roleChangeFailed,
  staffCodeLine,
  staffCountLine,
  unitCellLabel,
} from "./nhan-can-bo";
import { StaffAccountForm, type RoleChoice } from "./staff-account-form";
import { StaffImportDialog } from "./staff-import-dialog";
import { bangTraTuKetQua, traTen, type BangTraDanhMuc, type KetTra } from "./tra-danh-muc";

/**
 * Bảng danh bạ cán bộ của màn `/nguoi-dung` — theo prototype (`vigov-require/apps/admin/src/components/
 * admin/UserTable.tsx`, `UserFormDialog.tsx`), quyết định của chủ dự án 08/10/2026 (fix-web-admin thẻ A)
 * và quyết định của người dùng 09/10/2026 (đặc tả `cap-nhat-menu-nguoi-dung-phan-quyen-theo-prototype`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * Hợp đồng REST phục vụ màn hình này: đọc `GET /api/v1/staff`, `POST /api/v1/staff/searches` (chữ tìm
 * đi trong thân), `GET /api/v1/staff-counts` (tổng của dòng đếm khi không tìm), `POST /api/v1/staff-count-
 * queries` (tổng của dòng đếm khi đang tìm, chữ trong thân), hai danh mục `GET /api/v1/org-units`
 * · `GET /api/v1/roles`; ghi `POST /staff`, `PATCH /staff/{id}`, `POST`/`DELETE /staff/{id}/lockout`,
 * `PUT /staff/{id}/role`, `POST /staff/{id}/account`, `PUT /staff/{id}/password` (cùng `admin.user`) và
 * `DELETE /staff/{id}` (`admin.user.delete`, xoá mềm kèm lý do — luật 7).
 *
 * BẢY CỘT CỦA PROTOTYPE (người dùng 09/10/2026): Họ và tên (dòng phụ là thư điện tử) · Chức danh · Bộ
 * phận · Điện thoại · Đăng nhập gần nhất · Trạng thái · cột thao tác không tiêu đề. Cột Vai trò, Tài
 * khoản và hai cột điện thoại của 08/10 đã bỏ: trạng thái tài khoản thành chip thứ hai cạnh Trạng thái,
 * câu dài vào `title`; vai trò sửa trong hộp thoại Sửa.
 *
 * CHỖ MÀN HÌNH NÀY VẪN KHÁC PROTOTYPE, VÀ VÌ SAO:
 *
 *   · Cột "Điện thoại" hiện di động, không có thì máy bàn — NGUYÊN VĂN máy chủ trả (che hay không là
 *     việc của máy chủ, Nghị định 13, câu mở #16); `title` của ô nói đó là loại số nào. Cột này KHÔNG
 *     sắp được: khoá `phone` của máy chủ chỉ sắp theo máy bàn, không theo `di_dong ?? may_ban`, và màn
 *     hình không bịa ra một khoá sắp xếp mới.
 *   · Không ô mật khẩu (người dùng 09/10/2026, câu mở #9): mật khẩu tạm do máy chủ sinh, hiện một lần.
 *     Thêm có thư điện tử thì cấp tài khoản ngay sau khi thêm (`POST /staff/{id}/account`); quên mật
 *     khẩu là `Đặt lại mật khẩu` trong menu `⋯`.
 *   · MỘT vai trò, chọn trong hộp thoại Sửa và lưu bằng `PUT /staff/{id}/role` SAU lần PATCH hồ sơ —
 *     tuyến mang ràng buộc #13/#14. Không còn hộp thoại `Đổi vai trò` riêng.
 *   · `Khoá/Mở khoá` và `Cấp tài khoản`/`Đặt lại mật khẩu` gom vào menu `⋯` giữa Sửa và Xoá.
 *   · Xoá hỏi lý do (luật 7) và chỉ cho dòng KHÔNG có tài khoản (#10: nghỉ thì khoá, không xoá).
 *   · Tìm ở MÁY CHỦ, theo trang con trỏ: danh bạ một xã không đọc hết về trình duyệt để lọc. Nút trang
 *     chỉ hiện khi có trang trước hoặc trang sau.
 *   · Sắp xếp ở MÁY CHỦ (4b0b9ce3), không sắp một trang ở client: năm cột có khoá của máy chủ là nút thật.
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
  selfCode = null,
}: {
  active?: boolean;
  /**
   * The signed-in officer's staff code (`GET /api/v1/sessions/current` → `staff.code`), to disable the
   * role choice on their OWN row (#14). `null` = unknown: nothing is disabled, the server still refuses
   * (403 `self_target_forbidden`). Passed down, never read here: this component is driven in tests by
   * a hook runner that knows no context.
   */
  selfCode?: string | null;
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
   * The order the officer chose by clicking a column head. `null` = none: the server's default (code
   * ascending), and NOTHING is sent — no copy of that default lives in this file.
   */
  const [sort, setSort] = useState<StaffSort | null>(null);
  /**
   * The M of the count line, TAGGED WITH THE FILTER IT ANSWERS (`forLoc`, the very `loc` object the read
   * was made for). The line shows it only while that filter is still the applied one, so an answer for
   * an older search — or the commune total, while a search's own count is in flight — never sits next to
   * the N of a newer list. `total: null` = refused: the line then says only what is on screen.
   */
  const [count, setCount] = useState<{ forLoc: LocNguoiDung; total: number | null } | null>(null);

  /* ---- trạng thái của đường GHI ---------------------------------------------------------- */

  const [dangMo, datDangMo] = useState<DangMoGhi | null>(null);
  const [ban, datBan] = useState<BanNhapCanBo>(BAN_TRONG);
  /** The role chosen in the edit dialog (`""` = no role). Sent through `PUT .../role`, never the PATCH. */
  const [vaiTroID, datVaiTroID] = useState("");
  /**
   * The row as the PATCH returned it, once the profile is saved and the role change that followed was
   * refused. A second Lưu then compares against THIS row: an unchanged profile is not PATCHed again
   * (which would write a second audit entry for nothing), only the role is retried.
   */
  const [patchedRow, setPatchedRow] = useState<identity_canBoTomTat | null>(null);
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

  // The M of the count line: the commune's total with no search, the search's own match count with one
  // (same q/unit/published as the list, and fired by the same debounced `doiLoc`). Read on arrival, on
  // every filter change and after every write; not on paging or sorting — neither changes the set.
  // `stale` drops an answer whose effect was superseded; `forLoc` (above) hides one that is not the
  // applied filter's — two guards, because the second also covers "the list is back, the count is not".
  useEffect(() => {
    let stale = false;
    const forLoc = loc;
    const read =
      forLoc.tuKhoa === null ? getStaffCounts() : countStaffMatches(forLoc.tuKhoa, { boPhan: forLoc.boPhan });
    void read.then((answer) => {
      if (!stale) setCount({ forLoc, total: answer.ok ? answer.duLieu : null });
    });
    return () => {
      stale = true;
    };
  }, [loc, lanDoc]);

  const shownTotal = count !== null && count.forLoc === loc ? count.total : null;

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau.
    let bo = false;

    // Không truyền `limit` (mặc định của máy chủ). `sort`/`order` only when a head was clicked — they go
    // on the GET's URL or in the search's body. MỘT chỗ rẽ GET hay POST, và nó ở `docTrangDanhBa` —
    // cùng hàm màn `/danh-ba` dùng.
    docTrangDanhBa(loc.tuKhoa, {
      boPhan: loc.boPhan,
      cursor: nganXep.hienTai,
      sort: sort?.key,
      order: sort?.order,
    }).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
    });

    return () => {
      bo = true;
    };
  }, [nganXep, loc, sort, lanDoc]);

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

  /**
   * A click on a sort head. Back to the FIRST page, always: a cursor encodes a position in ONE order, and
   * sent with another it reads from the middle of a different sequence.
   */
  const changeSort = useCallback(
    (key: KhoaSapXep) => {
      setSort((current) => nextSort(current, key));
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
    datVaiTroID(m.kieu === "sua" ? m.canBo.role_id : "");
    setPatchedRow(null);
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
    setPatchedRow(null);
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
    setPatchedRow(null);
    datLoiMayChu("");
    setDeleting(null);
    setDeleteReason("");
    toast.success(cau);
    datLanDoc((n) => n + 1);
  }, []);

  /** Whether the edit dialog may change this row's role — not on the officer's own row (#14). */
  const roleChoiceFor = useCallback(
    (cb: identity_canBoTomTat): RoleChoice => (selfCode !== null && cb.code === selfCode ? "own" : "editable"),
    [selfCode],
  );

  /**
   * ADD. Without an email: one `POST /staff`, a directory entry only. WITH an email (user 09/10/2026):
   * the entry first, then AT ONCE the existing grant route `POST /staff/{id}/account`, whose one-time
   * password opens in the box below. If the grant fails the entry STAYS (it was created); the form
   * closes, the list re-reads (the row shows `Chỉ trong danh bạ`), and the grant's confirmation opens
   * with the server's sentence — its button is the retry, as `⋯ › Cấp tài khoản` is later.
   */
  const addStaff = useCallback(
    async (idempotencyKey: string) => {
      const draft = ban;
      const kq = await themCanBo(thanThem(draft), idempotencyKey);
      if (!kq.ok) {
        datLoiMayChu(kq.thongBao);
        return;
      }
      const created = kq.duLieu;
      const name = typeof created.full_name === "string" ? created.full_name : draft.hoTen;
      if (!canIssueAccount(draft.email)) {
        ghiXong(daThem(name));
        return;
      }
      // A replay of an earlier add (`{code, replayed: true}`) carries no row, so there is no id to grant.
      if (typeof created.id !== "string" || created.id === "") {
        ghiXong(daThem(name));
        toast.error(ADD_GRANT_NOT_CHAINED);
        return;
      }
      const grant = await capTaiKhoan(created.id);
      ghiXong(daThem(name));
      const password = grant.ok ? grant.duLieu.temporary_password : undefined;
      if (grant.ok && typeof password === "string" && password !== "") {
        datMatKhauTam({ kieu: "cap", maCanBo: created.code, hoTen: name, matKhau: password });
        return;
      }
      datMoTaiKhoan({ kieu: "cap", canBo: created });
      datLoiTaiKhoan(grant.ok ? CAU_PHAT_LAI_KHONG_CO_MAT_KHAU : grant.thongBao);
    },
    [ban, ghiXong],
  );

  /**
   * EDIT. The profile PATCH first; then, only if the role choice differs and may be changed, the
   * existing `PUT /staff/{id}/role` — so #13 and #14 still answer there, and their sentence reaches
   * this dialog. A refused role change leaves the dialog open with the profile already saved
   * (`patchedRow`); the list is re-read so the row shows what the server now holds.
   */
  const saveEdit = useCallback(
    async (cb: identity_canBoTomTat) => {
      const base = patchedRow ?? cb;
      const profileChanged = patchedRow === null || !sameDraft(banTuCanBo(base), ban);
      const roleWanted = roleChoiceFor(cb) === "editable" && vaiTroID !== base.role_id;
      let saved = base;
      if (profileChanged) {
        const kq = await suaCanBo(cb.id, thanSua(ban, base));
        if (!kq.ok) {
          datLoiMayChu(kq.thongBao);
          return;
        }
        saved = kq.duLieu;
      }
      if (!roleWanted) {
        ghiXong(ACCOUNT_SAVED);
        return;
      }
      const role = await doiVaiTroCanBo(cb.id, vaiTroID);
      if (role.ok) {
        ghiXong(ACCOUNT_SAVED);
        return;
      }
      if (profileChanged) {
        setPatchedRow(saved);
        datLanDoc((n) => n + 1);
      }
      datLoiMayChu(roleChangeFailed(role.thongBao));
    },
    [ban, ghiXong, patchedRow, roleChoiceFor, vaiTroID],
  );

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    datLoiMayChu("");
    datDangGui(true);

    // KHÔNG KIỂM ĐỘ DÀI, KHUÔN THƯ ĐIỆN TỬ HAY KÝ TỰ SỐ ĐIỆN THOẠI Ở ĐÂY. Máy chủ kiểm cả ba, mỗi thứ
    // kèm một câu tiếng Việt nói rõ phải sửa gì (`domain/danh_ba_ghi.go`); chép xuống client là bản
    // sao thứ hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
    let goi: Promise<unknown>;
    switch (dangMo.kieu) {
      case "them":
        goi = addStaff(dangMo.khoaChongTrung);
        break;
      case "sua":
        goi = saveEdit(dangMo.canBo);
        break;
      case "khoa":
        goi = datKhoaCanBo(dangMo.canBo.id, dangMo.khoa).then((kq) =>
          kq.ok ? ghiXong(daDatKhoa(kq.duLieu.full_name, dangMo.khoa)) : datLoiMayChu(kq.thongBao),
        );
        break;
      case "vaiTro":
        // No longer opened on this screen (user 09/10/2026: the role is chosen in the edit dialog).
        goi = Promise.resolve();
        break;
    }

    void goi.finally(() => datDangGui(false));
  }, [addStaff, dangGui, dangMo, ghiXong, saveEdit]);

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

  /** The row actions, in ONE object so no sixth action is added without passing through here. */
  const thaoTac = useMemo<ThaoTacDong>(
    () => ({
      sua: (cb) => moBieuMau({ kieu: "sua", canBo: cb }),
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

      {/* Staff import (user 09/10/2026: the prototype's 672px dialog). Only its own close calls
          `onImportClose` — after a 201 that close is the one-time passwords' explicit button. */}
      {importOpen && <StaffImportDialog onImported={() => datLanDoc((n) => n + 1)} onClose={onImportClose} />}

      {dangMo !== null && (dangMo.kieu === "them" || dangMo.kieu === "sua") && (
        <AccountDialog
          title={dangMo.kieu === "them" ? ADD_ACCOUNT_TITLE : ACCOUNT_EDIT_TITLE}
          description={
            dangMo.kieu === "them" ? (
              ADD_ACCOUNT_DESCRIPTION
            ) : (
              // The staff code as a small muted sub-line (#15: shown, never an input).
              <span className="text-xs">{staffCodeLine(dangMo.canBo.code)}</span>
            )
          }
          wide
          busy={dangGui}
          onDismiss={dongBieuMau}
        >
          <StaffAccountForm
            open={dangMo}
            draft={ban}
            setDraft={datBan}
            roleId={vaiTroID}
            setRoleId={datVaiTroID}
            roleChoice={dangMo.kieu === "sua" ? roleChoiceFor(dangMo.canBo) : "editable"}
            units={mucBoPhan}
            roles={mucVaiTro}
            serverError={loiMayChu}
            busy={dangGui}
            onSubmit={guiBieuMau}
            onCancel={dongBieuMau}
          />
        </AccountDialog>
      )}

      {/* Lock / unlock keeps its confirmation form (#13's refusal lands there). */}
      {dangMo !== null && dangMo.kieu === "khoa" && (
        <AccountDialog title={tieuDeKhoa(dangMo.canBo.full_name, dangMo.khoa)} wide={false} busy={dangGui} onDismiss={dongBieuMau}>
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
            canDelete={canDelete}
            sort={sort}
            onSort={changeSort}
          />
          {/* The prototype's count line; the cursor pager at the right of it ONLY when there is another
              page to go to (user 09/10/2026: no dead buttons on a one-page list). */}
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
            <p className="text-ink-muted m-0 text-[12px]">
              {staffCountLine(trangThai.trang.items.length, shownTotal)}
            </p>
            {hasOtherPage(nganXep, trangThai.trang.has_more, trangThai.trang.next_cursor) && (
              <DieuHuongTrang
                nganXep={nganXep}
                conTroTiep={trangThai.trang.next_cursor}
                conTrangSau={trangThai.trang.has_more}
                diToiTrang={diToiTrang}
              />
            )}
          </div>
        </>
      )}
    </section>
  );
}

/** Two drafts hold the same profile — the seven fields of `BanNhapCanBo`, compared as typed. */
function sameDraft(a: BanNhapCanBo, b: BanNhapCanBo): boolean {
  return (Object.keys(a) as (keyof BanNhapCanBo)[]).every((k) => a[k] === b[k]);
}

/**
 * Whether the list has a page before or after this one — the cursor pager's own two conditions
 * (`DieuHuongTrang`). The contract has no page count, so "more than one page" is exactly this.
 */
export function hasOtherPage(stack: NganXepConTro, hasMore: boolean, nextCursor: string): boolean {
  return coTrangTruoc(stack) || (hasMore && nextCursor !== "");
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
  description?: ReactNode;
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
 * The `⋯` menu of one row (user 09/10/2026): `Khoá`/`Mở khoá`, then `Đặt lại mật khẩu` OR `Cấp tài
 * khoản` — ONE of the two, never both: `has_account` splits two worlds the server already separates in
 * its WHERE clause, so the other action is ABSENT, not greyed. A row with no email keeps `Cấp tài khoản`
 * DISABLED with its reason as the item's second line (the same fixable gap as before; the server still
 * answers 409). Pure, exported: the closed menu renders no items to HTML, so tests read this list.
 */
export function staffRowMenuItems(cb: identity_canBoTomTat, actions: ThaoTacDong): ActionMenuItem[] {
  const lock: ActionMenuItem = {
    kind: "item",
    id: "lock",
    // Lock or unlock reads `active` — the very field the lockout route writes, so the label cannot drift.
    label: cb.active ? NUT_KHOA : NUT_MO_KHOA,
    icon: cb.active ? Lock : LockOpen,
    onSelect: () => actions.datKhoa(cb),
  };
  const credential: ActionMenuItem = cb.has_account
    ? { kind: "item", id: "password", label: NUT_DAT_LAI_MAT_KHAU, icon: KeyRound, onSelect: () => actions.datLaiMatKhau(cb) }
    : canIssueAccount(cb.email)
      ? { kind: "item", id: "account", label: NUT_CAP_TAI_KHOAN, icon: UserPlus, onSelect: () => actions.capTaiKhoan(cb) }
      : {
          kind: "item",
          id: "account",
          label: NUT_CAP_TAI_KHOAN,
          icon: UserPlus,
          onSelect: () => {},
          disabled: true,
          hint: NO_EMAIL_MENU_HINT,
        };
  return [lock, credential];
}

/**
 * Cụm nút của một dòng (`flex items-center justify-end gap-1.5`, prototype): `Sửa` · `⋯` · `Xoá`. MỖI
 * NÚT MANG TÊN NGƯỜI TRONG `aria-label`: hai mươi nút đọc lên giống hệt nhau là danh sách người dùng
 * trình đọc màn hình không chọn đúng được dòng — và chọn nhầm ở đây là khoá nhầm tài khoản của một cán bộ.
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

      <ActionMenu label={moreActionsLabel(person)} items={staffRowMenuItems(cb, thaoTac)} triggerVariant="secondary" />

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
 * The two chips of the `Trạng thái` cell (user 09/10/2026): the status, then the account — TEXT ONLY,
 * the prototype's chips (`icon={null}`; this overrides, for this table alone, the 08/10 note on
 * `StatusBadge` in `config-ui.tsx` — the shared Badge keeps its icon everywhere else). The tone follows
 * the CODE (`active`, `has_account`), never the words. The account chip's long reason is its `title`
 * and visually hidden text, so the row stays one line (≤ 56px).
 *
 * `active` and `has_account` ARE TWO QUESTIONS (`nhanTrangThai`): two chips, never one merged word.
 */
function StatusCell({ cb }: { cb: identity_canBoTomTat }) {
  const reason = accountBadgeReason(cb.has_account, cb.email);
  return (
    <div className="flex items-center gap-1.5">
      <Badge tone={cb.active ? "success" : "danger"} icon={null}>
        {nhanTrangThai(cb.active)}
      </Badge>
      <Badge tone={cb.has_account ? "info" : "neutral"} icon={null} title={reason} aria-describedby={`account-reason-${cb.id}`}>
        {nhanTaiKhoan(cb.has_account)}
      </Badge>
      <span id={`account-reason-${cb.id}`} className="an-thi-giac">
        {reason}
      </span>
    </div>
  );
}

/** The order chosen on the table: one server sort key and its direction. */
export type StaffSort = { readonly key: KhoaSapXep; readonly order: ChieuSapXep };

/**
 * One click on a head. Another column starts ascending; the same column goes asc → desc → none, where
 * none is the server's default (code ascending) — the prototype's TanStack cycle (asc / desc / unsorted).
 */
export function nextSort(current: StaffSort | null, key: KhoaSapXep): StaffSort | null {
  if (current === null || current.key !== key) return { key, order: "asc" };
  return current.order === "asc" ? { key, order: "desc" } : null;
}

/**
 * A sortable head in the prototype's shape (`<button className="flex items-center gap-1.5">label
 * <ArrowUpDown className="size-3 opacity-40"/></button>`). The active direction is told by `aria-sort`
 * on the `<th>` (owner 08/10/2026: no ↑/↓ glyph), and only on the active one — `aria-sort` names the
 * column the rows are ordered by, and there is at most one.
 */
function SortHead({
  label,
  sortKey,
  sort,
  onSort,
}: {
  label: string;
  sortKey: KhoaSapXep;
  sort: StaffSort | null;
  onSort: (key: KhoaSapXep) => void;
}) {
  const direction = sort !== null && sort.key === sortKey ? sort.order : null;
  return (
    <th
      scope="col"
      aria-sort={direction === null ? undefined : direction === "asc" ? "ascending" : "descending"}
    >
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className="flex items-center gap-1.5 border-0 bg-transparent p-0 [font-family:inherit] text-inherit [font-size:inherit] [font-weight:inherit] cursor-pointer"
      >
        {label}
        <ArrowUpDown aria-hidden="true" focusable="false" className="size-3 opacity-40" />
      </button>
    </th>
  );
}

/**
 * The prototype's six data columns, in its order (user 09/10/2026); the seventh is the actions column
 * with no visible head. `sortKey` where the server has a key for exactly what the column shows.
 * `Điện thoại` has NONE: the cell is the mobile, else the office line, and the server's `phone` key
 * orders by the office line alone — sorting by it would put rows in an order the column does not
 * show. No key is invented here; the head is plain text until the server offers one.
 */
const DATA_COLUMNS: readonly { readonly label: string; readonly sortKey?: KhoaSapXep }[] = [
  { label: "Họ và tên", sortKey: "full_name" },
  { label: "Chức danh", sortKey: "position" },
  { label: "Bộ phận", sortKey: "department" },
  { label: "Điện thoại" },
  { label: "Đăng nhập gần nhất", sortKey: "last_login_at" },
  { label: "Trạng thái", sortKey: "status" },
];

const NO_SORT = () => {};

// EXPORTED SO THE COLUMNS CAN BE PINNED BY A RENDER TEST: the parent reads the API in `useEffect`,
// which `renderToStaticMarkup` never runs.
export function BangCanBo({
  danhSach,
  thaoTac,
  traBoPhan,
  canDelete = false,
  sort = null,
  onSort = NO_SORT,
}: {
  danhSach: readonly identity_canBoTomTat[];
  thaoTac: ThaoTacDong;
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số. Không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  canDelete?: boolean;
  /** The applied order, for `aria-sort`. The screen owns it; the table only reports clicks. */
  sort?: StaffSort | null;
  onSort?: (key: KhoaSapXep) => void;
}) {
  return (
    // The prototype's frame (`border-line overflow-hidden rounded-[10px] border` around shadcn's Table),
    // as every config table draws it (`ConfigTable`); it scrolls sideways inside itself at 320px.
    <ConfigTable label="Danh sách cán bộ" caption="Danh sách cán bộ của đơn vị">
      <thead>
        <tr>
          {DATA_COLUMNS.map(({ label, sortKey }) =>
            sortKey === undefined ? (
              <th key={label} scope="col">
                {label}
              </th>
            ) : (
              <SortHead key={label} label={label} sortKey={sortKey} sort={sort} onSort={onSort} />
            ),
          )}
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
            const phone = phoneCell(cb.mobile, cb.phone);
            return (
              <tr key={cb.id}>
                <td>
                  <div className="text-navy font-semibold">{cb.full_name}</div>
                  <div className="text-ink-muted text-[11.5px]">{emailSubline(cb.email)}</div>
                </td>
                <td>{orDash(cb.position)}</td>
                <td>
                  <span className={unit.loai === "chuaGan" ? undefined : lopNhanDanhMuc(unit)}>{unitCellLabel(unit)}</span>
                </td>
                {/* ONE cell (user 09/10/2026): the mobile, else the office line, AS THE SERVER SENT IT —
                    no masking or unmasking here (#11/#16, Decree 13). `title` names which kind it is. */}
                <td title={phone.kind ?? undefined}>{phone.text}</td>
                <td>{nhanDangNhapGanNhat(cb.last_login_at)}</td>
                <td>
                  <StatusCell cb={cb} />
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
