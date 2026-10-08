"use client";

import { ArrowRight, CircleCheck, Pencil, Phone, Send, User, X } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { traTen } from "@/features/cau-hinh/tra-danh-muc";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import { nhanThoiDiem, staffNameWithCode } from "@/features/phan-anh/nhan-phieu";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import {
  addCitizenLetterNote,
  correctCitizenLetterSender,
  moveCitizenLetter,
  recordCitizenLetterResult,
  routeCitizenLetter,
  setCitizenLetterDeadline,
} from "@/lib/api/citizen-letters";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_citizenLetterOut,
  documents_letterLogEntryOut,
  documents_letterLogOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { RaiseTaskButton } from "./document-pending";
import { Glyph } from "./document-ui";
import {
  DUE_TONE_CLASS,
  EMPTY_ROUTING,
  EMPTY_SENDER,
  RESULT_EDITABLE_STATUSES,
  SENDER_UNKNOWN,
  WITHHELD_SENDER,
  WITHHELD_SUMMARY,
  activeDue,
  buildRouting,
  buildSenderCorrection,
  DUE_NOT_SET,
  dueDateText,
  dueInputValue,
  dueInstantOf,
  dueLabel,
  dueSettable,
  initialsOf,
  letterDate,
  letterNumber,
  letterStatusActiveTone,
  letterStatusHint,
  letterStatusLabel,
  letterTypeLabel,
  resultDraftOf,
  senderAddressText,
  senderView,
  statusStrip,
  type ResultDraft,
  type RoutingDraft,
  type SenderDraft,
  type StatusChipModel,
} from "./letter-display";
import { LetterStatusBadge } from "./letter-ui";

/** Heading id of the drawer — the dialog's accessible name. */
export const LETTER_DRAWER_TITLE_ID = "tieu-de-ngan-don-thu";

/** `Đơn số 7/2026 · nhận ngày 8/10/2026` — the prototype's first header line (`PetitionDetailDrawer.tsx:348-351`). */
export function letterDrawerTitle(number: number, year: number, receivedDate: string): string {
  return `Đơn số ${letterNumber(number, year)} · nhận ngày ${letterDate(receivedDate)}`;
}

/** Words of the drawer, the prototype's where it has them. */
export const LETTER_LOG_EMPTY = "Chưa chuyển cho bộ phận nào.";
/** The sub-line under "Chuyển thành nhiệm vụ" (`PetitionDetailDrawer.tsx:600-602`). */
export const RAISE_TASK_HINT = "Nhiệm vụ kế thừa hạn xử lý của đơn, để hai bên không lệch nhau.";
export const ROUTING_SECTION_TITLE = "Chuyển cho bộ phận khác";
export const RESULT_SECTION_TITLE = "Nội dung trả lời công dân";
export const RESULT_SAVE_LABEL = "Lưu nội dung trả lời";
/** The server's own sentence for a result outside the two statuses (`ErrLetterResultNotAllowed`). */
export const RESULT_LOCKED_HINT = "Chỉ ghi kết quả giải quyết khi đơn đang ở Thụ lý hoặc Đang giải quyết.";
const CLOSE_LABEL = "Đóng chi tiết đơn thư";
/** Not in the prototype (it has no deadline edit in the drawer) — patterned on "Sửa thông tin người gửi". */
export const DEADLINE_EDIT_LABEL = "Sửa hạn xử lý";
export const DEADLINE_SAVED = "Đã lưu hạn xử lý.";
export const DEADLINE_CLEARED = "Đã bỏ hạn xử lý.";

/**
 * ONE citizen letter — the prototype's right-hand drawer (`PetitionDetailDrawer.tsx`, 72rem), in the
 * shared `LargeDialog`, with the customer's rules where they differ (ADR 0078 #1–#4):
 *
 *   header   "Đơn số …" (the dialog's NAME — number and date, never the summary: a name enters the
 *            accessibility tree, rule 3 forbidden #4), the summary, the sender line. The phone is the
 *            server's MASKED text — NO `tel:` link (ADR 0078 consequence 1). A denunciation's sender,
 *            when withheld for this viewer, is one fixed sentence.
 *   strip    C3's main row + the "Rẽ nhánh:" row; a chip is clickable only for an arrow the SERVER
 *            lists in `next_statuses`, and only for a viewer who may work on the letter. A click opens
 *            the prototype's composer (note) and posts the status. Routing is NOT a status here.
 *   left     badges, figures (the ADDRESS IS NEVER SHOWN — only whether one exists), the clerk's "Hạn xử
 *            lý" (ADR 0079 lô 5 Q18, `petition.create`, open phases only), sender correction
 *            (an EMPTY form), "Chuyển thành nhiệm vụ" ("?"), "Chuyển cho bộ phận khác" (routing), and the
 *            result form in the place of the prototype's free-text answer (C10).
 *   right    "Nhật ký & Trao đổi" (one note) and the processing log, newest first, read-only.
 *
 * Every permission gate here is CONVENIENCE: the server checks every write (rule 5, forbidden #1).
 * Every write's outcome is said INSIDE the drawer: it is a native modal and a toast would sit under it.
 */
