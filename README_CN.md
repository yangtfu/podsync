# Podsync
源自 [mxpv/podsync](https://github.com/mxpv/podsync)，主要增加了B站支持，具体配置方式可以参考源项目，下面内容是机翻。

## ✨ 功能特点

- 支持 YouTube、Vimeo、**Bilibili** 以及其他可用平台。
- 以订阅源为粒度，灵活控制音/视频类型、质量上限、封面、语言与分类。
- 通过 ffmpeg 进行 mp3 编码和后处理。
- 支持时区的 Cron 风格更新调度。
- 节目过滤（标题/时长）与自动清理（保留最近 _N_ 集）。
- 每次刷新后可触发可配置的 Webhook 或自动化脚本。
- 支持 OPML 导出，方便播客客户端导入。
- AWS 一键部署模板 + Docker/Compose 方案开箱即用。
- 可运行于 Windows、macOS、Linux、ARM 单板机以及容器中。
- 自动更新 yt-dlp 并轮换 API 密钥，降低限流风险。

## 📋 依赖

直接运行二进制（非 Docker 环境）时，需要在系统中安装以下工具：

- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
- [`ffmpeg`](https://ffmpeg.org/)
- [`go`](https://go.dev/)

macOS 用户可通过 Homebrew 安装：

```bash
brew install yt-dlp ffmpeg go
```

## 📖 文档

- [如何获取 Vimeo API Token](./docs/how_to_get_vimeo_token.md)
- [如何获取 YouTube API Key](./docs/how_to_get_youtube_api_key.md)
- [Podsync 在 QNAP NAS 上的部署指南](./docs/how_to_setup_podsync_on_qnap_nas.md)
- [使用 cron 调度更新](./docs/cron.md)

## 🌙 每夜构建

Nightly 版本会在 `main` 分支上每天午夜构建一次，便于抢先体验修复：

```bash
docker run -it --rm ghcr.io/yangtfu/podsync:nightly
```

## 🔑 访问令牌

针对你想抓取的每个平台，都需要准备对应的 API 凭据：

- [如何获取 YouTube API key](https://elfsight.com/blog/2016/12/how-to-get-youtube-api-key-tutorial/)
- [如何生成 Vimeo Access Token](https://developer.vimeo.com/api/guides/start#generate-access-token)

Bilibili 目前不需要官方 API 凭据，但订阅源依旧会受到频率限制，请合理设置更新时间。

## ⚙️ 配置

创建一个配置文件（例如 `config.toml`）描述你希望托管的订阅源。可参考 [config.toml.example](./config.toml.example) 获取所有可用键位。

最小示例：

```toml
[server]
port = 8080

[storage]
  [storage.local]
  # 若通过 Docker 运行无需修改
  data_dir = "/app/data/"

[tokens]
youtube = "PASTE YOUR API KEY HERE" # 环境变量示例请参见 config.toml.example

[feeds]
  [feeds.ID1]
  url = "https://www.youtube.com/channel/UCxC5Ls6DwqV0e-CYcAKkExQ"
```

若运行在反向代理（nginx、Caddy 等）之后，请设置 `hostname` 以便生成的节目链接指向对外域名：

```toml
[server]
port = 8080
hostname = "https://my.test.host:4443"

[feeds]
  [feeds.ID1]
  # ...
```

HTTP 服务器仍会监听 `http://localhost:8080`，但 RSS 内的 enclosure 链接将使用你配置的 hostname。

### 🌍 环境变量

Podsync 支持通过以下环境变量传递配置与 API Key：

| 变量名                      | 描述                                                                 | 示例值                           |
|---------------------------|----------------------------------------------------------------------|----------------------------------|
| `PODSYNC_CONFIG_PATH`     | 配置文件路径（优先级高于 `--config` CLI 参数）                        | `/app/config.toml`               |
| `PODSYNC_YOUTUBE_API_KEY` | YouTube API key，可空格分隔实现轮换                                  | `key1` 或 `key1 key2 key3`       |
| `PODSYNC_VIMEO_API_KEY`   | Vimeo API key，可空格分隔实现轮换                                    | `key1` 或 `key1 key2`            |
| `PODSYNC_SOUNDCLOUD_API_KEY`| SoundCloud API key，可空格分隔实现轮换                             | `soundcloud_key1 soundcloud_key2`|
| `PODSYNC_TWITCH_API_KEY`  | Twitch API 凭据，格式为 `CLIENT_ID:CLIENT_SECRET`，可空格分隔多个    | `id1:secret1 id2:secret2`        |

### 🍪 将 cookies 传递给 yt-dlp

某些来源（YouTube 年龄限制视频、会员播放列表、需要登录的 Bilibili 流、验证码挑战等）只有在 `yt-dlp` 能复用已登录浏览器会话时才可下载。Podsync 会原样转发 `feeds.<ID>.youtube_dl_args` 中的内容，因此你可以按上游 [`yt-dlp` FAQ](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp) 的方式传递 cookies。

导出 Mozilla/Netscape 格式的 `cookies.txt`，并让 `yt-dlp` 指向它：  
  ```toml
  [feeds.members]
  url = "https://www.youtube.com/playlist?list=..."
  youtube_dl_args = ["--cookies", "/app/config/youtube-cookies.txt"]
  ```

请避免使用 FAQ 中的 `--cookies COOKIEFILE --cookies-from-browser BROWSER` 速记方式，它不会包含 YouTube 所需的隐私/无痕会话 cookies；请改用推荐的浏览器扩展进行导出。详尽流程参见 [`extractors` 指南](https://github.com/yt-dlp/yt-dlp/wiki/extractors#exporting-youtube-cookies)：在无痕窗口登录，使用扩展开导出 `youtube.com` cookies，随后立即关闭窗口。务必妥善保管导出的文件，并在 Docker 容器内与 `config.toml` 一同挂载。

## 🚀 运行

### 构建并运行二进制

确保准备好 `config.toml`，并且 `storage.local.data_dir` 指向本机可写路径：

```bash
git clone https://github.com/yangtfu/podsync
cd podsync
make
./bin/podsync --config config.toml
```

### 🐛 调试

推荐使用 [Visual Studio Code](https://code.visualstudio.com/) 搭配官方
[Go 扩展](https://marketplace.visualstudio.com/items?itemName=golang.go)。选择 **Run & Debug → Debug Podsync**，仓库已提供 `.vscode/launch.json`，可直接在本地单步调试订阅源更新。

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

请将导出的 cookie 文件保存到 `./cookies`，这样订阅源即可在 `youtube_dl_args` 中引用 `/app/cookies/youtube-cookies.txt` 等路径。

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

与 `docker run` 示例相同，请将 Netscape 格式的 cookie 文件放入 `./cookies` 并在订阅源配置中引用。

## 📦 发布

推送 git tag 后，CI 会自动构建二进制、Docker 镜像并发布发行包。

## 📄 许可证

本项目（与上游 mxpv/podsync 一样）使用 MIT License。详见 [LICENSE](LICENSE)。
