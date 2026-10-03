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
  countFigures,
  isHttpsLink,
  LINK_REL,
  richTextExtensions,
} from "./rich-text";

/**
 * The editor can produce EXACTLY the server's STAFF allow-list (ADR 0067 §1, widened §Sửa đổi
 * 03/10/2026): p, br, strong, em, ul, ol, li, h2, h3, a[href https], plus
 * `figure > img[data-file-id, alt] + figcaption`, `blockquote > p` and `p[data-role=byline]`. These tests
 * ask the schema, the commands, a stored body and a paste — the four ways markup gets into the box — and
 * read every element and attribute that comes out.
 */

/** The server's staff list (`service-comms/internal/richtext/richtext.go`, `SanitizeStaff`). */
const SERVER_ELEMENTS = new Set([
  "p",
  "br",
  "strong",
  "em",
  "ul",
  "ol",
  "li",
  "h2",
  "h3",
  "a",
  "figure",
  "img",
  "figcaption",
  "blockquote",
]);

/** Two ULIDs in the server's shape (`fileIDShape`: Crockford base32, upper case). */
const FILE_A = "01JBDYXMG00000000000000001";
const FILE_B = "01JBDYXMG00000000000000002";

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
    // The only attributes ever printed: an https href and the server's fixed rel on `a`; a ULID file id
    // and an alt on `img`; `data-role=byline` on `p`. NEVER `src`, `style`, `class` or a handler.
    expect(
      /^a\[(href=https:\/\/.+|rel=noopener noreferrer nofollow)\]$/i.test(a) ||
        /^img\[data-file-id=[0-9A-HJKMNP-TV-Z]{26}\]$/.test(a) ||
        /^img\[alt=.*\]$/.test(a) ||
        a === "p[data-role=byline]",
      a,
    ).toBe(true);
  }
  // An img only ever stands inside a figure.
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, "text/html");
  for (const img of Array.from(doc.body.querySelectorAll("img"))) {
    expect(img.parentElement?.tagName.toLowerCase(), html).toBe("figure");
  }
}

const HOSTILE = [
  "<h1>Một</h1><h4>Bốn</h4>",
  '<p style="color:red" class="x" onclick="alert(1)">màu</p>',
  '<img src="https://x.vn/a.png" onerror="alert(1)">',
  "<script>alert(1)</script><style>p{}</style><iframe src='https://x.vn'></iframe>",
  "<table><tr><td>ô</td></tr></table>",
  "<blockquote>trích</blockquote><pre><code>mã</code></pre><hr>",
  // Images: a loose one, one with a src AND an id, a figure with no valid id, a lowercase id.
  '<p>trước<img src="https://x.vn/b.png">sau</p>',
  '<figure style="x" class="c"><img src="https://x.vn/c.png" data-file-id="01JBDYXMG00000000000000002" alt=" cổng " onerror="alert(4)" style="w"><figcaption style="s" onclick="alert(5)">chú</figcaption><p>thừa</p></figure>',
  '<figure><img src="https://x.vn/d.png"><figcaption>không mã</figcaption></figure>',
  '<figure><img data-file-id="01jbdyxmg00000000000000003"></figure>',
  '<p data-role="tac-gia" class="x">vai lạ</p>',
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

  it("no command exists for URL images, code, tables, rules, underline or strike", () => {
    const commands = Object.keys(editorWith().commands);
    for (const banned of [
      "setImage",
      "toggleCode",
      "toggleCodeBlock",
      "setCodeBlock",
      "insertTable",
      "setHorizontalRule",
      "toggleUnderline",
      "toggleStrike",
      "setColor",
    ]) {
      expect(commands, banned).not.toContain(banned);
    }
  });

  it("the three new shapes have exactly one way in each: insertFigure (a file id), toggleBlockquote, toggleByline", () => {
    const commands = Object.keys(editorWith().commands);
    for (const c of ["insertFigure", "toggleBlockquote", "toggleByline"]) expect(commands, c).toContain(c);
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
        `<p><a href="https://xa.gov.vn/tin">liên kết</a></p>` +
        `<figure><img data-file-id="${FILE_A}" alt="Hội nghị"><figcaption>Ảnh: <em>xã</em></figcaption></figure>` +
        "<blockquote><p>trích</p></blockquote>",
    );
    const html = e.getHTML();
    expectOnlyServerMarkup(html);
    for (const tag of SERVER_ELEMENTS) expect(inventory(html).tags, tag).toContain(tag);
    expect(links(html)).toEqual([{ text: "liên kết", attrs: SAFE_LINK("https://xa.gov.vn/tin") }]);
  });
});

