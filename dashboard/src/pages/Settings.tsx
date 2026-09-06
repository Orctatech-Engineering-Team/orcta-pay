import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Field } from "@base-ui/react/field";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { getApiKey, getBaseUrl, maskKey, setApiKey, setBaseUrl } from "../lib/config";
import { settingsSchema } from "../lib/validators";
import { getTheme, toggleTheme, type Theme } from "../lib/theme";

type SettingsTab = "general" | "api-keys" | "appearance";

const TABS: { id: SettingsTab; label: string }[] = [
  { id: "general", label: "General" },
  { id: "api-keys", label: "API Keys" },
  { id: "appearance", label: "Appearance" },
];

export function SettingsPage() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");
  const [theme, setThemeState] = useState<Theme>(() => getTheme());

  const [apiKeyValue, setApiKeyValue] = useState(() => getApiKey());
  const [baseUrlValue, setBaseUrlValue] = useState(() => getBaseUrl());

  const [saved, setSaved] = useState(false);

  const handleSave = () => {
    const parsed = settingsSchema.safeParse({ apiKey: apiKeyValue.trim(), baseUrl: baseUrlValue.trim() });
    if (!parsed.success) return;
    setApiKey(apiKeyValue.trim());
    setBaseUrl(baseUrlValue.trim());
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  return (
    <div style={{ display: "flex", gap: 0, minHeight: "calc(100vh - 60px)" }}>
      <aside style={{
        width: 220,
        flexShrink: 0,
        borderRight: "1px solid var(--color-line-faint)",
        padding: "var(--space-4) 0",
        background: "var(--color-surface-soft)",
      }}>
        <div style={{ padding: "0 var(--space-4) var(--space-3)", borderBottom: "1px solid var(--color-line-faint)" }}>
          <h2 style={{ fontSize: "var(--text-md)", fontWeight: 600, margin: 0, color: "var(--color-ink-strong)" }}>Settings</h2>
        </div>
        <nav style={{ padding: "var(--space-2) var(--space-2)" }}>
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              style={{
                display: "block",
                width: "100%",
                textAlign: "left",
                padding: "8px var(--space-3)",
                borderRadius: "var(--radius-sm)",
                fontSize: "var(--text-sm)",
                fontWeight: 500,
                color: activeTab === tab.id ? "var(--color-accent-ink)" : "var(--color-ink-muted)",
                background: activeTab === tab.id ? "var(--color-accent-soft)" : "transparent",
                border: "none",
                cursor: "pointer",
                transition: "background 120ms, color 120ms",
              }}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </aside>

      <main style={{ flex: 1, minWidth: 0, padding: "var(--space-6) var(--space-8)" }}>
        {activeTab === "general" && (
          <div style={{ maxWidth: 560 }}>
            <h3 style={{ fontSize: "var(--text-lg)", fontWeight: 600, margin: "0 0 var(--space-1)", color: "var(--color-ink-strong)" }}>General</h3>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)", margin: "0 0 var(--space-5)" }}>
              Configure your dashboard connection.
            </p>

            <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-4)" }}>
              <Field.Root>
                <Field.Label style={{ display: "block", fontSize: "var(--text-sm)", fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>Base URL</Field.Label>
                <Input
                  className="input"
                  style={{ width: "100%" }}
                  placeholder="http://localhost:8080"
                  value={baseUrlValue}
                  onChange={(e) => setBaseUrlValue(e.target.value)}
                />
                <span style={{ fontSize: "var(--text-xs)", color: "var(--color-ink-faint)", marginTop: 4, display: "block" }}>
                  The Orcta Pay API endpoint.
                </span>
              </Field.Root>
            </div>

            <div style={{ marginTop: "var(--space-6)", display: "flex", gap: "var(--space-3)" }}>
              <Button className="btn" onClick={handleSave}>
                {saved ? "Saved" : "Save changes"}
              </Button>
              <Button className="btn ghost" onClick={() => void navigate({ to: "/" })}>
                Cancel
              </Button>
            </div>
          </div>
        )}

        {activeTab === "api-keys" && (
          <div style={{ maxWidth: 560 }}>
            <h3 style={{ fontSize: "var(--text-lg)", fontWeight: 600, margin: "0 0 var(--space-1)", color: "var(--color-ink-strong)" }}>API Keys</h3>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)", margin: "0 0 var(--space-5)" }}>
              Manage your API key. Store keys in Vault, not in code.
            </p>

            <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-4)" }}>
              <Field.Root>
                <Field.Label style={{ display: "block", fontSize: "var(--text-sm)", fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>Current key</Field.Label>
                <div style={{
                  padding: "10px var(--space-3)",
                  background: "var(--color-surface-muted)",
                  border: "1px solid var(--color-line-faint)",
                  borderRadius: "var(--radius-sm)",
                  fontFamily: "var(--font-mono)",
                  fontSize: "var(--text-sm)",
                  color: "var(--color-ink-muted)",
                  overflowWrap: "anywhere",
                }}>
                  {apiKeyValue ? maskKey(apiKeyValue) : "No key set"}
                </div>
              </Field.Root>

              <Field.Root>
                <Field.Label style={{ display: "block", fontSize: "var(--text-sm)", fontWeight: 600, marginBottom: 6, color: "var(--color-ink)" }}>API key</Field.Label>
                <Input
                  className="input"
                  style={{ width: "100%" }}
                  placeholder="pay_live_..."
                  value={apiKeyValue}
                  onChange={(e) => setApiKeyValue(e.target.value)}
                />
                <span style={{ fontSize: "var(--text-xs)", color: "var(--color-ink-faint)", marginTop: 4, display: "block" }}>
                  Your Orcta Pay API key.
                </span>
              </Field.Root>
            </div>

            <div style={{ marginTop: "var(--space-6)", display: "flex", gap: "var(--space-3)" }}>
              <Button className="btn" onClick={handleSave}>
                {saved ? "Saved" : "Save changes"}
              </Button>
              <Button className="btn ghost" onClick={() => void navigate({ to: "/" })}>
                Cancel
              </Button>
            </div>
          </div>
        )}

        {activeTab === "appearance" && (
          <div style={{ maxWidth: 560 }}>
            <h3 style={{ fontSize: "var(--text-lg)", fontWeight: 600, margin: "0 0 var(--space-1)", color: "var(--color-ink-strong)" }}>Appearance</h3>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--color-ink-muted)", margin: "0 0 var(--space-5)" }}>
              Choose your theme.
            </p>

            <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-4)" }}>
              <div style={{ display: "flex", gap: "var(--space-3)" }}>
                <button
                  onClick={() => { setThemeState("dark" as Theme); document.documentElement.setAttribute("data-theme", "dark"); localStorage.setItem("orcta_pay_theme", "dark"); }}
                  style={{
                    padding: "var(--space-4)",
                    borderRadius: "var(--radius-md)",
                    border: `2px solid ${theme === "dark" ? "var(--color-accent)" : "var(--color-line)"}`,
                    background: theme === "dark" ? "var(--color-accent-soft)" : "var(--color-surface)",
                    cursor: "pointer",
                    flex: 1,
                    textAlign: "center",
                  }}
                >
                  <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--color-ink-strong)" }}>Dark</div>
                  <div style={{ fontSize: "var(--text-xs)", color: "var(--color-ink-muted)", marginTop: 4 }}>Default</div>
                </button>
                <button
                  onClick={() => { setThemeState("light" as Theme); document.documentElement.setAttribute("data-theme", "light"); localStorage.setItem("orcta_pay_theme", "light"); }}
                  style={{
                    padding: "var(--space-4)",
                    borderRadius: "var(--radius-md)",
                    border: `2px solid ${theme === "light" ? "var(--color-accent)" : "var(--color-line)"}`,
                    background: theme === "light" ? "var(--color-accent-soft)" : "var(--color-surface)",
                    cursor: "pointer",
                    flex: 1,
                    textAlign: "center",
                  }}
                >
                  <div style={{ fontSize: "var(--text-sm)", fontWeight: 600, color: "var(--color-ink-strong)" }}>Light</div>
                  <div style={{ fontSize: "var(--text-xs)", color: "var(--color-ink-muted)", marginTop: 4 }}>Bright</div>
                </button>
              </div>
            </div>

            <div style={{ marginTop: "var(--space-6)" }}>
              <Button className="btn ghost" onClick={() => void navigate({ to: "/" })}>
                Back to dashboard
              </Button>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
