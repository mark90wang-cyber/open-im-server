# 任务：群系统灰条不计入未读（纯 SDK 方案 + 打包 aar / xcframework）

> 本文档自包含，可在新窗口独立执行。目标：修改 OpenIM 官方客户端 SDK（`openim-sdk-core`），
> 让群会话的**系统灰条**（退群 / 踢人 / 改群名等）不计入未读数，然后用 gomobile 打包出
> Android 的 `aar` 与 iOS 的 `xcframework`，供前端直接接入。

---

## 0. 背景与整体结论（先读）

### 需求
群聊消息流里的系统灰条（成员进出、禁言、改群名/公告等，`contentType` 在 1501–1520）
不应该让群会话的未读数（红点数字）增加，但灰条本身仍要显示在消息流里。

### 三个场景 & 根因
一条群灰条会在三种时机影响未读，需要分别处理：

| 场景 | 触发 | 处理方 | 状态 |
|------|------|--------|------|
| **在线**收到灰条 | App 前台，SDK `doMsgNew` 本地 `unread+1` | 服务端把灰条标 `unreadCount=false`，SDK 已自动跳过 | ✅ 服务端已完成 |
| **切后台→回前台** | 触发 `SyncAllConversationHashReadSeqs` 用 `maxSeq-hasReadSeq` **整体覆盖**未读 | SDK 需改 | ⬅ 本任务 |
| **冷启动 / 重连** | 同上，也汇聚到 `SyncAllConversationHashReadSeqs` | SDK 需改 | ⬅ 本任务 |

- 服务端侧已完成：`open-im-server` 的 `pkg/common/config/parse.go` 让群灰条消息带
  `Options["unreadCount"]=false`（解决“在线”场景）。**本任务不需要再改服务端。**
- 切后台恢复 和 冷启动 最终都调用同一个函数
  `internal/conversation_msg/sync.go` → `SyncAllConversationHashReadSeqs`，
  它用 `unread = maxSeq - hasReadSeq` **整段覆盖**本地未读，这个公式不区分灰条 → 根因。

### 本任务采用的方案（方案 X：纯本地统计）
在 `SyncAllConversationHashReadSeqs` 覆盖未读时，对**群会话**扣除本地统计到的灰条数量：

```
sysCount = 本地 chat_logs 表中 (hasReadSeq, maxSeq] 区间内 contentType∈灰条集合 的条数
unread   = max(0, maxSeq - hasReadSeq - sysCount)
```

**为什么选纯 SDK 方案**：避免改 `openimsdk/protocol` 和服务端接口（那需要 3 仓库版本协同 +
维护 protocol/SDK fork）。纯本地统计只改 SDK 一个仓库。

**已知局限（可接受，需知悉）**：
- 未读只会**偏大**（灰条没扣干净），**绝不会偏小**（真实未读消息永不会被错误清掉），方向安全。
- 冷启动 / 长时间后台后，`(hasReadSeq, maxSeq]` 区间消息本地可能还没拉全，此时 `sysCount`
  偏小、未读偏大；等 SDK 把消息补拉齐后，下一次同步会自动修正。
- 若实测“冷启动偏大”不可接受，再升级为“本地不全时主动 `PullMessageBySeqs` 拉齐区间再数”
  （准确但冷启动多一批网络请求）。本任务先落地方案 X。

---

## 1. 环境准备（WSL / Linux）

已确认：Go 1.22.7 已安装；gomobile、Android NDK、Xcode 均未配置。

### 1.1 Clone 源码
SDK 源码尚未 clone。放到 workspace 下：

```bash
cd /home/mark_/workspace
git clone ssh://git@local.vortex.com:222/mark/openim-sdk-core.git
cd openim-sdk-core
# 建议锁定到与你们后端匹配的稳定 tag（示例，按实际选择）：
# git checkout v3.8.3-patch.3
git rev-parse HEAD    # 记录基线 commit
```

> 版本务必与你们线上使用的 SDK 版本一致，否则打出来的库和现网 App 行为不匹配。
> 如果前端明确了当前用的 SDK tag，请 `git checkout <该 tag>` 后再改。

