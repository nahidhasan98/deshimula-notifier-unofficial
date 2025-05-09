# Deshimula Notifier (Unofficial)

An unofficial Discord notifier for [Deshimula](https://deshimula.com) that monitors and sends story updates to Discord channel.

📢 **[Join our Discord Channel](https://discord.gg/7R58CMmksV)** to receive Mula updates!

## Features

- 🔄 Automatically fetches new stories from Deshimula
- 📢 Sends notifications to Discord via webhooks
- 🏷️ Includes story title, company, tags, and full description
- 💾 Tracks sent stories to prevent duplicates
- 📝 Handles large messages by splitting them into chunks

## Project Structure

- `config/` - Configuration settings and HTTP client setup
- `errorhandling/` - Error types and handling utilities
- `mula/` - Core functionality for fetching and processing stories
- `storage/` - Story storage management to prevent duplicates

## Error Handling

The application includes comprehensive error handling for:
- Network issues
- Configuration problems
- Discord webhook failures
- Storage operations

## License

This is an unofficial tool and is not affiliated with [Deshimula](https://deshimula.com).

