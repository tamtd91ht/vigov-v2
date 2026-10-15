"use client";

import {
  CircleCheck,
  CloudOff,
  House,
  Inbox,
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
import { pickableResidentialUnits } from "@/features/cau-hinh/nhan-thon";
import { khoaSauLanGhi } from "@/features/thu-chi/nhan-thu-chi";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi";
import {
  bookStaffIntake,
  listIntakeFields,
  uploadStaffScenePhoto,
  type StaffIntakeInput,
} from "@/lib/api/phieu-phan-anh";
import type {
  identity_danhSachThonToDanPhoRa,
  petitions_citizenFieldListOut,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { coQuyen } from "@/lib/quyen";

import {
  HAMLET_NONE,
  INTAKE_ADDRESS_LABEL,
  INTAKE_ADDRESS_PLACEHOLDER,
  INTAKE_ANONYMOUS_LABEL,
  INTAKE_BUTTON,
  INTAKE_CANCEL,
  INTAKE_CHANNEL_DEFAULT,
  INTAKE_CHANNEL_LABEL,
  INTAKE_CHANNEL_OPTIONS,
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
  INTAKE_HAMLET_LABEL,
  INTAKE_NAME_LABEL,
  INTAKE_PHONE_LABEL,
  INTAKE_PHOTO_BUTTON,
  INTAKE_PHOTO_MAX,
  INTAKE_PHOTO_NOTE,
  INTAKE_PHOTO_RETRY,
  INTAKE_PHOTOS_SOME_FAILED,
  INTAKE_PHOTOS_TITLE,
  INTAKE_SUBMIT,
  INTAKE_TITLE,
  intakeContentError,
  intakePhotosDropped,
  PETITION_INTAKE_PERMISSION,
} from "./nhan-phieu";
import { BusyLabel, buttonClass, Glyph, LABEL_CLASS, TEXTAREA_CLASS } from "./petition-ui";
import { afterPhotoUpload } from "./verification-photos";

/**
 * "Nhập hộ phản ánh" (spec §11) — an officer books a petition for a citizen who called, came to the
 * office or met the hamlet head. `POST /api/v1/citizen-reports` (`feedback.create`).
 *
 * CHANNEL AND PHOTOS (owner, 15/10/2026, tester bug sheet web-admin-141026; backend 7c739880): the
 * officer picks "Tiếp nhận qua kênh" — `can-bo-nhap-ho` (default), `tong-dai`, `web-xa`, `zalo-oa`. The
 * select left on its default sends NO `channel` (the server records "chosen" vs "default" apart). Up to
 * five scene photos are chosen here and uploaded ONE BY ONE AFTER the 201, to POST …/{code}/photos: the
 * petition is booked first and stays booked whatever happens to a photo — the success screen shows the
 * code, then each photo's outcome, with "Gửi lại" on a refusal of the moment. The hamlet select is REAL since ADR 0088 §1: the officer picks
 * one of the commune's units IN USE (`pickableResidentialUnits`, the server's own predicate), and the
 * server checks the id with identity before booking (400 `residential_unit_not_offered` / 503 shown
 * verbatim). The petition is linked to NO citizen account (row 1), so the success
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
  content: string;
  address: string;
  reporterName: string;
  reporterPhone: string;
  anonymous: boolean;
  /** ULID of the picked thôn / tổ dân phố, `""` = none. */
  residentialUnitId: string;
  /** `""` = the select was left on its default (`can-bo-nhap-ho`) — sent as no `channel` at all. */
  channel: string;
};

const EMPTY: FormValues = {
  field: "",
  content: "",
  address: "",
  reporterName: "",
  reporterPhone: "",
  anonymous: false,
  residentialUnitId: "",
  channel: "",
};

/**
 * One scene photo chosen on the form. `preview` is a browser-local `blob:` URL (never sent anywhere), or
 * `null` where the browser cannot make one; revoked when the photo leaves the form or the form unmounts.
 * The FILE NAME IS NEVER SHOWN nor sent (it can carry a person's name — rule 3).
 */
export type IntakePhoto = { readonly key: string; readonly file: File; readonly preview: string | null };

/** Where one photo stands after the petition was booked (`afterPhotoUpload`'s states). */
export type IntakePhotoState =
  | { readonly kind: "waiting" }
  | { readonly kind: "uploading"; readonly percent: number }
  | { readonly kind: "checking" }
  | ReturnType<typeof afterPhotoUpload>;

export type IntakePhotoUpload = IntakePhoto & { readonly state: IntakePhotoState };

function previewUrl(file: File): string | null {
  try {
    return typeof URL.createObjectURL === "function" ? URL.createObjectURL(file) : null;
  } catch {
    return null;
  }
}

function revokePreview(p: { readonly preview: string | null }): void {
  if (p.preview !== null && typeof URL.revokeObjectURL === "function") URL.revokeObjectURL(p.preview);
}

/**
 * Adds picked files to the chosen ones, keeping at most `INTAKE_PHOTO_MAX` (the server's `photo_limit`).
 * Returns how many were left out, so the form can SAY so instead of dropping them silently.
 */
export function addIntakePhotos(
  current: readonly IntakePhoto[],
  files: readonly File[],
  mint: (file: File) => IntakePhoto,
): { photos: readonly IntakePhoto[]; dropped: number } {
  const room = Math.max(0, INTAKE_PHOTO_MAX - current.length);
  const taken = files.slice(0, room);
  return { photos: [...current, ...taken.map(mint)], dropped: files.length - taken.length };
}

export function StaffIntakeForm({
  onClose,
  onBooked,
  onSendingChange,
  loadFields = listIntakeFields,
  loadUnits = layDanhSachThonToDanPho,
  book = bookStaffIntake,
  uploadPhoto = uploadStaffScenePhoto,
}: {
  onClose: () => void;
  onBooked: () => void;
  onSendingChange?: (sending: boolean) => void;
  /** Injected only by tests; the screen always reads the contract routes. */
  loadFields?: () => Promise<KetQua<petitions_citizenFieldListOut>>;
  loadUnits?: () => Promise<KetQua<identity_danhSachThonToDanPhoRa>>;
  book?: (input: StaffIntakeInput, key: string) => Promise<KetQua<Pick<petitions_phieuPhanAnhRa, "code">>>;
  uploadPhoto?: typeof uploadStaffScenePhoto;
}) {
  const [fieldsReloads, setFieldsReloads] = useState(0);
  const [fields, setFields] = useState<{ key: number; result: KetQua<petitions_citizenFieldListOut> } | null>(null);
  const [values, setValues] = useState<FormValues>(EMPTY);
  /** ONE key per opening of the draft: kept across retries, renewed only after a 201 (`khoaSauLanGhi`). */
  const [key, setKey] = useState(khoaChongTrungMoi);
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const [bookedCode, setBookedCode] = useState<string | null>(null);
  const [units, setUnits] = useState<KetQua<identity_danhSachThonToDanPhoRa> | null>(null);
  const [photos, setPhotos] = useState<readonly IntakePhoto[]>([]);
  const [photosDropped, setPhotosDropped] = useState(0);
  const [uploads, setUploads] = useState<readonly IntakePhotoUpload[]>([]);
  const [photosBusy, setPhotosBusy] = useState(false);
  const seq = useRef(0);
  // Every preview URL still alive, so an unmount releases them all.
  const previews = useRef(new Set<string>());

  useEffect(() => {
    const alive = previews.current;
    return () => {
      for (const url of alive) URL.revokeObjectURL(url);
      alive.clear();
    };
  }, []);

  function mintPhoto(file: File): IntakePhoto {
    seq.current += 1;
    const preview = previewUrl(file);
    if (preview !== null) previews.current.add(preview);
    return { key: `anh-nhap-ho-${seq.current}`, file, preview };
  }

  function dropPhoto(p: { readonly preview: string | null }): void {
    if (p.preview !== null && previews.current.delete(p.preview)) revokePreview(p);
  }

  const setUpload = (key: string, state: IntakePhotoState) =>
    setUploads((list) => list.map((u) => (u.key === key ? { ...u, state } : u)));

  /**
   * ONE photo, ONE multipart request, ONE NEW Idempotency-Key per attempt (`lib/api/upload.ts`): a retry
   * is only offered where nothing was stored (`isTransientUpload`), so a fresh key cannot store twice.
   * The content type is the browser's; `""` leaves the part out and the server sniffs the bytes.
   */
  async function uploadOne(code: string, item: IntakePhoto): Promise<void> {
    setUpload(item.key, { kind: "uploading", percent: 0 });
    const r = await uploadPhoto(code, item.file, item.file.type, crypto.randomUUID(), {
      onProgress: (percent) => setUpload(item.key, { kind: "uploading", percent }),
      onSent: () => setUpload(item.key, { kind: "checking" }),
    });
    setUpload(item.key, afterPhotoUpload(r));
  }

  /** SEQUENTIAL, never parallel: the server counts the five under the petition's row lock. */
  async function uploadAll(code: string, items: readonly IntakePhoto[]): Promise<void> {
    if (items.length === 0) return;
    setPhotosBusy(true);
    onSendingChange?.(true);
    for (const item of items) await uploadOne(code, item);
    setPhotosBusy(false);
    onSendingChange?.(false);
  }

  // The commune's units, read once per opening. A failure leaves the select with its empty choice only
  // and says why under it — the petition can still be booked without a hamlet.
  useEffect(() => {
    let dropped = false;
    loadUnits().then((r) => {
      if (!dropped) setUnits(r);
    });
    return () => {
      dropped = true;
    };
  }, [loadUnits]);

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
    if (values.field === "" || intakeContentError(values.content) !== null || sending) return;
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
        // NO `Dân phản ánh lúc` ON THE FORM (bug sheet: the box was removed; prototype
        // `FeedbackEntryForm.tsx` has none). `""` leaves `clock_from` out of the body, and the server
        // then counts the deadline from the booking instant (`staff_intake.go`, `ClockFrom`).
        clockFrom: "",
        residentialUnitId: values.residentialUnitId,
        channel: values.channel,
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
      const code = r.duLieu.code;
      setBookedCode(code);
      onBooked();
      // THE PETITION IS BOOKED NOW, whatever happens next: the photos follow, and a photo that fails is
      // reported on the success screen next to the code — it never undoes or hides the booking.
      const chosen = photos;
      setPhotos([]);
      setPhotosDropped(0);
      setUploads(chosen.map((p) => ({ ...p, state: { kind: "waiting" } })));
      void uploadAll(code, chosen);
    });
  }

  if (bookedCode !== null) {
    return (
      <StaffIntakeDone
        code={bookedCode}
        photos={uploads}
        photosBusy={photosBusy}
        onRetryPhoto={(key) => {
          const it = uploads.find((u) => u.key === key);
          if (it === undefined || it.state.kind !== "retry" || photosBusy) return;
          void uploadAll(bookedCode, [it]);
        }}
        onClose={onClose}
        onAnother={() => {
          for (const u of uploads) dropPhoto(u);
          setUploads([]);
          setBookedCode(null);
          setValues(EMPTY);
        }}
      />
    );
  }

  return (
    <StaffIntakeFormView
      fields={currentFields}
      units={units}
      values={values}
      setValues={setValues}
      sending={sending}
      refusal={refusal}
      onSubmit={send}
      onCancel={onClose}
      onReloadFields={() => setFieldsReloads((n) => n + 1)}
      photos={photos}
      photosDropped={photosDropped}
      onAddPhotos={(files) => {
        const next = addIntakePhotos(photos, files, mintPhoto);
        setPhotos(next.photos);
        setPhotosDropped(next.dropped);
      }}
      onRemovePhoto={(key) => {
        const it = photos.find((p) => p.key === key);
        if (it !== undefined) dropPhoto(it);
        setPhotos((list) => list.filter((p) => p.key !== key));
        setPhotosDropped(0);
      }}
    />
  );
}

