"use client";

import { useEffect, useState } from "react";

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
 * (an empty refusal is one nobody can act on). "Khôi phục câu mặc định" is the only way back.
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
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền sửa lời hệ thống, nên tab này không hiển thị.
      </p>
    );
  }

  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-loi-he-thong">
      <h2 id="tieu-de-loi-he-thong">{SYSTEM_MESSAGES_TITLE}</h2>
      <p className="ghi-chu">{SYSTEM_MESSAGES_GUIDANCE}</p>
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
    <section aria-labelledby={headingId}>
      <h3 id={headingId}>{title}</h3>
      {note !== undefined && <p className="canh-bao-pham-vi">{note}</p>}
      {loaded === null ? (
        <p role="status">Đang tải lời hệ thống…</p>
      ) : !loaded.r.ok ? (
        <p className="thong-bao-loi" role="alert">
          {loaded.r.thongBao}
        </p>
      ) : loaded.r.duLieu.length === 0 ? (
        <p className="trang-thai-rong">Phân hệ này chưa có câu nào sửa được.</p>
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
      {loaded !== null && loaded.n !== reload && <p role="status">Đang đọc lại lời hệ thống…</p>}
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
    <article className="the-loi-he-thong" aria-label={m.code}>
      <p>
        <span className="ma-muc">{m.code}</span>{" "}
        {m.overridden && <span className="chip chip-hoat-dong">{OVERRIDDEN_BADGE}</span>}{" "}
        {MESSAGES_NOT_RAISED_YET.has(m.code) && <span className="chip chip-ngung">{NOT_RAISED_NOTE}</span>}
      </p>
      <p className="ghi-chu">{m.description}</p>
      <dl>
        <dt>Câu mặc định của phần mềm</dt>
        <dd>{m.default_text}</dd>
        <dt>Câu đang dùng</dt>
        <dd>{m.current_text}</dd>
      </dl>
      {edited !== null && <p className="ghi-chu">{edited}</p>}

      {mode === "edit" && (
        <form
          className="form-danh-muc"
          aria-label={`Sửa lời câu ${m.code}`}
          onSubmit={(e) => {
            e.preventDefault();
            onSave();
          }}
        >
          <fieldset disabled={busy}>
            <div className="o-nhap">
              <label htmlFor={inputId}>Câu của xã</label>
              <textarea
                id={inputId}
                rows={3}
                value={draft}
                maxLength={SYSTEM_MESSAGE_MAX}
                onChange={(e) => onDraft(e.target.value)}
              />
            </div>
            <div className="cum-nut">
              <button type="submit" className="nut-chinh">
                {SAVE_BUTTON}
              </button>
              <button type="button" className="nut-phu" onClick={onCancel}>
                {CANCEL_BUTTON}
              </button>
            </div>
          </fieldset>
        </form>
      )}

      {mode === "confirm-restore" && (
        <div className="cum-nut" role="group" aria-label="Xác nhận khôi phục câu mặc định">
          <p>{RESTORE_CONFIRM}</p>
          <button type="button" className="nut-chinh" disabled={busy} onClick={onRestore}>
            {RESTORE_CONFIRM_BUTTON}
          </button>
          <button type="button" className="nut-phu" disabled={busy} onClick={onCancel}>
            {CANCEL_BUTTON}
          </button>
        </div>
      )}

      {mode === "view" && (
        <div className="cum-nut">
          <button type="button" className="nut-phu" onClick={onEdit}>
            {EDIT_BUTTON}
          </button>
          {/* Only an overridden sentence has anything to restore. */}
          {m.overridden && (
            <button type="button" className="nut-phu" onClick={onAskRestore}>
              {RESTORE_BUTTON}
            </button>
          )}
        </div>
      )}

      {notice !== null &&
        (notice.ok ? (
          <p role="status">{notice.text}</p>
        ) : (
          // Server 400 sentences (markup, control characters…) arrive here verbatim.
          <p className="thong-bao-loi" role="alert">
            {notice.text}
          </p>
        ))}
    </article>
  );
}
