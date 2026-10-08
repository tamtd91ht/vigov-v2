"use client";

import { MessageSquareText, Plus, RotateCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";
import { PendingButton } from "@/components/ui/pending-feature";

import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import {
  listSystemMessages,
  type SystemMessage,
  type SystemMessageModule,
} from "@/lib/api/system-messages";

import { ConfigLoading, SMALL_BUTTON_CLASS } from "./config-ui";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import { systemMessagesTabDecision } from "./quyen-tab";
import {
  isTextChanged,
  OVERRIDDEN_BADGE,
  RESTORE_BUTTON,
  RESTORED_SENTENCE,
  restoreMessageFlow,
  SAVE_BUTTON,
  SAVED_SENTENCE,
  saveMessageFlow,
  SHIPPED_BADGE,
  SWITCH_OFF_BUTTON,
  SYSTEM_MESSAGE_MAX,
  SYSTEM_MESSAGE_SECTIONS,
  SYSTEM_MESSAGES_GUIDANCE,
  SYSTEM_MESSAGES_TITLE,
} from "./system-message-form";

/**
 * "Cấu hình → Lời hệ thống" — spec `07-loi-he-thong.md`, prototype `MessageTemplateTable.tsx`
 * (ADR 0079). One key, `admin.lookup`, on all nine routes — the tab hides as a whole without it
 * (convenience; the server refuses).
 *
 * "Thêm câu mới" AND "Tắt" ARE DRAWN AS "?" CONTROLS (ADR 0068 §14): the server's catalogue is closed
 * (a key outside it answers 404) and has no on/off state. "Xoá" is not drawn at all: every sentence
 * ships with the software, and the prototype hides "Xoá" for those.
 *
 * NO LAST-EDIT LINE, NO "NOT USED YET" NOTE, NO GROUP NOTICE, NO RESTORE CONFIRMATION: owner,
 * 08/10/2026, "Bỏ hết, đúng prototype". Who reworded a sentence and when stays in the audit trail.
 *
 * Each section loads and fails on its own: one service being down must not hide the other two.
 */
export function SystemMessagesTab() {
  const phien = usePhien();
  const decision = phien === null ? null : systemMessagesTabDecision(phien);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p role="alert" className="text-danger m-0 text-[12px] font-medium">
        {decision.thongBao}
      </p>
    ) : (
      // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
      <div className="flex min-w-0 flex-col items-center pb-10">
        <NoAccess className="pb-4" />
        <p className="m-0 max-w-md px-4 text-center text-[13px] text-ink-500">
          Tài khoản của bạn không có quyền sửa lời hệ thống, nên tab này không hiển thị.
        </p>
      </div>
    );
  }

  return (
    <section className="min-w-0" aria-labelledby="tieu-de-loi-he-thong">
      <h2 id="tieu-de-loi-he-thong" className="an-thi-giac">
        {SYSTEM_MESSAGES_TITLE}
      </h2>
      <div className="mb-4 flex flex-wrap items-start gap-3">
        <p className="text-ink-muted m-0 max-w-2xl text-[12.5px]">{SYSTEM_MESSAGES_GUIDANCE}</p>
        {ADD_ENTRY !== undefined && (
          // Default size, not sm (spec 07): it is the tab's one header action.
          <PendingButton
            info={ADD_ENTRY}
            variant="primary"
            icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
            className="ml-auto"
          />
        )}
      </div>
      {SYSTEM_MESSAGE_SECTIONS.map((s) => (
        <SystemMessageSection key={s.module} module={s.module} title={s.title} />
      ))}
    </section>
  );
}

/** Looked up by name so a renamed entry fails a test, not a screen. */
const ADD_ENTRY = PHAN_CHUA_DUNG.find((p) => p.ten === "Thêm câu mới");
const SWITCH_OFF_ENTRY = PHAN_CHUA_DUNG.find((p) => p.ten === "Tắt câu hệ thống");

/** The shared Badge's pill box, without its tone icon (same as tab-danh-muc's "Mặc định" pill). */
const PILL_CLASS =
  "inline-flex h-5 w-fit shrink-0 items-center rounded-4xl border border-solid px-2 py-0.5 text-xs leading-none font-medium whitespace-nowrap";

function SystemMessageSection({ module, title }: { module: SystemMessageModule; title: string }) {
  // After a restore the list is RE-READ (DELETE answers 204, and the server is the state). Cards are
  // keyed by the read that produced them, so they remount on the new data.
  const [reload, setReload] = useState(0);
  const [loaded, setLoaded] = useState<{ n: number; r: KetQua<readonly SystemMessage[]> } | null>(
    null,
  );

  useEffect(() => {
    let gone = false;
    void listSystemMessages(module).then((r) => {
      if (!gone) setLoaded({ n: reload, r });
    });
    return () => {
      gone = true;
    };
  }, [module, reload]);

  const headingId = `loi-he-thong-${module}`;
  return (
    <section className="mt-5 min-w-0 first-of-type:mt-0" aria-labelledby={headingId}>
      <h3 id={headingId} className="text-navy m-0 mb-2.5 text-[12.5px] font-bold">
        {title}
      </h3>
      {loaded === null ? (
        <ConfigLoading label="Đang tải lời hệ thống…" />
      ) : !loaded.r.ok ? (
        <ErrorState role="alert" title="Chưa tải được lời hệ thống" message={loaded.r.thongBao} className="py-6" />
      ) : loaded.r.duLieu.length === 0 ? (
        <EmptyState icon={MessageSquareText} title="Phân hệ này chưa có câu nào sửa được." className="py-6" />
      ) : (
        <div className="space-y-2.5">
          {loaded.r.duLieu.map((m) => (
            <SystemMessageCard
              key={`${m.code}:${loaded.n}`}
              module={module}
              initial={m}
              onRestored={() => setReload((n) => n + 1)}
            />
          ))}
        </div>
      )}
      {loaded !== null && loaded.n !== reload && (
        <p role="status" className="text-ink-muted m-0 mt-2 text-[11px]">
          Đang đọc lại lời hệ thống…
        </p>
      )}
    </section>
  );
}

