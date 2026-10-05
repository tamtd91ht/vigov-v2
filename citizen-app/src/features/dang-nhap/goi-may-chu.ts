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
import type { LocationCodes, MaDangNhap } from "../tinh-nang/zalo-api";

import {
  type BridgeRequestWithPhone,
  bridgeBodyWithPhone,
  type CommuneAppSessionRequest,
  communeAppSessionBody,
  diaChiPhien,
  docTraLoi,
  docTraLoiCauViGov,
  type ExchangedLocation,
  locationAddress,
  locationBody,
  type Phien,
  readLocation,
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

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * LOCATION EXCHANGE — `POST /api/v1/location`, same server and same host as the two calls above
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * One branch per thing the citizen does next — the only criterion for a separate branch:
 *
 *   `xong`                 200 with coordinates inside the world's range
 *   `qua-nhieu-lan`        429 — wait a few minutes (pressing again at once fails the same way)
 *   `zalo-khong-tra-loi`   502 — every Zalo failure, the ~2-minute token expiry included: press again
 *   `yeu-cau-hong`         400 / 405 — a body the server could not read; a fresh tap builds a fresh one
 *   `tam-ngung`            503 — the route is not wired on that server; pressing again changes nothing
 *   `khong-goi-duoc`       network, timeout, an unknown status, a 200 body out of shape
 *   `chua-khai-host`       the build has no `vihat-miniapp` address — nothing is sent
 */
export type LocationExchangeResult =
  | { kieu: "xong"; location: ExchangedLocation }
  | { kieu: "qua-nhieu-lan" }
  | { kieu: "zalo-khong-tra-loi" }
  | { kieu: "yeu-cau-hong" }
  | { kieu: "tam-ngung" }
  | { kieu: "khong-goi-duoc" }
  | { kieu: "chua-khai-host" };

/**
 * Exchange the two Zalo codes for coordinates.
 *
 * NEVER THROWS, and NEVER LOGS: the body carries a code that locates a real person, and the answer is
 * where they stand — often their doorstep (rule 3). `address` is only for tests to pass a fake address
 * (the real one is read at build time and is empty under Vitest), exactly like `phatHanhPhien`.
 */
export async function exchangeLocation(
  codes: LocationCodes,
  address: string = locationAddress(),
  /** The commune app's App ID (selects its secret), or `null` for the shared app. */
  appId: string | null = null,
): Promise<LocationExchangeResult> {
  if (address === "") return { kieu: "chua-khai-host" };

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), HAN_CHO_MS);

  try {
    const response = await fetch(address, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: locationBody(codes, appId),
      signal: controller.signal,
    });

    switch (response.status) {
      case 200: {
        const location = readLocation(await response.json());
        return location === null ? { kieu: "khong-goi-duoc" } : { kieu: "xong", location };
      }
      case 400:
      case 405:
        return { kieu: "yeu-cau-hong" };
      case 429:
        return { kieu: "qua-nhieu-lan" };
      case 502:
        return { kieu: "zalo-khong-tra-loi" };
      // 422 `app_not_configured`: the App ID is not one the server knows. Like 503, pressing again changes
      // nothing — only the shared app's body (no `appId`) can never meet it.
      case 422:
      case 503:
        return { kieu: "tam-ngung" };
      default:
        return { kieu: "khong-goi-duoc" };
    }
  } catch {
    return { kieu: "khong-goi-duoc" };
  } finally {
    clearTimeout(timer);
  }
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * ĐĂNG NHẬP TỪ APP RIÊNG CỦA MỘT XÃ — thân thứ tư (`hop-dong.ts` `communeAppSessionBody`), bảng mã trạng
 * thái RIÊNG vì việc người dân làm tiếp khác app chung.
 *
 * ⚠ SINCE ADR 0066 (01/10/2026) THIS IS A DIFFERENT SERVER: ViGov identity `POST /api/v1/citizen-sessions`,
 *   not `vihat-miniapp`. Hence NO default address here — the caller passes
 *   `communeAppSessionAddress(<identity host>)`. A default of `diaChiPhien()` would silently send the
 *   commune app's two Zalo codes to the commercial server again. Status table unchanged by the move:
 *
 *   `xong`                 201 có `vigovSession` dùng được
 *   `app-chua-san-sang`    422 — App ID lạ với máy chủ, hoặc app chưa gắn xã / xã ngừng: bấm lại vô ích
 *   `cau-tat`              503 — cầu phiên ViGov tắt hoặc chưa lắp ráp; 201 không phiên dùng được
 *   `yeu-cau-hong`         400 — thân máy chủ không nhận (thiếu mã, ViGov từ chối như lỗi nối dây)
 *   `ma-het-han`           401 — mã Zalo quá hạn; lượt sau lấy mã mới
 *   `zalo-khong-tra-loi`   502 — máy chủ không với tới Zalo; chờ một lát
 *   `qua-nhieu-lan`        429 — chờ vài phút
 *   `khong-goi-duoc`       mạng, quá hạn chờ, mã lạ, thân sai khuôn
 *   `chua-khai-host`       no identity address was handed in — nothing is sent
 *
 * App chung (`callBridge`) gộp 502/503/429 thành một `tam-ngung`; ở đây tách ra vì app riêng không có
 * đường lùi nào khác, và "chờ một lát" (502) khác hẳn "hệ thống chưa bật" (503).
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type CommuneAppBridgeResult =
  | { kieu: "xong"; phien: PhienViGovQuaCau }
  | { kieu: "app-chua-san-sang" }
  | { kieu: "cau-tat" }
  | { kieu: "yeu-cau-hong" }
  | { kieu: "ma-het-han" }
  | { kieu: "zalo-khong-tra-loi" }
  | { kieu: "qua-nhieu-lan" }
  | { kieu: "khong-goi-duoc" }
  | { kieu: "chua-khai-host" };

