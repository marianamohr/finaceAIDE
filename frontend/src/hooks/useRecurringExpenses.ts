import { useCallback, useEffect, useState } from "react"
import { api, RecurringExpense, RecurringExpenseWithStatus } from "@/services/api"

export type { RecurringExpense, RecurringExpenseWithStatus }

export function useRecurringExpenses() {
  const [items, setItems] = useState<RecurringExpense[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const fetch = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await api.getRecurringExpenses()
      setItems(data ?? [])
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => { fetch() }, [fetch])
  useEffect(() => {
    window.addEventListener("focus", fetch)
    return () => window.removeEventListener("focus", fetch)
  }, [fetch])

  const create = useCallback(
    async (body: { name: string; amount: number; category_id?: string; payment_method_id?: string; due_day?: number; start_month?: number; start_year?: number; end_month?: number; end_year?: number; is_variable?: boolean }) => {
      await api.createRecurringExpense(body)
      await fetch()
    },
    [fetch]
  )

  const update = useCallback(
    async (id: string, body: { name?: string; amount?: number; category_id?: string | null; payment_method_id?: string | null; due_day?: number | null; active?: boolean; start_month?: number | null; start_year?: number | null; end_month?: number | null; end_year?: number | null; is_variable?: boolean }) => {
      await api.updateRecurringExpense(id, body)
      await fetch()
    },
    [fetch]
  )

  const remove = useCallback(
    async (id: string) => {
      await api.deleteRecurringExpense(id)
      await fetch()
    },
    [fetch]
  )

  return { items, isLoading, create, update, remove, refetch: fetch }
}

export function useRecurringReminders(year: number, month: number) {
  const [reminders, setReminders] = useState<RecurringExpenseWithStatus[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const fetchReminders = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await api.getRecurringReminders(year, month)
      setReminders(data ?? [])
    } finally {
      setIsLoading(false)
    }
  }, [year, month])

  useEffect(() => { fetchReminders() }, [fetchReminders])

  const markPaid = useCallback(
    async (id: string, paymentMethodId?: string, amount?: number) => {
      await api.markRecurringPaid(id, year, month, paymentMethodId, amount)
      await fetchReminders()
      window.dispatchEvent(new CustomEvent("transactions:changed"))
    },
    [year, month, fetchReminders]
  )

  const unmarkPaid = useCallback(
    async (id: string) => {
      await api.unmarkRecurringPaid(id, year, month)
      await fetchReminders()
      window.dispatchEvent(new CustomEvent("transactions:changed"))
    },
    [year, month, fetchReminders]
  )

  const pendingCount = reminders.filter((r) => !r.paid).length

  return { reminders, isLoading, pendingCount, markPaid, unmarkPaid, refetch: fetchReminders }
}
