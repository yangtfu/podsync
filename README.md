# Podsync

![Podsync](docs/img/logo.png)

[![](https://github.com/yangtfu/podsync/workflows/CI/badge.svg)](https://github.com/yangtfu/podsync/actions?query=workflow%3ACI)
[![Nightly](https://github.com/yangtfu/podsync/actions/workflows/nightly.yml/badge.svg)](https://github.com/yangtfu/podsync/actions/workflows/nightly.yml)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/yangtfu/podsync)](https://github.com/yangtfu/podsync/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yangtfu/podsync)](https://goreportcard.com/report/github.com/yangtfu/podsync)

👉 [阅读中文版本 README](README_CN.md)

This repository is a fork of [mxpv/podsync](https://github.com/mxpv/podsync). The original project laid the foundation for
turning online video channels into podcast feeds, and this fork continues that mission while focusing on Bilibili support
and a handful of quality-of-life improvements that help self-hosters keep their feeds healthy.

If you just want the upstream experience, please visit the original project. If you're here, you're getting the upstream
feature set plus these additions while keeping compatibility with existing configs.

## ✨ Features

- Works with YouTube, Vimeo, **Bilibili**, and other supported providers.
- Feed-level knobs for audio/video variants, quality limits, artwork, language, and categories.
- mp3 encoding and post-processing via ffmpeg.
- Cron-style update scheduler with timezone support.
- Episode filters (title/duration) and automatic cleanup (keep last _N_ episodes).
- Configurable hooks for sending webhooks or triggering automations after each refresh.
- OPML export for easy client import.
- One-click deployment template for AWS plus Docker/Compose setups for everything else.
- Runs on Windows, macOS, Linux, ARM SBCs, and inside containers.
- Automatic yt-dlp self update and API key rotation to stay ahead of rate limits.

## 📋 Dependencies

Running the binary directly (outside Docker) requires the following tools to be available on your system:

- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
- [`ffmpeg`](https://ffmpeg.org/)
- [`go`](https://go.dev/)

macOS users can install the prerequisites with Homebrew:

```bash
brew install yt-dlp ffmpeg go
```

## 📖 Documentation

- [How to get Vimeo API token](./docs/how_to_get_vimeo_token.md)
- [How to get YouTube API Key](./docs/how_to_get_youtube_api_key.md)
- [Podsync on QNAP NAS Guide](./docs/how_to_setup_podsync_on_qnap_nas.md)
- [Schedule updates with cron](./docs/cron.md)

## 🌙 Nightly builds

Nightly builds are published every midnight from the `main` branch for anyone who wants the latest fixes early:

```bash
docker run -it --rm ghcr.io/yangtfu/podsync:nightly
```

## 🔑 Access tokens

You will need API credentials for each platform you plan to pull from:

- [How to get YouTube API key](https://elfsight.com/blog/2016/12/how-to-get-youtube-api-key-tutorial/)
- [Generate an access token for Vimeo](https://developer.vimeo.com/api/guides/start#generate-access-token)

Bilibili feeds do not currently require official API credentials, but your feeds will still respect rate limits so keep
update intervals reasonable.

## ⚙️ Configuration

Create a configuration file (for example `config.toml`) describing the feeds you want to host. Use
[config.toml.example](./config.toml.example) as a reference for every supported key.

Minimal configuration:

```toml
[server]
port = 8080

[storage]
  [storage.local]
  # Don't change if you run Podsync via Docker
  data_dir = "/app/data/"

[tokens]
youtube = "PASTE YOUR API KEY HERE" # See config.toml.example for environment variables

[feeds]
  [feeds.ID1]
  url = "https://www.youtube.com/channel/UCxC5Ls6DwqV0e-CYcAKkExQ"
```

Behind a reverse proxy (nginx, Caddy, etc.) set the `hostname` so generated episode URLs point to your external host:

```toml
[server]
port = 8080
hostname = "https://my.test.host:4443"

[feeds]
  [feeds.ID1]
  # ...
```

The HTTP server keeps listening on `http://localhost:8080`, but enclosure links inside the RSS feed use the hostname you configured.

### 🌍 Environment Variables

Podsync supports the following environment variables for configuration and API keys:

| Variable Name                | Description                                                                               | Example Value(s)                  |
|-----------------------------|-------------------------------------------------------------------------------------------|-----------------------------------|
| `PODSYNC_CONFIG_PATH`       | Path to the configuration file (overrides `--config` CLI flag)                            | `/app/config.toml`                |
| `PODSYNC_YOUTUBE_API_KEY`   | YouTube API key(s), space-separated for rotation                                          | `key1` or `key1 key2 key3`        |
| `PODSYNC_VIMEO_API_KEY`     | Vimeo API key(s), space-separated for rotation                                            | `key1` or `key1 key2`             |
| `PODSYNC_SOUNDCLOUD_API_KEY`| SoundCloud API key(s), space-separated for rotation                                       | `soundcloud_key1 soundcloud_key2` |
| `PODSYNC_TWITCH_API_KEY`    | Twitch API credentials formatted as `CLIENT_ID:CLIENT_SECRET`, space-separated for multi  | `id1:secret1 id2:secret2`         |

### 🍪 Passing cookies to yt-dlp

Some sources (age-gated YouTube videos, members-only playlists, Bilibili streams behind login, CAPTCHA challenges, etc.) only work when `yt-dlp` can reuse a signed-in browser session. Podsync simply forwards anything you place in `feeds.<ID>.youtube_dl_args`, so you can pass cookies the same way the upstream [`yt-dlp` FAQ describes](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp).

Export a `cookies.txt` in Mozilla/Netscape format and point `yt-dlp` to it:  
  ```toml
  [feeds.members]
  url = "https://www.youtube.com/playlist?list=..."
  youtube_dl_args = ["--cookies", "/app/config/youtube-cookies.txt"]
  ```

Avoid the FAQ's `--cookies COOKIEFILE --cookies-from-browser BROWSER` shortcut because it skips the private/incognito session cookies that YouTube requires; use the recommended cookie-export browser extensions instead. The [`extractors` guide](https://github.com/yt-dlp/yt-dlp/wiki/extractors#exporting-youtube-cookies) shows the full workflow (sign in from an incognito window, export `youtube.com` cookies with the extension, close the window immediately). Keep the resulting file private and mount it into Docker containers alongside your `config.toml`.

## 🚀 Run it

### Build and run the binary

Make sure you have `config.toml` in place and that `storage.local.data_dir` points to a writable path on your system:

```bash
git clone https://github.com/yangtfu/podsync
cd podsync
make
./bin/podsync --config config.toml
```

### 🐛 Debugging

Use [Visual Studio Code](https://code.visualstudio.com/) with the official
[Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go). Choose **Run & Debug → Debug Podsync**; the repo already
contains `.vscode/launch.json` so you can step through feed updates locally.

### 🐳 Docker

```bash
docker pull ghcr.io/yangtfu/podsync:latest
docker run \
  -p 8080:8080 \
  -v "$(pwd)"/data:/app/data/ \
  -v "$(pwd)"/db:/app/db/ \
  -v "$(pwd)"/cookies:/app/cookies/ \
  -v "$(pwd)"/config.toml:/app/config.toml \
  ghcr.io/yangtfu/podsync:latest
```

Store any exported cookie files under `./cookies` so feeds can reference paths such as `/app/cookies/youtube-cookies.txt` inside `youtube_dl_args`.

### 🐳 Docker Compose

```bash
services:
  podsync:
    image: ghcr.io/yangtfu/podsync
    container_name: podsync
    volumes:
      - ./data:/app/data/
      - ./db:/app/db/
      - ./cookies:/app/cookies/
      - ./config.toml:/app/config.toml
    ports:
      - 8080:8080
```

As with the plain `docker run` example, drop Netscape-format cookie files into `./cookies` and reference them from your feed configuration.

## 📦 Releases

Push a git tag and CI takes care of building binaries, Docker images, and publishing release artifacts.

## 📄 License

This project (like the upstream mxpv/podsync) is licensed under the MIT License. See [LICENSE](LICENSE) for details.
