import { create } from "zustand";
import type { PendingAction } from "@/services/api";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  displayContent?: string;
  createdAt: Date;
  imageUrl?: string;
  pendingAction?: PendingAction;
}

interface ChatState {
  messages: ChatMessage[];
  isLoading: boolean;
  resolvedActions: Record<string, "confirmed" | "cancelled">;
  addMessage: (msg: ChatMessage) => void;
  setLoading: (loading: boolean) => void;
  clearMessages: () => void;
  injectInsight: (body: string) => void;
  resolveAction: (id: string, resolution: "confirmed" | "cancelled") => void;
}

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  isLoading: false,
  resolvedActions: {},
  addMessage: (msg) => set((s) => ({ messages: [...s.messages, msg] })),
  setLoading: (isLoading) => set({ isLoading }),
  clearMessages: () => set({ messages: [], resolvedActions: {} }),
  injectInsight: (body: string) =>
    set((s) => ({
      messages: [
        ...s.messages,
        {
          id: crypto.randomUUID(),
          role: "assistant" as const,
          content: body,
          createdAt: new Date(),
        },
      ],
    })),
  resolveAction: (id, resolution) =>
    set((s) => ({ resolvedActions: { ...s.resolvedActions, [id]: resolution } })),
}));
