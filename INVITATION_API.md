> **当前运行配置更新：** 功能开关从config.yaml的social段读取（friends/direct_messages/room_invitations），来源从server.allowed_origins读取；正常启动不检查迁移版本，也不要求env文件。下面早期环境变量开关说明为历史版本；鉴权与业务协议保持不变。

# 聊天室邀请 API（第三批 S6/S7）

前缀 `/api/v1`；所有接口使用 **access** Bearer JWT，并要求当前账号 active。成功 envelope `{code:200,message,data,timestamp}`；新建 HTTP201，复用 HTTP200。失败 `{code,businessCode,message,data}`，`data` 为安全错误详情；不返回 SQL/密码。分页 `page=1,pageSize=20`，最大100，返回 `{list,total,page,pageSize,totalPages}`。

## 路由

|方法|路径|参数/返回|
|---|---|---|
|GET|`/users/me/invitable-chatrooms`|q、page、pageSize、inviteeId? → 当前有权限的群 + eligible/reason|
|GET|`/chatroom/:roomid/invite-candidates`|q、page、pageSize → 好友公开资料 + eligible/reason|
|POST|`/chatroom/:roomid/invitations`|`{inviteeId,message}` → `{invitation,created}`|
|GET|`/users/me/chatroom-invitations`|direction=received/sent、status、page、pageSize|
|GET|`/chatroom/:roomid/invitations`|status、page、pageSize；active owner/admin 全部，member 只看自己发出的|
|POST|`/chatroom-invitations/:invitationid/accept`|`{}` → `{invitation,created:false,chatroom,memberInfo,alreadyMember}`|
|POST|`/chatroom-invitations/:invitationid/reject`|`{}` → `{invitation,created:false}`|
|POST|`/chatroom-invitations/:invitationid/cancel`|`{}` → `{invitation,created:false}`|

InvitationDTO：invitationId、roomId、direction、status、inviter/invitee（公开资料）、room（最小群摘要）、message、createdAt/expiresAt/handledAt、invalidationReason、canAccept/canReject/canCancel/canEnter（当前仍为active群成员才可进入）。删除群显示不可用占位；接受前不返回密码、群成员或群历史。状态筛选为空表示全部；有效状态统一由数据库判断，list/count/summary 使用一致谓词与列表快照。

## 授权与生命周期

public 当前 active owner/admin/member 可邀请；private/protected 仅 owner/admin；direct 禁止。邀请者与接收方当前好友且 active，接收方尚未加入。禁言不阻止邀请/加入；新入群保留有效禁言，发送仍受限制。

七天有效期。每 `(room,invitee)` 一个 pending，同邀请者重试返回原记录，其他邀请者409（不泄露留言）。留言 trim 后最多200 Unicode 字符。pending 只能变 accepted/rejected/cancelled/expired/invalidated；拒绝24h/取消60s冷却，按该群与接收者最新记录，不可换邀请者绕过。新增10/min、100/UTC日，复用不占额度；限流由事务内数据库计数保证。

accept/reject 仅接收方；cancel 仅邀请者或当前 active 房主。接受锁双方用户（升序）→active group→邀请，重查关系/账号/权限/到期。成员、人数和状态同事务；只有新增/inactive→active增人数。其他合法路径已入群则 alreadyMember=true。accepted 重试仍在群返回事实，已离开/被踢返回 `INVITATION_ALREADY_USED`，绝不重新加入。过期/失效提交终态后410；真实数据库或提交错误回滚且不发布事件。

好友删除、退出/踢人、失去邀请权限、群类型变更或删除会原子失效 pending；接收方退出/踢人撤销旧 pending。public 降为 private/protected 仅撤销失权邀请；密码轮换不撤销仍有效的授权邀请。已有授权邀请可加入密码群，普通加入仍检查密码。账号状态无法通过现有管理 API 修改的场景，操作/有效状态查询和清理任务复查，不新增管理页面。

`ROOM_INVITATIONS_ENABLED` 正常 false，隔离 true。关闭禁新建/候选/接受新入群；授权历史/拒绝/取消保留，清理仍运行。邀请和已有好友事实独立于好友入口开关。

## 错误与恢复

403 INVITE_FORBIDDEN/INVITATIONS_DISABLED/NOT_FRIENDS/ACCOUNT_UNAVAILABLE；404 RESOURCE_NOT_FOUND（未知或无权邀请）；409 ALREADY_MEMBER/INVITATION_ALREADY_PENDING/INVITATION_ALREADY_USED/INVITATION_STATE_CONFLICT；410 INVITATION_EXPIRED/INVITATION_INVALIDATED；429 RATE_LIMITED/REQUEST_COOLDOWN（Retry-After）。

私有 WS `invitation/created,updated` 只发双方所有连接；`room_invitations/updated` 向有权限的管理者提示刷新，不包含留言。入群提交后订阅并向本人所有连接发送 `room/added`，群收到 `room_member/joined`。退出/踢人/删群发送 `room/removed` 并撤销订阅；订阅不改变真实人数。事件为查询提示，重连/页面激活恢复；可见收件箱 WS 不可用时30秒轮询。

前端申请中心 `/contacts/requests?kind=invitations&direction=received&status=pending&page=1`，默认 kind=friends。接受后提供进入按钮，不自动导航。非好友邀请入口引导申请，不自动发送。
