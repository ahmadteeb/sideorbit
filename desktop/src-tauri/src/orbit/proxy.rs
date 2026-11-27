//! ORBIT: a loopback proxy in front of the server's anisette gateway.
//!
//! The Apple sign-in library takes an anisette URL but can't send an
//! Authorization header. It is given `http://127.0.0.1:<port>/<secret>`, and
//! this adds the user's Orbit token on the way out. Anisette traffic carries no
//! Apple credentials. The path secret keeps other programs on this computer
//! from using the user's quota.

use std::{sync::Arc, time::Duration};

use rustls_platform_verifier::BuilderVerifierExt;
use tokio::{
    io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt, copy_bidirectional},
    net::{TcpListener, TcpStream},
    time::timeout,
};
use url::Url;

use super::session::Account;

const MAX_HEAD: usize = 16 << 10;
/// Longer than the server's own limit on a provisioning session.
const MAX_CONNECTION: Duration = Duration::from_secs(180);
const ENDPOINTS: [&str; 3] = ["client_info", "get_headers", "provisioning_session"];

/// Starts the proxy and returns the anisette URL to use. It runs until the app
/// exits.
pub async fn start(account: Arc<Account>) -> std::io::Result<String> {
    let listener = TcpListener::bind(("127.0.0.1", 0)).await?;
    let secret = super::random_token();
    let url = format!("http://{}/{secret}", listener.local_addr()?);
    tokio::spawn(async move {
        while let Ok((client, _)) = listener.accept().await {
            let account = account.clone();
            let secret = secret.clone();
            tokio::spawn(async move {
                if let Ok(Err(error)) =
                    timeout(MAX_CONNECTION, serve(client, account, secret)).await
                {
                    // Never logs headers or bodies.
                    tracing::warn!("anisette proxy: {error}");
                }
            });
        }
    });
    Ok(url)
}

/// Reads up to the blank line ending an HTTP head. Returns it and any bytes read
/// past it.
pub async fn read_head<R: AsyncRead + Unpin>(stream: &mut R) -> std::io::Result<(String, Vec<u8>)> {
    let mut data = Vec::new();
    let mut chunk = [0u8; 2048];
    loop {
        if let Some(end) = data.windows(4).position(|w| w == b"\r\n\r\n") {
            let rest = data.split_off(end + 4);
            let head = String::from_utf8(data)
                .map_err(|_| std::io::Error::other("request head is not text"))?;
            return Ok((head, rest));
        }
        if data.len() > MAX_HEAD {
            return Err(std::io::Error::other("request head is too long"));
        }
        let read = stream.read(&mut chunk).await?;
        if read == 0 {
            return Err(std::io::Error::other("connection closed early"));
        }
        data.extend_from_slice(&chunk[..read]);
    }
}

/// Which anisette endpoint a request asks for, if the secret is right.
fn endpoint<'a>(head: &'a str, secret: &str) -> Option<(&'a str, &'static str)> {
    let mut parts = head.lines().next()?.split(' ');
    let (method, target, version) = (parts.next()?, parts.next()?, parts.next()?);
    if !matches!(method, "GET" | "POST") || version != "HTTP/1.1" {
        return None;
    }
    let name = target
        .strip_prefix('/')?
        .strip_prefix(secret)?
        .strip_prefix("/v3/")?;
    ENDPOINTS
        .into_iter()
        .find(|e| *e == name)
        .map(|e| (method, e))
}

