package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/korjavin/tedsuggester/deepseek"
	"github.com/korjavin/tedsuggester/scheduler"
	"github.com/korjavin/tedsuggester/ted"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telebot.v3"
)

// Config holds all configuration values
type Config struct {
	BotToken       string
	TopicList      []string
	DeepseekAPIKey string
	GroupID        int64
}

// App represents the main application
type App struct {
	config *Config
	db     *sql.DB
	bot    *telebot.Bot
	ds     *deepseek.Client
	ted    *ted.Client
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("BOT_TOKEN is required")
	}

	topics := os.Getenv("TOPIC_LIST")
	if topics == "" {
		return nil, errors.New("TOPIC_LIST is required")
	}

	apiKey := os.Getenv("DEEPSEEK_APIKEY")
	if apiKey == "" {
		return nil, errors.New("DEEPSEEK_APIKEY is required")
	}

	groupIDStr := os.Getenv("TG_GROUP_ID")
	if groupIDStr == "" {
		return nil, errors.New("TG_GROUP_ID is required")
	}

	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TG_GROUP_ID: %v", err)
	}

	return &Config{
		BotToken:       botToken,
		TopicList:      strings.Split(topics, ","),
		DeepseekAPIKey: apiKey,
		GroupID:        groupID,
	}, nil
}

