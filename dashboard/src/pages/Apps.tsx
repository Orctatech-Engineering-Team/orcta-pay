import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@tanstack/react-form";
import { Dialog } from "@base-ui/react/dialog";
import { Field } from "@base-ui/react/field";
import { Select } from "@base-ui/react/select";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { makeClient } from "../lib/api";
import { formatDate } from "../lib/format";
import { mockApps, type AppRow } from "../lib/mock";
import { createAppSchema } from "../lib/validators";
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

  const apps = data ?? mockApps;
  const showMockBanner = !!error;
  const active = apps.filter((a) => !a.revoked).length;
  const revoked = apps.filter((a) => a.revoked).length;

  const form = useForm({
    defaultValues: { name: "", product: "orctago" as string },
    onSubmit: async ({ value }) => {
      const parsed = createAppSchema.safeParse(value);
      if (!parsed.success) {
        setErrorMsg(parsed.error.issues.map((i) => i.message).join(", "));
        return;
      }
      const client = makeClient();
      const { data: d, error: e } = await client.createApp(parsed.data);
      if (e) {
        setErrorMsg(e.message);
        showToast(e.message, { type: "error" });
        return;
      }
      if (d) {
        setNewKey({ api_key: d.api_key, prefix: d.prefix, name: d.name });
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
        showToast("Key rotated — copy now", { type: "success" });
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
            <h2 style={{ margin: 0, fontSize: 16 }}>Apps — API keys</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              Each service creates an app and gets a per-product <code>pay_live_…</code> key. Keys are stored in Vault at{" "}
              <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> and rendered to <code>ORCTA_PAY_API_KEY</code>. Use TanStack Mutation <code>createApp</code> with <code>invalidate ["apps"]</code> — mock fallback when offline.
            </p>
          </div>
          <Button className="btn" onClick={() => { setCreateOpen(true); setErrorMsg(null); }}>
            Create app
          </Button>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable — showing mock data
          </div>
        ) : null}
        {isLoading ? <p className="muted" style={{ marginTop: 8 }}>Loading…</p> : <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{isFetching ? "fetching…" : error ? "mock" : "live"}</span>}
        {errorMsg ? <pre style={{ background: "var(--color-bad-soft)", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap", marginTop: 10, border: "1px solid var(--color-bad-line)", overflowWrap: "anywhere" }}>{errorMsg}</pre> : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Total apps</span>
          <span className="stat-value">{apps.length}</span>
          <span className="stat-meta">{active} active · {revoked} revoked</span>
        </div>
        <div className="stat">
          <span className="stat-label">Per-product keys</span>
          <span className="stat-meta" style={{ marginTop: 4 }}>One key per product — plug <code>pay_live_…</code> into Go/TS SDKs. Vault path <code>secret/orcta/orcta-pay/keys/{"{product}"}</code>.</span>
        </div>
        <div className="stat" style={{ background: "var(--color-accent-faint)", borderColor: "var(--color-accent-ring)" }}>
          <span className="stat-label">Quick start</span>
          <span className="muted" style={{ fontSize: 12, fontFamily: "var(--font-mono)", overflowWrap: "anywhere" }}>pnpm add @orctatech/orcta-pay · ORCTA_PAY_API_KEY=pay_live_… · new OrctaPay({"{"} apiKey {"}"})</span>
        </div>
        <div className="stat">
          <span className="stat-label">Type safety</span>
          <span className="stat-meta">TS client returns <code>{"{data, error}"}</code> Result — never throws. Check <code>error</code> with <code>OrctaPayError</code>.</span>
        </div>
      </div>

      {newKey ? (
        <div className="card" style={{ borderColor: "var(--color-warn-line)", background: "var(--color-warn-soft)" }}>
          <h2 style={{ margin: 0, color: "var(--color-warn-ink)" }}>API key — copy now, shown once</h2>
          <p className="muted">This key for <strong>{newKey.name}</strong> will not be shown again. Store it in Vault.</p>
          <div style={{ background: "var(--color-ink-strong)", color: "var(--color-ink-inverse)", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, overflowWrap: "anywhere" }}>
            <code style={{ wordBreak: "break-all", color: "var(--color-ink-inverse)" }}>{newKey.api_key}</code>
            <Button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(newKey.api_key)}>
              Copy
            </Button>
          </div>
          <p className="muted" style={{ marginTop: 6 }}>Prefix: <code>{newKey.prefix}</code></p>
          <div style={{ background: "var(--color-surface)", padding: 10, borderRadius: 8, fontSize: 12, marginTop: 10, border: "1px solid var(--color-line)" }}>
            <strong>Use it in your service:</strong>
            <pre style={{ margin: "6px 0 0", whiteSpace: "pre-wrap", wordBreak: "break-all", color: "var(--color-ink)", overflowWrap: "anywhere" }}>{`pnpm add @orctatech/orcta-pay
ORCTA_PAY_API_KEY=${newKey.api_key}
# Vault: secret/orcta/orcta-pay/keys/${newKey.name}
import { OrctaPay } from "@orctatech/orcta-pay";
const pay = new OrctaPay({ apiKey: process.env.ORCTA_PAY_API_KEY! });`}</pre>
            <Button className="btn ghost" style={{ marginTop: 8 }} onClick={() => setNewKey(null)}>
              Dismiss
            </Button>
          </div>
        </div>
      ) : null}

      {rotatedKey ? (
        <div className="card" style={{ borderColor: "var(--color-accent-ring)", background: "var(--color-accent-faint)" }}>
          <h2 style={{ margin: 0 }}>Rotated key — copy now, shown once</h2>
          <div style={{ background: "var(--color-ink-strong)", color: "var(--color-ink-inverse)", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, overflowWrap: "anywhere" }}>
            <code style={{ wordBreak: "break-all", color: "var(--color-ink-inverse)" }}>{rotatedKey.api_key}</code>
            <Button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(rotatedKey.api_key)}>
              Copy
            </Button>
          </div>
          <div style={{ background: "var(--color-surface)", padding: 10, borderRadius: 8, fontSize: 12, marginTop: 10, border: "1px solid var(--color-line)" }}>
            <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-all", overflowWrap: "anywhere" }}>{`ORCTA_PAY_API_KEY=${rotatedKey.api_key}`}</pre>
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
                <th>product</th>
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
                  <td className="mono">{a.name}</td>
                  <td>{a.product}</td>
                  <td className="mono">{a.prefix}</td>
                  <td className="muted">{formatDate(a.created_at)}</td>
                  <td className="muted">{a.last_used_at ? formatDate(a.last_used_at) : "—"}</td>
                  <td>{a.revoked ? <span className="pill failed">revoked</span> : <span className="pill succeeded">active</span>}</td>
                  <td>
                    <div className="row" style={{ gap: 6 }}>
                      <Button
                        className="btn ghost"
                        style={{ padding: "4px 8px", fontSize: 12 }}
                        disabled={a.revoked || rotateMut.isPending}
                        onClick={() => { setErrorMsg(null); void rotateMut.mutateAsync(a.id); }}
                      >
                        {rotateMut.isPending ? "Rotating…" : "Rotate"}
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
              {apps.length === 0 && <tr><td colSpan={7} className="muted" style={{ padding: 16 }}>No apps yet. Create one.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      <Dialog.Root open={createOpen} onOpenChange={(o: boolean) => setCreateOpen(o)}>
        <Dialog.Portal>
          <Dialog.Backdrop className="modal-backdrop" />
          <Dialog.Popup className="modal" style={{ position: "fixed", top: "50%", left: "50%", transform: "translate(-50%, -50%)", maxHeight: "90vh", overflow: "auto" }}>
            <Dialog.Title style={{ marginTop: 0, fontSize: 16, fontWeight: 700 }}>Create app</Dialog.Title>
            <p className="muted" style={{ marginTop: 4 }}>
              An app is a per-service API key. Name it after the service (<code>orctago</code>, <code>pos</code>). The key is shown once.
            </p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void form.handleSubmit();
              }}
            >
              <form.Field
                name="name"
                validators={{
                  onChange: ({ value }: { value: string }) => {
                    const r = createAppSchema.shape.name.safeParse(value);
                    return r.success ? undefined : r.error.issues[0]?.message;
                  },
                }}
              >
                {(field) => (
                  <Field.Root style={{ marginTop: 12 }}>
                    <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>Name</Field.Label>
                    <Input
                      className="input"
                      style={{ width: "100%", marginTop: 6 }}
                      placeholder="orctago"
                      value={field.state.value}
                      onChange={(e) => field.handleChange(e.target.value)}
                      onBlur={field.handleBlur}
                    />
                    {field.state.meta.isTouched && field.state.meta.errors.length ? (
                      <Field.Error style={{ color: "var(--color-bad-ink)", fontSize: 12, marginTop: 4 }}>{String(field.state.meta.errors[0])}</Field.Error>
                    ) : null}
                  </Field.Root>
                )}
              </form.Field>

              <form.Field
                name="product"
                validators={{
                  onChange: ({ value }: { value: string }) => {
                    const r = createAppSchema.shape.product.safeParse(value);
                    return r.success ? undefined : r.error.issues[0]?.message;
                  },
                }}
              >
                {(field) => (
                  <Field.Root style={{ marginTop: 12 }}>
                    <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>Product</Field.Label>
                    <Select.Root value={field.state.value} onValueChange={(v: unknown) => field.handleChange(v as string)}>
                      <Select.Trigger className="select" style={{ width: "100%", marginTop: 6 }}>
                        <Select.Value />
                        <Select.Icon>▾</Select.Icon>
                      </Select.Trigger>
                      <Select.Portal>
                        <Select.Positioner>
                          <Select.Popup>
                            <Select.List>
                              <Select.Item value="orctago">orctago</Select.Item>
                              <Select.Item value="pos">pos</Select.Item>
                            </Select.List>
                          </Select.Popup>
                        </Select.Positioner>
                      </Select.Portal>
                    </Select.Root>
                    {field.state.meta.isTouched && field.state.meta.errors.length ? (
                      <Field.Error style={{ color: "var(--color-bad-ink)", fontSize: 12, marginTop: 4 }}>{String(field.state.meta.errors[0])}</Field.Error>
                    ) : null}
                  </Field.Root>
                )}
              </form.Field>

              <div className="row" style={{ marginTop: 14, justifyContent: "flex-end" }}>
                <Button type="button" className="btn ghost" onClick={() => setCreateOpen(false)}>
                  Cancel
                </Button>
                <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting]}>
                  {([canSubmit, isSubmitting]) => (
                    <Button type="submit" className="btn" disabled={!canSubmit || Boolean(isSubmitting)}>
                      {isSubmitting ? "Creating…" : "Create"}
                    </Button>
                  )}
                </form.Subscribe>
              </div>
            </form>
          </Dialog.Popup>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
