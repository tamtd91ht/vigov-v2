/**
 * TÊN MIỀN XÃ → APP ID ZALO — TỆP CHỌN ĐÍCH CỦA BƯỚC ĐẨY (ADR 0047, câu 1–2).
 *
 * TỆP NÀY CHỈ QUYẾT **ĐẨY BUNDLE LÊN ĐÂU**. Bundle đẩy lên mọi App ID là CÙNG MỘT bundle; không giá
 * trị nào ở đây đi vào bundle. Chỉ `scripts/deploy.mjs` (qua `dich-den.mjs`) nhập tệp này — không
 * `vite.config.ts`, không tệp nào dưới `src/`, và `dich-den.test.mjs` ghim đúng điều đó. Nhập nó
 * từ bước dựng là đúng thứ ADR 0044 điều kiện dừng #1 cấm: giá trị theo xã vào bundle.
 *
 * TỆP NÀY KHÔNG PHẢI NGUỒN SỰ THẬT VỀ XÃ CỦA MỘT APP ID. Bảng `MiniApp` (`app_id → tenant_id`) của
 * `service-platform` là nơi DUY NHẤT máy chủ đọc để biết một App ID phục vụ xã nào (ADR 0044, 0045).
 * Hai nơi nói về một cặp (xã, App ID) thì sẽ lệch; khi lệch, máy chủ thắng — app mở ra bị từ chối
 * hoặc vào đúng xã bảng platform nói, không bao giờ vào xã tệp này nói. Chưa có phép kiểm đối
 * chiếu hai nơi (ADR 0047, CÒN MỞ #3): thêm một dòng ở đây thì thêm cả dòng `MiniApp` ở platform.
 *
 * App ID không phải bí mật. `ZMP_TOKEN` thì CÓ — nó không bao giờ nằm ở đây (luật 8).
 *
 * KHÓA là TÊN MIỀN, chữ thường, không scheme, không cổng, không đường dẫn. Một xã giữ được nhiều
 * tên miền (sáp nhập — `service-platform/internal/domain/tenant.go:19-25`), nên hai khoá cùng trỏ
 * một App ID là hợp lệ. Tên miền ở đây chỉ là khoá tra lúc đẩy, KHÔNG BAO GIỜ là tham chiếu xã.
 *
 * GIÁ TRỊ dạng `<…>` là PLACEHOLDER: `--thu` in được kế hoạch với nó, lần chạy thật thì bị từ chối.
 * Dòng dưới là dòng VÍ DỤ duy nhất — `.example` là tên miền dành riêng (RFC 2606), không trỏ vào
 * xã nào. Chỉ điền App ID chủ dự án đã giao; đừng điền một con số đoán ra.
 */
export const APP_ID_THEO_TEN_MIEN = {
  "xa-vi-du.vigov.example": "<APP-ID-MINI-APP-CUA-XA>",
  // Given by the owner 2026-09-27. Its `mini_app` row in service-platform is entered by an
  // operator; until then the app deploys but the server refuses to open it (ADR 0047, #3).
  "thangbinh-danang.vigov.vn": "3291993990104489440",
};

/**
 * App ID của APP CHUNG (bản `goc`, không truyền tên miền). `null` = chưa khai trong tệp này.
 *
 * Hôm nay app chung là app mà `ZMP_TOKEN` trong `citizen-app/.env` thuộc về (`zmp login` ghi cả
 * hai), và App ID ấy do kho `vihat-miniapp` sở hữu. Để `null` thì đường app chung chạy như trước;
 * khai nó thì `deploy.mjs` còn kiểm được token trong môi trường đúng là token của app chung.
 */
export const APP_ID_APP_CHUNG = null;
