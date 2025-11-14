package models

import "time"

type Task struct {
    ID        string     `json:"id"`
    UserID    string     `json:"user_id"`
    Text      string     `json:"text"`
    Created   time.Time  `json:"created"`
    Deadline  *time.Time `json:"deadline,omitempty"`
    Completed bool       `json:"completed"`
    Priority  string     `json:"priority"` // "low", "medium", "high"
    Category  string     `json:"category"` // "study", "work", "personal"
}

type Goal struct {
    ID          string     `json:"id"`
    UserID      string     `json:"user_id"`
    Title       string     `json:"title"`
    Description string     `json:"description"`
    Created     time.Time  `json:"created"`
    Deadline    time.Time  `json:"deadline"`
    Steps       []GoalStep `json:"steps"`
    Progress    int        `json:"progress"` // 0-100%
    Completed   bool       `json:"completed"`
}

type GoalStep struct {
    ID        string `json:"id"`
    Text      string `json:"text"`
    Completed bool   `json:"completed"`
}