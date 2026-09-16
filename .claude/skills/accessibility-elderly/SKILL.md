---
name: accessibility-elderly
description: Use when designing or changing any citizen-facing interface — text size, touch targets, forms, messages, accessibility. Triggers on: citizen UI, UX, font size, touch target, accessibility, elderly, form, error message, contrast, readability.
---

# Skill: Citizen interfaces — everyone must be able to use them

Citizens **do not choose** this software. They have to use it to deal with their local
authority. Being unable to use it means **being unable to reach a public service** — that is
a rights issue, not a user-experience issue.

## REQUIRED

| # | Practice | Threshold |
|---|---|---|
| 1 | Body text size | ≥ 16px, zoom never disabled |
| 2 | Touch targets | ≥ 44×44px |
| 3 | Text/background contrast | ≥ 4.5:1 |
| 4 | Forms | **One task per screen**; labels **above** inputs, never placeholder-only |
| 5 | Error messages | Say **what to do next**, never an error code |
| 6 | Status | Text plus icon, **never colour alone** |
| 7 | Significant actions | Confirm, stating the consequence |

## Error messages — the shape

| Wrong | Right |
|---|---|
| "Error 422: Validation failed" | "That phone number is not valid. Enter 10 digits starting with 0." |
| "Unauthorized" | "Your session has expired. Sign in again to continue." |
| "Something went wrong" | "Could not send. Check your connection and try again." |

## Vietnamese written for citizens

Neutral, courteous forms of address. **Never** use administrative jargon where an everyday
word exists. **Never** abbreviate. Short sentences, one idea each.

## Submitting a petition — the highest-traffic citizen flow

This is the one screen most citizens will ever use. It is often used **outdoors, one-handed,
on mobile data, by someone who is upset about the thing they are reporting**.

| # | Requirement | Why |
|---|---|---|
| 1 | Content + photo are the only **required** fields | Every extra required field loses reports the commune needed |
| 2 | Location offered from GPS, **editable by hand** | GPS is wrong indoors and in alleys |
| 3 | Field (`linh_vuc`) may be left blank | Classification is a **clerk's** job, not the citizen's (rule 10) |
| 4 | Draft survives the app closing | A dropped connection must not lose a typed report |
| 5 | On success, show the **lookup code** large, and offer to copy it | It is the citizen's only handle on the case |
| 6 | Photo upload shows progress and survives a retry | Rural connections drop mid-upload |
| 7 | Never block submission on a failed **optional** step | A failed reverse-geocode must not stop the report |

## Reading progress back

Citizens check status far more often than they submit. Each status needs a **plain-language**
line, not the internal key: `dang_xu_ly` → "Đang được xử lý", with the expected completion
date and who to contact. Never show `tenant_id`, internal notes, or routing history (rule 4).

## What a screen must never do

| Never | Why |
|---|---|
| Show a raw status key (`da_dong`) or an internal code | Meaningless to a citizen |
| Require an account to **look up** a case by code | The code is the identity for lookup |
| Use red as the only signal of "overdue" | Colour alone fails colour-blind users (REQUIRED #6) |
| Auto-log-out while a form is half-typed | Loses the report |

→ `skills/zalo-miniapp-multi-tenant` · `skills/administrative-language` · `skills/petition-lifecycle`
