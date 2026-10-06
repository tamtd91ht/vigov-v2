/**
 * THE ONLY PRODUCTION FILE IN THIS APP ALLOWED TO TOUCH DEVICE STORAGE — and only `localStorage`, one key per
 * commune (see TWO APPS below).
 *
 * WHY IT EXISTS: ADR 0050 #7, owner decision 28/09/2026 ("3 điểm còn lại cũng theo require nhé"). The
 * requirements prototype keeps a feedback being written on the phone (`apps/miniapp/src/store/draft.ts`,
 * `NewFeedbackPage.tsx:124-136`), because losing what one typed — a phone call, a flat battery — is the
 * commonest reason a citizen gives up on reporting.
 *
 * TWO APPS, TWO STORES, ONE FILE.
 *   · The commune's OWN app (`--vao-thang`, `XA_CO_DINH !== null`, `AppRieng`) — a separate App ID, so a
 *     separate origin serving ONE commune: `feedbackDraftStore`, the single key `FEEDBACK_DRAFT_KEY`
 *     (unchanged since 28/09/2026, so drafts already saved on phones are still found).
 *   · The SHARED ViHAT app on the commune QR path (owner 06/10/2026: the commune apps cannot be published yet,
 *     so the shared app BORROWS the commune's full interface — "full nội dung trong citizen app"). One origin
 *     there serves EVERY commune, so a single key would offer commune A's draft — with the citizen's name and
 *     phone — inside commune B: a cross-commune leak (rule 1). `sharedAppDraftStore(host)` therefore keys by
 *     the commune host from the QR (`sharedAppDraftKey`), and refuses (no storage) without a valid host.
 * Each store fails closed in the OTHER build: the own-app store opens nothing when `XA_CO_DINH === null`, the
 * shared-app store opens nothing when `XA_CO_DINH !== null`. `phase1-collects-nothing.test.ts` and
 * `ranh-gioi-hai-nua.test.ts` §3b allow `localStorage` in THIS FILE ONLY; `sessionStorage`, cookies and
 * IndexedDB stay banned here too. `App.tsx` is the only importer.
 *
 * WHAT IS WRITTEN: exactly the six fields of `NhapPhieu`, rebuilt one by one (never a spread of the
 * caller's object, so a field added to the form later is not written silently). The name and phone typed
 * for the feedback are kept as the prototype keeps them, anonymous or not (see `toStored`). Cleared on a
 * successful send and on "Bỏ nháp" / "Huỷ bỏ". Never sent anywhere.
 *
 * Every call is wrapped: storage may be full, disabled, or throw on access. A draft is a convenience — it
 * must never block writing or sending a feedback.
 */
import type { FeedbackDraftStore, NhapPhieu } from "../cong-dan";
import { laTenMien } from "../lib/launch-params";
import { XA_CO_DINH } from "../lib/xa-co-dinh";

/**
 * The own app's single key. Versioned, like the prototype's, so a future shape change can ignore old drafts.
 * Never renamed: the drafts already saved on phones live under exactly this string.
 */
export const FEEDBACK_DRAFT_KEY = "vigov.feedback.draft.v1";

/**
 * The shared app's key for ONE commune: the own app's key plus the commune host from the QR, so two communes
 * opened on the same phone never see each other's draft. `null` when the host is not a well-formed domain —
 * no commune, no key, no storage (fail closed: rule 1, never a default on the isolation path).
 */
export function sharedAppDraftKey(communeHost: string): string | null {
  const host = communeHost.trim().toLowerCase();
  return laTenMien(host) ? `${FEEDBACK_DRAFT_KEY}:${host}` : null;
}

export type KeyValueStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

/**
 * Nothing the citizen typed about the incident — no description, no place: not worth asking them to resume
 * (the prototype's rule: `loadDraft` drops a draft with no content). A field picked with one tap, or a name
 * that came from Zalo, alone is not a draft.
 */