func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS talks (
			id TEXT PRIMARY KEY,
			title TEXT,
			description TEXT,
			suggested_at DATETIME,
			poll_message_id TEXT,
			selected BOOLEAN DEFAULT 0,
			discussion_questions TEXT
		);
	`)
	return err
}

func (app *App) setupHealthCheck() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity
		if err := app.db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Database connection failed: %v", err)
			return
		}

		// Check Telegram bot status by attempting to get chat info
		_, err := app.bot.ChatByID(app.config.GroupID)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Telegram bot connection failed: %v", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	go func() {
		log.Println("Starting health check server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Health check server failed: %v", err)
		}
	}()
}

func (app *App) setupScheduler() *scheduler.Scheduler {
	sched := scheduler.New()

	// Monday 08:00 - Search and create poll
	sched.AddTask(scheduler.Task{
		Name:     "search_and_poll",
		Schedule: time.Monday,
		Time:     scheduler.WeeklySchedule(time.Monday, 8, 0),
		Handler:  app.handleMondayTask,
	})

	// Wednesday 18:00 - Close poll
	sched.AddTask(scheduler.Task{
		Name:     "close_poll",
		Schedule: time.Wednesday,
		Time:     scheduler.WeeklySchedule(time.Wednesday, 18, 0),
		Handler:  app.handleWednesdayTask,
	})

	// Sunday 12:00 - Prepare discussion
	sched.AddTask(scheduler.Task{
		Name:     "prepare_discussion",
		Schedule: time.Sunday,
		Time:     scheduler.WeeklySchedule(time.Sunday, 12, 0),
		Handler:  app.handleSundayTask,
	})

	return sched
}

func main() {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := sql.Open("sqlite3", "./tedsuggester.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := initializeDatabase(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Telegram bot
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  config.BotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Initialize API clients
	ds := deepseek.NewClient(config.DeepseekAPIKey)
	tedClient := ted.NewClient()

	// Create application
	app := &App{
		config: config,
		db:     db,
		bot:    bot,
		ds:     ds,
		ted:    tedClient,
	}

	// Setup health check
	app.setupHealthCheck()

	// Setup and start scheduler
	sched := app.setupScheduler()
	ctx := context.Background()
	go sched.Start(ctx)

	// Start the bot
	log.Println("Starting TED Suggester bot...")
	bot.Start()
}

func (app *App) handleMondayTask(ctx context.Context) error {
	// Select a random topic
	topic := app.config.TopicList[rand.Intn(len(app.config.TopicList))]

	// Search for talks
	talks, err := app.ted.SearchTalks(ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to search talks: %w", err)
	}

	// Filter talks (10-20 minutes)
	filteredTalks := app.ted.FilterTalks(talks, 600, 1200)
	if len(filteredTalks) < 5 {
		return fmt.Errorf("not enough talks found for topic: %s", topic)
	}

	// Select 5-6 candidates
	candidates := filteredTalks[:min(6, len(filteredTalks))]

	// Generate descriptions
	var pollOptions []telebot.PollOption
	for _, talk := range candidates {
		desc, err := app.ds.GenerateDescription(ctx, talk.Title)
		if err != nil {
			return fmt.Errorf("failed to generate description: %w", err)
		}
		pollOptions = append(pollOptions, telebot.PollOption{
			Text: fmt.Sprintf("%s\n%s", talk.Title, desc),
		})
	}

	// Create poll
	poll := &telebot.Poll{
		Type:     telebot.PollRegular,
		Question: fmt.Sprintf("Which TED talk about %s should we discuss?", topic),
		Options:  pollOptions,
	}

	msg, err := app.bot.Send(telebot.ChatID(app.config.GroupID), poll)
	if err != nil {
		return fmt.Errorf("failed to send poll: %w", err)
	}

	// Save poll message ID and talks
	for i, talk := range candidates {
		_, err := app.db.ExecContext(ctx, `
			INSERT INTO talks (id, title, description, suggested_at, poll_message_id)
			VALUES (?, ?, ?, ?, ?)
		`, talk.ID, talk.Title, pollOptions[i].Text, time.Now(), msg.Poll.ID)
		if err != nil {
			return fmt.Errorf("failed to save talk: %w", err)
		}
	}

	return nil
}

func (app *App) handleWednesdayTask(ctx context.Context) error {
	// Get the most recent poll
	var pollID string
	err := app.db.QueryRowContext(ctx, `
		SELECT poll_message_id 
		FROM talks 
		WHERE suggested_at = (
			SELECT MAX(suggested_at) 
			FROM talks
		) LIMIT 1
	`).Scan(&pollID)
	if err != nil {
		return fmt.Errorf("failed to get poll ID: %w", err)
	}

	// Get the original poll message
	var messageID int
	err = app.db.QueryRowContext(ctx, `
		SELECT poll_message_id 
		FROM talks 
		WHERE suggested_at = (
			SELECT MAX(suggested_at) 
			FROM talks
		) LIMIT 1
	`).Scan(&messageID)
	if err != nil {
		return fmt.Errorf("failed to get message ID: %w", err)
	}

	// Create message reference
	msgRef := &telebot.Message{ID: messageID, Chat: &telebot.Chat{ID: app.config.GroupID}}

	// Stop the poll
	poll, err := app.bot.StopPoll(msgRef)
	if err != nil {
		return fmt.Errorf("failed to stop poll: %w", err)
	}

	// Find the winning option
	var winningOption *telebot.PollOption
	maxVotes := -1
	for _, option := range poll.Options {
		if option.VoterCount > maxVotes {
			winningOption = &option
			maxVotes = option.VoterCount
		}
	}

	if winningOption == nil {
		return errors.New("no winning option found")
	}

	// Get the corresponding talk
	var talkID, talkTitle string
	err = app.db.QueryRowContext(ctx, `
		SELECT id, title 
		FROM talks 
		WHERE poll_message_id = ? 
		AND description LIKE ?
	`, pollID, "%"+winningOption.Text+"%").Scan(&talkID, &talkTitle)
	if err != nil {
		return fmt.Errorf("failed to find winning talk: %w", err)
	}

	// Mark talk as selected
	_, err = app.db.ExecContext(ctx, `
		UPDATE talks 
		SET selected = 1 
		WHERE id = ?
	`, talkID)
	if err != nil {
		return fmt.Errorf("failed to mark talk as selected: %w", err)
	}

	// Create announcement
	announcement := fmt.Sprintf(
		"We're going to discuss \"%s\" on Sunday at 18:00 Berlin time! 🎉\n\n%s",
		talkTitle,
		winningOption.Text,
	)

	// Send and pin announcement
	announcementMsg, err := app.bot.Send(telebot.ChatID(app.config.GroupID), announcement)
	if err != nil {
		return fmt.Errorf("failed to send announcement: %w", err)
	}

	err = app.bot.Pin(announcementMsg)
	if err != nil {
		return fmt.Errorf("failed to pin message: %w", err)
	}

	return nil
}

func (app *App) handleSundayTask(ctx context.Context) error {
	// Get the selected talk
	var talkID, talkTitle, description string
	err := app.db.QueryRowContext(ctx, `
		SELECT id, title, description 
		FROM talks 
		WHERE selected = 1 
		ORDER BY suggested_at DESC 
		LIMIT 1
	`).Scan(&talkID, &talkTitle, &description)
	if err != nil {
		return fmt.Errorf("failed to get selected talk: %w", err)
	}

	// Generate discussion questions
	questions, err := app.ds.GenerateDiscussionQuestions(ctx, talkTitle, description)
	if err != nil {
		return fmt.Errorf("failed to generate discussion questions: %w", err)
	}

	// Save questions to database
	_, err = app.db.ExecContext(ctx, `
		UPDATE talks 
		SET discussion_questions = ?
		WHERE id = ?
	`, strings.Join(questions, "\n"), talkID)
	if err != nil {
		return fmt.Errorf("failed to save discussion questions: %w", err)
	}

	// Prepare discussion message
	discussionMessage := fmt.Sprintf(
		"Here are some questions to guide our discussion of \"%s\":\n\n%s\n\nSee you at 18:00!",
		talkTitle,
		strings.Join(questions, "\n• "),
	)

	// Send discussion message
	_, err = app.bot.Send(telebot.ChatID(app.config.GroupID), discussionMessage)
	if err != nil {
		return fmt.Errorf("failed to send discussion message: %w", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
