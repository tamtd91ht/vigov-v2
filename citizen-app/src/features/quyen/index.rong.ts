/**
 * BẢN RỖNG của ba màn quyền — thứ `vite.config.ts` trỏ tới khi dựng biến thể `goc`.
 *
 * Mỗi tên ở đây khai kiểu bằng `typeof` của chính bản thật, nên nó không thể lệch trong im lặng:
 * thêm một export vào `index.ts` mà quên ở đây thì `tsc --noEmit` đỏ ngay, thay vì bản `goc` vỡ
 * lúc dựng — hoặc tệ hơn, dựng được và mang theo `zmp-sdk` vào bản nộp.
 *
 * TOÀN BỘ `import` Ở ĐÂY LÀ `import type`. Kiểu bị xoá lúc biên dịch, nên một tham chiếu kiểu
 * không kéo mô-đun nào vào bundle; một dòng `import { … }` thường ở đây kéo ngược `ManQuyen.tsx`
 * — và qua nó là cả `zmp-sdk` — vào đúng bản dựng sinh ra để không có chúng.
 *
 * NHÃN TAB LÀ CHUỖI RỖNG, KHÔNG PHẢI "Quyền": chuỗi ở đây đi thẳng vào bundle bản `goc`. Tab ấy
 * không bao giờ được vẽ (xem `CO_MAN_QUYEN` và `screens.ts`), nên nhãn không có người đọc — và
 * một chữ không có người đọc mà vẫn nằm trong bản gửi duyệt là một chữ không nên có ở đó.
 */
import type * as DayDu from "./index";

export const CO_MAN_QUYEN: boolean = false;

export const MAN_QUYEN: typeof DayDu.MAN_QUYEN = {
  id: "quyen",
  tabLabel: "",
  headerTitle: "",
  component: () => null,
};

/**
 * RỖNG, và đó là toàn bộ điểm của tệp này: bản `goc` không xin quyền nào, nên chính sách của nó
 * KHÔNG ĐƯỢC có mục nào nói về quyền. Một mảng rỗng ở đây là thứ làm cho câu ấy đúng ở tầng mã
 * chứ không phải ở tầng lời hứa — `bundle-for-zalo.test.ts` grep bundle để chứng minh.
 */
export const MUC_CHINH_SACH_QUYEN: typeof DayDu.MUC_CHINH_SACH_QUYEN = [];
