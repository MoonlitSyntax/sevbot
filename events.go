package sevbot

import (
	"encoding/json"
	"fmt"
)

// PostType 定义了上报类型
const (
	PostTypeMessage   = "message"
	PostTypeNotice    = "notice"
	PostTypeRequest   = "request"
	PostTypeMetaEvent = "meta_event"
)

// MessageType 定义了消息类型
const (
	MessageTypePrivate = "private"
	MessageTypeGroup   = "group"
)

// NoticeType 定义了通知类型
const (
	NoticeTypeGroupUpload   = "group_upload"
	NoticeTypeGroupAdmin    = "group_admin"
	NoticeTypeGroupDecrease = "group_decrease"
	NoticeTypeGroupIncrease = "group_increase"
	NoticeTypeGroupBan      = "group_ban"
	NoticeTypeFriendAdd     = "friend_add"
	NoticeTypeGroupRecall   = "group_recall"
	NoticeTypeFriendRecall  = "friend_recall"
	NoticeTypeNotify        = "notify"
)

const (
	// SubTypeGroupAdmin
	SubTypeGroupAdminSet   = "set"   // 设置管理员
	SubTypeGroupAdminUnset = "unset" // 取消管理员

	// SubTypeGroupDecrease
	SubTypeGroupDecreaseLeave  = "leave"   // 主动退群
	SubTypeGroupDecreaseKick   = "kick"    // 成员被踢
	SubTypeGroupDecreaseKickMe = "kick_me" // 机器人被踢

	// SubTypeGroupIncrease
	SubTypeGroupIncreaseApprove = "approve" // 通过加群申请
	SubTypeGroupIncreaseInvite  = "invite"  // 被邀请入群

	// SubTypeGroupBan
	SubTypeGroupBanBan     = "ban"      // 禁言
	SubTypeGroupBanLiftBan = "lift_ban" // 解除禁言

	// SubTypeNotify
	SubTypeNotifyPoke      = "poke"       // 戳一戳
	SubTypeNotifyLuckyKing = "lucky_king" // 幸运王
	SubTypeNotifyHonor     = "honor"      // 荣誉通知
)

// RequestType 定义了请求类型
const (
	RequestTypeFriend = "friend"
	RequestTypeGroup  = "group"
)

const (
	// SubTypeRequest
	SubTypeGroupRequestAdd    = "add"    // 加群请求
	SubTypeGroupRequestInvite = "invite" // 邀请机器人入群
)

// MetaEventType 定义了元事件类型
const (
	MetaEventTypeLifecycle = "lifecycle"
	MetaEventTypeHeartbeat = "heartbeat"
)

const (
	SubTypeLifecycleEnable  = "enable"  // Bot 启用
	SubTypeLifecycleDisable = "disable" // Bot 停用
	SubTypeLifecycleConnect = "connect" // WebSocket 连接成功
)

// Event 是所有事件都必须实现的接口
type Event interface {
	// isEvent 是一个占位方法，用于确保只有此包中的类型可以实现此接口
	isEvent()
}

// BaseEvent 包含了所有事件上报的通用字段
type BaseEvent struct {
	Time     int64  `json:"time"`
	SelfID   int64  `json:"self_id"`
	PostType string `json:"post_type"`
}

// isEvent 实现了 Event 接口
func (e *BaseEvent) isEvent() {}

type MessageEvent struct {
	BaseEvent
	MessageType string       `json:"message_type"`
	SubType     string       `json:"sub_type"`
	MessageID   int32        `json:"message_id"`
	UserID      int64        `json:"user_id"`
	Message     MessageChain `json:"message"`
	RawMessage  string       `json:"raw_message"`
	Font        int32        `json:"font"`
	Sender      Sender       `json:"sender"`
}

type PrivateMessage struct {
	MessageEvent
}

type GroupMessage struct {
	MessageEvent
	GroupID   int64            `json:"group_id"`
	Anonymous *AnonymousMember `json:"anonymous,omitempty"`
}

type NoticeEvent struct {
	BaseEvent
	NoticeType string `json:"notice_type"`
}

type GroupUploadNotice struct {
	NoticeEvent
	GroupID int64 `json:"group_id"`
	UserID  int64 `json:"user_id"`
	File    struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		Busid int64  `json:"busid"`
	} `json:"file"`
}

