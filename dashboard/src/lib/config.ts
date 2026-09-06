const LS_URL = "orcta_pay_url";
const LS_SCOPE = "orcta_pay_scope";

export function getBaseUrl(): string {
  try {
    const ls = localStorage.getItem(LS_URL);
    if (ls) return ls;
  } catch {
    // ignore
  }
  const envUrl = import.meta.env.VITE_ORCTA_PAY_URL as string | undefined;
  if (envUrl) return envUrl;
  // When served from the same origin as the API (single image), use relative URLs.
  try {
    if (typeof window !== "undefined" && window.location?.origin) {
      return window.location.origin;
    }
  } catch {
    // ignore
  }
  return "http://localhost:8080";
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
