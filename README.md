# GitReviewAI 🚀

基于 AI 的 GitLab Merge Request 代码审查工具。

## 🙌 致谢

本项目开发得到了 **小米 MiMo Token 计划** 提供的 API Token 支持，特此感谢！

## 📋 功能特性

- **AI 智能代码审查** - 使用 OpenAI/GPT 模型自动分析 MR 代码变更
- **行级评论** - 精准地在代码行上标注问题并提供建议
- **文件过滤** - 自动跳过测试、配置和生成的代码文件
- **批量处理** - 分批处理大文件集，避免超出模型上下文限制
- **Markdown 报告** - 生成结构化的审查总结报告
- **Webhook 集成** - 通过 GitLab Webhook 自动触发审查

## 🏗️ 架构设计

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ GitLab MR   │────▶│  Webhook    │────▶│  Reviewer   │
│   Event     │     │   Handler   │     │   Engine    │
└─────────────┘     └─────────────┘     └──────┬──────┘
                                                │
                       ┌─────────────┐         │
                       │   GitLab    │◀────────┤
                       │    API      │         │
                       └─────────────┘         │
                                                │
                       ┌─────────────┐         │
                       │ OpenAI API  │◀────────┘
                       │  (MiMo/GPT) │
                       └─────────────┘
```

## 🚀 快速开始

### 1. 环境要求

- Go 1.21+
- GitLab 账号（需配置 API Token）
- OpenAI 兼容的 API（支持自定义基础 URL）

### 2. 配置

复制项目中的 `config.yaml` 文件并根据你的环境修改以下关键参数：

```yaml
# GitLab 配置
gitlab_url: "https://gitlab.com"          # 私有部署请改为你的 GitLab 地址
gitlab_token: "glpat-xxxxxxxxx"           # 需具备 api 权限

# OpenAI 配置
openai_api_key: "sk-xxxxxxxxx"            # 你的 API Key
openai_model: "gpt-4o"                    # 使用的模型
openai_base_url: "https://api.openai.com/v1"  # 兼容自定义网关

# 服务配置
port: 8080                                # 服务监听端口
webhook_token: "your-webhook-secret"      # GitLab Webhook 验证密钥（可选）
```

其他选项（如忽略路径、日志级别等）可按需调整。完整配置请参考仓库中的 `config.yaml` 文件。

### 3. 启动服务

```bash
# 克隆项目
git clone https://github.com/yuhua2000/gitreviewai.git
cd gitreviewai

# 安装依赖
go mod tidy

# 修改配置文件（按上节说明填写）
vi config.yaml

# 运行服务
go run cmd/server/main.go
```

## 使用Docker运行

### 构建Docker镜像

```bash
docker build -t gitreviewai .
```

### 运行容器（使用配置文件映射）

```bash
docker run -d \
  --name gitreviewai \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  gitreviewai
```

### 使用Docker Compose（推荐）

创建 `docker-compose.yml` 文件：

```yaml
version: '3.8'
services:
  gitreviewai:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml
```

然后运行：

```bash
docker-compose up -d
```

### 配置文件说明

请确保您的 `config.yaml` 文件包含以下必要配置：

```yaml
gitlab_token: "your_gitlab_token"
openai_api_key: "your_openai_api_key"
port: 8080
# 其他配置项...
```

服务启动后，即可在 GitLab 项目中配置 Webhook 指向 `http://你的IP:8080/webhook`。

## 📊 审查报告示例

审查完成后，AI 会在 MR 中发布结构化报告：

```markdown
# MR 审查报告

**审查时间：** 2024-01-15 09:30:25  
**行级评论数：** 12  
**总结评论数：** 3  

---

## 🔴 错误（必须修复）

### 1. 空指针风险
- **文件：** `internal/service/user.go:45`
- **问题：** 在未检查 `user` 是否为 nil 的情况下访问字段
- **建议：** 添加空值检查：`if user == nil { return nil, errors.New("user not found") }`

## 🟡 警告（建议修复）

### 2. 性能优化
- **文件：** `internal/dao/order.go:128`
- **问题：** 循环中重复创建数据库连接
- **建议：** 将连接创建移到循环外部

## 🟢 信息（可选优化）

### 3. 代码规范
- **文件：** `internal/utils/helper.go:67`
- **建议：** 函数过长，考虑拆分为更小的函数

---
*此报告由 GitReviewAI 自动生成，供开发者参考。*
```

## 🛠️ 开发路线图

- [x] GitLab Webhook 集成
- [x] AI 行级评论功能
- [x] Markdown 报告生成
- [ ] 自定义审查规则
- [ ] 审查历史统计
- [ ] 多语言优化

## 🤝 贡献指南

欢迎提交 issue 和 PR！请确保：
1. 代码使用 `go fmt` 格式化
2. 添加必要的单元测试
3. 更新相关文档

## 📄 许可证

MIT License © 2024 GitReviewAI