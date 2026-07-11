package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/quanttide/qtcloud-auth/model"
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

	h.codeMu.Lock()
	last, ok := h.codeLastSent[req.Phone]
	if ok && time.Since(last) < sendSMSRateLimit {
		h.codeMu.Unlock()
		WriteOAuthError(w, "rate_limit",
			fmt.Sprintf("retry after %.0f seconds", sendSMSRateLimit.Seconds()),
			http.StatusTooManyRequests)
		return
	}

	code, _ := generateCode()

	vc := model.VerificationCode{
		Phone:     req.Phone,
		Code:      code,
		ExpiresAt: time.Now().Add(codeTTL).Format(time.RFC3339),
		Used:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	h.codeStore[req.Phone] = append(h.codeStore[req.Phone], vc)
	h.codeLastSent[req.Phone] = time.Now()
	h.codeMu.Unlock()

	if err := h.smsSender.Send(r.Context(), req.Phone, code); err != nil {
		slog.Error("send sms", "error", err)
		WriteOAuthError(w, "server_error", "failed to send sms", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"message": "code sent"}, http.StatusOK)
}

// ── 内部辅助 ──

func (h *AuthHandler) findValidCode(phone, code string) *model.VerificationCode {
	h.codeMu.Lock()
	defer h.codeMu.Unlock()

	codes := h.codeStore[phone]
	now := time.Now()
	for i := len(codes) - 1; i >= 0; i-- {
		vc := codes[i]
		if vc.Code != code || vc.Used {
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
	h.codeMu.Lock()
	defer h.codeMu.Unlock()
	for i := range h.codeStore[phone] {
		if h.codeStore[phone][i].Code == code && !h.codeStore[phone][i].Used {
			h.codeStore[phone][i].Used = true
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
