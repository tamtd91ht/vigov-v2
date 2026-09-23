import { coQuyen } from "@/lib/quyen";
import {
  QUYEN_CHUYEN_VAN_BAN,
  QUYEN_QUAN_LY_DANH_MUC,
  QUYEN_QUAN_LY_NGUOI_DUNG,
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_PHAN_ANH,
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
  /** Đường dẫn — `null` nghĩa là CHƯA CÓ MÀN, xem `CHUA_CO_MAN` dưới. */
  readonly duong: string | null;
  /** Khoá quyền cần có để thấy mục. `null` = ai đăng nhập cũng thấy. */
  readonly khoa: string | null;
};

export type NhomMenu = {
  readonly ten: string;
  readonly muc: readonly MucMenu[];
};

/**
 * VÌ SAO CHÍN MỤC CÓ `duong: null` VẪN NẰM TRONG DANH SÁCH, thay vì bị xoá đi.
 *
 * Kho này có tiền lệ rõ và đúng: `dau-trang.tsx` TỪ CHỐI vẽ ô tìm kiếm vì *"vẽ ra một ô tìm
 * kiếm không tìm được gì là hứa với cán bộ một chức năng không tồn tại"*. Luật ấy giữ nguyên ở
 * đây — chín mục này KHÔNG phải liên kết, không bấm được, không điều hướng đi đâu.
 *
 * Nhưng xoá hẳn chúng thì sai theo chiều ngược lại, và chiều ấy vừa gây thiệt hại thật: ngày
 * 23/09/2026 chủ dự án nói *"vậy mà tôi tưởng làm xong hết rồi"* sau khi đọc một con số tiến độ
 * đếm mục việc thay vì đếm sản phẩm. Một thanh menu chỉ hiện năm mục lặp lại đúng sự hiểu nhầm
 * ấy: nó trình bày một phần năm sản phẩm như thể đó là toàn bộ sản phẩm.
 *
 * Nên chúng hiện, có nhãn thật, và nói thẳng là chưa có. Đây cũng là khuôn màn Danh bạ vừa dùng:
 * đưa phần KHÔNG dùng được RA MÀN HÌNH kèm lý do, thay vì giấu trong chú thích hoặc vẽ một nút
 * chắc chắn hỏng.
 *
 * KHI MỘT MÀN RA ĐỜI: đổi `duong: null` thành đường dẫn thật và điền `khoa`. Không cần đụng
 * component.
 */
export const CHUA_CO_MAN = "Chưa có màn hình";

export const NHOM_MENU: readonly NhomMenu[] = [
  {
    ten: "ĐIỀU HÀNH",
    muc: [
      { nhan: "Tổng quan", duong: null, khoa: null },
      { nhan: "Nhiệm vụ", duong: null, khoa: null },
      { nhan: "Sổ tay lãnh đạo", duong: null, khoa: null },
      { nhan: "Biên bản họp", duong: null, khoa: null },
      { nhan: "Văn bản & Đơn thư", duong: "/van-ban", khoa: QUYEN_CHUYEN_VAN_BAN },
      { nhan: "Giải ngân", duong: "/giai-ngan", khoa: QUYEN_XEM_GIAI_NGAN },
      { nhan: "Thu - Chi ngân sách", duong: "/giai-ngan/thu-chi", khoa: QUYEN_XEM_GIAI_NGAN },
      { nhan: "Thông báo", duong: null, khoa: null },
      { nhan: "Phản ánh người dân", duong: "/phan-anh", khoa: QUYEN_XEM_PHAN_ANH },
      { nhan: "Bản đồ kinh tế số", duong: null, khoa: null },
    ],
  },
  {
    ten: "QUẢN TRỊ",
    muc: [
      { nhan: "Nội dung Mini App", duong: null, khoa: null },
      { nhan: "Danh bạ cán bộ", duong: "/danh-ba", khoa: QUYEN_QUAN_LY_NGUOI_DUNG },
      { nhan: "Báo cáo", duong: null, khoa: null },
      { nhan: "Cấu hình", duong: "/cau-hinh", khoa: QUYEN_QUAN_LY_DANH_MUC },
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
        return coQuyen(dsQuyen, m.khoa);
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
