/**
 * CÁC MÀN CÒN LẠI CỦA APP RIÊNG — theo bản mẫu `vi-gov/zalo-miniapp` (chủ dự án, 28/09/2026: "làm đủ
 * các màn như bản mẫu"): tra cứu hồ sơ, truyền thanh, video, bản đồ, thông báo, cá nhân. Không màn định
 * danh, không đăng nhập (bỏ 28/09/2026). Họ tên ở Cá nhân CHỈ ĐỂ HIỂN THỊ (29/09/2026): nó được lấy từ
 * Zalo một lần lúc mở app (`TrangXa`), và không có thì hiện "Chưa xác định" — màn này không gọi Zalo,
 * vì hỏi lại ở đây là hỏi lần hai một câu bà con đã trả lời lúc mở app.
 *
 * MÀN NÀO CHƯA CÓ DỮ LIỆU THẬT thì hiện trạng thái trống bằng lời ("xã chưa cập nhật…"), KHÔNG dữ liệu
 * giả: một bản tin bịa trong app mang tên cơ quan nhà nước là một thông tin sai do xã phát hành.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: quét căn cước (dữ liệu định danh — luật 3, điều kiện dừng), đăng nhập, đăng xuất, lưu
 * cài đặt xuống máy (`localStorage` cấm ở nửa này — `ranh-gioi-hai-nua.test.ts` §3b; cỡ chữ sống trong
 * bộ nhớ của lần mở; công tắc "Nhận thông báo" đọc/ghi lựa chọn ở máy chủ qua phiên — ADR 0091 §4).
 */
import { useState } from "react";

import { readHttpsLink } from "../api/hop-dong-cong-khai";
import { BUILD_LABEL } from "../../lib/build-label";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";
import { type OpenExternal, useLeaveApp } from "./leave-app";
import { CUA_TOI, XA_TN } from "./noi-dung";
import type { NotificationPreferenceState } from "./notification-preference";
import { ONhapDong } from "./o-nhap";

/* ═══════════════════════════════ TRA CỨU HỒ SƠ ═══════════════════════════════ */

/**
 * THE NATIONAL PUBLIC-SERVICE PORTAL'S DOSSIER LOOKUP — where "Tra cứu" goes (owner 11/10/2026: the one-stop-shop
 * data is NOT in ViGov). A platform-wide address, the same for every commune, so a constant and not per-commune
 * configuration. The portal forbids framing (`X-Frame-Options: SAMEORIGIN`), so it opens in Zalo's browser through
 * the shell's declared door (`moRaNgoai("dich-vu-cong", …)`), after the leave-app question.
 */
export const DOSSIER_PORTAL_URL = "https://vpcp.dichvucong.gov.vn/p/home/dvc-tra-cuu-ho-so.html";

/**
 * Both fields filled the way the one-stop lookup needs them (`PROTOTYPE.md` §6.8, `DossierPage.tsx:71`): at least
 * 9 digits of phone (any spaces or dots typed are ignored), and exactly 4 digits of the file number. PURE.
 */
export function dossierLookupReady(phone: string, lastFour: string): boolean {
  return phone.replace(/\D/g, "").length >= 9 && /^\d{4}$/.test(lastFour.trim());
}

/** What the 4-digit box keeps of what was typed: digits only, at most four (`DossierPage.tsx:107`). PURE. */
export function lastFourDigits(typed: string): string {
  return typed.replace(/\D/g, "").slice(0, 4);
}

/**
 * Tra cứu hồ sơ — the prototype's screen (`DossierPage.tsx`, owner 11/10/2026): title, the two labelled fields with
 * their hints and examples, "Tra cứu" once both are complete. The lookup itself happens on the national portal:
 * "Tra cứu" asks before leaving (`leave-app.tsx`) and opens the portal. ⚠ THE TWO FIELDS NEVER LEAVE THE PHONE —
 * not in the URL, not in any request: the portal has no way to receive them, and a phone number in a URL would be
 * personal data in a URL (rule 3 forbidden #4). They stay in this screen's state and die with it.
 *
 * `openPortal` absent (tests, outside Zalo): no button — the sentence still says where the lookup is done.
 */
