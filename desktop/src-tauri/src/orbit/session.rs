//! ORBIT: the signed-in Orbit account (not the Apple ID). Holds the refresh
//! token in the system keyring and hands out access tokens.

use std::time::{Duration, Instant};

use serde::Deserialize;
use tokio::sync::Mutex;
use url::Url;

/// Renew a little before expiry, so a token never lapses mid-request.
const EARLY: Duration = Duration::from_secs(60);

/// Accepts "orbit.example.com" or a full URL. HTTPS only, because tokens travel
/// with every request; plain HTTP is allowed for a server on this computer.
pub fn parse_server(input: &str) -> Result<Url, String> {
    const HELP: &str = "Use your server's HTTPS address, like orbit.example.com.";
    let text = input.trim();
    let text = if text.contains("://") {
        text.to_string()
    } else {
        format!("https://{text}")
    };
    let url = Url::parse(&text).map_err(|_| HELP.to_string())?;
    let local = matches!(url.host_str(), Some("localhost" | "127.0.0.1"));
    let plain = url.username().is_empty()
        && url.password().is_none()
        && url.query().is_none()
        && url.fragment().is_none()
        && url.path() == "/";
    let secure = url.scheme() == "https" || (url.scheme() == "http" && local);
    let named = local || url.host_str().is_some_and(|h| h.contains('.'));
    if plain && secure && named {
        Ok(url)
    } else {
        Err(HELP.to_string())
    }
}

/// Where the refresh token is kept between runs.
pub trait RefreshStore: Send + Sync {
    fn load(&self) -> Option<(String, String)>;
    fn save(&self, server: &str, refresh: &str);
    fn clear(&self);
}

pub struct KeyringStore;

impl KeyringStore {
    fn entry() -> Option<keyring::Entry> {
        keyring::Entry::new("orbit-setup", "session").ok()
    }
}

impl RefreshStore for KeyringStore {
    // Without a working keyring the session simply lasts until the app closes:
    // a token is never written to a plain file.
    fn load(&self) -> Option<(String, String)> {
        let stored = Self::entry()?.get_password().ok()?;
        let (server, refresh) = stored.split_once('\n')?;
        Some((server.to_string(), refresh.to_string()))
    }

    fn save(&self, server: &str, refresh: &str) {
        let saved = Self::entry().map(|e| e.set_password(&format!("{server}\n{refresh}")));
        if !matches!(saved, Some(Ok(()))) {
            tracing::warn!("Couldn't save the Orbit session; you'll have to sign in next time.");
        }
    }

    fn clear(&self) {
        if let Some(entry) = Self::entry() {
            let _ = entry.delete_credential();
        }
    }
}

#[derive(Deserialize)]
struct TokenResponse {
    access_token: String,
    refresh_token: String,
    expires_in: u64,
}

#[derive(Deserialize, serde::Serialize, Debug, PartialEq)]
pub struct Methods {
    pub github: bool,
    pub google: bool,
    pub email: bool,
}

#[derive(Deserialize)]
struct ErrorBody {
    error: ErrorDetail,
}

#[derive(Deserialize)]
struct ErrorDetail {
    code: String,
    message: String,
    fix: Option<String>,
}

struct Session {
    server: Url,
    refresh: String,
    access: Option<(String, Instant)>,
}

pub struct Account {
    // Held across a refresh on purpose: a refresh token works once, and using it
    // twice signs the session out.
    session: Mutex<Option<Session>>,
    store: Box<dyn RefreshStore>,
    http: reqwest::Client,
}

enum Refused {
    /// The server no longer accepts the session.
    SignedOut,
    Other(String),
}

impl Account {
    pub fn new(store: Box<dyn RefreshStore>) -> Self {
        let session = store.load().and_then(|(server, refresh)| {
            Some(Session {
                server: parse_server(&server).ok()?,
                refresh,
                access: None,
            })
        });
        let http = reqwest::Client::builder()
            // A redirect could carry a token or sign-in code elsewhere.
            .redirect(reqwest::redirect::Policy::none())
            .timeout(Duration::from_secs(20))
            .build()
            .expect("HTTP client");
        Self {
            session: Mutex::new(session),
            store,
            http,
        }
    }

