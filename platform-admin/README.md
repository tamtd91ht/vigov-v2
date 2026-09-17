# platform-admin

The console the **vendor** uses to operate the platform across many communes.

## What this application may do — and may not

| May | May not |
|---|---|
| Create, lock, rename a commune; assign its domain | Read the content of files, petitions, documents |
| Quotas, service tier, active status | Read citizen personal data |
| System logs, service health | Read a commune's business audit trail |
| Aggregate counts received **via events** | Query a business database directly |

This is not a permission setting. **There is no client for the business services in this
application** — see `src/lib/api.ts`. The vendor cannot read commune data because no path
exists, and adding one would have to be written and reviewed.

→ `kb/10-decisions/0003-platform-admin-metadata-only.md`

## Why this is a separate application, not a role in commune-admin

A role can be granted. An application that was never given the client cannot be granted its
way into the data. For a government platform holding citizen data on behalf of public
authorities, that difference is the whole point.

## If support genuinely needs to see real data

It must be a **support session granted by the commune**: time-limited, narrow in scope, fully
audited, and **visible to the commune**. Never a standing permission.
