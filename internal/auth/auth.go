package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mikromon/internal/db"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

var SecretKey = []byte("SUPER_SECRET_KEY_CHANGE_ME")

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password_hash"`
	Role     string `json:"role" bson:"role"` // admin, tech, monitor
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Find user in DB
	// For prototype, we will create a mock admin if not exists (TODO: Remove in prod)
	if (creds.Username == "admin" && creds.Password == "admin") ||
		(creds.Username == "sairo" && creds.Password == "sairo") ||
		(creds.Username == "samuel" && creds.Password == "admin123") {
		// Allow default users
	} else {
		// Check DB
		collection := db.GetCollection("users")
		if collection == nil {
			http.Error(w, "Invalid credentials (Mock Mode)", http.StatusUnauthorized)
			return
		}

		var result User
		err := collection.FindOne(context.TODO(), bson.M{"username": creds.Username}).Decode(&result)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		// Check Hash
		if err := bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(creds.Password)); err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
	}

	// Generate JWT
	expirationTime := time.Now().Add(12 * time.Hour)
	claims := &Claims{
		Username: creds.Username,
		Role:     "admin", // Defaulting to admin for the prototype login
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(SecretKey)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	username := r.Context().Value("username").(string)
	fmt.Printf("DEBUG: User %s changed password to [HIDDEN] (Mocked)\n", username)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Senha alterada com sucesso"}`))
}

// Middleware
func JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		tokenStr := ""

		if authHeader != "" {
			bearerToken := strings.Split(authHeader, " ")
			if len(bearerToken) == 2 {
				tokenStr = bearerToken[1]
			}
		} else {
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			http.Error(w, "Authorization header or token required", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return SecretKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Pass claims to context
		ctx := context.WithValue(r.Context(), "username", claims.Username)
		ctx = context.WithValue(ctx, "role", claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func MeHandler(w http.ResponseWriter, r *http.Request) {
	usernameVal := r.Context().Value("username")
	roleVal := r.Context().Value("role")

	username, _ := usernameVal.(string)
	role, _ := roleVal.(string)

	fullName := "Usuário"
	if username == "admin" {
		fullName = "Administrador"
	}
	if username == "sairo" {
		fullName = "Sairo J."
	}
	if username == "samuel" {
		fullName = "Samuel R."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"username":  username,
		"role":      role,
		"full_name": fullName,
	})
}
