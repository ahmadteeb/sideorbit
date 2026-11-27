//! ORBIT: browser sign-in with a loopback redirect and PKCE (RFC 8252).

use std::time::Duration;

use base64::{Engine, engine::general_purpose::URL_SAFE_NO_PAD};
use sha2::{Digest, Sha256};
use tokio::{
    io::AsyncWriteExt,
    net::{TcpListener, TcpStream},
    time::timeout,
};
use url::Url;

/// How long the browser has to come back.
pub const WAIT: Duration = Duration::from_secs(300);

pub fn challenge(verifier: &str) -> String {
    URL_SAFE_NO_PAD.encode(Sha256::digest(verifier.as_bytes()))
}

/// Listens on this computer only; returns the redirect address to give the server.
pub async fn listen() -> std::io::Result<(TcpListener, String)> {
    let listener = TcpListener::bind(("127.0.0.1", 0)).await?;
    let redirect = format!(
        "http://127.0.0.1:{}/callback",
        listener.local_addr()?.port()
    );
    Ok((listener, redirect))
}

pub fn start_url(
    server: &Url,
    provider: &str,
    challenge: &str,
    redirect: &str,
) -> Result<String, String> {
    if !matches!(provider, "github" | "google") {
        return Err("Unknown sign-in method.".into());
    }
    let mut url = server
        .join(&format!("api/auth/{provider}/start"))
        .map_err(|e| e.to_string())?;
    url.query_pairs_mut()
        .append_pair("client", "desktop")
        .append_pair("code_challenge", challenge)
        .append_pair("redirect", redirect);
    Ok(url.into())
}

/// The result in a callback request line, or None if it isn't the callback.
fn parse_callback(head: &str) -> Option<Result<String, String>> {
    let target = head
        .lines()
        .next()?
        .strip_prefix("GET ")?
        .split(' ')
        .next()?;
    let url = Url::parse("http://127.0.0.1").ok()?.join(target).ok()?;
    if url.path() != "/callback" {
        return None;
    }
    let value = |name: &str| {
        url.query_pairs()
            .find(|(k, _)| k == name)
            .map(|(_, v)| v.into_owned())
    };
    if let Some(error) = value("error") {
        return Some(Err(explain(&error).to_string()));
    }
    value("code").map(Ok)
}

fn explain(code: &str) -> &'static str {
    match code {
        "access_denied" => "Sign-in was cancelled.",
        "email_unverified" => {
            "That account has no verified email. Verify it there, or use an email link."
        }
        "account_exists" => {
            "You already have an Orbit account with this email. Sign in the way you did before, or use an email link."
        }
        _ => "The sign-in service didn't respond. Try again.",
    }
}

const PAGE: &str = "<!doctype html><meta charset=utf-8><meta name=color-scheme content='light dark'><title>Orbit Setup</title><body style='font-family:system-ui;padding:2rem'><h1>Return to Orbit Setup</h1><p>You can close this tab.</p>";

async fn answer(stream: &mut TcpStream, status: &str, body: &str) {
    let response = format!(
        "HTTP/1.1 {status}\r\nContent-Type: text/html; charset=utf-8\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
    let _ = stream.write_all(response.as_bytes()).await;
}

/// Waits for the browser to arrive with a code. Other requests are turned away
/// and don't end the wait.
pub async fn wait_for_code(listener: TcpListener) -> Result<String, String> {
    let wait = async {
        loop {
            let (mut stream, _) = listener.accept().await.map_err(|e| e.to_string())?;
            let head = timeout(Duration::from_secs(5), super::proxy::read_head(&mut stream)).await;
            let Ok(Ok((head, _))) = head else { continue };
            match parse_callback(&head) {
                Some(result) => {
                    answer(&mut stream, "200 OK", PAGE).await;
                    return result;
                }
                None => answer(&mut stream, "404 Not Found", "").await,
            }
        }
    };
    timeout(WAIT, wait)
        .await
        .map_err(|_| "Sign-in timed out. Try again.".to_string())?
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::AsyncReadExt;

    #[test]
    fn pkce_matches_rfc_7636() {
        assert_eq!(
            challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"),
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
        );
    }

    #[test]
    fn the_start_address_encodes_the_redirect() {
        let server = Url::parse("https://orbit.example.com/").unwrap();
        let url = start_url(&server, "github", "abc", "http://127.0.0.1:5/callback").unwrap();
        assert_eq!(
            url,
            "https://orbit.example.com/api/auth/github/start?client=desktop&code_challenge=abc&redirect=http%3A%2F%2F127.0.0.1%3A5%2Fcallback"
        );
        assert!(start_url(&server, "../x", "a", "b").is_err());
    }

    async fn visit(redirect: &str, path: &str) {
        let port = Url::parse(redirect).unwrap().port().unwrap();
        let mut stream = TcpStream::connect(("127.0.0.1", port)).await.unwrap();
        stream
            .write_all(format!("GET {path} HTTP/1.1\r\nHost: x\r\n\r\n").as_bytes())
            .await
            .unwrap();
        let mut out = String::new();
        stream.read_to_string(&mut out).await.unwrap();
    }

    #[tokio::test]
    async fn waits_past_other_requests_for_the_code() {
        let (listener, redirect) = listen().await.unwrap();
        let waiting = tokio::spawn(wait_for_code(listener));
        visit(&redirect, "/favicon.ico").await;
        visit(&redirect, "/callback").await;
        visit(&redirect, "/callback?code=xyz").await;
        assert_eq!(waiting.await.unwrap(), Ok("xyz".to_string()));
    }

    #[tokio::test]
    async fn a_callback_error_is_explained() {
        let (listener, redirect) = listen().await.unwrap();
        let waiting = tokio::spawn(wait_for_code(listener));
        visit(&redirect, "/callback?error=account_exists").await;
        assert!(
            waiting
                .await
                .unwrap()
                .unwrap_err()
                .contains("already have an Orbit account")
        );
    }
}
