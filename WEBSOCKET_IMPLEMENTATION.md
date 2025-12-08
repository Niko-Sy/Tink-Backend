# WebSocket 聊天功能与历史消息实现文档

## 概述

本文档描述了 Tink 聊天室后端的 WebSocket 实时通信功能和历史消息管理的完整实现。

## 历史消息分页设计（快速参考）

### 核心设计原则
**page=1 返回最新消息，page 越大返回越早的历史消息**

```
时间轴:  [最早] ←←←←←←←←←←←←←←← [最新]
                                    ↑
消息ID:  M1 ← M2 ← M3 ... ← M98 ← M99 ← M100
         
查询结果（降序）:
page=1:  [M100, M99, M98, ..., M51]  ← 最新 50 条
page=2:  [M50, M49, M48, ..., M1]    ← 接下来 50 条（更早）

游标分页:
?page=1              → [M100...M51]
?before=M51          → [M50...M1]   ← 获取 M51 之前（更早）的消息
?before=M1           → []           ← 没有更早的消息了
```

### 前端使用流程
```
1. 首次加载:   GET /messages?page=1&pageSize=50
   获得:      [M100, M99, ..., M51] （降序）
   前端反转:   [M51, M52, ..., M100] （升序显示）
   
2. 向上滚动:   GET /messages?before=M51&pageSize=50
   获得:      [M50, M49, ..., M1] （降序）
   前端反转:   [M1, M2, ..., M50] （升序显示）
   插入位置:   在消息列表顶部
```

### 为什么这样设计？
✅ **用户体验好**: 进入聊天室立即看到最新消息，无需滚动  
✅ **避免数据偏移**: 游标分页确保加载历史时不受新消息影响  
✅ **性能优化**: 数据库按时间索引降序查询最快  
✅ **符合直觉**: 向上滚动 = 查看更早的历史


## 已实现的功能

### 1. HTTP REST API 接口

#### 1.1 发送消息
- **路由**: `POST /api/v1/chatroom/:roomid/messages`
- **文件**: `api/messages/sendmsg.go`
- **功能**: 
  - 验证用户是否在聊天室中
  - 检查用户禁言状态
  - 创建消息并保存到数据库
  - 通过 WebSocket 实时广播消息到房间所有在线成员
  - 更新聊天室最后活跃时间

#### 1.2 获取消息历史
- **路由**: `GET /api/v1/chatroom/:roomid/messages`
- **文件**: `api/messages/history.go`
- **功能**:
  - 支持传统分页 (`?page=1&pageSize=50`)
  - 支持游标分页 (`?before=messageId&pageSize=50`)
  - 验证用户权限（必须是聊天室成员）
  - 返回消息列表，包含发送者信息

##### 分页设计详解

**设计原则：page=1 返回最新消息**

1. **传统分页模式** (`?page=1&pageSize=50`):
   ```
   page=1: [消息50(最新), 消息49, ..., 消息1(较早)]  <- 降序排列
   page=2: [消息100, 消息99, ..., 消息51]
   page=3: [消息150, 消息149, ..., 消息101]
   ```
   - **优点**: 用户进入聊天室立即看到最新消息
   - **缺点**: 有新消息插入时，翻页会出现数据偏移
   - **适用场景**: 首次加载聊天记录

2. **游标分页模式** (`?before=messageId&pageSize=50`):
   ```
   首次: ?page=1              -> [M50, M49, ..., M1]
   滚动: ?before=M1&pageSize=50 -> [M100, M99, ..., M51]  <- 加载更早的历史
   滚动: ?before=M51            -> [M150, M149, ..., M101]
   ```
   - **优点**: 避免数据偏移，适合无限滚动
   - **缺点**: 无法跳页
   - **适用场景**: 向上滚动加载更多历史消息

3. **前端渲染建议**:
   ```javascript
   // 后端返回的消息是降序（最新在前）
   const messages = response.data.messages;  // [M50, M49, M48, ...]
   
   // 前端需要反转后渲染（最早的在上方，最新的在下方）
   const displayMessages = messages.reverse();  // [M48, M49, M50, ...]
   
   // 或者直接从数组末尾开始渲染
   messages.forEach(msg => chatContainer.prepend(renderMessage(msg)));
   ```

4. **数据库查询**:
   - `ORDER BY sent_at DESC` - 按时间降序
   - 总是返回最新的消息在前面
   - 游标分页使用 `WHERE sent_at < (SELECT sent_at FROM messages WHERE message_id = $before)`

