/**
 * MỞ PHIÊN CÔNG DÂN ViGov SAU KHI CÔNG DÂN XÁC NHẬN XÃ — qua một hàm TIÊM VÀO, không qua một đường nhập.
 *
 * VÌ SAO TIÊM (dependency injection), KHÔNG NHẬP:
 *
 *   Phiên ViGov chỉ mở được qua `vihat-miniapp` (`POST /api/v1/sessions` với `communeHostHint`,
 *   `communeConfirmed` — ADR 0045 · 0047 câu 7), bằng `accessToken` của `zmp-sdk`. `zmp-sdk` chỉ
 *   được nhập trong `features/tinh-nang/`, và nửa nhà nước KHÔNG được nhập nửa thương mại
 *   (`two-halves-boundary.test.ts` §3a, `citizen.test.tsx`). Nên nửa này chỉ KHAI hình dạng của hàm
 *   nó cần (`OpenVigovSession`); `App.tsx` — lớp vỏ, được nhập cả hai nửa — dựng hàm ấy từ client đăng
 *   nhập thương mại và truyền xuống. Không cạnh nhập cấm nào xuất hiện.
 *
 * ⚠ XÃ CHỈ VÀO PHIÊN BẰNG HÀNH VI XÁC NHẬN (ADR 0047 điều kiện dừng #4). `communeConfirmed` là hằng
 *   `true` ở đây vì hàm này CHỈ được gọi từ nút "Đúng, tiếp tục" — không có đường nào gọi nó khi mở app.
 *
 * ⚠ TÊN XÃ CỦA PHIÊN LÀ TÊN MÁY CHỦ TRẢ CÙNG PHIÊN, không phải tên màn xác nhận vừa hiện (ADR 0047
 *   §Trả lời mục 4). Hai tên có thể khác nhau trong một ca hiếm đã được chấp nhận; khi ấy phiên nói thật.
 *
 * ⚠ BEARER KHÔNG ĐI NGƯỢC LÊN. Kết quả trả cho màn hình chỉ có TÊN XÃ; bearer vào `vigov-session.ts` và
 *   chỉ `vigov-client.ts` đọc nó ra.
 */
import { setVigovSession, getVigovSession } from "./vigov-session";
import { isDomain } from "../../lib/launch-params";

/** Thứ nửa nhà nước gửi cho hàm mở phiên. Tên trường là tên trên dây của `vihat-miniapp`. */
export type SessionOpenRequest = {
  readonly communeHostHint: string;
  readonly communeConfirmed: true;
};

/**
 * Bốn nhánh, mỗi nhánh một việc người dân làm tiếp:
 *
 *   `xong`        có phiên ViGov (bearer + tên xã)
 *   `chua-mo`     cầu phiên tắt, xã chưa sẵn sàng, hoặc máy chủ trả một phiên KHÔNG có bearer —
 *                 bấm lại không đổi được gì; về phần giới thiệu kèm một câu
 *   `thu-lai`     mạng, Zalo, hoặc máy chủ bận — bấm lại có thể được
 *   `ngoai-zalo`  không chạy trong Zalo nên không có mã phiên Zalo nào
 */
export type SessionOpenResult =
  | {
      readonly kind: "xong";
      readonly token: string;
      readonly commune_name: string;
      /**
       * Tên miền công khai chính của xã CỦA PHIÊN (`communePrimaryHost`), hoặc `null`. Chỉ làm khoá tra
       * `?host=` cho tin tức và danh bạ — không vào `vigov-session.ts`, không gửi đi làm "xã của tôi".
       */
      readonly domain: string | null;
    }
  | { readonly kind: "chua-mo" }
  | { readonly kind: "thu-lai" }
  | { readonly kind: "ngoai-zalo" };

export type OpenVigovSession = (req: SessionOpenRequest) => Promise<SessionOpenResult>;

