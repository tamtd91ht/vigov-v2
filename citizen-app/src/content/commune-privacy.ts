/**
 * PRIVACY POLICY OF A COMMUNE'S APP — one template, filled per commune when the Zalo dossier is generated. Decree
 * 13/2023/ND-CP. Built like `commune-terms.ts`, and for the same reasons.
 *
 * WHY A SECOND POLICY (owner decisions of 11/10/2026):
 *
 *   To the citizen, the ViHAT Group app and the commune's app are TWO SEPARATE APPS. `chinh-sach-rieng-tu.ts` is the
 *   ViHAT app's and names no commune; this text is the commune's and names no company. It covers the commune
 *   interface wherever it runs: the commune's own Mini App (`App.tsx` `AppRieng`) and the commune page a commune QR
 *   opens in the shared App ID (`App.tsx` `QrCommuneApp`). The two differ in one place — how the session with the
 *   commune is opened — and the text says both.
 *
 *   The servers of the technical platform the app runs on (`vihat-miniapp`: the QR-path session bridge, the location
 *   exchange, the SDK failure report) are named NEUTRALLY, never by brand: "máy chủ của đơn vị cung cấp nền tảng kỹ
 *   thuật của ứng dụng" (owner decision 1).
 *
 * DECIDED VALUES, AND ONLY THESE (owner decision 6, 11/10/2026):
 *
 *   | Fact | Value |
 *   |---|---|
 *   | data controller | Ủy ban nhân dân <xã> — receives and processes petitions, answers view / correct / delete requests |
 *   | petition data (content, name, phone, coordinates, photos) | kept "theo thời hạn lưu trữ hồ sơ mà pháp luật về lưu trữ quy định cho loại hồ sơ ấy" |
 *   | session / identity data (verified phone, Zalo account id, IP, session log) | kept "cho tới khi bạn yêu cầu xoá" |
 *   | operator of the scene-photo store | "đơn vị cung cấp nền tảng kỹ thuật của ứng dụng, theo thoả thuận với Ủy ban nhân dân xã" |
 *
 *   Nothing else legal is decided here. NOT WRITTEN, still owed to the owner: the disaster alerts and the "Nhận thông
 *   báo" switch (ADR 0091 §2, §4) and the send form's map (ADR 0072 §Sửa đổi 11/10/2026 M1, M2, M5 — the basemap's
 *   viewport sentence awaits the owner, `ket-xuat-ho-so.ts`).
 *
 * FIDELITY LOCKS (`commune-privacy.test.ts`): every declaration of the bodies the commune interface sends sits
 * VERBATIM in this text — the own app's login (`COMMUNE_APP_SESSION_FIELDS`), the QR-path bridge
 * (`BRIDGE_FIELDS_WITH_PHONE`), the location exchange (`LOCATION_FIELDS` + `appId`), the SDK failure report
 * (`CLIENT_ERROR_FIELDS`), the scene-photo upload (`SCENE_PHOTO_UPLOAD_FIELDS`) — and every key the petition body,
 * the rating body and the draft store can carry has its phrase. The first four are JOINED FROM THE TABLES here
 * (nothing renders this text in the bundle, so nothing needs them as literals); the photo sentence is written out,
 * because `ket-xuat-ho-so.ts` imports this module and may not be imported back.
 *
 * WHY THE PER-COMMUNE VALUES ARE PARAMETERS: same as `commune-terms.ts` — they live in `COMMUNE_TERMS_BY_DOMAIN`
 * under `scripts/`, which no file under `src/` may import or name (rule 1, invariant 10). Nothing in the app imports
 * this module; only the dossier generator does.
 *
 * WHY A MISSING VALUE THROWS: a legal text published in the commune's name. A default or a printed placeholder would
 * publish a wrong controller, or none, and nothing would turn red.
 */

import {
  BRIDGE_FIELDS_WITH_PHONE,
  CLIENT_ERROR_FIELDS,
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  LOCATION_FIELDS,
} from "../features/dang-nhap/hop-dong";
import type { TruongGuiDi } from "../api/hop-dong-yeu-cau"; // vi-name-ok: existing type, imported, not a new name

import type { MucChinhSach } from "./chinh-sach-rieng-tu"; // vi-name-ok: existing type, imported
import { communeAuthority, communeTermsProblems, lowerFirst, type CommuneTermsValues } from "./commune-terms";
import { cauKhaiDichRaNgoai, COMMUNE_DESTINATIONS } from "./dich-ra-ngoai"; // vi-name-ok: existing export, imported

/** `1.0`: no citizen has read any version — the commune app is not on Zalo yet. From the first release on, every
 *  change to what is processed takes a new number. */
export const COMMUNE_PRIVACY_VERSION = "1.0";
/** The day the content became true: the owner split the policy and fixed its values (11/10/2026). */
export const COMMUNE_PRIVACY_EFFECTIVE_DATE = "11/10/2026";
export const COMMUNE_PRIVACY_TITLE = "Chính sách quyền riêng tư";

/** The neutral name of the technical platform's servers (owner decision 1). One string, so no paragraph varies it. */
export const PLATFORM_SERVER = "máy chủ của đơn vị cung cấp nền tảng kỹ thuật của ứng dụng";

