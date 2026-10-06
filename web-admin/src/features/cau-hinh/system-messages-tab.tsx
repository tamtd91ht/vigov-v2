"use client";

import { MessageSquareText, Pencil, RotateCcw, TriangleAlert } from "lucide-react";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { NoAccess } from "@/components/ui/no-access";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";

import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  listSystemMessages,
  type SystemMessage,
  type SystemMessageModule,
} from "@/lib/api/system-messages";

import { systemMessagesTabDecision } from "./quyen-tab";
import {
  CANCEL_BUTTON,
  EDIT_BUTTON,
  lastEditLine,
  MESSAGES_NOT_RAISED_YET,
  NOT_RAISED_NOTE,
  OVERRIDDEN_BADGE,
  RESTORE_BUTTON,
  RESTORE_CONFIRM,
  RESTORE_CONFIRM_BUTTON,
  RESTORED_SENTENCE,
  restoreMessageFlow,
  SAVE_BUTTON,
  SAVED_SENTENCE,
  saveMessageFlow,
  SYSTEM_MESSAGE_MAX,
  SYSTEM_MESSAGE_SECTIONS,
  SYSTEM_MESSAGES_GUIDANCE,
  SYSTEM_MESSAGES_TITLE,
} from "./system-message-form";

/**
 * "Cấu hình → Lời hệ thống" (§7). One key, `admin.lookup`, on all nine routes — the tab hides as a
 * whole without it (convenience; the server refuses).
 *
 * WHAT §7 DRAWS AND THIS DOES NOT: `+ Thêm câu mới` and `[Tắt]`. The catalogue is closed and lives
 * in each service's code — a new key is a release, not a row — and a refusal cannot be switched off
 * (an empty refusal is one nobody can act on). "Khôi phục lời gốc" is the only way back.
 *
 * Each section loads and fails on its own: one service being down must not hide the other two.
 */
export function SystemMessagesTab() {
  const phien = usePhien();
  const decision = phien === null ? null : systemMessagesTabDecision(phien);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền sửa lời hệ thống, nên tab này không hiển thị.
        </p>
      </div>
    );
  }

  return (
    // The prototype's `MessageTemplateTable` (ADR 0068 lần 5): the guidance as plain text on top, then
    // one titled group per module, each a stack of sentence cards. No outer card, no visible tab title.
    <section className="tab-danh-muc flex min-w-0 flex-col gap-5 [&>*]:my-0" aria-labelledby="tieu-de-loi-he-thong">
      <h2 id="tieu-de-loi-he-thong" className="an-thi-giac">
        {SYSTEM_MESSAGES_TITLE}
      </h2>
      <p className="ghi-chu m-0 max-w-2xl text-[13px] text-ink-500">{SYSTEM_MESSAGES_GUIDANCE}</p>
      {SYSTEM_MESSAGE_SECTIONS.map((s) => (
        <SystemMessageSection key={s.module} module={s.module} title={s.title} note={s.note} />
      ))}
    </section>
  );
}

