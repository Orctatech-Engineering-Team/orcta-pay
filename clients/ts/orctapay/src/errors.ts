/**
 * Typed error thrown by {@link OrctaPay} on non-2xx or network failures.
 */
export class OrctaPayError extends Error {
  readonly statusCode: number;
  readonly code: string;
  readonly cause?: unknown;

  constructor(
    message: string,
    statusCode: number,
    code: string,
    cause?: unknown,
  ) {
    super(message);
    this.name = "OrctaPayError";
    this.statusCode = statusCode;
    this.code = code;
    this.cause = cause;
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, OrctaPayError);
    }
  }
}