/// Builds the head sent to the server.
fn rewrite(head: &str, method: &str, endpoint: &str, authority: &str, token: &str) -> String {
    let headers = head.split("\r\n").skip(1).filter_map(|line| {
        let (name, value) = line.split_once(':')?;
        Some((name, value.trim()))
    });
    let upgrade = headers
        .clone()
        .any(|(n, v)| n.eq_ignore_ascii_case("upgrade") && v.eq_ignore_ascii_case("websocket"));
    let mut out = format!(
        "{method} /anisette/v3/{endpoint} HTTP/1.1\r\nHost: {authority}\r\nAuthorization: Bearer {token}\r\n"
    );
    for (name, value) in headers {
        let lower = name.to_ascii_lowercase();
        let dropped = matches!(lower.as_str(), "host" | "authorization" | "cookie")
            || lower.starts_with("proxy-")
            || (lower == "connection" && !upgrade);
        if !dropped {
            out.push_str(&format!("{name}: {value}\r\n"));
        }
    }
    // One request per connection, so every request gets its head rewritten.
    if !upgrade {
        out.push_str("Connection: close\r\n");
    }
    out.push_str("\r\n");
    out
}

async fn serve(
    mut client: TcpStream,
    account: Arc<Account>,
    secret: String,
) -> std::io::Result<()> {
    let (head, rest) = read_head(&mut client).await?;
    let Some((method, name)) = endpoint(&head, &secret) else {
        return refuse(&mut client, "404 Not Found").await;
    };
    let (server, token) = match account.access_token().await {
        Ok(found) => found,
        Err(_) => return refuse(&mut client, "401 Unauthorized").await,
    };
    let request = rewrite(&head, method, name, &authority(&server), &token);

    let host = server.host_str().unwrap_or_default().to_string();
    let tcp =
        TcpStream::connect((host.as_str(), server.port_or_known_default().unwrap_or(443))).await?;
    if server.scheme() == "http" {
        // Only a server on this computer (see parse_server).
        return forward(client, tcp, &request, &rest).await;
    }
    let tls = tls_connector()?
        .connect(
            rustls::pki_types::ServerName::try_from(host).map_err(std::io::Error::other)?,
            tcp,
        )
        .await?;
    forward(client, tls, &request, &rest).await
}

fn authority(server: &Url) -> String {
    let host = server.host_str().unwrap_or_default();
    match server.port() {
        Some(port) => format!("{host}:{port}"),
        None => host.to_string(),
    }
}

fn tls_connector() -> std::io::Result<tokio_rustls::TlsConnector> {
    let config = rustls::ClientConfig::builder_with_provider(Arc::new(
        rustls::crypto::aws_lc_rs::default_provider(),
    ))
    .with_safe_default_protocol_versions()
    .map_err(std::io::Error::other)?
    // The same trust as the rest of the app: the system's certificates.
    .with_platform_verifier()
    .map_err(std::io::Error::other)?
    .with_no_client_auth();
    Ok(tokio_rustls::TlsConnector::from(Arc::new(config)))
}

async fn forward<S: AsyncRead + AsyncWrite + Unpin>(
    mut client: TcpStream,
    mut upstream: S,
    head: &str,
    rest: &[u8],
) -> std::io::Result<()> {
    upstream.write_all(head.as_bytes()).await?;
    upstream.write_all(rest).await?;
    upstream.flush().await?;
    match copy_bidirectional(&mut client, &mut upstream).await {
        // The other side hanging up without a TLS goodbye is normal.
        Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => Ok(()),
        result => result.map(|_| ()),
    }
}

