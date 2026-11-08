import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { useAuth } from "@/hooks/useAuth";
import { useSettings } from "@/hooks/useSettings";
import { usePasskey } from "@/hooks/usePasskey";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";
import {
  LogOut, Shield, Bell, Tag, ScanFace, CheckCircle2,
  Store, Eye, EyeOff, KeyRound, PiggyBank, Target, ChevronRight, CalendarCheck, CreditCard,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { CurrencyToggle } from "@/components/CurrencyToggle/CurrencyToggle";
import { UserManualModal } from "@/components/UserManual/UserManualModal";
import { api } from "@/services/api";
import { toast } from "sonner";

const supportsPasskeys =
  typeof window !== "undefined" && !!window.PublicKeyCredential;

// ─── Modal: Editar Perfil ──────────────────────────────────────────────────

function EditProfileModal({
  open,
  onClose,
  onChangePassword,
  email,
}: {
  open: boolean;
  onClose: () => void;
  onChangePassword: () => void;
  email: string;
}) {
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Perfil</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div>
            <p className="text-xs text-muted-foreground mb-0.5">E-mail</p>
            <p className="text-sm font-medium">{email}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground mb-0.5">Tipo de conta</p>
            <p className="text-sm font-medium">Conta pessoal</p>
          </div>
        </div>
        <DialogFooter className="flex-col gap-2 sm:flex-row">
          <Button variant="outline" className="w-full sm:w-auto" onClick={onChangePassword}>
            Alterar senha →
          </Button>
          <DialogClose asChild>
            <Button variant="ghost" className="w-full sm:w-auto">Fechar</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ─── Modal: Alterar Senha ─────────────────────────────────────────────────

function ChangePasswordModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const reset = () => {
    setCurrent(""); setNext(""); setConfirm(""); setError(""); setSaving(false);
  };

  const handleClose = () => { reset(); onClose(); };

  const validate = (): string => {
    if (next.length < 8) return "Nova senha deve ter no mínimo 8 caracteres.";
    if (next === current) return "Nova senha deve ser diferente da senha atual.";
    if (next !== confirm) return "As senhas não coincidem.";
    return "";
  };

  const handleSave = async () => {
    const validationError = validate();
    if (validationError) { setError(validationError); return; }
    setSaving(true);
    setError("");
    try {
      await api.changePassword(current, next);
      toast.success("Senha alterada com sucesso");
      handleClose();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Erro ao alterar senha.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Alterar senha</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Senha atual</label>
            <Input
              type="password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              className="h-9"
              autoFocus
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Nova senha</label>
            <Input
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              className="h-9"
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Confirmar nova senha</label>
            <Input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              className="h-9"
              onKeyDown={(e) => { if (e.key === "Enter") handleSave(); }}
            />
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <DialogFooter className="flex-col gap-2 sm:flex-row">
          <Button
            onClick={handleSave}
            disabled={saving || !current || !next || !confirm}
            className="w-full sm:w-auto"
          >
            {saving ? "Salvando..." : "Salvar"}
          </Button>
          <Button variant="ghost" onClick={handleClose} className="w-full sm:w-auto">
            Cancelar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ─── Modal: Categorias ───────────────────────────────────────────────────

interface Category {
  slug: string;
  label: string;
  type: string;
  mode: string;
}

function CategoriesModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { categoryContext, loading, updateContext } = useSettings();
  const [allCategories, setAllCategories] = useState<Category[]>([]);

  useEffect(() => {
    if (open) {
      api.get<Category[]>("/categories").then(setAllCategories).catch(() => {});
    }
  }, [open]);

  const categories = allCategories.filter((c) => {
    if (c.type !== "expense") return false;
    if (categoryContext === "food_detail") return true;
    return c.mode === "standard";
  });

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Categorias ativas</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Modo alimentação detalhado</p>
              <p className="text-xs text-muted-foreground">
                {categoryContext === "food_detail"
                  ? "Categorias separadas por tipo de alimento"
                  : "Alimentação como categoria única"}
              </p>
            </div>
            <button
              onClick={() => updateContext(categoryContext === "standard" ? "food_detail" : "standard")}
              disabled={loading}
              className={cn(
                "relative w-11 h-6 rounded-full transition-colors shrink-0",
                categoryContext === "food_detail" ? "bg-primary" : "bg-muted"
              )}
            >
              <span
                className={cn(
                  "absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform",
                  categoryContext === "food_detail" ? "translate-x-5" : "translate-x-0"
                )}
              />
            </button>
          </div>
          {categories.length > 0 && (
            <div className="space-y-2">
              <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                {categories.length} categorias ativas
              </p>
              <div className="grid grid-cols-2 gap-1.5">
                {categories.map((c) => (
                  <span key={c.slug} className="text-xs bg-secondary rounded-lg px-2 py-1 text-center">
                    {c.label}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="ghost" className="w-full sm:w-auto">Fechar</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ─── Modal: Chave API ─────────────────────────────────────────────────────

function ApiKeyModal({
  open,
  onClose,
  userApiKey,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  userApiKey: string;
  onSaved: () => void;
}) {
  const { updateSetting } = useSettings();
  const [keyInput, setKeyInput] = useState("");
  const [keyVisible, setKeyVisible] = useState(false);
  const [keyEditing, setKeyEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);

  const maskKey = (key: string) => {
    if (!key) return "";
    if (key.length <= 14) return "****";
    return key.slice(0, 10) + "****..." + key.slice(-4);
  };

  const handleClose = () => {
    setKeyInput(""); setKeyVisible(false); setKeyEditing(false);
    setSaveError(false); setSaving(false);
    onClose();
  };

  const handleSave = async () => {
    if (!keyInput.trim()) return;
    setSaveError(false);
    setSaving(true);
    try {
      await updateSetting({ userApiKey: keyInput.trim() });
      setKeyInput(""); setKeyEditing(false);
      onSaved();
      handleClose();
    } catch {
      setSaveError(true);
    } finally {
      setSaving(false);
    }
  };

  const showInput = !userApiKey || keyEditing;

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Chave API</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <p className="text-xs text-muted-foreground">
            Chave Anthropic para usar o assistente financeiro.
          </p>
          {userApiKey && !keyEditing ? (
            <div className="flex items-center justify-between">
              <p className="text-xs font-mono text-muted-foreground">{maskKey(userApiKey)}</p>
              <button
                onClick={() => { setKeyEditing(true); setKeyInput(""); }}
                className="text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                Alterar
              </button>
            </div>
          ) : (
            <div className="relative">
              <Input
                type={keyVisible ? "text" : "password"}
                placeholder="sk-ant-..."
                value={keyInput}
                onChange={(e) => setKeyInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") handleSave(); }}
                className="pr-9 text-sm h-9"
                autoFocus={showInput}
              />
              <button
                type="button"
                onClick={() => setKeyVisible((v) => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                {keyVisible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          )}
          {saveError && (
            <p className="text-xs text-destructive">Erro ao salvar. Tente novamente.</p>
          )}
        </div>
        <DialogFooter className="flex-col gap-2 sm:flex-row">
          {showInput && (
            <Button
              onClick={handleSave}
              disabled={saving || !keyInput.trim()}
              className="w-full sm:w-auto"
            >
              {saving ? "Salvando..." : "Salvar"}
            </Button>
          )}
          <Button variant="ghost" onClick={handleClose} className="w-full sm:w-auto">
            {showInput ? "Cancelar" : "Fechar"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ─── Page ─────────────────────────────────────────────────────────────────

export default function Profile() {
  const { user, logout } = useAuth();
  const { categoryContext, showGoals, showLimitsCategories, loading, userApiKey, updateSetting } = useSettings();
  const { registerPasskey, isLoading: passkeyLoading, error: passkeyError } = usePasskey();
  const [passkeyDone, setPasskeyDone] = useState(false);
  const navigate = useNavigate();

  const [editProfileOpen, setEditProfileOpen] = useState(false);
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);
  const [categoriesOpen, setCategoriesOpen] = useState(false);
  const [apiKeyOpen, setApiKeyOpen] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);

  const handleRegisterPasskey = async () => {
    try {
      await registerPasskey();
      setPasskeyDone(true);
    } catch {
      // error shown via passkeyError
    }
  };

  const handleEditToChangePassword = () => {
    setEditProfileOpen(false);
    setTimeout(() => setChangePasswordOpen(true), 150);
  };

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Perfil" />

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-6 max-w-2xl mx-auto w-full">

        {/* ── CONTA ── */}
        <section>
          <h2 className="px-2 mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Conta
          </h2>
          <div className="rounded-xl bg-card border border-border overflow-hidden">
            <button
              onClick={() => setEditProfileOpen(true)}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                <span className="font-heading text-base font-bold text-primary">
                  {user?.email?.charAt(0).toUpperCase() ?? "U"}
                </span>
              </div>
              <div className="flex-1">
                <p className="text-sm font-semibold">{user?.email ?? "usuario@email.com"}</p>
                <p className="text-xs text-muted-foreground">Conta pessoal</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>
          </div>
        </section>

        {/* ── PREFERÊNCIAS ── */}
        <section>
          <h2 className="px-2 mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Preferências
          </h2>
          <div className="rounded-xl bg-card border border-border divide-y divide-border overflow-hidden">
            <CurrencyToggle />

            <div className="flex items-center gap-3 px-4 py-4">
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <Target className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Limites</p>
                <p className="text-xs text-muted-foreground">
                  {showLimitsCategories ? "Visível no menu" : "Oculto no menu"}
                </p>
              </div>
              <button
                onClick={() => updateSetting({ showLimitsCategories: !showLimitsCategories })}
                disabled={loading}
                className={cn(
                  "relative w-11 h-6 rounded-full transition-colors shrink-0",
                  showLimitsCategories ? "bg-primary" : "bg-muted"
                )}
              >
                <span
                  className={cn(
                    "absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform",
                    showLimitsCategories ? "translate-x-5" : "translate-x-0"
                  )}
                />
              </button>
            </div>

            <div className="flex items-center gap-3 px-4 py-4">
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <PiggyBank className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Reservas</p>
                <p className="text-xs text-muted-foreground">
                  {showGoals ? "Visível no menu" : "Oculto no menu"}
                </p>
              </div>
              <button
                onClick={() => updateSetting({ showGoals: !showGoals })}
                disabled={loading}
                className={cn(
                  "relative w-11 h-6 rounded-full transition-colors shrink-0",
                  showGoals ? "bg-primary" : "bg-muted"
                )}
              >
                <span
                  className={cn(
                    "absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform",
                    showGoals ? "translate-x-5" : "translate-x-0"
                  )}
                />
              </button>
            </div>

            <button
              onClick={() => setCategoriesOpen(true)}
              className="w-full flex items-center gap-3 px-4 py-4 text-left hover:bg-secondary/50 transition-colors"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <Tag className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Categorias ativas</p>
                <p className="text-xs text-muted-foreground">
                  {loading ? "Carregando..." : categoryContext === "food_detail" ? "Modo alimentação detalhado" : "Modo padrão"}
                </p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>
          </div>
        </section>

        {/* ── FERRAMENTAS E SEGURANÇA ── */}
        <section>
          <h2 className="px-2 mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Ferramentas e segurança
          </h2>
          <div className="rounded-xl bg-card border border-border divide-y divide-border overflow-hidden">
            <button
              onClick={() => navigate("/profile/payment-methods")}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <CreditCard className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Formas de Pagamento</p>
                <p className="text-xs text-muted-foreground">Cartões, débito e vouchers</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>

            <button
              onClick={() => navigate("/gastos-fixos")}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <CalendarCheck className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Gastos Fixos</p>
                <p className="text-xs text-muted-foreground">Contas recorrentes mensais</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>

            <button
              onClick={() => navigate("/profile/merchant-alias")}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <Store className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Dicionário de Mercados</p>
                <p className="text-xs text-muted-foreground">Gerencie apelidos para seus gastos</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>

            <button
              onClick={() => setApiKeyOpen(true)}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <KeyRound className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Chave API</p>
                <p className="text-xs text-muted-foreground">
                  {userApiKey ? "Chave configurada" : "Não configurada"}
                </p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>

            {supportsPasskeys && (
              <button
                onClick={handleRegisterPasskey}
                disabled={passkeyLoading || passkeyDone}
                className="w-full flex items-center gap-3 px-4 py-4 text-left hover:bg-secondary/50 transition-colors disabled:opacity-60"
              >
                <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                  <ScanFace className="w-4 h-4 text-muted-foreground" />
                </div>
                <div className="flex-1">
                  <p className="text-sm font-medium">Face ID</p>
                  <p className="text-xs text-muted-foreground">
                    {passkeyDone
                      ? "Face ID ativado com sucesso"
                      : passkeyLoading
                      ? "Aguarde..."
                      : passkeyError
                      ? passkeyError
                      : "Entrar sem senha no próximo acesso"}
                  </p>
                </div>
                {passkeyDone && <CheckCircle2 className="w-5 h-5 text-green-500 shrink-0" />}
              </button>
            )}

            <button
              onClick={() => setChangePasswordOpen(true)}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <Shield className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Alterar senha</p>
                <p className="text-xs text-muted-foreground">Segurança da conta</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>

            <div className="flex items-center gap-3 px-4 py-4">
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <Bell className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Notificações</p>
                <p className="text-xs text-muted-foreground">Lembretes de gastos</p>
              </div>
            </div>
          </div>
        </section>

        <Button
          variant="ghost"
          onClick={logout}
          className="w-full justify-start text-destructive hover:text-destructive hover:bg-destructive/10 rounded-xl h-12"
        >
          <LogOut className="w-4 h-4 mr-2" />
          Sair da conta
        </Button>

        {/* ── COMO USAR O SISTEMA ── */}
        <section>
          <h2 className="px-2 mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Como usar o sistema
          </h2>
          <div className="rounded-xl bg-card border border-border overflow-hidden">
            <button
              onClick={() => setManualOpen(true)}
              className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
            >
              <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                <span className="text-base">📖</span>
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">Manual do sistema</p>
                <p className="text-xs text-muted-foreground">8 funcionalidades · toque para abrir</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </button>
          </div>
        </section>

        <UserManualModal open={manualOpen} context="profile" onClose={() => setManualOpen(false)} />
      </div>

      <EditProfileModal
        open={editProfileOpen}
        onClose={() => setEditProfileOpen(false)}
        onChangePassword={handleEditToChangePassword}
        email={user?.email ?? ""}
      />
      <ChangePasswordModal
        open={changePasswordOpen}
        onClose={() => setChangePasswordOpen(false)}
      />
      <CategoriesModal
        open={categoriesOpen}
        onClose={() => setCategoriesOpen(false)}
      />
      <ApiKeyModal
        open={apiKeyOpen}
        onClose={() => setApiKeyOpen(false)}
        userApiKey={userApiKey}
        onSaved={() => {}}
      />

      <BottomNav />
    </div>
  );
}
