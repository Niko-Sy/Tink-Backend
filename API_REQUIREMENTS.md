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
开发环境: http://localhost:8080/api/v1
生产环境: https://api.tink.chat/api/v1
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

**接口**: `GET /auth/logout`

**请求头**: 需要 Authorization

**响应**:

```typescript
{
  "code": 200,
  "message": "退出成功"
}
```

### 2.4 刷新Token

**接口**: `GET /auth/refresh`

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

**接口**: `POST /auth/changepwd`

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

**接口**: `GET /users/me/userinfo`

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

**接口**: `GET /users/:userid/info`

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

**接口**: `POST /chatroom/createroom`

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

**接口**: `POST /chatroom/joinroom`

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

**接口**: `POST /chatroom/leaveroom`

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

**接口**: `GET /chatroom/:roomid/info`

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

**接口**: `POST /chatroom/:roomid/update`

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

**接口**: `POST /chatroom/:roomid/delete`

**权限**: 仅创建者可删除

1. ✅ 验证用户登录状态
2. ✅ 检查聊天室是否存在
3. ✅  **权限检查** : 只有房主（owner）可以删除
4. ✅ 执行软删除（设置 `room_status = 'deleted'`）

---

## 5. 消息相关接口

### 5.1 发送消息

**接口**: `POST /chatroom/:roomid/messages`

**请求体**:

```typescript
{
  "type": "text" | "image" | "file",  // 必填，消息类型
  "text": "消息内容",                   // 必填，消息文本
  "replyToMessageId": "M001"           // 可选，回复的消息ID
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "消息发送成功",
  "data": {
    "messageId": "M000000000000001",   // M+15位数字
    "roomId": "100000002",
    "userId": "U123456789",
    "userName": "张伟",
    "type": "text",
    "text": "消息内容",
    "time": "2025-11-23T10:00:00Z",
    "isOwn": true
  }
}
```

**功能说明**:
1. ✅ 验证用户是否在聊天室中
2. ✅ 检查用户禁言状态（全局禁言 + 聊天室禁言）
3. ✅ 创建消息并保存到数据库
4. ✅ 通过 WebSocket 实时广播消息到房间所有在线成员
5. ✅ 异步更新聊天室最后活跃时间

### 5.2 获取聊天室消息历史

**接口**: `GET /chatroom/:roomid/messages`

**查询参数**:

```
?page=1&pageSize=50           // 传统分页：page=1 返回最新消息
?before=M100&pageSize=50      // 游标分页：获取指定消息之前（更早）的消息
```

**分页设计说明**:

```
时间轴:  [最早] ←←←←←←←←←←←←←←← [最新]
消息ID:  M1 ← M2 ← M3 ... ← M98 ← M99 ← M100
         
查询结果（降序）:
page=1:  [M100, M99, M98, ..., M51]  ← 最新 50 条
page=2:  [M50, M49, M48, ..., M1]    ← 更早 50 条

游标分页:
?page=1              → [M100...M51]
?before=M51          → [M50...M1]   ← 获取 M51 之前的消息
```

**响应**:

```typescript
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M000000000000001",
        "roomId": "100000002",
        "userId": "U123456790",
        "userName": "李娜",
        "type": "text",
        "text": "大家好",
        "time": "2025-11-23T10:00:00Z",
        "isOwn": false,
        "isEdited": false,
        "editedAt": null,
        "replyToMessageId": null      // 可选，回复的消息ID
      }
    ],
    "total": 1000,
    "page": 1,
    "pageSize": 50,
    "hasMore": true
  }
}
```

**前端使用建议**:
- 首次加载：`GET /messages?page=1&pageSize=50`
- 向上滚动加载历史：`GET /messages?before=<最早消息ID>&pageSize=50`
- 前端需要将返回的消息列表反转显示（最早的在上，最新的在下）

### 5.3 撤回/删除消息

**接口**: `POST /chatroom/:roomid/messages/:messageid/delete`

**权限**: 消息发送者或管理员

**响应**:

```typescript
{
  "code": 200,
  "message": "消息已删除"
}
```

### 5.4 编辑消息

