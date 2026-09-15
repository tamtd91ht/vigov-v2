# commune-admin

Staff-facing web for **one commune at a time**, told apart by **domain**
(`tanphu.vigov.vn`). Next.js.

Users are **staff**: accounts issued by an administrator, RBAC, every action attributable.
A different trust class from citizens, which is why the citizen app is a separate application
rather than a role inside this one.

## Structure

```
src/
  app/          App Router — one route group per subsystem
  features/     screens and forms, grouped by subsystem
  lib/          tenant-config.ts (runtime config), session.ts (cookie rules)
  components/   shared UI
```

## Non-negotiables

| # | Rule |
|---|---|
| 1 | Commune configuration is loaded **server-side from `Host`**, never from `NEXT_PUBLIC_*` |
| 2 | A `Host` matching no commune returns **404** — never a fallback commune |
| 3 | Session cookies are scoped to the commune's **own host**. Never the parent domain |
| 4 | Route protection lives in **server-side middleware**, not in hidden buttons |
| 5 | Hiding a control by permission is UX, not security — the backend still checks |
| 6 | Types come from the generated contract, never hand-copied |

Rule 6 exists because it was measured wrong on the previous system: the declared "source of
truth" for types lived in the frontend and the backend imported it twice — three
hand-maintained copies, drifting.

→ Skill: `.claude/skills/nextjs-multi-tenant`
