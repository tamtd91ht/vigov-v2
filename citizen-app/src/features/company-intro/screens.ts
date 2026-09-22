import type { ComponentType } from "react";

import { MAN_GOI_Y_GIAI_PHAP } from "../goi-y-giai-phap/index";
import { MAN_DANH_THIEP } from "../tinh-nang/index";
import { MAN_TU_VAN, MAN_YEU_CAU } from "../yeu-cau/index";

import type { ScreenId, ThamSoMan } from "./dieu-huong";
import { AboutScreen } from "./AboutScreen";
import { ContactScreen } from "./ContactScreen";
import { HomeScreen } from "./HomeScreen";
import { GridGlyph, HomeGlyph, InfoGlyph, MailGlyph } from "./icons";
import { SolutionsScreen } from "./SolutionsScreen";

/**
 * The screen registry — the ONE place a screen is declared.
 *
 * Adding a screen means adding a row here; the tab bar, the header title and the routing
 * switch all read from this list. A second list would drift from this one, and the symptom of
 * that drift is a tab that leads nowhere.
 *
 * ⚠ NĂM MÀN, BỐN TAB — ĐỔI 21/09/2026 (khuya), THEO BẢN MẪU CỦA PM.
 *
 *   Bản mẫu vẽ BỐN tab, mỗi tab một biểu tượng: Trang chủ · Giải pháp · Về ViHAT · Liên hệ. Màn
 *   "Danh thiếp" không có tab trong bản mẫu, nên nó mất tab — nhưng KHÔNG mất đường tới: hai tính
 *   năng danh thiếp (`scanQRCode`, `keepScreen`, `downloadFile` sống ở đó) vào bằng hai mục của
 *   menu nhanh trên màn chủ, đúng như bản mẫu bày chúng ("Quét QR Lead", "Chụp danh thiếp").
 *
 *   MỘT MÀN KHÔNG CÓ TAB VẪN PHẢI TRẢ LỜI "TAB NÀO SÁNG KHI TÔI ĐANG MỞ" — đó là lý do `cho` là
 *   một hợp kiểu phân biệt chứ không phải một cờ tuỳ chọn. Không khai thì không biên dịch được.
 *   Bỏ trống câu ấy nghĩa là công dân đứng ở màn Danh thiếp và thanh tab không sáng ô nào: họ
 *   không biết mình đang ở đâu trong app, và với người lớn tuổi thì "không biết mình ở đâu" là
 *   lúc họ đóng app.
 */

/**
 * Kiểu và mỏ neo điều hướng sống ở `dieu-huong.ts` — một tệp LÁ, không nhập màn hình nào — và
 * được xuất lại ở đây để mọi nơi đang nhập từ `./screens` không phải đổi đường nhập. Xem khối chú
 * thích đầu tệp ấy về vòng nhập.
 */
export {
  MOC_CHANG_DUONG,
  MOC_DANG_NHAP,
  MOC_QUAN_LY_QUYEN,
  MOC_TIM_VAN_PHONG,
} from "./dieu-huong";
export type { DiemDen, ScreenId, ThamSoMan } from "./dieu-huong";

/** Hình vẽ trong bundle, không phải tệp ảnh. Xem `icons.tsx`. */
export type GlyphComponent = ComponentType<{ className?: string }>;

/**
 * Chỗ của một màn trên thanh tab. HAI DẠNG, KHÔNG CÓ DẠNG THỨ BA VÀ KHÔNG CÓ MẶC ĐỊNH.
 *
 *   • `tab` — màn có ô riêng trên thanh tab: một nhãn CHỮ và một hình. Cả hai bắt buộc; một tab
 *     chỉ có hình là một tab phải học thuộc mới dùng được.
 *   • `ngoai-tab` — màn tới được bằng đường khác (menu nhanh), và `tabSangLen` nói ô nào sáng khi
 *     công dân đang đứng ở đây. Không có trường ấy thì thanh tab không sáng ô nào và người dùng
 *     mất dấu vị trí của mình trong app.
 */
export type ChoTrenThanhTab =
  | { kieu: "tab"; nhan: string; glyph: GlyphComponent }
  | { kieu: "ngoai-tab"; tabSangLen: ScreenId };

export type ScreenDefinition = {
  id: ScreenId;
  /** Shown in the app header under the entity name, so the citizen always knows where they are. */
  headerTitle: string;
  cho: ChoTrenThanhTab;
  component: ComponentType<ThamSoMan>;
};

