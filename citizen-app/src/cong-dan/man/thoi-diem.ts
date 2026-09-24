/**
 * Mốc thời gian của hợp đồng → `dd/MM/yyyy HH:mm` THEO GIỜ VIỆT NAM (+07), GHIM.
 *
 * ⚠ KHÔNG THEO MÚI GIỜ CỦA MÁY. Mục sổ `citizen-app/mui-gio-theo-may-nguoi-dung` đã ghi: máy người
 * dùng đặt múi khác (người đi làm xa, máy mua ở nước ngoài) sẽ hiện hạn lệch vài giờ — và hạn ở đây
 * là CAM KẾT của một cơ quan nhà nước, đếm bằng giờ làm việc (luật 10). Lệch vài giờ là lệch cả buổi.
 *
 * TỰ CỘNG +7 GIỜ RỒI ĐỌC THEO UTC, không dùng `Intl` với `timeZone`: Việt Nam không có giờ mùa hè
 * nên +07 là chính xác quanh năm, và cách này không phụ thuộc bộ dữ liệu múi giờ của trình duyệt
 * nhúng trong Zalo — thứ không ai trong kho này đã kiểm trên máy thật.
 */
const LECH_VN_MS = 7 * 60 * 60 * 1000;

const hai = (n: number) => String(n).padStart(2, "0");

/** `null` khi chuỗi không đọc được — màn hình có câu riêng cho ca ấy, không hiện "Invalid Date". */
export function thoiDiemVN(iso: string): string | null {
  const ms = new Date(iso).getTime();
  if (Number.isNaN(ms)) return null;
  const t = new Date(ms + LECH_VN_MS);
  return `${hai(t.getUTCDate())}/${hai(t.getUTCMonth() + 1)}/${t.getUTCFullYear()} ${hai(
    t.getUTCHours(),
  )}:${hai(t.getUTCMinutes())}`;
}

export const THOI_DIEM_KHONG_DOC_DUOC = "Chưa rõ thời điểm";
