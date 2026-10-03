/**
 * The article body editor's schema — EXACTLY the server's STAFF allow-list (ADR 0067 §1 decisions 1 and 6,
 * widened by §Sửa đổi 03/10/2026, H1–H10 / K1–K8; `richtext.SanitizeStaff`).
 *
 *   server (service-comms/internal/richtext)                here
 *   p                                                       Paragraph
 *   br                                                      HardBreak
 *   strong / em                                             Bold / Italic
 *   ul / ol / li                                            BulletList / OrderedList / ListItem
 *   h2 / h3                                                 Heading, levels [2, 3]
 *   a, href https only, rel fixed                           Link, isAllowedUri = isHttpsLink
 *   figure > img[data-file-id, alt] + figcaption (inline)   Figure (top level, attrs) > Figcaption
 *   blockquote > p (inline)                                 Blockquote (top level, paragraphs only)
 *   p[data-role="byline"] (inline)                          Byline (top level)
 *
 * WHY THE SCHEMA AND NOT A FILTER: ProseMirror parses every input — the stored body, a paste, a drop —
 * through the schema's own parse rules, and anything the schema has no node or mark for is not
 * representable. So an `<img src>`, a `<table>`, a `<script>` or a `style` attribute pasted from Word does
 * not survive the paste, and `getHTML()` can only print the elements above. No string sanitiser is
 * written on this side (the repository has one sanitiser, on the server, and a second one written by hand
 * is how sanitisers get written wrongly).
 *
 * AN IMAGE IS A FILE ID, NEVER A URL (K2). The figure reads `data-file-id` (a ULID) and `alt` and nothing
 * else, and prints only those: an `<img src=…>` from anywhere is not representable, and an `<img>` outside
 * a figure is dropped. The editor SHOWS the image through a NodeView (`body-image-view.tsx`) that looks the
 * id up in the form's preview map — the `src` lives in that view's DOM, never in the document, so it can
 * never reach `getHTML()`.
 *
 * WHERE THE NEW BLOCKS MAY STAND mirrors the server's `enforceStaffShapes`: figure, blockquote and byline
 * are TOP LEVEL only (group `topBlock`, which only the document accepts — a list item's `block*` does
 * not), a quote holds paragraphs only.
 *
 * THE SERVER IS STILL THE WALL. It sanitises every write path; this schema only makes sure the officer
 * sees, while typing, what will be stored — instead of formatting that silently disappears on Lưu.
 *
 * Widening this list further (tables, video, another link scheme) is the ADR's stop condition #1 — the
 * customer's call, and the server's list must move first.
 */

import { Node, type Extensions, type NodeViewRenderer } from "@tiptap/core";
import Bold from "@tiptap/extension-bold";
import Document from "@tiptap/extension-document";
import HardBreak from "@tiptap/extension-hard-break";
import Heading from "@tiptap/extension-heading";
import Italic from "@tiptap/extension-italic";
import Link from "@tiptap/extension-link";
import { BulletList, ListItem, OrderedList } from "@tiptap/extension-list";
import Paragraph from "@tiptap/extension-paragraph";
import Text from "@tiptap/extension-text";
import { UndoRedo } from "@tiptap/extensions";
import type { Node as PMNode } from "@tiptap/pm/model";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Mapping } from "@tiptap/pm/transform";

/** Heading levels the server keeps (`h2`, `h3`). An `h1` pasted in is read as a paragraph. */
export const HEADING_LEVELS = [2, 3] as const;

/** The `rel` the server writes on every surviving link (`richtext.LinkRel`). Same here, so a stored
 * body opened again prints what it was stored as. */
export const LINK_REL = "noopener noreferrer nofollow";

/** Nodes of the schema, pinned by `rich-text.test.ts`. */
export const ALLOWED_NODES = [
  "blockquote",
  "bulletList",
  "byline",
  "doc",
  "figcaption",
  "figure",
  "hardBreak",
  "heading",
  "listItem",
  "orderedList",
  "paragraph",
  "text",
] as const;

/** Marks of the schema, pinned by `rich-text.test.ts`. */
export const ALLOWED_MARKS = ["bold", "italic", "link"] as const;

const HTTPS_PREFIX = "https://";

/**
 * Whether a link may be set: `https://` followed by a host, no whitespace, no userinfo.
 *
 * MIRRORS the server's last check (`isHTTPS` in richtext.go: case-insensitive `https://` prefix) and is
 * a little stricter: a host must be there, and `https://gov.vn@evil.example` — which reads as a
 * government link and goes somewhere else — is refused. The server would keep that one; refusing it at
 * the keyboard costs the officer nothing.
 */
