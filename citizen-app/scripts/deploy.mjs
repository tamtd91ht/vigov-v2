/**
 * Dựng → đồng bộ `app-config.json` → đẩy lên Zalo. KHÔNG hỏi câu nào, nhưng NÓI RA nó sắp làm gì.
 *
 * ```
 * node scripts/deploy.mjs                                      APP CHUNG, bản thử nghiệm (-t)
 * node scripts/deploy.mjs --phat-hanh                          APP CHUNG, BẢN PHÁT HÀNH (bỏ -t)
 * node scripts/deploy.mjs --domain=<tên-miền-xã>               APP RIÊNG, bản thử nghiệm (-t)
 * node scripts/deploy.mjs --domain=<tên-miền-xã> --phat-hanh   APP RIÊNG, BẢN PHÁT HÀNH
 * node scripts/deploy.mjs … --thu                              IN RA rồi DỪNG, không làm gì cả
 * ```
 *
 * HAI LUỒNG (ADR 0047): có `--domain` thì đẩy lên App ID riêng của xã ấy (tra trong
 * `ung-dung-theo-ten-mien.mjs`), không có thì đẩy lên app chung. Tên miền CHỈ chọn đích; bundle
 * là một, không biến thể (27/09/2026 — xem đầu `vite.config.ts`). `--bien-the` đã bỏ và bị từ chối.
 *
 * ⚠ ĐÍCH DO `ZMP_TOKEN` QUYẾT, KHÔNG DO `APP_ID` — đã đo, xem đầu `dich-den.mjs`. Nên đường app
 * riêng đòi `ZMP_TOKEN` trong MÔI TRƯỜNG và kiểm claim `appId` của nó khớp App ID đích trước khi
 * chạy gì cả; nó không bao giờ dùng `citizen-app/.env` của máy. Đường app chung giữ nguyên như
 * trước: không có token trong môi trường thì zmp-cli tự đọc `.env` (script không đọc tệp ấy).
 *
 * `--thu` in ra đúng kế hoạch và đúng dòng lệnh sẽ chạy, rồi dừng. Đọc kế hoạch mà không phải
 * đặt cược một lần đẩy để đọc nó.
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
 * Nhãn phiên bản mang cả ĐÍCH: hai lần đẩy cùng một commit, một lên app chung một lên app của xã,
 * mà nhãn giống nhau thì console Zalo có hai dòng không phân biệt được — và dòng người ta gửi đi
 * duyệt là dòng đoán ra.
 */
import { spawnSync } from "node:child_process";

import { docCauHinh } from "./cau-hinh.mjs";
import {
  chonDich,
  docCo,
  kiemBangAnhXa,
  kiemToken,
  laPlaceholder,
  nhanPhienBan,
} from "./dich-den.mjs";
import { dung } from "./dung.mjs";
import { APP_ID_APP_CHUNG, APP_ID_THEO_TEN_MIEN } from "./ung-dung-theo-ten-mien.mjs";

const ZMP = "zmp-cli@4.0.3";
const GIAY_CHO = 5;

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

/** Ngủ ĐỒNG BỘ: người chạy phải kịp đọc rồi Ctrl-C, mà Ctrl-C chỉ tới được khi chưa làm gì cả. */
function nghi(mili_giay) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, mili_giay);
}

// ---------------------------------------------------------------------------------------------

/** Mọi lỗi chọn đích đều là exit 2 kèm câu nói phải làm gì — không bao giờ một stack trace. */
function dungLai(loi) {
  console.error(`
${loi.message}
`);
  process.exit(2);
}

let co, dich;
try {
  kiemBangAnhXa(APP_ID_THEO_TEN_MIEN, APP_ID_APP_CHUNG);
  co = docCo(process.argv.slice(2));
  dich = chonDich(co.ten_mien, APP_ID_THEO_TEN_MIEN, APP_ID_APP_CHUNG);
} catch (loi) {
  dungLai(loi);
}
const { phat_hanh, chi_thu } = co;

// Token CHỈ đọc từ môi trường; `.env` của máy là việc của zmp-cli, script không mở nó. Kết quả
// kiểm được in ra kế hoạch; lần chạy thật thì từ chối nếu không qua — `--thu` vẫn in hết để
// người đọc thấy lần chạy thật SẼ bị chặn vì đâu.
const kiem_token = kiemToken(dich, process.env.ZMP_TOKEN, APP_ID_THEO_TEN_MIEN);

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
 */
// ĐỌC CÙNG MỘT NGUỒN VỚI BƯỚC DỰNG. Script này chạy NGOÀI Vite nên nó không tự thấy
// `.env.local`; `docCauHinh` là chỗ duy nhất biết cách đọc tệp ấy. Nhập lại nó ở đây là cách
// duy nhất để "deploy thấy host" và "vite thấy host" không bao giờ lệch nhau — chép logic đọc
// ra hai nơi thì ngày chúng lệch là ngày script chặn một bản dựng hợp lệ, hoặc tệ hơn: cho qua
// một bản không có host rồi đẩy nó lên Zalo.
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
      "  Hoặc đặt cho đúng một lần chạy: VIGOV_API_HOST=https://<host> npm run zmp:deploy\n" +
      "  (Thêm --thu để chỉ in ra kế hoạch mà không cần biến này.)\n",
  );
  process.exit(2);
}

const sha = git("rev-parse", "--short", "HEAD") || "khong-ro";
const dirty = git("status", "--porcelain") !== "";
const luc = new Date().toISOString().slice(0, 16).replace("T", " ");
// Nhãn mang cả ĐÍCH: cùng một commit đẩy lên hai app thì console Zalo phải phân biệt được.
const mota = nhanPhienBan({ dich, sha, luc, dirty });

