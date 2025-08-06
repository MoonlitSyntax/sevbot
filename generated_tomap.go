// Code generated manually to replace reflection-based structToMap; DO NOT EDIT.

package sevbot

// ToMapper 定义了可以转换为map的接口
type ToMapper interface {
	ToMap() map[string]any
}

// ToMap converts SendPrivateMessageRequest to map[string]any
func (s *SendPrivateMessageRequest) ToMap() map[string]any {
	return map[string]any{
		"user_id": s.UserID,
		"message": s.Message,
	}
}

// ToMap converts SendGroupMessageRequest to map[string]any
func (s *SendGroupMessageRequest) ToMap() map[string]any {
	return map[string]any{
		"group_id": s.GroupID,
		"message":  s.Message,
	}
}

// ToMap converts HandleFriendAddRequest to map[string]any
func (s *HandleFriendAddRequest) ToMap() map[string]any {
	result := map[string]any{
		"flag":    s.Flag,
		"approve": s.Approve,
	}
	if s.Remark != "" {
		result["remark"] = s.Remark
	}
	return result
}

// ToMap converts HandleGroupAddRequest to map[string]any
func (s *HandleGroupAddRequest) ToMap() map[string]any {
	result := map[string]any{
		"flag":     s.Flag,
		"sub_type": s.SubType,
		"approve":  s.Approve,
	}
	if s.Reason != "" {
		result["reason"] = s.Reason
	}
	return result
}

// 为简单的map[string]any类型提供ToMap方法
func toMapFromMap(m map[string]any) map[string]any {
	return m
}