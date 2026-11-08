import { useCallback, useEffect, useState } from "react"
import { api } from "@/services/api"

export interface LimitWithProgress {
  id: string
  category_id: string
  category_slug: string
  monthly_limit: number
  spent: number
  pct: number
}

export interface GroupWithProgress {
  id: string
  label: string
  monthly_limit: number
  spent: number
  pct: number
  members: LimitWithProgress[]
}

interface LimitsResponse {
  individual: LimitWithProgress[]
  groups: GroupWithProgress[]
}

export interface GroupMemberInput {
  category_id: string
  monthly_limit: number
}

export function useLimits() {
  const [individual, setIndividual] = useState<LimitWithProgress[]>([])
  const [groups, setGroups] = useState<GroupWithProgress[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const fetchLimits = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await api.get<LimitsResponse>("/account/limits")
      setIndividual(data?.individual ?? [])
      setGroups(data?.groups ?? [])
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLimits()
  }, [fetchLimits])

  useEffect(() => {
    window.addEventListener("focus", fetchLimits)
    return () => window.removeEventListener("focus", fetchLimits)
  }, [fetchLimits])

  const upsert = useCallback(
    async (categoryId: string, monthlyLimit: number) => {
      await api.put(`/account/limits/${categoryId}`, { monthly_limit: monthlyLimit })
      await fetchLimits()
    },
    [fetchLimits]
  )

  const remove = useCallback(
    async (categoryId: string) => {
      await api.delete(`/account/limits/${categoryId}`)
      await fetchLimits()
    },
    [fetchLimits]
  )

  const createGroup = useCallback(
    async (label: string, members: GroupMemberInput[]) => {
      await api.post("/account/limits/groups", { label, members })
      await fetchLimits()
    },
    [fetchLimits]
  )

  const updateGroup = useCallback(
    async (groupId: string, label: string, members: GroupMemberInput[]) => {
      await api.put(`/account/limits/groups/${groupId}`, { label, members })
      await fetchLimits()
    },
    [fetchLimits]
  )

  const removeGroup = useCallback(
    async (groupId: string) => {
      await api.delete(`/account/limits/groups/${groupId}`)
      await fetchLimits()
    },
    [fetchLimits]
  )

  const checkExceeds = useCallback(
    async (categoryId: string, amount: number): Promise<{ exceeds: boolean; current: number; limit: number }> => {
      return api.get(`/account/limits/check?category_id=${categoryId}&amount=${amount}`)
    },
    []
  )

  return {
    individual,
    groups,
    isLoading,
    upsert,
    remove,
    createGroup,
    updateGroup,
    removeGroup,
    checkExceeds,
    refetch: fetchLimits,
  }
}
