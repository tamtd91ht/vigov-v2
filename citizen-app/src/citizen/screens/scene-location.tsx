/**
 * "LẤY VỊ TRÍ HIỆN TẠI" — ONE piece for both send screens: the live form (`SubmitReportScreen`, shared app)
 * and the commune app's experience form (`PhanAnhAppXa`). Only the words and the look differ ("bạn" vs
 * "bà con", ADR 0050 #6; `cd-*` vs `xa-*` classes).
 *
 * The state half calls no `zmp-sdk` and no `vihat-miniapp` (`two-halves-boundary.test.ts` §3a): the shell
 * injects `GetSceneLocation` (`App.tsx`), which gets the Zalo codes and exchanges them at once.
 *
 * WHAT THE CITIZEN SEES follows the requirements prototype (`../vigov-require/apps/miniapp/src/
 * components/feedback/AddressBlock.tsx:105-124`): after a success, one line "Đã lấy vị trí hiện tại
 * (lat, lng)" with five decimals, and the button becomes "Lấy lại vị trí hiện tại". NEVER an address
 * guessed from the point: the address box stays exactly what the citizen typed, editable, and the
 * coordinates are shown beside it, not written into it (the prototype's reason: officers need "trước số
 * nhà 112", which only the person on the spot can say).
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS COORDINATES IN A URL OR A KEY (rule 3). They live in the form
 *   state of the screen that asked for them; the commune app's DRAFT does not carry them (its store
 *   writes the six `ReportDraft` fields, and the prototype's `saveDraft` keeps no `coords` either).
 */
import { useState } from "react";

import { isSceneLocation, type SceneLocation } from "../api/citizen-report-contract";

import { Icon } from "./Icon";

/**
 * Why there is no location — one branch per thing the citizen does next:
 *
 *   `tu-choi`         they said no on Zalo's dialog → nothing to fix; type the address
 *   `ngoai-zalo`      opened outside Zalo → only works inside Zalo
 *   `qua-nhieu-lan`   the server's per-IP limit → wait a few minutes, or type the address
 *   `thu-lai`         Zalo or the network did not answer, a code was missing → press again, or type it
 *   `tam-ngung`       the exchange is not available on this build / server → pressing again changes nothing
 */
export type SceneLocationFailure = "tu-choi" | "ngoai-zalo" | "qua-nhieu-lan" | "thu-lai" | "tam-ngung";

export type SceneLocationResult =
  | { readonly kind: "xong"; readonly location: SceneLocation }
  | { readonly kind: SceneLocationFailure };

/** Injected by the shell. Resolves to one of the branches above; the caller also guards a rejection. */
export type GetSceneLocation = () => Promise<SceneLocationResult>;

/** The client's own sentences (never the server's `message` — `vigov-client.ts` rule). */
export type SceneLocationWords = {
  readonly button: string;
  readonly button_again: string;
  readonly locating: string;
  readonly why: string;
  readonly found: (coordinates: string) => string;
  readonly failures: Readonly<Record<SceneLocationFailure, string>>;
};

/** "16.12346, 108.12346" — five decimals, as the prototype shows (about one metre; enough to recognise). */
export function formatCoordinates(location: SceneLocation): string {
  return `${location.lat.toFixed(5)}, ${location.lng.toFixed(5)}`;
}

/**
 * One tap → the location to keep, or the failure to say. PURE apart from calling `get`.
 *
 * A rejected promise and a "success" whose pair is not a valid location both become `thu-lai`: a half or
 * out-of-range pair is never kept, so it can never reach the petition body.
 */
export async function locateOnce(
  get: GetSceneLocation,
): Promise<{ location: SceneLocation | null; failure: SceneLocationFailure | null }> {
  let result: SceneLocationResult;
  try {
    result = await get();
  } catch {
    return { location: null, failure: "thu-lai" };
  }
  if (result.kind === "xong") {
    return isSceneLocation(result.location)
      ? { location: { lat: result.location.lat, lng: result.location.lng }, failure: null }
      : { location: null, failure: "thu-lai" };
  }
  return { location: null, failure: result.kind };
}

/**
 * Tap handling for a screen that keeps the location in its own form state (`onLocation`).
 *
 * The previous location is CLEARED when a new tap starts, as the prototype does (`NewFeedbackPage.tsx:
 * 157`): what the screen shows is always what would be sent, and a failed retry does not leave an older
 * point attached without the citizen knowing. A second tap while one runs is ignored (the button is
 * disabled too).
 */
export function useSceneLocation(get: GetSceneLocation | undefined, onLocation: (l: SceneLocation | null) => void) {
  const [locating, setLocating] = useState(false);
  const [failure, setFailure] = useState<SceneLocationFailure | null>(null);

  async function locate() {
    if (get === undefined || locating) return;
    setLocating(true);
    setFailure(null);
    onLocation(null);
    const next = await locateOnce(get);
    onLocation(next.location);
    setFailure(next.failure);
    setLocating(false);
  }

  /** A new, empty form: forget the last sentence too, or it would speak about a petition already sent. */
  function reset() {
    setFailure(null);
  }

  return { locating, failure, locate, reset };
}

/**
 * The button and its one status line. `look` picks the classes of the screen it sits in; both are full
 * tap targets (`.cd-nut-phu`, `.xa-nut` — `accessibility.test.ts`), and the status is WORDS, never only a
 * colour (`role="status"`, read out when it changes).
 */
export function SceneLocationControl(props: {
  words: SceneLocationWords;
  look: "shared" | "commune";
  locating: boolean;
  location: SceneLocation | null;
  failure: SceneLocationFailure | null;
  onLocate: () => void;
}) {
  const { words, look, locating, location, failure } = props;
  const label = locating ? words.locating : location !== null ? words.button_again : words.button;
  const shared = look === "shared";
  return (
    <div className={shared ? "cd-vi-tri" : "xa-vi-tri"}>
      <button
        type="button"
        className={shared ? "cd-nut-phu" : "xa-nut xa-nut--phu"}
        onClick={props.onLocate}
        disabled={locating}
      >
        {!shared && <Icon name="pin" size={20} />}
        {label}
      </button>
      <p className={shared ? "cd-ghi-chu" : "xa-phu"}>{words.why}</p>
      {location !== null && (
        <p className={shared ? "cd-cau" : "xa-phu"} role="status">
          {words.found(formatCoordinates(location))}
        </p>
      )}
      {failure !== null && (
        <p className={shared ? "cd-loi" : "xa-loi-o"} role="status">
          {words.failures[failure]}
        </p>
      )}
    </div>
  );
}