#### 1.3 编辑消息
- **路由**: `POST /api/v1/chatroom/:roomid/messages/:messageid/edit`
- **文件**: `api/messages/editmsg.go`
- **功能**:
  - 验证用户权限（消息发送者或管理员）
  - 更新消息内容
  - 通过 WebSocket 广播编辑事件

#### 1.4 删除消息
- **路由**: `POST /api/v1/chatroom/:roomid/messages/:messageid/delete`
- **文件**: `api/messages/deletemsg.go`
- **功能**:
  - 验证用户权限（消息发送者或管理员）
  - 软删除消息（改为系统提示）
  - 通过 WebSocket 广播删除事件

### 2. WebSocket 实时通信

#### 2.1 连接管理
- **路由**: `GET /ws?token=<jwt_token>`
- **文件**: `api/websocketmsg/websocket.go`
- **功能**:
  - JWT Token 认证
  - 维护用户在线状态
  - 自动订阅用户加入的所有聊天室
  - 断线重连支持
  - 心跳检测 (ping/pong)

#### 2.2 消息广播
- **函数**: `BroadcastToRoom(roomID string, msg WSMessage)`
- **功能**: 向指定聊天室的所有在线成员广播消息

#### 2.3 支持的 WebSocket 消息类型

##### 客户端发送消息
```json
{
  "type": "message",
  "action": "send",
  "data": {
    "roomId": "100000002",
    "messageType": "text",
    "text": "消息内容",
    "quotedMessageId": "M001" // 可选，回复消息
  }
}
```

##### 服务端推送新消息
```json
{
  "type": "message",
  "action": "new",
  "data": {
    "messageId": "M000000000000001",
    "roomId": "100000002",
    "userId": "U123456790",
    "userName": "李娜",
    "type": "text",
    "text": "消息内容",
    "time": "2025-11-23T10:00:00Z"
  }
}
```

##### 消息编辑通知
```json
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

##### 消息删除通知
```json
{
  "type": "message",
  "action": "delete",
  "data": {
    "roomId": "100000002",
    "messageId": "M001"
  }
}
```

##### 心跳包
```json
// 客户端发送
{ "type": "ping" }

// 服务端响应
{ "type": "pong" }
```

## 架构设计

### 数据流程

```
客户端 --> HTTP API --> 验证权限 --> 数据库操作 --> WebSocket 广播 --> 所有在线客户端
   |                                                    |
   +------------ WebSocket 连接 <----------------------+
```

### 核心组件

1. **Hub**: 管理所有 WebSocket 连接和聊天室订阅关系
   - `Clients map[string]*Client`: userId -> WebSocket 连接
   - `Rooms map[string]map[string]bool`: roomId -> userIds set

2. **Client**: 代表单个 WebSocket 连接
   - `Conn`: WebSocket 连接对象
   - `UserID`: 用户 ID
   - `Send chan []byte`: 消息发送通道

3. **消息处理流程**:
   - `readPump()`: 读取客户端消息
   - `writePump()`: 向客户端发送消息
   - `handleMessage()`: 处理各种类型的消息

## 权限验证

### 消息操作权限

1. **发送消息**:
   - 必须是聊天室成员
   - 不能被禁言（全局禁言或聊天室禁言）

2. **编辑消息**:
   - 消息发送者本人，或
   - 聊天室管理员/房主

3. **删除消息**:
   - 消息发送者本人，或
   - 聊天室管理员/房主

### 数据库查询函数

- `IsUserInChatroom`: 检查用户是否在聊天室
- `CanUserSendMessageInRoom`: 检查用户是否可以发送消息
- `GetUserChatroomMembership`: 获取用户在聊天室的成员信息
- `GetMessageByID`: 获取消息详情

## 数据库支持

### 主要查询

- `CreateMessage`: 创建消息
- `UpdateMessage`: 更新消息
- `DeleteMessageSoft`: 软删除消息
- `GetMessagesByRoom`: 分页获取消息历史
- `GetMessagesBefore`: 游标分页获取消息
- `GetMessageWithSender`: 获取消息及发送者信息
- `CountMessagesInRoom`: 统计消息数量

## 使用示例

### 前端连接 WebSocket

```javascript
const token = localStorage.getItem('token');
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.onopen = () => {
  console.log('WebSocket 连接成功');
  
  // 发送心跳
  setInterval(() => {
    ws.send(JSON.stringify({ type: 'ping' }));
  }, 30000);
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  
  if (msg.type === 'message' && msg.action === 'new') {
    // 处理新消息
    appendMessage(msg.data);
  } else if (msg.type === 'message' && msg.action === 'edit') {
    // 处理消息编辑
    updateMessage(msg.data.messageId, msg.data.newText);
  } else if (msg.type === 'message' && msg.action === 'delete') {
    // 处理消息删除
    removeMessage(msg.data.messageId);
  }
};
```

### 发送消息（通过 WebSocket）

```javascript
ws.send(JSON.stringify({
  type: 'message',
  action: 'send',
  data: {
    roomId: '100000002',
    messageType: 'text',
    text: '你好，大家！'
  }
}));
```

### 获取历史消息（HTTP API）

#### 场景 1: 首次进入聊天室（获取最新消息）
```javascript
// 获取最新的 50 条消息
const response = await fetch('/api/v1/chatroom/100000002/messages?page=1&pageSize=50', {
  headers: { 'Authorization': `Bearer ${token}` }
});

