import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/services/api"

export function useAgentInsights(unreadOnly = false) {
  return useQuery({
    queryKey: ["agent-insights", unreadOnly],
    queryFn: () => api.getInsights(unreadOnly),
    staleTime: 5 * 60 * 1000,
  })
}

export function useMarkInsightRead() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.markInsightRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agent-insights"] })
    },
  })
}
