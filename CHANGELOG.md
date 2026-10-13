# Changelog

## 0.1.0

### 中文

- 首个公开版本。
- 给电脑命名（如“Mac”“PC”），为每台显示器记录各台电脑接在哪个接口；面板显示每台显示器当前在显示哪台电脑，可以把任意显示器切到任意电脑。
- 切换后回读当前输入源，识别切换失败和被显示器弹回原输入的情况。
- 全局快捷键：在任何地方一键把显示器切到指定电脑。
- 支持 Apple 家庭（HomeKit），扫描二维码即可添加，可在“家庭” App 中切换显示器。
- 命令行（`list`、`switch`、`diagnose`、`version`）和 `moniflip://switch?to=…` 链接，方便接 Stream Deck、快捷指令和脚本。
- 应用内自动更新。
- 中文与英文界面：默认跟随系统，可在 设置 → 通用 → 语言 中切换。

### English

- First public release.
- Name your computers (for example "Mac" and "PC") and set, for each monitor, which port each computer is plugged into. The panel shows which computer each monitor is showing and switches any monitor to any computer.
- Reads the input source back after switching, so failed switches and monitors that bounce back to the previous input are detected.
- Global shortcuts: flip the monitors to a computer from anywhere.
- Apple Home (HomeKit) support: add Moniflip by scanning a QR code and switch monitors from the Home app.
- Command line (`list`, `switch`, `diagnose`, `version`) and `moniflip://switch?to=…` links, for Stream Deck, Shortcuts and scripts.
- In-app automatic updates.
- Chinese and English interface: follows the system language, or choose one in Settings → General → Language.
