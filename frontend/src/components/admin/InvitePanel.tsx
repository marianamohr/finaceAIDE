import { useState } from "react";
import { api } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Copy, X } from "lucide-react";

interface InvitePanelProps {
  onClose: () => void;
}

export function InvitePanel({ onClose }: InvitePanelProps) {
  const [email, setEmail] = useState("");
  const [link, setLink] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;
    setLoading(true);
    setLink(null);
    try {
      const data = await api.post<{ link: string }>("/admin/invites", { email });
      setLink(data.link);
      setEmail("");
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  };

  const copyLink = () => {
    if (!link) return;
    navigator.clipboard.writeText(link);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="rounded-xl bg-card border border-border p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-semibold">Convidar usuário</p>
        <button onClick={onClose} className="text-muted-foreground hover:text-foreground transition-colors">
          <X className="w-4 h-4" />
        </button>
      </div>
      <form onSubmit={handleSubmit} className="flex gap-2">
        <Input
          type="email"
          placeholder="email@exemplo.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="h-9 text-sm bg-secondary border-0"
        />
        <Button type="submit" size="sm" disabled={loading} className="shrink-0">
          {loading ? "Gerando…" : "Gerar link"}
        </Button>
      </form>
      {link && (
        <div className="flex items-center gap-2 rounded-lg bg-secondary px-3 py-2">
          <p className="text-xs text-muted-foreground truncate flex-1">{link}</p>
          <button onClick={copyLink} className="shrink-0 text-primary hover:text-primary/80 transition-colors">
            <Copy className="w-4 h-4" />
          </button>
          {copied && <span className="text-xs text-primary">Copiado!</span>}
        </div>
      )}
    </div>
  );
}