export function LetterDrawer({
  letter,
  log,
  now,
  units,
  directory = null,
  canBook,
  mayWork,
  onChanged,
  onClose,
}: {
  /** `null` = still reading. A refusal is shown verbatim — 404 is one sentence for every case. */
  letter: KetQua<documents_citizenLetterOut> | null;
  log: KetQua<documents_letterLogOut> | null;
  now: Date;
  units: BangTraDanhMuc;
  directory?: DanhBaTheoMa | null;
  /** `petition.create` — routing, sender correction and the deadline. */
  canBook: boolean;
  /** `petition.read` AND (assignee OR `petition.create`) — status, result, log. */
  mayWork: boolean;
  /** A write succeeded: re-read the letter, its log and the register row. */
  onChanged: () => void;
  onClose: () => void;
}) {
  const doc = letter !== null && letter.ok ? letter.duLieu : null;
  const title = doc !== null ? letterDrawerTitle(doc.number, doc.year, doc.received_date) : "Chi tiết đơn thư";
  const [saved, setSaved] = useState("");

  const done = (sentence: string) => {
    setSaved(sentence);
    onChanged();
  };

  return (
    <LargeDialog
      titleId={LETTER_DRAWER_TITLE_ID}
      onDismiss={onClose}
      className="bg-white md:w-[min(72rem,98vw)] xl:w-[min(72rem,98vw)]"
    >
      <header className="flex shrink-0 items-start gap-3 border-0 border-b border-solid border-line bg-white px-5 py-4">
        <div className="min-w-0 flex-1">
          <h2
            id={LETTER_DRAWER_TITLE_ID}
            tabIndex={-1}
            className="m-0 text-[11px] leading-snug font-semibold text-ink-muted tabular-nums outline-none"
          >
            {title}
          </h2>
          {doc !== null && <DrawerHeadline letter={doc} />}
        </div>
        <IconButton type="button" variant="secondary" size="md" label={CLOSE_LABEL} onClick={onClose}>
          <X aria-hidden="true" />
        </IconButton>
      </header>

      {letter === null && (
        <div className="p-6">
          <p role="status" className="an-thi-giac">
            Đang tải đơn thư…
          </p>
          <div aria-hidden="true" className="flex flex-col gap-3">
            <span className="block h-7 w-3/4 rounded-md bg-line motion-safe:animate-pulse" />
            <span className="block h-24 w-full rounded-md bg-line motion-safe:animate-pulse" />
          </div>
        </div>
      )}
      {letter !== null && !letter.ok && (
        <div className="p-5">
          <p className="thong-bao-loi m-0" role="alert">
            {letter.thongBao}
          </p>
        </div>
      )}

      {doc !== null && (
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:overflow-hidden">
          {mayWork && <StatusStrip key={`${doc.id}:${doc.status}`} letter={doc} onMoved={() => done("Đã cập nhật.")} />}

          <div className="flex min-w-0 flex-col md:min-h-0 md:flex-1 md:flex-row">
            <div className="min-w-0 flex-1 bg-canvas px-5 py-4 md:overflow-y-auto">
              <DrawerBadges letter={doc} now={now} />
              <dl className="m-0 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
                <Figure label="Địa chỉ người gửi">{senderAddressText(doc)}</Figure>
                {/* The unit's name only, as the prototype's figure (`:515-518`). The assignee is on the log. */}
                <Figure label="Bộ phận đang giữ">
                  {(doc.holding_unit_id ?? "") === "" ? "Chưa chuyển" : unitName(units, doc.holding_unit_id ?? "")}
                </Figure>
                {/* The stored date or "Không đặt" — the prototype's figure (`:519-524`), no count. */}
                <Figure label="Hạn xử lý">{dueDateText(activeDue(doc))}</Figure>
              </dl>

              {canBook && dueSettable(doc.status) && (
                <DeadlineEditor
                  key={`due:${doc.id}`}
                  letter={doc}
                  onSaved={(cleared) => done(cleared ? DEADLINE_CLEARED : DEADLINE_SAVED)}
                />
              )}

              {canBook && <SenderCorrection key={`sender:${doc.id}`} letter={doc} onSaved={() => done("Đã sửa thông tin người gửi.")} />}

              <div className="mt-4">
                <RaiseTaskButton />
                <p className="m-0 mt-1.5 text-[11px] text-ink-muted">{RAISE_TASK_HINT}</p>
              </div>

              {saved !== "" && (
                <p role="status" className="m-0 mt-4 flex items-center gap-2 text-[12.5px] font-medium text-success-600">
                  <Glyph icon={CircleCheck} className="size-4 shrink-0" />
                  {saved}
                </p>
              )}

              {canBook && (
                <RoutingBox
                  key={`route:${doc.id}`}
                  letterId={doc.id}
                  units={units}
                  directory={directory}
                  onRouted={() => done("Đã chuyển và ghi vết.")}
                />
              )}

              {(mayWork || hasResult(doc)) && (
                <ResultSection
                  key={`result:${doc.id}:${doc.updated_at}`}
                  letter={doc}
                  mayWork={mayWork}
                  onSaved={() => done("Đã lưu nội dung trả lời.")}
                />
              )}
            </div>

            <aside className="shrink-0 border-0 border-t border-solid border-line bg-white px-4 py-3 md:w-[24rem] md:overflow-y-auto md:border-t-0 md:border-l">
              {mayWork && <NoteComposer key={`note:${doc.id}`} letterId={doc.id} onAdded={() => done("Đã ghi nhật ký.")} />}
              <LetterLog log={log} units={units} directory={directory} />
            </aside>
          </div>
        </div>
      )}
    </LargeDialog>
  );
}

