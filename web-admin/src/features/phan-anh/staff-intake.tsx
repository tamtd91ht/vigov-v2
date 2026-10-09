"use client";

import {
  CalendarClock,
  CircleCheck,
  CloudOff,
  MapPin,
  Phone,
  Plus,
  RefreshCw,
  Shapes,
  Upload,
  UserRound,
  X,
} from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { ModalDialog } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { PendingButton, PendingField } from "@/components/ui/pending-feature";
import { khoaSauLanGhi } from "@/features/thu-chi/nhan-thu-chi";
import type { KetQua } from "@/lib/api/goi";
import { bookStaffIntake, listIntakeFields, type StaffIntakeInput } from "@/lib/api/phieu-phan-anh";
import type { petitions_citizenFieldListOut, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";
import { coQuyen } from "@/lib/quyen";

import {
  clockFromBounds,
  clockFromRfc3339,
  INTAKE_ADDRESS_LABEL,
  INTAKE_ADDRESS_PLACEHOLDER,
  INTAKE_ANONYMOUS_LABEL,
  INTAKE_BUTTON,
  INTAKE_CANCEL,
  INTAKE_CHANNEL_NOTE,
  INTAKE_CLOCK_HINT,
  INTAKE_CLOCK_LABEL,
  INTAKE_CONTENT_LABEL,
  INTAKE_CONTENT_PLACEHOLDER,
  INTAKE_DESCRIPTION,
  INTAKE_DONE_ANOTHER,
  INTAKE_DONE_CLOSE,
  INTAKE_DONE_CODE_LABEL,
  INTAKE_DONE_SENTENCE,
  INTAKE_DONE_TITLE,
  INTAKE_FIELD_LABEL,
  INTAKE_FIELD_PLACEHOLDER,
  INTAKE_FIELDS_EMPTY,
  INTAKE_FIELDS_LOADING,
  INTAKE_NAME_LABEL,
  INTAKE_PHONE_LABEL,
  INTAKE_SUBMIT,
  INTAKE_TITLE,
  intakeContentError,
  PETITION_INTAKE_PERMISSION,
  petitionPendingPart,
} from "./nhan-phieu";
import { BusyLabel, Glyph, LABEL_CLASS, TEXTAREA_CLASS } from "./petition-ui";

/**
 * "Nhập hộ phản ánh" (spec §11) — an officer books a petition for a citizen who called, came to the
 * office or met the hamlet head. `POST /api/v1/citizen-reports` (`feedback.create`).
 *
 * WHAT IS NOT ON THE FORM, ON PURPOSE (ADR 0028 Bổ sung 2026-10-02): no channel select — the channel is
 * always `can-bo-nhap-ho` (row 5). The hamlet select and the scene-photo picker are DISABLED
 * placeholders with their "?" (ADR 0068 §14): the server does not accept either yet and answers 400
 * (`PHAN_CHUA_DUNG`), so neither has a value in this form's state nor a key in the body. The petition is linked to NO citizen account (row 1), so the success
 * screen shows the LOOKUP CODE large and tells the officer to hand it over (row 3).
 *
 * The button is UX only: the server checks `feedback.create` on both routes (rule 5, forbidden #1).
 *
 * PERSONAL DATA (name, phone, the citizen's words) lives in this component's state and the POST body
 * only — never in the URL, storage, or a log (rule 3).
 */

export function StaffIntakeButton({
  permissions,
  onBooked,
}: {
  permissions: readonly string[];
  /** A petition was booked — the register re-reads. */
  onBooked: () => void;
}) {
  const [open, setOpen] = useState(false);
  if (!coQuyen(permissions, PETITION_INTAKE_PERMISSION)) return null;
  return (
    <>
      <Button
        type="button"
        variant="primary"
        icon={<Glyph icon={Plus} />}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        {INTAKE_BUTTON}
      </Button>
      {open && <StaffIntakeDialog onClose={() => setOpen(false)} onBooked={onBooked} />}
    </>
  );
}

const TITLE_ID = "tieu-de-nhap-ho-phan-anh";

/**
 * The prototype's centred dialog (`FeedbackEntryForm.tsx:131`, `sm:max-w-[42rem]`) in the shared
 * `ModalDialog` — native `showModal()`: the page behind is inert, Tab stays inside, Esc asks. Mounted =
 * open; focus goes back to the button that opened it.
 */
function StaffIntakeDialog({ onClose, onBooked }: { onClose: () => void; onBooked: () => void }) {
  const [sending, setSending] = useState(false);
  return (
    <ModalDialog
      titleId={TITLE_ID}
      size="lg"
      className="max-w-[42rem]"
      // Closing while the booking is in flight would hide whether a code was issued.
      onDismiss={() => {
        if (!sending) onClose();
      }}
    >
      <StaffIntakeForm onClose={onClose} onBooked={onBooked} onSendingChange={setSending} />
    </ModalDialog>
  );
}

type FormValues = {
  field: string;
  clockLocal: string;
  content: string;
  address: string;
  reporterName: string;
  reporterPhone: string;
  anonymous: boolean;
};

const EMPTY: FormValues = {
  field: "",
  clockLocal: "",
  content: "",
  address: "",
  reporterName: "",
  reporterPhone: "",
  anonymous: false,
};

export function StaffIntakeForm({
  onClose,
  onBooked,
  onSendingChange,
  loadFields = listIntakeFields,
  book = bookStaffIntake,
}: {
  onClose: () => void;
  onBooked: () => void;
  onSendingChange?: (sending: boolean) => void;
  /** Injected only by tests; the screen always reads the contract routes. */
  loadFields?: () => Promise<KetQua<petitions_citizenFieldListOut>>;
  book?: (input: StaffIntakeInput, key: string) => Promise<KetQua<Pick<petitions_phieuPhanAnhRa, "code">>>;
}) {
  const [fieldsReloads, setFieldsReloads] = useState(0);
  const [fields, setFields] = useState<{ key: number; result: KetQua<petitions_citizenFieldListOut> } | null>(null);
  const [values, setValues] = useState<FormValues>(EMPTY);
  /** ONE key per opening of the draft: kept across retries, renewed only after a 201 (`khoaSauLanGhi`). */
  const [key, setKey] = useState(khoaChongTrungMoi);
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const [bookedCode, setBookedCode] = useState<string | null>(null);
  const [bounds] = useState(() => clockFromBounds(new Date()));

  useEffect(() => {
    let dropped = false;
    loadFields().then((result) => {
      if (!dropped) setFields({ key: fieldsReloads, result });
    });
    return () => {
      dropped = true;
    };
  }, [loadFields, fieldsReloads]);

  const currentFields = fields !== null && fields.key === fieldsReloads ? fields.result : null;

  function send(): void {
    const clockFrom = clockFromRfc3339(values.clockLocal);
    if (clockFrom === null || values.field === "" || intakeContentError(values.content) !== null || sending) return;
    setSending(true);
    onSendingChange?.(true);
    book(
      {
        field: values.field,
        content: values.content,
        address: values.address,
        // ANONYMOUS SENDS NO NAME AND NO PHONE (prototype `FeedbackEntryForm.tsx`: both disabled and sent
        // as null). A name typed before the box was ticked must not reach the record the citizen asked
        // to keep anonymous (rule 3). Blank strings are left out of the body by `bookStaffIntake`.
        reporterName: values.anonymous ? "" : values.reporterName,
        reporterPhone: values.anonymous ? "" : values.reporterPhone,
        anonymous: values.anonymous,
        clockFrom,
      },
      key,
    ).then((r) => {
      setSending(false);
      onSendingChange?.(false);
      setKey((k) => khoaSauLanGhi(k, r.ok, khoaChongTrungMoi));
      if (!r.ok) {
        // THE SERVER'S SENTENCE VERBATIM — clock_from_out_of_range, intake_not_configured,
        // field_catalogue_unavailable, field_not_offered, 403: each says what to do.
        setRefusal(r.thongBao);
        return;
      }
      setRefusal(null);
      setBookedCode(r.duLieu.code);
      onBooked();
    });
  }

  if (bookedCode !== null) {
    return (
      <StaffIntakeDone
        code={bookedCode}
        onClose={onClose}
        onAnother={() => {
          setBookedCode(null);
          setValues(EMPTY);
        }}
      />
    );
  }

  return (
    <StaffIntakeFormView
      fields={currentFields}
      values={values}
      setValues={setValues}
      bounds={bounds}
      sending={sending}
      refusal={refusal}
      onSubmit={send}
      onCancel={onClose}
      onReloadFields={() => setFieldsReloads((n) => n + 1)}
    />
  );
}

/** Presentational — rendered to a string in tests. `fields === null` = loading. */
export function StaffIntakeFormView({
  fields,
  values,
  setValues = () => {},
  bounds,
  sending = false,
  refusal = null,
  onSubmit = () => {},
  onCancel = () => {},
  onReloadFields,
}: {
  fields: KetQua<petitions_citizenFieldListOut> | null;
  values: FormValues;
  setValues?: (v: FormValues) => void;
  bounds?: { readonly min: string; readonly max: string };
  sending?: boolean;
  refusal?: string | null;
  onSubmit?: () => void;
  onCancel?: () => void;
  onReloadFields?: () => void;
}) {
  const set = <K extends keyof FormValues>(k: K, v: FormValues[K]) => setValues({ ...values, [k]: v });
  const contentError = intakeContentError(values.content);
  const clockBad = clockFromRfc3339(values.clockLocal) === null;
  const items = fields !== null && fields.ok ? fields.duLieu.items : [];
  const fieldsReady = fields !== null && fields.ok && items.length > 0;
  const canSend = fieldsReady && values.field !== "" && contentError === null && !clockBad && !sending;

  return (
    // The prototype's dialog body (`FeedbackEntryForm.tsx:132-286`): header, then the fields in its
    // order — Lĩnh vực · Nội dung · [Địa chỉ | Thôn] · [Người gửi | Số điện thoại] · ẩn danh · kênh ·
    // ảnh — then Huỷ / Vào sổ phản ánh. `Dân phản ánh lúc` is ours (the deadline's starting point, ADR
    // 0028) and sits under the field it times. The header and the buttons stay in sight; the fields
    // scroll between them.
    <form
      className="form-danh-muc m-0 flex min-h-0 flex-col gap-4 border-0 bg-transparent p-0"
      aria-busy={sending}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        if (canSend) onSubmit();
      }}
    >
      <div className="flex shrink-0 items-start gap-3">
        <div className="min-w-0 flex-1">
          <h2 id={TITLE_ID} tabIndex={-1} className="m-0 text-lg leading-snug font-semibold text-ink-900">
            {INTAKE_TITLE}
          </h2>
          <p className="mt-1.5 mb-0 text-sm text-ink-500">{INTAKE_DESCRIPTION}</p>
        </div>
        <IconButton label="Đóng biểu mẫu nhập hộ" type="button" variant="ghost" onClick={onCancel} disabled={sending}>
          <Glyph icon={X} />
        </IconButton>
      </div>

      <div className="flex min-h-0 flex-col gap-3.5 overflow-y-auto pr-1 [&>*]:my-0">

      {/* LĨNH VỰC — REQUIRED on this channel (§11): the field is known at booking, so both deadlines
          are fixed at once (ADR 0028). The list is the commune's (`citizen-report-intake-fields`). */}
      {fields === null && (
        <p role="status" className="m-0 text-sm text-ink-500">
          {INTAKE_FIELDS_LOADING}
        </p>
      )}
      {fields !== null && !fields.ok && (
        <div className="flex flex-wrap items-center gap-3">
          <Glyph icon={CloudOff} className="size-[18px] shrink-0 text-danger-600" />
          <p className="thong-bao-loi m-0 min-w-0 flex-1" role="alert">
            {fields.thongBao}
          </p>
          {onReloadFields !== undefined && (
            <Button type="button" variant="secondary" size="sm" icon={<Glyph icon={RefreshCw} />} onClick={onReloadFields}>
              Tải lại
            </Button>
          )}
        </div>
      )}
      {fields !== null && fields.ok && items.length === 0 && (
        <p className="thong-bao-loi m-0" role="alert">
          {INTAKE_FIELDS_EMPTY}
        </p>
      )}

      <Field label={INTAKE_FIELD_LABEL} required htmlFor="nhap-ho-linh-vuc" icon={Shapes} kind="select" grow="auto" className="max-w-none">
        <select
          id="nhap-ho-linh-vuc"
          name="nhap-ho-linh-vuc"
          required
          value={values.field}
          disabled={!fieldsReady || sending}
          onChange={(e) => set("field", e.target.value)}
        >
          <option value="">{INTAKE_FIELD_PLACEHOLDER}</option>
          {items.map((f) => (
            <option key={f.code} value={f.code}>
              {f.label}
            </option>
          ))}
        </select>
      </Field>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field
          label={INTAKE_CLOCK_LABEL}
          htmlFor="nhap-ho-luc"
          icon={CalendarClock}
          grow="auto"
          hint={<span id="nhap-ho-luc-goi-y">{INTAKE_CLOCK_HINT}</span>}
        >
          <input
            id="nhap-ho-luc"
            name="nhap-ho-luc"
            type="datetime-local"
            value={values.clockLocal}
            min={bounds?.min}
            max={bounds?.max}
            disabled={sending}
            aria-describedby="nhap-ho-luc-goi-y"
            onChange={(e) => set("clockLocal", e.target.value)}
          />
        </Field>
      </div>

      {/* Prototype `FeedbackEntryForm.tsx:164-175`: `*` after the label, 3 rows, no line under it. */}
      <div>
        <label htmlFor="nhap-ho-noi-dung" className={LABEL_CLASS}>
          {INTAKE_CONTENT_LABEL}
          <span className="ml-1 text-danger">*</span>
        </label>
        <textarea
          id="nhap-ho-noi-dung"
          name="nhap-ho-noi-dung"
          rows={3}
          required
          className={TEXTAREA_CLASS}
          value={values.content}
          placeholder={INTAKE_CONTENT_PLACEHOLDER}
          disabled={sending}
          onChange={(e) => set("content", e.target.value)}
        />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label={INTAKE_ADDRESS_LABEL} htmlFor="nhap-ho-dia-chi" icon={MapPin} grow="auto">
          <input
            id="nhap-ho-dia-chi"
            name="nhap-ho-dia-chi"
            value={values.address}
            placeholder={INTAKE_ADDRESS_PLACEHOLDER}
            autoComplete="off"
            disabled={sending}
            onChange={(e) => set("address", e.target.value)}
          />
        </Field>

        {/* §11 `Thôn, tổ dân phố` — placeholder (ADR 0068 §14); `id` unique on the page. */}
        <PendingField
          info={petitionPendingPart("intakeHamlet")}
          id="nhap-ho-thon"
          kind="select"
          placeholder="— Chưa xác định —"
          className="max-w-none flex-auto"
        />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label={INTAKE_NAME_LABEL} htmlFor="nhap-ho-nguoi-gui" icon={UserRound} grow="auto">
          <input
            id="nhap-ho-nguoi-gui"
            name="nhap-ho-nguoi-gui"
            value={values.reporterName}
            autoComplete="off"
            disabled={sending || values.anonymous}
            onChange={(e) => set("reporterName", e.target.value)}
          />
        </Field>
        <Field label={INTAKE_PHONE_LABEL} htmlFor="nhap-ho-so-dien-thoai" icon={Phone} grow="auto">
          <input
            id="nhap-ho-so-dien-thoai"
            name="nhap-ho-so-dien-thoai"
            type="tel"
            inputMode="tel"
            value={values.reporterPhone}
            autoComplete="off"
            disabled={sending || values.anonymous}
            onChange={(e) => set("reporterPhone", e.target.value)}
          />
        </Field>
      </div>

      {/* Prototype `FeedbackEntryForm.tsx:222-230`: 12.5px, a 14px brand box. */}
      <div>
        <label htmlFor="nhap-ho-an-danh" className="flex items-center gap-2 text-[12.5px] text-ink">
          <input
            id="nhap-ho-an-danh"
            type="checkbox"
            className="accent-brand size-3.5"
            checked={values.anonymous}
            disabled={sending}
            onChange={(e) => set("anonymous", e.target.checked)}
          />{" "}
          {INTAKE_ANONYMOUS_LABEL}
        </label>
      </div>

      <p className="m-0 text-xs text-ink-500">{INTAKE_CHANNEL_NOTE}</p>

      {/* §11 `⬆ Đính ảnh hiện trường` — placeholder (ADR 0068 §14): a disabled picker button, never a
          `type="file"` input, so no file can be chosen and none can be sent. */}
      <PendingButton info={petitionPendingPart("intakePhotos")} icon={<Glyph icon={Upload} />} className="self-start" />

      {refusal !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {refusal}
        </p>
      )}
      </div>

      <div className="cum-nut shrink-0 justify-end">
        <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
          {INTAKE_CANCEL}
        </Button>
        <Button type="submit" variant="primary" disabled={!canSend} aria-busy={sending}>
          <BusyLabel busy={sending}>{INTAKE_SUBMIT}</BusyLabel>
        </Button>
      </div>
    </form>
  );
}

