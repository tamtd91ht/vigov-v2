/**
 * TỆP DUY NHẤT CỦA NỬA NHÀ NƯỚC ĐƯỢC GỌI MẠNG — và nó chỉ nói chuyện với ViGov.
 *
 * `phase1-collects-nothing.test.ts` miễn lệnh cấm `fetch` cho ĐÚNG những tệp được kê tên; tệp này
 * là tệp thứ ba (24/09/2026). `hop-dong-phan-anh.ts` nằm ngay cạnh và vẫn bị cấm.
 *
 * ⚠ HAI CỔNG ĐỨNG TRƯỚC MỌI LỜI GỌI, và cổng PHIÊN mở trước:
 *
 *   1. `layPhienViGov()` trả `null`  → `chua-co-phien`, KHÔNG gọi mạng. The bridge is wired end to end
 *      (`vihat-miniapp` forwards `vigovSession`, `mo-phien-vigov.ts` records it). Since `vihat-miniapp`
 *      4114f00 its Zalo account-id step is a REAL call (`LayMaTaiKhoan`, `GET graph.zalo.me/v2.0/me?fields=id`,
 *      `internal/zalo/ma_tai_khoan.go`) instead of a step that always refused with 503 — but that call has
 *      not yet been measured against real Zalo, so whether a session is issued on a real phone is unknown
 *      (`phien-vigov.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`).
 *   2. Địa chỉ ViGov rỗng            → `chua-cau-hinh`, KHÔNG gọi mạng (`dia-chi-vigov.ts`).
 *      Từ 26/09/2026 host của `petitions` đã có (ADR 0046), nên cổng này MỞ; chỉ cổng 1 còn giữ
 *      mọi yêu cầu lại. Thứ tự hai cổng vì thế là bất biến: có host mà không có phiên thì vẫn
 *      không gửi gì.
 *
 * ⚠ BA TUYẾN CÔNG KHAI (27/09/2026) — tra xã theo tên miền, danh bạ cán bộ, tin của xã — KHÔNG đi
 *   qua cổng phiên và KHÔNG mang bearer: chúng chỉ trả thứ xã đã công bố (`hop-dong-cong-khai.ts`).
 *   Cổng của chúng là TÊN MIỀN: không đúng khuôn (`lib/launch-params.ts` `laTenMien`) thì không gọi.
 *   Chúng đi qua CÙNG MỘT `fetch(` ở `goi` bên dưới — tệp này vẫn chỉ có một chỗ gọi mạng.
 *
 * ⚠ SCENE PHOTOS (02/10/2026, commune app only) add the ONLY two `fetch(` outside `goi`, each for a reason
 *   `goi` cannot serve: `readPickedPhoto` reads the picker's LOCAL temp file (no server at all), and
 *   `postPhotoToStorage` posts the bytes to the presigned object-store URL ViGov handed out — no bearer, no
 *   cookie, the form being the credential. Every ViGov route still goes through `goi`.
 *
 * ⚠ BEARER CHỈ ĐẾN TỪ `layPhienViGov()`. Không hàm nào ở đây nhận token qua tham số — một tham số
 * là một khe để ai đó nhét phiếu phiên của `vihat-miniapp` vào, và phiếu ấy KHÔNG phải phiên ViGov.
 *
 * ⚠ KHÔNG `console.*`, KHÔNG IN THÂN YÊU CẦU HAY PHẢN HỒI. Chúng mang nội dung phản ánh, họ tên và
 * số điện thoại của người thật (luật 3, bất biến 1). A failed call is reported ONLY through
 * `connection-log.ts` — fixed shape, `--demo` build only, no body (owner, 01/10/2026).
 */
