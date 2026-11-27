// ORBIT: sign in to your Orbit account before anything else. This is not the
// Apple ID. Every Orbit command lives in src-tauri/src/orbit.
import { FormEvent, ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getVersion } from "@tauri-apps/api/app";
import { ask } from "@tauri-apps/plugin-dialog";
import "./tokens.css";
import "./OrbitGate.css";
import icon from "./icon.svg";

type Methods = { github: boolean; google: boolean; email: boolean };
type Status = { server: string | null; email: string | null };

const SERVER_KEY = "orbit.server";
const CANCELLED = "Sign-in cancelled.";

export function OrbitGate({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status | undefined>();
  const [server, setServer] = useState(localStorage.getItem(SERVER_KEY) ?? "");
  const [methods, setMethods] = useState<Methods | null>(null);
  const [email, setEmail] = useState("");
  /** What is being waited for; empty when idle. */
  const [progress, setProgress] = useState("");
  const [cancellable, setCancellable] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState(false);
  const retry = useRef<() => void>(() => {});
  const choices = useRef<HTMLDivElement>(null);
  const busy = progress !== "";

  const refresh = useCallback(async () => {
    setStatus(await invoke<Status>("orbit_status"));
  }, []);

  useEffect(() => {
    refresh().catch(() => setStatus({ server: null, email: null }));
  }, [refresh]);

  // The server can end the session while the window is open.
  useEffect(() => {
    const unlisten = listen("orbit-signed-out", () => {
      setNotice("Your Orbit session ended. Sign in again.");
      setMethods(null);
      refresh().catch(() => {});
    });
    return () => {
      unlisten.then((stop) => stop()).catch(() => {});
    };
  }, [refresh]);

  // After connecting, once the choices are enabled (a disabled button can't
  // take focus).
  const connected = useRef(false);
  useEffect(() => {
    if (!methods) connected.current = false;
    else if (!busy && !connected.current) {
      connected.current = true;
      choices.current?.querySelector<HTMLElement>("button, input")?.focus();
    }
  }, [methods, busy]);

  const run = async (label: string, canCancel: boolean, action: () => Promise<void>) => {
    retry.current = () => run(label, canCancel, action);
    setError("");
    setNotice("");
    setCopied(false);
    setProgress(label);
    setCancellable(canCancel);
    try {
      await action();
    } catch (e) {
      // Cancelling is what the user asked for, not a failure.
      if (String(e) !== CANCELLED) setError(String(e));
    } finally {
      setProgress("");
      setCancellable(false);
    }
  };

  const connect = (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    run("Connecting…", false, async () => {
      setMethods(null);
      setMethods(await invoke<Methods>("orbit_methods", { server }));
      localStorage.setItem(SERVER_KEY, server.trim());
    });
  };

  const signIn = (provider: "github" | "google") =>
    run("Finish signing in in your browser…", true, async () => {
      await invoke("orbit_sign_in", { server, provider });
      await refresh();
    });

  const signInByEmail = (event: FormEvent) => {
    event.preventDefault();
    run(`Sending a link to ${email}. Open it on this computer.`, true, async () => {
      await invoke("orbit_sign_in_email", { server, email });
      await refresh();
    });
  };

  const cancel = useCallback(() => {
    invoke("orbit_cancel_sign_in").catch(() => {});
  }, []);

  useEffect(() => {
    if (!cancellable) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && cancel();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [cancellable, cancel]);

  // No tokens, codes or passwords: the error text and where it happened.
  const copyDiagnostics = async () => {
    const version = await getVersion().catch(() => "unknown");
    await navigator.clipboard.writeText(
      `Orbit Setup ${version}\nSystem: ${navigator.userAgent}\nServer: ${server}\nError: ${error}`,
    );
    setCopied(true);
  };

  const problem = error && (
    <div className="problem">
      <p className="error" role="alert">
        {error}
      </p>
      <div className="row">
        <button type="button" onClick={() => retry.current()}>
          Try again
        </button>
        <button type="button" onClick={copyDiagnostics}>
          {copied ? "Copied" : "Copy diagnostics"}
        </button>
      </div>
    </div>
  );

  if (status === undefined) {
    return (
      <main className="orbit-gate">
        <p role="status">Starting Orbit Setup…</p>
      </main>
    );
  }

  if (status.server) {
    const host = new URL(status.server).host;
    return (
      <div className="orbit-frame">
        <div className="orbit-bar">
          <span>
            {status.email ? `${status.email} on ` : "Orbit account on "}
            <code>{host}</code>
          </span>
          <button
            type="button"
            disabled={busy}
            onClick={async () => {
              const yes = await ask(
                "You'll be signed out of Orbit and of Apple ID here. Anything in progress is abandoned.",
                { title: `Sign out of Orbit on ${host}?`, kind: "warning" },
              );
              if (!yes) return;
              run("Signing out…", false, async () => {
                await invoke("orbit_sign_out");
                setMethods(null);
                await refresh();
              });
            }}
          >
            Sign out of Orbit
          </button>
          {problem}
        </div>
        {children}
      </div>
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
      {notice && <p role="alert">{notice}</p>}

      <form onSubmit={connect}>
        <label htmlFor="server">Server address</label>
        <span className="muted hint" id="server-hint">
          For example, orbit.example.com
        </span>
        <input
          id="server"
          autoFocus
          autoComplete="url"
          autoCapitalize="none"
          spellCheck={false}
          aria-describedby="server-hint"
          value={server}
          readOnly={busy}
          onChange={(e) => {
            setServer(e.target.value);
            setMethods(null);
            setError("");
          }}
        />
        {!methods && (
          <button type="submit" className="primary" disabled={busy || !server.trim()}>
            Continue
          </button>
        )}
      </form>

      {methods && (
        <div className="choices" ref={choices}>
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
                readOnly={busy}
                onChange={(e) => setEmail(e.target.value)}
              />
              <button type="submit" className="primary" disabled={busy}>
                Email me a sign-in link
              </button>
            </form>
          )}
        </div>
      )}

      {/* Always present, so screen readers hear each change. */}
      <div className="choices" role="status">
        {busy && (
          <p>
            <span className="spinner" aria-hidden="true" /> {progress}
          </p>
        )}
      </div>
      {cancellable && (
        <button type="button" onClick={cancel}>
          Cancel
        </button>
      )}
      {problem}
    </main>
  );
}