**接口**: `POST /chatroom/:roomid/messages/:messageid/edit`

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
    "messageId": "M000000000000001",
    "text": "编辑后的内容",
    "isEdited": true,
    "editedAt": "2025-11-23T10:05:00Z"
  }
}
```

### 5.5 标记消息已读

**接口**: `POST /chatroom/:roomid/messages/read`

**请求体**:

```typescript
{
  "lastReadMessageId": "M100"
}
```

---

## 6. 聊天室成员管理接口

### 6.1 获取聊天室成员列表

**接口**: `GET /chatroom/:roomid/members/memberlist`

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

**接口**: `GET /chatroom/:roomid/members/:userid/info`

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

**接口**: `POST /chatroom/:roomid/members/mute`

**权限**: 管理员权限

**请求体**:

```typescript
{
  "memberid": "M_U123456789_100000002"
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

**接口**: `POST /chatroom/:roomid/members/unmute`

**权限**: 管理员权限

**请求体**:

```typescript
{
  "memberid": "M_U123456789_100000002"
}
```

### 6.5 踢出成员

**接口**: `POST /chatroom/:roomid/members/kick`

**权限**: 管理员权限

**请求体**:

```typescript
{
  "memberid": "M_U123456789_100000002"
  "reason": "违规" //可选
}
```

### 6.6 设置管理员

**接口**: `POST /chatroom/:roomid/members/setadmin`

**权限**: 仅房主

**请求体**:

```typescript
{
  "memberid":"M_U123456789_100000002" 
}
```

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

**接口**: `POST /chatroom/:roomid/members/removeadmin`

**权限**: 仅房主

**请求体**:

```typescript
{
  "memberid":"M_U123456789_100000002" 
}
```

**响应**:

```typescript
{
  "code": 200,
  "message": "移除管理员权限成功",
}
```



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

**接口**: `POST /friends/:userid/delete`

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

**接口**: `POST /users/me/uploadavatar`

**请求**: multipart/form-data

```
file: <图片文件>
```

**限制**: 最大 5MB，支持 jpg/png/gif/webp

**响应**:

```typescript
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "url": "/static/images/avatars/avatar_U123456789_xxxx.jpg",
    "fileName": "avatar_U123456789_xxxx.jpg",
    "fileSize": 102400
  }
}
```

### 9.2 上传聊天图片

**接口**: `POST /chatroom/:roomid/uploadimage`

**请求**: multipart/form-data

```
file: <图片文件>
```

**限制**: 最大 5MB，支持 jpg/png/gif/webp

**响应**:

```typescript
{
  "code": 200,
  "message": "上传成功",
  "data": {
    "url": "/static/images/chat/100000002/chat_U123456789_xxxx.jpg",
    "fileName": "chat_U123456789_xxxx.jpg",
    "fileSize": 204800
  }
}
```

### 9.3 获取图片

**接口**: `GET /static/images/*filepath`

**说明**: 静态文件服务，直接返回图片文件

**路径结构**:
- 头像: `/static/images/avatars/{filename}`
- 聊天图片: `/static/images/chat/{roomId}/{filename}`

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

### 11.1 连接建立

**URL**: `ws://localhost:8080/ws` 或 `wss://api.tink.chat/ws`

**连接参数**:

```
?token=<jwt_token>
```

**连接流程**:

```
1. 客户端携带 JWT Token 发起 WebSocket 连接
2. 服务端验证 Token 有效性
3. 验证通过后建立连接，自动完成以下操作：
   - 将用户标记为在线状态
   - 自动订阅用户加入的所有聊天室
   - 增加各聊天室的在线人数计数
4. 连接断开时自动执行：
   - 将用户标记为离线状态
   - 从所有聊天室取消订阅
   - 减少各聊天室的在线人数计数
```

**连接示例**:

```javascript
const token = localStorage.getItem('token');
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.onopen = () => {
  console.log('WebSocket 连接成功');
  
  // 启动心跳检测（每30秒）
  setInterval(() => {
    ws.send(JSON.stringify({ type: 'ping' }));
  }, 30000);
};

ws.onclose = (event) => {
  console.log('WebSocket 连接关闭', event.code);
  // 实现重连逻辑
};

ws.onerror = (error) => {
  console.error('WebSocket 错误', error);
};
```

---

### 11.2 消息格式规范

#### 通用消息结构

```typescript
interface WSMessage {
  type: string;           // 消息类型
  action?: string;        // 操作类型
  data?: any;             // 消息数据
}
```

---

### 11.3 客户端发送消息

#### 11.3.1 发送聊天消息

```typescript
{
  "type": "message",
  "action": "send",
  "data": {
    "roomId": "100000002",           // 必填，聊天室ID
    "messageType": "text",           // 必填，消息类型: text|image|file
    "text": "消息内容",               // 必填，消息文本
    "quotedMessageId": "M001"        // 可选，回复的消息ID
  }
}
```

**服务端处理流程**:
1. 验证用户是否在聊天室中
2. 检查用户禁言状态（全局禁言 + 聊天室禁言）
3. 创建消息并保存到数据库
4. 广播消息到聊天室所有在线成员
5. 异步更新聊天室最后活跃时间

**错误响应**:

```typescript
// 不在聊天室中
{
  "type": "error",
  "action": "not_in_room",
  "data": { "message": "not in room" }
}

// 被禁言
{
  "type": "error",
  "action": "muted",
  "data": { "message": "muted" }
}
```

#### 11.3.2 心跳包

```typescript
// 客户端发送（建议每30秒）
{ "type": "ping" }
```

---

### 11.4 服务端推送消息

#### 11.4.1 新消息通知

```typescript
{
  "type": "message",
  "action": "new",
  "data": {
    "messageId": "M000000000000001",  // M+15位数字
    "roomId": "100000002",
    "userId": "U123456790",
    "userName": "李娜",                // 优先显示昵称，无则显示用户名
    "type": "text",
    "text": "消息内容",
    "time": "2025-11-23T10:00:00Z"    // ISO 8601 格式
  }
}
```

#### 11.4.2 消息编辑通知

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

#### 11.4.3 消息删除通知

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

#### 11.4.4 用户上线/下线通知

```typescript
{
  "type": "user_status",
  "action": "online" | "offline",
  "data": {
    "userId": "U123456790",
    "userName": "李娜",
    "status": "online" | "offline",
    "roomId": "100000002"            // 相关聊天室
  }
}
```

#### 11.4.5 聊天室成员变动

```typescript
{
  "type": "room_member",
  "action": "join" | "leave" | "kick",
  "data": {
    "roomId": "100000002",
    "userId": "U123456790",
    "userName": "李娜",
    "operatorId": "U123456789",       // kick 时的操作者
    "reason": "违规行为"               // kick 时的原因（可选）
  }
}
```

#### 11.4.6 禁言通知

```typescript
{
  "type": "mute",
  "action": "muted" | "unmuted",
  "data": {
    "roomId": "100000002",
    "userId": "U123456789",
    "muteUntil": "2025-11-23T11:00:00Z",  // muted 时必有
    "reason": "违反规定",                   // 可选
    "operatorId": "U123456788"             // 操作者ID
  }
}
```

#### 11.4.7 心跳响应

```typescript
{ "type": "pong" }
```

---

### 11.5 前端完整实现示例

```javascript
class WebSocketClient {
  constructor(token) {
    this.token = token;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.heartbeatInterval = null;
    this.messageHandlers = new Map();
  }

  connect() {
    this.ws = new WebSocket(`ws://localhost:8080/ws?token=${this.token}`);

    this.ws.onopen = () => {
      console.log('WebSocket 连接成功');
      this.reconnectAttempts = 0;
      this.startHeartbeat();
    };

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      this.handleMessage(msg);
    };

    this.ws.onclose = (event) => {
      console.log('WebSocket 连接关闭', event.code);
      this.stopHeartbeat();
      this.attemptReconnect();
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket 错误', error);
    };
  }

  startHeartbeat() {
    this.heartbeatInterval = setInterval(() => {
      if (this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000);
  }

  stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
      console.log(`${delay}ms 后尝试重连...`);
      setTimeout(() => this.connect(), delay);
    }
  }

  handleMessage(msg) {
    switch (msg.type) {
      case 'pong':
        // 心跳响应，无需处理
        break;
      case 'message':
        this.handleChatMessage(msg);
        break;
      case 'user_status':
        this.handleUserStatus(msg);
        break;
      case 'room_member':
        this.handleRoomMember(msg);
        break;
      case 'mute':
        this.handleMute(msg);
        break;
      case 'error':
        this.handleError(msg);
        break;
    }
  }

  handleChatMessage(msg) {
    switch (msg.action) {
      case 'new':
        // 新消息：追加到消息列表
        this.emit('newMessage', msg.data);
        break;
      case 'edit':
        // 消息编辑：更新对应消息内容
        this.emit('editMessage', msg.data);
        break;
      case 'delete':
        // 消息删除：从列表移除或显示"已删除"
        this.emit('deleteMessage', msg.data);
        break;
    }
  }

  handleUserStatus(msg) {
    this.emit('userStatus', msg.data);
  }

  handleRoomMember(msg) {
    this.emit('roomMember', { action: msg.action, ...msg.data });
  }

  handleMute(msg) {
    this.emit('mute', { action: msg.action, ...msg.data });
  }

  handleError(msg) {
    console.error('WebSocket 错误:', msg.action, msg.data);
    this.emit('error', { action: msg.action, ...msg.data });
  }

  // 发送聊天消息
  sendMessage(roomId, text, messageType = 'text', quotedMessageId = null) {
    const msg = {
      type: 'message',
      action: 'send',
      data: {
        roomId,
        messageType,
        text,
        ...(quotedMessageId && { quotedMessageId })
      }
    };
    this.ws.send(JSON.stringify(msg));
  }

  // 事件订阅
  on(event, handler) {
    if (!this.messageHandlers.has(event)) {
      this.messageHandlers.set(event, []);
    }
    this.messageHandlers.get(event).push(handler);
  }

  emit(event, data) {
    const handlers = this.messageHandlers.get(event) || [];
    handlers.forEach(handler => handler(data));
  }

  disconnect() {
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.close();
    }
  }
}

