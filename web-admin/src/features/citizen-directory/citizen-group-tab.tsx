"use client";

import { Plus, Trash2, Users } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { SkeletonRows } from "@/components/ui/skeleton";
import { createCitizenGroup, deleteCitizenGroup, newCitizenGroupBody, type CitizenGroup } from "@/lib/api/citizen-groups";

import {
  CANCEL_BUTTON,
  EMPTY_GROUPS_TEXT,
  EMPTY_GROUPS_TITLE,
  GROUPS_LOADING,
  GROUPS_TABLE_LABEL,
  GROUP_CREATED_TOAST,
  GROUP_CREATE_FAILED_TOAST,
  GROUP_REASON_LABEL,
  GROUP_REMOVED_TOAST,
  NEW_GROUP_BUTTON,
  NEW_GROUP_LABEL,
  NEW_GROUP_PLACEHOLDER,
  REMOVE_FAILED_TOAST,
  REMOVE_GROUP_BUTTON,
  REMOVE_GROUP_EXPLANATION,
  memberCountText,
  removeGroupLabel,
  removeGroupQuestion,
} from "./citizen-directory-words";

export const NEW_GROUP_INPUT_ID = "nhom-moi"; // vi-name-ok: DOM id, the prototype's own (`GroupManager.tsx:34`)
export const GROUP_REASON_INPUT_ID = "ly-do-go-nhom"; // vi-name-ok: DOM id, same convention

/**
 * The Nhóm tab — the prototype's `GroupManager` (chapter 16 §6): the "Lập nhóm mới" box (citizen.update
 * only), then the rows `name + description · n người · 🗑`. Removing a group asks for an OPTIONAL reason
 * (D7) in a confirm box above the table; the people stay in the directory. No edit control: the
 * prototype has none.
 *
 * ONE KEY PER CREATE ATTEMPT: minted when the tab mounts and after each success; a retry of a failed
 * send reuses it, so a lost 201 replays instead of answering `name_taken` for the group it already made.
 */
export function CitizenGroupTab({
  groups,
  canEdit,
  onChanged,
}: {
  /** `null` = not read yet (or refused): the skeleton. */
  groups: readonly CitizenGroup[] | null;
  canEdit: boolean;
  /** After a 201 / 204: the groups (and the list's group filter) must be read again. */
  onChanged: () => void;
}) {
  const [name, setName] = useState("");
  const [createKey, setCreateKey] = useState(() => crypto.randomUUID());
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState<CitizenGroup | null>(null);
  const [reason, setReason] = useState("");
  const [serverError, setServerError] = useState("");

  async function create() {
    const typed = name.trim();
    if (typed === "" || busy) return;
    setBusy(true);
    const kq = await createCitizenGroup(newCitizenGroupBody(typed), createKey);
    setBusy(false);
    if (!kq.ok) {
      toast.error(GROUP_CREATE_FAILED_TOAST);
      return;
    }
    setName("");
    setCreateKey(crypto.randomUUID());
    toast.success(GROUP_CREATED_TOAST);
    onChanged();
  }

  async function remove() {
    if (removing === null || busy) return;
    setBusy(true);
    const kq = await deleteCitizenGroup(removing.id, reason.trim());
    setBusy(false);
    if (!kq.ok) {
      setServerError(kq.thongBao);
      toast.error(REMOVE_FAILED_TOAST);
      return;
    }
    setRemoving(null);
    setReason("");
    setServerError("");
    toast.success(GROUP_REMOVED_TOAST);
    onChanged();
  }

  function openRemove(g: CitizenGroup) {
    setRemoving(g);
    setReason("");
    setServerError("");
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {canEdit && (
        <form
          className="border-line shadow-card rounded-card m-0 flex flex-wrap items-end gap-2 border border-solid bg-white p-4"
          aria-label={NEW_GROUP_LABEL}
          onSubmit={(e) => {
            e.preventDefault();
            void create();
          }}
        >
          <Field label={NEW_GROUP_LABEL} htmlFor={NEW_GROUP_INPUT_ID} grow="search" className="min-w-[16rem] flex-1">
            <input
              id={NEW_GROUP_INPUT_ID}
              value={name}
              placeholder={NEW_GROUP_PLACEHOLDER}
              autoComplete="off"
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Button type="submit" variant="primary" icon={<Plus aria-hidden="true" />} disabled={name.trim() === "" || busy} aria-busy={busy || undefined}>
            {NEW_GROUP_BUTTON}
          </Button>
        </form>
      )}

      {removing !== null && (
        <ConfirmDialog
          as="form"
          tone="danger"
          titleAs="h3"
          title={removeGroupQuestion(removing.name)}
          aria-label={removeGroupQuestion(removing.name)}
          onSubmit={(e) => {
            e.preventDefault();
            void remove();
          }}
          actions={
            <>
              <Button type="button" variant="secondary" onClick={() => setRemoving(null)} disabled={busy}>
                {CANCEL_BUTTON}
              </Button>
              <Button type="submit" variant="danger" icon={<Trash2 aria-hidden="true" />} disabled={busy} aria-busy={busy || undefined}>
                {REMOVE_GROUP_BUTTON}
              </Button>
            </>
          }
        >
          <p className="m-0">{REMOVE_GROUP_EXPLANATION}</p>
          <Field label={GROUP_REASON_LABEL} htmlFor={GROUP_REASON_INPUT_ID} grow="auto">
            <input id={GROUP_REASON_INPUT_ID} value={reason} autoComplete="off" onChange={(e) => setReason(e.target.value)} />
          </Field>
          {serverError !== "" && (
            <p className="thong-bao-loi m-0" role="alert">
              {serverError}
            </p>
          )}
        </ConfirmDialog>
      )}

      {groups === null ? (
        <div className="border-line shadow-card rounded-card border border-solid bg-white">
          <p className="an-thi-giac" role="status">
            {GROUPS_LOADING}
          </p>
          <SkeletonRows rows={3} columns={2} />
        </div>
      ) : groups.length === 0 ? (
        <div className="border-line rounded-card border border-dashed">
          <EmptyState icon={Users} tone="neutral" title={EMPTY_GROUPS_TITLE} description={EMPTY_GROUPS_TEXT} role="status" />
        </div>
      ) : (
        <div className="border-line shadow-card rounded-card border border-solid bg-white">
          <table className="w-full border-collapse text-[12.5px]">
            <caption className="an-thi-giac">{GROUPS_TABLE_LABEL}</caption>
            <tbody>
              {groups.map((g) => (
                <tr key={g.id} className="border-line border-b border-solid last:border-b-0">
                  <td className="px-4 py-2.5">
                    <p className="text-navy m-0 font-semibold">{g.name}</p>
                    {g.description !== "" && <p className="text-ink-muted m-0 text-[11.5px]">{g.description}</p>}
                  </td>
                  <td className="text-ink px-4 py-2.5 text-right whitespace-nowrap">{memberCountText(g.member_count)}</td>
                  {canEdit && (
                    <td className="px-4 py-2.5 text-right">
                      <IconButton label={removeGroupLabel(g.name)} variant="ghost" type="button" onClick={() => openRemove(g)}>
                        <Trash2 aria-hidden="true" className="size-4" />
                      </IconButton>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
