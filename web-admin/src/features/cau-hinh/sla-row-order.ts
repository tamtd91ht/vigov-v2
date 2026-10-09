import type { identity_dongSLARa } from "@/lib/api/schema.gen";

/**
 * Display order of the SLA table (user decision 09/10/2026, spec 08 "Sắp dòng theo prototype"):
 * Văn bản đến → Đơn thư (a kind only this system has, kept after Văn bản đến) → Phản ánh của người dân
 * → Nhiệm vụ. A kind this list does not know goes last, in the server's order — it is a schema the
 * screen was not built for, and hiding it would hide what an operator must see.
 *
 * Inside "Phản ánh của người dân" the default row leads, then the field rows by the LABEL the officer
 * reads, A→Z in Vietnamese collation — a code order would put "an-ninh" before "Rác thải" for reasons
 * nobody on screen can see. Every other kind keeps its default row first and the server's order after.
 *
 * DISPLAY ONLY: no figure is computed or changed here (rule 10 invariant 2).
 */
const KIND_ORDER: readonly string[] = ["van-ban-den", "don-thu", "phan-anh", "nhiem-vu"];

/** The kind whose field rows are sorted by label. */
const LABEL_SORTED_KIND = "phan-anh";

export function orderSlaRows(
  rows: readonly identity_dongSLARa[],
  labelOf: (row: identity_dongSLARa) => string,
): identity_dongSLARa[] {
  const rank = (kind: string) => {
    const i = KIND_ORDER.indexOf(kind);
    return i === -1 ? KIND_ORDER.length : i;
  };
  const collator = new Intl.Collator("vi");
  return rows
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const byKind = rank(a.row.work_kind) - rank(b.row.work_kind);
      if (byKind !== 0) return byKind;
      // Unknown kinds share one rank; keep them apart and in server order.
      if (a.row.work_kind !== b.row.work_kind) return a.index - b.index;
      if (a.row.is_default !== b.row.is_default) return a.row.is_default ? -1 : 1;
      if (a.row.work_kind === LABEL_SORTED_KIND && !a.row.is_default) {
        const byLabel = collator.compare(labelOf(a.row), labelOf(b.row));
        if (byLabel !== 0) return byLabel;
      }
      return a.index - b.index;
    })
    .map((x) => x.row);
}
