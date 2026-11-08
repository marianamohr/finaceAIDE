import { useState, useEffect } from "react";
import { api } from "@/services/api";
import { useAuthStore } from "@/store/auth";
import { useSettingsStore } from "@/store/settings";

export type CategoryContext = "standard" | "food_detail";

interface Settings {
  categoryContext: CategoryContext;
  showGoals: boolean;
  userApiKey: string;
  showLimitsCategories: boolean;
  showTools: boolean;
  onboardingSeen: boolean;
}

const defaults: Settings = {
  categoryContext: "standard",
  showGoals: true,
  userApiKey: "",
  showLimitsCategories: false,
  showTools: false,
  onboardingSeen: false,
};

export function useSettings() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const setShowGoalsStore = useSettingsStore((s) => s.setShowGoals);
  const setShowToolsStore = useSettingsStore((s) => s.setShowTools);
  const setShowLimitsCategoriesStore = useSettingsStore((s) => s.setShowLimitsCategories);
  const [settings, setSettings] = useState<Settings>(defaults);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated) return;
    api
      .getMe()
      .then((me) => {
        const next: Settings = {
          categoryContext: me.category_context as CategoryContext,
          showGoals: me.show_goals,
          userApiKey: me.user_api_key,
          showLimitsCategories: me.show_limits_categories,
          showTools: me.show_tools,
          onboardingSeen: me.onboarding_seen,
        };
        setSettings(next);
        setShowGoalsStore(next.showGoals);
        setShowToolsStore(next.showTools);
        setShowLimitsCategoriesStore(next.showLimitsCategories);
      })
      .catch(() => {/* keep defaults */})
      .finally(() => setLoading(false));
  }, [isAuthenticated, setShowGoalsStore, setShowToolsStore, setShowLimitsCategoriesStore]);

  const updateContext = async (ctx: CategoryContext) => {
    const prev = settings.categoryContext;
    setSettings((s) => ({ ...s, categoryContext: ctx }));
    try {
      await api.updateSettings({ category_context: ctx });
    } catch {
      setSettings((s) => ({ ...s, categoryContext: prev }));
    }
  };

  const updateSetting = async (patch: Partial<Omit<Settings, "categoryContext">>) => {
    const prev = settings;
    setSettings((s) => ({ ...s, ...patch }));
    if (patch.showGoals !== undefined) setShowGoalsStore(patch.showGoals);
    if (patch.showTools !== undefined) setShowToolsStore(patch.showTools);
    if (patch.showLimitsCategories !== undefined) setShowLimitsCategoriesStore(patch.showLimitsCategories);
    try {
      const body: {
        show_goals?: boolean;
        user_api_key?: string;
        show_limits_categories?: boolean;
        show_tools?: boolean;
        onboarding_seen?: boolean;
      } = {};
      if (patch.showGoals !== undefined) body.show_goals = patch.showGoals;
      if (patch.userApiKey !== undefined) body.user_api_key = patch.userApiKey;
      if (patch.showLimitsCategories !== undefined) body.show_limits_categories = patch.showLimitsCategories;
      if (patch.showTools !== undefined) body.show_tools = patch.showTools;
      if (patch.onboardingSeen !== undefined) body.onboarding_seen = patch.onboardingSeen;
      await api.updateSettings(body);
      // Re-fetch from server to replace optimistic state with real server state (gets masked API key)
      try {
        const me = await api.getMe();
        const next: Settings = {
          categoryContext: me.category_context as CategoryContext,
          showGoals: me.show_goals,
          userApiKey: me.user_api_key,
          showLimitsCategories: me.show_limits_categories,
          showTools: me.show_tools,
          onboardingSeen: me.onboarding_seen,
        };
        setSettings(next);
        setShowGoalsStore(next.showGoals);
        setShowToolsStore(next.showTools);
        setShowLimitsCategoriesStore(next.showLimitsCategories);
      } catch {
        // Re-fetch failed, keep optimistic state — not critical
      }
    } catch (err) {
      setSettings(prev);
      throw err;
    }
  };

  return {
    categoryContext: settings.categoryContext,
    showGoals: settings.showGoals,
    userApiKey: settings.userApiKey,
    showLimitsCategories: settings.showLimitsCategories,
    showTools: settings.showTools,
    onboardingSeen: settings.onboardingSeen,
    loading,
    updateContext,
    updateSetting,
  };
}
