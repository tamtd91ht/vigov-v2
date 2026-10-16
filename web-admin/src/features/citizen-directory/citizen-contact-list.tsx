"use client";

import { Search, Tag, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Pagination } from "@/components/ui/pagination";
import { PendingMarker } from "@/components/ui/pending-feature";
import { SkeletonRows } from "@/components/ui/skeleton";
import { Tooltip } from "@/components/ui/tooltip";
import { normalizeCitizenKeyword, type CitizenContact, type CitizenContactFilter, type CitizenContactPage, type CitizenContactTag } from "@/lib/api/citizen-contacts";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import { cn } from "@/lib/cn";

import { PHAN_CHUA_DUNG } from "./citizen-directory-pending";
import {
  ALL_GROUPS,
  COLUMN_NAME,
  COLUMN_PHONE,
  COLUMN_TAGS,
  COLUMN_UNIT,
  COLUMN_ZALO,
  EMPTY_CELL,
  GROUP_FILTER_LABEL,
  KEYWORD_TOO_LONG,
  ONLY_WITH_ZALO,
  PAGINATION_LABEL,
  SEARCH_LABEL,
  SEARCH_PLACEHOLDER,
  TABLE_LABEL,
  TAG_STRIP_LABEL,
  ZALO_COLUMN_TOOLTIP,
  ZALO_YES,
  deleteContactLabel,
  emptySentence,
  groupOptionText,
  pageLabel,
  tagChipText,
  tagsCellText,
  unitCellText,
} from "./citizen-directory-words";

/** Pause after the last keystroke before the keyword is applied (the staff directory's figure). */
export const SEARCH_DELAY_MS = 300;

export const SEARCH_INPUT_ID = "tim-nguoi-dan"; // vi-name-ok: DOM id, same convention as `tim-danh-ba`
export const GROUP_FILTER_ID = "loc-nhom-nguoi-dan"; // vi-name-ok: DOM id, same convention
export const ZALO_FILTER_ID = "loc-zalo-nguoi-dan"; // vi-name-ok: DOM id, same convention

/** One page read, three phases apart. */
export type ContactPageState = { phase: "loading" } | { phase: "error"; message: string } | { phase: "ready"; page: CitizenContactPage };

const SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 rounded-md border border-solid bg-white pr-9 pl-3 text-[12.5px] outline-none focus-visible:ring-[3px]";

/**
 * The Danh sách tab — the prototype's `CitizenWorkspace` list half (chapter 16 §2–§4): the filter row,
 * the tag strip, the table, the pagination. PRESENTATION PLUS THE SEARCH BOX'S OWN TYPING STATE: the
 * filter in force, the page and every handler are the workspace's.
 *
 * THE KEYWORD IS APPLIED AS TYPED, AFTER A PAUSE (`SEARCH_DELAY_MS`), as the prototype — no Tìm button;
 * Enter applies at once. Every application is `POST /citizen-contact-searches` with the words in the
 * BODY (ADR 0097 D11). The input has NO `name` and the form is `method="post"`: before JavaScript runs,
 * Enter in a form would otherwise put the typed digits on the URL (rule 3, forbidden #4).
 *
 * THE TAG STRIP IS REAL DATA AND A "?" (ADR 0068 §14): the tags and their counts come from the server;
 * filtering by one does not, because no read route has a tag field — `PHAN_CHUA_DUNG` says so on the
 * strip. The chips are disabled buttons at the prototype's position, never a filter that pretends.
 *
 * NUMBERS ARE MASKED HERE AND STAY MASKED: `phone_masked` is the only phone field of a row (D2). The
 * row opens the edit dialog only for a `citizen.update` holder; without it the row is not clickable and
 * the action column is not drawn (`CitizenWorkspace.tsx:178, 185`).
 */
