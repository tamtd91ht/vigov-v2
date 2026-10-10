"use client";

import { ImageUp, Link as LinkIcon, RefreshCw, X } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { cn } from "@/lib/cn";

import {
  bodyImageInFlight,
  bodyImageStateText,
  type BodyImageReady,
  type BodyImageState,
} from "./body-image";
import { COVER_ACCEPT, COVER_HINT, COVER_RETRY_BUTTON } from "./cover-image";

/**
 * What the form gives the editor to insert body images: the two calls, each already threading the
 * article id and filling the preview map (`so-noi-dung.tsx`). The editor only inserts the figure.
 */
export type BodyImageSource = {
  /** `null` = images may be inserted; a sentence = why `Chèn ảnh` is off (said next to it). */
  readonly blocked: string | null;
  readonly upload: (file: File, onState: (s: BodyImageState) => void) => Promise<BodyImageState>;
  readonly fromUrl: (url: string, onState: (s: BodyImageState) => void) => Promise<BodyImageState>;
};

export const BODY_IMAGE_PANEL_TITLE = "Chèn ảnh vào thân bài";
export const BODY_IMAGE_PICK = "Tải ảnh từ máy";
export const BODY_IMAGE_URL_LABEL = "Dán liên kết ảnh (https)";
export const BODY_IMAGE_URL_BUTTON = "Lấy ảnh từ liên kết";
export const BODY_IMAGE_CAPTION_LABEL = "Chú thích (không bắt buộc)";
export const BODY_IMAGE_URL_NOTE =
  "Máy chủ tải ảnh về, kiểm tra rồi lưu bản riêng — bài không trỏ tới trang gốc.";

/**
 * The `Chèn ảnh` panel: an in-frame section under the toolbar, like the link row — NOT a nested modal.
 * The article form is already a modal `<dialog>`; a second one inside it would be a second top layer
 * whose Esc also reaches the article's (React bubbles `cancel` through the tree) and closes the whole form.
 *
 * The caption is asked FIRST: picking a file starts the upload at once (fewest steps), so the caption must
 * already be there. It is plain text here and becomes the figure's content — editable afterwards right
 * under the image.
 *
 * Esc closes the panel only (stopped here, so the article dialog does not see it).
 */
export function BodyImagePanel({
  id,
  disabled,
  source,
  onReady,
  onClose,
}: {
  /** Prefix for the panel's ids (the editor's id). */
  id: string;
  disabled: boolean;
  source: BodyImageSource;
  /** A ready image and the caption typed: the editor inserts the figure at the cursor. */
  onReady: (image: BodyImageReady, caption: string) => void;
  onClose: () => void;
}) {
  const [caption, setCaption] = useState("");
  const [url, setUrl] = useState("");
  const [state, setState] = useState<BodyImageState>({ kind: "idle" });
  const busy = bodyImageInFlight(state);
  const off = disabled || busy;
  const text = bodyImageStateText(state);
  const failed = state.kind === "refused" || state.kind === "retry";

  async function finish(p: Promise<BodyImageState>): Promise<void> {
    const s = await p;
    if (s.kind === "ready") {
      onReady(s.image, caption);
      setCaption("");
      setUrl("");
      setState({ kind: "idle" });
    }
  }

  return (
    <div
      id={`${id}-image-panel`}
      role="group"
      aria-labelledby={`${id}-image-title`}
      className="flex flex-col gap-2.5 border-b border-line bg-surface-muted px-3 py-3 text-sm [&_p]:m-0"
      onKeyDown={(e) => {
        if (e.key === "Escape" && !busy) {
          e.preventDefault();
          e.stopPropagation();
          onClose();
        }
      }}
    >
      <div className="flex items-center gap-2">
        <p id={`${id}-image-title`} className="flex-1 font-semibold text-ink-900">
          {BODY_IMAGE_PANEL_TITLE}
        </p>
        <button
          type="button"
          className="inline-grid size-8 place-items-center rounded-lg text-ink-600 hover:bg-brand-50 focus-visible:outline-2 focus-visible:outline-brand-500 disabled:opacity-40"
          title="Đóng"
          disabled={busy}
          onClick={onClose}
        >
          <X aria-hidden="true" focusable="false" className="size-4" />
          <span className="an-thi-giac">Đóng khung chèn ảnh</span>
        </button>
      </div>

      <label htmlFor={`${id}-image-caption`} className="text-xs font-semibold text-ink-700">
        {BODY_IMAGE_CAPTION_LABEL}
      </label>
      <input
        id={`${id}-image-caption`}
        name={`${id}-image-caption`}
        autoComplete="off"
        value={caption}
        disabled={off}
        className={cn(controlClass, "h-[34px] text-sm")}
        onChange={(e) => setCaption(e.target.value)}
        onKeyDown={(e) => {
          // Enter inside the article form would submit the whole article.
          if (e.key === "Enter") e.preventDefault();
        }}
      />

      <div className="task-attachments flex flex-wrap items-center gap-2">
        {/* The cover's control shape: the input visually hidden but focusable, its label the button. */}
        <input
          id={`${id}-image-file`}
          name={`${id}-image-file`}
          type="file"
          accept={COVER_ACCEPT}
          className="an-thi-giac"
          aria-describedby={`${id}-image-file-hint`}
          disabled={off}
          onChange={(e) => {
            const f = e.target.files?.[0];
            e.target.value = ""; // the same file can be chosen again after a refusal
            if (f !== undefined) void finish(source.upload(f, setState));
          }}
        />
        <label htmlFor={`${id}-image-file`} className="nut-phu inline-flex items-center gap-2">
          <ImageUp aria-hidden="true" focusable="false" className="size-4" />
          {BODY_IMAGE_PICK}
        </label>
        <span className="ghi-chu text-xs text-ink-500" id={`${id}-image-file-hint`}>
          {COVER_HINT}
        </span>
      </div>

      <label htmlFor={`${id}-image-url`} className="text-xs font-semibold text-ink-700">
        {BODY_IMAGE_URL_LABEL}
      </label>
      <div className="flex flex-wrap items-center gap-2">
        <input
          id={`${id}-image-url`}
          name={`${id}-image-url`}
          type="url"
          inputMode="url"
          placeholder="https://"
          autoComplete="off"
          value={url}
          disabled={off}
          aria-describedby={`${id}-image-url-note`}
          className={cn(controlClass, "h-[34px] min-w-0 flex-[1_1_14rem] text-sm")}
          onChange={(e) => setUrl(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              if (!off && url.trim() !== "") void finish(source.fromUrl(url, setState));
            }
          }}
        />
        <Button
          type="button"
          size="sm"
          variant="secondary"
          className="min-h-0"
          icon={<LinkIcon aria-hidden="true" focusable="false" />}
          disabled={off || url.trim() === ""}
          onClick={() => void finish(source.fromUrl(url, setState))}
        >
          {BODY_IMAGE_URL_BUTTON}
        </Button>
      </div>
      <p className="ghi-chu text-xs text-ink-500" id={`${id}-image-url-note`}>
        {BODY_IMAGE_URL_NOTE}
      </p>

      {text !== "" && (
        <p role={failed ? "alert" : "status"} className={failed ? "thong-bao-loi text-[13px]" : "text-ink-700"}>
          {text}
        </p>
      )}
      {state.kind === "retry" && (
        <div>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<RefreshCw aria-hidden="true" focusable="false" />}
            disabled={disabled}
            onClick={() => void finish(source.upload(state.file, setState))}
          >
            {COVER_RETRY_BUTTON}
          </Button>
        </div>
      )}
    </div>
  );
}
