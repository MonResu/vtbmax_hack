package models

import "time"

type User struct {
    ID              string    `json:"id"`
    MAXUserID       string    `json:"max_user_id"`
    FirstName       string    `json:"first_name"`
    Username        string    `json:"username"`
    RegistrationDate time.Time `json:"registration_date"`
    LastActivity    time.Time `json:"last_activity"`
}

type UserData struct {
    UserID           string            `json:"user_id"`
    PomodoroSessions []PomodoroSession `json:"pomodoro_sessions"`
    Tasks            []Task           `json:"tasks"`
    Goals            []Goal           `json:"goals"`
    Settings         UserSettings     `json:"settings"`
}

type UserSettings struct {
    PomodoroWorkDuration int `json:"pomodoro_work_duration"` // в минутах
    PomodoroBreakDuration int `json:"pomodoro_break_duration"`
    NotificationsEnabled bool `json:"notifications_enabled"`
}