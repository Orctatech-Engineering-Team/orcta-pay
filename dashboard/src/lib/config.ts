const LS_KEY = "orcta_pay_api_key";
const LS_URL = "orcta_pay_url";
const LS_PRODUCT = "orcta_pay_product";

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

export function getProduct(): string {
  try {
    const ls = localStorage.getItem(LS_PRODUCT);
    if (ls) return ls;
  } catch {
    // ignore
  }
  return (import.meta.env.VITE_ORCTA_PAY_PRODUCT as string) || "orctago";
}

export function setProduct(product: string): void {
  try {
    if (product) localStorage.setItem(LS_PRODUCT, product);
    else localStorage.removeItem(LS_PRODUCT);
  } catch {
    // ignore
  }
}

export function maskKey(key: string): string {
  if (!key) return "— not set —";
  if (key.length <= 8) return "••••";
  return `${key.slice(0, 8)}…${key.slice(-4)}`;
}

export function inferProductFromKey(key: string): string {
  // Vault path is secret/orcta/orcta-pay/keys/{product}; key itself is opaque,
  // but localStorage product hint wins. Fallback to env product.
  void key;
  return getProduct();
}
