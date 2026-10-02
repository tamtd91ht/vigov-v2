/**
 * Câu chữ chung của màn Cấu hình hệ thống — hiện chỉ có một khối: những phần của đặc tả
 * (`docs/ui-ux/14-cau-hinh.md`) CHƯA DỰNG, kèm lý do thật.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MỖI MỤC RA TỚI MÀN HÌNH Ở ĐÚNG CHỖ ĐẶC TẢ ĐẶT NÓ (ADR 0068 §14): phần ấy được vẽ thành đúng loại
 * control nó sẽ là, bị vô hiệu, mang dấu "?"; bấm "?" thì đọc `ten` + `viSao` của mục ở đây. Khối gập
 * "N phần chưa dựng" cuối trang đã bỏ. Mục "Gửi báo cáo định kỳ" là thẻ thứ năm của tab Tự động hoá
 * (`automation-tab.tsx`).
 *
 * VIỆC ĐÃ QUYẾT KHÔNG LÀM THÌ KHÔNG CÓ MỤC Ở ĐÂY: "Tính lại số liệu Tổng quan" bị bỏ hẳn — màn Tổng
 * quan đếm trực tiếp, không lưu số liệu dựng sẵn (ADR 0053, ADR 0058). Một mục cho nó là báo cáo tiến
 * độ đếm như "chưa dựng" một việc cơ quan đã từ chối.
 *
 * `tools/tien_do_san_pham.py` đếm các dòng `ten: "` NẰM TRONG khối khai báo `PHAN_CHUA_DUNG` (tới
 * dấu `];` đầu dòng) — giữ nguyên tên hằng và hình khối.
 *
 * MỖI MỤC PHẢI ĐÚNG VÀO NGÀY NÓ CÒN Ở ĐÂY. Lượt nào dựng xong một phần thì xoá mục của phần ấy
 * trong cùng lượt — một mục nói "chưa có" về thứ đã có là câu sai nói với cán bộ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Gửi báo cáo định kỳ",
    viSao:
      "Tự gửi báo cáo tuần vào đầu tuần và báo cáo tháng vào ngày mùng 1. Việc này chờ màn Báo cáo: " +
      "chưa có báo cáo thì chưa có gì để gửi.",
  },
];
