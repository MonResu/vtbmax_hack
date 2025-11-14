package storage

import (
	"sync"
	"time"
	
	"proddy-bot/internal/models"
)

type MemoryStorage struct {
	mu           sync.RWMutex
	users        map[string]*models.User
	userData     map[string]*models.UserData
	pomodoroStats map[string]*models.PomodoroStats
	tasks        map[string][]*models.Task    // userID -> tasks
	goals        map[string][]*models.Goal    // userID -> goals
	pomodoroSessions map[string][]*models.PomodoroSession // userID -> sessions
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		users:           make(map[string]*models.User),
		userData:        make(map[string]*models.UserData),
		pomodoroStats:   make(map[string]*models.PomodoroStats),
		tasks:           make(map[string][]*models.Task),
		goals:           make(map[string][]*models.Goal),
		pomodoroSessions: make(map[string][]*models.PomodoroSession),
	}
}

// User methods
func (s *MemoryStorage) SaveUser(user *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if user.RegistrationDate.IsZero() {
		user.RegistrationDate = time.Now()
	}
	user.LastActivity = time.Now()
	
	s.users[user.MAXUserID] = user
	
	// Initialize user data if not exists
	if _, exists := s.userData[user.MAXUserID]; !exists {
		s.userData[user.MAXUserID] = &models.UserData{
			UserID: user.MAXUserID,
			Tasks:  []models.Task{},
			Goals:  []models.Goal{},
			Settings: models.UserSettings{
				PomodoroWorkDuration:  25,
				PomodoroBreakDuration: 5,
				NotificationsEnabled:  true,
			},
		}
	}
	
	return nil
}

func (s *MemoryStorage) GetUser(userID string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	user, exists := s.users[userID]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (s *MemoryStorage) UpdateUserActivity(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if user, exists := s.users[userID]; exists {
		user.LastActivity = time.Now()
	}
	return nil
}

// UserData methods
func (s *MemoryStorage) GetUserData(userID string) (*models.UserData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, exists := s.userData[userID]
	if !exists {
		return nil, nil
	}
	return data, nil
}

func (s *MemoryStorage) SaveUserData(data *models.UserData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.userData[data.UserID] = data
	return nil
}

// Task methods
func (s *MemoryStorage) SaveTask(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if task.Created.IsZero() {
		task.Created = time.Now()
	}
	
	s.tasks[task.UserID] = append(s.tasks[task.UserID], task)
	return nil
}

func (s *MemoryStorage) GetUserTasks(userID string) ([]*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	tasks, exists := s.tasks[userID]
	if !exists {
		return []*models.Task{}, nil
	}
	return tasks, nil
}

func (s *MemoryStorage) DeleteTask(userID, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	tasks, exists := s.tasks[userID]
	if !exists {
		return nil
	}
	
	for i, task := range tasks {
		if task.ID == taskID {
			s.tasks[userID] = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
	return nil
}

// Goal methods
func (s *MemoryStorage) SaveGoal(goal *models.Goal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if goal.Created.IsZero() {
		goal.Created = time.Now()
	}
	
	s.goals[goal.UserID] = append(s.goals[goal.UserID], goal)
	return nil
}

func (s *MemoryStorage) GetUserGoals(userID string) ([]*models.Goal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	goals, exists := s.goals[userID]
	if !exists {
		return []*models.Goal{}, nil
	}
	return goals, nil
}

// Pomodoro methods
func (s *MemoryStorage) SavePomodoroSession(session *models.PomodoroSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.pomodoroSessions[session.UserID] = append(s.pomodoroSessions[session.UserID], session)
	return nil
}

func (s *MemoryStorage) GetUserPomodoroSessions(userID string) ([]*models.PomodoroSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	sessions, exists := s.pomodoroSessions[userID]
	if !exists {
		return []*models.PomodoroSession{}, nil
	}
	return sessions, nil
}

// Stats methods
func (s *MemoryStorage) GetPomodoroStats(userID string) (*models.PomodoroStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats, exists := s.pomodoroStats[userID]
	if !exists {
		return &models.PomodoroStats{
			UserID: userID,
		}, nil
	}
	return stats, nil
}

func (s *MemoryStorage) UpdatePomodoroStats(stats *models.PomodoroStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.pomodoroStats[stats.UserID] = stats
	return nil
}