function unitName(units: BangTraDanhMuc, id: string): string {
  const found = traTen(units, id);
  switch (found.loai) {
    case "coTen":
      return found.ten;
    case "dangDoc":
      return "Đang tải…";
    case "chuaGan":
      return "Chưa chuyển";
    default:
      // The id itself: still names exactly one unit, and the catalogue may simply not be loaded.
      return id;
  }
}

function hasResult(letter: documents_citizenLetterOut): boolean {
  return (letter.result_document_no ?? "") !== "";
}

/** Summary (15px bold) and the sender line (`PetitionDetailDrawer.tsx:352-369`), masked. */
function DrawerHeadline({ letter }: { letter: documents_citizenLetterOut }) {
  const sender = senderView(letter);
  return (
    <>
      {letter.summary !== null ? (
        <p className="m-0 mt-0.5 text-[15px] leading-snug font-bold break-words text-navy">{letter.summary}</p>
      ) : (
        <p className="m-0 mt-0.5 text-[15px] leading-snug font-bold text-ink-muted">
          {letter.summary_withheld ? WITHHELD_SUMMARY : "Không ghi nội dung"}
        </p>
      )}
      <p className="m-0 mt-1 flex flex-wrap items-center gap-3 text-[11.5px] text-ink-muted">
        <span className="flex items-center gap-1">
          <User aria-hidden="true" focusable="false" className="size-3" />
          {sender.kind === "withheld" ? WITHHELD_SENDER : sender.kind === "unknown" ? SENDER_UNKNOWN : sender.name}
        </span>
        {sender.kind === "named" && sender.phone !== null && (
          // MASKED TEXT, NOT A LINK: the prototype's `tel:` would dial a number nobody may read (rule 3).
          <span className="flex items-center gap-1 tabular-nums">
            <Phone aria-hidden="true" focusable="false" className="size-3" />
            {sender.phone}
          </span>
        )}
      </p>
    </>
  );
}

/** The prototype's badge row (`PetitionDetailDrawer.tsx:489-511`) — no source badge: the record has none. */
function DrawerBadges({ letter, now }: { letter: documents_citizenLetterOut; now: Date }) {
  const due = dueLabel(activeDue(letter), letter.is_closed, now);
  return (
    <div className="mb-3 flex flex-wrap gap-2">
      <LetterStatusBadge status={letter.status} />
      <Badge className="border-violet/25 bg-violet/12 text-violet">{letterTypeLabel(letter.letter_type)}</Badge>
      <Badge className="border-line bg-surface text-ink">
        <span className={DUE_TONE_CLASS[due.tone]}>{due.text}</span>
      </Badge>
      {(letter.related_letter_id ?? "") !== "" && (
        <Badge tone="warning">Đã đánh dấu trùng</Badge>
      )}
    </div>
  );
}

/** The prototype's figure (`PetitionDetailDrawer.tsx:858-873`). */
function Figure({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-[10.5px] font-semibold tracking-wide text-ink-muted uppercase">{label}</dt>
      <dd className="m-0 mt-0.5 text-[12.5px] font-semibold break-words text-navy">{children}</dd>
    </div>
  );
}

/* ---- the status strip ---------------------------------------------------------------------- */

/**
 * The prototype's strip (`PetitionDetailDrawer.tsx:385-484`) on C3's statuses. Clicking a clickable
 * chip opens the composer — a note, then Huỷ / Xác nhận — and posts the arrow. The server's refusal
 * (e.g. "Đã giải quyết" before a result is recorded) is shown verbatim in the composer.
 */
