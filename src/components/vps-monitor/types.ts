export const SUBSIDIARIES = [
  { value: "IE", label: "IE 爱尔兰" },
  { value: "FR", label: "FR 法国" },
  { value: "GB", label: "GB 英国" },
  { value: "DE", label: "DE 德国" },
  { value: "ES", label: "ES 西班牙" },
  { value: "IT", label: "IT 意大利" },
  { value: "PL", label: "PL 波兰" },
  { value: "CA", label: "CA 加拿大" },
  { value: "US", label: "US 美国" },
];

export function modelLabel(code: string, models?: { planCode: string; name: string }[]): string {
  return models?.find((m) => m.planCode === code)?.name || code;
}

export function formatTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
