import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type PantrySettings, type PantrySettingsPatch } from "@/services/api";
import { useSettingsStore } from "@/store/settings";
import { useAuthStore } from "@/store/auth";

export const PANTRY_SETTINGS_KEY = ["pantry", "settings"] as const;

/** Carrega as configurações do servidor e espelha no store local (idioma, país...). */
export function usePantrySettings() {
  const token = useAuthStore((s) => s.token);
  const setFromServer = useSettingsStore((s) => s.setFromServer);
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: PANTRY_SETTINGS_KEY,
    queryFn: api.getPantrySettings,
    enabled: !!token,
  });

  useEffect(() => {
    if (query.data) setFromServer(query.data);
  }, [query.data, setFromServer]);

  const mutation = useMutation({
    mutationFn: (patch: PantrySettingsPatch) => api.updatePantrySettings(patch),
    onSuccess: (data: PantrySettings) => {
      queryClient.setQueryData(PANTRY_SETTINGS_KEY, data);
      setFromServer(data);
    },
  });

  return { settings: query.data, isLoading: query.isLoading, error: query.error, update: mutation };
}