export function StatusStrip({ letter, onMoved }: { letter: documents_citizenLetterOut; onMoved: () => void }) {
  const strip = statusStrip(letter.status, letter.next_statuses, true);
  const [pending, setPending] = useState<string | null>(null);
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const open = (code: string) => {
    setPending(code);
    setNote("");
    setError("");
  };

  const confirm = () => {
    if (pending === null || sending) return;
    setError("");
    setSending(true);
    void moveCitizenLetter(letter.id, { status: pending, note: note.trim() }).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setPending(null);
      setNote("");
      onMoved();
    });
  };

  return (
    <div className="shrink-0 border-0 border-b border-solid border-line bg-white px-5 py-3">
      <div role="group" aria-label="Các bước của đơn thư" className="flex min-w-0 flex-wrap items-stretch gap-1.5">
        {strip.main.map((chip) => (
          <StatusChip key={chip.code} chip={chip} onClick={() => open(chip.code)} />
        ))}
      </div>
      {strip.branches.length > 0 && (
        <div role="group" aria-label="Rẽ nhánh" className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
          <span aria-hidden="true" className="mr-1 text-[11px] text-ink-muted">
            Rẽ nhánh:
          </span>
          {strip.branches.map((chip) => (
            <StatusChip key={chip.code} chip={chip} onClick={() => open(chip.code)} />
          ))}
        </div>
      )}
      <p className="m-0 mt-2 text-[11.5px] text-ink-muted">{letterStatusHint(letter.status)}</p>

      {pending !== null && (
        <div className="mt-2.5 rounded-[10px] border border-l-4 border-solid border-brand/35 border-l-brand bg-white p-3">
          <p className="m-0 text-[12.5px] font-semibold text-navy">Chuyển sang “{letterStatusLabel(pending)}”</p>
          <div className="mt-2 flex flex-col">
            <label htmlFor="don-thu-ghi-chu-buoc" className="text-[11.5px] font-medium text-ink">
              Ghi chú
            </label>
            <textarea
              id="don-thu-ghi-chu-buoc"
              rows={2}
              autoFocus
              className="mt-1 box-border min-h-0 w-full rounded-lg border border-solid border-input bg-white px-2.5 py-2 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              placeholder="Ý kiến chỉ đạo, lý do chuyển, việc đã làm…"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          {error !== "" && (
            <p className="thong-bao-loi m-0 mt-2" role="alert">
              {error}
            </p>
          )}
          <div className="mt-2 flex items-center justify-end gap-2">
            <Button type="button" size="sm" variant="outline" disabled={sending} onClick={() => setPending(null)}>
              Huỷ
            </Button>
            <Button type="button" size="sm" variant="primary" disabled={sending} aria-busy={sending || undefined} onClick={confirm}>
              <BusyLabel busy={sending} label="Xác nhận" busyText="Đang lưu…" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

/** One chip (`PetitionDetailDrawer.tsx:902-949`): filled when current, white when clickable, faded otherwise. */
function StatusChip({ chip, onClick }: { chip: StatusChipModel; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={!chip.clickable}
      onClick={onClick}
      aria-current={chip.current ? "step" : undefined}
      title={chip.current ? letterStatusHint(chip.code) : chip.clickable ? `Chuyển sang ${chip.label}` : "Không chuyển thẳng sang bước này được"}
      className={cn(
        "min-w-[6.5rem] flex-1 rounded-[8px] border border-solid px-2.5 py-1.5 text-left [font-family:inherit] transition-colors",
        chip.current
          ? cn("border-transparent", letterStatusActiveTone(chip.code))
          : chip.clickable
            ? "cursor-pointer border-line bg-white text-ink hover:bg-canvas"
            : "cursor-not-allowed border-line/60 bg-white text-ink-muted/60",
      )}
    >
      <span className="block text-[12px] font-semibold">{chip.label}</span>
      <span className={cn("block text-[10.5px]", chip.current ? "text-white/80" : "text-ink-muted/70")}>
        {chip.current ? "đang ở đây" : chip.clickable ? "chuyển sang" : "—"}
      </span>
    </button>
  );
}

const SMALL_INPUT =
  "mt-1 box-border h-9 min-h-0 w-full min-w-0 rounded-lg border border-solid border-input bg-white px-2.5 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50";
const SMALL_LABEL = "block text-[11.5px] leading-none font-medium text-ink";

/* ---- the clerk's deadline (ADR 0079 lô 5 Q18) ---------------------------------------------- */

/**
 * The clerk-set "Hạn xử lý" — `PATCH …/deadline`. THE PROTOTYPE SHOWS the figure (`PetitionDetailDrawer
 * .tsx:519-523`, "Không đặt" when none) but edits it nowhere in the drawer; its only deadline control is
 * the entry form's date field (`DocumentEntryForm.tsx:227-238`, label "Hạn xử lý", `type="date"`, sent as
 * 17:00 of that day, `:118`). So: the sender correction's shape (outline button → grey inline box → Lưu /
 * Huỷ) around that date field, plus "Không đặt" to clear. The server picks the phase's column and refuses
 * a finished letter (409) — its sentence is shown verbatim. Shown only to `petition.create` (convenience).
 */
export function DeadlineEditor({
  letter,
  onSaved,
}: {
  letter: documents_citizenLetterOut;
  onSaved: (cleared: boolean) => void;
}) {
  const current = activeDue(letter);
  const [editing, setEditing] = useState(false);
  const [date, setDate] = useState("");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  if (!editing) {
    return (
      <div className="mt-3">
        <Button
          type="button"
          size="sm"
          variant="outline"
          icon={<Glyph icon={Pencil} />}
          onClick={() => {
            setDate(dueInputValue(current));
            setError("");
            setEditing(true);
          }}
        >
          {DEADLINE_EDIT_LABEL}
        </Button>
      </div>
    );
  }

  const send = (dueAt: string | null) => {
    if (sending) return;
    setError("");
    setSending(true);
    void setCitizenLetterDeadline(letter.id, { due_at: dueAt }).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setEditing(false);
      onSaved(dueAt === null);
    });
  };

  const instant = dueInstantOf(date);
  return (
    <form
      className="mt-3 flex flex-col gap-3 rounded-[10px] border border-solid border-line bg-canvas p-3"
      aria-label={DEADLINE_EDIT_LABEL}
      onSubmit={(e) => {
        e.preventDefault();
        if (instant !== null) send(instant);
      }}
    >
      <div className="min-w-0 sm:max-w-[16rem]">
        <label htmlFor="don-thu-han-xu-ly" className={SMALL_LABEL}>
          Hạn xử lý
        </label>
        <input
          id="don-thu-han-xu-ly"
          type="date"
          className={SMALL_INPUT}
          value={date}
          onChange={(e) => setDate(e.target.value)}
        />
      </div>
      {error !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" size="sm" variant="primary" disabled={sending || instant === null} aria-busy={sending || undefined}>
          <BusyLabel busy={sending} label="Lưu" busyText="Đang lưu…" />
        </Button>
        {current !== null && current !== "" && (
          <Button type="button" size="sm" variant="outline" disabled={sending} onClick={() => send(null)}>
            {DUE_NOT_SET}
          </Button>
        )}
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => {
            setEditing(false);
            setError("");
          }}
        >
          Huỷ
        </Button>
      </div>
    </form>
  );
}

