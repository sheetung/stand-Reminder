# Stand Reminder

Stand Reminder 是一个使用 Go 编写的轻量级 Windows 久坐提醒工具。

它会常驻系统托盘，检测鼠标和键盘活动，在你连续使用电脑达到设定时长后发送系统通知；同时提供一个本地网页控制中心，用来查看当前状态、调整提醒参数、管理休息流程，以及查看统计图表。

当前版本已经切换为 SQLite 本地存储，程序首次启动时会在可执行文件同目录下创建 `stand-reminder.db`，后续配置和统计数据都写入数据库。

## UI 展示

### 控制中心首页

![控制中心首页](figs/1.png)

### 统计视图

![统计视图 1](figs/2-0.png)

![统计视图 2](figs/2-1.png)

![统计视图 3](figs/2-3.png)

### 其他界面

![其他界面](figs/3.png)

## 功能说明

- Windows 托盘常驻运行，启动后不弹黑色终端窗口
- 检测键盘与鼠标活动，按连续使用时长进行久坐提醒
- 到达阈值后发送 Windows 系统通知，点击通知可打开控制中心
- 提供本地网页控制中心，默认地址为 `http://127.0.0.1:47831`
- 支持修改提醒分钟数、空闲重置分钟数、检测间隔、通知标题和通知内容
- 支持手动 `暂停`、`开始 / 恢复`、`我已起身活动`
- “我已起身活动”会停止当前计时，并在前端显示一个 10 分钟活动倒计时
- 支持中英文界面切换，默认中文
- 支持今天、7 天、20 天统计视图
- 使用 SQLite 本地数据库保存配置和统计数据
- GitHub Actions 可自动构建 Windows 版本，并在打标签时发布 Release

## 从 Release 下载

如果你只是想直接使用程序，推荐从 GitHub Releases 下载已经打包好的版本：

[点击前往 Releases 页面](https://github.com/sheetung/stand-Reminder/releases)

下载后解压 zip，运行其中的 `stand-reminder.exe` 即可。

## 本地运行

1. 安装 Go 1.26.1 或更高版本
2. 在项目目录执行：

```powershell
go build -ldflags='-H windowsgui' -o stand-reminder.exe .
```

3. 运行：

```powershell
.\stand-reminder.exe
```

4. 程序启动后会进入系统托盘
5. 单击托盘图标可打开控制中心

## 数据与发布说明

- 程序运行时会在可执行文件同目录下创建 `stand-reminder.db`
- 本地数据库文件不会提交到 Git 仓库
- GitHub Actions 会自动构建 Windows 可执行文件
- 推送形如 `v*` 的标签时，会自动生成 Release 并上传 zip 包

## 致谢

今天统计图的 UI 实现思路参考了 Mizuki 的文章，感谢原作者提供的设计与实现分享。

- 本文作者：Mizuki
- 本文链接：https://www.cnblogs.com/mizuki-vone/p/17752988.html
