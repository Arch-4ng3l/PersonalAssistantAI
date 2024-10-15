all:
	cd ./backend; \
	go run main.go handlers.go auth.go models.go config/config.go
