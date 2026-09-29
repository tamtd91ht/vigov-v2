/**
 * Mốc thời gian máy chủ trả về → `dd/MM/yyyy HH:mm` THEO GIỜ VIỆT NAM (+07), GHIM.
 *
 * ⚠ KHÔNG THEO MÚI GIỜ CỦA MÁY. Mục sổ `citizen-app/mui-gio-theo-may-nguoi-dung` đã ghi: máy người
 * dùng đặt múi khác (người đi làm xa, máy mua ở nước ngoài) sẽ hiện hạn lệch vài giờ — và hạn ở
 * nửa nhà nước là CAM KẾT của một cơ quan nhà nước, đếm bằng giờ làm việc (luật 10). Lệch vài giờ
 * là lệch cả buổi, và qua nửa đêm thì lệch cả NGÀY.
 *
 * ⚠ MỘT HÀM CHO CẢ HAI NỬA — quyết định 27/09/2026: Mini App hiện giờ ghim +07 như web-admin, ở
 * CẢ nửa doanh nghiệp (giờ hết hạn phiên, ngày gửi yêu cầu tư vấn) lẫn nửa công dân. Trước đó nửa
 * doanh nghiệp dùng `toLocaleString("vi-VN")` theo múi của máy, nên cùng một mốc đọc ra hai giờ
 * khác nhau tuỳ đang đứng ở màn nào. Hàm nằm ở `lib/` vì đây là vùng trung lập của
 * `two-halves-boundary.test.ts` — hai nửa cùng nhập được mà không nửa nào nhập nửa kia.
 *
 * TỰ CỘNG +7 GIỜ RỒI ĐỌC THEO UTC, không dùng `Intl` với `timeZone`: Việt Nam không có giờ mùa hè
 * nên +07 là chính xác quanh năm, và cách này không phụ thuộc bộ dữ liệu múi giờ của trình duyệt
 * nhúng trong Zalo — thứ không ai trong kho này đã kiểm trên máy thật.
 */
const VN_OFFSET_MS = 7 * 60 * 60 * 1000;

const two = (n: number) => String(n).padStart(2, "0");

/** `null` khi chuỗi không đọc được — màn hình có câu riêng cho ca ấy, không hiện "Invalid Date". */
export function vnDateTime(iso: string): string | null {
  const ms = new Date(iso).getTime();
  if (Number.isNaN(ms)) return null;
  const t = new Date(ms + VN_OFFSET_MS);
  return `${two(t.getUTCDate())}/${two(t.getUTCMonth() + 1)}/${t.getUTCFullYear()} ${two(
    t.getUTCHours(),
  )}:${two(t.getUTCMinutes())}`;
}

export const UNREADABLE_TIME = "Chưa rõ thời điểm";

/**
 * Một NGÀY máy chủ gửi dạng `YYYY-MM-DD` (ví dụ `published_on` của tin xã) → `dd/MM/yyyy`.
 *
 * ⚠ KHÔNG ĐI QUA `vnDateTime`. Một ngày không có giờ nên không có múi: `new Date("2026-09-27")` đọc
 * nó là nửa đêm UTC, và cộng +07 sẽ in ra "27/09/2026 07:00" — một giờ không ai đăng tin lúc ấy.
 * Chỉ đổi chỗ ba phần; ngày không có thật (`2026-02-30`) là `null`, không phải một ngày khác.
 */
export function vnDate(date: string): string | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (match === null) return null;
  const [, year, month, day] = match as unknown as [string, string, string, string];
  const d = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  if (d.getUTCFullYear() !== Number(year) || d.getUTCMonth() !== Number(month) - 1 || d.getUTCDate() !== Number(day)) {
    return null;
  }
  return `${day}/${month}/${year}`;
}

export const UNREADABLE_DATE = "Chưa rõ ngày";
