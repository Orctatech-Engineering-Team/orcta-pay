export function formatGHS(pesewas: number): string {
  const ghs = pesewas / 100;
  return new Intl.NumberFormat("en-GH", {
    style: "currency",
    currency: "GHS",
    minimumFractionDigits: 2,
  }).format(ghs);
}

export function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString("en-GH", {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return iso;
  }
}

export function ageMinutes(iso: string, now = Date.now()): string {
  const diff = now - new Date(iso).getTime();
  if (diff < 60_000) return `${Math.max(0, Math.floor(diff / 1000))}s`;
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}m`;
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)}h`;
  return `${Math.floor(diff / 86400_000)}d`;
}
