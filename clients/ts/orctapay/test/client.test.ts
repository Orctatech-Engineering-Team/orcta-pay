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
      // also handle Headers instance
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
    const res = await client.createCharge({
      product: "orctago",
      amount_pesewas: 1800,
      currency: "GHS",
      wallet: "0241234567",
    });

    expect(gotAuth).toBe("Bearer test-key");
    const key = gotBody["idempotency_key"] as string;
    expect(key.startsWith("optd-")).toBe(true);
    expect(key).toMatch(/^optd-orctago-hubtel-[0-9A-Z]{26}$/);
    expect(res.status).toBe("pending");
    if (res.status === "pending") {
      expect(res.ref).toBe("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV");
      expect(res.gateway).toBe("hubtel");
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
    await client.createCharge({
      product: "orctago",
      amount_pesewas: 100,
      currency: "GHS",
      wallet: "0241234567",
      idempotency_key: "optd-orctago-hubtel-01CUSTOMKEY12345678901234",
    });

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
    const res = await client.createCharge({
      product: "orctago",
      amount_pesewas: 1800,
      wallet: "0241234567",
    });
    expect(res.status).toBe("succeeded");
    if (res.status === "succeeded") {
      expect(res.amount_pesewas).toBe(1800);
      expect(res.currency).toBe("GHS");
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
    const st = await client.getChargeStatus("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV");
    expect(gotPath).toBe("/v1/charges/optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV/status");
    expect(gotAuth).toBe("Bearer k");
    expect(st.status).toBe("succeeded");
    expect(st.ref).toBeTruthy();
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
    const res = await client.createPayout({
      product: "orctago",
      entries: [
        { recipient: "0241111111", amount_pesewas: 1000 },
        { recipient: "0242222222", amount_pesewas: 1000 },
      ],
    });
    expect(res.batch_id).toBe("550e8400-e29b-41d4-a716-446655440000");
    expect(res.total_pesewas).toBe(2000);
    expect(gotBody["product"]).toBe("orctago");
    const entries = gotBody["entries"] as unknown[];
    expect(entries.length).toBe(2);
  });

  it("throws OrctaPayError on 401", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "unauthorized", message: "bad key" } }, { status: 401 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "bad" });
    await expect(
      client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" }),
    ).rejects.toMatchObject({ statusCode: 401, code: "unauthorized" } as Partial<OrctaPayError>);
  });

  it("throws on 500 with internal_error", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    await expect(
      client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" }),
    ).rejects.toMatchObject({ statusCode: 500 } as Partial<OrctaPayError>);

    // Also for getChargeStatus and createPayout.
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    await expect(client.getChargeStatus("optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV")).rejects.toMatchObject({
      statusCode: 500,
    } as Partial<OrctaPayError>);

    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "internal_error", message: "boom" } }, { status: 500 }),
    );
    await expect(
      client.createPayout({
        product: "orctago",
        entries: [{ recipient: "0241", amount_pesewas: 100 }],
      }),
    ).rejects.toMatchObject({ statusCode: 500 } as Partial<OrctaPayError>);
  });

  it("throws on 404 not_found", async () => {
    mockFetchOnce(async () =>
      jsonResponse({ error: { code: "not_found", message: "no such charge" } }, { status: 404 }),
    );
    const client = new OrctaPay({ baseUrl: "http://example.test", apiKey: "k" });
    await expect(client.getChargeStatus("optd-orctago-hubtel-01NOTFOUND0000000000000")).rejects.toMatchObject({
      statusCode: 404,
      code: "not_found",
    } as Partial<OrctaPayError>);
  });

  it("times out", async () => {
    mockFetchOnce(async (_url, init) => {
      // Respect abort signal like a real server would hang.
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
    await expect(
      client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "0241234567" }),
    ).rejects.toMatchObject({ statusCode: 408, code: "timeout" } as Partial<OrctaPayError>);
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
    await client.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "024" });
    expect(gotUrl).toBe("http://example.test/v1/charges");
  });

  it("generateReference format via createCharge", async () => {
    const { generateReference } = await import("../src/reference.js");
    const ref = generateReference("pos", "HubTel");
    expect(ref).toMatch(/^optd-pos-hubtel-[0-9A-Z]{26}$/);
  });
});