export function isHttpsLink(raw: string | undefined | null): boolean {
  if (raw === undefined || raw === null) return false;
  const url = raw.trim();
  if (url.length <= HTTPS_PREFIX.length) return false;
  if (url.slice(0, HTTPS_PREFIX.length).toLowerCase() !== HTTPS_PREFIX) return false;
  if (/[\s\u0000-\u001f\u007f-\u009f\\]/.test(url)) return false;
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  return (
    parsed.protocol === "https:" &&
    parsed.hostname !== "" &&
    parsed.username === "" &&
    parsed.password === ""
  );
}

/**
 * `<ol start="3" type="a">` would be stored without those attributes (the server keeps `ol` bare), so
 * the editor does not hold them either: what the officer sees numbered from 3 would reach residents
 * numbered from 1. ProseMirror ignores attributes a node type does not declare, so the `1. ` input rule
 * still works — it just cannot carry a start.
 */
const BareOrderedList = OrderedList.extend({
  addAttributes() {
    return {};
  },
});

/**
 * A link holds `href` and nothing else; `rel` is always `LINK_REL`. Tiptap's Link also reads `target`,
 * `class` and `title` from whatever is pasted, and the server drops all three — so the editor would show
 * a link that is not the one stored. Declaring only these two attributes makes the others unparseable.
 */
const HttpsLink = Link.extend({
  addAttributes() {
    return {
      href: { default: null, parseHTML: (el: HTMLElement) => el.getAttribute("href") },
      rel: { default: LINK_REL, parseHTML: () => LINK_REL, renderHTML: () => ({ rel: LINK_REL }) },
    };
  },
  // Tiptap turns a pasted bare URL into a link through a paste RULE that `linkOnPaste: false` does not
  // switch off. Links come from the toolbar box only (see `autolink` below).
  addPasteRules() {
    return [];
  },
});

/** The server's `fileIDShape`: a ULID in Crockford base32, upper case (no I, L, O, U). */
export const FILE_ID_SHAPE = /^[0-9A-HJKMNP-TV-Z]{26}$/;

/** The `data-role` value of the author/source line — the only one the server keeps. */
export const BYLINE_ROLE = "byline";

/** The group of the blocks the server keeps at TOP LEVEL only. Only the document accepts it. */
const TOP_BLOCK = "topBlock";

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    figure: {
      /**
       * Insert a READY body image at the cursor. Refuses an id that is not a ULID. `alt` absent = the
       * caption's text (see `figureAltFollowsCaption`).
       */
      insertFigure: (attrs: { fileId: string; alt?: string; caption?: string }) => ReturnType;
    };
    blockquote: {
      toggleBlockquote: () => ReturnType;
    };
    byline: {
      toggleByline: () => ReturnType;
    };
  }
}

/** The document takes the top-level-only blocks too; a list item (`paragraph block*`) does not. */
const TopLevelDocument = Document.extend({
  content: `(block | ${TOP_BLOCK})+`,
});

/** The `<img data-file-id>` a stored figure names, read the way `buildFigure` reads it (direct child). */
function figureImage(el: HTMLElement): HTMLElement | null {
  for (const c of Array.from(el.children)) {
    if (c.tagName.toLowerCase() === "img" && FILE_ID_SHAPE.test(c.getAttribute("data-file-id") ?? "")) {
      return c as HTMLElement;
    }
  }
  return null;
}

/**
 * What a stored figure's CONTENT is read from: a detached box holding a copy of its FIRST figcaption (an
 * empty one when there is none) and nothing else — the server's `buildFigure` keeps exactly that, so a
 * paragraph or a second caption inside a figure must not come back into the editor either. Built with
 * DOM calls, never from a string (rule 13).
 */
function figureContent(el: HTMLElement): HTMLElement {
  const box = el.ownerDocument.createElement("div");
  let caption: Element | null = null;
  for (const c of Array.from(el.children)) {
    if (c.tagName.toLowerCase() === "figcaption") {
      caption = c;
      break;
    }
  }
  box.appendChild(caption === null ? el.ownerDocument.createElement("figcaption") : caption.cloneNode(true));
  return box;
}

/**
 * `<figure><img data-file-id alt><figcaption>…</figcaption></figure>` — one body image.
 *
 * TWO NODES, NOT ONE: `figure` (the image: attributes only, not a textblock) holding exactly one
 * `figcaption` (the caption: inline content, so bold/italic/link like the server's). The caption cannot be
 * a figure's own text: then `Đoạn văn`, `Tiêu đề`, `Dòng tác giả/nguồn` — or typing `## ` — with the cursor
 * in a caption would turn the FIGURE into a paragraph and drop the image. A figcaption can only live in a
 * figure, so none of them applies there.
 *
 * ATTRIBUTES ARE `rendered: false` and printed by `renderHTML` alone, so nothing else (no `src`, no
 * `class`, no `style`) can reach the output through an attribute default. An EMPTY caption prints no
 * figcaption at all, as the server stores it (the figure's spec then has no content hole).
 *
 * `view` is the NodeView that DRAWS the image (`body-image-view.tsx`). Optional so the schema can be built
 * without React — the tests run it bare, and a bare editor draws `toDOM`, which has no `src` at all.
 */
