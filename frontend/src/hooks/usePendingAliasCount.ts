import { useState, useEffect } from "react";
import { api } from "@/services/api";

export function usePendingAliasCount() {
  const [count, setCount] = useState(0);

  useEffect(() => {
    api.get<{ count: number }>("/merchant-aliases/pending-count")
      .then((data) => setCount(data.count))
      .catch(() => { /* silently ignore — badge is non-critical */ });
  }, []);

  return count;
}
