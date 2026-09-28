/**
 * Save a downloaded file in the browser: a temporary `<a download>` on an object URL — no navigation,
 * so the page and its filters stay. The URL is released a moment later: released at once, some
 * browsers cancel the download.
 *
 * ONE COPY for the screen's two downloads (the Sổ theo dõi export and the import template).
 */
export function saveFile(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