/** Presentational — rendered to a string in tests. `fields === null` = loading. */
export function StaffIntakeFormView({
  fields,
  units = null,
  values,
  setValues = () => {},
  sending = false,
  refusal = null,
  onSubmit = () => {},
  onCancel = () => {},
  onReloadFields,
  photos = [],
  photosDropped = 0,
  onAddPhotos = () => {},
  onRemovePhoto = () => {},
}: {
  fields: KetQua<petitions_citizenFieldListOut> | null;
  /** The commune's residential units; `null` = loading. Only units IN USE are offered. */
  units?: KetQua<identity_danhSachThonToDanPhoRa> | null;
  values: FormValues;
  setValues?: (v: FormValues) => void;
  sending?: boolean;
  refusal?: string | null;
  onSubmit?: () => void;
  onCancel?: () => void;
  onReloadFields?: () => void;
  /** The scene photos chosen so far (at most `INTAKE_PHOTO_MAX`); uploaded after the booking. */
  photos?: readonly IntakePhoto[];
  /** How many picked files the last pick left out over the limit — said, never silent. */
  photosDropped?: number;
  onAddPhotos?: (files: File[]) => void;
  onRemovePhoto?: (key: string) => void;
}) {
  const set = <K extends keyof FormValues>(k: K, v: FormValues[K]) => setValues({ ...values, [k]: v });
  const contentError = intakeContentError(values.content);
  const items = fields !== null && fields.ok ? fields.duLieu.items : [];
  const fieldsReady = fields !== null && fields.ok && items.length > 0;
  // A NEW record: only units in use — the same predicate the server applies to the id it receives.
  const unitChoices = units !== null && units.ok ? pickableResidentialUnits(units.duLieu.items) : [];
  const canSend = fieldsReady && values.field !== "" && contentError === null && !sending;

  return (
    // The prototype's dialog body (`FeedbackEntryForm.tsx:132-286`): header, then the fields in its
    // order — Lĩnh vực · Nội dung · [Địa chỉ | Thôn] · [Người gửi | Số điện thoại] · ẩn danh · kênh ·
    // ảnh — then Huỷ / Vào sổ phản ánh. No `Dân phản ánh lúc` box (bug sheet, as the prototype): the
    // deadline counts from the booking instant. The header and the buttons stay in sight; the fields
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

        {/* §11 `Thôn, tổ dân phố` (prototype `FeedbackEntryForm.tsx:186-199`); `id` unique on the page. */}
        <Field
          label={INTAKE_HAMLET_LABEL}
          htmlFor="nhap-ho-thon"
          icon={House}
          kind="select"
          grow="auto"
          className="max-w-none"
          error={units !== null && !units.ok ? units.thongBao : undefined}
        >
          <select
            id="nhap-ho-thon"
            name="nhap-ho-thon"
            value={values.residentialUnitId}
            disabled={sending || units === null}
            onChange={(e) => set("residentialUnitId", e.target.value)}
          >
            <option value="">{HAMLET_NONE}</option>
            {unitChoices.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </Field>
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

      {/* "Tiếp nhận qua kênh" — prototype `FeedbackEntryForm.tsx:232-247`: a full-width select after the
          anonymous box. The tester's four channels, in the tester's order (bug sheet web-admin-141026). */}
      <Field label={INTAKE_CHANNEL_LABEL} htmlFor="nhap-ho-kenh" icon={Inbox} kind="select" grow="auto" className="max-w-none">
        <select
          id="nhap-ho-kenh"
          name="nhap-ho-kenh"
          value={values.channel === "" ? INTAKE_CHANNEL_DEFAULT : values.channel}
          disabled={sending}
          onChange={(e) => set("channel", e.target.value)}
        >
          {INTAKE_CHANNEL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </Field>

      <IntakePhotoPicker
        photos={photos}
        dropped={photosDropped}
        disabled={sending}
        onAdd={onAddPhotos}
        onRemove={onRemovePhoto}
      />

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
  photos = [],
  photosBusy = false,
  onRetryPhoto = () => {},
}: {
  code: string;
  onClose: () => void;
  onAnother: () => void;
  /** The photos chosen on the form and where each stands; `[]` = none were chosen. */
  photos?: readonly IntakePhotoUpload[];
  /** A photo is on its way: both buttons wait, so the outcome of every photo is seen. */
  photosBusy?: boolean;
  onRetryPhoto?: (key: string) => void;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  // Focus goes to `Đóng` once nothing is in flight; while photos upload it is disabled, so the title
  // holds focus (inside the dialog) instead.
  useEffect(() => {
    if (photosBusy) titleRef.current?.focus();
    else closeRef.current?.focus();
  }, [photosBusy]);
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start gap-3">
        <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-success-50 text-success-600">
          <CircleCheck className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <h2 id={TITLE_ID} ref={titleRef} tabIndex={-1} className="m-0 text-lg leading-snug font-semibold text-ink-900">
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
      {photos.length > 0 && <IntakePhotoOutcomes photos={photos} busy={photosBusy} onRetry={onRetryPhoto} />}
      <div className="cum-nut justify-end">
        <Button type="button" variant="secondary" onClick={onAnother} disabled={photosBusy}>
          {INTAKE_DONE_ANOTHER}
        </Button>
        <Button ref={closeRef} type="button" variant="primary" onClick={onClose} disabled={photosBusy}>
          {INTAKE_DONE_CLOSE}
        </Button>
      </div>
    </div>
  );
}

/**
 * §11 `⬆ Đính ảnh hiện trường` (prototype `FeedbackEntryForm.tsx:249-270`): a real file input, visually
 * hidden but focusable, whose label is the visible button (the `verification-photos.tsx` pattern), then
 * the chosen photos as thumbnails, each with its ×. Nothing is sent from here — the photos go up after
 * the booking (`StaffIntakeForm`).
 */
function IntakePhotoPicker({
  photos,
  dropped,
  disabled,
  onAdd,
  onRemove,
}: {
  photos: readonly IntakePhoto[];
  dropped: number;
  disabled: boolean;
  onAdd: (files: File[]) => void;
  onRemove: (key: string) => void;
}) {
  const full = photos.length >= INTAKE_PHOTO_MAX;
  const off = disabled || full;
  return (
    <div className="flex flex-col gap-2">
      <div>
        <input
          id="nhap-ho-anh"
          name="nhap-ho-anh"
          type="file"
          multiple
          accept="image/*"
          className="an-thi-giac peer"
          disabled={off}
          onChange={(e) => {
            const files = e.target.files === null ? [] : Array.from(e.target.files);
            e.target.value = "";
            if (files.length > 0) onAdd(files);
          }}
        />
        <label
          htmlFor="nhap-ho-anh"
          aria-disabled={off}
          className={cn(
            buttonClass("secondary", "md"),
            "peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-brand-500",
            off && "pointer-events-none opacity-50",
          )}
        >
          <Glyph icon={Upload} />
          {INTAKE_PHOTO_BUTTON}
          {photos.length > 0 && <span className="text-ink-500"> ({photos.length}/{INTAKE_PHOTO_MAX})</span>}
        </label>
      </div>
      <p className="m-0 text-xs text-ink-500">{INTAKE_PHOTO_NOTE}</p>
      {dropped > 0 && (
        <p className="thong-bao-loi m-0 text-xs" role="alert">
          {intakePhotosDropped(dropped)}
        </p>
      )}
      {photos.length > 0 && (
        <ul aria-label="Ảnh hiện trường đã chọn" className="m-0 flex list-none flex-wrap gap-2 p-0">
          {photos.map((p, i) => {
            const label = `Ảnh đã chọn ${i + 1}/${photos.length}`;
            return (
              <li key={p.key} className="relative size-20 overflow-hidden rounded-[10px] border border-line bg-surface-muted">
                <PhotoThumb preview={p.preview} label={label} />
                <button
                  type="button"
                  aria-label={`Bỏ ${label.toLowerCase()}`}
                  title="Bỏ ảnh"
                  disabled={disabled}
                  onClick={() => onRemove(p.key)}
                  className={cn(
                    "absolute top-1 right-1 grid size-6 cursor-pointer place-items-center rounded-full border-0 bg-ink-900/70 p-0 text-white",
                    "hover:bg-ink-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
                  )}
                >
                  <Glyph icon={X} className="size-3.5" />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

function PhotoThumb({ preview, label }: { preview: string | null; label: string }) {
  return preview !== null ? (
    // eslint-disable-next-line @next/next/no-img-element -- a local blob: preview, never a remote URL
    <img src={preview} alt={label} decoding="async" className="block size-full object-cover" />
  ) : (
    <span className="grid size-full place-items-center px-1 text-center text-[11px] text-ink-500">{label}</span>
  );
}

function photoStateText(s: IntakePhotoState): string {
  switch (s.kind) {
    case "waiting":
      return "Chờ tải lên";
    case "uploading":
      return `Đang tải ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra (quét mã độc, xử lý ảnh)…";
    case "stored":
      return "Đã đính vào phiếu.";
    case "retry":
      return `Chưa gửi được: ${s.message}`;
    case "refused":
      return `Bị từ chối: ${s.message}`;
  }
}

/** The success screen's photo list: one line per photo, a refusal read where the photo is. */
function IntakePhotoOutcomes({
  photos,
  busy,
  onRetry,
}: {
  photos: readonly IntakePhotoUpload[];
  busy: boolean;
  onRetry: (key: string) => void;
}) {
  const failed = photos.some((p) => p.state.kind === "retry" || p.state.kind === "refused");
  return (
    <section aria-label={INTAKE_PHOTOS_TITLE} className="flex flex-col gap-2">
      <h3 className="m-0 text-sm font-semibold text-ink-900">{INTAKE_PHOTOS_TITLE}</h3>
      {/* Not `role="alert"`: each failed photo's own line already is one. */}
      {failed && !busy && <p className="thong-bao-loi m-0 text-xs">{INTAKE_PHOTOS_SOME_FAILED}</p>}
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        {photos.map((p, i) => {
          const bad = p.state.kind === "retry" || p.state.kind === "refused";
          const label = `Ảnh ${i + 1}/${photos.length}`;
          return (
            <li key={p.key} className="flex min-w-0 items-center gap-3">
              <span className="size-12 shrink-0 overflow-hidden rounded-lg border border-line bg-surface-muted">
                <PhotoThumb preview={p.preview} label={label} />
              </span>
              <span className="min-w-0 flex-1 text-xs">
                <span className="font-semibold text-ink-900">{label}: </span>
                <span role={bad ? "alert" : "status"} className={bad ? "text-danger-600" : "text-ink-500"}>
                  {photoStateText(p.state)}
                </span>
              </span>
              {p.state.kind === "retry" && (
                <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={() => onRetry(p.key)}>
                  {INTAKE_PHOTO_RETRY}
                </Button>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
