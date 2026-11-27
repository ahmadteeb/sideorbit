//! A tiny HTTP server for tests.

use std::sync::{Arc, Mutex};

use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
};
use url::Url;

/// Answers each connection with `respond(request)` and records the requests.
pub async fn fake_server(
    respond: impl Fn(&str) -> (u16, String) + Send + Sync + 'static,
) -> (Url, Arc<Mutex<Vec<String>>>) {
    let listener = TcpListener::bind(("127.0.0.1", 0)).await.unwrap();
    let url = Url::parse(&format!("http://{}", listener.local_addr().unwrap())).unwrap();
    let seen = Arc::new(Mutex::new(Vec::new()));
    let log = seen.clone();
    tokio::spawn(async move {
        while let Ok((mut stream, _)) = listener.accept().await {
            let (head, mut body) = crate::orbit::proxy::read_head(&mut stream).await.unwrap();
            let length: usize = head
                .lines()
                .find_map(|l| {
                    l.to_ascii_lowercase()
                        .strip_prefix("content-length:")
                        .map(|v| v.trim().parse().unwrap())
                })
                .unwrap_or(0);
            while body.len() < length {
                let mut chunk = [0u8; 1024];
                let n = stream.read(&mut chunk).await.unwrap();
                body.extend_from_slice(&chunk[..n]);
            }
            let request = head + &String::from_utf8_lossy(&body);
            let (status, answer) = respond(&request);
            log.lock().unwrap().push(request);
            let response = format!(
                "HTTP/1.1 {status} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            );
            let _ = stream.write_all(response.as_bytes()).await;
            let _ = stream.shutdown().await;
        }
    });
    (url, seen)
}
