# Stand Reminder

一个使用 Go 编写的轻量级 Windows 久坐提醒工具。

它会在后台常驻系统托盘，检测鼠标键盘活动，并结合系统媒体播放状态判断你是否仍在使用电脑，累计连续使用时间，并在达到提醒阈值后发送 Windows 系统通知。程序同时提供一个本地网页控制台，方便查看状态、修改配置和测试通知。

## 功能特性

- Windows 托盘常驻运行，启动后不弹黑色终端窗口
- 检测鼠标键盘活动，并在视频/媒体播放时避免误判为空闲
- 达到提醒时发送 Windows 系统通知
- 支持本地网页控制台
- 支持修改提醒分钟数、空闲重置分钟数、检测间隔、通知标题和通知内容
- 支持手动暂停、恢复计时、测试通知
- 支持“我已起身活动”模式，并在页面中显示 10 分钟活动倒计时
- 支持中英文界面切换，默认中文
- GitHub Actions 可自动构建 Windows 版本，并在打标签时发布 Release

## 工作方式

程序会定期读取 Windows 最近一次键盘或鼠标输入时间，并额外检查系统是否有媒体正在播放，然后按下面的规则运行：

- 当用户持续活动时，累计久坐计时
- 当没有键鼠输入但系统媒体仍在播放时，仍视为正在使用电脑
- 当空闲时间达到 `idle_reset_minutes` 后，当前累计时间会被重置
- 当累计时间达到 `remind_after_minutes` 后，发送一次系统通知，并开始下一轮计时
- 如果手动点击“暂停”，程序会停止累计和提醒
- 如果点击“我已起身活动”，程序会停止计时，并进入 10 分钟活动休息状态
- 活动休息结束后，不会自动恢复，需要手动点击“开始 / 恢复”

## 控制中心

程序启动后会在本机开启一个网页控制台：

`http://127.0.0.1:47831`

控制中心支持：

- 查看当前状态
- 查看已累计时间、剩余时间、空闲时间、媒体播放状态
- 保存提醒设置
- 测试通知
- 暂停提醒
- 开始 / 恢复计时
- 进入活动休息模式
- 中英文切换

通常可以通过单击系统托盘图标打开控制中心。

## 配置文件

程序默认读取可执行文件同目录下的 `config.json`。

示例：

```json
{
  "remind_after_minutes": 45,
  "idle_reset_minutes": 5,
  "check_interval_seconds": 5,
  "notification_title": "Stand Reminder",
  "notification_message": "You've been active for a while. Time to stand up and stretch."
}
```

字段说明：

- `remind_after_minutes`：提醒间隔，单位分钟
- `idle_reset_minutes`：空闲多久后重置当前计时，单位分钟
- `check_interval_seconds`：检测输入状态的轮询间隔，单位秒
- `notification_title`：通知标题
- `notification_message`：通知内容

## 本地运行

1. 安装 Go 1.22 或更高版本
2. 在项目目录执行：

```powershell
go build -ldflags='-H windowsgui' -o stand-reminder.exe .
```

3. 运行：

```powershell
.\stand-reminder.exe
```

4. 程序启动后会进入系统托盘
5. 单击托盘图标打开控制中心

## 开发调试

如果你只是临时调试，也可以直接运行：

```powershell
go run .
```

但正式使用更推荐构建后运行 `stand-reminder.exe`。

## GitHub Actions

仓库内置了 Windows 构建工作流：

- 推送到分支时会自动构建 Windows 可执行文件
- 推送形如 `v*` 的标签时，会自动生成 Release
- Release 默认上传打包后的 zip 文件

## 项目结构

```text
.
├─ main.go
├─ config.json
├─ .github/workflows/build.yml
└─ internal/
   ├─ activity/    # Windows 活动检测
   ├─ app/         # 应用状态与控制逻辑
   ├─ config/      # 配置加载与保存
   ├─ notify/      # Windows 通知
   ├─ reminder/    # 久坐计时逻辑
   ├─ tray/        # 系统托盘
   └─ web/         # 本地网页控制台
```

## 说明

当前版本仅支持 Windows。
