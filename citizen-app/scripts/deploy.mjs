/**
 * Dựng → đồng bộ `app-config.json` → đẩy lên Zalo. ĐÍCH LUÔN ĐƯỢC CHỌN TƯỜNG MINH, và script NÓI RA
 * nó sắp làm gì trước khi làm.
 *
 * ```
 * node scripts/deploy.mjs --app=vihat                     App ViHAT (app chung), bản thử nghiệm (-t)
 * node scripts/deploy.mjs --domain=<tên-miền-xã>          APP RIÊNG của xã ấy, mở thẳng vào xã, bản thử nghiệm
 * node scripts/deploy.mjs                                 MENU chọn đích (chỉ khi có người ngồi trước cửa sổ lệnh)
 * node scripts/deploy.mjs … --phat-hanh                   BẢN PHÁT HÀNH (bỏ -t)
 * node scripts/deploy.mjs … --app-id=<chữ số>             App ID của đích, khi tệp ánh xạ chưa có (có thì phải trùng)
 * node scripts/deploy.mjs … --thu                         IN RA rồi DỪNG, không dựng, không đẩy
 * ```
 *
 * HAI ĐÍCH (chủ dự án 06/10/2026; ADR 0047): `--app=vihat` đẩy lên App ViHAT, bundle chung, KHÔNG nung
 * xã nào (ADR 0044). `--domain=<x>` đẩy lên App ID riêng của xã `<x>` và LUÔN nung `<x>` vào bundle —
 * app mở thẳng vào xã (ADR 0047 §6; cờ `--vao-thang` cũ, nay đã bỏ và bị từ chối). Không cờ nào: hỏi
 * bằng menu, hoặc TỪ CHỐI khi không có người để hỏi. KHÔNG BAO GIỜ một đích mặc định: trước 06/10/2026
 * "không cờ" nghĩa là app mà `ZMP_TOKEN` trong `citizen-app/.env` thuộc về — app người ấy đăng nhập lần
 * cuối trên máy ấy.
 *
 * QR mở App ViHAT vào một xã KHÔNG phải việc của script này — platform-admin làm (chi tiết xã → "Mở
 * bằng app ViHAT").
 *
 * ⚠ ĐÍCH DO `ZMP_TOKEN` QUYẾT, KHÔNG DO `APP_ID` — đã đo, xem đầu `dich-den.mjs`. Nên mọi đích lấy token
 * từ MÔI TRƯỜNG, hoặc — khi môi trường không có token đúng App ID và có người ngồi trước cửa sổ lệnh —
 * TỰ chạy `zmp login` (quét QR) cho đúng App ID ấy, rồi kiểm claim `appId` khớp App ID đích trước khi
 * chạy gì cả. Không đích nào dùng `citizen-app/.env`: zmp-cli chỉ đọc tệp ấy khi môi trường thiếu
 * `ZMP_TOKEN`, và script luôn đặt nó cho tiến trình con.
 *
 * VÌ SAO `login` CHẠY TRONG MỘT THƯ MỤC TẠM: `zmp login` ghi `APP_ID` + `ZMP_TOKEN` vào `.env` của
 * thư mục đang đứng, và chỉ hỏi "Mini App ID" khi chưa thấy `APP_ID` ở đâu. Chạy trong `citizen-app`
 * thì nó có thể không hỏi (đã có `APP_ID` cũ) và để lại một token trên đĩa. Thư mục tạm bị xoá ngay
 * sau khi đọc token; token chỉ sống trong biến của script và môi trường của tiến trình con zmp.
 *
 * `--thu` in ra đúng kế hoạch và đúng dòng lệnh sẽ chạy, rồi dừng. Đọc kế hoạch mà không phải
 * đặt cược một lần đẩy để đọc nó. Không có người để hỏi thì `--thu` từ chối đúng chỗ lần chạy thật
 * từ chối.
 *
 * VÌ SAO BA BƯỚC NẰM TRONG MỘT SCRIPT:
 *
 *   Bỏ sót bước giữa là nộp một app trắng trơn. Zalo không dùng `index.html` của ta: nó tự dựng
 *   vỏ rồi nạp đúng những tệp khai trong `app-config.json`, và `sync-config` là thứ điền danh
 *   sách ấy từ trang vừa dựng. Dựng xong mà quên đồng bộ thì `app-config.json` trỏ vào bản dựng
 *   của lần trước — không có lỗi nào báo ra.
 *
 *   Và bước dựng phải nằm ở đây vì **địa chỉ máy chủ được nung vào lúc dựng** (`vite.config.ts`).
 *   Dựng ngoài rồi đẩy trong là hai lệnh có thể lệch nhau: bundle mang một `VIGOV_API_HOST`, kế
 *   hoạch in ra một địa chỉ khác.
 *
 * VÌ SAO CÓ ĐƯỜNG PHÁT HÀNH, TRONG KHI TRƯỚC ĐÂY CỐ Ý KHÔNG CÓ:
 *
 *   `-t` là bản thử nghiệm; bỏ `-t` là phát hành. Trước đây script không có cờ ấy, để việc phát
 *   hành phải là một quyết định có người gõ tay ra. Nay người dùng cần đường ấy, nên **ma sát
 *   chuyển chỗ** chứ không biến mất: từ *"không có lệnh"* sang *"lệnh nói rõ nó đang làm gì rồi
 *   dừng 5 giây"*. Đừng bỏ phần in ra và phần đếm ngược — đó là toàn bộ cái phanh còn lại.
 *
 * DẤU `dirty` KHÔNG PHẢI TRANG TRÍ. Đẩy từ một cây làm việc còn thay đổi chưa commit nghĩa là
 * bản trên Zalo KHÔNG ứng với commit nào cả — không ai dựng lại được nó, kể cả chính người vừa
 * đẩy. Nhãn nói ra điều đó thay vì để người đọc console tưởng `<sha>` là đủ để truy.
 *
 * Nhãn phiên bản mang cả ĐÍCH (App ID, + tên miền cho app riêng): hai lần đẩy cùng một commit lên hai
 * app mà nhãn giống nhau thì console Zalo có hai dòng không phân biệt được.
 */
