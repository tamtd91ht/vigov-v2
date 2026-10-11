/**
 * CẢNH BÁO THIÊN TAI — the home banner and the alert screen (ADR 0091 §2; the prototype's `AlertBanner.tsx` and
 * `AlertPage.tsx`). Public by the commune's domain (`communeDisasterAlerts`), no session.
 *
 * Built to be read in bad conditions, as the prototype says: rain on the screen, wet hands, someone frightened.
 * Large text, the 16px floor, call buttons at least 52px high, no decoration that costs a frame.
 *
 * NEVER STALE AS CURRENT: each home open and each screen open reads afresh, and nothing is shown until that read
 * answers — never the previous answer while a new one is pending. NO POLLING. A failure on home hides the banner
 * silently (the home screen works without it); on the screen it says so, with "Thử lại".
 *
 * DEPARTURES FROM THE PROTOTYPE, each a rule of ADR 0091 §2: no "Cách … km" (it needs the resident's location), no
 * "you are here" dot on the map (same reason), no promise of a Zalo message in the empty state (no ZNS push yet).
 * Several alerts in force are all shown, newest first (the prototype shows one).
 *
 * ⚠ The phones may be an officer's own (comms 0026): shown, dialled through `tel:`, never logged or sent anywhere.
 */
import { useEffect, useRef, useState } from "react";

import { communeDisasterAlerts, type NewsReadResult } from "../api/goi-vigov";
import type { DangerZonePoint, DisasterAlert, EmergencyContact, EvacuationPoint } from "../api/hop-dong-cong-khai";
import { formatDateTime } from "../../lib/vn-format";

import { BieuTuong } from "./BieuTuong";
import { dichGoi } from "./DanhBaCanBoScreen"; // vi-name-ok: existing export, reused (rule 12 #3)
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa"; // vi-name-ok: existing components (rule 12 #3)
import { COMMUNE_NEWS_RATE_LIMIT, TIN_XA, XA_TN } from "./noi-dung"; // vi-name-ok: existing string tables
import { errorSentence, type LoiTin } from "./TinTucXaScreen"; // vi-name-ok: existing type, reused (rule 12 #3)

/* ═══════════════════════════════ LOAD ═══════════════════════════════ */

export type DisasterAlertsState =
  | { readonly kind: "loading" }
  | { readonly kind: "ready"; readonly items: readonly DisasterAlert[] }
  // vi-name-ok: `LoiTin` is the existing news-error type the shared sentence function takes
  | { readonly kind: "failed"; readonly error: LoiTin; readonly retryAfterSeconds: number | null };

/** The wait before the ONE retry after a 429 with no `Retry-After`, and the cap on a long one — the banner strip's. */
const RETRY_DEFAULT_SECONDS = 5;
const RETRY_MAX_SECONDS = 60;

/**
 * One read, with EXACTLY ONE retry after a 429 (the home banner strip's rule, `TrangXa.tsx` `loadBanners`): after the
 * server's `Retry-After`, capped at 60 s, 5 s when absent. A second 429 — or any other failure — is the answer.
 * PURE apart from `load` and `wait`, which the caller injects.
 */
export async function loadDisasterAlerts(
  load: () => Promise<NewsReadResult<readonly DisasterAlert[]>>,
  wait: (ms: number) => Promise<void>,
): Promise<DisasterAlertsState> {
  let result = await load();
  if (result.kieu === "rate-limited") {
    const seconds = result.retryAfterSeconds === null ? RETRY_DEFAULT_SECONDS : result.retryAfterSeconds;
    await wait(Math.min(Math.max(seconds, 1), RETRY_MAX_SECONDS) * 1000);
    result = await load();
  }
  if (result.kieu === "xong") return { kind: "ready", items: result.gia_tri };
  if (result.kieu === "rate-limited") return { kind: "failed", error: "rate-limited", retryAfterSeconds: result.retryAfterSeconds };
  if (result.kieu === "loi-mang") return { kind: "failed", error: "loi-mang", retryAfterSeconds: null };
  if (result.kieu === "khong-hop-le") return { kind: "failed", error: "khong-hop-le", retryAfterSeconds: null };
  return { kind: "failed", error: "loi-may-chu", retryAfterSeconds: null };
}

/**
 * The alerts as read by THIS mount — `loading` until the read answers. Read once per mount (the ref guards
 * StrictMode's double run, as `useCommuneBanners` does); `reload` reads again ("Thử lại"). An answer that arrives
 * after the screen is gone is dropped.
 */
