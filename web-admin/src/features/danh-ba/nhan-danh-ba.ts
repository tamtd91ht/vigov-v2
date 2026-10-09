/**
 * Câu chữ và phép quyết định của màn **Danh bạ cán bộ** (`docs/ui-ux/12-danh-ba-can-bo.md`, bản mẫu
 * `StaffDirectoryWorkspace.tsx`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÁCH KHỎI COMPONENT VÌ MỘT LÝ DO ĐÃ ĐO ĐƯỢC, không phải vì gọn: một quyết định nằm trong
 * module thuần kiểm được bằng một phép so chuỗi, còn cùng quyết định ấy viết thẳng trong JSX thì
 * chỉ kiểm được bằng cách kết xuất cả cây. Cùng khuôn với `components/danh-ba/nhan-ghi-danh-ba.ts`.
 *
 * TỪNG PHẦN BẢN MẪU CHƯA DỰNG ĐỀU CÓ LÝ DO Ở ĐÂY — `PHAN_CHUA_DUNG` là mô tả sau dấu "?" của
 * phần ấy, đặt ngay trên màn hình chứ không giấu trong chú thích.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { KetQua } from "@/lib/api/goi";
import type { StaffTallies } from "@/lib/api/can-bo";

/* ---- đầu trang ------------------------------------------------------------------------------ */

/** Tiêu đề trang — nguyên văn bản mẫu. */
export const TIEU_DE_TRANG = "Danh bạ cán bộ";

/**
 * Câu mô tả dưới tiêu đề — NGUYÊN VĂN bản mẫu (`StaffDirectoryWorkspace.tsx:149-152`).
 *
 * "Chọn người cần công khai rồi bấm …" nay đúng với màn hình: bảng có cột chọn và thanh hàng loạt.
 * Sự đồng ý của TỪNG người (#12, Nghị định 13) vẫn được hỏi — nút ấy mở hộp xác nhận cho từng người
 * đã chọn (`bulk-consent-form.tsx`), không công khai thẳng.
 */
export const MO_TA_TRANG =
  "Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm “Thêm vào danh bạ Mini App” để bà con gọi được.";

/** Hai nút bên phải đầu trang — nguyên văn bản mẫu. Mở hộp thoại ngay tại tab (chủ đầu tư chốt 09/10/2026). */
export const IMPORT_BUTTON = "Nhập từ Excel";
export const ADD_BUTTON = "Thêm cán bộ";

/**
 * Câu hiện khi tài khoản thiếu `admin.user`.
 *
 * NÓI RA TÊN KHOÁ. "Bạn không có quyền" trống trơn là câu khiến cán bộ gọi lên tỉnh hỏi mình
 * thiếu quyền gì; tên khoá là thứ quản trị viên của xã tìm được ngay trên màn Phân quyền.
 */
export const CAU_THIEU_QUYEN =
  "Tài khoản của bạn không có quyền quản lý người dùng (admin.user), nên danh bạ cán bộ không " +
  "hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/* ---- thẻ KPI -------------------------------------------------------------------------------- */

/** Nhãn ba thẻ — nguyên văn bản mẫu. */
export const KPI_TOTAL = "Tổng số cán bộ";
export const KPI_PUBLISHED = "Đang hiện trên Mini App";
export const NHAN_SO_KHOI = "Số khối / đơn vị";

/**
 * Trạng thái của một con số đếm. BA pha, không hai: "chưa đọc xong" và "đọc xong, được 0" là hai
 * sự thật khác nhau về một xã.
 */
export type SoKhoi =
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; so: number };

/** Đếm số mục của `GET /api/v1/org-units` — tuyến ấy trả nguyên danh sách, không phân trang. */
export function demSoKhoi(kq: KetQua<{ items: readonly unknown[] }> | null): SoKhoi {
  if (kq === null) return { pha: "dangDoc" };
  if (!kq.ok) return { pha: "loi", thongBao: kq.thongBao };
  return { pha: "xong", so: kq.duLieu.items.length };
}

