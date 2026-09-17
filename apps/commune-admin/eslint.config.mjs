import nextVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";

/**
 * Cấu hình ESLint (flat config).
 *
 * `next lint` đã bị bỏ khỏi Next.js 16, nên script `lint` cũ trong package.json chạy ra lỗi
 * "no such directory: ./lint" — tức là từ lúc nâng lên 16, lint của ứng dụng này không hề chạy,
 * mà vẫn xanh vì không ai gọi. Ở đây gọi thẳng `eslint` với bộ quy tắc của Next.
 *
 * `schema.gen.ts` nằm ngoài phạm vi lint: nó là tệp SINH RA, sửa tay sẽ mất ở lần sinh sau, nên
 * một cảnh báo lint trên nó là một cảnh báo không ai được phép sửa (luật 9, bất biến 8).
 */
const cauHinh = [
  {
    ignores: [".next/**", "node_modules/**", "next-env.d.ts", "src/lib/api/schema.gen.ts"],
  },
  ...nextVitals,
  ...nextTypescript,
  {
    rules: {
      // Tham số mở đầu bằng `_` là tham số cố ý chưa dùng — quy ước sẵn có trong kho này
      // (`lib/tenant-config.ts` dùng `_host` ở phần khung chưa nối). Không có dòng này, quy ước
      // ấy sinh ra cảnh báo không sửa được, và cảnh báo không sửa được thì người ta tắt cả lint.
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
    },
  },
];

export default cauHinh;
