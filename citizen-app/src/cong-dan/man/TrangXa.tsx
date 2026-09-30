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
 * trữ; lớp vỏ tiêm `draftStore` (chỉ `AppRieng`), tệp duy nhất chạm kho là `commune-app/feedback-draft-store.ts`.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: router, lưu trữ trực tiếp, tên xã từ biến môi trường, lớp gọi máy chủ của nó, OTP,
 * quét căn cước, số ngày cam kết viết cứng (luật 10). Bước chọn lĩnh vực CÓ, và lĩnh vực ấy là của phiếu (ADR 0050, thay ADR 0049).
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP (ADR 0047 §6). Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log.
 */
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { communeProfiles, type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import type { CommuneProfile, XaTraDuoc } from "../api/hop-dong-cong-khai";
import { communeAppReopen, dropCommuneAppSession, type OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { layPhienViGov } from "../api/phien-vigov";
import { DEMO_BUILD } from "../../lib/demo-build";

import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import {
  createSessionGate,
  type SessionGate,
  sessionGateMessage,
  sessionGateOffersRetry,
  type SessionGateState,
} from "./commune-session";
import { dichGoi } from "./DanhBaCanBoScreen";
import { DemoBand, DEMO_WORDS, withDemoName } from "./demo-mode";
import { ThanDanhBaXa, useDanhBaXa } from "./DanhBaXa";
import { DauManCon, KhoiTrangThai, RootTabHeader, SectionHeader, type SectionAccent, type Tone, TrangCon } from "./khung-xa";
import {
  APP_RIENG,
  COMMUNE_APP_SESSION,
  COMMUNE_OFFICE,
  CUA_TOI,
  DANH_BA,
  PHONE_VERIFICATION,
  type PhoneVerificationTask,
  TIN_XA,
  XA_GIAO_DIEN,
  XA_PA,
  XA_TN,
  zaloFailureSentence,
  zaloSupportCode,
} from "./noi-dung";
import {
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
import { BaiTinXa, DanhSachTinXa, HangTin, NewsOfType, useTinXa } from "./TinTucAppXa";
import type { GetSceneLocation } from "./scene-location";
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
  | { readonly kieu: "truyen-thanh" }
  | { readonly kieu: "video" }
  | { readonly kieu: "ban-do" }
  | { readonly kieu: "thong-bao" }
  | { readonly kieu: "su-kien" };

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
 * THE HOME BANNER — one rounded picture under the header, as the prototype's `BannerStrip` but with no
 * carousel and no autoplay (a strip that moves on its own is the hardest thing on a touch screen for a slow
 * reader, and there is one picture).
 *
 * TEMPORARY, the same per-domain bundle exception as the logo (ADR 0047 §6): `deploy.mjs --vao-thang` copies
 * `scripts/banner-xa/<domain>.png` into THAT build only, as `./banner-xa.png`. The real source later is the
 * commune-posted `banner` content / display profile in service-platform, read at runtime — then this file
 * name goes away.
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
const NHOM_CHUC_NANG: ReadonlyArray<{ tieu_de: string; vach: SectionAccent; them?: { nhan: string; man: ManXa }; o: readonly OMenu[] }> = [
  {
    tieu_de: XA_TN.nhom_chinh_quyen,
    // `.xa-dau-nhom` without a modifier IS the brand bar; `--brand` names it for the reader, no rule needed.
    vach: "brand",
    o: [
      { nhan: XA_GIAO_DIEN.o_gui, bieu_tuong: "megaphone", mau: "red", man: { kieu: "gui" } },
      { nhan: XA_TN.o_tra_cuu_ngan, bieu_tuong: "search", mau: "xanh", man: { kieu: "tra-cuu" } },
      { nhan: XA_GIAO_DIEN.o_danh_ba, bieu_tuong: "phone", mau: "luc", man: { kieu: "danh-ba" } },
      // SRS M6.1.12 (P1). The screen had no way in, so it read as "done" to anyone reading the code.
      { nhan: XA_TN.o_ban_do, bieu_tuong: "map", mau: "cam", man: { kieu: "ban-do" } },
    ],
  },
  {
    tieu_de: XA_TN.nhom_thong_tin,
    vach: "cam",
    them: { nhan: XA_TN.xem_them, man: { kieu: "tab", tab: "tin-tuc" } },
    o: [
      { nhan: XA_TN.o_tin_tuc, bieu_tuong: "news", mau: "xanh", man: { kieu: "tab", tab: "tin-tuc" } },
      { nhan: XA_TN.o_truyen_thanh, bieu_tuong: "radio", mau: "luc", man: { kieu: "truyen-thanh" } },
      { nhan: XA_TN.o_video, bieu_tuong: "play", mau: "cam", man: { kieu: "video" } },
      { nhan: XA_TN.o_su_kien, bieu_tuong: "clock", mau: "tim", man: { kieu: "su-kien" } },
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
  di: (m: ManXa) => void;
}) {
  const { xa, tin, di, ho_ten, petitions } = props;
  // Hai phiếu, hai tin — đúng bố cục trang chủ của prototype (`HomePage.tsx`).
  const tin_moi = tin.ds.muc.slice(0, 2);
  const phieu_moi = petitions.kind === "ready" ? petitions.items.slice(0, 2) : [];

  return (
    <>
      {/* Header theo prototype (`AppHeader.tsx`): logo xã trái, tên xã giữa, lời chào dưới. Nó đứng NGOÀI vùng
          cuộn — chỉ phần giữa cuộn (§5.2). Chuông giữ chỗ cũ tới khi có quyết định (chủ dự án, 30/09/2026). */}
      <RootTabHeader
        title={xa.ten}
        subtitle={ho_ten !== null ? XA_TN.xin_chao_ten(ho_ten) : xa.tinh}
        right={
          <button type="button" className="xa-hero__chuong" onClick={() => di({ kieu: "thong-bao" })} aria-label={XA_TN.thong_bao}>
            <BieuTuong ten="bell" co={22} />
          </button>
        }
      />
      <div className="xa-trang">
        <CommuneBanner />

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
              <KhoiTrangThai bieu_tuong="chat" cau={XA_TN.chua_co_phieu} />
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
              <KhoiTrangThai bieu_tuong="news" cau={XA_GIAO_DIEN.tin_moi_trong} />
            ) : (
              <ul className="xa-ds">
                {tin_moi.map((t) => (
                  <li key={t.id}>
                    <HangTin tin={t} onMo={(id) => di({ kieu: "bai", id, tu: "trang-chu" })} />
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
function DauTab({ tieu_de }: { tieu_de: string }) {
  return <RootTabHeader title={tieu_de} />;
}

/**
 * The gate, as a screen of its own — one task per screen (`skills/accessibility-elderly` #4). The screen it
 * covers stays MOUNTED underneath (hidden), so a half-written petition survives the detour.
 *
 *   `hoi`       WHY first, then what Zalo will ask, then the button — Zalo's dialog opens only on "Đồng ý"
 *               (policy 3.3.4; the words are `PHONE_VERIFICATION`'s, the same act as the shared app's)
 *   `dang-mo`   words, not a spinner
 *   `ket-qua`   one sentence saying what to do next; "Đồng ý…" again only where a new tap can help
 *   `demo`      demo build only: the act ran without a session and the server was not reached — one sentence
 *
 * Exported for `demo-mode.test.tsx` (rendered statically); `AppCuaXa` is its only caller.
 */
export function SessionGateScreen(props: {
  state: SessionGateState;
  task: PhoneVerificationTask;
  onAllow: () => void;
  onDecline: () => void;
  onClose: () => void;
}) {
  const { state } = props;
  return (
    <>
      <DauManCon tieu_de={DEMO_BUILD && state.kieu === "demo" ? DEMO_WORDS.notice_title : PHONE_VERIFICATION.title} onQuayLai={props.onClose} />
      <TrangCon>
        {/* Demo build only: ONE sentence and the way back — no "Đồng ý…" again, so no loop (`showDemoNotice`). */}
        {DEMO_BUILD && state.kieu === "demo" && (
          <>
            <KhoiTrangThai bieu_tuong="info" cau={DEMO_WORDS.task[props.task]} />
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onClose}>
              {DEMO_WORDS.back}
            </button>
          </>
        )}
        {state.kieu === "dang-mo" && <KhoiTrangThai bieu_tuong="user" cau={COMMUNE_APP_SESSION.working} dang_tai />}
        {state.kieu === "hoi" && (
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
        {state.kieu === "ket-qua" && (
          <>
            <KhoiTrangThai
              bieu_tuong="alert"
              loi
              cau={sessionGateMessage(state.outcome, props.task, state.zalo)}
              support_code={state.outcome === "thu-lai" ? zaloSupportCode(state.zalo) : null}
              nut={sessionGateOffersRetry(state.outcome) ? { nhan: PHONE_VERIFICATION.allow, onBam: props.onAllow } : undefined}
            />
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
  /** Demo build only: a phone-step failure let this open's acts run without a session (`commune-session.ts`). */
  const [demoWithoutSession, setDemoWithoutSession] = useState(false);
  const name = useRef(communeName);
  name.current = communeName;
  const gate = useRef<SessionGate | null>(null);
  if (gate.current === null) {
    gate.current = createSessionGate(
      open,
      () => name.current,
      setState,
      DEMO_BUILD ? { onProceed: () => setDemoWithoutSession(true) } : undefined,
    );
  }
  return { gate: gate.current, state, demoWithoutSession: DEMO_BUILD && demoWithoutSession };
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
  draftStore?: FeedbackDraftStore;
  openSession?: OpenCommuneAppSession;
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
  const veTab = (tab: TabXa) => datMan({ kieu: "tab", tab });
  const lop = `xa-app xa-co-chu--${co_chu}`;

  /* ── THE GATE: every personal act asks here first (`commune-session.ts`) ── */
  const { gate, state: gateState, demoWithoutSession } = useSessionGate(openSession, xa.ten);
  const [gateTask, setGateTask] = useState<PhoneVerificationTask>("submit");
  // The 403 path of the rating block (`usePhoneVerification`): the same opener, the same commune check.
  const reopenWithPhone = useMemo(() => (openSession ? communeAppReopen(openSession) : undefined), [openSession]);

  /** Run `act` with a session — at once if one exists, otherwise after the explanation and the tap. */
  function requireSession(task: PhoneVerificationTask, act: () => void) {
    setGateTask(task);
    gate.require(() => {
      act();
      // A session now exists: the home block and Cá nhân fill in without a second question. (Demo build
      // without a session: nothing to load, and loading would cover the screen just asked for with a notice.)
      if (!DEMO_BUILD || layPhienViGov() !== null) petitions.loadIfIdle();
    });
  }

  /** 401 / 403 `chua_xac_thuc_so`: forget the session, and run the act again through the gate. */
  const sessionLost =
    (task: PhoneVerificationTask): OnSessionLost =>
    (retry) => {
      // Demo build, no session because Zalo refused the phone: one sentence, and NOT the gate again — the
      // gate would run the act, the act would come back here, and that is the loop the demo must not have.
      if (DEMO_BUILD && gate.demoWithoutSession() && layPhienViGov() === null) {
        setGateTask(task);
        gate.showDemoNotice();
        return;
      }
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
        />
      );
      break;
    case "gui":
      man_con = (
        <CommuneSendScreen
          ten_xa={xa.ten}
          ho_ten={shownName}
          demoWithoutSession={demoWithoutSession}
          getSceneLocation={props.getSceneLocation}
          draftStore={props.draftStore}
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
        />
      );
      break;
    case "danh-ba":
      man_con = <ManDanhBa ten_mien={ten_mien} onQuayLai={ve} />;
      break;
    case "tra-cuu":
      man_con = <TraCuuHoSoXa onQuayLai={ve} />;
      break;
    case "tra-cuu-phieu":
      man_con = (
        <PetitionLookup
          onBack={() => veTab("phan-anh")}
          onSessionLost={sessionLost("lookup")}
          onChanged={() => void petitions.load()}
          reopenWithPhone={reopenWithPhone}
        />
      );
      break;
    case "truyen-thanh":
      man_con = (
        <ManChuaCoDuLieu tieu_de={XA_TN.truyen_thanh_tieu_de} bieu_tuong="radio" cau={XA_TN.truyen_thanh_trong} onQuayLai={ve} />
      );
      break;
    case "video":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.video_tieu_de} bieu_tuong="play" cau={XA_TN.video_trong} onQuayLai={ve} />;
      break;
    case "ban-do":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.ban_do_tieu_de} bieu_tuong="map" cau={XA_TN.ban_do_trong} onQuayLai={ve} />;
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
            />
          </TrangCon>
        </>
      );
      break;
    case "thong-bao":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.thong_bao} bieu_tuong="bell" cau={XA_TN.thong_bao_trong} onQuayLai={ve} />;
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
  // THE SCROLL MODEL (`PROTOTYPE.md` §5.2): `.xa-app` is one viewport-tall column; each `.xa-frame` is the
  // rest of it, holding a header, ONE scrolling `.xa-trang`, and any footer / tab bar — so only the middle
  // moves, and the demo band, header and bottom bar stay on screen. `hidden` on a frame must still hide it:
  // `.xa-frame[hidden]` in styles.css, since `display: flex` would otherwise win over the attribute.
  const withGate = (content: ReactNode) => (
    <div className={lop}>
      <DemoBand />
      {gateScreen !== null && <div className="xa-frame">{gateScreen}</div>}
      <div className="xa-frame" hidden={gateScreen !== null}>
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
        di={go}
      />
    );
  } else if (tab === "tin-tuc") {
    than = (
      <>
        <DauTab tieu_de={XA_TN.news_tab_title} />
        <div className="xa-trang xa-trang--tab">
          <DanhSachTinXa ten_mien={ten_mien} onMo={(id) => datMan({ kieu: "bai", id, tu: "tin-tuc" })} />
        </div>
      </>
    );
  } else if (tab === "phan-anh") {
    than = (
      <>
        <DauTab tieu_de={CUA_TOI.tieu_de} />
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
        <DauTab tieu_de={XA_TN.ca_nhan_tieu_de} />
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
        />
      </>
    );
  }

  return withGate(
    <>
      <main id="main">{than}</main>
      <ThanhTabXa tab={tab} onChon={veTab} />
    </>,
  );
}

type TrangTra =
  | { readonly kieu: "dang-tra" }
  | { readonly kieu: "loi"; readonly cau: string }
  | { readonly kieu: "xong"; readonly xa: XaCuaApp };

export function TrangXa(props: {
  /** Tên miền xã nung vào bản dựng (`lib/xa-co-dinh.ts`). */
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
  const { ten_mien, lay_ten, getSceneLocation, draftStore, openSession } = props;
  const [trang, datTrang] = useState<TrangTra>({ kieu: "dang-tra" });
  /** Mỗi lần bấm "Thử lại" tăng một — hiệu ứng tra chạy lại đúng một lần cho mỗi giá trị. */
  const [lan, datLan] = useState(0);
  // `withDemoName`: the demo build's sample name where Zalo gave none — identity in every other build.
  const [name, setName] = useState<NameAtEntry>(() =>
    withDemoName(lay_ten === undefined ? { kind: "settled", name: null } : { kind: "checking" }),
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
      .then((kq) => setName(withDemoName(afterNameCheck(kq))));
  }, [lay_ten]);

  async function askName() {
    if (lay_ten === undefined || name.kind !== "needs-consent") return;
    setName({ kind: "asking" });
    const kq = await lay_ten("ask").catch((): Awaited<ReturnType<LayTenZalo>> => ({ kieu: "khong-lay-duoc" }));
    setName(withDemoName(afterNameAsk(kq)));
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

  if (trang.kieu === "xong") {
    return (
      <AppCuaXa
        ten_mien={ten_mien}
        xa={trang.xa}
        name={name}
        onAgreeName={() => void askName()}
        onDeclineName={() => setName(withDemoName({ kind: "settled", name: null }))}
        getSceneLocation={getSceneLocation}
        draftStore={draftStore}
        openSession={openSession}
      />
    );
  }

  return (
    <div className="xa-app">
      <DemoBand />
      <div className="xa-trang xa-trang--con">
        {trang.kieu === "dang-tra" ? (
          <KhoiTrangThai bieu_tuong="build" cau={APP_RIENG.dang_mo} dang_tai />
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
