import { MerchantAliasManager } from "@/components/MerchantAliasManager/MerchantAliasManager";
import { PageHeader } from "@/components/PageHeader/PageHeader";

export default function ProfileMerchantAlias() {
  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Dicionário de Mercados" showBack />
      <div className="flex-1 overflow-y-auto px-4 pb-4 pt-4 max-w-2xl mx-auto w-full">
        <MerchantAliasManager />
      </div>
    </div>
  );
}
