/**
 * TỆP DUY NHẤT TRONG CẢ KHO ĐƯỢC GỌI MẠNG — và "duy nhất" ở đây là một ràng buộc kiểm được,
 * không phải một thoả thuận.
 *
 * `phase1-collects-nothing.test.ts` cấm `fetch` / `XMLHttpRequest` / `WebSocket` /
 * `EventSource` / `axios` trên toàn cây mã và MIỄN ĐÚNG MỘT ĐƯỜNG DẪN: chính tệp này. Không
 * phải thư mục này — TỆP này. `index.ts` nằm ngay cạnh vẫn bị cấm, và có một ca kiểm cho lệnh
 * cấm ăn đúng một vi phạm đặt trong `index.ts` để chứng minh điều đó.
 *
 *   Vì sao hẹp tới mức ấy: một ngoại lệ theo THƯ MỤC thì sáu tuần nữa lời gọi mạng thứ hai, thứ
 *   ba nằm rải trong thư mục và không có gì đỏ lên. Một ngoại lệ theo TỆP thì mỗi lời gọi mới
 *   phải đi qua đúng chỗ này, nơi có chú thích nói vì sao nó được phép.
 *
 * ⚠ TỆP NÀY CÓ MẶT TRONG BẢN NỘP, và điều đó vừa đảo chiều — 20/09/2026. Trước đây nó nằm sau
 * một cửa biến thể để bản nộp không gọi mạng; nay CẢ HAI biến thể gọi máy chủ thật, vì một nút
 * đăng nhập bấm là được thuyết phục vòng duyệt hơn hẳn một nút nói "bản này chưa nối máy chủ"
 * (điều 3.3.4). `bundle-for-zalo.test.ts` dựng thật cả hai biến thể rồi khẳng định CẢ HAI mang
 * đúng MỘT lời gọi, tới đúng MỘT tuyến.
 *
 * ⚠ TỆP NÀY KHÔNG BIẾT HỢP ĐỒNG. Đường dẫn, tên trường gửi đi và hình dạng phản hồi nằm trọn
 * trong `hop-dong.ts`; ở đây chỉ còn cơ chế gọi. Hợp đồng đổi thì sửa một tệp, và tệp ấy không
 * phải tệp này.
 *
 * ⚠ KHÔNG `console.log`, KHÔNG ĐƯA THÔNG BÁO LỖI CỦA MÁY CHỦ RA MÀN HÌNH. Thân yêu cầu mang hai
 * mã đổi được thành số điện thoại của một người thật (luật 3, bất biến 1). In nó ra để gỡ lỗi
 * là đưa dữ liệu cá nhân vào một nơi không ai gỡ lại được — kể cả khi nơi ấy chỉ là console
 * của một máy thử nghiệm.
 *
 * ⚠ KHÔNG LƯU GÌ XUỐNG MÁY. Phiếu phiên đi thẳng cho `useState` của màn hình gọi tới và mất đi
 * khi ứng dụng đóng — đúng cách lớp khám phá giữ xã đã chọn. Dây bẫy cấm `localStorage` /
 * `sessionStorage` / `cookie` / `indexedDB` KHÔNG được nới một dòng nào cho tệp này.
 */
import type { MaDangNhap } from "../tinh-nang/zalo-api";

import { diaChiPhien, docTraLoi, type Phien, thanYeuCau } from "./hop-dong";

/**
 * NĂM NHÁNH, cùng lối với `KetQuaXin` của `zalo-api.ts`: mỗi nhánh là một CÂU KHÁC NHAU trên
 * màn hình, không phải một mã lỗi. Hai nhánh giữa tách riêng vì VIỆC NGƯỜI DÙNG PHẢI LÀM khác
 * hẳn nhau — đó là tiêu chí duy nhất để tách một nhánh lỗi:
 *
 *   `ma-het-han` (401)          mã Zalo sai hoặc đã quá hai phút → **bấm lại ngay** là xong.
 *   `zalo-khong-tra-loi` (502)  máy chủ không với tới Zalo → bấm lại ngay cũng hỏng y như vậy;
 *                               việc cần làm là **chờ một lát rồi thử lại**. Nói "bấm lại đi"
 *                               ở đây là bảo người ta làm một việc vô ích, ba lần liền.
 *
 * Gộp chúng lại thành một câu chung chung thì cả hai nhóm người đều nhận một lời khuyên sai một
 * nửa — và không ai đọc được mã trạng thái để tự phân biệt.
 */
export type KetQuaPhien =
  | { kieu: "xong"; phien: Phien }
  | { kieu: "chua-khai-host" }
  | { kieu: "ma-het-han" }
  | { kieu: "zalo-khong-tra-loi" }
  | { kieu: "khong-goi-duoc" };

/** Quá hạn chờ máy chủ. Người dùng đang cầm máy đứng chờ, nên thà nói "bấm lại" sớm. */
const HAN_CHO_MS = 15_000;

/**
 * Đổi hai mã của Zalo lấy một phiên.
 *
 * KHÔNG NÉM RA NGOÀI: mọi đường đã quy về bốn nhánh trên, đúng cách `xin` trong `zalo-api.ts`
 * làm. Một `Promise` bị từ chối ở đây là một màn hình đứng im mãi ở trạng thái "đang gửi".
 *
 * FAIL CLOSED KHI CHƯA KHAI HOST: không đoán một địa chỉ, không rơi về `localhost`.
 */
export async function phatHanhPhien(
  ma: MaDangNhap,
  /**
   * Địa chỉ tuyến. MÃ SẢN PHẨM KHÔNG TRUYỀN THAM SỐ NÀY — nó có mặt để phép kiểm đưa vào một
   * địa chỉ giả, vì địa chỉ thật đọc lúc DỰNG và dưới Vitest thì luôn rỗng. Không có khe này
   * thì bốn nhánh dưới không ca nào chạy tới được, và một hàm không ai kiểm là một hàm sẽ hỏng
   * đúng ngày máy chủ thật trả về thứ nó không ngờ.
   */
  dia_chi: string = diaChiPhien(),
): Promise<KetQuaPhien> {
  if (dia_chi === "") return { kieu: "chua-khai-host" };

  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  try {
    const tra_loi = await fetch(dia_chi, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: thanYeuCau(ma),
      signal: bo_dieu_khien.signal,
    });

    // Hai mã trạng thái CÓ HỢP ĐỒNG, đọc đúng như hợp đồng khai — không suy diễn thêm mã nào.
    // 401: mã Zalo sai hoặc hết hạn. 502: máy chủ không với tới Zalo.
    if (tra_loi.status === 401) return { kieu: "ma-het-han" };
    if (tra_loi.status === 502) return { kieu: "zalo-khong-tra-loi" };
    if (!tra_loi.ok) return { kieu: "khong-goi-duoc" };

    const phien = docTraLoi(await tra_loi.json());
    return phien === null ? { kieu: "khong-goi-duoc" } : { kieu: "xong", phien };
  } catch {
    // Mất mạng, quá hạn chờ, thân trả lời không phải JSON — cùng một việc cần làm tiếp.
    return { kieu: "khong-goi-duoc" };
  } finally {
    clearTimeout(dong_ho);
  }
}