    pub async fn server(&self) -> Option<Url> {
        self.session.lock().await.as_ref().map(|s| s.server.clone())
    }

    pub async fn methods(&self, server: &Url) -> Result<Methods, String> {
        let response = self
            .http
            .get(join(server, "api/auth/methods"))
            .send()
            .await
            .map_err(|_| UNREACHABLE.to_string())?;
        read(response).await.map_err(|e| match e {
            Refused::Other(message) => message,
            Refused::SignedOut => "That address didn't answer like an Orbit server.".to_string(),
        })
    }

    pub async fn email_start(
        &self,
        server: &Url,
        email: &str,
        challenge: &str,
        redirect: &str,
    ) -> Result<(), String> {
        let body = serde_json::json!({
            "email": email, "client": "desktop",
            "code_challenge": challenge, "redirect": redirect,
        });
        let response = self.post(server, "api/auth/email/start", &body).await?;
        if response.status().is_success() {
            return Ok(());
        }
        Err(message_of(&response.bytes().await.unwrap_or_default()))
    }

    /// Exchanges the one-time code from the sign-in redirect for tokens.
    pub async fn exchange(&self, server: &Url, code: &str, verifier: &str) -> Result<(), String> {
        let body = serde_json::json!({
            "grant_type": "authorization_code", "code": code, "code_verifier": verifier,
        });
        let tokens: TokenResponse = read(self.post(server, "api/auth/token", &body).await?)
            .await
            .map_err(|e| match e {
                Refused::Other(message) => message,
                Refused::SignedOut => "That sign-in is no longer valid. Try again.".to_string(),
            })?;
        let mut session = self.session.lock().await;
        self.store.save(server.as_str(), &tokens.refresh_token);
        *session = Some(Session {
            server: server.clone(),
            refresh: tokens.refresh_token,
            access: Some((tokens.access_token, expiry(tokens.expires_in))),
        });
        Ok(())
    }

    /// The server and a currently valid access token for it.
    pub async fn access_token(&self) -> Result<(Url, String), String> {
        let mut guard = self.session.lock().await;
        let session = guard.as_mut().ok_or(SIGNED_OUT)?;
        if let Some((access, expires)) = &session.access
            && Instant::now() + EARLY < *expires
        {
            return Ok((session.server.clone(), access.clone()));
        }
        let body = serde_json::json!({
            "grant_type": "refresh_token", "refresh_token": session.refresh,
        });
        let response = self.post(&session.server, "api/auth/token", &body).await?;
        match read::<TokenResponse>(response).await {
            Ok(tokens) => {
                // Save the new refresh token before anything else can fail.
                self.store
                    .save(session.server.as_str(), &tokens.refresh_token);
                session.refresh = tokens.refresh_token;
                session.access = Some((tokens.access_token.clone(), expiry(tokens.expires_in)));
                Ok((session.server.clone(), tokens.access_token))
            }
            Err(Refused::SignedOut) => {
                self.store.clear();
                *guard = None;
                Err(SIGNED_OUT.to_string())
            }
            Err(Refused::Other(message)) => Err(message),
        }
    }

    /// Forgets the session, even if the server can't be told.
    pub async fn sign_out(&self) {
        let old = self.session.lock().await.take();
        self.store.clear();
        if let Some(session) = old {
            let body = serde_json::json!({ "refresh_token": session.refresh });
            let _ = self.post(&session.server, "api/auth/logout", &body).await;
        }
    }

    async fn post(
        &self,
        server: &Url,
        path: &str,
        body: &serde_json::Value,
    ) -> Result<reqwest::Response, String> {
        self.http
            .post(join(server, path))
            .header("Content-Type", "application/json")
            .body(body.to_string())
            .send()
            .await
            .map_err(|_| UNREACHABLE.to_string())
    }
}

const SIGNED_OUT: &str = "Sign in to your Orbit account first.";
const UNREACHABLE: &str =
    "Couldn't reach your Orbit server. Check the address and your connection.";