### 1.2 平台产出的硬约束（重要）
- **Android `aar`**：可在 WSL/Linux 上用 gomobile 产出（需 gomobile + Android SDK + NDK）。
- **iOS `xcframework`**：**必须在 macOS + Xcode 上编译**。gomobile 的 `-target=ios` 依赖
  Xcode/clang 工具链，**WSL/Linux 无法产出 xcframework**。
  → iOS 产物需要在一台 Mac 上执行第 4 节的 iOS 步骤。WSL 这边只能完成代码修改 + Android 打包。

---

## 2. 代码修改

只改 1 个逻辑函数 + 新增 1 个本地 DB 统计方法 + 1 组常量。

### 2.1 灰条 contentType 集合（与服务端 `isSendMsg=true` 的 13 个一致）

| contentType | 含义 |
|---|---|
| 1501 | 群创建 |
| 1504 | 退群 |
| 1507 | 群主转让 |
| 1508 | 踢人 |
| 1509 | 被邀请入群 |
| 1510 | 进群 |
| 1511 | 群解散 |
| 1512 | 成员禁言 |
| 1513 | 取消成员禁言 |
| 1514 | 全员禁言 |
| 1515 | 取消全员禁言 |
| 1519 | 群公告变更 |
| 1520 | 群名称变更 |

这些常量 SDK 里已定义在 `pkg/constant/constant.go`（`GroupCreatedNotification` 等）。
建议在 `internal/conversation_msg/` 下新增一个集合便于维护，例如：

```go
// internal/conversation_msg/group_tips.go  （新增文件）
package conversation_msg

import "github.com/openimsdk/openim-sdk-core/v3/pkg/constant"

// groupTipContentTypes 是会写入群消息流、但不应计入未读的系统灰条类型。
// 与服务端 config/notification.yml 中 isSendMsg=true 的群通知保持一致。
var groupTipContentTypes = []int{
	constant.GroupCreatedNotification,           // 1501
	constant.MemberQuitNotification,             // 1504
	constant.GroupOwnerTransferredNotification,  // 1507
	constant.MemberKickedNotification,           // 1508
	constant.MemberInvitedNotification,          // 1509
	constant.MemberEnterNotification,            // 1510
	constant.GroupDismissedNotification,         // 1511
	constant.GroupMemberMutedNotification,       // 1512
	constant.GroupMemberCancelMutedNotification, // 1513
	constant.GroupMutedNotification,             // 1514
	constant.GroupCancelMutedNotification,       // 1515
	constant.GroupInfoSetAnnouncementNotification, // 1519
	constant.GroupInfoSetNameNotification,       // 1520
}
```

> 请以 SDK 该版本 `pkg/constant/constant.go` 里的实际常量名为准；若某个常量名不同，
> 用对应数值或正确的常量名替换。数值集合固定为上表 13 个。

### 2.2 新增本地统计方法（DB 层）

本地消息按会话分表存储，表名 `chat_logs_<conversationID>`，含 `seq` 与 `content_type`
两列（均有索引）。在消息 DB 接口与实现中新增一个 count 方法。

接口（`pkg/db/db_interface.go`，找到 message 相关接口定义处加一行）：

```go
// GetGroupTipUnreadCount 统计某会话在 (hasReadSeq, maxSeq] 区间内、
// contentType 属于系统灰条集合的消息条数。
GetGroupTipUnreadCount(ctx context.Context, conversationID string, hasReadSeq, maxSeq int64, contentTypes []int) (int64, error)
```

实现（`pkg/db/chat_log_model.go`，与其它 `chat_logs_<id>` 方法放一起）：

```go
func (d *DataBase) GetGroupTipUnreadCount(ctx context.Context, conversationID string, hasReadSeq, maxSeq int64, contentTypes []int) (int64, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var count int64
	err := d.conn.WithContext(ctx).
		Table(utils.GetTableName(conversationID)).
		Where("seq > ? AND seq <= ? AND content_type IN ?", hasReadSeq, maxSeq, contentTypes).
		Count(&count).Error
	return count, errs.WrapMsg(err, "GetGroupTipUnreadCount failed")
}
```

> `utils.GetTableName(conversationID)` 是 SDK 里已有的“取 chat_logs 表名”工具（若名字不同，
> 参照同文件其它方法里获取表名的写法）。`errs` 用该文件已 import 的错误包。
> 若该 SDK 版本 DB 层用的是别的 ORM 封装/方法签名，按同文件既有 `Count` 用法适配。

### 2.3 修改未读覆盖公式

文件：`internal/conversation_msg/sync.go`，函数 `SyncAllConversationHashReadSeqs`。
该函数里未读用 `maxSeq - hasReadSeq` 计算的地方有**两处**，都要改：

