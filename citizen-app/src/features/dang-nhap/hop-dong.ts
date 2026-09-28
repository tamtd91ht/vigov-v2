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
import type { TruongGuiDi } from "../../api/hop-dong-yeu-cau";

import { laTenMien } from "../../lib/launch-params";

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
 * ⚠ KHOÁ HAI CHIỀU với `thanYeuCau` ngay dưới, đúng cơ chế của chính sách quyền riêng tư: mọi
 * khoá hàm ấy sinh ra phải có một dòng ở đây, và mọi dòng ở đây phải là một khoá hàm ấy sinh ra.
 * `ket-xuat-ho-so.test.ts` giữ cả hai chiều.
 */
export const TRUONG_GUI_DI_PHIEN: readonly TruongGuiDi[] = [
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
 * ⚠ HAI PHIÊN, HAI KHOÁ, KHÔNG BAO GIỜ LẪN. Phiếu thương mại nằm ở `token` GỐC thân trả lời (`docTraLoi`
 *   ở trên); phiên ViGov nằm dưới `vigovSession`. `docTraLoiCauViGov` KHÔNG đọc `token` gốc — cầm nhầm
 *   phiếu thương mại rồi gửi tới ViGov là đúng thứ ADR 0032 cấm. Và `docTraLoi` không đọc `vigovSession`:
 *   hành vi của khối đăng nhập (Tư vấn · Yêu cầu của tôi) giữ nguyên.
 *
 * ⚠ CẦU TẮT (hôm nay, ADR 0045 UNKNOWN #2): nhánh cũ của máy chủ đòi `phoneToken` và trả 400 cho thân
 *   này. Đó là đường BÌNH THƯỜNG hôm nay, không phải lỗi nối dây — `goi-may-chu.ts` đọc nó là "cầu tắt".
 *
 * ⚠ TÊN MIỀN XÃ LÀ GỢI Ý, KHÔNG PHẢI THAM CHIẾU XÃ: máy chủ ViGov phân giải nó lúc xác nhận (ADR 0047
 *   câu 3). Nó rời khỏi máy trong thân này — nói ra ở `TRUONG_GUI_DI_CAU_VIGOV` ngay dưới.
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type YeuCauCauViGov = {
  /** Access token của phiên Zalo (`getAccessToken`). */
  ma_truy_cap: string;
  /** Tên miền xã công dân vừa xác nhận trên màn xác nhận. */
  ten_mien_xa: string;
};

/**
 * Ba trường nhánh cầu đưa ra khỏi máy — và câu khai từng trường.
 *
 * ⚠ KHOÁ BA CHIỀU (27/09/2026): mọi khoá `thanYeuCauCauViGov` sinh ra có một dòng ở đây và ngược lại
 * (`cau-vigov.test.ts`); mọi `trong_chinh_sach` có mặt NGUYÊN VĂN trong mục Đăng nhập của chính sách
 * (`content/chinh-sach.test.ts`); và `content/ket-xuat-ho-so.ts` in bảng này vào khối "Những gì rời
 * khỏi máy" của hồ sơ nộp Zalo. Câu trong chính sách là CHUỖI VIẾT SẴN, không ghép lúc chạy:
 * `bundle-for-zalo.test.ts` đòi từng đoạn của mục Đăng nhập có mặt nguyên văn trong bundle.
 *
 * Mỗi câu khai viết để ĐỨNG TRONG MỘT DANH SÁCH cách nhau bằng `;` — không tự mang dấu `;`.
 */
export const TRUONG_GUI_DI_CAU_VIGOV: readonly TruongGuiDi[] = [
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
export function thanYeuCauCauViGov(yc: YeuCauCauViGov): string {
  return JSON.stringify({
    accessToken: yc.ma_truy_cap,
    communeHostHint: yc.ten_mien_xa,
    communeConfirmed: true,
  });
}

/* ────────────────────────────────────────────────────────────────────────────────────────────
 * THÂN THỨ HAI CỦA NHÁNH CẦU — MỞ LẠI PHIÊN KÈM `phoneToken` (28/09/2026, quyết định của người dùng)
 *
 *   gửi : { "accessToken", "communeHostHint", "communeConfirmed": true, "phoneToken" }
 *
 * CHỈ chạy khi một tuyến phản ánh của ViGov trả 403 `chua_xac_thuc_so` VÀ công dân tự bấm "Đồng ý chia
 * sẻ số điện thoại" rồi đồng ý trên hộp thoại của Zalo (`cong-dan/man/phone-verification.tsx`). Đúng
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
export type BridgeRequestWithPhone = YeuCauCauViGov & Pick<MaDangNhap, "ma_so_dien_thoai">;

/**
 * Bốn trường thân mở lại đưa ra khỏi máy. Ba dòng đầu LÀ `TRUONG_GUI_DI_CAU_VIGOV`; dòng `phoneToken` LÀ
 * dòng của `TRUONG_GUI_DI_PHIEN` — tham chiếu, không chép, để một câu khai không có hai bản trôi xa nhau.
 */
export const BRIDGE_FIELDS_WITH_PHONE: readonly TruongGuiDi[] = [
  ...TRUONG_GUI_DI_CAU_VIGOV,
  TRUONG_GUI_DI_PHIEN.find((t) => t.khoa === "phoneToken")!,
];

/** Thân mở lại. `communeConfirmed` vẫn là hằng `true`: xã là xã công dân đã xác nhận ở lần mở này. */
export function bridgeBodyWithPhone(yc: BridgeRequestWithPhone): string {
  return JSON.stringify({
    accessToken: yc.ma_truy_cap,
    communeHostHint: yc.ten_mien_xa,
    communeConfirmed: true,
    phoneToken: yc.ma_so_dien_thoai,
  });
}

/**
 * Phiên ViGov như cầu trả về, đã đổi sang tên của ta. Không có mã xã.
 *
 * `ten_mien_xa` là `communePrimaryHost` — tên miền công khai chính của xã CỦA PHIÊN. Nó CHỈ làm khoá
 * tra `?host=` cho hai màn công khai (tin tức, danh bạ); nó không phải tham chiếu xã, không được gửi
 * đi làm "xã của tôi", không vẽ ra, không lưu (ADR 0047 điều kiện dừng #1). `null` khi máy chủ không
 * gửi, gửi `""` (phiên không xã, hoặc xã chưa có tên miền chính), hoặc gửi một chuỗi sai khuôn.
 */
export type PhienViGovQuaCau = {
  token: string;
  het_han: string;
  ten_xa: string;
  da_xac_thuc_so: boolean;
  ten_mien_xa: string | null;
};

/**
 * `communePrimaryHost` → tên miền đúng khuôn, hoặc `null`.
 *
 * TUỲ CHỌN VÀ KHÔNG LÀM HỎNG PHIÊN: trường này mới, và một máy chủ cũ không gửi nó. Sai khuôn thì chỉ
 * mất hai màn công khai — không mất phiên, vì gửi phản ánh không cần tên miền. Chữ thường hoá trước khi
 * kiểm, đúng cách `thamSoXa` đọc `d`: tên miền không phân biệt hoa thường, và khuôn `laTenMien` thì có.
 */
function docTenMienPhien(v: unknown): string | null {
  if (typeof v !== "string") return null;
  const ten_mien = v.toLowerCase();
  return laTenMien(ten_mien) ? ten_mien : null;
}

/**
 * Thân 201 → phiên ViGov, `"khong-co-phien"` khi máy chủ trả lời thật rằng không có phiên nào dùng
 * được, hoặc `null` khi sai khuôn.
 *
 *   • không có `vigovSession`           → cầu tắt (một bản máy chủ cũ trả phiếu thương mại)
 *   • `vigovSession` không có `token`    → phiên không xã; hợp đồng cố ý không phát bearer
 *   • `tenantDisplayName` rỗng           → "không xã nào" — không bao giờ thay bằng tên màn xác nhận
 *   • `communePrimaryHost` vắng/""/sai   → phiên vẫn dùng được; `ten_mien_xa` = `null`
 */
export function docTraLoiCauViGov(than: unknown): PhienViGovQuaCau | "khong-co-phien" | null {
  if (typeof than !== "object" || than === null) return null;
  const { vigovSession } = than as Record<string, unknown>;
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
    het_han: typeof v.expiresAt === "string" ? v.expiresAt : "",
    ten_xa: v.tenantDisplayName,
    da_xac_thuc_so: v.phoneVerified,
    ten_mien_xa: docTenMienPhien(v.communePrimaryHost),
  };
}

/**
 * HOST ĐỌC LÚC DỰNG, KHÔNG PHẢI LÚC CHẠY — nay ở `api/dia-chi.ts`, một chỗ cho cả hai hợp đồng.
 *
 * ⚠ BẢN NỘP CŨNG GỌI MÁY CHỦ, NÊN QUÊN BIẾN `VIGOV_API_HOST` LÀ NỘP MỘT NÚT ĐĂNG NHẬP KHÔNG
 * ĐĂNG NHẬP NỔI. `scripts/deploy.mjs` chặn đường đẩy khi biến chưa khai — chặn ở đó chứ không
 * ném lỗi lúc dựng, vì `npm test` và `npm run dev` phải chạy được trên một máy chưa có địa chỉ
 * máy chủ nào. Lý do đầy đủ của cách đọc ấy nằm trong `api/dia-chi.ts`.
 */
