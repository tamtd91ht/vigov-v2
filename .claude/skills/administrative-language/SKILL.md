---
name: administrative-language
description: Use when writing any text a user will read — labels, messages, documentation, field names, content sent to citizens. Triggers on: Vietnamese, administrative terminology, feedback, complaint, denunciation, incoming document, outgoing document, spelling, label, wording, field naming.
---

# Skill: Vietnamese administrative language

A spelling error or a misused term inside a government application is a **business error**
somebody has to answer for — not a minor detail.

This skill has **no enforcing hook**: a machine cannot judge correct terminology in context.
That is exactly why it lives in `skills/` and not in `rules/critical/`, following the brain's
invariant: *a rule you cannot enforce is not a rule*.

## Three terms that are routinely confused — and differ legally

| Term (Vietnamese) | Meaning | Consequence |
|---|---|---|
| **Phản ánh, kiến nghị** (feedback, suggestion) | A resident raises an issue or proposes something | No mandatory procedure, no statutory deadline |
| **Khiếu nại** (complaint) | Disagreement with a specific **administrative decision** | Statutory procedure and **statutory deadlines** |
| **Tố cáo** (denunciation) | Reporting a violation of law | Separate procedure, **protection of the reporter** |

Mislabelling means **applying the wrong procedure and the wrong deadline**. In code, field
names and status names must follow these terms — getting it wrong at the schema level makes
every layer above wrong too.

## Other conventions

| Right | Wrong |
|---|---|
| Văn bản **đến** / văn bản **đi** (incoming / outgoing document) | văn bản vào / ra |
| **Thụ lý**, **luân chuyển**, **ban hành** (accept, route, issue) | xử lý, chuyển, phát hành |
| **UBND xã Tân Phú** | UBND Xã Tân Phú · ubnd xã tân phú |
| **Hồ sơ một cửa** (one-stop-shop file) | hồ sơ 1 cửa |

## REQUIRED

| # | Practice |
|---|---|
| 1 | Field names in code follow **business terminology**, not whichever word was convenient |
| 2 | Display labels live in configuration, not scattered through the code |
| 3 | Text for **citizens** uses everyday words; text for **staff** uses the precise term |
| 4 | No abbreviations in anything shown to citizens |

→ `kb/00-foundation/ubiquitous-language.md` · `/review-compliance`
