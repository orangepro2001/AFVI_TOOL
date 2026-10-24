# AFVI TOOLS

AFVI 检测机的辅助工具(**仅 Windows**,在 Linux 上交叉编译):

- **Model Copy**:型号的复制 / 复制并重命名 / 重命名(单台或整组),带型号列表和搜索框。
- **Export Data To Excel**:目前只有入口,分析与导出尚未实现。

界面是 C 语言写的原生 Win32 程序,所有控件自绘成 **Windows 2000 风格**;业务核心是 Go,通过 C 接口(`afvi_init / afvi_call / afvi_free`)被调用。
界面语言:**한국어(默认)**、简体中文、繁體中文、English,菜单里即时切换。

交付物:`build/native-win/AfviTools.exe`(静态链接,约 5.5 MB,单文件,只依赖 msvcrt/kernel32/user32/gdi32/shell32,免安装)。

## 机台

默认机台 **AFVI 13**(首次启动写入配置):FM1 `192.168.1.61`、FM2 `192.168.1.62`、BM `192.168.1.63`,读 `\\主机\PxRepository` 与 `\\主机\PxInventory`,账号 `pixel`(空密码)。
一个型号由四类条目组成(仓库目录、`.his`、`INSPECT_SPEC\N-NN`、`LIGHT_SPEC\N`),大小写不敏感,详见 [docs/NAMING.md](docs/NAMING.md)。

| 操作 | 允许 |
| --- | --- |
| 同机台 FM1 ↔ FM2 | 是 |
| 机台之间 | 只能整组(FM1→FM1、FM2→FM2、BM→BM);BM 可单独复制到其他机台的 BM |
| FM ↔ BM | 否 |
| 重命名 | 单个节点,或整组三台同时;失败时全部回滚 |

Model Copy 是原子操作:读完并检查源 → 删除目标 → 复制 → 校验;失败时清理不完整数据。执行前显示完整计划。规则在 `core/internal/policy`。

## 开发阶段的安全规则

**网络路径只读**,由文件系统层(`core/internal/vfs`)强制。试复制用本地机台(`scripts/dev-local.sh` 添加 `LOCAL TEST`)或临时目录。放开写入需要有意为之(配置 `settings.allowNetworkWrite` 或 `-allow-network-write`),发布前才做。

## 构建与测试

```bash
scripts/build.sh               # gofmt + vet + Go 测试,再用 mingw-w64 交叉编译 -> build/native-win/
scripts/build.sh --go-only     # 只做 Go 检查(不需要 mingw)
scripts/win32.sh               # 只构建应用:动态/静态两种链接,并在 Linux 上渲染预览图到 shots/
```

需要:Go、`x86_64-w64-mingw32-gcc`(apt: `gcc-mingw-w64-x86-64`)、python3、bc。测试不访问网络。Linux 上怎么测、哪些必须在 Windows 上冒烟,见 [docs/LINUX_TESTING.md](docs/LINUX_TESTING.md)。

## 结构

```
core/    Go 核心:vfs(只读保护、只读 SMB)、model、policy、machine、ops(计划/执行/回滚)、i18n、service、capi(C 接口的纯 Go 部分)
         cmd/afvi-core-dll   cgo 导出 afvi_*(c-shared 与 c-archive)    cmd/afvi-core + internal/server  可选的回环 HTTP 传输
libw2k/  Windows 2000 控件库(C99):w2k.h/w2k_draw.c(皮肤与基础绘制)、w2k_ctl.c(按钮、列表、菜单、页签等控件)、w2k_gfx.h(绘制接口)、w2k_gdi.c(Windows 后端)、w2k_png.c(Linux 后端)、demo/ 与 tests/(演示与快照测试)
gui/     C 界面:app.c(应用,调用 libw2k)、win_main.c(Win32 宿主)、preview/(Linux 测试宿主)、lang/*.json(四种语言)
docs/    ARCHITECTURE、NAMING、LINUX_TESTING、COMPONENT_LIBRARY、STACK_COMPARISON、validation/(各方案验证报告)
scripts/ build.sh  win32.sh  dev-local.sh
```

## 历史与其他方案

Avalonia 版在标签 `v0.1.0-avalonia`;Wails、Avalonia AOT、Lazarus 的验证在分支 `wails-rpc`、`avalonia-aot`、`lazarus-lcl`,结论汇总见 [docs/STACK_COMPARISON.md](docs/STACK_COMPARISON.md)。