/**
 * Đổi hai mã Zalo + App ID lấy một phiên công dân ViGov, tại identity. KHÔNG NÉM, KHÔNG LOG: thân mang mã
 * đổi được thành số điện thoại (luật 3). `address` is REQUIRED: `communeAppSessionAddress(<identity host>)`.
 */
export async function openCommuneAppSessionCall(
  req: CommuneAppSessionRequest,
  address: string,
): Promise<CommuneAppBridgeResult> {
  return postCommuneAppSession(communeAppSessionBody(req), address);
}

/** The commune app's status table for its login body. */
async function postCommuneAppSession(body: string, address: string): Promise<CommuneAppBridgeResult> {
  if (address === "") return { kieu: "chua-khai-host" };
  const answer = await postSession(body, address);
  if (answer === null) return { kieu: "khong-goi-duoc" };
  switch (answer.status) {
    case 200:
    case 201: {
      const phien = docTraLoiCauViGov(answer.body);
      if (phien === null) return { kieu: "khong-goi-duoc" };
      if (phien === "khong-co-phien") return { kieu: "cau-tat" };
      return { kieu: "xong", phien };
    }
    case 400:
      return { kieu: "yeu-cau-hong" };
    case 401:
      return { kieu: "ma-het-han" };
    case 422:
      return { kieu: "app-chua-san-sang" };
    case 429:
      return { kieu: "qua-nhieu-lan" };
    case 502:
      return { kieu: "zalo-khong-tra-loi" };
    case 503:
      return { kieu: "cau-tat" };
    default:
      return { kieu: "khong-goi-duoc" };
  }
}

/** Lời gọi cầu, dùng chung cho hai thân của app chung — một bảng mã trạng thái. */
async function callBridge(body: string, dia_chi: string): Promise<KetQuaCauViGov> {
  if (dia_chi === "") return { kieu: "chua-khai-host" };
  const answer = await postSession(body, dia_chi);
  if (answer === null) return { kieu: "khong-goi-duoc" };
  switch (answer.status) {
    case 201:
    case 200: {
      const phien = docTraLoiCauViGov(answer.body);
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
}

/**
 * MỘT `fetch` cho mọi thân đi cầu phiên. Thân trả lời chỉ đọc ở 200/201 — thân lỗi không được đọc (câu
 * của máy chủ không ra màn hình). `null` = mất mạng, quá hạn chờ, hoặc thân 2xx không phải JSON.
 */
async function postSession(body: string, address: string): Promise<{ status: number; body: unknown } | null> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), HAN_CHO_MS);
  try {
    const response = await fetch(address, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      signal: controller.signal,
    });
    const status = response.status;
    return { status, body: status === 200 || status === 201 ? await response.json() : null };
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}