function SystemMessageSection({
  module,
  title,
  note,
}: {
  module: SystemMessageModule;
  title: string;
  note?: string;
}) {
  // After a restore the list is RE-READ (DELETE answers 204, and the server is the state). Cards are
  // keyed by the read that produced them, so they remount on the new data; the restored card's
  // confirmation lives here, because the card that showed it is gone.
  const [reload, setReload] = useState(0);
  const [restored, setRestored] = useState<string | null>(null);
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
    <section className="m-0 flex min-w-0 flex-col gap-2.5 [&>*]:my-0" aria-labelledby={headingId}>
      <h3 id={headingId} className="flex items-center gap-2 text-[13px] font-bold text-ink-900">
        {title}
      </h3>
      {note !== undefined && (
        <Notice tone="neutral" icon={TriangleAlert} className="canh-bao-pham-vi">
          {note}
        </Notice>
      )}
      {loaded === null ? (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải lời hệ thống…
          </p>
          <SkeletonRows rows={3} columns={2} className="rounded-xl border border-line" />
        </>
      ) : !loaded.r.ok ? (
        <ErrorState role="alert" title="Chưa tải được lời hệ thống" message={loaded.r.thongBao} className="py-6" />
      ) : loaded.r.duLieu.length === 0 ? (
        <EmptyState icon={MessageSquareText} title="Phân hệ này chưa có câu nào sửa được." className="py-6" />
      ) : (
        loaded.r.duLieu.map((m) => (
          <SystemMessageCard
            key={`${m.code}:${loaded.n}`}
            module={module}
            initial={m}
            restoredNote={restored === m.code}
            onRestored={() => {
              setRestored(m.code);
              setReload((n) => n + 1);
            }}
          />
        ))
      )}
      {loaded !== null && loaded.n !== reload && (
        <p role="status" className="text-[13px] text-ink-500">
          Đang đọc lại lời hệ thống…
        </p>
      )}
    </section>
  );
}

export type CardMode = "view" | "edit" | "confirm-restore";

function SystemMessageCard({
  module,
  initial,
  restoredNote,
  onRestored,
}: {
  module: SystemMessageModule;
  initial: SystemMessage;
  restoredNote: boolean;
  onRestored: () => void;
}) {
  const [message, setMessage] = useState(initial);
  const [mode, setMode] = useState<CardMode>("view");
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ ok: boolean; text: string } | null>(
    restoredNote ? { ok: true, text: RESTORED_SENTENCE } : null,
  );

  async function save() {
    if (busy) return;
    setBusy(true);
    setNotice(null);
    const r = await saveMessageFlow(module, message.code, draft);
    setBusy(false);
    if (!r.ok) {
      // The draft stays: the refusal is about what was typed, and retyping it is the fix.
      setNotice({ ok: false, text: r.thongBao });
      return;
    }
    setMessage(r.duLieu);
    setMode("view");
    setNotice({ ok: true, text: SAVED_SENTENCE });
  }

  async function restore() {
    if (busy) return;
    setBusy(true);
    setNotice(null);
    const r = await restoreMessageFlow(module, message.code);
    setBusy(false);
    if (!r.ok) {
      setNotice({ ok: false, text: r.thongBao });
      return;
    }
    // The section re-reads and remounts this card on the server's state, with the confirmation.
    onRestored();
  }

  return (
    <SystemMessageCardView
      message={message}
      mode={mode}
      draft={draft}
      busy={busy}
      notice={notice}
      onEdit={() => {
        setDraft(message.current_text);
        setNotice(null);
        setMode("edit");
      }}
      onDraft={setDraft}
      onSave={() => void save()}
      onCancel={() => setMode("view")}
      onAskRestore={() => {
        setNotice(null);
        setMode("confirm-restore");
      }}
      onRestore={() => void restore()}
    />
  );
}

