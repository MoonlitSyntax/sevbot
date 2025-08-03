package main

import (
	"context"
	"fmt"
	bot "github.com/MoonlitSyntax/sevbot"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	b := bot.NewWebSocketBotSimple("ws://127.0.0.1:3001", "")

	slog.Info("机器人正在启动...")

	bot.On(b, func(api *bot.APIClient, event *bot.PrivateMessage) error {
		slog.Info("收到私聊消息", "user_id", event.UserID)

		replyText := formatMessageChainAnalysis(event.Message)
		_, err := api.SendPrivateMessage(context.Background(), event.UserID, bot.TextMessage(replyText))
		if err != nil {
			slog.Error("回复私聊消息失败", "error", err)
		}
		return err
	})

	bot.On(b, func(api *bot.APIClient, event *bot.GroupMessage) error {
		if event.UserID == event.SelfID {
			return nil
		}

		slog.Info("收到群消息", "group_id", event.GroupID, "user_id", event.UserID)

		replyText := formatMessageChainAnalysis(event.Message)

		replyMsg := bot.NewMessage().
			At(fmt.Sprintf("%d", event.UserID)).
			Text("\n").
			Text(replyText).
			Build()

		_, err := api.SendGroupMessage(context.Background(), event.GroupID, replyMsg)
		if err != nil {
			slog.Error("回复群消息失败", "error", err)
		}
		return err
	})

	bot.On(b, func(api *bot.APIClient, event *bot.FriendRequest) error {
		slog.Info("收到好友请求", "user_id", event.UserID, "comment", event.Comment)
		return nil
	})

	bot.On(b, func(api *bot.APIClient, event *bot.GroupRequest) error {
		slog.Info("收到群组请求", "group_id", event.GroupID, "user_id", event.UserID)
		return nil
	})

	bot.On(b, func(api *bot.APIClient, event *bot.NoticeEvent) error {
		slog.Info("收到通知事件", "notice_type", event.NoticeType)
		return nil
	})
	bot.On(b, func(api *bot.APIClient, event *bot.NotifyNotice) error {
		if event.SubType == "poke" {
			if event.GroupID > 0 {
				slog.Info("在群里被戳了一下！",
					"group_id", event.GroupID,
					"poker_id", event.UserID,
					"poked_id", event.TargetID,
				)

				replyMsg := bot.NewMessage().
					At(fmt.Sprintf("%d", event.UserID)).
					Text(" 戳我干嘛？").
					Build()
				_, err := api.SendGroupMessage(context.Background(), event.GroupID, replyMsg)
				return err

			} else {
				slog.Info("在私聊里被戳了一下！",
					"poker_id", event.UserID,
				)
				_, err := api.SendPrivateMessage(context.Background(), event.UserID, bot.TextMessage("不许戳我！"))
				return err
			}
		}
		return nil
	})
	bot.On(b, func(api *bot.APIClient, event *bot.HeartbeatMetaEvent) error {
		slog.Debug("收到心跳", "interval", event.Interval)
		return nil
	})

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := b.Run(); err != nil {
			log.Fatal("机器人运行失败:", err)
		}
	}()

	<-shutdownChan
	slog.Info("正在关闭机器人...")
	if err := b.Stop(); err != nil {
		slog.Error("关闭机器人时发生错误", "error", err)
	}
	slog.Info("机器人已关闭。")
}

func formatMessageChainAnalysis(chain bot.MessageChain) string {
	if len(chain) == 0 {
		return "我收到了一个空消息"
	}

	var builder strings.Builder
	builder.WriteString("我收到了这条消息，解析结果如下：\n")
	builder.WriteString("--------------------\n")

	for i, segment := range chain {
		segType := segment.GetMessageType()
		segString := segment.String()
		builder.WriteString(fmt.Sprintf("» 段落 %d\n", i+1))
		builder.WriteString(fmt.Sprintf("  类型: %s\n", segType))
		builder.WriteString(fmt.Sprintf("  内容: %s\n", segString))
	}
	builder.WriteString("--------------------")

	return builder.String()
}
