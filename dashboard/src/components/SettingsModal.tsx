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
          <Dialog.Title className="modal-title">Settings</Dialog.Title>

          {currentKey ? (
            <div style={{ marginBottom: 16, padding: "8px 12px", background: "var(--color-surface-muted)", border: "1px solid var(--color-line-faint)", borderRadius: 8, fontSize: 13, color: "var(--color-ink-muted)" }}>
              Current key: <code>{maskKey(currentKey)}</code>
            </div>
          ) : null}

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
                <Field.Root style={{ marginBottom: 14 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>API key</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%" }}
                    placeholder="pay_live_..."
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

            <form.Field name="product">
              {(field) => (
                <Field.Root style={{ marginBottom: 14 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>Product</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%" }}
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
                <Field.Root style={{ marginBottom: 14 }}>
                  <Field.Label style={{ display: "block", fontSize: 13, fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>Base URL</Field.Label>
                  <Input
                    className="input"
                    style={{ width: "100%" }}
                    placeholder="http://localhost:8080"
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

            <div className="row" style={{ justifyContent: "flex-end", gap: 8 }}>
              <Button type="button" className="btn ghost" onClick={onClose}>
                Cancel
              </Button>
              <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting]}>
                {([canSubmit, isSubmitting]) => (
                  <Button type="submit" className="btn" disabled={!canSubmit || Boolean(isSubmitting)}>
                    {isSubmitting ? "Saving..." : "Save"}
                  </Button>
                )}
              </form.Subscribe>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