// 使用示例
const wsClient = new WebSocketClient(token);
wsClient.connect();

wsClient.on('newMessage', (data) => {
  console.log('收到新消息:', data);
  appendMessageToChat(data);
});

wsClient.on('editMessage', (data) => {
  console.log('消息已编辑:', data);
  updateMessageInChat(data.messageId, data.newText);
});

wsClient.on('deleteMessage', (data) => {
  console.log('消息已删除:', data);
  removeMessageFromChat(data.messageId);
});

wsClient.on('mute', (data) => {
  if (data.action === 'muted') {
    showMuteNotification(data);
  }
});

// 发送消息
wsClient.sendMessage('100000002', '你好，大家！');

// 回复消息
wsClient.sendMessage('100000002', '这是回复', 'text', 'M000000000000001');
```

---

### 11.6 历史消息加载（HTTP API 配合）

WebSocket 用于实时消息推送，历史消息通过 HTTP API 获取。

#### 分页设计原则

**page=1 返回最新消息，page 越大返回越早的历史消息**

```
时间轴:  [最早] ←←←←←←←←←←←←←←← [最新]
                                    ↑
消息ID:  M1 ← M2 ← M3 ... ← M98 ← M99 ← M100
         
查询结果（降序）:
page=1:  [M100, M99, M98, ..., M51]  ← 最新 50 条
page=2:  [M50, M49, M48, ..., M1]    ← 更早 50 条

