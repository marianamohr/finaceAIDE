import { useEffect } from "react";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { AdminRoute } from "@/components/AdminRoute";
import { Navigate } from "react-router-dom";
import Login from "./pages/Login";
import Chat from "./pages/Chat";
import Consumo from "./pages/Consumo";
import Caixa from "./pages/Caixa";
import Profile from "./pages/Profile";
import ProfileCategories from "./pages/ProfileCategories";
import ProfileMerchantAlias from "./pages/ProfileMerchantAlias";
import ProfilePaymentMethods from "./pages/ProfilePaymentMethods";
import PendingApproval from "./pages/PendingApproval";
import Admin from "./pages/Admin";
import Invite from "./pages/Invite";
import NotFound from "./pages/NotFound";
import Tools from "./pages/Tools";
import Painel from "./pages/Painel";
import Reservas from "./pages/Reservas";
import Limites from "./pages/Limites";
import GastosFixos from "./pages/GastosFixos";
import GastosFixosLembretes from "./pages/GastosFixosLembretes";
import ProjecaoDetalhe from "./pages/ProjecaoDetalhe";
import Gestao from "./pages/Gestao";
import { AppShell } from "@/components/AppShell/AppShell";
import { api } from "@/services/api";
import { useCurrencyStore, Currency } from "@/store/currency";
import { useAuthStore } from "@/store/auth";
import { useBootstrapAuth } from "@/hooks/useBootstrapAuth";

const queryClient = new QueryClient();

function AppBootstrap({ children }: { children: React.ReactNode }) {
  const setCurrency = useCurrencyStore((s) => s.setCurrency);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { ready } = useBootstrapAuth();
  const qc = useQueryClient();

  useEffect(() => {
    if (!isAuthenticated || !ready) return;
    api.getCurrency()
      .then((data) => setCurrency(data.base_currency as Currency))
      .catch(() => {/* keep localStorage value */});
  }, [isAuthenticated, ready, setCurrency]);

  useEffect(() => {
    if (!isAuthenticated || !ready) return;
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth() + 1;
    qc.prefetchQuery({
      queryKey: ["dashboard", year, month],
      queryFn: () => api.getDashboard(year, month),
      staleTime: 60 * 1000,
    });
  }, [isAuthenticated, ready, qc]);

  return <>{children}</>;
}

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <Toaster />
      <Sonner />
      <BrowserRouter>
        <AppBootstrap>
        <Routes>
          <Route path="/" element={<Navigate to="/login" replace />} />
          <Route path="/login" element={<Login />} />
          <Route path="/pending" element={<PendingApproval />} />
          <Route path="/invite" element={<Invite />} />
          <Route path="/chat" element={<ProtectedRoute><AppShell><Chat /></AppShell></ProtectedRoute>} />
          <Route path="/dashboard" element={<Navigate to="/consumo" replace />} />
          <Route path="/consumo" element={<ProtectedRoute><AppShell><Consumo /></AppShell></ProtectedRoute>} />
          <Route path="/caixa" element={<ProtectedRoute><AppShell><Caixa /></AppShell></ProtectedRoute>} />
          <Route path="/profile" element={<ProtectedRoute><AppShell><Profile /></AppShell></ProtectedRoute>} />
          <Route path="/profile/categories" element={<ProtectedRoute><AppShell><ProfileCategories /></AppShell></ProtectedRoute>} />
          <Route path="/profile/merchant-alias" element={<ProtectedRoute><AppShell><ProfileMerchantAlias /></AppShell></ProtectedRoute>} />
          <Route path="/profile/payment-methods" element={<ProtectedRoute><AppShell><ProfilePaymentMethods /></AppShell></ProtectedRoute>} />
          <Route path="/tools" element={<ProtectedRoute><AppShell><Tools /></AppShell></ProtectedRoute>} />
          <Route path="/insights" element={<Navigate to="/painel" replace />} />
          <Route path="/painel" element={<ProtectedRoute><AppShell><Painel /></AppShell></ProtectedRoute>} />
          <Route path="/reservas" element={<ProtectedRoute><AppShell><Reservas /></AppShell></ProtectedRoute>} />
          <Route path="/limites" element={<ProtectedRoute><AppShell><Limites /></AppShell></ProtectedRoute>} />
          <Route path="/gastos-fixos" element={<ProtectedRoute><AppShell><GastosFixos /></AppShell></ProtectedRoute>} />
          <Route path="/gastos-fixos/lembretes" element={<ProtectedRoute><AppShell><GastosFixosLembretes /></AppShell></ProtectedRoute>} />
          <Route path="/projecao" element={<ProtectedRoute><AppShell><ProjecaoDetalhe /></AppShell></ProtectedRoute>} />
          <Route path="/gestao" element={<ProtectedRoute><AppShell><Gestao /></AppShell></ProtectedRoute>} />
          <Route path="/admin" element={<AdminRoute><Admin /></AdminRoute>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
        </AppBootstrap>
      </BrowserRouter>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
