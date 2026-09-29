/**
 * Vỏ của kênh công dân: một màn chọn việc (gửi · phản ánh của tôi · tra cứu), rồi đúng màn ấy.
 *
 * Có mặt trong bản dựng duy nhất (27/09/2026). Chừng nào chưa có phiên ViGov, ba màn phản ánh nói
 * "kênh chưa mở" — `api/vigov-session.ts` — và màn chọn việc nói điều ấy TRƯỚC khi người dân bấm
 * (`COMMUNE_NOT_LOGGED_IN`).
 *
 * HAI MÀN CÔNG KHAI (27/09/2026): "Tin tức của xã" và "Danh bạ cán bộ xã". Chúng cần TÊN MIỀN xã cho
 * `?host=`, và `App.tsx` là bên chọn nó (`publicLookupKey`): tên miền chính của xã CỦA PHIÊN
 * (`communePrimaryHost`), nếu không có thì `d` công dân đã xác nhận ở lần mở này. Không có cả hai —
 * mở app không qua QR và phiên không mang tên miền — thì hai lối vào BIẾN MẤT thay vì đoán một xã
 * (bundle không được mang giá trị theo xã, ADR 0047 điều kiện dừng #2).
 */
import { useState } from "react";

import type { ReopenWithPhone } from "../api/open-vigov-session";
import { getVigovSession } from "../api/vigov-session";

import { StaffDirectoryScreen } from "./StaffDirectoryScreen";
import { SubmitReportScreen } from "./SubmitReportScreen";
import { COMMUNE_NOT_LOGGED_IN, MY_REPORTS, DIRECTORY, SEND, BACK, COMMUNE_NEWS, LOOKUP } from "./copy";
import { MyReportsScreen } from "./MyReportsScreen";
import type { GetSceneLocation } from "./scene-location";
import { CommuneNewsScreen } from "./CommuneNewsScreen";
import { ReportLookupScreen } from "./ReportLookupScreen";

export const CITIZEN_CHANNEL_LABEL = "Phản ánh với xã";

/** Nút mở kênh, đặt trên tab đầu khi đã xác nhận xã. Nhãn nằm ở đây — không viết thẳng trong `App.tsx`. */
export function CitizenChannelButton({ onPress }: { onPress: () => void }) {
  return (
    <button type="button" className="cd-nut" onClick={onPress}>
      {CITIZEN_CHANNEL_LABEL}
    </button>
  );
}

type Screen =
  | { kind: "chon" }
  | { kind: "gui" }
  | { kind: "cua-toi" }
  /** `from_list`: mở từ "Phản ánh của tôi" — "Quay lại" về đúng danh sách ấy. */
  | { kind: "tra-cuu"; code: string; from_list: boolean }
  | { kind: "tin-tuc" }
  | { kind: "danh-ba" };

