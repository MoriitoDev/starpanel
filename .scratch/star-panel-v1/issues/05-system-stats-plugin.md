# 05: System-stats Plugin

**What to build:** built-in system CPU plus memory plus disk Widget using the same Plugin contract, proving real monitoring on top of the plumbing.

**Blocked by:** 03 frontend-plugin-hello-widget, 04 backend-plugin-subprocess-proxy.

**Status:** ready-for-agent

- [ ] System stats read returns CPU plus memory plus disk shape via versioned API
- [ ] System-stats Widget renders live values on default poll using stdlib proc source on Linux
- [ ] Non-Linux dev shows a clear stub instead of failing
- [ ] HTTP plus contract tests cover stats shape and Widget render
