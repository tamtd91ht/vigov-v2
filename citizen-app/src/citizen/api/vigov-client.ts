/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC ĐƯỢC GỌI MẠNG — và nó chỉ nói chuyện với ViGov.
 *
 * `phase1-collects-nothing.test.ts` miễn lệnh cấm `fetch` cho ĐÚNG những tệp được kê tên; tệp này
 * là tệp thứ ba (24/09/2026). `citizen-report-contract.ts` nằm ngay cạnh và vẫn bị cấm.
 *
 * ⚠ HAI CỔNG ĐỨNG TRƯỚC MỌI LỜI GỌI, và cổng PHIÊN mở trước — in practice that gate is still shut:
 *
 *   1. `getVigovSession()` trả `null`  → `chua-co-phien`, KHÔNG gọi mạng. The bridge is wired end to end
 *      (`vihat-miniapp` forwards `vigovSession`, `open-vigov-session.ts` records it), but `vihat-miniapp`'s
 *      Zalo account-id step always refuses (`internal/zalo/ma_tai_khoan.go:45-47`, ADR 0045 UNKNOWN #2)
 *      → 503, so no session is issued yet (`vigov-session.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`).
 *   2. Địa chỉ ViGov rỗng            → `chua-cau-hinh`, KHÔNG gọi mạng (`vigov-address.ts`).
 *      Từ 26/09/2026 host của `petitions` đã có (ADR 0046), nên cổng này MỞ; chỉ cổng 1 còn giữ
 *      mọi yêu cầu lại. Thứ tự hai cổng vì thế là bất biến: có host mà không có phiên thì vẫn
 *      không gửi gì.
 *
 * ⚠ BA TUYẾN CÔNG KHAI (27/09/2026) — tra xã theo tên miền, danh bạ cán bộ, tin của xã — KHÔNG đi
 *   qua cổng phiên và KHÔNG mang bearer: chúng chỉ trả thứ xã đã công bố (`public-contract.ts`).
 *   Cổng của chúng là TÊN MIỀN: không đúng khuôn (`lib/launch-params.ts` `isDomain`) thì không gọi.
 *   Chúng đi qua CÙNG MỘT `fetch(` ở `call` bên dưới — tệp này vẫn chỉ có một chỗ gọi mạng.
 *
 * ⚠ BEARER CHỈ ĐẾN TỪ `getVigovSession()`. Không hàm nào ở đây nhận token qua tham số — một tham số
 * là một khe để ai đó nhét phiếu phiên của `vihat-miniapp` vào, và phiếu ấy KHÔNG phải phiên ViGov.
 *
 * ⚠ KHÔNG `console.*`, KHÔNG IN THÂN YÊU CẦU HAY PHẢN HỒI. Chúng mang nội dung phản ánh, họ tên và
 * số điện thoại của người thật (luật 3, bất biến 1).
 */
import {
  type CitizenField,
  citizenFieldsAddress,
  listAddress,
  submitAddress,
  lookupAddress,
  readReport,
  readMyReportsPage,
  errorCode,
  FIELD_CATALOGUE_UNAVAILABLE_CODE,
  FIELD_NOT_OFFERED_CODE,
  isPhoneNotVerified,
  type MyReport,
  ratingAddress,
  readCitizenFields,
  type MyReportsPage,
} from "./citizen-report-contract";
import {
  type CommuneNewsArticle,
  type PublicStaff,
  type CommuneProfile,
  communeProfilesAddress,
  newsArticleAddress,
  directoryAddress,
  communeNewsAddress,
  communeLookupAddress,
  readNewsArticle,
  readDirectory,
  readCommuneNewsPage,
  readCommune,
  type NewsType,
  readCommuneProfiles,
  type CommuneNewsPage,
  type FoundCommune,
} from "./public-contract";
import type { SendAttempt } from "./send-attempt";
import { getVigovSession } from "./vigov-session";
import { isDomain } from "../../lib/launch-params";

/**
 * Mỗi nhánh là một VIỆC NGƯỜI DÂN PHẢI LÀM khác nhau.
 *
 *   `chua-co-phien` · `chua-cau-hinh`  kênh chưa mở trên ứng dụng — không gửi gì đi cả
 *   `xong`                              201 / 200, có phiếu
 *   `khong-thay`                        404 — MỘT câu cho "không có", "của người khác", "xã khác"
 *   `het-phien`                         401 — phiên không dùng được
 *   `can-xac-thuc-so`                   403 `chua_xac_thuc_so` — phiên chưa có số điện thoại đã xác
 *                                       thực; việc tiếp theo là HỎI công dân (`screens/phone-verification.tsx`).
 *                                       403 với mã khác là `loi-may-chu`: xin số không sửa được nó
 *   `dang-xu-ly-truoc`                 409 — lần gửi trước (cùng khoá) còn đang chạy; chờ rồi gửi lại
 *   `field-not-offered`                 400 `field_not_offered` — lĩnh vực đã gửi không còn trong danh mục
 *                                       xã mở; việc tiếp theo: tải lại danh mục, chọn lại (af3fff0)
 *   `khong-hop-le`                      400 với mã khác
 *   `field-catalogue-unavailable`       503 `field_catalogue_unavailable` — chưa đọc được bộ mã lĩnh vực;
 *                                       tự hết, thử lại sau ít phút; phiếu CHƯA được ghi nhận
 *   `kenh-chua-mo`                      503 khác — xã chưa cấu hình hạn; phiếu CHƯA được ghi nhận
 *   `loi-may-chu`                       500, mã lạ, hoặc thân sai khuôn
 *   `loi-mang`                          mất mạng, quá hạn chờ
 *
 * KHÔNG MANG CÂU CỦA MÁY CHỦ: câu 400 của `petitions` nói bằng tên trường kỹ thuật (`content`), và
 * màn hình người dân có câu riêng cho từng nhánh (`screens/copy.ts`).
 */
export type CallResult =
  | { kind: "chua-co-phien" }
  | { kind: "chua-cau-hinh" }
  | { kind: "xong"; report: MyReport }
  | { kind: "khong-thay" }
  | { kind: "het-phien" }
  | { kind: "can-xac-thuc-so" }
  | { kind: "dang-xu-ly-truoc" }
  | { kind: "field-not-offered" }
  | { kind: "khong-hop-le" }
  | { kind: "field-catalogue-unavailable" }
  | { kind: "kenh-chua-mo" }
  | { kind: "loi-may-chu" }
  | { kind: "loi-mang" };

/** Mọi nhánh trừ `xong` — chung cho mọi tuyến. */
type FailureBranch = Exclude<CallResult, { kind: "xong" }>;

/**
 * Kết quả của tuyến "Phản ánh của tôi". Cùng các nhánh dừng với `CallResult`; hợp đồng không có
 * 404/409/503 cho tuyến này, nên gặp chúng là `loi-may-chu` (xem `myReports`).
 */
export type ListResult = { kind: "xong"; page: MyReportsPage } | FailureBranch;

/** Quá hạn chờ. Người dân đang cầm máy đứng chờ; thà nói "thử lại" sớm. */
const WAIT_LIMIT_MS = 20_000;

/** Hai cổng đóng. Trả về phiên + địa chỉ, hoặc nhánh dừng. */
function openGate(address: string): { token: string; address: string } | FailureBranch {
  const session = getVigovSession();
  if (session === null || session.token === "") return { kind: "chua-co-phien" };
  if (address === "") return { kind: "chua-cau-hinh" };
  return { token: session.token, address };
}

/**
 * CHỖ DUY NHẤT GỌI `fetch` — mọi tuyến đi qua đây (`bundle-for-zalo.test.ts` đếm đúng một `fetch(`
 * của nửa này). `read` đọc thân 200/201; `null` là sai khuôn.
 */
async function call<T>(
  address: string,
  /** `token` VẮNG MẶT chỉ ở ba tuyến công khai — xem khối đầu tệp. */
  options: { method: "GET" | "POST"; token?: string; key?: string; body?: string },
  read: (body: unknown) => T | null,
  on_404: FailureBranch,
): Promise<{ kind: "xong"; value: T } | FailureBranch> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), WAIT_LIMIT_MS);

  const headers: Record<string, string> = { Accept: "application/json" };
  if (options.token !== undefined) headers["Authorization"] = `Bearer ${options.token}`;
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (options.key !== undefined) headers["Idempotency-Key"] = options.key;

  try {
    const response = await fetch(address, {
      method: options.method,
      headers,
      body: options.body,
      signal: controller.signal,
    });

    switch (response.status) {
      case 200:
      case 201: {
        const value = read(await response.json());
        return value === null ? { kind: "loi-may-chu" } : { kind: "xong", value };
      }
      case 400:
        return (await readErrorCode(response)) === FIELD_NOT_OFFERED_CODE
          ? { kind: "field-not-offered" }
          : { kind: "khong-hop-le" };
      case 401:
        return { kind: "het-phien" };
      case 403: {
        // Thân hỏng ở đây KHÔNG được rơi xuống `catch` bên dưới: đó là nhánh `loi-mang`, và bảo người
        // dân "kiểm tra mạng" cho một câu trả lời máy chủ đã gửi tới nơi là nói sai việc cần làm.
        let body: unknown = null;
        try {
          body = await response.json();
        } catch {
          return { kind: "loi-may-chu" };
        }
        return isPhoneNotVerified(body) ? { kind: "can-xac-thuc-so" } : { kind: "loi-may-chu" };
      }
      case 404:
        return on_404;
      case 409:
        return { kind: "dang-xu-ly-truoc" };
      case 503:
        return (await readErrorCode(response)) === FIELD_CATALOGUE_UNAVAILABLE_CODE
          ? { kind: "field-catalogue-unavailable" }
          : { kind: "kenh-chua-mo" };
      default:
        return { kind: "loi-may-chu" };
    }
  } catch {
    return { kind: "loi-mang" };
  } finally {
    clearTimeout(timer);
  }
}

