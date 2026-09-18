/**
 * BỘ SINH vCARD CỦA CHÍNH CHÚNG TÔI — mặt đối xứng của `danh-thiep.ts`.
 *
 * `danh-thiep.ts` ĐỌC một tấm thiếp của người khác. Tệp này VIẾT tấm thiếp của VihatSoftware,
 * và văn bản nó sinh ra đi vào đúng hai chỗ: nội dung mã QR chìa ra cho người khác quét, và một
 * tệp tải xuống máy. Hai chỗ ấy là hai thứ người ngoài cầm về — nên một dấu chấm phẩy không
 * thoát ở đây là một tên công ty hiện sai trên danh bạ của một khách hàng.
 *
 * ⚠ HÀM THUẦN, KHÔNG TÁC DỤNG PHỤ. Chuỗi vào, chuỗi ra. Không đọc DOM, không gọi nền tảng,
 * không ghi log. Nhờ thế nó kiểm được đầy đủ dưới Node, và nó KHÔNG CÓ CHỖ NÀO để rò rỉ.
 *
 * ⚠ KHÔNG MỘT TRƯỜNG NÀO ĐƯỢC BỊA. Mọi giá trị đọc từ `content/company-profile.ts`, tệp chỉ
 * chứa thứ đã công bố trên vihatsoftware.com và vihatgroup.com. Thêm một địa chỉ, một mã số
 * thuế hay một người đại diện vào đây là phát tán một tuyên bố sai dưới tên một pháp nhân có
 * thật — và lần này là phát tán bằng một tệp người ta lưu vào danh bạ, chỗ khó thu hồi nhất.
 *
 * KHÔNG CÓ DỮ LIỆU CÁ NHÂN Ở ĐÂY. Hotline và email là đầu mối doanh nghiệp của ViHAT Group
 * (`CONTACT.ownerNote`), không định danh một cá nhân nào — nên tấm thiếp này là dữ liệu doanh
 * nghiệp, không phải dữ liệu cá nhân theo Nghị định 13/2023/NĐ-CP.
 */
import { COMPANY, CONTACT } from "../../content/company-profile";

/** Một dòng vCard: tên trường (có thể mang tham số) và giá trị chưa thoát. */
export type TruongVCard = {
  /**
   * Tên trường, dùng NGUYÊN VĂN. Được phép mang tham số (`TEL;TYPE=WORK,VOICE`) — dấu phẩy và
   * dấu chấm phẩy trong phần tên là CÚ PHÁP, không phải dữ liệu, nên chúng không được thoát.
   */
  ten: string;
  /** Giá trị, sẽ được thoát. Chuỗi rỗng nghĩa là "không có nguồn" và dòng bị bỏ hẳn. */
  gia_tri: string;
};

/**
 * Thoát một giá trị theo RFC 6350 §3.4: `\` `;` `,` và xuống dòng.
 *
 * THỨ TỰ KHÔNG ĐỔI ĐƯỢC: dấu gạch chéo ngược phải đi TRƯỚC. Thoát nó sau cùng thì chính những
 * dấu gạch chéo vừa thêm vào cho `;` và `,` lại bị thoát thêm một lần nữa, và giá trị hiện ra
 * trên máy người nhận có một gạch chéo thừa trước mỗi dấu phẩy.
 *
 * `\r\n` và `\r` quy về `\n` trước, vì một giá trị nhiều dòng phải thành đúng MỘT chuỗi `\n`
 * trong vCard — hai ký tự điều khiển lọt vào giữa dòng là một tệp hỏng, không phải một tệp xấu.
 */
export function thoatVCard(gia_tri: string): string {
  return gia_tri
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .replace(/\\/g, "\\\\")
    .replace(/;/g, "\\;")
    .replace(/,/g, "\\,")
    .replace(/\n/g, "\\n");
}

/** Kết thúc dòng của vCard là CRLF (RFC 6350 §3.2), không phải `\n`. */
const HET_DONG = "\r\n";

