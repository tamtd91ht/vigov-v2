/**
 * QR MỞ APP VIHAT (APP CHUNG) VÀO MỘT XÃ — phần THUẦN của `npm run qr` (`make-qr.mjs` là phần chạy).
 *
 * CHỈ SINH QR, KHÔNG DỰNG, KHÔNG ĐẨY. App chung là MỘT bundle cho mọi xã (ADR 0044); xã đi vào bằng
 * tham số trên liên kết, không bằng một lần đẩy — nên lệnh này không bao giờ chạm `zmp-cli`.
 *
 * Liên kết là đúng khuôn service-platform dựng cho nút "Mở bằng app ViHAT" ở platform-admin
 * (`service-platform/internal/domain/shared_mini_app.go`, ADR 0070 §Sửa đổi 06/10/2026 #1):
 *
 *   mã mở app (x)   : https://zalo.me/s/<App ID app chung>/?x=<mã mở app>&src=qr
 *   bản thử (-t)    : thêm `env=TESTING&version=<n>` vào ĐẦU chuỗi tham số
 *
 * CHỈ QR `x` (chủ dự án, 13/10/2026; ADR 0047 §Sửa đổi 13/10/2026): bản app đang phát hành đã đọc `x`, nên QR
 * tên miền `d` không còn được sinh. Đầu vào VẪN LÀ `--domain` (chủ dự án, 12/10/2026: "lệnh npm vẫn dùng domain
 * cho dễ nhớ"): mã `x` lấy từ CHÍNH câu tra xã theo tên miền, `GET identity /api/v1/communes?host=` trả `launch_code`.
 * Mọi xã có mã từ lúc tạo (service-platform migration 0024, NOT NULL); identity không trả mã → DỪNG, không lùi về `d`.
 *
 * `env`/`version` là tham số NỀN TẢNG của Zalo (citizen-app/README.md §"Hai thứ đã kiểm"): thiếu chúng
 * thì Zalo chỉ mở bản đã phát hành, và app chưa phát hành trả "đang trong giai đoạn phát triển" trước khi
 * mã của ta chạy. `x` + `src=qr` là tham số của ta (`src/lib/launch-params.ts`, `thamSoXa`).
 *
 * ⚠ THAM SỐ DẪN GIAO DIỆN, KHÔNG CẤP GÌ (ADR 0047 câu 3): `x` là dữ liệu client cung cấp; app hỏi
 * identity (`?code=`) rồi mới hiện xã. Nên lệnh này hỏi identity trước khi in: tên miền identity
 * không biết thì QR mở ra phần giới thiệu, không vào xã nào — một tấm QR in ra dán ở bảng tin xã sống nhiều
 * năm, phát hiện nó vô dụng lúc ấy là quá muộn.
 *
 * FAIL CLOSED, KHÔNG CÓ LỐI KHẨN CẤP: khác `deploy.mjs` (có `--app-id` khi platform im lặng), ở đây
 * platform hay identity không trả lời thì DỪNG. Một QR mang App ID chưa đối chiếu là một QR in sai mà
 * không ai biết; đợi platform chạy lại chẳng mất gì.
 *
 * Mọi hàm ở đây thuần — `commune-qr.test.mjs` kiểm từng nhánh không cần mạng.
 */
import { kiemTenMien, SHARED_APP_NAME } from "./dich-den.mjs";

/** Tuyến công khai của identity tra xã theo tên miền — cùng tuyến màn xác nhận xã gọi. */
export const COMMUNES_PATH = "/api/v1/communes";

/** Gốc liên kết Mini App, đúng hằng `zaloMiniAppLinkBase` của service-platform. */
const ZALO_LINK_BASE = "https://zalo.me/s/";

const VALID_FLAGS = "--domain=<tên-miền-xã> · --version=<số bản thử> · --profile=dev · --out=<tệp .svg>";

/**
 * Đọc cờ. Cờ lạ thì DỪNG (gõ nhầm `--domian` mà bị bỏ qua là một QR không ai muốn).
 * `--domain` bắt buộc; `--version` vắng = bản phát hành; `--profile` vắng = prod (chủ dự án, 14/10/2026).
 */
