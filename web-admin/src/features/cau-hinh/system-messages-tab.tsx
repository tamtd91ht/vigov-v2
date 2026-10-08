"use client";

import { MessageSquareText, Plus, RotateCcw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";

import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import {
  listSystemMessages,
  type SystemMessage,
  type SystemMessageModule,
} from "@/lib/api/system-messages";

import {
  ConfigField,
  ConfigFormRow,
  ConfigLoading,
  SMALL_BUTTON_CLASS,
  formInputCls,
} from "./config-ui";
import { systemMessagesTabDecision } from "./quyen-tab";
import {
  ADD_BUTTON,
  ADD_CODE_LABEL,
  ADD_CODE_PLACEHOLDER,
  ADD_DESCRIPTION_LABEL,
  ADD_SUBMIT_BUTTON,
  ADD_TEXT_LABEL,
  ADDED_SENTENCE,
  addMessageFlow,
  type AddDraft,
  canSwitch,
  COMMUNE_BADGE,
  DELETE_BUTTON,
  DELETE_REASON_MAX,
  DELETED_SENTENCE,
  deleteMessageFlow,
  deleteReasonLabel,
  editableText,
  emptyAddDraft,
  FORM_CANCEL_BUTTON,
  isCommune,
  isTextChanged,
  OVERRIDDEN_BADGE,
  RESTORE_BUTTON,
  RESTORED_SENTENCE,
  restoreMessageFlow,
  SAVE_BUTTON,
  SAVED_SENTENCE,
  saveMessageFlow,
  sectionMessages,
  SHIPPED_BADGE,
  SWITCH_OFF_BUTTON,
  SWITCH_ON_BUTTON,
  SWITCHED_OFF_BADGE,
  SWITCHED_OFF_COMMUNE_BADGE,
  switchMessageFlow,
  SYSTEM_MESSAGE_MAX,
  SYSTEM_MESSAGE_SECTIONS,
  SYSTEM_MESSAGES_GUIDANCE,
  SYSTEM_MESSAGES_TITLE,
} from "./system-message-form";

/**
 * "Cấu hình → Lời hệ thống" — spec `07-loi-he-thong.md`, prototype `MessageTemplateTable.tsx`
 * (ADR 0079 Q2/Q5). One key, `admin.lookup`, on every route — the tab hides as a whole without it
 * (convenience; the server refuses).
 *
 * WHAT A CARD OFFERS FOLLOWS ITS ORIGIN (ADR 0079 Q2, lô 3 "Khi nào hiện Tắt"):
 *   shipped    Lưu · Khôi phục lời gốc (when reworded) · Tắt/Bật lại (when reworded). Never Xoá.
 *   commune    Lưu · Tắt/Bật lại · Xoá (soft, with a reason — rule 7). Never Khôi phục: no lời gốc.
 *
 * COMMUNE SENTENCES ARE STORED AND MANAGED ONLY (Q5b) — nothing shows them yet. The card does NOT say
 * so: lô 3 Q7(b) removed the "Chưa có chức năng nào dùng câu này" note and its consequences table states
 * Q7 wins over Q5b on this point of presentation.
 *
 * NO LAST-EDIT LINE, NO GROUP NOTICE, NO RESTORE CONFIRMATION: owner, 08/10/2026, "Bỏ hết, đúng
 * prototype". Who changed a sentence and when stays in the audit trail.
 *
 * Each service loads and fails on its own: one service being down must not hide the other two.
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
  // A separate component so no list is requested before the permission is known (hooks run there).
  return <SystemMessagesBody />;
}

type ModuleList = {
  /** The read that produced `r`; cards are keyed by it, so they remount on a re-read. */
  n: number;
  r: KetQua<readonly SystemMessage[]>;
};

/**
 * One service's list. After a restore, an add or a delete the list is RE-READ (those answer 204 or one
 * row, and the server is the state); `reloading` is true until the new read lands.
 */
function useModuleList(module: SystemMessageModule) {
  const [reload, setReload] = useState(0);
  const [loaded, setLoaded] = useState<ModuleList | null>(null);
  useEffect(() => {
    let gone = false;
    void listSystemMessages(module).then((r) => {
      if (!gone) setLoaded({ n: reload, r });
    });
    return () => {
      gone = true;
    };
  }, [module, reload]);
  return {
    loaded,
    reloading: loaded !== null && loaded.n !== reload,
    reload: () => setReload((n) => n + 1),
  };
}

