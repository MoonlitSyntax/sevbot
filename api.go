package sevbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// APIResponse OneBot标准API响应格式
type APIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
	Echo    string          `json:"echo,omitempty"`
}

// APIClient 统一API客户端，直接封装Adapter
type APIClient struct {
	adapter Adapter
	logger  *slog.Logger
	timeout time.Duration
}

// NewAPIClient 创建API客户端
func NewAPIClient(adapter Adapter) *APIClient {
	return &APIClient{
		adapter: adapter,
		logger:  slog.Default(),
		timeout: 30 * time.Second,
	}
}

// callAPI 内部API调用方法，处理OneBot协议兼容性
func (c *APIClient) callAPI(ctx context.Context, action string, params any, result any) error {
	// 设置默认超时
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
	}

	// 转换参数
	paramMap, err := c.structToMap(params)
	if err != nil {
		return fmt.Errorf("convert params failed: %w", err)
	}

	// 调用适配器
	data, err := c.adapter.CallAction(action, paramMap)
	if err != nil {
		c.logger.Error("API call failed", "action", action, "error", err)
		return err
	}

	// 解析响应
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}

	// OneBot协议兼容性处理：支持status为空的情况
	if resp.Status != "ok" && resp.Status != "" {
		c.logger.Error("API returned error",
			"action", action,
			"status", resp.Status,
			"retcode", resp.RetCode,
			"message", resp.Message)
		return NewBotError(ErrorTypeAPI, resp.RetCode, resp.Message, nil)
	}

	// 检查retcode
	if resp.Status == "" && resp.RetCode != 0 {
		c.logger.Error("API returned error",
			"action", action,
			"status", resp.Status,
			"retcode", resp.RetCode,
			"message", resp.Message)
		return NewBotError(ErrorTypeAPI, resp.RetCode, resp.Message, nil)
	}

	// 解析数据
	if result != nil && len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, result); err != nil {
			return fmt.Errorf("parse response data failed: %w", err)
		}
	}

	return nil
}

