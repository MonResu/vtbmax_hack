package models

import "time"

type PomodoroSession struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    StartTime   time.Time `json:"start_time"`
    EndTime     time.Time `json:"end_time"`
    Duration    int       `json:"duration"` // в минутах
    Completed   bool      `json:"completed"`
    Type        string    `json:"type"` // "work", "short_break", "long_break"
    Interrupted bool      `json:"interrupted"`
}

type PomodoroStats struct {
    UserID          string `json:"user_id"`
    TotalSessions   int    `json:"total_sessions"`
    CompletedToday  int    `json:"completed_today"`
    TotalFocusTime  int    `json:"total_focus_time"` // в минутах
    CurrentStreak   int    `json:"current_streak"`
}