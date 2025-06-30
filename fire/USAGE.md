# 使用说明

## 正确的启动方式

### 1. 启动服务器

**重要：必须先启动服务器，再访问HTML文件！**

选择以下任一方式启动服务器：

#### 方式A：Go服务器（推荐）
```bash
go run server.go
```

#### 方式B：Python服务器
```bash
python3 server.py
```

### 2. 访问应用

**正确方式：**
在浏览器中打开 `http://localhost:8000/fire.html`

**错误方式：**
❌ 直接双击 `fire.html` 文件
❌ 使用 `file://` 协议打开

## 为什么不能直接打开文件？

当直接通过文件系统打开HTML文件时：
- 浏览器使用 `file://` 协议
- 无法进行网络请求
- 会触发CORS错误
- 数据保存功能无法正常工作

## 如果看到CORS错误

1. **确保服务器正在运行**
   ```bash
   # 检查服务器是否运行
   curl http://localhost:8000
   ```

2. **使用正确的URL访问**
   - ✅ `http://localhost:8000/fire.html`
   - ❌ `file:///Users/edy/Desktop/fire/fire.html`

3. **检查浏览器控制台**
   - 按F12打开开发者工具
   - 查看Console标签页的错误信息

## 数据保存机制

- **HTTP服务器模式**：数据保存到 `fire.json` 文件
- **文件系统模式**：数据保存到浏览器localStorage
- **自动保存**：输入数据后自动保存
- **双重保障**：服务器失败时自动使用localStorage

## 故障排除

### 问题1：服务器启动失败
```bash
# 检查端口是否被占用
lsof -i :8000

# 使用其他端口
go run server.go -port 8080
```

### 问题2：无法访问页面
```bash
# 检查服务器状态
curl -I http://localhost:8000

# 检查防火墙设置
```

### 问题3：数据保存失败
- 检查 `fire.json` 文件权限
- 查看服务器日志
- 检查磁盘空间

## 开发模式

如果需要修改代码，可以启用自动重载：

```bash
# Go版本（需要安装air）
air

# Python版本（需要安装watchdog）
watchdog -p . -c "python3 server.py"
```

## 生产部署

```bash
# 编译Go服务器
go build -o fire-server server.go

# 运行编译后的服务器
./fire-server
``` 