// Mock data for when the API is not reachable. Shapes mirror the SQL sketches
// in PAYMENTS_SERVICE_DESIGN.md and api/openapi.yaml.

export type ChargeRow = {
  ref: string;
  product: string;
  gateway: string;
  amount_pesewas: number;
  currency: string;
  status: "pending" | "succeeded" | "failed";
  external_ref?: string;
  created_at: string;
  gateway_event?: { raw_request: unknown; raw_response: unknown };
};

export type PayoutBatchRow = {
  id: string;
  batch_date: string;
  status: "running" | "completed" | "partially_failed";
  vendor_count: number;
  gross_pesewas: number;
  commission_pesewas: number;
  net_pesewas: number;
  created_at: string;
  completed_at?: string;
  lines: PayoutLine[];
};

export type PayoutLine = {
  vendor_id: string;
  vendor_name: string;
  amount_pesewas: number;
  net_pesewas: number;
  recipient: string;
  reservation_status: "open" | "settled" | "released";
  reservation_created_at: string;
  reservation_resolved_at?: string;
};

export type LedgerEntry = {
  id: string;
  product: string;
  wallet: string;
  vendor_id: string;
  entry_type: "credit" | "debit";
  amount_pesewas: number;
  reason: "order_payment" | "commission_deduction" | "payout" | "adjustment";
  ref: string;
  value_time: string;
  booking_time: string;
  settlement_time: string | null;
  kind: "vendor" | "commission";
};

export type GatewayHealthRow = {
  gateway: string;
  channel: string;
  eligible: boolean;
  rolling_success_rate: number;
  p95_latency_ms: number;
  circuit_state: "closed" | "open" | "half_open";
  cost_bps: number;
  cost_fixed_pesewas: number;
  rank: number;
  open_since?: string;
};

export type WebhookRow = {
  id: string;
  aggregator_event_id: string;
  gateway: string;
  kind: string;
  payload: unknown;
  received_at: string;
  processed_at: string | null;
};

export type AppRow = {
  id: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at: string | null;
  revoked: boolean;
};

function ulidLike(seed: number): string {
  const base = "01ARZ3NDEKTSV4RRFFQ69G5FAV";
  return base.slice(0, 22) + String(seed).padStart(4, "0");
}

const now = Date.now();
const iso = (offsetMs: number) => new Date(now - offsetMs).toISOString();

export const mockCharges: ChargeRow[] = [
  {
    ref: `optd-orctago-hubtel-${ulidLike(1)}`,
    product: "orctago",
    gateway: "hubtel",
    amount_pesewas: 1850,
    currency: "GHS",
    status: "succeeded",
    external_ref: "HB-88421",
    created_at: iso(3600_000 * 2),
    gateway_event: {
      raw_request: { ClientReference: `optd-orctago-hubtel-${ulidLike(1)}`, Amount: "18.50" },
      raw_response: { status: "Success", transactionId: "HB-88421" },
    },
  },
  {
    ref: `optd-orctago-paystack-${ulidLike(2)}`,
    product: "orctago",
    gateway: "paystack",
    amount_pesewas: 5000,
    currency: "GHS",
    status: "pending",
    external_ref: "PSK_9f12aa",
    created_at: iso(3600_000 * 5),
    gateway_event: {
      raw_request: { reference: `optd-orctago-paystack-${ulidLike(2)}`, amount: 5000 },
      raw_response: { status: true, data: { status: "pending" } },
    },
  },
  {
    ref: `optd-pos-moolre-${ulidLike(3)}`,
    product: "pos",
    gateway: "moolre",
    amount_pesewas: 12000,
    currency: "GHS",
    status: "failed",
    external_ref: "MO-7781",
    created_at: iso(3600_000 * 24),
  },
  {
    ref: `optd-orctago-hubtel-${ulidLike(4)}`,
    product: "orctago",
    gateway: "hubtel",
    amount_pesewas: 2500,
    currency: "GHS",
    status: "pending",
    created_at: iso(600_000),
  },
  {
    ref: `optd-pos-hubtel-${ulidLike(5)}`,
    product: "pos",
    gateway: "hubtel",
    amount_pesewas: 750,
    currency: "GHS",
    status: "succeeded",
    external_ref: "HB-88455",
    created_at: iso(3600_000 * 48),
  },
];