import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline/promises";

import { BIEN_XA_CO_DINH, docCauHinh } from "./cau-hinh.mjs";
import {
  appConfigChoLanDay,
  appIdPrompt,
  buildMenu,
  checkAppIdInput,
  chonDich,
  docCo,
  hasTarget,
  kiemBangAnhXa,
  kiemToken,
  missingAppIdMessage,
  nhanPhienBan,
  noTargetMessage,
  pickMenuItem,
  planLines,
  savedRegistryNote,
  targetName,
  tokenTrongTepEnv,
  updateRegistryText,
} from "./dich-den.mjs";
import { dung } from "./dung.mjs";
import { APP_ID_APP_CHUNG, APP_ID_THEO_TEN_MIEN } from "./ung-dung-theo-ten-mien.mjs";

const ZMP = "zmp-cli@4.0.3";
const GIAY_CHO = 5;
const REGISTRY_URL = new URL("./ung-dung-theo-ten-mien.mjs", import.meta.url);

function git(...args) {
  const r = spawnSync("git", args, { encoding: "utf8" });
  return r.status === 0 ? r.stdout.trim() : "";
}

/**
 * `npx` là tệp lệnh của Windows, nên bước này cần shell — khác với `vite` trong dung.mjs.
 *
 * `env` chỉ đi vào TIẾN TRÌNH CON của zmp, không bao giờ vào `process.env` của script này: bước
 * dựng (`dung`) kế thừa `process.env`, và một App ID hay token đặt vào đó là một giá trị theo xã
 * nằm ngay cạnh `define:` của Vite — đúng thứ ADR 0047 điều kiện dừng #2 cấm.
 */
