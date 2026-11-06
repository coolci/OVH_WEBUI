type MonitorAccountSelection = {
  autoOrderAccountIds?: string[] | null;
  autoOrderAccountId?: string;
};

/** An explicit list wins over the legacy single-account field, including []. */
export function monitorAccountIds(selection?: MonitorAccountSelection | null): string[] {
  const ids = selection?.autoOrderAccountIds ?? [selection?.autoOrderAccountId || ""];
  return [...new Set(ids.map((id) => id.trim()).filter(Boolean))];
}

/** Default only on the user's explicit enable action, never on opening or clearing a form. */
export function accountsWhenEnablingAutoOrder(wasEnabled: boolean, enabled: boolean, selected: string[], available: string[]): string[] {
  return !wasEnabled && enabled && selected.length === 0 ? [...available] : selected;
}

export function isSinglePlanCode(value: string): boolean {
  return !!value.trim() && !/[\s,，、]/u.test(value.trim());
}

/** Search text becomes a custom model only through an explicit selection. */
export function customPlanCode(query: string, plans: { planCode: string }[]): string {
  const code = query.trim();
  return isSinglePlanCode(code) && !plans.some((p) => p.planCode.toLowerCase() === code.toLowerCase())
    ? code
    : "";
}

export function parseOptionCodes(value: string): string[] {
  return [...new Set(value.split(/[\s,，、]+/u).map((code) => code.trim()).filter(Boolean))];
}
