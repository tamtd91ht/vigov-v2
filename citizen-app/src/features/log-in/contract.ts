/**
 * HỢP ĐỒNG VỚI MÁY CHỦ — MỘT TỆP, VÀ LÀ TỆP DUY NHẤT BIẾT NÓ.
 *
 *   POST <api-host>/api/v1/sessions
 *   gửi : { "accessToken": "<getAccessToken()>", "phoneToken": "<token của getPhoneNumber()>" }
 *   nhận: 201 { "token": "<bearer phiên>", "expiresAt": "<RFC3339>" }
 *   lỗi : 401 mã Zalo sai hoặc hết hạn · 502 máy chủ không với tới Zalo
 *
 * ⚠ MÁY CHỦ LÀ `vihat-miniapp` — KHO RIÊNG, KHÔNG PHẢI ViGov. Đây là backend thương mại của
 * Tập đoàn ViHAT Group (bên vận hành, câu mở #28), độc lập với hệ thống hành chính. Đó là lý do tên tài nguyên là `sessions` chứ
 * không phải `citizen-sessions`: `citizen-session` là từ vựng của kênh công dân nhà nước, và
 * mượn từ vựng của một hệ thống khác là bước đầu của việc người sau tưởng hai thứ là một.
 *
 * HỢP ĐỒNG ĐÃ CHỐT (20/09/2026) nhưng máy chủ **đang dựng song song**, nên chưa gọi thử thật
 * được. Vì thế mọi thứ thuộc về dây vẫn nằm trọn ở đây:
 *
 *   • đường dẫn, tên trường gửi đi, hình dạng phản hồi → chỉ ở đây;
 *   • `server-calls.ts` chỉ biết "gửi thân này tới địa chỉ kia rồi đọc trả lời bằng hàm kia";
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
 *
 * ⚠ `ma_truy_cap` · `ma_so_dien_thoai` GIỮ TÊN CŨ: chúng là trường của `MaDangNhap` trong
 * `features/tinh-nang/zalo-api.ts` — nửa thương mại, ngoài lượt đổi tên này.
 */
import { diaChiApi } from "../../api/dia-chi";
import type { TruongGuiDi } from "../../api/hop-dong-yeu-cau";

import { isDomain } from "../../lib/launch-params";

import type { LocationCodes, MaDangNhap } from "../tinh-nang/zalo-api";

/**
 * Đường dẫn tuyến phát hành phiên. Không chứa gì của người dùng (luật 3, cấm #4).
 *
 * XUẤT RA CHỈ ĐỂ `bundle-for-zalo.test.ts` KHẲNG ĐỊNH NÓ CÓ MẶT TRONG CẢ HAI BẢN DỰNG — không
 * phải để tệp nào khác dùng. Phép kiểm ấy đọc hằng này thay vì gõ lại đường dẫn: gõ lại là tạo
 * bản sao thứ hai, và ngày hợp đồng đổi, bản sao ấy làm phép kiểm xanh vì **không tìm thấy gì**
 * — chính là chế độ hỏng mà tệp test kia sinh ra để chặn.
 */
export const SESSION_PATH = "/api/v1/sessions";

/**
 * PHIÊN GIỮ TRONG BỘ NHỚ — hình dạng của ta, không phải hình dạng của dây.
 *
 * ⚠ ĐÂY LÀ MỘT PHIẾU TẠM, VÀ MÃ Ở MỌI NƠI PHẢI COI NÓ LÀ TẠM. ADR 0005: sau khi công dân chọn
 * xã thì máy chủ PHÁT HÀNH LẠI phiên, nên bearer nhận được lúc đăng nhập không sống tới cuối
 * đời phiên làm việc. Bất cứ chỗ nào giả định "đăng nhập một lần rồi giữ mãi" đều sẽ phải viết
 * lại vào đúng ngày bước chọn xã xuất hiện — rẻ hơn nhiều là đừng dựng giả định ấy ngay bây giờ.
 *
 * KHÔNG CÓ `phone_number`, và sẽ không bao giờ có: ứng dụng nhận mã, không nhận số (luật 3).
 */
