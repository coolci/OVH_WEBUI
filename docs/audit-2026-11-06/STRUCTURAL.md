# 声明与UI基本封装检查

完整AST遍历确认下面文件的本地调用类别。声明/类型/字典不含独立执行函数；UI封装仅调用ref转发、样式组合和声明式子组件，实际业务事件由调用方传入。该检查覆盖结构和副作用入口，不等同于每个像素或可访问性状态都已人工验证。包含自有状态、effect、数据转换或HTML注入的组件不自动归类，另行审读。

| 文件 | 类别 | 调用集合 |
|---|---|---|
| [src/components/server-control/reinstall/constants.ts](../../src/components/server-control/reinstall/constants.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/components/telegram-order/types.ts](../../src/components/telegram-order/types.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/components/ui/accordion.tsx](../../src/components/ui/accordion.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/alert-dialog.tsx](../../src/components/ui/alert-dialog.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn, buttonVariants |
| [src/components/ui/alert.tsx](../../src/components/ui/alert.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cva, React.forwardRef, cn, alertVariants |
| [src/components/ui/aspect-ratio.tsx](../../src/components/ui/aspect-ratio.tsx) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/components/ui/avatar.tsx](../../src/components/ui/avatar.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/badge.tsx](../../src/components/ui/badge.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cva, cn, badgeVariants |
| [src/components/ui/breadcrumb.tsx](../../src/components/ui/breadcrumb.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/button.tsx](../../src/components/ui/button.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cva, React.forwardRef, cn, buttonVariants |
| [src/components/ui/calendar.tsx](../../src/components/ui/calendar.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cn, buttonVariants |
| [src/components/ui/card.tsx](../../src/components/ui/card.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/checkbox.tsx](../../src/components/ui/checkbox.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/collapsible.tsx](../../src/components/ui/collapsible.tsx) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/components/ui/command.tsx](../../src/components/ui/command.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/context-menu.tsx](../../src/components/ui/context-menu.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/dialog.tsx](../../src/components/ui/dialog.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/drawer.tsx](../../src/components/ui/drawer.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/dropdown-menu.tsx](../../src/components/ui/dropdown-menu.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/hover-card.tsx](../../src/components/ui/hover-card.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/input.tsx](../../src/components/ui/input.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/label.tsx](../../src/components/ui/label.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cva, React.forwardRef, cn, labelVariants |
| [src/components/ui/menubar.tsx](../../src/components/ui/menubar.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/navigation-menu.tsx](../../src/components/ui/navigation-menu.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn, cva, navigationMenuTriggerStyle |
| [src/components/ui/pagination.tsx](../../src/components/ui/pagination.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cn, React.forwardRef, buttonVariants |
| [src/components/ui/popover.tsx](../../src/components/ui/popover.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/progress.tsx](../../src/components/ui/progress.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/radio-group.tsx](../../src/components/ui/radio-group.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/resizable.tsx](../../src/components/ui/resizable.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cn |
| [src/components/ui/scroll-area.tsx](../../src/components/ui/scroll-area.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/separator.tsx](../../src/components/ui/separator.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/sheet.tsx](../../src/components/ui/sheet.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn, cva, sheetVariants |
| [src/components/ui/skeleton.tsx](../../src/components/ui/skeleton.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cn |
| [src/components/ui/slider.tsx](../../src/components/ui/slider.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/stat-card.tsx](../../src/components/ui/stat-card.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | forwardRef, cn |
| [src/components/ui/switch.tsx](../../src/components/ui/switch.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/table.tsx](../../src/components/ui/table.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/tabs.tsx](../../src/components/ui/tabs.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/terminal-card.tsx](../../src/components/ui/terminal-card.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | forwardRef, cn |
| [src/components/ui/textarea.tsx](../../src/components/ui/textarea.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/toast.tsx](../../src/components/ui/toast.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn, cva, toastVariants |
| [src/components/ui/toggle.tsx](../../src/components/ui/toggle.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | cva, React.forwardRef, cn, toggleVariants |
| [src/components/ui/tooltip.tsx](../../src/components/ui/tooltip.tsx) | UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方 | React.forwardRef, cn |
| [src/components/ui/use-toast.ts](../../src/components/ui/use-toast.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/ovh/server-control/types.ts](../../src/hooks/ovh/server-control/types.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/ovh/use-server-control.ts](../../src/hooks/ovh/use-server-control.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-account.ts](../../src/hooks/use-account.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-accounts.ts](../../src/hooks/use-accounts.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-active-account.ts](../../src/hooks/use-active-account.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-app-devices.ts](../../src/hooks/use-app-devices.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-availability.ts](../../src/hooks/use-availability.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-hide-ip.ts](../../src/hooks/use-hide-ip.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-history.ts](../../src/hooks/use-history.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-logs.ts](../../src/hooks/use-logs.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-monitor.ts](../../src/hooks/use-monitor.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-mrtg.ts](../../src/hooks/use-mrtg.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-notify-channels.ts](../../src/hooks/use-notify-channels.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-queue.ts](../../src/hooks/use-queue.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-server-aliases.ts](../../src/hooks/use-server-aliases.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-server-control.ts](../../src/hooks/use-server-control.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-servers.ts](../../src/hooks/use-servers.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-settings.ts](../../src/hooks/use-settings.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-stats.ts](../../src/hooks/use-stats.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-system-metrics.ts](../../src/hooks/use-system-metrics.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-telegram.ts](../../src/hooks/use-telegram.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-vps-control.ts](../../src/hooks/use-vps-control.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/hooks/use-vps-monitor.ts](../../src/hooks/use-vps-monitor.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/api-en.translated.ts](../../src/i18n/locales/api-en.translated.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/api-zh.generated.ts](../../src/i18n/locales/api-zh.generated.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/en.ts](../../src/i18n/locales/en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/common-layer.en.ts](../../src/i18n/locales/modules/common-layer.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/common-layer.zh.ts](../../src/i18n/locales/modules/common-layer.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/ctrl-dialogs.en.ts](../../src/i18n/locales/modules/ctrl-dialogs.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/ctrl-dialogs.zh.ts](../../src/i18n/locales/modules/ctrl-dialogs.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/live-chat.en.ts](../../src/i18n/locales/modules/live-chat.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/live-chat.zh.ts](../../src/i18n/locales/modules/live-chat.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/maint-dialogs.en.ts](../../src/i18n/locales/modules/maint-dialogs.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/maint-dialogs.zh.ts](../../src/i18n/locales/modules/maint-dialogs.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/server-safety.en.ts](../../src/i18n/locales/modules/server-safety.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/server-safety.zh.ts](../../src/i18n/locales/modules/server-safety.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/settings-pages.en.ts](../../src/i18n/locales/modules/settings-pages.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/settings-pages.zh.ts](../../src/i18n/locales/modules/settings-pages.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/snipe-pages.en.ts](../../src/i18n/locales/modules/snipe-pages.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/snipe-pages.zh.ts](../../src/i18n/locales/modules/snipe-pages.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/vps-dialogs.en.ts](../../src/i18n/locales/modules/vps-dialogs.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/vps-dialogs.zh.ts](../../src/i18n/locales/modules/vps-dialogs.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/watch-pages.en.ts](../../src/i18n/locales/modules/watch-pages.en.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/modules/watch-pages.zh.ts](../../src/i18n/locales/modules/watch-pages.zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/i18n/locales/zh.ts](../../src/i18n/locales/zh.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/lib/api-client.ts](../../src/lib/api-client.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/lib/types.ts](../../src/lib/types.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
| [src/vite-env.d.ts](../../src/vite-env.d.ts) | 声明、类型、资源或转导出；无本地执行函数 |  |
