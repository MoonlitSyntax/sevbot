package bot

// TextMessage 创建文本消息
func TextMessage(text string) MessageChain {
	return MessageChain{&Text{Text: text}}
}

// AtMessage 创建@消息
func AtMessage(qq string) MessageChain {
	return MessageChain{&At{QQ: qq}}
}

// AtAllMessage 创建@全体成员消息
func AtAllMessage() MessageChain {
	return MessageChain{&At{QQ: "all"}}
}

// ImageMessage 创建图片消息
func ImageMessage(file string) MessageChain {
	return MessageChain{&Image{FileMessage: FileMessage{File: file}}}
}

// MixedMessage 创建混合消息
func MixedMessage(messages ...SingleMessage) MessageChain {
	return MessageChain(messages)
}

// ReplyMessage 创建回复消息
func ReplyMessage(messageID string, text string) MessageChain {
	return MessageChain{
		&Reply{Id: messageID},
		&Text{Text: text},
	}
}