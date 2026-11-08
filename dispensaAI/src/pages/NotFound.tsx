import { Link } from "react-router-dom";
import { useT } from "@/i18n";

export default function NotFound() {
  const t = useT();
  return (
    <div className="h-[var(--app-height)] flex flex-col items-center justify-center gap-3 bg-background px-6 text-center">
      <p className="font-heading text-2xl font-bold">404</p>
      <p className="text-sm text-muted-foreground">{t("notFound.title")}</p>
      <Link to="/" className="text-sm text-primary underline">{t("notFound.back")}</Link>
    </div>
  );
}
