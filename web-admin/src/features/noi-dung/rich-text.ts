/**
 * The article body editor's schema — EXACTLY the server's allow-list (ADR 0067 §1 decision 1 and 6).
 *
 *   server (service-comms/internal/richtext/richtext.go)   here
 *   p                                                       Paragraph
 *   br                                                      HardBreak
 *   strong / em                                             Bold / Italic
 *   ul / ol / li                                            BulletList / OrderedList / ListItem
 *   h2 / h3                                                 Heading, levels [2, 3]
 *   a, href https only, rel fixed                           Link, isAllowedUri = isHttpsLink
 *
 * WHY THE SCHEMA AND NOT A FILTER: ProseMirror parses every input — the stored body, a paste, a drop —
 * through the schema's own parse rules, and anything the schema has no node or mark for is not
 * representable. So an `<img>`, a `<table>`, a `<script>` or a `style` attribute pasted from Word does
 * not survive the paste, and `getHTML()` can only print the elements above. No string sanitiser is
 * written on this side (the repository has one sanitiser, on the server, and a second one written by hand
 * is how sanitisers get written wrongly).
 *
 * THE SERVER IS STILL THE WALL. It sanitises every write path; this schema only makes sure the officer
 * sees, while typing, what will be stored — instead of formatting that silently disappears on Lưu.
 *
 * Widening this list (images, tables, another link scheme) is the ADR's stop condition #1 — the
 * customer's call, and the server's list must move first.
 */

import type { Extensions } from "@tiptap/core";
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

/** Heading levels the server keeps (`h2`, `h3`). An `h1` pasted in is read as a paragraph. */
export const HEADING_LEVELS = [2, 3] as const;

/** The `rel` the server writes on every surviving link (`richtext.LinkRel`). Same here, so a stored
 * body opened again prints what it was stored as. */
export const LINK_REL = "noopener noreferrer nofollow";

/** Nodes of the schema, pinned by `rich-text.test.ts`. */
export const ALLOWED_NODES = [
  "bulletList",
  "doc",
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

/**
 * The extension list. A FUNCTION, not a constant: Tiptap extensions carry per-editor storage, so two
 * editors (the create form and the edit form) must not share instances.
 */
export function richTextExtensions(): Extensions {
  return [
    Document,
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
    UndoRedo,
  ];
}

/**
 * The editor's document as the `body` to send. An empty editor prints `<p></p>`; that is "no body",
 * sent as `""` so the PATCH comparison against a stored `""` sees no change.
 */
export function bodyFromEditor(editor: { isEmpty: boolean; getHTML: () => string }): string {
  return editor.isEmpty ? "" : editor.getHTML();
}
