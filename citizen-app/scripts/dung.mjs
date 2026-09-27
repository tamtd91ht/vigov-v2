/**
 * Dựng bản đẩy lên Zalo — MỘT bản, không biến thể (27/09/2026, xem khối đầu `vite.config.ts`).
 *
 * VÌ SAO VẪN LÀ MỘT TỆP SCRIPT CHỨ KHÔNG PHẢI `vite build` TRẦN TRONG package.json: `deploy.mjs`
 * gọi bước dựng như một hàm và đọc mã thoát của nó trước khi chạy `zmp`. Một lệnh shell thì phải
 * trích dẫn đường dẫn, và trên Windows `npm run` chạy qua `cmd`.
 *
 * Gọi `vite` bằng `process.execPath` chứ không qua shell: không có chuỗi nào phải trích dẫn, nên
 * không có đường dẫn nào chứa dấu cách làm hỏng lệnh.
 *
 * ⚠ BIẾN LÚC DỰNG DUY NHẤT LÀ `VIGOV_API_HOST`, địa chỉ máy chủ của khối đăng nhập. Tệp này KHÔNG
 * chặn khi nó rỗng (dựng thử và chạy test phải được), nhưng `deploy.mjs` thì CHẶN: đẩy một bản chưa
 * khai địa chỉ là nộp một nút đăng nhập không đăng nhập nổi.
 */
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

/** Dựng, trả về mã thoát. */
export function dung() {
  const vite = fileURLToPath(new URL("../node_modules/vite/bin/vite.js", import.meta.url));
  const goc_du_an = fileURLToPath(new URL("..", import.meta.url));

  const ket_qua = spawnSync(process.execPath, [vite, "build"], {
    cwd: goc_du_an,
    stdio: "inherit",
    env: process.env,
  });
  return ket_qua.status ?? 1;
}

// Chạy thẳng (`node scripts/dung.mjs`) thì dựng; được `deploy.mjs` nhập thì chỉ xuất hàm.
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  if (process.argv.length > 2) {
    // Một tên biến thể cũ (`goc`, `day-du`) gõ theo thói quen phải DỪNG, không lặng lẽ bị bỏ qua:
    // người gõ đang tin rằng họ chọn được nội dung bản dựng, và điều đó không còn đúng.
    console.error(
      `Không nhận tham số (${process.argv.slice(2).join(" ")}). Bản dựng không còn biến thể nào — ` +
        "chạy `node scripts/dung.mjs` không tham số.",
    );
    process.exit(2);
  }
  process.exit(dung());
}
