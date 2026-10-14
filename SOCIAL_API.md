> **当前运行配置更新：** 功能开关从config.yaml的social段读取（friends/direct_messages/room_invitations），来源从server.allowed_origins读取；正常启动不检查迁移版本，也不要求env文件。下面早期环境变量开关说明为历史版本；鉴权与业务协议保持不变。

# 第一批好友 API 与实时契约

所有新增接口前缀 `/api/v1`，携带 `Authorization: Bearer <token>`。操作者来自 JWT；必须是 active 账号。`/users/:userid/info` 保留公开读取，但统一使用最小公开资料。

## 能力与响应

GET `/users/me/social-capabilities` 返回 `{friends:true,directMessages:false,roomInvitations:false}`。`SOCIAL_FRIENDS_ENABLED=false` 关闭好友业务 API（503/FEATURE_DISABLED），摘要计数归零。单聊与邀请在本批无业务路由，不能通过环境变量提前启用。group/direct 隔离保护始终有效。

成功响应 `{code:200,message,data,timestamp}`，新申请 HTTP 201，幂等 pending 重试 200。错误 `{code,message,businessCode,data,timestamp}`，限流同时返回 Retry-After。timestamp 为 UTC RFC3339。分页 `{list,total,page,pageSize,totalPages}`，默认20、最大100，筛选与计数同口径。

## 接口

| 方法 | 路径 | 参数/返回 |
|---|---|---|
| GET | /users/search | 恰选 userId 或 username；精确匹配，返回 `{profile,relationship}[]`，未找到为空 |
| GET | /users/:userid/relationship | `{state,pendingRequestId,canRequest,canMessage}` |
| POST | /friends/requests | `{toUserId,message?}`，留言最多200 Unicode字符；返回 `{request,relationship,created}` |
| GET | /users/me/friend-requests | direction=received/sent（默认received），status=pending/accepted/rejected/cancelled或省略，page/pageSize |
| POST | /friends/requests/:requestid/accept | 接收方接受，空对象，返回 `{request,relationship,created:false}` |
| POST | /friends/requests/:requestid/reject | 接收方拒绝，空对象，同上 |
| POST | /friends/requests/:requestid/cancel | 发起方取消，保留历史，空对象，同上 |
| GET | /users/me/friends | q、status=all/online/away/busy/offline、page/pageSize |
| GET | /friends/:userid | 当前好友详情 `{profile,friendSince,relationship:"friends",canMessage:false}`；非好友404 |
| DELETE | /friends/:userid | `{removed:true}`，重复删除成功；失效双方未处理的房间邀请 |
| GET | /users/me/social-summary | `{receivedPendingFriendRequests,receivedPendingInvitations:0,directUnreadMessages:0}` |

Profile：userId、可选 username、nickname、avatar、signature、onlineStatus、availability。没有邮箱、电话、密码、登录时间或群成员权限。隐藏在线状态为 null；数据库 do_not_disturb 映射 busy；不可用联系人保留最小占位，不返回旧昵称/签名/联系方式。

Relationship.state：self/none/outgoing_pending/incoming_pending/friends/unavailable。canMessage 本批始终 false。Request 包含 requestId、senderId、receiverId、message、status、createdAt、nullable handledAt、peer、direction、canAccept/canReject/canCancel。

## 状态与并发

pending→accepted/rejected/cancelled；同一终态操作重试幂等，对冲操作409。接受与建立规范化关系同一事务，提交后发布事件。删除后重放旧accept不会恢复关系。同向pending返回原申请，反向pending返回409/INCOMING_REQUEST_PENDING和已有requestId。

按双方最新终态控制冷却：rejected 24小时、cancelled 60秒，两个方向共同适用。申请者与接收者必须可用，目标设置须允许申请；接受时再次校验。无 user_settings 行采用原默认值。接受动作权限也反映接收设置。

查找10次/分钟，新申请10次/分钟、50次/UTC日；同向 pending 幂等重试不消耗新增配额。分钟限流为单实例进程内计数，日配额在双方锁保护下查询数据库。多实例发布必须先引入共享限流与实时投递。

## 业务错误

INVALID_ARGUMENT(400)、UNAUTHORIZED(401)、ACCOUNT_UNAVAILABLE/TARGET_NOT_ADDABLE/FORBIDDEN(403)、RESOURCE_NOT_FOUND(404)、ALREADY_FRIENDS/INCOMING_REQUEST_PENDING/REQUEST_STATE_CONFLICT(409)、RATE_LIMITED/REQUEST_COOLDOWN(429)、INTERNAL_ERROR(500)、FEATURE_DISABLED(503)。错误响应不返回 SQL、连接信息或私密留言。

## 实时事件

沿用 `/ws` 与 `{type,action,data,timestamp}`：friend_request/created、friend_request/updated、friendship/created、friendship/removed。只推送关系双方的所有在线连接。friend_presence/updated 返回受隐私控制的 Profile，接收方重新查询公开事实。离线事项通过请求列表和摘要恢复。

同账号多个连接独立订阅；最后一个连接断开才离线，房间在线人数按唯一用户。WS room/join|leave 只代表订阅，不广播真实成员变化；真实加入/离开由成功的成员写入发布。每帧一个JSON，队列有界、满时停止慢连接，发送通道不在发布者可引用时关闭。

## 数据库与发布

使用000003～000005新增迁移，保留000001/000002。000003修复的自申请、双向pending和处理时间均保留到friend_request_migration_audit。存在cancelled历史、direct或邀请数据时，危险down拒绝执行。

部署必须先在隔离环境验证迁移，确认真实数据库已有版本，再由部署流程执行升级；应用启动不自动对配置中的数据库执行迁移。新旧后端不可在创建direct业务数据后混跑；本批无创建direct入口。sqlc使用本地schema，`make generate`固定v1.30.0。

## 第二批能力更新

directMessages由DIRECT_MESSAGES_ENABLED控制，默认false，隔离开发true；联系人与关系的canMessage改为实际开启且是有效好友的打开能力。global mute只影响会话canSend，不阻止打开历史。social-summary在单聊开启时返回真实directUnreadMessages，朋友模块关闭不阻止查询已开启单聊摘要。完整接口见DIRECT_API.md。
