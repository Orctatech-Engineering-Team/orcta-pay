import { useNavigate } from "@tanstack/react-router";
import { Button } from "@base-ui/react/button";

export function NotFoundPage() {
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
        background: "var(--color-accent-faint)",
        border: "2px solid var(--color-accent-ring)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        marginBottom: "var(--space-6)",
      }}>
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.3-4.3" />
          <path d="m11 8v6" />
          <path d="m8 11h6" />
        </svg>
      </div>
      
      <h1 style={{
        fontSize: "var(--text-2xl)",
        fontWeight: 700,
        color: "var(--color-ink-strong)",
        margin: "0 0 var(--space-2)",
      }}>
        404
      </h1>
      
      <p style={{
        fontSize: "var(--text-lg)",
        color: "var(--color-ink-muted)",
        margin: "0 0 var(--space-6)",
        maxWidth: 400,
      }}>
        This page doesn't exist or you don't have access to it.
      </p>
      
      <Button
        className="btn"
        onClick={() => void navigate({ to: "/" })}
        style={{ minWidth: 120 }}
      >
        Go home
      </Button>
    </div>
  );
}
