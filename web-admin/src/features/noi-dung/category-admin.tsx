"use client";

import { Eye, EyeOff, FolderTree, ListOrdered, Pencil, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import type { UpdateCategoryIn } from "@/lib/api/noi-dung";
import type { comms_danhMucRa } from "@/lib/api/schema.gen";

import {
  CATEGORY_DELETE_BUTTON,
  CATEGORY_DELETE_NOTE,
  CATEGORY_EDIT_BUTTON,
  CATEGORY_HIDDEN,
  CATEGORY_HIDE_BUTTON,
  CATEGORY_HIDE_EXPLAINER,
  CATEGORY_PARENT_HINT,
  CATEGORY_REASON_LABEL,
  CATEGORY_SHOW_BUTTON,
  CATEGORY_SHOWN,
  CATEGORY_SLUG_FIXED,
  categoryEditValues,
  categoryPatchBody,
  DANH_MUC_RONG,
  DELETE_REASON_MAX_CHARS,
  dungCayDanhMuc,
  KHONG_CO_GI_DOI,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  nhanMucDanhMuc,
  parentChoices,
  TEN_DANH_MUC_TOI_DA,
  THU_TU_DANH_MUC_TOI_DA,
  type CategoryEditValues,
} from "./nhan-noi-dung";

/**
 * `⊞ Danh mục tin` — the commune's category tree with edit, hide/show and delete (ADR 0067 §3).
 *
 * EVERY REFUSAL IS THE SERVER'S SENTENCE, VERBATIM. `category_cycle`, `parent_missing`,
 * `category_not_empty`, the self-parent 400 and the slug 400 each come with a Vietnamese sentence written
 * for this case (`service-comms/internal/http/noi_dung_mini_app.go`, `categoryWriteError`); `KetQua`
 * deliberately carries no `code` (`lib/api/goi.ts`), so there is nothing here to branch on and no second
 * copy of the rule to drift.
 *
 * NO PERMISSION GATE HERE, same as the rest of the screen: `content.update` is checked by the server on
 * every write, and a missing key reaches the officer as the 403 sentence (rule 5, forbidden #1).
 *
 * The two writes are props so a test can watch the exact bodies without a network.
 */
export function CategoryAdmin({
  categories,
  update,
  remove,
  changed,
}: {
  categories: readonly comms_danhMucRa[];
  update: (id: string, body: UpdateCategoryIn) => Promise<KetQua<comms_danhMucRa>>;
  remove: (id: string, reason: string) => Promise<KetQua<null>>;
  /** Called after a successful write: the parent re-reads the tree (another officer may have moved it). */
  changed: () => void;
}) {
  const [open, setOpen] = useState<{ kind: "edit" | "delete"; id: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ id: string; message: string } | null>(null);

  const tree = dungCayDanhMuc(categories);

  function run(id: string, call: Promise<KetQua<unknown>>): void {
    setBusy(true);
    void call.then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setError({ id, message: kq.thongBao });
        return;
      }
      setError(null);
      setOpen(null);
      changed();
    });
  }

  if (tree.length === 0) return <p className="ghi-chu">{DANH_MUC_RONG}</p>;

  return (
    <div className="category-admin flex flex-col gap-2">
      <h3 className="m-0 text-[15px] font-semibold text-ink-900">Danh mục tin của xã</h3>
      <p className="ghi-chu m-0">{CATEGORY_HIDE_EXPLAINER}</p>
      <ul className="category-list m-0 rounded-xl border border-line px-4">
        {tree.map((m) => {
          const dm = m.dm;
          const rowError = error !== null && error.id === dm.id ? error.message : null;
          const editing = open !== null && open.id === dm.id && open.kind === "edit";
          const deleting = open !== null && open.id === dm.id && open.kind === "delete";
          return (
            <li key={dm.id} className="category-row last:border-b-0">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="ten-can-bo">{nhanMucDanhMuc(m)}</span>
                <Badge tone={dm.hidden ? "neutral" : "success"} icon={dm.hidden ? EyeOff : Eye}>
                  {dm.hidden ? CATEGORY_HIDDEN : CATEGORY_SHOWN}
                </Badge>
                <span className="dong-phu basis-full">
                  Slug: {dm.slug} · Thứ tự: {dm.order}
                </span>
              </div>
              {/* Every action keeps its word (spec §7: an act with consequences is never icon-only).
                  `min-h-0` lifts the legacy 44px of `.category-row .nut-phu` to the 34px row size. */}
              <div className="cum-nut">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="min-h-0"
                  icon={<Pencil aria-hidden="true" focusable="false" />}
                  aria-expanded={editing}
                  disabled={busy}
                  aria-label={`${CATEGORY_EDIT_BUTTON} danh mục ${dm.name}`}
                  onClick={() => {
                    setError(null);
                    setOpen(editing ? null : { kind: "edit", id: dm.id });
                  }}
                >
                  {CATEGORY_EDIT_BUTTON}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="min-h-0"
                  icon={dm.hidden ? <Eye aria-hidden="true" focusable="false" /> : <EyeOff aria-hidden="true" focusable="false" />}
                  disabled={busy}
                  aria-label={`${dm.hidden ? CATEGORY_SHOW_BUTTON : CATEGORY_HIDE_BUTTON}: ${dm.name}`}
                  onClick={() => run(dm.id, update(dm.id, { hidden: !dm.hidden }))}
                >
                  {dm.hidden ? CATEGORY_SHOW_BUTTON : CATEGORY_HIDE_BUTTON}
                </Button>
                <Button
                  type="button"
                  variant="danger"
                  size="sm"
                  className="min-h-0"
                  icon={<Trash2 aria-hidden="true" focusable="false" />}
                  aria-expanded={deleting}
                  disabled={busy}
                  aria-label={`${CATEGORY_DELETE_BUTTON} danh mục ${dm.name}`}
                  onClick={() => {
                    setError(null);
                    setOpen(deleting ? null : { kind: "delete", id: dm.id });
                  }}
                >
                  {CATEGORY_DELETE_BUTTON}
                </Button>
              </div>

              {rowError !== null && !editing && !deleting && (
                <p className="thong-bao-loi" role="alert">
                  {rowError}
                </p>
              )}

              {editing && (
                <CategoryEditForm
                  category={dm}
                  categories={categories}
                  busy={busy}
                  error={rowError}
                  cancel={() => setOpen(null)}
                  save={(body) => run(dm.id, update(dm.id, body))}
                  nothingChanged={() => setError({ id: dm.id, message: KHONG_CO_GI_DOI })}
                />
              )}

              {deleting && (
                <CategoryDeleteForm
                  category={dm}
                  busy={busy}
                  error={rowError}
                  cancel={() => setOpen(null)}
                  confirm={(reason) => run(dm.id, remove(dm.id, reason))}
                  hideInstead={() => run(dm.id, update(dm.id, { hidden: true }))}
                />
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Edit one category: name, parent, order. The slug is shown, not editable — an issued code never changes.
 *
 * THE PARENT SELECT LEAVES OUT THE CATEGORY AND ITS DESCENDANTS (`parentChoices`) as a hint only; the
 * server decides (409 `category_cycle`) and its sentence reaches `error`.
 *
 * `save` gets ONLY the fields that moved (`categoryPatchBody`). Nothing moved → no PATCH, a sentence.
 */
export function CategoryEditForm({
  category,
  categories,
  busy,
  error,
  cancel,
  save,
  nothingChanged,
}: {
  category: comms_danhMucRa;
  categories: readonly comms_danhMucRa[];
  busy: boolean;
  error: string | null;
  cancel: () => void;
  save: (body: UpdateCategoryIn) => void;
  nothingChanged: () => void;
}) {
  const [v, setV] = useState<CategoryEditValues>(() => categoryEditValues(category));
  const choices = parentChoices(category.id, categories);
  const fid = `sua-danh-muc-${category.id}`;
  const ready = v.name.trim() !== "";

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!ready) return;
    const body = categoryPatchBody(category, v);
    if (Object.keys(body).length === 0) {
      nothingChanged();
      return;
    }
    save(body);
  }

  return (
    <form className={SUB_FORM_CLASS} onSubmit={submit} aria-labelledby={`${fid}-tieu-de`}>
      <h4 id={`${fid}-tieu-de`}>Sửa danh mục: {category.name}</h4>
      <p className="ghi-chu m-0">
        Slug: <code>{category.slug}</code> — {CATEGORY_SLUG_FIXED}
      </p>

      <Field label="Tên danh mục *" htmlFor={`${fid}-ten`} grow="auto">
        <input
          id={`${fid}-ten`}
          name={`${fid}-ten`}
          value={v.name}
          maxLength={TEN_DANH_MUC_TOI_DA}
          autoComplete="off"
          onChange={(e) => setV({ ...v, name: e.target.value })}
        />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5">
          <Field label="Danh mục cha" htmlFor={`${fid}-cha`} kind="select" icon={FolderTree} grow="auto">
            <select
              id={`${fid}-cha`}
              value={v.parentId}
              aria-describedby={`${fid}-cha-goi-y`}
              onChange={(e) => setV({ ...v, parentId: e.target.value })}
            >
              <option value="">— Không có danh mục cha —</option>
              {choices.map((m) => (
                <option key={m.dm.id} value={m.dm.id}>
                  {nhanMucDanhMuc(m)}
                </option>
              ))}
            </select>
          </Field>
          <p className="ghi-chu m-0" id={`${fid}-cha-goi-y`}>
            {CATEGORY_PARENT_HINT}
          </p>
        </div>

        <Field label="Thứ tự hiển thị" htmlFor={`${fid}-thu-tu`} icon={ListOrdered} grow="auto">
          <input
            id={`${fid}-thu-tu`}
            name={`${fid}-thu-tu`}
            type="number"
            min={0}
            max={THU_TU_DANH_MUC_TOI_DA}
            value={v.order}
            onChange={(e) => setV({ ...v, order: e.target.value })}
          />
        </Field>
      </div>

      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <button type="button" className={cn("nut-phu", SUB_BUTTON, buttonVariants({ variant: "secondary" }))} disabled={busy} onClick={cancel}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className={cn("nut-chinh", SUB_BUTTON, buttonVariants({ variant: "primary" }))} disabled={busy || !ready}>
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}

/**
 * Delete one category — a soft delete with a MANDATORY reason (rule 7). The button stays off until a
 * reason is typed; the server refuses an empty one anyway.
 *
 * `Ẩn thay vì xoá` is offered up front rather than after a refusal: the refusal (409 `category_not_empty`)
 * is the common case — a category in use is the one somebody wants gone — and hiding reaches the same goal
 * without touching an article.
 */
export function CategoryDeleteForm({
  category,
  busy,
  error,
  cancel,
  confirm,
  hideInstead,
}: {
  category: comms_danhMucRa;
  busy: boolean;
  error: string | null;
  cancel: () => void;
  confirm: (reason: string) => void;
  hideInstead: () => void;
}) {
  const [reason, setReason] = useState("");
  const fid = `xoa-danh-muc-${category.id}`;
  const trimmed = reason.trim();

  function submit(e: FormEvent) {
    e.preventDefault();
    if (trimmed === "") return;
    confirm(trimmed);
  }

  return (
    <form className={SUB_FORM_CLASS} onSubmit={submit} aria-labelledby={`${fid}-tieu-de`}>
      <h4 id={`${fid}-tieu-de`}>Xoá danh mục: {category.name}</h4>
      <p className="ghi-chu m-0">{CATEGORY_DELETE_NOTE}</p>

      <Field label={CATEGORY_REASON_LABEL} htmlFor={`${fid}-ly-do`} grow="auto" className="[&_textarea]:py-2.5">
        <textarea
          id={`${fid}-ly-do`}
          name={`${fid}-ly-do`}
          rows={3}
          required
          value={reason}
          maxLength={DELETE_REASON_MAX_CHARS}
          onChange={(e) => setReason(e.target.value)}
        />
      </Field>

      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <button type="button" className={cn("nut-phu", SUB_BUTTON, buttonVariants({ variant: "secondary" }))} disabled={busy} onClick={cancel}>
          {NHAN_NUT_HUY}
        </button>
        {!category.hidden && (
          <button type="button" className={cn("nut-phu", SUB_BUTTON, buttonVariants({ variant: "secondary" }))} disabled={busy} onClick={hideInstead}>
            <EyeOff aria-hidden="true" focusable="false" />
            Ẩn thay vì xoá
          </button>
        )}
        <button type="submit" className={cn("nut-phu nut-xoa", SUB_BUTTON, buttonVariants({ variant: "danger" }))} disabled={busy || trimmed === ""}>
          <Trash2 aria-hidden="true" focusable="false" />
          Xoá danh mục
        </button>
      </div>
    </form>
  );
}

/**
 * The inline edit / delete form under a row: a muted panel, not the legacy blue-edged box. `h4` restyled
 * from here because `.form-danh-muc h4` sets its own margin and size.
 */
const SUB_FORM_CLASS = cn(
  "form-danh-muc mt-1 mb-0 flex flex-col gap-4 rounded-xl border border-line border-l-line bg-surface-muted p-4",
  "[&_h4]:m-0 [&_h4]:text-sm [&_h4]:font-semibold",
);

/** Native buttons keep `type` first; the legacy 44px / full-width rules of `.form-danh-muc` are lifted. */
const SUB_BUTTON = "w-auto min-h-0";
