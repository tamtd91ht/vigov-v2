"use client";

import { FileUp, Pencil } from "lucide-react";
import { useEffect, useId, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FieldError, FormMessage, hintClass, labelClass, TextAreaField, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatDateTime } from "@/features/communes/commune-parts";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { changeUploadPolicy, listUploadPolicies, type UploadPolicy } from "@/lib/api";
import { uploadPolicyError, type UploadPolicyField } from "@/lib/errors";
import { canManageUploadPolicies } from "@/lib/permissions";

import {
  buildUploadPolicyChange,
  formatMegabytes,
  formFromPolicy,
  MAX_FILES_CAP,
  mimeLabel,
  purposeLabel,
  type UploadPolicyForm,
} from "./upload-policy-model";

/**
 * `/gioi-han-tai-len` — the upload limits of every purpose, platform-wide (ADR 0073 #5, ADR 0052 §10).
 * Read with any `ops.*` key; "Sửa" only with `ops.upload_policy.manage` (a hint — the server checks).
 *
 * A purpose whose `mime_choices` is empty is one this build of service-platform does not class: no
 * PUT can change it, so it carries no "Sửa".
 */

export const PROPAGATION_NOTICE =
  "Giới hạn mới áp dụng chung cho mọi xã. Các dịch vụ áp dụng giới hạn mới sau tối đa 60 giây; tệp đã tải lên trước đó không bị ảnh hưởng.";

export const UNLIMITED_LABEL = "Không giới hạn";

