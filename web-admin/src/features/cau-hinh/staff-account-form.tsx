"use client";

import { Loader2 } from "lucide-react";

import type { DangMoGhi, MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import {
  ACCOUNT_EDIT_TITLE,
  ACCOUNT_EMAIL_LABEL,
  ACCOUNT_NO_UNIT_OPTION,
  CANH_BAO_DOI_DI_DONG_CONG_KHAI,
  EMAIL_PLACEHOLDER,
  MO_TA_CO_ZALO,
  NAME_PLACEHOLDER,
  NUT_HUY,
  NUT_LUU,
  O_BO_PHAN,
  O_CHUC_DANH,
  O_CO_ZALO,
  O_HO_TEN,
  PHONE_PLACEHOLDER,
  POSITION_PLACEHOLDER,
  coCanhBaoDoiDiDong,
} from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { selectCls } from "./config-ui";
import {
  ADD_ACCOUNT_BUTTON,
  ADD_ACCOUNT_TITLE,
  EMAIL_LOCKED_HINT,
  MISSING_ROLE_OPTION,
  MOBILE_LABEL,
  NO_ROLE_OPTION,
  NO_ROLES_YET,
  OFFICE_PHONE_LABEL,
  ONE_ROLE_HINT,
  OWN_ROLE_HINT,
  ROLE_LABEL,
  TEMP_PASSWORD_NOTE,
} from "./nhan-can-bo";
import { DIALOG_FOOTER_CLASS, DIALOG_LABEL_CLASS } from "./org-unit-dialog-classes";

/**
 * The `/nguoi-dung` account form — the prototype's `UserFormDialog` (`vigov-require/apps/admin/src/
 * components/admin/UserFormDialog.tsx`) as the user decided it on 09/10/2026. A NEW component, not a
 * third layout of `BieuMauGhiCanBo`: that file is shared with `/danh-ba` and `/mini-app`.
 *
 * WHAT DIFFERS FROM THE PROTOTYPE, AND WHY (user 09/10/2026):
 *
 *   · NO PASSWORD FIELD, on add or on edit. The password stays the one-time value the server generates
 *     (open question #9): adding with an email chains `POST /staff/{id}/account` and shows that value
 *     once; a forgotten password is `Đặt lại mật khẩu` in the row's `⋯` menu. On ADD, the prototype's
 *     password cell holds `TEMP_PASSWORD_NOTE` instead (bug sheet row 41).
 *   · ONE ROLE, a radio group laid out like the prototype's checkbox grid, with `ONE_ROLE_HINT`. The
 *     server holds one role per person (`nguoi_dung.vai_tro_id`). On EDIT the choice is saved through
 *     `PUT /staff/{id}/role` AFTER the profile PATCH — the route that carries #13/#14 — never as a field
 *     of the PATCH. On ADD (owner 10/10/2026, bug sheet row 41) it rides the create body as
 *     `role_ids: [id]`, checked server-side against the same #14 escalation rule in the same transaction.
 *   · TWO PHONE FIELDS (#16), each label saying which kind: the mobile beside Chức danh, the office
 *     line under Bộ phận. The MOBILE is grouped as typed (`xxx xxx xxxx`) and capped at ten digits; the
 *     draft keeps digits only, so nothing with a space is ever sent. The office line is not: a landline
 *     has eleven digits and its own grouping.
 *   · `Có Zalo` on edit only: the add body (`identity_themCanBoVao`) has no such field.
 *
 * PURELY PRESENTATIONAL: every value in through props, every change out through callbacks, every
 * network call at the caller (`danh-ba-can-bo.tsx`). So the branches nobody sees while developing —
 * "server refused", "sending" — render with `react-dom/server`.
 *
 * NO FORMAT CHECK HERE (email shape, phone characters, lengths): the server normalises and refuses each
 * with a Vietnamese sentence (`domain/danh_ba_ghi.go`); a copy here would drift (rule 9, forbidden #2).
 * The mobile's ten-digit cap is input shaping, not a check: the server still refuses eleven
 * (`ErrMobileTooManyDigits`), and its sentence — like `role_not_found` and `permission_escalation` —
 * reaches `serverError` verbatim, prefix stripped (`goi.ts`, `stripTechnicalPrefix`).
 */
/** Ten digits, the length of a Vietnamese mobile once normalised (`domain.ErrMobileTooManyDigits`). */
export const MOBILE_MAX_DIGITS = 10;

/** What the mobile box SHOWS for a draft value: digits grouped `xxx xxx xxxx`. */
export function formatMobile(value: string): string {
  // A stored value with other characters (a `+84…` typed before this form) is shown as it is, so
  // opening the edit form never rewrites a number nobody touched.
  if (!/^\d*$/.test(value)) return value;
  return [value.slice(0, 3), value.slice(3, 6), value.slice(6)].filter((g) => g !== "").join(" ");
}

/** What the DRAFT keeps for a typed mobile: digits only, at most `MOBILE_MAX_DIGITS`. */
export function mobileDigits(typed: string): string {
  return typed.replace(/\D/g, "").slice(0, MOBILE_MAX_DIGITS);
}

export type RoleChoice =
  /** The role radios are drawn and can be changed. */
  | "editable"
  /** Drawn, disabled, with `OWN_ROLE_HINT` — the signed-in officer's own row (#14). */
  | "own";

export function StaffAccountForm({
  open,
  draft,
  setDraft,
  roleId,
  setRoleId,
  roleChoice,
  units,
  roles,
  serverError,
  busy,
  onSubmit,
  onCancel,
}: {
  /** `them` or `sua` — the two profile forms. */
  open: Extract<DangMoGhi, { kieu: "them" } | { kieu: "sua" }>;
  draft: BanNhapCanBo;
  setDraft: (d: BanNhapCanBo) => void;
  /** The role chosen; `""` = no role. Add: sent as `role_ids`; edit: through `PUT .../role`. */
  roleId: string;
  setRoleId: (id: string) => void;
  roleChoice: RoleChoice;
  units: readonly MucChon[];
  roles: readonly MucChon[];
  serverError: string;
  busy: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const editing = open.kieu === "sua";
  const staff: identity_canBoTomTat | null = editing ? open.canBo : null;
  // THE EMAIL IS THE LOGIN NAME once an account exists: locked here, as the prototype does. The server
  // is the real guard (409 `staff_email_is_login` on clearing it). Without an account it stays editable,
  // so an email can be added now and the account issued later.
  const emailLocked = staff !== null && staff.has_account;
  const unitMissing = draft.boPhanID !== "" && !units.some((m) => m.id === draft.boPhanID);
  const roleMissing = roleId !== "" && !roles.some((m) => m.id === roleId);

  return (
    <form
      className="m-0 flex min-h-0 min-w-0 flex-col gap-4"
      aria-label={editing ? ACCOUNT_EDIT_TITLE : ADD_ACCOUNT_TITLE}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
        <div className="grid grid-cols-2 gap-4">
          <div className="col-span-2 block">
            <label htmlFor="o-ho-ten-can-bo" className={DIALOG_LABEL_CLASS}>
              {O_HO_TEN}
            </label>
            <input
              id="o-ho-ten-can-bo"
              name="o-ho-ten-can-bo"
              value={draft.hoTen}
              required
              placeholder={NAME_PLACEHOLDER}
              onChange={(e) => setDraft({ ...draft, hoTen: e.target.value })}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>

          {/* The prototype's row is (Thư điện tử, Mật khẩu); with no password field the email keeps its
              half of the row, so every field sits where the prototype draws it. On add the other half
              says who issues the password (open question #9). */}
          <div className="block min-w-0">
            <label htmlFor="o-email-can-bo" className={DIALOG_LABEL_CLASS}>
              {ACCOUNT_EMAIL_LABEL}
            </label>
            <input
              id="o-email-can-bo"
              name="o-email-can-bo"
              type="email"
              value={draft.email}
              disabled={emailLocked}
              aria-describedby={emailLocked ? "o-email-can-bo-mo-ta" : undefined}
              placeholder={EMAIL_PLACEHOLDER}
              onChange={(e) => setDraft({ ...draft, email: e.target.value })}
              className={cn(controlClass, "mt-1.5")}
            />
            {emailLocked && (
              <p id="o-email-can-bo-mo-ta" className="text-ink-muted m-0 mt-1 text-[11.5px]">
                {EMAIL_LOCKED_HINT}
              </p>
            )}
          </div>
          {editing ? (
            <div aria-hidden="true" />
          ) : (
            <div className="block min-w-0">
              <span className={DIALOG_LABEL_CLASS} aria-hidden="true">
                &nbsp;
              </span>
              <p className="text-ink-muted m-0 mt-1.5 text-[12px] leading-snug">
                {TEMP_PASSWORD_NOTE}
              </p>
            </div>
          )}

          <div className="block min-w-0">
            <label htmlFor="o-chuc-danh-can-bo" className={DIALOG_LABEL_CLASS}>
              {O_CHUC_DANH}
            </label>
            <input
              id="o-chuc-danh-can-bo"
              name="o-chuc-danh-can-bo"
              value={draft.chucDanh}
              placeholder={POSITION_PLACEHOLDER}
              onChange={(e) => setDraft({ ...draft, chucDanh: e.target.value })}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>
          <div className="block min-w-0">
            <label htmlFor="o-di-dong-can-bo" className={DIALOG_LABEL_CLASS}>
              {MOBILE_LABEL}
            </label>
            <input
              id="o-di-dong-can-bo"
              name="o-di-dong-can-bo"
              type="tel"
              inputMode="numeric"
              autoComplete="off"
              value={formatMobile(draft.diDongCaNhan)}
              placeholder={PHONE_PLACEHOLDER}
              onChange={(e) => setDraft({ ...draft, diDongCaNhan: mobileDigits(e.target.value) })}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>

          <div className="col-span-2 block">
            <label htmlFor="o-bo-phan-can-bo" className={DIALOG_LABEL_CLASS}>
              {O_BO_PHAN}
            </label>
            <select
              id="o-bo-phan-can-bo"
              name="o-bo-phan-can-bo"
              value={draft.boPhanID}
              onChange={(e) => setDraft({ ...draft, boPhanID: e.target.value })}
              className={cn(selectCls, "mt-1.5 h-9 w-full min-w-0 pr-8 text-[13px]")}
            >
              <option value="">{ACCOUNT_NO_UNIT_OPTION}</option>
              {units.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
              {/* A stored unit the catalogue no longer lists stays selected, so saving a phone number
                  never silently unlinks it. */}
              {unitMissing && <option value={draft.boPhanID}>{MISSING_ROLE_OPTION}</option>}
            </select>
          </div>

          {/* The office line under Bộ phận (user 09/10/2026): duty information, kept apart from the
              personal mobile above (#16). */}
          <div className="block min-w-0">
            <label htmlFor="o-may-ban-can-bo" className={DIALOG_LABEL_CLASS}>
              {OFFICE_PHONE_LABEL}
            </label>
            <input
              id="o-may-ban-can-bo"
              name="o-may-ban-can-bo"
              value={draft.mayBanCoQuan}
              placeholder={PHONE_PLACEHOLDER}
              onChange={(e) => setDraft({ ...draft, mayBanCoQuan: e.target.value })}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>
          <div aria-hidden="true" />

          <RoleRadios
            roleId={roleId}
            setRoleId={setRoleId}
            roles={roles}
            roleMissing={roleMissing}
            disabled={roleChoice === "own" || busy}
            own={roleChoice === "own"}
          />

          {editing && (
            <div className="col-span-2 block">
              <label htmlFor="o-co-zalo-can-bo" className="m-0 flex cursor-pointer items-center gap-2 text-[12.5px]">
                <input
                  id="o-co-zalo-can-bo"
                  name="o-co-zalo-can-bo"
                  type="checkbox"
                  checked={draft.coZalo}
                  aria-describedby="o-co-zalo-can-bo-mo-ta"
                  onChange={(e) => setDraft({ ...draft, coZalo: e.target.checked })}
                  className="accent-brand m-0 size-3.5"
                />
                <span className="text-navy">{O_CO_ZALO}</span>
              </label>
              <p id="o-co-zalo-can-bo-mo-ta" className="text-ink-muted m-0 mt-1 text-[11.5px]">
                {MO_TA_CO_ZALO}
              </p>
            </div>
          )}
        </div>

        {coCanhBaoDoiDiDong(open, draft) && (
          <p className="canh-bao-pham-vi m-0" role="status">
            {CANH_BAO_DOI_DI_DONG_CONG_KHAI}
          </p>
        )}

        {/* The server's sentence, verbatim, beside the button it refused (#13, #14). */}
        {serverError !== "" && (
          <p role="alert" className="text-danger m-0 text-[12px] font-medium">
            {serverError}
          </p>
        )}
      </div>

      <div className={DIALOG_FOOTER_CLASS}>
        <Button type="button" variant="outline" onClick={onCancel} disabled={busy}>
          {NUT_HUY}
        </Button>
        <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
          {busy && <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />}
          {editing ? NUT_LUU : ADD_ACCOUNT_BUTTON}
        </Button>
      </div>
    </form>
  );
}

/**
 * The role choice: native radios in the prototype's bordered two-column grid. `""` (no role) is the
 * first choice because it is a real destination of `PUT .../role`; a stored role the catalogue no
 * longer lists stays checked under its own label, so saving a phone number never moves anybody.
 *
 * `border-solid` because preflight is off here: a bare `border` draws nothing.
 */
function RoleRadios({
  roleId,
  setRoleId,
  roles,
  roleMissing,
  disabled,
  own,
}: {
  roleId: string;
  setRoleId: (id: string) => void;
  roles: readonly MucChon[];
  roleMissing: boolean;
  disabled: boolean;
  own: boolean;
}) {
  const choices: readonly MucChon[] = [
    { id: "", name: NO_ROLE_OPTION },
    ...roles,
    ...(roleMissing ? [{ id: roleId, name: MISSING_ROLE_OPTION }] : []),
  ];
  return (
    <div className="col-span-2 block">
      <span id="o-vai-tro-can-bo-nhan" className={DIALOG_LABEL_CLASS}>
        {ROLE_LABEL}
      </span>
      <div
        role="radiogroup"
        aria-labelledby="o-vai-tro-can-bo-nhan"
        aria-describedby="o-vai-tro-can-bo-mo-ta"
        className="border-line mt-1.5 grid grid-cols-2 gap-2 rounded-[10px] border border-solid p-3"
      >
        {roles.length === 0 && !roleMissing && (
          <p className="text-ink-muted col-span-2 m-0 text-[12.5px]">{NO_ROLES_YET}</p>
        )}
        {choices.map((r) => (
          <label
            key={r.id === "" ? "-" : r.id}
            className={cn(
              "m-0 flex items-center gap-2 text-[12.5px]",
              disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer",
            )}
          >
            <input
              type="radio"
              name="o-vai-tro-can-bo"
              value={r.id}
              checked={roleId === r.id}
              disabled={disabled}
              onChange={() => setRoleId(r.id)}
              className="accent-brand m-0 size-3.5"
            />
            <span className="text-navy">{r.name}</span>
          </label>
        ))}
      </div>
      <p id="o-vai-tro-can-bo-mo-ta" className="text-ink-muted m-0 mt-1 text-[11.5px]">
        {own ? OWN_ROLE_HINT : ONE_ROLE_HINT}
      </p>
    </div>
  );
}
