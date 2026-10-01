/**
 * AN ARTICLE'S BODY — `body_blocks` drawn as React elements, or the plain `body` when there are none.
 *
 * ⚠ NEVER HTML (rule 13 forbidden #3; ADR 0067 §1 decision 3). The server sends STRUCTURE — paragraph, heading,
 * list, runs of text with bold / italic / link — and every `text` here becomes a React text node, which escapes
 * every character. There is no markup to interpret, so a `<script>` that reached a stored body arrives as the
 * eight characters `<script>` at worst. Do not "simplify" this into an HTML string and one injection call:
 * `cong-khai.test.tsx` forbids it across the state half, and ADR 0067 stop condition #2 names it.
 *
 * READABLE BY AN AGEING EYE (`skills/accessibility-elderly`): body-size text everywhere, headings larger and
 * bold, real `<ul>`/`<ol>` so a screen reader announces "list, 3 items", and a link that says so twice — colour
 * AND underline, never colour alone — with a tap area grown to 44px by padding (`.xa-body-link`).
 *
 * HEADING LEVELS: the article title is the screen's `<h2>`, so the body's level 2 is drawn as `<h3>` and level 3
 * as `<h4>` — a screen reader's outline then reads the body as under the title, not beside it.
 */
import { type ReactNode } from "react";

import { type BodyBlock, type BodyRun, chiaDoan } from "../api/hop-dong-cong-khai";

/** `"\n"` inside a run's text is a line break: the text between, with a `<br>` between each. PURE. */
function withLineBreaks(text: string): ReactNode[] {
  const out: ReactNode[] = [];
  text.split("\n").forEach((line, i) => {
    if (i > 0) out.push(<br key={`br${i}`} />);
    if (line !== "") out.push(line);
  });
  return out;
}

/**
 * One run. A link becomes a tappable `<button>` ONLY with an `onLink` handler (the commune app, which has an
 * opener); without one its words are plain text — a styled word that does nothing on a tap reads as broken.
 */
function Run({ run, onLink }: { run: BodyRun; onLink?: (href: string) => void }) {
  let content: ReactNode = withLineBreaks(run.text);
  if (run.italic) content = <em>{content}</em>;
  if (run.bold) content = <strong>{content}</strong>;
  const href = run.href;
  if (href === undefined || onLink === undefined) return <>{content}</>;
  return (
    <button type="button" className="xa-body-link" onClick={() => onLink(href)}>
      {content}
    </button>
  );
}

function Runs({ runs, onLink }: { runs: readonly BodyRun[]; onLink?: (href: string) => void }) {
  return (
    <>
      {runs.map((r, i) => (
        <Run key={i} run={r} onLink={onLink} />
      ))}
    </>
  );
}

function Block({ block, paragraphClass, onLink }: { block: BodyBlock; paragraphClass: string; onLink?: (href: string) => void }) {
  switch (block.kind) {
    case "paragraph":
      return (
        <p className={paragraphClass}>
          <Runs runs={block.runs} onLink={onLink} />
        </p>
      );
    case "heading": {
      const Tag = block.level === 3 ? "h4" : "h3";
      return (
        <Tag className={`xa-body-heading xa-body-heading--${block.level}`}>
          <Runs runs={block.runs} onLink={onLink} />
        </Tag>
      );
    }
    case "bullet_list":
    case "ordered_list": {
      const Tag = block.kind === "ordered_list" ? "ol" : "ul";
      return (
        <Tag className="xa-body-list">
          {block.items.map((runs, i) => (
            <li key={i}>
              <Runs runs={runs} onLink={onLink} />
            </li>
          ))}
        </Tag>
      );
    }
    default:
      // The parser keeps only the four kinds above; a fifth reaching here is drawn as nothing, never guessed.
      return null;
  }
}

/**
 * The body. `blocks` absent → the plain text, split into paragraphs by blank lines (`chiaDoan`) — the exact
 * markup the screens drew before `body_blocks` existed, so an older server changes nothing. PURE.
 */
export function ArticleBody(props: {
  blocks: readonly BodyBlock[] | undefined;
  text: string;
  /** The screen's paragraph class (`xa-bai__doan` in the commune app, `cd-tin__doan` in the shared app). */
  paragraphClass: string;
  /** A tap on a link, with its https URL. Absent → links are plain words. */
  onLink?: (href: string) => void;
}) {
  if (props.blocks === undefined) {
    return (
      <>
        {chiaDoan(props.text).map((doan, i) => (
          <p key={i} className={props.paragraphClass}>
            {doan}
          </p>
        ))}
      </>
    );
  }
  return (
    <>
      {props.blocks.map((b, i) => (
        <Block key={i} block={b} paragraphClass={props.paragraphClass} onLink={props.onLink} />
      ))}
    </>
  );
}