fn join(server: &Url, path: &str) -> String {
    format!("{}{path}", server.as_str())
}

fn expiry(seconds: u64) -> Instant {
    Instant::now() + Duration::from_secs(seconds.min(24 * 3600))
}

fn message_of(body: &[u8]) -> String {
    match serde_json::from_slice::<ErrorBody>(body) {
        Ok(ErrorBody { error }) => match error.fix {
            Some(fix) => format!("{} {fix}", error.message),
            None => error.message,
        },
        Err(_) => "The server answered with an error.".to_string(),
    }
}

async fn read<T: serde::de::DeserializeOwned>(response: reqwest::Response) -> Result<T, Refused> {
    let ok = response.status().is_success();
    let body = response
        .bytes()
        .await
        .map_err(|_| Refused::Other(UNREACHABLE.to_string()))?;
    if ok {
        return serde_json::from_slice(&body).map_err(|_| {
            Refused::Other("That address didn't answer like an Orbit server.".into())
        });
    }
    if serde_json::from_slice::<ErrorBody>(&body).is_ok_and(|b| b.error.code == "invalid_grant") {
        return Err(Refused::SignedOut);
    }
    Err(Refused::Other(message_of(&body)))
}

#[cfg(test)]
pub mod tests {
    use super::*;
    use std::sync::{Arc, Mutex as StdMutex};

    #[derive(Clone, Default)]
    pub struct Memory(pub Arc<StdMutex<Option<(String, String)>>>);

    impl RefreshStore for Memory {
        fn load(&self) -> Option<(String, String)> {
            self.0.lock().unwrap().clone()
        }
        fn save(&self, server: &str, refresh: &str) {
            *self.0.lock().unwrap() = Some((server.into(), refresh.into()));
        }
        fn clear(&self) {
            *self.0.lock().unwrap() = None;
        }
    }

    #[test]
    fn server_addresses() {
        for (input, want) in [
            ("orbit.example.com", "https://orbit.example.com/"),
            (" https://Orbit.Example.com/ ", "https://orbit.example.com/"),
            ("http://127.0.0.1:8080", "http://127.0.0.1:8080/"),
        ] {
            assert_eq!(parse_server(input).unwrap().as_str(), want, "{input}");
        }
        for input in [
            "",
            "orbit",
            "http://orbit.example.com",
            "https://u:p@orbit.example.com",
            "https://orbit.example.com/path",
            "https://orbit.example.com?x=1",
            "ftp://orbit.example.com",
        ] {
            assert!(parse_server(input).is_err(), "{input}");
        }
    }

    #[tokio::test]
    async fn one_refresh_serves_concurrent_callers() {
        let (server, seen) = crate::orbit::testing::fake_server(|request| {
            assert!(request.contains("\"refresh_token\":\"old\""), "{request}");
            (
                200,
                r#"{"access_token":"a1","refresh_token":"new","expires_in":900}"#.into(),
            )
        })
        .await;
        let store = Memory::default();
        store.save(server.as_str(), "old");
        let account = Arc::new(Account::new(Box::new(store.clone())));
        let calls: Vec<_> = (0..8)
            .map(|_| {
                let account = account.clone();
                tokio::spawn(async move { account.access_token().await.unwrap().1 })
            })
            .collect();
        for call in calls {
            assert_eq!(call.await.unwrap(), "a1");
        }
        assert_eq!(seen.lock().unwrap().len(), 1);
        assert_eq!(store.load().unwrap().1, "new");
    }

    #[tokio::test]
    async fn a_refused_refresh_signs_out_but_an_outage_does_not() {
        for (status, body, kept) in [
            (
                400,
                r#"{"error":{"code":"invalid_grant","message":"No longer valid."}}"#,
                false,
            ),
            (502, "bad gateway", true),
        ] {
            let (server, _) =
                crate::orbit::testing::fake_server(move |_| (status, body.into())).await;
            let store = Memory::default();
            store.save(server.as_str(), "old");
            let account = Account::new(Box::new(store.clone()));
            assert!(account.access_token().await.is_err());
            assert_eq!(store.load().is_some(), kept, "status {status}");
        }
    }
}