/**
 * The `code` of an error body, or `null` when the body is absent or not JSON. Never throws: a server
 * that answered with a status HAS answered — a broken body must not become `loi-mang` ("check your
 * network"), which would tell the citizen to fix the wrong thing. Only `code` is read (`errorCode`).
 */
async function readErrorCode(response: Response): Promise<string | null> {
  try {
    return errorCode(await response.json());
  } catch {
    return null;
  }
}

/**
 * THE COMMUNE'S FIELD CATALOGUE for a new petition — `GET /api/v1/my-citizen-report-fields`. Session
 * needed (the commune comes from it); a verified phone is not. No parameter: nothing about "which commune"
 * leaves the phone (rule 1, forbidden #2). 503 `field_catalogue_unavailable` is its own branch — there is
 * NO fallback list anywhere in this app (ADR 0060 §3).
 */
export async function citizenReportFields(): Promise<
  { kind: "xong"; fields: readonly CitizenField[] } | FailureBranch
> {
  const gate = openGate(citizenFieldsAddress());
  if ("kind" in gate) return gate;
  const result = await call(gate.address, { method: "GET", token: gate.token }, readCitizenFields, { kind: "loi-may-chu" });
  return result.kind === "xong" ? { kind: "xong", fields: result.value } : result;
}

/** `value` của lớp gọi → nhánh `xong` của một phiếu. */
function toReportResult(result: { kind: "xong"; value: MyReport } | FailureBranch): CallResult {
  return result.kind === "xong" ? { kind: "xong", report: result.value } : result;
}