/** Petition data retention — owner decision 6, verbatim. */
export const PETITION_RETENTION = "theo thời hạn lưu trữ hồ sơ mà pháp luật về lưu trữ quy định cho loại hồ sơ ấy";
/** Session / identity data retention — owner decision 6, verbatim. */
export const SESSION_RETENTION = "cho tới khi bạn yêu cầu xoá";

/** The declarations of one body, as one list sentence: "a; b; và c". Each declaration carries no `;` itself. */
function listOf(fields: readonly TruongGuiDi[]): string {
  const phrases = fields.map((f) => f.trong_chinh_sach);
  return phrases.length <= 1 ? (phrases[0] ?? "") : `${phrases.slice(0, -1).join("; ")}; và ${phrases[phrases.length - 1]}`;
}

/** The `appId` line the own app's location body adds — the one row `COMMUNE_APP_LOCATION_FIELDS` has beyond the two. */
const LOCATION_APP_ID = COMMUNE_APP_LOCATION_FIELDS.filter((f) => !LOCATION_FIELDS.includes(f));

export type CommunePrivacy = {
  appLine: string;
  opening: string;
  sections: readonly MucChinhSach[];
};

/**
 * The text. Validates first and throws on any problem (the same checks as the terms), so no caller can render a
 * half-filled template.
 */
