/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC ĐƯỢC GỌI MẠNG — và nó chỉ nói chuyện với ViGov.
 *
 * `phase1-collects-nothing.test.ts` miễn lệnh cấm `fetch` cho ĐÚNG những tệp được kê tên; tệp này
 * là tệp thứ ba (24/09/2026). `hop-dong-phan-anh.ts` nằm ngay cạnh và vẫn bị cấm.
 *
 * ⚠ HAI CỔNG ĐỨNG TRƯỚC MỌI LỜI GỌI, và cổng PHIÊN mở trước — hôm nay cổng ấy đóng:
 *
 *   1. `layPhienViGov()` trả `null`  → `chua-co-phien`, KHÔNG gọi mạng. Cầu phiên chưa có
 *      (`phien-vigov.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`).
 *   2. Địa chỉ ViGov rỗng            → `chua-cau-hinh`, KHÔNG gọi mạng (`dia-chi-vigov.ts`).
 *      Từ 26/09/2026 host của `petitions` đã có (ADR 0046), nên cổng này MỞ; chỉ cổng 1 còn giữ
 *      mọi yêu cầu lại. Thứ tự hai cổng vì thế là bất biến: có host mà không có phiên thì vẫn
 *      không gửi gì.
 *
 * ⚠ BEARER CHỈ ĐẾN TỪ `layPhienViGov()`. Không hàm nào ở đây nhận token qua tham số — một tham số
 * là một khe để ai đó nhét phiếu phiên của `vihat-miniapp` vào, và phiếu ấy KHÔNG phải phiên ViGov.
 *
 * ⚠ KHÔNG `console.*`, KHÔNG IN THÂN YÊU CẦU HAY PHẢN HỒI. Chúng mang nội dung phản ánh, họ tên và
 * số điện thoại của người thật (luật 3, bất biến 1).
 */
import {
  diaChiDanhSach,
  diaChiGuiPhanAnh,
  diaChiTraCuu,
  docPhieu,
  docTrangPhieuCuaToi,
  type PhieuCuaToi,
  type TrangPhieuCuaToi,
} from "./hop-dong-phan-anh";
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

/** Mọi nhánh trừ `xong` — chung cho mọi tuyến. */
type NhanhKhongThanh = Exclude<KetQuaGoi, { kieu: "xong" }>;

/**
 * Kết quả của tuyến "Phản ánh của tôi". Cùng các nhánh dừng với `KetQuaGoi`; hợp đồng không có
 * 404/409/503 cho tuyến này, nên gặp chúng là `loi-may-chu` (xem `phanAnhCuaToi`).
 */
export type KetQuaDanhSach = { kieu: "xong"; trang: TrangPhieuCuaToi } | NhanhKhongThanh;

/** Quá hạn chờ. Người dân đang cầm máy đứng chờ; thà nói "thử lại" sớm. */
const HAN_CHO_MS = 20_000;

/** Hai cổng đóng. Trả về phiên + địa chỉ, hoặc nhánh dừng. */
function moCong(dia_chi: string): { token: string; dia_chi: string } | NhanhKhongThanh {
  const phien = layPhienViGov();
  if (phien === null || phien.token === "") return { kieu: "chua-co-phien" };
  if (dia_chi === "") return { kieu: "chua-cau-hinh" };
  return { token: phien.token, dia_chi };
}

/**
 * CHỖ DUY NHẤT GỌI `fetch` — mọi tuyến đi qua đây (`bundle-for-zalo.test.ts` đếm đúng một `fetch(`
 * của nửa này). `doc` đọc thân 200/201; `null` là sai khuôn.
 */
async function goi<T>(
  dia_chi: string,
  tuy_chon: { method: "GET" | "POST"; token: string; khoa?: string; than?: string },
  doc: (than: unknown) => T | null,
  khi_404: NhanhKhongThanh,
): Promise<{ kieu: "xong"; gia_tri: T } | NhanhKhongThanh> {
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
        const gia_tri = doc(await tra_loi.json());
        return gia_tri === null ? { kieu: "loi-may-chu" } : { kieu: "xong", gia_tri };
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
/** `gia_tri` của lớp gọi → nhánh `xong` của một phiếu. */
function thanhPhieu(kq: { kieu: "xong"; gia_tri: PhieuCuaToi } | NhanhKhongThanh): KetQuaGoi {
  return kq.kieu === "xong" ? { kieu: "xong", phieu: kq.gia_tri } : kq;
}

export async function guiPhanAnh(lan: LanGui): Promise<KetQuaGoi> {
  const cong = moCong(diaChiGuiPhanAnh());
  if ("kieu" in cong) return cong;
  return thanhPhieu(
    await goi(
      cong.dia_chi,
      { method: "POST", token: cong.token, khoa: lan.khoa, than: lan.than },
      docPhieu,
      // Tuyến gửi không có 404 trong hợp đồng; gặp nó là tuyến chưa được định tuyến ở cụm.
      { kieu: "loi-may-chu" },
    ),
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
  return thanhPhieu(
    await goi(cong.dia_chi, { method: "GET", token: cong.token }, docPhieu, { kieu: "khong-thay" }),
  );
}

/**
 * MỘT TRANG "PHẢN ÁNH CỦA TÔI". `con_tro` rỗng = trang đầu; trang sau truyền NGUYÊN VĂN `con_tro`
 * của trang trước.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI", "XÃ NÀO": công dân và xã đều lấy từ phiên ở máy chủ (luật 4, cấm #1;
 * luật 1, cấm #2). Hợp đồng không có 404/409/503 cho tuyến này — gặp chúng là tuyến lạc ở cụm,
 * nên đổi thành `loi-may-chu` thay vì mượn câu của tuyến gửi ("lần gửi trước đang chạy") vốn
 * không đúng với một lần XEM.
 */
export async function phanAnhCuaToi(con_tro: string): Promise<KetQuaDanhSach> {
  const cong = moCong(diaChiDanhSach(con_tro));
  if ("kieu" in cong) return cong;
  const kq = await goi(cong.dia_chi, { method: "GET", token: cong.token }, docTrangPhieuCuaToi, {
    kieu: "loi-may-chu",
  });
  if (kq.kieu === "xong") return { kieu: "xong", trang: kq.gia_tri };
  if (kq.kieu === "dang-xu-ly-truoc" || kq.kieu === "kenh-chua-mo") return { kieu: "loi-may-chu" };
  return kq;
}
