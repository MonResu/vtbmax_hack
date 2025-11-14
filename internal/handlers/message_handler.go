package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"proddy-bot/internal/models"
	"proddy-bot/internal/storage"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// Handler структура для обработчиков
type Handler struct {
	storage        storage.MemoryStorage
	activeTimers   map[string]*time.Timer // userID -> timer
	pomodoroStatus map[string]string      // userID -> status
}

// New создает новый экземпляр обработчика
func New(storage storage.MemoryStorage) *Handler {
	return &Handler{
		storage:        storage,
		activeTimers:   make(map[string]*time.Timer),
		pomodoroStatus: make(map[string]string),
	}
}

// HandleUpdate обрабатывает входящие обновления
func (h *Handler) HandleUpdate(ctx context.Context, api *maxbot.Api, update interface{}) {
	switch upd := update.(type) {
	case *schemes.MessageCreatedUpdate:
		h.handleMessage(ctx, api, upd)
	case *schemes.MessageCallbackUpdate:
		h.handleCallback(ctx, api, upd)
	}
}

// handleMessage обрабатывает текстовые сообщения
func (h *Handler) handleMessage(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCreatedUpdate) {
	chatID := int64(upd.Message.Recipient.ChatId)
	text := upd.Message.Body.Text
	userID := fmt.Sprintf("%d", upd.Message.Sender.UserId)

	fmt.Printf("Message from %s: %s\n", upd.Message.Sender.FirstName, text)

	// Регистрируем/обновляем пользователя
	h.registerUser(upd.Message.Sender, userID)

	response := h.generateResponse(ctx, api, text, upd.Message.Sender.FirstName, userID, chatID)

	// Отправляем ответ
	_, err := api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
	if err != nil {
		fmt.Printf("❌ Error sending message: %v\n", err)
	} else {
		fmt.Printf("✅ Message sent successfully!\n")
	}
}

// registerUser регистрирует или обновляет пользователя
func (h *Handler) registerUser(sender schemes.User, userID string) {
	user := &models.User{
		ID:               userID,
		MAXUserID:        userID,
		FirstName:        sender.FirstName,
		Username:         sender.Username,
		RegistrationDate: time.Now(),
		LastActivity:     time.Now(),
	}

	existingUser, _ := h.storage.GetUser(userID)
	if existingUser == nil {
		// Новый пользователь
		h.storage.SaveUser(user)
		fmt.Printf("✅ New user registered: %s (%s)\n", sender.FirstName, userID)
	} else {
		// Обновляем активность существующего пользователя
		h.storage.UpdateUserActivity(userID)
	}
}

// handleCallback обрабатывает callback-кнопки
func (h *Handler) handleCallback(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate) {
	// Обработка нажатий на кнопки
	userID := fmt.Sprintf("%d", upd.Callback.GetUserID())
	chatID := int64(upd.Callback.GetChatID())

	switch {
	case strings.HasPrefix(upd.Callback.Payload, "pomodoro_"):
		h.handlePomodoroCallback(ctx, api, upd, userID, chatID)
	case strings.HasPrefix(upd.Callback.Payload, "task_"):
		h.handleTaskCallback(ctx, api, upd, userID, chatID)
	case strings.HasPrefix(upd.Callback.Payload, "goal_"):
		h.handleGoalCallback(ctx, api, upd, userID, chatID)
	default:
		switch upd.Callback.Payload {
		case "tasks_list":
			h.handleTasksList(ctx, api, upd, userID)
		case "goals_list":
			h.handleGoalsList(ctx, api, upd, userID)
		case "stats":
			h.handleStats(ctx, api, upd, userID)
		}
	}
}

