/**
 * ĐỊA CHỈ MÁY CHỦ — MỘT CHỖ ĐỌC, CHO CẢ HAI HỢP ĐỒNG.
 *
 * ⚠ TỆP NÀY RA ĐỜI 22/09/2026 ĐỂ CHẶN MỘT BẢN SAO TRƯỚC KHI NÓ TỒN TẠI.
 *
 *   `features/dang-nhap/hop-dong.ts` đã đọc `__VIGOV_API_HOST__` và cắt dấu `/` thừa. Tuyến thứ
 *   hai (`/api/v1/requests`) cần đúng hai việc ấy. Chép chúng sang tệp thứ hai là dựng hai chỗ
 *   giữ một quy ước — và ngày ai đó sửa cách cắt dấu `/` ở một bên, bản còn lại gửi tới một địa
 *   chỉ có hai dấu gạch chéo, tức một `404` không ai đọc ra nguyên nhân.
 *
 * ⚠ ĐỊA CHỈ LÀ MỘT, KHÔNG PHẢI MỘT ĐỊA CHỈ CHO MỖI ĐƠN VỊ. Máy chủ là bên ghi phiên đang làm
 * việc với ai; client không được tự chọn máy chủ của mình.
 * → `.claude/skills/zalo-miniapp-multi-tenant` §"The API host is singular".
 *
 * ⚠ ĐỌC LÚC DỰNG, KHÔNG PHẢI LÚC CHẠY. `vite.config.ts` thay chuỗi `__VIGOV_API_HOST__` bằng giá
 * trị biến môi trường `VIGOV_API_HOST` ở bước dựng.
 *
 *   `typeof` chứ không đọc thẳng: khi chạy dưới Vitest mà không có bước thay chuỗi thì đọc thẳng
 *   là `ReferenceError` làm sập cả tệp test, còn `typeof` trên một tên chưa khai là hợp lệ. Hỏng
 *   theo hướng "chưa khai host" thì màn hình nói ra một câu tiếng Việt; hỏng theo hướng kia là
 *   một màn trắng.
 *
 * ⚠ KHÔNG CÓ HOST MẶC ĐỊNH, VÀ ĐÓ LÀ FAIL-CLOSED: đoán một địa chỉ là gửi dữ liệu của một người
 * thật tới một máy chủ không ai chọn. `scripts/deploy.mjs` chặn đường đẩy khi biến chưa khai.
 */
declare const __VIGOV_API_HOST__: string | undefined;

const API_HOST: string =
  typeof __VIGOV_API_HOST__ === "string" ? __VIGOV_API_HOST__.replace(/\/+$/, "") : "";

/**
 * Địa chỉ đầy đủ của một tuyến, hoặc chuỗi RỖNG khi chưa khai host.
 *
 * Chuỗi rỗng là tín hiệu "fail closed" mà mọi tầng gọi mạng phải đọc và dừng lại — không phải
 * một giá trị để nối thêm vào. Trả `null` thì mỗi nơi gọi phải nghĩ ra một nhánh riêng; trả
 * chuỗi rỗng thì nhánh ấy là một dòng `if` giống nhau ở mọi nơi.
 */
export function diaChiApi(duong_dan: string): string {
  return API_HOST === "" ? "" : `${API_HOST}${duong_dan}`;
}
