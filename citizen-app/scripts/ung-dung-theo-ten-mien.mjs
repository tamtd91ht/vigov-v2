/**
 * TÊN MIỀN XÃ → APP ID ZALO — TỆP CHỌN ĐÍCH CỦA BƯỚC ĐẨY (ADR 0047, câu 1–2).
 *
 * TỆP NÀY CHỈ QUYẾT **ĐẨY BUNDLE LÊN ĐÂU**. Không giá trị nào ở đây đi vào bundle (tên miền nung vào
 * app riêng của xã là giá trị của cờ `--domain`, không phải của tệp này). Chỉ `scripts/deploy.mjs` (qua `dich-den.mjs`) nhập tệp này — không
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
 * Since 02/10/2026 this file also holds a second table, `COMMUNE_TERMS_BY_DOMAIN` (bottom), read by
 * `ho-so-zalo.mjs` only. The sentences above about "chỉ deploy.mjs nhập tệp này" speak of `APP_ID_THEO_TEN_MIEN`.
 *
 * KHÓA là TÊN MIỀN, chữ thường, không scheme, không cổng, không đường dẫn. Một xã giữ được nhiều
 * tên miền (sáp nhập — `service-platform/internal/domain/tenant.go:19-25`), nên hai khoá cùng trỏ
 * một App ID là hợp lệ. Tên miền ở đây chỉ là khoá tra lúc đẩy, KHÔNG BAO GIỜ là tham chiếu xã.
 *
 * GIÁ TRỊ là chữ số. Dạng `<…>` là PLACEHOLDER, coi như CHƯA CÓ App ID: `deploy.mjs` hỏi (có người
 * ngồi trước cửa sổ lệnh) hoặc đòi `--app-id`. Mọi dòng ở đây hiện trên menu chọn đích của `deploy.mjs`,
 * nên không có dòng ví dụ. Chỉ điền App ID chủ dự án đã giao (platform-admin → chi tiết xã → ô QR);
 * đừng điền một con số đoán ra. `deploy.mjs` ghi được vào tệp này khi người chạy đồng ý — khi ấy tệp
 * phải được commit.
 */
export const APP_ID_THEO_TEN_MIEN = {
  // Given by the owner 2026-10-01, replacing 3291993990104489440 (given 2026-09-27). Its `mini_app`
  // row in service-platform is entered by the Jenkins stage `doi-app-id-thang-binh`, which also
  // switches the old row off; until it runs the app deploys but the server refuses to open it
  // (ADR 0047, #3).
  "thangbinh-danang.vigov.vn": "3043188591857102858",
};

/**
 * App ID của APP CHUNG — App ViHAT (`deploy.mjs --app=vihat`). `null` = chưa khai trong tệp này.
 *
 * App ID ấy do kho `vihat-miniapp` sở hữu. Từ 06/10/2026 `deploy.mjs` KHÔNG còn để zmp-cli đọc token
 * trong `citizen-app/.env` cho app chung: để `null` thì mỗi lần đẩy App ViHAT phải có người nhập App ID
 * (hoặc `--app-id`), rồi token phải mang đúng claim ấy. Ghi `"<chữ số>"` vào đây — tay, hoặc để
 * `deploy.mjs` ghi khi được hỏi — rồi commit.
 */
export const APP_ID_APP_CHUNG = null;

/**
 * PER-COMMUNE VALUES OF THE COMMUNE APP'S TERMS OF USE (owner, 02/10/2026; ADR 0044 §"Đã quyết 02/10/2026").
 *
 * Read ONLY by `scripts/ho-so-zalo.mjs`, which fills `src/content/commune-terms.ts` with one row to write the commune
 * dossier's `dieu-khoan-su-dung.txt`. Like the table above, nothing here reaches the bundle: no production file
 * under `src/` imports this file (`dich-den.test.mjs`), and `bundle-for-zalo.test.ts` reads this file raw.
 *
 * Same key as `APP_ID_THEO_TEN_MIEN` — the commune's domain — so the two rows of one commune sit side by side and
 * a reviewer sees both. A commune's dossier needs both rows; `ho-so-zalo.mjs` refuses when either is missing.
 *
 * Every field is REQUIRED, and a missing or `<…>` value stops the dossier (`communeTermsValuesFor`): this text is
 * published in the commune's name, so no default and no placeholder is ever printed into it. Fill only what the
 * owner has given. Field keys are unquoted on purpose: `bundle-for-zalo.test.ts` treats every `"key": "value"`
 * line of this file as an App ID mapping, and a province name is not one.
 */
export const COMMUNE_TERMS_BY_DOMAIN = {
  // Given by the owner 2026-10-02.
  "thangbinh-danang.vigov.vn": {
    displayName: "Xã Thăng Bình",
    province: "Thành phố Đà Nẵng",
    introductionUrl: "https://thangbinh.danang.gov.vn/gioi-thieu/gioi-thieu-chung",
  },
};