function SystemMessagesBody() {
  // Called in a fixed order — one hook per service, never in a loop.
  const lists = {
    petitions: useModuleList("petitions"),
    finance: useModuleList("finance"),
    reporting: useModuleList("reporting"),
  } satisfies Record<SystemMessageModule, ReturnType<typeof useModuleList>>;
  const [adding, setAdding] = useState(false);

  return (
    <section className="min-w-0" aria-labelledby="tieu-de-loi-he-thong">
      <h2 id="tieu-de-loi-he-thong" className="an-thi-giac">
        {SYSTEM_MESSAGES_TITLE}
      </h2>
      <SystemMessagesHeader adding={adding} onToggleAdd={() => setAdding((open) => !open)} />
      {adding && (
        <AddMessageForm
          onCancel={() => setAdding(false)}
          onAdded={(module) => {
            setAdding(false);
            lists[module].reload();
          }}
        />
      )}
      {SYSTEM_MESSAGE_SECTIONS.map((s) => {
        const list = lists[s.module];
        return (
          <SystemMessageSection
            key={s.group}
            section={s}
            loaded={list.loaded}
            reloading={list.reloading}
            onChanged={list.reload}
          />
        );
      })}
    </section>
  );
}

/** The tab's head row: guidance and "Thêm câu mới" (default size, not sm — spec 07). Exported for tests. */
export function SystemMessagesHeader({ adding, onToggleAdd }: { adding: boolean; onToggleAdd: () => void }) {
  return (
    <div className="mb-4 flex flex-wrap items-start gap-3">
      <p className="text-ink-muted m-0 max-w-2xl text-[12.5px]">{SYSTEM_MESSAGES_GUIDANCE}</p>
      <Button
        type="button"
        variant="primary"
        className="ml-auto"
        aria-expanded={adding}
        icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
        onClick={onToggleAdd}
      >
        {ADD_BUTTON}
      </Button>
    </div>
  );
}

function SystemMessageSection({
  section,
  loaded,
  reloading,
  onChanged,
}: {
  section: (typeof SYSTEM_MESSAGE_SECTIONS)[number];
  loaded: ModuleList | null;
  reloading: boolean;
  onChanged: () => void;
}) {
  const items = loaded !== null && loaded.r.ok ? sectionMessages(section, loaded.r.duLieu) : [];
  // A secondary section ("Dùng chung") is drawn only when it holds sentences; loading, error and empty
  // belong to the service's primary section.
  if (!section.primary && items.length === 0) return null;

  const headingId = `loi-he-thong-${section.group}`;
  return (
    <section className="mt-5 min-w-0 first-of-type:mt-0" aria-labelledby={headingId}>
      <h3 id={headingId} className="text-navy m-0 mb-2.5 text-[12.5px] font-bold">
        {section.title}
      </h3>
      {loaded === null ? (
        <ConfigLoading label="Đang tải lời hệ thống…" />
      ) : !loaded.r.ok ? (
        <ErrorState role="alert" title="Chưa tải được lời hệ thống" message={loaded.r.thongBao} className="py-6" />
      ) : items.length === 0 ? (
        <EmptyState icon={MessageSquareText} title="Phân hệ này chưa có câu nào sửa được." className="py-6" />
      ) : (
        <div className="space-y-2.5">
          {items.map((m) => (
            <SystemMessageCard key={`${m.code}:${loaded.n}`} module={section.module} initial={m} onChanged={onChanged} />
          ))}
        </div>
      )}
      {reloading && section.primary && (
        <p role="status" className="text-ink-muted m-0 mt-2 text-[11px]">
          Đang đọc lại lời hệ thống…
        </p>
      )}
    </section>
  );
}

/* ---- add -------------------------------------------------------------------------------------- */

