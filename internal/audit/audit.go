package audit

import (
	"context"
	"time"

	"mikromon/internal/db"
)

type AuditLog struct {
    Timestamp time.Time `bson:"timestamp"`
    User      string    `bson:"user"`
    Action    string    `bson:"action"`
    Target    string    `bson:"target"`
    Details   string    `bson:"details"`
}

func LogAction(user, action, target, details string) {
    // Run in background to not block request
    go func() {
        collection := db.GetCollection("audit_logs")
        if collection == nil {
            return // Skip logging in mock mode
        }
        entry := AuditLog{
            Timestamp: time.Now(),
            User:      user,
            Action:    action,
            Target:    target,
            Details:   details,
        }
        collection.InsertOne(context.Background(), entry)
    }()
}
