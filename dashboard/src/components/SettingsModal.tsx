import { Dialog } from "@base-ui/react/dialog";
import { Field } from "@base-ui/react/field";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { useForm } from "@tanstack/react-form";
import { getApiKey, getBaseUrl, getProduct, maskKey, setApiKey, setBaseUrl, setProduct } from "../lib/config";
import { settingsSchema } from "../lib/validators";

export function SettingsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const currentKey = getApiKey();

  const form = useForm({
    defaultValues: {
      apiKey: getApiKey(),
      baseUrl: getBaseUrl(),
      product: getProduct(),
    },
    onSubmit: async ({ value }) => {
      // Zod validation before persist — keep Result-style handling (no throw)
      const parsed = settingsSchema.safeParse({ apiKey: value.apiKey.trim(), baseUrl: value.baseUrl.trim() });
      if (!parsed.success) return;
      setApiKey(value.apiKey.trim());
      setBaseUrl(value.baseUrl.trim());
      setProduct(value.product.trim().toLowerCase());
      onClose();
      window.location.reload();
    },
  });

  return (
    <Dialog.Root open={open} onOpenChange={(o: boolean) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="modal-backdrop" />
        <Dialog.Popup className="modal" style={{ position: "fixed", top: "50%", left: "50%", transform: "translate(-50%, -50%)", maxHeight: "90vh", overflow: "auto" }}>
          <Dialog.Title style={{ marginTop: 0, fontSize: 16, fontWeight: 700 }}>Settings</Dialog.Title>
          <p className="muted" style={{ marginTop: 4 }}>
            Paste any product&apos;s API key. Keys live in Vault at <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> and are
            rendered to <code>.env</code> as <code>VITE_ORCTA_PAY_API_KEY</code>. This modal overrides via <code>localStorage</code>{" "}
            for demo — same shape any Orcta service uses with the TS client.
          </p>

          <div style={{ marginTop: 10, background: "#f8fafc", border: "1px solid #e2e8f0", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Current key: <code>{maskKey(currentKey)}</code> {currentKey ? `· ${currentKey.slice(0, 12)}…` : ""} — from{" "}
            <code>VITE_ORCTA_PAY_API_KEY</code> or localStorage. After creating an app, paste its <code>pay_live_…</code> here.
          </div>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void form.handleSubmit();
            }}
          >
            <form.Field
              name="apiKey"
              validators={{
                onChange: ({ value }: { value: string }) => {
                  const r = settingsSchema.shape.apiKey.safeParse(value);
                  return r.success ? undefined : r.error.issues[0]?.message;
                },
              }}
            >
              {(field) => (
                <Field.Root style={{ marginTop: 12 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>API key (Bearer)</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%", marginTop: 6 }}
                    placeholder="pay_live_… or pay_test_…"
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    onBlur={field.handleBlur}
                  />
                  {field.state.meta.isTouched && field.state.meta.errors.length ? (
                    <Field.Error style={{ color: "#dc2626", fontSize: 12, marginTop: 4 }}>{String(field.state.meta.errors[0])}</Field.Error>
                  ) : null}
                </Field.Root>
              )}
            </form.Field>

            <form.Field name="product">
              {(field) => (
                <Field.Root style={{ marginTop: 12 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>Product</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%", marginTop: 6 }}
                    placeholder="orctago"
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </Field.Root>
              )}
            </form.Field>

            <form.Field
              name="baseUrl"
              validators={{
                onChange: ({ value }: { value: string }) => {
                  const r = settingsSchema.shape.baseUrl.safeParse(value);
                  return r.success ? undefined : r.error.issues[0]?.message;
                },
              }}
            >
              {(field) => (
                <Field.Root style={{ marginTop: 12 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600 }}>Base URL</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%", marginTop: 6 }}
                    placeholder="http://localhost:8080"
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    onBlur={field.handleBlur}
                  />
                  {field.state.meta.isTouched && field.state.meta.errors.length ? (
                    <Field.Error style={{ color: "#dc2626", fontSize: 12, marginTop: 4 }}>{String(field.state.meta.errors[0])}</Field.Error>
                  ) : null}
                </Field.Root>
              )}
            </form.Field>

            <p className="muted" style={{ marginTop: 10 }}>
              Env fallback: <code>VITE_ORCTA_PAY_URL</code> and <code>VITE_ORCTA_PAY_API_KEY</code> in <code>dashboard/.env</code>.
              LocalStorage keys: <code>orcta_pay_api_key</code>, <code>orcta_pay_url</code>, <code>orcta_pay_product</code>.
            </p>

            <div className="row" style={{ marginTop: 14, justifyContent: "flex-end" }}>
              <Button type="button" className="btn ghost" onClick={onClose}>
                Cancel
              </Button>
              <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting]}>
                {([canSubmit, isSubmitting]) => (
                  <Button type="submit" className="btn" disabled={!canSubmit || Boolean(isSubmitting)}>
                    {isSubmitting ? "Saving…" : "Save & reload"}
                  </Button>
                )}
              </form.Subscribe>
            </div>
          </form>

          <div style={{ marginTop: 12, padding: 10, background: "#f1f5f9", borderRadius: 8, fontSize: 12, color: "#334155" }}>
            <strong>Any Orcta service can use the TS client the same way:</strong>
            <pre style={{ margin: "6px 0 0", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>{`import { OrctaPay } from "@orctatech/orcta-pay";
const pay = new OrctaPay({ apiKey: process.env.VITE_ORCTA_PAY_API_KEY! });
// or: new OrctaPay({ apiKey: getApiKey() }) from localStorage`}</pre>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
