/**
 * TỆP THỨ HAI — VÀ CUỐI CÙNG — TRONG CẢ KHO ĐƯỢC GỌI MẠNG.
 *
 * `phase1-collects-nothing.test.ts` cấm `fetch` / `XMLHttpRequest` / `sendBeacon` / `WebSocket` /
 * `EventSource` / `axios` trên toàn cây mã và miễn cho ĐÚNG HAI ĐƯỜNG DẪN — hai TỆP, không phải
 * hai thư mục:
 *
 *   `features/dang-nhap/goi-may-chu.ts`   phát hành phiên (`/api/v1/sessions`)
 *   `api/goi-may-chu.ts`                  tệp này — bề mặt yêu cầu (`/api/v1/requests`)
 *
 * ⚠ HAI TỆP, KHÔNG PHẢI MỘT THƯ MỤC, VÀ KHÁC BIỆT ẤY LÀ CẢ VẤN ĐỀ.
 *
 *   Miễn theo thư mục thì lời gọi mạng thứ ba, thứ tư mọc lên trong `api/` sáu tuần nữa mà không
 *   có gì đỏ lên — một "gửi log lỗi cho tiện", một bộ đo hành vi. Miễn theo TỆP thì mỗi lời gọi
 *   mới phải đi qua đúng một chỗ có chú thích nói vì sao nó được phép. `hop-dong-yeu-cau.ts` nằm
 *   NGAY CẠNH tệp này và vẫn bị cấm, và có một ca kiểm cho lệnh cấm ăn một `fetch` đặt ở đó.
 *
 * ⚠ TỆP NÀY KHÔNG BIẾT HỢP ĐỒNG. Đường dẫn, tên trường, hình dạng phản hồi nằm trọn trong
 * `hop-dong-yeu-cau.ts`; ở đây chỉ còn cơ chế gọi và ánh xạ mã trạng thái sang các nhánh kết quả.
 *
 * ⚠ KHÔNG `console.log`, KHÔNG BAO GIỜ IN THÂN YÊU CẦU. Thân mang ô ghi chú — chữ người dùng tự
 * gõ, và họ có thể gõ bất cứ thứ gì vào đó, kể cả số điện thoại của chính mình (luật 3, bất biến
 * 1). In nó ra để gỡ lỗi là đưa dữ liệu cá nhân vào một nơi không ai gỡ lại được.
 *
 * ⚠ KHÔNG LƯU GÌ XUỐNG MÁY. Phiếu phiên đi vào đây qua tham số và không bao giờ được ghi lại;
 * lệnh cấm `localStorage`/`sessionStorage`/`cookie`/`indexedDB` KHÔNG được nới một dòng nào cho
 * tệp này.
 */
import {
  diaChiYeuCau,
  docCauLoi,
  docDanhSach,
  docMaYeuCau,
  thanYeuCau,
  type YeuCauDaGui,
  type YeuCauMoi,
} from "./hop-dong-yeu-cau";

/**
 * SÁU NHÁNH, mỗi nhánh một VIỆC NGƯỜI DÙNG PHẢI LÀM khác nhau — tiêu chí duy nhất để tách nhánh.
 *
 *   `xong`             201, có mã yêu cầu để đọc lại.
 *   `chua-dang-nhap`   401 → mời đăng nhập lại; bấm gửi lần nữa cũng hỏng y như vậy.
 *   `tu-choi`          400/429/503 → máy chủ ĐÃ nói câu tiếng Việt của nó, và câu ấy khác nhau ở
 *                      cả ba mã. Gộp làm một nhánh là đúng, vì việc cần làm nằm trong CÂU, không
 *                      nằm trong mã — và client không được viết lại câu ấy.
 *   `chua-khai-host`   bản dựng quên `VIGOV_API_HOST`; câu nói với NGƯỜI DỰNG BẢN.
 *   `khong-goi-duoc`   mất mạng, quá hạn chờ, thân không phải JSON → chờ một lát rồi thử lại.
 *
 * `tu-choi` mang theo `cau` — NGUYÊN VĂN của máy chủ. Nó có thể rỗng khi máy chủ không nói gì,
 * và màn hình có câu lui cho đúng ca ấy.
 */
export type KetQuaGui =
  | { kieu: "xong"; ma_yeu_cau: string }
  | { kieu: "chua-dang-nhap" }
  | { kieu: "tu-choi"; cau: string }
  | { kieu: "chua-khai-host" }
  | { kieu: "khong-goi-duoc" };

export type KetQuaDoc =
  | { kieu: "xong"; danh_sach: readonly YeuCauDaGui[] }
  | { kieu: "chua-dang-nhap" }
  | { kieu: "chua-khai-host" }
  | { kieu: "khong-goi-duoc" };

