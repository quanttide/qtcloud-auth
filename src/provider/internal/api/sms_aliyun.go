package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/client"
	rpc "github.com/alibabacloud-go/tea-rpc/client"
	"github.com/alibabacloud-go/tea/tea"
)

// AliyunSMSSender 通过阿里云短信服务发送验证码（生产环境发送器）。
type AliyunSMSSender struct {
	client       *dysmsapi.Client
	signName     string
	templateCode string
}

// NewAliyunSMSSender 从环境变量构造发送器：
// SMS_ACCESS_KEY_ID / SMS_ACCESS_KEY_SECRET（专用子账号，最小权限 AliyunDysmsFullAccess）
// SMS_SIGN_NAME（签名，如"量潮科技"）/ SMS_TEMPLATE_CODE（模板 ID，如 SMS_xxxx）。
func NewAliyunSMSSender() (*AliyunSMSSender, error) {
	ak := os.Getenv("SMS_ACCESS_KEY_ID")
	sk := os.Getenv("SMS_ACCESS_KEY_SECRET")
	sign := os.Getenv("SMS_SIGN_NAME")
	tpl := os.Getenv("SMS_TEMPLATE_CODE")
	if ak == "" || sk == "" || sign == "" || tpl == "" {
		return nil, fmt.Errorf("aliyun sms requires SMS_ACCESS_KEY_ID/SMS_ACCESS_KEY_SECRET/SMS_SIGN_NAME/SMS_TEMPLATE_CODE")
	}
	cfg := &rpc.Config{
		AccessKeyId:     tea.String(ak),
		AccessKeySecret: tea.String(sk),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}
	client, err := dysmsapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("new dysms client: %w", err)
	}
	return &AliyunSMSSender{client: client, signName: sign, templateCode: tpl}, nil
}

func (s *AliyunSMSSender) Send(ctx context.Context, phone, code string) error {
	param, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return fmt.Errorf("marshal template param: %w", err)
	}
	resp, err := s.client.SendSms(&dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone),
		SignName:      tea.String(s.signName),
		TemplateCode:  tea.String(s.templateCode),
		TemplateParam: tea.String(string(param)),
	})
	if err != nil {
		return fmt.Errorf("send sms: %w", err)
	}
	if resp.Code == nil || *resp.Code != "OK" {
		return fmt.Errorf("send sms rejected: code=%s message=%s", tea.StringValue(resp.Code), tea.StringValue(resp.Message))
	}
	slog.Info("sms sent", "phone", phone)
	return nil
}
