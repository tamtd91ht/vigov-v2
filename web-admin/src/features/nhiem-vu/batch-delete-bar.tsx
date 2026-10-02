"use client";

import { Trash2 } from "lucide-react";
import { useState } from "react";

import {
  BATCH_CLEAR_BUTTON,
  BATCH_DELETE_BUTTON,
  BATCH_NOTE,
  BATCH_REASON_LABEL,
  batchProgressText,
  batchReason,
  batchResultLine,
  batchSummary,
  selectedCountLabel,
  type BatchDeleteResult,
} from "./batch-delete";
import { Glyph } from "./task-ui";

/**
 * The bar §2 draws where filter row 2's right side was: `Đã chọn {N} nhiệm vụ` + the red
 * `🗑 Xoá đã chọn`, with the ONE shared reason (user decision 28/09/2026).
 *
 * Presentational: the run itself lives in the page (`runBatchDelete`), which owns the selection and
 * the refresh. Shown while something is selected OR a result is on screen — the per-task result must
 * survive the selection emptying, or a fully successful batch would vanish without a word.
 */
export function BatchDeleteBar({
  count,
  progress,
  results,
  onRun,
  onClear,
}: {
  count: number;
  /** `{ done, total }` while running, else `null`. Every control is disabled meanwhile. */
  progress: { readonly done: number; readonly total: number } | null;
  /** The last run's per-task results, or `null`. */
  results: readonly BatchDeleteResult[] | null;
  onRun: (reason: string) => void;
  onClear: () => void;
}) {
  const [text, setText] = useState("");
  const reason = batchReason(text);
  const running = progress !== null;

  return (
    <div className="form-danh-muc" aria-labelledby="tieu-de-xoa-da-chon">
      {count > 0 && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (reason !== null && !running) onRun(reason);
          }}
        >
          <h3 id="tieu-de-xoa-da-chon">{selectedCountLabel(count)}</h3>
          <p className="ghi-chu">{BATCH_NOTE}</p>
          <div className="o-nhap">
            <label htmlFor="ly-do-xoa-da-chon">{BATCH_REASON_LABEL}</label>
            <input
              id="ly-do-xoa-da-chon"
              name="ly-do-xoa-da-chon"
              value={text}
              required
              autoComplete="off"
              disabled={running}
              onChange={(e) => setText(e.target.value)}
            />
          </div>
          <div className="cum-nut">
            <button type="button" className="nut-phu" disabled={running} onClick={onClear}>
              {BATCH_CLEAR_BUTTON}
            </button>
            <button type="submit" className="nut-xoa" disabled={running || reason === null}>
              <Glyph icon={Trash2} className="mr-1.5 inline size-[18px] align-[-4px]" />
              {BATCH_DELETE_BUTTON}
            </button>
          </div>
        </form>
      )}
      {count === 0 && results !== null && <h3 id="tieu-de-xoa-da-chon">{BATCH_DELETE_BUTTON}</h3>}
      {/* ALWAYS IN THE DOM while the bar is: a live region inserted later is one not every screen
          reader announces. */}
      <p role="status">
        {progress !== null
          ? batchProgressText(progress.done, progress.total)
          : results !== null
            ? batchSummary(results)
            : ""}
      </p>
      {results !== null && progress === null && (
        <ul aria-label="Kết quả xoá từng nhiệm vụ">
          {results.map((r) => (
            <li key={r.code} className={r.ok ? undefined : "thong-bao-loi"}>
              {batchResultLine(r)}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