export function parseQrFlags(argv) {
  const flags = { domain: null, version: null, out: null, profile: "prod" };
  for (const c of argv) {
    if (c.startsWith("--domain=")) {
      const v = c.slice("--domain=".length);
      if (v === "") throw new Error("--domain= để trống. Ghi tên miền của xã, vd --domain=xa-a.vigov.vn.");
      kiemTenMien(v);
      if (flags.domain !== null && flags.domain !== v) throw new Error("--domain có hai giá trị khác nhau.");
      flags.domain = v;
    } else if (c.startsWith("--version=")) {
      const v = c.slice("--version=".length);
      if (!/^[1-9]\d{0,5}$/.test(v)) {
        throw new Error(`--version="${v}" không phải số bản: một số nguyên dương, đúng số "Version" trên console Zalo.`);
      }
      flags.version = v;
    } else if (c.startsWith("--out=")) {
      const v = c.slice("--out=".length);
      if (!v.toLowerCase().endsWith(".svg")) throw new Error(`--out="${v}" phải là một tệp .svg.`);
      flags.out = v;
    } else if (c.startsWith("--profile=")) {
      const v = c.slice("--profile=".length);
      if (v !== "dev" && v !== "prod") throw new Error(`--profile="${v}" không có. Chỉ nhận dev (bỏ cờ = prod).`);
      flags.profile = v;
    } else if (c === "--app=vihat" || c.startsWith("--app-id=") || c === "--phat-hanh" || c === "--thu") {
      throw new Error(
        `Cờ "${c}" là của npm run zmp:deploy. Lệnh QR luôn dùng ${SHARED_APP_NAME}, App ID do platform trả; ` +
          `chỉ nhận: ${VALID_FLAGS}`,
      );
    } else {
      throw new Error(`Cờ "${c}" không có. Chỉ nhận: ${VALID_FLAGS}`);
    }
  }
  if (flags.domain === null) {
    throw new Error(`Thiếu --domain=<tên-miền-xã>. Ví dụ: npm run qr -- --domain=xa-a.vigov.vn\n  Cờ nhận: ${VALID_FLAGS}`);
  }
  return flags;
}

/**
 * Gốc https của identity, đọc từ `src/cong-dan/api/service-hosts.gen.ts` — tệp SINH từ
 * `deploy/hosts.yaml`, nơi duy nhất viết host công khai. Đọc chuỗi chứ không gõ lại host: gõ lại là
 * nơi thứ hai, và nó lệch ngày đổi host. Khuôn tệp đổi mà không đọc được thì DỪNG, không đoán.
 */
export function identityOriginFrom(generated, profile = "prod") {
  // Inside the table of the chosen profile only: the file holds two, and the first `identity:` is prod's.
  const table = profile === "dev" ? "SERVICE_API_HOSTS_DEV" : "SERVICE_API_HOSTS";
  const block = new RegExp(`export const ${table} = \\{([^}]*)\\}`).exec(String(generated ?? ""))?.[1] ?? "";
  const m = /\bidentity:\s*"([^"]*)"/.exec(block);
  const v = m?.[1] ?? "";
  let url = null;
  try {
    url = new URL(v);
  } catch {
    url = null;
  }
  if (url === null || url.protocol !== "https:" || url.origin !== v) {
    throw new Error(
      "Không sinh QR: không đọc được host identity (https) trong src/cong-dan/api/service-hosts.gen.ts.\n" +
        "  Tệp ấy sinh từ deploy/hosts.yaml bằng `make kb` (hoặc `go run ./tools/ingress`) — sinh lại rồi chạy lại.",
    );
  }
  return url.origin;
}

/** Nơi khai App ID app chung — người chạy lệnh không sửa được gì ở kho này. */
const FIX_SHARED = "platform-admin → khai App ViHAT (app chung) với App ID của nó";

/**
 * Kết quả `GET platform /api/v1/mini-app-ids?app=vihat` → App ID app chung, hoặc ném lỗi.
 * `outcome` là thứ `lookupMiniAppId` trả (`{ kind: "http", status, body }` | `{ kind: "network", reason }`).
 */
