import type { PendingFeatureInfo } from "@/components/ui/pending-feature";
import { coQuyen } from "@/lib/quyen";
import {
  ASSET_READ_PERMISSION,
  AUDIT_READ_PERMISSION,
  QUYEN_CAU_HINH_THOI_HAN,
  QUYEN_PHAN_QUYEN,
  QUYEN_QUAN_LY_DANH_MUC,
  QUYEN_QUAN_LY_NGUOI_DUNG,
  QUYEN_QUAN_LY_SO_DO,
  QUYEN_SOAN_THONG_BAO,
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_NHIEM_VU,
  QUYEN_XEM_NOI_DUNG,
  QUYEN_XEM_PHAN_ANH,
  QUYEN_XEM_VAN_BAN,
  REPORT_READ_PERMISSION,
} from "@/lib/quyen";

/**
 * Danh sách mục menu và luật lọc — `docs/ui-ux/15-phu-luc-giao-dien-chung.md` §2.
 *
 * MODULE THUẦN, TÁCH KHỎI COMPONENT, cùng lý do `KhungQuyen` được tách khỏi `CongQuyen`: hai
 * nhánh đáng kiểm nhất — "thiếu quyền" và "chưa đọc xong phiên" — là hai nhánh người viết mã
 * không bao giờ nhìn thấy, vì tài khoản của họ luôn có đủ quyền. Nằm trong component thì chúng
 * chỉ kiểm được qua một trình duyệt giả lập; tách ra thì kiểm bằng một hàm.
 */

export type MucMenu = {
  /** Nhãn hiện trên menu, nguyên văn đặc tả §2. */
  readonly nhan: string;
  /** Đường dẫn — `null` nghĩa là CHƯA CÓ MÀN, xem `PENDING_SCREENS` dưới. */
  readonly duong: string | null;
  /**
   * Khoá quyền cần có để thấy mục. `null` = ai đăng nhập cũng thấy. Một DANH SÁCH = có bất kỳ khoá
   * nào trong đó — chỉ dành cho mục dẫn tới một màn NHIỀU TAB mà mỗi tab tự canh khoá riêng của
   * nó, xem `KHOA_MO_CAU_HINH`.
   */
  readonly khoa: string | readonly string[] | null;
};

/**
 * Khoá mở MỤC MENU "Cấu hình" — mỗi khoá là khoá canh một tab trên `/cau-hinh`, không hơn.
 *
 * VÌ SAO MỘT PHÉP HỢP Ở ĐÂY KHÔNG TRÁI `quyetDinhTheoKhoa` ("một hàm, một khoá"): mục menu là CỬA
 * VÀO một màn nhiều tab, không phải một tab. Tab nào vẫn tự quyết theo đúng một khoá của nó
 * (`features/cau-hinh/quyen-tab.ts`), nên có `admin.sla` thì vào được trang nhưng tab Nhật ký hệ
 * thống vẫn ẩn. Canh cửa bằng MỘT khoá (`admin.lookup`, trước 26/09/2026) thì cán bộ chỉ giữ `admin.sla`
 * không tìm thấy lối vào tab Thời hạn xử lý — mà bảng thời hạn rỗng là xã không nhận được phản ánh
 * nào (`identity.ResolveDeadlines` từ chối).
 *
 * DANH SÁCH ĐÓNG, LIỆT KÊ TỪNG KHOÁ, chứ không phải `admin.*`: `admin.user.delete` (nút ở
 * `/danh-ba`) cố ý VẮNG. `admin.audit` VẮNG cho tới 29/09/2026 vì chưa tab nào canh nó; nay tab Nhật
 * ký hệ thống canh nó, nên nó vào đây cùng lượt dựng tab (ADR 0054 §6). Thêm một tab có cổng quyền
 * mới thì thêm đúng khoá của tab ấy vào đây, và chỉ khoá có thật trong bảng `quyen` (luật 5, bất
 * biến 3c).
 *
 * `admin.user` VÀ `admin.role` RA KHỎI DANH SÁCH NGÀY 05/10/2026, cùng lượt hai tab của chúng rời
 * `/cau-hinh` thành hai mục menu riêng (`Người dùng`, `Phân quyền`, theo bản mẫu chủ dự án duyệt).
 * Giữ lại thì người chỉ có `admin.user` thấy mục Cấu hình dẫn tới một màn không còn phần nào khoá ấy
 * mở ra — lối vào việc của họ nay là mục `Người dùng`.
 *
 * Ẩn mục menu là tiện dụng, không phải biện pháp: mọi tuyến sau các tab tự kiểm khoá (luật 5, cấm #1).
 */