export function useDisasterAlerts(domain: string): { state: DisasterAlertsState; reload: () => void } {
  const [state, setState] = useState<DisasterAlertsState>({ kind: "loading" });
  const started = useRef(false);
  const alive = useRef(true);
  const run = () => {
    setState({ kind: "loading" });
    void loadDisasterAlerts(
      () => communeDisasterAlerts(domain),
      (ms) => new Promise((done) => setTimeout(done, ms)),
    ).then((s) => {
      if (alive.current) setState(s);
    });
  };
  useEffect(() => {
    alive.current = true;
    if (!started.current) {
      started.current = true;
      run();
    }
    return () => {
      alive.current = false;
    };
    // The domain is the build's constant for the life of the app: one read per mount.
  }, []);
  return { state, reload: run };
}

/* ═══════════════════════════════ HOME BANNER ═══════════════════════════════ */

/**
 * The red band at the TOP of home (`AlertBanner.tsx`), above the picture strip — "the one thing a resident needs to
 * see on a storm day". Only when at least one alert is in force; nothing while loading or after a failure. It shows
 * the NEWEST alert's headline (the server falls back to the title); a tap opens the screen, which lists them all.
 * Its accessible name starts with "Cảnh báo thiên tai", so the warning is said in words, not by the red alone. PURE.
 */
export function HomeAlertBanner(props: { state: DisasterAlertsState; onOpen: () => void }) {
  if (props.state.kind !== "ready" || props.state.items.length === 0) return null;
  const newest = props.state.items[0]!;
  const headline = newest.headline !== "" ? newest.headline : newest.title;
  return (
    <div className="xa-alert-banner-wrap">
      <button type="button" className="xa-alert-banner" aria-label={XA_TN.alert_banner_label(headline)} onClick={props.onOpen}>
        <BieuTuong ten="cloud-rain" co={30} stroke={2.2} />
        <span className="xa-alert-banner__text">{headline}</span>
        <BieuTuong ten="chevron-right" co={24} />
      </button>
    </div>
  );
}

/* ═══════════════════════════════ SCREEN ═══════════════════════════════ */

/** The kinds whose danger zone is a FLOOD zone — the prototype's wording applies to these only. */
const FLOOD_KINDS: ReadonlySet<string> = new Set(["bao", "lu", "mua-lon"]);

/** Heading, caption and map label of the zone block for this kind. PURE, exported for tests. */
export function zoneWords(kind: string, commune: string): { title: string; caption: string; map: string } {
  return FLOOD_KINDS.has(kind)
    ? { title: XA_TN.alert_zone_flood, caption: XA_TN.alert_zone_flood_caption, map: XA_TN.alert_zone_flood_map(commune) }
    : { title: XA_TN.alert_zone_other, caption: XA_TN.alert_zone_other_caption, map: XA_TN.alert_zone_other_map(commune) };
}

/**
 * The danger zone drawn as SVG over a plain grid (`AlertPage.tsx:164-199`): no map library, so it draws even on a
 * failing line in the middle of a storm. The polygon is the commune's drawing on a 0..100 grid — an illustration,
 * not coordinates (ADR 0091 §2). JSX elements only, never an SVG string.
 */
export function DangerZoneMap(props: { zone: readonly DangerZonePoint[]; label: string }) {
  const points = props.zone.map((p) => `${p.x},${p.y}`).join(" ");
  return (
    <div className="xa-alert-map__canvas">
      <svg viewBox="0 0 100 100" className="xa-alert-map__svg" role="img" aria-label={props.label} focusable="false">
        <rect width="100" height="100" fill="#E8F0F4" />
        <path d="M0 68 H100" stroke="#CBD9E2" strokeWidth="5" />
        <path d="M0 30 H100" stroke="#D8E4EC" strokeWidth="3" />
        <path d="M34 0 V100" stroke="#CBD9E2" strokeWidth="4" />
        <path d="M70 0 V100" stroke="#D8E4EC" strokeWidth="2.5" />
        <polygon className="xa-alert-map__zone" points={points} strokeWidth="1.6" strokeDasharray="3 2" />
      </svg>
    </div>
  );
}

/**
 * "GỌI {số}" — a `tel:` link (the dialler opens; nothing goes over the network), 52px high, the number in the words.
 * Nothing when the number has nothing to dial: a button that dials nothing on a storm day is worse than none.
 */
function CallButton(props: { phone: string; who: string }) {
  const dial = dichGoi(props.phone);
  if (dial === null) return null;
  return (
    <a className="xa-call-button" href={dial} aria-label={XA_TN.alert_call_label(props.who, props.phone)}>
      <BieuTuong ten="phone" co={22} stroke={2.4} />
      {XA_TN.alert_call(props.phone)}
    </a>
  );
}

function EvacuationCard({ point }: { point: EvacuationPoint }) {
  return (
    <div className="xa-the xa-alert-card">
      <h4 className="xa-alert-card__name">{point.name}</h4>
      {point.address !== "" && <p className="xa-phu">{point.address}</p>}
      {point.capacity !== null && <p className="xa-phu">{XA_TN.alert_capacity(point.capacity)}</p>}
      <CallButton phone={point.phone} who={point.name} />
    </div>
  );
}

