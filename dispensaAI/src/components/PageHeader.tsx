import { ArrowLeft } from "lucide-react";
import { useNavigate } from "react-router-dom";

interface PageHeaderProps {
  title: string;
  showBack?: boolean;
  onBack?: () => void;
  right?: React.ReactNode;
}

export function PageHeader({ title, showBack, onBack, right }: PageHeaderProps) {
  const navigate = useNavigate();

  const handleBack = () => {
    if (onBack) onBack();
    else navigate(-1);
  };

  return (
    <header className="flex items-center gap-3 px-4 py-3 border-b border-border safe-top shrink-0">
      {showBack && (
        <button
          type="button"
          onClick={handleBack}
          aria-label="Voltar"
          className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-secondary transition-colors shrink-0"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
      )}
      <h1 className="flex-1 min-w-0 font-heading text-xl font-bold truncate">{title}</h1>
      {right}
    </header>
  );
}
