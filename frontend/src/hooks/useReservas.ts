import { useCallback, useEffect, useState } from "react"
import { api } from "@/services/api"

export interface Goal {
  id: string
  account_id: string
  name: string
  target_amount: number
  current_amount: number
  initial_amount: number
  monthly_contribution: number
  annual_rate: number
  allocation_pct: number
  goal_type: "emergency_global" | "travel" | "car" | "house" | "emergency" | "other"
  cover_image_url?: string
  institution: string
  account_type: string
  deadline?: string
  created_at: string
}

export interface GoalTransaction {
  id: string
  goal_id: string
  account_id: string
  amount: number
  description: string
  date: string
  source: "manual" | "initial" | "yield" | "withdrawal" | "chat"
  created_at: string
}

export interface SuggestedContribution {
  goal_id: string
  goal_name: string
  suggested_amount: number
  allocation_pct: number
}

export interface AddTransactionPayload {
  amount: number
  description?: string
  date?: string
  source: "manual" | "initial" | "yield" | "withdrawal"
  expense_category?: string
}

export interface CreateGoalPayload {
  name: string
  target_amount: number
  initial_amount?: number
  monthly_contribution?: number
  annual_rate?: number
  allocation_pct?: number
  goal_type?: "emergency_global" | "travel" | "car" | "house" | "emergency" | "other"
  cover_image_url?: string
  institution?: string
  account_type?: string
  deadline?: string
}

export function useReservas() {
  const [goals, setGoals] = useState<Goal[]>([])
  const [suggestions, setSuggestions] = useState<SuggestedContribution[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [fetchedGoals, fetchedSuggestions] = await Promise.all([
        api.get<Goal[]>("/goals"),
        api.get<SuggestedContribution[]>("/goals/suggestions"),
      ])
      setGoals(fetchedGoals ?? [])
      setSuggestions(fetchedSuggestions ?? [])
    } catch {
      setError("Não foi possível carregar as reservas.")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  const listTransactions = useCallback(async (goalId: string): Promise<GoalTransaction[]> => {
    const txs = await api.get<GoalTransaction[]>(`/goals/${goalId}/transactions`)
    return txs ?? []
  }, [])

  const createGoal = useCallback(async (payload: CreateGoalPayload): Promise<Goal> => {
    const goal = await api.post<Goal>("/goals", payload)
    await fetchAll()
    return goal
  }, [fetchAll])

  const updateGoal = useCallback(async (id: string, payload: Partial<CreateGoalPayload>): Promise<Goal> => {
    const goal = await api.put<Goal>(`/goals/${id}`, payload)
    await fetchAll()
    return goal
  }, [fetchAll])

  const addTransaction = useCallback(async (goalId: string, payload: AddTransactionPayload): Promise<GoalTransaction> => {
    const tx = await api.post<GoalTransaction>(`/goals/${goalId}/transactions`, payload)
    await fetchAll()
    return tx
  }, [fetchAll])

  return {
    goals,
    suggestions,
    loading,
    error,
    refetch: fetchAll,
    listTransactions,
    createGoal,
    updateGoal,
    addTransaction,
  }
}
