/**
 * APP RIÊNG CỦA MỘT XÃ — toàn bộ giao diện sau khi mở (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Mong muốn của chủ dự án (28/09/2026): chọn app của xã trên Zalo là thấy NGAY giao diện của xã ấy; xã
 * sau y hệt, không sửa mã. GIAO DIỆN theo bản mẫu `vi-gov/zalo-miniapp`, ĐỦ CÁC MÀN (chủ dự án: "làm đủ
 * các màn như bản mẫu đi"): trang chủ, phản ánh, tin tức, cá nhân, gửi phản ánh, tra cứu hồ
 * sơ, danh bạ, truyền thanh, video, bản đồ, thông báo.
 *
 * DỮ LIỆU:
 *   · công khai — tên xã (`/communes`), tin tức (`/commune-news`), dải ảnh trang chủ (`/commune-news?type=banner`,
 *     ADR 0067 §5), danh bạ (`/commune-staff`), theo tên miền, không phiên, không qua `vihat-miniapp`;
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
 * trữ; lớp vỏ tiêm `draftStore` (`AppRieng`, và app chung qua QR xã — khoá theo tên miền xã, 06/10/2026), tệp
 * duy nhất chạm kho là `commune-app/feedback-draft-store.ts`.
 *
 * TWO HOSTS (owner 06/10/2026): the commune's own app (`AppRieng`) and the SHARED app opened from a commune QR
 * (`QrCommuneApp`), which borrows this whole interface until the commune apps can be published.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: router, lưu trữ trực tiếp, tên xã từ biến môi trường, lớp gọi máy chủ của nó, OTP,
 * quét căn cước, số ngày cam kết viết cứng (luật 10). Bước chọn lĩnh vực CÓ, và lĩnh vực ấy là của phiếu (ADR 0050, thay ADR 0049).
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP (ADR 0047 §6). Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log.
 */
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { communeProfiles, type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import { communeBanners, type NewsReadResult } from "../api/goi-vigov";
import type { CommuneProfile, XaTraDuoc } from "../api/hop-dong-cong-khai";
import { type CommuneBannerItem, readHttpsLink } from "../api/hop-dong-cong-khai";
import { communeAppReopen, dropCommuneAppSession, type OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";

import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import {
  createSessionGate,
  offersAccountless,
  offersManualSend,
  type SessionGate,
  sessionGateMessage,
  sessionGateOffersRetry,
  type SessionGateState,
} from "./commune-session";
import { dichGoi } from "./DanhBaCanBoScreen";
import { ThanDanhBaXa, useDanhBaXa } from "./DanhBaXa";
import { DauManCon, KhoiTrangThai, RootTabHeader, SectionHeader, type SectionAccent, type Tone, TrangCon } from "./khung-xa";
import { type OpenExternal, useLeaveApp } from "./leave-app";
import {
  ACCOUNTLESS,
  APP_RIENG,
  COMMUNE_APP_SESSION,
  COMMUNE_OFFICE,
  CUA_TOI,
  DANH_BA,
  PHONE_VERIFICATION,
  type PhoneVerificationTask,
  TIN_XA,
  TYPED_CONTACT,
  XA_GIAO_DIEN,
  XA_PA,
  XA_TN,
  zaloFailureSentence,
  zaloSupportCode,
} from "./noi-dung";
import {
  AccountlessLookup,
  CommuneSendScreen,
  ListStatus,
  type MyPetitions,
  type OnSessionLost,
  PetitionCard,
  PetitionDetail,
  PetitionList,
  PetitionLookup,
  useMyPetitions,
} from "./PhanAnhAppXa";
import { CaNhanXa, type CoChu, ManChuaCoDuLieu, TraCuuHoSoXa } from "./TienIchAppXa";
import { BaiTinXa, DanhSachTinXa, FeaturedNews, HangTin, NewsOfType, type OpenVideo, useTinXa } from "./TinTucAppXa";
import type { GetSceneLocation } from "./scene-location";
import type { PickScenePhotos } from "./scene-photos";
import {
  afterNameAsk,
  afterNameCheck,
  type FeedbackDraftStore,
  type LayTenZalo,
  type NameAtEntry,
  nameShown,
} from "./trai-nghiem";

export type XaCuaApp = { readonly ten: string; readonly tinh: string };

/** Kết quả tra xã → xã của app, hoặc câu báo lỗi. THUẦN. Đúng MỘT xã, tên không rỗng. */
export function xaTuKetQuaTra(
  kq: KetQuaCongKhai<readonly XaTraDuoc[]>,
): { readonly xa: XaCuaApp } | { readonly loi: string } {
  if (kq.kieu === "xong" && kq.gia_tri.length === 1 && kq.gia_tri[0]!.ten.trim() !== "") {
    return { xa: kq.gia_tri[0]! };
  }
  if (kq.kieu === "xong" || kq.kieu === "khong-hop-le" || kq.kieu === "khong-thay") {
    return { loi: APP_RIENG.khong_thay };
  }
  return { loi: APP_RIENG.chua_ket_noi };
}

/* ═════════════════════════════════ ĐIỀU HƯỚNG — không router ═════════════════════════════════ */

export type TabXa = "trang-chu" | "phan-anh" | "tin-tuc" | "ca-nhan";

type ManXa =
  | { readonly kieu: "tab"; readonly tab: TabXa }
  | { readonly kieu: "gui" }
  | { readonly kieu: "phieu"; readonly ma: string; readonly tu: TabXa }
  | { readonly kieu: "bai"; readonly id: string; readonly tu: TabXa }
  | { readonly kieu: "danh-ba" }
  | { readonly kieu: "tra-cuu" }
  | { readonly kieu: "tra-cuu-phieu" }
  /** ADR 0083 (TEMPORARY): the accountless send, and its public lookup (`ma` "" = the citizen types it). */
  | { readonly kieu: "gui-khong-tai-khoan" }
  | { readonly kieu: "tra-cuu-khong-tai-khoan"; readonly ma: string }
  | { readonly kieu: "truyen-thanh" }
  | { readonly kieu: "video" }
  | { readonly kieu: "ban-do" }
  | { readonly kieu: "thong-bao" }
  | { readonly kieu: "su-kien" };

/**
 * One string per screen a citizen can be on — what the entry motion compares to tell "moved to another screen" from
 * "the same screen re-rendered". Two articles (or two petitions) are two screens. Memory only: never logged, never
 * sent. PURE, exported for tests.
 */
export function screenKey(m: ManXa): string {
  if (m.kieu === "tab") return `tab:${m.tab}`;
  if (m.kieu === "bai") return `bai:${m.id}`;
  if (m.kieu === "phieu") return `phieu:${m.ma}`;
  return m.kieu;
}

/**
 * How long the frame keeps its entry class: the 200ms of `xa-enter` (`styles.css`) plus a margin. After that the
 * class is GONE, which is the point: a frame that is later hidden by the gate and shown again must not play the
 * entry a second time (a `display: none` → shown element restarts any animation it still carries).
 */
const SCREEN_ENTRY_MS = 260;

// The prototype's four marks (`PROTOTYPE.md` §5.4): House · MessageSquareWarning · Newspaper · User.
const TAB: ReadonlyArray<{ tab: TabXa; nhan: string; bieu_tuong: TenBieuTuong }> = [
  { tab: "trang-chu", nhan: XA_GIAO_DIEN.tab_trang_chu, bieu_tuong: "home" },
  { tab: "phan-anh", nhan: XA_GIAO_DIEN.tab_phan_anh, bieu_tuong: "message-square-warning" },
  { tab: "tin-tuc", nhan: XA_GIAO_DIEN.tab_tin_tuc, bieu_tuong: "newspaper" },
  { tab: "ca-nhan", nhan: XA_TN.tab_ca_nhan, bieu_tuong: "user" },
];

/**
 * Thanh tab dưới — bốn mục, KHÔNG nút nổi, theo prototype khách (`BottomNav.tsx`). "Gửi phản ánh" có ở
 * nhóm "Chính quyền số" trên trang chủ và ở chân tab Phản ánh (`PetitionSendFooter`). The current tab says so
 * three ways: `aria-current`, bold label, heavier stroke — the brand colour is the fourth, never the only one.
 */
function ThanhTabXa({ tab, onChon }: { tab: TabXa; onChon: (t: TabXa) => void }) {
  return (
    <nav className="xa-tab" aria-label={XA_GIAO_DIEN.thanh_tab}>
      {TAB.map((t) => (
        <button
          key={t.tab}
          type="button"
          className={`xa-tab__muc${t.tab === tab ? " xa-tab__muc--on" : ""}`}
          aria-current={t.tab === tab ? "page" : undefined}
          onClick={() => onChon(t.tab)}
        >
          <BieuTuong ten={t.bieu_tuong} co={24} stroke={t.tab === tab ? 2.4 : 1.8} />
          <span>{t.nhan}</span>
        </button>
      ))}
    </nav>
  );
}

/* ═════════════════════════════════ TRANG CHỦ ═════════════════════════════════ */

/**
 * THE BUNDLED HOME BANNER — one rounded picture under the header. Since ADR 0067 §5 it is the FALLBACK of the
 * strip below (`HomeBanner`): shown while the strip loads, when the commune has posted no banner, when the call
 * fails, or when every posted picture failed to load — never a blank hero.
 *
 * STILL the per-domain bundle exception of ADR 0047:251 (`deploy.mjs --vao-thang` copies
 * `scripts/banner-xa/<domain>.png` into THAT build only, as `./banner-xa.png`). ADR 0067 §5 decision 6 ends that
 * exception once the strip is built; it is built (01/10/2026), but the fallback is kept until EVERY commune has
 * posted at least one banner — removing it earlier blanks the home screen of each commune that has not. When
 * that holds, this component, `scripts/banner-xa/` and the copy step in `deploy.mjs` go together. (On
 * 01/10/2026 no PNG is checked in under `scripts/banner-xa/`, so today the fallback draws nothing in any build;
 * the copy step stays because the deploy script's own tests pin it and a commune may still supply one.)
 *
 * No file (every other build, or a commune that supplied none) → `onError` → NOTHING at all, not a
 * placeholder: a grey box where the commune posted nothing reads as a screen that failed to load.
 * `alt=""`: the picture is decoration; the commune's name is the header's words.
 */
export function CommuneBanner() {
  const [missing, setMissing] = useState(false);
  if (missing) return null;
  return (
    <div className="xa-banner">
      <img className="xa-banner__anh" src="./banner-xa.png" alt="" onError={() => setMissing(true)} />
    </div>
  );
}

/**
 * THE IN-APP PATHS A BANNER MAY OPEN (`link_to` starting with `/`, ADR 0067 §5 decision 2) — a CLOSED table.
 *
 * The server accepts any `/…` path (migration 0012's CHECK), so which ones LEAD anywhere is decided here, by
 * the screens this app has. A path not in the table makes the banner a picture, not a button: a tap that
 * opens nothing — or the wrong screen — on a public authority's home page is worse than no tap. The path
 * words are the tabs' and tiles' own names, so a staff member can guess them; `/tin-tuc/<id>` opens one
 * article (the id is the server's own, as in the news list). No query, no fragment, one trailing `/` allowed.
 * `/gui-phan-anh` passes the same session gate as the home tile (`go`). Home (`/`) is not listed: the strip
 * sits on it.
 */
const BANNER_ROUTES: Readonly<Record<string, ManXa>> = {
  "/tin-tuc": { kieu: "tab", tab: "tin-tuc" },
  "/phan-anh": { kieu: "tab", tab: "phan-anh" },
  "/ca-nhan": { kieu: "tab", tab: "ca-nhan" },
  "/gui-phan-anh": { kieu: "gui" },
  "/danh-ba": { kieu: "danh-ba" },
  "/su-kien": { kieu: "su-kien" },
  "/truyen-thanh": { kieu: "truyen-thanh" },
  "/video": { kieu: "video" },
};

/** The screen an in-app `link_to` opens, or `null` (unknown → the banner is not tappable). PURE, exported for tests. */
export function bannerScreen(path: string): ManXa | null {
  if (!path.startsWith("/") || path.startsWith("//") || /[?#\s\\]/.test(path)) return null;
  const p = path.length > 1 && path.endsWith("/") ? path.slice(0, -1) : path;
  const known = BANNER_ROUTES[p];
  if (known !== undefined) return known;
  const article = /^\/tin-tuc\/([^/]+)$/.exec(p);
  if (article === null) return null;
  try {
    const id = decodeURIComponent(article[1]!);
    return id.trim() === "" ? null : { kieu: "bai", id, tu: "trang-chu" };
  } catch {
    return null; // a malformed %-escape
  }
}

/** What a tap on a banner does: open a screen, ask before an https page, or nothing. PURE, exported for tests. */
export type BannerTap = { readonly kind: "screen"; readonly screen: ManXa } | { readonly kind: "web"; readonly url: string };

/**
 * `linkTo` → the tap, or `null` (a picture only). An https target is tappable only when the shell gave an opener
 * (`canOpenWeb`) — without one (tests, a build with no opener) a button would do nothing.
 */
export function bannerTap(linkTo: string | undefined, canOpenWeb: boolean): BannerTap | null {
  if (linkTo === undefined) return null;
  if (linkTo.startsWith("/")) {
    const screen = bannerScreen(linkTo);
    return screen === null ? null : { kind: "screen", screen };
  }
  return canOpenWeb && readHttpsLink(linkTo) !== null ? { kind: "web", url: linkTo } : null;
}

/**
 * THE BANNER STRIP (ADR 0067 §5) — the commune's posted pictures, in the order the server returned them
 * (`display_order` ascending). Each picture's words are its `title`, as `alt`. A tappable one is a full-width
 * `<button>` around the picture, so its accessible name is the title; the others are plain pictures.
 *
 * NO CAROUSEL, NO AUTOPLAY (same reason as before): one picture fills the width; with more, each takes most of
 * it and the next one shows at the edge, and the citizen swipes when they choose to. PURE: `onFail` reports a
 * picture that did not load, and the caller drops it.
 */
export function BannerStrip(props: {
  items: readonly CommuneBannerItem[];
  canOpenWeb: boolean;
  onScreen: (m: ManXa) => void;
  onWeb: (url: string) => void;
  onFail: (id: string) => void;
}) {
  const many = props.items.length > 1;
  return (
    <div className="xa-banner">
      <ul className={`xa-banner-strip${many ? " xa-banner-strip--many" : ""}`} aria-label={XA_TN.banner_strip}>
        {props.items.map((b) => {
          const picture = (
            <img className="xa-banner__anh" src={b.imageUrl} alt={b.title} decoding="async" onError={() => props.onFail(b.id)} />
          );
          const tap = bannerTap(b.linkTo, props.canOpenWeb);
          return (
            <li key={b.id} className="xa-banner-strip__item">
              {tap === null ? (
                picture
              ) : (
                <button
                  type="button"
                  className="xa-banner-strip__tap"
                  onClick={() => (tap.kind === "screen" ? props.onScreen(tap.screen) : props.onWeb(tap.url))}
                >
                  {picture}
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * The strip as loaded this open — `null` while loading, on any failure, or when the commune posted none (all
 * three → the bundled fallback; a strip that failed says nothing, like the category chips: the home screen works
 * without it). Loaded once, with the ref guard `useTinXa` uses against StrictMode's double run.
 */
function useCommuneBanners(ten_mien: string): readonly CommuneBannerItem[] | null {
  const [items, setItems] = useState<readonly CommuneBannerItem[] | null>(null);
  const loaded = useRef(false);
  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void loadBanners(
      () => communeBanners(ten_mien),
      (ms) => new Promise((done) => setTimeout(done, ms)),
    ).then((got) => {
      if (got !== null) setItems(got);
    });
  }, [ten_mien]);
  return items;
}

/** Wait before the one banner retry when a 429 named no wait, and the cap when it named a long one. */
const BANNER_RETRY_DEFAULT_SECONDS = 5;
const BANNER_RETRY_MAX_SECONDS = 60;

/**
 * The strip's load: the posted banners, or `null` for the bundled picture — SILENTLY, whatever went wrong.
 *
 * A 429 (owner, 02/10/2026: the public news reads are limited per client) gets EXACTLY ONE retry, after the
 * server's `Retry-After` (capped at 60 s; 5 s when absent). One, never a loop: a second 429 — or any other
 * failure — ends in `null` and the bundled picture stays. The strip is decoration over a home screen that works
 * without it, so a failure here never says anything (same stance as the category chips). PURE apart from
 * `load` and `wait`, which the caller injects.
 */
export async function loadBanners(
  load: () => Promise<NewsReadResult<readonly CommuneBannerItem[]>>,
  wait: (ms: number) => Promise<void>,
): Promise<readonly CommuneBannerItem[] | null> {
  let kq = await load();
  if (kq.kieu === "rate-limited") {
    const seconds = kq.retryAfterSeconds === null ? BANNER_RETRY_DEFAULT_SECONDS : kq.retryAfterSeconds;
    await wait(Math.min(Math.max(seconds, 1), BANNER_RETRY_MAX_SECONDS) * 1000);
    kq = await load();
  }
  return kq.kieu === "xong" && kq.gia_tri.length > 0 ? kq.gia_tri : null;
}

/**
 * The home hero: the posted strip when there is something to show, else the bundled picture. A picture that
 * fails to load is dropped; when every one has failed, the bundled picture comes back — never a blank band.
 */
export function HomeBanner(props: {
  items: readonly CommuneBannerItem[] | null;
  canOpenWeb: boolean;
  onScreen: (m: ManXa) => void;
  onWeb: (url: string) => void;
}) {
  const [failed, setFailed] = useState<ReadonlySet<string>>(() => new Set());
  const shown = (props.items ?? []).filter((b) => !failed.has(b.id));
  if (shown.length === 0) return <CommuneBanner />;
  return (
    <BannerStrip
      items={shown}
      canOpenWeb={props.canOpenWeb}
      onScreen={props.onScreen}
      onWeb={props.onWeb}
      onFail={(id) => setFailed((s) => new Set(s).add(id))}
    />
  );
}

type OMenu = {
  nhan: string;
  bieu_tuong: TenBieuTuong;
  mau: Tone;
  man: ManXa;
};

/**
 * HAI NHÓM CHỨC NĂNG — đúng bố cục prototype khách (`FunctionGrid.tsx`): "Chính quyền số" và "Thông tin –
 * Truyền thông", lưới bốn cột, nền màu phủ cả ô (ô to thì vùng chạm to). Chia nhóm để người dân tìm theo
 * loại việc, và để xã thêm ô sau này không vỡ bố cục.
 */
// vi-name-ok: existing constant, now exported so `commune-screens.test.tsx` pins the tiles (rule 12 #3)
export const NHOM_CHUC_NANG: ReadonlyArray<{ tieu_de: string; vach: SectionAccent; them?: { nhan: string; man: ManXa }; o: readonly OMenu[] }> = [
  {
    tieu_de: XA_TN.nhom_chinh_quyen,
    // `.xa-dau-nhom` without a modifier IS the brand bar; `--brand` names it for the reader, no rule needed.
    vach: "brand",
    // `PROTOTYPE.md` §6.1: "Xem tất cả" of this group opens the directory.
    them: { nhan: XA_GIAO_DIEN.xem_tat_ca, man: { kieu: "danh-ba" } },
    // Marks and tones of §6.1 (wave 1, 30/09/2026).
    o: [
      { nhan: XA_GIAO_DIEN.o_gui, bieu_tuong: "camera-pin", mau: "red", man: { kieu: "gui" } },
      { nhan: XA_TN.o_tra_cuu_ngan, bieu_tuong: "file-search", mau: "xanh", man: { kieu: "tra-cuu" } },
      { nhan: XA_GIAO_DIEN.o_danh_ba, bieu_tuong: "contact-book", mau: "luc", man: { kieu: "danh-ba" } },
      // SRS M6.1.12 (P1). The screen had no way in, so it read as "done" to anyone reading the code. Not in
      // the prototype's grid; kept (owner, wave 1).
      { nhan: XA_TN.o_ban_do, bieu_tuong: "map", mau: "cam", man: { kieu: "ban-do" } },
    ],
  },
  {
    tieu_de: XA_TN.nhom_thong_tin,
    vach: "cam",
    them: { nhan: XA_TN.xem_them, man: { kieu: "tab", tab: "tin-tuc" } },
    o: [
      { nhan: XA_TN.o_tin_tuc, bieu_tuong: "newspaper", mau: "xanh", man: { kieu: "tab", tab: "tin-tuc" } },
      { nhan: XA_TN.o_truyen_thanh, bieu_tuong: "speaker", mau: "cyan", man: { kieu: "truyen-thanh" } },
      { nhan: XA_TN.o_video, bieu_tuong: "video", mau: "cam", man: { kieu: "video" } },
      { nhan: XA_TN.o_su_kien, bieu_tuong: "calendar", mau: "tim", man: { kieu: "su-kien" } },
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
        {XA_TN.name_card_title}
      </h2>
      <p>{XA_TN.name_card_why}</p>
      <p className="xa-phu">{XA_TN.name_card_zalo_asks}</p>
      {props.asking && (
        <p className="xa-phu" role="status">
          {XA_TN.name_card_asking}
        </p>
      )}
      <button type="button" className="xa-nut" onClick={props.onAgree} disabled={props.asking}>
        {XA_TN.name_card_agree}
      </button>
      <button type="button" className="xa-nut xa-nut--phu" onClick={props.onDecline} disabled={props.asking}>
        {XA_TN.name_card_decline}
      </button>
    </section>
  );
}

/**
 * The commune's profile from `/commune-profiles` (public, by domain, no session) — or `null` while loading,
 * when the call failed, or when the answer is not exactly one profile. `null` shows NOTHING: the office
 * block is information, the app works without it, and a guessed hotline is a wrong number published by a
 * public authority. Loaded once per open, with the ref guard `useTinXa` uses against StrictMode's double run.
 */
function useCommuneProfile(ten_mien: string): CommuneProfile | null {
  const [profile, setProfile] = useState<CommuneProfile | null>(null);
  const loaded = useRef(false);
  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void communeProfiles(ten_mien).then((kq) => {
      if (kq.kieu === "xong" && kq.gia_tri.length === 1) setProfile(kq.gia_tri[0]!);
    });
  }, [ten_mien]);
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
  const dial = hotline === "" ? null : dichGoi(hotline);
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
          <BieuTuong ten="phone" co={20} />
          {COMMUNE_OFFICE.call_hotline(hotline)}
        </a>
      )}
    </section>
  );
}

function TrangChuXa(props: {
  xa: XaCuaApp;
  ho_ten: string | null;
  /** The entry card, when the name still needs the citizen's consent; otherwise `null`. */
  name_card: ReactNode;
  tin: ReturnType<typeof useTinXa>;
  /** "Phản ánh của tôi" as loaded in this open — `idle` until the citizen asks (the gate). */
  petitions: MyPetitions;
  onOpenPetitions: () => void;
  onRetryPetitions: () => void;
  /** The commune's declared office (`/commune-profiles`), or `null` — then no office block. */
  profile: CommuneProfile | null;
  /** The posted banner strip (`useCommuneBanners`), or `null` — then the bundled picture. */
  banners: readonly CommuneBannerItem[] | null;
  /** Ask before opening a banner's https page (`useLeaveApp().ask`); absent → such banners are not tappable. */
  askLeave?: (url: string) => void;
  di: (m: ManXa) => void;
}) {
  const { xa, tin, di, ho_ten, petitions } = props;
  // Hai phiếu, hai tin — đúng bố cục trang chủ của prototype (`HomePage.tsx`).
  const tin_moi = tin.ds.muc.slice(0, 2);
  const phieu_moi = petitions.kind === "ready" ? petitions.items.slice(0, 2) : [];

  return (
    <>
      {/* Header theo prototype (`AppHeader.tsx`): logo xã trái, tên xã giữa, lời chào dưới. Nó đứng NGOÀI vùng
          cuộn — chỉ phần giữa cuộn (§5.2). Góc phải để trống cho Zalo: chuông đã bỏ (quyết định 10, 30/09/2026),
          "Thông báo" nay là một dòng ở Cá nhân › Của tôi. */}
      <RootTabHeader
        title={xa.ten}
        subtitle={ho_ten !== null ? XA_TN.xin_chao_ten(ho_ten) : xa.tinh}
        logoUrl={profileLogoUrl(props.profile)}
      />
      <div className="xa-trang">
        <HomeBanner
          items={props.banners}
          canOpenWeb={props.askLeave !== undefined}
          onScreen={di}
          onWeb={(url) => props.askLeave?.(url)}
        />

        <div className="xa-trang__than">
          {props.name_card}
          {NHOM_CHUC_NANG.map((nhom) => (
            <section key={nhom.tieu_de} className="xa-the xa-the--dem xa-nhom-cn">
              <SectionHeader
                title={nhom.tieu_de}
                accent={nhom.vach}
                more={nhom.them && { label: nhom.them.nhan, onPress: () => di(nhom.them!.man) }}
              />
              <div className="xa-luoi-4">
                {nhom.o.map((o) => (
                  <button key={o.nhan} type="button" className={`xa-o-4 xa-mau--${o.mau}`} onClick={() => di(o.man)}>
                    <BieuTuong ten={o.bieu_tuong} co={30} />
                    <span className="xa-o-4__nhan">{o.nhan}</span>
                  </button>
                ))}
              </div>
            </section>
          ))}

          <section className="xa-the xa-the--dem xa-nhom-cn">
            <SectionHeader
              title={XA_GIAO_DIEN.muc_phan_anh}
              accent="luc"
              more={{ label: XA_GIAO_DIEN.xem_tat_ca, onPress: () => di({ kieu: "tab", tab: "phan-anh" }) }}
            />
            {petitions.kind !== "ready" ? (
              // Not loaded: the card that asks — nothing personal is fetched at app open (ADR 0047:251).
              <ListStatus state={petitions} onOpen={props.onOpenPetitions} onRetry={props.onRetryPetitions} />
            ) : phieu_moi.length > 0 ? (
              <ul className="xa-ds">
                {phieu_moi.map((p) => (
                  <li key={p.ma_tra_cuu}>
                    <PetitionCard petition={p} onOpen={() => di({ kieu: "phieu", ma: p.ma_tra_cuu, tu: "trang-chu" })} />
                  </li>
                ))}
              </ul>
            ) : (
              <KhoiTrangThai bieu_tuong="chat" cau={XA_TN.chua_co_phieu} hint={XA_TN.home_petitions_empty_hint} />
            )}
          </section>

          <section className="xa-the xa-the--dem xa-nhom-cn">
            <SectionHeader
              title={XA_GIAO_DIEN.muc_tin_moi}
              accent="xanh"
              more={{ label: XA_GIAO_DIEN.xem_tat_ca, onPress: () => di({ kieu: "tab", tab: "tin-tuc" }) }}
            />
            {!tin.ds.da_co_trang_dau && tin.ds.dang_tai ? (
              <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai} dang_tai />
            ) : !tin.ds.da_co_trang_dau && tin.ds.loi !== null ? (
              <KhoiTrangThai bieu_tuong="alert" loi cau={TIN_XA.loi_may_chu} nut={{ nhan: TIN_XA.nut_thu_lai, onBam: tin.taiTiep }} />
            ) : tin_moi.length === 0 ? (
              <KhoiTrangThai bieu_tuong="news" cau={XA_GIAO_DIEN.tin_moi_trong} hint={XA_TN.news_empty_hint} />
            ) : (
              <ul className="xa-ds">
                {/* The newest item may be the large card — only with a cover that loaded (`FeaturedNews`); the
                    second stays compact. */}
                {tin_moi.map((t, i) => (
                  <li key={t.id}>
                    {i === 0 ? (
                      <FeaturedNews tin={t} onMo={(id) => di({ kieu: "bai", id, tu: "trang-chu" })} />
                    ) : (
                      <HangTin tin={t} compact onMo={(id) => di({ kieu: "bai", id, tu: "trang-chu" })} />
                    )}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <CommuneOffice profile={props.profile} />
        </div>
      </div>
    </>
  );
}

/* ═════════════════════════════════ CÁC TAB KHÁC ═════════════════════════════════ */

/**
 * "Gửi phản ánh mới" at the FOOT of the Phản ánh tab, as the prototype's `FeedbackListPage` footer: fixed
 * above the tab bar, full width — where the thumb already is, and where it stays while the list scrolls,
 * instead of scrolling away at the top. Its own words (`send_new_petition`); the home tile keeps `o_gui`.
 * The tap goes through the same gate as every personal act (`go` → `requireSession`).
 */
export function PetitionSendFooter({ onSend }: { onSend: () => void }) {
  return (
    <div className="xa-chan-gui">
      <button type="button" className="xa-nut xa-nut--chan" onClick={onSend}>
        <BieuTuong ten="plus" co={22} />
        {XA_TN.send_new_petition}
      </button>
    </div>
  );
}

/** Header of a root tab other than home: the commune's logo, the tab's title centred (`RootTabHeader`). */
function DauTab({ tieu_de, logoUrl }: { tieu_de: string; logoUrl: string }) {
  return <RootTabHeader title={tieu_de} logoUrl={logoUrl} />;
}

/**
 * The uploaded logo of the profile this open read, or `""` while it loads / when it failed — the header then
 * shows the bundled file (`LogoXa`). The profile is the one `/commune-profiles` returned for THIS build's domain;
 * nothing the citizen sends chooses it. PURE, exported for tests.
 */
export function profileLogoUrl(profile: CommuneProfile | null): string {
  return profile === null ? "" : profile.logo_url;
}

/**
 * The gate, as a screen of its own — one task per screen (`skills/accessibility-elderly` #4). The screen it
 * covers stays MOUNTED underneath (hidden), so a half-written petition survives the detour.
 *
 *   `hoi`       WHY first, then what Zalo will ask, then the button — Zalo's dialog opens only on "Đồng ý"
 *               (policy 3.3.4; the words are `PHONE_VERIFICATION`'s, the same act as the shared app's — except
 *               where the number GOES: `COMMUNE_APP_SESSION.zalo_asks`, ADR 0066). When the same tap also asks
 *               for the name (`asksName`), both sentences name both (`why_with_name`, `zalo_asks_with_name`)
 *   `dang-mo`   words, not a spinner — and "Về trang chủ", which works while the open runs
 *   `ket-qua`   one sentence saying what to do next, "Về trang chủ" always, a retry only where a new tap can
 *               help — the retry is "Đồng ý chia sẻ số điện thoại" again, since that tap is what opens Zalo's dialog
 *               — and, for SENDING when Zalo gave no number, "Gửi bằng họ tên và số điện thoại" under the line that
 *               says what that path means (no notification, follow by code) BEFORE the citizen chooses it (ADR 0080)
 *
 * Exported for `session-gate-exits.test.tsx` only.
 */
export function SessionGateScreen(props: {
  state: SessionGateState;
  task: PhoneVerificationTask;
  onAllow: () => void;
  onDecline: () => void;
  /** "Gửi bằng họ tên và số điện thoại" — absent = never offered (tests that pin the older screen). */
  onManual?: () => void;
  /**
   * The same tap also asks Zalo for the NAME (sending, name not settled — owner, 09/10/2026): the words before it
   * then say both (policy 3.3.4). Absent/false = only the number is asked, and only the number is named.
   */
  asksName?: boolean;
  /**
   * The accountless path (ADR 0083): "Gửi phản ánh không cần tài khoản" / "Tra cứu bằng mã phiếu" — only where
   * `offersAccountless` says (Zalo refused the access token). Absent = never offered.
   */
  onAccountless?: () => void;
  onClose: () => void;
}) {
  const { state } = props;
  const manual =
    state.kieu === "ket-qua" &&
    props.onManual !== undefined &&
    offersManualSend(state.outcome, props.task, state.zalo);
  const accountless =
    state.kieu === "ket-qua" &&
    props.onAccountless !== undefined &&
    offersAccountless(state.outcome, props.task, state.zalo);
  return (
    <>
      <DauManCon tieu_de={PHONE_VERIFICATION.title} onQuayLai={props.onClose} />
      <TrangCon>
        {state.kieu === "dang-mo" && (
          <>
            <KhoiTrangThai bieu_tuong="user" cau={COMMUNE_APP_SESSION.working} dang_tai shape="none" />
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onClose}>
              {XA_TN.nut_ve_trang_chu}
            </button>
          </>
        )}
        {state.kieu === "hoi" && (
          <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-cong-tieu-de">
            <h2 className="xa-dau-khoi__tieu-de" id="xa-cong-tieu-de">
              {PHONE_VERIFICATION.title}
            </h2>
            <p>{props.asksName === true ? COMMUNE_APP_SESSION.why_with_name : PHONE_VERIFICATION.why}</p>
            <p className="xa-phu">
              {props.asksName === true ? COMMUNE_APP_SESSION.zalo_asks_with_name : COMMUNE_APP_SESSION.zalo_asks}
            </p>
            <button type="button" className="xa-nut" onClick={props.onAllow}>
              {PHONE_VERIFICATION.allow}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onDecline}>
              {PHONE_VERIFICATION.decline}
            </button>
          </section>
        )}
        {state.kieu === "ket-qua" && (
          <>
            <KhoiTrangThai
              bieu_tuong="alert"
              loi
              cau={sessionGateMessage(state.outcome, props.task, state.zalo)}
              support_code={state.outcome === "thu-lai" ? zaloSupportCode(state.zalo) : null}
              nut={
                sessionGateOffersRetry(state.outcome, state.zalo)
                  ? { nhan: PHONE_VERIFICATION.allow, onBam: props.onAllow }
                  : undefined
              }
            />
            {manual && (
              <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-cong-tu-nhap">
                <p id="xa-cong-tu-nhap">{TYPED_CONTACT.offer}</p>
                <button type="button" className="xa-nut" onClick={props.onManual}>
                  {TYPED_CONTACT.button}
                </button>
              </section>
            )}
            {/* What the path costs is said ABOVE the button, before the citizen chooses it (ADR 0080 cost #2). */}
            {accountless && (
              <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-cong-khong-tk">
                <p id="xa-cong-khong-tk">{props.task === "lookup" ? ACCOUNTLESS.lookup_offer : ACCOUNTLESS.offer}</p>
                <button type="button" className="xa-nut" onClick={props.onAccountless}>
                  {props.task === "lookup" ? ACCOUNTLESS.lookup_button : ACCOUNTLESS.send_button}
                </button>
              </section>
            )}
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onClose}>
              {XA_TN.nut_ve_trang_chu}
            </button>
          </>
        )}
      </TrangCon>
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

function ManDanhBa({ ten_mien, onQuayLai }: { ten_mien: string; onQuayLai: () => void }) {
  const { trang, taiLai } = useDanhBaXa(ten_mien);
  return (
    <>
      <DauManCon tieu_de={DANH_BA.tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <ThanDanhBaXa trang={trang} onTai={taiLai} />
      </TrangCon>
    </>
  );
}

/* ═════════════════════════════════ APP ═════════════════════════════════ */

function AppCuaXa(props: {
  ten_mien: string;
  xa: XaCuaApp;
  /** The name request of this open, owned by `TrangXa` (the entry). Screens below only READ it. */
  name: NameAtEntry;
  onAgreeName: () => void;
  onDeclineName: () => void;
  getSceneLocation?: GetSceneLocation;
  pickScenePhotos?: PickScenePhotos;
  draftStore?: FeedbackDraftStore;
  openSession?: OpenCommuneAppSession;
  openVideo?: OpenVideo;
  openLink?: OpenExternal;
}) {
  const { ten_mien, xa, openSession } = props;
  // Họ tên lấy MỘT LẦN lúc mở app (`TrangXa`). Phiên ViGov chỉ trong bộ nhớ (`api/phien-vigov.ts`), mở ở
  // việc cá nhân đầu tiên qua cổng dưới đây. Thứ DUY NHẤT sống qua lần đóng app là NHÁP đang soạn, qua
  // `draftStore` lớp vỏ tiêm (ADR 0050 #7).
  const shownName = nameShown(props.name);
  const [co_chu, datCoChu] = useState<CoChu>("vua");
  const [man, datMan] = useState<ManXa>({ kieu: "tab", tab: "trang-chu" });
  const tin = useTinXa(ten_mien);
  const profile = useCommuneProfile(ten_mien);
  const banners = useCommuneBanners(ten_mien);
  // The home strip's "leave the app?" question. An article asks through its own (`BaiTinXa`).
  const leave = useLeaveApp(props.openLink);
  const veTab = (tab: TabXa) => datMan({ kieu: "tab", tab });
  const lop = `xa-app xa-co-chu--${co_chu}`;

  /*
   * THE ENTRY MOTION (owner, UI-1 02/10/2026). `entering` is decided DURING the render that changes the screen —
   * never in an effect after it, which would paint the new screen once still and then fade it in from nothing (a
   * flash). The effect only removes the class once the motion is over (`SCREEN_ENTRY_MS`). No `key` is added
   * anywhere: the tab bodies keep their state (a loaded petition list, the news pages) exactly as before.
   */
  const shownKey = screenKey(man);
  const [settledKey, setSettledKey] = useState<string | null>(null);
  const entering = settledKey !== shownKey;
  useEffect(() => {
    if (!entering) return;
    const timer = setTimeout(() => setSettledKey(shownKey), SCREEN_ENTRY_MS);
    return () => clearTimeout(timer);
  }, [entering, shownKey]);

  /* ── THE GATE: every personal act asks here first (`commune-session.ts`) ── */
  const { gate, state: gateState } = useSessionGate(openSession, xa.ten);
  const [gateTask, setGateTask] = useState<PhoneVerificationTask>("submit");
  // The 403 path of the rating block (`usePhoneVerification`): the same opener, the same commune check.
  const reopenWithPhone = useMemo(() => (openSession ? communeAppReopen(openSession) : undefined), [openSession]);

  /*
   * THE ZALO NAME, ASKED IN THE SAME STEP AS THE NUMBER (owner, 09/10/2026). Sending, with the name not settled in
   * this open (`needs-consent`): the gate's words name both (`asksName`), and the one tap on "Đồng ý" asks Zalo for
   * the number and — once a VERIFIED session is open — for the name. At most once per open: `TrangXa` asks only
   * from `needs-consent`, and any answer settles it. Never when the number was refused: the name would serve
   * nothing. Asked BEFORE the act, so the send screen's first render already sees "being asked" and never paints a
   * layout it would then change (`VerifiedSender`).
   */
  const asksName = gateTask === "submit" && props.name.kind === "needs-consent";
  /** Set when the citizen tapped "Đồng ý" on a gate that said it would ask for the name too. */
  const nameAfterPhone = useRef(false);
  const askName = useRef(props.onAgreeName);
  askName.current = props.onAgreeName;

  /** Run `act` with a session — at once if one exists, otherwise after the explanation and the tap. */
  function requireSession(task: PhoneVerificationTask, act: () => void) {
    setGateTask(task);
    gate.require(() => {
      if (nameAfterPhone.current && task === "submit" && layPhienViGov()?.phone_verified === true) askName.current();
      nameAfterPhone.current = false;
      act();
      // A session now exists: the home block and Cá nhân fill in without a second question — but only with a
      // VERIFIED phone. A phone-less session (ADR 0080) is refused by the list route, and that refusal would
      // drop the session and put the gate over the form the citizen just opened.
      if (layPhienViGov()?.phone_verified === true) petitions.loadIfIdle();
    }, task);
  }

  /** 401 / 403 `chua_xac_thuc_so`: forget the session, and run the act again through the gate. */
  const sessionLost =
    (task: PhoneVerificationTask): OnSessionLost =>
    (retry) => {
      dropCommuneAppSession();
      requireSession(task, retry);
    };

  const petitions = useMyPetitions(sessionLost("mine"));
  const openPetitions = () => requireSession("mine", () => void petitions.load());

  /** Navigation from a tap. The two personal screens pass the gate; everything else is public. */
  function go(m: ManXa) {
    if (m.kieu === "gui") requireSession("submit", () => datMan(m));
    else if (m.kieu === "tra-cuu-phieu") requireSession("lookup", () => datMan(m));
    else datMan(m);
  }

  const ve = () => veTab("trang-chu");
  let man_con = null;
  switch (man.kieu) {
    case "bai":
      man_con = (
        <BaiTinXa
          key={man.id}
          ten_mien={ten_mien}
          id={man.id}
          ds={tin.ds.muc}
          onMo={(id) => datMan({ kieu: "bai", id, tu: man.tu })}
          onQuayLai={() => veTab(man.tu)}
          openVideo={props.openVideo}
          openLink={props.openLink}
        />
      );
      break;
    case "gui":
      man_con = (
        <CommuneSendScreen
          ten_xa={xa.ten}
          ho_ten={shownName}
          nameAtEntry={props.name}
          getSceneLocation={props.getSceneLocation}
          pickScenePhotos={props.pickScenePhotos}
          draftStore={props.draftStore}
          typedContact={layPhienViGov()?.phone_verified === false}
          verifiedPhone={layPhienViGov()?.phone_verified === true}
          onBack={ve}
          onSessionLost={sessionLost("submit")}
          onSent={() => void petitions.load()}
          onOpenPetition={(ma) => datMan({ kieu: "phieu", ma, tu: "phan-anh" })}
        />
      );
      break;
    case "phieu":
      man_con = (
        <PetitionDetail
          key={man.ma}
          code={man.ma}
          onBack={() => veTab(man.tu)}
          onSessionLost={sessionLost("mine")}
          onChanged={() => void petitions.load()}
          reopenWithPhone={reopenWithPhone}
          pickScenePhotos={props.pickScenePhotos}
        />
      );
      break;
    case "danh-ba":
      man_con = <ManDanhBa ten_mien={ten_mien} onQuayLai={ve} />;
      break;
    case "tra-cuu":
      man_con = <TraCuuHoSoXa onQuayLai={ve} />;
      break;
    case "gui-khong-tai-khoan":
      // ADR 0083: no session exists on this path, so nothing here can lose one; `onSessionLost` / `onSent` are
      // never called by the accountless send (`CommuneSendScreen`). No photo picker (no photo route for these),
      // no location exchange (it needs the access token Zalo refused) — the screen drops both anyway.
      man_con = (
        <CommuneSendScreen
          ten_xa={xa.ten}
          ho_ten={shownName}
          draftStore={props.draftStore}
          accountless={{ domain: ten_mien }}
          onBack={ve}
          onSessionLost={() => {}}
          onSent={() => {}}
          onOpenPetition={(ma) => datMan({ kieu: "tra-cuu-khong-tai-khoan", ma })}
        />
      );
      break;
    case "tra-cuu-khong-tai-khoan":
      man_con = <AccountlessLookup key={man.ma} domain={ten_mien} initialCode={man.ma} onBack={() => veTab("phan-anh")} />;
      break;
    case "tra-cuu-phieu":
      man_con = (
        <PetitionLookup
          onBack={() => veTab("phan-anh")}
          onSessionLost={sessionLost("lookup")}
          onChanged={() => void petitions.load()}
          reopenWithPhone={reopenWithPhone}
          pickScenePhotos={props.pickScenePhotos}
        />
      );
      break;
    case "truyen-thanh":
    case "video": {
      // Same list as "su-kien" below, filtered by the server's `?type=` (comms accepts both types). Until
      // 01/10/2026 both tiles opened a static "nothing yet" screen although the commune could already
      // publish these types from web-admin — the citizen was told there was nothing when there was.
      const newsType = man.kieu;
      man_con = (
        <>
          <DauManCon tieu_de={newsType === "video" ? XA_TN.video_tieu_de : XA_TN.truyen_thanh_tieu_de} onQuayLai={ve} />
          <TrangCon>
            <NewsOfType
              ten_mien={ten_mien}
              type={newsType}
              onMo={(id) => datMan({ kieu: "bai", id, tu: "trang-chu" })}
              empty={newsType === "video" ? XA_TN.video_trong : XA_TN.truyen_thanh_trong}
              hint={newsType === "video" ? XA_TN.news_empty_hint : XA_TN.broadcast_empty_hint}
            />
          </TrangCon>
        </>
      );
      break;
    }
    case "ban-do":
      man_con = (
        <ManChuaCoDuLieu
          tieu_de={XA_TN.ban_do_tieu_de}
          bieu_tuong="map"
          cau={XA_TN.ban_do_trong}
          hint={XA_TN.map_empty_hint}
          onQuayLai={ve}
        />
      );
      break;
    case "su-kien":
      // The items the commune published AS EVENTS — the server's `?type=su-kien` (comms b22bf76), no longer
      // a guess from the free-text category.
      man_con = (
        <>
          <DauManCon tieu_de={XA_TN.su_kien_tieu_de} onQuayLai={ve} />
          <TrangCon>
            <NewsOfType
              ten_mien={ten_mien}
              type="su-kien"
              onMo={(id) => datMan({ kieu: "bai", id, tu: "trang-chu" })}
              empty={XA_TN.su_kien_trong}
              hint={XA_TN.news_empty_hint}
            />
          </TrangCon>
        </>
      );
      break;
    case "thong-bao":
      // Reached from Cá nhân › Của tôi (decision 10: no bell on the home header), so "Quay lại" goes back there.
      // The hint is the sentence that stood in Cá nhân's old "Thông báo" section: where messages will come from.
      man_con = (
        <ManChuaCoDuLieu
          tieu_de={XA_TN.thong_bao}
          bieu_tuong="bell"
          cau={XA_TN.thong_bao_trong}
          hint={XA_TN.thong_bao_chua_co}
          onQuayLai={() => veTab("ca-nhan")}
        />
      );
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
        asksName={asksName}
        onAllow={() => {
          nameAfterPhone.current = asksName;
          void gate.allow();
        }}
        onDecline={gate.decline}
        onManual={() => void gate.manual()}
        onAccountless={() => {
          // Leave the gate for the accountless screen (ADR 0083). The pending act is dropped: it needs a session.
          gate.reset();
          datMan(gateTask === "lookup" ? { kieu: "tra-cuu-khong-tai-khoan", ma: "" } : { kieu: "gui-khong-tai-khoan" });
        }}
        onClose={gate.reset}
      />
    );
  // THE SCROLL MODEL (`PROTOTYPE.md` §5.2): `.xa-app` is one viewport-tall column; each `.xa-frame` is the
  // rest of it, holding a header, ONE scrolling `.xa-trang`, and any footer / tab bar — so only the middle
  // moves, and the header and bottom bar stay on screen. `hidden` on a frame must still hide it:
  // `.xa-frame[hidden]` in styles.css, since `display: flex` would otherwise win over the attribute.
  //
  // Motion: the gate's frame is a NEW element each time the gate opens, so it always carries the entry class — it
  // plays once, on mount, and its state changes (asking → working → refused) reuse the element and play nothing.
  // The covered frame carries the class only while `entering`, so being un-hidden when the gate closes replays
  // nothing; it plays when the gate's act moved the citizen to a new screen, which is an entry.
  const withGate = (content: ReactNode) => (
    <div className={lop}>
      {gateScreen !== null && <div className="xa-frame xa-frame--enter">{gateScreen}</div>}
      <div className={entering ? "xa-frame xa-frame--enter" : "xa-frame"} hidden={gateScreen !== null}>
        {content}
      </div>
    </div>
  );

  if (man_con !== null) return withGate(man_con);

  const tab = man.kieu === "tab" ? man.tab : "trang-chu";
  let than;
  if (tab === "trang-chu") {
    // After the citizen agreed and Zalo still refused with a code, the slot says so — which capability, and
    // Zalo's code on its own secondary line — instead of going quiet on "Chưa xác định" (user rule
    // 30/09/2026: a permission the App ID lacks must be SEEN). Words and not colour alone: `role="status"`.
    const name_card =
      props.name.kind === "needs-consent" || props.name.kind === "asking" ? (
        <NameCard asking={props.name.kind === "asking"} onAgree={props.onAgreeName} onDecline={props.onDeclineName} />
      ) : props.name.kind === "settled" && props.name.zalo !== undefined ? (
        <>
          <p className="xa-loi-o" role="status">
            {XA_TN.name_zalo_failed(zaloFailureSentence(props.name.zalo))}
          </p>
          <p className="xa-phu">{zaloSupportCode(props.name.zalo)}</p>
        </>
      ) : null;
    than = (
      <TrangChuXa
        xa={xa}
        ho_ten={shownName}
        name_card={name_card}
        tin={tin}
        petitions={petitions.state}
        onOpenPetitions={openPetitions}
        onRetryPetitions={() => void petitions.load()}
        profile={profile}
        banners={banners}
        askLeave={leave.ask}
        di={go}
      />
    );
  } else if (tab === "tin-tuc") {
    than = (
      <>
        <DauTab tieu_de={XA_TN.news_tab_title} logoUrl={profileLogoUrl(profile)} />
        <div className="xa-trang xa-trang--tab">
          <DanhSachTinXa ten_mien={ten_mien} onMo={(id) => datMan({ kieu: "bai", id, tu: "tin-tuc" })} />
        </div>
      </>
    );
  } else if (tab === "phan-anh") {
    than = (
      <>
        <DauTab tieu_de={CUA_TOI.tieu_de} logoUrl={profileLogoUrl(profile)} />
        <div className="xa-trang xa-trang--tab xa-trang--co-chan">
          <PetitionList
            state={petitions.state}
            onOpenPetition={(ma) => datMan({ kieu: "phieu", ma, tu: "phan-anh" })}
            onLookup={() => go({ kieu: "tra-cuu-phieu" })}
            onOpen={openPetitions}
            onRetry={() => void petitions.load()}
            onLoadMore={() => void petitions.loadMore()}
          />
        </div>
        <PetitionSendFooter onSend={() => go({ kieu: "gui" })} />
      </>
    );
  } else {
    than = (
      <>
        <DauTab tieu_de={XA_TN.ca_nhan_tieu_de} logoUrl={profileLogoUrl(profile)} />
        <CaNhanXa
          ho_ten={shownName}
          ten_xa={xa.ten}
          tinh={xa.tinh}
          so_phieu={
            petitions.state.kind === "ready"
              ? { count: petitions.state.items.length, more: petitions.state.hasMore }
              : null
          }
          co_chu={co_chu}
          onDoiCoChu={datCoChu}
          onMoPhanAnh={() => veTab("phan-anh")}
          onMoTraCuu={() => datMan({ kieu: "tra-cuu" })}
          onOpenNotifications={() => datMan({ kieu: "thong-bao" })}
        />
      </>
    );
  }

  return withGate(
    <>
      <main id="main">{than}</main>
      <ThanhTabXa tab={tab} onChon={veTab} />
      {tab === "trang-chu" && leave.dialog}
    </>,
  );
}

type TrangTra =
  | { readonly kieu: "dang-tra" }
  | { readonly kieu: "loi"; readonly cau: string }
  | { readonly kieu: "xong"; readonly xa: XaCuaApp };

export function TrangXa(props: {
  /** Tên miền xã: nung vào bản dựng (`lib/xa-co-dinh.ts`), hoặc `d` của QR xã trên app chung (`QrCommuneApp`). */
  ten_mien: string;
  /**
   * Lấy họ tên từ Zalo (`getUserInfo`), do lớp vỏ tiêm — nửa này không nhập zmp-sdk (`ranh-gioi-hai-nua.test.ts`
   * §3a). Gọi CHỈ ở đây, lúc mở app (29/09/2026): một lần "check" không bật hộp của Zalo; nếu chưa được phép
   * thì thẻ giải thích mục đích hiện trên trang chủ, và "ask" chỉ gửi khi bà con bấm "Đồng ý" trên thẻ ấy.
   * Không truyền (chạy thử ngoài Zalo, test) thì không hỏi gì và tên là "Chưa xác định".
   */
  lay_ten?: LayTenZalo;
  /**
   * Lấy vị trí hiện tại (`getLocation` + đổi toạ độ ở `vihat-miniapp`), do lớp vỏ tiêm — nửa này không
   * nhập zmp-sdk. Không truyền thì màn gửi phản ánh không có nút vị trí (chạy thử ngoài Zalo, test).
   */
  getSceneLocation?: GetSceneLocation;
  /**
   * Chụp / chọn ảnh hiện trường (`requestCameraPermission` + `chooseImage` / `openMediaPicker`), do lớp vỏ tiêm — app riêng của
   * xã (`AppRieng`) và app chung qua QR xã (`QrCommuneApp`, owner 06/10/2026). Không truyền thì không có nút ảnh
   * nào (test).
   */
  pickScenePhotos?: PickScenePhotos;
  /**
   * Nháp phản ánh đang soạn (ADR 0050 #7), do lớp vỏ tiêm — app riêng của xã, và app chung qua QR xã với khoá
   * theo tên miền xã (06/10/2026). Không truyền thì không có nháp (chạy thử, test): nửa này không tự chạm kho
   * lưu trữ nào.
   */
  draftStore?: FeedbackDraftStore;
  /**
   * Mở phiên công dân ViGov (App ID + `getAccessToken` + `getPhoneNumber` → ViGov identity trực tiếp, ADR 0066), do lớp vỏ
   * tiêm — nửa này không nhập zmp-sdk. Gọi CHỈ ở việc cá nhân đầu tiên, sau lời giải thích và cú bấm đồng
   * ý (`commune-session.ts`), không bao giờ lúc mở app (ADR 0047:251). Không truyền (chạy thử, test) thì
   * mọi việc cá nhân nói "chưa kết nối" và không gọi mạng.
   */
  openSession?: OpenCommuneAppSession;
  /**
   * Mở đường dẫn video xã đăng kèm một tin, ra ngoài app (`moRaNgoai("video", …)`), do lớp vỏ tiêm — nửa này
   * không nhập `features/` hay zmp-sdk. Không truyền (chạy thử, test) thì chi tiết tin không có nút "Xem video".
   */
  openVideo?: OpenVideo;
  /**
   * Mở một liên kết xã gắn trong thân bài hoặc trên ảnh trang chủ, ra ngoài app (`moRaNgoai("lien-ket-xa", …)`),
   * do lớp vỏ tiêm. Mỗi lần bấm đều hỏi trước (`leave-app.tsx`). Không truyền (chạy thử, test) thì chữ liên kết
   * là chữ thường và ảnh trỏ ra ngoài không bấm được.
   */
  openLink?: OpenExternal;
  /**
   * Told the commune's name once its PUBLIC lookup succeeded — only the shared app's QR path passes it, to put the
   * name on Zalo's own top bar (owner, 08/10/2026). Never called with an empty name, nor on a failed lookup: the
   * shell then keeps its own title rather than a guessed commune.
   */
  onCommuneShown?: (communeName: string) => void;
}) {
  const { ten_mien, lay_ten, getSceneLocation, pickScenePhotos, draftStore, openSession, openVideo, openLink } = props;
  const { onCommuneShown } = props;
  const [trang, datTrang] = useState<TrangTra>({ kieu: "dang-tra" });
  /** Mỗi lần bấm "Thử lại" tăng một — hiệu ứng tra chạy lại đúng một lần cho mỗi giá trị. */
  const [lan, datLan] = useState(0);
  // The Zalo name, asked for as below.
  const [name, setName] = useState<NameAtEntry>(() =>
    lay_ten === undefined ? { kind: "settled", name: null } : { kind: "checking" },
  );
  /**
   * The ref, not the effect's dependency list, is what makes the check run ONCE: StrictMode mounts, unmounts
   * and remounts in development, and each mount re-runs the effect. A second `getUserInfo` would be a second
   * platform call for the same answer — the same guard `useTinXa` uses for the same reason.
   */
  const nameChecked = useRef(false);

  // Started with the commune lookup, not after it: the check never opens a dialog, so running it while
  // the commune name loads costs the citizen nothing, and the name is usually ready by the first screen.
  useEffect(() => {
    if (lay_ten === undefined || nameChecked.current) return;
    nameChecked.current = true;
    void lay_ten("check")
      .catch((): Awaited<ReturnType<LayTenZalo>> => ({ kieu: "khong-lay-duoc" }))
      .then((kq) => setName(afterNameCheck(kq)));
  }, [lay_ten]);

  /**
   * Zalo's name dialog — from the home card's "Đồng ý", or from the send gate's "Đồng ý" right after the number
   * (`AppCuaXa`). AT MOST ONCE per open (owner, 09/10/2026): the ref holds even when two callers act in the same
   * tick, before `name` has re-rendered out of `needs-consent`.
   */
  const nameAsked = useRef(false);
  async function askName() {
    if (lay_ten === undefined || name.kind !== "needs-consent" || nameAsked.current) return;
    nameAsked.current = true;
    setName({ kind: "asking" });
    const kq = await lay_ten("ask").catch((): Awaited<ReturnType<LayTenZalo>> => ({ kieu: "khong-lay-duoc" }));
    setName(afterNameAsk(kq));
  }

  useEffect(() => {
    let con_song = true;
    void traXaTheoTenMien(ten_mien).then((kq) => {
      if (!con_song) return;
      const kq_xa = xaTuKetQuaTra(kq);
      datTrang("xa" in kq_xa ? { kieu: "xong", xa: kq_xa.xa } : { kieu: "loi", cau: kq_xa.loi });
    });
    return () => {
      con_song = false;
    };
    // Tên miền là hằng của bản dựng; chỉ `lan` đổi.
  }, [lan]);

  const communeName = trang.kieu === "xong" ? trang.xa.ten.trim() : "";
  useEffect(() => {
    if (communeName !== "") onCommuneShown?.(communeName);
  }, [communeName, onCommuneShown]);

  if (trang.kieu === "xong") {
    return (
      <AppCuaXa
        ten_mien={ten_mien}
        xa={trang.xa}
        name={name}
        onAgreeName={() => void askName()}
        onDeclineName={() => setName({ kind: "settled", name: null })}
        getSceneLocation={getSceneLocation}
        pickScenePhotos={pickScenePhotos}
        draftStore={draftStore}
        openSession={openSession}
        openVideo={openVideo}
        openLink={openLink}
      />
    );
  }

  return (
    <div className="xa-app">
      <div className="xa-trang xa-trang--con">
        {trang.kieu === "dang-tra" ? (
          <KhoiTrangThai bieu_tuong="build" cau={APP_RIENG.dang_mo} dang_tai shape="none" />
        ) : (
          <KhoiTrangThai
            bieu_tuong="alert"
            loi
            cau={trang.cau}
            nut={{
              nhan: APP_RIENG.thu_lai,
              onBam: () => {
                datTrang({ kieu: "dang-tra" });
                datLan((n) => n + 1);
              },
            }}
          />
        )}
      </div>
    </div>
  );
}
