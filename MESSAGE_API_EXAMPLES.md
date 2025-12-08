# 历史消息 API 响应示例

## 场景演示

假设聊天室有 150 条消息，消息 ID 从 M001 到 M150（按时间顺序）

```
时间轴: M001 → M002 → ... → M148 → M149 → M150
        (最早)                            (最新)
```

---

## 示例 1: 首次进入聊天室

### 请求
```http
GET /api/v1/chatroom/100000002/messages?page=1&pageSize=50
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 响应
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M150",
        "roomId": "100000002",
        "userId": "U123456789",
        "userName": "张伟",
        "type": "text",
        "text": "这是最新的消息",
        "time": "2025-11-30T15:30:00Z",
        "isOwn": true,
        "isEdited": false,
        "editedAt": null
      },
      {
        "messageId": "M149",
        "roomId": "100000002",
        "userId": "U123456790",
        "userName": "李娜",
        "type": "text",
        "text": "倒数第二条消息",
        "time": "2025-11-30T15:29:55Z",
        "isOwn": false,
        "isEdited": false,
        "editedAt": null
      },
      // ... 共 50 条消息 ...
      {
        "messageId": "M101",
        "roomId": "100000002",
        "userId": "U123456791",
        "userName": "王芳",
        "type": "text",
        "text": "这是第 101 条消息",
        "time": "2025-11-30T14:00:00Z",
        "isOwn": false,
        "isEdited": false,
        "editedAt": null
      }
    ],
    "total": 150,
    "page": 1,
    "pageSize": 50,
    "hasMore": true
  }
}
```

### 前端处理
```javascript
const data = response.data;

// 后端返回的是 [M150, M149, ..., M101] （降序）
console.log('第一条:', data.messages[0].messageId);  // M150 (最新)
console.log('最后一条:', data.messages[49].messageId); // M101

// 前端渲染需要反转
const displayMessages = data.messages.reverse();
// 现在是 [M101, M102, ..., M150] （升序，符合聊天显示习惯）

displayMessages.forEach(msg => {
  chatContainer.appendChild(createMessageElement(msg));
});

// 保存最早的消息ID，用于后续加载更多
const oldestMessageId = displayMessages[0].messageId; // M101
```

---

## 示例 2: 向上滚动加载更多历史消息

### 请求（游标分页）
```http
GET /api/v1/chatroom/100000002/messages?before=M101&pageSize=50
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 响应
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M100",
        "roomId": "100000002",
        "userId": "U123456792",
        "userName": "赵敏",
        "type": "text",
        "text": "这是第 100 条消息",
        "time": "2025-11-30T13:59:50Z",
        "isOwn": false
      },
      {
        "messageId": "M099",
        "roomId": "100000002",
        "userId": "U123456789",
        "userName": "张伟",
        "type": "text",
        "text": "这是第 99 条消息",
        "time": "2025-11-30T13:59:45Z",
        "isOwn": true
      },
      // ... 共 50 条消息 ...
      {
        "messageId": "M051",
        "roomId": "100000002",
        "userId": "U123456790",
        "userName": "李娜",
        "type": "text",
        "text": "这是第 51 条消息",
        "time": "2025-11-30T12:00:00Z",
        "isOwn": false
      }
    ],
    "total": 150,
    "page": 1,
    "pageSize": 50,
    "hasMore": true
  }
}
```

### 前端处理
```javascript
// 后端返回 [M100, M099, ..., M051]
const olderMessages = response.data.messages.reverse();
// 反转后: [M051, M052, ..., M100]

// 在现有消息列表前面插入
currentMessages = [...olderMessages, ...currentMessages];
// 现在完整列表: [M051...M100, M101...M150]

// 更新最早消息ID
oldestMessageId = olderMessages[0].messageId; // M051

// 检查是否还有更多
if (response.data.hasMore) {
  console.log('还有更早的历史消息');
}
```

---

## 示例 3: 继续加载更早的历史

### 请求
```http
GET /api/v1/chatroom/100000002/messages?before=M051&pageSize=50
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 响应
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M050",
        "roomId": "100000002",
        "userId": "U123456789",
        "userName": "张伟",
        "type": "text",
        "text": "这是第 50 条消息",
        "time": "2025-11-30T11:59:55Z",
        "isOwn": true
      },
      // ... 共 50 条消息 ...
      {
        "messageId": "M001",
        "roomId": "100000002",
        "userId": "U123456789",
        "userName": "张伟",
        "type": "text",
        "text": "大家好，这是第一条消息！",
        "time": "2025-11-29T10:00:00Z",
        "isOwn": true
      }
    ],
    "total": 150,
    "page": 1,
    "pageSize": 50,
    "hasMore": false  // 没有更多了！
  }
}
```

---

## 示例 4: 查看第 2 页（传统分页）

