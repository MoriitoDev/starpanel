# 11 — Ship the Plugin's ZIP from the repo's build

Status: ready-for-agent

## What

The Plugin is installable from this repo without anyone installing Go on the target
machine. The repo gains a `build:disk-space` command at the root that runs
`apps/api/plugins/disk-space/build.ps1`, the built ZIP is published as a release artefact
rather than committed, and the README says where to get it. The Plugin's `backend` and
`dist/` stay out of git — they are build output — and the ZIP that ships carries the
`0o755` mode on the binary.

## Why

Ticket 10 built the artefact and left the question open: the Plugin is made of a compiled
binary, so "clone and import" only works if somebody ran Go. Leaving that to a person
reading a README is how a Plugin that works on the author's machine never works on
anybody else's, and it is the exact failure ADR-0009's optionality was supposed to avoid.

## Acceptance

- [ ] A root `pnpm build:disk-space` runs the Plugin's build script and fails loudly if it
      fails, without requiring `pnpm build` first
- [ ] The ZIP is produced for `linux/amd64` and its `backend` entry carries `0o755`,
      checked by `node apps/api/plugins/disk-space/verify-zip.mjs` in whatever CI exists
      or in the release checklist if there is no CI
- [ ] Nothing built is committed: `backend`, `dist/` and `state/` stay ignored, and a
      fresh clone still passes `pnpm test` and `go test ./...` without ever running the
      Plugin's build
- [ ] The README says where the ZIP comes from in one sentence, next to the mounts it
      needs, and the Plugin's own README says the same
- [ ] If the project grows a release process, the ZIP is an artefact of it; until then the
      README says plainly that you build it yourself with Go on the machine where you can

## Notes

- The alternative — committing the ZIP, or the `linux/amd64` binary itself — was rejected
  for two reasons that both matter: a binary in git is a binary nobody reviews, and a
  committed ZIP goes stale the moment the widget changes while looking perfectly current.
  The Plugin is a folder; the folder is the source of truth.
- `pnpm-workspace.yaml` and the root `package.json` already exist, so the command belongs
  there next to `build`. It shells out to PowerShell on Windows and to `build.sh` on POSIX
  if that script ever appears (ticket 10 left it optional on purpose).
- If a CI ever appears, this ticket is where `GOOS=linux GOARCH=amd64 go build` belongs,
  and the exec-bit assertion is the one test worth running on a Linux runner: it is the
  half that cannot be checked on Windows (`chmod` there only moves the read-only flag).
- Nothing in this ticket touches the Plugin's behaviour; it is packaging and words.