// generateResponse генерирует ответ на основе текста сообщения
func (h *Handler) generateResponse(ctx context.Context, api *maxbot.Api, text, userName, userID string, chatID int64) string {
	text = strings.ToLower(strings.TrimSpace(text))

	switch {
	case text == "/start" || text == "start" || text == "начать":
		return h.getWelcomeMessage(userName)

	case strings.Contains(text, "меню"):
		return h.getMainMenu()

	case strings.Contains(text, "помощь"):
		return h.getHelpMessage()

	case strings.Contains(text, "фокус") || strings.Contains(text, "pomodoro"):
		return h.getPomodoroStatus(userID)

	case strings.Contains(text, "задач") || strings.Contains(text, "дело"):
		return h.handleTaskCommand(text, userID)

	case strings.Contains(text, "цел"):
		return h.handleGoalCommand(text, userID)

	case strings.Contains(text, "стат"):
		return h.getStats(userID)

	default:
		return "🤔 Не совсем понял что ты имеешь в виду. Попробуй написать \"меню\" чтобы увидеть все возможности или \"помощь\" для справки!"
	}
}

// ========== POMODORO FUNCTIONALITY ==========

func (h *Handler) getPomodoroStatus(userID string) string {
	stats, _ := h.storage.GetPomodoroStats(userID)

	status := h.pomodoroStatus[userID]
	if status == "" {
		status = "не активен"
	}

	return fmt.Sprintf(`🎯 Режим фокуса (Pomodoro)

📊 Твоя статистика:
• Всего сессий: %d
• Завершено сегодня: %d
• Общее время фокуса: %d мин.
• Текущий статус: %s

Команды:
• "старт помодоро" - начать сессию (25 мин)
• "стоп помодоро" - завершить сессию
• "перерыв" - начать перерыв (5 мин)`,
		stats.TotalSessions,
		stats.CompletedToday,
		stats.TotalFocusTime,
		status)
}

func (h *Handler) handlePomodoroCallback(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string, chatID int64) {
	payload := upd.Callback.Payload

	switch payload {
	case "pomodoro_start":
		h.startPomodoro(ctx, api, userID, chatID)
	case "pomodoro_stop":
		h.stopPomodoro(ctx, api, userID, chatID)
	case "pomodoro_break":
		h.startBreak(ctx, api, userID, chatID)
	}
}