function zmp(env, ...args) {
  const r = spawnSync("npx", ["--yes", ZMP, ...args], {
    stdio: "inherit",
    shell: process.platform === "win32",
    env,
  });
  return r.status ?? 1;
}

/**
 * `zmp login` for ONE App ID, inside a throwaway directory (see the header for why). Returns the
 * token, or `null` if the login failed or wrote nothing — zmp-cli prints "Login failed!" and still
 * exits 0, so the token file is the only trustworthy signal.
 */
function dangNhapRieng(app_id) {
  const thu_muc = mkdtempSync(join(tmpdir(), "vigov-zmp-"));
  try {
    console.log(`\nĐăng nhập Zalo cho App ID ${app_id}.`);
    console.log('  Chọn "1. Login Via QR Code With Zalo App", quét mã bằng tài khoản Zalo có quyền trên app này.\n');
    // ZMP_TOKEN emptied so zmp-cli cannot pick up a token from this shell instead of logging in.
    spawnSync("npx", ["--yes", ZMP, "login"], {
      stdio: "inherit",
      shell: process.platform === "win32",
      cwd: thu_muc,
      env: { ...process.env, APP_ID: app_id, ZMP_TOKEN: "" },
    });
    const tep = join(thu_muc, ".env");
    return existsSync(tep) ? tokenTrongTepEnv(readFileSync(tep, "utf8")) : null;
  } finally {
    rmSync(thu_muc, { recursive: true, force: true });
  }
}

/** Ngủ ĐỒNG BỘ: người chạy phải kịp đọc rồi Ctrl-C, mà Ctrl-C chỉ tới được khi chưa làm gì cả. */
function nghi(mili_giay) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, mili_giay);
}

/**
 * One question on the terminal. The interface is closed after every answer so that a later
 * `spawnSync(..., { stdio: "inherit" })` (the `zmp login` QR prompt) owns stdin alone.
 */
async function ask(question) {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  try {
    return await rl.question(question);
  } finally {
    rl.close();
  }
}

/** Ask until `parse` accepts the answer. Ctrl-C is the way out. */
async function askUntil(question, parse) {
  for (;;) {
    try {
      return parse(await ask(question));
    } catch (loi) {
      console.log(`  ${loi.message}`);
    }
  }
}

// ---------------------------------------------------------------------------------------------

/** Mọi lỗi chọn đích đều là exit 2 kèm câu nói phải làm gì — không bao giờ một stack trace. */
function dungLai(loi) {
  console.error(`
${typeof loi === "string" ? loi : loi.message}
`);
  process.exit(2);
}

// Có người ngồi trước cửa sổ lệnh không. Không (Jenkins, một ống, `</dev/null`) thì không hỏi gì cả:
// mọi chỗ lẽ ra hỏi thành TỪ CHỐI kèm cờ phải truyền.
const interactive = Boolean(process.stdin.isTTY);

let bang = APP_ID_THEO_TEN_MIEN;
let app_chung = APP_ID_APP_CHUNG;
let co;
try {
  kiemBangAnhXa(bang, app_chung);
  co = docCo(process.argv.slice(2));
} catch (loi) {
  dungLai(loi);
}
const { phat_hanh, chi_thu } = co;

