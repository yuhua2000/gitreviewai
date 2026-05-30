package types

// 业务状态码（与 HTTP status 解耦，方便后续扩展）
// 成功码用 1000，错误码从 2000 开始，避免 0 值歧义
const (
	CodeSuccess       = 1000 // 成功
	CodeBadRequest    = 2001 // 请求参数错误
	CodeUnauthorized  = 2002 // 未授权
	CodeNotFound      = 2003 // 资源不存在
	CodeConflict      = 2004 // 状态冲突
	CodeInternalError = 2005 // 服务器内部错误
)

// Response 统一响应信封
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Success 构造成功响应
func Success(data any) Response {
	return Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	}
}

// Error 构造错误响应
func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
		Data:    nil,
	}
}
