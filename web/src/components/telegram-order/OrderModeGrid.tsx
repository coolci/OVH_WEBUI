import { cn } from "@/lib/utils";
import type { OrderMode } from "./types";
import { ORDER_MODES } from "./order-modes";

interface OrderModeGridProps {
  selectedMode: OrderMode['mode'];
  onSelectMode: (mode: OrderMode['mode']) => void;
}

export function OrderModeGrid({ selectedMode, onSelectMode }: OrderModeGridProps) {
  return (
    <div className="grid grid-cols-3 sm:grid-cols-3 md:grid-cols-5 lg:grid-cols-9 gap-2 sm:gap-2.5">
      {ORDER_MODES.map((mode) => (
        <button
          key={mode.mode}
          onClick={() => onSelectMode(mode.mode)}
          className={cn(
            "surface-card rounded-xl p-2.5 sm:p-3 text-left transition-all duration-200 border border-border/80 hover:border-border flex flex-col justify-between",
            selectedMode === mode.mode
              ? "border-primary bg-primary/10 text-foreground ring-1 ring-primary/40 shadow-sm"
              : "text-muted-foreground hover:text-foreground hover:bg-secondary/40"
          )}
        >
          <div className={cn("mb-1 sm:mb-1.5", mode.color)}>
            {mode.icon}
          </div>
          <div>
            <h3 className="font-semibold text-xs text-foreground truncate">{mode.name}</h3>
            <p className="text-[10px] text-muted-foreground mt-0.5 line-clamp-1 hidden md:block">
              {mode.example}
            </p>
          </div>
        </button>
      ))}
    </div>
  );
}