export type Session = {
  /** Bearer của phiên. Chỉ nằm trong bộ nhớ, không vẽ ra màn hình, không ghi xuống máy. */
  token: string;
  /** Hạn dùng, nguyên văn máy chủ trả về. Màn hình chỉ đọc lại, không tự tính hạn. */
  expires_at: string;
};

/**
 * Địa chỉ đầy đủ của tuyến. Rỗng khi chưa khai host — `server-calls.ts` fail-closed ở đó.
 *
 * HOST ĐỌC TỪ `api/dia-chi.ts` TỪ 22/09/2026: tuyến thứ hai (`/api/v1/requests`) cần đúng cùng
 * một host và đúng cùng cách cắt dấu `/` thừa, nên chỗ đọc biến môi trường chuyển ra một tệp
 * dùng chung thay vì được chép sang tệp thứ hai. Thứ VẪN CHỈ CÓ Ở ĐÂY là đường dẫn `/api/v1/sessions`.
 */
export function sessionAddress(): string {
  return diaChiApi(SESSION_PATH);
}

/**
 * HAI THỨ TUYẾN NÀY ĐƯA RA KHỎI MÁY — và câu khai từng thứ, cho hồ sơ nộp Zalo.
 *
 * ⚠ BẢNG NÀY RA ĐỜI 22/09/2026 VÌ MỘT CÂU SAI ĐÃ LỌT VÀO HỒ SƠ PHÁP LÝ.
 *
 *   `tmp/xin-quyen-zalo/README.md` khai *"Đây là chức năng DUY NHẤT của ứng dụng có dữ liệu rời
 *   khỏi máy"*. Câu ấy đúng cho tới giai đoạn B, rồi thành SAI — và không một phép kiểm nào đỏ
 *   lên, vì bảng sinh ra của hồ sơ đọc `KHAI_BAO_LOI_GOI`, mà bảng ấy **chỉ biết lời gọi
 *   `zmp-sdk`**. Một `fetch` thuần vô hình với nó.
 *
 *   Nên "thứ gì rời khỏi máy" nay là một BẢNG, không phải một câu văn xuôi: tuyến này khai hai
 *   dòng dưới đây, tuyến yêu cầu khai `TRUONG_GUI_DI` (`api/hop-dong-yeu-cau.ts`), và
 *   `content/ket-xuat-ho-so.ts` sinh ra khối "Những gì rời khỏi máy" từ cả hai.
 *
 * ⚠ KHOÁ HAI CHIỀU với `sessionRequestBody` ngay dưới, đúng cơ chế của chính sách quyền riêng tư: mọi
 * khoá hàm ấy sinh ra phải có một dòng ở đây, và mọi dòng ở đây phải là một khoá hàm ấy sinh ra.
 * `ket-xuat-ho-so.test.ts` giữ cả hai chiều.
 */
export const SESSION_SENT_FIELDS: readonly TruongGuiDi[] = [
  {
    khoa: "accessToken",
    trong_chinh_sach:
      "mã phiên Zalo của bạn — cho biết bạn là tài khoản Zalo nào đối với riêng ứng dụng này, và KHÔNG chứa tên hay ảnh đại diện",
  },
  {
    khoa: "phoneToken",
    trong_chinh_sach:
      "mã số điện thoại do Zalo cấp sau khi bạn đồng ý — SỐ ĐIỆN THOẠI KHÔNG NẰM TRONG MÃ NÀY, chỉ máy chủ đổi được mã thành số",
  },
];

/**
 * Hai mã của Zalo → thân yêu cầu đúng khuôn của máy chủ.
 *
 * ĐÂY LÀ CHỖ DUY NHẤT HAI TÊN TRƯỜNG GỬI ĐI ĐƯỢC VIẾT RA. Màn hình và tầng gọi mạng chỉ chuyền
 * `MaDangNhap` — kiểu của ta, đặt tên theo việc chứ không theo dây.
 */
