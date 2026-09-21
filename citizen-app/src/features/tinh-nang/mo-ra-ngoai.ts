/**
 * CỬA DUY NHẤT ĐƯA NGƯỜI DÙNG RA MỘT TRANG BÊN NGOÀI.
 *
 * ⚠ VÌ SAO MỘT LỚP BỌC MỎNG NHƯ THẾ NÀY LẠI ĐÁNG CÓ — nó không phải một lớp trừu tượng cho đẹp:
 *
 *   Chính sách quyền riêng tư khai ĐẾM ĐƯỢC: *"Có năm chỗ ứng dụng mở một trang bên ngoài"*, rồi
 *   liệt kê đủ năm. Trước tệp này, "năm" là một con số đếm bằng mắt trên một cây mã có ba hình
 *   dạng mở trang khác nhau (`moTrangWeb`, `<a target="_blank">`, và bất cứ thứ gì người sau nghĩ
 *   ra). Nó đã phải sửa ba lần trong hai ngày, lần nào cũng nhờ người đọc lại văn bản.
 *
 *   Với cửa này thì **một lối ra không khai được là một lối ra không viết ra được**: `moRaNgoai`
 *   chỉ nhận một `MaDichRaNgoai`, tức một mã đã có trong `content/dich-ra-ngoai.ts`, tức một dòng
 *   đã có trong câu khai của chính sách — vì chính câu ấy được dựng từ danh sách đó.
 *
 *   Nửa còn lại của cơ chế nằm ở `content/dich-ra-ngoai.test.ts`: nó cấm `moTrangWeb(`,
 *   `target="_blank"`, `window.open(` và gán `location.href` ở MỌI tệp ngoài hai tệp này. Không có
 *   lệnh cấm ấy thì cửa này chỉ là một lời đề nghị.
 *
 * ⚠ ĐI QUA `openWebview` CỦA NỀN TẢNG, KHÔNG PHẢI MỘT THẺ `<a target="_blank">`.
 *
 *   Bên trong Zalo, một liên kết ngoài mở bằng thẻ `a` không có thanh điều hướng và không có
 *   đường quay lại app — người dùng phải đóng cả Mini App để về chỗ cũ. Đây là lý do hai neo
 *   website (màn Liên hệ · trang chi tiết giải pháp) đã đổi từ `<a target="_blank">` sang nút gọi
 *   hàm này ngày 21/09/2026: chúng là hai chỗ CUỐI CÙNG còn đi bằng đường kia.
 *
 * KHÔNG CÓ GÌ CỦA NGƯỜI DÙNG ĐI KÈM. Địa chỉ truyền vào là địa chỉ đã có sẵn trong app (bản đồ
 * dựng từ địa chỉ văn phòng đã công bố, trang tin, trang chủ, OA) hoặc chính chuỗi người dùng vừa
 * quét được. Không token, không mã định danh, không toạ độ — và chính sách khai đúng như vậy.
 */

import { DICH_MO_RA_NGOAI, type MaDichRaNgoai } from "../../content/dich-ra-ngoai";

import { moTrangWeb } from "./zalo-api";

const DA_KHAI: ReadonlySet<string> = new Set(DICH_MO_RA_NGOAI.map((d) => d.ma));

/**
 * Mở một trang ngoài cho một đích ĐÃ KHAI. Trả về `true` khi nền tảng mở được.
 *
 * ⚠ CHƯA KHAI THÌ KHÔNG MỞ — fail closed, và đây không phải một nhánh chết:
 *
 *   Kiểu `MaDichRaNgoai` chặn ở tầng biên dịch, nhưng nó chặn được chừng nào không ai nới kiểu
 *   ấy ra (`string`) hay ép kiểu tại chỗ gọi. Ngày điều đó xảy ra, thứ đúng để làm là KHÔNG mở
 *   trang, chứ không phải mở một trang mà văn bản pháp lý không khai. Người dùng thấy đúng câu
 *   họ thấy khi nền tảng từ chối — một câu nói việc cần làm tiếp, không phải một mã lỗi.
 */
export async function moRaNgoai(dich: MaDichRaNgoai, duong_dan: string): Promise<boolean> {
  if (!DA_KHAI.has(dich) || duong_dan === "") return false;
  const ket_qua = await moTrangWeb(duong_dan);
  return ket_qua.kieu === "xong";
}
