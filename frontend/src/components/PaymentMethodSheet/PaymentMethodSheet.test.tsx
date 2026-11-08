import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { PaymentMethodSheet } from "./PaymentMethodSheet"
import type { PaymentMethod } from "@/hooks/usePaymentMethods"

const debit = {
  id: "pm1",
  account_id: "acc",
  name: "Itaú Débito",
  kind: "debit",
  active: true,
  color: "#000000",
} as unknown as PaymentMethod

describe("PaymentMethodSheet", () => {
  it("sem askAmount: não mostra campo de valor e seleciona sem amount", () => {
    const onSelect = vi.fn()
    render(<PaymentMethodSheet paymentMethods={[debit]} onSelect={onSelect} onClose={() => {}} />)

    expect(screen.queryByText(/Valor pago/i)).toBeNull()

    fireEvent.click(screen.getByText("Itaú Débito"))
    expect(onSelect).toHaveBeenCalledWith(debit, undefined)
  })

  it("askAmount: mostra o campo pré-preenchido e envia o valor digitado", () => {
    const onSelect = vi.fn()
    render(
      <PaymentMethodSheet
        paymentMethods={[debit]}
        onSelect={onSelect}
        onClose={() => {}}
        askAmount
        initialAmount={150}
        currencySymbol="R$"
      />
    )

    const input = screen.getByRole("spinbutton") as HTMLInputElement
    expect(input.value).toBe("150")

    fireEvent.change(input, { target: { value: "187.45" } })
    fireEvent.click(screen.getByText("Itaú Débito"))
    expect(onSelect).toHaveBeenCalledWith(debit, 187.45)
  })

  it("askAmount sem valor: opções desabilitadas e não chama onSelect", () => {
    const onSelect = vi.fn()
    render(
      <PaymentMethodSheet
        paymentMethods={[debit]}
        onSelect={onSelect}
        onClose={() => {}}
        askAmount
      />
    )

    const btn = screen.getByText("Itaú Débito").closest("button")!
    expect(btn).toBeDisabled()

    fireEvent.click(btn)
    expect(onSelect).not.toHaveBeenCalled()
  })
})
