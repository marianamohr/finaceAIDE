import { useState } from "react";
import { useMerchantAliases, MerchantAlias } from "@/hooks/useMerchantAliases";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Trash2, Pencil, Check, X, Plus, RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";

function AliasRow({
  alias,
  onUpdate,
  onDelete,
}: {
  alias: MerchantAlias;
  onUpdate: (id: string, display: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}) {
  const [editing, setEditing] = useState(!alias.is_custom);
  const [value, setValue] = useState(alias.display_name);

  const handleSave = async () => {
    if (!value.trim()) return;
    await onUpdate(alias.id, value.trim());
    setEditing(false);
  };

  return (
    <div className="flex items-center gap-2 text-sm py-1.5 border-b border-border last:border-0">
      <div className="flex-1 min-w-0">
        <p className="text-xs text-muted-foreground truncate">{alias.raw_name}</p>
        {editing ? (
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className="h-7 text-sm mt-0.5"
            autoFocus
            placeholder="Nome a exibir"
            onKeyDown={(e) => {
              if (e.key === "Enter") handleSave();
              if (e.key === "Escape") { setValue(alias.display_name); setEditing(false); }
            }}
          />
        ) : (
          <p className="font-medium truncate">{alias.display_name}</p>
        )}
      </div>

      {editing ? (
        <div className="flex gap-1 shrink-0">
          <button
            onClick={handleSave}
            className="w-7 h-7 flex items-center justify-center rounded-lg bg-primary/10 text-primary"
          >
            <Check className="w-3.5 h-3.5" />
          </button>
          {alias.is_custom && (
            <button
              onClick={() => { setValue(alias.display_name); setEditing(false); }}
              className="w-7 h-7 flex items-center justify-center rounded-lg bg-secondary text-muted-foreground"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      ) : (
        <div className="flex gap-1 shrink-0">
          <button
            onClick={() => setEditing(true)}
            className="w-7 h-7 flex items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground"
          >
            <Pencil className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => onDelete(alias.id)}
            className="w-7 h-7 flex items-center justify-center rounded-lg bg-destructive/10 text-destructive"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}

export function MerchantAliasManager() {
  const { aliases, loading, createAlias, updateAlias, deleteAlias, applyAliases } = useMerchantAliases();

  const [newRaw, setNewRaw] = useState("");
  const [newDisplay, setNewDisplay] = useState("");
  const [saving, setSaving] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<number | null>(null);

  const handleApply = async () => {
    setApplying(true);
    setApplyResult(null);
    try {
      const updated = await applyAliases();
      setApplyResult(updated);
    } finally {
      setApplying(false);
    }
  };

  const pending = aliases.filter((a) => !a.is_custom);
  const customized = aliases.filter((a) => a.is_custom);

  const handleCreate = async () => {
    if (!newRaw.trim() || !newDisplay.trim()) return;
    setSaving(true);
    try {
      await createAlias(newRaw.trim(), newDisplay.trim());
      setNewRaw("");
      setNewDisplay("");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="text-xs text-muted-foreground">Carregando...</p>;

  return (
    <div className="space-y-4">
      {pending.length > 0 && (
        <div>
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Para renomear ({pending.length})
          </p>
          <div className="rounded-lg bg-secondary/50 px-3">
            {pending.map((alias) => (
              <AliasRow
                key={alias.id}
                alias={alias}
                onUpdate={updateAlias}
                onDelete={deleteAlias}
              />
            ))}
          </div>
        </div>
      )}

      {customized.length > 0 && (
        <div>
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Renomeados ({customized.length})
          </p>
          <div className="rounded-lg bg-secondary/50 px-3">
            {customized.map((alias) => (
              <AliasRow
                key={alias.id}
                alias={alias}
                onUpdate={updateAlias}
                onDelete={deleteAlias}
              />
            ))}
          </div>
        </div>
      )}

      {aliases.length === 0 && (
        <p className="text-xs text-muted-foreground">
          Nenhum estabelecimento registrado ainda. Os nomes aparecerão aqui automaticamente ao escanear notas fiscais.
        </p>
      )}

      {customized.length > 0 && (
        <div className="pt-2 border-t border-border">
          <Button
            variant="outline"
            size="sm"
            className="w-full h-9 text-sm"
            onClick={handleApply}
            disabled={applying}
          >
            <RefreshCw className={cn("w-3.5 h-3.5 mr-2", applying && "animate-spin")} />
            {applying ? "Aplicando..." : "Aplicar apelidos às transações existentes"}
          </Button>
          {applyResult !== null && (
            <p className="text-xs text-muted-foreground text-center mt-1">
              {applyResult === 0 ? "Nenhuma transação atualizada." : `${applyResult} transação(ões) atualizada(s).`}
            </p>
          )}
        </div>
      )}

      <div className={cn("space-y-2", aliases.length > 0 && "pt-2 border-t border-border")}>
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Adicionar manualmente</p>
        <Input
          placeholder="Nome na nota fiscal (razão social)"
          value={newRaw}
          onChange={(e) => setNewRaw(e.target.value)}
          className="h-9 text-sm"
        />
        <div className="flex gap-2">
          <Input
            placeholder="Nome a exibir"
            value={newDisplay}
            onChange={(e) => setNewDisplay(e.target.value)}
            className="h-9 text-sm"
            onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
          />
          <Button
            size="sm"
            onClick={handleCreate}
            disabled={saving || !newRaw.trim() || !newDisplay.trim()}
            className="shrink-0 h-9 px-3"
          >
            <Plus className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