type GroupAdminNotice struct {
	NoticeEvent
	SubType string `json:"sub_type"` // "set", "unset"
	GroupID int64  `json:"group_id"`
	UserID  int64  `json:"user_id"`
}

type GroupDecreaseNotice struct {
	NoticeEvent
	SubType    string `json:"sub_type"` // "leave", "kick", "kick_me"
	GroupID    int64  `json:"group_id"`
	OperatorID int64  `json:"operator_id"`
	UserID     int64  `json:"user_id"`
}

type GroupIncreaseNotice struct {
	NoticeEvent
	SubType    string `json:"sub_type"` // "approve", "invite"
	GroupID    int64  `json:"group_id"`
	OperatorID int64  `json:"operator_id"`
	UserID     int64  `json:"user_id"`
}

type GroupBanNotice struct {
	NoticeEvent
	SubType    string `json:"sub_type"` // "ban", "lift_ban"
	GroupID    int64  `json:"group_id"`
	OperatorID int64  `json:"operator_id"`
	UserID     int64  `json:"user_id"`
	Duration   int64  `json:"duration"`
}

type FriendAddNotice struct {
	NoticeEvent
	UserID int64 `json:"user_id"`
}

type GroupRecallNotice struct {
	NoticeEvent
	GroupID    int64 `json:"group_id"`
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
	MessageID  int64 `json:"message_id"`
}

type FriendRecallNotice struct {
	NoticeEvent
	UserID    int64 `json:"user_id"`
	MessageID int64 `json:"message_id"`
}

type NotifyNotice struct {
	NoticeEvent
	SubType   string `json:"sub_type"` // "poke", "lucky_king", "honor"
	GroupID   int64  `json:"group_id,omitempty"`
	UserID    int64  `json:"user_id"`
	TargetID  int64  `json:"target_id,omitempty"`
	HonorType string `json:"honor_type,omitempty"`
}

type RequestEvent struct {
	BaseEvent
	RequestType string `json:"request_type"`
}

type FriendRequest struct {
	RequestEvent
	UserID  int64  `json:"user_id"`
	Comment string `json:"comment"`
	Flag    string `json:"flag"`
}

type GroupRequest struct {
	RequestEvent
	SubType string `json:"sub_type"` // "add", "invite"
	GroupID int64  `json:"group_id"`
	UserID  int64  `json:"user_id"`
	Comment string `json:"comment"`
	Flag    string `json:"flag"`
}

type MetaEvent struct {
	BaseEvent
	MetaEventType string `json:"meta_event_type"`
}

type LifecycleMetaEvent struct {
	MetaEvent
	SubType string `json:"sub_type"` // "enable", "disable", "connect"
}

type HeartbeatMetaEvent struct {
	MetaEvent
	Status   *BotStatus `json:"status"`
	Interval int64      `json:"interval"`
}

// tempEvent 用于预解析 JSON 以确定事件类型
type tempEvent struct {
	PostType      string `json:"post_type"`
	MessageType   string `json:"message_type,omitempty"`
	NoticeType    string `json:"notice_type,omitempty"`
	RequestType   string `json:"request_type,omitempty"`
	MetaEventType string `json:"meta_event_type,omitempty"`
}

type eventConstructor func() Event

var eventRegistry = make(map[string]eventConstructor)

func init() {
	// 注册消息事件
	eventRegistry[buildKey(PostTypeMessage, MessageTypePrivate)] = func() Event { return &PrivateMessage{} }
	eventRegistry[buildKey(PostTypeMessage, MessageTypeGroup)] = func() Event { return &GroupMessage{} }

	// 注册通知事件
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupUpload)] = func() Event { return &GroupUploadNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupAdmin)] = func() Event { return &GroupAdminNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupDecrease)] = func() Event { return &GroupDecreaseNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupIncrease)] = func() Event { return &GroupIncreaseNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupBan)] = func() Event { return &GroupBanNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeFriendAdd)] = func() Event { return &FriendAddNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeGroupRecall)] = func() Event { return &GroupRecallNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeFriendRecall)] = func() Event { return &FriendRecallNotice{} }
	eventRegistry[buildKey(PostTypeNotice, NoticeTypeNotify)] = func() Event { return &NotifyNotice{} }

	// 注册请求事件
	eventRegistry[buildKey(PostTypeRequest, RequestTypeFriend)] = func() Event { return &FriendRequest{} }
	eventRegistry[buildKey(PostTypeRequest, RequestTypeGroup)] = func() Event { return &GroupRequest{} }

	// 注册元事件
	eventRegistry[buildKey(PostTypeMetaEvent, MetaEventTypeLifecycle)] = func() Event { return &LifecycleMetaEvent{} }
	eventRegistry[buildKey(PostTypeMetaEvent, MetaEventTypeHeartbeat)] = func() Event { return &HeartbeatMetaEvent{} }
}

