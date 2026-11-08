import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { DollarSign, MessageSquare, Camera, BarChart3, ArrowRight, Shield, Zap } from "lucide-react";

export default function Landing() {
  const navigate = useNavigate();

  return (
    <div className="min-h-[var(--app-height)] flex flex-col bg-background safe-top safe-bottom">
      {/* Header */}
      <header className="flex items-center justify-between px-5 py-4">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
            <DollarSign className="w-4 h-4 text-primary" />
          </div>
          <span className="font-heading text-sm font-bold">FinanceAI</span>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate("/login")}
          className="text-xs font-medium"
        >
          Entrar
        </Button>
      </header>

      {/* Hero */}
      <main className="flex-1 flex flex-col items-center justify-center px-6 text-center">
        <div className="w-20 h-20 rounded-3xl bg-primary/10 flex items-center justify-center mb-8 animate-fade-in">
          <DollarSign className="w-10 h-10 text-primary" />
        </div>

        <h1 className="font-heading text-4xl md:text-5xl font-bold tracking-tight mb-4 animate-fade-in max-w-md">
          Suas finanças com{" "}
          <span className="text-primary">inteligência</span>
        </h1>

        <p className="text-muted-foreground text-base max-w-sm mb-8 animate-fade-in leading-relaxed">
          Converse com a IA, tire foto de notas fiscais e tenha controle total dos seus gastos.
        </p>

        <Button
          size="lg"
          onClick={() => navigate("/login")}
          className="h-14 px-8 rounded-2xl font-heading font-semibold text-base gap-2 animate-fade-in"
        >
          Começar agora
          <ArrowRight className="w-5 h-5" />
        </Button>

        {/* Features */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-16 w-full max-w-lg animate-fade-in">
          {[
            { icon: MessageSquare, title: "Chat com IA", desc: "Registre gastos conversando" },
            { icon: Camera, title: "Foto de notas", desc: "OCR automático com IA" },
            { icon: BarChart3, title: "Dashboard", desc: "Visualize seus gastos" },
          ].map(({ icon: Icon, title, desc }) => (
            <div key={title} className="rounded-2xl bg-card p-4 text-left">
              <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center mb-3">
                <Icon className="w-5 h-5 text-primary" />
              </div>
              <h3 className="font-heading text-sm font-semibold mb-0.5">{title}</h3>
              <p className="text-xs text-muted-foreground">{desc}</p>
            </div>
          ))}
        </div>

        {/* Trust badges */}
        <div className="flex gap-4 mt-10 mb-8 animate-fade-in">
          {[
            { icon: Shield, label: "Dados seguros" },
            { icon: Zap, label: "Tempo real" },
          ].map(({ icon: Icon, label }) => (
            <span key={label} className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Icon className="w-3.5 h-3.5" />
              {label}
            </span>
          ))}
        </div>
      </main>
    </div>
  );
}