export function communePrivacy(values: CommuneTermsValues): CommunePrivacy {
  const problems = communeTermsProblems(values);
  if (problems.length > 0) {
    throw new Error(`Không dựng được chính sách quyền riêng tư của app xã:\n${problems.map((p) => `  - ${p}`).join("\n")}`);
  }
  const { displayName, province, introductionUrl } = values;
  const authority = communeAuthority(displayName);

  const sections: MucChinhSach[] = [
    {
      ma: "ben-chiu-trach-nhiem",
      tieu_de: "Bên chịu trách nhiệm về dữ liệu",
      doan: [
        `${authority} là bên chịu trách nhiệm về dữ liệu cá nhân bạn gửi qua ứng dụng: ${authority} tiếp nhận và xử lý phản ánh bạn gửi, và trả lời các yêu cầu xem, sửa hoặc xoá dữ liệu của bạn.`,
        `Một số bước kỹ thuật đi qua ${PLATFORM_SERVER}. Từng mục dưới đây nói rõ đó là bước nào, và máy chủ ấy nhận những gì.`,
        `Thông tin liên hệ của ${authority} nằm tại trang Giới thiệu của xã: ${introductionUrl}`,
      ],
    },
    {
      ma: "du-lieu",
      tieu_de: "Dữ liệu ứng dụng xử lý",
      doan: [
        "Các ô bạn có thể điền: nội dung phản ánh, địa chỉ nơi xảy ra sự việc, họ tên và số điện thoại trên màn gửi phản ánh; lời nhận xét khi bạn đánh giá kết quả xử lý; và mã tra cứu khi bạn tra cứu một phản ánh.",
        "Ứng dụng không đọc danh bạ, không đọc tin nhắn và không theo dõi hành vi sử dụng của bạn.",
        "Ứng dụng không tự đọc thư viện ảnh của bạn. Nó chỉ nhận đúng những ảnh bạn tự chụp hoặc tự chọn để gửi kèm một phản ánh, và chỉ khi chính bạn bấm nút.",
      ],
    },
    {
      // Sources: `cong-dan/man/TrangXa.tsx`, `cong-dan/api/hop-dong-cong-khai.ts`, `ket-xuat-ho-so.ts` `DUONG_CONG_KHAI`;
      // the name and avatar: `zalo-api.ts` `readZaloProfile`, row `getUserInfo` (`commune_app`).
      ma: "khi-mo-ung-dung",
      tieu_de: "Khi bạn mở ứng dụng",
      doan: [
        "NGAY KHI ỨNG DỤNG MỞ RA, trước khi bạn bấm gì, ứng dụng hỏi hệ thống của xã tên xã, địa chỉ trụ sở, đường dây nóng, giờ làm việc, tin mới và dải ảnh trên trang chủ. Khi bạn tự mở tin tức, một bài viết hay danh bạ, ứng dụng hỏi tiếp đúng phần ấy. Những lời hỏi này chỉ mang tên miền của xã — cùng loại tin, chuyên mục hay mã bài viết khi bạn chọn — và không mang số điện thoại, mã Zalo hay thông tin nào khác của bạn.",
        "Ảnh bìa tin, ảnh trong bài, dải ảnh trang chủ và logo của xã được máy bạn tải thẳng từ kho ảnh công khai của hệ thống của xã ngay khi chúng hiện lên màn hình, không cần bạn bấm. Âm thanh của bản tin truyền thanh chỉ được tải khi bạn bấm 'Nghe', qua một đường dẫn chỉ dùng được trong thời gian ngắn. Những lần tải này không mang dữ liệu nào của bạn; nhưng như với mọi trang web, nơi lưu ảnh và âm thanh nhìn thấy địa chỉ IP và thông tin trình duyệt của máy bạn.",
        "Nếu bạn mở trang của xã bằng mã QR hoặc đường liên kết của xã, tên miền của xã lấy từ mã ấy chỉ để ứng dụng biết phải hiện trang của xã nào. Nó không mở phiên làm việc nào và không cho ai quyền gì: phiên làm việc với xã chỉ mở khi chính bạn đồng ý, như mục 'Xác nhận số điện thoại và phiên làm việc với xã' nói rõ.",
        "Ứng dụng xin dùng tên Zalo của bạn, nhiều nhất một lần mỗi lần mở, để điền sẵn dòng người gửi khi bạn gửi phản ánh. Ứng dụng nói rõ để làm gì trước, và Zalo chỉ hỏi bạn sau khi bạn đồng ý; bạn có quyền từ chối và tự nhập họ tên. Tên ấy chỉ nằm trên điện thoại này — trong bộ nhớ của ứng dụng, và trong bản nháp phản ánh nếu bạn để lại một bản nháp — và chỉ đi tới xã khi bạn tự bấm gửi phản ánh. Cùng lần lấy tên ấy, ảnh đại diện Zalo của bạn hiện ở màn Cá nhân; ảnh không được gửi đi đâu.",
      ],
    },
    {
      // The own app: `cong-dan/api/mo-phien-vigov.ts` → identity `POST /api/v1/citizen-sessions`, body
      // `communeAppSessionBody` (ADR 0066). The QR path: `App.tsx` `openSharedAppSession` → `vihat-miniapp`
      // `/api/v1/sessions`, body `bridgeBodyWithPhone`; what it forwards and keeps: `vihat-miniapp`
      // `internal/httpapi/sessions_vigov.go:47-140`, `internal/vigovcau/client.go:66-74,151-166`; what the commune
      // system keeps: `service-identity/internal/app/cau_phien_cong_dan.go:296-379`, `store/phien_cong_dan.go:426-437`.
      ma: "phien-lam-viec",
      tieu_de: "Xác nhận số điện thoại và phiên làm việc với xã",
      doan: [
        "Bạn không cần xác nhận gì để xem tin tức hay danh bạ. Ứng dụng chỉ mở phiên làm việc với xã khi bạn làm một việc của riêng mình: gửi phản ánh, xem 'Phản ánh của tôi', tra cứu phiếu, hoặc đánh giá kết quả xử lý. Trước đó, ứng dụng nói rõ vì sao xã cần số điện thoại của bạn; chỉ khi bạn bấm 'Đồng ý chia sẻ số điện thoại', Zalo mới hỏi bạn, và bạn có quyền từ chối.",
        "Zalo không trả số điện thoại của bạn về máy: ứng dụng chỉ nhận những mã dùng được một lần. Số điện thoại của bạn không nằm trong các mã ấy.",
        `TRONG ỨNG DỤNG CỦA XÃ, khi bạn đồng ý, ứng dụng gửi thẳng tới hệ thống của xã đúng những thứ sau, và không gì khác: ${listOf(COMMUNE_APP_SESSION_FIELDS)}. Hệ thống của xã đổi các mã ấy tại Zalo để biết tài khoản Zalo và số điện thoại của bạn, rồi mở phiên làm việc với xã.`,
        `NẾU BẠN MỞ TRANG CỦA XÃ BẰNG MÃ QR HOẶC ĐƯỜNG LIÊN KẾT CỦA XÃ, phiên làm việc được mở qua ${PLATFORM_SERVER}. Khi bạn đồng ý, ứng dụng gửi tới máy chủ ấy đúng những thứ sau, và không gì khác: ${listOf(BRIDGE_FIELDS_WITH_PHONE)}. Ở đây, việc bạn bấm 'Đồng ý chia sẻ số điện thoại' trên trang mang tên xã ở đầu màn hình chính là việc bạn xác nhận làm việc với đúng xã ấy.`,
        "Máy chủ ấy đổi các mã tại Zalo, rồi chuyển sang hệ thống của xã đúng những thứ sau: mã tài khoản Zalo của bạn đối với riêng ứng dụng này; số điện thoại Zalo của bạn; địa chỉ IP và thông tin trình duyệt của máy bạn ở lượt ấy; mã số của ứng dụng; tên miền của xã; và việc bạn đã xác nhận đúng xã. Hệ thống của xã tìm xã theo tên miền, rồi trả về một phiếu phiên làm việc cùng tên xã. Ứng dụng chỉ giữ phiên ấy nếu tên xã trả về trùng với tên xã đang hiện trên màn hình.",
        "Ở lượt ấy, máy chủ của đơn vị cung cấp nền tảng kỹ thuật KHÔNG lưu số điện thoại của bạn và không tạo tài khoản nào ở phía mình: số đi thẳng sang hệ thống của xã. Nếu lượt ấy không thành công, máy chủ ghi một dòng nhật ký — thời điểm, địa chỉ IP, kết quả và mã lý do, không có số điện thoại — và dòng ấy tự bị xoá chậm nhất sau 90 ngày.",
        `HỆ THỐNG CỦA XÃ LƯU: số điện thoại đã xác thực của bạn, làm danh tính của bạn khi làm việc với xã; mã tài khoản Zalo của bạn đối với ứng dụng này, cùng xã bạn làm việc gần nhất; bản ghi của từng phiên làm việc — thời điểm hết hạn, địa chỉ IP và thông tin trình duyệt lúc mở phiên, và một bản mã hoá một chiều của phiếu phiên; và một dòng nhật ký cho mỗi lần mở phiên, kèm địa chỉ IP. Những dữ liệu này được lưu ${SESSION_RETENTION}.`,
        "Phiếu phiên làm việc với xã chỉ nằm trong bộ nhớ của ứng dụng trên máy bạn: không ghi xuống máy, không hiện ra màn hình, và mất đi khi bạn đóng ứng dụng. Lần sau, ứng dụng hỏi lại bạn trước khi mở phiên mới.",
      ],
    },
    {
      // Body: `cong-dan/api/hop-dong-phan-anh.ts` `thanGuiPhanAnh` (every key it can carry has a phrase below —
      // `commune-privacy.test.ts`); anonymous: `service-petitions` domain `AnDanh` (ADR 0008); masking:
      // `service-petitions/internal/http/phieu_phan_anh.go:269-270`; typed contact: ADR 0080; accountless: ADR 0083
      // (`accountlessReportBody`); rating: `RATING_FIELDS`; location triggers: `zalo-api.ts` row `getLocation`.
      ma: "phan-anh",
      tieu_de: "Phản ánh gửi tới xã",
      doan: [
        "Phản ánh chỉ được gửi đi khi chính bạn bấm nút gửi, sau khi đã có phiên làm việc với xã như mục 'Xác nhận số điện thoại và phiên làm việc với xã' nói rõ. Ngay trên màn gửi, ứng dụng ghi rõ tên xã sẽ nhận phản ánh.",
        "KHI BẠN BẤM GỬI, ứng dụng gửi thẳng tới hệ thống tiếp nhận phản ánh của xã đúng những thứ sau, và không gì khác: nội dung phản ánh bạn viết; địa chỉ nơi xảy ra sự việc bạn điền; họ tên và số điện thoại bạn điền trong biểu mẫu; việc bạn có chọn gửi ẩn danh hay không; lĩnh vực bạn chọn, nếu bạn chọn; thôn, tổ dân phố bạn chọn, nếu bạn chọn; toạ độ nơi xảy ra sự việc, nếu ứng dụng đã lấy được vị trí với sự đồng ý của bạn; và, khi có toạ độ, việc bạn có tự chỉnh vị trí trên bản đồ hay không. Kèm theo là phiếu phiên làm việc với xã, để hệ thống biết phản ánh là của ai và gửi tới xã nào; ứng dụng không gửi mã xã hay mã người dùng nào khác.",
        "NẾU ZALO KHÔNG CHO ỨNG DỤNG SỐ ĐIỆN THOẠI CỦA BẠN — vì bạn từ chối chia sẻ, hoặc vì Zalo báo lỗi —, bạn vẫn gửi được phản ánh bằng họ tên và số điện thoại tự nhập. Khi ấy ứng dụng mở phiên làm việc với xã chỉ bằng mã phiên Zalo, không gửi mã số điện thoại; số bạn nhập chỉ để xã liên hệ lại, không được xác nhận, và không nhận thông báo nào về phản ánh; phản ánh gắn với tài khoản Zalo của bạn đối với ứng dụng này, và chỉ tra cứu được bằng mã tra cứu trên chính tài khoản Zalo ấy.",
        "NẾU ZALO KHÔNG CHO ỨNG DỤNG MÃ PHIÊN ZALO — khi ấy ứng dụng không mở được phiên làm việc nào với xã —, bạn vẫn gửi được phản ánh KHÔNG CẦN TÀI KHOẢN. Đây là cách gửi tạm thời, chỉ có cho tới khi ứng dụng được Zalo cấp quyền. Khi bạn bấm gửi theo cách này, ứng dụng gửi thẳng tới hệ thống của xã những thứ của biểu mẫu nói ở trên — trừ ảnh và toạ độ: cách này không gửi kèm được ảnh, và không có nút 'Lấy vị trí hiện tại' nên bạn ghi rõ nơi xảy ra ở ô địa chỉ —, cùng tên miền của xã để hệ thống biết gửi tới xã nào, và một mã ngẫu nhiên chống gửi trùng; KHÔNG kèm phiếu phiên, mã phiên Zalo hay mã người dùng nào. Họ tên và số điện thoại bạn tự nhập chỉ để xã liên hệ lại, không được xác nhận, và không gắn phản ánh với ai; bạn không nhận thông báo nào. Hệ thống của xã dùng địa chỉ mạng (IP) của lần gửi để giới hạn số phản ánh gửi theo cách này. Phản ánh chỉ theo dõi được bằng mã tra cứu: khi tra cứu, ứng dụng gửi mã tra cứu bạn nhập và tên miền của xã. Ai có mã đều xem được tình trạng, thời hạn và kết quả trả lời của phản ánh, nhưng không xem được họ tên, số điện thoại, nội dung hay địa chỉ — nên bạn đừng đưa mã cho người khác.",
        "NẾU BẠN CHỌN GỬI ẨN DANH, ứng dụng KHÔNG gửi họ tên và số điện thoại bạn đã điền, và màn hình của cán bộ không hiện bạn là ai. Nhưng ẩn danh không có nghĩa là hệ thống không biết: hệ thống của xã vẫn ghi phản ánh ấy là của bạn, qua phiên làm việc, để chỉ bạn xem lại được nó và để chống gửi phản ánh tràn lan. Nội dung, địa chỉ, toạ độ và ảnh vẫn được gửi như thường.",
        `VỊ TRÍ NƠI XẢY RA SỰ VIỆC KHÔNG BẮT BUỘC; bạn luôn tự gõ được địa chỉ. Ứng dụng chỉ lấy vị trí khi bạn chọn lĩnh vực phản ánh, khi bạn tiếp tục một phản ánh đang soạn chưa ghi địa chỉ, hoặc khi bạn bấm 'Lấy vị trí hiện tại' — và chỉ khi bạn đồng ý trên hộp thoại của Zalo. Khi ấy ứng dụng gửi tới ${PLATFORM_SERVER} đúng những thứ sau, và không gì khác: ${listOf(LOCATION_FIELDS)}. Máy chủ đổi mã tại Zalo và trả toạ độ về máy bạn; toạ độ hiện lên màn hình, và chỉ tới xã khi bạn bấm gửi phản ánh. Trong ứng dụng riêng của xã, lời gửi này có thêm ${listOf(LOCATION_APP_ID)}, để máy chủ đổi mã bằng đúng khoá của ứng dụng ấy.`,
        "AI XEM ĐƯỢC PHẢN ÁNH: chính bạn, và cán bộ của đúng xã ấy, để tiếp nhận và xử lý. Họ tên và số điện thoại bạn điền được che bớt trên màn hình của cán bộ, trừ cán bộ được giao quyền xem đầy đủ. Bạn thấy tiến độ và kết quả xử lý của phản ánh; ghi chú nội bộ của cán bộ không hiện cho bạn.",
        "Mỗi phản ánh được cấp một MÃ TRA CỨU ngay khi được ghi nhận, và mã ấy hiện lên cho bạn. 'Phản ánh của tôi' và tra cứu phiếu CHỈ MỞ ĐƯỢC PHẢN ÁNH CỦA CHÍNH BẠN: hệ thống nhận ra bạn từ phiên làm việc, không từ một tham số nào ứng dụng gửi lên. Khi xem danh sách, ứng dụng chỉ gửi phiếu phiên và con trỏ sang trang tiếp do chính hệ thống trả; khi tra cứu, ứng dụng gửi mã tra cứu bạn nhập. Mã không có, mã của người khác hay mã của xã khác đều nhận cùng một câu trả lời 'không tìm thấy'.",
        "KHI BẠN ĐÁNH GIÁ KẾT QUẢ XỬ LÝ, ứng dụng gửi mã tra cứu của phản ánh ấy, số sao bạn chọn (từ 1 đến 5) và, nếu bạn viết, lời nhận xét (tối đa 1000 ký tự). Lời nhận xét được gửi tới xã; ứng dụng không hiện lại nó cho bạn sau khi gửi.",
        `Phản ánh và những gì đi kèm — nội dung, họ tên, số điện thoại, toạ độ và ảnh — được lưu ở hệ thống của xã như một hồ sơ hành chính của xã, ${PETITION_RETENTION}.`,
      ],
    },
    {
      // `commune-app/feedback-draft-store.ts` — the one file allowed to touch `localStorage`
      // (`ranh-gioi-hai-nua.test.ts` §3b). `commune-privacy.test.ts` writes a draft through the real store and checks
      // the six keys against the six things named here.
      ma: "ban-nhap",
      tieu_de: "Bản nháp phản ánh trên máy bạn",
      doan: [
        "Khi bạn đang viết một phản ánh, ứng dụng LƯU BẢN NHÁP XUỐNG MÁY BẠN mỗi khi bạn gõ hoặc sửa, để bạn không mất những gì đã viết khi có cuộc gọi đến hay máy hết pin. Đây là thứ duy nhất của bạn mà ứng dụng lưu xuống máy.",
        "Bản nháp gồm đúng sáu thứ: lĩnh vực bạn chọn, nội dung phản ánh, địa chỉ, họ tên, số điện thoại bạn đã điền, và việc bạn có chọn gửi ẩn danh hay không. Họ tên và số điện thoại vẫn nằm trong bản nháp kể cả khi bạn chọn gửi ẩn danh. Ảnh và toạ độ KHÔNG nằm trong bản nháp.",
        "Bản nháp nằm trong phần lưu trữ riêng của ứng dụng trên máy bạn, và không bao giờ được gửi đi đâu. Mỗi xã có một bản nháp riêng: bản nháp viết cho xã này không hiện ra ở trang của một xã khác.",
        "Bản nháp KHÔNG tự hết hạn. Nó chỉ bị xoá khi bạn gửi phản ánh thành công, khi bạn bấm 'Bỏ nháp' lúc ứng dụng hỏi có viết tiếp bản nháp cũ không, hoặc khi bạn bấm 'Huỷ bỏ' để bỏ phản ánh đang viết.",
      ],
    },
    {
      // Built code: `cong-dan/man/scene-photos.tsx`, `zalo-api.ts` (`takeScenePhoto`, `chooseScenePhotos`), upload
      // `cong-dan/api/goi-vigov.ts` `uploadScenePhoto` (ONE multipart request per photo, ADR 0052 §Sửa đổi 09/10/2026);
      // scan, re-encode, EXIF dropped, private bucket, signed links ≤ 15 min, staff views audited (ADR 0047 G3,
      // ADR 0052 §12). The "KHI GỬI MỘT ẢNH" paragraph holds every `SCENE_PHOTO_UPLOAD_FIELDS` declaration verbatim
      // (`commune-privacy.test.ts`). Store operator and retention: owner decision 6.
      ma: "anh-hien-truong",
      tieu_de: "Ảnh hiện trường gửi kèm phản ánh",
      doan: [
        "Khi gửi một phản ánh, bạn CÓ THỂ đính kèm tối đa 5 ảnh hiện trường. Việc này KHÔNG BẮT BUỘC: không có ảnh, phản ánh vẫn gửi được như thường. Ứng dụng chỉ nhận ảnh JPEG, PNG hoặc WebP, không nhận video.",
        "Ảnh chỉ đến từ hai nút, và chỉ khi chính bạn bấm: 'Chụp ảnh' mở máy ảnh của điện thoại, sau khi Zalo hỏi bạn có cho phép dùng máy ảnh hay không; 'Chọn ảnh có sẵn' mở cửa sổ chọn ảnh của Zalo. Ứng dụng hỏi bạn một câu trước khi hộp thoại của Zalo hiện ra. Ứng dụng chỉ nhận những ảnh bạn đã chụp hoặc chọn, không xem các ảnh khác trong máy.",
        "Ảnh chỉ được gửi đi SAU KHI phản ánh của bạn đã được ghi nhận, và chỉ gửi tới hệ thống tiếp nhận phản ánh của xã, mỗi ảnh một lần gửi. Hệ thống ấy kiểm tra ảnh rồi lưu vào kho lưu tệp riêng, không công khai, của hệ thống tiếp nhận phản ánh của xã. Kho ấy do đơn vị cung cấp nền tảng kỹ thuật của ứng dụng vận hành, theo thoả thuận với Ủy ban nhân dân xã. Zalo không tải ảnh lên hộ: ứng dụng không giao cho Zalo một địa chỉ tải lên nào. Ảnh nào tải chưa được thì không làm hỏng phản ánh đã gửi; bạn tải lại ảnh ấy, hoặc chụp, chọn thêm ảnh, ở 'Phản ánh của tôi' trong lúc phản ánh còn ở bước 'Đã tiếp nhận'.",
        "KHI GỬI MỘT ẢNH, ứng dụng gửi tới hệ thống tiếp nhận phản ánh của xã, kèm phiên làm việc của bạn với xã, đúng những thứ sau, và không gì khác: mã tra cứu của phản ánh bạn đã gửi, do chính hệ thống của xã cấp, nằm trên đường dẫn; nó chỉ mở được phản ánh của chính bạn · dung lượng ảnh tính bằng byte · loại ảnh (JPEG, PNG hoặc WebP), đọc từ chính ảnh · ảnh bạn đã chụp hoặc chọn, đúng loại đã khai, dưới một tên cố định là “photo”, không phải tên tệp trên máy bạn; hệ thống của xã bỏ toàn bộ thông tin kèm theo ảnh (như vị trí chụp, thiết bị) trước khi lưu.",
        "Mỗi ảnh đi trong đúng một lần gửi ấy, không qua một địa chỉ tải lên riêng nào, và không có bước gửi nào khác cho ảnh. Tên tệp ảnh trên máy bạn không được gửi đi. Nếu mạng rớt giữa chừng và bạn bấm 'Tải lại', ứng dụng gửi lại đúng ảnh ấy kèm cùng một mã lần gửi, để hệ thống của xã không lưu một ảnh hai lần.",
        "Khi bạn mở một phản ánh của mình để xem lại ảnh đã gửi, ứng dụng chỉ gửi mã tra cứu ấy.",
        "TRƯỚC KHI LƯU, hệ thống quét mã độc trong ảnh, rồi giải mã và mã hoá lại ảnh thành JPEG. Việc mã hoá lại BỎ TOÀN BỘ thông tin kèm theo ảnh (EXIF), kể cả vị trí chụp và thông tin thiết bị. Chỉ bản đã làm sạch được giữ lại; bản bạn gửi lên không được giữ.",
        "AI XEM ĐƯỢC ẢNH: chính bạn, và cán bộ của đúng xã ấy có quyền xem phản ánh. Ảnh không bao giờ được công khai. Mỗi lần xem là qua một đường dẫn có chữ ký, dùng được tối đa 15 phút. Mỗi lần cán bộ xem ảnh đều được ghi lại trong nhật ký của hệ thống: ai xem, và xem lúc nào.",
        "Ảnh không được giữ trên máy bạn: ảnh không nằm trong bản nháp phản ánh, và ứng dụng không lưu ảnh vào thư viện ảnh của máy. Nếu bạn đóng ứng dụng trước khi gửi, bạn chọn lại ảnh.",
        `Ảnh hiện trường là một phần của phản ánh, và được lưu cùng phản ánh, ${PETITION_RETENTION}.`,
      ],
    },
    {
      // One line per call the commune interface makes — `commune-privacy.test.ts` checks every api name of
      // `communeAppCalls(KHAI_BAO_LOI_GOI)` appears here.
      ma: "cac-quyen",
      tieu_de: "Các quyền của Zalo mà ứng dụng dùng",
      doan: [
        "Mỗi quyền dưới đây chỉ được hỏi khi bạn tự bấm vào tính năng cần quyền ấy, và bạn có quyền từ chối: khi đó chỉ tính năng cần quyền ấy không dùng được, phần còn lại của ứng dụng vẫn dùng bình thường.",
        // 12/10/2026 (customer sheet row 7): with the permission already granted in Zalo, the commune app reopens the
        // session at open (`commune-session.ts` `openIfGranted`) — the policy says so, rather than "only on a tap".
        "Số điện thoại (getPhoneNumber) — để xác nhận số điện thoại của bạn với xã ở lần đầu bạn gửi, xem, tra cứu hoặc đánh giá phản ánh, sau khi ứng dụng nói rõ vì sao và bạn đồng ý. Ở những lần mở ứng dụng sau, nếu bạn đã cho phép chia sẻ số điện thoại trong Zalo, ứng dụng xác nhận lại ngay lúc mở để hiện phản ánh của bạn mà không hỏi lại; bạn tắt quyền ấy trong phần cài đặt của Zalo thì ứng dụng lại hỏi như lần đầu. Nhờ số này, xã biết phản ánh là của ai, và chỉ bạn xem được phản ánh của bạn. Ứng dụng chỉ nhận một mã, không nhận số.",
        "Ghi nhớ quyền số điện thoại (authorize, getSetting) — lúc mở ứng dụng và ngay trước khi lấy mã số điện thoại, ứng dụng xem bạn đã cho phép chưa và xin Zalo ghi nhớ quyền bạn vừa đồng ý, để mục quyền của ứng dụng trong Zalo hiện đúng là đã bật. Ứng dụng chỉ xin đúng quyền số điện thoại; bạn tắt được nó bất cứ lúc nào trong phần cài đặt của Zalo.",
        "Thông tin xác thực phiên Zalo (getAccessToken) — đi cùng mã số điện thoại để mở phiên làm việc với xã, và đi cùng mã vị trí khi ứng dụng lấy vị trí nơi xảy ra sự việc. Mã này không cho ứng dụng biết tên hay ảnh đại diện của bạn.",
        "Tên Zalo (getUserInfo) — để điền sẵn dòng người gửi khi bạn gửi phản ánh, và để hiện ảnh đại diện Zalo của bạn ở màn Cá nhân. Tên chỉ đi tới xã khi bạn tự bấm gửi phản ánh; ảnh đại diện không được gửi đi đâu.",
        `Vị trí (getLocation) — để lấy vị trí nơi xảy ra sự việc khi bạn gửi phản ánh. Ứng dụng chỉ nhận một mã; ${PLATFORM_SERVER} đổi mã thành toạ độ và trả toạ độ về máy bạn.`,
        "Máy ảnh (requestCameraPermission, chooseImage) — để bạn chụp ảnh hiện trường gửi kèm một phản ánh, khi chính bạn bấm 'Chụp ảnh'.",
        "Chọn ảnh (openMediaPicker) — để bạn chọn tối đa 5 ảnh hiện trường có sẵn gửi kèm một phản ánh, khi chính bạn bấm 'Chọn ảnh có sẵn'.",
        "Mở trang bên ngoài (openWebview), chia sẻ qua Zalo (openShareSheet) và thu nhỏ ứng dụng (minimizeApp) — chỉ chạy khi chính bạn bấm, và không đọc dữ liệu nào của bạn. Khi bạn chia sẻ một tin, chỉ đường liên kết tới tin ấy đi tới cuộc trò chuyện bạn chọn trong cửa sổ của Zalo.",
      ],
    },
    {
      ma: "noi-nhan",
      tieu_de: "Dữ liệu được gửi tới đâu",
      doan: [
        `Ứng dụng gửi dữ liệu tới đúng những nơi sau, và không nơi nào khác. Thứ nhất, hệ thống của xã: khi ứng dụng mở ra và khi bạn xem tin tức, danh bạ; khi bạn mở phiên làm việc với xã; khi bạn gửi, xem, tra cứu hoặc đánh giá phản ánh, và gửi ảnh hiện trường. Thứ hai, kho ảnh công khai của hệ thống của xã: khi ứng dụng hiện ảnh. Thứ ba, ${PLATFORM_SERVER}: khi ứng dụng lấy vị trí nơi xảy ra sự việc, khi một chức năng của Zalo bạn vừa dùng báo lỗi, và — nếu bạn mở trang của xã bằng mã QR hoặc đường liên kết của xã — khi bạn đồng ý mở phiên làm việc với xã. Thứ tư, Zalo, nền tảng mà ứng dụng chạy trên đó.`,
        // The commune interface's destinations only (`COMMUNE_DESTINATIONS`), counted from the same list the code
        // must name to open anything (`dich-ra-ngoai.test.ts`).
        cauKhaiDichRaNgoai(COMMUNE_DESTINATIONS),
        // The owner's sentence (ADR 0067 Còn mở #6), TRUE BY CONSTRUCTION: `cong-dan/man/leave-app.tsx` asks, then
        // hands the URL to `openWebview` through `moRaNgoai` — no log line, no counter, no call to any server.
        "Liên kết trong bài viết của xã mở ra ngoài ứng dụng, trong trình duyệt của Zalo, sau khi bạn xác nhận; ứng dụng không theo dõi việc bạn bấm liên kết nào hay xem gì ở trang ấy.",
        // The SDK failure report (`zalo-api.ts` `reportSdkFailure` → `vihat-miniapp` `client_errors.go`, which logs one
        // line, no IP, and stores nothing — lines 26-27).
        `KHI MỘT CHỨC NĂNG CỦA ZALO MÀ BẠN VỪA DÙNG BÁO LỖI — ví dụ chia sẻ số điện thoại hay vị trí không thành công —, ứng dụng gửi tới ${PLATFORM_SERVER} đúng những thứ sau, và không gì khác: ${listOf(CLIENT_ERROR_FIELDS)}. Máy chủ chỉ ghi chúng thành một dòng nhật ký kỹ thuật để tìm nguyên nhân lỗi — không kèm địa chỉ IP của bạn — và không lưu chúng vào cơ sở dữ liệu.`,
        "Việc bạn dùng Zalo chịu sự điều chỉnh của chính sách quyền riêng tư của Zalo, nằm ngoài phạm vi văn bản này.",
      ],
    },
    {
      ma: "thoi-gian-luu",
      tieu_de: "Thời gian lưu",
      doan: [
        `Phản ánh và những gì đi kèm — nội dung, họ tên, số điện thoại, toạ độ và ảnh hiện trường — được lưu ${PETITION_RETENTION}.`,
        `Số điện thoại đã xác thực, mã tài khoản Zalo, địa chỉ IP và nhật ký phiên làm việc được lưu ${SESSION_RETENTION}.`,
        "Bản nháp phản ánh trên máy bạn được lưu cho tới khi bạn gửi phản ánh, bấm 'Bỏ nháp' hoặc bấm 'Huỷ bỏ'. Phiếu phiên làm việc và tên Zalo chỉ nằm trong bộ nhớ của ứng dụng, và mất đi khi bạn đóng ứng dụng — trừ tên trong bản nháp, nếu bạn để lại một bản nháp.",
      ],
    },
    {
      ma: "quyen-cua-ban",
      tieu_de: "Quyền của bạn theo Nghị định 13/2023/NĐ-CP",
      doan: [
        "Bạn có quyền được biết, quyền đồng ý và rút lại đồng ý, quyền truy cập, chỉnh sửa, xoá và hạn chế việc xử lý dữ liệu cá nhân của mình, quyền phản đối và quyền khiếu nại.",
        "Bạn rút lại đồng ý cho một quyền bằng cách tắt quyền ấy trong phần cài đặt của Zalo. Bản nháp phản ánh trên máy bạn thì bạn tự xoá bằng nút 'Bỏ nháp' hoặc 'Huỷ bỏ' trên màn gửi phản ánh.",
        `Để xem, sửa hoặc xoá dữ liệu bạn đã gửi, hoặc thực hiện các quyền khác, bạn liên hệ ${authority} theo thông tin tại trang Giới thiệu của xã: ${introductionUrl}`,
      ],
    },
    {
      ma: "rui-ro",
      tieu_de: "Rủi ro có thể xảy ra",
      doan: [
        "Nội dung phản ánh là văn bản tự do. Bạn chỉ cần viết những gì xã cần để xử lý sự việc; đừng ghi số căn cước hay thông tin tài khoản ngân hàng vào đó.",
        "Bản nháp phản ánh nằm trên máy bạn, kể cả họ tên và số điện thoại đã điền, cho tới khi bạn gửi hoặc bỏ nháp. Người khác cầm máy bạn và mở trang của đúng xã ấy sẽ thấy nó.",
        "Toạ độ và ảnh hiện trường có thể cho thấy nơi bạn ở, nếu bạn gửi ngay tại nhà. Cán bộ của xã xem được toạ độ cùng phản ánh, kể cả khi bạn gửi ẩn danh.",
      ],
    },
    {
      ma: "hieu-luc",
      tieu_de: "Hiệu lực và cam kết cập nhật",
      doan: [
        `Chính sách này có hiệu lực từ ngày ${COMMUNE_PRIVACY_EFFECTIVE_DATE}, phiên bản ${COMMUNE_PRIVACY_VERSION}.`,
        `${authority} cập nhật và công bố chính sách này TRƯỚC khi ứng dụng bắt đầu bất kỳ việc thu thập, lưu trữ hoặc truyền dữ liệu nào mà bản hiện tại chưa có.`,
      ],
    },
  ];

  return {
    appLine: `Mini App ${displayName}, ${province}`,
    opening: `Chính sách này áp dụng cho ứng dụng Zalo Mini App của ${lowerFirst(displayName)}, ${province} — kể cả khi bạn mở trang của xã bằng mã QR hoặc đường liên kết của xã. ${authority} là bên chịu trách nhiệm về dữ liệu cá nhân bạn gửi qua ứng dụng. Ứng dụng đọc thông tin công khai của xã ngay khi mở; nó chỉ gửi dữ liệu của bạn đi khi chính bạn làm một việc của mình — như gửi, xem, tra cứu hoặc đánh giá phản ánh; và nó giữ bản nháp phản ánh trên máy bạn cho tới khi bạn gửi hoặc bỏ nháp.`,
    sections,
  };
}
