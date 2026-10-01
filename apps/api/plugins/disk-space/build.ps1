# Builds the disk-space Plugin: the linux/amd64 backend binary and the ZIP that
# "Import a Plugin…" accepts.
#
# The binary sits beside manifest.json and widget.js because that is the path
# the Manifest runs (`./backend`). The Go source is src/, because a file and a
# directory cannot share a name in one folder.
#
# Run from anywhere:  pwsh apps/api/plugins/disk-space/build.ps1

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

# The one target this Plugin promises (D13). CGO_ENABLED=0 is the difference
# between a static binary that runs on alpine and one that fails on musl.
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
# A build script that inherits GOFLAGS can be broken by whoever calls it: `-mod=mod`
# makes `go run <file>` resolve the file as a module path and fail. The two lines
# below are the difference between a script that works from anywhere and one that
# works from a developer's shell.
$env:GOFLAGS = ""

Write-Host "building backend (linux/amd64)…"
go build -C "$here/src" -trimpath -ldflags "-s -w -X main.version=0.1.0" -o "$here/backend" .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

# The ZIP is built with System.IO.Compression and NOT with Compress-Archive.
# Compress-Archive writes no Unix mode into a zip entry, so the panel would
# unpack a backend it cannot execute — and core would then log
# "backend exited (permission denied); restarting in 2s" forever while the
# Plugins section looked healthy. The Unix mode lives in the high 16 bits of
# the entry's external attributes; 0o755 for the binary, 0o644 for the rest.
Add-Type -AssemblyName System.IO.Compression.FileSystem
$dist = Join-Path $here "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$zip = Join-Path $dist "disk-space.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }

# Anything that is not the Plugin: the previous ZIP, the journal written at
# runtime, and the ignore file itself.
$excluded = @("dist", "state", ".archiveignore")

$archive = [System.IO.Compression.ZipFile]::Open($zip, "Create")
try {
  Get-ChildItem -Path $here -Recurse -File | Where-Object {
    $relative = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    -not ($excluded | Where-Object { $relative -eq $_ -or $relative.StartsWith("$_/") })
  } | ForEach-Object {
    $relative = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    $entry = $archive.CreateEntry("disk-space/$relative", [System.IO.Compression.CompressionLevel]::Optimal)
    # 0x1ED is 0o755 (the backend) and 0x1A4 is 0o644 (everything else):
    # PowerShell has no octal literals, so the modes are written in hex.
    $entry.ExternalAttributes = $(if ($relative -eq "backend") { 0x1ED } else { 0x1A4 }) -shl 16
    $entry.LastWriteTime = $_.LastWriteTime
    $input = [System.IO.File]::OpenRead($_.FullName)
    try {
      $output = $entry.Open()
      try { $input.CopyTo($output) } finally { $output.Dispose() }
    } finally { $input.Dispose() }
  }
} finally { $archive.Dispose() }

# The external attributes above are only read as a Unix mode when the entry
# says it was made on Unix, and that lives in the high byte of the central
# directory's "version made by" field. System.IO.Compression stamps 0x0014
# (FAT) there, and .NET exposes no way to set it — so a correct 0o755 would be
# ignored and Go's archive/zip would report 0666. Patching the 2-byte field in
# each central directory record is the one thing this build cannot leave to a
# library, and the probe below fails the build if it ever stops working.
$bytes = [System.IO.File]::ReadAllBytes($zip)
$patched = 0
for ($i = 0; $i -le $bytes.Length - 4; $i++) {
  if ($bytes[$i] -ne 0x50 -or $bytes[$i + 1] -ne 0x4B -or $bytes[$i + 2] -ne 0x01 -or $bytes[$i + 3] -ne 0x02) { continue }
  $versionMadeBy = [System.BitConverter]::ToUInt16($bytes, $i + 4)
  $bytes[$i + 5] = 3   # high byte: 3 is Unix, which is what makes the mode mean anything
  $patched++
}
if ($patched -eq 0) { throw "no central directory found in $zip" }
[System.IO.File]::WriteAllBytes($zip, $bytes)
Write-Host "marked $patched entries as Unix-made"

# maxPluginArchiveBytes is 32 MiB in apps/api/server.go; failing at 24 MiB is
# failing before the owner has picked the file, not after.
$size = (Get-Item $zip).Length
if ($size -gt 25165824) {
  throw "disk-space.zip is $size bytes; the panel refuses anything over 32 MiB"
}
Write-Host ("wrote {0} ({1:N1} MiB)" -f $zip, ($size / 1MB))

# The build checks its own artefact the way core will read it. A build script
# that only trusts what it wrote is how the mode bug got shipped once already:
# the ZIP looked right and Go saw 0666.
#
# The probe is a *local* program: it reads the ZIP on this machine, so it is
# built for the host and not for linux/amd64, and it is built to an explicit
# temp .exe because `go run` compiles to a name with no extension and then fails
# to exec it on Windows.
$probe = Join-Path ([System.IO.Path]::GetTempPath()) "disk-space-probe-$([guid]::NewGuid()).exe"
$crossOS = $env:GOOS
$crossArch = $env:GOARCH
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  & go build -o $probe "$here/probe_zip_mode.go"
  if ($LASTEXITCODE -ne 0) { throw "the mode probe would not build" }
  & $probe "$zip"
  if ($LASTEXITCODE -ne 0) { throw "the ZIP's backend is not executable as Go reads it" }
} finally {
  $env:GOOS = $crossOS
  $env:GOARCH = $crossArch
  Remove-Item $probe -Force -ErrorAction SilentlyContinue
}
