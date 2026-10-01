// @vitest-environment jsdom
//
// jsdom FOR THIS FILE ONLY (vitest.config.mts keeps Node for the rest): the editor is the one thing on
// this screen that cannot be checked without a DOM — ProseMirror parses HTML through the browser's own
// DOMParser, and paste handling lives on the EditorView.

import { Editor, getSchema } from "@tiptap/core";
import { afterEach, describe, expect, it } from "vitest";

import {
  ALLOWED_MARKS,
  ALLOWED_NODES,
  bodyFromEditor,
  isHttpsLink,
  LINK_REL,
  richTextExtensions,
} from "./rich-text";

/**
 * The editor can produce EXACTLY the server's allow-list (ADR 0067 §1): p, br, strong, em, ul, ol, li,
 * h2, h3, a[href https]. These tests ask the schema, the commands, a stored body and a paste — the four
 * ways markup gets into the box — and read every element and attribute that comes out.
 */

/** The server's list (`service-comms/internal/richtext/richtext.go`, `allowList`). */
const SERVER_ELEMENTS = new Set(["p", "br", "strong", "em", "ul", "ol", "li", "h2", "h3", "a"]);

const editors: Editor[] = [];

// jsdom has no ClipboardEvent; ProseMirror's `pasteHTML` / `pasteText` only need one to hand to
// `handlePaste`. A bare Event with an empty `clipboardData` is what a paste without data looks like.
if (typeof globalThis.ClipboardEvent === "undefined") {
  class ClipboardEventStub extends Event {
    readonly clipboardData = null;
  }
  (globalThis as unknown as { ClipboardEvent: typeof ClipboardEventStub }).ClipboardEvent =
    ClipboardEventStub;
}

/** The links of an HTML string as `{text, attrs}`, attributes sorted — their ORDER is not a property. */
function links(html: string): { text: string; attrs: string[] }[] {
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  return Array.from(doc.body.querySelectorAll("a")).map((a) => ({
    text: a.textContent ?? "",
    attrs: Array.from(a.attributes)
      .map((x) => `${x.name}=${x.value}`)
      .sort(),
  }));
}

const SAFE_LINK = (href: string) => [`href=${href}`, `rel=${LINK_REL}`];

function editorWith(content = ""): Editor {
  const e = new Editor({ extensions: richTextExtensions(), content });
  editors.push(e);
  return e;
}

afterEach(() => {
  while (editors.length > 0) editors.pop()?.destroy();
});

/** Every element and attribute in an HTML string, read by the browser parser (not a regex). */
function inventory(html: string): { tags: string[]; attrs: string[] } {
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  const tags: string[] = [];
  const attrs: string[] = [];
  for (const el of Array.from(doc.body.querySelectorAll("*"))) {
    const tag = el.tagName.toLowerCase();
    tags.push(tag);
    for (const a of Array.from(el.attributes)) attrs.push(`${tag}[${a.name}=${a.value}]`);
  }
  return { tags, attrs };
}

function expectOnlyServerMarkup(html: string): void {
  const { tags, attrs } = inventory(html);
  for (const t of tags) expect(SERVER_ELEMENTS.has(t), `element <${t}> in ${html}`).toBe(true);
  for (const a of attrs) {
    // The only attributes ever printed: an https href and the server's fixed rel, on `a`.
    expect(/^a\[(href=https:\/\/.+|rel=noopener noreferrer nofollow)\]$/i.test(a), a).toBe(true);
  }
}

const HOSTILE = [
  "<h1>Một</h1><h4>Bốn</h4>",
  '<p style="color:red" class="x" onclick="alert(1)">màu</p>',
  '<img src="https://x.vn/a.png" onerror="alert(1)">',
  "<script>alert(1)</script><style>p{}</style><iframe src='https://x.vn'></iframe>",
  "<table><tr><td>ô</td></tr></table>",
  "<blockquote>trích</blockquote><pre><code>mã</code></pre><hr>",
  "<p><u>gạch</u> <s>xoá</s> <code>c</code> <sub>1</sub> <span style='font-weight:normal'>s</span></p>",
  '<p><a href="javascript:alert(1)">js</a> <a href="http://x.vn">http</a> <a href="//x.vn">rel</a></p>',
  '<p><a href="https://xa.gov.vn" target="_blank" class="c" title="t" onclick="x">ok</a></p>',
  '<ol start="3" type="a"><li><p>ba</p></li></ol>',
].join("");

