import { luaChonCanBo } from "@/features/meetings/meeting-labels";
import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

/**
 * Pure half of `StaffCombobox` — matching and keyboard transitions, kept out of the component so
 * they run under vitest in plain Node (this repo has no DOM test environment, `vitest.config.mts`).
 */

/**
 * Lower-case, strip combining marks, and fold `đ`/`Đ` to `d`.
 *
 * `đ` needs its own line: it is a separate letter, not `d` plus a mark, so NFD leaves it intact.
 * Without the fold, "dung" does not find "Dũng" but also does not find "Đặng" — the one miss a
 * clerk typing without a Vietnamese keyboard would hit first.
 */
export function foldForSearch(s: string): string {
  return s
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[đĐ]/g, "d")
    .toLowerCase();
}

export type StaffOption = {
  /** Business code `CB-…` — what the form sends. `""` is the "nobody" choice. */
  readonly value: string;
  /** Text shown, `Họ tên · Chức vụ` (`nhanLuaChonCanBo`). */
  readonly label: string;
  /** Folded text the query is matched against. */
  readonly searchText: string;
};

/**
 * Directory → options. Labels come from `luaChonCanBo`, the same source `OChonCanBo` uses, so a
 * stored value missing from the directory still gets its own line instead of vanishing.
 *
 * MATCHES THE FULL NAME, NOT THE POSITION: "chu" would otherwise match every `Chủ tịch` and
 * `Chuyên viên` in the commune. The out-of-directory line has no name, so it matches its code.
 */
export function staffOptions(
  directory: readonly identity_canBoChonNguoiRa[],
  current: string,
): StaffOption[] {
  const nameByCode = new Map(directory.map((cb) => [cb.code, cb.full_name]));
  return luaChonCanBo(directory, current).map((o) => ({
    value: o.ma,
    label: o.nhan,
    searchText: foldForSearch(nameByCode.get(o.ma) ?? o.ma),
  }));
}

/**
 * Every whitespace-separated word of the query must appear SOMEWHERE in the name, in any order.
 * A plain substring would already find "van a" in "Trần Văn An"; words also find "nguyen an",
 * which is how people actually recall a colleague's name.
 */
export function filterStaffOptions(
  options: readonly StaffOption[],
  query: string,
): StaffOption[] {
  const words = foldForSearch(query).split(/\s+/).filter((w) => w !== "");
  if (words.length === 0) return [...options];
  return options.filter((o) => words.every((w) => o.searchText.includes(w)));
}

/** One line of the open list. */
export type VisibleOption = { readonly value: string; readonly label: string };

/**
 * What the list shows. No query → the "nobody" line first, then everyone, so the value can always
 * be cleared. A query → only matches: the "nobody" line would match no name, and keeping it on top
 * would make Enter clear the field instead of picking the first match.
 */
export function visibleOptions(
  options: readonly StaffOption[],
  emptyLabel: string,
  query: string | null,
): VisibleOption[] {
  if (query === null || query.trim() === "") {
    return [{ value: "", label: emptyLabel }, ...options];
  }
  return filterStaffOptions(options, query);
}

export type ComboboxState = {
  readonly open: boolean;
  /** `null` = not typing: the input shows the selected label. */
  readonly query: string | null;
  /** Index into the visible list, `-1` = none. */
  readonly active: number;
};

export const COMBOBOX_CLOSED: ComboboxState = { open: false, query: null, active: -1 };

/** Typing opens the list and highlights the first match, so Enter picks it. */
export function comboboxTyped(
  text: string,
  options: readonly StaffOption[],
  emptyLabel: string,
): ComboboxState {
  const visible = visibleOptions(options, emptyLabel, text);
  return { open: true, query: text, active: text.trim() !== "" && visible.length > 0 ? 0 : -1 };
}

export type ComboboxKeyResult = {
  readonly state: ComboboxState;
  /** A value to commit, or `null`. */
  readonly commit: string | null;
  /** The key was consumed — the caller calls `preventDefault`. */
  readonly handled: boolean;
};

/**
 * ARIA combobox keys (WAI-ARIA APG, editable combobox with list popup).
 *
 * ENTER IS ALWAYS CONSUMED, open or closed. The `Giao việc` form submits on Enter, and a task
 * created by that stray Enter carries an empty `Lãnh đạo giao việc` FOREVER — `PATCH` does not
 * accept that column. A native `<select>` never submitted on Enter; this box must not start.
 */
export function comboboxKey(
  state: ComboboxState,
  key: string,
  visible: readonly VisibleOption[],
  selected: string,
): ComboboxKeyResult {
  const count = visible.length;
  switch (key) {
    case "ArrowDown": {
      if (!state.open) {
        const i = visible.findIndex((o) => o.value === selected);
        return { state: { ...state, open: true, active: count === 0 ? -1 : Math.max(i, 0) }, commit: null, handled: true };
      }
      const active = count === 0 ? -1 : Math.min(state.active + 1, count - 1);
      return { state: { ...state, active }, commit: null, handled: true };
    }
    case "ArrowUp": {
      if (!state.open) {
        return { state: { ...state, open: true, active: count - 1 }, commit: null, handled: true };
      }
      const active = count === 0 ? -1 : Math.max(state.active - 1, 0);
      return { state: { ...state, active }, commit: null, handled: true };
    }
    case "Enter": {
      const picked = state.open ? visible[state.active] : undefined;
      if (picked !== undefined) {
        return { state: COMBOBOX_CLOSED, commit: picked.value, handled: true };
      }
      return { state, commit: null, handled: true };
    }
    case "Escape": {
      // Closing reverts the text to the selection — Escape never changes the value.
      if (state.open || state.query !== null) {
        return { state: COMBOBOX_CLOSED, commit: null, handled: true };
      }
      return { state, commit: null, handled: false };
    }
    case "Tab":
      // Leaving never picks anything: a value changes only by an explicit Enter or click.
      return { state: COMBOBOX_CLOSED, commit: null, handled: false };
    default:
      return { state, commit: null, handled: false };
  }
}
