/**
 * BỀ MẶT CỦA MÀN "GỢI Ý GIẢI PHÁP" — một màn NGOÀI TAB, vào bằng menu nhanh của màn chủ.
 *
 * ⚠ KHÔNG CÓ TAB THỨ NĂM, VÀ QUYẾT ĐỊNH ẤY KHÔNG ĐƯỢC LẬT LẠI Ở LƯỢT NÀY.
 *
 *   Thanh tab có bốn ô. Ở 320px mỗi ô chỉ còn khoảng 72px chữ, và `accessibility.test.ts` đo lại
 *   đúng phép chia ấy trên từng TỪ của từng nhãn. Ô thứ năm chỉ vào được bằng cách thu nhỏ chữ —
 *   tức là trả bằng đúng thứ README §Non-negotiables #6 cấm trả.
 *
 *   `tabSangLen: "home"` không phải một mặc định cho có: menu nhanh của màn chủ là đường vào duy
 *   nhất, nên ô "Trang chủ" là ô nói đúng công dân đang ở nhánh nào. Không khai trường ấy thì
 *   không biên dịch được — xem `ChoTrenThanhTab` trong `company-intro/screens.ts`.
 */
import type { ComponentType } from "react";

import type { ThamSoMan } from "../company-intro/dieu-huong";

import { GoiYGiaiPhapScreen, LOI } from "./GoiYGiaiPhapScreen";

export const MAN_GOI_Y_GIAI_PHAP: {
  id: "goi-y";
  headerTitle: string;
  cho: { kieu: "ngoai-tab"; tabSangLen: "home" };
  component: ComponentType<ThamSoMan>;
} = {
  id: "goi-y",
  // Tiêu đề thanh tiêu đề ĐỌC TỪ chính chuỗi mà `<h1>` của màn đọc: hai chỗ giữ một cái tên là
  // hai chỗ sẽ lệch, và lần lệch ấy hiện ra thành thanh tiêu đề nói một đằng, tiêu đề màn một nẻo.
  headerTitle: LOI.tieu_de,
  cho: { kieu: "ngoai-tab", tabSangLen: "home" },
  component: GoiYGiaiPhapScreen,
};