/**
 * CHẶN ĐƯỜNG ĐẨY KHI CHƯA KHAI ĐỊA CHỈ MÁY CHỦ — 20/09/2026, ngày bản nộp bắt đầu gọi máy chủ.
 *
 * Khối đăng nhập đọc `VIGOV_API_HOST` **lúc dựng**. Quên đặt biến thì app vẫn dựng xanh, vẫn
 * chạy, và cái nút đăng nhập hiện ra một câu nói với NGƯỜI DỰNG BẢN: *"Bản dựng này chưa được
 * khai địa chỉ máy chủ…"*. Đẩy bản ấy đi duyệt là nộp một nút đăng nhập không đăng nhập nổi,
 * kèm một câu chữ kỹ thuật — hồ sơ bị trả về, và người phát hiện ra là người duyệt.
 *
 * VÌ SAO CHẶN Ở ĐÂY CHỨ KHÔNG NÉM LỖI TRONG `vite.config.ts`: `npm test` và `npm run dev` phải
 * chạy được trên một máy chưa có địa chỉ máy chủ nào — `bundle-for-zalo.test.ts` dựng thật
 * bản đẩy lên Zalo trong mọi lần chạy test. Ma sát đặt đúng chỗ có hậu quả: đường ĐẨY LÊN ZALO.
 *
 * Chặn TRƯỚC mọi câu hỏi: không bắt ai chọn đích, nhập App ID hay quét QR cho một lần chạy chắc chắn
 * bị chặn.
 */
// ĐỌC CÙNG MỘT NGUỒN VỚI BƯỚC DỰNG. Script này chạy NGOÀI Vite nên nó không tự thấy
// `.env.local`; `docCauHinh` là chỗ duy nhất biết cách đọc tệp ấy. Nhập lại nó ở đây là cách
// duy nhất để "deploy thấy host" và "vite thấy host" không bao giờ lệch nhau.
//
// Lời gọi này cũng chạy phép kiểm danh sách trắng của `.env.local`, nên một bí mật đặt nhầm
// vào tệp ấy bị chặn ở ĐÂY nữa, trước cả bước dựng.
const api_host = docCauHinh().VIGOV_API_HOST;
if (api_host === "" && !chi_thu) {
  console.error(
    "\nVIGOV_API_HOST chưa được đặt.\n" +
      "  Khối đăng nhập đọc biến này LÚC DỰNG. Thiếu nó, bản đẩy lên sẽ có một nút đăng nhập\n" +
      "  không đăng nhập được, và hiện một câu dành cho người dựng bản.\n" +
      "  Cách thường dùng: chép `.env.local.example` thành `.env.local` rồi điền địa chỉ.\n" +
      "  Hoặc đặt cho đúng một lần chạy: VIGOV_API_HOST=https://<host> npm run zmp:deploy -- --app=vihat\n" +
      "  (Thêm --thu để chỉ in ra kế hoạch mà không cần biến này.)\n",
  );
  process.exit(2);
}

// 1. ĐÍCH. Cờ, hoặc menu. Không cờ và không ai để hỏi thì TỪ CHỐI — không bao giờ một đích mặc định.
let target = co;
if (!hasTarget(co)) {
  const menu = buildMenu(bang, app_chung);
  if (!interactive) dungLai(noTargetMessage(menu));
  console.log("\nĐẩy lên app nào?");
  for (const m of menu) console.log(`  ${m.number}. ${m.label}`);
  const item = await askUntil(`Chọn số (1–${menu.length}): `, (t) => pickMenuItem(menu, t));
  target = { ...co, shared_app: item.shared_app, ten_mien: item.ten_mien };
}

