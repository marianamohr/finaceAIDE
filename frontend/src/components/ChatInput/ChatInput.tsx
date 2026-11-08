import { useState, useRef } from "react";
import { Send, Camera, Image as ImageIcon, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCamera } from "@/hooks/useCamera";
import { resizeIfNeeded } from "@/lib/resizeImage";

interface ChatInputProps {
  onSendMessage: (text: string) => void;
  onSendImage: (file: File, instruction?: string) => void;
  disabled?: boolean;
}

export function ChatInput({ onSendMessage, onSendImage, disabled }: ChatInputProps) {
  const [text, setText] = useState("");
  const [pendingImage, setPendingImage] = useState<{ file: File; preview: string } | null>(null);
  const [instruction, setInstruction] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const { openCamera, pickImage } = useCamera();

  const handleSend = () => {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSendMessage(trimmed);
    setText("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const prepareImage = async (file: File) => {
    try {
      const ready = await resizeIfNeeded(file);
      const preview = URL.createObjectURL(ready);
      setPendingImage({ file: ready, preview });
      setInstruction("");
    } catch (err) {
      console.error("[prepareImage] resize failed", err);
      // Fall back to original file if resize fails
      const preview = URL.createObjectURL(file);
      setPendingImage({ file, preview });
      setInstruction("");
    }
  };

  const handleCamera = async () => {
    const file = await openCamera();
    if (!file) return;
    await prepareImage(file);
  };

  const handlePickImage = async () => {
    const file = await pickImage();
    if (!file) return;
    await prepareImage(file);
  };

  const handleSendImage = () => {
    if (!pendingImage) return;
    onSendImage(pendingImage.file, instruction.trim() || undefined);
    URL.revokeObjectURL(pendingImage.preview);
    setPendingImage(null);
    setInstruction("");
  };

  const handleCancelImage = () => {
    if (pendingImage) URL.revokeObjectURL(pendingImage.preview);
    setPendingImage(null);
    setInstruction("");
  };

  const handleInput = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 120) + "px";
  };

  return (
    <div className="border-t border-border bg-background px-4 pt-3 pb-3">
      {pendingImage && (
        <div className="max-w-2xl mx-auto mb-3 rounded-xl bg-secondary p-3 flex flex-col gap-2">
          <div className="flex items-start gap-3">
            <img
              src={pendingImage.preview}
              alt="preview"
              className="w-16 h-16 rounded-lg object-cover shrink-0"
            />
            <div className="flex-1 min-w-0">
              <p className="text-xs text-muted-foreground mb-1">Instrução opcional (ex: "categorizar como presentes")</p>
              <textarea
                value={instruction}
                onChange={(e) => setInstruction(e.target.value)}
                placeholder="Deixe em branco para categorização automática..."
                rows={2}
                className="w-full resize-none rounded-lg bg-background border-0 px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              />
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0 text-muted-foreground"
              onClick={handleCancelImage}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
          <Button
            type="button"
            className="w-full rounded-xl"
            onClick={handleSendImage}
            disabled={disabled}
          >
            <Send className="w-4 h-4 mr-2" />
            Enviar nota
          </Button>
        </div>
      )}

      <div className="flex items-end gap-2 max-w-2xl mx-auto">
        <div className="flex gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-10 w-10 rounded-xl shrink-0 text-muted-foreground"
            onClick={handleCamera}
            disabled={disabled}
          >
            <Camera className="w-5 h-5" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-10 w-10 rounded-xl shrink-0 text-muted-foreground"
            onClick={handlePickImage}
            disabled={disabled}
          >
            <ImageIcon className="w-5 h-5" />
          </Button>
        </div>

        <div className="flex-1 relative">
          <textarea
            ref={textareaRef}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onInput={handleInput}
            onKeyDown={handleKeyDown}
            placeholder="Diga quanto gastou..."
            disabled={disabled}
            rows={1}
            className="w-full resize-none rounded-xl bg-secondary border-0 px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          />
        </div>

        <Button
          type="button"
          size="icon"
          className="h-10 w-10 rounded-xl shrink-0"
          onClick={handleSend}
          disabled={disabled || !text.trim()}
        >
          <Send className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );
}
