"use client";

import { EditorContent, useEditor, useEditorState, type Editor } from "@tiptap/react";
import { useEffect, useState } from "react";

import { bodyFromEditor, isHttpsLink, richTextExtensions } from "./rich-text";

/**
 * §7 `Nội dung` — the rich-text box (ADR 0067 §1 decision 6). Tiptap, headless, with the schema of
 * `rich-text.ts`: the toolbar has exactly the buttons the server's allow-list keeps.
 *
 * THE EDITOR IS ITS OWN PREVIEW. ProseMirror draws the document node by node through the schema's
 * `toDOM`, so what the officer sees is built from the parsed structure — no string of HTML is ever handed
 * to the page (rule 13, forbidden #3; `ranh-gioi-html.test.ts` reads this folder for it).
 *
 * `onChange` FIRES ONLY ON AN EDIT. Loading a stored body is not one: a body stored without `<p>` would
 * print differently once parsed, and reporting that as a change would make every Lưu of the edit form
 * overwrite a body nobody touched (`thanSua` compares strings).
 *
 * `immediatelyRender: false` because the page is rendered on the server first and an editor needs a DOM;
 * until it mounts, a sentence stands in its place.
 */

export const EDITOR_LOADING = "Đang mở ô soạn thảo…";
export const LINK_INVALID =
  "Liên kết phải bắt đầu bằng https:// và có tên miền, không có dấu cách — liên kết khác bị bỏ khi lưu.";
export const LINK_NEEDS_SELECTION = "Bôi đen đoạn chữ cần gắn liên kết trước.";

type ToolbarState = {
  paragraph: boolean;
  h2: boolean;
  h3: boolean;
  bold: boolean;
  italic: boolean;
  bulletList: boolean;
  orderedList: boolean;
  link: boolean;
  canUndo: boolean;
  canRedo: boolean;
};

function toolbarState(editor: Editor | null): ToolbarState | null {
  if (editor === null) return null;
  return {
    paragraph: editor.isActive("paragraph"),
    h2: editor.isActive("heading", { level: 2 }),
    h3: editor.isActive("heading", { level: 3 }),
    bold: editor.isActive("bold"),
    italic: editor.isActive("italic"),
    bulletList: editor.isActive("bulletList"),
    orderedList: editor.isActive("orderedList"),
    link: editor.isActive("link"),
    canUndo: editor.can().undo(),
    canRedo: editor.can().redo(),
  };
}