// IN RA TRƯỚC KHI LÀM. Người chạy lệnh phải đọc được những điều quyết định hậu quả: đẩy lên app
// nào, vào bản thử nghiệm hay bản phát hành, và nhãn nào sẽ hiện trong console Zalo.
const vach = "─".repeat(78);
console.log(`\n${vach}`);
console.log("  zmp deploy — ĐỌC TRƯỚC KHI ĐỂ NÓ CHẠY TIẾP");
console.log(
  dich.loai === "app-rieng"
    ? `  Đích     : APP RIÊNG của tên miền ${dich.ten_mien}`
    : "  Đích     : APP CHUNG (không truyền --domain)",
);
console.log(
  `  App ID   : ${
    dich.app_id === null
      ? "(tệp ánh xạ chưa khai App ID app chung — ZMP_TOKEN quyết)"
      : laPlaceholder(dich.app_id)
        ? `${dich.app_id}  (PLACEHOLDER — lần chạy thật sẽ bị từ chối)`
        : dich.app_id
  }`,
);
console.log(`  Token    : ${kiem_token.ok ? "" : "KHÔNG QUA — "}${kiem_token.ly_do}`);
console.log(
  phat_hanh
    ? "  Loại bản : PHÁT HÀNH — bỏ -t. Bản này ra người dùng thật / gửi duyệt."
    : "  Loại bản : THỬ NGHIỆM — có -t. Chỉ mở được bằng link bản thử nghiệm.",
);
console.log(`  Nhãn     : ${mota}`);
// IN RA ĐỊA CHỈ MÁY CHỦ SẼ ĐI VÀO BUNDLE. Đây là thứ quyết định nút đăng nhập nói chuyện với ai,
// và nó được nung vào tệp gửi đi — người chạy lệnh phải đọc được nó trước khi để lệnh chạy tiếp.
console.log(`  Máy chủ  : ${api_host === "" ? "(chưa khai — chỉ hợp lệ với --thu)" : api_host}`);
console.log("  Các bước : vite build → zmp sync-config → zmp deploy");
console.log(`${vach}\n`);

// `-e` bỏ qua câu hỏi "This is not a ZMP Project?", `-o dist` chỉ thư mục (mặc định của CLI là
// `www`), `-p` chế độ không tương tác, `-m` nhãn phiên bản.
const co_zmp = ["deploy", "-o", "dist", "-e", "-p", "-m", mota];
if (!phat_hanh) co_zmp.splice(1, 0, "-t");

if (!kiem_token.ok && !chi_thu) {
  console.error(`
Không đẩy: ${kiem_token.ly_do}
`);
  process.exit(2);
}

// App ID đích đi vào tiến trình con cùng token, để một bản zmp-cli sau này có đọc `APP_ID` thì
// cũng đọc đúng đích chứ không đọc `.env` của máy. Bản 4.0.3 không đọc nó (đã đo) — đích thật vẫn
// là claim của token, và `kiemToken` vừa bảo đảm hai thứ ấy trùng nhau.
const env_zmp =
  dich.loai === "app-rieng" ? { ...process.env, APP_ID: dich.app_id } : { ...process.env };

if (phat_hanh) {
  // Chỉ đường PHÁT HÀNH mới đếm ngược. Bản thử nghiệm sai thì đẩy lại; bản phát hành sai thì đã
  // ra người dùng thật. Ma sát đặt đúng chỗ có hậu quả, chứ không rải đều cho có.
  //
  // Đếm ngược chạy TRƯỚC cả nhánh `--thu`, có chủ đích: `--thu` phải cho xem đúng thứ lần chạy
  // thật sẽ làm, kể cả quãng dừng. Một bản diễn tập bỏ mất cái phanh là một bản diễn tập nói
  // sai về lần chạy thật.
  process.stdout.write(`Dừng ${GIAY_CHO} giây để bạn kịp Ctrl-C: `);
  for (let con_lai = GIAY_CHO; con_lai > 0; con_lai -= 1) {
    process.stdout.write(`${con_lai}… `);
    nghi(1000);
  }
  console.log("\nBắt đầu.\n");
}

if (chi_thu) {
  console.log("--thu: dừng ở đây, không dựng và không đẩy gì cả. Dòng lệnh sẽ chạy:");
  // In biến LÚC DỰNG: địa chỉ máy chủ quyết định nút đăng nhập nói chuyện với ai. Một bản diễn
  // tập giấu mất nó là một bản diễn tập nói sai về lần chạy thật.
  console.log(`  VIGOV_API_HOST=${api_host || "<CHƯA KHAI>"} vite build`);
  console.log(`  npx --yes ${ZMP} sync-config dist/index.html`);
  const tien_to = dich.loai === "app-rieng" ? `APP_ID=${dich.app_id} ZMP_TOKEN=<môi trường> ` : "";
  console.log(
    `  ${tien_to}npx --yes ${ZMP} ${co_zmp.map((c) => (c.includes(" ") ? `"${c}"` : c)).join(" ")}`,
  );
  if (!kiem_token.ok) console.log(`
⚠ Lần chạy thật sẽ DỪNG ở đây: ${kiem_token.ly_do}`);
  process.exit(0);
}

let ma = dung();
if (ma !== 0) process.exit(ma);

ma = zmp(env_zmp, "sync-config", "dist/index.html");
if (ma !== 0) process.exit(ma);

process.exit(zmp(env_zmp, ...co_zmp));
