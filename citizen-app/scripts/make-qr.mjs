/**
 * `npm run qr -- --domain=<tên-miền-xã> [--version=<n>] [--profile=dev] [--out=<tệp .svg>]`
 *
 * `--profile=dev` (chủ dự án, 14/10/2026): QR mang thêm `pf=<mã profile dev>` — app gọi máy chủ dev trong lượt mở
 * ấy — và App ID + mã `x` tra ở platform/identity DEV. Mã đọc từ `VIGOV_DEV_PROFILE_CODE` (shell) hoặc
 * `.env.qr.local`, phải khớp digest trong `src/lib/runtime-profile.ts`. Bỏ cờ = prod, như trước.
 *
 * In liên kết + vẽ QR `x` (mã mở app) mở App ViHAT (app chung) vào một xã, và ghi MỘT tệp SVG để in. KHÔNG dựng,
 * KHÔNG đẩy lên Zalo. Luật và lý do: đầu `commune-qr.mjs`. Hai lời gọi mạng, cả hai CHỈ ĐỌC, công khai:
 *
 *   1. platform `GET /api/v1/mini-app-ids?app=vihat`  → App ID app chung (bảng `mini_app`)
 *   2. identity `GET /api/v1/communes?host=<domain>`  → tên xã + `launch_code` (mã `x` của QR)
 *
 * Lỗi nào cũng là exit 2 kèm câu nói phải làm gì — không stack trace, không tệp nào được ghi. Xã chưa có
 * `launch_code` cũng là lỗi: câu báo chỉ sang platform-admin để cập nhật mã.
 */
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { renderANSI, renderSVG } from "uqr";

import { createHash } from "node:crypto";

import {
  communeFrom,
  COMMUNES_PATH,
  defaultQrFile,
  DEV_PROFILE_CODE_FILE,
  devProfileCodeFrom,
  devProfileDigestFrom,
  identityOriginFrom,
  launchCodeLink,
  parseQrFlags,
  sharedAppIdFrom,
} from "./commune-qr.mjs";
import { VIGOV_PLATFORM_API_HOST, VIGOV_PLATFORM_API_HOST_DEV } from "./deploy-config.mjs";
import { checkPlatformApiHost, SHARED_APP_NAME } from "./dich-den.mjs";
import { lookupMiniAppId, LOOKUP_TIMEOUT_MS } from "./mini-app-id-lookup.mjs";

const APP_ROOT = fileURLToPath(new URL("..", import.meta.url));

function stop(err) {
  console.error(`\n${err.message}\n`);
  process.exit(2);
}

/** Same outcome shape as `lookupMiniAppId`, for the identity route. */
async function getJson(url) {
  let res;
  try {
    res = await fetch(url, {
      headers: { accept: "application/json" },
      redirect: "error",
      signal: AbortSignal.timeout(LOOKUP_TIMEOUT_MS),
    });
  } catch (err) {
    const reason = err?.name === "TimeoutError" ? `quá ${LOOKUP_TIMEOUT_MS / 1000} giây` : (err?.cause?.code ?? err?.name ?? "lỗi mạng");
    return { kind: "network", reason: String(reason) };
  }
  let body = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  return { kind: "http", status: res.status, body };
}

function readOptional(path) {
  try {
    return readFileSync(path, "utf8");
  } catch {
    return null;
  }
}

let flags, app_id, commune;
let profile_code = null;
try {
  flags = parseQrFlags(process.argv.slice(2));
  const dev = flags.profile === "dev";
  // A dev QR is checked BEFORE any network call: a code the app would not recognise is a QR that opens prod.
  if (dev) {
    profile_code = devProfileCodeFrom({
      env: process.env,
      fileText: readOptional(resolve(APP_ROOT, DEV_PROFILE_CODE_FILE)),
      expectedSha256: devProfileDigestFrom(readFileSync(resolve(APP_ROOT, "src/lib/runtime-profile.ts"), "utf8")),
      sha256Hex: (s) => createHash("sha256").update(s, "utf8").digest("hex"),
    });
  }
  // The App ID and the launch code come from the database of the profile the QR opens: dev's `x` names no commune on prod.
  const platform = checkPlatformApiHost(dev ? VIGOV_PLATFORM_API_HOST_DEV : VIGOV_PLATFORM_API_HOST);
  const identity = identityOriginFrom(
    readFileSync(resolve(APP_ROOT, "src/cong-dan/api/service-hosts.gen.ts"), "utf8"),
    flags.profile,
  );
  const communes_url = new URL(COMMUNES_PATH, identity);
  communes_url.searchParams.set("host", flags.domain);
  const [app_outcome, commune_outcome] = await Promise.all([
    lookupMiniAppId(platform, { app: "vihat" }),
    getJson(communes_url),
  ]);
  app_id = sharedAppIdFrom(app_outcome);
  commune = communeFrom(flags.domain, commune_outcome);
} catch (err) {
  stop(err);
}

/** `--out` is relative to where the command was typed; the default lives under citizen-app/qr/ (gitignored). */
const file =
  flags.out !== null
    ? resolve(process.cwd(), flags.out)
    : resolve(APP_ROOT, defaultQrFile(flags.domain, flags.version, flags.profile));
const link = launchCodeLink({ app_id, launch_code: commune.launch_code, version: flags.version, profile_code });

// ecc "M" for a code scanned off paper or a screen at arm's length; border 4 is the quiet zone the
// standard asks for, since a printed sheet has no card around it (unlike `MaQR.tsx`).
mkdirSync(dirname(file), { recursive: true });
writeFileSync(file, renderSVG(link, { ecc: "M", border: 4, pixelSize: 10 }), "utf8");

const rule = "─".repeat(78);
console.log(`
${rule}`);
console.log(`  App      : ${SHARED_APP_NAME} (app chung) — App ID ${app_id} (platform)`);
console.log(`  Xã       : ${commune.name}${commune.province ? `, ${commune.province}` : ""} — ${flags.domain} (identity)`);
console.log(
  flags.version === null
    ? "  Bản mở   : PHÁT HÀNH — chỉ mở được khi app chung đã phát hành"
    : `  Bản mở   : THỬ NGHIỆM — Version ${flags.version} trên console Zalo (env=TESTING)`,
);
console.log(
  flags.profile === "dev"
    ? "  Profile  : DEV — app gọi máy chủ dev trong lượt mở bằng QR này (mã pf=)"
    : "  Profile  : PROD",
);
console.log(`  Liên kết : ${link}`);
console.log(`  Tệp SVG  : ${file}`);
console.log(`${rule}
`);
console.log(renderANSI(link, { ecc: "M", border: 2 }));
