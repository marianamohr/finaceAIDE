import { useId, useState } from "react";
import { usePaymentMethods, PaymentMethod, PaymentMethodKind } from "@/hooks/usePaymentMethods";
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency";
import { usePrivacyStore } from "@/store/privacy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Pencil, PowerOff } from "lucide-react";
import { cn } from "@/lib/utils";

const KIND_LABELS: Record<PaymentMethodKind, string> = {
  credit_card: "Cartão de crédito",
  debit: "Débito",
  pix: "PIX",
  voucher: "Vale",
  cash: "Dinheiro",
};

interface FormState {
  name: string;
  kind: PaymentMethodKind;
  closing_day: string;
  due_day: string;
  credit_limit: string;
  last_four_digits: string;
  color: string;
}

const DEFAULT_FORM: FormState = {
  name: "",
  kind: "credit_card",
  closing_day: "",
  due_day: "",
  credit_limit: "",
  last_four_digits: "",
  color: "#6366f1",
};

function PaymentMethodRow({
  pm,
  onEdit,
  onDeactivate,
}: {
  pm: PaymentMethod;
  onEdit: (pm: PaymentMethod) => void;
  onDeactivate: (pm: PaymentMethod) => void;
}) {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  const hasBalance = pm.balance !== undefined && pm.kind === "cash"

  return (
    <div
      data-testid="payment-method-row"
      className="flex items-center gap-3 py-2.5 border-b border-border last:border-0"
    >
      <div
        data-testid={`color-chip-${pm.id}`}
        className="w-8 h-8 rounded-lg shrink-0"
        style={{ backgroundColor: pm.color || "#6366f1" }}
        aria-label={`Cor: ${pm.color}`}
      />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{pm.name}</p>
        {pm.kind === "credit_card" && pm.last_four_digits && (
          <p className="text-xs text-muted-foreground">••••{pm.last_four_digits}</p>
        )}
        <p className="text-xs text-muted-foreground">{KIND_LABELS[pm.kind]}</p>
      </div>
      {hasBalance && (
        <span className="text-xs font-medium px-2 py-0.5 rounded-full shrink-0 bg-green-500/10 text-green-500">
          {hidden ? maskCurrency(Math.max(pm.balance!, 0), currency) : formatCurrency(Math.max(pm.balance!, 0), currency)}
        </span>
      )}
      {!pm.active && (
        <span className="text-xs text-muted-foreground bg-secondary px-2 py-0.5 rounded-full shrink-0">
          Inativo
        </span>
      )}
      {pm.active && (
        <div className="flex gap-1 shrink-0">
          <button
            onClick={() => onEdit(pm)}
            aria-label={`Editar ${pm.name}`}
            className="w-7 h-7 flex items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground"
          >
            <Pencil className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => onDeactivate(pm)}
            aria-label={`Desativar ${pm.name}`}
            className="w-7 h-7 flex items-center justify-center rounded-lg bg-destructive/10 text-destructive"
          >
            <PowerOff className="w-3.5 h-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}

function PaymentMethodForm({
  initialValues,
  onSubmit,
  onCancel,
  submitLabel,
  saving,
  colorInputId,
}: {
  initialValues?: Partial<FormState>;
  onSubmit: (values: FormState) => Promise<void>;
  onCancel?: () => void;
  submitLabel: string;
  saving: boolean;
  colorInputId: string;
}) {
  const [form, setForm] = useState<FormState>({
    ...DEFAULT_FORM,
    ...initialValues,
  });
  const [error, setError] = useState<string | null>(null);

  const isCreditCard = form.kind === "credit_card";

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      setError("Nome é obrigatório.");
      return;
    }
    if (isCreditCard && !form.closing_day.trim()) {
      setError("Dia de fechamento é obrigatório para cartão de crédito.");
      return;
    }
    setError(null);
    await onSubmit(form);
  };

  const handleKindChange = (kind: PaymentMethodKind) => {
    setForm((prev) => ({
      ...prev,
      kind,
      closing_day: kind !== "credit_card" ? "" : prev.closing_day,
      due_day: kind !== "credit_card" ? "" : prev.due_day,
      credit_limit: kind !== "credit_card" ? "" : prev.credit_limit,
      last_four_digits: kind !== "credit_card" ? "" : prev.last_four_digits,
    }));
    setError(null);
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2 items-center">
        <label htmlFor={colorInputId} className="shrink-0 cursor-pointer">
          <div
            className="w-9 h-9 rounded-lg border border-border"
            style={{ backgroundColor: form.color }}
            title="Escolher cor"
          />
        </label>
        <input
          id={colorInputId}
          type="color"
          value={form.color}
          onChange={(e) => setForm((prev) => ({ ...prev, color: e.target.value }))}
          className="sr-only"
          aria-label="Cor"
        />
        <Input
          placeholder="Nome (ex: Nubank Crédito)"
          value={form.name}
          onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
          className="h-9 text-sm flex-1"
          aria-label="Nome"
          data-testid="name-input"
        />
      </div>

      <Select
        value={form.kind}
        onValueChange={(v) => handleKindChange(v as PaymentMethodKind)}
      >
        <SelectTrigger className="h-9 text-sm" aria-label="Tipo" data-testid="kind-select-trigger">
          <SelectValue placeholder="Tipo" />
        </SelectTrigger>
        <SelectContent>
          {(Object.entries(KIND_LABELS) as [PaymentMethodKind, string][]).map(([value, label]) => (
            <SelectItem key={value} value={value}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {isCreditCard && (
        <div className="grid grid-cols-3 gap-2" data-testid="credit-card-fields">
          <Input
            type="number"
            placeholder="Fechamento"
            value={form.closing_day}
            onChange={(e) => setForm((prev) => ({ ...prev, closing_day: e.target.value }))}
            min={1}
            max={31}
            className="h-9 text-sm"
            aria-label="Dia de fechamento"
            data-testid="closing-day-input"
          />
          <Input
            type="number"
            placeholder="Vencimento"
            value={form.due_day}
            onChange={(e) => setForm((prev) => ({ ...prev, due_day: e.target.value }))}
            min={1}
            max={31}
            className="h-9 text-sm"
            aria-label="Dia de vencimento"
            data-testid="due-day-input"
          />
          <Input
            type="number"
            placeholder="Limite (R$)"
            value={form.credit_limit}
            onChange={(e) => setForm((prev) => ({ ...prev, credit_limit: e.target.value }))}
            min={0}
            className="h-9 text-sm"
            aria-label="Limite de crédito"
            data-testid="credit-limit-input"
          />
        </div>
      )}

      {isCreditCard && (
        <Input
          type="text"
          inputMode="numeric"
          placeholder="4 últimos dígitos"
          value={form.last_four_digits}
          onChange={(e) => {
            const val = e.target.value.replace(/\D/g, "").slice(0, 4)
            setForm((prev) => ({ ...prev, last_four_digits: val }))
          }}
          maxLength={4}
          className="h-9 text-sm"
          aria-label="Últimos 4 dígitos"
          data-testid="last-four-digits-input"
        />
      )}

      {error && <p className="text-xs text-destructive" data-testid="form-error">{error}</p>}

      <div className="flex gap-2">
        {onCancel && (
          <Button variant="ghost" size="sm" onClick={onCancel} className="h-9">
            Cancelar
          </Button>
        )}
        <Button
          size="sm"
          onClick={handleSubmit}
          disabled={saving}
          className="flex-1 h-9"
          data-testid="submit-button"
        >
          {saving ? "Salvando..." : submitLabel}
        </Button>
      </div>
    </div>
  );
}


export function PaymentMethodManager() {
  const { paymentMethods, loading, createPaymentMethod, updatePaymentMethod, deletePaymentMethod } =
    usePaymentMethods();

  const [editTarget, setEditTarget] = useState<PaymentMethod | null>(null);
  const [deactivateTarget, setDeactivateTarget] = useState<PaymentMethod | null>(null);
  const [saving, setSaving] = useState(false);
  const [deactivating, setDeactivating] = useState(false);
  const [createKey, setCreateKey] = useState(0);

  const createColorId = useId();
  const editColorId = useId();

  const active = paymentMethods.filter((pm) => pm.active);
  const inactive = paymentMethods.filter((pm) => !pm.active);

  const handleCreate = async (form: FormState) => {
    setSaving(true);
    try {
      await createPaymentMethod({
        name: form.name.trim(),
        kind: form.kind,
        ...(form.kind === "credit_card" && form.closing_day
          ? { closing_day: parseInt(form.closing_day, 10) }
          : {}),
        ...(form.kind === "credit_card" && form.due_day
          ? { due_day: parseInt(form.due_day, 10) }
          : {}),
        ...(form.kind === "credit_card" && form.credit_limit
          ? { credit_limit: parseFloat(form.credit_limit) }
          : {}),
        last_four_digits: form.kind === "credit_card" && form.last_four_digits
          ? form.last_four_digits
          : null,
        color: form.color,
      });
      setCreateKey((k) => k + 1);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (form: FormState) => {
    if (!editTarget) return;
    setSaving(true);
    try {
      await updatePaymentMethod(editTarget.id, {
        name: form.name.trim(),
        kind: form.kind,
        closing_day:
          form.kind === "credit_card" && form.closing_day
            ? parseInt(form.closing_day, 10)
            : null,
        due_day:
          form.kind === "credit_card" && form.due_day
            ? parseInt(form.due_day, 10)
            : null,
        credit_limit:
          form.kind === "credit_card" && form.credit_limit
            ? parseFloat(form.credit_limit)
            : null,
        last_four_digits:
          form.kind === "credit_card" && form.last_four_digits
            ? form.last_four_digits
            : null,
        color: form.color,
      });
      setEditTarget(null);
    } finally {
      setSaving(false);
    }
  };

  const handleDeactivate = async () => {
    if (!deactivateTarget) return;
    setDeactivating(true);
    try {
      await deletePaymentMethod(deactivateTarget.id);
      setDeactivateTarget(null);
    } finally {
      setDeactivating(false);
    }
  };

  if (loading) return <p className="text-xs text-muted-foreground">Carregando...</p>;

  const editInitialValues: Partial<FormState> | undefined = editTarget
    ? {
        name: editTarget.name,
        kind: editTarget.kind,
        closing_day: editTarget.closing_day?.toString() ?? "",
        due_day: editTarget.due_day?.toString() ?? "",
        credit_limit: editTarget.credit_limit?.toString() ?? "",
        last_four_digits: editTarget.last_four_digits ?? "",
        color: editTarget.color || "#6366f1",
      }
    : undefined;

  return (
    <div className="space-y-4">
      {active.length > 0 && (
        <div>
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Formas cadastradas ({active.length})
          </p>
          <div className="rounded-lg bg-secondary/50 px-3">
            {active.map((pm) => (
              <PaymentMethodRow
                key={pm.id}
                pm={pm}
                onEdit={setEditTarget}
                onDeactivate={setDeactivateTarget}
              />
            ))}
          </div>
        </div>
      )}

      {inactive.length > 0 && (
        <div>
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Inativas ({inactive.length})
          </p>
          <div className="rounded-lg bg-secondary/50 px-3 opacity-60">
            {inactive.map((pm) => (
              <PaymentMethodRow
                key={pm.id}
                pm={pm}
                onEdit={setEditTarget}
                onDeactivate={setDeactivateTarget}
              />
            ))}
          </div>
        </div>
      )}

      {paymentMethods.length === 0 && (
        <p className="text-xs text-muted-foreground">
          Nenhuma forma de pagamento cadastrada. Adicione seu primeiro cartão ou conta abaixo.
        </p>
      )}

      <div className={cn("space-y-2", paymentMethods.length > 0 && "pt-2 border-t border-border")}>
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Adicionar
        </p>
        <PaymentMethodForm
          key={createKey}
          onSubmit={handleCreate}
          submitLabel="Adicionar"
          saving={saving}
          colorInputId={createColorId}
        />
      </div>

      <Dialog open={!!editTarget} onOpenChange={(open) => { if (!open) setEditTarget(null); }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Editar forma de pagamento</DialogTitle>
          </DialogHeader>
          {editTarget && editInitialValues && (
            <PaymentMethodForm
              initialValues={editInitialValues}
              onSubmit={handleUpdate}
              onCancel={() => setEditTarget(null)}
              submitLabel="Salvar"
              saving={saving}
              key={editTarget.id}
              colorInputId={editColorId}
            />
          )}
          <DialogFooter />
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={!!deactivateTarget}
        onOpenChange={(open) => { if (!open) setDeactivateTarget(null); }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Desativar forma de pagamento</AlertDialogTitle>
            <AlertDialogDescription>
              <strong>{deactivateTarget?.name}</strong> será desativada. Transações existentes
              serão preservadas.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancelar</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeactivate}
              disabled={deactivating}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              data-testid="confirm-deactivate"
            >
              {deactivating ? "Desativando..." : "Desativar"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
