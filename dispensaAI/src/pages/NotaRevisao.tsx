import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ChevronDown, ChevronUp } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { ReceiptItemRow, receiptItemDisplayName } from "@/components/ReceiptItemRow";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useConfirmPantryReceipt, useDiscardPantryReceipt, usePantryReceipt } from "@/hooks/usePantryReceipts";
import { usePaymentMethods } from "@/hooks/usePaymentMethods";
import { intlLocale, useT } from "@/i18n";
import { useSettingsStore } from "@/store/settings";
import {
  ApiError,
  type PantryPaymentClarification,
  type PantryReceiptItem,
  type PantryReceiptItemIgnoreReason,
  type UpdatePantryReceiptItemInput,
} from "@/services/api";

function IgnoredItemsSection({
  items,
  onReactivate,
}: {
  items: PantryReceiptItem[];
  onReactivate: (itemId: string) => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);

  const reasonLabel = (reason: PantryReceiptItemIgnoreReason) =>
    reason ? t(`receipt.ignoreReason.${reason}`) : "";

  return (
    <div className="border-t border-border">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm text-muted-foreground"
      >
        <span>{t("receipt.review.ignoredSection", { n: items.length })}</span>
        {open ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
      </button>
      {open && (
        <div>
          {items.map((item) => (
            <div key={item.id} className="px-4 py-2 border-t border-border/50 flex items-center gap-3">
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate">{item.raw_name}</p>
                <p className="text-xs text-muted-foreground">{reasonLabel(item.ignore_reason)}</p>
              </div>
              <Button type="button" variant="ghost" size="sm" onClick={() => onReactivate(item.id)}>
                {t("receipt.review.reactivate")}
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function NotaRevisao() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const t = useT();
  const language = useSettingsStore((s) => s.language);

  const { receipt, items: fetchedItems, isLoading } = usePantryReceipt(id);
  const { paymentMethods } = usePaymentMethods();
  const confirmMutation = useConfirmPantryReceipt();
  const discardMutation = useDiscardPantryReceipt();

  const [items, setItems] = useState<PantryReceiptItem[] | null>(null);
  const [createTransaction, setCreateTransaction] = useState(true);
  const [paymentMethodId, setPaymentMethodId] = useState<string | undefined>(undefined);
  const [clarification, setClarification] = useState<PantryPaymentClarification | null>(null);

  const needsCountRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (items === null && fetchedItems) setItems(fetchedItems);
  }, [fetchedItems, items]);

  const patchItem = (itemId: string, patch: Partial<PantryReceiptItem>) => {
    setItems((prev) => prev?.map((it) => (it.id === itemId ? { ...it, ...patch } : it)) ?? prev);
  };

  const activeItems = items?.filter((it) => !it.ignored) ?? [];
  const ignoredItems = items?.filter((it) => it.ignored) ?? [];
  const pendingCountItems = activeItems.filter(
    (it) => it.is_produce && it.needs_unit_count && it.unit_count == null && !it.keep_weight,
  );
  const pendingCountIds = new Set(pendingCountItems.map((it) => it.id));
  const otherItems = activeItems.filter((it) => !pendingCountIds.has(it.id));

  const buildUpdateItems = (): UpdatePantryReceiptItemInput[] =>
    (items ?? []).map((it) => ({
      id: it.id,
      name_pt: it.name_pt,
      name_de: it.name_de,
      category: it.category,
      subcategory: it.subcategory,
      quantity: it.quantity,
      unit: it.unit,
      unit_count: it.unit_count,
      keep_weight: it.keep_weight,
      ignored: it.ignored,
    }));

  const confirmDisabled =
    !items || pendingCountItems.length > 0 || (createTransaction && !paymentMethodId) || confirmMutation.isPending;

  function runConfirm(overridePaymentMethodId?: string) {
    if (!id) return;
    confirmMutation.mutate(
      {
        id,
        input: {
          create_transaction: createTransaction,
          payment_method_id: overridePaymentMethodId ?? paymentMethodId,
          items: buildUpdateItems(),
        },
      },
      {
        onSuccess: () => {
          toast.success(t("receipt.review.confirmed"));
          navigate("/");
        },
        onError: (err) => {
          if (err instanceof ApiError) {
            const body = err.body as Record<string, unknown> | null;
            if (body && Array.isArray(body.pending_unit_count_item_ids)) {
              toast.error(t("receipt.review.pendingCountToast"));
              needsCountRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
              return;
            }
            if (body && Array.isArray((body as unknown as PantryPaymentClarification).options)) {
              setClarification(body as unknown as PantryPaymentClarification);
              return;
            }
            toast.error((body && typeof body.error === "string" && body.error) || err.message);
            return;
          }
          toast.error(err instanceof Error ? err.message : t("common.error"));
        },
      },
    );
  }

  function handleClarificationChoice(pmId: string) {
    setPaymentMethodId(pmId);
    setClarification(null);
    runConfirm(pmId);
  }

  function handleDiscard() {
    if (!id) return;
    if (!window.confirm(t("receipt.review.confirmDiscard"))) return;
    discardMutation.mutate(id, {
      onSuccess: () => navigate("/nota"),
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  }

  if (isLoading || !receipt || !items) {
    return (
      <div className="flex-1 min-h-0 flex flex-col">
        <PageHeader title={t("receipt.review.title")} showBack />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">
            {isLoading ? t("common.loading") : t("receipt.review.notFound")}
          </p>
        </div>
      </div>
    );
  }

  const dateFormatted = receipt.purchased_at
    ? new Intl.DateTimeFormat(intlLocale(language), { dateStyle: "medium" }).format(new Date(receipt.purchased_at))
    : "-";
  const totalFormatted =
    receipt.total != null
      ? new Intl.NumberFormat(intlLocale(language), { style: "currency", currency: receipt.currency }).format(
          receipt.total,
        )
      : "-";

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader title={t("receipt.review.title")} showBack />

      <div className="flex-1 overflow-y-auto">
        <div className="px-4 py-3 border-b border-border space-y-1">
          {receipt.merchant && <p className="font-heading text-lg font-semibold">{receipt.merchant}</p>}
          <div className="flex items-center gap-3 text-sm text-muted-foreground">
            <span>{dateFormatted}</span>
            <span>&middot;</span>
            <span>
              {t("receipt.review.total")}: {totalFormatted}
            </span>
          </div>
        </div>

        {pendingCountItems.length > 0 && (
          <div ref={needsCountRef} className="px-4 py-3 border-b border-border bg-warning/10 space-y-3">
            <p className="text-sm font-semibold">{t("receipt.review.needsCount")}</p>
            {pendingCountItems.map((item) => (
              <div key={item.id} className="space-y-2 pb-3 border-b border-border/50 last:border-0 last:pb-0">
                <p className="text-sm">
                  {t("receipt.review.unitCountPrompt", {
                    quantity: String(item.quantity),
                    unit: item.unit,
                    name: receiptItemDisplayName(item, language),
                  })}
                </p>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    inputMode="numeric"
                    min={0}
                    placeholder={t("receipt.review.unitCountPlaceholder")}
                    value={item.unit_count ?? ""}
                    onChange={(e) =>
                      patchItem(item.id, {
                        unit_count: e.target.value === "" ? undefined : Number(e.target.value),
                      })
                    }
                    className="h-10 rounded-lg w-28"
                  />
                  <Button type="button" variant="ghost" size="sm" onClick={() => patchItem(item.id, { keep_weight: true })}>
                    {t("receipt.review.keepWeight", { unit: item.unit })}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {otherItems.length > 0 && (
          <div>
            <p className="px-4 pt-3 pb-1 text-sm font-semibold text-muted-foreground">{t("receipt.review.items")}</p>
            {otherItems.map((item) => (
              <ReceiptItemRow key={item.id} item={item} language={language} onChange={(patch) => patchItem(item.id, patch)} />
            ))}
          </div>
        )}

        {ignoredItems.length > 0 && (
          <IgnoredItemsSection
            items={ignoredItems}
            onReactivate={(itemId) => patchItem(itemId, { ignored: false, ignore_reason: "" })}
          />
        )}
      </div>

      <div className="border-t border-border px-4 py-3 space-y-3 shrink-0 safe-bottom">
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm font-medium">{t("receipt.review.createTransaction")}</span>
          <Switch checked={createTransaction} onCheckedChange={setCreateTransaction} />
        </div>

        {createTransaction && (
          <Select value={paymentMethodId} onValueChange={setPaymentMethodId}>
            <SelectTrigger className="h-11 rounded-xl">
              <SelectValue placeholder={t("receipt.review.paymentMethodPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {paymentMethods?.map((pm) => (
                <SelectItem key={pm.id} value={pm.id}>
                  {pm.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        <Button
          type="button"
          className="w-full h-12 rounded-xl font-heading font-semibold"
          disabled={confirmDisabled}
          onClick={() => runConfirm()}
        >
          {t("receipt.review.confirm")}
        </Button>

        <Button
          type="button"
          variant="ghost"
          className="w-full h-10 text-destructive"
          disabled={discardMutation.isPending}
          onClick={handleDiscard}
        >
          {t("receipt.review.discard")}
        </Button>
      </div>

      <Dialog open={clarification !== null} onOpenChange={(open) => !open && setClarification(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("receipt.review.clarificationTitle")}</DialogTitle>
            <DialogDescription>{clarification?.prompt || t("receipt.review.clarificationHint")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {clarification?.options.map((opt) => (
              <Button
                key={opt.payment_method_id}
                type="button"
                variant="outline"
                className="w-full justify-start h-11 rounded-xl"
                onClick={() => handleClarificationChoice(opt.payment_method_id)}
              >
                {opt.label}
              </Button>
            ))}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
