# WebSocket API 接口文档

## 概述

本文档描述了聊天室系统的 WebSocket 实时通信接口。WebSocket 用于实时消息推送、在线状态同步、房间管理等功能。

### 版本信息

- **API 版本**: v1.0
- **协议**: WebSocket (RFC 6455)
- **消息格式**: JSON
- **字符编码**: UTF-8

---

## 1. 连接建立

### 1.1 连接地址

**开发环境:**

```
ws://localhost:8080/ws?token=YOUR_JWT_TOKEN
```

**生产环境:**

```
wss://your-domain.com/ws?token=YOUR_JWT_TOKEN
```

### 1.2 连接参数

| 参数  | 类型   | 必需 | 说明                           |
| ----- | ------ | ---- | ------------------------------ |
| token | string | 是   | JWT 认证令牌，通过登录接口获取 |

### 1.3 连接示例 (JavaScript)

```javascript
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.onopen = () => {
    console.log('WebSocket 连接成功');
    // 连接成功后，服务器会自动将用户加入其所有房间
};

ws.onmessage = (event) => {
    try {
        const message = JSON.parse(event.data);
        console.log('收到消息:', message);
        handleMessage(message);
    } catch (error) {
        console.error('解析消息失败:', error);
    }
};

ws.onerror = (error) => {
    console.error('WebSocket 错误:', error);
};

ws.onclose = (event) => {
    console.log('WebSocket 连接关闭', event.code, event.reason);
    // 建议实现自动重连机制
};
```

### 1.4 连接生命周期

1. **连接建立**: 客户端发起 WebSocket 连接请求，携带 JWT token
2. **身份验证**: 服务器验证 token 有效性
3. **自动入室**: 验证成功后，服务器自动将用户加入其所有聊天室
4. **在线标记**: 用户状态自动设置为在线
5. **保持连接**: 通过心跳机制维持连接
6. **断开处理**: 连接断开时自动设置离线，离开所有房间

### 1.5 连接状态码

| 状态码 | 说明       |
| ------ | ---------- |
| 1000   | 正常关闭   |
| 1001   | 端点离开   |
| 1006   | 异常关闭   |
| 1011   | 服务器错误 |

---

## 2. 消息格式规范

### 2.1 基础消息结构

所有 WebSocket 消息都遵循以下 JSON 格式：

```json
{
    "type": "消息类型",
    "action": "操作动作",
    "data": {
        // 具体数据内容
    }
}
```

### 2.2 字段说明

| 字段   | 类型   | 必需 | 说明                                                |
| ------ | ------ | ---- | --------------------------------------------------- |
| type   | string | 是   | 消息的类型类别（如 message, room, notification 等） |
| action | string | 否   | 具体的操作动作（如 send, new, join 等）             |
| data   | object | 否   | 消息的具体数据内容                                  |

### 2.3 消息类型列表

| Type         | 说明         | 方向           |
| ------------ | ------------ | -------------- |
| ping         | 心跳请求     | 客户端→服务器 |
| pong         | 心跳响应     | 服务器→客户端 |
| message      | 聊天消息     | 双向           |
| room         | 房间操作     | 双向           |
| room_member  | 房间成员事件 | 服务器→客户端 |
| notification | 系统通知     | 服务器→客户端 |
| user_status  | 用户状态     | 双向           |
| typing       | 正在输入     | 双向           |
| error        | 错误消息     | 服务器→客户端 |

---

## 3. 心跳机制

### 3.1 客户端发送心跳 (Ping)

**客户端发送:**

```json
{
    "type": "ping"
}
```

**服务器响应:**

```json
{
    "type": "pong"
}
```

**说明:**

- 用于保持连接活跃，防止超时断开
- 建议每 30 秒发送一次心跳
- 服务器也会主动发送 Ping 帧
- 60 秒无响应会自动断开连接

**实现示例:**

```javascript
// 启动心跳
const heartbeatInterval = setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'ping' }));
    }
}, 30000);

// 停止心跳
clearInterval(heartbeatInterval);
```

---

## 4. 聊天消息

### 4.1 发送消息

客户端向聊天室发送消息。

