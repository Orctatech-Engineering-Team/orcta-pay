import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@tanstack/react-form";
import { Field } from "@base-ui/react/field";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { makeClient } from "../lib/api";
import { formatDate } from "../lib/format";
import type { AppRow } from "../lib/types";
import { showToast } from "../lib/toast";

export function AppsPage() {
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [newKey, setNewKey] = useState<{ api_key: string; prefix: string; name: string } | null>(null);
  const [rotatedKey, setRotatedKey] = useState<{ api_key: string; id: string } | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const { data, error, isLoading, isFetching } = useQuery({
    queryKey: ["apps"],
    queryFn: async () => {
      const client = makeClient();
      const { data: d, error: e } = await client.listApps();
      if (e) throw e;
      return (d as unknown as AppRow[]) ?? [];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const apps = data ?? [];
  const active = apps.filter((a) => !a.revoked).length;
  const revoked = apps.filter((a) => a.revoked).length;

  const form = useForm({
    defaultValues: { name: "" },
    onSubmit: async ({ value }) => {
      if (!value.name.trim()) {
        setErrorMsg("Name is required");
        return;
      }
      const client = makeClient();
      const { data: d, error: e } = await client.createApp({ name: value.name.trim(), product: value.name.trim() });
      if (e) {
        setErrorMsg(e.message);
        showToast(e.message, { type: "error" });
        return;
      }
      if (d) {
        setNewKey({ api_key: d.api_key, prefix: d.api_key_prefix, name: d.name });
        setCreateOpen(false);
        form.reset();
        void qc.invalidateQueries({ queryKey: ["apps"] });
        showToast("App created", { type: "success" });
      }
    },
  });

  const rotateMut = useMutation({
    mutationFn: async (appId: string) => {
      const client = makeClient();
      const { data: d, error: e } = await client.rotateAppKey(appId);
      if (e) throw e;
      return { appId, data: d };
    },
    onSuccess: (res) => {
      if (res?.data) {
        setRotatedKey({ api_key: res.data.api_key, id: res.appId });
        void qc.invalidateQueries({ queryKey: ["apps"] });
        showToast("Key rotated, copy now", { type: "success" });
      }
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : String(e);
      setErrorMsg(msg);
      showToast(msg, { type: "error" });
    },
  });

  const revokeMut = useMutation({
    mutationFn: async (appId: string) => {
      const client = makeClient();
      const { error: e } = await client.revokeApp(appId);
      if (e) throw e;
      return appId;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["apps"] });
      showToast("App revoked", { type: "success" });
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : String(e);
      setErrorMsg(msg);
      showToast(msg, { type: "error" });
    },
  });

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const el = document.createElement("textarea");
      el.value = text;
      document.body.appendChild(el);
      el.select();
      document.execCommand("copy");
      document.body.removeChild(el);
    }
    showToast("API key copied", { type: "success" });
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div>
            <h2 style={{ margin: 0 }}>Apps</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>API keys for your services. One key per app.</p>
          </div>
          <Button className="btn" onClick={() => { setCreateOpen(true); setErrorMsg(null); }}>
            Create app
          </Button>
        </div>
        {error ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Could not load apps from the API. No placeholder apps are shown.
          </div>
        ) : null}
        {isLoading ? <p className="muted" style={{ marginTop: 8 }}>Loading...</p> : <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{isFetching ? "fetching..." : error ? "error" : "live"}</span>}
        {errorMsg ? <pre style={{ background: "var(--color-bad-soft)", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap", marginTop: 10, border: "1px solid var(--color-bad-line)", overflowWrap: "anywhere" }}>{errorMsg}</pre> : null}
      </div>

      {createOpen && (
        <div className="card" style={{ borderColor: "var(--color-accent-ring)" }}>
          <div className="row" style={{ justifyContent: "space-between", marginBottom: 12 }}>
            <h3 style={{ margin: 0, fontSize: "var(--text-md)", fontWeight: 600 }}>Create app</h3>
            <Button className="btn ghost" style={{ padding: "4px 8px", fontSize: 12 }} onClick={() => setCreateOpen(false)}>
              Close
            </Button>
          </div>
          <p className="muted" style={{ marginBottom: 12, fontSize: "var(--text-sm)" }}>
            Name it after your service. The API key is shown once, then never again.
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void form.handleSubmit();
            }}
          >
            <div style={{ display: "flex", gap: 12, alignItems: "flex-start", flexWrap: "wrap" }}>
              <form.Field name="name">
                {(field) => (
                  <Field.Root style={{ flex: 1, minWidth: 200 }}>
                    <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>Name</Field.Label>
                    <Input
                      className="input"
                      style={{ width: "100%", marginTop: 6 }}
                      placeholder="e.g. orctago, orctapos, orctanet"
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.target.value)}
                      onBlur={field.handleBlur}
                    />
                    <span style={{ fontSize: 11, color: "var(--color-ink-faint)", marginTop: 4, display: "block" }}>
                      Usually the service name — used in charge refs and filtering
                    </span>
                  </Field.Root>
                )}
              </form.Field>

              <div style={{ display: "flex", gap: 8, marginTop: 22 }}>
                <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting]}>
                  {([canSubmit, isSubmitting]) => (
                    <Button type="submit" className="btn" disabled={!canSubmit || Boolean(isSubmitting)}>
                      {isSubmitting ? "Creating..." : "Create"}
                    </Button>
                  )}
                </form.Subscribe>
              </div>
            </div>
          </form>
        </div>
      )}

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Total apps</span>
          <span className="stat-value">{apps.length}</span>
          <span className="stat-meta">{active} active, {revoked} revoked</span>
        </div>
        <div className="stat">
          <span className="stat-label">Active</span>
          <span className="stat-value">{active}</span>
          <span className="stat-meta">Apps with valid keys</span>
        </div>
        <div className="stat">
          <span className="stat-label">Revoked</span>
          <span className="stat-value">{revoked}</span>
          <span className="stat-meta">Keys that no longer work</span>
        </div>
      </div>

      {newKey ? (
        <div className="card" style={{ borderColor: "var(--color-warn-line)", background: "var(--color-warn-soft)" }}>
          <h2 style={{ margin: 0, color: "var(--color-warn-ink)" }}>API key, copy now, shown once</h2>
          <p className="muted">This key for <strong>{newKey.name}</strong> will not be shown again. Store it in Vault.</p>
          <div style={{ background: "var(--color-ink-strong)", color: "var(--color-ink-inverse)", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, overflowWrap: "anywhere" }}>
            <code style={{ wordBreak: "break-all", color: "var(--color-ink-inverse)" }}>{newKey.api_key}</code>
            <Button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(newKey.api_key)}>
              Copy
            </Button>
          </div>
          <p className="muted" style={{ marginTop: 6 }}>Prefix: <code>{newKey.prefix}</code></p>
          <Button className="btn ghost" style={{ marginTop: 8 }} onClick={() => setNewKey(null)}>
            Dismiss
          </Button>
        </div>
      ) : null}

      {rotatedKey ? (
        <div className="card" style={{ borderColor: "var(--color-accent-ring)", background: "var(--color-accent-faint)" }}>
          <h2 style={{ margin: 0 }}>Rotated key, copy now, shown once</h2>
          <div style={{ background: "var(--color-ink-strong)", color: "var(--color-ink-inverse)", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, overflowWrap: "anywhere" }}>
            <code style={{ wordBreak: "break-all", color: "var(--color-ink-inverse)" }}>{rotatedKey.api_key}</code>
            <Button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(rotatedKey.api_key)}>
              Copy
            </Button>
          </div>
          <Button className="btn ghost" style={{ marginTop: 8 }} onClick={() => setRotatedKey(null)}>
            Dismiss
          </Button>
        </div>
      ) : null}

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>name</th>
                <th>prefix</th>
                <th>created_at</th>
                <th>last_used_at</th>
                <th>revoked</th>
                <th>actions</th>
              </tr>
            </thead>
            <tbody>
              {apps.map((a) => (
                <tr key={a.id} style={{ opacity: a.revoked ? 0.6 : 1 }}>
                  <td style={{ fontWeight: 500 }}>{a.name}</td>
                  <td>{a.prefix}</td>
                  <td className="muted">{formatDate(a.created_at)}</td>
                  <td className="muted">{a.last_used_at ? formatDate(a.last_used_at) : "-"}</td>
                  <td>{a.revoked ? <span className="pill failed">revoked</span> : <span className="pill succeeded">active</span>}</td>
                  <td>
                    <div className="row" style={{ gap: 6 }}>
                      <Button
                        className="btn ghost"
                        style={{ padding: "4px 8px", fontSize: 12 }}
                        disabled={a.revoked || rotateMut.isPending}
                        onClick={() => { setErrorMsg(null); void rotateMut.mutateAsync(a.id); }}
                      >
                        {rotateMut.isPending ? "Rotating..." : "Rotate"}
                      </Button>
                      <Button
                        className="btn ghost"
                        style={{ padding: "4px 8px", fontSize: 12 }}
                        disabled={a.revoked || revokeMut.isPending}
                        onClick={() => { if (confirm(`Revoke ${a.name}?`)) { setErrorMsg(null); void revokeMut.mutateAsync(a.id); } }}
                      >
                        Revoke
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {apps.length === 0 && <tr><td colSpan={6} className="muted" style={{ padding: 16 }}>No apps yet. Create one.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
