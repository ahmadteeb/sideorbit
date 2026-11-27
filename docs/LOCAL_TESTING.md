# Testing Orbit on your own network

Runs the whole server on one computer with Docker, for other devices on the same network. It needs no domain and no mail server. **Never expose this setup to the internet.**

## 1. Start it
```bash
cd deploy
./local.sh up
```
It finds this computer's address, generates secrets in `.env.local`, builds, starts and prints three links. `./local.sh logs`, `./local.sh down` (keeps data) and `./local.sh reset` (deletes it) do what they say.

- Ports used: **8443** (Orbit), **8025** (mail), **8081** (certificate), bound to your network address only. Docker's published ports bypass `ufw`, so devices on your network can reach them.
- If your router gives this computer a new address, run `reset`, then `up`.

## 2. Trust the certificate
The server uses a certificate authority that Caddy creates locally. Until a device trusts it, browsers warn and the apps refuse to connect.

> Whoever holds that authority's private key (it stays in a Docker volume on this computer) can impersonate any website to a device that trusts it. **Remove the trust when you finish testing.**

**This computer (Linux):**
```bash
./local.sh ca
sudo trust anchor --store orbit-local-ca.crt      # Arch, Fedora. Remove: sudo trust anchor --remove orbit-local-ca.crt
```
Debian/Ubuntu: copy it to `/usr/local/share/ca-certificates/orbit-local.crt` and run `sudo update-ca-certificates`. Chrome and Firefox keep their own list: import the file under their certificate settings.

**iPhone:** in Safari open `http://<address>:8081/ca.crt` → Allow → Settings → **Profile Downloaded** → Install. Then Settings → General → About → **Certificate Trust Settings** → switch it on. To remove: Settings → General → VPN & Device Management.

## 3. Sign in
Open `https://<address>:8443`, choose email, type any address. Open the mail link (`http://<address>:8025`, user `orbit`, password printed by `up`), then the message, then its link. Open that link **in the same browser or device** you are signing in on.

GitHub sign-in is optional: create an OAuth app with callback `https://<address>:8443/api/auth/github/callback`, put its ID and secret in `.env.local`, run `up` again. Google doesn't accept network addresses.

## 4. What to try
| | Check |
|---|---|
| Dashboard | Sign in; Settings → Enable alerts → Send test alert (on iPhone: Add to Home Screen first) |
| Gateway | The `curl` commands in `SELF_HOSTING.md` → "Verify your install", adding `--cacert deploy/orbit-local-ca.crt` |
| iPhone app | Install `Orbit.ipa` from the latest `ios` run with iloader or Orbit Setup; server address `<address>:8443` |
| Orbit Setup | Install the build from the latest `desktop` run; server `<address>:8443` (trust the certificate on that computer first) |

Report what failed together with `./local.sh logs` output. The logs hold no tokens or addresses.