// 2. APP ID CỦA ĐÍCH. Tệp ánh xạ, `--app-id`, hoặc hỏi. Không có và không ai để hỏi thì TỪ CHỐI.
let dich;
try {
  dich = chonDich(target, bang, app_chung);
} catch (loi) {
  dungLai(loi);
}
if (dich.app_id === null) {
  if (!interactive) dungLai(missingAppIdMessage(dich));
  const typed = await askUntil(appIdPrompt(dich), checkAppIdInput);
  try {
    // Qua `chonDich` lần nữa: App ID vừa gõ chịu đúng các phép chặn của `--app-id`.
    dich = { ...chonDich({ ...target, app_id: typed }, bang, app_chung), app_id_source: "typed" };
  } catch (loi) {
    dungLai(loi);
  }
  if (chi_thu) {
    console.log("  (--thu: không lưu vào tệp ánh xạ; lần chạy thật sẽ hỏi có lưu không.)");
  } else {
    const save = await ask("Lưu vào scripts/ung-dung-theo-ten-mien.mjs? (c/k) ");
    if (save.trim().toLowerCase() === "c") {
      const before = readFileSync(REGISTRY_URL, "utf8");
      try {
        writeFileSync(REGISTRY_URL, updateRegistryText(before, dich, dich.app_id), "utf8");
        // Nạp lại và kiểm lại như lúc khởi động: một lần ghi làm hỏng tệp phải bị bắt ở đây, không
        // phải ở lần đẩy kế tiếp của người khác.
        const mod = await import(`${REGISTRY_URL.href}?t=${Date.now()}`);
        kiemBangAnhXa(mod.APP_ID_THEO_TEN_MIEN, mod.APP_ID_APP_CHUNG);
        const reread = chonDich({ ...target, app_id: null }, mod.APP_ID_THEO_TEN_MIEN, mod.APP_ID_APP_CHUNG);
        if (reread.app_id !== dich.app_id) throw new Error("đọc lại tệp không ra App ID vừa ghi.");
        bang = mod.APP_ID_THEO_TEN_MIEN;
        app_chung = mod.APP_ID_APP_CHUNG;
      } catch (loi) {
        writeFileSync(REGISTRY_URL, before, "utf8");
        dungLai(`Không lưu được vào scripts/ung-dung-theo-ten-mien.mjs (tệp đã trả về như cũ): ${loi.message}`);
      }
      console.log(`\n${savedRegistryNote(dich)}\n`);
    }
  }
}

// 3. TOKEN. Từ môi trường; claim `appId` phải bằng App ID đích. Không đúng thì đăng nhập cho ĐÚNG App
// ID ấy (có người ngồi), hoặc từ chối. Token KHÔNG vào `process.env`: bước dựng kế thừa `process.env`
// (xem `zmp()`). `citizen-app/.env` không bao giờ là nguồn token, với bất kỳ đích nào.
let token = process.env.ZMP_TOKEN || undefined;
let token_source = "ZMP_TOKEN của môi trường";
let kiem_token = kiemToken(dich, token, bang);
if (!kiem_token.ok && interactive) {
  if (chi_thu) {
    token_source = `lần chạy thật sẽ đăng nhập Zalo (quét QR) cho App ID ${dich.app_id}, vì ZMP_TOKEN của môi trường`;
  } else {
    console.log(`\n${kiem_token.ly_do}`);
    token = dangNhapRieng(dich.app_id) ?? undefined;
    if (token === undefined) dungLai("Không đẩy: đăng nhập Zalo không thành công, không có token nào được ghi ra.");
    token_source = "đăng nhập Zalo vừa xong (thư mục tạm, đã xoá)";
    kiem_token = kiemToken(dich, token, bang);
  }
}

const own_app = dich.loai === "app-rieng";

/**
 * LOGO XÃ — TẠM THỜI (chủ dự án, 28/09/2026: "copy tạm sang đâu đó dùng trước, sau này nó sẽ cấu hình
 * ở platform-admin"). Ảnh nằm ở `scripts/logo-xa/<tên-miền>.png`, được chép vào `public/logo-xa.png`
 * cho ĐÚNG lần dựng app riêng của xã ấy rồi xoá ngay sau đó, nên không bản dựng nào khác mang logo của
 * một xã. Nguồn thật sẽ là `ho_so_hien_thi_xa.logo_url` của service-platform, đọc lúc chạy.
 */
