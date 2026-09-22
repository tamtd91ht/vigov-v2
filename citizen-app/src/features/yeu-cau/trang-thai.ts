/**
 * MÃ TRẠNG THÁI → NHÃN TIẾNG VIỆT. ĐÚNG MỘT CHỖ TRONG CẢ ỨNG DỤNG.
 *
 * Máy chủ cố ý trả về MÃ (`moi`, `dang_xu_ly`, `da_lien_he`, `dong`) chứ không trả câu chữ, để
 * đổi một nhãn không phải phát hành lại máy chủ (`internal/httpapi/yeu_cau.go`, khối đầu tệp).
 * Vế còn lại của quyết định ấy là NGHĨA VỤ của phía này: giữ bảng dịch ở đúng một chỗ.
 *
 * Hai bảng ở hai màn thì một hôm nào đó cùng một yêu cầu đọc ra hai trạng thái khác nhau tuỳ
 * người dùng đang đứng ở đâu — và người dùng không có cách nào biết cái nào đúng.
 *
 * ⚠ `Record<MaTrangThai, …>` CHỨ KHÔNG PHẢI `Record<string, …>`: thêm một mã ở hợp đồng mà quên
 * nhãn ở đây thì `tsc` đỏ, chứ không phải một ô trống trên màn hình của người dùng.
 *
 * ⚠ NHÃN KHÔNG ĐƯỢC MANG MỘT CAM KẾT THỜI GIAN. "Đang xử lý" chứ không "Xử lý trong 24 giờ":
 * không ai cam kết con số ấy, và người phát hiện ra là người đang chờ.
 */
import type { LoaiYeuCau, MaTrangThai } from "../../api/hop-dong-yeu-cau";

export const NHAN_TRANG_THAI: Readonly<Record<MaTrangThai, string>> = {
  moi: "Đã tiếp nhận",
  dang_xu_ly: "Đang xử lý",
  da_lien_he: "Đã liên hệ với bạn",
  dong: "Đã khép lại",
};

/**
 * Câu giải thích ngắn dưới mỗi nhãn.
 *
 * Một nhãn hai từ không nói cho người đọc biết họ cần làm gì tiếp, và với người lớn tuổi thì
 * "Đã khép lại" mà không giải thích là một câu nghe như bị từ chối.
 */
export const GIAI_THICH_TRANG_THAI: Readonly<Record<MaTrangThai, string>> = {
  moi: "Yêu cầu của bạn đã vào hệ thống của chúng tôi.",
  dang_xu_ly: "Chúng tôi đang chuẩn bị nội dung để trả lời bạn.",
  da_lien_he: "Chúng tôi đã liên hệ với bạn về yêu cầu này.",
  dong: "Yêu cầu này đã xong. Bạn có thể gửi một yêu cầu mới bất cứ lúc nào.",
};

/** Nhãn loại yêu cầu. Cùng lý do, cùng chỗ. */
export const NHAN_LOAI: Readonly<Record<LoaiYeuCau, string>> = {
  consult: "Tư vấn và báo giá",
  callback: "Đề nghị gọi lại",
};

/**
 * Ngày giờ cho người đọc, từ chuỗi RFC3339 máy chủ trả về.
 *
 * Giá trị lạ thì trả chuỗi RỖNG và màn hình bỏ hẳn dòng ấy — không hiện "Invalid Date", thứ vừa
 * là một chuỗi kỹ thuật vừa là tiếng Anh, trên một màn hình tiếng Việt.
 */
export function ngayDoc(rfc3339: string): string {
  if (rfc3339 === "") return "";
  const luc = new Date(rfc3339);
  return Number.isNaN(luc.getTime()) ? "" : luc.toLocaleString("vi-VN");
}