游标分页:
?page=1              → [M100...M51]
?before=M51          → [M50...M1]   ← 获取 M51 之前的消息
```

#### 完整消息列表组件示例

```javascript
class MessageList {
  constructor(roomId, token, wsClient) {
    this.roomId = roomId;
    this.token = token;
    this.wsClient = wsClient;
    this.messages = [];
    this.oldestMessageId = null;
    this.loading = false;

    // 监听 WebSocket 新消息
    this.wsClient.on('newMessage', (data) => {
      if (data.roomId === this.roomId) {
        this.addNewMessage(data);
      }
    });
  }

  async loadInitial() {
    const response = await fetch(
      `/api/v1/chatroom/${this.roomId}/messages?page=1&pageSize=50`,
      { headers: { 'Authorization': `Bearer ${this.token}` } }
    );
    const data = await response.json();

    if (data.code === 200 && data.data.messages.length > 0) {
      // 后端返回降序，前端反转为升序显示
      this.messages = data.data.messages.reverse();
      this.oldestMessageId = this.messages[0].messageId;
      this.render();
      this.scrollToBottom();
    }
  }

  async loadMore() {
    if (this.loading || !this.oldestMessageId) return false;

    this.loading = true;
    const response = await fetch(
      `/api/v1/chatroom/${this.roomId}/messages?before=${this.oldestMessageId}&pageSize=50`,
      { headers: { 'Authorization': `Bearer ${this.token}` } }
    );
    const data = await response.json();

    if (data.code === 200 && data.data.messages.length > 0) {
      const olderMessages = data.data.messages.reverse();
      this.messages = [...olderMessages, ...this.messages];
      this.oldestMessageId = olderMessages[0].messageId;
      this.render();
    }

    this.loading = false;
    return data.data.hasMore;
  }

