# News Bot Project

This is a Go project that implements a news bot capable of fetching and delivering news updates to users via Telegram. The bot uses the Telegram Bot API for communication and stores news updates in a Redis database.

## Features

- Fetches news data from an RSS feed.
- Stores news updates in a Redis database.
- Allows users to subscribe to news updates via the "/start" command.
- Provides a "/getnews" command to fetch a random news update for subscribed users.
- Logs activities to a file with log rotation capabilities.

## Installation

1. Make sure you have Go and Redis installed on your system.

2. Clone this repository to your local machine:

```bash
git clone https://github.com/yourusername/news-bot-project.git
cd news-bot-project
```

3. Set up the necessary environment variables. Create a `.env` file in the root directory and add the following environment variables:

```plaintext
REDIS_HOST=your_redis_host
REDIS_PASSWORD=your_redis_password
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
```

Replace `your_redis_host`, `your_redis_password`, and `your_telegram_bot_token` with your actual Redis server information and Telegram bot token.

## Configuration

The bot's logger behavior can be customized using the `config/logger_config.json` file. Modify the parameters to configure the logging directory, maximum log file size, number of log backups, log file age, log compression, and timezone.

## License

This project is licensed under the MIT License.

## Acknowledgments

The News Bot Project is inspired by the need for an efficient and informative news delivery system for Telegram users.
