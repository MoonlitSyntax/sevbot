package bot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var cqCodeRegex = regexp.MustCompile(`\[CQ:([^,\]]+)(,[^\]]*)?\]`)

// MessageType 定义了消息段的类型
type MessageType string

// 定义所有支持的消息段类型常量，避免魔法字符串
const (
	MsgTypeText      MessageType = "text"
	MsgTypeFace      MessageType = "face"
	MsgTypeImage     MessageType = "image"
	MsgTypeRecord    MessageType = "record"
	MsgTypeVideo     MessageType = "video"
	MsgTypeAt        MessageType = "at"
	MsgTypeRPS       MessageType = "rps"
	MsgTypeDice      MessageType = "dice"
	MsgTypeShake     MessageType = "shake"
	MsgTypePoke      MessageType = "poke"
	MsgTypeAnonymous MessageType = "anonymous"
	MsgTypeShare     MessageType = "share"
	MsgTypeContact   MessageType = "contact"
	MsgTypeLocation  MessageType = "location"
	MsgTypeMusic     MessageType = "music"
	MsgTypeReply     MessageType = "reply"
	MsgTypeForward   MessageType = "forward"
	MsgTypeNode      MessageType = "node"
	MsgTypeXML       MessageType = "xml"
	MsgTypeJSON      MessageType = "json"
)

// SingleMessage 是所有单个消息段都必须实现的接口
type SingleMessage interface {
	// GetMessageType 返回消息段的类型
	GetMessageType() MessageType
	fmt.Stringer
}

// MessageChain 代表一个消息链，由多个消息段组成
type MessageChain []SingleMessage

func (c MessageChain) String() string {
	var sb strings.Builder
	for _, s := range c {
		sb.WriteString(s.String())
	}
	return sb.String()
}

// tempMessageSegment 是一个用于 JSON 反序列化的辅助结构
type tempMessageSegment struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// MarshalJSON 实现了 MessageChain 的自定义 JSON 序列化
func (c MessageChain) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("[]"), nil
	}
	segments := make([]any, 0, len(c))
	for _, m := range c {
		segment := map[string]any{
			"type": m.GetMessageType(),
			"data": m,
		}
		segments = append(segments, segment)
	}
	return json.Marshal(segments)
}

// UnmarshalJSON 实现了 MessageChain 的自定义 JSON 反序列化
func (c *MessageChain) UnmarshalJSON(data []byte) error {
	var tempSegments []tempMessageSegment
	if err := json.Unmarshal(data, &tempSegments); err != nil {
		return fmt.Errorf("unmarshal message chain failed: %w", err)
	}

	*c = make(MessageChain, 0, len(tempSegments))
	for _, segment := range tempSegments {
		msgType := MessageType(segment.Type)
		builder, ok := singleMessageBuilder[msgType]
		if !ok {
			slog.Warn("unknown message type, skipping", "type", msgType)
			continue
		}

		msg := builder()
		if err := json.Unmarshal(segment.Data, msg); err != nil {
			slog.Error("unmarshal single message failed", "type", msgType, "data", string(segment.Data), "error", err)
			continue
		}
		*c = append(*c, msg)
	}
	return nil
}

