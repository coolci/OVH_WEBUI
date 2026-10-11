/** 存储模式单选项。用 radio 而不是多个 checkbox，UI 上就杜绝了"同时选两种"这条会被后端 400 的路径 */
export function StorageModeOption({
  checked,
  onSelect,
  label,
  hint,
  disabled,
}: {
  checked: boolean;
  onSelect: () => void;
  label: string;
  hint?: string;
  disabled?: boolean;
}) {
  return (
    <label
      className={
        "flex items-start gap-2 rounded-xl border px-3 py-2 cursor-pointer transition-colors " +
        (checked ? "border-foreground bg-secondary/50 " : "border-border hover:bg-secondary/30 ") +
        (disabled ? "opacity-50 cursor-not-allowed" : "")
      }
    >
      <input
        type="radio"
        checked={checked}
        onChange={() => !disabled && onSelect()}
        disabled={disabled}
        className="w-4 h-4 mt-0.5"
      />
      <span className="min-w-0">
        <span className="block font-semibold">{label}</span>
        {hint && <span className="block text-[11px] text-muted-foreground">{hint}</span>}
      </span>
    </label>
  );
}
