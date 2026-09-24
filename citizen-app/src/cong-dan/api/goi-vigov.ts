/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC ĐƯỢC GỌI MẠNG — và nó chỉ nói chuyện với ViGov.
 *
 * `phase1-collects-nothing.test.ts` miễn lệnh cấm `fetch` cho ĐÚNG những tệp được kê tên; tệp này
 * là tệp thứ ba (24/09/2026). `hop-dong-phan-anh.ts` nằm ngay cạnh và vẫn bị cấm.
 *
 * ⚠ HAI CỔNG ĐÓNG ĐỨNG TRƯỚC MỌI LỜI GỌI, và cả hai đều đóng hôm nay:
 *
 *   1. `layPhienViGov()` trả `null`  → `chua-co-phien`, KHÔNG gọi mạng. Cầu phiên chưa có
 *      (`phien-vigov.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`).
 *   2. Địa chỉ ViGov rỗng            → `chua-cau-hinh`, KHÔNG gọi mạng (`dia-chi-vigov.ts`).
 *
 * ⚠ BEARER CHỈ ĐẾN TỪ `layPhienViGov()`. Không hàm nào ở đây nhận token qua tham số — một tham số
 * là một khe để ai đó nhét phiếu phiên của `vihat-miniapp` vào, và phiếu ấy KHÔNG phải phiên ViGov.
 *
 * ⚠ KHÔNG `console.*`, KHÔNG IN THÂN YÊU CẦU HAY PHẢN HỒI. Chúng mang nội dung phản ánh, họ tên và
 * số điện thoại của người thật (luật 3, bất biến 1).
 */
import { docPhieu, diaChiGuiPhanAnh, diaChiTraCuu, type PhieuCuaToi } from "./hop-dong-phan-anh";
import type { LanGui } from "./lan-gui";
import { layPhienViGov } from "./phien-vigov";

/**
 * Mỗi nhánh là một VIỆC NGƯỜI DÂN PHẢI LÀM khác nhau.
 *
 *   `chua-co-phien` · `chua-cau-hinh`  kênh chưa mở trên ứng dụng — không gửi gì đi cả
 *   `xong`                              201 / 200, có phiếu
 *   `khong-thay`                        404 — MỘT câu cho "không có", "của người khác", "xã khác"
 *   `het-phien`                         401 — phiên không dùng được
 *   `dang-xu-ly-truoc`                  409 — lần gửi trước (cùng khoá) còn đang chạy; chờ rồi gửi lại
 *   `khong-hop-le`                      400
 *   `kenh-chua-mo`                      503 — xã chưa cấu hình hạn; phiếu CHƯA được ghi nhận
 *   `loi-may-chu`                       500, mã lạ, hoặc thân sai khuôn
 *   `loi-mang`                          mất mạng, quá hạn chờ
 *
 * KHÔNG MANG CÂU CỦA MÁY CHỦ: câu 400 của `petitions` nói bằng tên trường kỹ thuật (`content`), và
 * màn hình người dân có câu riêng cho từng nhánh (`man/noi-dung.ts`).
 */
export type KetQuaGoi =
  | { kieu: "chua-co-phien" }
  | { kieu: "chua-cau-hinh" }
  | { kieu: "xong"; phieu: PhieuCuaToi }
  | { kieu: "khong-thay" }
  | { kieu: "het-phien" }
  | { kieu: "dang-xu-ly-truoc" }
  | { kieu: "khong-hop-le" }
  | { kieu: "kenh-chua-mo" }
  | { kieu: "loi-may-chu" }
  | { kieu: "loi-mang" };

/** Quá hạn chờ. Người dân đang cầm máy đứng chờ; thà nói "thử lại" sớm. */
const HAN_CHO_MS = 20_000;

/** Hai cổng đóng. Trả về phiên + địa chỉ, hoặc nhánh dừng. */
function moCong(dia_chi: string): { token: string; dia_chi: string } | KetQuaGoi {
  const phien = layPhienViGov();
  if (phien === null || phien.token === "") return { kieu: "chua-co-phien" };
  if (dia_chi === "") return { kieu: "chua-cau-hinh" };
  return { token: phien.token, dia_chi };
}

async function goi(
  dia_chi: string,
  tuy_chon: { method: "GET" | "POST"; token: string; khoa?: string; than?: string },
  khi_404: KetQuaGoi,
): Promise<KetQuaGoi> {
  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  const tieu_de: Record<string, string> = {
    Accept: "application/json",
    Authorization: `Bearer ${tuy_chon.token}`,
  };
  if (tuy_chon.than !== undefined) tieu_de["Content-Type"] = "application/json";
  if (tuy_chon.khoa !== undefined) tieu_de["Idempotency-Key"] = tuy_chon.khoa;

  try {
    const tra_loi = await fetch(dia_chi, {
      method: tuy_chon.method,
      headers: tieu_de,
      body: tuy_chon.than,
      signal: bo_dieu_khien.signal,
    });

    switch (tra_loi.status) {
      case 200:
      case 201: {
        const phieu = docPhieu(await tra_loi.json());
        return phieu === null ? { kieu: "loi-may-chu" } : { kieu: "xong", phieu };
      }
      case 400:
        return { kieu: "khong-hop-le" };
      case 401:
        return { kieu: "het-phien" };
      case 404:
        return khi_404;
      case 409:
        return { kieu: "dang-xu-ly-truoc" };
      case 503:
        return { kieu: "kenh-chua-mo" };
      default:
        return { kieu: "loi-may-chu" };
    }
  } catch {
    return { kieu: "loi-mang" };
  } finally {
    clearTimeout(dong_ho);
  }
}

/**
 * Gửi MỘT lần gửi. Gọi lại với CÙNG `lan` là "gửi lại" — cùng thân, cùng `Idempotency-Key`.
 * Xem `lan-gui.ts`. Không ném ra ngoài.
 */
export async function guiPhanAnh(lan: LanGui): Promise<KetQuaGoi> {
  const cong = moCong(diaChiGuiPhanAnh());
  if ("kieu" in cong) return cong;
  return goi(
    cong.dia_chi,
    { method: "POST", token: cong.token, khoa: lan.khoa, than: lan.than },
    // Tuyến gửi không có 404 trong hợp đồng; gặp nó là tuyến chưa được định tuyến ở cụm.
    { kieu: "loi-may-chu" },
  );
}

/**
 * Tra MỘT phiếu của chính mình theo mã tra cứu.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI": máy chủ lấy người gửi từ phiên (luật 4, cấm #1). Mã rỗng thì trả
 * `khong-thay` mà không gọi — một đoạn đường dẫn rỗng không khớp phiếu nào.
 */
export async function traCuuPhieu(ma_tra_cuu: string): Promise<KetQuaGoi> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(diaChiTraCuu(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  return goi(cong.dia_chi, { method: "GET", token: cong.token }, { kieu: "khong-thay" });
}