/** Quá hạn chờ máy chủ. Người dùng đang cầm máy đứng chờ, nên thà nói "thử lại" sớm. */
const HAN_CHO_MS = 15_000;

/** Tiêu đề xác thực. MỘT CHỖ DỰNG, vì hai hàm dưới cùng gửi nó. */
function tieuDe(phien_token: string): Record<string, string> {
  return { Authorization: `Bearer ${phien_token}` };
}

/**
 * Gửi một yêu cầu.
 *
 * KHÔNG NÉM RA NGOÀI: mọi đường đã quy về năm nhánh trên. Một `Promise` bị từ chối ở đây là một
 * màn hình đứng im mãi ở trạng thái "đang gửi", sau khi người dùng vừa gõ xong một đoạn ghi chú.
 *
 * FAIL CLOSED KHI CHƯA CÓ PHIÊN: không gửi một yêu cầu không thuộc về ai. Màn hình đã chặn trước
 * bằng cách không vẽ biểu mẫu khi chưa đăng nhập — đây là lớp thứ hai, và nó có lý do: giữa lúc
 * người dùng đang gõ, phiên có thể hết hạn.
 */
export async function guiYeuCau(
  phien_token: string,
  yc: YeuCauMoi,
  /**
   * Địa chỉ tuyến. MÃ SẢN PHẨM KHÔNG TRUYỀN THAM SỐ NÀY — nó có mặt để phép kiểm đưa vào một địa
   * chỉ giả, vì địa chỉ thật đọc lúc DỰNG và dưới Vitest thì luôn rỗng. Không có khe này thì năm
   * nhánh dưới không ca nào chạy tới được.
   */
  dia_chi: string = diaChiYeuCau(),
): Promise<KetQuaGui> {
  if (dia_chi === "") return { kieu: "chua-khai-host" };
  if (phien_token === "") return { kieu: "chua-dang-nhap" };

  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  try {
    const tra_loi = await fetch(dia_chi, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...tieuDe(phien_token) },
      body: thanYeuCau(yc),
      signal: bo_dieu_khien.signal,
    });

    if (tra_loi.status === 401) return { kieu: "chua-dang-nhap" };

    // BA MÃ CÓ HỢP ĐỒNG, MỘT NHÁNH: 400 sai khuôn · 429 vượt trần · 503 gọi lại chưa cấu hình.
    // Cả ba đều kèm một câu tiếng Việt đã nói việc cần làm tiếp, nên client đọc câu ấy ra chứ
    // không viết lại. `catch` ở đây vì thân lỗi có thể không phải JSON.
    if (tra_loi.status === 400 || tra_loi.status === 429 || tra_loi.status === 503) {
      let cau = "";
      try {
        cau = docCauLoi(await tra_loi.json()) ?? "";
      } catch {
        cau = "";
      }
      return { kieu: "tu-choi", cau };
    }

    if (!tra_loi.ok) return { kieu: "khong-goi-duoc" };

    const ma = docMaYeuCau(await tra_loi.json());
    return ma === null ? { kieu: "khong-goi-duoc" } : { kieu: "xong", ma_yeu_cau: ma };
  } catch {
    return { kieu: "khong-goi-duoc" };
  } finally {
    clearTimeout(dong_ho);
  }
}

/**
 * Đọc danh sách yêu cầu CỦA CHÍNH MÌNH.
 *
 * ⚠ KHÔNG CÓ MỘT THAM SỐ NÀO NÓI "CỦA AI" — không `?nguoiDung=`, không `?phone=` — và việc KHÔNG
 * CÓ chúng chính là tính năng (luật 4, cấm #1: đổi một tham số là đọc dữ liệu của người khác).
 * Máy chủ lấy người dùng từ phiên trong tiêu đề `Authorization`.
 */
export async function docYeuCauCuaToi(
  phien_token: string,
  dia_chi: string = diaChiYeuCau(),
): Promise<KetQuaDoc> {
  if (dia_chi === "") return { kieu: "chua-khai-host" };
  if (phien_token === "") return { kieu: "chua-dang-nhap" };

  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  try {
    const tra_loi = await fetch(dia_chi, {
      method: "GET",
      headers: tieuDe(phien_token),
      signal: bo_dieu_khien.signal,
    });

    if (tra_loi.status === 401) return { kieu: "chua-dang-nhap" };
    if (!tra_loi.ok) return { kieu: "khong-goi-duoc" };

    return { kieu: "xong", danh_sach: docDanhSach(await tra_loi.json()) };
  } catch {
    return { kieu: "khong-goi-duoc" };
  } finally {
    clearTimeout(dong_ho);
  }
}
