"use client";

import { AlertTriangle } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { bookCitizenLetter, checkLetterDuplicates } from "@/lib/api/citizen-letters";
import type { documents_citizenLetterOut, documents_duplicateCandidateOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  LETTER_TYPES,
  buildBooking,
  duplicateQuery,
  emptyEntryDraft,
  letterDate,
  letterNumber,
  type EntryDraft,
} from "./letter-display";

/** Heading id of the dialog — its accessible name. */
const ENTRY_TITLE_ID = "tieu-de-vao-so-don-thu";

/** The prototype's dialog words (`PetitionEntryForm.tsx:107`, `:225`). */
export const LETTER_ENTRY_TITLE = "Vào sổ đơn thư công dân";
export const LETTER_ENTRY_SUBMIT = "Vào sổ đơn thư";

/**
 * The prototype's description says the sender, address and phone are REQUIRED and the deadline is
 * computed by type. Here neither holds: the sender is optional (C7) and no deadline is set yet (ADR 0078
 * #3) — so the sentence says what this register actually does.
 */
export const LETTER_ENTRY_DESCRIPTION =
  "Người gửi, địa chỉ, số điện thoại không bắt buộc: đơn không rõ người gửi vẫn vào sổ được. Số đơn do " +
  "hệ thống cấp khi lưu; sổ đơn thư hiện chưa đặt hạn giải quyết.";

/** The prototype's footer sentence of the duplicate box, verbatim (`PetitionEntryForm.tsx:195-197`). */
export const DUPLICATE_FOOTER = "Vẫn lưu được — hệ thống chỉ nhắc để cán bộ hỏi lại công dân.";

/** C11's confirm control: the clerk links the new letter to ONE earlier one, or to none. */
export const MERGE_INTO_LABEL = "Gộp vào đơn này";

/** The prototype's select / input frame (`PetitionEntryForm.tsx:26-27`): 36px, `rounded-md`, 13px. */
const ENTRY_SELECT =
  "box-border h-9 min-h-0 w-full min-w-0 cursor-pointer rounded-md border border-solid border-line bg-surface pr-9 pl-3 [font-family:inherit] text-[13px] text-navy outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50";

/** How long the clerk may pause typing before the duplicate check runs. */
const DUPLICATE_DEBOUNCE_MS = 500;