  addNewMessage(message) {
    this.messages.push(message);
    this.render();
    this.scrollToBottom();
  }

  render() {
    const container = document.getElementById('messages');
    container.innerHTML = this.messages
      .map(msg => this.renderMessage(msg))
      .join('');
  }

  renderMessage(msg) {
    const isOwn = msg.userId === currentUserId;
    return `
      <div class="message ${isOwn ? 'own' : ''}">
        <span class="user">${msg.userName}</span>
        <span class="text">${msg.text}</span>
        <span class="time">${new Date(msg.time).toLocaleTimeString()}</span>
      </div>
    `;
  }

  scrollToBottom() {
    const container = document.getElementById('messages');
    container.scrollTop = container.scrollHeight;
  }
}

// 使用
const messageList = new MessageList('100000002', token, wsClient);
await messageList.loadInitial();

// 监听滚动加载更多
document.getElementById('messages').addEventListener('scroll', async (e) => {
  if (e.target.scrollTop < 100) {
    const hasMore = await messageList.loadMore();
    if (!hasMore) {
      console.log('没有更多历史消息了');
    }
  }
});
```

---

### 11.7 技术规格

| 参数 | 值 | 说明 |
|------|------|------|
| 心跳间隔 | 30秒 | 客户端发送 ping |
| 读取超时 | 60秒 | 无消息则断开 |
| 写入超时 | 10秒 | 发送消息超时 |
| 消息缓冲 | 256条 | 每个连接的发送队列 |
| 最大消息大小 | 512字节 | 单条 WebSocket 消息 |

---

### 11.8 错误处理

| 错误类型 | action | 说明 | 处理建议 |
|----------|--------|------|----------|
| 未在聊天室 | `not_in_room` | 用户不是聊天室成员 | 提示用户先加入聊天室 |
| 被禁言 | `muted` | 用户被禁言无法发言 | 显示禁言提示和剩余时间 |
| Token 无效 | 连接失败 | JWT 过期或无效 | 刷新 Token 后重连 |

---

### 11.9 安全性

#### 已实现
- ✅ JWT Token 认证
- ✅ 聊天室成员身份验证
- ✅ 禁言状态检查（全局 + 聊天室）
- ✅ 消息发送权限验证

#### 建议加强
- 📋 消息内容过滤（敏感词、XSS）
- 📋 频率限制（每分钟最多 N 条消息）
- 📋 消息大小限制
- 📋 CORS 域名白名单

---

### 11.10 性能优化

#### 已实现
- ✅ 连接池管理（Hub 统一管理）
- ✅ 消息通道缓冲（256 条）
- ✅ 异步更新聊天室活跃时间
- ✅ 读写分离（readPump / writePump）
- ✅ 心跳保活机制
- ✅ 断线重连支持

#### 待优化
- 📋 Redis 缓存热点消息
- 📋 消息队列处理广播
- 📋 水平扩展（多 WebSocket 服务器）

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
