/**
 * Câu chữ chung của màn Cấu hình hệ thống — hiện chỉ có một khối: những phần của đặc tả
 * (`docs/ui-ux/14-cau-hinh.md`) CHƯA DỰNG, kèm lý do thật.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO KHỐI NÀY RA TỚI MÀN HÌNH chứ không nằm trong chú thích: một tab vắng mặt mà không có câu
 * nào giải thích thì cán bộ đọc thành "phần mềm không có chức năng này", và lần sau vẫn có người
 * hỏi lại. Cùng khuôn `PHAN_CHUA_DUNG` của các màn Biên bản họp, Thu - Chi, Phản ánh.
 *
 * VÌ SAO ĐÂY LÀ KHỐI CÓ `ten:` CUỐI CÙNG CỦA TỆP: `tools/tien_do_san_pham.py` đếm mọi dòng
 * `ten: "` từ lần xuất hiện đầu tiên của tên `PHAN_CHUA_DUNG` tới HẾT TỆP. Thêm một mảng khác có
 * trường `ten` phía dưới là báo cáo tiến độ đếm lố mà không ai thấy.
 *
 * MỖI MỤC PHẢI ĐÚNG VÀO NGÀY NÓ CÒN Ở ĐÂY. Lượt nào dựng xong một phần thì xoá mục của phần ấy
 * trong cùng lượt — một mục nói "chưa có" về thứ đã có là câu sai nói với cán bộ, và ca kiểm
 * (`khoi-chua-dung.test.tsx`) chỉ kiểm mục có HIỆN RA, không kiểm mục còn ĐÚNG.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Tự động hoá — Tính lại số liệu Tổng quan và Gửi báo cáo định kỳ (§9)",
    viSao:
      "Tab Tự động hoá có ba việc: nhắc việc sắp đến hạn và quá hạn, leo thang việc trễ hạn, bản tin " +
      "đầu tuần. Việc Tính lại số liệu Tổng quan được bỏ hẳn: màn Tổng quan đếm trực tiếp, không lưu " +
      "số liệu dựng sẵn, nên không có gì để tính lại (ADR 0053, ADR 0058). Việc Gửi báo cáo định kỳ " +
      "chờ màn Báo cáo: chưa có báo cáo thì chưa có gì để gửi (ADR 0058 mục 4).",
  },
  {
    // CẬP NHẬT CÂU NÀY TRONG CÙNG LƯỢT với mỗi tuyến nhập mới được nối (`excel-import-targets.tsx`):
    // nó kể tên từng thứ nhập được hôm nay, và `khoi-chua-dung.test.tsx` đối chiếu với
    // `CATALOGUE_IMPORTS` và các đích nhập riêng (Sơ đồ tổ chức, Thôn / Tổ dân phố).
    ten: "Nút Nhập từ Excel ở tab Người dùng và các nhóm còn lại của Danh mục (§3, §5)",
    viSao:
      "Trên màn này hôm nay nhập được từ Excel ba thứ: Sơ đồ tổ chức, Thôn / Tổ dân phố, và nhóm Loại " +
      "tài nguyên bản đồ ở tab Danh mục. Nhập danh sách cán bộ và các nhóm danh mục khác đã được quyết " +
      "(ADR 0059) nhưng dịch vụ sở hữu từng loại chưa mở tuyến nhập, nên chưa có nút.",
  },
];
