---
name: proto-contract
description: Use when designing or changing a contract between services — gRPC, protobuf, events, API versioning. Triggers on: proto, protobuf, gRPC, buf, contract, inter-service API, event schema, versioning, breaking change, code generation, generate.
---

# Skill: Contracts in .proto

`.proto` is the **source of truth**. Code is generated from it; documentation is generated
from it. This is Go's biggest advantage for the documentation problem — **the best
documentation is documentation nobody writes**.

## REQUIRED

| # | Practice |
|---|---|
| 1 | Changing a contract means editing the `.proto` and running `buf generate`. **Never edit generated files** |
| 2 | `buf lint` and `buf breaking` run inside `make check` |
| 3 | Field numbers are **never reused**; removed fields become `reserved` |
| 4 | New fields are always **optional**, with a safe default |
| 5 | Events carry the version **in the name**: `petition.received.v1` |
| 6 | Two event versions **run side by side** during a migration; removing the old one is separate, later work |
| 7 | Every RPC carries `tenant_id` in metadata, never in the message body |

## Why tenant_id belongs in metadata, not the body

In the body it is **data the caller declares** — and callers can declare it wrongly. In
metadata it is set by the interceptor from context and lifted back into context by the
receiver, **never within reach of business code**.

## Changing an event that already has consumers — stop condition

Check `kb/30-indexes/event-flows.json` for **who is listening**. If anyone is, do not change
it in place: publish `.v2` alongside, migrate consumers one by one, then retire `.v1`.

```protobuf
message Petition {
  string id = 1;
  string code = 2;
  reserved 3;                      // removed 'phone' — number 3 is never reused
  optional string note = 4;        // new fields are always optional
}
```

→ Rule 2 · `skills/events-and-queues`
