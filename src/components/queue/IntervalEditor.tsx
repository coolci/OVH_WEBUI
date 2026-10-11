import { useState } from "react";
import { toast } from "sonner";
import { useUpdateQueueInterval } from "@/hooks/use-queue";
import { RETRY_INTERVAL } from "@/hooks/use-settings";

export function IntervalEditor({ id, value }: { id: string; value: number }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(String(value));
  const update = useUpdateQueueInterval();

  const commit = () => {
    setEditing(false);
    const n = Number(draft);
    if (!n || n === value) return setDraft(String(value));
    if (n < RETRY_INTERVAL.min || n > RETRY_INTERVAL.max) {
      toast.error(`重试间隔要在 ${RETRY_INTERVAL.min} ~ ${RETRY_INTERVAL.max} 秒之间`);
      return setDraft(String(value));
    }
    update.mutate({ id, retryInterval: n });
  };

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => {
          setDraft(String(value));
          setEditing(true);
        }}
        className="font-medium text-foreground underline decoration-dotted underline-offset-2 hover:text-primary"
        title="点击修改这条任务的重试间隔"
      >
        {value}
      </button>
    );
  }

  return (
    <input
      autoFocus
      type="text"
      inputMode="numeric"
      value={draft}
      onChange={(e) => {
        const v = e.target.value;
        if (v === "" || /^\d*$/.test(v)) setDraft(v);
      }}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") commit();
        if (e.key === "Escape") {
          setDraft(String(value));
          setEditing(false);
        }
      }}
      className="w-14 px-1 py-0.5 rounded border border-input bg-background text-base sm:text-[11px] text-center"
    />
  );
}
