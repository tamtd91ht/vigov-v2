"use client";

import { useState } from "react";

import type { KetQua } from "@/lib/api/request";
import { seedRoleTemplates } from "@/lib/api/role-templates";
import type { identity_seedRoleTemplatesOut } from "@/lib/api/schema.gen";

import {
  SEED_BUTTON,
  SEED_CANCEL_BUTTON,
  SEED_CONFIRM_BUTTON,
  SEED_CONFIRM_TEXT,
  SEED_HINT,
  SEED_SENDING,
  seedSummary,
} from "./role-templates";

/** Where the panel stands. `result` holds the last answer until the next click. */
export type SeedPhase =
  | { readonly kind: "idle" }
  | { readonly kind: "confirming" }
  | { readonly kind: "sending" }
  | { readonly kind: "done"; readonly result: KetQua<identity_seedRoleTemplatesOut> };

/**
 * "Tạo tám vai trò mẫu" (ADR 0055 §2) — shown only to an account the tab already let edit
 * (`admin.role`); the server checks `admin.role` AND #14 (every key the templates grant) on the call.
 *
 * CONFIRM FIRST, INLINE: a click that creates roles with rights attached deserves one sentence read
 * before it. Inline, not a floating dialog — the same choice as every write form on this screen.
 *
 * NO RETRY LOOP, NO OPTIMISM: after a 200 the matrix is read again (`onSeeded`), because the new
 * columns and their ticks exist only on the server.
 */
export function RoleTemplateSeedPanel({ onSeeded }: { onSeeded: () => void }) {
  const [phase, setPhase] = useState<SeedPhase>({ kind: "idle" });

  async function send() {
    setPhase({ kind: "sending" });
    const result = await seedRoleTemplates();
    setPhase({ kind: "done", result });
    if (result.ok) onSeeded();
  }

  return (
    <RoleTemplateSeedView
      phase={phase}
      onOpen={() => setPhase({ kind: "confirming" })}
      onConfirm={() => void send()}
      onCancel={() => setPhase({ kind: "idle" })}
    />
  );
}

/**
 * Pure rendering, exported so the test renders each phase with `react-dom/server`: the 403
 * sentence naming the missing keys must reach the page VERBATIM, and a list of what was skipped
 * must say why it was skipped.
 */
export function RoleTemplateSeedView({
  phase,
  onOpen,
  onConfirm,
  onCancel,
}: {
  phase: SeedPhase;
  onOpen: () => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="khoi-vai-tro-mau">
      <p className="ghi-chu">{SEED_HINT}</p>

      {phase.kind === "confirming" || phase.kind === "sending" ? (
        <div className="form-danh-muc" role="group" aria-label={SEED_BUTTON}>
          <p>{SEED_CONFIRM_TEXT}</p>
          <div className="cum-nut">
            <button type="button" className="nut-chinh" onClick={onConfirm} disabled={phase.kind === "sending"}>
              {phase.kind === "sending" ? SEED_SENDING : SEED_CONFIRM_BUTTON}
            </button>
            <button type="button" className="nut-phu" onClick={onCancel} disabled={phase.kind === "sending"}>
              {SEED_CANCEL_BUTTON}
            </button>
          </div>
        </div>
      ) : (
        <p>
          <button type="button" className="nut-phu" onClick={onOpen}>
            {SEED_BUTTON}
          </button>
        </p>
      )}

      {phase.kind === "done" && !phase.result.ok && (
        // The server's sentence, verbatim — for 403 `permission_escalation` it NAMES the keys the
        // account lacks, which is the whole point of the refusal (`role_template.go`).
        <p className="thong-bao-loi" role="alert">
          {phase.result.thongBao}
        </p>
      )}

      {phase.kind === "done" && phase.result.ok && <SeedResult result={phase.result.duLieu} />}
    </div>
  );
}

function SeedResult({ result }: { result: identity_seedRoleTemplatesOut }) {
  const { lead, blocks } = seedSummary(result);
  return (
    <div role="status">
      <p>{lead}</p>
      {blocks.map((b) => (
        <div key={b.heading}>
          <p className="dong-phu">{b.heading}</p>
          <ul>
            {b.names.map((n) => (
              <li key={n}>{n}</li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