func (h *Handler) startPomodoro(ctx context.Context, api *maxbot.Api, userID string, chatID int64) {
	// Останавливаем предыдущий таймер если есть
	if timer, exists := h.activeTimers[userID]; exists {
		timer.Stop()
	}

	h.pomodoroStatus[userID] = "работа ⏰ 25 мин"

	session := &models.PomodoroSession{
		ID:        fmt.Sprintf("%d", time.Now().Unix()),
		UserID:    userID,
		StartTime: time.Now(),
		Duration:  25,
		Type:      "work",
		Completed: false,
	}

	h.storage.SavePomodoroSession(session)

	// Создаем таймер на 25 минут
	timer := time.AfterFunc(25*time.Minute, func() {
		h.completePomodoro(ctx, api, userID, chatID, session.ID)
	})

	h.activeTimers[userID] = timer

	response := "🎯 Pomodoro сессия началась!\n⏰ 25 минут фокуса...\n\nСосредоточься на задаче! 💪"
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) stopPomodoro(ctx context.Context, api *maxbot.Api, userID string, chatID int64) {
	if timer, exists := h.activeTimers[userID]; exists {
		timer.Stop()
		delete(h.activeTimers, userID)
	}

	h.pomodoroStatus[userID] = "остановлен"

	// Помечаем сессию как прерванную
	sessions, _ := h.storage.GetUserPomodoroSessions(userID)
	if len(sessions) > 0 {
		lastSession := sessions[len(sessions)-1]
		if !lastSession.Completed {
			lastSession.Interrupted = true
			lastSession.EndTime = time.Now()
			// Здесь нужно обновить сессию в storage
		}
	}

	response := "🛑 Pomodoro сессия остановлена\n\nМожешь начать заново когда будешь готов!"
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) startBreak(ctx context.Context, api *maxbot.Api, userID string, chatID int64) {
	// Останавливаем предыдущий таймер если есть
	if timer, exists := h.activeTimers[userID]; exists {
		timer.Stop()
	}

	h.pomodoroStatus[userID] = "перерыв ☕ 5 мин"

	// Создаем таймер на 5 минут
	timer := time.AfterFunc(5*time.Minute, func() {
		h.completeBreak(ctx, api, userID, chatID)
	})

	h.activeTimers[userID] = timer

	response := "☕ Время перерыва!\n⏰ 5 минут отдыха...\n\nРасслабься и отдохни! 😊"
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) completePomodoro(ctx context.Context, api *maxbot.Api, userID string, chatID int64, sessionID string) {
	delete(h.activeTimers, userID)
	h.pomodoroStatus[userID] = "завершен"

	// Обновляем сессию как завершенную
	sessions, _ := h.storage.GetUserPomodoroSessions(userID)
	for _, session := range sessions {
		if session.ID == sessionID {
			session.Completed = true
			session.EndTime = time.Now()
			// Здесь нужно обновить сессию в storage
			break
		}
	}

	// Обновляем статистику
	stats, _ := h.storage.GetPomodoroStats(userID)
	stats.TotalSessions++
	stats.CompletedToday++
	stats.TotalFocusTime += 25
	h.storage.UpdatePomodoroStats(stats)

	response := "✅ Pomodoro сессия завершена!\n\nОтличная работа! 🎉\n\nХочешь начать перерыв?"
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) completeBreak(ctx context.Context, api *maxbot.Api, userID string, chatID int64) {
	delete(h.activeTimers, userID)
	h.pomodoroStatus[userID] = "перерыв завершен"

	response := "✅ Перерыв завершен!\n\nГотов к новой сессии фокуса? 🚀"
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

// ========== TASK FUNCTIONALITY ==========

func (h *Handler) handleTaskCommand(text, userID string) string {
	text = strings.ToLower(text)

	switch {
	case strings.Contains(text, "добав") && strings.Contains(text, "задач"):
		return h.addTask(text, userID)
	case strings.Contains(text, "удали") && strings.Contains(text, "задач"):
		return h.deleteTask(text, userID)
	case strings.Contains(text, "выполни") && strings.Contains(text, "задач"):
		return h.completeTask(text, userID)
	case strings.Contains(text, "список") && strings.Contains(text, "задач"):
		return h.listTasks(userID)
	default:
		return h.getTasksStatus(userID)
	}
}

func (h *Handler) addTask(text, userID string) string {
	// Извлекаем описание задачи из текста
	parts := strings.Split(text, "задач")
	if len(parts) > 1 && len(parts[1]) > 0 {
		parts[1] = parts[1][1:]
	}
	if len(parts) < 2 {
		return "❌ Укажи описание задачи. Например: \"добавить задачу прочитать книгу\""
	}

	taskDescription := strings.TrimSpace(parts[1])
	if taskDescription == "" {
		return "❌ Описание задачи не может быть пустым"
	}

	task := &models.Task{
		ID:        fmt.Sprintf("%d", time.Now().Unix()),
		UserID:    userID,
		Text:      taskDescription,
		Created:   time.Now(),
		Completed: false,
		Priority:  "medium",
		Category:  "personal",
	}

	err := h.storage.SaveTask(task)
	if err != nil {
		return "❌ Ошибка при добавлении задачи"
	}

	return fmt.Sprintf("✅ Задача добавлена: \"%s\"\n\nИспользуй \"список задач\" чтобы посмотреть все задачи.", taskDescription)
}

func (h *Handler) deleteTask(text, userID string) string {
	tasks, _ := h.storage.GetUserTasks(userID)
	if len(tasks) == 0 {
		return "📝 У тебя пока нет задач для удаления!"
	}

	// Пытаемся извлечь номер задачи из текста
	var taskNumber int
	parts := strings.Fields(text)
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil && num > 0 && num <= len(tasks) {
			taskNumber = num
			break
		}
	}

	if taskNumber == 0 {
		return "❌ Укажи номер задачи для удаления. Например: \"удалить задачу 1\""
	}

	taskToDelete := tasks[taskNumber-1]
	err := h.storage.DeleteTask(userID, taskToDelete.ID)
	if err != nil {
		return "❌ Ошибка при удалении задачи"
	}

	return fmt.Sprintf("✅ Задача удалена: \"%s\"", taskToDelete.Text)
}

