/**
 * Câu chữ của phần "Lịch làm việc của xã" trên màn Cấu hình (`docs/ui-ux/14-cau-hinh.md §8`, khối
 * bảng phụ của tab Thời hạn xử lý). Hàm thuần: không gọi mạng, không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO Ở ĐÂY, VÀ KHÔNG ĐƯỢC THÊM VÀO.
 *
 * Ba bảng này là NỀN của cách đếm hạn xử lý theo giờ làm việc (ADR 0007, luật 10 bất biến 4), và
 * `identity` sở hữu cả ba cùng với phép cộng (`identity.AdvanceWorkingHours`). Một hàm "tính xem
 * tuần này xã làm bao nhiêu giờ" viết ở đây trông vô hại và là bản thứ hai của một phép tính có
 * hệ quả pháp lý: nó sẽ lệch bản của máy chủ vào đúng ngày lễ và ngày làm bù, và con số lệch ấy
 * là con số cán bộ đọc trên màn hình rồi báo cáo lên trên.
 *
 * Tệp này chỉ ĐẶT TÊN cho những gì máy chủ đã trả về.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/**
 * Tên thứ theo **ISO 8601**: 1 = thứ Hai … 7 = Chủ nhật.
 *
 * ISO CHỨ KHÔNG PHẢI SỐ THỨ TIẾNG VIỆT, và cũng không phải `Date.getDay()` của JavaScript (0 là
 * Chủ nhật). Hợp đồng nói ra điều đó bằng chính chú thích trên trường: quy đổi nhầm giữa hai hệ
 * là đúng cái lệch một ngày mà migration 0006 cảnh báo — và một ca làm việc lệch một ngày là mọi
 * hạn xử lý đi qua ngày ấy bị đếm sai.
 */
export function tenThu(thu: number): string {
  switch (thu) {
    case 1:
      return "Thứ Hai";
    case 2:
      return "Thứ Ba";
    case 3:
      return "Thứ Tư";
    case 4:
      return "Thứ Năm";
    case 5:
      return "Thứ Sáu";
    case 6:
      return "Thứ Bảy";
    case 7:
      return "Chủ nhật";
    default:
      // Không thể xảy ra khi ràng buộc CHECK của CSDL còn đúng (thu BETWEEN 1 AND 7). Nói ra
      // vẫn hơn in một con số trần đọc như một thứ có thật.
      return `Thứ không hợp lệ (${thu})`;
  }
}

/**
 * Giờ `HH:MM:SS` của hợp đồng → `07:30`.
 *
 * GIỜ TƯỜNG, KHÔNG PHẢI MỘT MỐC THỜI GIAN: schema cố ý không giữ ngày và không giữ múi giờ.
 * "07:30" là một chỉ dẫn cho cán bộ của xã; mốc thật chỉ sinh ra MỘT lần, trong hàm tính hạn của
 * `identity`, khi ghép nó với một ngày theo giờ Asia/Ho_Chi_Minh. Nên ở đây chỉ cắt bớt phần
 * giây để đọc cho gọn — không dựng `Date`, vì dựng `Date` là tự gán cho nó một ngày và một múi
 * giờ mà máy chủ chưa bao giờ hứa.
 */
export function nhanGio(gio: string): string {
  const phan = gio.split(":");
  const [gioSo, phutSo] = phan;
  if (phan.length < 2 || gioSo === undefined || phutSo === undefined) return gio;
  return `${gioSo}:${phutSo}`;
}

/** Một CA đọc thành `07:30 – 11:30`. Dấu gạch ngang dài, không phải dấu trừ. */
export function nhanCa(batDau: string, ketThuc: string): string {
  return `${nhanGio(batDau)} – ${nhanGio(ketThuc)}`;
}

/**
 * Ngày `YYYY-MM-DD` → `02/09/2026`.
 *
 * CẮT CHUỖI, KHÔNG DỰNG `Date`: trường này là một NGÀY, không mang giờ và không mang múi giờ.
 * Cho nó qua `new Date("2026-09-02")` là gán nửa đêm UTC, và ở mọi máy phía tây London nó hiện
 * thành ngày 01 — một ngày nghỉ lễ lùi một ngày là một hạn xử lý đếm xuyên qua ngày trụ sở đóng
 * cửa (`service-identity/internal/http/ngay_nghi_le.go`, chú thích trên `Date`).
 */
export function nhanNgay(ngayISO: string): string {
  const phan = ngayISO.split("-");
  const [nam, thang, ngay] = phan;
  if (phan.length !== 3 || nam === undefined || thang === undefined || ngay === undefined) {
    return ngayISO;
  }
  return `${ngay}/${thang}/${nam}`;
}

/**
 * Câu dẫn của cả phần — nói rõ đây là cấu hình CỦA XÃ NÀY, không phải hằng số của hệ thống.
 *
 * Câu ấy không thừa: hai xã cạnh nhau có ba bảng khác nhau, và một cán bộ đọc màn hình này rồi
 * tưởng "hệ thống quy định giờ làm như vậy" sẽ không bao giờ nghĩ tới việc phải sửa nó.
 */
export const DAN_LICH_LAM_VIEC =
  "Ba bảng dưới đây là cấu hình riêng của đơn vị này, không phải quy định chung của hệ thống. " +
  "Chúng quyết định cách đếm thời hạn xử lý theo giờ làm việc: hạn không chạy ngoài giờ làm, " +
  "không chạy ngày nghỉ lễ, và có chạy trong ngày làm bù.";

/**
 * Lịch tuần TRỐNG — trạng thái nguy hiểm nhất của ba bảng, và hôm nay là trạng thái của mọi xã.
 *
 * Máy chủ đã gửi kèm một `problems` mang đúng câu này, nên màn hình hiện CÂU CỦA MÁY CHỦ chứ
 * không câu ở đây (xem `tab-lich-lam-viec.tsx`). Hằng này chỉ dùng cho trạng thái rỗng của bảng,
 * chỗ không có `problems` nào để mượn — ví dụ một năm chưa khai ngày nghỉ lễ nào.
 */
export const LICH_TUAN_RONG =
  "Đơn vị chưa cấu hình ca làm việc nào. Chưa tính được thời hạn xử lý nào cho tới khi có ít " +
  "nhất một ca.";

export function nhanNgayNghiRong(nam: number): string {
  return `Năm ${nam} chưa khai ngày nghỉ lễ nào. Đây là trạng thái bình thường của một đơn vị chưa nhập lịch.`;
}

export function nhanNgayLamBuRong(nam: number): string {
  return (
    `Năm ${nam} không có ngày làm bù nào. Phần lớn các năm là như vậy — khác hẳn lịch tuần ` +
    "trống, vốn nghĩa là đơn vị không có giờ làm việc nào."
  );
}
