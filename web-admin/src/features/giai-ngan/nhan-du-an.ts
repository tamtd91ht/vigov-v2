/**
 * Câu chữ và cách định dạng của màn "Theo dõi giải ngân" (`docs/ui-ux/06-giai-ngan.md §7, §8`).
 * Hàm thuần: không gọi mạng, không dựng DOM, không đọc đồng hồ.
 *
 * KHÔNG MỘT PHÉP TÍNH NGHIỆP VỤ NÀO Ở ĐÂY. Tỷ lệ giải ngân, điểm chậm và cờ "đang chậm" đều do
 * máy chủ suy ra trên từng lần đọc, từ đồng hồ và từ ngưỡng của xã, và chúng về sẵn trong phản
 * hồi (`service-finance/internal/http/du_an.go`). Tính lại ở đây là dựng câu trả lời thứ hai cho
 * cùng một câu hỏi — và bản chạy trên trình duyệt sẽ lệch bản của máy chủ vào đúng ngày đổi năm
 * ngân sách, trong khi không bài test nào đỏ.
 */

import type { finance_hangMucRa } from "@/lib/api/schema.gen";

/**
 * Định dạng số theo `vi-VN`, GHIM chứ không theo cài đặt của máy: dấu phân cách hàng nghìn khác
 * nhau giữa các miền địa phương làm `100.000.000` đọc thành một trăm triệu ở máy này và thành
 * một trăm phẩy không ở máy khác.
 */
const DINH_DANG_SO = new Intl.NumberFormat("vi-VN");

