package main

// Moniflip 面板的数据接口（v2）。真实程序把检测结果和设置填进来，view*.go 只负责画和回调。
// 约定：所有回调在主线程调用，调用前判空；视图可以直接改带指针语义的字段（Enabled、Ports 等）。

// monitorRow 是一台显示器。
type monitorRow struct {
	ID       string   // 稳定 ID（含序列号），作 c.Key 用
	Name     string   // 如 "DELL G3223Q"
	Portrait bool     // 竖屏
	Online   bool     // 检测到了、能用 DDC 控制
	Enabled  bool     // 参与“全部切到 X”
	Current  int      // 当前显示的是哪台电脑：Computers 的下标；-1 未知或不属于任何电脑
	Ports    []string // 与 Computers 一一对应：该电脑接在这台显示器的哪个口（"DP"、"HDMI 1"…；"" 未设置）
	Options  []string // 这台显示器可选的接口名（来自 capabilities，可含用户自定义的 "0x31" 这类）
	Busy     bool     // 正在切换这台
}

type panel struct {
	Self      int      // 本机是 Computers 的第几个
	Computers []string // 电脑名，如 ["Mac", "PC"]；至少 2 台，通常正好 2 台
	Rows      []monitorRow

	Loading     bool   // 首次检测或手动刷新中
	Switching   bool   // 有切换在进行
	Notice      string // 上次操作结果，如 "已切到 PC"；出错时为原因
	NoticeError bool

	Page int // 0 = 主页，1 = 设置页（视图可以直接改它来切页）

	// 设置
	OpenAtLogin   bool
	Language      string   // 界面语言："" 跟随系统，"zh"、"en"
	Hotkeys       []string // 与 Computers 对应的全局快捷键（mygo accelerator 语法，如 "Ctrl+Alt+Cmd+P"；"" 无）
	HomeKit       bool     // 作为 Apple 家庭配件
	HomeKitAs     string   // 在“家庭”里的样子："switch" 每台电脑一个开关（空也当它），"tv" 一台电视
	HomeKitCode   string   // 配对码，如 "123-45-678"（HomeKit 打开时显示）
	HomeKitSetup  string   // 配对二维码的内容（X-HM://…），“家庭”App 扫它添加
	HomeKitPaired bool
	Version       string // 如 "0.1.0"

	// 动作
	SwitchAll       func(to int)      // 把勾选的显示器切到第 to 台电脑
	SwitchOne       func(row, to int) // 把第 row 台显示器切到第 to 台电脑
	Refresh         func()            // 重新检测
	Remove          func(row int)     // 移除离线显示器的记录
	Changed         func()            // 视图改了 Enabled/Ports/Computers 名称后调用，用于保存
	SetLogin        func(on bool)
	SetLanguage     func(language string)    // 改界面语言（取值同 Language），立即生效
	SetHotkey       func(to int, acc string) // acc 为 "" 表示清除
	SetHomeKit      func(on bool)
	SetHomeKitAs    func(as string) // 换样子；已配对时先确认（要重新添加），所以视图不直接改 HomeKitAs
	RenameComputer  func(i int, name string)
	AddComputer     func()
	RemoveComputer  func(i int)
	CheckUpdate     func()
	OpenSettings    func()            // 打开独立的设置窗口；为 nil 时（测试、设计稿）在面板里翻到设置页
	CopyDiagnostics func(done func()) // 复制诊断信息到剪贴板（反馈兼容性问题用）
	Open            func(url string)  // 用浏览器打开链接
	Quit            func()
}
