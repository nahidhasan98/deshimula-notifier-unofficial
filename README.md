# Deshimula Notifier Unofficial

An unofficial notification service for [Deshimula](https://deshimula.com) and [Oak](https://oakthu.com/) stories. This service monitors both platforms for new stories and sends notifications to Discord.

📢 **[Join our Discord Channel](https://discord.gg/7R58CMmksV)** to receive Mula updates!

## Features

- Monitors both Deshimula and Oak platforms for new stories
- Sends notifications to Discord with rich embeds via webhooks
- Handles story content in chunks for better readability
- Implements retry mechanism for failed operations
- Efficient storage of processed stories
- Concurrent processing of multiple stories

## Project Structure

```
.
├── base/           # Common functionality shared between services
├── config/         # Configuration management
├── errorhandling/  # Error handling and retry mechanisms
├── interfacer/     # Service interfaces
├── mula/          # Deshimula service implementation
├── oak/           # Oak service implementation
└── storage/       # Story storage implementation
```

## Setup

1. Clone the repository:
```bash
git clone https://github.com/nahidhasan98/deshimula-notifier-unofficial.git
cd deshimula-notifier-unofficial
```

2. Set up environment variables:
```bash
# Required environment variables
export WEBHOOK_ID_MULA="your_mula_webhook_id"
export WEBHOOK_TOKEN_MULA="your_mula_webhook_token"
export WEBHOOK_ID_OAK="your_oak_webhook_id"
export WEBHOOK_TOKEN_OAK="your_oak_webhook_token"
export WEBHOOK_ID_ERROR="your_error_webhook_id"
export WEBHOOK_TOKEN_ERROR="your_error_webhook_token"

# Optional environment variables
export MODE="DEVELOPMENT"  # Set to "DEVELOPMENT" to send all notifications to error webhook
```

3. Build and run:
```bash
go build
./deshimula-notifier-unofficial
```

## Architecture

The project follows a modular architecture with the following components:

### Base Package
- Provides common functionality for story services
- Handles Discord notifications
- Manages story storage
- Implements first-run handling
- Provides HTTP client configuration

### Service Implementations
- `Mula`: Implements Deshimula story parsing
- `Oak`: Implements Oak story parsing
- Both services inherit common functionality from the base package

### Error Handling
- Implements retry mechanism for failed operations
- Configurable retry attempts and delays
- Comprehensive error types and messages

### Storage
- Efficient storage of processed stories
- Prevents duplicate notifications
- Persists across service restarts

## First Run Behavior

On the first run, the service:
1. Processes only the most recent story (to show it in Discord)
2. Marks all other stories as seen (to prevent them from being processed in future runs)
3. Subsequent runs process only new stories

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Deshimula](https://deshimula.com)
- [Oak](https://oakthu.com)
- [Discord Text Hook](https://github.com/nahidhasan98/discord-text-hook)

