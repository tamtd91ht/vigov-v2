/**
 * APP RIÊNG CỦA MỘT XÃ — toàn bộ giao diện sau khi mở (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Mong muốn của chủ dự án (28/09/2026): chọn app của xã trên Zalo là thấy NGAY giao diện của xã ấy; xã
 * sau y hệt, không sửa mã. GIAO DIỆN theo bản mẫu `vi-gov/zalo-miniapp`, ĐỦ CÁC MÀN (chủ dự án: "làm đủ
 * các màn như bản mẫu đi"): trang chủ, phản ánh, tin tức, cá nhân, gửi phản ánh, tra cứu hồ
 * sơ, danh bạ, truyền thanh, video, bản đồ, thông báo.
 *
 * DỮ LIỆU:
 *   · công khai — tên xã (`/communes`), tin tức (`/commune-news`), danh bạ (`/commune-staff`), theo tên
 *     miền, không phiên, không qua `vihat-miniapp`;
 *   · họ tên — lấy từ Zalo MỘT LẦN, lúc mở app, ở đây (`getUserInfo`, tiêm từ lớp vỏ; quyết định của người
 *     dùng 29/09/2026). Các màn bên dưới chỉ HIỂN THỊ tên ấy hoặc "Chưa xác định" — không màn nào gọi Zalo
 *     lại. Tên chỉ điền sẵn, không cấp gì (luật 4);
 *   · phản ánh — sổ phản ánh THẬT của xã (29/09/2026), qua phiên công dân ViGov mở ở VIỆC CÁ NHÂN ĐẦU TIÊN
 *     (gửi, xem phản ánh của tôi, tra cứu phiếu, chấm sao): lời giải thích trước, rồi mới tới hộp thoại xin
 *     số của Zalo (`commune-session.ts`, chính sách 3.3.4). Danh tính người gửi lấy từ phiên ở máy chủ, xã
 *     lấy từ App ID của app — và phiên chỉ được giữ khi tên xã của nó TRÙNG tên xã trên đầu màn hình;
 *   · chưa có — truyền thanh, video, bản đồ, thông báo, tra cứu hồ sơ: trạng thái trống bằng lời.
 *
 * NHÁP PHẢN ÁNH (ADR 0050 #7, chủ dự án 28/09/2026): có, như bản mẫu — nhưng nửa này KHÔNG chạm kho lưu
 * trữ; lớp vỏ tiêm `draftStore` (chỉ `CommuneApp`), tệp duy nhất chạm kho là `commune-app/feedback-draft-store.ts`.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: router, lưu trữ trực tiếp, tên xã từ biến môi trường, lớp gọi máy chủ của nó, OTP,
 * quét căn cước, số ngày cam kết viết cứng (luật 10). Bước chọn lĩnh vực CÓ, và lĩnh vực ấy là của phiếu (ADR 0050, thay ADR 0049).
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP (ADR 0047 §6). Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log.
 */
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { communeProfiles, type PublicResult, lookupCommuneByDomain } from "../api/vigov-client";
import type { CommuneProfile, FoundCommune } from "../api/public-contract";
import { communeAppReopen, dropCommuneAppSession, type OpenCommuneAppSession } from "../api/open-vigov-session";

import { Icon, type IconName } from "./Icon";
import {
  createSessionGate,
  type SessionGate,
  sessionGateMessage,
  sessionGateOffersRetry,
  type SessionGateState,
} from "./commune-session";
import { dialTarget } from "./StaffDirectoryScreen";
import { CommuneDirectoryBody, useCommuneDirectory } from "./CommuneDirectory";
import { SubScreenHeader, StatusBlock, SubPage } from "./commune-frame";
import {
  COMMUNE_APP,
  COMMUNE_APP_SESSION,
  COMMUNE_OFFICE,
  MY_REPORTS,
  DIRECTORY,
  PHONE_VERIFICATION,
  type PhoneVerificationTask,
  COMMUNE_NEWS,
  COMMUNE_APP_UI,
  COMMUNE_APP_REPORTS,
  COMMUNE_APP_SCREENS,
} from "./copy";
import {
  CommuneSendScreen,
  ListStatus,
  type MyReportsState,
  type OnSessionLost,
  CommuneReportCard,
  CommuneReportDetail,
  CommuneReportList,
  CommuneReportLookup,
  useMyReports,
} from "./CommuneAppReports";
import { CommunePersonal, type FontSize, NoDataScreen, CommuneRecordLookup } from "./CommuneAppUtilities";
import { CommuneNewsArticle, CommuneNewsList, NewsRow, NewsOfType, useCommuneNews } from "./CommuneAppNews";
import type { GetSceneLocation } from "./scene-location";
import {
  afterNameAsk,
  afterNameCheck,
  type FeedbackDraftStore,
  type GetZaloName,
  type NameAtEntry,
  nameShown,
} from "./commune-app-model";

