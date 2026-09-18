/**
 * Đẩy bản dựng lên Zalo, KHÔNG hỏi câu nào.
 *
 * `npm run zmp:deploy` trước đây dừng lại ba lần: "This is not a ZMP Project?", "where is your
 * dist folder", và "description". Cả ba đều có cờ tắt — `-e`, `-o dist`, `-m` — cộng `-p` cho
 * chế độ không tương tác nói chung.
 *
 * VÌ SAO LÀ MỘT TỆP SCRIPT CHỨ KHÔNG PHẢI MỘT DÒNG DÀI TRONG package.json:
 *
 *   Vì `-m` cần một giá trị ĐỔI THEO TỪNG LẦN ĐẨY. Một mô tả cố định thì mọi phiên bản trong
 *   console Zalo mang cùng một nhãn, và lúc cần biết "bản đang chạy là bản nào" thì không còn
 *   gì để tra. Nhãn ở đây là `<sha ngắn> · <ngày giờ>`, nên mỗi bản đẩy truy ngược được về một
 *   commit.
 *
 *   Tính nhãn ấy trong `package.json` thì không được: trên Windows `npm run` chạy qua `cmd`,
 *   nơi `$(git rev-parse …)` chỉ là một chuỗi ký tự. Node thì chạy giống nhau ở mọi máy.
 *
 * DẤU `dirty` KHÔNG PHẢI TRANG TRÍ. Đẩy từ một cây làm việc còn thay đổi chưa commit nghĩa là
 * bản trên Zalo KHÔNG ứng với commit nào cả — không ai dựng lại được nó, kể cả chính người vừa
 * đẩy. Nhãn nói ra điều đó thay vì để người đọc console tưởng `<sha>` là đủ để truy.
 */
import { spawnSync } from "node:child_process";

const ZMP = "zmp-cli@4.0.3";

function git(...args) {
  const r = spawnSync("git", args, { encoding: "utf8" });
  return r.status === 0 ? r.stdout.trim() : "";
}

const sha = git("rev-parse", "--short", "HEAD") || "khong-ro";
const ban = git("status", "--porcelain") ? " · dirty" : "";
const luc = new Date().toISOString().slice(0, 16).replace("T", " ");
const mota = `${sha} · ${luc}${ban}`;

// `-t` = bản THỬ NGHIỆM. Bỏ nó là đẩy bản phát hành, và đó phải là một quyết định có người gõ
// ra chứ không phải mặc định của một script chạy tự động.
const args = ["--yes", ZMP, "deploy", "-o", "dist", "-t", "-e", "-p", "-m", mota];

console.log(`zmp deploy → ${mota}`);
const r = spawnSync("npx", args, { stdio: "inherit", shell: process.platform === "win32" });
process.exit(r.status ?? 1);
