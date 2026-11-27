//! ORBIT: everything Orbit adds to iloader. Keep it here so upstream updates
//! (`git subtree pull`) stay easy; edits elsewhere are marked "ORBIT:".

pub mod login;
pub mod proxy;
pub mod session;
#[cfg(test)]
pub mod testing;

use std::sync::{Arc, Mutex};

use base64::{Engine, engine::general_purpose::URL_SAFE_NO_PAD};
use serde::Serialize;
use tauri::{AppHandle, Manager, State};
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
    orbit.account.access_token().await?;
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
}

#[tauri::command]
pub async fn orbit_status(orbit: State<'_, Orbit>) -> Result<Status, String> {
    Ok(Status {
        server: orbit.account.server().await.map(String::from),
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
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| format!("Couldn't open your browser: {e}"))?;
    finish(&orbit, &server, listener, &verifier).await
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
}

async fn finish(
    orbit: &Orbit,
    server: &url::Url,
    listener: tokio::net::TcpListener,
    verifier: &str,
) -> Result<(), String> {
    let (cancel, cancelled) = oneshot::channel();
    // Replacing an older sender cancels the older wait.
    *orbit.cancel.lock().unwrap() = Some(cancel);
    let code = tokio::select! {
        code = login::wait_for_code(listener) => code?,
        _ = cancelled => return Err("Sign-in cancelled.".into()),
    };
    orbit.account.exchange(server, &code, verifier).await
}

#[tauri::command]
pub fn orbit_cancel_sign_in(orbit: State<'_, Orbit>) {
    if let Some(cancel) = orbit.cancel.lock().unwrap().take() {
        let _ = cancel.send(());
    }
}

#[tauri::command]
pub async fn orbit_sign_out(orbit: State<'_, Orbit>) -> Result<(), String> {
    orbit.account.sign_out().await;
    Ok(())
}
