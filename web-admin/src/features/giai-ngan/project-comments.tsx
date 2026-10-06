"use client";

import { MessagesSquare, Send } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing shared result type of goi.ts (rule 12 invariant 3)
import { getProjectComments, postProjectComment } from "@/lib/api/project-discussion";
import type {
  finance_projectCommentOut,
  finance_projectCommentsOut,
  identity_canBoChonNguoiRa,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { nhanMocKhoa } from "./nhan-ghi-giai-ngan";
import { MentionNoticePending } from "./pending-parts";
import {
  COMMENT_PLACEHOLDER,
  COMMENTS_DENIED,
  insertMention,
  matchingStaff,
  mentionedCodes,
  mentionQueryAt,
  mentionSegments,
  staffLabel,
  type PickedMention,
} from "./project-discussion-labels";
import type { PeopleCatalogue } from "./project-people";
import { DeniedNote, Glyph } from "./project-ui";

/**
 * §8.4 "Trao đổi" — a free discussion between staff about one project (prototype
 * `BudgetItemDetail.tsx:611-679`): messages oldest first, then the composer.
 *
 * MENTIONS: typing `@` opens a list of up to eight matching staff from the commune's directory; picking
 * one writes `@Full Name` into the text and remembers the CODE. What is sent is the codes whose
 * `@Full Name` is still in the text — so the body keeps the words a reader sees, and the server keeps
 * who was meant. Nobody is notified yet (`MentionNoticePending`).
 *
 * THE BODY IS RENDERED AS TEXT, NEVER HTML (rule 13): mentions are highlighted by cutting the string
 * into React text nodes (`mentionSegments`), so a message holding `<b>` shows `<b>`.
 *
 * `budget.read` is the route's key, as in the prototype: anyone who can see the project can discuss it.
 */

type CommentList =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly items: readonly finance_projectCommentOut[] };

function useProjectComments(projectId: string, reloadKey: number): CommentList {
  const key = `${projectId}|${reloadKey}`;
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<finance_projectCommentsOut> } | null>(null);

  useEffect(() => {
    let cancelled = false;
    getProjectComments(projectId).then((result) => {
      if (!cancelled) setLoaded({ key, result });
    });
    return () => {
      cancelled = true;
    };
  }, [projectId, key]);

  if (loaded === null || loaded.key !== key) return { phase: "loading" };
  if (!loaded.result.ok) return { phase: "error", message: loaded.result.thongBao };
  return { phase: "ready", items: loaded.result.duLieu.items };
}

export function ProjectCommentsPanel({
  projectId,
  staff,
  canComment,
}: {
  projectId: string;
  /** The staff directory (`useProjectPeople().staff`): names for authors, and the mention picker. */
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
  /** `budget.read` held. UX only — the route checks its key on every call. */
  canComment: boolean;
}) {
  const [reloads, setReloads] = useState(0);
  const comments = useProjectComments(projectId, reloads);
  const reload = () => setReloads((n) => n + 1);

  return (
    <div className="flex min-w-0 flex-col gap-3" data-comments-panel="">
      {comments.phase === "loading" && (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải trao đổi…
          </p>
          <div aria-hidden="true" className="flex flex-col gap-2">
            <Skeleton className="h-14 w-full" />
          </div>
        </div>
      )}
      {comments.phase === "error" && (
        <ErrorState
          title="Chưa tải được trao đổi của dự án"
          message={<span role="alert">{comments.message}</span>}
          onRetry={reload}
        />
      )}
      {comments.phase === "ready" && <CommentList items={comments.items} staff={staff} />}

      {canComment ? (
        <CommentComposer projectId={projectId} staff={staff} onPosted={reload} />
      ) : (
        <DeniedNote>{COMMENTS_DENIED}</DeniedNote>
      )}
    </div>
  );
}

/** Oldest first, as the server orders it. Presentational. */
export function CommentList({
  items,
  staff,
}: {
  items: readonly finance_projectCommentOut[];
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
}) {
  if (items.length === 0) {
    return <EmptyState icon={MessagesSquare} title="Chưa có ý kiến trao đổi nào." />;
  }
  return (
    <ol className="m-0 flex list-none flex-col gap-3 p-0" aria-label="Trao đổi về dự án">
      {items.map((c) => {
        // Only names the directory resolves are highlighted; an unresolved code leaves the text plain.
        const names =
          staff.phase === "ready"
            ? c.mentioned_staff_codes.flatMap((code) => {
                const n = staff.names.get(code);
                return n === undefined ? [] : [n];
              })
            : [];
        return (
          <li key={c.id} data-comment={c.id} className="rounded-xl border border-line bg-surface-subtle px-3 py-2.5">
            <div className="mb-1 flex flex-wrap items-baseline gap-x-2">
              <b className="text-[13px] text-ink-900">{staffLabel(c.author_code, staff)}</b>
              <time dateTime={c.created_at} className="text-xs text-ink-500 tabular-nums">
                {nhanMocKhoa(c.created_at)}
              </time>
            </div>
            <p className="m-0 text-sm break-words whitespace-pre-line text-ink-900">
              {mentionSegments(c.body, names).map((s, i) =>
                s.mention ? (
                  <span key={i} data-mention="" className="font-semibold text-brand-700">
                    {s.text}
                  </span>
                ) : (
                  s.text
                ),
              )}
            </p>
          </li>
        );
      })}
    </ol>
  );
}