/**
 * Chữ hiện trong thẻ KPI.
 *
 * KHÔNG BAO GIỜ TRẢ CHUỖI RỖNG, và không bao giờ trả `0` cho ca chưa đọc xong: một thẻ hiện số 0
 * trong lúc còn đang đọc là một câu khẳng định sai về một cơ quan nhà nước.
 */
export function nhanSoKhoi(so: SoKhoi): string {
  switch (so.pha) {
    case "dangDoc":
      return "đang đếm…";
    case "loi":
      return "chưa đọc được";
    case "xong":
      return String(so.so);
  }
}

/** Huy hiệu "N khối / đơn vị" — chỉ ghép con số khi đã đếm xong. */
export function unitCountText(so: SoKhoi): string {
  return so.pha === "xong" ? `${so.so} khối / đơn vị` : `${NHAN_SO_KHOI}: ${nhanSoKhoi(so)}`;
}

/**
 * The three cards' figures, from ONE read of `GET /api/v1/staff-counts` (`getStaffTallies`). `null` =
 * not read yet. Each card goes through `nhanSoKhoi`, so a card never shows `0` while still counting.
 *
 * "SỐ KHỐI / ĐƠN VỊ" COUNTS THE DEPARTMENTS THAT HAVE STAFF — the entries of `departments`, which
 * holds exactly the departments with somebody in them. That is the prototype's figure (the number of
 * groups in its directory), not the size of the org chart.
 */
export function tallyFigures(t: KetQua<StaffTallies> | null): {
  readonly total: string;
  readonly published: string;
  readonly departments: string;
} {
  const pick = (f: (x: StaffTallies) => number): string =>
    nhanSoKhoi(
      t === null ? { pha: "dangDoc" } : t.ok ? { pha: "xong", so: f(t.duLieu) } : { pha: "loi", thongBao: t.thongBao },
    );
  return {
    total: pick((x) => x.total),
    published: pick((x) => x.published),
    departments: pick((x) => x.departments.length),
  };
}

/**
 * How many people the list shows under the filters in force, WITHOUT search words — derived from
 * the tallies, never from the page length (the route is paginated). A department absent from
 * `departments` has nobody in it: 0/0 (`staff_counts.go`). `congKhai` `false` = "Chưa hiện".
 */
export function filteredTotal(t: StaffTallies, unitId: string, published: boolean | null): number {
  const scope =
    unitId === ""
      ? { total: t.total, published: t.published }
      : (t.departments.find((d) => d.id === unitId) ?? { total: 0, published: 0 });
  if (published === true) return scope.published;
  if (published === false) return scope.total - scope.published;
  return scope.total;
}

/**
 * One option of the department filter: "Tên (đang hiện/tổng)", the prototype's wording. Without
 * tallies (still reading, or unreadable) the name alone — never a guessed "(0/0)".
 */
export function departmentOptionText(name: string, id: string, t: StaffTallies | null): string {
  if (t === null) return name;
  const d = t.departments.find((x) => x.id === id);
  return `${name} (${d?.published ?? 0}/${d?.total ?? 0})`;
}

/** The line under the table — nguyên văn bản mẫu. `n` is the total for the filter, not the page. */
export function shownCountText(n: number): string {
  return `Hiển thị ${n} cán bộ.`;
}

/* ---- trạng thái của vùng danh sách ---------------------------------------------------------- */

/** Tiêu đề khi đọc danh sách hỏng; câu bên dưới là câu NGUYÊN VĂN của máy chủ. */
export const LOAD_FAILED_TITLE = "Chưa tải được danh sách";
/** Nút đọc lại đúng trang đang xem, cùng bộ lọc — cơ chế đọc lại sau mỗi lần ghi. */
export const RELOAD = "Tải lại";

/** Hai nút phân trang; vùng `nav` đã nói là phân trang. */
export const PAGE_PREVIOUS = "Trước";
export const PAGE_NEXT = "Sau";

/* ---- bảng ----------------------------------------------------------------------------------- */

/**
 * Nhãn cột — chữ của bản mẫu, TRỪ cột điện thoại.
 *
 * BẢN MẪU CÓ MỘT CỘT `Di động`; Ở ĐÂY CÓ HAI, VÀ NHÃN NÓI RÕ LOẠI SỐ. Câu mở #16 (chốt 22/09/2026):
 * máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13.
 */
