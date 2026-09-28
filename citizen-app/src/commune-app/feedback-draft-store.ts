/**
 * THE ONLY PRODUCTION FILE IN THIS APP ALLOWED TO TOUCH DEVICE STORAGE — and only `localStorage`, one key.
 *
 * WHY IT EXISTS: ADR 0050 #7, owner decision 28/09/2026 ("3 điểm còn lại cũng theo require nhé"). The
 * requirements prototype keeps a feedback being written on the phone (`apps/miniapp/src/store/draft.ts`,
 * `NewFeedbackPage.tsx:124-136`), because losing what one typed — a phone call, a flat battery — is the
 * commonest reason a citizen gives up on reporting.
 *
 * WHY ONLY THE COMMUNE'S OWN APP: that app is a SEPARATE Zalo App ID (built with `--vao-thang`, so
 * `XA_CO_DINH !== null`), i.e. a separate origin, and it has no published privacy policy yet (ADR 0047).
 * The shared ViHAT app promises "không lưu gì xuống máy" (`content/chinh-sach-rieng-tu.ts`) and its two
 * halves share one origin — so it must stay storage-free. Three layers keep it so:
 *   1. only `AppRieng` in `App.tsx` passes this store down (a test reads `AppChung`'s body for it);
 *   2. the default store below opens NO storage when `XA_CO_DINH === null`, i.e. in the shared app build;
 *   3. `phase1-collects-nothing.test.ts` and `ranh-gioi-hai-nua.test.ts` §3b allow `localStorage` in THIS
 *      FILE ONLY; `sessionStorage`, cookies and IndexedDB stay banned here too.
 *
 * WHAT IS WRITTEN: exactly the six fields of `NhapPhieu`, rebuilt one by one (never a spread of the
 * caller's object, so a field added to the form later is not written silently). The name and phone typed
 * for the feedback are kept as the prototype keeps them — EXCEPT when "Gửi ẩn danh" is on: then they are
 * written empty, so an anonymous draft does not leave the citizen's identity on the phone. Cleared on a
 * successful send and on "Bỏ nháp" / "Huỷ bỏ". Never sent anywhere.
 *
 * Every call is wrapped: storage may be full, disabled, or throw on access. A draft is a convenience — it
 * must never block writing or sending a feedback.
 */
import type { FeedbackDraftStore, NhapPhieu } from "../cong-dan";
import { XA_CO_DINH } from "../lib/xa-co-dinh";

/** The single key. Versioned, like the prototype's, so a future shape change can ignore old drafts. */
export const FEEDBACK_DRAFT_KEY = "vigov.feedback.draft.v1";

export type KeyValueStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

/**
 * Nothing the citizen typed about the incident — no description, no place: not worth asking them to resume
 * (the prototype's rule: `loadDraft` drops a draft with no content). A field picked with one tap, or a name
 * that came from Zalo, alone is not a draft.
 */
function isEmptyDraft(draft: NhapPhieu): boolean {
  return draft.noi_dung.trim() === "" && draft.dia_chi.trim() === "";
}

/** The exact record written — six fields, rebuilt explicitly. */
function toStored(draft: NhapPhieu): NhapPhieu {
  const anonymous = draft.an_danh === true;
  return {
    linh_vuc: draft.linh_vuc,
    noi_dung: draft.noi_dung,
    dia_chi: draft.dia_chi,
    ho_ten: anonymous ? "" : draft.ho_ten,
    dien_thoai: anonymous ? "" : draft.dien_thoai,
    an_danh: anonymous,
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
export function createFeedbackDraftStore(openStorage: () => KeyValueStorage | null): FeedbackDraftStore {
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
        return s === null ? null : parseDraft(s.getItem(FEEDBACK_DRAFT_KEY));
      } catch {
        return null;
      }
    },
    save: (draft) => {
      try {
        const s = storage();
        if (s === null) return;
        if (isEmptyDraft(draft)) s.removeItem(FEEDBACK_DRAFT_KEY);
        else s.setItem(FEEDBACK_DRAFT_KEY, JSON.stringify(toStored(draft)));
      } catch {
        // Full or blocked storage. Writing and sending the feedback must go on.
      }
    },
    clear: () => {
      try {
        storage()?.removeItem(FEEDBACK_DRAFT_KEY);
      } catch {
        // Nothing more to do.
      }
    },
  };
}

/**
 * The store `AppRieng` injects. FAILS CLOSED: in a build without a fixed commune (the shared ViHAT app,
 * every test run) it opens no storage at all, so even a wrong wiring cannot make the shared app write.
 */
export const feedbackDraftStore: FeedbackDraftStore = createFeedbackDraftStore(() =>
  XA_CO_DINH === null ? null : (globalThis.localStorage ?? null),
);
