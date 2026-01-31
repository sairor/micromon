package api

import (
    "encoding/json"
    "net/http"
    "fmt"
    // "mikromon/internal/db"
)

type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Role     string `json:"role"` // admin, tech, monitor
    FullName string `json:"full_name"`
}

var mockUsers = []User{
    {ID: "1", Username: "admin", Role: "admin", FullName: "Administrador"},
    {ID: "2", Username: "tech1", Role: "tech", FullName: "Técnico de Campo"},
}

func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
    // In prod: Fetch from DB
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(mockUsers)
}

func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
    var u User
    if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }
    
    // Mock Save
    u.ID = fmt.Sprintf("%d", len(mockUsers)+1)
    mockUsers = append(mockUsers, u)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(u)
}
