/**
 * HÀM MỞ PHIÊN CÔNG DÂN ViGov MÀ `App.tsx` TIÊM VÀO NỬA NHÀ NƯỚC — dựng từ hai mảnh của nửa thương mại:
 * `xinMaTruyCap` (`features/tinh-nang/`, nơi DUY NHẤT được gọi `zmp-sdk`) và `openVigovSessionViaBridge`
 * (`server-calls.ts`, tệp gọi mạng của khối đăng nhập).
 *
 * VÌ SAO Ở ĐÂY: cả hai mảnh đều thuộc nửa thương mại, và nửa nhà nước không được nhập chúng
 * (`two-halves-boundary.test.ts` §3a). Tệp này chỉ ghép hai mảnh; việc chuyển kết quả sang kiểu của nửa
 * nhà nước là của `App.tsx` — lớp vỏ, nơi DUY NHẤT được biết cả hai nửa.
 *
 * ⚠ KHÔNG ĐỤNG KHỐI ĐĂNG NHẬP. Không đọc, không ghi `session-store.tsx`; không gọi `issueSession`. Phiếu
 *   thương mại và phiên ViGov là hai phiên do hai hệ thống ký (ADR 0032).
 */
// vi-name-ok: importing two EXISTING names (`MaDangNhap`, `xinMaDangNhap`) from zalo-api.ts — no new name
import { type KetQuaXin, type MaDangNhap, readRuntimeAppId, xinMaDangNhap, xinMaTruyCap } from "../tinh-nang/zalo-api";

import {
  type CommuneAppBridgeResult,
  type VigovBridgeResult,
  openVigovSessionViaBridge,
  openCommuneAppSessionCall,
  reopenViGovSessionWithPhone,
} from "./server-calls";
import type { BridgeRequestWithPhone, CommuneAppSessionRequest, VigovBridgeRequest } from "./contract";

/** Thêm hai nhánh của bước lấy mã Zalo vào các nhánh của lời gọi cầu. */
export type BridgeSessionOpenResult = VigovBridgeResult | { kind: "ngoai-zalo" } | { kind: "khong-lay-duoc-ma" };

/**
 * Lấy access token rồi gọi cầu. `get_code` / `call_bridge` chỉ để phép kiểm thay hai mảnh — mã sản phẩm
 * không truyền chúng.
 */
export async function openCitizenSessionViaBridge(
  commune_domain: string,
  get_code: () => Promise<KetQuaXin<string>> = xinMaTruyCap,
  call_bridge: (req: VigovBridgeRequest) => Promise<VigovBridgeResult> = openVigovSessionViaBridge,
): Promise<BridgeSessionOpenResult> {
  const code = await get_code();
  if (code.kieu === "ngoai-zalo") return { kind: "ngoai-zalo" };
  if (code.kieu !== "xong" || code.du_lieu === "") return { kind: "khong-lay-duoc-ma" };
  return call_bridge({ ma_truy_cap: code.du_lieu, commune_domain });
}

/** Thêm nhánh `tu-choi`: công dân bấm "Từ chối" trên hộp thoại xin số của Zalo. */
export type ReopenWithPhoneBridgeResult = BridgeSessionOpenResult | { kind: "tu-choi" };

/**
 * MỞ LẠI phiên công dân ViGov KÈM mã số điện thoại — chỉ chạy SAU cú bấm "Đồng ý chia sẻ số điện thoại"
 * (`citizen/screens/phone-verification.tsx`), khi ViGov trả 403 `chua_xac_thuc_so`.
 *
 * `xinMaDangNhap` lấy ĐÚNG hai mã theo đúng thứ tự `vihat-miniapp` cần (`sessions_vigov.go:37-38`: mã tài
 * khoản trước, số sau): `getAccessToken` không hỏi ai, nên hỏng thì hỏng trước khi hộp thoại xin số hiện
 * ra. Dùng lại hàm ấy thay vì viết lời gọi thứ hai.
 *
 * ⚠ TỪ CHỐI = KHÔNG GỌI CẦU. Mã số rỗng (câu trả lời của nền tảng ở môi trường phát triển) cũng không gọi:
 *   gửi một `phoneToken` rỗng là mở lại một phiên y hệt phiên cũ.
 *
 * ⚠ MÃ SỐ KHÔNG RA KHỎI HÀM NÀY trừ vào đúng một lời gọi cầu. Kết quả trả lên không mang nó.
 *
 * `get_code` / `call_bridge` chỉ để phép kiểm thay hai mảnh — mã sản phẩm không truyền chúng.
 */
