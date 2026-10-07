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

/**
 * A percent or a point figure as the owner's spec prints it (spec 00 §6: `n.toLocaleString("vi-VN")`):
 * `50%`, `32,59%`, `chậm 31,36 điểm` — no forced decimals. The server's unit is hundredths, so at most
 * two decimals ever appear; a trailing `,00` is not printed (ADR 0068 lần 6, reviewer row G8).
 */
const DINH_DANG_PHAN_VAN = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 });

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
 * A list amount, short as the prototype prints it (`budget-display.ts:28-54`, `formatDongShort`):
 * `7,5 tỷ` · `100 triệu`. User decision 07/10/2026, for the list's `KH vốn năm` / `Đã giải ngân` only —
 * the full đồng stays beside it (hover, screen reader) and on the project page.
 *
 * ONE DECIMAL, ROUNDED DOWN, as the prototype does: rounding up would print a project as having
 * spent money it has not. Computed in `BigInt`, so the digit dropped is the real one. Under a million
 * the amount is written in full (`nhanTien`). NOT `lib/compact-dong`: that form is "9,64 tỷ đồng",
 * two decimals, rounded — a second format for the same column would read differently from the
 * prototype the user chose.
 */
export function shortDongLabel(dong: number): string {
  if (!Number.isSafeInteger(dong)) return "Không đọc được";
  const amount = BigInt(dong);
  const sign = amount < 0n ? "-" : "";
  const abs = amount < 0n ? -amount : amount;
  const tenths = (value: bigint, unit: bigint): string => {
    const whole = value / unit;
    const tenth = ((value % unit) * 10n) / unit;
    return tenth === 0n ? DINH_DANG_SO.format(whole) : `${DINH_DANG_SO.format(whole)},${tenth}`;
  };
  if (abs >= 1_000_000_000n) return `${sign}${tenths(abs, 1_000_000_000n)} tỷ`;
  if (abs >= 1_000_000n) return `${sign}${tenths(abs, 1_000_000n)} triệu`;
  return nhanTien(dong);
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
 * A hundredths-of-a-percent figure that has no "no denominator" case — the share of the budget year
 * gone (`time_elapsed_ratio`): 7096 ⇒ `70,96%`. Same format as `nhanTyLeGiaiNgan`, without its `null`
 * sentence, which would be false here.
 */
export function percentLabel(hundredths: number): string {
  if (!Number.isFinite(hundredths)) return "Không đọc được";
  return `${DINH_DANG_PHAN_VAN.format(hundredths / 100)}%`;
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
      // The project page's badge, spec 07 §Body 1 verbatim. The list writes its own lowercase
      // "chậm x điểm" (`delayPointsLabel`), as spec 02 §8 does.
      return `Chậm ${delayPointsLabel(t.diem)} so với tiến độ thời gian`;
  }
}

/** `31,36 điểm` from the server's `delay_score` (hundredths of a point). */
export function delayPointsLabel(hundredths: number): string {
  return `${DINH_DANG_PHAN_VAN.format(hundredths / 100)} điểm`;
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
  if (
    phan.length !== 3 ||
    nam === undefined ||
    thang === undefined ||
    ngay === undefined ||
    !/^\d+$/.test(thang) ||
    !/^\d+$/.test(ngay)
  ) {
    // Không đúng khuôn hợp đồng: hiện NGUYÊN chuỗi máy chủ gửi, đừng đoán. Một ngày đoán sai
    // trông y hệt một ngày đúng.
    return ngayISO;
  }
  // Unpadded, as the spec's `toLocaleDateString("vi-VN")` prints it: `27/8/2026` (spec 00 §6, row G9).
  return `${Number(ngay)}/${Number(thang)}/${nam}`;
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
      // The prototype's words for the projects with no category (`BudgetItemTable.tsx:132`).
      return "Chưa xếp hạng mục";
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

// Banner BẮT BUỘC của §1 ("không phải phần mềm kế toán") KHÔNG còn là hằng ở đây từ 29/09/2026:
// câu ấy là `budget.scope_notice` do máy chủ gửi (`scope_notice`), xã sửa được — xem `scope-notice.tsx`.

/**
 * Trạng thái rỗng: năm ngân sách chưa có dự án nào. Bình thường, không phải lỗi. The prototype's
 * sentence verbatim (spec 02 §8 "Rỗng", `BudgetWorkspace.tsx:371-374`). The extra line naming the
 * permission left with ADR 0068 lần 6: the spec card has none.
 */
export function nhanNamRong(nam: number): string {
  return `Thêm dự án và xếp vào hạng mục để bắt đầu theo dõi giải ngân năm ${nam}.`;
}

/**
 * What an account without `budget.read` is told on both Giải ngân pages, by the permission's Phân
 * quyền name (GN-07). Here, in a plain module, because the detail page is a Server Component.
 */
export const DISBURSEMENT_READ_DENIED =
  "Tài khoản của bạn chưa được cấp quyền “Xem giải ngân”, nên phần này không hiển thị. Liên hệ " +
  "quản trị viên của đơn vị nếu bạn cần quyền này.";

// The header note `GHI_CHU_CHI_XEM_GIAI_NGAN` ("… nhập giải ngân từ Excel chưa có") left the screen
// on 06/10/2026 (ADR 0068 lần 5): the prototype has no such line, and the Excel import is now live
// (`disbursement-import-dialog.tsx`).
