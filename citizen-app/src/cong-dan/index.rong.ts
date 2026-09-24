/**
 * BẢN RỖNG của kênh công dân — thứ `vite.config.ts` trỏ `bien-the/cong-dan` tới khi dựng `goc`.
 *
 * TOÀN BỘ `import` Ở ĐÂY LÀ `import type`: kiểu bị xoá lúc biên dịch, nên không mô-đun nào của kênh
 * công dân bị kéo vào bản nộp. Một `import` thường ở đây là đưa cả client ViGov vào bundle gửi duyệt.
 *
 * Kiểu khai bằng `typeof` của bản thật, nên thêm một export vào `index.ts` mà quên ở đây là
 * `tsc --noEmit` đỏ ngay; `cong-dan.test.ts` so hai bộ tên export.
 */
import type { ComponentProps, ComponentType } from "react";

import type * as DayDu from "./index";

export const CO_KENH_CONG_DAN: boolean = false;

export const KenhCongDan: ComponentType<ComponentProps<typeof DayDu.KenhCongDan>> = () => null;
export const NutVaoKenhCongDan: ComponentType<ComponentProps<typeof DayDu.NutVaoKenhCongDan>> = () =>
  null;
