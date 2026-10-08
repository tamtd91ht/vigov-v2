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
      "Tự gửi báo cáo tuần vào đầu tuần và báo cáo tháng vào ngày mùng 1. Việc này chờ chức năng xuất " +
      "báo cáo của màn Báo cáo: chưa xuất được tệp thì chưa có gì để gửi.",
  },
  {
    ten: "Thêm thời hạn cho một lĩnh vực",
    viSao:
      "Đặt thời hạn riêng cho một lĩnh vực, khác với thời hạn mặc định của loại việc. Chưa có đường ghi " +
      "nào nhận mã lĩnh vực: mã gửi lên phải được đối chiếu với bộ mã lĩnh vực chung, và cách đối chiếu " +
      "ấy chưa được chốt (ADR 0026). Số giờ của các dòng đang có thì sửa được bằng nút bút chì trên dòng.",
  },
  {
    ten: "Xoá thời hạn riêng",
    viSao:
      "Bỏ thời hạn riêng của một lĩnh vực để lĩnh vực ấy quay về thời hạn mặc định của loại việc. Máy " +
      "chủ chưa có đường xoá dòng thời hạn; phần này đang được làm (ADR 0079).",
  },
  {
    ten: "Màu của mục danh mục",
    viSao:
      "Gắn một màu cho từng mục danh mục để nhận ra nhanh trên các màn khác. Danh mục chưa lưu được màu; " +
      "phần này đang được làm (ADR 0079).",
  },
  {
    ten: "Thêm câu mới",
    viSao:
      "Xã tự thêm câu nói với người dân ngoài các câu đi kèm phần mềm. Máy chủ hiện chỉ cho sửa lời các " +
      "câu có sẵn; phần thêm câu đang được làm (ADR 0079).",
  },
  {
    ten: "Tắt câu hệ thống",
    viSao:
      "Tắt một câu đã sửa để hệ thống quay về dùng lời gốc mà không mất lời đã sửa. Máy chủ chưa có trạng " +
      "thái bật/tắt cho câu; trong lúc chờ, dùng \"Khôi phục lời gốc\" (ADR 0079).",
  },
  {
    ten: "Lần thử gửi thư gần nhất",
    viSao:
      "Ghi lại lần gửi thư thử gần nhất (lúc nào, tới đâu, có gửi được không) để người sau xem lại. Máy " +
      "chủ chưa lưu kết quả này; hiện chỉ thấy kết quả của lần thử ngay trên màn (ADR 0079).",
  },
  {
    ten: "Máy chủ thư của nền tảng",
    viSao:
      "Cho biết khi xã chưa khai máy chủ thư thì thư có đi bằng máy chủ chung của nền tảng hay không. Máy " +
      "chủ chưa trả thông tin này (ADR 0079).",
  },
  {
    ten: "Con bot của xã",
    viSao:
      "Xã dùng con bot Zalo mang tên mình thay cho bot chung của nền tảng: nhập mã bot, kiểm tra kết nối, " +
      "đăng ký webhook, quay về bot chung. Phần này đang được làm (ADR 0079); hiện mọi tin đi bằng bot chung.",
  },
  {
    ten: "Nhắc trước khi đến hạn qua Zalo",
    viSao:
      "Chọn nhắc trước bao nhiêu ngày khi việc sắp đến hạn. Ngưỡng \"sắp đến hạn\" hiện lấy theo cột \"Sắp " +
      "đến hạn khi còn\" ở tab Thời hạn xử lý; chỉnh riêng ở đây đang được làm (ADR 0079).",
  },
  {
    ten: "Thêm loại việc nhắn qua Zalo",
    viSao:
      "Chọn từng việc cụ thể được nhắn qua Zalo (giao việc mới, văn bản chuyển tới, phản ánh bị mở lại…). " +
      "Máy chủ hiện nhắn được bốn loại: sắp đến hạn, quá hạn, đôn đốc lên cấp trên, bản tin đầu tuần; các " +
      "loại còn lại đang được làm (ADR 0079).",
  },
  {
    ten: "Số cán bộ đã ghép nối",
    viSao:
      "Đếm bao nhiêu cán bộ trong xã đã ghép nối Zalo trên tổng số cán bộ. Máy chủ chưa trả tổng số này " +
      "(ADR 0079).",
  },
  {
    ten: "Cán bộ chưa ghép nối",
    viSao:
      "Liệt kê cả cán bộ chưa ghép nối Zalo để nhắc họ tự ghép nối. Máy chủ hiện chỉ trả người đã ghép " +
      "nối (ADR 0079).",
  },
];
