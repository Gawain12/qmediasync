package helpers

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot 是一个简单的 Telegram 客户端包装器，满足项目中对 helpers.TelegramBot 的基本使用。
type TelegramBot struct {
	bot    *tgbotapi.BotAPI
	chatId int64
}

// parseChatId 将字符串 chat id 转为 int64
func parseChatId(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("chat id 为空")
	}
	return strconv.ParseInt(s, 10, 64)
}

// NewTelegramBot 使用默认 HTTP 客户端创建 TelegramBot。
// 如果创建失败，返回 nil（调用方中有 nil 检查）。
func NewTelegramBot(token, chatId string) *TelegramBot {
	id, err := parseChatId(chatId)
	if err != nil {
		return nil
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil
	}
	bot.Debug = false
	return &TelegramBot{bot: bot, chatId: id}
}

// NewTelegramBotWithProxy 使用指定的 HTTP 代理创建 TelegramBot，失败时返回错误。
func NewTelegramBotWithProxy(token, chatId, proxyURL string) (*TelegramBot, error) {
	id, err := parseChatId(chatId)
	if err != nil {
		return nil, err
	}
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("解析代理地址失败: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// NewBotAPIWithClient 的签名期望 (token string, botURL string, client tgbotapi.HTTPClient)
	// 第二个参数可以传空字符串表示使用默认 bot URL
	bot, err := tgbotapi.NewBotAPIWithClient(token, "", client)
	if err != nil {
		return nil, err
	}
	bot.Debug = false
	return &TelegramBot{bot: bot, chatId: id}, nil
}

// SendMessageWithRetry 发送文本消息，带重试机制。消息以 HTML 格式发送。
func (t *TelegramBot) SendMessageWithRetry(message string, retries int) error {
	if t == nil || t.bot == nil {
		return fmt.Errorf("telegram bot 未初始化")
	}
	msg := tgbotapi.NewMessage(t.chatId, message)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	var lastErr error
	for i := 0; i < retries; i++ {
		_, err := t.bot.Send(msg)
		if err == nil {
			return nil
		}
		lastErr = err
		// 简单指数退避
		time.Sleep(time.Duration((i+1)*500) * time.Millisecond)
	}
	return lastErr
}

// SendPhoto 通过 URL 发送图片（作为 Telegram FileURL），caption 可以为空。
func (t *TelegramBot) SendPhoto(photoURL, caption string) error {
	if t == nil || t.bot == nil {
		return fmt.Errorf("telegram bot 未初始化")
	}
	photo := tgbotapi.NewPhoto(t.chatId, tgbotapi.FileURL(photoURL))
	if caption != "" {
		photo.Caption = caption
		photo.ParseMode = "HTML"
	}
	_, err := t.bot.Send(photo)
	return err
}

// TestTelegramBot 用于设置页面测试 Telegram 配置，返回 nil 表示成功，否则返回具体错误
// controllers/settings.go 中会调用 helpers.TestTelegramBot(token, chatId, proxyURL)
func TestTelegramBot(token, chatId, proxyURL string) error {
	var bot *TelegramBot
	var err error

	if proxyURL != "" {
		// 使用代理方式测试
		bot, err = NewTelegramBotWithProxy(token, chatId, proxyURL)
		if err != nil {
			return fmt.Errorf("创建 TelegramBot 失败: %v", err)
		}
	} else {
		// 使用默认（无代理）方式测试
		bot = NewTelegramBot(token, chatId)
		if bot == nil {
			return fmt.Errorf("创建 TelegramBot 失败，请检查 token 或 chatId 是否正确")
		}
	}

	// 发送一条短测试消息（一次重试）
	msg := "QMediaSync: Telegram 测试消息 — 测试连接成功"
	if err := bot.SendMessageWithRetry(msg, 1); err != nil {
		return fmt.Errorf("发送测试消息失败: %v", err)
	}
	return nil
}