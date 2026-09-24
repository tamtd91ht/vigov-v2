/**
 * Câu chữ và phép quyết định của màn **Danh bạ cán bộ** (`docs/ui-ux/12-danh-ba-can-bo.md`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÁCH KHỎI COMPONENT VÌ MỘT LÝ DO ĐÃ ĐO ĐƯỢC, không phải vì gọn: một quyết định nằm trong
 * module thuần kiểm được bằng một phép so chuỗi, còn cùng quyết định ấy viết thẳng trong JSX thì
 * chỉ kiểm được bằng cách kết xuất cả cây. Cùng khuôn với `components/danh-ba/nhan-ghi-danh-ba.ts`.
 *
 * MÀN NÀY ÍT HƠN ĐẶC TẢ RẤT NHIỀU, VÀ TỪNG PHẦN VẮNG MẶT ĐỀU CÓ LÝ DO Ở ĐÂY — `PHAN_CHUA_DUNG`
 * đưa đúng danh sách ấy RA MÀN HÌNH chứ không giấu trong chú thích. Một cán bộ mở `/danh-ba` và
 * không thấy ô tìm kiếm mà đặc tả vẽ sẽ kết luận hệ thống hỏng, rồi gọi lên tỉnh; thứ thật sự
 * thiếu là một tuyến API và hai quyết định của khách.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { KetQua } from "@/lib/api/goi";

/* ---- đầu trang ------------------------------------------------------------------------------ */

/** Tiêu đề trang — nguyên văn `docs/ui-ux/12-danh-ba-can-bo.md:3`. */
export const TIEU_DE_TRANG = "Danh bạ cán bộ";

/**
 * Câu mô tả dưới tiêu đề.
 *
 * NỬA ĐẦU LÀ NGUYÊN VĂN ĐẶC TẢ, NỬA SAU THÌ KHÔNG — VÀ ĐÓ LÀ CHỦ Ý. Đặc tả (§1, §2) viết:
 * *"Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App' để bà con
 * gọi được."* Vế thứ hai mô tả một nút mà màn hình này KHÔNG có và không được có: câu mở #12 (chốt
 * 22/09/2026) đã bỏ hẳn thao tác bật hàng loạt, và lược đồ chưa có cột nào giữ sự đồng ý của từng
 * người. In nguyên vế ấy ra là hứa với cán bộ một nút họ sẽ đi tìm và không thấy — tệ hơn hẳn việc
 * nói thẳng nó chưa mở.
 */
export const MO_TA_TRANG =
  "Toàn bộ cán bộ của xã: chức vụ, khối/đơn vị và số liên hệ. Việc công khai số lên Zalo Mini " +
  "App chưa mở — xem phần giải thích bên dưới.";

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

/** Nhãn thẻ KPI duy nhất dựng được — đặc tả §2 viết `SỐ KHỐI / ĐƠN VỊ`. */
export const NHAN_SO_KHOI = "Số khối / đơn vị";

/**
 * Trạng thái của con số "số khối / đơn vị". BA pha, không hai.
 *
 * "Chưa đọc xong" và "đọc xong, được 0" là hai sự thật khác nhau về một xã: cái sau nghĩa là sơ
 * đồ tổ chức còn trống và người quản trị phải làm gì đó.
 */
export type SoKhoi =
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; so: number };

/**
 * Đếm số khối / đơn vị từ kết quả đọc `GET /api/v1/org-units`.
 *
 * ĐẾM ĐƯỢC VÌ TUYẾN ẤY TRẢ NGUYÊN DANH SÁCH, KHÔNG PHÂN TRANG (`lib/api/danh-muc.ts`). Đó cũng
 * chính là lý do hai thẻ KPI kia KHÔNG dựng được: xem `PHAN_CHUA_DUNG`.
 */
export function demSoKhoi(kq: KetQua<{ items: readonly unknown[] }> | null): SoKhoi {
  if (kq === null) return { pha: "dangDoc" };
  if (!kq.ok) return { pha: "loi", thongBao: kq.thongBao };
  return { pha: "xong", so: kq.duLieu.items.length };
}

/**
 * Chữ hiện trong thẻ KPI.
 *
 * KHÔNG BAO GIỜ TRẢ CHUỖI RỖNG, và không bao giờ trả `0` cho ca chưa đọc xong: một thẻ hiện số 0
 * trong lúc còn đang đọc là một câu khẳng định sai về sơ đồ tổ chức của một cơ quan nhà nước.
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

/* ---- bảng ----------------------------------------------------------------------------------- */