### 请求
```http
GET /api/v1/chatroom/100000002/messages?page=2&pageSize=50
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 响应
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M100",
        "text": "第 100 条消息"
      },
      // ... M100 到 M51 ...
      {
        "messageId": "M051",
        "text": "第 51 条消息"
      }
    ],
    "total": 150,
    "page": 2,
    "pageSize": 50,
    "hasMore": true
  }
}
```

---

## 示例 5: 查看第 3 页

### 请求
```http
GET /api/v1/chatroom/100000002/messages?page=3&pageSize=50
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### 响应
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M050",
        "text": "第 50 条消息"
      },
      // ... M50 到 M1 ...
      {
        "messageId": "M001",
        "text": "第 1 条消息（最早的）"
      }
    ],
    "total": 150,
    "page": 3,
    "pageSize": 50,
    "hasMore": false
  }
}
```

---

## 示例 6: 回复消息

### 请求
```http
GET /api/v1/chatroom/100000002/messages?page=1&pageSize=50
```

### 响应（包含引用消息）
```json
{
  "code": 200,
  "data": {
    "messages": [
      {
        "messageId": "M150",
        "userId": "U123456789",
        "userName": "张伟",
        "type": "text",
        "text": "同意！",
        "time": "2025-11-30T15:30:00Z",
        "replyToMessageId": "M148"  // 引用了 M148
      },
      {
        "messageId": "M149",
        "userId": "U123456790",
        "userName": "李娜",
        "type": "text",
        "text": "我也觉得不错",
        "time": "2025-11-30T15:29:55Z"
      },
      {
        "messageId": "M148",
        "userId": "U123456791",
        "userName": "王芳",
        "type": "text",
        "text": "这个想法很好！",
        "time": "2025-11-30T15:29:50Z"
      }
    ]
  }
}
```

---

## 错误响应示例

### 未授权
```json
{
  "code": 401,
  "message": "未授权"
}
```

### 不在聊天室中
```json
{
  "code": 403,
  "message": "您不在该聊天室中"
}
```

### 参数错误
```json
{
  "code": 400,
  "message": "请求参数错误",
  "error": "page must be a positive integer"
}
```

---

## 字段说明

### 消息对象 (Message)
| 字段 | 类型 | 说明 |
|------|------|------|
| messageId | string | 消息唯一ID，格式：M + 15位数字 |
| roomId | string | 聊天室ID |
| userId | string | 发送者用户ID |
| userName | string | 发送者显示名称（优先昵称，其次用户名） |
| type | string | 消息类型：text/image/file/system_notification |
| text | string | 消息内容 |
| time | string | 发送时间（ISO 8601 格式） |
| isOwn | boolean | 是否是当前用户发送的 |
| isEdited | boolean | 是否已编辑 |
| editedAt | string/null | 编辑时间 |
| replyToMessageId | string | （可选）引用的消息ID |

### 响应元数据
| 字段 | 类型 | 说明 |
|------|------|------|
| total | number | 聊天室总消息数 |
| page | number | 当前页码（传统分页模式） |
| pageSize | number | 每页消息数 |
| hasMore | boolean | 是否还有更多历史消息 |

---

## 性能建议

1. **首次加载**: 使用 `?page=1&pageSize=20` 快速显示最新消息
2. **加载历史**: 使用游标分页 `?before=messageId&pageSize=50`
3. **缓存策略**: 前端缓存已加载的消息，避免重复请求
4. **虚拟滚动**: 消息数量多时使用虚拟列表优化渲染
5. **防抖处理**: 滚动加载添加防抖，避免频繁请求

## 最佳实践

```javascript
class ChatHistory {
  constructor(roomId) {
    this.roomId = roomId;
    this.messages = [];
    this.oldestId = null;
    this.newestId = null;
    this.hasMore = true;
    this.loading = false;
  }
  
  async loadInitial() {
    const data = await this.fetchMessages({ page: 1, pageSize: 20 });
    this.messages = data.messages.reverse();
    this.updateBoundaries();
    return this.messages;
  }
  
  async loadOlder() {
    if (!this.hasMore || this.loading) return [];
    
    this.loading = true;
    const data = await this.fetchMessages({ 
      before: this.oldestId, 
      pageSize: 50 
    });
    
    const older = data.messages.reverse();
    this.messages = [...older, ...this.messages];
    this.hasMore = data.hasMore;
    this.updateBoundaries();
    this.loading = false;
    
    return older;
  }
  
  addNewMessage(message) {
    this.messages.push(message);
    this.newestId = message.messageId;
  }
  
  updateBoundaries() {
    if (this.messages.length > 0) {
      this.oldestId = this.messages[0].messageId;
      this.newestId = this.messages[this.messages.length - 1].messageId;
    }
  }
  
  async fetchMessages(params) {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(
      `/api/v1/chatroom/${this.roomId}/messages?${query}`,
      { headers: { 'Authorization': `Bearer ${token}` } }
    );
    const result = await response.json();
    return result.data;
  }
}
```

