/**
 * ĐỊA CHỈ MÁY CHỦ ViGov CHO KÊNH CÔNG DÂN — MỘT HOST CHO MỖI DỊCH VỤ SỞ HỮU TUYẾN (ADR 0046).
 *
 * Chủ dự án chốt 26/09/2026: API đi theo DỊCH VỤ, `<service>.api.vigov.vn`. Tuyến
 * `/api/v1/my-citizen-reports…` thuộc `service-petitions`, nên host của nó là
 * `petitions.api.vigov.vn`. `<service>.api-stg.vigov.vn` có trong quy hoạch nhưng chủ dự án chỉ
 * chạy prod — không có dòng staging ở đây, và thêm một dòng như thế là quyết định của chủ dự án.
 *
 * ⚠ CHỈ `petitions` HÔM NAY. `identity` sẽ vào bảng này cùng lúc với cầu phiên công dân ViGov
 * (`phien-vigov.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`) — không sớm hơn: một host
 * không có tuyến nào gọi tới là một host không ai kiểm được nó đúng hay sai.
 *
 * ⚠ ĐÂY LÀ HẰNG SỐ TOÀN NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ — vì thế nó được phép nằm trong mã
 * (luật 1, bất biến 10 chỉ cấm GIÁ TRỊ RIÊNG TỪNG XÃ ở đó). Host nói "dịch vụ nào", không nói "xã
 * nào": mọi xã gọi cùng một host, và xã của yêu cầu do PHIÊN quyết định ở máy chủ (ADR 0005), không
 * do Host. Đổi một xã sang host khác ở đây là cho client chọn xã — đúng thứ luật 1, cấm #2 chặn.
 *
 * ⚠ HOST KHÔNG BAO GIỜ LÀ LÝ DO MỘT YÊU CẦU ĐI RA. `goi-vigov.ts` mở cổng PHIÊN trước cổng địa chỉ:
 * hôm nay `layPhienViGov()` trả `null`, nên có host rồi mà vẫn không một byte nào rời máy
 * (`cong-dan.test.tsx` khẳng định điều đó trên mã thật).
 *
 * ⚠ KHÔNG DÙNG LẠI `VIGOV_API_HOST`. Mặc cho cái tên, biến ấy là địa chỉ của `vihat-miniapp` —
 * backend THƯƠNG MẠI mà khối đăng nhập và bề mặt yêu cầu gọi (`api/dia-chi.ts`,
 * `features/dang-nhap/hop-dong.ts`). Gửi phiếu phản ánh của một công dân tới đó là gửi dữ liệu cá
 * nhân tới một hệ thống không phải cơ quan nhận nó.
 *
 * Chuỗi RỖNG vẫn là tín hiệu dừng (`chua-cau-hinh`) mà `goi-vigov.ts` đọc trước mọi lời gọi: một
 * dòng để trống, hoặc một tên dịch vụ không có trong bảng, là KHÔNG gọi — không bao giờ là đoán.
 */
export type DichVuViGov = "petitions";

const MAY_CHU_THEO_DICH_VU: Readonly<Record<DichVuViGov, string>> = {
  petitions: "https://petitions.api.vigov.vn",
};

/** Địa chỉ đầy đủ của một tuyến ViGov trên host của dịch vụ sở hữu nó, hoặc chuỗi RỖNG. */
export function diaChiViGov(dich_vu: DichVuViGov, duong_dan: string): string {
  // `hasOwnProperty` chứ không đọc thẳng: một tên lạ lọt qua kiểu (chuỗi dựng lúc chạy) phải ra
  // RỖNG, không ra `undefined/api/...` hay một khoá kế thừa của `Object.prototype`.
  if (!Object.prototype.hasOwnProperty.call(MAY_CHU_THEO_DICH_VU, dich_vu)) return "";
  const goc = MAY_CHU_THEO_DICH_VU[dich_vu];
  return goc === "" ? "" : `${goc}${duong_dan}`;
}
