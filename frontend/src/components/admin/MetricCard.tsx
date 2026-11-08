import { cn } from "@/lib/utils";

type MetricVariant = "primary" | "success" | "warning" | "destructive";

interface MetricCardProps {
  label: string;
  value: string | number;
  sub?: string;
  variant: MetricVariant;
}

const accentClass: Record<MetricVariant, string> = {
  primary:     "border-t-primary",
  success:     "border-t-success",
  warning:     "border-t-warning",
  destructive: "border-t-destructive",
};

const valueClass: Record<MetricVariant, string> = {
  primary:     "text-foreground",
  success:     "text-success",
  warning:     "text-warning",
  destructive: "text-destructive",
};

export function MetricCard({ label, value, sub, variant }: MetricCardProps) {
  return (
    <div className={cn(
      "rounded-xl bg-card border border-border border-t-2 p-4",
      accentClass[variant]
    )}>
      <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
        {label}
      </p>
      <p className={cn("font-heading text-2xl font-bold mt-1 tracking-tight", valueClass[variant])}>
        {value}
      </p>
      {sub && (
        <p className="text-[10px] text-muted-foreground mt-1">{sub}</p>
      )}
    </div>
  );
}
