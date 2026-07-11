package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"time"

	"github.com/quanttide/qtcloud-auth/auth"
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
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// sendSMSRateLimit 默认重发间隔.
const sendSMSRateLimit = 60 * time.Second

// codeTTL 验证码有效期.
const codeTTL = 5 * time.Minute

// SendCode POST /api/v1/sms/send 发送手机验证码.
func (h *AuthHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, "INVALID_INPUT", "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidPhone(req.Phone) {
		WriteError(w, "VALIDATION_ERROR", "invalid phone number", http.StatusBadRequest)
		return
	}

	// 频率限制
	h.codeMu.Lock()
	last, ok := h.codeLastSent[req.Phone]
	if ok && time.Since(last) < sendSMSRateLimit {
		h.codeMu.Unlock()
		WriteError(w, "RATE_LIMITED",
			fmt.Sprintf("retry after %.0f seconds", sendSMSRateLimit.Seconds()),
			http.StatusTooManyRequests)
		return
	}

	code, err := generateCode()
	if err != nil {
		slog.Error("generate code", "error", err)
		h.codeMu.Unlock()
		WriteError(w, "INTERNAL_ERROR", "failed to generate code", http.StatusInternalServerError)
		return
	}

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
		WriteError(w, "SMS_FAILED", "failed to send sms", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"message": "code sent"}, http.StatusOK)
}

// LoginByPhone POST /api/v1/login/phone 手机验证码登录/自动注册.
func (h *AuthHandler) LoginByPhone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, "INVALID_INPUT", "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidPhone(req.Phone) || req.Code == "" {
		WriteError(w, "VALIDATION_ERROR", "phone and code are required", http.StatusBadRequest)
		return
	}

	vc := h.findValidCode(req.Phone, req.Code)
	if vc == nil {
		WriteError(w, "INVALID_CODE", "invalid or expired verification code", http.StatusUnauthorized)
		return
	}

	h.markCodeUsed(req.Phone, req.Code)

	user, err := h.findUserByPhone(req.Phone)
	if err != nil {
		slog.Error("find user by phone", "error", err)
		WriteError(w, "INTERNAL_ERROR", "failed to find user", http.StatusInternalServerError)
		return
	}

	if user == nil {
		user = &model.User{
			Username:      req.Phone,
			Phone:         req.Phone,
			PhoneVerified: true,
			Nickname:      maskPhone(req.Phone),
			CreatedAt:     time.Now().Format(time.RFC3339),
		}
		data, mErr := json.Marshal(user)
		if mErr != nil {
			slog.Error("marshal user", "error", mErr)
			WriteError(w, "INTERNAL_ERROR", "failed to create user", http.StatusInternalServerError)
			return
		}
		id, cErr := h.store.Create("auth/users", data)
		if cErr != nil {
			slog.Error("create user", "error", cErr)
			WriteError(w, "INTERNAL_ERROR", "failed to create user", http.StatusInternalServerError)
			return
		}
		user.ID = id
	} else if !user.PhoneVerified {
		user.PhoneVerified = true
		data, _ := json.Marshal(user)
		_ = h.store.Update("auth/users", user.ID, data)
	}

	claims := map[string]any{
		"sub":   user.ID,
		"phone": user.Phone,
		"role":  user.RoleID,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token, err := auth.Sign(claims, h.secret)
	if err != nil {
		slog.Error("sign token", "error", err)
		WriteError(w, "INTERNAL_ERROR", "failed to generate token", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, authResponse{Token: token, User: *user}, http.StatusOK)
}

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
