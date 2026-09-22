/**
 * BỀ MẶT YÊU CẦU — hai màn NGOÀI TAB, cả hai vào bằng menu nhanh hoặc bằng một nút trên màn chủ.
 *
 * ⚠ THANH TAB VẪN BỐN Ô, VÀ QUYẾT ĐỊNH ẤY KHÔNG ĐƯỢC LẬT LẠI Ở LƯỢT NÀY. Ở 320px mỗi ô chỉ còn
 * khoảng 72px chữ (`accessibility.test.ts` đo lại trên từng TỪ của từng nhãn); ô thứ năm chỉ vào
 * được bằng cách thu nhỏ chữ — tức trả bằng đúng thứ README §Non-negotiables #6 cấm trả.
 *
 * `tabSangLen: "home"` không phải một mặc định cho có: cả hai màn vào từ màn chủ, nên ô "Trang
 * chủ" là ô nói đúng công dân đang ở nhánh nào. Không khai trường ấy thì không biên dịch được.
 */
import type { ComponentType } from "react";

import type { ThamSoMan } from "../company-intro/dieu-huong";

import { TU_VAN, YEU_CAU_CUA_TOI } from "./noi-dung";
import { TuVanBaoGiaScreen } from "./TuVanBaoGiaScreen";
import { YeuCauCuaToiScreen } from "./YeuCauCuaToiScreen";

type ManNgoaiTab<Id extends string> = {
  id: Id;
  headerTitle: string;
  cho: { kieu: "ngoai-tab"; tabSangLen: "home" };
  component: ComponentType<ThamSoMan>;
};

// Tiêu đề thanh tiêu đề ĐỌC TỪ chính chuỗi mà `<h1>` của màn đọc: hai chỗ giữ một cái tên là hai
// chỗ sẽ lệch, và lần lệch ấy hiện ra thành thanh tiêu đề nói một đằng, tiêu đề màn một nẻo.
export const MAN_TU_VAN: ManNgoaiTab<"tu-van"> = {
  id: "tu-van",
  headerTitle: TU_VAN.tieu_de,
  cho: { kieu: "ngoai-tab", tabSangLen: "home" },
  component: TuVanBaoGiaScreen,
};

export const MAN_YEU_CAU: ManNgoaiTab<"yeu-cau"> = {
  id: "yeu-cau",
  headerTitle: YEU_CAU_CUA_TOI.tieu_de,
  cho: { kieu: "ngoai-tab", tabSangLen: "home" },
  component: YeuCauCuaToiScreen,
};
