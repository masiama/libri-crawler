package main

type LogEvent string

const (
	LogEventCrawlerInitialized          LogEvent = "crawler_initialized"
	LogEventCrawlerDaemonStarted        LogEvent = "crawler_daemon_started"
	LogEventRedisInitializationFailed   LogEvent = "redis_initialization_failed"
	LogEventRedisCommandFetchFailed     LogEvent = "redis_command_fetch_failed"
	LogEventRedisCloseFailed            LogEvent = "redis_close_failed"
	LogEventCommandReceived             LogEvent = "command_received"
	LogEventLockAcquisitionFailed       LogEvent = "lock_acquisition_failed"
	LogEventLockExtensionFailed         LogEvent = "lock_extension_failed"
	LogEventLockReleaseFailed           LogEvent = "lock_release_failed"
	LogEventSeenURLExtensionFailed      LogEvent = "seen_urls_extension_failed"
	LogEventSeenURLCheckFailed          LogEvent = "seen_url_check_failed"
	LogEventSeenURLClearFailed          LogEvent = "seen_url_clear_failed"
	LogEventCrawlStarted                LogEvent = "crawl_started"
	LogEventCrawlProgress               LogEvent = "crawl_progress"
	LogEventCrawlProgressPublishFailed  LogEvent = "crawl_progress_publish_failed"
	LogEventCrawlCompleted              LogEvent = "crawl_completed"
	LogEventCrawlCompletedPublishFailed LogEvent = "crawl_completed_publish_failed"
	LogEventCrawlFailed                 LogEvent = "crawl_failed"
	LogEventCrawlRejectedDuplicate      LogEvent = "crawl_rejected_duplicate"
	LogEventSourceFetchFailed           LogEvent = "source_fetch_failed"
	LogEventBookExistsCheckFailed       LogEvent = "book_exists_check_failed"
	LogEventBookPublishFailed           LogEvent = "book_publish_failed"
	LogEventImageDownloadFailed         LogEvent = "image_download_failed"
	LogEventStorageInitializationFailed LogEvent = "storage_initialization_failed"
	LogEventInvalidSource               LogEvent = "invalid_source"
	LogEventLoadDotenvFailed            LogEvent = "load_dotenv_failed"
	LogEventHeartbeatPublishFailed      LogEvent = "heartbeat_publish_failed"
)
