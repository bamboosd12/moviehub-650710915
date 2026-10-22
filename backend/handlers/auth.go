package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB *pgxpool.Pool
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"ข้อมูล JSON ไม่ถูกต้อง"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.Email == "" || req.DisplayName == "" || len(req.Password) < 4 {
		http.Error(w, `{"error":"กรุณากรอกข้อมูลให้ครบและรหัสผ่านอย่างน้อย 4 ตัว"}`, http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"ไม่สามารถสร้างรหัสผ่านได้"}`, http.StatusInternalServerError)
		return
	}

	var id int64

	err = h.DB.QueryRow(
		r.Context(),
		`INSERT INTO users (name, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		req.DisplayName,
		req.Email,
		string(hash),
	).Scan(&id)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, `{"error":"อีเมลนี้ถูกใช้งานแล้ว"}`, http.StatusConflict)
			return
		}

		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"ไม่สามารถสร้างสมาชิกได้"}`, http.StatusInternalServerError)
			return
		}

		http.Error(w, `{"error":"เกิดข้อผิดพลาดของฐานข้อมูล"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"message": "สมัครสมาชิกสำเร็จ",
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, 400)
		return
	}

	var id int64
	var name, email, hash string

	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, email, password_hash
		 FROM users WHERE email=$1`,
		strings.ToLower(strings.TrimSpace(req.Email)),
	).Scan(&id, &name, &email, &hash)

	if err != nil || bcrypt.CompareHashAndPassword(
		[]byte(hash), []byte(req.Password),
	) != nil {
		http.Error(w, `{"error":"อีเมลหรือรหัสผ่านไม่ถูกต้อง"}`, 401)
		return
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		http.Error(w, `{"error":"JWT not configured"}`, 500)
		return
	}

	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(id, 10),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256, claims,
	).SignedString([]byte(secret))

	if err != nil {
		http.Error(w, `{"error":"Token error"}`, 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"member": map[string]interface{}{
			"id": id, "name": name, "email": email,
		},
	})
}