/**
 * Dựng một vCard 3.0 từ danh sách trường.
 *
 * TRƯỜNG RỖNG BỊ BỎ HẲN, KHÔNG SINH RA DÒNG TRỐNG: một `TITLE:` không có giá trị là một dòng
 * nói rằng "chúng tôi có chức danh, chỉ là để trống" — và với những trường chưa có nguồn (mã số
 * thuế, người đại diện) thì im lặng là câu trả lời đúng, xem README §"Còn thiếu".
 *
 * PHIÊN BẢN 3.0 CHỨ KHÔNG PHẢI 4.0: 3.0 là bản mọi ứng dụng danh bạ trên điện thoại đọc được,
 * và mục đích của tấm thiếp này là được lưu vào máy người khác, không phải đúng chuẩn mới nhất.
 */
export function dungVCard(truong: readonly TruongVCard[]): string {
  const dong = [
    "BEGIN:VCARD",
    "VERSION:3.0",
    ...truong
      .filter((mot) => mot.gia_tri !== "")
      .map((mot) => `${mot.ten}:${thoatVCard(mot.gia_tri)}`),
    "END:VCARD",
  ];
  // Dấu kết dòng cuối cùng cũng là CRLF: RFC 6350 yêu cầu nội dung kết thúc bằng một lần xuống
  // dòng, và một số ứng dụng danh bạ bỏ qua dòng `END:VCARD` nếu thiếu nó.
  return `${dong.join(HET_DONG)}${HET_DONG}`;
}

/**
 * Các trường của tấm thiếp VihatSoftware — ĐÚNG NHỮNG GÌ `COMPANY` VÀ `CONTACT` CÓ, không hơn.
 *
 * `N` có mặt bên cạnh `FN` vì vCard 3.0 khai `N` là bắt buộc, và một số ứng dụng danh bạ cũ bỏ
 * qua cả thẻ nếu thiếu nó. Đây là một tấm thiếp của TỔ CHỨC, nên `N` mang đúng tên tổ chức ấy,
 * không phải một cái tên người bịa ra để lấp chỗ.
 */
export const TRUONG_THIEP_CUA_CHUNG_TOI: readonly TruongVCard[] = [
  { ten: "FN", gia_tri: COMPANY.name },
  { ten: "N", gia_tri: `${COMPANY.name};;;;` },
  { ten: "ORG", gia_tri: COMPANY.name },
  { ten: "TEL;TYPE=WORK,VOICE", gia_tri: CONTACT.hotlineDialable },
  { ten: "EMAIL;TYPE=WORK", gia_tri: CONTACT.email },
  { ten: "URL", gia_tri: COMPANY.website },
];

/** Nội dung tấm thiếp, dùng cho cả mã QR lẫn tệp tải về. MỘT nguồn, hai chỗ dùng. */
export function vCardCuaChungToi(): string {
  return dungVCard(TRUONG_THIEP_CUA_CHUNG_TOI);
}

/**
 * Mã hoá base64 một chuỗi UTF-8.
 *
 * VÌ SAO KHÔNG GỌI THẲNG `btoa(chuoi)`: `btoa` chỉ nhận ký tự latin-1 và NÉM LỖI ngay khi gặp
 * một chữ có dấu. Tên công ty và mọi câu tiếng Việt đều có dấu, nên phải mã hoá ra byte UTF-8
 * trước rồi mới đưa từng byte cho `btoa`. Bỏ bước ấy thì nút tải về hỏng đúng ở tiếng Việt —
 * ngôn ngữ duy nhất ứng dụng này dùng.
 *
 * `downloadFile` của nền tảng nhận `fileBase64Data`, nên đây là dạng nó cần. Không có máy chủ
 * nào tham gia: chuỗi này đi thẳng từ bộ nhớ của app sang API ghi tệp của Zalo.
 */
export function base64Utf8(chuoi: string): string {
  const byte = new TextEncoder().encode(chuoi);
  let nhi_phan = "";
  for (let i = 0; i < byte.length; i += 1) nhi_phan += String.fromCharCode(byte[i]!);
  return btoa(nhi_phan);
}
