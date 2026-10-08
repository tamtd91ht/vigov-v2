"use client";

import { Send } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing shared result type of goi.ts (rule 12 invariant 3)
import { getProjectComments, postProjectComment } from "@/lib/api/project-discussion";
import type {
  finance_projectCommentOut,
  finance_projectCommentsOut,
  identity_canBoChonNguoiRa,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { nhanMocKhoa } from "./nhan-ghi-giai-ngan";
import { COMMENT_PLACEHOLDER, MENTION_PICKER_LIMIT, mentionLine, staffLabel } from "./project-discussion-labels";
import type { PeopleCatalogue } from "./project-people";
import { Glyph } from "./project-ui";

/**
 * §8.4 "Trao đổi" — a free discussion between staff about one project (spec 07, prototype
 * `BudgetItemDetail.tsx:611-679`): messages oldest first, then the composer.
 *
 * MENTIONS AS THE PROTOTYPE DRAWS THEM (ADR 0068 lần 6, row D35): a row of at most eight `@Tên` chips
 * under the box, the first eight of the commune's staff directory; pressing one toggles it. What is
 * sent is the CODES of the chips on (`mentioned_staff_codes`); the body stays exactly what was typed.
 * A message's mentions are listed under it as "Nhắc: @A, @B". Each person mentioned (but the author)
 * gets a bell notice from the server once the comment is saved (ADR 0081 #5) — nothing to show here.
 *
 * KNOWN COST of following the prototype: only the first eight staff of the directory can be mentioned.
 *
 * THE BODY IS RENDERED AS TEXT, NEVER HTML (rule 13): React text nodes only.
 *
 * `budget.read` is the route's key, as in the prototype: anyone who can see the project can discuss it.
 * Without it the composer is simply absent (spec 07); the route checks the key on every call.
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
  /** The staff directory (`useProjectPeople().staff`): names for authors, and the mention chips. */
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
  /** `budget.read` held. UX only — the route checks its key on every call. */
  canComment: boolean;
}) {
  const [reloads, setReloads] = useState(0);
  const comments = useProjectComments(projectId, reloads);
  const reload = () => setReloads((n) => n + 1);

  return (
    <div className="flex min-w-0 flex-col" data-comments-panel="">
      {comments.phase === "loading" && (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải trao đổi…
          </p>
          <Skeleton className="h-14 w-full" />
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

      {canComment && <CommentComposer projectId={projectId} staff={staff} onPosted={reload} />}
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
    return <p className="text-ink-muted m-0 text-[12.5px]">Chưa có ý kiến trao đổi nào.</p>;
  }
  return (
    <ol className="m-0 list-none p-0" aria-label="Trao đổi về dự án">
      {items.map((c) => {
        const mentions = mentionLine(c.mentioned_staff_codes, staff);
        return (
          <li
            key={c.id}
            data-comment={c.id}
            className="border-line bg-canvas mb-3 rounded-[10px] border border-solid px-3 py-2.5"
          >
            <div className="mb-1 flex flex-wrap items-baseline gap-x-2">
              <b className="text-navy text-[12.5px]">{staffLabel(c.author_code, staff)}</b>
              <time dateTime={c.created_at} className="text-ink-muted text-[11px] tabular-nums">
                {nhanMocKhoa(c.created_at)}
              </time>
            </div>
            <p className="m-0 text-[12.5px] break-words whitespace-pre-line">{c.body}</p>
            {mentions !== null && (
              <p className="text-brand m-0 mt-1 text-[11px]" data-mentions="">
                {mentions}
              </p>
            )}
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
  const [text, setText] = useState("");
  /** Codes of the chips switched on, in the order they were pressed. */
  const [picked, setPicked] = useState<readonly string[]>([]);
  const [idempotencyKey, setIdempotencyKey] = useState(khoaChongTrungMoi);
  const [busy, setBusy] = useState(false);
  const chips = staff.phase === "ready" ? staff.items.slice(0, MENTION_PICKER_LIMIT) : [];
  const empty = text.trim() === "";

  function toggle(code: string): void {
    setPicked((p) => (p.includes(code) ? p.filter((c) => c !== code) : [...p, code]));
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (empty || busy) return;
    setBusy(true);
    postProjectComment(projectId, { body: text, mentioned_staff_codes: [...picked] }, idempotencyKey).then((r) => {
      setBusy(false);
      if (!r.ok) {
        // The server's sentence; the spec's generic "Không gửi được ý kiến trao đổi." would hide why.
        toast.error(r.thongBao);
        return;
      }
      setText("");
      setPicked([]);
      setIdempotencyKey(khoaChongTrungMoi());
      onPosted();
    });
  }

  return (
    <form onSubmit={submit} aria-label="Gửi ý kiến trao đổi" className="mt-3 flex min-w-0 flex-col gap-2">
      <label htmlFor="trao-doi-moi" className="an-thi-giac">
        Ý kiến trao đổi
      </label>
      <textarea
        id="trao-doi-moi"
        name="trao-doi-moi"
        rows={2}
        value={text}
        placeholder={COMMENT_PLACEHOLDER}
        className={cn(controlClass, "h-auto min-h-16 w-full min-w-0 resize-y bg-white py-2 text-[12.5px] md:text-[12.5px]")}
        onChange={(e) => setText(e.target.value)}
      />

      <div className="flex flex-wrap items-center gap-2">
        {chips.length > 0 && (
          <div role="group" aria-label="Chọn cán bộ để nhắc tên" className="flex flex-wrap gap-2" data-mention-picker="">
            {chips.map((s) => {
              const on = picked.includes(s.code);
              return (
                <button
                  key={s.code}
                  type="button"
                  aria-pressed={on}
                  data-staff-code={s.code}
                  className={cn(
                    "border-line cursor-pointer rounded-full border border-solid px-2 py-0.5 [font-family:inherit] text-[11px]",
                    on ? "bg-brand/12 text-brand border-brand/25 font-semibold" : "text-ink-muted hover:bg-canvas bg-white",
                  )}
                  onClick={() => toggle(s.code)}
                >
                  @{s.full_name}
                </button>
              );
            })}
          </div>
        )}
        {staff.phase === "error" && (
          <p className="text-ink-muted m-0 text-[11px]">Chưa tải được danh bạ cán bộ, nên chưa nhắc tên được.</p>
        )}
        <Button
          type="submit"
          variant="primary"
          className="ml-auto"
          icon={<Glyph icon={Send} />}
          disabled={empty || busy}
          aria-busy={busy || undefined}
        >
          Gửi
        </Button>
      </div>
    </form>
  );
}
