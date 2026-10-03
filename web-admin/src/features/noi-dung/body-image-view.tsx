"use client";

import { NodeViewContent, NodeViewWrapper, type ReactNodeViewProps } from "@tiptap/react";
import { ImageOff } from "lucide-react";
import { createContext, useContext, useState } from "react";

import { cn } from "@/lib/cn";

import { BODY_IMAGE_NO_PREVIEW, bodyImagePreviewSrc } from "./body-image";

/**
 * How the editor DRAWS one body image (`figure` in `rich-text.ts`). The document holds a file id; this
 * view looks it up in the form's preview map — `file_id → preview_url`, from the detail's `body_images` and
 * from each completion / from-url reply — and draws the server's signed 1280 px preview.
 *
 * THE `src` LIVES HERE AND NOWHERE ELSE. It is this view's DOM, outside the node's content and attributes,
 * so `getHTML()` — the body that is saved — can never carry it. It is `bodyImagePreviewSrc` of the server's
 * string (http(s) only) and nothing else: never a `blob:` of the officer's file, never the pasted link
 * (`ranh-gioi-html.test.ts` holds this file to that).
 *
 * NO PREVIEW, OR AN EXPIRED ONE (≤ 15 minutes), DRAWS A SENTENCE — not a broken image icon the officer
 * would read as "the image is lost". The image is still in the body; saving and reopening re-signs it.
 */

/** The form's preview map. Provided around `EditorContent`; node views render inside it (portals). */
export const BodyImagePreviews = createContext<ReadonlyMap<string, string>>(new Map());

export const FIGURE_CAPTION_HINT = "Gõ chú thích dưới ảnh (không bắt buộc)";

export function BodyImageView({ node, selected }: ReactNodeViewProps) {
  const previews = useContext(BodyImagePreviews);
  const fileId = String(node.attrs.fileId ?? "");
  const alt = String(node.attrs.alt ?? "");
  const previewSrc = bodyImagePreviewSrc(previews.get(fileId));
  // The link that failed to load (expired, revoked): a newer link for the same file is tried again.
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

  return (
    <NodeViewWrapper
      as="figure"
      className={cn(
        "body-image my-3 rounded-control border border-transparent p-1",
        selected && "border-brand-500",
      )}
      data-drag-handle=""
    >
      <div contentEditable={false} className="select-none">
        {previewSrc !== null && failedSrc !== previewSrc ? (
          // A plain <img>, not next/image: the optimiser would fetch this presigned link server-side and
          // cache a bearer credential under the commune's own origin (the cover's reason).
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={previewSrc}
            alt={alt}
            draggable={false}
            referrerPolicy="no-referrer"
            className="mx-auto block h-auto max-h-[28rem] max-w-full rounded-lg"
            onError={() => setFailedSrc(previewSrc)}
          />
        ) : (
          <p
            className="m-0 flex min-h-24 items-center justify-center gap-2 rounded-lg border border-dashed border-line-strong bg-surface-muted px-3 py-4 text-center text-sm text-ink-500"
            role="note"
          >
            <ImageOff aria-hidden="true" focusable="false" className="size-4 shrink-0" />
            {BODY_IMAGE_NO_PREVIEW}
          </p>
        )}
      </div>
      <div className="relative mt-1.5">
        {/* The figcaption node draws itself (`<figcaption>`, `rich-text.ts`) inside this box. */}
        <NodeViewContent className="min-h-[1.5em] text-center text-sm text-ink-600" />
        {(node.firstChild?.content.size ?? 0) === 0 && (
          // Not a CSS placeholder: ProseMirror keeps a <br> in an empty caption, so `:empty` never matches.
          <span
            contentEditable={false}
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-0 top-0 text-center text-sm text-ink-400"
          >
            {FIGURE_CAPTION_HINT}
          </span>
        )}
      </div>
    </NodeViewWrapper>
  );
}
