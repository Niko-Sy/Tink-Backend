> **当前运行配置更新：** 功能开关从config.yaml的social段读取（friends/direct_messages/room_invitations），来源从server.allowed_origins读取；正常启动不检查迁移版本，也不要求env文件。下面早期环境变量开关说明为历史版本；鉴权与业务协议保持不变。

# 第二批单聊 API、恢复与发布契约

实现范围：S4～S5，好友之间唯一单聊，文本、回复、本人编辑/撤回、未读及本人阅读位置、多连接实时与完整变更恢复。邀请、媒体、陌生人私信、对方已读回执未启用。

## 能力与权限

正常启动读取 `DIRECT_MESSAGES_ENABLED`，默认false；`cmd/dev-social`显式true。开关在启动时确定，变更需重启。关闭时拒绝创建和新的内容写入，允许绑定参与者读取历史与更新本人阅读位置；已提交的同键发送可返回已有事实。好友开关与单聊开关独立，邀请能力固定false。

读取必须绑定direct_pairs的双方，系统管理员不能越权。操作者须active；写入要求双方active、仍为好友、开关开启、发送者未被全局禁言。其他群的禁言不影响私信。解除好友后历史保留只读，重新加好友复用原roomId。群聊入口仍拒绝direct。

## HTTP

前缀 `/api/v1`，JWT Bearer。成功 `{code:200,message,data,timestamp}`；新会话HTTP201，复用200。错误沿用SOCIAL_API.md的businessCode包装。

| 方法/路径 | 参数与响应 |
|---|---|
| POST /direct-conversations | `{peerUserId}`→`{conversation,created}` |
| GET /users/me/direct-conversations | cursor、pageSize默认20/上限100→`{list,hasMore,nextCursor}`；按lastActivityAt/roomId倒序 |
| GET /direct-conversations/:roomid | ConversationDTO，第三人404 |
| GET /direct-conversations/:roomid/messages | beforeSeq或afterSeq（二选一），默认50/上限100→`{messages,hasMore,nextCursor,latestSeq,changeRevision}`；消息按seq升序展示 |
| GET /direct-conversations/:roomid/changes | afterRevision、pageSize→`{changes,hasMore,nextCursor,upToRevision}` |
| POST /direct-conversations/:roomid/messages | `{type:"text",text,clientMessageId,quotedMessageId?}`→MessageDTO |
| POST /direct-conversations/:roomid/messages/:messageid/edit | `{text,expectedRevision}`→当前MessageDTO；旧版本409 |
| POST /direct-conversations/:roomid/messages/:messageid/delete | 空对象→撤回占位；重复撤回幂等 |
| POST /direct-conversations/:roomid/read | `{lastReadMessageId}`→`{roomId,lastReadSeq,unreadCount}` |

ConversationDTO：roomId、conversationKind=direct、peer公开资料、createdAt、lastActivityAt、lastMessage、unreadCount、lastReadSeq、latestSeq、changeRevision、canSend、readOnlyReason。

MessageDTO：messageId、roomId、conversationKind、clientMessageId、directSeq、revision、userId、sender公开资料、type、text、time、quotedMessageId、quote、editedAt、deletedAt、isDeleted。引用摘要不递归，最多100 Unicode字符；撤回后摘要与正文为占位/空正文。序号与版本以十进制字符串传输，时间UTC RFC3339。

readOnlyReason：feature_disabled/peer_unavailable/not_friends/globally_muted。不可用用户资料复用最小公开占位，不暴露邮箱、电话、系统角色或请求摘要。

## 幂等、版本、事务

- 文本去首尾空白后1～2000 Unicode字符，clientMessageId必须有效非零UUID。
- 按房间+发送者+UUID唯一。SHA-256摘要由固定字符串数组 `[roomId,actor,"text",规范文本,引用ID或空串]`编码计算，不对外返回，编辑/撤回不改变摘要。
- 同键同内容返回当前消息/撤回事实，同键不同内容409 MESSAGE_IDEMPOTENCY_CONFLICT。幂等命中先于新的写权限检查，但仍验证读取身份和账号。
- 双方用户按ID加锁，再锁配对/房间。消息、seq、revision、日志、活跃时间同事务；编辑/撤回仅推进revision，保持发送时间和会话排序。
- 创建房间、配对、两条成员记录同事务，延迟约束确保恰两人、普通成员、不可变种类。
- 读取/历史/变更使用一致快照。阅读只按本会话已提交消息推进GREATEST，未读不计本人或撤回。读取事件只发本人全部连接。
- 新会话10次/分钟，新私信30次/分钟；复用和消息幂等命中不计新增。单实例计数。

## WebSocket

沿用 `{type,action,data,timestamp}`。发送：`{type:"message",action:"send",data:{roomId,messageType:"text",text,clientMessageId,quotedMessageId?}}`。

提交后向发送连接回message/ack（完整MessageDTO），并向双方授权连接推送message/new、edited、deleted和direct/updated。direct/created补授权订阅；direct/read仅本人。发送失败为error/send_failed，data包含roomId、clientMessageId、code、businessCode、message、details，客户端不得把业务拒绝当传输失败重发。

HTTP和WS共用MessageService发送入口，kind由数据库推导。连接捕获路由注入的服务，不全局替换写入开关。直接消息投递再次验证接收连接的配对和账号。登录/重连恢复单聊订阅；订阅不改群人数，不产生群成员事件；本批不提供私信typing展示。

## 完整恢复

000006新增change_revision、direct_revision和direct_message_changes（只记录ID/动作/版本/时间，不复制正文），为已有消息回填基线并校准seq。数据库要求单聊消息携带版本且与日志/计数原子提交。

changes.action代表原日志动作，changes.message是同一响应快照中该消息的当前版本。旧new/edited日志可能携带当前撤回占位，客户端必须按MessageDTO.revision合并。nextCursor只按已确认日志页推进，不能以收到的WS最大版本或消息版本替代。

首次历史快照建立同步基线；已有缓存继续从原确认游标补changes。断线期间对旧消息的编辑/撤回也恢复。引用优先用已恢复原消息，未加载原文时使用服务端摘要。

## 前端与发布

/direct列表、/direct/:roomId会话，独立私信标签。联系人、公开资料、成员、消息头像四类入口统一创建/复用；非好友仅引导申请。单聊不调用群成员查询，不显示群管理/@选择。

ACK5秒等待，传输不确定用原键HTTP恢复；业务拒绝不回退。草稿/失败记录按账号和房间隔离，退出后清空，回退前验证当前token仍为原账号。仅页面可见、聚焦、消息底部实际进入视口才推进阅读；浏览旧历史、后台页不读到最新。不展示对方已读。

部署先核验数据库并应用000006，再发布兼容后端与前端，最后设置DIRECT_MESSAGES_ENABLED=true并重启。日志含数据时down拒绝；优先关闭新增写入、保留历史并前向修复。新迁移和测试仅在隔离环境执行，业务库未迁移。多实例需要共享实时/限流，仍不在本批声称支持。