/**
 * The commune as the header shows it. `ten` · `tinh` keep their old names: the shape is shared with the
 * commercial half's commune suggestion (`features/kham-pha`), which this rename does not touch.
 */
export type AppCommune = { readonly ten: string; readonly tinh: string };

/** Kết quả tra xã → xã của app, hoặc câu báo lỗi. THUẦN. Đúng MỘT xã, tên không rỗng. */
export function communeFromLookup(
  result: PublicResult<readonly FoundCommune[]>,
): { readonly commune: AppCommune } | { readonly error: string } {
  if (result.kind === "xong" && result.value.length === 1 && result.value[0]!.ten.trim() !== "") {
    return { commune: result.value[0]! };
  }
  if (result.kind === "xong" || result.kind === "khong-hop-le" || result.kind === "khong-thay") {
    return { error: COMMUNE_APP.not_found };
  }
  return { error: COMMUNE_APP.not_connected };
}

/* ═════════════════════════════════ ĐIỀU HƯỚNG — không router ═════════════════════════════════ */

export type CommuneTab = "trang-chu" | "phan-anh" | "tin-tuc" | "ca-nhan";

type CommuneScreen =
  | { readonly kind: "tab"; readonly tab: CommuneTab }
  | { readonly kind: "gui" }
  | { readonly kind: "phieu"; readonly code: string; readonly from: CommuneTab }
  | { readonly kind: "bai"; readonly id: string; readonly from: CommuneTab }
  | { readonly kind: "danh-ba" }
  | { readonly kind: "tra-cuu" }
  | { readonly kind: "tra-cuu-phieu" }
  | { readonly kind: "truyen-thanh" }
  | { readonly kind: "video" }
  | { readonly kind: "ban-do" }
  | { readonly kind: "thong-bao" }
  | { readonly kind: "su-kien" };

const TABS: ReadonlyArray<{ tab: CommuneTab; label: string; icon: IconName }> = [
  { tab: "trang-chu", label: COMMUNE_APP_UI.tab_home, icon: "home" },
  { tab: "phan-anh", label: COMMUNE_APP_UI.tab_reports, icon: "chat" },
  { tab: "tin-tuc", label: COMMUNE_APP_UI.tab_news, icon: "news" },
  { tab: "ca-nhan", label: COMMUNE_APP_SCREENS.tab_personal, icon: "user" },
];

/**
 * Thanh tab dưới — bốn mục, KHÔNG nút nổi, theo prototype khách (`BottomNav.tsx`). "Gửi phản ánh" có ở
 * nhóm "Chính quyền số" trên trang chủ và ở đầu tab Phản ánh.
 */
function CommuneTabBar({ tab, onSelect }: { tab: CommuneTab; onSelect: (t: CommuneTab) => void }) {
  return (
    <nav className="xa-tab" aria-label={COMMUNE_APP_UI.tab_bar}>
      {TABS.map((t) => (
        <button
          key={t.tab}
          type="button"
          className={`xa-tab__muc${t.tab === tab ? " xa-tab__muc--on" : ""}`}
          aria-current={t.tab === tab ? "page" : undefined}
          onClick={() => onSelect(t.tab)}
        >
          <Icon name={t.icon} size={24} />
          <span>{t.label}</span>
        </button>
      ))}
    </nav>
  );
}

/* ═════════════════════════════════ TRANG CHỦ ═════════════════════════════════ */

/**
 * LOGO XÃ trên header. TẠM THỜI: `deploy.mjs --vao-thang` chép `scripts/logo-xa/<tên-miền>.png` vào bản
 * dựng thành `./logo-xa.png` (chủ dự án, 28/09/2026); nguồn thật sau này là hồ sơ hiển thị của xã ở
 * service-platform, cấu hình qua platform-admin. Không có tệp thì ô trở về biểu tượng tòa nhà.
 * `alt=""`: tên xã đứng ngay cạnh bằng chữ.
 */
function CommuneLogo() {
  const [error, setError] = useState(false);
  if (error) {
    return (
      <span className="xa-hero__dai-dien" aria-hidden="true">
        <Icon name="build" size={24} />
      </span>
    );
  }
  return <img className="xa-hero__logo" src="./logo-xa.png" alt="" onError={() => setError(true)} />;
}

type MenuTile = {
  label: string;
  icon: IconName;
  color: "hong" | "xanh" | "luc" | "cam" | "navy" | "tim";
  screen: CommuneScreen;
};

/**
 * HAI NHÓM CHỨC NĂNG — đúng bố cục prototype khách (`FunctionGrid.tsx`): "Chính quyền số" và "Thông tin –
 * Truyền thông", lưới bốn cột, nền màu phủ cả ô (ô to thì vùng chạm to). Chia nhóm để người dân tìm theo
 * loại việc, và để xã thêm ô sau này không vỡ bố cục.
 */
