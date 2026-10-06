/**
 * BẢNG THEO TÊN MIỀN XÃ CỦA CÁC SCRIPT — từ 06/10/2026 chỉ còn `COMMUNE_TERMS_BY_DOMAIN`.
 *
 * APP ID KHÔNG CÒN Ở ĐÂY (chủ dự án 06/10/2026, phương án A). Hai bảng `APP_ID_THEO_TEN_MIEN` và
 * `APP_ID_APP_CHUNG` đã gỡ: `deploy.mjs` hỏi App ID của đích từ service-platform
 * (`GET /api/v1/mini-app-ids?app=vihat | ?host=<tên-miền>`, bảng `mini_app`) — chính bảng máy chủ đọc để
 * biết một App ID phục vụ xã nào (ADR 0044, 0045). Chép tay App ID vào đây là hai nguồn cho một sự thật,
 * và hai nguồn thì lệch (ADR 0047 CÒN MỞ #3). Khai / đổi App ID: platform-admin.
 *
 * KHÓA là TÊN MIỀN, chữ thường, không scheme, không cổng, không đường dẫn. Không giá trị nào ở đây đi
 * vào bundle: không tệp sản phẩm nào dưới `src/` và không `vite.config.ts` nhập tệp này
 * (`dich-den.test.mjs`), và `bundle-for-zalo.test.ts` đọc thô tệp này để khẳng định không khoá nào của
 * nó có mặt trong bundle dựng thật.
 */

/**
 * PER-COMMUNE VALUES OF THE COMMUNE APP'S TERMS OF USE (owner, 02/10/2026; ADR 0044 §"Đã quyết 02/10/2026").
 *
 * Read ONLY by `scripts/ho-so-zalo.mjs`, which fills `src/content/commune-terms.ts` with one row to write the commune
 * dossier's `dieu-khoan-su-dung.txt`. Nothing here reaches the bundle (see above).
 *
 * Every field is REQUIRED, and a missing or `<…>` value stops the dossier (`communeTermsValuesFor`): this text is
 * published in the commune's name, so no default and no placeholder is ever printed into it. Fill only what the
 * owner has given. Field keys are unquoted on purpose: `bundle-for-zalo.test.ts` treats every quoted key of this
 * file as a commune domain that must not appear in the bundle, and a field name is not one.
 */
export const COMMUNE_TERMS_BY_DOMAIN = {
  // Given by the owner 2026-10-02.
  "thangbinh-danang.vigov.vn": {
    displayName: "Xã Thăng Bình",
    province: "Thành phố Đà Nẵng",
    introductionUrl: "https://thangbinh.danang.gov.vn/gioi-thieu/gioi-thieu-chung",
  },
};
