// Flag local (por navegador/dispositivo, como o refresh token em services/api.ts)
// que marca se a usuária já viu a tela de inventário inicial — usado só para
// decidir se ela aparece sozinha na primeira vez que a dispensa está vazia.
// A tela continua acessível a qualquer momento por Ajustes, independente
// desse flag.
const SEEN_KEY = "dai-onboarding-seen";

export function hasSeenOnboarding(): boolean {
  try {
    return localStorage.getItem(SEEN_KEY) === "1";
  } catch {
    return true; // storage indisponível: não insiste em redirecionar
  }
}

export function markOnboardingSeen() {
  try {
    localStorage.setItem(SEEN_KEY, "1");
  } catch {
    /* storage indisponível */
  }
}