**客户端发送:**

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "text",
        "text": "你好，这是一条消息",
        "quotedMessageId": "M12345678901234567890"
    }
}
```

**参数说明:**

| 字段            | 类型   | 必需 | 说明                                    |
| --------------- | ------ | ---- | --------------------------------------- |
| roomId          | string | 是   | 聊天室ID                                |
| messageType     | string | 是   | 消息类型：`text`、`image`、`file` |
| text            | string | 是   | 消息内容或描述                          |
| quotedMessageId | string | 否   | 引用的消息ID（用于回复功能）            |
| mediaUrl        | string | 否   | 媒体文件URL（图片/文件消息时必需）      |

**消息类型说明:**

- `text`: 文本消息
- `image`: 图片消息（需要先上传图片获取 URL）
- `file`: 文件消息（需要先上传文件获取 URL）
- `system_notification`: 系统通知（仅服务器发送）

**发送文本消息示例:**

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "text",
        "text": "你好，世界！"
    }
}
```

**发送图片消息示例:**

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "image",
        "text": "分享图片",
        "mediaUrl": "http://example.com/uploads/chat/100000004/image.jpg"
    }
}
```

**发送文件消息示例:**

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "file",
        "text": "项目文档.pdf",
        "mediaUrl": "http://example.com/uploads/chat/100000004/document.pdf"
    }
}
```

