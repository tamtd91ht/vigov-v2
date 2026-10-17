"use client";

import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { pickableResidentialUnits } from "@/features/cau-hinh/nhan-thon";
import {
  contactEmailMasked,
  createCitizenContact,
  getCitizenContact,
  revealCitizenContactPhone,
  updateCitizenContact,
  type CitizenAttributes,
  type CitizenContact,
  type CitizenContactBody,
} from "@/lib/api/citizen-contacts";
import type { CitizenField } from "@/lib/api/citizen-fields";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import type { identity_thonToDanPhoRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { CitizenAttributeInputs } from "./citizen-attribute-inputs";
import { attributesFromInputs, inputsFromAttributes, type AttributeInputs } from "./citizen-attributes";
import {
  ADDED_TOAST,
  CANCEL_BUTTON,
  CONTACT_ADD_TITLE,
  CONTACT_DIALOG_DESCRIPTION,
  CONTACT_EDIT_TITLE,
  DETAIL_LOADING,
  EDIT_EMAIL_BUTTON,
  EMAIL_REPLACE_HINT,
  EMPTY_CONTACT_DRAFT,
  FIELD_ACTIVE,
  FIELD_ADDRESS,
  FIELD_EMAIL,
  FIELD_FULL_NAME,
  FIELD_GROUPS,
  FIELD_NOTE,
  FIELD_PHONE,
  FIELD_TAGS,
  FIELD_UNIT,
  NO_GROUPS_IN_DIALOG,
  REVEAL_BEFORE_SAVE_HINT,
  REVEAL_PHONE_BUTTON,
  REVEAL_PHONE_FAILED,
  SAVED_TOAST,
  SUBMIT_ADD,
  SUBMIT_EDIT,
  TAGS_HINT,
  TAGS_WARNING,
  UNIT_UNKNOWN,
  KEEP_EMAIL_BUTTON,
  canSubmitContact,
  emailProblem,
  parseTags,
  saveFailureToast,
  toggleGroup,
  type ContactDraft,
} from "./citizen-directory-words";

export const CONTACT_DIALOG_TITLE_ID = "tieu-de-nguoi-dan"; // vi-name-ok: DOM id, same convention as the app's other dialog ids
export const REVEAL_PHONE_BUTTON_ID = "xem-so-day-du"; // vi-name-ok: DOM id, same convention

/** What the dialog was opened for: adding (with the key of THIS opening) or editing one row. */
export type ContactDialogMode =
  | { readonly kind: "add"; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly contact: CitizenContact };

/**
 * The number field's state in EDIT mode (ADR 0097 D2): masked until "Xem số đầy đủ" is pressed; ONE
 * audited call per click; the full number lives here and nowhere else — closing the dialog forgets it,
 * reopening is a new entry on the server. In ADD mode the field is a plain input (`shown` from the start).
 */
type PhoneState =
  | { readonly kind: "masked"; readonly error: string | null }
  | { readonly kind: "loading" }
  | { readonly kind: "shown" };

const SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-8 w-full rounded-lg border border-solid bg-white pr-9 pl-2.5 text-sm outline-none focus-visible:ring-[3px]";

/**
 * "Thêm người dân" / "Sửa thông tin người dân" — the prototype's `CitizenForm` (chapter 16 §5) in the
 * repo's `ModalDialog`: 42rem, scrolls inside, two columns from `sm`. The deviations from the prototype
 * are the chapter's: no `Mã Zalo` field (D3), the masked number plus "Xem số đầy đủ" on edit (D2), the
 * fixed sentence under the tags (D4), the `Đang liên hệ` box (D6).
 *
 * EDIT MODE READS THE DETAIL ON OPEN (`GET /citizen-contacts/{id}`: address and note are not on the list
 * row, D13) — the number arrives MASKED there too. The full number is read ONLY by the reveal button
 * (`canReveal` = `citizen.update`; the server checks again and a 403 keeps the mask).
 *
 * `Lưu thay đổi` STAYS DISABLED UNTIL THE NUMBER IS REVEALED: the PATCH body must carry the full number
 * (the server normalises what it gets and refuses a mask), and sending a silently fetched number would
 * be an audited reveal nobody clicked. The hint under the field says so (`REVEAL_BEFORE_SAVE_HINT`).
 *
 * The refusals of a save are toasts, as the prototype: `phone_taken` → the chapter's sentence, anything
 * else → its other sentence (`saveFailureToast`). Nothing typed here is logged.
 *
 * ADR 0101 ADDS two things. `Email` (optional, #6) after the number — on edit the stored address arrives
 * MASKED, so it is drawn read-only and replaced only through "Nhập email mới"; until then the body carries
 * no `email` at all, never the mask. And the commune's live attributes (#3) after the static fields, in the
 * configured order (`CitizenAttributeInputs`), checked here as the server checks them before anything is
 * sent. The values of retired fields are carried over untouched (`attributesFromInputs`).
 */
export function CitizenContactDialog({
  mode,
  groups,
  units,
  fields = [],
  canReveal,
  onClose,
  onSaved,
}: {
  mode: ContactDialogMode;
  groups: readonly CitizenGroup[];
  /** The commune's LIVE attribute definitions, in sort order (`GET /citizen-fields`). Empty = none drawn. */
  fields?: readonly CitizenField[];
  units: readonly identity_thonToDanPhoRa[];
  /** Session holds `citizen.update` — draws the reveal button. Convenience only; the route checks. */
  canReveal: boolean;
  onClose: () => void;
  /** A 201 / 204: the list, the count, the tags and the groups must be read again. */
  onSaved: () => void;
}) {
  const editing = mode.kind === "edit";
  const [draft, setDraft] = useState<ContactDraft>(EMPTY_CONTACT_DRAFT);
  /** `null` while the detail is being read (edit) or when reading it failed; adding starts ready. */
  const [ready, setReady] = useState(!editing);
  const [detailError, setDetailError] = useState("");
  const [phone, setPhone] = useState<PhoneState>(editing ? { kind: "masked", error: null } : { kind: "shown" });
  const [saving, setSaving] = useState(false);
  /** The contact's saved attributes (edit) — the base the body is built on, so retired values survive. */
  const [stored, setStored] = useState<CitizenAttributes | undefined>(undefined);
  const [attrInputs, setAttrInputs] = useState<AttributeInputs>(() => inputsFromAttributes(fields, undefined));
  const [attrErrors, setAttrErrors] = useState<Readonly<Record<string, string>>>({});
  /** The masked address the detail answered (edit). */
  const [emailMasked, setEmailMasked] = useState("");
  /** Adding: the box is the address. Editing: `false` until "Nhập email mới" — the mask is never sent back. */
  const [emailEdited, setEmailEdited] = useState(!editing);
  const [emailError, setEmailError] = useState<string | null>(null);

  const contactId = mode.kind === "edit" ? mode.contact.id : null;
  const masked = mode.kind === "edit" ? mode.contact.phone_masked : "";

  useEffect(() => {
    if (contactId === null) return;
    let dropped = false;
    getCitizenContact(contactId).then((kq) => {
      if (dropped) return;
      if (!kq.ok) {
        setDetailError(kq.thongBao);
        return;
      }
      const d = kq.duLieu;
      setDraft({
        fullName: d.full_name,
        // The masked number is NOT put in the field: the field holds what will be SENT, and a mask is
        // refused. The mask is drawn beside the button until the reveal fills the field.
        phone: "",
        email: "",
        residentialUnitId: d.residential_unit_id,
        address: d.address,
        tagText: d.tags.join(", "),
        groupIds: d.group_ids,
        note: d.note,
        isActive: d.is_active,
      });
      setStored(d.attributes);
      setAttrInputs(inputsFromAttributes(fields, d.attributes));
      setEmailMasked(contactEmailMasked(d));
      setReady(true);
    });
    return () => {
      dropped = true;
    };
    // `fields` is read once, when the detail arrives: re-reading the detail because the definitions were
    // re-read would throw away what the officer is typing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contactId]);

  function reveal() {
    if (contactId === null || phone.kind !== "masked") return;
    setPhone({ kind: "loading" });
    revealCitizenContactPhone(contactId).then((r) => {
      if (r.ok) {
        setDraft((d) => ({ ...d, phone: r.duLieu.phone }));
        setPhone({ kind: "shown" });
      } else {
        // 403 and every other refusal alike: the mask stays, the chapter's sentence (§5, step 4).
        setPhone({ kind: "masked", error: REVEAL_PHONE_FAILED });
      }
    });
  }

  const submittable = ready && !saving && canSubmitContact(draft, editing, phone.kind === "shown");

  async function submit() {
    if (!submittable) return;
    const emailWrong = emailEdited ? emailProblem(draft.email) : null;
    const attrs = attributesFromInputs(fields, attrInputs, stored);
    setEmailError(emailWrong);
    setAttrErrors(attrs.ok ? {} : attrs.errors);
    if (emailWrong !== null || !attrs.ok) return;
    const body: CitizenContactBody = {
      full_name: draft.fullName.trim(),
      phone: draft.phone.trim(),
      residential_unit_id: draft.residentialUnitId,
      address: draft.address.trim(),
      note: draft.note.trim(),
      tags: parseTags(draft.tagText),
      group_ids: [...draft.groupIds],
      is_active: draft.isActive,
      attributes: attrs.attributes,
    };
    if (emailEdited) body.email = draft.email.trim();
    setSaving(true);
    const kq = mode.kind === "add" ? await createCitizenContact(body, mode.idempotencyKey) : await updateCitizenContact(mode.contact.id, body);
    setSaving(false);
    if (!kq.ok) {
      toast.error(saveFailureToast(kq.code));
      return;
    }
    toast.success(editing ? SAVED_TOAST : ADDED_TOAST);
    onSaved();
    onClose();
  }

  const pickable = pickableResidentialUnits(units);
  // A stored unit that is no longer pickable (retired) stays selected, so saving a tag never unlinks it.
  const unitMissing = draft.residentialUnitId !== "" && !pickable.some((u) => u.id === draft.residentialUnitId);
  const storedUnit = units.find((u) => u.id === draft.residentialUnitId);

  return (
    <ModalDialog
      titleId={CONTACT_DIALOG_TITLE_ID}
      className="max-w-[42rem]"
      closeDisabled={saving}
      onDismiss={() => !saving && onClose()}
      initialFocusId="nguoi-dan-ho-ten"
    >
      <ModalDialogHeader
        titleId={CONTACT_DIALOG_TITLE_ID}
        title={editing ? CONTACT_EDIT_TITLE : CONTACT_ADD_TITLE}
        description={CONTACT_DIALOG_DESCRIPTION}
      />

      {detailError !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {detailError}
        </p>
      )}
      {!ready && detailError === "" && (
        <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]" role="status">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          {DETAIL_LOADING}
        </p>
      )}

      <form
        className="m-0 flex min-h-0 flex-col gap-4 overflow-y-auto"
        aria-label={editing ? CONTACT_EDIT_TITLE : CONTACT_ADD_TITLE}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <fieldset disabled={!ready} className="m-0 grid min-w-0 gap-4 border-0 p-0 sm:grid-cols-2">
          <Field label={FIELD_FULL_NAME} htmlFor="nguoi-dan-ho-ten" required grow="auto">
            <input id="nguoi-dan-ho-ten" value={draft.fullName} autoComplete="off" onChange={(e) => setDraft({ ...draft, fullName: e.target.value })} />
          </Field>

          <Field
            label={FIELD_PHONE}
            htmlFor="nguoi-dan-so-dien-thoai"
            required
            grow="auto"
            hint={editing && phone.kind !== "shown" ? REVEAL_BEFORE_SAVE_HINT : undefined}
            error={phone.kind === "masked" && phone.error !== null ? phone.error : undefined}
          >
            {editing && phone.kind !== "shown" ? (
              // The mask, read-only, and the ONE button that asks the server for the digits (D2).
              <div className="flex items-center gap-2">
                <input id="nguoi-dan-so-dien-thoai" value={masked} readOnly aria-readonly="true" inputMode="tel" className="tabular-nums" />
                {canReveal && (
                  <Button
                    id={REVEAL_PHONE_BUTTON_ID}
                    type="button"
                    variant="secondary"
                    size="md"
                    className="shrink-0"
                    disabled={phone.kind === "loading"}
                    aria-busy={phone.kind === "loading" || undefined}
                    onClick={reveal}
                  >
                    {REVEAL_PHONE_BUTTON}
                  </Button>
                )}
              </div>
            ) : (
              <input
                id="nguoi-dan-so-dien-thoai"
                value={draft.phone}
                inputMode="tel"
                autoComplete="off"
                className="tabular-nums"
                onChange={(e) => setDraft({ ...draft, phone: e.target.value })}
              />
            )}
          </Field>

          <Field
            label={FIELD_EMAIL}
            htmlFor="nguoi-dan-email"
            grow="auto"
            error={emailError ?? undefined}
            hint={editing && emailEdited && emailMasked !== "" ? EMAIL_REPLACE_HINT : undefined}
            className="sm:col-span-2"
          >
            {emailEdited ? (
              <div className="flex items-center gap-2">
                <input
                  id="nguoi-dan-email"
                  type="email"
                  value={draft.email}
                  autoComplete="off"
                  spellCheck={false}
                  aria-invalid={emailError !== null}
                  onChange={(e) => setDraft({ ...draft, email: e.target.value })}
                />
                {editing && (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    className="shrink-0"
                    onClick={() => {
                      setEmailEdited(false);
                      setEmailError(null);
                      setDraft({ ...draft, email: "" });
                    }}
                  >
                    {KEEP_EMAIL_BUTTON}
                  </Button>
                )}
              </div>
            ) : (
              // The stored address, masked and read-only; replacing it is one explicit press.
              <div className="flex items-center gap-2">
                <input id="nguoi-dan-email" value={emailMasked} readOnly aria-readonly="true" placeholder="—" />
                <Button type="button" variant="secondary" size="md" className="shrink-0" onClick={() => setEmailEdited(true)}>
                  {EDIT_EMAIL_BUTTON}
                </Button>
              </div>
            )}
          </Field>

          <Field label={FIELD_UNIT} htmlFor="nguoi-dan-thon" kind="select" grow="auto">
            <select
              id="nguoi-dan-thon"
              className={SELECT_CLASS}
              value={draft.residentialUnitId}
              onChange={(e) => setDraft({ ...draft, residentialUnitId: e.target.value })}
            >
              <option value="">{UNIT_UNKNOWN}</option>
              {pickable.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name}
                </option>
              ))}
              {unitMissing && <option value={draft.residentialUnitId}>{storedUnit?.name ?? draft.residentialUnitId}</option>}
            </select>
          </Field>

          {/* D6: the one way to switch a row off. Bottom-aligned with the unit select beside it. */}
          <div className="flex flex-col justify-end">
            <label htmlFor="nguoi-dan-dang-lien-he" className="flex h-8 items-center gap-2 text-sm text-navy">
              <input
                id="nguoi-dan-dang-lien-he"
                type="checkbox"
                className="accent-brand size-4"
                checked={draft.isActive}
                onChange={(e) => setDraft({ ...draft, isActive: e.target.checked })}
              />
              {FIELD_ACTIVE}
            </label>
          </div>

          <Field label={FIELD_ADDRESS} htmlFor="nguoi-dan-dia-chi" grow="auto" className="sm:col-span-2">
            <input id="nguoi-dan-dia-chi" value={draft.address} autoComplete="off" onChange={(e) => setDraft({ ...draft, address: e.target.value })} />
          </Field>

          <Field label={FIELD_TAGS} htmlFor="nguoi-dan-nhan" grow="auto" className="sm:col-span-2" hint={TAGS_HINT}>
            <input id="nguoi-dan-nhan" value={draft.tagText} autoComplete="off" onChange={(e) => setDraft({ ...draft, tagText: e.target.value })} />
          </Field>
          {/* D4: fixed, not a hint that an error replaces — its own line, always drawn. */}
          <p className="text-danger m-0 -mt-2 text-[12px] font-medium sm:col-span-2" data-tags-warning="">
            {TAGS_WARNING}
          </p>

          <div className="flex flex-col gap-1.5 sm:col-span-2">
            <p className="m-0 block text-[13px] leading-tight font-semibold text-navy">{FIELD_GROUPS}</p>
            <div className="flex flex-wrap gap-1.5" role="group" aria-label={FIELD_GROUPS}>
              {groups.length === 0 && <p className="text-ink-muted m-0 text-[12.5px]">{NO_GROUPS_IN_DIALOG}</p>}
              {groups.map((g) => {
                const on = draft.groupIds.includes(g.id);
                return (
                  <button
                    key={g.id}
                    type="button"
                    aria-pressed={on}
                    onClick={() => setDraft({ ...draft, groupIds: toggleGroup(draft.groupIds, g.id) })}
                    className={cn(
                      "cursor-pointer rounded-full border border-solid px-3 py-1.5 [font-family:inherit] text-[12px] font-semibold",
                      on ? "border-brand/30 bg-brand/10 text-brand" : "border-line text-ink-muted bg-white",
                    )}
                  >
                    {g.name}
                  </button>
                );
              })}
            </div>
          </div>

          <Field label={FIELD_NOTE} htmlFor="nguoi-dan-ghi-chu" grow="auto" className="sm:col-span-2">
            <textarea id="nguoi-dan-ghi-chu" rows={2} value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} />
          </Field>

          <CitizenAttributeInputs
            fields={fields}
            inputs={attrInputs}
            stored={stored}
            errors={attrErrors}
            onChange={(code, value) => {
              setAttrInputs((cur) => ({ ...cur, [code]: value }));
              if (attrErrors[code] !== undefined) setAttrErrors(({ [code]: _gone, ...rest }) => rest);
            }}
          />
        </fieldset>

        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            {CANCEL_BUTTON}
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={!submittable}
            aria-busy={saving || undefined}
            icon={saving ? <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" /> : undefined}
          >
            {editing ? SUBMIT_EDIT : SUBMIT_ADD}
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
