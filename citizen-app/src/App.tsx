import { type ReactNode, useState } from "react";

import { LaunchParamsPanel } from "bien-the/chan-doan";
import {
  ChonXaScreen,
  CO_LOP_KHAM_PHA,
  GoiYXaScreen,
  NHAN_KHAM_PHA,
  phanGiaiGoiY,
  TrangXaScreen,
  type XaDemo,
} from "bien-the/kham-pha";

import { TabBar } from "./components/TabBar";
import { COMPANY } from "./content/company-profile";
import { DEFAULT_SCREEN_ID, findScreen, type ScreenId } from "./features/company-intro/screens";
import { batChanDoan, type KetQuaDo, thamSo as thamSoLaunch, thamSoMoApp } from "./lib/launch-params";

/**
 * Phase 1 shell: four static screens, no navigation library, no state beyond the current tab.
 *
 * WHY THE HEADER ALWAYS NAMES THE OWNER:
 *
 *   In phase 2 this header is where the COMMUNE name lives, on every screen, because a citizen
 *   who cannot see which commune they are addressing can submit to the wrong one (README
 *   §Non-negotiables #2). The slot was reserved from the first screen so phase 2 would fill it
 *   rather than invent it — and `KhungApp` below is where it now gets filled: a commune has been
 *   chosen, the slot holds its name; no commune yet, it holds the entity that published the app.
 *
 * WHY NOT A ROUTER:
 *
 *   Four sibling screens with no deep-linkable state do not need history. When phase 2 brings
 *   deep links (`https://zalo.me/s/<APP_ID>/?t=...`), routing becomes a real requirement and
 *   gets decided then, with that requirement in hand.
 */

/**
 * Vỏ ứng dụng, THUẦN — nhận mọi thứ qua tham số, không giữ trạng thái nào.
 *
 * Tách ra vì bất di dịch #2 ("đã chọn xã thì tên xã hiện trên MỌI màn hình") chỉ kiểm được khi
 * dựng được cái vỏ ấy với một xã đã chọn và với TỪNG màn hình một. Gói chung trong một component
 * có `useState` thì phép kiểm phải bấm nút, mà bộ test ở đây dựng bằng `react-dom/server` và
 * không có DOM để bấm. Một bất biến không kiểm được là một bất biến sẽ trôi.
 */
export function KhungApp(props: {
  man: ScreenId;
  onChonMan: (id: ScreenId) => void;
  /** Xã công dân đã xác nhận, hoặc `null`. Xem ghi chú về trạng thái phía client trong `App`. */
  xaDaChon: XaDemo | null;
  onDoiXa: () => void;
  /**
   * Đang ở lớp khám phá: chọn hoặc xác nhận xã. Lúc ấy thanh tab BIẾN MẤT.
   *
   * `skills/accessibility-elderly`, yêu cầu #4: mỗi màn một việc. Để thanh tab lại thì công dân
   * bấm nó, không có gì xảy ra (vì chưa có xã), và một nút bấm không phản hồi là cách nhanh
   * nhất để người lớn tuổi kết luận rằng app hỏng và bỏ đi.
   */
  khamPha?: boolean;
  children: ReactNode;
}) {
  const man = findScreen(props.man);

  return (
    <div className="app">
      <header className="app-header">
        <div className="app-header__hang">
          <div className="app-header__ten">
            {/* MỘT Ô, HAI CHỦ SỞ HỮU. Chưa chọn xã thì đây là đơn vị phát hành ứng dụng — thứ
                Zalo đã duyệt. Chọn xã rồi thì đây là xã, trên mọi màn hình, không có ngoại lệ. */}
            <p className="app-header__owner">{props.xaDaChon ? props.xaDaChon.ten : COMPANY.name}</p>
            {/* HAI NHÃN CỦA LỚP KHÁM PHÁ ĐỌC TỪ `bien-the/kham-pha`, KHÔNG VIẾT THẲNG Ở ĐÂY.
                `App.tsx` không nằm sau alias, nên một chuỗi viết thẳng trong tệp này đi vào cả
                bản `goc` — bản nộp — kể cả khi nhánh vẽ nó không bao giờ chạy. Xem chú thích
                của `NHAN_KHAM_PHA` trong `features/kham-pha/index.ts`. */}
            <p className="app-header__screen">
              {props.khamPha ? NHAN_KHAM_PHA.tieu_de_chon_xa : man.headerTitle}
            </p>
          </div>

          {/* ĐỔI XÃ LÀ HÀNH ĐỘNG TƯỜNG MINH, KHÔNG BAO GIỜ TỰ ĐỘNG (ADR 0005). Nút chỉ có mặt
              khi đã có xã để đổi — trước đó nó sẽ là một nút không nói lên điều gì. */}
          {props.xaDaChon && (
            <button type="button" className="app-header__doi-xa" onClick={props.onDoiXa}>
              {NHAN_KHAM_PHA.nut_doi_xa}
            </button>
          )}
        </div>
      </header>

      <main className="app-main" id="main">
        {props.children}
      </main>

      {!props.khamPha && <TabBar current={props.man} onSelect={props.onChonMan} />}
    </div>
  );
}