const FEATURE_GROUPS: ReadonlyArray<{
  title: string;
  stripe: "navy" | "cam";
  more?: { label: string; screen: CommuneScreen };
  tiles: readonly MenuTile[];
}> = [
  {
    title: COMMUNE_APP_SCREENS.group_government,
    stripe: "navy",
    tiles: [
      { label: COMMUNE_APP_UI.tile_send, icon: "megaphone", color: "hong", screen: { kind: "gui" } },
      { label: COMMUNE_APP_SCREENS.tile_lookup_short, icon: "search", color: "xanh", screen: { kind: "tra-cuu" } },
      { label: COMMUNE_APP_UI.tile_directory, icon: "phone", color: "luc", screen: { kind: "danh-ba" } },
      // SRS M6.1.12 (P1). The screen had no way in, so it read as "done" to anyone reading the code.
      { label: COMMUNE_APP_SCREENS.tile_map, icon: "map", color: "cam", screen: { kind: "ban-do" } },
    ],
  },
  {
    title: COMMUNE_APP_SCREENS.group_information,
    stripe: "cam",
    more: { label: COMMUNE_APP_SCREENS.view_more, screen: { kind: "tab", tab: "tin-tuc" } },
    tiles: [
      { label: COMMUNE_APP_SCREENS.tile_news, icon: "news", color: "xanh", screen: { kind: "tab", tab: "tin-tuc" } },
      { label: COMMUNE_APP_SCREENS.tile_broadcast, icon: "radio", color: "luc", screen: { kind: "truyen-thanh" } },
      { label: COMMUNE_APP_SCREENS.tile_video, icon: "play", color: "cam", screen: { kind: "video" } },
      { label: COMMUNE_APP_SCREENS.tile_events, icon: "clock", color: "tim", screen: { kind: "su-kien" } },
    ],
  },
];

/**
 * THE ENTRY CARD — the only place the app asks Zalo for the name, shown on the home screen right after
 * opening, and only when the silent check found no permission yet (`afterNameCheck`).
 *
 * WHY A CARD BEFORE ZALO'S DIALOG, NOT THE DIALOG STRAIGHT AWAY: Zalo's dialog says WHAT is shared, not
 * WHY. Policy 3.3.4 (quoted in `features/tinh-nang/khung.tsx`) refuses apps whose permission flow does not
 * state its purpose, and an elderly citizen who meets a system dialog the instant the app opens cannot
 * tell what they are agreeing to. So the card says why first, and the dialog opens only on "Đồng ý".
 * "Không" is an ordinary answer: the card goes away, the form's name field is simply empty.
 */
function NameCard(props: { asking: boolean; onAgree: () => void; onDecline: () => void }) {
  return (
    <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-ten-tieu-de">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-ten-tieu-de">
        {COMMUNE_APP_SCREENS.name_card_title}
      </h2>
      <p>{COMMUNE_APP_SCREENS.name_card_why}</p>
      <p className="xa-phu">{COMMUNE_APP_SCREENS.name_card_zalo_asks}</p>
      {props.asking && (
        <p className="xa-phu" role="status">
          {COMMUNE_APP_SCREENS.name_card_asking}
        </p>
      )}
      <button type="button" className="xa-nut" onClick={props.onAgree} disabled={props.asking}>
        {COMMUNE_APP_SCREENS.name_card_agree}
      </button>
      <button type="button" className="xa-nut xa-nut--phu" onClick={props.onDecline} disabled={props.asking}>
        {COMMUNE_APP_SCREENS.name_card_decline}
      </button>
    </section>
  );
}

/**
 * The commune's profile from `/commune-profiles` (public, by domain, no session) — or `null` while loading,
 * when the call failed, or when the answer is not exactly one profile. `null` shows NOTHING: the office
 * block is information, the app works without it, and a guessed hotline is a wrong number published by a
 * public authority. Loaded once per open, with the ref guard `useCommuneNews` uses against StrictMode's double run.
 */
function useCommuneProfile(domain: string): CommuneProfile | null {
  const [profile, setProfile] = useState<CommuneProfile | null>(null);
  const loaded = useRef(false);
  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void communeProfiles(domain).then((result) => {
      if (result.kind === "xong" && result.value.length === 1) setProfile(result.value[0]!);
    });
  }, [domain]);
  return profile;
}

/** The rows the commune actually declared, in reading order. PURE, exported for tests. */
export function officeRows(p: CommuneProfile): Array<{ label: string; value: string }> {
  return [
    { label: COMMUNE_OFFICE.address, value: p.office_address.trim() },
    { label: COMMUNE_OFFICE.hours, value: p.office_hours_text.trim() },
    { label: COMMUNE_OFFICE.hotline, value: p.hotline.trim() },
  ].filter((r) => r.value !== "");
}