**回复消息示例:**

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "text",
        "text": "我同意你的观点",
        "quotedMessageId": "M12345678901234567890"
    }
}
```

**发送流程:**

1. 验证用户是否在房间中
2. 验证用户是否被禁言
3. 验证消息类型和必需参数
4. 创建消息并保存到数据库
5. 广播消息到房间所有在线成员
6. 更新房间最后活跃时间

**可能的错误:**

- `not_in_room`: 用户不在该房间
- `muted`: 用户被禁言
- `invalid_type`: 无效的消息类型
- `missing_media`: 缺少媒体 URL

### 4.2 接收新消息

服务器向房间成员推送新消息。

**服务器推送:**

```json
{
    "type": "message",
    "action": "new",
    "data": {
        "messageId": "M12345678901234567890",
        "roomId": "100000001",
        "userId": "U100000002",
        "userName": "张三",
        "avatarUrl": "http://example.com/avatars/avatar_U100000002.jpg",
        "type": "text",
        "text": "你好，这是一条消息",
        "time": "2025-12-01T12:35:56Z",
        "quotedMessageId": "M12345678901234567890",
        "mediaUrl": "http://example.com/uploads/image.jpg"
    }
}
```

**字段说明:**

| 字段            | 类型   | 说明                                   |
| --------------- | ------ | -------------------------------------- |
| messageId       | string | 消息ID（M+20位数字）                   |
| roomId          | string | 聊天室ID                               |
| userId          | string | 发送者用户ID                           |
| userName        | string | 发送者显示名称（优先昵称，否则用户名） |
| avatarUrl       | string | 发送者头像URL（可选）                  |
| type            | string | 消息类型                               |
| text            | string | 消息内容                               |
| time            | string | 发送时间（ISO 8601格式）               |
| quotedMessageId | string | 被引用的消息ID（可选）                 |
| mediaUrl        | string | 媒体文件URL（可选）                    |

**系统消息示例:**

```json
{
    "type": "message",
    "action": "new",
    "data": {
        "messageId": "M12345678901234567890",
        "roomId": "100000001",
        "type": "system_notification",
        "text": "张三已被禁言10分钟",
        "time": "2025-12-01T12:00:00Z"
    }
}
```

### 4.3 消息已读

客户端标记消息为已读状态。

**客户端发送:**

```json
{
    "type": "message",
    "action": "read",
    "data": {
        "roomId": "100000001",
        "messageId": "M12345678901234567890"
    }
}
```

**说明:**

- 标记某条消息为已读
- 当前版本仅记录日志
- 未来版本将支持已读回执功能

### 4.4 消息删除通知

服务器通知消息被删除（通过 HTTP API 删除消息时触发）。

**服务器推送:**

```json
{
    "type": "message",
    "action": "deleted",
    "data": {
        "roomId": "100000001",
        "messageId": "M12345678901234567890",
        "timestamp": "2025-12-01T13:35:56Z"
    }
}
```

**说明:**

- 收到此通知后，客户端应从界面移除对应消息
- 仅房间管理员可删除消息

### 4.5 消息编辑通知

服务器通知消息被编辑（通过 HTTP API 编辑消息时触发）。

**服务器推送:**

```json
{
    "type": "message",
    "action": "edited",
    "data": {
        "roomId": "100000001",
        "messageId": "M12345678901234567890",
        "text": "这是编辑后的内容",
        "timestamp": "2025-12-01T13:35:56Z"
    }
}
```

**说明:**

- 收到此通知后，客户端应更新界面中的消息内容
- 仅消息发送者可编辑自己的消息

---

## 5. 房间操作

### 3.1 加入房间

**客户端发送:**

```json
{
    "type": "room",
    "action": "join",
    "data": {
        "roomId": "100000001"
    }
}
```

**服务器响应 (成功):**

```json
{
    "type": "room",
    "action": "joined",
    "data": {
        "roomId": "100000001",
        "success": true
    }
}
```

**服务器响应 (失败):**

```json
{
    "type": "error",
    "action": "not_in_room",
    "data": {
        "message": "You are not a member of this room"
    }
}
```

**房间成员通知 (广播给其他成员):**

```json
{
    "type": "room_member",
    "action": "joined",
    "data": {
        "roomId": "100000001",
        "userId": "U100000002"
    }
}
```

### 3.2 离开房间

**客户端发送:**

```json
{
    "type": "room",
    "action": "leave",
    "data": {
        "roomId": "100000001"
    }
}
```

**服务器响应:**

```json
{
    "type": "room",
    "action": "left",
    "data": {
        "roomId": "100000001",
        "success": true
    }
}
```

**房间成员通知 (广播给其他成员):**

```json
{
    "type": "room_member",
    "action": "left",
    "data": {
        "roomId": "100000001",
        "userId": "U100000002"
    }
}
```

### 3.3 成员被踢出

**服务器推送 (发给被踢用户):**

```json
{
    "type": "room_member",
    "action": "kicked",
    "data": {
	"userId":"U100000000"
        "roomId": "100000001",
        "reason": "违反聊天室规则",
        "timestamp": "2025-11-30T13:35:56Z"
    }
}
```

---

## 4. 用户状态

### 4.1 更新用户状态

**客户端发送:**

```json
{
    "type": "user_status",
    "action": "update",
    "data": {
        "status": "online"
    }
}
```

**状态值说明:**

- `"online"`: 在线
- `"offline"`: 离线
- `"away"`: 离开
- `"busy"`: 忙碌

### 4.2 用户状态变更通知

**服务器推送 (广播给相关房间):**

```json
{
    "type": "user_status",
    "action": "updated",
    "data": {
        "userId": "U100000002",
        "status": "online"
    }
}
```

---

## 6. 禁言通知

### 6.1 用户被禁言（个人通知）

当用户被禁言时，会收到个人通知：

**服务器推送（发给被禁言用户）:**

```json
{
    "type": "notification",
    "action": "muted",
    "data": {
        "roomId": "100000001",
        "duration": 600,
        "muteUntil": "2025-12-01T14:00:00Z"
    }
}
```

**永久禁言通知:**

```json
{
    "type": "notification",
    "action": "muted",
    "data": {
        "roomId": "100000001",
        "duration": -1
    }
}
```

**字段说明:**

- `roomId` (string): 聊天室ID
- `duration` (number, 可选): 禁言时长（秒）
- `muteUntil` (string, 可选): 禁言结束时间（ISO 8601格式）
- `permanent` (boolean, 可选): 是否永久禁言

### 6.2 禁言系统消息（房间广播）

禁言操作会向房间所有成员广播系统消息：

**服务器推送（广播给房间所有成员）:**

```json
{
    "type": "message",
    "action": "new",
    "data": {
        "messageId": "M12345678901234567890",
        "roomId": "100000001",
	"memberId":"M_U100000003_100000004"
        "type": "system_notification",
        "text": "张三已被禁言10分钟",
        "time": "2025-12-01T12:00:00Z"
    }
}
```

**消息格式说明:**

- 定时禁言：`{昵称}已被禁言X分钟` 或 `{昵称}已被禁言X秒`（小于60秒时）
- 永久禁言：`{昵称}已被永久禁言`
- 显示用户的昵称，如无昵称则显示用户名

### 6.3 用户解除禁言（个人通知）

当用户被解除禁言时，会收到个人通知：

**服务器推送（发给被解禁用户）:**

```json
{
    "type": "notification",
    "action": "unmuted",
    "data": {
        "roomId": "100000001"
    }
}
```

### 6.4 解禁系统消息（房间广播）

解禁操作会向房间所有成员广播系统消息：

**服务器推送（广播给房间所有成员）:**

```json
{
    "type": "message",
    "action": "new",
    "data": {
        "messageId": "M12345678901234567890",
        "roomId": "100000001",
	"memberId":"M_U100000003_100000004"
        "type": "system_notification",
        "text": "张三已被解除禁言",
        "time": "2025-12-01T12:00:00Z"
    }
}
```

---

## 7. 错误消息

### 7.1 通用错误格式

**服务器推送:**

```json
{
    "type": "error",
    "action": "错误类型",
    "data": {
        "message": "错误描述信息"
    }
}
```

### 7.2 常见错误类型

**无效数据:**

```json
{
    "type": "error",
    "action": "invalid_data",
    "data": {
        "message": "Invalid message data"
    }
}
```

**未加入房间:**

```json
{
    "type": "error",
    "action": "not_in_room",
    "data": {
        "message": "You are not a member of this room"
    }
}
```

**被禁言:**

```json
{
    "type": "error",
    "action": "muted",
    "data": {
        "message": "You are muted and cannot send messages"
    }
}
```

**无效消息类型:**

```json
{
    "type": "error",
    "action": "invalid_type",
    "data": {
        "message": "Invalid message type"
    }
}
```

**缺少媒体文件:**

```json
{
    "type": "error",
    "action": "missing_media",
    "data": {
        "message": "Media URL required for this message type"
    }
}
```

**内部错误:**

```json
{
    "type": "error",
    "action": "internal_error",
    "data": {
        "message": "Database not available"
    }
}
```

---

## 完整示例代码

### JavaScript/React 示例

```javascript
class WebSocketClient {
    constructor(token) {
        this.token = token;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
    }