export const mockPayoutBatches: PayoutBatchRow[] = [
  {
    id: "batch-2026-08-30",
    batch_date: "2026-08-30",
    status: "completed",
    vendor_count: 3,
    gross_pesewas: 45200,
    commission_pesewas: 4520,
    net_pesewas: 40680,
    created_at: iso(3600_000 * 26),
    completed_at: iso(3600_000 * 24),
    lines: [
      {
        vendor_id: "vendor-001",
        vendor_name: "Ama's Kitchen",
        amount_pesewas: 18500,
        net_pesewas: 16650,
        recipient: "0241111111",
        reservation_status: "settled",
        reservation_created_at: iso(3600_000 * 26),
        reservation_resolved_at: iso(3600_000 * 24),
      },
      {
        vendor_id: "vendor-002",
        vendor_name: "Kofi Grill",
        amount_pesewas: 15200,
        net_pesewas: 13680,
        recipient: "0242222222",
        reservation_status: "settled",
        reservation_created_at: iso(3600_000 * 26),
        reservation_resolved_at: iso(3600_000 * 24),
      },
      {
        vendor_id: "vendor-003",
        vendor_name: "Zion Mart",
        amount_pesewas: 11500,
        net_pesewas: 10350,
        recipient: "0243333333",
        reservation_status: "settled",
        reservation_created_at: iso(3600_000 * 26),
        reservation_resolved_at: iso(3600_000 * 24),
      },
    ],
  },
  {
    id: "batch-2026-08-31",
    batch_date: "2026-08-31",
    status: "running",
    vendor_count: 2,
    gross_pesewas: 30000,
    commission_pesewas: 3000,
    net_pesewas: 27000,
    created_at: iso(3600_000 * 3),
    lines: [
      {
        vendor_id: "vendor-001",
        vendor_name: "Ama's Kitchen",
        amount_pesewas: 18000,
        net_pesewas: 16200,
        recipient: "0241111111",
        reservation_status: "open",
        reservation_created_at: iso(3600_000 * 2),
      },
      {
        vendor_id: "vendor-004",
        vendor_name: "Sunset Cafe",
        amount_pesewas: 12000,
        net_pesewas: 10800,
        recipient: "0244444444",
        reservation_status: "open",
        reservation_created_at: iso(3600_000 * 1),
      },
    ],
  },
  {
    id: "batch-2026-08-29",
    batch_date: "2026-08-29",
    status: "partially_failed",
    vendor_count: 2,
    gross_pesewas: 22000,
    commission_pesewas: 2200,
    net_pesewas: 19800,
    created_at: iso(3600_000 * 72),
    completed_at: iso(3600_000 * 70),
    lines: [
      {
        vendor_id: "vendor-002",
        vendor_name: "Kofi Grill",
        amount_pesewas: 12000,
        net_pesewas: 10800,
        recipient: "0242222222",
        reservation_status: "settled",
        reservation_created_at: iso(3600_000 * 72),
        reservation_resolved_at: iso(3600_000 * 70),
      },
      {
        vendor_id: "vendor-005",
        vendor_name: "Edge Bistro",
        amount_pesewas: 10000,
        net_pesewas: 9000,
        recipient: "0245555555",
        reservation_status: "released",
        reservation_created_at: iso(3600_000 * 72),
        reservation_resolved_at: iso(3600_000 * 71),
      },
    ],
  },
];

export const mockLedger: LedgerEntry[] = [
  {
    id: "le-001",
    product: "orctago",
    wallet: "0241111111",
    vendor_id: "vendor-001",
    entry_type: "credit",
    amount_pesewas: 18500,
    reason: "order_payment",
    ref: mockCharges[0]!.ref,
    value_time: iso(3600_000 * 2),
    booking_time: iso(3600_000 * 2 - 120_000),
    settlement_time: iso(3600_000 * 1),
    kind: "vendor",
  },
  {
    id: "le-002",
    product: "orctago",
    wallet: "0241111111",
    vendor_id: "vendor-001",
    entry_type: "debit",
    amount_pesewas: 1850,
    reason: "commission_deduction",
    ref: mockCharges[0]!.ref,
    value_time: iso(3600_000 * 2),
    booking_time: iso(3600_000 * 2 - 120_000),
    settlement_time: iso(3600_000 * 1),
    kind: "commission",
  },
  {
    id: "le-003",
    product: "orctago",
    wallet: "0241111111",
    vendor_id: "vendor-001",
    entry_type: "debit",
    amount_pesewas: 16650,
    reason: "payout",
    ref: "batch-2026-08-30",
    value_time: iso(3600_000 * 24),
    booking_time: iso(3600_000 * 24),
    settlement_time: iso(3600_000 * 23),
    kind: "vendor",
  },
  {
    id: "le-004",
    product: "pos",
    wallet: "0243333333",
    vendor_id: "vendor-003",
    entry_type: "credit",
    amount_pesewas: 12000,
    reason: "order_payment",
    ref: mockCharges[2]!.ref,
    value_time: iso(3600_000 * 24),
    booking_time: iso(3600_000 * 24),
    settlement_time: null,
    kind: "vendor",
  },
];