function CommentComposer({
  projectId,
  staff,
  onPosted,
}: {
  projectId: string;
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
  onPosted: () => void;
}) {
  const box = useRef<HTMLTextAreaElement>(null);
  const [text, setText] = useState("");
  /** Caret position; `null` closes the picker (after a pick, on Escape). */
  const [caret, setCaret] = useState<number | null>(null);
  const [picked, setPicked] = useState<readonly PickedMention[]>([]);
  const [idempotencyKey, setIdempotencyKey] = useState(khoaChongTrungMoi);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** Where the caret goes once a pick has re-rendered the text. */
  const restoreCaret = useRef<number | null>(null);

  useEffect(() => {
    const at = restoreCaret.current;
    if (at === null || box.current === null) return;
    restoreCaret.current = null;
    box.current.focus();
    box.current.setSelectionRange(at, at);
  }, [text]);

  const query = caret === null ? null : mentionQueryAt(text, caret);
  const options = query !== null && staff.phase === "ready" ? matchingStaff(staff.items, query.query) : [];
  const empty = text.trim() === "";

  function pick(s: identity_canBoChonNguoiRa): void {
    if (query === null) return;
    const next = insertMention(text, query, s.full_name);
    restoreCaret.current = next.caret;
    setText(next.text);
    setCaret(null);
    setPicked((p) => [...p, { code: s.code, name: s.full_name }]);
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>): void {
    if (e.key === "Escape" && options.length > 0) {
      e.preventDefault();
      setCaret(null);
    }
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (empty || busy) return;
    setBusy(true);
    postProjectComment(
      projectId,
      { body: text, mentioned_staff_codes: mentionedCodes(text, picked) },
      idempotencyKey,
    ).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      setError(null);
      setText("");
      setCaret(null);
      setPicked([]);
      setIdempotencyKey(khoaChongTrungMoi());
      onPosted();
    });
  }

  return (
    <form onSubmit={submit} aria-label="Gửi ý kiến trao đổi" className="flex min-w-0 flex-col gap-2">
      <label htmlFor="trao-doi-moi" className="an-thi-giac">
        Ý kiến trao đổi
      </label>
      <textarea
        id="trao-doi-moi"
        name="trao-doi-moi"
        ref={box}
        rows={2}
        value={text}
        placeholder={COMMENT_PLACEHOLDER}
        className={cn(controlClass, "h-auto min-h-16 w-full min-w-0 resize-y py-2")}
        aria-controls={options.length > 0 ? "chon-nguoi-nhac-ten" : undefined}
        onChange={(e) => {
          setText(e.target.value);
          setCaret(e.target.selectionStart);
        }}
        onSelect={(e) => setCaret(e.currentTarget.selectionStart)}
        onKeyDown={onKeyDown}
      />

      {options.length > 0 && (
        <div
          id="chon-nguoi-nhac-ten"
          role="group"
          aria-label="Chọn cán bộ để nhắc tên"
          className="flex flex-wrap gap-1.5"
          data-mention-picker=""
        >
          {options.map((s) => (
            <button
              key={s.code}
              type="button"
              data-staff-code={s.code}
              className="inline-flex min-h-8 items-center gap-1 rounded-full border border-line bg-surface px-2.5 text-xs text-ink-700 hover:border-brand-500 hover:text-brand-700"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pick(s)}
            >
              @{s.full_name}
              {s.position !== "" && <span className="text-ink-500">· {s.position}</span>}
            </button>
          ))}
        </div>
      )}

      {staff.phase === "error" && (
        <p className="m-0 text-xs text-ink-500">Chưa tải được danh bạ cán bộ, nên chưa nhắc tên được.</p>
      )}

      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <MentionNoticePending />
        <Button
          type="submit"
          variant="primary"
          className="ml-auto"
          icon={busy ? undefined : <Glyph icon={Send} />}
          disabled={empty || busy}
          aria-busy={busy || undefined}
        >
          <BusyLabel busy={busy} label="Gửi" busyText="Đang gửi…" />
        </Button>
      </div>
    </form>
  );
}