/**
 * Nhãn cột — mỗi nhãn là đúng chữ của đặc tả §4, TRỪ cột điện thoại.
 *
 * ĐẶC TẢ CÓ MỘT CỘT `Di động`; Ở ĐÂY CÓ HAI, VÀ NHÃN NÓI RÕ LOẠI SỐ. Câu mở #16 (chốt 22/09/2026):
 * máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai
 * địa vị pháp lý khác nhau nghĩa là hai luật che, hai luật xuất Excel, hai luật công khai ra Mini
 * App — và một nhãn trung tính là chỗ người sắp bấm nút xuất không biết mình đang đụng loại nào.
 */
export const COT_HO_TEN = "Họ và tên";
export const COT_CHUC_VU = "Chức vụ";
export const COT_KHOI = "Khối / đơn vị";
export const COT_MAY_BAN = "Máy bàn cơ quan";
export const COT_DI_DONG = "Di động cá nhân";

/**
 * Nhãn nút sửa — chứa NGUYÊN VĂN chuỗi `15-phu-luc-giao-dien-chung.md §8` yêu cầu giữ.
 *
 * `ariaSua` GẮN THÊM TÊN NGƯỜI vào sau chuỗi ấy chứ không thay nó. Hai mươi dòng cho ra hai mươi
 * nút đọc lên giống hệt nhau là danh sách mà người dùng trình đọc màn hình không chọn đúng được
 * dòng nào — và ở đây chọn nhầm dòng nghĩa là sửa hồ sơ của một cán bộ khác.
 */
export const NUT_SUA_THONG_TIN = "Sửa thông tin cán bộ";

export function ariaSua(hoTen: string): string {
  return `${NUT_SUA_THONG_TIN}: ${hoTen}`;
}

/**
 * Câu cho một danh bạ rỗng.
 *
 * TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI (`15-phu-luc §6`): một xã vừa onboard có danh bạ
 * rỗng thật, và máy chủ trả `items: []` chứ không bao giờ trả `null`.
 */
export const DANH_BA_RONG =
  "Đơn vị chưa có cán bộ nào trong danh bạ. Khi cán bộ được thêm vào, danh sách sẽ hiện ở đây.";

/**
 * Câu nói rõ số di động ở màn này KHÔNG bị che, và vì sao.
 *
 * ĐÂY LÀ QUYẾT ĐỊNH #11, CHỐT 22/09/2026: không che trong nội bộ xã — cán bộ cùng xã cần gọi nhau
 * để làm việc, che thì họ truyền số qua kênh riêng và hệ thống mất cả vết lẫn quyền kiểm soát.
 * Phạm vi vẫn đóng chặt vì kho buộc `tenant_id`. Quyết định ấy CHỈ nói về màn hình nội bộ: bản
 * xuất Excel và mọi đường ra ngoài cơ quan VẪN CHE (luật 3, bất biến 4).
 *
 * Câu này phải có mặt trên màn hình vì người đọc cần biết mình đang nhìn dữ liệu cá nhân chưa che
 * của đồng nghiệp — không phải một bản đã che sẵn mà họ được phép chụp lại và gửi đi.
 */
export const GHI_CHU_SO_DIEN_THOAI =
  "Số di động cá nhân hiện đầy đủ cho cán bộ trong cùng đơn vị để liên hệ công việc. Đây là dữ " +
  "liệu cá nhân theo Nghị định 13/2023/NĐ-CP: không sao chép ra ngoài cơ quan và không công khai " +
  "khi chưa có sự đồng ý của chính người đó.";

/* ---- những phần đặc tả vẽ mà màn này KHÔNG dựng --------------------------------------------- */

/**
 * Một mục của danh sách "đặc tả có, ở đây không". `viSao` phải nói cả CÁI GÌ MỞ KHOÁ nó.
 *
 * CÙNG TÊN HẰNG, CÙNG HAI KHOÁ `ten` / `viSao` như các màn khác (`nhiem-vu`, `bien-ban`), vì
 * `tools/tien_do_san_pham.py` đếm đúng hình ấy: tìm chữ `PHAN_CHUA_DUNG` rồi đếm mọi dòng
 * `ten: "` tới HẾT TỆP. Đặt tên khác là báo cáo tiến độ in "không khai" trong khi màn vẫn hiện đủ
 * bảy dòng — và vì thế mảng dưới đây phải là khối CUỐI CÙNG có dòng `ten:` trong tệp này.
 */
