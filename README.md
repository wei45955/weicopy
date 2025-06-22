# WeiCopy - 跨设备复制粘贴工具

## 项目简介

WeiCopy是一个自托管的跨设备复制粘贴工具，允许用户在不同设备间快速共享文本、图片和文件，无需依赖第三方平台。

## 功能特点

- 简单的用户认证系统，无需复杂登录流程
- 支持文本、图片和文件的上传与共享
- 实时数据同步（无需刷新页面）
- 命令行接口支持，方便在终端环境中使用
- 账户隔离，保证数据安全
- 预留开放注册功能（当前默认关闭）
- 使用Docker容器化部署，便于迁移和管理

## 项目结构

```
├── backend/           # Go后端代码
├── frontend/          # React前端代码
├── docker/            # Docker相关配置
├── docker-compose.yml # Docker Compose配置文件
└── README.md          # 项目说明文档
```

## 使用方法

### 构建与运行

```bash
# 构建并启动服务
docker-compose up -d
```

### 命令行使用示例

```bash
# 上传文本
curl -X POST -H "Content-Type: text/plain" -H "Authorization: Bearer YOUR_TOKEN" -d "要上传的文本内容" http://your-server/api/clipboard/text

# 上传文件
curl -X POST -H "Authorization: Bearer YOUR_TOKEN" -F "file=@/path/to/your/file" http://your-server/api/clipboard/file

# 获取最新内容
curl -H "Authorization: Bearer YOUR_TOKEN" http://your-server/api/clipboard/latest > output_file
```

## 注意事项

- 默认不对外暴露端口，需要在Docker Compose配置中手动设置
- 初始用户需要手动在数据库中创建
- 默认关闭开放注册功能，可在配置中开启