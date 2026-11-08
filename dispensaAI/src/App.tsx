import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Toaster } from "@/components/ui/sonner";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { AppShell } from "@/components/AppShell";
import Login from "./pages/Login";
import Dispensa from "./pages/Dispensa";
import Nota from "./pages/Nota";
import NotaRevisao from "./pages/NotaRevisao";
import Lista from "./pages/Lista";
import Receitas from "./pages/Receitas";
import ReceitaDetalhe from "./pages/ReceitaDetalhe";
import Ajustes from "./pages/Ajustes";
import Inventario from "./pages/Inventario";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30 * 1000 } },
});

function Page({ children }: { children: React.ReactNode }) {
  return (
    <ProtectedRoute>
      <AppShell>{children}</AppShell>
    </ProtectedRoute>
  );
}

const App = () => (
  <QueryClientProvider client={queryClient}>
    <Toaster />
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<Page><Dispensa /></Page>} />
        <Route path="/nota" element={<Page><Nota /></Page>} />
        <Route path="/nota/:id" element={<Page><NotaRevisao /></Page>} />
        <Route path="/lista" element={<Page><Lista /></Page>} />
        <Route path="/receitas" element={<Page><Receitas /></Page>} />
        <Route path="/receitas/:id" element={<Page><ReceitaDetalhe /></Page>} />
        <Route path="/ajustes" element={<Page><Ajustes /></Page>} />
        <Route path="/inventario" element={<Page><Inventario /></Page>} />
        <Route path="/dispensa" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </BrowserRouter>
  </QueryClientProvider>
);

export default App;
