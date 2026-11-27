//! ORBIT: everything Orbit adds to iloader. Keep it here so upstream updates
//! (`git subtree pull`) stay easy; edits elsewhere are marked "ORBIT:".

pub mod login;
pub mod privacy;
pub mod proxy;
pub mod session;
#[cfg(test)]
pub mod testing;

use std::{
    future::Future,
    sync::{Arc, Mutex},
};

use base64::{Engine, engine::general_purpose::URL_SAFE_NO_PAD};
use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager, State};
use tauri_plugin_opener::OpenerExt;
use tokio::sync::{OnceCell, oneshot};

use session::{Account, KeyringStore, Methods, parse_server};

pub struct Orbit {
    account: Arc<Account>,
    anisette: OnceCell<String>,
    cancel: Mutex<Option<oneshot::Sender<()>>>,
}

impl Orbit {
    pub fn new() -> Self {
        Self {
            account: Arc::new(Account::new(Box::new(KeyringStore))),
            anisette: OnceCell::new(),
            cancel: Mutex::new(None),
        }
    }
}

/// 32 random bytes, base64url.
pub fn random_token() -> String {
    let mut bytes = [0u8; 32];
    getrandom::fill(&mut bytes).expect("no randomness from the system");
    URL_SAFE_NO_PAD.encode(bytes)
}

/// The anisette address for Apple sign-in: always the user's own Orbit server,
/// never a public anisette server.
pub async fn anisette_url(app: &AppHandle) -> Result<String, String> {
    let orbit = app.state::<Orbit>();
    // Fails early, with a clear message, when not signed in.
    if let Err(message) = orbit.account.access_token().await {
        // The server ended the session: the window has to show sign-in again.
        if orbit.account.server().await.is_none() {
            let _ = app.emit("orbit-signed-out", ());
        }
        return Err(message);
    }
    orbit
        .anisette
        .get_or_try_init(|| proxy::start(orbit.account.clone()))
        .await
        .cloned()
        .map_err(|e| format!("Couldn't start the anisette helper: {e}"))
}

#[derive(Serialize)]
pub struct Status {
    server: Option<String>,
    /// Which account, so a sign-in to the wrong one is visible.
    email: Option<String>,
}

#[tauri::command]
pub async fn orbit_status(orbit: State<'_, Orbit>) -> Result<Status, String> {
    let email = orbit.account.email().await;
    Ok(Status {
        server: orbit.account.server().await.map(String::from),
        email,
    })
}

#[tauri::command]
pub async fn orbit_methods(orbit: State<'_, Orbit>, server: String) -> Result<Methods, String> {
    orbit.account.methods(&parse_server(&server)?).await
}

/// Signs in through the browser with GitHub or Google.
#[tauri::command]
pub async fn orbit_sign_in(
    app: AppHandle,
    orbit: State<'_, Orbit>,
    server: String,
    provider: String,
) -> Result<(), String> {
    let server = parse_server(&server)?;
    let (listener, redirect) = login::listen().await.map_err(|e| e.to_string())?;
    let verifier = random_token();
    let url = login::start_url(&server, &provider, &login::challenge(&verifier), &redirect)?;
    let attempt = async {
        app.opener()
            .open_url(url, None::<&str>)
            .map_err(|e| format!("Couldn't open your browser: {e}"))?;
        finish(&orbit, &server, listener, &verifier).await
    };
    cancellable(&orbit, attempt).await
}

/// Sends a sign-in link, then waits for it to be opened on this computer.
#[tauri::command]
pub async fn orbit_sign_in_email(
    orbit: State<'_, Orbit>,
    server: String,
    email: String,
) -> Result<(), String> {
    let server = parse_server(&server)?;
    let (listener, redirect) = login::listen().await.map_err(|e| e.to_string())?;
    let verifier = random_token();
    let attempt = async {
        orbit
            .account
            .email_start(
                &server,
                email.trim(),
                &login::challenge(&verifier),
                &redirect,
            )
            .await?;
        finish(&orbit, &server, listener, &verifier).await
    };
    cancellable(&orbit, attempt).await
}

/// Runs a sign-in attempt that "Cancel" can stop at any point.
async fn cancellable(
    orbit: &Orbit,
    attempt: impl Future<Output = Result<(), String>>,
) -> Result<(), String> {
    let (cancel, cancelled) = oneshot::channel();
    // Replacing an older sender cancels the older attempt.
    *orbit.cancel.lock().unwrap() = Some(cancel);
    tokio::select! {
        result = attempt => result,
        _ = cancelled => Err(CANCELLED.into()),
    }
}

const CANCELLED: &str = "Sign-in cancelled.";

/// Waits for a code the server accepts. Any program here can reach the callback,
/// so a refused code doesn't end the attempt.
async fn finish(
    orbit: &Orbit,
    server: &url::Url,
    listener: tokio::net::TcpListener,
    verifier: &str,
) -> Result<(), String> {
    let wait = async {
        loop {
            let code = login::wait_for_code(&listener).await?;
            match orbit.account.exchange(server, &code, verifier).await {
                Ok(()) => return Ok(()),
                Err(message) => tracing::warn!("a sign-in code was refused: {message}"),
            }
        }
    };
    tokio::time::timeout(login::WAIT, wait)
        .await
        .map_err(|_| "Sign-in timed out. Try again.".to_string())?
}

#[tauri::command]
pub fn orbit_cancel_sign_in(orbit: State<'_, Orbit>) {
    if let Some(cancel) = orbit.cancel.lock().unwrap().take() {
        let _ = cancel.send(());
    }
}

#[tauri::command]
pub async fn orbit_sign_out(
    orbit: State<'_, Orbit>,
    apple: State<'_, crate::sideload::SideloaderMutex>,
) -> Result<(), String> {
    // The next person at this computer must not inherit the Apple sign-in.
    *apple.lock().unwrap() = None;
    orbit.account.sign_out().await;
    Ok(())
}
