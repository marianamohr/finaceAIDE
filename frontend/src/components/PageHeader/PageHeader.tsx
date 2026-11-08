import { User, ArrowLeft, Eye, EyeOff } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";
import { usePendingAliasCount } from "@/hooks/usePendingAliasCount";
import { usePrivacyStore } from "@/store/privacy";

interface PageHeaderProps {
  title?: string;
  showBack?: boolean;
  onBack?: () => void;
  /** When true, renders the logo slot instead of a plain title */
  logoSlot?: React.ReactNode;
}

export function PageHeader({ title, showBack, onBack, logoSlot }: PageHeaderProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const pendingAliasCount = usePendingAliasCount();
  const isProfile = location.pathname.startsWith("/profile");
  const valuesHidden = usePrivacyStore((s) => s.valuesHidden);
  const toggleValuesHidden = usePrivacyStore((s) => s.toggleValuesHidden);

  const handleBack = () => {
    if (onBack) onBack();
    else navigate(-1);
  };

  return (
    <header className="flex items-center gap-3 px-4 py-3 border-b border-border safe-top">
      {showBack ? (
        <button
          onClick={handleBack}
          className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-secondary transition-colors shrink-0"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
      ) : null}

      <div className="flex-1 min-w-0">
        {logoSlot ?? (
          <h1 className="font-heading text-xl font-bold truncate">{title}</h1>
        )}
      </div>

      <button
        onClick={toggleValuesHidden}
        aria-label={valuesHidden ? "Mostrar valores" : "Ocultar valores"}
        aria-pressed={valuesHidden}
        className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-secondary transition-colors shrink-0"
      >
        {valuesHidden ? (
          <EyeOff className="w-5 h-5 text-muted-foreground" />
        ) : (
          <Eye className="w-5 h-5 text-muted-foreground" />
        )}
      </button>

      {!isProfile && (
        <div className="flex items-center gap-1 lg:hidden">
          <button
            onClick={() => navigate("/profile")}
            className="relative w-8 h-8 flex items-center justify-center rounded-lg hover:bg-secondary transition-colors shrink-0"
          >
            <User className="w-5 h-5 text-muted-foreground" />
            {pendingAliasCount > 0 && (
              <span className={cn(
                "absolute top-0.5 right-0.5 w-2 h-2 rounded-full bg-destructive"
              )} />
            )}
          </button>
        </div>
      )}
    </header>
  );
}