export function CitizenContactList({
  filter,
  groups,
  tags,
  state,
  pageNumber,
  hasPrev,
  canEdit,
  onFilterChange,
  onPrev,
  onNext,
  onEdit,
  onDelete,
  confirm,
}: {
  filter: CitizenContactFilter;
  groups: readonly CitizenGroup[];
  tags: readonly CitizenContactTag[];
  state: ContactPageState;
  /** 1-based position in the cursor stack — the one page figure the client knows. */
  pageNumber: number;
  hasPrev: boolean;
  canEdit: boolean;
  onFilterChange: (change: Partial<CitizenContactFilter>) => void;
  onPrev: () => void;
  onNext: () => void;
  onEdit: (row: CitizenContact) => void;
  onDelete: (row: CitizenContact) => void;
  /** The delete confirm box, drawn above the table when open. */
  confirm?: ReactNode;
}) {
  const [typed, setTyped] = useState("");
  const [keywordError, setKeywordError] = useState("");

  function apply(text: string) {
    const kw = normalizeCitizenKeyword(text);
    if (kw.loai === "quaDai") {
      // The list keeps the conditions it had; the words do not leave the browser.
      setKeywordError(KEYWORD_TOO_LONG);
      return;
    }
    setKeywordError("");
    const next = kw.loai === "hopLe" ? kw.tu : "";
    if (next === filter.keyword) return;
    onFilterChange({ keyword: next });
  }

  useEffect(() => {
    const timer = setTimeout(() => apply(typed), SEARCH_DELAY_MS);
    return () => clearTimeout(timer);
    // `apply` closes over the filter in force; re-arming on every change of it is the point.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [typed, filter.keyword]);

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    apply(typed);
  }

  const page = state.phase === "ready" ? state.page : null;
  const hasNext = page !== null && page.has_more && page.next_cursor !== "";
  const pending = PHAN_CHUA_DUNG[0]!;

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {/* The prototype's filter row (`CitizenWorkspace.tsx:92-135`). Each `label + select` pair sits in a
          `contents` span, for the reason the staff directory gives (`HangLoc`): the legacy global rule
          `:has(> label + select)` would turn the row into a column. */}
      <div className="flex min-w-0 flex-wrap items-center gap-2.5">
        <div className="flex min-w-0 max-w-full flex-col">
          <form className="m-0 min-w-0" role="search" method="post" onSubmit={submit}>
            <Field label={SEARCH_LABEL} htmlFor={SEARCH_INPUT_ID} hideLabel icon={Search} grow="auto" className="w-72 max-w-full">
              <input
                id={SEARCH_INPUT_ID}
                type="search"
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                placeholder={SEARCH_PLACEHOLDER}
                autoComplete="off"
                aria-describedby="loi-tim-nguoi-dan"
                aria-invalid={keywordError !== ""}
                className="h-9! text-[12.5px]!"
              />
            </Field>
          </form>
          <p id="loi-tim-nguoi-dan" className="text-danger m-0 min-h-0 text-[12px] font-medium [&:not(:empty)]:mt-1.5" role="alert">
            {keywordError}
          </p>
        </div>

        <span className="contents">
          <label htmlFor={GROUP_FILTER_ID} className="an-thi-giac">
            {GROUP_FILTER_LABEL}
          </label>
          <select id={GROUP_FILTER_ID} className={SELECT_CLASS} value={filter.groupId} onChange={(e) => onFilterChange({ groupId: e.target.value })}>
            <option value="">{ALL_GROUPS}</option>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                {groupOptionText(g)}
              </option>
            ))}
          </select>
        </span>

        <label htmlFor={ZALO_FILTER_ID} className="text-ink flex items-center gap-2 text-[12.5px]">
          <input
            id={ZALO_FILTER_ID}
            type="checkbox"
            className="accent-brand size-3.5"
            checked={filter.hasOwnAppSession === true}
            onChange={(e) => onFilterChange({ hasOwnAppSession: e.target.checked ? true : null })}
          />
          {ONLY_WITH_ZALO}
        </label>
      </div>

      {tags.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={TAG_STRIP_LABEL} data-tag-strip="">
          <Tag aria-hidden="true" focusable="false" className="text-ink-muted size-3.5" />
          <PendingMarker info={pending} side="right" />
          {tags.map((t) => (
            <button
              key={t.tag}
              type="button"
              disabled
              className="border-line text-ink-muted rounded-full border border-solid bg-white px-2.5 py-1 [font-family:inherit] text-[11.5px] font-semibold disabled:opacity-60"
            >
              {tagChipText(t.tag, t.count)}
            </button>
          ))}
        </div>
      )}

      {confirm}

      <div className="border-line shadow-card rounded-card min-w-0 border border-solid bg-white">
        {state.phase === "loading" && (
          <>
            <p className="an-thi-giac" role="status">
              Đang tải danh bạ…
            </p>
            <SkeletonRows rows={5} columns={4} />
          </>
        )}
        {state.phase === "error" && (
          <p className="thong-bao-loi m-4" role="alert">
            {state.message}
          </p>
        )}
        {page !== null && page.items.length === 0 && <EmptyState tone="neutral" title={emptySentence(filter)} role="status" />}
        {page !== null && page.items.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[46rem] border-collapse text-[12.5px]">
              <caption className="an-thi-giac">{TABLE_LABEL}</caption>
              <thead>
                <tr className="text-ink-muted border-line border-b border-solid text-left text-[11px] uppercase">
                  <th scope="col" className="px-4 py-2.5 font-semibold">
                    {COLUMN_NAME}
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-semibold">
                    {COLUMN_PHONE}
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-semibold">
                    {COLUMN_UNIT}
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-semibold">
                    {COLUMN_TAGS}
                  </th>
                  <th scope="col" className="px-4 py-2.5 font-semibold">
                    {/* D13: the word stays "Zalo"; the meaning is the tooltip — and the `title`, for touch. */}
                    <Tooltip content={ZALO_COLUMN_TOOLTIP}>
                      <span tabIndex={0} title={ZALO_COLUMN_TOOLTIP} className="cursor-help underline decoration-dotted underline-offset-2">
                        {COLUMN_ZALO}
                      </span>
                    </Tooltip>
                  </th>
                  {canEdit && <th scope="col" className="px-4 py-2.5" />}
                </tr>
              </thead>
              <tbody>
                {page.items.map((row) => (
                  <tr
                    key={row.id}
                    onClick={canEdit ? () => onEdit(row) : undefined}
                    className={cn(
                      "border-line hover:bg-surface border-b border-solid last:border-b-0",
                      canEdit && "cursor-pointer",
                      // §10 #8: a switched-off row stays in the directory, dimmed.
                      !row.is_active && "opacity-60",
                    )}
                  >
                    <td className="text-navy px-4 py-2.5 font-semibold">{row.full_name}</td>
                    <td className="px-4 py-2.5 tabular-nums">{row.phone_masked}</td>
                    <td className="text-ink px-4 py-2.5">{unitCellText(row)}</td>
                    <td className="px-4 py-2.5">{tagsCellText(row.tags)}</td>
                    <td className="px-4 py-2.5">
                      {row.has_own_app_session ? <span className="text-leaf font-semibold">{ZALO_YES}</span> : <span className="text-ink-muted">{EMPTY_CELL}</span>}
                    </td>
                    {canEdit && (
                      <td className="px-4 py-2.5 text-right">
                        <IconButton
                          label={deleteContactLabel(row.full_name)}
                          variant="ghost"
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            onDelete(row);
                          }}
                        >
                          <Trash2 aria-hidden="true" className="size-4" />
                        </IconButton>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {(hasPrev || hasNext) && (
        <div className="text-ink flex items-center justify-end gap-3 text-[12.5px]">
          <span>{pageLabel(pageNumber)}</span>
          <Pagination hasPrev={hasPrev} hasNext={hasNext} onPrev={onPrev} onNext={onNext} busy={state.phase === "loading"} label={PAGINATION_LABEL} />
        </div>
      )}
    </div>
  );
}
