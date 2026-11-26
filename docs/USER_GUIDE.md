# Orbit: user guide (draft; Phase 6 adds screenshots)

## What you need
- An iPhone or iPad on iOS 16 or later, a free Apple ID, and a Mac, Windows or Linux computer **once**, plus a USB cable.

## 1. On your iPhone first
1. Install **LocalDevVPN** from the App Store.
2. Open it, allow the VPN configuration, and tap **Connect**.

## 2. On your computer (one time, about 5 minutes)
1. Download **Orbit Setup** for your system from the Releases page.
   - macOS: if it says it can't be opened, right-click the app → **Open**, or go to System Settings → Privacy & Security → **Open Anyway**.
   - Windows: if SmartScreen appears, click **More info → Run anyway**.
2. Sign in to your Orbit account.
3. Plug in your iPhone, unlock it and tap **Trust**, then enter your passcode.
4. If asked, let Orbit Setup prepare drivers. This is automatic, and Linux asks for your password once.
5. Sign in with your Apple ID. It's used on your computer only to install Orbit.
6. Wait for **Done**. Orbit and its pairing file are now on your iPhone.

## 3. Back on your iPhone
1. **Settings → Privacy & Security → Developer Mode → On.** The phone restarts.
2. **Settings → General → VPN & Device Management →** your Apple ID **→ Trust**.
3. Open **Orbit**, sign in, sign in with your Apple ID again (it stays on this iPhone), and allow notifications.
4. Tap **Add refresh automation** and follow the steps. This makes refresh reliable.

## Everyday use
Orbit refreshes your apps before they expire. Keep LocalDevVPN connected. If something needs you, Orbit tells you exactly what to do.

## Troubleshooting

| Message | Fix |
|---|---|
| "Turn on LocalDevVPN" | Open LocalDevVPN → Connect → back in Orbit tap **Retry** |
| "Pairing file needs replacing" | Usually after an iOS update. Open Orbit Setup on a computer → **Re-pair only** |
| "Confirm your Apple ID" | Tap the notification and enter your 2FA code |
| "No app slots left" | Free accounts allow 3 apps including Orbit. Remove one, or use the LiveContainer setup |
| "App ID limit reached" | Apple allows 10 new App IDs per week. Orbit shows when the next one frees up |
