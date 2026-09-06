const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

/** Generate a 26-char Crockford Base32 ULID-like string. */
export function newULID(): string {
  const entropy = new Uint8Array(10);
  // Runtime-agnostic random.
  const gCrypto = (globalThis as unknown as { crypto?: Crypto }).crypto;
  if (gCrypto?.getRandomValues) {
    gCrypto.getRandomValues(entropy);
  } else {
    // Fallback — insecure, only for environments without crypto.
    for (let i = 0; i < 10; i++) {
      entropy[i] = Math.floor(Math.random() * 256);
    }
  }

  // 48-bit millis; avoid 32-bit bitwise truncation.
  const ms = Date.now() % 0x1000000000000;
  const id = new Uint8Array(16);
  id[0] = Math.floor(ms / 0x10000000000) & 0xff;
  id[1] = Math.floor(ms / 0x100000000) & 0xff;
  id[2] = Math.floor(ms / 0x1000000) & 0xff;
  id[3] = Math.floor(ms / 0x10000) & 0xff;
  id[4] = Math.floor(ms / 0x100) & 0xff;
  id[5] = ms & 0xff;
  id.set(entropy, 6);
  return encodeCrockford(id);
}

function encodeCrockford(b: Uint8Array): string {
  const out = new Array<string>(26);
  let buffer = 0;
  let bitsLeft = 0;
  let idx = 0;
  for (const byte of b) {
    buffer = (buffer << 8) | byte;
    bitsLeft += 8;
    while (bitsLeft >= 5 && idx < 26) {
      bitsLeft -= 5;
      out[idx++] = CROCKFORD[(buffer >> bitsLeft) & 0x1f]!;
    }
  }
  if (idx < 26) {
    out[idx] = CROCKFORD[(buffer << (5 - bitsLeft)) & 0x1f]!;
  }
  return out.join("");
}

/**
 * Build `optd-{product}-{ulid}`.
 *
 * No gateway segment: the gateway is chosen server-side per charge
 * (hubtel / paystack / moolre, ranked by the router), so the client cannot
 * know it up front. The authoritative gateway is returned on ChargeResult.
 */
export function generateReference(product: string): string {
  const p = product || "default";
  return `optd-${p}-${newULID()}`;
}