/**
 * Gửi MỘT lần gửi. Gọi lại với CÙNG `attempt` là "gửi lại" — cùng thân, cùng `Idempotency-Key`.
 * Xem `send-attempt.ts`. Không ném ra ngoài.
 */
export async function submitReport(attempt: SendAttempt): Promise<CallResult> {
  const gate = openGate(submitAddress());
  if ("kind" in gate) return gate;
  return toReportResult(
    await call(
      gate.address,
      { method: "POST", token: gate.token, key: attempt.key, body: attempt.body },
      readReport,
      // Tuyến gửi không có 404 trong hợp đồng; gặp nó là tuyến chưa được định tuyến ở cụm.
      { kind: "loi-may-chu" },
    ),
  );
}

/**
 * Tra MỘT phiếu của chính mình theo mã tra cứu.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI": máy chủ lấy người gửi từ phiên (luật 4, cấm #1). Mã rỗng thì trả
 * `khong-thay` mà không gọi — một đoạn đường dẫn rỗng không khớp phiếu nào.
 */
export async function lookupReport(lookup_code: string): Promise<CallResult> {
  const code = lookup_code.trim();
  const gate = openGate(lookupAddress(code === "" ? "x" : code));
  if ("kind" in gate) return gate;
  if (code === "") return { kind: "khong-thay" };
  return toReportResult(
    await call(gate.address, { method: "GET", token: gate.token }, readReport, { kind: "khong-thay" }),
  );
}

