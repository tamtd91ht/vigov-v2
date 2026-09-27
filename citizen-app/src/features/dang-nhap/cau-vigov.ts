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
import { type KetQuaXin, xinMaTruyCap } from "../tinh-nang/zalo-api";

import { type KetQuaCauViGov, moPhienViGovQuaCau } from "./goi-may-chu";
import type { YeuCauCauViGov } from "./hop-dong";

/** Thêm hai nhánh của bước lấy mã Zalo vào các nhánh của lời gọi cầu. */
export type KetQuaMoPhienQuaCau = KetQuaCauViGov | { kieu: "ngoai-zalo" } | { kieu: "khong-lay-duoc-ma" };

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
  if (ma.kieu !== "xong" || ma.du_lieu === "") return { kieu: "khong-lay-duoc-ma" };
  return goi_cau({ ma_truy_cap: ma.du_lieu, ten_mien_xa });
}
