//! ORBIT: keeping sensitive data out of logs, error messages and shared folders.

use tracing_subscriber::filter::{LevelFilter, Targets};

/// What gets logged. Dependencies' debug output holds the Apple ID email,
/// anisette machine data and pairing material, and logs get pasted into bug
/// reports, so only this app logs at debug level. (Records from the `log` crate
/// fall under the default.)
pub fn log_filter() -> Targets {
    Targets::new()
        .with_default(LevelFilter::INFO)
        .with_target("iloader_lib", LevelFilter::DEBUG)
}

/// Removes the anisette proxy's secret from a message: HTTP errors quote URLs.
pub fn redact(text: &str) -> String {
    const HOST: &str = "127.0.0.1:";
    let mut out = String::new();
    let mut rest = text;
    while let Some(at) = rest.find(HOST) {
        let (before, tail) = rest.split_at(at + HOST.len());
        out.push_str(before);
        let port = tail.chars().take_while(char::is_ascii_digit).count();
        let (digits, after) = tail.split_at(port);
        out.push_str(digits);
        rest = after;
        if let Some(path) = after.strip_prefix('/') {
            let secret = path
                .chars()
                .take_while(|c| c.is_ascii_alphanumeric() || *c == '-' || *c == '_')
                .count();
            if secret >= 40 {
                out.push_str("/<hidden>");
                rest = &path[secret..];
            }
        }
    }
    out.push_str(rest);
    out
}

/// A folder only this user can open.
#[cfg(unix)]
pub fn private_dir(path: &std::path::Path) -> std::io::Result<()> {
    use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
    std::fs::DirBuilder::new()
        .recursive(true)
        .mode(0o700)
        .create(path)?;
    // It may have existed already with looser permissions.
    std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o700))
}

#[cfg(not(unix))]
pub fn private_dir(path: &std::path::Path) -> std::io::Result<()> {
    // The user's profile folder is already private on Windows.
    std::fs::create_dir_all(path)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::{Arc, Mutex};
    use tracing_subscriber::{Layer, layer::SubscriberExt};

    #[derive(Clone, Default)]
    struct Seen(Arc<Mutex<Vec<String>>>);

    impl<S: tracing::Subscriber> Layer<S> for Seen {
        fn on_event(
            &self,
            event: &tracing::Event<'_>,
            _: tracing_subscriber::layer::Context<'_, S>,
        ) {
            self.0.lock().unwrap().push(format!(
                "{} {}",
                event.metadata().target(),
                event.metadata().level()
            ));
        }
    }

    #[test]
    fn dependencies_debug_output_is_dropped() {
        let seen = Seen::default();
        let subscriber =
            tracing_subscriber::registry().with(seen.clone().with_filter(log_filter()));
        tracing::subscriber::with_default(subscriber, || {
            tracing::debug!(target: "isideload", "adi_pb");
            tracing::debug!(target: "idevice::remote_pairing", "keys");
            tracing::debug!(target: "log", "from the log crate");
            tracing::info!(target: "isideload", "progress");
            tracing::debug!(target: "iloader_lib::account", "ours");
        });
        assert_eq!(
            *seen.0.lock().unwrap(),
            ["isideload INFO", "iloader_lib::account DEBUG"]
        );
    }

    #[test]
    fn the_proxy_secret_is_removed_from_messages() {
        let secret = "A".repeat(43);
        let message =
            format!("error for url (http://127.0.0.1:4123/{secret}/v3/get_headers): refused");
        assert_eq!(
            redact(&message),
            "error for url (http://127.0.0.1:4123/<hidden>/v3/get_headers): refused"
        );
        for plain in ["no address", "http://127.0.0.1:80/callback", "127.0.0.1:"] {
            assert_eq!(redact(plain), plain);
        }
    }

    #[cfg(unix)]
    #[test]
    fn private_folders_are_owner_only() {
        use std::os::unix::fs::PermissionsExt;
        let dir = std::env::temp_dir().join(format!("orbit-test-{}", crate::orbit::random_token()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::set_permissions(&dir, std::fs::Permissions::from_mode(0o755)).unwrap();
        private_dir(&dir).unwrap();
        assert_eq!(
            std::fs::metadata(&dir).unwrap().permissions().mode() & 0o777,
            0o700
        );
        std::fs::remove_dir(&dir).unwrap();
    }
}
