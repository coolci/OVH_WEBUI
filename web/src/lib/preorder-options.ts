export function preorderOptions(options: string[], cpuCodes: string[] = []): string[] {
  const cpus = new Set(cpuCodes.map((value) => value.toLowerCase()));
  return options.filter((value) => !cpus.has(value.toLowerCase()) && !/^(?:cpu|processor)(?:-|$)/i.test(value.trim()));
}

export function retainPreorderPlan(previous: string, planCode: string, prerelease: boolean): string {
  return prerelease || previous === planCode ? planCode : "";
}

function stripPreorderPlanSuffix(option: string, plan: string): string {
  const opt = option.trim().toLowerCase();
  const suffixes: string[] = [];
  if (plan) {
    suffixes.push(plan);
    const lastDash = plan.lastIndexOf("-");
    if (lastDash > 0) {
      const region = plan.slice(lastDash + 1);
      if (region && /^[a-z]+$/.test(region)) {
        suffixes.push(plan.slice(0, lastDash));
      }
    }
  }
  for (const suffix of suffixes) {
    if (suffix && opt.endsWith(`-${suffix}`)) {
      return opt.slice(0, -(suffix.length + 1));
    }
  }
  return opt.replace(/-(?:\d{2}[a-z0-9]+-v\d+(?:-[a-z]+)?|rise|sys|sk)$/, "");
}

// This is form feedback only. The backend resolves official product keys and checks the cart.
export function preorderVariantCoversAll(fqn: string, options: string[]): boolean {
  const segments = fqn.toLowerCase().split(".");
  const plan = segments[0] || "";
  const parts = segments.slice(1);
  return options.every((option) => {
    const wanted = stripPreorderPlanSuffix(option, plan);
    if (!wanted.startsWith("ram-") && !wanted.startsWith("gpu-") && !/^(?:hybridsoft|soft|hard|no)?raid-/.test(wanted)) return true;
    return parts.some((part) => part === wanted);
  });
}