/** Today as `YYYY-MM-DD`, on the machine's calendar — the "Ngày nhận" default. */
function today(): string {
  const d = new Date();
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

/**
 * "Vào sổ đơn thư công dân" — the prototype's centred dialog (`PetitionEntryForm.tsx`, 44rem), with the
 * customer's rules where they differ (ADR 0078 #1):
 *   - the sender's three fields are OPTIONAL (C7), and "Không rõ người gửi" clears and disables them;
 *   - the duplicate warning is the SERVER's (C11), asked with a POST BODY — never a URL (rule 3,
 *     forbidden #4) — and each candidate has "Gộp vào đơn này", which sets `related_letter_id` on the
 *     booking (one at most). The server never links two letters on its own;
 *   - no deadline field and no number field: the server owns both.
 *
 * Self-contained (its own draft, its own Idempotency-Key) so both tabs can open it from the header. The
 * KEY IS MADE ONCE, WHEN THE DIALOG MOUNTS: a retry after a network error must replay the same booking,
 * not take a second number (`lib/api/citizen-letters.ts`).
 *
 * The refusal and the "Cần nội dung đơn." check are said INSIDE the dialog, under the fields: the
 * dialog is in the top layer and a toast would sit behind its backdrop (ADR 0068 lần 6 #4).
 */
export function LetterEntryDialog({
  units,
  onClose,
  onBooked,
  initialDraft,
}: {
  /** The org-unit catalogue, for "Chuyển ngay cho bộ phận". */
  units: BangTraDanhMuc;
  onClose: () => void;
  /** Called with the booked letter; the caller closes the dialog and says what was booked. */
  onBooked: (letter: documents_citizenLetterOut) => void;
  /** Tests and the preview start from a filled draft; the screen starts empty. */
  initialDraft?: EntryDraft;
}) {
  const [draft, setDraft] = useState<EntryDraft>(() => initialDraft ?? emptyEntryDraft(today()));
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  /** The candidates of the LAST check, kept with the query that produced them. */
  const [found, setFound] = useState<{ key: string; items: readonly documents_duplicateCandidateOut[] } | null>(null);

  const query = duplicateQuery(draft);
  const queryKey = query === null ? "" : JSON.stringify(query);

  useEffect(() => {
    if (queryKey === "") return;
    let dropped = false;
    const timer = window.setTimeout(() => {
      void checkLetterDuplicates(JSON.parse(queryKey)).then((k) => {
        // A failed check is not shown: it is a reminder, and the booking does not depend on it.
        if (!dropped && k.ok) setFound({ key: queryKey, items: k.duLieu.items });
      });
    }, DUPLICATE_DEBOUNCE_MS);
    return () => {
      dropped = true;
      window.clearTimeout(timer);
    };
  }, [queryKey]);

  // Candidates of an OLDER query are never shown under the current text.
  const candidates = found !== null && found.key === queryKey ? found.items : [];
  // A confirmed link to a letter that is no longer a candidate is not sent.
  const relatedLetterId = candidates.some((c) => c.id === draft.relatedLetterId) ? draft.relatedLetterId : "";

  const update = (patch: Partial<EntryDraft>) => setDraft((d) => ({ ...d, ...patch }));

  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (sending) return;
    const built = buildBooking({ ...draft, relatedLetterId });
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setError("");
    setSending(true);
    void bookCitizenLetter(built.body, idempotencyKey).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      onBooked(k.duLieu);
    });
  };

  const dismiss = () => {
    if (!sending) onClose();
  };

  const senderDisabled = draft.senderUnknown;

  return (
    <ModalDialog titleId={ENTRY_TITLE_ID} onDismiss={dismiss} className="sm:max-w-[44rem]" closeDisabled={sending}>
      <form className="flex min-h-0 flex-col gap-4" aria-label={LETTER_ENTRY_TITLE} onSubmit={submit}>
        <ModalDialogHeader titleId={ENTRY_TITLE_ID} title={LETTER_ENTRY_TITLE} description={LETTER_ENTRY_DESCRIPTION} />

        <div className="flex min-h-0 flex-col gap-3.5 overflow-y-auto">
          <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-[1fr_12rem]">
            <Field label="Người gửi" htmlFor="don-thu-nguoi-gui" grow="auto">
              <input
                id="don-thu-nguoi-gui"
                name="senderName"
                autoComplete="off"
                placeholder="Nguyễn Văn Tám"
                disabled={senderDisabled}
                value={draft.senderName}
                onChange={(e) => update({ senderName: e.target.value })}
              />
            </Field>
            <Field label="Ngày nhận" htmlFor="don-thu-ngay-nhan" grow="auto" required>
              <input
                id="don-thu-ngay-nhan"
                name="receivedDate"
                type="date"
                value={draft.receivedDate}
                onChange={(e) => update({ receivedDate: e.target.value })}
              />
            </Field>
          </div>

          <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-[1fr_12rem]">
            <Field label="Địa chỉ" htmlFor="don-thu-dia-chi" grow="auto">
              <input
                id="don-thu-dia-chi"
                name="senderAddress"
                autoComplete="off"
                disabled={senderDisabled}
                value={draft.senderAddress}
                onChange={(e) => update({ senderAddress: e.target.value })}
              />
            </Field>
            <Field label="Số điện thoại" htmlFor="don-thu-so-dien-thoai" grow="auto">
              <input
                id="don-thu-so-dien-thoai"
                name="senderPhone"
                type="tel"
                autoComplete="off"
                inputMode="tel"
                disabled={senderDisabled}
                value={draft.senderPhone}
                onChange={(e) => update({ senderPhone: e.target.value })}
              />
            </Field>
          </div>

          {/* C7: one tick says "the sender is not known" — the three fields empty and locked. */}
          <label htmlFor="don-thu-khong-ro" className="flex w-fit items-center gap-2 text-[12.5px] text-ink">
            <input
              id="don-thu-khong-ro"
              type="checkbox"
              className="size-3.5 accent-brand"
              checked={draft.senderUnknown}
              onChange={(e) =>
                update(
                  e.target.checked
                    ? { senderUnknown: true, senderName: "", senderPhone: "", senderAddress: "", relatedLetterId: "" }
                    : { senderUnknown: false },
                )
              }
            />
            Không rõ người gửi
          </label>

          <Field label="Loại đơn" htmlFor="don-thu-loai" kind="select" grow="auto" required>
            <select
              id="don-thu-loai"
              name="letterType"
              className={ENTRY_SELECT}
              value={draft.letterType}
              onChange={(e) => update({ letterType: e.target.value })}
            >
              {LETTER_TYPES.map((t) => (
                <option key={t.code} value={t.code}>
                  {t.label}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Nội dung đơn" htmlFor="don-thu-noi-dung" grow="auto" required>
            <textarea
              id="don-thu-noi-dung"
              name="summary"
              rows={3}
              className="h-auto py-2"
              value={draft.summary}
              onChange={(e) => update({ summary: e.target.value })}
            />
          </Field>

          {candidates.length > 0 && (
            <DuplicateBox
              candidates={candidates}
              selected={relatedLetterId}
              onSelect={(id) => update({ relatedLetterId: id })}
            />
          )}

          <Field label="Chuyển ngay cho bộ phận" htmlFor="don-thu-chuyen-ngay" kind="select" grow="auto">
            <select
              id="don-thu-chuyen-ngay"
              name="holdingUnit"
              className={ENTRY_SELECT}
              value={draft.holdingUnit}
              onChange={(e) => update({ holdingUnit: e.target.value })}
            >
              <option value="">— Chưa chuyển —</option>
              {units.pha === "xong" &&
                [...units.ten].map(([id, name]) => (
                  <option key={id} value={id}>
                    {name}
                  </option>
                ))}
            </select>
          </Field>

          {error !== "" && (
            <p className="thong-bao-loi m-0" role="alert">
              {error}
            </p>
          )}
        </div>

        <div className="flex flex-wrap justify-end gap-2 pt-1">
          <Button type="button" variant="outline" onClick={onClose} disabled={sending}>
            Huỷ
          </Button>
          <Button type="submit" variant="primary" disabled={sending} aria-busy={sending || undefined}>
            <BusyLabel busy={sending} label={LETTER_ENTRY_SUBMIT} busyText="Đang lưu…" />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/**
 * The prototype's tangerine warning (`PetitionEntryForm.tsx:177-199`), plus C11's confirm control: each
 * candidate carries "Gộp vào đơn này" — a toggle, at most one pressed; pressing the pressed one again
 * un-links. A candidate's summary is `null` when the letter being booked is a denunciation; then only
 * its number, date and similarity are shown.
 */
export function DuplicateBox({
  candidates,
  selected,
  onSelect,
}: {
  candidates: readonly documents_duplicateCandidateOut[];
  selected: string;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="rounded-[10px] border border-solid border-tangerine/25 bg-tangerine/8 p-3" role="region" aria-label="Cảnh báo đơn trùng">
      <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold text-navy">
        <AlertTriangle aria-hidden="true" focusable="false" className="size-4 shrink-0 text-tangerine" />
        Công dân này đã có {candidates.length} đơn nội dung tương tự
      </p>
      <ul className="m-0 mt-2 flex list-none flex-col gap-1.5 p-0">
        {candidates.map((c) => {
          const pressed = c.id === selected;
          const number = letterNumber(c.number, c.year);
          return (
            <li key={c.id} className="flex min-w-0 items-start gap-3 text-[12px]">
              <div className="min-w-0 flex-1">
                <span className="text-ink-muted">
                  Số {number} · {letterDate(c.received_date)} · giống {Math.round(c.similarity * 100)}%
                </span>
                {c.summary !== null && <p className="m-0 break-words text-navy">{c.summary}</p>}
              </div>
              <button
                type="button"
                aria-pressed={pressed}
                aria-label={`${MERGE_INTO_LABEL}: đơn số ${number}`}
                onClick={() => onSelect(pressed ? "" : c.id)}
                className={cn(
                  "h-7 shrink-0 cursor-pointer rounded-md border border-solid px-2.5 [font-family:inherit] text-[11.5px] font-semibold whitespace-nowrap",
                  pressed ? "border-tangerine bg-tangerine text-white" : "border-tangerine/40 bg-white text-tangerine hover:bg-tangerine/8",
                )}
              >
                {MERGE_INTO_LABEL}
              </button>
            </li>
          );
        })}
      </ul>
      <p className="m-0 mt-2 text-[11px] text-ink-muted">{DUPLICATE_FOOTER}</p>
    </div>
  );
}