/** Hai chữ số thập phân, đúng đơn vị đặc tả in ra: `10,33%` · `chậm 31,36 điểm`. */
const DINH_DANG_PHAN_VAN = new Intl.NumberFormat("vi-VN", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/**
 * Số tiền đồng → `100.000.000 đ`.
 *
 * SỐ ÂM HIỆN NGUYÊN LÀ SỐ ÂM. `remaining_amount` được phép âm — đặc tả §13 quy tắc 2 nói tỷ lệ
 * trên 100% thì HIỆN chứ không chặn — và kẹp nó về 0 là giấu một lần giải ngân vượt kế hoạch,
 * đúng con số có người cần nhìn thấy.
 */
export function nhanTien(dong: number): string {
  // Ca "có giá trị nhưng không phải số hữu hạn" là hợp đồng hỏng, không phải một trạng thái
  // nghiệp vụ, nên nó nói ra bằng một câu KHÁC thay vì hiện chữ "NaN đ".
  if (!Number.isFinite(dong)) return "Không đọc được";
  return `${DINH_DANG_SO.format(dong)} đ`;
}

/**
 * Tỷ lệ giải ngân — `disbursed_ratio`, đơn vị **phần vạn của phần trăm** (1033 ⇒ `10,33%`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * `null` KHÔNG PHẢI `0%`, VÀ KHÔNG ĐƯỢC HIỆN THÀNH `0%`. Máy chủ nói thẳng vì sao: `null` nghĩa
 * là dự án CHƯA ĐƯỢC BỐ TRÍ VỐN, nên không có mẫu số để chia; `0%` là một khẳng định rằng đã bố
 * trí vốn mà chưa chi đồng nào. Hiện `0%` cho một dự án chưa ai bố trí vốn là báo cáo nó như dự
 * án tệ nhất của xã (`du_an.go`, chú thích trên `DisbursedRatio`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function nhanTyLeGiaiNgan(phanVan: number | null): string {
  if (phanVan === null) return "Chưa bố trí vốn";
  if (!Number.isFinite(phanVan)) return "Không đọc được";
  return `${DINH_DANG_PHAN_VAN.format(phanVan / 100)}%`;
}

/**
 * Chip tiến độ dưới mã dự án — BA ca, và không ca nào là một ô trống.
 *
 * `is_delayed` và `delay_score` là hai trường KHÁC NHAU và cả hai đều đến từ máy chủ: cờ là kết
 * quả so điểm chậm với ngưỡng của xã, còn điểm là con số để hiện. `delay_score` `null` (chưa bố
 * trí vốn) thì không có con số nào để in, nên chip nói đúng điều đó thay vì in `chậm 0 điểm`.
 */
export type TienDoDuAn =
  | { loai: "chuaBoTriVon" }
  | { loai: "bamSat" }
  | { loai: "cham"; diem: number };

export function tienDoDuAn(diemCham: number | null, dangCham: boolean): TienDoDuAn {
  // Xét `null` TRƯỚC, và thứ tự ấy không làm mất một dự án chậm nào: máy chủ trả `is_delayed`
  // bằng `false` bất cứ khi nào không có điểm chậm, vì "dự án chưa bố trí vốn thì không chậm —
  // đó là trạng thái kế toán, không phải sự chậm trễ" (`domain.LaCham`).
  if (diemCham === null) return { loai: "chuaBoTriVon" };
  if (!dangCham) return { loai: "bamSat" };
  return { loai: "cham", diem: diemCham };
}

export function nhanTienDo(t: TienDoDuAn): string {
  switch (t.loai) {
    case "chuaBoTriVon":
      return "Chưa bố trí vốn";
    case "bamSat":
      // Đúng chữ đặc tả §8 in trong chip xanh.
      return "Bám sát tiến độ";
    case "cham":
      return `Chậm ${DINH_DANG_PHAN_VAN.format(t.diem / 100)} điểm`;
  }
}

/** Lớp CSS đi theo LOẠI kết quả, không theo câu chữ. Màu không phải tín hiệu duy nhất: chữ đã nói rõ. */
export function lopTienDo(t: TienDoDuAn): string {
  switch (t.loai) {
    case "chuaBoTriVon":
      return "chip chip-ngung";
    case "bamSat":
      return "chip chip-hoat-dong";
    case "cham":
      return "chip chip-cham";
  }
}

/**
 * Ngày dạng `YYYY-MM-DD` của hợp đồng → `31/12/2026`.
 *
 * CẮT CHUỖI, KHÔNG DỰNG `Date`. Trường này là một NGÀY, không mang giờ và không mang múi giờ;
 * cho nó qua `new Date("2026-12-31")` là gán cho nó nửa đêm UTC, và ở mọi máy đặt múi giờ phía
 * tây London nó hiện thành ngày 30 — một thời hạn giải ngân lùi một ngày
 * (`service-identity/internal/http/ngay_nghi_le.go`, chú thích trên `Date`).
 *
 * Chuỗi rỗng nghĩa là xã chưa đặt mốc ấy — máy chủ phát `""` cho một ngày chưa có, không phát
 * một ngày giả.
 */
export function nhanNgay(ngayISO: string): string {
  if (ngayISO === "") return "Chưa đặt";
  const phan = ngayISO.split("-");
  const [nam, thang, ngay] = phan;
  if (phan.length !== 3 || nam === undefined || thang === undefined || ngay === undefined) {
    // Không đúng khuôn hợp đồng: hiện NGUYÊN chuỗi máy chủ gửi, đừng đoán. Một ngày đoán sai
    // trông y hệt một ngày đúng.
    return ngayISO;
  }
  return `${ngay}/${thang}/${nam}`;
}

/**
 * Cột "Hạng mục" — tra `category_id` của dự án trong danh mục hạng mục kế hoạch vốn.
 *
 * BA CA, và gộp ca 1 với ca 3 là báo sai: "dự án chưa gắn hạng mục" là việc của cán bộ nhập
 * liệu, còn "mã hạng mục không còn trong danh mục" là dữ liệu lệch mà chỉ người quản trị sửa
 * được. Ca thứ ba là đường THÔNG THƯỜNG hôm nay, không phải chuyện hiếm: danh mục hạng mục
 * `GET /api/v1/capital-plan-categories` cố ý ship RỖNG cho tới khi có bước khởi tạo xã
 * (`service-finance/internal/http/hang_muc_ke_hoach_von.go`).
 */
export type HangMucDuAn =
  | { loai: "chuaGan" }
  | { loai: "coNhan"; nhan: string }
  | { loai: "chiCoMa"; ma: string };

export function hangMucDuAn(
  categoryId: string,
  danhMuc: readonly finance_hangMucRa[],
): HangMucDuAn {
  if (categoryId === "") return { loai: "chuaGan" };
  const thay = danhMuc.find((h) => h.id === categoryId);
  if (thay === undefined || thay.label === "") return { loai: "chiCoMa", ma: categoryId };
  return { loai: "coNhan", nhan: thay.label };
}

export function nhanHangMuc(h: HangMucDuAn): string {
  switch (h.loai) {
    case "chuaGan":
      return "Chưa gắn hạng mục";
    case "coNhan":
      return h.nhan;
    case "chiCoMa":
      // Hiện MÃ, và nói rõ vì sao chỉ có mã — không để người đọc tưởng chuỗi ULID là tên hạng mục.
      return `${h.ma} (không tra được trong danh mục hạng mục)`;
  }
}

export function lopHangMuc(h: HangMucDuAn): string | undefined {
  switch (h.loai) {
    case "coNhan":
      return undefined;
    case "chiCoMa":
      return "nhan-lech";
    case "chuaGan":
      return "nhan-trong";
  }
}

/**
 * Ngưỡng cảnh báo chậm máy chủ ĐÃ ÁP DỤNG cho danh sách này, không phải một con số web giữ.
 *
 * Máy chủ gửi `delay_threshold` về đúng vì lý do ấy: hôm nay nó là mặc định 10 điểm của §13 quy
 * tắc 5, ngày mai nó theo từng xã và từng năm ngân sách — và một client giữ bản sao của riêng nó
 * sẽ tiếp tục dán nhãn "chậm" theo con số cũ mà không có gì nói ra (`du_an.go`, `DelayThreshold`).
 */
export function nhanNguongCham(phanVan: number): string {
  if (!Number.isFinite(phanVan)) return "Ngưỡng cảnh báo chậm: không đọc được";
  return `Ngưỡng cảnh báo chậm: ${DINH_DANG_PHAN_VAN.format(phanVan / 100)} điểm`;
}

/**
 * Banner BẮT BUỘC hiện ở đầu màn (`06-giai-ngan §1`), nguyên văn.
 *
 * Không phải trang trí và không được rút gọn: nó là câu xã trả lời khi ai đó đối chiếu số trên
 * màn hình này với sổ sách kế toán và thấy lệch.
 */
export const CANH_BAO_KHONG_PHAI_KE_TOAN =
  "ViGov là công cụ theo dõi và điều hành, không phải phần mềm kế toán. Số liệu phục vụ chỉ " +
  "đạo, không thay thế sổ sách kế toán và không đối chiếu với Kho bạc.";

/** Trạng thái rỗng: năm ngân sách chưa có dự án nào. Bình thường, không phải lỗi. */
export function nhanNamRong(nam: number): string {
  return (
    `Năm ngân sách ${nam} chưa có dự án nào. Thêm dự án bằng nút ở trên (cần quyền ` +
    "budget.update)."
  );
}

/**
 * Ghi chú đầu màn: phần nào của bản thiết kế đã có đường ghi thật, phần nào chưa.
 *
 * ⚠ CÂU CŨ Ở ĐÂY — *"Màn hình hiện chỉ xem. Thêm dự án, ghi nhận khoản chi … chưa mở"* — ĐÃ SAI TỪ
 * 24/09/2026, khi chín tuyến ghi của phân hệ được nối vào. Nó được viết lại chứ không gỡ đi, vì một
 * câu đầu màn nói sai về việc cán bộ làm được gì là câu họ tin và làm theo.
 *
 * NHẬP EXCEL VẪN CHƯA CÓ, và đó là sự thật của hợp đồng: không tuyến nào nhận tệp. Chi tiết từng
 * phần chưa dựng nằm ở `PHAN_CHUA_DUNG_GHI` trong `nhan-ghi-giai-ngan.ts`, hiện ra trên màn.
 */
export const GHI_CHU_CHI_XEM_GIAI_NGAN =
  "Thêm dự án, sửa dự án và ghi nhận khoản chi đã mở, mỗi thao tác đứng sau đúng khoá quyền của " +
  "nó. Nhập giải ngân từ Excel chưa có tuyến nào phía sau nên chưa mở.";