const Figure = Node.create<{ view: NodeViewRenderer | null }>({
  name: "figure",
  group: TOP_BLOCK,
  content: "figcaption",
  isolating: true,
  draggable: true,
  selectable: true,

  addOptions() {
    return { view: null };
  },

  addAttributes() {
    return {
      fileId: { default: null, rendered: false },
      alt: { default: "", rendered: false },
    };
  },

  parseHTML() {
    return [
      {
        tag: "figure",
        // No valid img → NOT a figure: ProseMirror then reads its children as ordinary content, so the
        // caption's words stay and the image (unrepresentable) goes — the server drops such a figure too.
        getAttrs: (el) => {
          const img = figureImage(el);
          if (img === null) return false;
          return { fileId: img.getAttribute("data-file-id"), alt: (img.getAttribute("alt") ?? "").trim() };
        },
        contentElement: (el) => figureContent(el as HTMLElement),
      },
    ];
  },

  // Built as DOM, not as an array spec: ProseMirror's array spec allows a content hole only as the sole
  // child, and here the caption follows the img. The figure element itself is the content holder, so the
  // figcaption is appended AFTER the img. `data-file-id` first, then `alt` — the server's order.
  renderHTML({ node }) {
    const fileId = String(node.attrs.fileId ?? "");
    const alt = String(node.attrs.alt ?? "").trim();
    const figure = document.createElement("figure");
    const img = document.createElement("img");
    img.setAttribute("data-file-id", fileId);
    if (alt !== "") img.setAttribute("alt", alt);
    figure.appendChild(img);
    // An empty caption: no content holder, so no `<figcaption></figcaption>` is printed.
    return (node.firstChild?.content.size ?? 0) === 0 ? { dom: figure } : { dom: figure, contentDOM: figure };
  },

  addNodeView() {
    return this.options.view;
  },

  addProseMirrorPlugins() {
    return [figureAltFollowsCaption()];
  },

  addCommands() {
    return {
      insertFigure:
        ({ fileId, alt, caption }) =>
        ({ chain }) => {
          if (!FILE_ID_SHAPE.test(fileId)) return false;
          const text = (caption ?? "").trim();
          return chain()
            .insertContent({
              type: this.name,
              // The caption IS the description a screen reader needs; none = decorative (`alt` omitted).
              // Never the file name: "IMG_0412.jpg" read aloud describes nothing, and can name a person.
              attrs: { fileId, alt: (alt ?? text).trim() },
              content: [{ type: "figcaption", content: text === "" ? [] : [{ type: "text", text }] }],
            })
            .command(({ tr, state }) => {
              // A figure as the LAST block leaves nowhere to type after it (no gap cursor here).
              if (tr.doc.lastChild?.type.name === this.name) {
                tr.insert(tr.doc.content.size, state.schema.nodes.paragraph!.create());
              }
              return true;
            })
            .run();
        },
    };
  },
});

/** A figure's caption as plain text, trimmed — what its `alt` is compared with and copied from. */
function captionText(figure: PMNode): string {
  return figure.textContent.trim();
}

/**
 * KEEPS A FIGURE'S `alt` IN STEP WITH ITS CAPTION while the two still say the same thing. An image gets
 * its caption as `alt` when inserted (`insertFigure`); when the caption is edited later, an `alt` that
 * still equals the OLD caption follows it (an empty caption gives `alt=""`: decorative). An `alt` that
 * differs — one written for a stored body — is the author's own and is never touched.
 *
 * Figures are top level only, so the document's own children are all there is to look at. Each old
 * figure is followed through the step maps to where it now stands; a deleted one is skipped.
 */
function figureAltFollowsCaption(): Plugin {
  return new Plugin({
    key: new PluginKey("figureAltFollowsCaption"),
    appendTransaction(transactions, oldState, newState) {
      if (!transactions.some((t) => t.docChanged)) return null;
      const mapping = new Mapping();
      for (const t of transactions) mapping.appendMapping(t.mapping);
      const tr = newState.tr;
      oldState.doc.forEach((before, pos) => {
        if (before.type.name !== "figure") return;
        const oldAlt = String(before.attrs.alt ?? "");
        if (oldAlt !== captionText(before)) return;
        const mapped = mapping.mapResult(pos, 1);
        if (mapped.deleted) return;
        const after = newState.doc.nodeAt(mapped.pos);
        if (after === null || after.type.name !== "figure" || after.attrs.fileId !== before.attrs.fileId) return;
        const next = captionText(after);
        if (String(after.attrs.alt ?? "") === next) return;
        tr.setNodeMarkup(mapped.pos, undefined, { ...after.attrs, alt: next });
      });
      return tr.docChanged ? tr : null;
    },
  });
}

