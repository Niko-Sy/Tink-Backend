# ChatRoom 后端接口需求文档

## 文档信息

- **项目名称**: Tink ChatRoom
- **前端技术栈**: React + TypeScript + Vite
- **文档版本**: 1.0.0
- **最后更新**: 2025-11-23

---

## 目录

1. [通用说明](#1-通用说明)
2. [认证相关接口](#2-认证相关接口)
3. [用户管理接口](#3-用户管理接口)
4. [聊天室管理接口](#4-聊天室管理接口)
5. [消息相关接口](#5-消息相关接口)
6. [聊天室成员管理接口](#6-聊天室成员管理接口)
7. [好友关系接口](#7-好友关系接口)
8. [通知系统接口](#8-通知系统接口)
9. [文件上传接口](#9-文件上传接口)
10. [系统管理接口](#10-系统管理接口)
11. [实时通信接口](#11-实时通信接口websocket)
12. [数据模型定义](#12-数据模型定义)

---

## 1. 通用说明

### 1.1 基础URL

```
开发环境: http://localhost:3000/api
生产环境: https://api.tink.chat/api
```

### 1.2 请求头

所有需要认证的接口都需要在请求头中携带：

```http
Authorization: Bearer <token>
Content-Type: application/json
```

### 1.3 通用响应格式

```typescript
{
  "code": 200,           // 状态码：200成功，400客户端错误，500服务器错误
  "message": "success",  // 消息描述
  "data": {},            // 响应数据
  "timestamp": "2025-11-23T10:00:00Z"
}
```

### 1.4 错误码定义

```typescript
200: 成功
400: 请求参数错误
401: 未授权（token无效或过期）
403: 无权限访问
404: 资源不存在
409: 资源冲突（如用户名已存在）
422: 验证失败（如密码格式不正确）
500: 服务器内部错误
```

---

## 2. 认证相关接口

### 2.1 用户注册

**接口**: `POST /auth/register`

**请求体**:

```typescript
{
  "username": "zhangwei",        // 必填，3-20字符，仅字母数字下划线
  "password": "123456",          // 必填，最少6位
  "email": "zhangwei@example.com", // 必填，邮箱格式
  "nickname": "张伟"              // 可选，用户昵称
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "注册成功",
  "data": {
    "userId": "U123456789",      // 用户ID（U+9位数字）
    "username": "zhangwei",
    "email": "zhangwei@example.com",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "nickname": "张伟",
    "avatar": "https://example.com/default-avatar.jpg",
    "onlineStatus": "online",
    "accountStatus": "active",
    "systemRole": "user",
    "registerTime": "2025-11-23T10:00:00Z"
  }
}
```

### 2.2 用户登录

**接口**: `POST /auth/login`

**请求体**:

```typescript
{
  "username": "zhangwei",  // 必填，用户名或邮箱
  "password": "123456"     // 必填
}
```

**响应**: 同注册接口

### 2.3 退出登录

**接口**: `POST /auth/logout`

**请求头**: 需要 Authorization

**响应**:

```typescript
{
  "code": 200,
  "message": "退出成功"
}
```

### 2.4 刷新Token

**接口**: `POST /auth/refresh-token`

**请求头**: 需要 Authorization

**响应**:

```typescript
{
  "code": 200,
  "message": "刷新成功",
  "data": {
    "token": "new_token_here",
    "expiresIn": 86400  // 过期时间（秒）
  }
}
```

### 2.5 修改密码

**接口**: `POST /auth/change-password`

**请求体**:

```typescript
{
  "oldPassword": "123456",
  "newPassword": "654321"
}
```

---

## 3. 用户管理接口

### 3.1 获取当前用户信息

**接口**: `GET /users/me/info`

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "userId": "U123456789",
    "username": "zhangwei",
    "nickname": "张伟",
    "name": "张伟",
    "avatar": "https://example.com/avatar.jpg",
    "email": "zhangwei@example.com",
    "phone": "13800138000",
    "signature": "这个人很懒，什么都没有留下~",
    "onlineStatus": "online",
    "accountStatus": "active",
    "systemRole": "user",
    "globalMuteStatus": "unmuted",
    "globalMuteEndTime": null,
    "registerTime": "2025-11-20T10:00:00Z",
    "lastLoginTime": "2025-11-23T10:00:00Z"
  }
}
```

### 3.2 更新用户资料

**接口**: `POST /users/me/update`

**请求体**:

```typescript
{
  "nickname": "新昵称",
  "avatar": "https://example.com/new-avatar.jpg",
  "signature": "新个性签名",
  "phone": "13800138000",
  "email": "newemail@example.com"
}
```

### 3.3 根据ID获取用户信息

**接口**: `GET /users/:userId/info`

**响应**:不含手机号邮箱敏感信息

```
{
  "code": 200,
  "data": {
    "userId": "U123456789",
    "nickname": "张伟",
    "name": "张伟",
    "avatar": "https://example.com/avatar.jpg",
    "signature": "这个人很懒，什么都没有留下~",
    "onlineStatus": "online",
    "accountStatus": "active",
    "systemRole": "user",
  }
}
```

### 3.4 在聊天室内搜索用户

**接口**: `GET /chatroom/:roomid/members/search`

**查询参数**:

```
?keyword=张伟&page=1&pageSize=20
```

**响应**: 需要鉴权，若用户不属于聊天室内成员则无法进行搜索获得信息

```typescript
{
  "code": 200,
  "data": {
    "users": [
      {
        "userId": "U123456789",
        "username": "zhangwei",
        "nickname": "张伟",
        "avatar": "...",
        "onlineStatus": "online"
      }
    ],
    "total": 100,
    "page": 1,
    "pageSize": 20
  }
}
```

### 3.5 更新在线状态

**接口**: `POST /users/me/updatestatus`

**请求体**:

```typescript
{
  "onlineStatus": "online" | "away" | "busy" | "offline"
}
```

---

## 4. 聊天室管理接口

### 4.1 创建聊天室

**接口**: `POST /chatrooms/createroom`

**请求体**:

```typescript
{
  "name": "综合文字",
  "description": "综合聊天室",
  "type": "public" | "private" | "protected",
  "password": "123456",  // type为protected时必填
  "icon": "fas fa-comments"
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "创建成功",
  "data": {
    "roomId": "100000002",  // 9位数字ID
    "name": "综合文字",
    "description": "综合聊天室",
    "icon": "fas fa-comments",
    "type": "protected",
    "creatorId": "U123456789",
    "onlineCount": 1,
    "peopleCount": 1,
    "createdTime": "2025-11-23T10:00:00Z",
    "lastMessageTime": "2025-11-23T10:00:00Z"
  }
}
```

### 4.2 加入聊天室

**接口**: `POST /chatrooms/joinroom`

**请求体**:

```typescript
{
  "roomId":"100000001",
  "password": "123456"  // 仅protected类型需要
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "加入成功",
  "data": {
    "roomId": "100000002",
    "memberInfo": {
      "memberId": "M_U123456789_100000002",
      "roomId": "100000002",
      "userId": "U123456789",
      "roomRole": "member",
      "isMuted": false,
      "joinedAt": "2025-11-23T10:00:00Z",
      "isActive": true
    }
  }
}
```

 **功能** :

1. ✅ 从 JWT Token 获取当前用户 ID
2. ✅ 验证聊天室是否存在
3. ✅ 检查用户是否已经是成员（避免重复加入）
4. ✅ 根据聊天室类型处理：
   * **public** : 直接加入
   * **private_password** : 验证密码后加入
   * **private_invite_only** : 拒绝加入（需要邀请）
5. ✅ 创建成员记录（角色为 member）
6. ✅ 增加聊天室成员计数

### 4.3 退出聊天室

**接口**: `POST /chatrooms/leaveroom`

**请求体**:

```typescript
{
  "roomId":"100000001"
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "退出成功"
}
```

 **功能** :

1. ✅ 验证用户登录状态
2. ✅ 检查聊天室是否存在
3. ✅ 检查用户是否是聊天室成员
4. ✅  **房主保护** : 房主不能直接退出，需先转让权限或解散聊天室
5. ✅ 执行退出操作（软删除：设置 `is_active=false`, `left_at=NOW()`）
6. ✅ 减少聊天室成员计数

### 4.4 获取用户的聊天室列表

**接口**: `GET /users/me/chatrooms`

**查询参数**:

```
?page=1&pageSize=20
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "chatrooms": [
      {
        "roomId": "100000002",
        "name": "综合文字",
        "description": "综合聊天室",
        "icon": "fas fa-comments",
        "type": "public",
        "creatorId": "U123456789",
        "onlineCount": 8,
        "peopleCount": 156,
        "unread": 12,  // 未读消息数
        "createdTime": "2025-11-23T10:00:00Z",
        "lastMessageTime": "2025-11-23T10:30:00Z",
        "currentUserMember": {
          "memberId": "M_U123456789_100000002",
          "roomRole": "owner",
          "isMuted": false,
          "joinedAt": "2025-11-23T10:00:00Z"
        }
      }
    ],
    "total": 5,
    "page": 1,
    "pageSize": 20
  }
}
```

### 4.5 获取聊天室详情

**接口**: `GET /chatrooms/:roomId/info`

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "roomId": "100000002",
    "name": "综合文字",
    "description": "综合聊天室",
    "icon": "fas fa-comments",
    "type": "public",
    "creatorId": "U123456789",
    "onlineCount": 8,
    "peopleCount": 156,
    "createdTime": "2025-11-23T10:00:00Z",
    "lastMessageTime": "2025-11-23T10:30:00Z"
  }
}
```

### 4.6 更新聊天室信息

**接口**: `POST /chatrooms/:roomId/update`

**权限**: 需要管理员权限

**请求体**:

```typescript
{
  "name": "新名称",
  "description": "新描述",
  "icon": "fas fa-comments",
  "type": "public" | "private" | "protected",
  "password": "新密码"  // 可选
}
```

✅ 验证用户登录状态

1. ✅ 检查聊天室是否存在
2. ✅  **权限检查** : 只有房主或管理员可以修改
3. ✅ 类型转换: `public`→`public`, `private`→`private_invite_only`, `protected`→`private_password`
4. ✅ 部分更新: 只更新提供的字段（使用指针类型判断）
5. ✅ 返回更新后的聊天室信息

### 4.7 删除聊天室

**接口**: `POST /chatrooms/:roomId/delete`

**权限**: 仅创建者可删除

1. ✅ 验证用户登录状态
2. ✅ 检查聊天室是否存在
3. ✅  **权限检查** : 只有房主（owner）可以删除
4. ✅ 执行软删除（设置 `room_status = 'deleted'`）

---

## 5. 消息相关接口

### 5.1 发送消息

**接口**: `POST /chatrooms/:roomId/messages`

**请求体**:

```typescript
{
  "type": "text" | "image" | "file",
  "text": "消息内容",
  "replyToMessageId": "M001"  // 可选，回复的消息ID
}
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "messageId": "M001",
    "roomId": "100000002",
    "userId": "U123456789",
    "userName": "张伟",
    "type": "text",
    "text": "消息内容",
    "time": "2025-11-23T10:00:00Z",
    "isOwn": true,
    "importmessageId": ""
  }
}
```

### 5.2 获取聊天室消息历史

**接口**: `GET /chatrooms/:roomId/messages`

**查询参数**:

```
?page=1&pageSize=50&before=M100  // before: 获取指定消息之前的消息
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M001",
        "roomId": "100000002",
        "userId": "U123456790",
        "userName": "李娜",
        "type": "text",
        "text": "大家好",
        "time": "2025-11-23T10:00:00Z",
        "isOwn": false,
        "isEdited": false,
        "editedAt": null
      }
    ],
    "total": 1000,
    "page": 1,
    "pageSize": 50,
    "hasMore": true
  }
}
```

### 5.3 撤回/删除消息

**接口**: `POST /chatrooms/:roomId/messages/:messageId/delete`

**权限**: 消息发送者或管理员

**响应**:

```typescript
{
  "code": 200,
  "message": "消息已删除"
}
```

### 5.4 编辑消息

**接口**: `POST /chatrooms/:roomId/messages/:messageId/edit`

**权限**: 消息发送者或管理员

**请求体**:

```typescript
{
  "text": "编辑后的内容"
}
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "messageId": "M001",
    "text": "编辑后的内容",
    "isEdited": true,
    "editedAt": "2025-11-23T10:05:00Z"
  }
}
```

### 5.5 标记消息已读

**接口**: `POST /chatrooms/:roomId/messages/read`

**请求体**:

```typescript
{
  "lastReadMessageId": "M100"
}
```

---

## 6. 聊天室成员管理接口

### 6.1 获取聊天室成员列表

**接口**: `GET /chatrooms/:roomId/members/memberlist`

**查询参数**:

```
?page=1&pageSize=20&status=online // status: online|away|offline|all
```

**响应**: 需要鉴权，若用户不属于聊天室内成员则无法进行获得成员列表信息

```typescript
{
  "code": 200,
  "data": {
    "members": [
      {
        "userId": "U123456789",
        "username": "zhangwei",
        "nickname": "张伟",
        "name": "张伟",
        "avatar": "...",
        "status": "online",
        "memberInfo": {
          "memberId": "M_U123456789_100000002",
          "roomRole": "owner",
          "isMuted": false,
          "muteUntil": null,
          "joinedAt": "2025-11-23T10:00:00Z",
          "isActive": true
        }
      }
    ],
    "total": 156,
    "onlineCount": 8,
    "page": 1,
    "pageSize": 20
  }
}
```

### 6.2 获取用户在聊天室的成员信息

**接口**: `GET /chatrooms/:roomId/members/:userId/info`

**响应**: 需要鉴权，如果api请求本人不在聊天室内，则接口不应该返回信息

```typescript
{
  "code": 200,
  "data": {
    "memberId": "M_U123456789_100000002",
    "roomId": "100000002",
    "userId": "U123456789",
    "roomRole": "owner",
    "isMuted": false,
    "muteUntil": null,
    "joinedAt": "2025-11-23T10:00:00Z",
    "lastReadAt": "2025-11-23T10:30:00Z",
    "isActive": true
  }
}
```

### 6.3 禁言用户

**接口**: `POST /chatrooms/:roomId/members/:userId/mute`

**权限**: 管理员权限

**请求体**:

```typescript
{
  "duration": 3600,  // 禁言时长（秒），-1表示永久
  "reason": "违反规定"  // 可选
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "禁言成功",
  "data": {
    "muteUntil": "2025-11-23T11:00:00Z"
  }
}
```

### 6.4 解除禁言

**接口**: `POST /chatrooms/:roomId/members/:userId/unmute`

**权限**: 管理员权限

### 6.5 踢出成员

**接口**: `POST /chatrooms/:roomId/members/:userId/kick`

**权限**: 管理员权限

**请求体**:

```typescript
{
  "reason": "违反规定"  // 可选
}
```

### 6.6 设置管理员

**接口**: `POST /chatrooms/:roomId/members/:userId/set-admin`

**权限**: 仅房主

**响应**:

```typescript
{
  "code": 200,
  "message": "设置成功",
  "data": {
    "roomRole": "admin"
  }
}
```

### 6.7 取消管理员

**接口**: `POST /chatrooms/:roomId/members/:userId/remove-admin`

**权限**: 仅房主

---

## 7. 好友关系接口

### 7.1 发送好友请求

**接口**: `POST /friends/request`

**请求体**:

```typescript
{
  "targetUserId": "U123456790",
  "message": "你好，我想加你为好友"  // 可选
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "好友请求已发送",
  "data": {
    "requestId": "FR001"
  }
}
```

### 7.2 获取好友列表

**接口**: `GET /users/me/friends`

**查询参数**:

```
?status=online  // online|away|offline|all
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "friends": [
      {
        "userId": "U123456790",
        "username": "lina",
        "nickname": "李娜",
        "avatar": "...",
        "status": "online",
        "friendSince": "2025-11-20T10:00:00Z"
      }
    ],
    "total": 50
  }
}
```

### 7.3 获取好友请求列表

**接口**: `GET /users/me/friend-requests`

**查询参数**:

```
?type=received|sent&status=pending|accepted|rejected
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "requests": [
      {
        "requestId": "FR001",
        "fromUserId": "U123456790",
        "toUserId": "U123456789",
        "message": "你好",
        "status": "pending",
        "createdAt": "2025-11-23T10:00:00Z"
      }
    ],
    "total": 5
  }
}
```

### 7.4 处理好友请求

**接口**: `POST /friends/request/:requestId/handle`

**请求体**:

```typescript
{
  "action": "accept" | "reject"
}
```

### 7.5 删除好友

**接口**: `POST /friends/:userId/delete`

---

## 8. 通知系统接口

### 8.1 获取通知列表

**接口**: `GET /users/me/notifications`

**查询参数**:

```
?type=all|friend|chatroom|system&status=unread|read|all&page=1&pageSize=20
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "notifications": [
      {
        "notificationId": "N001",
        "type": "friend",  // friend|chatroom|system
        "title": "新好友请求",
        "content": "李娜想加你为好友",
        "data": {
          "requestId": "FR001",
          "userId": "U123456790"
        },
        "isRead": false,
        "createdAt": "2025-11-23T10:00:00Z"
      }
    ],
    "total": 20,
    "unreadCount": 5
  }
}
```

### 8.2 标记通知已读

**接口**: `POST /notifications/:notificationId/read`

### 8.3 标记所有通知已读

**接口**: `POST /users/me/notifications/read-all`

### 8.4 获取用户设置

**接口**: `GET /users/me/settings`

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "notifications": {
      "enableFriendRequest": true,
      "enableChatRoomMessage": true,
      "enableSystemNotice": true,
      "enableSound": true,
      "enableDesktopNotification": true
    },
    "privacy": {
      "allowSearchByPhone": true,
      "allowSearchByEmail": true,
      "showOnlineStatus": true
    }
  }
}
```

### 8.5 更新用户设置

**接口**: `POST /users/me/settings/update`

**请求体**: 同8.4响应格式

---

## 9. 文件上传接口

### 9.1 上传头像

**接口**: `POST /upload/avatar`

**请求**: multipart/form-data

```
file: <文件>
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "url": "https://cdn.example.com/avatars/xxx.jpg",
    "size": 102400,
    "type": "image/jpeg"
  }
}
```

### 9.2 上传聊天图片

**接口**: `POST /upload/image`

**限制**: 最大5MB，支持jpg/png/gif

### 9.3 上传文件

**接口**: `POST /upload/file`

**限制**: 最大100MB

---

## 10. 系统管理接口

### 10.1 举报用户/消息

**接口**: `POST /reports`

**请求体**:

```typescript
{
  "type": "user" | "message",
  "targetId": "U123456790" | "M001",  // 用户ID或消息ID
  "roomId": "100000002",  // 消息举报时必填
  "reason": "spam",  // spam|harassment|inappropriate|other
  "description": "详细描述"
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "举报已提交",
  "data": {
    "reportId": "RP001"
  }
}
```

### 10.2 反馈建议

**接口**: `POST /feedback`

**请求体**:

```typescript
{
  "type": "bug" | "feature" | "other",
  "title": "标题",
  "content": "详细内容",
  "contactEmail": "user@example.com"  // 可选
}
```

### 10.3 获取帮助中心文档

**接口**: `GET /help/articles`

**查询参数**:

```
?category=getting-started|account|chatroom|privacy
```

---

## 11. 实时通信接口（WebSocket）

### 11.1 连接

**URL**: `ws://localhost:3000/ws` 或 `wss://api.tink.chat/ws`

**连接时携带**:

```
?token=<jwt_token>
```

### 11.2 消息格式

#### 客户端发送消息

```typescript
{
  "type": "message",
  "action": "send",
  "data": {
    "roomId": "100000002",
    "messageType": "text",
    "text": "消息内容"
  }
}
```

#### 服务端推送新消息

```typescript
{
  "type": "message",
  "action": "new",
  "data": {
    "messageId": "M001",
    "roomId": "100000002",
    "userId": "U123456790",
    "userName": "李娜",
    "type": "text",
    "text": "消息内容",
    "time": "2025-11-23T10:00:00Z"
  }
}
```

#### 用户上线/下线通知

```typescript
{
  "type": "user_status",
  "action": "online" | "offline",
  "data": {
    "userId": "U123456790",
    "status": "online"
  }
}
```

#### 聊天室成员变动

```typescript
{
  "type": "room_member",
  "action": "join" | "leave" | "kick",
  "data": {
    "roomId": "100000002",
    "userId": "U123456790",
    "userName": "李娜"
  }
}
```

#### 禁言通知

```typescript
{
  "type": "mute",
  "action": "muted" | "unmuted",
  "data": {
    "roomId": "100000002",
    "userId": "U123456789",
    "muteUntil": "2025-11-23T11:00:00Z",
    "reason": "违反规定"
  }
}
```

#### 消息撤回通知

```typescript
{
  "type": "message",
  "action": "delete",
  "data": {
    "roomId": "100000002",
    "messageId": "M001"
  }
}
```

#### 消息编辑通知

```typescript
{
  "type": "message",
  "action": "edit",
  "data": {
    "roomId": "100000002",
    "messageId": "M001",
    "newText": "编辑后的内容",
    "editedAt": "2025-11-23T10:05:00Z"
  }
}
```

#### 心跳包

```typescript
// 客户端发送（每30秒）
{
  "type": "ping"
}

// 服务端响应
{
  "type": "pong"
}
```

---

## 12. 数据模型定义

### 12.1 User（用户）

```typescript
interface User {
  userId: string;              // 用户ID（U+9位数字）
  username: string;            // 用户名（唯一）
  nickname?: string;           // 昵称
  name: string;                // 显示名称
  avatar: string;              // 头像URL
  email?: string;              // 邮箱
  phone?: string;              // 手机号
  signature?: string;          // 个性签名
  onlineStatus: 'online' | 'away' | 'busy' | 'offline';
  accountStatus: 'active' | 'inactive' | 'suspended';
  systemRole: 'super_admin' | 'user';  // 全局角色
  globalMuteStatus?: 'muted' | 'unmuted';
  globalMuteEndTime?: string;
  registerTime: string;
  lastLoginTime: string;
}
```

### 12.2 ChatRoom（聊天室）

```typescript
interface ChatRoom {
  roomId: string;              // 聊天室ID（9位数字）
  name: string;                // 名称
  description: string;         // 描述
  icon: string;                // 图标
  type: 'public' | 'private' | 'protected';
  password?: string;           // 仅protected类型
  creatorId: string;           // 创建者ID
  onlineCount: number;         // 在线人数
  peopleCount: number;         // 总人数
  createdTime: string;
  lastMessageTime: string;
  unread?: number;             // 未读消息数（仅客户端）
}
```

### 12.3 ChatRoomMember（聊天室成员）

```typescript
interface ChatRoomMember {
  memberId: string;            // 成员ID
  roomId: string;              // 聊天室ID
  userId: string;              // 用户ID
  roomRole: 'owner' | 'admin' | 'member';
  isMuted: boolean;
  muteUntil?: string;
  joinedAt: string;
  lastReadAt?: string;
  isActive: boolean;
  leftAt?: string;
}
```

### 12.4 Message（消息）

```typescript
interface Message {
  messageId: string;           // 消息ID
  roomId: string;              // 聊天室ID
  userId: string;              // 发送者ID
  userName?: string;           // 发送者名称
  type: 'text' | 'image' | 'file' | 'system';
  text: string;                // 消息内容
  fileUrl?: string;            // 文件/图片URL
  time: string;                // 发送时间
  isOwn: boolean;              // 是否自己发送（客户端）
  isEdited?: boolean;          // 是否已编辑
  editedAt?: string;           // 编辑时间
  importmessageId?: string;    // 重要消息ID
  replyToMessageId?: string;   // 回复的消息ID
}
```

### 12.5 MuteRecord（禁言记录）

```typescript
interface MuteRecord {
  recordId: string;
  userId: string;
  roomId: string;
  mutedBy: string;             // 操作者ID
  muteStartTime: string;
  muteEndTime: string;
  reason?: string;
  active: boolean;
}
```

### 12.6 FriendRequest（好友请求）

```typescript
interface FriendRequest {
  requestId: string;
  fromUserId: string;
  toUserId: string;
  message?: string;
  status: 'pending' | 'accepted' | 'rejected';
  createdAt: string;
  handledAt?: string;
}
```

### 12.7 Notification（通知）

```typescript
interface Notification {
  notificationId: string;
  userId: string;              // 接收者ID
  type: 'friend' | 'chatroom' | 'system';
  title: string;
  content: string;
  data?: any;                  // 附加数据（JSON）
  isRead: boolean;
  createdAt: string;
}
```

### 12.8 Report（举报）

```typescript
interface Report {
  reportId: string;
  reporterId: string;          // 举报人ID
  type: 'user' | 'message';
  targetId: string;            // 被举报对象ID
  roomId?: string;             // 消息举报时的聊天室ID
  reason: 'spam' | 'harassment' | 'inappropriate' | 'other';
  description: string;
  status: 'pending' | 'resolved' | 'rejected';
  createdAt: string;
  resolvedAt?: string;
  resolvedBy?: string;
}
```

---

## 13. 权限验证说明

### 13.1 消息权限

- **发送消息**: 需要未被禁言（全局禁言或聊天室禁言）
- **编辑自己的消息**: 所有成员
- **编辑他人消息**: 管理员及以上
- **删除自己的消息**: 所有成员
- **删除他人消息**: 管理员及以上

### 13.2 成员管理权限

- **邀请成员**: 所有成员
- **踢出成员**: 管理员及以上
- **禁言成员**: 管理员及以上
- **设置管理员**: 仅房主
- **取消管理员**: 仅房主

### 13.3 聊天室管理权限

- **编辑聊天室信息**: 管理员及以上
- **删除聊天室**: 仅房主

### 13.4 系统权限

- **全局禁言**: 仅超级管理员
- **封禁账号**: 仅超级管理员

---

## 14. 业务流程说明

### 14.1 用户注册登录流程

1. 用户填写注册信息 → 后端验证 → 创建用户 → 返回token
2. 前端保存token到localStorage
3. 后续请求携带token在请求头中

### 14.2 加入聊天室流程

1. 用户输入聊天室ID和密码
2. 后端验证聊天室存在性和密码
3. 创建ChatRoomMember记录
4. 返回聊天室信息和成员信息
5. 通过WebSocket通知其他成员

### 14.3 发送消息流程

1. 前端通过WebSocket发送消息
2. 后端验证权限（是否被禁言）
3. 保存消息到数据库
4. 通过WebSocket推送给聊天室所有在线成员
5. 更新未读消息计数

### 14.4 禁言流程

1. 管理员点击禁言 → 前端验证权限
2. 发送禁言请求到后端
3. 后端验证权限并创建禁言记录
4. 更新ChatRoomMember的isMuted状态
5. 通过WebSocket通知被禁言用户
6. 被禁言用户无法再发送消息

---

## 15. 安全性要求

### 15.1 认证安全

- 使用JWT Token进行身份验证
- Token有效期建议24小时
- 支持刷新Token机制
- 敏感操作（如修改密码）需要二次验证

### 15.2 数据验证

- 所有输入必须进行严格的格式验证
- 防止SQL注入、XSS攻击
- 文件上传需要验证文件类型和大小
- 限制请求频率，防止DOS攻击

### 15.3 隐私保护

- 密码使用bcrypt加密存储
- 敏感信息（手机号、邮箱）不在公开接口返回
- 支持用户隐私设置
- 遵守数据保护法规

---

## 16. 性能优化建议

### 16.1 数据库优化

- 对userId、roomId、messageId等建立索引
- 消息表按聊天室分表或分区
- 使用Redis缓存热点数据（在线用户、聊天室信息）

### 16.2 接口优化

- 支持分页查询
- 使用CDN存储静态资源（头像、文件）
- 消息列表支持增量加载
- WebSocket使用心跳保活

### 16.3 扩展性

- 使用消息队列处理通知推送
- 支持水平扩展（多个WebSocket服务器）
- 数据库读写分离

---

## 17. 开发优先级建议

### P0（核心功能）

- 用户注册/登录
- 创建/加入聊天室
- 发送/接收消息
- WebSocket实时通信

### P1（重要功能）

- 获取消息历史
- 聊天室成员管理
- 禁言功能
- 文件上传

### P2（增强功能）

- 好友系统
- 通知系统
- 消息编辑/撤回
- 举报功能

### P3（优化功能）

- 帮助中心
- 反馈系统
- 高级搜索
- 数据统计

---

**备注**:

1. 所有时间格式使用ISO 8601标准（如：2025-11-23T10:00:00Z）
2. 所有ID建议使用雪花算法或UUID生成，确保唯一性
3. 建议使用版本控制（如 /api/v1/...）以便后续升级
4. 所有敏感操作需要添加操作日志
5. 建议实现请求限流（如每个用户每分钟最多发送30条消息）
