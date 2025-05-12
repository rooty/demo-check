# Changelog

## [Unreleased]
### Added
- Implemented `DemoURL` struct to represent URL data with fields `Id`, `Platform`, `Translit`, and `PlayURL`.
- Added `SafeCounter` struct with thread-safe methods for incrementing and decrementing counters and threads.
- Introduced `worker` function to process URLs concurrently using HTTP requests.
- Added `saveAccess` function to log failed URL access attempts into the database.
- Implemented `GetNetError2String` function to handle and format network-related errors.
- Added `convertHTTPError` function to map HTTP error codes to descriptive strings.
- Integrated `.env` file support using `github.com/joho/godotenv` for environment variable management.
- Added database connection and initialization logic, including table creation and truncation.
- Implemented multithreaded URL processing with a configurable number of threads (`-n` flag).
- Added support for graceful termination of the program with proper resource cleanup.

### Changed
- Enhanced HTTP client configuration with custom headers and timeout settings.
- Improved error handling for database operations and HTTP requests.

### Fixed
- Resolved potential race conditions in thread and counter management using `sync.Mutex`.

### Removed
- Commented out unused functions `hasNetError` and `hasTimeOut`.

## [0.1.0] - Initial Release
- Initial implementation of the `demo-check` tool for validating and logging URL accessibility.