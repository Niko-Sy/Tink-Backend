# Tink-Backend

当前个人项目配置：所有后端设置读取工作目录的config.yaml，不需要env文件，不在启动或健康检查时验证数据库迁移版本。云端已手工完成表结构时，即使没有schema_migrations也可以运行；不会自动重跑迁移。

## 启动与配置

`go run .`，或GoLand运行Package=chatroombackend，Working directory=Tink-Backend。指定配置可用`go run . -config /路径/config.yaml`。旧DATABASE_URL/BASE_URL/JWT_SECRET/功能开关环境变量不覆盖运行配置。

数据库使用database.url；JWT使用jwt.secret、expiration、refresh_expiration；CORS/WS来源使用server.allowed_origins；功能使用social.friends/direct_messages/room_invitations。缺少social段的旧YAML默认启用已交付三项功能；当前config.yaml显式开启。

好友API见[SOCIAL_API.md](./SOCIAL_API.md)，私信见[DIRECT_API.md](./DIRECT_API.md)，邀请见[INVITATION_API.md](./INVITATION_API.md)。仍使用独立access/refresh和一次性WS票据，事务/第三人权限不变。

## 个人部署

沿用根目录tinkbackend.service，工作目录放config.yaml，执行myapp-linux；没有EnvironmentFile前置要求。部署与本地前端见[当前手册](../P1_SOCIAL_DEPLOYMENT_GUIDE.md)。本地/生产可以共用现有云库；启动不清空在线人数，但两个进程的Hub/在线状态不提供跨进程同步。让前端统一连接一个后端即可共享实时通道。

## 验证工具（可选）

`go test ./...`、`make test-race`使用自动临时PG；`make generate`固定sqlc1.30；`make dev-social/dbcheck`仍提供可选隔离联调。数据库版本命令与巡检保留作为显式工具，不作为启动要求，不强制登记手工迁移的版本。
