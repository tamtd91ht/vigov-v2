"use client";

import { useState, type FormEvent } from "react";

import type { KetQua } from "@/lib/api/goi";
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
    <div className="category-admin">
      <h3>Danh mục tin của xã</h3>
      <p className="ghi-chu">{CATEGORY_HIDE_EXPLAINER}</p>
      <ul className="category-list">
        {tree.map((m) => {
          const dm = m.dm;
          const rowError = error !== null && error.id === dm.id ? error.message : null;
          const editing = open !== null && open.id === dm.id && open.kind === "edit";
          const deleting = open !== null && open.id === dm.id && open.kind === "delete";
          return (
            <li key={dm.id} className="category-row">
              <div>
                <span className="ten-can-bo">{nhanMucDanhMuc(m)}</span>
                <span className="dong-phu">
                  Slug: {dm.slug} · Thứ tự: {dm.order}
                </span>
                <span className={dm.hidden ? "chip chip-ngung" : "chip chip-hoat-dong"}>
                  {dm.hidden ? CATEGORY_HIDDEN : CATEGORY_SHOWN}
                </span>
              </div>
              <div className="cum-nut">
                <button
                  type="button"
                  className="nut-phu"
                  aria-expanded={editing}
                  disabled={busy}
                  aria-label={`${CATEGORY_EDIT_BUTTON} danh mục ${dm.name}`}
                  onClick={() => {
                    setError(null);
                    setOpen(editing ? null : { kind: "edit", id: dm.id });
                  }}
                >
                  {CATEGORY_EDIT_BUTTON}
                </button>
                <button
                  type="button"
                  className="nut-phu"
                  disabled={busy}
                  aria-label={`${dm.hidden ? CATEGORY_SHOW_BUTTON : CATEGORY_HIDE_BUTTON}: ${dm.name}`}
                  onClick={() => run(dm.id, update(dm.id, { hidden: !dm.hidden }))}
                >
                  {dm.hidden ? CATEGORY_SHOW_BUTTON : CATEGORY_HIDE_BUTTON}
                </button>
                <button
                  type="button"
                  className="nut-phu nut-xoa"
                  aria-expanded={deleting}
                  disabled={busy}
                  aria-label={`${CATEGORY_DELETE_BUTTON} danh mục ${dm.name}`}
                  onClick={() => {
                    setError(null);
                    setOpen(deleting ? null : { kind: "delete", id: dm.id });
                  }}
                >
                  {CATEGORY_DELETE_BUTTON}
                </button>
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
    <form className="form-danh-muc" onSubmit={submit} aria-labelledby={`${fid}-tieu-de`}>
      <h4 id={`${fid}-tieu-de`}>Sửa danh mục: {category.name}</h4>
      <p className="ghi-chu">
        Slug: <code>{category.slug}</code> — {CATEGORY_SLUG_FIXED}
      </p>

      <div className="o-nhap">
        <label htmlFor={`${fid}-ten`}>Tên danh mục *</label>
        <input
          id={`${fid}-ten`}
          name={`${fid}-ten`}
          value={v.name}
          maxLength={TEN_DANH_MUC_TOI_DA}
          autoComplete="off"
          onChange={(e) => setV({ ...v, name: e.target.value })}
        />
      </div>

      <div className="o-chon">
        <label htmlFor={`${fid}-cha`}>Danh mục cha</label>
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
      </div>
      <p className="ghi-chu" id={`${fid}-cha-goi-y`}>
        {CATEGORY_PARENT_HINT}
      </p>

      <div className="o-nhap">
        <label htmlFor={`${fid}-thu-tu`}>Thứ tự hiển thị</label>
        <input
          id={`${fid}-thu-tu`}
          name={`${fid}-thu-tu`}
          type="number"
          min={0}
          max={THU_TU_DANH_MUC_TOI_DA}
          value={v.order}
          onChange={(e) => setV({ ...v, order: e.target.value })}
        />
      </div>

      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={busy} onClick={cancel}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={busy || !ready}>
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
    <form className="form-danh-muc" onSubmit={submit} aria-labelledby={`${fid}-tieu-de`}>
      <h4 id={`${fid}-tieu-de`}>Xoá danh mục: {category.name}</h4>
      <p className="ghi-chu">{CATEGORY_DELETE_NOTE}</p>

      <div className="o-nhap">
        <label htmlFor={`${fid}-ly-do`}>{CATEGORY_REASON_LABEL}</label>
        <textarea
          id={`${fid}-ly-do`}
          name={`${fid}-ly-do`}
          rows={3}
          required
          value={reason}
          maxLength={DELETE_REASON_MAX_CHARS}
          onChange={(e) => setReason(e.target.value)}
        />
      </div>

      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={busy} onClick={cancel}>
          {NHAN_NUT_HUY}
        </button>
        {!category.hidden && (
          <button type="button" className="nut-phu" disabled={busy} onClick={hideInstead}>
            Ẩn thay vì xoá
          </button>
        )}
        <button type="submit" className="nut-phu nut-xoa" disabled={busy || trimmed === ""}>
          Xoá danh mục
        </button>
      </div>
    </form>
  );
}
