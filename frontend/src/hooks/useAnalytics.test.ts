import { describe, it, expect, vi, beforeEach } from "vitest"
import { renderHook, waitFor } from "@testing-library/react"
import { useAnalytics } from "@/hooks/useAnalytics"
import { api } from "@/services/api"

vi.mock("@/services/api", () => ({
  api: { get: vi.fn() },
}))

const makeSummary = (year: number, month: number) => ({
  income: 1000,
  expenses: -500,
  balance: 500,
  categories: { alimentacao: -200 },
  month: `${year}-${String(month).padStart(2, "0")}`,
})

beforeEach(() => vi.clearAllMocks())

describe("useAnalytics", () => {
  it("retorna 12 meses por padrão", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result } = renderHook(() => useAnalytics())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(12)
  })

  it("retorna apenas N meses quando months=3", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result } = renderHook(() => useAnalytics(3))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(3)
  })

  it("re-executa quando months muda", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result, rerender } = renderHook(
      ({ months }: { months: number }) => useAnalytics(months),
      { initialProps: { months: 12 } }
    )
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(12)

    rerender({ months: 6 })
    await waitFor(() => expect(result.current.data).toHaveLength(6))
  })
})
