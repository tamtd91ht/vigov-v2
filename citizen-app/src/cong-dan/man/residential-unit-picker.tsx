/**
 * THE OPTIONAL "THÔN, TỔ DÂN PHỐ" PICKER OF BOTH SEND SCREENS (owner, 09/10/2026 — ADR 0088 §Trả lời): the citizen
 * MAY say which thôn / tổ dân phố the problem is in; the commune uses it to route and count by place.
 *
 *   · The list is the COMMUNE's (`GET /api/v1/my-residential-units`, identity), read once when the form opens and
 *     only with a session. No parameter leaves the phone: the commune is the session's (rule 1, forbidden #2).
 *   · OPTIONAL in every state. "Không chọn" is always the first choice and the default; nothing here can block the
 *     send button. An EMPTY list hides the field (a commune with no units has nothing to ask). A list that failed
 *     to load is ONE sentence saying the petition can still be sent — no retry loop the citizen must get through.
 *   · There is NO built-in list (as for the field catalogue, ADR 0060 §3): a guessed unit is a petition counted in
 *     the wrong place.
 *   · Collapsed by default to one line "Thôn, tổ dân phố: <tên> · Chọn/Đổi" — a commune may have thirty units,
 *     and thirty tiles inside the form would push the send step off the screen. Open, it is one full-width tile
 *     per unit (≥ 48px, long names wrap), `<button role="radio">` in a `radiogroup`, the choice said in WORDS
 *     ("Đã chọn"), never by a border alone (README §Non-negotiables #6). No `<select>`: this app's only input
 *     controls live in `o-nhap.tsx` (`phase1-collects-nothing.test.ts`), and a native select is a 14px list on
 *     some phones.
 *   · Not kept in the draft store: the draft's six fields are a decided list (ADR 0050 #7), and a unit is cheap to
 *     pick again. Nothing here is logged.
 *
 * The shared app ("bạn") and the commune's own app ("bà con") pass their own words (`ResidentialUnitWords`).
 */
import { useEffect, useRef, useState } from "react";

import type { myResidentialUnits } from "../api/goi-vigov";
import type { ResidentialUnit } from "../api/hop-dong-phan-anh";

/** What the field knows about the commune's list. `none` = nothing to offer: the field is not drawn. */
export type ResidentialUnitList =
  | { readonly kind: "loading" }
  | { readonly kind: "none" }
  | { readonly kind: "failed" }
  | { readonly kind: "ready"; readonly units: readonly ResidentialUnit[] };

export type ResidentialUnitAnswer = Awaited<ReturnType<typeof myResidentialUnits>>;

/**
 * One answer → the field's state. PURE. An empty list, no session or no identity host in this build are `none`:
 * nothing to pick and nothing the citizen can do about it, so nothing is said. Every other failure (401, 5xx,
 * network, a malformed body) is `failed`: one sentence — the send itself deals with the session.
 */
export function readResidentialUnitAnswer(kq: ResidentialUnitAnswer): ResidentialUnitList {
  switch (kq.kieu) {
    case "xong":
      return kq.units.length === 0 ? { kind: "none" } : { kind: "ready", units: kq.units };
    case "chua-co-phien":
    case "chua-cau-hinh":
      return { kind: "none" };
    default:
      return { kind: "failed" };
  }
}

/** The picked unit's name in a loaded list, or "" — the citizen never reads an id. */
export function residentialUnitName(list: ResidentialUnitList, id: string): string {
  if (list.kind !== "ready" || id === "") return "";
  return list.units.find((u) => u.id === id)?.name ?? "";
}

/**
 * The list of this send screen. Loaded once on mount when `enabled` (ref guard: StrictMode runs effects twice, and
 * one screen must not cost two calls); `reload` after a 400 `residential_unit_not_offered`. `load` is injected so
 * the screen passes the API call and this file stays free of the network (`phase1-collects-nothing.test.ts`).
 */
export function useResidentialUnits(load: () => Promise<ResidentialUnitAnswer>, enabled: boolean) {
  const [list, setList] = useState<ResidentialUnitList>(enabled ? { kind: "loading" } : { kind: "none" });
  const busy = useRef(false);

  async function fetchList() {
    if (busy.current) return;
    busy.current = true;
    setList({ kind: "loading" });
    const next = readResidentialUnitAnswer(await load());
    busy.current = false;
    setList(next);
  }

  const started = useRef(false);
  useEffect(() => {
    if (!enabled || started.current) return;
    started.current = true;
    void fetchList();
  }, []);

  return { list, reload: () => void (enabled ? fetchList() : undefined) };
}

