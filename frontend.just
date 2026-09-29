# Install frontend dependencies
install:
    cd frontend && npm install

# Run frontend development server
dev:
    cd frontend && npm run dev

# Build frontend SPA production bundle
build:
    cd frontend && npm run build

# Clean frontend build artifacts
clean:
    rm -rf frontend/dist/
    find internal/web/static -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true