export const KHOA_MO_CAU_HINH: readonly string[] = [
  QUYEN_QUAN_LY_SO_DO, // Sơ đồ tổ chức — nút ghi
  QUYEN_QUAN_LY_DANH_MUC, // Danh mục — nút ghi
  QUYEN_CAU_HINH_THOI_HAN, // Thời hạn xử lý — bảng thời hạn và mọi nút ghi
  AUDIT_READ_PERMISSION, // Nhật ký hệ thống — cả tab
];

export type NhomMenu = {
  readonly ten: string;
  readonly muc: readonly MucMenu[];
};

/**
 * VÌ SAO CÁC MỤC CÓ `duong: null` VẪN NẰM TRONG DANH SÁCH, thay vì bị xoá đi. (No count here on
 * purpose: "chín mục" outlived the day it stopped being nine. The list below is the count.)
 *
 * Kho này có tiền lệ rõ và đúng: `dau-trang.tsx` TỪ CHỐI vẽ ô tìm kiếm vì *"vẽ ra một ô tìm
 * kiếm không tìm được gì là hứa với cán bộ một chức năng không tồn tại"*. Luật ấy giữ nguyên ở
 * đây — các mục này KHÔNG phải liên kết, không bấm được, không điều hướng đi đâu.
 *
 * Nhưng xoá hẳn chúng thì sai theo chiều ngược lại, và chiều ấy vừa gây thiệt hại thật: ngày
 * 23/09/2026 chủ dự án nói *"vậy mà tôi tưởng làm xong hết rồi"* sau khi đọc một con số tiến độ
 * đếm mục việc thay vì đếm sản phẩm. Một thanh menu chỉ hiện năm mục lặp lại đúng sự hiểu nhầm
 * ấy: nó trình bày một phần năm sản phẩm như thể đó là toàn bộ sản phẩm.
 *
 * So they show, with their real label, disabled, carrying the "?" of ADR 0068 §14: hover says
 * "Tính năng đang phát triển", pressing it reads the item's `PENDING_SCREENS` entry — what the screen
 * is and why it is not built. The same pattern every screen uses for its unbuilt parts: put what
 * does NOT work ON THE SCREEN with its reason, instead of hiding it in a comment or drawing a button
 * that surely fails.
 *
 * KEYED BY LABEL IN A SEPARATE TABLE, NOT A FIELD ON THE ITEM: `tools/tien_do_san_pham.py`
 * (`MAU_MUC`) reads each item as ONE `{ nhan, duong, khoa }` literal. An item with a fourth field
 * stops matching and silently drops out of the product progress count — the menu would then report
 * fewer unbuilt screens than it shows. `side-nav.test.tsx` holds that every `duong: null` item
 * has an entry here and no built item does.
 *
 * KHI MỘT MÀN RA ĐỜI: đổi `duong: null` thành đường dẫn thật, điền `khoa`, xoá dòng của nó trong
 * `PENDING_SCREENS`. Không cần đụng component.
 */
// EMPTY SINCE 04/10/2026: "Báo cáo" was the last item without a screen (`/bao-cao`, ADR 0053
// amendment). The table and its mechanism stay for the next item that is listed before it is built.
export const PENDING_SCREENS: Readonly<Record<string, PendingFeatureInfo>> = {};

/**
 * Grouped BY BUSINESS AREA (spec v2 §5, 02/10/2026). Only the grouping and the order moved; every
 * item keeps its label, route and permission key, and `locMenu` filters them exactly as before.
 *
 * `ten: ""` = a group drawn with no heading (Tổng quan stands alone at the top).
 *
 * "Thông báo" sits under CÔNG VIỆC, not NGƯỜI DÂN where the spec's table puts it: the spec says to
 * move it if it is staff-internal, and it is — `/thong-bao` is the internal announcement book sent to
 * departments (`docs/ui-ux/08-thong-bao.md` §1, `features/thong-bao/so-thong-bao.tsx`). Under
 * NGƯỜI DÂN it would read as a message to citizens, which it never is.
 *
 * "Người dùng" and "Phân quyền" (05/10/2026) are TWO FLAT ITEMS, where the prototype draws one parent
 * "Người dùng & Phân quyền" with these two as children. This menu has no child level, and adding one
 * means a fourth field on an item — which `tools/tien_do_san_pham.py` (`MAU_MUC`) would silently stop
 * counting. Same labels, routes, keys and order as the prototype's children; only the parent row is
 * missing. Each needs its own key, so a parent gated on either would open onto a screen that refuses.
 */
