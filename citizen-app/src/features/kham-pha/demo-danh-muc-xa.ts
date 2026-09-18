/**
 * DANH MỤC XÃ CHO BẢN TRÌNH DIỄN — DỮ LIỆU TẠM, CÓ HẠN SỬ DỤNG, PHẢI XOÁ.
 *
 * ┌────────────────────────────────────────────────────────────────────────────────────────┐
 * │ TỆP NÀY KHÔNG ĐƯỢC ĐI THEO BẢN PHỤC VỤ NGƯỜI THẬT.                                     │
 * │ Một cơ quan nhà nước hiển thị đơn vị hành chính không tồn tại là một sự cố, không phải  │
 * │ một lỗi giao diện. Trong một buổi trình diễn thì đây là dữ liệu mẫu và được nói ra như  │
 * │ vậy trên màn hình; trong bản phát hành thì nó là một tuyên bố sai dưới tên một cơ quan. │
 * └────────────────────────────────────────────────────────────────────────────────────────┘
 *
 * NGUỒN THẬT LÀ GÌ, VÀ VÌ SAO CHƯA DÙNG ĐƯỢC:
 *
 *   `ListTenants` của service `platform` — đã khai trong `proto/vigov/platform/v1/platform.proto`
 *   và chú thích ở đó nói đúng việc này: kênh công dân không có tên miền để suy ra xã, nên phải
 *   cho công dân thấy danh sách xã TRƯỚC khi biết xã nào. Nó trả về tên và tỉnh/thành của các xã
 *   ĐANG HOẠT ĐỘNG, và không có trường nào chở được dữ liệu nghiệp vụ.
 *
 *   RPC ấy **chưa có cài đặt**, và chú thích trong proto nói rõ điều kiện tiên quyết: cổng gRPC
 *   của service `platform` chưa có xác thực người gọi, nên `ListTenants` đang bị từ chối bằng
 *   INVALID_ARGUMENT. Xác thực trước, mở RPC sau. Tệp này biến mất ở bước "sau".
 *
 * VÌ SAO TÊN ĐƯỢC ĐẶT RA CHỨ KHÔNG LẤY TÊN XÃ CÓ THẬT — đây là lựa chọn an toàn hơn, không phải
 * lựa chọn tiện hơn:
 *
 *   Một danh mục demo mang tên xã có thật ngụ ý rằng xã ấy **đã là khách hàng của hệ thống**.
 *   Đó là một hàm ý sai, và nó được đưa ra trước đúng những người sẽ hỏi lại. Tên đặt ra không
 *   mang hàm ý đó. Cặp (tên xã, tỉnh/thành) dưới đây không ứng với đơn vị hành chính nào: các
 *   tỉnh/thành ở đây không có trong danh sách 34 tỉnh/thành sau sắp xếp 01/7/2025.
 *
 * VÌ SAO DANH SÁCH CÓ CẢ XÃ, PHƯỜNG VÀ ĐẶC KHU:
 *
 *   Từ 01/7/2025 chính quyền địa phương hai cấp: tỉnh → **xã · phường · đặc khu**, cấp huyện
 *   không còn. Một danh mục chỉ toàn "Xã" là một danh mục kể thiếu, và nó dạy người đọc mã một
 *   mô hình dữ liệu sai — rằng loại đơn vị là một hằng số chứ không phải một trường.
 *
 * VÌ SAO MÃ LÀ ULID CHỨ KHÔNG PHẢI MÃ HÀNH CHÍNH:
 *
 *   Luật 1, bất biến 2. `tenant_id` là định danh mờ, bất biến. QR in ở trụ sở xã sống nhiều năm
 *   và phải sống qua một lần sáp nhập; một mã mang nghĩa thì lần sáp nhập đầu tiên buộc phải
 *   viết lại khoá ngoại trên hồ sơ lưu trữ. Mã ở đây mang tiền tố `01JDEMXA` để bất kỳ ai nhìn
 *   thấy nó trong log hay trong một đường liên kết đều biết ngay đó là dữ liệu của bản trình
 *   diễn chứ không phải một xã thật.
 */

export type XaDemo = {
  /** ULID. Mờ và bất biến — không phải mã hành chính, không phải tên miền (luật 1, bất biến 2). */
  id: string;
  /** Tên hiển thị đầy đủ, kèm loại đơn vị. Đây là thứ công dân xác nhận, không phải mã. */
  ten: string;
  /** Tỉnh/thành. Cột này tồn tại vì hai xã trùng tên ở hai tỉnh là chuyện bình thường. */
  tinh: string;
};

/**
 * Danh mục mẫu. Tám dòng — đủ để màn chọn xã trông như một danh mục thật, đủ ngắn để không cần
 * ô tìm kiếm (giai đoạn 1 không có thẻ nhập liệu nào, xem `phase1-collects-nothing.test.ts`).
 */
export const DEMO_DANH_MUC_XA: readonly XaDemo[] = [
  { id: "01JDEMXA00000000000000000A", ten: "Xã An Thịnh", tinh: "Tỉnh Đông Hải" },
  { id: "01JDEMXA00000000000000000B", ten: "Xã Bình Khê", tinh: "Tỉnh Đông Hải" },
  { id: "01JDEMXA00000000000000000C", ten: "Đặc khu Hòn Mây", tinh: "Tỉnh Đông Hải" },
  { id: "01JDEMXA00000000000000000D", ten: "Phường Tân Lộc", tinh: "Thành phố Nam Giang" },
  { id: "01JDEMXA00000000000000000E", ten: "Phường Hoà Mỹ", tinh: "Thành phố Nam Giang" },
  { id: "01JDEMXA00000000000000000F", ten: "Phường Trung Chánh", tinh: "Thành phố Nam Giang" },
  { id: "01JDEMXA00000000000000000G", ten: "Xã Cẩm Sơn", tinh: "Tỉnh Tây Hoà" },
  { id: "01JDEMXA00000000000000000H", ten: "Xã Long Phú", tinh: "Tỉnh Tây Hoà" },
];

/**
 * Câu chữ hiện dưới danh mục. Nó nằm ở ĐÂY, cạnh dữ liệu nó nói về, chứ không nằm trong màn
 * hình: xoá tệp này thì câu ấy mất theo, và không còn một lời chú thích mồ côi nói về một danh
 * mục không còn tồn tại.
 */
export const DEMO_GHI_CHU = "Danh mục mẫu dùng cho bản trình diễn.";

/**
 * Tra một mã sang xã. **Không tìm thấy thì trả `null`, không đoán** — luật 1 cấm mặc định trên
 * đường cô lập, và ở đây "đoán" nghĩa là hiển thị một tên xã cho công dân xác nhận trong khi
 * không biết mã ấy trỏ vào đâu. Công dân xác nhận theo TÊN, nên một cái tên sai ở bước này là
 * một hồ sơ gửi nhầm cơ quan.
 */
export function DEMO_timTheoMa(ma: string): XaDemo | null {
  return DEMO_DANH_MUC_XA.find((xa) => xa.id === ma) ?? null;
}
