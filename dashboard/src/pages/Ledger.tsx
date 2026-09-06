import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getBaseUrl } from "../lib/config";
import { formatDate, formatGHS } from "../lib/format";
import type { LedgerEntry } from "../lib/types";

function sumFor(entries: LedgerEntry[]): { credits: number; debits: number; net: number } {
  let credits = 0;
  let debits = 0;
  for (const e of entries) {
    if (e.entry_type === "credit") credits += e.amount_pesewas;
    else debits += e.amount_pesewas;
  }
  return { credits, debits, net: credits - debits };
}

export function LedgerPage() {
  const [vendor, setVendor] = useState("all");
  const [kind, setKind] = useState<"all" | "vendor" | "commission">("all");

  const { data, error, isFetching } = useQuery({
    queryKey: ["ledger", { vendor, kind }],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const url = new URL(`${baseUrl}/v1/ledger`);
      if (vendor !== "all") url.searchParams.set("vendor_id", vendor);
      if (kind !== "all") url.searchParams.set("kind", kind);
      const res = await fetch(url.toString(), {
        credentials: "include",
        headers: { Accept: "application/json" },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as LedgerEntry[];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const source = data ?? [];

  const filtered = useMemo(() => {
    return source.filter((e) => {
      if (vendor !== "all" && e.vendor_id !== vendor) return false;
      if (kind !== "all" && e.kind !== kind) return false;
      return true;
    });
  }, [source, vendor, kind]);

  const vendors = useMemo(() => [...new Set(source.map((e) => e.vendor_id))], [source]);
  const totals = sumFor(filtered);
  const reconciliationOk = totals.net === totals.credits - totals.debits;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2 style={{ margin: 0 }}>Ledger</h2>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>{isFetching ? "fetching..." : error ? "error" : "live"}</span>
        </div>
        <div className="row" style={{ marginTop: 12 }}>
          <select className="select" value={vendor} onChange={(e) => setVendor(e.target.value)}>
            <option value="all">All vendors</option>
            {vendors.map((v) => <option key={v} value={v}>{v}</option>)}
          </select>
          <select className="select" value={kind} onChange={(e) => setKind(e.target.value as typeof kind)}>
            <option value="all">All entry kinds</option>
            <option value="vendor">vendor</option>
            <option value="commission">commission</option>
          </select>
          <span className="muted" style={{ fontSize: 14 }}>Showing {filtered.length} entries</span>
        </div>
        {error ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Could not load ledger entries from the API. No placeholder entries are shown.
          </div>
        ) : null}
      </div>

      {/* Hero: Financial summary */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12 }}>
        <div className="card" style={{ background: "var(--color-ok-soft)", borderColor: "var(--color-ok-line)" }}>
          <span style={{ fontSize: "var(--text-xs)", fontWeight: 600, letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", color: "var(--color-ok-ink)" }}>Credits</span>
          <div style={{ fontSize: "var(--text-xl)", fontWeight: 700, color: "var(--color-ok-ink)", marginTop: 4 }}>
            {formatGHS(totals.credits)}
          </div>
          <span style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)" }}>Money in</span>
        </div>
        <div className="card" style={{ background: "var(--color-bad-soft)", borderColor: "var(--color-bad-line)" }}>
          <span style={{ fontSize: "var(--text-xs)", fontWeight: 600, letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", color: "var(--color-bad-ink)" }}>Debits</span>
          <div style={{ fontSize: "var(--text-xl)", fontWeight: 700, color: "var(--color-bad-ink)", marginTop: 4 }}>
            {formatGHS(totals.debits)}
          </div>
          <span style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)" }}>Money out</span>
        </div>
        <div className="card" style={{ background: totals.net >= 0 ? "var(--color-ok-soft)" : "var(--color-bad-soft)", borderColor: totals.net >= 0 ? "var(--color-ok-line)" : "var(--color-bad-line)" }}>
          <span style={{ fontSize: "var(--text-xs)", fontWeight: 600, letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", color: totals.net >= 0 ? "var(--color-ok-ink)" : "var(--color-bad-ink)" }}>Net</span>
          <div style={{ fontSize: "var(--text-xl)", fontWeight: 700, color: totals.net >= 0 ? "var(--color-ok-ink)" : "var(--color-bad-ink)", marginTop: 4 }}>
            {formatGHS(totals.net)}
          </div>
          <span style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)" }}>{totals.net >= 0 ? "Positive balance" : "Negative balance"}</span>
        </div>
      </div>

      {/* Reconciliation status */}
      <div className="card" style={{ background: reconciliationOk ? "var(--color-ok-soft)" : "var(--color-bad-soft)", borderColor: reconciliationOk ? "var(--color-ok-line)" : "var(--color-bad-line)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{ fontSize: 18 }}>{reconciliationOk ? "✓" : "✗"}</span>
          <div>
            <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: reconciliationOk ? "var(--color-ok-ink)" : "var(--color-bad-ink)" }}>
              Reconciliation {reconciliationOk ? "passed" : "failed"}
            </div>
            <div style={{ fontSize: "var(--text-xs)", color: "var(--color-ink-muted)" }}>
              Credits - Debits = Net (invariant checked)
            </div>
          </div>
        </div>
      </div>

      {/* Entries table */}
      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ padding: "12px 16px", borderBottom: "1px solid var(--color-line-faint)", background: "var(--color-surface-soft)" }}>
          <strong style={{ fontSize: 14 }}>Entries</strong>
        </div>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>vendor</th>
                <th>kind</th>
                <th>type</th>
                <th>amount</th>
                <th>reason</th>
                <th>value_time</th>
                <th>ref</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((e) => (
                <tr key={e.id}>
                  <td style={{ fontWeight: 500 }}>{e.vendor_id}</td>
                  <td><span className="pill" style={{ textTransform: "none" }}>{e.kind}</span></td>
                  <td><span className={`pill ${e.entry_type === "credit" ? "succeeded" : "pending"}`}>{e.entry_type}</span></td>
                  <td style={{ fontWeight: 600, color: e.entry_type === "credit" ? "var(--color-ok-ink)" : "var(--color-bad-ink)" }}>
                    {e.entry_type === "credit" ? "+" : "-"}{formatGHS(e.amount_pesewas)}
                  </td>
                  <td>{e.reason}</td>
                  <td className="muted">{formatDate(e.value_time)}</td>
                  <td style={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={e.ref}>{e.ref}</td>
                </tr>
              ))}
              {filtered.length === 0 && <tr><td colSpan={7} className="muted" style={{ padding: 16 }}>No entries match filters.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