// buildKey 是一个辅助函数，用于生成注册表的键
func buildKey(postType, subType string) string {
	return fmt.Sprintf("%s.%s", postType, subType)
}

// UnmarshalEvent 是一个智能的事件解析函数（重构版）
// 它接收原始的 JSON 数据，并使用注册表自动解析成对应的具体事件结构体
func UnmarshalEvent(data []byte) (Event, error) {
	var pre tempEvent
	if err := json.Unmarshal(data, &pre); err != nil {
		return nil, fmt.Errorf("failed to pre-unmarshal event: %w", err)
	}

	var subType string
	switch pre.PostType {
	case PostTypeMessage:
		subType = pre.MessageType
	case PostTypeNotice:
		subType = pre.NoticeType
	case PostTypeRequest:
		subType = pre.RequestType
	case PostTypeMetaEvent:
		subType = pre.MetaEventType
	default:
		return nil, fmt.Errorf("unknown post type: %s", pre.PostType)
	}

	key := buildKey(pre.PostType, subType)
	constructor, ok := eventRegistry[key]
	if !ok {
		return nil, fmt.Errorf("no constructor registered for event key: %s", key)
	}

	event := constructor()
	if err := json.Unmarshal(data, event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to concrete event type '%T': %w", event, err)
	}

	return event, nil
}

type Friend struct {
	UserId   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}

type Group struct {
	Id         int64  `json:"id"`         // 群号
	Name       string `json:"name"`       // 群名称
	Permission string `json:"permission"` // Bot在群中的权限
}

type GroupMember struct {
	UserId   int64  `json:"user_id"`  // QQ号
	Nickname string `json:"nickname"` // 昵称
	Card     string `json:"card"`     // 群名片／备注
	Sex      string `json:"sex"`      // 性别
	Age      int32  `json:"age"`      // 年龄
	Area     string `json:"area"`     // 地区
	Level    string `json:"level"`    // 成员等级
	Role     string `json:"role"`     // 角色
}

type AnonymousMember struct {
	Id   int64  `json:"id"`   // 匿名用户 ID
	Name string `json:"name"` // 匿名用户名称
	Flag string `json:"flag"` // 匿名用户 flag，在调用禁言 API 时需要传入
}

type Sender struct {
	UserId   int64  `json:"user_id"`  // QQ号
	Nickname string `json:"nickname"` // 昵称
	Sex      string `json:"sex"`      // 性别
	Age      int32  `json:"age"`      // 年龄
}

type BotStatus struct {
	Online bool `json:"online"` // 当前QQ在线，true|false|nil
	Good   bool `json:"good"`   // 状态符合预期，意味着各模块正常运行、功能正常，且 QQ 在线
}

// APITypedResponse 泛型API响应，用于API客户端
type APITypedResponse[T any] struct {
	Status  string `json:"status"`
	Retcode int    `json:"retcode"`
	Message string `json:"msg"`
	Data    T      `json:"data"`
	Echo    string `json:"echo"`
}

type SendMessageResponse struct {
	MessageID int32 `json:"message_id"`
}

type MessageInfo struct {
	MessageID   int32        `json:"message_id"`
	RealID      int32        `json:"real_id"`
	Sender      Sender       `json:"sender"`
	Time        int64        `json:"time"`
	Message     MessageChain `json:"message"`
	RawMessage  string       `json:"raw_message"`
	MessageType string       `json:"message_type"`
	GroupID     int64        `json:"group_id,omitempty"`
}

type LoginInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
}

type Stranger struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Sex      string `json:"sex"`
	Age      int32  `json:"age"`
}