/**
 * The success screen. THE LOOKUP CODE IS THE POINT: the citizen will not see this petition in their
 * app (ADR 0028 Bổ sung 2026-10-02 rows 1–3), so the code the officer reads out is their only handle.
 * Large, monospace-ish, selectable — and `role="status"` so a screen reader announces it.
 */
export function StaffIntakeDone({
  code,
  onClose,
  onAnother,
}: {
  code: string;
  onClose: () => void;
  onAnother: () => void;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    closeRef.current?.focus();
  }, []);
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start gap-3">
        <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-success-50 text-success-600">
          <CircleCheck className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <h2 id={TITLE_ID} className="m-0 text-lg leading-snug font-semibold text-ink-900">
          {INTAKE_DONE_TITLE}
        </h2>
      </div>
      <div role="status" className="rounded-xl border border-brand-100 bg-brand-50 px-4 py-4 text-center">
        <p className="m-0 text-xs font-semibold text-ink-700">{INTAKE_DONE_CODE_LABEL}</p>
        <p className="ma-muc m-0 mt-1 text-3xl font-bold tracking-wide break-all text-ink-900 tabular-nums select-all">
          {code}
        </p>
      </div>
      <Notice tone="info">{INTAKE_DONE_SENTENCE}</Notice>
      <div className="cum-nut justify-end">
        <Button type="button" variant="secondary" onClick={onAnother}>
          {INTAKE_DONE_ANOTHER}
        </Button>
        <Button ref={closeRef} type="button" variant="primary" onClick={onClose}>
          {INTAKE_DONE_CLOSE}
        </Button>
      </div>
    </div>
  );
}