export function TraCuuHoSoXa({ onQuayLai, openPortal }: { onQuayLai: () => void; openPortal?: OpenExternal }) {
  const [phone, setPhone] = useState("");
  const [lastFour, setLastFour] = useState("");
  const leave = useLeaveApp(openPortal);
  const complete = dossierLookupReady(phone, lastFour);
  return (
    <>
      <DauManCon tieu_de={XA_TN.tra_cuu_tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <div className="xa-the xa-the--dem xa-khoi">
          <ONhapDong
            id="xa-sdt-ho-so"
            nhan={XA_TN.o_so_dien_thoai_ho_so}
            goi_y={XA_TN.goi_y_so_dien_thoai_ho_so}
            placeholder={XA_TN.placeholder_so_dien_thoai_ho_so}
            gia_tri={phone}
            toi_da={20}
            kieu_ban_phim="tel"
            onDoi={setPhone}
          />
          <div className="xa-lookup-code">
            <ONhapDong
              id="xa-bon-so-cuoi"
              nhan={XA_TN.o_bon_so_cuoi}
              goi_y={XA_TN.goi_y_bon_so_cuoi}
              placeholder={XA_TN.placeholder_bon_so_cuoi}
              gia_tri={lastFour}
              toi_da={4}
              kieu_ban_phim="numeric"
              onDoi={(v) => setLastFour(lastFourDigits(v))}
            />
          </div>
          {leave.ask !== undefined && (
            <button type="button" className="xa-nut" disabled={!complete} onClick={() => leave.ask?.(DOSSIER_PORTAL_URL)}>
              <BieuTuong ten="search" co={20} />
              {XA_TN.nut_tra_cuu}
            </button>
          )}
          {leave.ask !== undefined && !complete && <p className="xa-phu">{XA_TN.tra_cuu_can_ma}</p>}
          <p className="xa-phu">{XA_TN.tra_cuu_cong_dich_vu}</p>
        </div>
      </TrangCon>
      {leave.dialog}
    </>
  );
}

/* ═══════════════════════════════ MÀN CHƯA CÓ DỮ LIỆU ═══════════════════════════════ */

export function ManChuaCoDuLieu(props: {
  tieu_de: string;
  bieu_tuong: TenBieuTuong;
  cau: string;
  /** One muted sentence under `cau` (`KhoiTrangThai` `hint`), or none. */
  hint?: string;
  onQuayLai: () => void;
}) {
  return (
    <>
      <DauManCon tieu_de={props.tieu_de} onQuayLai={props.onQuayLai} />
      <TrangCon>
        <KhoiTrangThai bieu_tuong={props.bieu_tuong} cau={props.cau} hint={props.hint} />
      </TrangCon>
    </>
  );
}

/* ═══════════════════════════════ CÁ NHÂN ═══════════════════════════════ */

/**
 * Up to two letters for the avatar (`PROTOTYPE.md` §6.6, "Nguyễn Văn Hùng" → "NH"): the first letter of the
 * family name and of the given name — one letter for a one-word name. Upper-cased in Vietnamese. PURE.
 */
export function initials(fullName: string): string {
  const words = fullName.trim().split(/\s+/).filter((w) => w !== "");
  if (words.length === 0) return "";
  const first = words[0]!.charAt(0);
  const last = words.length > 1 ? words[words.length - 1]!.charAt(0) : "";
  return `${first}${last}`.toLocaleUpperCase("vi");
}

/** One row of Cá nhân: the 40px round icon on blue-50, a semibold label, a muted hint under it. */
function SettingText(props: { icon: TenBieuTuong; label: string; hint?: string }) {
  return (
    <>
      <span className="xa-setting-icon" aria-hidden="true">
        <BieuTuong ten={props.icon} co={22} />
      </span>
      <span className="xa-hang__chu">
        <strong className="xa-setting-label">{props.label}</strong>
        {props.hint !== undefined && <span className="xa-phu">{props.hint}</span>}
      </span>
    </>
  );
}

/**
 * "NHẬN THÔNG BÁO" (ADR 0091 §4; the prototype's row, `ProfilePage.tsx:87-93`). Its look follows what is KNOWN:
 *
 *   ready         a real switch (`role="switch"`, `aria-checked`, the knob's position — never colour alone); while a
 *                 save runs it is disabled and says so; OFF adds the line naming the two messages that still go
 *   unknown       no session read it yet: a plain button that says a tap shows the state — no knob, no guess
 *   loading       the same row, disabled, with the words
 *   read-failed   the sentence (`role="alert"`); a tap reads again
 *
 * A failed save keeps the server's value and puts its sentence under the row (`role="alert"`). PURE.
 */
export function NotificationSwitchRow(props: { state: NotificationPreferenceState; onPress: () => void }) {
  const { state } = props;
  const text = <SettingText icon="bell" label={XA_TN.notify_label} hint={XA_TN.notify_hint} />;
  if (state.kind !== "ready") {
    return (
      <>
        <button type="button" className="xa-hang xa-setting-switch" disabled={state.kind === "loading"} onClick={props.onPress}>
          {text}
          <BieuTuong ten="chevron-right" co={22} />
        </button>
        {state.kind === "unknown" && <p className="xa-phu xa-setting-note">{XA_TN.notify_unknown}</p>}
        {state.kind === "loading" && (
          <p className="xa-phu xa-setting-note" role="status">
            {XA_TN.notify_loading}
          </p>
        )}
        {state.kind === "read-failed" && (
          <p className="xa-error-box xa-setting-note" role="alert">
            {state.message}
          </p>
        )}
      </>
    );
  }
  return (
    <>
      <button
        type="button"
        role="switch"
        aria-checked={state.enabled}
        aria-busy={state.saving || undefined}
        className="xa-hang xa-setting-switch"
        disabled={state.saving}
        onClick={props.onPress}
      >
        {text}
        <span className={`xa-cong-tac${state.enabled ? " xa-cong-tac--bat" : ""}`} aria-hidden="true">
          <span className="xa-cong-tac__nut" />
        </span>
      </button>
      {state.saving && (
        <p className="xa-phu xa-setting-note" role="status">
          {XA_TN.notify_saving}
        </p>
      )}
      {!state.enabled && <p className="xa-phu xa-setting-note">{XA_TN.notify_off_note}</p>}
      {state.error !== null && (
        <p className="xa-error-box xa-setting-note" role="alert">
          {state.error}
        </p>
      )}
    </>
  );
}

/**
 * The avatar: the Zalo profile picture when the name request brought one (`NameAtEntry.avatarUrl`, https only) and
 * it loaded; otherwise the initials, otherwise the person icon. Decoration (`alt=""`): the name stands beside it.
 * The picture comes from Zalo's own servers, and its address is shown here and nowhere else.
 */
export function ProfileAvatar(props: { name: string | null; avatarUrl: string | null }) {
  const [failed, setFailed] = useState(false);
  const url = props.avatarUrl !== null && readHttpsLink(props.avatarUrl) !== null ? props.avatarUrl : null;
  if (url !== null && !failed) {
    return <img className="xa-avatar xa-avatar--image" src={url} alt="" decoding="async" onError={() => setFailed(true)} />;
  }
  const letters = props.name !== null ? initials(props.name) : "";
  return (
    <span className="xa-avatar" aria-hidden="true">
      {letters !== "" ? letters : <BieuTuong ten="user" co={30} />}
    </span>
  );
}

export function CaNhanXa(props: {
  /** Tên Zalo lấy lúc mở app, hoặc `null` → "Chưa xác định". */
  ho_ten: string | null;
  /** The Zalo profile picture brought by the same name request, or `null` → the initials. */
  avatarUrl?: string | null;
  ten_xa: string;
  tinh: string;
  /**
   * "Cỡ chữ lớn" — the prototype's ONE switch (owner 11/10/2026): on = every text of the app 20% larger, never
   * below the 16px floor either way (`styles.css` `.xa-co-chu--lon`). In memory only: no `localStorage`.
   */
  largeText: boolean;
  onLargeText: (on: boolean) => void;
  onMoPhanAnh: () => void;
  onMoTraCuu: () => void;
  /** "Thông báo" — the screen the home bell used to open (decision 10: the bell is gone from the header). */
  onOpenNotifications: () => void;
  /**
   * The "Nhận thông báo" switch (ADR 0091 §4), held by `AppCuaXa` so it survives tab changes. Absent = no row
   * (tests that pin the rest of the screen).
   */
  notification?: { state: NotificationPreferenceState; onPress: () => void };
}) {
  return (
    <div className="xa-trang xa-trang--tab">
      {/* The profile card (§6.6): what this app KNOWS — the Zalo name (and picture) taken at entry and the commune of
          the app. No hamlet and no phone line: the app holds neither, and a guessed line is a wrong one. */}
      <div className="xa-the xa-the--dem xa-profile">
        <ProfileAvatar name={props.ho_ten} avatarUrl={props.avatarUrl ?? null} />
        <span className="xa-hang__chu">
          <strong className="xa-profile__name">{props.ho_ten ?? XA_TN.chua_co_ten}</strong>
          <span className="xa-phu">
            {props.ten_xa}
            {props.tinh !== "" ? ` · ${props.tinh}` : ""}
          </span>
        </span>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.hien_thi}</h2>
      <div className="xa-the">
        {/* The whole row is the switch (≥ 48px), its state said by `aria-checked` and by the knob's position —
            never by colour alone. */}
        <button
          type="button"
          role="switch"
          aria-checked={props.largeText}
          className="xa-hang xa-setting-switch"
          onClick={() => props.onLargeText(!props.largeText)}
        >
          <SettingText icon="text" label={XA_TN.co_chu_lon} hint={XA_TN.text_size_hint} />
          <span className={`xa-cong-tac${props.largeText ? " xa-cong-tac--bat" : ""}`} aria-hidden="true">
            <span className="xa-cong-tac__nut" />
          </span>
        </button>
        <div className="xa-ke xa-ke--sat" />
        {props.notification !== undefined && (
          <>
            <NotificationSwitchRow state={props.notification.state} onPress={props.notification.onPress} />
            <div className="xa-ke xa-ke--sat" />
          </>
        )}
        {/* A statement, not a control: Vietnamese is the only language the app has. */}
        <div className="xa-hang xa-hang--tinh">
          <SettingText icon="globe" label={XA_TN.language} hint={XA_TN.language_value} />
        </div>
      </div>

      <h2 className="xa-dau-khoi xa-dau-khoi__tieu-de">{XA_TN.of_mine}</h2>
      <div className="xa-the">
        {/* The prototype's two rows (`ProfilePage.tsx:109-118`): labels only — no petition count under "Phản ánh của
            tôi" (owner 11/10/2026). */}
        <button type="button" className="xa-hang" onClick={props.onMoPhanAnh}>
          <SettingText icon="message-square-warning" label={CUA_TOI.tieu_de} />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onMoTraCuu}>
          <SettingText icon="file-search" label={XA_TN.lich_su_tra_cuu} />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
        <div className="xa-ke xa-ke--sat" />
        <button type="button" className="xa-hang" onClick={props.onOpenNotifications}>
          <SettingText icon="bell" label={XA_TN.thong_bao} />
          <BieuTuong ten="chevron-right" co={22} />
        </button>
      </div>

      {BUILD_LABEL !== "" && (
        <p className="xa-app-version">
          <BieuTuong ten="info" co={18} />
          <span>{XA_TN.app_version(BUILD_LABEL)}</span>
        </p>
      )}
    </div>
  );
}
