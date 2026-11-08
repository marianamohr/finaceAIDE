import { useEffect, useState } from "react"
import { api } from "@/services/api"
import { DashboardSummary } from "./useDashboard"

export interface MonthData {
  year: number
  month: number
  label: string
  income: number
  expenses: number
  balance: number
  categories: Record<string, number>
}

export interface GoalMonthlyPoint {
  year: number
  month: number
  total: number
}

export function useGoalHistory(months = 12) {
  const [data, setData] = useState<GoalMonthlyPoint[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<GoalMonthlyPoint[]>(`/goals/monthly-history?months=${months}`)
      .then((pts) => { setData(pts ?? []); setLoading(false) })
      .catch(() => { setData([]); setLoading(false) })
  }, [months])

  return { data, loading }
}

export interface PaymentMethodSummary {
  payment_method_id: string | null
  name: string
  kind: string
  color: string
  total: number
  count: number
}

export interface PaymentMethodMonthData {
  year: number
  month: number
  label: string
  methods: PaymentMethodSummary[]
}

export function usePaymentMethodAnalytics(months = 12) {
  const [data, setData] = useState<PaymentMethodMonthData[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<PaymentMethodMonthData[]>(`/transactions/payment-methods-analytics?months=${months}`)
      .then((d) => { setData(d ?? []); setLoading(false) })
      .catch(() => { setData([]); setLoading(false) })
  }, [months])

  return { data, loading }
}

export interface ExpenseProjectionMonth {
  year: number
  month: number
  label: string
  fixed: number
  installments: number
  card: number
  total: number
}

export interface ProjectionItem {
  name: string
  amount: number
  installment_info?: string
  category_label?: string
  card_name?: string
}

export interface ProjectionItems {
  fixed: ProjectionItem[]
  installments: ProjectionItem[]
  card: ProjectionItem[]
}

export interface ExpenseProjectionMonthDetail extends ExpenseProjectionMonth {
  items: ProjectionItems
}

export function useExpenseProjection(months = 6) {
  const [data, setData] = useState<ExpenseProjectionMonth[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<{ months: ExpenseProjectionMonth[] }>(`/transactions/expense-projection?months=${months}`)
      .then((d) => { setData(d?.months ?? []); setLoading(false) })
      .catch(() => { setData([]); setLoading(false) })
  }, [months])

  return { data, loading }
}

export function useExpenseProjectionDetail(months = 6) {
  const [data, setData] = useState<ExpenseProjectionMonthDetail[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    setError(false)
    api.get<{ months: ExpenseProjectionMonthDetail[] }>(`/transactions/expense-projection/detail?months=${months}`)
      .then((d) => { setData(d?.months ?? []); setLoading(false) })
      .catch(() => { setError(true); setLoading(false) })
  }, [months])

  return { data, loading, error }
}

export function useAnalytics(months = 12) {
  const [data, setData] = useState<MonthData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const now = new Date()
    const monthsList: { year: number; month: number }[] = []
    for (let i = months - 1; i >= 0; i--) {
      const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
      monthsList.push({ year: d.getFullYear(), month: d.getMonth() + 1 })
    }

    Promise.all(
      monthsList.map(({ year, month }) =>
        api.get<DashboardSummary>(`/transactions/summary?year=${year}&month=${month}`)
          .then((s) => ({ year, month, summary: s }))
          .catch(() => ({ year, month, summary: null as DashboardSummary | null }))
      )
    ).then((results) => {
      const allFailed = results.every((r) => r.summary === null)
      if (allFailed) {
        setError("Não foi possível carregar os dados.")
        setLoading(false)
        return
      }
      const ptMonths = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
      const monthData = results.map(({ year, month, summary }) => ({
        year,
        month,
        label: `${ptMonths[month - 1]}/${String(year).slice(2)}`,
        income: summary?.income ?? 0,
        expenses: Math.abs(summary?.expenses ?? 0),
        balance: summary?.balance ?? 0,
        categories: summary
          ? Object.fromEntries(Object.entries(summary.categories).map(([k, v]) => [k, Math.abs(v)]))
          : {},
      }))
      setData(monthData)
      setLoading(false)
    })
  }, [months])

  return { data, loading, error }
}
