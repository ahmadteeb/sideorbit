# Orbit: user guide (draft)

> **Orbit is not finished.** Today you can run the server, use the web dashboard and sign in with the iPhone app, which lists app sources, warns you before Orbit itself expires and shows your iPhone on the dashboard. **It can't install or refresh other apps yet, and there is no Orbit Setup desktop app.** Sections 2 and 3 below describe how it is meant to work.

## What works today
1. Open your server's address in a browser and sign in. *(screenshot: sign-in page)*
2. Install `Orbit.ipa` on your iPhone with [iloader](https://github.com/nab138/iloader), trust the profile and turn on Developer Mode.
3. Open Orbit, type your server's address and sign in. *(screenshot: iPhone sign-in)*
4. Home shows how long Orbit has left. Tap **Warn me before it expires**. *(screenshot: Home)*
5. For alerts in the dashboard on iPhone: Safari → Share → **Add to Home Screen**, open it from there, then Settings → **Enable alerts** → **Send test alert**. *(screenshot: alerts)*

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

### Dashboard and sign-in (today)

| What you see | Fix |
|---|---|
| "You already have an Orbit account with this email" | Sign in the way you did the first time, or use an email link |
| A sign-in link "no longer works" | Links work once, for 15 minutes. Ask for a new one |
| "This browser can't receive alerts" | On iPhone, alerts work only from the Home Screen app |
| "Notifications are blocked" | Allow them for the site in your browser's settings, then reload |
| "This account has reached its device limit" | Remove a device you no longer use in the dashboard |
| A removed iPhone isn't listed again | Open Orbit on it → Home → **Add it again** |

### Installing and refreshing (when built)

| Message | Fix |
|---|---|
| "Turn on LocalDevVPN" | Open LocalDevVPN → Connect → back in Orbit tap **Retry** |
| "Pairing file needs replacing" | Usually after an iOS update. Open Orbit Setup on a computer → **Re-pair only** |
| "Confirm your Apple ID" | Tap the notification and enter your 2FA code |
| "No app slots left" | Free accounts allow 3 apps including Orbit. Remove one, or use the LiveContainer setup |
| "App ID limit reached" | Apple allows 10 new App IDs per week. Orbit shows when the next one frees up |
