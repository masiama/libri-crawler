package main

type LogEvent string

const (
	LogEventCrawlerInitialized          LogEvent = "crawler_initialized"
	LogEventCrawlerServerStarted        LogEvent = "crawler_server_started"
	LogEventCrawlerServerFailed         LogEvent = "crawler_server_failed"
	LogEventRedisInitializationFailed   LogEvent = "redis_initialization_failed"
	LogEventCrawlStarted                LogEvent = "crawl_started"
	LogEventCrawlProgress               LogEvent = "crawl_progress"
	LogEventCrawlProgressPublishFailed  LogEvent = "crawl_progress_publish_failed"
	LogEventCrawlCompleted              LogEvent = "crawl_completed"
	LogEventCrawlCompletedPublishFailed LogEvent = "crawl_completed_publish_failed"
	LogEventCrawlFailed                 LogEvent = "crawl_failed"
	LogEventSourceFetchFailed           LogEvent = "source_fetch_failed"
	LogEventBookExistsCheckFailed       LogEvent = "book_exists_check_failed"
	LogEventBookPublishFailed           LogEvent = "book_publish_failed"
	LogEventImageDownloadFailed         LogEvent = "image_download_failed"
	LogEventHTTPResponseWriteFailed     LogEvent = "http_response_write_failed"
	LogEventStorageInitializationFailed LogEvent = "storage_initialization_failed"
	LogEventInvalidSource               LogEvent = "invalid_source"
	LogEventRequiredEnvMissing          LogEvent = "required_environment_variable_missing"
)