func (h *Handler) completeTask(text, userID string) string {
	tasks, _ := h.storage.GetUserTasks(userID)
	if len(tasks) == 0 {
		return "📝 У тебя пока нет задач!"
	}

	// Пытаемся извлечь номер задачи из текста
	var taskNumber int
	parts := strings.Fields(text)
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil && num > 0 && num <= len(tasks) {
			taskNumber = num
			break
		}
	}

	if taskNumber == 0 {
		return "❌ Укажи номер задачи для выполнения. Например: \"выполнить задачу 1\""
	}

	taskToComplete := tasks[taskNumber-1]
	taskToComplete.Completed = true
	// Здесь нужно обновить задачу в storage

	return fmt.Sprintf("✅ Задача выполнена: \"%s\"\n\nОтличная работа! 🎉", taskToComplete.Text)
}

func (h *Handler) listTasks(userID string) string {
	tasks, _ := h.storage.GetUserTasks(userID)

	if len(tasks) == 0 {
		return "📝 У тебя пока нет задач!\n\nДобавь первую задачу написав \"добавить задачу [описание]\""
	}

	var response strings.Builder
	response.WriteString("📝 Твои задачи:\n\n")

	for i, task := range tasks {
		status := "🔴"
		if task.Completed {
			status = "✅"
		}
		priorityIcon := "⚪"
		switch task.Priority {
		case "high":
			priorityIcon = "🔴"
		case "medium":
			priorityIcon = "🟡"
		case "low":
			priorityIcon = "🟢"
		}
		response.WriteString(fmt.Sprintf("%s%s %d. %s\n", status, priorityIcon, i+1, task.Text))
	}

	response.WriteString("\nКоманды:\n• \"выполнить задачу 1\" - отметить как выполненную\n• \"удалить задачу 1\" - удалить задачу")

	return response.String()
}

func (h *Handler) handleTaskCallback(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string, chatID int64) {
	payload := upd.Callback.Payload

	if strings.HasPrefix(payload, "task_complete_") {
		taskID := strings.TrimPrefix(payload, "task_complete_")
		h.completeTaskByID(ctx, api, userID, chatID, taskID)
	} else if strings.HasPrefix(payload, "task_delete_") {
		taskID := strings.TrimPrefix(payload, "task_delete_")
		h.deleteTaskByID(ctx, api, userID, chatID, taskID)
	}
}

func (h *Handler) completeTaskByID(ctx context.Context, api *maxbot.Api, userID string, chatID int64, taskID string) {
	tasks, _ := h.storage.GetUserTasks(userID)
	for _, task := range tasks {
		if task.ID == taskID {
			task.Completed = true
			// Здесь нужно обновить задачу в storage
			response := fmt.Sprintf("✅ Задача выполнена: \"%s\"", task.Text)
			api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
			return
		}
	}
}

func (h *Handler) deleteTaskByID(ctx context.Context, api *maxbot.Api, userID string, chatID int64, taskID string) {
	tasks, _ := h.storage.GetUserTasks(userID)
	for _, task := range tasks {
		if task.ID == taskID {
			err := h.storage.DeleteTask(userID, taskID)
			if err != nil {
				api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText("❌ Ошибка при удалении задачи"))
				return
			}
			response := fmt.Sprintf("✅ Задача удалена: \"%s\"", task.Text)
			api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
			return
		}
	}
}

// ========== GOAL FUNCTIONALITY ==========

func (h *Handler) handleGoalCommand(text, userID string) string {
	text = strings.ToLower(text)

	switch {
	case strings.Contains(text, "добав") && strings.Contains(text, "цел"):
		return h.addGoal(text, userID)
	case strings.Contains(text, "удали") && strings.Contains(text, "цел"):
		return h.deleteGoal(text, userID)
	case strings.Contains(text, "прогресс") && strings.Contains(text, "цел"):
		return h.updateGoalProgress(text, userID)
	case strings.Contains(text, "список") && strings.Contains(text, "цел"):
		return h.listGoals(userID)
	default:
		return h.getGoalsStatus(userID)
	}
}

