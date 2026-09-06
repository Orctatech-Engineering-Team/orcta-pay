/** Request for POST /v1/charges. Amount is whole pesewas in GHS. */
export interface CreateChargeRequest {
  product: string;
  amount_pesewas: number;
  currency?: string;
  wallet?: string;
  phone?: string;
  idempotency_key?: string;
  metadata?: Record<string, unknown>;
}

/** Sealed result of a charge. Mirrors Go's ChargeSucceeded | ChargePending | ChargeFailed. */
export type ChargeResult =
  | {
      status: "succeeded";
      ref: string;
      gateway: string;
      amount_pesewas: number;
      currency: string;
      external_ref?: string;
    }
  | {
      status: "pending";
      ref: string;
      gateway: string;
      external_ref?: string;
    }
  | {
      status: "failed";
      reason: string;
    };

/** Authoritative status from GET /v1/charges/{ref}/status. */
export interface ChargeStatus {
  ref: string;
  status: string;
  gateway: string;
  amount_pesewas: number;
  verified_at: string;
}

export type GetChargeStatusResponse = ChargeStatus;

/** One recipient in a payout batch. */
export interface PayoutEntry {
  recipient: string;
  amount_pesewas: number;
  currency?: string;
}

/** Request for POST /v1/payouts. */
export interface CreatePayoutRequest {
  product: string;
  entries: PayoutEntry[];
  idempotency_key?: string;
}

/** Created payout batch. */
export interface PayoutResult {
  batch_id: string;
  product: string;
  status: string;
  total_pesewas: number;
  created_at: string;
}

/** Request for POST /v1/apps. */
export interface CreateAppRequest {
  name: string;
  product: string;
}

/** Response from POST /v1/apps — api_key shown once. */
export interface CreateAppResponse {
  id: string;
  name: string;
  product: string;
  api_key: string;
  prefix: string;
}

/** App record from GET /v1/apps. */
export interface App {
  id: string;
  name: string;
  product: string;
  prefix: string;
  created_at: string;
  last_used_at?: string | null;
  revoked?: boolean;
  revoked_at?: string | null;
}

/** Response from POST /v1/apps/{id}/keys/rotate. */
export interface RotateAppKeyResponse {
  api_key: string;
  prefix?: string;
}
