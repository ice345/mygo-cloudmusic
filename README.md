# 网易云音乐 · MyGo

用 Go 和 [MyGo](https://github.com/egoist/mygo) 写的第三方网易云音乐桌面客户端，原生界面，无 WebView。

## 功能

- 扫码登录，登录态保存在系统凭据存储中
- 每日推荐、推荐歌单、排行榜、我的歌单
- 搜索单曲、歌手、专辑、用户与播客，输入时显示搜索建议，保留搜索历史
- 歌手页的热门与最新歌曲、专辑与动态，关注歌手；专辑页与用户页
- 播客：浏览与收听节目，订阅播客；在本机把节目转写成字幕（Qwen3-ASR），像歌词一样随播放滚动
- 私人 FM（推荐 / 熟悉 / 探索）与心动模式
- 边下边播，支持标准 / 较高 / 极高 / 无损（FLAC）/ Hi-Res 音质
- 听过的歌曲缓存在本机，再次播放无需等待
- 音量均衡，暂停与继续时淡出淡入，拖动进度无杂音
- 退出后再打开，从上次的播放队列与进度继续
- 逐字歌词与翻译，正在播放页的「律动」视图与歌曲评论
- 刘海播放器：在屏幕顶部刘海处显示迷你播放器，悬停展开（在设置中开启）
- 喜欢歌曲、播放队列、随机与循环播放
- 浅色 / 深色 / 跟随系统
- 音源插件：安装插件后，可以在边栏顶部切换到别的音乐来源（见下）

## 音源插件

除了内置的网易云音乐，还可以安装插件，把别的音乐来源接进来。插件用 JavaScript 写成，与 [mygo-music](https://github.com/egoist/mygo-music) 的插件通用，可以在[插件列表](https://mygo-music.egoist.dev/plugins)里找到。装了插件后，边栏顶部出现音源按钮，切换过去就是这个来源自己的首页、搜索、歌单与账号，播放也由它提供。不装插件时，一切照旧。

**安装**：设置 → 插件 → ＋，可以从 GitHub（`owner/repo` 或仓库的网址）、本机的文件夹或 zip 安装，或「浏览插件…」去挑一个。网页里的 `mygo-music://install-plugin?repo=owner/repo` 链接只会打开安装窗口，点「安装」之后才会下载。装好立即可用，不用重启。每个插件在设置里有自己的分组：设置项、账号、检查更新、更新、移除（移到废纸篓或回收站，它保存的设置留着，以备重装）。

**权限**：插件运行在你的电脑上，只拥有你允许的权限，装之前应用会逐项告诉你。

- 它可以访问的网站写在 `plugin.json` 里，访问其他网站（包括被重定向过去的）会被拒绝；
- 想运行本机程序的插件，装好后还要你在它的分组里单独打开「允许启动程序」，应用会列出那些程序。**程序能做你能做的一切**，只允许你信任的插件；
- 插件更新后要求更多权限时会再次询问；新版本新增了要运行的程序时，之前的允许会取消，需要重新打开；
- 插件保存的设置与登录信息在用户数据目录的 `plugin-data` 里，移除插件时不会删。

**只安装你信任的插件。** 插件的代码不由本应用审查；上面的权限是它能做的事的边界，不是对它的担保。

**自己写插件**：插件接口见 mygo-music 的 README，本应用在它之上的补充和各阶段的记录在 [`docs/plugins.md`](docs/plugins.md)。`internal/plugin/testdata/demo` 是上游的演示插件，用到了插件能做的每一样事，也可以装来试用。

## 构建

需要 Go 1.27+。

```bash
go tool mygo dev     # 开发模式运行
go tool mygo build   # 打包到 build/
go test ./...        # 运行测试
```

## 致谢

- [MyGo](https://github.com/egoist/mygo)：原生 UI 与应用框架
- [NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi)：eapi 加密与登录流程的参考
- [folia-major](https://github.com/chthollyphile/folia-major)：歌词动画效果的参考
- [BudouX](https://github.com/google/budoux)：日文与中文歌词的断行（模型以 Apache-2.0 授权，见 `internal/linebreak/models`）
- [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx)、[Qwen3-ASR](https://github.com/QwenLM/Qwen3-ASR) 与 [Silero VAD](https://github.com/snakers4/silero-vad)：本机语音识别
- [moejs](https://github.com/MoeForge/moejs)：运行音源插件的 JavaScript 引擎（Apache-2.0）
- [go-aac](https://github.com/tphakala/go-aac)：AAC 解码（LGPL-2.1 或更新版本，移植自 FFmpeg；本项目以 AGPL-3.0 公开全部源码，可自行重新编译）
- [oggvorbis](https://github.com/jfreymuth/oggvorbis)：Ogg Vorbis 解码（MIT）

本项目与网易云音乐官方无关，音乐版权归各权利人所有。

## 许可证

[AGPL-3.0](LICENSE)
