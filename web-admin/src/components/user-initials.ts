/**
 * Avatar initials for the topbar — spec §7: "2 chữ cái tắt (họ cuối + tên)", the first letters of
 * the LAST TWO words of a Vietnamese full name: "Nguyễn Văn An" → "VA", "Trần Bình" → "TB".
 *
 * Vietnamese names are written family name first and the given name last, so the last two words
 * are the ones a colleague would call someone by. One word gives one letter; a blank name gives an
 * empty string — and the caller then draws no avatar rather than a placeholder letter, the same
 * "no fallback identity" rule as `khoi-nguoi-dung.ts`.
 *
 * Purely decorative (the full name is printed right beside it): no personal data leaves the page.
 */
export function userInitials(fullName: string): string {
  const words = fullName.trim().split(/\s+/).filter((w) => w !== "");
  return words
    .slice(-2)
    .map((w) => Array.from(w)[0] ?? "")
    .join("")
    .toLocaleUpperCase("vi");
}
