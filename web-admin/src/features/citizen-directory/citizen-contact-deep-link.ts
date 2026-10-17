/**
 * `?id=` of `/nguoi-dan` → the directory row to open, or `null` (ADR 0102: the petition drawer's
 * "Xem trong danh bạ").
 *
 * A ULID OR NOTHING (26 Crockford base-32 characters — `service-identity` mints row ids with `ulid.Moi`).
 * The shape is the guarantee that this parameter can never carry a phone number: a value that is not a row
 * id is ignored, never sent (rule 3, forbidden #4). The detail route still decides whether this account in
 * this commune may read the row, and answers 404 for anybody else's.
 */
export function citizenContactDeepLinkId(raw: string | readonly string[] | undefined): string | null {
  if (typeof raw !== "string") return null;
  const id = raw.trim();
  return /^[0-9A-HJKMNP-TV-Z]{26}$/i.test(id) ? id.toUpperCase() : null;
}
