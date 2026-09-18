import { useEffect, useState } from "react";

import { TabBar } from "./components/TabBar";
import { COMPANY } from "./content/company-profile";
import { DEFAULT_SCREEN_ID, findScreen, type ScreenId } from "./features/company-intro/screens";
import { LaunchParamsPanel } from "./features/diagnostics/LaunchParamsPanel";
import { GoiYXaScreen } from "./features/kham-pha/GoiYXaScreen";
import { batChanDoan, type KetQuaDo, thamSo as thamSoLaunch, thamSoMoApp } from "./lib/launch-params";

/**
 * Phase 1 shell: four static screens, no navigation library, no state beyond the current tab.
 *
 * WHY THE HEADER ALWAYS NAMES THE OWNER:
 *
 *   In phase 2 this header is where the COMMUNE name lives, on every screen, because a citizen
 *   who cannot see which commune they are addressing can submit to the wrong one (README
 *   §Non-negotiables #2). Phase 1 has no commune, so the slot holds the entity that published
 *   the app. The slot exists from the first screen so phase 2 fills it rather than inventing it.
 *
 * WHY NOT A ROUTER:
 *
 *   Four sibling screens with no deep-linkable state do not need history. When phase 2 brings
 *   deep links (`https://zalo.me/s/<APP_ID>/?t=...`), routing becomes a real requirement and
 *   gets decided then, with that requirement in hand.
 */
export function App() {
  const [currentId, setCurrentId] = useState<ScreenId>(DEFAULT_SCREEN_ID);
  const screen = findScreen(currentId);
  const Screen = screen.component;

  // Đọc MỘT LẦN sau khi dựng. Bất đồng bộ vì `zmp-sdk` chỉ nhập được trong trình duyệt (xem
  // lib/launch-params.ts), nên bảng chẩn đoán xuất hiện ở lượt vẽ thứ hai — không sao, nó là
  // công cụ đo chứ không phải nội dung người dân đọc.
  const [thamSo, setThamSo] = useState<KetQuaDo | null>(null);
  const [boQuaGoiY, setBoQuaGoiY] = useState(false);

  // `t` có mặt ⇒ liên kết mang một GỢI Ý về xã. Không phải một lựa chọn — xem GoiYXaScreen.
  const p = thamSo ? thamSoLaunch(thamSo) : {};
  const goiYXa = !boQuaGoiY && p["t"] ? { maXa: p["t"], nguon: p["src"] ?? "" } : null;
  useEffect(() => {
    let conSong = true;
    void thamSoMoApp().then((t) => {
      if (conSong) setThamSo(t);
    });
    return () => {
      conSong = false;
    };
  }, []);

  return (
    <div className="app">
      <header className="app-header">
        <p className="app-header__owner">{COMPANY.name}</p>
        <p className="app-header__screen">{screen.headerTitle}</p>
      </header>

      <main className="app-main" id="main">
        {/* Bản đo: luôn hiện. Xem lý do và cách đóng cổng lại ở lib/launch-params.ts. */}
        {thamSo && batChanDoan(thamSo) && <LaunchParamsPanel thamSo={thamSo} />}

        {/* LỚP KHÁM PHÁ (ADR 0005). Tham số `t` chỉ DẪN GIAO DIỆN — nó không chọn xã, không
            mở một app khác, và không được phép làm hai việc đó. Xã chỉ thành thật khi server
            ghi nó vào phiên sau một xác nhận tường minh. Đây là lý do màn gợi ý thay thế nội
            dung công ty chứ không "redirect": không có nơi nào để redirect tới cho tới khi
            tuyến công dân phía máy chủ tồn tại. */}
        {goiYXa ? (
          <GoiYXaScreen
            maXa={goiYXa.maXa}
            nguon={goiYXa.nguon}
            onBoQua={() => setBoQuaGoiY(true)}
          />
        ) : (
          <Screen />
        )}
      </main>

      <TabBar current={currentId} onSelect={setCurrentId} />
    </div>
  );
}
