/**
 * HỢP ĐỒNG VỚI MÁY CHỦ — MỘT TỆP, VÀ LÀ TỆP DUY NHẤT BIẾT NÓ.
 *
 *   POST <api-host>/api/v1/sessions
 *   gửi : { "accessToken": "<getAccessToken()>", "phoneToken": "<token của getPhoneNumber()>" }
 *   nhận: 201 { "token": "<bearer phiên>", "expiresAt": "<RFC3339>" }
 *   lỗi : 401 mã Zalo sai hoặc hết hạn · 502 máy chủ không với tới Zalo
 *
 * ⚠ MÁY CHỦ LÀ `vihat-miniapp` — KHO RIÊNG, KHÔNG PHẢI ViGov. Đây là backend thương mại của
 * VihatSoftware, độc lập với hệ thống hành chính. Đó là lý do tên tài nguyên là `sessions` chứ
 * không phải `citizen-sessions`: `citizen-session` là từ vựng của kênh công dân nhà nước, và
 * mượn từ vựng của một hệ thống khác là bước đầu của việc người sau tưởng hai thứ là một.
 *
 * HỢP ĐỒNG ĐÃ CHỐT (20/09/2026) nhưng máy chủ **đang dựng song song**, nên chưa gọi thử thật
 * được. Vì thế mọi thứ thuộc về dây vẫn nằm trọn ở đây:
 *
 *   • đường dẫn, tên trường gửi đi, hình dạng phản hồi → chỉ ở đây;
 *   • `goi-may-chu.ts` chỉ biết "gửi thân này tới địa chỉ kia rồi đọc trả lời bằng hàm kia";
 *   • màn hình chỉ biết bốn nhánh kết quả, và không một câu chữ nào nhắc tên đường dẫn hay tên
 *     trường — câu chữ chỉ nói MỤC ĐÍCH (đăng nhập bằng số Zalo, nhận thông báo ZNS).
 *
 *   Đổi hợp đồng = sửa tệp này, không sửa gì khác. Một tên trường chép ra JSX hay ra một test
 *   là một bản sao sẽ lệch, và lần lệch ấy là một `400` không ai đọc ra nguyên nhân.
 *
 * ⚠ TÊN TRƯỜNG VIẾT `camelCase` TIẾNG ANH, khác mọi tệp còn lại: đây là khuôn của bên kia dây,
 * không phải khuôn của ta. Việt hoá nó cho "đồng bộ" là tự tạo ra một bản dịch phải nhớ.
 *
 * ⚠ KHÔNG CÓ MỘT BÍ MẬT NÀO Ở ĐÂY, VÀ KHÔNG BAO GIỜ ĐƯỢC CÓ. Khoá bí mật của Mini App chỉ nằm
 * ở máy chủ (ADR 0020, bất biến 2 · luật 8, cấm #5): ứng dụng gửi đi hai mã dùng một lần, máy
 * chủ là bên đổi chúng. Một `appSecret` xuất hiện trong tệp này là sự cố bí mật, không phải
 * một tiện ích.
 *
 * ⚠ ĐỊA CHỈ MÁY CHỦ LÀ MỘT. Không bao giờ ghép địa chỉ theo từng đơn vị: máy chủ là bên ghi
 * phiên đang làm việc với ai, client không được tự chọn máy chủ của mình.
 * → `.claude/skills/zalo-miniapp-multi-tenant` §"The API host is singular".
 */
import { diaChiApi } from "../../api/dia-chi";

import type { MaDangNhap } from "../tinh-nang/zalo-api";

/**
 * Đường dẫn tuyến phát hành phiên. Không chứa gì của người dùng (luật 3, cấm #4).
 *
 * XUẤT RA CHỈ ĐỂ `bundle-for-zalo.test.ts` KHẲNG ĐỊNH NÓ CÓ MẶT TRONG CẢ HAI BẢN DỰNG — không
 * phải để tệp nào khác dùng. Phép kiểm ấy đọc hằng này thay vì gõ lại đường dẫn: gõ lại là tạo
 * bản sao thứ hai, và ngày hợp đồng đổi, bản sao ấy làm phép kiểm xanh vì **không tìm thấy gì**
 * — chính là chế độ hỏng mà tệp test kia sinh ra để chặn.
 */
export const DUONG_DAN_PHIEN = "/api/v1/sessions";

