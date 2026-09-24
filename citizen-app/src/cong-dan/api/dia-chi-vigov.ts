/**
 * ĐỊA CHỈ MÁY CHỦ ViGov CHO KÊNH CÔNG DÂN — CHƯA CÓ, VÀ VÌ THẾ LÀ CHUỖI RỖNG (fail closed).
 *
 * ⚠ KHÔNG DÙNG LẠI `VIGOV_API_HOST`. Mặc cho cái tên, biến ấy là địa chỉ của `vihat-miniapp` —
 * backend THƯƠNG MẠI mà khối đăng nhập và bề mặt yêu cầu gọi (`api/dia-chi.ts`,
 * `features/dang-nhap/hop-dong.ts`: "MÁY CHỦ LÀ `vihat-miniapp` — KHO RIÊNG, KHÔNG PHẢI ViGov").
 * Gửi phiếu phản ánh của một công dân tới đó là gửi dữ liệu cá nhân tới một hệ thống không phải cơ
 * quan nhận nó.
 *
 * ⚠ KHÔNG CÓ GIÁ TRỊ MẶC ĐỊNH, VÀ KHÔNG BAO GIỜ ĐƯỢC CÓ. Chưa ai chốt kênh công dân gọi ViGov qua
 * host nào, và biến cấu hình lúc dựng cho nó chưa tồn tại (`scripts/cau-hinh.mjs` chặn mọi tên
 * ngoài danh sách trắng). Một địa chỉ đoán ở đây là nội dung phản ánh của người thật đi tới một
 * máy chủ không ai chọn.
 *
 * ⚠ MỘT ĐỊA CHỈ CHO MỌI XÃ, không phải một địa chỉ mỗi xã: xã của yêu cầu do PHIÊN quyết định,
 * không do client chọn máy chủ (`skills/zalo-miniapp-multi-tenant` §"The API host is singular").
 *
 * Chuỗi rỗng là tín hiệu dừng mà `goi-vigov.ts` đọc trước mọi lời gọi — cùng quy ước với
 * `api/dia-chi.ts`.
 */
const MAY_CHU_VIGOV: string = "";

/** Địa chỉ đầy đủ của một tuyến ViGov, hoặc chuỗi RỖNG khi chưa có máy chủ. */
export function diaChiViGov(duong_dan: string): string {
  return MAY_CHU_VIGOV === "" ? "" : `${MAY_CHU_VIGOV}${duong_dan}`;
}
