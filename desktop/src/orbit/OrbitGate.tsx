// ORBIT: sign in to your Orbit account before anything else. This is not the
// Apple ID. Every Orbit command lives in src-tauri/src/orbit.
import { FormEvent, ReactNode, useCallback, useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import "./tokens.css";
import "./OrbitGate.css";
import icon from "./icon.svg";

type Methods = { github: boolean; google: boolean; email: boolean };

const SERVER_KEY = "orbit.server";

export function OrbitGate({ children }: { children: ReactNode }) {
  const [signedInTo, setSignedInTo] = useState<string | null | undefined>();
  const [server, setServer] = useState(localStorage.getItem(SERVER_KEY) ?? "");
  const [methods, setMethods] = useState<Methods | null>(null);
  const [email, setEmail] = useState("");
  const [waiting, setWaiting] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    const status = await invoke<{ server: string | null }>("orbit_status");
    setSignedInTo(status.server);
  }, []);

  useEffect(() => {
    refresh().catch(() => setSignedInTo(null));
  }, [refresh]);

  const run = async (action: () => Promise<void>) => {
    setError("");
    setBusy(true);
    try {
      await action();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
      setWaiting("");
    }
  };

  const connect = (event: FormEvent) => {
    event.preventDefault();
    run(async () => {
      setMethods(null);
      setMethods(await invoke<Methods>("orbit_methods", { server }));
      localStorage.setItem(SERVER_KEY, server.trim());
    });
  };

  const signIn = (provider: "github" | "google") =>
    run(async () => {
      setWaiting("Finish signing in in your browser…");
      await invoke("orbit_sign_in", { server, provider });
      await refresh();
    });

  const signInByEmail = (event: FormEvent) => {
    event.preventDefault();
    run(async () => {
      setWaiting("Check your inbox, and open the link on this computer.");
      await invoke("orbit_sign_in_email", { server, email });
      await refresh();
    });
  };

  const cancel = useCallback(() => {
    invoke("orbit_cancel_sign_in").catch(() => {});
  }, []);

  useEffect(() => {
    if (!waiting) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && cancel();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [waiting, cancel]);

  if (signedInTo === undefined) {
    return (
      <main className="orbit-gate">
        <p role="status">Starting Orbit Setup…</p>
      </main>
    );
  }

  if (signedInTo) {
    return (
      <>
        <div className="orbit-bar">
          <span>
            Orbit account on <code>{new URL(signedInTo).host}</code>
          </span>
          <button
            type="button"
            onClick={() =>
              run(async () => {
                await invoke("orbit_sign_out");
                setMethods(null);
                await refresh();
              })
            }
          >
            Sign out of Orbit
          </button>
        </div>
        {children}
      </>
    );
  }

  return (
    <main className="orbit-gate">
      <img src={icon} alt="" width={64} height={64} />
      <h1>Sign in to Orbit</h1>
      <p className="muted">
        This is your Orbit account, not your Apple ID. It lets this computer use your server to
        sign in to Apple.
      </p>

      <form onSubmit={connect}>
        <label htmlFor="server">Server address</label>
        <input
          id="server"
          autoComplete="url"
          placeholder="orbit.example.com"
          value={server}
          disabled={busy}
          onChange={(e) => {
            setServer(e.target.value);
            setMethods(null);
          }}
        />
        <button type="submit" disabled={busy || !server.trim()}>
          Continue
        </button>
      </form>

      {methods && !waiting && (
        <div className="choices">
          {!methods.github && !methods.google && !methods.email && (
            <p>This server has no way to sign in set up. Ask whoever runs it.</p>
          )}
          {methods.github && (
            <button type="button" disabled={busy} onClick={() => signIn("github")}>
              Continue with GitHub
            </button>
          )}
          {methods.google && (
            <button type="button" disabled={busy} onClick={() => signIn("google")}>
              Continue with Google
            </button>
          )}
          {methods.email && (
            <form onSubmit={signInByEmail}>
              <label htmlFor="email">Email</label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                disabled={busy}
                onChange={(e) => setEmail(e.target.value)}
              />
              <button type="submit" className="primary" disabled={busy}>
                Email me a sign-in link
              </button>
            </form>
          )}
        </div>
      )}

      {waiting && (
        <div className="choices" role="status">
          <p>{waiting}</p>
          <button type="button" onClick={cancel}>
            Cancel
          </button>
        </div>
      )}

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </main>
  );
}
