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
import type { BridgeRequestWithPhone, CommuneAppSessionRequest, YeuCauCauViGov } from "./hop-dong";

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
 * → `vihat-miniapp` (`hop-dong.ts` thân thứ tư). Dùng cho CẢ lần mở đầu (việc cá nhân đầu tiên) LẪN lần
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
  if (app_id === null || app_id === "") return { kieu: "khong-ro-app" };
  const codes = await requestCodes();
  if (codes.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (codes.kieu === "tu-choi") return { kieu: "tu-choi" };
  if (codes.kieu !== "xong" || codes.du_lieu.ma_truy_cap === "" || codes.du_lieu.ma_so_dien_thoai === "") {
    return noCode(codes);
  }
  return call({
    ma_truy_cap: codes.du_lieu.ma_truy_cap,
    ma_so_dien_thoai: codes.du_lieu.ma_so_dien_thoai,
    app_id,
  });
}
