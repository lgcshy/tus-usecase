import sys
from loguru import logger


# Remove default logger
logger.remove()

# Add file logging
logger.add("logs/app.log", rotation="10 MB", retention="10 days")

# Add console logging with your specified format
logger.add(sys.stdout, colorize=True, format="<green>{time}</green> <level>{message}</level>")

# Export logger instance
log = logger