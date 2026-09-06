const LS_KEY = "orcta_pay_api_key";
const LS_URL = "orcta_pay_url";
const LS_SCOPE = "orcta_pay_scope";

export function getApiKey(): string {
  try {
    const ls = localStorage.getItem(LS_KEY);
    if (ls) return ls;
  } catch {
    // ignore
  }
  return import.meta.env.VITE_ORCTA_PAY_API_KEY as string || "";
}

export function setApiKey(key: string): void {
  try {
    if (key) localStorage.setItem(LS_KEY, key);
    else localStorage.removeItem(LS_KEY);
  } catch {
    // ignore
  }
}

export function getBaseUrl(): string {
  try {
    const ls = localStorage.getItem(LS_URL);
    if (ls) return ls;
  } catch {
    // ignore
  }
  return (import.meta.env.VITE_ORCTA_PAY_URL as string) || "http://localhost:8080";
}

export function setBaseUrl(url: string): void {
  try {
    if (url) localStorage.setItem(LS_URL, url);
    else localStorage.removeItem(LS_URL);
  } catch {
    // ignore
  }
}

export function getScope(): string {
  try {
    const ls = localStorage.getItem(LS_SCOPE);
    if (ls) return ls;
  } catch {
    // ignore
  }
  return "all";
}

export function setScope(scope: string): void {
  try {
    if (scope && scope !== "all") localStorage.setItem(LS_SCOPE, scope);
    else localStorage.removeItem(LS_SCOPE);
  } catch {
    // ignore
  }
}

export function maskKey(key: string): string {
  if (!key) return "not set";
  if (key.length <= 8) return "••••";
  return `${key.slice(0, 8)}…${key.slice(-4)}`;
}
