import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { OrctaPay } from "../src/client.js";
import { OrctaPayError } from "../src/errors.js";

function mockFetchOnce(
  handler: (url: string, init?: RequestInit) => Response | Promise<Response>,
) {
  const fn = vi.fn(async (url: string, init?: RequestInit) => handler(url, init as RequestInit));
  globalThis.fetch = fn as unknown as typeof fetch;
  return fn;
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  if (!headers.has("content-type")) headers.set("content-type", "application/json");
  return new Response(JSON.stringify(body), { ...init, headers });
}

describe("OrctaPay", () => {
  const origFetch = globalThis.fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    globalThis.fetch = origFetch;
  });

  it("createCharge success — sets Bearer and optd- prefix", async () => {
    let gotAuth = "";
    let gotBody: Record<string, unknown> = {};

    mockFetchOnce(async (_url, init) => {
      gotAuth = (init?.headers as Record<string, string>)?.["Authorization"] ?? "";
      if (!gotAuth && init?.headers instanceof Headers) {
        gotAuth = init.headers.get("Authorization") ?? "";
      }
      gotBody = JSON.parse(init?.body as string);
      return jsonResponse(
        {
          ref: "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV",
          gateway: "hubtel",
          status: "pending",
          external_ref: "EXT123",
        },
        { status: 201 },
      );
    });

    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "test-key" });
    const { data, error } = await client.createCharge({
      product: "orctago",
      amount_pesewas: 1800,
      currency: "GHS",
      wallet: "0241234567",
    });

    expect(error).toBeNull();
    expect(data).not.toBeNull();
    expect(gotAuth).toBe("Bearer test-key");
    const key = gotBody["idempotency_key"] as string;
    expect(key.startsWith("optd-")).toBe(true);
    expect(key).toMatch(/^optd-orctago-hubtel-[0-9A-Z]{26}$/);
    expect(data?.status).toBe("pending");
    if (data?.status === "pending") {
      expect(data.ref).toBe("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV");
      expect(data.gateway).toBe("hubtel");
    }
  });

  it("createCharge with explicit idempotency_key reuses it", async () => {
    let gotBody: Record<string, unknown> = {};
    mockFetchOnce(async (_url, init) => {
      gotBody = JSON.parse(init?.body as string);
      return jsonResponse(
        { ref: "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV", gateway: "hubtel", status: "pending" },
        { status: 201 },
      );
    });

    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data, error } = await client.createCharge({
      product: "orctago",
      amount_pesewas: 100,
      currency: "GHS",
      wallet: "0241234567",
      idempotency_key: "optd-orctago-hubtel-01CUSTOMKEY12345678901234",
    });

    expect(error).toBeNull();
    expect(data).not.toBeNull();
    expect(gotBody["idempotency_key"]).toBe("optd-orctago-hubtel-01CUSTOMKEY12345678901234");
  });

  it("createCharge maps succeeded", async () => {
    mockFetchOnce(async () =>
      jsonResponse(
        {
          ref: "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV",
          gateway: "hubtel",
          status: "succeeded",
          amount_pesewas: 1800,
          currency: "GHS",
          external_ref: "EXT",
        },
        { status: 201 },
      ),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data, error } = await client.createCharge({
      product: "orctago",
      amount_pesewas: 1800,
      wallet: "0241234567",
    });
    expect(error).toBeNull();
    expect(data?.status).toBe("succeeded");
    if (data?.status === "succeeded") {
      expect(data.amount_pesewas).toBe(1800);
      expect(data.currency).toBe("GHS");
    }
  });

  it("getChargeStatus fetches and returns", async () => {
    let gotPath = "";
    let gotAuth = "";
    mockFetchOnce(async (url, init) => {
      gotPath = new URL(url).pathname;
      if (init?.headers instanceof Headers) gotAuth = init.headers.get("Authorization") ?? "";
      else gotAuth = (init?.headers as Record<string, string>)?.["Authorization"] ?? "";
      return jsonResponse(
        {
          ref: "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV",
          status: "succeeded",
          gateway: "hubtel",
          amount_pesewas: 1800,
          verified_at: "2026-08-31T00:00:00Z",
        },
        { status: 200 },
      );
    });

    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data, error } = await client.getChargeStatus("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV");
    expect(error).toBeNull();
    expect(gotPath).toBe("/v1/charges/optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV/status");
    expect(gotAuth).toBe("Bearer k");
    expect(data?.status).toBe("succeeded");
    expect(data?.ref).toBeTruthy();
  });

  it("createPayout posts entries and returns batch", async () => {
    let gotBody: Record<string, unknown> = {};
    mockFetchOnce(async (_url, init) => {
      gotBody = JSON.parse(init?.body as string);
      const auth =
        init?.headers instanceof Headers
          ? init.headers.get("Authorization")
          : (init?.headers as Record<string, string>)?.["Authorization"];
      expect(auth).toBe("Bearer k");
      return jsonResponse(
        {
          batch_id: "550e8400-e29b-41d4-a716-446655440000",
          product: "orctago",
          status: "pending",
          total_pesewas: 2000,
          created_at: "2026-08-31T00:00:00Z",
        },
        { status: 201 },
      );
    });

    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data, error } = await client.createPayout({
      product: "orctago",
      entries: [
        { recipient: "0241111111", amount_pesewas: 1000 },
        { recipient: "0242222222", amount_pesewas: 1000 },
      ],
    });
    expect(error).toBeNull();
    expect(data?.batch_id).toBe("550e8400-e29b-41d4-a716-446655440000");
    expect(data?.total_pesewas).toBe(2000);
    expect(gotBody["product"]).toBe("orctago");
    const entries = gotBody["entries"] as unknown[];
    expect(entries.length).toBe(2);
  });

  it("createCharge returns error on 401", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "unauthorized", message: "bad key" } }, { status: 401 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "bad" });
    const { data, error } = await client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" });
    expect(data).toBeNull();
    expect(error).not.toBeNull();
    expect(error?.statusCode).toBe(401);
    expect(error?.code).toBe("unauthorized");
  });

  it("returns error on 500", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data: d1, error: e1 } = await client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" });
    expect(d1).toBeNull();
    expect(e1?.statusCode).toBe(500);

    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    const { data: d2, error: e2 } = await client.getChargeStatus("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV");
    expect(d2).toBeNull();
    expect(e2?.statusCode).toBe(500);

    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    const { data: d3, error: e3 } = await client.createPayout({
      product: "orctago",
      entries: [{ recipient: "0241", amount_pesewas: 100 }],
    });
    expect(d3).toBeNull();
    expect(e3?.statusCode).toBe(500);
  });

  it("returns error on 404 not_found", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "not_found", message: "no such charge" } }, { status: 404 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    const { data, error } = await client.getChargeStatus("optd-orctago-hubtel-01NOTFOUND0000000000000");
    expect(data).toBeNull();
    expect(error?.statusCode).toBe(404);
    expect(error?.code).toBe("not_found");
  });

  it("times out", async () => {
    mockFetchOnce(async (_url, init) => {
      return new Promise<Response>((_resolve, reject) => {
        const sig = init?.signal as AbortSignal | undefined;
        if (sig) {
          sig.addEventListener("abort", () =>
            reject(Object.assign(new Error("aborted"), { name: "AbortError" })),
          );
        }
      });
    });

    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k", timeout: 50 });
    const { data, error } = await client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" });
    expect(data).toBeNull();
    expect(error?.statusCode).toBe(408);
    expect(error?.code).toBe("timeout");
  });

  it("trims trailing slash from baseUrl", async () => {
    let gotUrl = "";
    mockFetchOnce(async (url) => {
      gotUrl = url;
      return jsonResponse(
        { ref: "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV", gateway: "hubtel", status: "pending" },
        { status: 201 },
      );
    });
    const client = new OrctaPay({ baseUrl: "http://example.test///", apiKey: "k" });
    const { data, error } = await client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "024" });
    expect(error).toBeNull();
    expect(data).not.toBeNull();
    expect(gotUrl).toBe("http://example.test/v1/charges");
  });

  it("generateReference format via createCharge", async () => {
    const { generateReference } = await import("../src/reference.js");
    const ref = generateReference("pos", "HubTel");
    expect(ref).toMatch(/^optd-pos-hubtel-[0-9A-Z]{26}$/);
  });
});