describe("figure, quote and byline print EXACTLY the stored contract", () => {
  it("each shape round-trips byte for byte", () => {
    for (const html of [
      `<figure><img data-file-id="${FILE_A}" alt="Hội nghị"><figcaption>Ảnh: <strong>UBND</strong> xã</figcaption></figure>`,
      `<figure><img data-file-id="${FILE_A}"></figure><p>sau</p>`,
      "<blockquote><p>một</p><p><em>hai</em></p></blockquote>",
      '<p>Thân.</p><p data-role="byline">Theo <a rel="noopener noreferrer nofollow" href="https://xa.gov.vn">UBND xã</a></p>',
    ]) {
      expect(editorWith(html).getHTML()).toBe(html);
    }
  });

  it("a stored figure keeps its id and alt and NOTHING else — no src, no style, no handler, no class", () => {
    const html = editorWith(
      `<figure class="c"><img src="https://x.vn/a.png" data-file-id="${FILE_B}" alt=" cổng " style="w" onerror="x"><figcaption onclick="y">chú</figcaption></figure>`,
    ).getHTML();
    expect(html).toBe(`<figure><img data-file-id="${FILE_B}" alt="cổng"><figcaption>chú</figcaption></figure>`);
  });

  it("an img outside a figure, or a figure without a valid id, loses the image and keeps the words", () => {
    const html = editorWith(
      '<p>trước<img src="https://x.vn/b.png">sau</p><figure><img src="https://x.vn/d.png"><figcaption>chữ</figcaption></figure>' +
        '<figure><img data-file-id="01jbdyxmg00000000000000003"></figure>',
    ).getHTML();
    expect(html).not.toContain("<img");
    expect(html).not.toContain("<figure");
    expect(html).toContain("trước");
    expect(html).toContain("chữ");
  });

  it("insertFigure prints a file id, never a src; refuses an id that is not a ULID", () => {
    const e = editorWith("<p>a</p>");
    e.commands.focus("end");
    expect(e.commands.insertFigure({ fileId: "https://x.vn/a.png" })).toBe(false);
    expect(e.commands.insertFigure({ fileId: "01jbdyxmg00000000000000003" })).toBe(false);
    expect(e.commands.insertFigure({ fileId: FILE_A, caption: " Ảnh hội nghị " })).toBe(true);
    const html = e.getHTML();
    expect(html).toContain(`<figure><img data-file-id="${FILE_A}"><figcaption>Ảnh hội nghị</figcaption></figure>`);
    expect(html).not.toMatch(/\ssrc=/);
    expectOnlyServerMarkup(html);
  });

  it("a figure inserted last leaves a paragraph after it — somewhere to keep typing", () => {
    const e = editorWith("<p>a</p>");
    e.commands.focus("end");
    e.commands.insertFigure({ fileId: FILE_A });
    expect(e.state.doc.lastChild?.type.name).toBe("paragraph");
  });

  it("a figure is never put inside a list item — the server keeps figures at top level only", () => {
    const e = editorWith("<ul><li><p>mục</p></li></ul>");
    e.commands.focus("end");
    e.commands.insertFigure({ fileId: FILE_A });
    const doc = new DOMParser().parseFromString(`<body>${e.getHTML()}</body>`, "text/html");
    expect(doc.body.querySelector("li figure, li img")).toBeNull();
  });

  it("Enter in a caption leaves the figure for a new paragraph — it never splits one image into two figures", () => {
    const e = editorWith(`<figure><img data-file-id="${FILE_A}"><figcaption>chú</figcaption></figure><p>x</p>`);
    e.commands.setTextSelection(5); // inside the caption (figure 0, figcaption 1, text 2..5), after "chú"
    e.view.someProp("handleKeyDown", (f) => f(e.view, new KeyboardEvent("keydown", { key: "Enter" })));
    expect(countFigures(e.state.doc)).toBe(1);
    expect(e.getHTML()).toBe(`<figure><img data-file-id="${FILE_A}"><figcaption>chú</figcaption></figure><p></p><p>x</p>`);
  });

  it("no block command run with the cursor in a caption can turn the figure into text (the image would be lost)", () => {
    const stored = `<figure><img data-file-id="${FILE_A}"><figcaption>chú</figcaption></figure><p>x</p>`;
    for (const run of [
      (e: Editor) => e.commands.setParagraph(),
      (e: Editor) => e.commands.toggleHeading({ level: 2 }),
      (e: Editor) => e.commands.toggleByline(),
      (e: Editor) => e.commands.toggleBlockquote(),
      (e: Editor) => e.commands.toggleBulletList(),
    ]) {
      const e = editorWith(stored);
      e.commands.setTextSelection(3);
      run(e);
      expect(countFigures(e.state.doc), run.toString()).toBe(1);
      expect(e.getHTML(), run.toString()).toContain(`<figure><img data-file-id="${FILE_A}"><figcaption>chú</figcaption></figure>`);
    }
  });

  it("a stray figcaption (no figure around it) is just text — it never conjures an image without a file", () => {
    const html = editorWith("<figcaption>lạc</figcaption><p>x</p>").getHTML();
    expect(html).not.toContain("<figure");
    expect(html).toContain("lạc");
  });

  it("toggleBlockquote wraps paragraphs; a heading is never quoted (the server keeps paragraphs only)", () => {
    const e = editorWith("<p>câu</p>");
    e.commands.setTextSelection(2);
    expect(e.commands.toggleBlockquote()).toBe(true);
    expect(e.getHTML()).toBe("<blockquote><p>câu</p></blockquote>");
    expect(e.isActive("blockquote")).toBe(true);
    expect(e.commands.toggleBlockquote()).toBe(true);
    expect(e.getHTML()).toBe("<p>câu</p>");
    const h = editorWith("<h2>tiêu đề</h2>");
    h.commands.selectAll();
    h.commands.toggleBlockquote();
    expect(h.getHTML()).not.toContain("<blockquote><h2>");
  });

  it("toggleByline turns the paragraph into the author line and back", () => {
    const e = editorWith("<p>Theo Báo X</p>");
    e.commands.focus("end");
    expect(e.commands.toggleByline()).toBe(true);
    expect(e.getHTML()).toBe('<p data-role="byline">Theo Báo X</p>');
    e.commands.toggleByline();
    expect(e.getHTML()).toBe("<p>Theo Báo X</p>");
  });

  it("countFigures counts the body images", () => {
    expect(countFigures(editorWith("<p>a</p>").state.doc)).toBe(0);
    const two = `<figure><img data-file-id="${FILE_A}"></figure><p>b</p><figure><img data-file-id="${FILE_B}"></figure>`;
    expect(countFigures(editorWith(two).state.doc)).toBe(2);
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
    expect(html).not.toMatch(/\ssrc=/);
    for (const kept of ["Một", "Bốn", "màu", "ô", "trích", "mã", "gạch", "xoá", "js", "http", "ok", "ba", "chú", "không mã", "vai lạ"]) {
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

  it("a pasted `<img src>` from another site does not survive — only a figure naming a file id would", () => {
    const e = editorWith("<p></p>");
    e.commands.focus("end");
    e.view.pasteHTML('<p>ảnh: <img src="https://bao.vn/anh.jpg" alt="x"></p><img src="data:image/png;base64,AAAA">');
    const html = e.getHTML();
    expect(html).not.toContain("<img");
    expect(html).not.toMatch(/\ssrc=/);
    expect(html).toContain("ảnh:");
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

  it("a body holding only an image (no caption, no text) is NOT empty — Lưu must not drop it", () => {
    const e = editorWith("");
    e.commands.insertFigure({ fileId: FILE_A });
    expect(bodyFromEditor(e)).toBe(`<figure><img data-file-id="${FILE_A}"></figure><p></p>`);
  });
});