/**
 * Trụ sở · giờ làm việc · đường dây nóng, as the commune declared them — nothing when it declared nothing.
 * The hours are display text, never parsed (ADR 0007). The hotline becomes a `tel:` button (the dialler
 * opens; nothing goes over the network), a full tap target in words, not an icon alone.
 */
export function CommuneOffice({ profile }: { profile: CommuneProfile | null }) {
  if (profile === null) return null;
  const rows = officeRows(profile);
  if (rows.length === 0) return null;
  const hotline = profile.hotline.trim();
  const dial = hotline === "" ? null : dialTarget(hotline);
  return (
    <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-tru-so">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-tru-so">
        {COMMUNE_OFFICE.title}
      </h2>
      {rows.map((r) => (
        <div key={r.label}>
          <p className="xa-nhan-o">{r.label}</p>
          <p className="xa-giu-dong">{r.value}</p>
        </div>
      ))}
      {dial !== null && (
        <a className="xa-nut" href={dial}>
          <Icon name="phone" size={20} />
          {COMMUNE_OFFICE.call_hotline(hotline)}
        </a>
      )}
    </section>
  );
}

function CommuneHomeTab(props: {
  commune: AppCommune;
  full_name: string | null;
  /** The entry card, when the name still needs the citizen's consent; otherwise `null`. */
  name_card: ReactNode;
  news: ReturnType<typeof useCommuneNews>;
  /** "Phản ánh của tôi" as loaded in this open — `idle` until the citizen asks (the gate). */
  reports: MyReportsState;
  onOpenReports: () => void;
  onRetryReports: () => void;
  /** The commune's declared office (`/commune-profiles`), or `null` — then no office block. */
  profile: CommuneProfile | null;
  go: (target: CommuneScreen) => void;
}) {
  const { commune, news, go, full_name, reports } = props;
  // Hai phiếu, hai tin — đúng bố cục trang chủ của prototype (`HomePage.tsx`).
  const latest_news = news.list.entries.slice(0, 2);
  const latest_reports = reports.kind === "ready" ? reports.items.slice(0, 2) : [];

  return (
    <div className="xa-trang">
      {/* Header theo prototype (`AppHeader.tsx`): biểu trưng trái, tên xã giữa, lời chào dưới; góc phải
          để trống cho bộ nút của Zalo. Màu giữ theo bản `vi-gov` chủ dự án đã chọn. */}
      <header className="xa-dau-xa">
        <CommuneLogo />
        <div className="xa-dau-xa__chu">
          <h1 className="xa-dau-xa__ten">{commune.ten}</h1>
          <p className="xa-dau-xa__chao">{full_name !== null ? COMMUNE_APP_SCREENS.greeting_name(full_name) : commune.tinh}</p>
        </div>
        <button type="button" className="xa-hero__chuong" onClick={() => go({ kind: "thong-bao" })} aria-label={COMMUNE_APP_SCREENS.notices}>
          <Icon name="bell" size={22} />
        </button>
      </header>

      <div className="xa-trang__than">
        {props.name_card}
        {FEATURE_GROUPS.map((group) => (
          <section key={group.title} className="xa-the xa-the--dem xa-nhom-cn">
            <div className={`xa-dau-nhom xa-dau-nhom--${group.stripe}`}>
              <h2 className="xa-dau-khoi__tieu-de">{group.title}</h2>
              {group.more && (
                <button type="button" className="xa-dau-khoi__them" onClick={() => go(group.more!.screen)}>
                  {group.more.label}
                </button>
              )}
            </div>
            <div className="xa-luoi-4">
              {group.tiles.map((tile) => (
                <button key={tile.label} type="button" className={`xa-o-4 xa-mau--${tile.color}`} onClick={() => go(tile.screen)}>
                  <Icon name={tile.icon} size={30} />
                  <span className="xa-o-4__nhan">{tile.label}</span>
                </button>
              ))}
            </div>
          </section>
        ))}

        <section className="xa-the xa-the--dem xa-nhom-cn">
          <div className="xa-dau-nhom xa-dau-nhom--luc">
            <h2 className="xa-dau-khoi__tieu-de">{COMMUNE_APP_UI.section_reports}</h2>
            <button type="button" className="xa-dau-khoi__them" onClick={() => go({ kind: "tab", tab: "phan-anh" })}>
              {COMMUNE_APP_UI.view_all}
            </button>
          </div>
          {reports.kind !== "ready" ? (
            // Not loaded: the card that asks — nothing personal is fetched at app open (ADR 0047:251).
            <ListStatus state={reports} onOpen={props.onOpenReports} onRetry={props.onRetryReports} />
          ) : latest_reports.length > 0 ? (
            <ul className="xa-ds">
              {latest_reports.map((p) => (
                <li key={p.lookup_code}>
                  <CommuneReportCard report={p} onOpen={() => go({ kind: "phieu", code: p.lookup_code, from: "trang-chu" })} />
                </li>
              ))}
            </ul>
          ) : (
            <StatusBlock icon="chat" text={COMMUNE_APP_SCREENS.no_reports_yet} />
          )}
        </section>

        <section className="xa-the xa-the--dem xa-nhom-cn">
          <div className="xa-dau-nhom xa-dau-nhom--navy">
            <h2 className="xa-dau-khoi__tieu-de">{COMMUNE_APP_UI.section_latest_news}</h2>
            <button type="button" className="xa-dau-khoi__them" onClick={() => go({ kind: "tab", tab: "tin-tuc" })}>
              {COMMUNE_APP_UI.view_all}
            </button>
          </div>
          {!news.list.has_first_page && news.list.loading ? (
            <StatusBlock icon="news" text={COMMUNE_NEWS.loading} loading />
          ) : !news.list.has_first_page && news.list.error !== null ? (
            <StatusBlock icon="alert" error text={COMMUNE_NEWS.server_error} button={{ label: COMMUNE_NEWS.retry_button, onPress: news.loadMore }} />
          ) : latest_news.length === 0 ? (
            <StatusBlock icon="news" text={COMMUNE_APP_UI.latest_news_empty} />
          ) : (
            <ul className="xa-ds">
              {latest_news.map((t) => (
                <li key={t.id}>
                  <NewsRow news={t} onOpen={(id) => go({ kind: "bai", id, from: "trang-chu" })} />
                </li>
              ))}
            </ul>
          )}
        </section>

        <CommuneOffice profile={props.profile} />
      </div>
    </div>
  );
}