const data = await response.json();
// data.data.messages 是降序数组：[最新消息, ..., 较早消息]

// 前端渲染时需要反转
const messages = data.data.messages.reverse(); // [较早消息, ..., 最新消息]
messages.forEach(msg => {
  appendMessage(msg); // 按时间顺序显示
});
```

#### 场景 2: 向上滚动加载更多历史消息（推荐使用游标分页）
```javascript
let oldestMessageId = null; // 当前已加载的最早消息ID

// 首次加载
async function loadInitialMessages() {
  const response = await fetch('/api/v1/chatroom/100000002/messages?page=1&pageSize=50', {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const data = await response.json();
  
  if (data.data.messages.length > 0) {
    // 记录最早的消息ID（数组最后一个，因为是降序）
    oldestMessageId = data.data.messages[data.data.messages.length - 1].messageId;
    
    // 反转后渲染
    renderMessages(data.data.messages.reverse());
  }
}

// 滚动到顶部时加载更多
async function loadMoreHistory() {
  if (!oldestMessageId) return;
  
  const response = await fetch(
    `/api/v1/chatroom/100000002/messages?before=${oldestMessageId}&pageSize=50`,
    { headers: { 'Authorization': `Bearer ${token}` } }
  );
  const data = await response.json();
  
  if (data.data.messages.length > 0) {
    // 更新最早消息ID
    oldestMessageId = data.data.messages[data.data.messages.length - 1].messageId;
    
    // 在顶部插入历史消息
    prependMessages(data.data.messages.reverse());
  }
  
  // 检查是否还有更多历史
  return data.data.hasMore;
}

// 监听滚动事件
chatContainer.addEventListener('scroll', (e) => {
  if (e.target.scrollTop === 0) { // 滚动到顶部
    loadMoreHistory();
  }
});
```

#### 场景 3: 跳页查看历史记录
```javascript
// 查看第 3 页的历史消息
const response = await fetch('/api/v1/chatroom/100000002/messages?page=3&pageSize=50', {
  headers: { 'Authorization': `Bearer ${token}` }
});

const data = await response.json();
console.log(`总消息数: ${data.data.total}`);
console.log(`当前页: ${data.data.page}`);
console.log(`是否还有更多: ${data.data.hasMore}`);
```

#### 完整的消息列表组件示例
```javascript
class MessageList {
  constructor(roomId, token) {
    this.roomId = roomId;
    this.token = token;
    this.messages = [];
    this.oldestMessageId = null;
    this.loading = false;
  }
  
  async loadInitial() {
    const response = await fetch(
      `/api/v1/chatroom/${this.roomId}/messages?page=1&pageSize=50`,
      { headers: { 'Authorization': `Bearer ${this.token}` } }
    );
    const data = await response.json();
    
    if (data.code === 200 && data.data.messages.length > 0) {
      // 后端返回降序，前端需要反转为升序显示
      this.messages = data.data.messages.reverse();
      this.oldestMessageId = this.messages[0].messageId; // 第一个是最早的
      this.render();
    }
  }
  
  async loadMore() {
    if (this.loading || !this.oldestMessageId) return;
    
    this.loading = true;
    const response = await fetch(
      `/api/v1/chatroom/${this.roomId}/messages?before=${this.oldestMessageId}&pageSize=50`,
      { headers: { 'Authorization': `Bearer ${this.token}` } }
    );
    const data = await response.json();
    
    if (data.code === 200 && data.data.messages.length > 0) {
      const olderMessages = data.data.messages.reverse();
      this.messages = [...olderMessages, ...this.messages]; // 在前面插入
      this.oldestMessageId = olderMessages[0].messageId;
      this.render();
    }
    
    this.loading = false;
    return data.data.hasMore;
  }
  
  addNewMessage(message) {
    // WebSocket 收到新消息，追加到末尾
    this.messages.push(message);
    this.render();
  }
  
  render() {
    // 渲染 this.messages（已经是升序：早 -> 晚）
    const container = document.getElementById('messages');
    container.innerHTML = this.messages
      .map(msg => this.renderMessage(msg))
      .join('');
  }
  
  renderMessage(msg) {
    return `
      <div class="message ${msg.isOwn ? 'own' : ''}">
        <span class="user">${msg.userName}</span>
        <span class="text">${msg.text}</span>
        <span class="time">${new Date(msg.time).toLocaleTimeString()}</span>
      </div>
    `;
  }
}

// 使用
const messageList = new MessageList('100000002', token);
await messageList.loadInitial();

// 滚动加载
chatContainer.addEventListener('scroll', async (e) => {
  if (e.target.scrollTop < 100) {
    const hasMore = await messageList.loadMore();
    if (!hasMore) {
      console.log('没有更多历史消息了');
    }
  }
});

// WebSocket 新消息
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'message' && msg.action === 'new') {
    messageList.addNewMessage(msg.data);
  }
};
```

### 编辑消息（HTTP API）

```javascript
fetch('/api/v1/chatroom/100000002/messages/M123/edit', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    text: '编辑后的消息内容'
  })
});
```

## 性能优化建议

### 已实现
1. ✅ WebSocket 连接池管理
2. ✅ 消息通道缓冲（256 消息）
3. ✅ 异步更新聊天室活跃时间
4. ✅ 读写分离（readPump / writePump）
5. ✅ 心跳保活机制

### 待优化
1. 📋 Redis 缓存热点消息
2. 📋 消息队列处理广播
3. 📋 水平扩展支持（多 WebSocket 服务器）
4. 📋 消息持久化异步化
5. 📋 限流保护（防止消息轰炸）

## 安全性

### 已实现
1. ✅ JWT Token 认证
2. ✅ 权限验证（聊天室成员身份）
3. ✅ 禁言检查
4. ✅ 软删除（保留消息审计）
5. ✅ CORS 控制（需在生产环境加强）

### 建议加强
1. 📋 消息内容过滤（敏感词、XSS）
2. 📋 频率限制（每分钟最多发送 N 条消息）
3. 📋 IP 白名单
4. 📋 消息大小限制

## 故障处理

### WebSocket 断线重连
- 客户端检测到连接断开后，应重新连接并自动订阅房间
- 服务端会在用户重连时自动加载其聊天室列表并订阅

### 消息丢失处理
- 客户端应在发送消息时生成临时 ID
- 收到服务端确认后替换为正式 messageId
- 未收到确认的消息应标记为"发送失败"

### 消息顺序保证
- 所有消息按数据库时间戳 `sent_at` 排序
- 客户端应根据 `messageId` 去重
- 建议使用游标分页避免消息插入导致的重复

## 测试建议

### 单元测试
- 消息权限验证逻辑
- WebSocket 消息序列化/反序列化
- 并发连接管理

### 集成测试
- 多用户同时发送消息
- 断线重连场景
- 消息编辑/删除广播

### 压力测试
- 单聊天室并发用户数
- 消息发送频率
- WebSocket 连接数上限

## 未来扩展

1. **文件消息支持**: 图片、文件上传和预览
2. **@提及功能**: 支持 @用户 通知
3. **已读回执**: 显示消息已读状态
4. **消息搜索**: 全文搜索历史消息
5. **消息表情反应**: 点赞、表情回应
6. **语音/视频**: WebRTC 实时音视频通话

## 相关文件

### API 层
- `api/messages/sendmsg.go` - 发送消息
- `api/messages/history.go` - 消息历史
- `api/messages/editmsg.go` - 编辑消息
- `api/messages/deletemsg.go` - 删除消息
- `api/websocketmsg/websocket.go` - WebSocket 处理

### 数据库层
- `db/message.sql.go` - 消息相关数据库操作
- `dbsql/query/message.sql` - SQL 查询定义

### 路由配置
- `main.go` - 路由注册和初始化

## 更新日志

### 2025-11-30
- ✅ 实现消息发送 HTTP API
- ✅ 实现消息历史查询（支持分页和游标）
- ✅ 实现消息编辑功能
- ✅ 实现消息删除功能
- ✅ 完善 WebSocket 广播机制
- ✅ 添加权限验证逻辑
- ✅ 注册所有消息路由

---

**维护者**: Tink Backend Team  
**最后更新**: 2025-11-30