export function App() {
  const [currentId, setCurrentId] = useState<ScreenId>(DEFAULT_SCREEN_ID);
  const screen = findScreen(currentId);
  const Screen = screen.component;

  // Đọc MỘT LẦN, NGAY LÚC DỰNG. `location.search` có sẵn đồng bộ, nên không còn `useEffect` và
  // không còn nhịp nhấp nháy: công dân quét QR thấy thẳng màn xác nhận xã, không thấy màn giới
  // thiệu công ty loé lên trước. Đó là món quà kèm theo của việc bỏ `zmp-sdk` — xem
  // lib/launch-params.ts. Hàm bọc try/catch, nên ngoài trình duyệt nó trả về "không có tham số".
  const [thamSo] = useState<KetQuaDo>(thamSoMoApp);

  /**
   * ⚠ XÃ ĐÃ CHỌN LÀ **TRẠNG THÁI GIAO DIỆN PHÍA CLIENT**, KHÔNG PHẢI MỘT PHIÊN.
   *
   * Nó sống trong `useState`: mất khi app đóng, không được lưu xuống máy, không được gửi đi đâu,
   * và **không cấp quyền gì cả**. ADR 0005: xã của phiên do MÁY CHỦ ghi sau khi công dân xác
   * nhận, và kênh công dân phía máy chủ chưa tồn tại (`ListTenants` còn chưa có cài đặt).
   *
   * Khi tuyến ấy sống, dòng này được thay bằng xã đọc ra từ phiên do máy chủ trả về — không phải
   * được "đồng bộ thêm" với nó. Hai nguồn cho một sự thật thì một trong hai sẽ cũ, và cái cũ là
   * cái đi vào hồ sơ gửi nhầm cơ quan.
   */
  const [xaDaChon, setXaDaChon] = useState<XaDemo | null>(null);
  const [dangChonXa, setDangChonXa] = useState(false);
  const [boQuaKhamPha, setBoQuaKhamPha] = useState(false);

  const p = thamSoLaunch(thamSo);

  /**
   * LỚP KHÁM PHÁ (ADR 0005). Tham số `t` chỉ DẪN GIAO DIỆN — nó không chọn xã, không mở một app
   * khác, và không được phép làm hai việc đó. Đây là lý do màn khám phá thay thế nội dung công ty
   * chứ không "redirect": không có nơi nào để redirect tới cho tới khi tuyến công dân phía máy
   * chủ tồn tại, và một Mini App chỉ có MỘT App ID cho mọi xã.
   */
  const goiY = phanGiaiGoiY(p["t"] ?? "", p["src"] ?? "");

  /**
   * `CO_LOP_KHAM_PHA` đứng ĐẦU điều kiện, và nó không phải một cờ tính năng lúc chạy: ở biến thể
   * `goc` nó là hằng `false` và các màn khám phá đã bị thay bằng bản rỗng (`vite.config.ts`).
   * Thiếu nó thì một đường liên kết có `t` mở ra một màn hình trắng — thứ người duyệt của Zalo
   * thấy trước tiên.
   */
  const dangKhamPha = CO_LOP_KHAM_PHA && !xaDaChon && !boQuaKhamPha && Boolean(p["t"]);

  let noiDung: ReactNode = <Screen />;
  if (dangChonXa) {
    // Công dân tự bấm "Đổi xã": không cần giải thích gì, chính họ vừa yêu cầu.
    noiDung = (
      <ChonXaScreen
        li_do={null}
        onChon={(xa) => {
          setXaDaChon(xa);
          setDangChonXa(false);
        }}
        onXemGioiThieu={() => {
          setDangChonXa(false);
          setBoQuaKhamPha(true);
        }}
      />
    );
  } else if (dangKhamPha) {
    noiDung =
      goiY.kieu === "chon-san" ? (
        <GoiYXaScreen
          xa={goiY.xa}
          nguon={goiY.nguon}
          onXacNhan={() => setXaDaChon(goiY.xa)}
          onChonXaKhac={() => setDangChonXa(true)}
        />
      ) : (
        <ChonXaScreen
          li_do={goiY.li_do}
          onChon={(xa) => setXaDaChon(xa)}
          onXemGioiThieu={() => setBoQuaKhamPha(true)}
        />
      );
  } else if (xaDaChon && currentId === DEFAULT_SCREEN_ID) {
    /**
     * ĐÃ CHỌN XÃ THÌ TAB ĐẦU LÀ TRANG CỦA XÃ ẤY — không phải một tab thứ năm, và không phải một
     * màn hình che mất thanh tab.
     *
     *   Thêm tab thì sổ màn hình có hai loại màn khác hẳn nhau trong một danh sách, và tab ấy
     *   dẫn đi đâu khi chưa chọn xã là một câu không có câu trả lời đúng. Che thanh tab thì phần
     *   giới thiệu — thứ Zalo đã duyệt — không còn đường tới.
     *
     *   Cách này giữ đúng một đường: bấm tab đầu là về trang xã, luôn luôn, kể cả sau khi công
     *   dân đi xem phần giới thiệu. Không có ngõ cụt nào mở ra.
     */
    noiDung = <TrangXaScreen xa={xaDaChon} />;
  }

  return (
    <KhungApp
      man={currentId}
      onChonMan={setCurrentId}
      xaDaChon={xaDaChon}
      onDoiXa={() => setDangChonXa(true)}
      khamPha={dangChonXa || dangKhamPha}
    >
      {/* Bảng chẩn đoán: chỉ mở bằng `debug`. Xem lib/launch-params.ts. */}
      {batChanDoan(thamSo) && <LaunchParamsPanel thamSo={thamSo} />}
      {noiDung}
    </KhungApp>
  );
}
