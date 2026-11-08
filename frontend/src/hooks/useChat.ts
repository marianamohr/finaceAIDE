import { useCallback } from "react";
import { useChatStore, ChatMessage } from "@/store/chat";
import { api, PendingAction } from "@/services/api";

export function useChat() {
  const { messages, isLoading, addMessage, setLoading } = useChatStore();

  const sendMessage = useCallback(async (content: string) => {
    const userMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content,
      createdAt: new Date(),
    };
    addMessage(userMsg);
    setLoading(true);

    try {
      const data = await api.post<{ reply: string; pending_action?: PendingAction }>(
        "/chat",
        { message: content, client_datetime: new Date().toISOString() }
      );
      const assistantMsg: ChatMessage = {
        id: crypto.randomUUID(),
        role: "assistant",
        content: data.reply,
        createdAt: new Date(),
        pendingAction: data.pending_action ?? undefined,
      };
      addMessage(assistantMsg);
    } catch {
      addMessage({
        id: crypto.randomUUID(),
        role: "assistant",
        content: "Desculpe, ocorreu um erro. Tente novamente.",
        createdAt: new Date(),
      });
    } finally {
      setLoading(false);
    }
  }, [addMessage, setLoading]);

  const sendImage = useCallback(async (file: File, instruction?: string) => {
    const preview = URL.createObjectURL(file);
    console.log("[sendImage] file selected", {
      name: file.name,
      type: file.type,
      sizeKB: Math.round(file.size / 1024),
    });
    addMessage({
      id: crypto.randomUUID(),
      role: "user",
      content: instruction ? `📸 Nota fiscal enviada\n_"${instruction}"_` : "📸 Nota fiscal enviada",
      imageUrl: preview,
      createdAt: new Date(),
    });
    setLoading(true);

    try {
      console.log("[sendImage] calling api.uploadImage");
      const data = await api.uploadImage(file, instruction);
      const pendingAction: PendingAction | undefined = data.pending_action ?? undefined;
      const savedCount = data.transactions?.length ?? 0;
      console.log("[sendImage] success", { pendingAction, savedCount, imageUrl: data.image_url });

      // Três respostas possíveis do backend:
      //   - pending_action  → fluxo de confirmação (modo padrão)
      //   - transactions    → já salvou direto (modo food_detail, alta confiança)
      //   - nenhum dos dois → não identificou nada com confiança suficiente
      let content: string;
      if (pendingAction) {
        content = "Identifiquei transações na nota fiscal. Confirma para salvar?";
      } else if (savedCount > 0) {
        const total = data.transactions!.reduce((sum, t) => sum + Math.abs(t.amount), 0);
        const label = savedCount === 1 ? "transação" : "transações";
        content = `✓ Salvei ${savedCount} ${label} da nota fiscal — R$ ${total.toFixed(2).replace(".", ",")}.`;
      } else {
        content = "Não foi possível identificar transações com confiança suficiente.";
      }

      addMessage({
        id: crypto.randomUUID(),
        role: "assistant",
        content,
        createdAt: new Date(),
        pendingAction,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Erro desconhecido.";
      console.error("[sendImage] failed", err);
      addMessage({
        id: crypto.randomUUID(),
        role: "assistant",
        content: `Não consegui processar a imagem. ${message}`,
        createdAt: new Date(),
      });
    } finally {
      setLoading(false);
    }
  }, [addMessage, setLoading]);

  return { messages, isLoading, sendMessage, sendImage };
}