    connect() {
        this.ws = new WebSocket(`ws://localhost:8080/ws?token=${this.token}`);
    
        this.ws.onopen = () => {
            console.log('WebSocket 连接成功');
            this.reconnectAttempts = 0;
            this.startHeartbeat();
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket 错误:', error);
        };

        this.ws.onclose = () => {
            console.log('WebSocket 连接关闭');
            this.stopHeartbeat();
            this.reconnect();
        };
    }

    handleMessage(message) {
        switch (message.type) {
            case 'pong':
                // 心跳响应
                break;
        
            case 'message':
                if (message.action === 'new') {
                    this.onNewMessage(message.data);
                } else if (message.action === 'deleted') {
                    this.onMessageDeleted(message.data);
                } else if (message.action === 'edited') {
                    this.onMessageEdited(message.data);
                }
                break;
        
            case 'room':
                if (message.action === 'joined') {
                    this.onRoomJoined(message.data);
                } else if (message.action === 'left') {
                    this.onRoomLeft(message.data);
                }
                break;
        
            case 'room_member':
                if (message.action === 'joined') {
                    this.onMemberJoined(message.data);
                } else if (message.action === 'left') {
                    this.onMemberLeft(message.data);
                } else if (message.action === 'kicked') {
                    this.onKicked(message.data);
                }
                break;
        
            case 'user_status':
                if (message.action === 'updated') {
                    this.onUserStatusUpdated(message.data);
                }
                break;
        
            case 'typing':
                this.onTypingStatus(message.data);
                break;
        
            case 'mute':
                if (message.action === 'muted') {
                    this.onMuted(message.data);
                } else if (message.action === 'unmuted') {
                    this.onUnmuted(message.data);
                }
                break;
        
            case 'error':
                this.onError(message.action, message.data);
                break;
        }
    }

    // 发送消息
    sendMessage(roomId, text, messageType = 'text', quotedMessageId = null) {
        const message = {
            type: 'message',
            action: 'send',
            data: {
                roomId: roomId,
                messageType: messageType,
                text: text
            }
        };
    
        if (quotedMessageId) {
            message.data.quotedMessageId = quotedMessageId;
        }
    
        this.send(message);
    }

    // 加入房间
    joinRoom(roomId) {
        this.send({
            type: 'room',
            action: 'join',
            data: { roomId: roomId }
        });
    }

