import { OrctaPayError } from "./errors.js";
import { generateReference } from "./reference.js";
import type {
  ChargeResult,
  ChargeStatus,
  CreateChargeRequest,
  CreatePayoutRequest,
  PayoutResult,
} from "./types.js";

export const DEFAULT_BASE_URL = "http://localhost:8080";
const DEFAULT_TIMEOUT_MS = 10_000;

export interface OrctaPayOptions {
  baseUrl?: string;
  apiKey: string;
  timeout?: number;
}

/**
 * Thin client for the Orcta Pay API.
 *
 * Runtime-agnostic: uses native `fetch` and `AbortController`.
 * Works on Node 18+, Bun, Deno, and browsers.
 */
export class OrctaPay {
  private readonly baseUrl: string;
  private readonly apiKey: string;
  private readonly timeout: number;

  constructor(opts: OrctaPayOptions) {
    // Mirror Go's baseURL fallback: ORCTA_PAY_URL env or localhost.
    let baseUrl = opts.baseUrl;
    if (!baseUrl) {
      try {
        const envUrl =
          typeof process !== "undefined"
            ? (process as unknown as { env?: Record<string, string> }).env
                ?.ORCTA_PAY_URL
            : undefined;
        if (envUrl) baseUrl = envUrl;
      } catch {
        // ignore
      }
    }
    if (!baseUrl) baseUrl = DEFAULT_BASE_URL;
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.apiKey = opts.apiKey;
    this.timeout = opts.timeout ?? DEFAULT_TIMEOUT_MS;
  }

  /** Initiate a charge. Generates idempotency_key when not supplied. */
  async createCharge(req: CreateChargeRequest): Promise<ChargeResult> {
    let key = req.idempotency_key;
    if (!key) {
      const product = req.product || "default";
      key = generateReference(product, "hubtel");
    }

    const currency = req.currency || "GHS";
    const body: Record<string, unknown> = {
      product: req.product,
      amount_pesewas: req.amount_pesewas,
      currency,
      idempotency_key: key,
    };

    const wallet = req.wallet ?? req.phone;
    if (wallet) {
      body["wallet"] = wallet;
      if (req.phone && req.phone !== wallet) {
        body["phone"] = req.phone;
      }
    } else if (req.phone) {
      body["wallet"] = req.phone;
    }

    if (req.metadata) body["metadata"] = req.metadata;

    const wire = await this.doJSON<ChargeWire>(
      "POST",
      "/v1/charges",
      body,
    );

    switch (wire.status) {
      case "pending":
        return {
          status: "pending",
          ref: wire.ref,
          gateway: wire.gateway,
          external_ref: wire.external_ref,
        };
      case "succeeded":
        return {
          status: "succeeded",
          ref: wire.ref,
          gateway: wire.gateway,
          amount_pesewas: wire.amount_pesewas,
          currency: wire.currency,
          external_ref: wire.external_ref,
        };
      case "failed":
        return { status: "failed", reason: wire.status };
      default: {
        if (wire.ref) {
          return {
            status: "pending",
            ref: wire.ref,
            gateway: wire.gateway,
            external_ref: wire.external_ref,
          };
        }
        return { status: "failed", reason: `unknown status: ${wire.status}` };
      }
    }
  }

  /** Fetch authoritative status for a charge. */
  async getChargeStatus(ref: string): Promise<ChargeStatus> {
    if (!ref) {
      throw new OrctaPayError("ref is required", 400, "invalid_request");
    }
    const path = `/v1/charges/${encodeURIComponent(ref)}/status`;
    return this.doJSON<ChargeStatus>("GET", path, undefined);
  }

  /** Create a payout batch. */
  async createPayout(req: CreatePayoutRequest): Promise<PayoutResult> {
    if (!req.product) {
      throw new OrctaPayError("product is required", 400, "invalid_request");
    }
    if (!req.entries || req.entries.length === 0) {
      throw new OrctaPayError("entries required", 400, "invalid_request");
    }

    const entries = req.entries.map((e) => ({
      recipient: e.recipient,
      amount_pesewas: e.amount_pesewas,
      currency: e.currency || "GHS",
    }));

    const body: Record<string, unknown> = {
      product: req.product,
      entries,
    };
    if (req.idempotency_key) body["idempotency_key"] = req.idempotency_key;

    return this.doJSON<PayoutResult>("POST", "/v1/payouts", body);
  }

  private async doJSON<T>(
    method: string,
    path: string,
    body: unknown,
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const controller = new AbortController();
    const tid = setTimeout(() => controller.abort(), this.timeout);

    const headers: Record<string, string> = {
      Accept: "application/json",
    };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (this.apiKey) headers["Authorization"] = `Bearer ${this.apiKey}`;

    let resp: Response;
    try {
      resp = await fetch(url, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });
    } catch (err) {
      clearTimeout(tid);
      if (err instanceof Error && err.name === "AbortError") {
        throw new OrctaPayError("Request timeout", 408, "timeout", err);
      }
      throw new OrctaPayError(
        err instanceof Error ? err.message : "Network error",
        0,
        "network_error",
        err,
      );
    }
    clearTimeout(tid);

    const text = await resp.text();

    if (resp.ok) {
      if (!text) return undefined as unknown as T;
      try {
        return JSON.parse(text) as T;
      } catch (e) {
        throw new OrctaPayError("decode error", resp.status, "decode_error", e);
      }
    }

    // Non-2xx: try error envelope.
    let code = "unknown";
    let message = text.trim() || resp.statusText || resp.status.toString();
    try {
      const parsed = JSON.parse(text) as {
        error?: { code?: string; message?: string };
      };
      if (parsed?.error) {
        if (parsed.error.code) code = parsed.error.code;
        if (parsed.error.message) message = parsed.error.message;
      }
    } catch {
      // keep raw text
    }

    // Normalise codes for callers mirroring Go sentinels.
    if (resp.status === 401) code = "unauthorized";
    else if (resp.status === 404) code = "not_found";
    else if (resp.status === 400) code = "invalid_request";
    else if (resp.status >= 500) code = code === "unknown" ? "internal_error" : code;

    throw new OrctaPayError(message, resp.status, code);
  }
}

interface ChargeWire {
  ref: string;
  product: string;
  gateway: string;
  amount_pesewas: number;
  currency: string;
  status: string;
  external_ref?: string;
  created_at?: string;
}