export function sessionRequestBody(code: MaDangNhap): string {
  return JSON.stringify({ accessToken: code.ma_truy_cap, phoneToken: code.ma_so_dien_thoai });
}

/**
 * Thân trả lời → `Session`, hoặc `null` nếu không đúng khuôn.
 *
 * KIỂM TỪNG TRƯỜNG CHỨ KHÔNG ÉP KIỂU: một `as` sẽ cho `undefined` đi tiếp và hiện ra màn hình dưới
 * dạng chữ "undefined" — hoặc tệ hơn, một phiên rỗng trông như đã đăng nhập.
 */
export function readResponse(body: unknown): Session | null {
  if (typeof body !== "object" || body === null) return null;
  const { token, expiresAt } = body as Record<string, unknown>;
  if (typeof token !== "string" || token === "") return null;
  if (typeof expiresAt !== "string") return null;
  return { token, expires_at: expiresAt };
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * NHÁNH CẦU PHIÊN ViGov CỦA CÙNG TUYẾN — `POST /api/v1/sessions` khi `vihat-miniapp` bật cầu
 * (ADR 0045 · ADR 0047 câu 7; `vihat-miniapp` `internal/httpapi/sessions.go:19-28`,
 * `sessions_vigov.go:158-175`)
 *
 *   gửi : { "accessToken", "communeHostHint": "<tên miền xã>", "communeConfirmed": true }
 *         — KHÔNG `phoneToken`: mở phiên sau khi xác nhận xã không xin số điện thoại (ADR 0045 câu 2).
 *         Thân KÈM `phoneToken` là thân thứ hai, chỉ cho lần mở lại khi ViGov đòi số —
 *         `bridgeBodyWithPhone` bên dưới
 *   nhận: 201 { "vigovSession": { "token"?, "expiresAt"?, "tenantDisplayName", "phoneVerified",
 *                                  "communePrimaryHost"? } }
 *   lỗi : 400 · 401 · 422 xã/app chưa sẵn sàng · 502 · 503 cầu tạm ngưng
 *
 * ⚠ HAI PHIÊN, HAI KHOÁ, KHÔNG BAO GIỜ LẪN. Phiếu thương mại nằm ở `token` GỐC thân trả lời (`readResponse`
 *   ở trên); phiên ViGov nằm dưới `vigovSession`. `readVigovBridgeResponse` KHÔNG đọc `token` gốc — cầm nhầm
 *   phiếu thương mại rồi gửi tới ViGov là đúng thứ ADR 0032 cấm. Và `readResponse` không đọc `vigovSession`:
 *   hành vi của khối đăng nhập (Tư vấn · Yêu cầu của tôi) giữ nguyên.
 *
 * ⚠ TWO WAYS THIS BODY FAILS TODAY, neither a wiring fault:
 *   · bridge NOT configured on that deployment — the old branch demands `phoneToken` and answers 400;
 *     `server-calls.ts` reads it as "cầu tắt".
 *   · bridge configured — `vihat-miniapp` routes any login carrying `communeHostHint` to ViGov's
 *     OpenCitizenSession (`internal/httpapi/sessions.go:81`), but its Zalo account-id step always
 *     refuses (`internal/zalo/ma_tai_khoan.go:45-47`, ADR 0045 UNKNOWN #2) → 503 "cầu tạm ngưng".
 *   Either way no `vigovSession` arrives, so the citizen session stays empty in practice.
 *
 * ⚠ TÊN MIỀN XÃ LÀ GỢI Ý, KHÔNG PHẢI THAM CHIẾU XÃ: máy chủ ViGov phân giải nó lúc xác nhận (ADR 0047
 *   câu 3). Nó rời khỏi máy trong thân này — nói ra ở `VIGOV_BRIDGE_SENT_FIELDS` ngay dưới.
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type VigovBridgeRequest = {
  /** Access token của phiên Zalo (`getAccessToken`). Cùng tên với trường của `MaDangNhap`. */
  ma_truy_cap: string;
  /** Tên miền xã công dân vừa xác nhận trên màn xác nhận. */
  commune_domain: string;
};

/**
 * Ba trường nhánh cầu đưa ra khỏi máy — và câu khai từng trường.
 *
 * ⚠ KHOÁ BA CHIỀU (27/09/2026): mọi khoá `vigovBridgeRequestBody` sinh ra có một dòng ở đây và ngược lại
 * (`vigov-bridge.test.ts`); mọi `trong_chinh_sach` có mặt NGUYÊN VĂN trong mục Đăng nhập của chính sách
 * (`content/chinh-sach.test.ts`); và `content/ket-xuat-ho-so.ts` in bảng này vào khối "Những gì rời
 * khỏi máy" của hồ sơ nộp Zalo. Câu trong chính sách là CHUỖI VIẾT SẴN, không ghép lúc chạy:
 * `bundle-for-zalo.test.ts` đòi từng đoạn của mục Đăng nhập có mặt nguyên văn trong bundle.
 *
 * Mỗi câu khai viết để ĐỨNG TRONG MỘT DANH SÁCH cách nhau bằng `;` — không tự mang dấu `;`.
 */
export const VIGOV_BRIDGE_SENT_FIELDS: readonly TruongGuiDi[] = [
  {
    khoa: "accessToken",
    trong_chinh_sach:
      "mã phiên Zalo của bạn, để máy chủ mở phiên làm việc với xã ấy — mã này KHÔNG chứa tên hay ảnh đại diện của bạn",
  },
  {
    khoa: "communeHostHint",
    trong_chinh_sach: "tên miền của xã ấy, lấy từ mã QR hoặc đường liên kết bạn đã dùng để mở ứng dụng",
  },
  {
    khoa: "communeConfirmed",
    trong_chinh_sach: "việc bạn đã bấm xác nhận đúng xã — máy chủ không mở phiên với xã nếu bạn chưa xác nhận",
  },
];

/** `communeConfirmed` là HẰNG `true`: thân này chỉ được dựng sau cú bấm xác nhận. */
export function vigovBridgeRequestBody(req: VigovBridgeRequest): string {
  return JSON.stringify({
    accessToken: req.ma_truy_cap,
    communeHostHint: req.commune_domain,
    communeConfirmed: true,
  });
}

/* ────────────────────────────────────────────────────────────────────────────────────────────
 * THÂN THỨ HAI CỦA NHÁNH CẦU — MỞ LẠI PHIÊN KÈM `phoneToken` (28/09/2026, quyết định của người dùng)
 *
 *   gửi : { "accessToken", "communeHostHint", "communeConfirmed": true, "phoneToken" }
 *
 * CHỈ chạy khi một tuyến phản ánh của ViGov trả 403 `chua_xac_thuc_so` VÀ công dân tự bấm "Đồng ý chia
 * sẻ số điện thoại" rồi đồng ý trên hộp thoại của Zalo (`citizen/screens/phone-verification.tsx`). Đúng
 * luồng ADR 0045:64-65: "khi gửi hồ sơ thì thêm getPhoneNumber()"; `vihat-miniapp` nhận `phoneToken`
 * TUỲ CHỌN ở nhánh này (`internal/httpapi/sessions_vigov.go:24-26, 77-94`) và chuyển số sang ViGov mà
 * không lưu vào CSDL thương mại (cùng tệp, điểm 2).
 *
 * THÂN CỦA BƯỚC XÁC NHẬN XÃ Ở TRÊN KHÔNG ĐỔI: nó vẫn không mang `phoneToken`, và câu "Bước xác nhận xã
 * không gửi mã số điện thoại của bạn" trong chính sách vẫn đúng. Đây là một thân KHÁC, một hàm KHÁC,
 * để không đường nào gắn nhầm mã số vào cú bấm "Đúng, tiếp tục".
 *
 * ⚠ CHÍNH SÁCH QUYỀN RIÊNG TƯ CHƯA CÓ CÂU CHO THÂN NÀY. Câu khai `phoneToken` dưới đây là câu của khối
 *   đăng nhập, dùng lại nguyên văn (nó đúng cho cả hai); nhưng mục Đăng nhập của chính sách chưa nói
 *   rằng mã số còn đi ở đường này, tới ViGov. Câu ấy là lời văn pháp lý của chủ dự án — CÒN NỢ, không
 *   viết thay ở đây. `content/chinh-sach.test.ts` ghim đúng khoảng hở ấy (`phoneToken`, và chỉ nó).
 * ──────────────────────────────────────────────────────────────────────────────────────────── */

/** Yêu cầu mở lại: bước xác nhận xã + mã số điện thoại của `getPhoneNumber`. */
export type BridgeRequestWithPhone = VigovBridgeRequest & Pick<MaDangNhap, "ma_so_dien_thoai">;

/**
 * Bốn trường thân mở lại đưa ra khỏi máy. Ba dòng đầu LÀ `VIGOV_BRIDGE_SENT_FIELDS`; dòng `phoneToken` LÀ
 * dòng của `SESSION_SENT_FIELDS` — tham chiếu, không chép, để một câu khai không có hai bản trôi xa nhau.
 */
export const BRIDGE_FIELDS_WITH_PHONE: readonly TruongGuiDi[] = [
  ...VIGOV_BRIDGE_SENT_FIELDS,
  SESSION_SENT_FIELDS.find((t) => t.khoa === "phoneToken")!,
];

/** Thân mở lại. `communeConfirmed` vẫn là hằng `true`: xã là xã công dân đã xác nhận ở lần mở này. */
export function bridgeBodyWithPhone(req: BridgeRequestWithPhone): string {
  return JSON.stringify({
    accessToken: req.ma_truy_cap,
    communeHostHint: req.commune_domain,
    communeConfirmed: true,
    phoneToken: req.ma_so_dien_thoai,
  });
}

/* ────────────────────────────────────────────────────────────────────────────────────────────
 * THÂN THỨ TƯ — ĐĂNG NHẬP TỪ APP RIÊNG CỦA MỘT XÃ (`vihat-miniapp` 4114f00, 29/09/2026)
 *
 *   gửi : { "accessToken", "phoneToken", "appId": "<App ID của app đang chạy>" }
 *         — KHÔNG `communeHostHint`, KHÔNG `communeConfirmed`: `vihat-miniapp` nhận ra app riêng từ
 *         `appId`, và ViGov tra xã từ dòng `mini_app` của App ID ấy (chế độ riêng). App không tự đặt ra
 *         một tên miền gợi ý nào (sự cố 27/09, ADR 0047:271-277).
 *   nhận: 201 { "vigovSession": {...} } — cùng khuôn nhánh cầu, đọc bằng `readVigovBridgeResponse`
 *   lỗi : 400 thiếu `phoneToken` / yêu cầu hỏng · 401 mã Zalo hết hạn · 422 `appId` lạ, hoặc app chưa gắn
 *         xã / xã ngừng hoạt động (MỘT câu) · 429 · 502 không với tới Zalo · 503 cầu tắt / chưa lắp ráp
 *         (`vihat-miniapp` `internal/httpapi/sessions.go` switch `chonApp`, `sessions_vigov.go:54-58`)
 *
 * ⚠ `phoneToken` BẮT BUỘC Ở MỌI LẦN, và đó là cách máy chủ XÁC MINH App ID: đổi `phoneToken` bằng secret
 *   của đúng app ấy phải thành công (`vihat-miniapp` `internal/httpapi/app_zalo.go` điểm 2). Nên không có
 *   thân "chỉ accessToken" cho app riêng — mọi lần mở phiên đều đi sau lời giải thích và hộp thoại xin số.
 *
 * ⚠ `appId` CHỈ CHỌN SECRET, KHÔNG CẤP GÌ. Nó đọc từ môi trường Zalo lúc chạy (`readRuntimeAppId`); một
 *   `appId` sai thì lượt đổi số thất bại (401/502) hoặc máy chủ trả 422 — không bao giờ thành một phiên.
 *
 * ⚠ VẮNG `appId` LÀ APP CHUNG ở phía máy chủ — và thân này KHÔNG có `communeHostHint`, nên máy chủ sẽ
 *   phát một PHIẾU THƯƠNG MẠI, tiêu luôn số điện thoại vào CSDL thương mại. Vì vậy `app_id` rỗng không
 *   bao giờ tới hàm này: `vigov-bridge.ts` dừng trước khi xin mã nào.
 * ──────────────────────────────────────────────────────────────────────────────────────────── */

/** Yêu cầu đăng nhập từ app riêng: hai mã Zalo + App ID của app đang chạy. */
export type CommuneAppSessionRequest = MaDangNhap & {
  /** App ID của Mini App đang chạy, như môi trường Zalo báo. Chỉ chọn secret ở máy chủ. */
  readonly app_id: string;
};

/**
 * Ba trường thân này đưa ra khỏi máy — và câu khai từng trường, cho hồ sơ nộp Zalo. Hai dòng mã là dòng
 * của `SESSION_SENT_FIELDS` (tham chiếu, không chép); dòng `appId` là dòng mới của thân này.
 *
 * ⚠ CHÍNH SÁCH QUYỀN RIÊNG TƯ CHƯA CÓ CÂU CHO THÂN NÀY — lời văn pháp lý của chủ dự án, CÒN NỢ, cùng thế
 *   đứng với `bridgeBodyWithPhone`. `content/chinh-sach.test.ts` ghim khoảng hở ấy.
 */
export const COMMUNE_APP_SESSION_FIELDS: readonly TruongGuiDi[] = [
  SESSION_SENT_FIELDS.find((t) => t.khoa === "accessToken")!,
  SESSION_SENT_FIELDS.find((t) => t.khoa === "phoneToken")!,
  {
    khoa: "appId",
    trong_chinh_sach:
      "mã số của ứng dụng xã bạn đang dùng, do Zalo cấp cho ứng dụng — cho máy chủ biết bạn đang làm việc với xã nào, và KHÔNG chứa gì về bạn",
  },
];

/** Thân đăng nhập từ app riêng. CHỖ DUY NHẤT tên trường `appId` được viết ra. */
export function communeAppSessionBody(req: CommuneAppSessionRequest): string {
  return JSON.stringify({
    accessToken: req.ma_truy_cap,
    phoneToken: req.ma_so_dien_thoai,
    appId: req.app_id,
  });
}

/**
 * Phiên ViGov như cầu trả về, đã đổi sang tên của ta. Không có mã xã.
 *
 * `commune_domain` là `communePrimaryHost` — tên miền công khai chính của xã CỦA PHIÊN. Nó CHỈ làm khoá
 * tra `?host=` cho hai màn công khai (tin tức, danh bạ); nó không phải tham chiếu xã, không được gửi
 * đi làm "xã của tôi", không vẽ ra, không lưu (ADR 0047 điều kiện dừng #1). `null` khi máy chủ không
 * gửi, gửi `""` (phiên không xã, hoặc xã chưa có tên miền chính), hoặc gửi một chuỗi sai khuôn.
 */
export type BridgeVigovSession = {
  token: string;
  expires_at: string;
  commune_name: string;
  phone_verified: boolean;
  commune_domain: string | null;
};

/**
 * `communePrimaryHost` → tên miền đúng khuôn, hoặc `null`.
 *
 * TUỲ CHỌN VÀ KHÔNG LÀM HỎNG PHIÊN: trường này mới, và một máy chủ cũ không gửi nó. Sai khuôn thì chỉ
 * mất hai màn công khai — không mất phiên, vì gửi phản ánh không cần tên miền. Chữ thường hoá trước khi
 * kiểm, đúng cách `communeParams` đọc `d`: tên miền không phân biệt hoa thường, và khuôn `isDomain` thì có.
 */
function readSessionDomain(v: unknown): string | null {
  if (typeof v !== "string") return null;
  const domain = v.toLowerCase();
  return isDomain(domain) ? domain : null;
}

/**
 * Thân 201 → phiên ViGov, `"khong-co-phien"` khi máy chủ trả lời thật rằng không có phiên nào dùng
 * được, hoặc `null` khi sai khuôn.
 *
 *   • không có `vigovSession`           → cầu tắt (một bản máy chủ cũ trả phiếu thương mại)
 *   • `vigovSession` không có `token`    → phiên không xã; hợp đồng cố ý không phát bearer
 *   • `tenantDisplayName` rỗng           → "không xã nào" — không bao giờ thay bằng tên màn xác nhận
 *   • `communePrimaryHost` vắng/""/sai   → phiên vẫn dùng được; `commune_domain` = `null`
 */
export function readVigovBridgeResponse(body: unknown): BridgeVigovSession | "khong-co-phien" | null {
  if (typeof body !== "object" || body === null) return null;
  const { vigovSession } = body as Record<string, unknown>;
  if (vigovSession === undefined) return "khong-co-phien";
  if (typeof vigovSession !== "object" || vigovSession === null) return null;
  const v = vigovSession as Record<string, unknown>;
  if (typeof v.tenantDisplayName !== "string" || typeof v.phoneVerified !== "boolean") return null;
  if (v.token !== undefined && typeof v.token !== "string") return null;
  if (v.expiresAt !== undefined && typeof v.expiresAt !== "string") return null;
  const token = typeof v.token === "string" ? v.token : "";
  if (token === "" || v.tenantDisplayName.trim() === "") return "khong-co-phien";
  return {
    token,
    expires_at: typeof v.expiresAt === "string" ? v.expiresAt : "",
    commune_name: v.tenantDisplayName,
    phone_verified: v.phoneVerified,
    commune_domain: readSessionDomain(v.communePrimaryHost),
  };
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * LOCATION EXCHANGE — `POST /api/v1/location` (`vihat-miniapp` 0dada0f, `internal/httpapi/position.go`)
 *
 *   send   : { "accessToken": "<getAccessToken()>", "locationToken": "<token of getLocation()>" }
 *   receive: 200 { "latitude": number, "longitude": number }   (Cache-Control: no-store)
 *   errors : { "message", "code" } — 400 invalid_request · 405 method_not_allowed · 429 rate_limited
 *            (10 / 5 min / IP, its own bucket) · 502 zalo_location_unavailable (EVERY Zalo failure,
 *            expired token included) · 503 unavailable
 *
 * SAME SERVER AND SAME HOST as the login route: `diaChiApi`, one address for the app (the skill's
 * "The API host is singular"). Public on the server side, like `/sessions`: what stands in the way is
 * the Zalo-issued token plus the per-IP limit — so no ViGov session and no commercial ticket is sent.
 *
 * The server's `message` is NOT read: the screen says its own sentence per branch (the rule of
 * `citizen/api/vigov-client.ts`), and the branch is chosen by status code, which the contract fixes.
 *
 * ⚠ NOTHING IS KEPT. The two codes live for one call; the coordinates go back to the screen, which
 *   shows them and sends them only inside a petition the citizen submits. `vihat-miniapp` stores and
 *   logs neither (README §`POST /api/v1/location`).
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** Path of the location exchange. Carries nothing of the user (rule 3, forbidden #4). */
export const LOCATION_PATH = "/api/v1/location";

/** Full address of the route, or EMPTY when the build has no host — `server-calls.ts` fails closed. */
export function locationAddress(): string {
  return diaChiApi(LOCATION_PATH);
}

/**
 * The two fields this route takes off the phone — and the sentence declaring each, for the Zalo
 * submission (`content/ket-xuat-ho-so.ts`). Locked both ways with `locationBody` by
 * `ket-xuat-ho-so.test.ts`, exactly as the three login bodies are.
 */
export const LOCATION_FIELDS: readonly TruongGuiDi[] = [
  {
    khoa: "accessToken",
    trong_chinh_sach:
      "mã phiên Zalo của bạn, để máy chủ đổi được mã vị trí — mã này KHÔNG chứa tên hay ảnh đại diện của bạn",
  },
  {
    khoa: "locationToken",
    trong_chinh_sach:
      "mã vị trí do Zalo cấp sau khi bạn đồng ý chia sẻ — TOẠ ĐỘ KHÔNG NẰM TRONG MÃ NÀY, chỉ máy chủ đổi được mã thành toạ độ, và máy chủ không lưu toạ độ ấy",
  },
];

/**
 * The commune app's location body carries a THIRD key, `appId` (`vihat-miniapp` 4114f00, `position.go`): it
 * selects which app secret exchanges the token — the commune app's own. Without it the server uses the
 * shared app's secret and Zalo answers 502 for a commune app's token. Same sentence as the login body's
 * `appId` row — one declaration, referenced, not copied.
 */
export const COMMUNE_APP_LOCATION_FIELDS: readonly TruongGuiDi[] = [
  ...LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS.find((t) => t.khoa === "appId")!,
];

/**
 * The two Zalo codes (+ the commune app's App ID) → the request body. THE ONLY PLACE the wire names are
 * written. `app_id` null = the shared app: the body stays the two keys it always was.
 */
export function locationBody(codes: LocationCodes, app_id: string | null = null): string {
  return JSON.stringify({
    accessToken: codes.access_token,
    locationToken: codes.location_token,
    ...(app_id === null ? {} : { appId: app_id }),
  });
}

/** Coordinates as read from the server, in our names. */
export type ExchangedLocation = { latitude: number; longitude: number };

/**
 * 200 body → coordinates, or `null` when MALFORMED.
 *
 * Checked field by field, never cast: a missing key would become `undefined` and reach the petition as
 * `null` — a half location. Out of the world's range is malformed too (the server already refuses such
 * a reading, `vihat-miniapp` README table, 502), and ViGov answers 400 to it — better said here, as
 * "try again", than at the moment the citizen presses send.
 */
export function readLocation(body: unknown): ExchangedLocation | null {
  if (typeof body !== "object" || body === null) return null;
  const { latitude, longitude } = body as Record<string, unknown>;
  if (typeof latitude !== "number" || typeof longitude !== "number") return null;
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) return null;
  if (latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180) return null;
  return { latitude, longitude };
}

/**
 * HOST ĐỌC LÚC DỰNG, KHÔNG PHẢI LÚC CHẠY — nay ở `api/dia-chi.ts`, một chỗ cho cả hai hợp đồng.
 *
 * ⚠ BẢN NỘP CŨNG GỌI MÁY CHỦ, NÊN QUÊN BIẾN `VIGOV_API_HOST` LÀ NỘP MỘT NÚT ĐĂNG NHẬP KHÔNG
 * ĐĂNG NHẬP NỔI. `scripts/deploy.mjs` chặn đường đẩy khi biến chưa khai — chặn ở đó chứ không
 * ném lỗi lúc dựng, vì `npm test` và `npm run dev` phải chạy được trên một máy chưa có địa chỉ
 * máy chủ nào. Lý do đầy đủ của cách đọc ấy nằm trong `api/dia-chi.ts`.
 */
