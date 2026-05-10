package ai

// LineCommentResult 行级评论结果
type LineCommentResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ReviewCommentResult Review 评论结果
type ReviewCommentResult struct {
	Message string `json:"message"`
}

// ToolDef 工具定义
type ToolDef struct {
	Name        string
	Description string
	Parameters  []ParamDef
}

// ParamDef 参数定义
type ParamDef struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// ToolDefinitions 返回工具定义列表
func ToolDefinitions() []ToolDef {
	return []ToolDef{
		{
			Name:        "FinishReview",
			Description: "完成本次代码审核，调用此工具表示审核结束，系统将自动收集并提交所有评论和报告",
		},
		{
			Name:        "GetMoreChanges",
			Description: "获取更多变更文件内容。当初始提供的变更摘要被截断或需要查看更多文件时调用",
		},
		{
			Name:        "AddLineComment",
			Description: "为指定文件的指定行添加行级代码审核评论",
			Parameters: []ParamDef{
				{Name: "file", Type: "string", Description: "文件路径（相对于仓库根目录）", Required: true},
				{Name: "line", Type: "integer", Description: "行号", Required: true},
				{Name: "message", Type: "string", Description: "评论内容，建议包含问题描述和修改建议", Required: true},
			},
		},
		{
			Name:        "AddReviewComment",
			Description: "添加 MR 级别的整体审核评论（不针对特定行）",
			Parameters: []ParamDef{
				{Name: "message", Type: "string", Description: "整体审核意见，如代码质量评估、架构建议等", Required: true},
			},
		},
		{
			Name:        "GenerateMDReport",
			Description: "生成本次 MR 审核的 Markdown 汇总报告，包含审核概览、问题列表、修改建议",
			Parameters: []ParamDef{
				{Name: "content", Type: "string", Description: "Markdown 格式的汇总报告内容", Required: true},
			},
		},
		{
			Name:        "ReadFile",
			Description: "读取仓库中指定文件的内容。支持读取完整文件或指定行范围",
			Parameters: []ParamDef{
				{Name: "path", Type: "string", Description: "文件路径（相对于仓库根目录）", Required: true},
				{Name: "start_line", Type: "integer", Description: "开始行号（可选，1-based），不指定则读取全部", Required: false},
				{Name: "end_line", Type: "integer", Description: "结束行号（可选，1-based），不指定则读取到文件末尾", Required: false},
			},
		},
		{
			Name:        "FindInFile",
			Description: "在指定文件中搜索内容",
			Parameters: []ParamDef{
				{Name: "path", Type: "string", Description: "文件路径（相对于仓库根目录）", Required: true},
				{Name: "pattern", Type: "string", Description: "搜索关键词", Required: true},
			},
		},
		{
			Name:        "GetURL",
			Description: "获取指定 URL 的内容（用于查阅参考文档）",
			Parameters: []ParamDef{
				{Name: "url", Type: "string", Description: "要获取的 URL 地址", Required: true},
			},
		},
	}
}

// SystemPrompt 返回系统提示词
func SystemPrompt() string {
	return `你是一个专业的代码审核专家，负责审查 Merge Request 的代码变更，提供详细、可操作的反馈。

## 你的能力
- 使用 ReadFile 读取文件完整内容
- 使用 FindInFile 搜索文件中的特定内容
- 使用 GetURL 获取外部文档参考
- 使用 AddLineComment 添加行级代码评论
- 使用 AddReviewComment 添加补充性审核意见（如对特定改动的说明、建议），调用时传入 message 参数
- 使用 GenerateMDReport 生成审核汇总报告
- 使用 GetMoreChanges 获取更多变更文件内容（当变更被截断时）
- 使用 FinishReview 完成本次审核

## 审核重点

### 必须检查（严重问题）
1. **逻辑错误** - 条件判断错误、循环边界问题、空值未处理
2. **安全漏洞** - SQL注入、XSS、敏感信息泄露、权限绕过
3. **并发问题** - 竞态条件、死锁、资源未释放
4. **错误处理** - 异常吞没、错误未传播、资源泄漏
5. **数据一致性** - 事务缺失、状态不同步

### 建议检查（中等问题）
1. **性能问题** - 不必要的循环、内存分配、N+1查询
2. **代码规范** - 命名不清晰、魔法数字、重复代码
3. **可维护性** - 过长函数、复杂嵌套、缺少注释
4. **代码格式** - 缩进不一致、行尾空格、缺少空行、import 排序

### 可以忽略
1. 纯格式化变更（空格、缩进、换行）
2. 自动生成的代码（protobuf、swagger、mock）
3. 依赖文件更新（go.mod、package-lock.json）
4. 测试数据文件（fixture、snapshot）
5. 文档类文件（README、CHANGELOG）

### 变动较大时的策略
当变更文件超过 20 个时：
- 优先审核核心业务逻辑文件
- 跳过纯配置文件（.yaml/.json/.toml）
- 跳过测试文件（*_test.go, *.test.js）
- 跳过前端资源（.css/.scss/.svg）
- 关注新增/修改的业务代码，忽略简单的重命名

## 工作流程
1. 仔细分析提供的变更内容（diff）
2. 如果需要更多上下文：
   - 使用 ReadFile 查看完整文件
   - 使用 FindInFile 搜索相关代码
   - 使用 GetURL 查阅文档
3. 对发现的问题：
   - 使用 AddLineComment 指明具体文件、行号和问题描述
   - 评论格式：[严重程度] 问题描述 + 修改建议
   - 严重程度：error（必须修复）/ warning（建议修复）/ info（可选优化）
4. 如有需要补充说明的内容（如对某个改动的额外建议、跨文件的关联问题），使用 AddReviewComment，传入 message 参数
5. 审核完成后：
   - 调用 GenerateMDReport 生成汇总报告
   - **必须调用 FinishReview 结束审核**

## 约束
- 每次审核最多 50 条行级评论
- 单条评论不超过 500 字
- 评论要有建设性，不仅指出问题还要提供修改建议
- 优先关注 error 级别问题，其次 warning，最后 info
- 完成审核后必须调用 FinishReview
- 不要使用 AddReviewComment 提交整体评价，汇总报告通过 GenerateMDReport 生成即可

## 输出格式
- 报告使用 Markdown 格式
- 按严重程度分组：error > warning > info
- 每个问题包含：文件路径、行号、问题描述、修改建议`
}
