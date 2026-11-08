import { useState, useEffect, useCallback } from "react";
import { api } from "@/services/api";

export interface MerchantAlias {
  id: string;
  raw_name: string;
  display_name: string;
  is_custom: boolean;
  created_at: string;
}

export function useMerchantAliases() {
  const [aliases, setAliases] = useState<MerchantAlias[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(() => {
    setLoading(true);
    api.get<MerchantAlias[]>("/merchant-aliases")
      .then((data) => { setAliases(data); setError(null); })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { fetch(); }, [fetch]);

  const createAlias = async (rawName: string, displayName: string) => {
    await api.post("/merchant-aliases", { raw_name: rawName, display_name: displayName });
    fetch();
  };

  const updateAlias = async (id: string, displayName: string) => {
    await api.put(`/merchant-aliases/${id}`, { display_name: displayName });
    fetch();
  };

  const deleteAlias = async (id: string) => {
    await api.delete(`/merchant-aliases/${id}`);
    fetch();
  };

  const applyAliases = async (): Promise<number> => {
    const result = await api.post<{ updated: number }>("/merchant-aliases/apply");
    return result.updated;
  };

  return { aliases, loading, error, createAlias, updateAlias, deleteAlias, applyAliases };
}
