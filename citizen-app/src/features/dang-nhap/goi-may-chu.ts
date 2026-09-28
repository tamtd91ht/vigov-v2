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
 * (điều 3.3.4). `bundle-for-zalo.test.ts` dựng thật bản đẩy lên Zalo (một bản, từ 27/09/2026)
 * rồi khẳng định nó mang đúng MỘT lời gọi, tới đúng MỘT tuyến.
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

import {
  type BridgeRequestWithPhone,
  bridgeBodyWithPhone,
  diaChiPhien,
  docTraLoi,
  docTraLoiCauViGov,
  type Phien,
  type PhienViGovQuaCau,
  thanYeuCau,
  thanYeuCauCauViGov,
  type YeuCauCauViGov,
} from "./hop-dong";

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

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * NHÁNH CẦU PHIÊN ViGov — cùng tuyến, cùng tệp gọi mạng, KHÔNG đụng `phatHanhPhien` ở trên
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Mỗi nhánh là một việc người dân làm tiếp — `App.tsx` dịch chúng sang kiểu của nửa nhà nước:
 *
 *   `xong`                phiên ViGov có bearer và tên xã
 *   `cau-tat`             400 (nhánh cũ đòi `phoneToken` — cầu chưa bật), hoặc 201 không có phiên
 *                         dùng được. Bấm lại không đổi được gì
 *   `chua-san-sang`       422 — app hoặc xã chưa sẵn sàng ở ViGov
 *   `ma-het-han`          401 — mã Zalo quá hạn; bấm lại lấy mã mới là xong
 *   `tam-ngung`           502 · 503 · 429 — chờ rồi thử lại
 *   `khong-goi-duoc`      mất mạng, quá hạn chờ, mã lạ, thân sai khuôn
 *   `chua-khai-host`      bản dựng không có địa chỉ `vihat-miniapp` — không gọi
 */
export type KetQuaCauViGov =
  | { kieu: "xong"; phien: PhienViGovQuaCau }
  | { kieu: "cau-tat" }
  | { kieu: "chua-san-sang" }
  | { kieu: "ma-het-han" }
  | { kieu: "tam-ngung" }
  | { kieu: "khong-goi-duoc" }
  | { kieu: "chua-khai-host" };

/**
 * Đổi access token Zalo + tên miền xã đã xác nhận lấy một phiên công dân ViGov, qua `vihat-miniapp`.
 *
 * ⚠ BEARER NHẬN VỀ LÀ CỦA ViGov (khoá `vigovSession`), không phải phiếu phiên của khối đăng nhập, và
 *   nó KHÔNG vào `kho-phien.tsx`: nó đi thẳng về cho bên gọi (`App.tsx` → `cong-dan/`). Không log.
 *
 * `dia_chi` chỉ để phép kiểm đưa địa chỉ giả vào — cùng lý do với `phatHanhPhien`.
 */
export function moPhienViGovQuaCau(yc: YeuCauCauViGov, dia_chi: string = diaChiPhien()): Promise<KetQuaCauViGov> {
  return callBridge(thanYeuCauCauViGov(yc), dia_chi);
}

/**
 * MỞ LẠI phiên công dân ViGov KÈM mã số điện thoại — chỉ sau cú bấm đồng ý của công dân, khi ViGov trả
 * 403 `chua_xac_thuc_so` (`hop-dong.ts` `bridgeBodyWithPhone`). Cùng tuyến, cùng các nhánh kết quả.
 *
 * ⚠ MÃ SỐ ĐIỆN THOẠI SỐNG ĐÚNG MỘT LỜI GỌI: nó vào thân, thân vào `fetch`, và không đi đâu khác — không
 *   log, không lưu, không trả ngược lên. Thứ trả về chỉ là phiên và cờ `da_xac_thuc_so` của máy chủ.
 */
export function reopenViGovSessionWithPhone(
  yc: BridgeRequestWithPhone,
  dia_chi: string = diaChiPhien(),
): Promise<KetQuaCauViGov> {
  return callBridge(bridgeBodyWithPhone(yc), dia_chi);
}

/** Lời gọi cầu, dùng chung cho hai thân — một chỗ `fetch`, một bảng mã trạng thái. */
async function callBridge(body: string, dia_chi: string): Promise<KetQuaCauViGov> {
  if (dia_chi === "") return { kieu: "chua-khai-host" };

  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  try {
    const tra_loi = await fetch(dia_chi, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      signal: bo_dieu_khien.signal,
    });

    switch (tra_loi.status) {
      case 201:
      case 200: {
        const phien = docTraLoiCauViGov(await tra_loi.json());
        if (phien === null) return { kieu: "khong-goi-duoc" };
        if (phien === "khong-co-phien") return { kieu: "cau-tat" };
        return { kieu: "xong", phien };
      }
      case 400:
        return { kieu: "cau-tat" };
      case 401:
        return { kieu: "ma-het-han" };
      case 422:
        return { kieu: "chua-san-sang" };
      case 429:
      case 502:
      case 503:
        return { kieu: "tam-ngung" };
      default:
        return { kieu: "khong-goi-duoc" };
    }
  } catch {
    return { kieu: "khong-goi-duoc" };
  } finally {
    clearTimeout(dong_ho);
  }
}