import {
  type CitizenField,
  citizenFieldsAddress,
  diaChiDanhSach,
  diaChiGuiPhanAnh,
  diaChiTraCuu,
  docPhieu,
  docTrangPhieuCuaToi,
  errorCode,
  FIELD_CATALOGUE_UNAVAILABLE_CODE,
  FIELD_NOT_OFFERED_CODE,
  isPhoneNotVerified,
  type PhieuCuaToi,
  photoCompletionAddress,
  photosAddress,
  type PhotoSlot,
  type PhotoUploadForm,
  ratingAddress,
  readCitizenFields,
  readPhotoSlot,
  readScenePhoto,
  readScenePhotoList,
  type ScenePhotoLink,
  type ScenePhotoOut,
  type ScenePhotoType,
  STORAGE_FILE_FIELD,
  type TrangPhieuCuaToi,
  verificationPhotosAddress,
} from "./hop-dong-phan-anh";
import {
  type BaiTinXa,
  bannersAddress,
  type CanBoCongKhai,
  type CommuneBannerItem,
  type CommuneProfile,
  communeProfilesAddress,
  diaChiBaiTin,
  diaChiDanhBa,
  diaChiTinXa,
  diaChiTraXa,
  docBaiTin,
  docDanhBa,
  docTrangTinXa,
  docXa,
  type NewsCategory,
  newsCategoriesAddress,
  type NewsType,
  readBanners,
  readCommuneProfiles,
  readNewsCategories,
  type TrangTinXa,
  type XaTraDuoc,
} from "./hop-dong-cong-khai";
import type { LanGui } from "./lan-gui";
import { type ConnectionRoute, describeThrown, hostOf, isNetworkFailure, logConnectionFailure } from "./connection-log";
import { layPhienViGov } from "./phien-vigov";
import { laTenMien } from "../../lib/launch-params";

/**
 * Mỗi nhánh là một VIỆC NGƯỜI DÂN PHẢI LÀM khác nhau.
 *
 *   `chua-co-phien` · `chua-cau-hinh`  kênh chưa mở trên ứng dụng — không gửi gì đi cả
 *   `xong`                              201 / 200, có phiếu
 *   `khong-thay`                        404 — MỘT câu cho "không có", "của người khác", "xã khác"
 *   `het-phien`                         401 — phiên không dùng được
 *   `can-xac-thuc-so`                   403 `chua_xac_thuc_so` — phiên chưa có số điện thoại đã xác
 *                                       thực; việc tiếp theo là HỎI công dân (`man/phone-verification.tsx`).
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
 * màn hình người dân có câu riêng cho từng nhánh (`man/noi-dung.ts`).
 */
export type KetQuaGoi =
  | { kieu: "chua-co-phien" }
  | { kieu: "chua-cau-hinh" }
  | { kieu: "xong"; phieu: PhieuCuaToi }
  | { kieu: "khong-thay" }
  | { kieu: "het-phien" }
  | { kieu: "can-xac-thuc-so" }
  | { kieu: "dang-xu-ly-truoc" }
  | { kieu: "field-not-offered" }
  | { kieu: "khong-hop-le" }
  | { kieu: "field-catalogue-unavailable" }
  | { kieu: "kenh-chua-mo" }
  | { kieu: "loi-may-chu" }
  | { kieu: "loi-mang" };

/** Mọi nhánh trừ `xong` — chung cho mọi tuyến. */
type NhanhKhongThanh = Exclude<KetQuaGoi, { kieu: "xong" }>;

/**
 * Kết quả của tuyến "Phản ánh của tôi". Cùng các nhánh dừng với `KetQuaGoi`; hợp đồng không có
 * 404/409/503 cho tuyến này, nên gặp chúng là `loi-may-chu` (xem `phanAnhCuaToi`).
 */
export type KetQuaDanhSach = { kieu: "xong"; trang: TrangPhieuCuaToi } | NhanhKhongThanh;

/**
 * 429 `rate_limited` — the public commune-news reads are limited per client (owner, 02/10/2026: 120/min, comms).
 * `retryAfterSeconds` is the server's `Retry-After` as a whole number of seconds; `null` when it is absent or not
 * one. Its own branch because "the system is broken, wait minutes" (`loi-may-chu`) is the wrong next step for a
 * citizen who only tapped quickly: the answer is "wait a few seconds".
 */
export type RateLimited = { kieu: "rate-limited"; retryAfterSeconds: number | null };

/** What one call can become — every route's branches, plus `rate-limited` until the route decides what it means. */
type CallResult<T> = { kieu: "xong"; gia_tri: T } | NhanhKhongThanh | RateLimited;

/**
 * A refusal the server explained with an error `code` — only for the photo routes (`refusals: true`), whose
 * 400/404/409/422/503 each carry several codes meaning different next steps (`PHOTO_ERROR`). 401, 403 and
 * 429 keep their shared branches. `code` is `null` when the body had none.
 */
