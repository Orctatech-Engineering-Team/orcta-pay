import { OrctaPay } from "@orctatech/orcta-pay";
import { getBaseUrl } from "./config";

export function makeClient(): OrctaPay {
  return new OrctaPay({
    baseUrl: getBaseUrl(),
    apiKey: "",
    timeout: 8000,
  });
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const baseUrl = getBaseUrl().replace(/\/+$/, "");
  return fetch(`${baseUrl}${path}`, {
    ...init,
    credentials: "include",
    headers: { Accept: "application/json", ...init.headers },
  });
}

export async function login(apiKey: string): Promise<void> {
  const response = await apiFetch("/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ api_key: apiKey }),
  });
  if (!response.ok) throw new Error(response.status === 401 ? "Invalid access key" : "Unable to sign in");
}

export async function logout(): Promise<void> {
  await apiFetch("/auth/logout", { method: "POST" });
}

export async function hasSession(): Promise<boolean> {
  return (await apiFetch("/auth/session")).ok;
}

export type HealthState = "ok" | "down" | "unknown" | "checking";

export async function checkHealth(): Promise<{ state: HealthState; detail: string }> {
  const baseUrl = getBaseUrl().replace(/\/+$/, "");
  const controllers: AbortController[] = [];
  const tryFetch = async (path: string): Promise<Response> => {
    const c = new AbortController();
    controllers.push(c);
    const t = setTimeout(() => c.abort(), 4000);
    try {
      return await fetch(`${baseUrl}${path}`, { signal: c.signal, headers: { Accept: "application/json" } });
    } finally {
      clearTimeout(t);
    }
  };

  // Prefer /healthz (unauthenticated). Fallback to /readyz.
  try {
    const r = await tryFetch("/healthz");
    if (r.ok) return { state: "ok", detail: `${r.status} ${r.statusText}` };
    const txt = await r.text().catch(() => "");
    return { state: "down", detail: `${r.status} ${txt.slice(0, 120)}` };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.includes("AbortError") || msg.includes("aborted")) {
      return { state: "down", detail: "timeout" };
    }
    // Network error, try /readyz as second probe before giving up.
    try {
      const r2 = await tryFetch("/readyz");
      if (r2.ok) return { state: "ok", detail: `${r2.status} ${r2.statusText}` };
      return { state: "down", detail: `${r2.status} unreachable` };
    } catch {
      return { state: "down", detail: msg.slice(0, 120) || "network error" };
    }
  } finally {
    for (const c of controllers) {
      try {
        c.abort();
      } catch {
        // ignore
      }
    }
  }
}