/* ---- sender correction --------------------------------------------------------------------- */

/**
 * "Sửa thông tin người gửi" (`PetitionDetailDrawer.tsx:527-588`). THE FORM STARTS EMPTY (ADR 0078 #4):
 * the screen never holds the full phone or the address, so it cannot prefill them, and an empty field
 * means "leave as is". The current (masked) value is only a placeholder.
 */
export function SenderCorrection({ letter, onSaved }: { letter: documents_citizenLetterOut; onSaved: () => void }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<SenderDraft>(EMPTY_SENDER);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const sender = senderView(letter);

  if (!editing) {
    return (
      <div className="mt-3">
        <Button type="button" size="sm" variant="outline" icon={<Glyph icon={Pencil} />} onClick={() => setEditing(true)}>
          Sửa thông tin người gửi
        </Button>
      </div>
    );
  }

  const save = () => {
    if (sending) return;
    const built = buildSenderCorrection(draft);
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setError("");
    setSending(true);
    void correctCitizenLetterSender(letter.id, built.body).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setEditing(false);
      setDraft(EMPTY_SENDER);
      onSaved();
    });
  };

  const withheld = sender.kind === "withheld";
  const keep = "Để trống: giữ nguyên";
  return (
    <form
      className="mt-3 flex flex-col gap-3 rounded-[10px] border border-solid border-line bg-canvas p-3"
      aria-label="Sửa thông tin người gửi"
      onSubmit={(e) => {
        e.preventDefault();
        save();
      }}
    >
      <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="min-w-0">
          <label htmlFor="sua-nguoi-gui-ten" className={SMALL_LABEL}>
            Người gửi
          </label>
          <input
            id="sua-nguoi-gui-ten"
            autoComplete="off"
            className={SMALL_INPUT}
            disabled={draft.clearAll}
            placeholder={withheld ? WITHHELD_SENDER : sender.kind === "named" ? sender.name : keep}
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
          />
        </div>
        <div className="min-w-0">
          <label htmlFor="sua-nguoi-gui-sdt" className={SMALL_LABEL}>
            Điện thoại
          </label>
          <input
            id="sua-nguoi-gui-sdt"
            type="tel"
            autoComplete="off"
            className={SMALL_INPUT}
            disabled={draft.clearAll}
            placeholder={sender.kind === "named" && sender.phone !== null ? sender.phone : keep}
            value={draft.phone}
            onChange={(e) => setDraft({ ...draft, phone: e.target.value })}
          />
        </div>
      </div>
      <div className="min-w-0">
        <label htmlFor="sua-nguoi-gui-dia-chi" className={SMALL_LABEL}>
          Địa chỉ
        </label>
        <input
          id="sua-nguoi-gui-dia-chi"
          autoComplete="off"
          className={SMALL_INPUT}
          disabled={draft.clearAll}
          placeholder={letter.has_sender_address ? "Đã có địa chỉ — nhập để thay" : keep}
          value={draft.address}
          onChange={(e) => setDraft({ ...draft, address: e.target.value })}
        />
      </div>
      <label htmlFor="sua-nguoi-gui-khong-ro" className="flex w-fit items-center gap-2 text-[12px] text-ink">
        <input
          id="sua-nguoi-gui-khong-ro"
          type="checkbox"
          className="size-3.5 accent-brand"
          checked={draft.clearAll}
          onChange={(e) => setDraft(e.target.checked ? { ...EMPTY_SENDER, clearAll: true } : EMPTY_SENDER)}
        />
        {SENDER_UNKNOWN} — xoá cả họ tên, số điện thoại và địa chỉ
      </label>
      {error !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
      <div className="flex gap-2">
        <Button type="submit" size="sm" variant="primary" disabled={sending} aria-busy={sending || undefined}>
          <BusyLabel busy={sending} label="Lưu" busyText="Đang lưu…" />
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={sending}
          onClick={() => {
            setEditing(false);
            setDraft(EMPTY_SENDER);
            setError("");
          }}
        >
          Huỷ
        </Button>
      </div>
    </form>
  );
}

