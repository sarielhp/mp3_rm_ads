# AntennaPod Setup Guide for ABS Feeds

This guide covers how to subscribe to and stream podcasts managed by `abs` using the **AntennaPod** mobile app.

---

## 1. Network Requirements

- **Tailscale**: Your mobile device must be connected to the Tailscale VPN.
- **Server Address**:
  - MagicDNS hostname: `http://tred:8080`
  - Tailscale IPv4: `http://100.123.184.3:8080`
- **Port 8080**: Dedicated direct HTTP endpoint provided by Caddy that bypasses Traefik's `mtls-strict` policy, enabling standard mobile podcast clients to stream and download audio without requiring client certificates.

---

## 2. Setup Methods

### Method 1: Bulk Import (All Podcasts via OPML) — Recommended

All configured podcast feeds are exported to an OPML file hosted directly on the server:
```
http://100.123.184.3:8080/podcasts/antennapod.opml
```
*(or `http://tred:8080/podcasts/antennapod.opml`)*

#### Steps on Mobile Device:
1. Ensure Tailscale is active on your phone.
2. Open your phone's web browser and navigate to:
   ```
   http://100.123.184.3:8080/podcasts/antennapod.opml
   ```
   Your browser will download the file `antennapod.opml`.
3. Open **AntennaPod**.
4. Go to **Subscriptions**.
5. Tap the three-dot menu (**⋮**) in the top-right corner $\rightarrow$ select **Import/Export**.
6. Select **OPML import**.
7. Choose the downloaded `antennapod.opml` file from your device's Downloads folder.
8. Tap **Import All** (or select individual shows).

---

### Method 2: Add an Individual Podcast Manually

To subscribe to a specific show by RSS address:
1. Open **AntennaPod**.
2. Tap **+** (**Add Podcast**).
3. Tap **Add Podcast by RSS address**.
4. Enter the show's local feed URL:
   ```
   http://100.123.184.3:8080/podcasts/<Show-Folder>/feed.xml
   ```
   *(or `http://tred:8080/podcasts/<Show-Folder>/feed.xml`)*

   *Examples:*
   - `http://100.123.184.3:8080/podcasts/5-4/feed.xml`
   - `http://100.123.184.3:8080/podcasts/The%20Daily/feed.xml`
5. Tap **Confirm** $\rightarrow$ tap **Subscribe**.

---

## 3. Regenerating the OPML File

If you add new podcast subscriptions using `abs server add <feed-url> [title]`, regenerate the server OPML file:

```bash
abs server opml export /media/podcasts/clean/antennapod.opml
```

This updates the OPML file served by Caddy so you can re-import any new feeds into AntennaPod.

---

## 4. Technical Architecture & Features

### Audio Streaming & Range Requests
- Caddy serves audio files with full HTTP Range request support (`206 Partial Content`).
- AntennaPod can seek forward/backward within audio files and resume partially streamed episodes instantly without re-downloading.

### Feed Updates & Cover Art
- Each podcast folder in `/media/podcasts/clean/<show>/` contains a standard iTunes-compatible `feed.xml` and local cover image (`cover.jpg` / `cover.png`).
- When `abs` downloads new episodes, adds subscriptions, or completes ad removal, it updates `feed.xml` with:
  - Exact file size (`length`) and MIME type (`audio/mpeg`).
  - Audio duration (`itunes:duration`).
  - Stable, deterministic episode GUIDs.
  - Properly escaped Tailscale enclosure URLs.
  - Channel and episode cover art icons via both standard RSS `<image><url>` and `<itunes:image href="...">` for seamless AntennaPod icon rendering.
- AntennaPod checks `feed.xml` on its standard refresh schedule or when pulled down manually.