export async function reopenCitizenSessionWithPhone(
  commune_domain: string,
  get_code: () => Promise<KetQuaXin<MaDangNhap>> = xinMaDangNhap,
  call_bridge: (req: BridgeRequestWithPhone) => Promise<VigovBridgeResult> = reopenViGovSessionWithPhone,
): Promise<ReopenWithPhoneBridgeResult> {
  const code = await get_code();
  if (code.kieu === "ngoai-zalo") return { kind: "ngoai-zalo" };
  if (code.kieu === "tu-choi") return { kind: "tu-choi" };
  if (code.kieu !== "xong" || code.du_lieu.ma_truy_cap === "" || code.du_lieu.ma_so_dien_thoai === "") {
    return { kind: "khong-lay-duoc-ma" };
  }
  return call_bridge({
    ma_truy_cap: code.du_lieu.ma_truy_cap,
    ma_so_dien_thoai: code.du_lieu.ma_so_dien_thoai,
    commune_domain,
  });
}

/** Thêm các nhánh của bước trước lời gọi: App ID không rõ, ngoài Zalo, từ chối, không lấy được mã. */
export type CommuneAppLoginResult =
  | CommuneAppBridgeResult
  | { kind: "khong-ro-app" }
  | { kind: "ngoai-zalo" }
  | { kind: "tu-choi" }
  | { kind: "khong-lay-duoc-ma" };

/**
 * MỞ PHIÊN CÔNG DÂN ViGov TỪ APP RIÊNG CỦA MỘT XÃ — App ID lúc chạy + `getAccessToken` + `getPhoneNumber`
 * → `vihat-miniapp` (`contract.ts` thân thứ tư). Dùng cho CẢ lần mở đầu (việc cá nhân đầu tiên) LẪN lần
 * mở lại khi ViGov trả 403 `chua_xac_thuc_so`: thân app riêng luôn mang số, nên hai việc là một lời gọi.
 *
 * CHỈ CHẠY SAU CÚ BẤM ĐỒNG Ý trên lời giải thích của nửa nhà nước (chính sách 3.3.4): hàm này bật hộp
 * thoại xin số của Zalo.
 *
 * THỨ TỰ LÀ THIẾT KẾ:
 *   1. App ID trước — không rõ thì DỪNG, không hộp thoại nào hiện ra. Thân thiếu `appId` là app chung ở
 *      máy chủ, và máy chủ sẽ tiêu số điện thoại vào một phiếu thương mại (ADR 0032).
 *   2. `xinMaDangNhap`: `getAccessToken` (không hỏi ai) rồi `getPhoneNumber` (hỏi) — hỏng thì hỏng trước
 *      khi người dân bị hỏi.
 *   3. Mã rỗng (câu trả lời của nền tảng ở môi trường phát triển) không gửi.
 *
 * ⚠ HAI MÃ VÀ APP ID SỐNG ĐÚNG MỘT LỜI GỌI. Kết quả trả lên không mang chúng. Không log.
 *
 * `readAppId` / `requestCodes` / `call` chỉ để phép kiểm thay ba mảnh — mã sản phẩm không truyền chúng.
 */
export async function openCommuneAppSessionWithPhone(
  readAppId: () => string | null = readRuntimeAppId,
  requestCodes: () => Promise<KetQuaXin<MaDangNhap>> = xinMaDangNhap,
  call: (req: CommuneAppSessionRequest) => Promise<CommuneAppBridgeResult> = openCommuneAppSessionCall,
): Promise<CommuneAppLoginResult> {
  const app_id = readAppId();
  if (app_id === null || app_id === "") return { kind: "khong-ro-app" };
  const codes = await requestCodes();
  if (codes.kieu === "ngoai-zalo") return { kind: "ngoai-zalo" };
  if (codes.kieu === "tu-choi") return { kind: "tu-choi" };
  if (codes.kieu !== "xong" || codes.du_lieu.ma_truy_cap === "" || codes.du_lieu.ma_so_dien_thoai === "") {
    return { kind: "khong-lay-duoc-ma" };
  }
  return call({
    ma_truy_cap: codes.du_lieu.ma_truy_cap,
    ma_so_dien_thoai: codes.du_lieu.ma_so_dien_thoai,
    app_id,
  });
}