func (h *Handler) addGoal(text, userID string) string {
	// Упрощенная логика добавления цели
	parts := strings.Split(text, "цел")
	if len(parts) < 2 {
		return "❌ Укажи описание цели. Например: \"добавить цель выучить английский\""
	}

	goalTitle := strings.TrimSpace(parts[1])
	if goalTitle == "" {
		return "❌ Название цели не может быть пустым"
	}

	goal := &models.Goal{
		ID:          fmt.Sprintf("%d", time.Now().Unix()),
		UserID:      userID,
		Title:       goalTitle,
		Description: "Описание цели",
		Created:     time.Now(),
		Deadline:    time.Now().AddDate(0, 1, 0), // +1 месяц
		Progress:    0,
		Completed:   false,
		Steps:       []models.GoalStep{},
	}

	err := h.storage.SaveGoal(goal)
	if err != nil {
		return "❌ Ошибка при добавлении цели"
	}

	return fmt.Sprintf("✅ Цель добавлена: \"%s\"\n\nИспользуй \"список целей\" чтобы посмотреть все цели.", goalTitle)
}

func (h *Handler) deleteGoal(text, userID string) string {
	goals, _ := h.storage.GetUserGoals(userID)
	if len(goals) == 0 {
		return "🎯 У тебя пока нет целей для удаления!"
	}

	// Аналогично задачам - извлекаем номер цели
	var goalNumber int
	parts := strings.Fields(text)
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil && num > 0 && num <= len(goals) {
			goalNumber = num
			break
		}
	}

	if goalNumber == 0 {
		return "❌ Укажи номер цели для удаления. Например: \"удалить цель 1\""
	}

	goalToDelete := goals[goalNumber-1]
	// Здесь нужно добавить метод DeleteGoal в storage
	return fmt.Sprintf("✅ Цель удалена: \"%s\"", goalToDelete.Title)
}

func (h *Handler) updateGoalProgress(text, userID string) string {
	goals, _ := h.storage.GetUserGoals(userID)
	if len(goals) == 0 {
		return "🎯 У тебя пока нет целей!"
	}

	// Упрощенная логика обновления прогресса
	return "🔄 Функция обновления прогресса целей скоро будет доступна!"
}

func (h *Handler) listGoals(userID string) string {
	goals, _ := h.storage.GetUserGoals(userID)

	if len(goals) == 0 {
		return "🎯 У тебя пока нет целей!\n\nДобавь первую цель написав \"добавить цель [название]\""
	}

	var response strings.Builder
	response.WriteString("🎯 Твои цели:\n\n")

	for i, goal := range goals {
		status := "🟡"
		if goal.Completed {
			status = "✅"
		} else if goal.Progress == 100 {
			status = "🟢"
		}
		progressBar := h.createProgressBar(goal.Progress)
		response.WriteString(fmt.Sprintf("%s %d. %s\n%s %d%%\n\n", status, i+1, goal.Title, progressBar, goal.Progress))
	}

	return response.String()
}

func (h *Handler) createProgressBar(progress int) string {
	const barLength = 10
	filled := progress * barLength / 100
	empty := barLength - filled

	bar := "🟩"
	for i := 0; i < filled; i++ {
		bar += "🟩"
	}
	for i := 0; i < empty; i++ {
		bar += "⬜"
	}
	bar += "🟩"

	return bar
}

func (h *Handler) handleGoalCallback(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string, chatID int64) {
	// Аналогично задачам - обработка callback для целей
}

// ========== ОСТАВШИЕСЯ МЕТОДЫ ==========

func (h *Handler) getWelcomeMessage(userName string) string {
	return fmt.Sprintf(`🎉 Добро пожаловать в Proddy, %s!

Я твой личный помощник по продуктивности! Вот что я умею:

🎯 Режим фокуса - Pomodoro таймер для концентрации
📝 Задачи и расписание - Организуй свои дела  
🎯 Цели и прогресс - Ставь цели и отслеживай прогресс
📊 Статистика - Анализируй свою продуктивность

Напиши "меню" чтобы открыть главное меню! 🚀`, userName)
}