**处 ① 主循环**（`for conversationID, v := range seqs { ... }`）——原代码：

```go
if v.MaxSeq-v.HasReadSeq < 0 {
	unreadCount = 0
	log.ZWarn(ctx, "unread count is less than 0", nil, "conversationID",
		conversationID, "maxSeq", v.MaxSeq, "hasReadSeq", v.HasReadSeq)
} else {
	unreadCount = int32(v.MaxSeq - v.HasReadSeq)
}
```

改为：

```go
if v.MaxSeq-v.HasReadSeq < 0 {
	unreadCount = 0
	log.ZWarn(ctx, "unread count is less than 0", nil, "conversationID",
		conversationID, "maxSeq", v.MaxSeq, "hasReadSeq", v.HasReadSeq)
} else {
	unreadCount = c.calcUnreadExcludingGroupTips(ctx, conversationID, v.MaxSeq, v.HasReadSeq)
}
```

**处 ② `conversationIDsNeedSync` 分支**（`for _, conversation := range conversationsOnServer { ... }`）——原代码：

```go
if v.MaxSeq-v.HasReadSeq < 0 {
	unreadCount = 0
	log.ZWarn(ctx, "unread count is less than 0", nil, "server seq", v, "conversation", conversation)
} else {
	unreadCount = int32(v.MaxSeq - v.HasReadSeq)
}
conversation.UnreadCount = unreadCount
```

改为：

```go
if v.MaxSeq-v.HasReadSeq < 0 {
	unreadCount = 0
	log.ZWarn(ctx, "unread count is less than 0", nil, "server seq", v, "conversation", conversation)
} else {
	unreadCount = c.calcUnreadExcludingGroupTips(ctx, conversation.ConversationID, v.MaxSeq, v.HasReadSeq)
}
conversation.UnreadCount = unreadCount
```

**新增辅助方法**（放在 `sync.go` 或上面的 `group_tips.go`）：

```go
// calcUnreadExcludingGroupTips 计算未读数；对群会话扣除区间内的系统灰条数量。
// 非群会话保持原公式 maxSeq - hasReadSeq。
func (c *Conversation) calcUnreadExcludingGroupTips(ctx context.Context, conversationID string, maxSeq, hasReadSeq int64) int32 {
	base := maxSeq - hasReadSeq
	if base <= 0 {
		return 0
	}
	// 仅对群会话处理（超级群本地会话 ID 前缀为 sg_）。
	if !c.isGroupConversation(ctx, conversationID) {
		return int32(base)
	}
	sysCount, err := c.db.GetGroupTipUnreadCount(ctx, conversationID, hasReadSeq, maxSeq, groupTipContentTypes)
	if err != nil {
		// 统计失败时退回原公式，保证“偏大不偏小”，不误伤真实未读。
		log.ZWarn(ctx, "GetGroupTipUnreadCount failed, fallback to raw unread", err,
			"conversationID", conversationID)
		return int32(base)
	}
	unread := base - sysCount
	if unread < 0 {
		unread = 0
	}
	return int32(unread)
}

// isGroupConversation 判断是否群（超级群）会话。
func (c *Conversation) isGroupConversation(ctx context.Context, conversationID string) bool {
	// 优先用本地会话记录的 ConversationType 判断；取不到时退回按 sg_ 前缀判断。
	if lc, err := c.db.GetConversation(ctx, conversationID); err == nil && lc != nil {
		return lc.ConversationType == constant.SuperGroupChatType
	}
	return strings.HasPrefix(conversationID, "sg_")
}
```

> - `c.db` 是该函数所在结构体里的本地 DB 句柄（`SyncAllConversationHashReadSeqs` 已在用 `c.db`）。
> - `constant.SuperGroupChatType` 为超级群会话类型；若该版本常量名不同，用对应的群会话类型常量。
> - 记得 `import "strings"`。
> - `GetConversation` 若签名不同，用该版本 DB 层“按 ID 取单个会话”的方法替换。

### 2.4 （可选，观察后再决定）多端已读同步路径
`internal/conversation_msg/read_drawing.go` 的 `doUnreadCount` 也用裸公式
`currentMaxSeq - hasReadSeq`。若上线后发现“多设备已读同步后未读又把灰条算回来”，
在这里套用同样的扣除逻辑（复用 `calcUnreadExcludingGroupTips`）。本任务先不改，先观察。

