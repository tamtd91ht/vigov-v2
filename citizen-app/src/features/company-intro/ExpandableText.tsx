import { useId, useState } from "react";

/**
 * A LONG PARAGRAPH, THREE LINES FIRST (owner 12/10/2026: "text walls" on Giải pháp).
 *
 * WHY A THRESHOLD IN CHARACTERS, NOT A MEASURED HEIGHT: the screens are tested with `renderToStaticMarkup`, which has
 * no layout, and a toggle that only appears after a measurement would be a toggle no test can see. 160 characters is
 * about three lines at 17px on a 320px screen; a shorter text is shown whole, with no button that would do nothing.
 *
 * THE FULL TEXT IS ALWAYS IN THE PAGE. The clamp is CSS (`.vh-clamp`), so a screen reader reads everything and
 * "Xem thêm" changes only what the eye sees. `aria-expanded` + `aria-controls` say which text the button opens.
 */
export const COLLAPSE_AFTER_CHARS = 160;

/** Whether `text` is long enough to start collapsed. PURE. */
export function needsCollapse(text: string): boolean {
  return text.trim().length > COLLAPSE_AFTER_CHARS;
}

export const EXPAND_LABEL = "Xem thêm";
export const COLLAPSE_LABEL = "Thu gọn";

export function ExpandableText(props: { text: string; className: string }) {
  const [open, setOpen] = useState(false);
  const id = useId();

  if (!needsCollapse(props.text)) return <p className={props.className}>{props.text}</p>;

  return (
    <>
      <p id={id} className={open ? props.className : `${props.className} vh-clamp`}>
        {props.text}
      </p>
      <button
        type="button"
        className="vh-more"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((was) => !was)}
      >
        {open ? COLLAPSE_LABEL : EXPAND_LABEL}
      </button>
    </>
  );
}