export function sharedAppIdFrom(outcome) {
  const unreachable = (why) =>
    new Error(
      `Không sinh QR: không hỏi được platform App ID của ${SHARED_APP_NAME} — ${why}.\n` +
        "  Lệnh không đoán App ID (fail closed). Thử lại khi platform chạy, hoặc lấy QR ở platform-admin → chi tiết xã → \"Mở bằng app ViHAT\".",
    );
  if (outcome?.kind === "network") throw unreachable(`không kết nối được (${outcome.reason})`);
  if (outcome?.kind !== "http") throw new Error("Không sinh QR: kết quả tra App ID không đọc được.");
  const { status, body } = outcome;
  const code = typeof body?.code === "string" ? body.code : null;
  if (status === 200) {
    const app_id = body?.app_id;
    if (typeof app_id !== "string" || !/^\d+$/.test(app_id)) {
      throw new Error('Không sinh QR: platform trả 200 nhưng thân không đúng hợp đồng {"app_id":"<chữ số>","source":"chung"}.');
    }
    if (body?.source !== "chung") {
      throw new Error(`Không sinh QR: platform nói App ID ${app_id} không phải app chung (source="${body?.source}").\n  Sửa ở: ${FIX_SHARED}.`);
    }
    return app_id;
  }
  if (status === 404 && code === "mini_app_id_not_found") {
    throw new Error(`Không sinh QR: platform chưa có App ID nào đang dùng cho ${SHARED_APP_NAME}.\n  Sửa ở: ${FIX_SHARED}, rồi chạy lại.`);
  }
  if (status === 409) {
    throw new Error(`Không sinh QR: platform có hơn một ${SHARED_APP_NAME} đang sống (409). Tắt app thừa ở platform-admin, rồi chạy lại.`);
  }
  if (status === 404) throw unreachable("platform chưa có tuyến tra App ID (404 không mang mã mini_app_id_not_found)");
  if (status === 429) throw unreachable("platform đang giới hạn tần suất (429)");
  if (status >= 500) throw unreachable(`platform lỗi (${status})`);
  throw new Error(`Không sinh QR: platform trả ${status}${code ? ` (${code})` : ""} cho tuyến tra App ID — không đúng hợp đồng.`);
}

/** The launch code's shape — the same as `src/lib/launch-params.ts` `isLaunchCode`: exactly 12 lowercase hex. */
const LAUNCH_CODE = /^[0-9a-f]{12}$/;

/**
 * Kết quả `GET identity /api/v1/communes?host=<domain>` → `{ name, province, launch_code }` của ĐÚNG MỘT xã,
 * hoặc ném. Thân `{ items: [{ name, province, launch_code? }] }`. Rỗng = identity không biết tên miền ấy: QR
 * sẽ mở phần giới thiệu, nên không in.
 *
 * `launch_code` vắng hoặc sai khuôn → DỪNG: không có mã thì không có QR nào mở được xã, và lệnh không tự cấp
 * mã (mã do service-platform cấp lúc tạo xã).
 */
export function communeFrom(domain, outcome) {
  if (outcome?.kind === "network") {
    throw new Error(`Không sinh QR: không hỏi được identity xã nào có tên miền ${domain} — không kết nối được (${outcome.reason}). Thử lại sau.`);
  }
  if (outcome?.kind !== "http") throw new Error("Không sinh QR: kết quả tra xã không đọc được.");
  const { status, body } = outcome;
  if (status !== 200) {
    throw new Error(`Không sinh QR: identity trả ${status} khi tra xã theo tên miền ${domain}. Thử lại sau, hoặc kiểm lại tên miền.`);
  }
  const items = body?.items;
  if (!Array.isArray(items)) throw new Error("Không sinh QR: identity trả 200 nhưng thân không có mảng `items`.");
  if (items.length === 0) {
    throw new Error(
      `Không sinh QR: identity không biết xã nào có tên miền ${domain}.\n` +
        "  QR ấy sẽ mở phần giới thiệu ViHAT, không vào xã nào. Kiểm lại tên miền (platform-admin → chi tiết xã → tên miền chính).",
    );
  }
  if (items.length > 1) throw new Error(`Không sinh QR: identity trả ${items.length} xã cho tên miền ${domain} — app sẽ không vào xã nào.`);
  const { name, province } = items[0] ?? {};
  if (typeof name !== "string" || name === "") throw new Error("Không sinh QR: identity trả một xã không có tên.");
  const code = items[0]?.launch_code;
  if (typeof code !== "string" || !LAUNCH_CODE.test(code)) {
    throw new Error(
      `Không sinh QR: identity không trả mã mở app (x) cho ${name} (${domain}).\n` +
        "  Mọi xã đều có mã từ lúc tạo (service-platform migration 0024), nên thiếu mã nghĩa là service-identity /\n" +
        "  service-platform đang chạy chưa phải bản có mã mở app. Xem mã ở platform-admin → chi tiết xã → thẻ QR, rồi chạy lại.",
    );
  }
  return { name, province: typeof province === "string" ? province : "", launch_code: code };
}

