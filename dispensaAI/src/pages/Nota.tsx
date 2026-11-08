import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Camera, Image, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useCamera } from "@/hooks/useCamera";
import { useCreatePantryReceipt } from "@/hooks/usePantryReceipts";
import { useT } from "@/i18n";
import { resizeIfNeeded } from "@/lib/resizeImage";
import { cn } from "@/lib/utils";
import type { PantryCountry } from "@/services/api";

type CountryChoice = PantryCountry | "auto";

const COUNTRY_CHOICES: CountryChoice[] = ["auto", "BR", "DE"];

function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "shrink-0 px-3 py-1.5 rounded-full text-sm border transition-colors",
        active
          ? "bg-primary text-primary-foreground border-transparent"
          : "bg-background text-foreground border-border",
      )}
    >
      {children}
    </button>
  );
}

export default function Nota() {
  const t = useT();
  const navigate = useNavigate();
  const { openCamera, pickImage } = useCamera();
  const createReceipt = useCreatePantryReceipt();

  const [file, setFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [instruction, setInstruction] = useState("");
  const [country, setCountry] = useState<CountryChoice>("auto");

  useEffect(() => {
    if (!file) {
      setPreviewUrl(null);
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const handlePick = async (picker: () => Promise<File | null>) => {
    const picked = await picker();
    if (!picked) return;
    try {
      const resized = await resizeIfNeeded(picked);
      setFile(resized);
    } catch {
      toast.error(t("common.error"));
    }
  };

  const handleCancel = () => setFile(null);

  const handleSend = () => {
    if (!file) return;
    createReceipt.mutate(
      {
        image: file,
        instruction: instruction.trim() || undefined,
        country: country === "auto" ? undefined : country,
      },
      {
        onSuccess: ({ receipt }) => navigate(`/nota/${receipt.id}`),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  if (createReceipt.isPending) {
    return (
      <div className="flex-1 min-h-0 flex flex-col">
        <PageHeader title={t("receipt.title")} />
        <div className="flex-1 flex flex-col items-center justify-center px-8 text-center gap-3">
          <Loader2 className="w-8 h-8 text-primary animate-spin" />
          <p className="text-sm text-muted-foreground">{t("receipt.processing")}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader title={t("receipt.title")} />
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-6">
        {previewUrl ? (
          <div className="space-y-4">
            <img
              src={previewUrl}
              alt={t("receipt.previewAlt")}
              className="w-full max-h-80 object-contain rounded-xl border border-border bg-secondary/30"
            />
            <div className="flex gap-2">
              <Button type="button" variant="outline" className="flex-1 h-11 rounded-xl" onClick={handleCancel}>
                {t("common.cancel")}
              </Button>
              <Button type="button" className="flex-1 h-11 rounded-xl font-semibold" onClick={handleSend}>
                {t("receipt.send")}
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <Button
              type="button"
              className="w-full h-16 rounded-xl text-base font-semibold gap-3"
              onClick={() => handlePick(openCamera)}
            >
              <Camera className="w-5 h-5" />
              {t("receipt.take")}
            </Button>
            <Button
              type="button"
              variant="outline"
              className="w-full h-16 rounded-xl text-base font-semibold gap-3"
              onClick={() => handlePick(pickImage)}
            >
              <Image className="w-5 h-5" />
              {t("receipt.pick")}
            </Button>
          </div>
        )}

        <div className="space-y-2">
          <p className="text-sm font-medium">{t("receipt.country.label")}</p>
          <div className="flex gap-2 overflow-x-auto pb-1">
            {COUNTRY_CHOICES.map((c) => (
              <Chip key={c} active={country === c} onClick={() => setCountry(c)}>
                {c === "auto" ? t("receipt.country.auto") : t(`settings.country.${c}`)}
              </Chip>
            ))}
          </div>
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">{t("receipt.instructionLabel")}</p>
          <Textarea
            value={instruction}
            onChange={(e) => setInstruction(e.target.value)}
            placeholder={t("receipt.instructionPlaceholder")}
            className="rounded-xl"
            rows={3}
          />
        </div>
      </div>
    </div>
  );
}
