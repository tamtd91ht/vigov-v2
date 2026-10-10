/**
 * TEST SUPPORT ONLY — imported by `*.test.ts(x)`, never by the app. A fake `XMLHttpRequest` for
 * `sendUpload` (`./upload.ts`): it records every request the way the server would read it (method, URL,
 * headers, the multipart parts IN ORDER) and answers with what the test says.
 *
 * One copy for every upload test, so a test of the task attachments and a test of the logo read the wire
 * the same way — the presigned era had five hand-written fakes that each checked a different subset.
 */

import { vi } from "vitest";

/** One part as sent: a text value, or `{ file: <name> }` for the binary part. */
export type SentPart = readonly [name: string, value: string | { readonly file: string; readonly size: number }];

export type SentUpload = {
  readonly method: string;
  readonly url: string;
  readonly headers: Readonly<Record<string, string>>;
  readonly withCredentials: boolean;
  readonly parts: readonly SentPart[];
};

/** What the fake answers: a status and a JSON body, or `"network"` for no answer at all. */
export type FakeAnswer = { readonly status: number; readonly body?: unknown; readonly raw?: string } | "network";

/**
 * Installs the fake. `answer` sees each request (already recorded) and returns the reply, or a promise
 * of it. Progress is
 * reported at 50% and then the upload's `onload` fires, before the reply — the order a browser uses.
 */
export function installFakeUploadXHR(answer: (req: SentUpload) => FakeAnswer | Promise<FakeAnswer>): SentUpload[] {
  const sent: SentUpload[] = [];
  class FakeXHR {
    status = 0;
    responseText = "";
    withCredentials = false;
    upload: {
      onprogress: ((e: { lengthComputable: boolean; loaded: number; total: number }) => void) | null;
      onload: (() => void) | null;
    } = { onprogress: null, onload: null };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    onabort: (() => void) | null = null;
    private method = "";
    private url = "";
    private headers: Record<string, string> = {};
    open(method: string, url: string) {
      this.method = method;
      this.url = url;
    }
    setRequestHeader(name: string, value: string) {
      this.headers[name] = value;
    }
    send(form: FormData) {
      const parts: SentPart[] = [];
      for (const [name, value] of form.entries()) {
        parts.push(
          typeof value === "string" ? [name, value] : [name, { file: (value as File).name, size: value.size }],
        );
      }
      const req: SentUpload = {
        method: this.method,
        url: this.url,
        headers: { ...this.headers },
        withCredentials: this.withCredentials,
        parts,
      };
      sent.push(req);
      // A promised answer lets a test hold the reply open (the window while the server scans).
      void Promise.resolve().then(async () => {
        const a = await answer(req);
        if (a === "network") {
          this.onerror?.();
          return;
        }
        this.upload.onprogress?.({ lengthComputable: true, loaded: 50, total: 100 });
        this.upload.onload?.();
        this.status = a.status;
        this.responseText = a.raw ?? (a.body === undefined ? "" : JSON.stringify(a.body));
        this.onload?.();
      });
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return sent;
}

/** The part names in order — the shape most assertions want. */
export function partNames(req: SentUpload | undefined): string[] {
  return (req?.parts ?? []).map(([n]) => n);
}

/** One text part's value, or `undefined` when it was not sent. */
export function partValue(req: SentUpload | undefined, name: string): string | undefined {
  const p = req?.parts.find(([n]) => n === name);
  return p === undefined || typeof p[1] !== "string" ? undefined : p[1];
}
