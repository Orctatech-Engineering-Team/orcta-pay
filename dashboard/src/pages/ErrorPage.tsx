import { useNavigate } from "@tanstack/react-router";
import { Button } from "@base-ui/react/button";

export function ErrorPage({ error }: { error?: Error | null }) {
  const navigate = useNavigate();

  return (
    <div style={{
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      minHeight: "60vh",
      textAlign: "center",
      padding: "var(--space-8)",
    }}>
      <div style={{
        width: 80,
        height: 80,
        borderRadius: "var(--radius-lg)",
        background: "var(--color-bad-soft)",
        border: "2px solid var(--color-bad-line)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        marginBottom: "var(--space-6)",
      }}>
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--color-bad)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="10" />
          <path d="m15 9-6 6" />
          <path d="m9 9 6 6" />
        </svg>
      </div>
      
      <h1 style={{
        fontSize: "var(--text-2xl)",
        fontWeight: 700,
        color: "var(--color-ink-strong)",
        margin: "0 0 var(--space-2)",
      }}>
        Something went wrong
      </h1>
      
      <p style={{
        fontSize: "var(--text-lg)",
        color: "var(--color-ink-muted)",
        margin: "0 0 var(--space-6)",
        maxWidth: 400,
      }}>
        {error?.message || "An unexpected error occurred. Please try again."}
      </p>
      
      <div style={{ display: "flex", gap: "var(--space-3)" }}>
        <Button
          className="btn"
          onClick={() => void navigate({ to: "/" })}
          style={{ minWidth: 120 }}
        >
          Go home
        </Button>
        <Button
          className="btn ghost"
          onClick={() => window.location.reload()}
          style={{ minWidth: 120 }}
        >
          Retry
        </Button>
      </div>
    </div>
  );
}