/* ---- routing ------------------------------------------------------------------------------- */

/**
 * "Chuyển cho bộ phận khác" — posting `routings`: unit required, officer optional, reason required.
 * Assignment is an ATTRIBUTE (C3): this never moves the status. `reason` is free text that may name a
 * citizen — it lives only in this state and the POST body (rule 3).
 */
function RoutingBox({
  letterId,
  units,
  directory,
  onRouted,
}: {
  letterId: string;
  units: BangTraDanhMuc;
  directory: DanhBaTheoMa | null;
  onRouted: () => void;
}) {
  const [draft, setDraft] = useState<RoutingDraft>(EMPTY_ROUTING);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const send = () => {
    if (sending) return;
    const built = buildRouting(draft);
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setError("");
    setSending(true);
    void routeCitizenLetter(letterId, built.body).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setDraft(EMPTY_ROUTING);
      onRouted();
    });
  };

  // The prototype's box (`PetitionDetailDrawer.tsx:631-701`): grey, ONE column, each field full width.
  // Its `bg-surface` is the prototype's light grey; in this kit that grey is `bg-canvas` (`surface` = white).
  return (
    <section aria-labelledby="tieu-de-chuyen-don-thu" className="mt-5 border-0 border-t border-solid border-line pt-4">
      <h3 id="tieu-de-chuyen-don-thu" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        {ROUTING_SECTION_TITLE}
      </h3>
      <form
        aria-labelledby="tieu-de-chuyen-don-thu"
        className="flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid border-line bg-canvas p-3"
        onSubmit={(e) => {
          e.preventDefault();
          send();
        }}
      >
        <div className="min-w-0">
          <label htmlFor="chuyen-don-thu-bo-phan" className={SMALL_LABEL}>
            Chuyển đến
          </label>
          <select
            id="chuyen-don-thu-bo-phan"
            className={cn(ROUTING_SELECT)}
            value={draft.toUnit}
            onChange={(e) => setDraft({ ...draft, toUnit: e.target.value })}
          >
            <option value="">— Chọn bộ phận —</option>
            {units.pha === "xong" &&
              [...units.ten].map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
          </select>
        </div>
        <div className="min-w-0">
          <label htmlFor="chuyen-don-thu-can-bo" className={SMALL_LABEL}>
            Người xử lý (không bắt buộc)
          </label>
          {directory !== null ? (
            <select
              id="chuyen-don-thu-can-bo"
              className={ROUTING_SELECT}
              value={draft.assignee}
              onChange={(e) => setDraft({ ...draft, assignee: e.target.value })}
            >
              <option value="">— Để bộ phận tự phân công —</option>
              {[...directory.values()].map((person) => (
                <option key={person.code} value={person.code}>
                  {person.full_name}
                  {person.position ? ` — ${person.position}` : ""}
                </option>
              ))}
            </select>
          ) : (
            // Directory not loaded: the staff CODE is typed — the only thing `assignee` accepts.
            <input
              id="chuyen-don-thu-can-bo"
              autoComplete="off"
              className={SMALL_INPUT}
              placeholder="Mã cán bộ — để trống nếu để bộ phận tự phân công"
              value={draft.assignee}
              onChange={(e) => setDraft({ ...draft, assignee: e.target.value })}
            />
          )}
        </div>
        <div className="min-w-0">
          <label htmlFor="chuyen-don-thu-ly-do" className={SMALL_LABEL}>
            Lý do chuyển
          </label>
          <input
            id="chuyen-don-thu-ly-do"
            autoComplete="off"
            className={SMALL_INPUT}
            placeholder="Thuộc thẩm quyền của bộ phận Địa chính"
            value={draft.reason}
            onChange={(e) => setDraft({ ...draft, reason: e.target.value })}
          />
        </div>
        {error !== "" && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" icon={<Glyph icon={Send} />} disabled={sending} aria-busy={sending || undefined}>
            <BusyLabel busy={sending} label="Chuyển và ghi vết" busyText="Đang chuyển…" />
          </Button>
        </div>
      </form>
    </section>
  );
}

const ROUTING_SELECT =
  "mt-1 box-border h-9 min-h-0 w-full min-w-0 cursor-pointer rounded-md border border-solid border-line bg-white pr-9 pl-3 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50";

/* ---- the result (C10) ---------------------------------------------------------------------- */

/**
 * The prototype's "Nội dung trả lời công dân" section (`PetitionDetailDrawer.tsx:705-724`), in the
 * same place and style — but the answer is C10's RESULT: the issued document (number, date, signer,
 * issuing body) and a summary, `PUT …/result`. Enabled only in Thụ lý / Đang giải quyết, for a viewer
 * who may work on the letter; otherwise the saved result is shown, read-only.
 */
export function ResultSection({
  letter,
  mayWork,
  onSaved,
}: {
  letter: documents_citizenLetterOut;
  mayWork: boolean;
  onSaved: () => void;
}) {
  const editable = mayWork && RESULT_EDITABLE_STATUSES.includes(letter.status);
  const [draft, setDraft] = useState<ResultDraft>(() => resultDraftOf(letter));
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const save = () => {
    if (!editable || sending) return;
    setError("");
    setSending(true);
    void recordCitizenLetterResult(letter.id, {
      result_document_no: draft.documentNo.trim(),
      result_document_date: draft.documentDate,
      result_signer: draft.signer.trim(),
      result_issuer: draft.issuer.trim(),
      result_summary: draft.summary.trim(),
    }).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      onSaved();
    });
  };

  const field = (id: string, label: string, key: keyof ResultDraft, type = "text") => (
    <div className="min-w-0">
      <label htmlFor={id} className={SMALL_LABEL}>
        {label}
      </label>
      <input
        id={id}
        type={type}
        autoComplete="off"
        className={SMALL_INPUT}
        disabled={!editable}
        value={draft[key]}
        onChange={(e) => setDraft({ ...draft, [key]: e.target.value })}
      />
    </div>
  );

  return (
    <section aria-labelledby="tieu-de-ket-qua-don-thu" className="mt-5 border-0 border-t border-solid border-line pt-4">
      <h3 id="tieu-de-ket-qua-don-thu" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        {RESULT_SECTION_TITLE}
      </h3>
      <form
        className="flex flex-col gap-3"
        aria-labelledby="tieu-de-ket-qua-don-thu"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
          {field("ket-qua-so-van-ban", "Số văn bản", "documentNo")}
          {field("ket-qua-ngay-ban-hanh", "Ngày ban hành", "documentDate", "date")}
          {field("ket-qua-nguoi-ky", "Người ký", "signer")}
          {field("ket-qua-co-quan", "Cơ quan ban hành", "issuer")}
        </div>
        <div className="min-w-0">
          <label htmlFor="ket-qua-tom-tat" className={SMALL_LABEL}>
            Tóm tắt kết quả
          </label>
          <textarea
            id="ket-qua-tom-tat"
            rows={4}
            disabled={!editable}
            placeholder={
              letter.summary_withheld && letter.result_summary === null && hasResult(letter)
                ? WITHHELD_SUMMARY
                : "Ghi rõ kết quả giải quyết để trả lời người gửi đơn."
            }
            className="mt-1 box-border w-full min-w-0 rounded-lg border border-solid border-input bg-white px-2.5 py-2 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50"
            value={draft.summary}
            onChange={(e) => setDraft({ ...draft, summary: e.target.value })}
          />
        </div>
        {!editable && mayWork && <p className="m-0 text-[11.5px] text-ink-muted">{RESULT_LOCKED_HINT}</p>}
        {error !== "" && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        {editable && (
          <div>
            <Button type="submit" size="sm" variant="primary" disabled={sending} aria-busy={sending || undefined}>
              <BusyLabel busy={sending} label={RESULT_SAVE_LABEL} busyText="Đang lưu…" />
            </Button>
          </div>
        )}
      </form>
    </section>
  );
}

