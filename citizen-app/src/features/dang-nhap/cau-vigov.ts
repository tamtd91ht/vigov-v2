/**
 * HÀM MỞ PHIÊN CÔNG DÂN ViGov MÀ `App.tsx` TIÊM VÀO NỬA NHÀ NƯỚC — dựng từ hai mảnh của nửa thương mại:
 * `xinMaTruyCap` (`features/tinh-nang/`, nơi DUY NHẤT được gọi `zmp-sdk`) và `moPhienViGovQuaCau`
 * (`goi-may-chu.ts`, tệp gọi mạng của khối đăng nhập).
 *
 * VÌ SAO Ở ĐÂY: cả hai mảnh đều thuộc nửa thương mại, và nửa nhà nước không được nhập chúng
 * (`ranh-gioi-hai-nua.test.ts` §3a). Tệp này chỉ ghép hai mảnh; việc chuyển kết quả sang kiểu của nửa
 * nhà nước là của `App.tsx` — lớp vỏ, nơi DUY NHẤT được biết cả hai nửa.
 *
 * ⚠ KHÔNG ĐỤNG KHỐI ĐĂNG NHẬP. Không đọc, không ghi `kho-phien.tsx`; không gọi `phatHanhPhien`. Phiếu
 *   thương mại và phiên ViGov là hai phiên do hai hệ thống ký (ADR 0032).
 */
// vi-name-ok: importing two EXISTING names (`MaDangNhap`, `xinMaDangNhap`) from zalo-api.ts — no new name
import {
  type KetQuaXin,
  type MaDangNhap,
  readRuntimeAppId,
  type SdkFailure,
  xinMaDangNhap,
  xinMaTruyCap,
} from "../tinh-nang/zalo-api";

import {
  type CommuneAppBridgeResult,
  type KetQuaCauViGov,
  moPhienViGovQuaCau,
  openCommuneAppSessionCall,
  reopenViGovSessionWithPhone,
} from "./goi-may-chu";
import type {
  BridgeRequestWithPhone,
  CommuneAppSessionRequest,
  YeuCauCauViGov,
} from "./hop-dong";
import { communeAppSessionAddress } from "./hop-dong";

/**
 * No code from Zalo. `failure` is what the SDK answered (capability + code) when it threw one; absent
 * when there was nothing to measure — an empty code (the platform's answer in a development environment)
 * or a throw without a code. Never a code of our own invention.
 */
export type NoCodeResult = { kieu: "khong-lay-duoc-ma"; failure?: SdkFailure };

/** The measured failure of a Zalo step, or nothing — so a caller cannot copy a field that is not there. */
export function noCode(result: KetQuaXin<unknown>): NoCodeResult {
  return result.kieu === "khong-lay-duoc" && result.failure !== undefined
    ? { kieu: "khong-lay-duoc-ma", failure: result.failure }
    : { kieu: "khong-lay-duoc-ma" };
}

/** Thêm hai nhánh của bước lấy mã Zalo vào các nhánh của lời gọi cầu. */
export type KetQuaMoPhienQuaCau = KetQuaCauViGov | { kieu: "ngoai-zalo" } | NoCodeResult;

/**
 * Lấy access token rồi gọi cầu. `lay_ma` / `goi_cau` chỉ để phép kiểm thay hai mảnh — mã sản phẩm
 * không truyền chúng.
 */