export type Refused = { kieu: "refused"; status: number; code: string | null };

/**
 * Routes with no 429 in their contract (the petition routes): one arriving there is a stray, and stays
 * `loi-may-chu` exactly as it was before 02/10/2026.
 */
function withoutRateLimit<T>(kq: CallResult<T>): { kieu: "xong"; gia_tri: T } | NhanhKhongThanh {
  return kq.kieu === "rate-limited" ? { kieu: "loi-may-chu" } : kq;
}

/**
 * `Retry-After` in seconds (RFC 9110 delay-seconds), or `null`. The HTTP-date form is not read: the server sends
 * seconds, and a phone clock that is off would turn a date into a nonsense wait. Never throws — a missing or
 * broken header object must not fall into the network branch ("check your connection" for an answer that came).
 */
function readRetryAfter(response: Response): number | null {
  let raw: string | null = null;
  try {
    raw = response.headers.get("Retry-After");
  } catch {
    return null;
  }
  if (typeof raw !== "string" || !/^\s*\d{1,6}\s*$/.test(raw)) return null;
  return Number(raw.trim());
}

/** Quá hạn chờ. Người dân đang cầm máy đứng chờ; thà nói "thử lại" sớm. */
const HAN_CHO_MS = 20_000;

/** Hai cổng đóng. Trả về phiên + địa chỉ, hoặc nhánh dừng. */
function moCong(dia_chi: string): { token: string; dia_chi: string } | NhanhKhongThanh {
  const phien = layPhienViGov();
  if (phien === null || phien.token === "") return { kieu: "chua-co-phien" };
  if (dia_chi === "") return { kieu: "chua-cau-hinh" };
  return { token: phien.token, dia_chi };
}

/**
 * CHỖ DUY NHẤT GỌI `fetch` TỚI ViGov — mọi tuyến ViGov đi qua đây (the two photo-byte calls below are not
 * ViGov routes; see the file header). `doc` đọc thân 200/201; `null` là sai khuôn.
 */
async function goi<T>(
  dia_chi: string,
  /** `token` VẮNG MẶT chỉ ở ba tuyến công khai — xem khối đầu tệp. `route` names the call in the log only. */
  tuy_chon: { route: ConnectionRoute; method: "GET" | "POST"; token?: string; khoa?: string; than?: string },
  doc: (than: unknown) => T | null,
  khi_404: NhanhKhongThanh,
): Promise<CallResult<T>> {
  const kq = await callLogged(dia_chi, tuy_chon, doc, khi_404);
  // Unreachable: `refusals` is not set on this path, so `callOnce` never answers `refused` here.
  return kq.kieu === "refused" ? { kieu: "loi-may-chu" } : kq;
}

/** `goi` for the photo routes: the server's refusal `code` comes back (`Refused`) instead of being folded. */
async function goiWithRefusals<T>(
  dia_chi: string,
  tuy_chon: { route: ConnectionRoute; method: "GET" | "POST"; token: string; khoa?: string; than?: string },
  doc: (than: unknown) => T | null,
): Promise<CallResult<T> | Refused> {
  return callLogged(dia_chi, { ...tuy_chon, refusals: true }, doc, { kieu: "khong-thay" });
}

/** One call, and the fixed-shape log line when it did not succeed (`connection-log.ts`). */
async function callLogged<T>(
  dia_chi: string,
  tuy_chon: { route: ConnectionRoute; method: "GET" | "POST"; token?: string; khoa?: string; than?: string; refusals?: boolean },
  doc: (than: unknown) => T | null,
  khi_404: NhanhKhongThanh,
): Promise<CallResult<T> | Refused> {
  const startedAt = Date.now();
  const trace: CallTrace = {};
  const kq = await callOnce(dia_chi, tuy_chon, doc, khi_404, trace);
  if (kq.kieu !== "xong") {
    logConnectionFailure({
      route: tuy_chon.route,
      method: tuy_chon.method,
      host: hostOf(dia_chi),
      outcome: kq.kieu,
      ...trace,
      elapsed_ms: Date.now() - startedAt,
    });
  }
  return kq;
}

/** What `callOnce` saw — a status and a thrown value's name, never a body (`connection-log.ts`). */
type CallTrace = { status?: number; error?: string };