// FileMessage 包含了文件类消息（图片、语音、视频）的通用字段
type FileMessage struct {
	// File 可以是文件名、绝对路径(file URI)、网络URL(http/https)、Base64(base64://)
	File string `json:"file"`
	// URL 是接收文件类消息时，由 OneBot 实现提供下载链接
	URL string `json:"url,omitempty"`
	// Cache 控制是否使用缓存（仅发送网络文件时）
	Cache   *bool  `json:"cache,omitempty"`
	Proxy   *bool  `json:"proxy,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

// Text 纯文本
type Text struct {
	Text string `json:"text"`
}

func (m *Text) GetMessageType() MessageType { return MsgTypeText }
func (m *Text) String() string              { return m.Text }

// Face QQ表情
type Face struct {
	Id string `json:"id"` // QQ 表情 ID
}

func (m *Face) GetMessageType() MessageType { return MsgTypeFace }
func (m *Face) String() string              { return fmt.Sprintf("[CQ:face,id=%s]", escapeCQ(m.Id)) }

// Image 图片
type Image struct {
	FileMessage
	Type string `json:"type,omitempty"` // "flash" 表示闪照
}

func (m *Image) GetMessageType() MessageType { return MsgTypeImage }
func (m *Image) String() string              { return fmt.Sprintf("[CQ:image,file=%s]", escapeCQ(m.File)) }

// Record 语音
type Record struct {
	FileMessage
	Magic bool `json:"magic,omitempty"` // 是否变声
}

func (m *Record) GetMessageType() MessageType { return MsgTypeRecord }
func (m *Record) String() string              { return fmt.Sprintf("[CQ:record,file=%s]", escapeCQ(m.File)) }

// Video 短视频
type Video struct {
	FileMessage
}

func (m *Video) GetMessageType() MessageType { return MsgTypeVideo }
func (m *Video) String() string              { return fmt.Sprintf("[CQ:video,file=%s]", escapeCQ(m.File)) }

// At @某人
type At struct {
	QQ string `json:"qq"` // @的QQ号, "all" 表示全体成员
}

func (m *At) GetMessageType() MessageType { return MsgTypeAt }
func (m *At) String() string              { return fmt.Sprintf("[CQ:at,qq=%s]", escapeCQ(m.QQ)) }

// RPS 猜拳魔法表情
type RPS struct{}

func (m *RPS) GetMessageType() MessageType { return MsgTypeRPS }
func (m *RPS) String() string              { return "[CQ:rps]" }

// Dice 掷骰子魔法表情
type Dice struct{}

func (m *Dice) GetMessageType() MessageType { return MsgTypeDice }
func (m *Dice) String() string              { return "[CQ:dice]" }

// Shake 窗口抖动
type Shake struct{}

func (m *Shake) GetMessageType() MessageType { return MsgTypeShake }
func (m *Shake) String() string              { return "[CQ:shake]" }

// Poke 戳一戳
type Poke struct {
	Type string `json:"type"`
	Id   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func (m *Poke) GetMessageType() MessageType { return MsgTypePoke }
func (m *Poke) String() string {
	return fmt.Sprintf("[CQ:poke,type=%s,id=%s]", escapeCQ(m.Type), escapeCQ(m.Id))
}

// Anonymous 匿名发消息
type Anonymous struct {
	Ignore string `json:"ignore,omitempty"` // 无法匿名时是否继续发送, 1 或 0
}

func (m *Anonymous) GetMessageType() MessageType { return MsgTypeAnonymous }
func (m *Anonymous) String() string              { return "[CQ:anonymous]" }

// Share 链接分享
type Share struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Image   string `json:"image,omitempty"` // 图片 URL
}

func (m *Share) GetMessageType() MessageType { return MsgTypeShare }
func (m *Share) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[CQ:share,url=%s,title=%s", escapeCQ(m.URL), escapeCQ(m.Title)))
	if m.Content != "" {
		sb.WriteString(fmt.Sprintf(",content=%s", escapeCQ(m.Content)))
	}
	if m.Image != "" {
		sb.WriteString(fmt.Sprintf(",image=%s", escapeCQ(m.Image)))
	}
	sb.WriteString("]")
	return sb.String()
}

// ContactType 定义了推荐联系人的类型
type ContactType string

const (
	ContactQQ    ContactType = "qq"    // 推荐好友
	ContactGroup ContactType = "group" // 推荐群
)

// Contact 推荐好友或群
type Contact struct {
	Type ContactType `json:"type"`
	Id   string      `json:"id"`
}

func (m *Contact) GetMessageType() MessageType { return MsgTypeContact }
func (m *Contact) String() string {
	return fmt.Sprintf("[CQ:contact,type=%s,id=%s]", m.Type, escapeCQ(m.Id))
}

// Location 位置
type Location struct {
	Lat     string `json:"lat"`
	Lon     string `json:"lon"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

func (m *Location) GetMessageType() MessageType { return MsgTypeLocation }
func (m *Location) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[CQ:location,lat=%s,lon=%s", escapeCQ(m.Lat), escapeCQ(m.Lon)))
	if m.Title != "" {
		sb.WriteString(fmt.Sprintf(",title=%s", escapeCQ(m.Title)))
	}
	if m.Content != "" {
		sb.WriteString(fmt.Sprintf(",content=%s", escapeCQ(m.Content)))
	}
	sb.WriteString("]")
	return sb.String()
}

// Music 音乐分享
type Music struct {
	Type    string `json:"type"`              // "qq", "163", "xm", "custom"
	Id      string `json:"id,omitempty"`      // 歌曲 ID, 非 custom 时使用
	URL     string `json:"url,omitempty"`     // 点击跳转 URL, custom 时使用
	Audio   string `json:"audio,omitempty"`   // 音乐 URL, custom 时使用
	Title   string `json:"title,omitempty"`   // 标题, custom 时使用
	Content string `json:"content,omitempty"` // 内容描述
	Image   string `json:"image,omitempty"`   // 封面图片 URL
}

func (m *Music) GetMessageType() MessageType { return MsgTypeMusic }
func (m *Music) String() string {
	if m.Type == "custom" {
		return fmt.Sprintf("[CQ:music,type=custom,url=%s,audio=%s,title=%s]",
			escapeCQ(m.URL), escapeCQ(m.Audio), escapeCQ(m.Title))
	}
	return fmt.Sprintf("[CQ:music,type=%s,id=%s]", escapeCQ(m.Type), escapeCQ(m.Id))
}

// Reply 回复
type Reply struct {
	Id string `json:"id"` // 回复时引用的消息 ID
}

func (m *Reply) GetMessageType() MessageType { return MsgTypeReply }
func (m *Reply) String() string              { return fmt.Sprintf("[CQ:reply,id=%s]", escapeCQ(m.Id)) }

// Forward 合并转发
type Forward struct {
	Id string `json:"id"` // 合并转发 ID, 需通过 API 获取内容
}

func (m *Forward) GetMessageType() MessageType { return MsgTypeForward }
func (m *Forward) String() string              { return fmt.Sprintf("[CQ:forward,id=%s]", escapeCQ(m.Id)) }

// Node 合并转发节点
type Node struct {
	Id string `json:"id,omitempty"` // 转发的消息 ID
	// 以下为自定义节点字段
	UserID   string       `json:"user_id,omitempty"`
	Nickname string       `json:"nickname,omitempty"`
	Content  MessageChain `json:"content,omitempty"`
}

func (m *Node) GetMessageType() MessageType { return MsgTypeNode }
func (m *Node) String() string {
	if len(m.Id) > 0 {
		return fmt.Sprintf("[CQ:node,id=%s]", escapeCQ(m.Id))
	}
	var sb strings.Builder
	for _, msg := range m.Content {
		sb.WriteString(msg.String())
	}
	return fmt.Sprintf("[CQ:node,user_id=%s,nickname=%s,content=%s]",
		escapeCQ(m.UserID), escapeCQ(m.Nickname), escapeCQ(sb.String()))
}

// XML 消息
type XML struct {
	Data string `json:"data"`
}

func (m *XML) GetMessageType() MessageType { return MsgTypeXML }
func (m *XML) String() string              { return fmt.Sprintf("[CQ:xml,data=%s]", escapeCQ(m.Data)) }

// JSON 消息
type JSON struct {
	Data string `json:"data"`
}

func (m *JSON) GetMessageType() MessageType { return MsgTypeJSON }
func (m *JSON) String() string              { return fmt.Sprintf("[CQ:json,data=%s]", escapeCQ(m.Data)) }

// singleMessageBuilder 是一个消息段构造函数的工厂
// key 是消息类型，value 是一个能创建对应消息段实例的函数
var singleMessageBuilder = make(map[MessageType]func() SingleMessage)

func init() {
	// 在程序初始化时，注册所有已知的消息段类型及其构造函数
	singleMessageBuilder[MsgTypeText] = func() SingleMessage { return &Text{} }
	singleMessageBuilder[MsgTypeFace] = func() SingleMessage { return &Face{} }
	singleMessageBuilder[MsgTypeImage] = func() SingleMessage { return &Image{} }
	singleMessageBuilder[MsgTypeRecord] = func() SingleMessage { return &Record{} }
	singleMessageBuilder[MsgTypeVideo] = func() SingleMessage { return &Video{} }
	singleMessageBuilder[MsgTypeAt] = func() SingleMessage { return &At{} }
	singleMessageBuilder[MsgTypeRPS] = func() SingleMessage { return &RPS{} }
	singleMessageBuilder[MsgTypeDice] = func() SingleMessage { return &Dice{} }
	singleMessageBuilder[MsgTypeShake] = func() SingleMessage { return &Shake{} }
	singleMessageBuilder[MsgTypePoke] = func() SingleMessage { return &Poke{} }
	singleMessageBuilder[MsgTypeAnonymous] = func() SingleMessage { return &Anonymous{} }
	singleMessageBuilder[MsgTypeShare] = func() SingleMessage { return &Share{} }
	singleMessageBuilder[MsgTypeContact] = func() SingleMessage { return &Contact{} }
	singleMessageBuilder[MsgTypeLocation] = func() SingleMessage { return &Location{} }
	singleMessageBuilder[MsgTypeMusic] = func() SingleMessage { return &Music{} }
	singleMessageBuilder[MsgTypeReply] = func() SingleMessage { return &Reply{} }
	singleMessageBuilder[MsgTypeForward] = func() SingleMessage { return &Forward{} }
	singleMessageBuilder[MsgTypeNode] = func() SingleMessage { return &Node{} }
	singleMessageBuilder[MsgTypeXML] = func() SingleMessage { return &XML{} }
	singleMessageBuilder[MsgTypeJSON] = func() SingleMessage { return &JSON{} }
}

func escapeCQ(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "[", "&#91;")
	s = strings.ReplaceAll(s, "]", "&#93;")
	s = strings.ReplaceAll(s, ",", "&#44;")
	return s
}