export async function moPhienCongDanQuaCau(
  ten_mien_xa: string,
  lay_ma: () => Promise<KetQuaXin<string>> = xinMaTruyCap,
  goi_cau: (yc: YeuCauCauViGov) => Promise<KetQuaCauViGov> = moPhienViGovQuaCau,
): Promise<KetQuaMoPhienQuaCau> {
  const ma = await lay_ma();
  if (ma.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (ma.kieu !== "xong" || ma.du_lieu === "") return noCode(ma);
  return goi_cau({ ma_truy_cap: ma.du_lieu, ten_mien_xa });
}

/** Thêm nhánh `tu-choi`: công dân bấm "Từ chối" trên hộp thoại xin số của Zalo. */
export type ReopenWithPhoneBridgeResult = KetQuaMoPhienQuaCau | { kieu: "tu-choi" };

/**
 * MỞ LẠI phiên công dân ViGov KÈM mã số điện thoại — chỉ chạy SAU cú bấm "Đồng ý chia sẻ số điện thoại"
 * (`cong-dan/man/phone-verification.tsx`), khi ViGov trả 403 `chua_xac_thuc_so`.
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
 * `lay_ma` / `goi_cau` chỉ để phép kiểm thay hai mảnh — mã sản phẩm không truyền chúng.
 */
export async function reopenCitizenSessionWithPhone(
  ten_mien_xa: string,
  lay_ma: () => Promise<KetQuaXin<MaDangNhap>> = xinMaDangNhap,
  goi_cau: (yc: BridgeRequestWithPhone) => Promise<KetQuaCauViGov> = reopenViGovSessionWithPhone,
): Promise<ReopenWithPhoneBridgeResult> {
  const ma = await lay_ma();
  if (ma.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (ma.kieu === "tu-choi") return { kieu: "tu-choi" };
  if (ma.kieu !== "xong" || ma.du_lieu.ma_truy_cap === "" || ma.du_lieu.ma_so_dien_thoai === "") {
    return noCode(ma);
  }
  return goi_cau({
    ma_truy_cap: ma.du_lieu.ma_truy_cap,
    ma_so_dien_thoai: ma.du_lieu.ma_so_dien_thoai,
    ten_mien_xa,
  });
}

/** Thêm các nhánh của bước trước lời gọi: App ID không rõ, ngoài Zalo, từ chối, không lấy được mã. */
export type CommuneAppLoginResult =
  | CommuneAppBridgeResult
  | { kieu: "khong-ro-app" }
  | { kieu: "ngoai-zalo" }
  | { kieu: "tu-choi" }
  | NoCodeResult;

/**
 * MỞ PHIÊN CÔNG DÂN ViGov TỪ APP RIÊNG CỦA MỘT XÃ — App ID lúc chạy + `getAccessToken` + `getPhoneNumber`
 * → ViGov identity `POST /api/v1/citizen-sessions` (`hop-dong.ts` thân thứ tư; ADR 0066 — no longer
 * `vihat-miniapp`). Dùng cho CẢ lần mở đầu (việc cá nhân đầu tiên) LẪN lần mở lại khi ViGov trả 403
 * `chua_xac_thuc_so`.
 *
 * ⚠ 401 AFTER SENDING `phoneToken` → ONE RETRY WITHOUT IT (ADR 0080, 08/10/2026). Until Zalo approves the
 *   commune's app, `getPhoneNumber` still hands out a token that identity cannot exchange, so the call with it
 *   is refused (`zalo_token_invalid`). The server deliberately does not downgrade (`hop-dong.ts`); the retry
 *   reuses the access token already taken — no SDK call, no dialog — and its answer is final: a second 401
 *   means the access token itself was refused, and that goes up as `ma-het-han`. Never a loop. A phone-less
 *   session comes back as `xong` with `da_xac_thuc_so: false`; the gate decides what it may be used for.
 *
 * ⚠ REFUSAL / NO PHONE CODE STILL CALL NOTHING HERE. The citizen then chooses "Gửi bằng họ tên và số điện
 *   thoại" on the gate, and only that tap sends the phone-less body (`openCommuneAppSessionWithoutPhone`): a
 *   session is opened by the citizen's act, never as a side effect of declining Zalo's dialog.
 *
 * CHỈ CHẠY SAU CÚ BẤM ĐỒNG Ý trên lời giải thích của nửa nhà nước (chính sách 3.3.4): hàm này bật hộp
 * thoại xin số của Zalo.
 *
 * `identityHost` comes from the state half's address map, through the injected opener (`App.tsx`,
 * `cong-dan/api/mo-phien-vigov.ts`) — see `communeAppSessionAddress` for why it is a parameter.
 *
 * THỨ TỰ LÀ THIẾT KẾ:
 *   0. No address → STOP before anything: asking for the phone with nowhere to send it is a consent spent
 *      for nothing.
 *   1. App ID trước — không rõ thì DỪNG, không hộp thoại nào hiện ra. The server picks the app secret
 *      (and so the commune) from `appId`; without it there is nothing to verify the codes against.
 *   2. `xinMaDangNhap`: `getAccessToken` (không hỏi ai) rồi `getPhoneNumber` (hỏi) — hỏng thì hỏng trước
 *      khi người dân bị hỏi.
 *   3. Mã rỗng (câu trả lời của nền tảng ở môi trường phát triển) không gửi.
 *
 * ⚠ HAI MÃ VÀ APP ID SỐNG ĐÚNG MỘT LỜI GỌI. Kết quả trả lên không mang chúng. Không log.
 *
 * `readAppId` / `requestCodes` / `call` chỉ để phép kiểm thay ba mảnh — mã sản phẩm không truyền chúng.
 */
export async function openCommuneAppSessionWithPhone(
  identityHost: string,
  readAppId: () => string | null = readRuntimeAppId,
  requestCodes: () => Promise<KetQuaXin<MaDangNhap>> = xinMaDangNhap,
  call: (req: CommuneAppSessionRequest, address: string) => Promise<CommuneAppBridgeResult> = openCommuneAppSessionCall,
): Promise<CommuneAppLoginResult> {
  const address = communeAppSessionAddress(identityHost);
  if (address === "") return { kieu: "chua-khai-host" };
  const app_id = readAppId();
  if (app_id === null || app_id === "") return { kieu: "khong-ro-app" };
  const codes = await requestCodes();
  if (codes.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (codes.kieu === "tu-choi") return { kieu: "tu-choi" };
  if (codes.kieu !== "xong" || codes.du_lieu.ma_truy_cap === "" || codes.du_lieu.ma_so_dien_thoai === "") {
    return noCode(codes);
  }
  const accessToken = codes.du_lieu.ma_truy_cap;
  const withPhone = await call(
    { ma_truy_cap: accessToken, ma_so_dien_thoai: codes.du_lieu.ma_so_dien_thoai, app_id },
    address,
  );
  if (withPhone.kieu !== "ma-het-han") return withPhone;
  // Exactly one retry, without the phone code (see the header). Its answer is returned whatever it is.
  return call({ ma_truy_cap: accessToken, ma_so_dien_thoai: "", app_id }, address);
}

/**
 * THE COMMUNE APP'S PHONE-LESS SESSION (ADR 0080 #1, #6) — App ID + `getAccessToken` only, no `phoneToken`, so
 * Zalo shows NO dialog. Called only after the citizen tapped "Gửi bằng họ tên và số điện thoại" on the gate,
 * which is offered only when Zalo gave no number (refused, or failed). Same order and same stops as above:
 * no address or no App ID → nothing asked, nothing sent.
 *
 * `readAppId` / `requestAccess` / `call` are for tests only — production code never passes them.
 */
export async function openCommuneAppSessionWithoutPhone(
  identityHost: string,
  readAppId: () => string | null = readRuntimeAppId,
  requestAccess: () => Promise<KetQuaXin<string>> = xinMaTruyCap,
  call: (req: CommuneAppSessionRequest, address: string) => Promise<CommuneAppBridgeResult> = openCommuneAppSessionCall,
): Promise<CommuneAppLoginResult> {
  const address = communeAppSessionAddress(identityHost);
  if (address === "") return { kieu: "chua-khai-host" };
  const app_id = readAppId();
  if (app_id === null || app_id === "") return { kieu: "khong-ro-app" };
  const access = await requestAccess();
  if (access.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (access.kieu !== "xong" || access.du_lieu === "") return noCode(access);
  return call({ ma_truy_cap: access.du_lieu, ma_so_dien_thoai: "", app_id }, address);
}
