import js from "@eslint/js";
import pluginVue from "eslint-plugin-vue";
import globals from "globals";

export default [
  { ignores: ["dist/**", "dist-demo/**", "node_modules/**", "test-results/**", "e2e/.auth/**"] },
  js.configs.recommended,
  ...pluginVue.configs["flat/recommended"],
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node, __APP_VERSION__: "readonly" },
    },
    rules: {
      "no-unused-vars": ["error", { argsIgnorePattern: "^_", varsIgnorePattern: "^_", ignoreRestSiblings: true, caughtErrors: "none" }],
      // Formatting is not enforced by a linter here.
      "vue/max-attributes-per-line": "off",
      "vue/singleline-html-element-content-newline": "off",
      "vue/html-self-closing": "off",
      "vue/html-indent": "off",
      "vue/html-closing-bracket-newline": "off",
      "vue/multi-word-component-names": "off",
      "vue/attributes-order": "off",
    },
  },
  {
    // Playwright specs must not sleep: wait on a condition instead.
    files: ["e2e/**/*.js"],
    rules: {
      "no-restricted-syntax": [
        "warn",
        {
          selector: "CallExpression[callee.property.name='waitForTimeout']",
          message: "Avoid fixed sleeps; assert on the state you are waiting for.",
        },
      ],
    },
  },
];