const LOGO_NGUON = own_app ? new URL(`./logo-xa/${dich.ten_mien}.png`, import.meta.url) : null;
const LOGO_DICH = new URL("../public/logo-xa.png", import.meta.url);
const co_logo = LOGO_NGUON !== null && existsSync(LOGO_NGUON);
/**
 * BANNER XÃ — the same TEMPORARY per-domain bundle exception as the logo above (ADR 0047 §6), for the
 * picture under the home header. `scripts/banner-xa/<domain>.png` is copied to `public/banner-xa.png`
 * for EXACTLY the commune's own-app build and removed in the same `finally`, so no other build — the
 * shared app above all — ever carries one commune's picture. No file for the domain = no banner: the
 * home screen renders nothing in its place (`TrangXa.tsx` `CommuneBanner`). Since 01/10/2026 the real
 * source exists — the commune-posted banners, read at runtime (`?type=banner`, ADR 0067 §5, `TrangXa.tsx`
 * `HomeBanner`) — and this copy is only the FALLBACK under them. Remove this step, `scripts/banner-xa/`
 * and `CommuneBanner` together once every commune has posted a banner (ADR 0067 §5 decision 6).
 */
const BANNER_SOURCE = own_app ? new URL(`./banner-xa/${dich.ten_mien}.png`, import.meta.url) : null;
const BANNER_TARGET = new URL("../public/banner-xa.png", import.meta.url);
const has_banner = BANNER_SOURCE !== null && existsSync(BANNER_SOURCE);

// MÔI TRƯỜNG CỦA BƯỚC DỰNG. Tên miền xã vào bundle khi và chỉ khi đích là app riêng, và luôn là đúng
// `--domain` vừa chọn App ID đích. App ViHAT thì biến bị XOÁ khỏi môi trường dựng: một
// `VIGOV_XA_CO_DINH` còn sót trong shell không được biến app chung thành app của một xã.
const env_dung = { ...process.env };
delete env_dung[BIEN_XA_CO_DINH];
if (own_app) env_dung[BIEN_XA_CO_DINH] = dich.ten_mien;

const sha = git("rev-parse", "--short", "HEAD") || "khong-ro";
const dirty = git("status", "--porcelain") !== "";
const luc = new Date().toISOString().slice(0, 16).replace("T", " ");
// Nhãn mang cả ĐÍCH: cùng một commit đẩy lên hai app thì console Zalo phải phân biệt được.
const mota = nhanPhienBan({ dich, sha, luc, dirty });

// IN RA TRƯỚC KHI LÀM.
const vach = "─".repeat(78);
console.log(`\n${vach}`);
console.log("  zmp deploy — ĐỌC TRƯỚC KHI ĐỂ NÓ CHẠY TIẾP");
for (const dong of planLines({
  dich,
  kiem_token,
  token_source,
  phat_hanh,
  api_host,
  mota,
  logo: co_logo ? `scripts/logo-xa/${dich.ten_mien}.png` : null,
  banner: has_banner ? `scripts/banner-xa/${dich.ten_mien}.png` : null,
})) {
  console.log(dong);
}
console.log(`${vach}\n`);

// `-e` bỏ qua câu hỏi "This is not a ZMP Project?", `-o dist` chỉ thư mục (mặc định của CLI là
// `www`), `-p` chế độ không tương tác, `-m` nhãn phiên bản.
const co_zmp = ["deploy", "-o", "dist", "-e", "-p", "-m", mota];
if (!phat_hanh) co_zmp.splice(1, 0, "-t");

// `--thu` có người ngồi mà token chưa đúng: lần chạy thật sẽ đăng nhập — đó không phải một lần bị chặn.
const will_log_in = chi_thu && interactive && !kiem_token.ok;
if (!kiem_token.ok && !chi_thu) dungLai(`Không đẩy: ${kiem_token.ly_do}`);

// App ID đích đi vào tiến trình con cùng token, để một bản zmp-cli sau này có đọc `APP_ID` thì
// cũng đọc đúng đích chứ không đọc `.env` của máy. Bản 4.0.3 không đọc nó (đã đo) — đích thật vẫn
// là claim của token, và `kiemToken` vừa bảo đảm hai thứ ấy trùng nhau. ZMP_TOKEN luôn được đặt
// (không rỗng), nên zmp-cli không bao giờ rơi về `citizen-app/.env`.
const env_zmp = { ...process.env, APP_ID: dich.app_id, ZMP_TOKEN: token };

