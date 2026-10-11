"use client";

import { CalendarClock, LoaderCircle, Plus, Save, X } from "lucide-react";
import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
import type { KetQua } from "@/lib/api/goi";
import type { CreateDisasterAlertBody, DisasterAlert, UpdateDisasterAlertBody } from "@/lib/api/disaster-alerts";

import {
  ADD_CONTACT,
  ADD_EVACUATION,
  ALERT_KINDS,
  CANCEL_BUTTON,
  CLOSE_LABEL,
  CONTACT_NAME,
  CONTACT_PHONE,
  CONTACT_ROLE,
  CONTACTS_PUBLIC_HINT,
  createBody,
  CREATE_TITLE,
  DETAIL_MAX,
  EDIT_TITLE,
  EMPTY_CONTACT_ROW,
  EMPTY_EVACUATION_ROW,
  EMPTY_FORM,
  EVAC_ADDRESS,
  EVAC_CAPACITY,
  EVAC_NAME,
  EVAC_PHONE,
  FORM_DESCRIPTION,
  formFromAlert,
  hasErrors,
  HEADLINE_HINT,
  HEADLINE_MAX,
  KIND_PLACEHOLDER,
  LABEL_CONTACTS,
  LABEL_DETAIL,
  LABEL_EVACUATION,
  LABEL_FROM,
  LABEL_HEADLINE,
  LABEL_KIND,
  LABEL_LEVEL,
  LABEL_TITLE,
  LABEL_TO,
  LABEL_ZONE,
  LEVEL_HINT,
  LEVEL_MAX,
  LIST_MAX_ROWS,
  localizeAlertRefusal,
  NOTHING_CHANGED,
  patchBody,
  REMOVE_ROW,
  SAVE_BUTTON,
  TIME_HINT,
  TITLE_MAX,
  validateAlertForm,
  ZONE_HINT,
  type AlertFormValues,
  type ContactRow,
  type EvacuationRow,
} from "./alert-form";
import { DangerZoneEditor } from "./danger-zone-editor";

const TITLE_ID = "disaster-alert-form-title";

/** 720px — wider than the content editor (600px): the canvas and the two repeatable lists sit side by side. */
const DIALOG_CLASS = "w-[min(45rem,calc(100vw-1rem))] p-0";

/**
 * Create / edit one alert, in the content screens' overlay (`OverlayDialog`, ADR 0068).
 *
 * THE DIALOG OWNS THE CALL; the parent owns the list. `create` / `update` are injected so the tests see the
 * exact body sent. The `Idempotency-Key` is made ONCE, when the dialog opens: a retry after a network error
 * reuses it, because the first attempt may already have reached residents.
 *
 * Problems are shown once Lưu was pressed (not while the officer is still typing the first letter), then
 * live. The server's 400 sentence still reaches the form, with only its field token in Vietnamese.
 *
 * EVERY TEXT IS PLAIN TEXT. The staff routes return `title` / `detail` / list cells as typed, unsanitised
 * (only the public route plain-texts them); they are only ever React text nodes or input values here —
 * never HTML (rule 13, forbidden #3).
 */