func unescapeCQ(s string) string {
	s = strings.ReplaceAll(s, "&#44;", ",")
	s = strings.ReplaceAll(s, "&#93;", "]")
	s = strings.ReplaceAll(s, "&#91;", "[")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

func ParseMessageFromString(raw string) (MessageChain, error) {
	if raw == "" {
		return MessageChain{}, nil
	}
	var chain MessageChain
	lastIndex := 0
	matches := cqCodeRegex.FindAllStringSubmatchIndex(raw, -1)
	for _, match := range matches {
		if match[0] > lastIndex {
			text := raw[lastIndex:match[0]]
			chain = append(chain, &Text{Text: unescapeCQ(text)})
		}
		cqTypeStr := raw[match[2]:match[3]]
		cqParamStr := ""
		if match[4] != -1 {
			cqParamStr = raw[match[4]+1 : match[5]]
		}
		msgType := MessageType(cqTypeStr)
		builder, ok := singleMessageBuilder[msgType]
		if !ok {
			slog.Warn("unknown CQ code type, skipping", "type", msgType)
			lastIndex = match[1]
			continue
		}

		msg := builder()
		paramsMap := parseCQParams(cqParamStr)
		jsonBytes, err := json.Marshal(paramsMap)
		if err != nil {
			slog.Error("failed to marshal CQ params", "params", paramsMap, "error", err)
			lastIndex = match[1]
			continue
		}
		if err := json.Unmarshal(jsonBytes, msg); err != nil {
			slog.Error("failed to unmarshal CQ message", "type", msgType, "params", paramsMap, "error", err)
			lastIndex = match[1]
			continue
		}
		chain = append(chain, msg)
		lastIndex = match[1]
	}
	if lastIndex < len(raw) {
		text := raw[lastIndex:]
		if text != "" {
			chain = append(chain, &Text{Text: unescapeCQ(text)})
		}
	}
	return chain, nil
}

func parseCQParams(paramStr string) map[string]string {
	params := make(map[string]string)
	if paramStr == "" {
		return params
	}

	pairs := strings.Split(paramStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := unescapeCQ(parts[0])
			value := unescapeCQ(parts[1])
			params[key] = value
		} else if len(parts) == 1 {
			key := unescapeCQ(parts[0])
			params[key] = ""
		}
	}
	return params
}
