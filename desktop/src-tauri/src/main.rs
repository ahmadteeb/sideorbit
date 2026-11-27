// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    #[cfg(target_os = "linux")]
    {
        use std::env;
        if env::var_os("__NV_DISABLE_EXPLICIT_SYNC").is_none() {
            unsafe {
                env::set_var("__NV_DISABLE_EXPLICIT_SYNC", "1");
            }
        }
    }

    // ORBIT: apps are unpacked under a predictable name in the temp folder, which
    // other users share on Linux. Use a private one. (Before any thread starts.)
    #[cfg(target_os = "linux")]
    {
        use std::{env, path::PathBuf};
        let cache = env::var_os("XDG_CACHE_HOME")
            .map(PathBuf::from)
            .or_else(|| env::var_os("HOME").map(|h| PathBuf::from(h).join(".cache")));
        if let Some(dir) = cache.map(|c| c.join("orbit-setup/tmp"))
            && iloader_lib::private_dir(&dir).is_ok()
        {
            unsafe { env::set_var("TMPDIR", &dir) };
        }
    }

    // To be quite honest, I have no idea how ring has made its way into the dependency tree in some cases.
    rustls::crypto::aws_lc_rs::default_provider()
        .install_default()
        .expect("Failed to install rustls crypto provider");
    isideload::init().expect("Failed to initialize error reporting");
    iloader_lib::run()
}
