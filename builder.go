package sevbot

import "fmt"

// MessageBuilder 提供了链式 API 来构建消息
type MessageBuilder struct {
	chain MessageChain
}

// NewMessage 创建一个新的消息构建器
func NewMessage() *MessageBuilder {
	return &MessageBuilder{
		chain: make(MessageChain, 0),
	}
}

// Text 添加纯文本消息段
func (b *MessageBuilder) Text(text string) *MessageBuilder {
	b.chain = append(b.chain, &Text{Text: text})
	return b
}

// Textf 添加格式化的纯文本消息段
func (b *MessageBuilder) Textf(format string, args ...any) *MessageBuilder {
	return b.Text(fmt.Sprintf(format, args...))
}

// Face 添加QQ表情
func (b *MessageBuilder) Face(id string) *MessageBuilder {
	b.chain = append(b.chain, &Face{Id: id})
	return b
}

// Image 添加图片
func (b *MessageBuilder) Image(file string) *MessageBuilder {
	b.chain = append(b.chain, &Image{FileMessage: FileMessage{File: file}})
	return b
}

// ImageFlash 添加闪照
func (b *MessageBuilder) ImageFlash(file string) *MessageBuilder {
	b.chain = append(b.chain, &Image{
		FileMessage: FileMessage{File: file},
		Type:        "flash",
	})
	return b
}

// Record 添加语音
func (b *MessageBuilder) Record(file string) *MessageBuilder {
	b.chain = append(b.chain, &Record{FileMessage: FileMessage{File: file}})
	return b
}

// Video 添加短视频
func (b *MessageBuilder) Video(file string) *MessageBuilder {
	b.chain = append(b.chain, &Video{FileMessage: FileMessage{File: file}})
	return b
}

// At 添加@某人
func (b *MessageBuilder) At(qq string) *MessageBuilder {
	b.chain = append(b.chain, &At{QQ: qq})
	return b
}

// AtAll 添加@全体成员
func (b *MessageBuilder) AtAll() *MessageBuilder {
	return b.At("all")
}

// Reply 添加回复
func (b *MessageBuilder) Reply(messageID string) *MessageBuilder {
	b.chain = append(b.chain, &Reply{Id: messageID})
	return b
}

// Share 添加链接分享
func (b *MessageBuilder) Share(url, title string) *MessageBuilder {
	b.chain = append(b.chain, &Share{
		URL:   url,
		Title: title,
	})
	return b
}

// ShareWithContent 添加带内容的链接分享
func (b *MessageBuilder) ShareWithContent(url, title, content, image string) *MessageBuilder {
	b.chain = append(b.chain, &Share{
		URL:     url,
		Title:   title,
		Content: content,
		Image:   image,
	})
	return b
}

// Contact 添加推荐联系人
func (b *MessageBuilder) Contact(contactType ContactType, id string) *MessageBuilder {
	b.chain = append(b.chain, &Contact{
		Type: contactType,
		Id:   id,
	})
	return b
}

// ContactQQ 添加推荐好友
func (b *MessageBuilder) ContactQQ(qq string) *MessageBuilder {
	return b.Contact(ContactQQ, qq)
}

// ContactGroup 添加推荐群
func (b *MessageBuilder) ContactGroup(groupID string) *MessageBuilder {
	return b.Contact(ContactGroup, groupID)
}

// Location 添加位置分享
func (b *MessageBuilder) Location(lat, lon string) *MessageBuilder {
	b.chain = append(b.chain, &Location{
		Lat: lat,
		Lon: lon,
	})
	return b
}

// LocationWithTitle 添加带标题的位置分享
func (b *MessageBuilder) LocationWithTitle(lat, lon, title, content string) *MessageBuilder {
	b.chain = append(b.chain, &Location{
		Lat:     lat,
		Lon:     lon,
		Title:   title,
		Content: content,
	})
	return b
}

// Music 添加音乐分享
func (b *MessageBuilder) Music(musicType, id string) *MessageBuilder {
	b.chain = append(b.chain, &Music{
		Type: musicType,
		Id:   id,
	})
	return b
}

// CustomMusic 添加自定义音乐分享
func (b *MessageBuilder) CustomMusic(url, audio, title string) *MessageBuilder {
	b.chain = append(b.chain, &Music{
		Type:  "custom",
		URL:   url,
		Audio: audio,
		Title: title,
	})
	return b
}

// XML 添加XML消息
func (b *MessageBuilder) XML(data string) *MessageBuilder {
	b.chain = append(b.chain, &XML{Data: data})
	return b
}

// JSON 添加JSON消息
func (b *MessageBuilder) JSON(data string) *MessageBuilder {
	b.chain = append(b.chain, &JSON{Data: data})
	return b
}

// Forward 添加合并转发
func (b *MessageBuilder) Forward(id string) *MessageBuilder {
	b.chain = append(b.chain, &Forward{Id: id})
	return b
}

// Node 添加转发节点
func (b *MessageBuilder) Node(id string) *MessageBuilder {
	b.chain = append(b.chain, &Node{Id: id})
	return b
}

// CustomNode 添加自定义转发节点
func (b *MessageBuilder) CustomNode(userID, nickname string, content MessageChain) *MessageBuilder {
	b.chain = append(b.chain, &Node{
		UserID:   userID,
		Nickname: nickname,
		Content:  content,
	})
	return b
}

// RPS 添加猜拳魔法表情
func (b *MessageBuilder) RPS() *MessageBuilder {
	b.chain = append(b.chain, &RPS{})
	return b
}

// Dice 添加掷骰子魔法表情
func (b *MessageBuilder) Dice() *MessageBuilder {
	b.chain = append(b.chain, &Dice{})
	return b
}

// Shake 添加窗口抖动
func (b *MessageBuilder) Shake() *MessageBuilder {
	b.chain = append(b.chain, &Shake{})
	return b
}

// Poke 添加戳一戳
func (b *MessageBuilder) Poke(pokeType, id string) *MessageBuilder {
	b.chain = append(b.chain, &Poke{
		Type: pokeType,
		Id:   id,
	})
	return b
}

// Anonymous 添加匿名发消息
func (b *MessageBuilder) Anonymous() *MessageBuilder {
	b.chain = append(b.chain, &Anonymous{})
	return b
}

// Add 添加自定义消息段
func (b *MessageBuilder) Add(message SingleMessage) *MessageBuilder {
	b.chain = append(b.chain, message)
	return b
}

// Append 追加另一个消息链
func (b *MessageBuilder) Append(chain MessageChain) *MessageBuilder {
	b.chain = append(b.chain, chain...)
	return b
}

// Build 构建最终的消息链
func (b *MessageBuilder) Build() MessageChain {
	return b.chain
}

// String 返回消息链的字符串表示
func (b *MessageBuilder) String() string {
	return b.chain.String()
}

// Clone 克隆当前构建器
func (b *MessageBuilder) Clone() *MessageBuilder {
	newChain := make(MessageChain, len(b.chain))
	copy(newChain, b.chain)
	return &MessageBuilder{chain: newChain}
}

// Clear 清空消息链
func (b *MessageBuilder) Clear() *MessageBuilder {
	b.chain = b.chain[:0]
	return b
}

// Len 返回消息段数量
func (b *MessageBuilder) Len() int {
	return len(b.chain)
}

// IsEmpty 检查是否为空
func (b *MessageBuilder) IsEmpty() bool {
	return len(b.chain) == 0
}