if (phat_hanh) {
  // Chỉ đường PHÁT HÀNH mới đếm ngược. Bản thử nghiệm sai thì đẩy lại; bản phát hành sai thì đã
  // ra người dùng thật. Ma sát đặt đúng chỗ có hậu quả, chứ không rải đều cho có.
  //
  // Đếm ngược chạy TRƯỚC cả nhánh `--thu`, có chủ đích: `--thu` phải cho xem đúng thứ lần chạy
  // thật sẽ làm, kể cả quãng dừng.
  process.stdout.write(`Dừng ${GIAY_CHO} giây để bạn kịp Ctrl-C: `);
  for (let con_lai = GIAY_CHO; con_lai > 0; con_lai -= 1) {
    process.stdout.write(`${con_lai}… `);
    nghi(1000);
  }
  console.log("\nBắt đầu.\n");
}

if (chi_thu) {
  console.log("--thu: dừng ở đây, không dựng và không đẩy gì cả. Dòng lệnh sẽ chạy:");
  // In biến LÚC DỰNG: địa chỉ máy chủ quyết định nút đăng nhập nói chuyện với ai, và tên miền xã
  // quyết định app mở vào đâu.
  const xa_dung = own_app ? ` ${BIEN_XA_CO_DINH}=${dich.ten_mien}` : "";
  console.log(`  VIGOV_API_HOST=${api_host || "<CHƯA KHAI>"}${xa_dung} vite build`);
  console.log(`  npx --yes ${ZMP} sync-config dist/index.html`);
  const nguon = will_log_in ? "<đăng nhập>" : "<môi trường>";
  console.log(
    `  APP_ID=${dich.app_id} ZMP_TOKEN=${nguon} npx --yes ${ZMP} ${co_zmp
      .map((c) => (c.includes(" ") ? `"${c}"` : c))
      .join(" ")}`,
  );
  if (!kiem_token.ok && !will_log_in) console.log(`\n⚠ Lần chạy thật sẽ DỪNG ở đây: ${kiem_token.ly_do}`);
  process.exit(0);
}

console.log(`Đẩy lên ${targetName(dich)} — App ID ${dich.app_id}.\n`);
if (co_logo) copyFileSync(LOGO_NGUON, LOGO_DICH);
if (has_banner) copyFileSync(BANNER_SOURCE, BANNER_TARGET);
let ma;
try {
  ma = dung(env_dung);
} finally {
  // `dist/` đã có bản chép; tệp trong `public/` không được nằm lại cho lần dựng App ViHAT kế tiếp.
  if (co_logo) rmSync(LOGO_DICH, { force: true });
  if (has_banner) rmSync(BANNER_TARGET, { force: true });
}
if (ma !== 0) process.exit(ma);

// `app-config.json` được COMMIT và dùng chung. App riêng đổi nó cho ĐÚNG lần đẩy này (ẩn thanh tiêu
// đề Zalo — `appConfigChoLanDay`) rồi TRẢ LẠI nguyên văn, kể cả khi zmp hỏng giữa chừng: một tệp bị bỏ
// quên ở trạng thái app riêng là lần đẩy App ViHAT kế tiếp mất thanh tiêu đề.
const TEP_APP_CONFIG = new URL("../app-config.json", import.meta.url);
const app_config_goc = readFileSync(TEP_APP_CONFIG, "utf8");
try {
  if (own_app) writeFileSync(TEP_APP_CONFIG, appConfigChoLanDay(app_config_goc, true), "utf8");
  ma = zmp(env_zmp, "sync-config", "dist/index.html");
  if (ma === 0) ma = zmp(env_zmp, ...co_zmp);
} finally {
  if (own_app) writeFileSync(TEP_APP_CONFIG, app_config_goc, "utf8");
}
process.exit(ma);