function isEmptyDraft(draft: NhapPhieu): boolean {
  return draft.noi_dung.trim() === "" && draft.dia_chi.trim() === "";
}

/**
 * The exact record written — six fields, rebuilt explicitly. Name and phone are kept even when "Gửi ẩn
 * danh" is on (owner 28/09/2026: "theo require, ẩn danh cũng lưu họ tên" — require's `saveDraft` writes
 * them unconditionally): anonymity is about what the STAFF see once sent, and switching it off again must
 * not make the citizen retype. What leaves the phone is still decided at send time (`taoPhieuTraiNghiem`).
 */
function toStored(draft: NhapPhieu): NhapPhieu {
  return {
    linh_vuc: draft.linh_vuc,
    noi_dung: draft.noi_dung,
    dia_chi: draft.dia_chi,
    ho_ten: draft.ho_ten,
    dien_thoai: draft.dien_thoai,
    an_danh: draft.an_danh === true,
  };
}

/** Read side: anything that is not exactly the stored shape is "no draft". */
function parseDraft(raw: string | null): NhapPhieu | null {
  if (raw === null || raw === "") return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof value !== "object" || value === null || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  const { linh_vuc, noi_dung, dia_chi, ho_ten, dien_thoai, an_danh } = record;
  if (
    typeof linh_vuc !== "string" ||
    typeof noi_dung !== "string" ||
    typeof dia_chi !== "string" ||
    typeof ho_ten !== "string" ||
    typeof dien_thoai !== "string" ||
    typeof an_danh !== "boolean"
  ) {
    return null;
  }
  const draft = toStored({ linh_vuc, noi_dung, dia_chi, ho_ten, dien_thoai, an_danh });
  return isEmptyDraft(draft) ? null : draft;
}

/**
 * A draft store over any key-value storage. `openStorage` returning `null` (or throwing) means "no
 * storage": every call becomes a no-op and `load` returns `null`. Tests pass a fake here.
 */
export function createFeedbackDraftStore(
  openStorage: () => KeyValueStorage | null,
  key: string = FEEDBACK_DRAFT_KEY,
): FeedbackDraftStore {
  const storage = (): KeyValueStorage | null => {
    try {
      return openStorage();
    } catch {
      return null;
    }
  };
  return {
    load: () => {
      try {
        const s = storage();
        return s === null ? null : parseDraft(s.getItem(key));
      } catch {
        return null;
      }
    },
    save: (draft) => {
      try {
        const s = storage();
        if (s === null) return;
        if (isEmptyDraft(draft)) s.removeItem(key);
        else s.setItem(key, JSON.stringify(toStored(draft)));
      } catch {
        // Full or blocked storage. Writing and sending the feedback must go on.
      }
    },
    clear: () => {
      try {
        storage()?.removeItem(key);
      } catch {
        // Nothing more to do.
      }
    },
  };
}

/**
 * The store `AppRieng` injects. FAILS CLOSED: in a build without a fixed commune (the shared ViHAT app,
 * every test run) it opens no storage at all — the shared app must use `sharedAppDraftStore`, whose key
 * names the commune, never this single key.
 */
export const feedbackDraftStore: FeedbackDraftStore = createFeedbackDraftStore(() =>
  XA_CO_DINH === null ? null : (globalThis.localStorage ?? null),
);

/**
 * The store the shared app injects on the commune QR path, keyed by that commune's host. FAILS CLOSED twice:
 * a malformed host gets no key (every call a no-op), and in a commune's own build (`XA_CO_DINH !== null`) it
 * opens nothing — that build has its own store and its own key.
 */
export function sharedAppDraftStore(communeHost: string): FeedbackDraftStore {
  const key = sharedAppDraftKey(communeHost);
  if (key === null) return createFeedbackDraftStore(() => null);
  return createFeedbackDraftStore(() => (XA_CO_DINH !== null ? null : (globalThis.localStorage ?? null)), key);
}