/** `goi` without the log. */
async function callOnce<T>(
  dia_chi: string,
  tuy_chon: { method: "GET" | "POST"; token?: string; khoa?: string; than?: string; refusals?: boolean },
  doc: (than: unknown) => T | null,
  khi_404: NhanhKhongThanh,
  trace: CallTrace,
): Promise<CallResult<T> | Refused> {
  const bo_dieu_khien = new AbortController();
  const dong_ho = setTimeout(() => bo_dieu_khien.abort(), HAN_CHO_MS);

  const tieu_de: Record<string, string> = { Accept: "application/json" };
  if (tuy_chon.token !== undefined) tieu_de["Authorization"] = `Bearer ${tuy_chon.token}`;
  if (tuy_chon.than !== undefined) tieu_de["Content-Type"] = "application/json";
  if (tuy_chon.khoa !== undefined) tieu_de["Idempotency-Key"] = tuy_chon.khoa;

  try {
    const tra_loi = await fetch(dia_chi, {
      method: tuy_chon.method,
      headers: tieu_de,
      body: tuy_chon.than,
      signal: bo_dieu_khien.signal,
    });
    trace.status = tra_loi.status;

    const s = tra_loi.status;
    if (tuy_chon.refusals === true && s >= 400 && s !== 401 && s !== 403 && s !== 429) {
      return { kieu: "refused", status: s, code: await readErrorCode(tra_loi) };
    }

    switch (tra_loi.status) {
      case 200:
      case 201: {
        // A body that is not JSON is the SERVER's answer, not the network's: before 01/10/2026 it fell to
        // the `catch` below and the citizen was told to check their connection for a reply that had arrived.
        let body: unknown;
        try {
          body = await tra_loi.json();
        } catch (err) {
          trace.error = describeThrown(err);
          // …but a body CUT OFF mid-read (the 20 s abort, a dropped link) is the network after all.
          return isNetworkFailure(err) ? { kieu: "loi-mang" } : { kieu: "loi-may-chu" };
        }
        const gia_tri = doc(body);
        return gia_tri === null ? { kieu: "loi-may-chu" } : { kieu: "xong", gia_tri };
      }
      case 400:
        return (await readErrorCode(tra_loi)) === FIELD_NOT_OFFERED_CODE
          ? { kieu: "field-not-offered" }
          : { kieu: "khong-hop-le" };
      case 401:
        return { kieu: "het-phien" };
      case 403: {
        // Thân hỏng ở đây KHÔNG được rơi xuống `catch` bên dưới: đó là nhánh `loi-mang`, và bảo người
        // dân "kiểm tra mạng" cho một câu trả lời máy chủ đã gửi tới nơi là nói sai việc cần làm.
        let body: unknown = null;
        try {
          body = await tra_loi.json();
        } catch {
          return { kieu: "loi-may-chu" };
        }
        return isPhoneNotVerified(body) ? { kieu: "can-xac-thuc-so" } : { kieu: "loi-may-chu" };
      }
      case 404:
        return khi_404;
      case 409:
        return { kieu: "dang-xu-ly-truoc" };
      case 429:
        // Only the wait is read; the body is never read (nothing to show, nothing to log).
        return { kieu: "rate-limited", retryAfterSeconds: readRetryAfter(tra_loi) };
      case 503:
        return (await readErrorCode(tra_loi)) === FIELD_CATALOGUE_UNAVAILABLE_CODE
          ? { kieu: "field-catalogue-unavailable" }
          : { kieu: "kenh-chua-mo" };
      default:
        return { kieu: "loi-may-chu" };
    }
  } catch (err) {
    trace.error = describeThrown(err);
    return { kieu: "loi-mang" };
  } finally {
    clearTimeout(dong_ho);
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
  { kieu: "xong"; fields: readonly CitizenField[] } | NhanhKhongThanh
> {
  const cong = moCong(citizenFieldsAddress());
  if ("kieu" in cong) return cong;
  const kq = withoutRateLimit(
    await goi(cong.dia_chi, { route: "report-fields", method: "GET", token: cong.token }, readCitizenFields, { kieu: "loi-may-chu" }),
  );
  return kq.kieu === "xong" ? { kieu: "xong", fields: kq.gia_tri } : kq;
}

/**
 * Gửi MỘT lần gửi. Gọi lại với CÙNG `lan` là "gửi lại" — cùng thân, cùng `Idempotency-Key`.
 * Xem `lan-gui.ts`. Không ném ra ngoài.
 */
/** `gia_tri` của lớp gọi → nhánh `xong` của một phiếu. */
function thanhPhieu(kq: { kieu: "xong"; gia_tri: PhieuCuaToi } | NhanhKhongThanh): KetQuaGoi {
  return kq.kieu === "xong" ? { kieu: "xong", phieu: kq.gia_tri } : kq;
}

export async function guiPhanAnh(lan: LanGui): Promise<KetQuaGoi> {
  const cong = moCong(diaChiGuiPhanAnh());
  if ("kieu" in cong) return cong;
  return thanhPhieu(
    withoutRateLimit(
      await goi(
        cong.dia_chi,
        { route: "send-petition", method: "POST", token: cong.token, khoa: lan.khoa, than: lan.than },
        docPhieu,
        // Tuyến gửi không có 404 trong hợp đồng; gặp nó là tuyến chưa được định tuyến ở cụm.
        { kieu: "loi-may-chu" },
      ),
    ),
  );
}

/**
 * Tra MỘT phiếu của chính mình theo mã tra cứu.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI": máy chủ lấy người gửi từ phiên (luật 4, cấm #1). Mã rỗng thì trả
 * `khong-thay` mà không gọi — một đoạn đường dẫn rỗng không khớp phiếu nào.
 */
export async function traCuuPhieu(ma_tra_cuu: string): Promise<KetQuaGoi> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(diaChiTraCuu(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  return thanhPhieu(
    withoutRateLimit(
      await goi(cong.dia_chi, { route: "lookup-petition", method: "GET", token: cong.token }, docPhieu, { kieu: "khong-thay" }),
    ),
  );
}

/**
 * MỘT TRANG "PHẢN ÁNH CỦA TÔI". `con_tro` rỗng = trang đầu; trang sau truyền NGUYÊN VĂN `con_tro`
 * của trang trước.
 *
 * KHÔNG CÓ THAM SỐ "CỦA AI", "XÃ NÀO": công dân và xã đều lấy từ phiên ở máy chủ (luật 4, cấm #1;
 * luật 1, cấm #2). Hợp đồng không có 404/409/503 cho tuyến này — gặp chúng là tuyến lạc ở cụm,
 * nên đổi thành `loi-may-chu` thay vì mượn câu của tuyến gửi ("lần gửi trước đang chạy") vốn
 * không đúng với một lần XEM.
 */
export async function phanAnhCuaToi(con_tro: string): Promise<KetQuaDanhSach> {
  const cong = moCong(diaChiDanhSach(con_tro));
  if ("kieu" in cong) return cong;
  const kq = withoutRateLimit(
    await goi(cong.dia_chi, { route: "my-petitions", method: "GET", token: cong.token }, docTrangPhieuCuaToi, {
      kieu: "loi-may-chu",
    }),
  );
  if (kq.kieu === "xong") return { kieu: "xong", trang: kq.gia_tri };
  if (kq.kieu === "dang-xu-ly-truoc" || kq.kieu === "kenh-chua-mo") return { kieu: "loi-may-chu" };
  return kq;
}

/**
 * The citizen rates THEIR OWN petition `ma_tra_cuu` (ADR 0050 point 2). `attempt` is one user act — body
 * from `ratingBody`, one `Idempotency-Key` — and calling again with the SAME attempt is "send again" after a
 * lost answer: the server replays the first result instead of recording a second rating (`lan-gui.ts`).
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
export async function ratePetition(ma_tra_cuu: string, attempt: LanGui): Promise<KetQuaGoi> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(ratingAddress(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  const kq = thanhPhieu(
    withoutRateLimit(
      await goi(
        cong.dia_chi,
        { route: "rate-petition", method: "POST", token: cong.token, khoa: attempt.khoa, than: attempt.than },
        docPhieu,
        { kieu: "khong-thay" },
      ),
    ),
  );
  return kq.kieu === "kenh-chua-mo" ? { kieu: "loi-may-chu" } : kq;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * SCENE PHOTOS — the citizen's own petition, by its lookup code (`hop-dong-phan-anh.ts` §SCENE PHOTOS)
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** What a photo route can become. No "whose" parameter: citizen and commune come from the session. */
export type PhotoCallResult<T> = { kieu: "xong"; gia_tri: T } | NhanhKhongThanh | RateLimited | Refused;

/**
 * Ask for ONE upload slot. `key` is the `Idempotency-Key` of this attempt — a NEW one for every attempt: a
 * replay carries no form (`readPhotoSlot`), so reusing a key could only ever bring back nothing to upload.
 */
export async function requestScenePhotoSlot(
  ma_tra_cuu: string,
  body: string,
  key: string,
): Promise<PhotoCallResult<PhotoSlot>> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(photosAddress(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  return goiWithRefusals(
    cong.dia_chi,
    { route: "photo-slot", method: "POST", token: cong.token, khoa: key, than: body },
    readPhotoSlot,
  );
}

/** Ask the server to check and keep the photo just posted to the store (scan, re-encode without EXIF). */
export async function completeScenePhoto(ma_tra_cuu: string, id: string): Promise<PhotoCallResult<ScenePhotoOut>> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(photoCompletionAddress(ma === "" ? "x" : ma, id === "" ? "x" : id));
  if ("kieu" in cong) return cong;
  if (ma === "" || id === "") return { kieu: "khong-thay" };
  return goiWithRefusals(cong.dia_chi, { route: "photo-complete", method: "POST", token: cong.token }, readScenePhoto);
}

/** The citizen's own stored photos, each with a read link that lives ≤ 15 minutes. */
export async function listScenePhotos(ma_tra_cuu: string): Promise<PhotoCallResult<readonly ScenePhotoLink[]>> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(photosAddress(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  return goiWithRefusals(cong.dia_chi, { route: "photo-list", method: "GET", token: cong.token }, readScenePhotoList);
}

/**
 * The commune's "sau xử lý" photos of the citizen's OWN petition — empty until `cho-dan-xac-nhan` (the server
 * decides, ADR 0047 row "THAY G8"). Each link lives ≤ 15 minutes: shown, refetched, never kept.
 *
 * `goi`, NOT `goiWithRefusals`: the contract has no refusal codes here (200 · 401 · 403 · 404 · 500 · 503), and
 * 404 carries the same body as GET …/{maTraCuu} — so it is the SAME `khong-thay` the lookup gives, one answer
 * for "no such code", "someone else's" and "another commune's" (rule 4, forbidden #2). 429 is not in the
 * contract and stays `loi-may-chu`, as on the petition routes.
 */
export async function listVerificationPhotos(ma_tra_cuu: string): Promise<PhotoCallResult<readonly ScenePhotoLink[]>> {
  const ma = ma_tra_cuu.trim();
  const cong = moCong(verificationPhotosAddress(ma === "" ? "x" : ma));
  if ("kieu" in cong) return cong;
  if (ma === "") return { kieu: "khong-thay" };
  return withoutRateLimit(
    await goi(cong.dia_chi, { route: "verification-photo-list", method: "GET", token: cong.token }, readScenePhotoList, {
      kieu: "khong-thay",
    }),
  );
}

/**
 * The bytes of a photo the citizen just picked, from the LOCAL temporary path Zalo's picker returned
 * (`openMediaPicker` without an upload URL: "đường dẫn tạm thời (local cache path)", `zmp-sdk/index.d.ts:4721`).
 *
 * ⚠ NOT A NETWORK CALL TO ANY SERVER: the path names a file on this phone. `fetch` is how a webview reads it,
 *   and this file is the one place the state half may write `fetch(` (`phase1-collects-nothing.test.ts`).
 *   Whether Zalo's webview serves that path to `fetch` has NOT been measured on a device — an unreadable path is
 *   `null`, and the screen says so in a sentence; it never throws. Nothing here logs the path.
 */
export async function readPickedPhoto(path: string): Promise<Blob | null> {
  if (path === "") return null;
  try {
    const answer = await fetch(path);
    // A local file read may answer status 0 (no HTTP involved) — only an explicit HTTP failure is refused.
    if (answer.status >= 400) return null;
    const blob = await answer.blob();
    return blob.size > 0 ? blob : null;
  } catch {
    return null;
  }
}

/** One upload to the object store can take a while on a rural connection; the API calls keep `HAN_CHO_MS`. */
const UPLOAD_WAIT_MS = 90_000;

export type StorageUploadResult = { kieu: "xong" } | { kieu: "tu-choi"; status: number } | { kieu: "loi-mang" } | { kieu: "loi-may-chu" };

/**
 * The presigned POST to the object store ViGov named in `form.url` — every `fields` entry first, the file LAST
 * as `file`, the type exactly as declared in the slot request. NO bearer and NO cookie: the form IS the
 * credential (ADR 0052), and the ViGov session must never reach the storage host. A non-https URL is refused
 * before a byte leaves (`readPhotoSlot` already checks; checked again because this is where bytes go).
 *
 * 4xx → `tu-choi` (the form expired, or the store refused the size/type): a NEW slot is the next step.
 */
export async function postPhotoToStorage(
  form: PhotoUploadForm,
  photo: Blob,
  content_type: ScenePhotoType,
): Promise<StorageUploadResult> {
  let host = "";
  try {
    const url = new URL(form.url);
    if (url.protocol !== "https:") return { kieu: "loi-may-chu" };
    host = url.host;
  } catch {
    return { kieu: "loi-may-chu" };
  }
  const startedAt = Date.now();
  const trace: CallTrace = {};
  const out = await uploadOnce(form, photo, content_type, trace);
  if (out.kieu !== "xong") {
    logConnectionFailure({
      route: "photo-storage",
      method: "POST",
      host,
      outcome: out.kieu,
      ...trace,
      elapsed_ms: Date.now() - startedAt,
    });
  }
  return out;
}

async function uploadOnce(
  form: PhotoUploadForm,
  photo: Blob,
  content_type: ScenePhotoType,
  trace: CallTrace,
): Promise<StorageUploadResult> {
  const body = new FormData();
  for (const [k, v] of Object.entries(form.fields)) body.append(k, v);
  // The part's name is a fixed word, never anything of the citizen's (rule 3, forbidden #4).
  body.append(STORAGE_FILE_FIELD, new Blob([photo], { type: content_type }), "photo");
  const stop = new AbortController();
  const timer = setTimeout(() => stop.abort(), UPLOAD_WAIT_MS);
  try {
    const answer = await fetch(form.url, { method: "POST", body, credentials: "omit", signal: stop.signal });
    trace.status = answer.status;
    if (answer.status >= 200 && answer.status < 300) return { kieu: "xong" };
    if (answer.status >= 400 && answer.status < 500) return { kieu: "tu-choi", status: answer.status };
    return { kieu: "loi-may-chu" };
  } catch (err) {
    trace.error = describeThrown(err);
    return { kieu: "loi-mang" };
  } finally {
    clearTimeout(timer);
  }
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
 *
 * The comms reads (news, categories, banners, one item) add `rate-limited` — `NewsReadResult`. The identity reads
 * keep this type: their contract has no 429, so one there stays `loi-may-chu` (`identityRead`).
 */
export type KetQuaCongKhai<T> =
  | { kieu: "xong"; gia_tri: T }
  | { kieu: "khong-hop-le" }
  | { kieu: "khong-thay" }
  | { kieu: "tam-ngung" }
  | { kieu: "loi-may-chu" }
  | { kieu: "loi-mang" }
  | { kieu: "chua-cau-hinh" };

/** A public comms read: `KetQuaCongKhai`, or 429 (owner, 02/10/2026). */
export type NewsReadResult<T> = KetQuaCongKhai<T> | RateLimited;

async function goiCongKhai<T>(
  route: ConnectionRoute,
  ten_mien: string,
  dia_chi: (ten_mien: string) => string,
  doc: (than: unknown) => T | null,
): Promise<NewsReadResult<T>> {
  // Cổng TÊN MIỀN đứng trước cổng địa chỉ: không có tên miền đúng khuôn thì không một byte nào đi ra.
  if (!laTenMien(ten_mien)) return { kieu: "khong-hop-le" };
  const url = dia_chi(ten_mien);
  if (url === "") return { kieu: "chua-cau-hinh" };
  const kq = await goi(url, { route, method: "GET" }, doc, { kieu: "khong-thay" });
  switch (kq.kieu) {
    case "xong":
    case "khong-hop-le":
    case "khong-thay":
    case "loi-may-chu":
    case "loi-mang":
    case "rate-limited":
      return kq;
    case "kenh-chua-mo":
      return { kieu: "tam-ngung" };
    default:
      // 401 / 409 không có trong hợp đồng của ba tuyến công khai: tuyến lạc ở cụm.
      return { kieu: "loi-may-chu" };
  }
}

/** An identity public read has no 429 in its contract: a stray one stays `loi-may-chu`, as before 02/10/2026. */
function identityRead<T>(kq: NewsReadResult<T>): KetQuaCongKhai<T> {
  return kq.kieu === "rate-limited" ? { kieu: "loi-may-chu" } : kq;
}

/**
 * Tên và tỉnh của xã ứng với tên miền trên QR — cho màn xác nhận. Mảng rỗng là "không xã nào".
 * Tên miền là KHOÁ TRA; kết quả không cấp gì, và không được nhớ làm xã của phiên.
 */
export function traXaTheoTenMien(ten_mien: string): Promise<KetQuaCongKhai<readonly XaTraDuoc[]>> {
  return goiCongKhai("commune-lookup", ten_mien, diaChiTraXa, docXa).then(identityRead);
}

/**
 * Danh bạ cán bộ xã đã công khai. Không ghi log nội dung: danh bạ mang số di động cá nhân — bản `--demo`
 * chỉ ghi khuôn cố định của `connection-log.ts` (tuyến, mã HTTP), không một dòng nào của thân.
 */
export function danhBaCanBoXa(ten_mien: string): Promise<KetQuaCongKhai<readonly CanBoCongKhai[]>> {
  return goiCongKhai("commune-staff", ten_mien, diaChiDanhBa, docDanhBa).then(identityRead);
}

/**
 * Trụ sở, đường dây nóng, giờ làm việc mà xã đã khai (`CommuneProfile`). Công khai theo tên miền, không
 * phiên. Không có logo — xem `hop-dong-cong-khai.ts`.
 */
export function communeProfiles(ten_mien: string): Promise<KetQuaCongKhai<readonly CommuneProfile[]>> {
  return goiCongKhai("commune-profile", ten_mien, communeProfilesAddress, readCommuneProfiles).then(identityRead);
}

/**
 * Một trang tin của xã, mới nhất trước. `con_tro` rỗng = trang đầu. `type` lọc ở máy chủ theo loại tin
 * (`NewsType`); không truyền = mọi loại, như trước.
 */
export function tinCuaXa(
  ten_mien: string,
  con_tro: string,
  type: NewsType | null = null,
  /** A category id from `newsCategories`; `null` = every category. The server includes its descendants. */
  category: string | null = null,
): Promise<NewsReadResult<TrangTinXa>> {
  return goiCongKhai("news", ten_mien, (t) => diaChiTinXa(t, con_tro, type, category), docTrangTinXa);
}

/**
 * The categories of the commune's news that hold at least one published item of `type` — the chip rows
 * of the news tab. Public by domain, like the list; the domain gate in `goiCongKhai` runs first.
 */
export function newsCategories(
  ten_mien: string,
  type: NewsType | null = null,
): Promise<NewsReadResult<readonly NewsCategory[]>> {
  return goiCongKhai("news-categories", ten_mien, (t) => newsCategoriesAddress(t, type), readNewsCategories);
}

/**
 * The commune's home banner strip (`?type=banner`, ADR 0067 §5) — public by domain, like the news list. Any
 * branch but `xong` with at least one banner leaves the bundled picture in place (`TrangXa.tsx`).
 */
export function communeBanners(ten_mien: string): Promise<NewsReadResult<readonly CommuneBannerItem[]>> {
  return goiCongKhai("banners", ten_mien, bannersAddress, readBanners);
}

/**
 * Toàn văn một tin. 404 là MỘT câu: tin chưa đăng, đã gỡ, hay của xã khác trả như nhau. `noView`: a re-read that
 * is not an open (`diaChiBaiTin`) — the server then does not count a view.
 */
export function baiTinCuaXa(
  ten_mien: string,
  id: string,
  options: { noView?: boolean } = {},
): Promise<NewsReadResult<BaiTinXa>> {
  if (id === "") return Promise.resolve({ kieu: "khong-thay" });
  return goiCongKhai("news-item", ten_mien, (t) => diaChiBaiTin(t, id, options), docBaiTin);
}
