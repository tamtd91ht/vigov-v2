"use client";

import { Copy, Download, TriangleAlert } from "lucide-react";
import { useId, useState } from "react";

import { Button } from "@/components/ui/button";
import { Notice } from "@/components/ui/notice";

/**
 * The recovery codes, shown ONCE. The server keeps only their hashes: this screen is the only
 * moment they exist in readable form, so leaving it must be a deliberate act — the "Tiếp tục"
 * button stays disabled until the person ticks that they have saved them.
 *
 * The parent owns the codes and drops them on `onSaved` (`signInReducer` "recovery_codes_saved",
 * or the regenerate screen's own state). Nothing here writes them anywhere but the clipboard or a
 * file the person asked for: no storage, no URL, no log.
 *
 * LOOK: the "shown once" sentence in a gold warning notice, the codes in a monospace grid, the two
 * existing actions (copy, download) as secondary buttons. The continue button carries no icon: its
 * label alone is the action.
 */

export const RECOVERY_CODES_SAVED_LABEL = "Tôi đã lưu các mã khôi phục ở nơi an toàn";

export function recoveryCodesText(codes: readonly string[]): string {
  return [
    "Mã khôi phục — ViGov Khu vận hành nền tảng",
    "Mỗi mã dùng được một lần, thay cho mã trên ứng dụng xác thực khi bạn không dùng được ứng dụng đó.",
    "",
    ...codes,
    "",
  ].join("\n");
}

export function RecoveryCodesOnce({
  codes,
  onSaved,
  continueLabel,
}: {
  codes: readonly string[];
  onSaved: () => void;
  continueLabel: string;
}) {
  const checkId = useId();
  const [saved, setSaved] = useState(false);
  const [notice, setNotice] = useState("");

  async function copy() {
    try {
      await navigator.clipboard.writeText(codes.join("\n"));
      setNotice("Đã chép các mã. Hãy dán vào nơi lưu trữ an toàn ngay.");
    } catch {
      setNotice("Trình duyệt không cho chép. Hãy tải tệp hoặc chép tay các mã.");
    }
  }

  function download() {
    // A Blob URL lives in this tab only and is revoked at once; the codes never form a navigable URL.
    const url = URL.createObjectURL(new Blob([recoveryCodesText(codes)], { type: "text/plain;charset=utf-8" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "ma-khoi-phuc-vigov-van-hanh.txt";
    a.click();
    URL.revokeObjectURL(url);
    setNotice("Đã tải tệp. Hãy cất tệp ở nơi an toàn và xoá bản ở thư mục tải về nếu không cần.");
  }

  return (
    <section className="flex flex-col gap-4" aria-labelledby={checkId + "-title"}>
      <h2 id={checkId + "-title"} className="m-0 text-base leading-snug font-semibold text-ink-900">
        Mã khôi phục của bạn
      </h2>
      <Notice tone="legal" icon={TriangleAlert}>
        <p>
          Mỗi mã dùng được <strong>một lần</strong> để đăng nhập khi bạn không dùng được ứng dụng xác thực. Các mã
          chỉ hiện <strong>một lần duy nhất</strong> tại đây; sau khi rời màn này, không ai xem lại được — kể cả
          bộ phận kỹ thuật.
        </p>
      </Notice>
      <ol className="m-0 grid list-none grid-cols-2 gap-2 p-0 sm:grid-cols-3">
        {codes.map((c) => (
          <li key={c} className="rounded-control border border-line bg-surface-muted px-3 py-2 text-center">
            <code className="font-mono text-[15px] tracking-[0.05em] break-all text-ink-900">{c}</code>
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="secondary" onClick={copy} icon={<Copy aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
          Chép các mã
        </Button>
        <Button
          type="button"
          variant="secondary"
          onClick={download}
          icon={<Download aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          Tải về tệp văn bản
        </Button>
      </div>
      <p className="m-0 min-h-5 text-[13px] text-ink-500" role="status" aria-live="polite">
        {notice}
      </p>
      <div className="flex items-start gap-2.5">
        <input
          id={checkId}
          type="checkbox"
          checked={saved}
          onChange={(e) => setSaved(e.target.checked)}
          className="m-0 mt-0.5 size-5 shrink-0 cursor-pointer accent-brand-600"
        />
        <label htmlFor={checkId} className="cursor-pointer text-sm font-medium text-ink-900">
          {RECOVERY_CODES_SAVED_LABEL}
        </label>
      </div>
      <Button type="button" variant="primary" size="lg" className="w-full sm:w-auto sm:self-start" disabled={!saved} onClick={onSaved}>
        {continueLabel}
      </Button>
    </section>
  );
}