export function AlertFormDialog({
  alert,
  create,
  update,
  saved,
  close,
}: {
  /** The alert being edited; absent = a new one. */
  alert?: DisasterAlert;
  create: (body: CreateDisasterAlertBody, idempotencyKey: string) => Promise<KetQua<DisasterAlert>>;
  update: (id: string, body: UpdateDisasterAlertBody) => Promise<KetQua<DisasterAlert>>;
  saved: (row: DisasterAlert, created: boolean) => void;
  close: () => void;
}) {
  const [initial] = useState<AlertFormValues>(() => (alert === undefined ? EMPTY_FORM : formFromAlert(alert)));
  const [v, setV] = useState<AlertFormValues>(initial);
  const [idempotencyKey] = useState(khoaChongTrungMoi);
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const editing = alert !== undefined;
  const errors = touched ? validateAlertForm(v) : {};

  function set<K extends keyof AlertFormValues>(key: K, value: AlertFormValues[K]): void {
    setV((old) => ({ ...old, [key]: value }));
  }

  function setEvacuation(i: number, patch: Partial<EvacuationRow>): void {
    setV((old) => ({ ...old, evacuation: old.evacuation.map((r, j) => (j === i ? { ...r, ...patch } : r)) }));
  }

  function setContact(i: number, patch: Partial<ContactRow>): void {
    setV((old) => ({ ...old, contacts: old.contacts.map((r, j) => (j === i ? { ...r, ...patch } : r)) }));
  }

  function dismiss(): void {
    if (!busy) close();
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (busy) return;
    setTouched(true);
    if (hasErrors(validateAlertForm(v))) return;
    let call: Promise<KetQua<DisasterAlert>>;
    if (editing) {
      const body = patchBody(initial, v);
      if (Object.keys(body).length === 0) {
        setServerError(NOTHING_CHANGED);
        return;
      }
      call = update(alert.id, body);
    } else {
      call = create(createBody(v), idempotencyKey);
    }
    setBusy(true);
    setServerError(null);
    void call.then((r) => {
      setBusy(false);
      if (r.ok) {
        saved(r.duLieu, !editing);
        return;
      }
      setServerError(localizeAlertRefusal(r.thongBao));
    });
  }

  return (
    <OverlayDialog titleId={TITLE_ID} onDismiss={dismiss} className={DIALOG_CLASS}>
      <form className="m-0 flex flex-col" onSubmit={submit} noValidate aria-labelledby={TITLE_ID}>
        <div className="flex items-start gap-3 px-5 pt-5 pb-3">
          <div className="min-w-0 flex-1">
            <h3 id={TITLE_ID} className="m-0 text-lg leading-snug font-semibold text-navy">
              {editing ? EDIT_TITLE : CREATE_TITLE}
            </h3>
            <p className="m-0 mt-1 text-[13px] text-ink-muted">{FORM_DESCRIPTION}</p>
          </div>
          <IconButton type="button" label={CLOSE_LABEL} className="-mt-1 -mr-2 min-h-0" disabled={busy} onClick={dismiss}>
            <X aria-hidden="true" focusable="false" />
          </IconButton>
        </div>

        <div className="relative flex max-h-[70vh] min-h-0 flex-col gap-4 overflow-y-auto px-5 pb-5 [&>*]:shrink-0">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={LABEL_KIND} htmlFor="alert-kind" kind="select" required grow="auto" error={errors.kind}>
              <select id="alert-kind" value={v.kind} required onChange={(e) => set("kind", e.target.value)}>
                <option value="">{KIND_PLACEHOLDER}</option>
                {ALERT_KINDS.map((k) => (
                  <option key={k.code} value={k.code}>
                    {k.label}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={LABEL_LEVEL} htmlFor="alert-level" grow="auto" hint={LEVEL_HINT} error={errors.level}>
              <input
                id="alert-level"
                value={v.level}
                maxLength={LEVEL_MAX}
                autoComplete="off"
                onChange={(e) => set("level", e.target.value)}
              />
            </Field>
          </div>

          <Field label={LABEL_TITLE} htmlFor="alert-title" required grow="auto" error={errors.title}>
            <input
              id="alert-title"
              value={v.title}
              maxLength={TITLE_MAX}
              required
              autoComplete="off"
              onChange={(e) => set("title", e.target.value)}
            />
          </Field>

          <Field label={LABEL_HEADLINE} htmlFor="alert-headline" grow="auto" hint={HEADLINE_HINT} error={errors.headline}>
            <input
              id="alert-headline"
              value={v.headline}
              maxLength={HEADLINE_MAX}
              autoComplete="off"
              onChange={(e) => set("headline", e.target.value)}
            />
          </Field>

          <Field
            label={LABEL_DETAIL}
            htmlFor="alert-detail"
            required
            grow="auto"
            error={errors.detail}
            className="[&_textarea]:py-2"
          >
            <textarea
              id="alert-detail"
              rows={5}
              value={v.detail}
              maxLength={DETAIL_MAX}
              required
              onChange={(e) => set("detail", e.target.value)}
            />
          </Field>

          <div className="flex flex-col gap-1.5">
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label={LABEL_FROM} htmlFor="alert-from" icon={CalendarClock} required grow="auto" error={errors.from}>
                <input
                  id="alert-from"
                  type="datetime-local"
                  value={v.fromLocal}
                  required
                  onChange={(e) => set("fromLocal", e.target.value)}
                />
              </Field>
              <Field label={LABEL_TO} htmlFor="alert-to" icon={CalendarClock} grow="auto" error={errors.to}>
                <input id="alert-to" type="datetime-local" value={v.toLocal} onChange={(e) => set("toLocal", e.target.value)} />
              </Field>
            </div>
            <p className="m-0 text-[12px] text-ink-muted">{TIME_HINT}</p>
          </div>

          <section className="flex flex-col gap-2" aria-labelledby="alert-zone-label">
            <h4 id="alert-zone-label" className="m-0 text-[13px] font-semibold text-navy">
              {LABEL_ZONE}
            </h4>
            <p className="m-0 text-[12px] text-ink-muted">{ZONE_HINT}</p>
            <DangerZoneEditor value={v.zone} onChange={(z) => set("zone", z)} disabled={busy} error={errors.zone} />
          </section>

          <section className="flex flex-col gap-2" aria-labelledby="alert-evac-label">
            <h4 id="alert-evac-label" className="m-0 text-[13px] font-semibold text-navy">
              {LABEL_EVACUATION}
            </h4>
            {v.evacuation.map((r, i) => (
              <div key={i} className="grid gap-2 rounded-lg border border-line p-3 sm:grid-cols-[1fr_1fr_10rem_6rem_auto]" data-testid="evacuation-row">
                <Field label={`${EVAC_NAME} (điểm ${i + 1})`} htmlFor={`evac-name-${i}`} required grow="auto">
                  <input id={`evac-name-${i}`} value={r.name} autoComplete="off" onChange={(e) => setEvacuation(i, { name: e.target.value })} />
                </Field>
                <Field label={EVAC_ADDRESS} htmlFor={`evac-address-${i}`} grow="auto">
                  <input id={`evac-address-${i}`} value={r.address} autoComplete="off" onChange={(e) => setEvacuation(i, { address: e.target.value })} />
                </Field>
                <Field label={EVAC_PHONE} htmlFor={`evac-phone-${i}`} grow="auto">
                  <input id={`evac-phone-${i}`} type="tel" value={r.phone} autoComplete="off" onChange={(e) => setEvacuation(i, { phone: e.target.value })} />
                </Field>
                <Field label={EVAC_CAPACITY} htmlFor={`evac-capacity-${i}`} grow="auto">
                  <input
                    id={`evac-capacity-${i}`}
                    type="number"
                    inputMode="numeric"
                    min={0}
                    step={1}
                    value={r.capacity}
                    onChange={(e) => setEvacuation(i, { capacity: e.target.value })}
                  />
                </Field>
                <div className="flex items-end">
                  <IconButton
                    type="button"
                    label={`${REMOVE_ROW} — điểm sơ tán ${i + 1}`}
                    disabled={busy}
                    onClick={() => set("evacuation", v.evacuation.filter((_, j) => j !== i))}
                  >
                    <X aria-hidden="true" focusable="false" />
                  </IconButton>
                </div>
              </div>
            ))}
            {errors.evacuation !== undefined && (
              <p role="alert" className="m-0 text-[12px] font-medium text-danger">
                {errors.evacuation}
              </p>
            )}
            <div>
              <Button
                type="button"
                size="sm"
                disabled={busy || v.evacuation.length >= LIST_MAX_ROWS}
                onClick={() => set("evacuation", [...v.evacuation, EMPTY_EVACUATION_ROW])}
                icon={<Plus aria-hidden="true" focusable="false" />}
              >
                {ADD_EVACUATION}
              </Button>
            </div>
          </section>

          <section className="flex flex-col gap-2" aria-labelledby="alert-contacts-label">
            <h4 id="alert-contacts-label" className="m-0 text-[13px] font-semibold text-navy">
              {LABEL_CONTACTS}
            </h4>
            <p className="m-0 text-[12px] text-ink-muted" data-testid="contacts-public-hint">
              {CONTACTS_PUBLIC_HINT}
            </p>
            {v.contacts.map((c, i) => (
              <div key={i} className="grid gap-2 rounded-lg border border-line p-3 sm:grid-cols-[1fr_1fr_10rem_auto]" data-testid="contact-row">
                <Field label={`${CONTACT_ROLE} (số ${i + 1})`} htmlFor={`contact-role-${i}`} required grow="auto">
                  <input id={`contact-role-${i}`} value={c.role} autoComplete="off" onChange={(e) => setContact(i, { role: e.target.value })} />
                </Field>
                <Field label={CONTACT_NAME} htmlFor={`contact-name-${i}`} required grow="auto">
                  <input id={`contact-name-${i}`} value={c.name} autoComplete="off" onChange={(e) => setContact(i, { name: e.target.value })} />
                </Field>
                <Field label={CONTACT_PHONE} htmlFor={`contact-phone-${i}`} required grow="auto">
                  <input id={`contact-phone-${i}`} type="tel" value={c.phone} autoComplete="off" onChange={(e) => setContact(i, { phone: e.target.value })} />
                </Field>
                <div className="flex items-end">
                  <IconButton
                    type="button"
                    label={`${REMOVE_ROW} — số khẩn cấp ${i + 1}`}
                    disabled={busy}
                    onClick={() => set("contacts", v.contacts.filter((_, j) => j !== i))}
                  >
                    <X aria-hidden="true" focusable="false" />
                  </IconButton>
                </div>
              </div>
            ))}
            {errors.contacts !== undefined && (
              <p role="alert" className="m-0 text-[12px] font-medium text-danger">
                {errors.contacts}
              </p>
            )}
            <div>
              <Button
                type="button"
                size="sm"
                disabled={busy || v.contacts.length >= LIST_MAX_ROWS}
                onClick={() => set("contacts", [...v.contacts, EMPTY_CONTACT_ROW])}
                icon={<Plus aria-hidden="true" focusable="false" />}
              >
                {ADD_CONTACT}
              </Button>
            </div>
          </section>

          {serverError !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {serverError}
            </p>
          )}
        </div>

        <div className="flex flex-wrap justify-end gap-2 border-t border-line px-5 py-3">
          <Button type="button" variant="secondary" disabled={busy} onClick={dismiss}>
            {CANCEL_BUTTON}
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={busy}
            aria-busy={busy || undefined}
            icon={
              busy ? (
                <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />
              ) : (
                <Save aria-hidden="true" focusable="false" />
              )
            }
          >
            {SAVE_BUTTON}
          </Button>
        </div>
      </form>
    </OverlayDialog>
  );
}
