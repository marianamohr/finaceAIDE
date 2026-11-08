import { useState } from "react";
import type { ReactNode } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface UserManualProps {
  mode: "modal" | "section";
  context?: "onboarding" | "profile";
  onClose?: () => void;
  onViewFull?: () => void;
}

interface SectionProps {
  icon: string;
  title: string;
  children: ReactNode;
}

function ManualSection({ icon, title, children }: SectionProps) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-base shrink-0 mt-0.5">{icon}</span>
      <div className="min-w-0">
        <p className="text-sm font-semibold">{title}</p>
        <div className="text-xs text-muted-foreground leading-relaxed mt-0.5">{children}</div>
      </div>
    </div>
  );
}

export function UserManual({ mode, context = "onboarding", onClose, onViewFull }: UserManualProps) {
  const isSection = mode === "section";
  const [expanded, setExpanded] = useState(!isSection);

  const content = (
    <div className={cn("flex flex-col gap-4", isSection && "px-1 pb-2 pt-1")}>
      {/* Por onde começar */}
      <div className="rounded-lg bg-amber-50 border border-amber-200 dark:bg-amber-950/30 dark:border-amber-800 p-3">
        <p className="text-xs font-bold mb-2">🚀 Por onde começar</p>
        <ol className="text-xs text-muted-foreground space-y-1.5 list-none">
          <li><span className="font-semibold text-foreground">1. Chave API</span> — Perfil → Chave API. Necessária para ativar o Chat. Veja o passo a passo na seção abaixo.</li>
          <li><span className="font-semibold text-foreground">2. Formas de Pagamento</span> — Perfil → Formas de Pagamento. Já criamos um <em>Débito</em> pra você. Adicione seus cartões.</li>
          <li><span className="font-semibold text-foreground">3. Gastos Fixos</span> — Perfil → Gastos Fixos. Aluguel, streaming, mensalidades.</li>
          <li><span className="font-semibold text-foreground">4. Limites</span> — Menu → Limites. Tetos de gasto por categoria.</li>
          <li><span className="font-semibold text-foreground">5. Reservas</span> — Menu → Reservas. Metas de poupança.</li>
        </ol>
      </div>

      {/* Funcionalidades */}
      <ManualSection icon="🔑" title="Chave API">
        <p>Para usar o Chat, você precisa de uma chave Anthropic:</p>
        <ol className="mt-1 space-y-0.5 list-decimal list-inside">
          <li>Acesse <strong>console.anthropic.com</strong> e crie uma conta</li>
          <li>No menu lateral, clique em <strong>"API Keys"</strong></li>
          <li>Clique em <strong>"Create Key"</strong>, dê um nome e confirme</li>
          <li>Copie a chave — começa com <code className="font-mono">sk-ant-...</code></li>
          <li>Cole em <strong>Perfil → Chave API</strong> e salve</li>
        </ol>
        <p className="mt-1 italic">A chave é exibida uma única vez. Se perder, gere outra.</p>
      </ManualSection>

      <ManualSection icon="💬" title="Chat financeiro">
        Registre gastos em linguagem natural. <em>"Gastei 45 no iFood no Nubank"</em> já salva, categoriza e vincula ao cartão.
        Sempre mencione a forma de pagamento: cartão de crédito vai pra fatura, vale sai do saldo do vale.
        Esqueceu? Corrija depois na tela Consumo.
      </ManualSection>

      <ManualSection icon="📊" title="Dashboard">
        Gráficos de gastos por categoria, comparação entre meses e evolução temporal. Prepare o coração antes de abrir.
      </ManualSection>

      <ManualSection icon="🧾" title="Consumo">
        Lista de todas as transações. Edite, delete ou corrija qualquer lançamento — inclusive a forma de pagamento esquecida.
      </ManualSection>

      <ManualSection icon="💰" title="Caixa">
        <p>Saldo em conta, faturas de cartão e dois recursos especiais:</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Saldo do mês anterior:</span> se sobrou dinheiro, o sistema pergunta se quer trazer pro mês atual.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Depósito de vale (VR/VA):</span> no início do mês, confirme quanto entrou no seu cartão de alimentação.</p>
      </ManualSection>

      <ManualSection icon="💳" title="Formas de Pagamento">
        <p><span className="font-semibold text-foreground">Cartão de crédito:</span> informe dia de fechamento e vencimento — o sistema monta a fatura.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Vale (VR/VA):</span> gastos saem do saldo do vale, não do seu bolso.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Débito/PIX:</span> débita direto da conta. Já existe um <em>Débito</em> criado pra você.</p>
      </ManualSection>

      <ManualSection icon="🎯" title="Limites">
        Tetos de gasto por categoria. Amarelo em 70%, vermelho ao estourar. O sistema avisa — como um amigo que sabe que você não vai ouvir, mas avisa assim mesmo.
      </ManualSection>

      <ManualSection icon="🐷" title="Reservas">
        <p>Metas de poupança com nome, valor-alvo e prazo. Aportes, rendimentos e saques também funcionam pelo chat.</p>
        <p className="mt-1 font-semibold text-foreground">⚠️ Atenção:</p>
        <p>Todo aporte sai do seu saldo em conta. Antes de um aporte grande, confira o saldo no Caixa — se não cobrir, o caixa fica negativo.</p>
      </ManualSection>

      <ManualSection icon="📅" title="Gastos Fixos">
        <p>Dois tipos na mesma tela:</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Gasto fixo</span> (aluguel, internet, academia): aparece como lembrete mensal no Caixa — você registra o pagamento manualmente.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Assinatura de cartão</span> (Netflix, Spotify): vinculada a um cartão de crédito, entra na fatura automaticamente.</p>
        <p className="mt-1 italic">Regra: se você paga, é gasto fixo. Se o cartão cobra, é assinatura.</p>
      </ManualSection>
    </div>
  );

  if (isSection) {
    return (
      <div id="manual" className="rounded-xl bg-card border border-border overflow-hidden">
        <button
          onClick={() => setExpanded((e) => !e)}
          className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
        >
          <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
            <span className="text-base">📖</span>
          </div>
          <div className="flex-1">
            <p className="text-sm font-semibold">Manual do sistema</p>
            <p className="text-xs text-muted-foreground">8 funcionalidades · toque para {expanded ? "fechar" : "expandir"}</p>
          </div>
          {expanded
            ? <ChevronUp className="w-4 h-4 text-muted-foreground shrink-0" />
            : <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
          }
        </button>
        {expanded && (
          <div className="border-t border-border px-4 py-4">
            {content}
          </div>
        )}
      </div>
    );
  }

  // mode === "modal": renderiza conteúdo + botões de ação
  return (
    <div className="flex flex-col gap-4 py-2">
      {content}
      <div className="flex gap-2 justify-end pt-2">
        {context === "onboarding" && (
          <Button variant="outline" size="sm" onClick={onViewFull}>
            Ver manual completo
          </Button>
        )}
        <Button size="sm" onClick={onClose}>
          {context === "profile" ? "Fechar" : "Começar →"}
        </Button>
      </div>
    </div>
  );
}