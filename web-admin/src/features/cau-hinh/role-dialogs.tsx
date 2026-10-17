"use client";

import { useId, useState } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { seedRoleTemplates } from "@/lib/api/role-templates";
import { createRole, deleteRole, updateRole } from "@/lib/api/roles";
import type { identity_seedRoleTemplatesOut, identity_vaiTroCotRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type
import { cn } from "@/lib/cn";

import { DIALOG_FOOTER_CLASS, DIALOG_LABEL_CLASS } from "./org-unit-dialog-classes";
import {
  ADD_ROLE_BUTTON,
  ADD_ROLE_HINT,
  ADD_TEMPLATE_ROLES_BUTTON,
  BUSY_ADDING,
  CANCEL,
  DELETE_REASON_LABEL,
  DELETE_ROLE,
  DELETE_ROLE_EXPLANATION,
  LEADER_HINT,
  LEADER_LABEL,
  RENAME_ROLE_HINT,
  RENAME_ROLE_TITLE,
  ROLE_NAME_LABEL,
  ROLE_NAME_PLACEHOLDER,
  ROLE_TEMPLATE_NAMES,
  SAVE,
  SEED_INTRO,
  SEED_KEEPS,
  SEED_TITLE,
  deleteReasonProblem,
  deleteRoleTitle,
  isNameRefusal,
  isReasonRefusal,
  roleNameProblem,
} from "./role-catalogue";

/**
 * The three dialogs that manage the commune's role list from the Phân quyền screen (owner decision
 * 15/10/2026). Same box, footer and label as the org chart's dialogs (`tab-so-do-to-chuc.tsx`
 * `BieuMauBoPhan`, `org-unit-delete-form.tsx`) — no new look.
 *
 * EACH DIALOG SENDS ITS OWN REQUEST and reports success to the screen, which reloads the matrix. A
 * refusal stays inside the dialog: under the field it is about when the server's `code` says so, at
 * the foot of the form otherwise — the server's sentence verbatim either way. Esc and ✕ are refused
 * while a request is in flight, so the form never leaves the screen with its answer still coming.
 */

const ERROR_CLASS = "text-danger m-0 mt-1 text-[12px] font-medium";
const FORM_CLASS = "m-0 flex min-h-0 min-w-0 flex-col gap-4";

/** Confirm, then `POST /api/v1/roles/defaults`. The answer goes back so the screen can count it. */
export function SeedRolesDialog({
  onDone,
  onCancel,
}: {
  onDone: (out: identity_seedRoleTemplatesOut) => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState("");

  async function submit() {
    if (sending) return;
    setSending(true);
    setRefusal("");
    const r = await seedRoleTemplates();
    setSending(false);
    if (r.ok) onDone(r.duLieu);
    else setRefusal(r.thongBao);
  }

  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!sending) onCancel();
      }}
      closeDisabled={sending}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={SEED_TITLE} description={SEED_INTRO} />
      <form
        className={FORM_CLASS}
        aria-label={SEED_TITLE}
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="min-h-0 space-y-3 overflow-y-auto -m-1 p-1">
          <ul className="text-navy m-0 list-disc space-y-0.5 pl-5 text-[13px]">
            {ROLE_TEMPLATE_NAMES.map((n) => (
              <li key={n}>{n}</li>
            ))}
          </ul>
          <p className="text-ink-muted m-0 text-[12.5px]">{SEED_KEEPS}</p>
          {refusal !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {refusal}
            </p>
          )}
        </div>
        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            {CANCEL}
          </Button>
          <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={ADD_TEMPLATE_ROLES_BUTTON} busyText={BUSY_ADDING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/**
 * Add a role (`role` absent) or rename one (`role` given, fields prefilled). Same form for both, as the
 * org chart's add/edit: the two differ in their title, their hint and the request sent.
 */
export function RoleFormDialog({
  role,
  onSaved,
  onCancel,
}: {
  role?: identity_vaiTroCotRa;
  onSaved: (name: string) => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const nameId = useId();
  const nameErrorId = useId();
  const leaderHintId = useId();
  const [name, setName] = useState(role?.name ?? "");
  const [isLeader, setIsLeader] = useState(role?.is_leader ?? false);
  const [sending, setSending] = useState(false);
  const [nameError, setNameError] = useState("");
  const [formError, setFormError] = useState("");

  const editing = role !== undefined;
  const title = editing ? RENAME_ROLE_TITLE : ADD_ROLE_BUTTON;

  async function submit() {
    if (sending) return;
    const problem = roleNameProblem(name);
    setNameError(problem);
    setFormError("");
    if (problem !== "") return;
    setSending(true);
    const body = { name: name.trim(), is_leader: isLeader };
    const r = editing ? await updateRole(role.id, body) : await createRole(body);
    setSending(false);
    if (r.ok) {
      onSaved(r.duLieu.name);
      return;
    }
    if (isNameRefusal(r.code)) setNameError(r.thongBao);
    else setFormError(r.thongBao);
  }

  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!sending) onCancel();
      }}
      closeDisabled={sending}
      initialFocusId={nameId}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={title} description={editing ? RENAME_ROLE_HINT : ADD_ROLE_HINT} />
      <form
        className={FORM_CLASS}
        aria-label={title}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto -m-1 p-1">
          <div className="block">
            <label htmlFor={nameId} className={DIALOG_LABEL_CLASS}>
              {ROLE_NAME_LABEL}
              <span className="text-danger">*</span>
            </label>
            <input
              id={nameId}
              name="name"
              value={name}
              required
              placeholder={ROLE_NAME_PLACEHOLDER}
              disabled={sending}
              aria-invalid={nameError !== ""}
              aria-describedby={nameError !== "" ? nameErrorId : undefined}
              onChange={(e) => setName(e.target.value)}
              className={cn(controlClass, "mt-1.5")}
            />
            {nameError !== "" && (
              <p id={nameErrorId} role="alert" className={ERROR_CLASS}>
                {nameError}
              </p>
            )}
          </div>

          <div className="block">
            <label className="m-0 flex cursor-pointer items-center gap-2 text-[12.5px]">
              <input
                type="checkbox"
                name="is_leader"
                checked={isLeader}
                disabled={sending}
                aria-describedby={leaderHintId}
                onChange={(e) => setIsLeader(e.target.checked)}
                className="accent-brand m-0 size-3.5"
              />
              <span className="text-navy">{LEADER_LABEL}</span>
            </label>
            <p id={leaderHintId} className="text-ink-muted m-0 mt-1 text-[11.5px]">
              {LEADER_HINT}
            </p>
          </div>

          {formError !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {formError}
            </p>
          )}
        </div>

        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            {CANCEL}
          </Button>
          <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={editing ? SAVE : ADD_ROLE_BUTTON} busyText={editing ? BUSY_SAVING : BUSY_ADDING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/**
 * Delete a role, with a mandatory reason (rule 7: a soft delete carries why). The 409 `role_in_use`
 * sentence — how many staff still hold the role — is the server's, shown as it wrote it.
 */
export function DeleteRoleDialog({
  role,
  onDeleted,
  onCancel,
}: {
  role: identity_vaiTroCotRa;
  onDeleted: () => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const reasonId = useId();
  const reasonErrorId = useId();
  const [reason, setReason] = useState("");
  const [sending, setSending] = useState(false);
  const [reasonError, setReasonError] = useState("");
  const [formError, setFormError] = useState("");
  const title = deleteRoleTitle(role.name);

  async function submit() {
    if (sending) return;
    const problem = deleteReasonProblem(reason);
    setReasonError(problem);
    setFormError("");
    if (problem !== "") return;
    setSending(true);
    const r = await deleteRole(role.id, reason.trim());
    setSending(false);
    if (r.ok) {
      onDeleted();
      return;
    }
    if (isReasonRefusal(r.code)) setReasonError(r.thongBao);
    else setFormError(r.thongBao);
  }

  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!sending) onCancel();
      }}
      closeDisabled={sending}
      initialFocusId={reasonId}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={title} description={DELETE_ROLE_EXPLANATION} />
      <form
        className={FORM_CLASS}
        aria-label={title}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto -m-1 p-1">
          <div className="block">
            <label htmlFor={reasonId} className={DIALOG_LABEL_CLASS}>
              {DELETE_REASON_LABEL}
              <span className="text-danger">*</span>
            </label>
            <textarea
              id={reasonId}
              name="reason"
              rows={3}
              value={reason}
              required
              disabled={sending}
              aria-invalid={reasonError !== ""}
              aria-describedby={reasonError !== "" ? reasonErrorId : undefined}
              onChange={(e) => setReason(e.target.value)}
              className={cn(controlClass, "mt-1.5 h-auto py-1.5")}
            />
            {reasonError !== "" && (
              <p id={reasonErrorId} role="alert" className={ERROR_CLASS}>
                {reasonError}
              </p>
            )}
          </div>
          {formError !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {formError}
            </p>
          )}
        </div>
        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            {CANCEL}
          </Button>
          <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={DELETE_ROLE} busyText={BUSY_DELETING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