/* ═════════════════════════════════ CÁC TAB KHÁC ═════════════════════════════════ */

function TabHeader({ title }: { title: string }) {
  return (
    <div className="xa-dau-tab">
      <h1 className="xa-dau-con__tieu-de">{title}</h1>
    </div>
  );
}

/**
 * The gate, as a screen of its own — one task per screen (`skills/accessibility-elderly` #4). The screen it
 * covers stays MOUNTED underneath (hidden), so a half-written petition survives the detour.
 *
 *   `hoi`       WHY first, then what Zalo will ask, then the button — Zalo's dialog opens only on "Đồng ý"
 *               (policy 3.3.4; the words are `PHONE_VERIFICATION`'s, the same act as the shared app's)
 *   `dang-mo`   words, not a spinner
 *   `ket-qua`   one sentence saying what to do next; "Đồng ý…" again only where a new tap can help
 */
function SessionGateScreen(props: {
  state: SessionGateState;
  task: PhoneVerificationTask;
  onAllow: () => void;
  onDecline: () => void;
  onClose: () => void;
}) {
  const { state } = props;
  return (
    <>
      <SubScreenHeader title={PHONE_VERIFICATION.title} onBack={props.onClose} />
      <SubPage>
        {state.kind === "dang-mo" && <StatusBlock icon="user" text={COMMUNE_APP_SESSION.working} loading />}
        {state.kind === "hoi" && (
          <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-cong-tieu-de">
            <h2 className="xa-dau-khoi__tieu-de" id="xa-cong-tieu-de">
              {PHONE_VERIFICATION.title}
            </h2>
            <p>{PHONE_VERIFICATION.why}</p>
            <p className="xa-phu">{PHONE_VERIFICATION.zalo_asks}</p>
            <button type="button" className="xa-nut" onClick={props.onAllow}>
              {PHONE_VERIFICATION.allow}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onDecline}>
              {PHONE_VERIFICATION.decline}
            </button>
          </section>
        )}
        {state.kind === "ket-qua" && (
          <>
            <StatusBlock
              icon="alert"
              error
              text={sessionGateMessage(state.outcome, props.task)}
              button={sessionGateOffersRetry(state.outcome) ? { label: PHONE_VERIFICATION.allow, onPress: props.onAllow } : undefined}
            />
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onClose}>
              {COMMUNE_APP_SCREENS.home_button}
            </button>
          </>
        )}
      </SubPage>
    </>
  );
}

/** `createSessionGate` held once per open. The commune name is read at call time, from the header's source. */
function useSessionGate(open: OpenCommuneAppSession | undefined, communeName: string) {
  const [state, setState] = useState<SessionGateState | null>(null);
  const name = useRef(communeName);
  name.current = communeName;
  const gate = useRef<SessionGate | null>(null);
  if (gate.current === null) gate.current = createSessionGate(open, () => name.current, setState);
  return { gate: gate.current, state };
}

function DirectoryScreen({ domain, onBack }: { domain: string; onBack: () => void }) {
  const { page, reload } = useCommuneDirectory(domain);
  return (
    <>
      <SubScreenHeader title={DIRECTORY.title} onBack={onBack} />
      <SubPage>
        <CommuneDirectoryBody page={page} onLoad={reload} />
      </SubPage>
    </>
  );
}

/* ═════════════════════════════════ APP ═════════════════════════════════ */

