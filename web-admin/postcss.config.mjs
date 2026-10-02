/**
 * PostCSS runs exactly one plugin: Tailwind v4.
 *
 * WHY TAILWIND HERE AND NOT A CDN OR A RUNTIME INJECTOR: the CSS is compiled at build time into a
 * static asset served by this app's own host. Nothing about it depends on the commune, so it is
 * the same file for every commune — which is the whole packaging model (rule 1, invariant 10).
 *
 * Preflight is deliberately NOT imported (see `src/app/globals.css`): it would restyle every
 * screen still drawn by the legacy classes.
 */
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
