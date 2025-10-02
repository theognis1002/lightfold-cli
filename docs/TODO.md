# Lightfold Deploy Implementation

## Overview
End-to-end SSH-based deployment system for DigitalOcean droplets. Takes detector output and deploys applications with nginx reverse proxy, systemd service management, and health checks.

## Phase 1: SSH Executor (Week 1)
- [ ] SSH connection management with retry logic
- [ ] Remote command execution with stdout/stderr streaming
- [ ] File upload via SCP/rsync over SSH
- [ ] Template rendering and remote file writing
- [ ] Sudo command execution
- [ ] Unit tests for SSH executor

## Phase 2: Core Deployment (Week 2)
- [ ] Base package installation (apt-get, nginx, language runtimes)
- [ ] Directory structure setup (/srv/<app>/{releases,shared})
- [ ] Release tarball creation and upload
- [ ] Build execution per framework (Python venv, npm install, go build)
- [ ] Environment variable management (.env files)
- [ ] Unit tests for deployment steps

## Phase 3: Service Configuration (Week 3)
- [ ] Systemd unit template and generation
- [ ] Framework-specific ExecStart command mapping
- [ ] Nginx reverse proxy template
- [ ] Static file serving configuration
- [ ] Service enable/start/restart management
- [ ] Nginx config test and reload
- [ ] Unit tests for service configuration

## Phase 4: Release Management (Week 4)
- [ ] Atomic symlink switching (current -> releases/<timestamp>)
- [ ] Health check implementation (HTTP GET with retries)
- [ ] Automatic rollback on health check failure
- [ ] Release retention cleanup (keep last 5)
- [ ] Manual rollback command (--rollback flag)
- [ ] Integration tests for release management

## Phase 5: Integration & CLI
- [ ] Update config schema with deployment options
- [ ] Integrate deployment executor with orchestrator
- [ ] Add --env-file flag for environment variables
- [ ] Add --env KEY=VALUE flag for individual vars
- [ ] Add --skip-build flag
- [ ] Add --rollback flag
- [ ] Stream deployment logs to TUI progress display

## Phase 6: Testing & Documentation
- [ ] End-to-end test: Go API deployment
- [ ] End-to-end test: Next.js app deployment
- [ ] End-to-end test: FastAPI deployment
- [ ] End-to-end test: Django deployment
- [ ] End-to-end test: Express.js deployment
- [ ] Rollback test scenarios
- [ ] Update AGENTS.md with deployment architecture
- [ ] Update README.md with deployment usage

## Acceptance Criteria
- ✅ Deploy a Go API project to DigitalOcean droplet
- ✅ App accessible on port 80 via droplet IP
- ✅ Nginx proxies to app on port 8000
- ✅ Health check passes, deployment marked successful
- ✅ Re-running deploy creates new release, keeps previous for rollback
- ✅ Rollback command works correctly
- ✅ No SSL/certbot (access via IP only)
- ✅ Idempotent - safe to re-run multiple times

## Framework-Specific ExecStart Commands

### Python
- **Django**: `/srv/<app>/shared/venv/bin/gunicorn <module>.wsgi:application --bind 127.0.0.1:8000 --workers 2`
- **FastAPI**: `/srv/<app>/shared/venv/bin/uvicorn main:app --host 127.0.0.1 --port 8000 --workers 2`
- **Flask**: `/srv/<app>/shared/venv/bin/gunicorn app:app --bind 127.0.0.1:8000 --workers 2`

### Node.js
- **Next.js**: `PORT=8000 /usr/bin/node /srv/<app>/current/.next/standalone/server.js`
- **Express.js**: `PORT=8000 /usr/bin/node /srv/<app>/current/server.js`
- **NestJS**: `PORT=8000 /usr/bin/node /srv/<app>/current/dist/main.js`

### Go
- **Go HTTP**: `/srv/<app>/current/app --port 8000`

### PHP
- **Laravel**: `php-fpm` + nginx FastCGI proxy (different from reverse proxy pattern)

### Ruby
- **Rails**: `/srv/<app>/shared/bundle/bin/puma -C /srv/<app>/current/config/puma.rb`

## Notes
- All apps normalize to port 8000 internally
- Nginx listens on port 80, proxies to 127.0.0.1:8000
- No SSL/certbot in initial implementation
- Access via droplet IP only
- Release directory: `/srv/<app>/releases/<timestamp>/`
- Current symlink: `/srv/<app>/current -> releases/<timestamp>/`
- Shared directories: `/srv/<app>/shared/{env,logs,static,media}/`
- Service user: `www-data`
- Retention: Keep last 5 releases