/** The words of one app's voice. `prompt` and `failed` carry the voice; the rest are the same in both. */
export type ResidentialUnitWords = {
  readonly label: string;
  /** "Thôn, tổ dân phố" — on the summary line, the confirmation step and the send footer. */
  readonly summary_label: string;
  readonly none: string;
  readonly open: string;
  readonly change: string;
  /**
   * The accessible names of "Chọn" / "Đổi": the visible word alone does not say what it opens, and the field line
   * above has a "Đổi" of its own. Each STARTS with its visible word, so a voice command naming what is seen works.
   */
  readonly open_name: string;
  readonly change_name: string;
  readonly picked: string;
  readonly prompt: string;
  readonly loading: string;
  readonly failed: string;
};

export type ResidentialUnitLook = "shared" | "commune";

/**
 * The field. `refusal`: the sentence of a 400 `residential_unit_not_offered` (the server's, or ours) — said HERE,
 * at the field, with the list open so the citizen can pick again or choose "Không chọn". `id` prefixes the
 * element ids (one field per screen).
 */
export function ResidentialUnitField(props: {
  id: string;
  list: ResidentialUnitList;
  picked: string;
  refusal: string | null;
  words: ResidentialUnitWords;
  look: ResidentialUnitLook;
  onPick: (id: string) => void;
}) {
  const { list, words, look } = props;
  const [open, setOpen] = useState(false);
  const shared = look === "shared";
  const hintClass = shared ? "cd-ghi-chu" : "xa-phu";
  const refusal =
    props.refusal === null ? null : (
      <p className={shared ? "cd-loi" : "xa-error-box"} role="alert">
        {props.refusal}
      </p>
    );

  // Nothing to offer: no field. A refusal still says why the send stopped (the list may have emptied since).
  if (list.kind === "none") return refusal;

  const labelId = `${props.id}-label`;
  const listId = `${props.id}-list`;
  const pickedName = residentialUnitName(list, props.picked);
  const expanded = list.kind === "ready" && (open || props.refusal !== null);

  function pick(id: string) {
    props.onPick(id);
    setOpen(false);
  }

  const options: ReadonlyArray<{ id: string; name: string }> =
    list.kind === "ready" ? [{ id: "", name: words.none }, ...list.units] : [];

  return (
    <div className="cd-o">
      <p className="cd-o__nhan" id={labelId}>
        {words.label}
      </p>
      {refusal}
      {list.kind === "loading" && (
        <p className={hintClass} role="status">
          {words.loading}
        </p>
      )}
      {list.kind === "failed" && <p className={hintClass}>{words.failed}</p>}
      {list.kind === "ready" && (
        <>
          <div className={shared ? "cd-field-picked" : "xa-field-picked"}>
            <span className={shared ? "cd-field-picked__text" : "xa-field-picked__text"}>
              {words.summary_label}: <strong>{pickedName === "" ? words.none : pickedName}</strong>
            </span>
            <button
              type="button"
              className={shared ? "cd-field-change" : "xa-field-change"}
              aria-label={pickedName === "" ? words.open_name : words.change_name}
              aria-expanded={expanded}
              aria-controls={listId}
              onClick={() => setOpen((o) => !o)}
            >
              {pickedName === "" ? words.open : words.change}
            </button>
          </div>
          {expanded && (
            <div id={listId}>
              <p className={hintClass}>{words.prompt}</p>
              <div className="cd-fields" role="radiogroup" aria-labelledby={labelId}>
                {options.map((u) => {
                  const on = u.id === "" ? pickedName === "" : props.picked === u.id;
                  return (
                    <button
                      key={u.id === "" ? "none" : u.id}
                      type="button"
                      role="radio"
                      aria-checked={on}
                      className={`cd-field${on ? " cd-field--picked" : ""}`}
                      onClick={() => pick(u.id)}
                    >
                      <span className="cd-field__label">{u.name}</span>
                      {on && <span className="cd-field__state">{words.picked}</span>}
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
