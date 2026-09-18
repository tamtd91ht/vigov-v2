/**
 * Dựng → đồng bộ `app-config.json` → đẩy lên Zalo. KHÔNG hỏi câu nào, nhưng NÓI RA nó sắp làm gì.
 *
 * ```
 * node scripts/deploy.mjs --bien-the=day-du              bản ĐẦY ĐỦ, bản thử nghiệm (-t)
 * node scripts/deploy.mjs --bien-the=goc                 bản GỐC,    bản thử nghiệm (-t)
 * node scripts/deploy.mjs --bien-the=goc --phat-hanh     bản GỐC,    BẢN PHÁT HÀNH (bỏ -t)
 * node scripts/deploy.mjs --bien-the=goc --phat-hanh --thu   IN RA rồi DỪNG, không làm gì cả
 * ```
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
 *   Và bước dựng phải nằm ở đây vì **biến thể quyết định ở lúc dựng** (`vite.config.ts`). Dựng
 *   ngoài rồi đẩy trong là hai lệnh có thể lệch nhau: dựng `day-du`, đẩy dưới nhãn `goc`.
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
 * Nhãn phiên bản mang cả BIẾN THỂ: hai lần đẩy cùng một commit, một `goc` một `day-du`, mà nhãn
 * giống nhau thì console Zalo có hai dòng không phân biệt được — và dòng người ta gửi đi duyệt
 * là dòng đoán ra.
 */
import { spawnSync } from "node:child_process";

import { BIEN_THE, dung, MO_TA_BIEN_THE } from "./dung.mjs";

const ZMP = "zmp-cli@4.0.3";
const GIAY_CHO = 5;

function git(...args) {
  const r = spawnSync("git", args, { encoding: "utf8" });
  return r.status === 0 ? r.stdout.trim() : "";
}

/** `npx` là tệp lệnh của Windows, nên bước này cần shell — khác với `vite` trong dung.mjs. */
function zmp(...args) {
  const r = spawnSync("npx", ["--yes", ZMP, ...args], {
    stdio: "inherit",
    shell: process.platform === "win32",
  });
  return r.status ?? 1;
}

/** Ngủ ĐỒNG BỘ: người chạy phải kịp đọc rồi Ctrl-C, mà Ctrl-C chỉ tới được khi chưa làm gì cả. */
function nghi(mili_giay) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, mili_giay);
}

// ---------------------------------------------------------------------------------------------

const co = process.argv.slice(2);
const bien_the = (co.find((c) => c.startsWith("--bien-the="))?.split("=")[1] ?? "day-du").trim();
const phat_hanh = co.includes("--phat-hanh");
const chi_thu = co.includes("--thu");

if (!BIEN_THE.includes(bien_the)) {
  console.error(`--bien-the="${bien_the}" không có. Chỉ nhận: ${BIEN_THE.join(" · ")}`);
  process.exit(2);
}

const sha = git("rev-parse", "--short", "HEAD") || "khong-ro";
const ban = git("status", "--porcelain") ? " · dirty" : "";
const luc = new Date().toISOString().slice(0, 16).replace("T", " ");
const mota = `${bien_the} · ${sha} · ${luc}${ban}`;

// IN RA TRƯỚC KHI LÀM. Người chạy lệnh phải đọc được ba điều quyết định hậu quả: dựng biến thể
// nào, đẩy vào bản thử nghiệm hay bản phát hành, và nhãn nào sẽ hiện trong console Zalo.
const vach = "─".repeat(78);
console.log(`\n${vach}`);
console.log("  zmp deploy — ĐỌC TRƯỚC KHI ĐỂ NÓ CHẠY TIẾP");
console.log(`  Biến thể : ${bien_the}  (${MO_TA_BIEN_THE[bien_the]})`);
console.log(
  phat_hanh
    ? "  Loại bản : PHÁT HÀNH — bỏ -t. Bản này ra người dùng thật / gửi duyệt."
    : "  Loại bản : THỬ NGHIỆM — có -t. Chỉ mở được bằng link bản thử nghiệm.",
);
console.log(`  Nhãn     : ${mota}`);
console.log("  Các bước : vite build → zmp sync-config → zmp deploy");
console.log(`${vach}\n`);

// `-e` bỏ qua câu hỏi "This is not a ZMP Project?", `-o dist` chỉ thư mục (mặc định của CLI là
// `www`), `-p` chế độ không tương tác, `-m` nhãn phiên bản.
const co_zmp = ["deploy", "-o", "dist", "-e", "-p", "-m", mota];
if (!phat_hanh) co_zmp.splice(1, 0, "-t");

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
  console.log(`  VIGOV_BIEN_THE=${bien_the} vite build`);
  console.log(`  npx --yes ${ZMP} sync-config dist/index.html`);
  console.log(`  npx --yes ${ZMP} ${co_zmp.map((c) => (c.includes(" ") ? `"${c}"` : c)).join(" ")}`);
  process.exit(0);
}

let ma = dung(bien_the);
if (ma !== 0) process.exit(ma);

ma = zmp("sync-config", "dist/index.html");
if (ma !== 0) process.exit(ma);

process.exit(zmp(...co_zmp));
