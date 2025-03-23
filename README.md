# TED Suggester Bot

A Telegram bot that suggests TED talks for group discussion, manages polls, and prepares discussion materials.

## Features

- Weekly TED talk suggestions based on topics
- Automated polls for group voting
- AI-generated discussion questions
- Persistent storage of suggested talks
- Automated scheduling of weekly tasks

## Architecture

The application consists of several components:

1. **Main Application** - Handles the Telegram bot and scheduling
2. **Deepseek Client** - Interface for AI-generated descriptions and questions
3. **TED Client** - Interface for searching TED talks
4. **Scheduler** - Manages weekly tasks
5. **Database** - SQLite database for storing talk history

## Requirements

1. Go 1.20+
2. Telegram bot token
3. Deepseek API key
4. TED API key
5. SQLite3

## Getting API Keys

### 1. Telegram Bot Token
1. Open Telegram and search for @BotFather
2. Start a chat and use `/newbot` command
3. Follow the instructions to create a new bot
4. Save the provided token

### 2. Deepseek API Key
1. Go to [Deepseek Developer Portal](https://developer.deepseek.com)
2. Create an account if you don't have one
3. Create a new application
4. Save the API key

### 3. TED API Key
1. Go to [TED API Developer Portal](https://developer.ted.com)
2. Sign up for an account
3. Create a new application
4. Save the API key

## Configuration

Create a `.env` file with the following variables:

```bash
BOT_TOKEN=your_telegram_bot_token
TOPIC_LIST=topic1,topic2,topic3
DEEPSEEK_APIKEY=your_deepseek_key
TED_APIKEY=your_ted_key
TG_GROUP_ID=your_group_id
```

## Running the Application

### Local Development

1. Install dependencies:
```bash
go mod tidy
```

2. Run the application:
```bash
go run main.go
```

### Using Podman

1. Build the container:
```bash
podman build -t tedsuggester .
```

2. Run the container:
```bash
podman run -d \
  --env-file .env \
  --name tedsuggester \
  tedsuggester
```

## GitHub Actions

The repository includes a GitHub Actions workflow that automatically builds and pushes a Docker image to GitHub Container Registry when changes are pushed to the main branch.

## Error Handling

The application includes retry logic for API calls and handles common errors:
- API rate limiting
- Network connectivity issues
- Invalid responses
- Database errors

## License

MIT License