function ContactCard({ contact }: { contact: EmergencyContact }) {
  return (
    <div className="xa-the xa-alert-card">
      <h4 className="xa-alert-card__name">{contact.role}</h4>
      {contact.name !== "" && <p className="xa-phu">{contact.name}</p>}
      <CallButton phone={contact.phone} who={contact.role} />
    </div>
  );
}

/**
 * ONE alert (`AlertPage.tsx:47-134`): the red frame (title, level, "Có hiệu lực từ:" / "Đến:", detail), the zone
 * when one was drawn, the evacuation points, the emergency numbers. A block with nothing in it is left out — an empty
 * "Điểm sơ tán" heading would read as "there is nowhere to go". PURE.
 */
export function DisasterAlertCard({ alert, communeName }: { alert: DisasterAlert; communeName: string }) {
  const from = formatDateTime(alert.effectiveFrom);
  const to = alert.effectiveTo === undefined ? null : formatDateTime(alert.effectiveTo);
  const zone = zoneWords(alert.kind, communeName);
  const titleId = `xa-alert-${alert.id}`;
  return (
    <article className="xa-alert" aria-labelledby={titleId}>
      <section className="xa-alert__frame">
        <div className="xa-alert__head">
          <BieuTuong ten="alert" co={32} stroke={2.3} />
          <div>
            <h2 className="xa-alert__title" id={titleId}>
              {alert.title}
            </h2>
            {alert.level !== "" && <p className="xa-alert__level">{alert.level}</p>}
          </div>
        </div>
        <dl className="xa-alert__times">
          {from !== null && (
            <div>
              <dt>{XA_TN.alert_from}</dt>
              <dd>{from}</dd>
            </div>
          )}
          {to !== null && (
            <div>
              <dt>{XA_TN.alert_to}</dt>
              <dd>{to}</dd>
            </div>
          )}
        </dl>
        {alert.detail !== "" && <p className="xa-alert__detail xa-giu-dong">{alert.detail}</p>}
      </section>

      {alert.dangerZone.length >= 3 && (
        <section className="xa-the xa-alert-map">
          <h3 className="xa-alert-map__title">{zone.title}</h3>
          <DangerZoneMap zone={alert.dangerZone} label={zone.map} />
          <p className="xa-phu xa-alert-map__caption">{zone.caption}</p>
        </section>
      )}

      {alert.evacuationPoints.length > 0 && (
        <section className="xa-alert-list">
          <h3 className="xa-alert-list__title">{XA_TN.alert_evacuation}</h3>
          {alert.evacuationPoints.map((p, i) => (
            <EvacuationCard key={`${i}-${p.name}`} point={p} />
          ))}
        </section>
      )}

      {alert.emergencyContacts.length > 0 && (
        <section className="xa-alert-list">
          <h3 className="xa-alert-list__title">{XA_TN.alert_contacts}</h3>
          {alert.emergencyContacts.map((c, i) => (
            <ContactCard key={`${i}-${c.role}`} contact={c} />
          ))}
        </section>
      )}
    </article>
  );
}

/** What the screen shows for a state: words while loading, the sentence and "Thử lại" on failure, the empty state, or every alert. PURE. */
export function DisasterAlertBody(props: { state: DisasterAlertsState; communeName: string; onRetry: () => void }) {
  const { state } = props;
  if (state.kind === "loading") return <KhoiTrangThai bieu_tuong="cloud-rain" cau={XA_TN.alert_loading} dang_tai shape="card" />;
  if (state.kind === "failed") {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={errorSentence(state.error, state.retryAfterSeconds, COMMUNE_NEWS_RATE_LIMIT)}
        nut={state.error !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onRetry } : undefined}
      />
    );
  }
  if (state.items.length === 0) {
    return <KhoiTrangThai bieu_tuong="cloud-rain" cau={XA_TN.alert_empty} hint={XA_TN.alert_empty_hint(props.communeName)} />;
  }
  return (
    <div className="xa-alerts">
      {state.items.map((a) => (
        <DisasterAlertCard key={a.id} alert={a} communeName={props.communeName} />
      ))}
    </div>
  );
}

/** The screen: title "Cảnh báo thiên tai", back, and a fresh read of the alerts in force. */
export function DisasterAlertScreen(props: { domain: string; communeName: string; onBack: () => void }) {
  const { state, reload } = useDisasterAlerts(props.domain);
  return (
    <>
      <DauManCon tieu_de={XA_TN.alert_title} onQuayLai={props.onBack} />
      <TrangCon>
        <DisasterAlertBody state={state} communeName={props.communeName} onRetry={reload} />
      </TrangCon>
    </>
  );
}
