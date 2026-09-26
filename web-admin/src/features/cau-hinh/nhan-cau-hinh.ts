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
    ten: "Thanh chuyển tab (§0)",
    viSao:
      "Các phần của màn này đang dựng nối tiếp nhau trong trang. Thanh tab đặt ra một câu về giao " +
      "diện chưa ai trả lời: tài khoản chỉ mở được một tab thì thanh ấy hiện một nút đứng một mình " +
      "hay không hiện. Lý do đầy đủ ghi ở đầu `app/cau-hinh/page.tsx`.",
  },
  {
    ten: "Tab Sơ đồ tổ chức — xoá bộ phận (§1, §12.4)",
    viSao:
      "Hợp đồng chưa có tuyến xoá bộ phận. §12.4 đòi chặn khi bộ phận còn cán bộ HOẶC còn hồ sơ " +
      "đang giữ, mà hồ sơ nằm ở các dịch vụ khác — cần một hợp đồng giữa các dịch vụ trước khi có " +
      "tuyến xoá.",
  },
  {
    ten: "Tab Trường bản đồ (§6)",
    viSao:
      "Hợp đồng chưa có tuyến nào. Đặc tả cũng chưa tự khớp: §6 vẽ ba thao tác `✎` · `Tắt` · `🗑` " +
      "trên mỗi trường, trong khi §11 chỉ đề xuất `GET/POST`, và câu chú thích mô tả việc xoá " +
      "đúng bằng kết quả của việc tắt (ẩn khỏi biểu mẫu, giữ dữ liệu). Cần chốt xoá và tắt khác " +
      "nhau thế nào trước khi dựng.",
  },
  {
    ten: "Tab Lời hệ thống (§7)",
    viSao:
      "Chưa chốt dịch vụ nào sở hữu 32 khoá `report.*` của bảng `loi_he_thong` (ADR 0024, mục " +
      "để trống), nên chưa có bảng và chưa có tuyến.",
  },
  {
    ten: "Tab Tự động hoá (§9)",
    viSao:
      "Hệ thống chưa có bộ lập lịch chạy việc nền theo nhịp cho từng đơn vị. Một công tắc bật/tắt " +
      "mà không có gì chạy phía sau là một lời hứa suông.",
  },
  {
    ten: "Tab Máy chủ thư (§10)",
    viSao:
      "§12.6 đòi mật khẩu SMTP được mã hoá khi lưu. Kho chưa có gói mã hoá dùng chung và chưa có " +
      "khoá bí mật cho việc ấy — một khoá bí mật mới cần người phụ trách hạ tầng quyết định.",
  },
  {
    ten: "Nút Nhập từ Excel (§1, §2, §3, §5)",
    viSao: "Hợp đồng chưa có tuyến nhập Excel nào cho các danh sách của màn này.",
  },
  {
    ten: "Xem nhật ký hệ thống (§12.1, quyền `admin.audit`)",
    viSao:
      "Chưa có tuyến đọc nhật ký. Nhật ký nằm rải ở từng dịch vụ, nên một màn xem chung cần một " +
      "hợp đồng giữa các dịch vụ trước.",
  },
  {
    ten: "Thêm vai trò mới ở tab Phân quyền (§4)",
    viSao:
      "Hợp đồng chỉ có `GET /api/v1/roles`, chưa có tuyến tạo vai trò; và chưa chốt ai được tạo " +
      "vai trò mới cho một đơn vị.",
  },
];