function CommuneAppShell(props: {
  domain: string;
  commune: AppCommune;
  /** The name request of this open, owned by `CommuneHome` (the entry). Screens below only READ it. */
  name: NameAtEntry;
  onAgreeName: () => void;
  onDeclineName: () => void;
  getSceneLocation?: GetSceneLocation;
  draftStore?: FeedbackDraftStore;
  openSession?: OpenCommuneAppSession;
}) {
  const { domain, commune, openSession } = props;
  // Họ tên lấy MỘT LẦN lúc mở app (`CommuneHome`). Phiên ViGov chỉ trong bộ nhớ (`api/vigov-session.ts`), mở ở
  // việc cá nhân đầu tiên qua cổng dưới đây. Thứ DUY NHẤT sống qua lần đóng app là NHÁP đang soạn, qua
  // `draftStore` lớp vỏ tiêm (ADR 0050 #7).
  const shownName = nameShown(props.name);
  const [font_size, setFontSize] = useState<FontSize>("vua");
  const [screen, setScreen] = useState<CommuneScreen>({ kind: "tab", tab: "trang-chu" });
  const news = useCommuneNews(domain);
  const profile = useCommuneProfile(domain);
  const goToTab = (tab: CommuneTab) => setScreen({ kind: "tab", tab });
  const selector = `xa-app xa-co-chu--${font_size}`;

  /* ── THE GATE: every personal act asks here first (`commune-session.ts`) ── */
  const { gate, state: gateState } = useSessionGate(openSession, commune.ten);
  const [gateTask, setGateTask] = useState<PhoneVerificationTask>("submit");
  // The 403 path of the rating block (`usePhoneVerification`): the same opener, the same commune check.
  const reopenWithPhone = useMemo(() => (openSession ? communeAppReopen(openSession) : undefined), [openSession]);

  /** Run `act` with a session — at once if one exists, otherwise after the explanation and the tap. */
  function requireSession(task: PhoneVerificationTask, act: () => void) {
    setGateTask(task);
    gate.require(() => {
      act();
      // A session now exists: the home block and Cá nhân fill in without a second question.
      reports.loadIfIdle();
    });
  }

  /** 401 / 403 `chua_xac_thuc_so`: forget the session, and run the act again through the gate. */
  const sessionLost =
    (task: PhoneVerificationTask): OnSessionLost =>
    (retry) => {
      dropCommuneAppSession();
      requireSession(task, retry);
    };

  const reports = useMyReports(sessionLost("mine"));
  const openReports = () => requireSession("mine", () => void reports.load());

  /** Navigation from a tap. The two personal screens pass the gate; everything else is public. */
  function go(target: CommuneScreen) {
    if (target.kind === "gui") requireSession("submit", () => setScreen(target));
    else if (target.kind === "tra-cuu-phieu") requireSession("lookup", () => setScreen(target));
    else setScreen(target);
  }

  const goBack = () => goToTab("trang-chu");
  let sub_screen = null;
  switch (screen.kind) {
    case "bai":
      sub_screen = (
        <CommuneNewsArticle
          key={screen.id}
          domain={domain}
          id={screen.id}
          list={news.list.entries}
          onOpen={(id) => setScreen({ kind: "bai", id, from: screen.from })}
          onBack={() => goToTab(screen.from)}
        />
      );
      break;
    case "gui":
      sub_screen = (
        <CommuneSendScreen
          commune_name={commune.ten}
          full_name={shownName}
          getSceneLocation={props.getSceneLocation}
          draftStore={props.draftStore}
          onBack={goBack}
          onSessionLost={sessionLost("submit")}
          onSent={() => void reports.load()}
          onOpenReport={(code) => setScreen({ kind: "phieu", code, from: "phan-anh" })}
        />
      );
      break;
    case "phieu":
      sub_screen = (
        <CommuneReportDetail
          key={screen.code}
          code={screen.code}
          onBack={() => goToTab(screen.from)}
          onSessionLost={sessionLost("mine")}
          onChanged={() => void reports.load()}
          reopenWithPhone={reopenWithPhone}
        />
      );
      break;
    case "danh-ba":
      sub_screen = <DirectoryScreen domain={domain} onBack={goBack} />;
      break;
    case "tra-cuu":
      sub_screen = <CommuneRecordLookup onBack={goBack} />;
      break;
    case "tra-cuu-phieu":
      sub_screen = (
        <CommuneReportLookup
          onBack={() => goToTab("phan-anh")}
          onSessionLost={sessionLost("lookup")}
          onChanged={() => void reports.load()}
          reopenWithPhone={reopenWithPhone}
        />
      );
      break;
    case "truyen-thanh":
      sub_screen = (
        <NoDataScreen title={COMMUNE_APP_SCREENS.broadcast_title} icon="radio" text={COMMUNE_APP_SCREENS.broadcast_empty} onBack={goBack} />
      );
      break;
    case "video":
      sub_screen = <NoDataScreen title={COMMUNE_APP_SCREENS.video_title} icon="play" text={COMMUNE_APP_SCREENS.video_empty} onBack={goBack} />;
      break;
    case "ban-do":
      sub_screen = <NoDataScreen title={COMMUNE_APP_SCREENS.map_title} icon="map" text={COMMUNE_APP_SCREENS.map_empty} onBack={goBack} />;
      break;
    case "su-kien":
      // The items the commune published AS EVENTS — the server's `?type=su-kien` (comms b22bf76), no longer
      // a guess from the free-text category.
      sub_screen = (
        <>
          <SubScreenHeader title={COMMUNE_APP_SCREENS.events_title} onBack={goBack} />
          <SubPage>
            <NewsOfType
              domain={domain}
              type="su-kien"
              onOpen={(id) => setScreen({ kind: "bai", id, from: "trang-chu" })}
              empty={COMMUNE_APP_SCREENS.events_empty}
            />
          </SubPage>
        </>
      );
      break;
    case "thong-bao":
      sub_screen = <NoDataScreen title={COMMUNE_APP_SCREENS.notices} icon="bell" text={COMMUNE_APP_SCREENS.notices_empty} onBack={goBack} />;
      break;
    default:
      break;
  }
  // The gate covers the current screen; that screen stays mounted (`hidden`), so what was typed survives.
  const gateScreen =
    gateState === null ? null : (
      <SessionGateScreen
        state={gateState}
        task={gateTask}
        onAllow={() => void gate.allow()}
        onDecline={gate.decline}
        onClose={gate.reset}
      />
    );
  const withGate = (content: ReactNode) => (
    <div className={selector}>
      {gateScreen}
      <div hidden={gateScreen !== null}>{content}</div>
    </div>
  );

  if (sub_screen !== null) return withGate(sub_screen);

  const tab = screen.kind === "tab" ? screen.tab : "trang-chu";
  let body;
  if (tab === "trang-chu") {
    const name_card =
      props.name.kind === "needs-consent" || props.name.kind === "asking" ? (
        <NameCard asking={props.name.kind === "asking"} onAgree={props.onAgreeName} onDecline={props.onDeclineName} />
      ) : null;
    body = (
      <CommuneHomeTab
        commune={commune}
        full_name={shownName}
        name_card={name_card}
        news={news}
        reports={reports.state}
        onOpenReports={openReports}
        onRetryReports={() => void reports.load()}
        profile={profile}
        go={go}
      />
    );
  } else if (tab === "tin-tuc") {
    body = (
      <>
        <TabHeader title={COMMUNE_NEWS.title} />
        <div className="xa-trang xa-trang--tab">
          <CommuneNewsList
            domain={domain}
            list={news.list}
            onOpen={(id) => setScreen({ kind: "bai", id, from: "tin-tuc" })}
            onLoad={news.loadMore}
          />
        </div>
      </>
    );
  } else if (tab === "phan-anh") {
    body = (
      <>
        <TabHeader title={MY_REPORTS.title} />
        <div className="xa-trang xa-trang--tab">
          <button type="button" className="xa-nut xa-nut--hong xa-nut--dau" onClick={() => go({ kind: "gui" })}>
            <Icon name="megaphone" size={22} />
            {COMMUNE_APP_UI.tile_send}
          </button>
          <CommuneReportList
            state={reports.state}
            onOpenReport={(code) => setScreen({ kind: "phieu", code, from: "phan-anh" })}
            onLookup={() => go({ kind: "tra-cuu-phieu" })}
            onOpen={openReports}
            onRetry={() => void reports.load()}
            onLoadMore={() => void reports.loadMore()}
          />
        </div>
      </>
    );
  } else {
    body = (
      <>
        <TabHeader title={COMMUNE_APP_SCREENS.personal_title} />
        <CommunePersonal
          full_name={shownName}
          commune_name={commune.ten}
          province={commune.tinh}
          report_count={
            reports.state.kind === "ready"
              ? { count: reports.state.items.length, more: reports.state.hasMore }
              : null
          }
          font_size={font_size}
          onFontSizeChange={setFontSize}
          onOpenReports={() => goToTab("phan-anh")}
          onOpenLookup={() => setScreen({ kind: "tra-cuu" })}
        />
      </>
    );
  }

  return withGate(
    <>
      <main id="main">{body}</main>
      <CommuneTabBar tab={tab} onSelect={goToTab} />
    </>,
  );
}