/**
 * MỘT TRANG "PHẢN ÁNH CỦA TÔI". `cursor` rỗng = trang đầu; trang sau truyền NGUYÊN VĂN `cursor`
 * của trang trước.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI", "XÃ NÀO": công dân và xã đều lấy từ phiên ở máy chủ (luật 4, cấm #1;
 * luật 1, cấm #2). Hợp đồng không có 404/409/503 cho tuyến này — gặp chúng là tuyến lạc ở cụm,
 * nên đổi thành `loi-may-chu` thay vì mượn câu của tuyến gửi ("lần gửi trước đang chạy") vốn
 * không đúng với một lần XEM.
 */
export async function myReports(cursor: string): Promise<ListResult> {
  const gate = openGate(listAddress(cursor));
  if ("kind" in gate) return gate;
  const result = await call(gate.address, { method: "GET", token: gate.token }, readMyReportsPage, {
    kind: "loi-may-chu",
  });
  if (result.kind === "xong") return { kind: "xong", page: result.value };
  if (result.kind === "dang-xu-ly-truoc" || result.kind === "kenh-chua-mo") return { kind: "loi-may-chu" };
  return result;
}

/**
 * The citizen rates THEIR OWN petition `lookup_code` (ADR 0050 point 2). `attempt` is one user act — body
 * from `ratingBody`, one `Idempotency-Key` — and calling again with the SAME attempt is "send again" after a
 * lost answer: the server replays the first result instead of recording a second rating (`send-attempt.ts`).
 *
 * NO "WHOSE" OR "WHICH COMMUNE" PARAMETER: both come from the session on the server (rule 4, forbidden #1;
 * rule 1, forbidden #2). 200 is the petition as it now stands (same shape as the GET), so the screen
 * re-renders from it — including a status the server changed.
 *
 * 409 IS `dang-xu-ly-truoc` HERE AND MEANS TWO THINGS: `petition_state` (not rateable in this status, or it
 * changed meanwhile) and idem's "the same key is still in flight". Both have the same next step for the
 * citizen — reload the petition — so the screen does not need to tell them apart. 503 is not in this
 * route's contract and becomes `loi-may-chu`, as on the list route.
 */