/** Pure rendering of one sentence, exported so the tests read each mode's markup. */
export function SystemMessageCardView({
  message: m,
  mode,
  draft,
  busy,
  notice,
  onEdit,
  onDraft,
  onSave,
  onCancel,
  onAskRestore,
  onRestore,
}: {
  message: SystemMessage;
  mode: CardMode;
  draft: string;
  busy: boolean;
  notice: { ok: boolean; text: string } | null;
  onEdit: () => void;
  onDraft: (s: string) => void;
  onSave: () => void;
  onCancel: () => void;
  onAskRestore: () => void;
  onRestore: () => void;
}) {
  const inputId = `loi-he-thong-sua-${m.code}`;
  const edited = lastEditLine(m);
  return (
    <article
      className="the-loi-he-thong m-0 flex min-w-0 flex-col gap-2 rounded-[10px] border border-line bg-surface p-3 [&>*]:my-0"
      aria-label={m.code}
    >
      <p className="flex flex-wrap items-center gap-2">
        {/* The code as a small bordered chip, as the prototype draws it. */}
        <span className="ma-muc rounded border border-line bg-surface-muted px-1.5 py-0.5 text-[11px] text-ink-500">{m.code}</span>{" "}
        {/* Tone by the CODE (`overridden`, the not-raised set); icon + word, never colour alone. */}
        {m.overridden && <Badge tone="info" icon={Pencil}>{OVERRIDDEN_BADGE}</Badge>}{" "}
        {MESSAGES_NOT_RAISED_YET.has(m.code) && <Badge tone="neutral">{NOT_RAISED_NOTE}</Badge>}
      </p>
      <p className="ghi-chu text-xs text-ink-500">{m.description}</p>
      <dl className="grid gap-x-4 gap-y-1 sm:grid-cols-[minmax(0,12rem)_minmax(0,1fr)] [&_dd]:m-0 [&_dt]:text-xs [&_dt]:font-semibold [&_dt]:text-ink-700">
        <dt>Câu mặc định của phần mềm</dt>
        <dd>{m.default_text}</dd>
        <dt>Câu đang dùng</dt>
        <dd>{m.current_text}</dd>
      </dl>
      {edited !== null && <p className="ghi-chu text-xs text-ink-500">{edited}</p>}

      {mode === "edit" && (
        <form
          className="form-danh-muc m-0"
          aria-label={`Sửa lời câu ${m.code}`}
          onSubmit={(e) => {
            e.preventDefault();
            onSave();
          }}
        >
          <fieldset disabled={busy} className="m-0 flex min-w-0 flex-col gap-3 border-0 p-0">
            <div className="o-nhap m-0">
              <label htmlFor={inputId}>Câu của xã</label>
              <textarea
                id={inputId}
                rows={3}
                value={draft}
                maxLength={SYSTEM_MESSAGE_MAX}
                onChange={(e) => onDraft(e.target.value)}
              />
            </div>
            <div className="cum-nut flex flex-wrap justify-end gap-2">
              <Button type="submit" variant="primary" aria-busy={busy}>
                {SAVE_BUTTON}
              </Button>
              <Button type="button" variant="secondary" onClick={onCancel}>
                {CANCEL_BUTTON}
              </Button>
            </div>
          </fieldset>
        </form>
      )}

      {mode === "confirm-restore" && (
        <ConfirmDialog
          className="cum-nut m-0"
          role="group"
          aria-label="Xác nhận khôi phục lời gốc"
          icon={RotateCcw}
          title="Xác nhận khôi phục lời gốc"
          titleAs="h4"
          actions={
            <>
              <Button type="button" variant="primary" disabled={busy} aria-busy={busy} onClick={onRestore}>
                {RESTORE_CONFIRM_BUTTON}
              </Button>
              <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
                {CANCEL_BUTTON}
              </Button>
            </>
          }
        >
          <p className="m-0">{RESTORE_CONFIRM}</p>
        </ConfirmDialog>
      )}

      {mode === "view" && (
        <div className="cum-nut flex flex-wrap gap-2">
          <Button type="button" variant="primary" size="sm" icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />} onClick={onEdit}>
            {EDIT_BUTTON}
          </Button>
          {/* Only an overridden sentence has anything to restore. */}
          {m.overridden && (
            <Button type="button" variant="outline" size="sm" icon={<RotateCcw aria-hidden="true" focusable="false" strokeWidth={1.8} />} onClick={onAskRestore}>
              {RESTORE_BUTTON}
            </Button>
          )}
        </div>
      )}

      {notice !== null &&
        (notice.ok ? (
          <p role="status" className="text-sm font-medium text-success-600">
            {notice.text}
          </p>
        ) : (
          // Server 400 sentences (markup, control characters…) arrive here verbatim.
          <p className="thong-bao-loi" role="alert">
            {notice.text}
          </p>
        ))}
    </article>
  );
}
