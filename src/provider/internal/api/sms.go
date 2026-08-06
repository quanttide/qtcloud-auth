package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"time"

	"github.com/quanttide/qtcloud-auth/internal/model"
)

// SMSSender 短信发送抽象.
type SMSSender interface {
	Send(ctx context.Context, phone, code string) error
}

// ConsoleSender 开发调试用，将验证码打印到日志.
type ConsoleSender struct{}

func (s *ConsoleSender) Send(_ context.Context, phone, code string) error {
	slog.Info("sms code", "phone", phone, "code", code)
	return nil
}

var phoneRegexp = regexp.MustCompile(`^1[3-9]\d{9}$`)

func isValidPhone(phone string) bool {
	return phoneRegexp.MatchString(phone)
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func generateCode() (string, error) {
	if fixed := os.Getenv("SMS_TEST_CODE"); fixed != "" {
		return fixed, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

const (
	sendSMSRateLimit = 60 * time.Second
	codeTTL          = 5 * time.Minute
)

// SendCode POST /oauth/sms/send 发送手机验证码.
// 验证码与频率限制均基于共享存储（FC 多实例一致），不依赖实例本地状态。
func (h *AuthHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteOAuthError(w, "invalid_request", "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidPhone(req.Phone) {
		WriteOAuthError(w, "invalid_request", "invalid phone number", http.StatusBadRequest)
		return
	}

	// 频率限制：同一手机号 60s 内只能发送一次（取最近一条验证码的创建时间）
	if codes, err := h.listCodes(); err == nil {
		for i := len(codes) - 1; i >= 0; i-- {
			if codes[i].Phone == req.Phone {
				created, cerr := time.Parse(time.RFC3339, codes[i].CreatedAt)
				if cerr == nil && time.Since(created) < sendSMSRateLimit {
					WriteOAuthError(w, "rate_limit",
						fmt.Sprintf("retry after %.0f seconds", sendSMSRateLimit.Seconds()),
						http.StatusTooManyRequests)
					return
				}
				break
			}
		}
	}

	code, _ := generateCode()

	vc := model.VerificationCode{
		Phone:     req.Phone,
		Code:      code,
		ExpiresAt: time.Now().Add(codeTTL).Format(time.RFC3339),
		Used:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(vc)
	if _, err := h.store.Create("auth/verification_codes", data); err != nil {
		slog.Error("create verification code", "error", err)
		WriteOAuthError(w, "server_error", "failed to store code", http.StatusInternalServerError)
		return
	}

	if err := h.smsSender.Send(r.Context(), req.Phone, code); err != nil {
		slog.Error("send sms", "error", err)
		WriteOAuthError(w, "server_error", "failed to send sms", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"message": "code sent"}, http.StatusOK)
}

// ── 内部辅助 ──

// listCodes 从共享存储读取全部验证码（按创建时间升序）。
func (h *AuthHandler) listCodes() ([]model.VerificationCode, error) {
	data, err := h.store.List("auth/verification_codes")
	if err != nil {
		return nil, err
	}
	var codes []model.VerificationCode
	if err := json.Unmarshal(data, &codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (h *AuthHandler) findValidCode(phone, code string) *model.VerificationCode {
	codes, err := h.listCodes()
	if err != nil {
		return nil
	}
	now := time.Now()
	for i := len(codes) - 1; i >= 0; i-- {
		vc := codes[i]
		if vc.Phone != phone || vc.Code != code || vc.Used {
			continue
		}
		exp, err := time.Parse(time.RFC3339, vc.ExpiresAt)
		if err != nil || now.After(exp) {
			continue
		}
		return &vc
	}
	return nil
}

func (h *AuthHandler) markCodeUsed(phone, code string) {
	codes, err := h.listCodes()
	if err != nil {
		return
	}
	for _, vc := range codes {
		if vc.Phone == phone && vc.Code == code && !vc.Used {
			vc.Used = true
			data, err := json.Marshal(vc)
			if err != nil {
				return
			}
			_ = h.store.Update("auth/verification_codes", vc.ID, data)
			return
		}
	}
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