type LookupState =
  | { readonly kind: "dang-tra" }
  | { readonly kind: "loi"; readonly text: string }
  | { readonly kind: "xong"; readonly commune: AppCommune };

export function CommuneHome(props: {
  /** Tên miền xã nung vào bản dựng (`lib/fixed-commune.ts`). */
  domain: string;
  /**
   * Lấy họ tên từ Zalo (`getUserInfo`), do lớp vỏ tiêm — nửa này không nhập zmp-sdk (`two-halves-boundary.test.ts`
   * §3a). Gọi CHỈ ở đây, lúc mở app (29/09/2026): một lần "check" không bật hộp của Zalo; nếu chưa được phép
   * thì thẻ giải thích mục đích hiện trên trang chủ, và "ask" chỉ gửi khi bà con bấm "Đồng ý" trên thẻ ấy.
   * Không truyền (chạy thử ngoài Zalo, test) thì không hỏi gì và tên là "Chưa xác định".
   */
  getName?: GetZaloName;
  /**
   * Lấy vị trí hiện tại (`getLocation` + đổi toạ độ ở `vihat-miniapp`), do lớp vỏ tiêm — nửa này không
   * nhập zmp-sdk. Không truyền thì màn gửi phản ánh không có nút vị trí (chạy thử ngoài Zalo, test).
   */
  getSceneLocation?: GetSceneLocation;
  /**
   * Nháp phản ánh đang soạn (ADR 0050 #7), do lớp vỏ tiêm — CHỈ app riêng của xã. Không truyền thì không
   * có nháp (app chung, chạy thử, test): nửa này không tự chạm kho lưu trữ nào.
   */
  draftStore?: FeedbackDraftStore;
  /**
   * Mở phiên công dân ViGov (App ID + `getAccessToken` + `getPhoneNumber` → `vihat-miniapp`), do lớp vỏ
   * tiêm — nửa này không nhập zmp-sdk. Gọi CHỈ ở việc cá nhân đầu tiên, sau lời giải thích và cú bấm đồng
   * ý (`commune-session.ts`), không bao giờ lúc mở app (ADR 0047:251). Không truyền (chạy thử, test) thì
   * mọi việc cá nhân nói "chưa kết nối" và không gọi mạng.
   */
  openSession?: OpenCommuneAppSession;
}) {
  const { domain, getName, getSceneLocation, draftStore, openSession } = props;
  const [page, setPage] = useState<LookupState>({ kind: "dang-tra" });
  /** Mỗi lần bấm "Thử lại" tăng một — hiệu ứng tra chạy lại đúng một lần cho mỗi giá trị. */
  const [attempt, setAttempt] = useState(0);
  const [name, setName] = useState<NameAtEntry>(() =>
    getName === undefined ? { kind: "settled", name: null } : { kind: "checking" },
  );
  /**
   * The ref, not the effect's dependency list, is what makes the check run ONCE: StrictMode mounts, unmounts
   * and remounts in development, and each mount re-runs the effect. A second `getUserInfo` would be a second
   * platform call for the same answer — the same guard `useCommuneNews` uses for the same reason.
   */
  const nameChecked = useRef(false);

  // Started with the commune lookup, not after it: the check never opens a dialog, so running it while
  // the commune name loads costs the citizen nothing, and the name is usually ready by the first screen.
  useEffect(() => {
    if (getName === undefined || nameChecked.current) return;
    nameChecked.current = true;
    void getName("check")
      .catch((): Awaited<ReturnType<GetZaloName>> => ({ kind: "khong-lay-duoc" }))
      .then((result) => setName(afterNameCheck(result)));
  }, [getName]);

  async function askName() {
    if (getName === undefined || name.kind !== "needs-consent") return;
    setName({ kind: "asking" });
    const result = await getName("ask").catch((): Awaited<ReturnType<GetZaloName>> => ({ kind: "khong-lay-duoc" }));
    setName(afterNameAsk(result));
  }

  useEffect(() => {
    let alive = true;
    void lookupCommuneByDomain(domain).then((result) => {
      if (!alive) return;
      const commune_result = communeFromLookup(result);
      setPage("commune" in commune_result ? { kind: "xong", commune: commune_result.commune } : { kind: "loi", text: commune_result.error });
    });
    return () => {
      alive = false;
    };
    // Tên miền là hằng của bản dựng; chỉ `attempt` đổi.
  }, [attempt]);

  if (page.kind === "xong") {
    return (
      <CommuneAppShell
        domain={domain}
        commune={page.commune}
        name={name}
        onAgreeName={() => void askName()}
        onDeclineName={() => setName({ kind: "settled", name: null })}
        getSceneLocation={getSceneLocation}
        draftStore={draftStore}
        openSession={openSession}
      />
    );
  }

  return (
    <div className="xa-app">
      <div className="xa-trang xa-trang--con">
        {page.kind === "dang-tra" ? (
          <StatusBlock icon="build" text={COMMUNE_APP.opening} loading />
        ) : (
          <StatusBlock
            icon="alert"
            error
            text={page.text}
            button={{
              label: COMMUNE_APP.retry,
              onPress: () => {
                setPage({ kind: "dang-tra" });
                setAttempt((n) => n + 1);
              },
            }}
          />
        )}
      </div>
    </div>
  );
}