/** The caption of a figure. Only ever parsed INSIDE a figure (`context`): a stray figcaption is text. */
const Figcaption = Node.create({
  name: "figcaption",
  content: "inline*",

  parseHTML() {
    return [{ tag: "figcaption", context: "figure/" }];
  },

  renderHTML() {
    return ["figcaption", 0];
  },

  addKeyboardShortcuts() {
    return {
      // Enter in a caption leaves the figure for a new paragraph below it.
      Enter: ({ editor }) => {
        const { $from } = editor.state.selection;
        if ($from.parent.type.name !== this.name) return false;
        const after = $from.after($from.depth - 1);
        return editor.chain().insertContentAt(after, { type: "paragraph" }).setTextSelection(after + 1).run();
      },
    };
  },
});

/** `<blockquote><p>…</p>…</blockquote>` — a quote of paragraphs, top level only. */
const Blockquote = Node.create({
  name: "blockquote",
  group: TOP_BLOCK,
  content: "paragraph+",
  defining: true,

  parseHTML() {
    return [{ tag: "blockquote" }];
  },

  renderHTML() {
    return ["blockquote", 0];
  },

  addCommands() {
    return {
      toggleBlockquote:
        () =>
        ({ commands }) =>
          commands.toggleWrap(this.name),
    };
  },
});

/**
 * `<p data-role="byline">…</p>` — the author/source line at the end (H4), free text. A node of its own
 * rather than a paragraph attribute: a plain paragraph then can never carry `data-role` from a paste, and
 * a byline can only stand at top level. Its look (right, italic) is the editor frame's CSS, never a class
 * in the document.
 */
const Byline = Node.create({
  name: "byline",
  group: TOP_BLOCK,
  content: "inline*",
  defining: true,

  parseHTML() {
    // Above the paragraph's rule (priority 50), which would otherwise take every `p`.
    return [{ tag: `p[data-role="${BYLINE_ROLE}"]`, priority: 60 }];
  },

  renderHTML() {
    return ["p", { "data-role": BYLINE_ROLE }, 0];
  },

  addCommands() {
    return {
      toggleByline:
        () =>
        ({ commands }) =>
          commands.toggleNode(this.name, "paragraph"),
    };
  },
});

/** How many body images a document holds — the 20-image cap counts these. */
export function countFigures(doc: PMNode): number {
  let n = 0;
  // Figures are top level only: the document's own children are all there is to count.
  doc.forEach((child) => {
    if (child.type.name === "figure") n++;
  });
  return n;
}

/**
 * The extension list. A FUNCTION, not a constant: Tiptap extensions carry per-editor storage, so two
 * editors (the create form and the edit form) must not share instances.
 *
 * `figureView`: the NodeView that draws a body image (the editor component passes it); absent = the
 * bare schema, drawn by `toDOM` without any `src`.
 */
export function richTextExtensions(opts: { figureView?: NodeViewRenderer } = {}): Extensions {
  return [
    TopLevelDocument,
    Paragraph,
    Text,
    HardBreak,
    Bold,
    Italic,
    Heading.configure({ levels: [...HEADING_LEVELS] }),
    BulletList,
    BareOrderedList,
    ListItem,
    HttpsLink.configure({
      // Links come from the toolbar's box, where the https check runs and says why it refused. Turning a
      // typed or pasted URL into a link behind the officer's back is a link nobody checked.
      autolink: false,
      linkOnPaste: false,
      openOnClick: false,
      protocols: [],
      defaultProtocol: "https",
      // Every input path — the stored body, a paste, a `setLink` — passes this. A `javascript:` or an
      // `http:` link pasted from elsewhere loses its tag and keeps its text, as on the server.
      isAllowedUri: (url) => isHttpsLink(url),
      // `configure` deep-merges into Tiptap's defaults (`target="_blank"`), so the two must be nulled
      // explicitly; `rel` comes from the attribute above, printed after `href` as the server prints it.
      HTMLAttributes: { target: null, class: null, rel: null },
    }),
    Figure.configure({ view: opts.figureView ?? null }),
    Figcaption,
    Blockquote,
    Byline,
    UndoRedo,
  ];
}

/**
 * The editor's document as the `body` to send. An empty editor prints `<p></p>`; that is "no body",
 * sent as `""` so the PATCH comparison against a stored `""` sees no change.
 */
export function bodyFromEditor(editor: { isEmpty: boolean; getHTML: () => string; state: { doc: PMNode } }): string {
  // Tiptap's `isEmpty` looks at TEXT: a body holding only an image with no caption reads as empty to it,
  // and sending "" would drop that image on Lưu.
  return editor.isEmpty && countFigures(editor.state.doc) === 0 ? "" : editor.getHTML();
}