export function CitizenChannel({
  onClose,
  domain = null,
  reopenWithPhone,
  getSceneLocation,
}: {
  /** Không truyền = không có nút "Quay lại" (app riêng của xã: kênh là màn gốc, không có chỗ để về). */
  onClose?: () => void;
  /** Tên miền xã công dân đã xác nhận ở lần mở này, hoặc `null`. Chỉ làm khoá tra `?host=`. */
  domain?: string | null;
  /**
   * Mở lại phiên kèm số điện thoại khi một tuyến phản ánh trả 403 `chua_xac_thuc_so` — lớp vỏ dựng nó
   * (`App.tsx`), ba màn phản ánh chỉ gọi nó SAU cú bấm đồng ý của công dân (`phone-verification.tsx`).
   */
  reopenWithPhone?: ReopenWithPhone;
  /** "Lấy vị trí hiện tại" on the send screen — the shell builds it (`App.tsx`); absent = no button. */
  getSceneLocation?: GetSceneLocation;
}) {
  const [screen, setScreen] = useState<Screen>({ kind: "chon" });
  // Đọc MỘT LẦN lúc dựng, cùng cách ba màn phản ánh đọc (`useState(getVigovSession)`): phiên chỉ được
  // ghi trước khi kênh mở, ở bước xác nhận xã.
  const [has_session] = useState(() => getVigovSession() !== null);
  /**
   * Bumped when the citizen changes a petition from its detail (a rating). The kept list below is keyed by
   * it, so it is re-read from the server instead of showing the status from before — a 1–2 star rating
   * reopens the petition, and "Quay lại" must not show it as still resolved. The cost: the pages loaded
   * with "Xem thêm" are loaded again from the first.
   */
  const [listVersion, setListVersion] = useState(0);
  const backToChoice = () => setScreen({ kind: "chon" });

  if (screen.kind === "gui") {
    return <SubmitReportScreen onBack={backToChoice} reopenWithPhone={reopenWithPhone} getSceneLocation={getSceneLocation} />;
  }
  if (screen.kind === "tin-tuc" && domain !== null) {
    return <CommuneNewsScreen domain={domain} onBack={backToChoice} />;
  }
  if (screen.kind === "danh-ba" && domain !== null) {
    return <StaffDirectoryScreen domain={domain} onBack={backToChoice} />;
  }

  if (screen.kind === "cua-toi" || (screen.kind === "tra-cuu" && screen.from_list)) {
    // DANH SÁCH GIỮ NGUYÊN khi mở một phiếu: nó chỉ bị ẩn (`hidden`), không bị gỡ, nên "Quay lại"
    // trả người dân về đúng những trang họ đã bấm "Xem thêm" — không tải lại từ đầu. Vẫn chỉ trong
    // bộ nhớ; đóng kênh là mất.
    const report_open = screen.kind === "tra-cuu";
    return (
      <>
        <div hidden={report_open}>
          <MyReportsScreen
            key={listVersion}
            onBack={backToChoice}
            onOpenReport={(code) => setScreen({ kind: "tra-cuu", code, from_list: true })}
            onSubmitReport={() => setScreen({ kind: "gui" })}
            reopenWithPhone={reopenWithPhone}
          />
        </div>
        {screen.kind === "tra-cuu" && (
          <ReportLookupScreen
            key={screen.code}
            initial_code={screen.code}
            onBack={() => setScreen({ kind: "cua-toi" })}
            reopenWithPhone={reopenWithPhone}
            onChanged={() => setListVersion((v) => v + 1)}
          />
        )}
      </>
    );
  }

  if (screen.kind === "tra-cuu") return <ReportLookupScreen onBack={backToChoice} reopenWithPhone={reopenWithPhone} />;

  return (
    <section className="cd-man" aria-label={CITIZEN_CHANNEL_LABEL}>
      {onClose !== undefined && (
        <button type="button" className="quay-lai" onClick={onClose}>
          {BACK}
        </button>
      )}
      <h1 className="cd-tieu-de">{CITIZEN_CHANNEL_LABEL}</h1>
      {!has_session && (
        <p className="cd-loi" role="status">
          {COMMUNE_NOT_LOGGED_IN.text}
          {domain !== null && ` ${COMMUNE_NOT_LOGGED_IN.remaining}`}
        </p>
      )}
      <button type="button" className="cd-nut" onClick={() => setScreen({ kind: "gui" })}>
        {SEND.title}
      </button>
      <button type="button" className="cd-nut" onClick={() => setScreen({ kind: "cua-toi" })}>
        {MY_REPORTS.title}
      </button>
      <button
        type="button"
        className="cd-nut"
        onClick={() => setScreen({ kind: "tra-cuu", code: "", from_list: false })}
      >
        {LOOKUP.title}
      </button>
      {domain !== null && (
        <>
          <button type="button" className="cd-nut" onClick={() => setScreen({ kind: "tin-tuc" })}>
            {COMMUNE_NEWS.title}
          </button>
          <button type="button" className="cd-nut" onClick={() => setScreen({ kind: "danh-ba" })}>
            {DIRECTORY.title}
          </button>
        </>
      )}
    </section>
  );
}
