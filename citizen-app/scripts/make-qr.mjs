/**
 * `npm run qr -- --domain=<tên-miền-xã> [--version=<n>] [--out=<tệp .svg>]`
 *
 * In liên kết + vẽ QR mở App ViHAT (app chung) vào một xã, và ghi tệp SVG để in. KHÔNG dựng, KHÔNG đẩy
 * lên Zalo. Luật và lý do: đầu `commune-qr.mjs`. Hai lời gọi mạng, cả hai CHỈ ĐỌC, công khai:
 *
 *   1. platform `GET /api/v1/mini-app-ids?app=vihat`  → App ID app chung (bảng `mini_app`)
 *   2. identity `GET /api/v1/communes?host=<domain>`  → tên xã, chính câu app hỏi khi mở QR
 *
 * Lỗi nào cũng là exit 2 kèm câu nói phải làm gì — không stack trace, không tệp nào được ghi.
 */
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { renderANSI, renderSVG } from "uqr";

import { communeFrom, COMMUNES_PATH, defaultQrFile, identityOriginFrom, parseQrFlags, sharedAppIdFrom, sharedAppLink } from "./commune-qr.mjs";
import { VIGOV_PLATFORM_API_HOST } from "./deploy-config.mjs";
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

let flags, app_id, commune;
try {
  flags = parseQrFlags(process.argv.slice(2));
  const platform = checkPlatformApiHost(VIGOV_PLATFORM_API_HOST);
  const identity = identityOriginFrom(readFileSync(resolve(APP_ROOT, "src/cong-dan/api/service-hosts.gen.ts"), "utf8"));
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

const link = sharedAppLink({ app_id, domain: flags.domain, version: flags.version });
// `--out` is relative to where the command was typed; the default lives under citizen-app/qr/ (gitignored).
const file = flags.out !== null ? resolve(process.cwd(), flags.out) : resolve(APP_ROOT, defaultQrFile(flags.domain, flags.version));
// ecc "M" for a code scanned off paper or a screen at arm's length; border 4 is the quiet zone the
// standard asks for, since a printed sheet has no card around it (unlike `MaQR.tsx`).
mkdirSync(dirname(file), { recursive: true });
writeFileSync(file, renderSVG(link, { ecc: "M", border: 4, pixelSize: 10 }), "utf8");

const rule = "─".repeat(78);
console.log(`\n${rule}`);
console.log(`  App      : ${SHARED_APP_NAME} (app chung) — App ID ${app_id} (platform)`);
console.log(`  Xã       : ${commune.name}${commune.province ? `, ${commune.province}` : ""} — ${flags.domain} (identity)`);
console.log(
  flags.version === null
    ? "  Bản mở   : PHÁT HÀNH — chỉ mở được khi app chung đã phát hành"
    : `  Bản mở   : THỬ NGHIỆM — Version ${flags.version} trên console Zalo (env=TESTING)`,
);
console.log(`  Liên kết : ${link}`);
console.log(`  Tệp SVG  : ${file}`);
console.log(`${rule}\n`);
console.log(renderANSI(link, { ecc: "M", border: 2 }));