export const mockGateways: GatewayHealthRow[] = [
  {
    gateway: "hubtel",
    channel: "mtn-gh",
    eligible: true,
    rolling_success_rate: 0.982,
    p95_latency_ms: 420,
    circuit_state: "closed",
    cost_bps: 195,
    cost_fixed_pesewas: 0,
    rank: 1,
  },
  {
    gateway: "paystack",
    channel: "mtn-gh",
    eligible: true,
    rolling_success_rate: 0.976,
    p95_latency_ms: 310,
    circuit_state: "closed",
    cost_bps: 200,
    cost_fixed_pesewas: 0,
    rank: 2,
  },
  {
    gateway: "moolre",
    channel: "mtn-gh",
    eligible: false,
    rolling_success_rate: 0.88,
    p95_latency_ms: 890,
    circuit_state: "open",
    cost_bps: 150,
    cost_fixed_pesewas: 20,
    rank: 99,
    open_since: iso(3600_000),
  },
  {
    gateway: "hubtel",
    channel: "vodafone-gh",
    eligible: true,
    rolling_success_rate: 0.991,
    p95_latency_ms: 380,
    circuit_state: "closed",
    cost_bps: 195,
    cost_fixed_pesewas: 0,
    rank: 1,
  },
  {
    gateway: "paystack",
    channel: "vodafone-gh",
    eligible: true,
    rolling_success_rate: 0.945,
    p95_latency_ms: 340,
    circuit_state: "half_open",
    cost_bps: 200,
    cost_fixed_pesewas: 0,
    rank: 3,
  },
];

export const mockWebhooks: WebhookRow[] = [
  {
    id: "wh-001",
    aggregator_event_id: "hubtel-evt-9f12aa",
    gateway: "hubtel",
    kind: "charge.succeeded",
    payload: { ClientReference: mockCharges[0]!.ref, Status: "Success", Amount: "18.50" },
    received_at: iso(3600_000 * 1 + 300_000),
    processed_at: iso(3600_000 * 1),
  },
  {
    id: "wh-002",
    aggregator_event_id: "hubtel-evt-9f12aa",
    gateway: "hubtel",
    kind: "charge.succeeded",
    payload: { ClientReference: mockCharges[0]!.ref, Status: "Success", Amount: "18.50" },
    received_at: iso(3600_000 * 1 + 120_000),
    processed_at: null,
  },
  {
    id: "wh-003",
    aggregator_event_id: "psk-evt-cc45",
    gateway: "paystack",
    kind: "charge.pending",
    payload: { reference: mockCharges[1]!.ref, status: "pending" },
    received_at: iso(3600_000 * 4),
    processed_at: iso(3600_000 * 4 - 30_000),
  },
  {
    id: "wh-004",
    aggregator_event_id: "moolre-tx-7781",
    gateway: "moolre",
    kind: "charge.failed",
    payload: { externalref: mockCharges[2]!.ref, status: "failed", reason: "insufficient_funds" },
    received_at: iso(3600_000 * 23),
    processed_at: iso(3600_000 * 23 - 10_000),
  },
];

export const mockApps: AppRow[] = [
  {
    id: "app_01ARZ3NDEKTSV4RR0001",
    name: "orctago",
    prefix: "pay_live_orctago_abc1",
    created_at: iso(3600_000 * 48),
    last_used_at: iso(3600_000 * 1),
    revoked: false,
  },
  {
    id: "app_01ARZ3NDEKTSV4RR0002",
    name: "pos",
    prefix: "pay_live_pos_xyz2",
    created_at: iso(3600_000 * 72),
    last_used_at: iso(3600_000 * 5),
    revoked: false,
  },
  {
    id: "app_01ARZ3NDEKTSV4RR0003",
    name: "legacy-pos",
    prefix: "pay_live_pos_old9",
    created_at: iso(3600_000 * 240),
    last_used_at: null,
    revoked: true,
  },
];
