import { useState } from "react";
import {
  startRegistration,
  startAuthentication,
} from "@simplewebauthn/browser";
import { api, saveRefreshToken } from "@/services/api";
import { useAuthStore, User } from "@/store/auth";
import { useNavigate } from "react-router-dom";

type AuthResponse = { user: User; access_token: string; refresh_token?: string };

export function usePasskey() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { setAuth } = useAuthStore();
  const navigate = useNavigate();

  const registerPasskey = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const options = await api.post<any>("/auth/passkey/register/begin");
      const credential = await startRegistration({ optionsJSON: options.response ?? options });
      await api.post("/auth/passkey/register/finish", credential);
    } catch (e: any) {
      setError(e.message || "Falha ao registrar Face ID");
      throw e;
    } finally {
      setIsLoading(false);
    }
  };

  const loginWithPasskey = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const beginData = await api.post<{ publicKey: any; session_id: string }>(
        "/auth/passkey/login/begin"
      );
      const credential = await startAuthentication({ optionsJSON: beginData.publicKey });
      const data = await api.post<AuthResponse>(
        `/auth/passkey/login/finish?session_id=${beginData.session_id}`,
        credential
      );
      setAuth(data.user, data.access_token);
      if (data.refresh_token) saveRefreshToken(data.refresh_token);
      if (data.user.status === "pending") {
        navigate("/pending");
      } else if (data.user.role === "admin") {
        navigate("/admin");
      } else {
        navigate("/chat");
      }
    } catch (e: any) {
      setError(e.message || "Falha ao autenticar com Face ID");
    } finally {
      setIsLoading(false);
    }
  };

  return { registerPasskey, loginWithPasskey, isLoading, error };
}
