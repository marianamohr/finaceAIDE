export const CATEGORY_LABELS: Record<string, string> = {
  salario: 'Salário',
  vr: 'VR',
  ppr: 'PPR',
  freelance: 'Freelance',
  investimentos_receita: 'Investimentos',
  aluguel_recebido: 'Aluguel recebido',
  outras_entradas: 'Outras entradas',
  alimentacao: 'Alimentação',
  transporte: 'Transporte',
  saude: 'Saúde',
  casa: 'Casa',
  lazer: 'Lazer',
  educacao: 'Educação',
  vestuario: 'Vestuário',
  presentes: 'Presentes',
  servicos: 'Serviços',
  pets: 'Pets',
  emprestimos: 'Empréstimos',
  impostos: 'Impostos',
  beleza: 'Beleza',
  investimentos_despesa: 'Investimentos',
  outros: 'Outros',
  comer_fora: 'Comer fora',
  comida: 'Comida',
  frutaria: 'Frutaria',
  bebidas_nao_alcoolicas: 'Bebidas não alcoólicas',
  bebidas_alcoolicas: 'Bebidas alcoólicas',
  higiene_pessoal: 'Higiene pessoal',
  limpeza_da_casa: 'Limpeza da casa',
  reserva: 'Reserva',
}

export function categoryLabel(slug: string): string {
  return CATEGORY_LABELS[slug] ?? slug
}

export const CATEGORY_EMOJIS: Record<string, string> = {
  alimentacao: "🛒",
  transporte: "🚗",
  saude: "🏥",
  casa: "🏠",
  lazer: "🎮",
  educacao: "📚",
  vestuario: "👗",
  presentes: "🎁",
  servicos: "🔧",
  pets: "🐾",
  emprestimos: "💳",
  impostos: "🧾",
  beleza: "💅",
  investimentos_despesa: "📈",
  outros: "📦",
  comer_fora: "🍽️",
  comida: "🛒",
  frutaria: "🍎",
  bebidas_nao_alcoolicas: "🥤",
  bebidas_alcoolicas: "🍺",
  higiene_pessoal: "🧴",
  limpeza_da_casa: "🧹",
}

export function categoryEmoji(slug: string): string {
  return CATEGORY_EMOJIS[slug] ?? "📦"
}

export const EXPENSE_SLUGS_STANDARD = [
  "alimentacao", "transporte", "saude", "casa", "lazer", "educacao",
  "vestuario", "presentes", "servicos", "pets", "emprestimos", "impostos", "beleza",
  "investimentos_despesa", "outros",
]

export const EXPENSE_SLUGS_FOOD_DETAIL = [
  "comer_fora", "comida", "frutaria", "bebidas_nao_alcoolicas",
  "bebidas_alcoolicas", "higiene_pessoal", "limpeza_da_casa",
  "transporte", "saude", "casa", "lazer", "educacao",
  "vestuario", "presentes", "servicos", "pets", "emprestimos", "impostos", "beleza",
  "investimentos_despesa", "outros",
]