function AddMessageForm({
  onCancel,
  onAdded,
}: {
  onCancel: () => void;
  onAdded: (module: SystemMessageModule) => void;
}) {
  const [draft, setDraft] = useState<AddDraft>(emptyAddDraft);
  // Minted when the form OPENS and kept across a retry: a second click after a lost answer is the same
  // add, not a second sentence. The form unmounts on success, so the next add gets a new key.
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (sending) return;
    setError(null);
    setSending(true);
    const r = await addMessageFlow(draft, idempotencyKey);
    setSending(false);
    if (!r.ok) {
      // Ours (missing field, unknown prefix) or the server's sentence verbatim — 409 message_code_taken
      // already reads "Mã này đã được dùng cho một câu khác.".
      setError(r.thongBao);
      return;
    }
    toast.success(ADDED_SENTENCE);
    onAdded(r.duLieu.module);
  }

  return (
    <AddMessageFormView
      draft={draft}
      sending={sending}
      error={error}
      onDraft={(d) => {
        setDraft(d);
        setError(null);
      }}
      onSubmit={() => void submit()}
      onCancel={onCancel}
    />
  );
}

/**
 * The grey add form (spec 07 "Form thêm câu"), pure rendering — exported for tests. Spec `bg-surface` is
 * the page colour, `bg-background` here (config-ui TOKEN TRAP). No group control, as the prototype: the
 * group — and the service that stores the sentence — is the code's prefix (`groupOfCode`).
 */
