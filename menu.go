package main

import (
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"

	"my-weread-desk/internal/app"
)

// buildAppMenu creates the native macOS menu bar. Wails v2.7.1 exposes the
// complete App and Edit menu roles, but not the individual File/View/Window
// roles, so the app-specific menus are assembled here.
//
// The menu bar is intentionally macOS-only. Windows keeps the custom HTML
// title bar and the tray menu.
func buildAppMenu(application *app.App) *menu.Menu {
	if runtime.GOOS != "darwin" {
		return nil
	}

	action := func(name string) menu.Callback {
		return func(*menu.CallbackData) {
			application.MenuAction(name)
		}
	}

	file := menu.NewMenu()
	file.AddText("同步书架", keys.CmdOrCtrl("r"), action("sync"))
	file.AddSeparator()
	file.AddText("打开数据目录", keys.Combo("d", keys.OptionOrAltKey, keys.CmdOrCtrlKey), func(*menu.CallbackData) {
		application.OpenDataDir()
	})

	edit := menu.EditMenu()
	edit.SetLabel("编辑")

	nav := menu.NewMenu()
	nav.AddText("书架", keys.CmdOrCtrl("1"), action("shelf"))
	nav.AddText("搜索书城", keys.CmdOrCtrl("2"), action("search"))
	nav.AddText("统计", keys.CmdOrCtrl("3"), action("stats"))
	nav.AddText("助手", keys.CmdOrCtrl("4"), action("assistant"))
	nav.AddText("穿越", keys.CmdOrCtrl("5"), action("traversal"))
	nav.AddSeparator()
	nav.AddText("设置…", keys.CmdOrCtrl(","), action("settings"))

	view := menu.NewMenu()
	view.AddText("禅模式", keys.Combo("z", keys.ControlKey, keys.OptionOrAltKey), action("zen"))
	view.AddText("打字模式", keys.Combo("d", keys.ControlKey, keys.OptionOrAltKey), action("typing"))
	view.AddSeparator()
	reading := view.AddSubmenu("阅读方式")
	reading.AddText("左右翻页", nil, action("mode:page"))
	reading.AddText("上下滚动", nil, action("mode:scroll"))
	materials := view.AddSubmenu("界面材质")
	materials.AddText("纸页", nil, action("carrier:paper"))
	materials.AddText("竹简", nil, action("carrier:bamboo"))
	materials.AddText("玉简", nil, action("carrier:jade"))
	materials.AddText("圆珠", nil, action("carrier:orb"))

	window := menu.NewMenu()
	window.AddText("最小化", keys.CmdOrCtrl("m"), func(*menu.CallbackData) {
		application.MinimiseWindow()
	})
	window.AddText("最大化/还原", nil, func(*menu.CallbackData) {
		application.MaximiseWindow()
	})
	window.AddSeparator()
	window.AddText("窗口置顶", keys.Combo("t", keys.ControlKey, keys.OptionOrAltKey), action("toggleTop"))

	help := menu.NewMenu()
	help.AddText("键盘快捷键", nil, func(*menu.CallbackData) {
		application.ShowShortcuts()
	})
	help.AddText("打开数据目录", nil, func(*menu.CallbackData) {
		application.OpenDataDir()
	})

	return menu.NewMenuFromItems(
		menu.AppMenu(),
		menu.SubMenu("文件", file),
		edit,
		menu.SubMenu("前往", nav),
		menu.SubMenu("显示", view),
		menu.SubMenu("窗口", window),
		menu.SubMenu("帮助", help),
	)
}