/* ---- the log ------------------------------------------------------------------------------- */

/**
 * "Nhật ký & Trao đổi" (`PetitionDetailDrawer.tsx:736-762`): one note, `POST …/log-entries`. The
 * Idempotency-Key belongs to the DRAFT — made once, renewed only after the note is saved — so a retry
 * after a network error replays the same note instead of adding a second, unremovable line.
 */
export function NoteComposer({ letterId, onAdded }: { letterId: string; onAdded: () => void }) {
  const [text, setText] = useState("");
  const [key, setKey] = useState(() => crypto.randomUUID());
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const add = () => {
    const content = text.trim();
    if (content === "" || sending) return;
    setError("");
    setSending(true);
    void addCitizenLetterNote(letterId, content, key).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setText("");
      setKey(crypto.randomUUID());
      onAdded();
    });
  };

  return (
    <section aria-labelledby="tieu-de-nhat-ky-don-thu" className="mb-4">
      <h3 id="tieu-de-nhat-ky-don-thu" className="m-0 mb-2 text-[12.5px] font-bold text-navy">
        Nhật ký &amp; Trao đổi
      </h3>
      <label htmlFor="don-thu-nhat-ky" className="an-thi-giac">
        Nội dung ghi nhật ký
      </label>
      <textarea
        id="don-thu-nhat-ky"
        rows={3}
        className="box-border w-full min-w-0 rounded-lg border border-solid border-input bg-white px-2.5 py-2 [font-family:inherit] text-[12.5px] text-navy outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        placeholder="Đã làm được gì, còn vướng gì…"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      {error !== "" && (
        <p className="thong-bao-loi m-0 mt-1.5" role="alert">
          {error}
        </p>
      )}
      <div className="mt-1.5 flex justify-end">
        <Button type="button" size="sm" variant="primary" disabled={text.trim() === "" || sending} onClick={add}>
          <BusyLabel busy={sending} label="Ghi nhật ký" busyText="Đang ghi…" />
        </Button>
      </div>
    </section>
  );
}