func (c *APIClient) structToMap(v any) (map[string]any, error) {
	if v == nil {
		return make(map[string]any), nil
	}

	if m, ok := v.(map[string]any); ok {
		return m, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

type SendPrivateMessageRequest struct {
	UserID  int64        `json:"user_id"`
	Message MessageChain `json:"message"`
}

type SendGroupMessageRequest struct {
	GroupID int64        `json:"group_id"`
	Message MessageChain `json:"message"`
}

// SendPrivateMessage 发送私聊消息
func (c *APIClient) SendPrivateMessage(ctx context.Context, userID int64, message MessageChain) (*SendMessageResponse, error) {
	req := &SendPrivateMessageRequest{
		UserID:  userID,
		Message: message,
	}
	var resp SendMessageResponse
	err := c.callAPI(ctx, "send_private_msg", req, &resp)
	return &resp, err
}

// SendGroupMessage 发送群消息
func (c *APIClient) SendGroupMessage(ctx context.Context, groupID int64, message MessageChain) (*SendMessageResponse, error) {
	req := &SendGroupMessageRequest{
		GroupID: groupID,
		Message: message,
	}
	var resp SendMessageResponse
	err := c.callAPI(ctx, "send_group_msg", req, &resp)
	return &resp, err
}

// DeleteMessage 撤回消息
func (c *APIClient) DeleteMessage(ctx context.Context, messageID int32) error {
	params := map[string]any{"message_id": messageID}
	return c.callAPI(ctx, "delete_msg", params, nil)
}

// GetMessage 获取消息
func (c *APIClient) GetMessage(ctx context.Context, messageID int32) (*MessageInfo, error) {
	params := map[string]any{"message_id": messageID}
	var resp MessageInfo
	err := c.callAPI(ctx, "get_msg", params, &resp)
	return &resp, err
}

// GetGroupList 获取群列表
func (c *APIClient) GetGroupList(ctx context.Context) ([]Group, error) {
	var resp []Group
	err := c.callAPI(ctx, "get_group_list", nil, &resp)
	return resp, err
}

// GetGroupInfo 获取群信息
func (c *APIClient) GetGroupInfo(ctx context.Context, groupID int64, noCache bool) (*Group, error) {
	params := map[string]any{
		"group_id": groupID,
		"no_cache": noCache,
	}
	var resp Group
	err := c.callAPI(ctx, "get_group_info", params, &resp)
	return &resp, err
}

// GetGroupMemberList 获取群成员列表
func (c *APIClient) GetGroupMemberList(ctx context.Context, groupID int64) ([]GroupMember, error) {
	params := map[string]any{"group_id": groupID}
	var resp []GroupMember
	err := c.callAPI(ctx, "get_group_member_list", params, &resp)
	return resp, err
}

// GetGroupMemberInfo 获取群成员信息
func (c *APIClient) GetGroupMemberInfo(ctx context.Context, groupID, userID int64, noCache bool) (*GroupMember, error) {
	params := map[string]any{
		"group_id": groupID,
		"user_id":  userID,
		"no_cache": noCache,
	}
	var resp GroupMember
	err := c.callAPI(ctx, "get_group_member_info", params, &resp)
	return &resp, err
}

// SetGroupKick 群组踢人
func (c *APIClient) SetGroupKick(ctx context.Context, groupID, userID int64, rejectAddRequest bool) error {
	params := map[string]any{
		"group_id":           groupID,
		"user_id":            userID,
		"reject_add_request": rejectAddRequest,
	}
	return c.callAPI(ctx, "set_group_kick", params, nil)
}

// SetGroupBan 群组单人禁言
func (c *APIClient) SetGroupBan(ctx context.Context, groupID, userID int64, duration int32) error {
	params := map[string]any{
		"group_id": groupID,
		"user_id":  userID,
		"duration": duration,
	}
	return c.callAPI(ctx, "set_group_ban", params, nil)
}

// GetFriendList 获取好友列表
func (c *APIClient) GetFriendList(ctx context.Context) ([]Friend, error) {
	var resp []Friend
	err := c.callAPI(ctx, "get_friend_list", nil, &resp)
	return resp, err
}

// GetStrangerInfo 获取陌生人信息
func (c *APIClient) GetStrangerInfo(ctx context.Context, userID int64, noCache bool) (*Stranger, error) {
	params := map[string]any{
		"user_id":  userID,
		"no_cache": noCache,
	}
	var resp Stranger
	err := c.callAPI(ctx, "get_stranger_info", params, &resp)
	return &resp, err
}

// ==================== 请求处理API ====================

type HandleFriendAddRequest struct {
	Flag    string `json:"flag"`
	Approve bool   `json:"approve"`
	Remark  string `json:"remark,omitempty"`
}

type HandleGroupAddRequest struct {
	Flag    string `json:"flag"`
	SubType string `json:"sub_type"`
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

// SetFriendAddRequest 处理加好友请求
func (c *APIClient) SetFriendAddRequest(ctx context.Context, req *HandleFriendAddRequest) error {
	return c.callAPI(ctx, "set_friend_add_request", req, nil)
}

// SetGroupAddRequest 处理加群请求／邀请
func (c *APIClient) SetGroupAddRequest(ctx context.Context, req *HandleGroupAddRequest) error {
	return c.callAPI(ctx, "set_group_add_request", req, nil)
}

// ReplyTo 回复指定事件
func (c *APIClient) ReplyTo(ctx context.Context, event Event, message MessageChain) error {
	switch e := event.(type) {
	case *PrivateMessage:
		_, err := c.SendPrivateMessage(ctx, e.UserID, message)
		return err
	case *GroupMessage:
		_, err := c.SendGroupMessage(ctx, e.GroupID, message)
		return err
	default:
		return fmt.Errorf("unsupported event type for reply: %T", event)
	}
}

// ReplyTextTo 回复文本消息
func (c *APIClient) ReplyTextTo(ctx context.Context, event Event, text string) error {
	return c.ReplyTo(ctx, event, TextMessage(text))
}

// ReplyTextfTo 回复格式化文本消息
func (c *APIClient) ReplyTextfTo(ctx context.Context, event Event, format string, args ...interface{}) error {
	return c.ReplyTextTo(ctx, event, fmt.Sprintf(format, args...))
}

// ApproveRequest 批准请求
func (c *APIClient) ApproveRequest(ctx context.Context, event Event, remark string) error {
	switch e := event.(type) {
	case *FriendRequest:
		req := &HandleFriendAddRequest{
			Flag:    e.Flag,
			Approve: true,
			Remark:  remark,
		}
		return c.SetFriendAddRequest(ctx, req)
	case *GroupRequest:
		req := &HandleGroupAddRequest{
			Flag:    e.Flag,
			SubType: e.SubType,
			Approve: true,
			Reason:  remark,
		}
		return c.SetGroupAddRequest(ctx, req)
	default:
		return fmt.Errorf("unsupported event type for approve: %T", event)
	}
}

// RejectRequest 拒绝请求
func (c *APIClient) RejectRequest(ctx context.Context, event Event, reason string) error {
	switch e := event.(type) {
	case *FriendRequest:
		req := &HandleFriendAddRequest{
			Flag:    e.Flag,
			Approve: false,
			Remark:  reason,
		}
		return c.SetFriendAddRequest(ctx, req)
	case *GroupRequest:
		req := &HandleGroupAddRequest{
			Flag:    e.Flag,
			SubType: e.SubType,
			Approve: false,
			Reason:  reason,
		}
		return c.SetGroupAddRequest(ctx, req)
	default:
		return fmt.Errorf("unsupported event type for reject: %T", event)
	}
}
