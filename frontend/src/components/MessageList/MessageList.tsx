import { useEffect, useRef } from "react";
import { ChatMessage } from "@/store/chat";
import { Bot, User } from "lucide-react";
import { cn } from "@/lib/utils";
import { ConfirmationCard } from "@/components/ConfirmationCard";

interface MessageListProps {
  messages: ChatMessage[];
  isLoading: boolean;
}

export function MessageList({ messages, isLoading }: MessageListProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isLoading]);

  if (messages.length === 0 && !isLoading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-end pb-6 px-6 text-center">
        <div className="w-14 h-14 rounded-2xl bg-primary/10 flex items-center justify-center mb-4">
          <Bot className="w-7 h-7 text-primary" />
        </div>
        <h2 className="font-heading text-lg font-semibold mb-1">Olá! Sou o FinanceAI</h2>
        <p className="text-muted-foreground text-sm max-w-[260px]">
          Me diga seus gastos ou tire foto de uma nota fiscal. Eu cuido do resto!
        </p>
        <div className="mt-6 flex flex-col gap-2 w-full max-w-[280px]">
          {["Gastei 50 no mercado", "Qual meu saldo do mês?", "📸 Enviar nota fiscal"].map((hint) => (
            <div
              key={hint}
              className="px-4 py-2.5 rounded-xl bg-secondary text-secondary-foreground text-xs text-center"
            >
              {hint}
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">
      {messages.map((msg) => (
        <div
          key={msg.id}
          className={cn(
            "flex gap-2.5 animate-fade-in",
            msg.role === "user" ? "flex-row-reverse" : "flex-row"
          )}
        >
          <div
            className={cn(
              "w-7 h-7 rounded-lg flex items-center justify-center shrink-0 mt-0.5",
              msg.role === "user"
                ? "bg-primary/10 text-primary"
                : "bg-secondary text-muted-foreground"
            )}
          >
            {msg.role === "user" ? <User className="w-3.5 h-3.5" /> : <Bot className="w-3.5 h-3.5" />}
          </div>

          <div
            className={cn(
              "max-w-[80%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed",
              msg.role === "user"
                ? "bg-primary text-primary-foreground rounded-br-md"
                : "bg-secondary text-secondary-foreground rounded-bl-md"
            )}
          >
            {msg.imageUrl && (
              <img
                src={msg.imageUrl}
                alt="Nota fiscal"
                className="rounded-lg mb-2 max-h-48 object-cover"
              />
            )}
            <p className="whitespace-pre-wrap">{msg.displayContent ?? msg.content}</p>
          </div>
          {msg.role === "assistant" && msg.pendingAction && (
            <div className="ml-9.5 max-w-[80%]">
              <ConfirmationCard pendingAction={msg.pendingAction} />
            </div>
          )}
        </div>
      ))}

      {isLoading && (
        <div className="flex gap-2.5 animate-fade-in">
          <div className="w-7 h-7 rounded-lg bg-secondary flex items-center justify-center shrink-0">
            <Bot className="w-3.5 h-3.5 text-muted-foreground" />
          </div>
          <div className="bg-secondary rounded-2xl rounded-bl-md px-4 py-3 flex gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-pulse-dot" />
            <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-pulse-dot [animation-delay:0.2s]" />
            <span className="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-pulse-dot [animation-delay:0.4s]" />
          </div>
        </div>
      )}

      <div ref={bottomRef} />
    </div>
  );
}