/** Kết quả cho màn hình — KHÔNG mang bearer. */
export type ConfirmationResult =
  | { kind: "da-mo"; commune_name: string; domain: string | null }
  | { kind: "chua-mo" }
  | { kind: "thu-lai" }
  | { kind: "ngoai-zalo" };

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * MỞ LẠI PHIÊN KÈM SỐ ĐIỆN THOẠI — khi một tuyến phản ánh trả 403 `chua_xac_thuc_so` (quyết định của
 * người dùng 28/09/2026; ADR 0045:64-65 "khi gửi hồ sơ thì thêm getPhoneNumber()")
 *
 * ⚠ HÀM TIÊM VÀO KHÔNG NHẬN THAM SỐ, VÀ ĐÓ LÀ CHỦ ĐÍCH:
 *
 *   • Mã số điện thoại của Zalo KHÔNG BAO GIỜ vào nửa này. Lớp vỏ xin nó, gửi nó đi trong ĐÚNG MỘT
 *     lời gọi cầu, rồi bỏ; thứ quay về đây chỉ là phiên mới và một cờ `phone_verified`.
 *   • Tên miền gửi làm `communeHostHint` là tên miền công dân ĐÃ XÁC NHẬN ở lần mở này — lớp vỏ giữ
 *     nó (`App.tsx`). Nửa này không có đường nào đưa một tên miền khác vào: không có tham số để đưa.
 *
 * ⚠ XÃ KHÔNG ĐƯỢC ĐỔI LẶNG LẼ (README §Non-negotiables #3): phiên mới chỉ thay phiên cũ khi tên xã
 *   của nó TRÙNG tên xã phiên đang dùng. Lệch là `khac-xa` và phiên cũ giữ nguyên — công dân đã xác
 *   nhận gửi tới một xã, và lần mở lại này không phải một lần xác nhận mới.
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** Kết quả hàm mở lại mà lớp vỏ tiêm vào. Không mang mã số điện thoại. */
export type ReopenWithPhoneResult =
  | {
      readonly kind: "xong";
      readonly token: string;
      readonly commune_name: string;
      /** `phoneVerified` của phiên mới, nguyên văn máy chủ trả. */
      readonly phone_verified: boolean;
    }
  /** Công dân bấm "Từ chối" trên hộp thoại của Zalo — không lời gọi cầu nào đi ra. */
  | { readonly kind: "tu-choi" }
  | { readonly kind: "chua-mo" }
  | { readonly kind: "thu-lai" }
  | { readonly kind: "ngoai-zalo" };

export type ReopenWithPhone = () => Promise<ReopenWithPhoneResult>;

/**
 * Kết quả cho màn hình — KHÔNG mang bearer, không mang gì của số điện thoại.
 *
 *   `da-xac-thuc`        phiên mới có số đã xác thực, đã GHI; màn gọi lại việc cũ đúng một lần
 *   `van-chua-xac-thuc`  máy chủ mở lại được nhưng vẫn không xác thực được số — hỏi lại không đổi gì
 *   `khac-xa`            phiên mới thuộc một xã khác xã đang làm việc — KHÔNG ghi
 *   `tu-choi` · `chua-mo` · `thu-lai` · `ngoai-zalo` — như tên
 */
export type PhoneVerificationOutcome =
  | { kind: "da-xac-thuc" }
  | { kind: "van-chua-xac-thuc" }
  | { kind: "khac-xa" }
  | { kind: "tu-choi" }
  | { kind: "chua-mo" }
  | { kind: "thu-lai" }
  | { kind: "ngoai-zalo" };

/**
 * Gọi hàm mở lại đã tiêm, và chỉ ghi phiên mới khi nó SỬA ĐƯỢC việc: có bearer, cùng xã, đã xác thực số.
 *
 * Không ném ra ngoài. Không có phiên hiện tại thì không có gì để "mở lại" — `chua-mo`, không gọi.
 */
export async function reopenSessionWithPhone(reopen: ReopenWithPhone): Promise<PhoneVerificationOutcome> {
  const current = getVigovSession();
  if (current === null) return { kind: "chua-mo" };
  let result: ReopenWithPhoneResult;
  try {
    result = await reopen();
  } catch {
    return { kind: "thu-lai" };
  }
  if (result.kind !== "xong") return { kind: result.kind };
  if (result.token === "" || result.commune_name.trim() === "") return { kind: "chua-mo" };
  if (result.commune_name !== current.commune_name) return { kind: "khac-xa" };
  if (!result.phone_verified) return { kind: "van-chua-xac-thuc" };
  setVigovSession({ token: result.token, commune_name: result.commune_name });
  return { kind: "da-xac-thuc" };
}

/**
 * Gọi hàm mở phiên đã tiêm với tên miền công dân vừa xác nhận, rồi ghi phiên vào bộ nhớ.
 *
 * Không ném ra ngoài: một hàm tiêm vào ném lỗi là `thu-lai`, không phải một màn hình đứng im. Một
 * `xong` thiếu bearer hoặc thiếu tên xã là `chua-mo` — KHÔNG BAO GIỜ giả vờ đã có phiên.
 */
export async function openSessionAfterConfirmation(open: OpenVigovSession, domain: string): Promise<ConfirmationResult> {
  let result: SessionOpenResult;
  try {
    result = await open({ communeHostHint: domain, communeConfirmed: true });
  } catch {
    return { kind: "thu-lai" };
  }
  if (result.kind !== "xong") return { kind: result.kind };
  if (result.token === "" || result.commune_name.trim() === "") return { kind: "chua-mo" };
  setVigovSession({ token: result.token, commune_name: result.commune_name });
  // KIỂM KHUÔN LẠI Ở ĐÂY dù nửa thương mại đã kiểm: hàm này nhận từ BÊN NGOÀI nửa nhà nước, và một tên
  // miền sai khuôn đi vào `?host=` là để máy chủ phân tích một thứ không phải tên miền.
  const session_domain = typeof result.domain === "string" && isDomain(result.domain) ? result.domain : null;
  return { kind: "da-mo", commune_name: result.commune_name, domain: session_domain };
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * APP RIÊNG CỦA MỘT XÃ — MỞ PHIÊN Ở VIỆC CÁ NHÂN ĐẦU TIÊN (ADR 0047:251 · ADR 0045:148-176, 311)
 *
 * Không đăng nhập lúc mở app. Phiên mở khi công dân làm việc cá nhân đầu tiên — gửi phản ánh, xem phản
 * ánh của mình, tra cứu phiếu của mình, chấm sao — SAU lời giải thích và cú bấm đồng ý, vì lần mở nào của
 * app riêng cũng xin số điện thoại (`vihat-miniapp` xác minh App ID bằng lượt đổi số).
 *
 * ⚠ HÀM TIÊM VÀO KHÔNG NHẬN THAM SỐ, cùng lý do `ReopenWithPhone`: App ID, hai mã Zalo và số điện thoại
 *   không bao giờ vào nửa này. Không có tên miền nào để gửi — xã do máy chủ tra từ App ID.
 *
 * ⚠ CÙNG XÃ VỚI ĐẦU MÀN HÌNH, HOẶC KHÔNG GHI. Header của app riêng đọc tên xã từ tuyến công khai theo
 *   tên miền của bản dựng; phiên đọc tên xã từ dòng `mini_app` của App ID. Hai nguồn ấy phải nói CÙNG một
 *   xã (cả hai là `Name` của tenant — `service-identity/internal/app/cau_phien_cong_dan.go:362` và
 *   `internal/http/danh_muc_xa.go`). Lệch là `khac-xa`: phiên KHÔNG được ghi, vì mọi phản ánh gửi bằng nó
 *   sẽ tới một xã khác xã công dân đang nhìn thấy (README §Non-negotiables #2, #5).
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** Kết quả hàm mở phiên của app riêng mà lớp vỏ tiêm vào. Không mang App ID, mã Zalo hay số điện thoại. */
export type CommuneAppSessionResult =
  | {
      readonly kind: "xong";
      readonly token: string;
      readonly commune_name: string;
      /** `phoneVerified` của phiên, nguyên văn máy chủ trả. */
      readonly phone_verified: boolean;
    }
  /** Công dân bấm "Từ chối" trên hộp thoại xin số của Zalo — không lời gọi nào đi ra. */
  | { readonly kind: "tu-choi" }
  /** Ứng dụng của xã chưa được kết nối (App ID lạ với máy chủ, chưa gắn xã, xã ngừng) — bấm lại vô ích. */
  | { readonly kind: "chua-ket-noi" }
  /** Hệ thống nhận phản ánh qua ứng dụng đang tạm ngưng — thử lại sau. */
  | { readonly kind: "tam-ngung" }
  /** Zalo chưa trả lời, hoặc thử quá nhiều lần — chờ một lát rồi bấm lại (bấm ngay cũng hỏng y vậy). */
  | { readonly kind: "cho-lat" }
  /** Mạng, mã Zalo quá hạn — bấm lại ngay có thể được. */
  | { readonly kind: "thu-lai" }
  | { readonly kind: "ngoai-zalo" };

export type OpenCommuneAppSession = () => Promise<CommuneAppSessionResult>;

/**
 * Kết quả cho màn hình — KHÔNG mang bearer.
 *
 *   `da-mo`               phiên đã ghi; màn làm tiếp việc cá nhân
 *   `khac-xa`             phiên thuộc xã khác xã trên đầu màn hình — KHÔNG ghi
 *   `chua-xac-thuc-so`    máy chủ mở được nhưng không xác thực được số — hỏi lại không đổi gì
 *   `chua-mo`             máy chủ trả phiên không dùng được (thiếu bearer / tên xã)
 *   còn lại               như `CommuneAppSessionResult`
 */
export type CommuneAppSessionOutcome =
  | { readonly kind: "da-mo" }
  | { readonly kind: "khac-xa" }
  | { readonly kind: "chua-xac-thuc-so" }
  | { readonly kind: "chua-mo" }
  | Exclude<CommuneAppSessionResult, { kind: "xong" }>;

/**
 * Gọi hàm mở phiên đã tiêm, rồi ghi phiên CHỈ KHI nó dùng được: có bearer, có tên xã, TRÙNG `shownCommuneName`
 * (tên xã đang hiện trên đầu màn hình), và đã xác thực số (mọi tuyến phản ánh đòi số — một phiên chưa xác
 * thực chỉ dẫn tới 403 `chua_xac_thuc_so` ngay sau đó).
 *
 * Không ném ra ngoài: hàm tiêm ném lỗi là `thu-lai`.
 */
export async function openCommuneAppSession(
  open: OpenCommuneAppSession,
  shownCommuneName: string,
): Promise<CommuneAppSessionOutcome> {
  let result: CommuneAppSessionResult;
  try {
    result = await open();
  } catch {
    return { kind: "thu-lai" };
  }
  if (result.kind !== "xong") return result;
  if (result.token === "" || result.commune_name.trim() === "") return { kind: "chua-mo" };
  if (shownCommuneName.trim() === "" || result.commune_name !== shownCommuneName) return { kind: "khac-xa" };
  if (!result.phone_verified) return { kind: "chua-xac-thuc-so" };
  setVigovSession({ token: result.token, commune_name: result.commune_name });
  return { kind: "da-mo" };
}

/**
 * Quên phiên của app riêng — khi một tuyến phản ánh trả 401 (phiên hết hạn / bị thu hồi) hoặc 403
 * `chua_xac_thuc_so`. Việc cá nhân kế tiếp lại đi qua cổng (`commune-session.ts`): lời giải thích, cú bấm
 * đồng ý, rồi một phiên MỚI kèm số. Giữ một bearer máy chủ đã từ chối là để mọi lần bấm sau hỏng y hệt.
 */
export function dropCommuneAppSession(): void {
  setVigovSession(null);
}

/**
 * Hàm mở phiên của app riêng, nhìn như một `ReopenWithPhone` — cho lần 403 `chua_xac_thuc_so` (và cho
 * `usePhoneVerification` của các màn dùng chung). Thân app riêng luôn mang số, nên "mở lại kèm số" chính là
 * mở phiên lần nữa; `reopenSessionWithPhone` vẫn là bên so tên xã với phiên đang dùng và quyết định ghi.
 */
export function communeAppReopen(open: OpenCommuneAppSession): ReopenWithPhone {
  return async () => {
    const result = await open();
    switch (result.kind) {
      case "xong":
      case "tu-choi":
      case "thu-lai":
      case "ngoai-zalo":
        return result;
      case "cho-lat":
        return { kind: "thu-lai" };
      case "chua-ket-noi":
      case "tam-ngung":
        return { kind: "chua-mo" };
    }
  };
}
