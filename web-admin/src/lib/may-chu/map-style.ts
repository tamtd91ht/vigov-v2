import "server-only";

/**
 * The basemap style URL of the economic map (`/ban-do`) — ADR 0072 §Sửa đổi H1.
 *
 * WHY AN ENVIRONMENT VARIABLE IS ALLOWED: the basemap is the same for every commune, so it is a
 * PLATFORM constant (rule 8 invariant 5). What it must never be is `NEXT_PUBLIC_*`: that is baked into
 * the bundle at build time (`Dockerfile` refuses to build with one, `ranh-gioi-nguon.test.ts` goes red).
 * Read here, on the server, at REQUEST time, and handed to the client component as a prop.
 *
 * NO DEFAULT AND NO FALLBACK HOST — unlike `goc-dich-vu.ts`, whose default names an in-cluster service.
 * A default here would send every officer's viewport to a host nobody chose on exactly the day the
 * configuration broke (ADR 0072 H1, "Thiếu biến"). Unset or invalid → `null` → the page shows
 * "Chưa cấu hình bản đồ nền." and draws no map; the register still works.
 *
 * Expected value (deploy manifest, Jenkinsfile): `https://tiles.openfreemap.org/styles/liberty`.
 */
export const MAP_STYLE_URL_VAR = "MAP_STYLE_URL";

/**
 * The style URL, or `null` when unset or unusable. Read on every call (a bad value must not crash
 * `next start`). An invalid value is logged by VARIABLE NAME ONLY, never its value (rule 8 invariant 3).
 */
export function mapStyleUrl(env: Readonly<Record<string, string | undefined>> = process.env): string | null {
  const raw = (env[MAP_STYLE_URL_VAR] ?? "").trim();
  if (raw === "") return null;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    console.error(`${MAP_STYLE_URL_VAR} không phải một địa chỉ hợp lệ — trang bản đồ không có nền`);
    return null;
  }
  // https only: the style, its tiles, glyphs and sprite are fetched by every officer's browser over the
  // public internet; a plain-http style would let anyone on the path rewrite the map.
  if (url.protocol !== "https:" || url.username !== "" || url.password !== "") {
    console.error(`${MAP_STYLE_URL_VAR} phải là https và không chứa thông tin đăng nhập — trang bản đồ không có nền`);
    return null;
  }
  return url.toString();
}