export function UploadPolicyTable({
  items,
  canEdit,
  onEdit,
}: {
  items: readonly UploadPolicy[];
  canEdit: boolean;
  onEdit: (p: UploadPolicy) => void;
}) {
  if (items.length === 0) {
    return (
      <Card as="section">
        <EmptyState icon={FileUp} title="Chưa có giới hạn tải lên nào." />
      </Card>
    );
  }
  return (
    <TableScroll sticky aria-label="Giới hạn tải lên">
      <table className={DATA_TABLE_CLASS}>
        <caption className="sr-only">Giới hạn tải lên theo mục đích, áp dụng chung mọi xã</caption>
        <thead>
          <tr>
            <th scope="col">Mục đích</th>
            <th scope="col">Dung lượng tối đa</th>
            <th scope="col">Kiểu tệp</th>
            <th scope="col">Số tệp tối đa mỗi bản ghi</th>
            <th scope="col">Cập nhật</th>
            {canEdit ? (
              <th scope="col">
                <span className="sr-only">Thao tác</span>
              </th>
            ) : null}
          </tr>
        </thead>
        <tbody>
          {items.map((p) => (
            <tr key={p.purpose}>
              <td>
                <span className="block font-semibold text-ink-900">{purposeLabel(p.purpose)}</span>
                <code className="font-mono text-xs text-ink-500">{p.purpose}</code>
              </td>
              <td className="whitespace-nowrap text-ink-700">
                {formatMegabytes(p.max_bytes)}
                {p.max_bytes_cap > 0 ? (
                  <span className="block text-xs text-ink-500">Trần {formatMegabytes(p.max_bytes_cap)}</span>
                ) : null}
              </td>
              <td className="text-ink-700">{p.allowed_mime_types.map(mimeLabel).join(", ") || "—"}</td>
              <td className="text-ink-700">{p.max_files_per_subject === null ? UNLIMITED_LABEL : p.max_files_per_subject}</td>
              <td className="text-[13px] text-ink-500">
                {formatDateTime(p.updated_at)}
                <span className="block font-mono">{p.updated_by}</span>
              </td>
              {canEdit ? (
                <td>
                  {p.mime_choices.length > 0 ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => onEdit(p)}
                      icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                      aria-label={`Sửa giới hạn: ${purposeLabel(p.purpose)}`}
                    >
                      Sửa
                    </Button>
                  ) : null}
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

export function UploadPolicyScreen() {
  const guarded = useGuardedError();
  const keys = usePermissionKeys();
  const [items, setItems] = useState<UploadPolicy[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editing, setEditing] = useState<UploadPolicy | null>(null);

  useEffect(() => {
    let alive = true;
    listUploadPolicies().then(
      (ps) => {
        if (alive) setItems(ps);
      },
      (err: unknown) => {
        if (alive) setLoadError(guarded(err, "xem giới hạn tải lên"));
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  if (items === null) {
    return loadError === null ? (
      <Card>
        <p role="status" className="sr-only">
          Đang tải giới hạn tải lên…
        </p>
        <SkeletonRows rows={6} />
      </Card>
    ) : (
      <Card as="section">
        <ErrorState role="alert" title="Chưa tải được giới hạn tải lên" message={loadError} />
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Notice tone="info">{PROPAGATION_NOTICE}</Notice>
      <UploadPolicyTable items={items} canEdit={canManageUploadPolicies(keys)} onEdit={setEditing} />
      {editing !== null ? (
        <UploadPolicyDialog
          // A fresh form per purpose: the dialog's state never carries one purpose's values into another.
          key={editing.purpose}
          policy={editing}
          onClose={() => setEditing(null)}
          onSaved={(p) => {
            setItems((prev) => (prev ?? []).map((x) => (x.purpose === p.purpose ? p : x)));
            setEditing(null);
          }}
        />
      ) : null}
    </div>
  );
}

/** The dialog body, without loading or saving, so its fields are rendered in tests. */
export function UploadPolicyFormFields({
  policy,
  form,
  onChange,
  busy,
  error,
}: {
  policy: UploadPolicy;
  form: UploadPolicyForm;
  onChange: (f: UploadPolicyForm) => void;
  busy: boolean;
  error: { field: UploadPolicyField; text: string } | null;
}) {
  const groupId = useId();
  const at = (f: UploadPolicyField) => (error?.field === f ? error.text : null);
  const set = (patch: Partial<UploadPolicyForm>) => onChange({ ...form, ...patch });

  return (
    <>
      <Notice tone="info">{PROPAGATION_NOTICE}</Notice>
      <TextField
        label="Dung lượng tối đa mỗi tệp (MB)"
        hint={`1 MB = 1 048 576 byte. Trần của mục đích này: ${formatMegabytes(policy.max_bytes_cap)}.`}
        name="max_megabytes"
        type="text"
        inputMode="decimal"
        autoComplete="off"
        value={form.megabytes}
        onChange={(e) => set({ megabytes: e.target.value })}
        disabled={busy}
        error={at("maxBytes")}
      />

      <fieldset className="m-0 flex min-w-0 flex-col gap-2 border-0 p-0" aria-describedby={at("mimeTypes") ? groupId + "-mime-error" : undefined}>
        <legend className={labelClass}>Kiểu tệp được tải lên</legend>
        {policy.mime_choices.map((m) => {
          const checked = form.mimeTypes.includes(m);
          return (
            <label key={m} className="flex min-h-9 cursor-pointer items-center gap-2.5 text-sm text-ink-900">
              <input
                type="checkbox"
                name="allowed_mime_types"
                value={m}
                checked={checked}
                disabled={busy}
                onChange={(e) =>
                  set({ mimeTypes: e.target.checked ? [...form.mimeTypes, m] : form.mimeTypes.filter((x) => x !== m) })
                }
                className="m-0 size-5 shrink-0 cursor-pointer accent-brand-600"
              />
              <span>
                {mimeLabel(m)} <code className="font-mono text-xs text-ink-500">{m}</code>
              </span>
            </label>
          );
        })}
        {at("mimeTypes") ? <FieldError id={groupId + "-mime-error"} text={at("mimeTypes") ?? ""} /> : null}
      </fieldset>

      <fieldset className="m-0 flex min-w-0 flex-col gap-2 border-0 p-0">
        <legend className={labelClass}>Số tệp tối đa mỗi bản ghi</legend>
        <p className={hintClass}>Ví dụ: số ảnh tối đa của một phản ánh, số tệp đính kèm của một bài viết.</p>
        <label className="flex min-h-9 cursor-pointer items-center gap-2.5 text-sm text-ink-900">
          <input
            type="radio"
            name="files_limit"
            value="unlimited"
            checked={!form.limited}
            disabled={busy}
            onChange={() => set({ limited: false })}
            className="m-0 size-5 shrink-0 cursor-pointer accent-brand-600"
          />
          {UNLIMITED_LABEL}
        </label>
        <label className="flex min-h-9 cursor-pointer items-center gap-2.5 text-sm text-ink-900">
          <input
            type="radio"
            name="files_limit"
            value="limited"
            checked={form.limited}
            disabled={busy}
            onChange={() => set({ limited: true })}
            className="m-0 size-5 shrink-0 cursor-pointer accent-brand-600"
          />
          Giới hạn số tệp
        </label>
        {form.limited ? (
          <TextField
            label="Số tệp tối đa"
            hint={`Số nguyên từ 1 đến ${MAX_FILES_CAP}.`}
            name="max_files_per_subject"
            type="text"
            inputMode="numeric"
            autoComplete="off"
            value={form.maxFiles}
            onChange={(e) => set({ maxFiles: e.target.value })}
            disabled={busy}
            error={at("maxFiles")}
          />
        ) : null}
        {!form.limited && at("maxFiles") ? <FieldError id={groupId + "-files-error"} text={at("maxFiles") ?? ""} /> : null}
      </fieldset>

      <TextAreaField
        label="Lý do"
        hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký vận hành."
        name="reason"
        rows={3}
        maxLength={500}
        value={form.reason}
        onChange={(e) => set({ reason: e.target.value })}
        disabled={busy}
        error={at("reason")}
      />
      <FormMessage text={error?.field === "form" ? error.text : null} />
    </>
  );
}

function UploadPolicyDialog({
  policy,
  onClose,
  onSaved,
}: {
  policy: UploadPolicy;
  onClose: () => void;
  onSaved: (p: UploadPolicy) => void;
}) {
  const guarded = useGuardedError();
  const [form, setForm] = useState<UploadPolicyForm>(() => formFromPolicy(policy));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: UploadPolicyField; text: string } | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const built = buildUploadPolicyChange(form, policy);
    if (!built.ok) return setError({ field: built.field, text: built.text });
    setBusy(true);
    setError(null);
    try {
      onSaved(await changeUploadPolicy(policy.purpose, built.body));
    } catch (err) {
      let field: UploadPolicyField = "form";
      const text = guarded(err, "sửa giới hạn tải lên", (e) => {
        const known = uploadPolicyError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Sửa giới hạn: ${purposeLabel(policy.purpose)}`} icon={Pencil} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <UploadPolicyFormFields policy={policy} form={form} onChange={setForm} busy={busy} error={error} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Lưu giới hạn"}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
