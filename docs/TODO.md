# Lightfold Deploy Implementation

## Overview
End-to-end SSH-based deployment system for DigitalOcean droplets. Takes detector output and deploys applications with nginx reverse proxy, systemd service management, and health checks.

## Phase 1: SSH Executor (Week 1)
- [x] SSH connection management with retry logic
- [x] Remote command execution with stdout/stderr streaming
- [x] File upload via SCP/rsync over SSH
- [x] Template rendering and remote file writing
- [x] Sudo command execution
- [x] Unit tests for SSH executor

## Phase 2: Core Deployment (Week 2)
- [x] Base package installation (apt-get, nginx, language runtimes)
- [x] Directory structure setup (/srv/<app>/{releases,shared})
- [x] Release tarball creation and upload
- [x] Build execution per framework (Python venv, npm install, go build)
- [x] Environment variable management (.env files)
- [x] Unit tests for deployment steps

## Phase 3: Service Configuration (Week 3)
- [x] Systemd unit template and generation
- [x] Framework-specific ExecStart command mapping
- [x] Nginx reverse proxy template
- [x] Static file serving configuration
- [x] Service enable/start/restart management
- [x] Nginx config test and reload
- [x] Unit tests for service configuration

## Phase 4: Release Management (Week 4)
- [x] Atomic symlink switching (current -> releases/<timestamp>)
- [x] Health check implementation (HTTP GET with retries)
- [x] Automatic rollback on health check failure
- [x] Release retention cleanup (keep last 5)
- [x] Manual rollback command (--rollback flag)
- [x] Integration tests for release management

## Phase 5: Integration & CLI
- [x] Update config schema with deployment options
- [x] Integrate deployment executor with orchestrator
- [x] Add --env-file flag for environment variables
- [x] Add --env KEY=VALUE flag for individual vars
- [x] Add --skip-build flag
- [x] Add --rollback flag
- [x] Stream deployment logs to TUI progress display

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


## nginx template
```
server {
  listen 80;
  server_name _;

  access_log /var/log/nginx/<app>_access.log;
  error_log  /var/log/nginx/<app>_error.log;

  location /static/ { alias /srv/<app>/shared/static/; }
  location /media/  { alias /srv/<app>/shared/media/; }

  location / {
    proxy_pass http://127.0.0.1:8000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

## systemd unit template
```
[Unit]
Description=<app>
After=network.target

[Service]
WorkingDirectory=/srv/<app>/current
EnvironmentFile=/srv/<app>/shared/env/.env
ExecStart=<EXEC_START>
Restart=always
RestartSec=5
User=www-data
Group=www-data
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```