/** Sổ màn hình — giống nhau ở mọi biến thể bản dựng. */
export const SCREENS: readonly ScreenDefinition[] = [
  {
    id: "home",
    headerTitle: "Trang chủ",
    cho: { kieu: "tab", nhan: "Trang chủ", glyph: HomeGlyph },
    component: HomeScreen,
  },
  {
    id: "solutions",
    headerTitle: "Giải pháp",
    cho: { kieu: "tab", nhan: "Giải pháp", glyph: GridGlyph },
    component: SolutionsScreen,
  },
  MAN_DANH_THIEP,
  MAN_GOI_Y_GIAI_PHAP,
  // BỀ MẶT YÊU CẦU (giai đoạn B, 22/09/2026) — hai màn NGOÀI TAB. Thanh tab vẫn bốn ô; xem
  // `features/yeu-cau/index.ts` về vì sao ô thứ năm là một phép đo chứ không một khẩu vị.
  MAN_TU_VAN,
  MAN_YEU_CAU,
  {
    id: "about",
    headerTitle: "Về ViHAT",
    cho: { kieu: "tab", nhan: "Về ViHAT", glyph: InfoGlyph },
    component: AboutScreen,
  },
  {
    id: "contact",
    headerTitle: "Liên hệ",
    cho: { kieu: "tab", nhan: "Liên hệ", glyph: MailGlyph },
    component: ContactScreen,
  },
];

/** Một màn CÓ ô trên thanh tab — kiểu đã thu hẹp, để `cho.nhan` đọc được mà không phải ép kiểu. */
export type ManCoTab = ScreenDefinition & {
  cho: Extract<ChoTrenThanhTab, { kieu: "tab" }>;
};

/**
 * Những ô thanh tab vẽ ra, theo đúng thứ tự sổ màn hình khai.
 *
 * LỌC TỪ `SCREENS`, KHÔNG PHẢI MỘT DANH SÁCH THỨ HAI: hai danh sách sẽ lệch, và triệu chứng của
 * lần lệch ấy là một tab dẫn tới một màn không còn tồn tại.
 */
export const TABS: readonly ManCoTab[] = SCREENS.filter(
  (man): man is ManCoTab => man.cho.kieu === "tab",
);

/**
 * Ô nào trên thanh tab sáng lên khi màn `id` đang mở.
 *
 * Màn có tab thì là chính nó; màn không có tab thì là ô cha nó đã khai. Hàm này là chỗ DUY NHẤT
 * trả lời câu ấy — một `if` viết lại trong `TabBar` là chỗ thứ hai để lệch.
 */
export function tabDangSang(id: ScreenId): ScreenId {
  const man = findScreen(id);
  return man.cho.kieu === "tab" ? man.id : man.cho.tabSangLen;
}

/**
 * Giữ lại cái tên cũ cho những chỗ chỉ quan tâm tới bốn màn giới thiệu công ty.
 *
 * `bundle-for-zalo.test.ts` dùng nó để khẳng định bản `goc` vẫn là một app có nội dung, chứ
 * không phải một bản rỗng — và phép kiểm ấy không nên đỏ chỉ vì thứ tự tab đổi.
 *
 * ⚠ ĐÂY LÀ ĐÚNG `TABS`, KHÔNG PHẢI MỘT PHÉP LỌC THỨ HAI — đổi 22/09/2026. Trước đây nó loại trừ
 * MỘT id viết thẳng (`danh-thiep`), và đó là một danh sách âm: ngày màn "Gợi ý giải pháp" ra đời
 * cũng là một màn ngoài tab, nó lặng lẽ lọt vào đây và ca "bốn màn giới thiệu" đỏ lên vì một lý
 * do không liên quan gì tới nội dung. Lọc theo TÍNH CHẤT — có ô trên thanh tab hay không — thì
 * danh sách tự đúng mỗi lần sổ màn hình đổi.
 */
export const MAN_GIOI_THIEU: readonly ScreenDefinition[] = TABS;

export const DEFAULT_SCREEN_ID: ScreenId = "home";

export function findScreen(id: ScreenId): ScreenDefinition {
  const screen = SCREENS.find((candidate) => candidate.id === id);
  // Fail closed rather than render a blank page: an unknown id is a programming error, and a
  // silently empty screen is the kind of defect that reaches a review submission.
  if (!screen) {
    throw new Error(`Unknown screen id: ${id}`);
  }
  return screen;
}
