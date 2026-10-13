"use client";

import { Trash2, TriangleAlert } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, hintClass, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Notice } from "@/components/ui/notice";
import { deleteCommune, type CommuneDetail } from "@/lib/api";

import {
  canSubmitDeletion,
  communeDeletionError,
  DELETION_EXPLANATION,
  DELETION_NEEDS_INACTIVE,
  type OwnerError,
} from "./owner-model";
import { at, formLevel, ReasonField, useOwnerError } from "./owner-parts";

/**
 * "Xoá xã" — the SOFT delete of a commune, OWNER ONLY (ADR 0048 §Sửa đổi 13/10/2026 #7, #8).
 *
 * TWO LAYERS, BOTH THE SERVER'S: the commune inactive for 7 days in a row, and one of its domains typed
 * again. The 7-day clock is not in the commune detail the console reads (`communeDetailView` has no
 * deactivation time), so the button opens for any inactive commune and the server's 409 says when the
 * deletion becomes possible. The domain field is never pre-filled: typing it IS the confirmation.
 *
 * After 201 the commune no longer resolves (GET /communes/{id} answers 404), so the page leaves for the
 * list, replacing this entry in the history so Back does not land on a commune that is gone.
 */

export const DELETE_LABEL = "Xoá xã";
export const DELETED_PATH = "/xa?deleted=1";

export function CommuneDeletionSection({ commune }: { commune: CommuneDetail }) {
  const [open, setOpen] = useState(false);
  return (
    <Card as="section" aria-labelledby="commune-deletion" className="border-danger-200">
      <CardHeader>
        <Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-danger-600" />
        <CardTitle id="commune-deletion">Vùng nguy hiểm — Xoá xã</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col items-start gap-3">
        <p className="m-0 text-sm text-ink-700">{DELETION_EXPLANATION}</p>
        <Button
          type="button"
          variant="danger"
          disabled={commune.active}
          icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={() => setOpen(true)}
        >
          {DELETE_LABEL}
        </Button>
        {commune.active ? <p className={hintClass}>{DELETION_NEEDS_INACTIVE}</p> : null}
      </CardContent>
      {open ? <CommuneDeletionDialog commune={commune} onClose={() => setOpen(false)} /> : null}
    </Card>
  );
}

function CommuneDeletionDialog({ commune, onClose }: { commune: CommuneDetail; onClose: () => void }) {
  const router = useRouter();
  const toError = useOwnerError(communeDeletionError);
  const [confirmDomain, setConfirmDomain] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<OwnerError | null>(null);
  const ready = canSubmitDeletion(confirmDomain, reason);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !ready) return;
    setBusy(true);
    setError(null);
    try {
      await deleteCommune(commune.id, { confirmDomain: confirmDomain.trim(), reason: reason.trim() });
      router.replace(DELETED_PATH);
    } catch (err) {
      setError(toError(err, "xoá xã"));
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Xoá ${commune.name}?`} tone="danger" icon={TriangleAlert} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
          <p>{DELETION_EXPLANATION}</p>
        </Notice>
        <TextField
          label="Gõ lại một tên miền của xã để xác nhận"
          hint="Gõ đúng một trong các tên miền của xã, không kèm https:// hay đường dẫn."
          name="confirm_domain"
          type="text"
          inputMode="url"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          value={confirmDomain}
          onChange={(e) => setConfirmDomain(e.target.value)}
          disabled={busy}
          error={at(error, "confirmDomain")}
        />
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "confirmDomain", "reason")} />
        <DialogActions>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
          <Button type="submit" variant="danger-solid" disabled={busy || !ready} aria-busy={busy}>
            {busy ? "Đang xoá…" : DELETE_LABEL}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