function SystemMessageCard({
  module,
  initial,
  onRestored,
}: {
  module: SystemMessageModule;
  initial: SystemMessage;
  onRestored: () => void;
}) {
  const [message, setMessage] = useState(initial);
  const [draft, setDraft] = useState(initial.current_text);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    if (busy) return;
    setBusy(true);
    setError(null);
    const r = await saveMessageFlow(module, message.code, draft);
    setBusy(false);
    if (!r.ok) {
      // The draft stays: the refusal is about what was typed, and retyping it is the fix.
      setError(r.thongBao);
      return;
    }
    setMessage(r.duLieu);
    setDraft(r.duLieu.current_text);
    toast.success(SAVED_SENTENCE);
  }

  async function restore() {
    if (busy) return;
    setBusy(true);
    setError(null);
    const r = await restoreMessageFlow(module, message.code);
    setBusy(false);
    if (!r.ok) {
      setError(r.thongBao);
      return;
    }
    toast.success(RESTORED_SENTENCE);
    // The section re-reads and remounts this card on the server's state.
    onRestored();
  }

  return (
    <SystemMessageCardView
      message={message}
      draft={draft}
      busy={busy}
      error={error}
      onDraft={(s) => {
        setDraft(s);
        setError(null);
      }}
      onSave={() => void save()}
      onRestore={() => void restore()}
    />
  );
}

/** Pure rendering of one sentence, exported so the tests read its markup. */
export function SystemMessageCardView({
  message: m,
  draft,
  busy,
  error,
  onDraft,
  onSave,
  onRestore,
}: {
  message: SystemMessage;
  draft: string;
  busy: boolean;
  error: string | null;
  onDraft: (s: string) => void;
  onSave: () => void;
  onRestore: () => void;
}) {
  const inputId = `loi-he-thong-sua-${m.code}`;
  const errorId = `${inputId}-loi`;
  return (
    <article className="border-line rounded-[10px] border border-solid bg-white p-3" aria-label={m.code}>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <code className="text-ink-muted border-line rounded border border-solid bg-[#F7FAFC] px-1.5 py-0.5 text-[10.5px]">
          {m.code}
        </code>
        {/* ATTRIBUTES (origin, wording), not statuses: text-only pills like the prototype (:145, :154)
            and the "Mặc định" pill of tab-danh-muc — no tone icon. Spec `bg-surface` is the page
            colour, `bg-background` in this app (config-ui TOKEN TRAP). */}
        <span className={cn(PILL_CLASS, "bg-background text-ink border-line")}>{SHIPPED_BADGE}</span>
        {m.overridden && (
          <span className={cn(PILL_CLASS, "bg-tangerine/12 text-tangerine border-tangerine/25")}>
            {OVERRIDDEN_BADGE}
          </span>
        )}
      </div>
      <p className="text-ink-muted m-0 mb-2 text-[11.5px]">{m.description}</p>
      <form
        className="m-0"
        aria-label={`Sửa lời câu ${m.code}`}
        onSubmit={(e) => {
          e.preventDefault();
          if (isTextChanged(draft, m.current_text)) onSave();
        }}
      >
        <label htmlFor={inputId} className="an-thi-giac">
          Nội dung câu {m.code}
        </label>
        <textarea
          id={inputId}
          rows={2}
          value={draft}
          maxLength={SYSTEM_MESSAGE_MAX}
          disabled={busy}
          aria-invalid={error !== null ? true : undefined}
          aria-describedby={error !== null ? errorId : undefined}
          onChange={(e) => onDraft(e.target.value)}
          className={cn(controlClass, "h-auto py-2 text-[12.5px]")}
        />
        {error !== null && (
          // Our sentence (empty, too long) or the server's 400 sentence, verbatim.
          <p id={errorId} role="alert" className="text-danger m-0 mt-1.5 text-[12px] font-medium">
            {error}
          </p>
        )}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button
            type="submit"
            variant="primary"
            size="sm"
            className={SMALL_BUTTON_CLASS}
            disabled={busy || !isTextChanged(draft, m.current_text)}
            aria-busy={busy}
          >
            {SAVE_BUTTON}
          </Button>
          {/* Only an overridden sentence has anything to restore. Runs on click, as the prototype does. */}
          {m.overridden && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={SMALL_BUTTON_CLASS}
              disabled={busy}
              aria-busy={busy}
              icon={<RotateCcw aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={onRestore}
            >
              {RESTORE_BUTTON}
            </Button>
          )}
          {/* Only a sentence carrying the commune's own wording has anything to switch off (ADR 0079
              lô 3); a shipped sentence in force cannot be off. Commune-added sentences join later. */}
          {m.overridden && SWITCH_OFF_ENTRY !== undefined && (
            // The wrapper takes `className`; `[&>button]` reaches the button inside it.
            <PendingButton info={SWITCH_OFF_ENTRY} variant="outline" size="sm" className="[&>button]:min-h-0">
              {SWITCH_OFF_BUTTON}
            </PendingButton>
          )}
        </div>
      </form>
    </article>
  );
}
