/**
 * ĐIỀU HƯỚNG BÊN TRONG ỨNG DỤNG — kiểu và mỏ neo, KHÔNG có màn hình nào.
 *
 * VÌ SAO TÁCH KHỎI `screens.ts` DÙ CHỈ CÓ VÀI DÒNG: `screens.ts` nhập cả năm màn hình, nên mọi
 * tệp cần một cái tên ở đây mà nhập từ đó sẽ tạo một vòng nhập (màn hình -> sổ màn hình -> màn
 * hình). Vòng nhập ấy chạy được cho tới ngày ai đó đọc một hằng ngay lúc nạp mô-đun thay vì lúc
 * vẽ, và hôm ấy nó là một `undefined` không ai giải thích nổi. Tệp này là một lá: nó không nhập
 * một màn hình nào.
 */

import { mocTinhNang } from "../tinh-nang/khung";

export type ScreenId =
  | "home"
  | "solutions"
  | "danh-thiep"
  | "goi-y"
  | "tu-van"
  | "yeu-cau"
  | "about"
  | "contact";

/**
 * MỘT ĐIỂM ĐẾN BÊN TRONG ỨNG DỤNG — màn nào, và chỗ nào trong màn ấy.
 *
 * VÌ SAO CẦN `moc` CHỨ KHÔNG CHỈ CẦN `man`:
 *
 *   Menu nhanh trên màn chủ có những mục dẫn tới một KHỐI nằm giữa một màn khác — "Văn phòng" là
 *   khối tìm văn phòng ở giữa tab Liên hệ, "Quyền" là một màn con chỉ hiện ra sau một lần bấm.
 *   Thả người dùng xuống đầu tab rồi để họ tự cuộn đi tìm là một nút làm nửa việc nó hứa, và với
 *   người lớn tuổi thì một nút làm nửa việc khó phân biệt với một nút hỏng.
 *
 *   `moc` có HAI dạng, và khác nhau ấy là chủ đích:
 *     • một `id` CÓ THẬT trong DOM (`MOC_TIM_VAN_PHONG`, `MOC_CHANG_DUONG`) — vỏ app cuộn tới nó;
 *     • tên một màn CON (`MOC_QUAN_LY_QUYEN`) — màn cha đọc và mở thẳng màn con ấy.
 *   Dạng thứ hai không cuộn được vì chỗ ấy chưa được vẽ; hàm cuộn không tìm thấy gì và không làm
 *   gì cả, đúng như nó phải thế.
 */
export type DiemDen = { man: ScreenId; moc?: string };

/**
 * Tham số MỌI màn hình nhận được. Cả hai đều tuỳ chọn, và điều đó có lý do: ba trong năm màn
 * không cần biết gì về điều hướng, và bắt chúng khai một tham số không dùng là mời người sau
 * truyền vào đó một thứ khác.
 */
export type ThamSoMan = {
  /** Chỗ cần tới bên trong màn này. Xem `DiemDen`. */
  moc?: string;
  /** Đi tới một chỗ khác trong app. Vỏ app (`App.tsx`) là nơi DUY NHẤT cài đặt hàm này. */
  onDi?: (diem: DiemDen) => void;
};

/** Mỏ neo dải lịch sử trên màn Về ViHAT. Một hằng, vì `AboutScreen` và menu nhanh cùng đọc nó. */
export const MOC_CHANG_DUONG = "chang-duong";

/**
 * Tên màn con "Quản lý quyền" — KHÔNG phải một `id` trong DOM.
 *
 * Màn quyền vẫn là màn CON của tab Liên hệ, và quyết định ấy KHÔNG bị lật lại ở lượt này: nó dựa
 * trên phép đo bề rộng thanh tab ở 320px (`accessibility.test.ts`), nơi một tab thứ sáu chỉ vào
 * được bằng cách thu nhỏ chữ. Thứ thêm vào chỉ là một đường dẫn thẳng tới nó từ màn chủ.
 */
export const MOC_QUAN_LY_QUYEN = "quan-ly-quyen";

/**
 * Mỏ neo của hai khối tính năng mà menu nhanh dẫn tới — ĐỌC TỪ CHÍNH HÀM SINH RA `id` ẤY.
 *
 * Gõ tay `"tn-van-phong"` ở đây thì ngày quy ước `id` trong `khung.tsx` đổi, hai mục menu im lặng
 * không cuộn đi đâu: người bấm thấy đúng đầu màn và tự kết luận là nút hỏng. Không có gì đỏ lên,
 * vì cả hai bên đều "đúng" khi đọc riêng.
 *
 * ⚠ `MOC_QUET_MA_QR` VÀ `MOC_SO_HOA_THIEP` ĐÃ ĐƯỢC GỠ (22/09/2026). Hai mục menu "Quét QR Lead" và
 * "Chụp danh thiếp" gộp lại thành MỘT mục "Danh thiếp" dẫn tới ĐẦU màn ấy, nên không còn ai đọc
 * hai hằng kia. Một hằng không ai đọc là một hằng người sau tưởng là đang được dùng.
 */
export const MOC_TIM_VAN_PHONG = mocTinhNang("van-phong");

/**
 * Mỏ neo khối đăng nhập trên màn Liên hệ — mục "Đăng nhập" của menu nhanh dẫn thẳng tới đây.
 *
 * ĐỌC TỪ `mocTinhNang`, ĐÚNG NHƯ `MOC_TIM_VAN_PHONG`, và vì đúng cái lý do ghi ở trên: khối ấy là
 * `KhoiDangNhap` trong `features/tinh-nang/LienHeTinhNang.tsx`, dựng bằng `KhungTinhNang ma="dang-nhap"`,
 * nên `id` của tiêu đề nó do chính hàm kia sinh ra. Gõ tay `"tn-dang-nhap"` ở đây là dựng chỗ thứ
 * hai giữ một quy ước, và chỗ thứ hai là chỗ sẽ lệch.
 */
export const MOC_DANG_NHAP = mocTinhNang("dang-nhap");