export async function rateReport(lookup_code: string, attempt: SendAttempt): Promise<CallResult> {
  const code = lookup_code.trim();
  const gate = openGate(ratingAddress(code === "" ? "x" : code));
  if ("kind" in gate) return gate;
  if (code === "") return { kind: "khong-thay" };
  const result = toReportResult(
    await call(
      gate.address,
      { method: "POST", token: gate.token, key: attempt.key, body: attempt.body },
      readReport,
      { kind: "khong-thay" },
    ),
  );
  return result.kind === "kenh-chua-mo" ? { kind: "loi-may-chu" } : result;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * BA TUYẾN CÔNG KHAI THEO TÊN MIỀN XÃ — không phiên, không bearer
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Mỗi nhánh là một câu khác nhau trên màn:
 *
 *   `xong`          200, đúng khuôn
 *   `khong-hop-le`  tên miền sai khuôn (KHÔNG gọi mạng) hoặc 400
 *   `khong-thay`    404 — chỉ tuyến chi tiết tin có
 *   `tam-ngung`     503 — nền tảng không phân giải được lúc này
 *   `loi-may-chu`   500, mã lạ, thân sai khuôn
 *   `loi-mang`      mất mạng, quá hạn chờ
 *   `chua-cau-hinh` bảng host thiếu dòng — không gọi
 */
export type PublicResult<T> =
  | { kind: "xong"; value: T }
  | { kind: "khong-hop-le" }
  | { kind: "khong-thay" }
  | { kind: "tam-ngung" }
  | { kind: "loi-may-chu" }
  | { kind: "loi-mang" }
  | { kind: "chua-cau-hinh" };

async function callPublic<T>(
  domain: string,
  address: (domain: string) => string,
  read: (body: unknown) => T | null,
): Promise<PublicResult<T>> {
  // Cổng TÊN MIỀN đứng trước cổng địa chỉ: không có tên miền đúng khuôn thì không một byte nào đi ra.
  if (!isDomain(domain)) return { kind: "khong-hop-le" };
  const url = address(domain);
  if (url === "") return { kind: "chua-cau-hinh" };
  const result = await call(url, { method: "GET" }, read, { kind: "khong-thay" });
  switch (result.kind) {
    case "xong":
    case "khong-hop-le":
    case "khong-thay":
    case "loi-may-chu":
    case "loi-mang":
      return result;
    case "kenh-chua-mo":
      return { kind: "tam-ngung" };
    default:
      // 401 / 409 không có trong hợp đồng của ba tuyến công khai: tuyến lạc ở cụm.
      return { kind: "loi-may-chu" };
  }
}

/**
 * Tên và tỉnh của xã ứng với tên miền trên QR — cho màn xác nhận. Mảng rỗng là "không xã nào".
 * Tên miền là KHOÁ TRA; kết quả không cấp gì, và không được nhớ làm xã của phiên.
 */
export function lookupCommuneByDomain(domain: string): Promise<PublicResult<readonly FoundCommune[]>> {
  return callPublic(domain, communeLookupAddress, readCommune);
}

/** Danh bạ cán bộ xã đã công khai. Không ghi log gì: danh bạ mang số di động cá nhân. */
export function communeStaffDirectory(domain: string): Promise<PublicResult<readonly PublicStaff[]>> {
  return callPublic(domain, directoryAddress, readDirectory);
}

/**
 * Trụ sở, đường dây nóng, giờ làm việc mà xã đã khai (`CommuneProfile`). Công khai theo tên miền, không
 * phiên. Không có logo — xem `public-contract.ts`.
 */
export function communeProfiles(domain: string): Promise<PublicResult<readonly CommuneProfile[]>> {
  return callPublic(domain, communeProfilesAddress, readCommuneProfiles);
}

/**
 * Một trang tin của xã, mới nhất trước. `cursor` rỗng = trang đầu. `type` lọc ở máy chủ theo loại tin
 * (`NewsType`); không truyền = mọi loại, như trước.
 */
export function communeNews(
  domain: string,
  cursor: string,
  type: NewsType | null = null,
): Promise<PublicResult<CommuneNewsPage>> {
  return callPublic(domain, (d) => communeNewsAddress(d, cursor, type), readCommuneNewsPage);
}

/** Toàn văn một tin. 404 là MỘT câu: tin chưa đăng, đã gỡ, hay của xã khác trả như nhau. */
export function communeNewsArticle(domain: string, id: string): Promise<PublicResult<CommuneNewsArticle>> {
  if (id === "") return Promise.resolve({ kind: "khong-thay" });
  return callPublic(domain, (d) => newsArticleAddress(d, id), readNewsArticle);
}