---

## 3. 本地编译校验（WSL）

改完先做纯 Go 编译，确保语法/依赖没问题（不依赖 gomobile）：

```bash
cd /home/mark_/workspace/openim-sdk-core
go build ./...
go vet ./internal/conversation_msg/ ./pkg/db/
```

两条都通过再进入打包。

---

## 4. 打包

### 4.1 Android aar（可在 WSL 完成）

**依赖安装（一次性）**：

```bash
# 1) gomobile / gobind
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# 2) Android SDK + NDK
#    需要 Android command line tools 与 NDK。推荐 NDK r20b (20.1.5948944) 或 20.0.5594570，
#    与官方文档一致，避免 gomobile 兼容性问题。
#    安装后设置环境变量（路径按实际安装位置）：
export ANDROID_HOME=$HOME/Android/Sdk
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/20.1.5948944
export PATH="$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools"

# 3) 初始化 gomobile（会下载所需组件）
gomobile init
```

**打包**（仓库自带 `make android`，等价于官方 gomobile 命令）：

```bash
cd /home/mark_/workspace/openim-sdk-core
make android
# 等价命令（如 make 目标不适用可直接跑）：
# GOARCH=arm64 gomobile bind -androidapi 21 -v -trimpath \
#   -ldflags='-s -w -extldflags "-Wl,--gc-sections,--as-needed,-z,max-page-size=16384"' \
#   -o ./open_im_sdk.aar -target=android ./open_im_sdk/ ./open_im_sdk_callback/
```

产物：仓库根目录 `open_im_sdk.aar`。交付给 Android 前端替换现有 aar。

> 提示：默认 `make android` 里 `GOARCH=amd64`；如需覆盖全部 ABI，用
> `-target=android/arm64,android/arm,android/amd64,android/386`（体积更大，按前端需要）。

### 4.2 iOS xcframework（必须在 macOS 上）

WSL 无法产出 iOS 库。将改好的代码（同一份 SDK 仓库）在一台 Mac 上执行：

```bash
# macOS 前置：安装 Xcode（>=15.4）并 xcode-select 指向它；安装 Go；然后：
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
export PATH="$PATH:$(go env GOPATH)/bin"
gomobile init

cd openim-sdk-core
make ios
# 等价命令：
# GOARCH=arm64 gomobile bind -v -trimpath -ldflags "-s -w" \
#   -o build/OpenIMCore.xcframework -target=ios ./open_im_sdk/ ./open_im_sdk_callback/
```

产物：`build/OpenIMCore.xcframework`。交付给 iOS 前端替换现有 framework。

> 如果没有 Mac：可用 macOS CI（如 GitHub Actions 的 `macos-latest` runner）跑 `make ios`，
> 把改好的分支推上去，用 CI 产出 xcframework 工件下载。

---

## 5. 验收（前端接入后回归）

对同一个有系统灰条的群，逐项验证未读红点数字：

1. **在线**：群里发生一次退群/踢人/改群名 → 未读**不增加**（服务端已保证，回归确认）。
2. **切后台再回前台**：未读**不因灰条变化**（本次 SDK 改动核心场景）。
3. **杀进程冷启动 / 断网重连**：未读**不因灰条偏高**；若曾长时间离线，允许短暂偏大，
   随消息拉全后下次同步自愈。
4. **真实未读不被误清**：群里有真实聊天未读 + 夹杂灰条时，真实未读数**保持正确**、不被清零。
5. 单聊、非群会话未读**行为不变**。

---

## 6. 交付物清单

- 修改后的 `openim-sdk-core` 分支（含 `sync.go`、DB 层 count 方法、灰条常量集合）。
- `open_im_sdk.aar`（Android，WSL 产出）。
- `OpenIMCore.xcframework`（iOS，macOS/CI 产出）。
- 记录本次基于的 SDK 基线 tag/commit，便于后续升级时 rebase 本改动。

---

## 附：为什么服务端不用再改
服务端 `pkg/common/config/parse.go` 已让群灰条消息带 `Options["unreadCount"]=false`，
覆盖“在线”场景。本任务的 SDK 改动覆盖“切后台/冷启动”场景。两者配合，三个场景全覆盖。
无需改 `openimsdk/protocol`，无需改服务端接口，无跨仓库 proto 版本协同成本。
