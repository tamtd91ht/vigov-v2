"use client";

import { EditorContent, useEditor, useEditorState, type Editor } from "@tiptap/react";
import {
  Bold,
  Heading1,
  Heading2,
  Italic,
  Link as LinkIcon,
  List,
  ListOrdered,
  Pilcrow,
  Redo2,
  Undo2,
  Unlink,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { cn } from "@/lib/cn";

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
      // The editor's own height, so the dialog does not jump when the editor mounts a tick later.
      <p
        className="ghi-chu m-0 grid min-h-[15rem] place-items-center rounded-control border border-line-strong bg-surface-muted px-4 text-center"
        role="status"
      >
        {EDITOR_LOADING}
      </p>
    );
  }

  const off = disabled;
  // ICON BUTTONS THAT KEEP THEIR WORD. The Vietnamese name stays in the button as visually hidden text
  // (`an-thi-giac`), so the accessible name — and the toolbar's test — read exactly what the old text
  // button said; `title` gives a sighted officer the same word on hover. No Radix Tooltip: the toolbar
  // must stay plain markup. aria-pressed on a toggle, plain on an action: a screen reader says "đang bật"
  // only where it is true.
  const tool = (
    label: string,
    Icon: LucideIcon,
    run: () => void,
    opts: { pressed?: boolean; disabled?: boolean } = {},
  ) => (
    <button
      type="button"
      className={TOOL_CLASS}
      title={label}
      aria-pressed={opts.pressed}
      disabled={off || opts.disabled === true}
      onMouseDown={(e) => e.preventDefault() /* keep the selection in the editor */}
      onClick={run}
    >
      <Icon aria-hidden="true" focusable="false" strokeWidth={1.9} className="size-[18px]" />
      <span className="an-thi-giac">{label}</span>
    </button>
  );

  return (
    <div
      className={cn(
        "rich-text overflow-hidden rounded-control border border-line-strong bg-surface",
        "transition-[border-color,box-shadow] duration-150",
        "focus-within:border-brand-500 focus-within:shadow-[0_0_0_3px_var(--brand-100)]",
        // The area loses its own frame: the box above draws one frame for toolbar + area + link row,
        // and its focus ring stands in for the area's legacy outline.
        "[&_.rich-text-area]:min-h-48 [&_.rich-text-area]:rounded-none [&_.rich-text-area]:border-0",
        "[&_.rich-text-area]:px-3.5 [&_.rich-text-area]:py-3 [&_.rich-text-area:focus-visible]:outline-none",
      )}
    >
      <div
        className="cum-nut rich-text-toolbar m-0 flex flex-wrap items-center gap-0.5 border-b border-line bg-surface-muted px-1.5 py-1"
        role="toolbar"
        aria-label="Định dạng nội dung"
      >
        {tool("Đoạn văn", Pilcrow, () => editor.chain().focus().setParagraph().run(), { pressed: state.paragraph })}
        {tool("Tiêu đề lớn", Heading1, () => editor.chain().focus().toggleHeading({ level: 2 }).run(), {
          pressed: state.h2,
        })}
        {tool("Tiêu đề nhỏ", Heading2, () => editor.chain().focus().toggleHeading({ level: 3 }).run(), {
          pressed: state.h3,
        })}
        <ToolSeparator />
        {tool("Đậm", Bold, () => editor.chain().focus().toggleBold().run(), { pressed: state.bold })}
        {tool("Nghiêng", Italic, () => editor.chain().focus().toggleItalic().run(), { pressed: state.italic })}
        <ToolSeparator />
        {tool("Danh sách chấm", List, () => editor.chain().focus().toggleBulletList().run(), {
          pressed: state.bulletList,
        })}
        {tool("Danh sách số", ListOrdered, () => editor.chain().focus().toggleOrderedList().run(), {
          pressed: state.orderedList,
        })}
        <ToolSeparator />
        {tool("Hoàn tác", Undo2, () => editor.chain().focus().undo().run(), { disabled: !state.canUndo })}
        {tool("Làm lại", Redo2, () => editor.chain().focus().redo().run(), { disabled: !state.canRedo })}
      </div>

      {/* The link row sits under the toolbar, inside the same frame: it acts on the selection in the area
          below, so it belongs to the toolbar, not to a separate block of the form. */}
      <div className="rich-text-link m-0 flex flex-wrap items-center gap-2 border-b border-line bg-surface-muted px-2 py-1.5">
        <label htmlFor={`${id}-link`} className="an-thi-giac">
          Liên kết cho đoạn chữ đang chọn
        </label>
        <div className="relative min-w-0 flex-[1_1_14rem]">
          <LinkIcon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-ink-500"
          />
          <input
            id={`${id}-link`}
            name={`${id}-link`}
            type="url"
            inputMode="url"
            placeholder="https://"
            autoComplete="off"
            title="Liên kết cho đoạn chữ đang chọn"
            value={linkDraft}
            disabled={off}
            // Same frame as every other control, 34px to sit in the toolbar band.
            className={cn(controlClass, "h-[34px] pl-8 text-sm")}
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
        </div>
        <div className="cum-nut flex flex-wrap gap-1">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="min-h-0"
            icon={<LinkIcon aria-hidden="true" focusable="false" />}
            disabled={off}
            onClick={applyLink}
          >
            Gắn liên kết
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="min-h-0"
            icon={<Unlink aria-hidden="true" focusable="false" />}
            disabled={off || !state.link}
            onClick={() => editor.chain().focus().extendMarkRange("link").unsetLink().run()}
          >
            Gỡ liên kết
          </Button>
        </div>
        {linkError !== null && (
          <p className="thong-bao-loi m-0 basis-full text-[13px]" role="alert">
            {linkError}
          </p>
        )}
      </div>

      <EditorContent editor={editor} />
    </div>
  );
}

/** 36px square, pressed = brand tint. No legacy class: `.rich-text-toolbar .nut-phu` would force 44px. */
const TOOL_CLASS = cn(
  "inline-grid size-9 cursor-pointer place-items-center rounded-lg border border-transparent bg-transparent p-0 text-ink-700",
  "transition-[background-color,color] duration-150",
  "hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
  "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
  "aria-pressed:border-brand-100 aria-pressed:bg-brand-50 aria-pressed:text-brand-700",
  "disabled:cursor-not-allowed disabled:opacity-40",
);

function ToolSeparator() {
  return <span aria-hidden="true" className="mx-1 h-5 w-px shrink-0 bg-line" />;
}
