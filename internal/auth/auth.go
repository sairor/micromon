package auth

import (
	"context"
	"encoding/json"
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
    if creds.Username == "admin" && creds.Password == "admin" {
        // Allow default admin for initial setup or mock mode
    } else {
        // Check DB
        collection := db.GetCollection("users")
        if collection == nil {
             // Mock Mode: Only admin/admin allowed if DB is down
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

// Middleware
func JwtMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Authorization header required", http.StatusUnauthorized)
            return
        }

        bearerToken := strings.Split(authHeader, " ")
        if len(bearerToken) != 2 {
            http.Error(w, "Invalid token format", http.StatusUnauthorized)
            return
        }

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(bearerToken[1], claims, func(token *jwt.Token) (interface{}, error) {
            return SecretKey, nil
        })

        if err != nil || !token.Valid {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        // Pass context if needed
        next.ServeHTTP(w, r)
    })
}

func MeHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(`{"user": "active"}`))
}
