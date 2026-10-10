import {
  CANH_BAO_DOI_DI_DONG_CONG_KHAI,
  NUT_HUY,
  O_DI_DONG,
  coCanhBaoDoiDiDong,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BUSY_SAVING, BusyLabel } from "./busy-label";
import {
  CHECK_MINI_APP,
  CHECK_ZALO,
  CONTACT_ADD_TITLE,
  CONTACT_EDIT_TITLE,
  EMAIL_NOTE,
  FIELD_EMAIL,
  FIELD_FULL_NAME,
  FIELD_POSITION,
  FIELD_UNIT,
  SAVE_BUTTON,
  UNIT_NOT_IN_CATALOGUE,
  UNIT_PLACEHOLDER,
} from "./staff-contact";
import type { ContactErrors } from "./staff-contact";

const SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 w-full rounded-md border border-solid bg-white pr-9 pl-3 text-[12.5px] outline-none focus-visible:ring-[3px]";

/**
 * The body of the add / edit dialog — the prototype's `StaffContactForm` (`space-y-4`: name, unit,
 * position, the numbers and the e-mail, two boxes and a note, then Huỷ · Lưu).
 *
 * PRESENTATION ONLY, NO HOOK: values in through props, changes out through callbacks, the network at
 * the call site (`danh-ba-lien-he.tsx`). So the screen's flow test reads exactly what the screen hands
 * it, and the dialog frame around it (`ConfigDialog`) is the only part that needs React proper.
 *
 * ONE NUMBER FIELD, `Di động`, as the prototype. `Máy bàn cơ quan` is NOT DRAWN here (bug sheet row 26),
 * display only: the draft still carries the stored landline (`banTuCanBo`) and the save sends it back
 * unchanged, so nothing is lost; open question #16 keeps the field, and `/nguoi-dung`'s account form still
 * edits it. Never relabel the mobile box to hold both — the landline is duty information, the mobile is
 * Decree 13 personal data.
 *
 * "Gọi được qua Zalo" is offered when adding too: the create route has no such field, so the screen
 * writes it with a second call (`zaloAfterCreate`). "Hiện trên danh bạ Mini App" is drawn only for a
 * session holding `content.update`, and never publishes by itself (`publicationStep`).
 *
 * `noValidate`: the required fields are checked by `validateContact` and said UNDER each field, in
 * Vietnamese — the browser's own bubbles are in the browser's language and vanish.
 */
export function StaffContactForm({
  editing,
  draft,
  setDraft,
  showOnMiniApp,
  setShowOnMiniApp,
  canPublish,
  units,
  errors,
  serverError,
  sending,
  onSubmit,
  onCancel,
}: {
  /** The row being edited; `null` = adding. */
  editing: identity_canBoTomTat | null;
  draft: BanNhapCanBo;
  setDraft: (b: BanNhapCanBo) => void;
  showOnMiniApp: boolean;
  setShowOnMiniApp: (v: boolean) => void;
  /** Session holds `content.update` — draws the Mini App box. */
  canPublish: boolean;
  units: readonly MucChon[];
  errors: ContactErrors;
  serverError: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const unitMissing = draft.boPhanID !== "" && !units.some((u) => u.id === draft.boPhanID);
  return (
    <form
      className="m-0 space-y-4"
      aria-label={editing === null ? CONTACT_ADD_TITLE : CONTACT_EDIT_TITLE}
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <Field label={FIELD_FULL_NAME} htmlFor="staff-full-name" required error={errors.fullName} grow="auto">
        <input
          id="staff-full-name"
          name="staff-full-name"
          value={draft.hoTen}
          aria-invalid={errors.fullName !== undefined}
          onChange={(e) => setDraft({ ...draft, hoTen: e.target.value })}
        />
      </Field>

      <Field label={FIELD_UNIT} htmlFor="staff-unit" required error={errors.unit} grow="auto">
        <select
          id="staff-unit"
          name="staff-unit"
          value={draft.boPhanID}
          aria-invalid={errors.unit !== undefined}
          onChange={(e) => setDraft({ ...draft, boPhanID: e.target.value })}
          className={SELECT_CLASS}
        >
          <option value="">{UNIT_PLACEHOLDER}</option>
          {units.map((u) => (
            <option key={u.id} value={u.id}>
              {u.name}
            </option>
          ))}
          {/* A stored unit the catalogue no longer lists stays selected, so saving a phone number never
              silently unlinks it. */}
          {unitMissing && <option value={draft.boPhanID}>{UNIT_NOT_IN_CATALOGUE}</option>}
        </select>
      </Field>

      <Field label={FIELD_POSITION} htmlFor="staff-position" grow="auto">
        <input
          id="staff-position"
          name="staff-position"
          value={draft.chucDanh}
          onChange={(e) => setDraft({ ...draft, chucDanh: e.target.value })}
        />
      </Field>

      {/* The prototype's row: `Di động` · e-mail. */}
      <div className="grid grid-cols-2 gap-3">
        <Field label={O_DI_DONG} htmlFor="staff-mobile" grow="auto" className="min-w-0">
          <input
            id="staff-mobile"
            name="staff-mobile"
            inputMode="tel"
            value={draft.diDongCaNhan}
            onChange={(e) => setDraft({ ...draft, diDongCaNhan: e.target.value })}
          />
        </Field>
        <Field label={FIELD_EMAIL} htmlFor="staff-email" grow="auto" className="min-w-0">
          <input
            id="staff-email"
            name="staff-email"
            type="email"
            value={draft.email}
            onChange={(e) => setDraft({ ...draft, email: e.target.value })}
          />
        </Field>
      </div>

      {/* Decision 28/09/2026: changing the mobile of a published person takes them off the Mini App. */}
      {editing !== null && coCanhBaoDoiDiDong({ kieu: "sua", canBo: editing }, draft) && (
        <p className="canh-bao-pham-vi m-0" role="status">
          {CANH_BAO_DOI_DI_DONG_CONG_KHAI}
        </p>
      )}

      <div className="space-y-2">
        <label htmlFor="staff-has-zalo" className="m-0 flex cursor-pointer items-center gap-2.5 text-[12.5px]">
          <input
            id="staff-has-zalo"
            type="checkbox"
            checked={draft.coZalo}
            onChange={(e) => setDraft({ ...draft, coZalo: e.target.checked })}
            className="accent-brand m-0 size-3.5"
          />
          {CHECK_ZALO}
        </label>
        {canPublish && (
          <label htmlFor="staff-on-mini-app" className="m-0 flex cursor-pointer items-center gap-2.5 text-[12.5px]">
            <input
              id="staff-on-mini-app"
              type="checkbox"
              checked={showOnMiniApp}
              onChange={(e) => setShowOnMiniApp(e.target.checked)}
              className="accent-brand m-0 size-3.5"
            />
            {CHECK_MINI_APP}
          </label>
        )}
        <p className="text-ink-muted m-0 text-[11px] font-normal">{EMAIL_NOTE}</p>
      </div>

      {/* The server's sentence, verbatim (duplicate, bad e-mail, 403, 404…) — never rewritten here. */}
      {serverError !== "" && (
        <p role="alert" className="text-danger m-0 text-[12px] font-medium">
          {serverError}
        </p>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
          {NUT_HUY}
        </Button>
        <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label={SAVE_BUTTON} busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
  );
}
