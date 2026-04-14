.PHONY: dev dev-api dev-web build clean

# Run both Go API and SvelteKit dev server concurrently
dev:
	@echo "Starting Go API on :8080 and SvelteKit dev on :5173..."
	@$(MAKE) dev-api &
	@$(MAKE) dev-web
	@wait

dev-api:
	go run . serve --no-browser

dev-web:
	cd web && source ~/.nvm/nvm.sh && nvm use 22 && npm run dev -- --open

# Build production binary (frontend embedded in web/build)
build:
	./build.sh

clean:
	rm -f ninefingers
	rm -rf web/build web/.svelte-kit
