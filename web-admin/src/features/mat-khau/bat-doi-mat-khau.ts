import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

/**
 * Đường BẮT ĐỔI MẬT KHẨU ở lần đăng nhập đầu — phần quyết định, tách khỏi phần điều hướng.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VIỆC CỦA WEB Ở ĐÂY KHÔNG PHẢI GIỮ AN TOÀN. MÁY CHỦ ĐÃ GIỮ RỒI, và giữ theo cách không ai vô
 * hiệu được từ trình duyệt: `service-identity/internal/http/middleware.go` chặn NGAY TRONG
 * `XacThuc`, bằng một DANH SÁCH CHO PHÉP ba tuyến và từ chối phần còn lại — nên một tuyến viết
 * sau, mà không ai nghĩ tới chuyện này, mặc định bị CHẶN chứ không mặc định mở. Câu trả lời là
 * 403 `password_change_required`.
 *
 * VIỆC CỦA WEB LÀ KHÔNG ĐỂ NGƯỜI DÙNG ĐÂM VÀO BỨC TƯỜNG ẤY MÀ KHÔNG HIỂU VÌ SAO: cờ bật thì đưa
 * họ tới đúng màn hình gỡ được cờ, thay vì để họ bấm quanh và nhận 403 ở mọi nơi.
 *
 * KHÔNG CÓ BẢN SAO THỨ HAI CỦA DANH SÁCH BA TUYẾN Ở ĐÂY, và đó là điều tệp này tồn tại để giữ.
 * Máy chủ so danh sách ấy với chính các hằng `mux.Handle` nó đăng ký, nên nó không lệch được khỏi
 * mux. Một bản thứ hai ở client thì lệch được, và ngày nó lệch là ngày web chặn hoặc cho qua khác
 * máy chủ — theo chiều nào cũng sai, và theo chiều "cho qua" thì im lặng. Ở đây chỉ có MỘT phép
 * so, và nó so với đúng MỘT màn hình của chính ứng dụng này.
 *
 * 403 CHỨ KHÔNG PHẢI 401, VÀ KHÁC BIỆT ẤY LÀ LÝ DO HÀM NÀY KHÔNG ĐỤNG TỚI PHIÊN: phiên VẪN hợp
 * lệ, chỉ có thao tác bị từ chối. Xoá phiên rồi đẩy về `/dang-nhap` là dựng một vòng lặp không
 * lối ra — người dùng đăng nhập lại đúng, và quay lại y nguyên trạng thái này.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Màn hình duy nhất gỡ được cờ. Một hằng, một chỗ khai. */
export const DUONG_DAN_DOI_MAT_KHAU = "/doi-mat-khau";

/**
 * Bỏ dấu `/` đuôi trước khi so.
 *
 * `/doi-mat-khau/` và `/doi-mat-khau` là CÙNG một trang với Next (`trailingSlash` mặc định là
 * false), nhưng là hai chuỗi khác nhau với `===`. Không chuẩn hoá thì một người tới bằng đường có
 * dấu gạch đuôi sẽ bị đẩy về chính trang họ đang đứng, lặp lại mỗi lần trang dựng xong — đúng
 * hình dạng vòng lặp mà cả tệp này viết ra để tránh.
 */
function chuanHoaDuongDan(duongDan: string): string {
  return duongDan.length > 1 && duongDan.endsWith("/") ? duongDan.slice(0, -1) : duongDan;
}

/**
 * Nơi phải đưa người dùng tới, hoặc `null` nếu không phải đưa đi đâu cả.
 *
 * BỐN CA TRẢ `null`, và ba trong bốn là ca KHÔNG ĐƯỢC PHÉP ĐOÁN:
 *
 *   - `phien === null` — CHƯA ĐỌC XONG, khác hẳn "đọc xong và cờ tắt". Đẩy đi lúc này là đẩy đi
 *     dựa trên một điều chưa biết, và nó sẽ cướp trang ngay dưới tay người đang đọc.
 *   - `!phien.ok` — không đọc được phiên. Không suy ra gì cả: 401 thật sự do chính lời gọi ấy
 *     báo, và cổng quyền đã hiện câu của máy chủ (`features/quyen/cong-quyen.tsx`).
 *   - cờ tắt — trạng thái bình thường của mọi tài khoản đã đổi mật khẩu.
 *   - đã đứng sẵn ở màn đổi mật khẩu — không tự đẩy mình đi.
 */
export function duongDanBatDoiMatKhau(
  phien: KetQua<identity_phienHienTaiRa> | null,
  duongDanHienTai: string,
): string | null {
  if (phien === null) return null;
  if (!phien.ok) return null;
  if (!phien.duLieu.must_change_password) return null;
  if (chuanHoaDuongDan(duongDanHienTai) === DUONG_DAN_DOI_MAT_KHAU) return null;
  return DUONG_DAN_DOI_MAT_KHAU;
}

/**
 * Cờ bắt đổi có đang bật không — dùng để hiện lời nhắc TRÊN CHÍNH màn đổi mật khẩu.
 *
 * Tách khỏi hàm trên vì hai câu hỏi khác nhau: hàm trên hỏi "có phải đi đâu không" và trả `null`
 * khi đã đứng đúng chỗ; hàm này hỏi "vì sao người này đang ở đây", và câu trả lời phải còn đúng
 * ĐÚNG LÚC đang đứng ở đó.
 */
export function dangBiBatDoi(phien: KetQua<identity_phienHienTaiRa> | null): boolean {
  return phien !== null && phien.ok && phien.duLieu.must_change_password;
}