describe("schema = the server's allow-list", () => {
  it("nodes and marks are exactly the pinned lists", () => {
    const schema = getSchema(richTextExtensions());
    expect(Object.keys(schema.nodes).sort()).toEqual([...ALLOWED_NODES].sort());
    expect(Object.keys(schema.marks).sort()).toEqual([...ALLOWED_MARKS].sort());
  });

  it("no command exists for images, code, quotes, tables, rules, underline or strike", () => {
    const commands = Object.keys(editorWith().commands);
    for (const banned of [
      "setImage",
      "toggleCode",
      "toggleCodeBlock",
      "setCodeBlock",
      "toggleBlockquote",
      "setBlockquote",
      "insertTable",
      "setHorizontalRule",
      "toggleUnderline",
      "toggleStrike",
      "setColor",
    ]) {
      expect(commands, banned).not.toContain(banned);
    }
  });

  it("heading levels 1 and 4 cannot be made; 2 and 3 can", () => {
    const e = editorWith("<p>x</p>");
    expect(e.commands.toggleHeading({ level: 1 })).toBe(false);
    expect(e.commands.toggleHeading({ level: 4 })).toBe(false);
    expect(e.commands.toggleHeading({ level: 2 })).toBe(true);
    expect(e.getHTML()).toBe("<h2>x</h2>");
    expect(e.commands.setHeading({ level: 3 })).toBe(true);
    expect(e.getHTML()).toBe("<h3>x</h3>");
  });

  it("every allowed format round-trips to the server's elements", () => {
    const e = editorWith(
      "<h2>Lớn</h2><h3>Nhỏ</h3><p><strong>đậm</strong> <em>nghiêng</em><br>dòng</p>" +
        "<ul><li><p>a</p></li></ul><ol><li><p>b</p></li></ol>" +
        `<p><a href="https://xa.gov.vn/tin">liên kết</a></p>`,
    );
    const html = e.getHTML();
    expectOnlyServerMarkup(html);
    for (const tag of SERVER_ELEMENTS) expect(inventory(html).tags, tag).toContain(tag);
    expect(links(html)).toEqual([{ text: "liên kết", attrs: SAFE_LINK("https://xa.gov.vn/tin") }]);
  });
});

describe("a stored body with markup outside the list", () => {
  it("prints back ONLY the server's elements and attributes", () => {
    expectOnlyServerMarkup(editorWith(HOSTILE).getHTML());
  });

  it("drops script/style content, keeps the text of what it unwraps", () => {
    const html = editorWith(HOSTILE).getHTML();
    expect(html).not.toContain("alert");
    expect(html).not.toContain("p{}");
    for (const kept of ["Một", "Bốn", "màu", "ô", "trích", "mã", "gạch", "xoá", "js", "http", "ok", "ba"]) {
      expect(html, kept).toContain(kept);
    }
  });

  it("a non-https link loses its tag and keeps its text; an https one keeps only href + rel", () => {
    const html = editorWith(HOSTILE).getHTML();
    // Exactly one link survives — the https one — and it carries href + rel, nothing else.
    expect(links(html)).toEqual([{ text: "ok", attrs: SAFE_LINK("https://xa.gov.vn") }]);
  });

  it("an ordered list keeps no start — residents would see it numbered from 1", () => {
    expect(editorWith('<ol start="3"><li><p>ba</p></li></ol>').getHTML()).toBe("<ol><li><p>ba</p></li></ol>");
  });
});

describe("paste", () => {
  it("pasted HTML is reduced to the list — the same parser, the same result", () => {
    const e = editorWith("<p></p>");
    e.commands.focus("end");
    expect(e.view.pasteHTML(HOSTILE)).toBe(true);
    const html = e.getHTML();
    expectOnlyServerMarkup(html);
    expect(html).not.toContain("alert");
    expect(html).toContain("màu");
  });

  it("a pasted URL does not become a link by itself", () => {
    const e = editorWith("<p></p>");
    e.commands.focus("end");
    e.view.pasteText("https://xa.gov.vn/tin");
    expect(e.getHTML()).not.toContain("<a");
  });
});

describe("links are https only", () => {
  it("`isHttpsLink`", () => {
    expect(isHttpsLink("https://xa.gov.vn")).toBe(true);
    expect(isHttpsLink(" HTTPS://xa.gov.vn/a?b#c ")).toBe(true);
    for (const bad of [
      "",
      "https://",
      "http://xa.gov.vn",
      "javascript:alert(1)",
      "//xa.gov.vn",
      "/tin-tuc",
      "https://gov.vn@evil.example",
      "https://xa.gov.vn/a b",
      "data:text/html,x",
      null,
      undefined,
    ]) {
      expect(isHttpsLink(bad), String(bad)).toBe(false);
    }
  });

  it("`setLink` refuses anything but https", () => {
    const e = editorWith("<p>chữ</p>");
    e.commands.selectAll();
    expect(e.commands.setLink({ href: "javascript:alert(1)" })).toBe(false);
    expect(e.commands.setLink({ href: "http://xa.gov.vn" })).toBe(false);
    expect(e.getHTML()).not.toContain("<a");
    expect(e.commands.setLink({ href: "https://xa.gov.vn" })).toBe(true);
    expect(links(e.getHTML())).toEqual([{ text: "chữ", attrs: SAFE_LINK("https://xa.gov.vn") }]);
  });
});

describe("body sent", () => {
  it("an empty editor is `\"\"`, not `<p></p>`", () => {
    expect(bodyFromEditor(editorWith(""))).toBe("");
    expect(bodyFromEditor(editorWith("<p>a</p>"))).toBe("<p>a</p>");
  });
});