export const NHOM_MENU: readonly NhomMenu[] = [
  {
    ten: "",
    muc: [{ nhan: "Tổng quan", duong: "/tong-quan", khoa: REPORT_READ_PERMISSION }],
  },
  {
    ten: "CÔNG VIỆC",
    muc: [
      { nhan: "Nhiệm vụ", duong: "/nhiem-vu", khoa: QUYEN_XEM_NHIEM_VU },
      { nhan: "Biên bản họp", duong: "/nhiem-vu/bien-ban", khoa: QUYEN_XEM_NHIEM_VU },
      { nhan: "Văn bản & Đơn thư", duong: "/van-ban", khoa: QUYEN_XEM_VAN_BAN },
      { nhan: "Thông báo", duong: "/thong-bao", khoa: QUYEN_SOAN_THONG_BAO },
      { nhan: "Sổ tay lãnh đạo", duong: "/nhiem-vu/so-tay", khoa: QUYEN_XEM_NHIEM_VU },
    ],
  },
  {
    ten: "TÀI CHÍNH",
    muc: [
      { nhan: "Giải ngân", duong: "/giai-ngan", khoa: QUYEN_XEM_GIAI_NGAN },
      { nhan: "Thu - Chi ngân sách", duong: "/giai-ngan/thu-chi", khoa: QUYEN_XEM_GIAI_NGAN },
    ],
  },
  {
    ten: "NGƯỜI DÂN",
    muc: [
      { nhan: "Phản ánh người dân", duong: "/phan-anh", khoa: QUYEN_XEM_PHAN_ANH },
      { nhan: "Nội dung Mini App", duong: "/noi-dung", khoa: QUYEN_XEM_NOI_DUNG },
      { nhan: "Bản đồ kinh tế số", duong: "/ban-do", khoa: ASSET_READ_PERMISSION },
    ],
  },
  {
    ten: "HỆ THỐNG",
    muc: [
      { nhan: "Danh bạ cán bộ", duong: "/danh-ba", khoa: QUYEN_QUAN_LY_NGUOI_DUNG },
      { nhan: "Báo cáo", duong: "/bao-cao", khoa: REPORT_READ_PERMISSION },
      { nhan: "Người dùng", duong: "/nguoi-dung", khoa: QUYEN_QUAN_LY_NGUOI_DUNG },
      { nhan: "Phân quyền", duong: "/nguoi-dung/phan-quyen", khoa: QUYEN_PHAN_QUYEN },
      { nhan: "Cấu hình", duong: "/cau-hinh", khoa: KHOA_MO_CAU_HINH },
    ],
  },
];

/**
 * Lọc menu theo quyền của phiên hiện tại.
 *
 * `dsQuyen === null` nghĩa là CHƯA ĐỌC XONG phiên, không phải "không có quyền" — ba trạng thái,
 * không hai, đúng như `CongQuyen` phân biệt. Lúc ấy chỉ hiện các mục không cần quyền; hiện đủ
 * rồi rút bớt khi phiên về sẽ làm menu nhấp nháy, còn ẩn hết thì thanh bên trống trơn trong
 * khoảnh khắc đầu và người dùng tưởng mình mất quyền.
 *
 * MỤC CHƯA CÓ MÀN KHÔNG BỊ LỌC THEO QUYỀN, có chủ ý: chúng không mở ra dữ liệu nào cả, nên
 * không có gì để rò rỉ. Lọc chúng theo một khoá quyền sẽ khai một khoá cho một màn chưa tồn
 * tại — và khoá ấy sẽ phải đoán, vì chưa tuyến nào khai nó.
 */
export function locMenu(
  nhom: readonly NhomMenu[],
  dsQuyen: readonly string[] | null,
): readonly NhomMenu[] {
  return nhom
    .map((n) => ({
      ten: n.ten,
      muc: n.muc.filter((m) => {
        if (m.duong === null) return true;
        if (m.khoa === null) return true;
        if (dsQuyen === null) return false;
        if (typeof m.khoa === "string") return coQuyen(dsQuyen, m.khoa);
        return m.khoa.some((k) => coQuyen(dsQuyen, k));
      }),
    }))
    .filter((n) => n.muc.length > 0);
}

/** Mục đang chọn: khớp CHÍNH XÁC hoặc là tiền tố theo đoạn đường dẫn. */
export function dangChon(duongMuc: string | null, duongHienTai: string): boolean {
  if (duongMuc === null) return false;
  if (duongMuc === duongHienTai) return true;
  // `/giai-ngan` phải sáng khi đang ở `/giai-ngan/thu-chi`, nhưng `/van-ban` KHÔNG được sáng khi
  // ở `/van-ban-khac`. So theo ĐOẠN, không so chuỗi trần.
  return duongHienTai.startsWith(duongMuc + "/");
}
