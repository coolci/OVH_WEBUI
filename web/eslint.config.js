import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // dist / scripts 是构建产物与 Node 回归脚本,不参与浏览器源码 lint
  { ignores: ["dist", "scripts"] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
          // shadcn/ui 组件文件按约定同时导出样式变体与配套 hook,不拆文件
          allowExportNames: [
            "badgeVariants",
            "buttonVariants",
            "toggleVariants",
            "navigationMenuTriggerStyle",
            "useFormField",
            "useSidebar",
            "toast",
          ],
        },
      ],
      // 未使用的变量 / 参数交给 tsc(noUnusedLocals / noUnusedParameters)报错,不重复报
      "@typescript-eslint/no-unused-vars": "off",
      // parseInt 必须写进制;比较一律用 ===(与 null 比较除外)
      radix: "error",
      eqeqeq: ["error", "smart"],
    },
  },
);