export const COT_HO_TEN = "Họ và tên";
export const COT_CHUC_VU = "Chức vụ";
export const COT_KHOI = "Khối / đơn vị";
export const COT_MAY_BAN = "Máy bàn cơ quan";
export const COT_DI_DONG = "Di động cá nhân";

/**
 * Nhãn nút sửa. `ariaSua` GẮN THÊM TÊN NGƯỜI: hai mươi nút đọc lên giống hệt nhau là danh sách mà
 * người dùng trình đọc màn hình không chọn đúng được dòng nào.
 */
export const NUT_SUA_THONG_TIN = "Sửa thông tin cán bộ";

export function ariaSua(hoTen: string): string {
  return `${NUT_SUA_THONG_TIN}: ${hoTen}`;
}

/** The row's selection box, and the page's — each names what it selects, for the reason above. */
export function selectRowLabel(fullName: string): string {
  return `Chọn ${fullName}`;
}
export const SELECT_PAGE_LABEL = "Chọn tất cả cán bộ trên trang này";

/**
 * Câu cho một danh bạ rỗng — NGUYÊN VĂN bản mẫu (`StaffDirectoryWorkspace.tsx:283-284`). Nút "Nhập
 * từ Excel" nó nhắc tới nằm ngay trên đầu tab.
 */
export const DANH_BA_RONG =
  "Chưa có cán bộ nào. Tải mẫu Excel ở nút “Nhập từ Excel”, điền theo bảng danh bạ xã đang dùng rồi tải lên.";

/**
 * Câu cho một danh sách rỗng KHI ĐANG TÌM HOẶC LỌC.
 *
 * KHÁC `DANH_BA_RONG`, VÀ PHẢI KHÁC: "chưa có cán bộ nào" in ra dưới một ô tìm vừa gõ sai chính tả là
 * một câu khẳng định sai về cả cơ quan. Câu này KHÔNG nhắc lại chữ đã tìm.
 */
export const KHONG_KHOP_LOC =
  "Không có cán bộ nào khớp điều kiện tìm kiếm hoặc bộ lọc đang chọn. Thử bỏ bớt điều kiện.";

/* ---- những phần bản mẫu vẽ mà màn này KHÔNG dựng ------------------------------------------- */

/**
 * Một mục của danh sách "bản mẫu có, ở đây không". `viSao` phải nói cả CÁI GÌ MỞ KHOÁ nó.
 *
 * CÙNG TÊN HẰNG, CÙNG HAI KHOÁ `ten` / `viSao` như các màn khác, vì `tools/tien_do_san_pham.py` đếm
 * các dòng `ten: "` NẰM TRONG khối khai báo `PHAN_CHUA_DUNG` (tới dấu `];` đầu dòng).
 */
export type PhanChuaDung = { readonly ten: string; readonly viSao: string };

/**
 * Những phần bản mẫu vẽ mà hợp đồng chưa cho phép dựng. Mỗi mục ra tới màn hình ở đúng chỗ bản mẫu
 * đặt nó (ADR 0068 §14), là control nó sẽ là, bị vô hiệu, mang dấu "?". Dựng xong một phần thì XOÁ
 * dòng của nó ở đây.
 *
 * Ba mục cũ đã ra khỏi danh sách (09/10/2026): hai thẻ KPI nay đọc `GET /api/v1/staff-counts`, và
 * cột / ô Ảnh đại diện bỏ hẳn vì bản mẫu không có.
 */
export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  // No bulk soft-delete route: the server deletes one staff row per call, each with its own reason
  // (#10, rule 7). A loop of single deletes would record ONE reason for many archival records.
  {
    ten: "Xoá đã chọn",
    viSao:
      "Hệ thống chưa có cách xoá nhiều cán bộ trong một lần. Mỗi lần xoá cần một lý do riêng cho " +
      "từng người, nên hãy xoá từng người bằng nút thùng rác trên dòng của người đó.",
  },
];

/** One entry by its `ten`. Throws on an unknown name — a "?" with no description is never drawn. */
export function pendingPart(ten: string): PhanChuaDung {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