func (h *Handler) getMainMenu() string {
	return `🎯 Главное меню Proddy

Выбери что хочешь сделать:

🎯 Режим фокуса (Pomodoro) - напиши "фокус"
📝 Мои задачи и расписание - напиши "задачи"  
🎯 Цели и прогресс - напиши "цели"
📊 Статистика и аналитика - напиши "статистика"

Или просто напиши что тебя интересует! 😊`
}

func (h *Handler) getHelpMessage() string {
	return `🆘 Помощь по командам

🎯 Pomodoro таймер:
• "старт помодоро" - начать сессию (25 мин)
• "стоп помодоро" - завершить сессию
• "перерыв" - начать перерыв (5 мин)

📝 Управление задачами:
• "добавить задачу [описание]" - новая задача
• "список задач" - все задачи
• "выполнить задачу 1" - отметить выполненной
• "удалить задачу 1" - удалить задачу

🎯 Управление целями:
• "добавить цель [название]" - новая цель
• "список целей" - все цели
• "прогресс цель 1 50" - обновить прогресс

Просто напиши нужную команду! 🚀`
}

func (h *Handler) getTasksStatus(userID string) string {
	tasks, _ := h.storage.GetUserTasks(userID)

	completed := 0
	for _, task := range tasks {
		if task.Completed {
			completed++
		}
	}

	return fmt.Sprintf(`📝 Управление задачами

📊 Твои задачи:
• Всего задач: %d
• Выполнено: %d
• Осталось: %d

Команды:
• "добавить задачу [описание]" - новая задача
• "список задач" - посмотреть все задачи
• "выполнить задачу 1" - отметить выполненной
• "удалить задачу 1" - удалить задачу`,
		len(tasks), completed, len(tasks)-completed)
}

func (h *Handler) getGoalsStatus(userID string) string {
	goals, _ := h.storage.GetUserGoals(userID)

	completed := 0
	inProgress := 0
	for _, goal := range goals {
		if goal.Completed {
			completed++
		} else if goal.Progress > 0 {
			inProgress++
		}
	}

	return fmt.Sprintf(`🎯 Работа с целями

📊 Твои цели:
• Всего целей: %d
• Завершено: %d
• В процессе: %d
• Новые: %d

Команды:
• "добавить цель [название]" - новая цель
• "список целей" - посмотреть все цели
• "прогресс цель 1 50" - обновить прогресс`,
		len(goals), completed, inProgress, len(goals)-completed-inProgress)
}

func (h *Handler) getStats(userID string) string {
	stats, _ := h.storage.GetPomodoroStats(userID)
	tasks, _ := h.storage.GetUserTasks(userID)
	goals, _ := h.storage.GetUserGoals(userID)

	completedTasks := 0
	for _, task := range tasks {
		if task.Completed {
			completedTasks++
		}
	}

	taskCompletion := 0.0
	if len(tasks) > 0 {
		taskCompletion = float64(completedTasks) / float64(len(tasks)) * 100
	}

	return fmt.Sprintf(`📊 Статистика продуктивности

🎯 Фокус:
• Сессий Pomodoro: %d
• Время фокуса: %d мин.
• Текущая серия: %d дней

📝 Задачи:
• Всего задач: %d
• Выполнено: %d (%.0f%%)

🎯 Цели:
• Активных целей: %d

Продолжай в том же духе! 💪`,
		stats.TotalSessions, stats.TotalFocusTime, stats.CurrentStreak,
		len(tasks), completedTasks, taskCompletion,
		len(goals))
}

// Callback handlers (оставшиеся)
func (h *Handler) handleTasksList(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string) {
	response := h.listTasks(userID)
	chatID := upd.Callback.GetChatID()
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) handleGoalsList(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string) {
	response := h.listGoals(userID)
	chatID := upd.Callback.GetChatID()
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}

func (h *Handler) handleStats(ctx context.Context, api *maxbot.Api, upd *schemes.MessageCallbackUpdate, userID string) {
	response := h.getStats(userID)
	chatID := upd.Callback.GetChatID()
	api.Messages.Send(ctx, maxbot.NewMessage().SetChat(chatID).SetText(response))
}