/** Words of a log row's pill, by kind; a status change names the status it moved INTO. */
function logPill(entry: documents_letterLogEntryOut): string | null {
  switch (entry.kind) {
    case "chuyen-trang-thai":
      return entry.to_status ? letterStatusLabel(entry.to_status) : null;
    case "ket-qua":
      return "Ghi kết quả giải quyết";
    case "sua-nguoi-gui":
      return "Sửa thông tin người gửi";
    default:
      return null;
  }
}

/**
 * "Dòng thời gian chuyển tiếp" (`PetitionDetailDrawer.tsx:764-836`) over the processing log, newest
 * first as the server sends it. READ-ONLY: the log is append-only (rule 7, forbidden #5) and no
 * control on any line edits or removes it. The actor is named as the prototype does (`actor_name`,
 * `:779-784`): the full name from the directory read once by the register, or the bare code when the
 * directory does not know it (not loaded, or a retired account) — the code still names one person.
 * The STORED trail keeps the business code either way (rule 6, invariant 8); this is presentation.
 */
export function LetterLog({
  log,
  units,
  directory,
}: {
  log: KetQua<documents_letterLogOut> | null;
  units: BangTraDanhMuc;
  directory: DanhBaTheoMa | null;
}) {
  return (
    <section aria-labelledby="tieu-de-dong-thoi-gian-don-thu" className="flex min-w-0 flex-col">
      <h3 id="tieu-de-dong-thoi-gian-don-thu" className="m-0 mb-2.5 text-[12.5px] font-bold text-navy">
        Dòng thời gian chuyển tiếp
      </h3>
      {log === null && (
        <p role="status" className="m-0 text-[12.5px] text-ink-muted">
          Đang tải nhật ký xử lý…
        </p>
      )}
      {log !== null && !log.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          {log.thongBao}
        </p>
      )}
      {log !== null && log.ok && log.duLieu.items.length === 0 && (
        <p className="m-0 text-[12.5px] text-ink-muted">{LETTER_LOG_EMPTY}</p>
      )}
      {log !== null && log.ok && log.duLieu.items.length > 0 && (
        <ol className="m-0 flex list-none flex-col gap-3 p-0">
          {log.duLieu.items.map((entry) => (
            <LogRow key={entry.id} entry={entry} units={units} directory={directory} />
          ))}
        </ol>
      )}
    </section>
  );
}

function LogRow({
  entry,
  units,
  directory,
}: {
  entry: documents_letterLogEntryOut;
  units: BangTraDanhMuc;
  directory: DanhBaTheoMa | null;
}) {
  const actorName = directory?.get(entry.actor_code)?.full_name ?? "";
  const who = actorName === "" ? entry.actor_code : actorName;
  const pill = logPill(entry);
  const moved = entry.kind === "luan-chuyen" && (entry.to_unit_id ?? "") !== "";
  return (
    <li className="flex min-w-0 gap-2.5">
      <span
        aria-hidden="true"
        className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-brand/12 text-[11px] font-bold text-brand"
      >
        {initialsOf(actorName)}
      </span>
      <div className="min-w-0 flex-1 [&>p]:m-0">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <b className="text-[12.5px] text-navy">{who}</b>
          <time dateTime={entry.at} className="text-[11px] text-ink-muted tabular-nums">
            {nhanThoiDiem(entry.at)}
          </time>
        </div>
        {pill !== null && (
          <span className="mt-1 inline-block rounded-full bg-brand/12 px-2 py-0.5 text-[11px] font-semibold text-brand">
            {pill}
          </span>
        )}
        {moved && (
          <p className="mt-1 flex items-start gap-1.5 text-[12px] text-navy">
            <ArrowRight aria-hidden="true" focusable="false" className="mt-0.5 size-3 shrink-0 text-ink-muted" />
            <span>
              <span className="text-ink-muted">
                {(entry.from_unit_id ?? "") === "" ? "Một cửa" : unitName(units, entry.from_unit_id ?? "")}{" "}
                <span aria-hidden="true">→</span>
                <span className="an-thi-giac">đến</span>{" "}
              </span>
              <b className="font-semibold">{unitName(units, entry.to_unit_id ?? "")}</b>
            </span>
          </p>
        )}
        {entry.kind === "luan-chuyen" && (entry.assignee_code ?? "") !== "" && (
          <p className="mt-0.5 flex items-start gap-1.5 text-[12px] text-navy">
            <User aria-hidden="true" focusable="false" className="mt-0.5 size-3 shrink-0 text-ink-muted" />
            <span>
              <span className="text-ink-muted">Phụ trách: </span>
              <b className="font-semibold">{staffNameWithCode(entry.assignee_code ?? "", directory)}</b>
            </span>
          </p>
        )}
        {(entry.content ?? "") !== "" && <p className="mt-1 text-[12.5px] break-words whitespace-pre-line">{entry.content}</p>}
      </div>
    </li>
  );
}
