"use client";

import { Archive, ArchiveRestore, Pencil, Plus, Tags, TriangleAlert } from "lucide-react";
import { useEffect, useId, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { controlClass, FieldError, FormMessage, labelClass, TextAreaField, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  createPetitionField,
  editPetitionField,
  listPetitionFields,
  setPetitionFieldActivation,
  type PetitionField,
  type PetitionFieldList,
} from "@/lib/api";
import { petitionFieldError, type PetitionFieldFormField } from "@/lib/errors";
import { canManagePetitionFields } from "@/lib/permissions";

/**
 * `/linh-vuc-phan-anh` — the tier-1 petition field codes (ADR 0060, ADR 0073 #3). ONE code set for
 * every commune; a commune's own wording, order and on/off switch are tier 2, in service-petitions,
 * and not here. Read with any `ops.*` key; every write needs `ops.petition_field.manage` (a hint —
 * the server checks).
 *
 * A CODE IS FOREVER: stored on archival petitions as typed (ADR 0060 §4). The edit dialog shows it as
 * text, never as an input, and the edit body has no `code` key (lib/api.ts). A code is never deleted:
 * "Ngừng dùng" retires it in every commune, and a retired code stays listed because it still labels
 * old petitions.
 */

type FieldError = { field: PetitionFieldFormField; text: string };

export const CODE_FOREVER_NOTICE =
  "Mã lĩnh vực được ghi vào các phản ánh và giữ nguyên vĩnh viễn: sau khi cấp, mã không đổi được và không xoá được. Kiểm tra kỹ trước khi lưu.";

export const RETIRE_WARNING =
  "Ngừng dùng có hiệu lực ở mọi xã (các dịch vụ áp dụng sau tối đa 60 giây): người dân không chọn được lĩnh vực này cho phản ánh mới. Phản ánh cũ vẫn giữ nhãn lĩnh vực. Có thể dùng lại sau.";

const TONE_LABELS: Record<string, string> = {
  blue: "Xanh dương",
  green: "Xanh lá",
  orange: "Cam",
  purple: "Tím",
  cyan: "Xanh lơ",
  red: "Đỏ",
};

export function toneLabel(tone: string): string {
  if (tone === "") return "Không đặt";
  return TONE_LABELS[tone] ?? tone;
}

export function PetitionFieldTable({
  items,
  canManage,
  onEdit,
  onToggle,
}: {
  items: readonly PetitionField[];
  canManage: boolean;
  onEdit: (f: PetitionField) => void;
  onToggle: (f: PetitionField) => void;
}) {
  if (items.length === 0) {
    return (
      <Card as="section">
        <EmptyState icon={Tags} title="Chưa có mã lĩnh vực nào." />
      </Card>
    );
  }
  return (
    <TableScroll sticky aria-label="Lĩnh vực phản ánh cấp 1">
      <table className={DATA_TABLE_CLASS}>
        <caption className="sr-only">Mã lĩnh vực phản ánh cấp 1, gồm cả mã đã ngừng dùng</caption>
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Nhãn mặc định</th>
            <th scope="col">Thứ tự</th>
            <th scope="col">Biểu tượng</th>
            <th scope="col">Tông màu</th>
            <th scope="col">Trạng thái</th>
            {canManage ? (
              <th scope="col">
                <span className="sr-only">Thao tác</span>
              </th>
            ) : null}
          </tr>
        </thead>
        <tbody>
          {items.map((f) => (
            <tr key={f.code}>
              <td className="font-mono text-[13px] text-ink-900">{f.code}</td>
              <td className="font-semibold text-ink-900">{f.default_label}</td>
              <td className="text-ink-700">{f.sort_order}</td>
              <td className="font-mono text-[13px] text-ink-700">{f.icon || "—"}</td>
              <td className="text-ink-700">{toneLabel(f.tone)}</td>
              <td>{f.active ? <Badge tone="success">Đang dùng</Badge> : <Badge tone="neutral">Ngừng dùng</Badge>}</td>
              {canManage ? (
                <td>
                  <div className="flex flex-wrap gap-1">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => onEdit(f)}
                      icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                      aria-label={`Sửa lĩnh vực ${f.code}`}
                    >
                      Sửa
                    </Button>
                    <Button
                      type="button"
                      variant={f.active ? "danger" : "secondary"}
                      size="sm"
                      onClick={() => onToggle(f)}
                      icon={
                        f.active ? (
                          <Archive aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        ) : (
                          <ArchiveRestore aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        )
                      }
                      aria-label={`${f.active ? "Ngừng dùng" : "Dùng lại"} lĩnh vực ${f.code}`}
                    >
                      {f.active ? "Ngừng dùng" : "Dùng lại"}
                    </Button>
                  </div>
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

type Open = { kind: "create" } | { kind: "edit"; field: PetitionField } | { kind: "toggle"; field: PetitionField } | null;

export function PetitionFieldActions({ canManage, onCreate }: { canManage: boolean; onCreate: () => void }) {
  if (!canManage) return null;
  return (
    <Button type="button" variant="primary" onClick={onCreate} icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
      Thêm lĩnh vực
    </Button>
  );
}

export function PetitionFieldScreen() {
  const guarded = useGuardedError();
  const keys = usePermissionKeys();
  const canManage = canManagePetitionFields(keys);
  const [list, setList] = useState<PetitionFieldList | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [open, setOpen] = useState<Open>(null);

  useEffect(() => {
    let alive = true;
    listPetitionFields().then(
      (l) => {
        if (alive) setList(l);
      },
      (err: unknown) => {
        if (alive) setLoadError(guarded(err, "xem lĩnh vực phản ánh"));
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  function saved(f: PetitionField) {
    setList((prev) => {
      if (prev === null) return prev;
      const exists = prev.items.some((x) => x.code === f.code);
      return { ...prev, items: exists ? prev.items.map((x) => (x.code === f.code ? f : x)) : [...prev.items, f] };
    });
    setOpen(null);
  }

  if (list === null) {
    return loadError === null ? (
      <Card>
        <p role="status" className="sr-only">
          Đang tải lĩnh vực phản ánh…
        </p>
        <SkeletonRows rows={6} />
      </Card>
    ) : (
      <Card as="section">
        <ErrorState role="alert" title="Chưa tải được lĩnh vực phản ánh" message={loadError} />
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-start gap-3">
        <Notice tone="info" className="min-w-0 flex-1 basis-80">
          Bộ mã dùng chung cho mọi xã. Nhãn, thứ tự và việc bật/tắt riêng của từng xã là cấu hình của xã, không sửa ở
          đây.
        </Notice>
        <PetitionFieldActions canManage={canManage} onCreate={() => setOpen({ kind: "create" })} />
      </div>
      <PetitionFieldTable
        items={list.items}
        canManage={canManage}
        onEdit={(f) => setOpen({ kind: "edit", field: f })}
        onToggle={(f) => setOpen({ kind: "toggle", field: f })}
      />
      {open?.kind === "create" ? <FieldDialog tones={list.tones} field={null} onClose={() => setOpen(null)} onSaved={saved} /> : null}
      {open?.kind === "edit" ? (
        <FieldDialog key={open.field.code} tones={list.tones} field={open.field} onClose={() => setOpen(null)} onSaved={saved} />
      ) : null}
      {open?.kind === "toggle" ? (
        <ActivationDialog key={open.field.code} field={open.field} onClose={() => setOpen(null)} onSaved={saved} />
      ) : null}
    </div>
  );
}

function useFieldError() {
  const guarded = useGuardedError();
  return (err: unknown, action: string): FieldError | null => {
    let field: PetitionFieldFormField = "form";
    const text = guarded(err, action, (e) => {
      const known = petitionFieldError(e);
      if (known) field = known.field;
      return known?.text ?? null;
    });
    return text === null ? null : { field, text };
  };
}

export type FieldForm = { code: string; label: string; sortOrder: string; icon: string; tone: string; reason: string };

/** The form's values as a request, or the first field to fix. The server checks everything again. */
export function checkFieldForm(form: FieldForm, creating: boolean): FieldError | null {
  if (creating && form.code.trim() === "") return { field: "code", text: "Hãy nhập mã lĩnh vực." };
  if (form.label.trim() === "") return { field: "label", text: "Hãy nhập nhãn mặc định." };
  const t = form.sortOrder.trim();
  if (!/^\d+$/.test(t) || Number(t) < 1 || Number(t) > 10000) {
    return { field: "sortOrder", text: "Thứ tự phải là số nguyên từ 1 đến 10000." };
  }
  if (form.reason.trim() === "") return { field: "reason", text: "Hãy ghi lý do." };
  return null;
}

/** The dialog body, without saving, so it is rendered in tests: the code is an input only on create. */
export function PetitionFieldFormFields({
  field,
  tones,
  form,
  onChange,
  busy,
  error,
}: {
  field: PetitionField | null;
  tones: readonly string[];
  form: FieldForm;
  onChange: (f: FieldForm) => void;
  busy: boolean;
  error: FieldError | null;
}) {
  const toneId = useId();
  const at = (f: PetitionFieldFormField) => (error?.field === f ? error.text : null);
  const set = (patch: Partial<FieldForm>) => onChange({ ...form, ...patch });
  return (
    <>
      {field === null ? (
        <>
          <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
            {CODE_FOREVER_NOTICE}
          </Notice>
          <TextField
            label="Mã lĩnh vực"
            hint="Chữ thường không dấu và chữ số, các phần nối bằng dấu gạch ngang; tối đa 64 ký tự. Ví dụ: moi-truong."
            name="code"
            type="text"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={64}
            value={form.code}
            onChange={(e) => set({ code: e.target.value })}
            disabled={busy}
            error={at("code")}
          />
        </>
      ) : (
        <div className="flex min-w-0 flex-col gap-1">
          <span className={labelClass}>Mã lĩnh vực (không đổi được)</span>
          <code className="font-mono text-sm break-all text-ink-900">{field.code}</code>
        </div>
      )}
      <TextField
        label="Nhãn mặc định"
        hint="Tối đa 200 ký tự, một dòng."
        name="default_label"
        type="text"
        autoComplete="off"
        maxLength={200}
        value={form.label}
        onChange={(e) => set({ label: e.target.value })}
        disabled={busy}
        error={at("label")}
      />
      <TextField
        label="Thứ tự mặc định"
        hint="Số nguyên từ 1 đến 10000; số nhỏ đứng trước."
        name="sort_order"
        type="text"
        inputMode="numeric"
        autoComplete="off"
        value={form.sortOrder}
        onChange={(e) => set({ sortOrder: e.target.value })}
        disabled={busy}
        error={at("sortOrder")}
      />
      <TextField
        label="Biểu tượng (không bắt buộc)"
        hint="Tên biểu tượng lucide, ví dụ Trash2. Để trống nếu không đặt."
        name="icon"
        type="text"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        maxLength={64}
        value={form.icon}
        onChange={(e) => set({ icon: e.target.value })}
        disabled={busy}
        error={at("icon")}
      />
      <div className="flex min-w-0 flex-col gap-1.5">
        <label htmlFor={toneId} className={labelClass}>
          Tông màu
        </label>
        <select
          id={toneId}
          name="tone"
          className={controlClass}
          value={form.tone}
          onChange={(e) => set({ tone: e.target.value })}
          disabled={busy}
          aria-invalid={at("tone") ? true : undefined}
          aria-describedby={at("tone") ? toneId + "-error" : undefined}
        >
          <option value="">{toneLabel("")}</option>
          {tones.map((t) => (
            <option key={t} value={t}>
              {toneLabel(t)}
            </option>
          ))}
        </select>
        {at("tone") ? <FieldError id={toneId + "-error"} text={at("tone") ?? ""} /> : null}
      </div>
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

function FieldDialog({
  field,
  tones,
  onClose,
  onSaved,
}: {
  field: PetitionField | null;
  tones: readonly string[];
  onClose: () => void;
  onSaved: (f: PetitionField) => void;
}) {
  const fieldError = useFieldError();
  const creating = field === null;
  const [form, setForm] = useState<FieldForm>(() => ({
    code: "",
    label: field?.default_label ?? "",
    sortOrder: field ? String(field.sort_order) : "",
    icon: field?.icon ?? "",
    tone: field?.tone ?? "",
    reason: "",
  }));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const problem = checkFieldForm(form, creating);
    if (problem) return setError(problem);
    const presentation = {
      defaultLabel: form.label.trim(),
      sortOrder: Number(form.sortOrder.trim()),
      icon: form.icon.trim(),
      tone: form.tone,
      reason: form.reason,
    };
    setBusy(true);
    setError(null);
    try {
      onSaved(
        field === null
          ? await createPetitionField({ ...presentation, code: form.code.trim() })
          : await editPetitionField(field.code, presentation),
      );
    } catch (err) {
      setError(fieldError(err, creating ? "thêm lĩnh vực phản ánh" : "sửa lĩnh vực phản ánh"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      title={creating ? "Thêm lĩnh vực phản ánh" : `Sửa lĩnh vực ${field.code}`}
      icon={creating ? Plus : Pencil}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <PetitionFieldFormFields field={field} tones={tones} form={form} onChange={setForm} busy={busy} error={error} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : creating ? "Cấp mã lĩnh vực" : "Lưu thay đổi"}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function ActivationDialog({
  field,
  onClose,
  onSaved,
}: {
  field: PetitionField;
  onClose: () => void;
  onSaved: (f: PetitionField) => void;
}) {
  const fieldError = useFieldError();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);
  const retiring = field.active;
  const label = retiring ? "Ngừng dùng" : "Dùng lại";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      onSaved(await setPetitionFieldActivation(field.code, { active: !retiring, reason }));
    } catch (err) {
      setError(fieldError(err, retiring ? "ngừng dùng lĩnh vực phản ánh" : "dùng lại lĩnh vực phản ánh"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      title={`${label} lĩnh vực “${field.default_label}”?`}
      tone={retiring ? "danger" : "default"}
      icon={retiring ? TriangleAlert : ArchiveRestore}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        {retiring ? (
          <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
            {RETIRE_WARNING}
          </Notice>
        ) : (
          <p>Lĩnh vực sẽ dùng lại được ở mọi xã (các dịch vụ áp dụng sau tối đa 60 giây).</p>
        )}
        <p>
          Mã: <code className="font-mono text-[13px] text-ink-900">{field.code}</code>
        </p>
        <TextAreaField
          label="Lý do"
          hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký vận hành."
          name="reason"
          rows={3}
          maxLength={500}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          disabled={busy}
          error={error?.field === "reason" ? error.text : null}
        />
        <FormMessage text={error && error.field !== "reason" ? error.text : null} />
        <DialogActions>
          <Button type="submit" variant={retiring ? "danger-solid" : "primary"} disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : label}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
