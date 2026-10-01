"use client";

import { useId, useState } from "react";

/**
 * The recovery codes, shown ONCE. The server keeps only their hashes: this screen is the only
 * moment they exist in readable form, so leaving it must be a deliberate act — the "Tiếp tục"
 * button stays disabled until the person ticks that they have saved them.
 *
 * The parent owns the codes and drops them on `onSaved` (`signInReducer` "recovery_codes_saved",
 * or the regenerate screen's own state). Nothing here writes them anywhere but the clipboard or a
 * file the person asked for: no storage, no URL, no log.
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
    <section className="recovery-codes" aria-labelledby={checkId + "-title"}>
      <h2 id={checkId + "-title"} className="form-subtitle">
        Mã khôi phục của bạn
      </h2>
      <p>
        Mỗi mã dùng được <strong>một lần</strong> để đăng nhập khi bạn không dùng được ứng dụng xác thực. Các mã
        chỉ hiện <strong>một lần duy nhất</strong> tại đây; sau khi rời màn này, không ai xem lại được — kể cả
        bộ phận kỹ thuật.
      </p>
      <ol className="code-list">
        {codes.map((c) => (
          <li key={c}>
            <code>{c}</code>
          </li>
        ))}
      </ol>
      <div className="button-row">
        <button type="button" className="secondary-button" onClick={copy}>
          Chép các mã
        </button>
        <button type="button" className="secondary-button" onClick={download}>
          Tải về tệp văn bản
        </button>
      </div>
      <p className="form-notice" role="status" aria-live="polite">
        {notice}
      </p>
      <div className="check-field">
        <input id={checkId} type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} />
        <label htmlFor={checkId}>{RECOVERY_CODES_SAVED_LABEL}</label>
      </div>
      <button type="button" className="primary-button" disabled={!saved} onClick={onSaved}>
        {continueLabel}
      </button>
    </section>
  );
}
