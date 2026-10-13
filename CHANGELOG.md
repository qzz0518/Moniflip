# Changelog

## 0.1.1

### 中文

- Apple 家庭可以选择显示为 **开关**（新的默认）或 **电视**：在 设置 → Apple 家庭 → 在“家庭”中显示为 里选择。开关是每台电脑一个，打开哪个就切到哪台，场景、自动化和配件按钮都能用；只有两台电脑时，关掉一个也会切到另一台。
- 在 0.1.0 里已经打开 Apple 家庭的，升级后仍是电视。改成开关后，要在“家庭” App 里删掉原来的 Moniflip 再重新添加。
- “家庭”里的状态每 30 秒与显示器同步一次：用显示器按键或另一台电脑切换后，“家庭”最多 30 秒就会跟上，按钮和自动化不会一直按不动。
- 修复：改了电脑列表、关掉再打开 Apple 家庭后，“家庭”有时很久都找不到 Moniflip。

### English

- Apple Home: choose to show Moniflip as **Switches** (the new default) or a **TV**, in Settings → Apple Home → Show in Home as. Switches give each computer its own switch — turn one on to flip the monitors to that computer — and work in scenes, automations and with accessory buttons. With exactly two computers, turning a switch off also flips to the other one.
- If Apple Home was on in 0.1.0, Moniflip stays a TV after the update. To use switches, change it in Settings, then remove the old Moniflip in the Home app and add it again.
- Apple Home re-reads the monitors every 30 seconds: after a switch made with the monitor's buttons or from the other computer, Home catches up within 30 seconds, so buttons and automations don't stay stuck.
- Fixed: after changing the computers or turning Apple Home off and on, the Home app could take a long time to find Moniflip.

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