export function AddMessageFormView({
  draft,
  sending,
  error,
  onDraft,
  onSubmit,
  onCancel,
}: {
  draft: AddDraft;
  sending: boolean;
  error: string | null;
  onDraft: (d: AddDraft) => void;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const errorId = "loi-he-thong-them-loi";
  const describedBy = error !== null ? errorId : undefined;
  return (
    <form
      className="border-line bg-background mb-4 rounded-[10px] border border-solid p-3"
      aria-label="Thêm câu hệ thống của xã"
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <div className="grid gap-3 sm:grid-cols-[14rem_minmax(0,1fr)]">
        <ConfigField label={ADD_CODE_LABEL} htmlFor="loi-he-thong-them-ma">
          <input
            id="loi-he-thong-them-ma"
            className={formInputCls}
            value={draft.code}
            placeholder={ADD_CODE_PLACEHOLDER}
            autoComplete="off"
            spellCheck={false}
            disabled={sending}
            aria-describedby={describedBy}
            onChange={(e) => onDraft({ ...draft, code: e.target.value })}
          />
        </ConfigField>
        <ConfigField label={ADD_DESCRIPTION_LABEL} htmlFor="loi-he-thong-them-giai-thich">
          <input
            id="loi-he-thong-them-giai-thich"
            className={formInputCls}
            value={draft.description}
            maxLength={SYSTEM_MESSAGE_MAX}
            disabled={sending}
            onChange={(e) => onDraft({ ...draft, description: e.target.value })}
          />
        </ConfigField>
      </div>
      <ConfigField label={ADD_TEXT_LABEL} htmlFor="loi-he-thong-them-noi-dung" className="mt-3">
        <textarea
          id="loi-he-thong-them-noi-dung"
          rows={2}
          value={draft.text}
          maxLength={SYSTEM_MESSAGE_MAX}
          disabled={sending}
          aria-describedby={describedBy}
          onChange={(e) => onDraft({ ...draft, text: e.target.value })}
          className={cn(controlClass, "mt-1 h-auto py-2 text-[12.5px]")}
        />
      </ConfigField>
      {error !== null && (
        <p id={errorId} role="alert" className="text-danger m-0 mt-2 text-[12px] font-medium">
          {error}
        </p>
      )}
      <div className="mt-3 flex gap-2">
        <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label={ADD_SUBMIT_BUTTON} busyText={BUSY_SAVING} />
        </Button>
        <Button type="button" variant="outline" disabled={sending} onClick={onCancel}>
          {FORM_CANCEL_BUTTON}
        </Button>
      </div>
    </form>
  );
}

/* ---- one card --------------------------------------------------------------------------------- */

function SystemMessageCard({
  module,
  initial,
  onChanged,
}: {
  module: SystemMessageModule;
  initial: SystemMessage;
  onChanged: () => void;
}) {
  const [message, setMessage] = useState(initial);
  const [draft, setDraft] = useState(editableText(initial));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // `null` = the delete step is closed; otherwise the reason being typed.
  const [deleteReason, setDeleteReason] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  async function save() {
    if (busy) return;
    setBusy(true);
    setError(null);
    const r = await saveMessageFlow(module, message, draft);
    setBusy(false);
    if (!r.ok) {
      // The draft stays: the refusal is about what was typed, and retyping it is the fix.
      setError(r.thongBao);
      return;
    }
    setMessage(r.duLieu);
    setDraft(editableText(r.duLieu));
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
    onChanged();
  }

  async function toggle() {
    if (busy) return;
    setBusy(true);
    setError(null);
    const r = await switchMessageFlow(module, message);
    setBusy(false);
    if (!r.ok) {
      setError(r.thongBao);
      return;
    }
    // The words being typed are kept: the switch and the words are two acts on the server too.
    // No toast: the switch changing IS the confirmation (prototype `MessageTemplateTable.tsx:107-116`
    // toasts only the error; ADR 0079 lô 6 #3). A refusal still shows, in place on the card.
    setMessage(r.duLieu);
  }

  async function remove() {
    if (busy || deleteReason === null || module === "reporting") return;
    setDeleteError(null);
    setBusy(true);
    const r = await deleteMessageFlow(module, message.code, deleteReason);
    setBusy(false);
    if (!r.ok) {
      setDeleteError(r.thongBao);
      return;
    }
    toast.success(DELETED_SENTENCE);
    onChanged();
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
      onToggle={() => void toggle()}
      deleteReason={deleteReason}
      deleteError={deleteError}
      onDeleteStart={() => {
        setDeleteReason("");
        setDeleteError(null);
      }}
      onDeleteReason={(s) => {
        setDeleteReason(s);
        setDeleteError(null);
      }}
      onDeleteSubmit={() => void remove()}
      onDeleteCancel={() => {
        setDeleteReason(null);
        setDeleteError(null);
      }}
    />
  );
}

/** The shared Badge's pill box, without its tone icon (same as tab-danh-muc's "Mặc định" pill). */
const PILL_CLASS =
  "inline-flex h-5 w-fit shrink-0 items-center rounded-4xl border border-solid px-2 py-0.5 text-xs leading-none font-medium whitespace-nowrap";

/** Pure rendering of one sentence, exported so the tests read its markup. */
export function SystemMessageCardView({
  message: m,
  draft,
  busy,
  error,
  onDraft,
  onSave,
  onRestore,
  onToggle,
  deleteReason = null,
  deleteError = null,
  onDeleteStart,
  onDeleteReason,
  onDeleteSubmit,
  onDeleteCancel,
}: {
  message: SystemMessage;
  draft: string;
  busy: boolean;
  error: string | null;
  onDraft: (s: string) => void;
  onSave: () => void;
  onRestore: () => void;
  onToggle: () => void;
  /** `null` = the delete step is closed. */
  deleteReason?: string | null;
  deleteError?: string | null;
  onDeleteStart: () => void;
  onDeleteReason: (s: string) => void;
  onDeleteSubmit: () => void;
  onDeleteCancel: () => void;
}) {
  const inputId = `loi-he-thong-sua-${m.code}`;
  const errorId = `${inputId}-loi`;
  const reasonId = `loi-he-thong-ly-do-xoa-${m.code}`;
  const commune = isCommune(m);
  const stored = editableText(m);
  return (
    <article
      className={cn("border-line rounded-[10px] border border-solid bg-white p-3", !m.is_active && "opacity-60")}
      aria-label={m.code}
    >
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <code className="text-ink-muted border-line rounded border border-solid bg-[#F7FAFC] px-1.5 py-0.5 text-[10.5px]">
          {m.code}
        </code>
        {/* ATTRIBUTES (origin, wording, switch), not statuses: text-only pills like the prototype
            (:144-162) and the "Mặc định" pill of tab-danh-muc — no tone icon. Spec `bg-surface` is the
            page colour, `bg-background` in this app (config-ui TOKEN TRAP). */}
        {commune ? (
          <span className={cn(PILL_CLASS, "bg-brand/12 text-brand border-brand/25")}>{COMMUNE_BADGE}</span>
        ) : (
          <span className={cn(PILL_CLASS, "bg-background text-ink border-line")}>{SHIPPED_BADGE}</span>
        )}
        {!commune && m.overridden && (
          <span className={cn(PILL_CLASS, "bg-tangerine/12 text-tangerine border-tangerine/25")}>
            {OVERRIDDEN_BADGE}
          </span>
        )}
        {!m.is_active && (
          <span className={cn(PILL_CLASS, "bg-ink-muted/12 text-ink border-line")}>
            {commune ? SWITCHED_OFF_COMMUNE_BADGE : SWITCHED_OFF_BADGE}
          </span>
        )}
      </div>
      {/* A commune sentence may have no description; the prototype draws nothing then. */}
      {m.description !== "" && <p className="text-ink-muted m-0 mb-2 text-[11.5px]">{m.description}</p>}
      <form
        className="m-0"
        aria-label={`Sửa lời câu ${m.code}`}
        onSubmit={(e) => {
          e.preventDefault();
          if (isTextChanged(draft, stored)) onSave();
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
          // Our sentence (empty, too long) or the server's sentence, verbatim.
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
            disabled={busy || !isTextChanged(draft, stored)}
            aria-busy={busy}
          >
            {SAVE_BUTTON}
          </Button>
          {/* Only a reworded shipped sentence has a lời gốc to go back to. Runs on click, as the prototype does. */}
          {!commune && m.overridden && (
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
          {/* Only a sentence carrying the commune's own wording has anything to switch (ADR 0079 lô 3). */}
          {canSwitch(m) && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={SMALL_BUTTON_CLASS}
              disabled={busy}
              aria-busy={busy}
              onClick={onToggle}
            >
              {m.is_active ? SWITCH_OFF_BUTTON : SWITCH_ON_BUTTON}
            </Button>
          )}
          {/* A shipped sentence is never deleted — only reworded back (spec 07, server 409). */}
          {commune && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={cn(SMALL_BUTTON_CLASS, "text-danger ml-auto")}
              disabled={busy || deleteReason !== null}
              aria-expanded={deleteReason !== null}
              icon={<Trash2 aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={onDeleteStart}
            >
              {DELETE_BUTTON}
            </Button>
          )}
        </div>
      </form>
      {commune && deleteReason !== null && (
        // The REASON STEP (rule 7: the server soft-deletes and requires `reason`; ADR 0079 "Giữ bất kể
        // spec"), compact and inline like the other tabs' delete rows. A sibling of the edit form, never
        // nested in it. No explanatory sentence: the prototype has none (lô 3 Q7).
        <ConfigFormRow
          columns="sm:grid-cols-[minmax(0,1fr)_auto]"
          className="mt-2"
          aria-label={`Xoá câu ${m.code}`}
          // `required` stays for assistive tech; the browser's bubble is replaced by our sentence.
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            onDeleteSubmit();
          }}
          onKeyDown={(e) => {
            if (e.key === "Escape" && !busy) onDeleteCancel();
          }}
        >
          <ConfigField label={deleteReasonLabel(m.code)} htmlFor={reasonId}>
            <input
              id={reasonId}
              name="reason"
              required
              autoFocus
              maxLength={DELETE_REASON_MAX}
              className={formInputCls}
              value={deleteReason}
              disabled={busy}
              aria-invalid={deleteError !== null ? true : undefined}
              onChange={(e) => onDeleteReason(e.target.value)}
            />
          </ConfigField>
          <div className="flex gap-2">
            <Button type="submit" variant="danger" size="sm" className={SMALL_BUTTON_CLASS} disabled={busy} aria-busy={busy}>
              <BusyLabel busy={busy} label={DELETE_BUTTON} busyText={BUSY_DELETING} />
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={SMALL_BUTTON_CLASS}
              disabled={busy}
              onClick={onDeleteCancel}
            >
              {FORM_CANCEL_BUTTON}
            </Button>
          </div>
          {deleteError !== null && (
            <p role="alert" className="text-danger col-span-full m-0 text-[12px] font-medium">
              {deleteError}
            </p>
          )}
        </ConfigFormRow>
      )}
    </article>
  );
}
