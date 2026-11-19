/**
 * Canonical English dictionary for backend API status/error codes.
 * Enforces 100% compile-time key parity with `apiZh` without unsafe casts.
 */
import type { apiZh } from "../api-zh.generated";
import { apiEnGenerated } from "../api-en.generated";
import { apiEn as apiEnTranslated } from "../api-en.translated";

export const apiEn = {
  ...apiEnGenerated,
  ...apiEnTranslated,
} satisfies Record<keyof typeof apiZh, string>;
