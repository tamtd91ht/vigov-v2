/**
 * CỬA DUY NHẤT ĐƯA NGƯỜI DÙNG RA MỘT TRANG BÊN NGOÀI.
 *
 * ⚠ VÌ SAO MỘT LỚP BỌC MỎNG NHƯ THẾ NÀY LẠI ĐÁNG CÓ — nó không phải một lớp trừu tượng cho đẹp:
 *
 *   Chính sách quyền riêng tư khai ĐẾM ĐƯỢC: *"Có <N> chỗ ứng dụng mở một trang bên ngoài"*, rồi
 *   liệt kê đủ N (bảy từ 01/10/2026). Trước tệp này, con số ấy là một con số đếm bằng mắt trên một cây mã có ba hình
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
 * dựng từ địa chỉ văn phòng đã công bố, trang tin, trang chủ, OA), chính chuỗi người dùng vừa
 * quét được, hoặc (01/10/2026) đường dẫn video xã đã đăng kèm một tin và liên kết xã gắn trong thân bài
 * hay trên ảnh trang chủ — `App.tsx` tiêm lời gọi `moRaNgoai("video", …)` / `moRaNgoai("lien-ket-xa", …)`
 * xuống nửa nhà nước, vì nửa ấy không được nhập tệp này. Không token, không
 * mã định danh, không toạ độ — và chính sách khai đúng như vậy.
 */

import { DICH_MO_RA_NGOAI, type MaDichRaNgoai } from "../../content/dich-ra-ngoai";

import { moTrangWeb } from "./zalo-api";

const DA_KHAI: ReadonlySet<string> = new Set(DICH_MO_RA_NGOAI.map((d) => d.ma));

/**
 * Destinations whose address is chosen by a COMMUNE (an article body link, a home-strip banner) and must
 * be re-checked here, at the door, not only by the caller.
 *
 * WHY THE DOOR CHECKS AGAIN (security review 02/10/2026, F2): every caller today runs `readHttpsLink`
 * before calling (`cong-dan/man/leave-app.tsx`, `TrangXa.tsx`), so this is defence in depth, not a fix
 * of a live hole. But the opener is the ONE place every exit passes through; a future caller that forgets
 * the check would hand `openWebview` a `javascript:`/`http:` address, or `https://gov.vn@other.example`,
 * whose confirmation sentence names a host the citizen never reaches.
 */
// `video` joined 02/10/2026 (owner: "làm theo đề xuất"): its address is the commune's `video_url`, and the
// caller's `readVideoUrl` checks https but not the user part.
const HTTPS_ONLY: ReadonlySet<MaDichRaNgoai> = new Set<MaDichRaNgoai>(["lien-ket-xa", "video"]);

/**
 * An absolute `https:` URL with a host and no user part. A LOCAL COPY of `readHttpsLink`
 * (`cong-dan/api/hop-dong-cong-khai.ts`), on purpose: this file is in the commercial half, and
 * `ranh-gioi-hai-nua.test.ts` forbids it (3a) — and every file outside `./cong-dan/` (the ViGov-client
 * ban) — from importing that module. Keep the two rules identical; if one tightens, tighten both.
 */
function isHttpsLink(v: string): boolean {
  try {
    const u = new URL(v);
    return u.protocol === "https:" && u.hostname !== "" && u.username === "" && u.password === "";
  } catch {
    return false;
  }
}

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
  // Same failure result as a platform refusal, so the caller shows its own "what to do next" sentence.
  if (HTTPS_ONLY.has(dich) && !isHttpsLink(duong_dan)) return false;
  const ket_qua = await moTrangWeb(duong_dan);
  return ket_qua.kieu === "xong";
}
