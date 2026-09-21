/**
 * Ba tuyến đọc lịch làm việc **của một xã** — `GET /api/v1/working-hours`,
 * `GET /api/v1/public-holidays?year=`, `GET /api/v1/swap-working-days?year=`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA BẢNG NÀY LÀ CẤU HÌNH CỦA TỪNG XÃ, KHÔNG PHẢI HẰNG SỐ CỦA HỆ THỐNG. Chúng là nền của cách
 * đếm hạn xử lý theo GIỜ LÀM VIỆC (ADR 0007, luật 10 bất biến 4): giờ làm của xã, ngày xã đóng
 * cửa, và ngày xã vẫn làm dù lịch tuần nói không. Hai xã cạnh nhau có ba bảng khác nhau.
 *
 * VÀ KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO ĐƯỢC VIẾT Ở PHÍA WEB. `identity` sở hữu cả ba bảng và
 * sở hữu luôn phép cộng (`identity.AdvanceWorkingHours`). Một bản tính thứ hai ở trình duyệt sẽ
 * cho ra một con số khác bản của máy chủ vào đúng ngày lễ, và con số hiện trên màn hình cán bộ
 * là con số được báo cáo lên trên.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY. Không có hàm ghi nào: máy chủ cũng không có tuyến ghi —
 * ai được sửa lịch làm việc của một xã chưa có ai hỏi, và lịch làm việc là CĂN CỨ CỦA MỘT CAM
 * KẾT ĐÃ PHÁT RA (`service-identity/internal/http/lich_lam_viec.go`, khối đầu tệp).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HỢP ĐỒNG CÒN THIẾU THAM SỐ `year` CỦA HAI TUYẾN THEO NĂM — cùng lỗ hổng đã ghi ở `du-an.ts`.
 * `openapi.json` không khai `parameters` cho `/api/v1/public-holidays` và
 * `/api/v1/swap-working-days`, trong khi `docNamTruyVan` bắt buộc đúng MỘT giá trị `year` và trả
 * 400 nếu thiếu, sai định dạng, ngoài khoảng, hoặc gửi hai lần
 * (`service-identity/internal/http/ngay_nghi_le.go:54`). Đã báo lên; sửa ở `tools/apidoc`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, type KetQua } from "./goi";
import type {
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
  identity_get_public_holidays,
  identity_get_swap_working_days,
  identity_get_working_hours,
} from "./schema.gen";

/**
 * GET /api/v1/working-hours — lịch tuần thông thường của xã. KHÔNG có tham số năm: lịch tuần
 * không gắn với một năm nào.
 *
 * Phản hồi mang theo `problems`: những sai sót máy chủ SUY RA TỪ CHÍNH những dòng nó vừa trả về
 * (lịch trống, hai ca chồng giờ). Đó là dữ liệu của xã đang hỏng, không phải lời gọi hỏng — nên
 * tuyến vẫn trả 200, và màn hình phải hiện chúng ra.
 */
export function layLichLamViec(): Promise<KetQua<identity_danhSachCaLamViecRa>> {
  const duongDan: identity_get_working_hours["duongDan"] = "/api/v1/working-hours";
  return docJSON<identity_danhSachCaLamViecRa>(duongDan);
}

/** Dựng `?year=` cho hai tuyến theo năm. Một chỗ duy nhất, để hai màn không hiểu "năm" khác nhau. */
function themNam(duongDan: string, nam: number): string {
  // Đúng MỘT giá trị `year`: máy chủ đọc nguyên mảng và từ chối khi có hai, vì `?year=2026&year=2027`
  // sẽ lặng lẽ đọc một trong hai năm người gọi hỏi (`ngay_nghi_le.go:66`). `URLSearchParams.set`
  // giữ đúng một giá trị.
  const truyVan = new URLSearchParams();
  truyVan.set("year", String(nam));
  return `${duongDan}?${truyVan.toString()}`;
}

export function duongDanNgayNghiLe(nam: number): string {
  const duongDan: identity_get_public_holidays["duongDan"] = "/api/v1/public-holidays";
  return themNam(duongDan, nam);
}

export function duongDanNgayLamBu(nam: number): string {
  const duongDan: identity_get_swap_working_days["duongDan"] = "/api/v1/swap-working-days";
  return themNam(duongDan, nam);
}

/**
 * GET /api/v1/public-holidays — ngày xã KHÔNG làm việc trong một năm.
 *
 * 409 là một trạng thái CÓ THẬT của tuyến này: một ngày vừa được khai là nghỉ lễ vừa được khai
 * là làm bù. Máy chủ TỪ CHỐI thay vì chọn bên, và câu thông báo của nó nêu đúng những ngày ấy —
 * nên giao diện hiện nguyên văn câu đó, không rút gọn.
 */
export function layNgayNghiLe(nam: number): Promise<KetQua<identity_danhSachNgayNghiLeRa>> {
  return docJSON<identity_danhSachNgayNghiLeRa>(duongDanNgayNghiLe(nam));
}

/**
 * GET /api/v1/swap-working-days — ngày xã CÓ làm việc dù lịch tuần nói không.
 *
 * Một dòng là một CA, không phải một ngày: ngày làm bù có nghỉ trưa là hai dòng cách nhau. Và
 * một năm không có ngày làm bù nào là chuyện BÌNH THƯỜNG — khác hẳn lịch tuần trống, nghĩa là xã
 * không có giờ làm việc nào (`service-identity/internal/http/ngay_lam_bu.go`).
 */
export function layNgayLamBu(nam: number): Promise<KetQua<identity_danhSachCaLamBuRa>> {
  return docJSON<identity_danhSachCaLamBuRa>(duongDanNgayLamBu(nam));
}
