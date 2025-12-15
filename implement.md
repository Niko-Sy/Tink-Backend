## 计划: Tink聊天室后端缺失功能补全

基于对整个项目的深度分析,当前项目 **核心聊天功能已完整** ,但 **社交和平台功能严重缺失** 。虽然数据库设计完善(已创建8张扩展表),SQL查询也已编写,但多个重要模块的API层完全未实现,导致整体完成度约65-70%。

### 步骤

1. **补全消息已读功能** - 实现 `POST /chatroom/:roomid/messages/read` 接口,完善未读消息计数机制
2. **实现好友系统API** - 基于已有的 `friend_request` 和 `friends` 表,创建 `api/friends/` 目录,实现发送/处理好友请求、好友列表等5个接口
3. **实现通知系统API** - 基于已有的 `notifications` 表和SQL查询,创建 `api/notifications/` 目录,实现通知列表、标记已读等接口
4. **实现用户设置API** - 基于已有的 `user_settings` 表,创建 `api/settings/` 目录,实现获取/更新用户设置(通知偏好、隐私控制等)
5. **实现举报与反馈系统** - 基于已有的 `reports` 和 `feedbacks` 表,创建对应API目录,实现用户举报(用户/消息)和反馈建议功能
6. **增强WebSocket实时通知** - 在 `api/websocketmsg/websocket.go` 中添加用户上线/下线、成员变动(加入/退出/踢出)、禁言等事件的实时推送

### 进一步考虑

1. **SQL代码生成问题** - [extension](vscode-file://vscode-app/d:/Programfiles/Microsoft%20VS%20Code/resources/app/out/vs/code/electron-browser/workbench/workbench.html) 目录下的 `friend.sql`、`notification.sql`、`user_settings.sql` 已编写但未生成Go代码,需要更新 `sqlc.yaml` 配置并运行 `sqlc generate`
2. **路由注册** - 所有新增API需要在 `main.go` 中注册路由组,并应用JWT认证中间件
3. **优先级权衡** - 是优先完成P0/P1核心功能(好友、通知、设置)达到MVP状态,还是先完善现有模块(如WebSocket增强、消息引用详情)?