async fn refuse(client: &mut TcpStream, status: &str) -> std::io::Result<()> {
    client
        .write_all(
            format!("HTTP/1.1 {status}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
                .as_bytes(),
        )
        .await
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::orbit::session::{RefreshStore, tests::Memory};

    #[test]
    fn only_the_three_endpoints_with_the_secret() {
        let ok = |line: &str| endpoint(&format!("{line}\r\n\r\n"), "s3cret").map(|(_, e)| e);
        assert_eq!(
            ok("GET /s3cret/v3/client_info HTTP/1.1"),
            Some("client_info")
        );
        assert_eq!(
            ok("POST /s3cret/v3/get_headers HTTP/1.1"),
            Some("get_headers")
        );
        for line in [
            "GET /v3/client_info HTTP/1.1",
            "GET /wrong/v3/client_info HTTP/1.1",
            "GET /s3cretx/v3/client_info HTTP/1.1",
            "GET /s3cret/v3/../api/me HTTP/1.1",
            "GET /s3cret/v3/client_info?x=1 HTTP/1.1",
            "DELETE /s3cret/v3/client_info HTTP/1.1",
            "GET http://evil.example/s3cret/v3/client_info HTTP/1.1",
        ] {
            assert_eq!(ok(line), None, "{line}");
        }
    }

    #[test]
    fn the_head_gains_the_token_and_loses_what_the_caller_sent() {
        let head = "POST /s/v3/get_headers HTTP/1.1\r\nHost: 127.0.0.1:5\r\nAuthorization: Basic x\r\nCookie: a=b\r\nContent-Length: 2\r\nConnection: keep-alive\r\n\r\n";
        let out = rewrite(head, "POST", "get_headers", "orbit.example.com", "tok");
        assert!(out.starts_with("POST /anisette/v3/get_headers HTTP/1.1\r\nHost: orbit.example.com\r\nAuthorization: Bearer tok\r\n"));
        assert!(
            out.contains("Content-Length: 2\r\n") && out.ends_with("Connection: close\r\n\r\n")
        );
        for gone in ["Basic", "Cookie", "127.0.0.1", "keep-alive", "/s/"] {
            assert!(!out.contains(gone), "{gone} survived: {out}");
        }
        let ws = "GET /s/v3/provisioning_session HTTP/1.1\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: k\r\n\r\n";
        let out = rewrite(ws, "GET", "provisioning_session", "h", "t");
        assert!(
            out.contains("Connection: Upgrade\r\n") && !out.contains("close"),
            "{out}"
        );
    }

    async fn call(url: &str, path: &str) -> String {
        let url = Url::parse(&format!("{url}{path}")).unwrap();
        let mut stream = TcpStream::connect(("127.0.0.1", url.port().unwrap()))
            .await
            .unwrap();
        let request = format!("GET {} HTTP/1.1\r\nHost: x\r\n\r\n", url.path());
        stream.write_all(request.as_bytes()).await.unwrap();
        let mut response = String::new();
        stream.read_to_string(&mut response).await.unwrap();
        response
    }

    #[tokio::test]
    async fn forwards_with_the_token_and_refuses_everything_else() {
        let (server, seen) = crate::orbit::testing::fake_server(|request| {
            if request.starts_with("POST /api/auth/token") {
                (
                    200,
                    r#"{"access_token":"abc","refresh_token":"n","expires_in":900}"#.into(),
                )
            } else {
                (200, "anisette says hello".into())
            }
        })
        .await;
        let store = Memory::default();
        store.save(server.as_str(), "r");
        let url = start(Arc::new(Account::new(Box::new(store))))
            .await
            .unwrap();

        let ok = call(&url, "/v3/client_info").await;
        assert!(
            ok.starts_with("HTTP/1.1 200") && ok.ends_with("anisette says hello"),
            "{ok}"
        );
        let forwarded = seen.lock().unwrap().last().unwrap().clone();
        assert!(
            forwarded.starts_with("GET /anisette/v3/client_info HTTP/1.1"),
            "{forwarded}"
        );
        assert!(
            forwarded.contains("Authorization: Bearer abc"),
            "{forwarded}"
        );

        let before = seen.lock().unwrap().len();
        let base = url.rsplit_once('/').unwrap().0.to_string();
        assert!(
            call(&base, "/guess/v3/client_info")
                .await
                .starts_with("HTTP/1.1 404")
        );
        assert!(call(&url, "/v3/other").await.starts_with("HTTP/1.1 404"));
        assert_eq!(seen.lock().unwrap().len(), before);
    }

    #[tokio::test]
    async fn signed_out_gets_401() {
        let url = start(Arc::new(Account::new(Box::new(Memory::default()))))
            .await
            .unwrap();
        assert!(
            call(&url, "/v3/client_info")
                .await
                .starts_with("HTTP/1.1 401")
        );
    }
}
