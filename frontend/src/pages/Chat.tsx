import { useState, useEffect } from "react";
import { useChat } from "@/hooks/useChat";
import { ChatInput } from "@/components/ChatInput/ChatInput";
import { MessageList } from "@/components/MessageList/MessageList";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { UserManualModal } from "@/components/UserManual/UserManualModal";
import { DollarSign, KeyRound } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useSettings } from "@/hooks/useSettings";

export default function Chat() {
  const { messages, isLoading, sendMessage, sendImage } = useChat();
  const { userApiKey, loading: settingsLoading, onboardingSeen, updateSetting } = useSettings();
  const navigate = useNavigate();
  const [manualOpen, setManualOpen] = useState(false);

  useEffect(() => {
    if (!settingsLoading && !onboardingSeen) setManualOpen(true);
  }, [settingsLoading, onboardingSeen]);

  const handleCloseManual = () => {
    setManualOpen(false);
    updateSetting({ onboardingSeen: true });
  };

  const handleViewFull = () => {
    handleCloseManual();
    navigate("/profile#manual");
  };

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader
        logoSlot={
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
              <DollarSign className="w-4 h-4 text-primary" />
            </div>
            <div>
              <p className="font-heading text-sm font-semibold">FinanceAI</p>
              <p className="text-[10px] text-muted-foreground">Assistente financeiro</p>
            </div>
          </div>
        }
      />

      {!settingsLoading && !userApiKey ? (
        <div className="flex-1 flex flex-col items-center justify-center gap-4 px-6 text-center">
          <div className="w-14 h-14 rounded-2xl bg-primary/10 flex items-center justify-center">
            <KeyRound className="w-7 h-7 text-primary" />
          </div>
          <div className="space-y-1">
            <p className="font-heading text-base font-semibold">Configure sua chave API</p>
            <p className="text-sm text-muted-foreground">
              Para usar o assistente, você precisa adicionar sua chave da Anthropic no perfil.
            </p>
          </div>
          <button
            onClick={() => navigate("/profile")}
            className="text-sm px-4 h-9 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            Ir para o Perfil
          </button>
        </div>
      ) : (
        <>
          {/* Messages */}
          <MessageList messages={messages} isLoading={isLoading} />

          {/* Input */}
          <ChatInput
            onSendMessage={sendMessage}
            onSendImage={sendImage}
            disabled={isLoading}
          />
        </>
      )}

      {/* Bottom Nav */}
      <BottomNav />

      <UserManualModal open={manualOpen} onClose={handleCloseManual} onViewFull={handleViewFull} />
    </div>
  );
}
