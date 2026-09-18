/**
 * BẢN RỖNG của lớp khám phá — thứ `vite.config.ts` trỏ tới khi dựng biến thể `goc`.
 *
 * Mỗi hàm ở đây khai kiểu bằng `typeof` của chính bản thật, nên nó **không thể lệch trong im
 * lặng**: thêm một export vào `index.ts` mà quên ở đây thì `tsc --noEmit` đỏ ngay, thay vì bản
 * `goc` vỡ lúc dựng — hoặc tệ hơn, dựng được và mang theo thứ đáng lẽ không có.
 *
 * TOÀN BỘ `import` Ở ĐÂY LÀ `import type`, VÀ ĐÓ LÀ ĐIỀU KIỆN ĐỂ TỆP NÀY CÓ NGHĨA:
 *
 *   Kiểu bị xoá lúc biên dịch, nên tham chiếu kiểu không kéo mô-đun nào vào bundle. Một dòng
 *   `import { … }` thường ở đây kéo ngược `demo-danh-muc-xa.ts` vào bản gửi duyệt và làm hỏng
 *   đúng việc mà tệp này sinh ra để làm.
 *
 * `phanGiaiGoiY` trả về "không có gợi ý nào" — đúng nhánh mà `goi-y.ts` dùng cho mọi trường hợp
 * không tra được xã. Nhánh ấy không bao giờ được vẽ ra, vì `CO_LOP_KHAM_PHA` bằng `false` đã
 * chặn từ `App.tsx`; giá trị này chỉ để hàm có một câu trả lời đúng kiểu.
 */
import type { ComponentProps, ComponentType } from "react";

import type * as DayDu from "./index";

export const CO_LOP_KHAM_PHA: boolean = false;

export type { XaDemo } from "./demo-danh-muc-xa";

export const phanGiaiGoiY: typeof DayDu.phanGiaiGoiY = () => ({
  kieu: "phai-chon",
  li_do: "khong-tra-duoc",
});

/** Ba màn của lớp khám phá, rỗng. Không vẽ gì, và không có chuỗi nào đi vào bundle. */
export const ChonXaScreen: ComponentType<ComponentProps<typeof DayDu.ChonXaScreen>> = () => null;
export const GoiYXaScreen: ComponentType<ComponentProps<typeof DayDu.GoiYXaScreen>> = () => null;
export const TrangXaScreen: ComponentType<ComponentProps<typeof DayDu.TrangXaScreen>> = () => null;

/**
 * CHUỖI RỖNG, KHÔNG PHẢI NHÃN THẬT — cùng lý do với `MAN_QUYEN.tabLabel` rỗng trước đây: chuỗi
 * viết ở đây đi thẳng vào bundle bản `goc`, và bản `goc` là bản nộp. Nhánh vẽ chúng không bao
 * giờ chạy khi `CO_LOP_KHAM_PHA` là `false`, nên không ai đọc hai chuỗi này; một chữ không có
 * người đọc mà vẫn nằm trong bản gửi duyệt là một chữ không nên có ở đó.
 */
export const NHAN_KHAM_PHA: typeof DayDu.NHAN_KHAM_PHA = {
  tieu_de_chon_xa: "",
  nut_doi_xa: "",
};