    // 离开房间
    leaveRoom(roomId) {
        this.send({
            type: 'room',
            action: 'leave',
            data: { roomId: roomId }
        });
    }

    // 发送正在输入状态
    sendTyping(roomId, isTyping) {
        this.send({
            type: 'typing',
            data: {
                roomId: roomId,
                typing: isTyping
            }
        });
    }

    // 标记消息已读
    markAsRead(roomId, messageId) {
        this.send({
            type: 'message',
            action: 'read',
            data: {
                roomId: roomId,
                messageId: messageId
            }
        });
    }

    // 发送数据
    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            console.error('WebSocket 未连接');
        }
    }

    // 心跳机制
    startHeartbeat() {
        this.heartbeatTimer = setInterval(() => {
            this.send({ type: 'ping' });
        }, 30000); // 每 30 秒一次
    }

    stopHeartbeat() {
        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer);
            this.heartbeatTimer = null;
        }
    }

    // 重连机制
    reconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            console.log(`尝试重连 (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
            setTimeout(() => this.connect(), 3000);
        } else {
            console.error('达到最大重连次数，放弃重连');
        }
    }

    // 关闭连接
    close() {
        this.stopHeartbeat();
        if (this.ws) {
            this.ws.close();
        }
    }

    // 事件回调（需要自己实现）
    onNewMessage(data) { console.log('新消息:', data); }
    onMessageDeleted(data) { console.log('消息删除:', data); }
    onMessageEdited(data) { console.log('消息编辑:', data); }
    onRoomJoined(data) { console.log('加入房间:', data); }
    onRoomLeft(data) { console.log('离开房间:', data); }
    onMemberJoined(data) { console.log('成员加入:', data); }
    onMemberLeft(data) { console.log('成员离开:', data); }
    onKicked(data) { console.log('被踢出:', data); }
    onUserStatusUpdated(data) { console.log('用户状态更新:', data); }
    onTypingStatus(data) { console.log('正在输入:', data); }
    onMuted(data) { console.log('被禁言:', data); }
    onUnmuted(data) { console.log('解除禁言:', data); }
    onError(action, data) { console.error('错误:', action, data); }
}

// 使用示例
const token = localStorage.getItem('jwt_token');
const wsClient = new WebSocketClient(token);
wsClient.connect();

// 发送消息
wsClient.sendMessage('100000001', '你好，世界！');

// 加入房间
wsClient.joinRoom('100000001');

// 发送正在输入
wsClient.sendTyping('100000001', true);
```

---

## 注意事项

1. **所有消息都必须是合法的 JSON 格式**
2. **`action` 字段必须正确填写**，特别是发送消息时必须设置 `"action": "send"`
3. **图片和文件消息需要先通过 HTTP API 上传文件**，获取 URL 后再通过 WebSocket 发送
4. **建议实现心跳机制**，防止连接超时
5. **建议实现重连机制**，提高连接稳定性
6. **注意处理错误消息**，给用户友好的提示
7. **连接断开时会自动设置用户离线**，重连后会自动设置在线并重新加入所有房间

## 常见问题

### Q1: 发送消息时出现 "Unknown message type/action" 错误？

**A:** 检查消息格式，确保包含正确的 `type` 和 `action` 字段。发送聊天消息时必须设置：

```json
{
    "type": "message",
    "action": "send",  // 必须有这个字段！
    "data": { ... }
}
```

### Q2: 如何发送图片或文件？

**A:** 先通过 HTTP API 上传文件，然后使用返回的 URL 发送消息：

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "image",
        "text": "图片描述",
        "mediaUrl": "http://example.com/uploads/image.jpg"
    }
}
```

### Q3: 连接后如何接收房间消息？

**A:** 连接成功后，服务器会自动将用户加入其所有房间，无需手动调用 join。但如果需要加入新房间，使用 join 操作。

### Q4: 如何实现消息回复功能？

**A:** 使用 `quotedMessageId` 字段：

```json
{
    "type": "message",
    "action": "send",
    "data": {
        "roomId": "100000001",
        "messageType": "text",
        "text": "这是回复内容",
        "quotedMessageId": "M_原消息ID"
    }
}
```

### Q5: 被禁言后还能发消息吗？

**A:** 不能。服务器会返回错误消息：

```json
{
    "type": "error",
    "action": "muted",
    "data": {
        "message": "You are muted and cannot send messages"
    }
}
```
