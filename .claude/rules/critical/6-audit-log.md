# RULE 6 — Audit trail

Administrative files carry legal weight. When a complaint, dispute or inspection arrives,
the question is always **"who did what, and when"**. Not being able to answer is the
authority's problem.

An audit trail is **not a technical log**. Logs exist for debugging, rotate by size, and may
be lost. The audit trail is **business data**: retained at least 12 months, never deleted.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every **write** to business data leaves an entry |
| 2 | An entry carries: **who · what · on which record · when · from which IP · in which commune** |
| 3 | The entry is written **in the same transaction** as the change — never "afterwards, if it works" |
| 4 | Entries are **append-only**. Not editable, not deletable, not even by an administrator |
| 5 | Entries hold **before and after** values for significant fields — with personal data masked (rule 3) |
| 6 | System actions (background jobs, migrations) are audited too, with a system principal |
| 7 | Reading **full** personal data, or reading **across communes**, is itself audited |
| 8 | The "who" is the **business code** (`CB-00123`), never the internal id. `authz.Principal` carries both: `.Ma` for the trail, `.ID` for authorisation. **No fallback** — an empty `.Ma` refuses the write |

**Why invariant 8 is not a naming preference:** `actor_id` is read years later, by a person
handling a complaint or an inspection. `CB-00123` names somebody with no lookup still alive;
a ULID names nobody, and the row it points at may by then be gone. Worse, the two are
indistinguishable on sight, so a column holding both is a column nobody can query — which is
what this repository measured on 2026-09-22, when **six** write paths put `Principal.ID` there
and no test turned red. The policy had been written in a comment since the login path was built
(`service-identity/internal/app/dang_nhap.go:151`) and nothing could read a comment.

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | Recording the trail with `log.Info(...)` | Logs rotate and are lost. The trail needs durable storage |
| 2 | Writing the entry outside the transaction | The change can succeed while the entry fails — a state the rules do not permit (rule 2) |
| 3 | Editing or deleting an audit entry | A trail that can be edited has no evidentiary value |
| 4 | Storing raw before/after values of personal-data fields | The audit log becomes a personal-data store |

## STOP CONDITIONS

1. A new operation where it is **unclear whether it must be audited**
2. A request to delete or edit audit entries
3. A request for a retention period other than 12 months

→ Enforcement: `hooks/audit_guard.py` (advisory)
→ Enforcement (8): `hooks/audit_actor_guard.py` (BLOCK, at the write) **and**
  `tools/check_audit_actor.py` (whole repo, in `make check`). Two shapes for the same reason
  rule 5's invariant 3c needs two: a hook sees one file and never re-reads what is already on
  disk — which is where five of the six defects lived. **Both only check the SYNTACTIC SHAPE**
  (`p.ID`, a `nd-…`/ULID literal); a value laundered through a local variable escapes them, and
  `tools/vet_actor.py` states that limit in full rather than implying a guarantee it cannot make
→ Skill: `skills/audit-trail`
