/**
 * THE SHARED APP'S LAUNCH-CODE STEP (ADR 0047 §Sửa đổi 12/10/2026, owner): a commune link carries `?x=<code>`,
 * not the commune's domain. This step turns the code into the commune's domain ONCE, at launch, through the
 * public `GET /api/v1/communes?code=`, then hands that domain to the caller — and from there everything (public
 * `?host=` reads, draft key, bridge host, client-error host) runs on it exactly as it ran on the old `d`.
 *
 * WHAT THE CITIZEN SEES, AND WHY IT IS THE SAME AS BEFORE:
 *   resolving          the commune app's own waiting line (`APP_RIENG.dang_mo`) in the commune look — never the
 *                      ViHAT intro, never the code, never a commune name the server has not given
 *   no commune         `APP_RIENG.khong_thay` + "Thử lại" — exactly what `TrangXa` showed for a `d` that named
 *                      no commune (`xaTuKetQuaTra`), so no new sentence is invented
 *   network / 503 / …  `APP_RIENG.chua_ket_noi` + "Thử lại" — the same retry `TrangXa` offers
 *
 * ⚠ THE CODE GRANTS NOTHING. It only picks whose PUBLIC pages are looked up; the session still opens only by the
 *   citizen's explicit act through `TrangXa`'s gate, and is kept only if its commune name matches (ADR 0005 · 0019
 *   · 0022). Nothing here logs the code or the answer.
 */
import { type ReactNode, useEffect, useState } from "react";

import { communeByLaunchCode, type KetQuaCongKhai } from "../api/goi-vigov"; // vi-name-ok: existing exported type
import type { CommuneByCode } from "../api/hop-dong-cong-khai";
import { laTenMien } from "../../lib/launch-params"; // vi-name-ok: existing exported function
import { KhoiTrangThai } from "./khung-xa"; // vi-name-ok: existing component
import { APP_RIENG } from "./noi-dung"; // vi-name-ok: existing string table

/** What the lookup produced: the commune's domain, or the sentence to show. */
export type LaunchCodeOutcome = { readonly host: string } | { readonly message: string };

/**
 * Lookup answer → the commune's domain, or the sentence to show. PURE. Exactly ONE commune with a non-empty
 * name and a well-formed domain; anything else on a success is "no commune" — never pick one of two.
 */
export function hostFromLaunchCode(
  result: KetQuaCongKhai<readonly CommuneByCode[]>, // vi-name-ok: existing exported type
): LaunchCodeOutcome {
  if (result.kieu === "xong" && result.gia_tri.length === 1) { // vi-name-ok: existing union fields
    const only = result.gia_tri[0]!; // vi-name-ok: existing union field
    if (only.ten.trim() !== "" && laTenMien(only.host)) return { host: only.host }; // vi-name-ok: existing names
  }
  if (result.kieu === "xong" || result.kieu === "khong-hop-le" || result.kieu === "khong-thay") { // vi-name-ok: existing union
    return { message: APP_RIENG.khong_thay }; // vi-name-ok: existing string key
  }
  return { message: APP_RIENG.chua_ket_noi }; // vi-name-ok: existing string key
}

type Step =
  | { readonly kind: "resolving" }
  | { readonly kind: "failed"; readonly message: string }
  | { readonly kind: "resolved"; readonly host: string };

export function CommuneByLaunchCode(props: {
  /** `x` from the launch link, already shape-checked by `thamSoXa`; checked again before the call. */
  code: string;
  /** Renders the commune's interface for the resolved domain — the shell's `QrCommuneApp`. */
  children: (host: string) => ReactNode;
}) {
  const { code, children } = props;
  const [step, setStep] = useState<Step>({ kind: "resolving" });
  /** Each "Thử lại" bumps it once — the lookup effect re-runs exactly once per value. */
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let alive = true;
    void communeByLaunchCode(code).then((result) => {
      if (!alive) return;
      const outcome = hostFromLaunchCode(result);
      setStep("host" in outcome ? { kind: "resolved", host: outcome.host } : { kind: "failed", message: outcome.message });
    });
    return () => {
      alive = false;
    };
  }, [code, attempt]);

  if (step.kind === "resolved") return <>{children(step.host)}</>;

  // Same markup as `TrangXa`'s own lookup screen, so the citizen sees one continuous waiting line.
  return (
    <div className="xa-app">
      <div className="xa-trang xa-trang--con">
        {step.kind === "resolving" ? (
          // vi-name-ok: existing component props
          <KhoiTrangThai bieu_tuong="build" cau={APP_RIENG.dang_mo} dang_tai shape="none" />
        ) : (
          <KhoiTrangThai
            bieu_tuong="alert" // vi-name-ok: existing prop
            loi // vi-name-ok: existing prop
            cau={step.message} // vi-name-ok: existing prop
            nut={{ // vi-name-ok: existing prop
              nhan: APP_RIENG.thu_lai, // vi-name-ok: existing prop and string key
              onBam: () => { // vi-name-ok: existing prop
                setStep({ kind: "resolving" });
                setAttempt((n) => n + 1);
              },
            }}
          />
        )}
      </div>
    </div>
  );
}