export function RichTextEditor({
  id,
  labelId,
  initialHtml,
  disabled,
  onChange,
}: {
  /** id of the editable area — the hint below points at it. */
  id: string;
  /** id of the visible label (`Nội dung`): the editable area is a `div`, so `htmlFor` cannot reach it. */
  labelId: string;
  initialHtml: string;
  disabled: boolean;
  onChange: (body: string) => void;
}) {
  const [linkDraft, setLinkDraft] = useState("");
  const [linkError, setLinkError] = useState<string | null>(null);

  const editor = useEditor({
    extensions: richTextExtensions(),
    content: initialHtml,
    editable: !disabled,
    immediatelyRender: false,
    // No `<style>` injected at runtime: a Content-Security-Policy without 'unsafe-inline' would block
    // it (rule 13, invariant 5). The few rules ProseMirror needs live in globals.css (`.rich-text-area`).
    injectCSS: false,
    editorProps: {
      attributes: {
        id,
        class: "rich-text-area",
        role: "textbox",
        "aria-multiline": "true",
        "aria-labelledby": labelId,
        "aria-describedby": `${id}-hint`,
      },
    },
    onUpdate: ({ editor: e }) => onChange(bodyFromEditor(e)),
  });

  // `editable` above is read once; a form that starts saving after mount must freeze the box too.
  useEffect(() => {
    if (editor !== null && editor.isEditable === disabled) editor.setEditable(!disabled);
  }, [editor, disabled]);

  // The selector's snapshot stays at its first value (no editor yet) until the editor's first
  // transaction, so the toolbar falls back to reading the editor directly in between.
  const selected = useEditorState({ editor, selector: ({ editor: e }) => toolbarState(e) });
  const state = selected ?? toolbarState(editor);

  function applyLink(): void {
    if (editor === null) return;
    const href = linkDraft.trim();
    if (!isHttpsLink(href)) {
      setLinkError(LINK_INVALID);
      return;
    }
    if (editor.state.selection.empty && !editor.isActive("link")) {
      setLinkError(LINK_NEEDS_SELECTION);
      return;
    }
    editor.chain().focus().extendMarkRange("link").setLink({ href }).run();
    setLinkError(null);
    setLinkDraft("");
  }

  if (editor === null || state === null) {
    return (
      <p className="ghi-chu" role="status">
        {EDITOR_LOADING}
      </p>
    );
  }

  const off = disabled;
  // aria-pressed on a toggle, plain on an action: a screen reader says "đang bật" only where it is true.
  const toggle = (label: string, pressed: boolean, run: () => void) => (
    <button
      type="button"
      className="nut-phu"
      aria-pressed={pressed}
      disabled={off}
      onMouseDown={(e) => e.preventDefault() /* keep the selection in the editor */}
      onClick={run}
    >
      {label}
    </button>
  );

  return (
    <div className="rich-text">
      <div className="cum-nut rich-text-toolbar" role="toolbar" aria-label="Định dạng nội dung">
        {toggle("Đoạn văn", state.paragraph, () => editor.chain().focus().setParagraph().run())}
        {toggle("Tiêu đề lớn", state.h2, () => editor.chain().focus().toggleHeading({ level: 2 }).run())}
        {toggle("Tiêu đề nhỏ", state.h3, () => editor.chain().focus().toggleHeading({ level: 3 }).run())}
        {toggle("Đậm", state.bold, () => editor.chain().focus().toggleBold().run())}
        {toggle("Nghiêng", state.italic, () => editor.chain().focus().toggleItalic().run())}
        {toggle("Danh sách chấm", state.bulletList, () => editor.chain().focus().toggleBulletList().run())}
        {toggle("Danh sách số", state.orderedList, () => editor.chain().focus().toggleOrderedList().run())}
        <button
          type="button"
          className="nut-phu"
          disabled={off || !state.canUndo}
          onClick={() => editor.chain().focus().undo().run()}
        >
          Hoàn tác
        </button>
        <button
          type="button"
          className="nut-phu"
          disabled={off || !state.canRedo}
          onClick={() => editor.chain().focus().redo().run()}
        >
          Làm lại
        </button>
      </div>

      <EditorContent editor={editor} />

      <div className="o-nhap rich-text-link">
        <label htmlFor={`${id}-link`}>Liên kết cho đoạn chữ đang chọn</label>
        <input
          id={`${id}-link`}
          name={`${id}-link`}
          type="url"
          inputMode="url"
          placeholder="https://"
          autoComplete="off"
          value={linkDraft}
          disabled={off}
          onChange={(e) => {
            setLinkDraft(e.target.value);
            setLinkError(null);
          }}
          onKeyDown={(e) => {
            // Enter inside the article form would submit the whole article.
            if (e.key === "Enter") {
              e.preventDefault();
              applyLink();
            }
          }}
        />
        <div className="cum-nut">
          <button type="button" className="nut-phu" disabled={off} onClick={applyLink}>
            Gắn liên kết
          </button>
          <button
            type="button"
            className="nut-phu"
            disabled={off || !state.link}
            onClick={() => editor.chain().focus().extendMarkRange("link").unsetLink().run()}
          >
            Gỡ liên kết
          </button>
        </div>
        {linkError !== null && (
          <p className="thong-bao-loi" role="alert">
            {linkError}
          </p>
        )}
      </div>
    </div>
  );
}
