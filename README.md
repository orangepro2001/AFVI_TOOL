# AFVI TOOLS

AFVI 检测机的辅助工具,有两个功能:

- **Model Copy**:型号的复制 / 复制并重命名 / 重命名(单台或整组),带型号列表和搜索框。
- **Export Data To Excel**:目前只有入口(页签、菜单、核心接口 `POST /v1/export/excel` 返回 501),分析与导出尚未实现。

同时它是 **C# + Go 做 Windows 2000 风格跨平台桌面应用的可行性验证**:GUI 是 C#(Avalonia + Classic 主题,Windows 2000 外观),
业务核心是 Go(独立进程,回环 HTTP + 令牌)。Linux 桌面上开发和测试,Windows 上发布。

界面语言:**한국어(默认)**、简体中文、繁體中文、English。菜单 `查看 > 语言` 切换,即时生效并记住。

## 机台

默认机台 **AFVI 13**(首次启动自动写入配置):FM1 `192.168.1.61`、FM2 `192.168.1.62`、BM `192.168.1.63`,
读 `\\主机\PxRepository` 与 `\\主机\PxInventory`,账号 `pixel`(空密码)。可在 `文件 > 机台配置` 添加其他机台。
一个型号在一个节点上由 4 类条目组成(仓库目录、`.his`、`INSPECT_SPEC\N-NN`、`LIGHT_SPEC\N`),大小写不敏感,
详见 [docs/NAMING.md](docs/NAMING.md)。

### 复制 / 重命名规则(`core/internal/policy`)

| 操作 | 允许 |
| --- | --- |
| 同机台 FM1 ↔ FM2 | 是 |
| 机台之间 | 只能整组(FM1→FM1、FM2→FM2、BM→BM) |
| FM ↔ BM | 否 |
| BM | 只能复制到其他机台的 BM,或自己重命名 |
| 复制并重命名 | 复制时填新名称(同节点内也可,BM 除外) |
| 重命名 | 单个节点,或整组三台同时;失败时全部回滚 |

**Model Copy 是原子操作**:先检查并读完源 → 删除目标(大小写不敏感的同名型号的全部条目)→ 复制 → 校验(文件列表与大小)。
复制失败时清理不完整数据,不留半个型号。点确认前会显示完整计划(哪些要删、哪些要复制)。

## 开发阶段的安全规则

**网络路径只读**。核心在文件系统层(`core/internal/vfs`)强制:`\\...` 路径在 Linux 上走只读 SMB 客户端,
在 Windows 上以只读方式打开,任何写操作都会返回 `ErrReadOnly`;计划里会标出"只读,无法作为目标"。
要试复制,用本地机台:

```bash
scripts/dev-local.sh           # 加一个可写机台 "LOCAL TEST"(.local-mirror/LOCAL 下的 FM1/FM2/BM)
scripts/run-dev.sh             # 启动界面;源选 AFVI 13 / FM1,选型号,"整组复制"到 LOCAL TEST(只读取网络)
```

放开写入需要有意为之:配置里 `settings.allowNetworkWrite=true` 或核心参数 `-allow-network-write`,发布前才做。

## 构建、测试、运行

需要 Go(`core/go.mod`)和 .NET 10 SDK(没有的话:`dotnet-install.sh --channel 10.0 --install-dir ~/.dotnet`)。

```bash
scripts/build.sh               # gofmt + vet + Go 测试 -> bin/afvi-core -> 编译 GUI -> GUI 测试
scripts/build.sh --platform win|linux|mac|all   # 另外交叉构建原生程序到 ./build/win、./build/linux、./build/mac
                               # 每个目录只有一个文件 AfviTools[.exe](约 25 MB,免安装):.NET 运行时、原生库和 Go 核心都在里面,
                               # 压缩 + 裁剪;首次启动把原生库和核心解压到用户数据目录,对用户透明。mac 默认 Apple Silicon(Intel 用 MAC_ARCH=x64)。
                               # 构建后会在 Linux 上对打包出的程序跑 --selftest(四种语言 × 所有窗口);Windows/mac 版未在对应系统上运行过
scripts/run-dev.sh [--lang=en] [--fontsize=16]
scripts/screenshot.sh shots/main.png --lang=ko    # 在 Xvfb 里渲染窗口,看外观用
```

测试从不访问网络:Go 用临时目录搭出 6 个节点,C# 起真实的 `afvi-core` 进程对本地机台操作;
`dotnet test` 还会用 Avalonia Headless + Skia 把各窗口渲染成 `shots/*.png`。

## 结构

```
core/                Go 核心(afvi-core)
  cmd/afvi-core        入口:回环 HTTP,启动时在 stdout 打印端口和令牌,stdin 关闭即退出
  internal/vfs         文件系统抽象:本地/UNC(只读保护)、只读 SMB
  internal/model       型号命名、4 类条目的归并、.his 改名
  internal/policy      复制/重命名规则
  internal/machine     机台配置(config.json,默认 AFVI 13)
  internal/ops         计划(plan)与执行、回滚、任务管理
  internal/i18n        核心的四语言包(计划、警告、拒绝原因)
  internal/server      HTTP/JSON 接口
app/                 C# 界面
  src/AfviTools        Avalonia + Classic.Avalonia.Theme(Windows 2000 风格)、MVVM
  src/AfviTools/Assets/lang/*.json   界面的四语言包(内嵌);运行目录下 lang/*.json 可新增/覆盖语言
  tests/AfviTools.Tests
scripts/  docs/  deploy/  .github/workflows/ci.yml
```

接口和进程模型见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 持续集成 / 部署

`.github/workflows/ci.yml`:push main 与 PR 时跑 Go(Linux + Windows)、GUI 测试、Windows 打包。`deploy` 任务
是**已禁用的占位**,交付方式见 [deploy/README.md](deploy/README.md)。
