/**
 * CỬA DUY NHẤT vào ba màn quyền — cùng cơ chế, cùng lý do với `features/kham-pha/index.ts`.
 *
 * VÌ SAO LỚP NÀY PHẢI BIẾN MẤT ĐƯỢC:
 *
 *   Ba màn này nhập `zmp-sdk` — 256 kB thô / 64 kB gzip, và là cửa vào mọi API dữ liệu công dân
 *   của nền tảng. Biến thể `goc` (bản nộp tối thiểu) không được mang theo một byte nào của nó:
 *   không phải vì mã sai, mà vì một bản nộp nói *"app này không thu thập gì"* mà trong bundle có
 *   sẵn `getPhoneNumber` là một bản nộp tự mâu thuẫn.
 *
 *   Tree-shaking KHÔNG làm được việc đó (một `import` tĩnh có mặt là mô-đun vào bundle). Thứ làm
 *   được là `resolve.alias`: `bien-the/quyen` trỏ sang `index.rong.ts` ở bản `goc`.
 *
 * BA BIẾN THỂ, HAI CÂU TRẢ LỜI CHO CỬA NÀY (xem `vite.config.ts`):
 *
 *   | Biến thể | `bien-the/quyen` | Vì sao |
 *   |---|---|---|
 *   | `goc`    | `index.rong.ts` | Bản nộp tối thiểu — không xin quyền nào |
 *   | `quyen`  | `index.ts`      | **Bản nộp xin quyền** — Zalo phải THẤY chỗ dùng ba quyền |
 *   | `day-du` | `index.ts`      | Demo nội bộ |
 *
 * → README §"Ba biến thể bản dựng"
 */
import type { ComponentType } from "react";

import { KhuQuyen } from "./ManQuyen";

/**
 * Ba màn quyền có mặt trong bản dựng này hay không.
 *
 * `features/company-intro/screens.ts` đọc cờ này để thêm (hay không thêm) một tab. Bản rỗng trả
 * `false`, và khi ấy app đúng bằng bốn màn giới thiệu — không có tab thứ năm dẫn tới một màn
 * trống. Kiểu ghi rõ `boolean` chứ không để suy ra `true`: bản rỗng phải gán được vào cùng kiểu.
 */
export const CO_MAN_QUYEN: boolean = true;

/**
 * Tab của khu vực ba màn quyền.
 *
 * Nhãn nằm Ở ĐÂY chứ không ở `screens.ts`, và đó là chủ đích: `screens.ts` không nằm sau alias,
 * nên mọi chữ viết trong đó sẽ có mặt trong CẢ bản `goc`. Đặt nhãn sau cửa này thì bản `goc`
 * không mang theo một chữ nào của lớp quyền.
 */
export const MAN_QUYEN: {
  id: "quyen";
  tabLabel: string;
  headerTitle: string;
  component: ComponentType;
} = {
  id: "quyen",
  tabLabel: "Quyền",
  headerTitle: "Quyền ứng dụng cần",
  component: KhuQuyen,
};

/**
 * ĐÚNG HAI CÁI TÊN ĐI QUA CỬA NÀY, và đó là chủ đích: cửa càng hẹp thì bản rỗng càng khó lệch.
 * Ba màn bên trong nhập thẳng lẫn nhau (cùng thư mục) và bộ test của chúng cũng vậy — chỉ mã
 * NGOÀI `src/features/quyen/` mới bắt buộc đi qua `bien-the/quyen`.
 */
