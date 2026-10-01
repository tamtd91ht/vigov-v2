import nextVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";

/**
 * ESLint flat config, mirroring web-admin's.
 *
 * `next lint` was removed in Next.js 16: the old `lint` script here ran nothing and nobody
 * noticed, because nobody called it. This calls `eslint` with Next's rule set directly.
 */
const config = [
  { ignores: [".next/**", "node_modules/**", "next-env.d.ts"] },
  ...nextVitals,
  ...nextTypescript,
  {
    rules: {
      // A parameter starting with `_` is DELIBERATELY unused: a signature that is already right
      // while the body does not need it yet. Without this, that convention is an unfixable warning.
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
    },
  },
];

export default config;
