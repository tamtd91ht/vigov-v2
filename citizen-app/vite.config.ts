import { defineConfig } from "vite";

/**
 * Build configuration for the Zalo Mini App bundle.
 *
 * `base: "./"` — assets must be referenced RELATIVE to the bundle. A Mini App is served from a
 * path chosen by the platform, not from the root of a domain we control, so absolute `/assets/…`
 * URLs resolve to somewhere that does not exist and the app opens blank on a real device while
 * working perfectly in `vite preview`.
 *
 * No `@vitejs/plugin-react`: its job is Fast Refresh in dev. Vite's own esbuild pipeline already
 * compiles `.tsx` with the automatic JSX runtime declared in tsconfig.json, so the plugin would
 * be a dependency added for a convenience this four-screen app does not need.
 */
export default defineConfig({
  base: "./",
  build: {
    outDir: "dist",
    // Zalo reviews and hosts a static bundle. Keeping sourcemaps out keeps the uploaded
    // package small and keeps our source off a device we do not control.
    sourcemap: false,
  },
});
