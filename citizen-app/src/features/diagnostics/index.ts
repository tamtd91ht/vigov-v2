/**
 * Cửa duy nhất vào bảng chẩn đoán — cùng cơ chế với `features/kham-pha/index.ts`, cùng lý do.
 *
 * Bảng chẩn đoán là công cụ nội bộ. Nó chỉ mở khi app được mở kèm `debug`, nên người dân không
 * thấy; nhưng trong bản gửi duyệt thì **mã của nó vẫn nằm trong bundle**, và một người duyệt đọc
 * bundle thấy một bảng in tham số nội bộ là một câu hỏi phải trả lời ở vòng duyệt. Biến thể
 * `goc` thay tệp này bằng `index.rong.ts` nên bảng ấy không còn tồn tại để mà hỏi.
 */
export { LaunchParamsPanel } from "./LaunchParamsPanel";