export type PhanChuaDung = { readonly ten: string; readonly viSao: string };

/**
 * Bảy phần đặc tả vẽ mà hợp đồng hoặc một quyết định của khách chưa cho phép dựng.
 *
 * ĐƯA RA MÀN HÌNH, KHÔNG GIẤU TRONG CHÚ THÍCH. Vẽ ra một điều khiển không chạy được tệ hơn hẳn
 * không vẽ; nhưng KHÔNG vẽ mà cũng không nói gì thì cán bộ đi tìm một thứ đặc tả đã hứa với họ.
 * Mỗi dòng nói luôn cái gì mở khoá nó, để lần sau không ai phải ngồi suy lại.
 *
 * KHÔNG DÒNG NÀO Ở ĐÂY LÀ "CHƯA LÀM TỚI". Ba dòng đầu thiếu tuyến API; hai dòng giữa thiếu cột
 * trong lược đồ VÀ bị chặn bởi một quyết định đã chốt; dòng cuối thiếu một khoá quyền mà bảng
 * `quyen` không có.
 */
export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Ô tìm theo tên, chức vụ, số điện thoại (§3)",
    viSao:
      "GET /api/v1/staff không nhận tham số tìm kiếm nào — hợp đồng chỉ có limit, cursor, sort, " +
      "order. Lọc tại trình duyệt chỉ lọc được 20 dòng của trang đang mở, nên gõ tên một người ở " +
      "trang sau sẽ ra danh sách rỗng và người tìm kết luận sai rằng người đó không có trong hệ " +
      "thống. Mở khoá bằng một tham số truy vấn mới trên tuyến ấy.",
  },
  {
    ten: "Bộ lọc theo khối / đơn vị và theo trạng thái hiển thị (§3)",
    viSao: "Cùng tuyến, cùng lý do, cùng cách mở khoá như ô tìm kiếm.",
  },
  {
    ten: "Thẻ TỔNG SỐ CÁN BỘ (§2)",
    viSao:
      "Hợp đồng cố ý không trả tổng số: máy chủ đọc theo mốc (keyset) và không chạy COUNT(*) trên " +
      "bảng đã phân mảnh. Đếm số dòng của trang đang mở rồi gọi đó là tổng là báo một con số " +
      "không ai tính.",
  },
  {
    ten: "Thẻ ĐANG HIỆN TRÊN MINI APP, cột Trên Mini App, và hai nút thêm/rút khỏi danh bạ (§2, §4)",
    viSao:
      "Lược đồ không có trường hien_tren_mini_app, và câu mở #12 (chốt 22/09/2026) đã bỏ hẳn thao " +
      "tác bật hàng loạt: công khai số di động ra kênh công khai phải HỎI Ý từng người và LƯU LẠI " +
      "sự đồng ý kèm thời điểm. Chưa có chỗ nào giữ bằng chứng ấy, và phần đã công khai thì không " +
      "thu lại được.",
  },
  {
    ten: "Dòng phụ Có Zalo, ảnh đại diện, thứ tự hiển thị (§4, §5)",
    viSao: "Không có trường nào tương ứng trong hợp đồng REST (identity.canBoTomTat).",
  },
  {
    ten: "Nhập từ Excel và tải mẫu Excel (§6)",
    viSao:
      "Không có tuyến nào trong hợp đồng. Một bản nhập khớp theo email hoặc họ tên + khối còn cần " +
      "một quy tắc gộp bản ghi mà máy chủ phải là nơi quyết định, không phải trình duyệt.",
  },
  {
    ten: "Nút Xoá khỏi danh bạ (§4)",
    viSao:
      "Câu mở #10 tách XOÁ khỏi KHOÁ và cho xoá một quyền riêng; bảng quyen chưa có khoá ấy, nên " +
      "tuyến xoá chưa tồn tại. Cán bộ nghỉ hưu hoặc chuyển công tác thì khoá tài khoản ở màn Cấu " +
      "hình, không xoá — hồ sơ đã xử lý phải còn đọc được tên người thực hiện.",
  },
];

/** Tiêu đề của khối giải thích trên. */
export const TIEU_DE_PHAN_CHUA_DUNG = "Những phần chưa mở trên màn hình này";
