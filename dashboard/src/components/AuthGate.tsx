import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@base-ui/react/button";
import { Input } from "@base-ui/react/input";
import { hasSession, login } from "../lib/api";

export function AuthGate({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const session = useQuery({ queryKey: ["operator-session"], queryFn: hasSession, retry: false, staleTime: 60_000 });

  if (session.isPending) return <div className="auth-screen"><div className="auth-card muted">Checking session…</div></div>;
  if (session.data) return <>{children}</>;

  return (
    <main className="auth-screen">
      <form className="auth-card" onSubmit={async (event) => {
        event.preventDefault();
        setSubmitting(true);
        setError("");
        try {
          await login(key);
          setKey("");
          await queryClient.invalidateQueries({ queryKey: ["operator-session"] });
        } catch (cause) {
          setError(cause instanceof Error ? cause.message : "Unable to sign in");
        } finally {
          setSubmitting(false);
        }
      }}>
        <div className="auth-brand">
          <img src="/favicon.png" alt="" width={40} height={40} />
          <div><h1>Orcta Pay</h1><span>Operator dashboard</span></div>
        </div>
        <div>
          <label className="auth-label" htmlFor="access-key">Operator access key</label>
          <Input id="access-key" className="input auth-input" type="password" autoComplete="current-password" autoFocus required value={key} onChange={(event) => setKey(event.target.value)} />
          <p className="muted auth-help">Use the server-side <code>ORCTA_PAY_API_KEY</code>. It is exchanged for an HttpOnly session and is not stored in this browser.</p>
        </div>
        {error ? <div className="auth-error" role="alert">{error}</div> : null}
        <Button className="btn auth-submit" type="submit" disabled={submitting}>{submitting ? "Signing in…" : "Sign in"}</Button>
      </form>
    </main>
  );
}