/**
 * PHIÊN GIỮ TRONG BỘ NHỚ — hình dạng của ta, không phải hình dạng của dây.
 *
 * ⚠ ĐÂY LÀ MỘT PHIẾU TẠM, VÀ MÃ Ở MỌI NƠI PHẢI COI NÓ LÀ TẠM. ADR 0005: sau khi công dân chọn
 * xã thì máy chủ PHÁT HÀNH LẠI phiên, nên bearer nhận được lúc đăng nhập không sống tới cuối
 * đời phiên làm việc. Bất cứ chỗ nào giả định "đăng nhập một lần rồi giữ mãi" đều sẽ phải viết
 * lại vào đúng ngày bước chọn xã xuất hiện — rẻ hơn nhiều là đừng dựng giả định ấy ngay bây giờ.
 *
 * KHÔNG CÓ `so_dien_thoai`, và sẽ không bao giờ có: ứng dụng nhận mã, không nhận số (luật 3).
 */
export type Phien = {
  /** Bearer của phiên. Chỉ nằm trong bộ nhớ, không vẽ ra màn hình, không ghi xuống máy. */
  token: string;
  /** Hạn dùng, nguyên văn máy chủ trả về. Màn hình chỉ đọc lại, không tự tính hạn. */
  het_han: string;
};

/**
 * Địa chỉ đầy đủ của tuyến. Rỗng khi chưa khai host — `goi-may-chu.ts` fail-closed ở đó.
 *
 * HOST ĐỌC TỪ `api/dia-chi.ts` TỪ 22/09/2026: tuyến thứ hai (`/api/v1/requests`) cần đúng cùng
 * một host và đúng cùng cách cắt dấu `/` thừa, nên chỗ đọc biến môi trường chuyển ra một tệp
 * dùng chung thay vì được chép sang tệp thứ hai. Thứ VẪN CHỈ CÓ Ở ĐÂY là đường dẫn `/api/v1/sessions`.
 */
export function diaChiPhien(): string {
  return diaChiApi(DUONG_DAN_PHIEN);
}

/**
 * Hai mã của Zalo → thân yêu cầu đúng khuôn của máy chủ.
 *
 * ĐÂY LÀ CHỖ DUY NHẤT HAI TÊN TRƯỜNG GỬI ĐI ĐƯỢC VIẾT RA. Màn hình và tầng gọi mạng chỉ chuyền
 * `MaDangNhap` — kiểu của ta, đặt tên theo việc chứ không theo dây.
 */
export function thanYeuCau(ma: MaDangNhap): string {
  return JSON.stringify({ accessToken: ma.ma_truy_cap, phoneToken: ma.ma_so_dien_thoai });
}

/**
 * Thân trả lời → `Phien`, hoặc `null` nếu không đúng khuôn.
 *
 * KIỂM TỪNG TRƯỜNG CHỨ KHÔNG ÉP KIỂU: một `as TraLoiPhien` sẽ cho `undefined` đi tiếp và hiện
 * ra màn hình dưới dạng chữ "undefined" — hoặc tệ hơn, một phiên rỗng trông như đã đăng nhập.
 */
export function docTraLoi(than: unknown): Phien | null {
  if (typeof than !== "object" || than === null) return null;
  const { token, expiresAt } = than as Record<string, unknown>;
  if (typeof token !== "string" || token === "") return null;
  if (typeof expiresAt !== "string") return null;
  return { token, het_han: expiresAt };
}

/**
 * HOST ĐỌC LÚC DỰNG, KHÔNG PHẢI LÚC CHẠY — nay ở `api/dia-chi.ts`, một chỗ cho cả hai hợp đồng.
 *
 * ⚠ BẢN NỘP CŨNG GỌI MÁY CHỦ, NÊN QUÊN BIẾN `VIGOV_API_HOST` LÀ NỘP MỘT NÚT ĐĂNG NHẬP KHÔNG
 * ĐĂNG NHẬP NỔI. `scripts/deploy.mjs` chặn đường đẩy khi biến chưa khai — chặn ở đó chứ không
 * ném lỗi lúc dựng, vì `npm test` và `npm run dev` phải chạy được trên một máy chưa có địa chỉ
 * máy chủ nào. Lý do đầy đủ của cách đọc ấy nằm trong `api/dia-chi.ts`.
 */