/**
 * `?x=<launch code>&src=qr`. Parameter order: Zalo's (`env`, `version`) first, ours (`x`, `src`) after — the
 * measured shape.
 */
export function launchCodeLink({ app_id, launch_code, version = null, profile_code = null }) {
  if (!LAUNCH_CODE.test(String(launch_code))) throw new Error("Không sinh QR x: mã mở app không đúng 12 ký tự hex thường.");
  const q = new URLSearchParams();
  if (version !== null) {
    q.set("env", "TESTING");
    q.set("version", version);
  }
  q.set("x", launch_code);
  q.set("src", "qr");
  // `pf` — the dev profile code (src/lib/runtime-profile.ts). Absent = prod. Never Zalo's own `env`: that one picks
  // the Zalo build (TESTING) and a link has room for one.
  if (profile_code !== null) q.set("pf", profile_code);
  return `${ZALO_LINK_BASE}${app_id}/?${q.toString()}`;
}

/**
 * Tệp SVG mặc định: `qr/<tên-miền>[.dev][.test-v<n>].svg`. Bản thử và profile dev không ghi đè bản phát hành.
 * Tên tệp mang tên miền chứ không mang mã: đó là tệp cục bộ để in, không phải nội dung QR.
 */
export function defaultQrFile(domain, version = null, profile = "prod") {
  return `qr/${domain}${profile === "dev" ? ".dev" : ""}${version === null ? "" : `.test-v${version}`}.svg`;
}

/** Name of the variable holding the dev profile code — shell first, then `.env.qr.local` (gitignored). */
export const DEV_PROFILE_CODE_VAR = "VIGOV_DEV_PROFILE_CODE";
export const DEV_PROFILE_CODE_FILE = ".env.qr.local";

/** URL-safe, long enough that its SHA-256 in the bundle cannot be brute-forced. */
const DEV_PROFILE_CODE = /^[A-Za-z0-9_-]{20,128}$/;

/** The digest the app compares against, read from `src/lib/runtime-profile.ts` — not retyped here. */
export function devProfileDigestFrom(source) {
  const m = /DEV_PROFILE_CODE_SHA256\s*=\s*"([0-9a-f]{64})"/.exec(String(source ?? ""));
  if (m === null) {
    throw new Error("Không sinh QR dev: không đọc được DEV_PROFILE_CODE_SHA256 (64 hex) trong src/lib/runtime-profile.ts.");
  }
  return m[1];
}

/**
 * The dev profile code, or throw. Shell variable wins over the file (same order as `.env.local`). The code must hash
 * to the digest the app carries: a QR with a code the app does not know opens PROD silently — printed, it would look
 * like a dev QR that "does not work", and nobody would know why.
 *
 * `sha256Hex` is injected so this stays pure.
 */
export function devProfileCodeFrom({ env, fileText, expectedSha256, sha256Hex }) {
  let code = (env?.[DEV_PROFILE_CODE_VAR] ?? "").trim();
  if (code === "" && typeof fileText === "string") {
    const m = new RegExp(`^[ \\t]*${DEV_PROFILE_CODE_VAR}[ \\t]*=[ \\t]*(\\S*)[ \\t]*$`, "m").exec(fileText);
    code = m?.[1] ?? "";
  }
  if (code === "") {
    throw new Error(
      `Không sinh QR dev: thiếu ${DEV_PROFILE_CODE_VAR}.\n` +
        `  Đặt trong citizen-app/${DEV_PROFILE_CODE_FILE} (dòng ${DEV_PROFILE_CODE_VAR}=<mã>) hoặc biến shell. Mã do người giữ\n` +
        "  profile dev cấp; KHÔNG commit, KHÔNG dán vào chat hay tài liệu.",
    );
  }
  if (!DEV_PROFILE_CODE.test(code)) {
    throw new Error(`Không sinh QR dev: ${DEV_PROFILE_CODE_VAR} sai khuôn (20–128 ký tự A-Z a-z 0-9 - _).`);
  }
  if (sha256Hex(code) !== expectedSha256) {
    throw new Error(
      `Không sinh QR dev: ${DEV_PROFILE_CODE_VAR} không khớp DEV_PROFILE_CODE_SHA256 trong src/lib/runtime-profile.ts.\n` +
        "  QR ấy sẽ mở PROD, không phải dev. Lấy đúng mã, hoặc xoay mã (mã mới + digest mới + bản app mới).",
    );
  }
  return code;
}
