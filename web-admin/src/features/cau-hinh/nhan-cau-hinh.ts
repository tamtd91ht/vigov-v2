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
    ten: "Lời hệ thống — 32 câu nhóm Báo cáo `report.*` (§7)",
    viSao:
      "Câu của Phản ánh và Thu – Chi đã sửa được ở tab Lời hệ thống. Riêng 32 khoá `report.*` thì " +
      "chưa chốt dịch vụ nào sở hữu (ADR 0024, mục để trống), nên chưa có bảng và chưa có tuyến.",
  },
  {
    ten: "Tab Tự động hoá (§9)",
    viSao:
      "Hệ thống chưa có bộ lập lịch chạy việc nền theo nhịp cho từng đơn vị. Một công tắc bật/tắt " +
      "mà không có gì chạy phía sau là một lời hứa suông.",
  },
  {
    ten: "Nút Nhập từ Excel ở tab Thôn / Tổ dân phố, Người dùng và Danh mục (§2, §3, §5)",
    viSao:
      "Trên màn này hiện chỉ nhập được Sơ đồ tổ chức từ Excel. Nhập danh sách cán bộ phải đi đúng " +
      "các quyết định về mật khẩu đầu tiên, mã cán bộ và số điện thoại (câu hỏi #9, #15, #16), và " +
      "cách ánh xạ các cột của tệp vào những quyết định ấy chưa được thiết kế. Thôn / Tổ dân phố, " +
      "Người dùng và Danh mục đều chưa có tuyến nhập nào trong hợp đồng.",
  },
];
