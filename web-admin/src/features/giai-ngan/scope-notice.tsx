/**
 * The mandatory banner of the budget screens (`06-giai-ngan §1`) — "ViGov is not accounting software".
 *
 * THE SENTENCE IS THE SERVER'S, NOT OURS. It is `budget.scope_notice`, a system message a commune may
 * reword (`14-cau-hinh §7`: "đi kèm mọi số liệu API trả về"), sent as `scope_notice` on
 * `GET /api/v1/investment-projects` and on each project. A constant here — as there was until
 * 29/09/2026 — would keep showing the software's wording to a commune that changed it on the Lời hệ
 * thống tab, and nothing would say so.
 *
 * NO FALLBACK TEXT: absent or blank renders nothing. Inventing the sentence client-side is exactly
 * the second copy this component exists to remove; the server always sends it.
 *
 * Not `role="alert"`: it is always there, not an event that just happened.
 */
export function ScopeNotice({ text }: { text: string | undefined }) {
  if (text === undefined || text.trim() === "") return null;
  return <p className="canh-bao-pham-vi">{text}</p>;
}
